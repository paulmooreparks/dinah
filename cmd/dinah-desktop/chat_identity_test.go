package main

import (
	"encoding/json"
	"strings"
	"testing"

	"studio/internal/acp"
)

func modelOptions(current string, options string) []acp.ConfigOption {
	return []acp.ConfigOption{{ID: "model", Name: "Model", Category: "model", Type: "select", CurrentValue: json.RawMessage(`"` + current + `"`), Options: json.RawMessage(options)}}
}

// The context Dinah gives a chat with each turn says which provider it runs
// on and which model, by the model's name, and says the new one after the
// model changes. A chat whose provider offers no model option is told its
// provider alone.
func TestEachTurnsContextTellsTheChatItsProviderAndModel(t *testing.T) {
	a, _, s, _, _ := busChat(t)
	context := func() string {
		t.Helper()
		text, err := a.appletPromptContext(s.ID())
		if err != nil {
			t.Fatal(err)
		}
		return text
	}
	provider := providerDisplayName(s.Provider())
	s.UpdateOptions(nil)
	if text := context(); !strings.Contains(text, "This chat runs on "+provider+".\n") {
		t.Fatalf("a chat with no model option is not told its provider:\n%s", text)
	}
	s.UpdateOptions(modelOptions("glm-5.1:cloud", `[{"value":"deepseek-v4.1-flash:cloud","name":"DeepSeek V4.1 Flash"},{"value":"glm-5.1:cloud","name":"GLM 5.1"}]`))
	if text := context(); !strings.Contains(text, "This chat runs on "+provider+" with the model GLM 5.1.\n") {
		t.Fatalf("the model is not named:\n%s", text)
	}
	// The model changes during the conversation, and a list in groups names it too.
	s.UpdateOptions(modelOptions("sonnet", `[{"name":"Claude","options":[{"value":"opus","name":"Opus"},{"value":"sonnet","name":"Sonnet 5.5"}]}]`))
	text := context()
	if !strings.Contains(text, "with the model Sonnet 5.5.\n") || strings.Contains(text, "GLM 5.1") {
		t.Fatalf("the new model is not told:\n%s", text)
	}
	// A value the list does not name is told as it is.
	s.UpdateOptions(modelOptions("mystery-1", `[{"value":"other","name":"Other"}]`))
	if text := context(); !strings.Contains(text, "with the model mystery-1.\n") {
		t.Fatalf("an unlisted model is not told as its value:\n%s", text)
	}
}
