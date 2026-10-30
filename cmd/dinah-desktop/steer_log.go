package main

import (
	"context"
	"fmt"

	"studio/internal/bus"
)

// LogQueueDefault writes one line to the log when the composer paints Queue as the default for a chat that is working: the
// chat, what CanSteer answered (or the error that came instead), whether the answer was superseded by a newer request, and
// what the bus knew of the chat's hook turn at that moment. The page calls it once for each change, not for each keystroke,
// so that the next time Queue shows for a chat that can steer, the log says which side answered false and why.
func (a *App) LogQueueDefault(chat string, canSteer bool, failure string, superseded bool) {
	facts := "no bus identity"
	if busID := a.knownBusID(chat); busID != "" {
		ran, err := a.busHost().IfOpen(func(b *bus.Bus) error {
			f, err := b.SteerFactsOf(context.Background(), busID)
			if err != nil {
				facts = "the bus could not say: " + err.Error()
				return nil
			}
			facts = fmt.Sprintf("agent bound %t (turn %s), hook turn %t (%s, active %t, began %s, ended %s), reaches %t", f.Bound, f.BindingToken, f.HookTurn, f.TurnToken, f.Active, f.BeganAt, f.EndedAt, f.Reaches)
			return nil
		})
		if !ran || err != nil {
			facts = "the bus was not open"
		}
	}
	result := fmt.Sprintf("CanSteer said %t", canSteer)
	if failure != "" {
		result = "CanSteer failed: " + failure
	}
	logf("chat", "info", "chat %s shows Queue as the default: %s, superseded %t; %s", chat, result, superseded, facts)
}
