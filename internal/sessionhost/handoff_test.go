package sessionhost

import (
	"fmt"
	"strings"
	"testing"
)

func TestHandoffFormatterUsesHostAttributionAndProduct(t *testing.T) {
	var speakers []string
	f := HandoffFormatter{ProductName: "Host Product", Speaker: func(m ConversationMessage) string {
		speakers = append(speakers, m.Role)
		return "host/" + m.Role
	}}
	history := []ConversationMessage{{Role: "you", Text: "first"}, {Role: "agent", Text: "second"}}
	got, dropped := f.Compact(history, MinimumHandoffBudget)
	if dropped != 0 || !strings.HasSuffix(got, "[host/you]\nfirst\n\n[host/agent]\nsecond\n\n") || strings.Join(speakers, ",") != "you,agent" {
		t.Fatalf("short handoff changed: %q, %d, %v", got, dropped, speakers)
	}
	if got := f.Summary(history, MinimumHandoffBudget); got != "What came before this chat, one line for each message:\n- [host/you] first\n- [host/agent] second\n" {
		t.Fatalf("summary bytes %q", got)
	}
	var large []ConversationMessage
	for i := 0; i < 200; i++ {
		large = append(large, ConversationMessage{Role: "agent", Text: fmt.Sprintf("%d %s", i, strings.Repeat("x", 200))})
	}
	summary := f.Summary(large, MinimumHandoffBudget)
	compact, count := f.Compact(large, MinimumHandoffBudget)
	if count == 0 || !strings.Contains(summary, "kept in Host Product\n") || !strings.Contains(compact, "kept in Host Product\n") {
		t.Fatal("compacted handoff lost product attribution")
	}
	if large[0].Text != "0 "+strings.Repeat("x", 200) {
		t.Fatal("formatting mutated history")
	}
}
