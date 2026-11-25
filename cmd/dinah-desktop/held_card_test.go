package main

import (
	"context"
	"testing"
	"time"

	"studio/internal/bus"
	"studio/internal/sessionhost"
)

// The held card says how many messages wait and when the opening began.
func TestHeldOpeningBodyWording(t *testing.T) {
	created := time.Date(2026, 11, 21, 12, 0, 0, 0, time.UTC)
	day := created.Local().Format("2 Jan")
	for _, c := range []struct {
		count int
		stage string
		want  string
	}{
		{1, "opening", "1 message is waiting. Opening this chat on " + day + " never finished."},
		{8, "opening", "8 messages are waiting. Opening this chat on " + day + " never finished."},
		{2, "failed", "2 messages are waiting. Opening this chat on " + day + " failed."},
		{0, "opening", "No messages are waiting. Opening this chat on " + day + " never finished."},
	} {
		if got := heldOpeningBody(c.count, c.stage, created); got != c.want {
			t.Errorf("count %d, stage %s: got %q, want %q", c.count, c.stage, got, c.want)
		}
	}
}

// Only an opening that never finished or failed, with no transfer and a
// retry that is allowed, gets the held card.
func TestHeldOpeningStageRules(t *testing.T) {
	for _, c := range []struct {
		stage    string
		transfer bool
		retry    bool
		want     bool
	}{
		{"opening", false, true, true},
		{"failed", false, true, true},
		{"routing-recovery", false, false, false},
		{"routing-recovery", false, true, false},
		{"opening", true, true, false},
		{"failed", false, false, false},
	} {
		if got := heldOpening(c.stage, c.transfer, c.retry); got != c.want {
			t.Errorf("stage %s, transfer %v, retry %v: got %v", c.stage, c.transfer, c.retry, got)
		}
	}
}

func holdChat(t *testing.T, a *App, origin, recipient, cwd, stage string, created time.Time) {
	t.Helper()
	route := sessionhost.WakeRoute{Origin: origin, Target: origin, Recipient: recipient, TargetRecipient: recipient, Cwd: cwd, Provider: "codex", Title: "Parser Team Lead"}
	if err := writeWaitingChats(map[string]sessionhost.WakeFailure{origin: {Route: route, Attempt: "held-attempt", Stage: stage, Error: "The previous opening has no confirmed completion.", Created: created}}); err != nil {
		t.Fatal(err)
	}
	if err := a.ReloadWaitingChatState(); err != nil {
		t.Fatal(err)
	}
}

// A held opening gets the short card, and Dismiss hides it until another
// message is held for the chat.
func TestHeldOpeningCardAndDismissUntilAnotherMessage(t *testing.T) {
	a, _ := providerTestApp(t)
	origin := "codex:parser-lead"
	recipient, cwd := closedChat(t, a, origin, "Parser Team Lead")
	created := time.Date(2026, 11, 21, 8, 33, 0, 0, time.UTC)
	holdChat(t, a, origin, recipient, cwd, "opening", created)
	cards, err := a.Notifications()
	if err != nil || len(cards) != 1 {
		t.Fatal(cards, err)
	}
	card := cards[0]
	want := "1 message is waiting. Opening this chat on " + created.Local().Format("2 Jan") + " never finished."
	if !card.HeldOpening || card.Title != "Messages held for Parser Team Lead" || card.Body != want || !card.RetryAllowed || len(card.Details) == 0 {
		t.Fatalf("held card: %+v", card)
	}
	again, err := a.Notifications()
	if err != nil || len(again) != 1 || again[0].IncidentVersion != card.IncidentVersion {
		t.Fatal("the version changed with nothing new", again, err)
	}
	if err := a.DismissNotification(card.ID); err != nil {
		t.Fatal(err)
	}
	if cards, err := a.Notifications(); err != nil || len(cards) != 0 {
		t.Fatal("dismiss did not hide the card", cards, err)
	}
	if cards, err := a.Notifications(); err != nil || len(cards) != 0 {
		t.Fatal("the card came back with nothing new", cards, err)
	}
	if err := a.busHost().With(func(b *bus.Bus) error {
		s, err := b.Session(context.Background(), recipient)
		if err != nil {
			return err
		}
		_, err = b.Send(context.Background(), bus.Sender{}, s.Address(), "Another question", nil, 0)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	cards, err = a.Notifications()
	if err != nil || len(cards) != 1 || cards[0].IncidentVersion == card.IncidentVersion || cards[0].Body[:24] != "2 messages are waiting. " {
		t.Fatal("another held message did not bring the card back", cards, err)
	}
}

// A failed opening says so; a hold that needs routing recovery keeps the
// card it had before.
func TestHeldCardStagesFailedAndRoutingRecovery(t *testing.T) {
	a, _ := providerTestApp(t)
	origin := "codex:parser-lead"
	recipient, cwd := closedChat(t, a, origin, "Parser Team Lead")
	created := time.Date(2026, 11, 21, 8, 33, 0, 0, time.UTC)
	holdChat(t, a, origin, recipient, cwd, "failed", created)
	cards, err := a.Notifications()
	if err != nil || len(cards) != 1 || !cards[0].HeldOpening || cards[0].Body != "1 message is waiting. Opening this chat on "+created.Local().Format("2 Jan")+" failed." {
		t.Fatal("failed opening", cards, err)
	}
	holdChat(t, a, origin, recipient, cwd, "routing-recovery", created)
	cards, err = a.Notifications()
	if err != nil || len(cards) != 1 || cards[0].HeldOpening || cards[0].Title == "Messages held for Parser Team Lead" || cards[0].Body != "1 message is waiting. Recover this original chat or remove it." {
		t.Fatal("routing recovery lost its card", cards, err)
	}
}
