package main

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/netip"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"dinah/internal/bench"
	"dinah/internal/browser"
	"dinah/internal/contract"
	"dinah/internal/httphead"
	"dinah/internal/resident"
)

// defaultListen is where dinah serve listens when --listen names nothing. The
// port is fixed so a URL survives a restart, and a port somebody else holds
// is answered as unreachable rather than by trying another.
const defaultListen = "127.0.0.1:7340"

// listenFunc binds an address.
type listenFunc func(network, address string) (net.Listener, error)

// serveBase and serveListen are the two seams runServe is handed through:
// the context an interrupt is added to, and the function that binds. They
// hold context.Background and net.Listen, and only a test in this package
// replaces them, to stop the server without a signal and to see whether
// anything was bound at all. tableSiteRecorder is the precedent for a seam of
// this shape here.
var (
	serveBase   = context.Background
	serveListen = listenFunc(net.Listen)
)

// openURL opens a browser on a URL. It is browser.Open, and only a test in
// this package replaces it, to see what dinah ui would have opened without
// opening anything.
var openURL = browser.Open

// runServe serves the workbench over HTTP on the loopback interface until an
// interrupt arrives.
func runServe(s *session, parsed *arguments) int {
	ctx, stop := signal.NotifyContext(serveBase(), os.Interrupt)
	defer stop()
	return serveUntil(ctx, s, parsed, serveListen, nil)
}

// runUI serves the workbench as runServe does and opens a browser on it once
// the address is bound and announced. The two commands build one handler, so
// a browser pointed at dinah serve gets the same pages; this command differs
// only in opening one. A browser that will not open is reported on stderr and
// the server goes on serving, because the address it printed still works.
func runUI(s *session, parsed *arguments) int {
	ctx, stop := signal.NotifyContext(serveBase(), os.Interrupt)
	defer stop()
	return serveUntil(ctx, s, parsed, serveListen, func(url string) {
		if parsed.has("no-browser") {
			return
		}
		if err := openURL(url); err != nil {
			s.errLine(s.r.T("ui.no-browser", "url", url, "error", err.Error()))
		}
	})
}

// serveUntil is runServe with the context and the listen function handed
// in. The order is the specification's: parse --listen, apply the loopback
// rule, open the workbench, listen, print one line, call afterListen when it
// is set, and serve until the context is cancelled. Each refusal before the
// listen binds nothing. afterListen runs after the line is printed and before
// serving starts, and a request it causes waits in the listener's backlog
// until the server reads it.
func serveUntil(ctx context.Context, s *session, parsed *arguments, listen listenFunc, afterListen func(url string)) int {
	address := parsed.value("listen")
	if address == "" {
		address = defaultListen
	}
	host, port, err := net.SplitHostPort(address)
	if err != nil || !portNumber(port) {
		return s.reportError(contract.Refuse(contract.Usage, address))
	}
	bind, ok := loopbackHost(host)
	if !ok {
		return s.reportError(contract.Refuse(contract.NotLoopback, address))
	}
	library, err := s.open()
	if err != nil {
		return s.reportError(err)
	}
	root, err := filepath.Abs(library.Bench.Root)
	if err != nil {
		return s.reportError(err)
	}
	// A workbench this platform or this volume cannot watch is served from
	// disk per request, as it always was. That is not a refusal, and the
	// server writes nothing while it runs, so no line says so.
	held, err := residentOpen(root)
	if err != nil {
		held = nil
	}
	closeHeld := func() {
		if held != nil {
			held.Close()
		}
	}
	listener, err := listen("tcp", net.JoinHostPort(bind, port))
	if err != nil {
		closeHeld()
		return s.reportError(err)
	}
	bound := listener.Addr().(*net.TCPAddr).Port
	url := "http://" + net.JoinHostPort(bind, strconv.Itoa(bound)) + "/"
	s.announce(url, root)
	server := &http.Server{
		Handler: httphead.Handler(httphead.Config{
			Root:         root,
			Home:         s.home,
			DefaultActor: s.actor,
			Agent:        s.agent,
			Host:         bind,
			Port:         bound,
			Lang:         s.r.Tag,
			ParseLine:    typedLineParser(s.cfg),
			Resident:     held,
		}),
		ReadHeaderTimeout: 10 * time.Second,
	}
	if afterListen != nil {
		afterListen(url)
	}
	served := make(chan error, 1)
	go func() { served <- server.Serve(listener) }()
	select {
	case <-ctx.Done():
		stopping, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		server.Shutdown(stopping)
		closeHeld()
		return 0
	case err := <-served:
		closeHeld()
		return s.reportError(err)
	}
}

// residentOpen opens the workbench held in memory. It is resident.Open, and
// only a test in this package replaces it.
var residentOpen = func(root string) (*resident.Workbench, error) {
	return resident.Open(root, resident.Options{})
}

// typedLineParser is the parser the pages' command log runs a typed line
// through: the terminal's own steps, with the reader's own aliases.
func typedLineParser(cfg *bench.Config) func(words []string) (httphead.TypedLine, *contract.Refusal) {
	return func(words []string) (httphead.TypedLine, *contract.Refusal) {
		typed, refusal := parseTypedLine(cfg, words)
		if refusal != nil {
			return httphead.TypedLine{}, refusal
		}
		return httphead.TypedLine{Command: typed.Command, Arguments: typed.Arguments, Actor: typed.Actor}, nil
	}
}

// announce prints the one line dinah serve writes to stdout. It needs no
// flush: the process's stdout writer holds back only an incomplete UTF-8
// sequence at the end of a write, and a whole line carries none, so a caller
// waiting for the address reads it as soon as it is written.
func (s *session) announce(url, root string) {
	if s.format == formatHuman {
		s.line(s.r.T("serve.listening", "url", url, "workbench", root))
		return
	}
	announced := struct {
		URL       string `json:"url"`
		Workbench string `json:"workbench"`
	}{url, root}
	line, _ := json.Marshal(announced)
	s.line(string(line))
}

// portNumber reports whether a port is written as a decimal number from 0 to
// 65535.
func portNumber(port string) bool {
	if port == "" || strings.Trim(port, "0123456789") != "" {
		return false
	}
	n, err := strconv.Atoi(port)
	return err == nil && n <= 65535
}

// loopbackHost applies the loopback rule to the host part of --listen and
// returns the literal the head binds. The rule is a whitelist: localhost,
// bound as 127.0.0.1 so neither the hosts file nor the resolver decides where
// the server listens; an IPv4 literal in 127.0.0.0/8; and the IPv6 loopback
// address, matched as a parsed address with no zone and not IPv4-mapped.
// Everything else, the empty host included, is refused.
func loopbackHost(host string) (string, bool) {
	if strings.EqualFold(host, "localhost") {
		return "127.0.0.1", true
	}
	addr, err := netip.ParseAddr(host)
	if err != nil || addr.Zone() != "" {
		return "", false
	}
	if addr.Is4() && loopback4.Contains(addr) {
		return addr.String(), true
	}
	if addr.Is6() && !addr.Is4In6() && addr == netip.IPv6Loopback() {
		return addr.String(), true
	}
	return "", false
}

// loopback4 is the IPv4 loopback block.
var loopback4 = netip.MustParsePrefix("127.0.0.0/8")
