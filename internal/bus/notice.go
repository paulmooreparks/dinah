package bus

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// A group's message that does not mention a chat reaches it as a notice: one
// line saying who sent it and how it begins, with the message's number
// (docs/plans/group-attention.md). The notice starts no turn and waits for the
// chat's next one, and the chat reads the whole message with ReadMessage if it
// wants it. A message that mentions a chat, or everyone, is never a notice.

// NoticeLimit is how many notices one delivery shows for a group. A longer run
// is cut to the newest of them and a count of the earlier ones, so a busy group
// does not cost the context the notices were meant to save.
const NoticeLimit = 5

// NoticeLine is the one line that stands for a message: its first line, on one
// line and at most TitleLimit characters, as a title is cut (firstLine). A
// message with no text says how many files it carries.
func NoticeLine(m Message) string {
	if line := firstLine(m.Text); line != "" {
		return line
	}
	switch n := len(m.Attachments); n {
	case 0:
		return "(no text)"
	case 1:
		return "(1 attached file)"
	default:
		return fmt.Sprintf("(%d attached files)", n)
	}
}

// noticeEntry is the line that stands for a notice in an agent's prompt.
func noticeEntry(m Message) string {
	return fmt.Sprintf("[message %d] %s, %s: %s", m.ID, shortTime(m.CreatedAt), m.sender(), NoticeLine(m))
}

// formatNotices writes the notices of one delivery for an agent, a block for
// each group in the order its first notice came.
func formatNotices(notices []Message) string {
	var order []string
	byGroup := map[string][]Message{}
	for _, m := range notices {
		if _, ok := byGroup[m.Room]; !ok {
			order = append(order, m.Room)
		}
		byGroup[m.Room] = append(byGroup[m.Room], m)
	}
	var b strings.Builder
	for _, room := range order {
		group := byGroup[room]
		fmt.Fprintf(&b, "\nIn the group chat %q, these messages do not mention you, so each is shown as its first line. Read one whole with the bus's read_message tool (message_id), and answer in the group only if you have something the group needs.\n", group[0].groupName())
		if len(group) > NoticeLimit {
			cut := len(group) - NoticeLimit
			fmt.Fprintf(&b, "%d earlier messages, from message %d to message %d, are not shown; read_message reads them.\n", cut, group[0].ID, group[cut-1].ID)
			group = group[cut:]
		}
		for _, m := range group {
			b.WriteString(noticeEntry(m))
			b.WriteString("\n")
		}
	}
	return b.String()
}

// ReadMessage returns a group's message to a chat that is in the group, or that
// sent it or was sent it, whole, as an agent receives a message that mentions
// it. It is how a chat reads what a notice began.
func (b *Bus) ReadMessage(ctx context.Context, sessionID string, id int64) (Message, error) {
	s, err := b.Session(ctx, sessionID)
	if err != nil {
		return Message{}, err
	}
	m, err := b.Message(ctx, id)
	if err != nil {
		return Message{}, err
	}
	if m.Kind != "text" || m.Room == "" {
		return Message{}, fmt.Errorf("message %d is not a message in a group chat; read other messages with the inbox", id)
	}
	var one int
	err = b.db.QueryRowContext(ctx, `SELECT 1 FROM room_members WHERE room = ? AND member_kind = 'session' AND member = ?`, m.Room, s.ID).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		if !b.mayRead(ctx, s, m) {
			return Message{}, fmt.Errorf("message %d is in a group chat this chat is not in", id)
		}
	} else if err != nil {
		return Message{}, err
	}
	if err := b.db.QueryRowContext(ctx, `SELECT `+trustedSQL("m.from_workspace", "?")+` FROM messages m WHERE m.id = ?`, s.Workspace, s.Workspace, id).Scan(&m.Trusted); err != nil {
		return Message{}, err
	}
	return m, nil
}
