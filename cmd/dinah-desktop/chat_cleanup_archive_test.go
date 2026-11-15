package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCleanupArchiveMovesOutOfStartupFoldersAndResumes(t *testing.T) {
	_, _ = providerTestApp(t)
	path := transcriptPath("invented")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("retained transcript bytes"), 0o600); err != nil {
		t.Fatal(err)
	}
	r := chatCleanupArchive{Version: 1, Row: ChatCleanupRow{SessionID: "invented"}}
	if err := r.addFile(path); err != nil {
		t.Fatal(err)
	}
	if err := r.save(); err != nil {
		t.Fatal(err)
	}
	if err := r.moveFiles(false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("startup transcript still present %v", err)
	}
	resumed, err := readChatCleanupArchive("invented")
	if err != nil {
		t.Fatal(err)
	}
	if err = resumed.moveFiles(false); err != nil {
		t.Fatal(err)
	}
	if err = resumed.moveFiles(true); err != nil {
		t.Fatal(err)
	}
	if err = resumed.moveFiles(true); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "retained transcript bytes" {
		t.Fatalf("restored bytes %q %v", data, err)
	}
}

func TestCleanupArchiveRefusesConflictsAndEscapingPaths(t *testing.T) {
	_, _ = providerTestApp(t)
	r := chatCleanupArchive{Version: 1, Row: ChatCleanupRow{SessionID: "invented"}, Files: []chatCleanupArchiveFile{{Path: "../outside", Slot: "files/0"}}}
	if err := r.moveFiles(false); err == nil {
		t.Fatal("archive escaped its home")
	}
	path := filepath.Join(configDir(), "queues", "invented.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}
	r.Files = nil
	if err := r.addFile(path); err != nil {
		t.Fatal(err)
	}
	if err := r.moveFiles(false); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("different"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := r.moveFiles(true); err == nil {
		t.Fatal("restore overwrote another file")
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "different" {
		t.Fatal("conflict damaged active file", err)
	}
}
