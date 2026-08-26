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
	if got.code == 0 {
		t.Errorf("a preview holding three workbenches still to carry forward exited 0:\n%s", got.out)
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
