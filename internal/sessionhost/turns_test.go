package sessionhost

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"studio/internal/acp"
)

func rejectsReleased(t *testing.T, f func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Error("invalid ownership was accepted")
		}
	}()
	f()
}

func TestSubmissionContentionAndErrorRelease(t *testing.T) {
	var mu sync.Mutex
	turns := NewTurns(&mu)
	p := NewPrompt("question", nil).WithReply(context.Background(), &mu)
	func() {
		gate := turns.Begin()
		defer gate.Release()
		gate.RegisterNative(1, 1, p)
		if other := turns.TryBegin(); other != nil {
			other.Release()
			t.Fatal("busy gate admitted another submission")
		}
		// An error return releases the gate without completing or cancelling.
	}()
	select {
	case <-p.reply.done:
		t.Fatal("release settled a reply")
	default:
	}
	if p.Cancelled() {
		t.Fatal("release cancelled a reply")
	}
	gate := turns.TryBegin()
	if gate == nil {
		t.Fatal("error path retained gate")
	}
	gate.Release()
	gate.Release()
	rejectsReleased(t, func() { gate.Pending() })
	rejectsReleased(t, func() { gate.RegisterHook(2, 2, "", p) })
	other := turns.Begin()
	defer other.Release()
	if len(other.Pending()) != 1 {
		t.Fatal("release lost pending work")
	}
}

func TestReceiptConfirmsWhileGateHeldAndAfterRelease(t *testing.T) {
	for _, deferred := range []bool{false, true} {
		var mu sync.Mutex
		turns := NewTurns(&mu)
		gate := turns.Begin()
		p := NewPrompt("question", nil).WithReply(context.Background(), &mu)
		r := gate.RegisterNative(1, 1, p)
		if deferred {
			gate.Release()
		}
		// The native callback must not acquire the gate, even synchronously.
		done := make(chan struct{})
		var winners atomic.Int32
		go func() {
			var wg sync.WaitGroup
			for i := 0; i < 20; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					confirmed, ok := r.Confirm()
					if ok {
						if !confirmed.SameReply(p) {
							t.Error("receipt changed reply identity")
						}
						winners.Add(1)
					}
				}()
			}
			wg.Wait()
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("confirmation waited for the gate")
		}
		gate.Release()
		if winners.Load() != 1 || !r.Delivered() {
			t.Fatalf("confirmation winners: %d", winners.Load())
		}
	}
}

func TestCompletionReleasesBeforeWaitersAndSettlesOnce(t *testing.T) {
	var mu sync.Mutex
	turns := NewTurns(&mu)
	current := NewPrompt("current", nil).WithReply(context.Background(), &mu)
	asked := NewPrompt("steer", nil).WithReply(context.Background(), &mu)
	gate := turns.Begin()
	mu.Lock()
	turns.ReserveLocked(asked)
	turns.MarkDeliveredLocked(asked.AsLateSteer(1, 2))
	completion := turns.DetachReplyLocked()
	mu.Unlock()
	gate.Finish(completion, current, nil, End{StopReason: "end_turn"})
	gate.Finish(completion, current, nil, End{Err: errors.New("duplicate")})
	gate.Release()
	if !mu.TryLock() {
		t.Fatal("completion retained session lock")
	}
	mu.Unlock()
	other := turns.TryBegin()
	if other == nil {
		t.Fatal("completion retained gate")
	}
	other.Release()
	// The caller emits agent:turn after Finish. Both waiters must already be
	// ready there, irrespective of when their goroutines get scheduled.
	for _, p := range []Prompt{asked, current} {
		select {
		case <-p.reply.done:
		default:
			t.Fatal("turn event would precede waiter settlement")
		}
		if p.reply.end.Err != nil || p.reply.end.StopReason != "end_turn" {
			t.Fatal("duplicate completion changed result")
		}
	}
}

func TestFallbackKeepsOriginalReplyUntilItsTurn(t *testing.T) {
	var mu sync.Mutex
	turns := NewTurns(&mu)
	asked := NewPrompt("question", nil).WithReply(context.Background(), &mu)
	gate := turns.Begin()
	mu.Lock()
	turns.ReserveLocked(asked)
	completion := turns.DetachReplyLocked()
	mu.Unlock()
	late := asked.AsLateSteer(1, 2)
	gate.Finish(completion, NewPrompt("current", nil), []Prompt{late}, End{})
	select {
	case <-asked.reply.done:
		t.Fatal("undelivered fallback settled early")
	default:
	}
	late.Resolve(End{StopReason: "end_turn"})
	select {
	case <-asked.reply.done:
	default:
		t.Fatal("fallback lost waiter")
	}
}

func TestCompletionUnlocksBeforeResolvingAskedThenCurrent(t *testing.T) {
	var mu sync.Mutex
	turns := NewTurns(&mu)
	asked := NewPrompt("asked", nil).WithReply(context.Background(), &mu)
	current := NewPrompt("current", nil).WithReply(context.Background(), &mu)
	// Hold the current reply's once operation to observe Finish between the
	// two waiter resolutions, without adding production scheduling hooks.
	held, release := make(chan struct{}), make(chan struct{})
	go current.reply.once.Do(func() { close(held); <-release })
	<-held
	gate := turns.Begin()
	mu.Lock()
	turns.ReserveLocked(asked)
	turns.MarkDeliveredLocked(asked)
	completion := turns.DetachReplyLocked()
	mu.Unlock()
	done := make(chan struct{})
	go func() { gate.Finish(completion, current, nil, End{}); close(done) }()
	t.Cleanup(func() {
		close(release)
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Error("Finish did not return")
		}
	})
	select {
	case <-asked.reply.done:
	case <-time.After(time.Second):
		t.Fatal("asked waiter did not resolve before current")
	}
	if !mu.TryLock() {
		t.Fatal("waiter resolved while session locked")
	}
	mu.Unlock()
	other := turns.TryBegin()
	if other == nil {
		t.Fatal("waiter resolved while steering gate locked")
	}
	other.Release()
	select {
	case <-done:
		t.Fatal("current resolution was bypassed")
	default:
	}
}

func TestPromptCopiesAndPendingViewsDoNotExposeMutableState(t *testing.T) {
	var mu sync.Mutex
	turns := NewTurns(&mu)
	blocks := []acp.ContentBlock{{Type: "resource", Resource: &acp.EmbeddedResource{Text: "original"}}}
	p := NewPrompt("original", blocks)
	blocks[0].Resource.Text = "input mutation"
	view := p.Blocks()
	view[0].Resource.Text = "output mutation"
	if p.Blocks()[0].Resource.Text != "original" {
		t.Fatal("content aliases caller storage")
	}
	gate := turns.Begin()
	defer gate.Release()
	gate.RegisterHook(1, 2, "original", p)
	pending := gate.Pending()
	*pending[0].Fallback = NewPrompt("replacement", nil)
	pending[0].ID = 99
	again := gate.Pending()
	if again[0].ID != 1 || again[0].Fallback.Text() != "original" {
		t.Fatal("pending view changed private state")
	}
}

func TestReplyCaptureUsesInjectedMutexAndOriginalIdentity(t *testing.T) {
	var mu sync.Mutex
	turns := NewTurns(&mu)
	p := NewPrompt("question", nil).WithReply(context.Background(), &mu)
	mu.Lock()
	turns.StartCaptureLocked(&p, true)
	turns.AppendCaptureLocked("narration")
	turns.ResetCaptureLocked()
	turns.AppendCaptureLocked("answer")
	mu.Unlock()
	p.Resolve(End{StopReason: "end_turn"})
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	words, end, completed := p.Wait(ctx)
	if words != "answer" || end.Err != nil || !completed {
		t.Fatalf("reply: %q %+v %v", words, end, completed)
	}
	var otherMu sync.Mutex
	other := NewTurns(&otherMu)
	rejectsReleased(t, func() { other.ReserveLocked(p) })
}
