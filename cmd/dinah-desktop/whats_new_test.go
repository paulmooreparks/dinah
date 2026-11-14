package main

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestWhatsNewIsOneDurableCardPerHome(t *testing.T) {
	isolateWorkspaceSettings(t)
	a := NewApp()
	var wg sync.WaitGroup
	errors := make(chan error, 16)
	for range 16 {
		wg.Add(1)
		go func() { defer wg.Done(); errors <- a.keepWhatsNew() }()
	}
	wg.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatal(err)
		}
	}
	store, err := readNotifications()
	if err != nil || len(store.Items) != 1 || !keptWhatsNew(store.Items[0]) || store.Items[0].Created.IsZero() {
		t.Fatalf("first start: %+v %v", store, err)
	}
	if err := NewApp().DismissNotification(whatsNewNotificationID); err != nil {
		t.Fatal(err)
	}
	if err := NewApp().keepWhatsNew(); err != nil {
		t.Fatal(err)
	}
	store, err = readNotifications()
	if err != nil || len(store.Items) != 1 || !store.Items[0].Dismissed {
		t.Fatalf("dismissal: %+v %v", store, err)
	}
}

func TestWhatsNewSurvivesRetentionAndClearWithoutReappearing(t *testing.T) {
	isolateWorkspaceSettings(t)
	oldNow, oldMax := notificationNow, maxNotifications
	t.Cleanup(func() { notificationNow, maxNotifications = oldNow, oldMax })
	now := time.Date(2026, 11, 14, 0, 0, 0, 0, time.UTC)
	notificationNow = func() time.Time { return now }
	maxNotifications = 1
	a := NewApp()
	if err := a.keepWhatsNew(); err != nil {
		t.Fatal(err)
	}
	now = now.Add(30 * 24 * time.Hour)
	for _, id := range []string{"ordinary_1", "ordinary_2"} {
		if _, err := a.SaveNotification(Notification{ID: id, Kind: "app", Severity: "info", Title: "An ordinary report."}); err != nil {
			t.Fatal(err)
		}
	}
	items, err := a.Notifications()
	if err != nil || len(items) != 2 || !keptWhatsNew(items[0]) {
		t.Fatalf("active retention: %+v %v", items, err)
	}
	if err := a.ClearNotifications(); err != nil {
		t.Fatal(err)
	}
	now = now.Add(30 * 24 * time.Hour)
	if _, err := a.SaveNotification(Notification{ID: "ordinary_3", Kind: "app", Severity: "info", Title: "A later report."}); err != nil {
		t.Fatal(err)
	}
	if err := NewApp().keepWhatsNew(); err != nil {
		t.Fatal(err)
	}
	store, err := readNotifications()
	if err != nil || len(store.Items) != 2 || !keptWhatsNew(store.Items[0]) || !store.Items[0].Dismissed {
		t.Fatalf("tombstone: %+v %v", store, err)
	}
}

func TestWhatsNewPreservesAnUnreadableStore(t *testing.T) {
	isolateWorkspaceSettings(t)
	file := filepath.Join(configDir(), "notifications.json")
	if err := os.MkdirAll(filepath.Dir(file), 0700); err != nil {
		t.Fatal(err)
	}
	before := []byte("an unreadable saved store")
	if err := os.WriteFile(file, before, 0600); err != nil {
		t.Fatal(err)
	}
	if err := NewApp().keepWhatsNew(); err == nil {
		t.Fatal("replaced an unreadable notification store")
	}
	after, err := os.ReadFile(file)
	if err != nil || string(after) != string(before) {
		t.Fatalf("store changed: %q %v", after, err)
	}
}
