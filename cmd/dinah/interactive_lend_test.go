//go:build tui

package main

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"dinah/internal/contract"
	"dinah/internal/screen/keyboard"
	"dinah/internal/testenv"
)

// editorAppends points the editor at this test binary standing in as an
// editor that records its launch and appends a line to the file it was
// handed, and answers the log.
func editorAppends(t *testing.T) string {
	t.Helper()
	log := filepath.Join(t.TempDir(), "editor.log")
	t.Setenv("DINAH_EDITOR", os.Args[0])
	t.Setenv(editorRecordVar, log)
	t.Setenv(testenv.EditorAppendVar, "appended by the editor")
	return log
}

// lendScript is one run that lends the terminal: the frames each cycle drew,
// counted from where each cycle started, and when each cycle started.
type lendScript struct {
	mu      sync.Mutex
	frames  []string
	starts  []int
	started []time.Time
}

// seamed installs the recorder on a seam.
func (l *lendScript) seamed(seam *interactiveSeams, cycles chan *tea.Program) {
	seam.frame = func(content string) {
		l.mu.Lock()
		l.frames = append(l.frames, content)
		l.mu.Unlock()
	}
	seam.program = func(program *tea.Program) {
		l.mu.Lock()
		l.starts = append(l.starts, len(l.frames))
		l.started = append(l.started, time.Now())
		l.mu.Unlock()
		cycles <- program
	}
}

// firstFrameOf answers the first frame a cycle drew.
func (l *lendScript) firstFrameOf(t *testing.T, cycle int) string {
	t.Helper()
	l.mu.Lock()
	defer l.mu.Unlock()
	if cycle >= len(l.starts) || l.starts[cycle] >= len(l.frames) {
		t.Fatalf("cycle %d drew no frame", cycle+1)
	}
	return l.frames[l.starts[cycle]]
}

// drawnSize is the number of columns of a frame's widest row and its rows.
func drawnSize(frame string) (int, int) {
	rows := strings.Split(frame, "\n")
	widest := 0
	for _, row := range rows {
		widest = max(widest, displayWidth(visible(row)))
	}
	return widest, len(rows)
}

// isEnterAtTheLine accepts Enter pressed while the command line is open.
func isEnterAtTheLine(m *interactiveModel, msg tea.Msg) bool {
	pressed, ok := msg.(keyMsg)
	return ok && pressed.key.String() == "enter" && m.mode == modePrompt && m.prompt == promptJump
}

// TestEditLendsTheTerminalAndTakesItBack is the seam half of
// dinah-623/criteria/11, with /48. edit fx-1 typed at the command line ends
// the program, runs the editor, a stand-in that appends a line, and starts a
// new program over the same model: the card's file carries the line
// afterwards. A key and a resize delivered between Enter and the editor's
// start do not stop the lend completing within ten seconds, the key is not
// acted on, and the first frame after the lend is drawn at the size the
// resize set. A key pressed once the new program has confirmed its flush is
// acted on, and after q the output ends with the bracketed-paste disable
// after the last enable.
func TestEditLendsTheTerminalAndTakesItBack(t *testing.T) {
	root := tuiBench(t)
	log := editorAppends(t)
	anchor := anchorPath(t, root, "fx-1")
	var output lockedBuffer
	s, seam := newScript(t, 100, 30, true)
	seam.output = &output
	cycles := make(chan *tea.Program, 8)
	var lend lendScript
	lend.seamed(seam, cycles)
	var entered time.Time
	var once sync.Once
	seam.observe = func(m *interactiveModel, msg tea.Msg) {
		if !isEnterAtTheLine(m, msg) {
			return
		}
		once.Do(func() {
			entered = time.Now()
			seam.resize(80, 24)
			go s.write("t")
			go m.program.Send(resizeMsg{})
		})
	}
	run := s.run(root, seam, func() {
		waitForProgram(t, cycles)
		s.write(":edit fx-1" + keyEnter)
		waitForProgram(t, cycles)
		s.waitFor("the flush after the lend", isFlushed)
		if journaled(t, root, "fx-1", contract.EventClaimed) {
			t.Error("the key typed while the terminal was lent was acted on")
		}
		s.write("t")
		s.waitFor("t after the lend", isKey("t"))
		s.write("q")
	})
	if run.model == nil {
		t.Fatalf("the run never finished: %q", run.errw)
	}
	if took := lend.started[1].Sub(entered); took > 10*time.Second {
		t.Errorf("the lend took %s", took)
	}
	data, err := os.ReadFile(anchor)
	if err != nil || !strings.Contains(string(data), "appended by the editor") {
		t.Errorf("the card's file does not carry the editor's line: %v\n%s", err, data)
	}
	if launches := editorLaunches(t, log); len(launches) != 1 {
		t.Errorf("the editor was launched %d times", len(launches))
	}
	if width, height := drawnSize(lend.firstFrameOf(t, 1)); width > 79 || height != 24 {
		t.Errorf("the first frame after the lend is %dx%d, wanted the 80x24 the resize set", width, height)
	}
	if !journaled(t, root, "fx-1", contract.EventClaimed) {
		t.Error("the key pressed after the lend was not acted on")
	}
	out := output.String()
	enable, disable := strings.LastIndex(out, keyboard.BracketedPasteOn), strings.LastIndex(out, keyboard.BracketedPasteOff)
	if enable < 0 || disable < enable {
		t.Errorf("the output does not end with the bracketed-paste disable after the last enable: enable at %d, disable at %d", enable, disable)
	}
	if strings.Count(out, keyboard.BracketedPasteOn) != 2 || strings.Count(out, keyboard.BracketedPasteOff) != 2 {
		t.Errorf("each cycle should enable and disable once: %d enables, %d disables", strings.Count(out, keyboard.BracketedPasteOn), strings.Count(out, keyboard.BracketedPasteOff))
	}
}

// TestAResizeDuringTheLendIsHonoured is dinah-623/criteria/48: each cycle
// measures the window before its first frame, so a head lent at 100x30 and
// returned at 80x24 draws its first frame at 80x24, and one returned at
// 50x10 draws the too-small notice.
func TestAResizeDuringTheLendIsHonoured(t *testing.T) {
	for _, size := range [][2]int{{80, 24}, {50, 10}} {
		root := tuiBench(t)
		editorAppends(t)
		s, seam := newScript(t, 100, 30, true)
		cycles := make(chan *tea.Program, 8)
		var lend lendScript
		lend.seamed(seam, cycles)
		var once sync.Once
		seam.observe = func(m *interactiveModel, msg tea.Msg) {
			if isEnterAtTheLine(m, msg) {
				once.Do(func() { seam.resize(size[0], size[1]) })
			}
		}
		run := s.run(root, seam, func() {
			waitForProgram(t, cycles)
			s.write(":edit fx-1" + keyEnter)
			waitForProgram(t, cycles)
			s.waitFor("the flush after the lend", isFlushed)
			s.write(keyCtrlC)
		})
		if run.model == nil {
			t.Fatalf("the run never finished: %q", run.errw)
		}
		first := lend.firstFrameOf(t, 1)
		if size[1] < interactiveMinimumHeight {
			notice := run.model.s.r.T("interactive.too-small", "size", "60x12")
			if !strings.Contains(first, notice) {
				t.Errorf("the first frame at %dx%d is not the too-small notice:\n%s", size[0], size[1], first)
			}
			continue
		}
		if width, height := drawnSize(first); width > size[0]-1 || height != size[1] {
			t.Errorf("the first frame after the lend is %dx%d, wanted %dx%d", width, height, size[0], size[1])
		}
	}
}
