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
		for _, query := range []string{
			`DELETE FROM steers WHERE session=? OR session IN (SELECT agent_session FROM agent_sessions WHERE session=?)`,
			`DELETE FROM steering_submissions WHERE session=? OR session IN (SELECT agent_session FROM agent_sessions WHERE session=?)`,
			`DELETE FROM hook_turns WHERE session=? OR session IN (SELECT agent_session FROM agent_sessions WHERE session=?)`,
		} {
			if _, err := t.Exec(query, id, id); err != nil {
				return err
			}
		}
		for _, query := range []string{
			`DELETE FROM host_results WHERE recipient=?`,
			`DELETE FROM agent_sessions WHERE session=?`,
			`DELETE FROM team_usage WHERE session=?`,
			`DELETE FROM team_budgets WHERE lead=?`,
			`DELETE FROM recipient_recycle WHERE recipient=?`,
		} {
			if _, err := t.Exec(query, id); err != nil {
				return err
			}
		}
		_, err := t.Exec(`UPDATE sessions SET name='',workspace='',title='',started_at='',last_seen='',stop_continues=0,hops=0 WHERE id=?`, id)
		return err
	})
}

// StoredSessionDeliveries reports retained undelivered queue ownership, even
// when ordinary pending counts suppress a binned recipient.
func (b *Bus) StoredSessionDeliveries(ctx context.Context, id string) (int, error) {
	var count int
	err := b.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM deliveries WHERE recipient_kind='session' AND recipient=? AND delivered_at IS NULL AND cancelled_at IS NULL`, id).Scan(&count)
	return count, err
}

// RecipientAvailable reports whether an existing address admits new messages.
// A closed, restored address is available even while its old queue is dormant.
func (b *Bus) RecipientAvailable(ctx context.Context, id string) (bool, error) {
	var available bool
	err := b.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM sessions WHERE id=? AND removed_at IS NULL)`, id).Scan(&available)
	return available, err
}
