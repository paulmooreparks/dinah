package residenttest

import (
	"errors"
	"sync"

	"dinah/internal/resident"
)

// Manual is the notifier a test drives by hand. It honours every sentence of
// resident.Notifier's contract on every platform, so the resident's state
// machine is tested without a real watcher.
//
// Its two failure switches are independent and consumed in this order: Fail
// affects only the next Next, and an Arm answers an error while a FailArm
// count remains, decrementing it, and succeeds once the count is spent. A
// delivery made while the notifier is closed is dropped, as a real watch that
// is down loses what happens meanwhile.
type Manual struct {
	mu   sync.Mutex
	cond *sync.Cond
	// closeMu serialises Close, so a second Close waits for the first.
	closeMu sync.Mutex

	armed    bool
	root     string
	arms     int
	attempts int
	valid    bool
	running  bool

	queue   []resident.Batch
	holding bool
	held    resident.Batch
	fail    error
	failArm int
}

// NewManual answers a notifier that is not yet armed.
func NewManual() *Manual {
	m := &Manual{}
	m.cond = sync.NewCond(&m.mu)
	return m
}

// errArmRefused is what Arm answers while a FailArm count remains.
var errArmRefused = errors.New("residenttest: arm refused")

// Arm records the root and succeeds, unless a FailArm count remains.
func (m *Manual) Arm(root string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.attempts++
	if m.failArm > 0 {
		m.failArm--
		return errArmRefused
	}
	m.armed = true
	m.root = root
	m.valid = true
	m.arms++
	m.cond.Broadcast()
	return nil
}

// Next blocks until a batch or a failure is available, or the notifier is
// closed.
func (m *Manual) Next() (resident.Batch, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.running = true
	defer func() {
		m.running = false
		m.cond.Broadcast()
	}()
	for {
		if !m.armed {
			return resident.Batch{}, resident.ErrClosed
		}
		if m.fail != nil {
			err := m.fail
			m.fail = nil
			return resident.Batch{}, err
		}
		if len(m.queue) > 0 {
			batch := m.queue[0]
			m.queue = m.queue[1:]
			return batch, nil
		}
		m.cond.Wait()
	}
}

// Valid answers false after Invalidate, until the next Arm.
func (m *Manual) Valid() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.valid
}

// Close stops watching. A Next in flight answers ErrClosed, and Close
// returns once it has returned. A second Close, concurrent or not, answers
// nil.
func (m *Manual) Close() error {
	m.closeMu.Lock()
	defer m.closeMu.Unlock()
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.armed {
		return nil
	}
	m.armed = false
	m.queue = nil
	m.cond.Broadcast()
	for m.running {
		m.cond.Wait()
	}
	return nil
}

// Deliver reports changes, or queues them while Hold is in force. A delivery
// made while the notifier is closed is dropped.
func (m *Manual) Deliver(changes ...resident.Change) {
	m.deliver(resident.Batch{Changes: append([]resident.Change(nil), changes...)})
}

// Overflow reports that changes were lost.
func (m *Manual) Overflow() {
	m.deliver(resident.Batch{Overflow: true})
}

// deliver hands one batch over, or holds it.
func (m *Manual) deliver(batch resident.Batch) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.armed {
		return
	}
	if m.holding {
		m.held.Changes = append(m.held.Changes, batch.Changes...)
		m.held.Overflow = m.held.Overflow || batch.Overflow
		return
	}
	m.queue = append(m.queue, batch)
	m.cond.Broadcast()
}

// Fail makes the next Next answer err.
func (m *Manual) Fail(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.fail = err
	m.cond.Broadcast()
}

// FailArm makes the next n calls of Arm answer an error.
func (m *Manual) FailArm(n int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.failArm = n
}

// Invalidate makes Valid answer false until the next Arm.
func (m *Manual) Invalidate() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.valid = false
}

// Hold queues every delivery until Release.
func (m *Manual) Hold() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.holding = true
}

// Release hands every delivery held since Hold over together, as one batch.
func (m *Manual) Release() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.holding = false
	if !m.armed || (len(m.held.Changes) == 0 && !m.held.Overflow) {
		m.held = resident.Batch{}
		return
	}
	m.queue = append(m.queue, m.held)
	m.held = resident.Batch{}
	m.cond.Broadcast()
}

// Arms answers how many times Arm has succeeded.
func (m *Manual) Arms() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.arms
}

// Attempts answers how many times Arm has been called, failing or not.
func (m *Manual) Attempts() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.attempts
}

// Root answers the root the last successful Arm recorded.
func (m *Manual) Root() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.root
}
