package verb

import (
	"path/filepath"
	"strings"

	"dinah/internal/bench"
	"dinah/internal/contract"
	"dinah/internal/msg"
)

// StateView is one state as a read reports it.
type StateView struct {
	// ID is the state's identifier.
	ID string `json:"id"`
	// Title is the state's title.
	Title string `json:"title"`
	// Kind is one of intake, work and done.
	Kind string `json:"kind"`
	// OperatorOwned marks a state only the operator moves a card out of.
	OperatorOwned bool `json:"operator_owned"`
	// Capacity is the declared limit, zero for unlimited.
	Capacity int `json:"capacity,omitempty"`
	// Count is the number of live cards the state holds.
	Count int `json:"count"`
}

// Status is where the bench stands and what the reader holds.
type Status struct {
	// Bench is the workbench's title.
	Bench string `json:"bench"`
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
}

// Status reports where the bench stands.
func (l *Library) Status(req *Request) (*Status, error) {
	cards, err := l.Bench.Cards()
	if err != nil {
		return nil, err
	}
	status := &Status{
		Bench:      l.Bench.Title,
		Root:       l.Bench.Root,
		Actor:      req.Actor,
		Operator:   l.Bench.Operator,
		IsOperator: req.Actor != "" && req.Actor == l.Bench.Operator,
		Profile:    l.Bench.Profile,
		Holding:    []CardView{},
		Blocked:    []CardView{},
	}
	counts := map[string]int{}
	for _, card := range cards {
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
			ID:            state.ID,
			Title:         state.Title,
			Kind:          state.Kind,
			OperatorOwned: state.OperatorOwned,
			Capacity:      state.Capacity,
			Count:         counts[state.ID],
		}
		views = append(views, view)
	}
	return views
}

// Listing is the cards of a state in queue order.
type Listing struct {
	// State is the state listed, empty when the listing spans the bench.
	State string `json:"state,omitempty"`
	// Cards are the cards, in the order CORE-QUEUE-1 fixes.
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
	offers := make([]Offer, 0, len(states))
	for _, state := range states {
		var ready []*bench.Card
		for _, card := range cards {
			if card.State == state.ID && card.Substate == contract.SubstateReady {
				ready = append(ready, card)
			}
		}
		sortByArrival(ready)
		offer := Offer{State: state.ID, Title: state.Title}
		if len(ready) > 0 {
			offer.Card = l.view(ready[0])
		}
		offers = append(offers, offer)
	}
	return offers, nil
}

// Detail is a card and everything below it a reader asked to see.
type Detail struct {
	// Card is the card itself.
	Card CardView `json:"card"`
	// Body is the card's framing prose.
	Body string `json:"body"`
	// Links are what the card records about other cards.
	Links []LinkView `json:"links,omitempty"`
	// Comments are the card's comments in timestamp order.
	Comments []CommentView `json:"comments,omitempty"`
	// Path is the file the card lives in.
	Path string `json:"path"`
}

// LinkView is one link as a read reports it.
type LinkView struct {
	// Kind is the link's open-valued kind.
	Kind string `json:"kind"`
	// To is the identifier of the card the link names.
	To string `json:"to"`
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

// Show reads a card, or the file anything below it names.
func (l *Library) Show(req *Request) (*Detail, string, error) {
	head, rest, _ := strings.Cut(req.Card, "/")
	found, err := l.Bench.ResolveCard(head)
	if err != nil {
		return nil, "", err
	}
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
	card := found.Card
	if err := l.lapse(card); err != nil {
		return nil, "", err
	}
	detail := &Detail{Card: *l.view(card), Body: card.Body, Path: card.AnchorPath()}
	for _, link := range card.Links {
		detail.Links = append(detail.Links, LinkView{Kind: link.Kind, To: link.To})
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

// Fsck checks the bench for structural defects.
func (l *Library) Fsck() ([]bench.Finding, error) {
	return l.Bench.Fsck()
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
// version and is never conflated with it.
const ToolRelease = "0.1.0"

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
