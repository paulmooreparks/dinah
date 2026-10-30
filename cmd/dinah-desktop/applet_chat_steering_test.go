package main

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"studio/internal/acp"
	"studio/internal/applet"
	"studio/internal/bus"
)

type chatSteeringConnection struct {
	*answeringConnection
	mode      string
	started   chan struct{}
	release   chan struct{}
	submitted chan struct{}
	accepted  atomic.Bool
}

func (c *chatSteeringConnection) CanSteer(string) bool { return c.mode != "unsupported" }
func (c *chatSteeringConnection) Steer(_ context.Context, p acp.PromptParams, delivered func()) error {
	if !strings.Contains(p.Prompt[0].Text, "Question during work?") {
		return errors.New("missing question")
	}
	defer close(c.submitted)
	switch c.mode {
	case "accepted":
		delivered()
		c.accepted.Store(true)
	case "idle":
		return acp.ErrNoActiveTurn
	case "uncertain":
		return errors.New("connection lost after submission")
	}
	return nil
}
func (c *chatSteeringConnection) Call(ctx context.Context, method string, p, out any) error {
	if method == "session/prompt" && p.(acp.PromptParams).Prompt[0].Text == "Own work" {
		close(c.started)
		<-c.release
		words := "Original answer"
		if c.accepted.Load() {
			words = "Steered answer"
		}
		update, _ := json.Marshal(map[string]any{"sessionId": "chat-B", "update": map[string]any{"sessionUpdate": "agent_message_chunk", "content": map[string]string{"type": "text", "text": words}}})
		c.a.newAgentHandler().Notify("session/update", update)
		return c.testProviderConnection.Call(ctx, method, p, out)
	}
	return c.answeringConnection.Call(ctx, method, p, out)
}

func TestChatQuestionSteersActiveTurnOrFallsBackOnce(t *testing.T) {
	for _, mode := range []string{"accepted", "pending", "idle", "uncertain", "unsupported", "cancel-pending", "hook-accepted", "hook-pending"} {
		t.Run(mode, func(t *testing.T) {
			a, ws, chats := askTestApp(t)
			base := a.runtime.ProviderLocked("codex").Connection().(*answeringConnection)
			c := &chatSteeringConnection{answeringConnection: base, mode: mode, started: make(chan struct{}), release: make(chan struct{}), submitted: make(chan struct{})}
			setTestConnection(a, "codex", c)
			s := chats["B"]
			if strings.HasPrefix(mode, "hook-") {
				s = replaceTestIdentity(a, s, "Provider", "claude")
				a.runtime.PutProviderLocked("claude", a.runtime.ProviderLocked("codex"))
				a.busCoordinator().JoinBus(s, "hook-recipient")
				if err := a.busHost().With(func(b *bus.Bus) error {
					_, err := b.RegisterSession(context.Background(), s.BusID(), s.Cwd(), s.Title())
					return err
				}); err != nil {
					t.Fatal(err)
				}
				if err := a.busHost().With(func(b *bus.Bus) error {
					return b.BindAgentSession(context.Background(), "claude-agent", s.BusID())
				}); err != nil {
					t.Fatal(err)
				}
			}
			s.StartWork()
			finished := make(chan struct{})
			go func() {
				a.turnIO().Run(s, promptPointer("Own work", []acp.ContentBlock{{Type: "text", Text: "Own work"}}))
				close(finished)
			}()
			select {
			case <-c.started:
			case <-finished:
				t.Fatal("turn exited before provider call")
			case <-time.After(testWait):
				t.Fatal("provider call never started")
			}
			if strings.HasPrefix(mode, "hook-") {
				token := s.Hook()
				if err := a.busHost().With(func(b *bus.Bus) error {
					return b.BindAgentTurn(context.Background(), "claude-agent", s.BusID(), token, "own-prompt")
				}); err != nil {
					t.Fatal(err)
				}
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			type result struct {
				out map[string]any
				err error
			}
			done := make(chan result, 1)
			go func() {
				out, err := askWithContext(t, ctx, a, ws, "chat-A", "chat-B", "Question during work?")
				done <- result{out, err}
			}()
			if strings.HasPrefix(mode, "hook-") {
				for deadline := time.Now().Add(testWait); ; {
					n := s.PendingCount()
					if n > 0 {
						break
					}
					if time.Now().After(deadline) {
						t.Fatal("no hook submission")
					}
					time.Sleep(time.Millisecond)
				}
				if mode == "hook-accepted" {
					if err := a.busHost().With(func(b *bus.Bus) error {
						got, err := b.TakeSteers(context.Background(), s.BusID())
						if len(got) != 1 {
							t.Errorf("hook got %d steers", len(got))
						}
						a.turnIO().NoticeSteers(b.SteerDelivered, s)
						return err
					}); err != nil {
						t.Fatal(err)
					}
					c.accepted.Store(true)
				}
			} else if mode == "unsupported" {
				waitQueued(t, s, 1)
			} else {
				select {
				case <-c.submitted:
				case <-time.After(testWait):
					t.Fatal("no steering submission")
				}
			}
			if mode == "uncertain" {
				if r := <-done; r.err == nil {
					t.Fatal("uncertain delivery returned success")
				}
			} else if mode == "cancel-pending" {
				cancel()
				if r := <-done; r.err == nil {
					t.Fatal("cancelled request returned success")
				}
			} else {
				select {
				case r := <-done:
					t.Fatalf("answered before turn finished: %v", r)
				default:
				}
			}
			if mode == "accepted" || mode == "hook-accepted" {
				secondCtx, stopSecond := context.WithCancel(context.Background())
				second := make(chan error, 1)
				go func() {
					_, err := askWithContext(t, secondCtx, a, ws, "chat-C", "chat-B", "Second question")
					second <- err
				}()
				waitQueued(t, s, 1)
				stopSecond()
				if err := <-second; err == nil {
					t.Fatal("additional question did not stay queued")
				}
				waitQueued(t, s, 0)
				if _, _, _, err := a.askTarget(applet.WithInvocation(context.Background(), applet.Invocation{Workspace: ws, OwnerKind: "chat", OwnerID: "chat-B"}), "chat-C"); err == nil {
					t.Fatal("steered recipient asked a third chat")
				}
			}
			close(c.release)
			select {
			case <-finished:
			case <-time.After(testWait):
				t.Fatal("turn did not settle")
			}
			if mode != "uncertain" && mode != "cancel-pending" {
				r := <-done
				want := "Answer to: Question during work?"
				if mode == "accepted" || mode == "hook-accepted" {
					want = "Steered answer"
				}
				if r.err != nil || r.out["reply"] != want {
					t.Fatalf("reply %v, error %v, want %q", r.out, r.err, want)
				}
			}
			calls := append([]providerCall(nil), base.testProviderConnection.calls...)
			prompts := 0
			for _, call := range calls {
				if call.method == "session/cancel" {
					t.Fatal("steering cancelled work")
				}
				if call.method == "session/prompt" {
					prompts++
				}
			}
			wantPrompts := 2
			if mode == "accepted" || mode == "hook-accepted" || mode == "uncertain" || mode == "cancel-pending" {
				wantPrompts = 1
			}
			if prompts != wantPrompts {
				t.Fatalf("%d prompts, want %d", prompts, wantPrompts)
			}
		})
	}
}
