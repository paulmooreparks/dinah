package main

import "slices"

// Binning suppresses chat-owned cards without altering their retained content.
func (a *App) cleanupVisibleNotification(item Notification) (Notification, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if _, removed := a.settings.RemovedChats[item.SessionID]; item.SessionID != "" && removed {
		return item, false
	}
	if _, removed := a.settings.RemovedChats[item.Lead]; item.Lead != "" && removed {
		return item, false
	}
	if len(item.Chats) != 0 {
		item.Chats = slices.Clone(item.Chats)
		item.Chats = slices.DeleteFunc(item.Chats, func(chat BusChatRef) bool {
			_, removed := a.settings.RemovedChats[chat.ID]
			return removed
		})
		return item, len(item.Chats) != 0
	}
	return item, true
}

func cleanupEraseNotifications(ids []string) error {
	notificationStoreMu.Lock()
	defer notificationStoreMu.Unlock()
	store, err := readNotifications()
	if err != nil {
		return err
	}
	store.Items = slices.DeleteFunc(store.Items, func(item Notification) bool {
		return slices.Contains(ids, item.SessionID) || slices.Contains(ids, item.Lead)
	})
	for index := range store.Items {
		item := &store.Items[index]
		item.Chats = slices.DeleteFunc(item.Chats, func(chat BusChatRef) bool { return slices.Contains(ids, chat.ID) })
	}
	store.Items = slices.DeleteFunc(store.Items, func(item Notification) bool { return item.Kind == "restart-left" && len(item.Chats) == 0 })
	return writeNotifications(store)
}
