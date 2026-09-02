package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
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
			adviceLeavesTheConfirmationToTheReader(t, argv)
			// The advice as printed previews, which is the findings code 5,
			// and the preview's own foot carries the reader to the second
			// step. Both steps are run here, because a test that took only
			// the second would pass against advice that skipped the first.
			previewed := runCLI(t, where, argv...)
			if previewed.code != 5 {
				t.Fatalf("taking the refusal's own advice, dinah %v, exited %d rather than reaching the preview: %s%s", argv, previewed.code, previewed.out, previewed.errw)
			}
			if !strings.Contains(previewed.out, workbench) {
				t.Errorf("the preview the advice reached does not name the workbench it would carry forward:\n%s", previewed.out)
			}
			took := runCLI(t, where, append(append([]string{}, argv...), "--yes")...)
			if took.code != 0 {
				t.Fatalf("the advice, confirmed, exited %d: %s%s", took.code, took.out, took.errw)
			}
			opened := runCLI(t, where, append(scope, "status")...)
			if opened.code != 0 {
				t.Errorf("the workbench does not open after the advice was followed: %d %s", opened.code, opened.errw)
			}
		})
	}
}

// TestTheClimbingSweepRepairsRatherThanRefuses holds the premise the two
// unqualified sweep-advice dispositions rest on, which is that no refusal
// carrying that advice is composed on a path that never opened a workbench.
//
// The unqualified fragment renders when nameTheWorkbench finds workbenchRoot
// empty, and a refusal composed without an open is what leaves it empty. The
// nearest such path in the tree is sweepRoot, which discovers a workbench and
// then deliberately stops short of opening it, because opening a workbench
// that needs the vocabulary carried forward refuses that workbench by name and
// the repair would refuse to run on exactly the workbenches it exists for. The
// doc comment there says as much, and the next reader who puts the open back
// is the reader who has not read it.
//
// So the premise is exercised rather than argued. Each run below is the
// climbing form of the sweep over a workbench the sweep exists to repair, and
// each asserts that the run reaches the preview and prints neither the refusal
// it came to clear nor the sentence that names no workbench. Four lines in
// sweepRoot that open what it discovered turn both of these red and put the
// circular advice of round one back on the screen, which is the plant this
// test was written from.
//
// The blind spot is worth naming, because this guard is narrower than the
// disposition it supports. It watches the two sweeps. A raise site anywhere
// else that composed one of these refusals without an open would print the
// unqualified sentence and nothing here would see it, and the same is true of
// a workbench shape the fixture below does not build. That residue is held by
// reading the raise sites, which is recorded above sweepAdviceNeedingNoScope
// rather than claimed as covered.
func TestTheClimbingSweepRepairsRatherThanRefuses(t *testing.T) {
	for _, from := range []string{"standing in the workbench", "naming it from elsewhere"} {
		t.Run(from, func(t *testing.T) {
			tree, workbench := preVocabularyFixture(t)
			where, scope := workbench, []string{}
			if from == "naming it from elsewhere" {
				where, scope = tree, []string{"--workbench", workbench}
			}

			swept := runCLI(t, where, append(scope, "check", "--migrate-vocabulary")...)
			// What the run printed is read before its exit code, so that a
			// red run names the sentence that reached the operator rather
			// than stopping at the number beside it.
			if strings.Contains(swept.errw, contract.NeedsVocabularyMigration) {
				t.Errorf("the sweep refused the workbench it came to repair:\n%s", swept.errw)
			}
			assertNoUnqualifiedSweepAdvice(t, swept.out+swept.errw)
			if swept.code != 5 {
				t.Fatalf("the climbing vocabulary sweep exited %d rather than reaching the preview: %s%s", swept.code, swept.out, swept.errw)
			}
			if !strings.Contains(swept.out, workbench) {
				t.Errorf("the preview does not name the workbench it would carry forward:\n%s", swept.out)
			}
		})
	}
}

// unqualifiedSweepAdvice names the two fragments that tell a reader to run a
// sweep and leave him to work out for himself which workbench to run it on.
// Each is the last member of its refusal's alternation, and each is
// dispositioned in sweepAdviceNeedingNoScope as text no invocation renders.
var unqualifiedSweepAdvice = []string{
	"refusal.dinah.needs-vocabulary-migration.next",
	"refusal.dinah.vocabulary-mixed.next",
}

// assertNoUnqualifiedSweepAdvice asserts that a run printed neither of those
// two fragments.
//
// The sentence is rendered from the catalog rather than written out here, so
// that rewording the English cannot quietly retire the assertion.
func assertNoUnqualifiedSweepAdvice(t *testing.T, printed string) {
	t.Helper()
	for _, key := range unqualifiedSweepAdvice {
		advice := msg.For(msg.Base).T(key)
		if strings.Contains(printed, advice) {
			t.Errorf("the run printed %s, which tells its reader to run a sweep without saying where:\n%s", key, printed)
		}
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
			adviceLeavesTheConfirmationToTheReader(t, argv)
			// The hand edit is the repair here, so the sweep the sentence
			// names finds nothing left to carry and says so. That is why this
			// half asserts a clean run rather than a preview: the reader is
			// told what the sweep would do, and in this shape it would do
			// nothing.
			carried := runCLI(t, where, argv...)
			if carried.code != 0 {
				t.Fatalf("taking the refusal's own advice, dinah %v, exited %d: %s%s", argv, carried.code, carried.out, carried.errw)
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

// adviceLeavesTheConfirmationToTheReader asserts that a piece of advice does
// not carry --yes, so that the first thing the reader is told to type is the
// preview rather than the irreversible rewrite.
//
// The rewrite waits for --yes because it cannot be undone, and the preview it
// prints ends by naming the second step. Advice that skips that step routes
// the reader past the one thing that would have told him what the command was
// about to do, which is the complaint dinah-362 was filed over. The container
// half of this family has read the preview since round one, in
// TestTheBareWorkbenchAdviceIsACommandThatWorks, and this is the assertion
// that half already makes, named so the vocabulary halves can make it too.
func adviceLeavesTheConfirmationToTheReader(t *testing.T, argv []string) {
	t.Helper()
	for _, flag := range argv {
		if flag == "--yes" {
			t.Fatalf("the advice dinah %v names --yes, so it walks the reader past the preview the rewrite deliberately provides", argv)
		}
	}
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
// Three shapes land here. One is a sentence describing what a command does
// rather than asking anybody to type it. The second is a sentence that does
// ask, and reaches its reader only inside a run he has already scoped himself,
// so the invocation he repeats is his own and carries whatever scope he gave
// it. The third is text no invocation renders.
//
// The two unqualified `.next` fragments are that third shape, and the reason
// recorded for them here was wrong for a round. It said they render on a sweep
// over a tree. A sweep prints a report row per workbench and never composes a
// refusal, so no sweep prints either sentence, and a reviewer replaced both
// English texts with nonsense and watched this package stay green. What is
// true is narrower. Each of the two shapes ends in an alternation whose last
// member carries no condition, because the last member of an alternation
// cannot carry one and still be a fallback, and that member is the fragment
// dispositioned here. Nothing reaches it: the refusals it belongs to are
// composed by session.reportError, which calls nameTheWorkbench first, and
// session.open is the only writer of the workbenchRoot that function reads.
//
// What would make one render is worth naming, because it is the situation
// these dispositions exist for. A raise site that composed one of these two
// refusals without an open would leave workbenchRoot empty and print the
// unqualified sentence, which is the unscoped advice dinah-362 was blocked
// over. sweepRoot is the nearest such path in the tree today: it resolves a
// workbench through bench.DiscoverSource and records workbenchSource while
// leaving workbenchRoot alone, and putting the open back there is four lines
// that compile, pass vet, and print the round-one blocker again.
// TestTheClimbingSweepRepairsRatherThanRefuses runs that path and fails on
// exactly that change.
//
// Two things guard this and neither guards all of it, so the division is
// written down rather than left to be inferred. The behavioural test above
// covers the two sweeps, which is where the hazard lives today.
// TestWorkbenchRootHasOneWriter covers a different proposition, that a
// refusal's named workbench is always the one the open just resolved, and it
// says nothing about whether some path reaches a refusal without opening at
// all. What neither reaches is a raise site elsewhere in the tree composing
// one of these two refusals without an open. That rests on reading the raise
// sites: every one of them today is behind bench.Open, which session.open
// calls, and a card adding another is a card that has to come back to this
// map.
var sweepAdviceNeedingNoScope = map[string]string{
	"check.bare-workbench":                          "a finding row printed by a sweep the caller has already scoped, describing what the pending repair does rather than naming a fresh invocation",
	"refusal.dinah.vocabulary-retired.next":         "the instruction is the hand edit, and the command is named only to say which rename to perform by hand; running it would do nothing, because the workbench this refusal fires on already declares the current vocabulary and the sweep skips it",
	"refusal.dinah.needs-vocabulary-migration.next": "the alternation's unconditional last member, which no invocation renders today, for the reasons written above this map",
	"refusal.dinah.vocabulary-mixed.next":           "the alternation's unconditional last member, unrendered on the same evidence",
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
	declared := testBodiesDeclaredHere(t)
	for key, named := range sweepAdviceProvenByRunning {
		if !seen[key] {
			t.Errorf("%s is dispositioned as proven by running and no catalog message by that key names a sweep, so the disposition outlived its message", key)
		}
		// The name of a test is the whole of this disposition's claim, so a
		// name nothing declares is the same defect the MCP head's argument
		// exemptions carried until this card: paperwork reading as an argued
		// decision and standing for nothing. Renaming or deleting the test
		// that follows a sentence has to cost somebody a line here.
		body, ok := declared[named]
		if !ok {
			t.Errorf("%s is dispositioned as proven by %s and this package declares no test by that name", key, named)
			continue
		}
		// Declaring the name is not enough, and a reviewer proved it by
		// pointing a disposition at this guard itself and watching it pass.
		// A test that follows a sentence has to render that sentence to cut
		// the command out of it, so it names the catalog key as a literal,
		// and requiring the literal is what separates the test that follows
		// this sentence from a test that follows another one.
		if !strings.Contains(body, strconv.Quote(key)) {
			t.Errorf("%s is dispositioned as proven by %s and that test's body never names %s, so it follows some other sentence or none", key, named, key)
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

// testBodiesDeclaredHere answers every test function this package declares,
// each against its own source text, read out of the sources rather than out of
// a list, so the answer cannot go stale on its own.
//
// A body runs from its declaration to the next declaration at column zero,
// which is what gofmt guarantees of a Go source and what makes this reading
// safe without a parser.
func testBodiesDeclaredHere(t *testing.T) map[string]string {
	t.Helper()
	sources, err := filepath.Glob("*_test.go")
	if err != nil {
		t.Fatalf("glob the test sources: %v", err)
	}
	if len(sources) == 0 {
		t.Fatal("no test source was found, so this read proves nothing")
	}
	declared := map[string]string{}
	for _, source := range sources {
		text, err := os.ReadFile(source)
		if err != nil {
			t.Fatalf("read %s: %v", source, err)
		}
		name := ""
		body := &strings.Builder{}
		keep := func() {
			if name != "" {
				declared[name] = body.String()
			}
		}
		for _, line := range strings.Split(string(text), "\n") {
			if strings.HasPrefix(line, "func ") {
				keep()
				name, body = "", &strings.Builder{}
				if declaration := strings.TrimPrefix(line, "func "); strings.HasPrefix(declaration, "Test") {
					if cut := strings.Index(declaration, "("); cut > 0 {
						name = declaration[:cut]
					}
				}
			}
			body.WriteString(line)
			body.WriteString("\n")
		}
		keep()
	}
	return declared
}

// TestWorkbenchRootHasOneWriter holds that session.open is the only thing
// that writes workbenchRoot, so a refusal that names a workbench names the one
// the open just resolved rather than one some earlier path left behind.
//
// That is the whole of what this guard holds, and the distinction cost a
// review round. A second writer sets the field, and a field that is set makes
// nameTheWorkbench attach a workbench, so a second writer can only ever make
// the unqualified fragment render less often. What makes it render is a
// refusal composed on a path that never opened, which is the opposite
// direction and which TestTheClimbingSweepRepairsRatherThanRefuses exercises.
// Both propositions are worth holding; neither substitutes for the other.
//
// The sources are parsed rather than the behaviour exercised, because a second
// writer is a write no current call path reaches, and a behavioural test would
// go green on the very code that introduced it. Parsing rather than matching
// text is the round-four repair: the scan this replaces read a line for the
// field name immediately left of an assignment, so
// `s.workbenchRoot, s.workbenchSource = root, source` passed it while the same
// two operands in the other order failed, and += escaped for the same reason.
// An assignment is a shape in the syntax tree, so the syntax tree is what
// answers the question.
func TestWorkbenchRootHasOneWriter(t *testing.T) {
	sources, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("glob the package sources: %v", err)
	}
	writers := map[string]int{}
	read := 0
	for _, source := range sources {
		if strings.HasSuffix(source, "_test.go") {
			continue
		}
		parsed, err := parser.ParseFile(token.NewFileSet(), source, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", source, err)
		}
		read++
		for _, decl := range parsed.Decls {
			where := source + " (file scope)"
			if function, ok := decl.(*ast.FuncDecl); ok {
				where = source + " " + declaredName(function)
			}
			ast.Inspect(decl, func(node ast.Node) bool {
				switch found := node.(type) {
				case *ast.AssignStmt:
					for _, target := range found.Lhs {
						if namesWorkbenchRoot(target) {
							writers[where]++
						}
					}
				case *ast.KeyValueExpr:
					if key, ok := found.Key.(*ast.Ident); ok && key.Name == "workbenchRoot" {
						writers[where]++
					}
				case *ast.UnaryExpr:
					// Handing the field's address to something else is a
					// write this file cannot follow, so it counts as one.
					if found.Op == token.AND && namesWorkbenchRoot(found.X) {
						writers[where]++
					}
				}
				return true
			})
		}
	}
	if read == 0 {
		t.Fatal("no package source was parsed, so this read proves nothing")
	}
	if len(writers) != 1 {
		t.Fatalf("workbenchRoot is written in %d places and this guard rests on there being one: %v", len(writers), writers)
	}
	for where := range writers {
		if where != "main.go (*session).open" {
			t.Errorf("workbenchRoot's one writer is %s rather than session.open in main.go, so the dispositions in this file need rereading", where)
		}
	}
}

// namesWorkbenchRoot reports whether an expression is a selection of the
// workbenchRoot field, which is the form every write to it takes.
func namesWorkbenchRoot(expr ast.Expr) bool {
	selected, ok := expr.(*ast.SelectorExpr)
	return ok && selected.Sel.Name == "workbenchRoot"
}

// declaredName answers a function declaration's name, carrying its receiver
// type where it has one, so that the place a write happens reads the way a
// person would say it.
func declaredName(function *ast.FuncDecl) string {
	name := function.Name.Name
	if function.Recv == nil || len(function.Recv.List) == 0 {
		return name
	}
	receiver := function.Recv.List[0].Type
	if pointer, ok := receiver.(*ast.StarExpr); ok {
		if to, ok := pointer.X.(*ast.Ident); ok {
			return "(*" + to.Name + ")." + name
		}
	}
	if to, ok := receiver.(*ast.Ident); ok {
		return "(" + to.Name + ")." + name
	}
	return name
}
