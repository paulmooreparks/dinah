package main

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"sort"
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
		if state.WorkActivity().Busy() {
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
	if a.exitCallNames == nil {
		a.exitCallNames = map[string]int{}
	}
	a.exitCallNames[name]++
	a.mu.Unlock()
	var once sync.Once
	return func() {
		once.Do(func() {
			a.mu.Lock()
			a.exitCalls--
			if a.exitCallNames[name]--; a.exitCallNames[name] <= 0 {
				delete(a.exitCallNames, name)
			}
			if a.exitCalls == 0 && a.exitDrained != nil {
				close(a.exitDrained)
				a.exitDrained = nil
			}
			a.mu.Unlock()
		})
	}
}

// exitSettle is how long a close waits for the page calls already running
// to finish before it decides whether to ask. A call is counted as unfinished
// work only while it runs, so a close that looked at one instant asked about
// calls that ended a moment later: Paul confirmed Restart Now quickly on 18
// November 2026 and was asked to confirm exiting, which a few seconds' wait
// avoided. A call that finishes while the close waits is not interrupted, so
// there is nothing to ask about.
var exitSettle = 5 * time.Second

// settleThenClose waits until the running page calls have finished, or
// exitSettle has passed, and closes the window again. That close decides as
// usual, asking when work is still running.
func (a *App) settleThenClose(drained <-chan struct{}) {
	timer := time.NewTimer(exitSettle)
	defer timer.Stop()
	select {
	case <-drained:
	case <-timer.C:
	}
	a.mu.Lock()
	a.exitSettling = false
	a.exitSettled = true
	retry := a.exitRetry
	a.mu.Unlock()
	if retry != nil {
		retry()
	} else if a.ctx != nil {
		wruntime.Quit(a.ctx)
	}
}

// logExitAsk says in the log which calls and activities were still running
// when a close asked to confirm, so that a question nobody expected can be
// traced to its cause.
func (a *App) logExitAsk(status exitStatus) {
	if status.UnfinishedRequests == 0 {
		return
	}
	a.mu.Lock()
	calls := make([]string, 0, len(a.exitCallNames))
	for name, n := range a.exitCallNames {
		calls = append(calls, fmt.Sprintf("%s×%d", name, n))
	}
	work := make([]string, 0, len(a.exitWork))
	for _, label := range a.exitWork {
		work = append(work, label)
	}
	a.mu.Unlock()
	sort.Strings(calls)
	sort.Strings(work)
	logf("core", "info", "asking to confirm exit with page calls still running [%s] and activities [%s]", strings.Join(calls, ", "), strings.Join(work, ", "))
}

func (a *App) abandonClose() {
	a.mu.Lock()
	a.exitConfirmed = false
	a.closeApproved = false
	a.exitSettled = false
	a.mu.Unlock()
	a.restartAbandoned()
}
func (a *App) confirmExitIfNeeded(ctx context.Context) bool {
	a.mu.Lock()
	if a.exitConfirming || a.exitSettling {
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
	// Page calls still running are given exitSettle to finish before the
	// close decides, once per close. The channel is made while a call is
	// counted, under the lock its end takes, so its end cannot be missed.
	if !a.exitSettled && a.exitCalls > 0 {
		a.exitSettling = true
		drained := make(chan struct{})
		a.exitDrained = drained
		a.mu.Unlock()
		go a.settleThenClose(drained)
		return false
	}
	a.exitSettled = false
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
	a.logExitAsk(status)
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
