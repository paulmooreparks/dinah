package sessionhost

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"sort"
	"sync"
	"time"

	"studio/internal/acp"
)

//go:embed subagent_service.go
var SubagentServiceSource []byte

// SubagentService routes child-session activity through the shared runtime.
// Runtime access uses the shell state mutex; host effects run outside it.
type SubagentService struct {
	mu      *sync.Mutex
	runtime *Runtime
	host    SubagentHost
}
type SubagentHost struct {
	Emit            func(string, any)
	Connection      func(*Conversation) Connection
	CancelQuestions func(map[string]bool)
	AskPermission   func(context.Context, string, *SubagentInfo, json.RawMessage, []acp.PermissionOption) (string, bool)
	ProductName     string
}

func NewSubagentService(mu *sync.Mutex, runtime *Runtime, host SubagentHost) *SubagentService {
	return &SubagentService{mu: mu, runtime: runtime, host: host}
}

func (h *SubagentService) Owner(id string) (*Conversation, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if s := h.runtime.ConversationLocked(id); s != nil {
		return s, false
	}
	if s := h.runtime.SubagentRootLocked(id); s != nil {
		return s, true
	}
	return nil, false
}

func (h *SubagentService) Find(id string) (*Conversation, SubagentInfo, bool) {
	h.mu.Lock()
	s := h.runtime.SubagentRootLocked(id)
	h.mu.Unlock()
	if s == nil {
		return nil, SubagentInfo{}, false
	}

	info, found := s.Subagent(id)
	if !found {
		return nil, SubagentInfo{}, false
	}
	return s, info, true
}

func (h *SubagentService) Update(sessionID string, update json.RawMessage) bool {
	owner, child := h.Owner(sessionID)
	if owner == nil {
		return false
	}
	var u struct {
		SessionUpdate string `json:"sessionUpdate"`
		Child         string `json:"subagentSessionId"`
		Name          string `json:"name"`
		Task          string `json:"task"`
		Prompt        string `json:"prompt"`
		Capabilities  struct {
			Cancel bool `json:"cancel"`
		} `json:"capabilities"`
		State string `json:"state"`
	}
	_ = json.Unmarshal(update, &u)
	if u.Child != "" && u.Child != sessionID && u.Child != owner.ID() {
		switch u.SessionUpdate {
		case "subagent_spawned":
			h.Spawn(owner, sessionID, u.Child, u.Name, u.Task, u.Prompt, u.Capabilities.Cancel)
		case "subagent_state_update":
			h.End(owner, u.Child, u.State)
		}
	}
	if !child {
		return false
	}
	h.KeepUpdate(owner, sessionID, u.SessionUpdate, update)
	return true
}

func (h *SubagentService) RunTimes(id string) (RunSubagent, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.runtime.RunSubagentLocked(id)
}

func (h *SubagentService) NoteRun(info SubagentInfo) {
	if !info.ThisRun {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.runtime.NoteSubagentLocked(info)
}

func (h *SubagentService) Spawn(root *Conversation, parent, id, name, task, prompt string, canCancel bool) {
	prior, seen := h.RunTimes(id)

	info := root.SpawnSubagent(root.ID(), parent, id, name, task, prompt, canCancel, seen, prior.Started, prior.Ended)

	h.NoteRun(info)
	h.mu.Lock()
	h.runtime.PutSubagentRootLocked(id, root)
	h.mu.Unlock()
	h.host.Emit("agent:subagent", map[string]any{"sessionId": root.ID(), "subagent": info})
}

func (h *SubagentService) End(root *Conversation, id, state string) {
	if !terminalSubagentState(state) {
		return
	}
	info, ids, changed := root.EndSubagent(id, state)
	if !changed {
		return
	}
	h.NoteRun(info)
	h.CancelQuestions(ids)
	h.host.Emit("agent:subagent", map[string]any{"sessionId": root.ID(), "subagent": info})
}

func (h *SubagentService) Disconnect(root *Conversation) {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	changed := root.DisconnectSubagents(now)
	sort.Slice(changed, func(i, j int) bool { return changed[i].Started < changed[j].Started })
	for _, info := range changed {
		h.NoteRun(info)
		h.host.Emit("agent:subagent", map[string]any{"sessionId": root.ID(), "subagent": info})
	}
}

func (h *SubagentService) KeepUpdate(root *Conversation, id, kind string, update json.RawMessage) {
	switch kind {
	case "available_commands_update", "config_option_update":
		// A subagent takes no commands or settings from the person.
		return
	}

	seq, found, replaying := root.KeepSubagentEvent(id, kind, update)

	if !found {
		return
	}
	if !replaying {
		h.host.Emit("agent:subagent-update", map[string]any{"rootId": root.ID(), "subagentId": id, "seq": seq, "update": update})
	}
}

func (h *SubagentService) DropIndex(rootID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.runtime.DropSubagentIndexLocked(rootID)
}

func (h *SubagentService) RequestChat(id string) (*Conversation, *SubagentInfo) {
	s, child := h.Owner(id)
	if s == nil || !child {
		return s, nil
	}

	info, found := s.Subagent(id)
	if !found {
		return s, nil
	}
	return s, &info
}

func (h *SubagentService) Updates(id string) (SubagentRecord, error) {
	h.mu.Lock()
	s := h.runtime.SubagentRootLocked(id)
	h.mu.Unlock()
	if s == nil {
		return SubagentRecord{}, errors.New(h.host.ProductName + " does not know that subagent; its chat may need to be opened again")
	}

	rec, found := s.SubagentRecord(id)
	if !found {
		return SubagentRecord{}, errors.New(h.host.ProductName + " does not know that subagent; its chat may need to be opened again")
	}
	return rec, nil
}

func (h *SubagentService) Cancel(id string) error {
	root, info, ok := h.Find(id)
	if !ok {
		return errors.New(h.host.ProductName + " does not know that subagent")
	}
	if info.State != subagentRunning {
		return errors.New("that subagent has already finished")
	}
	if !info.CanCancel {
		return errors.New("the agent does not let that subagent be stopped on its own")
	}
	conn := h.host.Connection(root)
	if conn == nil {
		return errors.New("this chat is not connected to its agent")
	}

	ids := root.SubagentFamily(id)

	h.CancelQuestions(ids)
	return conn.Notify("session/cancel", acp.CancelParams{SessionID: id})
}

func (h *SubagentService) CancelQuestions(ids map[string]bool) { h.host.CancelQuestions(ids) }

func (h *SubagentService) Ask(ctx context.Context, chat *Conversation, sub *SubagentInfo, toolCall json.RawMessage, options []acp.PermissionOption) (string, bool) {
	if terminalSubagentState(sub.State) && !sub.Local {
		return "", false
	}
	return h.host.AskPermission(ctx, chat.ID(), sub, toolCall, options)
}
