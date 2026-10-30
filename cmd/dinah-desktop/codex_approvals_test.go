package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"studio/internal/acp"
	"studio/internal/applet"
	"studio/internal/applet/broker"
	"studio/internal/codexagent"
	"studio/internal/ollamaagent"
)

func TestCodexAppletApprovalsPersistIndependently(t *testing.T) {
	a, c := providerTestApp(t)
	a.settings.Approvals = ApprovalsAll
	a.settings.CodexAppletApprovals = ""
	if err := saveSettings(a.settings); err != nil {
		t.Fatal(err)
	}
	if got := closeBusWhenDone(t, NewApp()).State().CodexAppletApprovals; got != "ask" {
		t.Fatalf("Devin approval migrated to Codex: %q", got)
	}
	ws := t.TempDir()
	info, err := a.NewSessionWithProviderAt(ws, "codex")
	if err != nil {
		t.Fatal(err)
	}
	if got := c.calls[0].params.(codexagent.SessionParams).AppletApprovals; got != "ask" {
		t.Fatalf("default approval %q", got)
	}
	if err := a.SetCodexAppletApprovals("allow"); err != nil {
		t.Fatal(err)
	}
	if closeBusWhenDone(t, NewApp()).State().CodexAppletApprovals != "allow" || a.settings.Approvals != ApprovalsAll {
		t.Fatal("independent preference did not persist")
	}
	count := len(c.calls)
	if _, err := a.OpenSessionAt(ws, info.ID); err != nil {
		t.Fatal(err)
	}
	if len(c.calls) != count {
		t.Fatal("changing preference reconfigured an existing thread")
	}
	a.runtime.RemoveConversationLocked(info.ID)
	if _, err := a.OpenSessionAt(ws, info.ID); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, call := range c.calls[count:] {
		if call.method == "session/load" {
			found = call.params.(codexagent.SessionParams).AppletApprovals == "allow"
		}
	}
	if !found {
		t.Fatal("native resume did not receive saved preference")
	}
	if _, ok := a.providerSessionParams("devin", "", ws, nil).(acp.NewSessionParams); !ok {
		t.Fatal("Codex preferences leaked into ACP parameters")
	}
}

func TestApproveEverythingPreferencesAndAppletBoundary(t *testing.T) {
	a, r, ctx := settingsAppletFixture(t)
	for _, provider := range []string{"codex", "ollama"} {
		t.Run(provider, func(t *testing.T) {
			var err error
			key := "codexAppletApprovals"
			if provider == "codex" {
				err = a.SetCodexAppletApprovals("all")
			} else {
				key = "ollamaApprovals"
				err = a.SetOllamaApprovals("all")
			}
			if err != nil {
				t.Fatal(err)
			}
			state := closeBusWhenDone(t, NewApp()).State()
			if provider == "codex" && state.CodexAppletApprovals != "all" || provider == "ollama" && state.OllamaApprovals != "all" {
				t.Fatal("Approve Everything did not persist")
			}
			params := a.providerSessionParams(provider, "", t.TempDir(), nil)
			if provider == "codex" {
				if params.(codexagent.SessionParams).AppletApprovals != "all" {
					t.Fatal("missing Codex policy")
				}
			} else {
				p := params.(ollamaagent.SessionParams)
				if p.AppletApprovals != "all" || p.CommandApprovals != "all" {
					t.Fatal("missing Ollama policy")
				}
			}
			before := readAppletSettingsTest(t, r, ctx)
			input, _ := json.Marshal(map[string]any{"revision": before.Revision, "changes": map[string]any{key: "ask"}})
			if _, err := r.Execute(ctx, "dinah.settings/update@1", string(input)); err == nil {
				t.Fatal("Settings applet changed approval preference")
			}
			result, err := r.Execute(ctx, "dinah.settings/read@1", `{}`)
			if err != nil || strings.Contains(result.OutputJSON, key) {
				t.Fatal("Settings applet exposed approval preference")
			}
			a.settings.Approvals = ApprovalsAsk
			a.runtime.PutConversationLocked(provider, newSessionState(&sessionSeed{ID: provider, Provider: provider}))
			consentCtx, cancel := context.WithCancel(context.Background())
			cancel()
			if err := a.appletConsent(consentCtx, broker.Invocation{OwnerKind: "chat", OwnerID: provider}, applet.Action{Capabilities: []string{"settings.write"}}); err != nil {
				t.Fatalf("bundled action still asked: %v", err)
			}
		})
	}
}

func TestCodexAppletApprovalsRejectInvalidSavedValue(t *testing.T) {
	a, _ := providerTestApp(t)
	a.settings.CodexAppletApprovals = "future-policy"
	if err := saveSettings(a.settings); err != nil {
		t.Fatal(err)
	}
	b := closeBusWhenDone(t, NewApp())
	if b.startupErr == nil {
		t.Fatal("invalid saved approval preference was silently accepted")
	}
	if err := b.SetCodexAppletApprovals("allow"); err == nil {
		t.Fatal("invalid settings were overwritten")
	}
	data, err := os.ReadFile(filepath.Join(configDir(), "settings.json"))
	if err != nil || !strings.Contains(string(data), "future-policy") {
		t.Fatal("invalid saved settings were changed")
	}
}

func TestCodexAppletApprovalSaveFailureAndSDKBoundary(t *testing.T) {
	a, r, ctx := settingsAppletFixture(t)
	if err := a.SetCodexAppletApprovals("ask"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(configDir(), "settings.json.pending"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := a.SetCodexAppletApprovals("allow"); err == nil {
		t.Fatal("failed save succeeded")
	}
	if a.State().CodexAppletApprovals != "ask" {
		t.Fatal("failed save changed effective preference")
	}
	if err := os.Remove(filepath.Join(configDir(), "settings.json.pending")); err != nil {
		t.Fatal(err)
	}
	if closeBusWhenDone(t, NewApp()).State().CodexAppletApprovals != "ask" {
		t.Fatal("failed save persisted preference")
	}
	if err := a.SetCodexAppletApprovals("never"); err == nil {
		t.Fatal("unknown preference accepted")
	}
	before := readAppletSettingsTest(t, r, ctx)
	input, _ := json.Marshal(map[string]any{"revision": before.Revision, "changes": map[string]any{"codexAppletApprovals": "allow"}})
	if _, err := r.Execute(ctx, "dinah.settings/update@1", string(input)); err == nil {
		t.Fatal("Settings applet changed security preference")
	}
	result, err := r.Execute(ctx, "dinah.settings/read@1", `{}`)
	if err != nil || strings.Contains(result.OutputJSON, "codexAppletApprovals") {
		t.Fatalf("security preference exposed: %s %v", result.OutputJSON, err)
	}
}
