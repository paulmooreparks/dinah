package bench

import (
	"os"
	"path/filepath"
	"testing"
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
// The first case asserts both colliding paths, which is also how it proves
// both halves of the collection were read: checkCardNumbers answers findings
// and nothing else, so there is no examined count to assert, and a sweep that
// read only the live half could not produce a finding naming an archived
// anchor.
func TestCheckReportsADuplicateCardNumber(t *testing.T) {
	// twoHalves writes a live card and an archived card, each carrying the
	// number it is given.
	twoHalves := func(t *testing.T, live, archived string) string {
		t.Helper()
		root := newFixture(t)
		write(t, filepath.Join(root, CardsDir, "c00000000001", CardAnchor), numberedCard(live))
		write(t, filepath.Join(root, ArchiveDir, CardsDir, "c00000000002", CardAnchor), numberedCard(archived))
		return root
	}

	t.Run("two cards carrying one number", func(t *testing.T) {
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
			t.Fatalf("two cards carry number 7 and dinah check reported %d findings: %+v", len(duplicates), duplicates)
		}
		wanted := map[string]bool{
			filepath.Join(root, CardsDir, "c00000000001", CardAnchor):             false,
			filepath.Join(root, ArchiveDir, CardsDir, "c00000000002", CardAnchor): false,
		}
		for _, finding := range duplicates {
			if finding.Detail != "7" {
				t.Errorf("a duplicate finding carries detail %q, wanted the number in decimal", finding.Detail)
			}
			if _, named := wanted[finding.Path]; !named {
				t.Errorf("a duplicate finding names %s, which is neither colliding card's anchor", finding.Path)
				continue
			}
			wanted[finding.Path] = true
		}
		for path, seen := range wanted {
			if !seen {
				t.Errorf("no duplicate finding names %s, so that half of the collection was not read", path)
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
		if reported := findingsOfKey(findings, FindingCardNumberDuplicate); len(reported) != 0 {
			t.Errorf("the detector reported %+v over a workbench where every number is unique", reported)
		}
	})

	t.Run("an archived collection that will not read", func(t *testing.T) {
		// The archived half is a plain file rather than a directory here, so
		// the fixture writes no archived card: the collection nobody can list
		// is the whole of what this case is about.
		root := newFixture(t)
		write(t, filepath.Join(root, CardsDir, "c00000000001", CardAnchor), numberedCard("7"))
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
		write(t, filepath.Join(root, CardsDir, "c00000000001", CardAnchor), numberedCard("7"))
		archived := filepath.Join(root, ArchiveDir, CardsDir, "c00000000002")
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
