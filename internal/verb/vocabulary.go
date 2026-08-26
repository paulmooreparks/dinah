package verb

import (
	"dinah/internal/bench"
)

// TreeVocabularyReport is what a tree-wide dinah check --migrate-vocabulary
// run returns: every candidate the walk found, sorted into exactly one of
// these five outcomes, so a caller who reads only the counts still knows
// nothing was silently skipped.
type TreeVocabularyReport struct {
	Migrated       []TreeVocabularyEntry   `json:"migrated"`
	AlreadyCurrent []string                `json:"already_current,omitempty"`
	Unsupported    []TreeVocabularyEntry   `json:"unsupported,omitempty"`
	Malformed      []string                `json:"malformed,omitempty"`
	Failed         []TreeVocabularyFailure `json:"failed,omitempty"`
}

// TreeVocabularyEntry is one workbench the walk acted on or declined to act
// on, carrying the revision it declared and, where the migration ran, how many
// cards it rewrote.
type TreeVocabularyEntry struct {
	Path     string `json:"path"`
	Revision string `json:"revision,omitempty"`
	Cards    int    `json:"cards,omitempty"`
}

// TreeVocabularyFailure is one workbench the walk could not carry forward, and
// the reason it could not.
type TreeVocabularyFailure struct {
	Path   string `json:"path"`
	Reason string `json:"reason"`
}

// MigrateVocabularyTree carries every workbench at or beneath a root from the
// retired state/substate vocabulary to the current column/state one.
//
// The walk is deliberately two calls rather than one. bench.Enumerate reads a
// root's entries and tests each child, so a root that is itself directly a
// workbench is invisible to it; bench.RecognizedAt asks the same question of
// the root itself, which is what the ordinary discovery climb already does at
// its first rung. Taking the union of the two is what makes a single workbench
// a tree of size one rather than an empty tree.
//
// A failure on one workbench does not end the walk. The operator's own boards
// sit spread across customer directories, and one unwritable anchor should not
// cost him the migration of every board after it in the walk order. The report
// names what failed and the caller's exit code carries that outward, which is
// what keeps an unattended run from being read as a clean pass.
func MigrateVocabularyTree(root string) (*TreeVocabularyReport, error) {
	report := &TreeVocabularyReport{Migrated: []TreeVocabularyEntry{}}
	candidates, err := vocabularyCandidates(root)
	if err != nil {
		return report, err
	}
	for _, path := range candidates {
		migrateOneVocabulary(report, path)
	}
	return report, nil
}

// vocabularyCandidates answers every directory at or beneath the root that
// offers a workbench, the root itself first and in walk order after that, with
// no directory named twice.
func vocabularyCandidates(root string) ([]string, error) {
	var candidates []string
	seen := map[string]bool{}
	add := func(path string) {
		if path == "" || seen[path] {
			return
		}
		seen[path] = true
		candidates = append(candidates, path)
	}
	found, err := bench.RecognizedAt(root)
	if err != nil {
		return nil, err
	}
	add(found)
	listed, err := bench.Enumerate(root)
	if err != nil {
		return nil, err
	}
	for _, candidate := range listed {
		add(candidate.Path)
	}
	return candidates, nil
}

// migrateOneVocabulary classifies one candidate and sorts it into the report,
// migrating it when its declared revision sits inside the pre-vocabulary
// window and leaving it alone otherwise.
func migrateOneVocabulary(report *TreeVocabularyReport, path string) {
	major, minor, ok, err := bench.ClassifyVocabulary(path)
	if err != nil {
		report.Failed = append(report.Failed, TreeVocabularyFailure{Path: path, Reason: err.Error()})
		return
	}
	if !ok {
		report.Malformed = append(report.Malformed, path)
		return
	}
	revision := bench.RevisionText(major, minor)
	switch {
	case bench.WithinPreVocabulary(major, minor):
		opened, err := bench.OpenPreVocabulary(path)
		if err != nil {
			report.Failed = append(report.Failed, TreeVocabularyFailure{Path: path, Reason: err.Error()})
			return
		}
		cards, err := bench.MigrateVocabulary(opened)
		if err != nil {
			report.Failed = append(report.Failed, TreeVocabularyFailure{Path: path, Reason: err.Error()})
			return
		}
		report.Migrated = append(report.Migrated, TreeVocabularyEntry{Path: path, Revision: revision, Cards: len(cards)})
	case bench.WithinCurrent(major, minor):
		report.AlreadyCurrent = append(report.AlreadyCurrent, path)
	default:
		report.Unsupported = append(report.Unsupported, TreeVocabularyEntry{Path: path, Revision: revision})
	}
}

// Clean reports whether the run needs a person. A failed or malformed
// workbench needs one, and so does a run that migrated nothing at all, since
// asking for a migration and getting none is an answer rather than a success.
func (r *TreeVocabularyReport) Clean() bool {
	return len(r.Failed) == 0 && len(r.Malformed) == 0 && len(r.Migrated) > 0
}
