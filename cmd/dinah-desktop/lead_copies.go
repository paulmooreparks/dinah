package main

import (
	"context"
	"fmt"
	"strings"

	"studio/internal/bus"
)

// When the person writes directly to a chat that reports to a lead, the lead
// is sent a copy of what the person wrote and of the chat's answer, in the
// lead's team room (docs/design/team-hierarchy.md). It is the copy a manager's
// manager sends the manager when writing to one of the manager's staff, so the
// lead sees the thread and knows whether its own instructions still stand. The
// exchange itself stays in the member's chat. The copy says that no reply is
// needed.
//
// Since 18 November 2026 an exchange makes one copy, posted when the member's
// turn on it has ended, with what the person wrote and the member's answer
// together, and only the lead receives it: the member and the rest of the
// team are given neither the copy nor a notice of it (bus.SendOnlyToRecipients),
// though it stays in the team room for the person to read. Until then the
// person's message and the answer were two copies, and every member of the
// team room, the member whose exchange it was among them, was given a notice
// of each (Paul).

// leadCopyLimit is the most characters of a message a copy repeats. The rest
// stays in the member's chat, and the copy says so.
const leadCopyLimit = 6000

// pendingLeadCopy is what the person wrote to a member whose answer is still
// to come. From is how many messages the member's saved conversation had when
// the person first wrote, so the answer is what follows the person's message
// after that point. Wrote holds each message the person sent before the
// answer, so that two of them make one copy.
type pendingLeadCopy struct {
	group, lead, leadTitle, member string
	from                           int
	wrote                          []string
}

func clipForLead(text, member string) string {
	text = strings.TrimSpace(text)
	if r := []rune(text); len(r) > leadCopyLimit {
		return string(r[:leadCopyLimit]) + "\n\n(The rest is in " + member + "'s chat.)"
	}
	return text
}

// copyToLead notes what the person wrote to chat id, when the chat reports to
// a lead, for the copy copyAnswerToLead posts once the chat has answered.
func (a *App) copyToLead(id, text string, attachments int) {
	text = strings.TrimSpace(text)
	if text == "" && attachments == 0 {
		return
	}
	lead, err := a.ChatLead(id)
	if err != nil {
		logf("bus", "warning", "chat %s's lead could not be read to copy the person's message: %v", id, err)
		return
	}
	if lead == nil || lead.Group == "" {
		return
	}
	member := a.sessionTitle(id, "")
	if member == "" {
		member = "a chat that reports to you"
	}
	if attachments > 0 {
		text += fmt.Sprintf("\n\n(With %d attachment(s), which stay in %s's chat.)", attachments, member)
	}
	if value, ok := a.leadCopies.Load(id); ok {
		pending := value.(pendingLeadCopy)
		pending.wrote = append(pending.wrote, text)
		a.leadCopies.Store(id, pending)
		return
	}
	from := 0
	if tr, err := readTranscript(id); err == nil {
		from = len(tr.Messages)
	}
	a.leadCopies.Store(id, pendingLeadCopy{group: lead.Group, lead: lead.Bus, leadTitle: lead.Title, member: member, from: from, wrote: []string{text}})
}

// copyAnswerToLead posts the one copy of the person's exchange with a member,
// once the turn that answered the person's message has ended. A turn that ends
// before the person's message has run, as when it was queued behind another,
// leaves the copy waiting for the next one.
func (a *App) copyAnswerToLead(s *Session) {
	value, ok := a.leadCopies.Load(s.ID())
	if !ok {
		return
	}
	pending := value.(pendingLeadCopy)
	tr, err := readTranscript(s.ID())
	if err != nil || pending.from > len(tr.Messages) {
		return
	}
	asked := -1
	for i, m := range tr.Messages[pending.from:] {
		if m.Role == "you" {
			asked = pending.from + i
			break
		}
	}
	if asked < 0 {
		return
	}
	a.leadCopies.Delete(s.ID())
	parts := []string{}
	for _, m := range tr.Messages[asked+1:] {
		if m.Role == "agent" && strings.TrimSpace(m.Text) != "" {
			parts = append(parts, strings.TrimSpace(m.Text))
		}
	}
	err = a.busHost().With(func(b *bus.Bus) error {
		_, err := b.SendOnlyToRecipients(context.Background(), bus.Sender{}, "#"+pending.group, leadCopyText(pending, b.PersonName(), parts), []string{pending.lead})
		return err
	})
	if err != nil {
		logf("bus", "warning", "the copy for %s of the person's exchange with %s could not be posted: %v", pending.leadTitle, pending.member, err)
	}
}

// leadCopyText is the one copy of an exchange: what the person wrote, and the
// member's answer or that its turn ended without one.
func leadCopyText(pending pendingLeadCopy, person string, answer []string) string {
	wrote := clipForLead(strings.Join(pending.wrote, "\n\n"), pending.member)
	if len(answer) == 0 {
		return fmt.Sprintf("Copy for %s: %s wrote to %s directly, and %s's turn ended without an answer to %s. No reply is needed.\n\n%s wrote:\n%s",
			pending.leadTitle, person, pending.member, pending.member, person, person, wrote)
	}
	return fmt.Sprintf("Copy for %s: %s wrote to %s directly, and %s answered. No reply is needed.\n\n%s wrote:\n%s\n\n%s answered:\n%s",
		pending.leadTitle, person, pending.member, pending.member, person, wrote, pending.member, clipForLead(strings.Join(answer, "\n\n"), pending.member))
}
