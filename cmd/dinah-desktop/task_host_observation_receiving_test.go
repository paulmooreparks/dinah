//go:build quality_task_host

package main

import (
	"testing"
	"time"
)

func TestTaskHostPendingObservationBinding(t *testing.T) {
	for _, mode := range []string{"cancel", "shutdown", "bin"} {
		t.Run(mode, func(t *testing.T) {
			f := newFrozenTaskHost(t).(*frozenTaskHost)
			r := req()
			b := start(t, f, r)
			protected := f.Snapshot().ProtectedRecords
			f.Change("block-owned-work-and-register-question-observer")
			if pending := f.Execute(b.Handle, r); pending.State != "in_progress" {
				t.Fatal(pending)
			}
			live := f.Snapshot()
			if live.Observers != 1 || live.Workers != 2 || live.Questions != 1 {
				t.Fatalf("binding has no live observer and pending question: %+v", live)
			}
			if f.roomPeer.cancelled.Load() != 0 || f.roomPeer.completed.Load() != 0 {
				t.Fatal("observation ended before cancellation")
			}
			switch mode {
			case "cancel":
				f.Cancel(b.Handle)
			case "bin":
				f.Change("bin-owning-chat")
			case "shutdown":
				f.StartShutdown()
				f.FinishShutdown(time.Minute)
			}
			f.Change("settle-owned-cancellation")
			ended := f.Snapshot()
			if ended.Observers != 0 || ended.Workers != 0 || ended.Questions != 0 || f.roomPeer.cancelled.Load() != 1 || f.roomPeer.completed.Load() != 1 {
				t.Fatalf("observation did not cancel and complete: %+v", ended)
			}
			if receipt := ended.Receipts[r.Operation+"-observe"]; receipt.State != "unknown" && receipt.State != "cancelled" {
				t.Fatalf("observer terminal receipt missing: %+v", receipt)
			}
			if !ended.Checkpointed || ended.ProtectedRecords != protected {
				t.Fatal("cleanup did not retain receipts and linked records")
			}
			f.Restart()
			if restarted := f.Snapshot(); restarted.Observers != 0 || restarted.ProtectedRecords != protected {
				t.Fatalf("restart changed observation or protected records: %+v", restarted)
			}
		})
	}
}
