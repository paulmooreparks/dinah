package terminals

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

// PendingTerminal describes resources whose completion has not been confirmed.
// IDs remain stable across spawn, registration and a timed-out shutdown.
type PendingTerminal struct {
	ID            string `json:"id"`
	PID           int    `json:"pid,omitempty"`
	SpawnPending  bool   `json:"spawnPending"`
	ShellPending  bool   `json:"shellPending"`
	OutputPending bool   `json:"outputPending"`
	ReadError     string `json:"readError,omitempty"`
	WaitError     string `json:"waitError,omitempty"`
	CloseError    string `json:"closeError,omitempty"`
}

// ShutdownError retains the incomplete stages without discarding live tracking.
type ShutdownError struct {
	Cause   error
	Pending []PendingTerminal
}

func (e *ShutdownError) Error() string {
	return fmt.Sprintf("terminal shutdown incomplete: %v; pending=%+v", e.Cause, e.Pending)
}

func (e *ShutdownError) Unwrap() error { return e.Cause }

func (a *Service) signalLocked() {
	if a.changed != nil {
		close(a.changed)
	}
	a.changed = make(chan struct{})
}

func (a *Service) requestCloseLocked(t *terminal) {
	if t.session == nil || t.closing {
		return
	}
	t.closing = true
	go func() {
		err := t.session.Close()
		if err != nil {
			a.mu.Lock()
			t.closeErr = err
			a.signalLocked()
			a.mu.Unlock()
		}
	}()
}

func (a *Service) pendingLocked() ([]PendingTerminal, bool) {
	pending := make([]PendingTerminal, 0, len(a.active))
	failed := false
	for _, t := range a.active {
		p := PendingTerminal{ID: t.id, PID: t.pid, SpawnPending: t.session == nil,
			ShellPending: !t.shellDone || t.waitErr != nil, OutputPending: !t.outputDone}
		if t.readErr != nil {
			p.ReadError = t.readErr.Error()
			failed = true
		}
		if t.waitErr != nil {
			p.WaitError = t.waitErr.Error()
			failed = true
		}
		if t.closeErr != nil {
			p.CloseError = t.closeErr.Error()
			failed = true
		}
		pending = append(pending, p)
	}
	sort.Slice(pending, func(i, j int) bool { return strings.Compare(pending[i].ID, pending[j].ID) < 0 })
	return pending, failed
}

// Shutdown permanently closes start admission and requests closure of every
// admitted terminal, including starts still spawning. Only the host lifecycle
// calls it, outside emit callbacks. It waits for shell termination and clean
// output EOF, not for exit callback delivery. A timeout or read/wait error
// preserves incomplete accounting; a later call may observe further progress.
func (a *Service) Shutdown(ctx context.Context) error {
	a.mu.Lock()
	a.stopping = true
	for _, t := range a.active {
		a.requestCloseLocked(t)
	}
	for {
		pending, failed := a.pendingLocked()
		if len(pending) == 0 {
			a.mu.Unlock()
			return nil
		}
		cause := ctx.Err()
		if cause == nil && failed {
			cause = fmt.Errorf("terminal completion failed")
		}
		if cause != nil {
			a.mu.Unlock()
			return &ShutdownError{Cause: cause, Pending: pending}
		}
		if a.changed == nil {
			a.changed = make(chan struct{})
		}
		changed := a.changed
		a.mu.Unlock()
		select {
		case <-ctx.Done():
		case <-changed:
		}
		a.mu.Lock()
	}
}
