// Package host owns the window's broker generations without depending on its UI.
package host

import (
	"context"
	_ "embed"
	"errors"
	"sync"
	"time"

	"studio/internal/applet"
	"studio/internal/applet/broker"
)

//go:embed service.go
var Source []byte

type Options struct {
	State      string
	Builtins   func() ([]applet.Bundle, error)
	Extensions func() ([]applet.Bundle, error)
	Revision   func() string
	Emit       func(string, any)
	Consent    func(context.Context, broker.Invocation, applet.Action) error
	Log        func(level, format string, args ...any)
	// Logf takes the entries of a run that is not recorded, by the applet's
	// source (broker.Broker.Logf).
	Logf func(source, level, format string, args ...any)
}

type Service struct {
	options    Options
	mu         sync.Mutex
	current    *broker.Broker
	retired    []*broker.Broker
	revision   string
	generation uint64
	closed     bool
	started    bool
	stop       chan struct{}
	closeDone  chan struct{}
	work       sync.WaitGroup
	watch      func(*broker.Broker) error
	prune      func(*broker.Broker, time.Time) (int, error)
	wait       func(<-chan struct{}) bool
}

var ErrClosed = errors.New("applet host is closed")

func New(options Options) *Service {
	return &Service{options: options, stop: make(chan struct{}), closeDone: make(chan struct{}),
		watch: (*broker.Broker).Watch, prune: (*broker.Broker).Prune,
		wait: func(stop <-chan struct{}) bool {
			timer := time.NewTimer(24 * time.Hour)
			defer timer.Stop()
			select {
			case <-stop:
				return false
			case <-timer.C:
				return true
			}
		},
	}
}

// Broker returns the current catalog. Loaders and event callbacks run without
// mu held. An invalidation during loading discards that candidate and retries.
func (s *Service) Broker() (*broker.Broker, error) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil, ErrClosed
	}
	s.work.Add(1)
	s.mu.Unlock()
	defer s.work.Done()
	for {
		if s.stopped() {
			return nil, ErrClosed
		}
		revision := s.options.Revision()
		s.mu.Lock()
		if s.closed {
			s.mu.Unlock()
			return nil, ErrClosed
		}
		if s.current != nil && s.revision != "" && revision != s.revision {
			s.retireLocked()
		}
		if s.current != nil {
			b := s.current
			s.mu.Unlock()
			return b, nil
		}
		generation := s.generation
		s.mu.Unlock()
		bundles, err := s.options.Builtins()
		if err != nil {
			return nil, err
		}
		if s.stopped() {
			return nil, ErrClosed
		}
		extensions, err := s.options.Extensions()
		if err != nil {
			s.options.Emit("extensions:error", err.Error())
		}
		registry, err := applet.New(append(bundles, extensions...)...)
		if err != nil {
			return nil, err
		}
		candidate := &broker.Broker{Registry: registry, State: s.options.State, Emit: s.options.Emit, Consent: s.options.Consent, Logf: s.options.Logf}
		s.mu.Lock()
		if s.closed {
			s.mu.Unlock()
			return nil, ErrClosed
		}
		if generation != s.generation {
			s.mu.Unlock()
			continue
		}
		if s.current == nil {
			s.current, s.revision = candidate, revision
		}
		b := s.current
		s.mu.Unlock()
		return b, nil
	}
}

func (s *Service) retireLocked() {
	if s.current != nil {
		s.retired = append(s.retired, s.current)
		s.current = nil
	}
	s.generation++
}

func (s *Service) Invalidate() {
	s.mu.Lock()
	if !s.closed {
		s.retireLocked()
	}
	s.mu.Unlock()
}

func (s *Service) snapshot() []*broker.Broker {
	s.mu.Lock()
	defer s.mu.Unlock()
	hosts := append([]*broker.Broker{}, s.retired...)
	if s.current != nil {
		hosts = append(hosts, s.current)
	}
	return hosts
}

func (s *Service) NonResumableExecuting() int {
	n := 0
	for _, b := range s.snapshot() {
		n += b.NonResumableExecuting()
	}
	return n
}

// StartMaintenance starts one worker on its initial broker, including after that
// broker retires. It watches once, prunes immediately and then waits a day after
// each prune. Close joins the worker; loader and prune operations are not cancelled.
func (s *Service) StartMaintenance() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrClosed
	}
	if s.started {
		return nil
	}
	s.started = true
	s.work.Add(1)
	go func() { defer s.work.Done(); s.maintain() }()
	return nil
}

func (s *Service) stopped() bool {
	select {
	case <-s.stop:
		return true
	default:
		return false
	}
}

func (s *Service) log(level, format string, args ...any) {
	if s.options.Log != nil {
		s.options.Log(level, format, args...)
	}
}

func (s *Service) maintain() {
	b, err := s.Broker()
	if err != nil {
		if !errors.Is(err, ErrClosed) {
			s.log("error", "applet records: %v", err)
		}
		return
	}
	if s.stopped() {
		return
	}
	if err := s.watch(b); err != nil {
		s.log("warning", "changes the command line makes to applet actions will show after a restart: %v", err)
	}
	for {
		if s.stopped() {
			return
		}
		removed, err := s.prune(b, time.Now())
		if err != nil {
			s.log("warning", "deleting old applet records: %v", err)
		}
		if removed > 0 {
			s.log("info", "deleted %d applet records older than %d days", removed, int(broker.Retention.Hours()/24))
		}
		if !s.wait(s.stop) {
			return
		}
	}
}

// Close prevents new catalog and maintenance work, joins admitted operations,
// then closes retired brokers in order and the current broker. It may wait for
// revision, loader or prune callbacks. It does not cancel running invocations.
func (s *Service) Close() {
	s.mu.Lock()
	if s.closed {
		done := s.closeDone
		s.mu.Unlock()
		<-done
		return
	}
	s.closed = true
	close(s.stop)
	s.mu.Unlock()
	s.work.Wait()
	for _, b := range s.snapshot() {
		_ = b.Close()
	}
	close(s.closeDone)
}
