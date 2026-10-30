package main

import (
	"context"
	"crypto/rand"
	"errors"
	"strings"
	"sync"
	"time"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type exitStatus struct {
	Chats               int  `json:"chats"`
	Terminals           int  `json:"terminals"`
	UnfinishedRequests  int  `json:"unfinishedRequests"`
	NonResumableActions bool `json:"nonResumableActions"`
}

func (s exitStatus) needsConfirmation() bool {
	return s.Chats > 0 || s.Terminals > 0 || s.UnfinishedRequests > 0 || s.NonResumableActions
}

// ExitStatus is a snapshot of work owned by this application process.
func (a *App) ExitStatus() exitStatus {
	a.mu.Lock()
	sessions := make([]*Session, 0, a.runtime.ConversationCountLocked())
	for _, s := range a.runtime.ConversationsLocked() {
		sessions = append(sessions, s)
	}
	status := exitStatus{UnfinishedRequests: len(a.exitWork) + a.exitCalls}
	extensionHost := a.extensions
	terminalHost := a.terminals
	a.mu.Unlock()
	if terminalHost != nil {
		status.Terminals = terminalHost.Count()
	}
	for _, s := range sessions {
		// Background work and subagents end with the agent's process when
		// Dinah exits.
		state := s.State()
		if state.Working || state.Switching || len(state.Queued) > 0 || state.HasSteer || state.BackgroundCount > 0 || state.RunningSubagents > 0 {
			status.Chats++
		}
	}
	if extensionHost != nil && extensionHost.Executing() > 0 {
		status.NonResumableActions = true
	}
	a.appletMu.Lock()
	host := a.appletHost
	a.appletMu.Unlock()
	if host != nil && host.NonResumableExecuting() > 0 {
		status.NonResumableActions = true
	}
	return status
}

func (a *App) BeginExitWork(label string) (string, error) {
	label = strings.TrimSpace(label)
	if label == "" || len(label) > 160 {
		return "", errors.New("invalid activity label")
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if len(a.exitWork) >= 1024 {
		return "", errors.New("too many unfinished activities")
	}
	if a.exitWork == nil {
		a.exitWork = map[string]string{}
	}
	token := rand.Text()
	a.exitWork[token] = label
	return token, nil
}
func (a *App) EndExitWork(token string) { a.mu.Lock(); delete(a.exitWork, token); a.mu.Unlock() }

// A query, a presentation change or the exit handshake itself may be
// interrupted by exiting; any other call still running is unfinished work.
// The classes are in pageMethods (page_api.go), and a method without a row
// there is conservatively treated as unfinished work.
func (a *App) observeExitCall(name string) func() {
	if class, ok := pageMethods[name]; ok && class == pageQuery {
		return nil
	}
	a.mu.Lock()
	a.exitCalls++
	a.mu.Unlock()
	var once sync.Once
	return func() { once.Do(func() { a.mu.Lock(); a.exitCalls--; a.mu.Unlock() }) }
}

func (a *App) abandonClose() {
	a.mu.Lock()
	a.exitConfirmed = false
	a.closeApproved = false
	a.mu.Unlock()
	a.restartAbandoned()
}
func (a *App) confirmExitIfNeeded(ctx context.Context) bool {
	a.mu.Lock()
	if a.exitConfirming {
		a.mu.Unlock()
		return false
	}
	if a.exitConfirmed {
		a.mu.Unlock()
		return true
	}
	if a.closeRequest != "" {
		a.mu.Unlock()
		return false
	}
	a.exitConfirming = true
	prompt := a.exitPrompt
	a.mu.Unlock()
	status := a.ExitStatus()
	if !status.needsConfirmation() {
		a.mu.Lock()
		a.exitConfirming = false
		a.mu.Unlock()
		return true
	}
	if prompt != nil {
		accepted := prompt(ctx, status)
		a.mu.Lock()
		a.exitConfirming = false
		a.exitConfirmed = accepted
		a.mu.Unlock()
		if !accepted {
			a.abandonClose()
		}
		return accepted
	}
	id := rand.Text()
	a.mu.Lock()
	a.closeApproved = false
	a.exitRequest = id
	a.exitRequestStatus = status
	a.exitTimer = time.AfterFunc(5*time.Minute, func() { a.expireExitConfirmation(id) })
	a.mu.Unlock()
	a.emit("dinah:exit-confirmation", exitConfirmation{ID: id, Status: status})
	return false
}

// exitConfirmation is the request to confirm leaving, as the
// dinah:exit-confirmation event carries it.
type exitConfirmation struct {
	ID     string     `json:"id"`
	Status exitStatus `json:"status"`
}

// PendingExitConfirmation is the confirmation of leaving the person has not
// answered, with the status it was asked with, or nil when there is none. A
// page that missed the event reads it here.
func (a *App) PendingExitConfirmation() *exitConfirmation {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.exitRequest == "" {
		return nil
	}
	return &exitConfirmation{ID: a.exitRequest, Status: a.exitRequestStatus}
}

func (a *App) acceptExitConfirmation(id string, accepted bool) error {
	a.mu.Lock()
	if id == "" || id != a.exitRequest {
		a.mu.Unlock()
		return errors.New("the exit confirmation is no longer current")
	}
	a.exitRequest = ""
	a.exitConfirming = false
	a.exitConfirmed = accepted
	if a.exitTimer != nil {
		a.exitTimer.Stop()
		a.exitTimer = nil
	}
	if !accepted {
		a.closeApproved = false
	}
	a.mu.Unlock()
	a.emit("dinah:exit-confirmation-resolved", map[string]string{"id": id})
	if !accepted {
		a.restartAbandoned()
	}
	return nil
}
func (a *App) AnswerExitConfirmation(id string, accepted bool) error {
	if err := a.acceptExitConfirmation(id, accepted); err != nil {
		return err
	}
	if accepted {
		wruntime.Quit(a.ctx)
	}
	return nil
}
func (a *App) expireExitConfirmation(id string) {
	a.mu.Lock()
	if id != a.exitRequest {
		a.mu.Unlock()
		return
	}
	a.exitRequest = ""
	a.exitConfirming = false
	a.exitConfirmed = false
	a.closeApproved = false
	if a.exitTimer != nil {
		a.exitTimer.Stop()
		a.exitTimer = nil
	}
	a.mu.Unlock()
	a.restartAbandoned()
	a.emit("dinah:exit-confirmation-expired", map[string]string{"id": id})
}
