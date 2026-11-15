package broker

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"studio/internal/applet"
)

func TestCancelOwnerStopsPendingEffectsWithoutTouchingOtherOwners(t *testing.T) {
	var calls atomic.Int32
	release := make(chan struct{})
	close(release)
	b := &Broker{Registry: testRegistry(t, &calls, release), State: t.TempDir()}
	entered := make(chan string, 3)
	cancelled := make(chan string, 3)
	consentRelease := make(chan struct{})
	defer close(consentRelease)
	b.Consent = func(ctx context.Context, v Invocation, _ applet.Action) error {
		entered <- v.Workspace + "/" + v.OwnerID
		select {
		case <-ctx.Done():
			cancelled <- v.Workspace + "/" + v.OwnerID
			return ctx.Err()
		case <-consentRelease:
			return nil
		}
	}
	ws, otherWS := t.TempDir(), t.TempDir()
	deadline := time.Now().Add(10 * time.Second)
	one, err := b.Start(ws, "chat", "one", onceRef, `{}`, "", deadline)
	if err != nil {
		t.Fatal(err)
	}
	two, err := b.Start(ws, "chat", "two", onceRef, `{}`, "", deadline)
	if err != nil {
		t.Fatal(err)
	}
	three, err := b.Start(otherWS, "chat", "one", onceRef, `{}`, "", deadline)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		select {
		case <-entered:
		case <-time.After(5 * time.Second):
			t.Fatal("consent did not start")
		}
	}
	b.CancelOwner(ws, "chat", "one")
	select {
	case owner := <-cancelled:
		if owner != ws+"/one" {
			t.Fatal("cancelled another owner", owner)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("revoked owner was not cancelled")
	}
	waitFor(t, b, ws, "one", one.ID, "cancelled")
	if calls.Load() != 0 {
		t.Fatal("cancelled invocation performed its effect")
	}
	for _, v := range []Invocation{two, three} {
		current, err := b.Read(v.Workspace, v.ID)
		if err != nil || current.Status != "running" {
			t.Fatal("another owner's work changed", current, err)
		}
	}
	b.CancelOwner(ws, "chat", "two")
	b.CancelOwner(otherWS, "chat", "one")
	waitFor(t, b, ws, "two", two.ID, "cancelled")
	waitFor(t, b, otherWS, "one", three.ID, "cancelled")
	if calls.Load() != 0 {
		t.Fatal("pending cancelled work reached handler")
	}
}
