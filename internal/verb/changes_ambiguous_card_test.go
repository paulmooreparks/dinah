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
// The live half answers fx-1 with two cards, fx-2 with one, and fx-3 with
// none. The archive answers fx-1 with one and fx-3 with two. The live
// collision reaches watchedCard's live unwrap, with the archived card on the
// same number standing where the unrepaired fall-through would land. The
// archive collision reaches the archived unwrap, which nothing else can: a
// live card on the number answers the first attempt and the mirror is never
// read.
//
// The cards are filed through the verb and archived through the verb, and only
// then are the numbers rewritten, because add mints by counting up from the
// highest in use and a reference stops resolving the moment its number
// collides.
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
	live := h.library.Bench.CardsRoot()
	archived := h.library.Bench.ArchivedCardsRoot()
	for _, c := range []struct {
		root   string
		id     string
		number int
	}{
		{live, ids[0], 1}, {live, ids[1], 1}, {live, ids[2], 2},
		{archived, ids[3], 1}, {archived, ids[4], 3}, {archived, ids[5], 3},
	} {
		renumberIn(t, c.root, c.id, c.number)
	}
	h.reopen()
	return h, ids
}

// renumberIn rewrites one card's ordinal in its own anchor, naming the root so
// the same helper reaches a live card and an archived one. Nothing in the tool
// offers this, because a number is set at birth and never reused, and this is
// the only way to build the state add refuses to mint.
func renumberIn(t *testing.T, root, id string, number int) {
	t.Helper()
	card, err := bench.LoadCard(root, id)
	if err != nil {
		t.Fatalf("load %s under %s: %v", id, filepath.Base(root), err)
	}
	card.Number = number
	if err := card.Save(); err != nil {
		t.Fatalf("renumber %s: %v", id, err)
	}
}

// TestTheChangesCardFilterRefusesAnAmbiguousNumber is dinah-487 AC-9's bench
// leg and AC-11's second caller.
//
// fx-1 is the live collision, where the archived card on the same number is
// exactly what the unrepaired fall-through would key the watch on. fx-3 is the
// archive collision, where the live attempt refuses unknown-card first, so
// only the archived unwrap can carry the ambiguity out rather than flattening
// it to unknown-card at the third arm.
func TestTheChangesCardFilterRefusesAnAmbiguousNumber(t *testing.T) {
	h, ids := ambiguousCardFixture(t)
	for _, c := range []struct {
		ref  string
		want []string
	}{
		{ref: "fx-1", want: sortedPair(ids[0], ids[1])},
		{ref: "fx-3", want: sortedPair(ids[4], ids[5])},
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

// sortedPair answers two identifiers in ascending order, which is the order
// the refusal's rows come out in because the scan preserves what os.ReadDir
// gives it. The fixture's identifiers are minted rather than written, so a
// check cannot name them in the order it happens to have filed them.
func sortedPair(first, second string) []string {
	if first > second {
		first, second = second, first
	}
	return []string{first, second}
}
