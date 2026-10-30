package sessionhost

import (
	_ "embed"
	"studio/internal/acp"
	"sync"
	"time"
)

//go:embed closed_sessions.go
var ClosedSessionsSource []byte

type ClosedSessionsHost struct {
	NormalizeProvider   func(string) string
	Clock               func() time.Time
	LeaveBus            func(*Conversation)
	Emit                func(string, any)
	StopProvider        func(string)
	CallWithin          func(Connection, time.Duration, string, any, any) error
	RevokeSession       func(string)
	CloseChatLog        func(string)
	CancelQuestions     func(string)
	DisconnectSubagents func(*Conversation)
	Log                 func(string, string, ...any)
}

// ClosedSessions uses the host's state mutex for Runtime operations and owns
// the sole wait group for session-close and fallback provider-stop work.
// Host effects run outside the state mutex. Clock retains its existing use
// under the private conversation mutex and must not re-enter state operations.
type ClosedSessions struct {
	mu      *sync.Mutex
	runtime *Runtime
	host    ClosedSessionsHost
	ends    sync.WaitGroup
}

func NewClosedSessions(mu *sync.Mutex, runtime *Runtime, host ClosedSessionsHost) *ClosedSessions {
	return &ClosedSessions{mu: mu, runtime: runtime, host: host}
}

// Wait joins tracked end work. The caller must first stop and join the watcher
// that admits work through EndClosed. It does not join unrelated turns.
func (c *ClosedSessions) Wait() { c.ends.Wait() }
func (c *ClosedSessions) conversation(id string) *Conversation {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.runtime.ConversationLocked(id)
}
func (c *ClosedSessions) hasWindow(id string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.runtime.HasWindowLocked(id)
}

// A chat that has been closed for ClosedSessionEnd without a break has its
// session with the agent ended (docs/plans/group-chats.md, decision 11).
// Until then Dinah keeps the session loaded, so that a message or the
// person brings the chat back at once. The chats this covers are those that
// keep a loaded session while closed: a chat open in the background that
// closed itself for being idle (bus_wake.go, park), and a chat whose window
// the person closed, which Dinah keeps open with no window. The clock starts
// when the watcher first sees the chat closed with nothing running, and it
// stops when the chat has a window again, is open in the background, runs a
// turn or has work of the agent's running after a turn; a chat opened again
// before the time is up starts from nothing the next time it closes.
//
// The session is ended by ACP's session/close where the agent advertises
// sessionCapabilities.close
// (https://agentclientprotocol.com/protocol/session-setup). Dinah's own
// adapters for Codex and Ollama advertise it, as does the Claude adapter it
// runs; for Devin it depends on what the running Devin says at initialize.
// An agent with no session/close is stopped instead, which ends every
// session in its process, but only when every session it has loaded is due
// and the agent advertises loadSession, so each chat can be loaded again;
// otherwise its sessions stay loaded. Once ended, the chat is closed as a
// saved chat is, and opening it, whether by the person or for a message
// that wakes it, loads it again as opening a saved chat does.
//
// A session the agent fails to close stays loaded and is treated as a chat
// that closed itself in the background, with its clock started again.

const (
	// ClosedSessionEnd is how long a chat stays closed before Dinah ends its
	// session with the agent. Dinah Development chose four hours on 28
	// October 2026, and it can be changed here.
	ClosedSessionEnd = 4 * time.Hour
	// CloseSessionPatience is how long the agent is given to close one
	// session.
	CloseSessionPatience = 30 * time.Second
)

// WaitForEnd waits while Dinah ends the chat's session, if it is.
func (c *ClosedSessions) WaitForEnd(id string) {
	s := c.conversation(id)
	if s == nil {
		return
	}
	ending := s.Ending()
	if ending != nil {
		ending.Wait()
	}
}

// EndClosed keeps each chat's clock, as the comment at the top of
// this file describes, and ends the sessions of the chats closed for
// ClosedSessionEnd. The bus watcher calls it at every round.
func (c *ClosedSessions) EndClosed() {
	c.mu.Lock()
	known, windows := c.runtime.WindowsLocked()
	all := make([]*Conversation, 0, c.runtime.ConversationCountLocked())
	for _, s := range c.runtime.ConversationsLocked() {
		all = append(all, s)
	}
	c.mu.Unlock()
	// Until the page has said which chats have windows, no chat is known to
	// be closed.
	if !known {
		return
	}
	now := c.host.Clock()
	due := map[string]bool{}
	byProvider := map[string][]*Conversation{}
	for _, s := range all {
		provider := c.host.NormalizeProvider(s.Provider())
		byProvider[provider] = append(byProvider[provider], s)

		if s.ClosedDue(windows[s.ID()], now, ClosedSessionEnd) {
			due[s.ID()] = true
		}
	}
	for provider, sessions := range byProvider {
		var ending []*Conversation
		for _, s := range sessions {
			if due[s.ID()] {
				ending = append(ending, s)
			}
		}
		if len(ending) == 0 {
			continue
		}
		c.mu.Lock()
		var conn Connection
		var init acp.InitializeResult
		if p := c.runtime.ProviderLocked(provider); p != nil {
			conn, init = p.Connection(), p.Initialization()
		}
		c.mu.Unlock()
		if conn == nil {
			continue
		}
		switch {
		case init.AgentCapabilities.SessionCapabilities.Close != nil:
			for _, s := range ending {
				if done := c.beginEnd(s); done != nil {
					c.ends.Add(1)
					go func() {
						defer c.ends.Done()
						c.closeSession(conn, s, done)
					}()
				}
			}
		case init.AgentCapabilities.LoadSession && len(ending) == len(sessions):
			var stopping []*Conversation
			var dones []*SessionEnd
			for _, s := range ending {
				if done := c.beginEnd(s); done != nil {
					stopping, dones = append(stopping, s), append(dones, done)
				}
			}
			if len(stopping) != len(sessions) {
				// A chat changed while it was looked at; the others are put
				// back as they were and looked at again next round.
				for i, s := range stopping {
					c.abandonEnd(s, dones[i])
				}
				continue
			}
			c.ends.Add(1)
			go func() {
				defer c.ends.Done()
				c.host.Log("info", "stopping %s, which cannot close a session, since all %d of its chats have been closed for %s", provider, len(stopping), ClosedSessionEnd)
				c.host.StopProvider(provider)
				for i, s := range stopping {
					s.FinishEnd()
					dones[i].Complete()
				}
			}()
		}
	}
}

// beginEnd marks a chat as having its session ended, unless it has
// changed since it was found due: it is closed to the person and off the
// bus, so nothing starts a turn in it meanwhile. It returns an opaque completion to
// settle when the end is done, or nil.
func (c *ClosedSessions) beginEnd(s *Conversation) *SessionEnd {
	if c.hasWindow(s.ID()) {
		return nil
	}

	done := s.BeginEnd()
	if done == nil {
		return nil
	}

	c.host.LeaveBus(s)
	EmitSession(c.host.Emit, s)
	return done
}

// abandonEnd puts a chat whose session was about to end back as a
// chat closed with its session loaded.
func (c *ClosedSessions) abandonEnd(s *Conversation, done *SessionEnd) {
	s.AbandonEndAt(c.host.Clock)

	done.Complete()
	EmitSession(c.host.Emit, s)
}

// closeSession ends one chat's session with session/close, and lets the
// chat go as the end of its agent's process does (watchProvider).
func (c *ClosedSessions) closeSession(conn Connection, s *Conversation, done *SessionEnd) {
	if err := c.host.CallWithin(conn, CloseSessionPatience, "session/close", acp.CloseSessionParams{SessionID: s.ID()}, nil); err != nil {
		c.host.Log("error", "chat %s has been closed for %s, but its agent did not close its session, which stays loaded: %v", s.ID(), ClosedSessionEnd, err)
		c.abandonEnd(s, done)
		return
	}
	c.mu.Lock()
	c.runtime.RemoveExpectedLocked(s.ID(), s)
	c.mu.Unlock()
	c.host.RevokeSession(s.ID())
	c.host.CloseChatLog(s.ID())
	c.host.CancelQuestions(s.ID())
	c.host.DisconnectSubagents(s)
	s.FinishEnd()
	done.Complete()
	c.host.Log("info", "ended the session of chat %s, closed for %s", s.ID(), ClosedSessionEnd)
	EmitSession(c.host.Emit, s)
}
