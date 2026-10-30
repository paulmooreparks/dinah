package main

import "testing"

// The saved copy of a chat that is live in the engine comes with the
// generation of its event log and the number it is consistent with. After a
// turn the copy holds the turn's end and the number is the log's head, which
// includes the turn's event; a chat the engine does not have comes without
// either.
func TestTheSavedCopyOfALiveChatSaysWhichLogEntryItHolds(t *testing.T) {
	a, _, s, me, turns := busChat(t)
	before := a.SavedConversation(s.ID())
	if before.StreamGen == "" || before.StreamGen != s.InfoVersioned().StreamGen {
		t.Fatalf("a live chat's saved copy has the log %q, not %q", before.StreamGen, s.InfoVersioned().StreamGen)
	}
	sendTo(t, a, me, "Is the parser release ready?")
	ring(t, a)
	waitTurn(t, turns)
	after := a.SavedConversation(s.ID())
	head := s.Info().StreamHead
	if after.StreamGen != before.StreamGen || after.StreamThrough == 0 || after.StreamThrough != head {
		t.Fatalf("after a turn the copy is consistent with %d of a log whose head is %d (%s)", after.StreamThrough, head, after.StreamGen)
	}
	if gone := a.SavedConversation("no-such-chat"); gone.StreamGen != "" || gone.StreamThrough != 0 {
		t.Fatalf("a chat the engine does not have: %+v", gone)
	}
}
