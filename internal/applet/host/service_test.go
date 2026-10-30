package host

import (
	"context"
	"encoding/json"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"studio/internal/applet"
	"studio/internal/applet/broker"
)

func options(t *testing.T) Options {
	return Options{State: t.TempDir(), Builtins: func() ([]applet.Bundle, error) { return nil, nil }, Extensions: func() ([]applet.Bundle, error) { return nil, nil }, Revision: func() string { return "one" }, Emit: func(string, any) {}}
}

func TestGenerationsAndCallbacks(t *testing.T) {
	o := options(t)
	revision := "one"
	var s *Service
	unlocked := func() {
		t.Helper()
		if !s.mu.TryLock() {
			t.Fatal("callback holds service lock")
		}
		s.mu.Unlock()
	}
	o.Revision = func() string { unlocked(); return revision }
	builds := 0
	o.Builtins = func() ([]applet.Bundle, error) { unlocked(); builds++; return nil, nil }
	o.Extensions = func() ([]applet.Bundle, error) { unlocked(); return nil, errors.New("unavailable") }
	events := 0
	o.Emit = func(name string, data any) {
		unlocked()
		if name != "extensions:error" || data != "unavailable" {
			t.Fatal(name, data)
		}
		events++
	}
	s = New(o)
	first, err := s.Broker()
	if err != nil {
		t.Fatal(err)
	}
	same, _ := s.Broker()
	if same != first || builds != 1 {
		t.Fatal("catalog was not reused")
	}
	revision = "two"
	second, err := s.Broker()
	if err != nil || second == first {
		t.Fatal("revision did not retire broker", err)
	}
	s.Invalidate()
	third, err := s.Broker()
	if err != nil || third == second {
		t.Fatal("explicit invalidation failed", err)
	}
	all := s.snapshot()
	if len(all) != 3 || all[0] != first || all[1] != second || all[2] != third || events != 3 {
		t.Fatal("generation order", all, events)
	}
	s.Close()
}

func TestInvalidationDuringLoadingAndRegistrationErrors(t *testing.T) {
	o := options(t)
	var s *Service
	loads := 0
	o.Builtins = func() ([]applet.Bundle, error) {
		loads++
		if loads == 1 {
			s.Invalidate()
		}
		return nil, nil
	}
	s = New(o)
	if _, err := s.Broker(); err != nil || loads != 2 {
		t.Fatal("stale candidate published", err, loads)
	}
	o.Builtins = func() ([]applet.Bundle, error) { return nil, errors.New("builtins unavailable") }
	s = New(o)
	if _, err := s.Broker(); err == nil {
		t.Fatal("loader error lost")
	}
	o.Builtins = func() ([]applet.Bundle, error) { return []applet.Bundle{{}}, nil }
	s = New(o)
	if _, err := s.Broker(); err == nil {
		t.Fatal("invalid registry accepted")
	}
}

func TestRetiredAndCurrentExecutionSurviveClose(t *testing.T) {
	o := options(t)
	release := make(chan struct{})
	started := make(chan struct{}, 2)
	finished := make(chan struct{}, 2)
	o.Emit = func(name string, data any) {
		if v, ok := data.(broker.Invocation); name == "applet:invocation" && ok && v.Status == "done" {
			finished <- struct{}{}
		}
	}
	defer func() {
		close(release)
		for i := 0; i < 2; i++ {
			select {
			case <-finished:
			case <-time.After(5 * time.Second):
				t.Error("execution did not finish")
				return
			}
		}
	}()
	ref := "test.host/run@1"
	o.Builtins = func() ([]applet.Bundle, error) {
		return []applet.Bundle{{
			Manifest: applet.Manifest{ID: "test.host", Version: "1.0.0", Title: "Host", APIVersion: 1, Actions: []applet.Action{{Ref: ref, Title: "Run", Replay: "once", InputSchema: json.RawMessage(`{"type":"object"}`), OutputSchema: json.RawMessage(`{"type":"object"}`)}}},
			Sources:  map[string][]byte{"implementation": []byte("test")},
			Handlers: map[string]applet.Handler{ref: func(ctx context.Context, input string) (applet.Result, error) {
				started <- struct{}{}
				<-release
				return applet.Result{OutputJSON: `{}`}, nil
			}},
		}}, nil
	}
	s := New(o)
	var consent atomic.Int32
	s.options.Consent = func(context.Context, broker.Invocation, applet.Action) error { consent.Add(1); return nil }
	first, err := s.Broker()
	if err != nil {
		t.Fatal(err)
	}
	ws := t.TempDir()
	if _, err = first.Start(ws, "chat", "owner", ref, `{}`, "", time.Now().Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	s.Invalidate()
	second, err := s.Broker()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = second.Start(ws, "chat", "owner", ref, `{}`, "", time.Now().Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		select {
		case <-started:
		case <-time.After(5 * time.Second):
			t.Fatal("handler did not start")
		}
	}
	if n := s.NonResumableExecuting(); n != 2 {
		t.Fatal("missing retired/current execution", n)
	}
	s.Close()
	if s.NonResumableExecuting() != 2 || consent.Load() != 2 {
		t.Fatal("close cancelled execution or consent changed")
	}
}
