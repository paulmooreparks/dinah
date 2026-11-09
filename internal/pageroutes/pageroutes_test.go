package pageroutes

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRoutesAreFoundByPrefixAndBadOnesRefused(t *testing.T) {
	Register("/test-fonts/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("font " + r.URL.Path)) }))
	h, ok := Lookup("/test-fonts/a.woff2")
	if !ok {
		t.Fatal("the route was not found")
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/test-fonts/a.woff2", nil))
	if rec.Body.String() != "font /test-fonts/a.woff2" {
		t.Fatalf("answer: %q", rec.Body.String())
	}
	if _, ok := Lookup("/test-fontsx/a"); ok {
		t.Fatal("a path outside the prefix was matched")
	}
	for _, bad := range []string{"test", "/Test/", "/test", "//", "/a b/"} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("%q was accepted", bad)
				}
			}()
			Register(bad, http.NotFoundHandler())
		}()
	}
	func() {
		defer func() {
			if recover() == nil {
				t.Error("a repeated prefix was accepted")
			}
		}()
		Register("/test-fonts/", http.NotFoundHandler())
	}()
	if p := Prefixes(); len(p) == 0 || p[len(p)-1] != "/test-fonts/" {
		t.Fatalf("prefixes: %v", p)
	}
}
