package verb

import (
	"strings"

	"dinah/internal/bench"
	"dinah/internal/contract"
)

// forestRow is one candidate EnumerateDeep found beneath a root, carrying
// either an opened Library or the refusal name that stopped it short of one.
// That name is EnumerateDeep's own Refused, passed through unopened, or
// bench.Open's own refusal on a candidate EnumerateDeep could describe but
// could not open, which is a malformed definition or a mid-walk removal.
// Exactly one of Library and Candidate.Refused is set.
//
// Both of those are the one fact that the workbench itself would not read, so
// both belong on Candidate.Refused. A workbench that opened and then refused
// the question this read asks of it is a different fact, and it never reaches
// this type: it arises inside each builder's loop, after the row is made, and
// is reported on the member's own Unanswered field.
type forestRow struct {
	Candidate bench.Candidate
	Library   *Library
}

// forestCandidates is the walk-and-open sequence every root-scoped read
// shares. It runs bench.EnumerateDeep once and opens every candidate that walk
// could describe, in the walk's own order.
//
// home is threaded through the way the MCP head already threads it for a
// per-call workbench: the caller's own configured home, so a workbench opened
// this way carries the same instruction-layer context a single-workbench call
// would.
//
// The error return is the walk's own whole-call refusal, which is an empty,
// missing or non-directory root. A workbench that fails once the walk has
// found it never reaches this return; it comes back as a row carrying a
// refusal name, because one unreadable workbench among twenty-five must not
// take the other twenty-four with it.
func forestCandidates(root, home string, maxDepth int) ([]forestRow, error) {
	listed, err := bench.EnumerateDeep(root, maxDepth)
	if err != nil {
		return nil, err
	}
	rows := make([]forestRow, 0, len(listed))
	for _, candidate := range listed {
		rows = append(rows, openCandidate(candidate, home))
	}
	return rows, nil
}

// openCandidate turns one walked candidate into a row. A candidate the walk
// already refused is passed through unopened, since opening a directory whose
// anchor would not read would only produce the same refusal a second time.
func openCandidate(candidate bench.Candidate, home string) forestRow {
	if candidate.Refused != "" {
		return forestRow{Candidate: candidate}
	}
	opened, err := bench.Open(candidate.Path)
	if err != nil {
		candidate.Refused = refusalNameOf(err)
		return forestRow{Candidate: candidate}
	}
	return forestRow{Candidate: candidate, Library: New(opened, home)}
}

// refusalNameOf is the contract name an error carries, and UnreadableBench for
// an error that is not a refusal. A row reports why it carries no answer, so
// the fallback names the condition rather than leaving the field empty beside
// an equally empty answer.
//
// internal/bench/bench.go holds a copy of this function, spelled identically,
// because that package's own walk needs the same answer and an unexported
// helper cannot cross a package boundary. Change one and change the other.
// The copy exists rather than an exported name because naming it in bench's
// public surface would publish an error-classification detail no caller
// outside these two files has any use for.
func refusalNameOf(err error) string {
	if refusal, ok := err.(*contract.Refusal); ok {
		return refusal.Name
	}
	return contract.UnreadableBench
}

// Forest is one dinah tree --root answer: every workbench the walk found
// beneath Root, each with its own Tree or, on a workbench that would not open,
// a refusal name in place of one.
type Forest struct {
	// Root is the directory the walk started from, as the caller named it.
	Root string `json:"root"`
	// Workbenches are the walk's rows in its own order, never nil, so an
	// answer finding nothing marshals as an empty array.
	Workbenches []WorkbenchTree `json:"workbenches"`
}

// WorkbenchTree is one workbench's row of a Forest: the identity the walk
// read off its anchor, and the tree that workbench answered with.
type WorkbenchTree struct {
	bench.Candidate
	// Unanswered is the refusal name this workbench's own Tree call raised,
	// empty on a row that answered and on a row that never opened. It is
	// separate from the embedded Candidate.Refused, which says the workbench
	// would not read: a row carrying Unanswered read perfectly well, carries
	// its title and its slug, and declined the question. A client drawing a
	// workbench it could not read differently from one that declined a read
	// asks which field is set, rather than inspecting how a refusal name is
	// spelled.
	Unanswered string `json:"unanswered,omitempty"`
	// Tree is this workbench's own answer, byte for byte what a single
	// workbench call produces. It is absent on a row that would not read and
	// on a row that read and did not answer, which the two refusal fields
	// tell apart.
	Tree *Tree `json:"tree,omitempty"`
}

// TreeForest asks every workbench beneath root the same Tree question, with
// the one chain, level and query every workbench in the answer is asked with.
//
// The chain and the level are checked once, ahead of the loop. An unknown axis
// is a mistake in what the caller typed rather than a fact about any workbench
// that answers it, so refusing it once, the way Library.Tree already refuses
// it for one workbench, is truer than refusing it twenty-five times or burying
// it in one row's Unanswered.
func TreeForest(root, home string, req *Request, chain []string, level string, maxDepth int) (*Forest, error) {
	if len(chain) == 0 {
		chain = DefaultChain()
	}
	if err := checkChain(chain); err != nil {
		return nil, err
	}
	if err := checkLevel(level, TreeLevels); err != nil {
		return nil, err
	}
	rows, err := forestCandidates(root, home, maxDepth)
	if err != nil {
		return nil, err
	}
	forest := &Forest{Root: root, Workbenches: make([]WorkbenchTree, 0, len(rows))}
	for _, row := range rows {
		member := WorkbenchTree{Candidate: row.Candidate}
		if row.Library != nil {
			tree, err := row.Library.Tree(req, chain, level)
			if err != nil {
				member.Unanswered = refusalNameOf(err)
			} else {
				member.Tree = tree
			}
		}
		forest.Workbenches = append(forest.Workbenches, member)
	}
	return forest, nil
}

// RootStatus is one dinah status --root answer.
type RootStatus struct {
	// Root is the directory the walk started from, as the caller named it.
	Root string `json:"root"`
	// Workbenches are the walk's rows in its own order, never nil.
	Workbenches []WorkbenchStatus `json:"workbenches"`
}

// WorkbenchStatus is one workbench's row of a RootStatus.
type WorkbenchStatus struct {
	bench.Candidate
	// Unanswered is the refusal name this workbench's own Status call raised,
	// empty on a row that answered and on a row that never opened. It is
	// separate from the embedded Candidate.Refused, which says the workbench
	// would not read: a row carrying Unanswered read perfectly well, carries
	// its title and its slug, and declined the question. A client drawing a
	// workbench it could not read differently from one that declined a read
	// asks which field is set, rather than inspecting how a refusal name is
	// spelled.
	Unanswered string `json:"unanswered,omitempty"`
	// Status is this workbench's own answer, absent on a row that would not
	// read and on a row that read and did not answer.
	Status *Status `json:"status,omitempty"`
}

// StatusForest asks every workbench beneath root the same Status question a
// bare dinah status asks one. req.Actor carries through unchanged, so Holding
// reports the calling actor's own held cards in each workbench, the same actor
// a bare dinah status --workbench <path> would report for.
func StatusForest(root, home string, req *Request, maxDepth int) (*RootStatus, error) {
	rows, err := forestCandidates(root, home, maxDepth)
	if err != nil {
		return nil, err
	}
	answer := &RootStatus{Root: root, Workbenches: make([]WorkbenchStatus, 0, len(rows))}
	for _, row := range rows {
		member := WorkbenchStatus{Candidate: row.Candidate}
		if row.Library != nil {
			status, err := row.Library.Status(req)
			if err != nil {
				member.Unanswered = refusalNameOf(err)
			} else {
				member.Status = status
			}
		}
		answer.Workbenches = append(answer.Workbenches, member)
	}
	return answer, nil
}

// RootListing is one dinah ls --root answer.
type RootListing struct {
	// Root is the directory the walk started from, as the caller named it.
	Root string `json:"root"`
	// Workbenches are the walk's rows in its own order, never nil.
	Workbenches []WorkbenchListing `json:"workbenches"`
}

// WorkbenchListing is one workbench's row of a RootListing.
type WorkbenchListing struct {
	bench.Candidate
	// Unanswered is the refusal name this workbench's own List call raised,
	// empty on a row that answered and on a row that never opened. It is
	// separate from the embedded Candidate.Refused, which says the workbench
	// would not read: a row carrying Unanswered read perfectly well, carries
	// its title and its slug, and declined the question. A client drawing a
	// workbench it could not read differently from one that declined a read
	// asks which field is set, rather than inspecting how a refusal name is
	// spelled.
	Unanswered string `json:"unanswered,omitempty"`
	// Listing is this workbench's own answer, absent on a row that would not
	// read and on a row that read and did not answer.
	Listing *Listing `json:"listing,omitempty"`
}

// ListForest asks every workbench beneath root the same List question, with
// req.Column and req.ReadyOnly applied identically in each.
//
// A column reference is resolved inside each workbench rather than once, which
// is the only reading available: the columns of one workbench are not the
// columns of another, so a reference naming a column here and nothing there
// leaves that second workbench refusing on its own row while the first still
// answers.
func ListForest(root, home string, req *Request, maxDepth int) (*RootListing, error) {
	rows, err := forestCandidates(root, home, maxDepth)
	if err != nil {
		return nil, err
	}
	answer := &RootListing{Root: root, Workbenches: make([]WorkbenchListing, 0, len(rows))}
	for _, row := range rows {
		member := WorkbenchListing{Candidate: row.Candidate}
		if row.Library != nil {
			listing, err := row.Library.List(req)
			if err != nil {
				member.Unanswered = refusalNameOf(err)
			} else {
				member.Listing = listing
			}
		}
		answer.Workbenches = append(answer.Workbenches, member)
	}
	return answer, nil
}

// RootOffers is one dinah next --root answer.
type RootOffers struct {
	// Root is the directory the walk started from, as the caller named it.
	Root string `json:"root"`
	// Workbenches are the walk's rows in its own order, never nil.
	Workbenches []WorkbenchOffers `json:"workbenches"`
}

// WorkbenchOffers is one workbench's row of a RootOffers.
type WorkbenchOffers struct {
	bench.Candidate
	// Unanswered is the refusal name this workbench's own Next call raised,
	// empty on a row that answered and on a row that never opened. It is
	// separate from the embedded Candidate.Refused, which says the workbench
	// would not read: a row carrying Unanswered read perfectly well, carries
	// its title and its slug, and declined the question. A client drawing a
	// workbench it could not read differently from one that declined a read
	// asks which field is set, rather than inspecting how a refusal name is
	// spelled.
	Unanswered string `json:"unanswered,omitempty"`
	// Offers is this workbench's own answer, absent on a row that would not
	// read and on a row that read and did not answer.
	Offers []Offer `json:"offers,omitempty"`
}

// NextForest asks every workbench beneath root the same Next question, with
// req.Column applied identically in each, and changes nothing anywhere.
func NextForest(root, home string, req *Request, maxDepth int) (*RootOffers, error) {
	rows, err := forestCandidates(root, home, maxDepth)
	if err != nil {
		return nil, err
	}
	answer := &RootOffers{Root: root, Workbenches: make([]WorkbenchOffers, 0, len(rows))}
	for _, row := range rows {
		member := WorkbenchOffers{Candidate: row.Candidate}
		if row.Library != nil {
			offers, err := row.Library.Next(req)
			if err != nil {
				member.Unanswered = refusalNameOf(err)
			} else {
				member.Offers = offers
			}
		}
		answer.Workbenches = append(answer.Workbenches, member)
	}
	return answer, nil
}

// RootSearch is one dinah search --root answer.
type RootSearch struct {
	// Root is the directory the walk started from, as the caller named it.
	Root string `json:"root"`
	// Workbenches are the walk's rows in its own order, never nil.
	Workbenches []WorkbenchSearch `json:"workbenches"`
}

// WorkbenchSearch is one workbench's row of a RootSearch.
type WorkbenchSearch struct {
	bench.Candidate
	// Unanswered is the refusal name this workbench's own Search call raised,
	// empty on a row that answered and on a row that never opened. It is
	// separate from the embedded Candidate.Refused, which says the workbench
	// would not read: a row carrying Unanswered read perfectly well, carries
	// its title and its slug, and declined the question. A client drawing a
	// workbench it could not read differently from one that declined a read
	// asks which field is set, rather than inspecting how a refusal name is
	// spelled.
	Unanswered string `json:"unanswered,omitempty"`
	// Results are this workbench's own hits, absent on a row that would not
	// read and on a row that read and did not answer.
	Results *SearchResults `json:"results,omitempty"`
}

// SearchForest asks every workbench beneath root the same Search question,
// with the one phrase, the one filter and the one archive choice.
//
// An empty phrase is refused once, ahead of the walk, the way TreeForest
// refuses an unknown axis: a caller who typed no phrase made one mistake
// rather than twenty-five, and the refusal belongs to what they typed rather
// than to any workbench that would have answered it.
func SearchForest(root, home string, req *Request, maxDepth int) (*RootSearch, error) {
	if strings.TrimSpace(req.SearchText) == "" {
		return nil, contract.Refuse(contract.EmptySearch, "")
	}
	rows, err := forestCandidates(root, home, maxDepth)
	if err != nil {
		return nil, err
	}
	answer := &RootSearch{Root: root, Workbenches: make([]WorkbenchSearch, 0, len(rows))}
	for _, row := range rows {
		member := WorkbenchSearch{Candidate: row.Candidate}
		if row.Library != nil {
			results, err := row.Library.Search(req)
			if err != nil {
				member.Unanswered = refusalNameOf(err)
			} else {
				member.Results = results
			}
		}
		answer.Workbenches = append(answer.Workbenches, member)
	}
	return answer, nil
}
