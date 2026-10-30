package main

import (
	"studio/internal/providers"
	"studio/internal/sessionhost"
)

type JournalEntry = sessionhost.JournalEntry

func journalPath(id string) string {
	return sessionhost.JournalPath(configDir(), id, providers.For(providerForSession(id)).JournalFolder)
}

func (a *App) journal(id, kind string) { a.journals.Record(id, kind, journalPath) }

// SessionTimes returns message times in journal order, or an empty list.
func (a *App) SessionTimes(id string) []JournalEntry { return a.journals.Times(id, journalPath) }

func (a *App) forgetJournal(id string) { a.journals.Forget(id, journalPath) }
