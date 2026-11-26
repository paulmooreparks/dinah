package sessionhost

import (
	"testing"
	"time"
)

func TestRuntimeInterestDefersCloseWithoutDeletingConversation(t *testing.T) {
	c := NewConversation(Identity{ID: "invented"}, InitialActivity{Open: true}, false, false)
	now := time.Now()
	c.ClosedDue(false, now, time.Second)
	if !c.ClosedDue(false, now.Add(2*time.Second), time.Second) {
		t.Fatal("fixture was not due")
	}
	release, err := c.HoldRuntime()
	if err != nil {
		t.Fatal(err)
	}
	if c.BeginEnd() != nil {
		t.Fatal("runtime interest allowed native close")
	}
	release()
	release()
	ending := c.BeginEnd()
	if ending == nil {
		t.Fatal("release did not return to normal close policy")
	}
	if _, err := c.HoldRuntime(); err == nil {
		t.Fatal("held an ending conversation")
	}
	c.FinishEnd()
	ending.Complete()
}

func TestObservedSubagentDefersAutomaticClose(t *testing.T) {
	c := NewConversation(Identity{ID: "invented-hook-work"}, InitialActivity{Open: true}, false, false)
	now := time.Now()
	c.ClosedDue(false, now, time.Second)
	if !c.ClosedDue(false, now.Add(2*time.Second), time.Second) {
		t.Fatal("fixture was not due")
	}
	c.ObserveHookSubagents(1)
	if c.BeginEnd() != nil {
		t.Fatal("hook work allowed native close")
	}
	if c.ClosedDue(false, now.Add(time.Hour), time.Second) {
		t.Fatal("hook work allowed close timeout")
	}
	c.ObserveHookSubagents(0)
	if c.ClosedDue(false, now.Add(time.Hour), time.Second) {
		t.Fatal("busy interval did not reset the close clock")
	}
	if !c.ClosedDue(false, now.Add(2*time.Hour), time.Second) || c.BeginEnd() == nil {
		t.Fatal("stopped work did not return to normal close policy")
	}
}
