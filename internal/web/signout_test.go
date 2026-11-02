package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestBrowserSignoutScopeAndSignInAgainPage(t *testing.T) {
	s, b := newServer(t)
	first, second := signIn(t, s), signIn(t, s)
	ctx := context.Background()
	page := do(s, "GET", "/", nil, map[string]string{"Cookie": first})
	for _, text := range []string{`action="/signout"`, `action="/signout/everywhere"`, `class="btn"`, "Sign Out Everywhere"} {
		if !strings.Contains(page.Body.String(), text) {
			t.Fatalf("missing authenticated control %q", text)
		}
	}
	if w := do(s, "POST", "/signout/everywhere", nil, map[string]string{"Cookie": first, "Origin": "https://other.example"}); w.Code != http.StatusForbidden {
		t.Fatal("foreign sign-out accepted")
	}
	if w := do(s, "GET", "/signout", nil, map[string]string{"Cookie": first}); w.Code != http.StatusMethodNotAllowed {
		t.Fatal("GET sign-out changed authority")
	}
	post := func(path, cookie string) {
		t.Helper()
		w := do(s, "POST", path, nil, map[string]string{"Cookie": cookie, "Origin": "http://" + addr})
		if w.Code != http.StatusSeeOther {
			t.Fatalf("sign-out: %d %s", w.Code, w.Body)
		}
		cookies := w.Result().Cookies()
		if len(cookies) != 1 || cookies[0].Value != "" || cookies[0].MaxAge >= 0 {
			t.Fatal("sign-out did not remove the browser cookie")
		}
	}
	post("/signout", first)
	w := do(s, "GET", "/", nil, map[string]string{"Cookie": first})
	if w.Code != http.StatusUnauthorized || !strings.Contains(w.Body.String(), "Sign in again") || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("revoked token did not show the sign-in-again page")
	}
	if w := do(s, "GET", "/", nil, map[string]string{"Cookie": second}); w.Code != http.StatusOK {
		t.Fatal("sign-out revoked a different browser")
	}
	third := signIn(t, s)
	unused, err := b.NewWebCode(ctx)
	if err != nil {
		t.Fatal(err)
	}
	post("/signout/everywhere", second)
	for _, cookie := range []string{first, second, third} {
		if w := do(s, "GET", "/", nil, map[string]string{"Cookie": cookie}); w.Code != http.StatusUnauthorized {
			t.Fatal("global sign-out left a browser authenticated")
		}
	}
	if _, err := b.ExchangeWebCode(ctx, unused); err == nil {
		t.Fatal("unused link bypassed global sign-out")
	}
}

type eventProbe struct {
	header http.Header
	flush  chan struct{}
}

func (p *eventProbe) Header() http.Header         { return p.header }
func (p *eventProbe) WriteHeader(int)             {}
func (p *eventProbe) Write(b []byte) (int, error) { return len(b), nil }
func (p *eventProbe) Flush() {
	select {
	case p.flush <- struct{}{}:
	default:
	}
}

func TestRevokingBrowserTokenEndsItsEventStream(t *testing.T) {
	s, b := newServer(t)
	cookie := signIn(t, s)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r := httptest.NewRequest("GET", "/events", nil).WithContext(ctx)
	r.Host = addr
	r.Header.Set("Cookie", cookie)
	w := &eventProbe{header: http.Header{}, flush: make(chan struct{}, 1)}
	done := make(chan struct{})
	go func() { defer close(done); s.ServeHTTP(w, r) }()
	select {
	case <-w.flush:
	case <-time.After(3 * time.Second):
		t.Fatal("event stream did not start")
	}
	if err := b.RevokeAllWebTokens(context.Background()); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("revoked event stream kept serving")
	}
}
