package main

import (
	"context"
	"fmt"
	"reflect"
	"studio/internal/bus"
	"testing"
)

func TestMentionNavigationClearsOnlyItsNotification(t *testing.T) {
	a, _, me, group, _, events := mentionRig(t)
	pass(t, a)
	chatSays(t, a, me, group, "First invented mention", bus.Human)
	chatSays(t, a, me, group, "Second invented mention", bus.Human)
	pass(t, a)
	cards := mentionCards(t, a)
	if len(cards) != 2 {
		t.Fatal("missing actual mention notifications", cards)
	}
	var original bus.Message
	if err := a.busHost().With(func(b *bus.Bus) error {
		var err error
		original, err = b.Message(context.Background(), cards[0].Message)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	entries, err := a.DecisionEntries()
	if err != nil || entries.Count != 0 {
		t.Fatal("plain mention entered Decisions", entries, err)
	}
	page, err := a.Decisions(0)
	if err != nil || len(page.Items) != 0 {
		t.Fatal("plain mention entered the request list", page, err)
	}
	if _, err := a.DecisionByID(fmt.Sprintf("mention:%d", cards[0].Message)); err == nil {
		t.Fatal("old mention detail remained in Decisions")
	}
	// Ordinary read receipts do not claim that navigation reached the message.
	if err := a.BusMarkGroupRead(group); err != nil {
		t.Fatal(err)
	}
	pass(t, a)
	if len(mentionCards(t, a)) != 2 {
		t.Fatal("background read acknowledged navigation")
	}
	if err := a.AcknowledgeMention(cards[0].Message); err != nil {
		t.Fatal(err)
	}
	if err := a.AcknowledgeMention(cards[0].Message); err != nil {
		t.Fatal(err)
	}
	remaining := mentionCards(t, a)
	if len(remaining) != 1 || remaining[0].Message != cards[1].Message {
		t.Fatal("navigation cleared a different target", remaining)
	}
	if err := a.busHost().With(func(b *bus.Bus) error {
		after, err := b.Message(context.Background(), original.ID)
		if err == nil && !reflect.DeepEqual(original, after) {
			t.Fatal("navigation changed the source message")
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if err := a.AcknowledgeMention(-1); err == nil {
		t.Fatal("invalid target acknowledged")
	}
	// Re-reading persisted notification and attention state does not revive it.
	pass(t, a)
	if len(mentionCards(t, a)) != 1 {
		t.Fatal("acknowledgement was lost")
	}
	_ = events
}

func TestGroupReplyClearsMentionsAndPreservesStructuredRequests(t *testing.T) {
	a, _, me, group, _, _ := mentionRig(t)
	pass(t, a)
	chatSays(t, a, me, group, "An invented mention", bus.Human)
	var request bus.Decision
	if err := a.busHost().With(func(b *bus.Bus) error {
		var err error
		request, err = b.AskPerson(context.Background(), me, bus.AskPersonInput{OperationID: "preserve-request", Group: "#" + group, Questions: []bus.PersonQuestion{{Number: 1, Text: "Proceed?", Recommendation: "Wait."}}})
		return err
	}); err != nil {
		t.Fatal(err)
	}
	pass(t, a)
	if err := a.AcknowledgeMention(request.MessageID); err == nil {
		t.Fatal("navigation settled a structured request")
	}
	if _, err := a.BusSend("#"+group, "An invented reply in the source chat", 0); err != nil {
		t.Fatal(err)
	}
	pass(t, a)
	cards := mentionCards(t, a)
	if len(cards) != 1 || cards[0].Message != request.MessageID {
		t.Fatal("reply failed to distinguish request from mention", cards)
	}
	entries, err := a.DecisionEntries()
	if err != nil || entries.Count != 1 || entries.Items[0].ID != request.ID {
		t.Fatal("group reply altered the structured request", entries, err)
	}
}

func TestMentionNotificationsCarryAuthoritativeRequestIdentity(t *testing.T) {
	a, _, me, group, _, _ := mentionRig(t)
	pass(t, a)
	live := []Notification{}
	prior := a.sink
	a.sink = func(name string, value any) {
		if name == "notifications:added" {
			live = append(live, value.(Notification))
		}
		if prior != nil {
			prior(name, value)
		}
	}
	chatSays(t, a, me, group, "Invented plain mention", bus.Human)
	var request bus.Decision
	if err := a.busHost().With(func(b *bus.Bus) error {
		var err error
		request, err = b.AskPerson(context.Background(), me, bus.AskPersonInput{OperationID: "identity-request", Group: "#" + group, Questions: []bus.PersonQuestion{{Number: 1, Text: "Proceed?", Recommendation: "Wait."}}})
		return err
	}); err != nil {
		t.Fatal(err)
	}
	pass(t, a)
	if len(live) != 2 {
		t.Fatal("missing actual live notifications", live)
	}
	for _, item := range live {
		if item.Message == request.MessageID && item.RequestID != request.ID {
			t.Fatal("live structured request lost its identity", item)
		}
		if item.Message != request.MessageID && item.RequestID != "" {
			t.Fatal("plain mention became a request", item)
		}
	}
	// Preserve an old persisted card with no added field. Its source decides
	// routing on every read, independently of the Decisions index.
	notificationStoreMu.Lock()
	store, err := readNotifications()
	if err == nil {
		for i := range store.Items {
			store.Items[i].RequestID = ""
		}
		err = writeNotifications(store)
	}
	notificationStoreMu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	items, err := a.Notifications()
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range items {
		if item.Message == request.MessageID && item.RequestID != request.ID {
			t.Fatal("saved old request card was misclassified", item)
		}
		if item.Message != request.MessageID && item.RequestID != "" {
			t.Fatal("saved plain mention became a request", item)
		}
	}
}
