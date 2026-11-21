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
	BeginAllocation func(AllocationIntent) (AllocationCallbacks, error)
	Allocate        func(Connection, any, func(string) error) (acp.NewSessionResult, error)
	// RecordListedSessions durably retains native identities before they are
	// returned to a caller. A page is an observation, never proof of absence.
	RecordListedSessions func(string, string, []acp.SessionInfo) error
	BeginListing         func(string, string) (uint64, error)
	FinishListing        func(string, string, uint64, []acp.SessionInfo, bool) error
	RecordedActivity     func(string) string
	BeforeOpen           func(string, string) (Info, bool, error)
	MissingSession       func(string, string, error) (Info, error)
	NormalizeProvider    func(string) string
	PrepareApplets       func(string, string) error
	Live                 func(string) (Connection, acp.InitializeResult, error)
	Defaults             func(string) map[string]json.RawMessage
	SavedOptions         func(string) (map[string]json.RawMessage, string)
	NewChannel           func(Connection, string) (ChannelCapability, []any, error)
	ManagedServers       func() ([]any, error)
	// ConfigureServers adds shell-owned session capabilities after defaults.
	// The capability retains the authority and revokes it when opening fails.
	ConfigureServers func(ChannelCapability, string, string, []any) ([]any, error)
	BusSessionID     func(string) string
	WithBusServer    func(string, string, string, []any) []any
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
	// BeforeLoad is called once before the agent is asked to load or resume a
	// saved chat, for anything the host must undo at the agent first. It
	// returns nothing: a host that fails here logs it, and the load is asked
	// for all the same. It may be nil.
	BeforeLoad func(conn Connection, provider, id string)
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

type AllocationIntent struct {
	Source, Workspace, Provider, Recipient, Kind, Revision string
}

type AllocationCallbacks struct {
	Dispatched func() error
	Received   func(string) error
	Finished   func(string, error) error
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

	// LoadLimit replaces ReopenLoadLimit when it is positive, which a test
	// uses to meet the limit quickly. It is set before the first open.
	LoadLimit time.Duration
	// outstanding has, by chat, the connection that still owes an answer to
	// a load given up at the limit (boundedLoad).
	loadMu      sync.Mutex
	outstanding map[string]Connection
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
	return l.newSession(ws, provider, continuing, "", AllocationIntent{Kind: "new_chat"}, nil)
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
	return l.newSession(ws, provider, true, adopt, AllocationIntent{Kind: "new_chat"}, nil)
}

func (l *SessionLifecycle) NewForTransfer(source, ws, provider, adopt, revision string) (Info, error) {
	return l.newSession(ws, provider, true, adopt, AllocationIntent{Source: source, Kind: "switch", Revision: revision}, nil)
}

// NewRecorded starts a fresh session for an already durable operation intent.
// Source is an opaque operation reference, not necessarily a native chat.
func (l *SessionLifecycle) NewRecorded(intent AllocationIntent, callbacks AllocationCallbacks) (Info, error) {
	if intent.Source == "" || intent.Kind == "" || intent.Recipient != "" || callbacks.Dispatched == nil || callbacks.Received == nil || callbacks.Finished == nil {
		return Info{}, &AllocationNotCreatedError{Err: errors.New("recorded allocation requires an operation source, kind, fresh recipient and all receipt callbacks")}
	}
	return l.newSession(intent.Workspace, intent.Provider, false, "", intent, &callbacks)
}

// combineAllocationCallbacks preserves every receipt observer even when another
// observer fails. The native request remains blocked by any dispatch error.
func combineAllocationCallbacks(first, second AllocationCallbacks) AllocationCallbacks {
	return AllocationCallbacks{
		Dispatched: func() error {
			var a, b error
			if first.Dispatched != nil {
				a = first.Dispatched()
			}
			if second.Dispatched != nil {
				b = second.Dispatched()
			}
			return errors.Join(a, b)
		},
		Received: func(id string) error {
			var a, b error
			if first.Received != nil {
				a = first.Received(id)
			}
			if second.Received != nil {
				b = second.Received(id)
			}
			return errors.Join(a, b)
		},
		Finished: func(id string, outcome error) error {
			var a, b error
			if first.Finished != nil {
				a = first.Finished(id, outcome)
			}
			if second.Finished != nil {
				b = second.Finished(id, outcome)
			}
			return errors.Join(a, b)
		},
	}
}

// AllocationNotCreatedError marks a failure before dispatch or method-not-found
// for session/new without a native receipt. Transport loss and post-allocation errors do not
// carry this marker, even when a later operation returns a JSON-RPC error.
type AllocationNotCreatedError struct{ Err error }

func (e *AllocationNotCreatedError) Error() string { return e.Err.Error() }
func (e *AllocationNotCreatedError) Unwrap() error { return e.Err }

func (l *SessionLifecycle) newSession(ws, provider string, continuing bool, adopt string, intent AllocationIntent, recorded *AllocationCallbacks) (info Info, resultErr error) {
	allocationMayExist := false
	receipt := ""
	callbacks, finalized := AllocationCallbacks{}, false
	if recorded != nil {
		callbacks = *recorded
	}
	defer func() {
		if resultErr != nil && receipt != "" {
			info = Info{ID: receipt, Provider: provider, Cwd: ws}
			resultErr = &acp.SessionAllocationError{SessionID: receipt, Err: resultErr}
		}
		if resultErr != nil && !allocationMayExist {
			resultErr = &AllocationNotCreatedError{Err: resultErr}
		}
		if !finalized && callbacks.Finished != nil {
			if err := callbacks.Finished(receipt, resultErr); err != nil {
				resultErr = errors.Join(resultErr, err)
				if receipt != "" {
					info = Info{ID: receipt, Provider: provider, Cwd: ws}
					resultErr = &acp.SessionAllocationError{SessionID: receipt, Err: resultErr}
				}
			}
		}
	}()
	provider = l.host.NormalizeProvider(provider)
	if ws == "" || !filepath.IsAbs(ws) {
		return Info{}, errors.New("choose an absolute workspace path first")
	}
	intent.Workspace, intent.Provider, intent.Recipient = ws, provider, adopt
	if l.host.BeginAllocation != nil {
		var err error
		var global AllocationCallbacks
		global, err = l.host.BeginAllocation(intent)
		if err != nil {
			return Info{}, err
		}
		callbacks = combineAllocationCallbacks(global, callbacks)
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
	if l.host.ConfigureServers != nil {
		servers, err = l.host.ConfigureServers(channel, provider, ws, servers)
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
	if callbacks.Dispatched != nil {
		if err := callbacks.Dispatched(); err != nil {
			return Info{}, err
		}
	}
	allocationMayExist = true
	received := func(id string) error {
		if id == "" {
			return errors.New("the provider returned an empty allocation receipt")
		}
		if receipt != "" && receipt != id {
			return errors.New("the provider returned conflicting allocation receipts")
		}
		receipt = id
		if callbacks.Received != nil {
			return callbacks.Received(id)
		}
		return nil
	}
	params := l.host.SessionParams(provider, "", ws, servers)
	if l.host.Allocate != nil {
		ns, err = l.host.Allocate(conn, params, received)
	} else {
		err = l.host.Call(conn, "session/new", params, &ns)
		if err == nil {
			err = received(ns.SessionID)
		}
	}
	if err != nil {
		var known *acp.SessionAllocationError
		if errors.As(err, &known) && receipt == "" {
			receipt = known.SessionID
		}
		var rejection *acp.Error
		if receipt == "" && errors.As(err, &rejection) && rejection.Code == acp.CodeMethodNotFound {
			allocationMayExist = false
		}
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
	opts, applied, err := l.host.RestoreOptions(conn, s.ID(), ns.ConfigOptions, defaults, nil)
	if err == nil {
		err = l.host.FinishOptions(s, opts)
	}
	// A default the agent did not answer for in time leaves the new chat with
	// the agent's own value, which the chat's settings then show. The page has
	// no place for a note on a chat it has just made, so the line goes to the
	// chat's log and to Dinah's log.
	if err == nil {
		for _, sub := range applied {
			if sub.Unanswered() && sub.Note() != "" {
				l.host.Log("warning", "chat %s was started with settings unrestored: %s", s.ID(), sub.Note())
				if l.host.NoteLog != nil {
					l.host.NoteLog(s.ID(), sub.Note())
				}
			}
		}
	}
	if err != nil {
		l.mu.Lock()
		l.runtime.RemoveConversationLocked(s.ID())
		l.mu.Unlock()
		s.Close()
		return Info{}, err
	}
	if callbacks.Finished != nil {
		finalized = true
		if err := callbacks.Finished(s.ID(), nil); err != nil {
			l.mu.Lock()
			l.runtime.RemoveConversationLocked(s.ID())
			l.mu.Unlock()
			s.Close()
			return Info{}, err
		}
	}
	bound = true
	if adopt == "" {
		l.host.JoinBus(s, busID)
	}
	info = s.InfoVersioned()
	if !continuing {
		EmitInfo(l.host.Emit, info)
	}
	return info, nil
}

func (l *SessionLifecycle) List(ws string) ([]Info, error) {
	return l.listProviders(ws, l.host.ListProviders())
}

// ListProvider refreshes one explicitly used program without touching others.
func (l *SessionLifecycle) ListProvider(ws, provider string) ([]Info, error) {
	return l.listProviders(ws, []string{provider})
}

func (l *SessionLifecycle) listProviders(ws string, providers []string) ([]Info, error) {
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
	for _, provider := range providers {
		conn, init, err := l.host.Live(provider)
		if err == nil && init.AgentCapabilities.SessionCapabilities.List != nil {
			var sequence uint64
			if l.host.BeginListing != nil {
				sequence, err = l.host.BeginListing(provider, ws)
				if err != nil {
					return nil, err
				}
			}
			observed := []acp.SessionInfo{}
			complete := false
			cursors := map[string]bool{}
			cursor := ""
			for page := 0; page < 5; page++ {
				var res acp.ListSessionsResult
				if err := l.host.CallWithin(conn, 15*time.Second, "session/list", acp.ListSessionsParams{Cwd: ws, Cursor: cursor}, &res); err != nil {
					break
				}
				observed = append(observed, res.Sessions...)
				if l.host.FinishListing == nil && l.host.RecordListedSessions != nil {
					if err := l.host.RecordListedSessions(provider, ws, res.Sessions); err != nil {
						return nil, err
					}
				}
				for _, x := range res.Sessions {
					hide := l.host.Hidden(x.SessionID)
					if !seen[x.SessionID] && !hide && layout.SamePath(x.Cwd, ws) {
						out = append(out, Info{ID: x.SessionID, Provider: provider, Cwd: x.Cwd, Title: l.host.Title(x.SessionID, x.Title), UpdatedAt: x.UpdatedAt})
						seen[x.SessionID] = true
					}
				}
				if res.NextCursor == "" {
					complete = true
					break
				}
				if cursors[res.NextCursor] {
					break
				}
				cursors[res.NextCursor] = true
				cursor = res.NextCursor
			}
			if l.host.FinishListing != nil {
				if err := l.host.FinishListing(provider, ws, sequence, observed, complete); err != nil {
					return nil, err
				}
			}
		}
	}
	if l.host.RecordedActivity != nil {
		for i := range out {
			out[i].UpdatedAt = LatestActivity(out[i].UpdatedAt, l.host.RecordedActivity(out[i].ID))
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		left, _ := time.Parse(time.RFC3339Nano, out[i].UpdatedAt)
		right, _ := time.Parse(time.RFC3339Nano, out[j].UpdatedAt)
		return left.After(right)
	})
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
	return l.reopen(ws, id, nil, "loading the full history")
}

// ReopenWith closes the chat if it is open and opens it again with the
// option values given in place of its saved ones, as Load Full History closes
// and opens it. It is for a choice that an agent takes only when a session is
// made, which a session already open cannot be given. It refuses while the
// chat is working or switching, for the same reason.
func (l *SessionLifecycle) ReopenWith(ws, id string, override map[string]json.RawMessage) (Info, error) {
	return l.reopen(ws, id, override, "opening it again")
}

// reopen is the closing and opening both share; what says, in an error,
// what the chat was to be closed for.
func (l *SessionLifecycle) reopen(ws, id string, override map[string]json.RawMessage, what string) (Info, error) {
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
			return Info{}, errors.New("wait for the provider change to finish before " + what)
		}
		if s.Working() {
			s.SetSwitching(false)
			return Info{}, errors.New("wait for this chat to finish its turn before " + what)
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
	return l.open(ws, id, override, true)
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
	return l.openExact(ws, id, override, full)
}

// OpenExact opens the target already resolved and checked by a delivery attempt.
// It must not redirect the attempt to another recipient during the open.
func (l *SessionLifecycle) OpenExact(ws, id string) (Info, error) {
	return l.openExact(ws, id, nil, false)
}

func (l *SessionLifecycle) openExact(ws, id string, override map[string]json.RawMessage, full bool) (Info, error) {
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
	if l.host.BeforeOpen != nil {
		if info, handled, err := l.host.BeforeOpen(ws, id); handled || err != nil {
			call.info, call.err = info, err
			return call.info, call.err
		}
	}
	call.info, call.err = l.openOne(ws, id, override, full)
	if call.err != nil && notFound(call.err) && l.host.MissingSession != nil {
		call.info, call.err = l.host.MissingSession(ws, id, call.err)
	}
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
	// A load given up at the limit is still the agent's to answer. Until it
	// answers, or its process ends, the chat is not asked for again, since
	// two loads of one session at once would mix their replays.
	if l.loadOutstanding(conn, id) {
		return Info{}, ErrLoadOutstanding
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
	if l.host.RecordedActivity != nil {
		updated = LatestActivity(updated, l.host.RecordedActivity(id))
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
		s.SetReplayToPage(full)
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
	if channelErr == nil && l.host.ConfigureServers != nil {
		servers, channelErr = l.host.ConfigureServers(channel, provider, cwd, servers)
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
		closable := init.AgentCapabilities.SessionCapabilities.Close != nil
		if l.host.BeforeLoad != nil {
			l.host.BeforeLoad(conn, provider, id)
		}
		stop := l.host.WatchReopen(id, method)
		err = l.boundedLoad(conn, id, method, l.host.SessionParams(provider, id, cwd, servers), &res, closable)
		stop()
		// A resume the agent refuses falls back to a load once, unless the
		// refusal is that it has no such session, which a load would meet too.
		// A resume given up at the limit is no refusal, and its request is
		// still the agent's to answer, so no load follows it.
		if resumed && err != nil && !notFound(err) && !IsLoadTimeout(err) {
			l.host.Log("warning", "session/resume of chat %s was refused (%v), so it is loaded instead", id, err)
			why = "session/resume was refused: " + err.Error()
			resumed = false
			s.SetReplaying(true)
			s.SetReplayToPage(full)
			l.host.InitTranscript(s, true)
			stop = l.host.WatchReopen(id, "session/load")
			err = l.boundedLoad(conn, id, "session/load", l.host.SessionParams(provider, id, cwd, servers), &res, closable)
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
	// A setting the agent did not answer for in time is said the same way,
	// in one line for all such settings (Options.Restore), and in Dinah's log.
	for _, sub := range applied {
		if sub.Note() == "" {
			continue
		}
		if sub.Unanswered() {
			l.host.Log("warning", "chat %s was opened with settings unrestored: %s", id, sub.Note())
		}
		l.host.NoteLog(id, sub.Note())
		info.Notes = append(info.Notes, sub.Note())
	}
	return info, nil
}

// ReopenLoadLimit is how long the agent is given to answer the request that
// reopens a chat (session/load or session/resume) before the open fails and
// the chat stays closed. On 21 November 2026 one chat's load was never
// answered, and every caller of the open waited with it, without end.
//
// The limit has two sides. It is longer than any healthy load seen: the
// longest in that day's logs, a history of 56 MiB, answered in well under a
// minute, and the reopen watch (WatchReopen) writes its line and saves the
// stacks at one minute, before the limit ends the wait. It is shorter than
// bushost.ConnectPatience (two minutes) less the grace the page is given to
// open a chat first (five seconds), because the wake scheduler forgets a
// connect it asked for after that patience, and a failure that arrived later
// would not be reported to the chat's team or to the person. The requests
// that restore the chat's settings come after the load and have a limit of
// their own for all of them together (RestoreSettingsLimit), so the two
// limits and the grace have to fit the patience as a sum.
// TestTheReopenLimitFitsTheWakePatience keeps that relation. The limit is a
// constant in code, since a wrong value can only be found by reading logs. A
// chat whose healthy load takes longer than this needs the limit and
// ConnectPatience raised together.
const ReopenLoadLimit = 90 * time.Second

// LoadTimeoutError is what an open fails with when the agent did not answer
// the load within the limit.
type LoadTimeoutError struct {
	Method string
	Limit  time.Duration
}

func (e *LoadTimeoutError) Error() string {
	within := e.Limit.String()
	if e.Limit >= time.Second {
		within = fmt.Sprintf("%d seconds", int(e.Limit/time.Second))
	}
	return fmt.Sprintf("The agent did not answer the request to load this chat (%s) within %s, so the chat stays closed. Its saved history and any messages waiting for it are kept.", e.Method, within)
}

// ErrorDetail gives the page the kind of failure, so that its note does not
// say the agent reported it.
func (e *LoadTimeoutError) ErrorDetail() any {
	return map[string]any{"kind": "open-timeout", "method": e.Method, "seconds": int(e.Limit / time.Second)}
}

// IsLoadTimeout reports whether an open failed at the load limit.
func IsLoadTimeout(err error) bool {
	var e *LoadTimeoutError
	return errors.As(err, &e)
}

// ErrLoadOutstanding is what an open fails with while the agent still owes
// an answer to an earlier load of the same chat that was given up.
var ErrLoadOutstanding = errors.New("The agent has not yet answered the earlier request to load this chat, so it is not asked again while that request is open. Try again later, or restart the agent to end the request.")

func (l *SessionLifecycle) loadLimit() time.Duration {
	if l.LoadLimit > 0 {
		return l.LoadLimit
	}
	return ReopenLoadLimit
}

// loadOutstanding reports whether conn still owes an answer to a load of the
// chat that was given up. A connection that has ended owes nothing.
func (l *SessionLifecycle) loadOutstanding(conn Connection, id string) bool {
	l.loadMu.Lock()
	owing, ok := l.outstanding[id]
	l.loadMu.Unlock()
	if !ok || owing != conn {
		return false
	}
	select {
	case <-conn.Done():
		return false
	default:
		return true
	}
}

// LoadOutstanding reports whether a connection still owes an answer to a
// load of the chat that was given up. The handler drops the session updates
// of such a chat, which are the replay of the load nobody waits for.
func (l *SessionLifecycle) LoadOutstanding(id string) bool {
	l.loadMu.Lock()
	conn, ok := l.outstanding[id]
	l.loadMu.Unlock()
	return ok && l.loadOutstanding(conn, id)
}

// boundedLoad makes the request that reopens a chat and waits for its answer
// for the load limit at most. At the limit it returns a LoadTimeoutError and
// the caller closes the chat as for any failed open.
//
// The request itself is not withdrawn, because the protocol has no request
// that cancels a load, and the agent's process is left running, because it
// serves the other chats of its provider. The request stays open on the
// connection, so its answer is matched to it and to nothing else when it
// arrives, however late. The chat is marked as owed an answer until then,
// which keeps a second load from being sent beside the first (openAt) and
// drops the replay the first still sends (Handler.Notify). A late answer is
// logged and its result is discarded. Where the agent advertises
// session/close, the session a late answer left loaded is closed, as a chat
// is closed before it is loaded again (reopen). The mark also ends when the
// connection does, which fails the request.
func (l *SessionLifecycle) boundedLoad(conn Connection, id, method string, params any, res *acp.LoadSessionResult, closable bool) error {
	var out acp.LoadSessionResult
	done := make(chan error, 1)
	began := time.Now()
	go func() { done <- l.host.Call(conn, method, params, &out) }()
	limit := l.loadLimit()
	timer := time.NewTimer(limit)
	defer timer.Stop()
	select {
	case err := <-done:
		if err == nil {
			*res = out
		}
		return err
	case <-timer.C:
	}
	l.loadMu.Lock()
	if l.outstanding == nil {
		l.outstanding = map[string]Connection{}
	}
	l.outstanding[id] = conn
	l.loadMu.Unlock()
	l.host.Log("error", "chat %s: %s was not answered within %s, so the open fails and the chat stays closed; the agent is left running", id, method, limit)
	go func() {
		err := <-done
		waited := time.Since(began).Round(time.Second)
		if err != nil {
			l.host.Log("info", "chat %s: the %s given up earlier ended after %s: %v", id, method, waited, err)
		} else {
			l.host.Log("warning", "chat %s: the %s given up earlier was answered after %s, and the answer is discarded", id, method, waited)
			if closable {
				if err := l.host.CallWithin(conn, CloseSessionPatience, "session/close", acp.CloseSessionParams{SessionID: id}, nil); err != nil {
					l.host.Log("warning", "chat %s: the session its late answer left loaded could not be closed: %v", id, err)
				}
			}
		}
		l.loadMu.Lock()
		if l.outstanding[id] == conn {
			delete(l.outstanding, id)
		}
		l.loadMu.Unlock()
	}()
	return &LoadTimeoutError{Method: method, Limit: limit}
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
