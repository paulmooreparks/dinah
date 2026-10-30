package bus

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
)

// ResultKind identifies a host completion report. Its embedded output has
// the authority of its source, and the report itself asks for no reply.
const ResultKind = "host_result"

// HostReport is the record a host result keeps of an invocation: what was
// asked, how it ended, and what came back. The message keeps it as JSON,
// which makes it the stable identity of the result across a restart, and
// FormatOne writes it out as text a person and an agent can read.
//
// Text is the result as the action wrote it for a reader, in Markdown, and
// From is who produced it, such as the chat that answered, as a person would
// say it. Both are optional and left out of the JSON when empty, so a report
// made before they existed is kept, and compared, byte for byte as it was.
type HostReport struct {
	InvocationID string          `json:"invocationId"`
	Request      json.RawMessage `json:"request"`
	Status       string          `json:"status"`
	Output       json.RawMessage `json:"output,omitempty"`
	Error        string          `json:"error,omitempty"`
	Text         string          `json:"text,omitempty"`
	From         string          `json:"from,omitempty"`
}

// EnqueueResult records a host result and its delivery in one transaction.
// The stable key makes reconciliation safe after a restart or a lost reply.
// It addresses a session ID even when that session is currently closed.
// This is a host API and is not exposed as an agent messaging tool.
func (b *Bus) EnqueueResult(ctx context.Context, key, sessionID, text string) (Message, error) {
	if key == "" || sessionID == "" || strings.TrimSpace(text) == "" {
		return Message{}, errors.New("a host result needs a key, a recipient and text")
	}
	var id int64
	err := b.tx(ctx, func(t *sql.Tx) error {
		var previousRecipient, previous string
		err := t.QueryRow(`SELECT h.message_id, h.recipient, m.text FROM host_results h JOIN messages m ON m.id = h.message_id WHERE h.key = ?`, key).Scan(&id, &previousRecipient, &previous)
		if err == nil {
			if previousRecipient != sessionID || previous != text {
				return errors.New("the host result key already identifies another result")
			}
			return nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		title, err := sessionLabel(t, sessionID)
		if err != nil {
			return err
		}
		r := chatResolved(sessionID, title)
		id, err = b.insertMessage(t, Sender{}, ResultKind, r, 0, text, "", "", nil)
		if err != nil {
			return err
		}
		// A host report must never be attributed to the person. It has no
		// agent hop chain, since it completes an already accepted operation.
		// The sender's name is stored with the message, which makes it
		// wire, so it stays Dinah's whatever the product's profile says.
		if _, err := t.Exec(`UPDATE messages SET from_name = 'Dinah' WHERE id = ?`, id); err != nil {
			return err
		}
		_, err = t.Exec(`INSERT INTO host_results (key, recipient, message_id) VALUES (?, ?, ?)`, key, sessionID, id)
		return err
	})
	if err != nil {
		return Message{}, err
	}
	return b.Message(ctx, id)
}
