package main

import (
	"context"

	"studio/internal/bus"
	"studio/internal/sessionhost"
)

func (a *App) steeringLedger() *sessionhost.SteeringLedger {
	a.ledgerOnce.Do(func() {
		a.ledger = sessionhost.NewSteeringLedger(sessionhost.SteeringLedgerHost{
			Conversation: a.session,
			Save: func(session, payload string) (int64, error) {
				var id int64
				err := a.busHost().With(func(b *bus.Bus) error {
					var err error
					id, err = b.SaveSteeringSubmission(context.Background(), session, payload)
					return err
				})
				return id, err
			},
			SetState: func(session string, id int64, state string) error {
				return a.busHost().With(func(b *bus.Bus) error { return b.SetSteeringSubmissionState(context.Background(), session, id, state) })
			},
			Recover: func(session string) ([]bus.SteeringSubmission, error) {
				var out []bus.SteeringSubmission
				err := a.busHost().With(func(b *bus.Bus) error {
					var err error
					out, err = b.RecoverSteeringSubmissions(context.Background(), session)
					return err
				})
				return out, err
			},
			LogError: func(err error) { logf("steering", "error", "saving delivery state: %v", err) },
		})
	})
	return a.ledger
}

// RecoverSteering returns retained submissions only when the chat is idle.
// The UI offers copying them to a draft, never automatic delivery.
func (a *App) RecoverSteering(id string) ([]bus.SteeringSubmission, error) {
	return a.steeringLedger().Recover(id)
}
func (a *App) DismissSteering(id string, submission int64) error {
	return a.steeringLedger().Dismiss(id, submission)
}
