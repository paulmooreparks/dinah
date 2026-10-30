package sessionhost

import (
	"encoding/json"
	"errors"
	"reflect"
	"studio/internal/acp"
	"sync"
	"testing"
)

type lifecycleTestChannel struct {
	bind   func(string) error
	revoke func()
}

func (c *lifecycleTestChannel) Bind(id string) error { return c.bind(id) }
func (c *lifecycleTestChannel) Revoke()              { c.revoke() }

func TestSessionLifecycleNewPublicationAndRollbackOrder(t *testing.T) {
	for _, mode := range []string{"success", "bind", "restore", "finish", "stale"} {
		t.Run(mode, func(t *testing.T) {
			var mu sync.Mutex
			var runtime Runtime
			conn := &lifecycleConnection{done: make(chan struct{})}
			runtime.PutProviderLocked("codex", NewProvider(conn, acp.InitializeResult{}, nil))
			var order []string
			failure := errors.New("failed")
			var published *Conversation
			ch := &lifecycleTestChannel{bind: func(string) error {
				order = append(order, "bind")
				if mode == "bind" {
					return failure
				}
				return nil
			}, revoke: func() { order = append(order, "revoke") }}
			host := SessionLifecycleHost{
				NormalizeProvider: func(p string) string { return p }, PrepareApplets: func(string, string) error { return nil },
				Live:       func(string) (Connection, acp.InitializeResult, error) { return conn, acp.InitializeResult{}, nil },
				Defaults:   func(string) map[string]json.RawMessage { return nil },
				NewChannel: func(Connection, string) (ChannelCapability, []any, error) { return ch, nil, nil }, ManagedServers: func() ([]any, error) { return nil, nil },
				BusSessionID: func(string) string { return "bus" }, WithBusServer: func(string, string, string, []any) []any { return nil },
				EndBusSession: func(string) { order = append(order, "bus-end") }, SessionParams: func(string, string, string, []any) any { return nil },
				Call: func(_ Connection, method string, _ any, out any) error {
					if method != "session/new" {
						t.Fatal(method)
					}
					order = append(order, "new")
					out.(*acp.NewSessionResult).SessionID = "chat"
					if mode == "stale" {
						runtime.PutProviderLocked("codex", NewProvider(&lifecycleConnection{done: make(chan struct{})}, acp.InitializeResult{}, nil))
					}
					return nil
				},
				InitTranscript: func(s *Conversation, _ bool) {
					mu.Lock()
					defer mu.Unlock()
					if runtime.ConversationLocked("chat") != s {
						t.Fatal("transcript preceded publication")
					}
					published = s
					order = append(order, "transcript")
				},
				BeginLog: func(string, string, string, string) { order = append(order, "log") },
				RestoreOptions: func(Connection, string, []acp.ConfigOption, map[string]json.RawMessage, *Reopening) ([]acp.ConfigOption, []AppliedSubstitution, error) {
					order = append(order, "restore")
					if mode == "restore" {
						return nil, nil, failure
					}
					return nil, nil, nil
				},
				FinishOptions: func(s *Conversation, _ []acp.ConfigOption) error {
					order = append(order, "finish")
					if mode == "finish" {
						return failure
					}
					s.SetRestoring(false)
					return nil
				},
				JoinBus: func(*Conversation, string) { order = append(order, "join") }, Emit: func(string, any) { order = append(order, "event") },
			}
			service := NewSessionLifecycle(&mu, &runtime, host)
			_, err := service.New(t.TempDir(), "codex", false)
			want := []string{"new", "bind"}
			switch mode {
			case "success":
				want = append(want, "transcript", "log", "restore", "finish", "join", "event")
			case "restore":
				want = append(want, "transcript", "log", "restore", "revoke", "bus-end")
			case "finish":
				want = append(want, "transcript", "log", "restore", "finish", "revoke", "bus-end")
			default:
				want = append(want, "revoke", "bus-end")
			}
			if !reflect.DeepEqual(order, want) {
				t.Fatalf("order %v", order)
			}
			if mode == "success" {
				if err != nil || runtime.ConversationLocked("chat") != published {
					t.Fatal(err)
				}
			} else {
				if err == nil || runtime.ConversationLocked("chat") != nil {
					t.Fatalf("rollback %v", err)
				}
				if published != nil && published.Open() {
					t.Fatal("failed conversation remained open")
				}
			}
		})
	}
}

// A session started under a bus identity the chat already has uses that
// identity and never makes, joins or ends one: the old chat owns it until the
// switch commits, and the switch moves it. A failure to register the identity
// fails the session before the agent is asked for anything.
func TestAdoptingANewSessionUsesTheChatsBusIdentityAndNeverEndsIt(t *testing.T) {
	for _, mode := range []string{"success", "register", "restore", "finish"} {
		t.Run(mode, func(t *testing.T) {
			var mu sync.Mutex
			var runtime Runtime
			conn := &lifecycleConnection{done: make(chan struct{})}
			runtime.PutProviderLocked("codex", NewProvider(conn, acp.InitializeResult{}, nil))
			var order []string
			failure := errors.New("failed")
			ch := &lifecycleTestChannel{bind: func(string) error { return nil }, revoke: func() { order = append(order, "revoke") }}
			host := SessionLifecycleHost{
				NormalizeProvider: func(p string) string { return p }, PrepareApplets: func(string, string) error { return nil },
				Live:       func(string) (Connection, acp.InitializeResult, error) { return conn, acp.InitializeResult{}, nil },
				Defaults:   func(string) map[string]json.RawMessage { return nil },
				NewChannel: func(Connection, string) (ChannelCapability, []any, error) { return ch, nil, nil }, ManagedServers: func() ([]any, error) { return nil, nil },
				BusSessionID:  func(string) string { order = append(order, "made"); return "fresh" },
				WithBusServer: func(string, string, string, []any) []any { order = append(order, "served-fresh"); return nil },
				AdoptBusServer: func(id, _ string, servers []any) ([]any, error) {
					order = append(order, "adopt-"+id)
					if mode == "register" {
						return nil, failure
					}
					return servers, nil
				},
				EndBusSession: func(id string) { order = append(order, "bus-end-"+id) }, SessionParams: func(string, string, string, []any) any { return nil },
				Call: func(_ Connection, method string, _ any, out any) error {
					order = append(order, "new")
					out.(*acp.NewSessionResult).SessionID = "chat"
					return nil
				},
				InitTranscript: func(*Conversation, bool) {},
				BeginLog:       func(string, string, string, string) {},
				RestoreOptions: func(Connection, string, []acp.ConfigOption, map[string]json.RawMessage, *Reopening) ([]acp.ConfigOption, []AppliedSubstitution, error) {
					if mode == "restore" {
						return nil, nil, failure
					}
					return nil, nil, nil
				},
				FinishOptions: func(s *Conversation, _ []acp.ConfigOption) error {
					if mode == "finish" {
						return failure
					}
					s.SetRestoring(false)
					return nil
				},
				JoinBus: func(*Conversation, string) { order = append(order, "join") }, Emit: func(string, any) {},
			}
			service := NewSessionLifecycle(&mu, &runtime, host)
			_, err := service.NewAdopting(t.TempDir(), "codex", "window-kept")
			want := map[string][]string{
				"success":  {"adopt-window-kept", "new"},
				"register": {"adopt-window-kept", "revoke"},
				"restore":  {"adopt-window-kept", "new", "revoke"},
				"finish":   {"adopt-window-kept", "new", "revoke"},
			}[mode]
			if !reflect.DeepEqual(order, want) {
				t.Fatalf("order %v, want %v", order, want)
			}
			if (err == nil) != (mode == "success") {
				t.Fatalf("error %v", err)
			}
		})
	}
}

// Forgetting a chat that is working cancels its turn first, so a hidden chat
// is not left running with nobody to stop it; an idle chat has nothing to
// cancel, and a chat the runtime does not have is only forgotten.
func TestForgettingAWorkingChatCancelsItsTurn(t *testing.T) {
	for _, tc := range []struct {
		name    string
		working bool
		known   bool
		cancels int
	}{{"working", true, true, 1}, {"idle", false, true, 0}, {"unknown", false, false, 0}} {
		t.Run(tc.name, func(t *testing.T) {
			var mu sync.Mutex
			var runtime Runtime
			var order []string
			s := NewConversation(Identity{ID: "chat", Provider: "ollama"}, InitialActivity{Open: true, Working: tc.working}, false, false)
			if tc.known {
				runtime.PutConversationLocked("chat", s)
			}
			host := SessionLifecycleHost{
				Cancel:   func(got *Conversation) error { order = append(order, "cancel"); return nil },
				LeaveBus: func(*Conversation) { order = append(order, "leave") }, RevokeSession: func(string) { order = append(order, "revoke") },
				ForgetJournal: func(string) {}, DropSubagentIndex: func(string) {}, ForgetTitle: func(string) {}, CancelQuestions: func(string) {},
			}
			NewSessionLifecycle(&mu, &runtime, host).Forget("chat")
			cancels := 0
			for _, step := range order {
				if step == "cancel" {
					cancels++
				}
			}
			if cancels != tc.cancels {
				t.Fatalf("%d cancellations, want %d: %v", cancels, tc.cancels, order)
			}
			if tc.cancels == 1 && (len(order) < 2 || order[0] != "cancel" || order[1] != "leave") {
				t.Fatalf("the turn must be cancelled before the chat leaves the bus: %v", order)
			}
			if runtime.ConversationLocked("chat") != nil {
				t.Fatal("the chat is still in the runtime")
			}
		})
	}
}
