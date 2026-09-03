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

// strayContainerBench builds a workbench through the tool and renames its
// directory inside the container to a name a person typed, which is the third
// shape the container sweep meets and the one an identifier-width filter never
// sees. It answers the directory the workbench now sits in.
func strayContainerBench(t *testing.T, project string) string {
	t.Helper()
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if got := runCLI(t, project, "init", project, "--slug", "st", "--operator", "alka"); got.code != 0 {
		t.Fatalf("init at %s: %d %s", project, got.code, got.errw)
	}
	written := benchDir(t, project)
	stray := filepath.Join(project, bench.UserBaseName, "the-one-i-named-myself")
	if err := os.Rename(written, stray); err != nil {
		t.Fatalf("rename to a hand-typed name: %v", err)
	}
	return stray
}

// sweepScopes are the two repairs whose scope this card reconciled. Every case
// below runs against both, because a rule that held for one of them and not
// the other is the defect dinah-362 was filed over.
var sweepScopes = []string{"--migrate-container", "--migrate-vocabulary"}

// TestASweepWithNoRootClimbsLikeEveryOtherCheck asserts dinah-362 AC-2. An
// operator standing above his workbenches and inside none of them used to get
// two different answers from one command: a bare check refused, and the same
// check with a sweep flag walked downward over everything beneath him. Both
// halves now climb, so both halves refuse, and the refusal is the same one.
func TestASweepWithNoRootClimbsLikeEveryOtherCheck(t *testing.T) {
	tree := resolvedDir(t, emptyTree(t))
	legacyContainerBench(t, filepath.Join(tree, "project", bench.UserBaseName))

	bare := runCLI(t, tree, "check")
	if !strings.Contains(bare.errw, contract.NoWorkbenchFound) {
		t.Fatalf("a bare check above the workbenches does not refuse %s: %d %s%s", contract.NoWorkbenchFound, bare.code, bare.out, bare.errw)
	}
	for _, flag := range sweepScopes {
		got := runCLI(t, tree, "check", flag, "--yes")
		if got.code != bare.code {
			t.Errorf("check %s exited %d where a bare check exited %d, and the two answer the same question", flag, got.code, bare.code)
		}
		if !strings.Contains(got.errw, contract.NoWorkbenchFound) {
			t.Errorf("check %s above the workbenches does not refuse %s:\n%s%s", flag, contract.NoWorkbenchFound, got.out, got.errw)
		}
	}
	// Nothing was swept while the sweeps were refusing, which is the half of
	// the claim a refusal message cannot make on its own.
	if ids := bench.ListWorkbenchIDs(filepath.Join(tree, "project", bench.UserBaseName)); len(ids) != 1 || bench.IsWorkbenchID(ids[0]) {
		t.Errorf("a refused sweep still moved something: the container holds %v", ids)
	}
}

// TestNamingOneWorkbenchSweepsThatWorkbenchAlone asserts dinah-362 AC-4.
// --workbench used to name a directory to walk downward from under these two
// flags and one workbench to act on everywhere else, so an operator who had
// learned the flag anywhere else had learned something false here, and the
// failure widened the blast radius rather than refusing.
func TestNamingOneWorkbenchSweepsThatWorkbenchAlone(t *testing.T) {
	tree := resolvedDir(t, emptyTree(t))
	named := legacyContainerBench(t, filepath.Join(tree, "named", bench.UserBaseName))
	sibling := legacyContainerBench(t, filepath.Join(tree, "sibling", bench.UserBaseName))

	got := runCLI(t, tree, "--workbench", named, "check", "--migrate-container", "--yes")
	if got.code != 0 {
		t.Fatalf("a named workbench exited %d: %s%s", got.code, got.out, got.errw)
	}
	if bench.Exists(named) {
		t.Error("the workbench that was named was left under its legacy name")
	}
	if !bench.Exists(sibling) {
		t.Error("naming one workbench swept a sibling the caller never named")
	}

	// The same tree, asked the other way, reaches both.
	swept := runCLI(t, tree, "check", "--root", ".", "--migrate-container", "--yes")
	if swept.code != 0 {
		t.Fatalf("the root-scoped sweep exited %d: %s%s", swept.code, swept.out, swept.errw)
	}
	if bench.Exists(sibling) {
		t.Error("--root . left the sibling under its legacy name")
	}
}

// TestNamingAWorkbenchAndARootAtOnceIsRefused asserts the third of dinah-362
// AC-4's three cases, for both sweeps. The refusal is the one the five
// root-scoped read verbs already give, raised by the helper they share, so no
// new refusal is minted here.
func TestNamingAWorkbenchAndARootAtOnceIsRefused(t *testing.T) {
	tree := resolvedDir(t, emptyTree(t))
	named := legacyContainerBench(t, filepath.Join(tree, "named", bench.UserBaseName))
	for _, flag := range sweepScopes {
		got := runCLI(t, tree, "--workbench", named, "check", "--root", ".", flag)
		if !strings.Contains(got.errw, contract.ConflictingScope) {
			t.Errorf("check %s with both scopes does not refuse %s: %d %s%s", flag, contract.ConflictingScope, got.code, got.out, got.errw)
		}
	}
}

// TestTheSweepsValidateADepthTheWayEveryRootScopedVerbDoes asserts dinah-362
// AC-5. The depth bounds nothing inside either sweep, which is a cut this card
// took deliberately, and the two refusals it can raise still have to read the
// same on check as they do on status, because a caller learns them once.
func TestTheSweepsValidateADepthTheWayEveryRootScopedVerbDoes(t *testing.T) {
	tree := resolvedDir(t, emptyTree(t))
	legacyContainerBench(t, filepath.Join(tree, "project", bench.UserBaseName))
	for _, flag := range sweepScopes {
		lonely := runCLI(t, tree, "check", "--max-depth", "3", flag)
		if !strings.Contains(lonely.errw, contract.DepthWithoutRoot) {
			t.Errorf("check %s --max-depth with no root does not refuse %s: %d %s%s", flag, contract.DepthWithoutRoot, lonely.code, lonely.out, lonely.errw)
		}
		for _, depth := range []string{"three", "-1"} {
			got := runCLI(t, tree, "check", "--root", ".", "--max-depth", depth, flag)
			if !strings.Contains(got.errw, contract.MalformedDepth) {
				t.Errorf("check %s --max-depth %s does not refuse %s: %d %s%s", flag, depth, contract.MalformedDepth, got.code, got.out, got.errw)
			}
		}
		if got := runCLI(t, tree, "check", "--root", ".", "--max-depth", "4", flag, "--yes"); got.code != 0 {
			t.Errorf("check %s with a well-formed depth exited %d: %s%s", flag, got.code, got.out, got.errw)
		}
	}
}

// TestAScopeFlagBesideAFormThatCannotWalkIsRefused asserts dinah-362 AC-10 and
// AC-11. Neither scope flag has a downward walk to aim or to bound outside the
// two sweeps, so accepting one beside anything else would drop a word the
// caller typed, which is the complaint this card was filed over.
func TestAScopeFlagBesideAFormThatCannotWalkIsRefused(t *testing.T) {
	root := newBench(t)
	elsewhere := t.TempDir()
	forms := [][]string{
		{},
		{"--finish"},
		{"--migrate-ordinals"},
		{"--migrate-slugs"},
		{"--migrate-columns"},
		{"--migrate-workstreams"},
		{"--witness"},
		{"--remint", elsewhere},
	}
	scopes := [][]string{{"--root", "."}, {"--max-depth", "3"}, {"--root", ".", "--max-depth", "3"}}
	for _, form := range forms {
		for _, scope := range scopes {
			argv := append(append([]string{"check"}, scope...), form...)
			got := runCLI(t, root, argv...)
			if !strings.Contains(got.errw, contract.Usage) {
				t.Errorf("check %v does not refuse %s: %d %s%s", argv[1:], contract.Usage, got.code, got.out, got.errw)
			}
		}
		// The same form without a scope flag is untouched by this card, so a
		// refusal here would mean the guard had swallowed the whole command
		// rather than the combination.
		argv := append([]string{"check"}, form...)
		if got := runCLI(t, root, argv...); strings.Contains(got.errw, contract.Usage) {
			t.Errorf("check %v refuses %s with no scope flag given:\n%s", form, contract.Usage, got.errw)
		}
	}
}

// TestTheScopeRefusalNamesWhatTheCallerTyped asserts the detail half of
// dinah-362 AC-11: the refusal carries the value the caller gave, so a reader
// is told which word was not understood rather than being handed the sentence
// alone. --root is named ahead of --max-depth when both were given.
func TestTheScopeRefusalNamesWhatTheCallerTyped(t *testing.T) {
	root := newBench(t)
	for _, c := range []struct {
		argv   []string
		detail string
	}{
		{[]string{"check", "--root", "somewhere", "--finish"}, "somewhere"},
		{[]string{"check", "--max-depth", "77", "--witness"}, "77"},
		{[]string{"check", "--root", "somewhere", "--max-depth", "77"}, "somewhere"},
	} {
		got := runCLI(t, root, c.argv...)
		if !strings.Contains(got.errw, c.detail) {
			t.Errorf("check %v does not name %q in its refusal:\n%s", c.argv[1:], c.detail, got.errw)
		}
	}
}

// TestAModalFlagBesideAFlagItWouldStarveIsRefused asserts dinah-362 AC-13, the
// operator's ruling on D-6. check dispatches on the first modal flag it finds
// and returns, so every other repair the caller asked for used to be dropped
// with no message. That is the same complaint the card was filed over, one
// flag combination sideways, and the operator chose the refusal over the
// silence knowing it breaks any script relying on the drop.
func TestAModalFlagBesideAFlagItWouldStarveIsRefused(t *testing.T) {
	root := newBench(t)
	elsewhere := t.TempDir()
	for _, c := range []struct {
		argv   []string
		detail string
	}{
		{[]string{"check", "--migrate-container", "--finish"}, "--migrate-container --finish"},
		{[]string{"check", "--migrate-vocabulary", "--witness"}, "--migrate-vocabulary --witness"},
		{[]string{"check", "--remint", elsewhere, "--migrate-slugs"}, "--remint --migrate-slugs"},
		{[]string{"check", "--migrate-vocabulary", "--migrate-container"}, "--migrate-vocabulary --migrate-container"},
		{[]string{"check", "--migrate-container", "--remint", elsewhere}, "--migrate-container --remint"},
		{[]string{"check", "--witness", "--remint", elsewhere, "--migrate-vocabulary"}, "--migrate-vocabulary --remint --witness"},
		{[]string{"check", "--migrate-container", "--migrate-ordinals", "--migrate-columns"}, "--migrate-container --migrate-ordinals --migrate-columns"},
	} {
		got := runCLI(t, root, c.argv...)
		if !strings.Contains(got.errw, contract.Usage) {
			t.Errorf("check %v does not refuse %s: %d %s%s", c.argv[1:], contract.Usage, got.code, got.out, got.errw)
		}
		if !strings.Contains(got.errw, c.detail) {
			t.Errorf("check %v does not name %q as the conflict:\n%s", c.argv[1:], c.detail, got.errw)
		}
		// The value --remint carries is the caller's directory, not a flag, so
		// it stays out of a sentence listing flags.
		if strings.Contains(got.errw, elsewhere) {
			t.Errorf("check %v prints --remint's directory in a list of conflicting flags:\n%s", c.argv[1:], got.errw)
		}
	}
}

// TestTheCombinationsThatStillRunStillRun is the other half of dinah-362
// AC-13, and it is the half that keeps the refusal above from being one that
// refuses everything. Each of these was legal before this card and stays legal
// after it.
func TestTheCombinationsThatStillRunStillRun(t *testing.T) {
	root := newBench(t)
	for _, argv := range [][]string{
		{"check"},
		{"check", "--finish", "--migrate-ordinals", "--witness"},
		{"check", "--migrate-slugs", "--migrate-columns", "--migrate-workstreams"},
		{"check", "--migrate-container"},
		{"check", "--migrate-container", "--yes"},
		{"check", "--migrate-vocabulary", "--yes"},
		{"check", "--finish", "--yes"},
		{"check", "--root", ".", "--migrate-container", "--yes"},
		{"check", "--root", ".", "--max-depth", "2", "--migrate-vocabulary", "--yes"},
	} {
		if got := runCLI(t, root, argv...); got.code != 0 {
			t.Errorf("check %v exited %d, and this card refuses none of it: %s%s", argv[1:], got.code, got.out, got.errw)
		}
	}
}

// TestAnAppliedSweepNamesTheShapesItMoved asserts dinah-362 AC-7. The applied
// run used to print one sentence over every workbench it touched, saying each
// had been carried into a container. That is what happens to a bare workbench
// and not to the other two shapes, so the operator whose thirteen workbenches
// were all renamed in place was told something untrue about his own data.
func TestAnAppliedSweepNamesTheShapesItMoved(t *testing.T) {
	tree := resolvedDir(t, emptyTree(t))
	bare := bareWorkbench(t, filepath.Join(tree, "bareproject"))
	legacy := legacyContainerBench(t, filepath.Join(tree, "legacyproject", bench.UserBaseName))
	stray := strayContainerBench(t, filepath.Join(tree, "strayproject"))

	applied := runCLI(t, tree, "check", "--root", ".", "--migrate-container", "--yes")
	if applied.code != 0 {
		t.Fatalf("the sweep exited %d: %s%s", applied.code, applied.out, applied.errw)
	}
	catalog := msg.For(msg.Base)
	carried := catalog.TN("check.container-migrated-bare", 1)
	renamed := catalog.TN("check.container-migrated-legacy", 2)
	if !strings.Contains(applied.out, carried) {
		t.Errorf("the applied run does not carry the heading %q:\n%s", carried, applied.out)
	}
	if !strings.Contains(applied.out, renamed) {
		t.Errorf("the applied run does not carry the heading %q:\n%s", renamed, applied.out)
	}
	// One workbench was carried and two were renamed, so a run reporting
	// three of either has grouped a shape into the wrong bucket.
	if strings.Contains(applied.out, catalog.TN("check.container-migrated-bare", 3)) {
		t.Errorf("the applied run counts every workbench as bare:\n%s", applied.out)
	}
	if strings.Contains(applied.out, catalog.TN("check.container-migrated-legacy", 3)) {
		t.Errorf("the applied run counts every workbench as already contained:\n%s", applied.out)
	}
	for _, from := range []string{bare, legacy, stray} {
		if !strings.Contains(applied.out, from) {
			t.Errorf("the applied run does not name %s among the rows of either heading:\n%s", from, applied.out)
		}
	}
}

// TestTheOldBlanketMigrationHeadingIsGone asserts the removal half of
// dinah-362 AC-7. A key left in the catalogs would be a sentence nothing
// prints, and a key left in the renderer would be the old blanket line
// printing beside the two that replaced it.
func TestTheOldBlanketMigrationHeadingIsGone(t *testing.T) {
	for _, tag := range msg.Tags() {
		for _, key := range []string{"check.container-migrated", "check.container-migrated.one", "check.container-migrated.other"} {
			if _, carried := msg.CatalogEntry(tag, key); carried {
				t.Errorf("%s still carries %s, which nothing prints any more", tag, key)
			}
		}
	}
}

// TestTheBareWorkbenchAdviceIsACommandThatWorks asserts the half of dinah-362
// AC-6 the spec left out, and it is the half that had actually broken. Narrowing
// the sweeps to a climb took the bare shape out of reach of the command this
// refusal recommends, because a bare workbench is exactly what discovery no
// longer returns, so the tool answered a refusal with advice that produced the
// same refusal. The test runs the advice rather than reading it, which is the
// only way to tell a sentence naming a command from a sentence naming one that
// does the job.
func TestTheBareWorkbenchAdviceIsACommandThatWorks(t *testing.T) {
	tree := resolvedDir(t, emptyTree(t))
	project := bareWorkbench(t, filepath.Join(tree, "myproject"))

	refused := runCLI(t, project, "check", "--migrate-container", "--yes")
	if !strings.Contains(refused.errw, contract.NoWorkbenchFound) {
		t.Fatalf("standing in a bare workbench does not refuse %s: %d %s%s", contract.NoWorkbenchFound, refused.code, refused.out, refused.errw)
	}
	advice := msg.For(msg.Base).T("refusal.dinah.no-workbench-found.bare", "bare", project)
	if !strings.Contains(refused.errw, "--migrate-container") {
		t.Fatalf("the refusal does not recommend the container repair at all:\n%s", refused.errw)
	}
	argv, found := adviceInvocation(advice)
	if !found {
		t.Fatalf("no `dinah ...` invocation was found in the advice %q, so this test would assert nothing", advice)
	}
	// The advice reaches the repair's own preview rather than the repair, which
	// is the findings code 5 and is what this command does with anything it is
	// about to move. Following it through to the move is the whole claim, so the
	// confirmation is given here rather than the preview taken as the answer.
	took := runCLI(t, project, argv...)
	if took.code != 5 {
		t.Fatalf("taking the refusal's own advice, dinah %v, exited %d rather than reaching the preview: %s%s", argv, took.code, took.out, took.errw)
	}
	if !strings.Contains(took.out, project) {
		t.Errorf("the preview the advice reached does not name the workbench it would repair:\n%s", took.out)
	}
	confirmed := runCLI(t, project, append(argv, "--yes")...)
	if confirmed.code != 0 {
		t.Fatalf("the advice, confirmed, exited %d: %s%s", confirmed.code, confirmed.out, confirmed.errw)
	}
	ids := bench.ListWorkbenchIDs(filepath.Join(project, bench.UserBaseName))
	if len(ids) != 1 {
		t.Errorf("the advice ran and the workbench is not in a container: the container holds %v", ids)
	}
	// The workbench answers as itself from where it now sits, which is what
	// the operator wanted when he followed the advice.
	if got := runCLI(t, project, "status"); got.code != 0 {
		t.Errorf("the repaired workbench does not open: %d %s", got.code, got.errw)
	}
}

// adviceInvocation cuts the first backticked `dinah ...` command out of a
// catalog sentence and answers it as an argument list with the program name
// dropped, which is the form runCLI takes.
func adviceInvocation(sentence string) ([]string, bool) {
	opened := strings.Index(sentence, "`dinah ")
	if opened < 0 {
		return nil, false
	}
	rest := sentence[opened+1:]
	closed := strings.Index(rest, "`")
	if closed < 0 {
		return nil, false
	}
	words := strings.Fields(rest[:closed])
	if len(words) < 2 {
		return nil, false
	}
	return words[1:], true
}

// TestTheDamagedWorkbenchAdviceIsACommandThatWorks follows the next step
// dinah.damaged-workbench prints, the whole way, on the tree it prints over.
//
// The sentence offers a reader two things and this test takes both. Passing
// --workbench at the directory the refusal named gets past discovery, which
// runs no recognition test on an explicit pointer, and reaches the per-field
// refusal that says which part of the file is wrong. Restoring the anchor and
// running the invocation the sentence names then answers cleanly, which is
// what the reader was promised when he was told to confirm.
func TestTheDamagedWorkbenchAdviceIsACommandThatWorks(t *testing.T) {
	tree := resolvedDir(t, emptyTree(t))
	project := filepath.Join(tree, "myproject")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if got := runCLI(t, project, "init", project, "--slug", "fx", "--operator", "alka"); got.code != 0 {
		t.Fatalf("init: %d %s", got.code, got.errw)
	}
	workbench := benchDir(t, project)
	anchor := filepath.Join(workbench, bench.WorkbenchAnchor)
	backup, err := os.ReadFile(anchor)
	if err != nil {
		t.Fatalf("read the anchor this test restores from: %v", err)
	}
	// A hand edit that took the three keys recognition reads and left the
	// fence standing, which is the shape nobody had produced before this card.
	if err := os.WriteFile(anchor, []byte("---\ntitle: Fixture\nslug: fx\n---\nStanding text.\n"), 0o644); err != nil {
		t.Fatalf("damage the anchor: %v", err)
	}

	refused := runCLI(t, project, "status")
	if !strings.Contains(refused.errw, contract.DamagedBench) {
		t.Fatalf("standing in a damaged workbench does not refuse %s: %d %s%s", contract.DamagedBench, refused.code, refused.out, refused.errw)
	}
	advice := msg.For(msg.Base).T("refusal.dinah.damaged-workbench.next", "detail", workbench)

	// The first half of the advice: the pointer reaches the file, and what
	// comes back names the field rather than repeating that something is
	// damaged. That is the whole reason the refusal names a directory and not
	// the workbench.md inside it.
	if !strings.Contains(advice, "--workbench") {
		t.Fatalf("the advice no longer offers the pointer this half follows: %q", advice)
	}
	pointed := runCLI(t, project, "--workbench", workbench, "status")
	if pointed.code == 0 {
		t.Fatalf("an explicit pointer at a damaged workbench opened it: %s", pointed.out)
	}
	if strings.Contains(pointed.errw, contract.DamagedBench) || strings.Contains(pointed.errw, contract.NoWorkbenchFound) {
		t.Errorf("the pointer got no further than discovery did, so it says nothing new:\n%s", pointed.errw)
	}
	if !strings.Contains(pointed.errw, contract.Malformed) {
		t.Errorf("the pointer does not reach the per-field refusal the advice promises:\n%s", pointed.errw)
	}
	// The name alone is a substring of dinah.malformed-depth, so the field
	// this fixture's damage actually removed is named beside it. A refusal
	// from anywhere else in the malformed family satisfies the check above
	// and cannot satisfy this one.
	if !strings.Contains(pointed.errw, "profile") {
		t.Errorf("the refusal does not name the field the damage removed:\n%s", pointed.errw)
	}

	// The second half: restore the file the way the sentence says, then run
	// the invocation it names and read what it answers.
	argv, found := adviceInvocation(advice)
	if !found {
		t.Fatalf("no `dinah ...` invocation was found in the advice %q, so this test would assert nothing", advice)
	}
	if err := os.WriteFile(anchor, backup, 0o644); err != nil {
		t.Fatalf("restore the anchor: %v", err)
	}
	took := runCLI(t, project, argv...)
	if took.code != 0 {
		t.Fatalf("taking the refusal's own advice, dinah %v, exited %d: %s%s", argv, took.code, took.out, took.errw)
	}
}
