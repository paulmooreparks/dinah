package main

const whatsNewNotificationID = "whats-new-20261114"

func keptWhatsNew(item Notification) bool {
	return item.Kind == "app" && item.ID == whatsNewNotificationID
}

func (a *App) keepWhatsNew() error {
	_, _, err := a.saveNotification(Notification{
		ID: whatsNewNotificationID, Kind: "app", Severity: "info", Title: "What's New",
		Body:    "Open Decisions to read questions and recommendations and send your answers. Open Notifications to find requests you missed, and read Working with Chats for help with Broadcast and Pause Chats and Restart. In Settings, you can choose how many chats open in the background and remove older program copies. Running copies and the newest copy kept for going back cannot be removed. If your product has Updates, you can compare installed and running versions there.",
		Details: []string{},
	})
	return err
}
