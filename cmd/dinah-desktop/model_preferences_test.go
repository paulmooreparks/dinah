package main

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"studio/internal/acp"
	"studio/internal/agents"
)

func modelPreferenceFixture(t *testing.T) *App {
	t.Helper()
	a, _ := providerTestApp(t)
	a.settings.DefaultProvider = "devin"
	a.settings.Catalog = []acp.ConfigOption{{ID: "devin-model", Category: "model", Type: "select", Options: json.RawMessage(`[{"value":"devin-only","name":"Devin Only"}]`)}, {ID: "thinking", Type: "boolean"}}
	a.settings.ProviderCatalogs = map[string][]acp.ConfigOption{"codex": {{ID: "model", Category: "model", Type: "select", Options: json.RawMessage(`[{"name":"Models","options":[{"value":"codex-only","name":"Codex Only"}]}]`)}, {ID: "effort", Type: "select", Options: json.RawMessage(`[{"value":"high","name":"High"}]`)}}}
	return a
}

func TestModelPreferencesProviderIsolationAndCopies(t *testing.T) {
	a := modelPreferenceFixture(t)
	if _, err := a.SetDefaultForProvider("codex", "model", json.RawMessage(`"codex-only"`)); err != nil {
		t.Fatal(err)
	}
	if _, err := a.SetPinnedForProvider("codex", "codex-only", true); err != nil {
		t.Fatal(err)
	}
	if _, err := a.SetDefaultForProvider("devin", "thinking", json.RawMessage(`false`)); err != nil {
		t.Fatal(err)
	}
	if _, err := a.SetDefaultForProvider("devin", "devin-model", json.RawMessage(`"devin-only"`)); err != nil {
		t.Fatal(err)
	}
	if _, err := a.SetPinnedForProvider("devin", "devin-only", true); err != nil {
		t.Fatal(err)
	}
	if a.defaultProvider() != "devin" {
		t.Fatal("editing Codex preferences changed the default agent")
	}
	restarted := NewApp()
	for provider, model := range map[string]string{"devin": "devin-only", "codex": "codex-only"} {
		prefs, err := restarted.ProviderModelPreferences(provider)
		if err != nil {
			t.Fatal(err)
		}
		if len(prefs.Pinned) != 1 || prefs.Pinned[0] != model || len(prefs.Catalog) != 2 {
			t.Fatalf("provider preferences mixed: %+v", prefs)
		}
		key := "model"
		if provider == "devin" {
			key = "devin-model"
		}
		if string(prefs.Defaults[key]) != `"`+model+`"` {
			t.Fatalf("default did not persist: %+v", prefs)
		}
		prefs.Pinned[0] = "mutated"
		prefs.Catalog[0].Options[0] = '!'
		prefs.Defaults[key][0] = '!'
		again, _ := restarted.ProviderModelPreferences(provider)
		if again.Pinned[0] != model || !json.Valid(again.Catalog[0].Options) || !json.Valid(again.Defaults[key]) {
			t.Fatal("returned state shared storage with settings")
		}
	}
}

// The page reads a provider's saved model list for a new chat before any chat of
// that provider exists. With Claude as the default and Ollama's list saved from
// an earlier chat, ProviderModelPreferences("ollama") returns Ollama's list,
// defaults and favorites, and it connects to nothing and leaves the default
// where it was. The window once showed no Model control for Ollama in this
// state, because the page was given only the default provider's list.
func TestModelPreferencesServeASavedOllamaListWithoutConnecting(t *testing.T) {
	a, _ := providerTestApp(t)
	a.settings.DefaultProvider = "claude"
	a.settings.ProviderCatalogs = map[string][]acp.ConfigOption{
		"claude": {{ID: "model", Category: "model", Type: "select", Options: json.RawMessage(`[{"value":"claude-only","name":"Claude Only"}]`)}},
		"ollama": {{ID: "model", Category: "model", Type: "select", Options: json.RawMessage(`[{"value":"deepseek-v4.1-flash:cloud","name":"deepseek-v4.1-flash:cloud"},{"value":"glm-5.1:cloud","name":"glm-5.1:cloud"}]`)}, {ID: "think", Type: "boolean"}},
	}
	a.settings.ProviderDefaults = map[string]map[string]json.RawMessage{"ollama": {"model": json.RawMessage(`"deepseek-v4.1-flash:cloud"`)}}
	a.settings.ProviderPinned = map[string][]string{"ollama": {"glm-5.1:cloud"}}
	states := len(a.ProviderStates())

	prefs, err := a.ProviderModelPreferences("ollama")
	if err != nil {
		t.Fatal(err)
	}
	if prefs.Provider != "ollama" || len(prefs.Catalog) != 2 || prefs.Catalog[0].ID != "model" || !strings.Contains(string(prefs.Catalog[0].Options), "glm-5.1:cloud") {
		t.Fatalf("Ollama's saved list was not served: %+v", prefs)
	}
	if string(prefs.Defaults["model"]) != `"deepseek-v4.1-flash:cloud"` || len(prefs.Pinned) != 1 || prefs.Pinned[0] != "glm-5.1:cloud" {
		t.Fatalf("Ollama's defaults and favorites were not served: %+v", prefs)
	}
	if strings.Contains(string(prefs.Catalog[0].Options), "claude-only") {
		t.Fatal("Ollama was given Claude's list")
	}
	if a.defaultProvider() != "claude" {
		t.Fatalf("reading Ollama's preferences moved the default to %q", a.defaultProvider())
	}
	// Nothing was started or reported: no provider process, no state.
	if a.runtime.ProviderLocked("ollama") != nil || len(a.ProviderStates()) != states {
		t.Fatal("reading preferences connected to Ollama")
	}
	// A provider with nothing saved gets an empty list, not another's.
	delete(a.settings.ProviderCatalogs, "ollama")
	if prefs, err := a.ProviderModelPreferences("ollama"); err != nil || len(prefs.Catalog) != 0 {
		t.Fatalf("with nothing saved: %+v %v", prefs, err)
	}
}

func TestModelPreferencesValidateSelectedCatalogAndAllowReset(t *testing.T) {
	a := modelPreferenceFixture(t)
	for _, provider := range []string{"", "gemini", "unknown"} {
		if _, err := a.ProviderModelPreferences(provider); err == nil {
			t.Fatalf("unsupported provider read: %q", provider)
		}
		if _, err := a.SetDefaultForProvider(provider, "model", nil); err == nil {
			t.Fatalf("unsupported provider write: %q", provider)
		}
		if _, err := a.SetPinnedForProvider(provider, "model", false); err == nil {
			t.Fatalf("unsupported provider pin: %q", provider)
		}
	}
	for _, value := range []json.RawMessage{json.RawMessage(`"devin-only"`), json.RawMessage(`true`), json.RawMessage(`{`)} {
		if _, err := a.SetDefaultForProvider("codex", "model", value); err == nil {
			t.Fatalf("invalid model accepted: %s", value)
		}
	}
	if _, err := a.SetPinnedForProvider("codex", "high", true); err == nil {
		t.Fatal("non-model option pinned as model")
	}
	if _, err := a.SetPinnedForProvider("codex", "devin-only", true); err == nil {
		t.Fatal("other provider model pinned")
	}
	a.settings.ProviderCatalogs["codex"] = nil
	a.settings.ProviderDefaults = map[string]map[string]json.RawMessage{"codex": {"removed-option": json.RawMessage(`"old-model"`)}}
	a.settings.ProviderPinned = map[string][]string{"codex": {"old-model"}}
	prefs, err := a.ProviderModelPreferences("codex")
	if err != nil || prefs.Catalog == nil || len(prefs.Catalog) != 0 || len(prefs.Defaults) != 1 || len(prefs.Pinned) != 1 {
		t.Fatalf("missing catalog discarded preferences: %+v %v", prefs, err)
	}
	if _, err := a.SetDefaultForProvider("codex", "removed-option", json.RawMessage(` null `)); err != nil {
		t.Fatal(err)
	}
	if _, err := a.SetPinnedForProvider("codex", "old-model", false); err != nil {
		t.Fatal(err)
	}
	if _, err := a.SetDefaultForProvider("codex", "model", json.RawMessage(`"codex-only"`)); err == nil {
		t.Fatal("new value accepted without catalog")
	}
	if agents.Allowed(acp.ConfigOption{Type: "boolean"}, json.RawMessage(`null`)) {
		t.Fatal("null is not a boolean option value")
	}
}

func TestModelPreferencesFailedSavePreservesMemory(t *testing.T) {
	for _, provider := range []string{"devin", "codex"} {
		t.Run(provider, func(t *testing.T) {
			a := modelPreferenceFixture(t)
			key, model := "model", "codex-only"
			if provider == "devin" {
				key, model = "devin-model", "devin-only"
			}
			before, _ := a.ProviderModelPreferences(provider)
			stateTestWrite(t, filepath.Join(configDir(), "settings.json.pending"), "invalid transaction")
			value, _ := json.Marshal(model)
			if _, err := a.SetDefaultForProvider(provider, key, value); err == nil {
				t.Fatal("failed default save accepted")
			}
			if _, err := a.SetPinnedForProvider(provider, model, true); err == nil {
				t.Fatal("failed favorite save accepted")
			}
			after, _ := a.ProviderModelPreferences(provider)
			beforeJSON, _ := json.Marshal(before)
			afterJSON, _ := json.Marshal(after)
			if string(beforeJSON) != string(afterJSON) || a.defaultProvider() != "devin" {
				t.Fatal("failed save changed in-memory preferences")
			}
		})
	}
}
