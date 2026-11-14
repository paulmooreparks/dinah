package bus

import (
	"context"
	"strconv"
)

// DecisionEntry keeps attention navigation independent of full question bodies.
type DecisionEntry struct {
	ID        string `json:"id"`
	MessageID int64  `json:"messageId"`
}

type DecisionEntryState struct {
	Count int             `json:"count"`
	Items []DecisionEntry `json:"items"`
}

func decisionIdentity(kind string, message int64) string {
	return kind + ":" + strconv.FormatInt(message, 10)
}

// DecisionEntries uses the same open-row predicate as Decisions in one SQL read.
// It neither marks messages read nor changes requests or attention cards.
func (b *Bus) DecisionEntries(ctx context.Context) (DecisionEntryState, error) {
	out := DecisionEntryState{Items: []DecisionEntry{}}
	rows, err := b.db.QueryContext(ctx, `SELECT m.id, CASE WHEN p.message_id IS NULL THEN 'mention' ELSE 'request' END`+openDecisionSQL+` ORDER BY m.id DESC`)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var entry DecisionEntry
		var kind string
		if err := rows.Scan(&entry.MessageID, &kind); err != nil {
			return out, err
		}
		entry.ID = decisionIdentity(kind, entry.MessageID)
		out.Items = append(out.Items, entry)
	}
	if err := rows.Err(); err != nil {
		return out, err
	}
	out.Count = len(out.Items)
	return out, nil
}
