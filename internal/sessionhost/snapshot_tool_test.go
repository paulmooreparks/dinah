package sessionhost

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"

	"studio/internal/appletcontext"
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

func TestCodexInterruptedFragmentsAreRedactedAfterJoining(t *testing.T) {
	// This synthetic token has no authority. Every split must be safe even
	// when completion never supplies a replacement result.
	fake := appletcontext.Prefix + strings.Repeat("A", 26) + "." + strings.Repeat("B", 26)
	for split := 1; split < len(fake); split++ {
		t.Run(fmt.Sprint(split), func(t *testing.T) {
			dir := t.TempDir()
			var mu sync.Mutex
			capture := NewTranscriptCapture(&mu, TranscriptCaptureHost{
				StateDir: func() string { return dir }, VisibleText: appletcontext.Redact, RedactText: appletcontext.Redact,
				Log: func(string, string, string, ...any) {}, TurnEnd: func(string, string, error) {},
			})
			s := NewConversation(Identity{ID: "redaction", Provider: "codex", Cwd: dir}, InitialActivity{Open: true}, false, false)
			capture.Initialize(s, false)
			for _, fragment := range []string{fake[:split], fake[split:]} {
				u, err := json.Marshal(map[string]any{"sessionUpdate": "tool_call_update", "toolCallId": "A", "content": []any{map[string]any{"type": "content", "content": map[string]string{"type": "text", "text": fragment}}}})
				if err != nil {
					t.Fatal(err)
				}
				capture.RecordUpdate(s, u)
			}
			capture.FinishTurn(s, nil, "cancelled")
			tr, err := ReadTranscript(dir, s.ID())
			if err != nil {
				t.Fatal(err)
			}
			if len(tr.Messages) != 2 || tr.Messages[0].Role != "tool" || tr.Messages[0].Text != appletcontext.Redacted || tr.Messages[1].Role != "status" {
				t.Fatal("interrupted tool output was not redacted")
			}
		})
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
