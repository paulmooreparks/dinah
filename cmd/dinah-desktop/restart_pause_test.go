package main

import (
	"os"
	"strings"
	"testing"
)

func TestPauseAndRestartWritesDownTheWorkingChatsAndHoldsTurns(t *testing.T) {
	a, path := watchedApp(t)
	if err := a.PauseAndRestart(); err == nil {
		t.Fatal("a pause was started with no newer build")
	}
	os.WriteFile(path, []byte("build two, longer"), 0o755)
	a.checkUpdate()
	info, err := a.NewSessionAt(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	a.session(info.ID).StartWork()
	if err := a.PauseAndRestart(); err != nil {
		t.Fatal(err)
	}
	st := a.UpdateState()
	if !st.Pending || !st.Pausing || len(st.Blockers) != 1 {
		t.Fatalf("the state is %+v", st)
	}
	if err := a.holdingTurns(); err == nil {
		t.Fatal("turns are not held while the restart waits for the chats to pause")
	}
	r := readPaused()
	if len(r.Chats) != 1 || r.Chats[0].Chat != info.ID || r.Chats[0].Told || r.Run == "" {
		t.Fatalf("the record is %+v", r)
	}
	// The window closing writes down that the chat was still working, and a
	// second look adds nothing.
	a.noteInterrupted()
	r = readPaused()
	if len(r.Chats) != 1 || r.Chats[0].Ended {
		t.Fatalf("after the close the record is %+v", r)
	}
	a.CancelRestart()
	if st := a.UpdateState(); st.Pending || st.Pausing || len(st.Blockers) != 0 {
		t.Fatalf("after cancelling the state is %+v", st)
	}
}

func TestAChatIsToldWhatHappenedToItsTurn(t *testing.T) {
	cut := resumeText(pausedChat{}, true)
	paused := resumeText(pausedChat{Told: true, Ended: true}, true)
	ended := resumeText(pausedChat{Ended: true}, true)
	cancelled := resumeText(pausedChat{Told: true}, false)
	if !strings.Contains(cut, "cut short") || !strings.Contains(paused, "You paused") || strings.Contains(ended, "cut short") || !strings.Contains(cancelled, "cancelled") {
		t.Fatalf("the messages are:\n%s\n%s\n%s\n%s", cut, paused, ended, cancelled)
	}
}
