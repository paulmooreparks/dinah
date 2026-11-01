package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"studio/internal/bus"
	"studio/internal/sessionhost"
)

func TestWaitingContinuationKeepsBothRecipientsAndEveryDelivery(t *testing.T) {
	a, c := providerTestApp(t)
	a.connectGraceOverride = -1
	parent, child := "codex:older", "codex:later"
	parentBus, cwd := closedChat(t, a, parent, "Original owner")
	childBus, _ := closedChat(t, a, child, "Continuation owner")
	older := &sessionhost.Transcript{SessionID: parent, Provider: "codex", Workspace: cwd, Complete: true, Messages: []sessionhost.ConversationMessage{{Role: "you", Text: "Preserve this history"}}}
	next := &sessionhost.Transcript{SessionID: child, Provider: "codex", Workspace: cwd, Complete: true, ParentSessionID: parent, PendingHandoff: true, ParentHistory: older.Messages}
	for _, tr := range []*sessionhost.Transcript{older, next} {
		if err := sessionhost.WriteTranscript(configDir(), tr); err != nil {
			t.Fatal(err)
		}
	}
	beforeParent, _ := json.Marshal(readSaved(t, parent))
	beforeChild, _ := json.Marshal(readSaved(t, child))
	r := watchConnects(a)
	var childAttempt string
	a.ChatWindows(nil)
	wakeRound(t, a)
	for _, request := range r.take() {
		if request["deliveryOrigin"] == parent {
			t.Fatal("conflicting parent wake was sent to the page", request)
		}
		if request["deliveryOrigin"] == child {
			childAttempt = request["deliveryAttempt"].(string)
		}
	}
	if countCalls(c, "session/load") != 0 || countCalls(c, "session/resume") != 0 {
		t.Fatal("conflicting routing opened a provider")
	}
	records, err := readWaitingChats()
	if err != nil {
		t.Fatal(err)
	}
	record, ok := records[parent]
	if !ok || record.Stage != "routing-recovery" || record.Route.Recipient != parentBus || record.Route.TargetRecipient != childBus {
		t.Fatal(records)
	}
	if _, err := a.RetryWaitingSession(parent); err == nil {
		t.Fatal("manual retry bypassed identity recovery")
	}
	err = a.busHost().With(func(b *bus.Bus) error {
		recipient, err := b.RegisterSession(context.Background(), parentBus, cwd, "Original owner")
		if err != nil {
			return err
		}
		if _, err := b.Send(context.Background(), bus.Sender{}, recipient.Address(), "A newer authorized question", nil, 0); err != nil {
			return err
		}
		return b.EndSession(context.Background(), parentBus)
	})
	if err != nil {
		t.Fatal(err)
	}
	wakeRound(t, a)
	if len(r.take()) != 0 {
		t.Fatal("newer delivery bypassed recovery")
	}
	err = a.busHost().With(func(b *bus.Bus) error {
		for id, want := range map[string]int{parentBus: 2, childBus: 1} {
			n, _, err := b.Doorbell(context.Background(), id)
			if err != nil {
				return err
			}
			if n != want {
				t.Errorf("%s has %d pending deliveries, want %d", id, n, want)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	afterParent, _ := json.Marshal(readSaved(t, parent))
	afterChild, _ := json.Marshal(readSaved(t, child))
	if string(beforeParent) != string(afterParent) || string(beforeChild) != string(afterChild) {
		t.Fatal("saved history or pending handoff changed")
	}
	a.mu.Lock()
	first, second := a.settings.BusSessions[parent], a.settings.BusSessions[child]
	a.mu.Unlock()
	if first != parentBus || second != childBus {
		t.Fatal("recipient ownership changed")
	}
	cards, err := a.Notifications()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, card := range cards {
		if card.Kind == "delivery-failure" && card.Lead == parent {
			found = true
			if card.RetryAllowed || card.SessionID != child || !strings.Contains(card.Body, "2 messages are waiting.") {
				t.Fatal(card)
			}
		}
	}
	if !found {
		t.Fatal("actual-target recovery card missing")
	}
	withResume(a)
	if childAttempt == "" {
		t.Fatal("child's own pending delivery was not admitted")
	}
	if _, err := a.OpenWaitingSession(child, childAttempt); err != nil {
		t.Fatal(err)
	}
	err = a.busHost().With(func(b *bus.Bus) error {
		a.busCoordinator().Ring(context.Background(), b, a.session(child))
		for id, want := range map[string]int{parentBus: 2, childBus: 0} {
			n, _, err := b.Doorbell(context.Background(), id)
			if err != nil {
				return err
			}
			if n != want {
				t.Errorf("%s has %d pending after child delivery, want %d", id, n, want)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestWaitingFailureCardsSurviveRetentionAndRejectUnknownStore(t *testing.T) {
	a, _ := providerTestApp(t)
	route := sessionhost.WakeRoute{Origin: "codex:old", Target: "codex:missing", Recipient: "old-bus", TargetRecipient: "new-bus", Cwd: t.TempDir(), Provider: "codex", Title: "Saved work"}
	err := writeWaitingChats(map[string]sessionhost.WakeFailure{route.Origin: {Route: route, Attempt: "owned-attempt", Stage: "routing-recovery", Error: "Resource not found does not establish deletion", Created: time.Unix(1, 0)}})
	if err != nil {
		t.Fatal(err)
	}
	cards, err := a.Notifications()
	if err != nil || len(cards) != 1 || cards[0].SessionID != route.Target {
		t.Fatal(cards, err)
	}
	if err := a.ClearNotifications(); err != nil {
		t.Fatal(err)
	}
	cards, err = a.Notifications()
	if err != nil || len(cards) != 0 {
		t.Fatal("clear did not dismiss the recovery incident", cards, err)
	}
	if records, err := readWaitingChats(); err != nil || len(records) != 1 {
		t.Fatal("clear changed the durable delivery hold", records, err)
	}
	for _, data := range []string{`{"version":2,"items":{}}`, `{"version":1,"items":null}`, `{"version":1,"items":{"wrong":{"attempt":"a","stage":"failed"}}}`} {
		if _, err := decodeWaitingChats([]byte(data)); err == nil {
			t.Fatal("invalid recovery state accepted", data)
		}
	}
}

func TestWaitingStoreReadErrorKeepsOrdinaryNotificationsAndExplicitReload(t *testing.T) {
	a, _ := providerTestApp(t)
	if _, err := a.SaveNotification(Notification{ID: "ordinary", Kind: "app", Severity: "info", Title: "Unrelated saved notice"}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(configDir(), "waiting-chats.json")
	if err := os.WriteFile(path, []byte("invalid JSON"), 0600); err != nil {
		t.Fatal(err)
	}
	cards, err := a.Notifications()
	if err != nil || len(cards) != 2 {
		t.Fatal(cards, err)
	}
	found := false
	for _, card := range cards {
		if card.Kind == "delivery-store-error" {
			found = true
		}
	}
	if !found {
		t.Fatal("corrupt recovery state had no diagnostic card")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := a.ReloadWaitingChatState(); err != nil {
		t.Fatal(err)
	}
	cards, err = a.Notifications()
	if err != nil || len(cards) != 1 || cards[0].ID != "ordinary" {
		t.Fatal(cards, err)
	}
}
