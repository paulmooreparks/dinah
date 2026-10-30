package sessionhost

import (
	"testing"
	"time"
)

// Once Close has begun, no turn starts; a turn that would have run needs a host, so running one here would panic.
func TestNoTurnStartsOnceCloseHasBegun(t *testing.T) {
	var io TurnIO
	if !io.Close(time.Second) {
		t.Fatal("Close with nothing running did not return")
	}
	io.start(nil, nil)
	if !io.Close(time.Second) {
		t.Fatal("a turn was started after Close")
	}
}

// A turn that is still running at the limit is reported, and Close returns.
func TestCloseReturnsAtItsLimitWhenATurnIsStillRunning(t *testing.T) {
	var io TurnIO
	io.running.Add(1)
	defer io.running.Done()
	t0 := time.Now()
	if io.Close(50 * time.Millisecond) {
		t.Fatal("Close said everything ended")
	}
	if time.Since(t0) > 2*time.Second {
		t.Fatal("Close did not return at its limit")
	}
}
