package sessionhost

import (
	"context"
	_ "embed"
	"encoding/json"
	"maps"
	"sort"
	"sync"

	"studio/internal/acp"
)

//go:embed permissions.go
var PermissionsSource []byte

type PermissionHost struct {
	NextKey        func() string
	Emit           func(string, any)
	NoticeQuestion func(string, json.RawMessage)
}

// Permissions owns pending permission waiters. Host callbacks run outside mu.
type Permissions struct {
	mu        sync.Mutex
	questions map[string]*question
	asked     int
	host      PermissionHost
}

func NewPermissions(host PermissionHost) *Permissions {
	return &Permissions{host: host, questions: make(map[string]*question)}
}
func (p *Permissions) HasPending(session string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, q := range p.questions {
		if q.session == session {
			return true
		}
	}
	return false
}

type question struct {
	session string
	// subagent is the subagent that asks, when one does; session is then
	// its chat.
	subagent string
	ch       chan string
	// ask is what the agent:permission event carries, kept so that a page
	// that missed the event can read it (Pending), and order is where the
	// question came among the open ones.
	ask   map[string]any
	order int
}

// Pending returns, oldest first, what the agent:permission event carried for
// each question still open. A page that missed an event reads them here. The
// maps are copies, so the caller may change them.
func (p *Permissions) Pending() []map[string]any {
	p.mu.Lock()
	defer p.mu.Unlock()
	open := make([]*question, 0, len(p.questions))
	for _, q := range p.questions {
		open = append(open, q)
	}
	sort.Slice(open, func(i, j int) bool { return open[i].order < open[j].order })
	out := make([]map[string]any, 0, len(open))
	for _, q := range open {
		out = append(out, maps.Clone(q.ask))
	}
	return out
}

func (p *Permissions) Answer(key, optionID string) {
	p.mu.Lock()
	q := p.questions[key]
	delete(p.questions, key)
	p.mu.Unlock()
	if q != nil {
		q.ch <- optionID
	}
}

// Ask registers a permission, emits its card, notifies the host and waits.
// A supplied subagent attributes the card and allows targeted cancellation.
func (p *Permissions) Ask(ctx context.Context, session string, sub *SubagentInfo, toolCall json.RawMessage, options []acp.PermissionOption) (string, bool) {
	key := p.host.NextKey()
	q := &question{session: session, ch: make(chan string, 1)}
	if sub != nil {
		q.subagent = sub.ID
	}
	ask := map[string]any{"sessionId": session, "key": key, "toolCall": toolCall, "options": options}
	if sub != nil {
		ask["subagent"] = map[string]string{"id": sub.ID, "name": sub.Name}
	}
	q.ask = ask
	p.mu.Lock()
	p.asked++
	q.order = p.asked
	p.questions[key] = q
	p.mu.Unlock()
	defer func() {
		p.mu.Lock()
		delete(p.questions, key)
		p.mu.Unlock()
		p.host.Emit("agent:answered", map[string]string{"sessionId": session, "key": key})
	}()
	p.host.Emit("agent:permission", ask)
	p.host.NoticeQuestion(session, toolCall)
	select {
	case id, ok := <-q.ch:
		return id, ok
	case <-ctx.Done():
		return "", false
	}
}

// CancelSession answers every open question of a session with cancelled,
// as the protocol requires when a turn is cancelled.
func (p *Permissions) CancelSession(session string) {
	p.mu.Lock()
	var qs []*question
	for k, q := range p.questions {
		if q.session == session {
			qs = append(qs, q)
			delete(p.questions, k)
		}
	}
	p.mu.Unlock()
	for _, q := range qs {
		close(q.ch)
	}
}

func (p *Permissions) CancelSubagents(ids map[string]bool) {
	p.mu.Lock()
	var qs []*question
	for k, q := range p.questions {
		if q.subagent != "" && ids[q.subagent] {
			qs = append(qs, q)
			delete(p.questions, k)
		}
	}
	p.mu.Unlock()
	for _, q := range qs {
		close(q.ch)
	}
}
