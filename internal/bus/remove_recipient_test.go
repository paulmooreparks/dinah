package bus

import (
	"context"
	"path/filepath"
	"testing"
)

func TestRemovedRecipientCancelsWithoutDeliveryAndCannotReopen(t *testing.T) {
	b, root := open(t)
	ctx := context.Background()
	cwd := workspace(t, root, "work")
	one, err := b.RegisterSession(ctx, "removed-owner", cwd, "Remove this")
	if err != nil {
		t.Fatal(err)
	}
	two, err := b.RegisterSession(ctx, "separate-owner", cwd, "Keep this")
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range []Session{one, two} {
		if _, err := b.Send(ctx, Sender{}, s.Address(), "Keep disposition truthful", nil, 0); err != nil {
			t.Fatal(err)
		}
	}
	if err := b.RemoveRecipient(ctx, one.ID); err != nil {
		t.Fatal(err)
	}
	if err := b.RemoveRecipient(ctx, one.ID); err != nil {
		t.Fatal(err)
	}
	var cancelled, delivered, pending int
	if err := b.db.QueryRow(`SELECT SUM(cancelled_at IS NOT NULL),SUM(delivered_at IS NOT NULL),SUM(cancelled_at IS NULL AND delivered_at IS NULL) FROM deliveries WHERE recipient=?`, one.ID).Scan(&cancelled, &delivered, &pending); err != nil {
		t.Fatal(err)
	}
	if cancelled != 1 || delivered != 0 || pending != 0 {
		t.Fatal("cancelled was confused with delivered", cancelled, delivered, pending)
	}
	if n, _, err := b.Doorbell(ctx, one.ID); err != nil || n != 0 {
		t.Fatal(n, err)
	}
	if msgs, _, err := b.Deliver(ctx, one.ID, DoorbellTurn); err != nil || len(msgs) != 0 {
		t.Fatal("removed recipient received messages", msgs, err)
	}
	wakes, err := b.Waking(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, wake := range wakes {
		if wake.Session == one.ID {
			t.Fatal("removed recipient was scheduled to wake")
		}
	}
	if n, err := b.PendingSessionDeliveries(ctx, two.ID); err != nil || n != 1 {
		t.Fatal("separate recipient changed", n, err)
	}
	if _, err := b.RegisterSession(ctx, one.ID, cwd, "Old process reconnecting"); err == nil {
		t.Fatal("removed recipient registered again")
	}
	if _, err := b.Send(ctx, Sender{}, one.Address(), "A late message", nil, 0); err == nil {
		t.Fatal("a send to a removed address appeared successful")
	}
	if err := b.Close(); err != nil {
		t.Fatal(err)
	}
	restarted, err := OpenAt(filepath.Join(b.dir, "bus.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer restarted.Close()
	if _, err := restarted.RegisterSession(ctx, one.ID, cwd, "After restart"); err == nil {
		t.Fatal("restart lost removal")
	}
	if n, _, err := restarted.Doorbell(ctx, one.ID); err != nil || n != 0 {
		t.Fatal(n, err)
	}
}
