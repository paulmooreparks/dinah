package bus

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func TestWakeWaitingNoticeIsPrivateDurableAndDoesNotCreateANoticeLoop(t *testing.T) {
	b, dir := open(t)
	ctx := context.Background()
	caller, err := b.RegisterSession(ctx, "caller", dir, "Caller")
	if err != nil {
		t.Fatal(err)
	}
	target, err := b.RegisterSession(ctx, "target", dir, "Target")
	if err != nil {
		t.Fatal(err)
	}
	other, err := b.RegisterSession(ctx, "other", dir, "Other")
	if err != nil {
		t.Fatal(err)
	}
	m, err := b.Send(ctx, Sender{Session: &caller}, target.Address(), "Please work", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	senders, err := b.WakeSenders(ctx, target.ID)
	if err != nil || len(senders) != 1 || senders[0].First != m.ID || senders[0].Session != caller.ID {
		t.Fatalf("senders %+v: %v", senders, err)
	}
	if err := b.NotifyWakeWaiting(ctx, target.ID, target.Title, senders[0], 8); err != nil {
		t.Fatal(err)
	}
	reopened, err := OpenAt(filepath.Join(b.dir, "bus.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if err := reopened.NotifyWakeWaiting(ctx, target.ID, "Renamed", senders[0], 9); err != nil {
		t.Fatal(err)
	}
	if n, _, err := b.Doorbell(ctx, other.ID); err != nil || n != 0 {
		t.Fatalf("unrelated chat notified: %d %v", n, err)
	}
	if senders, err := b.WakeSenders(ctx, caller.ID); err != nil || len(senders) != 0 {
		t.Fatalf("host notice created a notice loop: %+v %v", senders, err)
	}
	msgs, _, err := b.Deliver(ctx, caller.ID, DoorbellTurn)
	if err != nil || len(msgs) != 1 {
		t.Fatalf("not exactly one notice: %+v %v", msgs, err)
	}
	if msgs[0].Kind != ResultKind || msgs[0].FromName != "Dinah" || msgs[0].Asked || !strings.Contains(msgs[0].Text, "Target") || !strings.Contains(msgs[0].Text, "all 8 places") {
		t.Fatalf("wrong notice: %+v", msgs[0])
	}
	// The original actionable delivery remains pending.
	if n, _, err := b.Doorbell(ctx, target.ID); err != nil || n != 1 {
		t.Fatalf("original lost: %d %v", n, err)
	}
}

func TestWakeWaitingOnlyNotifiesActionableSendersIncludingThePerson(t *testing.T) {
	b, dir := open(t)
	ctx := context.Background()
	a, _ := b.RegisterSession(ctx, "a", dir, "A")
	z, _ := b.RegisterSession(ctx, "z", dir, "Z")
	passive, _ := b.RegisterSession(ctx, "passive", dir, "Passive")
	g, err := b.CreateGroup(ctx, Sender{}, "Work", []string{a.ID, z.ID, passive.ID})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.SendMentioning(ctx, Sender{Session: &a}, "#"+g.Name, "For Z", nil, 0, []string{z.ID}); err != nil {
		t.Fatal(err)
	}
	if senders, err := b.WakeSenders(ctx, passive.ID); err != nil || len(senders) != 0 {
		t.Fatalf("passive group reader notified: %+v %v", senders, err)
	}
	if _, err := b.Send(ctx, Sender{}, z.Address(), "From the person", nil, 0); err != nil {
		t.Fatal(err)
	}
	senders, err := b.WakeSenders(ctx, z.ID)
	if err != nil || len(senders) != 2 {
		t.Fatalf("missing sender: %+v %v", senders, err)
	}
	for _, sender := range senders {
		if err := b.NotifyWakeWaiting(ctx, z.ID, z.Title, sender, 8); err != nil {
			t.Fatal(err)
		}
	}
	inbox, err := b.HumanInbox(ctx, 100, false)
	if err != nil || len(inbox) != 1 || inbox[0].Kind != ResultKind || inbox[0].FromName != "Dinah" {
		t.Fatalf("person did not receive a host notice: %+v %v", inbox, err)
	}
}
