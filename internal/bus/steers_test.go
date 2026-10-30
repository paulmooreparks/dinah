package bus

import (
	"context"
	"testing"
)

func TestASteerIsTakenOnceOrWithdrawn(t *testing.T) {
	ctx := context.Background()
	b, root := open(t)
	chat, _ := b.RegisterSession(ctx, "window-1", workspace(t, root, "parser"), "")
	if reach, _ := b.HooksReach(ctx, chat.ID); reach {
		t.Fatal("a chat no hook has seen was taken as reached")
	}
	if err := b.BindAgentSession(ctx, "claude-session", chat.ID); err != nil {
		t.Fatal(err)
	}
	if got, _ := b.SessionOfAgent(ctx, "claude-session"); got != chat.ID {
		t.Fatalf("the agent's session belongs to %q", got)
	}
	if reach, _ := b.HooksReach(ctx, chat.ID); !reach {
		t.Fatal("a bound chat was not taken as reached")
	}
	first, _ := b.QueueSteer(ctx, chat.ID, "Use the new retry interval.")
	second, _ := b.QueueSteer(ctx, chat.ID, "And skip the docs.")
	got, err := b.TakeSteers(ctx, chat.ID)
	if err != nil || len(got) != 2 || got[0].ID != first || got[1].Text != "And skip the docs." {
		t.Fatalf("took %+v %v", got, err)
	}
	if again, _ := b.TakeSteers(ctx, chat.ID); len(again) != 0 {
		t.Fatal("a steer was taken twice")
	}
	if ok, _ := b.WithdrawSteer(ctx, second); ok {
		t.Fatal("a delivered steer was withdrawn")
	}
	if done, _ := b.SteerDelivered(ctx, first); !done {
		t.Fatal("a taken steer is not marked delivered")
	}
	third, _ := b.QueueSteer(ctx, chat.ID, "Never mind.")
	if ok, _ := b.WithdrawSteer(ctx, third); !ok {
		t.Fatal("a waiting steer could not be withdrawn")
	}
	if got, _ := b.TakeSteers(ctx, chat.ID); len(got) != 0 {
		t.Fatal("a withdrawn steer was taken")
	}
}

func TestAPromptsMarkerFindsItsChat(t *testing.T) {
	prompt := "Fix the build.\n\n[Dinah Applet Context]\n...\n" + SteerMarker("window-ABC123")
	if got := MarkedSession(prompt); got != "window-ABC123" {
		t.Fatalf("the marker gave %q", got)
	}
	if got := MarkedSession("No marker here."); got != "" {
		t.Fatalf("a prompt without a marker gave %q", got)
	}
	if got := MarkedSession("[Dinah chat: a b]"); got != "" {
		t.Fatalf("a malformed marker gave %q", got)
	}
}
