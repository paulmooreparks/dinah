package main

import (
	"bytes"
	"io"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"dinah/internal/bench"
	"dinah/internal/contract"
	"dinah/internal/screen"
	"dinah/internal/verb"
)

// The smallest window --watch runs in: the status line's uncut parts take 34
// columns in English and the first frame's line 38, and six rows leave four
// for the drawing.
const (
	watchMinimumWidth  = 40
	watchMinimumHeight = 6
)

// watchWait is how long one wait for a change lasts before the loop looks at
// the window's size and the clock again.
const watchWait = time.Second

// exitInterrupted is the status a one-shot coloured drawing exits with when
// an interrupt stops it: the status a POSIX shell reports for a command that
// ended on SIGINT, which XCU 2.8.2 requires only to be greater than 128.
const exitInterrupted = 130

// watchClock is how a redraw's time is written, as a 24-hour local time.
const watchClock = "15:04:05"

// terminalSeams stands in for everything a drawing reads from outside the
// process: the terminal, its size, the interrupts and the clock. It is nil in
// a shipped binary, and a test sets it to drive every path of a watch and of
// a coloured drawing without a real terminal. afterRead runs after a frame's
// view has been read and before the frame is written, which is where a test
// changes the workbench under a frame.
type terminalSeams struct {
	screen    func(text io.Writer) screen.Screen
	size      func() (width, height int)
	notify    func(c chan<- os.Signal)
	stop      func(c chan<- os.Signal)
	now       func() time.Time
	afterRead func()
}

// terminalSeam is the seam a test installs; nil means the real terminal.
var terminalSeam *terminalSeams

// terminal opens the terminal layer on the session's raw stdout, handing it
// the session's stdout writer so text still goes through consolewriter.
func (s *session) terminal() screen.Screen {
	if terminalSeam != nil && terminalSeam.screen != nil {
		return terminalSeam.screen(s.out)
	}
	return screen.Open(s.rawOut, s.out)
}

// frameSize reads the window's width and height through the ladder every
// layout reads, unclamped, afresh on every call.
func frameSize() (int, int) {
	if terminalSeam != nil && terminalSeam.size != nil {
		return terminalSeam.size()
	}
	return rawWindowWidth(), windowHeight()
}

// interruptSignals are the signals that end a watch or a coloured drawing:
// os.Interrupt, which is Ctrl+C and, on Windows, Ctrl+Break; SIGTERM, which
// on Windows also carries a closed console, a logoff and a shutdown; and on
// POSIX SIGHUP, a terminal that went away.
func interruptSignals() []os.Signal {
	signals := []os.Signal{os.Interrupt, syscall.SIGTERM}
	if runtime.GOOS != "windows" {
		signals = append(signals, syscall.SIGHUP)
	}
	return signals
}

// notifyInterrupts registers c for the interrupt signals. Until stopInterrupts
// runs, an interrupt is delivered to c and does not end the process, which is
// what os/signal documents for a signal passed to Notify.
func notifyInterrupts(c chan<- os.Signal) {
	if terminalSeam != nil && terminalSeam.notify != nil {
		terminalSeam.notify(c)
		return
	}
	signal.Notify(c, interruptSignals()...)
}

// stopInterrupts gives the interrupt signals back to Go's default handling.
func stopInterrupts(c chan<- os.Signal) {
	if terminalSeam != nil && terminalSeam.stop != nil {
		terminalSeam.stop(c)
		return
	}
	signal.Stop(c)
}

// now is the clock a redraw is stamped with.
func now() time.Time {
	if terminalSeam != nil && terminalSeam.now != nil {
		return terminalSeam.now()
	}
	return time.Now()
}

// colourAllowed reports whether the environment allows colour at all:
// NO_COLOR absent or empty, as https://no-color.org specifies. It turns
// colour off and changes nothing else.
func colourAllowed() bool {
	return os.Getenv("NO_COLOR") == ""
}

// drawOnce writes a drawing once. Without colour it is written as plain lines
// and changes nothing about the terminal, so no signal is registered and Go's
// default handling of Ctrl+C stands. With colour, the interrupt channel is
// registered before the first coloured write and read between lines, and an
// interrupt resets the colour, leaves the line it was on ended, and exits
// with exitInterrupted.
func (s *session) drawOnce(lines []drawnLine) int {
	scr := s.terminal()
	if !colourAllowed() || !scr.Colour() {
		for _, line := range lines {
			s.line(line.text)
		}
		return 0
	}
	interrupts := make(chan os.Signal, 1)
	notifyInterrupts(interrupts)
	for _, line := range lines {
		select {
		case <-interrupts:
			scr.Reset()
			stopInterrupts(interrupts)
			return exitInterrupted
		default:
		}
		if err := scr.Line(line.segments()); err != nil {
			scr.Reset()
			stopInterrupts(interrupts)
			return s.reportError(err)
		}
	}
	stopInterrupts(interrupts)
	return 0
}

// watchCheck answers the refusal a watch meets before it changes anything:
// a stdout that is not a terminal, a window that reports no size or is too
// small, and a terminal whose description lacks what redrawing in place
// needs, in that order. Each carries the view's name and the reason.
func watchCheck(scr screen.Screen, view string) error {
	live, reason, extra := scr.Live()
	if !live && reason == contract.WatchNotATerminal {
		return contract.RefuseWith(contract.WatchUnavailable, view, map[string]string{"reason": contract.WatchNotATerminal})
	}
	width, height := frameSize()
	if width <= 0 || height <= 0 {
		return contract.RefuseWith(contract.WatchUnavailable, view, map[string]string{"reason": contract.WatchNoSize})
	}
	if width < watchMinimumWidth || height < watchMinimumHeight {
		return contract.RefuseWith(contract.WatchUnavailable, view, map[string]string{
			"reason":  contract.WatchTooSmall,
			"size":    strconv.Itoa(width) + "x" + strconv.Itoa(height),
			"minimum": strconv.Itoa(watchMinimumWidth) + "x" + strconv.Itoa(watchMinimumHeight),
		})
	}
	if !live && reason == contract.WatchMissingCapability {
		return contract.RefuseWith(contract.WatchUnavailable, view, map[string]string{"reason": contract.WatchMissingCapability, "capability": extra})
	}
	if !live {
		return contract.RefuseWith(contract.WatchUnavailable, view, map[string]string{"reason": contract.WatchNoTerminalDescription})
	}
	return nil
}

// waited is what one wait for a change answered.
type waited struct {
	set *verb.ChangeSet
	err error
}

// watch draws a view and redraws it in place each time the workbench changes,
// the window's size changes, or a claim the last frame drew expires, until
// an interrupt ends it.
//
// The interrupt channel is registered before the terminal changes and read
// only between rows and while waiting, so a row being written when Ctrl+C
// arrives is finished first. On the first interrupt the loop stops reading
// the channel, restores the terminal, and only then gives the signal back.
// A second interrupt arriving during the restore is delivered to the still
// registered channel, which nothing reads any more, and is dropped there,
// because os/signal documents that it "will not block sending to c"; the
// restore therefore always completes. The wait still running on its
// goroutine is abandoned, and it only reads.
func (s *session) watch(l *verb.Library, req *verb.Request) int {
	scr := s.terminal()
	if err := watchCheck(scr, req.View); err != nil {
		return s.reportError(err)
	}
	glyphs := s.boardGlyphSet(req.ViewPlain)
	colour := colourAllowed() && scr.Colour()
	interrupts := make(chan os.Signal, 1)
	notifyInterrupts(interrupts)
	first, err := l.Changes(&verb.Request{Verb: "changes", Actor: req.Actor})
	if err != nil {
		stopInterrupts(interrupts)
		return s.reportError(err)
	}
	if err := scr.Begin(); err != nil {
		scr.Restore(1)
		stopInterrupts(interrupts)
		return s.reportError(err)
	}
	w := &watcher{s: s, l: l, req: req, scr: scr, glyphs: glyphs, colour: colour, interrupts: interrupts}
	w.status = statusParts{time: s.r.T("view.watch.since", "time", now().Format(watchClock))}
	return w.run(first.Cursor)
}

// statusParts are the three parts of a watch's status line before they are
// joined: when the frame was drawn, what changed, and how to stop.
type statusParts struct {
	time, change string
}

// watcher is one watch in progress.
type watcher struct {
	s          *session
	l          *verb.Library
	req        *verb.Request
	scr        screen.Screen
	glyphs     boardGlyphs
	colour     bool
	interrupts chan os.Signal
	status     statusParts
	// width and height are the size the last frame was drawn at, and
	// lastRow the last row it wrote.
	width, height, lastRow int
	// expiry is the earliest claim expiry among the cards the last frame
	// drew, zero where none of them is held.
	expiry time.Time
}

// run draws frames until an interrupt or an error ends the watch.
func (w *watcher) run(cursor string) int {
	for {
		interrupted, err := w.frame()
		if err != nil {
			return w.leave(err)
		}
		if interrupted {
			return w.leave(nil)
		}
		next, interrupted, err := w.wait(cursor)
		if err != nil {
			return w.leave(err)
		}
		if interrupted {
			return w.leave(nil)
		}
		cursor = next
	}
}

// wait blocks until the next frame is due: a change after cursor, a window
// whose size differs from the last frame's, or the clock passing the
// earliest drawn expiry. It answers the cursor to wait from after the next
// frame.
func (w *watcher) wait(cursor string) (string, bool, error) {
	for {
		answer := make(chan waited, 1)
		go func(since string) {
			set, err := w.l.Changes(&verb.Request{Verb: "changes", Actor: w.req.Actor, Since: since, Wait: true, Timeout: watchWait})
			answer <- waited{set: set, err: err}
		}(cursor)
		var got waited
		select {
		case got = <-answer:
		case <-w.interrupts:
			return cursor, true, nil
		}
		if got.err != nil {
			return cursor, false, got.err
		}
		cursor = got.set.Cursor
		if got.set.Changed {
			w.status = statusParts{
				time:   w.s.r.T("view.watch.updated", "time", now().Format(watchClock)),
				change: w.s.changeText(got.set),
			}
			return cursor, false, nil
		}
		width, height := frameSize()
		if width != w.width || height != w.height {
			if err := w.scr.Clear(); err != nil {
				return cursor, false, err
			}
			w.stamp()
			return cursor, false, nil
		}
		if !w.expiry.IsZero() && !now().Before(w.expiry) {
			w.stamp()
			return cursor, false, nil
		}
	}
}

// stamp moves the status line's time to now and keeps its change, which is
// what a redraw caused by a resize or an expiry shows.
func (w *watcher) stamp() {
	w.status.time = w.s.r.T("view.watch.updated", "time", now().Format(watchClock))
}

// frame reads the view afresh and writes one frame, reading the window's size
// immediately before, and answers whether an interrupt arrived while it was
// written.
func (w *watcher) frame() (bool, error) {
	width, height := frameSize()
	w.width, w.height = width, height
	answer, err := w.l.DrawView(w.req)
	if err != nil {
		return false, err
	}
	if terminalSeam != nil && terminalSeam.afterRead != nil {
		terminalSeam.afterRead()
	}
	if width < watchMinimumWidth || height < watchMinimumHeight {
		notice := w.s.r.T("view.watch.too-small", "size", strconv.Itoa(watchMinimumWidth)+"x"+strconv.Itoa(watchMinimumHeight))
		rows := []drawnLine{{text: cutText(notice, width-1, w.glyphs.ellipsis)}}
		return w.write(rows, nil, height)
	}
	w.expiry = earliestExpiry(answer)
	lines := w.lines(answer, width)
	if len(lines) > height-1 {
		hidden := len(lines) - (height - 2)
		notice := w.s.r.T("view.watch.hidden", "count", strconv.Itoa(hidden))
		lines = append(lines[:height-2:height-2], drawnLine{text: cutText(notice, width-1, w.glyphs.ellipsis)})
	}
	stops := w.s.r.T("view.watch.stops")
	separator := w.s.r.T("view.watch.separator")
	status := drawnLine{text: statusLine(w.status.time, w.status.change, stops, separator, width-1, w.glyphs.ellipsis)}
	return w.write(lines, &status, height)
}

// lines lays the view out for a frame width columns wide. A board is drawn
// at that window. Everything else is what the same command line without
// --watch prints, laid out at one column fewer and cut to fit wherever a
// table's line would still reach past it.
func (w *watcher) lines(answer *verb.ViewAnswer, width int) []drawnLine {
	var lines []drawnLine
	if answer.View.Layout == bench.ViewLayoutColumns && !answer.View.Explained {
		lines = w.s.columnsView(answer, w.l.Bench, w.glyphs, width, w.req.All, w.req.Card != "")
	} else {
		for _, line := range w.s.drawnText(answer, w.l.Bench, w.req, width-1) {
			lines = append(lines, drawnLine{text: cutText(line, width-1, w.glyphs.ellipsis)})
		}
	}
	if !w.colour {
		for i := range lines {
			lines[i].spans = nil
		}
	}
	return lines
}

// write places a frame's rows one at a time, erases every row below the last
// of them, and writes the status line on the window's last row. The
// interrupt channel is read between rows, so a row being written is always
// finished. No row is written with a line feed and none reaches past the
// window, so a frame never scrolls it.
func (w *watcher) write(rows []drawnLine, status *drawnLine, height int) (bool, error) {
	for i, line := range rows {
		select {
		case <-w.interrupts:
			return true, nil
		default:
		}
		if err := w.scr.Row(i+1, line.segments()); err != nil {
			return false, err
		}
		w.lastRow = i + 1
	}
	if len(rows) < height {
		if err := w.scr.EraseBelow(len(rows)); err != nil {
			return false, err
		}
	}
	if status == nil {
		return false, nil
	}
	select {
	case <-w.interrupts:
		return true, nil
	default:
	}
	if err := w.scr.Row(height, status.segments()); err != nil {
		return false, err
	}
	w.lastRow = height
	return false, nil
}

// drawnText is what drawView prints for a view that is not drawn as a board,
// laid out at the given width, as lines. It runs drawView on a copy of the
// session whose stdout is a buffer, so a watch places exactly the lines the
// same command line without --watch prints, --explain and a card argument
// included, and a later change to either drawing reaches both.
func (s *session) drawnText(answer *verb.ViewAnswer, b *bench.Bench, req *verb.Request, width int) []string {
	var buffer bytes.Buffer
	frame := *s
	frame.out = &buffer
	frame.width = width
	frame.drawView(answer, b, req)
	return splitLines(strings.TrimSuffix(buffer.String(), "\n"))
}

// leave restores the terminal and gives the interrupt signals back, in that
// order, then reports the error that ended the watch, if one did.
func (w *watcher) leave(err error) int {
	restoreErr := w.scr.Restore(w.lastRow)
	stopInterrupts(w.interrupts)
	if err != nil {
		return w.s.reportError(err)
	}
	if restoreErr != nil {
		return w.s.reportError(restoreErr)
	}
	return 0
}

// earliestExpiry is the soonest claim expiry among the cards a view drew, or
// the zero time where none of them carries one that parses.
func earliestExpiry(answer *verb.ViewAnswer) time.Time {
	var earliest time.Time
	for _, section := range answer.View.Sections {
		for _, card := range section.Cards {
			if card.State != contract.StateActive || card.Expires == "" {
				continue
			}
			expires, err := time.Parse(time.RFC3339, card.Expires)
			if err != nil {
				continue
			}
			if earliest.IsZero() || expires.Before(earliest) {
				earliest = expires
			}
		}
	}
	return earliest
}

// changeText is what a change reads as in the status line: the last event
// of the answer, in the answer's own order, and how many others came with
// it; where the answer carried no event, the last entity that left, then the
// columns, then the workbench.
func (s *session) changeText(set *verb.ChangeSet) string {
	if len(set.Events) > 0 {
		last := set.Events[len(set.Events)-1]
		text := s.eventText(last)
		if more := len(set.Events) - 1; more > 0 {
			text += s.r.T("view.watch.more", "count", strconv.Itoa(more))
		}
		return withoutControls(text)
	}
	if len(set.Gone) > 0 {
		gone := set.Gone[len(set.Gone)-1]
		ref := gone.Ref
		if ref == "" {
			ref = gone.ID
		}
		return withoutControls(s.r.T("view.watch.gone", "ref", ref, "fate", s.token(gone.Fate)))
	}
	if len(set.Columns) > 0 {
		return s.r.T("view.watch.columns-changed")
	}
	return s.r.T("view.watch.workbench-changed")
}

// eventText is one event as the status line names it, from the pieces the
// changes table draws: the subject, the action, the detail and the actor.
// An expired claim names the holder it lapsed from rather than reading as
// though that holder caused the expiry.
func (s *session) eventText(ev verb.ChangeEvent) string {
	subject := changeSubject(ev)
	action := s.token(ev.Event.Event)
	if ev.Event.Event == contract.EventExpired {
		return s.r.T("view.watch.change.expired", "subject", subject, "action", action, "actor", ev.Actor.Name)
	}
	detail := s.eventDetail(ev.Event)
	if detail == "" {
		return s.r.T("view.watch.change.bare", "subject", subject, "action", action, "actor", ev.Actor.Name)
	}
	return s.r.T("view.watch.change", "subject", subject, "action", action, "detail", detail, "actor", ev.Actor.Name)
}
