package main

import (
	"context"
	"testing"

	"studio/internal/bus"
)

func TestDecisionEntriesReuseDurableAttention(t *testing.T) {
	a, _, me, group, notifier, _ := mentionRig(t)
	pass(t, a)
	var request bus.Decision
	if err := a.busHost().With(func(b *bus.Bus) error {
		var err error
		request, err = b.AskPerson(context.Background(), me, bus.AskPersonInput{OperationID: "entry-request", Group: "#" + group, Questions: []bus.PersonQuestion{{Number: 1, Text: "Should we proceed?", Recommendation: "Proceed."}}})
		return err
	}); err != nil {
		t.Fatal(err)
	}
	pass(t, a)
	pass(t, a)
	cards := mentionCards(t, a)
	if len(cards) != 1 || cards[0].Message != request.MessageID || notifier.count() != 1 {
		t.Fatalf("card and native notice %v %v", cards, notifier.count())
	}
	entries, err := a.DecisionEntries()
	if err != nil || entries.Count != 1 || entries.Items[0].ID != request.ID || entries.Items[0].MessageID != cards[0].Message {
		t.Fatalf("entries %+v %v", entries, err)
	}
	// Plain group mentions retain their card without becoming a request.
	chatSays(t, a, me, group, "Please review this result", bus.Human)
	pass(t, a)
	entries, err = a.DecisionEntries()
	if err != nil || entries.Count != 1 || len(mentionCards(t, a)) != 2 {
		t.Fatalf("plain mention %+v %v", entries, err)
	}
	if err := a.DismissNotification(cards[0].ID); err != nil {
		t.Fatal(err)
	}
	entries, err = a.DecisionEntries()
	if err != nil || entries.Count != 1 {
		t.Fatalf("dismissing attention closed a request %+v %v", entries, err)
	}
	plainCards := mentionCards(t, a)
	if len(plainCards) != 1 {
		t.Fatal("plain mention card missing", plainCards)
	}
	if err := a.AcknowledgeMention(plainCards[0].Message); err != nil {
		t.Fatal(err)
	}
	entries, err = a.DecisionEntries()
	if err != nil || entries.Count != 1 || entries.Items[0].ID != request.ID {
		t.Fatalf("dismissed mention remained indexed %+v %v", entries, err)
	}
}

func TestDirectDecisionMakesRetainedAttention(t *testing.T) {
	a, _, me, _, notifier, _ := mentionRig(t)
	pass(t, a)
	var request bus.Decision
	if err := a.busHost().With(func(b *bus.Bus) error {
		var err error
		request, err = b.AskPerson(context.Background(), me, bus.AskPersonInput{OperationID: "direct-entry", Questions: []bus.PersonQuestion{{Number: 1, Text: "May we proceed?", Recommendation: "Proceed."}}})
		return err
	}); err != nil {
		t.Fatal(err)
	}
	pass(t, a)
	pass(t, a)
	cards := mentionCards(t, a)
	if len(cards) != 1 || cards[0].Group != "" || cards[0].Message != request.MessageID || notifier.count() != 1 {
		t.Fatalf("direct request attention %v, native %d", cards, notifier.count())
	}
}
