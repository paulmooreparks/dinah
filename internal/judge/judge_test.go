package judge

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type titleIn struct {
	Line string `json:"line" doc:"the first line of what the person wrote"`
}

// shortTitle is a test kind whose rule cuts the line and whose check refuses a
// title that repeats the whole line.
var shortTitle = Declare(Kind[titleIn, string]{
	Name:        "test-title",
	Describe:    "Uses the first line, cut short.",
	Correctable: true,
	System:      "You write short titles.",
	Form:        TextForm(20),
	Check: func(in titleIn, out string) error {
		if out == in.Line {
			return errors.New("the title is the whole line")
		}
		return nil
	},
	Deadline: 200 * time.Millisecond,
	Rule: func(in titleIn) string {
		if len(in.Line) > 10 {
			return in.Line[:10]
		}
		return in.Line
	},
})

type answerIn struct {
	Items []string `json:"items" doc:"the things to choose among"`
}

type pick struct {
	Chosen []int `json:"chosen"`
}

var picker = Declare(Kind[answerIn, pick]{
	Name:     "test-pick",
	Describe: "Takes the first item.",
	System:   "You pick items.",
	Form:     JSONForm[pick]("an object with chosen, a list of item numbers"),
	Deadline: 200 * time.Millisecond,
	Rule:     func(answerIn) pick { return pick{Chosen: []int{0}} },
})

// fake is a judge whose answer a test sets.
type fake struct {
	mu    sync.Mutex
	asked []Request
	reply func(ctx context.Context, r Request) (Response, error)
}

func (f *fake) Ask(ctx context.Context, r Request) (Response, error) {
	f.mu.Lock()
	f.asked = append(f.asked, r)
	f.mu.Unlock()
	return f.reply(ctx, r)
}

func (f *fake) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.asked)
}

func says(text string) *fake {
	return &fake{reply: func(context.Context, Request) (Response, error) {
		return Response{Text: text, Model: "m1", Seed: 7, Cost: Cost{In: 30, Out: 4}}, nil
	}}
}

func newSet(t *testing.T, j Judge) *Judges {
	t.Helper()
	s, err := OpenStore(filepath.Join(t.TempDir(), "judgements.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	set := New(Options{Store: s, Judge: j, PoolMin: 1, PoolMax: 1})
	if j != nil {
		set.SetJudge(j, "test", "tokens")
	}
	return set
}

func only(t *testing.T, j *Judges) Record {
	t.Helper()
	rows, _, err := j.Store().List(Filter{})
	if err != nil || len(rows) != 1 {
		t.Fatalf("%d records: %v", len(rows), err)
	}
	return rows[0]
}

// A judge's good answer is used, and the record keeps everything about the call:
// what the judge was shown, what it said, the rule's answer for comparison, the
// settings and the cost.
func TestAGoodAnswerIsUsedAndRecorded(t *testing.T) {
	f := says("  Parser  release \n")
	j := newSet(t, f)
	r := Call(context.Background(), j, shortTitle, titleIn{Line: "Please tag the parser release"}, Effect{Kind: "chat", ID: "window-1"})
	if r.Out != "Parser release" || r.Used != UsedJudge || r.Why != "" || r.ID == 0 {
		t.Fatalf("result %+v", r)
	}
	rec := only(t, j)
	if rec.Kind != "test-title" || rec.Version != shortTitle.Version() || rec.Input != `{"line":"Please tag the parser release"}` || rec.Answer != "  Parser  release \n" ||
		rec.RuleAnswer != `"Please tag"` || rec.Out != `"Parser release"` || rec.Used != UsedJudge || rec.Provider != "test" || rec.Model != "m1" ||
		rec.Seed != 7 || rec.CostUnit != "tokens" || rec.CostIn != 30 || rec.CostOut != 4 || rec.EffectKind != "chat" || rec.EffectID != "window-1" ||
		rec.DeadlineMs != 200 || rec.Time.IsZero() {
		t.Fatalf("record %+v", rec)
	}
	if got := f.asked[0]; !strings.Contains(got.System, "You write short titles.") || !strings.Contains(got.System, "plain text, one line, at most 20 characters") || got.Input != rec.Input || got.JSON {
		t.Fatalf("the judge was asked %+v", got)
	}
}

// Off, with no judge, and with no set at all, the rule answers and the record
// says why.
func TestWithoutAJudgeTheRuleAnswers(t *testing.T) {
	in := titleIn{Line: "Please tag the parser release"}
	f := says("Parser release")
	j := newSet(t, f)
	j.SetEnabled(false)
	if r := Call(context.Background(), j, shortTitle, in, Effect{}); r.Out != "Please tag" || r.Used != UsedRule || r.Why != WhyOff {
		t.Fatalf("off: %+v", r)
	}
	j.SetEnabled(true)
	j.SetJudge(nil, "", "")
	if r := Call(context.Background(), j, shortTitle, in, Effect{}); r.Out != "Please tag" || r.Why != WhyNoJudge {
		t.Fatalf("no judge: %+v", r)
	}
	if r := Call(context.Background(), nil, shortTitle, in, Effect{}); r.Out != "Please tag" || r.Why != WhyNoJudge || r.ID != 0 {
		t.Fatalf("no set: %+v", r)
	}
	if f.count() != 0 {
		t.Fatal("a judge was asked though it was off or absent")
	}
	rows, _, _ := j.Store().List(Filter{})
	if len(rows) != 2 || rows[0].Why != WhyNoJudge || rows[1].Why != WhyOff || rows[1].Input == "" || rows[1].RuleAnswer != `"Please tag"` {
		t.Fatalf("records %+v", rows)
	}
}

// A judge that is late, whether it honors its context or not, costs the call no
// more than its deadline, and the rule answers.
func TestALateJudgeFallsBackAtTheDeadline(t *testing.T) {
	release := make(chan struct{})
	defer close(release)
	for name, reply := range map[string]func(context.Context, Request) (Response, error){
		"honors": func(ctx context.Context, _ Request) (Response, error) {
			<-ctx.Done()
			return Response{}, ctx.Err()
		},
		"ignores": func(context.Context, Request) (Response, error) {
			<-release
			return Response{Text: "Too late"}, nil
		},
	} {
		j := newSet(t, &fake{reply: reply})
		start := time.Now()
		r := Call(context.Background(), j, shortTitle, titleIn{Line: "Please tag the parser release"}, Effect{})
		if took := time.Since(start); took > 2*time.Second || took < 150*time.Millisecond {
			t.Fatalf("%s: the call took %v", name, took)
		}
		if r.Out != "Please tag" || r.Used != UsedRule || r.Why != WhyLate {
			t.Fatalf("%s: %+v", name, r)
		}
		if rec := only(t, j); rec.Why != WhyLate || rec.ElapsedMs < 150 {
			t.Fatalf("%s: record %+v", name, rec)
		}
	}
}

// A call that waits in the queue past its deadline is answered by the rule
// without being sent, and a call that has its turn in time is sent.
func TestACallThatExpiresInTheQueueIsNotSent(t *testing.T) {
	release := make(chan struct{})
	var started atomic.Int32
	f := &fake{reply: func(_ context.Context, _ Request) (Response, error) {
		started.Add(1)
		<-release
		return Response{Text: "Held"}, nil
	}}
	j := newSet(t, f)
	first := make(chan Result[string], 1)
	go func() {
		first <- Call(context.Background(), j, shortTitle, titleIn{Line: "The first one, long enough"}, Effect{})
	}()
	deadline := time.Now().Add(2 * time.Second)
	for started.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	r := Call(context.Background(), j, shortTitle, titleIn{Line: "The second one, long enough"}, Effect{})
	if r.Why != WhyLate || r.Out != "The second" {
		t.Fatalf("the queued call: %+v", r)
	}
	if f.count() != 1 {
		t.Fatalf("the judge was asked %d times, and the expired call must not be sent", f.count())
	}
	close(release)
	<-first
}

// An answer in the wrong form, or that fails the kind's check, or an error from
// the judge, is not used; the record keeps the answer for the person to see.
func TestAnAnswerThatIsWrongIsNotUsed(t *testing.T) {
	in := titleIn{Line: "Please tag the parser"}
	cases := []struct {
		name  string
		judge Judge
		why   string
		kept  string
	}{
		{"too long", says("This title is much longer than twenty characters"), WhyForm, "This title is much longer than twenty characters"},
		{"empty", says("   "), WhyForm, "   "},
		{"fails the check", says("Tag the release now"), WhyCheck, "Tag the release now"},
		{"error", &fake{reply: func(context.Context, Request) (Response, error) { return Response{}, errors.New("connection refused") }}, WhyError, "connection refused"},
	}
	for _, c := range cases {
		j := newSet(t, c.judge)
		in := in
		if c.why == WhyCheck {
			in.Line = "Tag the release now"
		}
		r := Call(context.Background(), j, shortTitle, in, Effect{})
		if r.Used != UsedRule || r.Why != c.why || r.Out != shortTitle.Rule(in) {
			t.Fatalf("%s: %+v", c.name, r)
		}
		if rec := only(t, j); rec.Why != c.why || rec.Answer != c.kept || rec.Used != UsedRule {
			t.Fatalf("%s: record %+v", c.name, rec)
		}
	}
	// A JSON answer must decode into the declared shape, with nothing else.
	for _, text := range []string{`{"chosen":[1],"extra":true}`, `not json`, `{"chosen":[1]} and more`, `[1]`} {
		j := newSet(t, says(text))
		if r := Call(context.Background(), j, picker, answerIn{Items: []string{"a", "b"}}, Effect{}); r.Why != WhyForm || len(r.Out.Chosen) != 1 || r.Out.Chosen[0] != 0 {
			t.Fatalf("%q: %+v", text, r)
		}
	}
	j := newSet(t, says(`{"chosen":[1,0]}`))
	if r := Call(context.Background(), j, picker, answerIn{Items: []string{"a", "b"}}, Effect{}); r.Used != UsedJudge || len(r.Out.Chosen) != 2 {
		t.Fatalf("a good JSON answer: %+v", r)
	}
}

// An applet context is removed from what the judge is shown, from what is
// recorded and from the answer, whatever put it there.
func TestAnAppletContextNeverReachesTheJudgeOrTheRecord(t *testing.T) {
	secret := "dinah-applet-v1:5QAHWLZE3I6FEG3ZDG634FNDCD.53G4C65FBRVPZX77KYPVNNDMAH"
	f := says("Title " + secret)
	j := newSet(t, f)
	Call(context.Background(), j, shortTitle, titleIn{Line: "Use " + secret + " here"}, Effect{})
	if strings.Contains(f.asked[0].Input, "5QAHWLZE") {
		t.Fatalf("the judge was shown a context: %s", f.asked[0].Input)
	}
	rec := only(t, j)
	for _, field := range []string{rec.Input, rec.Answer, rec.RuleAnswer, rec.Out} {
		if strings.Contains(field, "5QAHWLZE") {
			t.Fatalf("the record holds a context: %+v", rec)
		}
	}
}

// The same input is rendered to the same request twice.
func TestTheSameInputIsTheSameRequest(t *testing.T) {
	f := says("Title")
	j := newSet(t, f)
	for i := 0; i < 2; i++ {
		Call(context.Background(), j, shortTitle, titleIn{Line: "Same line"}, Effect{})
	}
	if f.asked[0] != f.asked[1] {
		t.Fatalf("%+v differs from %+v", f.asked[0], f.asked[1])
	}
}

// The registry lists what a judge may be shown, and a kind's version changes
// when its prompt does.
func TestTheRegistryListsEveryInputAndTheVersionFollowsThePrompt(t *testing.T) {
	var info *Info
	for _, k := range Kinds() {
		if k.Name == "test-title" {
			k := k
			info = &k
		}
	}
	if info == nil || len(info.Input) != 1 || info.Input[0].Name != "line" || info.Input[0].Doc == "" || info.Describe == "" || !info.Correctable || info.DeadlineMs != 200 || info.Version != shortTitle.Version() {
		t.Fatalf("registry entry %+v", info)
	}
	changed := Declare(Kind[titleIn, string]{Name: "test-title-changed", Describe: "x", System: "You write longer titles.", Form: TextForm(20), Deadline: time.Second, Rule: func(titleIn) string { return "" }})
	if changed.Version() == shortTitle.Version() || len(changed.Version()) != 12 {
		t.Fatalf("versions %s and %s", changed.Version(), shortTitle.Version())
	}
	mustPanic := func(name string, f func()) {
		t.Helper()
		defer func() {
			if recover() == nil {
				t.Errorf("%s was accepted", name)
			}
		}()
		f()
	}
	mustPanic("a duplicate", func() {
		Declare(Kind[titleIn, string]{Name: "test-title", Describe: "x", System: "y", Form: TextForm(5), Deadline: time.Second, Rule: func(titleIn) string { return "" }})
	})
	mustPanic("an input field without a doc", func() {
		type bare struct {
			A string `json:"a"`
		}
		Declare(Kind[bare, string]{Name: "test-bare", Describe: "x", System: "y", Form: TextForm(5), Deadline: time.Second, Rule: func(bare) string { return "" }})
	})
	mustPanic("no deadline", func() {
		Declare(Kind[titleIn, string]{Name: "test-nodeadline", Describe: "x", System: "y", Form: TextForm(5), Rule: func(titleIn) string { return "" }})
	})
}
