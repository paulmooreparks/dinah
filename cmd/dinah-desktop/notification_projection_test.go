package main

import (
	"context"
	"testing"

	"studio/internal/bus"
	"studio/internal/requestsettlement"
)

func TestNotificationProjectionRepairsFromCommittedSourceAfterRestart(t *testing.T) {
	a, _, owner, group, _, _ := mentionRig(t)
	pass(t, a) // Establish the ordinary mention baseline before the message.
	chatSays(t, a, owner, group, "An invented request for the person.", bus.Human)
	pass(t, a)
	cards := mentionCards(t, a)
	if len(cards) != 1 {
		t.Fatal("the real mention did not make one projection", cards)
	}
	card := cards[0]
	var settled requestsettlement.Receipt
	if err := a.busHost().With(func(b *bus.Bus) error {
		source, err := b.DecisionForMessage(context.Background(), card.Message)
		if err != nil {
			return err
		}
		settled, err = b.DismissRequest(context.Background(), requestsettlement.Input{ID: source.ID, Revision: source.Revision, OperationID: "source-before-projection"})
		return err
	}); err != nil || settled.State != "committed" {
		t.Fatal(settled, err)
	}
	// The source commit succeeds without the legacy notification adapter or
	// watcher. This leaves the real saved projection behind intentionally.
	kept, err := a.notificationByID(card.ID)
	if err != nil || kept.Dismissed {
		t.Fatal("the fixture did not retain a stale projection", kept, err)
	}
	a.busHost().CloseHandle()
	restarted := NewApp()
	t.Cleanup(func() { restarted.busHost().CloseHandle() })
	items, err := restarted.Notifications()
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range items {
		if item.ID == card.ID {
			t.Fatal("a missed event restored the settled request to the open view", items)
		}
	}
	kept, err = restarted.notificationByID(card.ID)
	if err != nil || !kept.Dismissed {
		t.Fatal("rereading did not durably repair the projection", kept, err)
	}
	source, err := restarted.RequestState(settled.ID)
	if err != nil || source.Status != "dismissed" || source.Wording == "" {
		t.Fatal("the original source history was lost", source, err)
	}
}
