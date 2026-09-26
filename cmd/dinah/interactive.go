//go:build tui

package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"

	"dinah/internal/contract"
	"dinah/internal/screen"
	"dinah/internal/screen/keyboard"
	"dinah/internal/verb"
)

// The smallest window the terminal head draws in, and the sizes its regions
// keep inside it.
const (
	// interactiveMinimumWidth and interactiveMinimumHeight are the smallest
	// window the head starts in and draws its full screen in.
	interactiveMinimumWidth  = 60
	interactiveMinimumHeight = 12
	// interactiveWideWidth is the narrowest window that draws the detail
	// pane beside the list pane.
	interactiveWideWidth = 100
	// interactiveMinimumBody is the fewest rows the body keeps.
	interactiveMinimumBody = 3
	// interactiveMessageLimit is the most rows the message area takes
	// outside the comment prompt.
	interactiveMessageLimit = 3
	// interactiveAreaMinimum and interactiveAreaMaximum bound the rows the
	// message area takes while the comment prompt is open, its label
	// included.
	interactiveAreaMinimum = 3
	interactiveAreaMaximum = 5
)

// interactiveDefaultView is the view dinah tui works in when none is named.
const interactiveDefaultView = "board"

// interactiveSeams stands in for everything the terminal head reads from or
// writes to outside the process: both streams' terminal check, the window's
// size, the keys, the terminal's description and the terminal itself. It is
// nil in a shipped binary. A test sets it, and then no mode or termios call
// reaches a terminal: the head writes its two bracketed-paste sequences to
// output with the frames, and feeds the POSIX decoder from keys on every
// GOOS.
type interactiveSeams struct {
	stdinTerminal  bool                           // what the terminal check answers for stdin
	stdoutTerminal bool                           // and for stdout
	width, height  int                            // what interactiveSize answers
	keys           io.Reader                      // bytes for the POSIX decoder, paste markers included, in place of the terminal
	terminfo       *screen.Terminfo               // the entry the decoder uses
	output         io.Writer                      // frames, in place of the terminal
	program        func(*tea.Program)             // called before Run, so a test can Send messages
	update         func(tea.Msg)                  // called at the top of Update, where a test plants a panic
	view           func()                         // called at the top of View, where a test plants a panic
	frame          func(string)                   // called with the content of every frame View answers
	command        func()                         // called at the top of every command the head returns
	discard        func()                         // called by the reader on every flush it makes
	finish         func(*interactiveModel, error) // called with the final model and Run's error
	repanic        func(any)                      // replaces the final panic of a crash report
	// lineDispatch, when set, is called by the command line with the name
	// of every command it is about to dispatch, and a true answer stops the
	// line there. It exists for the test that every command reaches its run
	// function, and is nil in a shipped binary as the whole seam is.
	lineDispatch func(name string) bool
	// lineLibrary, when set, is handed every library a line session opens,
	// so a test can reach the library a line opened in this process.
	lineLibrary func(*verb.Library)
	// bindingValue, when set, is called on each value a key binding
	// substitutes, before the value is checked, so a test can supply a value
	// no reference, slug or view name Dinah mints could carry.
	bindingValue func(placeholder, value string) string
	// lend, when set, is called by the head in place of running a command
	// that lends the terminal, with the session that command would run in,
	// so a test drives the lend without starting an editor. It answers the
	// exit status.
	lend func(line *session, words []string) int
	// pump reads keys across every cycle of one run, started by the first
	// cycle's reader.
	pump *keyPump
	// mu guards width and height, which a test changes while the program
	// runs.
	mu sync.Mutex
}

// resize changes the size interactiveSize answers under the seam.
func (seam *interactiveSeams) resize(width, height int) {
	seam.mu.Lock()
	defer seam.mu.Unlock()
	seam.width, seam.height = width, height
}

// interactiveSeam is the seam a test installs; nil means the real terminal.
var interactiveSeam *interactiveSeams

// runTUIHead runs the terminal head over one view, the board where none is
// named, in the build tagged tui. runTUI has already refused a machine format.
//
// The view is read once, and a refusal is reported exactly as dinah view
// reports it, so nothing reaches the terminal before that read has answered.
// Only then is the terminal checked, and only then does the interface start.
func runTUIHead(s *session, parsed *arguments) int {
	req := s.request("view", parsed)
	req.View = at(parsed.rest(), 0)
	if req.View == "" {
		req.View = interactiveDefaultView
	}
	req.ViewPlain = parsed.has("plain")
	req.Lang = s.r.Tag
	return s.withBench(func(l *verb.Library) int {
		first, err := l.DrawView(req)
		if err != nil {
			code := s.reportError(err)
			s.reportSectionRefused(l, req, err)
			return code
		}
		entry, err := s.interactiveCheck(req.View)
		if err != nil {
			return s.reportError(err)
		}
		s.pinStart(parsed.value("actor"), scanLangFlag(s.args))
		if interactiveSeam != nil {
			s.lineOnOpen = interactiveSeam.lineLibrary
		}
		return s.interactive(l, req, first, entry)
	})
}

// interactiveCheck answers the refusal the head meets before it changes
// anything, in the order the reasons are checked: a stream that is not a
// terminal, a window that reports no size, a window too small, and off
// Windows a TERM that is dumb or unset and a description that cannot be read
// or lacks an arrow key. It answers the terminal's description where one is
// read.
func (s *session) interactiveCheck(view string) (*screen.Terminfo, error) {
	if !s.interactiveTerminals() {
		return nil, contract.RefuseWith(contract.TUIUnavailable, view, map[string]string{"reason": contract.WatchNotATerminal})
	}
	width, height := s.interactiveSize()
	if width <= 0 || height <= 0 {
		return nil, contract.RefuseWith(contract.TUIUnavailable, view, map[string]string{"reason": contract.WatchNoSize})
	}
	if width < interactiveMinimumWidth || height < interactiveMinimumHeight {
		return nil, contract.RefuseWith(contract.TUIUnavailable, view, map[string]string{
			"reason":  contract.WatchTooSmall,
			"size":    strconv.Itoa(width) + "x" + strconv.Itoa(height),
			"minimum": strconv.Itoa(interactiveMinimumWidth) + "x" + strconv.Itoa(interactiveMinimumHeight),
		})
	}
	if runtime.GOOS == "windows" {
		return nil, nil
	}
	name := os.Getenv("TERM")
	if name == "" || name == "dumb" {
		return nil, contract.RefuseWith(contract.TUIUnavailable, view, map[string]string{"reason": contract.TUIDumbTerminal})
	}
	entry, err := s.interactiveTerminfo(name)
	if err != nil {
		return nil, contract.RefuseWith(contract.TUIUnavailable, view, map[string]string{"reason": contract.TUINoKeyDescription})
	}
	if missing := keyboard.MissingArrow(entry); missing != "" {
		return nil, contract.RefuseWith(contract.TUIUnavailable, view, map[string]string{
			"reason":     contract.TUINoKeyDescription,
			"capability": missing,
		})
	}
	return entry, nil
}

// interactiveTerminals reports whether standard input and standard output
// are both terminals.
func (s *session) interactiveTerminals() bool {
	if seam := interactiveSeam; seam != nil {
		return seam.stdinTerminal && seam.stdoutTerminal
	}
	in, ok := s.in.(*os.File)
	if !ok || !keyboard.IsTerminal(in) {
		return false
	}
	return s.rawOut != nil && keyboard.IsTerminal(s.rawOut)
}

// interactiveTerminfo reads the terminal's description by name, from the
// seam's entry under test.
func (s *session) interactiveTerminfo(name string) (*screen.Terminfo, error) {
	if seam := interactiveSeam; seam != nil {
		if seam.terminfo == nil {
			return nil, screen.ErrNoTerminfo
		}
		return seam.terminfo, nil
	}
	return screen.LoadTerminfo(name, os.Getenv)
}

// interactiveSize reads the window's size, and is the one place the head
// reads it from: the console window's rectangle on Windows, TIOCGWINSZ on
// POSIX, and the seam's size under test. COLUMNS and LINES are never read,
// because a window resized while the head runs has to be measured afresh.
func (s *session) interactiveSize() (int, int) {
	if seam := interactiveSeam; seam != nil {
		seam.mu.Lock()
		defer seam.mu.Unlock()
		return seam.width, seam.height
	}
	if s.rawOut == nil {
		return 0, 0
	}
	width, height, err := keyboard.WindowSize(s.rawOut)
	if err != nil {
		return 0, 0
	}
	return width, height
}

// interactiveSink takes what the key reader decodes and queues it, and
// returns at once whatever the program is doing, so the reader always comes
// back to its wait and sees Stop there. A forwarder goroutine hands the queue
// to the program in order.
type interactiveSink struct {
	mu     sync.Mutex
	queued []tea.Msg
	// ready carries one signal that the queue has grown; a signal sent while
	// one is already waiting is dropped, since the forwarder drains the whole
	// queue on each.
	ready chan struct{}
	// done is closed when the cycle ends, which stops the forwarder.
	done    chan struct{}
	program *tea.Program
}

// newInteractiveSink builds the sink of one cycle over its program.
func newInteractiveSink(program *tea.Program) *interactiveSink {
	return &interactiveSink{
		ready:   make(chan struct{}, 1),
		done:    make(chan struct{}),
		program: program,
	}
}

// queue appends a message and signals the forwarder without blocking.
func (k *interactiveSink) queue(msg tea.Msg) {
	k.mu.Lock()
	k.queued = append(k.queued, msg)
	k.mu.Unlock()
	select {
	case k.ready <- struct{}{}:
	default:
	}
}

// forward hands every queued message to the program in order until the cycle
// ends. A message still queued when it ends belongs to a program that has
// ended, and is dropped with it.
func (k *interactiveSink) forward() {
	for {
		select {
		case <-k.done:
			return
		case <-k.ready:
		}
		k.mu.Lock()
		taken := k.queued
		k.queued = nil
		k.mu.Unlock()
		for _, msg := range taken {
			select {
			case <-k.done:
				return
			default:
			}
			k.program.Send(msg)
		}
	}
}

// stop ends the forwarder, and is safe to call once per cycle.
func (k *interactiveSink) stop() {
	close(k.done)
}

// Event queues a key or a paste, stamped with its generation.
func (k *interactiveSink) Event(event keyboard.Event) {
	if event.Paste {
		k.queue(pasteMsg{content: event.Text, gen: event.Gen})
		return
	}
	k.queue(keyMsg{key: interactiveKeyMsg(event.Key), gen: event.Gen})
}

// Resized queues the notice that the console's buffer size changed.
func (k *interactiveSink) Resized() {
	k.queue(resizeMsg{})
}

// Flushed queues the confirmation of a flush.
func (k *interactiveSink) Flushed(gen uint64) {
	k.queue(flushedMsg{gen: gen})
}

// interactive runs the terminal head over the view first answered, until it
// is quit, and answers the exit code. It is a loop over cycles: each cycle
// runs one program over the same model, and a command that lends the
// terminal ends the cycle it was asked in, runs with the terminal given back,
// and starts the next.
func (s *session) interactive(l *verb.Library, req *verb.Request, first *verb.ViewAnswer, entry *screen.Terminfo) int {
	waiter, err := s.open()
	s.library = l
	if err != nil {
		return s.reportError(err)
	}
	minted, err := waiter.Changes(&verb.Request{Verb: "changes", Actor: req.Actor})
	if err != nil {
		return s.reportError(err)
	}
	width, height := s.interactiveSize()
	model := newInteractiveModel(s, l, waiter, req, first, minted.Cursor, width, height)
	for {
		runErr, started := s.cycle(model, entry)
		if !started {
			return s.reportError(runErr)
		}
		if interactiveSeam != nil && interactiveSeam.finish != nil && model.lend == nil {
			interactiveSeam.finish(model, runErr)
		}
		if crash := model.crashed(); crash != nil {
			return s.reportCrash(crash)
		}
		if model.lend != nil && runErr == nil {
			model.lendTerminal()
			continue
		}
		switch {
		case errors.Is(runErr, tea.ErrInterrupted):
			return exitInterrupted
		case runErr != nil:
			return s.reportError(runErr)
		}
		return 0
	}
}

// cycle runs one program over the model: it measures the window, takes the
// keyboard, starts a reader and its forwarder, runs a new Bubble Tea program,
// and gives the keyboard back on every path out, a panic included. It answers
// Run's error and whether the cycle started at all, and leaves any lend the
// model asked for on the model.
//
// The keyboard is restored in a function deferred before Run is called, so
// it runs on every path out of this function, a panic in the head's own code
// included. The same function is called as soon as Run returns, so the crash
// report of a recovered panic is written to a terminal already given back,
// and the next cycle starts on a terminal this one has already restored.
func (s *session) cycle(model *interactiveModel, entry *screen.Terminfo) (error, bool) {
	width, height := s.interactiveSize()
	if width > 0 && height > 0 {
		model.width, model.height = width, height
		model.fit()
	}
	options, frames := s.interactiveOptions(model.width, model.height)
	program := tea.NewProgram(model, options...)
	model.program = program
	model.gate = &changeGate{}
	if frames != nil {
		// A piece the console wrote short asks for the whole screen on the
		// next frame. The renderer calls this while it holds its own lock,
		// so the message is sent from a goroutine of its own.
		frames.repaint = func() { go program.Send(repaintMsg{}) }
	}
	leave, reader, err := s.enterKeyboard(entry)
	if err != nil {
		return err, false
	}
	model.reader = reader
	sink := newInteractiveSink(program)
	readerDone := make(chan struct{})
	var restoreOnce sync.Once
	restore := func() {
		restoreOnce.Do(func() {
			reader.Stop()
			<-readerDone
			sink.stop()
			// Anything the frames writer still holds is written before the
			// bracketed-paste disable and the output mode's restore.
			if frames != nil {
				frames.Flush()
			}
			if err := leave(); err != nil {
				s.errLine(contract.OutcomeUnreachable + " " + err.Error())
			}
		})
	}
	defer restore()
	go func() {
		defer close(readerDone)
		defer func() {
			if r := recover(); r != nil {
				model.recordCrash(r)
				go program.Send(crashMsg{})
			}
		}()
		reader.Run(sink)
	}()
	go sink.forward()
	if interactiveSeam != nil && interactiveSeam.program != nil {
		interactiveSeam.program(program)
	}
	_, runErr := program.Run()
	restore()
	model.gate.close()
	return runErr, true
}

// interactiveOptions are the program's options for this GOOS, with the
// terminal or the seam's writer as output. Under the seam the program also
// ignores signals, and the frames writer, where the GOOS has one, is answered
// so the head can flush it as it leaves.
func (s *session) interactiveOptions(width, height int) ([]tea.ProgramOption, *consoleFrames) {
	var output io.Writer = s.rawOut
	if interactiveSeam != nil {
		output = interactiveSeam.output
	}
	options, frames := interactiveProgramOptions(runtime.GOOS, output, os.Environ(), width, height)
	if interactiveSeam != nil {
		options = append(options, tea.WithWindowSize(width, height), tea.WithoutSignals())
	}
	return options, frames
}

// interactiveProgramOptions answers the program's options on a GOOS: input
// disabled because keys come from Dinah's reader, the ANSI colour profile or
// the one with no colour at all under NO_COLOR, the environment
// interactiveEnviron answers, and the output.
//
// On Windows, and on Windows only, the output is a consoleFrames around the
// writer rather than the terminal file itself, and the size is passed with
// tea.WithWindowSize, because Bubble Tea has no terminal output there to read
// it from. Together with TERM=xterm, that leaves Bubble Tea's renderer
// nothing to write that Microsoft's "Console Virtual Terminal Sequences" page
// does not list, as section 16 of the specification derives from the
// renderer's source. Off Windows the output is the writer, and Bubble Tea
// keeps the process's TERM and its own size handling.
func interactiveProgramOptions(goos string, output io.Writer, environ []string, width, height int) ([]tea.ProgramOption, *consoleFrames) {
	profile := colorprofile.ANSI
	if !colourAllowed() {
		profile = colorprofile.Ascii
	}
	options := []tea.ProgramOption{
		tea.WithInput(nil),
		tea.WithColorProfile(profile),
		tea.WithEnvironment(interactiveEnviron(environ, goos)),
	}
	if goos != "windows" {
		return append(options, tea.WithOutput(output)), nil
	}
	frames := newConsoleFrames(output)
	return append(options, tea.WithOutput(frames), tea.WithWindowSize(width, height)), frames
}

// interactiveEnviron is the environment Bubble Tea reads. On Windows it is
// the process's own with every TERM entry removed and exactly TERM=xterm
// added, whatever TERM the process had, since a Windows shell may set one
// such as cygwin that leaves the renderer no capabilities, and without
// capabilities it writes insert mode, which Microsoft's page does not list.
// The process's own environment is not changed. Off Windows it is the
// process's own.
func interactiveEnviron(environ []string, goos string) []string {
	if goos != "windows" {
		return environ
	}
	kept := make([]string, 0, len(environ)+1)
	for _, variable := range environ {
		name, _, _ := strings.Cut(variable, "=")
		if strings.EqualFold(name, "TERM") {
			continue
		}
		kept = append(kept, variable)
	}
	return append(kept, "TERM=xterm")
}

// enterKeyboard takes the keyboard and answers how to give it back and the
// reader to take keys from. Under the seam it writes the bracketed-paste
// enable to the seam's output and changes no mode.
func (s *session) enterKeyboard(entry *screen.Terminfo) (func() error, keyboard.Reader, error) {
	if seam := interactiveSeam; seam != nil {
		out := seam.output
		if _, err := io.WriteString(out, keyboard.BracketedPasteOn); err != nil {
			return nil, nil, err
		}
		if seam.pump == nil {
			keys := seam.keys
			if keys == nil {
				keys = strings.NewReader("")
			}
			seam.pump = startKeyPump(keys)
		}
		leave := func() error {
			_, err := io.WriteString(out, keyboard.BracketedPasteOff)
			return err
		}
		source := &cycleKeys{pump: seam.pump, done: make(chan struct{})}
		reader := keyboard.NewStreamReader(source, seam.terminfo, seam.discard)
		return leave, &cycleReader{Reader: reader, keys: source}, nil
	}
	in, ok := s.in.(*os.File)
	if !ok {
		return nil, nil, contract.RefuseWith(contract.TUIUnavailable, interactiveDefaultView, map[string]string{"reason": contract.WatchNotATerminal})
	}
	keyboard, err := keyboard.EnterKeyboard(in, s.rawOut, entry)
	if err != nil {
		return nil, nil, err
	}
	reader, err := keyboard.NewReader()
	if err != nil {
		keyboard.Leave()
		return nil, nil, err
	}
	return keyboard.Leave, reader, nil
}

// keyPump reads the seam's keys on a goroutine of its own for the whole run,
// across every cycle, so a cycle's reader that has stopped leaves no read in
// flight that would take bytes meant for the next cycle's. Each cycle reads
// the pump through a cycleKeys of its own.
type keyPump struct {
	mu      sync.Mutex
	pending []byte
	ended   bool
	// changed is closed and replaced whenever pending grows or the source
	// ends, which wakes every reader waiting on it.
	changed chan struct{}
}

// startKeyPump starts reading source into a pump.
func startKeyPump(source io.Reader) *keyPump {
	pump := &keyPump{changed: make(chan struct{})}
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := source.Read(buf)
			pump.mu.Lock()
			pump.pending = append(pump.pending, buf[:n]...)
			if err != nil {
				pump.ended = true
			}
			close(pump.changed)
			pump.changed = make(chan struct{})
			pump.mu.Unlock()
			if err != nil {
				return
			}
		}
	}()
	return pump
}

// cycleKeys is one cycle's view of the pump, which answers the end of input
// once its cycle has ended, whatever the pump still holds.
type cycleKeys struct {
	pump *keyPump
	done chan struct{}
	once sync.Once
}

// Read answers what the pump holds, waiting for more, and the end of input
// once the cycle has ended or the source has.
func (c *cycleKeys) Read(p []byte) (int, error) {
	for {
		select {
		case <-c.done:
			return 0, io.EOF
		default:
		}
		c.pump.mu.Lock()
		if len(c.pump.pending) > 0 {
			n := copy(p, c.pump.pending)
			c.pump.pending = c.pump.pending[n:]
			c.pump.mu.Unlock()
			return n, nil
		}
		if c.pump.ended {
			c.pump.mu.Unlock()
			return 0, io.EOF
		}
		changed := c.pump.changed
		c.pump.mu.Unlock()
		select {
		case <-c.done:
			return 0, io.EOF
		case <-changed:
		}
	}
}

// end ends the cycle's view, and is safe to call more than once.
func (c *cycleKeys) end() {
	c.once.Do(func() { close(c.done) })
}

// cycleReader is the seam's reader of one cycle: the stream reader over the
// cycle's view of the pump, whose Stop ends that view before it stops the
// reader, so no read of the stopped cycle is left waiting on the pump.
type cycleReader struct {
	keyboard.Reader
	keys *cycleKeys
}

// Stop ends the cycle's view of the keys and then the reader.
func (r *cycleReader) Stop() {
	r.keys.end()
	r.Reader.Stop()
}

// reportCrash writes what a recovered panic left, after the terminal has been
// restored: the sentence saying so, the panic's value cleaned, and each line
// of the stack captured where it was recovered. It then panics again, so the
// process ends the way Go ends any panic and with Go's own exit status, with
// the value crashPanicValue cleans: the Go runtime prints a panic's value to
// the terminal raw, and a value carrying an escape sequence would otherwise
// reach it. Under test the seam's repanic stands in for that panic and is
// handed the original value, and once it has returned the head answers 0,
// because the status of the panic it stands in for is Go's to give and not
// the head's.
func (s *session) reportCrash(crash *interactiveCrash) int {
	s.errLine(s.r.T("interactive.crashed"))
	s.errLine(withoutControls(fmt.Sprint(crash.value)))
	for _, line := range strings.Split(strings.TrimRight(string(crash.stack), "\n"), "\n") {
		s.errLine(withoutControls(line))
	}
	if interactiveSeam != nil && interactiveSeam.repanic != nil {
		interactiveSeam.repanic(crash.value)
		return 0
	}
	panic(crashPanicValue(crash.value))
}

// crashPanicValue is the value the head panics again with after a crash: an
// error whose text is the original value's, with every control character
// replaced as withoutControls replaces it.
func crashPanicValue(value any) error {
	return errors.New(withoutControls(fmt.Sprint(value)))
}
