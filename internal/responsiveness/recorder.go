package responsiveness

import (
	"math"
	"sync"
	"sync/atomic"
	"time"
)

const maxCounter = uint64(1<<53 - 1)

type stat struct {
	count, total, maximum uint64
	offset                time.Duration
	saturated             bool
}

func (s *stat) add(duration, offset time.Duration) {
	if duration < 0 {
		return
	}
	if s.count < maxCounter {
		s.count++
	} else {
		s.saturated = true
	}
	n := uint64(duration)
	if math.MaxUint64-s.total < n {
		s.total = math.MaxUint64
		s.saturated = true
	} else {
		s.total += n
	}
	if n > s.maximum || s.count == 1 {
		s.maximum = n
		s.offset = offset
	}
}
func (s stat) view() Stats {
	v := Stats{Count: number(float64(s.count)), Total: number(float64(s.total) / 1e6), Max: missing(Unavailable), Offset: missing(Unavailable), Saturated: s.saturated}
	if s.count != 0 {
		v.Max = number(float64(s.maximum) / 1e6)
		v.Offset = number(float64(s.offset) / 1e6)
	}
	return v
}

type runtimeValue struct {
	count, lower, upper, mutex           float64
	countState, maximumState, mutexState uint8
	open                                 bool
	start, duration                      time.Duration
}
type storedSlot struct {
	sequence                  uint64
	valid, queueSeen          bool
	stats                     [categoryCount][2]stat
	subscribers, depth, drops uint64
	runtime                   runtimeValue
}
type Token struct {
	generation uint64
	started    time.Time
	category   Category
	kind       Kind
}

func (t Token) Active() bool { return t.generation != 0 }

type Recorder struct {
	enabled             atomic.Bool
	generation          atomic.Uint64
	control             sync.Mutex
	mu                  sync.Mutex
	anchor              time.Time
	slots               [Capacity]storedSlot
	overwritten, missed uint64
	stop, done          chan struct{}
	closed              bool
}

func (r *Recorder) Enabled() bool { return r != nil && r.enabled.Load() }
func (r *Recorder) Begin(name string, kind Kind) Token {
	if !r.Enabled() {
		return Token{}
	}
	g := r.generation.Load()
	at := time.Now()
	if !r.enabled.Load() || r.generation.Load() != g {
		return Token{}
	}
	return Token{g, at, Classify(name), kind}
}
func (r *Recorder) End(t Token, subscribers, depth, drops int) {
	if !t.Active() {
		return
	}
	at := time.Now()
	r.record(t, at, subscribers, depth, drops)
}
func (r *Recorder) record(t Token, at time.Time, subscribers, depth, drops int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.enabled.Load() || r.generation.Load() != t.generation || t.category >= categoryCount || t.kind > Bridge {
		return
	}
	elapsed := at.Sub(r.anchor)
	if elapsed < 0 {
		return
	}
	s := r.slot(uint64(elapsed / Period))
	if s == nil {
		return
	}
	s.stats[t.category][t.kind].add(at.Sub(t.started), elapsed%Period)
	if t.kind == Bridge && subscribers >= 0 {
		s.queueSeen = true
		s.subscribers = max(s.subscribers, uint64(max(0, subscribers)))
		s.depth = max(s.depth, uint64(max(0, depth)))
		s.drops = min(maxCounter, s.drops+uint64(max(0, drops)))
	}
}
func (r *Recorder) slot(sequence uint64) *storedSlot {
	s := &r.slots[sequence%Capacity]
	// A completion that waited behind a snapshot may already be outside
	// retention. It must not evict a newer observation in the same slot.
	if s.valid && s.sequence > sequence {
		return nil
	}
	if !s.valid || s.sequence != sequence {
		if s.valid {
			r.overwritten = min(maxCounter, r.overwritten+1)
		}
		*s = storedSlot{sequence: sequence, valid: true}
	}
	return s
}

// Control calls join the old sampler before starting another. A flood of
// toggles therefore cannot accumulate background samplers or metric buffers.
func (r *Recorder) Set(on bool) State {
	r.control.Lock()
	defer r.control.Unlock()
	return r.set(on)
}
func (r *Recorder) set(on bool) State {
	if r.closed {
		on = false
	}
	if r.enabled.Load() == on {
		r.mu.Lock()
		defer r.mu.Unlock()
		return r.state()
	}
	r.enabled.Store(false)
	if r.stop != nil {
		close(r.stop)
		<-r.done
		r.stop = nil
		r.done = nil
	}
	r.mu.Lock()
	r.slots = [Capacity]storedSlot{}
	r.overwritten = 0
	r.missed = 0
	r.anchor = time.Now()
	r.generation.Add(1)
	if on {
		r.slot(0)
		r.stop = make(chan struct{})
		r.done = make(chan struct{})
		r.enabled.Store(true)
	}
	state := r.state()
	stop, done, generation, anchor := r.stop, r.done, r.generation.Load(), r.anchor
	r.mu.Unlock()
	if on {
		go r.sampleLoop(stop, done, generation, anchor)
	}
	return state
}
func (r *Recorder) Close() { r.control.Lock(); defer r.control.Unlock(); r.set(false); r.closed = true }
func (r *Recorder) state() State {
	label := "off"
	if r.enabled.Load() {
		label = "on"
	}
	anchor := r.anchor
	if anchor.IsZero() {
		anchor = time.Now()
	}
	return State{r.enabled.Load(), r.generation.Load(), Clock{anchor.UTC(), 0}, label}
}
func (r *Recorder) sampleLoop(stop, done chan struct{}, generation uint64, anchor time.Time) {
	defer close(done)
	reader := runtimeReader{}
	reader.read(time.Now(), anchor)
	ticker := time.NewTicker(Period)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			// Use completion time, not a queued ticker's old timestamp.
			now := time.Now()
			value := reader.read(now, anchor)
			r.mu.Lock()
			if r.enabled.Load() && r.generation.Load() == generation {
				s := r.slot(uint64(now.Sub(anchor) / Period))
				if s != nil {
					s.runtime = value
				}
				if value.duration >= 2*Period {
					r.missed = min(maxCounter, r.missed+uint64(value.duration/Period)-1)
				}
			}
			r.mu.Unlock()
		}
	}
}
func (r *Recorder) Snapshot() Snapshot {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	state := r.state()
	elapsed := now.Sub(r.anchor)
	if r.anchor.IsZero() {
		elapsed = 0
	}
	result := Snapshot{Version: 1, Recording: state, Captured: Clock{now.UTC(), float64(elapsed) / 1e6}, Coverage: Coverage{Overwritten: r.overwritten, Missed: r.missed, Disabled: missing(Unavailable)}, Slots: []Slot{}}
	if !r.enabled.Load() {
		return result
	}
	latest := uint64(elapsed / Period)
	first := uint64(0)
	if latest >= Capacity {
		first = latest - Capacity + 1
	}
	for sequence := first; sequence <= latest; sequence++ {
		s := r.slots[sequence%Capacity]
		if !s.valid || s.sequence != sequence {
			continue
		}
		start := time.Duration(sequence) * Period
		duration := min(Period, elapsed-start)
		span := Span{state.Anchor, float64(start) / 1e6, float64(duration) / 1e6, duration == Period}
		categories := [categoryCount]CategoryStats{}
		for c := Category(0); c < categoryCount; c++ {
			categories[c] = CategoryStats{s.stats[c][App].view(), s.stats[c][Bridge].view()}
		}
		queue := Queues{missing(Unavailable), missing(Unavailable), missing(Unavailable)}
		if s.queueSeen {
			queue = Queues{number(float64(s.subscribers)), number(float64(s.depth)), number(float64(s.drops))}
		}
		v := s.runtime
		runtimeSpan := Span{state.Anchor, float64(v.start) / 1e6, float64(v.duration) / 1e6, v.duration > 0}
		rt := RuntimeInterval{Scope: "process_wide_interval", Time: runtimeSpan, GCCount: runtimeNumber(v.countState, v.count), GCLower: runtimeNumber(v.maximumState, v.lower), GCUpper: runtimeNumber(v.maximumState, v.upper), GCOpen: v.open, Mutex: runtimeNumber(v.mutexState, v.mutex)}
		if v.open {
			rt.GCUpper = missing(Unavailable)
		}
		result.Slots = append(result.Slots, Slot{span, Categories{categories[Text], categories[Tool], categories[Chat], categories[Team], categories[Bus], categories[Other]}, queue, rt})
	}
	result.Coverage.Retained = len(result.Slots)
	return result
}
