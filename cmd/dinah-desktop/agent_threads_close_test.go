package main

import (
	"errors"
	"testing"
	"time"
)

// windowedChat puts an open chat with a window into the App, with no wait
// before a closed window is looked at.
func windowedChat(t *testing.T, a *App, id string, initial sessionInitial) *Session {
	t.Helper()
	a.windowCloseSettle = -1
	s := newSessionState(&sessionSeed{ID: id, Provider: providerForSession(id), Cwd: t.TempDir()}, initial)
	a.mu.Lock()
	_, windows := a.runtime.WindowsLocked()
	a.runtime.PutConversationLocked(s.ID(), s)
	a.mu.Unlock()
	ids := []string{id}
	for other := range windows {
		ids = append(ids, other)
	}
	a.ChatWindows(ids)
	return s
}

// closeWindows reports that no chat has a window any more, and waits for the
// looks at the closed chats to end.
func closeWindows(a *App) {
	a.ChatWindows(nil)
	a.windowCloses.Wait()
}

// With the setting on, closing the last window of an idle chat ends its
// session and archives its thread then, without waiting four hours, and the
// thread is taken out of the archive before the chat is loaded again.
func TestClosingAnIdleChatsWindowArchivesItsThreadWithTheSettingOn(t *testing.T) {
	a, c := threadTestApp(t)
	if err := a.SetCodexArchiveClosed(true); err != nil {
		t.Fatal(err)
	}
	const id = "codex:shut"
	s := windowedChat(t, a, id, sessionInitial{Open: true})
	if got := c.lines("session/close", "archive"); len(got) != 0 {
		t.Fatalf("a chat with a window was ended: %v", got)
	}
	closeWindows(a)
	if got := c.lines("session/close", "archive"); !equalLines(got, "session/close", "archive "+id) {
		t.Fatalf("the connection was asked %v", got)
	}
	if a.session(id) != nil {
		t.Fatal("the chat's session is still loaded")
	}
	if archived, _ := threadRecords(a, id); !archived {
		t.Fatal("the archived thread was not recorded")
	}
	if _, err := a.OpenSessionAt(s.Cwd(), id); err != nil {
		t.Fatalf("the archived chat did not open: %v", err)
	}
	if got := c.lines("archive", "unarchive", "session/load"); !equalLines(got, "archive "+id, "unarchive "+id, "session/load") {
		t.Fatalf("the connection was asked %v", got)
	}
	if archived, _ := threadRecords(a, id); archived {
		t.Fatal("the thread is still recorded as archived")
	}
}

// With the setting off, closing a chat's window asks the agent nothing, and
// the chat keeps its session, as before.
func TestClosingAChatsWindowChangesNothingWithTheSettingOff(t *testing.T) {
	a, c := threadTestApp(t)
	const id = "codex:kept"
	windowedChat(t, a, id, sessionInitial{Open: true})
	closeWindows(a)
	if got := c.lines("session/close", "archive"); len(got) != 0 {
		t.Fatalf("the connection was asked %v", got)
	}
	if s := a.session(id); s == nil || !s.Open() {
		t.Fatal("the chat lost its session")
	}
	// A chat of a provider without the setting is left alone when another
	// provider's setting is on.
	if err := a.SetCodexArchiveClosed(true); err != nil {
		t.Fatal(err)
	}
	a.ChatWindows([]string{id})
	windowedChat(t, a, "claude:other", sessionInitial{Open: true})
	a.ChatWindows([]string{id})
	a.windowCloses.Wait()
	if s := a.session("claude:other"); s == nil || !s.Open() {
		t.Fatal("a chat of another provider lost its session")
	}
	if got := c.lines("session/close", "archive"); len(got) != 0 {
		t.Fatalf("the connection was asked %v", got)
	}
}

// A chat that is working when its window closes keeps its session, and so
// does one that a task or a service chat holds. The held chat is ended and
// archived by the ordinary path once it has been closed for long enough.
func TestClosingTheWindowOfABusyChatLeavesItToTheOrdinaryPath(t *testing.T) {
	a, c := threadTestApp(t)
	if err := a.SetCodexArchiveClosed(true); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 11, 22, 0, 0, 0, 0, time.UTC)
	a.wakeNow = func() time.Time { return now }
	windowedChat(t, a, "codex:working", sessionInitial{Open: true, Working: true})
	held := windowedChat(t, a, "codex:held", sessionInitial{Open: true})
	release, err := held.HoldRuntime()
	if err != nil {
		t.Fatal(err)
	}
	closeWindows(a)
	if got := c.lines("session/close", "archive"); len(got) != 0 {
		t.Fatalf("a busy chat was ended: %v", got)
	}
	for _, id := range []string{"codex:working", "codex:held"} {
		if s := a.session(id); s == nil || !s.Open() {
			t.Fatalf("chat %s lost its session", id)
		}
	}
	release()
	a.closedSessions().EndClosed()
	if got := c.lines("session/close", "archive"); len(got) != 0 {
		t.Fatalf("the held chat was ended before it was due: %v", got)
	}
	now = now.Add(closedSessionEnd)
	a.closedSessions().EndClosed()
	a.closedSessions().Wait()
	if got := c.lines("session/close", "archive"); !equalLines(got, "session/close", "archive codex:held") {
		t.Fatalf("the connection was asked %v", got)
	}
	if s := a.session("codex:working"); s == nil || !s.Open() {
		t.Fatal("the working chat lost its session")
	}
}

// A chat that a message waits for keeps its session when its window closes,
// so the message is delivered to it as to any chat that is loaded.
func TestClosingTheWindowOfAChatWithAMessageWaitingDoesNotArchiveIt(t *testing.T) {
	a, c := threadTestApp(t)
	if err := a.SetCodexArchiveClosed(true); err != nil {
		t.Fatal(err)
	}
	const id = "codex:asked"
	closedChat(t, a, id, "Asked Chat")
	if waits, err := a.messageWaitsFor(id); err != nil || !waits {
		t.Fatalf("the waiting message was not seen: %v %v", waits, err)
	}
	windowedChat(t, a, id, sessionInitial{Open: true})
	closeWindows(a)
	if got := c.lines("session/close", "archive"); len(got) != 0 {
		t.Fatalf("a chat with a message waiting was ended: %v", got)
	}
	if s := a.session(id); s == nil || !s.Open() {
		t.Fatal("the chat lost its session")
	}
	if archived, waiting := threadRecords(a, id); archived || waiting {
		t.Fatalf("the record says archived %v, waiting %v", archived, waiting)
	}
}

// Closing a window returns at once, whatever the agent does with the request
// to close the session. A thread that could not be archived is logged and
// nothing else changes. A chat whose window is back before the wait is over
// keeps its session.
func TestClosingAWindowNeverWaitsForTheAgent(t *testing.T) {
	a, c := threadTestApp(t)
	if err := a.SetCodexArchiveClosed(true); err != nil {
		t.Fatal(err)
	}
	const id = "codex:slowclose"
	windowedChat(t, a, id, sessionInitial{Open: true})
	gate, asked := make(chan struct{}), make(chan struct{}, 1)
	c.mu.Lock()
	c.onCall = func(method string) {
		if method == "session/close" {
			asked <- struct{}{}
			<-gate
		}
	}
	c.mu.Unlock()
	c.failing("archive", errors.New("stand-in refusal"))
	returned := make(chan struct{})
	go func() { a.ChatWindows(nil); close(returned) }()
	select {
	case <-returned:
	case <-time.After(testWait):
		t.Fatal("the report of the closed window waited")
	}
	select {
	case <-asked:
	case <-t.Context().Done():
		t.Fatal("the agent was not asked to close the session")
	}
	close(gate)
	a.windowCloses.Wait()
	if a.session(id) != nil {
		t.Fatal("the chat's session is still loaded")
	}
	if got := c.lines("archive"); !equalLines(got, "archive "+id) {
		t.Fatalf("the connection was asked %v", got)
	}
	if archived, waiting := threadRecords(a, id); archived || waiting {
		t.Fatalf("a refused archive was recorded: archived %v, waiting %v", archived, waiting)
	}

	// A window that is back within the wait keeps the chat's session.
	const back = "codex:back"
	windowedChat(t, a, back, sessionInitial{Open: true})
	a.windowCloseSettle = 200 * time.Millisecond
	a.ChatWindows(nil)
	a.ChatWindows([]string{back})
	a.windowCloses.Wait()
	if s := a.session(back); s == nil || !s.Open() {
		t.Fatal("a chat whose window came back lost its session")
	}
	if got := c.lines("session/close"); !equalLines(got, "session/close") {
		t.Fatalf("the connection was asked %v", got)
	}
}

// After shutdown has stopped the looks, a closed window ends no session.
func TestNoSessionIsEndedForAWindowClosedAtShutdown(t *testing.T) {
	a, c := threadTestApp(t)
	if err := a.SetCodexArchiveClosed(true); err != nil {
		t.Fatal(err)
	}
	const id = "codex:quitting"
	windowedChat(t, a, id, sessionInitial{Open: true})
	a.stopWindowCloses()
	closeWindows(a)
	if got := c.lines("session/close", "archive"); len(got) != 0 {
		t.Fatalf("the connection was asked %v", got)
	}
}
