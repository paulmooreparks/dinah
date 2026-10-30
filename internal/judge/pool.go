package judge

import (
	"context"
	"errors"
	"sync"
	"time"
)

// The pool is the program's own limit on judge calls in flight at once
// (docs/plans/judgement-agents.md, step 4). For Ollama it is the program's side
// only, because how many requests the server serves at once is the server's own
// setting, which the program does not change: a pool larger than that only
// queues at the server, and the page's pool line shows that it grew without
// getting faster.
const (
	// GrowFraction is the share of a call's deadline it must wait for a place
	// before the pool grows by one.
	GrowFraction = 0.5
	// ShrinkFactor is how many times the longest deadline of any kind the pool
	// must sit idle before it shrinks by one.
	ShrinkFactor = 10
	// DefaultPoolMin and DefaultPoolMax are the pool's bounds until the person sets
	// others. The largest is a default and not a ceiling.
	DefaultPoolMin = 1
	DefaultPoolMax = 4
)

// PoolChange is the last time the pool changed size and why.
type PoolChange struct {
	At   time.Time `json:"at"`
	From int       `json:"from"`
	To   int       `json:"to"`
	Why  string    `json:"why"`
}

// PoolState is the pool as the page shows it. GrowAt and ShrinkAfter are the
// two named fractions, sent with the size so the page holds neither.
type PoolState struct {
	Size        int     `json:"size"`
	Min         int     `json:"min"`
	Max         int     `json:"max"`
	GrowAt      float64 `json:"growAt"`
	ShrinkAfter int     `json:"shrinkAfter"`
	// InFlight is how many calls are with the judge now, a call that was answered
	// by its rule for being late among them until its request ends.
	InFlight int         `json:"inFlight"`
	Changed  *PoolChange `json:"changed,omitempty"`
}

type waiter struct {
	ready   chan struct{}
	granted bool
}

type pool struct {
	mu       sync.Mutex
	now      func() time.Time
	longest  func() time.Duration
	size     int
	min, max int
	inflight int
	waiters  []*waiter
	idleFrom time.Time
	changed  *PoolChange
}

func newPool(now func() time.Time, longest func() time.Duration, min, max int) *pool {
	if min < 1 {
		min = DefaultPoolMin
	}
	if max < min {
		max = DefaultPoolMax
		if max < min {
			max = min
		}
	}
	return &pool{now: now, longest: longest, size: min, min: min, max: max, idleFrom: now()}
}

// setBounds sets the smallest and largest size, and brings the size inside them.
func (p *pool) setBounds(min, max int) error {
	if min < 1 || max < min {
		return errors.New("the pool needs a smallest size of 1 or more and a largest that is not below it")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.min, p.max = min, max
	switch {
	case p.size < min:
		p.resizeLocked(min, "the bounds changed")
	case p.size > max:
		p.resizeLocked(max, "the bounds changed")
	}
	return nil
}

func (p *pool) resizeLocked(to int, why string) {
	p.changed = &PoolChange{At: p.now(), From: p.size, To: to, Why: why}
	p.size = to
	p.idleFrom = p.now()
	p.grantLocked()
}

// grantLocked gives places to the calls that are waiting, in order, while there
// are places.
func (p *pool) grantLocked() {
	for p.inflight < p.size && len(p.waiters) > 0 {
		w := p.waiters[0]
		p.waiters = p.waiters[1:]
		w.granted = true
		p.inflight++
		close(w.ready)
	}
}

// shrinkLocked takes one place away when the pool has been idle for ShrinkFactor
// times the longest deadline, and starts the idle time again, so a longer idle
// takes away another.
func (p *pool) shrinkLocked() {
	if p.inflight != 0 || len(p.waiters) != 0 || p.size <= p.min {
		return
	}
	if p.now().Sub(p.idleFrom) >= ShrinkFactor*p.longest() {
		p.resizeLocked(p.size-1, "shrank: idle")
	}
}

// state is the pool as it is now.
func (p *pool) state() PoolState {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.shrinkLocked()
	return PoolState{Size: p.size, Min: p.min, Max: p.max, GrowAt: GrowFraction, ShrinkAfter: ShrinkFactor, InFlight: p.inflight, Changed: p.changed}
}

func (p *pool) release() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.inflight--
	if p.inflight == 0 && len(p.waiters) == 0 {
		p.idleFrom = p.now()
	}
	p.grantLocked()
}

// acquire waits for a place until ctx ends and returns the function that gives it
// back. A call that has waited GrowFraction of its deadline makes the pool grow by
// one, up to its largest size.
func (p *pool) acquire(ctx context.Context, deadline time.Duration) (func(), bool) {
	p.mu.Lock()
	p.shrinkLocked()
	if p.inflight < p.size && len(p.waiters) == 0 {
		p.inflight++
		p.mu.Unlock()
		return p.release, true
	}
	w := &waiter{ready: make(chan struct{})}
	p.waiters = append(p.waiters, w)
	p.mu.Unlock()
	timer := time.NewTimer(time.Duration(float64(deadline) * GrowFraction))
	defer timer.Stop()
	for {
		select {
		case <-w.ready:
			return p.release, true
		case <-timer.C:
			p.mu.Lock()
			if !w.granted && p.size < p.max {
				p.resizeLocked(p.size+1, "grew: a call waited")
			}
			p.mu.Unlock()
		case <-ctx.Done():
			p.mu.Lock()
			if w.granted {
				p.mu.Unlock()
				p.release()
				return nil, false
			}
			for i, x := range p.waiters {
				if x == w {
					p.waiters = append(p.waiters[:i], p.waiters[i+1:]...)
					break
				}
			}
			p.mu.Unlock()
			return nil, false
		}
	}
}
