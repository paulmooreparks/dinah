package main

import (
	"context"

	"strings"
	"studio/internal/bus"
	"studio/internal/requestsettlement"
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
	if strings.HasPrefix(in.ID, "task-q-") {
		r, err := a.DismissRequest(requestsettlement.Input{ID: in.ID, Revision: in.Revision, OperationID: in.OperationID})
		out := bus.DecisionReceipt{OperationID: r.OperationID, ID: r.ID, Revision: r.Revision, State: r.State, Closed: r.State == "committed", MessageIDs: []int64{}}
		if r.State == "refused" {
			out.State = "rejected"
			out.Reason = &bus.DecisionRejection{Code: r.Code, Message: r.Text}
		}
		return out, err
	}
	var out bus.DecisionReceipt
	err := a.busHost().With(func(b *bus.Bus) error {
		var err error
		out, err = b.DismissDecision(context.Background(), in)
		return err
	})
	return out, err
}

// DismissRequest is a trusted window method. Services have no dismissal port.
func (a *App) DismissRequest(in requestsettlement.Input) (requestsettlement.Receipt, error) {
	if strings.HasPrefix(in.ID, "notification:") {
		r, err := a.settleNotification(in, "person", "Dismiss")
		if err == nil && r.State == "committed" {
			if dispatchErr := a.flushRequestCompletions(); dispatchErr != nil {
				logf("core", "error", "dispatching report settlement completion: %v", dispatchErr)
			}
		}
		return r, err
	}
	if strings.HasPrefix(in.ID, "task-q-") {
		a.taskServices.mu.Lock()
		h := a.taskServices.host
		a.taskServices.mu.Unlock()
		if h == nil {
			return requestsettlement.Refuse(in, "request_unavailable", "The task host is unavailable."), nil
		}
		return h.DismissQuestion(in)
	}
	var out requestsettlement.Receipt
	err := a.busHost().With(func(b *bus.Bus) error {
		var err error
		out, err = b.DismissRequest(context.Background(), in)
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
