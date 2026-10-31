package sessionhost

import (
	"fmt"
	"strings"
	"testing"
)

func TestCodexStreamMatchesCompletedReplay(t *testing.T) {
	start := `{"sessionUpdate":"tool_call","toolCallId":"A","title":"Run","status":"in_progress"}`
	delta := `{"sessionUpdate":"tool_call_update","toolCallId":"A","content":[{"type":"content","content":{"type":"text","text":"x"}}]}`
	complete := fmt.Sprintf(`{"sessionUpdate":"tool_call_update","toolCallId":"A","title":"Run","status":"completed","content":[{"type":"content","content":{"type":"text","text":%q}}]}`, strings.Repeat("x", 20000))
	live := []string{start}
	for range 20000 {
		live = append(live, delta)
	}
	partial := recordedRows(t, "codex", false, live)
	if len(partial) != 1 || len(partial[0].Text) > maxConversationToolText+100 {
		t.Fatalf("partial rows=%d", len(partial))
	}
	live = append(live, complete, complete)
	got := recordedRows(t, "codex", false, live)
	want := recordedRows(t, "codex", true, []string{strings.Replace(complete, "tool_call_update", "tool_call", 1)})
	if len(got) != 1 || strings.Join(rowTexts(got), "\n") != strings.Join(rowTexts(want), "\n") {
		t.Fatalf("live rows=%d, replay rows=%d", len(got), len(want))
	}
}

func TestCodexInterleavedAndMissingStarts(t *testing.T) {
	rows := recordedRows(t, "codex", false, []string{
		`{"sessionUpdate":"tool_call_update","toolCallId":"A","content":[{"type":"content","content":{"type":"text","text":"hel"}}]}`,
		`{"sessionUpdate":"tool_call","toolCallId":"B","title":"Other","status":"in_progress"}`,
		`{"sessionUpdate":"tool_call_update","toolCallId":"A","content":[{"type":"content","content":{"type":"text","text":"lo"}}]}`,
		`{"sessionUpdate":"tool_call_update","toolCallId":"B","title":"Other","status":"failed"}`,
	})
	if len(rows) != 2 || rows[0].Text != "hello" || rows[1].Text != "Other\nStatus: failed" {
		t.Fatalf("rows=%q", rowTexts(rows))
	}
}
