package main

import (
	"path/filepath"
	"slices"
	"testing"

	"studio/internal/workspaces"
)

func writeSettingsFile(t *testing.T, text string) {
	t.Helper()
	stateTestWrite(t, filepath.Join(configDir(), "settings.json"), text)
}

// An install that starts without a settings file limits Ollama's file tools
// to the workspace folders; one that has settings and no choice, as every
// install from before the choice does, keeps going anywhere; and a value
// nobody wrote is read the same way.
func TestTheDefaultFileScopeOfANewAndAnExistingInstall(t *testing.T) {
	providerTestApp(t)
	fresh, err := loadSettings()
	if err != nil || fresh.OllamaFileScope != ollamaScopeWorkspace || ollamaFileScope(fresh) != ollamaScopeWorkspace {
		t.Fatalf("a new install: %q %v", fresh.OllamaFileScope, err)
	}
	writeSettingsFile(t, `{"approvals":"devin"}`)
	existing, err := loadSettings()
	if err != nil || existing.OllamaFileScope != "" || ollamaFileScope(existing) != ollamaScopeAnywhere {
		t.Fatalf("an existing install: %q %v", existing.OllamaFileScope, err)
	}
	writeSettingsFile(t, `{"ollamaFileScope":"everywhere-and-more","ollamaFileFolders":["relative","`+filepath.ToSlash(t.TempDir())+`"]}`)
	odd, err := loadSettings()
	if err != nil || ollamaFileScope(odd) != ollamaScopeAnywhere || len(odd.OllamaFileFolders) != 1 {
		t.Fatalf("an unknown value: %q %v, folders %v", odd.OllamaFileScope, err, odd.OllamaFileFolders)
	}
	writeSettingsFile(t, `{"ollamaFileScope":"folders"}`)
	kept, _ := loadSettings()
	if ollamaFileScope(kept) != ollamaScopeFolders {
		t.Fatalf("a stored choice: %q", kept.OllamaFileScope)
	}
}

// The person's choice is validated, saved in one write and reported in the
// state; a scope that is not one of the three and a folder that is not a full
// path are refused and change nothing.
func TestChoosingWhereOllamasFileToolsMayGo(t *testing.T) {
	a, _ := providerTestApp(t)
	one, two := t.TempDir(), t.TempDir()
	if err := a.SetOllamaFileScope("folders", []string{one, two, one, ""}); err != nil {
		t.Fatal(err)
	}
	st := a.State()
	if st.OllamaFileScope != ollamaScopeFolders || !slices.Equal(st.OllamaFileFolders, []string{one, two}) {
		t.Fatalf("the state: %q %v", st.OllamaFileScope, st.OllamaFileFolders)
	}
	reloaded, err := loadSettings()
	if err != nil || reloaded.OllamaFileScope != ollamaScopeFolders || !slices.Equal(reloaded.OllamaFileFolders, []string{one, two}) {
		t.Fatalf("what was saved: %q %v %v", reloaded.OllamaFileScope, reloaded.OllamaFileFolders, err)
	}
	if err := a.SetOllamaFileScope("sometimes", nil); err == nil {
		t.Fatal("an unknown scope was accepted")
	}
	if err := a.SetOllamaFileScope("folders", []string{"relative/path"}); err == nil {
		t.Fatal("a folder that is not a full path was accepted")
	}
	if a.State().OllamaFileScope != ollamaScopeFolders {
		t.Fatal("a refused choice changed the setting")
	}
	if err := a.SetOllamaFileScope("anywhere", nil); err != nil || a.State().OllamaFileScope != ollamaScopeAnywhere {
		t.Fatalf("anywhere: %v", err)
	}
}

// The chat's workspace folders are its own folder and the local sources of the
// workspace it belongs to, and the listed folders are added to them; with
// anywhere there is no limit. The function the connection asks reads the
// settings at each call.
func TestWhichFoldersAChatsFileToolsMayReach(t *testing.T) {
	a, _ := providerTestApp(t)
	cwd, source, listed := t.TempDir(), t.TempDir(), t.TempDir()
	a.mu.Lock()
	a.settings.WorkspaceRecords = []workspaces.Record{
		{ID: "w1", Name: "Mine", ExecutionPath: cwd, Sources: []workspaces.Source{{ID: "s1", Kind: "local", Path: source}, {ID: "s2", Kind: "web", Name: "A page"}}},
		{ID: "w2", Name: "Other", ExecutionPath: t.TempDir(), Sources: []workspaces.Source{{ID: "s3", Kind: "local", Path: t.TempDir()}}},
	}
	a.settings.OllamaFileScope, a.settings.OllamaFileFolders = ollamaScopeWorkspace, nil
	a.mu.Unlock()
	folders, limited := a.ollamaFileScopeFor("chat", cwd)
	if !limited || !slices.Equal(folders, []string{filepath.Clean(cwd), filepath.Clean(source)}) {
		t.Fatalf("workspace folders: %v %v", folders, limited)
	}
	if err := a.SetOllamaFileScope("folders", []string{listed}); err != nil {
		t.Fatal(err)
	}
	if folders, limited = a.ollamaFileScopeFor("chat", cwd); !limited || !slices.Equal(folders, []string{filepath.Clean(cwd), filepath.Clean(source), listed}) {
		t.Fatalf("with a listed folder: %v %v", folders, limited)
	}
	// A chat in no registered workspace has its own folder alone.
	stranger := t.TempDir()
	if folders, _ = a.ollamaFileScopeFor("chat", stranger); !slices.Equal(folders, []string{filepath.Clean(stranger), listed}) {
		t.Fatalf("a chat in no workspace: %v", folders)
	}
	if err := a.SetOllamaFileScope("anywhere", nil); err != nil {
		t.Fatal(err)
	}
	if folders, limited = a.ollamaFileScopeFor("chat", cwd); limited || folders != nil {
		t.Fatalf("anywhere: %v %v", folders, limited)
	}
}
