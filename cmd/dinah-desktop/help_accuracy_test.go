package main

import (
	"os"
	"testing"
	"time"
)

func TestHelpTwoMinutePauseClaim(t *testing.T) {
	a, _ := providerTestApp(t)
	info, err := a.NewSessionAt(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	a.session(info.ID).StartWork()
	a.update = &updateWatch{pending: true, pausing: true, pausedAt: time.Now().Add(-time.Minute)}
	if a.UpdateState().Overdue {
		t.Fatal("notice is overdue before two minutes")
	}
	a.update.pausedAt = time.Now().Add(-3 * time.Minute)
	if st := a.UpdateState(); !st.Overdue || len(st.Blockers) != 1 {
		t.Fatalf("two-minute notice omits working chat: %+v", st)
	}
	if pauseGrace != 2*time.Minute {
		t.Fatal("documented grace period changed")
	}
}
func TestHelpInstalledBuildDoesNotRequestRestart(t *testing.T) {
	a, path := watchedApp(t)
	if err := os.WriteFile(path, []byte("new fixture program"), 0755); err != nil {
		t.Fatal(err)
	}
	a.checkUpdate()
	if st := a.UpdateState(); !st.Ready || st.Pending || a.update.restarting {
		t.Fatalf("installation observation requested restart: %+v", st)
	}
}
