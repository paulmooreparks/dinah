package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf16"

	"dinah/internal/bench"
	"dinah/internal/contract"
	"dinah/internal/msg"
	"dinah/internal/screen"
	"dinah/internal/verb"
)

// The watch's tests drive the loop through terminalSeam: a screen that
// records what was drawn where, a size the test moves, and interrupts the
// test delivers. They run the watch on a goroutine of its own and act on the
// workbench from the test's goroutine through --workbench, so neither
// changes the working directory under the other.

// watchLog is the order things happened in across the screen and the seams,
// which is how a test asserts that one came before another.
type watchLog struct {
	mu      sync.Mutex
	entries []string
}

func (l *watchLog) add(entry string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries = append(l.entries, entry)
}

func (l *watchLog) all() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]string(nil), l.entries...)
}

// index is the position of the first entry starting with prefix at or after
// from, or -1.
func (l *watchLog) index(prefix string, from int) int {
	for i, entry := range l.all() {
		if i >= from && strings.HasPrefix(entry, prefix) {
			return i
		}
	}
	return -1
}

// watchFrame is what the window showed when a frame finished: each row's
// text by row, and the size the rig reported while it was drawn.
type watchFrame struct {
	rows          map[int]string
	width, height int
	at            time.Time
}

// text is the frame's rows from the first to the last, blank rows included.
func (f watchFrame) text() string {
	var lines []string
	for row := 1; row <= f.height; row++ {
		lines = append(lines, f.rows[row])
	}
	return strings.Join(lines, "\n")
}

// fakeScreen is a terminal that keeps what each row shows, a frame ending
// either with its status row or, for the one-line notice, with the erase
// under it. A hook runs before each row is written.
type fakeScreen struct {
	mu        sync.Mutex
	log       *watchLog
	colour    bool
	live      bool
	reason    string
	extra     string
	rig       *watchRig
	rows      map[int]string
	frames    []watchFrame
	maxRow    int
	lines     []string
	beforeRow func(row int)
}

func (f *fakeScreen) Colour() bool { return f.colour }

func (f *fakeScreen) Live() (bool, string, string) { return f.live, f.reason, f.extra }

func (f *fakeScreen) Begin() error {
	f.log.add("begin")
	return f.Clear()
}

func (f *fakeScreen) Clear() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.log.add("clear")
	f.rows = map[int]string{}
	return nil
}

func (f *fakeScreen) Row(row int, segments []screen.Segment) error {
	if f.beforeRow != nil {
		f.beforeRow(row)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.log.add(fmt.Sprintf("row %d", row))
	if row > f.maxRow {
		f.maxRow = row
	}
	f.rows[row] = screen.Text(segments)
	width, height := f.rig.size()
	if row == height {
		f.frames = append(f.frames, watchFrame{rows: copyRows(f.rows), width: width, height: height, at: time.Now()})
	}
	return nil
}

func (f *fakeScreen) EraseBelow(row int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.log.add(fmt.Sprintf("erase-below %d", row))
	for r := range f.rows {
		if r > row {
			delete(f.rows, r)
		}
	}
	width, height := f.rig.size()
	if row == 1 && (width < watchMinimumWidth || height < watchMinimumHeight) {
		f.frames = append(f.frames, watchFrame{rows: copyRows(f.rows), width: width, height: height, at: time.Now()})
	}
	return nil
}

func (f *fakeScreen) Line(segments []screen.Segment) error {
	f.mu.Lock()
	f.lines = append(f.lines, screen.Text(segments))
	f.mu.Unlock()
	f.log.add("line")
	return nil
}

func (f *fakeScreen) Reset() error {
	f.log.add("reset")
	return nil
}

func (f *fakeScreen) Restore(afterRow int) error {
	f.log.add(fmt.Sprintf("restore %d", afterRow))
	return nil
}

// allFrames is every frame finished so far.
func (f *fakeScreen) allFrames() []watchFrame {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]watchFrame(nil), f.frames...)
}

func copyRows(rows map[int]string) map[int]string {
	copied := make(map[int]string, len(rows))
	for row, text := range rows {
		copied[row] = text
	}
	return copied
}

// watchRig is one installed seam: the screen, the size, the interrupt
// channel the watch registered, and the log they share.
type watchRig struct {
	mu         sync.Mutex
	width      int
	height     int
	log        *watchLog
	scr        *fakeScreen
	registered chan<- os.Signal
	notified   int
}

// newWatchRig installs a seam reporting a live, colour-capable terminal of
// the given size, and removes it when the test ends.
func newWatchRig(t *testing.T, width, height int) *watchRig {
	t.Helper()
	log := &watchLog{}
	rig := &watchRig{width: width, height: height, log: log}
	rig.scr = &fakeScreen{log: log, colour: true, live: true, rig: rig, rows: map[int]string{}}
	rig.install(func(io.Writer) screen.Screen { return rig.scr })
	t.Cleanup(func() { terminalSeam = nil })
	return rig
}

// install puts the rig's seams in place around a given screen.
func (r *watchRig) install(open func(io.Writer) screen.Screen) {
	terminalSeam = &terminalSeams{
		screen: open,
		size:   r.size,
		notify: func(c chan<- os.Signal) {
			r.mu.Lock()
			r.registered = c
			r.notified++
			r.mu.Unlock()
			r.log.add("notify")
		},
		stop: func(chan<- os.Signal) { r.log.add("stop") },
	}
}

func (r *watchRig) size() (int, int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.width, r.height
}

// resize moves the size the next read reports.
func (r *watchRig) resize(width, height int) {
	r.mu.Lock()
	r.width, r.height = width, height
	r.mu.Unlock()
	r.log.add(fmt.Sprintf("resize %dx%d", width, height))
}

// interrupt delivers an interrupt the way os/signal does: to the registered
// channel, dropped where the channel is full, since Notify "will not block
// sending to c".
func (r *watchRig) interrupt() {
	r.mu.Lock()
	c := r.registered
	r.mu.Unlock()
	if c == nil {
		return
	}
	select {
	case c <- os.Interrupt:
		r.log.add("interrupt delivered")
	default:
		r.log.add("interrupt dropped")
	}
}

// asideRun is a command running on a goroutine of its own.
type asideRun struct {
	done chan struct{}
	code int
	out  string
	errw string
}

// runAside runs one invocation through runCLI on a goroutine of its own, so
// the output check still reads both its streams. It names the workbench with
// --workbench, so which workbench it opens does not depend on the working
// directory another goroutine's runCLI may have set.
func runAside(t *testing.T, root string, argv ...string) *asideRun {
	t.Helper()
	r := &asideRun{done: make(chan struct{})}
	full := append([]string{"--workbench", soleBenchDir(t, root)}, argv...)
	go func() {
		got := runCLI(t, root, full...)
		r.code, r.out, r.errw = got.code, got.out, got.errw
		close(r.done)
	}()
	return r
}

// finish waits for an invocation to end.
func (r *asideRun) finish(t *testing.T) {
	t.Helper()
	select {
	case <-r.done:
	case <-time.After(20 * time.Second):
		t.Fatal("the invocation did not end within twenty seconds")
	}
}

// actAside runs one act against the workbench and fails unless it exits zero.
func actAside(t *testing.T, root string, argv ...string) {
	t.Helper()
	r := runAside(t, root, argv...)
	r.finish(t)
	if r.code != 0 {
		t.Fatalf("%v: exit %d\n%s", argv, r.code, r.errw)
	}
}

// waitFor polls a condition for up to limit.
func waitFor(t *testing.T, limit time.Duration, what string, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(limit)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("waited %s and %s did not happen", limit, what)
}

// watchedBench is a workbench declaring a columns view that collapses
// nothing, with one card in its intake column.
func watchedBench(t *testing.T) string {
	root := newBench(t)
	declareViewsIn(t, root, bench.ViewsKey+":\n  every:\n    title: Every\n    layout: columns\n    order: column\n    collapsed: []\n    sections:\n      - query: \"state:ready,active,blocked\"\n")
	mustRunHere(t, root, "add", "A card to watch")
	return root
}

// TestWatchRefusesBeforeDrawing is dinah-288/criteria/17: --watch with
// --json, with no view, on output that is not a terminal, in a window of
// 39x24 or of 80x5, and on a terminal description lacking cup, each refused
// with nothing on stdout, while 40x6 draws.
func TestWatchRefusesBeforeDrawing(t *testing.T) {
	root := watchedBench(t)
	machine := runCLI(t, root, "--json", "view", "every", "--watch")
	if machine.code == 0 || !strings.Contains(machine.out, `"refusal": "malformed"`) || !strings.Contains(machine.out, `"detail": "--watch --json"`) || strings.Contains(machine.out, `"view"`) {
		t.Errorf("--watch --json answered %d %q", machine.code, machine.out)
	}
	unnamed := runCLI(t, root, "view", "--watch")
	if unnamed.code == 0 || unnamed.out != "" || !strings.HasPrefix(unnamed.errw, contract.Malformed+" --watch ") {
		t.Errorf("--watch with no view answered %d %q %q", unnamed.code, unnamed.out, unnamed.errw)
	}
	redirected := runCLI(t, root, "view", "every", "--watch")
	if redirected.code == 0 || redirected.out != "" || !strings.HasPrefix(redirected.errw, contract.WatchUnavailable+" ") || !strings.Contains(redirected.errw, contract.WatchNotATerminal) {
		t.Errorf("redirected output answered %d %q %q", redirected.code, redirected.out, redirected.errw)
	}
	for _, size := range [][2]int{{39, 24}, {80, 5}} {
		rig := newWatchRig(t, size[0], size[1])
		got := runCLI(t, root, "view", "every", "--watch")
		want := fmt.Sprintf("%dx%d", size[0], size[1])
		if got.code == 0 || got.out != "" || !strings.Contains(got.errw, contract.WatchTooSmall) || !strings.Contains(got.errw, want) {
			t.Errorf("a %s window answered %d %q %q", want, got.code, got.out, got.errw)
		}
		if len(rig.log.all()) != 0 {
			t.Errorf("a refused watch touched the terminal: %v", rig.log.all())
		}
	}
	noCup := terminfoFixture(t, "64", "dinah-no-cup")
	rig := newWatchRig(t, 80, 24)
	rig.install(func(text io.Writer) screen.Screen { return screen.NewTerminfo(noCup, text) })
	got := runCLI(t, root, "view", "every", "--watch")
	if got.code == 0 || got.out != "" || !strings.Contains(got.errw, contract.WatchMissingCapability) || !strings.Contains(got.errw, "cup") {
		t.Errorf("a description lacking cup answered %d %q %q", got.code, got.out, got.errw)
	}
	fits := newWatchRig(t, 40, 6)
	watching := runAside(t, root, "view", "every", "--watch")
	waitFor(t, 5*time.Second, "a frame at 40x6", func() bool { return len(fits.scr.allFrames()) > 0 })
	fits.interrupt()
	watching.finish(t)
	if watching.code != 0 {
		t.Errorf("a 40x6 watch exited %d: %s", watching.code, watching.errw)
	}
}

// terminfoFixture reads one of the screen package's committed entries.
func terminfoFixture(t *testing.T, middle, name string) *screen.Terminfo {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "internal", "screen", "testdata", "terminfo", middle, name))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	entry, err := screen.ParseTerminfo(data)
	if err != nil {
		t.Fatalf("parse %s: %v", name, err)
	}
	return entry
}

// TestAMoveDuringAFrameIsDrawnNext is dinah-288/criteria/18: a card moved
// after a frame's view was read and before it was written is drawn by the
// next frame in its destination column only, with no further change to the
// workbench, and that frame's status line names the move. No frame shows the
// card in two columns.
func TestAMoveDuringAFrameIsDrawnNext(t *testing.T) {
	root := watchedBench(t)
	rig := newWatchRig(t, 80, 24)
	moved := false
	terminalSeam.afterRead = func() {
		if moved {
			return
		}
		moved = true
		actAside(t, root, "move", "fx-1", "doing")
	}
	watching := runAside(t, root, "view", "every", "--watch")
	waitFor(t, 5*time.Second, "a second frame", func() bool { return len(rig.scr.allFrames()) >= 2 })
	rig.interrupt()
	watching.finish(t)
	frames := rig.scr.allFrames()
	if !strings.Contains(frames[0].text(), "Intake (1)") {
		t.Errorf("the first frame, read before the move, drew:\n%s", frames[0].text())
	}
	second := frames[1].text()
	if !strings.Contains(second, "Doing (1)") || strings.Contains(second, "Intake (") {
		t.Errorf("the next frame drew:\n%s", second)
	}
	if status := frames[1].rows[24]; !strings.Contains(status, "fx-1 moved") || !strings.Contains(status, "by alka") {
		t.Errorf("the next frame's status line reads %q", status)
	}
	for i, frame := range frames {
		if strings.Count(frame.text(), "○ 1") > 1 {
			t.Errorf("frame %d draws the card twice:\n%s", i+1, frame.text())
		}
	}
}

// recordingConsole stands in for a console screen buffer in the head's
// tests, keeping the cursor and the attributes the way the documented
// functions change them, and recording every call in the shared log.
type recordingConsole struct {
	mu            sync.Mutex
	log           *watchLog
	info          screen.BufferInfo
	cursorSize    uint32
	cursorVisible bool
	units         int
	text          []string
	onWrite       func(written int)
	onRestore     func()
}

func (r *recordingConsole) ScreenBufferInfo() (screen.BufferInfo, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.info, nil
}

func (r *recordingConsole) SetCursorPosition(x, y int) error {
	r.mu.Lock()
	r.info.CursorX, r.info.CursorY = x, y
	r.mu.Unlock()
	r.log.add(fmt.Sprintf("position %d,%d", x, y))
	return nil
}

func (r *recordingConsole) SetTextAttribute(attributes uint16) error {
	r.mu.Lock()
	r.info.Attributes = attributes
	r.mu.Unlock()
	r.log.add(fmt.Sprintf("attribute %#04x", attributes))
	return nil
}

func (r *recordingConsole) FillCharacter(character rune, length, x, y int) error {
	r.log.add(fmt.Sprintf("fill %d at %d,%d", length, x, y))
	return nil
}

func (r *recordingConsole) FillAttribute(attributes uint16, length, x, y int) error {
	return nil
}

func (r *recordingConsole) CursorInfo() (uint32, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.cursorSize, r.cursorVisible, nil
}

func (r *recordingConsole) SetCursorInfo(size uint32, visible bool) error {
	r.mu.Lock()
	r.cursorSize, r.cursorVisible = size, visible
	r.mu.Unlock()
	r.log.add(fmt.Sprintf("cursor visible %v", visible))
	if visible && r.onRestore != nil {
		r.onRestore()
	}
	return nil
}

// Write is what consolewriter would hand WriteConsoleW, counted in UTF-16
// units, with the cursor advanced one cell per rune.
func (r *recordingConsole) Write(p []byte) (int, error) {
	text := string(p)
	r.mu.Lock()
	r.text = append(r.text, text)
	r.units += len(utf16.Encode([]rune(text)))
	for _, c := range text {
		if c == '\n' {
			r.info.CursorX = 0
			r.info.CursorY++
			continue
		}
		r.info.CursorX++
	}
	written := len(r.text)
	r.mu.Unlock()
	r.log.add(fmt.Sprintf("text %q", text))
	if r.onWrite != nil {
		r.onWrite(written)
	}
	return len(p), nil
}

// busyBench is a workbench whose board, drawn with --all, is taller than
// any window the watch's tests use short of 60 rows: sixty cards across
// twelve columns.
func busyBench(t *testing.T) string {
	root := newBenchFromDefinition(t, fourteenColumns)
	declareViewsIn(t, root, allColumnsView)
	for i := 0; i < 60; i++ {
		position := 2 + i%12
		addTo(t, root, position, fmt.Sprintf("Card %d with a title long enough to be cut at any width the board draws", i))
	}
	return root
}

// TestCtrlCMidFrameRestoresTheConsole is the console half of
// dinah-288/criteria/19. The window is 1,000 columns wide, which puts all
// twelve columns in one band and makes the frame more than 8,000 UTF-16
// units long before the interrupt arrives, while row 10's text is being
// written; a second interrupt arrives while the restore is showing the
// cursor. Row 10 is finished and erased to its end, no text reaching the
// console carries an escape, the restore puts the attributes and the cursor
// back, the second interrupt goes unread, the signal is given back only after
// the restore's last call, and the watch exits 0.
func TestCtrlCMidFrameRestoresTheConsole(t *testing.T) {
	root := busyBench(t)
	rig := newWatchRig(t, 1000, 60)
	console := &recordingConsole{
		log:           rig.log,
		info:          screen.BufferInfo{Attributes: 0x0007, Left: 0, Top: 0, Right: 999, Bottom: 59},
		cursorSize:    25,
		cursorVisible: true,
	}
	rowStarted := -1
	console.onWrite = func(int) {
		console.mu.Lock()
		onRow := console.info.CursorY == 9
		console.mu.Unlock()
		if rowStarted < 0 && onRow {
			rowStarted = len(rig.log.all())
			rig.interrupt()
		}
	}
	console.onRestore = func() { rig.interrupt() }
	rig.install(func(text io.Writer) screen.Screen { return screen.NewConsole(console, console) })
	watching := runAside(t, root, "view", "every", "--watch", "--all")
	watching.finish(t)
	if watching.code != 0 {
		t.Fatalf("the interrupted watch exited %d: %s", watching.code, watching.errw)
	}
	if rowStarted < 0 {
		t.Fatalf("row 10 was never written, so the interrupt was never delivered mid-row:\n%s", strings.Join(rig.log.all(), "\n"))
	}
	log := rig.log.all()
	afterRow := log[rowStarted:]
	fill := -1
	for i, entry := range afterRow {
		if strings.HasPrefix(entry, "fill ") && strings.HasSuffix(entry, ",9") {
			fill = i
			break
		}
		if strings.HasPrefix(entry, "position 0,10") {
			break
		}
	}
	if fill < 0 {
		t.Errorf("row 10 was not finished and erased after the interrupt:\n%s", strings.Join(afterRow, "\n"))
	}
	if next := rig.log.index("position 0,10", rowStarted); next >= 0 && next < rig.log.index("cursor visible true", 0) {
		t.Error("a row after the interrupted one was written before the restore")
	}
	if console.units <= 8000 {
		t.Errorf("the console received %d UTF-16 units, not more than 8,000, so the case did not reach consolewriter's split", console.units)
	}
	for _, text := range console.text {
		if strings.ContainsRune(text, 0x1b) {
			t.Errorf("text reaching the console carries an escape: %q", text)
		}
	}
	if console.info.Attributes != 0x0007 || !console.cursorVisible {
		t.Errorf("after the restore the attributes are %#04x and the cursor visible %v", console.info.Attributes, console.cursorVisible)
	}
	dropped := rig.log.index("interrupt", rig.log.index("fill 1", rowStarted)+1)
	shown := rig.log.index("cursor visible true", 0)
	stopped := rig.log.index("stop", 0)
	lastRestoreWrite := -1
	for i, entry := range log {
		if entry == `text "\n"` {
			lastRestoreWrite = i
		}
	}
	if dropped < 0 || shown < 0 || stopped < 0 || lastRestoreWrite < 0 {
		t.Fatalf("the log lacks the second interrupt, the restore or the stop:\n%s", strings.Join(log, "\n"))
	}
	if !(shown <= dropped && dropped < lastRestoreWrite && lastRestoreWrite < stopped) {
		t.Errorf("the second interrupt, the restore and the stop happened out of order:\n%s", strings.Join(log[shown-3:], "\n"))
	}
}

// terminfoSequence matches one whole sequence the xterm-256color entry
// supplies for the watch: cup, el, ed, clear, civis, cnorm, setaf and sgr0.
var terminfoSequence = regexp.MustCompile(`\x1b\[[0-9;?]*[A-Za-z]|\x1b\(B`)

// sequenceRecorder keeps every write the terminfo layer makes and delivers an
// interrupt while the write for one row is happening.
type sequenceRecorder struct {
	mu      sync.Mutex
	writes  []string
	onWrite func(text string)
}

func (s *sequenceRecorder) Write(p []byte) (int, error) {
	s.mu.Lock()
	s.writes = append(s.writes, string(p))
	s.mu.Unlock()
	if s.onWrite != nil {
		s.onWrite(string(p))
	}
	return len(p), nil
}

// TestCtrlCMidFrameKeepsEverySequenceWhole is the POSIX half of
// dinah-288/criteria/19: through the terminfo layer over the committed
// xterm-256color entry, every escape in every write begins a sequence that
// ends within that write, an interrupt during row 10 still finishes it, and
// the watch exits 0 with the cursor shown by the restore.
func TestCtrlCMidFrameKeepsEverySequenceWhole(t *testing.T) {
	root := busyBench(t)
	entry := terminfoFixture(t, "78", "xterm-256color")
	rig := newWatchRig(t, 1000, 60)
	recorder := &sequenceRecorder{}
	interrupted := false
	recorder.onWrite = func(text string) {
		if !interrupted && strings.HasPrefix(text, "\x1b[10;1H") {
			interrupted = true
			rig.interrupt()
		}
		if strings.Contains(text, "\x1b[?25h") {
			rig.interrupt()
		}
	}
	rig.install(func(io.Writer) screen.Screen { return screen.NewTerminfo(entry, recorder) })
	watching := runAside(t, root, "view", "every", "--watch", "--all")
	watching.finish(t)
	if watching.code != 0 || !interrupted {
		t.Fatalf("the watch exited %d, interrupted %v: %s", watching.code, interrupted, watching.errw)
	}
	escapes := 0
	for _, write := range recorder.writes {
		whole := len(terminfoSequence.FindAllString(write, -1))
		all := strings.Count(write, "\x1b")
		escapes += all
		if whole != all {
			t.Errorf("a write carries %d escapes and %d whole sequences: %q", all, whole, write)
		}
	}
	if escapes == 0 {
		t.Error("no write carried an escape, so nothing was checked")
	}
	last := recorder.writes[len(recorder.writes)-1]
	if !strings.Contains(last, "\x1b[?25h") || !strings.HasSuffix(last, "\n") {
		t.Errorf("the last write is not the restore: %q", last)
	}
	for _, write := range recorder.writes {
		if strings.HasPrefix(write, "\x1b[11;1H") {
			t.Error("a row after the interrupted one was written")
		}
	}
}

// TestTheStatusLineNamesTheChange is dinah-288/criteria/20: the first frame
// reads watching since, a change reads the redraw's time and the last event
// as subject, action, detail and actor, more events add and N more, a change
// with no event names the departure, the columns or the workbench, and when
// the line is too wide only the change is cut.
func TestTheStatusLineNamesTheChange(t *testing.T) {
	root := watchedBench(t)
	s := &session{r: msg.For(msg.Base)}
	events := func(names ...string) *verb.ChangeSet {
		set := &verb.ChangeSet{}
		for _, name := range names {
			ev := verb.ChangeEvent{Scope: verb.ScopeCard, Ref: "fx-7"}
			ev.Event.Event = name
			ev.Event.Actor.Name = "alka"
			set.Events = append(set.Events, ev)
		}
		return set
	}
	cases := []struct {
		set  *verb.ChangeSet
		want string
	}{
		{events("claimed"), "fx-7 claimed by alka"},
		{events("created", "claimed", "released"), "fx-7 released by alka, and 2 more"},
		{events("expired"), "fx-7 expired, held by alka"},
		{&verb.ChangeSet{Gone: []verb.GoneEntity{{ID: "a1", Ref: "fx-3", Fate: verb.FateArchived}}}, "fx-3 archived"},
		{&verb.ChangeSet{Columns: []verb.ColumnChange{{ID: "c1", Title: "Doing"}}}, "the columns changed"},
		{&verb.ChangeSet{}, "the workbench changed"},
	}
	for _, c := range cases {
		if got := s.changeText(c.set); got != c.want {
			t.Errorf("the change reads %q, want %q", got, c.want)
		}
	}
	change := "dinah-594 moved Implement to Agent Code Review by claude"
	line := statusLine("updated 09:41:07", change, "Ctrl+C stops", " · ", 100, tailEllipsis)
	if line != "updated 09:41:07 · "+change+" · Ctrl+C stops" {
		t.Errorf("a line that fits reads %q", line)
	}
	cut := statusLine("updated 09:41:07", change, "Ctrl+C stops", " · ", 79, tailEllipsis)
	if cut != "updated 09:41:07 · dinah-594 moved Implement to Agent Code Revi… · Ctrl+C stops" {
		t.Errorf("a line cut to 79 reads %q, not the specification's own example", cut)
	}
	narrow := statusLine("updated 09:41:07", change, "Ctrl+C stops", " · ", 20, tailEllipsis)
	if narrow != "d… · Ctrl+C stops" {
		t.Errorf("a line cut to 20 reads %q", narrow)
	}
	rig := newWatchRig(t, 80, 24)
	watching := runAside(t, root, "view", "every", "--watch")
	waitFor(t, 5*time.Second, "the first frame", func() bool { return len(rig.scr.allFrames()) >= 1 })
	actAside(t, root, "move", "fx-1", "doing")
	waitFor(t, 5*time.Second, "a second frame", func() bool { return len(rig.scr.allFrames()) >= 2 })
	rig.interrupt()
	watching.finish(t)
	frames := rig.scr.allFrames()
	first, second := frames[0].rows[24], frames[1].rows[24]
	if !regexp.MustCompile(`^watching since \d\d:\d\d:\d\d · Ctrl\+C stops$`).MatchString(first) {
		t.Errorf("the first frame's status reads %q", first)
	}
	if !regexp.MustCompile(`^updated \d\d:\d\d:\d\d · fx-1 moved Intake to Doing by alka · Ctrl\+C stops$`).MatchString(second) {
		t.Errorf("the second frame's status reads %q", second)
	}
}

// TestAResizeRedrawsWithinTwoSeconds is dinah-288/criteria/21: a new width
// with no change to the workbench is drawn within two seconds, and the
// window is cleared first.
func TestAResizeRedrawsWithinTwoSeconds(t *testing.T) {
	root := watchedBench(t)
	rig := newWatchRig(t, 80, 24)
	watching := runAside(t, root, "view", "every", "--watch")
	waitFor(t, 5*time.Second, "the first frame", func() bool { return len(rig.scr.allFrames()) >= 1 })
	resized := time.Now()
	from := len(rig.log.all())
	rig.resize(100, 24)
	waitFor(t, 5*time.Second, "a frame at the new width", func() bool {
		frames := rig.scr.allFrames()
		return frames[len(frames)-1].width == 100
	})
	frames := rig.scr.allFrames()
	elapsed := frames[len(frames)-1].at.Sub(resized)
	rig.interrupt()
	watching.finish(t)
	if elapsed > 2*time.Second {
		t.Errorf("the redraw at the new width took %s", elapsed)
	}
	clear := rig.log.index("clear", from)
	row := rig.log.index("row 1", from)
	if clear < 0 || row < 0 || clear > row {
		t.Errorf("the window was not cleared before the redraw: %v", rig.log.all()[from:])
	}
}

// TestFramesStayInsideTheWindow is dinah-288/criteria/22 and
// dinah-288/criteria/27: at 40x6, 80x24 and 200x60 every row draws in at
// most W-1 columns, nothing is written below row H, a drawing taller than
// H-1 lines keeps H-2 of them and the hidden-lines line, and the status line
// is on row H with Ctrl+C stops whole. A window taken to 30x24 draws only
// the notice on row 1, cut to 29, and a window taken back to 80x24 draws a
// normal frame. A height moved from 24 to 10 is read before the next frame.
func TestFramesStayInsideTheWindow(t *testing.T) {
	root := busyBench(t)
	for _, size := range [][2]int{{40, 6}, {80, 24}, {200, 60}} {
		width, height := size[0], size[1]
		rig := newWatchRig(t, width, height)
		watching := runAside(t, root, "view", "every", "--watch", "--all")
		waitFor(t, 5*time.Second, "the first frame", func() bool { return len(rig.scr.allFrames()) >= 1 })
		rig.interrupt()
		watching.finish(t)
		frame := rig.scr.allFrames()[0]
		assertFrame(t, frame, width, height)
		if rig.scr.maxRow > height {
			t.Errorf("at %dx%d row %d was written", width, height, rig.scr.maxRow)
		}
		if height < 60 && !strings.Contains(frame.rows[height-1], "more lines below") {
			t.Errorf("at %dx%d the last row of drawing is %q, want the hidden-lines line", width, height, frame.rows[height-1])
		}
	}
	rig := newWatchRig(t, 80, 24)
	watching := runAside(t, root, "view", "every", "--watch")
	waitFor(t, 5*time.Second, "the first frame", func() bool { return len(rig.scr.allFrames()) >= 1 })
	rig.resize(30, 24)
	waitFor(t, 5*time.Second, "the notice", func() bool {
		frames := rig.scr.allFrames()
		return frames[len(frames)-1].width == 30
	})
	frames := rig.scr.allFrames()
	notice := frames[len(frames)-1]
	if got := notice.rows[1]; displayWidth(got) > 29 || !strings.HasPrefix(got, "window too small") {
		t.Errorf("the notice row reads %q", got)
	}
	if len(notice.rows) != 1 {
		t.Errorf("the notice frame draws %d rows, want 1: %v", len(notice.rows), notice.rows)
	}
	rig.resize(80, 24)
	waitFor(t, 5*time.Second, "a normal frame", func() bool {
		frames := rig.scr.allFrames()
		return frames[len(frames)-1].width == 80
	})
	frames = rig.scr.allFrames()
	assertFrame(t, frames[len(frames)-1], 80, 24)
	before := len(frames)
	rig.mu.Lock()
	rig.height = 10
	rig.mu.Unlock()
	actAside(t, root, "add", "One more card")
	waitFor(t, 5*time.Second, "a frame at height 10", func() bool {
		frames := rig.scr.allFrames()
		return len(frames) > before && frames[len(frames)-1].height == 10
	})
	rig.interrupt()
	watching.finish(t)
	frames = rig.scr.allFrames()
	short := frames[len(frames)-1]
	assertFrame(t, short, 80, 10)
	for row := range short.rows {
		if row > 10 {
			t.Errorf("the height-10 frame wrote row %d", row)
		}
	}
}

// assertFrame holds one frame to its window: rows no wider than W-1, and
// the status line on row H with Ctrl+C stops whole.
func assertFrame(t *testing.T, frame watchFrame, width, height int) {
	t.Helper()
	for row, text := range frame.rows {
		if displayWidth(text) > width-1 {
			t.Errorf("at %dx%d row %d draws %d columns: %q", width, height, row, displayWidth(text), text)
		}
		if row > height {
			t.Errorf("at %dx%d row %d holds text", width, height, row)
		}
	}
	if status := frame.rows[height]; !strings.HasSuffix(status, "Ctrl+C stops") {
		t.Errorf("at %dx%d the status row reads %q", width, height, status)
	}
}

// TestARemovedWorkbenchEndsTheWatch is dinah-288/criteria/23: the watch
// restores the terminal as on Ctrl+C, reports the refusal on stderr and
// exits with the non-zero code reportError returns, without retrying.
func TestARemovedWorkbenchEndsTheWatch(t *testing.T) {
	root := watchedBench(t)
	rig := newWatchRig(t, 80, 24)
	watching := runAside(t, root, "view", "every", "--watch")
	waitFor(t, 5*time.Second, "the first frame", func() bool { return len(rig.scr.allFrames()) >= 1 })
	if err := os.RemoveAll(soleBenchDir(t, root)); err != nil {
		t.Fatalf("remove the workbench: %v", err)
	}
	watching.finish(t)
	if watching.code == 0 || watching.errw == "" {
		t.Errorf("the watch of a removed workbench exited %d with %q", watching.code, watching.errw)
	}
	restore := rig.log.index("restore", 0)
	stop := rig.log.index("stop", 0)
	if restore < 0 || stop < restore {
		t.Errorf("the terminal was not restored before the signal was given back: %v", rig.log.all())
	}
	if frames := len(rig.scr.allFrames()); frames > 2 {
		t.Errorf("the watch drew %d frames after the workbench went, so it retried", frames)
	}
}

// TestALapsedClaimIsDrawnOnceAndNamedOnce is dinah-288/criteria/28: a claim
// expiring while the workbench is quiet is drawn as ready within two
// seconds of its expiry; the frame that lapses it causes exactly one further
// frame, whose status line names the expiry, and then three quiet seconds
// pass with no frame and no journal line; and the expired event names the
// holder, not the watcher.
func TestALapsedClaimIsDrawnOnceAndNamedOnce(t *testing.T) {
	root := watchedBench(t)
	mustRunHere(t, root, "move", "fx-1", "doing")
	journalPath := filepath.Join(filepath.Dir(anchorPath(t, root, "fx-1")), "journal.ndjson")
	mustRunHere(t, root, "claim", "fx-1", "--expires", "2s")
	claimed := time.Now()
	rig := newWatchRig(t, 80, 24)
	watching := runAside(t, root, "--actor", "watcher", "view", "every", "--watch")
	waitFor(t, 5*time.Second, "the first frame", func() bool { return len(rig.scr.allFrames()) >= 1 })
	if first := rig.scr.allFrames()[0].text(); !strings.Contains(first, "● 1") {
		t.Fatalf("the first frame does not draw the claim:\n%s", first)
	}
	var ready time.Time
	waitFor(t, 6*time.Second, "the card drawn as ready", func() bool {
		for _, frame := range rig.scr.allFrames() {
			if strings.Contains(frame.text(), "○ 1") {
				ready = frame.at
				return true
			}
		}
		return false
	})
	if late := ready.Sub(claimed.Add(2 * time.Second)); late > 2*time.Second {
		t.Errorf("the lapsed claim was drawn as ready %s after it expired", late)
	}
	waitFor(t, 5*time.Second, "a frame naming the expiry", func() bool {
		frames := rig.scr.allFrames()
		return strings.Contains(frames[len(frames)-1].rows[24], "fx-1 expired, held by alka")
	})
	settled := len(rig.scr.allFrames())
	journal := journalLines(t, journalPath)
	time.Sleep(3 * time.Second)
	if frames := len(rig.scr.allFrames()); frames != settled {
		t.Errorf("%d further frames were drawn in three quiet seconds", frames-settled)
	}
	if lines := journalLines(t, journalPath); lines != journal {
		t.Errorf("the journal grew from %d to %d lines in three quiet seconds", journal, lines)
	}
	rig.interrupt()
	watching.finish(t)
	expired := 0
	data, err := os.ReadFile(journalPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.Contains(line, `"event":"expired"`) {
			expired++
			if !strings.Contains(line, `"name":"alka"`) {
				t.Errorf("the expired event is not attributed to the holder: %s", line)
			}
		}
	}
	if expired != 1 {
		t.Errorf("the journal carries %d expired events, want 1", expired)
	}
}

// journalLines counts the lines of a card's journal.
func journalLines(t *testing.T, path string) int {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return strings.Count(string(data), "\n")
}

// TestAnInterruptedColouredDrawingExits130 is dinah-288/criteria/29: a
// coloured one-shot drawing interrupted between lines resets the colour,
// leaves its last line ended, gives the signal back and exits 130, and a
// drawing with colour off registers no handler at all.
func TestAnInterruptedColouredDrawingExits130(t *testing.T) {
	root := colourFixture(t)
	rig := newWatchRig(t, 80, 24)
	rig.scr.beforeRow = nil
	lines := 0
	rig.install(func(io.Writer) screen.Screen {
		return &interruptingScreen{fakeScreen: rig.scr, after: 3, rig: rig, lines: &lines}
	})
	got := runCLI(t, root, "view", "every")
	if got.code != exitInterrupted {
		t.Errorf("the interrupted drawing exited %d, want %d", got.code, exitInterrupted)
	}
	if lines != 3 {
		t.Errorf("%d lines were written, want the three before the interrupt", lines)
	}
	log := rig.log.all()
	if strings.Join(log, " ") != "notify line line line interrupt delivered reset stop" {
		t.Errorf("the drawing did %v", log)
	}
	t.Setenv("NO_COLOR", "1")
	quiet := newWatchRig(t, 80, 24)
	if got := runCLI(t, root, "view", "every"); got.code != 0 || got.out == "" {
		t.Errorf("the drawing with colour off exited %d", got.code)
	}
	if quiet.notified != 0 || len(quiet.log.all()) != 0 {
		t.Errorf("a drawing with colour off registered a handler: %v", quiet.log.all())
	}
}

// interruptingScreen delivers an interrupt once a number of lines are
// written, which is between two lines of a one-shot drawing.
type interruptingScreen struct {
	*fakeScreen
	after int
	rig   *watchRig
	lines *int
}

func (s *interruptingScreen) Line(segments []screen.Segment) error {
	err := s.fakeScreen.Line(segments)
	*s.lines++
	if *s.lines == s.after {
		s.rig.interrupt()
	}
	return err
}
