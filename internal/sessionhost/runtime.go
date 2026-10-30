package sessionhost

import (
	_ "embed"
	"time"

	"studio/internal/bushost"
	"studio/internal/profile"
)

//go:embed runtime.go
var RuntimeSource []byte

// Runtime is the sole registry and wake scheduler. Its zero value is ready for
// use and must never be copied. Every Locked operation requires the host state
// mutex, including compound calls that observe settings and session state.
// Operations acquire conversation mutexes after that lock and never call hosts.
type Runtime struct {
	noCopy        noCopy
	conversations map[string]*Conversation
	providers     map[string]*Provider
	subagentRoots map[string]*Conversation
	runSubagents  map[string]RunSubagent
	asking        map[string]map[string]int
	wake          bushost.Scheduler
	// chatLogs keeps the total of the open chats' event logs. A chat's log
	// joins it when the chat is put here and leaves when it is removed.
	chatLogs *ChatLogs
	// queues keeps the queued messages of the chats put here, on disk.
	queues *QueueStore
}

// SetQueueStore makes the chats put here keep their queues in the store.
func (r *Runtime) SetQueueStore(q *QueueStore) { r.queues = q }

// ChatLogsLocked returns the registry of the open chats' event logs, made
// with the default bounds on first use. A host that wants other bounds or a
// log of trims sets them before the first chat opens (SetChatLogsLocked).
func (r *Runtime) ChatLogsLocked() *ChatLogs {
	if r.chatLogs == nil {
		r.chatLogs = NewChatLogs(DefaultChatLogLimits)
	}
	return r.chatLogs
}

// SetChatLogsLocked replaces the registry before any chat is open.
func (r *Runtime) SetChatLogsLocked(logs *ChatLogs) { r.chatLogs = logs }

type noCopy struct{}

func (*noCopy) Lock()   {}
func (*noCopy) Unlock() {}

func (r *Runtime) ConversationLocked(id string) *Conversation { return r.conversations[id] }
func (r *Runtime) PutConversationLocked(id string, c *Conversation) {
	if r.conversations == nil {
		r.conversations = map[string]*Conversation{}
	}
	if old := r.conversations[id]; old != nil && old != c {
		r.ChatLogsLocked().deregister(old.log)
	}
	r.conversations[id] = c
	r.ChatLogsLocked().register(c.log)
	if q := r.queues; q != nil {
		c.SetQueueSink(func(queue []StoredPrompt) { q.Submit(id, queue) })
	}
}
func (r *Runtime) RemoveConversationLocked(id string) {
	if c := r.conversations[id]; c != nil {
		r.ChatLogsLocked().deregister(c.log)
	}
	delete(r.conversations, id)
}
func (r *Runtime) RemoveExpectedLocked(id string, c *Conversation) bool {
	if r.conversations[id] != c {
		return false
	}
	r.ChatLogsLocked().deregister(c.log)
	delete(r.conversations, id)
	return true
}
func (r *Runtime) ConversationsLocked() []*Conversation {
	out := make([]*Conversation, 0, len(r.conversations))
	for _, c := range r.conversations {
		out = append(out, c)
	}
	return out
}
func (r *Runtime) ConversationCountLocked() int       { return len(r.conversations) }
func (r *Runtime) ProviderLocked(id string) *Provider { return r.providers[id] }
func (r *Runtime) PutProviderLocked(id string, p *Provider) {
	if r.providers == nil {
		r.providers = map[string]*Provider{}
	}
	r.providers[id] = p
}
func (r *Runtime) ProviderIDsLocked() []string {
	var ids []string
	for id := range r.providers {
		ids = append(ids, id)
	}
	return ids
}
func (r *Runtime) DetachProviderLocked(id string, expected *Provider) []*Conversation {
	if r.providers[id] != expected {
		return nil
	}
	delete(r.providers, id)
	var ended []*Conversation
	for key, c := range r.conversations {
		provider := profile.ProviderOrLegacy(c.Provider())
		if provider == id {
			ended = append(ended, c)
			r.ChatLogsLocked().deregister(c.log)
			delete(r.conversations, key)
		}
	}
	return ended
}

type RunSubagent struct{ Started, Ended string }

func (r *Runtime) RunSubagentLocked(id string) (RunSubagent, bool) {
	v, ok := r.runSubagents[id]
	return v, ok
}
func (r *Runtime) NoteSubagentLocked(info SubagentInfo) {
	if !info.ThisRun {
		return
	}
	if r.runSubagents == nil {
		r.runSubagents = map[string]RunSubagent{}
	}
	r.runSubagents[info.ID] = RunSubagent{info.Started, info.Ended}
}
func (r *Runtime) SubagentRootLocked(id string) *Conversation { return r.subagentRoots[id] }
func (r *Runtime) PutSubagentRootLocked(id string, c *Conversation) {
	if r.subagentRoots == nil {
		r.subagentRoots = map[string]*Conversation{}
	}
	r.subagentRoots[id] = c
}
func (r *Runtime) DropSubagentIndexLocked(root string) {
	for id, c := range r.subagentRoots {
		if c.ID() == root {
			delete(r.subagentRoots, id)
		}
	}
}
func (r *Runtime) BackgroundUsedLocked() int {
	n := r.wake.BackgroundPending()
	for _, c := range r.conversations {
		if c.UsesBackgroundSlot() {
			n++
		}
	}
	return n
}

// QueueWarmLocked queues the chats to open at start for their groups, once.
func (r *Runtime) QueueWarmLocked(chats []bushost.Decision) { r.wake.QueueWarm(chats) }
func (r *Runtime) WarmedLocked() bool                       { return r.wake.Warmed() }

// NextWarmLocked returns the next group chat to open at start, if fewer than
// bushost.WarmConcurrency are connecting.
func (r *Runtime) NextWarmLocked(now time.Time) (bushost.Decision, bool) {
	return r.wake.NextWarm(func(chat string) bool {
		c := r.conversations[chat]
		if c == nil {
			return false
		}
		state := c.State()
		return state.Open && !state.Parked
	}, now)
}
func (r *Runtime) ConsiderWakeLocked(chat, path string, newest int64, hidden bool, now time.Time) (bushost.Decision, bool) {
	open := false
	if c := r.conversations[chat]; c != nil {
		state := c.State()
		open = state.Open && !state.Parked
	}
	return r.wake.Consider(chat, path, newest, hidden, open, now)
}
func (r *Runtime) NextWakeLocked(paths map[string]string, now time.Time) (bushost.Decision, bool) {
	live := r.BackgroundUsedLocked() - r.wake.BackgroundPending()
	return r.wake.Next(paths, live, now)
}
func (r *Runtime) CompleteWakeLocked(chat string, err error, now time.Time) (*Conversation, bool) {
	completed := r.wake.Complete(chat, err, now)
	if completed.Asked && err == nil && completed.Background && !completed.Window {
		if c := r.conversations[chat]; c != nil {
			if completed.Warm {
				c.ConnectedWarm(now)
			} else {
				c.ConnectedBackground(now)
			}
			return c, completed.Asked
		}
	}
	return nil, completed.Asked
}
func (r *Runtime) SetWindowsLocked(ids []string)                  { r.wake.SetWindows(ids) }
func (r *Runtime) WindowsLocked() (bool, map[string]bool)         { return r.wake.Windows() }
func (r *Runtime) WindowsKnownLocked() bool                       { return r.wake.Known() }
func (r *Runtime) HasWindowLocked(id string) bool                 { return r.wake.HasWindow(id) }
func (r *Runtime) FailedSinceLocked(id string, t time.Time) error { return r.wake.FailedSince(id, t) }
func (r *Runtime) ExpireWakeLocked(now time.Time)                 { r.wake.Expire(now) }
func (r *Runtime) AskCountLocked() int                            { return len(r.asking) }

func (r *Runtime) BeginAskLocked(from, to string) bool {
	seen := map[string]bool{}
	next := []string{to}
	for len(next) > 0 {
		id := next[len(next)-1]
		next = next[:len(next)-1]
		if id == from {
			return false
		}
		if seen[id] {
			continue
		}
		seen[id] = true
		for t := range r.asking[id] {
			next = append(next, t)
		}
	}
	if r.asking == nil {
		r.asking = map[string]map[string]int{}
	}
	if r.asking[from] == nil {
		r.asking[from] = map[string]int{}
	}
	r.asking[from][to]++
	return true
}
func (r *Runtime) EndAskLocked(from, to string) {
	if r.asking[from][to]--; r.asking[from][to] <= 0 {
		delete(r.asking[from], to)
	}
	if len(r.asking[from]) == 0 {
		delete(r.asking, from)
	}
}
