package bus

import (
	"context"
	"testing"
	"time"
)

// The chats of groups with a recent message are listed, the most recently
// active group's first, in the order they joined it, and each chat once.
func TestGroupChatsAreListedByRecentActivity(t *testing.T) {
	ctx := context.Background()
	b, root := open(t)
	studio, pudl := workspace(t, root, "studio"), workspace(t, root, "pudl")
	a, _ := b.RegisterSession(ctx, "window-a", studio, "Architect")
	e, _ := b.RegisterSession(ctx, "window-e", studio, "Engine")
	d, _ := b.RegisterSession(ctx, "window-d", pudl, "Docs")
	p, _ := b.RegisterSession(ctx, "window-p", pudl, "Parser")
	old, _ := b.CreateGroup(ctx, Sender{Session: &a}, "Old", []string{"Engine"})
	recent, _ := b.CreateGroup(ctx, Sender{Session: &a}, "Recent", []string{"Docs", "Parser"})
	quiet, _ := b.CreateGroup(ctx, Sender{Session: &e}, "Quiet", []string{"Docs"})
	for _, g := range []Room{old, recent} {
		if _, err := b.Send(ctx, Sender{Session: &a}, "#"+g.Name, "hello", nil, 0); err != nil {
			t.Fatal(err)
		}
	}
	_ = quiet
	week := time.Now().Add(-WarmSince)
	// The old group's message is eight days old.
	if _, err := b.db.Exec(`UPDATE messages SET created_at = ? WHERE room = ?`, time.Now().Add(-8*24*time.Hour).UTC().Format(time.RFC3339Nano), old.Name); err != nil {
		t.Fatal(err)
	}
	got, err := b.GroupChats(ctx, week)
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, c := range got {
		ids = append(ids, c.Session)
	}
	// Architect sends to both groups but is a member of both too, so it is
	// listed once, with the recent group's members after it.
	want := []string{a.ID, d.ID, p.ID}
	if len(ids) != len(want) {
		t.Fatalf("listed %v, want %v", ids, want)
	}
	for i := range want {
		if ids[i] != want[i] {
			t.Fatalf("listed %v, want %v", ids, want)
		}
	}
	if got[0].WorkspacePath == "" {
		t.Fatal("a chat has no folder")
	}
	// Engine is only in the old group and in Quiet, which has no message.
	for _, id := range ids {
		if id == e.ID {
			t.Fatal("a chat of groups with no recent message was listed")
		}
	}
}

// WarmSince is how far back a group's latest message may be for its chats to
// be opened at start.
const WarmSince = 7 * 24 * time.Hour

// Arriving lists what the doorbell counts, without taking it, and leaves out
// what the doorbell leaves out: a group's mention from an agent once the
// group's budget is spent.
func TestArrivingAgreesWithTheDoorbell(t *testing.T) {
	g := newBudgetGroup(t)
	ctx := context.Background()
	g.budget(0)
	g.say(&g.a, "an agent asks", g.c.ID)
	g.say(nil, "the person asks", g.c.ID)
	g.say(&g.a, "for information")
	msgs, more, err := g.b.Arriving(ctx, g.c.ID, 5)
	if err != nil || more != 0 || len(msgs) != 1 || msgs[0].Text != "the person asks" {
		t.Fatalf("arriving %+v (%d more) %v", msgs, more, err)
	}
	if g.rings(g.c) != 1 {
		t.Fatal("the doorbell disagrees")
	}
	again, _, _ := g.b.Arriving(ctx, g.c.ID, 5)
	if len(again) != 1 {
		t.Fatal("Arriving took the message")
	}
	g.budget(NoTurnBudget)
	msgs, more, _ = g.b.Arriving(ctx, g.c.ID, 1)
	if len(msgs) != 1 || more != 1 {
		t.Fatalf("with a limit of one: %d messages, %d more", len(msgs), more)
	}
}
