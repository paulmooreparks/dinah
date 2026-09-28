//go:build tui

package main

import (
	"errors"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"dinah/internal/durable"
)

// isWaiting accepts a wait notice delivered to the model.
func isWaiting(msg tea.Msg) bool {
	_, ok := msg.(waitingMsg)
	return ok
}

// TestAWaitIsDrawnInTheStatusLineUntilTheAnswer asserts that a wait notice
// reaches the terminal UI's model as a waitingMsg sent through
// tea.Program.Send, that the message area then draws it in place of the
// status line, and that the answer of the head's read clears it.
func TestAWaitIsDrawnInTheStatusLineUntilTheAnswer(t *testing.T) {
	root := tuiBench(t)
	s, seam := newScript(t, 100, 30, true)
	run := s.run(root, seam, func() {
		s.waitFor("the first size", func(msg tea.Msg) bool {
			_, ok := msg.(tea.WindowSizeMsg)
			return ok
		})
		forwardWaiting(durable.Wait{
			Op:      "append",
			Path:    "journal.ndjson",
			Last:    errors.New("the file is in use"),
			Elapsed: 5 * time.Second,
			Notice:  1,
		})
		s.waitFor("the wait notice", isWaiting)
		s.write(keyCtrlC)
	})
	m := run.model
	if m == nil {
		t.Fatalf("the run never finished: %q", run.errw)
	}
	lines := m.messageLines()
	if len(lines) != 1 || !strings.Contains(lines[0], "waiting for journal.ndjson") {
		t.Fatalf("the message area draws %q, wanted the wait notice", lines)
	}
	m.handleViewRead(viewReadMsg{seq: m.readSeq, err: errors.New("the read answered")})
	if m.waiting != "" {
		t.Errorf("the notice %q outlived the read's answer", m.waiting)
	}
}
