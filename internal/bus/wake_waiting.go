package bus

import (
	"context"
	"fmt"
)

// WakeSender identifies an original sender whose actionable message still waits.
// First makes a notice idempotent when a watcher retries or restarts.
type WakeSender struct {
	Session string
	First   int64
}

func (b *Bus) WakeSenders(ctx context.Context, target string) ([]WakeSender, error) {
	limit := hopLimitSQL("m.from_workspace", "s.workspace")
	rows, err := b.db.QueryContext(ctx, `SELECT CASE WHEN m.from_session = '' THEN ? ELSE m.from_session END, MIN(m.id)
		FROM deliveries d JOIN messages m ON m.id = d.message_id
		JOIN sessions s ON d.recipient_kind = 'session' AND d.recipient = s.id
		WHERE s.id = ? AND s.removed_at IS NULL AND d.delivered_at IS NULL AND d.cancelled_at IS NULL
		AND m.kind NOT IN ('`+ResultKind+`', '`+MemberKind+`')
		AND (m.kind = '`+QuestionKind+`' OR (`+budgetOpenSQL+` AND (m.room <> '' OR `+limit+` = `+fmt.Sprint(NoHopLimit)+` OR m.hops <= `+limit+`)))
		AND (m.room = '' OR m.mentions LIKE '%,' || s.id || ',%' OR m.mentions LIKE '%,*,%')
		GROUP BY m.from_session ORDER BY MIN(m.id)`, Human, target)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []WakeSender
	for rows.Next() {
		var sender WakeSender
		if err := rows.Scan(&sender.Session, &sender.First); err != nil {
			return nil, err
		}
		out = append(out, sender)
	}
	return out, rows.Err()
}

// NotifyWakeWaiting sends a host notice, never a reply attributed to the person.
// The caller supplies a chat's display title, not a path or message contents.
func (b *Bus) NotifyWakeWaiting(ctx context.Context, target, title string, sender WakeSender, limit int) error {
	key := fmt.Sprintf("background-wait:%s:%s:%d", target, sender.Session, sender.First)
	if queued, err := b.ResultQueued(ctx, key); err != nil || queued {
		return err
	}
	text := fmt.Sprintf("%s has not been opened for your message because all %d places for background chats are in use. Its message remains queued and it will open when a place becomes available. This notice needs no reply.", title, limit)
	_, err := b.EnqueueResult(ctx, key, sender.Session, text)
	return err
}
