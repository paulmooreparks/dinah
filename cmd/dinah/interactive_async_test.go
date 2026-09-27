//go:build tui

package main

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

// TestAKeyDuringASlowReadIsNotDropped is dinah-636: the board-wide read that
// follows a change somebody else made runs as a command off the event loop,
// so a key pressed while one is still in flight is queued rather than held
// behind it or dropped, and replayed once the read lands. readGate gates that
// read alone, apart from the change wait, so the change is detected normally
// and only the redraw it dispatches is held.
func TestAKeyDuringASlowReadIsNotDropped(t *testing.T) {
	root := tuiBench(t)
	s, seam := newScript(t, 100, 30, true)
	hold := make(chan struct{})
	seam.readGate = func() { <-hold }
	s.when("Z", func() { step(t, root, "move", "fx-1", "acceptance") })
	run := s.run(root, seam, func() {
		s.write("Z")
		s.waitFor("the change", isChange)
		s.write("l")
		s.waitFor("l", isKey("l"))
		close(hold)
		s.waitFor("the redraw the change caused", isViewRead)
		s.write(keyCtrlC)
	})
	m := run.model
	if m == nil {
		t.Fatalf("the run never finished: %q", run.errw)
	}
	wantModel(t, "the focused lane after the queued l was replayed", focusedColumn(m), "Acceptance")
}

// TestAKeyAfterAnActIsQueuedAndReplayed is dinah-636: an act's own redraw
// runs off the event loop exactly as the watcher's does, so a key pressed
// while it is in flight is queued rather than dropped or, as it once was,
// processed synchronously against the state the act is about to replace.
// readGate holds the redraw t's claim dispatches; j, pressed while it is
// held, must still move the selection once the redraw lands and replays it.
func TestAKeyAfterAnActIsQueuedAndReplayed(t *testing.T) {
	root := tuiBench(t)
	s, seam := newScript(t, 100, 30, true)
	hold := make(chan struct{})
	seam.readGate = func() { <-hold }
	run := s.run(root, seam, func() {
		s.write("t")
		s.waitFor("t", isKey("t"))
		s.write("j")
		s.waitFor("j", isKey("j"))
		close(hold)
		s.waitFor("t's own read landing", isViewRead)
		s.write(keyCtrlC)
	})
	m := run.model
	if m == nil {
		t.Fatalf("the run never finished: %q", run.errw)
	}
	wantModel(t, "the selection after the queued j was replayed", selectedRef(m), "fx-2")
}

// TestAnActProducesOneReadNotTwo is dinah-636: after a successful act, the
// wait for a change already in flight when the act was made, launched by
// Init before any act could happen, answers the very change the act itself
// made. That answer's watch epoch is the one Init launched it under, which
// the act's own success has since moved past, so it must not dispatch a
// second, asynchronous read of the view on top of the act's own: the head's
// own read generation, readSeq, must still read exactly 1, the one the act's
// own reread bumped, once the watcher's report has been fully handled.
func TestAnActProducesOneReadNotTwo(t *testing.T) {
	root := tuiBench(t)
	s, seam := newScript(t, 100, 30, true)
	var readSeq uint64
	seam.observe = func(m *interactiveModel, _ tea.Msg) { readSeq = m.readSeq }
	run := s.run(root, seam, func() {
		s.write("t")
		s.waitFor("t", isKey("t"))
		s.waitFor("the watcher's report of the same change", isChange)
		s.write(keyCtrlC)
	})
	m := run.model
	if m == nil {
		t.Fatalf("the run never finished: %q", run.errw)
	}
	if readSeq != 1 {
		t.Errorf("the read generation after one act and the watcher's report of it is %d, wanted exactly 1: the watcher's stale report of the act's own change dispatched a second read", readSeq)
	}
}

// countingViewReads accepts once at least n viewReadMsg values have been
// seen, counting into the pointer given so a later assertion can read it too.
func countingViewReads(count *int, n int) func(tea.Msg) bool {
	return func(msg tea.Msg) bool {
		if _, ok := msg.(viewReadMsg); ok {
			*count++
		}
		return *count >= n
	}
}

// TestAnOutsideChangeInTheSameBatchAsTheActIsStillDrawn is dinah-636: a wait
// launched before the head's own act can answer a batch that carries more
// than an echo of that act, since Changes reports everything since the old
// cursor in one shot. Holding every command from the start means the act's
// own write and a second actor's write both land before the watcher's stale
// wait ever runs, so both arrive in the one batch that wait answers. That
// batch must still be drawn: batchIsOwnAct must find the second actor's event
// and refuse to treat the batch as nothing but the act's own echo.
func TestAnOutsideChangeInTheSameBatchAsTheActIsStillDrawn(t *testing.T) {
	root := tuiBench(t)
	s, seam := newScript(t, 100, 30, true)
	hold := make(chan struct{})
	seam.command = func() { <-hold }
	var readSeq uint64
	seam.observe = func(m *interactiveModel, _ tea.Msg) { readSeq = m.readSeq }
	count := 0
	run := s.run(root, seam, func() {
		s.write("t")
		s.waitFor("t", isKey("t"))
		step(t, root, "comment", "fx-2", "from someone else", "--actor", "bram")
		close(hold)
		s.waitFor("both the act's own read and the batch's", countingViewReads(&count, 2))
		s.write(keyCtrlC)
	})
	m := run.model
	if m == nil {
		t.Fatalf("the run never finished: %q", run.errw)
	}
	if readSeq != 2 {
		t.Errorf("the read generation after an act whose watch batch also carries another actor's change is %d, wanted 2: the outside change must still be drawn rather than swallowed as an echo of the act", readSeq)
	}
}
