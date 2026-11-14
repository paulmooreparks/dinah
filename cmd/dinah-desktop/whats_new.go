package main

const whatsNewNotificationID = "whats-new-20261114"

func keptWhatsNew(item Notification) bool {
	return item.Kind == "app" && item.ID == whatsNewNotificationID
}

func (a *App) keepWhatsNew() error {
	_, _, err := a.saveNotification(Notification{
		ID: whatsNewNotificationID, Kind: "app", Severity: "info", Title: "What's New",
		Body:    "Decisions keeps questions and recommendations in one place. Notifications keeps requests you missed, and Working with Chats explains Broadcast and Pause Chats and Restart. Settings lets you choose how many chats can open in the background and remove older program copies while protecting running copies and the newest fallback. Products with Updates also show installed and running versions there.",
		Details: []string{},
	})
	return err
}
