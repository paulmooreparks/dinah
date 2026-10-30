package bus

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestHostResultIsDurablePrivateAndIdempotent(t *testing.T) {
	b, dir := open(t)
	ctx := context.Background()
	caller, err := b.RegisterSession(ctx, "caller", dir, "Caller")
	if err != nil {
		t.Fatal(err)
	}
	other, err := b.RegisterSession(ctx, "other", dir, "Other")
	if err != nil {
		t.Fatal(err)
	}
	if err := b.EndSession(ctx, caller.ID); err != nil {
		t.Fatal(err)
	}
	first, err := b.EnqueueResult(ctx, "operation-one", caller.ID, "The parser is ready.")
	if err != nil {
		t.Fatal(err)
	}
	// A new connection simulates reconciliation after a process restart.
	reopened, err := OpenAt(filepath.Join(dir, "bus.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	again, err := reopened.EnqueueResult(ctx, "operation-one", caller.ID, first.Text)
	if err != nil || again.ID != first.ID {
		t.Fatalf("reconciliation duplicated the result: %+v %v", again, err)
	}
	if _, err := reopened.EnqueueResult(ctx, "operation-one", other.ID, first.Text); err == nil {
		t.Fatal("the same key was reused for another caller")
	}
	if _, err := reopened.EnqueueResult(ctx, "operation-one", caller.ID, "Changed"); err == nil {
		t.Fatal("the same key was reused for another result")
	}
	if n, _, err := reopened.Doorbell(ctx, other.ID); err != nil || n != 0 {
		t.Fatalf("another chat received the private result: %d %v", n, err)
	}
	if _, err := reopened.RegisterSession(ctx, caller.ID, dir, "Caller"); err != nil {
		t.Fatal(err)
	}
	msgs, _, err := reopened.Deliver(ctx, caller.ID, DoorbellTurn)
	if err != nil || len(msgs) != 1 {
		t.Fatalf("the reopened caller lost its result: %+v %v", msgs, err)
	}
	if msgs[0].FromName != "Dinah" || msgs[0].Kind != ResultKind || msgs[0].Asked || msgs[0].Trusted {
		t.Fatalf("the result acquired the wrong attribution or authority: %+v", msgs[0])
	}
	if _, err := reopened.Send(ctx, Sender{Session: &caller}, "", "Thanks", nil, first.ID); err == nil {
		t.Fatal("replying to a host report sent a message to Paul")
	}
	formatted := FormatForAgent(msgs, PersonFallback)
	if !strings.Contains(formatted, first.Text) || strings.Contains(formatted, "reply tool") {
		t.Fatalf("the report encouraged an acknowledgement loop: %s", formatted)
	}
	if _, err := reopened.EnqueueResult(ctx, "operation-one", caller.ID, first.Text); err != nil {
		t.Fatal(err)
	}
	if n, _, err := reopened.Doorbell(ctx, caller.ID); err != nil || n != 0 {
		t.Fatalf("reconciliation redelivered a consumed result: %d %v", n, err)
	}
}

func TestHostResultWithUnknownCallerDoesNotPartlyCommit(t *testing.T) {
	b, dir := open(t)
	ctx := context.Background()
	if _, err := b.EnqueueResult(ctx, "result", "missing", "Done"); err == nil {
		t.Fatal("a result for an unknown caller was accepted")
	}
	var count int
	if err := b.db.QueryRow(`SELECT COUNT(*) FROM messages`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("a failed delivery partly committed: %d %v", count, err)
	}
	s, err := b.RegisterSession(ctx, "caller", dir, "Caller")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.EnqueueResult(ctx, "result", s.ID, "Done"); err != nil {
		t.Fatal(err)
	}
	msgs, _, err := b.Deliver(ctx, s.ID, DoorbellTurn)
	if err != nil || len(msgs) != 1 || msgs[0].FromName == Human {
		t.Fatalf("the host report was attributed to Paul: %+v %v", msgs, err)
	}
}

// A host result reaches the chat that asked as text: a line with the request
// and its status, the error if any, and then the text the action wrote, with
// its line breaks, rather than the JSON the report is kept as. A report with
// no text, such as one saved before reports had any, shows its output as an
// indented JSON block. A text that is not a report is written as it is. The
// bus reads no action's output to do this.
func TestHostReportIsWrittenAsText(t *testing.T) {
	reply := "1. **Your empty-group approach is supported.**\n\n   Update the spans.\n\n2. Take v0.88.0 now."
	text := "Reply from the chat \"PUDL Development\" (session codex:pudl):\n\n" + reply
	output := `{"reply":` + jsonString(reply) + `,"sessionId":"codex:pudl","stopReason":"end_turn","title":"PUDL Development"}`
	report := `{"invocationId":"ee23","request":{"sessionId":"codex:pudl","question":"Please answer:\n1. Is it supported?"},"status":"done","output":` + output + `,"text":` + jsonString(text) + `}`
	got := formatHostReport(report)
	if want := "Request ee23 is done.\n\n" + text; got != want {
		t.Fatalf("the report is\n%s\nnot\n%s", got, want)
	}
	legacy := formatHostReport(`{"invocationId":"ee24","request":{"sessionId":"codex:pudl","question":"Q"},"status":"done","output":` + output + `}`)
	for _, want := range []string{"Request ee24 is done.", "Output:\n\n```json\n{\n  \"reply\": ", "\"title\": \"PUDL Development\"\n}\n```"} {
		if !strings.Contains(legacy, want) {
			t.Fatalf("the legacy report lacks %q:\n%s", want, legacy)
		}
	}
	failed := formatHostReport(`{"invocationId":"f1","request":{"sessionId":"x","question":"Q"},"status":"failed","error":"the chat is closed"}`)
	if failed != "Request f1 is failed.\nError: the chat is closed" {
		t.Fatalf("a failed report: %s", failed)
	}
	fenced := formatHostReport(`{"invocationId":"o1","request":{},"status":"done","output":{"code":"` + "```" + `"}}`)
	if !strings.Contains(fenced, "````json\n") || !strings.HasSuffix(fenced, "\n````") {
		t.Fatalf("output with a fence in it: %s", fenced)
	}
	if plain := formatHostReport("Changed"); plain != "Changed" {
		t.Fatalf("a plain result was rewritten: %q", plain)
	}
}

// A result saved before reports had text marshals to the same report as
// before, so reconciliation enqueuing it again still matches the result the
// bus already holds.
func TestLegacyHostReportReenqueuesUnchanged(t *testing.T) {
	b, dir := open(t)
	ctx := context.Background()
	caller, err := b.RegisterSession(ctx, "caller", dir, "Caller")
	if err != nil {
		t.Fatal(err)
	}
	legacy := `{"invocationId":"r1","request":{"q":1},"status":"done","output":{"ok":true}}`
	first, err := b.EnqueueResult(ctx, "legacy", caller.ID, legacy)
	if err != nil {
		t.Fatal(err)
	}
	again, err := json.Marshal(HostReport{InvocationID: "r1", Request: json.RawMessage(`{"q":1}`), Status: "done", Output: json.RawMessage(`{"ok":true}`)})
	if err != nil || string(again) != legacy {
		t.Fatalf("a report without text changed shape: %s %v", again, err)
	}
	if m, err := b.EnqueueResult(ctx, "legacy", caller.ID, string(again)); err != nil || m.ID != first.ID {
		t.Fatalf("re-enqueueing a legacy result: %+v %v", m, err)
	}
}

// A delivered message is described for a person without the instructions an
// agent is given, and a result says who produced it and how it ended.
func TestDescribeIsForAReader(t *testing.T) {
	reply := Describe(Message{ID: 7, Kind: ResultKind, FromName: "Dinah", CreatedAt: "2026-10-26T10:00:00Z", Text: `{"invocationId":"ee23","request":{},"status":"done","text":"Reply from the chat \"PUDL\" (session s):\n\nYes.","from":"PUDL"}`})
	if reply.Kind != "result" || reply.From != "PUDL" || reply.Status != "done" || reply.Text != "Request ee23 is done.\n\nReply from the chat \"PUDL\" (session s):\n\nYes." {
		t.Fatalf("a reply: %+v", reply)
	}
	legacy := Describe(Message{Kind: ResultKind, FromName: "Dinah", Text: `{"invocationId":"r1","request":{},"status":"failed","error":"closed"}`})
	if legacy.From != "" || legacy.Status != "failed" || !strings.Contains(legacy.Text, "Error: closed") {
		t.Fatalf("a report with no producer: %+v", legacy)
	}
	text := Describe(Message{ID: 8, Kind: "text", FromName: "Docs Pass", FromWorkspace: "PUDL", Room: "release", RoomTitle: "Release Review", Text: "Shipped.", Attachments: []Attachment{{Index: 1, Rel: "notes.md", Size: 12, SHA256: strings.Repeat("a", 64)}}})
	if text.Kind != "text" || text.Workspace != "PUDL" || text.Text != "Posted in the group chat Release Review.\n\nShipped.\n\nAttachment 1: notes.md (12 bytes)." {
		t.Fatalf("a message: %+v", text)
	}
	release := Describe(Message{Kind: "release", FromName: "pudl-1", FromWorkspace: "PUDL", Version: "v0.90.0", Text: "Notes."})
	if release.Text != "Release notice: PUDL v0.90.0.\n\nNotes." || strings.Contains(release.Text, "verdict tool") {
		t.Fatalf("a release: %+v", release)
	}
}

func jsonString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}
