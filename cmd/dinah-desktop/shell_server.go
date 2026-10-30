package main

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	extension "github.com/paulmooreparks/dinah-extension-go"
	"studio/internal/bridge"
)

// shellAddr is where the window's page is served, on the profile's port
// (product.ShellPort). The port is fixed because the page keeps a little in
// its own storage, which a browser keeps per address.
var shellAddr = product.ShellAddr()

// shellServer serves the window's page, its files and the bridge to the
// App on a loopback address, and the window loads it as a browser would.
// The page is the app's whole authority over the person's files and agents,
// so only the window may reach it: a request must name this server's own
// host, which defeats DNS rebinding; it must carry the cookie the window
// received through a one-time code; a call must come from the page's own
// origin; and the Content Security Policy lets the page run only its own
// scripts, so HTML that reaches the page cannot run as script.
type shellServer struct {
	app    *App
	bridge *bridge.Bridge
	files  http.Handler
	addr   string
	token  string

	mu    sync.Mutex
	codes map[string]time.Time

	http *http.Server
	ln   net.Listener
}

// shellCookie is the cookie that signs the window in (product.ShellCookie).
var shellCookie = product.ShellCookie

// shellPolicy is the page's Content Security Policy. Scripts come only from
// the page's own files; WebAssembly may compile, for the Terminal's shells.
// Inline styles are allowed because styles cannot run script, and the
// applets set them throughout.
const shellPolicy = "default-src 'self'; script-src 'self' 'wasm-unsafe-eval'; style-src 'self' 'unsafe-inline'; " +
	"img-src 'self' data: blob:; font-src 'self' data:; media-src 'self' blob:; connect-src 'self'; " +
	"worker-src 'self' blob:; frame-src 'self' blob:; object-src 'none'; base-uri 'none'; " +
	"form-action 'self'; frame-ancestors 'self'"

func newShellServer(app *App, frontend fs.FS, addr string) (*shellServer, error) {
	b, err := newPageBridge(app, pageMethods)
	if err != nil {
		return nil, err
	}
	s := &shellServer{app: app, bridge: b, files: http.FileServerFS(frontend), addr: addr,
		token: rand.Text(), codes: map[string]time.Time{}}
	s.bridge.ObserveCall = app.observeExitCall
	s.bridge.Runtime("BrowserOpenURL", func(_ context.Context, args []json.RawMessage) error {
		var u string
		if len(args) == 0 || json.Unmarshal(args[0], &u) != nil {
			return errors.New("BrowserOpenURL needs an address")
		}
		return app.openInBrowser(u)
	})
	s.bridge.Runtime("ClipboardSetText", func(_ context.Context, args []json.RawMessage) error {
		var text string
		if len(args) == 0 || json.Unmarshal(args[0], &text) != nil {
			return errors.New("ClipboardSetText needs the text")
		}
		return wruntime.ClipboardSetText(app.ctx, text)
	})
	s.bridge.Runtime("Quit", func(context.Context, []json.RawMessage) error {
		wruntime.Quit(app.ctx)
		return nil
	})
	app.events = s.bridge
	// The port may still be held for a moment by a window that has just
	// ended, as when Restart to Update starts this one, so a refusal is
	// tried again for a few seconds before it is reported.
	ln, err := net.Listen("tcp", addr)
	for tries := 0; err != nil && tries < 20; tries++ {
		time.Sleep(250 * time.Millisecond)
		ln, err = net.Listen("tcp", addr)
	}
	if err != nil {
		return nil, fmt.Errorf("%s cannot serve its window on %s, perhaps because another program is using that port: %w", product.Name, addr, err)
	}
	s.ln = ln
	// A window under test asks for any free port, and the address is the
	// one it got.
	s.addr = ln.Addr().String()
	s.http = &http.Server{Handler: s, ReadHeaderTimeout: 10 * time.Second}
	return s, nil
}

func (s *shellServer) serve() { s.http.Serve(s.ln) }

func (s *shellServer) close() { s.http.Close() }

// entry returns an address that signs the window in, good once and for one
// minute.
func (s *shellServer) entry() string {
	code := rand.Text()
	s.mu.Lock()
	s.codes[code] = time.Now().Add(time.Minute)
	s.mu.Unlock()
	return "http://" + s.addr + "/enter?code=" + url.QueryEscape(code)
}

func (s *shellServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !s.hostAllowed(r.Host) {
		http.Error(w, product.Name+" answers only on "+s.addr+".", http.StatusMisdirectedRequest)
		return
	}
	h := w.Header()
	h.Set("Content-Security-Policy", shellPolicy)
	h.Set("X-Frame-Options", "SAMEORIGIN")
	h.Set("Referrer-Policy", "same-origin")
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Cache-Control", "no-store")
	if r.URL.Path == "/enter" {
		s.enter(w, r)
		return
	}
	if strings.HasPrefix(r.URL.Path, "/extension-assets/") {
		s.app.extensionAsset(w, r)
		return
	}
	if r.URL.Path == "/extension-sdk/dinah-extension.js" && (r.Method == "GET" || r.Method == "HEAD") {
		w.Header().Set("Content-Type", "text/javascript")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		if r.Method == "GET" {
			w.Write(extension.BrowserBridge)
			frameKeys, _ := fs.ReadFile(assets, "frontend/native-frame.js")
			w.Write(frameKeys)
		}
		return
	}
	if strings.HasPrefix(r.URL.Path, "/pudl/") && (r.Method == "GET" || r.Method == "HEAD") {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		s.files.ServeHTTP(w, r)
		return
	}
	c, err := r.Cookie(shellCookie)
	if err != nil || subtle.ConstantTimeCompare([]byte(c.Value), []byte(s.token)) != 1 {
		http.Error(w, "Open "+product.Name+" from its window.", http.StatusUnauthorized)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead && !s.sameOrigin(r) {
		http.Error(w, "Calls are accepted only from "+product.Name+"'s own page.", http.StatusForbidden)
		return
	}
	switch {
	case strings.HasPrefix(r.URL.Path, "/bridge/"):
		s.bridge.Handler("/bridge").ServeHTTP(w, r)
	case r.URL.Path == "/upload" && r.Method == http.MethodPost:
		r.Body = http.MaxBytesReader(w, r.Body, 8<<30)
		s.app.uploadDropped(w, r)
	case r.URL.Path == "/doc-file" || r.URL.Path == "/resource-attachment" || r.URL.Path == "/draft-file" || r.URL.Path == "/bus-attachment":
		s.app.docFiles().ServeHTTP(w, r)
	default:
		s.files.ServeHTTP(w, r)
	}
}

// enter exchanges a one-time code for the window's cookie, then moves on to
// the page with a page of its own rather than a redirect: the code is
// followed from the toolkit's own page, which makes a redirect chain
// cross-site, and a browser withholds a SameSite=Strict cookie from such a
// chain, while a navigation this page starts is same-site.
func (s *shellServer) enter(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	s.mu.Lock()
	expires, ok := s.codes[code]
	delete(s.codes, code)
	s.mu.Unlock()
	if !ok || time.Now().After(expires) {
		http.Error(w, "That way in has expired. Open "+product.Name+" from its window.", http.StatusUnauthorized)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: shellCookie, Value: s.token, Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode})
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<!doctype html><html lang="en"><head><meta charset="utf-8"><title>%s</title>`+
		`<meta http-equiv="refresh" content="0; url=/"></head><body></body></html>`, html.EscapeString(product.Name))
}

func (s *shellServer) hostAllowed(host string) bool {
	_, port, err := net.SplitHostPort(s.addr)
	if err != nil {
		return false
	}
	return host == s.addr || host == "localhost:"+port
}

func (s *shellServer) sameOrigin(r *http.Request) bool {
	o := r.Header.Get("Origin")
	if o == "" || o == "null" {
		return r.Header.Get("Sec-Fetch-Site") == "same-origin"
	}
	u, err := url.Parse(o)
	return err == nil && u.Scheme == "http" && u.Host == r.Host
}
