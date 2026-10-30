package main

import (
	"sync"

	"studio/internal/sessionhost"
)

// stateClocks are the counters of the window's own state events: one for what
// App.mu guards (accounts and the settings preferences) and one for the update
// watcher's state under updMu. Each is advanced only inside the lock that
// guards the value it numbers (internal/sessionhost/state_events.go).
type stateClocks struct {
	once   sync.Once
	app    *sessionhost.StateClock
	update *sessionhost.StateClock
}

func (a *App) appClock() *sessionhost.StateClock {
	a.clocks.once.Do(func() {
		a.clocks.app, a.clocks.update = sessionhost.NewStateClock(), sessionhost.NewStateClock()
	})
	return a.clocks.app
}

func (a *App) updateClock() *sessionhost.StateClock {
	a.appClock()
	return a.clocks.update
}
