package verb

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"dinah/internal/bench"
	"dinah/internal/contract"
)

// ambiguousCardFixture builds the shape dinah-487's spec calls for and answers
// the six identifiers in filing order.
//
// The registry names every claimant of a number, live and archived alike, so
// fx-1 answers with the two live twins and the archived odd one beside them,
// fx-2 with one, and fx-3 with the two archived twins. The archived odd one on
// fx-1's number is what keeps the test from accepting an answer that names
// only the live half, and the lone live card on fx-2 is what keeps the third
// arm below honest, because a filter that closed on the ambiguous case and
// refused every number would pass a test that never resolved anything.
//
// The cards are filed through the verb and archived through the verb, and only
// then are the numbers rewritten in the card-number registry, because add
// allocates the next number above the registry's high-water mark and a
// reference stops resolving the moment its number collides.
func ambiguousCardFixture(t *testing.T) (*harness, []string) {
	t.Helper()
	h := newHarness(t)
	var ids []string
	for _, title := range []string{
		"the live twin", "the other live twin", "the link source",
		"the archived odd one", "the archived twin", "the other archived twin",
	} {
		ref := h.add(title)
		ids = append(ids, h.card(ref).ID)
	}
	for _, id := range ids[3:] {
		if response := h.library.Archive(&Request{Verb: "archive", Actor: "alka", Ref: id}); response.Outcome != contract.OutcomeOK {
			t.Fatalf("archive %s: %s %s", id, response.Outcome, response.Refusal)
		}
		h.reopen()
	}
	// The registry is one file, so the rewrite lands there in one write
	// rather than per card: the live collision sits on fx-1, the lone live
	// card on fx-2, and the archive collision on fx-3, and the half each card
	// stands in is the half the archive step above already put it in.
	lines := []string{
		"1 " + ids[0], "1 " + ids[1], "2 " + ids[2],
		"1 " + ids[3], "3 " + ids[4], "3 " + ids[5],
	}
	if err := bench.WriteNumberLines(filepath.Join(h.library.Bench.Root, bench.CardNumbersName), lines); err != nil {
		t.Fatalf("rewrite the registry: %v", err)
	}
	h.reopen()
	return h, ids
}

// TestTheChangesCardFilterRefusesAnAmbiguousNumber is dinah-487 AC-9's bench
// leg and AC-11's second caller.
//
// The rows come out in the registry's file order, which the fixture wrote, so
// fx-1 carries the two live twins with the archived odd one beside them and
// fx-3 carries the two archived twins. Naming the claimants whole is the point
// the registry route makes over the half-by-half route the below-format tests
// in internal/bench drive: a number with three cards behind it answers with
// three rows, and a caller reaching for any one of them can see it.
func TestTheChangesCardFilterRefusesAnAmbiguousNumber(t *testing.T) {
	h, ids := ambiguousCardFixture(t)
	for _, c := range []struct {
		ref  string
		want []string
	}{
		{ref: "fx-1", want: []string{ids[0], ids[1], ids[3]}},
		{ref: "fx-3", want: []string{ids[4], ids[5]}},
	} {
		id, err := h.library.watchedCard(c.ref)
		if id != "" {
			t.Errorf("%s answered the identifier %s, and an ambiguous number answers none", c.ref, id)
		}
		var refusal *contract.Refusal
		if !errors.As(err, &refusal) || refusal.Name != contract.AmbiguousCard {
			t.Fatalf("%s refused %v and two cards carrying the number make it %s", c.ref, err, contract.AmbiguousCard)
		}
		if got := strings.Split(refusal.Extra["cards"], "\n"); strings.Join(got, ",") != strings.Join(c.want, ",") {
			t.Errorf("%s carries the rows %v and the candidates are %v", c.ref, got, c.want)
		}
	}

	// The third arm is untouched and the filter is not closed on the case it
	// was built for. An archived card answers by its identifier through the
	// second attempt, a well-formed identifier no card file backs is accepted
	// by the third arm, and the one live card on number 2 still resolves.
	for _, c := range []struct {
		ref  string
		want string
	}{
		{ref: ids[3], want: ids[3]},
		{ref: "c00000000099", want: "c00000000099"},
		{ref: "fx-2", want: ids[2]},
	} {
		id, err := h.library.watchedCard(c.ref)
		if err != nil || id != c.want {
			t.Errorf("%s answered %q and %v, and it should answer %s", c.ref, id, err, c.want)
		}
	}
}
