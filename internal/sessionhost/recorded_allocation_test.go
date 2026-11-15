package sessionhost

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"studio/internal/acp"
)

func TestRecordedAllocationPersistsBeforeSetupAndRetainsEveryOutcome(t *testing.T) {
	for _, mode := range []string{"success", "prepare", "dispatch", "unknown", "canceled", "shutdown after receipt", "bind", "stale", "restore", "finish", "receipt", "finish record", "global receipt"} {
		t.Run(mode, func(t *testing.T) {
			var mu sync.Mutex
			var runtime Runtime
			conn := &lifecycleConnection{done: make(chan struct{})}
			runtime.PutProviderLocked("codex", NewProvider(conn, acp.InitializeResult{}, nil))
			dir := t.TempDir()
			intentFile, dispatchFile, receiptFile := filepath.Join(dir, "intent"), filepath.Join(dir, "dispatch"), filepath.Join(dir, "receipt")
			if err := os.WriteFile(intentFile, []byte("task:operation"), 0600); err != nil {
				t.Fatal(err)
			}
			calls, finished, received, revoked := 0, 0, 0, false
			finalID := ""
			var finalOutcome error
			failure := errors.New("invented " + mode + " failure")
			callbacks := AllocationCallbacks{
				Dispatched: func() error {
					if _, err := os.Stat(intentFile); err != nil {
						t.Fatal("dispatch preceded intent", err)
					}
					if mode == "dispatch" {
						return failure
					}
					return os.WriteFile(dispatchFile, []byte("unknown until receipt"), 0600)
				},
				Received: func(id string) error {
					received++
					if id != "codex:received" {
						t.Fatal(id)
					}
					if mode == "receipt" {
						return failure
					}
					return os.WriteFile(receiptFile, []byte(id), 0600)
				},
				Finished: func(id string, outcome error) error {
					finished++
					finalID, finalOutcome = id, outcome
					if mode == "finish record" {
						return failure
					}
					return nil
				},
			}
			host := SessionLifecycleHost{
				NormalizeProvider: func(p string) string { return p },
				PrepareApplets: func(string, string) error {
					if mode == "prepare" {
						return failure
					}
					return nil
				},
				Live:     func(string) (Connection, acp.InitializeResult, error) { return conn, acp.InitializeResult{}, nil },
				Defaults: func(string) map[string]json.RawMessage { return nil },
				NewChannel: func(Connection, string) (ChannelCapability, []any, error) {
					return &lifecycleTestChannel{bind: func(string) error {
						if data, err := os.ReadFile(receiptFile); err != nil || string(data) != "codex:received" {
							t.Fatal("binding preceded durable receipt", err)
						}
						if mode == "bind" {
							return failure
						}
						return nil
					}, revoke: func() { revoked = true }}, nil, nil
				},
				ManagedServers: func() ([]any, error) { return nil, nil },
				BusSessionID:   func(string) string { return "new-recipient" },
				WithBusServer:  func(_ string, _ string, _ string, servers []any) []any { return servers },
				EndBusSession:  func(string) {},
				SessionParams:  func(string, string, string, []any) any { return nil },
				Call: func(_ Connection, method string, _ any, out any) error {
					calls++
					if method != "session/new" {
						t.Fatal("unexpected native call", method)
					}
					if _, err := os.Stat(dispatchFile); err != nil {
						t.Fatal("native allocation preceded dispatch record", err)
					}
					if mode == "canceled" {
						return context.Canceled
					}
					if mode == "unknown" {
						return failure
					}
					out.(*acp.NewSessionResult).SessionID = "codex:received"
					if mode == "stale" {
						runtime.PutProviderLocked("codex", NewProvider(&lifecycleConnection{done: make(chan struct{})}, acp.InitializeResult{}, nil))
					}
					return nil
				},
				InitTranscript: func(*Conversation, bool) {}, BeginLog: func(string, string, string, string) {},
				RestoreOptions: func(Connection, string, []acp.ConfigOption, map[string]json.RawMessage, *Reopening) ([]acp.ConfigOption, []AppliedSubstitution, error) {
					if mode == "restore" {
						return nil, nil, failure
					}
					return nil, nil, nil
				},
				FinishOptions: func(*Conversation, []acp.ConfigOption) error {
					if mode == "shutdown after receipt" {
						return context.Canceled
					}
					if mode == "finish" {
						return failure
					}
					return nil
				},
				JoinBus: func(*Conversation, string) {}, Emit: func(string, any) {},
			}
			if mode == "global receipt" {
				host.BeginAllocation = func(intent AllocationIntent) (AllocationCallbacks, error) {
					if intent.Source != "task:operation" || intent.Kind != "task_binding" {
						t.Fatal("task operation was interpreted as a native chat", intent)
					}
					return AllocationCallbacks{Received: func(string) error { return failure }}, nil
				}
			}
			info, err := NewSessionLifecycle(&mu, &runtime, host).NewRecorded(AllocationIntent{Source: "task:operation", Workspace: dir, Provider: "codex", Kind: "task_binding", Revision: "approved"}, callbacks)
			if finished != 1 {
				t.Fatal("finish callback count", finished)
			}
			if mode == "prepare" || mode == "dispatch" {
				var notCreated *AllocationNotCreatedError
				if calls != 0 || info.ID != "" || !errors.As(err, &notCreated) || finalID != "" {
					t.Fatal("pre-dispatch failure lost noncreation result", calls, info, err)
				}
				return
			}
			if calls != 1 {
				t.Fatal("allocation replayed", calls)
			}
			if mode == "unknown" || mode == "canceled" {
				var notCreated *AllocationNotCreatedError
				if err == nil || info.ID != "" || received != 0 || errors.As(err, &notCreated) || finalID != "" || finalOutcome == nil {
					t.Fatal("unknown outcome was fabricated or replayed", info, err)
				}
				if _, err := os.Stat(dispatchFile); err != nil {
					t.Fatal("unknown outcome lost dispatch obligation", err)
				}
				return
			}
			if received != 1 || info.ID != "codex:received" || finalID != info.ID {
				t.Fatal("receipt lost", received, info, finalID, err)
			}
			if mode == "success" {
				if err != nil || revoked || runtime.ConversationLocked(info.ID) == nil || finalOutcome != nil {
					t.Fatal("success was not published", info, err)
				}
			} else {
				var allocation *acp.SessionAllocationError
				var notCreated *AllocationNotCreatedError
				if !errors.As(err, &allocation) || allocation.SessionID != info.ID || errors.As(err, &notCreated) || !revoked || runtime.ConversationLocked(info.ID) != nil {
					t.Fatal("post-allocation failure lost receipt or retained authority", info, err)
				}
			}
		})
	}
}

func TestRecordedAllocationRejectsMissingAuthorityBeforeAnyNativeCall(t *testing.T) {
	complete := AllocationCallbacks{Dispatched: func() error { return nil }, Received: func(string) error { return nil }, Finished: func(string, error) error { return nil }}
	for _, missing := range []string{"source", "kind", "recipient", "dispatch", "receipt", "finish"} {
		t.Run(missing, func(t *testing.T) {
			intent := AllocationIntent{Source: "task:operation", Kind: "task_binding", Workspace: t.TempDir(), Provider: "codex"}
			callbacks := complete
			switch missing {
			case "source":
				intent.Source = ""
			case "kind":
				intent.Kind = ""
			case "recipient":
				intent.Recipient = "existing"
			case "dispatch":
				callbacks.Dispatched = nil
			case "receipt":
				callbacks.Received = nil
			case "finish":
				callbacks.Finished = nil
			}
			// An empty service would panic if validation reached provider preparation.
			info, err := (&SessionLifecycle{}).NewRecorded(intent, callbacks)
			var notCreated *AllocationNotCreatedError
			if info.ID != "" || !errors.As(err, &notCreated) {
				t.Fatal(info, err)
			}
		})
	}
}
