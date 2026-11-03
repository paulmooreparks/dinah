package main

import (
	"context"

	"studio/internal/bus"
)

func (a *App) BusGroupReach(group string, mentionIDs []string) (bus.GroupReach, error) {
	var out bus.GroupReach
	err := a.busHost().With(func(b *bus.Bus) error {
		var err error
		out, err = b.GroupReach(context.Background(), group, mentionIDs)
		return err
	})
	return out, err
}

func (a *App) BusGroupAttentionSettings() (bus.GroupAttentionSettings, error) {
	var out bus.GroupAttentionSettings
	err := a.busHost().With(func(b *bus.Bus) error {
		var err error
		out, err = b.GroupAttentionSettings(context.Background())
		return err
	})
	return out, err
}

func (a *App) SetBusGroupAttentionSettings(limit, minutes int) error {
	return a.busHost().With(func(b *bus.Bus) error {
		return b.SetGroupAttentionSettings(context.Background(), limit, minutes)
	})
}
