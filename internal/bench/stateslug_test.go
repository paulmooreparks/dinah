package bench

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"dinah/internal/contract"
)

// operatorFlow is the flow of the workbench this project runs itself on, used
// as the fixture for slug derivation because it is the input the migration
// meets first and the one whose answers were worked out by hand ahead of the
// code.
var operatorFlow = []struct {
	title string
	slug  string
}{
	{title: "Intake", slug: "intake"},
	{title: "Triage", slug: "triage"},
	{title: "Design Queue", slug: "design-queue"},
	{title: "Spec", slug: "spec"},
	{title: "Agent Design Review", slug: "agent-design-review"},
	{title: "Operator Design Review", slug: "operator-design-review"},
	{title: "Build Queue", slug: "build-queue"},
	{title: "Implement", slug: "implement"},
	{title: "Agent Code Review", slug: "agent-code-review"},
	{title: "Operator Code Review", slug: "operator-code-review"},
	{title: "Test", slug: "test"},
	{title: "Merge", slug: "merge"},
	{title: "Acceptance", slug: "acceptance"},
	{title: "Done", slug: "done"},
}

// TestSlugDerivationMatchesTheHandRun asserts that SlugifyDashed reproduces the
// slug worked out by hand for each title of the live flow, and that the
// fourteen answers are distinct, which is what makes the migration over that
// workbench collision-free.
func TestSlugDerivationMatchesTheHandRun(t *testing.T) {
	seen := map[string]string{}
	for _, want := range operatorFlow {
		got := SlugifyDashed(want.title)
		if got != want.slug {
			t.Errorf("%q derived %q, wanted %q", want.title, got, want.slug)
		}
		if !ValidStateSlug(got) {
			t.Errorf("%q derived %q, which the grammar refuses", want.title, got)
		}
		if first, ok := seen[got]; ok {
			t.Errorf("%q and %q both derived %q", first, want.title, got)
		}
		seen[got] = want.title
	}
}

// TestSlugDerivationLowersByAsciiRules asserts that derivation lowercases the
// way the format's encoding rule requires, so a title carrying an uppercase I
// derives the same slug on every machine. A locale-aware lowering of I under
// Turkish is not i, and a slug that differed by locale would be a different
// name for the same state depending on who ran the tool.
func TestSlugDerivationLowersByAsciiRules(t *testing.T) {
	if got := SlugifyDashed("Interim Inspection"); got != "interim-inspection" {
		t.Errorf("derived %q, wanted interim-inspection", got)
	}
	if got := SlugifyDashed("I"); got != "i" {
		t.Errorf("an uppercase I derived %q, wanted i", got)
	}
}

// TestSlugDerivationShapesEveryAwkwardTitle asserts the derivation rules the
// grammar forces: a run of characters outside the set collapses to one dash,
// no dash leads or trails, a leading run of digits goes because a slug opens
// on a letter, and a title with no letter in it derives nothing at all.
func TestSlugDerivationShapesEveryAwkwardTitle(t *testing.T) {
	cases := []struct {
		title  string
		wanted string
	}{
		{title: "Agent Code Review", wanted: "agent-code-review"},
		{title: "  Spec  ", wanted: "spec"},
		{title: "Design / Queue", wanted: "design-queue"},
		{title: "2026 Review", wanted: "review"},
		{title: "Stage 2", wanted: "stage-2"},
		{title: "---", wanted: ""},
		{title: "2026", wanted: ""},
		{title: "", wanted: ""},
	}
	for _, c := range cases {
		if got := SlugifyDashed(c.title); got != c.wanted {
			t.Errorf("%q derived %q, wanted %q", c.title, got, c.wanted)
		}
	}
}

// TestTheGrammarRefusesEveryMalformedSlug asserts the shapes StateSlugPattern
// excludes: a leading digit, a leading or trailing dash, a doubled dash and an
// uppercase character. Each is refused wherever a slug is set by hand.
func TestTheGrammarRefusesEveryMalformedSlug(t *testing.T) {
	valid := []string{"spec", "agent-code-review", "stage-2", "a", "a-1-b"}
	for _, slug := range valid {
		if !ValidStateSlug(slug) {
			t.Errorf("%q should conform to %s", slug, StateSlugPattern)
		}
	}
	malformed := []string{"", "2spec", "-spec", "spec-", "agent--code", "Spec", "agent code", "agent_code"}
	for _, slug := range malformed {
		if ValidStateSlug(slug) {
			t.Errorf("%q should not conform to %s", slug, StateSlugPattern)
		}
	}
}

// TestOpenRefusesAMalformedOrDuplicatedSlug asserts CORE-STATE-10 over a
// workbench on disk: a stored slug outside the grammar and a slug two states
// share are each refused as malformed, at any declared major, because both are
// a stored value somebody chose rather than a field nobody has filled in.
func TestOpenRefusesAMalformedOrDuplicatedSlug(t *testing.T) {
	root := newFixture(t)
	write(t, filepath.Join(root, StatesDir, "b00000000001", StateAnchor), "---\ntitle: Only\nslug: Only\nkind: work\n---\n")
	if _, err := Open(root); !refusedMalformed(err) {
		t.Errorf("a malformed slug should refuse malformed, got %v", err)
	}

	shared := newTwoStateFixture(t, "dinah-core/1.0", "review", "review")
	if _, err := Open(shared); !refusedMalformed(err) {
		t.Errorf("two states sharing a slug should refuse malformed, got %v", err)
	}
}

// TestOpenGatesTheMissingSlugOnTheDeclaredMajor asserts the rule that keeps an
// unmigrated workbench usable: a state carrying no slug opens while the
// workbench declares a major below SlugMandatoryMajor, and is refused once it
// declares that major, because the revision introducing CORE-STATE-10 is the
// revision that binds a workbench to it.
func TestOpenGatesTheMissingSlugOnTheDeclaredMajor(t *testing.T) {
	tolerated := newFixture(t)
	write(t, filepath.Join(tolerated, StatesDir, "b00000000001", StateAnchor), "---\ntitle: Only\nkind: work\n---\n")
	opened, err := Open(tolerated)
	if err != nil {
		t.Fatalf("a workbench below the mandating major should open: %v", err)
	}
	if opened.States[0].Slug != "" {
		t.Errorf("the state carries slug %q, wanted none", opened.States[0].Slug)
	}

	if err := admitSlug(&State{ID: "b00000000001"}, SlugMandatoryMajor, map[string]bool{}); !refusedMalformed(err) {
		t.Errorf("a missing slug at the mandating major should refuse malformed, got %v", err)
	}
	if err := admitSlug(&State{ID: "b00000000001"}, SlugMandatoryMajor-1, map[string]bool{}); err != nil {
		t.Errorf("a missing slug below the mandating major should open, got %v", err)
	}
}

// TestStateByRefReadsTheSlugAheadOfTheTitle asserts the resolution order a
// reference takes: the identifier, then the slug, then the title. A reference
// matching one state's slug and another state's title resolves to the state
// whose slug it is, which is the tier that makes a spaced title reachable
// without quoting it.
func TestStateByRefReadsTheSlugAheadOfTheTitle(t *testing.T) {
	root := newTwoStateFixture(t, "dinah-core/1.0", "agent-code-review", "review")
	opened, err := Open(root)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	// The second state is titled Review and the first is slugged review, so
	// the two tiers disagree and the slug tier is the one that answers.
	opened.States[0].Slug = "review"
	opened.States[1].Title = "Review"
	if state := opened.StateByRef("review"); state == nil || state.ID != "b00000000001" {
		t.Errorf("a slug should resolve ahead of another state's title, got %v", state)
	}
	if state := opened.StateByRef("b00000000002"); state == nil || state.ID != "b00000000002" {
		t.Errorf("an identifier should resolve ahead of everything, got %v", state)
	}
	opened.States[0].Slug = "agent-code-review"
	if state := opened.StateByRef("agent-code-review"); state == nil || state.ID != "b00000000001" {
		t.Error("a slug should resolve a state whose title needs quoting")
	}
	if state := opened.StateByRef("AGENT-CODE-REVIEW"); state == nil || state.ID != "b00000000001" {
		t.Error("a slug should resolve without regard to ASCII case")
	}
}

// TestCheckReportsWhatTheSlugMigrationRepairs asserts the checker and the
// repair together: a state with no slug is reported while the workbench
// declares a major below the mandating one, the migration derives a slug from
// each title and says which one it gave, a second run finds nothing left to do,
// and the workbench checks clean afterwards.
func TestCheckReportsWhatTheSlugMigrationRepairs(t *testing.T) {
	root := newFixture(t)
	write(t, filepath.Join(root, StatesDir, "b00000000001", StateAnchor), "---\ntitle: Agent Code Review\nkind: work\n---\nState text.\n")
	opened, err := Open(root)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	findings, err := opened.Check()
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if len(findings) != 1 || findings[0].Key != FindingSlugMissing {
		t.Fatalf("wanted one missing-slug finding, got %v", findings)
	}

	assigned, reported := opened.BackfillStateSlugs()
	if len(reported) != 0 {
		t.Errorf("the migration reported %v", reported)
	}
	if len(assigned) != 1 || assigned[0].Slug != "agent-code-review" {
		t.Fatalf("the migration assigned %v", assigned)
	}

	reopened, err := Open(root)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	if got := reopened.States[0].Slug; got != "agent-code-review" {
		t.Errorf("the state anchor carries slug %q", got)
	}
	if body := readAnchor(t, root); !strings.Contains(body, "State text.") {
		t.Errorf("the migration lost the state's own instructions:\n%s", body)
	}
	again, reported := reopened.BackfillStateSlugs()
	if len(again) != 0 || len(reported) != 0 {
		t.Errorf("a second run assigned %v and reported %v", again, reported)
	}
	findings, err = reopened.Check()
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if len(findings) != 0 {
		t.Errorf("a migrated workbench should check clean, got %v", findings)
	}
}

// TestCheckReportsAStoredSlugItWillNotRepair asserts that a malformed or
// duplicated slug is reported and left alone. Either one is a value somebody
// wrote on purpose, so the checker names it for a person and the migration does
// not overwrite a decision it cannot read.
func TestCheckReportsAStoredSlugItWillNotRepair(t *testing.T) {
	root := newTwoStateFixture(t, "dinah-core/1.0", "review", "review")
	// Open refuses the duplicate, so the checker is driven over a bench built
	// in memory, which is the shape a workbench reaches check in once a later
	// release tolerates reading one to report on it.
	opened := &Bench{
		Root:    root,
		Profile: "dinah-core/1.0",
		States: []*State{
			{ID: "b00000000001", Title: "One", Slug: "review"},
			{ID: "b00000000002", Title: "Two", Slug: "review"},
			{ID: "b00000000003", Title: "Three", Slug: "Three"},
		},
	}
	findings := opened.checkStateSlugs()
	if len(findings) != 2 {
		t.Fatalf("wanted a duplicate and a malformed finding, got %v", findings)
	}
	if findings[0].Key != FindingSlugDuplicate || findings[1].Key != FindingSlugMalformed {
		t.Errorf("findings %v", findings)
	}
}

// TestInstantiateDerivesASlugForEveryState asserts CORE-JSON-9 and
// CORE-STATE-10 over the interchange form: a state object carrying no slug
// member is given one derived from its title, two states of one title take the
// derived name and the first free suffix in array order, and an explicit slug
// is taken as given.
func TestInstantiateDerivesASlugForEveryState(t *testing.T) {
	definition := readDefinition(t, `{
	  "profile": "dinah-core/1.0",
	  "title": "Fixture",
	  "states": [
	    {"id": "b00000000001", "title": "Agent Code Review", "kind": "intake"},
	    {"id": "b00000000002", "title": "Review", "kind": "work"},
	    {"id": "b00000000003", "title": "Review", "kind": "work"},
	    {"id": "b00000000004", "title": "Finished", "kind": "done", "slug": "done"}
	  ]
	}`)
	root := filepath.Join(t.TempDir(), "bench")
	if err := Instantiate(root, "fx", "alka", definition); err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	opened, err := Open(root)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	wanted := []string{"agent-code-review", "review", "review-2", "done"}
	for position, want := range wanted {
		if got := opened.States[position].Slug; got != want {
			t.Errorf("state %d carries slug %q, wanted %q", position+1, got, want)
		}
	}
}

// TestInstantiateRefusesASlugItCannotHonour asserts the two malformed cases at
// the write surface: an explicit slug outside the grammar or already asked for
// by another state, and a title deriving to nothing a slug can be made of.
func TestInstantiateRefusesASlugItCannotHonour(t *testing.T) {
	cases := []struct {
		name   string
		states string
	}{
		{
			name:   "an explicit slug outside the grammar",
			states: `{"id": "b00000000001", "title": "One", "kind": "work", "slug": "Not A Slug"}`,
		},
		{
			name:   "two explicit slugs asking for one value",
			states: `{"id": "b00000000001", "title": "One", "kind": "work", "slug": "review"}, {"id": "b00000000002", "title": "Two", "kind": "work", "slug": "review"}`,
		},
		{
			name:   "a title no slug can be derived from",
			states: `{"id": "b00000000001", "title": "---", "kind": "work"}`,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			definition := readDefinition(t, `{"profile": "dinah-core/1.0", "title": "Fixture", "states": [`+c.states+`]}`)
			root := filepath.Join(t.TempDir(), "bench")
			err := Instantiate(root, "fx", "alka", definition)
			if !refusedMalformed(err) {
				t.Errorf("wanted malformed, got %v", err)
			}
		})
	}
}

// TestTheInterchangeFormCarriesTheSlug asserts CORE-JSON-9: a state object
// carries slug beside the members it already admits, and a definition exported
// and read back keeps it.
func TestTheInterchangeFormCarriesTheSlug(t *testing.T) {
	root := newFixture(t)
	opened, err := Open(root)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	encoded, err := opened.Export()
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if !strings.Contains(string(encoded), `"slug": "only"`) {
		t.Errorf("the exported state carries no slug:\n%s", encoded)
	}
	definition, err := ReadDefinition(encoded)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	second := filepath.Join(t.TempDir(), "bench")
	if err := Instantiate(second, "fx", "alka", definition); err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	reopened, err := Open(second)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	if got := reopened.States[0].Slug; got != "only" {
		t.Errorf("the round trip carried slug %q, wanted only", got)
	}
}

// readDefinition parses an interchange object a test wrote by hand.
func readDefinition(t *testing.T, text string) *Definition {
	t.Helper()
	definition, err := ReadDefinition([]byte(text))
	if err != nil {
		t.Fatalf("read definition: %v", err)
	}
	return definition
}

// readAnchor reads the anchor of the fixture's first state.
func readAnchor(t *testing.T, root string) string {
	t.Helper()
	text, err := ReadText(filepath.Join(root, StatesDir, "b00000000001", StateAnchor))
	if err != nil {
		t.Fatalf("read anchor: %v", err)
	}
	return text
}

// newTwoStateFixture writes a workbench of two states carrying the slugs a
// case needs, which is the smallest shape the uniqueness and precedence rules
// can be driven over.
func newTwoStateFixture(t *testing.T, profile, first, second string) string {
	t.Helper()
	root := t.TempDir()
	fm := NewFrontmatter()
	fm.Set("format", "1")
	fm.Set("profile", profile)
	fm.Set("title", "Fixture")
	fm.Set("slug", "fx")
	fm.Set("operator", "alka")
	fm.SetSeq("states", []string{"b00000000001", "b00000000002"})
	write(t, filepath.Join(root, WorkbenchAnchor), fm.Render("Standing text.\n"))
	write(t, filepath.Join(root, StatesDir, "b00000000001", StateAnchor), stateAnchor("One", first, "intake"))
	write(t, filepath.Join(root, StatesDir, "b00000000002", StateAnchor), stateAnchor("Two", second, "work"))
	return root
}

// stateAnchor renders one state anchor for a fixture.
func stateAnchor(title, slug, kind string) string {
	fm := NewFrontmatter()
	fm.Set("title", title)
	if slug != "" {
		fm.Set(SlugField, slug)
	}
	fm.Set("kind", kind)
	return fm.Render("State text.\n")
}

// refusedMalformed reports whether an error is the profile's malformed
// refusal, which is the one answer every rule in this file refuses with.
func refusedMalformed(err error) bool {
	var refusal *contract.Refusal
	return errors.As(err, &refusal) && refusal.Name == contract.Malformed
}
