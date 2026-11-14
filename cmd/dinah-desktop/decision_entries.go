package main

import (
	"context"
	"studio/internal/bus"
)

func (a *App) DecisionEntries() (bus.DecisionEntryState, error) {
	var out bus.DecisionEntryState
	err := a.busHost().With(func(b *bus.Bus) error {
		var err error
		out, err = b.DecisionEntries(context.Background())
		return err
	})
	return out, err
}
