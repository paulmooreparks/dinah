package sessionhost

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"studio/internal/acp"
	"studio/internal/profile"
)

//go:embed lifecycle.go
var LifecycleSource []byte

// ProviderSpec describes provider policy supplied by the shell. Prepare and
// Ready run under connect serialization and must not re-enter Live or Restart.
type ProviderSpec struct {
	ID, Starting, Stopped string
	ReadyDetail           func(acp.InitializeResult) string
	InitWithin            time.Duration
	InitError             string
	Traces                bool
	Prepare               func() (ProviderStart, error)
}
type ProviderStart struct {
	Launch func(acp.Handler) (Connection, error)
	Ready  func(acp.InitializeResult)
}

// LifecycleHost supplies effects outside the state and conversation locks.
// Callbacks reached during Live must not re-enter Live or Restart.
type LifecycleHost struct {
	Handler             func() acp.Handler
	TraceForProvider    func(string) Trace
	RedactTrace         func(string) string
	RevokeChannels      func(Connection)
	CloseChatLog        func(string)
	CancelQuestions     func(string)
	DisconnectSubagents func(*Conversation)
	Emit                func(string, any)
	ClientVersion       string
}

// ProviderLifecycle shares the host state mutex and sole Runtime registry.
// Construct it once per host; it and its referenced owners must never be copied.
// Stop deliberately does not acquire a start lock, retaining concurrent stop behavior.
type ProviderLifecycle struct {
	// startsMu guards starts, the lock of each provider's start. Starting a
	// provider takes seconds, so the lock is the provider's own: a chat on a
	// running provider, and a chat that needs another provider, never wait for a
	// start that is not theirs.
	startsMu sync.Mutex
	starts   map[string]*sync.Mutex
	mu       *sync.Mutex
	runtime  *Runtime
	specs    map[string]ProviderSpec
	host     LifecycleHost

	// stateMu guards states, the last state told to the page for each
	// provider, so that a page that missed an agent:state event can read it
	// (States). It is a leaf: nothing is acquired while it is held.
	stateMu sync.Mutex
	states  map[string]ProviderState
	// clock numbers the snapshots of states, taken under stateMu
	// (state_events.go).
	clock *StateClock
}

// ProviderState is a provider process's state, as the agent:state event
// carries it.
type ProviderState struct {
	Provider string `json:"provider"`
	State    string `json:"state"`
	Detail   string `json:"detail"`
	// StateGen and StateVer order the snapshots of this state the page is
	// given (state_events.go).
	StateGen string `json:"stateGen,omitempty"`
	StateVer uint64 `json:"stateVer,omitempty"`
}

// States returns the last state told to the page for each provider that has
// had one, in the order of the provider's name. A provider that has never
// started has none.
func (l *ProviderLifecycle) States() []ProviderState {
	l.stateMu.Lock()
	defer l.stateMu.Unlock()
	out := make([]ProviderState, 0, len(l.states))
	for provider := range l.states {
		out = append(out, l.snapshotLocked(provider))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Provider < out[j].Provider })
	return out
}

func NewProviderLifecycle(mu *sync.Mutex, runtime *Runtime, specs []ProviderSpec, host LifecycleHost) *ProviderLifecycle {
	l := &ProviderLifecycle{mu: mu, runtime: runtime, specs: make(map[string]ProviderSpec), host: host, states: make(map[string]ProviderState), clock: NewStateClock()}
	for _, spec := range specs {
		l.specs[spec.ID] = spec
	}
	return l
}
func normalizeProvider(id string) string {
	return profile.ProviderOrLegacy(id)
}

// state tells the page a provider's state.
func (l *ProviderLifecycle) state(provider, state, detail string) {
	// Stored before it is emitted, so a page that sees the event and then
	// reads the states never reads an older one.
	// The version is taken in the same acquisition, so that events that reach
	// the page out of order are put back by it (state_events.go).
	l.stateMu.Lock()
	l.states[provider] = ProviderState{Provider: provider, State: state, Detail: detail}
	snapshot := l.snapshotLocked(provider)
	l.stateMu.Unlock()
	EmitProviderState(l.host.Emit, snapshot)
}

// snapshotLocked is a provider's stored state with the next version. The
// caller holds stateMu.
func (l *ProviderLifecycle) snapshotLocked(provider string) ProviderState {
	s := l.states[provider]
	s.StateGen, s.StateVer = l.clock.Next()
	return s
}

// Live returns a provider's running connection, starting the
// provider when it is not running. A provider whose process has ended is
// waited for, so that its chats are let go before the new one starts.
func (l *ProviderLifecycle) Live(provider string) (Connection, acp.InitializeResult, error) {
	provider = normalizeProvider(provider)
	adapter, found := l.specs[provider]
	if !found {
		return nil, acp.InitializeResult{}, errors.New("that agent is not supported yet")
	}
	// The lock is the provider's own, so a chat waits only for a start of the provider
	// it needs, including the hook that runs when that provider is ready, and never
	// for another provider's.
	lock := l.startLock(provider)
	lock.Lock()
	defer lock.Unlock()
	// Another chat may have started it while this one waited for the lock, in which
	// case that one start is reused and the provider is not started twice.
	if conn, init, ok := l.running(provider); ok {
		return conn, init, nil
	}
	l.mu.Lock()
	p := l.runtime.ProviderLocked(provider)
	l.mu.Unlock()
	if p != nil {
		// A provider whose process has ended is waited for, so that its chats are let
		// go before the new one starts.
		p.Wait()
	}
	return l.start(&adapter)
}

// running is a provider's connection when its process is running.
func (l *ProviderLifecycle) running(provider string) (Connection, acp.InitializeResult, bool) {
	l.mu.Lock()
	p := l.runtime.ProviderLocked(provider)
	l.mu.Unlock()
	if p == nil {
		return nil, acp.InitializeResult{}, false
	}
	select {
	case <-p.Connection().Done():
		return nil, acp.InitializeResult{}, false
	default:
		return p.Connection(), p.Initialization(), true
	}
}

// startLock is a provider's own start lock.
func (l *ProviderLifecycle) startLock(provider string) *sync.Mutex {
	l.startsMu.Lock()
	defer l.startsMu.Unlock()
	if l.starts == nil {
		l.starts = map[string]*sync.Mutex{}
	}
	if l.starts[provider] == nil {
		l.starts[provider] = &sync.Mutex{}
	}
	return l.starts[provider]
}

// start starts a provider: prepares it, launches it, initializes
// it, records it and watches for its end. The caller holds the provider's start lock.
func (l *ProviderLifecycle) start(adapter *ProviderSpec) (Connection, acp.InitializeResult, error) {
	start, err := adapter.Prepare()
	if err != nil {
		return nil, acp.InitializeResult{}, err
	}
	l.state(adapter.ID, "starting", adapter.Starting)
	conn, err := start.Launch(l.host.Handler())
	if err != nil {
		l.state(adapter.ID, "failed", err.Error())
		return nil, acp.InitializeResult{}, err
	}
	conn.RegisterTraceRedactor(l.host.RedactTrace)
	ctx, cancel := context.WithTimeout(context.Background(), adapter.InitWithin)
	defer cancel()
	var init acp.InitializeResult
	err = conn.Call(ctx, "initialize", acp.InitializeParams{
		ProtocolVersion:    1,
		ClientCapabilities: acp.ClientCaps(),
		ClientInfo:         &acp.Implementation{Name: "dinah", Title: "Dinah", Version: l.host.ClientVersion},
	}, &init)
	if err != nil {
		conn.Close()
		if adapter.InitError != "" {
			err = fmt.Errorf("%s: %w", adapter.InitError, err)
		}
		l.state(adapter.ID, "failed", err.Error())
		return nil, acp.InitializeResult{}, err
	}
	var trace Trace
	if adapter.Traces {
		// The trace is the app's for its whole life; a stop writes what it
		// holds.
		trace = l.host.TraceForProvider(adapter.ID)
	}
	p := NewProvider(conn, init, trace)
	l.mu.Lock()
	l.runtime.PutProviderLocked(adapter.ID, p)
	l.mu.Unlock()
	go l.Watch(adapter.ID, p)
	l.state(adapter.ID, "ready", adapter.ReadyDetail(init))
	if start.Ready != nil {
		start.Ready(init)
	}
	return conn, init, nil
}

// Watch waits for a provider's process to end and lets its chats
// go: each loses its session, its log is closed, its open questions are
// cancelled and its subagents end with it, their outcome unknown
// (subagents.go). Finish releases completion waiters after those effects.
func (l *ProviderLifecycle) Watch(provider string, p *Provider) {
	defer p.Finish()
	<-p.Connection().Done()
	l.host.RevokeChannels(p.Connection())
	l.mu.Lock()
	ended := l.runtime.DetachProviderLocked(provider, p)
	l.mu.Unlock()
	for _, s := range ended {
		l.host.CloseChatLog(s.ID())
		l.host.CancelQuestions(s.ID())
		dropped := s.ProviderStopped()
		for _, prompt := range dropped {
			prompt.Resolve(End{StopReason: "error", Err: errors.New("the chat's agent stopped before the question's turn")})
		}
		l.host.DisconnectSubagents(s)
		EmitSession(l.host.Emit, s)
	}
	detail := provider + " stopped"
	if adapter, found := l.specs[provider]; found {
		detail = adapter.Stopped
	}
	if e := p.Connection().Err(); e != nil && !strings.Contains(e.Error(), "killed") {
		detail += ": " + e.Error()
	}
	l.state(provider, "stopped", detail)
}

func (l *ProviderLifecycle) Connect(provider string) error { _, _, err := l.Live(provider); return err }

// Stop ends a provider's process and with it every chat in it. The
// chats stay in the provider's history and can be opened again.
func (l *ProviderLifecycle) Stop(provider string) {
	l.mu.Lock()
	p := l.runtime.ProviderLocked(normalizeProvider(provider))
	l.mu.Unlock()
	if p == nil {
		return
	}
	p.Connection().Close()
	p.Wait()
	p.Flush()
}

func (l *ProviderLifecycle) Restart(provider string) error {
	l.Stop(provider)
	return l.Connect(provider)
}

func (l *ProviderLifecycle) StopAll() {
	l.mu.Lock()
	var ids []string
	for _, id := range l.runtime.ProviderIDsLocked() {
		ids = append(ids, id)
	}
	l.mu.Unlock()
	for _, id := range ids {
		l.Stop(id)
	}
}
