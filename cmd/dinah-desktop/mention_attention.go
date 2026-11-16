package main

import (
	"context"
	"studio/internal/bus"
)

// AcknowledgeMention is available only through the trusted window. The page
// calls it after the exact message has been opened, scrolled to and marked.
func (a *App) AcknowledgeMention(messageID int64) error {
	return a.busHost().With(func(b *bus.Bus) error {
		if err := b.AcknowledgeMention(context.Background(), messageID); err != nil {
			return err
		}
		a.dismissAnsweredMentions(context.Background(), b)
		return nil
	})
}
