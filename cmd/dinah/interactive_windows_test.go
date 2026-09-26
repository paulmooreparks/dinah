//go:build tui && windows

package main

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf16"

	tea "charm.land/bubbletea/v2"

	"dinah/internal/consolewriter"
)

// insertFrameModel draws frame A, whose row 3 reads abcdefgh, until it is
// told to switch, and then frame B, whose row 3 reads abXYcdefgh and whose
// other rows are A's.
type insertFrameModel struct {
	switched bool
}

// switchFrameMsg switches insertFrameModel from frame A to frame B.
type switchFrameMsg struct{}

// Init starts nothing.
func (m insertFrameModel) Init() tea.Cmd { return nil }

// Update switches the frame, and quits on tea.QuitMsg's command.
func (m insertFrameModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if _, ok := msg.(switchFrameMsg); ok {
		m.switched = true
	}
	return m, nil
}

// View draws the frame the model is on, as the head draws its own: on the
// alternate screen with bracketed paste left to Dinah.
func (m insertFrameModel) View() tea.View {
	row := "abcdefgh"
	if m.switched {
		row = "abXYcdefgh"
	}
	view := tea.NewView(strings.Join([]string{"first row", "second row", row, "fourth row"}, "\n"))
	view.AltScreen = true
	view.DisableBracketedPasteMode = true
	return view
}

// drawInsertPair draws frame A and then frame B through a program built with
// the options given, waiting on the buffer's content between them, and
// answers the bytes written after A was on the screen.
func drawInsertPair(t *testing.T, options []tea.ProgramOption, frames *consoleFrames, buffer *lockedBuffer) string {
	t.Helper()
	program := tea.NewProgram(insertFrameModel{}, append(options, tea.WithoutSignals())...)
	done := make(chan error, 1)
	go func() {
		_, err := program.Run()
		done <- err
	}()
	waitForOutput(t, buffer, "abcdefgh")
	mark := len(buffer.String())
	program.Send(switchFrameMsg{})
	waitForOutput(t, buffer, "XY")
	written := buffer.String()[mark:]
	program.Quit()
	if err := <-done; err != nil {
		t.Fatalf("the program ended with %v", err)
	}
	if frames != nil {
		frames.Flush()
	}
	return written
}

// environWithoutTERM is the process environment with every TERM entry
// removed.
func environWithoutTERM() []string {
	var kept []string
	for _, variable := range os.Environ() {
		name, _, _ := strings.Cut(variable, "=")
		if !strings.EqualFold(name, "TERM") {
			kept = append(kept, variable)
		}
	}
	return kept
}

// TestAnInsertedRunNeverUsesInsertMode is dinah-603/criteria/43. It draws a
// frame pair whose row 3 gains two characters in the middle through a
// program built with exactly the options interactiveProgramOptions answers
// for windows, and finds no insert mode, CR or LF in the bytes written for
// the second frame.
//
// It claims only that this pair, which drives a renderer without
// capabilities into insert mode, is drawn without it under the Windows
// options. It does not claim the ICH branch was taken: under TERM=xterm the
// renderer may draw the pair with putRange. That insert mode is unreachable
// rests on the source, where insertCells writes ESC [4h only when capICH is
// absent and xterm carries it (section 16.3 of the specification).
// TestTheInsertPairReachesInsertModeWithoutTERM shows the pair is one that
// reaches insert mode when the capabilities are taken away.
func TestAnInsertedRunNeverUsesInsertMode(t *testing.T) {
	var buffer lockedBuffer
	options, frames := interactiveProgramOptions("windows", &buffer, os.Environ(), 40, 10)
	written := drawInsertPair(t, options, frames, &buffer)
	for _, forbidden := range []string{"\x1b[4h", "\x1b[4l", "\r", "\n"} {
		if strings.Contains(written, forbidden) {
			t.Errorf("the bytes written for frame B hold %q: %q", forbidden, written)
		}
	}
	t.Logf("frame B was written as %q", written)
}

// TestTheInsertPairReachesInsertModeWithoutTERM is the arming of
// dinah-603/criteria/43 kept as a test: the same pair through the same
// options, with TERM removed from the environment Bubble Tea reads, writes
// ESC [4h, so the pair TestAnInsertedRunNeverUsesInsertMode draws is one that
// reaches insert mode when the renderer has no capabilities.
func TestTheInsertPairReachesInsertModeWithoutTERM(t *testing.T) {
	var buffer lockedBuffer
	options, frames := interactiveProgramOptions("windows", &buffer, os.Environ(), 40, 10)
	options = append(options, tea.WithEnvironment(environWithoutTERM()))
	written := drawInsertPair(t, options, frames, &buffer)
	if !strings.Contains(written, "\x1b[4h") {
		t.Fatalf("without TERM the bytes written for frame B hold no ESC [4h: %q", written)
	}
	t.Logf("frame B was written as %q", written)
}

// shortConsole is a fake console standing where WriteConsoleW does. It
// records every call it is given and what it shows, and once armed it
// answers the first call holding an escape sequence as written only up to
// the middle of that sequence, as lpNumberOfCharsWritten would report a
// short write.
type shortConsole struct {
	mu        sync.Mutex
	armed     bool
	shown     []uint16
	calls     [][]uint16
	shortCall int
	shortAt   int
}

// write is the console's WriteConsoleW.
func (c *shortConsole) write(u []uint16) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls = append(c.calls, append([]uint16(nil), u...))
	n := len(u)
	if c.armed {
		for i := 0; i+2 < len(u); i++ {
			if u[i] == 0x1b && u[i+1] == '[' {
				n, c.armed = i+2, false
				c.shortCall, c.shortAt = len(c.calls)-1, n
				break
			}
		}
	}
	c.shown = append(c.shown, u[:n]...)
	return n, nil
}

// arm makes the next call holding an escape sequence a short one.
func (c *shortConsole) arm() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.armed = true
}

// String answers everything the console shows.
func (c *shortConsole) String() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return string(utf16.Decode(c.shown))
}

// isRepaint accepts the message consoleFrames sends after a short write.
func isRepaint(msg tea.Msg) bool {
	_, ok := msg.(repaintMsg)
	return ok
}

// TestAShortConsoleWriteIsFinishedAndRepainted is dinah-603/criteria/49. The
// head runs with the Windows options over the console writer of
// internal/consolewriter, whose console is a fake that, once the board is
// drawn, answers the frame for j as written only up to the middle of its
// first escape sequence. The writer's next call carries exactly the rest of
// that piece, and the next frame is a full repaint: a clear followed by
// every row of the screen.
func TestAShortConsoleWriteIsFinishedAndRepainted(t *testing.T) {
	root := tuiBench(t)
	s, seam := newScript(t, 100, 30, true)
	console := &shortConsole{shortCall: -1}
	seam.output = consolewriter.NewConsole(console.write)
	mark := -1
	run := s.run(root, seam, func() {
		deadline := time.Now().Add(20 * time.Second)
		for !strings.Contains(visible(console.String()), "fx-1") && time.Now().Before(deadline) {
			time.Sleep(5 * time.Millisecond)
		}
		mark = settledLength(console.String)
		console.arm()
		s.write("j")
		if s.waitFor("the repaint a short write asks for", isRepaint) != nil {
			waitForClear(t, console.String, mark)
		}
		s.write(keyCtrlC)
	})
	if run.model == nil || mark < 0 {
		t.Fatalf("the run never finished: %q", run.errw)
	}
	console.mu.Lock()
	shortCall, shortAt, calls := console.shortCall, console.shortAt, console.calls
	console.mu.Unlock()
	if shortCall < 0 {
		t.Fatal("the fake console never answered a call short")
	}
	given := calls[shortCall]
	t.Logf("call %d was given %d units and answered %d, cutting %q", shortCall, len(given), shortAt, string(utf16.Decode(given[max(0, shortAt-4):min(len(given), shortAt+8)])))
	if shortCall+1 >= len(calls) {
		t.Fatal("no call followed the short one")
	}
	rest := string(utf16.Decode(given[shortAt:]))
	if next := string(utf16.Decode(calls[shortCall+1])); next != rest {
		t.Errorf("the call after the short one carried %q, and the rest of the piece is %q", next, rest)
	}
	rows := strings.Split(run.model.content(), "\n")
	if why := fullRepaint(console.String()[mark:], rows); why != "" {
		t.Errorf("the frame after the short write is not a full repaint: %s", why)
	}
}

// microsoftListed matches the escape sequences of the first table of section
// 16.3 of the specification, each listed on Microsoft's "Console Virtual
// Terminal Sequences" page: the alternate screen, the cursor's visibility,
// Dinah's bracketed-paste switch, cursor position with and without
// parameters, cursor up, down, forward and back, CHA, VPA, reverse index,
// erase in display below and whole, erase in line to the end and to the
// start, ECH, ICH, DCH, and SGR, whose parameters sgrListed checks.
var microsoftListed = regexp.MustCompile(`^(\x1b\[\?(1049|25|2004)[hl]|\x1b\[([0-9]+;[0-9]+)?H|\x1b\[[0-9]*[ABCDGdXP@]|\x1bM|\x1b\[2?J|\x1b\[1?K|\x1b\[[0-9;]*m)$`)

// sgrListed are the SGR parameters section 16.3 lists: reset, bold, normal
// intensity, reverse video and its reset, the foreground colours 1, 3 and 4
// and the default foreground.
var sgrListed = map[string]bool{"": true, "0": true, "1": true, "22": true, "7": true, "27": true, "31": true, "33": true, "34": true, "39": true}

// microsoftListedOutput answers every way output departs from the first
// table of section 16.3, read with ECMA-48's grammar, and how many sequences
// of each kind it saw.
func microsoftListedOutput(output string) (departures []string, kinds map[string]int) {
	kinds = map[string]int{}
	buf := []byte(output)
	for at := 0; at < len(buf); {
		b := buf[at]
		switch {
		case b == 0x1b:
			length, complete := escapeExtent(buf[at:])
			sequence := string(buf[at : at+length])
			at += length
			if !complete || !microsoftListed.MatchString(sequence) {
				departures = append(departures, fmt.Sprintf("the sequence %q", sequence))
				continue
			}
			kinds[sequenceKind(sequence)]++
			if strings.HasSuffix(sequence, "m") {
				for _, parameter := range strings.Split(strings.TrimSuffix(strings.TrimPrefix(sequence, "\x1b["), "m"), ";") {
					if !sgrListed[parameter] {
						departures = append(departures, fmt.Sprintf("the SGR parameter %q in %q", parameter, sequence))
					}
				}
			}
		case b < 0x20 || b == 0x7f:
			departures = append(departures, fmt.Sprintf("the control character %#x", b))
			at++
		default:
			at++
		}
	}
	return departures, kinds
}

// sequenceKind names a sequence by its final byte and any private marker,
// with its numeric parameters set aside, for the count the sweep reports.
func sequenceKind(sequence string) string {
	return regexp.MustCompile(`[0-9;]+`).ReplaceAllString(sequence, "n")
}

// TestTheWindowsOutputIsOnlyWhatMicrosoftLists is the first clause of
// dinah-603/criteria/44, built for and run on Windows only, because Bubble
// Tea keys its scroll optimisation and newline mapping on runtime.GOOS. It
// drives every mode, the help, a resize and each prompt through the program
// built with interactiveProgramOptions for windows, at the four widths the
// layout tests use, parses what reaches the console with ECMA-48's grammar,
// and fails on any escape sequence, control character or SGR parameter
// outside the first table of section 16.3, reporting how many sequences of
// each kind it saw.
func TestTheWindowsOutputIsOnlyWhatMicrosoftLists(t *testing.T) {
	total := map[string]int{}
	for _, width := range []int{60, 99, 100, 160} {
		var output lockedBuffer
		run := everyModeRun(t, tuiBench(t), width, 30, &output, nil)
		if run.model == nil {
			t.Fatalf("the run at width %d never finished: %q", width, run.errw)
		}
		departures, kinds := microsoftListedOutput(output.String())
		for _, departure := range departures {
			t.Errorf("at width %d the program wrote %s, which the first table of section 16.3 does not list", width, departure)
		}
		for kind, count := range kinds {
			total[kind] += count
		}
	}
	var report []string
	for kind, count := range total {
		report = append(report, fmt.Sprintf("%q %d", kind, count))
	}
	sort.Strings(report)
	t.Logf("sequences seen by kind: %s", strings.Join(report, ", "))
}

// TestALendWritesOnlyWhatMicrosoftListsAndRepaintsWhole is dinah-623/criteria/35.
// It drives a whole lend through the program built for Windows, so every
// byte passes through consoleFrames: edit fx-1 typed at the command line ends
// the program, the stand-in editor runs while the seam's window is resized
// from 100x30 to 90x25, and a new program starts. Every sequence written
// across the lend is on the first table of section 16.3, and the new program
// repaints the whole board at the new size after it.
func TestALendWritesOnlyWhatMicrosoftListsAndRepaintsWhole(t *testing.T) {
	root := tuiBench(t)
	editorAppends(t)
	var output lockedBuffer
	s, seam := newScript(t, 100, 30, true)
	seam.output = &output
	cycles := make(chan *tea.Program, 8)
	var lend lendScript
	lend.seamed(seam, cycles)
	mark := -1
	var once sync.Once
	seam.observe = func(m *interactiveModel, msg tea.Msg) {
		if isEnterAtTheLine(m, msg) {
			once.Do(func() { seam.resize(90, 25) })
		}
	}
	run := s.run(root, seam, func() {
		waitForProgram(t, cycles)
		s.write(":edit fx-1" + keyEnter)
		waitForProgram(t, cycles)
		mark = len(output.String())
		s.waitFor("the flush after the lend", isFlushed)
		waitForOutput(t, &output, "fx-1")
		settledLength(output.String)
		s.write(keyCtrlC)
	})
	if run.model == nil || mark < 0 {
		t.Fatalf("the run never finished: %q", run.errw)
	}
	departures, kinds := microsoftListedOutput(output.String())
	for _, departure := range departures {
		t.Errorf("across the lend the program wrote %s, which the first table of section 16.3 does not list", departure)
	}
	first := lend.firstFrameOf(t, 1)
	if width, height := drawnSize(first); width > 89 || height != 25 {
		t.Errorf("the first frame after the lend is %dx%d, wanted 90x25", width, height)
	}
	if why := fullRepaint(output.String()[mark:], strings.Split(first, "\n")); why != "" {
		t.Errorf("the board after the lend was not repainted whole: %s", why)
	}
	t.Logf("%d sequence kinds seen across the lend", len(kinds))
}
