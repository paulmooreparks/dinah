package main

import (
	"errors"
	"testing"
)

func TestRestartLeftGroupsFailedChatsAndSurvivesReload(t *testing.T) {
	a, _ := providerTestApp(t)
	cwd := t.TempDir()
	r := pausedRecord{Run: "restart-one", Chats: []pausedChat{{Chat: "a", Title: "First chat"}, {Chat: "b", Title: "Second chat", Cwd: cwd}, {Chat: "ok", Title: "Opened chat"}}}
	if err := writeRestartAwaiting(r); err != nil {
		t.Fatal(err)
	}
	a.restartChatOpened("a", cwd, errors.New("unverified first turn"))
	a.restartChatOpened("b", "", errors.New("provider unavailable"))
	a.restartChatOpened("ok", cwd, nil)
	a.restartChatOpened("ok", cwd, errors.New("later unrelated failure"))
	a.restartChatOpened("a", cwd, errors.New("repeated callback"))
	a.restartChatOpened("unrelated", cwd, errors.New("unrelated failure"))
	items, err := a.Notifications()
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || len(items[0].Chats) != 2 || items[0].Chats[0].Title != "First chat" || items[0].Chats[1].ID != "b" {
		t.Fatalf("notifications: %+v", items)
	}
	b, _ := restarted(t, a)
	again, err := b.Notifications()
	if err != nil {
		t.Fatal(err)
	}
	if len(again) != 1 || again[0].ID != items[0].ID || len(again[0].Chats) != 2 {
		t.Fatalf("reloaded: %+v", again)
	}
	if err = b.DismissNotification(items[0].ID); err != nil {
		t.Fatal(err)
	}
	b.restartChatOpened("a", cwd, errors.New("late callback"))
	again, err = b.Notifications()
	if err != nil || len(again) != 0 {
		t.Fatalf("dismissed: %+v %v", again, err)
	}
}

func TestRestartLeftRejectsInvalidChatTargets(t *testing.T) {
	n := Notification{ID: "restart-left-test", Kind: "restart-left", Severity: "warning", Title: "Needs recovery", Chats: []BusChatRef{{ID: "a", Title: "Chat", Cwd: "relative"}}}
	if validateNotification(n) == nil {
		t.Fatal("relative workspace accepted")
	}
	n.Chats[0].Cwd = t.TempDir()
	if err := validateNotification(n); err != nil {
		t.Fatal(err)
	}
}
