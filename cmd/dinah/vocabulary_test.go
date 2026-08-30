package main

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"dinah/internal/bench"
	"dinah/internal/contract"
	"dinah/internal/verb"
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
	unwindCards(t, root)
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

// unwindCards is the half of unwind that touches the cards and nothing else,
// so a test can build the workbench whose cards are written in the retired
// vocabulary under an anchor still declaring the current one. That is what a
// workbench carried across the rename at its anchor and not in its cards looks
// like, and no writer here produces it, so a test that wants the shape has to
// make it by hand.
func unwindCards(t *testing.T, root string) {
	t.Helper()
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

// buildTreeFixture writes four workbenches under one root: one in the root's
// own container, one two levels below it in the same shape, one standing bare
// inside a sibling directory, and one already carrying the current vocabulary.
// It answers the root and the anchor directory of each.
//
// The fixture plants two shapes on purpose, because the tool reaches
// workbenches in both and a fixture holding only one of them cannot tell when
// the other stops working. atRoot and nested carry the shape dinah init
// writes, a workbench inside the .dinah container of a directory, and atRoot
// puts it at the walk's own root, which is the position the walk used to
// probe with a different question and lose (dinah-312). sibling carries a bare
// anchor with no .dinah anywhere on its path, a layout no command produces,
// kept because the --workbench override and the native-home rung of the
// ordinary discovery climb both rest on benchIn's unconditional anchor check
// and nothing else in this fixture exercises it.
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
	// plantBare creates a workbench and moves its anchor directory out of the
	// container dinah init wrote it into, so the anchor stands at a path of
	// the fixture's own choosing with no .dinah above it. Only sibling is
	// planted this way now, and the comment on this function says why that one
	// still is.
	plantBare := func(where string, cards ...string) string {
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
	// plantInContainer creates a workbench exactly where dinah init puts one,
	// inside the .dinah container of the directory it is given, and answers the
	// anchor directory it wrote. Nothing is renamed, so what the fixture holds
	// is what a person running dinah init in that directory would hold.
	//
	// internal/bench/rootenumeration_test.go carries a function of the same
	// name doing the same job for the tests there. The two cannot be one
	// helper, since an unexported helper does not cross a package boundary,
	// and they differ in mechanism on purpose: that one calls
	// bench.Instantiate directly because the package under test is the one
	// that owns it, and this one runs the real command because these tests
	// exercise the command. Each says where the other is so that a reader
	// who changes one finds the other, since nothing in the build, the vet
	// pass or the test run reports a duplicate written across two packages.
	plantInContainer := func(where string, cards ...string) string {
		if err := os.MkdirAll(where, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if got := runCLI(t, where, "init", where, "--slug", "fx", "--operator", "alka"); got.code != 0 {
			t.Fatalf("init at %s: %d %s", where, got.code, got.errw)
		}
		anchor := benchDir(t, where)
		for _, title := range cards {
			if got := runCLI(t, where, "--workbench", anchor, "add", title); got.code != 0 {
				t.Fatalf("add at %s: %d %s", anchor, got.code, got.errw)
			}
		}
		return anchor
	}
	atRoot = plantInContainer(root, "a card at the root")
	nested = plantInContainer(filepath.Join(root, "customer", "project"), "a nested card", "a second nested card")
	sibling = plantBare(filepath.Join(root, "elsewhere"), "a card in a sibling")
	current = plantBare(filepath.Join(root, "already"), "a card that needs nothing")
	// The nested workbench's two cards are stood in the two conditions that
	// are not the default, because an absent state key reads as ready. A
	// fixture whose every card is ready cannot tell a condition carried
	// across the rename from one dropped on the floor, and the wrong-order
	// rename this migration is recovering from drops exactly that key.
	for _, argv := range [][]string{
		{"move", "fx-1", "doing"},
		{"claim", "fx-1"},
		{"move", "fx-2", "doing"},
		{"block", "fx-2", "the fixture wants a card that is not ready"},
	} {
		if got := runCLI(t, root, append([]string{"--workbench", nested}, argv...)...); got.code != 0 {
			t.Fatalf("%v at %s: %d %s", argv, nested, got.code, got.errw)
		}
	}
	for _, where := range []string{atRoot, nested, sibling} {
		unwind(t, where)
	}
	return root, atRoot, nested, sibling, current
}

// standing is where one card stands: the column it sits in, and the condition
// it is in there. The two are read together and compared together because the
// migration renames both keys and the order of the two renames is what went
// wrong, so a check on either half alone passes against a run that lost the
// other.
type standing struct {
	column    string
	condition string
}

// preVocabularyStanding reads where every card of an unwound workbench stands,
// off the retired pair of keys, without opening the workbench. Neither opener
// will read these cards: Open refuses the workbench by name and
// OpenPreVocabulary reads its columns rather than its cards.
func preVocabularyStanding(t *testing.T, root string) map[string]standing {
	t.Helper()
	stood := map[string]standing{}
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
		condition := fm.Value("substate")
		if condition == "" {
			t.Fatalf("the card %s carries no substate key, so this workbench was not unwound", id)
		}
		stood[id] = standing{column: column, condition: condition}
	}
	if len(stood) == 0 {
		t.Fatalf("%s holds no cards, so nothing here can be compared across the run", root)
	}
	return stood
}

// assertConditionVaries fails when every card in view stands in the default
// condition. LoadCard reads an absent state key as ready, so a comparison of
// conditions over cards that are all ready cannot tell a condition the
// migration carried across from one it dropped, and the assertion resting on
// it would be asserting nothing.
func assertConditionVaries(t *testing.T, stood map[string]standing) {
	t.Helper()
	for _, was := range stood {
		if was.condition != contract.StateReady {
			return
		}
	}
	t.Fatalf("every card in view stands in %q, so comparing conditions across the run asserts nothing", contract.StateReady)
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
	stood := map[string]standing{}
	for _, where := range []string{atRoot, nested, sibling} {
		for id, was := range preVocabularyStanding(t, where) {
			stood[id] = was
		}
	}
	assertConditionVaries(t, stood)

	got := runCLI(t, root, "--workbench", root, "check", "--migrate-vocabulary", "--yes")
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
			// Both halves are compared against the values the card stood in
			// before the run, read off the unwound anchor, rather than merely
			// tested for emptiness or for the default. Renaming the two keys
			// in the wrong order leaves a card reading `column: ready` with no
			// state at all, and a card in that shape answers a non-empty
			// column and, because an absent state reads as ready, the
			// right-looking state. Only the values themselves tell the two
			// apart, and only a fixture holding a card that is not ready makes
			// the second comparison say anything.
			was := stood[card.ID]
			if card.Column != was.column {
				t.Errorf("a card of %s stands in column %q and stood in %q before the run", where, card.Column, was.column)
			}
			if card.State != was.condition {
				t.Errorf("a card of %s is in the condition %q and was in %q before the run", where, card.State, was.condition)
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

// denyWrite makes one file of a workbench impossible for the migration to
// write, by the mechanism the platform this test runs on actually uses, and
// restores the permission when the test ends.
//
// The two platforms refuse a write on different grounds, and a test that
// exercises only one of them leaves the other untested while reading green.
// docs/design/format.md says which is which, in the passage on the ordinal
// migration: every write in this format is a temporary file renamed over its
// target, so the right that governs a write is the right to replace a name,
// and "POSIX grants that right through the containing directory, so a
// read-only anchor sitting in a directory its owner can write is replaced,
// while an anchor in a directory nobody can write is refused. Windows asks
// the file's own attribute instead and refuses the read-only anchor."
//
// So Windows is denied by the file's own mode and POSIX by the containing
// directory's. Making the file read-only on Linux and macOS denies nothing at
// all, the migration succeeds, and the assertion that a failure was reported
// fires: that is exactly how this test failed on two of the three platforms
// this ships to while passing on the third.
func denyWrite(t *testing.T, path string) {
	t.Helper()
	target := path
	if runtime.GOOS != "windows" {
		target = filepath.Dir(path)
	}
	info, err := os.Stat(target)
	if err != nil {
		t.Fatalf("stat %s: %v", target, err)
	}
	restore := info.Mode().Perm()
	denied := os.FileMode(0o444)
	if target != path {
		denied = 0o555
	}
	if err := os.Chmod(target, denied); err != nil {
		t.Fatalf("chmod %s: %v", target, err)
	}
	t.Cleanup(func() { os.Chmod(target, restore) })
}

// allowWrite undoes denyWrite ahead of the cleanup, which is what an operator
// does when the report names the cause and asks him to clear it and run the
// migration again.
func allowWrite(t *testing.T, path string) {
	t.Helper()
	target := path
	allowed := os.FileMode(0o644)
	if runtime.GOOS != "windows" {
		target = filepath.Dir(path)
		allowed = 0o755
	}
	if err := os.Chmod(target, allowed); err != nil {
		t.Fatalf("chmod %s: %v", target, err)
	}
}

// TestTheVocabularyMigrationCarriesOnPastAFailure asserts the other half of
// AC-16: a workbench the walk cannot write is reported with its path and its
// reason, the other two are migrated anyway with every card standing where it
// stood, and the exit code says a person is needed.
//
// The surviving workbenches are checked by value rather than by opening. A
// card whose column was replaced by its condition opens perfectly well, so a
// test asking only whether bench.Open answers cleanly reads green against a
// migration that destroyed every card it touched, which is the assertion class
// this file has now produced three times. Carrying on past a failure is worth
// nothing if what the walk carries on to do is wrong, so the two halves are
// asserted together.
func TestTheVocabularyMigrationCarriesOnPastAFailure(t *testing.T) {
	root, atRoot, nested, sibling, _ := buildTreeFixture(t)
	survivors := []string{atRoot, nested}
	// Where every card of the two workbenches the walk should still carry
	// forward stands, read off the unwound anchors before the run. The
	// denied workbench is the sibling rather than the nested one, so that
	// the survivors include the workbench holding the cards that are not
	// ready and the comparison of conditions below says something.
	stood := map[string]map[string]standing{}
	all := map[string]standing{}
	for _, where := range survivors {
		stood[where] = preVocabularyStanding(t, where)
		for id, was := range stood[where] {
			all[id] = was
		}
	}
	assertConditionVaries(t, all)
	denyWrite(t, filepath.Join(sibling, bench.WorkbenchAnchor))

	got := runCLI(t, root, "--workbench", root, "check", "--migrate-vocabulary", "--yes")
	if got.code == 0 {
		t.Errorf("a run with a failed workbench exited 0:\n%s", got.out)
	}
	if !strings.Contains(got.out, sibling) {
		t.Errorf("the report does not name the workbench it could not write:\n%s", got.out)
	}
	for _, where := range survivors {
		if !strings.Contains(got.out, where) {
			t.Errorf("the report stopped naming %s, so the walk did not carry on past the failure:\n%s", where, got.out)
		}
		if _, err := bench.Open(where); err != nil {
			t.Errorf("%s was not migrated, so the walk stopped at the failure: %v", where, err)
			continue
		}
		assertStood(t, where, stood[where])
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

	got := runCLI(t, root, "--workbench", root, "check", "--migrate-vocabulary", "--yes")
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
		// The collection directory answers only for the collection. A run
		// that rewrote every card and then stopped before renaming the
		// directory leaves the same answer there, so the cards are asked
		// separately: a card of a candidate this walk declined to open still
		// carries the retired pair of keys and not the current column key.
		for _, id := range bench.ListIDs(filepath.Join(where, bench.CardsDir)) {
			if cardCarries(t, where, id, "column") {
				t.Errorf("the card %s of %s was rewritten, and this candidate should not have been opened at all", id, where)
			}
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

// migratedStanding reads where every live card of a migrated workbench stands,
// through the ordinary opener, so the comparison against what the cards stood
// in before the run goes through the same reader every other command uses.
func migratedStanding(t *testing.T, root string) map[string]standing {
	t.Helper()
	opened, err := bench.Open(root)
	if err != nil {
		t.Fatalf("open the migrated %s: %v", root, err)
	}
	cards, err := opened.Cards()
	if err != nil {
		t.Fatalf("read the cards of %s: %v", root, err)
	}
	stands := map[string]standing{}
	for _, card := range cards {
		stands[card.ID] = standing{column: card.Column, condition: card.State}
	}
	return stands
}

// assertStood compares where every card of a migrated workbench stands against
// where it stood before the run, by value rather than by presence. A card whose
// column was replaced by its condition still answers a non-empty column, and an
// absent state key reads as ready, so a destroyed card reads back plausibly in
// both fields and only the values themselves tell the two apart.
func assertStood(t *testing.T, root string, stood map[string]standing) {
	t.Helper()
	stands := migratedStanding(t, root)
	if len(stands) != len(stood) {
		t.Errorf("%s holds %d cards and held %d before the run", root, len(stands), len(stood))
	}
	for id, want := range stood {
		got, carried := stands[id]
		if !carried {
			t.Errorf("the card %s of %s is gone", id, root)
			continue
		}
		if got.column != want.column {
			t.Errorf("the card %s of %s stands in column %q and stood in %q before the run", id, root, got.column, want.column)
		}
		if got.condition != want.condition {
			t.Errorf("the card %s of %s is in the condition %q and was in %q before the run", id, root, got.condition, want.condition)
		}
	}
}

// TestASecondVocabularyMigrationChangesNothing asserts the promise the
// migration's own failure report makes. That report asks the operator to clear
// the cause and run the command again, so a second run over a tree the first
// run already carried forward has to be an inspection rather than an act: every
// file byte-identical, every card standing where it stood, and every workbench
// reported as already current.
//
// It is asserted byte for byte rather than field by field because the way this
// went wrong was invisible to a field check. The migration's skip-guard could
// not tell its own output from its input, so the second run renamed each card's
// condition over its column, and the card that came out still carried a column
// key and still carried a state key and still opened and still listed.
func TestASecondVocabularyMigrationChangesNothing(t *testing.T) {
	root, atRoot, nested, sibling, _ := buildTreeFixture(t)
	migrated := []string{atRoot, nested, sibling}
	stood := map[string]map[string]standing{}
	all := map[string]standing{}
	for _, where := range migrated {
		stood[where] = preVocabularyStanding(t, where)
		for id, was := range stood[where] {
			all[id] = was
		}
	}
	assertConditionVaries(t, all)

	first := runCLI(t, root, "--workbench", root, "check", "--migrate-vocabulary", "--yes")
	if first.code != 0 {
		t.Fatalf("the first run: %d %s\n%s", first.code, first.errw, first.out)
	}
	before := map[string]map[string]string{}
	for _, where := range migrated {
		before[where] = treeContents(t, where)
		assertStood(t, where, stood[where])
	}

	// The second run goes through the JSON head so that what it carried
	// forward is read as a count rather than matched against the English
	// heading. A substring match asks after one spelling of one number, so a
	// fourth workbench in the fixture would slip past it and a reworded
	// catalog entry would silently stop it asserting, which is the class of
	// check this file has already had to repair twice.
	second := runCLI(t, root, "--json", "--workbench", root, "check", "--migrate-vocabulary", "--yes")
	if second.code != 0 {
		t.Fatalf("the second run: %d %s\n%s", second.code, second.errw, second.out)
	}
	report := verb.TreeVocabularyReport{}
	if err := json.Unmarshal([]byte(second.out), &report); err != nil {
		t.Fatalf("read the second run's JSON report: %v\n%s", err, second.out)
	}
	if report.Outcome != contract.ReadOK {
		t.Errorf("a walk with nothing left to carry forward reports outcome %q, wanted %q:\n%s", report.Outcome, contract.ReadOK, second.out)
	}
	if len(report.Migrated) != 0 {
		t.Errorf("the second run carried %d workbenches forward, and there was nothing left to carry:\n%s", len(report.Migrated), second.out)
	}
	if len(report.AlreadyCurrent) != len(migrated)+1 {
		t.Errorf("the second run reports %d workbenches as already current, and the tree holds %d", len(report.AlreadyCurrent), len(migrated)+1)
	}
	current := map[string]bool{}
	for _, where := range report.AlreadyCurrent {
		current[where] = true
	}
	for _, where := range migrated {
		if !current[where] {
			t.Errorf("the second run does not report %s as already current, so it did not say what it found there:\n%s", where, second.out)
		}
		after := treeContents(t, where)
		if len(before[where]) != len(after) {
			t.Errorf("%s held %d files after the first run and %d after the second", where, len(before[where]), len(after))
		}
		for path, content := range after {
			if before[where][path] != content {
				t.Errorf("%s changed under a second run that had nothing to do", filepath.Join(where, path))
			}
		}
		assertStood(t, where, stood[where])
	}
}

// TestTheVocabularyMigrationResumesAWorkbenchLeftHalfConverted asserts the
// case that actually bit: a run that fails partway leaves a workbench neither
// old nor new, and the run the operator makes after clearing the cause has to
// finish it without touching what the first run already did.
//
// The half-converted state is produced rather than written by hand, by denying
// the migration the write to one card and letting it fail inside its own card
// loop, so the shape under test is the shape a real failure leaves.
func TestTheVocabularyMigrationResumesAWorkbenchLeftHalfConverted(t *testing.T) {
	root, _, nested, _, _ := buildTreeFixture(t)
	stood := preVocabularyStanding(t, nested)
	assertConditionVaries(t, stood)
	ids := bench.ListIDs(filepath.Join(nested, bench.CardsDir))
	if len(ids) < 2 {
		t.Fatalf("%s holds %d cards, and this test needs one card rewritten before the failure", nested, len(ids))
	}
	// The last identifier in the order the migration walks, so the cards
	// before it are rewritten and the workbench is genuinely half converted
	// rather than untouched.
	last := ids[len(ids)-1]
	denyWrite(t, filepath.Join(nested, bench.CardsDir, last, bench.CardAnchor))

	failed := runCLI(t, root, "--workbench", root, "check", "--migrate-vocabulary", "--yes")
	if failed.code == 0 {
		t.Fatalf("the run that could not write a card exited 0:\n%s", failed.out)
	}
	if !strings.Contains(failed.out, nested) {
		t.Fatalf("the report does not name the workbench it could not finish:\n%s", failed.out)
	}
	carried := 0
	for _, id := range ids {
		if cardCarries(t, nested, id, "column") {
			carried++
		}
	}
	if carried == 0 || carried == len(ids) {
		t.Fatalf("%d of %d cards carry a column key, so the failure did not leave the workbench half converted", carried, len(ids))
	}

	allowWrite(t, filepath.Join(nested, bench.CardsDir, last, bench.CardAnchor))
	resumed := runCLI(t, root, "--workbench", root, "check", "--migrate-vocabulary", "--yes")
	if resumed.code != 0 {
		t.Fatalf("the resumed run: %d %s\n%s", resumed.code, resumed.errw, resumed.out)
	}
	assertStood(t, nested, stood)
}

// cardCarries reports whether one card's header carries a key, read straight
// off the anchor. A half-converted workbench opens under neither opener, so
// nothing here can go through one.
func cardCarries(t *testing.T, root, id, key string) bool {
	t.Helper()
	text, err := os.ReadFile(filepath.Join(root, bench.CardsDir, id, bench.CardAnchor))
	if err != nil {
		t.Fatalf("read the card %s: %v", id, err)
	}
	fm, _ := bench.ParseAnchor(string(text))
	return fm.Has(key)
}

// TestTheVocabularyMigrationWritesNothingWithoutTheConfirmation asserts the
// preview. The root of this walk is a directory rather than a workbench, so a
// run started from a home directory or a drive root reaches every board on the
// machine, and what it does to each one cannot be undone. A bare run therefore
// names what it would carry forward, writes nothing at all, names the flag that
// authorizes the rewrite, and exits non-zero because the work is still waiting.
func TestTheVocabularyMigrationWritesNothingWithoutTheConfirmation(t *testing.T) {
	root, atRoot, nested, sibling, current := buildTreeFixture(t)
	candidates := []string{atRoot, nested, sibling}
	before := map[string]map[string]string{}
	for _, where := range append(candidates, current) {
		before[where] = treeContents(t, where)
	}

	got := runCLI(t, root, "--workbench", root, "check", "--migrate-vocabulary")
	if got.code != contract.ExitCodeForRead(contract.ReadFindings) {
		t.Errorf("a preview holding three workbenches still to carry forward exited %d, wanted %d:\n%s", got.code, contract.ExitCodeForRead(contract.ReadFindings), got.out)
	}
	// The same preview through the JSON head, so the outcome a scripted or
	// MCP-shaped caller reads is asserted beside the exit code a shell reads.
	// dinah-346 added both, and a run carrying one without the other would
	// leave half of this command's callers guessing again.
	machine := runCLI(t, root, "--json", "--workbench", root, "check", "--migrate-vocabulary")
	if machine.code != contract.ExitCodeForRead(contract.ReadFindings) {
		t.Errorf("the JSON preview exited %d, wanted %d:\n%s", machine.code, contract.ExitCodeForRead(contract.ReadFindings), machine.out)
	}
	previewed := verb.TreeVocabularyReport{}
	if err := json.Unmarshal([]byte(machine.out), &previewed); err != nil {
		t.Fatalf("read the JSON preview: %v\n%s", err, machine.out)
	}
	if previewed.Outcome != contract.ReadFindings {
		t.Errorf("a preview with three workbenches left to carry forward reports outcome %q, wanted %q", previewed.Outcome, contract.ReadFindings)
	}
	if !strings.Contains(got.out, "--yes") {
		t.Errorf("the preview does not name the flag that would authorize it:\n%s", got.out)
	}
	for _, where := range candidates {
		if !strings.Contains(got.out, where) {
			t.Errorf("the preview does not name %s, which it would have rewritten:\n%s", where, got.out)
		}
		if !bench.Exists(filepath.Join(where, bench.PreVocabularyDir)) {
			t.Errorf("%s no longer carries a %s directory, so the preview wrote", where, bench.PreVocabularyDir)
		}
	}
	for where, was := range before {
		now := treeContents(t, where)
		if len(was) != len(now) {
			t.Errorf("%s held %d files before the preview and %d after", where, len(was), len(now))
		}
		for path, content := range now {
			if was[path] != content {
				t.Errorf("the preview rewrote %s", filepath.Join(where, path))
			}
		}
	}
}

// TestACardWrittenInTheRetiredVocabularyIsRefusedRatherThanMisread asserts the
// case the version gate cannot see. A workbench whose anchor declares the
// current format passes the gate, so a workbench carried across the rename at
// its anchor and not in its cards is opened, and every card's state key then
// holds a column identifier where the reader expects one of ready, active and
// blocked. Before this refusal, dinah ls printed that identifier under the
// heading naming the card's condition and exited 0, which is the silent
// misread the gate exists to prevent, reached by the one route the gate cannot
// look down.
//
// This migration writes the anchor last, so it cannot produce the shape. A
// hand edit or another tool can, and the fixture makes it by hand for that
// reason.
//
// The refusal this asserts is VocabularyRetired rather than VocabularyMixed,
// and the two are kept apart because their sentences are different sentences.
// A card unwound whole carries one vocabulary and not both, so telling its
// reader to remove a mixture would describe a file that is not there. The
// mixed refusal keeps the shapes that really do hold half of each vocabulary,
// which TestACardCarryingHalfOfEachVocabularyIsRefusedAsMixed covers.
func TestACardWrittenInTheRetiredVocabularyIsRefusedRatherThanMisread(t *testing.T) {
	root, _, _, _, current := buildTreeFixture(t)
	unwindCards(t, current)

	for _, argv := range [][]string{{"ls"}, {"status"}, {"show", "fx-1"}} {
		got := runCLI(t, root, append([]string{"--workbench", current}, argv...)...)
		if got.code == 0 {
			t.Errorf("%v over a workbench whose cards were never carried across exited 0:\n%s", argv, got.out)
		}
		if !strings.Contains(got.errw, contract.VocabularyRetired) {
			t.Errorf("%v refused with %q, wanted the %s refusal", argv, got.errw, contract.VocabularyRetired)
		}
	}

	// The refusal names the card, not merely the anchor filename. A board of
	// any size makes the difference between an operator who knows where to
	// look and one who does not.
	ids := bench.ListIDs(filepath.Join(current, bench.CardsDir))
	if len(ids) == 0 {
		t.Fatalf("%s holds no cards, so this test asserts nothing", current)
	}
	got := runCLI(t, root, "--workbench", current, "ls")
	if !strings.Contains(got.errw, ids[0]) {
		t.Errorf("the refusal does not name the card %s:\n%s", ids[0], got.errw)
	}
}

// TestAnAnchorCarryingBothSequenceKeysIsRefusedBeforeAnythingIsWritten asserts
// the other end of the same class. An anchor carrying states: beside columns:
// is half of each vocabulary, and the rename refuses rather than choosing
// which of the two lists the workbench's flow really is. The refusal used to
// come last, after every card and the whole collection had been rewritten,
// which left the workbench opening under neither opener and refusing
// identically on every later run. It now comes first, so the workbench is left
// exactly as it was found and one hand edit frees it.
func TestAnAnchorCarryingBothSequenceKeysIsRefusedBeforeAnythingIsWritten(t *testing.T) {
	// The sibling rather than the workbench at the root, because this test
	// compares one workbench byte for byte and the sibling is the one the rest
	// of the fixture leaves alone.
	root, _, _, sibling, _ := buildTreeFixture(t)
	rewriteFile(t, filepath.Join(sibling, bench.WorkbenchAnchor), func(text string) string {
		return strings.Replace(text, "\nstates:\n", "\ncolumns:\n- b00000000001\nstates:\n", 1)
	})
	before := treeContents(t, sibling)

	got := runCLI(t, root, "--workbench", root, "check", "--migrate-vocabulary", "--yes")
	if got.code == 0 {
		t.Errorf("a run meeting an anchor in both vocabularies exited 0:\n%s", got.out)
	}
	if !strings.Contains(got.out, contract.VocabularyMixed) {
		t.Errorf("the report does not carry the %s refusal:\n%s", contract.VocabularyMixed, got.out)
	}
	// The report must not leak the frontmatter package's own error text, which
	// names an internal package to an operator and tells him nothing he can act
	// on.
	if strings.Contains(got.out, "frontmatter:") {
		t.Errorf("the report leaks an internal package name:\n%s", got.out)
	}
	after := treeContents(t, sibling)
	if len(before) != len(after) {
		t.Fatalf("%s held %d files before the refused run and %d after", sibling, len(before), len(after))
	}
	for path, content := range after {
		if before[path] != content {
			t.Errorf("%s changed under a run that refused the workbench", filepath.Join(sibling, path))
		}
	}
}

// TestACardCarryingBothVocabulariesIsRefusedByName asserts that the migration's
// own card guard names the card it refused. The report of a tree-wide run
// prints the workbench's path, so a refusal naming only the anchor filename
// tells an operator that one card of several hundred is the problem and does
// not tell him which.
func TestACardCarryingBothVocabulariesIsRefusedByName(t *testing.T) {
	root, atRoot, _, _, _ := buildTreeFixture(t)
	ids := bench.ListIDs(filepath.Join(atRoot, bench.CardsDir))
	if len(ids) == 0 {
		t.Fatalf("%s holds no cards", atRoot)
	}
	rewriteFile(t, filepath.Join(atRoot, bench.CardsDir, ids[0], bench.CardAnchor), func(text string) string {
		return strings.Replace(text, "\nstate: ", "\ncolumn: b00000000001\nstate: ", 1)
	})

	got := runCLI(t, root, "--workbench", root, "check", "--migrate-vocabulary", "--yes")
	if got.code == 0 {
		t.Errorf("a run meeting a card in both vocabularies exited 0:\n%s", got.out)
	}
	if !strings.Contains(got.out, contract.VocabularyMixed) {
		t.Errorf("the report does not carry the %s refusal:\n%s", contract.VocabularyMixed, got.out)
	}
	if !strings.Contains(got.out, ids[0]) {
		t.Errorf("the report does not name the card %s it refused:\n%s", ids[0], got.out)
	}
}

// TestTheVocabularyMigrationSeesTheWorkbenchYouAreStandingIn is dinah-312
// AC-3, and it is the card's own reproduction turned into a test: dinah init
// in a directory, that workbench carried back to the revision before the
// rename, and dinah check --migrate-vocabulary run from inside the directory
// with no flag naming anything.
//
// The command used to report zero workbenches and write nothing there, and the
// next command the operator typed then refused the same workbench for needing
// the migration he had just been told there was nothing to do. Running it one
// directory higher found the workbench, so the two runs disagreed about a
// board neither of them had to look far for.
func TestTheVocabularyMigrationSeesTheWorkbenchYouAreStandingIn(t *testing.T) {
	base := t.TempDir()
	board := filepath.Join(base, "board")
	t.Setenv("DINAH_HOME", filepath.Join(base, "home"))
	t.Setenv("DINAH_ACTOR", "alka")
	t.Setenv("DINAH_LANG", "")
	t.Setenv("DINAH_FORMAT", "")
	t.Setenv("DINAH_WORKBENCH", "")
	if err := os.MkdirAll(board, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", board, err)
	}
	if got := runCLI(t, board, "init", ".", "--slug", "repro", "--operator", "paul"); got.code != 0 {
		t.Fatalf("init at %s: %d %s", board, got.code, got.errw)
	}
	anchor := benchDir(t, board)
	// This run names no workbench, so the head takes its own working
	// directory as the root and every path it prints is spelled the way
	// os.Getwd spells that directory. anchor is joined onto the fixture's
	// t.TempDir() value instead, and the two spellings of one directory
	// agree here today with nothing promising they will: a temporary
	// directory sitting behind a symlink, or one handed out as a short
	// name, spells itself differently through the two routes. resolvedDir
	// reproduces the head's own os.Chdir/os.Getwd sequence rather than
	// normalising one side of the comparison, which is the remedy this
	// package already carries for the hazard (dinah-312). The other uses
	// of anchor below hand the path to the head or to bench.Open as an
	// argument, and each of those resolves what it is given, so they stay
	// on the fixture's spelling.
	resolvedAnchor := resolvedDir(t, anchor)
	if err := os.MkdirAll(filepath.Join(board, bench.CardsDir), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if got := runCLI(t, board, "--workbench", anchor, "add", "a card the migration carries"); got.code != 0 {
		t.Fatalf("add at %s: %d %s", anchor, got.code, got.errw)
	}
	unwind(t, anchor)
	stood := preVocabularyStanding(t, anchor)

	// The preview first, because that is what the operator's own run was, and
	// what it answered was that there was nothing there.
	preview := runCLI(t, board, "check", "--migrate-vocabulary")
	if !strings.Contains(preview.out, resolvedAnchor) {
		t.Errorf("a preview run from %s does not name the workbench standing in it:\n%s", board, preview.out)
	}
	if preview.code == 0 {
		t.Errorf("a preview holding a workbench still to carry forward exited 0:\n%s", preview.out)
	}
	if !bench.Exists(filepath.Join(anchor, bench.PreVocabularyDir)) {
		t.Errorf("%s no longer carries a %s directory, so the preview wrote", anchor, bench.PreVocabularyDir)
	}

	got := runCLI(t, board, "check", "--migrate-vocabulary", "--yes")
	if got.code != 0 {
		t.Fatalf("migrate from %s: %d %s\n%s", board, got.code, got.errw, got.out)
	}
	if !strings.Contains(got.out, resolvedAnchor) {
		t.Fatalf("the report does not name the workbench standing in %s:\n%s", board, got.out)
	}
	opened, err := bench.Open(anchor)
	if err != nil {
		t.Fatalf("the migrated %s does not open: %v", anchor, err)
	}
	if opened.Profile != bench.ProfileVersion {
		t.Errorf("the migrated %s declares %s", anchor, opened.Profile)
	}
	assertStood(t, anchor, stood)
}
