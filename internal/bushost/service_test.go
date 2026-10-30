package bushost

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"studio/internal/bus"
	"sync"
	"testing"
	"time"
)

func testOptions(t *testing.T) Options {
	return Options{State: t.TempDir(), Labels: func() map[string]string { return nil }, Reconcile: func(context.Context, *bus.Bus, map[string]bool) {}, Deliver: func(context.Context, *bus.Bus) {}, Wake: func(context.Context, *bus.Bus) {}, Emit: func(string, any) {}}
}

// A workspace in the window is on the bus before any chat starts in it, so a
// message to it waits for its first chat rather than failing.
func TestAWorkspaceInTheWindowIsOnTheBus(t *testing.T) {
	labelNow := "Roadmap"
	folder := filepath.Join(t.TempDir(), "roadmap")
	if err := os.MkdirAll(folder, 0o755); err != nil {
		t.Fatal(err)
	}
	o := testOptions(t)
	o.Labels = func() map[string]string { return map[string]string{folder: labelNow} }
	s := New(o)
	defer s.CloseHandle()

	err := s.With(func(b *bus.Bus) error {
		s.round(context.Background(), b)
		_, err := b.Send(context.Background(), bus.Sender{}, "roadmap", "Plan the next release.", nil, 0)
		return err
	})
	if err != nil {
		t.Fatalf("a message to a workspace with no chat yet failed: %v", err)
	}
	if s.known[folder] != "Roadmap" {
		t.Fatal("the workspace was not recorded as on the bus")
	}
	// A renamed workspace is labelled again at the watcher's next look.
	labelNow = "Roadmap 2027"
	var label string
	err = s.With(func(b *bus.Bus) error {
		s.round(context.Background(), b)
		ws, err := b.Workspaces(context.Background())
		if len(ws) == 1 {
			label = ws[0].Display()
		}
		return err
	})
	if err != nil || label != "Roadmap 2027" {
		t.Fatalf("the workspace is shown as %q: %v", label, err)
	}
}

func TestWatcherCallbacksAndTwoPhaseShutdown(t *testing.T) {
	o := testOptions(t)
	var s *Service
	entered, release := make(chan struct{}), make(chan struct{})
	var order []string
	free := func() {
		if !s.mu.TryLock() {
			t.Error("callback holds service lock")
		} else {
			s.mu.Unlock()
		}
	}
	o.Labels = func() map[string]string { free(); order = append(order, "labels"); return nil }
	o.Reconcile = func(context.Context, *bus.Bus, map[string]bool) {
		free()
		order = append(order, "reconcile")
		close(entered)
		<-release
	}
	o.Emit = func(string, any) { free(); order = append(order, "changed") }
	o.Deliver = func(context.Context, *bus.Bus) { free(); order = append(order, "deliver") }
	o.Wake = func(context.Context, *bus.Bus) { free(); order = append(order, "wake") }
	s = New(o)
	defer s.CloseHandle()
	s.StartWatcher(context.Background())
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("watcher did not run")
	}
	stopped := make(chan struct{})
	go func() { s.StopWatcher(); close(stopped) }()
	select {
	case <-s.stop:
	case <-time.After(5 * time.Second):
		t.Fatal("stop was not signalled")
	}
	select {
	case <-stopped:
		t.Fatal("stop did not join callback")
	default:
	}
	close(release)
	select {
	case <-stopped:
	case <-time.After(5 * time.Second):
		t.Fatal("stop did not finish")
	}
	if !reflect.DeepEqual(order, []string{"labels", "reconcile", "changed", "deliver", "wake"}) {
		t.Fatal(order)
	}
	done := s.done
	s.StartWatcher(context.Background())
	if done != s.done {
		t.Fatal("watcher restarted")
	}
	b, err := s.Bus()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = b.ChangeCounter(context.Background()); err != nil {
		t.Fatal("stop closed handle", err)
	}
	s.CloseHandle()
	if _, err = s.Bus(); err == nil {
		t.Fatal("handle reopened")
	}
}

func TestQuestionPostingAndOrphanWithdrawalAreSerialized(t *testing.T) {
	s := New(testOptions(t))
	defer s.CloseHandle()
	b, err := s.Bus()
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	ws := t.TempDir()
	if _, err = b.RegisterSession(ctx, "from", ws, "From"); err != nil {
		t.Fatal(err)
	}
	if _, err = b.RegisterSession(ctx, "to", ws, "To"); err != nil {
		t.Fatal(err)
	}
	sender, err := b.Session(ctx, "from")
	if err != nil {
		t.Fatal(err)
	}
	orphan, err := b.Ask(ctx, sender, "to", "orphan")
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	ids := make(chan int64, 12)
	for i := 0; i < 12; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			id, err := s.PostQuestion(ctx, "from", "to", "question")
			if err != nil {
				t.Error(err)
			} else {
				ids <- id
			}
		}()
		go func() { defer wg.Done(); s.WithdrawOrphans(ctx, b) }()
	}
	wg.Wait()
	close(ids)
	s.WithdrawOrphans(ctx, b)
	waiting, err := b.WaitingQuestions(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(waiting) != 12 {
		t.Fatalf("active question was withdrawn: %v", waiting)
	}
	for _, id := range waiting {
		if id == orphan.ID {
			t.Fatal("orphan remained")
		}
	}
	for id := range ids {
		s.DropQuestion(id)
	}
	waiting, err = b.WaitingQuestions(ctx)
	if err != nil || len(waiting) != 0 {
		t.Fatal("dropped questions remain", waiting, err)
	}
}
