package main

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"studio/internal/terminals"
)

const terminalShutdownBudget = 15 * time.Second

type terminalShutdownRecord struct {
	Version     int                         `json:"version"`
	ID          string                      `json:"id"`
	At          time.Time                   `json:"at"`
	Error       string                      `json:"error"`
	Pending     []terminals.PendingTerminal `json:"pending"`
	NoticeError string                      `json:"noticeError,omitempty"`
	NoticeSaved bool                        `json:"noticeSaved,omitempty"`
}

// shutdownTerminals is called only by the host lifecycle, outside terminal
// callbacks. The UI context may already be canceled, so it owns a fresh budget.
func (a *App) shutdownTerminals(shutdown func(context.Context) error) {
	ctx, cancel := context.WithTimeout(context.Background(), terminalShutdownBudget)
	defer cancel()
	err := shutdown(ctx)
	if err == nil {
		return
	}
	a.terminalShutdownIncomplete.Store(true)
	a.updMu.Lock()
	if a.update != nil {
		a.update.restarting, a.update.pending = false, false
	}
	a.updMu.Unlock()
	r := terminalShutdownRecord{Version: 1, ID: "terminal-shutdown-" + rand.Text(), At: time.Now().UTC(), Error: redactAppletContexts(err.Error())}
	var incomplete *terminals.ShutdownError
	if errors.As(err, &incomplete) {
		r.Pending = append([]terminals.PendingTerminal(nil), incomplete.Pending...)
		for i := range r.Pending {
			r.Pending[i].ReadError = redactAppletContexts(r.Pending[i].ReadError)
			r.Pending[i].WaitError = redactAppletContexts(r.Pending[i].WaitError)
			r.Pending[i].CloseError = redactAppletContexts(r.Pending[i].CloseError)
		}
	}
	logf("terminal", "error", "%s", r.Error)
	a.persistTerminalShutdown(r)
	if err := a.saveTerminalShutdownNotice(r); err != nil {
		r.NoticeError = redactAppletContexts(err.Error())
		logf("terminal", "error", "saving incomplete-shutdown notice: %v", err)
		a.persistTerminalShutdown(r)
	} else {
		r.NoticeSaved = true
		a.persistTerminalShutdown(r)
	}
	flushAppLog()
}

func (a *App) persistTerminalShutdown(r terminalShutdownRecord) {
	data, err := json.MarshalIndent(r, "", "  ")
	if err == nil {
		err = os.MkdirAll(logDir(), 0o700)
	}
	if err == nil {
		// Keep an incident record independently of the rolling log preference.
		err = writeStateFile(filepath.Join(logDir(), r.ID+".json"), data)
	}
	if err != nil {
		logf("terminal", "error", "saving terminal shutdown diagnostics: %v", err)
	}
	// The latest record also retries a failed notice write on the next launch.
	if err := writeStateFile(filepath.Join(configDir(), "terminal-shutdown.json"), data); err != nil {
		logf("terminal", "error", "saving terminal shutdown recovery record: %v", err)
	}
}

func (a *App) saveTerminalShutdownNotice(r terminalShutdownRecord) error {
	detail := r.Error
	if len(detail) > 4096 {
		detail = detail[:4096] + "\nFurther details are retained in the shutdown diagnostics."
	}
	_, _, err := a.saveNotification(Notification{ID: r.ID, Kind: "app", Severity: "warning",
		Title:   "Terminal shutdown was incomplete",
		Body:    "The previous window could not confirm that all terminals finished closing. Automatic Restart to Update was skipped. Detached programs may still be running.",
		Details: []string{detail, fmt.Sprintf("Recorded at %s", r.At.Format(time.RFC3339))}})
	return err
}

func (a *App) restoreTerminalShutdownNotice() {
	data, err := readRegularStateFile(filepath.Join(configDir(), "terminal-shutdown.json"))
	if errors.Is(err, os.ErrNotExist) {
		return
	}
	var r terminalShutdownRecord
	if err == nil {
		err = json.Unmarshal(data, &r)
	}
	if err == nil && (r.Version != 1 || !notificationID.MatchString(r.ID) || r.Error == "") {
		err = errors.New("invalid terminal shutdown record")
	}
	if err == nil && r.NoticeSaved {
		return
	}
	if err == nil {
		err = a.saveTerminalShutdownNotice(r)
		if err == nil {
			r.NoticeSaved = true
			a.persistTerminalShutdown(r)
		}
	}
	if err != nil {
		logf("terminal", "error", "restoring terminal shutdown notice: %v", err)
	}
}
