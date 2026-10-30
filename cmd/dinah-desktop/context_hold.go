package main

import (
	"errors"
	"fmt"
	"slices"

	"studio/internal/sessionhost"
)

// What the window does when a chat's known context window is full
// (docs/plans/ollama-context-window.md). A provider that measures its window
// says so with a context module (provider_modules.go); this file asks it and
// names none.

// contextWindow is where a chat's window stands, as its provider measures it.
type contextWindow struct {
	Used, Size int
	// Full says the window is known and the next send is to be held.
	Full bool
	// Choice is the course the person chose for a full window, or "".
	Choice string
	// CanSetAside says setting turns aside would free some, so that the course is
	// offered only when it can work.
	CanSetAside bool
}

// contextModule is a provider's measuring of its chats' windows and the courses
// it can take when one is full.
type contextModule struct {
	// State says where the chat's window stands.
	State func(a *App, conn agentConnection, id string) (contextWindow, error)
	// SetAside sets the chat's oldest turns aside and says how many.
	SetAside func(a *App, conn agentConnection, id string) (int, error)
	// Choose keeps the course the person chose, or with "" forgets it.
	Choose func(a *App, conn agentConnection, id, choice string) error
}

// contextOf is the context module of a chat's provider, and false when its
// provider has none.
func (a *App) contextOf(s *Session) (*contextModule, bool) {
	m, ok := providerModules[normalizedProvider(s.Provider())]
	if !ok || m.Context == nil {
		return nil, false
	}
	return m.Context, true
}

// contextGate is called before a prompt starts a turn. When the chat's window is
// full it holds the prompt, which it has then put back in the chat's queue, and
// returns true, unless the person already chose a course for this chat, which
// then runs without a hold.
func (a *App) contextGate(s *Session, p *prompt) bool {
	m, ok := a.contextOf(s)
	if !ok {
		return false
	}
	conn := a.sessionConn(s)
	if conn == nil {
		return false
	}
	w, err := m.State(a, conn, s.ID())
	if err != nil {
		logf("chat", "warning", "the context window of chat %s could not be read: %v", s.ID(), err)
		return false
	}
	s.SetContextChoice(w.Choice)
	if !w.Full {
		if s.Hold() != nil {
			s.SetHold(nil)
		}
		return false
	}
	switch w.Choice {
	case sessionhost.HoldSetAside:
		n, err := m.SetAside(a, conn, s.ID())
		if err != nil {
			logf("chat", "warning", "turns of chat %s could not be set aside: %v", s.ID(), err)
		}
		if err == nil && n > 0 {
			return false
		}
		// Nothing could be set aside, so the send waits for the person to choose
		// again, and does not go on into a window that is full.
	case sessionhost.HoldCompact:
		s.HoldPrompt(p)
		go func() { _, _ = a.compactHeld(s, a.holdFor(w)) }()
		return true
	}
	s.SetHold(a.holdFor(w))
	s.HoldPrompt(p)
	a.emitHeld(s)
	return true
}

// holdFor is the hold the page shows for a window that is full.
func (a *App) holdFor(w contextWindow) *sessionhost.ContextHold {
	actions := []string{sessionhost.HoldCompact}
	if w.CanSetAside {
		actions = []string{sessionhost.HoldSetAside, sessionhost.HoldCompact}
	}
	return &sessionhost.ContextHold{Kind: "context", Used: w.Used, Size: w.Size, Actions: actions}
}

// emitHeld tells the page the chat's queue and information, which now say a send
// is held.
func (a *App) emitHeld(s *Session) {
	sessionhost.EmitQueue(a.emit, s)
	sessionhost.EmitSession(a.emit, s)
}

// compactHeld takes every prompt out of the chat's queue, the held one first,
// continues the chat with its own provider as a compaction, and sends the prompts to
// the new chat in order as they were made, so that what was attached to them goes
// too: the pictures are in the prompt, and the files it links to are in the
// workspace, which the new chat shares. The queue is taken whole because a
// continuation is refused while anything is queued. When the chat cannot be
// continued, the prompts go back in the queue, the hold is put back as restore, the
// page is told, and the reason is logged, so the chat is as it was and its queued
// messages are not lost.
func (a *App) compactHeld(s *Session, restore *sessionhost.ContextHold) (SessionInfo, error) {
	held := s.TakeQueued()
	if len(held) == 0 {
		return SessionInfo{}, errors.New("no send is held")
	}
	s.SetHold(nil)
	result, err := a.continuations().Compact(s)
	if err != nil {
		s.RestoreQueued(held)
		s.SetHold(restore)
		a.emitHeld(s)
		logf("chat", "warning", "chat %s could not be compacted, so its %d queued messages wait again: %v", s.ID(), len(held), err)
		return SessionInfo{}, fmt.Errorf("the chat could not be continued: %w", err)
	}
	a.emitHeld(s)
	var failed error
	for _, p := range held {
		if err := a.turnIO().SendPrompt(result.Session.ID, p); err != nil && failed == nil {
			failed = err
		}
	}
	// The messages are queued in the new chat, which keeps them, so the earlier
	// chat's file no longer needs to.
	s.SettleCarry()
	if failed != nil {
		logf("chat", "warning", "chat %s was compacted, but a queued message was not sent to the new chat: %v", s.ID(), failed)
		return result.Session, fmt.Errorf("the chat was continued, but a queued message was not sent: %w", failed)
	}
	return result.Session, nil
}

// ResolveHold runs the course the person chose for a held send, and remembers it
// for the chat. A set-aside releases the held send in the same chat; a compaction
// continues the chat with its own provider and sends it to the new chat, whose
// information it returns.
func (a *App) ResolveHold(id, action string) (SessionInfo, error) {
	s := a.session(id)
	if s == nil {
		return SessionInfo{}, errors.New("that chat is not open")
	}
	hold := s.Hold()
	if hold == nil {
		return SessionInfo{}, errors.New("no send is held in this chat")
	}
	if !slices.Contains(hold.Actions, action) {
		return SessionInfo{}, errors.New("that is not a course for this chat")
	}
	m, ok := a.contextOf(s)
	if !ok {
		return SessionInfo{}, errors.New("this chat has no context window to manage")
	}
	conn := a.sessionConn(s)
	if conn == nil {
		return SessionInfo{}, errors.New("the agent is not running")
	}
	if err := m.Choose(a, conn, id, action); err != nil {
		return SessionInfo{}, err
	}
	s.SetContextChoice(action)
	if action == sessionhost.HoldCompact {
		return a.compactHeld(s, hold)
	}
	n, err := m.SetAside(a, conn, id)
	if err != nil {
		return SessionInfo{}, err
	}
	if n == 0 {
		return SessionInfo{}, errors.New("no turn could be set aside, because none had its size recorded; compact the chat instead")
	}
	s.SetHold(nil)
	sessionhost.EmitSession(a.emit, s)
	a.turnIO().StartWaiting(s)
	return s.Info(), nil
}

// ForgetContextChoice forgets the course the person chose for a full window, so
// that the chat asks again the next time it fills.
func (a *App) ForgetContextChoice(id string) error {
	s := a.session(id)
	if s == nil {
		return errors.New("that chat is not open")
	}
	m, ok := a.contextOf(s)
	if !ok {
		return errors.New("this chat has no context window to manage")
	}
	conn := a.sessionConn(s)
	if conn == nil {
		return errors.New("the agent is not running")
	}
	if err := m.Choose(a, conn, id, ""); err != nil {
		return err
	}
	s.SetContextChoice("")
	sessionhost.EmitSession(a.emit, s)
	return nil
}

// loadContextChoice reads the course already chosen for a chat that has just
// opened, so that its menu offers to forget it.
func (a *App) loadContextChoice(s *Session) {
	m, ok := a.contextOf(s)
	if !ok {
		return
	}
	conn := a.sessionConn(s)
	if conn == nil {
		return
	}
	if w, err := m.State(a, conn, s.ID()); err == nil {
		s.SetContextChoice(w.Choice)
	}
}
