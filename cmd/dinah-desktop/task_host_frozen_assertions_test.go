//go:build quality_task_host

package main

import (
	"reflect"
	"testing"
	"time"
)

// Factory is intentionally unbound until an immutable host candidate arrives.
// Bindings must call production entry points and read actual persisted receipts.
var Factory func(*testing.T) Host

type Request struct {
	Task, Generation, Kind, Operation, Target, Digest, Input string
	Rooms                                                    bool
}
type Result struct{ Handle, Receipt, State, Code, Identity string }
type Counts struct{ Starts, Loads, Prompts, Actions, Posts, Allocations int }
type Snapshot struct {
	Counts                                                Counts
	Workers, Questions, Observers                         int
	AdmissionClosed, HandlesRevoked, Checkpointed, Joined bool
	CancelRequests                                        int
	Events                                                []string
	Receipts                                              map[string]Result
	ProtectedRecords                                      string
}
type Host interface {
	Review(Request) Result
	Begin(review Result, caller string) Result
	Execute(handle string, request Request) Result
	Rebind(task, generation, caller string) Result
	Inspect()
	Snapshot() Snapshot
	Restart()
	Advance(time.Duration)
	Change(string)
	Cancel(string)
	StartShutdown()
	FinishShutdown(time.Duration)
}

// Change configures invented peers or invokes real lifecycle mutations described
// in the binding note. It must never set expected host outcomes or receipts.
func fresh(t *testing.T) Host {
	t.Helper()
	if Factory == nil {
		t.Fatal("task-host binding absent; acceptance has not run")
	}
	return Factory(t)
}
func req() Request {
	return Request{Task: "task-a", Generation: "generation-1", Kind: "TakeTurn", Operation: "op-1", Target: "chat-a", Digest: "digest-1", Input: "input-1"}
}
func start(t *testing.T, h Host, r Request) Result {
	t.Helper()
	review := h.Review(r)
	b := h.Begin(review, "trusted-person")
	if b.Handle == "" || b.Code != "" {
		t.Fatalf("positive admission failed: %+v", b)
	}
	return b
}
func quiet(t *testing.T, h Host, c Counts) {
	t.Helper()
	if h.Snapshot().Counts != c {
		t.Fatalf("unexpected execution: before %+v after %+v", c, h.Snapshot().Counts)
	}
}
func refused(t *testing.T, r Result) {
	t.Helper()
	if r.Code == "" || r.State == "succeeded" || r.Handle != "" {
		t.Fatalf("not refused: %+v", r)
	}
}
func TestFrozenTaskHostAuthority(t *testing.T) {
	for _, mode := range []string{"forged-person", "bare-service", "foreign-invocation", "expired-review", "stale-generation"} {
		t.Run(mode, func(t *testing.T) {
			h := fresh(t)
			r := req()
			review := h.Review(r)
			before := h.Snapshot().Counts
			caller := mode
			if mode == "expired-review" {
				h.Advance(2 * time.Hour)
				caller = "trusted-person"
			}
			if mode == "stale-generation" {
				h.Change("advance-task-generation")
				caller = "trusted-person"
			}
			refused(t, h.Begin(review, caller))
			quiet(t, h, before)
		})
	}
}
func TestFrozenTaskHostObservationAndPositiveControl(t *testing.T) {
	h := fresh(t)
	zero := Counts{}
	h.Inspect()
	h.Review(req())
	quiet(t, h, zero)
	b := start(t, h, req())
	result := h.Execute(b.Handle, req())
	if result.State != "succeeded" || result.Receipt == "" {
		t.Fatal(result)
	}
	if h.Snapshot().Counts.Prompts != 1 {
		t.Fatal("positive control did not reach one real turn entry")
	}
}
func TestFrozenTaskHostRevokedHandles(t *testing.T) {
	for _, change := range []string{"deadline", "cancel", "bin-owning-chat", "bin-referenced-chat", "workspace-remount", "workspace-missing", "workspace-ambiguous", "workspace-inaccessible", "restart"} {
		t.Run(change, func(t *testing.T) {
			h := fresh(t)
			b := start(t, h, req())
			before := h.Snapshot().Counts
			switch change {
			case "deadline":
				h.Advance(2 * time.Hour)
			case "cancel":
				h.Cancel(b.Handle)
			case "restart":
				h.Restart()
			default:
				h.Change(change)
			}
			refused(t, h.Execute(b.Handle, req()))
			quiet(t, h, before)
			if change == "bin-owning-chat" || change == "bin-referenced-chat" {
				h.Change("restore-chat")
				refused(t, h.Execute(b.Handle, req()))
				quiet(t, h, before)
			}
		})
	}
}
func TestFrozenTaskHostOperationIdentity(t *testing.T) {
	h := fresh(t)
	r := req()
	b := start(t, h, r)
	a := h.Execute(b.Handle, r)
	if a.State != "succeeded" || a.Receipt == "" {
		t.Fatal(a)
	}
	before := h.Snapshot().Counts
	again := h.Execute(b.Handle, r)
	if again.Receipt != a.Receipt || again.Identity != a.Identity {
		t.Fatal("same operation lost its receipt")
	}
	quiet(t, h, before)
	for _, field := range []string{"kind", "target", "input", "digest"} {
		changed := r
		switch field {
		case "kind":
			changed.Kind = "OpenChat"
		case "target":
			changed.Target = "chat-b"
		case "input":
			changed.Input = "input-2"
		case "digest":
			changed.Digest = "digest-2"
		}
		refused(t, h.Execute(b.Handle, changed))
		quiet(t, h, before)
	}
}
func TestFrozenTaskHostUnknownNeverReplays(t *testing.T) {
	for _, kind := range []string{"OpenChat", "TakeTurn", "InvokeAction", "PostRoom"} {
		t.Run(kind, func(t *testing.T) {
			h := fresh(t)
			if kind == "PostRoom" {
				h.Change("enable-complete-room-peer")
			}
			r := req()
			r.Kind = kind
			r.Rooms = kind == "PostRoom"
			b := start(t, h, r)
			h.Change("lose-response-after-dispatch")
			got := h.Execute(b.Handle, r)
			if got.State != "unknown" || got.Receipt == "" {
				t.Fatal(got)
			}
			before := h.Snapshot().Counts
			again := h.Execute(b.Handle, r)
			if again.State != "unknown" || again.Receipt != got.Receipt {
				t.Fatal(again)
			}
			quiet(t, h, before)
			h.Restart()
			recovered := h.Rebind(r.Task, r.Generation, "record-lock-only")
			quiet(t, h, before)
			if recovered.Handle == "" {
				t.Fatal("recovery authority absent", recovered)
			}
			h.Inspect()
			quiet(t, h, before)
			if persisted := h.Snapshot().Receipts[r.Operation]; persisted.State != "unknown" || persisted.Receipt != got.Receipt {
				t.Fatal("unknown receipt lost", persisted)
			}
			r.Operation = "replacement-op"
			refused(t, h.Execute(recovered.Handle, r))
			quiet(t, h, before)
		})
	}
}
func TestFrozenTaskHostRecoveryNeedsFreshConsent(t *testing.T) {
	h := fresh(t)
	r := req()
	b := start(t, h, r)
	got := h.Execute(b.Handle, r)
	if got.State != "succeeded" {
		t.Fatal(got)
	}
	before := h.Snapshot().Counts
	h.Restart()
	recovered := h.Rebind(r.Task, r.Generation, "record-lock-only")
	quiet(t, h, before)
	refused(t, h.Execute(b.Handle, r))
	r.Operation = "new-op"
	refused(t, h.Execute(recovered.Handle, r))
	quiet(t, h, before)
	freshAuthority := start(t, h, r)
	result := h.Execute(freshAuthority.Handle, r)
	if result.State != "succeeded" {
		t.Fatal(result)
	}
	if h.Snapshot().Counts.Prompts != before.Prompts+1 {
		t.Fatal("fresh consent positive control failed")
	}
}
func TestFrozenTaskHostRoomsAbsentBeforeAnyWork(t *testing.T) {
	h := fresh(t)
	r := req()
	r.Rooms = true
	before := h.Snapshot().Counts
	review := h.Review(r)
	if review.Code != "unavailable" && review.Code != "unsupported" {
		t.Fatal(review)
	}
	refused(t, h.Begin(review, "trusted-person"))
	quiet(t, h, before)
}
func TestFrozenTaskHostCancellationAndShutdown(t *testing.T) {
	for _, mode := range []string{"cancel", "shutdown"} {
		t.Run(mode, func(t *testing.T) {
			h := fresh(t)
			r := req()
			b := start(t, h, r)
			protected := h.Snapshot().ProtectedRecords
			h.Change("block-owned-work-and-register-question-observer")
			pending := h.Execute(b.Handle, r)
			if pending.State != "in_progress" || h.Snapshot().Workers == 0 {
				t.Fatal("no live owned work", pending)
			}
			before := h.Snapshot().Counts
			if mode == "cancel" {
				h.Cancel(b.Handle)
			} else {
				h.StartShutdown()
			}
			refused(t, h.Execute(b.Handle, Request{Task: r.Task, Generation: r.Generation, Operation: "after-close", Kind: "TakeTurn", Target: r.Target}))
			quiet(t, h, before)
			if mode == "shutdown" {
				h.FinishShutdown(time.Minute)
			} else {
				h.Change("settle-owned-cancellation")
			}
			s := h.Snapshot()
			if s.CancelRequests == 0 || s.Workers != 0 || s.Questions != 0 || s.Observers != 0 || !s.HandlesRevoked || !s.Checkpointed {
				t.Fatalf("cleanup did not finish: %+v", s)
			}
			if mode == "shutdown" {
				if !s.AdmissionClosed || !s.Joined {
					t.Fatal("shutdown did not close and join")
				}
				positions := map[string]int{}
				for i, event := range s.Events {
					if _, ok := positions[event]; !ok {
						positions[event] = i
					}
				}
				for _, pair := range [][2]string{{"admission-closed", "cancel-requested"}, {"cancel-requested", "receipts-persisted"}, {"receipts-persisted", "shared-teardown"}} {
					a, okA := positions[pair[0]]
					z, okZ := positions[pair[1]]
					if !okA || !okZ || a >= z {
						t.Fatal("shutdown ordering", s.Events)
					}
				}
			}
			receipt, ok := s.Receipts[r.Operation]
			if !ok || (receipt.State != "cancelled" && receipt.State != "unknown") {
				t.Fatal("unsettled outcome not retained", receipt)
			}
			if s.ProtectedRecords != protected {
				t.Fatal("cleanup changed linked records")
			}
			h.Restart()
			h.Inspect()
			quiet(t, h, before)
			if !reflect.DeepEqual(h.Snapshot().Receipts, s.Receipts) {
				t.Fatal("restart changed durable receipts")
			}
		})
	}
}

func TestFrozenTaskHostForgedHandle(t *testing.T) {
	h := fresh(t)
	before := h.Snapshot().Counts
	refused(t, h.Execute("invented-handle", req()))
	quiet(t, h, before)
}
func TestFrozenTaskHostBinCancelsPendingWork(t *testing.T) {
	h := fresh(t)
	r := req()
	b := start(t, h, r)
	h.Change("block-owned-work-and-register-question-observer")
	if result := h.Execute(b.Handle, r); result.State != "in_progress" {
		t.Fatal(result)
	}
	before := h.Snapshot().Counts
	h.Change("bin-owning-chat")
	h.Change("settle-owned-cancellation")
	s := h.Snapshot()
	if s.CancelRequests == 0 || s.Workers != 0 || !s.HandlesRevoked {
		t.Fatal("bin did not cancel pending task")
	}
	refused(t, h.Execute(b.Handle, r))
	h.Change("restore-chat")
	refused(t, h.Execute(b.Handle, r))
	quiet(t, h, before)
}
