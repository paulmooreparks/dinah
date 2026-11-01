package bus

import (
	"context"
	"database/sql"
)

// RemoveRecipient is idempotent and preserves the difference between cancelled
// and delivered. A removed address cannot be registered or receive new messages.
func (b *Bus) RemoveRecipient(ctx context.Context, recipient string) error {
	if recipient == "" {
		return nil
	}
	return b.tx(ctx, func(t *sql.Tx) error {
		now := b.stamp()
		if _, err := t.Exec(`UPDATE sessions SET removed_at=COALESCE(removed_at,?),ended_at=COALESCE(ended_at,?) WHERE id=?`, now, now, recipient); err != nil {
			return err
		}
		if _, err := t.Exec(`UPDATE relays SET state='failed',reason='recipient chat removed' WHERE lead=? AND state='pending' AND EXISTS (SELECT 1 FROM deliveries d WHERE d.message_id=relays.message_id AND d.recipient_kind='session' AND d.recipient=? AND d.delivered_at IS NULL AND d.cancelled_at IS NULL)`, recipient, recipient); err != nil {
			return err
		}
		if _, err := t.Exec(`UPDATE deliveries SET cancelled_at=?,cancel_reason='recipient chat removed' WHERE recipient_kind='session' AND recipient=? AND delivered_at IS NULL AND cancelled_at IS NULL`, now, recipient); err != nil {
			return err
		}
		_, err := t.Exec(`DELETE FROM room_members WHERE member_kind='session' AND member=?`, recipient)
		return err
	})
}
