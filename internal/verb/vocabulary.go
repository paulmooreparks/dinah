package verb

import (
	"dinah/internal/bench"
	"dinah/internal/contract"
)

// TreeVocabularyReport is what a tree-wide dinah check --migrate-vocabulary
// run returns: every candidate the walk found, sorted into exactly one of
// these five outcomes, so a caller who reads only the counts still knows
// nothing was silently skipped.
type TreeVocabularyReport struct {
	// Outcome is contract.ReadFindings when Clean reports false and
	// contract.ReadOK when it reports true, on the terms CheckReport carries
	// the same member: always present, so a caller reads one string rather
	// than reconstructing Clean's own rule from the five collections below
	// (dinah-346).
	Outcome string `json:"outcome"`
	// Preview says the run wrote nothing and that Migrated names what a run
	// carrying the confirmation would rewrite. A reader that acts on the
	// counts alone would otherwise take a preview for a migration, which is
	// the one misreading this report must not allow.
	Preview        bool                    `json:"preview,omitempty"`
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
// bench.Enumerate alone answers the root and everything beneath it. The walk
// used to be two calls, one asking the root whether it was itself a workbench
// and one listing what sat below it, and the two disagreed about whether the
// root's own .dinah counted: neither of them opened it, so a workbench living
// there, which is where dinah init writes one, was invisible to both halves of
// the union and the command reported an empty tree over a board that was
// sitting in front of it (dinah-312).
//
// A failure on one workbench does not end the walk. The operator's own boards
// sit spread across customer directories, and one unwritable anchor should not
// cost him the migration of every board after it in the walk order. The report
// names what failed and the caller's exit code carries that outward, which is
// what keeps an unattended run from being read as a clean pass.
//
// apply is what separates a preview from a migration, and false is the
// default the command gives it. The walk's own reach is the reason: the root
// is a directory rather than a workbench, so a run started from a home
// directory or a drive root reaches every board on the machine, and the
// rewrite it performs has no undo behind it. A preview classifies every
// candidate exactly as a migration does and writes nothing, so the operator
// reads the blast radius off the report before he authorizes it.
func MigrateVocabularyTree(root string, apply bool) (*TreeVocabularyReport, error) {
	report := &TreeVocabularyReport{Preview: !apply, Migrated: []TreeVocabularyEntry{}}
	candidates, err := vocabularyCandidates(root)
	if err != nil {
		return report, err
	}
	for _, path := range candidates {
		migrateOneVocabulary(report, path, apply)
	}
	report.Outcome = contract.ReadOK
	if !report.Clean() {
		report.Outcome = contract.ReadFindings
	}
	return report, nil
}

// vocabularyCandidates answers every directory at or beneath the root that
// carries a workbench, including the root itself, with no directory named
// twice.
//
// The walk is bench.ScanContainers rather than bench.Enumerate, and dinah-285
// is why. Enumerate answers through the discovery walk, which since the
// containment rule landed no longer returns a bare workbench, and a workbench
// written in the retired vocabulary is old enough to be exactly the one likely
// to be sitting bare. Running this sweep through discovery would therefore have hidden
// the workbenches it exists for. What Enumerate was chosen for still holds and
// this walk keeps it: one function answers the root and everything beneath it,
// so the root's own .dinah cannot go unopened while every other directory's is
// opened (dinah-312).
func vocabularyCandidates(root string) ([]string, error) {
	listed, err := bench.ScanContainers(root)
	if err != nil {
		return nil, err
	}
	candidates := make([]string, 0, len(listed))
	for _, candidate := range listed {
		candidates = append(candidates, candidate.Path)
	}
	return candidates, nil
}

// migrateOneVocabulary classifies one candidate and sorts it into the report,
// migrating it when its declared revision sits inside the pre-vocabulary
// window and apply is set, and leaving it alone otherwise.
//
// A preview stops at the classification and does not open the workbench.
// Opening is a read, and a preview that opened would report a workbench the
// migration could not open as a failure rather than as a candidate, which is
// better information. It would also be the one place in this command where a
// run the operator did not confirm reaches inside a workbench at all, and a
// preview whose reach a reader has to reason about is worth less than one
// whose reach is nothing. The real run reports what it meets.
func migrateOneVocabulary(report *TreeVocabularyReport, path string, apply bool) {
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
	case bench.WithinPreVocabulary(major, minor) && !apply:
		report.Migrated = append(report.Migrated, TreeVocabularyEntry{Path: path, Revision: revision})
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

// Clean reports whether the run needs a person, which is the question the
// command's exit code answers. A workbench the walk failed on needs one, and so
// does a candidate it could not classify. So does a preview that found a
// workbench still to carry forward, because the migration has not happened and
// the confirmation that would perform it is a person's to give: an unattended
// run reporting zero and exiting zero, having written nothing and left every
// board unmigrated, is the reading this whole report exists to prevent.
//
// A run that migrated nothing is clean, and that is deliberate rather than an
// oversight. Every other repair flag on this command exits zero against a
// workbench with nothing to repair, an unattended sweep over a tree already
// carried across the rename is the ordinary case rather than a mistake, and an
// exit code that called it a failure would train a reader to ignore the one
// this function exists to raise. The report still says what happened: a run
// that carried nothing forward prints a count of zero and names every
// workbench that already declares the current format, so nothing here reports
// work that did not happen.
func (r *TreeVocabularyReport) Clean() bool {
	if r.Preview && len(r.Migrated) > 0 {
		return false
	}
	return len(r.Failed) == 0 && len(r.Malformed) == 0
}
