package main

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestExitGuardIdleWorkAndCancellation(t *testing.T) {
	app := &App{}
	app.runtime.PutConversationLocked("idle", newSessionState(&sessionSeed{}, sessionInitial{Open: true}))
	app.runtime.PutConversationLocked("active", newSessionState(&sessionSeed{}, sessionInitial{Working: true}))
	prompts := 0
	app.exitPrompt = func(_ context.Context, status exitStatus) bool {
		prompts++
		if status.Chats != 1 || status.Terminals != 0 {
			t.Fatalf("wrong exit status: %+v", status)
		}
		return false
	}
	if app.confirmExitIfNeeded(nil) || prompts != 1 {
		t.Fatal("active exit did not require confirmation")
	}
	if !app.runtime.ConversationLocked("active").Working() {
		t.Fatal("cancelling exit interrupted work")
	}
	app.runtime.ConversationLocked("active").ReleaseWork()
	if !app.confirmExitIfNeeded(nil) || prompts != 1 {
		t.Fatal("idle chats triggered confirmation")
	}
	token, err := app.BeginExitWork("Terminal command")
	if err != nil {
		t.Fatal(err)
	}
	if !app.ExitStatus().needsConfirmation() {
		t.Fatal("terminal command was not counted")
	}
	app.EndExitWork(token)
	app.EndExitWork(token)
	if app.ExitStatus().needsConfirmation() {
		t.Fatal("completed work remained active")
	}
}

func TestExitConfirmationSurvivesOneFlushAndResetsAfterFailure(t *testing.T) {
	app, _ := providerTestApp(t)
	app.runtime.PutConversationLocked("active", newSessionState(&sessionSeed{}, sessionInitial{Working: true}))
	prompts := 0
	app.exitPrompt = func(context.Context, exitStatus) bool { prompts++; return true }
	if !app.confirmExitIfNeeded(nil) || app.requestUIStateFlush() {
		t.Fatal("exit did not start the flush after confirmation")
	}
	id := app.closeRequest
	if err := app.acceptUIStateFlush(id, true); err != nil {
		t.Fatal(err)
	}
	if !app.confirmExitIfNeeded(nil) || !app.requestUIStateFlush() || prompts != 1 {
		t.Fatal("flush acknowledgement caused a duplicate prompt")
	}
	app.abandonClose()
	if !app.confirmExitIfNeeded(nil) || prompts != 2 {
		t.Fatal("abandoned exit reused old consent")
	}
	app.requestUIStateFlush()
	id = app.closeRequest
	app.acceptUIStateFlush(id, false)
	if app.exitConfirmed {
		t.Fatal("failed save retained exit consent")
	}
	app.confirmExitIfNeeded(nil)
	app.requestUIStateFlush()
	app.expireUIStateFlush(app.closeRequest)
	if app.exitConfirmed {
		t.Fatal("timed-out save retained exit consent")
	}
}

func TestExitGuardChecksWorkThatStartedDuringFlush(t *testing.T) {
	app, _ := providerTestApp(t)
	if !app.confirmExitIfNeeded(nil) || app.requestUIStateFlush() {
		t.Fatal("idle close did not begin saving")
	}
	app.acceptUIStateFlush(app.closeRequest, true)
	app.runtime.PutConversationLocked("new", newSessionState(&sessionSeed{}, sessionInitial{Working: true}))
	prompted := false
	app.exitPrompt = func(context.Context, exitStatus) bool { prompted = true; return false }
	if app.confirmExitIfNeeded(nil) || !prompted || app.closeApproved {
		t.Fatal("new work bypassed the final confirmation or reused save approval")
	}
}

func TestExitCallAccountingExcludesQueriesAndCloseProtocol(t *testing.T) {
	app := &App{}
	for _, name := range []string{"ExitStatus", "NativeWindowState", "NativeWindowAction", "AckUIStateFlush", "BeginExitWork", "ReadWorkspaceMount"} {
		if app.observeExitCall(name) != nil {
			t.Fatalf("recoverable call %s was counted", name)
		}
	}
	done := app.observeExitCall("SaveDraft")
	if done == nil || !app.ExitStatus().needsConfirmation() {
		t.Fatal("unfinished write was not tracked")
	}
	done()
	done()
	if app.ExitStatus().needsConfirmation() {
		t.Fatal("completed write remained active")
	}
}

// A page that missed the confirmation reads the same request, with the status
// it was asked with and not the status now, until it is answered or expires.
func TestPendingExitConfirmationIsWhatTheEventCarried(t *testing.T) {
	app, _ := providerTestApp(t)
	app.runtime.PutConversationLocked("active", newSessionState(&sessionSeed{}, sessionInitial{Working: true}))
	var event any
	app.sink = func(name string, data any) {
		if name == "dinah:exit-confirmation" {
			event = data
		}
	}
	if app.PendingExitConfirmation() != nil {
		t.Fatal("a confirmation is pending before one was asked")
	}
	if app.confirmExitIfNeeded(nil) {
		t.Fatal("busy close bypassed the confirmation")
	}
	pending := app.PendingExitConfirmation()
	if pending == nil || pending.ID != app.exitRequest {
		t.Fatalf("pending = %+v", pending)
	}
	same := func() {
		t.Helper()
		got, _ := json.Marshal(app.PendingExitConfirmation())
		want, _ := json.Marshal(event)
		if string(got) != string(want) || !strings.Contains(string(got), `"status":{"chats":1`) {
			t.Fatalf("pending %s differs from the event %s", got, want)
		}
	}
	same()
	// Work that started after the question was asked does not change it.
	app.runtime.PutConversationLocked("another", newSessionState(&sessionSeed{}, sessionInitial{Working: true}))
	same()
	if err := app.acceptExitConfirmation(pending.ID, false); err != nil {
		t.Fatal(err)
	}
	if app.PendingExitConfirmation() != nil {
		t.Fatal("an answered confirmation is still pending")
	}
	app.confirmExitIfNeeded(nil)
	again := app.PendingExitConfirmation()
	if again == nil || again.ID == pending.ID {
		t.Fatalf("a second confirmation: %+v", again)
	}
	app.expireExitConfirmation(again.ID)
	if app.PendingExitConfirmation() != nil {
		t.Fatal("an expired confirmation is still pending")
	}
}

func TestExitConfirmationHandshakeRejectsStaleRepliesAndExpires(t *testing.T) {
	app, _ := providerTestApp(t)
	app.runtime.PutConversationLocked("active", newSessionState(&sessionSeed{}, sessionInitial{Working: true}))
	emitted := 0
	app.sink = func(name string, _ any) {
		if name == "dinah:exit-confirmation" {
			emitted++
		}
	}
	if app.confirmExitIfNeeded(nil) {
		t.Fatal("busy native close bypassed the page confirmation")
	}
	first := app.exitRequest
	if first == "" || app.confirmExitIfNeeded(nil) || emitted != 1 {
		t.Fatal("repeat native close duplicated the confirmation")
	}
	if app.acceptExitConfirmation("stale", true) == nil {
		t.Fatal("stale confirmation accepted")
	}
	if err := app.acceptExitConfirmation(first, false); err != nil {
		t.Fatal(err)
	}
	if app.exitConfirmed || app.exitConfirming || !app.runtime.ConversationLocked("active").Working() {
		t.Fatal("Cancel changed running work or kept consent")
	}
	app.confirmExitIfNeeded(nil)
	second := app.exitRequest
	if second == first {
		t.Fatal("confirmation identity was reused")
	}
	app.expireExitConfirmation(first)
	if app.exitRequest != second {
		t.Fatal("stale timeout expired a newer confirmation")
	}
	app.expireExitConfirmation(second)
	if app.exitConfirmed || app.exitConfirming || app.exitTimer != nil {
		t.Fatal("timeout did not cancel exit")
	}
	app.confirmExitIfNeeded(nil)
	if err := app.acceptExitConfirmation(app.exitRequest, true); err != nil {
		t.Fatal(err)
	}
	if !app.confirmExitIfNeeded(nil) || app.requestUIStateFlush() {
		t.Fatal("confirmed exit did not proceed to layout saving")
	}
	app.expireUIStateFlush(app.closeRequest)
}

// A restart that is the way out of the program, and a read that can run long,
// are not unfinished work: counted while they ran, the restart would warn
// about itself and a chat still reading its history would look busy. A call that
// changes something still counts.
func TestARestartAndAReadAreNotUnfinishedWork(t *testing.T) {
	app := &App{}
	for _, name := range []string{"RestartToUpdate", "ChatHistory", "ChatImageInput", "Notifications", "InstalledPackages", "PlanPackages"} {
		if done := app.observeExitCall(name); done != nil {
			t.Errorf("%s is counted as unfinished work", name)
		}
		if app.ExitStatus().needsConfirmation() {
			t.Errorf("%s running asks for a confirmation to exit", name)
		}
	}
	done := app.observeExitCall("Send")
	if done == nil || !app.ExitStatus().needsConfirmation() {
		t.Fatal("a call that changes something is not counted")
	}
	done()
	if app.ExitStatus().needsConfirmation() {
		t.Fatal("a finished call is still counted")
	}
}

// Closing the window for an update, with nothing running, exits without asking
// again: the restart request is still running when the window checks.
func TestARestartRequestedWithNothingRunningExitsWithoutAskingAgain(t *testing.T) {
	app, _ := providerTestApp(t)
	asked := 0
	app.exitPrompt = func(context.Context, exitStatus) bool { asked++; return true }
	done := app.observeExitCall("RestartToUpdate")
	if done != nil {
		defer done()
	}
	if !app.confirmExitIfNeeded(nil) || asked != 0 {
		t.Fatalf("the restart asked to confirm exit %d times", asked)
	}
}
