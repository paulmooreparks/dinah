package verb

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"dinah/internal/bench"
	"dinah/internal/contract"
)

// The definitions these tests reshape to. Each names the fixture's own column
// identifiers, so a column the definition repeats is kept and a column it
// leaves out is retired, with no matching by title and no per-run flag for the
// ordinary case.
//
// Every one of them keeps the two done columns last, because a done column
// stands in the terminal region of the flow and dinah check reports a board
// that puts an ordinary station after one. A fixture reshaping into a board
// check would report would be testing the reshape against a shape nobody may
// legally ask for.
const (
	// dropsAftercare retires the aftercare station and declares nothing about
	// where its cards go, which is the refusal AC-2 exercises.
	dropsAftercare = `{
  "profile": "dinah-core/0.7",
  "title": "Fixture",
  "columns": [
    { "id": "a00000000001", "title": "Intake", "kind": "intake" },
    { "id": "a00000000002", "title": "Doing", "kind": "work", "capacity": 1 },
    { "id": "a00000000003", "title": "Review", "kind": "work", "operator_owned": true },
    { "id": "a00000000004", "title": "Finished", "kind": "done" },
    { "id": "a00000000006", "title": "Closed", "kind": "done" }
  ]
}`

	// dropsAftercareIntoReview retires the same station and declares on the
	// review column that it takes over from it, which is the template author
	// saying once what every workbench born of the template should do.
	dropsAftercareIntoReview = `{
  "profile": "dinah-core/0.7",
  "title": "Fixture",
  "columns": [
    { "id": "a00000000001", "title": "Intake", "kind": "intake" },
    { "id": "a00000000002", "title": "Doing", "kind": "work", "capacity": 1 },
    { "id": "a00000000003", "title": "Review", "kind": "work", "operator_owned": true,
      "replaces": ["a00000000005"] },
    { "id": "a00000000004", "title": "Finished", "kind": "done" },
    { "id": "a00000000006", "title": "Closed", "kind": "done" }
  ]
}`

	// claimedTwice has two columns declaring they take over from the same
	// retirement, which is not a choice the tool makes for the operator.
	claimedTwice = `{
  "profile": "dinah-core/0.7",
  "title": "Fixture",
  "columns": [
    { "id": "a00000000001", "title": "Intake", "kind": "intake" },
    { "id": "a00000000002", "title": "Doing", "kind": "work", "capacity": 1,
      "replaces": ["a00000000005"] },
    { "id": "a00000000003", "title": "Review", "kind": "work", "operator_owned": true,
      "replaces": ["a00000000005"] },
    { "id": "a00000000004", "title": "Finished", "kind": "done" },
    { "id": "a00000000006", "title": "Closed", "kind": "done" }
  ]
}`

	// rekindsAftercare keeps every column and redeclares the aftercare
	// station as intake, which flips TakesWorkUp from true to false under
	// whatever stands there.
	rekindsAftercare = `{
  "profile": "dinah-core/0.7",
  "title": "Fixture",
  "columns": [
    { "id": "a00000000001", "title": "Intake", "kind": "intake" },
    { "id": "a00000000002", "title": "Doing", "kind": "work", "capacity": 1 },
    { "id": "a00000000003", "title": "Review", "kind": "work", "operator_owned": true },
    { "id": "a00000000005", "title": "Aftercare", "kind": "intake" },
    { "id": "a00000000004", "title": "Finished", "kind": "done" },
    { "id": "a00000000006", "title": "Closed", "kind": "done" }
  ]
}`

	// dropsTwo retires the aftercare and review stations at once, which is
	// what a destination resolving into the retired set needs.
	dropsTwo = `{
  "profile": "dinah-core/0.7",
  "title": "Fixture",
  "columns": [
    { "id": "a00000000001", "title": "Intake", "kind": "intake" },
    { "id": "a00000000002", "title": "Doing", "kind": "work", "capacity": 1 },
    { "id": "a00000000004", "title": "Finished", "kind": "done" },
    { "id": "a00000000006", "title": "Closed", "kind": "done" }
  ]
}`

	// addsTriage retires the aftercare station and adds a station carrying no
	// identifier of its own, so the only handle a --map has on it is its
	// title.
	addsTriage = `{
  "profile": "dinah-core/0.7",
  "title": "Fixture",
  "columns": [
    { "id": "a00000000001", "title": "Intake", "kind": "intake" },
    { "id": "new", "title": "Triage", "kind": "work" },
    { "id": "a00000000002", "title": "Doing", "kind": "work", "capacity": 1 },
    { "id": "a00000000003", "title": "Review", "kind": "work", "operator_owned": true },
    { "id": "a00000000004", "title": "Finished", "kind": "done" },
    { "id": "a00000000006", "title": "Closed", "kind": "done" }
  ]
}`

	// addsTwoTriage adds two stations sharing one title and neither carrying
	// an identifier, so nothing tells them apart.
	addsTwoTriage = `{
  "profile": "dinah-core/0.7",
  "title": "Fixture",
  "columns": [
    { "id": "a00000000001", "title": "Intake", "kind": "intake" },
    { "id": "new-one", "title": "Triage", "kind": "work", "slug": "triage-one" },
    { "id": "new-two", "title": "Triage", "kind": "work", "slug": "triage-two" },
    { "id": "a00000000002", "title": "Doing", "kind": "work", "capacity": 1 },
    { "id": "a00000000003", "title": "Review", "kind": "work", "operator_owned": true },
    { "id": "a00000000004", "title": "Finished", "kind": "done" },
    { "id": "a00000000006", "title": "Closed", "kind": "done" }
  ]
}`

	// retitlesDoing keeps every column and changes one title, which is the
	// smallest reshape that rewrites a kept column and records that it did.
	//
	// It repeats every other element of the fixture's own definition word for
	// word, instructions included, because definition wins for definition: an
	// element that dropped its instructions would have its anchor rewritten
	// without them and would count as changed, which is correct behaviour and
	// would leave this test unable to tell the one column it changed on
	// purpose from five it changed by omission. The slug is declared on the
	// retitled column alone, since a derived slug follows the title and this
	// column is meant to change its title and nothing else.
	retitlesDoing = `{
  "profile": "dinah-core/0.7",
  "title": "Fixture",
  "columns": [
    { "id": "a00000000001", "title": "Intake", "kind": "intake",
      "instructions": "Intake instructions.\n" },
    { "id": "a00000000002", "title": "Underway", "kind": "work", "capacity": 1, "slug": "doing",
      "instructions": "Doing instructions.\n" },
    { "id": "a00000000003", "title": "Review", "kind": "work", "operator_owned": true,
      "instructions": "Review instructions.\n" },
    { "id": "a00000000005", "title": "Aftercare", "kind": "work",
      "instructions": "Aftercare instructions.\n" },
    { "id": "a00000000004", "title": "Finished", "kind": "done",
      "instructions": "Finished instructions.\n" },
    { "id": "a00000000006", "title": "Closed", "kind": "done",
      "instructions": "Closed instructions.\n" }
  ]
}`
)

// strandedID is a well-formed identifier the fixture's own columns sequence
// never carries, which is what a hand edit leaves behind on a card.
const strandedID = "b00000000009"

// source writes a definition to a file outside the workbench, the way a person
// hands one to --from, and answers the path.
func (h *harness) source(body string) string {
	h.t.Helper()
	path := filepath.Join(h.t.TempDir(), "shape.json")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		h.t.Fatalf("write the definition: %v", err)
	}
	return path
}

// reshape runs the verb over the workbench as it now stands, reopening
// afterwards so the next read sees whatever the run wrote.
func (h *harness) reshape(source string, confirm bool, mapped ...string) (*ReshapeReport, error) {
	h.t.Helper()
	report, err := h.library.Reshape(&Request{
		Verb: "reshape", Actor: "alka", From: source, Map: mapped, Confirm: confirm,
	})
	h.reopen()
	return report, err
}

// digest hashes every file under the workbench, by path and by content, so
// that a run required to write nothing can be held to it. A refusal that
// wrote a column anchor, appended a journal line or rewrote the workbench
// anchor changes this value; one that wrote nothing does not.
func (h *harness) digest() string {
	h.t.Helper()
	sum := sha256.New()
	var paths []string
	err := filepath.WalkDir(h.root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		relative, relErr := filepath.Rel(h.root, path)
		if relErr != nil {
			return relErr
		}
		paths = append(paths, filepath.ToSlash(relative))
		return nil
	})
	if err != nil {
		h.t.Fatalf("walk the workbench: %v", err)
	}
	sort.Strings(paths)
	for _, relative := range paths {
		body, readErr := os.ReadFile(filepath.Join(h.root, filepath.FromSlash(relative)))
		if readErr != nil {
			h.t.Fatalf("read %s: %v", relative, readErr)
		}
		sum.Write([]byte(relative))
		sum.Write([]byte{0})
		sum.Write(body)
		sum.Write([]byte{0})
	}
	return hex.EncodeToString(sum.Sum(nil))
}

// sequence is the workbench's own columns sequence, read off disk.
func (h *harness) sequence() []string {
	h.t.Helper()
	return h.library.Bench.ColumnSequence()
}

// strand rewrites a card's column by hand to an identifier the workbench does
// not name, which is what the hand edit reshape exists to replace produces and
// what dinah check already reports as check.unknown-column. No verb writes
// this state, so the test writes it.
func (h *harness) strand(ref, column string) {
	h.t.Helper()
	card := h.card(ref)
	card.Column = column
	if err := card.Save(); err != nil {
		h.t.Fatalf("strand %s: %v", ref, err)
	}
	h.reopen()
}

// refusalName reads the refusal name off an error a verb returned, failing the
// test when the error is not a refusal at all.
func refusalName(t *testing.T, err error) string {
	t.Helper()
	if err == nil {
		t.Fatal("wanted a refusal, got no error at all")
	}
	refusal, ok := err.(*contract.Refusal)
	if !ok {
		t.Fatalf("wanted a refusal, got %T: %v", err, err)
	}
	return refusal.Name
}

// refusalDetail reads the detail and the named values off a refusal, joined,
// which is what a test asserting that a refusal names something reads.
func refusalText(t *testing.T, err error) string {
	t.Helper()
	if err == nil {
		t.Fatal("wanted a refusal, got no error at all")
	}
	refusal, ok := err.(*contract.Refusal)
	if !ok {
		t.Fatalf("wanted a refusal, got %T: %v", err, err)
	}
	parts := []string{refusal.Detail}
	for name, value := range refusal.Extra {
		parts = append(parts, name+"="+value)
	}
	sort.Strings(parts)
	return strings.Join(parts, " ")
}

// TestAPreviewWritesNothingAndNamesEveryColumn is dinah-316 AC-1. The preview
// is the whole of the validation phase with the report printed and the write
// phase skipped, so what it reports is what an apply would do and what it
// leaves on disk is exactly what it found.
func TestAPreviewWritesNothingAndNamesEveryColumn(t *testing.T) {
	h := newHarness(t)
	first := h.readyAt("carried one", aftercare)
	h.readyAt("carried two", aftercare)
	before := h.digest()

	report, err := h.reshape(h.source(dropsAftercareIntoReview), false)
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if after := h.digest(); after != before {
		t.Errorf("the preview wrote to the workbench: %s became %s", before, after)
	}
	if report.Applied {
		t.Error("the preview reports itself as applied")
	}
	dispositions := map[string]string{}
	for _, column := range report.Columns {
		dispositions[column.ID] = column.Disposition
	}
	for id, want := range map[string]string{
		intake: ReshapeKept, doing: ReshapeKept, review: ReshapeKept,
		finished: ReshapeKept, closed: ReshapeKept, aftercare: ReshapeRetired,
	} {
		if got := dispositions[id]; got != want {
			t.Errorf("%s: wanted the disposition %s, got %q", id, want, got)
		}
	}
	if len(report.Retirements) != 1 {
		t.Fatalf("wanted one retirement, got %d", len(report.Retirements))
	}
	retirement := report.Retirements[0]
	if retirement.ID != aftercare || retirement.Destination != review {
		t.Errorf("wanted aftercare retiring into review, got %s into %s", retirement.ID, retirement.Destination)
	}
	if retirement.Cards != 2 {
		t.Errorf("wanted the two cards standing there counted, got %d", retirement.Cards)
	}
	// The cards themselves have not moved, which is the half of "wrote
	// nothing" a digest of the tree would also catch and which is worth
	// asserting directly, since it is the state the operator is deciding about.
	if column := h.card(first).Column; column != aftercare {
		t.Errorf("the preview moved a card: %s stands at %s", first, column)
	}
}

// TestAnOccupiedRetirementWithNoDestinationIsRefused is dinah-316 AC-2. Which
// station the work continues at is the operator's decision, and no rule in the
// flow answers it, so the tool refuses by name rather than choosing.
//
// Both forms raise it, because the phase that raises it is the phase both
// forms run. That is the property worth pinning: a preview that reported
// cheerfully and left the refusal for the confirmed run would send an operator
// into a write knowing less than the preview did.
func TestAnOccupiedRetirementWithNoDestinationIsRefused(t *testing.T) {
	h := newHarness(t)
	h.readyAt("carried one", aftercare)
	h.readyAt("carried two", aftercare)
	before := h.digest()
	source := h.source(dropsAftercare)

	for _, confirm := range []bool{false, true} {
		_, err := h.reshape(source, confirm)
		if got := refusalName(t, err); got != contract.ReshapeNeedsDestination {
			t.Errorf("confirm=%v: wanted %s, got %s", confirm, contract.ReshapeNeedsDestination, got)
		}
		text := refusalText(t, err)
		if !strings.Contains(text, aftercareSlug) {
			t.Errorf("confirm=%v: the refusal does not name the column: %q", confirm, text)
		}
		// The count the refusal carries is the count of live cards standing
		// in the column, which is what the operator is deciding about. It is
		// compared against a fresh walk of the card collection rather than
		// against the literal 2, so a fixture that files a third card fails
		// here rather than passing against a stale number.
		standing := 0
		cards, cardsErr := h.library.Bench.Cards()
		if cardsErr != nil {
			t.Fatalf("cards: %v", cardsErr)
		}
		for _, card := range cards {
			if card.Column == aftercare {
				standing++
			}
		}
		if !strings.Contains(text, "cards="+strconv.Itoa(standing)) {
			t.Errorf("confirm=%v: wanted the refusal to carry the live count %d, got %q", confirm, standing, text)
		}
		if after := h.digest(); after != before {
			t.Errorf("confirm=%v: the refused run wrote to the workbench", confirm)
		}
	}
}

// TestAConfirmedReshapeCarriesTheCardsAndArchivesTheColumn is dinah-316 AC-3.
//
// The archive lands at archive/columns/<id>, which is the archive mirror at
// the retiring entity's own level and is where bench.ArchiveTarget has always
// put one. The criterion as written named columns/archive/<id>, a path nothing
// in the tool writes; the text is corrected on the card and the assertion
// below reads the path the code actually uses.
func TestAConfirmedReshapeCarriesTheCardsAndArchivesTheColumn(t *testing.T) {
	h := newHarness(t)
	carried := h.readyAt("carried", aftercare)

	report, err := h.reshape(h.source(dropsAftercare), true, aftercare+"="+review)
	if err != nil {
		t.Fatalf("reshape: %v", err)
	}
	if !report.Applied {
		t.Error("the run reports itself as a preview")
	}
	if column := h.card(carried).Column; column != review {
		t.Fatalf("wanted the card carried to review, it stands at %s", column)
	}
	events := h.events(carried)
	last := events[len(events)-1]
	if last.Event != contract.EventMoved || !last.Reshape {
		t.Fatalf("wanted a moved event carrying the reshape marker, got %+v", last)
	}
	if last.From != aftercare || last.To != review {
		t.Errorf("wanted from %s to %s, got from %s to %s", aftercare, review, last.From, last.To)
	}
	if last.FromTitle != "Aftercare" || last.ToTitle != "Review" {
		t.Errorf("wanted the titles as of the move, got %q and %q", last.FromTitle, last.ToTitle)
	}
	for _, id := range h.sequence() {
		if id == aftercare {
			t.Errorf("the retired column is still in the columns sequence")
		}
	}
	archived := filepath.Join(h.root, bench.ArchiveDir, bench.ColumnsDir, aftercare, bench.ColumnAnchor)
	if !bench.Exists(archived) {
		t.Errorf("wanted the retired column under %s, it is not there", archived)
	}
	if bench.Exists(filepath.Join(h.root, bench.ColumnsDir, aftercare)) {
		t.Errorf("the retired column is still standing in the live half of the collection")
	}
}

// TestABlockedCardIsCarriedWithItsBlock is dinah-316 AC-4. CORE-MOVE-8 forbids
// a move changing a card's state or its holder, and a reshape's carry is a
// moved event like any other, so the block, its reason and its holder all
// survive the carry untouched. The report names the card so that nobody
// reading it afterwards mistakes a carried block for a cleared one.
func TestABlockedCardIsCarriedWithItsBlock(t *testing.T) {
	h := newHarness(t)
	blocked := h.readyAt("blocked", aftercare)
	h.mustDo(&Request{Verb: Block, Actor: "alka", Card: blocked, Reason: "a vendor has not answered", Kind: "external"})

	report, err := h.reshape(h.source(dropsAftercare), true, aftercare+"="+review)
	if err != nil {
		t.Fatalf("reshape: %v", err)
	}
	card := h.card(blocked)
	if card.State != contract.StateBlocked {
		t.Errorf("wanted the card still blocked, its state is %q", card.State)
	}
	if card.BlockReason != "a vendor has not answered" {
		t.Errorf("the block reason changed: %q", card.BlockReason)
	}
	if card.Column != review {
		t.Errorf("wanted the blocked card carried to review, it stands at %s", card.Column)
	}
	named := false
	for _, retirement := range report.Retirements {
		for _, id := range retirement.Blocked {
			if id == card.ID {
				named = true
			}
		}
	}
	if !named {
		t.Errorf("the report does not name %s as carried while blocked", card.ID)
	}
}

// TestAKindFlipUnderAHeldCardIsRefused is dinah-316 AC-5. A work column
// redeclared as intake stops taking work up, and the claim already standing
// there becomes a claim no guard would ever have allowed. check reports that
// state and repairs nothing about it, so the reshape refuses the write rather
// than creating a defect for check to find afterwards.
func TestAKindFlipUnderAHeldCardIsRefused(t *testing.T) {
	h := newHarness(t)
	held := h.readyAt("held", aftercare)
	h.mustDo(&Request{Verb: Claim, Actor: "alka", Card: held, Holder: "alka"})
	before := h.digest()
	source := h.source(rekindsAftercare)

	for _, confirm := range []bool{false, true} {
		_, err := h.reshape(source, confirm)
		if got := refusalName(t, err); got != contract.ReshapeHeldCardInQueue {
			t.Errorf("confirm=%v: wanted %s, got %s", confirm, contract.ReshapeHeldCardInQueue, got)
		}
		text := refusalText(t, err)
		if !strings.Contains(text, aftercareSlug) || !strings.Contains(text, h.card(held).ID) {
			t.Errorf("confirm=%v: wanted the column and the held card named, got %q", confirm, text)
		}
		if after := h.digest(); after != before {
			t.Errorf("confirm=%v: the refused run wrote to the workbench", confirm)
		}
	}
}

// TestAKindFlipOverReadyCardsIsAllowed is dinah-316 AC-6, and it is what tells
// the refusal above from one that refuses every kind flip. A card standing
// ready in a column that stops taking work up is the ordinary handoff: nobody
// is holding it, so nothing is taken up where nothing may be, and the card
// waits there for a pull exactly as a card in any intake column does.
func TestAKindFlipOverReadyCardsIsAllowed(t *testing.T) {
	h := newHarness(t)
	waiting := h.readyAt("waiting", aftercare)

	if _, err := h.reshape(h.source(rekindsAftercare), true); err != nil {
		t.Fatalf("reshape: %v", err)
	}
	// The card is a kept column's card rather than a carried one, so its own
	// column field is untouched and no moved event was written for it.
	if column := h.card(waiting).Column; column != aftercare {
		t.Errorf("wanted the card left where it stands, it is at %s", column)
	}
	for _, ev := range h.events(waiting) {
		if ev.Reshape {
			t.Errorf("a kept column's card was carried: %+v", ev)
		}
	}
	if kind := h.library.Bench.Column(aftercare).Kind; kind != contract.KindIntake {
		t.Errorf("wanted the new kind written, the column is %q", kind)
	}
}

// TestAMapSourceCarryingNoCardIsRefused is dinah-316 AC-7. A typed identifier
// that matches nothing has to read as a refusal rather than as a silent no-op,
// which is the defect class the working agreement's "a refusal that any value
// satisfies is not a refusal" paragraph warns about: without this row, --map
// would accept any word at all and quietly carry nothing.
func TestAMapSourceCarryingNoCardIsRefused(t *testing.T) {
	h := newHarness(t)
	h.readyAt("carried", aftercare)
	absent := "c00000000007"
	if h.library.Bench.Column(absent) != nil {
		t.Fatalf("the fixture carries a column at %s, so this case is not the one it names", absent)
	}

	_, err := h.reshape(h.source(dropsAftercare), false, absent+"="+review)
	if got := refusalName(t, err); got != contract.ReshapeMapSourceEmpty {
		t.Fatalf("wanted %s, got %s", contract.ReshapeMapSourceEmpty, got)
	}
	if text := refusalText(t, err); !strings.Contains(text, absent) {
		t.Errorf("the refusal does not name the identifier: %q", text)
	}
}

// TestAStrandedCardIsAdoptedByAnExplicitMap is dinah-316 AC-8. A card orphaned
// by an earlier hand edit is no part of reshape's own sort, which walks two
// column lists and never meets an identifier absent from both, so it is
// untouched until the operator names it. Named, it rides the same carry
// machinery a retiring column's occupants get, and no structural act runs for
// it because there is no live column left to archive.
func TestAStrandedCardIsAdoptedByAnExplicitMap(t *testing.T) {
	h := newHarness(t)
	orphan := h.readyAt("orphan", aftercare)
	h.strand(orphan, strandedID)
	if detail, found := finding(h.check(), bench.FindingUnknownColumn); !found || detail != strandedID {
		t.Fatalf("wanted check to report the stranded column before the run, got %q %v", detail, found)
	}
	sequenceBefore := strings.Join(h.sequence(), " ")

	if _, err := h.reshape(h.source(rekindsAftercare), true, strandedID+"="+review); err != nil {
		t.Fatalf("reshape: %v", err)
	}
	card := h.card(orphan)
	if card.Column != review {
		t.Fatalf("wanted the adopted card carried to review, it stands at %s", card.Column)
	}
	events := h.events(orphan)
	last := events[len(events)-1]
	if last.Event != contract.EventMoved || !last.Reshape || last.From != strandedID || last.To != review {
		t.Errorf("wanted a reshape-marked move out of the stranded identifier, got %+v", last)
	}
	if _, found := finding(h.check(), bench.FindingUnknownColumn); found {
		t.Errorf("check still reports an unknown column for the adopted card")
	}
	// No structural act ran, so nothing was archived under the stranded
	// identifier and the columns sequence is unchanged with respect to it,
	// which it never named in the first place.
	if bench.Exists(filepath.Join(h.root, bench.ArchiveDir, bench.ColumnsDir, strandedID)) {
		t.Errorf("an adopted identifier was archived, and it names no column to archive")
	}
	if got := strings.Join(h.sequence(), " "); got != sequenceBefore {
		t.Errorf("the columns sequence changed: %q became %q", sequenceBefore, got)
	}
}

// TestADestinationInsideTheRetiredSetIsRefused is dinah-316 AC-10. Without
// this row the run would carry one retirement's cards into another retiring
// column and then meet dinah.column-occupied at the archive step, halfway
// through a run that had already written some of its moves. The refusal fires
// during validation instead, so the half-completed run cannot happen.
func TestADestinationInsideTheRetiredSetIsRefused(t *testing.T) {
	h := newHarness(t)
	h.readyAt("in aftercare", aftercare)
	h.readyAt("in review", review)
	before := h.digest()
	source := h.source(dropsTwo)

	// Every spelling ColumnByRef admits is exercised, because the rule is
	// about the column a destination resolves to and not about how the caller
	// wrote it. The three reach the refusal by two different routes: an
	// identifier is a key of the retired set itself, where a slug and a title
	// are not, so a guard covering only one of the two would leave a
	// destination written the other way through.
	for _, spelling := range []string{review, "review", "Review"} {
		for _, confirm := range []bool{false, true} {
			_, err := h.reshape(source, confirm, aftercare+"="+spelling, review+"="+doing)
			name := refusalName(t, err)
			if name != contract.ReshapeDestinationRetiring {
				t.Fatalf("%s confirm=%v: wanted %s, got %s", spelling, confirm, contract.ReshapeDestinationRetiring, name)
			}
			// The occupancy refusal is the one this rule exists to make
			// unreachable, so its absence is asserted rather than assumed: a
			// run that reached it would have written moves before it did.
			if name == contract.Occupied {
				t.Errorf("%s confirm=%v: the run reached the occupancy refusal", spelling, confirm)
			}
			if text := refusalText(t, err); !strings.Contains(text, spelling) {
				t.Errorf("%s confirm=%v: the refusal does not name the retiring destination: %q", spelling, confirm, text)
			}
			if after := h.digest(); after != before {
				t.Errorf("%s confirm=%v: the refused run wrote to the workbench", spelling, confirm)
			}
		}
	}
}

// TestAnAdoptedIdentifierIsAlsoADestinationTheRunRetires is the other route
// into the refusal above, and the one an identifier-spelled live column never
// reaches, because a live column is caught by the lookup against the workbench
// before the retired set is consulted at all.
//
// An adopted identifier names no live column, so ColumnByRef answers nothing
// for it and the retired set is where it is found. Cards carried there would
// stand at an identifier this same run is carrying cards out of, which is the
// same defect as carrying them into a column about to be archived and is
// refused on the same terms.
func TestAnAdoptedIdentifierIsAlsoADestinationTheRunRetires(t *testing.T) {
	h := newHarness(t)
	orphan := h.readyAt("orphan", aftercare)
	h.strand(orphan, strandedID)
	h.readyAt("in aftercare", aftercare)
	before := h.digest()
	source := h.source(dropsAftercare)

	// The entry adopting the stranded identifier is written first, so the
	// entry aiming at it meets a retired set that already carries it.
	_, err := h.reshape(source, false, strandedID+"="+review, aftercare+"="+strandedID)
	if got := refusalName(t, err); got != contract.ReshapeDestinationRetiring {
		t.Fatalf("wanted %s, got %s", contract.ReshapeDestinationRetiring, got)
	}
	if text := refusalText(t, err); !strings.Contains(text, strandedID) {
		t.Errorf("the refusal does not name the adopted identifier: %q", text)
	}
	if after := h.digest(); after != before {
		t.Errorf("the refused run wrote to the workbench")
	}
}

// TestADestinationMayBeAColumnTheRunAdds is dinah-316 AC-11. An added column
// exists only in the incoming definition while validation runs, so
// ColumnByRef cannot see it and the destination resolves against the added
// elements by title instead. The carry then names a column that is live,
// because write-phase step one wrote it before any card was carried.
//
// The title is Triage rather than the criterion's own Intake because the
// fixture already carries a live column titled Intake, and a destination
// matching a live column resolves against the live workbench at the first tier
// and never reaches the added elements at all. Triage exercises the tier the
// criterion is about; Intake would have exercised the tier above it.
func TestADestinationMayBeAColumnTheRunAdds(t *testing.T) {
	h := newHarness(t)
	carried := h.readyAt("carried", aftercare)

	if _, err := h.reshape(h.source(addsTriage), true, aftercare+"=Triage"); err != nil {
		t.Fatalf("reshape: %v", err)
	}
	minted := ""
	for _, id := range h.sequence() {
		column := h.library.Bench.Column(id)
		if column != nil && column.Title == "Triage" {
			minted = id
		}
	}
	if minted == "" {
		t.Fatal("the run wrote no column titled Triage")
	}
	if column := h.card(carried).Column; column != minted {
		t.Errorf("wanted the card at the minted column %s, it stands at %s", minted, column)
	}
	events := h.events(carried)
	last := events[len(events)-1]
	if last.ToTitle != "Triage" || last.To != minted {
		t.Errorf("wanted the move to name the added column, got to %s titled %q", last.To, last.ToTitle)
	}
}

// TestADestinationMatchingTwoAddedColumnsIsRefused is dinah-316 AC-12. Neither
// candidate carries a live identifier yet, so the refusal distinguishes them
// by where each sits in the new definition's own columns array, which is the
// one handle both of them do carry.
func TestADestinationMatchingTwoAddedColumnsIsRefused(t *testing.T) {
	h := newHarness(t)
	h.readyAt("carried", aftercare)
	before := h.digest()
	source := h.source(addsTwoTriage)

	for _, confirm := range []bool{false, true} {
		_, err := h.reshape(source, confirm, aftercare+"=Triage")
		if got := refusalName(t, err); got != contract.ReshapeDestinationAmbiguous {
			t.Fatalf("confirm=%v: wanted %s, got %s", confirm, contract.ReshapeDestinationAmbiguous, got)
		}
		text := refusalText(t, err)
		if !strings.Contains(text, "Triage") {
			t.Errorf("confirm=%v: the refusal does not name the title: %q", confirm, text)
		}
		// The two candidates sit at positions 1 and 2 of the definition's
		// columns array, and both are named, since naming one would leave the
		// reader unable to tell which of the two the tool meant.
		if !strings.Contains(text, "1") || !strings.Contains(text, "2") {
			t.Errorf("confirm=%v: the refusal does not name both positions: %q", confirm, text)
		}
		if after := h.digest(); after != before {
			t.Errorf("confirm=%v: the refused run wrote to the workbench", confirm)
		}
	}
}

// TestTheReportCountsTheCardsThisRunDoesNotKnowAbout is dinah-316 AC-13. The
// count is what reshape can honestly compute from a diff of two column lists;
// naming the identifier takes a pass over every card, which dinah check
// already makes and this command does not duplicate. The report says so, and
// the help text says so, so an operator meets the two-command workflow rather
// than guessing that the second command exists.
func TestTheReportCountsTheCardsThisRunDoesNotKnowAbout(t *testing.T) {
	h := newHarness(t)
	orphan := h.readyAt("orphan", aftercare)
	h.strand(orphan, strandedID)
	h.readyAt("ordinary", doing)
	source := h.source(retitlesDoing)

	preview, err := h.reshape(source, false)
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if preview.StrandedCards != 1 {
		t.Errorf("wanted one stranded card counted in the preview, got %d", preview.StrandedCards)
	}
	applied, err := h.reshape(source, true)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if applied.StrandedCards != 1 {
		t.Errorf("wanted one stranded card counted after the write, got %d", applied.StrandedCards)
	}
	// The run named nothing about the stranded card, so it is stranded still,
	// which is what "untouched by default" means.
	if column := h.card(orphan).Column; column != strandedID {
		t.Errorf("the run moved a card it does not know about: %s", column)
	}
}

// TestADestinationNamingNothingIsRefused is dinah-316 AC-14. It is the third
// tier of destination resolution, and it is a different code path from AC-7's
// unresolvable --map source: this one is the right side of the entry, and the
// refusal is the one every other verb already raises for a column reference
// that resolves to nothing rather than a synonym minted for this verb.
func TestADestinationNamingNothingIsRefused(t *testing.T) {
	h := newHarness(t)
	h.readyAt("carried", aftercare)
	before := h.digest()
	source := h.source(dropsAftercare)

	for _, confirm := range []bool{false, true} {
		_, err := h.reshape(source, confirm, aftercare+"=Nowhere")
		if got := refusalName(t, err); got != contract.UnknownColumn {
			t.Fatalf("confirm=%v: wanted %s, got %s", confirm, contract.UnknownColumn, got)
		}
		if text := refusalText(t, err); !strings.Contains(text, "Nowhere") {
			t.Errorf("confirm=%v: the refusal does not name what the caller wrote: %q", confirm, text)
		}
		if after := h.digest(); after != before {
			t.Errorf("confirm=%v: the refused run wrote to the workbench", confirm)
		}
	}
}

// TestOneRetirementClaimedTwiceIsRefused is dinah-316 AC-15. Two destinations
// for one set of carried cards is not a choice the tool makes for the
// operator, so it refuses and names the identifier together with both columns
// declaring it, which is what the operator needs in order to edit the
// definition.
func TestOneRetirementClaimedTwiceIsRefused(t *testing.T) {
	h := newHarness(t)
	h.readyAt("carried", aftercare)
	before := h.digest()
	source := h.source(claimedTwice)

	for _, confirm := range []bool{false, true} {
		_, err := h.reshape(source, confirm)
		if got := refusalName(t, err); got != contract.Malformed {
			t.Fatalf("confirm=%v: wanted %s, got %s", confirm, contract.Malformed, got)
		}
		text := refusalText(t, err)
		for _, want := range []string{aftercare, "Doing", "Review"} {
			if !strings.Contains(text, want) {
				t.Errorf("confirm=%v: the refusal does not name %s: %q", confirm, want, text)
			}
		}
		if after := h.digest(); after != before {
			t.Errorf("confirm=%v: the refused run wrote to the workbench", confirm)
		}
	}
}

// derivedTriageID is the identifier write-phase step one gives the Triage
// element of addsTriage: derived from the source's own content hash and the
// element's position, so a test can compute it before the run does.
func derivedTriageID(t *testing.T, source string) string {
	t.Helper()
	body, err := os.ReadFile(source)
	if err != nil {
		t.Fatalf("read the definition: %v", err)
	}
	return bench.DeriveColumnID(bench.SourceDigest(body), 1)
}

// TestARetryFinishesAnAddedColumnLeftAsABareDirectory is dinah-316 AC-16. The
// crash window is between creating the column's directory and writing its
// anchor, and the fixture reproduces exactly the state such a crash leaves:
// the directory at the identifier the run derives, empty, with the identifier
// absent from the workbench's own sequence.
//
// What makes the retry land is that the identifier is derived rather than
// drawn. A random draw could not be recomputed, so the retry would mint a
// second identifier and leave the first directory behind with nothing pointing
// at it.
func TestARetryFinishesAnAddedColumnLeftAsABareDirectory(t *testing.T) {
	h := newHarness(t)
	carried := h.readyAt("carried", aftercare)
	source := h.source(addsTriage)
	derived := derivedTriageID(t, source)
	if err := os.MkdirAll(filepath.Join(h.root, bench.ColumnsDir, derived), 0o755); err != nil {
		t.Fatalf("plant the half-written directory: %v", err)
	}

	if _, err := h.reshape(source, true, aftercare+"=Triage"); err != nil {
		t.Fatalf("the retry: %v", err)
	}
	dir := filepath.Join(h.root, bench.ColumnsDir, derived)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	if len(entries) != 1 || entries[0].Name() != bench.ColumnAnchor {
		t.Fatalf("wanted exactly the column anchor in %s, got %v", dir, entries)
	}
	// The anchor the retry wrote is the anchor a run that met no crash would
	// have written, byte for byte, which is what makes finishing the prior
	// attempt equal to redoing it.
	fresh := newHarness(t)
	fresh.readyAt("carried", aftercare)
	freshSource := fresh.source(addsTriage)
	if _, err := fresh.reshape(freshSource, true, aftercare+"=Triage"); err != nil {
		t.Fatalf("the from-scratch run: %v", err)
	}
	want := bench.ColumnAnchorText(fresh.root, derivedTriageID(t, freshSource))
	if got := bench.ColumnAnchorText(h.root, derived); got != want {
		t.Errorf("the retry wrote a different anchor:\n%q\nwanted\n%q", got, want)
	}
	if got := countIn(h.sequence(), derived); got != 1 {
		t.Errorf("wanted the identifier in the sequence exactly once, got %d", got)
	}
	if column := h.card(carried).Column; column != derived {
		t.Errorf("wanted the card carried into the finished column, it stands at %s", column)
	}
}

// TestARetryFinishesAnAddedColumnAlreadyWritten is dinah-316 AC-17. The second
// crash window is between writing the anchor and appending the identifier to
// the sequence, and the fixture plants a whole, valid anchor with no sequence
// entry pointing at it. The retry overwrites the anchor with identical bytes
// and appends the identifier once, never twice.
func TestARetryFinishesAnAddedColumnAlreadyWritten(t *testing.T) {
	h := newHarness(t)
	source := h.source(addsTriage)
	derived := derivedTriageID(t, source)

	// The anchor is planted by running the whole write against a second
	// workbench and copying out what step one wrote there, so the fixture is
	// the state the crash leaves rather than a hand-written approximation of
	// it that could differ from what the code writes.
	donor := newHarness(t)
	donorSource := donor.source(addsTriage)
	if _, err := donor.reshape(donorSource, true); err != nil {
		t.Fatalf("the donor run: %v", err)
	}
	planted := bench.ColumnAnchorText(donor.root, derivedTriageID(t, donorSource))
	if planted == "" {
		t.Fatal("the donor run wrote no anchor to plant")
	}
	target := filepath.Join(h.root, bench.ColumnsDir, derived, bench.ColumnAnchor)
	if err := bench.WriteText(target, planted); err != nil {
		t.Fatalf("plant the anchor: %v", err)
	}
	h.reopen()
	if got := countIn(h.sequence(), derived); got != 0 {
		t.Fatalf("the fixture is not the state it names: the sequence already carries %s", derived)
	}

	if _, err := h.reshape(source, true); err != nil {
		t.Fatalf("the retry: %v", err)
	}
	if got := bench.ColumnAnchorText(h.root, derived); got != planted {
		t.Errorf("the retry changed the planted anchor:\n%q\nwanted\n%q", got, planted)
	}
	if got := countIn(h.sequence(), derived); got != 1 {
		t.Errorf("wanted the identifier appended exactly once, got %d", got)
	}
}

// TestARewrittenKeptColumnRecordsItAndAnUnchangedOneDoesNot is the write
// phase's fourth step read from both sides. A column the new definition
// actually changes records a column_updated event on the workbench journal; a
// column the definition repeats unchanged records nothing, so a reshape that
// only adds and retires leaves no noise on the columns it never touched.
func TestARewrittenKeptColumnRecordsItAndAnUnchangedOneDoesNot(t *testing.T) {
	h := newHarness(t)

	report, err := h.reshape(h.source(retitlesDoing), true)
	if err != nil {
		t.Fatalf("reshape: %v", err)
	}
	if len(report.Updated) != 1 || report.Updated[0] != doing {
		t.Fatalf("wanted the one changed column reported, got %v", report.Updated)
	}
	if title := h.library.Bench.Column(doing).Title; title != "Underway" {
		t.Errorf("wanted the new title written, the column reads %q", title)
	}
	updated := map[string]int{}
	for _, ev := range h.benchEvents() {
		if ev.Event == contract.EventColumnUpdated {
			updated[ev.Note]++
		}
	}
	if updated[doing] != 1 {
		t.Errorf("wanted one column_updated event for the changed column, got %d", updated[doing])
	}
	for _, quiet := range []string{intake, review, aftercare, finished, closed} {
		if updated[quiet] != 0 {
			t.Errorf("%s was repeated unchanged and still recorded %d events", quiet, updated[quiet])
		}
	}
}

// TestReplacesPersistsOnTheColumnItIsDeclaredOn is dinah-316 D-6 read off
// disk. replaces is absent from knownColumnKeys, so writeColumnFromMember's
// generic pass writes it into the column's own frontmatter and exportColumn's
// matching pass reads it back out, exactly as reject_to travels. An
// unpersisted member would vanish on a round trip through extract, which is
// the CORE-JSON-7 failure this guards.
func TestReplacesPersistsOnTheColumnItIsDeclaredOn(t *testing.T) {
	h := newHarness(t)
	h.readyAt("carried", aftercare)

	if _, err := h.reshape(h.source(dropsAftercareIntoReview), true); err != nil {
		t.Fatalf("reshape: %v", err)
	}
	anchor := bench.ColumnAnchorText(h.root, review)
	if !strings.Contains(anchor, bench.ReplacesKey) || !strings.Contains(anchor, aftercare) {
		t.Fatalf("the declaration did not reach the column's own anchor:\n%s", anchor)
	}
	exported, err := h.library.Bench.Export()
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if !strings.Contains(string(exported), bench.ReplacesKey) {
		t.Errorf("the exported definition dropped the member:\n%s", exported)
	}
}

// countIn is how many times an identifier appears in a sequence, which is what
// tells a retry that appended once from one that appended twice.
func countIn(sequence []string, id string) int {
	count := 0
	for _, carried := range sequence {
		if carried == id {
			count++
		}
	}
	return count
}
