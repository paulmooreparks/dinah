package main

import (
	"errors"
	"strings"
	"time"

	"studio/internal/requestsettlement"
)

func notificationRequestID(item Notification) string {
	id := "notification:" + item.ID
	if item.IncidentVersion != "" {
		id += ":" + item.IncidentVersion
	}
	return id
}

func (a *App) notificationRequest(store notificationStore, id string) (notificationSettlement, error) {
	if saved, ok := store.ReportSettlements[id]; ok {
		return saved, nil
	}
	items := store.Items
	if strings.Count(id, ":") > 1 {
		incidents, err := a.recoveryNotifications()
		if err != nil {
			return notificationSettlement{}, err
		}
		items = incidents
	}
	for _, item := range items {
		if notificationRequestID(item) == id {
			status := "open"
			if item.Dismissed || item.IncidentVersion != "" && store.RecoveryDismissals[item.ID] == item.IncidentVersion {
				status = "dismissed"
			}
			return notificationSettlement{ID: id, Revision: 1, Report: item, Disposition: requestsettlement.Disposition{Status: status}}, nil
		}
	}
	return notificationSettlement{}, errors.New("the report is unavailable")
}

// NotificationRequest reads the kept source, including its closed history.
func (a *App) NotificationRequest(id string) (notificationSettlement, error) {
	notificationStoreMu.Lock()
	defer notificationStoreMu.Unlock()
	store, err := readNotifications()
	if err != nil {
		return notificationSettlement{}, err
	}
	return a.notificationRequest(store, id)
}

// settleNotification is private. The person entry point fixes actor to person;
// an owner entry point must first resolve its authenticated chat context.
func (a *App) settleNotification(in requestsettlement.Input, actor, command string) (requestsettlement.Receipt, error) {
	if !requestsettlement.ValidOperationID(in.OperationID) || in.Revision < 1 || !strings.HasPrefix(in.ID, "notification:") {
		return requestsettlement.Refuse(in, "invalid_request", "The report settlement input is invalid."), nil
	}
	notificationStoreMu.Lock()
	defer notificationStoreMu.Unlock()
	store, err := readNotifications()
	if err != nil {
		return requestsettlement.Receipt{}, err
	}
	operationActor := actor
	if command == "Withdraw" {
		operationActor = requestsettlement.ChatActor(actor)
	}
	op := requestsettlement.Key(operationActor, command, in.OperationID)
	if previous, ok := store.ReportOperations[op]; ok {
		if previous.Input != in {
			return requestsettlement.Refuse(in, "operation_changed", "This operation ID describes different input."), nil
		}
		return previous.Receipt, nil
	}
	source, err := a.notificationRequest(store, in.ID)
	if err != nil {
		return requestsettlement.Refuse(in, "request_unavailable", "The report is unavailable."), nil
	}
	if command != "Dismiss" && command != "Withdraw" {
		return requestsettlement.Refuse(in, "not_authorized", "This report has no answer command."), nil
	}
	if command == "Dismiss" && actor != "person" || command == "Withdraw" && (source.Report.RequestOwner == "" || source.Report.RequestOwner != actor) {
		return requestsettlement.Refuse(in, "not_authorized", "Only the person may dismiss a report and only its asking chat may withdraw it."), nil
	}
	if source.Status != "open" {
		r := requestsettlement.Refuse(in, "request_closed", "This report is already closed.")
		r.Outcome = source.Status
		return r, nil
	}
	if source.Revision != in.Revision {
		return requestsettlement.Refuse(in, "revision_changed", "This report changed."), nil
	}
	// A pending permission belongs to its provider, and is outside this stage.
	if source.Report.Kind == "attention" && !strings.HasPrefix(source.Report.PermissionKey, "task-q-") {
		return requestsettlement.Refuse(in, "dismissal_unavailable", "Dismiss is unavailable until a documented cancellation adapter is provided for this permission request."), nil
	}
	if source.Report.PermissionKey != "" || source.Report.Kind == "mention" && source.Report.Message != 0 {
		return requestsettlement.Refuse(in, "source_required", "Settle the original question before changing its notification."), nil
	}
	release, err := requestsettlement.Claim(configDir(), operationActor, command, in)
	if errors.Is(err, requestsettlement.ErrOperationChanged) {
		return requestsettlement.Refuse(in, "operation_changed", err.Error()), nil
	}
	if err != nil {
		return requestsettlement.Receipt{}, err
	}
	defer release()
	status, kind, wording := "dismissed", "person", "The person dismissed this report without answering."
	if command == "Withdraw" {
		status, kind, wording = "withdrawn", "owner", "The asking chat withdrew this report without answering."
	}
	source.Status, source.ActorKind, source.SettledAt = status, kind, notificationNow().UTC().Format(time.RFC3339Nano)
	source.Revision++
	source.Completion = notificationCompletion{Recipient: source.Report.RequestOwner, Outcome: status, Wording: wording}
	if store.ReportSettlements == nil {
		store.ReportSettlements = map[string]notificationSettlement{}
	}
	if store.ReportOperations == nil {
		store.ReportOperations = map[string]notificationOperation{}
	}
	store.ReportSettlements[in.ID] = source
	receipt := requestsettlement.Receipt{ID: in.ID, Revision: source.Revision, OperationID: in.OperationID, State: "committed", Outcome: status, SettledAt: source.SettledAt}
	store.ReportOperations[op] = notificationOperation{Input: in, Receipt: receipt}
	for i := range store.Items {
		if notificationRequestID(store.Items[i]) == in.ID {
			store.Items[i].Dismissed = true
		}
	}
	if source.Report.IncidentVersion != "" {
		if store.RecoveryDismissals == nil {
			store.RecoveryDismissals = map[string]string{}
		}
		store.RecoveryDismissals[source.Report.ID] = source.Report.IncidentVersion
	}
	if err := writeNotifications(store); err != nil {
		return requestsettlement.Receipt{}, err
	}
	return receipt, nil
}
