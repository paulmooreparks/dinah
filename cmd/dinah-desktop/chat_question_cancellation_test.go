package main

import (
	"context"
	"testing"
	"time"
)

// Once postQuestion registers the committed ID, cancellation must return
// through askOverBus's normal deferred withdrawal and clear both owners.
func TestCancelledRegisteredQuestionIsWithdrawn(t *testing.T) {
	a, ws, _, _, _ := closedAskApp(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := askInBackground(t, ctx, a, ws, "chat-A", "codex:closed", "Cancel the registered question")
	var id int64
	for deadline := time.Now().Add(5 * time.Second); id == 0; {
		for _, pending := range waitingQuestions(t, a) {
			if a.busHost().HasQuestion(pending) {
				id = pending
				break
			}
		}
		if id != 0 {
			cancel()
		}
		if id == 0 {
			if time.Now().After(deadline) {
				t.Fatal("the committed question was never registered")
			}
			time.Sleep(time.Millisecond)
		}
	}
	if result := answer(t, done); result.err == nil {
		t.Fatal("cancelled question returned an answer")
	}
	if a.busHost().HasQuestion(id) {
		t.Fatalf("question %d remains registered after deferred withdrawal", id)
	}
	if ids := waitingQuestions(t, a); len(ids) != 0 {
		t.Fatalf("deferred withdrawal left questions %v", ids)
	}
}
