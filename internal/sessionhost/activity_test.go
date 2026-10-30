package sessionhost

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"
)

func TestActivityAdmissionRechecksWithinSharedLock(t *testing.T) {
	var mu sync.Mutex
	turns, details := NewTurns(&mu), NewDetails(&mu)
	a := NewActivity(turns, details, InitialActivity{Open: true})
	mu.Lock()
	if !a.ReadyLocked() {
		t.Fatal("initial readiness")
	}
	// Prompt construction runs outside the lock. A replay or switch beginning
	// before admission must prevent the old readiness observation from winning.
	details.SetReplayingLocked(true)
	ready, _, _ := a.AdmitSendLocked(NewPrompt("blocked", nil))
	if ready || a.WorkingLocked() || a.QueueCountLocked() != 0 {
		t.Fatal("stale readiness admitted input")
	}
	details.SetReplayingLocked(false)
	ready, queued, _ := a.AdmitSendLocked(NewPrompt("first", nil))
	if !ready || queued || !a.WorkingLocked() {
		t.Fatal("idle admission failed")
	}
	ready, queued, texts := a.AdmitSendLocked(NewPrompt("second", nil))
	if !ready || !queued || !reflect.DeepEqual(texts, []string{"second"}) {
		t.Fatal("busy admission lost queue")
	}
	texts[0] = "changed"
	if a.QueueTextsLocked()[0] != "second" {
		t.Fatal("queue view aliases owner")
	}
	a.SetSwitchingLocked(true)
	ready, _, _ = a.QueueActiveLocked(NewPrompt("blocked", nil))
	if ready || a.QueueCountLocked() != 1 {
		t.Fatal("switch admitted queue mutation")
	}
	a.SetSwitchingLocked(false)
	a.ReleaseWorkLocked()
	ready, working, _ := a.QueueActiveLocked(NewPrompt("rebuild", nil))
	if !ready || working || a.QueueCountLocked() != 1 {
		t.Fatal("idle queue path should return to Send")
	}
	mu.Unlock()
}

func TestActivityNextTurnPriorityAndErrorRetention(t *testing.T) {
	for _, interrupt := range []bool{false, true} {
		for _, advance := range []bool{false, true} {
			var mu sync.Mutex
			turns, details := NewTurns(&mu), NewDetails(&mu)
			a := NewActivity(turns, details, InitialActivity{Open: true, Working: true})
			mu.Lock()
			a.EnqueueLocked(NewPrompt("queued", nil))
			bus := NewPrompt("bus", nil)
			late := []Prompt{NewPrompt("late", nil)}
			if interrupt {
				p := NewPrompt("interrupt", nil)
				a.SetInterruptLocked(&p)
			}
			next, steered, q, _ := a.NextTurnLocked(&bus, late, advance)
			expected := []string{"late", "bus", "queued"}
			switch {
			case interrupt:
				if next == nil || next.Text() != "interrupt" || !steered {
					t.Fatal("interrupt priority lost")
				}
			case advance:
				if next == nil || next.Text() != "late" || steered {
					t.Fatal("late prompt priority lost")
				}
				expected = expected[1:]
			default:
				if next != nil || steered || a.WorkingLocked() || a.IdleSinceLocked().IsZero() {
					t.Fatal("failed turn did not stop")
				}
			}
			if !reflect.DeepEqual(q, expected) {
				t.Fatalf("queue order: %v want %v", q, expected)
			}
			if next != nil && !a.WorkingLocked() {
				t.Fatal("next turn lost working state")
			}
			late[0] = NewPrompt("changed", nil)
			q[0] = "changed"
			if !reflect.DeepEqual(a.QueueTextsLocked(), expected) {
				t.Fatal("next-turn inputs or view alias queue")
			}
			mu.Unlock()
		}
	}
}

func TestActivityNextTurnPreservesReplySettlementBoundary(t *testing.T) {
	var mu sync.Mutex
	turns, details := NewTurns(&mu), NewDetails(&mu)
	a := NewActivity(turns, details, InitialActivity{Open: true, Working: true})
	current := NewPrompt("current", nil).WithReply(context.Background(), &mu)
	asked := NewPrompt("asked", nil).WithReply(context.Background(), &mu)
	gate := turns.Begin()
	defer gate.Release()
	mu.Lock()
	turns.StartCaptureLocked(&current, false)
	turns.ReserveLocked(asked)
	turns.MarkDeliveredLocked(asked)
	next, _, _, completion := a.NextTurnLocked(nil, nil, false)
	if next != nil || turns.CapturingLocked() || turns.ReservedLocked() {
		t.Fatal("final critical section left old turn state")
	}
	for _, p := range []Prompt{current, asked} {
		select {
		case <-p.reply.done:
			t.Fatal("state transition settled waiter under locks")
		default:
		}
	}
	mu.Unlock()
	gate.Finish(completion, current, nil, End{StopReason: "end_turn"})
	gate.Finish(completion, current, nil, End{Err: errors.New("duplicate")})
	if other := turns.TryBegin(); other == nil {
		t.Fatal("completion retained gate")
	} else {
		other.Release()
	}
	for _, p := range []Prompt{current, asked} {
		select {
		case <-p.reply.done:
			if p.reply.end.Err != nil {
				t.Fatal("duplicate completion changed reply")
			}
		default:
			t.Fatal("turn event could precede waiter")
		}
	}
}

func TestActivityLifecycleAndEndCompletion(t *testing.T) {
	var mu sync.Mutex
	a := NewActivity(NewTurns(&mu), NewDetails(&mu), InitialActivity{Open: true})
	now := time.Now()
	mu.Lock()
	a.ConnectedBackgroundLocked(now)
	if !a.ParkLocked() || !a.ParkedLocked() || a.BackgroundLocked() {
		t.Fatal("park transition")
	}
	a.MarkClosedLocked(now)
	if !a.ClosedDueLocked(false, now.Add(time.Hour), time.Minute) {
		t.Fatal("closed session not due")
	}
	end := a.BeginEndLocked()
	if end == nil || a.OpenLocked() || a.ParkedLocked() || a.BeginEndLocked() != nil {
		t.Fatal("end admission")
	}
	a.FinishEndLocked()
	if a.EndingLocked() != nil {
		t.Fatal("ending state retained")
	}
	// Cleanup ends before waiter completion, as in the desktop close path.
	select {
	case <-end.done:
		t.Fatal("cleanup settled end waiter early")
	default:
	}
	mu.Unlock()
	end.Complete()
	end.Wait()
	mu.Lock()
	a.JoinBusLocked("old")
	a.PublishBusTitleLocked("old", "old title")
	a.JoinBusLocked("new")
	a.PublishBusTitleLocked("old", "stale title")
	if a.SameBusTitleLocked("stale title") || a.BusIDLocked() != "new" {
		t.Fatal("stale title reached replacement bus identity")
	}
	mu.Unlock()
}
