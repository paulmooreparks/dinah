package main

import (
	"context"
	"strings"
	"testing"

	"studio/internal/bus"
	"studio/internal/sessionhost"
)

// What the person writes to a member, and the member's answer, are copied to
// the member's lead in its team room, addressed to the lead alone.
func TestThePersonsMessageToAMemberAndItsAnswerAreCopiedToTheLead(t *testing.T) {
	a, _, member, leadBus, _, _ := teamApp(t)
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
	if err := writeTranscript(&sessionhost.Transcript{SessionID: member.ID(), Provider: "codex", Workspace: member.Cwd(), Complete: true, Messages: []ConversationMessage{{Role: "you", Text: "An earlier question"}, {Role: "agent", Text: "An earlier answer"}}}); err != nil {
		t.Fatal(err)
	}
	a.copyToLead(member.ID(), "Check the corrected figures.", 0)
	got := copies()
	if len(got) != 1 || !strings.Contains(got[0].Text, "Check the corrected figures.") || !strings.Contains(got[0].Text, "No reply is needed.") {
		t.Fatalf("the copy of the person's message is %+v", got)
	}
	// A turn that ends before the person's message has run leaves the answer
	// still to come.
	a.copyAnswerToLead(member)
	if len(copies()) != 1 {
		t.Fatal("an earlier turn's answer was copied as the answer")
	}
	if err := writeTranscript(&sessionhost.Transcript{SessionID: member.ID(), Provider: "codex", Workspace: member.Cwd(), Complete: true, Messages: []ConversationMessage{{Role: "you", Text: "An earlier question"}, {Role: "agent", Text: "An earlier answer"}, {Role: "you", Text: "Check the corrected figures."}, {Role: "agent", Text: "All four figures are right."}}}); err != nil {
		t.Fatal(err)
	}
	a.copyAnswerToLead(member)
	got = copies()
	if len(got) != 2 || !strings.Contains(got[1].Text, "All four figures are right.") || strings.Contains(got[1].Text, "An earlier answer") {
		t.Fatalf("the copy of the answer is %+v", got)
	}
	a.copyAnswerToLead(member)
	if len(copies()) != 2 {
		t.Fatal("the answer was copied twice")
	}
	// A chat with no lead is not copied anywhere.
	before := len(room())
	a.copyToLead("codex:nobody", "Hello", 0)
	if len(room()) != before {
		t.Fatal("a chat without a lead was copied")
	}
}
