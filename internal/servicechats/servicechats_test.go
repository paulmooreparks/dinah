package servicechats

import (
	"context"
	"errors"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// fakeBackend takes turns that last until their context ends or the test
// lets them go.
type fakeBackend struct {
	Backend
	turns   atomic.Int32
	entered chan string
	release chan struct{}
	mu      sync.Mutex
	changes []Change
	opened  []bool
}

func newFakeBackend() *fakeBackend {
	return &fakeBackend{entered: make(chan string, 8), release: make(chan struct{})}
}

func (b *fakeBackend) TakeTurn(ctx context.Context, service, chat string, prompt Prompt, opened bool) (Reply, error) {
	b.turns.Add(1)
	b.mu.Lock()
	b.opened = append(b.opened, opened)
	b.mu.Unlock()
	b.entered <- service + "/" + chat
	select {
	case <-ctx.Done():
		return Reply{Text: "partial", StopReason: "cancelled"}, nil
	case <-b.release:
		return Reply{Text: "answer to " + prompt.Text, StopReason: "end_turn"}, nil
	}
}

func (b *fakeBackend) PublishChange(_ context.Context, change Change) error {
	b.mu.Lock()
	b.changes = append(b.changes, change)
	b.mu.Unlock()
	return nil
}

func (b *fakeBackend) ReleaseChat(context.Context, string, string) error { return nil }

func TestPortActsUnderItsRegisteredName(t *testing.T) {
	b := newFakeBackend()
	w := New(b)
	p := w.ForService("sample")
	if err := p.PublishChange(context.Background(), "folder", "record-1"); err != nil {
		t.Fatal(err)
	}
	if len(b.changes) != 1 || b.changes[0] != (Change{Service: "sample", Workspace: "folder", Key: "record-1"}) {
		t.Fatalf("change: %+v", b.changes)
	}
	if p.Service() != "sample" {
		t.Fatal(p.Service())
	}
}

func TestTurnReturnsTheReplyAndSaysWhichKindOfChat(t *testing.T) {
	b := newFakeBackend()
	w := New(b)
	p := w.ForService("sample")
	close(b.release)
	reply, err := p.TakeTurn(context.Background(), "chat-a", Prompt{Text: "one", Label: "Sample"})
	if err != nil || reply.Text != "answer to one" || reply.StopReason != "end_turn" {
		t.Fatalf("%+v %v", reply, err)
	}
	if _, err := p.TakeTurnInExistingChat(context.Background(), "chat-b", Prompt{Text: "two"}); err != nil {
		t.Fatal(err)
	}
	if len(b.opened) != 2 || !b.opened[0] || b.opened[1] {
		t.Fatalf("opened flags: %v", b.opened)
	}
	if _, err := p.TakeTurn(context.Background(), "chat-a", Prompt{Text: "  "}); err == nil {
		t.Fatal("an empty prompt was taken")
	}
	if w.Running() != 0 {
		t.Fatal("a call was left counted")
	}
}

func TestSecondTurnInAChatIsRefusedAndNeverReachesTheBackend(t *testing.T) {
	b := newFakeBackend()
	w := New(b)
	p := w.ForService("sample")
	done := make(chan error, 1)
	go func() {
		_, err := p.TakeTurn(context.Background(), "chat-a", Prompt{Text: "first"})
		done <- err
	}()
	<-b.entered
	if _, err := p.TakeTurn(context.Background(), "chat-a", Prompt{Text: "second"}); !errors.Is(err, ErrBusy) {
		t.Fatalf("second turn: %v", err)
	}
	if err := p.ReleaseChat(context.Background(), "chat-a"); !errors.Is(err, ErrBusy) {
		t.Fatalf("release during a turn: %v", err)
	}
	if b.turns.Load() != 1 {
		t.Fatalf("the backend saw %d turns", b.turns.Load())
	}
	close(b.release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestCancelTurnStopsOnlyThisServicesTurn(t *testing.T) {
	b := newFakeBackend()
	w := New(b)
	p, other := w.ForService("sample"), w.ForService("other")
	type result struct {
		reply Reply
		err   error
	}
	done := make(chan result, 1)
	go func() {
		reply, err := p.TakeTurn(context.Background(), "chat-a", Prompt{Text: "first"})
		done <- result{reply, err}
	}()
	<-b.entered
	if requested, err := other.CancelTurn(context.Background(), "chat-a"); requested || err != nil {
		t.Fatalf("another service cancelled the turn: %v %v", requested, err)
	}
	if requested, err := p.CancelTurn(context.Background(), "chat-b"); requested || err != nil {
		t.Fatalf("a chat with no turn: %v %v", requested, err)
	}
	requested, err := p.CancelTurn(context.Background(), "chat-a")
	if !requested || err != nil {
		t.Fatalf("cancel: %v %v", requested, err)
	}
	// CancelTurn returned, so the turn has returned too.
	select {
	case r := <-done:
		if r.err != nil || r.reply.StopReason != "cancelled" || r.reply.Text != "partial" {
			t.Fatalf("%+v", r)
		}
	case <-time.After(time.Second):
		t.Fatal("the cancelled turn had not returned")
	}
}

func TestCancelledContextStopsTheTurn(t *testing.T) {
	b := newFakeBackend()
	w := New(b)
	p := w.ForService("sample")
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		reply, err := p.TakeTurn(ctx, "chat-a", Prompt{Text: "first"})
		if reply.StopReason != "cancelled" {
			t.Errorf("stop reason %q", reply.StopReason)
		}
		done <- err
	}()
	<-b.entered
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled turn: %v", err)
	}
	if _, err := p.TakeTurn(ctx, "chat-a", Prompt{Text: "again"}); !errors.Is(err, context.Canceled) {
		t.Fatalf("a finished context was admitted: %v", err)
	}
	if b.turns.Load() != 1 {
		t.Fatal("a turn was taken with a finished context")
	}
}

func TestShutdownClosesAdmissionCancelsTurnsAndLeavesNoGoroutine(t *testing.T) {
	before := runtime.NumGoroutine()
	b := newFakeBackend()
	w := New(b)
	p := w.ForService("sample")
	errs := make(chan error, 2)
	for _, chat := range []string{"chat-a", "chat-b"} {
		go func() {
			_, err := p.TakeTurn(context.Background(), chat, Prompt{Text: "work"})
			errs <- err
		}()
	}
	<-b.entered
	<-b.entered
	if w.Running() != 2 {
		t.Fatalf("running %d", w.Running())
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := w.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := <-errs; !errors.Is(err, ErrClosed) {
			t.Fatalf("a turn stopped by shutdown: %v", err)
		}
	}
	if w.Running() != 0 {
		t.Fatal("calls remained after shutdown")
	}
	if _, err := p.TakeTurn(context.Background(), "chat-a", Prompt{Text: "late"}); !errors.Is(err, ErrClosed) {
		t.Fatalf("a turn after shutdown: %v", err)
	}
	if _, err := p.OpenChat(context.Background(), OpenRequest{Workspace: "folder", Program: "program"}); !errors.Is(err, ErrClosed) {
		t.Fatalf("an opening after shutdown: %v", err)
	}
	if err := p.PublishChange(context.Background(), "folder", "key"); !errors.Is(err, ErrClosed) {
		t.Fatalf("a notice after shutdown: %v", err)
	}
	if b.turns.Load() != 2 {
		t.Fatal("a refused call reached the backend")
	}
	// A second shutdown is harmless.
	if err := w.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for runtime.NumGoroutine() > before && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if now := runtime.NumGoroutine(); now > before {
		t.Fatalf("%d goroutines before, %d after shutdown", before, now)
	}
}

// stubbornBackend ignores the cancellation of its context.
type stubbornBackend struct {
	Backend
	entered chan struct{}
	release chan struct{}
}

func (b *stubbornBackend) TakeTurn(context.Context, string, string, Prompt, bool) (Reply, error) {
	close(b.entered)
	<-b.release
	return Reply{StopReason: "end_turn"}, nil
}

func TestShutdownReportsCallsThatOutliveItsBudget(t *testing.T) {
	b := &stubbornBackend{entered: make(chan struct{}), release: make(chan struct{})}
	w := New(b)
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = w.ForService("sample").TakeTurn(context.Background(), "chat-a", Prompt{Text: "work"})
	}()
	<-b.entered
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if err := w.Shutdown(ctx); err == nil {
		t.Fatal("shutdown claimed that a running call had returned")
	}
	close(b.release)
	<-done
	if err := w.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestZeroPortRefusesEveryCall(t *testing.T) {
	var p Port
	if _, err := p.TakeTurn(context.Background(), "chat-a", Prompt{Text: "x"}); !errors.Is(err, ErrClosed) {
		t.Fatal(err)
	}
	if _, err := p.OpenChat(context.Background(), OpenRequest{Workspace: "w", Program: "p"}); !errors.Is(err, ErrClosed) {
		t.Fatal(err)
	}
	if _, err := p.CancelTurn(context.Background(), "chat-a"); !errors.Is(err, ErrClosed) {
		t.Fatal(err)
	}
	if err := p.ReleaseChat(context.Background(), "chat-a"); !errors.Is(err, ErrClosed) {
		t.Fatal(err)
	}
}
