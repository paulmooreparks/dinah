package main

import (
	"context"
	"strings"
	"testing"

	"studio/internal/bus"
	"studio/internal/sessionhost"
)

// What the person writes to a member, and the member's answer, make one copy
// for the member's lead in its team room, posted once the member's turn on it
// has ended. Only the lead receives it: the member is given neither the copy
// nor a notice of it. Until 18 November 2026 the person's message and the
// answer were two copies, and every member of the team room, this member
// among them, was given a notice of each (Paul).
func TestThePersonsExchangeWithAMemberIsOneCopyForTheLeadAlone(t *testing.T) {
	a, _, member, leadBus, memberBus, _ := teamApp(t)
	group := ""
	if err := a.busHost().With(func(b *bus.Bus) error {
		var err error
		group, err = b.TeamGroup(context.Background(), leadBus)
		return err
	}); err != nil || group == "" {
		t.Fatal("no team room", group, err)
	}
	room := func() []bus.Message {
		var out []bus.Message
		if err := a.busHost().With(func(b *bus.Bus) error {
			var err error
			out, err = b.RoomMessages(context.Background(), group, 50)
			return err
		}); err != nil {
			t.Fatal(err)
		}
		return out
	}
	copies := func() []bus.Message {
		out := []bus.Message{}
		for _, m := range room() {
			if strings.HasPrefix(m.Text, "Copy for ") {
				out = append(out, m)
			}
		}
		return out
	}
	given := func(session string) []bus.Message {
		var out []bus.Message
		if err := a.busHost().With(func(b *bus.Bus) error {
			var err error
			out, _, err = b.Deliver(context.Background(), session, bus.DoorbellTurn)
			return err
		}); err != nil {
			t.Fatal(err)
		}
		return out
	}
	if err := writeTranscript(&sessionhost.Transcript{SessionID: member.ID(), Provider: "codex", Workspace: member.Cwd(), Complete: true, Messages: []ConversationMessage{{Role: "you", Text: "An earlier question"}, {Role: "agent", Text: "An earlier answer"}}}); err != nil {
		t.Fatal(err)
	}
	given(memberBus)
	given(leadBus)
	a.copyToLead(member.ID(), "Check the corrected figures.", 0)
	a.copyToLead(member.ID(), "And the totals.", 0)
	if got := copies(); len(got) != 0 {
		t.Fatalf("a copy was posted before the member answered: %+v", got)
	}
	// A turn that ends before the person's message has run leaves the copy
	// still to come.
	a.copyAnswerToLead(member)
	if len(copies()) != 0 {
		t.Fatal("an earlier turn's answer was copied as the answer")
	}
	if err := writeTranscript(&sessionhost.Transcript{SessionID: member.ID(), Provider: "codex", Workspace: member.Cwd(), Complete: true, Messages: []ConversationMessage{{Role: "you", Text: "An earlier question"}, {Role: "agent", Text: "An earlier answer"}, {Role: "you", Text: "Check the corrected figures."}, {Role: "you", Text: "And the totals."}, {Role: "agent", Text: "All four figures are right."}}}); err != nil {
		t.Fatal(err)
	}
	a.copyAnswerToLead(member)
	got := copies()
	if len(got) != 1 || !strings.Contains(got[0].Text, "Check the corrected figures.") || !strings.Contains(got[0].Text, "And the totals.") ||
		!strings.Contains(got[0].Text, "All four figures are right.") || strings.Contains(got[0].Text, "An earlier answer") || !strings.Contains(got[0].Text, "No reply is needed.") {
		t.Fatalf("the copy of the exchange is %+v", got)
	}
	a.copyAnswerToLead(member)
	if len(copies()) != 1 {
		t.Fatal("the exchange was copied twice")
	}
	// The lead is given the copy; the member is given nothing of it.
	if lead := given(leadBus); len(lead) != 1 || lead[0].ID != got[0].ID {
		t.Fatalf("the lead was given %+v", lead)
	}
	for _, m := range given(memberBus) {
		if m.ID == got[0].ID {
			t.Fatalf("the member was given the copy of its own exchange: %+v", m)
		}
	}
	// A turn that ended with no answer still makes one copy, which says so.
	a.copyToLead(member.ID(), "Any news?", 0)
	if err := writeTranscript(&sessionhost.Transcript{SessionID: member.ID(), Provider: "codex", Workspace: member.Cwd(), Complete: true, Messages: []ConversationMessage{{Role: "you", Text: "An earlier question"}, {Role: "agent", Text: "An earlier answer"}, {Role: "you", Text: "Check the corrected figures."}, {Role: "you", Text: "And the totals."}, {Role: "agent", Text: "All four figures are right."}, {Role: "you", Text: "Any news?"}}}); err != nil {
		t.Fatal(err)
	}
	a.copyAnswerToLead(member)
	got = copies()
	if len(got) != 2 || !strings.Contains(got[1].Text, "ended without an answer") || !strings.Contains(got[1].Text, "Any news?") {
		t.Fatalf("the copy of an unanswered message is %+v", got)
	}
	// A chat with no lead is not copied anywhere.
	before := len(room())
	a.copyToLead("codex:nobody", "Hello", 0)
	if len(room()) != before {
		t.Fatal("a chat without a lead was copied")
	}
}
