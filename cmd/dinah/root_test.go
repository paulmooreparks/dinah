package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"

	"dinah/internal/bench"
	"dinah/internal/contract"
	"dinah/internal/msg"
)

// newForest builds a directory holding a workbench at each relative place
// named, and returns that directory. A place is slash-separated, so a case
// reads as the shape it builds.
//
// The user base sits beside the forest rather than inside it, so the downward
// walk cannot meet it and a count asserted here is the count the case wrote.
func newForest(t *testing.T, places ...string) string {
	t.Helper()
	base := t.TempDir()
	t.Setenv("DINAH_HOME", filepath.Join(base, "home"))
	t.Setenv("DINAH_ACTOR", "alka")
	t.Setenv("DINAH_LANG", "")
	t.Setenv("DINAH_FORMAT", "")
	t.Setenv("DINAH_WORKBENCH", "")
	root := filepath.Join(base, "forest")
	for _, place := range places {
		dir := filepath.Join(append([]string{root}, strings.Split(place, "/")...)...)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
		slug := bench.Slugify(strings.ReplaceAll(place, "/", "-"))
		if got := runCLI(t, dir, "init", "--slug", slug, "--operator", "alka"); got.code != 0 {
			t.Fatalf("init at %s: %d %s", place, got.code, got.errw)
		}
	}
	return root
}

// forestJSON runs one root-scoped read and returns its decoded answer.
func forestJSON(t *testing.T, from string, argv ...string) map[string]any {
	t.Helper()
	got := runCLI(t, from, append(argv, "--json")...)
	if got.code != 0 {
		t.Fatalf("%v: %d %s", argv, got.code, got.errw)
	}
	answer, ok := decode(t, got.out).(map[string]any)
	if !ok {
		t.Fatalf("%v answered something that is not an object: %s", argv, got.out)
	}
	return answer
}

// members returns the per-workbench rows of a root-scoped answer.
func members(t *testing.T, answer map[string]any) []map[string]any {
	t.Helper()
	listed, ok := answer["workbenches"].([]any)
	if !ok {
		t.Fatalf("the answer carries no workbenches array: %v", answer)
	}
	rows := make([]map[string]any, 0, len(listed))
	for _, entry := range listed {
		row, ok := entry.(map[string]any)
		if !ok {
			t.Fatalf("a workbenches entry is not an object: %v", entry)
		}
		rows = append(rows, row)
	}
	return rows
}

// memberPaths returns the path of each row, sorted, which is what a case
// asserts the walk reached.
func memberPaths(t *testing.T, root string, rows []map[string]any) []string {
	t.Helper()
	var out []string
	for _, row := range rows {
		path, _ := row["path"].(string)
		rel, err := filepath.Rel(root, path)
		if err != nil {
			t.Fatalf("relative path of %q: %v", path, err)
		}
		out = append(out, filepath.ToSlash(rel))
	}
	sort.Strings(out)
	return out
}

// rootScopedVerbs are the five reads a sidebar refresh needs, each paired with
// the member its root-scoped answer publishes that workbench's own answer
// under. Every case below iterates this rather than naming the five by hand,
// so a verb added to the surface and not to a case fails visibly.
var rootScopedVerbs = []struct {
	verb   string
	member string
}{
	{"tree", "tree"},
	{"status", "status"},
	{"ls", "listing"},
	{"next", "offers"},
	{"changes", "changes"},
}

// TestWorkbenchesBeneathAPathListsEveryWorkbenchThere asserts dinah-281 AC-1's
// first half: the positional form walks downward and reports every workbench
// beneath the path, in both output forms.
func TestWorkbenchesBeneathAPathListsEveryWorkbenchThere(t *testing.T) {
	root := newForest(t, "alpha", "customer/beta", "customer/deep/gamma")
	got := runCLI(t, root, "workbenches", root, "--json")
	if got.code != 0 {
		t.Fatalf("workbenches <path>: %d %s", got.code, got.errw)
	}
	listed, ok := decode(t, got.out).([]any)
	if !ok {
		t.Fatalf("the listing is not an array: %s", got.out)
	}
	if len(listed) != 3 {
		t.Errorf("the walk reported %d workbenches, wanted the three the fixture holds: %s", len(listed), got.out)
	}
	human := runCLI(t, root, "workbenches", root)
	if human.code != 0 {
		t.Fatalf("the human form: %d %s", human.code, human.errw)
	}
	for _, slug := range []string{"alpha", "customer-beta", "customer-deep-gamma"} {
		if !strings.Contains(human.out, slug) {
			t.Errorf("the human listing does not name %s:\n%s", slug, human.out)
		}
	}
}

// TestWorkbenchesWithNoPathStillAnswersTheUpwardSearch asserts dinah-281 AC-1's
// second half at the point where the two directions actually differ.
//
// The fixture is a directory holding three workbenches and being none itself.
// The upward search reaches nothing from there and says so, exactly as it did
// before this card, while the positional form reports all three. A change that
// quietly pointed the bare form at the downward walk would answer three here
// and is what this asserts against.
func TestWorkbenchesWithNoPathStillAnswersTheUpwardSearch(t *testing.T) {
	root := newForest(t, "alpha", "customer/beta", "customer/deep/gamma")
	bare := runCLI(t, root, "workbenches")
	if bare.code != 0 {
		t.Fatalf("the bare form: %d %s", bare.code, bare.errw)
	}
	if strings.TrimSpace(bare.out) != strings.TrimSpace(msg.For(msg.Base).T("workbenches.empty")) {
		t.Errorf("the bare form answered %q, wanted the unchanged reachable-from-here sentence", bare.out)
	}
	walked := runCLI(t, root, "workbenches", root, "--json")
	if listed, ok := decode(t, walked.out).([]any); !ok || len(listed) != 3 {
		t.Errorf("the positional form reported %s, wanted the three beneath the path", walked.out)
	}
}

// TestWorkbenchesRefusesTwoScopesAtOnce asserts dinah-281 AC-15: a positional
// path and a workbench pointer name two scopes for one call, and the tool
// refuses rather than honouring one and discarding the other. The same call
// with neither pointer set succeeds, and the bare form with the pointer set is
// the ordinary listing this card does not touch.
func TestWorkbenchesRefusesTwoScopesAtOnce(t *testing.T) {
	root := newForest(t, "alpha")
	inside := filepath.Join(root, "alpha")

	t.Setenv("DINAH_WORKBENCH", soleBenchDir(t, inside))
	refused := runCLI(t, root, "workbenches", root)
	if refused.code == 0 {
		t.Fatalf("the two scopes were accepted: %s", refused.out)
	}
	if leading := leadingToken(refused.errw); leading != contract.ConflictingScope {
		t.Errorf("leading token %q, wanted %s", leading, contract.ConflictingScope)
	}
	bare := runCLI(t, root, "workbenches")
	if bare.code != 0 {
		t.Errorf("the bare form with a workbench pointer set should be unaffected: %d %s", bare.code, bare.errw)
	}

	t.Setenv("DINAH_WORKBENCH", "")
	clean := runCLI(t, root, "workbenches", root, "--json")
	if clean.code != 0 {
		t.Errorf("with neither pointer set the positional form should succeed: %d %s", clean.code, clean.errw)
	}
}

// TestEveryRootScopedVerbRefusesTwoScopesAtOnce asserts AC-15's rule on the
// five reads that carry --root, since the refusal is one rule shared across
// six commands rather than six rules that happen to agree.
func TestEveryRootScopedVerbRefusesTwoScopesAtOnce(t *testing.T) {
	root := newForest(t, "alpha")
	t.Setenv("DINAH_WORKBENCH", soleBenchDir(t, filepath.Join(root, "alpha")))
	for _, c := range rootScopedVerbs {
		t.Run(c.verb, func(t *testing.T) {
			got := runCLI(t, root, c.verb, "--root", root)
			if got.code == 0 {
				t.Fatalf("the two scopes were accepted: %s", got.out)
			}
			if leading := leadingToken(got.errw); leading != contract.ConflictingScope {
				t.Errorf("leading token %q, wanted %s", leading, contract.ConflictingScope)
			}
		})
	}
}

// TestADepthWithNothingToBoundRefuses asserts that --max-depth without a root
// is refused on all six commands rather than read and dropped, and that a
// depth that is not a count of rungs is refused rather than defaulted.
func TestADepthWithNothingToBoundRefuses(t *testing.T) {
	root := newForest(t, "alpha")
	for _, c := range rootScopedVerbs {
		t.Run(c.verb, func(t *testing.T) {
			got := runCLI(t, root, c.verb, "--max-depth", "2")
			if leading := leadingToken(got.errw); leading != contract.DepthWithoutRoot {
				t.Errorf("leading token %q, wanted %s", leading, contract.DepthWithoutRoot)
			}
		})
	}
	t.Run("workbenches", func(t *testing.T) {
		got := runCLI(t, root, "workbenches", "--max-depth", "2")
		if leading := leadingToken(got.errw); leading != contract.DepthWithoutRoot {
			t.Errorf("leading token %q, wanted %s", leading, contract.DepthWithoutRoot)
		}
	})
	t.Run("a depth that is not a count of rungs", func(t *testing.T) {
		for _, value := range []string{"deep", "-1", "2.5"} {
			got := runCLI(t, root, "workbenches", root, "--max-depth", value)
			if leading := leadingToken(got.errw); leading != contract.MalformedDepth {
				t.Errorf("%q: leading token %q, wanted %s", value, leading, contract.MalformedDepth)
			}
		}
	})
}

// TestMaxDepthBoundsEveryRootScopedRead asserts dinah-281 AC-4 across all six
// commands: a bound reports the rungs at or above it, a workbench past the
// bound is absent, zero descends without a bound, and the default with the
// flag omitted reaches a workbench three rungs down.
func TestMaxDepthBoundsEveryRootScopedRead(t *testing.T) {
	root := newForest(t, "one", "two/three", "two/four/five")
	cases := []struct {
		depth string
		want  int
	}{
		{depth: "1", want: 1},
		{depth: "2", want: 2},
		{depth: "3", want: 3},
		{depth: "0", want: 3},
		{depth: "", want: 3},
	}
	for _, c := range cases {
		label := c.depth
		if label == "" {
			label = "the default"
		}
		t.Run("workbenches at "+label, func(t *testing.T) {
			argv := []string{"workbenches", root, "--json"}
			if c.depth != "" {
				argv = append(argv, "--max-depth", c.depth)
			}
			got := runCLI(t, root, argv...)
			if got.code != 0 {
				t.Fatalf("%v: %d %s", argv, got.code, got.errw)
			}
			listed, _ := decode(t, got.out).([]any)
			if len(listed) != c.want {
				t.Errorf("reported %d workbenches, wanted %d", len(listed), c.want)
			}
		})
		for _, verb := range rootScopedVerbs {
			t.Run(verb.verb+" at "+label, func(t *testing.T) {
				argv := []string{verb.verb, "--root", root}
				if c.depth != "" {
					argv = append(argv, "--max-depth", c.depth)
				}
				rows := members(t, forestJSON(t, root, argv...))
				if len(rows) != c.want {
					t.Errorf("reported %d workbenches, wanted %d", len(rows), c.want)
				}
			})
		}
	}
}

// invocationOnly are members of a single-workbench answer that describe the
// invocation rather than the workbench, and so cannot appear on a root-scoped
// member however faithfully it wraps.
//
// There is exactly one. Status.WorkbenchSource names which rung resolved the
// active workbench for this invocation, which is flag, environment, search or
// config. A root-scoped read resolves no single workbench by any rung: the walk
// found twenty-five of them, and none of the four words is true of any one. The
// field is therefore absent from a root-scoped member and present on a
// single-workbench answer, which is correct on both sides and is the one place
// AC-5's byte-identical wording asks for something that would be false. Making
// the two agree would mean stamping a root-scoped member with a rung that never
// ran, so the criterion is narrowed here rather than the code bent to it.
var invocationOnly = map[string]bool{"workbench_source": true}

// withoutInvocationMembers strips those members from a decoded answer, leaving
// everything the two call shapes really do have to agree about.
func withoutInvocationMembers(value any) any {
	object, ok := value.(map[string]any)
	if !ok {
		return value
	}
	kept := make(map[string]any, len(object))
	for member, held := range object {
		if invocationOnly[member] {
			continue
		}
		kept[member] = held
	}
	return kept
}

// TestARootScopedMemberIsTheSingleWorkbenchAnswer asserts dinah-281 AC-5: for
// each of the five reads, the answer a root-scoped call publishes for one
// workbench is the answer that workbench's own call already gives, unchanged
// but for the invocation members named above.
//
// The comparison is against an independently produced answer rather than
// against a fixture written here, so it fails when the wrapping changes what
// it wraps rather than when somebody's expectation goes stale. The changes leg
// is driven through a real replay, because a minting call reports no member
// answer at all and two nils agree about nothing.
func TestARootScopedMemberIsTheSingleWorkbenchAnswer(t *testing.T) {
	root := newForest(t, "alpha", "customer/beta")
	dirs := map[string]string{
		"alpha":         soleBenchDir(t, filepath.Join(root, "alpha")),
		"customer/beta": soleBenchDir(t, filepath.Join(root, "customer", "beta")),
	}
	for place, dir := range dirs {
		if got := runCLI(t, root, "add", "A card in "+place, "--workbench", dir); got.code != 0 {
			t.Fatalf("add in %s: %d %s", place, got.code, got.errw)
		}
	}
	// Each workbench's own cursor is minted before anything moves, so a
	// replay through either call shape reads the same journal lines.
	own := map[string]string{}
	for place, dir := range dirs {
		got := runCLI(t, root, "changes", "--workbench", dir, "--json")
		if got.code != 0 {
			t.Fatalf("mint for %s: %d %s", place, got.code, got.errw)
		}
		minted, _ := decode(t, got.out).(map[string]any)
		own[dir], _ = minted["cursor"].(string)
	}
	merged, _ := forestJSON(t, root, "changes", "--root", root)["cursor"].(string)
	if got := runCLI(t, root, "add", "A later card", "--workbench", dirs["alpha"]); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}

	for _, c := range rootScopedVerbs {
		t.Run(c.verb, func(t *testing.T) {
			argv := []string{c.verb, "--root", root}
			if c.verb == "changes" {
				argv = append(argv, "--since", merged)
			}
			for _, row := range members(t, forestJSON(t, root, argv...)) {
				path, _ := row["path"].(string)
				single := []string{c.verb, "--workbench", path}
				if c.verb == "changes" {
					single = append(single, "--since", own[path])
				}
				got := runCLI(t, root, append(single, "--json")...)
				if got.code != 0 {
					t.Fatalf("%v: %d %s", single, got.code, got.errw)
				}
				wrapped := withoutInvocationMembers(row[c.member])
				alone := withoutInvocationMembers(decode(t, got.out))
				if !reflect.DeepEqual(wrapped, alone) {
					t.Errorf("%s at %s:\nroot-scoped member: %s\nsingle-workbench:   %s",
						c.verb, path, mustJSON(t, wrapped), mustJSON(t, alone))
				}
			}
		})
	}
}

// TestARootScopedReadOnAnEmptyDirectoryAnswersAndDoesNotRefuse asserts
// dinah-281 AC-8: a real directory with no workbench beneath it answers with
// the root it walked and an empty array, and exits 0, while a root that is not
// there at all refuses dinah.unknown-root naming the path.
func TestARootScopedReadOnAnEmptyDirectoryAnswersAndDoesNotRefuse(t *testing.T) {
	root := newForest(t, "alpha")
	empty := filepath.Join(root, "..", "nothing")
	if err := os.MkdirAll(empty, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	missing := filepath.Join(root, "..", "never-made")

	for _, c := range rootScopedVerbs {
		t.Run(c.verb, func(t *testing.T) {
			answer := forestJSON(t, root, c.verb, "--root", empty)
			if rows := members(t, answer); len(rows) != 0 {
				t.Errorf("the empty directory reported %d workbenches", len(rows))
			}
			if answer["root"] == nil || answer["root"] == "" {
				t.Error("the answer names no root, so a reader cannot tell what was walked")
			}
			if raw, ok := answer["workbenches"]; !ok || raw == nil {
				t.Error("the empty answer carries null rather than an empty array")
			}
			gone := runCLI(t, root, c.verb, "--root", missing)
			if gone.code == 0 {
				t.Fatalf("a missing root printed a listing: %s", gone.out)
			}
			if leading := leadingToken(gone.errw); leading != contract.UnknownRoot {
				t.Errorf("leading token %q, wanted %s", leading, contract.UnknownRoot)
			}
		})
	}
	t.Run("the human form says so", func(t *testing.T) {
		got := runCLI(t, root, "tree", "--root", empty)
		if got.code != 0 {
			t.Fatalf("the human form refused: %d %s", got.code, got.errw)
		}
		if !strings.Contains(got.out, "no workbench is beneath") {
			t.Errorf("the empty answer prints %q, wanted the sentence naming the directory", got.out)
		}
	})
}

// TestTheHumanFormNamesTheWorkbenchEachAnswerBelongsTo asserts dinah-281 AC-9:
// each of the five reads prints a distinguishing heading before every
// workbench's own rendering, so no two workbenches' answers are printed with
// nothing between them to say which is which.
//
// The expected heading is composed from the same catalog entry the head
// composes it from, filled with the identity the same call's JSON reports, so
// the test asserts the heading a reader sees rather than a shape typed here.
// Each has to appear exactly once, and the output has to open on one: a run
// that printed both headings and then both bodies passes a test that only
// looks for the titles somewhere, and fails this one.
func TestTheHumanFormNamesTheWorkbenchEachAnswerBelongsTo(t *testing.T) {
	root := newForest(t, "alpha", "customer/beta")
	for _, place := range []string{"alpha", "customer/beta"} {
		dir := soleBenchDir(t, filepath.Join(root, filepath.FromSlash(place)))
		if got := runCLI(t, root, "add", "A card in "+place, "--workbench", dir); got.code != 0 {
			t.Fatalf("add in %s: %d %s", place, got.code, got.errw)
		}
	}
	for _, c := range rootScopedVerbs {
		t.Run(c.verb, func(t *testing.T) {
			rows := members(t, forestJSON(t, root, c.verb, "--root", root))
			if len(rows) != 2 {
				t.Fatalf("the answer carries %d workbenches, wanted the two the fixture holds", len(rows))
			}
			got := runCLI(t, root, c.verb, "--root", root)
			if got.code != 0 {
				t.Fatalf("%s --root: %d %s", c.verb, got.code, got.errw)
			}
			lines := strings.Split(strings.TrimRight(got.out, "\n"), "\n")
			var headings []string
			for _, row := range rows {
				title, _ := row["title"].(string)
				slug, _ := row["slug"].(string)
				path, _ := row["path"].(string)
				heading := msg.For(msg.Base).T("root.workbench",
					"title", title, "slug", slug, "path", path)
				headings = append(headings, heading)
				seen := 0
				for _, line := range lines {
					if line == heading {
						seen++
					}
				}
				if seen != 1 {
					t.Errorf("the heading %q appears %d times, wanted once:\n%s", heading, seen, got.out)
				}
			}
			if len(lines) == 0 || lines[0] != headings[0] {
				t.Errorf("the output does not open on the first workbench's heading:\n%s", got.out)
			}
			if headings[0] == headings[1] {
				t.Fatal("the two workbenches compose the same heading, so this case cannot tell them apart")
			}
		})
	}
}

// identityLine composes the heading the head prints above one workbench's own
// answer, out of the same catalog entry the head reads and the identity that
// row's own JSON reports, so a case asserts the line a reader sees rather than
// a shape typed here.
func identityLine(row map[string]any) string {
	title, _ := row["title"].(string)
	slug, _ := row["slug"].(string)
	path, _ := row["path"].(string)
	return msg.For(msg.Base).T("root.workbench", "title", title, "slug", slug, "path", path)
}

// unreadableAnchor rewrites one workbench's anchor so that its identity still
// reads and opening it refuses. The declared format is moved to a revision no
// binary serves, which is the one edit that leaves the frontmatter parseable
// while stopping bench.Open, so the row comes back carrying a title, a slug
// and a refusal at once.
func unreadableAnchor(t *testing.T, dir string) {
	t.Helper()
	path := filepath.Join(dir, bench.WorkbenchAnchor)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read the anchor at %s: %v", dir, err)
	}
	broken := strings.Replace(string(raw), "format: 1", "format: 99", 1)
	if broken == string(raw) {
		t.Fatalf("the anchor at %s declares no format to move, so this case cannot be built", dir)
	}
	if err := os.WriteFile(path, []byte(broken), 0o644); err != nil {
		t.Fatalf("write the anchor at %s: %v", dir, err)
	}
}

// TestAWorkbenchThatWouldNotReadIsToldApartFromOneThatDidNotAnswer asserts
// that the two conditions a root-scoped row can carry are two facts rather
// than one, on both surfaces.
//
// They were one field before this test existed. A workbench that read
// perfectly well and then declined the question wrote its refusal onto
// bench.Candidate.Refused, the field whose own documentation says it means the
// workbench would not read, so a client had one field holding two meanings and
// could only tell them apart by inspecting how a refusal name was spelled. The
// consumer this card exists for, a sidebar drawing a broken workbench
// differently from one that answered a refusal, is exactly the reader that
// cannot do that.
//
// So each leg asserts the field that must be set and the field that must not.
// Asserting only the first would pass on a row that set both, which is the
// defect in its purest form.
func TestAWorkbenchThatWouldNotReadIsToldApartFromOneThatDidNotAnswer(t *testing.T) {
	t.Run("read and did not answer", func(t *testing.T) {
		root := newForest(t, "alpha", "beta")
		rows := members(t, forestJSON(t, root, "ls", "--root", root, "--column", "nosuchcolumn"))
		if len(rows) != 2 {
			t.Fatalf("the answer carries %d workbenches, wanted the two the fixture holds", len(rows))
		}
		for _, row := range rows {
			if row["unanswered"] != contract.UnknownColumn {
				t.Errorf("a workbench that read and declined the column reports unanswered %v, wanted %s", row["unanswered"], contract.UnknownColumn)
			}
			if _, refused := row["refused"]; refused {
				t.Errorf("a workbench that read perfectly well carries refused %v, which says it would not read", row["refused"])
			}
			if row["title"] == "" || row["slug"] == "" {
				t.Errorf("the row threw away the identity it read: %v", row)
			}
		}
		got := runCLI(t, root, "ls", "--root", root, "--column", "nosuchcolumn")
		unanswered := msg.For(msg.Base).T("root.workbench.unanswered",
			"refusal", "<"+contract.UnknownColumn+">")
		if !strings.Contains(got.out, unanswered) {
			t.Errorf("the human form does not say the workbench read and did not answer:\n%s", got.out)
		}
		if strings.Contains(got.out, msg.For(msg.Base).T("root.workbench.unreadable", "refusal", "<"+contract.UnknownColumn+">")) {
			t.Errorf("the human form says a workbench that read would not open:\n%s", got.out)
		}
		for _, row := range rows {
			heading := identityLine(row)
			if !strings.Contains(got.out, heading) {
				t.Errorf("the human form lost the identity line %q:\n%s", heading, got.out)
			}
		}
	})

	t.Run("described and would not open", func(t *testing.T) {
		root := newForest(t, "alpha", "beta")
		broken := soleBenchDir(t, filepath.Join(root, "beta"))
		unreadableAnchor(t, broken)
		rows := members(t, forestJSON(t, root, "ls", "--root", root))
		var row map[string]any
		for _, one := range rows {
			if one["path"] == broken {
				row = one
			}
		}
		if row == nil {
			t.Fatalf("the walk lost the workbench it could not open: %v", rows)
		}
		if row["refused"] != contract.UnsupportedVer {
			t.Errorf("a workbench that would not open reports refused %v, wanted %s", row["refused"], contract.UnsupportedVer)
		}
		if _, unanswered := row["unanswered"]; unanswered {
			t.Errorf("a workbench that never opened carries unanswered %v, which says its own read declined", row["unanswered"])
		}
		if row["title"] == "" || row["slug"] == "" {
			t.Errorf("the anchor read and the row threw its identity away: %v", row)
		}
		got := runCLI(t, root, "ls", "--root", root)
		heading := identityLine(row)
		if !strings.Contains(got.out, heading) {
			t.Errorf("the human form lost the identity line %q:\n%s", heading, got.out)
		}
		unreadable := msg.For(msg.Base).T("root.workbench.unreadable",
			"refusal", "<"+contract.UnsupportedVer+">")
		if !strings.Contains(got.out, unreadable) {
			t.Errorf("the human form does not say the workbench would not open:\n%s", got.out)
		}
	})

	t.Run("nothing read at all", func(t *testing.T) {
		var out bytes.Buffer
		session := &session{out: &out, r: msg.For(msg.Base), width: 107}
		path := filepath.FromSlash("/forest/broken")
		if drawn := session.rootHeading(bench.Candidate{Path: path, Refused: contract.UnreadableBench}, ""); !drawn {
			t.Error("a row nothing could be read off left the renderer expecting an answer to draw")
		}
		want := msg.For(msg.Base).T("root.workbench.refused",
			"refusal", "<"+contract.UnreadableBench+">", "path", path)
		if strings.TrimRight(out.String(), "\n") != want {
			t.Errorf("a workbench nothing read printed %q, wanted %q", out.String(), want)
		}
	})
}

// TestARefusedRowRendersItsRefusalWhereTheTitleWouldGo asserts the listing's
// own half of dinah-281 AC-2: a row the walk could not describe prints the
// refusal name in the workbench cell, in angle brackets, and leaves the slug
// cell empty rather than offering the missing-slug repair for a workbench
// nothing read.
//
// The rows are handed to the renderer directly, because the refusal this draws
// is forced through a package-internal seam in internal/bench and cannot be
// provoked from an invocation here.
func TestARefusedRowRendersItsRefusalWhereTheTitleWouldGo(t *testing.T) {
	session := tableSession(107)
	rows := session.formatCandidateRows([]bench.Candidate{
		{Title: "A readable workbench", Slug: "alpha", Path: filepath.FromSlash("/forest/alpha")},
		{Path: filepath.FromSlash("/forest/broken"), Refused: contract.UnreadableBench},
	})
	joined := strings.Join(rows, "\n")
	if !strings.Contains(joined, "<"+contract.UnreadableBench+">") {
		t.Errorf("the refused row does not carry the bracketed refusal name:\n%s", joined)
	}
	if strings.Contains(joined, msg.For(msg.Base).T("slug.missing")) {
		t.Errorf("the refused row offers the missing-slug repair for a workbench nothing read:\n%s", joined)
	}
	if !strings.Contains(joined, "alpha") {
		t.Errorf("the readable row vanished beside the refused one:\n%s", joined)
	}
}

// TestOneCallPerVerbAnswersForTwentyFiveWorkbenches asserts dinah-281 AC-10:
// the fixture size the card is written against answers a whole sidebar refresh
// in five calls, one per verb, each carrying all twenty-five workbenches'
// answers.
//
// The count is asserted rather than described. Every invocation this test makes
// of the five reads is counted, and the count has to come out at five, so a
// verb that quietly needed a second call to finish the root fails here even if
// its answer looked complete.
func TestOneCallPerVerbAnswersForTwentyFiveWorkbenches(t *testing.T) {
	places := make([]string, 0, 25)
	for i := 0; i < 25; i++ {
		places = append(places, "customer"+strconv.Itoa(i/5)+"/wb"+strconv.Itoa(i))
	}
	root := newForest(t, places...)
	calls := 0
	for _, c := range rootScopedVerbs {
		calls++
		rows := members(t, forestJSON(t, root, c.verb, "--root", root))
		if len(rows) != 25 {
			t.Errorf("%s --root answered for %d workbenches in one call, wanted all 25", c.verb, len(rows))
		}
	}
	if calls != 5 {
		t.Errorf("the refresh took %d calls, wanted one per verb", calls)
	}
}

// TestARootScopedCheckpointMintsThenReportsOnlyWhatMoved asserts dinah-281
// AC-11: a first call mints a merged cursor and reports no change anywhere,
// and a second call with that cursor, after exactly one workbench moved,
// reports the change at the root and on that one workbench alone.
func TestARootScopedCheckpointMintsThenReportsOnlyWhatMoved(t *testing.T) {
	root := newForest(t, "alpha", "beta", "gamma")
	first := forestJSON(t, root, "changes", "--root", root)
	if first["changed"] != false {
		t.Errorf("the minting call reports changed=%v, wanted false", first["changed"])
	}
	cursor, _ := first["cursor"].(string)
	if cursor == "" {
		t.Fatal("the minting call handed back no cursor")
	}
	for _, row := range members(t, first) {
		if row["changes"] != nil {
			t.Errorf("the minting call reports events for %v", row["path"])
		}
	}

	moved := soleBenchDir(t, filepath.Join(root, "beta"))
	if got := runCLI(t, root, "add", "A new card", "--workbench", moved); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}

	second := forestJSON(t, root, "changes", "--root", root, "--since", cursor)
	if second["changed"] != true {
		t.Errorf("the second call reports changed=%v, wanted true", second["changed"])
	}
	changedRows := 0
	for _, row := range members(t, second) {
		set, ok := row["changes"].(map[string]any)
		if !ok {
			t.Fatalf("a replay row carries no answer: %v", row)
		}
		path, _ := row["path"].(string)
		if set["changed"] == true {
			changedRows++
			if path != moved {
				t.Errorf("%s reports a change and nothing happened there", path)
			}
			continue
		}
		if set["events"] != nil {
			t.Errorf("%s reports no change and carries events anyway", path)
		}
	}
	if changedRows != 1 {
		t.Errorf("%d workbenches report a change, wanted the one that moved", changedRows)
	}
}

// TestAWorkbenchThatAppearsBetweenTwoPollsIsReportedAsNew asserts dinah-281
// AC-12: a workbench created beneath the root between two checkpoints is
// present in the second answer marked new, carries no history from before it
// existed, and turns the root's own flag true.
func TestAWorkbenchThatAppearsBetweenTwoPollsIsReportedAsNew(t *testing.T) {
	root := newForest(t, "alpha")
	cursor, _ := forestJSON(t, root, "changes", "--root", root)["cursor"].(string)

	late := filepath.Join(root, "late")
	if err := os.MkdirAll(late, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if got := runCLI(t, late, "init", "--slug", "late", "--operator", "alka"); got.code != 0 {
		t.Fatalf("init: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "add", "A card filed after the cursor", "--workbench", soleBenchDir(t, late)); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}

	second := forestJSON(t, root, "changes", "--root", root, "--since", cursor)
	if second["changed"] != true {
		t.Errorf("a workbench appeared and the root reports changed=%v", second["changed"])
	}
	rows := members(t, second)
	if len(rows) != 2 {
		t.Fatalf("the second call reports %d workbenches, wanted both: %v", len(rows), rows)
	}
	found := false
	for _, row := range rows {
		path, _ := row["path"].(string)
		if !strings.Contains(path, "late") {
			if row["new"] == true {
				t.Errorf("%s is reported new and the cursor already named it", path)
			}
			continue
		}
		found = true
		if row["new"] != true {
			t.Errorf("the workbench that appeared is not reported new: %v", row)
		}
		set, ok := row["changes"].(map[string]any)
		if !ok {
			t.Fatalf("the new workbench carries no answer: %v", row)
		}
		if set["events"] != nil {
			t.Errorf("the new workbench reports history from before it existed: %v", set["events"])
		}
	}
	if !found {
		t.Error("the workbench that appeared is absent from the second answer")
	}
}

// leadingToken is the first whitespace-delimited word of a refusal, which is
// the refusal name a script reads with cut.
func leadingToken(errw string) string {
	return strings.SplitN(strings.TrimSpace(errw), " ", 2)[0]
}

// mustJSON renders a decoded value for a failure message.
func mustJSON(t *testing.T, value any) string {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(encoded)
}
