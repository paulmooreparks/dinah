package bench

import (
	"errors"
	"os"
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
		if !ValidColumnSlug(got) {
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
// name for the same column depending on who ran the tool.
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

// TestTheGrammarRefusesEveryMalformedSlug asserts the shapes ColumnSlugPattern
// excludes: a leading digit, a leading or trailing dash, a doubled dash and an
// uppercase character. Each is refused wherever a slug is set by hand.
func TestTheGrammarRefusesEveryMalformedSlug(t *testing.T) {
	valid := []string{"spec", "agent-code-review", "stage-2", "a", "a-1-b"}
	for _, slug := range valid {
		if !ValidColumnSlug(slug) {
			t.Errorf("%q should conform to %s", slug, ColumnSlugPattern)
		}
	}
	malformed := []string{"", "2spec", "-spec", "spec-", "agent--code", "Spec", "agent code", "agent_code"}
	for _, slug := range malformed {
		if ValidColumnSlug(slug) {
			t.Errorf("%q should not conform to %s", slug, ColumnSlugPattern)
		}
	}
}

// TestOpenGatesAStoredSlugOnTheDeclaredMajor asserts the rule that keeps one
// hand-typed slug from closing a workbench: a stored slug outside the grammar
// and a slug two columns share both open while the workbench declares a major
// below SlugMandatoryMajor, and both refuse at that major.
//
// The tolerant side is what the checker and the migration stand on. Every
// command has to open a workbench before it can do anything with it, so a
// reader refusing here would take away the `dinah check` that names the defect
// and the `dinah check --migrate-slugs` that repairs the columns around it, and
// the operator would have no way back in through the tool at all.
func TestOpenGatesAStoredSlugOnTheDeclaredMajor(t *testing.T) {
	root := newFixture(t)
	write(t, filepath.Join(root, ColumnsDir, "b00000000001", ColumnAnchor), "---\ntitle: Only\nslug: Only\nkind: work\n---\n")
	opened, err := Open(root)
	if err != nil {
		t.Fatalf("a workbench below the mandating major should open: %v", err)
	}
	if got := opened.Columns[0].Slug; got != "Only" {
		t.Errorf("the column carries slug %q, wanted the stored value", got)
	}

	shared, err := Open(newTwoColumnFixture(t, ProfileVersion, "review", "review"))
	if err != nil {
		t.Fatalf("two columns sharing a slug should open below the mandating major: %v", err)
	}
	if len(shared.Columns) != 2 {
		t.Errorf("the workbench carries %d columns", len(shared.Columns))
	}

	// The mandating major is out of Open's reach while this binary claims an
	// earlier one, so the refusing side is driven over admitSlug directly.
	anchor := map[string]string{"path": filepath.Join("columns", "b00000000001", ColumnAnchor)}
	malformed := admitSlug(&Column{ID: "b00000000001", Slug: "Only"}, SlugMandatoryMajor, map[string]bool{}, anchor)
	if !refusedMalformed(malformed) {
		t.Errorf("a malformed slug at the mandating major should refuse malformed, got %v", malformed)
	}
	assertNamesColumnAndFile(t, malformed, "b00000000001", anchor["path"])

	taken := map[string]bool{"review": true}
	duplicate := admitSlug(&Column{ID: "b00000000002", Slug: "review"}, SlugMandatoryMajor, taken, anchor)
	if !refusedMalformed(duplicate) {
		t.Errorf("a duplicated slug at the mandating major should refuse malformed, got %v", duplicate)
	}
	assertNamesColumnAndFile(t, duplicate, "b00000000002", anchor["path"])
}

// assertNamesColumnAndFile asserts the standard every refusal in Open holds to:
// the column it was raised over is named, and so is the file a reader opens to
// repair it.
func assertNamesColumnAndFile(t *testing.T, err error, column, path string) {
	t.Helper()
	var refusal *contract.Refusal
	if !errors.As(err, &refusal) {
		t.Fatalf("wanted a refusal, got %v", err)
	}
	if !strings.Contains(refusal.Detail, column) {
		t.Errorf("the refusal should name column %s, got %q", column, refusal.Detail)
	}
	if refusal.Extra["path"] != path {
		t.Errorf("the refusal should carry path %s, got %q", path, refusal.Extra["path"])
	}
}

// TestOpenGatesTheMissingSlugOnTheDeclaredMajor asserts the rule that keeps an
// unmigrated workbench usable: a column carrying no slug opens while the
// workbench declares a major below SlugMandatoryMajor, and is refused once it
// declares that major, because the revision introducing CORE-STATE-10 is the
// revision that binds a workbench to it.
func TestOpenGatesTheMissingSlugOnTheDeclaredMajor(t *testing.T) {
	tolerated := newFixture(t)
	write(t, filepath.Join(tolerated, ColumnsDir, "b00000000001", ColumnAnchor), "---\ntitle: Only\nkind: work\n---\n")
	opened, err := Open(tolerated)
	if err != nil {
		t.Fatalf("a workbench below the mandating major should open: %v", err)
	}
	if opened.Columns[0].Slug != "" {
		t.Errorf("the column carries slug %q, wanted none", opened.Columns[0].Slug)
	}

	anchor := map[string]string{"path": filepath.Join("columns", "b00000000001", ColumnAnchor)}
	refused := admitSlug(&Column{ID: "b00000000001"}, SlugMandatoryMajor, map[string]bool{}, anchor)
	if !refusedMalformed(refused) {
		t.Errorf("a missing slug at the mandating major should refuse malformed, got %v", refused)
	}
	assertNamesColumnAndFile(t, refused, "b00000000001", anchor["path"])
	if err := admitSlug(&Column{ID: "b00000000001"}, SlugMandatoryMajor-1, map[string]bool{}, anchor); err != nil {
		t.Errorf("a missing slug below the mandating major should open, got %v", err)
	}
}

// TestColumnByRefReadsTheSlugAheadOfTheTitle asserts the resolution order a
// reference takes: the identifier, then the slug, then the title. A reference
// matching one column's slug and another column's title resolves to the column
// whose slug it is, which is the tier that makes a spaced title reachable
// without quoting it.
func TestColumnByRefReadsTheSlugAheadOfTheTitle(t *testing.T) {
	root := newTwoColumnFixture(t, ProfileVersion, "agent-code-review", "review")
	opened, err := Open(root)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	// The second column is titled Review and the first is slugged review, so
	// the two tiers disagree and the slug tier is the one that answers.
	opened.Columns[0].Slug = "review"
	opened.Columns[1].Title = "Review"
	if column := opened.ColumnByRef("review"); column == nil || column.ID != "b00000000001" {
		t.Errorf("a slug should resolve ahead of another column's title, got %v", column)
	}
	if column := opened.ColumnByRef("b00000000002"); column == nil || column.ID != "b00000000002" {
		t.Errorf("an identifier should resolve ahead of everything, got %v", column)
	}
	opened.Columns[0].Slug = "agent-code-review"
	if column := opened.ColumnByRef("agent-code-review"); column == nil || column.ID != "b00000000001" {
		t.Error("a slug should resolve a column whose title needs quoting")
	}
	if column := opened.ColumnByRef("AGENT-CODE-REVIEW"); column == nil || column.ID != "b00000000001" {
		t.Error("a slug should resolve without regard to ASCII case")
	}
}

// TestCheckReportsWhatTheSlugMigrationRepairs asserts the checker and the
// repair together: a column with no slug is reported while the workbench
// declares a major below the mandating one, the migration derives a slug from
// each title and says which one it gave, a second run finds nothing left to do,
// and the workbench checks clean afterwards.
func TestCheckReportsWhatTheSlugMigrationRepairs(t *testing.T) {
	root := newFixture(t)
	write(t, filepath.Join(root, ColumnsDir, "b00000000001", ColumnAnchor), "---\ntitle: Agent Code Review\nkind: work\n---\nColumn text.\n")
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

	assigned, reported := opened.BackfillColumnSlugs()
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
	if got := reopened.Columns[0].Slug; got != "agent-code-review" {
		t.Errorf("the column anchor carries slug %q", got)
	}
	if body := readColumnAnchorText(t, root); !strings.Contains(body, "Column text.") {
		t.Errorf("the migration lost the column's own instructions:\n%s", body)
	}
	again, reported := reopened.BackfillColumnSlugs()
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

// TestTheMandatoryMajorIsComparedAgainstTheResolvedReading drives the slug
// check's threshold to two values this build's own constant does not carry, so
// that a correct wiring and each of the two wirings a mistake here would
// produce give different answers.
//
// At a threshold of 1 the two readings of dinah-core/1.0 disagree: the
// resolved reading is major 0 and fires, and the bare tokenizer's unaliased
// reading is major 1 and does not, so a check still reaching splitProfile
// reports nothing where a finding is wanted. At a threshold of 0 nothing
// should fire, since 0 is not below 0, so a body ignoring its own parameter
// and comparing against SlugMandatoryMajor reports a finding where none is
// wanted. Neither value alone separates the two, which is why both are driven.
//
// The third call declares 0.1 literally, the revision the alias resolves
// dinah-core/1.0 to, and asserts the same finding, so the two spellings are
// shown to compute one answer once the seam is wired.
func TestTheMandatoryMajorIsComparedAgainstTheResolvedReading(t *testing.T) {
	opened, err := Open(newTwoColumnFixture(t, ProfileVersion, "", "second"))
	if err != nil {
		t.Fatalf("a workbench carrying an absent slug should open: %v", err)
	}
	findings := opened.checkColumnSlugsWithin(1)
	if len(findings) != 1 || findings[0].Key != FindingSlugMissing {
		t.Fatalf("wanted one missing finding at a mandatory major of 1, got %v", findings)
	}
	if findings[0].Detail != "b00000000001" {
		t.Errorf("the finding names %q, wanted the column carrying no slug", findings[0].Detail)
	}
	if findings := opened.checkColumnSlugsWithin(0); len(findings) != 0 {
		t.Errorf("wanted no finding at a mandatory major of 0, got %v", findings)
	}

	// The literal spelling of the revision the alias resolves dinah-core/1.0
	// to sits below the floor dinah-287 raised, so no workbench declaring it
	// opens through Open any more. The declaration is put on the opened bench
	// instead, which is where the check reads it from and is therefore the
	// whole of the seam under test: a body still calling the bare tokenizer
	// reads major 1 here and reports nothing.
	literal, err := Open(newTwoColumnFixture(t, ProfileVersion, "", "second"))
	if err != nil {
		t.Fatalf("a workbench carrying an absent slug should open: %v", err)
	}
	literal.Profile = "dinah-core/0.1"
	findings = literal.checkColumnSlugsWithin(1)
	if len(findings) != 1 || findings[0].Key != FindingSlugMissing {
		t.Fatalf("wanted one missing finding for the literal 0.1, got %v", findings)
	}
	if findings[0].Detail != "b00000000001" {
		t.Errorf("the finding names %q, wanted the column carrying no slug", findings[0].Detail)
	}
}

// TestCheckReportsAStoredSlugItWillNotRepair asserts that a malformed or
// duplicated slug is reported and left alone. Either one is a value somebody
// wrote on purpose, so the checker names it for a person and the migration does
// not overwrite a decision it cannot read.
//
// The workbench is read off disk through Open, which is the point of the rule
// this test stands on: the checker only ever sees a workbench a reader agreed
// to open, so a reader refusing these two slugs would leave both findings
// unreachable and this test asserting nothing anybody can meet.
func TestCheckReportsAStoredSlugItWillNotRepair(t *testing.T) {
	root := newTwoColumnFixture(t, ProfileVersion, "review", "review")
	opened, err := Open(root)
	if err != nil {
		t.Fatalf("a workbench carrying a duplicated slug should open: %v", err)
	}
	findings := opened.checkColumnSlugs()
	if len(findings) != 1 || findings[0].Key != FindingSlugDuplicate {
		t.Fatalf("wanted one duplicate finding, got %v", findings)
	}
	if want := opened.ColumnAnchorPath("b00000000002"); findings[0].Path != want {
		t.Errorf("the finding names %q, wanted the second column's anchor %q", findings[0].Path, want)
	}

	malformed := newTwoColumnFixture(t, ProfileVersion, "Only", "second")
	stored, err := Open(malformed)
	if err != nil {
		t.Fatalf("a workbench carrying a malformed slug should open: %v", err)
	}
	findings = stored.checkColumnSlugs()
	if len(findings) != 1 || findings[0].Key != FindingSlugMalformed {
		t.Fatalf("wanted one malformed finding, got %v", findings)
	}
	if findings[0].Detail != "b00000000001" {
		t.Errorf("the finding names %q, wanted the column carrying the slug", findings[0].Detail)
	}
	if want := stored.ColumnAnchorPath("b00000000001"); findings[0].Path != want {
		t.Errorf("the finding names %q, wanted the column's anchor %q", findings[0].Path, want)
	}

	// The migration leaves both alone and repairs what it can around them,
	// which is the way out of a workbench carrying one.
	assigned, reported := stored.BackfillColumnSlugs()
	if len(assigned) != 0 || len(reported) != 0 {
		t.Errorf("the migration assigned %v and reported %v over slugs already stored", assigned, reported)
	}
	if got := stored.Columns[0].Slug; got != "Only" {
		t.Errorf("the migration overwrote a stored slug with %q", got)
	}
}

// TestTheSlugMigrationNamesWhatStoppedIt asserts that the two conditions only
// the run itself can know are filed under keys that describe them: a title no
// slug can be derived from, and a column anchor the run could not write to.
// Neither is a column carrying a malformed slug or a column carrying none, and
// reporting them under those keys would print a sentence about a slug's shape
// for a column that has no slug at all.
func TestTheSlugMigrationNamesWhatStoppedIt(t *testing.T) {
	root := newFixture(t)
	write(t, filepath.Join(root, ColumnsDir, "b00000000001", ColumnAnchor), "---\ntitle: 2026\nkind: work\n---\n")
	opened, err := Open(root)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	assigned, reported := opened.BackfillColumnSlugs()
	if len(assigned) != 0 {
		t.Errorf("the migration assigned %v for a title it cannot derive from", assigned)
	}
	if len(reported) != 1 || reported[0].Key != FindingSlugUnderivable {
		t.Fatalf("wanted one underivable finding, got %v", reported)
	}
	if reported[0].Detail != "b00000000001" {
		t.Errorf("the finding names %q, wanted the column", reported[0].Detail)
	}

	// The anchor goes after the workbench is open, which is the column a run
	// meets when something takes the file away underneath it.
	unwritable := newFixture(t)
	stripped, err := Open(unwritable)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	stripped.Columns[0].Slug = ""
	if err := os.Remove(stripped.ColumnAnchorPath("b00000000001")); err != nil {
		t.Fatalf("remove anchor: %v", err)
	}
	assigned, reported = stripped.BackfillColumnSlugs()
	if len(assigned) != 0 {
		t.Errorf("the migration assigned %v over an anchor it cannot write", assigned)
	}
	if len(reported) != 1 || reported[0].Key != FindingSlugUnwritable {
		t.Fatalf("wanted one unwritable finding, got %v", reported)
	}
}

// TestTheMigrationWritesTheSlugWhereTheWriterPutsIt asserts that a migrated
// anchor and a newly written one carry the slug in the same place, so two
// anchors of one workbench do not differ by which code path wrote them.
func TestTheMigrationWritesTheSlugWhereTheWriterPutsIt(t *testing.T) {
	root := newFixture(t)
	write(t, filepath.Join(root, ColumnsDir, "b00000000001", ColumnAnchor), "---\ntitle: Agent Code Review\nkind: work\noperator_owned: true\n---\nColumn text.\n")
	opened, err := Open(root)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if _, reported := opened.BackfillColumnSlugs(); len(reported) != 0 {
		t.Fatalf("the migration reported %v", reported)
	}
	keys := anchorKeys(readColumnAnchorText(t, root))
	wanted := []string{"title", "slug", "kind", "operator_owned"}
	if strings.Join(keys, ",") != strings.Join(wanted, ",") {
		t.Errorf("the migrated anchor reads %v, wanted %v", keys, wanted)
	}
}

// anchorKeys returns the frontmatter keys of an anchor in the order they stand
// in the file, which is what a person reading two anchors side by side sees.
func anchorKeys(text string) []string {
	fm, _ := ParseAnchor(text)
	return fm.Keys()
}

// TestInstantiateDerivesASlugForEveryColumn asserts CORE-JSON-9 and
// CORE-STATE-10 over the interchange form: a column object carrying no slug
// member is given one derived from its title, two columns of one title take the
// derived name and the first free suffix in array order, and an explicit slug
// is taken as given.
func TestInstantiateDerivesASlugForEveryColumn(t *testing.T) {
	definition := readDefinition(t, `{
	  "profile": "dinah-core/0.7",
	  "title": "Fixture",
	  "columns": [
	    {"id": "b00000000001", "title": "Agent Code Review", "kind": "intake"},
	    {"id": "b00000000002", "title": "Review", "kind": "work"},
	    {"id": "b00000000003", "title": "Review", "kind": "work"},
	    {"id": "b00000000004", "title": "Finished", "kind": "done", "slug": "done"}
	  ]
	}`)
	root := containedPath(filepath.Join(t.TempDir(), "workbench"))
	if err := Instantiate(root, "fx", "alka", definition); err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	opened, err := Open(root)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	wanted := []string{"agent-code-review", "review", "review-2", "done"}
	for position, want := range wanted {
		if got := opened.Columns[position].Slug; got != want {
			t.Errorf("column %d carries slug %q, wanted %q", position+1, got, want)
		}
	}
}

// TestInstantiateRefusesASlugItCannotHonour asserts the two malformed cases at
// the write surface: an explicit slug outside the grammar or already asked for
// by another column, and a title deriving to nothing a slug can be made of.
func TestInstantiateRefusesASlugItCannotHonour(t *testing.T) {
	cases := []struct {
		name    string
		columns string
		// detail is what the refusal names, which is the value the author has
		// to change. A title deriving to nothing never became a slug, so a
		// refusal calling it one sends the reader looking for a field the
		// definition does not carry.
		detail string
	}{
		{
			name:    "an explicit slug outside the grammar",
			columns: `{"id": "b00000000001", "title": "One", "kind": "work", "slug": "Not A Slug"}`,
			detail:  "slug Not A Slug",
		},
		{
			name:    "two explicit slugs asking for one value",
			columns: `{"id": "b00000000001", "title": "One", "kind": "work", "slug": "review"}, {"id": "b00000000002", "title": "Two", "kind": "work", "slug": "review"}`,
			detail:  "slug review",
		},
		{
			name:    "a title no slug can be derived from",
			columns: `{"id": "b00000000001", "title": "---", "kind": "work"}`,
			detail:  "title ---",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			definition := readDefinition(t, `{"profile": "dinah-core/0.7", "title": "Fixture", "columns": [`+c.columns+`]}`)
			root := containedPath(filepath.Join(t.TempDir(), "workbench"))
			err := Instantiate(root, "fx", "alka", definition)
			if !refusedMalformed(err) {
				t.Fatalf("wanted malformed, got %v", err)
			}
			var refusal *contract.Refusal
			if errors.As(err, &refusal) && refusal.Detail != c.detail {
				t.Errorf("the refusal names %q, wanted %q", refusal.Detail, c.detail)
			}
		})
	}
}

// TestTheInterchangeFormCarriesTheSlug asserts CORE-JSON-9: a column object
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
		t.Errorf("the exported column carries no slug:\n%s", encoded)
	}
	definition, err := ReadDefinition(encoded)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	second := containedPath(filepath.Join(t.TempDir(), "workbench"))
	if err := Instantiate(second, "fx", "alka", definition); err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	reopened, err := Open(second)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	if got := reopened.Columns[0].Slug; got != "only" {
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

// readColumnAnchorText reads the anchor of the fixture's first column.
func readColumnAnchorText(t *testing.T, root string) string {
	t.Helper()
	text, err := ReadText(filepath.Join(root, ColumnsDir, "b00000000001", ColumnAnchor))
	if err != nil {
		t.Fatalf("read anchor: %v", err)
	}
	return text
}

// newTwoColumnFixture writes a workbench of two columns carrying the slugs a
// case needs, which is the smallest shape the uniqueness and precedence rules
// can be driven over.
func newTwoColumnFixture(t *testing.T, profile, first, second string) string {
	t.Helper()
	root := t.TempDir()
	fm := NewFrontmatter()
	fm.Set("format", "1")
	fm.Set("profile", profile)
	fm.Set("title", "Fixture")
	fm.Set("slug", "fx")
	fm.Set("operator", "alka")
	fm.SetSeq("columns", []string{"b00000000001", "b00000000002"})
	write(t, filepath.Join(root, WorkbenchAnchor), fm.Render("Standing text.\n"))
	write(t, filepath.Join(root, ColumnsDir, "b00000000001", ColumnAnchor), columnAnchor("One", first, "intake"))
	write(t, filepath.Join(root, ColumnsDir, "b00000000002", ColumnAnchor), columnAnchor("Two", second, "work"))
	return root
}

// columnAnchor renders one column anchor for a fixture.
func columnAnchor(title, slug, kind string) string {
	fm := NewFrontmatter()
	fm.Set("title", title)
	if slug != "" {
		fm.Set(SlugField, slug)
	}
	fm.Set("kind", kind)
	return fm.Render("Column text.\n")
}

// refusedMalformed reports whether an error is the profile's malformed
// refusal, which is the one answer every rule in this file refuses with.
func refusedMalformed(err error) bool {
	var refusal *contract.Refusal
	return errors.As(err, &refusal) && refusal.Name == contract.Malformed
}
