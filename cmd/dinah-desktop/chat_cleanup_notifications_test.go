package main

import (
	"testing"
	"time"
)

func TestCleanupRetainsThenErasesOnlyChatOwnedNotifications(t *testing.T) {
	a, _ := providerTestApp(t)
	id := "notice-chat"
	cleanupFixtureChat(t, a, id, "codex", t.TempDir(), "")
	store := notificationStore{Version: 1, Items: []Notification{
		{ID: "chat-card", Kind: "attention", Severity: "info", Title: "Invented question", SessionID: id, PermissionKey: "fixture", Created: time.Now().UTC()},
		{ID: "unrelated-card", Kind: "app", Severity: "info", Title: "Keep unrelated card", Created: time.Now().UTC()},
	}}
	if err := writeNotifications(store); err != nil {
		t.Fatal(err)
	}
	if err := a.RemoveChat(id); err != nil {
		t.Fatal(err)
	}
	visible, err := a.Notifications()
	if err != nil || len(visible) != 1 || visible[0].ID != "unrelated-card" {
		t.Fatalf("visible %+v %v", visible, err)
	}
	retained, err := readNotifications()
	if err != nil || len(retained.Items) != 2 {
		t.Fatalf("retained %+v %v", retained, err)
	}
	if restored := a.RestoreRemovedChat(id); !restored.OK {
		t.Fatal(restored)
	}
	visible, err = a.Notifications()
	if err != nil || len(visible) != 2 {
		t.Fatalf("restored %+v %v", visible, err)
	}
	if err := cleanupEraseNotifications([]string{id}); err != nil {
		t.Fatal(err)
	}
	retained, err = readNotifications()
	if err != nil || len(retained.Items) != 1 || retained.Items[0].ID != "unrelated-card" {
		t.Fatalf("erased %+v %v", retained, err)
	}
}
