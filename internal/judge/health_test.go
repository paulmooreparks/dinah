package judge

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// failing is a judge whose every call fails with a provider's words, and which counts the requests it was sent.
func failing(text string) (*fake, *atomic.Int32) {
	var n atomic.Int32
	return &fake{reply: func(context.Context, Request) (Response, error) {
		n.Add(1)
		return Response{}, errors.New(text)
	}}, &n
}

func newClockedSet(t *testing.T, j Judge) (*Judges, *time.Time) {
	t.Helper()
	set := newSet(t, j)
	clock := time.Date(2026, 10, 31, 9, 0, 0, 0, time.UTC)
	set.now = func() time.Time { return clock }
	return set, &clock
}

func call(set *Judges) Result[string] {
	return Call(context.Background(), set, shortTitle, titleIn{Line: "Rename the parser module"}, Effect{Kind: "chat", ID: "c"})
}

// After three errors in a row from a provider it is paused: further calls answer by the rule at once, with the reason, and
// send no request. When the pause is over one request goes, and if it fails too the pause is twice as long; when a request
// is answered the pause is over and the errors are forgotten.
func TestRepeatedErrorsPauseTheProviderWithABackOffAndOneProbe(t *testing.T) {
	f, sent := failing("ollama: you have reached your monthly usage limit, add usage credits (ref: abc-123)")
	set, clock := newClockedSet(t, f)
	for i := 0; i < pauseAfter; i++ {
		if r := call(set); r.Why != WhyError {
			t.Fatalf("call %d: %+v", i, r)
		}
	}
	if sent.Load() != pauseAfter {
		t.Fatalf("%d requests for %d calls", sent.Load(), pauseAfter)
	}
	pauses := set.Pauses()
	if len(pauses) != 1 || pauses[0].Provider != "test" || !pauses[0].Until.Equal(clock.Add(pauseFirst)) || strings.Contains(pauses[0].Reason, "ref:") || !strings.Contains(pauses[0].Reason, "monthly usage limit") {
		t.Fatalf("the pause: %+v", pauses)
	}
	// While it lasts no request is sent, and the record says why.
	for i := 0; i < 5; i++ {
		if r := call(set); r.Why != WhyPaused || r.Used != UsedRule {
			t.Fatalf("a call during the pause: %+v", r)
		}
	}
	if sent.Load() != pauseAfter {
		t.Fatalf("a request was sent during the pause: %d", sent.Load())
	}
	rec, err := set.Store().Get(call(set).ID)
	if err != nil || !strings.HasPrefix(rec.Answer, "paused until ") || !strings.Contains(rec.Answer, "monthly usage limit") {
		t.Fatalf("the record of a paused call: %+v %v", rec, err)
	}
	// When it is over, exactly one probe goes; it fails, and the pause doubles.
	*clock = clock.Add(pauseFirst + time.Second)
	if r := call(set); r.Why != WhyError || sent.Load() != pauseAfter+1 {
		t.Fatalf("the probe: %+v, %d requests", r, sent.Load())
	}
	if pauses = set.Pauses(); len(pauses) != 1 || !pauses[0].Until.Equal(clock.Add(2*pauseFirst)) {
		t.Fatalf("the pause after a failed probe: %+v", pauses)
	}
	// It stops growing at its longest.
	for i := 0; i < 10; i++ {
		*clock = clock.Add(pauseLongest + time.Second)
		call(set)
	}
	if pauses = set.Pauses(); len(pauses) != 1 || !pauses[0].Until.Equal(clock.Add(pauseLongest)) {
		t.Fatalf("the longest pause: %+v", pauses)
	}
}

func TestAProbeThatIsAnsweredEndsThePause(t *testing.T) {
	var broken atomic.Bool
	broken.Store(true)
	f := &fake{reply: func(context.Context, Request) (Response, error) {
		if broken.Load() {
			return Response{}, errors.New("down")
		}
		return Response{Text: "Rename parser"}, nil
	}}
	set, clock := newClockedSet(t, f)
	for i := 0; i < pauseAfter; i++ {
		call(set)
	}
	if len(set.Pauses()) != 1 {
		t.Fatal("not paused")
	}
	broken.Store(false)
	*clock = clock.Add(pauseFirst + time.Second)
	if r := call(set); r.Used != UsedJudge {
		t.Fatalf("the probe: %+v", r)
	}
	if len(set.Pauses()) != 0 {
		t.Fatalf("the pause outlived an answer: %+v", set.Pauses())
	}
	// Two errors are not enough for a new pause: the count started again.
	broken.Store(true)
	call(set)
	call(set)
	if r := call(set); r.Why != WhyError || len(set.Pauses()) != 1 {
		t.Fatalf("the count did not start again: %+v", r)
	}
}

// Only errors pause: slow or malformed answers do not, and the other providers are not touched.
func TestOnlyErrorsPause(t *testing.T) {
	slow := &fake{reply: func(ctx context.Context, r Request) (Response, error) {
		<-ctx.Done()
		return Response{}, ctx.Err()
	}}
	set, _ := newClockedSet(t, slow)
	for i := 0; i < pauseAfter+2; i++ {
		if r := call(set); r.Why != WhyLate {
			t.Fatalf("call %d: %+v", i, r)
		}
	}
	if len(set.Pauses()) != 0 {
		t.Fatalf("late answers paused the provider: %+v", set.Pauses())
	}
}

func TestAShortReasonIsOneLineWithoutARequestReference(t *testing.T) {
	long := ShortReason("a\n\nb (ref: 9f2) " + strings.Repeat("x", 300))
	if strings.ContainsAny(long, "\n") || strings.Contains(long, "ref:") || len([]rune(long)) > 160 {
		t.Fatalf("%q", long)
	}
	if got := ShortReason("quota used up (ref: 1)"); got != "quota used up" {
		t.Fatalf("%q", got)
	}
}

// Setting the judge again, as changing the settings does, tries the provider afresh.
func TestSettingTheJudgeAgainEndsAPause(t *testing.T) {
	f, sent := failing("down")
	set, _ := newClockedSet(t, f)
	for i := 0; i < pauseAfter; i++ {
		call(set)
	}
	if len(set.Pauses()) != 1 {
		t.Fatal("not paused")
	}
	set.SetJudge(f, "test", "tokens")
	if len(set.Pauses()) != 0 {
		t.Fatal("the pause outlived a new judge")
	}
	before := sent.Load()
	if call(set); sent.Load() != before+1 {
		t.Fatal("no request was sent after the judge was set again")
	}
}
