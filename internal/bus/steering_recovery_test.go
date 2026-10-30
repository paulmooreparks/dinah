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

// A Stop hook that is blocked, because a message waits for the agent, does not end the hook turn: the agent carries on
// in the same turn, and the chat stays steerable through however many blocked stops there are.
func TestABlockedStopLeavesTheHookTurnSteerable(t *testing.T) {
	ctx := context.Background()
	b, root := open(t)
	chat, err := b.RegisterSession(ctx, "window-stops", workspace(t, root, "stops"), "")
	if err != nil {
		t.Fatal(err)
	}
	other, err := b.RegisterSession(ctx, "window-sender", workspace(t, root, "sender"), "")
	if err != nil {
		t.Fatal(err)
	}
	if err := b.BeginHookTurn(ctx, chat.ID, "turn"); err != nil {
		t.Fatal(err)
	}
	if err := b.BindAgentTurn(ctx, "claude:agent", chat.ID, "turn", "p1"); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		if _, err := b.Send(ctx, Sender{Session: &other}, chat.Address(), "Please look at this.", nil, 0); err != nil {
			t.Fatal(err)
		}
		msgs, block, err := b.Deliver(ctx, chat.ID, "Stop")
		if err != nil || !block || len(msgs) == 0 {
			t.Fatalf("stop %d: the stop was not blocked: %v %v %v", i, len(msgs), block, err)
		}
		if reached, err := b.HooksReach(ctx, chat.ID); err != nil || !reached {
			t.Fatalf("after blocked stop %d the chat is not steerable: %v %v", i, reached, err)
		}
		if _, err := b.QueueSteer(ctx, chat.ID, "mid-turn guidance"); err != nil {
			t.Fatal(err)
		}
		if got, err := b.TakeAgentSteers(ctx, "claude:agent", "p1"); err != nil || len(got) != 1 {
			t.Fatalf("after blocked stop %d the turn's hook could not take a steer: %+v %v", i, got, err)
		}
	}
}

// The facts behind a steer decision are what the log line for it reports, including when the hook turn began and ended.
func TestSteerFactsSayWhyAChatCanOrCannotBeSteered(t *testing.T) {
	ctx := context.Background()
	b, root := open(t)
	chat, err := b.RegisterSession(ctx, "window-facts", workspace(t, root, "facts"), "")
	if err != nil {
		t.Fatal(err)
	}
	if f, err := b.SteerFactsOf(ctx, chat.ID); err != nil || f.Bound || f.HookTurn || f.Reaches {
		t.Fatalf("a chat no hook has seen: %+v %v", f, err)
	}
	if err := b.BeginHookTurn(ctx, chat.ID, "abcdefghijkl"); err != nil {
		t.Fatal(err)
	}
	if err := b.BindAgentTurn(ctx, "claude:agent", chat.ID, "abcdefghijkl", "p1"); err != nil {
		t.Fatal(err)
	}
	f, err := b.SteerFactsOf(ctx, chat.ID)
	if err != nil || !f.Bound || f.BindingToken != "abcdefgh" || !f.HookTurn || !f.Active || f.BeganAt == "" || f.EndedAt != "" || !f.Reaches {
		t.Fatalf("a chat in a running turn: %+v %v", f, err)
	}
	if err := b.EndHookTurn(ctx, chat.ID, "abcdefghijkl"); err != nil {
		t.Fatal(err)
	}
	f, err = b.SteerFactsOf(ctx, chat.ID)
	if err != nil || f.Active || f.EndedAt == "" || f.Reaches {
		t.Fatalf("a chat between turns: %+v %v", f, err)
	}
}

// A stop that is blocked after the window's prompt for the turn has come back, and ended its hook turn, reopens the hook
// turn: the agent carries on in it, so it is steerable and its own hook takes a steer.
func TestABlockedStopAfterThePromptReturnedReopensTheHookTurn(t *testing.T) {
	ctx := context.Background()
	b, root := open(t)
	chat, err := b.RegisterSession(ctx, "window-late-stop", workspace(t, root, "late"), "")
	if err != nil {
		t.Fatal(err)
	}
	other, err := b.RegisterSession(ctx, "window-late-sender", workspace(t, root, "latesender"), "")
	if err != nil {
		t.Fatal(err)
	}
	check := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	check(b.BeginHookTurn(ctx, chat.ID, "turn"))
	check(b.BindAgentTurn(ctx, "claude:agent", chat.ID, "turn", "p1"))
	check(b.EndHookTurn(ctx, chat.ID, "turn")) // the prompt came back first
	if reached, _ := b.HooksReach(ctx, chat.ID); reached {
		t.Fatal("a chat between turns was steerable")
	}
	_, err = b.Send(ctx, Sender{Session: &other}, chat.Address(), "One more thing.", nil, 0)
	check(err)
	if _, block, err := b.Deliver(ctx, chat.ID, "Stop"); err != nil || !block {
		t.Fatalf("the stop was not blocked: %v %v", block, err)
	}
	if reached, _ := b.HooksReach(ctx, chat.ID); !reached {
		t.Fatal("the agent carried on past the blocked stop, but the chat was not steerable")
	}
	_, err = b.QueueSteer(ctx, chat.ID, "mid-turn guidance")
	check(err)
	if got, err := b.TakeAgentSteers(ctx, "claude:agent", "p1"); err != nil || len(got) != 1 {
		t.Fatalf("the turn's hook did not take the steer: %+v %v", got, err)
	}
	// A stop that is not blocked leaves the turn ended, as before.
	check(b.EndHookTurn(ctx, chat.ID, "turn"))
	if _, block, _ := b.Deliver(ctx, chat.ID, "Stop"); block {
		t.Fatal("a stop with nothing waiting was blocked")
	}
	if reached, _ := b.HooksReach(ctx, chat.ID); reached {
		t.Fatal("a stop that ended the turn left the chat steerable")
	}
}
