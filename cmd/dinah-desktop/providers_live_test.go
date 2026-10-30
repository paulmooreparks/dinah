//go:build live

package main

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"studio/internal/acp"
	"studio/internal/codexagent"
)

type providerLiveTrace struct {
	sync.Mutex
	text strings.Builder
}

// This connection check sends no model prompt and leaves account management
// with Codex. It exercises an explicit executable without a discovery PATH.
func TestLiveCodexExecutableOverride(t *testing.T) {
	executable := os.Getenv("DINAH_CODEX_EXECUTABLE")
	if executable == "" {
		t.Skip("set DINAH_CODEX_EXECUTABLE for a read-only installed runtime check")
	}
	previous := configOverride
	configOverride = t.TempDir()
	t.Cleanup(func() { configOverride = previous })
	t.Setenv("PATH", "")
	a := NewApp()
	t.Cleanup(func() { a.lifecycle().StopAll() })
	if err := a.SetAgentExecutable("codex", executable); err != nil {
		t.Fatal(err)
	}
	info := a.AgentCheck("codex")
	if info.Status != "connected" || info.ExecutableOverride != executable || len(info.Catalog) == 0 {
		t.Fatalf("explicit executable connection failed: %s %s", info.Status, info.Detail)
	}
	t.Log("selected Codex executable connected and returned models with an empty discovery PATH; no prompt sent")
}

func (w *providerLiveTrace) Write(b []byte) (int, error) {
	w.Lock()
	defer w.Unlock()
	return w.text.Write(b)
}

func (w *providerLiveTrace) String() string {
	w.Lock()
	defer w.Unlock()
	return w.text.String()
}

// This opt-in smoke test uses the installed Codex and its managed login. It
// sends two short turns, consumes the selected account's allowance, and keeps
// all Dinah state, flow files and artifacts in throwaway directories.
func TestLiveCodexAppletAndResume(t *testing.T) {
	if os.Getenv("DINAH_LIVE_CODEX") != "1" {
		t.Skip("set DINAH_LIVE_CODEX=1 to send live Codex turns")
	}
	previous := configOverride
	configOverride = t.TempDir()
	t.Cleanup(func() { configOverride = previous })
	packagedAppletHelper(t)
	a := NewApp()
	a.settings.DefaultProvider = "codex"
	ws := t.TempDir()
	a.settings.Workspace = ws
	recorder := record(a)
	sink := a.sink
	a.sink = func(name string, data any) {
		// The shared Devin recorder answers permissions with a Devin option ID.
		// Codex choices must be answered using the IDs in this request.
		if name != "agent:permission" {
			sink(name, data)
		}
		if name == "agent:permission" {
			question := data.(map[string]any)
			var tool struct {
				Title string `json:"title"`
			}
			_ = json.Unmarshal(question["toolCall"].(json.RawMessage), &tool)
			choice := ""
			for _, option := range question["options"].([]acp.PermissionOption) {
				if strings.Contains(tool.Title, "applet_") || strings.Contains(tool.Title, "dinah-applets") {
					if option.Name == "Accept" || option.Name == "Allow Once" || option.Name == "Yes" {
						choice = option.OptionID
						break
					}
				}
				if option.Kind == "reject_once" {
					choice = option.OptionID
				}
			}
			t.Logf("Codex permission %q: selected %q from %v", tool.Title, choice, question["options"])
			a.Answer(question["key"].(string), choice)
		}
	}
	t.Cleanup(func() { a.lifecycle().StopAll(); a.closeAppletBridge() })
	// Retain method-only protocol diagnostics if the installed runtime rejects
	// a request before it reaches the host's permission UI.
	var executable string
	for _, provider := range a.AgentProviders() {
		if provider.ID == "codex" {
			executable = provider.Path
		}
	}
	trace := &providerLiveTrace{}
	conn, err := codexagent.Start(executable, []string{"app-server"}, configDir(), a.newAgentHandler(), trace)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(conn.Close)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	var init acp.InitializeResult
	if err := conn.Call(ctx, "initialize", acp.InitializeParams{ProtocolVersion: 1, ClientCapabilities: acp.ClientCaps(), ClientInfo: &acp.Implementation{Name: "dinah", Title: "Dinah", Version: appVersion}}, &init); err != nil {
		t.Fatal(err)
	}
	p := newTestProvider(conn, init)
	a.runtime.PutProviderLocked("codex", p)
	go a.lifecycle().Watch("codex", p)
	account := a.AgentCheck("codex")
	if account.Status != "connected" {
		t.Fatalf("Codex connection is not ready: %s", account.Detail)
	}
	withTestApplets(t, a)
	info, err := a.NewSessionWithProviderAt(ws, "codex")
	if err != nil {
		t.Fatal(err)
	}
	s := a.session(info.ID)
	if _, end := liveAsk(t, a, s, "Picture", livePicturePrompt); end.err != nil || end.stopReason != "end_turn" {
		t.Fatalf("turn failed: %s %v", end.stopReason, end.err)
	}
	invocations, err := a.AppletInvocationsAt(ws)
	if err != nil {
		t.Fatal(err)
	}
	pictureID := ""
	for _, v := range invocations {
		if v.OwnerID == s.ID() && v.Action == testPictureRef && v.Status == "done" && len(v.Artifacts) == 1 {
			pictureID = v.ID
		}
	}
	if pictureID == "" {
		recorder.mu.Lock()
		response := recorder.said.String()
		recorder.mu.Unlock()
		t.Fatalf("Codex did not draw a picture through the managed applet: %s\nProtocol methods:\n%s", response, trace.String())
	}
	if a.runtime.ProviderLocked("devin") != nil {
		t.Fatal("the Codex chat started Devin")
	}
	a.AgentStop("codex")
	if _, err := a.OpenSessionAt(ws, s.ID()); err != nil {
		t.Fatal(err)
	}
	reopened := a.session(s.ID())
	words, end := liveAsk(t, a, reopened, "Resume", "Reply with only the picture invocation ID from this conversation. Do not invoke any tools or draw another picture.")
	if end.err != nil || !strings.Contains(words, pictureID) {
		t.Fatalf("resumed session did not retain the picture result: %v", end.err)
	}
	after, err := a.AppletInvocationsAt(ws)
	if err != nil || len(after) != len(invocations) {
		t.Fatal("resume duplicated an applet action")
	}
	t.Log("Codex, the managed picture applet and a resumed chat passed without Devin")
}
