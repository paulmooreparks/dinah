package main

import (
	"encoding/json"
	"testing"
)

// A line Dinah says in the chat, such as turns set aside in a full window, is
// kept in the saved copy as a status row, once, with the kind it was said as, so
// a chat reopened from its saved copy shows it where it was said and as a
// warning when it was one.
func TestAStatusLineIsKeptInTheSavedCopy(t *testing.T) {
	a, _ := providerTestApp(t)
	s := continuationSource(t, a)
	update := json.RawMessage(`{"sessionUpdate":"dinah_status","kind":"warning","text":"The earliest 3 turns are set aside to make room, and stay in the saved chat."}`)
	a.transcriptCapture().RecordUpdate(s, update)
	a.cmu.Lock()
	tr := s.HistorySavedLocked()
	a.cmu.Unlock()
	found := 0
	for _, m := range tr.Messages {
		if m.Role == "status" && m.Text == "The earliest 3 turns are set aside to make room, and stay in the saved chat." && m.Kind == "warning" {
			found++
		}
	}
	if found != 1 {
		t.Fatalf("the status line is in the saved copy %d times: %+v", found, tr.Messages)
	}
}
