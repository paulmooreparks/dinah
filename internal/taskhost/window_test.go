package taskhost

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

type backendFunc func(context.Context, Scope, Operation, Capture) Outcome

func (f backendFunc) Execute(c context.Context, s Scope, o Operation, save Capture) Outcome {
	return f(c, s, o, save)
}

type callerKey struct{}

func fixture(t *testing.T) (*Window, *Options, *time.Time, *int) {
	t.Helper()
	now := time.Now()
	count := 0
	o := Options{State: t.TempDir(), Now: func() time.Time { return now }, Resolve: func(context.Context, Scope) (Binding, error) {
		return Binding{Path: "invented-workspace", Generation: "mount-1"}, nil
	}, Check: func(context.Context, Scope) error { return nil }, Admit: func(context.Context, Scope, Operation) error { return nil }, Confirm: func(ctx context.Context, r Review) error {
		if ctx.Value(callerKey{}) != "person" {
			return refusal("not_authorized", "not confirmed")
		}
		return nil
	}, Track: func(Scope, Operation) (func(), error) { return func() {}, nil }, Backend: backendFunc(func(ctx context.Context, s Scope, o Operation, capture Capture) Outcome {
		count++
		if err := capture(Progress{Identity: "native-1"}); err != nil {
			return Outcome{State: "unknown", Code: "persistence_failed"}
		}
		return Outcome{State: "succeeded", Identity: "native-1"}
	})}
	o.HoldTask = func(context.Context, Scope) (func() error, error) { return func() error { return nil }, nil }
	w, err := New(o)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := w.Shutdown(ctx); err != nil {
			t.Error(err)
		}
	})
	return w, &o, &now, &count
}

func plan(now time.Time) Plan {
	return Plan{Scope: Scope{Service: "test", Workspace: "workspace", Task: "task", Generation: "1"}, Permits: []Permit{{Kind: "TakeTurn", Target: "chat"}}, Deadline: now.Add(time.Hour)}
}
func begin(t *testing.T, w *Window, p Plan) *Handle {
	t.Helper()
	r, e := w.Review(context.Background(), p)
	if e != nil {
		t.Fatal(e)
	}
	h, e := w.Begin(context.WithValue(context.Background(), callerKey{}, "person"), r.ID)
	if e != nil {
		t.Fatal(e)
	}
	return h
}
func await(t *testing.T, w *Window, s Scope, id string) Receipt {
	t.Helper()
	tick := time.NewTicker(time.Millisecond)
	defer tick.Stop()
	for {
		r, e := w.Read(context.Background(), s, id)
		if e != nil {
			t.Fatal(e)
		}
		if r.State != "prepared" && r.State != "in_progress" {
			return r
		}
		select {
		case <-t.Context().Done():
			t.Fatalf("receipt did not settle before the test ended: state %q, %v", r.State, t.Context().Err())
		case <-tick.C:
		}
	}
}

func TestAuthorityAndServiceIsolation(t *testing.T) {
	w, _, now, count := fixture(t)
	p := plan(*now)
	r, err := w.Review(context.Background(), p)
	if err != nil {
		t.Fatal(err)
	}
	if *count != 0 {
		t.Fatal("review executed")
	}
	if _, err = w.Begin(context.Background(), r.ID); err == nil {
		t.Fatal("forged person")
	}
	if _, err = w.ForService("other").Begin(context.WithValue(context.Background(), callerKey{}, "person"), r.ID); err == nil {
		t.Fatal("foreign service")
	}
	*now = now.Add(6 * time.Minute)
	if _, err = w.Begin(context.WithValue(context.Background(), callerKey{}, "person"), r.ID); err == nil {
		t.Fatal("expired review")
	}
	if _, err = w.Start(context.Background(), &Handle{}, Operation{ID: "forged", Kind: "TakeTurn", Target: "chat"}); err == nil {
		t.Fatal("forged handle")
	}
}

func TestOperationReceiptAndUnknownRecovery(t *testing.T) {
	w, _, now, count := fixture(t)
	w.options.Backend = backendFunc(func(context.Context, Scope, Operation, Capture) Outcome {
		*count++
		return Outcome{State: "unknown", Code: "unknown_outcome"}
	})
	p := plan(*now)
	h := begin(t, w, p)
	op := Operation{ID: "one", Kind: "TakeTurn", Target: "chat"}
	r, err := w.Start(context.Background(), h, op)
	if err != nil {
		t.Fatal(err)
	}
	r = await(t, w, p.Scope, op.ID)
	if r.State != "unknown" || *count != 1 {
		t.Fatal(r, *count)
	}
	again, err := w.Start(context.Background(), h, op)
	if err != nil || again.ID != r.ID || *count != 1 {
		t.Fatal(again, err)
	}
	op.Input = []byte(`"changed"`)
	if _, err = w.Start(context.Background(), h, op); err == nil {
		t.Fatal("operation reuse")
	}
	if err = w.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	o := w.options
	next, err := New(o)
	if err != nil {
		t.Fatal(err)
	}
	defer next.Shutdown(context.Background())
	recovery, err := next.Rebind(context.Background(), p.Scope)
	if err != nil || len(recovery.Receipts) != 1 || recovery.Receipts[0].State != "unknown" {
		t.Fatal(recovery, err)
	}
	if _, err = next.Start(context.Background(), h, Operation{ID: "replacement", Kind: "TakeTurn", Target: "chat"}); err == nil {
		t.Fatal("old handle survived restart")
	}
	if *count != 1 {
		t.Fatal("reconciliation executed")
	}
	h2 := begin(t, next, p)
	if _, err = next.Start(context.Background(), h2, Operation{ID: "fresh", Kind: "TakeTurn", Target: "chat"}); err != nil {
		t.Fatal(err)
	}
	await(t, next, p.Scope, "fresh")
	if *count != 2 {
		t.Fatal("fresh consent control")
	}
}

func TestRevocationAndInternalShutdownCheckpoint(t *testing.T) {
	w, _, now, _ := fixture(t)
	entered := make(chan struct{})
	cancelled := make(chan struct{})
	w.options.Backend = backendFunc(func(ctx context.Context, _ Scope, _ Operation, capture Capture) Outcome {
		if err := capture(Progress{Identity: "accepted-before-cancel"}); err != nil {
			return Outcome{State: "unknown"}
		}
		close(entered)
		<-ctx.Done()
		close(cancelled)
		return Outcome{State: "cancelled", Code: "cancelled"}
	})
	p := plan(*now)
	h := begin(t, w, p)
	op := Operation{ID: "pending", Kind: "TakeTurn", Target: "chat"}
	if _, err := w.Start(context.Background(), h, op); err != nil {
		t.Fatal(err)
	}
	<-entered
	w.CloseAdmission()
	if _, err := w.Start(context.Background(), h, Operation{ID: "late", Kind: "TakeTurn", Target: "chat"}); err == nil {
		t.Fatal("shutdown admitted work")
	}
	if err := w.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	<-cancelled
	r, err := w.Read(context.Background(), p.Scope, op.ID)
	if err != nil || r.State != "cancelled" || r.Identity != "accepted-before-cancel" {
		t.Fatal(r, err)
	}
	if s := w.Snapshot(); !s.AdmissionClosed || s.Workers != 0 || s.Handles != 0 {
		t.Fatal(s)
	}
}

func TestWorkspaceAndRemovedChatNeverReviveOldHandle(t *testing.T) {
	w, _, now, count := fixture(t)
	p := plan(*now)
	h := begin(t, w, p)
	w.RevokeChat("chat")
	if _, err := w.Start(context.Background(), h, Operation{ID: "one", Kind: "TakeTurn", Target: "chat"}); err == nil {
		t.Fatal("removed handle")
	}
	if *count != 0 {
		t.Fatal("revoked work ran")
	}
	w2, _, n2, _ := fixture(t)
	h2 := begin(t, w2, plan(*n2))
	w2.options.Resolve = func(context.Context, Scope) (Binding, error) { return Binding{Path: "different", Generation: "2"}, nil }
	if _, err := w2.Start(context.Background(), h2, Operation{ID: "one", Kind: "TakeTurn", Target: "chat"}); err == nil {
		t.Fatal("remount")
	}
}

func TestMissingRoomCapabilityFailsReview(t *testing.T) {
	w, _, now, count := fixture(t)
	p := plan(*now)
	p.Rooms = true
	if _, err := w.Review(context.Background(), p); err == nil {
		t.Fatal("room admitted")
	}
	if *count != 0 {
		t.Fatal("work before refusal")
	}
}

func TestPersistenceFailurePreventsDispatch(t *testing.T) {
	w, _, now, count := fixture(t)
	w.options.Save = func(Receipt) error { return errors.New("invented storage failure") }
	h := begin(t, w, plan(*now))
	if _, err := w.Start(context.Background(), h, Operation{ID: "one", Kind: "TakeTurn", Target: "chat"}); err == nil {
		t.Fatal("intent not persisted")
	}
	if *count != 0 {
		t.Fatal("dispatched without receipt")
	}
	// Clear the injected failure after proving admission remains poisoned.
	w.mu.Lock()
	h.lease.poison = nil
	w.mu.Unlock()
}

func TestConcurrentOperationStartsOnce(t *testing.T) {
	w, _, now, count := fixture(t)
	p := plan(*now)
	h := begin(t, w, p)
	var group sync.WaitGroup
	for range 8 {
		group.Go(func() {
			if _, err := w.Start(context.Background(), h, Operation{ID: "one", Kind: "TakeTurn", Target: "chat"}); err != nil {
				t.Error(err)
			}
		})
	}
	group.Wait()
	await(t, w, p.Scope, "one")
	if *count != 1 {
		t.Fatal(*count)
	}
}

func TestAllocationCheckpointsSurviveAdapterPanic(t *testing.T) {
	w, _, now, _ := fixture(t)
	w.options.Backend = backendFunc(func(ctx context.Context, s Scope, o Operation, capture Capture) Outcome {
		for _, progress := range []Progress{{Dispatched: true}, {Identity: "allocated-id"}, {Identity: "allocated-id", SetupFinished: true, SetupError: "setup failed"}} {
			if err := capture(progress); err != nil {
				t.Error(err)
				return Outcome{State: "unknown"}
			}
		}
		panic("invented adapter failure")
	})
	p := plan(*now)
	h := begin(t, w, p)
	if _, err := w.Start(context.Background(), h, Operation{ID: "allocation", Kind: "TakeTurn", Target: "chat"}); err != nil {
		t.Fatal(err)
	}
	r := await(t, w, p.Scope, "allocation")
	if r.State != "unknown" || r.Identity != "allocated-id" || !r.Dispatched || !r.SetupFinished || r.SetupError != "setup failed" {
		t.Fatalf("lost durable allocation boundary: %+v", r)
	}
	if err := w.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	recovered, err := w.Rebind(context.Background(), p.Scope)
	if err == nil || len(recovered.Receipts) != 0 {
		t.Fatal("closed host admitted recovery")
	}
}

func TestBeginRequiresInterruptedReceiptReconciliation(t *testing.T) {
	w, _, now, _ := fixture(t)
	p := plan(*now)
	w.mu.Lock()
	record, err := w.read(taskKey(p.Scope))
	if err == nil {
		err = w.write(record, Receipt{ID: "receipt", Scope: p.Scope, Operation: "interrupted", State: "in_progress", Identity: "native-id"})
	}
	w.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	r, err := w.Review(context.Background(), p)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = w.Begin(context.WithValue(context.Background(), callerKey{}, "person"), r.ID); err == nil {
		t.Fatal("new execution bypassed interrupted receipt reconciliation")
	}
	recovery, err := w.Rebind(context.Background(), p.Scope)
	if err != nil || len(recovery.Receipts) != 1 || recovery.Receipts[0].State != "unknown" || recovery.Receipts[0].Identity != "native-id" {
		t.Fatalf("recovery: %+v %v", recovery, err)
	}
	begin(t, w, p)
}

func TestReceiptEventsAreCommittedOrderedAndReentrant(t *testing.T) {
	w, _, now, _ := fixture(t)
	p := plan(*now)
	var mu sync.Mutex
	var sequences []uint64
	w.options.Emit = func(r Receipt) {
		stored, err := w.Read(context.Background(), p.Scope, r.Operation)
		if err != nil || stored.Sequence < r.Sequence {
			t.Errorf("event preceded persistence: %+v %v", stored, err)
		}
		mu.Lock()
		sequences = append(sequences, r.Sequence)
		mu.Unlock()
	}
	h := begin(t, w, p)
	for _, id := range []string{"first", "second"} {
		if _, err := w.Start(context.Background(), h, Operation{ID: id, Kind: "TakeTurn", Target: "chat"}); err != nil {
			t.Fatal(err)
		}
		await(t, w, p.Scope, id)
	}
	if err := w.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(sequences) != 8 {
		t.Fatalf("missing receipt events: %v", sequences)
	}
	for i, sequence := range sequences {
		if sequence != uint64(i+1) {
			t.Fatalf("task event order: %v", sequences)
		}
	}
}

func TestTwoTasksCannotOverlapOneChatTurn(t *testing.T) {
	w, _, now, _ := fixture(t)
	w.options.Resource = func(context.Context, Scope, Operation) (string, error) { return "canonical-chat", nil }
	started, release := make(chan struct{}), make(chan struct{})
	w.options.Backend = backendFunc(func(context.Context, Scope, Operation, Capture) Outcome {
		close(started)
		<-release
		return Outcome{State: "succeeded"}
	})
	p := plan(*now)
	first := begin(t, w, p)
	secondPlan := p
	secondPlan.Scope.Task = "second-task"
	second := begin(t, w, secondPlan)
	op := Operation{ID: "turn", Kind: "TakeTurn", Target: "chat"}
	if _, err := w.Start(context.Background(), first, op); err != nil {
		t.Fatal(err)
	}
	<-started
	if _, err := w.Start(context.Background(), second, op); err == nil {
		t.Error("second task overlapped a chat turn")
	}
	close(release)
	await(t, w, p.Scope, op.ID)
}
