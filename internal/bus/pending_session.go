package bus

import "context"

// PendingSessionDeliveries counts all direct deliveries still owned by this
// recipient. It does not apply wake filters, read or acknowledge any message.
func (b *Bus) PendingSessionDeliveries(ctx context.Context, recipient string) (int, error) {
	var count int
	err := b.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM deliveries WHERE recipient_kind = 'session' AND recipient = ? AND delivered_at IS NULL AND cancelled_at IS NULL`, recipient).Scan(&count)
	return count, err
}
