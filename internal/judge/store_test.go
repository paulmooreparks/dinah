package judge

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func openTest(t *testing.T) (*Store, *time.Time) {
	t.Helper()
	s, err := OpenStore(filepath.Join(t.TempDir(), "judgements.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	clock := time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return clock }
	return s, &clock
}

func add(t *testing.T, s *Store, at time.Time, r Record) int64 {
	t.Helper()
	r.Time = at
	if r.Used == "" {
		r.Used = UsedJudge
	}
	if r.Version == "" {
		r.Version = "v1"
	}
	id, err := s.Add(r)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

// The list is newest first, is paged by a cursor, and is narrowed by kind, by
// used and why, by model and by what the decision was applied to.
func TestTheListIsPagedAndFiltered(t *testing.T) {
	s, clock := openTest(t)
	base := *clock
	for i := 0; i < 7; i++ {
		r := Record{Kind: "chat-title", Model: "m1", EffectKind: "chat", EffectID: "c1"}
		if i%3 == 0 {
			r.Kind, r.Model, r.Used, r.Why, r.EffectID = "set-aside", "m2", UsedRule, WhyLate, "c2"
		}
		add(t, s, base.Add(time.Duration(i)*time.Minute), r)
	}
	page, next, err := s.List(Filter{Limit: 3})
	if err != nil || len(page) != 3 || next == 0 || page[0].ID != 7 || page[2].ID != 5 {
		t.Fatalf("first page %v next %d %v", ids(page), next, err)
	}
	page, next, _ = s.List(Filter{Limit: 3, Before: next})
	if len(page) != 3 || page[0].ID != 4 || next == 0 {
		t.Fatalf("second page %v next %d", ids(page), next)
	}
	page, next, _ = s.List(Filter{Limit: 3, Before: next})
	if len(page) != 1 || page[0].ID != 1 || next != 0 {
		t.Fatalf("last page %v next %d", ids(page), next)
	}
	for name, c := range map[string]struct {
		f    Filter
		want []int64
	}{
		"kind":   {Filter{Kind: "set-aside"}, []int64{7, 4, 1}},
		"used":   {Filter{Used: UsedRule}, []int64{7, 4, 1}},
		"why":    {Filter{Why: WhyLate}, []int64{7, 4, 1}},
		"model":  {Filter{Model: "m1"}, []int64{6, 5, 3, 2}},
		"effect": {Filter{EffectKind: "chat", EffectID: "c2"}, []int64{7, 4, 1}},
		"time":   {Filter{From: base.Add(2 * time.Minute), To: base.Add(5 * time.Minute)}, []int64{5, 4, 3}},
		"none":   {Filter{Kind: "nothing"}, nil},
	} {
		got, _, err := s.List(c.f)
		if err != nil || !same(ids(got), c.want) {
			t.Errorf("%s: %v want %v (%v)", name, ids(got), c.want, err)
		}
	}
	if r, err := s.Get(4); err != nil || r.ID != 4 {
		t.Fatalf("get: %+v %v", r, err)
	}
	if _, err := s.Get(99); err == nil {
		t.Fatal("a record that is not there was found")
	}
}

func ids(rs []Record) []int64 {
	var out []int64
	for _, r := range rs {
		out = append(out, r.ID)
	}
	return out
}

func same(a, b []int64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// The summary totals by kind and model, with average and longest time, how often
// the judge differed from the rule, and the run of fallbacks the widget warns by.
func TestTheSummaryTotalsWhatTheGraphAndTheWidgetRead(t *testing.T) {
	s, clock := openTest(t)
	base := *clock
	add(t, s, base.Add(-48*time.Hour), Record{Kind: "chat-title", Model: "m1", ElapsedMs: 999, Out: `"old"`, RuleAnswer: `"old"`})
	add(t, s, base, Record{Kind: "chat-title", Model: "m1", ElapsedMs: 100, CostUnit: "tokens", CostIn: 30, CostOut: 5, Out: `"A"`, RuleAnswer: `"B"`})
	add(t, s, base.Add(time.Minute), Record{Kind: "chat-title", Model: "m1", ElapsedMs: 300, CostUnit: "tokens", CostIn: 20, CostOut: 5, Out: `"A"`, RuleAnswer: `"A"`})
	id := add(t, s, base.Add(2*time.Minute), Record{Kind: "set-aside", Model: "m2", Used: UsedRule, Why: WhyLate, ElapsedMs: 800, CostUnit: "tokens"})
	if err := s.Correct(id, "[1]"); err != nil {
		t.Fatal(err)
	}
	sum, err := s.Summarize(base.Add(-time.Hour), base.Add(time.Hour))
	if err != nil || sum.Calls != 3 || sum.Fallbacks != 1 || len(sum.Groups) != 2 {
		t.Fatalf("summary %+v %v", sum, err)
	}
	title, aside := sum.Groups[0], sum.Groups[1]
	if title.Kind != "chat-title" || title.Calls != 2 || title.AvgMs != 200 || title.MaxMs != 300 || title.Disagreed != 1 || title.CostIn != 50 || title.CostOut != 10 || title.CostUnit != "tokens" {
		t.Fatalf("titles %+v", title)
	}
	if aside.Kind != "set-aside" || aside.Fallbacks != 1 || aside.Corrected != 1 {
		t.Fatalf("set-aside %+v", aside)
	}
	if sum.Latest == nil || !sum.Latest.Equal(base.Add(2*time.Minute)) || sum.Streak != 1 {
		t.Fatalf("latest %v streak %d", sum.Latest, sum.Streak)
	}
	// The streak counts the fallbacks since the judge last answered, and leaves
	// out the person's off switch.
	add(t, s, base.Add(3*time.Minute), Record{Kind: "chat-title", Used: UsedRule, Why: WhyOff})
	add(t, s, base.Add(4*time.Minute), Record{Kind: "chat-title", Used: UsedRule, Why: WhyError})
	if sum, _ = s.Summarize(base.Add(-time.Hour), base.Add(time.Hour)); sum.Streak != 2 {
		t.Fatalf("streak %d", sum.Streak)
	}
	add(t, s, base.Add(5*time.Minute), Record{Kind: "chat-title"})
	if sum, _ = s.Summarize(base.Add(-time.Hour), base.Add(time.Hour)); sum.Streak != 0 {
		t.Fatalf("streak %d after the judge answered", sum.Streak)
	}
}

// A correction is kept with its time, and a record that is not there cannot be
// corrected.
func TestACorrectionIsKept(t *testing.T) {
	s, _ := openTest(t)
	id := add(t, s, time.Now(), Record{Kind: "chat-title"})
	if err := s.Correct(id, `"Mine"`); err != nil {
		t.Fatal(err)
	}
	r, err := s.Get(id)
	if err != nil || r.Correction != `"Mine"` || r.CorrectedAt == nil {
		t.Fatalf("%+v %v", r, err)
	}
	if err := s.Correct(99, "x"); err == nil {
		t.Fatal("a missing record was corrected")
	}
	// A record that was not corrected leaves the time out of its JSON.
	other := add(t, s, time.Now(), Record{Kind: "chat-title"})
	fresh, _ := s.Get(other)
	if raw, _ := json.Marshal(fresh); strings.Contains(string(raw), "correctedAt") {
		t.Fatalf("an uncorrected record says when it was corrected: %s", raw)
	}
}

// Records older than the retention are dropped, and the person sets the
// retention.
func TestOldRecordsAreDroppedByTheRetention(t *testing.T) {
	s, clock := openTest(t)
	add(t, s, clock.Add(-100*24*time.Hour), Record{Kind: "chat-title"})
	add(t, s, clock.Add(-40*24*time.Hour), Record{Kind: "chat-title"})
	add(t, s, *clock, Record{Kind: "chat-title"})
	if err := s.Prune(); err != nil {
		t.Fatal(err)
	}
	if rows, _, _ := s.List(Filter{}); len(rows) != 2 {
		t.Fatalf("%d records after the default retention", len(rows))
	}
	s.SetRetention(30 * 24 * time.Hour)
	if err := s.Prune(); err != nil {
		t.Fatal(err)
	}
	if rows, _, _ := s.List(Filter{}); len(rows) != 1 {
		t.Fatalf("%d records after a shorter retention", len(rows))
	}
	s.SetRetention(0)
	if s.retention != DefaultRetention {
		t.Fatal("zero did not restore the default")
	}
}

// The typical cost of a provider's calls is the median of those that cost something
// in its unit, with their number; a call that cost nothing, one of unknown cost, one in
// another unit and one of another provider are left out.
func TestTypicalCostIsTheMedianOfTheCallsThatCostSomething(t *testing.T) {
	s, clock := openTest(t)
	if m, n, err := s.TypicalCost("codex", "tokens"); err != nil || m != 0 || n != 0 {
		t.Fatalf("nothing recorded: %d %d %v", m, n, err)
	}
	for _, in := range []int{30000, 26000, 41000} {
		add(t, s, *clock, Record{Kind: "k", Provider: "codex", CostUnit: "tokens", CostIn: in})
	}
	add(t, s, *clock, Record{Kind: "k", Provider: "codex", CostUnit: "tokens"})
	add(t, s, *clock, Record{Kind: "k", Provider: "codex", CostUnit: UnitUnknown})
	add(t, s, *clock, Record{Kind: "k", Provider: "codex", CostUnit: "acu", CostIn: 9})
	add(t, s, *clock, Record{Kind: "k", Provider: "claude", CostUnit: "tokens", CostIn: 1})
	if m, n, err := s.TypicalCost("codex", "tokens"); err != nil || m != 30000 || n != 3 {
		t.Fatalf("odd count: %d %d %v", m, n, err)
	}
	add(t, s, *clock, Record{Kind: "k", Provider: "codex", CostUnit: "tokens", CostIn: 20000, CostOut: 2000})
	if m, n, err := s.TypicalCost("codex", "tokens"); err != nil || m != (26000+30000)/2 || n != 4 {
		t.Fatalf("even count: %d %d %v", m, n, err)
	}
}
