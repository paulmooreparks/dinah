package judge

import (
	"context"
	"sync"
	"testing"
	"time"
)

// holding is a judge whose answers wait until the test lets them go.
type holding struct {
	mu      sync.Mutex
	started int
	release chan struct{}
	// stubborn makes the judge ignore its context, so it keeps its place until it
	// is let go.
	stubborn bool
}

func (h *holding) Ask(ctx context.Context, _ Request) (Response, error) {
	h.mu.Lock()
	h.started++
	h.mu.Unlock()
	if h.stubborn {
		<-h.release
		return Response{Text: "Title", Model: "m"}, nil
	}
	select {
	case <-h.release:
		return Response{Text: "Title", Model: "m"}, nil
	case <-ctx.Done():
		return Response{}, ctx.Err()
	}
}

func (h *holding) count() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.started
}

func poolSet(t *testing.T, j Judge, min, max int, c *clock) *Judges {
	t.Helper()
	s, err := OpenStore(t.TempDir() + "/judgements.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	o := Options{Store: s, Judge: j, PoolMin: min, PoolMax: max}
	if c != nil {
		o.Now = c.now
	}
	set := New(o)
	set.SetJudge(j, "test", "tokens")
	return set
}

// A call that has waited half its deadline for a place makes the pool grow by one,
// and is then sent; the pool never goes above its largest size, which is the
// person's and has no ceiling of its own.
func TestThePoolGrowsWhenACallHasWaitedHalfItsDeadline(t *testing.T) {
	h := &holding{release: make(chan struct{})}
	defer close(h.release)
	set := poolSet(t, h, 1, 3, nil)
	var wg sync.WaitGroup
	results := make(chan Result[string], 3)
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results <- callOnce(set)
		}()
		time.Sleep(20 * time.Millisecond)
	}
	// The first holds the one place. The second waits half of 200 ms, the pool
	// grows to 2 and it is sent; the third waits the same and the pool grows to 3.
	deadline := time.Now().Add(2 * time.Second)
	for h.count() < 3 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if h.count() != 3 {
		t.Fatalf("%d calls reached the judge", h.count())
	}
	st := set.PoolState()
	if st.Size != 3 || st.Min != 1 || st.Max != 3 || st.GrowAt != GrowFraction || st.ShrinkAfter != ShrinkFactor || st.Changed == nil || st.Changed.Why != "grew: a call waited" || st.Changed.From != 2 || st.Changed.To != 3 {
		t.Fatalf("pool %+v", st)
	}
}

// With its largest size reached, a call waits until its deadline and is answered
// by its rule, and a largest size above the old default of four is accepted.
func TestThePoolNeverGrowsPastItsLargestSize(t *testing.T) {
	h := &holding{release: make(chan struct{}), stubborn: true}
	defer close(h.release)
	set := poolSet(t, h, 1, 1, nil)
	go callOnce(set)
	time.Sleep(30 * time.Millisecond)
	if r := callOnce(set); r.Why != WhyLate || set.PoolState().Size != 1 || h.count() != 1 {
		t.Fatalf("a call past the largest size: %+v, pool %+v, %d asked", r, set.PoolState(), h.count())
	}
	if err := set.SetPool(1, 12); err != nil {
		t.Fatalf("a largest size above four was refused: %v", err)
	}
	if st := set.PoolState(); st.Max != 12 || st.Size != 1 {
		t.Fatalf("pool %+v", st)
	}
}

// The pool shrinks by one after ShrinkFactor times the longest deadline idle, one
// step for each such wait, and never below its smallest size.
func TestThePoolShrinksWhenItHasBeenIdleLongEnough(t *testing.T) {
	c := &clock{t: monday}
	set := poolSet(t, spendy(10), 1, 4, c)
	if err := set.SetPool(3, 4); err != nil {
		t.Fatal(err)
	}
	if st := set.PoolState(); st.Size != 3 || st.Changed == nil || st.Changed.Why != "the bounds changed" {
		t.Fatalf("raising the smallest size: %+v", st)
	}
	if err := set.SetPool(1, 4); err != nil {
		t.Fatal(err)
	}
	idle := ShrinkFactor * longestDeadline()
	c.set(monday.Add(idle - time.Millisecond))
	if st := set.PoolState(); st.Size != 3 {
		t.Fatalf("it shrank early: %+v", st)
	}
	c.set(monday.Add(idle))
	if st := set.PoolState(); st.Size != 2 || st.Changed == nil || st.Changed.Why != "shrank: idle" || st.Changed.From != 3 || st.Changed.To != 2 {
		t.Fatalf("after one idle time: %+v", st)
	}
	c.set(monday.Add(2 * idle))
	if st := set.PoolState(); st.Size != 1 {
		t.Fatalf("after two: %+v", st)
	}
	c.set(monday.Add(10 * idle))
	if st := set.PoolState(); st.Size != 1 {
		t.Fatalf("it went below its smallest size: %+v", st)
	}
}

// The bounds are whole numbers of 1 or more with the smallest not above the
// largest, and a size outside new bounds is brought inside them.
func TestThePoolsBoundsAreChecked(t *testing.T) {
	set := poolSet(t, spendy(10), 1, 4, nil)
	for _, bad := range [][2]int{{0, 3}, {-1, 3}, {3, 2}, {0, 0}} {
		if err := set.SetPool(bad[0], bad[1]); err == nil {
			t.Errorf("%v was accepted", bad)
		}
	}
	if st := set.PoolState(); st.Min != 1 || st.Max != 4 {
		t.Fatalf("a refused change moved the bounds: %+v", st)
	}
	if err := set.SetPool(2, 2); err != nil {
		t.Fatal(err)
	}
	if st := set.PoolState(); st.Size != 2 {
		t.Fatalf("%+v", st)
	}
	if err := set.SetPool(1, 1); err != nil {
		t.Fatal(err)
	}
	if st := set.PoolState(); st.Size != 1 || st.Changed.Why != "the bounds changed" {
		t.Fatalf("%+v", st)
	}
}
