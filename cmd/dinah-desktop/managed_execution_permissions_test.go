package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"studio/internal/agents"
)

func managedPermissionsFixture(t *testing.T) (string, []string) {
	t.Helper()
	old := configOverride
	configOverride = t.TempDir()
	t.Cleanup(func() { configOverride = old })
	root := filepath.Join(configDir(), "workspaces", "ws_test", "files")
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	roots := managedExecutionRoots([]WorkspaceRecord{{ID: "ws_test", ExecutionPath: root}})
	if len(roots) != 1 || roots[0] != root {
		t.Fatalf("managed roots: %v", roots)
	}
	return root, roots
}

func TestManagedExecutionPermissions(t *testing.T) {
	root, roots := managedPermissionsFixture(t)
	for _, path := range []string{
		filepath.Join(configDir(), "settings.json"),
		filepath.Join(configDir(), "workspaces", "ws_test", "metadata.json"),
		filepath.Join(configDir(), "workspaces", "ws_other", "files", "draft.md"),
		filepath.Join(root+"-other", "draft.md"),
	} {
		if agents.ProtectedPath(stateLayout(), path, roots...) == "" {
			t.Errorf("unregistered or protected path allowed: %s", path)
		}
		if err := agents.WriteTextFile(stateLayout(), path, "bad", roots...); err == nil {
			t.Errorf("protected write succeeded: %s", path)
		}
	}
	path := filepath.Join(root, "drafts", "one.md")
	if err := agents.WriteTextFile(stateLayout(), path, "draft", roots...); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "draft" {
		t.Fatalf("managed draft: %q, %v", data, err)
	}
	if agents.ProtectedPath(stateLayout(), path) == "" {
		t.Fatal("managed path was allowed without a registered root")
	}
	bad := managedExecutionRoots([]WorkspaceRecord{{ID: "ws_other", ExecutionPath: root}, {ID: "../escape", ExecutionPath: root}, {ID: "ws_test", ExecutionPath: configDir()}})
	if len(bad) != 0 {
		t.Fatalf("invalid managed records allowed: %v", bad)
	}
}

func TestManagedExecutionWriteRejectsLinks(t *testing.T) {
	root, roots := managedPermissionsFixture(t)
	outside := filepath.Join(configDir(), "private")
	if err := os.MkdirAll(outside, 0o700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "redirect")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symbolic links are unavailable: %v", err)
	}
	if err := agents.WriteTextFile(stateLayout(), filepath.Join(link, "settings.json"), "bad", roots...); err == nil {
		t.Fatal("managed write escaped through a nested link")
	}
	if _, err := os.Stat(filepath.Join(outside, "settings.json")); !os.IsNotExist(err) {
		t.Fatalf("escaped file exists: %v", err)
	}
	if err := os.Remove(link); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(root); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, root); err != nil {
		t.Fatal(err)
	}
	if err := agents.WriteTextFile(stateLayout(), filepath.Join(root, "settings.json"), "bad", roots...); err == nil {
		t.Fatal("managed write accepted a redirected execution root")
	}
}

func TestManagedExecutionDevinRules(t *testing.T) {
	root, roots := managedPermissionsFixture(t)
	read := func(path string) permissions {
		t.Helper()
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var cfg struct{ Permissions permissions }
		if err := json.Unmarshal(data, &cfg); err != nil {
			t.Fatal(err)
		}
		return cfg.Permissions
	}
	want := "Write(" + root + string(filepath.Separator) + "**)"
	for _, level := range []string{ApprovalsDinah, ApprovalsAll} {
		path, err := devinConfig(level, roots...)
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, rule := range read(path).Allow {
			found = found || rule == want
			if strings.Contains(rule, filepath.Join(configDir(), "workspaces")+string(filepath.Separator)+"**") {
				t.Fatal("permission grants the whole workspace store")
			}
		}
		if !found {
			t.Fatalf("%s did not allow the registered managed directory", level)
		}
		if _, err := devinConfig(level); err != nil {
			t.Fatal(err)
		}
		for _, rule := range read(path).Allow {
			if rule == want {
				t.Fatal("removed workspace permission survived the saved-rule merge")
			}
		}
	}
	path, err := devinConfig(ApprovalsAsk, roots...)
	if err != nil {
		t.Fatal(err)
	}
	if len(read(path).Allow) != 0 {
		t.Fatal("Ask approvals implicitly allowed managed writes")
	}
}

func TestManagedExecutionRecordStorage(t *testing.T) {
	root, roots := managedPermissionsFixture(t)
	want := filepath.Join(filepath.Dir(root), "runs")
	if got := stateLayout().RunDir(root); got != want {
		t.Fatalf("managed record directory %s, want %s", got, want)
	}
	for _, workspace := range []string{t.TempDir(), filepath.Join(root, "nested"), filepath.Join(configDir(), "applets", "hash")} {
		if got := stateLayout().RunDir(workspace); got != filepath.Join(workspace, ".dinah", "runs") {
			t.Fatalf("non-managed record directory changed: %s", got)
		}
	}
	store := stateLayout().Runs(root)
	if err := store.Write("managed-record", []byte(`{"id":"managed-record","status":"done"}`)); err != nil {
		t.Fatal(err)
	}
	if !store.Exists("managed-record") {
		t.Fatal("managed record was not found")
	}
	if data, err := store.Read("managed-record"); err != nil || !strings.Contains(string(data), `"status":"done"`) {
		t.Fatalf("managed record read: %s, %v", data, err)
	}
	if _, err := os.Stat(filepath.Join(root, ".dinah")); !os.IsNotExist(err) {
		t.Fatalf("managed record metadata was placed in writable files: %v", err)
	}
	if err := agents.WriteTextFile(stateLayout(), store.SlotPath("managed-record", 0), "bad", roots...); err == nil {
		t.Fatal("agent could change managed record metadata")
	}
}
