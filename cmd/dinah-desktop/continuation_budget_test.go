package main

import (
	"encoding/json"
	"strings"
	"testing"
	"unicode/utf8"

	"studio/internal/acp"
	"studio/internal/providers"
)

// The handoff that used to break the switch: a history far larger than the
// target's window, which must arrive compacted, newest first.
var (
	budgetCodex  = providers.For("codex").HandoffChars
	budgetClaude = providers.For("claude").HandoffChars
	budgetOllama = providers.For("ollama").HandoffChars
)

func TestAHandoffFitsTheProvidersContext(t *testing.T) {
	// Codex's app-server refuses a request of more than 1,048,576 characters,
	// which is what a long chat's handoff used to exceed.
	if budgetCodex >= 1_048_576 {
		t.Fatalf("the Codex budget is not under the app-server's request cap")
	}
	history := []ConversationMessage{{Role: "you", Provider: "claude", Text: "The first request."}}
	for i := 0; i < 40; i++ {
		history = append(history, ConversationMessage{Role: "agent", Provider: "claude", Text: strings.Repeat("middle words ", 12_000)})
	}
	history = append(history, ConversationMessage{Role: "you", Provider: "claude", Text: "The newest words."})

	text, dropped := compactHandoff(history, budgetCodex)
	if len(text) > budgetCodex {
		t.Fatalf("the handoff is %d characters, over the budget of %d", len(text), budgetCodex)
	}
	if dropped == 0 || dropped >= len(history) {
		t.Fatalf("dropped %d of %d messages", dropped, len(history))
	}
	if !strings.Contains(text, "summarised") || !strings.Contains(text, "full history is kept") {
		t.Fatal("the handoff does not say what was summarised")
	}
	// The newest message is kept whole, and the older ones appear as one line
	// each, so the new agent knows what came before.
	if !strings.Contains(text, "[you, claude]\nThe newest words.") {
		t.Fatal("the newest message was not kept whole")
	}
	if !strings.Contains(text, "- [you, claude] The first request.") {
		t.Fatal("the first message is not in the summary")
	}
}

// A history that fits is handed over whole, with no note about summarising.
func TestAShortHandoffGoesWhole(t *testing.T) {
	history := []ConversationMessage{
		{Role: "you", Provider: "claude", Text: "What changed today?"},
		{Role: "agent", Provider: "claude", Text: "Two things."},
	}
	text, dropped := compactHandoff(history, budgetCodex)
	if dropped != 0 || strings.Contains(text, "summarised") {
		t.Fatalf("a short history was compacted: %d dropped", dropped)
	}
	if !strings.Contains(text, "What changed today?") || !strings.Contains(text, "Two things.") {
		t.Fatalf("the handoff lost a message: %s", text)
	}
}

// The budget follows the target: Codex's cap, a Claude window, and an Ollama
// model's own context length, which the agent reports as a string.
func TestTheBudgetFollowsTheTarget(t *testing.T) {
	if got := continuationBudget("codex", nil); got != budgetCodex {
		t.Fatalf("Codex budget %d", got)
	}
	if got := continuationBudget("claude", nil); got != budgetClaude {
		t.Fatalf("Claude budget %d", got)
	}
	if got := continuationBudget("ollama", nil); got != budgetOllama {
		t.Fatalf("Ollama budget with no context option %d", got)
	}
	options := []acp.ConfigOption{{ID: "context", Category: "context", CurrentValue: json.RawMessage(`"32768"`)}}
	if got := continuationBudget("ollama", options); got != 49152 {
		t.Fatalf("Ollama budget for a 32768-token model %d", got)
	}
	number := []acp.ConfigOption{{ID: "context", CurrentValue: json.RawMessage(`8192`)}}
	if got := continuationBudget("ollama", number); got != 12288 {
		t.Fatalf("Ollama budget for a numeric option %d", got)
	}
}

// A single message larger than the whole budget is clipped, and the clip
// never splits a character.
func TestOneHugeMessageIsClipped(t *testing.T) {
	history := []ConversationMessage{{Role: "agent", Provider: "claude", Text: strings.Repeat("é", 400_000)}}
	text, _ := compactHandoff(history, budgetLeast*2)
	if len(text) > budgetLeast*2 {
		t.Fatalf("a huge message was not clipped: %d characters", len(text))
	}
	if !strings.Contains(text, "clipped to fit") {
		t.Fatal("the clip is not marked")
	}
	for _, r := range text {
		if r == '\uFFFD' {
			t.Fatal("the clip split a character")
		}
	}
	// Each size cuts the two-byte characters at a different place, and no
	// clip may come out longer than its size or end inside a character.
	long := strings.Repeat("\u00E9", 5_000)
	for size := 60; size < 200; size++ {
		got := clipText(long, size)
		if len(got) > size || !utf8.ValidString(got) {
			t.Fatalf("clipText to %d bytes gave %d bytes, valid %v", size, len(got), utf8.ValidString(got))
		}
	}
}
