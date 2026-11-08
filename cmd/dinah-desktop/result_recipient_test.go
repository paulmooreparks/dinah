package main

import "testing"

// An answer to a chat that was continued, twice here as PUDL Team Lead was,
// goes to the bus identity its last continuation took over with it; a chat
// with its own identity keeps it, and a continuation that was only prepared,
// or whose identity moved elsewhere, gives none (9 November 2026).
func TestAnAnswerToAContinuedChatGoesToTheIdentityItsContinuationTookOver(t *testing.T) {
	s := &Settings{
		BusSessions:  map[string]string{"third": "window-team-lead", "own": "window-own"},
		ChatBindings: map[string]string{"first": "second", "second": "third"},
		ChatTransfers: map[string]chatTransfer{
			"first":  {Source: "first", Target: "second", Recipient: "window-team-lead", Stage: "committed"},
			"second": {Source: "second", Target: "third", Recipient: "window-team-lead", Stage: "committed"},
			"half":   {Source: "half", Target: "other", Recipient: "window-other", Stage: "prepared"},
		},
	}
	for chat, want := range map[string]string{"first": "window-team-lead", "second": "window-team-lead", "own": "window-own", "half": "", "never": ""} {
		if got := resultRecipient(s, chat); got != want {
			t.Errorf("an answer to %s goes to %q, not %q", chat, got, want)
		}
	}
	// The identity moved on to a chat outside the chain: the answer has no
	// recipient rather than reaching that chat.
	s.BusSessions = map[string]string{"elsewhere": "window-team-lead"}
	if got := resultRecipient(s, "first"); got != "" {
		t.Errorf("an answer whose identity moved outside the chain goes to %q", got)
	}
}
