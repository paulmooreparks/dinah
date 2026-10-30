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
	if reached, err := b.HooksReach(ctx, chat.ID); err != nil || reached {
		t.Fatalf("old binding reached new turn: %v %v", reached, err)
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
