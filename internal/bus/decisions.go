package bus

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"studio/internal/appletcontext"
	"studio/internal/requestsettlement"
)

type PersonQuestion struct {
	Number         int      `json:"number"`
	Text           string   `json:"text"`
	Recommendation string   `json:"recommendation"`
	Choices        []string `json:"choices"`
	Answered       bool     `json:"answered"`
	Answer         string   `json:"answer"`
}

type AskPersonInput struct {
	OperationID string           `json:"operationId"`
	Group       string           `json:"group,omitempty"`
	Questions   []PersonQuestion `json:"questions"`
}

type Decision struct {
	requestsettlement.Disposition
	ID              string           `json:"id"`
	Revision        int              `json:"revision"`
	MessageID       int64            `json:"messageId"`
	Kind            string           `json:"kind"`
	CreatedAt       string           `json:"createdAt"`
	AskerSession    string           `json:"askerSession"`
	Asker           string           `json:"asker"`
	Workspace       string           `json:"workspace"`
	Group           string           `json:"group"`
	GroupTitle      string           `json:"groupTitle"`
	Destination     string           `json:"destination"`
	Text            string           `json:"text"`
	Questions       []PersonQuestion `json:"questions"`
	CanDismiss      bool             `json:"canDismiss"`
	DismissalReason string           `json:"dismissalReason"`
}

type DecisionPage struct {
	Items      []Decision `json:"items"`
	NextBefore int64      `json:"nextBefore"`
}

type DecisionAnswer struct {
	Number        int    `json:"number"`
	Text          string `json:"text"`
	AsRecommended bool   `json:"asRecommended"`
}

type AnswerDecisionInput struct {
	OperationID string           `json:"operationId"`
	ID          string           `json:"id"`
	Revision    int              `json:"revision"`
	Answers     []DecisionAnswer `json:"answers"`
}

type DismissDecisionInput struct {
	OperationID string `json:"operationId"`
	ID          string `json:"id"`
	Revision    int    `json:"revision"`
}

type DecisionReceipt struct {
	OperationID string             `json:"operationId"`
	State       string             `json:"state"`
	ID          string             `json:"id"`
	Revision    int                `json:"revision"`
	Closed      bool               `json:"closed"`
	MessageIDs  []int64            `json:"messageIds"`
	Reason      *DecisionRejection `json:"reason"`
}

type DecisionRejection struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func decisionText(s string, max int) bool {
	return utf8.ValidString(s) && strings.TrimSpace(s) != "" && utf8.RuneCountInString(s) <= max
}

func decisionOperationID(s string) bool { return decisionText(s, 128) }

func decisionFingerprint(kind string, value any) string {
	encoded, _ := json.Marshal(value)
	sum := sha256.Sum256(append([]byte(kind+"\n"), encoded...))
	return hex.EncodeToString(sum[:])
}

// AskPerson commits the durable request together with its bus message. Unlike
// ordinary team progress, this explicit decision goes to the person directly.
func (b *Bus) AskPerson(ctx context.Context, from Session, in AskPersonInput) (Decision, error) {
	var out Decision
	if from.ID == "" || !decisionOperationID(in.OperationID) || len(in.Questions) < 1 || len(in.Questions) > 20 {
		return out, errors.New("Ask between 1 and 20 numbered questions.")
	}
	fingerprint := decisionFingerprint("ask", in)
	seen := map[int]bool{}
	questions := make([]PersonQuestion, len(in.Questions))
	for i, q := range in.Questions {
		if q.Number < 1 || int64(q.Number) > 9007199254740991 || seen[q.Number] || !decisionText(q.Text, 8000) || !decisionText(q.Recommendation, 8000) || len(q.Choices) > 20 || q.Answered || q.Answer != "" {
			return out, errors.New("Give each question a different positive number, its text and a recommendation.")
		}
		seen[q.Number] = true
		q.Text = appletcontext.Redact(q.Text)
		q.Recommendation = appletcontext.Redact(q.Recommendation)
		q.Choices = append([]string{}, q.Choices...)
		for j, c := range q.Choices {
			if !decisionText(c, 2000) {
				return out, errors.New("One of the suggested answers could not be read.")
			}
			q.Choices[j] = appletcontext.Redact(c)
		}
		questions[i] = q
	}
	person := b.PersonName()
	err := b.tx(ctx, func(t *sql.Tx) error {
		var id int64
		var old string
		err := t.QueryRow(`SELECT message_id,fingerprint FROM person_requests WHERE asker=? AND operation_id=?`, from.ID, in.OperationID).Scan(&id, &old)
		if err == nil {
			if old != fingerprint {
				return errors.New("The questions changed after this request was saved. Submit a new request.")
			}
			out, _, err = b.decision(t, id)
			return err
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		r := resolved{label: Human, toKind: "human", recipients: []recipient{{"human", Human}}}
		if in.Group != "" {
			r, err = groupResolved(t, Sender{Session: &from}, in.Group)
			if err != nil {
				return err
			}
			r.mentions = "," + Human + ","
			r.recipients = append(r.recipients, recipient{"human", Human})
		}
		var words strings.Builder
		words.WriteString("A decision from " + person + " is requested.\n")
		for _, q := range questions {
			fmt.Fprintf(&words, "\nQuestion %d. %s\nRecommendation: %s\n", q.Number, q.Text, q.Recommendation)
			if len(q.Choices) > 0 {
				fmt.Fprintf(&words, "Choices: %s\n", strings.Join(q.Choices, "; "))
			}
		}
		id, err = b.insertMessage(t, Sender{Session: &from}, "text", r, 0, words.String(), "", "", nil)
		if err != nil {
			return err
		}
		encoded, _ := json.Marshal(questions)
		if _, err = t.Exec(`INSERT INTO person_requests (message_id,asker,operation_id,fingerprint,revision,closed,questions) VALUES (?,?,?,?,1,0,?)`, id, from.ID, in.OperationID, fingerprint, string(encoded)); err != nil {
			return err
		}
		out, _, err = b.decision(t, id)
		return err
	})
	return out, err
}

// Decisions contains structured requests. Plain mentions remain in their chat
// and Notifications, independent of delivery read state or the first-use mark.
const activeDecisionSQL = ` FROM messages m JOIN person_requests p ON p.message_id=m.id WHERE p.closed=0 `

const openDecisionSQL = activeDecisionSQL + ` AND NOT EXISTS (SELECT 1 FROM sessions removed WHERE removed.id=m.from_session AND removed.removed_at IS NOT NULL)`

func decisionMessageID(id string) (int64, error) {
	parts := strings.Split(id, ":")
	if len(parts) != 2 || (parts[0] != "request" && parts[0] != "mention") {
		return 0, errors.New("This request could not be identified. Refresh the list.")
	}
	n, e := strconv.ParseInt(parts[1], 10, 64)
	if e != nil || n < 1 || strconv.FormatInt(n, 10) != parts[1] {
		return 0, errors.New("This request could not be identified. Refresh the list.")
	}
	return n, nil
}

// DecisionForMessage resolves a projection's original source identity without
// requiring the page to guess whether it was an explicit request or a mention.
func (b *Bus) DecisionForMessage(ctx context.Context, message int64) (Decision, error) {
	t, err := b.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return Decision{}, err
	}
	defer t.Rollback()
	d, _, err := b.decision(t, message)
	return d, err
}

func (b *Bus) decision(t *sql.Tx, id int64) (Decision, bool, error) {
	out := Decision{Questions: []PersonQuestion{}, Revision: 1, Kind: "mention"}
	m, err := scanMessage(t.QueryRow(messageSelect+` WHERE m.id=?`, id))
	if err != nil {
		return out, false, err
	}
	out.MessageID, out.CreatedAt, out.AskerSession, out.Asker, out.Workspace = m.ID, m.CreatedAt, m.FromSession, m.From, m.FromWorkspaceLabel
	out.Group, out.GroupTitle, out.Text = m.Room, m.RoomTitle, m.Text
	out.Destination = m.From
	if m.Room != "" {
		out.Destination = "#" + m.RoomTitle
	}
	var encoded string
	var closed bool
	err = t.QueryRow(`SELECT revision,closed,questions FROM person_requests WHERE message_id=?`, id).Scan(&out.Revision, &closed, &encoded)
	if err == nil {
		out.Kind = "request"
		if err = json.Unmarshal([]byte(encoded), &out.Questions); err != nil {
			return out, false, err
		}
	} else if errors.Is(err, sql.ErrNoRows) {
		var count int
		if err = t.QueryRow(`SELECT COUNT(*)`+openDecisionSQL+` AND m.id=?`, id).Scan(&count); err != nil {
			return out, false, err
		}
		closed = count == 0
	} else {
		return out, false, err
	}
	out.ID = decisionIdentity(out.Kind, id)
	out.Status = "open"
	if closed {
		out.Status = "answered"
	}
	err = t.QueryRow(`SELECT status,settled_at,actor_kind FROM request_settlements WHERE request_id=?`, out.ID).Scan(&out.Status, &out.SettledAt, &out.ActorKind)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return out, false, err
	}
	if out.SettledAt != "" {
		closed = true
		if out.Kind == "mention" {
			out.Revision++
		}
	}
	if !closed {
		out.CanDismiss = true
		if out.Kind == "request" {
			available, err := decisionDestinationAvailable(t, out)
			if err != nil {
				return out, false, err
			}
			if !available {
				out.DismissalReason = "The asking chat or group is no longer available."
			}
		}
	}
	return out, closed, nil
}

func (b *Bus) Decisions(ctx context.Context, before int64) (DecisionPage, error) {
	out := DecisionPage{Items: []Decision{}}
	if before < 0 {
		return out, errors.New("Earlier requests could not be identified. Refresh the list.")
	}
	t, err := b.db.BeginTx(ctx, nil)
	if err != nil {
		return out, err
	}
	defer t.Rollback()
	rows, err := t.Query(`SELECT m.id`+openDecisionSQL+` AND (?=0 OR m.id<?) ORDER BY m.id DESC LIMIT 101`, before, before)
	if err != nil {
		return out, err
	}
	ids := []int64{}
	for rows.Next() {
		var id int64
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return out, err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	if len(ids) > 100 {
		ids = ids[:100]
		out.NextBefore = ids[len(ids)-1]
	}
	for _, id := range ids {
		d, _, err := b.decision(t, id)
		if err != nil {
			return out, err
		}
		out.Items = append(out.Items, d)
	}
	return out, nil
}

func decisionReceipt(t *sql.Tx, op, fingerprint string) (DecisionReceipt, bool, error) {
	out := DecisionReceipt{OperationID: op, State: "unknown", MessageIDs: []int64{}}
	var stored, encoded string
	err := t.QueryRow(`SELECT fingerprint,receipt FROM decision_operations WHERE operation_id=?`, op).Scan(&stored, &encoded)
	if errors.Is(err, sql.ErrNoRows) {
		return out, false, nil
	}
	if err != nil {
		return out, false, err
	}
	if fingerprint != "" && stored != fingerprint {
		return out, false, errors.New("This request changed after it was saved. Submit a new request.")
	}
	err = json.Unmarshal([]byte(encoded), &out)
	return out, true, err
}

func storeDecisionReceipt(t *sql.Tx, fingerprint string, r DecisionReceipt) error {
	encoded, _ := json.Marshal(r)
	_, err := t.Exec(`INSERT INTO decision_operations (operation_id,fingerprint,receipt) VALUES (?,?,?)`, r.OperationID, fingerprint, string(encoded))
	return err
}

func rejectDecision(t *sql.Tx, op, fp, id string, revision int, closed bool, code, message string) (DecisionReceipt, error) {
	out := DecisionReceipt{OperationID: op, State: "rejected", ID: id, Revision: revision, Closed: closed, MessageIDs: []int64{}, Reason: &DecisionRejection{Code: code, Message: message}}
	return out, storeDecisionReceipt(t, fp, out)
}

func (b *Bus) DecisionOperation(ctx context.Context, op string) (DecisionReceipt, error) {
	out := DecisionReceipt{OperationID: op, State: "unknown", MessageIDs: []int64{}}
	if !decisionOperationID(op) {
		return out, errors.New("This action could not be identified. Refresh and try again.")
	}
	t, err := b.db.BeginTx(ctx, nil)
	if err != nil {
		return out, err
	}
	defer t.Rollback()
	previous, found, err := decisionReceipt(t, op, "")
	if err != nil || found {
		return previous, err
	}
	if err := t.Rollback(); err != nil {
		return out, err
	}
	settled, err := b.RequestOperation(ctx, "person", "Dismiss", op)
	if err != nil {
		return out, err
	}
	if settled.State == "committed" {
		return DecisionReceipt{OperationID: op, ID: settled.ID, Revision: settled.Revision, State: "committed", Closed: true, MessageIDs: []int64{}}, nil
	}
	return out, nil
}

func (b *Bus) AnswerDecision(ctx context.Context, in AnswerDecisionInput) (DecisionReceipt, error) {
	var out DecisionReceipt
	if !decisionOperationID(in.OperationID) || in.Revision < 1 || len(in.Answers) < 1 || len(in.Answers) > 20 {
		return out, errors.New("Provide an answer before sending.")
	}
	id, err := decisionMessageID(in.ID)
	if err != nil {
		return out, err
	}
	// Syntax failures are not operation outcomes. Validate them before reading
	// mutable state, so stale input cannot hide a malformed answer.
	seenAnswers := map[int]bool{}
	mention := strings.HasPrefix(in.ID, "mention:")
	for _, answer := range in.Answers {
		if seenAnswers[answer.Number] {
			return out, errors.New("The same question was answered twice.")
		}
		seenAnswers[answer.Number] = true
		if mention && (len(in.Answers) != 1 || answer.Number != 0 || answer.AsRecommended) || !mention && answer.Number < 1 {
			return out, errors.New("The answer does not refer to a valid question number.")
		}
		if answer.AsRecommended && answer.Text != "" || !answer.AsRecommended && !decisionText(answer.Text, 8000) {
			return out, errors.New("Write an answer or choose As Recommended.")
		}
	}
	fp := decisionFingerprint("answer", in)
	person := b.PersonName()
	err = b.tx(ctx, func(t *sql.Tx) error {
		var found bool
		var err error
		out, found, err = decisionReceipt(t, in.OperationID, fp)
		if err != nil || found {
			return err
		}
		d, closed, err := b.decision(t, id)
		if errors.Is(err, sql.ErrNoRows) {
			out, err = rejectDecision(t, in.OperationID, fp, in.ID, 0, true, "decision_unavailable", "This request is no longer available. Refresh the list.")
			return err
		}
		if err != nil {
			return err
		}
		if d.ID != in.ID {
			return errors.New("This request could not be identified. Refresh the list.")
		}
		indexes := map[int]int{}
		for i, q := range d.Questions {
			indexes[q.Number] = i
		}
		if d.Kind == "request" {
			for _, answer := range in.Answers {
				if _, ok := indexes[answer.Number]; !ok {
					return errors.New("This question could not be found. Refresh the request.")
				}
			}
		}
		if closed {
			out, err = rejectDecision(t, in.OperationID, fp, in.ID, d.Revision, true, "decision_closed", "This request is already closed. Refresh the list.")
			return err
		}
		if d.Revision != in.Revision {
			out, err = rejectDecision(t, in.OperationID, fp, in.ID, d.Revision, false, "revision_changed", "This request changed. Refresh it before sending your answers.")
			return err
		}
		available, err := decisionDestinationAvailable(t, d)
		if err != nil {
			return err
		}
		if !available {
			out, err = rejectDecision(t, in.OperationID, fp, in.ID, d.Revision, false, "decision_unavailable", "The asking chat or group is no longer available. No answer was sent.")
			return err
		}
		var words strings.Builder
		origin := &AnswerOrigin{Version: 1, RequestID: d.ID, MessageID: d.MessageID, Kind: d.Kind, Questions: []AnsweredQuestion{}}
		recommended := map[int]bool{}
		for _, a := range in.Answers {
			recommended[a.Number] = a.AsRecommended
			if d.Kind == "mention" {
				words.WriteString(a.Text)
				origin.Questions = append(origin.Questions, AnsweredQuestion{Number: 0, Text: appletcontext.Redact(d.Text), Answer: appletcontext.Redact(a.Text)})
				continue
			}
			index := indexes[a.Number]
			if d.Questions[index].Answered {
				out, err = rejectDecision(t, in.OperationID, fp, in.ID, d.Revision, false, "already_answered", "One of these questions has already been answered. Refresh the request.")
				return err
			}
			q := &d.Questions[index]
			answer := a.Text
			if a.AsRecommended {
				answer = q.Recommendation
			}
			q.Answered, q.Answer = true, appletcontext.Redact(answer)
			fmt.Fprintf(&words, "Question %d. %s\n%s answered: %s\n\n", q.Number, q.Text, person, q.Answer)
		}
		if d.Kind == "request" {
			for _, q := range d.Questions {
				if seenAnswers[q.Number] {
					origin.Questions = append(origin.Questions, AnsweredQuestion{Number: q.Number, Text: appletcontext.Redact(q.Text), Answer: q.Answer, AsRecommended: recommended[q.Number]})
				}
			}
		}
		r := chatResolved(d.AskerSession, d.Asker)
		if d.Group != "" {
			r, err = groupResolved(t, Sender{}, "#"+d.Group)
			if err != nil {
				return err
			}
			// A person answer addresses its original asker, including a closed chat.
			r.mentions = "," + d.AskerSession + ","
			r.recipients = append(r.recipients, recipient{"session", d.AskerSession})
		}
		reply, err := b.insertMessage(t, Sender{}, "text", r, id, words.String(), "", "", nil)
		if err != nil {
			return err
		}
		encodedOrigin, err := json.Marshal(origin)
		if err != nil {
			return err
		}
		if _, err = t.Exec(`INSERT INTO decision_answer_origins(message_id,origin) VALUES(?,?)`, reply, string(encodedOrigin)); err != nil {
			return err
		}
		closed = true
		if d.Kind == "request" {
			for _, q := range d.Questions {
				if !q.Answered {
					closed = false
				}
			}
			d.Revision++
			encoded, _ := json.Marshal(d.Questions)
			if _, err = t.Exec(`UPDATE person_requests SET revision=?,closed=?,questions=? WHERE message_id=?`, d.Revision, closed, string(encoded), id); err != nil {
				return err
			}
		}
		out = DecisionReceipt{OperationID: in.OperationID, State: "committed", ID: d.ID, Revision: d.Revision, Closed: closed, MessageIDs: []int64{reply}}
		if closed {
			stamp := b.stamp()
			if _, err = t.Exec(`INSERT INTO request_settlements VALUES(?,?,?,?)`, d.ID, "answered", stamp, "person"); err != nil {
				return err
			}
			if _, err = t.Exec(`INSERT INTO request_completion_obligations VALUES(?,?,?,?,?)`, d.ID, d.AskerSession, "answered", "The person answered this request.", reply); err != nil {
				return err
			}
		}
		return storeDecisionReceipt(t, fp, out)
	})
	return out, err
}

func (b *Bus) DismissDecision(ctx context.Context, in DismissDecisionInput) (DecisionReceipt, error) {
	if !decisionOperationID(in.OperationID) || in.Revision < 1 {
		return DecisionReceipt{}, errors.New("This action could not be identified. Refresh and try again.")
	}
	if _, err := decisionMessageID(in.ID); err != nil {
		return DecisionReceipt{}, err
	}
	// Old dismissal receipts remain authoritative after the adapter changes.
	t, err := b.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return DecisionReceipt{}, err
	}
	previous, found, lookupErr := decisionReceipt(t, in.OperationID, decisionFingerprint("dismiss", in))
	rollbackErr := t.Rollback()
	if lookupErr != nil || rollbackErr != nil {
		return DecisionReceipt{}, errors.Join(lookupErr, rollbackErr)
	}
	if found {
		return previous, nil
	}
	r, err := b.DismissRequest(ctx, requestsettlement.Input{OperationID: in.OperationID, ID: in.ID, Revision: in.Revision})
	out := DecisionReceipt{OperationID: r.OperationID, ID: r.ID, Revision: r.Revision, State: r.State, Closed: r.State == "committed", MessageIDs: []int64{}}
	if r.State == "refused" {
		out.State = "rejected"
		out.Reason = &DecisionRejection{Code: r.Code, Message: r.Text}
		if r.Code == "request_closed" || r.Code == "unsupported_source" && strings.HasPrefix(in.ID, "mention:") {
			out.Closed = true
			out.Reason.Code = "decision_closed"
		}
	}
	return out, err
}

func decisionToken() (string, error) {
	var bytes [24]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes[:]), nil
}

func decisionSessionAvailable(t *sql.Tx, id string) (bool, error) {
	var count int
	err := t.QueryRow(`SELECT COUNT(*) FROM sessions s WHERE s.id=? AND s.removed_at IS NULL
  AND NOT EXISTS (SELECT 1 FROM team_members tm WHERE tm.member=s.id AND tm.retired=1)`, id).Scan(&count)
	return count == 1, err
}

func decisionDestinationAvailable(t *sql.Tx, d Decision) (bool, error) {
	available, err := decisionSessionAvailable(t, d.AskerSession)
	if err != nil || !available || d.Group == "" {
		return available, err
	}
	var members int
	err = t.QueryRow(`SELECT COUNT(*) FROM room_members rm JOIN rooms r ON r.name=rm.room WHERE rm.room=? AND rm.member_kind='session' AND rm.member=?`, d.Group, d.AskerSession).Scan(&members)
	return members == 1, err
}
