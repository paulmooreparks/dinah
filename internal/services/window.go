package services

import (
	"context"
	"errors"
	"sort"
	"sync"

	"studio/internal/taskhost"
)

// Participant checkpoints and joins its own background workers during shutdown.
// The task host has already closed admission and cancelled execution handles.
type Participant interface{ Shutdown(context.Context) error }

// TaskRecords is implemented by an execution consumer's durable task store.
// Authoring-only services need not implement it and cannot admit task execution.
type TaskRecords interface {
	TaskGeneration(context.Context, string, string) (string, error)
	HoldTask(context.Context, string, string) (func() error, error)
}
type WindowFactory func(taskhost.Client) (Service, Participant, error)

type TaskPresentation struct{ Workspace, Task, Title, Page, State, Unavailable string }
type TaskPresenter interface {
	DescribeTask(context.Context, string, string) (TaskPresentation, error)
}
type WindowPresentation struct {
	Name  string
	Pages []string
}

var windowFactories = map[string]WindowFactory{}
var windowPresentations = map[string]WindowPresentation{}

func HasWindowServices() bool { mu.RLock(); defer mu.RUnlock(); return len(windowFactories) > 0 }

func RegisterWindow(name string, make WindowFactory) {
	RegisterWindowNamed(name, name, nil, make)
}

func RegisterWindowNamed(name, displayName string, pages []string, make WindowFactory) {
	if !namePattern.MatchString(name) || make == nil {
		panic("services: invalid window service registration")
	}
	if displayName == "" || len(displayName) > 256 {
		panic("services: invalid window service display name")
	}
	for _, page := range pages {
		if !namePattern.MatchString(page) {
			panic("services: invalid owned page identity")
		}
	}
	mu.Lock()
	defer mu.Unlock()
	if registry[name] != nil || windowFactories[name] != nil {
		panic("services: service registered twice: " + name)
	}
	windowFactories[name] = make
	windowPresentations[name] = WindowPresentation{Name: displayName, Pages: append([]string(nil), pages...)}
}

type Window struct {
	mu           sync.Mutex
	services     map[string]Service
	records      map[string]TaskRecords
	presenters   map[string]TaskPresenter
	display      map[string]WindowPresentation
	participants []Participant
	closed       bool
}

// NewWindow constructs instances without exposing the root task host to a service.
// Factory errors are returned before any instance is published to page callers.
func NewWindow(host *taskhost.Window) (*Window, error) {
	mu.RLock()
	makers := map[string]WindowFactory{}
	display := map[string]WindowPresentation{}
	for name, make := range windowFactories {
		makers[name] = make
		display[name] = windowPresentations[name]
	}
	mu.RUnlock()
	if len(makers) > 0 && host == nil {
		return nil, errors.New("window task binding is unavailable")
	}
	w := &Window{services: map[string]Service{}, records: map[string]TaskRecords{}, presenters: map[string]TaskPresenter{}, display: display}
	names := make([]string, 0, len(makers))
	for name := range makers {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		s, p, err := makers[name](host.ForService(name))
		if p != nil {
			w.participants = append(w.participants, p)
		}
		if err != nil || s == nil || p == nil {
			if err == nil {
				err = errors.New("window service requires a service and shutdown participant")
			}
			if host != nil {
				host.CloseAdmission()
			}
			return w, err
		}
		w.services[name] = s
		if records, ok := p.(TaskRecords); ok {
			w.records[name] = records
		}
		if presenter, ok := p.(TaskPresenter); ok {
			w.presenters[name] = presenter
		}
	}
	return w, nil
}

func (w *Window) DescribeTask(ctx context.Context, service, workspace, task string) (TaskPresentation, WindowPresentation, error) {
	if w == nil {
		return TaskPresentation{}, WindowPresentation{}, errors.New("window task metadata is unavailable")
	}
	w.mu.Lock()
	p, display, closed := w.presenters[service], w.display[service], w.closed
	display.Pages = append([]string(nil), display.Pages...)
	w.mu.Unlock()
	if closed || p == nil {
		return TaskPresentation{}, WindowPresentation{}, errors.New("the service provides no current task presentation")
	}
	value, err := p.DescribeTask(ctx, workspace, task)
	return value, display, err
}

// Records returns the private task-store port on the shutdown participant.
// This port is never reflected into the page service's methods.
func (w *Window) Records(name string) (TaskRecords, bool) {
	if w == nil {
		return nil, false
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return nil, false
	}
	r, ok := w.records[name]
	return r, ok
}

func (w *Window) Lookup(name string) (Service, bool) {
	if w == nil {
		return nil, false
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return nil, false
	}
	s, ok := w.services[name]
	return s, ok
}

func (w *Window) Shutdown(ctx context.Context) error {
	if w == nil {
		return nil
	}
	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		return nil
	}
	w.closed = true
	participants := append([]Participant(nil), w.participants...)
	w.mu.Unlock()
	var errs []error
	for _, p := range participants {
		errs = append(errs, p.Shutdown(ctx))
	}
	return errors.Join(errs...)
}
