package bench

import (
	"runtime"
	"sync"
	"sync/atomic"
)

// parallelWorkers is the size of the pool a bounded parallel walk opens: one
// worker per logical CPU the runtime reports, floored at one.
//
// The adversarial review attached to the storage workstream measured
// diminishing returns from adding workers past the machine's own core count
// (44 ms at 8 goroutines, 37 ms at 16, against a 234 ms serial walk), so a
// pool sized to the machine rather than to a constant chosen once tracks that
// curve on whatever machine the walk actually runs on, and needs no knob
// anybody has to remember to tune when the fleet changes shape.
func parallelWorkers() int {
	n := runtime.GOMAXPROCS(0)
	if n < 1 {
		return 1
	}
	return n
}

// parallelRead answers read(0) through read(n-1) across a bounded worker
// pool, in the shape a serial `for i := range n` loop would have answered
// them: the results come back in index order, and where more than one index
// fails, the error reported is the one at the lowest index, which is the
// error a serial loop over the same range would have stopped on first.
//
// The pool is opened fresh for this one call and closed when it returns,
// never shared across two calls and never held between them. Dinah is a
// one-shot process: a command opens the bench, does its work and exits, so
// there is no long-lived owner a shared pool could answer to, and a pool kept
// alive across calls would need to be quiesced before the process could exit
// clean. Opening one walk at a time also means two unrelated walks in the
// same process never contend with each other for a slot in one pool.
//
// read must take no lock the workbench's own writers take. A parallel reader
// that reached into the per-card lock would serialise the very walk this
// function exists to parallelise, and every caller of this function reads
// through the lock-free path the serial walk it replaces already used.
//
// A failure part-way through does not stop the workers already running: every
// index is read, because a worker part-way through a file it has already
// opened has nothing to gain by abandoning the read, and stopping the pool
// early buys back only the reads that had not yet started. What changes is
// which of the failures is reported, and reporting the lowest index rather
// than whichever goroutine happens to finish first is what keeps two runs
// over the same broken bench from reporting two different errors.
func parallelRead[T any](n int, read func(i int) (T, error)) ([]T, error) {
	if n == 0 {
		return nil, nil
	}
	results := make([]T, n)
	errs := make([]error, n)
	workers := parallelWorkers()
	if workers > n {
		workers = n
	}
	var next int64 = -1
	var wg sync.WaitGroup
	wg.Add(workers)
	for w := 0; w < workers; w++ {
		go func() {
			defer wg.Done()
			for {
				i := int(atomic.AddInt64(&next, 1))
				if i >= n {
					return
				}
				results[i], errs[i] = read(i)
			}
		}()
	}
	wg.Wait()
	for i := 0; i < n; i++ {
		if errs[i] != nil {
			return nil, errs[i]
		}
	}
	return results, nil
}
