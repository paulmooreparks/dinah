package sessionhost

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"

	"studio/internal/acp"
)

type lifecycleConnection struct {
	done       chan struct{}
	once       sync.Once
	initialize func(context.Context) error
}

func (c *lifecycleConnection) Call(ctx context.Context, method string, in, out any) error {
	return c.initialize(ctx)
}
func (*lifecycleConnection) Notify(string, any) error                  { return nil }
func (c *lifecycleConnection) Done() <-chan struct{}                   { return c.done }
func (*lifecycleConnection) Err() error                                { return nil }
func (c *lifecycleConnection) Close()                                  { c.once.Do(func() { close(c.done) }) }
func (*lifecycleConnection) RegisterTraceSecret(string)                {}
func (*lifecycleConnection) RegisterTraceRedactor(func(string) string) {}

type lifecycleTrace struct{ flush func() }

func (f lifecycleTrace) Flush() error { f.flush(); return nil }

func lifecycleFixture() (*ProviderLifecycle, *lifecycleConnection) {
	c := &lifecycleConnection{done: make(chan struct{}), initialize: func(context.Context) error { return nil }}
	h := LifecycleHost{
		Handler: func() acp.Handler { return nil }, RedactTrace: func(s string) string { return s },
		RevokeChannels: func(Connection) {}, CloseChatLog: func(string) {}, CancelQuestions: func(string) {},
		DisconnectSubagents: func(*Conversation) {}, Emit: func(string, any) {},
	}
	spec := ProviderSpec{ID: "codex", Starting: "starting", Stopped: "stopped", InitWithin: time.Second, ReadyDetail: func(acp.InitializeResult) string { return "ready" }, Prepare: func() (ProviderStart, error) {
		return ProviderStart{Launch: func(acp.Handler) (Connection, error) { return c, nil }}, nil
	}}
	return NewProviderLifecycle(&sync.Mutex{}, &Runtime{}, []ProviderSpec{spec}, h), c
}
func receiveLifecycle(t *testing.T, ch <-chan struct{}) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(5 * time.Second):
		t.Fatal("lifecycle operation did not complete")
	}
}

func TestLifecyclePreparationLaunchAndInitializeFailures(t *testing.T) {
	for _, phase := range []string{"prepare", "launch", "initialize", "timeout"} {
		t.Run(phase, func(t *testing.T) {
			l, c := lifecycleFixture()
			var states []string
			l.host.Emit = func(_ string, v any) { states = append(states, v.(map[string]any)["state"].(string)) }
			spec := l.specs["codex"]
			spec.InitError = "initialize Codex"
			switch phase {
			case "prepare":
				spec.Prepare = func() (ProviderStart, error) { return ProviderStart{}, errors.New("prepare failed") }
			case "launch":
				spec.Prepare = func() (ProviderStart, error) {
					return ProviderStart{Launch: func(acp.Handler) (Connection, error) { return nil, errors.New("launch failed") }}, nil
				}
			case "initialize":
				c.initialize = func(context.Context) error { return errors.New("initialize failed") }
			case "timeout":
				spec.InitWithin = time.Millisecond
				c.initialize = func(ctx context.Context) error { <-ctx.Done(); return ctx.Err() }
			}
			l.specs["codex"] = spec
			if _, _, err := l.Live("codex"); err == nil {
				t.Fatal("failed start succeeded")
			}
			if l.runtime.ProviderLocked("codex") != nil {
				t.Fatal("failed start published provider")
			}
			expected := []string{"starting", "failed"}
			if phase == "prepare" {
				expected = nil
			}
			if !reflect.DeepEqual(states, expected) {
				t.Fatalf("states %v, want %v", states, expected)
			}
			if phase == "initialize" || phase == "timeout" {
				receiveLifecycle(t, c.done)
			}
		})
	}
}

// States holds the last state told to the page for each provider, stored
// before the event is emitted, so a page that missed the event reads the same
// thing and one that saw it never reads an older one.
func TestLifecycleKeepsTheLastStateOfEachProvider(t *testing.T) {
	l, c := lifecycleFixture()
	var told []ProviderState
	l.host.Emit = func(name string, v any) {
		if name != "agent:state" {
			return
		}
		m := v.(map[string]any)
		told = append(told, ProviderState{Provider: m["provider"].(string), State: m["state"].(string), Detail: m["detail"].(string)})
		// Whatever the page is told is already readable.
		got := l.States()
		if len(got) != 1 || bare(got[0]) != told[len(told)-1] {
			t.Errorf("an event for %v preceded its stored state %v", told[len(told)-1], got)
		}
	}
	if got := l.States(); len(got) != 0 {
		t.Fatalf("states before any provider started: %v", got)
	}
	if _, _, err := l.Live("codex"); err != nil {
		t.Fatal(err)
	}
	want := []ProviderState{{Provider: "codex", State: "ready", Detail: "ready"}}
	if got := l.States(); !reflect.DeepEqual(bareAll(got), want) || len(told) != 2 || told[0].State != "starting" {
		t.Fatalf("after starting: states %v, told %v", got, told)
	}
	l.Stop("codex")
	receiveLifecycle(t, c.done)
	if got := l.States(); len(got) != 1 || got[0].State != "stopped" {
		t.Fatalf("after stopping: %v", got)
	}
}

func TestLifecycleSerializesLiveThroughReadyHook(t *testing.T) {
	l, c := lifecycleFixture()
	prepared := 0
	entered, release := make(chan struct{}), make(chan struct{})
	spec := l.specs["codex"]
	spec.Prepare = func() (ProviderStart, error) {
		// Preparation must be able to acquire the shared state lock.
		l.mu.Lock()
		l.mu.Unlock()
		prepared++
		return ProviderStart{Launch: func(acp.Handler) (Connection, error) { return c, nil }, Ready: func(acp.InitializeResult) { close(entered); <-release }}, nil
	}
	l.specs["codex"] = spec
	first := make(chan struct{})
	go func() {
		defer close(first)
		if err := l.Connect("codex"); err != nil {
			t.Error(err)
		}
	}()
	receiveLifecycle(t, entered)
	if l.startLock("codex").TryLock() {
		l.startLock("codex").Unlock()
		t.Fatal("ready hook lost connect serialization")
	}
	l.mu.Lock()
	p := l.runtime.ProviderLocked("codex")
	l.mu.Unlock()
	if p == nil {
		t.Fatal("ready hook preceded publication")
	}
	second := make(chan struct{})
	go func() {
		defer close(second)
		if err := l.Connect("codex"); err != nil {
			t.Error(err)
		}
	}()
	close(release)
	receiveLifecycle(t, first)
	receiveLifecycle(t, second)
	if prepared != 1 {
		t.Fatalf("prepared %d times", prepared)
	}
	l.Stop("codex")
}

func TestLifecycleStopRevocationCleanupCompletionAndFlushOrder(t *testing.T) {
	l, c := lifecycleFixture()
	var order []string
	var orderMu sync.Mutex
	add := func(s string) { orderMu.Lock(); order = append(order, s); orderMu.Unlock() }
	entered, release := make(chan struct{}), make(chan struct{})
	cleanup, releaseCleanup := make(chan struct{}), make(chan struct{})
	l.host.RevokeChannels = func(Connection) { add("revoke"); close(entered); <-release }
	l.host.CloseChatLog = func(string) { add("log"); close(cleanup); <-releaseCleanup }
	l.host.CancelQuestions = func(string) { add("questions") }
	l.host.DisconnectSubagents = func(*Conversation) { add("subagents") }
	l.host.Emit = func(name string, _ any) { add(name) }
	p := NewProvider(c, acp.InitializeResult{}, lifecycleTrace{func() { add("flush") }})
	s := NewConversation(Identity{ID: "chat", Provider: "codex"}, InitialActivity{Open: true}, false, false)
	l.runtime.PutProviderLocked("codex", p)
	l.runtime.PutConversationLocked("chat", s)
	go l.Watch("codex", p)
	// Stop must not acquire the connect lock.
	l.startLock("codex").Lock()
	stopped := make(chan struct{})
	go func() { defer close(stopped); l.Stop("codex") }()
	receiveLifecycle(t, entered)
	l.mu.Lock()
	same := l.runtime.ConversationLocked("chat") == s
	l.mu.Unlock()
	if !same {
		t.Fatal("detach preceded revocation")
	}
	select {
	case <-p.Finished():
		t.Fatal("completed during revocation")
	default:
	}
	close(release)
	receiveLifecycle(t, cleanup)
	l.mu.Lock()
	detached := l.runtime.ConversationLocked("chat") == nil && l.runtime.ProviderLocked("codex") == nil
	l.mu.Unlock()
	if !detached {
		t.Fatal("cleanup preceded atomic detach")
	}
	select {
	case <-p.Finished():
		t.Fatal("completed during cleanup")
	default:
	}
	close(releaseCleanup)
	receiveLifecycle(t, stopped)
	l.startLock("codex").Unlock()
	want := []string{"revoke", "log", "questions", "subagents", "agent:session", "agent:state", "flush"}
	if !reflect.DeepEqual(order, want) {
		t.Fatalf("order %v, want %v", order, want)
	}
	if s.Info().Open {
		t.Fatal("provider stop left conversation open")
	}
}

func TestLifecycleStaleWatcherKeepsReplacementAndEffects(t *testing.T) {
	l, c := lifecycleFixture()
	old := NewProvider(c, acp.InitializeResult{}, nil)
	replacement := NewProvider(nil, acp.InitializeResult{}, nil)
	s := NewConversation(Identity{ID: "chat", Provider: "codex"}, InitialActivity{Open: true}, false, false)
	l.runtime.PutProviderLocked("codex", replacement)
	l.runtime.PutConversationLocked("chat", s)
	var order []string
	l.host.RevokeChannels = func(Connection) { order = append(order, "revoke") }
	l.host.CloseChatLog = func(string) { t.Error("stale watcher cleaned replacement") }
	l.host.Emit = func(name string, _ any) { order = append(order, name) }
	c.Close()
	l.Watch("codex", old)
	if l.runtime.ProviderLocked("codex") != replacement || l.runtime.ConversationLocked("chat") != s {
		t.Fatal("stale watcher detached replacement")
	}
	receiveLifecycle(t, old.Finished())
	if !reflect.DeepEqual(order, []string{"revoke", "agent:state"}) {
		t.Fatalf("stale watcher effects %v", order)
	}
}

// bare is a provider state without the numbers that order its snapshots.
func bare(s ProviderState) ProviderState {
	s.StateGen, s.StateVer = "", 0
	return s
}

func bareAll(in []ProviderState) []ProviderState {
	out := make([]ProviderState, len(in))
	for i, s := range in {
		out[i] = bare(s)
	}
	return out
}

// While one provider is starting, which takes seconds, a chat on a provider that is
// running and a chat that needs another provider are not made to wait for it. A
// second request for the provider that is starting waits for that one start.
func TestAProviderStartHoldsBackNoOtherProvider(t *testing.T) {
	newSpec := func(id string, ready func(acp.InitializeResult)) ProviderSpec {
		c := &lifecycleConnection{done: make(chan struct{}), initialize: func(context.Context) error { return nil }}
		return ProviderSpec{ID: id, Starting: "starting", Stopped: "stopped", InitWithin: time.Second, ReadyDetail: func(acp.InitializeResult) string { return "ready" },
			Prepare: func() (ProviderStart, error) {
				return ProviderStart{Launch: func(acp.Handler) (Connection, error) { return c, nil }, Ready: ready}, nil
			}}
	}
	h := LifecycleHost{
		Handler: func() acp.Handler { return nil }, RedactTrace: func(s string) string { return s },
		RevokeChannels: func(Connection) {}, CloseChatLog: func(string) {}, CancelQuestions: func(string) {},
		DisconnectSubagents: func(*Conversation) {}, Emit: func(string, any) {},
	}
	entered, release := make(chan struct{}), make(chan struct{})
	slow := newSpec("slow", func(acp.InitializeResult) { close(entered); <-release })
	l := NewProviderLifecycle(&sync.Mutex{}, &Runtime{}, []ProviderSpec{newSpec("running", nil), newSpec("other", nil), slow}, h)
	t.Cleanup(l.StopAll)
	if err := l.Connect("running"); err != nil {
		t.Fatal(err)
	}
	starting := make(chan struct{})
	go func() {
		defer close(starting)
		if err := l.Connect("slow"); err != nil {
			t.Error(err)
		}
	}()
	receiveLifecycle(t, entered)
	answered := func(provider string) <-chan error {
		done := make(chan error, 1)
		go func() { _, _, err := l.Live(provider); done <- err }()
		return done
	}
	for _, provider := range []string{"running", "other"} {
		select {
		case err := <-answered(provider):
			if err != nil {
				t.Fatalf("%s: %v", provider, err)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("a chat on %s waited for another provider's start", provider)
		}
	}
	second := answered("slow")
	select {
	case <-second:
		t.Fatal("a second request for the provider that is starting did not wait for it")
	case <-time.After(100 * time.Millisecond):
	}
	close(release)
	receiveLifecycle(t, starting)
	select {
	case err := <-second:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the second request did not reuse the start")
	}
}
