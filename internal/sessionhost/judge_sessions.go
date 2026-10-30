package sessionhost

import (
	"encoding/json"
	"strings"
	"sync"

	"studio/internal/acp"
)

// A judgement call on a provider reached through the protocol runs in a session of
// its own on the provider's running connection (docs/plans/judgement-agents.md,
// Judges on Every Provider). The connection's handler is shared with the chats, so
// the sessions of calls are registered here, and the handler gives what the agent
// says in one to its call and nowhere else: not to a chat, not to a log, and not to
// the person. Every request the agent makes of the client in such a session is
// refused.

// JudgeSessions are the sessions that judgement calls have open now.
type JudgeSessions struct {
	mu   sync.Mutex
	byID map[string]*JudgeSession
	// ended are the sessions of calls that are over, so that what the agent says
	// late in one is dropped; pending counts the sessions being created, whose
	// first updates can arrive before the call knows the session's id, and held are
	// the updates of unknown sessions kept meanwhile; sink gives a held update to the
	// handler once the call knows which session it is.
	ended   map[string]bool
	pending int
	held    []json.RawMessage
	sink    func(method string, params json.RawMessage)
}

// NewJudgeSessions makes an empty set.
func NewJudgeSessions() *JudgeSessions {
	return &JudgeSessions{byID: map[string]*JudgeSession{}, ended: map[string]bool{}}
}

// Begin says a call is creating a session. An update of a session Dinah does not
// know that changes something shared, such as the commands the agent offers, is
// held until Register or Abandon, because it may be the new session's.
func (r *JudgeSessions) Begin() {
	r.mu.Lock()
	r.pending++
	r.mu.Unlock()
}

// Abandon ends a Begin whose session was not created.
func (r *JudgeSessions) Abandon() { r.settle("") }

// Register adds the session that Begin was for. refused is called, once, when the
// agent asks the client for anything, which a judge never may, so that the call can
// end. Updates held meanwhile are dropped if they were the session's and otherwise
// given to the handler.
func (r *JudgeSessions) Register(id string, refused func()) *JudgeSession {
	s := &JudgeSession{refusedHook: refused}
	r.mu.Lock()
	r.byID[id] = s
	r.mu.Unlock()
	r.settle(id)
	return s
}

func (r *JudgeSessions) settle(id string) {
	r.mu.Lock()
	r.pending--
	var give []json.RawMessage
	if r.pending == 0 {
		give, r.held = r.held, nil
	}
	sink := r.sink
	r.mu.Unlock()
	for _, params := range give {
		var n struct {
			SessionID string `json:"sessionId"`
		}
		if json.Unmarshal(params, &n) == nil && n.SessionID == id && id != "" {
			continue
		}
		if sink != nil {
			sink("session/update", params)
		}
	}
}

// holdUpdate keeps an update of a session Dinah does not know while a call is
// creating one, and says so. A session that was a call's is dropped.
func (r *JudgeSessions) holdUpdate(sessionID string, params json.RawMessage) bool {
	if r == nil {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.ended[sessionID] {
		return true
	}
	if r.pending == 0 {
		return false
	}
	r.held = append(r.held, append(json.RawMessage(nil), params...))
	return true
}

// over says the session was a call's and has ended.
func (r *JudgeSessions) over(id string) bool {
	if r == nil || id == "" {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.ended[id]
}

// Unregister removes a session, after which what the agent says in it is dropped
// as it is for any session Dinah does not know.
func (r *JudgeSessions) Unregister(id string) {
	r.mu.Lock()
	delete(r.byID, id)
	// The set is bounded; a session that old no longer sends anything.
	if len(r.ended) > 1024 {
		r.ended = map[string]bool{}
	}
	r.ended[id] = true
	r.mu.Unlock()
}

func (r *JudgeSessions) lookup(id string) *JudgeSession {
	if r == nil || id == "" {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.byID[id]
}

// JudgeSession is what one call's session has said so far.
type JudgeSession struct {
	mu          sync.Mutex
	text        strings.Builder
	used        int
	usageSeen   bool
	refused     []string
	refusedHook func()
}

// Text is the agent's answer so far, which is its message and not its thinking.
func (s *JudgeSession) Text() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.text.String()
}

// Usage is the context the session reports it uses, from the agent's
// usage_update, and whether it reported any. The protocol defines it as how full the
// session's context is, so for a session that holds one call it is what the call
// used, including the agent's own instructions to itself.
func (s *JudgeSession) Usage() (used int, reported bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.used, s.usageSeen
}

// Refused lists what the agent asked of the client and was refused.
func (s *JudgeSession) Refused() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.refused...)
}

// update reads one session update.
func (s *JudgeSession) update(raw json.RawMessage) {
	var u struct {
		SessionUpdate string `json:"sessionUpdate"`
		Used          *int   `json:"used"`
		Content       struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if json.Unmarshal(raw, &u) != nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	switch u.SessionUpdate {
	case "agent_message_chunk":
		if u.Content.Type == "text" {
			s.text.WriteString(u.Content.Text)
		}
	case "usage_update":
		if u.Used != nil {
			s.used, s.usageSeen = *u.Used, true
		}
	}
}

// refuse notes a request the agent made and answers it. A permission request is
// answered with the agent's own rejection, or as cancelled when it offers none; any
// other request is an error.
func (s *JudgeSession) refuse(method string, params json.RawMessage) (any, *acp.Error) {
	s.mu.Lock()
	first := len(s.refused) == 0
	s.refused = append(s.refused, method)
	hook := s.refusedHook
	s.mu.Unlock()
	if first && hook != nil {
		hook()
	}
	if method != "session/request_permission" {
		return nil, &acp.Error{Code: acp.CodeMethodNotFound, Message: "a judgement call may not use " + method}
	}
	var p acp.RequestPermissionParams
	if json.Unmarshal(params, &p) == nil {
		for _, kind := range []string{"reject_once", "reject_always"} {
			for _, o := range p.Options {
				if o.Kind == kind {
					return acp.Selected(o.OptionID), nil
				}
			}
		}
	}
	return acp.Cancelled(), nil
}
