package bus

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// InboxPage uses stable message IDs for paging and explicit bulk boundaries.
type InboxPage struct {
	Items      []Message
	Unread     int
	NextBefore int64
	Newest     int64
}

func (b *Bus) HumanInboxPage(ctx context.Context, view string, before int64, limit int) (InboxPage, error) {
	out := InboxPage{Items: []Message{}}
	filter := ""
	switch view {
	case "", "unread":
		filter = " AND delivered_at IS NULL AND inbox_dismissed_at IS NULL"
	case "read":
		filter = " AND delivered_at IS NOT NULL AND inbox_dismissed_at IS NULL"
	case "dismissed":
		filter = " AND inbox_dismissed_at IS NOT NULL"
	case "all":
	default:
		return out, errors.New("unknown Inbox view")
	}
	if before < 0 {
		return out, errors.New("invalid Inbox cursor")
	}
	if limit < 1 || limit > 200 {
		limit = 200
	}
	if err := b.db.QueryRowContext(ctx, `SELECT COALESCE(MAX(message_id),0),COUNT(DISTINCT CASE WHEN delivered_at IS NULL AND inbox_dismissed_at IS NULL THEN message_id END) FROM deliveries WHERE recipient_kind='human' AND cancelled_at IS NULL`).Scan(&out.Newest, &out.Unread); err != nil {
		return out, err
	}
	rows, err := b.db.QueryContext(ctx, `SELECT DISTINCT message_id FROM deliveries WHERE recipient_kind='human' AND cancelled_at IS NULL AND message_id<=? AND (?=0 OR message_id<?)`+filter+` ORDER BY message_id DESC LIMIT ?`, out.Newest, before, before, limit+1)
	if err != nil {
		return out, err
	}
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return out, err
		}
		ids = append(ids, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return out, err
	}
	if len(ids) > limit {
		ids = ids[:limit]
		out.NextBefore = ids[len(ids)-1]
	}
	for _, id := range ids {
		m, err := b.HumanInboxMessage(ctx, id)
		if err != nil {
			return out, err
		}
		out.Items = append(out.Items, m)
	}
	return out, nil
}

// HumanInboxMessage checks Inbox membership without changing read state.
func (b *Bus) HumanInboxMessage(ctx context.Context, id int64) (Message, error) {
	var unread, dismissed bool
	err := b.db.QueryRowContext(ctx, `SELECT delivered_at IS NULL,inbox_dismissed_at IS NOT NULL FROM deliveries WHERE recipient_kind='human' AND cancelled_at IS NULL AND message_id=? LIMIT 1`, id).Scan(&unread, &dismissed)
	if errors.Is(err, sql.ErrNoRows) {
		return Message{}, errors.New("this message is not in your Inbox")
	}
	if err != nil {
		return Message{}, err
	}
	m, err := b.Message(ctx, id)
	m.Unread, m.Dismissed = unread, dismissed
	return m, err
}

// ChangeHumanInbox records read or dismissal state without acknowledging an answer.
// A through operation is explicitly bounded by a displayed snapshot's highest ID.
func (b *Bus) ChangeHumanInbox(ctx context.Context, id int64, through, dismiss bool) error {
	if id <= 0 {
		return errors.New("choose an Inbox message or a nonempty snapshot")
	}
	if !through {
		if _, err := b.HumanInboxMessage(ctx, id); err != nil {
			return err
		}
	}
	column := "delivered_at"
	if dismiss {
		column = "inbox_dismissed_at"
	}
	comparison := "="
	if through {
		comparison = "<="
	}
	where := fmt.Sprintf("recipient_kind='human' AND cancelled_at IS NULL AND message_id%s? AND %s IS NULL", comparison, column)
	var count int
	if err := b.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM deliveries WHERE "+where, id).Scan(&count); err != nil || count == 0 {
		return err
	}
	return b.tx(ctx, func(t *sql.Tx) error {
		_, err := t.Exec("UPDATE deliveries SET "+column+"=? WHERE "+where, b.stamp(), id)
		return err
	})
}

// RoomMessagesFrom validates the target and includes preceding context through newest.
func (b *Bus) RoomMessagesFrom(ctx context.Context, room string, id int64, preceding int) ([]Message, error) {
	m, err := b.Message(ctx, id)
	if err != nil {
		return nil, err
	}
	if m.Room != room || room == "" {
		return nil, errors.New("the target message is not in this group")
	}
	preceding = max(0, min(100, preceding))
	start := id
	if preceding > 0 {
		if err := b.db.QueryRowContext(ctx, `SELECT COALESCE(MIN(id),?) FROM (SELECT id FROM messages WHERE room=? AND id<? ORDER BY id DESC LIMIT ?)`, id, room, id, preceding).Scan(&start); err != nil {
			return nil, err
		}
	}
	return b.queryMessages(ctx, messageSelect+` WHERE m.room=? AND m.id>=? ORDER BY m.id`, room, start)
}
