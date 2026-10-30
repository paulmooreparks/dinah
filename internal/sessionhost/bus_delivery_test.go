package sessionhost

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"studio/internal/bus"
	"studio/internal/bushost"
)

func TestBusAwaitReadinessAndFailurePrecedeCancellation(t *testing.T) {
	var mu sync.Mutex
	var runtime Runtime
	c := NewConversation(Identity{ID: "chat"}, InitialActivity{Open: true}, false, false)
	c.JoinBus("chat-bus")
	runtime.PutConversationLocked(c.ID(), c)
	owner := NewBusChats(&mu, &runtime, BusChatsHost{})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	now := time.Now()
	if got, err := owner.AwaitAskable(ctx, c.ID(), now); got != c || err != nil {
		t.Fatalf("ready chat lost to cancellation: %v, %v", got, err)
	}
	expected := errors.New("connection failed")
	runtime.CompleteWakeLocked("missing", expected, now)
	if _, err := owner.AwaitAskable(ctx, "missing", now); !errors.Is(err, expected) {
		t.Fatalf("connection failure lost to cancellation: %v", err)
	}
	if _, err := owner.AwaitAskable(ctx, "unknown", now); !errors.Is(err, context.Canceled) {
		t.Fatalf("unready chat did not cancel: %v", err)
	}
}

func TestBusAskRegistersBeforeAwaitAndWithdrawsOnCancellation(t *testing.T) {
	var mu sync.Mutex
	var runtime Runtime
	service := bushost.New(bushost.Options{State: t.TempDir()})
	t.Cleanup(service.CloseHandle)
	b, err := service.Bus()
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"caller", "target"} {
		if _, err = b.RegisterSession(context.Background(), id, t.TempDir(), id); err != nil {
			t.Fatal(err)
		}
	}
	caller := NewConversation(Identity{ID: "caller"}, InitialActivity{Open: true}, false, false)
	caller.JoinBus("caller")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var registered int64
	accesses := 0
	owner := NewBusChats(&mu, &runtime, BusChatsHost{
		Clock:         time.Now,
		EnsureBusChat: func(target BusAskTarget) (string, error) { return target.SessionID, nil },
		BusService: func() *bushost.Service {
			accesses++
			if accesses == 2 {
				ids, err := b.WaitingQuestions(context.Background())
				if err != nil || len(ids) != 1 {
					t.Fatalf("post did not precede defer setup: %v, %v", ids, err)
				}
				registered = ids[0]
				if !service.HasQuestion(registered) {
					t.Fatal("committed question not registered")
				}
				cancel()
			}
			return service
		},
	})
	_, end := owner.Ask(ctx, caller, BusAskTarget{SessionID: "target"}, "label", "question")
	if !errors.Is(end.Err, context.Canceled) || end.StopReason != "error" {
		t.Fatalf("result = %+v", end)
	}
	if registered == 0 || service.HasQuestion(registered) {
		t.Fatal("deferred withdrawal did not clear registration")
	}
	ids, err := b.WaitingQuestions(context.Background())
	if err != nil || len(ids) != 0 {
		t.Fatalf("deferred withdrawal left questions: %v, %v", ids, err)
	}
}

func TestBusDeliveryPreparationFailureRestoresWorkAndReleasesGate(t *testing.T) {
	var mu sync.Mutex
	var runtime Runtime
	service := bushost.New(bushost.Options{State: t.TempDir()})
	t.Cleanup(service.CloseHandle)
	b, err := service.Bus()
	if err != nil {
		t.Fatal(err)
	}
	row, err := b.RegisterSession(context.Background(), "target", t.TempDir(), "Target")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = b.Send(context.Background(), bus.Sender{}, row.Address(), "Message", nil, 0); err != nil {
		t.Fatal(err)
	}
	c := NewConversation(Identity{ID: "chat"}, InitialActivity{Open: true}, false, false)
	c.JoinBus("target")
	expected := errors.New("preparation failed")
	prepared, logged := false, false
	owner := NewBusChats(&mu, &runtime, BusChatsHost{
		HoldingTurns: func() error { return nil },
		AgentPerson:  func() string { return "Person" },
		BusPictures:  func(*Conversation, []bus.Message) ([]Attachment, string) { return nil, "" },
		PreparePrompt: func(s *Conversation, text string, pics []Attachment) (Prompt, error) {
			if !s.Working() {
				t.Fatal("delivery did not claim idle work before preparing")
			}
			if gate := s.TrySubmission(); gate != nil {
				gate.Release()
				t.Fatal("preparation escaped gate")
			}
			if !mu.TryLock() {
				t.Fatal("preparation retained runtime lock")
			}
			mu.Unlock()
			prepared = true
			return Prompt{}, expected
		},
		Emit: func(string, any) { t.Fatal("failed preparation emitted successful delivery") },
		Log: func(string, string, ...any) {
			if c.Working() {
				t.Fatal("logging preceded work restoration")
			}
			if gate := c.TrySubmission(); gate != nil {
				gate.Release()
				t.Fatal("failure released gate before logging")
			}
			logged = true
		},
	})
	owner.Ring(context.Background(), b, c)
	if !prepared || !logged || c.Working() {
		t.Fatal("delivery failure skipped restoration")
	}
	gate := c.TrySubmission()
	if gate == nil {
		t.Fatal("delivery retained gate after returning")
	}
	gate.Release()
}
