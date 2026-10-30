package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNotificationsSurviveRestartAndDurableDismissal(t *testing.T) {
	isolateWorkspaceSettings(t)
	workspace := t.TempDir()
	a := NewApp()
	n := Notification{ID: "failure_1", Kind: "turn-failure", Severity: "error", Title: "A step failed.", Body: "The agent reported a failure.", Details: []string{"Read file: permission denied"}, SessionID: "saved-chat", SessionTitle: "Review a Flow", Workspace: workspace, Provider: "codex"}
	saved, err := a.SaveNotification(n)
	if err != nil || saved.Created.IsZero() {
		t.Fatalf("save: %+v %v", saved, err)
	}
	b := NewApp()
	list, err := b.Notifications()
	if err != nil || len(list) != 1 || list[0].SessionID != n.SessionID || list[0].Workspace != workspace || list[0].Details[0] != n.Details[0] {
		t.Fatalf("restart: %+v %v", list, err)
	}
	if b.runtime.ConversationCountLocked() != 0 {
		t.Fatal("restoring notifications opened a chat")
	}
	if err := b.DismissNotification(n.ID); err != nil {
		t.Fatal(err)
	}
	c := NewApp()
	list, err = c.Notifications()
	if err != nil || len(list) != 0 {
		t.Fatalf("dismissal lost: %+v %v", list, err)
	}
	duplicate, err := c.SaveNotification(n)
	if err != nil || !duplicate.Dismissed {
		t.Fatal("late duplicate resurrected dismissed notice")
	}
	n.ID = "approval"
	n.Kind = "approval"
	if _, err := c.SaveNotification(n); err == nil {
		t.Fatal("persisted a live approval request")
	}
}

// Every toast is kept as a report: a document's with its draft, which the
// card reopens, and any other with its severity.
func TestReportsOfEveryKindAreKept(t *testing.T) {
	isolateWorkspaceSettings(t)
	a := NewApp()
	n := Notification{ID: "document_1", Kind: "document", Severity: "error", Title: `Publishing "Note 09" failed.`, Body: "A changed passage could not be found on the page.", Details: []string{}, DraftID: "f8036f48ac6d471e39aeb00fce5929e8"}
	if _, err := a.SaveNotification(n); err != nil {
		t.Fatal(err)
	}
	for i, severity := range []string{"error", "warning", "success", "info"} {
		if _, err := a.SaveNotification(Notification{ID: "app_" + severity, Kind: "app", Severity: severity, Title: "Report " + string(rune('A'+i)), Details: []string{}}); err != nil {
			t.Fatal(err)
		}
	}
	run := Notification{ID: "run_1", Kind: "run", Severity: "error", Title: `Review for "Note 09" failed.`, Details: []string{}, RunID: "r20261012", Workspace: t.TempDir(), DraftID: n.DraftID}
	if _, err := a.SaveNotification(run); err != nil {
		t.Fatal(err)
	}
	list, err := NewApp().Notifications()
	if err != nil || len(list) != 6 || list[0].DraftID != n.DraftID || list[0].Kind != "document" || list[4].Severity != "info" || list[5].RunID != "r20261012" {
		t.Fatalf("restart: %+v %v", list, err)
	}
	for _, bad := range []Notification{
		{ID: "d2", Kind: "document", Severity: "error", Title: "x", DraftID: "not-a-draft"},
		{ID: "d3", Kind: "document", Severity: "error", Title: "x", DraftID: n.DraftID, SessionID: "chat"},
		{ID: "d4", Kind: "document", Severity: "error", Title: "x", DraftID: n.DraftID, Workspace: "relative"},
		{ID: "e2", Kind: "app", Severity: "error", Title: "x", DraftID: n.DraftID},
		{ID: "e3", Kind: "app", Severity: "error", Title: "x", SessionID: "chat"},
		{ID: "e4", Kind: "app", Severity: "loud", Title: "x"},
		{ID: "e5", Kind: "turn-failure", Severity: "info", Title: "x", SessionID: "chat", Workspace: t.TempDir()},
		{ID: "r2", Kind: "run", Severity: "error", Title: "x", Workspace: t.TempDir()},
		{ID: "r3", Kind: "run", Severity: "error", Title: "x", RunID: "r1", Workspace: "relative"},
		{ID: "r4", Kind: "app", Severity: "error", Title: "x", RunID: "r1"},
	} {
		if _, err := a.SaveNotification(bad); err == nil {
			t.Fatalf("accepted %+v", bad)
		}
	}
}

// Reports stay for seven days, the store keeps the newest 500, and Clear All
// dismisses every report.
func TestReportsAgeOutAndClear(t *testing.T) {
	isolateWorkspaceSettings(t)
	t.Cleanup(func() { notificationNow = time.Now })
	kept := maxNotifications
	maxNotifications = 20
	t.Cleanup(func() { maxNotifications = kept })
	start := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)
	notificationNow = func() time.Time { return start }
	a := NewApp()
	if _, err := a.SaveNotification(Notification{ID: "old", Kind: "app", Severity: "info", Title: "Saved.", Details: []string{}}); err != nil {
		t.Fatal(err)
	}
	notificationNow = func() time.Time { return start.Add(8 * 24 * time.Hour) }
	if list, _ := a.Notifications(); len(list) != 0 {
		t.Fatalf("a report older than seven days is still shown: %+v", list)
	}
	for i := range maxNotifications + 3 {
		if _, err := a.SaveNotification(Notification{ID: fmt.Sprintf("r%d", i), Kind: "app", Severity: "info", Title: "Saved.", Details: []string{}}); err != nil {
			t.Fatal(err)
		}
	}
	list, _ := a.Notifications()
	if len(list) != maxNotifications || list[0].ID != "r3" {
		t.Fatalf("kept %d, first %s", len(list), list[0].ID)
	}
	if err := a.ClearNotifications(); err != nil {
		t.Fatal(err)
	}
	if list, _ := NewApp().Notifications(); len(list) != 0 {
		t.Fatalf("Clear All left %d", len(list))
	}
}

func TestNotificationsPreserveStoreOnFailure(t *testing.T) {
	isolateWorkspaceSettings(t)
	a := NewApp()
	n := Notification{ID: "failure", Kind: "turn-failure", Severity: "error", Title: "Failure", SessionID: "chat", Workspace: t.TempDir(), Details: []string{}}
	if _, err := a.SaveNotification(n); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(configDir(), "notifications.json")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path+".pending", 0700); err != nil {
		t.Fatal(err)
	}
	if err := a.DismissNotification(n.ID); err == nil {
		t.Fatal("failed dismissal reported success")
	}
	after, _ := os.ReadFile(path)
	if string(before) != string(after) {
		t.Fatal("failed dismissal changed durable notice")
	}
	if err := os.Remove(path + ".pending"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"version":99,"items":[]}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := a.SaveNotification(n); err == nil {
		t.Fatal("future schema overwritten")
	}
	if _, err := a.Notifications(); err == nil {
		t.Fatal("future schema silently dropped")
	}
}
