package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dinah/internal/bench"
	"dinah/internal/contract"
	"dinah/internal/msg"
)

// Advice a refusal gives is a promise, and dinah-362 broke a family of those
// promises at once. Narrowing the two repair sweeps from a downward walk to a
// climb took every unqualified `dinah check --migrate-container` and
// `dinah check --migrate-vocabulary` in the catalogs out of reach of a reader
// who is not standing in the workbench the refusal is about, and the reader of
// each of these refusals is very often exactly that: he reached the refusal by
// naming a workbench with --workbench, from wherever he happened to be.
//
// The tests below run each sentence rather than reading it, from both of the
// positions its reader can occupy, because a sentence naming a command and a
// sentence naming a command that does the job read identically. The guard at
// the foot of the file holds the family closed, so a sentence added later
// cannot join it without somebody saying which of the two it is.

// TestTheContainerMigrationAdviceIsACommandThatWorks asserts that the advice
// on dinah.needs-container-migration repairs the workbench it names, from
// wherever the reader typed the invocation that produced it.
//
// The refusal is reachable only by naming the workbench, since a bare
// workbench is exactly what discovery does not return, so its reader is always
// standing somewhere of his own choosing. Both positions here are real: the
// directory above the project, and a subdirectory of the project itself.
func TestTheContainerMigrationAdviceIsACommandThatWorks(t *testing.T) {
	for _, from := range []string{"above the project", "inside the project"} {
		t.Run(from, func(t *testing.T) {
			tree := resolvedDir(t, emptyTree(t))
			project := bareWorkbench(t, filepath.Join(tree, "myproject"))
			where := tree
			if from == "inside the project" {
				where = filepath.Join(project, "src")
			}

			refused := runCLI(t, where, "--workbench", project, "status")
			if !strings.Contains(refused.errw, contract.NeedsContainerMigration) {
				t.Fatalf("naming a bare workbench does not refuse %s: %d %s%s", contract.NeedsContainerMigration, refused.code, refused.out, refused.errw)
			}
			argv := adviceFrom(t, refused.errw, "refusal.dinah.needs-container-migration.next", "detail", project)
			took := runCLI(t, where, argv...)
			if took.code != 0 {
				t.Fatalf("taking the refusal's own advice, dinah %v, exited %d: %s%s", argv, took.code, took.out, took.errw)
			}
			ids := bench.ListWorkbenchIDs(filepath.Join(project, bench.UserBaseName))
			if len(ids) != 1 {
				t.Fatalf("the advice ran and the workbench is not in a container: the container holds %v", ids)
			}
			// The refusal ends "then open it again", so the claim is not that
			// the command exits zero but that the workbench it named opens
			// afterwards.
			opened := runCLI(t, where, "--workbench", filepath.Join(project, bench.UserBaseName, ids[0]), "status")
			if opened.code != 0 {
				t.Errorf("the repaired workbench does not open: %d %s", opened.code, opened.errw)
			}
		})
	}
}

// TestTheVocabularyMigrationAdviceIsACommandThatWorks asserts the same of
// dinah.needs-vocabulary-migration, whose reader can be standing in the
// workbench or naming it from elsewhere.
//
// The half that broke is the second one. A pre-vocabulary workbench sits
// inside a container, so the climb reaches it from inside and the unqualified
// advice went on working there; it is the reader who named the workbench with
// --workbench who was answered with dinah.no-workbench-found instead of the
// repair.
func TestTheVocabularyMigrationAdviceIsACommandThatWorks(t *testing.T) {
	for _, from := range []string{"standing in the workbench", "naming it from elsewhere"} {
		t.Run(from, func(t *testing.T) {
			tree, workbench := preVocabularyFixture(t)
			where, scope := workbench, []string{}
			if from == "naming it from elsewhere" {
				where, scope = tree, []string{"--workbench", workbench}
			}

			refused := runCLI(t, where, append(scope, "status")...)
			if !strings.Contains(refused.errw, contract.NeedsVocabularyMigration) {
				t.Fatalf("a pre-vocabulary workbench does not refuse %s: %d %s%s", contract.NeedsVocabularyMigration, refused.code, refused.out, refused.errw)
			}
			argv := adviceFrom(t, refused.errw, "refusal.dinah.needs-vocabulary-migration.next-named", contract.ValueWorkbench, workbench)
			took := runCLI(t, where, argv...)
			if took.code != 0 {
				t.Fatalf("taking the refusal's own advice, dinah %v, exited %d: %s%s", argv, took.code, took.out, took.errw)
			}
			opened := runCLI(t, where, append(scope, "status")...)
			if opened.code != 0 {
				t.Errorf("the workbench does not open after the advice was followed: %d %s", opened.code, opened.errw)
			}
		})
	}
}

// TestTheMixedVocabularyAdviceIsACommandThatWorks asserts the same of
// dinah.vocabulary-mixed, whose sentence tells the reader to hand-edit the
// file and then run the migration. The hand edit is the repair; the command is
// what carries the workbench forward afterwards, and it has to be runnable
// from where the reader is standing for that sentence to mean anything.
func TestTheMixedVocabularyAdviceIsACommandThatWorks(t *testing.T) {
	for _, from := range []string{"standing in the workbench", "naming it from elsewhere"} {
		t.Run(from, func(t *testing.T) {
			tree, workbench := mixedCardFixture(t)
			where, scope := workbench, []string{}
			if from == "naming it from elsewhere" {
				where, scope = tree, []string{"--workbench", workbench}
			}

			refused := runCLI(t, where, append(scope, "ls")...)
			if !strings.Contains(refused.errw, contract.VocabularyMixed) {
				t.Fatalf("a card carrying both vocabularies does not refuse %s: %d %s%s", contract.VocabularyMixed, refused.code, refused.out, refused.errw)
			}
			argv := adviceFrom(t, refused.errw, "refusal.dinah.vocabulary-mixed.next-named", contract.ValueWorkbench, workbench)
			// The sentence's own first clause, performed: the card is left
			// carrying one vocabulary rather than half of each.
			dropPreVocabularyState(t, workbench)
			took := runCLI(t, where, argv...)
			if took.code != 0 {
				t.Fatalf("taking the refusal's own advice, dinah %v, exited %d: %s%s", argv, took.code, took.out, took.errw)
			}
			listed := runCLI(t, where, append(scope, "ls")...)
			if listed.code != 0 {
				t.Errorf("the workbench does not list after the advice was followed: %d %s", listed.code, listed.errw)
			}
		})
	}
}

// adviceFrom renders one catalog fragment with the values the refusal carried,
// asserts the rendered sentence really is what the run printed, and answers the
// invocation cut out of it.
//
// Rendering and matching are both done because either alone proves less than
// it looks. Rendering alone would let this file assert against a sentence no
// refusal reaches, which is how a fragment behind a condition that never holds
// would pass; matching alone would leave the argument list to be written out
// by hand here, which is reading the advice rather than running it.
func adviceFrom(t *testing.T, printed, key, name, value string) []string {
	t.Helper()
	advice := msg.For(msg.Base).T(key, name, value)
	if !strings.Contains(printed, advice) {
		t.Fatalf("the refusal did not print %s:\nwanted the clause %q\ngot %s", key, advice, printed)
	}
	argv, found := adviceInvocation(advice)
	if !found {
		t.Fatalf("no `dinah ...` invocation was found in the advice %q, so this test would assert nothing", advice)
	}
	return argv
}

// preVocabularyFixture writes one workbench carrying a card, in the shape a
// build before dinah-287 wrote, and answers the directory above it and the
// workbench's own directory. It is unwind's caller rather than a second copy
// of it, so the retired shape is described in one place.
func preVocabularyFixture(t *testing.T) (tree, workbench string) {
	t.Helper()
	tree = resolvedDir(t, emptyTree(t))
	project := filepath.Join(tree, "customer")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if got := runCLI(t, project, "init", project, "--slug", "fx", "--operator", "alka"); got.code != 0 {
		t.Fatalf("init: %d %s", got.code, got.errw)
	}
	workbench = benchDir(t, project)
	if got := runCLI(t, project, "--workbench", workbench, "add", "a card"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}
	unwind(t, workbench)
	return tree, workbench
}

// mixedCardFixture writes one workbench whose anchor declares the current
// vocabulary and one of whose cards carries a key from each, which is the
// shape no writer produces and the one dinah.vocabulary-mixed refuses.
func mixedCardFixture(t *testing.T) (tree, workbench string) {
	t.Helper()
	tree = resolvedDir(t, emptyTree(t))
	project := filepath.Join(tree, "customer")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if got := runCLI(t, project, "init", project, "--slug", "fx", "--operator", "alka"); got.code != 0 {
		t.Fatalf("init: %d %s", got.code, got.errw)
	}
	workbench = benchDir(t, project)
	if got := runCLI(t, project, "--workbench", workbench, "add", "a card"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}
	for _, id := range bench.ListIDs(filepath.Join(workbench, bench.CardsDir)) {
		rewriteFile(t, filepath.Join(workbench, bench.CardsDir, id, bench.CardAnchor), func(text string) string {
			return strings.Replace(text, "\nstate: ", "\nsubstate: ready\nstate: ", 1)
		})
	}
	return tree, workbench
}

// dropPreVocabularyState performs the hand edit dinah.vocabulary-mixed asks
// for on a card: the file is left carrying one vocabulary rather than half of
// each.
func dropPreVocabularyState(t *testing.T, workbench string) {
	t.Helper()
	for _, id := range bench.ListIDs(filepath.Join(workbench, bench.CardsDir)) {
		rewriteFile(t, filepath.Join(workbench, bench.CardsDir, id, bench.CardAnchor), func(text string) string {
			return strings.Replace(text, "\nsubstate: ready", "", 1)
		})
	}
}

// narrowedSweepFlags are the two repair sweeps dinah-362 narrowed from a
// downward walk from the current directory to the ordinary climb.
var narrowedSweepFlags = []string{"--migrate-container", "--migrate-vocabulary"}

// sweepAdviceProvenByRunning names every catalog message that tells a reader to
// run one of the two narrowed sweeps, against the test that follows it rather
// than reading it. A message here is a promise the tool keeps, and the test
// named beside it is what keeps it.
var sweepAdviceProvenByRunning = map[string]string{
	"refusal.dinah.no-workbench-found.bare":               "TestTheBareWorkbenchAdviceIsACommandThatWorks",
	"refusal.dinah.needs-container-migration.next":        "TestTheContainerMigrationAdviceIsACommandThatWorks",
	"refusal.dinah.needs-vocabulary-migration.next-named": "TestTheVocabularyMigrationAdviceIsACommandThatWorks",
	"refusal.dinah.vocabulary-mixed.next-named":           "TestTheMixedVocabularyAdviceIsACommandThatWorks",
}

// sweepAdviceNeedingNoScope names every remaining catalog message that mentions
// one of the two sweeps, with the reason its reader has nothing to run that the
// climb could get wrong.
//
// Two shapes land here. One is a sentence describing what a command does rather
// than asking anybody to type it. The other is a sentence that does ask, and
// reaches its reader only inside a run he has already scoped himself, so the
// invocation he repeats is his own and carries whatever scope he gave it.
var sweepAdviceNeedingNoScope = map[string]string{
	"check.bare-workbench":                          "a finding row printed by a sweep the caller has already scoped, describing what the pending repair does rather than naming a fresh invocation",
	"refusal.dinah.vocabulary-retired.next":         "the instruction is the hand edit, and the command is named only to say which rename to perform by hand; running it would do nothing, because the workbench this refusal fires on already declares the current vocabulary and the sweep skips it",
	"refusal.dinah.needs-vocabulary-migration.next": "the unqualified sibling of next-named, which renders only where the head resolved no single workbench, and that is a sweep over a tree the caller has already named",
	"refusal.dinah.vocabulary-mixed.next":           "the unqualified sibling of next-named, rendering under the same condition and for the same reason",
}

// TestEverySweepAdviceIsDispositioned holds the family of catalog sentences
// naming the two narrowed sweeps closed, in all eight catalogs.
//
// Narrowing the sweeps invalidated advice at several sites at once, and the
// round that first repaired one of them armed the repair only where the last
// reviewer had pointed. The two maps above are what stops that happening a
// third time: a message naming one of the sweeps has to be either a promise
// some test follows or a statement somebody has written down as a statement,
// and a message that is neither fails here rather than shipping unread.
//
// Every catalog is read rather than the base alone. The keys are shared, but
// the text is not, and a translation is free to name a command the English
// does not; a guard reading English only would be blind to exactly the locale
// nobody on this project can proofread.
func TestEverySweepAdviceIsDispositioned(t *testing.T) {
	seen := map[string]bool{}
	for _, tag := range msg.Tags() {
		for _, key := range msg.Keys() {
			entry, ok := msg.CatalogEntry(tag, key)
			if !ok || !namesANarrowedSweep(entry.Text) {
				continue
			}
			seen[key] = true
			_, proven := sweepAdviceProvenByRunning[key]
			_, descriptive := sweepAdviceNeedingNoScope[key]
			switch {
			case proven && descriptive:
				t.Errorf("%s is both proven by running and recorded as needing no scope; one of the two is wrong", key)
			case !proven && !descriptive:
				t.Errorf("the %s catalog's %s names a sweep dinah-362 narrowed, and nothing says whether following it works or that it asks nobody to run anything", tag, key)
			}
		}
	}
	if len(seen) == 0 {
		t.Fatal("no catalog message names either sweep, so this guard read nothing")
	}
	declared := testFunctionsDeclaredHere(t)
	for key, named := range sweepAdviceProvenByRunning {
		if !seen[key] {
			t.Errorf("%s is dispositioned as proven by running and no catalog message by that key names a sweep, so the disposition outlived its message", key)
		}
		// The name of a test is the whole of this disposition's claim, so a
		// name nothing declares is the same defect the MCP head's argument
		// exemptions carried until this card: paperwork reading as an argued
		// decision and standing for nothing. Renaming or deleting the test
		// that follows a sentence has to cost somebody a line here.
		if !declared[named] {
			t.Errorf("%s is dispositioned as proven by %s and this package declares no test by that name", key, named)
		}
	}
	for key := range sweepAdviceNeedingNoScope {
		if !seen[key] {
			t.Errorf("%s is dispositioned as needing no scope and no catalog message by that key names a sweep, so the disposition outlived its message", key)
		}
	}
}

// namesANarrowedSweep reports whether a message names either of the two flags
// whose reach this card narrowed.
func namesANarrowedSweep(text string) bool {
	for _, flag := range narrowedSweepFlags {
		if strings.Contains(text, flag) {
			return true
		}
	}
	return false
}

// testFunctionsDeclaredHere answers the names of every test function this
// package declares, read out of the sources rather than out of a list, so the
// answer cannot go stale on its own.
func testFunctionsDeclaredHere(t *testing.T) map[string]bool {
	t.Helper()
	sources, err := filepath.Glob("*_test.go")
	if err != nil {
		t.Fatalf("glob the test sources: %v", err)
	}
	if len(sources) == 0 {
		t.Fatal("no test source was found, so this read proves nothing")
	}
	declared := map[string]bool{}
	for _, source := range sources {
		text, err := os.ReadFile(source)
		if err != nil {
			t.Fatalf("read %s: %v", source, err)
		}
		for _, line := range strings.Split(string(text), "\n") {
			if !strings.HasPrefix(line, "func Test") {
				continue
			}
			name := strings.TrimPrefix(line, "func ")
			if cut := strings.Index(name, "("); cut > 0 {
				declared[name[:cut]] = true
			}
		}
	}
	return declared
}
