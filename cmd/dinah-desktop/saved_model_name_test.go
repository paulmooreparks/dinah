package main

import (
	"encoding/json"
	"testing"

	"studio/internal/acp"
)

// A closed chat shows the model it gets back when it opens: its saved value,
// after a substitution stored for that value, named as the program's last list
// names it. A value the list no longer names, a chat with nothing saved, and a
// program with no choice of model show no model rather than a code.
func TestSavedModelNameIsTheModelAClosedChatGetsBack(t *testing.T) {
	codexModels := []acp.ConfigOption{{ID: "model", Category: "model", Type: "select", CurrentValue: json.RawMessage(`"sol"`),
		Options: json.RawMessage(`[{"value":"sol","name":"Sol"},{"value":"luna","name":"Luna"}]`)}}
	devinModels := []acp.ConfigOption{{ID: "model", Category: "model", Type: "select", CurrentValue: json.RawMessage(`"adaptive"`),
		Options: json.RawMessage(`[{"value":"adaptive","name":"Adaptive"},{"value":"swe-2","name":"SWE-2"}]`)}}
	saved := Settings{
		Catalog:          devinModels,
		ProviderCatalogs: map[string][]acp.ConfigOption{"codex": codexModels, "ollama": {{ID: "think", Type: "boolean"}}},
		SessionOptions: map[string]map[string]json.RawMessage{
			"codex:plain":    {"model": json.RawMessage(`"luna"`)},
			"codex:replaced": {"model": json.RawMessage(`"retired"`)},
			"codex:gone":     {"model": json.RawMessage(`"unknown"`)},
			"devin-chat":     {"model": json.RawMessage(`"adaptive"`)},
			"ollama:chat":    {"think": json.RawMessage(`true`)},
		},
		OptionSubstitutions: OptionSubstitutions{"codex": {{Option: "model", From: json.RawMessage(`"retired"`), To: json.RawMessage(`"sol"`)}}},
	}
	for _, tc := range []struct{ provider, session, want string }{
		{"codex", "codex:plain", "Luna"},
		{"codex", "codex:replaced", "Sol"},
		{"codex", "codex:gone", ""},
		{"codex", "codex:nothing-saved", ""},
		{"devin", "devin-chat", "Adaptive"},
		{"ollama", "ollama:chat", ""},
	} {
		if got := savedModelName(saved, tc.provider, tc.session); got != tc.want {
			t.Errorf("%s: got %q, want %q", tc.session, got, tc.want)
		}
	}
}
