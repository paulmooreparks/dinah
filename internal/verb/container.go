package verb

import (
	"path/filepath"
	"sort"
	"strings"

	"dinah/internal/bench"
	"dinah/internal/contract"
)

// TreeContainerReport is what a tree-wide dinah check --migrate-container run
// returns: every workbench the sweep found, sorted into exactly one of these
// collections, so a caller reading only the counts still knows nothing was
// silently skipped.
//
// It is shaped after TreeVocabularyReport rather than invented beside it,
// because the two commands answer the same question about different repairs
// and a reader who has learned one report should not have to learn a second.
type TreeContainerReport struct {
	// Outcome is contract.ReadFindings when Clean reports false and
	// contract.ReadOK when it reports true, which is the member every report
	// on this surface carries so that a caller reads one string rather than
	// reconstructing Clean's own rule.
	Outcome string `json:"outcome"`
	// Preview says the run wrote nothing and that Migrated names what a run
	// carrying the confirmation would move. A reader acting on the counts
	// alone would otherwise take a preview for a migration.
	Preview bool `json:"preview,omitempty"`
	// Migrated is every workbench the run carried into a container, or would
	// carry, each naming where it came from and where it went.
	Migrated []ContainerEntry `json:"migrated"`
	// AlreadyContained is every workbench already sitting where the rule puts
	// one, under a name this build minted.
	AlreadyContained []string `json:"already_contained,omitempty"`
	// Duplicates is every identifier more than one directory claims, keyed on
	// the identifier and carrying the paths claiming it. Nothing is done to
	// any of them.
	Duplicates map[string][]string `json:"duplicates,omitempty"`
	// Failed is every workbench the sweep could not carry forward, and why.
	Failed []TreeContainerFailure `json:"failed,omitempty"`
}

// ContainerEntry is one workbench the sweep acted on, or would act on: the
// directory it sat in, the directory it now sits in, and the shape it was
// found in. To is empty on a preview, which has moved nothing and cannot
// honestly name a destination, since the identifier is minted at the moment of
// the move.
type ContainerEntry struct {
	Path  string `json:"path"`
	To    string `json:"to,omitempty"`
	Shape string `json:"shape"`
}

// TreeContainerFailure is one workbench the sweep could not carry forward, and
// the reason it could not.
type TreeContainerFailure struct {
	Path   string `json:"path"`
	Shape  string `json:"shape"`
	Reason string `json:"reason"`
}

// RemintReport is what dinah check --remint answers with: the one directory it
// was given and the one it renamed that directory to.
type RemintReport struct {
	Outcome string `json:"outcome"`
	From    string `json:"from"`
	To      string `json:"to"`
	ID      string `json:"id"`
}

// MigrateContainerTree carries every workbench at or beneath a root into the
// shape the containment rule fixes: a directory named by an identifier Dinah
// minted, sitting immediately inside a .dinah container.
//
// The sweep runs bench.ScanContainers rather than bench.Enumerate, and the
// reason is the whole point of the command. Enumerate answers through the
// discovery walk, which no longer returns a bare workbench and has never been
// able to see a container subdirectory whose name is neither width, so a sweep
// built on it would be blind to two of the three shapes it exists to repair.
//
// A failure on one workbench does not end the walk. The operator's own boards
// sit spread across customer directories, and one workbench somebody has open
// should not cost him the migration of every board after it in the walk order.
//
// apply is what separates a preview from a migration, and false is the default
// the command gives it, for the reason MigrateVocabularyTree states: the root
// is a directory rather than a workbench, so a run started from a home
// directory reaches every board on the machine, and this repair moves
// directories. A preview classifies every workbench exactly as a migration
// does and writes nothing.
func MigrateContainerTree(root string, apply bool) (*TreeContainerReport, error) {
	report := &TreeContainerReport{Preview: !apply, Migrated: []ContainerEntry{}}
	candidates, err := bench.ScanContainers(root)
	if err != nil {
		return report, err
	}
	duplicates := bench.DuplicateWorkbenchIDs(candidates)
	if len(duplicates) > 0 {
		report.Duplicates = duplicates
	}
	held := heldByDuplicate(duplicates)
	for _, candidate := range candidates {
		migrateOneContainer(report, candidate, apply, held)
	}
	report.Outcome = contract.ReadOK
	if !report.Clean() {
		report.Outcome = contract.ReadFindings
	}
	return report, nil
}

// heldByDuplicate is the set of directories a duplicate report names, which
// the sweep leaves alone whatever shape they are in. Renaming one of two
// directories claiming an identifier is exactly the choice this command
// refuses to make on its own.
func heldByDuplicate(duplicates map[string][]string) map[string]bool {
	held := map[string]bool{}
	for _, paths := range duplicates {
		for _, path := range paths {
			held[path] = true
		}
	}
	return held
}

// migrateOneContainer classifies one workbench and sorts it into the report,
// carrying it into a container when its shape asks for that and apply is set.
func migrateOneContainer(report *TreeContainerReport, candidate bench.ContainerCandidate, apply bool, held map[string]bool) {
	shape := string(candidate.Shape)
	if candidate.Shape == bench.ShapeContained {
		report.AlreadyContained = append(report.AlreadyContained, candidate.Path)
		return
	}
	if held[candidate.Path] {
		return
	}
	if !apply {
		report.Migrated = append(report.Migrated, ContainerEntry{Path: candidate.Path, Shape: shape})
		return
	}
	moved, err := bench.MigrateContainer(candidate.Path)
	if err != nil {
		failure := TreeContainerFailure{Path: candidate.Path, Shape: shape, Reason: err.Error()}
		report.Failed = append(report.Failed, failure)
		return
	}
	report.Migrated = append(report.Migrated, ContainerEntry{Path: candidate.Path, To: moved, Shape: shape})
}

// Clean reports whether the run needs a person, which is the question the
// command's exit code answers. A workbench the sweep failed on needs one, and
// so does a duplicated identifier, which this command deliberately does not
// repair and which therefore stays true after the run. So does a preview that
// found a workbench still to carry forward, because the migration has not
// happened and the confirmation that would perform it is a person's to give.
//
// A run that migrated nothing is clean, on the terms TreeVocabularyReport.Clean
// already sets out: an unattended sweep over a tree already carried forward is
// the ordinary case rather than a mistake.
func (r *TreeContainerReport) Clean() bool {
	if r.Preview && len(r.Migrated) > 0 {
		return false
	}
	return len(r.Failed) == 0 && len(r.Duplicates) == 0
}

// BareWorkbenchFindings renders the bare workbenches a sweep found as check
// findings, so the one rendering path dinah check already has covers them.
func (r *TreeContainerReport) BareWorkbenchFindings() []bench.Finding {
	var findings []bench.Finding
	for _, entry := range r.Migrated {
		if entry.Shape != string(bench.ShapeBare) {
			continue
		}
		path := filepath.Join(entry.Path, bench.WorkbenchAnchor)
		findings = append(findings, bench.Finding{Path: path, Key: bench.FindingBareWorkbench, Detail: entry.Path})
	}
	return findings
}

// DuplicateFindings renders the duplicated identifiers a sweep found as check
// findings, each naming every path claiming the identifier so that an operator
// can choose between them without opening a second report.
func (r *TreeContainerReport) DuplicateFindings() []bench.Finding {
	ids := make([]string, 0, len(r.Duplicates))
	for id := range r.Duplicates {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	findings := make([]bench.Finding, 0, len(ids))
	for _, id := range ids {
		paths := r.Duplicates[id]
		detail := id + ": " + strings.Join(paths, ", ")
		findings = append(findings, bench.Finding{Path: paths[0], Key: bench.FindingDuplicateWorkbenchID, Detail: detail})
	}
	return findings
}

// RemintWorkbench gives one workbench directory a fresh identifier. It is the
// narrow, explicit repair beside the tree sweep, for the case the sweep must
// never decide on its own, and it takes exactly one path.
func RemintWorkbench(path string) (*RemintReport, error) {
	moved, err := bench.Remint(path)
	if err != nil {
		return nil, err
	}
	report := &RemintReport{
		Outcome: contract.ReadOK,
		From:    path,
		To:      moved,
		ID:      filepath.Base(moved),
	}
	return report, nil
}
