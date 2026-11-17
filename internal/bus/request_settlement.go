package bus

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"studio/internal/requestsettlement"
)

// DismissRequest is called by the trusted person window, never a chat tool.
func (b *Bus) DismissRequest(ctx context.Context, in requestsettlement.Input) (requestsettlement.Receipt, error) {
	return b.settleRequest(ctx, "person", "Dismiss", in)
}

// WithdrawRequest receives the invoking chat resolved by the authenticated tool.
func (b *Bus) WithdrawRequest(ctx context.Context, from Session, in requestsettlement.Input) (requestsettlement.Receipt, error) {
	if from.ID == "" {
		return requestsettlement.Refuse(in, "not_authorized", "The asking chat could not be identified."), nil
	}
	return b.settleRequest(ctx, from.ID, "Withdraw", in)
}

func (b *Bus) settleRequest(ctx context.Context, actor, command string, in requestsettlement.Input) (requestsettlement.Receipt, error) {
	out := requestsettlement.Refuse(in, "invalid_request", "The request identity, revision or operation ID is invalid.")
	if !requestsettlement.ValidOperationID(in.OperationID) || in.Revision < 1 {
		return out, nil
	}
	id, err := decisionMessageID(in.ID)
	if err != nil {
		return out, nil
	}
	fp := decisionFingerprint(command, in)
	operationActor := actor
	if command == "Withdraw" {
		operationActor = requestsettlement.ChatActor(actor)
	}
	err = b.tx(ctx, func(t *sql.Tx) error {
		var encoded, old string
		err := t.QueryRow(`SELECT fingerprint,receipt FROM request_settlement_operations WHERE actor=? AND command=? AND operation_id=?`, operationActor, command, in.OperationID).Scan(&old, &encoded)
		if err == nil {
			if old != fp {
				out = requestsettlement.Refuse(in, "operation_changed", "This operation ID already describes a different request.")
				return nil
			}
			out = requestsettlement.Receipt{}
			return json.Unmarshal([]byte(encoded), &out)
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		d, closed, err := b.decision(t, id)
		if errors.Is(err, sql.ErrNoRows) {
			out = requestsettlement.Refuse(in, "request_unavailable", "This request is unavailable.")
			return nil
		}
		if err != nil {
			return err
		}
		if d.ID != in.ID {
			return nil
		}
		if d.Kind != "request" {
			out = requestsettlement.Refuse(in, "unsupported_source", "This message belongs to its chat and has no request settlement command.")
			return nil
		}
		if command == "Withdraw" {
			available, err := decisionSessionAvailable(t, actor)
			if err != nil {
				return err
			}
			if actor != d.AskerSession || !available {
				out = requestsettlement.Refuse(in, "not_authorized", "Only the available asking chat can withdraw its request.")
				return nil
			}
		}
		if closed {
			out = requestsettlement.Refuse(in, "request_closed", "This request is already closed.")
			out.Outcome = d.Status
			return nil
		}
		if d.Revision != in.Revision {
			out = requestsettlement.Refuse(in, "revision_changed", "This request changed. Refresh it before continuing.")
			return nil
		}
		release, err := requestsettlement.Claim(b.dir, operationActor, command, in)
		if errors.Is(err, requestsettlement.ErrOperationChanged) {
			out = requestsettlement.Refuse(in, "operation_changed", err.Error())
			return nil
		}
		if err != nil {
			return err
		}
		defer release()
		status, kind, wording := "dismissed", "person", "The person dismissed this request without answering."
		if command == "Withdraw" {
			status, kind, wording = "withdrawn", "owner", "The asking chat withdrew this request without answering."
		}
		stamp := b.stamp()
		if _, err = t.Exec(`INSERT INTO request_settlements VALUES(?,?,?,?)`, d.ID, status, stamp, kind); err != nil {
			return err
		}
		if d.Kind == "request" {
			if _, err = t.Exec(`UPDATE person_requests SET revision=revision+1,closed=1 WHERE message_id=?`, id); err != nil {
				return err
			}
		}
		if _, err = t.Exec(`UPDATE deliveries SET inbox_dismissed_at=? WHERE message_id=? AND recipient_kind='human'`, stamp, id); err != nil {
			return err
		}
		if _, err = t.Exec(`INSERT INTO request_completion_obligations(request_id,recipient,outcome,wording) VALUES(?,?,?,?)`, d.ID, d.AskerSession, status, wording); err != nil {
			return err
		}
		out = requestsettlement.Receipt{OperationID: in.OperationID, ID: d.ID, Revision: d.Revision + 1, State: "committed", Outcome: status, SettledAt: stamp}
		data, err := json.Marshal(out)
		if err != nil {
			return err
		}
		_, err = t.Exec(`INSERT INTO request_settlement_operations VALUES(?,?,?,?,?)`, operationActor, command, in.OperationID, fp, string(data))
		return err
	})
	// Dispatch failure leaves the durable obligation for later reconciliation.
	if err == nil && out.State == "committed" {
		_ = b.DispatchRequestCompletions(ctx)
	}
	return out, err
}

// RequestOperation is scoped by its trusted actor and command, without mutation.
func (b *Bus) RequestOperation(ctx context.Context, actor, command, operation string) (requestsettlement.Receipt, error) {
	out := requestsettlement.Receipt{OperationID: operation, State: "unknown"}
	if !requestsettlement.ValidOperationID(operation) {
		return out, errors.New("invalid operation ID")
	}
	var encoded string
	if command == "Withdraw" {
		actor = requestsettlement.ChatActor(actor)
	}
	err := b.db.QueryRowContext(ctx, `SELECT receipt FROM request_settlement_operations WHERE actor=? AND command=? AND operation_id=?`, actor, command, operation).Scan(&encoded)
	if errors.Is(err, sql.ErrNoRows) {
		return out, nil
	}
	if err != nil {
		return out, err
	}
	err = json.Unmarshal([]byte(encoded), &out)
	return out, err
}

// DispatchRequestCompletions inserts the notice and records its identity in one
// transaction. Retrying after a lost commit response cannot insert another notice.
func (b *Bus) DispatchRequestCompletions(ctx context.Context) error {
	var ready int
	err := b.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM request_completion_obligations o JOIN sessions s ON s.id=o.recipient
 WHERE o.message_id=0 AND s.ended_at IS NULL AND s.removed_at IS NULL
 AND NOT EXISTS(SELECT 1 FROM team_members tm WHERE tm.member=s.id AND tm.retired=1)`).Scan(&ready)
	if err != nil || ready == 0 {
		return err
	}
	return b.tx(ctx, func(t *sql.Tx) error {
		rows, err := t.Query(`SELECT request_id,recipient,outcome,wording FROM request_completion_obligations WHERE message_id=0 AND recipient<>''`)
		if err != nil {
			return err
		}
		type notice struct{ id, recipient, outcome, text string }
		pending := []notice{}
		for rows.Next() {
			var n notice
			if err = rows.Scan(&n.id, &n.recipient, &n.outcome, &n.text); err != nil {
				rows.Close()
				return err
			}
			pending = append(pending, n)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		for _, n := range pending {
			available, err := decisionSessionAvailable(t, n.recipient)
			if err != nil {
				return err
			}
			if !available {
				continue
			}
			var open int
			if err = t.QueryRow(`SELECT COUNT(*) FROM sessions WHERE id=? AND ended_at IS NULL`, n.recipient).Scan(&open); err != nil {
				return err
			}
			if open == 0 {
				continue
			}
			label, err := sessionLabel(t, n.recipient)
			if err != nil {
				return err
			}
			output, _ := json.Marshal(map[string]string{"id": n.id, "outcome": n.outcome})
			report, _ := json.Marshal(HostReport{InvocationID: "request-settlement:" + n.id, Status: "done", Request: json.RawMessage(`{}`), Output: output, Text: n.text + " Request: " + n.id})
			id, err := b.insertMessage(t, Sender{}, ResultKind, chatResolved(n.recipient, label), 0, string(report), "", "", nil)
			if err != nil {
				return err
			}
			if err = signHostReport(t, id); err != nil {
				return err
			}
			if _, err = t.Exec(`UPDATE request_completion_obligations SET message_id=? WHERE request_id=? AND message_id=0`, id, n.id); err != nil {
				return err
			}
		}
		return nil
	})
}

// RequestCompletionAvailable observes the original recipient without waking it.
func (b *Bus) RequestCompletionAvailable(ctx context.Context, recipient string) (bool, error) {
	var n int
	err := b.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sessions s WHERE s.id=? AND s.ended_at IS NULL AND s.removed_at IS NULL
 AND NOT EXISTS(SELECT 1 FROM team_members tm WHERE tm.member=s.id AND tm.retired=1)`, recipient).Scan(&n)
	return n == 1, err
}

func (b *Bus) RequestOwnerAvailable(ctx context.Context, owner string) (bool, error) {
	t, err := b.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return false, err
	}
	defer t.Rollback()
	return decisionSessionAvailable(t, owner)
}
