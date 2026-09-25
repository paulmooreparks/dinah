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
	command        func()                         // called at the top of every command the head returns
	discard        func()                         // called by the reader on every flush it makes
	finish         func(*interactiveModel, error) // called with the final model and Run's error
	repanic        func(any)                      // replaces the final panic of a crash report
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

// runTUI starts the terminal head over one view, the board where none is
// named.
//
// A machine format is refused before the workbench is opened. The view is
// then read once, and a refusal is reported exactly as dinah view reports
// it, so nothing reaches the terminal before that read has answered. Only
// then is the terminal checked, and only then does the interface start.
func runTUI(s *session, parsed *arguments) int {
	req := s.request("view", parsed)
	req.View = at(parsed.rest(), 0)
	if req.View == "" {
		req.View = interactiveDefaultView
	}
	req.ViewPlain = parsed.has("plain")
	req.Lang = s.r.Tag
	if s.format != formatHuman {
		// The detail names the command and the flag it refuses, as dinah
		// view names --watch --json.
		detail := strings.Join([]string{s.command, "--json"}, " ")
		return s.reportError(contract.Refuse(contract.Malformed, detail))
	}
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
	if missing := screen.MissingArrow(entry); missing != "" {
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

// interactiveSink hands what the key reader decodes to the program.
type interactiveSink struct {
	program *tea.Program
}

// Event sends a key or a paste, stamped with its generation.
func (k interactiveSink) Event(event screen.Event) {
	if event.Paste {
		k.program.Send(pasteMsg{content: event.Text, gen: event.Gen})
		return
	}
	k.program.Send(keyMsg{key: interactiveKeyMsg(event.Key), gen: event.Gen})
}

// Resized sends the notice that the console's buffer size changed.
func (k interactiveSink) Resized() {
	k.program.Send(resizeMsg{})
}

// Flushed sends the confirmation of a flush.
func (k interactiveSink) Flushed(gen uint64) {
	k.program.Send(flushedMsg{gen: gen})
}

// interactive runs the terminal head over the view first answered, until it
// is quit, and answers the exit code.
//
// The input side, the bracketed-paste disable included, is restored in a
// function deferred before Run is called, so it runs on every path out of
// this function, a panic in the head's own code included. The same function
// is called as soon as Run returns, so the crash report of a recovered panic
// is written to a terminal already given back.
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
	program := tea.NewProgram(model, s.interactiveOptions(width, height)...)
	model.program = program
	leave, reader, err := s.enterKeyboard(entry)
	if err != nil {
		return s.reportError(err)
	}
	model.reader = reader
	readerDone := make(chan struct{})
	var restoreOnce sync.Once
	restore := func() {
		restoreOnce.Do(func() {
			reader.Stop()
			<-readerDone
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
		reader.Run(interactiveSink{program: program})
	}()
	if interactiveSeam != nil && interactiveSeam.program != nil {
		interactiveSeam.program(program)
	}
	final, runErr := program.Run()
	restore()
	model.gate.close()
	if interactiveSeam != nil && interactiveSeam.finish != nil {
		finished, _ := final.(*interactiveModel)
		interactiveSeam.finish(finished, runErr)
	}
	if crash := model.crashed(); crash != nil {
		return s.reportCrash(crash)
	}
	switch {
	case errors.Is(runErr, tea.ErrInterrupted):
		return exitInterrupted
	case runErr != nil:
		return s.reportError(runErr)
	}
	return 0
}

// interactiveOptions are the program's options: input disabled because keys
// come from Dinah's reader, the terminal or the seam's writer as output, the
// ANSI colour profile, or the one with no colour at all under NO_COLOR, and
// the environment interactiveEnviron answers.
// Under the seam the program is also given the seam's size and ignores
// signals.
func (s *session) interactiveOptions(width, height int) []tea.ProgramOption {
	profile := colorprofile.ANSI
	if !colourAllowed() {
		profile = colorprofile.Ascii
	}
	environ := tea.WithEnvironment(interactiveEnviron(os.Environ(), runtime.GOOS))
	if seam := interactiveSeam; seam != nil {
		return []tea.ProgramOption{
			tea.WithInput(nil),
			tea.WithOutput(seam.output),
			tea.WithColorProfile(profile),
			environ,
			tea.WithWindowSize(width, height),
			tea.WithoutSignals(),
		}
	}
	return []tea.ProgramOption{
		tea.WithInput(nil),
		tea.WithOutput(s.rawOut),
		tea.WithColorProfile(profile),
		environ,
	}
}

// interactiveEnviron is the environment Bubble Tea reads: the process's own,
// less TERM on Windows. Bubble Tea's renderer chooses which cursor and
// repeat sequences to write from TERM, and for terminals such as kitty,
// alacritty, wezterm and tmux it writes REP and HPA, which Microsoft's
// "Console Virtual Terminal Sequences" page does not list. A Windows console
// never needs TERM, and without it the renderer writes only sequences that
// page documents, so the head relies on nothing undocumented beyond the two
// corners the operator accepted.
func interactiveEnviron(environ []string, goos string) []string {
	if goos != "windows" {
		return environ
	}
	kept := make([]string, 0, len(environ))
	for _, variable := range environ {
		name, _, _ := strings.Cut(variable, "=")
		if strings.EqualFold(name, "TERM") {
			continue
		}
		kept = append(kept, variable)
	}
	return kept
}

// enterKeyboard takes the keyboard and answers how to give it back and the
// reader to take keys from. Under the seam it writes the bracketed-paste
// enable to the seam's output and changes no mode.
func (s *session) enterKeyboard(entry *screen.Terminfo) (func() error, screen.Reader, error) {
	if seam := interactiveSeam; seam != nil {
		out := seam.output
		if _, err := io.WriteString(out, screen.BracketedPasteOn); err != nil {
			return nil, nil, err
		}
		keys := seam.keys
		if keys == nil {
			keys = strings.NewReader("")
		}
		leave := func() error {
			_, err := io.WriteString(out, screen.BracketedPasteOff)
			return err
		}
		return leave, screen.NewStreamReader(keys, seam.terminfo, seam.discard), nil
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

// reportCrash writes what a recovered panic left, after the terminal has been
// restored: the sentence saying so, the panic's value cleaned, and each line
// of the stack captured where it was recovered. It then panics again with the
// original value, so the process ends the way Go ends any panic and with
// Go's own exit status. Under test the seam's repanic stands in for that
// panic, and once it has returned the head answers 0, because the status of
// the panic it stands in for is Go's to give and not the head's.
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
	panic(crash.value)
}
