package sessionhost

import (
	"context"
	"sync"
	"testing"
	"time"

	"studio/internal/applet/broker"
	"studio/internal/bus"
)

func TestResultReconciliationRetriesBeforeMarkingAndQualifiesWorkspaceKeys(t *testing.T) {
	var mu sync.Mutex
	var runtime Runtime
	store := &broker.Broker{State: t.TempDir()}
	b, err := bus.OpenAt(t.TempDir() + "/bus.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = b.Close() })
	caller, err := b.RegisterSession(context.Background(), "caller", t.TempDir(), "Caller")
	if err != nil {
		t.Fatal(err)
	}
	workspaces := []string{t.TempDir(), t.TempDir()}
	for _, ws := range workspaces {
		v := broker.Invocation{ID: "same-id", Workspace: ws, OwnerKind: "chat", OwnerID: "owner", Action: ChatAskAction, InputJSON: `{"question":"q"}`, OutputJSON: `{"title":"Answerer","reply":"a"}`, Status: "done", Deadline: time.Now().Add(time.Hour)}
		if err := store.Save(&v); err != nil {
			t.Fatal(err)
		}
	}
	saved := ""
	acquired, lookedUp := 0, 0
	clock := time.Now()
	owner := NewBusChats(&mu, &runtime, BusChatsHost{
		ResultBroker: func() (*broker.Broker, error) {
			if !mu.TryLock() {
				t.Fatal("broker acquisition retained state lock")
			}
			mu.Unlock()
			acquired++
			return store, nil
		},
		SavedBusIdentity: func(id string) string {
			if !mu.TryLock() {
				t.Fatal("saved identity callback retained state lock")
			}
			defer mu.Unlock()
			if id != "owner" {
				t.Fatal(id)
			}
			lookedUp++
			return saved
		},
		Log:   func(string, string, ...any) {},
		Clock: func() time.Time { return clock },
	})
	reported := map[string]bool{}
	owner.ReconcileResults(context.Background(), b, reported)
	if len(reported) != 0 || acquired != 1 || lookedUp != 2 {
		t.Fatalf("missing identity was reported: %v, %d, %d", reported, acquired, lookedUp)
	}
	saved = caller.ID
	clock = clock.Add(resultRetryLongest + time.Second) // past the wait after the first failure
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	owner.ReconcileResults(ctx, b, reported)
	if len(reported) != 0 || acquired != 2 || lookedUp != 4 {
		t.Fatalf("failed enqueue was reported: %v, %d, %d", reported, acquired, lookedUp)
	}
	clock = clock.Add(resultRetryLongest + time.Second)
	owner.ReconcileResults(context.Background(), b, reported)
	if len(reported) != 2 || acquired != 3 || lookedUp != 6 {
		t.Fatalf("successful retry was not recorded: %v, %d, %d", reported, acquired, lookedUp)
	}
	for _, ws := range workspaces {
		if !reported[ChatResultKey(ws, "same-id")] {
			t.Fatal("workspace-qualified result was lost")
		}
	}
	owner.ReconcileResults(context.Background(), b, reported)
	if acquired != 4 || lookedUp != 6 {
		t.Fatal("already-reported result was enqueued again")
	}
	owner.ReconcileResults(context.Background(), b, map[string]bool{})
	msgs, _, err := b.Deliver(context.Background(), caller.ID, bus.DoorbellTurn)
	if err != nil || len(msgs) != 2 {
		t.Fatalf("restart deduplication changed: %v, %v", msgs, err)
	}
	want := `{"invocationId":"same-id","request":{"question":"q"},"status":"done","output":{"title":"Answerer","reply":"a"}}`
	for _, m := range msgs {
		if m.Text != want {
			t.Fatalf("legacy report bytes changed: %q", m.Text)
		}
	}
}

// A result whose caller has no saved bus identity is not looked up again every second: it waits, for longer each time
// up to a limit, and is tried at once when a chat joins the bus.
func TestAResultWithoutACallerIdentityIsRetriedWithBackoffAndWhenAChatJoins(t *testing.T) {
	var mu sync.Mutex
	var runtime Runtime
	store := &broker.Broker{State: t.TempDir()}
	b, err := bus.OpenAt(t.TempDir() + "/bus.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = b.Close() })
	v := broker.Invocation{ID: "lonely", Workspace: t.TempDir(), OwnerKind: "chat", OwnerID: "gone", Action: ChatAskAction, InputJSON: `{"question":"q"}`, OutputJSON: `{"reply":"a"}`, Status: "done", Deadline: time.Now().Add(time.Hour)}
	if err := store.Save(&v); err != nil {
		t.Fatal(err)
	}
	clock := time.Now()
	lookups, logs := 0, 0
	owner := NewBusChats(&mu, &runtime, BusChatsHost{
		ResultBroker:     func() (*broker.Broker, error) { return store, nil },
		SavedBusIdentity: func(string) string { lookups++; return "" },
		Log:              func(string, string, ...any) { logs++ },
		Clock:            func() time.Time { return clock },
	})
	reported := map[string]bool{}
	round := func() { owner.ReconcileResults(context.Background(), b, reported) }
	round()
	for i := 0; i < 40; i++ { // four seconds of rounds, inside the first wait
		clock = clock.Add(100 * time.Millisecond)
		round()
	}
	if lookups != 1 || logs != 1 {
		t.Fatalf("within its first wait the result was looked up %d times and logged %d times", lookups, logs)
	}
	clock = clock.Add(resultRetryFirst)
	round()
	if lookups != 2 {
		t.Fatalf("after the first wait it was looked up %d times", lookups)
	}
	// The wait doubles, so the same time again is not enough.
	clock = clock.Add(resultRetryFirst)
	round()
	if lookups != 2 {
		t.Fatalf("the second wait is longer than the first, but it was looked up %d times", lookups)
	}
	// Any chat joining the bus makes it due at once.
	owner.retries.reset()
	round()
	if lookups != 3 {
		t.Fatalf("a chat joining did not make it due: %d lookups", lookups)
	}
	// The wait stops growing at its limit.
	for i := 0; i < 20; i++ {
		clock = clock.Add(resultRetryLongest)
		round()
	}
	before := lookups
	clock = clock.Add(resultRetryLongest - time.Second)
	round()
	if lookups != before {
		t.Fatal("the result was looked up before its longest wait ended")
	}
	if _, waiting := owner.retries.by[ChatResultKey(v.Workspace, v.ID)]; !waiting || owner.retries.by[ChatResultKey(v.Workspace, v.ID)].delay != resultRetryLongest {
		t.Fatalf("the wait is %v, not its limit", owner.retries.by)
	}
}
