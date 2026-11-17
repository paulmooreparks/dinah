package bus

import (
	"context"
	"database/sql"
	"strings"
)

// Relays. A team member's message to the person that was rerouted to its lead has a relay record.
// The lead's choice not to relay is its own judgement and is no failure; a relay fails when the
// delivery to the lead fails, when the lead's turn on it ends in an error, or when the lead cannot
// run. The window observes each where it happens and marks the relay failed with the reason, and
// then tells the person directly (teams.md, Paul's answers).
//
// A relay is handled once the lead's turn on it has ended without an error: the lead has had the
// message and has done with it what it judged right, which may be to pass it on over the bus, to
// answer the person in its own chat, or to do nothing. Until 17 November 2026 a relay stayed
// pending for good, and its member counted as waiting until its lead next sent a message on the
// bus, so a member whose lead answered the person in its own chat waited for ever.

// Relay states.
const (
	RelayPending = "pending"
	RelayFailed  = "failed"
	RelayHandled = "handled"
)

// Relay is the record of a rerouted message.
type Relay struct {
	MessageID int64
	Member    string
	Lead      string
	CreatedAt string
	State     string
	Reason    string
	Reported  bool
}

func scanRelays(rows *sql.Rows) ([]Relay, error) {
	defer rows.Close()
	var out []Relay
	for rows.Next() {
		var r Relay
		if err := rows.Scan(&r.MessageID, &r.Member, &r.Lead, &r.CreatedAt, &r.State, &r.Reason, &r.Reported); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

const relayColumns = `r.message_id, r.member, r.lead, r.created_at, r.state, r.reason, r.reported`

// PendingRelays are the relays that have not failed and whose delivery the lead has not yet taken, which
// is where a lead that cannot run is found.
func (b *Bus) PendingRelays(ctx context.Context) ([]Relay, error) {
	rows, err := b.db.QueryContext(ctx, `SELECT `+relayColumns+` FROM relays r
		WHERE r.state = 'pending' AND EXISTS (SELECT 1 FROM deliveries d WHERE d.message_id = r.message_id
			AND d.recipient_kind = 'session' AND d.recipient = r.lead AND d.delivered_at IS NULL AND d.cancelled_at IS NULL) ORDER BY r.message_id`)
	if err != nil {
		return nil, err
	}
	return scanRelays(rows)
}

// FailRelay marks one relay failed, with the reason the person is told. A relay already failed keeps its
// first reason.
func (b *Bus) FailRelay(ctx context.Context, message int64, reason string) error {
	return b.tx(ctx, func(t *sql.Tx) error {
		_, err := t.Exec(`UPDATE relays SET state = 'failed', reason = ? WHERE message_id = ? AND state = 'pending'`, strings.TrimSpace(reason), message)
		return err
	})
}

// FailPendingRelaysTo marks failed the pending relays to a lead that it has not taken, for a delivery
// to the lead that could not be made. It returns their message numbers.
func (b *Bus) FailPendingRelaysTo(ctx context.Context, lead, reason string) ([]int64, error) {
	var failed []int64
	err := b.tx(ctx, func(t *sql.Tx) error {
		rows, err := t.Query(`SELECT r.message_id FROM relays r WHERE r.lead = ? AND r.state = 'pending' AND EXISTS
			(SELECT 1 FROM deliveries d WHERE d.message_id = r.message_id AND d.recipient_kind = 'session' AND d.recipient = r.lead AND d.delivered_at IS NULL AND d.cancelled_at IS NULL)`, lead)
		if err != nil {
			return err
		}
		for rows.Next() {
			var id int64
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return err
			}
			failed = append(failed, id)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		for _, id := range failed {
			if _, err := t.Exec(`UPDATE relays SET state = 'failed', reason = ? WHERE message_id = ?`, strings.TrimSpace(reason), id); err != nil {
				return err
			}
		}
		return nil
	})
	return failed, err
}

// FailRelaysIn marks failed the pending relays among the messages a lead's turn was given, for a turn that
// ended in an error. It returns their message numbers.
func (b *Bus) FailRelaysIn(ctx context.Context, lead string, messages []int64, reason string) ([]int64, error) {
	var failed []int64
	err := b.tx(ctx, func(t *sql.Tx) error {
		for _, id := range messages {
			res, err := t.Exec(`UPDATE relays SET state = 'failed', reason = ? WHERE message_id = ? AND lead = ? AND state = 'pending'`, strings.TrimSpace(reason), id, lead)
			if err != nil {
				return err
			}
			if n, _ := res.RowsAffected(); n > 0 {
				failed = append(failed, id)
			}
		}
		return nil
	})
	return failed, err
}

// RelaysHandledIn marks handled the pending relays among the messages a lead's turn was given, for a turn that
// ended without an error. It returns their message numbers.
func (b *Bus) RelaysHandledIn(ctx context.Context, lead string, messages []int64) ([]int64, error) {
	var handled []int64
	err := b.tx(ctx, func(t *sql.Tx) error {
		for _, id := range messages {
			res, err := t.Exec(`UPDATE relays SET state = 'handled' WHERE message_id = ? AND lead = ? AND state = 'pending'`, id, lead)
			if err != nil {
				return err
			}
			if n, _ := res.RowsAffected(); n > 0 {
				handled = append(handled, id)
			}
		}
		return nil
	})
	return handled, err
}

// HandleTakenRelays marks handled the pending relays whose delivery the lead has taken, but for the ones
// given: the relays of turns still running, which end by themselves. A window calls it once as it starts,
// when a relay taken before it started belongs to a turn that is over. It returns their message numbers.
func (b *Bus) HandleTakenRelays(ctx context.Context, running []int64) ([]int64, error) {
	skip := map[int64]bool{}
	for _, id := range running {
		skip[id] = true
	}
	var handled []int64
	err := b.tx(ctx, func(t *sql.Tx) error {
		rows, err := t.Query(`SELECT r.message_id FROM relays r WHERE r.state = 'pending' AND EXISTS
			(SELECT 1 FROM deliveries d WHERE d.message_id = r.message_id AND d.recipient_kind = 'session' AND d.recipient = r.lead AND d.delivered_at IS NOT NULL)`)
		if err != nil {
			return err
		}
		for rows.Next() {
			var id int64
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return err
			}
			if !skip[id] {
				handled = append(handled, id)
			}
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		for _, id := range handled {
			if _, err := t.Exec(`UPDATE relays SET state = 'handled' WHERE message_id = ? AND state = 'pending'`, id); err != nil {
				return err
			}
		}
		return nil
	})
	return handled, err
}

// UnreportedFailures are the failed relays the person has not been told of, with their messages.
func (b *Bus) UnreportedFailures(ctx context.Context) ([]Relay, []Message, error) {
	rows, err := b.db.QueryContext(ctx, `SELECT `+relayColumns+` FROM relays r WHERE r.state = 'failed' AND r.reported = 0 ORDER BY r.message_id`)
	if err != nil {
		return nil, nil, err
	}
	relays, err := scanRelays(rows)
	if err != nil {
		return nil, nil, err
	}
	messages := make([]Message, 0, len(relays))
	for _, r := range relays {
		m, err := b.Message(ctx, r.MessageID)
		if err != nil {
			return nil, nil, err
		}
		messages = append(messages, m)
	}
	return relays, messages, nil
}

// RelayReported records that the person was told of a failed relay, so that it is told once.
func (b *Bus) RelayReported(ctx context.Context, message int64) error {
	return b.tx(ctx, func(t *sql.Tx) error {
		_, err := t.Exec(`UPDATE relays SET reported = 1 WHERE message_id = ?`, message)
		return err
	})
}

// RelayOf is the relay record of a message, and false when it was not rerouted.
func (b *Bus) RelayOf(ctx context.Context, message int64) (Relay, bool, error) {
	rows, err := b.db.QueryContext(ctx, `SELECT `+relayColumns+` FROM relays r WHERE r.message_id = ?`, message)
	if err != nil {
		return Relay{}, false, err
	}
	rs, err := scanRelays(rows)
	if err != nil || len(rs) == 0 {
		return Relay{}, false, err
	}
	return rs[0], true, nil
}
