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
