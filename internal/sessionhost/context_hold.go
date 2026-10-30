package sessionhost

// A send held because the chat's known context window is full, and the course
// the person chose for it (docs/plans/ollama-context-window.md). The hold is
// part of the chat's information, which is numbered, so a window opened late or
// a page that resynced sees it, and the held message waits in the chat's queue,
// as a message sent during a load does.

// ContextHold says a send is held because the chat's window is full.
type ContextHold struct {
	// Kind is "context".
	Kind string `json:"kind"`
	// Used and Size are the tokens Ollama counted for the conversation and the
	// window they are measured against.
	Used int `json:"used"`
	Size int `json:"size"`
	// Actions are the courses the person may choose.
	Actions []string `json:"actions"`
}

// The courses for a full window.
const (
	HoldSetAside = "set-aside"
	HoldCompact  = "compact"
)

// SetHold records that a send is held, or with nil that none is.
func (c *Conversation) SetHold(h *ContextHold) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if h == nil {
		c.hold = nil
		return
	}
	copied := *h
	copied.Actions = append([]string(nil), h.Actions...)
	c.hold = &copied
}

// Hold is the hold, or nil when nothing is held.
func (c *Conversation) Hold() *ContextHold {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.hold == nil {
		return nil
	}
	copied := *c.hold
	copied.Actions = append([]string(nil), c.hold.Actions...)
	return &copied
}

// SetContextChoice records the course the person chose for a full window, or ""
// for none.
func (c *Conversation) SetContextChoice(choice string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.choice = choice
}

// ContextChoice is the course the person chose, or "".
func (c *Conversation) ContextChoice() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.choice
}

// HoldPrompt puts a prompt that was about to start a turn back at the front of
// the chat's queue and frees the chat, so that the prompt waits there, shown as
// waiting, until the hold is resolved.
func (c *Conversation) HoldPrompt(p *Prompt) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.activity.HoldLocked(*p)
}

// TakeQueued removes every prompt in the queue, in order, the held one first. A
// compaction carries them all to the new chat, since a continuation is refused while
// anything is queued. They stay kept on disk until SettleCarry, or until RestoreQueued
// puts them back.
func (c *Conversation) TakeQueued() []Prompt {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.activity.TakeForCarryLocked()
}

// SettleCarry says the prompts TakeQueued took have been queued in the chat they
// were carried to.
func (c *Conversation) SettleCarry() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.activity.SettleCarryLocked()
}

// SetQueueSink says where what is kept of the chat's queue goes each time it changes.
func (c *Conversation) SetQueueSink(sink func([]StoredPrompt)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.activity.sink = sink
}

// QueuedIDs are the ids of the prompts kept for the chat, so that a message found in
// another file is known to be the same.
func (c *Conversation) QueuedIDs() map[string]bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	ids := map[string]bool{}
	for _, p := range c.activity.queued {
		ids[p.qid] = true
	}
	for _, p := range c.activity.carrying {
		ids[p.qid] = true
	}
	return ids
}

// RestoreQueued puts prompts taken by TakeQueued back at the front of the queue.
func (c *Conversation) RestoreQueued(ps []Prompt) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.activity.RestoreQueuedLocked(ps)
}

// TakeHeld removes the prompt a hold put at the front of the queue and returns
// it, or nil when the queue is empty.
func (c *Conversation) TakeHeld() *Prompt {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.activity.TakeFirstQueuedLocked()
}
