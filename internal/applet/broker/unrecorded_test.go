package broker

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"studio/internal/applet"
)

// observed collects what a broker tells the page and the log.
type observed struct {
	mu     sync.Mutex
	events []string
	logs   []string
}

func (o *observed) broker(t *testing.T, calls *atomic.Int32, release chan struct{}) *Broker {
	t.Helper()
	return &Broker{Registry: testRegistry(t, calls, release), State: t.TempDir(),
		Emit: func(name string, _ any) { o.mu.Lock(); o.events = append(o.events, name); o.mu.Unlock() },
		Logf: func(source, level, format string, _ ...any) {
			o.mu.Lock()
			o.logs = append(o.logs, level+" "+source+" "+format)
			o.mu.Unlock()
		}}
}

func (o *observed) counts() (events, logs int) {
	o.mu.Lock()
	defer o.mu.Unlock()
	return len(o.events), len(o.logs)
}

func runOnce(t *testing.T, b *Broker, ws, ownerKind, owner, ref string) (string, string) {
	t.Helper()
	id, err := NewID()
	if err != nil {
		t.Fatal(err)
	}
	out, err := b.Run(context.Background(), ws, ownerKind, owner, id, ref, `{"value":1}`, "", time.Now().Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	return id, out
}

func released() chan struct{} {
	c := make(chan struct{})
	close(c)
	return c
}

// A pane's read, an extension's own pane asking for an action declared to
// change nothing, gives its answer and leaves no saved record, no line in the
// log above debug and no event for the page.
func TestAPanesReadLeavesNoRecordLogLineOrEvent(t *testing.T) {
	var o observed
	b := o.broker(t, &atomic.Int32{}, released())
	ws := t.TempDir()
	id, out := runOnce(t, b, ws, "applet", "extension:test.pane", readRef)
	if out != `{"value":1}` {
		t.Fatalf("the answer was %q", out)
	}
	if _, err := b.Read(ws, id); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("a record was saved: %v", err)
	}
	if saved, err := b.Saved(ws); err != nil || len(saved) != 0 {
		t.Fatalf("saved invocations: %v %v", saved, err)
	}
	events, logs := o.counts()
	if events != 0 {
		t.Fatalf("the page was told of %d invocation events: %v", events, o.events)
	}
	if logs == 0 {
		t.Fatal("the run was not logged at all")
	}
	for _, l := range o.logs {
		if !strings.HasPrefix(l, "debug ") {
			t.Fatalf("a run that is not recorded logged above debug: %q", l)
		}
	}
	if b.Executing() != 0 {
		t.Fatal("the run was left active")
	}
}

// A chat's read keeps its record, with its event and its log line, since the
// chat polls the invocation by its ID.
func TestAChatsReadKeepsItsRecord(t *testing.T) {
	var o observed
	b := o.broker(t, &atomic.Int32{}, released())
	ws := t.TempDir()
	id, _ := runOnce(t, b, ws, "chat", "chat-1", readRef)
	v, err := b.Read(ws, id)
	if err != nil || v.Status != "done" {
		t.Fatalf("a chat's read has no finished record: %+v %v", v, err)
	}
	if events, _ := o.counts(); events == 0 {
		t.Fatal("the page was not told of a chat's invocation")
	}
}

// A pane's invocation of an action that is not declared a read keeps its record.
func TestAPanesInvocationOfAnActionThatIsNotAReadKeepsItsRecord(t *testing.T) {
	var o observed
	b := o.broker(t, &atomic.Int32{}, released())
	ws := t.TempDir()
	for _, ref := range []string{onceRef, pureRef} {
		id, _ := runOnce(t, b, ws, "applet", "extension:test.pane", ref)
		if v, err := b.Read(ws, id); err != nil || v.Status != "done" {
			t.Fatalf("%s left no finished record: %+v %v", ref, v, err)
		}
	}
}

// A read that can make an artifact or ask the person needs its record, and an
// invocation from something other than an extension's pane does too.
func TestAReadThatMakesArtifactsOrComesFromElsewhereKeepsItsRecord(t *testing.T) {
	schema := json.RawMessage(`{"type":"object"}`)
	handler := func(ctx context.Context, input string) (applet.Result, error) {
		return applet.Result{OutputJSON: input}, nil
	}
	const artifactRef = "test.art/read@1"
	r, err := applet.New(applet.Bundle{
		Manifest: applet.Manifest{ID: "test.art", Version: "1.0.0", APIVersion: 1, Title: "Art", Actions: []applet.Action{
			{Ref: artifactRef, Title: "Read", Replay: "pure", Effect: "read", Capabilities: []string{"artifact.create"}, InputSchema: schema, OutputSchema: schema},
		}},
		Handlers: map[string]applet.Handler{artifactRef: handler},
		Sources:  map[string][]byte{"test": []byte("test")},
	})
	if err != nil {
		t.Fatal(err)
	}
	b := &Broker{Registry: r, State: t.TempDir()}
	ws := t.TempDir()
	id, _ := runOnce(t, b, ws, "applet", "extension:test.pane", artifactRef)
	if _, err := b.Read(ws, id); err != nil {
		t.Fatalf("a read that can make an artifact left no record: %v", err)
	}
	var o observed
	b2 := o.broker(t, &atomic.Int32{}, released())
	id, _ = runOnce(t, b2, ws, "applet", "test.provider", readRef)
	if _, err := b2.Read(ws, id); err != nil {
		t.Fatalf("an applet view's read left no record: %v", err)
	}
}

// A pane's read that is not recorded can still be cancelled by its caller,
// and by no one else, and a read that fails reports its error.
func TestAPanesReadThatIsNotRecordedCanBeCancelled(t *testing.T) {
	var o observed
	release := make(chan struct{})
	b := o.broker(t, &atomic.Int32{}, release)
	ws := t.TempDir()
	id, err := NewID()
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := b.Run(context.Background(), ws, "applet", "extension:test.pane", id, readRef, `{}`, "", time.Now().Add(time.Minute))
		done <- err
	}()
	for deadline := time.Now().Add(5 * time.Second); b.Executing() == 0; time.Sleep(5 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal("the read never started")
		}
	}
	if err := b.Cancel(ws, "applet", "extension:someone.else", id); err == nil {
		t.Fatal("another caller cancelled the read")
	}
	if err := b.Cancel(ws, "applet", "extension:test.pane", id); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("the cancelled read returned %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the cancelled read did not return")
	}
	if b.Executing() != 0 {
		t.Fatal("the cancelled read was left active")
	}
	if events, _ := o.counts(); events != 0 {
		t.Fatalf("a cancelled read told the page: %v", o.events)
	}
	// An invalid input is refused as a recorded one is, and leaves nothing.
	_, err = b.Run(context.Background(), ws, "applet", "extension:test.pane", id, readRef, `{"value":"no"}`, "", time.Now().Add(time.Minute))
	if err == nil {
		t.Fatal("an input that fails the schema was accepted")
	}
}
