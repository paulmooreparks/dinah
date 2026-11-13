package main

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

func TestPermissionAttentionSurvivesCancellationAndRestartUntilAnswered(t *testing.T) {
	isolateWorkspaceSettings(t)
	a := NewApp()
	a.settings.SessionTitles = map[string]string{"chat": "Review"}
	native := &fakeNotifier{known: true}
	a.notices.setNotifier(native)
	ready := make(chan string, 2)
	a.sink = func(name string, data any) {
		if name == "agent:permission" {
			ready <- data.(map[string]any)["key"].(string)
		}
	}
	ask := func() chan struct{} {
		done := make(chan struct{})
		go func() {
			a.permissions().Ask(context.Background(), "chat", nil, json.RawMessage(`{"title":"Approve this step\nDo not show this second line"}`), nil)
			close(done)
		}()
		return done
	}
	done := ask()
	key := <-ready
	items, err := NewApp().Notifications()
	if err != nil || len(items) != 1 || items[0].Body != "Approve this step" || items[0].SessionTitle != "Review" || items[0].PermissionKey != key {
		t.Fatalf("saved attention: %+v %v", items, err)
	}
	if len(native.shown) != 1 {
		t.Fatal("native signal was not paired with saved attention")
	}
	a.permissions().CancelSession("chat")
	<-done
	a.Answer(key, "allow")
	items, err = NewApp().Notifications()
	if err != nil || len(items) != 1 {
		t.Fatalf("cancellation or a stale answer lost attention: %+v %v", items, err)
	}
	if err := a.DismissNotification(items[0].ID); err != nil {
		t.Fatal(err)
	}
	done = ask()
	key = <-ready
	a.Answer(key, "allow")
	<-done
	items, err = NewApp().Notifications()
	if err != nil || len(items) != 0 {
		t.Fatalf("real answer retained attention: %+v %v", items, err)
	}

}

func TestAttentionDoesNotAgeOutOrLoseToReportCapacity(t *testing.T) {
	isolateWorkspaceSettings(t)
	a := NewApp()
	a.keepQuestion("chat", "old", nil)
	previousClock, previousLimit := notificationNow, maxNotifications
	t.Cleanup(func() { notificationNow, maxNotifications = previousClock, previousLimit })
	notificationNow = func() time.Time { return time.Now().Add(30 * 24 * time.Hour) }
	maxNotifications = 1
	for _, id := range []string{"first", "second"} {
		if _, err := a.SaveNotification(Notification{ID: id, Kind: "app", Severity: "info", Title: id}); err != nil {
			t.Fatal(err)
		}
	}
	items, err := NewApp().Notifications()
	if err != nil || len(items) != 2 {
		t.Fatalf("capacity or age lost attention: %+v %v", items, err)
	}
	if err := a.DismissNotification(permissionNoticeID("old")); err != nil {
		t.Fatal(err)
	}
	items, err = a.Notifications()
	if err != nil || len(items) != 1 {
		t.Fatalf("dismissed attention stayed active: %+v %v", items, err)
	}
}
