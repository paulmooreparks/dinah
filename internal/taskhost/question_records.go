package taskhost

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"studio/internal/records"
	"studio/internal/requestsettlement"
)

type questionExecutionKey struct{}
type questionExecution struct {
	window    *Window
	operation string
}

type questionIndex struct {
	ID    string `json:"id"`
	Scope Scope  `json:"scope"`
}

type QuestionRecord struct {
	Question  Question `json:"question"`
	Operation string   `json:"operation"`
	Scope     Scope    `json:"scope"`
	Revision  int      `json:"revision"`
	Recipient string   `json:"recipient,omitempty"`
	requestsettlement.Disposition
	Receipts   map[string]requestsettlement.Receipt `json:"receipts"`
	Completion *QuestionCompletion                  `json:"completion,omitempty"`
}

type QuestionCompletion struct {
	ID        string `json:"id"`
	Recipient string `json:"recipient,omitempty"`
	Outcome   string `json:"outcome"`
	Wording   string `json:"wording"`
	MessageID int64  `json:"messageId,omitempty"`
}

func (w *Window) admitQuestion(scope Scope, operation string, question Question, questions *Questions) error {
	w.mu.Lock()
	defer w.unlock()
	w.questions = questions
	r, err := w.read(taskKey(scope))
	if err != nil {
		return err
	}
	if w.closed || w.root.Err() != nil {
		return refusal("cancelled", "Question admission is closed.")
	}
	if r.Questions == nil {
		r.Questions = map[string]QuestionRecord{}
	}
	if _, ok := r.Questions[question.Key]; ok {
		return refusal("request_closed", "This question identity already exists.")
	}
	recipient := scope.OwnerChat
	if w.options.QuestionRecipient != nil {
		recipient = w.options.QuestionRecipient(scope)
	}
	if scope.OwnerChat != "" && recipient == "" {
		return refusal("unavailable", "The asking chat has no known completion destination.")
	}
	// The immutable index only locates the authoritative task record. It carries
	// no disposition and is safe to leave behind if the task write fails.
	b, _ := json.Marshal(questionIndex{ID: question.Key, Scope: scope})
	if err = (records.Store{Dir: filepath.Join(w.options.State, "task-question-index")}).Write(question.Key, b); err != nil {
		return err
	}
	r.Questions[question.Key] = QuestionRecord{Question: question, Operation: operation, Scope: scope, Recipient: recipient, Revision: 1, Disposition: requestsettlement.Disposition{Status: "open"}, Receipts: map[string]requestsettlement.Receipt{}}
	return w.write(r, r.Receipts[operation])
}

func (w *Window) QuestionRecord(key string) (QuestionRecord, error) {
	return ReadQuestionRecord(w.options.State, key)
}

// ReadQuestionRecord is passive and does not construct a task consumer.
func ReadQuestionRecord(state, key string) (QuestionRecord, error) {
	if !strings.HasPrefix(key, "task-q-") || len(key) > 256 || strings.ContainsAny(key, "/\\\x00") || filepath.Base(key) != key {
		return QuestionRecord{}, os.ErrNotExist
	}
	b, err := (records.Store{Dir: filepath.Join(state, "task-question-index")}).Read(key)
	if err != nil {
		return QuestionRecord{}, err
	}
	var index questionIndex
	if err = json.Unmarshal(b, &index); err != nil {
		return QuestionRecord{}, err
	}
	if index.ID != key || !validScope(index.Scope) {
		return QuestionRecord{}, refusal("persistence_failed", "The question index is invalid.")
	}
	b, err = (records.Store{Dir: filepath.Join(state, "task-operations")}).Read(taskKey(index.Scope))
	if err != nil {
		return QuestionRecord{}, err
	}
	var r taskRecord
	if err := json.Unmarshal(b, &r); err != nil || r.ID != taskKey(index.Scope) || r.Receipts == nil {
		return QuestionRecord{}, refusal("persistence_failed", "The question's task record is invalid.")
	}
	q, ok := r.Questions[key]
	if !ok {
		return QuestionRecord{}, os.ErrNotExist
	}
	if q.Question.Key != key || q.Scope != index.Scope || q.Revision < 1 || q.Operation == "" {
		return QuestionRecord{}, refusal("persistence_failed", "The saved question identity is inconsistent.")
	}
	return clone(q), nil
}

// QuestionRecords enumerates only immutable locators, then reads every source
// through its checksummed record. It does not restore an asking or execute work.
func QuestionRecords(state string) ([]QuestionRecord, error) {
	entries, err := os.ReadDir(filepath.Join(state, "task-question-index"))
	if errors.Is(err, os.ErrNotExist) {
		return []QuestionRecord{}, nil
	}
	if err != nil {
		return nil, err
	}
	keys := map[string]bool{}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		for _, suffix := range []string{".run0.json", ".run1.json", ".json"} {
			if strings.HasSuffix(entry.Name(), suffix) {
				keys[strings.TrimSuffix(entry.Name(), suffix)] = true
				break
			}
		}
	}
	ordered := make([]string, 0, len(keys))
	for key := range keys {
		ordered = append(ordered, key)
	}
	sort.Strings(ordered)
	out := []QuestionRecord{}
	for _, key := range ordered {
		q, err := ReadQuestionRecord(state, key)
		if errors.Is(err, os.ErrNotExist) {
			continue // A locator whose admission never committed grants no authority.
		}
		if err != nil {
			return nil, err
		}
		out = append(out, q)
	}
	return out, nil
}

// RecordQuestionNotice acknowledges dispatch without changing the operation's
// terminal receipt, sequence or original settlement receipt.
func (w *Window) RecordQuestionNotice(key string, message int64) error {
	q, err := w.QuestionRecord(key)
	if err != nil {
		return err
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.leases[taskKey(q.Scope)] == nil {
		release, err := w.store.Hold(taskKey(q.Scope))
		if err != nil {
			return err
		}
		defer release()
	}
	r, err := w.read(taskKey(q.Scope))
	if err != nil {
		return err
	}
	q = r.Questions[key]
	if message <= 0 || q.Status == "open" || q.Completion == nil {
		return refusal("stale", "The completed question could not be identified.")
	}
	if q.Completion.MessageID != 0 {
		if q.Completion.MessageID != message {
			return refusal("stale", "The question already has another completion notice.")
		}
		return nil
	}
	q.Completion.MessageID = message
	r.Questions[key] = q
	b, err := json.Marshal(r)
	if err != nil {
		return err
	}
	return w.store.Write(r.ID, b)
}

// reconcileResult observes a completed operation without attempting settlement.
func (w *Window) reconcileQuestion(scope Scope, key string) (QuestionRecord, questionResult, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	r, err := w.read(taskKey(scope))
	if err != nil {
		return QuestionRecord{}, questionResult{}, err
	}
	q, ok := r.Questions[key]
	if !ok || q.Scope != scope || q.Status == "open" {
		return QuestionRecord{}, questionResult{}, nil
	}
	if q.Completion == nil {
		return QuestionRecord{}, questionResult{}, refusal("persistence_failed", "The closed question has no completion obligation.")
	}
	result := questionResult{err: refusal(q.Status, q.Completion.Wording)}
	if q.Status == "answered" {
		var wrapped struct {
			Answer json.RawMessage `json:"answer"`
		}
		if err := json.Unmarshal(r.Receipts[q.Operation].Result, &wrapped); err != nil {
			return QuestionRecord{}, questionResult{}, err
		}
		if q.Question.AnswerType == "" {
			var selected string
			if err := json.Unmarshal(wrapped.Answer, &selected); err != nil {
				return QuestionRecord{}, questionResult{}, err
			}
			result = questionResult{answer: QuestionAnswer{OptionIDs: []string{selected}}}
		} else {
			result = questionResult{}
			if err := json.Unmarshal(wrapped.Answer, &result.answer); err != nil {
				return QuestionRecord{}, questionResult{}, err
			}
		}
	}
	return clone(q), result, nil
}

func (w *Window) commitQuestion(scope Scope, key, outcome, actor string, in requestsettlement.Input, answer *QuestionAnswer) (requestsettlement.Receipt, error) {
	out := requestsettlement.Refuse(in, "invalid_request", "The settlement operation is invalid.")
	if !requestsettlement.ValidOperationID(in.OperationID) || in.ID != key {
		return out, nil
	}
	// Read an existing result before acquiring current consumer authority. A
	// receipt remains observable after its task loses execution authority.
	w.mu.Lock()
	before, err := w.read(taskKey(scope))
	settlementContext := w.root
	if held := w.leases[taskKey(scope)]; held != nil {
		settlementContext = held.ctx
	}
	w.mu.Unlock()
	if err != nil {
		return out, err
	}
	prior, found := before.Questions[key]
	if !found {
		return requestsettlement.Refuse(in, "request_unavailable", "The question is unavailable."), nil
	}
	if prior.Scope != scope {
		return requestsettlement.Refuse(in, "not_authorized", "This question belongs to another owner."), nil
	}
	index := requestsettlement.Key(actor, outcome, in.OperationID)
	if previous, ok := prior.Receipts[index]; ok {
		if previous.ID != in.ID || previous.Revision != in.Revision+1 {
			return requestsettlement.Refuse(in, "operation_changed", "This operation ID describes different input."), nil
		}
		return previous, nil
	}
	if prior.Status != "open" {
		out = requestsettlement.Refuse(in, "request_closed", "This question has ended.")
		out.Outcome = prior.Status
		return out, nil
	}
	if outcome != "cancelled" {
		if w.options.HoldTask == nil {
			return requestsettlement.Refuse(in, "unavailable", "The task record cannot be held during settlement."), nil
		}
		release, err := w.options.HoldTask(settlementContext, scope)
		if err != nil || release == nil {
			return requestsettlement.Refuse(in, "not_authorized", "The task record is unavailable for settlement."), nil
		}
		defer release()
	}
	w.mu.Lock()
	// Questions holds its own settlement mutex here. Event callbacks run only
	// after that mutex is released by the caller.
	defer w.mu.Unlock()
	r, err := w.read(taskKey(scope))
	if err != nil {
		return out, err
	}
	q, ok := r.Questions[key]
	if !ok {
		return requestsettlement.Refuse(in, "request_unavailable", "The question is unavailable."), nil
	}
	if q.Scope != scope {
		return requestsettlement.Refuse(in, "not_authorized", "This question belongs to another owner."), nil
	}
	if previous, ok := q.Receipts[index]; ok {
		if previous.ID != in.ID || previous.Revision != in.Revision+1 {
			return requestsettlement.Refuse(in, "operation_changed", "This operation ID describes different input."), nil
		}
		return previous, nil
	}
	if q.Status != "open" {
		out = requestsettlement.Refuse(in, "request_closed", "This question has ended.")
		out.Outcome = q.Status
		return out, nil
	}
	if q.Revision != in.Revision {
		return requestsettlement.Refuse(in, "revision_changed", "This question changed."), nil
	}
	if outcome == "dismissed" || outcome == "withdrawn" || outcome == "answered" {
		if w.closed {
			return requestsettlement.Refuse(in, "cancelled", "The host is shutting down."), nil
		}
		if err = w.options.Check(settlementContext, scope); err != nil {
			return requestsettlement.Refuse(in, "not_authorized", err.Error()), nil
		}
		if l := w.leases[taskKey(scope)]; l == nil || l.revoked || l.ctx.Err() != nil {
			return requestsettlement.Refuse(in, "cancelled", "The task authority was revoked."), nil
		}
	}
	command := map[string]string{"dismissed": "Dismiss", "withdrawn": "Withdraw", "answered": "Answer", "cancelled": "Cancel"}[outcome]
	releaseOperation, err := requestsettlement.Claim(w.options.State, actor, command, in)
	if errors.Is(err, requestsettlement.ErrOperationChanged) {
		return requestsettlement.Refuse(in, "operation_changed", err.Error()), nil
	}
	if err != nil {
		return requestsettlement.Receipt{}, err
	}
	defer releaseOperation()
	stamp := w.options.Now().UTC().Format(time.RFC3339Nano)
	kind, wording := "owner", "The asking owner withdrew this request without answering."
	if outcome == "dismissed" {
		kind, wording = "person", "The person dismissed this request without answering."
	}
	if outcome == "answered" {
		kind, wording = "person", "The person answered this request."
	}
	if outcome == "cancelled" {
		kind, wording = "host", "The task request was cancelled or its authority was revoked."
	}
	q.Status, q.ActorKind, q.SettledAt = outcome, kind, stamp
	q.Revision++
	q.Completion = &QuestionCompletion{ID: key, Recipient: q.Recipient, Outcome: outcome, Wording: wording}
	out = requestsettlement.Receipt{OperationID: in.OperationID, ID: key, Revision: q.Revision, State: "committed", Outcome: outcome, SettledAt: stamp}
	q.Receipts[index] = out
	r.Questions[key] = q
	receipt := r.Receipts[q.Operation]
	receipt.Disposition = outcome
	receipt.State, receipt.Code, receipt.Reason = "failed", outcome, wording
	if outcome == "cancelled" {
		receipt.State = "cancelled"
	}
	receipt.Result = nil
	if outcome == "answered" {
		receipt.Identity = q.Operation
		receipt.State, receipt.Code, receipt.Reason = "succeeded", "", ""
		var value any = answer
		if q.Question.AnswerType == "" && answer != nil {
			value = answer.OptionIDs[0]
		}
		receipt.Result, _ = json.Marshal(map[string]any{"answer": value})
	}
	if err = w.write(r, receipt); err != nil {
		return requestsettlement.Receipt{}, err
	}
	return out, nil
}

func (w *Window) flushQuestionEvents() { w.mu.Lock(); w.unlock() }

// DismissQuestion is only exposed by the window's person entry point.
func (w *Window) DismissQuestion(in requestsettlement.Input) (requestsettlement.Receipt, error) {
	record, err := w.QuestionRecord(in.ID)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return requestsettlement.Receipt{}, err
		}
		return requestsettlement.Refuse(in, "request_unavailable", "The question is unavailable."), nil
	}
	w.mu.Lock()
	q := w.questions
	w.mu.Unlock()
	if record.Status == "open" && q != nil {
		return q.Dismiss(in)
	}
	return w.commitQuestion(record.Scope, in.ID, "dismissed", "person", in, nil)
}

func (c Client) WithdrawQuestion(ctx context.Context, scope Scope, key, operationID string) (requestsettlement.Receipt, error) {
	in := requestsettlement.Input{ID: key, Revision: 1, OperationID: operationID}
	scope, err := c.scope(scope)
	if err != nil {
		return requestsettlement.Refuse(in, "not_authorized", err.Error()), nil
	}
	return c.window.withdrawQuestion(ctx, scope, key, operationID, requestsettlement.ServiceActor(scope.Service))
}

// WithdrawQuestionByChat is a root-window adapter. A registered service receives
// only Client, and cannot select this actor or obtain this root port.
func (w *Window) WithdrawQuestionByChat(ctx context.Context, scope Scope, key, operationID, canonicalChat string) (requestsettlement.Receipt, error) {
	in := requestsettlement.Input{ID: key, Revision: 1, OperationID: operationID}
	record, err := w.QuestionRecord(key)
	if err != nil {
		return requestsettlement.Receipt{}, err
	}
	if canonicalChat == "" || record.Recipient != canonicalChat || record.Scope != scope || scope.OwnerChat == "" {
		return requestsettlement.Refuse(in, "not_authorized", "This question belongs to another asking chat."), nil
	}
	return w.withdrawQuestion(ctx, scope, key, operationID, requestsettlement.ChatActor(canonicalChat))
}

func (w *Window) withdrawQuestion(ctx context.Context, scope Scope, key, operationID, actor string) (requestsettlement.Receipt, error) {
	in := requestsettlement.Input{ID: key, Revision: 1, OperationID: operationID}
	record, err := w.QuestionRecord(key)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return requestsettlement.Receipt{}, err
		}
		return requestsettlement.Refuse(in, "request_unavailable", "The question is unavailable."), nil
	}
	if scope != record.Scope {
		return requestsettlement.Refuse(in, "not_authorized", "This question belongs to another owner."), nil
	}
	if record.Status != "open" {
		return w.commitQuestion(scope, key, "withdrawn", actor, in, nil)
	}
	if err = w.options.Check(ctx, scope); err != nil {
		return requestsettlement.Refuse(in, "not_authorized", err.Error()), nil
	}
	w.mu.Lock()
	q := w.questions
	w.mu.Unlock()
	if record.Status == "open" && q != nil {
		return q.withdraw(scope, in, actor)
	}
	return w.commitQuestion(scope, key, "withdrawn", actor, in, nil)
}

// QuestionOperation reads only this registered consumer's original withdrawal.
// It needs no fresh execution authority and cannot settle a question.
func (c Client) QuestionOperation(ctx context.Context, scope Scope, key, operation string) (requestsettlement.Receipt, error) {
	in := requestsettlement.Input{ID: key, OperationID: operation, Revision: 1}
	scope, err := c.scope(scope)
	if err != nil || !requestsettlement.ValidOperationID(operation) {
		return requestsettlement.Refuse(in, "not_authorized", "The original withdrawal could not be identified."), nil
	}
	if err := ctx.Err(); err != nil {
		return requestsettlement.Receipt{}, err
	}
	return c.window.questionOperation(scope, key, operation, requestsettlement.ServiceActor(scope.Service))
}

func (w *Window) QuestionOperationByChat(ctx context.Context, scope Scope, key, operation, canonicalChat string) (requestsettlement.Receipt, error) {
	in := requestsettlement.Input{ID: key, Revision: 1, OperationID: operation}
	if err := ctx.Err(); err != nil {
		return requestsettlement.Receipt{}, err
	}
	record, err := w.QuestionRecord(key)
	if err != nil {
		return requestsettlement.Receipt{}, err
	}
	if canonicalChat == "" || record.Recipient != canonicalChat || record.Scope != scope || scope.OwnerChat == "" {
		return requestsettlement.Refuse(in, "not_authorized", "The withdrawal belongs to another asking chat."), nil
	}
	return w.questionOperation(scope, key, operation, requestsettlement.ChatActor(canonicalChat))
}

func (w *Window) questionOperation(scope Scope, key, operation, actor string) (requestsettlement.Receipt, error) {
	in := requestsettlement.Input{ID: key, Revision: 1, OperationID: operation}
	if !requestsettlement.ValidOperationID(operation) {
		return requestsettlement.Refuse(in, "invalid_request", "The original operation ID is invalid."), nil
	}
	record, err := w.QuestionRecord(key)
	if err != nil {
		return requestsettlement.Receipt{}, err
	}
	if record.Scope != scope {
		return requestsettlement.Refuse(in, "not_authorized", "The withdrawal belongs to another task or asking chat."), nil
	}
	if receipt, ok := record.Receipts[requestsettlement.Key(actor, "withdrawn", operation)]; ok {
		return receipt, nil
	}
	return requestsettlement.Receipt{OperationID: operation, ID: key, State: "unknown"}, nil
}
