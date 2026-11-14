package main

import (
	"context"

	"studio/internal/bus"
)

// BusGroupMessageContext returns at most five messages strictly before the
// target in its group, oldest first, without marking anything read.
func (a *App) BusGroupMessageContext(group string, messageId int64) ([]BusMessage, error) {
	group, err := groupName(group)
	if err != nil {
		return nil, err
	}
	out := []BusMessage{}
	err = a.busHost().With(func(b *bus.Bus) error {
		r, err := b.Room(context.Background(), group)
		if err != nil {
			return err
		}
		messages, err := b.RoomMessageContext(context.Background(), r.Name, messageId)
		if err == nil {
			out = busMessages(messages)
		}
		return err
	})
	return out, err
}
