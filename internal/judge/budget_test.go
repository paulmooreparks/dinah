package judge

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// clock is a time the test sets.
type clock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *clock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

// advance moves the clock on, as a model that takes d to answer does.
func (c *clock) advance(d time.Duration) {
	c.mu.Lock()
	c.t = c.t.Add(d)
	c.mu.Unlock()
}

func (c *clock) set(t time.Time) {
	c.mu.Lock()
	c.t = t
	c.mu.Unlock()
}

// A Monday, so the default period begins on it.
var monday = time.Date(2026, 10, 26, 0, 0, 0, 0, time.Local)

func budgeted(t *testing.T, j Judge, limit *int, c *clock) *Judges {
	t.Helper()
	s, err := OpenStore(filepath.Join(t.TempDir(), "judgements.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	set := New(Options{Store: s, Judge: j, PoolMin: 1, PoolMax: 1, Now: c.now})
	set.SetJudge(j, "test", "tokens")
	set.SetBudgets(map[string]Budget{"test": {Limit: limit, Unit: "tokens"}}, DefaultPeriod)
	return set
}

func spendy(cost int) *fake {
	return &fake{reply: func(context.Context, Request) (Response, error) {
		return Response{Text: "Title", Model: "m1", Cost: Cost{In: cost - 20, Out: 20}}, nil
	}}
}

func ptr(n int) *int { return &n }

func callOnce(set *Judges) Result[string] {
	return Call(context.Background(), set, shortTitle, titleIn{Line: "Please tag the parser release"}, Effect{})
}

// The period is the week that holds now, beginning at the person's weekday and
// hour, in local time, and the allowance is the share of the limit that the share
// of the period gone has earned.
func TestThePeriodAndTheAllowanceFollowTheClock(t *testing.T) {
	at := func(day, hour, minute int) time.Time { return time.Date(2026, 10, day, hour, minute, 0, 0, time.Local) }
	cases := []struct {
		name       string
		now        time.Time
		period     Period
		start, end time.Time
	}{
		{"midweek", at(28, 15, 30), DefaultPeriod, at(26, 0, 0), at(2, 0, 0).AddDate(0, 1, 0)},
		{"the moment it begins", at(26, 0, 0), DefaultPeriod, at(26, 0, 0), at(2, 0, 0).AddDate(0, 1, 0)},
		{"just before it ends", at(25, 23, 59), DefaultPeriod, at(19, 0, 0), at(26, 0, 0)},
		{"before the hour on its day", at(30, 8, 0), Period{Weekday: int(time.Friday), Hour: 9}, at(23, 9, 0), at(30, 9, 0)},
		{"after the hour on its day", at(30, 10, 0), Period{Weekday: int(time.Friday), Hour: 9}, at(30, 9, 0), at(6, 9, 0).AddDate(0, 1, 0)},
		{"sunday is 0", at(28, 12, 0), Period{Weekday: 0, Hour: 0}, at(25, 0, 0), at(1, 0, 0).AddDate(0, 1, 0)},
	}
	for _, c := range cases {
		start, end := PeriodAt(c.now, c.period)
		if !start.Equal(c.start) || !end.Equal(c.end) {
			t.Errorf("%s: %v to %v, want %v to %v", c.name, start, end, c.start, c.end)
		}
	}
	start, end := PeriodAt(monday.Add(3*24*time.Hour), DefaultPeriod)
	if got := Allowance(700, start, end, start.Add(24*time.Hour)); got < 99.99 || got > 100.01 {
		t.Fatalf("a day into a week of 700 is %v", got)
	}
	if Allowance(700, start, end, start.Add(-time.Hour)) != 0 || Allowance(700, start, end, end.Add(time.Hour)) != 700 {
		t.Fatal("the allowance is not 0 before the period and the limit after it")
	}
}

// With no limit a call is never refused. With one, a call is sent only while
// what has been spent is below the allowance, the allowance grows with the period,
// a refused call is answered by its rule with the reason budget, and a new period
// starts the spending again.
func TestACallIsSentOnlyWhileSpendingIsBelowTheAllowance(t *testing.T) {
	c := &clock{t: monday.Add(24 * time.Hour)}
	none := budgeted(t, spendy(100), nil, c)
	for i := 0; i < 5; i++ {
		if r := callOnce(none); r.Used != UsedJudge {
			t.Fatalf("a call with no budget was refused: %+v", r)
		}
	}
	// 1000 for the week; a day in, the allowance is a seventh of it, about 143.
	set := budgeted(t, spendy(100), ptr(1000), c)
	for i, want := range []string{UsedJudge, UsedJudge, UsedRule} {
		if r := callOnce(set); r.Used != want {
			t.Fatalf("call %d used %s (%+v)", i+1, r.Used, r)
		}
	}
	rows, _, _ := set.Store().List(Filter{})
	if rows[0].Why != WhyBudget || rows[0].Provider != "test" || rows[0].Out != `"Please tag"` || rows[0].CostIn != 0 {
		t.Fatalf("the refused call's record %+v", rows[0])
	}
	if uses, _ := set.BudgetUses(); len(uses) != 1 || uses[0].Spent != 200 || uses[0].Left != 800 || uses[0].Limit != 1000 || uses[0].Unit != "tokens" {
		t.Fatalf("the use %+v", uses)
	}
	// A day later the allowance has caught up, and a call is sent again.
	c.set(monday.Add(48 * time.Hour))
	if r := callOnce(set); r.Used != UsedJudge {
		t.Fatalf("after the allowance grew %+v", r)
	}
	// The next period starts from nothing, however much was spent in the last.
	c.set(monday.Add(7*24*time.Hour + time.Hour))
	if uses, _ := set.BudgetUses(); uses[0].Spent != 0 || !uses[0].PeriodStart.Equal(monday.AddDate(0, 0, 7)) {
		t.Fatalf("the new period %+v", uses)
	}
	c.set(monday.Add(7*24*time.Hour + 24*time.Hour))
	if r := callOnce(set); r.Used != UsedJudge {
		t.Fatalf("in the new period %+v", r)
	}
}

// A budget of 0 is a real budget that refuses every call, the first call of a
// period goes because spending and allowance are both nothing and the next is held
// to the even pace, and a budget counts only the unit of its provider.
func TestABudgetOfNothingRefusesEverythingAndCountsItsOwnUnit(t *testing.T) {
	c := &clock{t: monday.Add(24 * time.Hour)}
	zero := budgeted(t, spendy(10), ptr(0), c)
	if r := callOnce(zero); r.Why != WhyBudget {
		t.Fatalf("a budget of nothing %+v", r)
	}
	start := budgeted(t, spendy(10), ptr(1000), &clock{t: monday})
	if r := callOnce(start); r.Used != UsedJudge {
		t.Fatalf("the first call of a period was refused %+v", r)
	}
	if r := callOnce(start); r.Why != WhyBudget {
		t.Fatalf("the second call, with the allowance still nothing, was sent %+v", r)
	}
	// Spending in another unit is not this budget's.
	set := budgeted(t, spendy(100), ptr(1000), c)
	set.SetBudgets(map[string]Budget{"test": {Limit: ptr(1000), Unit: "acu"}}, DefaultPeriod)
	set.SetJudge(spendy(100), "test", "acu")
	for i := 0; i < 2; i++ {
		callOnce(set)
	}
	set.SetBudgets(map[string]Budget{"test": {Limit: ptr(1000), Unit: "tokens"}}, DefaultPeriod)
	if uses, _ := set.BudgetUses(); len(uses) != 1 || uses[0].Spent != 0 {
		t.Fatalf("calls counted in acu were spent from a budget in tokens: %+v", uses)
	}
	// A provider with no limit has no row.
	set.SetBudgets(map[string]Budget{"test": {Unit: "tokens"}}, DefaultPeriod)
	if uses, _ := set.BudgetUses(); len(uses) != 0 {
		t.Fatalf("a provider with no limit has a row: %+v", uses)
	}
}
