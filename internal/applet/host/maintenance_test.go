package host

import (
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"studio/internal/applet"
	"studio/internal/applet/broker"
)

func await(t *testing.T, ch <-chan struct{}) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(5 * time.Second):
		t.Fatal("operation did not complete")
	}
}

func TestCloseJoinsCatalogCallbacks(t *testing.T) {
	for _, phase := range []string{"revision", "builtins", "extensions"} {
		t.Run(phase, func(t *testing.T) {
			entered, release := make(chan struct{}), make(chan struct{})
			o := options(t)
			block := func() { close(entered); <-release }
			switch phase {
			case "revision":
				o.Revision = func() string { block(); return "one" }
			case "builtins":
				o.Builtins = func() ([]applet.Bundle, error) { block(); return nil, nil }
			case "extensions":
				o.Extensions = func() ([]applet.Bundle, error) { block(); return nil, nil }
			}
			s := New(o)
			result := make(chan error, 1)
			go func() { _, err := s.Broker(); result <- err }()
			await(t, entered)
			done, other := make(chan struct{}), make(chan struct{})
			go func() { s.Close(); close(done) }()
			await(t, s.stop)
			go func() { s.Close(); close(other) }()
			select {
			case <-done:
				t.Fatal("Close returned while callback was blocked")
			default:
			}
			if _, err := s.Broker(); !errors.Is(err, ErrClosed) {
				t.Fatal("catalog admitted after close", err)
			}
			if err := s.StartMaintenance(); !errors.Is(err, ErrClosed) {
				t.Fatal("maintenance admitted after close", err)
			}
			close(release)
			await(t, done)
			await(t, other)
			if err := <-result; !errors.Is(err, ErrClosed) {
				t.Fatal("catalog published after close", err)
			}
			if len(s.snapshot()) != 0 {
				t.Fatal("candidate retained after close")
			}
		})
	}
}

func TestCloseDuringMaintenanceLoadDoesNotStartWatcher(t *testing.T) {
	o := options(t)
	entered, release := make(chan struct{}), make(chan struct{})
	o.Revision = func() string { close(entered); <-release; return "one" }
	s := New(o)
	var watches atomic.Int32
	s.watch = func(*broker.Broker) error { watches.Add(1); return nil }
	if err := s.StartMaintenance(); err != nil {
		t.Fatal(err)
	}
	await(t, entered)
	done := make(chan struct{})
	go func() { s.Close(); close(done) }()
	await(t, s.stop)
	select {
	case <-done:
		t.Fatal("Close did not join maintenance loading")
	default:
	}
	close(release)
	await(t, done)
	if watches.Load() != 0 {
		t.Fatal("watcher started after shutdown")
	}
}

func TestCloseJoinsPrune(t *testing.T) {
	s := New(options(t))
	s.watch = func(*broker.Broker) error { return nil }
	entered, release := make(chan struct{}), make(chan struct{})
	var prunes atomic.Int32
	s.prune = func(*broker.Broker, time.Time) (int, error) { prunes.Add(1); close(entered); <-release; return 0, nil }
	if err := s.StartMaintenance(); err != nil {
		t.Fatal(err)
	}
	await(t, entered)
	done, other := make(chan struct{}), make(chan struct{})
	go func() { s.Close(); close(done) }()
	await(t, s.stop)
	go func() { s.Close(); close(other) }()
	select {
	case <-done:
		t.Fatal("Close returned before prune")
	default:
	}
	close(release)
	await(t, done)
	await(t, other)
	if err := s.StartMaintenance(); !errors.Is(err, ErrClosed) {
		t.Fatal("maintenance restarted", err)
	}
	if prunes.Load() != 1 {
		t.Fatal("unexpected prune after close")
	}
}

func TestMaintenanceRetainsInitialBrokerAndStopsIdleWait(t *testing.T) {
	s := New(options(t))
	watched := make(chan *broker.Broker, 2)
	pruned := make(chan *broker.Broker, 2)
	waiting := make(chan struct{}, 2)
	tick := make(chan struct{})
	s.watch = func(b *broker.Broker) error { watched <- b; return nil }
	s.prune = func(b *broker.Broker, _ time.Time) (int, error) { pruned <- b; return 0, nil }
	s.wait = func(stop <-chan struct{}) bool {
		waiting <- struct{}{}
		select {
		case <-stop:
			return false
		case <-tick:
			return true
		}
	}
	if err := s.StartMaintenance(); err != nil {
		t.Fatal(err)
	}
	if err := s.StartMaintenance(); err != nil {
		t.Fatal(err)
	}
	await(t, waiting)
	first := <-watched
	if <-pruned != first {
		t.Fatal("initial prune used another broker")
	}
	s.Invalidate()
	current, err := s.Broker()
	if err != nil || current == first {
		t.Fatal("catalog did not retire", err)
	}
	tick <- struct{}{}
	await(t, waiting)
	if <-pruned != first {
		t.Fatal("maintenance switched brokers")
	}
	done := make(chan struct{})
	go func() { s.Close(); close(done) }()
	await(t, done)
	select {
	case <-watched:
		t.Fatal("watch restarted")
	default:
	}
}
