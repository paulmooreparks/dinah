// Package bushost owns the desktop bus handle and message watcher.
package bushost

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"studio/internal/bus"
)

//go:embed service.go
var ServiceSource []byte

type Options struct {
	State     string
	Labels    func() map[string]string
	Reconcile func(context.Context, *bus.Bus, map[string]bool)
	Deliver   func(context.Context, *bus.Bus)
	Wake      func(context.Context, *bus.Bus)
	// Notice, when set, is called after the change counter has risen, with the
	// bus, to find what the person should be told of (mention_notices.go).
	Notice func(context.Context, *bus.Bus)
	Emit   func(string, any)
	Log    func(string, string, ...any)
}

type Service struct {
	options    Options
	mu         sync.Mutex
	handle     *bus.Bus
	closed     bool
	stopped    bool
	stop, done chan struct{}
	qmu        sync.Mutex
	questions  map[int64]bool
	// These caches are owned by the single watcher.
	known    map[string]string
	reported map[string]bool
	seen     int64
}

func New(o Options) *Service {
	return &Service{options: o, known: map[string]string{}, reported: map[string]bool{}, seen: -1}
}
func (s *Service) Bus() (*bus.Bus, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil, errors.New("the window bus is closed")
	}
	if s.handle != nil {
		return s.handle, nil
	}
	b, err := bus.OpenAt(filepath.Join(s.options.State, "bus.db"))
	if err != nil {
		return nil, err
	}
	s.handle = b
	return b, nil
}
func (s *Service) With(f func(*bus.Bus) error) error {
	b, err := s.Bus()
	if err != nil {
		return err
	}
	return f(b)
}
func (s *Service) StartWatcher(ctx context.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopped || s.closed || s.done != nil {
		return
	}
	s.stop, s.done = make(chan struct{}), make(chan struct{})
	go s.watch(ctx, s.stop, s.done)
}

// StopWatcher joins its current round but leaves the handle available for
// session departure and question cleanup. Callbacks are not cancellable here.
func (s *Service) StopWatcher() {
	s.mu.Lock()
	if !s.stopped {
		s.stopped = true
		if s.stop != nil {
			close(s.stop)
		}
	}
	done := s.done
	s.mu.Unlock()
	if done != nil {
		<-done
	}
}
func (s *Service) CloseHandle() {
	s.StopWatcher()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	if s.handle != nil {
		_ = s.handle.Close()
		s.handle = nil
	}
}
func (s *Service) log(level, format string, args ...any) {
	if s.options.Log != nil {
		s.options.Log(level, format, args...)
	}
}
func (s *Service) watch(ctx context.Context, stop, done chan struct{}) {
	defer close(done)
	b, err := s.Bus()
	if err != nil {
		return
	}
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-stop:
			return
		case <-tick.C:
		}
		s.round(ctx, b)
	}
}
func (s *Service) round(ctx context.Context, b *bus.Bus) {
	for path, label := range s.options.Labels() {
		if previous, ok := s.known[path]; ok && previous == label {
			continue
		}
		if _, err := b.LabelWorkspace(ctx, path, label); err != nil {
			s.log("error", "putting workspace %s on the bus: %v", path, err)
			continue
		}
		s.known[path] = label
	}
	s.options.Reconcile(ctx, b, s.reported)
	if n, err := b.ChangeCounter(ctx); err == nil && n != s.seen {
		s.seen = n
		s.options.Emit("bus:changed", map[string]int64{"counter": n})
		if s.options.Notice != nil {
			s.options.Notice(ctx, b)
		}
	}
	s.options.Deliver(ctx, b)
	s.WithdrawOrphans(ctx, b)
	s.options.Wake(ctx, b)
}
func (s *Service) PostQuestion(ctx context.Context, from, to, question string) (int64, error) {
	s.qmu.Lock()
	defer s.qmu.Unlock()
	var id int64
	err := s.With(func(b *bus.Bus) error {
		sender, err := b.Session(ctx, from)
		if err != nil {
			return err
		}
		m, err := b.Ask(ctx, sender, to, question)
		id = m.ID
		return err
	})
	if err != nil {
		return 0, fmt.Errorf("sending the question over the bus: %w", err)
	}
	if s.questions == nil {
		s.questions = map[int64]bool{}
	}
	s.questions[id] = true
	return id, nil
}

// HasQuestion reports whether this host still owns a wait for the question.
func (s *Service) HasQuestion(id int64) bool {
	s.qmu.Lock()
	defer s.qmu.Unlock()
	return s.questions[id]
}

func (s *Service) DropQuestion(id int64) {
	s.qmu.Lock()
	delete(s.questions, id)
	err := s.With(func(b *bus.Bus) error { return b.WithdrawQuestion(context.Background(), id) })
	s.qmu.Unlock()
	if err != nil {
		s.log("error", "withdrawing question %d: %v", id, err)
	}
}
func (s *Service) WithdrawOrphans(ctx context.Context, b *bus.Bus) {
	s.qmu.Lock()
	ids, err := b.WaitingQuestions(ctx)
	if err != nil {
		s.qmu.Unlock()
		s.log("error", "looking for questions nobody waits for: %v", err)
		return
	}
	type problem struct {
		id  int64
		err error
	}
	var problems []problem
	for _, id := range ids {
		if s.questions[id] {
			continue
		}
		if err := b.WithdrawQuestion(ctx, id); err != nil {
			problems = append(problems, problem{id, err})
		}
	}
	s.qmu.Unlock()
	for _, p := range problems {
		s.log("error", "withdrawing question %d: %v", p.id, p.err)
	}
}
