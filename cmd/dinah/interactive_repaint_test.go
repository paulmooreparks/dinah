//go:build tui

package main

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

// clearSequence is what Bubble Tea's renderer writes to clear the screen
// before a full repaint: cursor home, then erase the whole display.
const clearSequence = "\x1b[H\x1b[2J"

// fullRepaint reports why output, read from its start, is not a full
// repaint of rows: a clear, followed by the text of every row in order. A
// row is matched by its words, since the renderer may move the cursor over
// a run of blanks instead of writing them, and rows may carry styling, which
// is set aside. It answers the empty string when
// output is one.
func fullRepaint(output string, rows []string) string {
	at := strings.Index(output, clearSequence)
	if at < 0 {
		return "no clear was written"
	}
	text := visible(output[at:])
	for i, row := range rows {
		for _, word := range strings.Fields(visible(row)) {
			found := strings.Index(text, word)
			if found < 0 {
				return fmt.Sprintf("row %d's word %q does not follow the clear in order", i+1, word)
			}
			text = text[found+len(word):]
		}
	}
	return ""
}

// waitForOutput waits until the buffer holds text, failing after a generous
// guard. The wait is on the buffer's content: the guard only stops a run
// that never draws.
func waitForOutput(t *testing.T, buffer *lockedBuffer, text string) {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for !strings.Contains(visible(buffer.String()), text) {
		if time.Now().After(deadline) {
			t.Fatalf("the output never held %q; it holds %q", text, buffer.String())
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// settledLength answers the output's length once the renderer has stopped
// writing to it, which the tests take as the mark after which a repaint is
// looked for. It waits for the length to hold across several reads, and the
// read that answers is on the content; the pause between reads only paces
// them.
func settledLength(output func() string) int {
	last, steady := -1, 0
	for steady < 5 {
		time.Sleep(20 * time.Millisecond)
		if now := len(output()); now == last {
			steady++
		} else {
			last, steady = now, 0
		}
	}
	return last
}

// waitForClear waits until the output after mark holds a clear, failing
// after a guard that only stops a run that never writes one.
func waitForClear(t *testing.T, output func() string, mark int) bool {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for !strings.Contains(output()[mark:], clearSequence) {
		if time.Now().After(deadline) {
			t.Errorf("no clear was written after the mark; the output after it is %q", output()[mark:])
			return false
		}
		time.Sleep(5 * time.Millisecond)
	}
	return true
}

// isCtrlL accepts the key message of ctrl+l.
func isCtrlL(msg tea.Msg) bool {
	k, ok := msg.(keyMsg)
	return ok && k.key.String() == "ctrl+l"
}

// TestCtrlLRepaintsTheWholeScreen is dinah-603/criteria/50. With the board
// drawn and the output settled, ctrl+l writes a clear followed by every row
// of the screen, and performs no act: fx-1's anchor and journal are
// byte-identical, and the mode and the selection are those of a run that
// never pressed it. The full help lists ctrl+l through the catalogs.
func TestCtrlLRepaintsTheWholeScreen(t *testing.T) {
	root := tuiBench(t)
	anchor, journal := anchorText(t, root, "fx-1"), journalText(t, root, "fx-1")
	s, seam := newScript(t, 100, 30, true)
	var output lockedBuffer
	seam.output = &output
	mark := -1
	run := s.run(root, seam, func() {
		waitForOutput(t, &output, "fx-1")
		mark = settledLength(output.String)
		s.write("\x0c")
		if s.waitFor("ctrl+l", isCtrlL) != nil {
			waitForClear(t, output.String, mark)
		}
		s.write(keyCtrlC)
	})
	if run.model == nil || mark < 0 {
		t.Fatalf("the run never finished: %q", run.errw)
	}
	rows := strings.Split(run.model.content(), "\n")
	if why := fullRepaint(output.String()[mark:], rows); why != "" {
		t.Errorf("ctrl+l did not repaint the whole screen: %s", why)
	}
	still := runTUIThrough(t, tuiBench(t), tuiSeam(t, strings.NewReader(keyCtrlC), 100, 30))
	wantModel(t, "the mode after ctrl+l", run.model.mode, still.model.mode)
	wantModel(t, "the selection after ctrl+l", selectedRef(run.model), selectedRef(still.model))
	if anchorText(t, root, "fx-1") != anchor || journalText(t, root, "fx-1") != journal {
		t.Error("ctrl+l acted on fx-1")
	}
	r := run.model.s.r
	var listed []string
	for _, column := range newInteractiveKeys(r, "→", "Done", "Implement").fullHelp(false, interactiveOffer{}) {
		for _, binding := range column {
			listed = append(listed, binding.Help().Key+" "+binding.Help().Desc)
		}
	}
	if want := r.T("interactive.key.ctrl-l") + " " + r.T("interactive.help.repaint"); !strings.Contains(strings.Join(listed, "|"), want) {
		t.Errorf("the full help does not list %q: %q", want, listed)
	}
}
