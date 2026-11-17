package main

import "errors"

// notificationByID reads one kept notice, dismissed or not. DismissNotification
// itself is in notifications.go, and it removes the notice only.
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
