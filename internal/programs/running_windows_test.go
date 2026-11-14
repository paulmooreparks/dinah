//go:build windows

package programs

import (
	"bufio"
	"bytes"
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

func TestPlaceRetainsAnActuallyRunningProgram(t *testing.T) {
	if os.Getenv("PROGRAMS_RETENTION_HELPER") == "1" {
		fmt.Fprintln(os.Stdout, "ready")
		io.Copy(io.Discard, os.Stdin)
		os.Exit(0)
	}
	dir := t.TempDir()
	dst, src := filepath.Join(dir, "window.exe"), filepath.Join(dir, "next.exe")
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	image, err := os.ReadFile(self)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, image, 0o755); err != nil {
		t.Fatal(err)
	}
	child := exec.Command(dst, "-test.run=^TestPlaceRetainsAnActuallyRunningProgram$")
	child.Env = append(os.Environ(), "PROGRAMS_RETENTION_HELPER=1")
	// This console helper has no window and receives input only from its pipe.
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
	exited := make(chan error, 1)
	go func() { exited <- child.Wait() }()
	waited := false
	t.Cleanup(func() {
		input.Close()
		if waited {
			return
		}
		select {
		case <-exited:
		case <-time.After(10 * time.Second):
			child.Process.Kill()
			<-exited
		}
	})
	ready := make(chan string, 1)
	go func() { line, _ := bufio.NewReader(output).ReadString('\n'); ready <- line }()
	select {
	case line := <-ready:
		if line != "ready\n" {
			t.Fatalf("helper: %q", line)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("helper did not become ready")
	}
	writeProgram(t, src, "replacement one")
	if err := Place(src, dst); err != nil {
		t.Fatal(err)
	}
	asides, err := filepath.Glob(dst + ".old-*")
	if err != nil || len(asides) != 1 {
		t.Fatalf("first set-aside file: %v %v", asides, err)
	}
	runningFile := asides[0]
	writeProgram(t, src, "replacement two")
	if err := Place(src, dst); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(runningFile); err != nil || !bytes.Equal(got, image) {
		t.Fatalf("running file changed: %v", err)
	}
	input.Close()
	select {
	case err := <-exited:
		waited = true
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("helper did not exit after its pipe closed")
	}
	writeProgram(t, src, "replacement three")
	if err := Place(src, dst); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(runningFile); err != nil || !bytes.Equal(got, image) {
		t.Fatalf("retired running file removed: %v", err)
	}
}
