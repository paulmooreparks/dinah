package bench

import (
	"fmt"
	"sync/atomic"
	"testing"
	"time"
)

// TestParallelReadPreservesOrder asserts that the answer comes back indexed
// exactly as a serial `for i := range n` loop would have built it, whatever
// order the pool's workers happen to finish reading in.
//
// Arming: swapping `results[i] = ...` for an append inside parallelRead (the
// natural mistake once reads race) reddens this by scrambling the order,
// because a worker pool answers reads out of arrival order and an append
// records whichever worker got there first.
func TestParallelReadPreservesOrder(t *testing.T) {
	const n = 200
	results, err := parallelRead(n, func(i int) (int, error) {
		// A tiny, size-dependent sleep spreads completion order across the
		// pool: without it every worker could happen to finish in launch
		// order on a fast machine, and the test would pass by accident
		// rather than by the indexing this asserts.
		time.Sleep(time.Duration(n-i) * time.Microsecond)
		return i * 2, nil
	})
	if err != nil {
		t.Fatalf("parallelRead: %v", err)
	}
	if len(results) != n {
		t.Fatalf("got %d results, wanted %d", len(results), n)
	}
	for i, v := range results {
		if v != i*2 {
			t.Fatalf("results[%d] = %d, wanted %d: order was not preserved", i, v, i*2)
		}
	}
}

// TestParallelReadReportsTheLowestFailingIndex asserts the mid-walk failure
// contract this card settles: where more than one read fails, the error
// reported is the one at the lowest index, which is the failure a serial loop
// over the same range would have stopped on first. Reporting whichever
// goroutine happened to fail first instead would make two runs over the same
// broken input report two different errors.
//
// Arming: reporting the first error a worker observes, in completion order
// rather than index order (the race a naive `if err != nil { return nil, err
// }` inside the worker goroutine falls into), reddens this: a fast failure at
// a high index would then race a slow failure at a low index and could win.
func TestParallelReadReportsTheLowestFailingIndex(t *testing.T) {
	const n = 50
	failing := map[int]bool{40: true, 10: true, 25: true}
	_, err := parallelRead(n, func(i int) (int, error) {
		if failing[i] {
			// The highest-indexed failure sleeps least, so it is the one most
			// likely to finish first if completion order, rather than index
			// order, governed the answer.
			time.Sleep(time.Duration(n-i) * time.Millisecond / 5)
			return 0, fmt.Errorf("planted failure at %d", i)
		}
		return i, nil
	})
	if err == nil {
		t.Fatal("parallelRead reported no error over a walk carrying three planted failures")
	}
	wanted := "planted failure at 10"
	if err.Error() != wanted {
		t.Errorf("parallelRead reported %q, wanted %q (the lowest failing index)", err.Error(), wanted)
	}
}

// TestParallelReadRunsFromMoreThanOneGoroutineAtOnce asserts concurrency
// structurally: at least two of the pool's workers are inside read at the
// same moment, which is what "the walk runs in parallel" means, and it is
// asserted without measuring wall time, which the CI performance workbench's
// own test already does and which a loaded runner can make flaky.
//
// Each worker announces itself on a channel the instant it starts and then
// blocks until the test has observed two such announcements, so the
// assertion cannot pass merely because two reads happened to be scheduled
// back to back on one worker; it passes only if two are inside read
// concurrently. A 2-second timeout fails the test explicitly rather than
// hanging the suite if concurrency regresses to one worker.
//
// Arming: forcing parallelWorkers to answer 1 reddens this by name, because a
// pool of one worker can never have two readers inside read at once, and the
// test times out waiting for the second announcement instead of silently
// passing.
func TestParallelReadRunsFromMoreThanOneGoroutineAtOnce(t *testing.T) {
	const n = 8
	started := make(chan struct{}, n)
	release := make(chan struct{})
	var seen int32

	done := make(chan error, 1)
	go func() {
		_, err := parallelRead(n, func(i int) (int, error) {
			started <- struct{}{}
			<-release
			return i, nil
		})
		done <- err
	}()

	seenTwo := make(chan struct{})
	go func() {
		for range started {
			if atomic.AddInt32(&seen, 1) >= 2 {
				close(seenTwo)
				return
			}
		}
	}()

	select {
	case <-seenTwo:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for two workers to be inside read at once; the pool is not running in parallel")
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatalf("parallelRead: %v", err)
	}
}

// TestParallelReadOfZeroAnswersNothing pins the empty case rather than
// leaving it to fall out of the loop by accident: a walk over an empty
// collection asks for no reads and reports no error, on the terms
// cardsWith's own empty-collection callers already rely on.
func TestParallelReadOfZeroAnswersNothing(t *testing.T) {
	calls := 0
	results, err := parallelRead(0, func(i int) (int, error) {
		calls++
		return i, nil
	})
	if err != nil {
		t.Fatalf("parallelRead(0, ...): %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("parallelRead(0, ...) answered %d results", len(results))
	}
	if calls != 0 {
		t.Fatalf("parallelRead(0, ...) called read %d times", calls)
	}
}

// TestParallelWorkersIsAtLeastOne pins parallelWorkers's floor: whatever
// GOMAXPROCS reports, a pool of zero workers would leave parallelRead's loop
// never launched and every read unanswered.
func TestParallelWorkersIsAtLeastOne(t *testing.T) {
	if got := parallelWorkers(); got < 1 {
		t.Fatalf("parallelWorkers() = %d, wanted at least 1", got)
	}
}
