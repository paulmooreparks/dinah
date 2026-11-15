//go:build quality_task_host

package main

import (
	"context"
	"slices"
	"strings"
	"testing"

	"studio/internal/taskhost"
)

func TestTaskHostRoomPeerRequiresClaimAndCorrelatesRoots(t *testing.T) {
	f := newFrozenTaskHost(t).(*frozenTaskHost)
	scope := f.scope(req())
	ctx := context.Background()
	inspection, err := f.store.client.Inspect(ctx, scope)
	if err != nil || inspection.Rooms || slices.Contains(inspection.Operations, "PostRoom") {
		t.Fatal(inspection, err)
	}
	f.Change("enable-complete-room-peer")
	inspection, err = f.store.client.Inspect(ctx, scope)
	if err != nil || !inspection.Rooms || !slices.Contains(inspection.Operations, "PostRoom") {
		t.Fatal(inspection, err)
	}
	peer := f.a.taskRooms()
	op := taskhost.Operation{ID: "root", Kind: "PostRoom", Target: "invented-room"}
	operation := func(kind string) taskhost.Operation { value := op; value.Kind = kind; return value }
	foreign := scope
	foreign.OwnerChat = "another-lead"
	if _, err := peer.Review(ctx, foreign, taskhost.Permit{Kind: op.Kind, Target: op.Target}); err == nil {
		t.Fatal("foreign canonical lead was accepted")
	}
	if _, err := peer.Ensure(ctx, scope, operation("EnsureRoom"), nil); err != nil {
		t.Fatal(err)
	}
	if _, err := peer.Release(ctx, scope, operation("ReleaseRoom"), nil); err != nil {
		t.Fatal(err)
	}
	if _, err := peer.Review(ctx, scope, taskhost.Permit{Kind: op.Kind, Target: op.Target}); err == nil {
		t.Fatal("room without a phase claim was accepted")
	}
	if _, err := peer.Claim(ctx, scope, operation("ClaimRoom"), nil); err != nil {
		t.Fatal(err)
	}
	if _, err := peer.Post(ctx, scope, op, nil); err != nil {
		t.Fatal(err)
	}
	observed, err := peer.Observe(ctx, scope, operation("ObserveRoom"), nil)
	if err != nil || !strings.Contains(string(observed.Result), `"root":"root-root"`) {
		t.Fatal(observed, err)
	}
	if recovered, err := peer.Recover(ctx, scope, operation("RecoverRoom"), nil); err != nil || string(recovered.Result) != string(observed.Result) {
		t.Fatal(recovered, err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := peer.Observe(cancelled, scope, operation("ObserveRoom"), nil); err == nil {
		t.Fatal("cancelled observation was accepted")
	}
	if _, err := peer.Post(ctx, scope, op, nil); err == nil {
		t.Fatal("peer hid a repeated post attempt")
	}
	if f.Snapshot().Counts.Posts != 2 {
		t.Fatal("post counter hid a repeated attempt")
	}
}
