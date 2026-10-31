package main

import (
	"context"
	"studio/internal/bus"
)

// BusGroupLookup distinguishes unresolved names from operational failures.
// Room is present only when State is resolved. Not-found does not prove deletion.
type BusGroupLookup struct {
	State string   `json:"state"`
	Room  *BusRoom `json:"room,omitempty"`
}

// BusResolveGroup resolves stable, current and recorded historical names for UI
// navigation. Sending continues to reject stale titles through the existing API.
func (a *App) BusResolveGroup(key string) (BusGroupLookup, error) {
	var out BusGroupLookup
	err := a.busHost().With(func(b *bus.Bus) error {
		state, room, err := b.ResolveRoom(context.Background(), key)
		out.State = state
		if err == nil && room != nil {
			r := a.busRoom(b, *room)
			out.Room = &r
		}
		return err
	})
	return out, err
}
