package sessionhost

import (
	_ "embed"
	"errors"
	"sync"
	"sync/atomic"
)

//go:embed turns.go
var TurnsSource []byte

// Turns owns reply capture, pending steering and the submission gate. mu is
// the conversation mutex, injected before publication. Locked operations
// participate in compound transitions inside Conversation.
type Turns struct {
	mu             *sync.Mutex
	gate           sync.Mutex
	capture        *Prompt
	asked          bool
	askedSteer     *Prompt
	askedDelivered bool
	pending        []PendingSteer
}

func NewTurns(mu *sync.Mutex) *Turns { return &Turns{mu: mu} }
func (t *Turns) requirePrompt(p Prompt) {
	if p.reply != nil && p.reply.mu != t.mu {
		panic("sessionhost: prompt belongs to another conversation")
	}
}

// Submission has one owner. It serializes submissions and turn completion.
// Operations after Release panic as a programming error; Release is idempotent
// and neither cancels nor resolves a reply. Receipts have a separate lifetime.
type Submission struct {
	turns     *Turns
	active    atomic.Bool
	completed sync.Once
}

func (t *Turns) Begin() *Submission {
	t.gate.Lock()
	s := &Submission{turns: t}
	s.active.Store(true)
	return s
}
func (t *Turns) TryBegin() *Submission {
	if !t.gate.TryLock() {
		return nil
	}
	s := &Submission{turns: t}
	s.active.Store(true)
	return s
}
func (s *Submission) require() {
	if s == nil || !s.active.Load() {
		panic("sessionhost: released submission")
	}
}
func (s *Submission) Release() {
	if s != nil && s.active.CompareAndSwap(true, false) {
		s.turns.gate.Unlock()
	}
}

// PendingSteer is a copied observation. Prompt fields are opaque immutable
// values; Receipt retains the original delivery identity after detachment.
type PendingSteer struct {
	ID, LedgerID int64
	Text         string
	Receipt      *DeliveryReceipt
	Fallback     *Prompt
}

func copyPending(in []PendingSteer) []PendingSteer {
	out := append([]PendingSteer(nil), in...)
	for i := range out {
		if out[i].Fallback != nil {
			p := *out[i].Fallback
			out[i].Fallback = &p
		}
	}
	return out
}
func (s *Submission) Pending() []PendingSteer {
	s.require()
	t := s.turns
	t.mu.Lock()
	defer t.mu.Unlock()
	return copyPending(t.pending)
}
func (s *Submission) TakePending() []PendingSteer {
	s.require()
	t := s.turns
	t.mu.Lock()
	defer t.mu.Unlock()
	p := copyPending(t.pending)
	t.pending = nil
	return p
}
func (s *Submission) Restore(p PendingSteer) {
	s.require()
	if p.Receipt != nil && p.Receipt.turns != s.turns {
		panic("sessionhost: receipt belongs to another conversation")
	}
	if p.Fallback != nil {
		s.turns.requirePrompt(*p.Fallback)
	}
	t := s.turns
	t.mu.Lock()
	defer t.mu.Unlock()
	t.pending = append(t.pending, copyPending([]PendingSteer{p})[0])
}
func (s *Submission) Drop(id int64) {
	s.require()
	t := s.turns
	t.mu.Lock()
	defer t.mu.Unlock()
	for i, p := range t.pending {
		if p.ID == id {
			t.pending = append(t.pending[:i:i], t.pending[i+1:]...)
			return
		}
	}
}
func (s *Submission) RegisterHook(id, ledgerID int64, text string, p Prompt) {
	s.Restore(PendingSteer{ID: id, LedgerID: ledgerID, Text: text, Fallback: &p})
}

// DeliveryReceipt confirms the original captured prompt once, independently
// of submission or registry lifetime. Confirm takes only the session mutex.
type DeliveryReceipt struct {
	turns     *Turns
	prompt    Prompt
	delivered bool
}

func (s *Submission) RegisterNative(id, ledgerID int64, p Prompt) *DeliveryReceipt {
	s.require()
	s.turns.requirePrompt(p)
	r := &DeliveryReceipt{turns: s.turns, prompt: p}
	s.Restore(PendingSteer{ID: id, LedgerID: ledgerID, Receipt: r})
	return r
}
func (r *DeliveryReceipt) Confirm() (Prompt, bool) {
	r.turns.mu.Lock()
	defer r.turns.mu.Unlock()
	if r.delivered {
		return Prompt{}, false
	}
	r.delivered = true
	return r.prompt, true
}
func (r *DeliveryReceipt) Delivered() bool {
	r.turns.mu.Lock()
	defer r.turns.mu.Unlock()
	return r.delivered
}
func (r *DeliveryReceipt) Prompt() Prompt { return r.prompt }

func (t *Turns) CapturingLocked() bool { return t.capture != nil }
func (t *Turns) AskedLocked() bool     { return t.asked }
func (t *Turns) ReservedLocked() bool  { return t.askedSteer != nil }
func (t *Turns) StartCaptureLocked(p *Prompt, reset bool) {
	t.capture, t.asked = nil, false
	if p != nil {
		t.requirePrompt(*p)
		t.capture, t.asked = p.capture(), p.Asked()
	}
	if reset && t.capture != nil {
		t.capture.resetCaptureLocked()
	}
}
func (t *Turns) ResetCaptureLocked() {
	if t.capture != nil {
		t.capture.resetCaptureLocked()
	}
	if t.askedSteer != nil {
		t.askedSteer.resetCaptureLocked()
	}
}
func (t *Turns) AppendCaptureLocked(text string) {
	if t.capture != nil {
		t.capture.appendCaptureLocked(text)
	}
	if t.askedSteer != nil {
		t.askedSteer.appendCaptureLocked(text)
	}
}
func (t *Turns) ReserveLocked(p Prompt) {
	t.requirePrompt(p)
	t.askedSteer, t.askedDelivered = &p, false
}
func (t *Turns) ClearReservationLocked() { t.askedSteer, t.askedDelivered = nil, false }
func (t *Turns) MarkDeliveredLocked(p Prompt) {
	if p.Asked() && t.askedSteer != nil && t.askedSteer.SameReply(p) {
		t.askedDelivered = true
	}
}

// Completion carries detached reply settlement across the session-lock
// boundary. Finish releases the steering gate before resolving either reply.
type Completion struct {
	owner     *Turns
	asked     *Prompt
	delivered bool
}

func (t *Turns) DetachReplyLocked() Completion {
	c := Completion{t, t.askedSteer, t.askedDelivered}
	t.ClearReservationLocked()
	return c
}
func (s *Submission) Finish(c Completion, current Prompt, late []Prompt, end End) {
	s.completed.Do(func() {
		s.require()
		if c.owner != s.turns {
			panic("sessionhost: completion belongs to another conversation")
		}
		s.turns.requirePrompt(current)
		s.Release()
		if c.asked != nil {
			fallback := false
			for _, p := range late {
				if p.SameReply(*c.asked) {
					fallback = true
				}
			}
			if c.delivered || !fallback {
				askedEnd := end
				if !c.delivered && askedEnd.Err == nil {
					askedEnd.Err = errors.New("the question's steering delivery could not be confirmed; it was not retried")
				}
				c.asked.Resolve(askedEnd)
			}
		}
		current.Resolve(end)
	})
}

func (t *Turns) PendingCountLocked() int { return len(t.pending) }
