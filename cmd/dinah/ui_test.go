package main

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"dinah/internal/contract"
)

// uiRun is one dinah ui run in process through runCLI, with the browser
// recorder and the serve seams in place.
type uiRun struct {
	cancel context.CancelFunc
	done   chan invocation
	mu     sync.Mutex
	// opened are the URLs openURL was handed.
	opened []string
	// fetched carries the status a GET of each opened URL answered.
	fetched chan int
}

// startUI runs dinah ui with argv from dir, with openURL replaced by a
// recorder that answers fail, and the serve seams replaced so the test can
// stop the server and bind what it likes. The recorder cannot read stdout
// while runCLI holds it, so the order the specification gives, the startup
// line before the browser, is held by serveUntil's own order, and the test
// holds the URL opened to the URL printed once the run returns.
func startUI(t *testing.T, dir string, listen listenFunc, fail error, argv ...string) *uiRun {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	u := &uiRun{cancel: cancel, done: make(chan invocation, 1), fetched: make(chan int, 4)}
	previousBase, previousListen, previousOpen := serveBase, serveListen, openURL
	serveBase = func() context.Context { return ctx }
	serveListen = listen
	openURL = func(url string) error {
		u.mu.Lock()
		u.opened = append(u.opened, url)
		u.mu.Unlock()
		// The server starts reading once this returns, and the request
		// waits in the listener's backlog until it does.
		go func() {
			response, err := http.Get(url)
			if err != nil {
				u.fetched <- 0
				return
			}
			response.Body.Close()
			u.fetched <- response.StatusCode
		}()
		return fail
	}
	t.Cleanup(func() {
		cancel()
		serveBase, serveListen, openURL = previousBase, previousListen, previousOpen
	})
	go func() { u.done <- runCLI(t, dir, append([]string{"ui"}, argv...)...) }()
	return u
}

// stop cancels the run and waits for what it wrote.
func (u *uiRun) stop(t *testing.T) invocation {
	t.Helper()
	u.cancel()
	select {
	case got := <-u.done:
		return got
	case <-time.After(30 * time.Second):
		t.Fatal("dinah ui did not return")
	}
	return invocation{}
}

// status waits for the recorder's GET.
func (u *uiRun) status(t *testing.T) int {
	t.Helper()
	select {
	case status := <-u.fetched:
		return status
	case got := <-u.done:
		t.Fatalf("dinah ui returned %d before it served: %s", got.code, got.errw)
	case <-time.After(30 * time.Second):
		t.Fatal("the opened URL never answered")
	}
	return 0
}

// TestUIOpensTheBoundURLAfterAnnouncingIt is dinah-338/criteria/1: the
// browser is opened once, on the URL the startup line printed, after that
// line is written, and the URL answers.
func TestUIOpensTheBoundURLAfterAnnouncingIt(t *testing.T) {
	root := newBench(t)
	u := startUI(t, root, net.Listen, nil, "--listen", "127.0.0.1:0")
	if status := u.status(t); status != http.StatusOK {
		t.Errorf("GET of the opened URL answered %d", status)
	}
	got := u.stop(t)
	if got.code != 0 {
		t.Errorf("wanted exit 0, got %d %s", got.code, got.errw)
	}
	u.mu.Lock()
	defer u.mu.Unlock()
	if len(u.opened) != 1 {
		t.Fatalf("openURL was called %d times", len(u.opened))
	}
	url := u.opened[0]
	if !strings.HasPrefix(url, "http://127.0.0.1:") || !strings.HasSuffix(url, "/") || strings.HasSuffix(url, ":0/") {
		t.Errorf("opened %q", url)
	}
	if lines := strings.Split(strings.TrimSpace(got.out), "\n"); len(lines) != 1 || !strings.HasSuffix(lines[0], " at "+url) {
		t.Errorf("stdout is %q, which does not announce the URL opened, %s, as its one line", got.out, url)
	}
}

// TestUINoBrowser holds --no-browser to opening nothing.
func TestUINoBrowser(t *testing.T) {
	root := newBench(t)
	listening := make(chan string, 1)
	u := startUI(t, root, func(network, address string) (net.Listener, error) {
		l, err := net.Listen(network, address)
		if err == nil {
			listening <- l.Addr().String()
		}
		return l, err
	}, nil, "--no-browser", "--listen", "127.0.0.1:0")
	address := <-listening
	response, err := http.Get("http://" + address + "/whoami")
	if err != nil || response.StatusCode != http.StatusOK {
		t.Errorf("the server does not answer: %v", err)
	}
	u.stop(t)
	u.mu.Lock()
	defer u.mu.Unlock()
	if len(u.opened) != 0 {
		t.Errorf("--no-browser opened %v", u.opened)
	}
}

// TestUIBrowserFailureStillServes holds a browser that will not open to one
// line on stderr naming the URL, with the server still answering.
func TestUIBrowserFailureStillServes(t *testing.T) {
	root := newBench(t)
	u := startUI(t, root, net.Listen, errors.New("no browser here"), "--listen", "127.0.0.1:0")
	if status := u.status(t); status != http.StatusOK {
		t.Errorf("GET / answered %d after the browser failed", status)
	}
	got := u.stop(t)
	u.mu.Lock()
	defer u.mu.Unlock()
	want := "Could not open a browser (no browser here). Open " + u.opened[0] + " yourself.\n"
	if got.code != 0 || got.errw != want {
		t.Errorf("wanted exit 0 and stderr %q, got %d %q", want, got.code, got.errw)
	}
}

// TestUIRefusesWhatServeRefuses holds ui to serve's two refusals, each raised
// before anything is bound.
func TestUIRefusesWhatServeRefuses(t *testing.T) {
	root := newBench(t)
	for address, refusal := range map[string]string{"0.0.0.0:7340": contract.NotLoopback, "127.0.0.1": contract.Usage} {
		bound := false
		u := startUI(t, root, func(string, string) (net.Listener, error) {
			bound = true
			return nil, errors.New("the test binds nothing")
		}, nil, "--listen", address)
		got := <-u.done
		if got.code != contract.ExitCode(contract.OutcomeRefused) || !strings.HasPrefix(got.errw, refusal+" ") || bound {
			t.Errorf("--listen %s: wanted exit 2 leading with %s and nothing bound, got %d %q bound=%v", address, refusal, got.code, got.errw, bound)
		}
		if len(u.opened) != 0 {
			t.Errorf("--listen %s opened a browser", address)
		}
	}
}
