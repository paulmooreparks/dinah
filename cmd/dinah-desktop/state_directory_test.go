package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func stateTestWrite(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestStateDirectoryMigrationPreservesAllRecords(t *testing.T) {
	root := t.TempDir()
	old, target := filepath.Join(root, "legacy"), filepath.Join(root, "home", ".dinah")
	files := map[string]string{"settings.json": `{"workspace":"test"}`, "notes/example.md": "note", "journal/codex/thread.jsonl": "journal", "conversations/thread.json": "conversation", "applets/workspace/invocation.json": "applet result", "history.jsonl": "history", "devin-ask.json": "permissions", "empty": ""}
	for path, text := range files {
		stateTestWrite(t, filepath.Join(old, path), text)
	}
	if err := os.MkdirAll(filepath.Join(old, "empty-directory"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := migrateStateDirectory(old, target); err != nil {
		t.Fatal(err)
	}
	for path, want := range files {
		for _, base := range []string{old, target} {
			got, err := os.ReadFile(filepath.Join(base, path))
			if err != nil || string(got) != want {
				t.Fatalf("%s was not preserved: %q %v", base, got, err)
			}
		}
	}
	if info, err := os.Stat(filepath.Join(target, "empty-directory")); err != nil || !info.IsDir() {
		t.Fatal("empty directory was lost")
	}
	stateTestWrite(t, filepath.Join(target, "settings.json"), "changed after migration")
	stateTestWrite(t, filepath.Join(old, "settings.json"), "old app changed")
	if err := migrateStateDirectory(old, target); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(filepath.Join(target, "settings.json"))
	if string(got) != "changed after migration" {
		t.Fatal("completed migration ran twice")
	}
}

func TestStateDirectoryMigrationConflictAndRetry(t *testing.T) {
	root := t.TempDir()
	old, target := filepath.Join(root, "old"), filepath.Join(root, "new")
	stateTestWrite(t, filepath.Join(old, "settings.json"), `{"workspace":"original"}`)
	stateTestWrite(t, filepath.Join(old, "notes", "personal.md"), "personal")
	stateTestWrite(t, filepath.Join(target, "settings.json"), `{"workspace":"other"}`)
	if err := migrateStateDirectory(old, target); err == nil || !strings.Contains(err.Error(), "conflict") {
		t.Fatalf("conflict was not explicit: %v", err)
	}
	if _, err := os.Stat(filepath.Join(target, "notes", "personal.md")); !os.IsNotExist(err) {
		t.Fatal("migration copied data before checking all conflicts")
	}
	// Simulate an interrupted copy with its durable plan and one completed file.
	stateTestWrite(t, filepath.Join(target, "settings.json"), `{"workspace":"original"}`)
	plan, err := scanStateMigration(old)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(plan)
	stateTestWrite(t, filepath.Join(target, migrationPlanName), string(data))
	stateTestWrite(t, filepath.Join(target, ".state-copy-abandoned"), "partial temporary data")
	if err := migrateStateDirectory(old, target); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(filepath.Join(target, "notes", "personal.md"))
	if string(got) != "personal" {
		t.Fatal("interrupted migration did not resume")
	}
}

func TestStateDirectoryRejectsChangedSourceAndSpecialPaths(t *testing.T) {
	root := t.TempDir()
	old, target := filepath.Join(root, "old"), filepath.Join(root, "new")
	stateTestWrite(t, filepath.Join(old, "settings.json"), `{}`)
	plan, _ := scanStateMigration(old)
	data, _ := json.Marshal(plan)
	stateTestWrite(t, filepath.Join(target, migrationPlanName), string(data))
	stateTestWrite(t, filepath.Join(old, "settings.json"), `{"changed":true}`)
	if err := migrateStateDirectory(old, target); err == nil || !strings.Contains(err.Error(), "changed") {
		t.Fatalf("changed source accepted: %v", err)
	}
	if err := migrateStateDirectory(old, filepath.Join(old, "child")); err == nil {
		t.Fatal("nested target accepted")
	}
	linkRoot := t.TempDir()
	source := filepath.Join(linkRoot, "source")
	destination := filepath.Join(linkRoot, "destination")
	stateTestWrite(t, filepath.Join(source, "file"), "safe")
	if err := os.Symlink(filepath.Join(source, "file"), filepath.Join(source, "link")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if err := migrateStateDirectory(source, destination); err == nil {
		t.Fatal("symlink was followed")
	}
}

func TestStateDirectoryOverrideNeverMigrates(t *testing.T) {
	previous := configOverride
	configOverride = t.TempDir()
	t.Cleanup(func() { configOverride = previous })
	if err := initializeStateDirectory(); err != nil {
		t.Fatal(err)
	}
	if configDir() != configOverride {
		t.Fatal("test override ignored")
	}
	if _, err := os.Stat(filepath.Join(configOverride, migrationDoneName)); !os.IsNotExist(err) {
		t.Fatal("test override attempted migration")
	}
}
