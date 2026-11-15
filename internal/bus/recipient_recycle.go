package bus

import (
	"context"
	"database/sql"
)

// BinRecipient withdraws an address without consuming or cancelling its queue.
// The existing removed-recipient trigger refuses every new direct delivery.
func (b *Bus) BinRecipient(ctx context.Context, id string) error {
	if id == "" {
		return nil
	}
	return b.tx(ctx, func(t *sql.Tx) error {
		if _, err := t.Exec(`INSERT INTO recipient_recycle(recipient,restored) VALUES (?,0) ON CONFLICT(recipient) DO UPDATE SET restored=0`, id); err != nil {
			return err
		}
		_, err := t.Exec(`UPDATE sessions SET removed_at=COALESCE(removed_at,?),ended_at=COALESCE(ended_at,?) WHERE id=?`, b.stamp(), b.stamp(), id)
		return err
	})
}

// RestoreRecipient restores the address closed. Its retained queue stays
// dormant until manual registration or a new delivery, without losing bytes.
func (b *Bus) RestoreRecipient(ctx context.Context, id string) error {
	if id == "" {
		return nil
	}
	return b.tx(ctx, func(t *sql.Tx) error {
		if _, err := t.Exec(`UPDATE sessions SET removed_at=NULL,ended_at=? WHERE id=? AND EXISTS (SELECT 1 FROM recipient_recycle r WHERE r.recipient=sessions.id)`, b.stamp(), id); err != nil {
			return err
		}
		_, err := t.Exec(`UPDATE recipient_recycle SET restored=1 WHERE recipient=?`, id)
		return err
	})
}

// ForgetBinnedRecipient permanently disposes delivery rows and membership.
// Messages remain in the bus history under its existing retention policy.
func (b *Bus) ForgetBinnedRecipient(ctx context.Context, id string) error {
	if id == "" {
		return nil
	}
	if err := b.RemoveRecipient(ctx, id); err != nil {
		return err
	}
	return b.tx(ctx, func(t *sql.Tx) error {
		if _, err := t.Exec(`DELETE FROM deliveries WHERE recipient_kind='session' AND recipient=?`, id); err != nil {
			return err
		}
		_, err := t.Exec(`DELETE FROM recipient_recycle WHERE recipient=?`, id)
		return err
	})
}
