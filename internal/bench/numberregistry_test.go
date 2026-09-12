package bench

import (
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"dinah/internal/contract"
)

// mustRead reads one file the assertions below compare byte for byte, stopping
// the test on a workbench none of them can inspect.
func mustRead(t *testing.T, path string) string {
	t.Helper()
	text, err := ReadText(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return text
}

// migrationCard writes one card of the pre-migration fixture: an anchor
// carrying the number the frontmatter still held before the registry arrived,
// and a journal opening with a created event at the timestamp given, which is
// the first half of the tie-break the migration orders colliding cards by.
func migrationCard(t *testing.T, root, id, number, created string) string {
	t.Helper()
	dir := filepath.Join(root, CardsDir, id)
	write(t, filepath.Join(dir, CardAnchor), numberedCard(number))
	write(t, filepath.Join(dir, JournalName), strings.Replace(cleanJournal, "2026-08-17T09:00:00Z", created, 1))
	return dir
}

// buildOldFormatFixture writes the workbench the number migration is driven
// against: the shape a workbench still waiting for the migration holds, with
// every number in its anchor and no registry file at all. The population is
// the two colliding groups the criteria name and nothing else unusual, so a
// defect in the composition shows as a defect in the groups rather than as
// noise from a crowded fixture.
func buildOldFormatFixture(t *testing.T) string {
	t.Helper()
	root := preRegistryFixture(t)
	// A pair on 5 whose created events differ by a second: the earlier card
	// keeps the number and the later one moves above the high-water mark.
	migrationCard(t, root, "c00000000002", "5", "2026-08-17T09:00:00Z")
	migrationCard(t, root, "c00000000003", "5", "2026-08-17T09:00:01Z")
	// A pair on 9 whose created timestamps are identical, so the identifier
	// decides: ascending order keeps the first and moves the second.
	migrationCard(t, root, "c00000000004", "9", "2026-08-17T09:10:00Z")
	migrationCard(t, root, "c00000000005", "9", "2026-08-17T09:10:00Z")
	// One archived card claiming a number of its own, which is the half a
	// composition that walked the live collection alone would drop.
	archiveDir(t, migrationCard(t, root, "c00000000006", "6", "2026-08-17T09:05:00Z"))
	return root
}

// TestResolveCardReadsTheRegistry is dinah-488 AC-5, the resolution half of the
// registry: every answer a number reference can get comes from the lines, and
// the identifier route never asks for them at all.
//
// The fixture carries a live claim on 1 and on 2, a tombstone holding 3, two
// lines colliding on 4, and a fifth card no line names, so each behaviour
// asserted below has a line of its own to stand on. The tombstone is the case
// that cannot be driven through any other shape, because only a number given
// back and nobody retaken refuses unknown-card while the file still spells
// the number.
func TestResolveCardReadsTheRegistry(t *testing.T) {
	root := newFixture(t)
	for _, id := range []string{"c00000000002", "c00000000003", "c00000000004", "c00000000005"} {
		write(t, filepath.Join(root, CardsDir, id, CardAnchor), cleanCard)
		write(t, filepath.Join(root, CardsDir, id, JournalName), cleanJournal)
	}
	write(t, filepath.Join(root, CardNumbersName),
		"1 c00000000001\n2 c00000000002\n3 -\n4 c00000000003\n4 c00000000004\n")
	opened, err := Open(root)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	for _, c := range []struct {
		ref string
		id  string
	}{
		{ref: "fx-1", id: "c00000000001"},
		{ref: "fx-2", id: "c00000000002"},
	} {
		found, err := opened.resolveCardIn(opened.CardsRoot(), c.ref)
		if err != nil {
			t.Fatalf("%s should resolve to the one card its line claims: %v", c.ref, err)
		}
		if found.Card.ID != c.id {
			t.Errorf("%s resolved to %s, and its line claims %s alone", c.ref, found.Card.ID, c.id)
		}
	}
	// The single-claimant load still reads the half it was given, so a card
	// standing in the live half refuses by number against the mirror.
	if _, err := opened.resolveCardIn(opened.ArchivedCardsRoot(), "fx-1"); mustRefuse(t, err).Name != contract.UnknownCard {
		t.Errorf("fx-1 against the archive refused %v, and its claimant stands in the live half", err)
	}
	for _, ref := range []string{"fx-404", "fx-3"} {
		if _, err := opened.resolveCardIn(opened.CardsRoot(), ref); mustRefuse(t, err).Name != contract.UnknownCard {
			t.Errorf("%s refused %v, and no card claims it: not the line the registry holds, not any anchor", ref, err)
		}
	}
	_, err = opened.resolveCardIn(opened.CardsRoot(), "fx-4")
	refusal := mustRefuse(t, err)
	if refusal.Name != contract.AmbiguousCard {
		t.Fatalf("two lines claim number 4 and the resolution refused %s", refusal.Name)
	}
	if refusal.Detail != "fx-4" {
		t.Errorf("the refusal details %q and the reader typed fx-4", refusal.Detail)
	}
	if got := refusal.Extra["cards"]; got != "c00000000003\nc00000000004" {
		t.Errorf("the refusal carries the rows %q, wanted both claimants in file order", got)
	}
	// The identifier route loads the card directly and never consults the
	// registry, which the fifth card proves: no line names it, and it resolves
	// by identifier all the same.
	found, err := opened.resolveCardIn(opened.CardsRoot(), "c00000000005")
	if err != nil {
		t.Fatalf("the 12-hex reference should resolve without the registry: %v", err)
	}
	if found.Card.ID != "c00000000005" {
		t.Errorf("the identifier resolved to %s, which is not the card it names", found.Card.ID)
	}
}

// TestMigrateNumbersBuildsTheRegistry is dinah-488 AC-6, driven over a fixture
// written directly on disk because the tool cannot produce a duplicate: two
// cards can only arrive holding one number through a hand edit or a union
// merge, and the migration's tie-breaks exist for exactly that state.
//
// The high-water mark over this fixture is 9, and the colliding groups are
// processed in ascending order, so the loser of the 5 group takes 10 and the
// identifier-loser of the 9 group takes 11. The registry's own bytes are
// asserted whole rather than through lookups, because the file's order is
// allocation order and a composition that sorted wrongly would answer every
// lookup correctly.
func TestMigrateNumbersBuildsTheRegistry(t *testing.T) {
	root := buildOldFormatFixture(t)
	opened, err := Open(root)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	written, ids, findings, err := opened.MigrateNumbers("alka", "2026-08-18T09:00:00Z")
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if written != 6 {
		t.Errorf("the migration wrote %d lines, and six cards stand on the workbench", written)
	}
	wantIDs := []string{"c00000000003", "c00000000005"}
	if strings.Join(ids, ",") != strings.Join(wantIDs, ",") {
		t.Errorf("the migration renumbered %v, wanted the later 5-claimant and then the identifier-later 9-claimant", ids)
	}
	if len(findings) != 2 {
		t.Fatalf("the migration reported %d findings, wanted one per renumbered card: %+v", len(findings), findings)
	}
	for at, finding := range findings {
		if finding.Key != FindingCardNumberRenumbered {
			t.Errorf("finding %d reads %s, wanted %s", at, finding.Key, FindingCardNumberRenumbered)
		}
		if finding.Detail != wantIDs[at] {
			t.Errorf("finding %d names %s, and the cards moved were %v", at, finding.Detail, wantIDs)
		}
	}
	wantRegistry := "1 c00000000001\n" +
		"5 c00000000002\n" +
		"6 c00000000006\n" +
		"9 c00000000004\n" +
		"10 c00000000003\n" +
		"11 c00000000005\n"
	if got := mustRead(t, filepath.Join(root, CardNumbersName)); got != wantRegistry {
		t.Errorf("the registry reads\n%s\nand the tie-breaks order it as\n%s", got, wantRegistry)
	}
	for _, id := range []string{
		"c00000000001", "c00000000002", "c00000000003", "c00000000004", "c00000000005",
	} {
		if anchor := mustRead(t, filepath.Join(root, CardsDir, id, CardAnchor)); strings.Contains(anchor, "number:") {
			t.Errorf("the anchor of %s still carries its number:\n%s", id, anchor)
		}
	}
	if anchor := mustRead(t, filepath.Join(root, ArchiveDir, CardsDir, "c00000000006", CardAnchor)); strings.Contains(anchor, "number:") {
		t.Errorf("the archived card's anchor still carries its number:\n%s", anchor)
	}
	definition := mustRead(t, filepath.Join(root, WorkbenchAnchor))
	if !strings.Contains(definition, "format: "+strconv.Itoa(RegistryFormat)+"\n") {
		t.Errorf("the workbench anchor still declares the old format:\n%s", definition)
	}
	for _, moved := range []struct {
		id   string
		from string
		to   string
	}{
		{id: "c00000000003", from: "5", to: "10"},
		{id: "c00000000005", from: "9", to: "11"},
	} {
		events, torn, err := ReadJournal(filepath.Join(root, CardsDir, moved.id, JournalName))
		if err != nil {
			t.Fatalf("read the journal of %s: %v", moved.id, err)
		}
		if torn {
			t.Fatalf("the journal of %s ends in a partial line", moved.id)
		}
		if len(events) != 2 || events[1].Event != contract.EventRenumbered {
			t.Fatalf("the journal of %s carries %d lines, and a renumbered card carries its created line and one renumbered", moved.id, len(events))
		}
		if events[1].From != moved.from || events[1].To != moved.to {
			t.Errorf("the renumbered line on %s reads %s to %s, wanted %s to %s", moved.id, events[1].From, events[1].To, moved.from, moved.to)
		}
		if events[1].Actor != "alka" || events[1].TS != "2026-08-18T09:00:00Z" {
			t.Errorf("the renumbered line on %s reads actor %s at %s, wanted alka at the fixed now", moved.id, events[1].Actor, events[1].TS)
		}
	}
}

// TestMigrateNumbersIsDeterministicAndIdempotent is dinah-488 AC-7.
//
// Determinism is what makes two clones migrating one workbench independently
// agree, which is the whole point of moving the number out of the card: the
// same anchors, the same journals and the same clock must compose the same
// file, or the registry itself becomes the collision it exists to prevent.
// Idempotence is what makes the migration a safe thing to re-run over a
// workbench that already carries it, on the crash story the migration's own
// write order tells: the registry first, the events and the strip behind it,
// the format stamp last, so a workbench still declaring the old format re-runs
// and one declaring the new format has nothing left to do.
func TestMigrateNumbersIsDeterministicAndIdempotent(t *testing.T) {
	const now = "2026-08-18T09:00:00Z"
	first := buildOldFormatFixture(t)
	second := buildOldFormatFixture(t)
	for _, root := range []string{first, second} {
		opened, err := Open(root)
		if err != nil {
			t.Fatalf("open %s: %v", root, err)
		}
		if _, _, _, err := opened.MigrateNumbers("alka", now); err != nil {
			t.Fatalf("migrate %s: %v", root, err)
		}
	}
	if a := mustRead(t, filepath.Join(first, CardNumbersName)); a != mustRead(t, filepath.Join(second, CardNumbersName)) {
		t.Errorf("two copies of one fixture minted different registries:\n%s\n%s",
			mustRead(t, filepath.Join(first, CardNumbersName)), mustRead(t, filepath.Join(second, CardNumbersName)))
	}

	opened, err := Open(first)
	if err != nil {
		t.Fatalf("reopen the migrated workbench: %v", err)
	}
	before := everyFile(t, first)
	written, ids, findings, err := opened.MigrateNumbers("alka", "2026-08-19T09:00:00Z")
	if err != nil {
		t.Fatalf("the second migration: %v", err)
	}
	if written != 0 {
		t.Errorf("the second migration wrote %d lines, and an already migrated workbench needs none", written)
	}
	if len(ids) != 0 {
		t.Errorf("the second migration renumbered %v", ids)
	}
	if len(findings) != 0 {
		t.Errorf("the second migration reported %v", findings)
	}
	after := everyFile(t, first)
	if len(before) != len(after) {
		t.Fatalf("the second migration changed how many files stand, %d before and %d after", len(before), len(after))
	}
	for path, text := range before {
		if after[path] != text {
			t.Errorf("%s changed while the second migration ran", path)
		}
	}
}

// TestCheckReportsTheRegistryDefects is dinah-488 AC-8, the deep audit of the
// six states the registry can be left in. The family in check_test.go breaks
// one defect per case and asserts the key alone; this test holds each
// condition to the finding's whole shape, because the path is what a reader
// opens and the detail is what a reader searches the file for, and a finding
// that named the wrong file or the wrong bytes would pass a key-only check.
//
// The full report is compared rather than the findings of one key, because
// each fixture plants exactly one defect: a second finding riding along is a
// defect in the detector's partition, not noise to filter out. The three
// cases after the table carry the properties the check owed the registry from
// the start: a clean migrated workbench reports nothing at all, an archived
// card that will not load is reported rather than passed over, and a
// workbench whose cards carry no line draws one missing finding per card and
// no duplicate report.
func TestCheckReportsTheRegistryDefects(t *testing.T) {
	registry := func(root string) string { return filepath.Join(root, CardNumbersName) }
	anchor := func(root string) string { return filepath.Join(root, CardsDir, "c00000000001", CardAnchor) }
	cases := []struct {
		name    string
		breakIt func(*testing.T, string)
		want    func(root string) []Finding
	}{
		{
			name: "a line the grammar refuses",
			breakIt: func(t *testing.T, root string) {
				write(t, registry(root), "1 c00000000001\none c00000000001\n")
			},
			want: func(root string) []Finding {
				return []Finding{{Path: registry(root), Key: FindingCardNumberMalformed, Detail: "one c00000000001"}}
			},
		},
		{
			name: "two lines claiming one number",
			breakIt: func(t *testing.T, root string) {
				write(t, filepath.Join(root, CardsDir, "c00000000002", CardAnchor), cleanCard)
				write(t, filepath.Join(root, CardsDir, "c00000000002", JournalName), cleanJournal)
				write(t, registry(root), "1 c00000000001\n1 c00000000002\n")
			},
			want: func(root string) []Finding {
				return []Finding{
					{Path: registry(root), Key: FindingCardNumberDuplicate, Detail: "1 c00000000001"},
					{Path: registry(root), Key: FindingCardNumberDuplicate, Detail: "1 c00000000002"},
				}
			},
		},
		{
			name: "two lines claiming one identifier",
			breakIt: func(t *testing.T, root string) {
				write(t, registry(root), "1 c00000000001\n2 c00000000001\n")
			},
			want: func(root string) []Finding {
				return []Finding{
					{Path: registry(root), Key: FindingCardNumberRepeated, Detail: "1 c00000000001"},
					{Path: registry(root), Key: FindingCardNumberRepeated, Detail: "2 c00000000001"},
				}
			},
		},
		{
			name: "a line naming no card",
			breakIt: func(t *testing.T, root string) {
				write(t, registry(root), "1 c00000000001\n2 c00000000002\n")
			},
			want: func(root string) []Finding {
				return []Finding{{Path: registry(root), Key: FindingCardNumberStranded, Detail: "2 c00000000002"}}
			},
		},
		{
			name: "a card no line claims",
			breakIt: func(t *testing.T, root string) {
				write(t, registry(root), "")
			},
			want: func(root string) []Finding {
				return []Finding{{Path: anchor(root), Key: FindingCardNumberMissing, Detail: "c00000000001"}}
			},
		},
		{
			name: "a card still carrying its number in frontmatter",
			breakIt: func(t *testing.T, root string) {
				edit(t, root, "state: ready", "state: ready\nnumber: 1")
			},
			want: func(root string) []Finding {
				return []Finding{{Path: anchor(root), Key: FindingCardNumberInFrontmatter, Detail: "c00000000001"}}
			},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root := newFixture(t)
			c.breakIt(t, root)
			opened, err := Open(root)
			if err != nil {
				t.Fatalf("open: %v", err)
			}
			findings, err := opened.Check()
			if err != nil {
				t.Fatalf("check: %v", err)
			}
			want := c.want(root)
			if len(findings) != len(want) {
				t.Fatalf("check reported %d findings over one defect, wanted %d: %+v", len(findings), len(want), findings)
			}
			for at := range want {
				if findings[at].Key != want[at].Key {
					t.Errorf("finding %d reads %s, wanted %s", at, findings[at].Key, want[at].Key)
				}
				if findings[at].Path != want[at].Path {
					t.Errorf("finding %d names %s, wanted %s", at, findings[at].Path, want[at].Path)
				}
				if findings[at].Detail != want[at].Detail {
					t.Errorf("finding %d carries the detail %q, wanted %q", at, findings[at].Detail, want[at].Detail)
				}
			}
		})
	}

	t.Run("a clean migrated workbench reports none of the six", func(t *testing.T) {
		opened, err := Open(newFixture(t))
		if err != nil {
			t.Fatalf("open: %v", err)
		}
		findings, err := opened.Check()
		if err != nil {
			t.Fatalf("check: %v", err)
		}
		if len(findings) != 0 {
			t.Fatalf("a clean migrated workbench reports %+v", findings)
		}
	})

	t.Run("an archived card whose anchor will not load is reported rather than passed over", func(t *testing.T) {
		root := newFixture(t)
		archived := filepath.Join(root, ArchiveDir, CardsDir, "c00000000002")
		write(t, registry(root), "1 c00000000001\n7 c00000000002\n")
		plantUnreadable(t, filepath.Join(archived, CardAnchor))
		_, loadErr := LoadCard(filepath.Join(root, ArchiveDir, CardsDir), "c00000000002")
		if loadErr == nil {
			t.Fatal("the planted anchor loads cleanly, so this case proves nothing")
		}
		opened, err := Open(root)
		if err != nil {
			t.Fatalf("open: %v", err)
		}
		findings, err := opened.Check()
		if err != nil {
			t.Fatalf("check: %v", err)
		}
		unreadable := findingsOfKey(findings, unreadableCardFinding(loadErr))
		if len(unreadable) != 1 {
			t.Fatalf("the archived card that will not load drew %d findings, and the line claiming it means it cannot be passed over: %+v", len(unreadable), findings)
		}
		if unreadable[0].Path != archived {
			t.Errorf("the finding names %s, wanted the card's directory %s", unreadable[0].Path, archived)
		}
	})

	t.Run("a workbench whose cards carry no line draws one missing per card and no duplicate", func(t *testing.T) {
		root := newFixture(t)
		write(t, filepath.Join(root, CardsDir, "c00000000002", CardAnchor), cleanCard)
		write(t, filepath.Join(root, CardsDir, "c00000000002", JournalName), cleanJournal)
		write(t, registry(root), "")
		opened, err := Open(root)
		if err != nil {
			t.Fatalf("open: %v", err)
		}
		findings, err := opened.Check()
		if err != nil {
			t.Fatalf("check: %v", err)
		}
		want := []Finding{
			{Path: anchor(root), Key: FindingCardNumberMissing, Detail: "c00000000001"},
			{Path: filepath.Join(root, CardsDir, "c00000000002", CardAnchor), Key: FindingCardNumberMissing, Detail: "c00000000002"},
		}
		if len(findings) != len(want) {
			t.Fatalf("two cards carry no line and check reported %d findings, wanted one missing each: %+v", len(findings), findings)
		}
		for at := range want {
			if findings[at].Key != want[at].Key || findings[at].Path != want[at].Path || findings[at].Detail != want[at].Detail {
				t.Errorf("finding %d reads %+v, wanted %+v", at, findings[at], want[at])
			}
		}
	})
}
