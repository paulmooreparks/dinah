package taskhost

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"slices"
	"sync"
	"time"

	"studio/internal/requestsettlement"
)

type QuestionTarget struct {
	Href   string `json:"href,omitempty"`
	Reason string `json:"reason,omitempty"`
}

// QuestionOwner is resolved through trusted registration and task-store ports.
// The caller's task ID supplies neither a display name nor a destination.
type QuestionOwner struct {
	Service       string         `json:"service"`
	ServiceName   string         `json:"serviceName"`
	Workspace     string         `json:"workspace"`
	WorkspaceName string         `json:"workspaceName"`
	Task          string         `json:"task"`
	Generation    string         `json:"generation"`
	TaskTitle     string         `json:"taskTitle"`
	Target        QuestionTarget `json:"target"`
}

type QuestionOption struct {
	OptionID string `json:"optionId"`
	Name     string `json:"name"`
	Kind     string `json:"kind"`
}

type Question struct {
	QuestionOwner
	Key           string           `json:"key"`
	Question      string           `json:"question"`
	Options       []QuestionOption `json:"options"`
	AskedAt       time.Time        `json:"askedAt"`
	AnswerType    string           `json:"answerType,omitempty"`
	MinLength     int              `json:"minLength,omitempty"`
	MaxLength     int              `json:"maxLength,omitempty"`
	MinSelections int              `json:"minSelections,omitempty"`
	MaxSelections int              `json:"maxSelections,omitempty"`
}

type QuestionEnd struct {
	Revision    int    `json:"revision,omitempty"`
	Disposition string `json:"disposition"`
	Key         string `json:"key"`
	Service     string `json:"service"`
	Workspace   string `json:"workspace"`
	Task        string `json:"task"`
	Generation  string `json:"generation"`
}

func (q Question) MarshalJSON() ([]byte, error) {
	type plain Question
	data, err := json.Marshal(plain(q))
	if err != nil || q.AnswerType == "" {
		return data, err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return nil, err
	}
	if q.AnswerType == "text" {
		fields["minLength"], _ = json.Marshal(q.MinLength)
		fields["maxLength"], _ = json.Marshal(q.MaxLength)
	} else {
		fields["minSelections"], _ = json.Marshal(q.MinSelections)
		fields["maxSelections"], _ = json.Marshal(q.MaxSelections)
	}
	return json.Marshal(fields)
}

type QuestionPorts struct {
	Describe func(context.Context, Scope) (QuestionOwner, error)
	Check    func(context.Context, Scope) error
	Now      func() time.Time
	Emit     func(string, any)
}

type pendingQuestion struct {
	revision    int
	disposition string
	execution   questionExecution
	view        Question
	scope       Scope
	ctx         context.Context
	cancel      context.CancelFunc
	answer      chan questionResult
}

type questionResult struct {
	answer QuestionAnswer
	err    error
}

type Questions struct {
	mu      sync.Mutex
	ports   QuestionPorts
	pending map[string]*pendingQuestion
	closed  bool
}

func NewQuestions(ports QuestionPorts) (*Questions, error) {
	if ports.Describe == nil || ports.Check == nil || ports.Emit == nil {
		return nil, refusal("unavailable", "task questions require metadata, authority and event ports")
	}
	if ports.Now == nil {
		ports.Now = time.Now
	}
	return &Questions{ports: ports, pending: map[string]*pendingQuestion{}}, nil
}

func (q *Questions) Ask(ctx context.Context, scope Scope, text string, options []QuestionOption) (string, error) {
	answer, err := q.ask(ctx, scope, text, QuestionSpec{Options: options})
	if err != nil {
		return "", err
	}
	return answer.OptionIDs[0], nil
}

func (q *Questions) AskTyped(ctx context.Context, scope Scope, text string, spec QuestionSpec) (QuestionAnswer, error) {
	if spec.Type != "text" && spec.Type != "multiple" {
		return QuestionAnswer{}, refusal("stale", "a typed question requires text or multiple selection")
	}
	return q.ask(ctx, scope, text, spec)
}

func (q *Questions) ask(ctx context.Context, scope Scope, text string, spec QuestionSpec) (QuestionAnswer, error) {
	if err := ctx.Err(); err != nil {
		return QuestionAnswer{}, err
	}
	if !validScope(scope) || text == "" || len(text) > 65536 {
		return QuestionAnswer{}, refusal("stale", "a task question requires bounded text and answer options")
	}
	if err := spec.validate(); err != nil {
		return QuestionAnswer{}, err
	}
	if err := q.ports.Check(ctx, scope); err != nil {
		return QuestionAnswer{}, err
	}
	owner, err := q.ports.Describe(ctx, scope)
	if err != nil {
		return QuestionAnswer{}, err
	}
	if owner.Service != scope.Service || owner.Workspace != scope.Workspace || owner.Task != scope.Task || owner.Generation != scope.Generation || owner.ServiceName == "" || owner.WorkspaceName == "" || owner.TaskTitle == "" || (owner.Target.Href == "") == (owner.Target.Reason == "") {
		return QuestionAnswer{}, refusal("unavailable", "the task question's trusted display metadata is incomplete")
	}
	questionCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	p := &pendingQuestion{view: Question{QuestionOwner: owner, Key: "task-q-" + rand.Text(), Question: text, Options: slices.Clone(spec.Options), AskedAt: q.ports.Now().UTC(), AnswerType: spec.Type, MinLength: spec.MinLength, MaxLength: spec.MaxLength, MinSelections: spec.MinSelections, MaxSelections: spec.MaxSelections}, scope: scope, ctx: questionCtx, cancel: cancel, answer: make(chan questionResult, 1)}
	if execution, ok := ctx.Value(questionExecutionKey{}).(questionExecution); ok {
		p.execution = execution
		p.revision = 1
		if err := execution.window.admitQuestion(scope, execution.operation, p.view, q); err != nil {
			return QuestionAnswer{}, err
		}
	}
	q.mu.Lock()
	if q.closed || questionCtx.Err() != nil {
		q.mu.Unlock()
		if p.execution.window != nil {
			_, _ = p.execution.window.commitQuestion(scope, p.view.Key, "cancelled", "host", requestsettlement.Input{ID: p.view.Key, Revision: 1, OperationID: "host-cancel:" + p.view.Key}, nil)
			p.execution.window.flushQuestionEvents()
		}
		return QuestionAnswer{}, refusal("cancelled", "task question admission is closed")
	}
	q.pending[p.view.Key] = p
	q.mu.Unlock()
	defer q.finish(p)
	q.ports.Emit("task:permission", clone(p.view))
	select {
	case answer := <-p.answer:
		return answer.answer, answer.err
	case <-questionCtx.Done():
		q.reconcile(p)
		select {
		case answer := <-p.answer:
			return answer.answer, answer.err
		default:
			return QuestionAnswer{}, refusal("cancelled", "the task question was cancelled or revoked")
		}
	}
}

func (q *Questions) finish(p *pendingQuestion) {
	q.reconcile(p)
	q.mu.Lock()
	current := q.pending[p.view.Key] == p
	if current {
		p.disposition = "cancelled"
		if p.execution.window != nil {
			r, err := p.execution.window.commitQuestion(p.scope, p.view.Key, "cancelled", "host", requestsettlement.Input{ID: p.view.Key, Revision: 1, OperationID: "host-cancel:" + p.view.Key}, nil)
			if err != nil || r.State != "committed" {
				q.mu.Unlock()
				q.reconcile(p)
				return
			}
			p.disposition = r.Outcome
			p.revision = r.Revision
		}
		delete(q.pending, p.view.Key)
	}
	q.mu.Unlock()
	if current {
		if p.execution.window != nil {
			p.execution.window.flushQuestionEvents()
		}
		q.ended(p)
	}
}

// reconcile releases a waiter only from its already committed source record.
// It repairs a lost commit response without submitting another operation.
func (q *Questions) reconcile(p *pendingQuestion) {
	if p.execution.window == nil {
		return
	}
	record, result, err := p.execution.window.reconcileQuestion(p.scope, p.view.Key)
	if err != nil || record.Status == "" {
		return
	}
	q.mu.Lock()
	if q.pending[p.view.Key] != p {
		q.mu.Unlock()
		return
	}
	delete(q.pending, p.view.Key)
	p.disposition = record.Status
	p.revision = record.Revision
	p.answer <- result
	q.mu.Unlock()
	q.ended(p)
}

func (q *Questions) ended(p *pendingQuestion) {
	q.ports.Emit("task:answered", QuestionEnd{Revision: p.revision, Disposition: p.disposition, Key: p.view.Key, Service: p.scope.Service, Workspace: p.scope.Workspace, Task: p.scope.Task, Generation: p.scope.Generation})
}

// Answer is called only from the window's existing trusted person Answer path.
// Scope and revocation are rechecked after locating the opaque question key.
func (q *Questions) Answer(key, option string) error {
	return q.settle(key, QuestionAnswer{OptionIDs: []string{option}}, false)
}

// AnswerTyped is reached only through the trusted window answer entry point.
func (q *Questions) AnswerTyped(key string, answer QuestionAnswer) error {
	return q.settle(key, answer, true)
}

func (q *Questions) settle(key string, answer QuestionAnswer, typed bool) error {
	q.mu.Lock()
	p := q.pending[key]
	q.mu.Unlock()
	if p == nil {
		return refusal("stale", "the task question is no longer pending")
	}
	if err := p.ctx.Err(); err != nil {
		q.finish(p)
		return refusal("cancelled", "the task question was revoked")
	}
	if err := q.ports.Check(p.ctx, p.scope); err != nil {
		p.cancel()
		q.finish(p)
		return err
	}
	if typed != (p.view.AnswerType != "") {
		return refusal("not_authorized", "this question requires its declared answer entry point")
	}
	spec := QuestionSpec{Type: p.view.AnswerType, Options: p.view.Options, MinLength: p.view.MinLength, MaxLength: p.view.MaxLength, MinSelections: p.view.MinSelections, MaxSelections: p.view.MaxSelections}
	validated, err := spec.validateAnswer(answer)
	if err != nil {
		return err
	}
	q.mu.Lock()
	if q.pending[key] != p || p.ctx.Err() != nil {
		q.mu.Unlock()
		return refusal("stale", "the task question ended before the answer")
	}
	delete(q.pending, key)
	if p.execution.window != nil {
		r, err := p.execution.window.commitQuestion(p.scope, key, "answered", "person", requestsettlement.Input{ID: key, Revision: 1, OperationID: "person-answer:" + key}, &validated)
		if err != nil || r.State != "committed" {
			q.pending[key] = p
			q.mu.Unlock()
			if err != nil {
				return err
			}
			return refusal(r.Code, r.Text)
		}
		p.revision = r.Revision
	}
	p.disposition = "answered"
	p.answer <- questionResult{answer: validated}
	q.mu.Unlock()
	if p.execution.window != nil {
		p.execution.window.flushQuestionEvents()
	}
	q.ended(p)
	return nil
}

// Dismiss is available only to the trusted person window.
func (q *Questions) Dismiss(in requestsettlement.Input) (requestsettlement.Receipt, error) {
	return q.endWithoutAnswer(in, "dismissed", "person", nil)
}

// withdraw is reached through a registered Client whose service is fixed.
func (q *Questions) withdraw(scope Scope, in requestsettlement.Input, actor string) (requestsettlement.Receipt, error) {
	return q.endWithoutAnswer(in, "withdrawn", actor, &scope)
}

func (q *Questions) endWithoutAnswer(in requestsettlement.Input, outcome, actor string, scope *Scope) (requestsettlement.Receipt, error) {
	if q == nil {
		return requestsettlement.Refuse(in, "request_closed", "The question is no longer pending."), nil
	}
	q.mu.Lock()
	p := q.pending[in.ID]
	if p == nil {
		q.mu.Unlock()
		return requestsettlement.Refuse(in, "request_closed", "The question is no longer pending."), nil
	}
	if scope != nil && *scope != p.scope {
		q.mu.Unlock()
		return requestsettlement.Refuse(in, "not_authorized", "This question belongs to another task or asking chat."), nil
	}
	if p.execution.window == nil {
		q.mu.Unlock()
		return requestsettlement.Refuse(in, "unavailable", "This question has no durable operation binding."), nil
	}
	if p.ctx.Err() != nil || q.closed {
		q.mu.Unlock()
		return requestsettlement.Refuse(in, "cancelled", "The request authority was revoked."), nil
	}
	r, err := p.execution.window.commitQuestion(p.scope, in.ID, outcome, actor, in, nil)
	if err != nil || r.State != "committed" {
		q.mu.Unlock()
		q.reconcile(p)
		return r, err
	}
	delete(q.pending, in.ID)
	p.disposition = outcome
	p.revision = r.Revision
	p.answer <- questionResult{err: refusal(outcome, "The request was "+outcome+" without an answer.")}
	q.mu.Unlock()
	p.execution.window.flushQuestionEvents()
	q.ended(p)
	return r, nil
}

func (q *Questions) Pending() []Question {
	if q == nil {
		return []Question{}
	}
	q.mu.Lock()
	pending := make([]*pendingQuestion, 0, len(q.pending))
	for _, p := range q.pending {
		pending = append(pending, p)
	}
	q.mu.Unlock()
	// Consumer generations can change independently of an execution handle.
	// Refreshing the pending view withdraws authority that no longer exists.
	for _, p := range pending {
		q.reconcile(p)
		if p.ctx.Err() != nil || q.ports.Check(p.ctx, p.scope) != nil {
			p.cancel()
			q.finish(p)
		}
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	result := []Question{}
	for _, p := range q.pending {
		if p.ctx.Err() == nil {
			result = append(result, clone(p.view))
		}
	}
	slices.SortFunc(result, func(a, b Question) int { return a.AskedAt.Compare(b.AskedAt) })
	return result
}

func (q *Questions) Close() {
	if q == nil {
		return
	}
	q.mu.Lock()
	q.closed = true
	pending := make([]*pendingQuestion, 0, len(q.pending))
	for _, p := range q.pending {
		pending = append(pending, p)
		p.cancel()
	}
	q.mu.Unlock()
	for _, p := range pending {
		q.finish(p)
	}
}
