package bus

import (
	"context"
	"database/sql"
)

// SteeringSubmission keeps the original payload before a provider can accept it.
// Recovery never resends a pending submission: a lost receipt is ambiguous.
type SteeringSubmission struct {
	ID      int64  `json:"id"`
	Session string `json:"sessionId"`
	Payload string `json:"payload"`
	State   string `json:"state"`
}

func (b *Bus) SaveSteeringSubmission(ctx context.Context, session, payload string) (int64, error) {
	var id int64
	err := b.tx(ctx, func(t *sql.Tx) error {
		r, err := t.Exec(`INSERT INTO steering_submissions(session,payload,state,created_at) VALUES (?,?,'pending',?)`, session, payload, b.stamp())
		if err != nil {
			return err
		}
		id, err = r.LastInsertId()
		return err
	})
	return id, err
}

func (b *Bus) SetSteeringSubmissionState(ctx context.Context, session string, id int64, state string) error {
	return b.tx(ctx, func(t *sql.Tx) error {
		_, err := t.Exec(`UPDATE steering_submissions SET state=? WHERE id=? AND session=?`, state, id, session)
		return err
	})
}

func (b *Bus) RecoverSteeringSubmissions(ctx context.Context, session string) ([]SteeringSubmission, error) {
	rows, err := b.db.QueryContext(ctx, `SELECT id,session,payload,state FROM steering_submissions WHERE session=? AND state IN ('pending','queued','failed') ORDER BY id`, session)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []SteeringSubmission{}
	for rows.Next() {
		var s SteeringSubmission
		if err := rows.Scan(&s.ID, &s.Session, &s.Payload, &s.State); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}
