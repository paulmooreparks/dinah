package judge

import (
	"context"
	"testing"
	"time"
)

// slowly is a judge that answers after a delay and does not stop when its context
// does, with a cost, as a request that has gone too far to take back.
type slowly struct {
	delay time.Duration
	text  string
}

func (s *slowly) Ask(_ context.Context, _ Request) (Response, error) {
	time.Sleep(s.delay)
	return Response{Text: s.text, Model: "m-late", Seed: 7, Cost: Cost{In: 90, Out: 10}}, nil
}

// failingLate is a judge that fails after a delay, with the words of its failure.
type failingLate struct{ delay time.Duration }

func (f *failingLate) Ask(context.Context, Request) (Response, error) {
	time.Sleep(f.delay)
	return Response{}, context.DeadlineExceeded
}

// A call that is late is answered by its rule at the deadline, and its request is
// not cancelled: when the answer arrives its cost is written to the same record and
// counted against the budget, with the answer kept and marked as not used. The
// request holds its place in the pool until it ends.
func TestALateJudgesCostIsCountedWhenItsRequestEnds(t *testing.T) {
	c := &clock{t: monday.Add(48 * time.Hour)}
	set := budgeted(t, &slowly{delay: 450 * time.Millisecond, text: "Late title"}, ptr(100000), c)
	start := time.Now()
	r := callOnce(set)
	if took := time.Since(start); r.Why != WhyLate || r.Used != UsedRule || r.Out != "Please tag" || took > 400*time.Millisecond {
		t.Fatalf("the caller was answered %+v after %v", r, took)
	}
	rec, _ := set.Store().Get(r.ID)
	if rec.Why != WhyLate || rec.Used != UsedRule || rec.CostIn != 0 || rec.Answer != "" {
		t.Fatalf("at the deadline the record %+v", rec)
	}
	if st := set.PoolState(); st.InFlight != 1 {
		t.Fatalf("the late request does not hold its place: %+v", st)
	}
	if uses, _ := set.BudgetUses(); uses[0].Spent != 0 {
		t.Fatalf("spent %d before the request ended", uses[0].Spent)
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if got, _ := set.Store().Get(r.ID); got.CostIn != 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	rec, _ = set.Store().Get(r.ID)
	if rec.Why != WhyLate || rec.Used != UsedRule || rec.Out != `"Please tag"` || rec.Answer != "Late title" || rec.CostIn != 90 || rec.CostOut != 10 || rec.CostUnit != "tokens" || rec.Model != "m-late" || rec.Seed != 7 {
		t.Fatalf("after the request ended the record %+v", rec)
	}
	if uses, _ := set.BudgetUses(); uses[0].Spent != 100 {
		t.Fatalf("spent %d after the request ended", uses[0].Spent)
	}
	deadline = time.Now().Add(2 * time.Second)
	for set.PoolState().InFlight != 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if st := set.PoolState(); st.InFlight != 0 {
		t.Fatalf("the place was not given back: %+v", st)
	}
}

// A request is costed whichever ends first, the caller giving up at the deadline or
// the request ending, including when they end at almost the same moment.
func TestALateRequestIsCompletedWhicheverEndsFirst(t *testing.T) {
	for _, delay := range []time.Duration{250 * time.Millisecond, 205 * time.Millisecond, 199 * time.Millisecond} {
		c := &clock{t: monday.Add(48 * time.Hour)}
		set := budgeted(t, &slowly{delay: delay, text: "Title"}, ptr(100000), c)
		r := callOnce(set)
		deadline := time.Now().Add(3 * time.Second)
		var rec Record
		for time.Now().Before(deadline) {
			rec, _ = set.Store().Get(r.ID)
			if rec.CostIn != 0 {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
		if rec.CostIn != 90 || rec.CostOut != 10 {
			t.Fatalf("delay %v: used %s why %q, record %+v", delay, r.Used, r.Why, rec)
		}
	}
}

// A late request that fails keeps its failure as the answer on the record, and
// costs nothing.
func TestALateRequestThatFailsKeepsItsFailure(t *testing.T) {
	c := &clock{t: monday.Add(48 * time.Hour)}
	set := budgeted(t, &failingLate{delay: 300 * time.Millisecond}, ptr(100000), c)
	r := callOnce(set)
	deadline := time.Now().Add(3 * time.Second)
	var rec Record
	for time.Now().Before(deadline) {
		rec, _ = set.Store().Get(r.ID)
		if rec.Answer != "" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if rec.Why != WhyLate || rec.Answer != "context deadline exceeded" || rec.CostIn != 0 {
		t.Fatalf("record %+v", rec)
	}
}
