package extensions

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	extension "github.com/paulmooreparks/dinah-extension-go"
	"studio/internal/helpercompat"
)

func TestExtensionControlAndCallbackProtocolGuards(t *testing.T) {
	m := New(t.TempDir(), Hooks{})
	defer m.Close()
	if err := m.serve(); err != nil {
		t.Fatal(err)
	}
	m.scopes["fixture"] = scope{ctx: context.Background(), workspace: extension.Workspace{Path: t.TempDir()}, permissions: []string{"workspace.read"}}
	for _, path := range []string{"/control", "/host"} {
		for _, offer := range []string{"1", "2", ""} {
			body := `{"tool":"host.workspaces","input":{}}`
			token := "fixture"
			if path == "/control" {
				body = `{"id":"fixture.extension","lease":"fixture-lease-123456","operation":"pause"}`
				token = m.controlToken
			}
			req, _ := http.NewRequest(http.MethodPost, "http://"+m.listener.Addr().String()+path, bytes.NewBufferString(body))
			req.Header.Set("Authorization", "Bearer "+token)
			req.Header.Set(helpercompat.OfferHeader, offer)
			res, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			res.Body.Close()
			want := 200
			if path == "/control" {
				want = 204
			}
			if offer == "2" {
				want = 412
			}
			if res.StatusCode != want {
				t.Fatalf("%s %q: %d", path, offer, res.StatusCode)
			}
		}
	}
}

func TestPackageMutationStopsBeforeIncompatibleHost(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodOptions {
			t.Error("incompatible control operation dispatched")
		}
		helpercompat.Policy{Channel: helpercompat.ExtensionControl, Numbers: []int{2}}.Accept(w, r)
	}))
	defer server.Close()
	root := t.TempDir()
	dir := filepath.Join(root, ".hosts")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(controlAddress{URL: server.URL + "/control", Token: "fixture-token-123456", PID: os.Getpid()})
	if err := os.WriteFile(filepath.Join(dir, "fixture.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	mutated := false
	err := WithPausedHosts(context.Background(), root, "fixture.extension", func() error { mutated = true; return nil })
	if !helpercompat.IsRefusal(err) || mutated {
		t.Fatalf("mutation: %v, changed %v", err, mutated)
	}
}
