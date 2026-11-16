package main

import (
	"context"
	"errors"
	"strings"

	"studio/internal/bus"
	"studio/internal/requestsettlement"
)

func (a *App) notificationByID(id string) (Notification, error) {
	if !notificationID.MatchString(id) {
		return Notification{}, errors.New("invalid notification identity")
	}
	notificationStoreMu.Lock()
	store, err := readNotifications()
	notificationStoreMu.Unlock()
	if err != nil {
		return Notification{}, err
	}
	for _, item := range store.Items {
		if item.ID == id {
			return item, nil
		}
	}
	incidents, err := a.recoveryNotifications()
	if err != nil {
		return Notification{}, err
	}
	for _, item := range incidents {
		if item.ID == id {
			return item, nil
		}
	}
	return Notification{}, errors.New("the notification is not stored")
}

func (a *App) pendingPermission(key string) bool {
	for _, p := range a.PendingPermissions() {
		if p["key"] == key {
			return true
		}
	}
	return false
}

// DismissNotification preserves its legacy input while committing the source
// question before its notification projection. It never supplies an answer.
func (a *App) DismissNotification(id string) error {
	item, err := a.notificationByID(id)
	if err != nil || item.Dismissed {
		return err
	}
	operation := "notification:" + requestsettlement.Key(id, item.IncidentVersion)
	if strings.HasPrefix(item.PermissionKey, "task-q-") {
		r, err := a.DismissRequest(requestsettlement.Input{ID: item.PermissionKey, Revision: 1, OperationID: operation})
		if err != nil {
			return err
		}
		if r.State != "committed" && r.Code != "request_closed" {
			return errors.New(r.Text)
		}
		return a.dismissNotificationProjection(id)
	}
	if item.PermissionKey != "" {
		if a.pendingPermission(item.PermissionKey) {
			return errors.New("Dismiss is unavailable because this provider permission request has no approved cancellation adapter.")
		}
		// This is history of an already ended permission, with no live waiter.
		return a.dismissNotificationProjection(id)
	}
	if item.Kind == "mention" && item.Message != 0 {
		err := a.busHost().With(func(b *bus.Bus) error {
			ctx := context.Background()
			requestID, err := b.PersonRequestIdentity(ctx, item.Message)
			if err != nil || requestID == "" {
				return err
			}
			d, err := b.DecisionForMessage(ctx, item.Message)
			if err != nil {
				return err
			}
			previous, err := b.RequestOperation(ctx, "person", "Dismiss", operation)
			if err != nil {
				return err
			}
			if previous.State == "committed" {
				if previous.ID != d.ID {
					return errors.New("The saved dismissal belongs to another request.")
				}
				return nil
			}
			r, err := b.DismissRequest(ctx, requestsettlement.Input{ID: d.ID, Revision: d.Revision, OperationID: operation})
			if err != nil {
				return err
			}
			if r.State != "committed" && r.Code != "request_closed" {
				return errors.New(r.Text)
			}
			return nil
		})
		if err != nil {
			return err
		}
		return a.dismissNotificationProjection(id)
	}
	id = notificationRequestID(item)
	r, err := a.DismissRequest(requestsettlement.Input{ID: id, Revision: 1, OperationID: operation})
	if err != nil {
		return err
	}
	if r.State != "committed" && r.Code != "request_closed" {
		return errors.New(r.Text)
	}
	return nil
}
