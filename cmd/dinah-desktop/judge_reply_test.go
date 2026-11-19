package main

import (
	"slices"
	"strings"
	"testing"
	"time"

	"studio/internal/judge"
)

// replyApp opens a chat whose saved conversation ends with the agent's
// answer, with a model chosen and the kind on or off.
func replyApp(t *testing.T, on bool, messages ...ConversationMessage) (*App, *judge.Judges, string) {
	t.Helper()
	a, _ := calibrationApp(t)
	choose(t, a, func(s *JudgementSettings) { s.Model = "tiny:4b" })
	if on {
		choose(t, a, func(s *JudgementSettings) {
			s.KindsOff = slices.DeleteFunc(slices.Clone(s.KindsOff), func(n string) bool { return n == replySuggestionKindName })
		})
	}
	info, err := a.NewSessionAt(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) == 0 {
		messages = []ConversationMessage{{Role: "you", Text: "Rename the entry."}, {Role: "agent", Text: "Done.\n\nShall I rename the pinned one too?"}}
	}
	if err := writeTranscript(&conversationTranscript{SessionID: info.ID, Provider: "codex", Workspace: info.Cwd, Messages: messages}); err != nil {
		t.Fatal(err)
	}
	set, err := a.judges()
	if err != nil {
		t.Fatal(err)
	}
	return a, set, info.ID
}

func replyRecords(t *testing.T, a *App) []judge.Record {
	t.Helper()
	rows, err := a.JudgementRecords(JudgementFilter{Kind: replySuggestionKindName})
	if err != nil {
		t.Fatal(err)
	}
	return rows.Records
}

// The kind starts off, and while it is off the page's asking makes no call
// and leaves nothing in the record.
func TestASuggestedReplyStartsOffAndThenNothingIsAskedOrRecorded(t *testing.T) {
	a, set, id := replyApp(t, false)
	k := kindNamed(t, a, replySuggestionKindName)
	if k.Enabled || !k.DefaultOff || k.InputCap != 8000 || k.DeadlineMs != 10000 {
		t.Fatalf("the kind is %+v", k)
	}
	set.SetJudge(&answering{`{"reply":"Yes."}`}, "test", "tokens")
	got, err := a.SuggestReply(id)
	if err != nil || got.Text != "" || got.Call != 0 {
		t.Fatalf("a suggestion was made with the kind off: %+v %v", got, err)
	}
	if n := len(replyRecords(t, a)); n != 0 {
		t.Fatalf("%d calls were recorded with the kind off", n)
	}
}

// With the kind on, the judge is shown the person's last message and the
// agent's answer, and its reply is the suggestion. Asking again for the
// conversation as it stands gives the same suggestion from the record and
// makes no second call.
func TestASuggestedReplyIsMadeOnceForAConversationAsItStands(t *testing.T) {
	a, set, id := replyApp(t, true,
		ConversationMessage{Role: "you", Text: "An earlier question."}, ConversationMessage{Role: "agent", Text: "An earlier answer."},
		ConversationMessage{Role: "you", Text: "Rename the entry.\n\n[Dinah added this block]"}, ConversationMessage{Role: "status", Text: "working"},
		ConversationMessage{Role: "agent", Text: "Done."}, ConversationMessage{Role: "agent", Text: "Shall I rename the pinned one too?"})
	set.SetJudge(&answering{`{"reply":" Yes, rename the pinned one too. "}`}, "test", "tokens")
	got, err := a.SuggestReply(id)
	if err != nil || got.Text != "Yes, rename the pinned one too." || got.Call == 0 {
		t.Fatalf("the suggestion is %+v %v", got, err)
	}
	rec, err := set.Store().Get(got.Call)
	if err != nil || rec.Used != judge.UsedJudge || rec.EffectKind != "chat" || rec.EffectID != id {
		t.Fatalf("the record is %+v %v", rec, err)
	}
	if rec.Input != `{"you":"Rename the entry.","agent":"Done.\n\nShall I rename the pinned one too?"}` {
		t.Fatalf("the judge was shown %s", rec.Input)
	}
	set.SetJudge(&answering{`{"reply":"Something else."}`}, "test", "tokens")
	again, _ := a.SuggestReply(id)
	if again != got || len(replyRecords(t, a)) != 1 {
		t.Fatalf("asking again gave %+v with %d calls recorded, want the first suggestion and one call", again, len(replyRecords(t, a)))
	}
}

// A suggestion the person took or dropped is not made again for the same
// conversation, and its record says what became of it. Only this chat's own
// suggestion can be marked.
func TestASuggestedRepliesOutcomeIsKeptAndItIsNotOfferedAgain(t *testing.T) {
	a, set, id := replyApp(t, true)
	set.SetJudge(&answering{`{"reply":"Yes."}`}, "test", "tokens")
	got, _ := a.SuggestReply(id)
	if got.Text != "Yes." {
		t.Fatalf("the suggestion is %+v", got)
	}
	if err := a.ReplySuggestionOutcome(id, got.Call, "sent"); err == nil {
		t.Fatal("an outcome that is neither taken nor dropped was kept")
	}
	if err := a.ReplySuggestionOutcome("another-chat", got.Call, replyTaken); err == nil {
		t.Fatal("another chat marked this chat's suggestion")
	}
	if err := a.ReplySuggestionOutcome(id, got.Call, replyTaken); err != nil {
		t.Fatal(err)
	}
	if again, _ := a.SuggestReply(id); again.Text != "" || len(replyRecords(t, a)) != 1 {
		t.Fatalf("a taken suggestion was offered again: %+v, %d calls", again, len(replyRecords(t, a)))
	}
	// The first outcome stands.
	if err := a.ReplySuggestionOutcome(id, got.Call, replyDropped); err != nil {
		t.Fatal(err)
	}
	if outcome, _ := set.Store().Outcome(got.Call); outcome != replyTaken {
		t.Fatalf("the outcome is %q", outcome)
	}
}

// A suggestion that was neither taken nor dropped when the person's next
// message starts a turn was passed over, and a judge that proposed nothing
// leaves no outcome.
func TestASuggestedReplyThatWasNotTakenIsPassedOver(t *testing.T) {
	a, set, id := replyApp(t, true)
	set.SetJudge(&answering{`{"reply":"Yes."}`}, "test", "tokens")
	got, _ := a.SuggestReply(id)
	a.noteReplyPassedOver(id)
	deadline := time.Now().Add(5 * time.Second)
	for {
		if outcome, _ := set.Store().Outcome(got.Call); outcome == replyPassedOver {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the suggestion was not marked as passed over")
		}
		time.Sleep(10 * time.Millisecond)
	}
	b, set2, id2 := replyApp(t, true)
	set2.SetJudge(&answering{`{"reply":""}`}, "test", "tokens")
	nothing, _ := b.SuggestReply(id2)
	if nothing.Text != "" || nothing.Call == 0 {
		t.Fatalf("a judge that proposed nothing gave %+v", nothing)
	}
	b.noteReplyPassedOver(id2)
	b.judging.work.Wait()
	if outcome, _ := set2.Store().Outcome(nothing.Call); outcome != "" {
		t.Fatalf("an empty suggestion has the outcome %q", outcome)
	}
}

// A reply that is too long, has too many lines, or is not in form is not
// shown, and neither is anything where the agent has not answered.
func TestAnUnusableSuggestedReplyIsNotShown(t *testing.T) {
	a, set, id := replyApp(t, true)
	for _, bad := range []string{`Yes.`, `{"reply":"` + strings.Repeat("x", replySuggestionLimit+1) + `"}`, `{"reply":"1\n2\n3\n4\n5\n6\n7"}`, `{"reply":"Yes","more":1}`} {
		set.SetJudge(&answering{bad}, "test", "tokens")
		if got, _ := a.SuggestReply(id); got.Text != "" {
			t.Fatalf("%q was offered as %+v", bad, got)
		}
	}
	b, set2, id2 := replyApp(t, true, ConversationMessage{Role: "agent", Text: "Hello."}, ConversationMessage{Role: "you", Text: "Do the thing."})
	set2.SetJudge(&answering{`{"reply":"Yes."}`}, "test", "tokens")
	if got, _ := b.SuggestReply(id2); got.Text != "" || len(replyRecords(t, b)) != 0 {
		t.Fatalf("a suggestion was made where the agent has not answered: %+v", got)
	}
}

// What the judge is shown is cut: the person's message from its start and
// the agent's from its end, where its question is.
func TestWhatTheJudgeIsShownForAReplyIsCut(t *testing.T) {
	in, ok := replyInput([]ConversationMessage{{Role: "you", Text: strings.Repeat("a", replyYouLimit) + "TAIL"}, {Role: "agent", Text: "HEAD" + strings.Repeat("b", replyAgentLimit-1) + "?"}})
	if !ok || len([]rune(in.You)) != replyYouLimit || strings.Contains(in.You, "TAIL") || len([]rune(in.Agent)) != replyAgentLimit || strings.Contains(in.Agent, "HEAD") || !strings.HasSuffix(in.Agent, "?") {
		t.Fatalf("the input is %d and %d characters", len([]rune(in.You)), len([]rune(in.Agent)))
	}
	if _, ok := replyInput(nil); ok {
		t.Fatal("an empty conversation has a reply to make")
	}
}
