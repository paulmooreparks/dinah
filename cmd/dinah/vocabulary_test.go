package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"dinah/internal/bench"
)

// unwind is what a workbench written before dinah-287 carries, applied to one
// this build wrote: the two frontmatter keys on every card, the collection
// directory, each member's anchor filename, the anchor's own sequence key, and
// the revision it declares.
//
// The fixtures are unwound rather than checked in, because a fixture directory
// is evidence of a shape somebody captured and these trees are scaffolding for
// one test. The compat corpus under internal/bench/testdata already holds the
// captured pre-vocabulary shapes, and this unwinding is asserted against them
// there: internal/bench's own interchange test migrates every one of those
// four fixtures and reads the result back.
func unwind(t *testing.T, root string) {
	t.Helper()
	anchor := filepath.Join(root, bench.WorkbenchAnchor)
	rewriteFile(t, anchor, func(text string) string {
		text = strings.Replace(text, "\ncolumns:\n", "\nstates:\n", 1)
		return strings.Replace(text, "profile: "+bench.ProfileVersion, "profile: dinah-core/0.6", 1)
	})
	for _, dir := range []string{
		filepath.Join(root, bench.CardsDir),
		filepath.Join(root, bench.ArchiveDir, bench.CardsDir),
	} {
		for _, id := range bench.ListIDs(dir) {
			card := filepath.Join(dir, id, bench.CardAnchor)
			if !bench.Exists(card) {
				continue
			}
			rewriteFile(t, card, func(text string) string {
				text = strings.Replace(text, "\nstate: ", "\nsubstate: ", 1)
				return strings.Replace(text, "\ncolumn: ", "\nstate: ", 1)
			})
		}
	}
	for _, parent := range []string{root, filepath.Join(root, bench.ArchiveDir)} {
		columns := filepath.Join(parent, bench.ColumnsDir)
		if !bench.Exists(columns) {
			continue
		}
		for _, id := range bench.ListIDs(columns) {
			from := filepath.Join(columns, id, bench.ColumnAnchor)
			if !bench.Exists(from) {
				continue
			}
			if err := os.Rename(from, filepath.Join(columns, id, bench.PreVocabularyAnchor)); err != nil {
				t.Fatalf("rename the column anchor: %v", err)
			}
		}
		if err := os.Rename(columns, filepath.Join(parent, bench.PreVocabularyDir)); err != nil {
			t.Fatalf("rename the collection: %v", err)
		}
	}
}

// rewriteFile reads a file, hands its text to a function, and writes back what
// comes out, failing when the edit found nothing to change.
func rewriteFile(t *testing.T, path string, edit func(string) string) {
	t.Helper()
	text, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	edited := edit(string(text))
	if edited == string(text) {
		t.Fatalf("the edit to %s changed nothing, so the fixture is not the shape this test builds on:\n%s", path, text)
	}
	if err := os.WriteFile(path, []byte(edited), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// buildTreeFixture writes four workbenches under one root: one at the root
// itself, one nested two levels below it, one inside a sibling directory, and
// one already carrying the current vocabulary. It answers the root and the
// anchor directory of each.
//
// The root itself carries a workbench because bench.Enumerate tests a root's
// children and never the root, so a walk relying on it alone loses exactly this
// one, and a fixture that never built it could not tell.
func buildTreeFixture(t *testing.T) (root string, atRoot, nested, sibling, current string) {
	t.Helper()
	base := t.TempDir()
	root = filepath.Join(base, "trees")
	t.Setenv("DINAH_HOME", filepath.Join(base, "home"))
	t.Setenv("DINAH_ACTOR", "alka")
	t.Setenv("DINAH_LANG", "")
	t.Setenv("DINAH_FORMAT", "")
	t.Setenv("DINAH_WORKBENCH", "")
	planted := 0
	// plant creates a workbench and moves its anchor directory to where the
	// fixture wants it. dinah init always writes into a .dinah base, and what
	// this fixture needs is anchor directories standing at paths of its own
	// choosing, including one standing at the walk's root.
	plant := func(where string, cards ...string) string {
		planted++
		scratch := filepath.Join(base, "planting", strconv.Itoa(planted))
		if err := os.MkdirAll(scratch, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if got := runCLI(t, scratch, "init", scratch, "--slug", "fx", "--operator", "alka"); got.code != 0 {
			t.Fatalf("init at %s: %d %s", scratch, got.code, got.errw)
		}
		anchor := benchDir(t, scratch)
		for _, title := range cards {
			if got := runCLI(t, scratch, "--workbench", anchor, "add", title); got.code != 0 {
				t.Fatalf("add at %s: %d %s", anchor, got.code, got.errw)
			}
		}
		if err := os.MkdirAll(filepath.Dir(where), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.Rename(anchor, where); err != nil {
			t.Fatalf("plant %s: %v", where, err)
		}
		return where
	}
	atRoot = plant(root, "a card at the root")
	nested = plant(filepath.Join(root, "customer", "project"), "a nested card", "a second nested card")
	sibling = plant(filepath.Join(root, "elsewhere"), "a card in a sibling")
	current = plant(filepath.Join(root, "already"), "a card that needs nothing")
	for _, where := range []string{atRoot, nested, sibling} {
		unwind(t, where)
	}
	return root, atRoot, nested, sibling, current
}

// preVocabularyColumns reads the column identifier every card of an unwound
// workbench stands in, off the old state key, without opening the workbench.
// Neither opener will read these cards: Open refuses the workbench by name and
// OpenPreVocabulary reads its columns rather than its cards.
func preVocabularyColumns(t *testing.T, root string) map[string]string {
	t.Helper()
	stood := map[string]string{}
	dir := filepath.Join(root, bench.CardsDir)
	for _, id := range bench.ListIDs(dir) {
		text, err := os.ReadFile(filepath.Join(dir, id, bench.CardAnchor))
		if err != nil {
			t.Fatalf("read the card %s: %v", id, err)
		}
		fm, _ := bench.ParseAnchor(string(text))
		column := fm.Value("state")
		if column == "" {
			t.Fatalf("the card %s carries no state key, so this workbench was not unwound", id)
		}
		stood[id] = column
	}
	if len(stood) == 0 {
		t.Fatalf("%s holds no cards, so nothing here can be compared across the run", root)
	}
	return stood
}

// TestTheVocabularyMigrationWalksTheWholeTree asserts dinah-287 AC-16: every
// pre-vocabulary workbench at or beneath the root is carried across the rename,
// including the one standing at the root itself, and a workbench already
// carrying the current vocabulary is reported and left alone.
func TestTheVocabularyMigrationWalksTheWholeTree(t *testing.T) {
	root, atRoot, nested, sibling, current := buildTreeFixture(t)
	before := treeContents(t, current)
	// Where every card stands before the run, read off the pre-vocabulary
	// anchors, so the run can be checked for having carried the value across
	// rather than merely for having written the key.
	stood := map[string]string{}
	for _, where := range []string{atRoot, nested, sibling} {
		for id, column := range preVocabularyColumns(t, where) {
			stood[id] = column
		}
	}

	got := runCLI(t, root, "--workbench", root, "check", "--migrate-vocabulary")
	if got.code != 0 {
		t.Fatalf("migrate the tree: %d %s\n%s", got.code, got.errw, got.out)
	}
	for _, where := range []string{atRoot, nested, sibling} {
		if !strings.Contains(got.out, where) {
			t.Errorf("the report does not name %s:\n%s", where, got.out)
		}
		if bench.Exists(filepath.Join(where, bench.PreVocabularyDir)) {
			t.Errorf("%s still carries a %s directory", where, bench.PreVocabularyDir)
		}
		if !bench.Exists(filepath.Join(where, bench.ColumnsDir)) {
			t.Errorf("%s carries no %s directory", where, bench.ColumnsDir)
		}
		opened, err := bench.Open(where)
		if err != nil {
			t.Errorf("the migrated %s does not open: %v", where, err)
			continue
		}
		if opened.Profile != bench.ProfileVersion {
			t.Errorf("the migrated %s declares %s", where, opened.Profile)
		}
		cards, err := opened.Cards()
		if err != nil {
			t.Fatalf("read the cards of %s: %v", where, err)
		}
		if len(cards) == 0 {
			t.Errorf("the migrated %s reports no cards", where)
		}
		declared := map[string]bool{}
		for _, column := range opened.Columns {
			declared[column.ID] = true
		}
		for _, card := range cards {
			if card.State != "ready" {
				t.Errorf("a card of %s carries the state %q, wanted the condition rather than a column identifier", where, card.State)
			}
			// The column is compared against the identifier the card stood in
			// before the run, read off the unwound anchor, rather than merely
			// tested for emptiness. Renaming the two keys in the wrong order
			// leaves a card reading `column: ready` with no state at all, and
			// a card in that shape answers a non-empty column and, because an
			// absent state reads as ready, the right-looking state. Only the
			// value itself tells the two apart.
			if want := stood[card.ID]; card.Column != want {
				t.Errorf("a card of %s stands in column %q and stood in %q before the run", where, card.Column, want)
			}
			if !declared[card.Column] {
				t.Errorf("a card of %s names the column %q, which the workbench does not declare", where, card.Column)
			}
		}
	}
	if !strings.Contains(got.out, current) {
		t.Errorf("the report does not name the workbench that needed nothing:\n%s", got.out)
	}
	after := treeContents(t, current)
	if len(before) != len(after) {
		t.Fatalf("the untouched workbench held %d files before the run and %d after", len(before), len(after))
	}
	for path, content := range after {
		if before[path] != content {
			t.Errorf("%s changed under a run that had nothing to do there", path)
		}
	}
}

// TestTheVocabularyMigrationCarriesOnPastAFailure asserts the other half of
// AC-16: a workbench the walk cannot write is reported with its path and its
// reason, the other two are migrated anyway, and the exit code says a person is
// needed.
func TestTheVocabularyMigrationCarriesOnPastAFailure(t *testing.T) {
	root, atRoot, nested, sibling, _ := buildTreeFixture(t)
	// The anchor is made unwritable rather than the directory, because the
	// migration's last write is to that file and a directory permission does
	// not stop a rename on every platform this runs on.
	locked := filepath.Join(nested, bench.WorkbenchAnchor)
	if err := os.Chmod(locked, 0o444); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { os.Chmod(locked, 0o644) })

	got := runCLI(t, root, "--workbench", root, "check", "--migrate-vocabulary")
	if got.code == 0 {
		t.Errorf("a run with a failed workbench exited 0:\n%s", got.out)
	}
	if !strings.Contains(got.out, nested) {
		t.Errorf("the report does not name the workbench it could not write:\n%s", got.out)
	}
	for _, where := range []string{atRoot, sibling} {
		if !strings.Contains(got.out, where) {
			t.Errorf("the report stopped naming %s, so the walk did not carry on past the failure:\n%s", where, got.out)
		}
		if _, err := bench.Open(where); err != nil {
			t.Errorf("%s was not migrated, so the walk stopped at the failure: %v", where, err)
		}
	}
}

// TestTheVocabularyMigrationReportsWhatItWillNotOpen asserts the two outcomes
// that are neither a migration nor a failure: a candidate declaring a revision
// outside the window this migration covers, and one whose anchor carries no
// profile at all. Neither is opened, both are named, and a run holding only
// those two exits non-zero on the malformed one alone.
func TestTheVocabularyMigrationReportsWhatItWillNotOpen(t *testing.T) {
	root, atRoot, nested, _, _ := buildTreeFixture(t)
	rewriteFile(t, filepath.Join(atRoot, bench.WorkbenchAnchor), func(text string) string {
		return strings.Replace(text, "profile: dinah-core/0.6", "profile: dinah-core/9.9", 1)
	})
	rewriteFile(t, filepath.Join(nested, bench.WorkbenchAnchor), func(text string) string {
		return strings.Replace(text, "profile: dinah-core/0.6\n", "", 1)
	})

	got := runCLI(t, root, "--workbench", root, "check", "--migrate-vocabulary")
	if got.code == 0 {
		t.Errorf("a run holding a candidate it could not classify exited 0:\n%s", got.out)
	}
	if !strings.Contains(got.out, "dinah-core/9.9") {
		t.Errorf("the report does not carry the revision it declined to open:\n%s", got.out)
	}
	for _, where := range []string{atRoot, nested} {
		if !strings.Contains(got.out, where) {
			t.Errorf("the report does not name %s:\n%s", where, got.out)
		}
		if bench.Exists(filepath.Join(where, bench.ColumnsDir)) {
			t.Errorf("%s was migrated, and neither of these candidates should be opened at all", where)
		}
	}
}

// treeContents reads every file of a tree, so a test can assert that a run left
// a workbench it had nothing to do in byte-identical.
func treeContents(t *testing.T, root string) map[string]string {
	t.Helper()
	contents := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		contents[relative] = string(data)
		return nil
	})
	if err != nil {
		t.Fatalf("read %s: %v", root, err)
	}
	return contents
}
