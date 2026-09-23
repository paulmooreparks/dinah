package bench

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// migrationFixture is a workbench standing where the retirement finds one: it
// declares the format before the retirement, it declares no field, and it
// carries whichever cards the case plants.
//
// Each card is given as its whole body, because what the migration does to a
// body is the subject and a fixture that composed one would be asserting
// against its own composer.
func migrationFixture(t *testing.T, bodies map[string]string) string {
	t.Helper()
	root := containedPath(t.TempDir())
	write(t, filepath.Join(root, WorkbenchAnchor), strings.Replace(
		benchDefinition, "format: 7", "format: "+strconv.Itoa(FieldsFormat-1), 1))
	write(t, filepath.Join(root, ColumnsDir, "b00000000001", ColumnAnchor), columnDefinition)
	lines := ""
	number := 0
	for id := range bodies {
		number++
		lines += strconv.Itoa(number) + " " + id + "\n"
	}
	write(t, filepath.Join(root, CardNumbersName), lines)
	for id, body := range bodies {
		write(t, filepath.Join(root, CardsDir, id, CardAnchor), body)
		write(t, filepath.Join(root, CardsDir, id, JournalName), cleanJournal)
	}
	return root
}

// cardBody composes one card anchor from its frontmatter tail and its body,
// which is the one thing the fixtures above vary.
func cardBody(extra, body string) string {
	return "---\ntitle: A card\ncolumn: b00000000001\nstate: ready\n" + extra + "---\n" + body
}

// everyFileUnder reads every file beneath a directory, so a run that was to
// write nothing can be held to that byte for byte rather than by sampling.
//
// snapshot in check_test.go answers the same question over the anchors alone,
// which is what the idempotence check there needs. A migration appends to a
// journal as well as rewriting an anchor, so a comparison that read only the
// anchors would call a run clean that had written a journal line.
func everyFileUnder(t *testing.T, root string) map[string]string {
	t.Helper()
	files := map[string]string{}
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		text, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		files[path] = string(text)
		return nil
	})
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	return files
}

// TestTheMigrationLiftsTheHeadingAndReportsWhatItCannotLift asserts the write
// pass. A card carrying a well-formed heading gains the value under the
// declared key and loses the heading from its body, a card carrying the
// heading with nothing beneath it loses the heading and gains no key and is
// named, a card carrying neither is untouched, the workbench declares the key
// and is stamped, and each changed card gains one journal line naming the
// field.
//
// The preview runs first over the same fixture and is held to changing no
// byte, so the two phases are asserted against one set of cards rather than
// against two fixtures that could drift apart.
func TestTheMigrationLiftsTheHeadingAndReportsWhatItCannotLift(t *testing.T) {
	root := migrationFixture(t, map[string]string{
		"c00000000001": cardBody("", "Framing.\n\n## Branch\n\ndinah-498-declared-fields\n\nMore framing.\n"),
		"c00000000002": cardBody("", "Framing.\n\n## Branch\n\n## Next\n\nMore framing.\n"),
		"c00000000003": cardBody("", "Framing with no heading.\n"),
	})

	before := everyFileUnder(t, root)
	opened, err := openFixtureAtAnyFormat(t, root)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	preview, err := opened.MigrateBranches("alka", "2026-09-14T10:00:00Z", false)
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if !preview.Preview || len(preview.Lifted) != 1 || len(preview.Emptied) != 1 || preview.Untouched != 1 {
		t.Errorf("the preview reports %+v, wanted one lift, one empty heading and one untouched card", preview)
	}
	assertUnchanged(t, before, everyFileUnder(t, root), "the preview")

	opened, err = openFixtureAtAnyFormat(t, root)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	report, err := opened.MigrateBranches("alka", "2026-09-14T10:00:00Z", true)
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if report.Preview {
		t.Error("a confirmed run reports itself as a preview")
	}
	if len(report.Lifted) != 1 || report.Lifted[0].Value != "dinah-498-declared-fields" {
		t.Errorf("the run lifted %+v, wanted one value", report.Lifted)
	}
	if len(report.Emptied) != 1 {
		t.Errorf("the run emptied %+v, wanted one card", report.Emptied)
	}
	if report.Untouched != 1 {
		t.Errorf("the run left %d cards untouched, wanted one", report.Untouched)
	}
	if !report.Declared || !report.Stamped {
		t.Errorf("the run declared %v and stamped %v, wanted both", report.Declared, report.Stamped)
	}
	// Every member of the report names a card the one way, so a reader is
	// never shown two spellings of one card. The lifted card's reference is
	// the comparison rather than its identifier, which is what it used to be.
	if len(report.Written) != 2 {
		t.Fatalf("the run records %v as written, wanted the two cards it changed", report.Written)
	}
	for _, ref := range report.Written {
		if strings.HasPrefix(ref, "c0000") {
			t.Errorf("the written account names %q, which is an identifier where every other member names a reference", ref)
		}
	}
	if report.Written[0] != report.Lifted[0].Card && report.Written[1] != report.Lifted[0].Card {
		t.Errorf("the written account %v names neither of the cards Lifted and Emptied name", report.Written)
	}

	migrated, err := openFixtureAtAnyFormat(t, root)
	if err != nil {
		t.Fatalf("open the migrated workbench: %v", err)
	}
	if migrated.Format != FieldsFormat {
		t.Errorf("the workbench declares format %d, wanted %d", migrated.Format, FieldsFormat)
	}
	declared := migrated.DeclaredFieldOf(BranchFieldKey)
	if declared == nil || declared.Type != FieldTypeString || !declared.Declares(KindCard) || declared.Declares(KindColumn) {
		t.Fatalf("the workbench declares %+v for %s", declared, BranchFieldKey)
	}
	lifted, err := migrated.LoadCardIn(migrated.CardsRoot(), "c00000000001")
	if err != nil {
		t.Fatalf("load the lifted card: %v", err)
	}
	if got := FieldValue(lifted.FM, BranchFieldKey); got != "dinah-498-declared-fields" {
		t.Errorf("the lifted card stores %q", got)
	}
	if want := "Framing.\n\nMore framing.\n"; lifted.Body != want {
		t.Errorf("the lifted body is %q, wanted %q", lifted.Body, want)
	}
	emptied, err := migrated.LoadCardIn(migrated.CardsRoot(), "c00000000002")
	if err != nil {
		t.Fatalf("load the emptied card: %v", err)
	}
	if FieldValue(emptied.FM, BranchFieldKey) != "" {
		t.Errorf("the empty heading gained a key: %q", FieldValue(emptied.FM, BranchFieldKey))
	}
	if want := "Framing.\n\n## Next\n\nMore framing.\n"; emptied.Body != want {
		t.Errorf("the emptied body is %q, wanted %q", emptied.Body, want)
	}
	untouched := filepath.Join(root, CardsDir, "c00000000003", CardAnchor)
	if before[untouched] != everyFileUnder(t, root)[untouched] {
		t.Error("the card carrying no heading was rewritten")
	}
	for _, id := range []string{"c00000000001", "c00000000002"} {
		events, _, err := ReadJournal(filepath.Join(root, CardsDir, id, JournalName))
		if err != nil {
			t.Fatalf("journal of %s: %v", id, err)
		}
		if len(events) != 2 || events[1].Field != BranchFieldKey {
			t.Errorf("%s carries %d events, the last naming %q", id, len(events), events[len(events)-1].Field)
		}
	}

	// A second run over the migrated workbench classifies every card as
	// untouched, which is what makes re-running safe after a stop part way.
	again, err := openFixtureAtAnyFormat(t, root)
	if err != nil {
		t.Fatalf("reopen the migrated workbench: %v", err)
	}
	second, err := again.MigrateBranches("alka", "2026-09-14T11:00:00Z", true)
	if err != nil {
		t.Fatalf("second run: %v", err)
	}
	if second.Untouched != 3 || len(second.Lifted) != 0 {
		t.Errorf("the second run reports %+v, wanted three untouched cards", second)
	}
}

// TestAMigrationMeetingAConflictWritesNothing asserts the classification pass.
// A run meeting a conflict names every conflicting card rather than the first,
// leaves every card and the workbench anchor byte-identical, stamps no format,
// declares no field, and reports itself as needing a person.
//
// The fixture also holds a card that would otherwise have been lifted, and the
// assertion below reads that card's own bytes, so a run that classified
// nothing at all cannot pass this.
func TestAMigrationMeetingAConflictWritesNothing(t *testing.T) {
	root := migrationFixture(t, map[string]string{
		"c00000000001": cardBody("field_values:\n  git.branch: something-else\n", "Framing.\n\n## Branch\n\ndinah-498-declared-fields\n"),
		"c00000000002": cardBody("", "Framing.\n\n## Branch\n\nfirst\n\n## Branch\n\nsecond\n"),
		"c00000000003": cardBody("", "Framing.\n\n## Branch\n\nwould-have-been-lifted\n"),
	})
	before := everyFileUnder(t, root)
	opened, err := openFixtureAtAnyFormat(t, root)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	report, err := opened.MigrateBranches("alka", "2026-09-14T10:00:00Z", true)
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if len(report.Conflicts) != 2 {
		t.Fatalf("the run reports %+v, wanted both conflicting cards", report.Conflicts)
	}
	conditions := map[string]bool{}
	for _, conflict := range report.Conflicts {
		conditions[conflict.Condition] = true
	}
	for _, want := range []string{BranchConflictAnchorDiffers, BranchConflictTwoHeadings} {
		if !conditions[want] {
			t.Errorf("no conflict reports the condition %s: %+v", want, report.Conflicts)
		}
	}
	if len(report.Lifted) != 1 {
		t.Errorf("the run classified %d lifts, and the fixture holds one card that would have been lifted", len(report.Lifted))
	}
	if report.Declared || report.Stamped {
		t.Errorf("a run that met a conflict declared %v and stamped %v", report.Declared, report.Stamped)
	}
	if report.Clean() {
		t.Error("a run that met a conflict reports itself clean, so nothing carries the exit code outward")
	}
	assertUnchanged(t, before, everyFileUnder(t, root), "the refused run")

	// The same fixture with both conflicts repaired completes and stamps the
	// format, which pins the refusing case beside the accepting one.
	write(t, filepath.Join(root, CardsDir, "c00000000001", CardAnchor),
		cardBody("", "Framing.\n\n## Branch\n\ndinah-498-declared-fields\n"))
	write(t, filepath.Join(root, CardsDir, "c00000000002", CardAnchor),
		cardBody("", "Framing.\n\n## Branch\n\nfirst\n"))
	repaired, err := openFixtureAtAnyFormat(t, root)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	done, err := repaired.MigrateBranches("alka", "2026-09-14T11:00:00Z", true)
	if err != nil {
		t.Fatalf("second migrate: %v", err)
	}
	if len(done.Conflicts) != 0 || !done.Stamped || !done.Declared {
		t.Errorf("the repaired run reports %+v", done)
	}
}

// TestTheMigrationNormalisesTheBlankLinesAroundARemovedHeading settles
// dinah-498/questions/4. The three shapes no real card carries are built as
// fixtures and read back, and the rule they are held to is that no body comes
// out of the run carrying a leading blank line, a trailing blank line, or two
// blank lines together where the heading used to stand.
func TestTheMigrationNormalisesTheBlankLinesAroundARemovedHeading(t *testing.T) {
	cases := []struct {
		name  string
		body  string
		value string
		want  string
	}{
		{
			name:  "the ordinary shape every real card carries",
			body:  "Framing.\n\n## Branch\n\nwork-branch\n\nMore framing.\n",
			value: "work-branch",
			want:  "Framing.\n\nMore framing.\n",
		},
		{
			name:  "a heading standing first in the body",
			body:  "## Branch\n\nwork-branch\n\nFraming.\n",
			value: "work-branch",
			want:  "Framing.\n",
		},
		{
			name:  "a heading standing last in the body",
			body:  "Framing.\n\n## Branch\n\nwork-branch\n",
			value: "work-branch",
			want:  "Framing.\n",
		},
		{
			name:  "a value line followed immediately by another heading",
			body:  "Framing.\n\n## Branch\nwork-branch\n## Next\n\nMore framing.\n",
			value: "work-branch",
			want:  "Framing.\n\n## Next\n\nMore framing.\n",
		},
		{
			name:  "a heading that is the whole body",
			body:  "## Branch\n\nwork-branch\n",
			value: "work-branch",
			want:  "",
		},
		{
			name:  "an empty heading standing last",
			body:  "Framing.\n\n## Branch\n",
			value: "",
			want:  "Framing.\n",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			at := headingLines(c.body)
			if len(at) != 1 {
				t.Fatalf("the fixture carries %d headings", len(at))
			}
			value, rewritten := liftBranchHeading(c.body, at[0])
			if value != c.value {
				t.Errorf("the value read is %q, wanted %q", value, c.value)
			}
			if rewritten != c.want {
				t.Errorf("the body becomes %q, wanted %q", rewritten, c.want)
			}
			if strings.HasPrefix(rewritten, "\n") {
				t.Errorf("the body opens with a blank line: %q", rewritten)
			}
			if strings.Contains(rewritten, "\n\n\n") {
				t.Errorf("the body carries two blank lines together: %q", rewritten)
			}
			if strings.HasSuffix(rewritten, "\n\n") {
				t.Errorf("the body closes with a blank line: %q", rewritten)
			}
		})
	}
}

// assertUnchanged holds two snapshots to byte equality, naming every file that
// differs rather than the first, and asserts that the snapshot it read was not
// empty, so a walk that found nothing cannot pass for a run that wrote
// nothing.
func assertUnchanged(t *testing.T, before, after map[string]string, what string) {
	t.Helper()
	if len(before) == 0 {
		t.Fatal("the snapshot read no files at all, so this comparison asserts nothing")
	}
	if len(before) != len(after) {
		t.Errorf("%s changed the file count from %d to %d", what, len(before), len(after))
	}
	for path, text := range before {
		if after[path] != text {
			t.Errorf("%s rewrote %s", what, path)
		}
	}
}

// TestTheBranchRepairNeverStampsAStoreDownwards holds the floor the format
// comparison in MigrateBranches is, rather than the equality it used to be.
//
// The equality was written while FieldsFormat was the newest format there
// was, so "equal to" and "at least" were the same test. Later formats arrived
// and they stopped being the same: a store already past this one was stamped
// back down by a repair that only ever meant to raise it. That was survivable
// until dinah-525, which refuses to open a store below the current format, so
// a store stamped down by a repair is a store no ordinary read will open.
//
// Two arms, because the bug had two halves and the fix has two lines. The
// number in the file must not move, and neither must the number the opened
// workbench carries in memory: the second was unconditional even after the
// first was made conditional, so the file kept its format while the value
// held over it said something lower, and the next save for any reason would
// have written that lower number down.
func TestTheBranchRepairNeverStampsAStoreDownwards(t *testing.T) {
	root := migrationFixture(t, map[string]string{
		"c00000000001": cardBody("", "Framing with no heading.\n"),
	})
	// Planted above the format this repair stamps, which is the state every
	// store that has moved on is in.
	above := FieldsFormat + 1
	anchor := filepath.Join(root, WorkbenchAnchor)
	raw, err := os.ReadFile(anchor)
	if err != nil {
		t.Fatalf("read the anchor: %v", err)
	}
	text := string(raw)
	write(t, anchor, strings.Replace(
		text, "format: "+strconv.Itoa(FieldsFormat-1), "format: "+strconv.Itoa(above), 1))

	opened, err := openFixtureAtAnyFormat(t, root)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if opened.Format != above {
		t.Fatalf("the fixture opened at format %d, wanted %d, so this case proves nothing", opened.Format, above)
	}
	report, err := opened.MigrateBranches("alka", "2026-09-14T10:00:00Z", true)
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if report.Stamped {
		t.Error("the repair stamped a store that was already past the format it stamps")
	}
	if opened.Format != above {
		t.Errorf("the opened workbench carries format %d after the repair, and it was %d", opened.Format, above)
	}

	reopened, err := openFixtureAtAnyFormat(t, root)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	if reopened.Format != above {
		t.Errorf("the store records format %d after the repair, and it recorded %d", reopened.Format, above)
	}
}
