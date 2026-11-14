package main

import (
	"io/fs"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

func TestShellMissingPageFilesAreNotStored(t *testing.T) {
	frontend, err := fs.Sub(assets, "frontend")
	if err != nil {
		t.Fatal(err)
	}
	s := &shellServer{files: http.FileServerFS(frontend), addr: "127.0.0.1:47616", token: "token", codes: map[string]shellCode{}}
	for _, method := range []string{http.MethodGet, http.MethodHead} {
		for _, resource := range []string{"/product-applets.json", "/missing-file.js", "/pudl/missing-file.css"} {
			r := httptest.NewRequest(method, "http://"+s.addr+resource, nil)
			r.AddCookie(&http.Cookie{Name: shellCookie, Value: s.token})
			w := httptest.NewRecorder()
			s.ServeHTTP(w, r)
			if w.Code != http.StatusNotFound {
				t.Fatalf("%s %s status %d", method, resource, w.Code)
			}
			if got := w.Result().Header.Get("Cache-Control"); got != "no-store" {
				t.Errorf("%s %s Cache-Control=%q", method, resource, got)
			}
		}
	}
	for _, rangeValue := range []string{"", "bytes=999999999-"} {
		r := httptest.NewRequest(http.MethodGet, "http://"+s.addr+"/app.js", nil)
		r.AddCookie(&http.Cookie{Name: shellCookie, Value: s.token})
		if rangeValue != "" {
			r.Header.Set("Range", rangeValue)
		}
		w := httptest.NewRecorder()
		s.ServeHTTP(w, r)
		want := http.StatusOK
		if rangeValue != "" {
			want = http.StatusRequestedRangeNotSatisfiable
		}
		if w.Code != want || w.Result().Header.Get("Cache-Control") != "no-store" {
			t.Fatalf("range %q status %d Cache-Control=%q", rangeValue, w.Code, w.Result().Header.Get("Cache-Control"))
		}
	}
}

func TestNormalShellStartupUsesFixedProfilePort(t *testing.T) {
	address := shellStartupAddress(false)
	host, port, err := net.SplitHostPort(address)
	if err != nil || host != "127.0.0.1" || port != strconv.Itoa(product.ShellPort) || port == "0" {
		t.Fatalf("normal startup address %q, profile port %d: %v", address, product.ShellPort, err)
	}
	if second := shellStartupAddress(false); second != address {
		t.Fatalf("normal startup changed origin: %s then %s", address, second)
	}
	if hidden := shellStartupAddress(true); hidden != "127.0.0.1:0" {
		t.Fatalf("hidden test startup could meet the live app: %s", hidden)
	}
}

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
