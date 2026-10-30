package judge

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"studio/internal/appletcontext"
)

// Calibration chooses the judge's model by measurement and not by guess
// (docs/plans/judgement-agents.md, step 5). The provider reports the models there
// are; the first one listed is asked which few suit short judgement calls; each
// candidate is run through a fixed set of calls whose right answers are known; and
// the model that answers every one correctly, and fastest, becomes the judge. The
// calibration calls are ordinary recorded calls that use the pool and count
// against the budget, with the effect kind "calibration" and the model as its id.

// EffectCalibration is the effect kind of the calls calibration makes.
const EffectCalibration = "calibration"

// WarmUpDeadline is how long a model has to load before it is scored: ten times the
// three seconds the kinds of call are given, since a local model's first call takes
// many seconds and the scored calls must not be spoiled by it.
const WarmUpDeadline = LateFactor * 3 * time.Second

// Case is one call of the fixed set, and what its right answer is.
type CaseInfo struct {
	Kind    string `json:"kind"`
	Name    string `json:"name"`
	Input   string `json:"input"`
	Accepts string `json:"accepts"`
}

type caseOutcome struct {
	passed    bool
	used, why string
	answer    string
	elapsedMs int64
}

type caseEntry struct {
	info CaseInfo
	run  func(ctx context.Context, j *Judges, effect Effect) caseOutcome
}

var (
	caseMu  sync.Mutex
	caseSet []caseEntry
)

// DeclareCase adds a call to the fixed set that calibration scores a model by. The
// call is the kind's, with the input given, and the model is right when it answers
// with the judge's answer and accept says the answer is one of the right ones;
// accepts says in words what a right answer is, so the set can be reviewed.
func DeclareCase[In, Out any](k *Kind[In, Out], name string, in In, accepts string, accept func(Out) bool) {
	input, _ := json.Marshal(in)
	entry := caseEntry{
		info: CaseInfo{Kind: k.Name, Name: name, Input: string(input), Accepts: accepts},
		run: func(ctx context.Context, j *Judges, effect Effect) caseOutcome {
			start := time.Now()
			r := Call(ctx, j, k, in, effect)
			return caseOutcome{passed: r.Used == UsedJudge && accept(r.Out), used: r.Used, why: r.Why, answer: encode(r.Out), elapsedMs: time.Since(start).Milliseconds()}
		},
	}
	caseMu.Lock()
	defer caseMu.Unlock()
	caseSet = append(caseSet, entry)
}

// Cases is the fixed set, in the order it was declared.
func Cases() []CaseInfo {
	caseMu.Lock()
	defer caseMu.Unlock()
	out := make([]CaseInfo, 0, len(caseSet))
	for _, c := range caseSet {
		out = append(out, c.info)
	}
	return out
}

// KindsWithoutCases lists the declared kinds that the fixed set does not test, so
// a kind cannot be added that calibration would give a judge for without testing
// it.
func KindsWithoutCases() []string {
	has := map[string]bool{}
	for _, c := range Cases() {
		has[c.Kind] = true
	}
	var out []string
	for _, k := range Kinds() {
		if !has[k.Name] {
			out = append(out, k.Name)
		}
	}
	return out
}

type candidatesIn struct {
	Models []string `json:"models" doc:"the names of the models installed, as the provider reports them"`
}

type candidatesOut struct {
	Candidates []string `json:"candidates"`
}

// candidatesKind is the question the first model listed is asked: which models
// suit short judgement calls. It establishes no fact. The list is the provider's,
// and the choice among them is taste, which the check keeps to the list.
var candidatesKind = Declare(Kind[candidatesIn, candidatesOut]{
	Name:     "judge-candidates",
	Describe: "Tries every model the provider reports.",
	System:   "You choose which of a list of language models are likely to suit short, simple judgement calls, such as writing a title for a chat. Prefer fast, capable general models, and leave out models for other work. Choose a few, using names from the list only.",
	Form:     JSONForm[candidatesOut]("an object with candidates, a list of model names taken from the list you were given"),
	Check: func(in candidatesIn, out candidatesOut) error {
		if len(out.Candidates) == 0 {
			return fmt.Errorf("no model was chosen")
		}
		for _, name := range out.Candidates {
			if !slices.Contains(in.Models, name) {
				return fmt.Errorf("%q is not one of the models listed", name)
			}
		}
		return nil
	},
	Deadline: 3 * time.Second,
	Rule:     func(in candidatesIn) candidatesOut { return candidatesOut{Candidates: in.Models} },
})

func init() {
	DeclareCase(candidatesKind, "chooses from the list", candidatesIn{Models: []string{"alpha:cloud", "beta:4b", "gamma-embed", "delta:cloud"}},
		"a list of one or more names, every one taken from the input", func(out candidatesOut) bool { return len(out.Candidates) > 0 })
}

// Model is a model the provider reports. Resident says that running it loads
// nothing new into memory: it runs in the provider's cloud, or is already loaded.
type Model struct {
	Name     string
	Resident bool
}

// ModelReport is how one model did.
type ModelReport struct {
	Name      string `json:"name"`
	Candidate bool   `json:"candidate"`
	// Loaded says this run loaded the model into memory, which only a run the
	// person started may do.
	Loaded   bool   `json:"loaded"`
	Passed   int    `json:"passed"`
	Total    int    `json:"total"`
	MedianMs int64  `json:"medianMs"`
	Why      string `json:"why"`
}

// Report is what a run found. State is "chosen" or "none", or "stopped" when the
// budget did not allow the run to finish.
type Report struct {
	At       time.Time     `json:"at"`
	Provider string        `json:"provider"`
	State    string        `json:"state"`
	Chosen   string        `json:"chosen"`
	Note     string        `json:"note"`
	Models   []ModelReport `json:"models"`
}

func (j *Judges) trial(x Judge, provider, unit string) *Judges {
	j.mu.Lock()
	defer j.mu.Unlock()
	return &Judges{store: j.store, judge: x, enabled: true, off: map[string]bool{}, provider: provider, unit: unit, budgets: j.budgets, period: j.period,
		pool: j.pool, now: j.now, logf: j.logf, onRec: j.onRec}
}

// warmUp makes one small call that loads a model, with WarmUpDeadline to do it in,
// and records it as a call of its own. Its answer is not used.
func (j *Judges) warmUp(ctx context.Context, x Judge, provider, unit, model string) {
	start := time.Now()
	rec := Record{Time: j.now(), Kind: "calibration-warmup", Input: "{}", RuleAnswer: `""`, Out: `""`, Used: UsedRule, Provider: provider, EffectKind: EffectCalibration, EffectID: model,
		DeadlineMs: WarmUpDeadline.Milliseconds()}
	dctx, cancel := context.WithTimeout(ctx, WarmUpDeadline)
	defer cancel()
	release, got := j.pool.acquire(dctx, WarmUpDeadline)
	if !got {
		rec.Why = WhyLate
		j.record(rec)
		return
	}
	defer release()
	resp, err := x.Ask(dctx, Request{System: "Answer with the single word ready.", Input: "{}", MaxTokens: 5})
	rec.ElapsedMs = time.Since(start).Milliseconds()
	rec.Model, rec.Temperature, rec.Seed = resp.Model, resp.Temperature, resp.Seed
	rec.CostUnit, rec.CostIn, rec.CostOut = costUnit(unit, resp.Cost), resp.Cost.In, resp.Cost.Out
	rec.Answer = appletcontext.Redact(resp.Text)
	switch {
	case err != nil && dctx.Err() != nil:
		rec.Why = WhyLate
	case err != nil:
		rec.Why, rec.Answer = WhyError, appletcontext.Redact(err.Error())
	default:
		rec.Used = UsedJudge
	}
	j.record(rec)
}

// Calibrate runs the fixed set against the models and reports which is the best
// judge. build makes the judge for a model. A model is chosen only if it answers
// every case correctly, and then the fastest by the median time of its cases wins.
// The run stops, and chooses nothing, if the budget refuses a call or ctx ends.
func Calibrate(ctx context.Context, j *Judges, provider, unit string, models []Model, build func(name string) Judge) Report {
	report := Report{At: j.now(), Provider: provider, State: "none", Models: []ModelReport{}}
	if len(models) == 0 {
		report.Note = "The provider reports no models."
		return report
	}
	names := make([]string, len(models))
	for i, m := range models {
		names[i] = m.Name
	}
	// The first model listed is asked which suit.
	first := models[0]
	asker := j.trial(build(first.Name), provider, unit)
	j.warmUp(ctx, asker.judge, provider, unit, first.Name)
	r := Call(ctx, asker, candidatesKind, candidatesIn{Models: names}, Effect{Kind: EffectCalibration, ID: first.Name})
	candidates := r.Out.Candidates
	if r.Used == UsedJudge {
		report.Note = fmt.Sprintf("%s, the first model listed, suggested %d of %d.", first.Name, len(candidates), len(names))
	} else {
		report.Note = fmt.Sprintf("%s, the first model listed, could not suggest any (%s), so every model was tried.", first.Name, r.Why)
	}
	isCandidate := map[string]bool{}
	for _, c := range candidates {
		isCandidate[c] = true
	}
	for _, m := range models {
		report.Models = append(report.Models, ModelReport{Name: m.Name, Candidate: isCandidate[m.Name], Loaded: !m.Resident})
	}
	caseMu.Lock()
	set := slices.Clone(caseSet)
	caseMu.Unlock()
	type scored struct {
		idx    int
		median int64
	}
	var passers []scored
	for i, m := range models {
		row := &report.Models[i]
		if !row.Candidate {
			row.Why = "not suggested"
			continue
		}
		row.Total = len(set)
		trial := j.trial(build(m.Name), provider, unit)
		j.warmUp(ctx, trial.judge, provider, unit, m.Name)
		var times []int64
		why := ""
		for _, c := range set {
			if ctx.Err() != nil {
				report.State, report.Note = "stopped", report.Note+" The run was stopped."
				return report
			}
			out := c.run(ctx, trial, Effect{Kind: EffectCalibration, ID: m.Name})
			if out.why == WhyBudget {
				report.State, report.Note = "stopped", report.Note+" The run stopped because the budget did not allow the next call."
				return report
			}
			times = append(times, out.elapsedMs)
			if out.passed {
				row.Passed++
			} else if why == "" {
				switch {
				case out.used == UsedRule:
					why = "failed " + c.info.Name + ": " + out.why
				default:
					why = "failed " + c.info.Name + ": answered " + out.answer
				}
			}
		}
		sort.Slice(times, func(a, b int) bool { return times[a] < times[b] })
		if len(times) > 0 {
			row.MedianMs = times[len(times)/2]
		}
		row.Why = why
		if row.Passed == row.Total && row.Total > 0 {
			passers = append(passers, scored{i, row.MedianMs})
		}
	}
	if len(passers) == 0 {
		report.Note += " No model answered every case correctly, so none was chosen and every call keeps using its rule."
		return report
	}
	best := passers[0]
	for _, p := range passers[1:] {
		if p.median < best.median {
			best = p
		}
	}
	report.State, report.Chosen = "chosen", models[best.idx].Name
	report.Models[best.idx].Why = "chosen: every case correct, and the fastest"
	for _, p := range passers {
		if p.idx != best.idx {
			report.Models[p.idx].Why = "every case correct, not the fastest"
		}
	}
	report.Note = strings.TrimSpace(report.Note)
	return report
}
