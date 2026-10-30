package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"studio/internal/acp"
	"studio/internal/bus"
)

func continuationSource(t *testing.T, a *App) *Session {
	t.Helper()
	s := newSessionState(&sessionSeed{ID: "original-devin", Provider: "devin", Cwd: t.TempDir(), Title: "Original Chat"}, sessionInitial{Open: true})
	a.runtime.PutConversationLocked(s.ID(), s)
	a.transcriptCapture().Initialize(s, false)
	a.transcriptCapture().RecordMessage(s, "you", "Create a report.", false, false)
	a.transcriptCapture().RecordMessage(s, "agent", "The report is saved.", false, false)
	a.transcriptCapture().FinishTurn(s, nil, "end_turn")
	return s
}

// The links between saved conversations are read again only for the files
// that changed, and follow a rewritten link and a removed file.
func TestConversationLinksFollowChangedFiles(t *testing.T) {
	parent, child := "links-parent-"+t.Name(), "links-child-"+t.Name()
	if err := writeTranscript(&conversationTranscript{SessionID: child, Provider: "codex", Workspace: "C:/W", ParentSessionID: parent, Complete: true}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Remove(transcriptPath(child)) })
	if conversationChild(parent) != child {
		t.Fatal("the link was not found")
	}
	// Rewritten with the time moved on, as a later write would leave it.
	if err := writeTranscript(&conversationTranscript{SessionID: child, Provider: "codex", Workspace: "C:/W", Complete: true, Deleted: true}); err != nil {
		t.Fatal(err)
	}
	later := time.Now().Add(time.Minute)
	os.Chtimes(transcriptPath(child), later, later)
	if conversationChild(parent) != "" {
		t.Fatal("a rewritten conversation's old link was kept")
	}
	if _, records := conversationLinks(); records[child] == nil || !records[child].Deleted {
		t.Fatal("the rewritten head was not read")
	}
	os.Remove(transcriptPath(child))
	if _, records := conversationLinks(); records[child] != nil {
		t.Fatal("a removed conversation was kept")
	}
}

func TestContinuationCreatesLinkedChatWithoutPrompt(t *testing.T) {
	a, c := providerTestApp(t)
	s := continuationSource(t, a)
	preview, err := a.PreviewSessionContinuation(s.ID(), "codex")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(preview.Context, "Do not repeat completed actions") {
		t.Fatal("handoff lacks replay boundary")
	}
	result, err := a.ContinueSessionWithProvider(s.ID(), "codex", preview.Revision)
	if err != nil {
		t.Fatal(err)
	}
	for _, call := range c.calls {
		if call.method == "session/prompt" {
			t.Fatal("switch sent a prompt")
		}
	}
	if result.ParentSessionID != s.ID() || result.Session.Provider != "codex" || a.session(s.ID()) != s {
		t.Fatal("native source session was lost")
	}
	head, err := a.ResolveSessionContinuation(s.ID())
	if err != nil || head != result.Session.ID {
		t.Fatalf("old link did not resolve: %q %v", head, err)
	}
	if err := a.Send(s.ID(), "Another request", nil, nil); err == nil {
		t.Fatal("superseded source accepted a new prompt")
	}
	if got := a.continuations().FilterAncestors([]SessionInfo{s.Info(), result.Session}, s.Cwd()); len(got) != 1 || got[0].ID != result.Session.ID {
		t.Fatalf("duplicate conversation rows: %+v", got)
	}
	// The successor remains listed after a restart before its provider connects.
	restarted := NewApp()
	listed := restarted.continuations().FilterAncestors(nil, s.Cwd())
	if len(listed) != 1 || listed[0].ID != result.Session.ID {
		t.Fatalf("persisted continuation missing: %+v", listed)
	}
	prefix, err := restarted.SessionContinuation(result.Session.ID)
	if err != nil || len(prefix.History) != len(preview.History)+1 || prefix.History[len(prefix.History)-1].Role != "provider_switch" {
		t.Fatalf("history did not survive restart: %+v %v", prefix, err)
	}
	next := a.session(result.Session.ID)
	// Parent changes cannot alter the handoff already reviewed by the person.
	a.transcriptCapture().RecordMessage(s, "agent", "Unreviewed late text", false, false)
	a.transcriptCapture().FinishTurn(s, nil, "end_turn")
	handoff, err := a.continuations().PendingContext(next)
	if err != nil || handoff != preview.Context {
		t.Fatal("handoff changed after approval")
	}
	a.turnIO().Run(next, promptPointer("Continue the report", []acp.ContentBlock{{Type: "text", Text: "Continue the report"}}))
	var sent acp.PromptParams
	for _, call := range c.calls {
		if call.method == "session/prompt" {
			sent = call.params.(acp.PromptParams)
		}
	}
	if len(sent.Prompt) != 3 || sent.Prompt[1].Text != preview.Context {
		t.Fatalf("first prompt lost handoff: %+v", sent)
	}
	if pending, _ := a.continuations().PendingContext(next); pending != "" {
		t.Fatal("successful prompt did not consume pending handoff")
	}
}

func TestContinuationRejectsChangedOrUnavailableHistory(t *testing.T) {
	a, c := providerTestApp(t)
	s := continuationSource(t, a)
	p, err := a.PreviewSessionContinuation(s.ID(), "codex")
	if err != nil {
		t.Fatal(err)
	}
	a.transcriptCapture().RecordMessage(s, "agent", "A new visible response.", false, false)
	a.transcriptCapture().FinishTurn(s, nil, "end_turn")
	if _, err := a.ContinueSessionWithProvider(s.ID(), "codex", p.Revision); err == nil {
		t.Fatal("outdated confirmation accepted")
	}
	if len(c.calls) != 0 {
		t.Fatal("outdated confirmation created provider session")
	}
	missing := newSessionState(&sessionSeed{ID: "never-captured", Provider: "devin", Cwd: s.Cwd()}, sessionInitial{Open: true})
	a.runtime.PutConversationLocked(missing.ID(), missing)
	if _, err := a.PreviewSessionContinuation(missing.ID(), "codex"); err == nil || !strings.Contains(err.Error(), "reopen") {
		t.Fatalf("missing history accepted: %v", err)
	}
	empty := newSessionState(&sessionSeed{ID: "captured-empty", Provider: "devin", Cwd: s.Cwd()}, sessionInitial{Open: true})
	a.runtime.PutConversationLocked(empty.ID(), empty)
	a.transcriptCapture().Initialize(empty, false)
	if p, err := a.PreviewSessionContinuation(empty.ID(), "codex"); err != nil || len(p.History) != 0 {
		t.Fatalf("complete empty chat refused: %v", err)
	}
}

func TestContinuationGuardsBusyAndAppletWork(t *testing.T) {
	a, _ := providerTestApp(t)
	s := continuationSource(t, a)
	var clearApproval func()
	for _, test := range []struct {
		name  string
		set   func()
		clear func()
	}{
		{"working", func() { s.StartWork() }, func() { s.ReleaseWork() }},
		{"queued", func() { s.Enqueue(newPrompt("Later", nil)) }, func() { s.ClearQueue() }},
		{"steering", func() { s.SetInterrupt(promptPointer("Change", nil)) }, func() { s.SetInterrupt(nil) }},
		{"approval", func() { clearApproval = holdPermission(t, a, s.ID()) }, func() { clearApproval() }},
	} {
		t.Run(test.name, func(t *testing.T) {
			test.set()
			defer test.clear()
			// The preview shows what must finish, as facts, and has nothing to hand over;
			// the continuation itself is refused.
			p, err := a.PreviewSessionContinuation(s.ID(), "codex")
			if err != nil || p.Blocked == "" || len(p.History) != 0 || p.Revision != "" {
				t.Fatalf("a busy chat's preview: %+v %v", p, err)
			}
			running := p.Facts.Running
			switch test.name {
			case "working":
				if !running.Turn || !running.CanStop {
					t.Fatalf("a running turn is not a fact: %+v", running)
				}
			case "queued":
				if running.Queued != 1 {
					t.Fatalf("queued messages are not a fact: %+v", running)
				}
			case "approval":
				if !running.Approvals {
					t.Fatalf("a pending approval is not a fact: %+v", running)
				}
			}
			if _, err := a.ContinueSessionWithProvider(s.ID(), "codex", p.Revision); err == nil {
				t.Fatal("a busy chat was continued")
			}
		})
	}
	id, err := newAppletID()
	if err != nil {
		t.Fatal(err)
	}
	v := AppletInvocation{ID: id, Workspace: s.Cwd(), OwnerKind: "chat", OwnerID: s.ID(), Status: "waiting", Deadline: time.Now().Add(time.Hour)}
	if err := saveAppletInvocation(&v); err != nil {
		t.Fatal(err)
	}
	if p, err := a.PreviewSessionContinuation(s.ID(), "codex"); err != nil || !strings.Contains(p.Blocked, "applet") || p.Facts.Running.Applets != 1 {
		t.Fatalf("pending applet work: %+v %v", p, err)
	}
}

func TestContinuationRedactionAndTransferWarnings(t *testing.T) {
	a, _ := providerTestApp(t)
	s := continuationSource(t, a)
	secret := "dinah-applet-v1:" + strings.Repeat("A", 32) + "." + strings.Repeat("B", 32)
	a.transcriptCapture().RecordUpdate(s, json.RawMessage(`{"sessionUpdate":"agent_thought_chunk","content":{"type":"text","text":"PRIVATE_REASONING"}}`))
	a.transcriptCapture().RecordMessage(s, "agent", "Visible result "+secret, false, false)
	a.transcriptCapture().RecordPrompt(s, promptPointer("Look at this image", []acp.ContentBlock{{Type: "text", Text: "Look at this image"}, {Type: "image", Data: "PRIVATE_IMAGE_DATA"}}))
	a.transcriptCapture().FinishTurn(s, nil, "end_turn")
	p, err := a.PreviewSessionContinuation(s.ID(), "codex")
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Warnings) == 0 || !strings.Contains(p.Context, "not included") {
		t.Fatal("non-text omission was not disclosed")
	}
	for _, private := range []string{secret, "PRIVATE_REASONING", "PRIVATE_IMAGE_DATA"} {
		if strings.Contains(p.Context, private) {
			t.Fatalf("private content entered handoff: %q", private)
		}
	}
	// Text attachment and skill-expanded instructions must remain available.
	a.transcriptCapture().RecordPrompt(s, promptPointer("/review", []acp.ContentBlock{{Type: "text", Text: "Selected skill instructions"}, {Type: "resource", Resource: &acp.EmbeddedResource{URI: "file:///notes.txt", Text: "Attachment text"}}}))
	a.transcriptCapture().FinishTurn(s, nil, "end_turn")
	p, err = a.PreviewSessionContinuation(s.ID(), "codex")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(p.Context, "Selected skill instructions") || !strings.Contains(p.Context, "Attachment text") {
		t.Fatal("textual prompt context was lost")
	}
}

func TestContinuationReplayPreservesCapturedHistory(t *testing.T) {
	a, _ := providerTestApp(t)
	s := continuationSource(t, a)
	s.SetReplaying(true)
	a.transcriptCapture().Initialize(s, true)
	a.transcriptCapture().RecordUpdate(s, json.RawMessage(`{"sessionUpdate":"agent_message_chunk","content":{"type":"text","text":"The report "}}`))
	a.transcriptCapture().RecordUpdate(s, json.RawMessage(`{"sessionUpdate":"agent_message_chunk","content":{"type":"text","text":"is saved."}}`))
	s.SetReplaying(false)
	a.transcriptCapture().FinishReplay(s, true)
	if _, err := a.PreviewSessionContinuation(s.ID(), "codex"); err != nil {
		t.Fatalf("chunked partial replay damaged complete local history: %v", err)
	}
	s.SetReplaying(true)
	a.transcriptCapture().Initialize(s, true)
	a.transcriptCapture().RecordUpdate(s, json.RawMessage(`{"sessionUpdate":"agent_message_chunk","content":{"type":"text","text":"An external message not captured here."}}`))
	s.SetReplaying(false)
	a.transcriptCapture().FinishReplay(s, true)
	// The agent's history has a message the saved copy lacked, so its replay
	// becomes the saved copy: nothing is left out of a handoff, and the chat
	// reopens from the copy next time.
	p, err := a.PreviewSessionContinuation(s.ID(), "codex")
	if err != nil || !strings.Contains(p.Context, "An external message not captured here.") {
		t.Fatalf("the replay did not become the saved copy: %v %q", err, p.Context)
	}
	if v := a.SavedConversation(s.ID()); !v.Available || len(v.Messages) != 1 {
		t.Fatalf("the replayed copy cannot reopen the chat: %+v", v)
	}
	if replayRepresented([]ConversationMessage{{Role: "you", Text: "Do not delete the file"}}, []ConversationMessage{{Role: "you", Text: "Delete the file"}}) {
		t.Fatal("partial words accepted as whole message")
	}
}

// What Dinah adds to the person's prompt for the agent, bus messages and the
// marker for the hooks, does not make the agent's replay of that prompt look
// like history the saved copy lacks, so the chat still reopens from it.
func TestReplayOfAPromptWithBusMessagesAndMarkerMatches(t *testing.T) {
	a, _ := providerTestApp(t)
	s := continuationSource(t, a)
	a.transcriptCapture().RecordPrompt(s, promptPointer("Carry on", []acp.ContentBlock{{Type: "text", Text: "Carry on"}}))
	a.transcriptCapture().RecordMessage(s, "you", "Dinah bus: 1 new message.\n[message 9] from pudl-1: released", false, false)
	a.transcriptCapture().RecordMessage(s, "agent", "Done.", false, false)
	a.transcriptCapture().FinishTurn(s, nil, "end_turn")
	s.SetReplaying(true)
	a.transcriptCapture().Initialize(s, true)
	for _, u := range []string{
		`{"sessionUpdate":"user_message_chunk","content":{"type":"text","text":"Create a report."}}`,
		`{"sessionUpdate":"agent_message_chunk","content":{"type":"text","text":"The report is saved."}}`,
		`{"sessionUpdate":"user_message_chunk","content":{"type":"text","text":"Carry on"}}`,
		`{"sessionUpdate":"user_message_chunk","content":{"type":"text","text":"Dinah bus: 1 new message.\n[message 9] from pudl-1: released"}}`,
		`{"sessionUpdate":"user_message_chunk","content":{"type":"text","text":"[Dinah Applet Context]\nprivate"}}`,
		`{"sessionUpdate":"user_message_chunk","content":{"type":"text","text":"` + bus.SteerMarker("window-ABC") + `"}}`,
		`{"sessionUpdate":"agent_message_chunk","content":{"type":"text","text":"Done."}}`,
	} {
		a.transcriptCapture().RecordUpdate(s, json.RawMessage(u))
	}
	s.SetReplaying(false)
	a.transcriptCapture().FinishReplay(s, true)
	if v := a.SavedConversation(s.ID()); !v.Available {
		t.Fatal("the replay of a prompt with bus messages and the marker made the saved copy unusable")
	}
}

type failingContinuationConnection struct {
	*testProviderConnection
	failNew, failPrompt bool
}

func (c *failingContinuationConnection) Call(ctx context.Context, method string, p, out any) error {
	if method == "session/new" && c.failNew {
		return errors.New("connection failed")
	}
	if method == "session/prompt" && c.failPrompt {
		return errors.New("connection failed before turn start")
	}
	return c.testProviderConnection.Call(ctx, method, p, out)
}

func TestContinuationFailurePreservesOriginalAndPendingHandoff(t *testing.T) {
	a, c := providerTestApp(t)
	s := continuationSource(t, a)
	failing := &failingContinuationConnection{testProviderConnection: c, failNew: true}
	setTestConnection(a, "codex", failing)
	p, err := a.PreviewSessionContinuation(s.ID(), "codex")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.ContinueSessionWithProvider(s.ID(), "codex", p.Revision); err == nil {
		t.Fatal("creation failure ignored")
	}
	if head, _ := a.ResolveSessionContinuation(s.ID()); head != s.ID() || !s.Open() || s.Switching() {
		t.Fatal("failed switch changed original")
	}
	failing.failNew = false
	result, err := a.ContinueSessionWithProvider(s.ID(), "codex", p.Revision)
	if err != nil {
		t.Fatal(err)
	}
	next := a.session(result.Session.ID)
	failing.failPrompt = true
	a.turnIO().Run(next, promptPointer("Next", []acp.ContentBlock{{Type: "text", Text: "Next"}}))
	if context, _ := a.continuations().PendingContext(next); context != p.Context {
		t.Fatal("failed send lost handoff")
	}
	loaded, err := readTranscript(next.ID())
	if err != nil || !loaded.PendingHandoff || loaded.Handoff != p.Context {
		t.Fatalf("pending handoff not durable: %+v %v", loaded, err)
	}
}

func TestContinuationHistoryWriteFailureQuarantinesNewSession(t *testing.T) {
	a, _ := providerTestApp(t)
	s := continuationSource(t, a)
	p, err := a.PreviewSessionContinuation(s.ID(), "codex")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(transcriptPath("codex:created")+".tmp", 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := a.ContinueSessionWithProvider(s.ID(), "codex", p.Revision); err == nil {
		t.Fatal("history write failure ignored")
	}
	if head, _ := a.ResolveSessionContinuation(s.ID()); head != s.ID() {
		t.Fatal("failed switch hid original")
	}
	if a.session("codex:created") != nil {
		t.Fatal("failed continuation remained open")
	}
	if _, err := a.appletPromptContext("codex:created"); err == nil {
		t.Fatal("failed continuation retained applet capability")
	}
	if !a.hidden("codex:created") {
		t.Fatal("orphan native session was not quarantined")
	}
}

func TestContinuationCycleAndOversizeAreRefused(t *testing.T) {
	a, _ := providerTestApp(t)
	s := continuationSource(t, a)
	// A history far beyond any provider's window is no longer refused: it is
	// summarised to fit, and the preview says so.
	a.transcriptCapture().RecordMessage(s, "agent", strings.Repeat("x", 2<<20), false, false)
	a.transcriptCapture().FinishTurn(s, nil, "end_turn")
	preview, err := a.PreviewSessionContinuation(s.ID(), "codex")
	if err != nil {
		t.Fatalf("an oversize history was refused: %v", err)
	}
	if len(preview.Context) > budgetCodex || !strings.Contains(preview.Context, "earlier message(s) are summarised") {
		t.Fatalf("the handoff was not compacted: %d characters", len(preview.Context))
	}
	found := false
	for _, w := range preview.Warnings {
		if strings.Contains(w, "do not fit the context") {
			found = true
		}
	}
	if !found {
		t.Fatalf("the preview did not say the history was summarised: %v", preview.Warnings)
	}
	tr, err := readTranscript(s.ID())
	if err != nil {
		t.Fatal(err)
	}
	tr.ParentSessionID = s.ID()
	if err := writeTranscript(tr); err != nil {
		t.Fatal(err)
	}
	if _, err := a.ResolveSessionContinuation(s.ID()); err == nil {
		t.Fatal("continuation cycle accepted")
	}
	if filepath.Dir(transcriptPath("../../escape")) != filepath.Join(configDir(), "conversations") {
		t.Fatal("session ID escaped transcript storage")
	}
}

func TestContinuationPrivateChildAndDeletedLeaf(t *testing.T) {
	a, _ := providerTestApp(t)
	s := continuationSource(t, a)
	blocked := false
	a.sink = func(name string, data any) {
		if name == "agent:options" {
			id, _ := data.(map[string]any)["sessionId"].(string)
			if id == "codex:created" {
				blocked = a.Send(id, "Unreviewed premature message", nil, nil) != nil
			}
		}
	}
	p, err := a.PreviewSessionContinuation(s.ID(), "codex")
	if err != nil {
		t.Fatal(err)
	}
	r, err := a.ContinueSessionWithProvider(s.ID(), "codex", p.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if !blocked {
		t.Fatal("child accepted a prompt before durable handoff")
	}
	if err := a.transcriptCapture().Tombstone(r.Session.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := a.ResolveSessionContinuation(s.ID()); err == nil {
		t.Fatal("old link reopened deleted continuation")
	}
	if got := a.continuations().FilterAncestors([]SessionInfo{s.Info(), r.Session}, s.Cwd()); len(got) != 0 {
		t.Fatalf("deleted chat was resurrected: %+v", got)
	}
	tr, err := readTranscript(r.Session.ID)
	if err != nil || !tr.Deleted || tr.ParentSessionID != s.ID() || len(tr.Messages) != 0 || len(tr.ParentHistory) != 0 || tr.Handoff != "" || tr.PendingHandoff {
		t.Fatalf("deleted leaf retained conversation content: %+v %v", tr, err)
	}
	a.transcriptCapture().RecordMessage(a.session(r.Session.ID), "agent", "Late deleted message", false, false)
	a.transcriptCapture().FinishTurn(a.session(r.Session.ID), nil, "end_turn")
	tr, err = readTranscript(r.Session.ID)
	if err != nil || len(tr.Messages) != 0 {
		t.Fatal("late update repopulated deleted transcript")
	}
}

func TestContinuationReplayLimitationsAndInterruptedCapture(t *testing.T) {
	a, _ := providerTestApp(t)
	s := continuationSource(t, a)
	s.SetReplaying(true)
	a.transcriptCapture().Initialize(s, true)
	a.transcriptCapture().RecordUpdate(s, json.RawMessage(`{"sessionUpdate":"user_message_chunk","content":{"type":"image"}}`))
	s.SetReplaying(false)
	a.transcriptCapture().FinishReplay(s, true)
	p, err := a.PreviewSessionContinuation(s.ID(), "codex")
	if err != nil || len(p.Warnings) == 0 {
		t.Fatalf("replayed image omission not disclosed: %v", err)
	}
	a.transcriptCapture().RecordPrompt(s, promptPointer("Interrupted request", []acp.ContentBlock{{Type: "text", Text: "Interrupted request"}}))
	s.SetReplaying(true)
	a.transcriptCapture().Initialize(s, true)
	s.SetReplaying(false)
	a.transcriptCapture().FinishReplay(s, true)
	// The cut-off turn does not block the switch, and the handoff says it.
	if p, err := a.PreviewSessionContinuation(s.ID(), "codex"); err != nil || !slices.Contains(p.Warnings, interruptedTurnNote) {
		t.Fatalf("interrupted capture was not disclosed: %v", err)
	}
	tr, err := readTranscript(s.ID())
	if err != nil || !strings.Contains(tr.Messages[0].Text, "Create a report") {
		t.Fatal("interrupted replay lost captured history")
	}
}

// Every switch keeps a deterministic summary beside the new chat, so a second
// switch can be made when the earlier chats, and their providers, are gone.
func TestASwitchKeepsADeterministicSummary(t *testing.T) {
	a, _ := providerTestApp(t)
	s := continuationSource(t, a)
	preview, err := a.PreviewSessionContinuation(s.ID(), "codex")
	if err != nil {
		t.Fatal(err)
	}
	result, err := a.ContinueSessionWithProvider(s.ID(), "codex", preview.Revision)
	if err != nil {
		t.Fatal(err)
	}
	child, err := readTranscript(result.Session.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(child.Summary, "one line for each message") || !strings.Contains(child.Summary, "- [you, devin] Create a report.") {
		t.Fatalf("the switch kept no summary beside the new chat: %q", child.Summary)
	}
	// The earlier chat is gone entirely, so its saved summary stands in.
	if err := os.Remove(transcriptPath(s.ID())); err != nil {
		t.Fatal(err)
	}
	again, err := a.PreviewSessionContinuation(result.Session.ID, "claude")
	if err != nil {
		t.Fatalf("a second switch without the earlier chat failed: %v", err)
	}
	if !strings.Contains(again.Context, "summary saved beside it") || !strings.Contains(again.Context, "Create a report.") {
		t.Fatalf("the handoff did not use the saved summary: %s", again.Context)
	}
	warned := false
	for _, w := range again.Warnings {
		if strings.Contains(w, "could not be read in full") {
			warned = true
		}
	}
	if !warned {
		t.Fatalf("the preview did not warn about the unreadable history: %v", again.Warnings)
	}
	for _, w := range again.Warnings {
		if strings.Contains(w, "Create a report.") {
			t.Fatalf("the preview repeated the whole summary as a warning: %v", again.Warnings)
		}
	}
	// Starting the new chat's transcript again keeps the summary, and
	// deleting the chat clears it.
	a.transcriptCapture().Initialize(newSessionState(&sessionSeed{ID: result.Session.ID, Provider: "codex", Cwd: s.Cwd()}), true)
	if child, err := readTranscript(result.Session.ID); err != nil || child.Summary == "" {
		t.Fatalf("starting the transcript again lost the summary: %v", err)
	}
	if err := a.transcriptCapture().Tombstone(result.Session.ID); err != nil {
		t.Fatal(err)
	}
	if child, err := readTranscript(result.Session.ID); err != nil || child.Summary != "" {
		t.Fatalf("deleting the chat kept its summary: %v", err)
	}
}

func TestSavedConversationRetainsPromptImages(t *testing.T) {
	a, _ := providerTestApp(t)
	s := continuationSource(t, a)
	image := acp.ContentBlock{Type: "image", MimeType: "image/png", Data: "aW1hZ2U="}
	a.transcriptCapture().RecordPrompt(s, promptPointer("Inspect image", []acp.ContentBlock{{Type: "text", Text: "Inspect image"}, image}))
	saved := a.SavedConversation(s.ID())
	found := false
	for _, message := range saved.Messages {
		for _, got := range message.Images {
			if got.Data == image.Data && got.MimeType == image.MimeType {
				found = true
			}
		}
	}
	if !found {
		t.Fatal("saved history lost the attached image")
	}
}
