package main

import (
	"context"
	"errors"
	"strings"
	"testing"

	"studio/internal/bus"
	"studio/internal/sessionhost"
)

// teamApp has a lead chat and a member chat on the bus, the member led by the lead, and returns the bus
// ids and a helper that makes the member mention the person.
func teamApp(t *testing.T) (a *App, lead, member *Session, leadBus, memberBus string, mention func(text string) bus.Message) {
	t.Helper()
	a, c := providerTestApp(t)
	a.teamsActive.Store(true)
	c.newIDs = []string{"codex:lead", "codex:member"}
	ws := t.TempDir()
	leadInfo, err := a.NewSessionAt(ws)
	if err != nil {
		t.Fatal(err)
	}
	memberInfo, err := a.NewSessionAt(ws)
	if err != nil {
		t.Fatal(err)
	}
	lead, member = a.session(leadInfo.ID), a.session(memberInfo.ID)
	leadBus, memberBus = a.busSessionID(lead.ID()), a.busSessionID(member.ID())
	var group string
	err = a.busHost().With(func(b *bus.Bus) error {
		var err error
		group, err = b.SetLead(context.Background(), memberBus, leadBus)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	mention = func(text string) bus.Message {
		var m bus.Message
		err := a.busHost().With(func(b *bus.Bus) error {
			s, err := b.Session(context.Background(), memberBus)
			if err != nil {
				return err
			}
			m, err = b.SendMentioning(context.Background(), bus.Sender{Session: &s}, "#"+group, text, nil, 0, []string{"paul"})
			return err
		})
		if err != nil {
			t.Fatal(err)
		}
		return m
	}
	return a, lead, member, leadBus, memberBus, mention
}

func relayCards(t *testing.T, a *App) []Notification {
	t.Helper()
	items, err := a.Notifications()
	if err != nil {
		t.Fatal(err)
	}
	var out []Notification
	for _, n := range items {
		if n.Kind == "relay" {
			out = append(out, n)
		}
	}
	return out
}

func checkRelaysNow(t *testing.T, a *App) {
	t.Helper()
	if err := a.busHost().With(func(b *bus.Bus) error { a.checkRelays(context.Background(), b); return nil }); err != nil {
		t.Fatal(err)
	}
}

// A relay whose delivery to the lead could not be made, whose lead's turn on it ended in an error, or whose
// lead cannot run, makes one card for the person with the member's message and the reason; a relay that is
// delivered and simply not relayed makes none.
func TestARelayThatFailsNotifiesThePersonOnceWithTheReason(t *testing.T) {
	a, lead, _, leadBus, _, mention := teamApp(t)
	// Nothing has failed.
	one := mention("Need a decision on the parser.")
	checkRelaysNow(t, a)
	if got := relayCards(t, a); len(got) != 0 {
		t.Fatalf("a relay that waited made %d cards", len(got))
	}
	// The delivery to the lead fails.
	a.relayDeliveryFailed(lead.ID(), errors.New("the agent would not start"))
	checkRelaysNow(t, a)
	cards := relayCards(t, a)
	if len(cards) != 1 || cards[0].Message != one.ID || !strings.Contains(strings.Join(cards[0].Details, " "), "the agent would not start") || !strings.Contains(cards[0].Body, "parser") {
		t.Fatalf("the delivery failure: %+v", cards)
	}
	checkRelaysNow(t, a)
	if got := relayCards(t, a); len(got) != 1 {
		t.Fatalf("the failure was reported %d times", len(got))
	}
	// The lead's turn on a relay ends in an error.
	two := mention("Second question.")
	var delivered []ConversationMessage
	delivered = append(delivered, ConversationMessage{Role: "delivered", Delivered: &sessionhost.DeliveredMessage{ID: two.ID}})
	p := sessionhost.NewPrompt("x", nil).WithDelivery("From a chat", delivered, nil)
	a.noteRelayTurnStart(lead, &p)
	a.relayTurnEnded(lead, errors.New("usage limit reached"))
	checkRelaysNow(t, a)
	cards = relayCards(t, a)
	if len(cards) != 2 || cards[1].Message != two.ID || !strings.Contains(strings.Join(cards[1].Details, " "), "usage limit reached") {
		t.Fatalf("the turn failure: %+v", cards)
	}
	// A turn that ended well fails nothing.
	three := mention("Third question.")
	p = sessionhost.NewPrompt("x", nil).WithDelivery("From a chat", []ConversationMessage{{Role: "delivered", Delivered: &sessionhost.DeliveredMessage{ID: three.ID}}}, nil)
	a.noteRelayTurnStart(lead, &p)
	a.relayTurnEnded(lead, nil)
	checkRelaysNow(t, a)
	if got := relayCards(t, a); len(got) != 2 {
		t.Fatalf("a turn that ended well made a card: %d", len(got))
	}
	// The lead cannot run: its context window is full and a send is held.
	lead.SetHold(&sessionhost.ContextHold{Kind: "context", Used: 1900, Size: 2048, Actions: []string{sessionhost.HoldCompact}})
	checkRelaysNow(t, a)
	cards = relayCards(t, a)
	if len(cards) != 3 || cards[2].Message != three.ID || !strings.Contains(strings.Join(cards[2].Details, " "), "context window is full") {
		t.Fatalf("a lead that cannot run: %+v", cards)
	}
	_ = leadBus
}

// A deleted lead sends its members back to the person: the members have no lead and the group says so.
func TestDeletingALeadEndsItsTeam(t *testing.T) {
	a, lead, _, leadBus, memberBus, mention := teamApp(t)
	a.forgetTeam(lead.ID())
	err := a.busHost().With(func(b *bus.Bus) error {
		l, _, err := b.LeadOf(context.Background(), memberBus)
		if err != nil || l != "" {
			t.Fatalf("the member still has the lead %q %v", l, err)
		}
		if g, _ := b.TeamGroup(context.Background(), leadBus); g != "" {
			t.Fatalf("the lead still has a team group %q", g)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	_ = mention
}
