package bus

import "context"

// PendingSessionDeliveries counts all direct deliveries still owned by this
// recipient. It does not apply wake filters, read or acknowledge any message.
func (b *Bus) PendingSessionDeliveries(ctx context.Context, recipient string) (int, error) {
	var count int
	err := b.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM deliveries WHERE recipient_kind = 'session' AND recipient = ? AND delivered_at IS NULL AND cancelled_at IS NULL AND NOT EXISTS (SELECT 1 FROM sessions s WHERE s.id=deliveries.recipient AND s.removed_at IS NOT NULL)`, recipient).Scan(&count)
	return count, err
}

// NewestSessionDelivery returns the ID of the newest message sent directly to
// this recipient, or zero when none was. It counts every delivery, whether it
// still waits, was delivered or was cancelled, so the result changes only when
// a new message arrives for the recipient. A delivery or a withdrawal never
// lowers it. Like PendingSessionDeliveries it reads nothing on the
// recipient's behalf.
func (b *Bus) NewestSessionDelivery(ctx context.Context, recipient string) (int64, error) {
	var newest int64
	err := b.db.QueryRowContext(ctx, `SELECT COALESCE(MAX(message_id), 0) FROM deliveries WHERE recipient_kind = 'session' AND recipient = ?`, recipient).Scan(&newest)
	return newest, err
}
