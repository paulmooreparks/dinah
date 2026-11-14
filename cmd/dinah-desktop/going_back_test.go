package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"studio/internal/bus"
)

// These opt-in tests exchange invented state with the previous checkout.
// They do not use a person's settings, messages, credentials or browser data.
func goingBackRoot(t *testing.T) string {
	t.Helper()
	root := os.Getenv("DINAH_GOING_BACK_ROOT")
	if root == "" {
		t.Skip("run scripts/going-back-trial.ps1 for the cross-version check")
	}
	rel, err := filepath.Rel(os.TempDir(), root)
	if err != nil || !filepath.IsAbs(root) || rel == "." || strings.HasPrefix(rel, "..") {
		t.Fatal("fixture must be a child of the temporary directory")
	}
	return root
}

func TestGoingBackWriteCurrentState(t *testing.T) {
	root := goingBackRoot(t)
	previous := configOverride
	t.Cleanup(func() { configOverride = previous; closeAppLog() })
	for _, name := range []string{"full", "native", "legacy", "different-profile"} {
		home := filepath.Join(root, name)
		configOverride = home
		workspace := filepath.Join(home, "workspace")
		if err := os.MkdirAll(workspace, 0o700); err != nil {
			t.Fatal(err)
		}
		// These are invented pre-change files, including their byte formatting.
		originals := map[string][]byte{
			"settings.json":      []byte("{\n  \"approvals\":\"ask\",\"newWindowMode\":\"floating\",\"sessionTitles\":{\"fixture-chat\":\"Invented chat\"}\n}\n"),
			"notifications.json": []byte("{\n  \"version\":1,\"items\":[]\n}\n"),
		}
		for file, data := range originals {
			if err := os.WriteFile(filepath.Join(home, file), data, 0o600); err != nil {
				t.Fatal(err)
			}
		}
		baseline := int64(23)
		if err := saveSettings(Settings{BackgroundChatLimit: 11, AttentionReadSafe: true, MentionBaseline: &baseline, Approvals: ApprovalsAsk, NewWindowMode: "floating", SessionTitles: map[string]string{"fixture-chat": "Invented chat"}}); err != nil {
			t.Fatal(err)
		}
		a := &App{}
		if err := a.SaveUIState(UIState{Version: 1, Values: map[string]json.RawMessage{
			"dinah-decisions:request%3Afixture": json.RawMessage(`{"version":1,"answers":{"1":{"text":"Invented draft","asRecommended":false}},"reviewed":"","operation":null}`),
			"dinah-workbench-layout":            json.RawMessage(`"?open=workspace"`),
		}}); err != nil {
			t.Fatal(err)
		}
		items := []Notification{{ID: "ordinary", Kind: "app", Severity: "info", Title: "Invented ordinary card", Created: time.Now().UTC()}}
		if name == "full" || name == "native" {
			items = append(items,
				Notification{ID: "attention_fixture", Kind: "attention", Severity: "info", Title: "Invented permission", SessionID: "fixture-chat", PermissionKey: "q1", Created: time.Now().UTC()},
				Notification{ID: "restart_fixture", Kind: "restart-left", Severity: "warning", Title: "Invented recovery", Chats: []BusChatRef{{ID: "fixture-chat", Cwd: workspace, Title: "Invented chat"}}, Created: time.Now().UTC()})
		} else {
			// The new retention rule keeps this unresolved card even when old.
			items = append(items, Notification{ID: "mention-1", Kind: "mention", Message: 1, Severity: "info", Title: "Invented old mention", Created: time.Now().UTC().Add(-8 * 24 * time.Hour)})
		}
		if name == "legacy" {
			for i := 10; i < 512; i++ {
				items = append(items, Notification{ID: fmt.Sprintf("mention-%d", i), Kind: "mention", Message: int64(i), Severity: "info", Title: "Invented retained mention", Created: time.Now().UTC()})
			}
		}
		if err := writeNotifications(notificationStore{Version: 1, Items: items}); err != nil {
			t.Fatal(err)
		}
		if _, err := readNotifications(); err != nil {
			t.Fatal(err)
		}
		record := pausedRecord{Run: "invented-restart", WithoutUpdate: true, Chats: []pausedChat{{Chat: "fixture-chat", Bus: "fixture-asker", Title: "Invented chat", Cwd: workspace, Told: true, Ended: true}}}
		if err := writePaused(record); err != nil {
			t.Fatal(err)
		}
		if err := writeRestartAwaiting(record); err != nil {
			t.Fatal(err)
		}
		for file, original := range originals {
			data, err := os.ReadFile(filepath.Join(home, file+beforeDecisions))
			if err != nil || string(data) != string(original) {
				t.Fatalf("%s backup differs: %v", file, err)
			}
		}
		browserRoot := webviewRoot()
		for _, kind := range []string{profileMain, profileDocuments, profileShow} {
			folders := []string{"rollback-old" + profileSuffix[kind]}
			if name == "different-profile" {
				folders = []string{"earlier-a" + profileSuffix[kind], "earlier-b" + profileSuffix[kind]}
			}
			for _, folder := range folders {
				if err := os.MkdirAll(filepath.Join(browserRoot, folder), 0o700); err != nil {
					t.Fatal(err)
				}
			}
			chosen := webviewProfile(browserRoot, kind, "rollback-new")
			if err := os.WriteFile(filepath.Join(chosen, "invented-marker.txt"), []byte("invented browser marker"), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		if name != "full" && name != "native" {
			continue
		}
		ctx := context.Background()
		b, err := bus.OpenAt(filepath.Join(home, "bus.db"))
		if err != nil {
			t.Fatal(err)
		}
		me, err := b.RegisterSession(ctx, "fixture-asker", workspace, "Invented asker")
		if err != nil {
			t.Fatal(err)
		}
		d, err := b.AskPerson(ctx, me, bus.AskPersonInput{OperationID: "invented-ask", Questions: []bus.PersonQuestion{{Number: 1, Text: "Which invented option?", Recommendation: "Use A."}, {Number: 2, Text: "When?", Recommendation: "Tomorrow."}}})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := b.AnswerDecision(ctx, bus.AnswerDecisionInput{OperationID: "invented-answer", ID: d.ID, Revision: 1, Answers: []bus.DecisionAnswer{{Number: 1, AsRecommended: true}}}); err != nil {
			t.Fatal(err)
		}
		if _, err := b.PreviewDecisionBroadcast(ctx, "all", "Invented preview", nil); err != nil {
			t.Fatal(err)
		}
		if err := b.EndSession(ctx, me.ID); err != nil {
			t.Fatal(err)
		}
		if err := b.Close(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestGoingBackCurrentReadsAfterYesterday(t *testing.T) {
	root := goingBackRoot(t)
	previous := configOverride
	t.Cleanup(func() { configOverride = previous })
	configOverride = filepath.Join(root, "full")
	settings, err := loadSettings()
	if err != nil || settings.BackgroundChatLimit != 0 || settings.AttentionReadSafe {
		t.Fatalf("old rewrite result: %+v %v", settings, err)
	}
	if backgroundChatLimit(settings) != 8 {
		t.Fatal("returning to new build did not apply the default after the old rewrite")
	}
	b, err := bus.OpenAt(filepath.Join(configOverride, "bus.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	page, err := b.Decisions(context.Background(), 0)
	if err != nil || len(page.Items) != 1 || page.Items[0].Revision != 2 || !page.Items[0].Questions[0].Answered || page.Items[0].Questions[1].Answered {
		t.Fatalf("decision survived but old plain reply did not settle it: %+v %v", page, err)
	}
	receipt, err := b.DecisionOperation(context.Background(), "invented-answer")
	if err != nil || receipt.State != "committed" || receipt.Revision != 2 {
		t.Fatalf("receipt: %+v %v", receipt, err)
	}
	if r := readPaused(); !r.WithoutUpdate || len(r.Chats) != 1 {
		t.Fatalf("unconsumed restart record: %+v", r)
	}
	if store, err := readNotifications(); err != nil || len(store.Items) != 3 {
		t.Fatalf("new reader cannot recover untouched incompatible cards: %+v %v", store, err)
	}
	state, err := (&App{}).ReadUIState()
	if err != nil || state.Values["dinah-decisions:request%3Afixture"] == nil {
		t.Fatalf("decision draft lost: %+v %v", state, err)
	}
}
