package main

import (
	"encoding/json"
	"strings"
	"studio/internal/acp"
	"testing"
)

func TestClaudeModelRestoreMatchesProviderNamesBeforeSubstitution(t *testing.T) {
	for _, tc := range []struct {
		name, saved, savedName string
		note                   bool
	}{
		{"alias", "fable", "Fable 5.1", false},
		{"full ID", "claude-fable-5-1", "Fable 5.1", false},
		{"display name", "Fable 5.1", "", false},
		{"unknown model", "retired-model", "Retired model", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, c := providerTestApp(t)
			a.runtime.PutProviderLocked("claude", a.runtime.ProviderLocked("codex"))
			c.options = []acp.ConfigOption{{ID: "model", Name: "Model", Category: "model", Type: "select", CurrentValue: json.RawMessage(`"default"`), Options: json.RawMessage(`[{"value":"default","name":"Default"},{"value":"claude-fable-5-1","name":"Fable 5.1"}]`)}}
			id, ws := "claude:model-choice", t.TempDir()
			saved, _ := json.Marshal(tc.saved)
			catalog, _ := json.Marshal([]map[string]string{{"value": tc.saved, "name": tc.savedName}})
			a.settings.SessionProviders = map[string]string{id: "claude"}
			a.settings.SessionOptions = map[string]map[string]json.RawMessage{id: {"model": saved}}
			a.settings.ProviderCatalogs = map[string][]acp.ConfigOption{"claude": {{ID: "model", Category: "model", Type: "select", Options: catalog}}}
			a.settings.OptionSubstitutions = OptionSubstitutions{"claude": {{Option: "model", From: saved, FromName: tc.savedName, To: json.RawMessage(`"claude-fable-5-1"`), ToName: "Fable 5.1"}}}
			if err := saveSettings(a.settings); err != nil {
				t.Fatal(err)
			}
			info, err := a.OpenSessionAt(ws, id)
			if err != nil {
				t.Fatal(err)
			}
			if got := modelOf(t, a, id); got != `"claude-fable-5-1"` {
				t.Fatalf("model %s", got)
			}
			if tc.note {
				if len(info.Notes) != 1 || !strings.Contains(info.Notes[0], "Retired model") || !strings.Contains(info.Notes[0], "wasn't offered") {
					t.Fatalf("missing genuine substitution: %q", info.Notes)
				}
			} else {
				if len(info.Notes) != 0 {
					t.Fatalf("false substitution: %q", info.Notes)
				}
				if string(a.settings.SessionOptions[id]["model"]) != `"claude-fable-5-1"` {
					t.Fatal("confirmed offered ID was not persisted")
				}
			}
		})
	}
}
