//go:build quality_task_host

package main

import (
	"context"
	"testing"
)

func TestTaskHostRepeatedPendingOperationReturnsReceipt(t *testing.T) {
	f := newFrozenTaskHost(t).(*frozenTaskHost)
	r := req()
	b := start(t, f, r)
	f.Change("block-owned-work-and-register-question-observer")
	pending := f.Execute(b.Handle, r)
	if pending.State != "in_progress" {
		t.Fatal(pending)
	}
	before := f.Snapshot().Counts
	repeated, err := f.store.client.Start(context.Background(), f.handles[b.Handle], f.operation(r))
	if err != nil || repeated.ID != pending.Receipt || repeated.State != "in_progress" {
		t.Fatalf("pending receipt was hidden by busy admission: %+v %v", repeated, err)
	}
	changed := r
	changed.Operation = "another-turn"
	if _, err := f.store.client.Start(context.Background(), f.handles[b.Handle], f.operation(changed)); err == nil {
		t.Fatal("another turn overlapped pending work")
	}
	quiet(t, f, before)
	f.Cancel(b.Handle)
	f.Change("settle-owned-cancellation")
}
