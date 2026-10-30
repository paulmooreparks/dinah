package sessionhost

import (
	"strings"
	"sync"
	"testing"
)

// The updates of one Read and one Bash call as the Claude adapter sent them
// live, and as the replay of its history sent the same two calls, both
// measured on 29 October 2026 from the adapter's own protocol traffic.
var (
	claudeLiveUpdates = []string{
		`{"sessionUpdate":"tool_call","toolCallId":"A","title":"Read File","status":"pending","content":[]}`,
		`{"sessionUpdate":"tool_call_update","toolCallId":"A","title":"Read hello.txt"}`,
		`{"sessionUpdate":"tool_call","toolCallId":"B","title":"Terminal","status":"pending","content":[]}`,
		`{"sessionUpdate":"tool_call_update","toolCallId":"A","status":"completed"}`,
		`{"sessionUpdate":"tool_call_update","toolCallId":"B","title":"echo hi"}`,
		`{"sessionUpdate":"tool_call_update","toolCallId":"B","content":[{"type":"content","content":{"type":"text","text":"Echo the word hi"}}]}`,
		`{"sessionUpdate":"tool_call_update","toolCallId":"B","status":"completed","content":[{"type":"content","content":{"type":"text","text":"` + "```console\\nhi\\n```" + `"}}]}`,
	}
	claudeReplayUpdates = []string{
		`{"sessionUpdate":"tool_call","toolCallId":"A","title":"Read hello.txt","status":"pending","content":[]}`,
		`{"sessionUpdate":"tool_call_update","toolCallId":"A","status":"completed"}`,
		`{"sessionUpdate":"tool_call","toolCallId":"B","title":"echo hi","status":"pending","content":[{"type":"content","content":{"type":"text","text":"Echo the word hi"}}]}`,
		`{"sessionUpdate":"tool_call_update","toolCallId":"B","status":"completed","content":[{"type":"content","content":{"type":"text","text":"` + "```console\\nhi\\n```" + `"}}]}`,
	}
)

// recordedRows records updates for a chat of a provider, as a live turn or as
// a replay, and returns the rows the saved copy then holds.
func recordedRows(t *testing.T, provider string, replay bool, updates []string) []ConversationMessage {
	t.Helper()
	dir := t.TempDir()
	var mu sync.Mutex
	capture := NewTranscriptCapture(&mu, TranscriptCaptureHost{StateDir: func() string { return dir }, VisibleText: func(s string) string { return s }, RedactText: func(s string) string { return s }, Log: func(string, string, string, ...any) {}, TurnEnd: func(string, string, error) {}})
	s := NewConversation(Identity{ID: "chat", Provider: provider, Cwd: dir}, InitialActivity{Open: true}, replay, false)
	capture.Initialize(s, replay)
	for _, u := range updates {
		capture.RecordUpdate(s, []byte(u))
	}
	if replay {
		capture.FinishReplay(s, true)
	} else {
		capture.FinishTurn(s, nil, "end_turn")
	}
	tr, err := ReadTranscript(dir, "chat")
	if err != nil {
		t.Fatal(err)
	}
	var rows []ConversationMessage
	for _, m := range tr.Messages {
		if m.Role == "tool" || m.Role == "limitation" {
			rows = append(rows, m)
		}
	}
	return rows
}

func rowTexts(rows []ConversationMessage) []string {
	out := make([]string, len(rows))
	for i, r := range rows {
		out[i] = r.Text
	}
	return out
}

// A Claude call is recorded live the way its history replays: one row for the
// call, with its final title, the status it began with and its content, and
// one for how it ended. So the saved copy of a chat and the replay of its
// history agree, and the warning that a replay holds what the copy lacks means
// something again.
func TestAClaudeToolCallIsRecordedLiveAsItsReplayHasIt(t *testing.T) {
	live := recordedRows(t, "claude", false, claudeLiveUpdates)
	replay := recordedRows(t, "claude", true, claudeReplayUpdates)
	want := []string{
		"Read hello.txt\nStatus: pending",
		"Status: completed",
		"echo hi\nStatus: pending\nEcho the word hi",
		"Status: completed\n```console\nhi\n```",
	}
	// The two calls overlapped when they were live, the second beginning
	// before the first had ended, and the replay lists each call's rows
	// together; the live copy puts each ending after its call's own rows.
	if got := rowTexts(live); strings.Join(got, "\x00") != strings.Join(want, "\x00") {
		t.Fatalf("live rows:\n%q\nwant:\n%q", got, want)
	}
	if got := rowTexts(replay); strings.Join(got, "\x00") != strings.Join(want, "\x00") {
		t.Fatalf("replay rows:\n%q\nwant:\n%q", got, want)
	}
	if !ReplayRepresented(live, replay) {
		t.Fatalf("the replay %q is not represented by the live rows %q", rowTexts(replay), rowTexts(live))
	}
}

// A call that runs on its own, the usual case, gives rows that match the
// replay's exactly and in order.
func TestAClaudeCallThatRunsAloneMatchesItsReplayRowForRow(t *testing.T) {
	live := recordedRows(t, "claude", false, []string{claudeLiveUpdates[2], claudeLiveUpdates[4], claudeLiveUpdates[5], claudeLiveUpdates[6]})
	replay := recordedRows(t, "claude", true, claudeReplayUpdates[2:])
	if strings.Join(rowTexts(live), "\x00") != strings.Join(rowTexts(replay), "\x00") {
		t.Fatalf("live %q, replay %q", rowTexts(live), rowTexts(replay))
	}
	if !ReplayRepresented(live, replay) {
		t.Fatal("the replay is not represented by the live rows")
	}
}

// Another provider's calls are recorded one row for each event, as before.
func TestOtherProvidersCallsKeepOneRowForEachEvent(t *testing.T) {
	rows := recordedRows(t, "codex", false, claudeLiveUpdates)
	if len(rows) != len(claudeLiveUpdates) {
		t.Fatalf("%d rows for %d events: %q", len(rows), len(claudeLiveUpdates), rowTexts(rows))
	}
}

// An update for a call the history has no first row for, such as the first one
// seen after a chat was opened mid-call, still gets a row, and a call's long
// result is cut the way every long result is.
func TestAnUpdateForAnUnknownCallStillGetsARow(t *testing.T) {
	rows := recordedRows(t, "claude", false, []string{
		`{"sessionUpdate":"tool_call_update","toolCallId":"Z","title":"Run it"}`,
		`{"sessionUpdate":"tool_call_update","toolCallId":"Z","status":"completed","content":[{"type":"content","content":{"type":"text","text":"` + strings.Repeat("x", maxConversationToolText+500) + `"}}]}`,
	})
	if len(rows) != 2 || rows[0].Text != "Run it" || !strings.HasPrefix(rows[1].Text, "Status: completed\n") || !strings.Contains(rows[1].Text, "more characters of this result are in the chat's log") {
		t.Fatalf("rows %q", rowTexts(rows))
	}
}

// A handoff built from a saved copy in the old form, a row for each event, reads
// as it did: every call's title, command and result is in the summary and the
// handoff, as it is for the same calls in the new form.
func TestAHandoffFromAnOldManyRowCopyStillReadsCorrectly(t *testing.T) {
	old := recordedRows(t, "codex", false, claudeLiveUpdates)
	fresh := recordedRows(t, "claude", false, claudeLiveUpdates)
	formatter := HandoffFormatter{ProductName: "Dinah", Speaker: func(m ConversationMessage) string { return m.Role }}
	for name, rows := range map[string][]ConversationMessage{"old": old, "new": fresh} {
		summary := formatter.Summary(rows, 4096)
		handoff, summarised := formatter.Compact(rows, 1<<20)
		if summarised != 0 {
			t.Fatalf("%s: %d messages were summarised away", name, summarised)
		}
		for _, want := range []string{"Read hello.txt", "echo hi", "hi"} {
			if !strings.Contains(handoff, want) {
				t.Errorf("%s: the handoff lacks %q:\n%s", name, want, handoff)
			}
		}
		if !strings.Contains(summary, "echo hi") || !strings.Contains(summary, "Read hello.txt") || strings.Count(summary, "\n") != len(rows)+1 {
			t.Errorf("%s: the summary of %d rows:\n%s", name, len(rows), summary)
		}
	}
}
