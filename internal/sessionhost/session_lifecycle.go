package sessionhost

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"studio/internal/acp"
	"studio/internal/layout"
	"studio/internal/providers"
	"sync"
	"time"
)

//go:embed session_lifecycle.go
var SessionLifecycleSource []byte

// ChannelCapability authorizes binding and revocation without exposing the
// shell's channel token or record to session state or views.
type ChannelCapability interface {
	Bind(string) error
	Revoke()
}

type SessionLifecycleHost struct {
	NormalizeProvider func(string) string
	PrepareApplets    func(string, string) error
	Live              func(string) (Connection, acp.InitializeResult, error)
	Defaults          func(string) map[string]json.RawMessage
	SavedOptions      func(string) (map[string]json.RawMessage, string)
	NewChannel        func(Connection, string) (ChannelCapability, []any, error)
	ManagedServers    func() ([]any, error)
	BusSessionID      func(string) string
	WithBusServer     func(string, string, string, []any) []any
	// AdoptBusServer serves an identity the chat already has to the new
	// session's agent, registering it on the bus, and reports a failure
	// where WithBusServer opens the chat without the bus.
	AdoptBusServer func(id, workspace string, servers []any) ([]any, error)
	EndBusSession  func(string)
	Call           func(Connection, string, any, any) error
	CallWithin     func(Connection, time.Duration, string, any, any) error
	SessionParams  func(string, string, string, []any) any
	InitTranscript func(*Conversation, bool)
	// ResumeTranscript opens the chat's saved copy for a reopen without a
	// replay, and says why it could not, or "" when it did (history.go).
	ResumeTranscript func(*Conversation) string
	BeginLog         func(string, string, string, string)
	NoteLog          func(string, string)
	RestoreOptions   func(Connection, string, []acp.ConfigOption, map[string]json.RawMessage, *Reopening) ([]acp.ConfigOption, []AppliedSubstitution, error)
	FinishOptions    func(*Conversation, []acp.ConfigOption) error
	JoinBus          func(*Conversation, string)
	// StartWaiting sends the first message held while the chat loaded.
	StartWaiting func(*Conversation)
	// RestoreQueue puts back the queued messages kept for a chat that has opened,
	// before it takes a turn (queue_restore.go); nil for none.
	RestoreQueue func(*Conversation)
	// AfterOpen is called once a chat has opened, for what the provider keeps
	// for it that the chat's information should say.
	AfterOpen func(*Conversation)
	// ResolveContinuation gives the newest chat that continues the one asked
	// for with another provider, or the chat itself.
	ResolveContinuation    func(string) (string, error)
	Emit                   func(string, any)
	ListProviders          func() []string
	Hidden                 func(string) bool
	Title                  func(string, string) string
	FilterAncestors        func([]Info, string) []Info
	SavedProvider          func(string) (string, error)
	WaitForEnd             func(string)
	Unpark                 func(*Conversation)
	DropSubagentIndex      func(string)
	SettleReplayedPrompt   func(*Conversation)
	FlushReplay            func(*Conversation)
	DisconnectSubagents    func(*Conversation)
	FinishTranscriptReplay func(*Conversation, bool)
	Reopening              func(string) *Reopening
	WatchReopen            func(string, string) func()
	Connected              func(string, error)
	Cancel                 func(*Conversation) error
	Tombstone              func(string) error
	// ForgetTeam is called when a chat is deleted: the members it led become ordinary chats, and
	// its own place in a team ends.
	ForgetTeam      func(string)
	ForgetOptions   func(string)
	ForgetTitle     func(string)
	LeaveBus        func(*Conversation)
	RevokeSession   func(string)
	ForgetJournal   func(string)
	CancelQuestions func(string)
	Log             func(string, string, ...any)
}

// SessionLifecycle shares the host state mutex and sole Runtime. Callbacks run
// outside that mutex; transcript serialization remains the host's authority.
type SessionLifecycle struct {
	mu      *sync.Mutex
	runtime *Runtime
	host    SessionLifecycleHost

	// opening has the opens in flight, by chat, so that two opens of one
	// chat, such as the page's connect and the engine's own after the grace,
	// take turns instead of each calling the agent.
	openMu  sync.Mutex
	opening map[string]*openCall
}

// openCall is one open in flight, whose result a second open of the chat
// waits for and returns.
type openCall struct {
	done chan struct{}
	info Info
	err  error
}

func NewSessionLifecycle(mu *sync.Mutex, runtime *Runtime, host SessionLifecycleHost) *SessionLifecycle {
	return &SessionLifecycle{mu: mu, runtime: runtime, host: host}
}
func (l *SessionLifecycle) conversation(id string) *Conversation {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.runtime.ConversationLocked(id)
}
func (l *SessionLifecycle) providerConnectionLocked(provider string) Connection {
	if p := l.runtime.ProviderLocked(provider); p != nil {
		return p.Connection()
	}
	return nil
}
func (l *SessionLifecycle) New(ws, provider string, continuing bool) (Info, error) {
	return l.newSession(ws, provider, continuing, "")
}

// NewAdopting starts the session that a provider switch continues a chat in.
// With the chat's bus identity, the new session is started under it, since
// the helper that serves the bus to the agent is given its identity when the
// session starts and the bus does not change. It does not join the identity
// or save it: the old chat keeps it until the switch commits, and the switch
// then moves it (Continuations.Continue). A failure to register the identity
// fails the session, since a switch that left the chat without its place on
// the bus would defeat the point of adopting it, and a failure never ends the
// identity, which the old chat still owns. With no identity, a chat that has
// not used the bus, it starts as New does.
func (l *SessionLifecycle) NewAdopting(ws, provider, adopt string) (Info, error) {
	return l.newSession(ws, provider, true, adopt)
}

func (l *SessionLifecycle) newSession(ws, provider string, continuing bool, adopt string) (Info, error) {
	provider = l.host.NormalizeProvider(provider)
	if ws == "" || !filepath.IsAbs(ws) {
		return Info{}, errors.New("choose an absolute workspace path first")
	}
	if err := l.host.PrepareApplets(provider, ws); err != nil {
		return Info{}, fmt.Errorf("prepare the managed applet integration: %w", err)
	}
	conn, _, err := l.host.Live(provider)
	if err != nil {
		return Info{}, err
	}
	defaults := l.host.Defaults(provider)
	var ns acp.NewSessionResult
	channel, servers, err := l.host.NewChannel(conn, ws)
	if err != nil {
		return Info{}, err
	}
	if providers.For(provider).ManagedServers {
		servers, err = l.host.ManagedServers()
		if err != nil {
			channel.Revoke()
			return Info{}, err
		}
	}
	busID := adopt
	bound := false
	defer func() {
		if !bound {
			channel.Revoke()
			if adopt == "" {
				l.host.EndBusSession(busID)
			}
		}
	}()
	if adopt == "" {
		busID = l.host.BusSessionID("")
		servers = l.host.WithBusServer(busID, ws, "", servers)
	} else if servers, err = l.host.AdoptBusServer(adopt, ws, servers); err != nil {
		return Info{}, fmt.Errorf("keep the chat's place on the bus: %w", err)
	}
	if err := l.host.Call(conn, "session/new", l.host.SessionParams(provider, "", ws, servers), &ns); err != nil {
		return Info{}, err
	}
	if err := channel.Bind(ns.SessionID); err != nil {
		return Info{}, err
	}
	s := NewConversation(Identity{ID: ns.SessionID, Provider: provider, Cwd: ws}, InitialActivity{Open: true, Switching: continuing}, false, true)
	l.mu.Lock()
	if l.providerConnectionLocked(provider) != conn {
		l.mu.Unlock()
		return Info{}, errors.New("the agent stopped while opening the session")
	}
	select {
	case <-conn.Done():
		l.mu.Unlock()
		return Info{}, errors.New("the agent stopped while opening the session")
	default:
	}
	l.runtime.PutConversationLocked(s.ID(), s)
	l.mu.Unlock()
	l.host.InitTranscript(s, false)
	l.host.BeginLog(s.ID(), l.host.NormalizeProvider(provider), ws, "")
	opts, _, err := l.host.RestoreOptions(conn, s.ID(), ns.ConfigOptions, defaults, nil)
	if err == nil {
		err = l.host.FinishOptions(s, opts)
	}
	if err != nil {
		l.mu.Lock()
		l.runtime.RemoveConversationLocked(s.ID())
		l.mu.Unlock()
		s.Close()
		return Info{}, err
	}
	bound = true
	if adopt == "" {
		l.host.JoinBus(s, busID)
	}
	info := s.InfoVersioned()
	if !continuing {
		EmitInfo(l.host.Emit, info)
	}
	return info, nil
}

func (l *SessionLifecycle) List(ws string) ([]Info, error) {
	if ws == "" || !filepath.IsAbs(ws) {
		return []Info{}, errors.New("choose an absolute workspace path first")
	}
	l.mu.Lock()
	var out []Info
	seen := map[string]bool{}
	for _, s := range l.runtime.ConversationsLocked() {
		if layout.SamePath(s.Cwd(), ws) {
			out = append(out, s.InfoVersioned())
			seen[s.ID()] = true
		}
	}
	l.mu.Unlock()
	for _, provider := range l.host.ListProviders() {
		conn, init, err := l.host.Live(provider)
		if err == nil && init.AgentCapabilities.SessionCapabilities.List != nil {
			cursor := ""
			for page := 0; page < 5; page++ {
				var res acp.ListSessionsResult
				if err := l.host.CallWithin(conn, 15*time.Second, "session/list", acp.ListSessionsParams{Cwd: ws, Cursor: cursor}, &res); err != nil {
					break
				}
				for _, x := range res.Sessions {
					hide := l.host.Hidden(x.SessionID)
					if !seen[x.SessionID] && !hide && layout.SamePath(x.Cwd, ws) {
						out = append(out, Info{ID: x.SessionID, Provider: provider, Cwd: x.Cwd, Title: l.host.Title(x.SessionID, x.Title), UpdatedAt: x.UpdatedAt})
						seen[x.SessionID] = true
					}
				}
				if res.NextCursor == "" {
					break
				}
				cursor = res.NextCursor
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].UpdatedAt > out[j].UpdatedAt })
	return l.host.FilterAncestors(out, ws), nil
}

func (l *SessionLifecycle) Open(ws, id string, override map[string]json.RawMessage) (Info, error) {
	return l.open(ws, id, override, false)
}

// LoadFullHistory closes the chat if it is open and reopens it with
// session/load, so that the agent replays the whole conversation into a fresh
// saved copy. It is a new conversation with a new log generation, as for a chat
// closed and opened again, because session/load is documented for starting a
// session's replay and not for one that is already active. It refuses while
// the chat is working or switching, since closing would cut the turn
// (docs/plans/session-resume.md).
func (l *SessionLifecycle) LoadFullHistory(ws, id string) (Info, error) {
	id = l.current(id)
	if s := l.conversation(id); s != nil {
		if !layout.SamePath(s.Cwd(), ws) {
			return Info{}, errors.New("that session belongs to another workspace")
		}
		state := s.State()
		if state.Restoring || state.Replaying {
			return Info{}, errors.New("this chat is still being restored")
		}
		if !s.BeginSwitch() {
			return Info{}, errors.New("wait for the provider change to finish before loading the full history")
		}
		if s.Working() {
			s.SetSwitching(false)
			return Info{}, errors.New("wait for this chat to finish its turn before loading the full history")
		}
		// The agent is asked to close the session where it can be, so that the
		// load is not made on a session it still holds active. A close it
		// refuses leaves the chat as it is.
		if conn, init, err := l.host.Live(l.host.NormalizeProvider(s.Provider())); err == nil && init.AgentCapabilities.SessionCapabilities.Close != nil {
			if err := l.host.CallWithin(conn, CloseSessionPatience, "session/close", acp.CloseSessionParams{SessionID: id}, nil); err != nil {
				s.SetSwitching(false)
				return Info{}, fmt.Errorf("the agent did not close this chat's session: %w", err)
			}
		}
		l.Forget(id)
		s.SetSwitching(false)
	}
	return l.open(ws, id, nil, true)
}

// current is the chat to open for one that was asked for: the newest chat that
// continues it, so a window, a warm open or a wake open of a chat that was
// switched to another provider never loads the superseded one. A chat whose
// continuations cannot be resolved is opened as asked.
func (l *SessionLifecycle) current(id string) string {
	if l.host.ResolveContinuation == nil {
		return id
	}
	head, err := l.host.ResolveContinuation(id)
	if err != nil || head == "" || head == id {
		return id
	}
	l.host.Log("info", "chat %s continues as %s, so that chat is opened", id, head)
	return head
}

func (l *SessionLifecycle) open(ws, id string, override map[string]json.RawMessage, full bool) (Info, error) {
	id = l.current(id)
	l.openMu.Lock()
	if call, ok := l.opening[id]; ok {
		l.openMu.Unlock()
		<-call.done
		return call.info, call.err
	}
	if l.opening == nil {
		l.opening = map[string]*openCall{}
	}
	call := &openCall{done: make(chan struct{})}
	l.opening[id] = call
	l.openMu.Unlock()
	defer func() {
		l.openMu.Lock()
		delete(l.opening, id)
		l.openMu.Unlock()
		close(call.done)
	}()
	call.info, call.err = l.openOne(ws, id, override, full)
	return call.info, call.err
}

func (l *SessionLifecycle) openOne(ws, id string, override map[string]json.RawMessage, full bool) (Info, error) {
	info, err := l.openAt(ws, id, override, full)
	if err != nil {
		l.host.Log("error", "chat %s could not be opened in %s: %v", id, ws, err)
	}
	// A chat connected for a message that wakes it may now be open in the
	// background (bus_wake.go).
	l.host.Connected(id, err)
	if s := l.conversation(id); err == nil && s != nil {
		notes := info.Notes
		info = s.InfoVersioned()
		info.Notes = notes
	}
	return info, err
}

func (l *SessionLifecycle) openAt(ws, id string, override map[string]json.RawMessage, full bool) (Info, error) {
	provider, err := l.host.SavedProvider(id)
	if err != nil {
		return Info{}, err
	}
	if ws == "" || !filepath.IsAbs(ws) {
		return Info{}, errors.New("choose an absolute workspace path first")
	}
	if err := l.host.PrepareApplets(provider, ws); err != nil {
		return Info{}, fmt.Errorf("prepare the managed applet integration: %w", err)
	}
	// A chat whose session is being ended is opened once that is done, by
	// loading it again (closed_sessions.go).
	l.host.WaitForEnd(id)
	if s := l.conversation(id); s != nil {
		if !layout.SamePath(s.Cwd(), ws) {
			return Info{}, errors.New("that session belongs to another workspace")
		}

		state := s.State()
		loading := state.Restoring || state.Replaying

		if loading {
			return Info{}, errors.New("this chat is still restoring its provider settings")
		}
		// A chat that closed itself in the background is opened as it is
		// (bus_wake.go).
		l.host.Unpark(s)
		return s.InfoVersioned(), nil
	}
	conn, init, err := l.host.Live(provider)
	if err != nil {
		return Info{}, err
	}
	if !init.AgentCapabilities.LoadSession {
		return Info{}, errors.New("this agent cannot reopen a past session")
	}
	saved, savedTitle := l.host.SavedOptions(id)
	for option, value := range override {
		saved[option] = append(json.RawMessage(nil), value...)
	}
	cwd, title, updated := ws, savedTitle, ""
	// The agent's list is read only for a title the chat does not have yet:
	// listing costs seconds for each chat reopened.
	if title == "" && init.AgentCapabilities.SessionCapabilities.List != nil {
		stop := l.host.WatchReopen(id, "listing the agent's sessions")
		cursor := ""
		for page := 0; page < 5; page++ {
			var list acp.ListSessionsResult
			if err := l.host.CallWithin(conn, 15*time.Second, "session/list", acp.ListSessionsParams{Cwd: ws, Cursor: cursor}, &list); err != nil {
				break
			}
			found := false
			for _, x := range list.Sessions {
				if x.SessionID == id && layout.SamePath(x.Cwd, ws) {
					cwd, updated = x.Cwd, x.UpdatedAt
					if title == "" {
						title = x.Title
					}
					found = true
					break
				}
			}
			if found || list.NextCursor == "" {
				break
			}
			cursor = list.NextCursor
		}
		stop()
	}
	// Register the session before loading, so the replayed updates and any
	// file writes are known to belong to it.
	// An agent that advertises session/resume reopens a chat without replaying
	// it, when Dinah's saved copy is whole. The advertisement is read at each
	// reopen. A chat opened to load its full history always replays.
	tryResume := !full && init.AgentCapabilities.SessionCapabilities.Resume != nil
	why := "the full history was asked for"
	if !full && !tryResume {
		why = "the agent does not advertise session/resume"
	}
	s := NewConversation(Identity{ID: id, Provider: provider, Cwd: cwd, Title: title, UpdatedAt: updated}, InitialActivity{Open: true}, !tryResume, true)
	// The replay announces the chat's subagents again (subagents.go).
	l.host.DropSubagentIndex(id)
	resumed := false
	if tryResume {
		why = l.host.ResumeTranscript(s)
		resumed = why == ""
	}
	if !resumed {
		s.SetReplaying(true)
		l.host.InitTranscript(s, true)
	}
	l.mu.Lock()
	l.runtime.PutConversationLocked(id, s)
	l.mu.Unlock()
	l.host.BeginLog(id, l.host.NormalizeProvider(provider), cwd, title)
	EmitSession(l.host.Emit, s)
	var res acp.LoadSessionResult
	channel, servers, channelErr := l.host.NewChannel(conn, cwd)
	if channelErr == nil && providers.For(provider).ManagedServers {
		servers, channelErr = l.host.ManagedServers()
	}
	busID := l.host.BusSessionID(id)
	if channelErr == nil {
		servers = l.host.WithBusServer(busID, cwd, title, servers)
	}
	if channelErr != nil {
		channel.Revoke()
		err = channelErr
	} else {
		method := "session/load"
		if resumed {
			method = "session/resume"
			l.host.Log("info", "chat %s is reopened with session/resume", id)
		} else {
			l.host.Log("info", "chat %s is reopened with session/load: %s", id, why)
		}
		stop := l.host.WatchReopen(id, method)
		err = l.host.Call(conn, method, l.host.SessionParams(provider, id, cwd, servers), &res)
		stop()
		// A resume the agent refuses falls back to a load once, unless the
		// refusal is that it has no such session, which a load would meet too.
		if resumed && err != nil && !notFound(err) {
			l.host.Log("warning", "session/resume of chat %s was refused (%v), so it is loaded instead", id, err)
			why = "session/resume was refused: " + err.Error()
			resumed = false
			s.SetReplaying(true)
			l.host.InitTranscript(s, true)
			stop = l.host.WatchReopen(id, "session/load")
			err = l.host.Call(conn, "session/load", l.host.SessionParams(provider, id, cwd, servers), &res)
			stop()
		}
		if err == nil {
			err = channel.Bind(id)
		}
		if err != nil {
			channel.Revoke()
		}
	}
	// What the replay left unsent goes before anything new, a user message
	// it ended on among it (delivered.go).
	// A resume has no replay to settle, flush, end or compare with the copy.
	var applied []AppliedSubstitution
	if !resumed {
		l.host.SettleReplayedPrompt(s)
		l.host.FlushReplay(s)
		s.EndReplay()
		// The RFD has the agent end every subagent its history leaves running
		// with disconnected. One it leaves running all the same has no turn to
		// run in, so its outcome is unknown, as after a turn (subagents.go).
		l.host.DisconnectSubagents(s)
		stop := l.host.WatchReopen(id, "checking the replay against the saved copy")
		l.host.FinishTranscriptReplay(s, err == nil)
		stop()
	}
	if err == nil {
		s.SetResumed(resumed)
		stop := l.host.WatchReopen(id, "restoring the chat's settings")
		var opts []acp.ConfigOption
		opts, applied, err = l.host.RestoreOptions(conn, id, res.ConfigOptions, saved, l.host.Reopening(provider))
		if err == nil {
			if len(applied) > 0 {
				overrides := map[string]OptionOverride{}
				for _, sub := range applied {
					overrides[sub.Option] = OptionOverride{From: sub.From, To: sub.To}
				}
				s.SetOverrides(overrides)
			}
			err = l.host.FinishOptions(s, opts)
		}
		stop()
	}
	if err != nil {
		channel.Revoke()
		l.host.EndBusSession(busID)
		l.mu.Lock()
		l.runtime.RemoveConversationLocked(id)
		l.mu.Unlock()
		s.Close()
		EmitSession(l.host.Emit, s)
		return Info{}, err
	}
	l.host.JoinBus(s, busID)
	if l.host.RestoreQueue != nil {
		l.host.RestoreQueue(s)
	}
	if l.host.AfterOpen != nil {
		l.host.AfterOpen(s)
	}
	if l.host.StartWaiting != nil {
		l.host.StartWaiting(s)
	}
	info := s.InfoVersioned()
	EmitInfo(l.host.Emit, info)
	// A substitution the person stored is said in the chat, and kept in its
	// log, each time it is made.
	for _, sub := range applied {
		l.host.NoteLog(id, sub.Note())
		info.Notes = append(info.Notes, sub.Note())
	}
	return info, nil
}

// notFound says the agent answered that the session does not exist.
func notFound(err error) bool {
	var e *acp.Error
	return errors.As(err, &e) && e.Code == acp.CodeResourceNotFound
}

var ErrNoDelete = errors.New("this agent does not support removing sessions")

func (l *SessionLifecycle) Delete(id string) error {
	if s := l.conversation(id); s != nil {
		if !s.BeginSwitch() {
			return errors.New("wait for the provider change to finish before deleting this chat")
		}

		defer func() { s.SetSwitching(false) }()
	}
	provider, err := l.host.SavedProvider(id)
	if err != nil {
		return err
	}
	conn, init, err := l.host.Live(provider)
	if err != nil {
		return err
	}
	if init.AgentCapabilities.SessionCapabilities.Delete == nil {
		return ErrNoDelete
	}
	if s := l.conversation(id); s != nil {
		_ = l.host.Cancel(s)
	}
	if err := l.host.Call(conn, "session/delete", acp.DeleteSessionParams{SessionID: id}, nil); err != nil {
		return err
	}
	if err := l.host.Tombstone(id); err != nil {
		return fmt.Errorf("the provider deleted this chat, but its local history could not be marked deleted: %w", err)
	}
	if l.host.ForgetTeam != nil {
		l.host.ForgetTeam(id)
	}
	if s := l.conversation(id); s != nil {
		s.Close()
	}
	l.Forget(id)
	l.host.ForgetOptions(id)
	return nil
}

// Forget lets a chat go from the window. A turn still running is cancelled
// first: hiding a chat the provider cannot delete (Ollama's) came here
// without stopping it, and the turn went on with nobody to see it or to stop
// it, running tools the person had allowed, until it ended by itself.
func (l *SessionLifecycle) Forget(id string) {
	if s := l.conversation(id); s != nil {
		if l.host.Cancel != nil && s.State().Working {
			_ = l.host.Cancel(s)
		}
		l.host.LeaveBus(s)
	}
	l.host.RevokeSession(id)
	l.host.ForgetJournal(id)
	l.host.DropSubagentIndex(id)
	l.mu.Lock()
	l.runtime.RemoveConversationLocked(id)
	l.mu.Unlock()
	l.host.ForgetTitle(id)
	l.host.CancelQuestions(id)
}
