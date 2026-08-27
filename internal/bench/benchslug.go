package bench

import "path/filepath"

// checkWorkbenchSlug applies the slug invariant to the workbench's own
// identity. The workbench carries a slug, and the slug it carries conforms to
// the grammar, so that the checker speaks about a hand-edited anchor as well
// as about one written before the field existed. Unlike checkColumnSlugs, this carries no major
// gate: no CORE-BENCH statement makes a workbench slug mandatory at any
// major the way CORE-STATE-10 does for a column, and nothing in Open refuses
// a workbench for lacking one at any major, so there is no threshold past
// which this check would be reporting a condition Open has already turned
// into a refusal. The finding is informational on every conforming major
// this binary opens, and stays that way.
func (b *Bench) checkWorkbenchSlug() []Finding {
	anchor := filepath.Join(b.Root, WorkbenchAnchor)
	if b.Slug == "" {
		return []Finding{{Path: anchor, Key: FindingWorkbenchSlugMissing}}
	}
	if !ValidSlug(b.Slug) {
		return []Finding{{Path: anchor, Key: FindingWorkbenchSlugMalformed, Detail: b.Slug}}
	}
	return nil
}

// WorkbenchSlugAssignment is what the workbench-slug migration reports when
// it derives one, named the way SlugAssignment is for a column.
type WorkbenchSlugAssignment struct {
	// Title is the workbench's title, which is what the slug was derived
	// from.
	Title string `json:"title"`
	// Slug is the slug written to the workbench's anchor.
	Slug string `json:"slug"`
}

// BackfillWorkbenchSlug derives a slug for the workbench when it carries
// none, writes it via Save, and reports what it derived. A slug already on
// disk is left untouched, malformed or not, for the same reason
// BackfillColumnSlugs leaves a stored column slug alone: somebody chose that
// value on purpose. A title deriving nothing usable is reported and the
// caller carries on rather than the migration stopping outright, matching
// BackfillColumnSlugs' own report-and-continue behavior.
//
// Unlike BackfillColumnSlugs, there is no collision to resolve here:
// workbench-slug uniqueness is not a resolution concern anywhere in this
// codebase today.
func (b *Bench) BackfillWorkbenchSlug() (*WorkbenchSlugAssignment, []Finding, error) {
	if b.Slug != "" {
		return nil, nil, nil
	}
	path := filepath.Join(b.Root, WorkbenchAnchor)
	derived := Slugify(b.Title)
	if derived == "" {
		return nil, []Finding{{Path: path, Key: FindingWorkbenchSlugUnderivable}}, nil
	}
	b.Slug = derived
	if err := b.Save(); err != nil {
		return nil, nil, err
	}
	return &WorkbenchSlugAssignment{Title: b.Title, Slug: derived}, nil, nil
}
