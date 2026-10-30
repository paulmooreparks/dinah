package sessionhost

import (
	"context"
	_ "embed"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"studio/internal/acp"
	"studio/internal/agents"
	"studio/internal/claudeagent"
)

//go:embed handler.go
var HandlerSource []byte

// Handler receives provider messages through named host effects. It holds no
// registry or conversation locks while invoking host callbacks. Transcript
// callbacks retain the host transcript mutex as their serialization authority.
type Handler struct{ host HandlerHost }

func NewHandler(host HandlerHost) *Handler {
	h := &Handler{host: host}
	if host.Judges != nil {
		host.Judges.mu.Lock()
		host.Judges.sink = h.Notify
		host.Judges.mu.Unlock()
	}
	return h
}

var _ acp.Handler = (*Handler)(nil)

type HandlerHost struct {
	ClaudeAccountReport     func(json.RawMessage)
	RedactContexts          func(string) string
	VisibleConversationText func(string) string
	LookupConversation      func(string) *Conversation
	SubagentUpdate          func(string, json.RawMessage) bool
	ReplayBoundary          func(*Conversation, string)
	// RecordConversationUpdate records an update in the saved history and, when
	// the update is to be logged, appends it to the chat's log in the same
	// critical section; the event it returns is emitted afterwards.
	RecordConversationUpdate func(*Conversation, json.RawMessage, bool) *LoggedEvent
	LogUpdate                func(string, json.RawMessage)
	Journal                  func(string, string)
	SetOptions               func(*Conversation, []acp.ConfigOption) error
	SetCommands              func([]Command)
	ResolveTitle             func(string, string) string
	PublishBusTitle          func(*Conversation)
	FlushReplay              func(*Conversation)
	Emit                     func(string, any)
	ResolveRequestChat       func(string) (*Conversation, *SubagentInfo)
	AskPermission            func(context.Context, string, json.RawMessage, []acp.PermissionOption) (string, bool)
	AskAsSubagent            func(context.Context, *Conversation, *SubagentInfo, json.RawMessage, []acp.PermissionOption) (string, bool)
	WriteTextFile            func(string, string) error
	DisplayPath              func(string, string) string
	ProductName              string
	ReplayBatch              int
	// Judges are the sessions that judgement calls have open on the connection.
	// What the agent says in one goes to its call alone, and its requests are
	// refused (judge_sessions.go). Nil when no call is made through the protocol.
	Judges *JudgeSessions
	// TurnCost is told what a request of a chat used, in its provider's unit, from the turn_cost
	// updates some adapters send; they are not forwarded to the page.
	TurnCost func(chat string, tokens int64)
}

// Notify receives the agent's notifications. A session/update carries the
// session's id and the update itself.
func (h *Handler) Notify(method string, params json.RawMessage) {
	a := h.host
	if method == claudeagent.AuthStatusFromClaude {
		a.ClaudeAccountReport(params)
		return
	}
	if method == "codex/account/login/completed" {
		var result struct {
			Success bool   `json:"success"`
			Error   string `json:"error"`
		}
		if json.Unmarshal(params, &result) != nil {
			return
		}
		state := "signed-in"
		if !result.Success {
			state = "failed"
		}
		provider, _, _ := strings.Cut(method, "/")
		a.Emit("agent:auth", map[string]any{"provider": provider, "state": state, "error": result.Error})
		return
	}
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
	if js := a.Judges.lookup(n.SessionID); js != nil {
		js.update(n.Update)
		return
	}
	if a.Judges.over(n.SessionID) {
		return
	}
	n.Update = json.RawMessage(a.RedactContexts(string(n.Update)))
	var u struct {
		SessionUpdate string             `json:"sessionUpdate"`
		ConfigOptions []acp.ConfigOption `json:"configOptions"`
		Title         *string            `json:"title"`
		UpdatedAt     *string            `json:"updatedAt"`
	}
	_ = json.Unmarshal(n.Update, &u)
	if u.SessionUpdate == "turn_cost" {
		var cost struct {
			Tokens int64 `json:"tokens"`
		}
		if json.Unmarshal(n.Update, &cost) == nil && cost.Tokens > 0 && a.TurnCost != nil {
			a.TurnCost(n.SessionID, cost.Tokens)
		}
		return
	}
	if u.SessionUpdate == "usage_update" {
		n.Update = withWindowKnown(n.Update)
	}
	if u.SessionUpdate == "user_message_chunk" {
		var chunk struct {
			Content struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		}
		if json.Unmarshal(n.Update, &chunk) == nil && chunk.Content.Type == "text" {
			if clean := a.VisibleConversationText(chunk.Content.Text); clean != chunk.Content.Text {
				chunk.Content.Text = strings.TrimSpace(clean)
				if chunk.Content.Text == "" {
					return
				}
				var fields map[string]json.RawMessage
				_ = json.Unmarshal(n.Update, &fields)
				fields["content"], _ = json.Marshal(chunk.Content)
				n.Update, _ = json.Marshal(fields)
			}
		}
	}
	// A subagent's updates are its own and not the chat's conversation; its
	// announcement and its end are recorded, and go on below as the chat's
	// when they arrive on the chat (subagents.go).
	if a.SubagentUpdate(n.SessionID, n.Update) {
		return
	}
	s := a.LookupConversation(n.SessionID)
	// Background work is the chat's state, not part of its conversation: the
	// page learns of it through the chat's info (background_tasks.go).
	if s != nil && s.BackgroundUpdate(n.Update) {
		EmitSession(a.Emit, s)
		return
	}
	// A replayed user message is held until it ends, to be recognised as a
	// delivery the saved copy holds (delivered.go).
	if s != nil {
		a.ReplayBoundary(s, u.SessionUpdate)
	}
	// The chat's event log takes what happens now, appended where the update
	// is recorded in the saved history so that the two agree to anyone who
	// reads them; a replayed history is already in the log from when it
	// happened. The updates that only change the chat's state are not part of
	// its transcript and are not logged.
	replaying := s != nil && s.Replaying()
	logIt := s != nil && !replaying
	switch u.SessionUpdate {
	case "config_option_update", "available_commands_update", "session_info_update":
		logIt = false
	}
	logged := a.RecordConversationUpdate(s, n.Update, logIt)
	if s != nil && !replaying {
		a.LogUpdate(s.ID(), n.Update)
	}
	// A message of the agent begins where an agent chunk follows anything but
	// another one, as the page draws it; its start goes into the session's
	// journal, except while the agent replays history.
	if s != nil {
		switch u.SessionUpdate {
		case "agent_message_chunk", "agent_thought_chunk", "user_message_chunk", "tool_call":
			starts := s.UpdateKind(u.SessionUpdate)
			if starts {
				a.Journal(s.ID(), "agent")
			}
		}
	}
	switch u.SessionUpdate {
	case "tool_call":
		h.toolCall(s)
	case "agent_message_chunk":
		h.agentMessage(s, n.Update)
	case "config_option_update":
		h.configOptions(s, u.ConfigOptions)
		return
	case "available_commands_update":
		if s == nil && a.Judges.holdUpdate(n.SessionID, params) {
			return
		}
		h.availableCommands(n.SessionID, n.Update)
		return
	case "session_info_update":
		h.sessionInfo(s, u.Title, u.UpdatedAt)
		return
	}
	if logged != nil {
		logged.Emit(a.Emit)
		return
	}
	if s != nil && u.SessionUpdate == "user_message_chunk" && s.HoldReplay(n.Update) {
		return
	}
	if s != nil {
		if replay, full := s.QueueIfReplaying(n.Update, a.ReplayBatch); replay {
			if full {
				a.FlushReplay(s)
			}
			return
		}
	}
	EmitChat(a.Emit, s, EventUpdate, map[string]any{"sessionId": n.SessionID, "update": n.Update})
}

// Request answers the agent's requests of the client.
func (h *Handler) Request(ctx context.Context, method string, params json.RawMessage) (any, *acp.Error) {
	a := h.host
	var base struct {
		SessionID string `json:"sessionId"`
	}
	_ = json.Unmarshal(params, &base)
	if js := a.Judges.lookup(base.SessionID); js != nil {
		return js.refuse(method, params)
	}
	switch method {
	case "session/request_permission":
		if ctx.Err() != nil {
			return acp.Cancelled(), nil
		}
		var p acp.RequestPermissionParams
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, &acp.Error{Code: acp.CodeInternal, Message: err.Error()}
		}
		p.ToolCall = json.RawMessage(a.RedactContexts(string(p.ToolCall)))
		// A subagent's question is put in its chat, saying which subagent
		// asks (subagents.go).
		var id string
		var ok bool
		if chat, sub := a.ResolveRequestChat(p.SessionID); sub != nil {
			id, ok = a.AskAsSubagent(ctx, chat, sub, p.ToolCall, p.Options)
		} else {
			id, ok = a.AskPermission(ctx, p.SessionID, p.ToolCall, p.Options)
		}
		if !ok {
			return acp.Cancelled(), nil
		}
		return acp.Selected(id), nil
	case "fs/read_text_file":
		var p acp.ReadTextFileParams
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, &acp.Error{Code: acp.CodeInternal, Message: err.Error()}
		}
		b, err := os.ReadFile(p.Path)
		if err != nil {
			return nil, &acp.Error{Code: acp.CodeInternal, Message: err.Error()}
		}
		return acp.ReadTextFileResult{Content: agents.SliceLines(string(b), p.Line, p.Limit)}, nil
	case "fs/write_text_file":
		var p acp.WriteTextFileParams
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, &acp.Error{Code: acp.CodeInternal, Message: err.Error()}
		}
		// Where the agent may write is decided by its approvals: Devin asks
		// the person before writing outside the workspace, and the person's
		// answer is the decision, so Dinah does not refuse a write
		// the person allowed. It refuses only the folders that hold the
		// approval rules themselves, Dinah's settings and Devin's, so
		// that the agent can never approve itself; the person's own flows,
		// which live among Dinah's settings, may be written.
		// A subagent writes in its chat's workspace, and the chat shows the
		// write (subagents.go).
		ws, chatID := "", p.SessionID
		chat, sub := a.ResolveRequestChat(p.SessionID)
		if chat != nil {
			ws, chatID = chat.Cwd(), chat.ID()
		}
		target := filepath.Clean(p.Path)
		if err := a.WriteTextFile(target, p.Content); err != nil {
			return nil, &acp.Error{Code: acp.CodeInternal, Message: err.Error()}
		}
		wrote := map[string]any{"sessionId": chatID, "path": a.DisplayPath(ws, p.Path), "bytes": len(p.Content)}
		if sub != nil {
			wrote["subagent"] = map[string]string{"id": sub.ID, "name": sub.Name}
		}
		a.Emit("agent:file", wrote)
		return struct{}{}, nil
	}
	return nil, &acp.Error{Code: acp.CodeMethodNotFound, Message: a.ProductName + " does not offer " + method}
}
