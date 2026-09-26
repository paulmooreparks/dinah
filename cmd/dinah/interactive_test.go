//go:build tui

package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"dinah/internal/screen"
)

// tuiDefinition is the flow the terminal head's tests stand in: an intake, a
// work column rejecting back to itself's predecessor, an operator-owned
// acceptance column that rejects to the work column, and a done column.
const tuiDefinition = `{
  "profile": "dinah-core/0.12",
  "title": "Terminal",
  "columns": [
    { "id": "t10000000001", "title": "Intake", "kind": "intake" },
    { "id": "t10000000002", "title": "Implement", "kind": "work" },
    { "id": "t10000000003", "title": "Acceptance", "kind": "work", "operator_owned": true, "reject_to": "implement" },
    { "id": "t10000000004", "title": "Done", "kind": "done" }
  ]
}`

// The key strings the compiled xterm fixture declares, which the head's
// decoder reads under the seam. TestTheXtermFixtureDeclaresTheKeysTheTestsSend
// in internal/screen holds them to the entry.
const (
	xtermUp       = "\x1bOA"
	xtermDown     = "\x1bOB"
	xtermRight    = "\x1bOC"
	xtermLeft     = "\x1bOD"
	xtermHome     = "\x1bOH"
	xtermEnd      = "\x1bOF"
	xtermPageUp   = "\x1b[5~"
	xtermPageDown = "\x1b[6~"
	xtermDelete   = "\x1b[3~"
	keyEnter      = "\r"
	keyCtrlC      = "\x03"
	keyCtrlD      = "\x04"
	keyCtrlG      = "\x07"
	keyBackspace  = "\x7f"
	pasteOpen     = "\x1b[200~"
	pasteClose    = "\x1b[201~"
)

// xtermEntry reads the compiled xterm entry internal/screen's own tests read.
func xtermEntry(t *testing.T) *screen.Terminfo {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "internal", "screen", "testdata", "terminfo", "78", "xterm"))
	if err != nil {
		t.Fatalf("read the xterm fixture: %v", err)
	}
	entry, err := screen.ParseTerminfo(data)
	if err != nil {
		t.Fatalf("parse the xterm fixture: %v", err)
	}
	return entry
}

// tuiBench builds the terminal flow with cards standing where a test needs
// them: two in Implement and three in Acceptance, in that filing order.
func tuiBench(t *testing.T) string {
	t.Helper()
	root := newBenchFromDefinition(t, tuiDefinition)
	steps := [][]string{
		{"add", "Build the parser"}, {"move", "fx-1", "implement"},
		{"add", "Write the guide"}, {"move", "fx-2", "implement"},
		{"add", "First to accept"}, {"move", "fx-3", "acceptance"},
		{"add", "Second to accept"}, {"move", "fx-4", "acceptance"},
		{"add", "Third to accept"}, {"move", "fx-5", "acceptance"},
	}
	for _, step := range steps {
		if got := runCLI(t, root, step...); got.code != 0 {
			t.Fatalf("%v: %d %s", step, got.code, got.errw)
		}
	}
	return root
}

// tuiTerm is the TERM a run through the seam is given, which the
// dumb-terminal check reads off Windows. A test naming another sets it back.
var tuiTerm = "xterm"

// tuiWatchdog is how long a run through the seam may last before the test
// kills the program, so a script that never quits fails rather than hangs.
const tuiWatchdog = 30 * time.Second

// tuiRun is one run of dinah tui through the seam: the invocation, every byte
// the program wrote to the terminal, the final model, Run's error, and how
// many flushes the reader made.
type tuiRun struct {
	invocation
	output   string
	model    *interactiveModel
	runErr   error
	flushes  int
	finished bool
}

// markedPaste is an empty paste between both markers. A script that opens
// with it runs as on a terminal that honours the bracketed-paste request, so
// Enter in the jump and filter prompts asks for no flush and the keys typed
// after it are read. It is discarded in browse mode like any paste.
const markedPaste = pasteOpen + pasteClose

// tuiSeam is a seam with both streams terminals, a window of width by
// height, the xterm entry, and the keys to feed the decoder, which open with
// markedPaste.
func tuiSeam(t *testing.T, keys io.Reader, width, height int) *interactiveSeams {
	t.Helper()
	seam := unmarkedSeam(t, keys, width, height)
	seam.keys = io.MultiReader(strings.NewReader(markedPaste), keys)
	return seam
}

// unmarkedSeam is tuiSeam on a terminal that has sent no paste marker, which
// is where the flush fallback of section 6.4 applies.
func unmarkedSeam(t *testing.T, keys io.Reader, width, height int) *interactiveSeams {
	t.Helper()
	return &interactiveSeams{
		stdinTerminal:  true,
		stdoutTerminal: true,
		width:          width,
		height:         height,
		keys:           keys,
		terminfo:       xtermEntry(t),
	}
}

// runTUIThrough runs dinah tui with a seam, through runCLI, and answers what
// it wrote and left. TERM names an entry off Windows, which the dumb-terminal
// check reads, and the seam is removed when the run ends.
func runTUIThrough(t *testing.T, root string, seam *interactiveSeams, argv ...string) tuiRun {
	t.Helper()
	t.Setenv("TERM", tuiTerm)
	var output lockedBuffer
	var run tuiRun
	var mu sync.Mutex
	// A test that brings its own output, such as a fake console, reads what
	// was written from it, and run.output stays empty.
	if seam.output == nil {
		seam.output = &output
	}
	userDiscard := seam.discard
	seam.discard = func() {
		mu.Lock()
		run.flushes++
		mu.Unlock()
		if userDiscard != nil {
			userDiscard()
		}
	}
	userFinish := seam.finish
	seam.finish = func(model *interactiveModel, err error) {
		run.model, run.runErr, run.finished = model, err, true
		if userFinish != nil {
			userFinish(model, err)
		}
	}
	userProgram := seam.program
	done := make(chan struct{})
	seam.program = func(program *tea.Program) {
		go func() {
			select {
			case <-done:
			case <-time.After(tuiWatchdog):
				t.Errorf("the program was still running after %s, so the test killed it", tuiWatchdog)
				program.Kill()
			}
		}()
		if userProgram != nil {
			userProgram(program)
		}
	}
	interactiveSeam = seam
	defer func() {
		close(done)
		interactiveSeam = nil
	}()
	run.invocation = runCLI(t, root, append([]string{"tui"}, argv...)...)
	run.output = output.String()
	return run
}

// lockedBuffer is a buffer written from Bubble Tea's renderer goroutine and
// read by the test, guarded by a mutex.
type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

// Write appends to the buffer.
func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

// String answers everything written so far.
func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// ansiSequence matches one control sequence or OSC string the program wrote.
var ansiSequence = regexp.MustCompile("\x1b\\[[0-9;?]*[ -/]*[@-~]|\x1b\\][^\x07\x1b]*(\x07|\x1b\\\\)|\x1b[@-Z\\\\-_]")

// visible is output with every control sequence removed, which is the text a
// person would read.
func visible(output string) string {
	return ansiSequence.ReplaceAllString(output, "")
}

// TestTheHeadStartsDrawsTheBoardAndQuits is the smallest run of the head: it
// starts on the board, draws the cards of the focused lane, and q ends it
// with exit 0.
func TestTheHeadStartsDrawsTheBoardAndQuits(t *testing.T) {
	root := tuiBench(t)
	run := runTUIThrough(t, root, tuiSeam(t, strings.NewReader("q"), 100, 30))
	if run.code != 0 {
		t.Fatalf("exit %d, stderr %q", run.code, run.errw)
	}
	if run.out != "" {
		t.Errorf("stdout carried %q, and the head draws only on the terminal", run.out)
	}
	if !strings.Contains(visible(run.output), "Build the parser") {
		t.Errorf("the frame did not draw the focused lane's first card:\n%s", visible(run.output))
	}
	if !run.finished || run.model == nil {
		t.Fatal("the program never reached the finish seam")
	}
}
