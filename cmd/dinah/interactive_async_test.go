//go:build tui

package main

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

// TestAKeyDuringASlowReadIsNotDropped is dinah-636: rereadAsync, the
// board-wide read that follows a change somebody else made, runs as a
// command off the event loop, so a key pressed while one is still in flight
// is not held behind it. readGate gates that read alone, apart from the
// change wait, so the change is detected normally and only the redraw it
// dispatches is held. A key pressed while that redraw cannot complete must
// still move the selection: selectCard applies at once and never waits on
// the read that follows it.
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
		s.write(keyCtrlC)
	})
	m := run.model
	if m == nil {
		t.Fatalf("the run never finished: %q", run.errw)
	}
	wantModel(t, "the focused lane after l pressed while the redraw was held", focusedColumn(m), "Acceptance")
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
