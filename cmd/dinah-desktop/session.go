package main

import (
	"context"
	"errors"
	"maps"
	"strings"

	"studio/internal/acp"
	"studio/internal/bus"
	"studio/internal/sessionhost"
)

type Session = sessionhost.Conversation
type sessionSeed = sessionhost.Identity
type sessionInitial = sessionhost.InitialActivity

func newSessionState(seed *sessionSeed, initial ...sessionInitial) *Session {
	return newLoadingSessionState(seed, false, false, initial...)
}
func newLoadingSessionState(seed *sessionSeed, replay, restore bool, initial ...sessionInitial) *Session {
	state := sessionInitial{}
	if len(initial) > 0 {
		state = initial[0]
	}
	return sessionhost.NewConversation(*seed, state, replay, restore)
}

// replayBatch is how many replayed updates go to the page in one event.
const replayBatch = 200

// flushReplay sends the page the replayed updates it has not had yet.
func (a *App) flushReplay(s *Session) {
	sessionhost.EmitChatReplay(a.emit, s, s.DrainReplay())
}

// ChatEvents is what a page missed of a chat's transcript: the entries of the
// chat's event log after the number given, for the generation given. A page
// that sees a number beyond the next one, or is told to resync, reads them
// here; a chat not open here answers with Reset, and the page reopens it
// (docs/plans/event-streams.md).
func (a *App) ChatEvents(id, gen string, after uint64) sessionhost.ChatEventsView {
	s := a.session(id)
	if s == nil {
		return sessionhost.ChatEventsView{Events: []sessionhost.ChatEvent{}, Reset: true}
	}
	return s.ChatEvents(gen, after)
}

// turnEnd is how a turn ended, for a caller waiting on it.
type turnEnd struct {
	stopReason string
	err        error
}

// prompt aliases the opaque input and reply identity owned by sessionhost.
type prompt = sessionhost.Prompt

func newPrompt(text string, blocks []acp.ContentBlock) prompt {
	return sessionhost.NewPrompt(text, blocks)
}

func promptPointer(text string, blocks []acp.ContentBlock) *prompt {
	p := newPrompt(text, blocks)
	return &p
}

// dropPrompts resolves the callers whose queued prompts will not run.
func dropPrompts(prompts []prompt, why string) {
	for _, p := range prompts {
		p.Resolve(sessionhost.End{StopReason: "error", Err: errors.New(why)})
	}
}

type SessionInfo = sessionhost.Info

func (a *App) session(id string) *Session {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.runtime.ConversationLocked(id)
}

// NewSession starts a session in the workspace in front, with the person's
// default settings.
func (a *App) NewSession() (SessionInfo, error) {
	return a.NewSessionAt(a.currentWorkspace())
}

// NewSessionAt retains the chosen workspace while the agent connects.
func (a *App) NewSessionAt(ws string) (SessionInfo, error) {
	return a.NewSessionWithProviderAt(ws, a.defaultProvider())
}

func (a *App) NewSessionWithProviderAt(ws, provider string) (SessionInfo, error) {
	return a.sessionsLifecycle().New(ws, provider, false)
}

// Sessions lists the sessions of the workspace in front: those open in this
// run, and those in Devin's history when Devin can list them. Newest first.
func (a *App) Sessions() ([]SessionInfo, error) {
	return a.SessionsAt(a.currentWorkspace())
}

// SessionsAt lists one workspace without changing the current selection.
func (a *App) SessionsAt(ws string) ([]SessionInfo, error) { return a.sessionsLifecycle().List(ws) }

// OpenSession reopens a session from Devin's history in this run. Devin
// replays the conversation as session/update notifications before it
// answers, so the page sees the whole history arrive. The session gets the
// settings the person last chose in it.
func (a *App) OpenSession(id string) (SessionInfo, error) {
	return a.OpenSessionAt(a.currentWorkspace(), id)
}

// OpenSessionAt uses the workspace associated with the chat window. A chat
// that cannot be opened is logged with the agent's reason, which the page
// shows only until the window closes.
func (a *App) OpenSessionAt(ws, id string) (SessionInfo, error) {
	return a.sessionsLifecycle().Open(ws, id, nil)
}

// LoadFullHistory reopens a chat with session/load, so that the agent replays
// the whole conversation, thinking rows and subagents included, and brings in
// any change made to the session outside Dinah. A chat reopened with
// session/resume shows only its saved copy until this is run. It refuses while
// the chat is working.
func (a *App) LoadFullHistory(ws, id string) (SessionInfo, error) {
	return a.sessionsLifecycle().LoadFullHistory(ws, id)
}

// DeleteSession removes a session from Devin's session history, where Devin
// supports it, and so from every list that reads it, Devin Desktop's too. A
// session open in this run is forgotten here as well. It returns
// errNoDelete when Devin does not support deleting, so that the page can
// offer to hide the session instead.
var errNoDelete = sessionhost.ErrNoDelete

func (a *App) DeleteSession(id string) error { return a.sessionsLifecycle().Delete(id) }

// HideSession takes a session off this app's list without deleting it in
// Devin.
func (a *App) HideSession(id string) error {
	_ = a.updateSettings(func(s *Settings) error {
		s.Hidden = append(s.Hidden, id)
		return nil
	})
	a.sessionsLifecycle().Forget(id)
	return nil
}

// UnhideSessions brings back every hidden session.
func (a *App) UnhideSessions() error {
	return a.updateSettings(func(s *Settings) error {
		s.Hidden = nil
		return nil
	})
}

func (a *App) hidden(id string) bool {
	for _, h := range a.settings.Hidden {
		if h == id {
			return true
		}
	}
	return false
}

// RenameSession gives a session the person's own name, or with an empty
// name, goes back to the agent's title. It returns every name given. The
// bus takes the new title at once, so the chat is known by it everywhere.
func (a *App) RenameSession(id, title string) (map[string]string, error) {
	titles, err := a.renameSession(id, title)
	if s := a.session(id); s != nil {
		a.busCoordinator().PublishBusTitle(s)
	} else if t := strings.TrimSpace(title); t != "" && err == nil {
		// A closed chat the bus knows, such as one in a group chat, is
		// known by its new title at once too. A cleared name goes back to
		// the agent's title, which only the open chat has, so the bus keeps
		// the old one until the chat opens.
		a.mu.Lock()
		busID := a.settings.BusSessions[id]
		a.mu.Unlock()
		if busID != "" {
			if e := a.busHost().With(func(b *bus.Bus) error { return b.SetTitle(context.Background(), busID, t) }); e != nil && !errors.Is(e, bus.ErrNotFound) {
				logf("bus", "error", "giving chat %s its title on the bus: %v", id, e)
			}
		}
	}
	return titles, err
}

func (a *App) renameSession(id, title string) (map[string]string, error) {
	title = bus.CleanTitle(title)
	// A copy: the caller sends it to the page after the lock is released, and
	// another chat may be renamed meanwhile, writing the map itself, which
	// ends the program (15 October 2026).
	var titles map[string]string
	err := a.updateSettings(func(s *Settings) error {
		if s.SessionTitles == nil {
			s.SessionTitles = map[string]string{}
		}
		if title == "" {
			delete(s.SessionTitles, id)
		} else {
			s.SessionTitles[id] = title
		}
		titles = maps.Clone(s.SessionTitles)
		return nil
	})
	if titles == nil {
		a.mu.Lock()
		titles = maps.Clone(a.settings.SessionTitles)
		a.mu.Unlock()
	}
	return titles, err
}

// freshSelection leaves the Editor's selection out of a message when it is
// the one the chat's previous message carried, from the same document, so
// a passage left selected is not attached to every message as its subject.
// A selection that changes, or comes back after another, is sent again.
func freshSelection(s *Session, doc *DocRef) *DocRef { return sessionhost.FreshSelection(s, doc) }

// makePrompt builds a prompt from the person's words, the document they
// picked and what they attached.
func (a *App) makePrompt(s *Session, text string, doc *DocRef, atts []Attachment) (prompt, error) {
	return a.promptFrom(s, withDoc(text, a.draftDocRef(freshSelection(s, doc))), atts)
}

// promptFrom builds a prompt from text that is already complete: a steering
// message, whose selection was weighed once already, or a prompt the app
// makes, which has no selection and leaves the record of the person's last
// one alone.
func (a *App) promptFrom(s *Session, full string, atts []Attachment) (prompt, error) {
	instructions, err := a.providerSkillPrompt(s, full)
	if err != nil {
		return prompt{}, err
	}
	blocks, err := a.blocks(s, instructions, atts)
	return newPrompt(full, blocks), err
}

func (a *App) Send(id, text string, doc *DocRef, atts []Attachment) error {
	return a.turnIO().Send(id, text, doc, atts)
}

func (a *App) askWait(ctx context.Context, s *Session, p prompt) (string, turnEnd) {
	words, end := a.turnIO().AskWait(ctx, s, p)
	return words, turnEnd{stopReason: end.StopReason, err: end.Err}
}
func (a *App) Queue(id, text string, doc *DocRef, atts []Attachment) error {
	return a.turnIO().Queue(id, text, doc, atts)
}
func (a *App) Unqueue(id string) { a.turnIO().Unqueue(id) }
func (a *App) Steer(id, text string, doc *DocRef, atts []Attachment) error {
	return a.turnIO().Steer(id, text, doc, atts)
}
func (a *App) StopAndSteer(id, text string, doc *DocRef, atts []Attachment) error {
	return a.turnIO().StopAndSteer(id, text, doc, atts)
}
func (a *App) Cancel(id string) error { return a.turnIO().Cancel(id) }
