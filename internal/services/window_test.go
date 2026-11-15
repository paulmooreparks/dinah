package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"studio/internal/taskhost"
)

type windowParticipant struct{ closed bool }

func (p *windowParticipant) Shutdown(context.Context) error { p.closed = true; return nil }
func (p *windowParticipant) TaskGeneration(context.Context, string, string) (string, error) {
	return "generation-1", nil
}
func (p *windowParticipant) HoldTask(context.Context, string, string) (func() error, error) {
	return func() error { return nil }, nil
}

type windowBackend struct{ calls int }

func (b *windowBackend) Execute(context.Context, taskhost.Scope, taskhost.Operation, taskhost.Capture) taskhost.Outcome {
	b.calls++
	return taskhost.Outcome{State: "succeeded"}
}

func windowHost(t *testing.T) (*taskhost.Window, *windowBackend) {
	t.Helper()
	b := &windowBackend{}
	h, err := taskhost.New(taskhost.Options{State: t.TempDir(),
		Resolve: func(context.Context, taskhost.Scope) (taskhost.Binding, error) {
			return taskhost.Binding{Path: "local", Generation: "1"}, nil
		},
		Check:   func(context.Context, taskhost.Scope) error { return nil },
		Admit:   func(context.Context, taskhost.Scope, taskhost.Operation) error { return nil },
		Confirm: func(context.Context, taskhost.Review) error { return nil },
		Track:   func(taskhost.Scope, taskhost.Operation) (func(), error) { return func() {}, nil }, Backend: b,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := h.Shutdown(ctx); err != nil {
			t.Error(err)
		}
	})
	return h, b
}

func registerWindowTest(t *testing.T, name string, factory WindowFactory) {
	t.Helper()
	RegisterWindow(name, factory)
	t.Cleanup(func() { mu.Lock(); delete(windowFactories, name); delete(windowPresentations, name); mu.Unlock() })
}

func TestWindowFactoryKeepsTaskStorePrivateAndStartsNoWork(t *testing.T) {
	p := &windowParticipant{}
	registerWindowTest(t, "private-task-test", func(taskhost.Client) (Service, Participant, error) {
		return Object(&counter{}), p, nil
	})
	h, backend := windowHost(t)
	w, err := NewWindow(h)
	if err != nil {
		t.Fatal(err)
	}
	store, ok := w.Records("private-task-test")
	if !ok || store != p {
		t.Fatal("private task port was lost")
	}
	s, ok := w.Lookup("private-task-test")
	if !ok {
		t.Fatal("page service was lost")
	}
	if _, err := s.Call(context.Background(), "HoldTask", nil); !errors.Is(err, ErrUnknownMethod) {
		t.Fatalf("task lock exposed to page: %v", err)
	}
	if backend.calls != 0 || h.Snapshot().Workers != 0 {
		t.Fatal("factory started work")
	}
	if err := w.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !p.closed {
		t.Fatal("participant was not joined")
	}
	if _, ok := w.Records("private-task-test"); ok {
		t.Fatal("closed window exposed task store")
	}
}

func TestWindowFactoryFailureClosesAdmission(t *testing.T) {
	p := &windowParticipant{}
	registerWindowTest(t, "failed-task-test", func(taskhost.Client) (Service, Participant, error) {
		return nil, p, errors.New("construction failed")
	})
	h, backend := windowHost(t)
	w, err := NewWindow(h)
	if err == nil || !h.Snapshot().AdmissionClosed || backend.calls != 0 {
		t.Fatal("factory failure left admission open")
	}
	if err := w.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !p.closed {
		t.Fatal("failed factory participant was not cleaned up")
	}
}
