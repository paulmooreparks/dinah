package sessionhost

import (
	"context"
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
	// EndOnToolCall ends a call whose agent starts a tool. An agent whose own
	// rules refuse a tool asks the client for nothing, so the request that is
	// refused here never comes, and the tool's start is the only sign that the
	// agent tried to act. It is set before the set is used.
	EndOnToolCall bool
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
	s := &JudgeSession{refusedHook: refused, endOnToolCall: r.EndOnToolCall}
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
	// endOnToolCall is the set's EndOnToolCall, and acted are the tools the
	// agent started while it was set.
	endOnToolCall bool
	acted         []string
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

// Acted lists the tools the agent started in a session whose set ends a call
// for that.
func (s *JudgeSession) Acted() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.acted...)
}

// update reads one session update.
func (s *JudgeSession) update(raw json.RawMessage) {
	var u struct {
		SessionUpdate string `json:"sessionUpdate"`
		Used          *int   `json:"used"`
		Title         string `json:"title"`
		Kind          string `json:"kind"`
		Content       struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if json.Unmarshal(raw, &u) != nil {
		return
	}
	var end func()
	s.mu.Lock()
	switch u.SessionUpdate {
	case "agent_message_chunk":
		if u.Content.Type == "text" {
			s.text.WriteString(u.Content.Text)
		}
	case "usage_update":
		if u.Used != nil {
			s.used, s.usageSeen = *u.Used, true
		}
	case "tool_call":
		if s.endOnToolCall {
			name := u.Title
			if name == "" {
				name = u.Kind
			}
			if name == "" {
				name = "a tool"
			}
			if len(s.acted) == 0 && len(s.refused) == 0 {
				end = s.refusedHook
			}
			s.acted = append(s.acted, name)
		}
	}
	s.mu.Unlock()
	if end != nil {
		end()
	}
}

// refuse notes a request the agent made and answers it. A permission request is
// answered with the agent's own rejection, or as cancelled when it offers none; any
// other request is an error.
func (s *JudgeSession) refuse(method string, params json.RawMessage) (any, *acp.Error) {
	s.mu.Lock()
	first := len(s.refused) == 0 && len(s.acted) == 0
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

// JudgeHandler is the handler of a connection that only judgement calls use, such
// as a provider process started for them. What the agent says in a call's session
// goes to the call, every request it makes of the client is refused, and anything
// else it sends is dropped, since no chat is on the connection.
type JudgeHandler struct {
	Sessions *JudgeSessions
}

// Notify gives a session's update to its call.
func (h *JudgeHandler) Notify(method string, params json.RawMessage) {
	if method != "session/update" {
		return
	}
	var n struct {
		SessionID string          `json:"sessionId"`
		Update    json.RawMessage `json:"update"`
	}
	if json.Unmarshal(params, &n) != nil || len(n.Update) == 0 {
		return
	}
	if js := h.Sessions.lookup(n.SessionID); js != nil {
		js.update(n.Update)
	}
}

// Request refuses what the agent asks of the client: in a call's session as the
// call's own refusal, which ends the call, and outside one as a method the client
// does not have.
func (h *JudgeHandler) Request(_ context.Context, method string, params json.RawMessage) (any, *acp.Error) {
	var base struct {
		SessionID string `json:"sessionId"`
	}
	_ = json.Unmarshal(params, &base)
	if js := h.Sessions.lookup(base.SessionID); js != nil {
		return js.refuse(method, params)
	}
	return nil, &acp.Error{Code: acp.CodeMethodNotFound, Message: "a connection for judgement calls may not use " + method}
}
