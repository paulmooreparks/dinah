package main

import (
	"context"

	"studio/internal/bus"
)

func (a *App) Decisions(before int64) (bus.DecisionPage, error) {
	var out bus.DecisionPage
	err := a.busHost().With(func(b *bus.Bus) error {
		var err error
		out, err = b.Decisions(context.Background(), before)
		return err
	})
	return out, err
}

func (a *App) DecisionByID(id string) (bus.DecisionDetail, error) {
	var out bus.DecisionDetail
	err := a.busHost().With(func(b *bus.Bus) error { var err error; out, err = b.DecisionByID(context.Background(), id); return err })
	return out, err
}

func (a *App) AnswerDecision(in bus.AnswerDecisionInput) (bus.DecisionReceipt, error) {
	var out bus.DecisionReceipt
	err := a.busHost().With(func(b *bus.Bus) error {
		var err error
		out, err = b.AnswerDecision(context.Background(), in)
		return err
	})
	return out, err
}

func (a *App) DismissDecision(in bus.DismissDecisionInput) (bus.DecisionReceipt, error) {
	var out bus.DecisionReceipt
	err := a.busHost().With(func(b *bus.Bus) error {
		var err error
		out, err = b.DismissDecision(context.Background(), in)
		return err
	})
	return out, err
}

func (a *App) DecisionOperation(operationID string) (bus.DecisionReceipt, error) {
	var out bus.DecisionReceipt
	err := a.busHost().With(func(b *bus.Bus) error {
		var err error
		out, err = b.DecisionOperation(context.Background(), operationID)
		return err
	})
	return out, err
}

func (a *App) PreviewDecisionBroadcast(scope, text string) (bus.DecisionBroadcastPreview, error) {
	working := []string{}
	a.mu.Lock()
	for _, c := range a.runtime.ConversationsLocked() {
		info := c.Info()
		if info.Working {
			if id := a.settings.BusSessions[info.ID]; id != "" {
				working = append(working, id)
			}
		}
	}
	a.mu.Unlock()
	var out bus.DecisionBroadcastPreview
	err := a.busHost().With(func(b *bus.Bus) error {
		var err error
		out, err = b.PreviewDecisionBroadcast(context.Background(), scope, text, working)
		return err
	})
	return out, err
}

func (a *App) ConfirmDecisionBroadcast(in bus.ConfirmDecisionBroadcastInput) (bus.DecisionReceipt, error) {
	var out bus.DecisionReceipt
	err := a.busHost().With(func(b *bus.Bus) error {
		var err error
		out, err = b.ConfirmDecisionBroadcast(context.Background(), in)
		return err
	})
	return out, err
}
