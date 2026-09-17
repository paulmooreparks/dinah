package verb

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"dinah/internal/bench"
	"dinah/internal/contract"
)

// The roster words. Each names a set the workbench holds, or in the case of
// workbenches a set the filesystem holds, and none of them is an address in
// the reference grammar. They belong to the list command alone, and no other
// command accepts one.
const (
	RosterColumns     = "columns"
	RosterCards       = "cards"
	RosterWorkstreams = "workstreams"
	RosterAttachments = "attachments"
	RosterWorkbenches = "workbenches"
)

// RosterWords are the five words list accepts beside the reference grammar, in
// the order the guide and the bare listing draw them. The first four name a
// collection this workbench holds and each of those is a row of the bare
// listing; workbenches names what lies beneath a directory, so it has no row
// and it needs --root to mean anything.
var RosterWords = []string{RosterColumns, RosterCards, RosterWorkstreams, RosterAttachments, RosterWorkbenches}

// RosterWordOf reports the roster word a list argument spells, and whether it
// spells one at all.
//
// The words are resolved ahead of the reference grammar, so a column or a
// workstream whose slug is spelled like one of them does not shadow the word.
// A command cannot let the data it reads redefine the words it is written in,
// and both readings stay reachable: such a column answers to its identifier
// and to its name, and such a workstream answers to workstream/<slug>.
func RosterWordOf(ref string) (string, bool) {
	trimmed := strings.TrimSpace(ref)
	for _, word := range RosterWords {
		if trimmed == word {
			return word, true
		}
	}
	return "", false
}

// Roster is one top-level collection of the workbench, named by the word a
// reader types back and counted.
type Roster struct {
	Reference string `json:"reference"`
	Holds     string `json:"holds"`
	Count     int    `json:"count"`
}

// RosterListing is what a bare list answers. It carries every roster of the
// workbench, in the fixed order of the table the guide draws.
type RosterListing struct {
	Rosters []Roster `json:"rosters"`
}

// ListAnswer is one workbench's answer to a list question inside a forest row.
// Exactly one group of members is set, and which group is fixed by the
// reference the caller wrote, so a client that knows what it asked for reads
// the members for that shape and no others.
//
// The union is flat rather than nested because a member has to mean one thing
// wherever it appears: cards is []CardView under every reference that answers
// cards at all, which is what lets a client written against one shape read the
// cards of another without learning a second spelling.
//
// Query, Cards and Count carry no omitempty, and Matches is the reason. A
// single-workbench answer to `list cards` is a Matches, which publishes an
// empty query string and a zero count rather than dropping either, so a forest
// row answering the same question publishes them too. Under omitempty the
// cards row could never carry the query member at all, because the query for
// the roster word is empty by construction.
//
// Dropping omitempty alone would have put those three members on every other
// arm as well, and a forest row is required to read byte for byte as the
// single-workbench answer to the same question, which a roster carrying an
// empty cards array does not. So the shape the reference selected is carried
// beside the members and MarshalJSON publishes that arm's members and no
// others. The declared tags stay what a decoder reads.
type ListAnswer struct {
	// shape is which arm the reference filled. It is unexported because it is
	// how this type publishes itself rather than something it publishes, and
	// a decoder reading one of these back reads the members instead.
	shape ListShape

	Rosters     []Roster         `json:"rosters,omitempty"`
	Columns     []ColumnView     `json:"columns,omitempty"`
	Workstreams []WorkstreamView `json:"workstreams,omitempty"`
	Kind        string           `json:"kind,omitempty"`
	Ref         string           `json:"ref,omitempty"`
	Attachments []AttachmentView `json:"attachments,omitempty"`
	Query       string           `json:"query"`
	Column      string           `json:"column,omitempty"`
	Cards       []CardView       `json:"cards"`
	Count       int              `json:"count"`
}

// ListShape names which arm of a list answer the reference selected. A head
// reads it to choose a renderer and to choose what it publishes under --json,
// and the shape is decided in one place so that the terminal and the machine
// surface cannot answer one reference two ways.
type ListShape string

const (
	// ShapeRosters is the bare invocation and the workbench spellings.
	ShapeRosters ListShape = "rosters"
	// ShapeColumns is the roster word columns.
	ShapeColumns ListShape = "columns"
	// ShapeWorkstreams is the roster word workstreams.
	ShapeWorkstreams ListShape = "workstreams"
	// ShapeWorkbenches is the roster word workbenches, whose answer is an
	// enumeration of directories rather than a read of this workbench, so
	// the library fills nothing and the head runs the walk.
	ShapeWorkbenches ListShape = "workbenches"
	// ShapeAttachments is the roster word attachments and any reference
	// ending in an attachments collection.
	ShapeAttachments ListShape = "attachments"
	// ShapeMatches is the roster word cards and any workstream reference,
	// both of which answer through Library.Query.
	ShapeMatches ListShape = "matches"
	// ShapeQueue is a column, whose cards come in queue order.
	ShapeQueue ListShape = "queue"
	// ShapeHistory is a card's journal.
	ShapeHistory ListShape = "history"
	// ShapeContents is every other reference, and any reference walked under
	// --depth or read in the archive half.
	ShapeContents ListShape = "contents"
)

// ListResult is what Library.ListRef answers: the shape the reference
// selected, and the one member that shape fills. A head reads Shape and then
// the member named for it, and every member is the answer of the library call
// the reference routes to rather than an envelope composed here.
type ListResult struct {
	Shape       ListShape
	Rosters     *RosterListing
	Columns     []ColumnView
	Workstreams *WorkstreamListing
	Attachments *AttachmentListing
	Matches     *Matches
	Queue       *Listing
	History     []bench.Event
	Contents    *Tree
}

// Answer projects a result onto the flat union a forest row publishes. It
// answers nil for the two shapes no forest row can carry, which are a journal
// and a containment walk. Neither reaches a row: --root refuses a journal
// reference and every reference whose plain reading is a walk, and it refuses
// --depth, which is the other door a walk could arrive through.
func (r *ListResult) Answer() *ListAnswer {
	answer := &ListAnswer{shape: r.Shape, Cards: []CardView{}}
	switch r.Shape {
	case ShapeRosters:
		answer.Rosters = r.Rosters.Rosters
	case ShapeColumns:
		answer.Columns = r.Columns
	case ShapeWorkstreams:
		answer.Workstreams = r.Workstreams.Workstreams
	case ShapeAttachments:
		answer.Kind = r.Attachments.Kind
		answer.Ref = r.Attachments.Ref
		answer.Attachments = r.Attachments.Attachments
	case ShapeMatches:
		answer.Query = r.Matches.Query
		answer.Cards = r.Matches.Cards
		answer.Count = r.Matches.Count
	case ShapeQueue:
		answer.Column = r.Queue.Column
		answer.Cards = r.Queue.Cards
	default:
		return nil
	}
	return answer
}

// MarshalJSON publishes the members of the arm the reference selected, which
// is what makes a forest row read as the single-workbench answer to the same
// question. An answer carrying no shape, which is one a caller built rather
// than one Answer composed, publishes every member its tags declare.
func (a ListAnswer) MarshalJSON() ([]byte, error) {
	switch a.shape {
	case ShapeRosters:
		return json.Marshal(RosterListing{Rosters: a.Rosters})
	case ShapeColumns:
		return json.Marshal(a.Columns)
	case ShapeWorkstreams:
		return json.Marshal(WorkstreamListing{Workstreams: a.Workstreams})
	case ShapeAttachments:
		return json.Marshal(AttachmentListing{Kind: a.Kind, Ref: a.Ref, Attachments: a.Attachments})
	case ShapeMatches:
		return json.Marshal(Matches{Query: a.Query, Cards: a.Cards, Count: a.Count})
	case ShapeQueue:
		return json.Marshal(Listing{Column: a.Column, Cards: a.Cards})
	}
	type plain ListAnswer
	return json.Marshal(plain(a))
}

// The four flag names list declares that a reference shape may refuse. They
// are spelled here as a reader types them, because the refusal's detail names
// the combination the reader wrote rather than the flag alone.
const (
	flagDepth    = "--depth"
	flagReady    = "--ready"
	flagArchived = "--archived"
	flagRoot     = "--root"
)

// listFlags is which of the four flags one reference shape reads. Three rules
// produce every value. --depth switches a reference from its reader's answer
// to its containment walk, so a roster word and a journal, neither of which
// has a walk, refuse it. --ready narrows an answer of cards, so only the three
// shapes whose answer is cards read it. --archived resolves a reference in the
// archive half, so every reference reads it and a roster word does not.
type listFlags struct {
	depth    bool
	ready    bool
	archived bool
	root     bool
}

// refuseListFlag composes the refusal a flag a shape does not read raises. The
// detail names the combination, because the flag on its own is one this
// command declares and a sentence saying it was not understood would be false.
func refuseListFlag(flag, subject string) error {
	return contract.Refuse(contract.Usage, fmt.Sprintf("%s beside %s", flag, subject))
}

// check refuses the first flag this invocation names that this shape does not
// read.
func (f listFlags) check(req *Request, subject string) error {
	if req.Depth != "" && !f.depth {
		return refuseListFlag(flagDepth, subject)
	}
	if req.ReadyOnly && !f.ready {
		return refuseListFlag(flagReady, subject)
	}
	if req.Archived && !f.archived {
		return refuseListFlag(flagArchived, subject)
	}
	if req.Root != "" && !f.root {
		return refuseListFlag(flagRoot, subject)
	}
	return nil
}

// rosterSubject names a roster word the way the refusal reads it.
func rosterSubject(word string) string { return "the roster word " + word }

// The subjects the reference shapes are named by when a refusal reports the
// combination a reader wrote.
const (
	subjectWorkbench  = "the workbench"
	subjectColumn     = "a column reference"
	subjectWorkstream = "a workstream reference"
	subjectCard       = "a card reference"
	subjectBelowCard  = "a reference below a card"
	subjectCollection = "a collection reference"
	subjectJournal    = "a journal reference"
)

// RootScopedReference reports whether --root may be written beside a
// reference, and, where it may not, how the refusal names it.
//
// The classification is syntactic, and it has to be: --root fans one question
// out over many workbenches, so no workbench is open yet to resolve the
// reference against. The five roster words are the command's own grammar, the
// workbench spellings name the same sort of thing everywhere, and a workstream
// declares its kind in its own prefix. A bare word that reads as a card
// reference under the card grammar is refused, and every other bare word is
// taken for a column, which each workbench resolves in its own table and
// refuses on its own row where it carries no such column.
//
// One case the rule cannot separate is a column whose slug happens to read as
// a card reference, which is refused here rather than fanned out. Nothing
// resolves that spelling without a workbench, and refusing it is the reading
// that never answers a different question in each workbench.
func RootScopedReference(ref string) (bool, string) {
	trimmed := strings.TrimSpace(ref)
	if _, roster := RosterWordOf(trimmed); roster {
		return true, ""
	}
	if trimmed == "" || bench.IsWorkbenchRef(trimmed) {
		return true, ""
	}
	if strings.HasPrefix(trimmed, bench.WorkstreamRefPrefix) {
		return true, ""
	}
	if _, rest, found := strings.Cut(trimmed, "/"); found {
		file, own := bench.CardOwnFile(rest)
		switch {
		case own && file == bench.CardFileJournal:
			return false, subjectJournal
		case own:
			return false, subjectCard
		}
		return false, subjectBelowCard
	}
	if bench.ReadsAsCardRef(trimmed) {
		return false, subjectCard
	}
	return true, ""
}

// archivedReadingWalks reports whether --archived turns this reference into a
// containment walk, which is the reading a forest row cannot publish. It
// answers the question syntactically, for the same reason RootScopedReference
// does: --root fans one question out before any workbench is open, so nothing
// has resolved the reference yet.
//
// A bare invocation and the workbench spellings count the archive half's
// rosters, and a roster word refuses the flag outright, so neither reads as a
// walk. A workstream refuses the flag by name on dinah-523/decisions/12, and
// that refusal is the one a reader wants to be told about, so it is left to
// fire inside each workbench rather than overwritten here. Everything else
// --root admits is a column reference, whose archive-half reading is the walk.
func archivedReadingWalks(ref string) bool {
	trimmed := strings.TrimSpace(ref)
	if trimmed == "" || bench.IsWorkbenchRef(trimmed) {
		return false
	}
	if _, roster := RosterWordOf(trimmed); roster {
		return false
	}
	return !strings.HasPrefix(trimmed, bench.WorkstreamRefPrefix)
}

// ListRef answers what a reference holds, which is the whole of the list
// command.
//
// Nine reference shapes reach nine library calls, and this function is the one
// place that maps between them. It composes no envelope of its own except the
// roster, which has no library call behind it, so what a head publishes for a
// reference is what the call behind that reference already published.
//
// --archived and --depth both move a reference off its reader's answer and
// onto the containment walk. That is one rule rather than two special cases:
// the walk is the only reading the archive half has, because a queue and a
// membership index are live-half structures with no archived counterpart, and
// the walk is what --depth was asking for in the first place.
func (l *Library) ListRef(req *Request) (*ListResult, error) {
	ref := strings.TrimSpace(req.Ref)
	level := req.Depth
	if level == "" {
		level = LevelMembers
	}
	if word, roster := RosterWordOf(ref); roster {
		return l.rosterWord(req, word)
	}
	if ref == "" || bench.IsWorkbenchRef(ref) {
		if err := (listFlags{depth: true, archived: true, root: true}).check(req, subjectWorkbench); err != nil {
			return nil, err
		}
		// --archived counts the four rosters over the archive half rather
		// than walking it. The workbench itself is never archived, so a walk
		// under the flag is refused by the resolver before it starts, and
		// counting what the mirror holds is the only reading the flag has
		// here.
		if req.Depth != "" {
			return l.listContents(req, level)
		}
		rosters, err := l.Rosters(req.Archived)
		if err != nil {
			return nil, err
		}
		return &ListResult{Shape: ShapeRosters, Rosters: rosters}, nil
	}
	if head, rest, found := strings.Cut(ref, "/"); found {
		if file, own := bench.CardOwnFile(rest); own && file == bench.CardFileJournal {
			return l.listJournal(req, head)
		}
	}
	entity, collection, err := l.Bench.ResolveReferenceIn(halfFor(req), ref)
	if err != nil {
		return nil, unknownColumnFor(l.Bench, ref, err)
	}
	if collection != nil {
		if err := (listFlags{depth: true, archived: true}).check(req, subjectCollection); err != nil {
			return nil, err
		}
		if req.Depth == "" && !req.Archived && collection.Mount.Kind == bench.KindAttachment {
			return l.listAttachments(req, ref)
		}
		return l.listContents(req, level)
	}
	switch entity.Kind {
	case bench.KindColumn:
		if err := (listFlags{depth: true, ready: true, archived: true, root: true}).check(req, subjectColumn); err != nil {
			return nil, err
		}
		if req.Depth != "" || req.Archived {
			return l.listContents(req, level)
		}
		queued := *req
		queued.Column = ref
		listing, err := l.List(&queued)
		if err != nil {
			return nil, err
		}
		return &ListResult{Shape: ShapeQueue, Queue: listing}, nil
	case bench.KindWorkstream:
		// --archived is the one flag of the four a workstream reference
		// does not read, on the ruling recorded as dinah-523/decisions/12.
		// A workstream holds a membership rather than a containment, and
		// the walk from one draws that membership, so neither of a
		// workstream's two readings has an archive half to read: the
		// membership index is a live-half structure and the walk over it
		// is the same structure seen from above. The flag is refused by
		// name here, as it already is beside the roster word workstreams,
		// rather than answering a live membership under a flag that asked
		// for the archive.
		if err := (listFlags{depth: true, ready: true, root: true}).check(req, subjectWorkstream); err != nil {
			return nil, err
		}
		if req.Depth != "" {
			return l.listContents(req, level)
		}
		return l.listMatches(req, workstreamSelector(l.Bench, entity))
	case bench.KindCard:
		if err := (listFlags{depth: true, archived: true}).check(req, subjectCard); err != nil {
			return nil, err
		}
		return l.listContents(req, level)
	}
	if err := (listFlags{depth: true, archived: true}).check(req, subjectBelowCard); err != nil {
		return nil, err
	}
	return l.listContents(req, level)
}

// unknownColumnFor restates a resolver refusal as unknown-column where the
// reference the reader wrote reads as a column reference and nothing else.
//
// A bare word with no slash in it is a column, a card or a workstream handle,
// and the resolver tries them in that order and reports the last one it
// tried. A reader who typed a word that reads as no card reference at all was
// naming a column, so they are told the workbench declares no such column and
// shown the columns it does declare, which is the refusal the retired
// listing command gave them
// before this command absorbed it. A word that reads as a card reference keeps
// the resolver's own unknown-card, because that is what the reader was asking
// about.
func unknownColumnFor(b *bench.Bench, ref string, err error) error {
	var refusal *contract.Refusal
	if !errors.As(err, &refusal) || refusal.Name != contract.UnknownCard {
		return err
	}
	if strings.Contains(ref, "/") || bench.ReadsAsCardRef(ref) {
		return err
	}
	return contract.Refuse(contract.UnknownColumn, ref)
}

// CheckWorkbenchesFlags refuses a flag the roster word workbenches does not
// read. It is exported because that word is the one reference a head answers
// off a directory rather than off a workbench, so the head branches on it
// before any library is open and the flag check has to travel with the branch.
func CheckWorkbenchesFlags(req *Request) error {
	return listFlags{root: true}.check(req, rosterSubject(RosterWorkbenches))
}

// rosterWord answers one of the five words that are the command's own grammar
// rather than addresses.
func (l *Library) rosterWord(req *Request, word string) (*ListResult, error) {
	flags := listFlags{root: true}
	if word == RosterCards {
		flags.ready = true
	}
	if err := flags.check(req, rosterSubject(word)); err != nil {
		return nil, err
	}
	switch word {
	case RosterColumns:
		columns, err := l.Columns()
		if err != nil {
			return nil, err
		}
		return &ListResult{Shape: ShapeColumns, Columns: columns}, nil
	case RosterWorkstreams:
		listing, err := l.Workstreams()
		if err != nil {
			return nil, err
		}
		return &ListResult{Shape: ShapeWorkstreams, Workstreams: listing}, nil
	case RosterAttachments:
		return l.listAttachments(req, "")
	case RosterWorkbenches:
		return &ListResult{Shape: ShapeWorkbenches}, nil
	}
	return l.listMatches(req, "")
}

// listMatches answers the cards a selector picks out, which is the one door
// both the cards roster word and a workstream reference go through. --ready is
// composed onto the selector rather than applied to the answer, so
// `list cards --ready` and `query state:ready` print the same bytes and the
// published query member reads what the reader would otherwise have typed.
func (l *Library) listMatches(req *Request, selector string) (*ListResult, error) {
	asked := *req
	asked.Query = narrowToReady(selector, req.ReadyOnly)
	matches, err := l.Query(&asked)
	if err != nil {
		return nil, err
	}
	return &ListResult{Shape: ShapeMatches, Matches: matches}, nil
}

// narrowToReady composes --ready onto a selector, and it is the one place the
// flag becomes a query term. Composing rather than filtering an answer is what
// makes the published query member read what the reader would otherwise have
// typed, and it is why both readings of a reference narrow the same way: the
// membership a walk is rooted on is selected by the same string the no-depth
// answer selects by, so `list workstream/<slug> --ready` and
// `list workstream/<slug> --depth all --ready` cannot part company over which
// cards the flag admits. That is the ruling recorded as dinah-523/decisions/14,
// and section 3.6 of the contract is the sentence behind it: --depth over a
// workstream draws the same membership and then walks below it.
func narrowToReady(selector string, readyOnly bool) string {
	if !readyOnly {
		return selector
	}
	ready := FieldState + ":" + contract.StateReady
	if selector == "" {
		return ready
	}
	return selector + " " + ready
}

// workstreamSelector is the query that answers a workstream's cards. The
// membership index is what holds them, because a workstream contains nothing:
// cards join and leave one, and the containment table says so by leaving the
// kind out.
func workstreamSelector(b *bench.Bench, entity *bench.EntityRef) string {
	handle := entity.ID
	if workstream := b.Workstream(entity.ID); workstream != nil && workstream.Slug != "" {
		handle = workstream.Slug
	}
	return FieldWorkstream + ":" + handle
}

// listAttachments answers an entity's own attachments, named either by the
// roster word or by a reference ending in the collection. Its caller has
// already run the flag check for the shape that reached it.
func (l *Library) listAttachments(req *Request, ref string) (*ListResult, error) {
	asked := *req
	asked.Ref = ref
	listing, err := l.Attachments(&asked)
	if err != nil {
		return nil, err
	}
	return &ListResult{Shape: ShapeAttachments, Attachments: listing}, nil
}

// listJournal renders a card's journal, which is a file rather than a
// collection and so is answered by a library call rather than by the
// containment walk.
//
// Both spellings of the file reach this, because the resolver has always
// treated `<card>/journal` and `<card>/journal.ndjson` as one reference and
// reading the word off bench.CardOwnFile is what keeps them one here.
func (l *Library) listJournal(req *Request, card string) (*ListResult, error) {
	if err := (listFlags{archived: true}).check(req, subjectJournal); err != nil {
		return nil, err
	}
	asked := *req
	asked.Card = card
	events, err := l.history(&asked)
	if err != nil {
		return nil, err
	}
	return &ListResult{Shape: ShapeHistory, History: events}, nil
}

// history is History reading the half the request names. The live half is
// History itself; the archive half resolves the card in the mirror and reads
// the journal the card carried in with it, because an archived card keeps its
// own directory and its own journal.
func (l *Library) history(req *Request) ([]bench.Event, error) {
	if !req.Archived {
		return l.History(req)
	}
	found, err := l.Bench.ResolveArchivedCard(req.Card)
	if err != nil {
		return nil, err
	}
	events, _, err := bench.ReadJournal(found.Card.JournalPath())
	return events, err
}

// listContents walks the containment grammar down from the reference.
func (l *Library) listContents(req *Request, level string) (*ListResult, error) {
	tree, err := l.Contents(req, level)
	if err != nil {
		return nil, err
	}
	return &ListResult{Shape: ShapeContents, Contents: tree}, nil
}

// Rosters counts the top-level collections of the workbench, in the fixed
// order the bare listing draws them.
//
// Three of the four come off the containment table, which is what the
// workbench mounts, and workstreams is the fourth because a workstream is a
// membership rather than a containment and the table leaves it out on purpose.
// The attachments count is the workbench's own, and not the attachments of
// everything beneath it.
func (l *Library) Rosters(archived bool) (*RosterListing, error) {
	half := bench.LiveHalf
	if archived {
		half = bench.ArchivedHalf
	}
	listing := &RosterListing{Rosters: []Roster{}}
	for _, mount := range bench.Contains(bench.KindWorkbench) {
		count, err := bench.CountIn(l.Bench.CollectionRootIn(half, mount.Dir))
		if err != nil {
			return nil, err
		}
		listing.Rosters = append(listing.Rosters, Roster{
			Reference: mount.Dir,
			Holds:     mount.Kind,
			Count:     count,
		})
		// The workstreams row follows the cards row, which is where the
		// guide's table draws it. It is not read off the containment table
		// because the table leaves the kind out on purpose.
		if mount.Kind == bench.KindCard {
			streams, err := bench.CountIn(l.Bench.CollectionRootIn(half, bench.WorkstreamsDir))
			if err != nil {
				return nil, err
			}
			listing.Rosters = append(listing.Rosters, Roster{
				Reference: RosterWorkstreams,
				Holds:     bench.KindWorkstream,
				Count:     streams,
			})
		}
	}
	return listing, nil
}
