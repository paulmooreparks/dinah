package verb

import (
	"path/filepath"
	"strconv"
	"strings"

	"dinah/internal/bench"
	"dinah/internal/contract"
	"dinah/internal/msg"
)

// StateView is one state as a read reports it.
type StateView struct {
	// ID is the state's identifier.
	ID string `json:"id"`
	// Slug is the state's short handle. It is absent on a state written
	// before the field existed and left out until the migration runs.
	Slug string `json:"slug,omitempty"`
	// Title is the state's title.
	Title string `json:"title"`
	// Kind is one of intake, work and done.
	Kind string `json:"kind"`
	// OperatorOwned marks a state only the operator moves a card out of.
	OperatorOwned bool `json:"operator_owned"`
	// AwaitingOutside marks a state where the workbench waits on somebody
	// who is not an owner of it, so no owner takes work up there. It
	// answers a different question from OperatorOwned, which is about
	// departure alone, and a state may carry both.
	AwaitingOutside bool `json:"awaiting_outside"`
	// Capacity is the declared limit, zero for unlimited.
	Capacity int `json:"capacity,omitempty"`
	// Count is the number of live cards the state holds.
	Count int `json:"count"`
}

// Status is where the bench stands and what the reader holds.
type Status struct {
	// Bench is the workbench's title.
	Bench string `json:"workbench"`
	// Root is the directory the bench was discovered in.
	Root string `json:"root"`
	// Actor is the owner this invocation acts as.
	Actor string `json:"actor,omitempty"`
	// IsOperator says whether that owner is the operator of this bench,
	// which is the question CORE-OWNER-2 obliges the tool to answer.
	IsOperator bool `json:"is_operator"`
	// Operator is the owner reserved acts belong to.
	Operator string `json:"operator,omitempty"`
	// Profile is the conformance claim of the bench definition.
	Profile string `json:"profile"`
	// States are the flow with each station's occupancy.
	States []StateView `json:"states"`
	// Holding are the cards this actor holds right now.
	Holding []CardView `json:"holding"`
	// Blocked are the cards waiting on the operator.
	Blocked []CardView `json:"blocked"`
	// WorkbenchSource names which rung resolved the active workbench for
	// this invocation: flag, environment, search, or config.
	WorkbenchSource string `json:"workbench_source,omitempty"`
}

// Status reports where the bench stands.
func (l *Library) Status(req *Request) (*Status, error) {
	cards, err := l.Bench.Cards()
	if err != nil {
		return nil, err
	}
	status := &Status{
		Bench:           l.Bench.Title,
		Root:            l.Bench.Root,
		Actor:           req.Actor,
		Operator:        l.Bench.Operator,
		IsOperator:      req.Actor != "" && req.Actor == l.Bench.Operator,
		Profile:         l.Bench.Profile,
		Holding:         []CardView{},
		Blocked:         []CardView{},
		WorkbenchSource: req.WorkbenchSource,
	}
	counts := map[string]int{}
	for _, card := range cards {
		if err := l.lapseRead(card); err != nil {
			return nil, err
		}
		counts[card.State]++
		if card.Holder != "" && card.Holder == req.Actor {
			status.Holding = append(status.Holding, *l.view(card))
		}
		if card.Substate == contract.SubstateBlocked {
			status.Blocked = append(status.Blocked, *l.view(card))
		}
	}
	status.States = l.stateViews(counts)
	return status, nil
}

// States reports the flow in order, which is the order of the list in
// workbench.md frontmatter and the single authority for it.
func (l *Library) States() ([]StateView, error) {
	cards, err := l.Bench.Cards()
	if err != nil {
		return nil, err
	}
	counts := map[string]int{}
	for _, card := range cards {
		counts[card.State]++
	}
	return l.stateViews(counts), nil
}

// stateViews renders the flow with each station's occupancy.
func (l *Library) stateViews(counts map[string]int) []StateView {
	views := make([]StateView, 0, len(l.Bench.States))
	for _, state := range l.Bench.States {
		view := StateView{
			ID:              state.ID,
			Slug:            state.Slug,
			Title:           state.Title,
			Kind:            state.Kind,
			OperatorOwned:   state.OperatorOwned,
			AwaitingOutside: state.AwaitingOutside,
			Capacity:        state.Capacity,
			Count:           counts[state.ID],
		}
		views = append(views, view)
	}
	return views
}

// Listing is the cards of a state in queue order.
type Listing struct {
	// State is the state listed, empty when the listing spans the bench.
	State string `json:"state,omitempty"`
	// Cards are the cards, in the order CORE-QUEUE-3 fixes.
	Cards []CardView `json:"cards"`
}

// List presents a state's cards in the profile's fixed order: earliest
// arrival first, ties broken by ascending identifier. A tool may offer other
// orders beside it, and this one stays available.
func (l *Library) List(req *Request) (*Listing, error) {
	cards, err := l.Bench.Cards()
	if err != nil {
		return nil, err
	}
	listing := &Listing{Cards: []CardView{}}
	var wanted *bench.State
	if req.State != "" {
		wanted = l.Bench.StateByRef(req.State)
		if wanted == nil {
			return nil, contract.Refuse(contract.UnknownState, req.State)
		}
		listing.State = wanted.ID
	}
	var kept []*bench.Card
	for _, card := range cards {
		if wanted != nil && card.State != wanted.ID {
			continue
		}
		if err := l.lapseRead(card); err != nil {
			return nil, err
		}
		if req.ReadyOnly && card.Substate != contract.SubstateReady {
			continue
		}
		kept = append(kept, card)
	}
	sortByArrival(kept)
	for _, card := range kept {
		listing.Cards = append(listing.Cards, *l.view(card))
	}
	return listing, nil
}

// Offer is the card a state offers next, or the absence of one.
type Offer struct {
	// State is the state the offer concerns.
	State string `json:"state"`
	// Title is that state's title.
	Title string `json:"title"`
	// Card is the card offered, absent when the state has nothing ready.
	Card *CardView `json:"card,omitempty"`
	// AwaitingOutside says the state waits on somebody outside the
	// workbench, so it offers nothing whatever is standing there. It tells a
	// reader an empty offer here means waiting rather than nothing ready.
	AwaitingOutside bool `json:"awaiting_outside,omitempty"`
}

// Next reports the card a state offers, and changes nothing. Offering a card
// is not assigning it: the owner reads what is next and claims it in a second
// command, which is the pull discipline of section 6.3.
func (l *Library) Next(req *Request) ([]Offer, error) {
	cards, err := l.Bench.Cards()
	if err != nil {
		return nil, err
	}
	states := l.Bench.States
	if req.State != "" {
		wanted := l.Bench.StateByRef(req.State)
		if wanted == nil {
			return nil, contract.Refuse(contract.UnknownState, req.State)
		}
		states = []*bench.State{wanted}
	}
	for _, card := range cards {
		if err := l.lapseRead(card); err != nil {
			return nil, err
		}
	}
	offers := make([]Offer, 0, len(states))
	for _, state := range states {
		offer := Offer{State: state.ID, Title: state.Title}
		// A waiting state offers nothing whatever is ready there, because a
		// claim would be refused, and an offer a claim refuses is an offer
		// nobody can take.
		if state.AwaitingOutside {
			offer.AwaitingOutside = true
		} else if head := headOfReady(state.ID, cards); head != nil {
			offer.Card = l.view(head)
		}
		offers = append(offers, offer)
	}
	return offers, nil
}

// headOfReady returns the next card pull (or next) would take from the given
// state, or nil if the state holds no ready card. The order is the queue
// order CORE-QUEUE-3 fixes, namely arrival into the current state first with
// the lower card identifier breaking a tie, and only ready cards are eligible.
//
// Reading the cards out of the bench's own latch-free snapshot means the
// returned card may have lapsed in between; both call sites re-read under the
// card's lock inside their own transaction, so a held card here is filtered
// again with `claim`'s substate test at the only moment it would matter. A
// pull that reaches a held or blocked card re-reads it under its lock and
// refuses accordingly.
func headOfReady(stateID string, cards []*bench.Card) *bench.Card {
	var ready []*bench.Card
	for _, card := range cards {
		if card.State == stateID && card.Substate == contract.SubstateReady {
			ready = append(ready, card)
		}
	}
	if len(ready) == 0 {
		return nil
	}
	sortByArrival(ready)
	return ready[0]
}

// Detail is a card and everything below it a reader asked to see.
type Detail struct {
	// Card is the card itself.
	Card CardView `json:"card"`
	// Body is the card's framing prose.
	Body string `json:"body"`
	// Links are what the card records about other cards.
	Links []LinkView `json:"links,omitempty"`
	// Attachments are the card's attachments in creation order.
	Attachments []AttachmentView `json:"attachments,omitempty"`
	// Comments are the card's comments in timestamp order.
	Comments []CommentView `json:"comments,omitempty"`
	// Path is the file the card lives in.
	Path string `json:"path"`
}

// AttachmentView is one attachment as a read reports it.
type AttachmentView struct {
	// ID is the attachment's identifier.
	ID string `json:"id"`
	// Ordinal is the attachment's one-based position among the attachments
	// of the entity it hangs from: the stored ordinal, or the
	// directory-order position on an attachment whose anchor carries none.
	Ordinal int `json:"ordinal"`
	// Ref is what a person types to name the attachment: the card's own
	// reference, then attachments and the attachment's ordinal. Resolved
	// here so a ref printed after a rename still names the attachment the
	// view describes.
	Ref string `json:"ref"`
	// Filename is the attachment's current filename.
	Filename string `json:"filename"`
	// Description is the optional prose describing the attachment, absent
	// when the anchor carries none.
	Description string `json:"description,omitempty"`
	// Provenance says where the bytes came from.
	Provenance string `json:"provenance"`
}

// LinkView is one link as a read reports it.
type LinkView struct {
	// Kind is the link's open-valued kind.
	Kind string `json:"kind"`
	// To is the identifier of the card the link names.
	To string `json:"to"`
	// Ref is what a person types to name that card on show or any other
	// reference-accepting command: its alias when the card can still be
	// found, the bare identifier otherwise. A link's own stored value never
	// changes (card.go's Link carries an identifier, not an alias, by
	// design), so this is resolved fresh on every read rather than stored.
	Ref string `json:"ref"`
}

// CommentView is one comment as a read reports it.
type CommentView struct {
	// ID is the comment's identifier.
	ID string `json:"id"`
	// TS is when it was written.
	TS string `json:"ts"`
	// Author is who wrote it.
	Author string `json:"author"`
	// Body is the comment itself.
	Body string `json:"body"`
}

// Show reads a card, or the file any other reference names.
//
// A card comes back as a Detail with an empty text. Every other reference
// comes back the other way round, as a nil Detail beside the text of the file
// it named, since nothing but a card has a view to build. A caller reads the
// pair rather than assuming the Detail.
func (l *Library) Show(req *Request) (*Detail, string, error) {
	head, rest, _ := strings.Cut(req.Card, "/")
	// A state is an entity of the workbench, and the containment walk prints
	// a reference for one, so show reads it the way path and edit do rather
	// than refusing over a reference the tool told the reader to type.
	if rest == "" {
		if state := l.Bench.StateByRef(head); state != nil {
			text, err := bench.ReadText(l.Bench.StateAnchorPath(state.ID))
			if err != nil {
				return nil, "", contract.Refuse(contract.UnknownPath, head)
			}
			return nil, text, nil
		}
	}
	// A composed reference is whatever the resolver reaches, which is why the
	// resolution comes before the card is loaded: the head may name the
	// workbench or a state rather than a card, and every one of those forms is
	// a reference the containment walk prints.
	if rest != "" {
		path, err := l.Bench.ResolvePath(req.Card)
		if err != nil {
			return nil, "", err
		}
		text, err := bench.ReadText(path)
		if err != nil {
			return nil, "", contract.Refuse(contract.UnknownPath, rest)
		}
		return nil, text, nil
	}
	found, err := l.Bench.ResolveCard(head)
	if err != nil {
		return nil, "", err
	}
	card := found.Card
	if err := l.lapseRead(card); err != nil {
		return nil, "", err
	}
	detail := &Detail{Card: *l.view(card), Body: card.Body, Path: card.AnchorPath()}
	for _, link := range card.Links {
		detail.Links = append(detail.Links, LinkView{Kind: link.Kind, To: link.To, Ref: l.linkRef(link.To)})
	}
	attachments, err := bench.Attachments(card.Dir)
	if err != nil {
		return nil, "", err
	}
	cardRef := card.Ref(l.Bench.Slug)
	for _, attachment := range attachments {
		ordinal := displayOrdinal(attachment)
		view := AttachmentView{
			ID:          attachment.ID,
			Ordinal:     ordinal,
			Ref:         attachmentRef(cardRef, ordinal),
			Filename:    attachment.Filename,
			Description: attachment.Description,
			Provenance:  attachment.Provenance,
		}
		detail.Attachments = append(detail.Attachments, view)
	}
	comments, err := bench.Comments(card.Dir)
	if err != nil {
		return nil, "", err
	}
	for _, comment := range comments {
		view := CommentView{ID: comment.ID, TS: comment.TS, Author: comment.Author, Body: comment.Body}
		detail.Comments = append(detail.Comments, view)
	}
	return detail, "", nil
}

// displayOrdinal is the one-based position a read reports for an attachment,
// and the only number the position column, the JSON view and the printed ref
// are built from.
//
// It counts the attachment's place in the sorted collection and never reads
// the anchor's stored ordinal, because a position and a stored ordinal are
// different things. resolve.go's position arm answers a reference by indexing
// SortByOrdinal(ListIDs(collection)), so a position is an index into that
// sequence. The stored ordinal is the sort key that orders the sequence, and
// NextOrdinal hands out highest-plus-one, so one delete leaves a permanent gap
// after which the two stop coinciding. Counting the place here makes the
// display, contents and the resolver agree by construction on a stamped
// collection, on an unstamped one, and on a gapped one alike.
//
// Zero means the attachment is not a member of the collection its directory
// sits in, which no caller can produce today, since Show's attachments come
// out of that same listing. It is reported rather than smoothed over, so that
// an unaddressable row shows up as one instead of pointing at the first file.
func displayOrdinal(attachment *bench.Attachment) int {
	collection := filepath.Dir(attachment.Dir)
	ids := bench.SortByOrdinal(collection, bench.AttachmentAnchor, bench.ListIDs(collection))
	for n, id := range ids {
		if id == attachment.ID {
			return n + 1
		}
	}
	return 0
}

// attachmentRef composes the reference a person types to reach one attachment
// from its card: the card's own reference, then attachments and the position
// the caller resolved through displayOrdinal.
func attachmentRef(cardRef string, ordinal int) string {
	return cardRef + "/" + bench.AttachmentsDir + "/" + strconv.Itoa(ordinal)
}

// linkRef resolves a link's stored card identifier to what a person types to
// reach it. A link records only the identifier (card.go's Link comment: "a
// declaration rather than an entity"), so the alias is resolved fresh on
// every read rather than carried by the link itself, the same way a card's
// own Ref is computed at view time rather than stored. A card the link names
// that is no longer findable, archived or otherwise, still shows something
// typeable: the bare identifier the link already carried.
func (l *Library) linkRef(id string) string {
	if card, err := bench.LoadCard(l.Bench.CardsRoot(), id); err == nil {
		return card.Ref(l.Bench.Slug)
	}
	if card, err := bench.LoadCard(l.Bench.ArchivedCardsRoot(), id); err == nil {
		return card.Ref(l.Bench.Slug)
	}
	return id
}

// History reports a card's recorded acts in the order they were recorded. An
// identifier carried in an act is never resolved against the bench as it now
// stands, so a state renamed after a move still reads under its old title.
func (l *Library) History(req *Request) ([]bench.Event, error) {
	found, err := l.Bench.ResolveCard(req.Card)
	if err != nil {
		return nil, err
	}
	events, _, err := bench.ReadJournal(found.Card.JournalPath())
	if err != nil {
		return nil, err
	}
	return events, nil
}

// Served is the instruction chain at a position, with the legal moves that
// travel alongside it when the position is a card's.
type Served struct {
	// Instructions are the three layers, never written into one another.
	Instructions Instructions `json:"instructions"`
	// LegalMoves are the departures legal for the card at this moment.
	LegalMoves []LegalMove `json:"legal_moves,omitempty"`
	// State is the state the instructions were served for.
	State string `json:"state"`
}

// Instructions serves the chain at a position named by a card or by a state.
func (l *Library) Instructions(req *Request) (*Served, error) {
	if state := l.Bench.StateByRef(req.Card); state != nil {
		served := &Served{
			State: state.ID,
			Instructions: Instructions{
				Global:   bench.GlobalInstructions(l.Home),
				Standing: l.Bench.Standing,
				State:    state.Instructions,
			},
		}
		return served, nil
	}
	found, err := l.Bench.ResolveCard(req.Card)
	if err != nil {
		return nil, contract.Refuse(contract.UnknownPath, req.Card)
	}
	served := &Served{
		State:        found.Card.State,
		Instructions: *l.serve(found.Card),
		LegalMoves:   l.legalMoves(found.Card),
	}
	return served, nil
}

// Identity is who the actor is and whether that owner is the operator.
type Identity struct {
	// Actor is the owner the ladder produced.
	Actor string `json:"actor"`
	// IsOperator answers CORE-OWNER-2 for this bench.
	IsOperator bool `json:"is_operator"`
	// Operator is the owner reserved acts belong to.
	Operator string `json:"operator,omitempty"`
}

// Whoami reports the resolved actor and whether it is the operator.
func (l *Library) Whoami(req *Request) (*Identity, error) {
	if req.Actor == "" {
		return nil, contract.Refuse(contract.NoOwner, "")
	}
	identity := &Identity{
		Actor:      req.Actor,
		IsOperator: req.Actor == l.Bench.Operator,
		Operator:   l.Bench.Operator,
	}
	return identity, nil
}

// CheckReport is what check answers with: the structural defects the bench
// carries, and the account of a repair the request asked for.
//
// The account is what keeps a repair from being silent. A migration that
// stamped a creation ordinal it could only guess at, or that a lock kept out of
// a card, is the only moment anybody can still tell a guess from a recovered
// fact, so it says so here rather than leaving a workbench that reads clean
// afterwards either way.
type CheckReport struct {
	// Findings are the defects the checker names, together with whatever a
	// repair in the same request could not do.
	Findings []bench.Finding `json:"findings"`
	// StampedOrdinals counts the creation ordinals the migration wrote, and
	// is absent from a request that did not ask for the migration.
	StampedOrdinals *int `json:"stamped_ordinals,omitempty"`
	// AssignedSlugs are the states the slug migration repaired with the slug
	// each one was given. It is absent from a request that asked for no
	// migration and from a request that asked and found nothing to repair,
	// which MigratedSlugs below is what separates.
	AssignedSlugs []bench.SlugAssignment `json:"assigned_slugs,omitempty"`
	// MigratedSlugs says the slug migration ran, so a caller can tell an
	// empty list of assignments from a migration nobody asked for.
	MigratedSlugs bool `json:"migrated_slugs,omitempty"`
	// AssignedWorkbenchSlug is the slug the workbench-slug migration derived
	// for the workbench itself, absent when the workbench already carried
	// one or when no migration was asked for.
	AssignedWorkbenchSlug *bench.WorkbenchSlugAssignment `json:"assigned_workbench_slug,omitempty"`
	// RemovedStrandedStates are the identifiers the states migration removed
	// from the workbench's own states list. It is absent from a request that
	// asked for no migration and from a request that asked and found nothing
	// to repair, which MigratedStates below is what separates.
	RemovedStrandedStates []string `json:"removed_stranded_states,omitempty"`
	// MigratedStates says the stranded-state migration ran, so a caller can
	// tell an empty list of removals from a migration nobody asked for.
	MigratedStates bool `json:"migrated_states,omitempty"`
	// AssignedWorkstreamSlugs are the workstreams the slug migration
	// repaired with the slug each one was given, on the terms AssignedSlugs
	// carries the states.
	AssignedWorkstreamSlugs []bench.WorkstreamSlugAssignment `json:"assigned_workstream_slugs,omitempty"`
	// AdoptedWorkstreams are the identifiers the adoption repair created a
	// workstream at, each one a membership the live cards already carried
	// that named nothing. It is absent from a request that asked for no
	// migration and from a request that asked and found nothing to adopt,
	// which MigratedWorkstreams below is what separates.
	AdoptedWorkstreams []string `json:"adopted_workstreams,omitempty"`
	// MigratedWorkstreams says the adoption repair ran, so a caller can tell
	// an empty list of adoptions from a migration nobody asked for.
	MigratedWorkstreams bool `json:"migrated_workstreams,omitempty"`
}

// Check checks the bench for structural defects, and repairs nothing unless a
// marker in the request asks it to.
//
// A request carrying the finish marker completes or rolls back the
// interrupted structural acts first, so nobody finishes an act without the
// report that named it, and then reports what the bench still carries. A
// request carrying the migrate-ordinals marker stamps the creation ordinals a
// workbench written before the field carries none of, which is a one-time
// repair rather than a read-path fallback. A request carrying the
// migrate-slugs marker does the same for the states of a workbench that
// predate the slug field, names the slug it gave each one, and derives the
// workbench's own slug when the workbench itself predates that field. A
// request carrying the migrate-states marker removes every stranded
// identifier from the workbench's own states list.
//
// A non-nil error return still carries a non-nil report when the migration
// ran: the report is what the run had already stamped and already guessed
// before whatever ended it, and a caller that discards it on the error path
// loses that account the same way the run it is reporting on must not.
func (l *Library) Check(req *Request) (*CheckReport, error) {
	report := &CheckReport{}
	if req != nil && req.MigrateSlugs {
		assigned, reported := l.Bench.BackfillStateSlugs()
		report.MigratedSlugs = true
		report.AssignedSlugs = assigned
		report.Findings = append(report.Findings, reported...)
		streamAssigned, streamReported := l.Bench.BackfillWorkstreamSlugs()
		report.AssignedWorkstreamSlugs = streamAssigned
		report.Findings = append(report.Findings, streamReported...)
		wsAssigned, wsReported, err := l.Bench.BackfillWorkbenchSlug()
		report.AssignedWorkbenchSlug = wsAssigned
		report.Findings = append(report.Findings, wsReported...)
		if err != nil {
			return report, err
		}
	}
	if req != nil && req.MigrateWorkstreams {
		adopted, err := l.adoptWorkstreams(req)
		report.MigratedWorkstreams = true
		report.AdoptedWorkstreams = adopted
		if err != nil {
			return report, err
		}
	}
	if req != nil && req.MigrateStates {
		removed, err := l.Bench.RemoveStrandedStates()
		report.MigratedStates = true
		report.RemovedStrandedStates = removed
		if err != nil {
			return report, err
		}
	}
	if req != nil && req.MigrateOrdinals {
		stamped, reported, err := l.Bench.BackfillOrdinals(req.Actor, bench.Stamp(l.Now()))
		report.StampedOrdinals = &stamped
		report.Findings = append(report.Findings, reported...)
		if err != nil {
			return report, err
		}
	}
	if req == nil || !req.Finish {
		findings, err := l.Bench.Check()
		if err != nil {
			return nil, err
		}
		report.Findings = append(report.Findings, findings...)
		return report, nil
	}
	unresolved, err := l.Bench.FinishInterrupted(req.Actor, bench.Stamp(l.Now()))
	if err != nil {
		return nil, err
	}
	remaining, err := l.Bench.Check()
	if err != nil {
		return nil, err
	}
	// What the finish would not resolve it has already described more
	// precisely than a second pass can, so its own finding is the one the
	// reader gets for that path.
	reported := map[string]bool{}
	for _, finding := range unresolved {
		reported[finding.Path] = true
	}
	report.Findings = append(report.Findings, unresolved...)
	for _, finding := range remaining {
		if reported[finding.Path] {
			continue
		}
		report.Findings = append(report.Findings, finding)
	}
	return report, nil
}

// adoptWorkstreams creates a workstream at every identifier the live cards
// list that names none, keeping the identifier so that no card file is touched
// and every reference already written down still resolves.
//
// It is a repair somebody asks for rather than one that runs at open, because
// a tool that mints entities nobody asked for is writing into a file it does
// not understand, and a workbench opened by accident would gain directories
// its owner never made.
//
// The workbench's own lock covers the run, which is what the writer of a new
// entity of a workbench-level collection already takes.
func (l *Library) adoptWorkstreams(req *Request) ([]string, error) {
	dangling, err := l.Bench.DanglingWorkstreams()
	if err != nil {
		return nil, err
	}
	if len(dangling) == 0 {
		return nil, nil
	}
	now := bench.Stamp(l.Now())
	lock, err := bench.Acquire(l.Bench.Root, req.Actor, now)
	if err != nil {
		return nil, err
	}
	defer lock.Release()
	var adopted []string
	for _, id := range dangling {
		workstream, err := l.Bench.AdoptWorkstream(id)
		if err != nil {
			return adopted, err
		}
		ev := bench.Event{TS: now, Event: contract.EventCreated, Actor: req.Actor}
		if err := bench.AppendEvent(workstream.JournalPath(), ev); err != nil {
			return adopted, err
		}
		adopted = append(adopted, id)
	}
	return adopted, nil
}

// Export writes the interchange form of the bench definition.
func (l *Library) Export() ([]byte, error) {
	return l.Bench.Export()
}

// Extract copies the bench's definition out as a template.
func (l *Library) Extract(target string) error {
	absolute, err := filepath.Abs(target)
	if err != nil {
		return err
	}
	return l.Bench.Extract(absolute)
}

// SettingView is one user setting as the listing reports it: what the key is,
// what the ladder resolved it to, and which rung of that ladder answered.
//
// Value carries no omitempty. A setting nothing set is a row a reader is
// owed, and dropping the member would leave the machine form unable to say
// so.
type SettingView struct {
	// Key is the setting's name.
	Key string `json:"key"`
	// Value is what the ladder resolved, empty when no rung carried one.
	Value string `json:"value"`
	// Source is the rung that answered, unset when none did and unknown for
	// a key the tool does not know.
	Source string `json:"source"`
}

// SettingsContext carries the per-invocation values Settings needs beyond
// the config file itself, one field per input some setting's own ladder
// reads. It exists so that adding a setting with a ladder of its own does not
// keep growing Settings' own parameter list past what a reader can hold.
type SettingsContext struct {
	// LangFlag and ActorFlag are the --lang and --actor flags this
	// invocation carried.
	LangFlag, ActorFlag string
	// WorkbenchFlag and WorkbenchEnv are --workbench and DINAH_WORKBENCH,
	// the two override rungs the workbench setting's own ladder reads ahead
	// of the search and the stored default.
	WorkbenchFlag, WorkbenchEnv string
	// GOOS and LookPath are what the editor ladder needs to test a fallback
	// binary's presence.
	GOOS     string
	LookPath func(string) bool
	// CWD, Home and NativeHome are what the workbench setting's ladder needs
	// to run the same discovery walk a real invocation would.
	CWD, Home, NativeHome string
}

// Settings reports every setting the tool knows, resolved through the ladder
// that actually decides each one, followed by whatever else the config file
// carries.
//
// The resolvers are called rather than restated, because the stored value a
// caller reads with `config get` cannot tell an unset key from one somebody
// set to the same word the default happens to use. ctx carries what this
// invocation supplies each ladder, so the listing reports the ladder as it
// stands for this run rather than for an imagined one.
//
// A key outside the tool's set is reported with its stored value and the
// source unknown. It survives every write, so hiding it here would leave a
// reader wondering why a setting they can see in the file does nothing.
func Settings(cfg *bench.Config, ctx SettingsContext) []SettingView {
	views := make([]SettingView, 0, len(bench.ConfigKeys))
	for _, key := range bench.ConfigKeys {
		views = append(views, setting(key, cfg, ctx))
	}
	for _, key := range cfg.Keys() {
		if bench.KnownConfigKey(key) {
			continue
		}
		views = append(views, SettingView{Key: key, Value: cfg.Get(key), Source: bench.SourceUnknown})
	}
	return views
}

// setting resolves one known key. A key this switch does not answer for is a
// key somebody added to ConfigKeys without giving it a ladder, so it falls
// back to the stored value, which is the one rung every setting has.
func setting(key string, cfg *bench.Config, ctx SettingsContext) SettingView {
	switch key {
	case "lang":
		value, source := bench.ResolveLangSource(ctx.LangFlag, cfg)
		return SettingView{Key: key, Value: value, Source: source}
	case "actor":
		value, source := bench.ResolveActorSource(ctx.ActorFlag, cfg)
		return SettingView{Key: key, Value: value, Source: source}
	case "editor":
		value, source, _ := bench.ResolveEditorSource(cfg, ctx.GOOS, ctx.LookPath)
		return SettingView{Key: key, Value: value, Source: source}
	case "workbench":
		value, source := bench.ResolveWorkbenchSource(
			ctx.CWD,
			ctx.WorkbenchFlag,
			ctx.WorkbenchEnv,
			ctx.Home,
			ctx.NativeHome,
			cfg.Get("workbench"),
		)
		return SettingView{Key: key, Value: value, Source: source}
	}
	stored := cfg.Get(key)
	if stored == "" {
		return SettingView{Key: key, Value: "", Source: bench.SourceUnset}
	}
	return SettingView{Key: key, Value: stored, Source: bench.SourceConfig}
}

// VersionReport is what this binary is and what it conforms to. The two numbers
// have two audiences and are never conflated: the tool's own release number
// says what you are running, and the conformance claim says what it promises.
type VersionReport struct {
	// Tool is this binary's own release number.
	Tool string `json:"tool"`
	// Profile is the conformance claim, carrying a major and a minor number
	// and no maturity channel.
	Profile string `json:"profile"`
	// Format is the storage format version the binary implements.
	Format int `json:"format"`
	// Catalogs are the shipped locales with their coverage, present only
	// when the caller asked for them.
	Catalogs []CatalogCoverage `json:"catalogs,omitempty"`
}

// CatalogCoverage is one shipped locale's key coverage against the base
// catalog, which is what version --catalogs reports.
type CatalogCoverage struct {
	// Tag is the locale tag.
	Tag string `json:"tag"`
	// Translated is the number of keys carrying a translation.
	Translated int `json:"translated"`
	// Present is the number of keys the catalog carries at all.
	Present int `json:"present"`
	// Total is the number of keys the base catalog carries.
	Total int `json:"total"`
}

// ToolRelease is this binary's own release number. It is not the profile
// version and is never conflated with it. A release build overwrites it with
// the tag it was built from, through -ldflags -X, which only reaches a
// variable; a build from source keeps the default below.
var ToolRelease = "0.1.0"

// Version reports what this binary is and what it conforms to, optionally
// with the coverage of every shipped catalog.
func Version(withCatalogs bool) *VersionReport {
	release := &VersionReport{
		Tool:    ToolRelease,
		Profile: bench.ProfileVersion,
		Format:  bench.StorageFormat,
	}
	if !withCatalogs {
		return release
	}
	for _, tag := range msg.Tags() {
		translated, present, total := msg.Coverage(tag)
		coverage := CatalogCoverage{
			Tag:        tag,
			Translated: translated,
			Present:    present,
			Total:      total,
		}
		release.Catalogs = append(release.Catalogs, coverage)
	}
	return release
}
