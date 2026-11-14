package extensions

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	extension "github.com/paulmooreparks/dinah-extension-go"
	"studio/internal/helpercompat"
)

type callbackProbeBody struct{ read bool }

func (b *callbackProbeBody) Read([]byte) (int, error) {
	b.read = true
	return 0, io.EOF
}

func TestHostOptionsAuthenticatesAndNegotiatesWithoutReadingACallback(t *testing.T) {
	m := New(t.TempDir(), Hooks{})
	defer m.Close()
	if err := m.serve(); err != nil {
		t.Fatal(err)
	}
	expired, cancel := context.WithCancel(context.Background())
	cancel()
	m.mu.Lock()
	m.scopes["fixture"] = scope{ctx: context.Background()}
	m.scopes["expired"] = scope{ctx: expired}
	m.mu.Unlock()
	for _, tc := range []struct {
		name, token, offer, method, host, origin string
		status                                   int
	}{
		{name: "equal", token: "fixture", offer: "1", status: 204},
		{name: "legacy", token: "fixture", status: 204},
		{name: "incompatible", token: "fixture", offer: "2", status: 412},
		{name: "missing-token", offer: "1", status: 403},
		{name: "unknown-token", token: "unknown", offer: "1", status: 403},
		{name: "expired", token: "expired", offer: "1", status: 403},
		{name: "wrong-host", token: "fixture", offer: "1", host: "other.invalid", status: 403},
		{name: "browser", token: "fixture", offer: "1", origin: "https://other.invalid", status: 403},
		{name: "get", token: "fixture", offer: "1", method: http.MethodGet, status: 403},
	} {
		t.Run(tc.name, func(t *testing.T) {
			method := tc.method
			if method == "" {
				method = http.MethodOptions
			}
			body := &callbackProbeBody{}
			r := httptest.NewRequest(method, "http://"+m.listener.Addr().String()+"/host", body)
			if tc.host != "" {
				r.Host = tc.host
			}
			if tc.token != "" {
				r.Header.Set("Authorization", "Bearer "+tc.token)
			}
			r.Header.Set(helpercompat.OfferHeader, tc.offer)
			r.Header.Set("Origin", tc.origin)
			w := httptest.NewRecorder()
			m.hostHTTP(w, r)
			if w.Code != tc.status || body.read {
				t.Fatalf("status %d, body read %v", w.Code, body.read)
			}
			if tc.status == 204 {
				if w.Header().Get(helpercompat.SelectedHeader) != "1" || w.Header().Get(helpercompat.OfferHeader) != "1" || w.Body.Len() != 0 {
					t.Fatalf("probe response: headers %v, body %s", w.Header(), w.Body.String())
				}
			} else if tc.status == 403 && w.Header().Get(helpercompat.SelectedHeader) != "" {
				t.Fatal("unauthorized request negotiated a protocol")
			}
		})
	}
}

func TestHostOptionsWorksWithTheSDKCallbackClient(t *testing.T) {
	m := New(t.TempDir(), Hooks{})
	defer m.Close()
	if err := m.serve(); err != nil {
		t.Fatal(err)
	}
	m.mu.Lock()
	m.scopes["fixture"] = scope{ctx: context.Background(), workspace: extension.Workspace{ID: "fixture-workspace", Path: t.TempDir()}, permissions: []string{"workspace.read"}}
	m.mu.Unlock()
	client := &extension.Host{Scope: extension.Scope{URL: m.url, Token: "fixture"}}
	var workspaces []extension.Workspace
	if err := client.Call(context.Background(), "host.workspaces", struct{}{}, &workspaces); err != nil {
		t.Fatal(err)
	}
	if len(workspaces) != 1 || workspaces[0].ID != "fixture-workspace" {
		t.Fatalf("SDK callback result: %+v", workspaces)
	}
}
