package bench

import (
	"path/filepath"
	"strings"

	"dinah/internal/contract"
)

// RemoveColumnID drops one identifier from the workbench's own ordered
// columns list and from every declared route, and writes the anchor back in
// one write, preserving every other key exactly as Save does. It is
// retirement's own write to the definition, made while the bench lock the
// retiring act already holds is still in force, so the single authority for
// order (format.md, "Flow definition") never names a column whose directory
// the same act just moved or removed.
//
// The routes move with the columns list because the anchor must never name a
// column whose directory is not there, and a route naming one would be exactly
// that. Removing an id already absent from both is a no-op, which is what makes
// a second call over the same bench safe.
func (b *Bench) RemoveColumnID(id string) error {
	ids := b.FM.Seq("columns")
	kept := make([]string, 0, len(ids))
	for _, existing := range ids {
		if existing != id {
			kept = append(kept, existing)
		}
	}
	routed := b.RemoveColumnIDFromRoutes(id)
	if len(kept) == len(ids) && !routed {
		return nil
	}
	b.FM.SetSeq("columns", kept)
	return WriteText(filepath.Join(b.Root, WorkbenchAnchor), b.FM.Render(b.Standing))
}

// AddColumnID appends one identifier to the workbench's own ordered columns
// list and to no route, and writes the anchor back, preserving every other key
// exactly as RemoveColumnID does. It is restoration's own write to the
// definition, made while the bench lock the restoring act already holds is
// still in force. Adding an id already present is a no-op, which is what makes
// a second call over the same bench safe.
//
// A route is a choice somebody made, and restoring a column is not evidence
// about which routes wanted it, which is the argument reshape already makes for
// dropping a tier override rather than guessing one. A route that lost a column
// to a retirement and wants it back gains it by an edit.
func (b *Bench) AddColumnID(id string) error {
	ids := b.FM.Seq("columns")
	for _, existing := range ids {
		if existing == id {
			return nil
		}
	}
	b.FM.SetSeq("columns", append(append([]string{}, ids...), id))
	return WriteText(filepath.Join(b.Root, WorkbenchAnchor), b.FM.Render(b.Standing))
}

// RemoveStrandedColumns drops every stranded identifier from the workbench's
// own ordered columns list and from every declared route in one write, and
// returns what it removed from the columns list.
// Unlike a slug backfill there is no title or further metadata to report:
// the column's own directory is exactly what is missing.
//
// Removing every stranded id sometimes leaves the columns list with none at
// all, which CORE-BENCH-2 forbids the workbench from ending up with. When
// that would happen, RemoveStrandedColumns refuses with
// contract.RepairWouldEmptyColumns instead of writing, and leaves
// StrandedColumns and the file exactly as they were, so a following check
// still reports the same findings it reported before the refusal.
func (b *Bench) RemoveStrandedColumns() ([]string, error) {
	if len(b.StrandedColumns) == 0 {
		return nil, nil
	}
	ids := b.FM.Seq("columns")
	stranded := map[string]bool{}
	for _, id := range b.StrandedColumns {
		stranded[id] = true
	}
	kept := make([]string, 0, len(ids))
	for _, existing := range ids {
		if !stranded[existing] {
			kept = append(kept, existing)
		}
	}
	if len(kept) == 0 {
		return nil, contract.Refuse(contract.RepairWouldEmptyColumns, strings.Join(ids, ", "))
	}
	// A repair that would empty a route is not refused, because a workbench
	// keeping its columns is the invariant and an empty route is a finding.
	b.removeFromRoutes(stranded)
	b.FM.SetSeq("columns", kept)
	if err := WriteText(filepath.Join(b.Root, WorkbenchAnchor), b.FM.Render(b.Standing)); err != nil {
		return nil, err
	}
	removed := append([]string{}, b.StrandedColumns...)
	b.StrandedColumns = nil
	return removed, nil
}
