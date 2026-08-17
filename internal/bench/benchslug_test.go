package bench

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

// TestCheckReportsAndRepairsAMissingWorkbenchSlug asserts the workbench-slug
// side of the migration: check reports a finding for a workbench written
// before the field existed, the migration derives one from the title and
// writes it, a second run finds nothing left to do, and a workbench that
// already carries a slug is untouched by either.
func TestCheckReportsAndRepairsAMissingWorkbenchSlug(t *testing.T) {
	root := newFixture(t)
	editWorkbench(t, root, "slug: fx\n", "")
	opened, err := Open(root)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	findings, err := opened.Check()
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if !hasFinding(findings, FindingWorkbenchSlugMissing) {
		t.Fatalf("wanted a workbench-slug-missing finding, got %v", findings)
	}

	assigned, reported, err := opened.BackfillWorkbenchSlug()
	if err != nil {
		t.Fatalf("backfill: %v", err)
	}
	if len(reported) != 0 {
		t.Errorf("the migration reported %v", reported)
	}
	if assigned == nil || assigned.Slug != "fixture" {
		t.Fatalf("the migration assigned %v, wanted slug fixture derived from the title", assigned)
	}

	reopened, err := Open(root)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	if got := reopened.Slug; got != "fixture" {
		t.Errorf("the workbench anchor carries slug %q", got)
	}

	again, reported, err := reopened.BackfillWorkbenchSlug()
	if err != nil {
		t.Fatalf("second backfill: %v", err)
	}
	if again != nil || len(reported) != 0 {
		t.Errorf("a second run assigned %v and reported %v", again, reported)
	}
	findings, err = reopened.Check()
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if hasFinding(findings, FindingWorkbenchSlugMissing) {
		t.Errorf("a migrated workbench should carry no workbench-slug-missing finding, got %v", findings)
	}
}

// TestBackfillWorkbenchSlugLeavesAStoredSlugAlone asserts that a workbench
// already carrying a slug is left exactly as it stands, malformed or not, the
// same rule BackfillStateSlugs holds to for a state.
func TestBackfillWorkbenchSlugLeavesAStoredSlugAlone(t *testing.T) {
	root := newFixture(t)
	opened, err := Open(root)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	assigned, reported, err := opened.BackfillWorkbenchSlug()
	if err != nil {
		t.Fatalf("backfill: %v", err)
	}
	if assigned != nil || len(reported) != 0 {
		t.Errorf("the migration touched a workbench that already carries a slug: assigned %v, reported %v", assigned, reported)
	}
	if got := opened.Slug; got != "fx" {
		t.Errorf("the migration changed the stored slug to %q", got)
	}
}

// TestBackfillWorkbenchSlugNamesAnUnderivableTitle asserts that a title no
// slug can be derived from is reported under its own key and the migration
// gives the workbench no slug, mirroring FindingSlugUnderivable for a state.
func TestBackfillWorkbenchSlugNamesAnUnderivableTitle(t *testing.T) {
	root := newFixture(t)
	editWorkbench(t, root, "title: Fixture\nslug: fx\n", "title: 2026\n")
	opened, err := Open(root)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	assigned, reported, err := opened.BackfillWorkbenchSlug()
	if err != nil {
		t.Fatalf("backfill: %v", err)
	}
	if assigned != nil {
		t.Errorf("the migration assigned %v for a title it cannot derive from", assigned)
	}
	if !hasFinding(reported, FindingWorkbenchSlugUnderivable) {
		t.Fatalf("wanted an underivable finding, got %v", reported)
	}
}

// hasFinding reports whether a findings slice carries one of a given key.
func hasFinding(findings []Finding, key string) bool {
	for _, finding := range findings {
		if finding.Key == key {
			return true
		}
	}
	return false
}

// TestCandidateSlugOmitsWhenAbsent asserts AC-5: the JSON view of a
// workbench carrying no slug omits the key entirely rather than serving an
// empty string, matching the convention StateView.Slug already carries.
func TestCandidateSlugOmitsWhenAbsent(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, WorkbenchAnchor), "---\nformat: 1\nprofile: dinah-core/1.0\ntitle: NoSlug\noperator: alka\nstates:\n  - b00000000001\n---\n")
	candidate := describe(root)
	if candidate.Slug != "" {
		t.Fatalf("wanted no slug, got %q", candidate.Slug)
	}
	data, err := json.Marshal(candidate)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(data), `"slug"`) {
		t.Errorf("the encoded candidate carries a slug key for a workbench with none: %s", data)
	}

	withSlug := describe(newFixture(t))
	data, err = json.Marshal(withSlug)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(data), `"slug":"fx"`) {
		t.Errorf("the encoded candidate should carry its slug: %s", data)
	}
}
