package bench

import (
	"path/filepath"
	"strings"

	"dinah/internal/contract"
)

// RemoveStateID drops one identifier from the workbench's own ordered
// states list and writes the anchor back, preserving every other key
// exactly as Save does. It is retirement's own write to the definition,
// made while the bench lock the retiring act already holds is still in
// force, so the single authority for order (format.md, "Flow definition")
// never names a state whose directory the same act just moved or removed.
// Removing an id already absent is a no-op, which is what makes a second
// call over the same bench safe.
func (b *Bench) RemoveStateID(id string) error {
	ids := b.FM.Seq("states")
	kept := make([]string, 0, len(ids))
	for _, existing := range ids {
		if existing != id {
			kept = append(kept, existing)
		}
	}
	if len(kept) == len(ids) {
		return nil
	}
	b.FM.SetSeq("states", kept)
	return WriteText(filepath.Join(b.Root, WorkbenchAnchor), b.FM.Render(b.Standing))
}

// RemoveStrandedStates drops every stranded identifier from the workbench's
// own ordered states list in one write, and returns what it removed.
// Unlike a slug backfill there is no title or further metadata to report:
// the state's own directory is exactly what is missing.
//
// Removing every stranded id sometimes leaves the states list with none at
// all, which CORE-BENCH-2 forbids the workbench from ending up with. When
// that would happen, RemoveStrandedStates refuses with
// contract.RepairWouldEmptyStates instead of writing, and leaves
// StrandedStates and the file exactly as they were, so a following check
// still reports the same findings it reported before the refusal.
func (b *Bench) RemoveStrandedStates() ([]string, error) {
	if len(b.StrandedStates) == 0 {
		return nil, nil
	}
	ids := b.FM.Seq("states")
	stranded := map[string]bool{}
	for _, id := range b.StrandedStates {
		stranded[id] = true
	}
	kept := make([]string, 0, len(ids))
	for _, existing := range ids {
		if !stranded[existing] {
			kept = append(kept, existing)
		}
	}
	if len(kept) == 0 {
		return nil, contract.Refuse(contract.RepairWouldEmptyStates, strings.Join(ids, ", "))
	}
	b.FM.SetSeq("states", kept)
	if err := WriteText(filepath.Join(b.Root, WorkbenchAnchor), b.FM.Render(b.Standing)); err != nil {
		return nil, err
	}
	removed := append([]string{}, b.StrandedStates...)
	b.StrandedStates = nil
	return removed, nil
}
