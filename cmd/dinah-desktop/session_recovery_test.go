package main

import (
	"bytes"
	"os"
	"slices"
	"strings"
	"testing"
)

func TestRecoveryPreservesUnsentContinuationAndCreatesFreshThread(t *testing.T) {
	a, c := providerTestApp(t)
	parent := continuationSource(t, a)
	a.cmu.Lock()
	parentHistory := parent.HistorySavedLocked()
	a.cmu.Unlock()
	tr := &conversationTranscript{SessionID: "codex:missing", Provider: "codex", Workspace: parent.Cwd(), ParentSessionID: parent.ID(), ParentHistory: append([]ConversationMessage{}, parentHistory.Messages...), PendingHandoff: true, Complete: true, Messages: []ConversationMessage{}}
	if err := writeTranscript(tr); err != nil {
		t.Fatal(err)
	}
	original, err := os.ReadFile(transcriptPath(tr.SessionID))
	if err != nil {
		t.Fatal(err)
	}
	view := a.SavedConversation(tr.SessionID)
	if !view.Available || !view.Complete || len(view.Messages) != len(parentHistory.Messages) {
		t.Fatalf("an unsent continuation hid its saved history: %+v", view)
	}
	preview, err := a.PreviewSessionRecovery(tr.SessionID, "codex")
	if err != nil || !strings.Contains(preview.Context, "The report is saved.") {
		t.Fatalf("offline preview: %v", err)
	}
	if _, err := a.RecoverSession(tr.SessionID, "codex", "stale"); err == nil {
		t.Fatal("a stale recovery preview was accepted")
	}
	result, err := a.RecoverSession(tr.SessionID, "codex", preview.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if result.Session.ID == tr.SessionID || result.ParentSessionID != tr.SessionID {
		t.Fatalf("recovery reused the failed thread: %+v", result)
	}
	for _, call := range c.calls {
		if call.method == "session/prompt" || call.method == "session/load" {
			t.Fatal("recovery contacted the failed thread or ran a model turn")
		}
	}
	after, _ := os.ReadFile(transcriptPath(tr.SessionID))
	if !bytes.Equal(original, after) {
		t.Fatal("recovery rewrote the original history")
	}
	if head, err := a.ResolveSessionContinuation(tr.SessionID); err != nil || head != result.Session.ID {
		t.Fatalf("the failed chat does not resolve to its recovery: %s %v", head, err)
	}
	if handoff, err := a.continuations().PendingContext(a.session(result.Session.ID)); err != nil || handoff != preview.Context {
		t.Fatalf("recovery lost the reviewed context: %v", err)
	}
}

// A chat whose last turn was cut off, as when Dinah closes during a turn,
// can be recovered: the copy holds everything before the turn's end, it is
// still shown as incomplete, and the handoff says the end is missing.
func TestACutOffTurnCanBeRecoveredWithANote(t *testing.T) {
	for _, captureError := range []string{"", errInterruptedTurn} {
		a, _ := providerTestApp(t)
		tr := &conversationTranscript{SessionID: "codex:partial", Provider: "codex", Workspace: t.TempDir(), CaptureError: captureError, Messages: []ConversationMessage{{Role: "you", Text: "Keep this partial record."}}}
		if err := writeTranscript(tr); err != nil {
			t.Fatal(err)
		}
		view := a.SavedConversation(tr.SessionID)
		if !view.Available || view.Complete || !view.Recoverable || !strings.Contains(view.Warning, "cut off") || len(view.Messages) != 1 {
			t.Fatalf("a cut-off turn was misrepresented: %+v", view)
		}
		preview, err := a.PreviewSessionRecovery(tr.SessionID, "codex")
		if err != nil {
			t.Fatalf("a cut-off turn could not be recovered: %v", err)
		}
		if !slices.Contains(preview.Warnings, interruptedTurnNote) || !strings.Contains(preview.Context, "Keep this partial record.") {
			t.Fatalf("the handoff does not say the turn was cut off: %+v", preview.Warnings)
		}
	}
}

// Any other fault in capturing the history still refuses recovery.
func TestAnIncompleteCopyIsStillRefused(t *testing.T) {
	a, _ := providerTestApp(t)
	tr := &conversationTranscript{SessionID: "codex:partial", Provider: "codex", Workspace: t.TempDir(), Complete: true, CaptureError: errOutsideHistory, Messages: []ConversationMessage{{Role: "you", Text: "Keep this partial record."}}}
	if err := writeTranscript(tr); err != nil {
		t.Fatal(err)
	}
	if view := a.SavedConversation(tr.SessionID); !view.Available || view.Complete || view.Recoverable {
		t.Fatalf("an incomplete copy was offered for recovery: %+v", view)
	}
	if _, err := a.PreviewSessionRecovery(tr.SessionID, "codex"); err == nil {
		t.Fatal("an incomplete copy was accepted as recovery context")
	}
}
