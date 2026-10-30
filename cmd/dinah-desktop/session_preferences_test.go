package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"studio/internal/acp"
)

func TestExistingSessionRestoresEffectiveModelAfterDefaultsChange(t *testing.T) {
	a, _ := providerTestApp(t)
	ws := t.TempDir()
	info, err := a.NewSessionAt(ws)
	if err != nil {
		t.Fatal(err)
	}
	// The provider's original model must be saved even without an explicit
	// model selection in this chat.
	if got := string(a.settings.SessionOptions[info.ID]["model"]); got != `"initial"` {
		t.Fatalf("effective initial model was not saved: %s", got)
	}
	if err := a.SetDefaultProvider("devin"); err != nil {
		t.Fatal(err)
	}
	a.settings.ProviderDefaults = map[string]map[string]json.RawMessage{"codex": {"model": json.RawMessage(`"codex-default"`)}}
	if err := saveSettings(a.settings); err != nil {
		t.Fatal(err)
	}
	restarted := NewApp()
	t.Cleanup(func() { restarted.busHost().CloseHandle() })
	t.Cleanup(restarted.closeAppletBridge)
	c := newTestProviderConnection()
	c.options[0].CurrentValue = json.RawMessage(`"changed"`)
	restarted.runtime.PutProviderLocked("codex", newTestProvider(c, acp.InitializeResult{AgentCapabilities: acp.AgentCapabilities{LoadSession: true}}))
	reopened, err := restarted.OpenSessionAt(ws, info.ID)
	if err != nil {
		t.Fatal(err)
	}
	if reopened.Provider != "codex" || restarted.runtime.ProviderLocked("devin") != nil {
		t.Fatalf("existing chat used the changed default provider: %+v", reopened)
	}
	if got := string(restarted.SessionOptions(info.ID)[0].CurrentValue); got != `"initial"` {
		t.Fatalf("existing chat used a new default model: %s", got)
	}
	if got := restarted.settings.SessionProviders[info.ID]; got != "codex" {
		t.Fatalf("saved provider %q", got)
	}
}

func TestSessionProviderLegacyAndUnavailableChoices(t *testing.T) {
	a, _ := providerTestApp(t)
	if got, err := a.savedSessionProvider("legacy-devin-id"); err != nil || got != "devin" {
		t.Fatalf("legacy session followed Codex default: %q %v", got, err)
	}
	s := newSessionState(&sessionSeed{ID: "legacy-devin-id", Provider: "devin"}, sessionInitial{Open: true})
	if err := a.setOptions(s, newTestProviderConnection().options); err != nil {
		t.Fatal(err)
	}
	restarted := NewApp()
	t.Cleanup(func() { restarted.busHost().CloseHandle() })
	if got, err := restarted.savedSessionProvider(s.ID()); err != nil || got != "devin" {
		t.Fatalf("saved Devin session followed Codex default: %q %v", got, err)
	}
	restarted.settings.SessionProviders[s.ID()] = "unavailable-provider"
	if _, err := restarted.OpenSessionAt(t.TempDir(), s.ID()); err == nil || !strings.Contains(err.Error(), "unavailable") {
		t.Fatalf("unavailable provider fell back: %v", err)
	}
}

func TestUnavailableSavedModelDoesNotFallBackOrReplacePreferences(t *testing.T) {
	a, c := providerTestApp(t)
	ws := t.TempDir()
	id := "codex:existing"
	a.settings.SessionProviders = map[string]string{id: "codex"}
	a.settings.SessionOptions = map[string]map[string]json.RawMessage{id: {"model": json.RawMessage(`"removed-model"`)}}
	if err := saveSettings(a.settings); err != nil {
		t.Fatal(err)
	}
	if _, err := a.OpenSessionAt(ws, id); err == nil || !strings.Contains(err.Error(), "no longer available") {
		t.Fatalf("removed model silently fell back: %v", err)
	}
	if a.session(id) != nil {
		t.Fatal("failed restoration left a sendable session")
	}
	if got := string(NewApp().settings.SessionOptions[id]["model"]); got != `"removed-model"` {
		t.Fatalf("failed restoration overwrote saved choice: %s", got)
	}
	for _, call := range c.calls {
		if call.method == "session/prompt" {
			t.Fatal("restoration sent a prompt")
		}
	}
}

func TestSessionOptionPersistenceFailureIsVisibleAndCopyOnWrite(t *testing.T) {
	a, c := providerTestApp(t)
	s := newSessionState(&sessionSeed{ID: "codex:existing", Provider: "codex"}, sessionInitial{Open: true})
	if err := a.setOptions(s, c.options); err != nil {
		t.Fatal(err)
	}
	before := a.settings.SessionOptions[s.ID()]
	blocked := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocked, []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	previous := configOverride
	configOverride = blocked
	defer func() { configOverride = previous }()
	visible := false
	a.sink = func(name string, data any) {
		if name == "agent:options" {
			visible = data.(map[string]any)["error"] != nil
		}
	}
	options := append([]acp.ConfigOption(nil), c.options...)
	options[0].CurrentValue = json.RawMessage(`"changed"`)
	if err := a.setOptions(s, options); err == nil || !visible {
		t.Fatalf("save failure was hidden: %v visible=%v", err, visible)
	}
	if string(before["model"]) != `"initial"` || string(a.settings.SessionOptions[s.ID()]["model"]) != `"initial"` {
		t.Fatal("failed save mutated prior preferences")
	}
}

func TestSessionRestoreBlocksEarlySendAndPreservesHiddenPreferences(t *testing.T) {
	a, c := providerTestApp(t)
	info, err := a.NewSessionAt(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	s := a.session(info.ID)
	s.SetRestoring(true)
	// A message sent while the settings are restored is held, and no turn
	// starts for it until the restoration has ended.
	if err := a.Send(info.ID, "must not run yet", nil, nil); err != nil {
		t.Fatalf("a message sent while settings were restored was refused: %v", err)
	}
	a.sink = func(name string, data any) {
		if name == "agent:options" {
			if err := a.Send(info.ID, "still restoring", nil, nil); err != nil {
				t.Errorf("a message sent during restoration was refused: %v", err)
			}
		}
	}
	if err := a.optionService().Finish(s, a.SessionOptions(info.ID)); err != nil {
		t.Fatal(err)
	}
	a.sink = nil
	if countCalls(c, "session/prompt") != 0 || len(s.Info().Queued) != 2 {
		t.Fatalf("a held message ran before the settings were restored: %d prompts, queue %v", countCalls(c, "session/prompt"), s.Info().Queued)
	}
	s.SetRestoring(false)
	if err := a.HideSession(info.ID); err != nil {
		t.Fatal(err)
	}
	if a.settings.SessionProviders[info.ID] != "codex" || string(a.settings.SessionOptions[info.ID]["model"]) != `"initial"` {
		t.Fatal("hiding the chat discarded its saved provider/model")
	}
}
