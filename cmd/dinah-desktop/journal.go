package main

import (
	"studio/internal/providers"
	"studio/internal/sessionhost"
	"time"
)

type JournalEntry = sessionhost.JournalEntry

func journalPath(id string) string {
	return sessionhost.JournalPath(configDir(), id, providers.For(providerForSession(id)).JournalFolder)
}

func (a *App) journal(id, kind string) {
	if folderClosed() {
		return
	}
	a.journals.Record(id, kind, journalPath)
	if s := a.session(id); s != nil {
		at := time.Now().UTC().Format(time.RFC3339Nano)
		s.UpdateDisplay(nil, &at)
		sessionhost.EmitSession(a.emit, s)
	}
}

// SessionTimes returns message times in journal order, or an empty list.
func (a *App) SessionTimes(id string) []JournalEntry { return a.journals.Times(id, journalPath) }

func (a *App) forgetJournal(id string) { a.journals.Forget(id, journalPath) }

func (a *App) recordedActivity(id string) string {
	at := a.journals.Latest(id, journalPath)
	if at.IsZero() {
		return ""
	}
	return at.UTC().Format(time.RFC3339Nano)
}
