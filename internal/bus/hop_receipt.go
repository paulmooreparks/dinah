package bus

import "context"

// HopHold describes a pending recipient excluded by the direct-message hop
// guard. It is an observation for receipts and changes no delivery state.
type HopHold struct {
	Recipient string
	Limit     int
}

// HopLimitedRecipients uses the same configured limit as the doorbell.
func (b *Bus) HopLimitedRecipients(ctx context.Context, messageID int64) ([]HopHold, error) {
	limit := hopLimitSQL("m.from_workspace", "s.workspace")
	rows, err := b.db.QueryContext(ctx, `SELECT DISTINCT s.id, `+limit+`
		FROM deliveries d JOIN messages m ON m.id = d.message_id
		JOIN sessions s ON (d.recipient_kind = 'session' AND d.recipient = s.id)
			OR (d.recipient_kind = 'workspace' AND d.recipient = s.workspace AND s.id <> m.from_session)
		WHERE m.id = ? AND m.room = '' AND d.delivered_at IS NULL AND d.cancelled_at IS NULL
		AND s.removed_at IS NULL AND NOT EXISTS (SELECT 1 FROM recipient_recycle rr WHERE rr.recipient = s.id)
		AND `+limit+` <> ? AND m.hops > `+limit+` ORDER BY 1`, messageID, NoHopLimit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []HopHold
	for rows.Next() {
		var hold HopHold
		if err := rows.Scan(&hold.Recipient, &hold.Limit); err != nil {
			return nil, err
		}
		out = append(out, hold)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()
	for i := range out {
		s, err := b.Session(ctx, out[i].Recipient)
		if err != nil {
			return nil, err
		}
		out[i].Recipient = s.Display()
	}
	return out, nil
}
