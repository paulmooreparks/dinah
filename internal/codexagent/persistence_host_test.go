package codexagent

import (
	"context"
	"encoding/json"
	"os"
	"reflect"
	"sync"
	"testing"
	"time"

	"studio/internal/acp"
	"studio/internal/applet/broker"
	"studio/internal/sessionhost"
)

type persistenceChannel struct{ bound *bool }

func (c persistenceChannel) Bind(string) error { *c.bound = true; return nil }
func (c persistenceChannel) Revoke()           {}

// This crosses the real adapter, SessionLifecycle and Continuations boundaries.
// The callbacks replace only the desktop's settings and bus integration.
func TestPersistenceReceiptPreservesHostContinuationAndRecoveryTransactions(t *testing.T) {
	for _, recovery := range []bool{false, true} {
		for _, mode := range []string{"success", "rejected", "cancelled"} {
			name := map[bool]string{false: "continue", true: "recover"}[recovery] + "/" + mode
			t.Run(name, func(t *testing.T) {
				dir, cwd := t.TempDir(), t.TempDir()
				f := &persistenceFixture{reject: mode == "rejected"}
				conn := f.connection(t, cwd)
				ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
				defer cancel()
				if mode == "cancelled" {
					f.beforeReceipt = cancel
				}
				var transcriptMu, stateMu sync.Mutex
				var runtime sessionhost.Runtime
				runtime.PutProviderLocked("codex", sessionhost.NewProvider(conn, acp.InitializeResult{}, nil))
				parent := sessionhost.NewConversation(sessionhost.Identity{ID: "source", Provider: "old", Cwd: cwd, Title: "PUDL Verification"}, sessionhost.InitialActivity{Open: true}, false, false)
				parent.JoinBus("team-member-window")
				ancestor := &sessionhost.Transcript{SessionID: "ancestor", Provider: "older", Workspace: cwd, Complete: true, Messages: []sessionhost.ConversationMessage{{Role: "you", Text: "inherited ancestor history"}}}
				if err := sessionhost.WriteTranscript(dir, ancestor); err != nil {
					t.Fatal(err)
				}
				original := &sessionhost.Transcript{SessionID: "source", Provider: "old", Workspace: cwd, Complete: true, ParentSessionID: "ancestor", PendingHandoff: true, Handoff: "pending source handoff", ParentHistory: ancestor.Messages, Messages: []sessionhost.ConversationMessage{{Role: "you", Text: "retained user history"}, {Role: "agent", Text: "retained answer"}}}
				if err := sessionhost.WriteTranscript(dir, original); err != nil {
					t.Fatal(err)
				}
				parent.HistoryRestoreSavedLocked(original)
				before, err := os.ReadFile(sessionhost.TranscriptPath(dir, "source"))
				if err != nil {
					t.Fatal(err)
				}
				if !recovery {
					runtime.PutConversationLocked("source", parent)
				}
				bound, moved, revoked := false, false, false
				owner := "source"
				titles := map[string]string{"source": "PUDL Verification"}
				lookup := func(id string) *sessionhost.Conversation {
					stateMu.Lock()
					defer stateMu.Unlock()
					return runtime.ConversationLocked(id)
				}
				lifecycle := sessionhost.NewSessionLifecycle(&stateMu, &runtime, sessionhost.SessionLifecycleHost{
					NormalizeProvider: func(s string) string { return s }, PrepareApplets: func(string, string) error { return nil },
					Live: func(string) (sessionhost.Connection, acp.InitializeResult, error) {
						return conn, acp.InitializeResult{}, nil
					},
					Defaults: func(string) map[string]json.RawMessage { return nil },
					NewChannel: func(sessionhost.Connection, string) (sessionhost.ChannelCapability, []any, error) {
						return persistenceChannel{&bound}, nil, nil
					},
					ManagedServers: func() ([]any, error) { return nil, nil },
					AdoptBusServer: func(id, _ string, servers []any) ([]any, error) {
						if id != "team-member-window" {
							t.Fatal("lost adopted identity")
						}
						return servers, nil
					},
					EndBusSession: func(string) { t.Fatal("ended existing team identity") },
					SessionParams: func(_, _, workspace string, _ []any) any { return acp.NewSessionParams{Cwd: workspace} },
					Call:          func(c sessionhost.Connection, method string, p, out any) error { return c.Call(ctx, method, p, out) },
					InitTranscript: func(s *sessionhost.Conversation, replay bool) {
						transcriptMu.Lock()
						defer transcriptMu.Unlock()
						s.InitializeHistoryLocked(dir, replay)
					},
					BeginLog: func(string, string, string, string) {},
					RestoreOptions: func(_ sessionhost.Connection, _ string, options []acp.ConfigOption, _ map[string]json.RawMessage, _ *sessionhost.Reopening) ([]acp.ConfigOption, []sessionhost.AppliedSubstitution, error) {
						return options, nil, nil
					},
					FinishOptions: func(s *sessionhost.Conversation, _ []acp.ConfigOption) error { s.SetRestoring(false); return nil },
				})
				continuations := sessionhost.NewContinuations(&transcriptMu, &stateMu, &runtime, sessionhost.ContinuationHost{
					StateDir: func() string { return dir }, SupportedProvider: func(string) bool { return true }, NormalizeProvider: func(s string) string { return s },
					Session: lookup, Invocations: func(string, string) ([]broker.Invocation, error) { return nil, nil }, HasPending: func(string) bool { return false },
					Budget:      func(string, []acp.ConfigOption) int { return sessionhost.MinimumHandoffBudget },
					Formatter:   sessionhost.HandoffFormatter{ProductName: "Dinah", Speaker: func(m sessionhost.ConversationMessage) string { return m.Role }},
					BusIdentity: func(*sessionhost.Conversation) string { return "team-member-window" },
					NewSession:  lifecycle.NewAdopting,
					MoveBusIdentity: func(from, to, id string) error {
						if from != owner || id != "team-member-window" {
							t.Fatal("changed team routing")
						}
						owner, moved = to, true
						return nil
					},
					HideSession: func(string) error { return nil }, RevokeSession: func(id string) {
						if id == "source" {
							revoked = true
						}
					}, Emit: func(string, any) {},
					Title: func(id string) string { return titles[id] }, RenameSession: func(id, title string) (map[string]string, error) { titles[id] = title; return titles, nil },
				})
				var preview sessionhost.SessionContinuationPreview
				if recovery {
					preview, err = continuations.PreviewRecovery("source", "codex")
				} else {
					preview, err = continuations.PreviewSession("source", "codex")
				}
				if err != nil {
					t.Fatal(err)
				}
				var result sessionhost.SessionContinuationResult
				if recovery {
					result, err = continuations.Recover("source", "codex", preview.Revision)
				} else {
					result, err = continuations.ContinueSession("source", "codex", preview.Revision)
				}
				after, readErr := os.ReadFile(sessionhost.TranscriptPath(dir, "source"))
				if readErr != nil || !reflect.DeepEqual(before, after) {
					t.Fatal("rewrote original saved history")
				}
				if mode != "success" {
					if err == nil || bound || moved || revoked || owner != "source" || lookup("codex:new-thread") != nil {
						t.Fatalf("failed creation committed host transaction: %v", err)
					}
					if !recovery && (!parent.Open() || parent.State().Switching) {
						t.Fatal("damaged parent after failure")
					}
					return
				}
				if err != nil {
					t.Fatal(err)
				}
				next := lookup(result.Session.ID)
				if !bound || !moved || !revoked || owner != result.Session.ID || result.Session.Title != "PUDL Verification" || next == nil {
					t.Fatalf("lost title/team transaction: %+v", result)
				}
				saved, err := sessionhost.ReadTranscript(dir, next.ID())
				if err != nil || saved.ParentSessionID != "source" || !saved.PendingHandoff || saved.Handoff == "" || len(saved.ParentHistory) != 4 || len(saved.Messages) != 0 {
					t.Fatalf("handoff %+v: %v", saved, err)
				}
				if saved.ParentHistory[0].Text != "inherited ancestor history" || saved.ParentHistory[1].Text != "retained user history" || saved.ParentHistory[2].Text != "retained answer" {
					t.Fatal("lost inherited history")
				}
				pending, err := continuations.PendingContext(next)
				if err != nil || pending != saved.Handoff {
					t.Fatal("context write consumed pending handoff")
				}
				resolved, err := continuations.Resolve("source")
				if err != nil || resolved != next.ID() {
					t.Fatal("lost continuation ancestry")
				}
			})
		}
	}
}
