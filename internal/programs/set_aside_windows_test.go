//go:build windows

package programs

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

func TestSetAsideRunningImageAndLateStart(t *testing.T) {
	if os.Getenv("SET_ASIDE_HELPER") == "1" {
		fmt.Fprintln(os.Stdout, "ready")
		io.Copy(io.Discard, os.Stdin)
		os.Exit(0)
	}
	dir := t.TempDir()
	service := NewSetAsideService(dir, []string{"window"})
	path := filepath.Join(dir, FileName("window")+".old-20261113120000000")
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	image, err := os.ReadFile(self)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, image, 0o755); err != nil {
		t.Fatal(err)
	}
	writeProgram(t, filepath.Join(dir, FileName("window")+".old-20261114120000000"), "fallback")
	before, err := service.List()
	if err != nil {
		t.Fatal(err)
	}
	var selection string
	for _, item := range before.Items {
		if item.Name == filepath.Base(path) {
			if !item.CanRemove {
				t.Fatalf("helper not yet running: %+v; %v", item, before.Warnings)
			}
			selection = item.ID
		}
	}
	child := exec.Command(path, "-test.run=^TestSetAsideRunningImageAndLateStart$")
	child.Env = append(os.Environ(), "SET_ASIDE_HELPER=1")
	child.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: windows.CREATE_NO_WINDOW}
	input, err := child.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	output, err := child.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	exit := make(chan error, 1)
	go func() { exit <- child.Wait() }()
	waited := false
	t.Cleanup(func() {
		input.Close()
		if !waited {
			select {
			case <-exit:
			case <-time.After(10 * time.Second):
				child.Process.Kill()
				<-exit
			}
		}
	})
	ready := make(chan string, 1)
	go func() { line, _ := bufio.NewReader(output).ReadString('\n'); ready <- line }()
	select {
	case line := <-ready:
		if line != "ready\n" {
			t.Fatalf("child: %q", line)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("helper did not become ready")
	}
	listed, err := service.List()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, item := range listed.Items {
		if item.ID == selection {
			found = true
			if item.CanRemove || item.ReasonCode != "in_use" || len(item.RunningPids) != 1 || item.RunningPids[0] != uint32(child.Process.Pid) {
				t.Fatalf("running image not identified: %+v; warnings %v", item, listed.Warnings)
			}
		}
	}
	if !found {
		t.Fatal("running image selection changed")
	}
	result, err := service.Remove(RemoveSetAsideInput{IDs: []string{selection}})
	if err != nil || len(result.Items) != 1 || result.Items[0].ReasonCode != "in_use" {
		t.Fatalf("late-start removal: %+v %v", result, err)
	}
	// Rename after startup, as an installer does. PSAPI must still identify
	// the loaded image rather than following a replacement at its old name.
	renamed := filepath.Join(dir, FileName("window")+".before-renamed-20261112-120000")
	if err := os.Rename(path, renamed); err != nil {
		t.Fatal(err)
	}
	writeProgram(t, path, "different image at the old name")
	listed, err = service.List()
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range listed.Items {
		if item.Name == filepath.Base(renamed) && (item.CanRemove || len(item.RunningPids) == 0) {
			t.Fatalf("renamed running image not identified: %+v; warnings %v", item, listed.Warnings)
		}
		if item.Name == filepath.Base(path) && len(item.RunningPids) != 0 {
			t.Fatalf("replacement mistaken for running image: %+v", item)
		}
	}
	input.Close()
	select {
	case err := <-exit:
		waited = true
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("helper did not exit")
	}
}

func TestSetAsideRemovesOnlyChosenEligibleCopies(t *testing.T) {
	dir := t.TempDir()
	for _, suffix := range []string{".old-20261112120000000", ".old-20261113120000000", ".old-20261114120000000"} {
		writeProgram(t, filepath.Join(dir, FileName("window")+suffix), suffix)
	}
	service := NewSetAsideService(dir, []string{"window"})
	listed, err := service.List()
	if err != nil {
		t.Fatal(err)
	}
	var fallback, chosen SetAsideProgram
	for _, item := range listed.Items {
		if item.Fallback {
			fallback = item
		} else {
			chosen = item
		}
	}
	result, err := service.Remove(RemoveSetAsideInput{IDs: []string{chosen.ID, fallback.ID}})
	if err != nil || result.RemovedBytes != chosen.SizeBytes || result.Items[0].State != "removed" || result.Items[1].ReasonCode != "fallback" {
		t.Fatalf("remove result: %+v %v", result, err)
	}
	remaining, err := service.List()
	if err != nil || len(remaining.Items) != 2 {
		t.Fatalf("remaining files: %+v %v", remaining, err)
	}
}

func TestSetAsideKernelRefusesAStartAfterTheLastSnapshot(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, FileName("window")+".old-20261113120000000")
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	image, err := os.ReadFile(self)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, image, 0o755); err != nil {
		t.Fatal(err)
	}
	writeProgram(t, filepath.Join(dir, FileName("window")+".old-20261114120000000"), "fallback")
	files, err := NewSetAsideService(dir, []string{"window"}).scan()
	if err != nil {
		t.Fatal(err)
	}
	var chosen asideFile
	for _, file := range files {
		if file.CanRemove {
			chosen = file
		}
	}
	code, err := removeAside(chosen, func() error {
		// This callback runs after DELETE access and identity verification.
		// Starting here exercises the kernel guard rather than a rescan.
		child := exec.Command(path, "-test.run=^TestSetAsideRunningImageAndLateStart$")
		child.Env = append(os.Environ(), "SET_ASIDE_HELPER=1")
		child.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: windows.CREATE_NO_WINDOW}
		input, err := child.StdinPipe()
		if err != nil {
			return err
		}
		output, err := child.StdoutPipe()
		if err != nil {
			input.Close()
			return err
		}
		if err := child.Start(); err != nil {
			input.Close()
			return err
		}
		exit := make(chan error, 1)
		go func() { exit <- child.Wait() }()
		t.Cleanup(func() {
			input.Close()
			select {
			case err := <-exit:
				if err != nil {
					t.Error(err)
				}
			case <-time.After(10 * time.Second):
				child.Process.Kill()
				<-exit
				t.Error("helper did not exit after its pipe closed")
			}
		})
		ready := make(chan string, 1)
		go func() { line, _ := bufio.NewReader(output).ReadString('\n'); ready <- line }()
		select {
		case line := <-ready:
			if line != "ready\n" {
				return fmt.Errorf("helper: %q", line)
			}
		case <-time.After(10 * time.Second):
			return fmt.Errorf("helper did not become ready")
		}
		return nil
	})
	if err == nil || code != "in_use" {
		t.Fatalf("kernel did not refuse the mapped image: %s %v", code, err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("running executable removed: %v", err)
	}
}
