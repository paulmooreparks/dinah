package bus

import (
	"context"
	"database/sql"
	"errors"
	"strings"
)

// QuestionKind is the kind of a question one chat asks another through the
// window's chat applet (dinah.chat/ask@2). It is sent from the asking chat's
// session to the asked chat's, so it wakes the asked chat as any message
// sent to a chat does (Waking), and a closed chat is opened for it. The bus
// never hands a question over as an ordinary message (Deliver, Doorbell):
// the window's invocation that asked it takes it (TakeQuestion) and runs it
// as a turn whose words return to the asking chat, or withdraws it when it
// stops waiting (WithdrawQuestion). A question is not subject to the hop
// limit, since the window refuses a loop of chats asking each other and a
// chat answering a question cannot ask another in that turn.
const QuestionKind = "question"

// ErrQuestionGone is returned for a question that has been taken or
// withdrawn already.
var ErrQuestionGone = errors.New("the question has already been taken or withdrawn")

// Ask records a question from the session from to the session to, which may
// be closed, and its delivery to that session.
func (b *Bus) Ask(ctx context.Context, from Session, to, text string) (Message, error) {
	if strings.TrimSpace(text) == "" {
		return Message{}, errors.New("the question is empty")
	}
	if from.ID == "" || to == "" || from.ID == to {
		return Message{}, errors.New("a question goes from one chat's session to another's")
	}
	var message Message
	err := b.tx(ctx, func(t *sql.Tx) error {
		title, err := sessionLabel(t, to)
		if errors.Is(err, sql.ErrNoRows) {
			return errors.New("the bus does not know the chat asked")
		}
		if err != nil {
			return err
		}
		id, err := b.insertMessage(t, Sender{Session: &from}, QuestionKind, chatResolved(to, title), 0, text, "", "", nil)
		if err != nil {
			return err
		}
		// Capture the result before committing. A cancelled post-commit read
		// would hide the ID the caller needs to withdraw its question.
		// Questions have neither attachments nor mentions.
		message, err = scanMessage(t.QueryRowContext(ctx, messageSelect+` WHERE m.id = ?`, id))
		return err
	})
	if err != nil {
		return Message{}, err
	}
	return message, nil
}

// TakeQuestion marks the question id delivered to the session that was
// asked it, and returns it. It returns ErrQuestionGone for a question taken
// or withdrawn already. The session's next message continues the question's
// chain of hops, as a message that Deliver hands over does.
func (b *Bus) TakeQuestion(ctx context.Context, sessionID string, id int64) (Message, error) {
	err := b.tx(ctx, func(t *sql.Tx) error {
		var delivery int64
		var hops int
		err := t.QueryRow(`SELECT d.id, m.hops FROM deliveries d JOIN messages m ON m.id = d.message_id
			WHERE d.message_id = ? AND m.kind = '`+QuestionKind+`' AND d.recipient_kind = 'session' AND d.recipient = ?
			AND d.delivered_at IS NULL`, id, sessionID).Scan(&delivery, &hops)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrQuestionGone
		}
		if err != nil {
			return err
		}
		now := b.stamp()
		if _, err := t.Exec(`UPDATE deliveries SET delivered_at = ?, delivered_session = ? WHERE id = ?`, now, sessionID, delivery); err != nil {
			return err
		}
		_, err = t.Exec(`UPDATE sessions SET last_seen = ?, hops = ? WHERE id = ?`, now, hops, sessionID)
		return err
	})
	if err != nil {
		return Message{}, err
	}
	m, err := b.Message(ctx, id)
	m.Asked = true
	return m, err
}

// WithdrawQuestion takes back a question that has not been taken, so that
// it no longer wakes the chat it was for. The question stays among the
// bus's messages, as a record that it was asked; only its delivery goes.
func (b *Bus) WithdrawQuestion(ctx context.Context, id int64) error {
	return b.tx(ctx, func(t *sql.Tx) error {
		_, err := t.Exec(`DELETE FROM deliveries WHERE message_id = ? AND delivered_at IS NULL
			AND EXISTS (SELECT 1 FROM messages m WHERE m.id = ? AND m.kind = '`+QuestionKind+`')`, id, id)
		return err
	})
}

// WaitingQuestions returns the questions that have been neither taken nor
// withdrawn, oldest first.
func (b *Bus) WaitingQuestions(ctx context.Context) ([]int64, error) {
	rows, err := b.db.QueryContext(ctx, `SELECT DISTINCT d.message_id FROM deliveries d JOIN messages m ON m.id = d.message_id
		WHERE d.delivered_at IS NULL AND m.kind = '`+QuestionKind+`' ORDER BY d.message_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}
