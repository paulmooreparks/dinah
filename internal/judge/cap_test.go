package judge

import (
	"context"
	"strings"
	"testing"
	"time"
)

type capIn struct {
	Text string `json:"text" doc:"some text"`
}

var cappedKind = Declare(Kind[capIn, string]{
	Name: "test-capped", Describe: "Uses the text.", System: "You repeat.", Form: ParagraphsForm(), Deadline: 200 * time.Millisecond,
	Rule: func(in capIn) string { return "rule" }, InputCap: 80, DefaultOff: true,
})

// A kind declares the most input it may send and whether it starts off, and the
// registry says so; a call with more input than the cap is answered by its rule
// without asking the judge, and one within it is sent.
func TestAKindsInputCapIsDeclaredAndEnforced(t *testing.T) {
	var info *Info
	for _, k := range Kinds() {
		if k.Name == "test-capped" {
			k := k
			info = &k
		}
	}
	if info == nil || info.InputCap != 80 || !info.DefaultOff {
		t.Fatalf("registry entry %+v", info)
	}
	f := says("Answer line one\nline two")
	set := newSet(t, f)
	small := Call(context.Background(), set, cappedKind, capIn{Text: "short"}, Effect{})
	if small.Used != UsedJudge || small.Out != "Answer line one\nline two" || f.count() != 1 {
		t.Fatalf("a call within the cap %+v (%d asked)", small, f.count())
	}
	big := Call(context.Background(), set, cappedKind, capIn{Text: strings.Repeat("x", 200)}, Effect{})
	if big.Used != UsedRule || big.Why != WhyError || big.Out != "rule" || f.count() != 1 {
		t.Fatalf("a call over the cap %+v (%d asked)", big, f.count())
	}
	rec, _ := set.Store().Get(big.ID)
	if !strings.Contains(rec.Answer, "more than the 80 this kind may send") {
		t.Fatalf("record %+v", rec)
	}
	// With the judge off, the reason is that, and not the cap.
	set.SetEnabled(false)
	if r := Call(context.Background(), set, cappedKind, capIn{Text: strings.Repeat("x", 200)}, Effect{}); r.Why != WhyOff {
		t.Fatalf("with the judges off %+v", r)
	}
}

func TestTheParagraphsFormTakesSeveralLinesAndRefusesNothingButEmpty(t *testing.T) {
	form := ParagraphsForm()
	if got, err := form.Parse("  First line.\r\nSecond line.\n\n"); err != nil || got != "First line.\nSecond line." {
		t.Fatalf("%q %v", got, err)
	}
	if _, err := form.Parse(" \n "); err == nil {
		t.Fatal("an empty answer was accepted")
	}
}
