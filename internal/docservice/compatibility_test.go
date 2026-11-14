package docservice

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"studio/internal/helpercompat"
)

func TestDocumentControlChecksProtocolBeforeOpening(t *testing.T) {
	h := start(t, t.TempDir(), true)
	for _, offer := range []string{"1", "2", ""} {
		req, _ := http.NewRequest(http.MethodGet, "http://"+h.s.addr+"/control/ping", nil)
		req.Header.Set("Authorization", "Bearer "+h.s.token)
		req.Header.Set(helpercompat.OfferHeader, offer)
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		want := 200
		if offer == "2" {
			want = 412
		}
		if res.StatusCode != want {
			t.Fatalf("offer %q: %d", offer, res.StatusCode)
		}
	}
	req, _ := http.NewRequest(http.MethodPost, "http://"+h.s.addr+"/control/open", strings.NewReader(`{"application":"editor"}`))
	req.Header.Set("Authorization", "Bearer "+h.s.token)
	req.Header.Set(helpercompat.OfferHeader, "2")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != 412 || len(h.s.Documents()) != 0 {
		t.Fatal("incompatible document request dispatched")
	}
}

func TestEnsureNeverReplacesAnIncompatibleDocumentService(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodOptions {
			t.Error("sent operation before compatible probe")
		}
		helpercompat.Policy{Channel: helpercompat.Documents, Numbers: []int{2}}.Accept(w, r)
	}))
	defer server.Close()
	dir := t.TempDir()
	if err := writeEndpoint(dir, Endpoint{Addr: strings.TrimPrefix(server.URL, "http://"), Token: "fixture"}); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(endpointFile(dir))
	if err != nil {
		t.Fatal(err)
	}
	started := false
	_, err = Ensure(dir, func() error { started = true; return nil })
	if !helpercompat.IsRefusal(err) || started {
		t.Fatalf("Ensure: %v, started %v", err, started)
	}
	after, err := os.ReadFile(endpointFile(dir))
	if err != nil || string(before) != string(after) {
		t.Fatal("incompatible endpoint changed")
	}
}
