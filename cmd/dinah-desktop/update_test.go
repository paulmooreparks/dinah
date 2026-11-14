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

func TestWaitingToRestartHoldsNewWritesAndWaitsForOneUnderWay(t *testing.T) {
	a, path := watchedApp(t)
	os.WriteFile(path, []byte("build two, longer"), 0o755)
	a.checkUpdate()
	done, err := a.admitWrite()
	if err != nil {
		t.Fatalf("a write was refused with no restart waiting: %v", err)
	}
	a.updMu.Lock()
	a.update.pending = true
	a.updMu.Unlock()
	if st := a.UpdateState(); st.Writing != 1 {
		t.Fatalf("the state is %+v with one write under way", st)
	}
	if _, err := a.admitWrite(); err == nil || !strings.Contains(err.Error(), "after the restart") {
		t.Fatalf("a new write was admitted while the restart waits: %v", err)
	}
	finished := make(chan struct{})
	go func() { a.restartWhenIdle(); close(finished) }()
	select {
	case <-finished:
		t.Fatal("the restart did not wait for the write under way")
	case <-time.After(700 * time.Millisecond):
	}
	done()
	done()
	select {
	case <-finished:
	case <-time.After(5 * time.Second):
		t.Fatal("the restart did not go on once the write had ended")
	}
	if st := a.UpdateState(); st.Writing != 0 {
		t.Fatalf("the state is %+v after the write ended", st)
	}
	a.CancelRestart()
	if done, err := a.admitWrite(); err != nil {
		t.Fatalf("writes are still held after cancelling: %v", err)
	} else {
		done()
	}
}

// The launch seam records the program and arguments without starting a process.
func TestSameProgramRestartUsesRunningExecutableAndKeepsItsTarget(t *testing.T) {
	a, _ := providerTestApp(t)
	done, err := a.admitWrite()
	if err != nil {
		t.Fatal(err)
	}
	defer done()
	if err := a.PauseAndRestart(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(a.CancelRestart)
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	a.updMu.Lock()
	target := a.update.target
	a.update.pausedAt = time.Now().Add(-pauseGrace)
	a.updMu.Unlock()
	if target != self || a.UpdateState().Ready {
		t.Fatalf("target %q, self %q", target, self)
	}
	if err := a.holdingTurns(); err == nil {
		t.Fatal("new turns were admitted")
	}
	if _, err := a.admitWrite(); err == nil {
		t.Fatal("new writes were admitted")
	}
	if err := a.PauseAndRestart(); err == nil {
		t.Fatal("duplicate pause was admitted")
	}
	// A later installed update must not retarget an already requested restart.
	a.updMu.Lock()
	a.update.path, a.update.ready = "later-installed-build", true
	a.updMu.Unlock()
	if err := a.RestartToUpdate(true); err != nil {
		t.Fatal(err)
	}
	launched := false
	a.startUpdateUsing(func(path string, args ...string) error {
		launched = true
		if path != self || len(args) != 2 || args[0] != "--after" {
			t.Fatalf("launch %q %v", path, args)
		}
		return nil
	})
	if !launched {
		t.Fatal("same-program restart did not launch")
	}
}

func TestRestartNowOverridesSameProgramPauseWithoutUpdate(t *testing.T) {
	a, _ := providerTestApp(t)
	info, err := a.NewSessionAt(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	a.session(info.ID).StartWork()
	if err := a.PauseAndRestart(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(a.CancelRestart)
	a.updMu.Lock()
	a.update.pausedAt = time.Now().Add(-pauseGrace)
	a.updMu.Unlock()
	if st := a.UpdateState(); st.Ready || !st.Pending || !st.Pausing || !st.Overdue {
		t.Fatalf("state %+v", st)
	}
	if r := readPaused(); !r.WithoutUpdate || len(r.Chats) != 1 {
		t.Fatalf("record %+v", r)
	}
	if err := a.RestartToUpdate(true); err != nil {
		t.Fatal(err)
	}
	a.updMu.Lock()
	closing := a.update.restarting
	a.updMu.Unlock()
	if !closing {
		t.Fatal("Restart Now did not close")
	}
	if r := readPaused(); !r.WithoutUpdate || r.Chats[0].Ended {
		t.Fatalf("interrupted record %+v", r)
	}
}
