package bus

import (
	"context"
	"database/sql"
	"errors"
	"slices"
)

// AnswerOrigin is constructed only by AnswerDecision, alongside its reply.
// Its question and answer strings are snapshots after the bus's redaction.
type AnswerOrigin struct {
	Version   int                `json:"version"`
	RequestID string             `json:"requestId"`
	MessageID int64              `json:"messageId"`
	Kind      string             `json:"kind"`
	Questions []AnsweredQuestion `json:"questions"`
}

type AnsweredQuestion struct {
	Number        int    `json:"number"`
	Text          string `json:"text"`
	Answer        string `json:"answer"`
	AsRecommended bool   `json:"asRecommended"`
}

func (a *AnswerOrigin) Clone() *AnswerOrigin {
	if a == nil {
		return nil
	}
	out := *a
	out.Questions = slices.Clone(a.Questions)
	return &out
}

type DecisionDetail struct {
	Decision Decision `json:"decision"`
	Closed   bool     `json:"closed"`
	ReadOnly bool     `json:"readOnly"`
}

// DecisionByID reads a request, including a closed one, without changing
// its delivery, attention or mutation state.
func (b *Bus) DecisionByID(ctx context.Context, identity string) (DecisionDetail, error) {
	out := DecisionDetail{ReadOnly: true}
	id, err := decisionMessageID(identity)
	if err != nil {
		return out, err
	}
	tx, err := b.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return out, err
	}
	defer tx.Rollback()
	d, closed, err := b.decision(tx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return out, errors.New("this decision is no longer available")
	}
	if err != nil {
		return out, err
	}
	if d.ID != identity {
		return out, errors.New("decision identity has the wrong kind")
	}
	if d.Kind == "mention" {
		var eligible bool
		err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM messages m JOIN deliveries d ON d.message_id=m.id WHERE m.id=? AND m.kind='text' AND m.from_session<>'' AND m.room<>'' AND d.recipient_kind='human')`, id).Scan(&eligible)
		if err != nil {
			return out, err
		}
		if !eligible {
			return out, errors.New("this decision is no longer available")
		}
	}
	d.CanDismiss, d.DismissalReason = false, ""
	out.Decision, out.Closed = d, closed
	return out, tx.Commit()
}
