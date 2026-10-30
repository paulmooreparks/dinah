package main

import (
	"context"

	"studio/internal/bus"
)

// A title is one short line (bus.CleanTitle). Titles stored before that limit
// may be a whole prompt, with the context blocks Dinah injected into it, so at
// start each stored title is replaced by its cleaned form, on the bus and in
// the names the person gave. The pass changes nothing when every title is
// already clean.
func (a *App) shortenTitles() {
	err := a.busHost().With(func(b *bus.Bus) error {
		n, err := b.ShortenTitles(context.Background())
		if err == nil && n > 0 {
			logf("bus", "info", "shortened %d stored chat titles to one short line", n)
		}
		return err
	})
	if err != nil {
		logf("bus", "error", "shortening the stored chat titles: %v", err)
	}
	// The messages stored before the bus stopped keeping applet contexts.
	err = a.busHost().With(func(b *bus.Bus) error {
		n, err := b.RedactStoredMessages(context.Background())
		if err == nil && n > 0 {
			logf("bus", "info", "removed the applet contexts from %d stored messages", n)
		}
		return err
	})
	if err != nil {
		logf("bus", "error", "removing applet contexts from stored messages: %v", err)
	}
	err = a.updateSettings(func(s *Settings) error {
		for id, title := range s.SessionTitles {
			if clean := bus.CleanTitle(title); clean != title {
				if clean == "" {
					delete(s.SessionTitles, id)
				} else {
					s.SessionTitles[id] = clean
				}
			}
		}
		return nil
	})
	if err != nil {
		logf("core", "error", "shortening the saved chat names: %v", err)
	}
}
