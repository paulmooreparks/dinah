package bus

import (
	"context"
	"testing"
)

// The newest message for a recipient changes when another message arrives,
// and stays the same when the waiting messages are delivered.
func TestNewestSessionDeliveryRisesOnlyWhenAMessageArrives(t *testing.T) {
	b, root := open(t)
	ctx := context.Background()
	cwd := workspace(t, root, "work")
	s, err := b.RegisterSession(ctx, "held-owner", cwd, "Held")
	if err != nil {
		t.Fatal(err)
	}
	other, err := b.RegisterSession(ctx, "other-owner", cwd, "Other")
	if err != nil {
		t.Fatal(err)
	}
	if n, err := b.NewestSessionDelivery(ctx, s.ID); err != nil || n != 0 {
		t.Fatal("a recipient with no messages", n, err)
	}
	first, err := b.Send(ctx, Sender{}, s.Address(), "First", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.Send(ctx, Sender{}, other.Address(), "For someone else", nil, 0); err != nil {
		t.Fatal(err)
	}
	n, err := b.NewestSessionDelivery(ctx, s.ID)
	if err != nil || n != first.ID {
		t.Fatal("newest after the first message", n, first.ID, err)
	}
	if msgs, _, err := b.Deliver(ctx, s.ID, DoorbellTurn); err != nil || len(msgs) != 1 {
		t.Fatal(msgs, err)
	}
	if after, err := b.NewestSessionDelivery(ctx, s.ID); err != nil || after != n {
		t.Fatal("a delivery changed the newest message", after, n, err)
	}
	second, err := b.Send(ctx, Sender{}, s.Address(), "Second", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	if after, err := b.NewestSessionDelivery(ctx, s.ID); err != nil || after != second.ID || after == n {
		t.Fatal("a new arrival after a delivery did not change the newest message", after, second.ID, err)
	}
}
