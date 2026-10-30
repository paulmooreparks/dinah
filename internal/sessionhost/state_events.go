package sessionhost

import (
	"crypto/rand"
	_ "embed"
	"sync/atomic"
)

//go:embed state_events.go
var StateEventsSource []byte

// The events that carry the whole of some state: a session's information, its
// queued prompts and its options, a provider's state, the window's accounts and
// settings, and the update state. They are emitted only through the functions
// in this file and its siblings in the window, which take a generation and a
// version from the state's owner in the same critical section that copies the
// value out, and put them in the event as stateGen and stateVer. The page keeps
// the highest version it has applied for each owner and ignores an event with a
// lower one, so events that reach it out of order, because they are emitted
// after the owner's lock is released, cannot leave it on older state. A
// different generation always wins: it is a new owner, such as a chat opened
// again or an engine that started again (docs/plans/state-versions.md).
const (
	EventSession = "agent:session"
	EventQueue   = "agent:queue"
	EventOptions = "agent:options"
	EventState   = "agent:state"
	// EventChatState carries every chat's state: its life, whether it has a window and what it is doing, and the
	// counts of each group (chat_state.go in the window).
	EventChatState = "agent:chatstate"
)

// StateClock is an owner's counter. Next must be called while the owner's lock
// is held, in the acquisition that copies the value out, so that the number
// orders the snapshots the way the lock orders them. The atomic is for callers
// that copy the clock's position without that lock.
type StateClock struct {
	gen string
	n   atomic.Uint64
}

// NewStateClock makes a counter with a generation of its own.
func NewStateClock() *StateClock { return &StateClock{gen: rand.Text()} }

// NewStateClockGen makes a counter with a given generation, for an owner that
// already has one.
func NewStateClockGen(gen string) *StateClock { return &StateClock{gen: gen} }

// Next returns the generation and the next version.
func (c *StateClock) Next() (string, uint64) { return c.gen, c.n.Add(1) }

// Stamp adds the generation and version to an event's data.
func Stamp(data map[string]any, gen string, ver uint64) map[string]any {
	data["stateGen"], data["stateVer"] = gen, ver
	return data
}

// InfoVersioned is Info with the generation and version taken in the same
// acquisition of the conversation's mutex that reads it. It is what the page
// is given, by event or by call; Info stays unnumbered for the code that
// compares or logs information.
func (c *Conversation) InfoVersioned() Info {
	c.mu.Lock()
	defer c.mu.Unlock()
	info := c.infoLocked()
	info.StateGen, info.StateVer = c.clock.Next()
	return info
}

// EmitSession emits a conversation's current information as agent:session.
func EmitSession(emit func(string, any), s *Conversation) {
	emit(EventSession, s.InfoVersioned())
}

// EmitProviderState emits a provider's state, which its lifecycle has just
// stored and numbered under its lock, as agent:state.
func EmitProviderState(emit func(string, any), s ProviderState) {
	emit(EventState, map[string]any{"provider": s.Provider, "state": s.State, "detail": s.Detail, "stateGen": s.StateGen, "stateVer": s.StateVer})
}

// EmitInfo emits information that InfoVersioned has just read, when the caller
// also keeps it, as agent:session.
func EmitInfo(emit func(string, any), info Info) { emit(EventSession, info) }

// EmitChatStates emits every chat's state, numbered under the window's clock, as agent:chatstate.
func EmitChatStates(emit func(string, any), view any) { emit(EventChatState, view) }

// EmitQueue emits a conversation's queued prompts, as they are now, as
// agent:queue.
func EmitQueue(emit func(string, any), s *Conversation) {
	s.mu.Lock()
	queued := s.activity.QueueTextsLocked()
	gen, ver := s.clock.Next()
	s.mu.Unlock()
	emit(EventQueue, Stamp(map[string]any{"sessionId": s.ID(), "queued": queued}, gen, ver))
}

// EmitQueueCleared emits an empty queue for a conversation that is gone or has
// just had its queue taken, when the conversation itself is no longer to be
// read. It carries no version, so it is applied as it always was.
func EmitQueueCleared(emit func(string, any), id string) {
	emit(EventQueue, map[string]any{"sessionId": id, "queued": []string{}})
}

// EmitOptions emits a conversation's options, as they are now, as
// agent:options, with the error from saving them if there was one.
func EmitOptions(emit func(string, any), s *Conversation, saveErr error) {
	s.mu.Lock()
	options := s.details.OptionsLocked()
	gen, ver := s.clock.Next()
	s.mu.Unlock()
	event := Stamp(map[string]any{"sessionId": s.ID(), "options": options}, gen, ver)
	if saveErr != nil {
		event["error"] = "The chat's provider settings could not be saved: " + saveErr.Error()
	}
	emit(EventOptions, event)
}
