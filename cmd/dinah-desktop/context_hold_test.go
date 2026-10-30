package main

import (
	"encoding/base64"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"studio/internal/acp"
	"studio/internal/ollamaagent"
	"studio/internal/sessionhost"
)

// withContextModule gives the test's provider the Ollama adapter's context
// module, whose requests the fake connection answers.
func withContextModule(t *testing.T, provider string) {
	t.Helper()
	original := providerModules[provider]
	m := original
	m.Context = providerModules["ollama"].Context
	providerModules[provider] = m
	t.Cleanup(func() { providerModules[provider] = original })
}

// fullWindow is a chat whose window is full at 92%.
var fullWindow = ollamaagent.ContextState{Used: 1900, Size: 2048, Known: true, Full: true, CanSetAside: true}

func holdFor(t *testing.T, s *Session) *sessionhost.ContextHold {
	t.Helper()
	deadline := time.Now().Add(testWait)
	for time.Now().Before(deadline) {
		if h := s.Hold(); h != nil {
			return h
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("no send was held")
	return nil
}

// A send to a chat whose known window is full is held: it waits in the chat's
// queue, the chat's information says it is held and what the person may choose,
// and nothing is sent to the agent.
func TestASendToAFullWindowIsHeldInTheQueue(t *testing.T) {
	a, c := providerTestApp(t)
	withContextModule(t, "codex")
	info, err := a.NewSessionAt(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	c.window = fullWindow
	if err := a.Send(info.ID, "Hello", nil, nil); err != nil {
		t.Fatal(err)
	}
	s := a.session(info.ID)
	hold := holdFor(t, s)
	if hold.Kind != "context" || hold.Used != 1900 || hold.Size != 2048 || len(hold.Actions) != 2 || hold.Actions[0] != sessionhost.HoldSetAside || hold.Actions[1] != sessionhost.HoldCompact {
		t.Fatalf("the hold is %+v", hold)
	}
	if got := countCalls(c, "session/prompt"); got != 0 {
		t.Fatalf("%d prompts were sent to the agent", got)
	}
	shown := s.InfoVersioned()
	if len(shown.Queued) != 1 || shown.Queued[0] != "Hello" || shown.Hold == nil || shown.Choice != "" || s.Working() {
		t.Fatalf("the chat's information: %+v", shown)
	}
}

// A window that is not full holds nothing.
func TestAWindowThatIsNotFullHoldsNothing(t *testing.T) {
	a, c := providerTestApp(t)
	withContextModule(t, "codex")
	r := watchConnects(a)
	info, err := a.NewSessionAt(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	c.window = ollamaagent.ContextState{Used: 1000, Size: 2048, Known: true}
	if err := a.Send(info.ID, "Hello", nil, nil); err != nil {
		t.Fatal(err)
	}
	waitTurn(t, r.turns)
	if s := a.session(info.ID); s.Hold() != nil || countCalls(c, "session/prompt") != 1 {
		t.Fatal("a send was held with room in the window")
	}
}

// Setting turns aside is remembered for the chat, runs, releases the held send
// in the same chat, and clears the hold; a refusal when nothing could be set
// aside keeps the hold.
func TestResolvingAHoldBySettingTurnsAsideReleasesTheSend(t *testing.T) {
	a, c := providerTestApp(t)
	withContextModule(t, "codex")
	r := watchConnects(a)
	info, err := a.NewSessionAt(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	c.window = fullWindow
	if err := a.Send(info.ID, "Hello", nil, nil); err != nil {
		t.Fatal(err)
	}
	s := a.session(info.ID)
	holdFor(t, s)
	if _, err := a.ResolveHold(info.ID, "shrug"); err == nil {
		t.Fatal("a course that is not offered was run")
	}
	c.asideTurns = 0
	if _, err := a.ResolveHold(info.ID, sessionhost.HoldSetAside); err == nil || s.Hold() == nil {
		t.Fatalf("a set-aside that set nothing aside released the send: %v", err)
	}
	c.window = fullWindow
	c.asideTurns = 3
	if _, err := a.ResolveHold(info.ID, sessionhost.HoldSetAside); err != nil {
		t.Fatal(err)
	}
	waitTurn(t, r.turns)
	if s.Hold() != nil || s.ContextChoice() != sessionhost.HoldSetAside || c.choice != sessionhost.HoldSetAside || countCalls(c, "session/prompt") != 1 {
		t.Fatalf("hold %v, choice %q (adapter %q), prompts %d", s.Hold(), s.ContextChoice(), c.choice, countCalls(c, "session/prompt"))
	}
	if shown := s.InfoVersioned(); shown.Hold != nil || shown.Choice != sessionhost.HoldSetAside || len(shown.Queued) != 0 {
		t.Fatalf("the information after: %+v", shown)
	}
	// Ask Me Again forgets the choice.
	if err := a.ForgetContextChoice(info.ID); err != nil || s.ContextChoice() != "" || c.choice != "" {
		t.Fatalf("the choice was not forgotten: %q %q %v", s.ContextChoice(), c.choice, err)
	}
}

// A chat with a remembered choice runs it without a hold: the turns are set
// aside and the send goes on.
func TestARememberedChoiceRunsWithoutAHold(t *testing.T) {
	a, c := providerTestApp(t)
	withContextModule(t, "codex")
	r := watchConnects(a)
	info, err := a.NewSessionAt(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	c.window = fullWindow
	c.window.Choice = sessionhost.HoldSetAside
	c.asideTurns = 2
	if err := a.Send(info.ID, "Hello", nil, nil); err != nil {
		t.Fatal(err)
	}
	waitTurn(t, r.turns)
	if s := a.session(info.ID); s.Hold() != nil || c.asides != 1 || countCalls(c, "session/prompt") != 1 || s.ContextChoice() != sessionhost.HoldSetAside {
		t.Fatalf("hold %v, set-asides %d, prompts %d", s.Hold(), c.asides, countCalls(c, "session/prompt"))
	}
	// When nothing can be set aside the send waits for the person, and does not
	// go on into a full window.
	c.window = fullWindow
	c.window.Choice = sessionhost.HoldSetAside
	c.asideTurns = 0
	if err := a.Send(info.ID, "Again", nil, nil); err != nil {
		t.Fatal(err)
	}
	holdFor(t, a.session(info.ID))
	if countCalls(c, "session/prompt") != 1 {
		t.Fatal("a send went into a full window when nothing could be set aside")
	}
}

// Compacting continues the chat with its own provider, takes the held send out of
// the earlier chat's queue and sends it to the new chat, which the earlier chat
// links to; the course is remembered.
func TestResolvingAHoldByCompactingContinuesTheChatAndSendsTheHeldMessage(t *testing.T) {
	a, c := providerTestApp(t)
	withContextModule(t, "codex")
	r := watchConnects(a)
	c.newIDs = []string{"codex:first", "codex:second"}
	info, err := a.NewSessionAt(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if info.ID != "codex:first" {
		t.Fatalf("the first chat is %s", info.ID)
	}
	old := a.session(info.ID)
	a.transcriptCapture().RecordMessage(old, "you", "Earlier question", false, false)
	a.transcriptCapture().RecordMessage(old, "agent", "Earlier answer", false, false)
	a.transcriptCapture().FinishTurn(old, nil, "end_turn")
	c.window = fullWindow
	if err := a.Send(info.ID, "Hello", nil, nil); err != nil {
		t.Fatal(err)
	}
	holdFor(t, old)
	next, err := a.ResolveHold(info.ID, sessionhost.HoldCompact)
	if err != nil {
		t.Fatal(err)
	}
	if next.ID != "codex:second" {
		t.Fatalf("the continuation is %s", next.ID)
	}
	waitTurn(t, r.turns)
	got := prompts(c)
	if len(got) != 1 || len(got[0]) == 0 {
		t.Fatalf("prompts: %q", got)
	}
	if old.Hold() != nil || len(old.Info().Queued) != 0 || old.ContextChoice() != sessionhost.HoldCompact || c.choice != sessionhost.HoldCompact {
		t.Fatalf("the earlier chat: hold %v, queue %v, choice %q", old.Hold(), old.Info().Queued, old.ContextChoice())
	}
	if head, err := a.ResolveSessionContinuation(info.ID); err != nil || head != "codex:second" {
		t.Fatalf("the earlier chat continues as %q (%v)", head, err)
	}
}

// What was attached to a held message goes to the new chat with it, through the
// same path as a message sent there: a picture is in the prompt the new chat's
// agent is given, and a file is linked from the workspace both chats share.
func TestCompactingSendsTheHeldMessagesAttachmentsToTheNewChat(t *testing.T) {
	a, c := providerTestApp(t)
	withContextModule(t, "codex")
	withImageCapability(a, "codex", true)
	r := watchConnects(a)
	c.newIDs = []string{"codex:first", "codex:second"}
	ws := t.TempDir()
	info, err := a.NewSessionAt(ws)
	if err != nil {
		t.Fatal(err)
	}
	old := a.session(info.ID)
	a.transcriptCapture().RecordMessage(old, "you", "Earlier question", false, false)
	a.transcriptCapture().RecordMessage(old, "agent", "Earlier answer", false, false)
	a.transcriptCapture().FinishTurn(old, nil, "end_turn")
	c.window = fullWindow
	picture := base64.StdEncoding.EncodeToString([]byte("not really a png, but bytes"))
	file := base64.StdEncoding.EncodeToString([]byte{0, 1, 2, 3, 4, 5})
	atts := []Attachment{{Kind: "file", Name: "look.png", MimeType: "image/png", Data: picture}, {Kind: "file", Name: "data.bin", MimeType: "application/octet-stream", Data: file}}
	if err := a.Send(info.ID, "Look at these", nil, atts); err != nil {
		t.Fatal(err)
	}
	holdFor(t, old)
	if _, err := a.ResolveHold(info.ID, sessionhost.HoldCompact); err != nil {
		t.Fatal(err)
	}
	waitTurn(t, r.turns)
	var sent []acp.ContentBlock
	for _, call := range c.calls {
		if p, ok := call.params.(acp.PromptParams); ok && call.method == "session/prompt" && p.SessionID == "codex:second" {
			sent = p.Prompt
		}
	}
	var gotPicture, gotLink bool
	for _, b := range sent {
		gotPicture = gotPicture || (b.Type == "image" && b.Data == picture)
		gotLink = gotLink || (b.Type == "resource_link" && b.Name == "data.bin")
	}
	if !gotPicture || !gotLink {
		t.Fatalf("the new chat was not given the picture (%v) and the file (%v): %+v", gotPicture, gotLink, sent)
	}
}

// A full window offers only the courses that can work: setting turns aside is left
// out when the adapter says none could be freed.
func TestOnlyTheCoursesThatCanWorkAreOffered(t *testing.T) {
	a, c := providerTestApp(t)
	withContextModule(t, "codex")
	info, err := a.NewSessionAt(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	c.window = ollamaagent.ContextState{Used: 1900, Size: 2048, Known: true, Full: true, CanSetAside: false}
	if err := a.Send(info.ID, "Hello", nil, nil); err != nil {
		t.Fatal(err)
	}
	hold := holdFor(t, a.session(info.ID))
	if len(hold.Actions) != 1 || hold.Actions[0] != sessionhost.HoldCompact {
		t.Fatalf("the courses offered are %v", hold.Actions)
	}
}

// Messages queued behind a held send go to the new chat with it, in their order: a
// continuation is refused while anything is queued, so compacting the chat took
// only the held one and failed every time when others waited.
func TestCompactingCarriesEveryQueuedMessageToTheNewChat(t *testing.T) {
	a, c := providerTestApp(t)
	withContextModule(t, "codex")
	r := watchConnects(a)
	c.newIDs = []string{"codex:first", "codex:second"}
	info, err := a.NewSessionAt(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	old := a.session(info.ID)
	a.transcriptCapture().RecordMessage(old, "you", "Earlier question", false, false)
	a.transcriptCapture().RecordMessage(old, "agent", "Earlier answer", false, false)
	a.transcriptCapture().FinishTurn(old, nil, "end_turn")
	c.window = fullWindow
	for i, text := range []string{"One", "Two", "Three"} {
		if err := a.Send(info.ID, text, nil, nil); err != nil {
			t.Fatal(err)
		}
		waitHeldQueue(t, old, i+1)
	}
	if n := len(old.Info().Queued); n != 3 {
		t.Fatalf("%d messages are queued", n)
	}
	queued := old.Info().Queued
	next, err := a.ResolveHold(info.ID, sessionhost.HoldCompact)
	if err != nil || next.ID != "codex:second" {
		t.Fatalf("compacting: %v %v", next.ID, err)
	}
	waitForPrompts(t, c, 3)
	got := prompts(c)
	for i, want := range queued {
		if !strings.Contains(got[i], want) {
			t.Fatalf("prompt %d is %q, want the queued %q: %q", i, got[i], want, got)
		}
	}
	_ = r
	if old.Hold() != nil || len(old.Info().Queued) != 0 {
		t.Fatalf("the earlier chat still holds %v and queues %v", old.Hold(), old.Info().Queued)
	}
}

func waitForPrompts(t *testing.T, c *testProviderConnection, n int) {
	t.Helper()
	for deadline := time.Now().Add(testWait); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		if len(prompts(c)) >= n {
			return
		}
	}
	t.Fatalf("only %d of %d prompts reached the agent: %q", len(prompts(c)), n, prompts(c))
}

// A compaction that fails puts everything back: the queue in its order, the hold
// with its courses, the page told, so the card is true and the person can choose
// again, and the chat can be compacted when what stopped it is gone.
func TestAFailedCompactionPutsTheHoldAndTheQueueBack(t *testing.T) {
	a, c := providerTestApp(t)
	withContextModule(t, "codex")
	var events []string
	a.sink = func(name string, _ any) { events = append(events, name) }
	c.newIDs = []string{"codex:first", "codex:second"}
	info, err := a.NewSessionAt(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	old := a.session(info.ID)
	a.transcriptCapture().RecordMessage(old, "you", "Earlier question", false, false)
	a.transcriptCapture().RecordMessage(old, "agent", "Earlier answer", false, false)
	a.transcriptCapture().FinishTurn(old, nil, "end_turn")
	c.window = fullWindow
	for i, text := range []string{"One", "Two"} {
		if err := a.Send(info.ID, text, nil, nil); err != nil {
			t.Fatal(err)
		}
		waitHeldQueue(t, old, i+1)
	}
	queued := old.Info().Queued
	c.newErr = errors.New("the agent refused")
	events = nil
	if _, err := a.ResolveHold(info.ID, sessionhost.HoldCompact); err == nil || !strings.Contains(err.Error(), "could not be continued") {
		t.Fatalf("a failed compaction: %v", err)
	}
	hold := old.Hold()
	if hold == nil || len(hold.Actions) != 2 || !slices.Equal(old.Info().Queued, queued) {
		t.Fatalf("after the failure: hold %+v, queue %v, want %v", hold, old.Info().Queued, queued)
	}
	if !slices.Contains(events, "agent:queue") || !slices.Contains(events, "agent:session") {
		t.Fatalf("the page was not told: %v", events)
	}
	if sw := old.Info().Switch; sw == nil || sw.Failed == "" {
		t.Fatalf("the failure is not shown: %+v", old.Info().Switch)
	}
	// What stopped it is gone: the same hold resolves.
	c.newErr = nil
	if next, err := a.ResolveHold(info.ID, sessionhost.HoldCompact); err != nil || next.ID != "codex:second" {
		t.Fatalf("compacting again: %v %v", next.ID, err)
	}
}

// waitHeldQueue waits until n messages are queued in the chat, since a send reaches
// the queue from the chat's own goroutine.
func waitHeldQueue(t *testing.T, s *Session, n int) {
	t.Helper()
	for deadline := time.Now().Add(testWait); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		if len(s.Info().Queued) == n && s.Hold() != nil {
			return
		}
	}
	t.Fatalf("%d messages are queued, not %d", len(s.Info().Queued), n)
}
