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
// exchange itself stays in the member's chat. Each copy is addressed to the
// lead alone and says that no reply is needed.

// leadCopyLimit is the most characters of a message a copy repeats. The rest
// stays in the member's chat, and the copy says so.
const leadCopyLimit = 6000

// pendingLeadCopy is a copy of the person's message that was posted and whose
// answer is still to come. From is how many messages the member's saved
// conversation had when the person wrote, so the answer is what follows the
// person's message after that point.
type pendingLeadCopy struct {
	group, lead, leadTitle, member string
	from                           int
}

func clipForLead(text, member string) string {
	text = strings.TrimSpace(text)
	if r := []rune(text); len(r) > leadCopyLimit {
		return string(r[:leadCopyLimit]) + "\n\n(The rest is in " + member + "'s chat.)"
	}
	return text
}

// copyToLead posts the copy of what the person wrote to chat id, when the chat
// reports to a lead. A failure is logged and does not stop the person's
// message, which has already been sent.
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
	from := 0
	if tr, err := readTranscript(id); err == nil {
		from = len(tr.Messages)
	}
	member := a.sessionTitle(id, "")
	if member == "" {
		member = "a chat that reports to you"
	}
	if attachments > 0 {
		text += fmt.Sprintf("\n\n(With %d attachment(s), which stay in %s's chat.)", attachments, member)
	}
	posted := false
	err = a.busHost().With(func(b *bus.Bus) error {
		body := fmt.Sprintf("Copy for %s: %s wrote to %s directly. No reply is needed.\n\n%s", lead.Title, b.PersonName(), member, clipForLead(text, member))
		_, err := b.SendActionRecipients(context.Background(), bus.Sender{}, "#"+lead.Group, body, nil, 0, []string{lead.Bus})
		posted = err == nil
		return err
	})
	if err != nil {
		logf("bus", "warning", "the copy for %s of the person's message to %s could not be posted: %v", lead.Title, member, err)
	}
	if posted {
		a.leadCopies.Store(id, pendingLeadCopy{group: lead.Group, lead: lead.Bus, leadTitle: lead.Title, member: member, from: from})
	}
}

// copyAnswerToLead posts the copy of a member's answer once the turn that
// answered the person's message has ended. A turn that ends before the
// person's message has run, as when it was queued behind another, leaves the
// copy waiting for the next one.
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
		var body string
		if len(parts) == 0 {
			body = fmt.Sprintf("Copy for %s: %s's turn ended without an answer to %s. No reply is needed.", pending.leadTitle, pending.member, b.PersonName())
		} else {
			body = fmt.Sprintf("Copy for %s: %s answered %s. No reply is needed.\n\n%s", pending.leadTitle, pending.member, b.PersonName(), clipForLead(strings.Join(parts, "\n\n"), pending.member))
		}
		_, err := b.SendActionRecipients(context.Background(), bus.Sender{}, "#"+pending.group, body, nil, 0, []string{pending.lead})
		return err
	})
	if err != nil {
		logf("bus", "warning", "the copy for %s of %s's answer could not be posted: %v", pending.leadTitle, pending.member, err)
	}
}
