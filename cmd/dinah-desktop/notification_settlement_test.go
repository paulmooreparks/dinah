package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"studio/internal/requestsettlement"
)

func TestReportSettlementKeepsTheSourceAndObligationInTheSameRecord(t *testing.T) {
	for _, command := range []string{"Dismiss", "Withdraw"} {
		t.Run(command, func(t *testing.T) {
			isolateWorkspaceSettings(t)
			a := NewApp()
			a.settings.BusSessions = map[string]string{"asking-chat": "window-original-owner"}
			saved, err := a.SaveNotification(Notification{ID: "report", Kind: "turn-failure", Severity: "error", Title: "Invented failure", Body: "The result is unknown.", Details: []string{"Do not repeat the work."}, SessionID: "asking-chat", Workspace: t.TempDir()})
			if err != nil {
				t.Fatal(err)
			}
			id := notificationRequestID(saved)
			before, err := a.NotificationRequest(id)
			if err != nil || before.Status != "open" || before.Report.RequestOwner != "window-original-owner" {
				t.Fatal(before, err)
			}
			in := requestsettlement.Input{ID: id, Revision: 1, OperationID: "close-report"}
			actor := "person"
			if command == "Withdraw" {
				actor = "window-original-owner"
			}
			storePath := filepath.Join(configDir(), "notifications.json")
			rawBefore, err := os.ReadFile(storePath)
			if err != nil {
				t.Fatal(err)
			}
			unsupported, err := a.settleNotification(in, actor, "Answer")
			rawAfter, readErr := os.ReadFile(storePath)
			if err != nil || readErr != nil || unsupported.State != "refused" || !reflect.DeepEqual(rawAfter, rawBefore) {
				t.Fatal("report acquired answer authority", unsupported, err, readErr)
			}
			first, err := a.settleNotification(in, actor, command)
			if err != nil || first.State != "committed" {
				t.Fatal(first, err)
			}
			restarted := NewApp()
			after, err := restarted.NotificationRequest(id)
			if err != nil || after.Status != first.Outcome || !reflect.DeepEqual(after.Report, before.Report) || after.Completion.Outcome != first.Outcome || after.Completion.Recipient != saved.RequestOwner {
				t.Fatal("report settlement lost its source or completion", after, err)
			}
			repeat, err := restarted.settleNotification(in, actor, command)
			if err != nil || !reflect.DeepEqual(repeat, first) {
				t.Fatal("report receipt changed after restart", repeat, err)
			}
			in.OperationID = "late"
			late, err := restarted.settleNotification(in, actor, command)
			if err != nil || late.Code != "request_closed" || late.Outcome != first.Outcome {
				t.Fatal(late, err)
			}
			items, err := restarted.Notifications()
			if err != nil || len(items) != 0 {
				t.Fatal("settled report remained open", items, err)
			}
		})
	}
}

func TestOwnerlessReportDoesNotInventARecipient(t *testing.T) {
	isolateWorkspaceSettings(t)
	a := NewApp()
	saved, err := a.SaveNotification(Notification{ID: "ownerless", Kind: "app", Severity: "warning", Title: "Invented report", RequestOwner: "forged"})
	if err != nil || saved.RequestOwner != "" {
		t.Fatal("caller supplied a report owner", saved, err)
	}
	in := requestsettlement.Input{ID: notificationRequestID(saved), Revision: 1, OperationID: "withdraw"}
	r, err := a.settleNotification(in, "forged", "Withdraw")
	if err != nil || r.Code != "not_authorized" {
		t.Fatal(r, err)
	}
	in.OperationID = "dismiss"
	r, err = a.DismissRequest(in)
	if err != nil || r.State != "committed" {
		t.Fatal(r, err)
	}
	source, err := a.NotificationRequest(in.ID)
	if err != nil || source.Completion.Recipient != "" || source.Status != "dismissed" {
		t.Fatal("ownerless report invented completion destination", source, err)
	}
}
