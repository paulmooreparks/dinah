package guidepin

import (
	"errors"
	"strings"
	"testing"
)

// TestCarriesAnswersTheSentenceItIsGiven pins the accepting case beside the
// two refusals, because a Carries that refused everything would satisfy the
// refusing cases on its own and prove nothing.
func TestCarriesAnswersTheSentenceItIsGiven(t *testing.T) {
	if err := Carries("references", ARestoredColumnLandsAtTheEndOfTheOrder); err != nil {
		t.Errorf("the references guide carries this sentence and Carries refuses it: %v", err)
	}
}

// TestCarriesToleratesAReWrapOfTheGuidesSource holds the folding rule. The
// guides are hard-wrapped, so a pinned sentence spans several source lines
// and a re-wrap must not read as a deleted sentence.
func TestCarriesToleratesAReWrapOfTheGuidesSource(t *testing.T) {
	rewrapped := strings.ReplaceAll(ARestoredColumnLandsAtTheEndOfTheOrder, " ", "\n   ")
	if rewrapped == ARestoredColumnLandsAtTheEndOfTheOrder {
		t.Fatal("the re-wrap left the sentence unchanged, so this case tests nothing")
	}
	if err := Carries("references", rewrapped); err != nil {
		t.Errorf("a re-wrapped spelling of a carried sentence is refused: %v", err)
	}
}

// TestCarriesRefusesAnEmptySentence stops a pin naming nothing from passing.
// An empty needle is a substring of every haystack, so the plain containment
// answer here would be yes.
func TestCarriesRefusesAnEmptySentence(t *testing.T) {
	for _, sentence := range []string{"", "   ", "\n\t "} {
		// errors.Is rather than a plain non-nil check. An empty pin against a
		// guide that does exist would also be refused by the containment test
		// below it, so a test asking only whether some error came back cannot
		// tell this refusal from that accident.
		if err := Carries("references", sentence); !errors.Is(err, ErrNoSentence) {
			t.Errorf("Carries answers %v to the empty pin %q rather than refusing it as a pin naming nothing", err, sentence)
		}
	}
}

// TestCarriesRefusesAnUnknownTopic stops a pin naming a guide this build does
// not serve from passing by reading an empty guide.
func TestCarriesRefusesAnUnknownTopic(t *testing.T) {
	// errors.Is rather than a plain non-nil check, for the reason the
	// sentinels are declared: an unknown topic yields an empty guide text and
	// the containment test then answers no on its own, so a Carries with no
	// topic refusal at all would still return an error here.
	if err := Carries("no-such-guide", ARestoredColumnLandsAtTheEndOfTheOrder); !errors.Is(err, ErrUnknownTopic) {
		t.Errorf("Carries answers %v to a pin naming a guide this build does not serve rather than refusing the topic", err)
	}
}

// TestPinnedNamesTheReferencesGuideAndCopiesItsRoster holds the roster's own
// shape: five statements, every one of them naming a guide and a proving
// test, and a returned slice a caller cannot write back through.
func TestPinnedNamesTheReferencesGuideAndCopiesItsRoster(t *testing.T) {
	roster := Pinned()
	if len(roster) != 5 {
		t.Fatalf("the roster holds %d statements and the references guide's archive section makes five claims", len(roster))
	}
	for _, statement := range roster {
		if statement.Topic == "" || statement.Text == "" || statement.Provenance == "" {
			t.Errorf("a pinned statement is incomplete: %+v", statement)
		}
	}
	roster[0].Text = "overwritten"
	if Pinned()[0].Text == "overwritten" {
		t.Error("Pinned hands out the roster itself, so a caller can rewrite what every other caller reads")
	}
}
