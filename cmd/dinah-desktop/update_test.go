package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// watchedApp returns an app watching a stand-in for the installed build.
func watchedApp(t *testing.T) (*App, string) {
	t.Helper()
	a, _ := providerTestApp(t)
	path := filepath.Join(t.TempDir(), "dinah-desktop.exe")
	if err := os.WriteFile(path, []byte("build one"), 0o755); err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(path)
	sum, _ := fileHash(path)
	a.update = &updateWatch{path: path, hash: sum, size: info.Size(), mod: info.ModTime()}
	return a, path
}

func TestANewerInstalledBuildIsNoticed(t *testing.T) {
	a, path := watchedApp(t)
	a.checkUpdate()
	if a.UpdateState().Ready {
		t.Fatal("the build the window runs was taken for a newer one")
	}
	// The same bytes written again are not a new build.
	later := time.Now().Add(time.Minute)
	os.Chtimes(path, later, later)
	a.checkUpdate()
	if a.UpdateState().Ready {
		t.Fatal("a file touched but unchanged was taken for a newer build")
	}
	if err := os.WriteFile(path, []byte("build two, longer"), 0o755); err != nil {
		t.Fatal(err)
	}
	a.checkUpdate()
	if !a.UpdateState().Ready {
		t.Fatal("a newer build was not noticed")
	}
}

func TestWaitingToRestartHoldsNewTurns(t *testing.T) {
	a, path := watchedApp(t)
	if err := a.RestartToUpdate(false); err == nil {
		t.Fatal("a restart was started with no newer build")
	}
	os.WriteFile(path, []byte("build two, longer"), 0o755)
	a.checkUpdate()
	info, err := a.NewSessionAt(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	s := a.session(info.ID)
	s.StartWork()
	if err := a.RestartToUpdate(false); err != nil {
		t.Fatal(err)
	}
	if st := a.UpdateState(); !st.Pending || st.Working != 1 {
		t.Fatalf("the state is %+v", st)
	}
	if err := a.Send(info.ID, "One more thing.", nil, nil); err == nil || !strings.Contains(err.Error(), "still in the box") {
		t.Fatalf("a new prompt was not held: %v", err)
	}
	if err := a.Queue(info.ID, "And this.", nil, nil); err == nil {
		t.Fatal("a queued prompt was not held")
	}
	a.CancelRestart()
	if a.UpdateState().Pending {
		t.Fatal("cancelling left the restart waiting")
	}
	if err := a.holdingTurns(); err != nil {
		t.Fatalf("turns are still held after cancelling: %v", err)
	}
}

func TestANewBuildWaitsForTheOldWindow(t *testing.T) {
	got := afterProcessEnds([]string{"dinah-desktop.exe", "--after", "999999999", "--bus-mcp"})
	if len(got) != 2 || got[1] != "--bus-mcp" {
		t.Fatalf("the arguments after --after are %v", got)
	}
	if got := afterProcessEnds([]string{"dinah-desktop.exe"}); len(got) != 1 {
		t.Fatalf("plain arguments changed: %v", got)
	}
}
