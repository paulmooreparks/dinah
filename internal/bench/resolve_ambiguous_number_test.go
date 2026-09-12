package bench

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"dinah/internal/contract"
)

// preRegistryFixture answers the fixture as a workbench still waiting for the
// number migration: no registry file, the format below the one the registry
// arrived at, and the fixture card's number back in its anchor. These tests
// drive the synthesis that reads numbers out of frontmatter, which is the
// state every workbench the migration has not reached is still read through,
// and the registry's own route is driven by
// TestResolveCardReadsTheRegistry.
func preRegistryFixture(t *testing.T) string {
	t.Helper()
	root := newFixture(t)
	if err := os.Remove(filepath.Join(root, CardNumbersName)); err != nil {
		t.Fatalf("remove the registry: %v", err)
	}
	editWorkbench(t, root, "format: "+strconv.Itoa(RegistryFormat), "format: "+strconv.Itoa(ContainerFormat))
	edit(t, root, "state: ready", "state: ready\nnumber: 1")
	return root
}

// ambiguousNumberFixture writes the workbench dinah-487's criteria are driven
// against and answers its root.
//
// It carries two collisions rather than one. The live half answers fx-1 with
// two cards, fx-2 with one, and fx-3 with none, while the archive answers fx-1
// with one and fx-3 with two. The first collision reaches a live-half unwrap
// and the second reaches an archived-half unwrap, which no single collision
// can do: a live card on the number answers the first attempt, so the archive
// is never read at all.
//
// The archived card on number 1 is what stops a live-half check passing for
// the wrong reason. Without it the unrepaired fall-through refuses
// unknown-card too, and a test asserting only that the call refused would go
// green on the defect.
//
// Every number is written by hand, because add mints by scanning for the
// highest in use and counts up, so the tool cannot produce this state. The
// workbench is dropped below the registry's format, because the synthesized
// index reads the numbers these anchors carry and the registry would read
// none of them.
func ambiguousNumberFixture(t *testing.T) string {
	t.Helper()
	root := preRegistryFixture(t)
	card := func(id, title string, number int) string {
		dir := filepath.Join(root, CardsDir, id)
		write(t, filepath.Join(dir, CardAnchor),
			fmt.Sprintf("---\ntitle: %s\nnumber: %d\ncolumn: b00000000001\nstate: ready\n---\nFraming.\n", title, number))
		write(t, filepath.Join(dir, JournalName), cleanJournal)
		return dir
	}
	card("c00000000002", "The live twin", 1)
	card("c00000000003", "The link source", 2)
	archiveDir(t, card("c00000000004", "The archived odd one", 1))
	archiveDir(t, card("c00000000005", "The archived twin", 3))
	archiveDir(t, card("c00000000006", "The other archived twin", 3))
	return root
}

// orderingFixture writes AC-4's variant: three live cards on one number and
// nothing in the archive, so the refusal's rows can be asserted whole.
func orderingFixture(t *testing.T) string {
	t.Helper()
	root := preRegistryFixture(t)
	for _, id := range []string{"c00000000002", "c00000000003"} {
		dir := filepath.Join(root, CardsDir, id)
		write(t, filepath.Join(dir, CardAnchor),
			fmt.Sprintf("---\ntitle: Card %s\nnumber: 1\ncolumn: b00000000001\nstate: ready\n---\nFraming.\n", id))
		write(t, filepath.Join(dir, JournalName), cleanJournal)
	}
	return root
}

// openAmbiguityFixture opens a bench on one of the two fixtures above.
func openAmbiguityFixture(t *testing.T, root string) *Bench {
	t.Helper()
	opened, err := Open(root)
	if err != nil {
		t.Fatalf("open %s: %v", root, err)
	}
	return opened
}

// mustRefuse reads the refusal out of an error and stops the test when the
// error carries none, so a check asserting a name never reports a nil
// dereference in place of the name it wanted.
func mustRefuse(t *testing.T, err error) *contract.Refusal {
	t.Helper()
	if err == nil {
		t.Fatalf("wanted a refusal and the call answered no error at all")
	}
	refusal := refusalOf(err)
	if refusal == nil {
		t.Fatalf("wanted a refusal, got %v", err)
	}
	return refusal
}

// TestAUniqueNumberResolvesExactlyAsBefore is dinah-487 AC-1, the
// compatibility half, driven against both halves of the collection.
//
// Only the two-or-more-matches case changes. A number one card carries
// resolves to that card and a number no card carries refuses unknown-card,
// either side of the mirror.
func TestAUniqueNumberResolvesExactlyAsBefore(t *testing.T) {
	root := preRegistryFixture(t)
	archived := filepath.Join(root, CardsDir, "c00000000002")
	write(t, filepath.Join(archived, CardAnchor),
		"---\ntitle: The archived card\nnumber: 2\ncolumn: b00000000001\nstate: ready\n---\nFraming.\n")
	write(t, filepath.Join(archived, JournalName), cleanJournal)
	archiveDir(t, archived)
	b := openAmbiguityFixture(t, root)

	found, err := b.resolveCardIn(b.CardsRoot(), "fx-1")
	if err != nil {
		t.Fatalf("the live half answers fx-1 with one card: %v", err)
	}
	if found.Card.ID != "c00000000001" {
		t.Errorf("fx-1 resolves to %s and the live half holds only c00000000001 on that number", found.Card.ID)
	}
	found, err = b.resolveCardIn(b.ArchivedCardsRoot(), "fx-2")
	if err != nil {
		t.Fatalf("the archived half answers fx-2 with one card: %v", err)
	}
	if found.Card.ID != "c00000000002" {
		t.Errorf("fx-2 resolves to %s in the mirror and only c00000000002 stands there", found.Card.ID)
	}
	for _, half := range []string{b.CardsRoot(), b.ArchivedCardsRoot()} {
		if _, err := b.resolveCardIn(half, "fx-404"); mustRefuse(t, err).Name != contract.UnknownCard {
			t.Errorf("a number no card carries refuses %v, and it should go on refusing %s", err, contract.UnknownCard)
		}
	}
}

// TestAnAmbiguousNumberNamesEveryCandidateInOrder is dinah-487 AC-4.
//
// Three rather than two, so a guard that happens to pass on a pair cannot pass
// by printing a fixed number of rows. The order is asserted because cardsWith
// preserves what ListIDs gives it and os.ReadDir documents sorting by
// filename, so a later change to that walk reddens a check rather than
// quietly reordering a refusal.
func TestAnAmbiguousNumberNamesEveryCandidateInOrder(t *testing.T) {
	b := openAmbiguityFixture(t, orderingFixture(t))
	_, err := b.resolveCardIn(b.CardsRoot(), "fx-1")
	refusal := mustRefuse(t, err)
	if refusal.Name != contract.AmbiguousCard {
		t.Fatalf("three cards carry number 1 and the resolution refused %s", refusal.Name)
	}
	if refusal.Detail != "fx-1" {
		t.Errorf("the refusal details %q and the reader typed fx-1", refusal.Detail)
	}
	rows := strings.Split(refusal.Extra["cards"], "\n")
	if len(rows) != 3 {
		t.Fatalf("the refusal carries %d rows and three cards carry the number", len(rows))
	}
	want := []string{"c00000000001", "c00000000002", "c00000000003"}
	if strings.Join(rows, ",") != strings.Join(want, ",") {
		t.Errorf("the rows read %v and ascending identifier order is %v", rows, want)
	}
}

// TestALinkTargetRefusesAnAmbiguousNumber is dinah-487 AC-5's bench leg and
// AC-11's first caller.
//
// fx-1 is the live collision, where the unwrap under test is the live one and
// the archive holds a card on the same number for the fall-through to land on.
// fx-3 is the archive collision, where the live attempt refuses unknown-card
// first and only the archived unwrap can carry the ambiguity out. The
// synthesized index walks the live half before the archived one, so fx-1's
// rows carry the archive's claimant behind the live pair.
func TestALinkTargetRefusesAnAmbiguousNumber(t *testing.T) {
	b := openAmbiguityFixture(t, ambiguousNumberFixture(t))
	for _, c := range []struct {
		ref  string
		want []string
	}{
		{ref: "fx-1", want: []string{"c00000000001", "c00000000002", "c00000000004"}},
		{ref: "fx-3", want: []string{"c00000000005", "c00000000006"}},
	} {
		id, refusal := b.ResolveLinkTarget(c.ref)
		if id != "" {
			t.Errorf("%s answered the identifier %s, and an ambiguous number answers none", c.ref, id)
		}
		if refusal == nil || refusal.Name != contract.AmbiguousCard {
			t.Fatalf("%s refused %v, and both cards carrying the number make it %s", c.ref, refusal, contract.AmbiguousCard)
		}
		if got := strings.Split(refusal.Extra["cards"], "\n"); strings.Join(got, ",") != strings.Join(c.want, ",") {
			t.Errorf("%s carries the rows %v and the candidates are %v", c.ref, got, c.want)
		}
	}
	// The sole answer to its number still resolves, so the unwraps closed
	// nothing that was open before them.
	if id, refusal := b.ResolveLinkTarget("fx-2"); id != "c00000000003" || refusal != nil {
		t.Errorf("fx-2 answers %q and %v, and one live card carries number 2", id, refusal)
	}
}

// TestTheEditTargetRefusesAnAmbiguousNumber is dinah-487 AC-12.
//
// ResolveEditTarget discards the entity resolver's error and retries through
// ResolvePath, which is a discard-and-retry none of the spec's searches
// reaches. It is left alone because the retry descends to the same
// resolveCardIn and raises the same refusal a second time. That is two code
// paths agreeing rather than anything written down, so it is driven here.
func TestTheEditTargetRefusesAnAmbiguousNumber(t *testing.T) {
	b := openAmbiguityFixture(t, ambiguousNumberFixture(t))
	path, err := b.ResolveEditTarget("fx-1")
	if path != "" {
		t.Errorf("edit would open %s, and an ambiguous reference names no file", path)
	}
	if name := mustRefuse(t, err).Name; name != contract.AmbiguousCard {
		t.Errorf("edit refused %s on an ambiguous number and the retry should re-raise %s", name, contract.AmbiguousCard)
	}
}
