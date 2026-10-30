package judge

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

type calIn struct {
	Line string `json:"line" doc:"a line to title"`
}

var calKind = Declare(Kind[calIn, string]{
	Name:     "test-calibrated",
	Describe: "Uses the line.",
	System:   "You write titles.",
	Form:     TextForm(40),
	Deadline: 200 * time.Millisecond,
	Rule:     func(in calIn) string { return in.Line },
})

func init() {
	DeclareCase(calKind, "tokenizer", calIn{Line: "Please rename the parser's tokenizer module to lexer"}, "contains lexer or tokenizer",
		func(out string) bool {
			return strings.Contains(strings.ToLower(out), "lexer") || strings.Contains(strings.ToLower(out), "tokenizer")
		})
	DeclareCase(calKind, "nightly", calIn{Line: "Why does the nightly build fail on the Windows runner only?"}, "contains nightly, build or Windows",
		func(out string) bool {
			o := strings.ToLower(out)
			return strings.Contains(o, "nightly") || strings.Contains(o, "build") || strings.Contains(o, "windows")
		})
}

// model is a stand-in for a model with a behaviour.
type model struct {
	name  string
	delay time.Duration
	// bad answers every title with something that is wrong.
	bad bool
	// firstCall is how long the model's very first call takes, as a cold local
	// model's does.
	firstCall time.Duration
	// suggests is what it answers when asked for candidates; nil is every model.
	suggests []string
	// invents makes it suggest a model that is not on the list.
	invents bool
	// clk, when set, is a fake clock that the model's delay advances instead of sleeping, so which model is faster does
	// not depend on how loaded the machine is.
	clk *clock

	mu    sync.Mutex
	calls int
}

func (m *model) Ask(_ context.Context, r Request) (Response, error) {
	m.mu.Lock()
	m.calls++
	first := m.calls == 1
	m.mu.Unlock()
	if first {
		time.Sleep(m.firstCall)
	}
	if m.clk != nil {
		m.clk.advance(m.delay)
	} else {
		time.Sleep(m.delay)
	}
	resp := Response{Model: m.name, Seed: 7, Cost: Cost{In: 10, Out: 2}}
	var in struct {
		Line   string   `json:"line"`
		Models []string `json:"models"`
	}
	_ = json.Unmarshal([]byte(r.Input), &in)
	switch {
	case in.Models != nil:
		list := in.Models
		if m.suggests != nil {
			list = m.suggests
		}
		if m.invents {
			list = []string{"nonexistent:model"}
		}
		b, _ := json.Marshal(map[string]any{"candidates": list})
		resp.Text = string(b)
	case m.bad:
		resp.Text = "Nothing useful"
	case in.Line != "":
		resp.Text = in.Line
		if len(resp.Text) > 38 {
			resp.Text = resp.Text[:38]
		}
	default:
		resp.Text = "ready"
	}
	return resp, nil
}

func registryOf(models ...*model) (func(string) Judge, []Model) {
	byName := map[string]*model{}
	var list []Model
	for _, m := range models {
		byName[m.name] = m
		list = append(list, Model{Name: m.name, Resident: true})
	}
	return func(name string) Judge { return byName[name] }, list
}

func calibrationSet(t *testing.T, c *clock) *Judges {
	t.Helper()
	set := budgeted(t, spendy(10), nil, c)
	return set
}

// The model that answers every case correctly and fastest is chosen; one that
// answers wrongly is not, and the report says why for each.
func TestCalibrationChoosesTheFastestModelThatAnswersEveryCase(t *testing.T) {
	c := &clock{t: monday.Add(48 * time.Hour)}
	set := calibrationSet(t, c)
	build, models := registryOf(&model{name: "fast", clk: c}, &model{name: "slow", delay: 40 * time.Millisecond, clk: c}, &model{name: "wrong", bad: true, clk: c})
	rep := Calibrate(context.Background(), set, "test", "tokens", models, build)
	if rep.State != "chosen" || rep.Chosen != "fast" || len(rep.Models) != 3 {
		t.Fatalf("report %+v", rep)
	}
	by := map[string]ModelReport{}
	for _, m := range rep.Models {
		by[m.Name] = m
	}
	total := len(Cases()) - 0
	if by["fast"].Passed != by["fast"].Total || by["fast"].Total != total || by["slow"].Passed != by["slow"].Total || by["fast"].Why != "chosen: every case correct, and the fastest" ||
		by["slow"].Why != "every case correct, not the fastest" || by["slow"].MedianMs != 40 || by["fast"].MedianMs != 0 {
		t.Fatalf("fast %+v slow %+v", by["fast"], by["slow"])
	}
	if w := by["wrong"]; w.Passed >= w.Total || !strings.HasPrefix(w.Why, "failed ") || w.Loaded {
		t.Fatalf("wrong %+v", w)
	}
	if !strings.Contains(rep.Note, "fast, the first model listed, suggested 3 of 3") {
		t.Fatalf("note %q", rep.Note)
	}
}

// When no model answers every case, none is chosen and the report says so.
func TestCalibrationChoosesNothingWhenNoModelAnswersEveryCase(t *testing.T) {
	set := calibrationSet(t, &clock{t: monday.Add(48 * time.Hour)})
	build, models := registryOf(&model{name: "a", bad: true}, &model{name: "b", bad: true})
	rep := Calibrate(context.Background(), set, "test", "tokens", models, build)
	if rep.State != "none" || rep.Chosen != "" || !strings.Contains(rep.Note, "none was chosen and every call keeps using its rule") {
		t.Fatalf("report %+v", rep)
	}
	if len(Cases()) == 0 || len(rep.Models) != 2 {
		t.Fatalf("report %+v", rep)
	}
}

// Only the models the first one suggests are run, and a suggestion of a model that
// is not on the list is refused, so every model is tried instead.
func TestCalibrationRunsOnlyTheCandidatesAndRefusesAnInventedOne(t *testing.T) {
	c := &clock{t: monday.Add(48 * time.Hour)}
	set := calibrationSet(t, c)
	build, models := registryOf(&model{name: "asker", suggests: []string{"b"}}, &model{name: "b"}, &model{name: "c"})
	rep := Calibrate(context.Background(), set, "test", "tokens", models, build)
	by := map[string]ModelReport{}
	for _, m := range rep.Models {
		by[m.Name] = m
	}
	if by["asker"].Candidate || !by["b"].Candidate || by["c"].Candidate || by["c"].Why != "not suggested" || by["c"].Total != 0 || rep.Chosen != "b" {
		t.Fatalf("report %+v", rep)
	}
	rows, _, _ := set.Store().List(Filter{EffectKind: EffectCalibration, EffectID: "c"})
	if len(rows) != 0 {
		t.Fatalf("a model that was not suggested was called %d times", len(rows))
	}
	set2 := calibrationSet(t, c)
	build2, models2 := registryOf(&model{name: "liar", invents: true}, &model{name: "other"})
	rep2 := Calibrate(context.Background(), set2, "test", "tokens", models2, build2)
	if !rep2.Models[0].Candidate || !rep2.Models[1].Candidate || !strings.Contains(rep2.Note, "so every model was tried") || !strings.Contains(rep2.Note, "check") {
		t.Fatalf("report %+v", rep2)
	}
}

// A model's first call is made to load it and is not scored, so a cold model that
// takes longer than a deadline to load is judged on how it answers.
func TestCalibrationWarmsAModelUpBeforeScoringIt(t *testing.T) {
	set := calibrationSet(t, &clock{t: monday.Add(48 * time.Hour)})
	build, models := registryOf(&model{name: "cold", firstCall: 450 * time.Millisecond})
	rep := Calibrate(context.Background(), set, "test", "tokens", models, build)
	if rep.Chosen != "cold" {
		t.Fatalf("a cold model was not chosen: %+v", rep)
	}
	warm, _, _ := set.Store().List(Filter{Kind: "calibration-warmup"})
	if len(warm) == 0 || warm[len(warm)-1].EffectKind != EffectCalibration || warm[len(warm)-1].Used != UsedJudge {
		t.Fatalf("the warm-up was not recorded: %+v", warm)
	}
}

// The calls are ordinary recorded calls with the calibration effect and the model
// as its id, and they count against the budget; a call the budget refuses stops
// the run, which then chooses nothing.
func TestCalibrationCallsAreRecordedAndBudgeted(t *testing.T) {
	c := &clock{t: monday.Add(48 * time.Hour)}
	set := calibrationSet(t, c)
	build, models := registryOf(&model{name: "only"})
	if rep := Calibrate(context.Background(), set, "test", "tokens", models, build); rep.Chosen != "only" {
		t.Fatalf("report %+v", rep)
	}
	rows, _, _ := set.Store().List(Filter{EffectKind: EffectCalibration, EffectID: "only", Limit: 500})
	kinds := map[string]bool{}
	for _, r := range rows {
		kinds[r.Kind] = true
		if r.Provider != "test" || r.CostIn == 0 && r.Kind != "calibration-warmup" {
			t.Fatalf("record %+v", r)
		}
	}
	if !kinds["calibration-warmup"] || !kinds["judge-candidates"] || !kinds["test-calibrated"] {
		t.Fatalf("kinds recorded %v", kinds)
	}
	if uses := func() int { u, _ := set.BudgetUses(); return len(u) }(); uses != 0 {
		t.Fatalf("a budget appeared")
	}
	zero := budgeted(t, spendy(10), ptr(0), c)
	build, models = registryOf(&model{name: "only"})
	rep := Calibrate(context.Background(), zero, "test", "tokens", models, build)
	if rep.State != "stopped" || rep.Chosen != "" || !strings.Contains(rep.Note, "budget") {
		t.Fatalf("report %+v", rep)
	}
}

// A run whose context ends stops and chooses nothing.
func TestCalibrationStopsWhenItsContextEnds(t *testing.T) {
	set := calibrationSet(t, &clock{t: monday.Add(48 * time.Hour)})
	build, models := registryOf(&model{name: "a"}, &model{name: "b"})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if rep := Calibrate(ctx, set, "test", "tokens", models, build); rep.Chosen != "" || rep.State == "chosen" {
		t.Fatalf("report %+v", rep)
	}
	if rep := Calibrate(context.Background(), set, "test", "tokens", nil, build); rep.State != "none" || !strings.Contains(rep.Note, "no models") {
		t.Fatalf("no models %+v", rep)
	}
}

// A kind that the fixed set does not test is listed, so a kind added later cannot
// be calibrated without being tested.
func TestAKindWithoutACaseIsListed(t *testing.T) {
	without := KindsWithoutCases()
	if slices.Contains(without, "test-calibrated") || slices.Contains(without, "judge-candidates") || !slices.Contains(without, "test-title") {
		t.Fatalf("kinds without a case: %v", without)
	}
	cases := Cases()
	if len(cases) < 3 || cases[0].Kind == "" || cases[0].Input == "" || cases[0].Accepts == "" {
		t.Fatalf("cases %+v", cases)
	}
}
