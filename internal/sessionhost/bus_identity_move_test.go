package sessionhost

import (
	"errors"
	"testing"
)

func movePair(id string, hasIt bool) (old, next *Conversation) {
	old = NewConversation(Identity{ID: "old", Provider: "a", Cwd: "."}, InitialActivity{Open: true}, false, false)
	next = NewConversation(Identity{ID: "next", Provider: "b", Cwd: "."}, InitialActivity{Open: true}, false, false)
	if hasIt {
		old.JoinBus(id)
	}
	return old, next
}

// The old chat lets go of the identity before the new chat takes it, and the
// settings are written once, with both chats, after the new chat has it.
func TestMovingABusIdentityReleasesBeforeItJoinsAndWritesOnce(t *testing.T) {
	old, next := movePair("window-x", true)
	writes := 0
	c := &Continuations{host: ContinuationHost{MoveBusIdentity: func(from, to, id string) error {
		writes++
		if from != "old" || to != "next" || id != "window-x" {
			t.Fatalf("the write was %s to %s for %s", from, to, id)
		}
		if old.BusID() != "" || next.BusID() != "window-x" {
			t.Fatalf("at the write the old chat has %q and the new chat has %q", old.BusID(), next.BusID())
		}
		return nil
	}}}
	if err := c.moveBusIdentity(old, next, "window-x"); err != nil {
		t.Fatal(err)
	}
	if writes != 1 || old.BusID() != "" || next.BusID() != "window-x" {
		t.Fatalf("%d writes; old %q, new %q", writes, old.BusID(), next.BusID())
	}
}

// A failed settings write puts the identity back with the old chat, and the
// new chat does not keep it.
func TestAFailedBusIdentityWriteGivesTheIdentityBack(t *testing.T) {
	old, next := movePair("window-x", true)
	c := &Continuations{host: ContinuationHost{MoveBusIdentity: func(string, string, string) error { return errors.New("disk full") }}}
	if err := c.moveBusIdentity(old, next, "window-x"); err == nil {
		t.Fatal("a failed write was reported as moved")
	}
	if old.BusID() != "window-x" || next.BusID() != "" {
		t.Fatalf("after the failure the old chat has %q and the new chat has %q", old.BusID(), next.BusID())
	}
}

// A chat that was not joined to the bus in memory is not joined by a failed
// move; only the identity it had comes back.
func TestAFailedBusIdentityWriteDoesNotJoinAChatThatHadNone(t *testing.T) {
	old, next := movePair("window-x", false)
	c := &Continuations{host: ContinuationHost{MoveBusIdentity: func(string, string, string) error { return errors.New("disk full") }}}
	if err := c.moveBusIdentity(old, next, "window-x"); err == nil {
		t.Fatal("a failed write was reported as moved")
	}
	if old.BusID() != "" || next.BusID() != "" {
		t.Fatalf("after the failure the old chat has %q and the new chat has %q", old.BusID(), next.BusID())
	}
}
