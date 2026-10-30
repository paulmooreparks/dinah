package sessionhost

import (
	"context"
	_ "embed"
	"sync"
	"time"

	"studio/internal/bus"
	"studio/internal/profile"
)

//go:embed conversation.go
var ConversationSource []byte

// Identity is copied during construction. ID, Provider and Cwd remain fixed.
type Identity struct{ ID, Provider, Cwd, Title, UpdatedAt string }

// Conversation owns one stable mutex and all live conversation state. It must
// never be copied. History operations require the host transcript mutex.
type Conversation struct {
	mu       sync.Mutex
	identity Identity
	turns    *Turns
	details  *Details
	activity *Activity
	history  History
	// resumed is set once for a chat reopened with session/resume.
	resumed bool
	// hold and choice are a send held for a full context window and the course
	// the person chose for it (context_hold.go).
	hold   *ContextHold
	choice string
	// switchState is the chat's switch, while it runs and after it failed
	// (switch.go).
	switchState *SwitchState
	// lead is the chat's lead, when it is a team member (teams.go).
	lead *LeadRef
	// log is the chat's numbered events (chat_events.go). Its mutex is a
	// leaf, which may be taken under mu.
	log *chatLog
	// clock numbers the snapshots of state the page is given (state_events.go).
	clock *StateClock
}

func NewConversation(identity Identity, initial InitialActivity, replay, restore bool) *Conversation {
	// A title is one short line, whatever made it (bus.CleanTitle).
	identity.Title = bus.CleanTitle(identity.Title)
	c := &Conversation{identity: identity, log: newChatLog(identity.ID)}
	c.clock = NewStateClockGen(c.log.gen)
	c.history.head = func() uint64 { _, n := c.log.position(); return n }
	c.turns = NewTurns(&c.mu)
	c.details = NewDetails(&c.mu)
	c.activity = NewActivity(c.turns, c.details, initial)
	c.details.SetReplayingLocked(replay)
	c.details.SetRestoringLocked(restore)
	return c
}
func (c *Conversation) ID() string       { return c.identity.ID }
func (c *Conversation) Provider() string { return c.identity.Provider }
func (c *Conversation) Cwd() string      { return c.identity.Cwd }
func (c *Conversation) Title() string    { c.mu.Lock(); defer c.mu.Unlock(); return c.identity.Title }
func (c *Conversation) UpdatedAt() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.identity.UpdatedAt
}
func (c *Conversation) UpdateDisplay(title, updated *string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if title != nil {
		c.identity.Title = bus.CleanTitle(*title)
	}
	if updated != nil {
		c.identity.UpdatedAt = *updated
	}
}
func (c *Conversation) WithReply(ctx context.Context, p Prompt) Prompt {
	return p.WithReply(ctx, &c.mu)
}

// Clock is called under the conversation mutex. It must not re-enter the
// conversation or acquire the host state lock, and receives no mutable state.
type Clock func() time.Time

func (c *Conversation) ParkAt(now Clock) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.activity.ParkLocked() {
		return false
	}
	c.activity.MarkClosedLocked(now())
	return true
}
func (c *Conversation) BackgroundAt(now Clock) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.activity.SetBackgroundLocked()
	c.activity.SetIdleSinceLocked(now())
}
func (c *Conversation) AbandonEndAt(now Clock) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.activity.AbandonEndLocked()
	c.activity.MarkClosedLocked(now())
}

// InitializeHistoryLocked requires the host transcript mutex and preserves
// transcript mutex, conversation mutex, then head-cache mutex ordering.
func (c *Conversation) InitializeHistoryLocked(dir string, replay bool) {
	c.mu.Lock()
	links, _ := ConversationLinks(dir)
	c.activity.SetSupersededLocked(links[c.ID()] != "")
	c.mu.Unlock()
	provider := profile.ProviderOrLegacy(c.Provider())
	c.history.InitializeLocked(dir, c.ID(), provider, c.Cwd(), replay)
}

// ResumeHistoryLocked requires the host transcript mutex. It opens the saved
// copy for a chat reopened with session/resume and returns why the copy could
// not be used, or "" when it was (history.go).
func (c *Conversation) ResumeHistoryLocked(dir string) string {
	c.mu.Lock()
	links, _ := ConversationLinks(dir)
	c.activity.SetSupersededLocked(links[c.ID()] != "")
	c.mu.Unlock()
	provider := profile.ProviderOrLegacy(c.Provider())
	return c.history.ResumeLocked(dir, c.ID(), provider, c.Cwd())
}

// SetResumed records that the chat was reopened with session/resume.
func (c *Conversation) SetResumed(v bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.resumed = v
}

// SessionInfo is what the page shows of a session.
type Info struct {
	Provider  string   `json:"provider"`
	ID        string   `json:"sessionId"`
	Cwd       string   `json:"cwd"`
	Title     string   `json:"title"`
	UpdatedAt string   `json:"updatedAt"`
	Open      bool     `json:"open"`
	Working   bool     `json:"working"`
	Queued    []string `json:"queued"`
	// Background is the agent's work still running after its turn ended,
	// oldest first.
	Background []BackgroundTask `json:"background"`
	// Subagents are the chat's subagents and theirs, running or ended, in
	// the order they began; each has its parent's ID, so the page can draw
	// them as a tree (subagents.go).
	Subagents []SubagentInfo `json:"subagents"`
	// InBackground is true for a chat open in the background, without a
	// window, for a message that woke it, and Parked for one that has
	// closed itself since for being idle, which the page shows as closed
	// (bus_wake.go).
	InBackground bool `json:"inBackground"`
	Parked       bool `json:"parked"`
	// Notes are lines for the chat to show once, about the opening that
	// returned them, such as a stored substitution it made
	// (option_substitutions.go). Only opening a chat returns them.
	Notes []string `json:"notes,omitempty"`
	// StreamGen and StreamHead are the chat's event log's generation and
	// highest number when the info was read (chat_events.go), so a page
	// knows the generation of a chat it has had no event from yet.
	StreamGen  string `json:"streamGen"`
	StreamHead uint64 `json:"streamHead"`
	// StateGen and StateVer order the snapshots of this information the page
	// is given, by event or by call (state_events.go). Info leaves them empty.
	StateGen string `json:"stateGen,omitempty"`
	StateVer uint64 `json:"stateVer,omitempty"`
	// Resumed is true when the chat was reopened with session/resume, so no
	// history was replayed and its saved copy is all it has of earlier turns
	// (docs/plans/session-resume.md).
	Resumed bool `json:"resumed,omitempty"`
	// Hold is a send held because the chat's known context window is full, and
	// Choice the course the person chose, remembered for the chat, for such a
	// time (context_hold.go).
	Hold   *ContextHold `json:"hold,omitempty"`
	Choice string       `json:"choice,omitempty"`
	// Switch is the chat's switch to another provider or workspace, while it runs,
	// which stage it is in, and why it failed if it did (switch.go).
	Switch *SwitchState `json:"switch,omitempty"`
	// Lead is the chat that leads this one and the key of the team's group chat, for a team member, or nil
	// (docs/plans/teams.md). The window sets it and sends the information again when it changes.
	Lead *LeadRef `json:"lead,omitempty"`
}

// LeadRef is a chat's lead: its session on the bus, its chat's ID in the window, its title and the team's group key.
type LeadRef struct {
	Bus     string `json:"bus"`
	Chat    string `json:"chat"`
	Title   string `json:"title"`
	Group   string `json:"group"`
	Retired bool   `json:"retired"`
}

// SetLead records the chat's lead, or with nil that it has none, and says whether it changed.
func (c *Conversation) SetLead(l *LeadRef) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if (c.lead == nil) == (l == nil) && (l == nil || *c.lead == *l) {
		return false
	}
	if l == nil {
		c.lead = nil
	} else {
		copied := *l
		c.lead = &copied
	}
	return true
}

func (c *Conversation) Info() Info {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.infoLocked()
}
func (c *Conversation) infoLocked() Info {
	provider := profile.ProviderOrLegacy(c.Provider())
	// The log's mutex is a leaf, so it may be taken under mu.
	gen, head := c.log.position()
	return Info{Provider: provider, ID: c.ID(), Cwd: c.Cwd(), Title: c.identity.Title, UpdatedAt: c.identity.UpdatedAt, Open: c.activity.OpenLocked(), Working: c.activity.WorkingLocked(), Queued: c.activity.QueueTextsLocked(), Background: c.details.BackgroundLocked(), Subagents: c.details.SubagentListLocked(), InBackground: c.activity.BackgroundLocked(), Parked: c.activity.ParkedLocked(), StreamGen: gen, StreamHead: head, Resumed: c.resumed, Hold: c.hold, Choice: c.choice, Switch: c.copySwitchLocked(), Lead: c.copyLeadLocked()}
}
