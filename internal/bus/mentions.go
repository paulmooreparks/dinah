package bus

import (
	"context"
	"strings"
)

// A message mentions the person when an agent sent it to them: it was sent to
// the person directly, or to a group chat with paul in its mentions, and so
// the person has a delivery of kind "human", which is their inbox. A group
// message that does not mention them has a delivery of kind humanRoom
// instead, and counts only in the group's window. The deliveries are the
// bus's own data, so a mention is found the same way whichever program sent it
// (docs/plans/mention-notification.md).

// kindsNotMentions are the messages that are not an agent asking something of
// the person: completion reports, lines saying who joined a group, and
// another chat's question.
var kindsNotMentions = "('" + ResultKind + "', '" + MemberKind + "', '" + QuestionKind + "')"

// Mentions returns the messages that agents sent the person which the person
// has not read, with numbers above after, oldest first. It changes nothing.
func (b *Bus) Mentions(ctx context.Context, after int64) ([]Message, error) {
	rows, err := b.db.QueryContext(ctx, `SELECT m.id FROM deliveries d JOIN messages m ON m.id = d.message_id
		WHERE d.recipient_kind = 'human' AND d.delivered_at IS NULL AND m.from_session <> '' AND m.id > ?
			AND m.kind NOT IN `+kindsNotMentions+` ORDER BY m.id`, after)
	if err != nil {
		return nil, err
	}
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := make([]Message, 0, len(ids))
	for _, id := range ids {
		m, err := b.Message(ctx, id)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, nil
}

// HighestMessage is the number of the newest message on the bus, or 0.
func (b *Bus) HighestMessage(ctx context.Context) (int64, error) {
	var n int64
	err := b.db.QueryRowContext(ctx, `SELECT COALESCE(MAX(id), 0) FROM messages`).Scan(&n)
	return n, err
}

// MentionsRead returns those of the messages whose mention of the person has
// been read, in the inbox or in the group's window.
func (b *Bus) MentionsRead(ctx context.Context, ids []int64) ([]int64, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	args := make([]any, len(ids))
	marks := make([]string, len(ids))
	for i, id := range ids {
		args[i], marks[i] = id, "?"
	}
	rows, err := b.db.QueryContext(ctx, `SELECT d.message_id FROM deliveries d WHERE d.recipient_kind = 'human' AND d.delivered_at IS NOT NULL
		AND d.message_id IN (`+strings.Join(marks, ",")+`)`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}
