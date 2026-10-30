package main

import (
	"context"
	"encoding/json"
	"sort"
	"sync"
	"time"

	"studio/internal/bus"
	"studio/internal/sessionhost"
)

// The chat states of every surface are sent as one numbered event, agent:chatstate, carrying the whole of them: the
// state of each open chat and of each member of a group or team, and the counts of each group. The page keeps the
// highest number it has applied, as for the other whole-state events (docs/plans/state-versions.md), and ChatStates
// returns the same value for a page that loads or has lost an event. A team's counts travel in its strip's event
// (team:state), and a chat's own state and its team's counts in its information (agent:session).

// ChatStateEntry is one chat in the event: its ID in the window (empty for a session outside it), its session on the
// bus (empty when it has none), and its state.
type ChatStateEntry struct {
	SessionID string `json:"sessionId"`
	Bus       string `json:"bus"`
	ChatState
}

// GroupStateEntry is a group's counts, and the state it shows as a whole.
type GroupStateEntry struct {
	Name   string      `json:"name"`
	Counts StateCounts `json:"counts"`
}

// ChatStatesView is every chat's state, numbered.
type ChatStatesView struct {
	Chats    []ChatStateEntry  `json:"chats"`
	Groups   []GroupStateEntry `json:"groups"`
	StateGen string            `json:"stateGen"`
	StateVer uint64            `json:"stateVer"`
}

// chatStateSender sends the event a moment after something a chat's state depends on changes, and only when the
// value differs from the one last sent.
type chatStateSender struct {
	mu     sync.Mutex // guards the fields below
	timer  *time.Timer
	closed bool
	sent   string
	clock  *sessionhost.StateClock
	// run is held for a whole pass, so that closing waits for the one in progress.
	run sync.Mutex
}

const chatStateDelay = 400 * time.Millisecond

// chatStatesChanged says that something a chat's state depends on may have changed.
func (a *App) chatStatesChanged() {
	s := &a.chatStates
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.timer != nil || s.closed {
		return
	}
	s.timer = time.AfterFunc(chatStateDelay, a.sendChatStates)
}

// closeChatStates stops the event being sent, as the app ends: a pass in progress finishes, and none starts.
func (a *App) closeChatStates() {
	s := &a.chatStates
	s.mu.Lock()
	s.closed = true
	if s.timer != nil {
		s.timer.Stop()
		s.timer = nil
	}
	s.mu.Unlock()
	s.run.Lock()
	s.run.Unlock()
}

func (a *App) chatStatesOpen() bool {
	s := &a.chatStates
	s.mu.Lock()
	defer s.mu.Unlock()
	return !s.closed && !folderClosed()
}

// computeChatStates works out every chat's state, and records on each open chat its own state and its team's counts,
// so that its information carries them. It returns the value without a number, and the chats whose information
// changed.
func (a *App) computeChatStates() (ChatStatesView, []*Session) {
	ctx := context.Background()
	view := ChatStatesView{Chats: []ChatStateEntry{}, Groups: []GroupStateEntry{}}
	byChat := map[string]int{}
	add := func(e ChatStateEntry) {
		key := e.SessionID
		if key == "" {
			key = "bus:" + e.Bus
		}
		if _, ok := byChat[key]; ok {
			return
		}
		byChat[key] = len(view.Chats)
		view.Chats = append(view.Chats, e)
	}
	compute := func(b *bus.Bus) {
		a.mu.Lock()
		open := a.runtime.ConversationsLocked()
		a.mu.Unlock()
		for _, s := range open {
			busID := a.knownBusID(s.ID())
			retired := false
			if b != nil && busID != "" {
				_, retired, _ = b.LeadOf(ctx, busID)
			}
			st := a.chatState(ctx, b, busID, s.ID(), retired, false)
			add(ChatStateEntry{SessionID: s.ID(), Bus: busID, ChatState: st})
		}
		if b == nil {
			return
		}
		rooms, err := b.Rooms(ctx)
		if err != nil {
			return
		}
		for _, r := range rooms {
			var states []ChatState
			for _, m := range r.Members {
				if m.Kind == "human" {
					continue
				}
				chat := a.chatOfBus(m.Key)
				_, retired, _ := b.LeadOf(ctx, m.Key)
				st := a.chatState(ctx, b, m.Key, chat, retired, m.Open)
				states = append(states, st)
				add(ChatStateEntry{SessionID: chat, Bus: m.Key, ChatState: st})
			}
			view.Groups = append(view.Groups, GroupStateEntry{Name: r.Name, Counts: countStates(states)})
		}
	}
	ran, err := a.busHost().IfOpen(func(b *bus.Bus) error { compute(b); return nil })
	if err != nil {
		logf("bus", "error", "reading the chats' states from the bus: %v", err)
	}
	if !ran || err != nil {
		compute(nil)
	}
	// The chats come out of maps, so they are put in an order that does not change between passes, or every pass
	// would look different from the last.
	sort.Slice(view.Chats, func(i, j int) bool {
		if view.Chats[i].SessionID != view.Chats[j].SessionID {
			return view.Chats[i].SessionID < view.Chats[j].SessionID
		}
		return view.Chats[i].Bus < view.Chats[j].Bus
	})
	sort.Slice(view.Groups, func(i, j int) bool { return view.Groups[i].Name < view.Groups[j].Name })
	return view, a.recordChatStates(view)
}

// recordChatStates puts each open chat's state in its information, and, for a lead, its team's counts, and returns the
// chats whose information changed.
func (a *App) recordChatStates(view ChatStatesView) []*Session {
	var changed []*Session
	teams := map[string]*StateCounts{}
	_, _ = a.busHost().IfOpen(func(b *bus.Bus) error {
		leads, err := b.Leads(context.Background())
		if err != nil {
			return err
		}
		for _, lead := range leads {
			c := a.teamCounts(b, context.Background(), lead, map[string]bool{lead: true})
			c.Finish()
			teams[lead] = &c
		}
		return nil
	})
	for _, e := range view.Chats {
		s := a.session(e.SessionID)
		if s == nil || e.SessionID == "" {
			continue
		}
		st := e.ChatState
		if s.SetState(&st, teams[e.Bus]) {
			changed = append(changed, s)
		}
	}
	return changed
}

// teamCounts counts the states of every member under a lead, nested ones included.
func (a *App) teamCounts(b *bus.Bus, ctx context.Context, lead string, seen map[string]bool) StateCounts {
	var c StateCounts
	members, err := b.TeamMembers(ctx, lead)
	if err != nil {
		return c
	}
	for _, m := range members {
		if seen[m.Session] {
			continue
		}
		seen[m.Session] = true
		c.Add(a.chatState(ctx, b, m.Session, a.chatOfBus(m.Session), m.Retired, false))
		if led, _ := b.IsLead(ctx, m.Session); led {
			c.Merge(a.teamCounts(b, ctx, m.Session, seen))
		}
	}
	return c
}

func (a *App) chatStateClock() *sessionhost.StateClock {
	s := &a.chatStates
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.clock == nil {
		s.clock = sessionhost.NewStateClock()
	}
	return s.clock
}

// ChatStates is every chat's state, numbered, for a page that loads or has missed an event.
func (a *App) ChatStates() ChatStatesView {
	view, _ := a.computeChatStates()
	view.StateGen, view.StateVer = a.chatStateClock().Next()
	return view
}

// sendChatStates is a pass: it works the states out and sends them as agent:chatstate when they differ from the last sent.
func (a *App) sendChatStates() {
	s := &a.chatStates
	s.run.Lock()
	defer s.run.Unlock()
	s.mu.Lock()
	s.timer = nil
	closed := s.closed
	s.mu.Unlock()
	if closed || folderClosed() {
		return
	}
	view, changed := a.computeChatStates()
	for _, c := range changed {
		sessionhost.EmitSession(a.emit, c)
	}
	data, _ := json.Marshal(view)
	s.mu.Lock()
	same := s.sent == string(data)
	s.sent = string(data)
	s.mu.Unlock()
	if same {
		return
	}
	view.StateGen, view.StateVer = a.chatStateClock().Next()
	sessionhost.EmitChatStates(a.emit, view)
}
