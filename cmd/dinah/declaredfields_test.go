package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dinah/internal/bench"
)

// declareFieldsOn writes a fields block into a workbench's own anchor, which
// is the only way one arrives: no verb declares a field, and the block is
// frontmatter a person edits.
func declareFieldsOn(t *testing.T, root, block string) {
	t.Helper()
	dir := soleBenchDir(t, root)
	path := filepath.Join(dir, bench.WorkbenchAnchor)
	text, err := bench.ReadText(path)
	if err != nil {
		t.Fatalf("read the workbench anchor: %v", err)
	}
	fm, body := bench.ParseAnchor(text)
	fm.SetRaw(bench.FieldsKey, bench.SplitLines(strings.TrimSuffix(block, "\n")))
	if err := bench.WriteText(path, fm.Render(body)); err != nil {
		t.Fatalf("write the workbench anchor: %v", err)
	}
}

// cliDeclaration is the block the cases below start from: one key on a card
// alone, and one on every kind because it names none.
const cliDeclaration = `fields:
  git.branch:
    type: string
    meaning: the branch the card's code lives on
    on: [card]
  venue.deposit-paid:
    type: date
    meaning: the day the deposit fell due
`

// TestShowPrintsTheDeclaredFieldsInDeclarationOrder asserts what a reader
// sees. A card prints one line per declared field it carries, in the order the
// workbench declares them, after the two levels and before the holder; a
// declared field the card does not carry prints nothing, and a key the card
// stores that the workbench does not declare prints nothing either.
func TestShowPrintsTheDeclaredFieldsInDeclarationOrder(t *testing.T) {
	root := newBench(t)
	declareFieldsOn(t, root, cliDeclaration)
	if got := runCLI(t, root, "add", "A card"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}
	for _, write := range [][]string{
		{"set", "fx-1", "venue.deposit-paid", "2026-09-14"},
		{"set", "fx-1", "git.branch", "dinah-498-declared-fields"},
	} {
		if got := runCLI(t, root, write...); got.code != 0 {
			t.Fatalf("%v: %d %s", write, got.code, got.errw)
		}
	}
	// A key nothing declares is planted by hand, because no write would land
	// one, and it is what the "prints nothing" half is read against.
	dir := soleBenchDir(t, root)
	anchor := filepath.Join(dir, bench.CardsDir, soleCardID(t, dir), bench.CardAnchor)
	text, err := bench.ReadText(anchor)
	if err != nil {
		t.Fatalf("read the card anchor: %v", err)
	}
	fm, body := bench.ParseAnchor(text)
	bench.SetFieldValue(fm, "imported.key", "kept")
	if err := bench.WriteText(anchor, fm.Render(body)); err != nil {
		t.Fatalf("write the card anchor: %v", err)
	}

	got := runCLI(t, root, "show", "fx-1")
	if got.code != 0 {
		t.Fatalf("show: %d %s", got.code, got.errw)
	}
	lines := strings.Split(got.out, "\n")
	at := func(want string) int {
		for i, line := range lines {
			if strings.TrimSpace(line) == want {
				return i
			}
		}
		t.Errorf("the output carries no line %q:\n%s", want, got.out)
		return -1
	}
	branch := at("git.branch: dinah-498-declared-fields")
	deposit := at("venue.deposit-paid: 2026-09-14")
	if branch < 0 || deposit < 0 {
		return
	}
	if branch > deposit {
		t.Errorf("the two lines print at %d and %d, wanted the order the workbench declares them in:\n%s", branch, deposit, got.out)
	}
	if strings.Contains(got.out, "imported.key") {
		t.Errorf("show prints a key the workbench does not declare:\n%s", got.out)
	}

	// A second card carries no value at all, so no field line prints.
	if added := runCLI(t, root, "add", "Another card"); added.code != 0 {
		t.Fatalf("add: %d %s", added.code, added.errw)
	}
	bare := runCLI(t, root, "show", "fx-2")
	if bare.code != 0 {
		t.Fatalf("show: %d %s", bare.code, bare.errw)
	}
	if strings.Contains(bare.out, "git.branch") || strings.Contains(bare.out, "venue.deposit-paid") {
		t.Errorf("a card carrying no value still prints a field line:\n%s", bare.out)
	}
}

// TestAnUndeclaredWriteIsRefusedWithTheKeysItCouldHaveUsed asserts the
// refusal a person meets: the name, the key they typed, the keys this
// workbench does declare on that kind, and what to do next.
func TestAnUndeclaredWriteIsRefusedWithTheKeysItCouldHaveUsed(t *testing.T) {
	root := newBench(t)
	declareFieldsOn(t, root, cliDeclaration)
	if got := runCLI(t, root, "add", "A card"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}
	got := runCLI(t, root, "set", "fx-1", "git.upstream", "origin")
	if got.code == 0 {
		t.Fatalf("the undeclared write succeeded:\n%s", got.out)
	}
	for _, want := range []string{"undeclared-field", "git.upstream", "git.branch", "venue.deposit-paid", "workbench.md"} {
		if !strings.Contains(got.errw, want) {
			t.Errorf("the refusal does not carry %q:\n%s", want, got.errw)
		}
	}

	// The same key written where the declaration does not reach names the
	// kinds it does reach, which is the other half of this one refusal.
	elsewhere := runCLI(t, root, "set", "doing", "git.branch", "main")
	if elsewhere.code == 0 {
		t.Fatalf("a write to a column the declaration does not name succeeded:\n%s", elsewhere.out)
	}
	if !strings.Contains(elsewhere.errw, "card") {
		t.Errorf("the refusal does not name the kind the declaration lists:\n%s", elsewhere.errw)
	}
}

// TestTheBranchMigrationReportsItsClassificationAndItsWrites asserts the
// command surface of the migration: the preview writes nothing and says so,
// the confirmed run lifts, empties, declares and stamps, and a conflict stops
// the run with a non-zero exit and names every card it cannot carry across.
func TestTheBranchMigrationReportsItsClassificationAndItsWrites(t *testing.T) {
	root := newBench(t)
	dir := soleBenchDir(t, root)
	// The workbench a fresh init writes already declares the format the
	// retirement arrived at, so it is wound back to the one before it: the
	// migration exists for a workbench standing where the retirement finds
	// one, and a fixture already stamped would exercise nothing.
	windBackFormat(t, dir)
	for _, title := range []string{"A card", "An empty heading", "No heading"} {
		if got := runCLI(t, root, "add", title); got.code != 0 {
			t.Fatalf("add %s: %d %s", title, got.code, got.errw)
		}
	}
	setBody(t, root, "fx-1", "Framing.\n\n## Branch\n\ndinah-498-declared-fields\n\nMore framing.\n")
	setBody(t, root, "fx-2", "Framing.\n\n## Branch\n")

	preview := runCLI(t, root, "check", "--migrate-branches")
	if preview.code != 0 {
		t.Fatalf("the preview exited %d: %s", preview.code, preview.errw)
	}
	for _, want := range []string{"Nothing was written", "dinah-498-declared-fields"} {
		if !strings.Contains(preview.out, want) {
			t.Errorf("the preview does not say %q:\n%s", want, preview.out)
		}
	}
	if bodyOf(t, root, "fx-1") == "" || !strings.Contains(bodyOf(t, root, "fx-1"), "## Branch") {
		t.Error("the preview rewrote a body")
	}

	applied := runCLI(t, root, "check", "--migrate-branches", "--yes")
	if applied.code != 0 {
		t.Fatalf("the migration exited %d: %s", applied.code, applied.errw)
	}
	for _, want := range []string{"dinah-498-declared-fields", "git.branch", "4"} {
		if !strings.Contains(applied.out, want) {
			t.Errorf("the migration does not report %q:\n%s", want, applied.out)
		}
	}
	if strings.Contains(bodyOf(t, root, "fx-1"), "## Branch") {
		t.Error("the migrated card still carries the heading")
	}
	shown := runCLI(t, root, "show", "fx-1")
	if !strings.Contains(shown.out, "git.branch: dinah-498-declared-fields") {
		t.Errorf("the lifted value does not print:\n%s", shown.out)
	}

	// A fresh workbench with a conflict on it, so the refusing case is read
	// against a run that would otherwise have written.
	other := newBench(t)
	otherDir := soleBenchDir(t, other)
	windBackFormat(t, otherDir)
	for _, title := range []string{"Two headings", "Would have been lifted"} {
		if got := runCLI(t, other, "add", title); got.code != 0 {
			t.Fatalf("add %s: %d %s", title, got.code, got.errw)
		}
	}
	setBody(t, other, "fx-1", "Framing.\n\n## Branch\n\nfirst\n\n## Branch\n\nsecond\n")
	setBody(t, other, "fx-2", "Framing.\n\n## Branch\n\nwould-have-been-lifted\n")
	conflicted := runCLI(t, other, "check", "--migrate-branches", "--yes")
	if conflicted.code == 0 {
		t.Fatalf("a run that met a conflict exited zero:\n%s", conflicted.out)
	}
	if !strings.Contains(conflicted.out, "fx-1") {
		t.Errorf("the conflict report does not name the card:\n%s", conflicted.out)
	}
	if !strings.Contains(bodyOf(t, other, "fx-2"), "## Branch") {
		t.Error("a run that met a conflict rewrote a card it could have lifted")
	}
}

// windBackFormat stamps a workbench with the format before the retirement, so
// a fixture the tool has just written stands where the migration finds one.
func windBackFormat(t *testing.T, dir string) {
	t.Helper()
	path := filepath.Join(dir, bench.WorkbenchAnchor)
	text, err := bench.ReadText(path)
	if err != nil {
		t.Fatalf("read the workbench anchor: %v", err)
	}
	fm, body := bench.ParseAnchor(text)
	fm.Set("format", "3")
	if err := bench.WriteText(path, fm.Render(body)); err != nil {
		t.Fatalf("write the workbench anchor: %v", err)
	}
}

// setBody writes a card's whole body, which is what the migration reads and
// rewrites.
func setBody(t *testing.T, root, card, body string) {
	t.Helper()
	if got := runCLIWithInput(t, root, strings.NewReader(body), "set", card, "body", "-"); got.code != 0 {
		t.Fatalf("set the body of %s: %d %s", card, got.code, got.errw)
	}
}

// bodyOf reads a card's body back off disk.
func bodyOf(t *testing.T, root, card string) string {
	t.Helper()
	got := runCLI(t, root, "get", card, "body")
	if got.code != 0 {
		t.Fatalf("get the body of %s: %d %s", card, got.code, got.errw)
	}
	return got.out
}

// soleCardID answers the identifier of the one card a workbench carries, for
// a case that has to reach an anchor no command writes.
func soleCardID(t *testing.T, dir string) string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(dir, bench.CardsDir))
	if err != nil {
		t.Fatalf("list the cards: %v", err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			return entry.Name()
		}
	}
	t.Fatal("the workbench carries no card")
	return ""
}

// TestHelpForSetOffersTheDeclaredKeysBesideTheBuiltInNames asserts the second
// vocabulary source. The field argument of `set` reaches a field of a kind's
// own set and a key the reader's own workbench declares alike, so the help
// page offers both; asked where no workbench opens, it offers the half that
// lives in the binary rather than nothing at all.
func TestHelpForSetOffersTheDeclaredKeysBesideTheBuiltInNames(t *testing.T) {
	root := newBench(t)
	declareFieldsOn(t, root, cliDeclaration)
	got := runCLI(t, root, "help", "set")
	if got.code != 0 {
		t.Fatalf("help set: %d %s", got.code, got.errw)
	}
	for _, want := range []string{"severity", "git.branch", "venue.deposit-paid"} {
		if !strings.Contains(got.out, want) {
			t.Errorf("the help page for set does not offer %q:\n%s", want, got.out)
		}
	}

	// The page away from a workbench offers no vocabulary at all rather than
	// half of one. vocabularyValues opens the workbench before it resolves a
	// source that lives in one and answers nothing when that open fails, so
	// this is the head's own rule rather than this source's.
	bare := t.TempDir()
	away := runCLI(t, bare, "help", "set")
	if away.code != 0 {
		t.Fatalf("help set away from a workbench: %d %s", away.code, away.errw)
	}
	if strings.Contains(away.out, "git.branch") {
		t.Errorf("the help page away from a workbench offers a key no workbench declared:\n%s", away.out)
	}
}
