package main

import (
	"context"
	"strings"

	"studio/internal/bus"
	"studio/internal/requestsettlement"
	"studio/internal/taskhost"
)

// RequestOperation is the person's passive lookup for an original dismissal.
// It never starts a replacement operation when the result is unknown.
func (a *App) RequestOperation(id, operation string) (requestsettlement.Receipt, error) {
	in := requestsettlement.Input{ID: id, OperationID: operation}
	if !requestsettlement.ValidOperationID(operation) {
		return requestsettlement.Refuse(in, "invalid_request", "The original operation ID is invalid."), nil
	}
	if strings.HasPrefix(id, "notification:") {
		return a.reportOperation("person", "Dismiss", in)
	}
	if strings.HasPrefix(id, "task-q-") {
		record, err := taskhost.ReadQuestionRecord(configDir(), id)
		if err != nil {
			return requestsettlement.Receipt{}, err
		}
		if receipt, ok := record.Receipts[requestsettlement.Key("person", "dismissed", operation)]; ok {
			return receipt, nil
		}
		return requestsettlement.Receipt{ID: id, OperationID: operation, State: "unknown"}, nil
	}
	var out requestsettlement.Receipt
	err := a.busHost().With(func(b *bus.Bus) error {
		var err error
		out, err = b.RequestOperation(context.Background(), "person", "Dismiss", operation)
		return err
	})
	if err == nil && out.State == "committed" && out.ID != id {
		return requestsettlement.Refuse(in, "not_authorized", "This operation belongs to another request."), nil
	}
	return out, err
}

func (a *App) reportOperation(actor, command string, in requestsettlement.Input) (requestsettlement.Receipt, error) {
	notificationStoreMu.Lock()
	defer notificationStoreMu.Unlock()
	store, err := readNotifications()
	if err != nil {
		return requestsettlement.Receipt{}, err
	}
	if command == "Withdraw" {
		actor = requestsettlement.ChatActor(actor)
	}
	if previous, ok := store.ReportOperations[requestsettlement.Key(actor, command, in.OperationID)]; ok {
		if previous.Input.ID != in.ID {
			return requestsettlement.Refuse(in, "not_authorized", "This operation belongs to another report."), nil
		}
		return previous.Receipt, nil
	}
	return requestsettlement.Receipt{ID: in.ID, OperationID: in.OperationID, State: "unknown"}, nil
}
