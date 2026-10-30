package main

import (
	"os"
	"testing"
)

// A turn that outlives the limit and writes after the app's folder is closed leaves no file, recreates no
// folder and does not panic.
func TestALateWriteAfterTheFolderClosesWritesNothing(t *testing.T) {
	a, _ := providerTestApp(t)
	dir := configDir()
	a.closeAgentLogs()
	if err := writeTranscript(&conversationTranscript{SessionID: "codex:late", Provider: "codex", Workspace: dir}); err != errFolderClosed {
		t.Fatalf("a late transcript write: %v", err)
	}
	if err := saveSettings(Settings{}); err != errFolderClosed {
		t.Fatalf("a late settings write: %v", err)
	}
	a.remember(dir, "codex:late", "send", "late", nil)
	a.journal("codex:late", "send")
	if _, _, err := a.newAppletChannel(nil, dir); err != errFolderClosed && a.appletBridge == nil {
		t.Fatalf("a late applet channel: %v", err)
	}
	for _, name := range []string{"conversations", "history.jsonl", "journal", "applet-hosts"} {
		if _, err := os.Stat(dir + string(os.PathSeparator) + name); err == nil {
			t.Fatalf("%s was written after the folder closed", name)
		}
	}
}
