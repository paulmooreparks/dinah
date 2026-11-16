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

func (a *App) notificationRequestIdentities(items []Notification) ([]Notification, error) {
	needed := false
	for _, item := range items {
		needed = needed || item.Kind == "mention"
	}
	if !needed {
		return items, nil
	}
	err := a.busHost().With(func(b *bus.Bus) error {
		for i := range items {
			if items[i].Kind != "mention" {
				continue
			}
			identity, err := b.PersonRequestIdentity(context.Background(), items[i].Message)
			if err != nil {
				return err
			}
			items[i].RequestID = identity
		}
		return nil
	})
	return items, err
}
