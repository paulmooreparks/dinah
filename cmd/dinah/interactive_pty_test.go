//go:build tui && linux

package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"

	"dinah/internal/screen/keyboard"
)

// TestAFrameReachesThePseudoTerminalWithoutStaircasing is
// dinah-603/criteria/34. A frame drawn to a buffer cannot show whether the
// terminal adds the carriage return, so this builds dinah and dinah-tui into
// one directory and runs dinah tui board, which starts dinah-tui, on a real
// pseudo-terminal of 100 by 30, with an environment built
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
	binary := buildProgram(t, gobin, dir, "dinah", "")
	buildProgram(t, gobin, dir, "dinah-tui", "tui")
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

// TestALendRestoresThePseudoTerminal is the pseudo-terminal half of
// dinah-623/criteria/11. It runs dinah tui board on a real pseudo-terminal of
// 100 by 30 with the editor a shell script that appends a line to the file
// it is handed, types edit fx-1 at the command line, and, straight after
// Enter, types m and resizes the terminal to 80 by 24. The lend completes
// within ten seconds, the card's file carries the editor's line, and the
// first board after the lend is drawn at 80 columns. m typed during the
// switch is not acted on, since it would have opened the move menu and left
// the t typed after the lend without effect, and t claims fx-1. Before that
// t, the last bracketed-paste sequence written is the enable, and paste is
// never switched off while a frame is up (dinah-623/criteria/52). After q the
// output ends with the bracketed-paste disable after the last enable, and
// the terminal's modes are the ones it had before dinah tui started.
func TestALendRestoresThePseudoTerminal(t *testing.T) {
	gobin, err := exec.LookPath("go")
	if err != nil {
		t.Skip("go is not on PATH, so the binary cannot be built for the pseudo-terminal")
	}
	root := tuiBench(t)
	// A thousand cards waiting in intake, which the board does not draw,
	// make the second program's Init, which opens the workbench and reads
	// the view again, outlast the first tick of the renderer Bubble Tea
	// starts before Init. That is the position where the renderer draws its
	// own zero View ahead of the head's first frame (dinah-623/criteria/52).
	for i := 0; i < pastePositionCards; i++ {
		step(t, root, "add", "Waiting "+strconv.Itoa(i))
	}
	workbench := soleBenchDir(t, root)
	anchor := anchorPath(t, root, "fx-1")
	dir := t.TempDir()
	binary := buildProgram(t, gobin, dir, "dinah", "")
	buildProgram(t, gobin, dir, "dinah-tui", "tui")
	editor := filepath.Join(dir, "editor.sh")
	if err := os.WriteFile(editor, []byte("#!/bin/sh\necho appended by the editor >> \"$1\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	fixture, err := filepath.Abs(filepath.Join("..", "..", "internal", "screen", "testdata", "terminfo"))
	if err != nil {
		t.Fatal(err)
	}

	control, subordinate := openPseudoTerminal(t)
	defer control.Close()
	watcher, err := os.OpenFile(subordinate.Name(), os.O_RDWR|unix.O_NOCTTY, 0)
	if err != nil {
		t.Fatalf("open a second handle on the subordinate side: %v", err)
	}
	defer watcher.Close()
	if err := unix.IoctlSetWinsize(int(watcher.Fd()), unix.TIOCSWINSZ, &unix.Winsize{Row: 30, Col: 100}); err != nil {
		t.Fatalf("TIOCSWINSZ: %v", err)
	}
	before, err := unix.IoctlGetTermios(int(watcher.Fd()), unix.TCGETS)
	if err != nil {
		t.Fatalf("TCGETS: %v", err)
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
		"DINAH_EDITOR=" + editor,
		"PATH=" + os.Getenv("PATH"),
	}
	if err := child.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	subordinate.Close()

	var mu sync.Mutex
	var seen bytes.Buffer
	read := make(chan struct{}, 1024)
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := control.Read(buf)
			if n > 0 {
				mu.Lock()
				seen.Write(buf[:n])
				mu.Unlock()
				read <- struct{}{}
			}
			if err != nil {
				close(read)
				return
			}
		}
	}()
	written := func() string {
		mu.Lock()
		defer mu.Unlock()
		return seen.String()
	}
	waitUntil := func(what string, holds func(string) bool) {
		t.Helper()
		deadline := time.After(10 * time.Second)
		for !holds(written()) {
			select {
			case _, open := <-read:
				if !open {
					t.Fatalf("the terminal closed while waiting for %s, having read %q", what, written())
				}
			case <-deadline:
				child.Process.Kill()
				t.Fatalf("waited 10s for %s, having read %q", what, written())
			}
		}
	}
	entersScreen := func(n int) func(string) bool {
		return func(out string) bool { return strings.Count(out, altScreenOn) >= n }
	}
	waitUntil("the first board", func(out string) bool { return strings.Count(out, "\n") >= 2 })
	if _, err := control.Write([]byte(":edit fx-1\r")); err != nil {
		t.Fatalf("write the line: %v", err)
	}
	if _, err := control.Write([]byte("m")); err != nil {
		t.Fatalf("write m: %v", err)
	}
	if err := unix.IoctlSetWinsize(int(watcher.Fd()), unix.TIOCSWINSZ, &unix.Winsize{Row: 24, Col: 80}); err != nil {
		t.Fatalf("TIOCSWINSZ: %v", err)
	}
	lent := time.Now()
	waitUntil("the board after the lend", entersScreen(2))
	if took := time.Since(lent); took > 10*time.Second {
		t.Errorf("the lend took %s", took)
	}
	mark := strings.LastIndex(written(), altScreenOn)
	waitUntil("the board drawn after the lend", func(out string) bool { return strings.Contains(out[mark:], "Build the parser") })
	// dinah-623/criteria/52: the second program has drawn its board, so
	// every mode its first frames set has been written. The last paste
	// sequence before the first key typed at it must be the enable, or a
	// paste into it is no longer marked.
	firstKey := written()
	if enable, disable := strings.LastIndex(firstKey, keyboard.BracketedPasteOn), strings.LastIndex(firstKey, keyboard.BracketedPasteOff); enable < 0 || disable > enable {
		t.Errorf("the last paste sequence before the first key of the program after the lend is the disable: enable at %d, disable at %d: %q", enable, disable, firstKey[mark:min(len(firstKey), mark+60)])
	}
	claimed := func() bool {
		for _, event := range cardEvents(t, root, "fx-1") {
			if event.Event == "claimed" {
				return true
			}
		}
		return false
	}
	// The new program discards what was typed before its flush is confirmed,
	// which no byte on the terminal announces, so t is typed again until it
	// lands. A t typed after the claim does nothing, since the claim is no
	// longer offered.
	for attempt := 0; attempt < 5 && !claimed(); attempt++ {
		if _, err := control.Write([]byte("t")); err != nil {
			t.Fatalf("write t: %v", err)
		}
		deadline := time.Now().Add(2 * time.Second)
		for !claimed() && time.Now().Before(deadline) {
			time.Sleep(50 * time.Millisecond)
		}
	}
	if !claimed() {
		t.Errorf("t pressed after the lend claimed nothing, so the m typed during it was acted on or t was lost")
	}
	if _, err := control.Write([]byte("q")); err != nil {
		t.Fatalf("write q: %v", err)
	}
	exited := make(chan error, 1)
	go func() { exited <- child.Wait() }()
	select {
	case err := <-exited:
		if err != nil {
			t.Fatalf("the child exited with %v, having written %q", err, written())
		}
	case <-time.After(30 * time.Second):
		child.Process.Kill()
		t.Fatalf("the child did not exit within 30s of q, having written %q", written())
	}
	out := written()
	enable, disable := strings.LastIndex(out, keyboard.BracketedPasteOn), strings.LastIndex(out, keyboard.BracketedPasteOff)
	if enable < 0 || disable < enable {
		t.Errorf("the output does not end with the bracketed-paste disable after the last enable: enable at %d, disable at %d", enable, disable)
	}
	if why := pasteOffOnTheAlternateScreen(out); why != "" {
		t.Error(why)
	}
	after := out[mark:]
	if widest := longestRun(after, "─"); widest > 79 {
		t.Errorf("the board after the lend draws a rule %d columns wide, and the terminal was 80", widest)
	}
	data, err := os.ReadFile(anchor)
	if err != nil || !strings.Contains(string(data), "appended by the editor") {
		t.Errorf("the card's file does not carry the editor's line: %v", err)
	}
	restored, err := unix.IoctlGetTermios(int(watcher.Fd()), unix.TCGETS)
	if err != nil {
		t.Fatalf("TCGETS after: %v", err)
	}
	if restored.Iflag != before.Iflag || restored.Oflag != before.Oflag || restored.Lflag != before.Lflag || restored.Cflag != before.Cflag {
		t.Errorf("the terminal's modes after q are %+v, and before dinah tui started they were %+v", restored, before)
	}
}

// pastePositionCards is how many cards the pseudo-terminal lend test adds to
// intake so that the Init after the lend outlasts the renderer's first tick.
const pastePositionCards = 1000

// altScreenOn is the sequence that enters the alternate screen, which each
// program writes once as its frames begin.
const altScreenOn = "\x1b[?1049h"

// longestRun is the most consecutive copies of a glyph in a text.
func longestRun(text, glyph string) int {
	longest, run := 0, 0
	for len(text) > 0 {
		if strings.HasPrefix(text, glyph) {
			run++
			longest = max(longest, run)
			text = text[len(glyph):]
			continue
		}
		run = 0
		text = text[1:]
	}
	return longest
}
