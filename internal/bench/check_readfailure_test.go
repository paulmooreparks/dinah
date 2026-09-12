package bench

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dinah/internal/contract"
)

// numberedCard is a card anchor carrying the number a duplicate-number test
// needs, built from cleanCard so the rest of the anchor stays the fixture the
// other check tests use.
func numberedCard(number string) string {
	return "---\ntitle: A card\nnumber: " + number + "\ncolumn: b00000000001\nstate: ready\n---\nFraming.\n"
}

// findingsOfKey keeps the findings carrying one key, which is what a test
// asserting a count of one kind compares rather than the whole report.
func findingsOfKey(findings []Finding, key string) []Finding {
	var kept []Finding
	for _, finding := range findings {
		if finding.Key == key {
			kept = append(kept, finding)
		}
	}
	return kept
}

// TestCheckReportsACollectionItCouldNotRead asserts that Check answers with an
// error when a collection in its walk cannot be read, and that it answers with
// the findings it had already gathered alongside that error, so the partial
// report survives at the library boundary.
//
// What a reader is told about a partial report is dinah-462's to decide, so no
// exit code is asserted here and neither caller of Check is touched.
//
// The succeeding case runs in the same test: the same workbench with every
// collection readable returns the same one finding and a nil error.
func TestCheckReportsACollectionItCouldNotRead(t *testing.T) {
	// oneDefect writes a workbench carrying exactly one defect Check reports,
	// a workstream directory with no anchor.
	oneDefect := func(t *testing.T) string {
		t.Helper()
		root := newFixture(t)
		if err := os.MkdirAll(filepath.Join(root, WorkstreamsDir, "f00000000001"), 0o755); err != nil {
			t.Fatalf("mkdir a workstream with no anchor: %v", err)
		}
		return root
	}

	t.Run("every collection readable", func(t *testing.T) {
		root := oneDefect(t)
		opened, err := Open(root)
		if err != nil {
			t.Fatalf("open: %v", err)
		}
		findings, err := opened.Check()
		if err != nil {
			t.Fatalf("Check answered an error over a readable workbench: %v", err)
		}
		if len(findings) != 1 {
			t.Fatalf("Check reported %d findings over a workbench carrying exactly one defect: %+v", len(findings), findings)
		}
		if findings[0].Key != FindingMissingAnchor {
			t.Errorf("the one finding reads %s, wanted %s", findings[0].Key, FindingMissingAnchor)
		}
	})

	t.Run("a collection read after a finding was gathered", func(t *testing.T) {
		root := oneDefect(t)
		opened, err := Open(root)
		if err != nil {
			t.Fatalf("open: %v", err)
		}
		// The archived cards collection is read by checkCardNumbers, which
		// runs after the workstream check has already gathered the one
		// genuine finding, so this case is where the partial slice is
		// asserted at all.
		plantUnreadable(t, filepath.Join(root, ArchiveDir, CardsDir))

		findings, err := opened.Check()
		if err == nil {
			t.Fatalf("Check returned no error for a workbench whose archived cards collection it could not read, reporting %+v", findings)
		}
		if len(findingsOfKey(findings, FindingMissingAnchor)) != 1 {
			t.Errorf("Check dropped the findings it had already gathered, answering %+v", findings)
		}
	})

	t.Run("the live cards collection will not read", func(t *testing.T) {
		root := oneDefect(t)
		opened, err := Open(root)
		if err != nil {
			t.Fatalf("open: %v", err)
		}
		// The live cards collection is the walk's first read, so nothing has
		// been gathered when it fails and this case asserts the error alone.
		if err := os.RemoveAll(filepath.Join(root, CardsDir)); err != nil {
			t.Fatalf("clear the cards collection: %v", err)
		}
		plantUnreadable(t, filepath.Join(root, CardsDir))

		findings, err := opened.Check()
		if err == nil {
			t.Fatalf("Check returned no error for a workbench whose cards collection it could not read, reporting %+v", findings)
		}
	})
}

// TestCheckReportsADuplicateCardNumber covers the detector this card mints.
//
// The first case asserts both colliding lines, which is also how it proves
// the whole registry was read: checkCardNumbers answers findings and nothing
// else, so there is no examined count to assert, and a sweep that dropped the
// lines naming archived cards could not produce the finding carrying the
// archived line.
func TestCheckReportsADuplicateCardNumber(t *testing.T) {
	// twoHalves writes a live card and an archived card, and claims both in
	// the registry under the numbers it is given. The anchors carry no number
	// key, which is the shape the registry era leaves behind.
	twoHalves := func(t *testing.T, live, archived string) string {
		t.Helper()
		root := newFixture(t)
		write(t, filepath.Join(root, ArchiveDir, CardsDir, "c00000000002", CardAnchor), cleanCard)
		write(t, filepath.Join(root, CardNumbersName), live+" c00000000001\n"+archived+" c00000000002\n")
		return root
	}

	t.Run("two lines claiming one number", func(t *testing.T) {
		root := twoHalves(t, "7", "7")
		opened, err := Open(root)
		if err != nil {
			t.Fatalf("open: %v", err)
		}
		findings, err := opened.Check()
		if err != nil {
			t.Fatalf("check: %v", err)
		}
		duplicates := findingsOfKey(findings, FindingCardNumberDuplicate)
		if len(duplicates) != 2 {
			t.Fatalf("two lines claim number 7 and dinah check reported %d findings: %+v", len(duplicates), duplicates)
		}
		registry := filepath.Join(root, CardNumbersName)
		wanted := map[string]bool{"7 c00000000001": false, "7 c00000000002": false}
		for _, finding := range duplicates {
			if finding.Path != registry {
				t.Errorf("a duplicate finding names %s, wanted the registry file %s", finding.Path, registry)
			}
			if _, named := wanted[finding.Detail]; !named {
				t.Errorf("a duplicate finding carries the detail %q, which is neither colliding line", finding.Detail)
				continue
			}
			wanted[finding.Detail] = true
		}
		for line, seen := range wanted {
			if !seen {
				t.Errorf("no duplicate finding carries the line %q, so the card that line claims was not read", line)
			}
		}
	})

	t.Run("every number unique", func(t *testing.T) {
		root := twoHalves(t, "7", "8")
		opened, err := Open(root)
		if err != nil {
			t.Fatalf("open: %v", err)
		}
		findings, err := opened.Check()
		if err != nil {
			t.Fatalf("check: %v", err)
		}
		if len(findings) != 0 {
			t.Fatalf("a workbench whose registry claims every card once reports %v, and it should report nothing", findings)
		}
	})

	t.Run("an archived collection that will not read", func(t *testing.T) {
		// The archived half is a plain file rather than a directory here, so
		// the fixture writes no archived card: the collection nobody can list
		// is the whole of what this case is about.
		root := newFixture(t)
		opened, err := Open(root)
		if err != nil {
			t.Fatalf("open: %v", err)
		}
		plantUnreadable(t, filepath.Join(root, ArchiveDir, CardsDir))
		if _, err := opened.Check(); err == nil {
			t.Fatal("Check answered a clean bill over an archived collection nobody could list")
		}
	})

	t.Run("an archived card whose anchor will not load", func(t *testing.T) {
		root := newFixture(t)
		archived := filepath.Join(root, ArchiveDir, CardsDir, "c00000000002")
		// The registry claims the card under a number of its own, so the
		// stranded probe reaches it: the probe owns every card a line claims
		// and the walk below owns the rest, and this case is the probe's half
		// of that partition, which is where a card would be passed over.
		write(t, filepath.Join(root, CardNumbersName), "1 c00000000001\n7 c00000000002\n")
		// The anchor path is built from the CardAnchor constant rather than
		// from a literal, so a rename of that constant is a compile error
		// here instead of a fixture planting a name nothing looks at.
		plantUnreadable(t, filepath.Join(archived, CardAnchor))

		// The expected key is taken from unreadableCardFinding applied to the
		// error this fixture's anchor actually produces, rather than written
		// out here, because that classifier is what the live walk uses and
		// the point of the case is that one key serves both halves.
		_, loadErr := LoadCard(filepath.Join(root, ArchiveDir, CardsDir), "c00000000002")
		if loadErr == nil {
			t.Fatal("the planted anchor loads cleanly, so this case proves nothing")
		}
		wantKey := unreadableCardFinding(loadErr)

		opened, err := Open(root)
		if err != nil {
			t.Fatalf("open: %v", err)
		}
		findings, err := opened.Check()
		if err != nil {
			t.Fatalf("one unloadable card left the sweep unable to finish: %v", err)
		}
		unreadable := findingsOfKey(findings, wantKey)
		if len(unreadable) != 1 {
			t.Fatalf("the archived card whose anchor will not load drew %d findings of key %s: %+v", len(unreadable), wantKey, findings)
		}
		if unreadable[0].Path != archived {
			t.Errorf("the finding names %s, wanted the card's directory %s: one key must not print two spellings depending on which half found the card", unreadable[0].Path, archived)
		}
	})

	t.Run("a live card whose anchor will not load is reported once", func(t *testing.T) {
		root := newFixture(t)
		live := filepath.Join(root, CardsDir, "c00000000003")
		plantUnreadable(t, filepath.Join(live, CardAnchor))

		opened, err := Open(root)
		if err != nil {
			t.Fatalf("open: %v", err)
		}
		findings, err := opened.Check()
		if err != nil {
			t.Fatalf("check: %v", err)
		}
		naming := 0
		for _, finding := range findings {
			if finding.Path == live {
				naming++
			}
		}
		if naming != 1 {
			t.Errorf("the live card is named by %d findings, wanted the one Check's own card walk reports: %+v", naming, findings)
		}
	})
}

// TestCheckPassesOverCardsCarryingNoNumber pins the below-format half of the
// detector's contract.
//
// A workbench the number migration has not reached holds no registry, so the
// by-number index is synthesized from the anchors and feeds resolution alone:
// none of the line findings fire over it, and every card draws one missing
// finding because no line claims it. Two anchors carrying one number collide
// in resolution rather than in check, and a guard that grouped cards by the
// numbers in their anchors would flood exactly the population this detector
// shipped for, a workbench waiting for its migration, with duplicate reports,
// so the genuine collision rides in the fixture and the duplicate report must
// stay empty anyway.
func TestCheckPassesOverCardsCarryingNoNumber(t *testing.T) {
	numberlessCard := "---\ntitle: A card\ncolumn: b00000000001\nstate: ready\n---\nFraming.\n"

	root := preRegistryFixture(t)
	// The fixture card carries number 1 from the pre-registry fixture, and 7
	// is what the collision rides on. The anchor is rewritten rather than any
	// registry because this workbench holds none.
	write(t, filepath.Join(root, CardsDir, "c00000000001", CardAnchor), numberedCard("7"))
	write(t, filepath.Join(root, CardsDir, "c00000000002", CardAnchor), numberlessCard)
	archivedCollides := filepath.Join(root, ArchiveDir, CardsDir, "c00000000003", CardAnchor)
	write(t, archivedCollides, numberedCard("7"))
	write(t, filepath.Join(root, ArchiveDir, CardsDir, "c00000000004", CardAnchor), numberlessCard)

	opened, err := Open(root)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	findings, err := opened.Check()
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if reported := findingsOfKey(findings, FindingCardNumberDuplicate); len(reported) != 0 {
		t.Errorf("a workbench below the registry's format reported %+v, and the synthesized index feeds resolution rather than check", reported)
	}
	missing := findingsOfKey(findings, FindingCardNumberMissing)
	if len(missing) != 4 {
		t.Fatalf("four cards stand on a workbench no line claims, and check reported %d missing findings: %+v", len(missing), missing)
	}
	anchors := map[string]bool{
		filepath.Join(root, CardsDir, "c00000000001", CardAnchor): false,
		filepath.Join(root, CardsDir, "c00000000002", CardAnchor): false,
		archivedCollides: false,
		filepath.Join(root, ArchiveDir, CardsDir, "c00000000004", CardAnchor): false,
	}
	for _, finding := range missing {
		if _, named := anchors[finding.Path]; !named {
			t.Errorf("a missing finding names %s, which is no card anchor in this fixture", finding.Path)
			continue
		}
		anchors[finding.Path] = true
	}
	for anchor, seen := range anchors {
		if !seen {
			t.Errorf("no missing finding names %s", anchor)
		}
	}
	// The synthesized index still answers resolution, and the collision it
	// holds is the one check declined to report: two cards carry number 7, so
	// a reference by number must refuse between them rather than resolve.
	_, refusal := opened.ResolveLinkTarget("fx-7")
	if refusal == nil || refusal.Name != contract.AmbiguousCard {
		t.Fatalf("fx-7 refused %v, and two cards carry the number below the registry's format", refusal)
	}
	if got := strings.Split(refusal.Extra["cards"], "\n"); strings.Join(got, ",") != "c00000000001,c00000000003" {
		t.Errorf("fx-7 carries the rows %v, wanted the live claimant before the archived one", got)
	}
}
