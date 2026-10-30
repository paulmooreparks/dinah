package broker

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"studio/internal/applet"
)

const pureRef, onceRef, readRef = "test.slow/pure@1", "test.slow/once@1", "test.slow/read@1"

// testRegistry has a pure action and a once action that each wait for
// release before returning their input.
func testRegistry(t *testing.T, calls *atomic.Int32, release chan struct{}) *applet.Registry {
	t.Helper()
	schema := json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{"value":{"type":"integer"}}}`)
	handler := func(ctx context.Context, input string) (applet.Result, error) {
		calls.Add(1)
		select {
		case <-release:
		case <-ctx.Done():
			return applet.Result{}, ctx.Err()
		}
		return applet.Result{OutputJSON: input}, nil
	}
	r, err := applet.New(applet.Bundle{
		Manifest: applet.Manifest{ID: "test.slow", Version: "1.0.0", APIVersion: 1, Title: "Slow", Actions: []applet.Action{
			{Ref: pureRef, Title: "Pure", Replay: "pure", Capabilities: []string{}, InputSchema: schema, OutputSchema: schema},
			{Ref: onceRef, Title: "Once", Replay: "once", Capabilities: []string{}, InputSchema: schema, OutputSchema: schema},
			{Ref: readRef, Title: "Read", Replay: "once", Effect: "read", Capabilities: []string{}, InputSchema: schema, OutputSchema: schema},
		}},
		Handlers: map[string]applet.Handler{pureRef: handler, onceRef: handler, readRef: handler},
		Sources:  map[string][]byte{"test": []byte("test")},
	})
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func waitFor(t *testing.T, b *Broker, ws, owner, id, status string) Invocation {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		v, err := b.Get(ws, "chat", owner, id)
		if err != nil {
			t.Fatal(err)
		}
		if v.Status == status {
			return v
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("invocation %s did not become %s", id, status)
	return Invocation{}
}

// Two programs share the invocations; the one not executing an invocation
// neither runs it again nor fails it nor cancels it, and sees its result.
func TestInvocationExecutedElsewhere(t *testing.T) {
	state, ws := t.TempDir(), t.TempDir()
	var calls atomic.Int32
	release := make(chan struct{})
	r := testRegistry(t, &calls, release)
	carrier := &Broker{Registry: r, State: state}
	other := &Broker{Registry: r, State: state}
	for _, ref := range []string{pureRef, onceRef} {
		v, err := carrier.Start(ws, "chat", "run", ref, `{"value":1}`, "", time.Now().Add(time.Minute))
		if err != nil {
			t.Fatal(err)
		}
		for calls.Load() == 0 {
			time.Sleep(5 * time.Millisecond)
		}
		seen, err := other.Get(ws, "chat", "run", v.ID)
		if err != nil || seen.Status != "running" {
			t.Fatalf("%s: the other program saw %q %v", ref, seen.Status, err)
		}
		if _, err := other.StartID(ws, "chat", "run", v.ID, ref, `{"value":1}`, "", v.Deadline); err != nil {
			t.Fatal(err)
		}
		if err := other.Cancel(ws, "chat", "run", v.ID); !errors.Is(err, ErrElsewhere) {
			t.Fatalf("%s: another program's invocation was cancelled: %v", ref, err)
		}
		release <- struct{}{}
		if done := waitFor(t, other, ws, "run", v.ID, "done"); done.OutputJSON != `{"value":1}` {
			t.Fatalf("%s: output %s", ref, done.OutputJSON)
		}
		if calls.Load() != 1 || other.Executing() != 0 {
			t.Fatalf("%s: the action ran %d times", ref, calls.Load())
		}
		calls.Store(0)
	}
}

// An invocation a stopped program left running is run again when it is
// pure, and failed when it may have had effects.
func TestInvocationLeftRunning(t *testing.T) {
	state, ws := t.TempDir(), t.TempDir()
	var calls atomic.Int32
	release := make(chan struct{})
	close(release)
	b := &Broker{Registry: testRegistry(t, &calls, release), State: state}
	for _, ref := range []string{pureRef, onceRef} {
		digest, _ := b.Registry.Digest(ref)
		v := Invocation{ID: "left-" + strings.TrimSuffix(strings.TrimPrefix(ref, "test.slow/"), "@1"), Workspace: ws, OwnerKind: "chat", OwnerID: "run", Action: ref, Digest: digest, InputJSON: `{"value":2}`, Status: "running", Deadline: time.Now().Add(time.Minute), Created: time.Now()}
		if err := b.Save(&v); err != nil {
			t.Fatal(err)
		}
		if ref == pureRef {
			waitFor(t, b, ws, "run", v.ID, "done")
		} else if got := waitFor(t, b, ws, "run", v.ID, "failed"); !strings.Contains(got.Error, "stopped before") {
			t.Fatalf("error %q", got.Error)
		}
	}
	if calls.Load() != 1 {
		t.Fatalf("the actions ran %d times", calls.Load())
	}
}

func TestUndeclaredEffectsAreRefused(t *testing.T) {
	var calls atomic.Int32
	b := &Broker{Registry: testRegistry(t, &calls, make(chan struct{})), State: t.TempDir()}
	v := Invocation{Action: pureRef}
	if err := b.prepare(&v, applet.Result{OutputJSON: `{}`, Request: &applet.Request{Title: "Illegal"}}); err == nil {
		t.Fatal("an undeclared human request was allowed")
	}
	if err := b.prepare(&v, applet.Result{OutputJSON: `{}`, Artifacts: []applet.Artifact{{Key: "x", Name: "x.txt", MIME: "text/plain"}}}); err == nil {
		t.Fatal("an undeclared artifact was allowed")
	}
}

// An action's readable text is kept with its saved invocation beside its
// output, read back by another program, and dropped with the output when
// the result is refused.
func TestResultTextIsSavedWithTheInvocation(t *testing.T) {
	const ref = "test.text/say@1"
	schema := json.RawMessage(`{"type":"object"}`)
	text := "The reply, as written:\n\n- one\n- two"
	r, err := applet.New(applet.Bundle{
		Manifest: applet.Manifest{ID: "test.text", Version: "1.0.0", APIVersion: 1, Title: "Text", Actions: []applet.Action{{Ref: ref, Title: "Say", Replay: "pure", Capabilities: []string{}, InputSchema: schema, OutputSchema: schema}}},
		Handlers: map[string]applet.Handler{ref: func(ctx context.Context, input string) (applet.Result, error) {
			return applet.Result{OutputJSON: `{"said":true}`, Text: text}, nil
		}},
		Sources: map[string][]byte{"test": []byte("test")},
	})
	if err != nil {
		t.Fatal(err)
	}
	state, ws := t.TempDir(), t.TempDir()
	b := &Broker{Registry: r, State: state}
	v, err := b.Start(ws, "chat", "run", ref, `{}`, "", time.Now().Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if done := waitFor(t, b, ws, "run", v.ID, "done"); done.Text != text || done.OutputJSON != `{"said":true}` {
		t.Fatalf("the result lost its text: %+v", done)
	}
	if saved, err := (&Broker{Registry: r, State: state}).Read(ws, v.ID); err != nil || saved.Text != text {
		t.Fatalf("the saved invocation lost its text: %q %v", saved.Text, err)
	}
	refused := Invocation{Action: ref}
	if err := b.prepare(&refused, applet.Result{OutputJSON: `{}`, Text: "\xff"}); err == nil {
		t.Fatal("text that is not UTF-8 was kept")
	}
}

func TestNonResumableExecutingFollowsReplayContract(t *testing.T) {
	release := make(chan struct{})
	calls := &atomic.Int32{}
	b := &Broker{Registry: testRegistry(t, calls, release), State: t.TempDir()}
	defer b.Close()
	ws := t.TempDir()
	first, err := b.Start(ws, "chat", "owner", pureRef, `{}`, "", time.Now().Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if b.NonResumableExecuting() != 0 {
		t.Fatal("pure action was treated as nonresumable")
	}
	// A read, such as the Git pane's wait for a change, changes nothing, so
	// it does not hold exit even though it is not safe to repeat.
	read, err := b.Start(ws, "chat", "owner", readRef, `{}`, "", time.Now().Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if b.NonResumableExecuting() != 0 {
		t.Fatal("a read action was counted as unfinished work")
	}
	second, err := b.Start(ws, "chat", "owner", onceRef, `{}`, "", time.Now().Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if b.NonResumableExecuting() != 1 {
		t.Fatal("once action was not counted")
	}
	close(release)
	waitFor(t, b, ws, "owner", first.ID, "done")
	waitFor(t, b, ws, "owner", second.ID, "done")
	waitFor(t, b, ws, "owner", read.ID, "done")
	if b.NonResumableExecuting() != 0 {
		t.Fatal("finished action remained active")
	}
}
