package bus

import (
	"context"
	"database/sql"
	"errors"
)

// PersonRequestIdentity reads the authoritative structured source, including
// closed requests. Notification routing must not depend on a list projection.
func (b *Bus) PersonRequestIdentity(ctx context.Context, id int64) (string, error) {
	var exists bool
	if err := b.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM person_requests WHERE message_id=?)`, id).Scan(&exists); err != nil {
		return "", err
	}
	if !exists {
		return "", nil
	}
	return decisionIdentity("request", id), nil
}

// AcknowledgeMention is the person's exact-message navigation acknowledgement.
// It changes attention metadata only and refuses structured requests.
func (b *Bus) AcknowledgeMention(ctx context.Context, id int64) error {
	return b.tx(ctx, func(tx *sql.Tx) error {
		var eligible bool
		if err := tx.QueryRow(`SELECT EXISTS(SELECT 1 FROM messages m JOIN deliveries d ON d.message_id=m.id
		 WHERE m.id=? AND m.kind='text' AND m.from_session<>'' AND d.recipient_kind='human'
		 AND d.cancelled_at IS NULL AND NOT EXISTS(SELECT 1 FROM person_requests p WHERE p.message_id=m.id))`, id).Scan(&eligible); err != nil {
			return err
		}
		if !eligible {
			return errors.New("This message is not a mention addressed to you.")
		}
		_, err := tx.Exec(`INSERT INTO mention_acknowledgements(message_id,opened_at) VALUES(?,?) ON CONFLICT(message_id) DO NOTHING`, id, b.stamp())
		return err
	})
}

// MentionAttentionHandled observes navigation or a reply without answering a
// source request. Group replies clear preceding plain mentions in that chat.
func (b *Bus) MentionAttentionHandled(ctx context.Context, id int64) (bool, error) {
	var handled bool
	err := b.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM messages m WHERE m.id=? AND (
	 EXISTS(SELECT 1 FROM mention_acknowledgements a WHERE a.message_id=m.id)
	 OR EXISTS(SELECT 1 FROM messages r WHERE r.reply_to=m.id AND r.from_session='' AND r.from_name='paul' AND r.kind='text')
	 OR (m.room<>'' AND NOT EXISTS(SELECT 1 FROM person_requests p WHERE p.message_id=m.id)
	 AND EXISTS(SELECT 1 FROM messages r WHERE r.room=m.room AND r.id>m.id AND r.from_session='' AND r.from_name='paul' AND r.kind='text'))))`, id).Scan(&handled)
	return handled, err
}
