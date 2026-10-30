package extensions

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	extension "github.com/paulmooreparks/dinah-extension-go"
	"studio/internal/applet"
)

func helperManifest() extension.Manifest {
	return extension.Manifest{ID: "test.native-extension", Version: "0.1.0", Contract: 1, Title: "Native fixture", Publisher: extension.Publisher{Name: "Test"}, Backends: map[string]string{runtime.GOOS: "backend.exe"}, Permissions: []string{"workspace.read", "process.run:git"}, Actions: []extension.Action{
		{Name: "inspect", Title: "Inspect", InputSchema: json.RawMessage(`{"type":"object","properties":{"z":{"type":"string"},"a":{"type":"boolean"}}}`), OutputSchema: json.RawMessage(`{"type":"object"}`), Permissions: []string{"workspace.read"}, Replay: "pure"},
		{Name: "version", Title: "Git version", InputSchema: json.RawMessage(`{"type":"object"}`), OutputSchema: json.RawMessage(`{"type":"object"}`), Permissions: []string{"process.run:git"}, Replay: "once"},
		{Name: "fail", Title: "Fail", InputSchema: json.RawMessage(`{"type":"object"}`), OutputSchema: json.RawMessage(`{"type":"object"}`), Replay: "pure"},
	}}
}

func TestMain(m *testing.M) {
	if os.Getenv("EXTENSION_TEST_HELPER") == "1" {
		err := extension.Serve(context.Background(), helperManifest(), map[string]extension.Handler{
			"fail": func(context.Context, *extension.Host, json.RawMessage) (any, error) {
				return nil, fmt.Errorf("controlled backend failure")
			},
			"inspect": func(ctx context.Context, h *extension.Host, input json.RawMessage) (any, error) {
				var result []extension.Workspace
				err := h.Call(ctx, "host.workspaces", map[string]any{}, &result)
				return map[string]any{"workspaces": result, "scope": h.Scope}, err
			},
			"version": func(ctx context.Context, h *extension.Host, input json.RawMessage) (any, error) {
				return h.Run(ctx, extension.RunInput{Program: "git", Args: []string{"--version"}, Cwd: h.Scope.Workspace.Path, Explanation: "Read the installed Git version."})
			},
		})
		if err != nil {
			os.Exit(2)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func TestSignedBackendScopeLifecycleAndIsolation(t *testing.T) {
	t.Setenv("EXTENSION_TEST_HELPER", "1")
	root := t.TempDir()
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	in, err := os.Open(self)
	if err != nil {
		t.Fatal(err)
	}
	out, err := os.OpenFile(filepath.Join(root, "backend.exe"), os.O_CREATE|os.O_WRONLY, 0700)
	if err != nil {
		t.Fatal(err)
	}
	_, err = io.Copy(out, in)
	in.Close()
	out.Close()
	if err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(helperManifest())
	if err = os.WriteFile(filepath.Join(root, extension.ManifestFile), data, 0600); err != nil {
		t.Fatal(err)
	}
	_, key, _ := ed25519.GenerateKey(rand.Reader)
	manifest, err := extension.Sign(root, key)
	if err != nil {
		t.Fatal(err)
	}
	ws := extension.Workspace{ID: "fixture", Path: t.TempDir(), Name: "Fixture"}
	m := New(t.TempDir(), Hooks{Workspace: func(id string) (extension.Workspace, error) { return ws, nil }})
	t.Cleanup(func() { m.Close() })
	_, err = m.Store.Install(root, "test", extension.Approval{Fingerprint: extension.Fingerprint(manifest.Publisher.Key), Signature: manifest.Signature})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if _, err = m.Call(ctx, manifest.ID, "fail", ws.ID, json.RawMessage(`{}`)); err == nil || !strings.Contains(err.Error(), "controlled backend failure") {
		t.Fatalf("tool failure was lost: %v", err)
	}
	for i := 0; i < 2; i++ {
		data, err = m.Call(ctx, manifest.ID, "inspect", ws.ID, json.RawMessage(`{}`))
		if err != nil {
			t.Fatal(err)
		}
		var reply struct {
			Scope      extension.Scope
			Workspaces []extension.Workspace
		}
		if err = json.Unmarshal(data, &reply); err != nil || len(reply.Workspaces) != 1 || reply.Workspaces[0].ID != ws.ID {
			t.Fatalf("bad response %s: %v", data, err)
		}
		// The scope is revoked before returning the action result.
		host := extension.Host{Scope: reply.Scope}
		if err = host.Call(ctx, "host.workspaces", map[string]any{}, nil); err == nil {
			t.Fatal("completed action retained host authority")
		}
	}
	// The SDK repeats the structured result as JSON text for older clients,
	// which is not text for a reader.
	if _, text, err := m.callSigned(ctx, manifest.ID, "inspect", ws.ID, json.RawMessage(`{}`), ""); err != nil || text != "" {
		t.Fatalf("the JSON copy of the result became its text: %q %v", text, err)
	}
	data, err = m.Call(ctx, manifest.ID, "version", ws.ID, json.RawMessage(`{}`))
	if err != nil || !strings.Contains(string(data), "git version") {
		t.Fatalf("real host command: %s %v", data, err)
	}
	oldSignature := manifest.Signature
	updated := helperManifest()
	updated.Version = "0.1.1"
	data, _ = json.Marshal(updated)
	os.WriteFile(filepath.Join(root, extension.ManifestFile), data, 0600)
	manifest, err = extension.Sign(root, key)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = m.Store.Install(root, "test", extension.Approval{Fingerprint: extension.Fingerprint(manifest.Publisher.Key), Signature: manifest.Signature}); err != nil {
		t.Fatal(err)
	}
	if _, _, err = m.callSigned(ctx, manifest.ID, "inspect", ws.ID, json.RawMessage(`{}`), oldSignature); err == nil {
		t.Fatal("old discovered action acquired a newer package's authority")
	}
	if _, err = m.Call(ctx, manifest.ID, "inspect", ws.ID, json.RawMessage(`{}`)); err != nil {
		t.Fatalf("new signed backend did not replace existing session: %v", err)
	}
	if err = os.WriteFile(filepath.Join(m.Store.Path(manifest), "backend.exe"), []byte("tampered"), 0700); err != nil {
		// Windows prevents replacement while running. Stop first, then corrupt it.
		m.Stop(manifest.ID)
		if err = os.WriteFile(filepath.Join(m.Store.Path(manifest), "backend.exe"), []byte("tampered"), 0700); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = m.Call(ctx, manifest.ID, "inspect", ws.ID, json.RawMessage(`{}`)); err == nil {
		t.Fatal("tampered backend ran")
	}
	bundles, err := m.Bundles()
	if err != nil || len(bundles) != 0 {
		t.Fatalf("damaged extension affected catalog: %v", err)
	}
}

// installFixture signs and installs a manifest whose backend never runs, so
// catalog and host-callback behaviour can be tested without a process.
func installFixture(t *testing.T, m *Manager, manifest extension.Manifest) extension.Manifest {
	t.Helper()
	root := t.TempDir()
	data, _ := json.Marshal(manifest)
	if err := os.WriteFile(filepath.Join(root, extension.ManifestFile), data, 0600); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(root, "backend.exe"), []byte("never run"), 0700)
	_, key, _ := ed25519.GenerateKey(rand.Reader)
	signed, err := extension.Sign(root, key)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = m.Store.Install(root, "test", extension.Approval{Fingerprint: extension.Fingerprint(signed.Publisher.Key), Signature: signed.Signature}); err != nil {
		t.Fatal(err)
	}
	return signed
}

func TestEffectLevelsReachConsentAndCommandReview(t *testing.T) {
	manifest := helperManifest()
	manifest.Actions = append(manifest.Actions,
		extension.Action{Name: "stage", Title: "Stage", InputSchema: json.RawMessage(`{"type":"object"}`), OutputSchema: json.RawMessage(`{"type":"object"}`), Permissions: []string{"process.run:git"}, Replay: "once", Effect: extension.EffectLocal},
		extension.Action{Name: "push", Title: "Push", InputSchema: json.RawMessage(`{"type":"object"}`), OutputSchema: json.RawMessage(`{"type":"object"}`), Permissions: []string{"process.run:git"}, Replay: "once", Effect: extension.EffectNetwork})
	reviewed := 0
	m := New(t.TempDir(), Hooks{Review: func(ctx context.Context, id string, in extension.RunInput) ([]string, error) {
		reviewed++
		return in.Args, nil
	}})
	defer m.Close()
	signed := installFixture(t, m, manifest)
	bundles, err := m.Bundles()
	if err != nil || len(bundles) != 1 {
		t.Fatalf("bundles: %v %d", err, len(bundles))
	}
	asks := map[string]bool{}
	for _, action := range bundles[0].Manifest.Actions {
		asks[action.Ref] = applet.Asks(action)
		if action.Ref == signed.ID+"/stage@1" && action.Effect != extension.EffectLocal {
			t.Fatalf("effect level lost on the applet action: %q", action.Effect)
		}
	}
	// A process action with no level keeps asking; a local one does not; a
	// network one does. A pure read without process permission never did.
	if !asks[signed.ID+"/version@1"] || asks[signed.ID+"/stage@1"] || !asks[signed.ID+"/push@1"] || asks[signed.ID+"/inspect@1"] {
		t.Fatalf("consent does not follow effect levels: %v", asks)
	}
	if err = m.Store.Review(signed.ID, true); err != nil {
		t.Fatal(err)
	}
	ws := extension.Workspace{ID: "fixture", Path: t.TempDir()}
	run := func(effect string) error {
		s := scope{workspace: ws, extensionID: signed.ID, permissions: []string{"process.run:git"}, effect: effect}
		raw, _ := json.Marshal(extension.RunInput{Program: "git", Args: []string{"--version"}, Cwd: ws.Path})
		_, err := m.hostCall(context.Background(), s, "host.run", raw)
		return err
	}
	for i, effect := range []string{extension.EffectRead, extension.EffectLocal} {
		if err = run(effect); err != nil {
			t.Fatal(err)
		}
		if reviewed != 0 {
			t.Fatalf("Show Me First reviewed a %s command (%d)", effect, i)
		}
	}
	for i, effect := range []string{"", extension.EffectHistory, extension.EffectNetwork} {
		if err = run(effect); err != nil {
			t.Fatal(err)
		}
		if reviewed != i+1 {
			t.Fatalf("Show Me First did not review a %q command", effect)
		}
	}
}

func TestHostFilePermissionsAndRoot(t *testing.T) {
	m := New(t.TempDir(), Hooks{})
	defer m.Close()
	ws := extension.Workspace{ID: "fixture", Path: t.TempDir()}
	s := scope{workspace: ws, extensionID: "test.native-extension", permissions: []string{"workspace.read", "workspace.write"}}
	invoke := func(tool string, input any) (any, error) {
		raw, _ := json.Marshal(input)
		return m.hostCall(context.Background(), s, tool, raw)
	}
	created, err := invoke("host.write", extension.FileInput{Path: "note.txt", Text: "first"})
	if err != nil {
		t.Fatal(err)
	}
	first := created.(extension.FileResult)
	if _, err = invoke("host.write", extension.FileInput{Path: "note.txt", Text: "overwrite"}); err == nil {
		t.Fatal("overwrote without revision")
	}
	if _, err = invoke("host.write", extension.FileInput{Path: "note.txt", Text: "second", ExpectedSHA256: first.SHA256}); err != nil {
		t.Fatal(err)
	}
	if _, err = invoke("host.write", extension.FileInput{Path: "note.txt", Text: "stale", ExpectedSHA256: first.SHA256}); err == nil {
		t.Fatal("stale overwrite")
	}
	outside := t.TempDir()
	os.WriteFile(filepath.Join(outside, "secret"), []byte("private"), 0600)
	if _, err = invoke("host.read", extension.FileInput{Path: filepath.Join(outside, "secret")}); err == nil {
		t.Fatal("read outside workspace")
	}
	if err = os.Symlink(outside, filepath.Join(ws.Path, "escape")); err == nil {
		if _, err = invoke("host.read", extension.FileInput{Path: "escape/secret"}); err == nil {
			t.Fatal("followed escaping link")
		}
	}
	s.permissions = []string{"workspace.read"}
	if _, err = invoke("host.write", extension.FileInput{Path: "new", Text: "no"}); err == nil {
		t.Fatal("write without permission")
	}
	if _, err = invoke("host.run", extension.RunInput{Program: "git", Args: []string{"--version"}, Cwd: ws.Path}); err == nil {
		t.Fatal("process without permission")
	}
}

func TestCallbackRejectsBrowserAndExpiredScopes(t *testing.T) {
	m := New(t.TempDir(), Hooks{})
	defer m.Close()
	if err := m.serve(); err != nil {
		t.Fatal(err)
	}
	m.scopes["fixture"] = scope{ctx: context.Background(), workspace: extension.Workspace{Path: t.TempDir()}, permissions: []string{"workspace.read"}}
	for _, origin := range []string{"null", "http://127.0.0.1"} {
		req, _ := http.NewRequest("POST", m.url, bytes.NewBufferString(`{"tool":"host.workspaces","input":{}}`))
		req.Header.Set("Origin", origin)
		req.Header.Set("Authorization", "Bearer fixture")
		response, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != 403 {
			t.Fatalf("browser callback accepted: %d", response.StatusCode)
		}
	}
}
func TestSchemaComparisonKeepsIntegerPrecision(t *testing.T) {
	if canonicalJSON(json.RawMessage(`{"const":9007199254740993}`)) == canonicalJSON(json.RawMessage(`{"const":9007199254740992}`)) {
		t.Fatal("schema constants lost integer precision")
	}
	if canonicalJSON(json.RawMessage(`{"type":"object","title":"x"}`)) != canonicalJSON(json.RawMessage(`{"title":"x","type":"object"}`)) {
		t.Fatal("schema comparison depends on property order")
	}
}
func TestControlLeasesSurviveCancellationAndOverlap(t *testing.T) {
	m := New(t.TempDir(), Hooks{})
	defer m.Close()
	if err := m.serve(); err != nil {
		t.Fatal(err)
	}
	id := "test.native-extension"
	m.Pause(id, "native-lease-one-123")
	m.Pause(id, "native-lease-two-123")
	m.Resume(id, "native-lease-one-123")
	if len(m.pauses[id]) != 1 {
		t.Fatal("one mutator released another mutator's lease")
	}
	ctx, cancel := context.WithCancel(context.Background())
	err := WithPausedHosts(ctx, m.Store.Root, id, func() error {
		m.mu.Lock()
		count := len(m.pauses[id])
		m.mu.Unlock()
		if count != 2 {
			t.Errorf("CLI did not pause the live host: %d", count)
		}
		cancel()
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	m.mu.Lock()
	count := len(m.pauses[id])
	m.mu.Unlock()
	if count != 1 {
		t.Fatal("cancelled CLI operation left its lease armed")
	}
	m.Resume(id, "native-lease-two-123")
	for _, origin := range []string{"null", "https://other.invalid"} {
		data, _ := json.Marshal(controlRequest{ID: id, Lease: "browser-lease-1234", Operation: "pause"})
		req, _ := http.NewRequest("POST", "http://"+m.listener.Addr().String()+"/control", bytes.NewReader(data))
		req.Header.Set("Origin", origin)
		req.Header.Set("Authorization", "Bearer "+m.controlToken)
		response, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != 403 {
			t.Fatal("browser reached privileged host control")
		}
	}
}

// A tool's text content is the result's readable text, apart from the copy
// of the structured result that MCP asks a tool to send as JSON text.
func TestToolTextIsCarriedApartFromItsJSON(t *testing.T) {
	structured := json.RawMessage(`{"branch":"main","ahead":2}`)
	result := &mcp.CallToolResult{Content: []mcp.Content{
		&mcp.TextContent{Text: "On **main**, two commits ahead."},
		&mcp.TextContent{Text: `{"ahead": 2, "branch": "main"}`},
		&mcp.ImageContent{Data: []byte("x"), MIMEType: "image/png"},
		&mcp.TextContent{Text: "Push when ready."},
	}, StructuredContent: structured}
	if got := resultText(result, structured); got != "On **main**, two commits ahead.\n\nPush when ready." {
		t.Fatalf("text = %q", got)
	}
	if got := resultText(&mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(structured)}}}, structured); got != "" {
		t.Fatalf("the JSON copy became text: %q", got)
	}
}
