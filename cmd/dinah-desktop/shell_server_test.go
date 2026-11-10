package main

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The window's page is served with the Content Security Policy, and the
// policy lets the page run only its own script files: no inline script, no
// event-handler attribute, no javascript: address and no eval.
func TestShellPageHasContentSecurityPolicy(t *testing.T) {
	frontend, err := fs.Sub(assets, "frontend")
	if err != nil {
		t.Fatal(err)
	}
	s := &shellServer{files: http.FileServerFS(frontend), addr: "127.0.0.1:47616", token: "token", codes: map[string]shellCode{}}
	for _, path := range []string{"/", "/index.html", "/app.js"} {
		r := httptest.NewRequest("GET", "http://127.0.0.1:47616"+path, nil)
		r.AddCookie(&http.Cookie{Name: shellCookie, Value: "token"})
		w := httptest.NewRecorder()
		s.ServeHTTP(w, r)
		if w.Code != http.StatusOK && w.Code != http.StatusMovedPermanently {
			t.Fatalf("%s: status %d", path, w.Code)
		}
		if got := w.Header().Get("Content-Security-Policy"); got != shellPolicy {
			t.Errorf("%s is served with the policy %q, want %q", path, got, shellPolicy)
		}
	}
	var script string
	for _, d := range strings.Split(shellPolicy, ";") {
		if f := strings.Fields(d); len(f) > 0 && f[0] == "script-src" {
			script = d
		}
	}
	if script == "" {
		t.Fatal("the policy has no script-src")
	}
	for _, loose := range []string{"'unsafe-inline'", "'unsafe-eval'", "'unsafe-hashes'", "data:", "blob:", "*"} {
		if strings.Contains(script, loose) {
			t.Errorf("the policy's script-src allows %s: %s", loose, script)
		}
	}
	if !strings.Contains(shellPolicy, "object-src 'none'") || !strings.Contains(shellPolicy, "base-uri 'none'") {
		t.Errorf("the policy must allow no plug-in and no base address: %s", shellPolicy)
	}
}

// Only the D2 worker of the Diagrams applet may evaluate code, and it may
// reach nothing; every other file, the worker's neighbours included, keeps
// the page's policy.
func TestOnlyTheD2WorkerMayEvaluate(t *testing.T) {
	frontend, err := fs.Sub(assets, "frontend")
	if err != nil {
		t.Fatal(err)
	}
	s := &shellServer{files: http.FileServerFS(frontend), addr: "127.0.0.1:47616", token: "token", codes: map[string]shellCode{}}
	policy := func(path string) string {
		r := httptest.NewRequest("GET", "http://127.0.0.1:47616"+path, nil)
		r.AddCookie(&http.Cookie{Name: shellCookie, Value: "token"})
		w := httptest.NewRecorder()
		s.ServeHTTP(w, r)
		if w.Code != http.StatusOK {
			t.Fatalf("%s: status %d", path, w.Code)
		}
		return w.Header().Get("Content-Security-Policy")
	}
	if got := policy(d2WorkerPath + "?v=1"); got != d2WorkerPolicy || !strings.Contains(got, "default-src 'none'") {
		t.Fatalf("the D2 worker is served with %q", got)
	}
	for _, path := range []string{"/yavc/content/diagram-kit/d2.js", "/yavc/content/md-editor/import-word-worker.js", "/yavc/content/diagram-kit/worker.js/../d2.js"} {
		if got := policy(path); got != shellPolicy {
			t.Errorf("%s is served with %q", path, got)
		}
	}
}
