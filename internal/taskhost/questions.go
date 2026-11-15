package taskhost

import (
	"context"
	"crypto/rand"
	"slices"
	"sync"
	"time"
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
	Key      string           `json:"key"`
	Question string           `json:"question"`
	Options  []QuestionOption `json:"options"`
	AskedAt  time.Time        `json:"askedAt"`
}

type QuestionEnd struct {
	Key        string `json:"key"`
	Service    string `json:"service"`
	Workspace  string `json:"workspace"`
	Task       string `json:"task"`
	Generation string `json:"generation"`
}

type QuestionPorts struct {
	Describe func(context.Context, Scope) (QuestionOwner, error)
	Check    func(context.Context, Scope) error
	Now      func() time.Time
	Emit     func(string, any)
}

type pendingQuestion struct {
	view   Question
	scope  Scope
	ctx    context.Context
	cancel context.CancelFunc
	answer chan string
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
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if !validScope(scope) || text == "" || len(text) > 65536 || len(options) == 0 || len(options) > 16 {
		return "", refusal("stale", "a task question requires bounded text and answer options")
	}
	seen := map[string]bool{}
	for _, option := range options {
		if option.OptionID == "" || option.Name == "" || len(option.OptionID) > 512 || len(option.Name) > 4096 || seen[option.OptionID] {
			return "", refusal("stale", "question options must have distinct bounded identities and names")
		}
		seen[option.OptionID] = true
	}
	if err := q.ports.Check(ctx, scope); err != nil {
		return "", err
	}
	owner, err := q.ports.Describe(ctx, scope)
	if err != nil {
		return "", err
	}
	if owner.Service != scope.Service || owner.Workspace != scope.Workspace || owner.Task != scope.Task || owner.Generation != scope.Generation || owner.ServiceName == "" || owner.WorkspaceName == "" || owner.TaskTitle == "" || (owner.Target.Href == "") == (owner.Target.Reason == "") {
		return "", refusal("unavailable", "the task question's trusted display metadata is incomplete")
	}
	questionCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	p := &pendingQuestion{view: Question{QuestionOwner: owner, Key: "task-q-" + rand.Text(), Question: text, Options: slices.Clone(options), AskedAt: q.ports.Now().UTC()}, scope: scope, ctx: questionCtx, cancel: cancel, answer: make(chan string, 1)}
	q.mu.Lock()
	if q.closed || questionCtx.Err() != nil {
		q.mu.Unlock()
		return "", refusal("cancelled", "task question admission is closed")
	}
	q.pending[p.view.Key] = p
	q.mu.Unlock()
	defer q.finish(p)
	q.ports.Emit("task:permission", clone(p.view))
	select {
	case answer := <-p.answer:
		return answer, nil
	case <-questionCtx.Done():
		select {
		case answer := <-p.answer:
			return answer, nil
		default:
			return "", refusal("cancelled", "the task question was cancelled or revoked")
		}
	}
}

func (q *Questions) finish(p *pendingQuestion) {
	q.mu.Lock()
	current := q.pending[p.view.Key] == p
	if current {
		delete(q.pending, p.view.Key)
	}
	q.mu.Unlock()
	if current {
		q.ended(p)
	}
}

func (q *Questions) ended(p *pendingQuestion) {
	q.ports.Emit("task:answered", QuestionEnd{Key: p.view.Key, Service: p.scope.Service, Workspace: p.scope.Workspace, Task: p.scope.Task, Generation: p.scope.Generation})
}

// Answer is called only from the window's existing trusted person Answer path.
// Scope and revocation are rechecked after locating the opaque question key.
func (q *Questions) Answer(key, option string) error {
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
	if !slices.ContainsFunc(p.view.Options, func(o QuestionOption) bool { return o.OptionID == option }) {
		return refusal("not_authorized", "the answer is outside the offered choices")
	}
	q.mu.Lock()
	if q.pending[key] != p || p.ctx.Err() != nil {
		q.mu.Unlock()
		return refusal("stale", "the task question ended before the answer")
	}
	delete(q.pending, key)
	p.answer <- option
	q.mu.Unlock()
	q.ended(p)
	return nil
}

func (q *Questions) Pending() []Question {
	if q == nil {
		return []Question{}
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
