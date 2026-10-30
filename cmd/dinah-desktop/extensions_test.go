package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	extension "github.com/paulmooreparks/dinah-extension-go"
)

func TestExtensionAssetsAreVerifiedAndSandboxed(t *testing.T) {
	a, _ := providerTestApp(t)
	folder := t.TempDir()
	os.Mkdir(filepath.Join(folder, "web"), 0700)
	manifest := extension.Manifest{ID: "test.assets", Version: "0.1.0", Contract: 1, Title: "Assets", Publisher: extension.Publisher{Name: "Test"}, Backends: map[string]string{"windows": "backend.exe"}, Permissions: []string{"workspace.read"}, Actions: []extension.Action{{Name: "inspect", Title: "Inspect", Replay: "pure", InputSchema: json.RawMessage(`{"type":"object"}`), OutputSchema: json.RawMessage(`{"type":"object"}`)}}, Views: []extension.View{{ID: "main", Title: "Main", Page: "web/index.html"}}}
	data, _ := json.Marshal(manifest)
	os.WriteFile(filepath.Join(folder, extension.ManifestFile), data, 0600)
	os.WriteFile(filepath.Join(folder, "backend.exe"), []byte("fixture"), 0600)
	os.WriteFile(filepath.Join(folder, "web/index.html"), []byte("<p>verified</p>"), 0600)
	_, key, _ := ed25519.GenerateKey(rand.Reader)
	manifest, err := extension.Sign(folder, key)
	if err != nil {
		t.Fatal(err)
	}
	manager := a.extensionManager()
	t.Cleanup(func() { manager.Close() })
	if _, err = manager.Store.Install(folder, "test", extension.Approval{Fingerprint: extension.Fingerprint(manifest.Publisher.Key), Signature: manifest.Signature}); err != nil {
		t.Fatal(err)
	}
	request := func(path string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "http://127.0.0.1/extension-assets/test.assets/0.1.0/"+path, nil)
		a.extensionAsset(w, r)
		return w
	}
	response := request("web/index.html")
	policy := response.Header().Get("Content-Security-Policy")
	if response.Code != 200 || response.Body.String() != "<p>verified</p>" || !strings.Contains(policy, "sandbox allow-scripts allow-forms;") || !strings.Contains(policy, "form-action 'none'") || response.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Fatalf("asset policy failed: %d %s", response.Code, policy)
	}
	if response = request("backend.exe"); response.Code != 404 {
		t.Fatal("backend executable was served")
	}
	os.WriteFile(filepath.Join(manager.Store.Path(manifest), "web/index.html"), []byte("changed"), 0600)
	if response = request("web/index.html"); response.Code != 409 {
		t.Fatal("tampered HTML was served")
	}
	if err = manager.Store.Enable(manifest.ID, false); err != nil {
		t.Fatal(err)
	}
	if response = request("web/index.html"); response.Code != 404 {
		t.Fatal("disabled extension asset was served")
	}
}

func TestPreparedTicketIsConsumedWhenLaunchFails(t *testing.T) {
	a, _ := providerTestApp(t)
	ticket := ""
	a.sink = func(name string, value any) {
		if name == "extension:terminal" {
			ticket = value.(map[string]any)["ticket"].(string)
		}
	}
	if err := a.extensionService().Hooks.Terminal(context.Background(), extension.TerminalInput{Program: "missing-extension-terminal-fixture"}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.StartPreparedExtensionTerminal(ticket, 80, 24); err == nil {
		t.Fatal("missing program launched")
	}
	if _, err := a.StartPreparedExtensionTerminal(ticket, 80, 24); err == nil || err.Error() != "this prepared command was already run or expired" {
		t.Fatalf("ticket survived failed launch: %v", err)
	}
}
