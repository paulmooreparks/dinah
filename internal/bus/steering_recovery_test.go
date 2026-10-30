package bus

import (
	"context"
	"path/filepath"
	"testing"
)

func TestHookTurnRejectsDelayedEvents(t *testing.T) {
	ctx := context.Background()
	b, root := open(t)
	chat, err := b.RegisterSession(ctx, "window-turns", workspace(t, root, "turns"), "")
	if err != nil {
		t.Fatal(err)
	}
	check := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	check(b.BeginHookTurn(ctx, chat.ID, "first"))
	check(b.BindAgentTurn(ctx, "devin:agent", chat.ID, "first", "prompt-1"))
	check(b.EndHookTurn(ctx, chat.ID, "first"))
	check(b.BeginHookTurn(ctx, chat.ID, "second"))
	// A steer can be queued as soon as the turn starts, before its first hook has run, since the agent has been tied to
	// the chat and the turn is a hook turn; the old binding still cannot take it (below).
	if reached, err := b.HooksReach(ctx, chat.ID); err != nil || !reached {
		t.Fatalf("a new hook turn of a tied chat is not steerable at once: %v %v", reached, err)
	}
	check(b.BindAgentTurn(ctx, "devin:agent", chat.ID, "second", "prompt-2"))
	_, err = b.QueueSteer(ctx, chat.ID, "new guidance")
	check(err)
	check(b.BindAgentTurn(ctx, "devin:agent", chat.ID, "first", "prompt-1"))
	if got, err := b.TakeAgentSteers(ctx, "devin:agent", "prompt-1"); err != nil || len(got) != 0 {
		t.Fatalf("old hook consumed new input: %+v %v", got, err)
	}
	if got, err := b.TakeAgentSteers(ctx, "devin:agent", "prompt-2"); err != nil || len(got) != 1 {
		t.Fatalf("current hook: %+v %v", got, err)
	}
	_, err = b.QueueSteer(ctx, chat.ID, "not consumed")
	check(err)
	check(b.EndHookTurn(ctx, chat.ID, "second"))
	if got, err := b.TakeAgentSteers(ctx, "devin:agent", "prompt-2"); err != nil || len(got) != 0 {
		t.Fatalf("closed turn delivered input: %+v %v", got, err)
	}
}

func TestSteeringSubmissionSurvivesRestartWithoutResending(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "bus.db")
	b, err := OpenAt(path)
	if err != nil {
		t.Fatal(err)
	}
	payload := `{"text":"keep scope","attachments":[{"kind":"file","data":"YWJj"}]}`
	id, err := b.SaveSteeringSubmission(ctx, "codex:thread", payload)
	if err != nil {
		t.Fatal(err)
	}
	b.Close()
	b, err = OpenAt(path)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	got, err := b.RecoverSteeringSubmissions(ctx, "codex:thread")
	if err != nil || len(got) != 1 || got[0].Payload != payload || got[0].State != "pending" {
		t.Fatalf("recovery: %+v %v", got, err)
	}
	if err = b.SetSteeringSubmissionState(ctx, "codex:other", id, "sent"); err != nil {
		t.Fatal(err)
	}
	got, _ = b.RecoverSteeringSubmissions(ctx, "codex:thread")
	if len(got) != 1 {
		t.Fatal("another session changed the receipt")
	}
	if err = b.SetSteeringSubmissionState(ctx, "codex:thread", id, "dismissed"); err != nil {
		t.Fatal(err)
	}
	got, err = b.RecoverSteeringSubmissions(ctx, "codex:thread")
	if err != nil || len(got) != 0 {
		t.Fatalf("dismissed record recovered: %+v %v", got, err)
	}
}

// A steer typed while a turn is starting, before the agent's first hook of the turn has run, is accepted, and the
// turn's own hook hands it over once it runs; when the turn has ended it is no longer steerable.
func TestASteerQueuedBeforeTheTurnsFirstHookIsHandedOverByIt(t *testing.T) {
	ctx := context.Background()
	b, root := open(t)
	chat, err := b.RegisterSession(ctx, "window-early", workspace(t, root, "early"), "")
	if err != nil {
		t.Fatal(err)
	}
	if err := b.BeginHookTurn(ctx, chat.ID, "first"); err != nil {
		t.Fatal(err)
	}
	if reached, _ := b.HooksReach(ctx, chat.ID); reached {
		t.Fatal("a chat no hook has ever seen was steerable")
	}
	if err := b.BindAgentTurn(ctx, "claude:agent", chat.ID, "first", "p1"); err != nil {
		t.Fatal(err)
	}
	if err := b.EndHookTurn(ctx, chat.ID, "first"); err != nil {
		t.Fatal(err)
	}
	if reached, _ := b.HooksReach(ctx, chat.ID); reached {
		t.Fatal("a chat between turns was steerable")
	}
	if err := b.BeginHookTurn(ctx, chat.ID, "second"); err != nil {
		t.Fatal(err)
	}
	if reached, _ := b.HooksReach(ctx, chat.ID); !reached {
		t.Fatal("a chat whose turn is starting was not steerable")
	}
	if _, err := b.QueueSteer(ctx, chat.ID, "typed early"); err != nil {
		t.Fatal(err)
	}
	if got, _ := b.TakeAgentSteers(ctx, "claude:agent", "p1"); len(got) != 0 {
		t.Fatalf("the old turn's hook took it: %+v", got)
	}
	if err := b.BindAgentTurn(ctx, "claude:agent", chat.ID, "second", "p2"); err != nil {
		t.Fatal(err)
	}
	if got, err := b.TakeAgentSteers(ctx, "claude:agent", "p2"); err != nil || len(got) != 1 || got[0].Text != "typed early" {
		t.Fatalf("the turn's own hook did not hand it over: %+v %v", got, err)
	}
}
