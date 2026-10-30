package main

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"studio/internal/acp"
	"studio/internal/bus"
	"studio/internal/codexagent"
	"studio/internal/sessionhost"
)

// withResume has the test agent advertise session/resume.
func withResume(a *App) {
	initial := a.runtime.ProviderLocked("codex").Initialization()
	initial.AgentCapabilities.SessionCapabilities.Resume = &struct{}{}
	setTestInitialization(a, "codex", initial)
}

// withoutResume has the test agent stop advertising it.
func withoutResume(a *App) {
	initial := a.runtime.ProviderLocked("codex").Initialization()
	initial.AgentCapabilities.SessionCapabilities.Resume = nil
	setTestInitialization(a, "codex", initial)
}

// wholeCopy writes the saved copy of a closed chat and returns its ID. The
// copy is whole unless the test changes it.
func wholeCopy(t *testing.T, ws string, change func(*sessionhost.Transcript)) string {
	t.Helper()
	id := "codex:resumed"
	tr := &sessionhost.Transcript{SessionID: id, Provider: "codex", Workspace: ws, Complete: true, Messages: []sessionhost.ConversationMessage{
		{Role: "you", Text: "Hello", Provider: "codex", SessionID: id},
		{Role: "agent", Text: "Hi there", Provider: "codex", SessionID: id},
	}}
	if change != nil {
		change(tr)
	}
	if err := sessionhost.WriteTranscript(configDir(), tr); err != nil {
		t.Fatal(err)
	}
	return id
}

func readSaved(t *testing.T, id string) *sessionhost.Transcript {
	t.Helper()
	tr, err := sessionhost.ReadTranscript(configDir(), id)
	if err != nil {
		t.Fatal(err)
	}
	return tr
}

// closeChat leaves the chat saved and not open, as after a restart.
func closeChat(a *App, id string) { a.runtime.RemoveConversationLocked(id) }

func TestAWholeCopyIsResumedWithoutAReplay(t *testing.T) {
	a, c := providerTestApp(t)
	withResume(a)
	ws := t.TempDir()
	id := wholeCopy(t, ws, nil)
	info, err := a.OpenSessionAt(ws, id)
	if err != nil {
		t.Fatal(err)
	}
	if countCalls(c, "session/resume") != 1 || countCalls(c, "session/load") != 0 {
		t.Fatalf("the chat was opened with %d resumes and %d loads", countCalls(c, "session/resume"), countCalls(c, "session/load"))
	}
	if !info.Resumed {
		t.Fatalf("the open did not say the chat was resumed: %+v", info)
	}
	if state := a.session(id).State(); state.Replaying || state.Restoring {
		t.Fatalf("the resumed chat is still loading: %+v", state)
	}
	kept := readSaved(t, id)
	if !kept.Complete || len(kept.Messages) != 2 || kept.Messages[1].Text != "Hi there" {
		t.Fatalf("the saved copy was not kept: %+v", kept)
	}
	view := a.SavedConversation(id)
	if view.StreamGen == "" || view.StreamThrough != 0 {
		t.Fatalf("a resumed chat is not at (generation, 0): %q %d", view.StreamGen, view.StreamThrough)
	}
	if view.StreamGen != info.StreamGen {
		t.Fatalf("the saved copy is for generation %q, the chat is in %q", view.StreamGen, info.StreamGen)
	}
	if len(view.Messages) != 2 {
		t.Fatalf("the page is given %d messages", len(view.Messages))
	}
}

func TestWhatPreventsAResumeTakesTheLoad(t *testing.T) {
	cases := []struct {
		name   string
		resume bool
		change func(*sessionhost.Transcript)
		none   bool
	}{
		{name: "the agent does not advertise resume", resume: false},
		{name: "there is no saved copy", resume: true, none: true},
		{name: "the copy is not complete", resume: true, change: func(tr *sessionhost.Transcript) { tr.Complete = false }},
		{name: "the copy has a capture error", resume: true, change: func(tr *sessionhost.Transcript) { tr.CaptureError = "cut" }},
		{name: "the copy was deleted", resume: true, change: func(tr *sessionhost.Transcript) { tr.Deleted = true }},
		{name: "the copy is another provider's", resume: true, change: func(tr *sessionhost.Transcript) { tr.Provider = "devin" }},
		{name: "the copy is another workspace's", resume: true, change: func(tr *sessionhost.Transcript) { tr.Workspace = t.TempDir() }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a, c := providerTestApp(t)
			if tc.resume {
				withResume(a)
			}
			ws := t.TempDir()
			id := "codex:resumed"
			if !tc.none {
				id = wholeCopy(t, ws, tc.change)
			}
			info, err := a.OpenSessionAt(ws, id)
			if err != nil {
				t.Fatal(err)
			}
			if countCalls(c, "session/load") != 1 || countCalls(c, "session/resume") != 0 || info.Resumed {
				t.Fatalf("the chat was opened with %d loads and %d resumes, resumed %v", countCalls(c, "session/load"), countCalls(c, "session/resume"), info.Resumed)
			}
		})
	}
}

// The decision is read from the agent's answer at each reopen.
func TestTheResumeDecisionIsMadeAtEachReopen(t *testing.T) {
	a, c := providerTestApp(t)
	ws := t.TempDir()
	id := wholeCopy(t, ws, nil)
	if _, err := a.OpenSessionAt(ws, id); err != nil {
		t.Fatal(err)
	}
	if countCalls(c, "session/load") != 1 {
		t.Fatal("the first reopen did not load")
	}
	closeChat(a, id)
	withResume(a)
	if info, err := a.OpenSessionAt(ws, id); err != nil || !info.Resumed {
		t.Fatalf("the second reopen was not a resume: %+v %v", info, err)
	}
	closeChat(a, id)
	withoutResume(a)
	if info, err := a.OpenSessionAt(ws, id); err != nil || info.Resumed {
		t.Fatalf("the third reopen resumed an agent that no longer advertises it: %+v %v", info, err)
	}
	if countCalls(c, "session/resume") != 1 || countCalls(c, "session/load") != 2 {
		t.Fatalf("%d resumes and %d loads", countCalls(c, "session/resume"), countCalls(c, "session/load"))
	}
}

func TestAResumeRefusedForAnyOtherReasonFallsBackToALoad(t *testing.T) {
	a, c := providerTestApp(t)
	withResume(a)
	c.resumeErr = &acp.Error{Code: acp.CodeMethodNotFound, Message: "Method not found"}
	ws := t.TempDir()
	id := wholeCopy(t, ws, nil)
	info, err := a.OpenSessionAt(ws, id)
	if err != nil {
		t.Fatal(err)
	}
	if info.Resumed || countCalls(c, "session/resume") != 1 || countCalls(c, "session/load") != 1 {
		t.Fatalf("resumed %v after %d resumes and %d loads", info.Resumed, countCalls(c, "session/resume"), countCalls(c, "session/load"))
	}
	if state := a.session(id).State(); state.Replaying {
		t.Fatalf("the chat is still replaying: %+v", state)
	}
}

func TestAResumeAnsweredNotFoundIsAFailedReopenWithTheCopyKept(t *testing.T) {
	a, c := providerTestApp(t)
	withResume(a)
	c.resumeErr = &acp.Error{Code: acp.CodeResourceNotFound, Message: "Resource not found"}
	ws := t.TempDir()
	id := wholeCopy(t, ws, nil)
	_, err := a.OpenSessionAt(ws, id)
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("the reopen did not fail: %v", err)
	}
	if countCalls(c, "session/load") != 0 {
		t.Fatal("a session the agent does not have was loaded")
	}
	if a.session(id) != nil {
		t.Fatal("the failed chat is still open")
	}
	if kept := readSaved(t, id); !kept.Complete || len(kept.Messages) != 2 {
		t.Fatalf("the saved copy was changed: %+v", kept)
	}
}

// A turn cut off by a crash leaves a copy not marked complete, which takes the
// load once and is whole afterwards.
func TestACutOffTurnIsLoadedOnceAndThenResumed(t *testing.T) {
	a, c := providerTestApp(t)
	withResume(a)
	ws := t.TempDir()
	id := wholeCopy(t, ws, func(tr *sessionhost.Transcript) {
		tr.Complete = false
		tr.Messages = append(tr.Messages, sessionhost.ConversationMessage{Role: "you", Text: "And then?", Provider: "codex", SessionID: tr.SessionID})
	})
	// The agent has the answer the crash cut off.
	c.onLoad = func() {
		s := a.session(id)
		for _, m := range [][2]string{{"you", "Hello"}, {"agent", "Hi there"}, {"you", "And then?"}, {"agent", "Then this."}} {
			a.transcriptCapture().RecordMessage(s, m[0], m[1], false, false)
		}
	}
	if info, err := a.OpenSessionAt(ws, id); err != nil || info.Resumed {
		t.Fatalf("the cut-off chat was resumed: %+v %v", info, err)
	}
	if countCalls(c, "session/load") != 1 {
		t.Fatal("the cut-off chat was not loaded")
	}
	kept := readSaved(t, id)
	if !kept.Complete || kept.CaptureError != "" || len(kept.Messages) != 4 || kept.Messages[3].Text != "Then this." {
		t.Fatalf("the load did not replace the copy with the turn in it: %+v", kept)
	}
	closeChat(a, id)
	if info, err := a.OpenSessionAt(ws, id); err != nil || !info.Resumed {
		t.Fatalf("the chat was not resumed once its copy was whole: %+v %v", info, err)
	}
}

func TestLoadFullHistoryReopensWithALoadAsANewGeneration(t *testing.T) {
	a, c := providerTestApp(t)
	withResume(a)
	withSessionClose(a)
	ws := t.TempDir()
	id := wholeCopy(t, ws, nil)
	first, err := a.OpenSessionAt(ws, id)
	if err != nil || !first.Resumed {
		t.Fatalf("%+v %v", first, err)
	}
	info, err := a.LoadFullHistory(ws, id)
	if err != nil {
		t.Fatal(err)
	}
	if info.Resumed || countCalls(c, "session/load") != 1 || countCalls(c, "session/close") != 1 {
		t.Fatalf("resumed %v after %d loads and %d closes", info.Resumed, countCalls(c, "session/load"), countCalls(c, "session/close"))
	}
	if info.StreamGen == first.StreamGen {
		t.Fatal("loading the full history kept the chat's log generation")
	}
	if state := a.session(id).State(); state.Replaying {
		t.Fatalf("the full history is still replaying: %+v", state)
	}
}

func TestLoadFullHistoryRefusesWhileTheChatIsWorking(t *testing.T) {
	a, c := providerTestApp(t)
	withResume(a)
	ws := t.TempDir()
	id := wholeCopy(t, ws, nil)
	if _, err := a.OpenSessionAt(ws, id); err != nil {
		t.Fatal(err)
	}
	s := a.session(id)
	s.StartWork()
	if !s.Working() {
		t.Fatal("the chat is not working")
	}
	if _, err := a.LoadFullHistory(ws, id); err == nil {
		t.Fatal("the full history was loaded while the chat was working")
	}
	if a.session(id) != s || countCalls(c, "session/load") != 0 {
		t.Fatal("the working chat was closed")
	}
}

// A message waiting for a chat that was resumed is delivered at once. It is not
// announced as arriving, which is for a chat still replaying.
func TestAMessageWaitingForAResumedChatIsDeliveredAtOnce(t *testing.T) {
	a, c := providerTestApp(t)
	withResume(a)
	r := watchConnects(a)
	var mu sync.Mutex
	arriving := 0
	inner := a.sink
	a.sink = func(name string, data any) {
		if name == "agent:arriving" {
			mu.Lock()
			arriving++
			mu.Unlock()
		}
		inner(name, data)
	}
	busID, cwd := closedChat(t, a, "codex:waiting", "Waiting Chat")
	err := a.busHost().With(func(b *bus.Bus) error {
		_, err := b.CreateGroup(context.Background(), bus.Sender{}, "Waiting Team", []string{busID})
		if err != nil {
			return err
		}
		_, err = b.SendMentioning(context.Background(), bus.Sender{}, "#Waiting Team", "Back again?", nil, 0, []string{busID})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	a.ChatWindows(nil)
	tr := &sessionhost.Transcript{SessionID: "codex:waiting", Provider: "codex", Workspace: cwd, Complete: true, Messages: []sessionhost.ConversationMessage{
		{Role: "you", Text: "Hello", Provider: "codex", SessionID: "codex:waiting"},
	}}
	if err := sessionhost.WriteTranscript(configDir(), tr); err != nil {
		t.Fatal(err)
	}
	wakeRound(t, a)
	openAsAsked(t, a, r.take())
	ring(t, a)
	waitTurn(t, r.turns)
	if countCalls(c, "session/resume") != 1 || countCalls(c, "session/load") != 0 {
		t.Fatalf("%d resumes and %d loads", countCalls(c, "session/resume"), countCalls(c, "session/load"))
	}
	if got := prompts(c); len(got) != 1 || !strings.Contains(got[0], "Back again?") {
		t.Fatalf("the waiting message was not delivered: %q", got)
	}
	mu.Lock()
	defer mu.Unlock()
	if arriving != 0 {
		t.Fatalf("a resumed chat announced %d messages as arriving", arriving)
	}
}

// A message sent while a chat is still loading its history waits in the chat's
// queue and goes to the agent when the load ends.
func TestAMessageSentWhileAChatLoadsWaitsAndIsSentWhenItEnds(t *testing.T) {
	a, c := providerTestApp(t)
	r := watchConnects(a)
	ws := t.TempDir()
	id := wholeCopy(t, ws, nil)
	entered, release := make(chan struct{}), make(chan struct{})
	c.onLoad = func() {
		close(entered)
		<-release
	}
	opened := make(chan error, 1)
	go func() {
		_, err := a.OpenSessionAt(ws, id)
		opened <- err
	}()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("the load did not start")
	}
	if err := a.Send(id, "Sent during the load", nil, nil); err != nil {
		t.Fatalf("a message sent during the load was refused: %v", err)
	}
	if queued := a.session(id).Info().Queued; len(queued) != 1 || queued[0] != "Sent during the load" {
		t.Fatalf("the message is not shown as waiting: %v", queued)
	}
	close(release)
	if err := <-opened; err != nil {
		t.Fatal(err)
	}
	waitTurn(t, r.turns)
	got := prompts(c)
	if len(got) != 1 || !strings.Contains(got[0], "Sent during the load") {
		t.Fatalf("the waiting message was not sent when the load ended: %q", got)
	}
	if queued := a.session(id).Info().Queued; len(queued) != 0 {
		t.Fatalf("the message is still queued: %v", queued)
	}
}

// A message the person writes to a group that mentions everyone opens a closed
// member in the background and asks it; one that mentions nobody does not, any
// more than a chat's does.
func TestThePersonsMessageToEveryoneOpensAClosedMember(t *testing.T) {
	a, _ := providerTestApp(t)
	r := watchConnects(a)
	id, cwd := "codex:sleeper", t.TempDir()
	busID := bus.WindowSessionPrefix + "sleeper"
	a.saveBusSession(id, busID)
	var other bus.Session
	var group bus.Room
	err := a.busHost().With(func(b *bus.Bus) error {
		sleeper, err := b.RegisterSession(context.Background(), busID, cwd, "Sleeper")
		if err != nil {
			return err
		}
		other, err = b.RegisterSession(context.Background(), bus.WindowSessionPrefix+"other", cwd, "Other")
		if err != nil {
			return err
		}
		group, err = b.CreateGroup(context.Background(), bus.Sender{}, "Sleepers", []string{sleeper.ID, other.ID})
		if err != nil {
			return err
		}
		return b.EndSession(context.Background(), busID)
	})
	if err != nil {
		t.Fatal(err)
	}
	a.ChatWindows(nil)
	// The window opens a group's members when it starts, which is not what is
	// tested here.
	wakeRound(t, a)
	r.take()
	send := func(from *bus.Session, text string, mentions ...string) {
		t.Helper()
		sender := bus.Sender{}
		if from != nil {
			sender = bus.Sender{Session: from}
		}
		err := a.busHost().With(func(b *bus.Bus) error {
			_, err := b.SendMentioning(context.Background(), sender, "#"+group.Name, text, nil, 0, mentions)
			return err
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	send(&other, "A chat speaking to nobody")
	wakeRound(t, a)
	if asked := askedIDs(r.take()); len(asked) != 0 {
		t.Fatalf("a chat's message that mentions nobody opened %v", asked)
	}
	send(nil, "Is anyone there?")
	wakeRound(t, a)
	if asked := askedIDs(r.take()); len(asked) != 0 {
		t.Fatalf("the person's message that mentions nobody opened %v", asked)
	}
	send(nil, "Is anyone there?", bus.Everyone)
	wakeRound(t, a)
	if asked := askedIDs(r.take()); len(asked) != 1 || asked[0] != id {
		t.Fatalf("the person's message to everyone did not open the closed member: %v", asked)
	}
}

// A chat the page was asked to connect and did not is opened by the engine
// once the grace has passed, and the messages waiting for it are delivered.
func TestAChatTheWindowDoesNotOpenIsOpenedByTheEngine(t *testing.T) {
	a, c := providerTestApp(t)
	a.connectGraceOverride = 50 * time.Millisecond
	r := watchConnects(a)
	busID, cwd := closedChat(t, a, "codex:ignored", "Ignored Chat")
	err := a.busHost().With(func(b *bus.Bus) error {
		if _, err := b.CreateGroup(context.Background(), bus.Sender{}, "Ignoring Team", []string{busID}); err != nil {
			return err
		}
		_, err := b.SendMentioning(context.Background(), bus.Sender{}, "#Ignoring Team", "Are you there?", nil, 0, []string{busID})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	// The chat has a window, which the page restored and does not act on.
	a.ChatWindows([]string{"codex:ignored"})
	wakeRound(t, a)
	if asked := askedIDs(r.take()); len(asked) != 1 || asked[0] != "codex:ignored" {
		t.Fatalf("the page was not asked: %v", asked)
	}
	deadline := time.Now().Add(5 * time.Second)
	for s := a.session("codex:ignored"); s == nil || !s.State().Ready; s = a.session("codex:ignored") {
		if time.Now().After(deadline) {
			t.Fatal("the engine did not open a chat the window left alone")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if got := a.session("codex:ignored").Cwd(); got != cwd {
		t.Fatalf("the chat was opened in %s", got)
	}
	ring(t, a)
	waitTurn(t, r.turns)
	if got := prompts(c); len(got) != 1 || !strings.Contains(got[0], "Are you there?") {
		t.Fatalf("the waiting message was not delivered: %q", got)
	}
}

// A chat that was switched to another provider is never opened as itself: an
// open of it goes to the newest chat that continues it, and the agent is never
// asked for the superseded one.
func TestOpeningAContinuedChatOpensOnlyItsHead(t *testing.T) {
	a, c := providerTestApp(t)
	ws := t.TempDir()
	write := func(id, parent string) {
		tr := &sessionhost.Transcript{SessionID: id, Provider: "codex", Workspace: ws, ParentSessionID: parent, Complete: true}
		if err := sessionhost.WriteTranscript(configDir(), tr); err != nil {
			t.Fatal(err)
		}
	}
	write("codex:first", "")
	write("codex:second", "codex:first")
	write("codex:third", "codex:second")
	info, err := a.OpenSessionAt(ws, "codex:first")
	if err != nil {
		t.Fatal(err)
	}
	if info.ID != "codex:third" {
		t.Fatalf("the open returned %s, not the newest continuation", info.ID)
	}
	for _, call := range c.calls {
		if call.method != "session/load" {
			continue
		}
		if id := call.params.(codexagent.SessionParams).SessionID; id != "codex:third" {
			t.Fatalf("the agent was asked for %s", id)
		}
	}
	if countCalls(c, "session/load") != 1 || a.session("codex:first") != nil || a.session("codex:second") != nil {
		t.Fatalf("%d loads; a superseded chat is open", countCalls(c, "session/load"))
	}
}

// Two opens of one chat at once, as the page's connect and the engine's own
// after the grace can be, take turns: the second waits for the first and
// returns its result, and the agent is asked once.
func TestTwoOpensOfOneChatAtOnceAskTheAgentOnce(t *testing.T) {
	a, c := providerTestApp(t)
	ws := t.TempDir()
	id := wholeCopy(t, ws, nil)
	entered, release := make(chan struct{}), make(chan struct{})
	c.onLoad = func() {
		close(entered)
		<-release
	}
	type result struct {
		info SessionInfo
		err  error
	}
	first, second := make(chan result, 1), make(chan result, 1)
	go func() {
		info, err := a.OpenSessionAt(ws, id)
		first <- result{info, err}
	}()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("the first open did not reach the agent")
	}
	go func() {
		info, err := a.OpenSessionAt(ws, id)
		second <- result{info, err}
	}()
	// The second open must be waiting, not finished, while the first is.
	select {
	case r := <-second:
		t.Fatalf("the second open returned before the first finished: %+v %v", r.info, r.err)
	case <-time.After(200 * time.Millisecond):
	}
	close(release)
	one, two := <-first, <-second
	if one.err != nil || two.err != nil {
		t.Fatalf("the opens failed: %v, %v", one.err, two.err)
	}
	if one.info.ID != id || two.info.ID != id || one.info.StreamGen != two.info.StreamGen {
		t.Fatalf("the opens returned different chats: %+v and %+v", one.info, two.info)
	}
	if countCalls(c, "session/load") != 1 {
		t.Fatalf("the agent was asked %d times", countCalls(c, "session/load"))
	}
}
