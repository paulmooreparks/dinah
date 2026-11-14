//go:build windows && quality_set_aside

package programs_test

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sync"
	"syscall"
	"testing"
	"time"

	"studio/internal/programs"
)

// These black-box checks were authored from contract 9579730e before reading
// implementation code. The opt-in tag keeps them separate until integration.
func TestQualitySetAsideChild(t *testing.T) {
	if os.Getenv("QUALITY_SET_ASIDE_CHILD") != "1" {
		return
	}
	fmt.Println("ready")
	_, _ = io.Copy(io.Discard, os.Stdin)
	os.Exit(0)
}

func qualityCopy(t *testing.T, dir, name string, executable bool) string {
	t.Helper()
	data := []byte("invented program " + name)
	if executable {
		self, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		data, err = os.ReadFile(self)
		if err != nil {
			t.Fatal(err)
		}
	}
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, data, 0600); err != nil {
		t.Fatal(err)
	}
	return p
}

func qualityStart(t *testing.T, p string) uint32 {
	t.Helper()
	cmd := exec.Command(p, "-test.run=^TestQualitySetAsideChild$")
	cmd.Env = append(os.Environ(), "QUALITY_SET_ASIDE_CHILD=1")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	in, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	var once sync.Once
	t.Cleanup(func() { once.Do(func() { _ = in.Close(); _ = cmd.Process.Kill(); _ = cmd.Wait() }) })
	ready := make(chan bool, 1)
	go func() { s := bufio.NewScanner(out); ready <- s.Scan() && s.Text() == "ready" }()
	select {
	case ok := <-ready:
		if !ok {
			t.Fatal("hidden child did not acknowledge readiness")
		}
	case <-time.After(15 * time.Second):
		t.Fatal("hidden child readiness deadline")
	}
	return uint32(cmd.Process.Pid)
}

func qualityItem(t *testing.T, s *programs.SetAsideService, name string) programs.SetAsideProgram {
	t.Helper()
	list, err := s.List()
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range list.Items {
		if item.Name == name {
			return item
		}
	}
	t.Fatalf("missing candidate %s", name)
	return programs.SetAsideProgram{}
}

func qualityUnchanged(t *testing.T, p string) func() {
	t.Helper()
	before, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(data)
	return func() {
		t.Helper()
		after, err := os.Stat(p)
		if err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		if !os.SameFile(before, after) || digest != sha256.Sum256(data) {
			t.Fatalf("protected file changed: %s", p)
		}
	}
}

func qualityRemove(t *testing.T, s *programs.SetAsideService, item programs.SetAsideProgram, state, reason string) {
	t.Helper()
	result, err := s.Remove(programs.RemoveSetAsideInput{IDs: []string{item.ID}})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Items) != 1 || result.Items[0].ID != item.ID || result.Items[0].State != state {
		t.Fatalf("removal result: %+v", result)
	}
	if reason != "" && result.Items[0].ReasonCode != reason {
		t.Fatalf("refusal reason: %+v", result)
	}
	if state == "refused" && result.RemovedBytes != 0 {
		t.Fatalf("refusal reported removed bytes: %+v", result)
	}
	if state == "removed" && result.RemovedBytes != item.SizeBytes {
		t.Fatalf("removed byte count: %+v", result)
	}
}

func TestQualitySetAsideProtection(t *testing.T) {
	old := programs.FileName("window") + ".old-20261113010000000"
	newest := programs.FileName("window") + ".old-20261114010000000"
	t.Run("running set-aside and same basename elsewhere", func(t *testing.T) {
		a, b := t.TempDir(), t.TempDir()
		running := qualityCopy(t, a, old, true)
		fallbackA := qualityCopy(t, a, newest, false)
		other := qualityCopy(t, b, old, true)
		fallbackB := qualityCopy(t, b, newest, false)
		checkRun, checkA, checkB := qualityUnchanged(t, running), qualityUnchanged(t, fallbackA), qualityUnchanged(t, fallbackB)
		pid := qualityStart(t, running)
		sa, sb := programs.NewSetAsideService(a, []string{"window"}), programs.NewSetAsideService(b, []string{"window"})
		ri, oi := qualityItem(t, sa, old), qualityItem(t, sb, old)
		if ri.CanRemove || !slices.Contains(ri.RunningPids, pid) {
			t.Fatalf("running identity not protected: %+v", ri)
		}
		if !oi.CanRemove || len(oi.RunningPids) != 0 {
			t.Fatalf("different file mistaken for running image: %+v", oi)
		}
		qualityRemove(t, sa, ri, "refused", "in_use")
		qualityRemove(t, sa, qualityItem(t, sa, newest), "refused", "fallback")
		qualityRemove(t, sb, oi, "removed", "")
		if _, err := os.Stat(other); !os.IsNotExist(err) {
			t.Fatalf("selected eligible file remains: %v", err)
		}
		checkRun()
		checkA()
		checkB()
	})
	t.Run("starts after list", func(t *testing.T) {
		dir := t.TempDir()
		p := qualityCopy(t, dir, old, true)
		f := qualityCopy(t, dir, newest, false)
		check, checkF := qualityUnchanged(t, p), qualityUnchanged(t, f)
		s := programs.NewSetAsideService(dir, []string{"window"})
		item := qualityItem(t, s, old)
		if !item.CanRemove {
			t.Fatalf("initial item not removable: %+v", item)
		}
		qualityStart(t, p)
		qualityRemove(t, s, item, "refused", "in_use")
		check()
		checkF()
	})
	t.Run("manual pattern and fallback precedence", func(t *testing.T) {
		dir := t.TempDir()
		s := programs.NewSetAsideService(dir, []string{"window", "command"})
		legacy := programs.FileName("window") + ".before-test-20261115-010000"
		qualityCopy(t, dir, legacy, false)
		fallback := qualityCopy(t, dir, newest, false)
		tie := qualityCopy(t, dir, newest+"-tie", false)
		manualFallback := programs.FileName("command") + ".before-test-20261115-010000"
		mf := qualityCopy(t, dir, manualFallback, false)
		checks := []func(){qualityUnchanged(t, fallback), qualityUnchanged(t, tie), qualityUnchanged(t, mf)}
		item := qualityItem(t, s, legacy)
		if item.Fallback || !item.CanRemove {
			t.Fatalf("manual copy displaced old fallback: %+v", item)
		}
		qualityRemove(t, s, item, "removed", "")
		for _, name := range []string{newest, newest + "-tie", manualFallback} {
			item := qualityItem(t, s, name)
			if !item.Fallback || item.CanRemove {
				t.Fatalf("fallback flags: %+v", item)
			}
			qualityRemove(t, s, item, "refused", "fallback")
		}
		for _, check := range checks {
			check()
		}
	})
	t.Run("replacement invalidates selection", func(t *testing.T) {
		dir := t.TempDir()
		p := qualityCopy(t, dir, old, false)
		qualityCopy(t, dir, newest, false)
		s := programs.NewSetAsideService(dir, []string{"window"})
		item := qualityItem(t, s, old)
		if err := os.Rename(p, p+".unlisted"); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, bytes.Repeat([]byte("replacement"), 30), 0600); err != nil {
			t.Fatal(err)
		}
		check := qualityUnchanged(t, p)
		qualityRemove(t, s, item, "refused", "changed_or_missing")
		check()
	})
}

func TestQualitySetAsideHardLinkIdentity(t *testing.T) {
	dir := t.TempDir()
	old := programs.FileName("window") + ".old-20261113010000000"
	p := qualityCopy(t, dir, old, true)
	qualityCopy(t, dir, programs.FileName("window")+".old-20261114010000000", false)
	alias := filepath.Join(t.TempDir(), "different-name.exe")
	if err := os.Link(p, alias); err != nil {
		t.Fatal(err)
	}
	check := qualityUnchanged(t, p)
	pid := qualityStart(t, alias)
	s := programs.NewSetAsideService(dir, []string{"window"})
	item := qualityItem(t, s, old)
	if item.CanRemove || !slices.Contains(item.RunningPids, pid) {
		t.Fatalf("hard-linked running image not identified: %+v", item)
	}
	qualityRemove(t, s, item, "refused", "in_use")
	check()
}
