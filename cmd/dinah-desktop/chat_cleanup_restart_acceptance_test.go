//go:build chatcleanup_acceptance

package main

import (
	"context"
	"sync"
	"testing"
	"time"

	"studio/internal/acp"
	"studio/internal/bus"
)

// This fixture replaces every adapter before constructing a lifecycle. It must
// run in a nonparallel test. No real preparation, process or provider is reachable.
type chatCleanupAcceptanceProviders struct {
	mu          sync.Mutex
	starts      map[string]int
	calls       map[string]map[string]int
	connections []*chatCleanupAcceptanceConnection
}

type chatCleanupAcceptanceConnection struct {
	*testProviderConnection
	owner    *chatCleanupAcceptanceProviders
	provider string
	callMu   sync.Mutex
}

func (c *chatCleanupAcceptanceConnection) Call(ctx context.Context, method string, params, result any) error {
	c.owner.mu.Lock()
	if c.owner.calls[c.provider] == nil {
		c.owner.calls[c.provider] = map[string]int{}
	}
	c.owner.calls[c.provider][method]++
	c.owner.mu.Unlock()
	c.callMu.Lock()
	defer c.callMu.Unlock()
	switch method {
	case "initialize":
		return acp.Result(acp.InitializeResult{ProtocolVersion: 1, AgentCapabilities: acp.AgentCapabilities{LoadSession: true}}, result)
	case "session/list":
		return acp.Result(map[string]any{"sessions": []any{}}, result)
	default:
		return c.testProviderConnection.Call(ctx, method, params, result)
	}
}

func chatCleanupAcceptanceCountProviders(t *testing.T) *chatCleanupAcceptanceProviders {
	t.Helper()
	f := &chatCleanupAcceptanceProviders{starts: map[string]int{}, calls: map[string]map[string]int{}}
	previous := providerModules
	providerModules = map[string]providerModule{}
	for id := range previous {
		providerModules[id] = providerModule{Adapter: &providerAdapter{
			id: id, initWithin: time.Second, readyDetail: connectedDetail("Invented provider"),
			prepare: func(*App) (providerStart, error) {
				return providerStart{launch: func(acp.Handler) (agentConnection, error) {
					c := &chatCleanupAcceptanceConnection{testProviderConnection: newTestProviderConnection(), owner: f, provider: id}
					f.mu.Lock()
					f.starts[id]++
					f.connections = append(f.connections, c)
					f.mu.Unlock()
					return c, nil
				}}, nil
			},
		}}
	}
	t.Cleanup(func() { providerModules = previous })
	return f
}

// Count returns launch counts when method is empty, or the actual RPC count.
func (f *chatCleanupAcceptanceProviders) Count(provider, method string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	if method == "" {
		return f.starts[provider]
	}
	return f.calls[provider][method]
}

func (f *chatCleanupAcceptanceProviders) Bind(t *testing.T, a *App) {
	t.Helper()
	a.connectGraceOverride = -1
	a.judging.noAuto = true
	t.Cleanup(a.closeAppletBridge)
	t.Cleanup(a.closeFileAccess)
	t.Cleanup(a.closeChatStates)
	t.Cleanup(a.closeJudges)
	t.Cleanup(a.queueStore.Close)
	t.Cleanup(a.closeAgentLogs)
	t.Cleanup(func() {
		for id := range providerModules {
			a.AgentStop(id)
		}
		a.turnIO().Close(3 * time.Second)
		a.busHost().CloseHandle()
	})
}

// RestartChats runs the production chat startup operations synchronously, then
// performs a real discovery round after the page's ChatWindows notification.
// It omits unrelated update/install/maintenance goroutines from App.startup.
func (a *App) chatCleanupAcceptanceRestartChats() error {
	if err := a.reconcileChatRemovals(); err != nil {
		return err
	}
	a.queueRestorer().Sweep()
	a.ChatWindows(nil)
	return a.chatCleanupAcceptanceDiscoveryRound()
}

func (a *App) chatCleanupAcceptanceDiscoveryRound() error {
	return a.busHost().With(func(b *bus.Bus) error {
		a.busCoordinator().ConnectWaking(context.Background(), b)
		a.busCoordinator().DeliverRound(context.Background(), b)
		return nil
	})
}

func TestCleanupAcceptanceRestartCountsRealProviderEntry(t *testing.T) {
	a, _ := providerTestApp(t)
	ws := t.TempDir()
	id := "codex:counted-restart"
	cleanupFixtureChat(t, a, id, "codex", ws, "window-counted-restart")
	if err := a.RemoveChat(id); err != nil {
		t.Fatal(err)
	}
	a.busHost().CloseHandle()
	f := chatCleanupAcceptanceCountProviders(t)
	b := NewApp()
	f.Bind(t, b)
	requests := watchConnects(b)
	if err := b.chatCleanupAcceptanceRestartChats(); err != nil {
		t.Fatal(err)
	}
	if err := b.busHost().With(func(busStore *bus.Bus) error {
		_, err := busStore.EnqueueResult(context.Background(), "rejected-counted-message", "window-counted-restart", "Never prompt the binned chat")
		if err == nil {
			t.Error("binned recipient accepted a message")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := b.chatCleanupAcceptanceDiscoveryRound(); err != nil {
		t.Fatal(err)
	}
	if asked := requests.take(); len(asked) != 0 {
		t.Fatalf("binned chat requested: %v", asked)
	}
	for provider := range providerModules {
		for _, method := range []string{"", "session/load", "session/resume", "session/prompt"} {
			if n := f.Count(provider, method); n != 0 {
				t.Fatalf("%s %s count %d", provider, method, n)
			}
		}
	}
	if result := b.RestoreRemovedChat(id); !result.OK {
		t.Fatal(result)
	}
	if f.Count("codex", "") != 0 {
		t.Fatal("Restore started a provider")
	}
	if _, err := b.OpenSessionAt(ws, id); err != nil {
		t.Fatal(err)
	}
	if f.Count("codex", "") != 1 || f.Count("codex", "session/load") != 1 {
		t.Fatal("explicit reopening did not exercise counted launch and load")
	}
	b.askWait(context.Background(), b.session(id), newPrompt("Invented positive control", nil))
	if f.Count("codex", "session/prompt") != 1 {
		t.Fatal("positive control did not exercise the prompt counter")
	}
}
