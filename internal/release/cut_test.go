package release

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fixture builds a small repository that stands in for the trunk: a VERSION
// file holding the line under test, three carded commits each carrying a dev
// tag, and one uncarded infrastructure commit with no tag.
//
// The tags deliberately mix the two shapes, because the trunk this stands in
// for does too and will go on doing so.
type fixture struct {
	git   Git
	root  string
	sha   map[string]string
	start string
}

func run(t *testing.T, g Git, args ...string) string {
	t.Helper()
	out, err := g.Run(args...)
	if err != nil {
		t.Fatalf("fixture setup: %v", err)
	}
	return out
}

func write(t *testing.T, dir, name, body string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("fixture setup: %v", err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("fixture setup: %v", err)
	}
}

func commit(t *testing.T, f *fixture, message string, files map[string]string) string {
	t.Helper()
	for name, body := range files {
		write(t, f.root, name, body)
		run(t, f.git, "add", name)
	}
	run(t, f.git, "commit", "-m", message)
	return run(t, f.git, "rev-parse", "HEAD")
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	root := t.TempDir()
	g := Git{Dir: root}
	f := &fixture{git: g, root: root, sha: map[string]string{}}

	run(t, g, "init", "--initial-branch=main")
	run(t, g, "config", "user.email", "fixture@example.invalid")
	run(t, g, "config", "user.name", "Fixture")
	// A repository-local commit template or signing configuration on the
	// machine running the tests would otherwise reach these commits.
	run(t, g, "config", "commit.gpgsign", "false")

	f.sha["root"] = commit(t, f, "start the 0.1 line", map[string]string{
		"VERSION":          "0.1\n",
		"internal/base.go": "package base\n\nconst A = 1\n",
	})
	f.start = f.sha["root"]

	f.sha["one"] = commit(t, f, "dinah-1: add the first thing", map[string]string{
		"internal/one.go": "package one\n",
	})
	run(t, g, "tag", "v0.1.0-dev.1")

	f.sha["two"] = commit(t, f, "dinah-2: change the shared constant", map[string]string{
		"internal/base.go": "package base\n\nconst A = 2\n",
	})
	run(t, g, "tag", "v0.1.2-dev")

	f.sha["three"] = commit(t, f, "dinah-3: change the shared constant again", map[string]string{
		"internal/base.go": "package base\n\nconst A = 3\n",
	})
	run(t, g, "tag", "v0.1.3-dev")

	f.sha["infra"] = commit(t, f, "keep the workspace pointed at the repository folder", map[string]string{
		"tooling.txt": "not a card, not tagged\n",
	})

	return f
}

func TestCandidatesReadCardsTagsAndSkipUntaggedCommits(t *testing.T) {
	f := newFixture(t)
	candidates, err := Candidates(f.git, f.start, "main", "0.1")
	if err != nil {
		t.Fatalf("Candidates: %v", err)
	}
	if len(candidates) != 3 {
		t.Fatalf("Candidates found %d commits; want the three tagged ones: %+v", len(candidates), candidates)
	}
	wantCards := []string{"dinah-1", "dinah-2", "dinah-3"}
	wantTags := []string{"v0.1.0-dev.1", "v0.1.2-dev", "v0.1.3-dev"}
	for i, c := range candidates {
		if c.Card != wantCards[i] {
			t.Errorf("candidate %d belongs to %q; want %q", i, c.Card, wantCards[i])
		}
		if c.Tag != wantTags[i] {
			t.Errorf("candidate %d carries tag %q; want %q", i, c.Tag, wantTags[i])
		}
	}
	for _, c := range candidates {
		if c.SHA == f.sha["infra"] {
			t.Error("the untagged infrastructure commit was offered as a cut candidate")
		}
	}
}

func TestMinorStartFindsTheVersionBump(t *testing.T) {
	f := newFixture(t)
	start, err := MinorStart(f.git, "main", "0.1")
	if err != nil {
		t.Fatalf("MinorStart: %v", err)
	}
	if start != f.sha["root"] {
		t.Errorf("the 0.1 line starts at %s; want the root commit %s", start, f.sha["root"])
	}

	bump := commit(t, f, "open the 0.2 line", map[string]string{"VERSION": "0.2\n"})
	start, err = MinorStart(f.git, "main", "0.2")
	if err != nil {
		t.Fatalf("MinorStart after the bump: %v", err)
	}
	if start != bump {
		t.Errorf("the 0.2 line starts at %s; want the bump commit %s", start, bump)
	}
}

func TestCardOfReadsOnlyACardPrefix(t *testing.T) {
	cases := map[string]string{
		"dinah-359: package and install the extension": "dinah-359",
		"build: stay on main":                          "",
		"no colon here at all":                         "",
		"dinah-x: not a number":                        "",
		"-12: no slug":                                 "",
	}
	for subject, want := range cases {
		if got := CardOf(subject); got != want {
			t.Errorf("CardOf(%q) = %q; want %q", subject, got, want)
		}
	}
}

// The pipeline cannot tell a mistyped identifier from a card whose merge has
// not landed, so both refuse identically and the run stops before anything is
// assembled.
func TestSelectRefusesACardThatResolvesToNothing(t *testing.T) {
	f := newFixture(t)
	candidates, err := Candidates(f.git, f.start, "main", "0.1")
	if err != nil {
		t.Fatalf("Candidates: %v", err)
	}
	_, err = Select([]string{"dinah-2", "dinah-999"}, candidates, mustLinks(t, "dinah-2>none,dinah-999>none"), nil)
	if err == nil {
		t.Fatal("a cards input naming an unresolvable card was accepted")
	}
	if !strings.Contains(err.Error(), "dinah-999") {
		t.Errorf("the refusal does not name the unresolvable card: %v", err)
	}
	if strings.Contains(err.Error(), "dinah-2") {
		t.Errorf("the refusal names a card that resolved perfectly well: %v", err)
	}
	// The reviewer's note: a refusal an operator reads mid-cut has to say what
	// to do about it, the way the conflict refusal already does.
	for _, remedy := range []string{"identifier is wrong", "wait for it", "dispatch the cut again"} {
		if !strings.Contains(err.Error(), remedy) {
			t.Errorf("the refusal does not tell the operator to %q: %v", remedy, err)
		}
	}
}

func TestSelectRefusesACutThatHoldsBackAPredecessor(t *testing.T) {
	f := newFixture(t)
	candidates, err := Candidates(f.git, f.start, "main", "0.1")
	if err != nil {
		t.Fatalf("Candidates: %v", err)
	}
	links := mustLinks(t, "dinah-3>dinah-2")

	_, err = Select([]string{"dinah-3"}, candidates, links, nil)
	if err == nil {
		t.Fatal("a cut holding back a predecessor was accepted")
	}
	want := "dinah-3 depends on dinah-2, which is not in this cut and was not in an earlier one"
	if !strings.Contains(err.Error(), want) {
		t.Errorf("the refusal does not name the violated pair; want %q, got: %v", want, err)
	}

	selection, err := Select([]string{"dinah-2", "dinah-3"}, candidates, mustLinks(t, "dinah-3>dinah-2,dinah-2>none"), nil)
	if err != nil {
		t.Fatalf("a cut carrying both the card and its predecessor was refused: %v", err)
	}
	if len(selection.Picks) != 2 || selection.Picks[0].Card != "dinah-2" || selection.Picks[1].Card != "dinah-3" {
		t.Errorf("the picks are not the two cards in the trunk's order: %+v", selection.Picks)
	}
}

// A predecessor an earlier beta already carries satisfies the check, which is
// what makes a second cut of the same minor possible at all.
func TestSelectAcceptsAPredecessorAnEarlierCutCarried(t *testing.T) {
	f := newFixture(t)
	candidates, err := Candidates(f.git, f.start, "main", "0.1")
	if err != nil {
		t.Fatalf("Candidates: %v", err)
	}
	links := mustLinks(t, "dinah-3>dinah-2")
	carried := map[string]bool{f.sha["two"]: true}

	selection, err := Select([]string{"dinah-3"}, candidates, links, carried)
	if err != nil {
		t.Fatalf("a cut whose predecessor is already in the lineage was refused: %v", err)
	}
	if len(selection.Picks) != 1 || selection.Picks[0].Card != "dinah-3" {
		t.Errorf("the picks are not just the one new card: %+v", selection.Picks)
	}
}

func TestSelectSkipsACardTheBaseAlreadyCarries(t *testing.T) {
	f := newFixture(t)
	candidates, err := Candidates(f.git, f.start, "main", "0.1")
	if err != nil {
		t.Fatalf("Candidates: %v", err)
	}
	carried := map[string]bool{f.sha["two"]: true}
	selection, err := Select([]string{"dinah-2", "dinah-3"}, candidates, mustLinks(t, "dinah-2>none,dinah-3>none"), carried)
	if err != nil {
		t.Fatalf("Select: %v", err)
	}
	if len(selection.Picks) != 1 || selection.Picks[0].Card != "dinah-3" {
		t.Errorf("a card the base already carries was picked again: %+v", selection.Picks)
	}
	if len(selection.AlreadyCarried) != 1 || selection.AlreadyCarried[0] != "dinah-2" {
		t.Errorf("the already-carried card was not reported: %+v", selection.AlreadyCarried)
	}
}

// mustLinks parses a dependency declaration a test expects to be well formed.
func mustLinks(t *testing.T, spec string) Dependencies {
	t.Helper()
	deps, err := ParseLinks(spec)
	if err != nil {
		t.Fatalf("ParseLinks(%q): %v", spec, err)
	}
	return deps
}

// The links input takes one answer per card, so the placeholder cannot be
// written once for a whole cut. An earlier shape accepted the bare word "none"
// and switched the dependency check off for any number of cards, which is why
// that spelling is refused by name rather than merely being undocumented.
func TestParseLinksRequiresOneAnswerPerCard(t *testing.T) {
	if _, err := ParseLinks(""); err == nil {
		t.Error("an empty links input was accepted, which would switch the dependency check off silently")
	}
	bare, err := ParseLinks("none")
	if err == nil {
		t.Error("the bare word none was accepted, which switches the dependency check off for a cut of any size")
	} else if !strings.Contains(err.Error(), "once per card") {
		t.Errorf("the refusal does not say what to write instead: %v", err)
	}
	if bare.Answered("dinah-3") {
		t.Error("a refused input still reports a card as answered")
	}

	deps := mustLinks(t, " dinah-3>dinah-2 , dinah-9>dinah-1 ")
	if len(deps.Links) != 2 || deps.Links[0] != (Link{"dinah-3", "dinah-2"}) || deps.Links[1] != (Link{"dinah-9", "dinah-1"}) {
		t.Errorf("ParseLinks read %+v", deps.Links)
	}
	if !deps.Answered("dinah-3") || !deps.Answered("dinah-9") || deps.Answered("dinah-1") {
		t.Errorf("the answered set is wrong: %+v", deps)
	}

	none := mustLinks(t, "dinah-3>none")
	if len(none.Links) != 0 {
		t.Errorf("dinah-3>none produced a link: %+v", none.Links)
	}
	if !none.Answered("dinah-3") {
		t.Error("dinah-3>none did not answer for dinah-3")
	}

	if _, err := ParseLinks("dinah-3"); err == nil {
		t.Error("a links entry with no predecessor was accepted")
	}
	if _, err := ParseLinks("dinah-3>none,dinah-3>dinah-2"); err == nil {
		t.Error("a card declared both unconstrained and dependent was accepted")
	}
}

// The refusal this test guards is the one a placeholder used to walk past. A
// card in the cut that the links input never mentions is an omission, and an
// omission is not a claim that the card has no predecessors.
func TestSelectRefusesACardTheLinksInputSaysNothingAbout(t *testing.T) {
	f := newFixture(t)
	candidates, err := Candidates(f.git, f.start, "main", "0.1")
	if err != nil {
		t.Fatalf("Candidates: %v", err)
	}
	// dinah-2 is declared, dinah-3 is not, and both are in the cut.
	_, err = Select([]string{"dinah-2", "dinah-3"}, candidates, mustLinks(t, "dinah-2>none"), nil)
	if err == nil {
		t.Fatal("a cut naming a card the links input says nothing about was accepted")
	}
	if !strings.Contains(err.Error(), "dinah-3") {
		t.Errorf("the refusal does not name the undeclared card: %v", err)
	}
	if strings.Contains(err.Error(), "dinah-2") && !strings.Contains(err.Error(), "dinah-3>") {
		t.Errorf("the refusal names a card that was declared: %v", err)
	}
	for _, remedy := range []string{"blocks and parked_behind", ">none", "Nothing was assembled"} {
		if !strings.Contains(err.Error(), remedy) {
			t.Errorf("the refusal does not tell the operator about %q: %v", remedy, err)
		}
	}
}

// What the run was told to assume is printed rather than kept to itself, so a
// bad cut can be read back afterwards.
func TestSelectReportsTheCardsItTreatedAsUnconstrained(t *testing.T) {
	f := newFixture(t)
	candidates, err := Candidates(f.git, f.start, "main", "0.1")
	if err != nil {
		t.Fatalf("Candidates: %v", err)
	}
	selection, err := Select([]string{"dinah-2", "dinah-3"}, candidates, mustLinks(t, "dinah-3>dinah-2,dinah-2>none"), nil)
	if err != nil {
		t.Fatalf("Select: %v", err)
	}
	if len(selection.Unconstrained) != 1 || selection.Unconstrained[0] != "dinah-2" {
		t.Errorf("the cards treated as having no predecessor are %+v; want just dinah-2", selection.Unconstrained)
	}
}

func TestCherryPickAssemblesTheCutAndRecordsProvenance(t *testing.T) {
	f := newFixture(t)
	candidates, err := Candidates(f.git, f.start, "main", "0.1")
	if err != nil {
		t.Fatalf("Candidates: %v", err)
	}
	selection, err := Select([]string{"dinah-1", "dinah-2"}, candidates, mustLinks(t, "dinah-1>none,dinah-2>none"), nil)
	if err != nil {
		t.Fatalf("Select: %v", err)
	}
	if err := CherryPick(f.git, f.start, selection.Picks); err != nil {
		t.Fatalf("CherryPick: %v", err)
	}

	carried, err := CarriedSHAs(f.git, "HEAD")
	if err != nil {
		t.Fatalf("CarriedSHAs: %v", err)
	}
	for _, name := range []string{"one", "two"} {
		if !carried[f.sha[name]] {
			t.Errorf("the assembled tree does not record %s (%s) as cherry-picked", name, f.sha[name])
		}
	}
	if carried[f.sha["three"]] {
		t.Error("the assembled tree claims to carry a card that was not in the cut")
	}
}

// A conflict stops the run where it stands. Nothing later happens, and the
// checkout is left with no cherry-pick in progress, so the tree the job
// discards carries no half-applied state.
func TestCherryPickAbortsOnAConflictAndNamesIt(t *testing.T) {
	f := newFixture(t)
	candidates, err := Candidates(f.git, f.start, "main", "0.1")
	if err != nil {
		t.Fatalf("Candidates: %v", err)
	}
	// dinah-3 rewrites the line dinah-2 introduced, so picking it onto the
	// minor's start without dinah-2 cannot apply.
	selection, err := Select([]string{"dinah-3"}, candidates, mustLinks(t, "dinah-3>none"), nil)
	if err != nil {
		t.Fatalf("Select: %v", err)
	}
	err = CherryPick(f.git, f.start, selection.Picks)
	if err == nil {
		t.Fatal("a conflicting cherry-pick was reported as a clean assembly")
	}
	if !strings.Contains(err.Error(), f.sha["three"]) || !strings.Contains(err.Error(), "dinah-3") {
		t.Errorf("the failure names neither the conflicting commit nor its card: %v", err)
	}
	for _, remedy := range []string{"no tag was pushed", "rebasing", "drop dinah-3"} {
		if !strings.Contains(err.Error(), remedy) {
			t.Errorf("the failure does not tell the operator about %q: %v", remedy, err)
		}
	}

	head, runErr := f.git.Run("rev-parse", "HEAD")
	if runErr != nil {
		t.Fatalf("rev-parse after the abort: %v", runErr)
	}
	if head != f.start {
		t.Errorf("the aborted assembly left HEAD at %s rather than back at the base %s", head, f.start)
	}
	if _, statErr := os.Stat(filepath.Join(f.root, ".git", "CHERRY_PICK_HEAD")); statErr == nil {
		t.Error("a cherry-pick is still in progress after the abort")
	}
}
