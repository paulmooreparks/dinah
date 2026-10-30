package sessionhost

import (
	"context"
	"errors"
	"reflect"
	"studio/internal/acp"
	"sync"
	"testing"
)

func turnFixture() (*TurnIO, *Conversation) {
	c := NewConversation(Identity{ID: "chat", Provider: "devin"}, InitialActivity{Open: true, Working: true}, false, false)
	c.JoinBus("bus-chat")
	conn := &lifecycleConnection{done: make(chan struct{}), initialize: func(context.Context) error { return nil }}
	h := TurnHost{
		LookupConversation: func(string) *Conversation { return c }, SessionConnection: func(*Conversation) Connection { return conn },
		HoldingTurns: func() error { return nil }, Unpark: func(*Conversation) {}, Journal: func(string, string) {},
		RecordPrompt: func(*Conversation, *Prompt) {}, LogPrompt: func(string, string, string) {},
		RecordDelivered:        func(*Conversation, string, []ConversationMessage) {},
		PrepareProviderApplets: func(string, string) error { return nil }, AppletPromptContext: func(string) (string, error) { return "context", nil },
		PendingContinuationContext: func(*Conversation) (string, error) { return "handoff", nil }, ConsumeContinuationContext: func(*Conversation) {},
		BeginHookTurn: func(string, string) error { return nil }, EndHookTurn: func(string, string) error { return nil },
		Call: func(_ Connection, _ string, _ any, out any) error {
			out.(*acp.PromptResult).StopReason = "end_turn"
			return nil
		},
		FinishTranscript: func(*Conversation, error, string) {}, DisconnectSubagents: func(*Conversation) {},
		BusAfterTurn: func(*Conversation) *Prompt { return nil }, BusWaiting: func(*Conversation) (string, []ConversationMessage) { return "", nil },
		Emit: func(string, any) {}, VisibleConversationText: func(s string) string { return s },
		LogError: func(string, string, ...any) {}, SteeringState: func(string, int64, string) {},
	}
	return NewTurnIO(h), c
}

func TestTurnIOBlockedCallAndHookClosePreserveGateAndWaiterOrder(t *testing.T) {
	for _, closeFails := range []bool{false, true} {
		t.Run(map[bool]string{false: "success", true: "hook-close-error"}[closeFails], func(t *testing.T) {
			io, c := turnFixture()
			p := c.WithReply(context.Background(), NewPrompt("question", nil))
			callEntered, callRelease := make(chan struct{}), make(chan struct{})
			closeEntered, closeRelease := make(chan struct{}), make(chan struct{})
			var order []string
			io.host.Call = func(_ Connection, method string, _ any, out any) error {
				if method != "session/prompt" {
					t.Error(method)
				}
				order = append(order, "call")
				close(callEntered)
				<-callRelease
				out.(*acp.PromptResult).StopReason = "end_turn"
				return nil
			}
			io.host.ConsumeContinuationContext = func(*Conversation) { order = append(order, "consume") }
			hookErr := errors.New("hook close failed")
			io.host.EndHookTurn = func(string, string) error {
				order = append(order, "hook-close")
				close(closeEntered)
				<-closeRelease
				if closeFails {
					return hookErr
				}
				return nil
			}
			io.host.FinishTranscript = func(_ *Conversation, err error, _ string) {
				order = append(order, "transcript")
				if errors.Is(err, hookErr) != closeFails {
					t.Errorf("transcript error %v", err)
				}
				if gate := c.TrySubmission(); gate != nil {
					gate.Release()
					t.Error("transcript finalized outside gate")
				}
			}
			io.host.DisconnectSubagents = func(*Conversation) { order = append(order, "subagents") }
			io.host.BusAfterTurn = func(*Conversation) *Prompt { order = append(order, "bus"); return nil }
			io.host.Emit = func(name string, _ any) {
				order = append(order, name)
				if name == "agent:turn" {
					select {
					case <-p.reply.done:
					default:
						t.Error("turn event preceded waiter settlement")
					}
					gate := c.TrySubmission()
					if gate == nil {
						t.Error("turn event retained gate")
					} else {
						gate.Release()
					}
				}
			}
			done := make(chan struct{})
			go func() { defer close(done); io.Run(c, &p) }()
			receiveLifecycle(t, callEntered)
			gate := c.TrySubmission()
			if gate == nil {
				t.Fatal("provider call held submission gate")
			}
			gate.Release()
			close(callRelease)
			receiveLifecycle(t, closeEntered)
			if gate := c.TrySubmission(); gate != nil {
				gate.Release()
				t.Fatal("hook close escaped submission gate")
			}
			select {
			case <-p.reply.done:
				t.Fatal("waiter settled during hook close")
			default:
			}
			close(closeRelease)
			receiveLifecycle(t, done)
			want := []string{"call", "consume", "hook-close", "transcript", "subagents"}
			if !closeFails {
				want = append(want, "bus")
			}
			want = append(want, "agent:turn", "agent:session")
			if !reflect.DeepEqual(order, want) {
				t.Fatalf("order %v, want %v", order, want)
			}
		})
	}
}

func TestTurnIOPreparationPrecedenceAndHandoffConsumption(t *testing.T) {
	for _, phase := range []string{"prepare", "nil-connection", "handoff", "call"} {
		t.Run(phase, func(t *testing.T) {
			io, c := turnFixture()
			p := NewPrompt("input", nil).WithLabel("label")
			prepared, consumed := 0, 0
			expected := errors.New(phase)
			io.host.PrepareProviderApplets = func(string, string) error {
				prepared++
				if phase == "prepare" || phase == "nil-connection" {
					return expected
				}
				return nil
			}
			if phase == "nil-connection" {
				io.host.SessionConnection = func(*Conversation) Connection { return nil }
			}
			if phase == "handoff" {
				io.host.PendingContinuationContext = func(*Conversation) (string, error) { return "", expected }
			}
			if phase == "call" {
				io.host.Call = func(Connection, string, any, any) error { return expected }
			}
			io.host.ConsumeContinuationContext = func(*Conversation) { consumed++ }
			var result error
			io.host.FinishTranscript = func(_ *Conversation, err error, _ string) { result = err }
			io.Run(c, &p)
			if prepared != 1 || consumed != 0 {
				t.Fatalf("preparation=%d consumption=%d", prepared, consumed)
			}
			if phase == "nil-connection" {
				if result == nil || result.Error() != "the agent stopped before the turn could start" {
					t.Fatalf("precedence %v", result)
				}
			} else if !errors.Is(result, expected) {
				t.Fatalf("error %v", result)
			}
			gate := c.TrySubmission()
			if gate == nil {
				t.Fatal("error retained gate")
			}
			gate.Release()
		})
	}
}

func TestTurnIONoticeContentionAndWithdrawalRestoration(t *testing.T) {
	io, c := turnFixture()
	gate := c.BeginSubmission()
	defer gate.Release()
	p := NewPrompt("steer", nil)
	gate.RegisterHook(10, 20, "steer", p)
	done := make(chan struct{})
	go func() {
		defer close(done)
		io.NoticeSteers(func(context.Context, int64) (bool, error) {
			t.Error("contended notice reached borrowed bus")
			return false, nil
		}, c)
	}()
	receiveLifecycle(t, done)
	io.host.WithdrawHookSteer = func(int64) (bool, error) { return false, errors.New("database unavailable") }
	if late := io.SettleSteersWithSubmission(c, gate); len(late) != 0 {
		t.Fatal("failed withdrawal queued duplicate")
	}
	if pending := gate.Pending(); len(pending) != 1 || pending[0].ID != 10 {
		t.Fatalf("lost pending ownership %v", pending)
	}
	gate.Release()
	calls := 0
	io.NoticeSteers(func(context.Context, int64) (bool, error) { calls++; return true, nil }, c)
	if calls != 1 {
		t.Fatalf("delivery calls %d", calls)
	}
	next := c.BeginSubmission()
	defer next.Release()
	if len(next.Pending()) != 0 {
		t.Fatal("delivered steer retained")
	}
}

type turnSteerer struct {
	*lifecycleConnection
	submit func(func())
}

func (c *turnSteerer) Steer(_ context.Context, _ acp.PromptParams, confirm func()) error {
	c.submit(confirm)
	return nil
}

func TestTurnIOReceiptConfirmationSurvivesReleaseAndRunsOnce(t *testing.T) {
	for _, deferred := range []bool{false, true} {
		t.Run(map[bool]string{false: "synchronous", true: "deferred"}[deferred], func(t *testing.T) {
			io, _ := turnFixture()
			c := NewConversation(Identity{ID: "chat", Provider: "codex"}, InitialActivity{Open: true, Working: true}, false, false)
			p := c.WithReply(context.Background(), NewPrompt("question", nil))
			c.Reserve(p)
			var mu sync.Mutex
			order := []string{}
			note := func(s string) { mu.Lock(); order = append(order, s); mu.Unlock() }
			var confirm func()
			conn := &turnSteerer{lifecycleConnection: &lifecycleConnection{}, submit: func(fn func()) {
				confirm = fn
				if !deferred {
					fn()
				}
			}}
			io.host.SessionConnection = func(*Conversation) Connection { return conn }
			io.host.SaveSteering = func(*Conversation, string, *DocRef, []Attachment, Prompt) (int64, error) { return 9, nil }
			io.host.SteeringState = func(_ string, _ int64, state string) { note("ledger:" + state) }
			io.host.RecordPrompt = func(*Conversation, *Prompt) { note("transcript") }
			io.host.LogPrompt = func(string, string, string) { note("log") }
			io.host.Remember = func(string, string, string, string, *DocRef) {}
			io.host.Emit = func(name string, v any) {
				if name == "agent:sent" {
					note("sent")
				} else if name == "agent:steer" && v.(map[string]any)["state"] == "accepted" {
					note("accepted")
				}
			}
			gate := c.BeginSubmission()
			if err := io.SubmitSteer(c, gate, p, "question", nil, nil); err != nil {
				gate.Release()
				t.Fatal(err)
			}
			gate.Release()
			var callbacks sync.WaitGroup
			for range 2 {
				callbacks.Add(1)
				go func() { defer callbacks.Done(); confirm() }()
			}
			callbacks.Wait()
			want := []string{"sent", "ledger:delivered", "transcript", "log", "accepted"}
			if !reflect.DeepEqual(order, want) {
				t.Fatalf("receipt effects %v, want %v", order, want)
			}
			if gate := c.TrySubmission(); gate == nil {
				t.Fatal("receipt retained submission gate")
			} else {
				gate.Release()
			}
		})
	}
}
