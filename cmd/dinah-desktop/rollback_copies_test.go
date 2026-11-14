package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestRollbackCopiesPrecedeBothWritersAndAreNeverReplaced(t *testing.T) {
	isolateWorkspaceSettings(t)
	for _, name := range []string{"settings.json", "notifications.json"} {
		original := []byte("{\n  \"approvals\": \"ask\"\n}\n")
		if name == "notifications.json" {
			original = []byte("{\n  \"version\":1, \"items\":[]\n}\n")
		}
		path := filepath.Join(configDir(), name)
		if err := os.WriteFile(path, original, 0o600); err != nil {
			t.Fatal(err)
		}
		for range 2 {
			var err error
			if name == "settings.json" {
				err = saveSettings(Settings{BackgroundChatLimit: 11, AttentionReadSafe: true})
			} else {
				_, err = (&App{}).SaveNotification(Notification{ID: "attention_fixture", Kind: "attention", Severity: "info", Title: "Invented permission", SessionID: "fixture-chat", PermissionKey: "q1"})
			}
			if err != nil {
				t.Fatal(err)
			}
			kept, err := os.ReadFile(path + beforeDecisions)
			if err != nil || !bytes.Equal(kept, original) {
				t.Fatalf("%s copy %s %v", name, kept, err)
			}
		}
	}
}
