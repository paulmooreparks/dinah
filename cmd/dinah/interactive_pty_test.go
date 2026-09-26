//go:build tui && linux

package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// TestAFrameReachesThePseudoTerminalWithoutStaircasing is
// dinah-603/criteria/34. A frame drawn to a buffer cannot show whether the
// terminal adds the carriage return, so this runs the built binary as dinah
// tui board on a real pseudo-terminal of 100 by 30, with an environment built
// from nothing, reads the controlling side until the second frame row has
// arrived, writes q, and requires the child to exit 0 with every line feed it
// wrote preceded by a carriage return. It is also the one automated run of
// the real keyboard entry, key reader and exit path on POSIX.
func TestAFrameReachesThePseudoTerminalWithoutStaircasing(t *testing.T) {
	gobin, err := exec.LookPath("go")
	if err != nil {
		t.Skip("go is not on PATH, so the binary cannot be built for the pseudo-terminal")
	}
	root := tuiBench(t)
	workbench := soleBenchDir(t, root)
	dir := t.TempDir()
	binary := filepath.Join(dir, "dinah")
	build := exec.Command(gobin, "build", "-o", binary, ".")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	fixture, err := filepath.Abs(filepath.Join("..", "..", "internal", "screen", "testdata", "terminfo"))
	if err != nil {
		t.Fatal(err)
	}

	control, subordinate := openPseudoTerminal(t)
	defer control.Close()
	size := &unix.Winsize{Row: 30, Col: 100}
	if err := unix.IoctlSetWinsize(int(subordinate.Fd()), unix.TIOCSWINSZ, size); err != nil {
		t.Fatalf("TIOCSWINSZ: %v", err)
	}
	child := exec.Command(binary, "tui", "board")
	child.Stdin, child.Stdout, child.Stderr = subordinate, subordinate, subordinate
	child.SysProcAttr = &syscall.SysProcAttr{Setsid: true, Setctty: true, Ctty: 0}
	child.Env = []string{
		"TERM=xterm",
		"TERMINFO=" + fixture,
		"DINAH_HOME=" + filepath.Join(dir, "home"),
		"DINAH_WORKBENCH=" + workbench,
		"DINAH_ACTOR=alka",
		"PATH=" + os.Getenv("PATH"),
	}
	if err := child.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	subordinate.Close()

	read := make(chan []byte, 64)
	go func() {
		defer close(read)
		buf := make([]byte, 4096)
		for {
			n, err := control.Read(buf)
			if n > 0 {
				read <- append([]byte(nil), buf[:n]...)
			}
			if err != nil {
				return
			}
		}
	}()
	var seen bytes.Buffer
	deadline := time.After(30 * time.Second)
	sentQuit := false
	for !sentQuit {
		select {
		case chunk, open := <-read:
			if !open {
				t.Fatalf("the terminal closed before two rows arrived, having read %q", seen.String())
			}
			seen.Write(chunk)
			if bytes.Count(seen.Bytes(), []byte("\n")) >= 2 {
				if _, err := control.Write([]byte("q")); err != nil {
					t.Fatalf("write q: %v", err)
				}
				sentQuit = true
			}
		case <-deadline:
			child.Process.Kill()
			t.Fatalf("no second frame row within 30s, having read %q", seen.String())
		}
	}
	exited := make(chan error, 1)
	go func() { exited <- child.Wait() }()
	select {
	case err := <-exited:
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			t.Fatalf("the child exited %d after q, having written %q", exit.ExitCode(), seen.String())
		}
		if err != nil {
			t.Fatalf("wait: %v", err)
		}
	case <-time.After(30 * time.Second):
		child.Process.Kill()
		t.Fatalf("the child did not exit within 30s of q, having written %q", seen.String())
	}
	for chunk := range read {
		seen.Write(chunk)
	}
	written := seen.Bytes()
	if len(written) == 0 {
		t.Fatal("read zero bytes from the pseudo-terminal, so this proves nothing")
	}
	feeds := bytes.Count(written, []byte("\n"))
	if feeds < 2 {
		t.Fatalf("read %d line feeds, wanted at least two: %q", feeds, written)
	}
	for at, b := range written {
		if b == '\n' && (at == 0 || written[at-1] != '\r') {
			t.Fatalf("the line feed at byte %d of %d is bare, so the frame staircases: %q", at, len(written), written[max(0, at-40):at+1])
		}
	}
	t.Logf("read %d bytes carrying %d line feeds, each after a carriage return", len(written), feeds)
}

// openPseudoTerminal opens a pseudo-terminal pair through /dev/ptmx, unlocking
// the subordinate side with TIOCSPTLCK and naming it with TIOCGPTN.
func openPseudoTerminal(t *testing.T) (control, subordinate *os.File) {
	t.Helper()
	fd, err := unix.Open("/dev/ptmx", unix.O_RDWR|unix.O_NOCTTY|unix.O_CLOEXEC, 0)
	if err != nil {
		t.Fatalf("open /dev/ptmx: %v", err)
	}
	control = os.NewFile(uintptr(fd), "/dev/ptmx")
	if err := unix.IoctlSetPointerInt(fd, unix.TIOCSPTLCK, 0); err != nil {
		t.Fatalf("TIOCSPTLCK: %v", err)
	}
	number, err := unix.IoctlGetInt(fd, unix.TIOCGPTN)
	if err != nil {
		t.Fatalf("TIOCGPTN: %v", err)
	}
	subordinate, err = os.OpenFile("/dev/pts/"+strconv.Itoa(number), os.O_RDWR|unix.O_NOCTTY, 0)
	if err != nil {
		t.Fatalf("open the subordinate side: %v", err)
	}
	return control, subordinate
}
