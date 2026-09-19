package verb

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"dinah/internal/bench"
	"dinah/internal/contract"
	"dinah/internal/msg"
)

// ColumnView is one column as a read reports it.
type ColumnView struct {
	// ID is the column's identifier.
	ID string `json:"id"`
	// Slug is the column's short handle. It is absent on a column written
	// before the field existed and left out until the migration runs.
	Slug string `json:"slug,omitempty"`
	// Title is the column's title.
	Title string `json:"title"`
	// Kind is one the profile declares, which is intake, work or done, or
	// one carrying a layer's prefix. TakesWorkUp answers the question a
	// reader of this field usually means.
	Kind string `json:"kind"`
	// OperatorOwned marks a column only the operator moves a card out of.
	OperatorOwned bool `json:"operator_owned"`
	// AwaitingOutside marks a column where the workbench waits on somebody
	// who is not an owner of it, so no owner takes work up there. It
	// answers a different question from OperatorOwned, which is about
	// departure alone, and a column may carry both.
	AwaitingOutside bool `json:"awaiting_outside"`
	// TakesWorkUp says an owner takes work up at this column. A reader that
	// wants the fact reads it here rather than deriving it from Kind, which is
	// the whole point of publishing it.
	TakesWorkUp bool `json:"takes_work_up"`
	// PullDestination is the column a pull standing at this column would
	// carry a card into, absent where no pull could carry it anywhere: a
	// done column, a column waiting on somebody outside, a work column whose
	// immediate downstream does not itself take work up, or a run of queues
	// meeting any of those before it reaches a column that does. It answers
	// the other half of the question TakesWorkUp exists for, which is why it
	// stands beside it.
	//
	// It is carriesInto's own answer rendered as a reference rather than a
	// second copy of the rule, so a reader that draws the act and a pull
	// that performs it can never disagree about where the card lands. A
	// reader could walk the flow itself from the fields above, and a reader
	// that does holds a copy of this rule that goes stale the next time the
	// walk changes.
	PullDestination string `json:"pull_destination,omitempty"`
	// Capacity is the declared limit, zero for unlimited.
	Capacity int `json:"capacity,omitempty"`
	// RequireFields are the declared field keys a card must hold a value for
	// before it enters this column, in the column's own declaration order,
	// which is the order the refusal names them in. It is absent on a column
	// declaring none.
	RequireFields []string `json:"require_fields,omitempty"`
	// Hold is the column's item hold in the words a person types, which is
	// on, out or both, and absent on a column holding neither way. It is
	// bench.Column.Hold unchanged. Open already parses gate_items into that
	// vocabulary and refuses anything else, so this member translates
	// nothing, and a translation put here would be the second one.
	//
	// A client needs it to say which way an item filed against this column
	// would hold, and the alternative is a client-side copy of the rule
	// that goes stale the next time the rule changes, which is the argument
	// PullDestination above already carries.
	Hold string `json:"hold,omitempty"`
	// Fields are the values the column itself carries for the fields its
	// workbench declares, on the terms CardView.Fields carries a card's.
	Fields map[string]string `json:"fields,omitempty"`
	// RejectTo is the column a card goes to when the work at this column is
	// refused, empty where the column declares no such destination. It is
	// published as the reference the declaration carries rather than as a
	// resolved identifier, because a reference naming no column opens the
	// workbench anyway and a reader is owed what was written. Whether the
	// reference resolves is `dinah check`'s question, under
	// check.reject-target-unknown.
	RejectTo string `json:"reject_to,omitempty"`
	// Count is the number of live cards the column holds.
	Count int `json:"count"`
	// AttachmentCount is how many attachments hang from the column itself,
	// which is a rubric or a reference document somebody attached to the
	// station rather than to any card standing at it.
	AttachmentCount int `json:"attachment_count,omitempty"`
	// CommentCount is how many comments hang from the column itself,
	// which is a note somebody left on the station rather than on any card
	// standing at it.
	CommentCount int `json:"comment_count,omitempty"`
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
	// Columns are the flow with each station's occupancy.
	Columns []ColumnView `json:"columns"`
	// Holding are the cards this actor holds right now.
	Holding []CardView `json:"holding"`
	// Blocked are the cards waiting on the operator.
	Blocked []CardView `json:"blocked"`
	// WorkbenchSource names which rung resolved the active workbench for
	// this invocation: flag, environment, search, or config.
	WorkbenchSource string `json:"workbench_source,omitempty"`
	// AttachmentCount is how many attachments hang from the workbench
	// itself, sibling to the counts each column and each card now carries.
	AttachmentCount int `json:"attachment_count,omitempty"`
}

// Status reports where the bench stands.
func (l *Library) Status(req *Request) (*Status, error) {
	cards, err := l.Bench.Cards()
	if err != nil {
		return nil, err
	}
	benchAttachments, err := bench.CountAttachments(l.Bench.Root)
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
		AttachmentCount: benchAttachments,
	}
	counts := map[string]int{}
	for _, card := range cards {
		if err := l.lapseRead(card, req.Actor); err != nil {
			return nil, err
		}
		counts[card.Column]++
		view, err := l.view(card)
		if err != nil {
			return nil, err
		}
		if card.Holder != "" && card.Holder == req.Actor {
			status.Holding = append(status.Holding, *view)
		}
		if card.State == contract.StateBlocked {
			status.Blocked = append(status.Blocked, *view)
		}
	}
	columns, err := l.columnViews(counts)
	if err != nil {
		return nil, err
	}
	status.Columns = columns
	return status, nil
}

// Columns reports the flow in order, which is the order of the list in
// workbench.md frontmatter and the single authority for it.
func (l *Library) Columns() ([]ColumnView, error) {
	cards, err := l.Bench.Cards()
	if err != nil {
		return nil, err
	}
	counts := map[string]int{}
	for _, card := range cards {
		counts[card.Column]++
	}
	return l.columnViews(counts)
}

// columnViews renders the flow with each station's occupancy.
func (l *Library) columnViews(counts map[string]int) ([]ColumnView, error) {
	views := make([]ColumnView, 0, len(l.Bench.Columns))
	for _, column := range l.Bench.Columns {
		attachments, err := bench.CountAttachments(l.Bench.ColumnDir(column.ID))
		if err != nil {
			return nil, err
		}
		comments, err := bench.CountComments(l.Bench.ColumnDir(column.ID))
		if err != nil {
			return nil, err
		}
		view := ColumnView{
			ID:              column.ID,
			Slug:            column.Slug,
			Title:           column.Title,
			Kind:            column.Kind,
			OperatorOwned:   column.OperatorOwned,
			AwaitingOutside: column.AwaitingOutside,
			TakesWorkUp:     column.TakesWorkUp(),
			Capacity:        column.Capacity,
			RejectTo:        column.RejectTo,
			Count:           counts[column.ID],
			AttachmentCount: attachments,
			CommentCount:    comments,
			RequireFields:   column.RequireFields,
			Hold:            column.Hold,
			Fields:          l.declaredFieldValues(column.FM, bench.KindColumn),
		}
		if destination := carriesInto(column, l.Bench.Columns); destination != nil {
			view.PullDestination = columnRef(destination)
		}
		views = append(views, view)
	}
	return views, nil
}

// Listing is the cards of a column in queue order.
type Listing struct {
	// Column is the column listed, empty when the listing spans the bench.
	Column string `json:"column,omitempty"`
	// Cards are the cards, in the order CORE-QUEUE-3 fixes.
	Cards []CardView `json:"cards"`
}

// List presents a column's cards in the profile's fixed order: earliest
// arrival first, ties broken by ascending identifier. A tool may offer other
// orders beside it, and this one stays available.
func (l *Library) List(req *Request) (*Listing, error) {
	cards, err := l.Bench.Cards()
	if err != nil {
		return nil, err
	}
	listing := &Listing{Cards: []CardView{}}
	var wanted *bench.Column
	if req.Column != "" {
		wanted = l.Bench.ColumnByRef(req.Column)
		if wanted == nil {
			return nil, contract.Refuse(contract.UnknownColumn, req.Column)
		}
		listing.Column = wanted.ID
	}
	var kept []*bench.Card
	for _, card := range cards {
		if wanted != nil && card.Column != wanted.ID {
			continue
		}
		if err := l.lapseRead(card, req.Actor); err != nil {
			return nil, err
		}
		if req.ReadyOnly && card.State != contract.StateReady {
			continue
		}
		kept = append(kept, card)
	}
	sortByArrival(kept)
	for _, card := range kept {
		view, err := l.view(card)
		if err != nil {
			return nil, err
		}
		listing.Cards = append(listing.Cards, *view)
	}
	return listing, nil
}

// Offer is the card a column offers next, or the absence of one.
type Offer struct {
	// Column is the column the offer concerns.
	Column string `json:"column"`
	// Title is that column's title.
	Title string `json:"title"`
	// Card is the card offered, absent when the column has nothing ready.
	Card *CardView `json:"card,omitempty"`
	// AwaitingOutside says the column waits on somebody outside the
	// workbench, so it offers nothing whatever is standing there. It tells a
	// reader an empty offer here means waiting rather than nothing ready.
	AwaitingOutside bool `json:"awaiting_outside,omitempty"`
	// NoTaker says no act can take a card up from this column, so it offers
	// nothing whatever is standing there. AwaitingOutside beside it says the
	// same thing and says who the workbench is waiting on, so a column carrying
	// the flag sets both.
	NoTaker bool `json:"no_taker,omitempty"`
	// TakenByPull says the card offered leaves by a pull into the column beyond
	// rather than by a claim here, because nobody takes work up where it
	// stands. A reader that acts on an offer needs to know which act to use.
	TakenByPull bool `json:"taken_by_pull,omitempty"`
	// AboveTier says the column, or for a column a pull would carry its card
	// through, the column beyond, holds ready work, and that none of it the
	// caller's declared tier is admitted for. It is mutually exclusive with
	// Card, and it is a different answer from an offer of nothing at all: a
	// reader acting on AboveTier knows there is work here waiting on a more
	// senior caller, where a reader of an empty offer carrying no flag knows
	// there is nothing waiting on anyone yet.
	AboveTier bool `json:"above_tier,omitempty"`
	// RequiredTier is the tier the withheld work requires at the column the
	// act would land it in, and SatisfiedBy is every entry of the workbench's
	// table at or above that tier, in levels.tier order from the lowest
	// satisfying rung upward, so the cheapest model that would do the work is
	// first. Both are present only on an offer carrying AboveTier, because on
	// any other offer there is nothing being withheld to explain.
	RequiredTier string      `json:"required_tier,omitempty"`
	SatisfiedBy  []ModelView `json:"satisfied_by,omitempty"`
}

// ModelView is one entry of a workbench's tier table as an offer carries it.
type ModelView struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
	Server   string `json:"server,omitempty"`
}

// modelViews renders the entries at or above one rung in the shape an offer
// carries them.
func modelViews(models []bench.TierModel) []ModelView {
	views := make([]ModelView, 0, len(models))
	for _, model := range models {
		views = append(views, ModelView{Provider: model.Provider, Model: model.Model, Server: model.Server})
	}
	return views
}

// Next reports the card a column offers, and changes nothing. Offering a card
// is not assigning it: the owner reads what is next and claims it in a second
// command, which is the pull discipline of section 6.3.
func (l *Library) Next(req *Request) ([]Offer, error) {
	cards, err := l.Bench.Cards()
	if err != nil {
		return nil, err
	}
	columns := l.Bench.Columns
	if req.Column != "" {
		wanted := l.Bench.ColumnByRef(req.Column)
		if wanted == nil {
			return nil, contract.Refuse(contract.UnknownColumn, req.Column)
		}
		columns = []*bench.Column{wanted}
	}
	for _, card := range cards {
		if err := l.lapseRead(card, req.Actor); err != nil {
			return nil, err
		}
	}
	offers := make([]Offer, 0, len(columns))
	for _, column := range columns {
		offer := Offer{Column: column.ID, Title: column.Title}
		// A column offers its head card when some act could take that card up,
		// and offers nothing when none could. A claim could take it where the
		// column takes work up. A pull could take it where carriesInto names a
		// column to carry it to, which is the function the pull itself reads,
		// so the offer and the act cannot disagree.
		//
		// The lookup reads the whole flow rather than the columns this request
		// reports, since a named column's downstream is a fact about the
		// workbench rather than about the request.
		beyond := carriesInto(column, l.Bench.Columns)
		byPull := !column.TakesWorkUp() && beyond != nil
		if !column.TakesWorkUp() && !byPull {
			offer.NoTaker = true
			offer.AwaitingOutside = column.AwaitingOutside
		} else {
			// The tier the card has to meet is the tier of the column the act
			// would land it in, which is this column for a claim and the
			// column beyond for a pull. Reading it anywhere else would offer
			// work a claim here then refuses, or withhold work on the strength
			// of a requirement nothing is about to check.
			landing := column
			if byPull {
				landing = beyond
			}
			head, hadReady, withheld := headOfReadyForTier(l.Bench, column.ID, landing, cards, selectionAdmission(l.Bench, req))
			switch {
			case head != nil:
				view, err := l.view(head)
				if err != nil {
					return nil, err
				}
				offer.Card = view
				offer.TakenByPull = byPull
			case hadReady:
				offer.AboveTier = true
				offer.RequiredTier = withheld
				offer.SatisfiedBy = modelViews(l.Bench.SatisfyingModels(withheld))
			}
		}
		offers = append(offers, offer)
	}
	return offers, nil
}

// admission is what selection measures a ready card against: what the caller
// declared, and whether any requirement governs the act being selected for.
//
// The two are separate values because an empty declaration already means
// something else. TierAdmission reads it as a caller that cannot be shown to
// meet any floor and admits it nowhere a requirement applies, which is the
// strict reading the claim gate asks for. An act no requirement can refuse
// therefore cannot be described by clearing the declaration, since that is the
// most selective input rather than the least, and it has to say instead that
// nothing filters it.
type admission struct {
	declared string
	filters  bool
}

// admits reports whether the act this admission describes could take the card
// at landing. An act nothing filters takes the first ready card without
// reading any requirement.
func (a admission) admits(b *bench.Bench, card *bench.Card, landing *bench.Column) bool {
	if !a.filters {
		return true
	}
	admitted, _ := b.TierAdmission(card, landing, a.declared)
	return admitted
}

// selectionAdmission answers what selection filters an invocation's ready
// cards by, and it follows the claim gate rather than restating it.
//
// A pull that takes the card up is a claim, so selection filters by what the
// caller declared and the card an offer shows is a card the gate admits. A
// pull carrying --no-claim leaves the card ready and takes nothing up, so
// pull's own gate does not run claimableTier on it at all: the call sits
// inside pull's `if !req.NoClaim`, under a comment saying that no requirement
// the card carries can refuse such a pull. Selection makes the same exemption
// here. Filtering anyway would withhold cards the gate was never going to
// refuse, and would answer a caller that declared no tier that ready work
// stands above one.
//
// A caller passing --no-claim is therefore answered as a caller whose model
// the gate never reads. What the caller is describes a claimant, and this
// invocation does not claim.
//
// next carries no --no-claim of its own, so it filters wherever the gate does.
// What it reports is what a claim or a claiming pull would take, which is the
// act the gate governs.
func selectionAdmission(b *bench.Bench, req *Request) admission {
	resolved, _ := b.TierOf(req.Provider, req.Model, req.Server)
	// The three exemptions the gate takes are taken here too, so the offer and
	// the gate cannot disagree about a card. A pull carrying --no-claim takes
	// nothing up. A workbench declaring no tiers block refuses no claim on
	// tier grounds. The operator is admitted at every tier whatever he
	// declares.
	filters := !req.NoClaim && b.DeclaresTierTable()
	if req.Actor != "" && req.Actor == b.Operator {
		filters = false
	}
	return admission{declared: resolved, filters: filters}
}

// headOfReadyForTier returns the card pull (or next) would take from the
// given column, or nil when nothing there is available to the caller. The
// order is the queue order CORE-QUEUE-3 fixes, namely arrival into the
// current column first with the lower card identifier breaking a tie, and
// only ready cards are eligible. Nothing is reordered by what a card
// requires: the scan runs down the arrival order and stops at the first card
// the declared tier is admitted for, so a caller's own eligible run reaches
// it in the order it arrived.
//
// landing is the column this scan is being run to decide whether a claim or
// a pull could take the card into, and the admission is read there. It
// differs from columnID when the scan crosses a buffer a pull would carry
// the card through, which is the same distinction TakenByPull already
// reports.
//
// Reading the cards out of the bench's own latch-free snapshot means the
// returned card may have lapsed in between; every call site re-reads under
// the card's lock inside its own transaction, so a held card here is
// filtered again with `claim`'s state test at the only moment it would
// matter. A pull that reaches a held or blocked card re-reads it under its
// lock and refuses accordingly.
//
// hadReady says whether the column held any ready card at all, admitted or
// not, and it is what lets a caller tell a queue with nothing in it from a
// queue whose work stands above the tier the caller resolved to. Those are
// different answers to a reader deciding what to do next, so nothing here
// collapses them into a nil card.
//
// The third answer is the tier the first withheld card requires at the landing
// column, which is what an offer and a refusal name so that a caller reads the
// obstacle rather than deriving it. It is the first in arrival order rather
// than the highest, because that is the card the caller would have been handed
// had it resolved high enough.
//
// The declaration is self-reported and nothing verifies it, exactly as it is
// on a claim. This filters what a caller is shown; it establishes nothing
// about the caller. Where the act being selected for is one no requirement can
// refuse, by carries no filter and every ready card is eligible.
func headOfReadyForTier(b *bench.Bench, columnID string, landing *bench.Column, cards []*bench.Card, by admission) (*bench.Card, bool, string) {
	var ready []*bench.Card
	for _, card := range cards {
		if card.Column == columnID && card.State == contract.StateReady {
			ready = append(ready, card)
		}
	}
	if len(ready) == 0 {
		return nil, false, ""
	}
	sortByArrival(ready)
	withheld := ""
	for _, card := range ready {
		if by.admits(b, card, landing) {
			return card, true, ""
		}
		if withheld == "" {
			withheld = card.RequiredTier(b, landing)
		}
	}
	return nil, true, withheld
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
	// Checklist are the card's checklist items in creation order.
	Checklist []ItemView `json:"checklist,omitempty"`
	// Path is the file the card lives in.
	Path string `json:"path"`
	// Withheld names the members this answer did not carry because the
	// caller named a field set that left them out. A name here is a positive
	// statement rather than a silence: it says the card holds that member and
	// this answer does not carry it. A member that is neither carried nor
	// named here is empty on the card.
	//
	// The spelling is the one Instructions.Withheld already uses for the same
	// shape of fact, so an agent that has learned to read a withheld
	// instruction layer reads a withheld card member without learning
	// anything new.
	Withheld []string `json:"withheld,omitempty"`
	// Reread is what a caller passes back to show, with the fields it now
	// wants, to be served a withheld member. It is the card's own reference,
	// and it is present exactly when Withheld is non-empty.
	Reread string `json:"reread,omitempty"`
	// selected is the field set this answer was shaped by, nil on an
	// unshaped answer. It is unexported, so it reaches no payload and no
	// caller outside this package. MarshalJSON needs it because a member the
	// caller left out and a member the card carries empty are the same value
	// once the answer is built, and the payload has to tell them apart.
	selected detailSelection
	// narrowedBy are the filter flags that dropped an entry this card holds,
	// spelled as a caller types them. It is unexported for the reason
	// selected is: it reaches no payload, because a machine reader recovers
	// by reading the reference again and a fresh call carries no flag from
	// the last one. A person at a terminal is the reader who needs it, since
	// the sentence under the announcement would otherwise tell them to name
	// a member they have already named.
	narrowedBy []string
}

// MarshalJSON writes the members this answer carries and no others.
//
// Three of Detail's members are absent from a payload whenever they are empty,
// because their own tags say so. The other three are present whatever they
// hold: every unshaped answer carries a card view, a body, and a path, and a
// reader of the wire format may rely on that. A shaped answer has to leave a
// member the caller did not ask for out rather than carry it empty, since
// carrying it empty is the claim withheld exists to deny, so those three are
// written through pointers here and a nil pointer is a member this answer does
// not carry.
//
// An unshaped answer sets all three, so its payload is byte for byte the
// payload this type produced before the field list existed.
//
// carried is a second declaration of the same wire format and nothing in the
// language ties it to Detail, so a member added above and forgotten here would
// be dropped from every payload this type writes. Deriving the payload from
// Detail by reflection would remove the second declaration and would also
// reorder the members, since the encoder writes a map in sorted key order and
// this wire format is frozen. The copy therefore stays and a guard stands over
// it: TestTheDetailPayloadCarriesEveryMemberDetailDeclares marshals a Detail
// with every member filled and fails on any name that reaches the type and not
// the payload, or the payload and not the type.
func (d Detail) MarshalJSON() ([]byte, error) {
	type carried struct {
		Card        *CardView        `json:"card,omitempty"`
		Body        *string          `json:"body,omitempty"`
		Links       []LinkView       `json:"links,omitempty"`
		Attachments []AttachmentView `json:"attachments,omitempty"`
		Comments    []CommentView    `json:"comments,omitempty"`
		Checklist   []ItemView       `json:"checklist,omitempty"`
		Path        *string          `json:"path,omitempty"`
		Withheld    []string         `json:"withheld,omitempty"`
		Reread      string           `json:"reread,omitempty"`
	}
	answer := carried{
		Links:       d.Links,
		Attachments: d.Attachments,
		Comments:    d.Comments,
		Checklist:   d.Checklist,
		Withheld:    d.Withheld,
		Reread:      d.Reread,
	}
	if d.selected.carries("card") {
		card := d.Card
		answer.Card = &card
	}
	if d.selected.carries("body") {
		body := d.Body
		answer.Body = &body
	}
	if d.selected.carries("path") {
		path := d.Path
		answer.Path = &path
	}
	return json.Marshal(answer)
}

// Carries reports whether this answer carries the named member. A renderer
// outside this package asks it before drawing a member, and an unshaped answer
// answers yes to every name.
//
// A renderer cannot read the answer's own emptiness instead. Three of Detail's
// members are slices or strings, so a member left out reads as nothing and
// draws nothing, but the card view is a struct, and a card view the caller
// left out is a zero struct that draws a header with an empty reference, an
// empty title, and an empty column. The payload tells the two apart through
// MarshalJSON, and this asks the same question from the terminal's side so
// that the two heads cannot disagree about what an answer holds.
func (d Detail) Carries(name string) bool {
	return d.selected.carries(name)
}

// NarrowedBy reports the filter flags that dropped an entry this card holds,
// spelled as a caller types them. The terminal reads it to decide which
// recovery it can honestly offer: a member the field list left out is served
// by naming it, and a member a filter narrowed is served by dropping the
// flag, and a reader told to do the first when the second is what they need
// has been sent to write a call they have already written.
func (d Detail) NarrowedBy() []string {
	if len(d.narrowedBy) == 0 {
		return nil
	}
	flags := make([]string, len(d.narrowedBy))
	copy(flags, d.narrowedBy)
	return flags
}

// DetailFields are the members of a Detail, in the order withheld reports
// them. A member added to Detail that a caller may ask for is added here in
// the position the JSON payload prints it.
//
// A name a caller may write is not always a member of the payload.
// DetailSelectors is the set the fields argument accepts, and it is what the
// vocabulary table, the refusal sentence and the selection all read, so the
// help table, the refusal sentence and the selection cannot drift apart.
var DetailFields = []string{"card", "body", "links", "attachments", "comments", "checklist", "path"}

// DetailModifiers are the names show's fields argument accepts that are not
// members of a Detail. Each one names a member and asks for that member in
// full rather than as an index, so each is a depth rather than a member.
var DetailModifiers = []string{"comments.full", "checklist.full"}

// DetailSelectors are every name the fields argument accepts, in the order
// withheld reports them: the members of a Detail, with each modifier inserted
// after the member it names.
var DetailSelectors = []string{"card", "body", "links", "attachments",
	"comments", "comments.full", "checklist", "checklist.full", "path"}

// modifierSuffix is what a selector carries beyond the member's own name when
// it asks for that member's bodies. It is syntax a caller types rather than
// prose, so it lives here rather than in a catalogue.
const modifierSuffix = ".full"

// modifierFor is the selector that asks for one member in full.
func modifierFor(member string) string { return member + modifierSuffix }

// baseOfModifier is the member a modifier names.
func baseOfModifier(modifier string) string {
	return strings.TrimSuffix(modifier, modifierSuffix)
}

// detailSelection is the set of members one answer carries, or nil where the
// caller named no field set and the answer carries every member.
type detailSelection map[string]bool

// carries reports whether a member belongs in the answer. A nil selection is
// the unshaped read, which carries everything.
func (s detailSelection) carries(name string) bool {
	if s == nil {
		return true
	}
	return s[name]
}

// full reports whether the bodies of a collection member were asked for. A nil
// selection is the unshaped read, which carries each collection as an index,
// so it answers false for every name.
func (s detailSelection) full(name string) bool {
	if s == nil {
		return false
	}
	return s[modifierFor(name)]
}

// The two flag words show's filters are typed as. They are spelled here as a
// reader types them, because a refusal's detail names what the reader wrote
// rather than a field of a request, which is the rule list.go's own flag
// constants already follow.
const (
	flagSince      = "--since"
	flagUnresolved = "--unresolved"
)

// detailFilters are the two narrowings a caller may put on show, already
// parsed. sinceSet tells an ordinal of zero, which serves every body, apart
// from no ordinal at all, which serves none.
type detailFilters struct {
	since      int
	sinceSet   bool
	unresolved bool
}

// named reports whether the caller wrote either filter.
func (f detailFilters) named() bool { return f.sinceSet || f.unresolved }

// flagWord is the filter a refusal about this call names: the first one the
// caller wrote, in the order show's own parameters declare them.
func (f detailFilters) flagWord() string {
	if f.sinceSet {
		return flagSince
	}
	if f.unresolved {
		return flagUnresolved
	}
	return ""
}

// parseDetailFilters reads show's two filter arguments off the request.
//
// An ordinal that is not a non-negative whole number is refused dinah.usage,
// which is the name this tool already raises over an argument it declares and
// this call may not carry. An ordinal larger than the number of comments the
// card holds is not refused, on the card's own decision: a station polling
// with an ordinal it remembers must not be turned away for having remembered
// a comment that has since been deleted.
func parseDetailFilters(req *Request) (detailFilters, error) {
	filters := detailFilters{unresolved: req.Unresolved}
	written := strings.TrimSpace(req.SinceComment)
	if written == "" {
		return filters, nil
	}
	ordinal, err := strconv.Atoi(written)
	if err != nil || ordinal < 0 {
		return detailFilters{}, contract.Refuse(contract.Usage, flagSince+" "+written)
	}
	filters.since, filters.sinceSet = ordinal, true
	return filters, nil
}

// refuseDetailFilter composes the refusal a filter raises where the answer it
// shapes is not one this call carries. The detail is the flag word alone
// where the whole of the fault is that the flag was written, and it names the
// combination where each half is legal and the pair is not, which is the
// shape refuseListFlag already composes for list's own flags.
func refuseDetailFilter(detail string) error {
	return contract.Refuse(contract.Usage, detail)
}

// checkDetailFilters refuses a filter that shapes a member this answer would
// not carry. Dinah's stated rule is that an argument a tool would accept and
// then drop tells the caller a narrowing ran when none did, so the surface
// turns the call away instead of answering a question nobody asked.
//
// The two questions are asked in the order a reader meets them. A filter
// whose member the field list leaves out is the plainer mistake, and the
// refusal names the flag alone. A filter meeting the modifier of its own
// member is the pair, since both names are legal and only the combination is
// not, so that detail names the combination.
func checkDetailFilters(chosen detailSelection, filters detailFilters) error {
	if chosen == nil {
		return nil
	}
	if filters.sinceSet && !chosen.carries("comments") {
		return refuseDetailFilter(flagSince)
	}
	if filters.unresolved && !chosen.carries("checklist") {
		return refuseDetailFilter(flagUnresolved)
	}
	if filters.sinceSet && chosen.full("comments") {
		return refuseDetailFilter(flagSince + " beside " + modifierFor("comments"))
	}
	return nil
}

// parseDetailFields reads show's fields argument into the members the answer
// is to carry. The argument absent, empty, or blank means every member, which
// is what show has always returned, so an unshaped call is untouched by this
// whole mechanism.
//
// A modifier sets its own name and the member it names, so a selection built
// from `comments.full` answers true to carries("comments") as well: the full
// form is the index plus the bodies rather than a second member, and naming
// both names is the same as naming the modifier alone.
//
// Surrounding whitespace on each name is ignored and a repeated name carries
// its member once. Every unrecognised name is reported, sorted, rather than
// the first one the loop reaches, which is the rule checkArguments already
// holds for argument names, and the refusal is composed before any card is
// read.
func parseDetailFields(fields string) (detailSelection, error) {
	if strings.TrimSpace(fields) == "" {
		return nil, nil
	}
	declared := map[string]bool{}
	for _, name := range DetailSelectors {
		declared[name] = true
	}
	chosen := detailSelection{}
	unknown := map[string]bool{}
	for _, written := range strings.Split(fields, ",") {
		name := strings.TrimSpace(written)
		if name == "" {
			continue
		}
		if !declared[name] {
			unknown[name] = true
			continue
		}
		chosen[name] = true
		if base := baseOfModifier(name); base != name {
			chosen[base] = true
		}
	}
	if len(unknown) > 0 {
		named := make([]string, 0, len(unknown))
		for name := range unknown {
			named = append(named, name)
		}
		sort.Strings(named)
		return nil, unknownDetailField(strings.Join(named, ", "), "")
	}
	// A list that survives the split naming nothing at all is refused rather
	// than answered. `--fields ,` reaches here, and the answer to it would
	// carry no member of the card whatsoever, under an announcement listing
	// everything the card holds, which is an answer nobody asks for. Blank
	// stays the unshaped read the paragraph above declares it to be, because
	// an argument with nothing in it is an argument the caller did not give,
	// where an argument carrying a separator is a list and a list names a
	// member.
	if len(chosen) == 0 {
		return nil, unknownDetailField(strings.TrimSpace(fields), "")
	}
	return chosen, nil
}

// unknownDetailField raises dinah.unknown-field for show's fields argument. It
// raises that name rather than a new one because the reader's mistake is the
// one the name already covers: a field this tool does not have, named where a
// field was asked for.
//
// The declared set rides as a value read off DetailSelectors rather than
// written into the catalog, so a name a caller may write reaches the sentence
// without a translator being asked for anything.
//
// The base sentence says which read was refused and what a card carries, and
// it says neither of the two things that are true at only one of the raise
// sites, because a sentence that named the unrecognised names would be false
// where the names are all legal and the reference is not a card. Each of those
// rides a fragment switched on by its own value. reference is filled where the
// refusal is about the reference, and unknown is filled where it is about the
// names, so the two are never both set and the composed sentence carries the
// one clause that holds. unknown repeats the refusal's own detail because a
// fragment renders on the presence of a named value and the detail is set at
// every raise site, so the detail cannot be the condition that tells the two
// sites apart.
func unknownDetailField(detail, reference string) error {
	extra := map[string]string{"fields": strings.Join(DetailSelectors, ", ")}
	if reference != "" {
		extra["reference"] = reference
	} else {
		extra["unknown"] = detail
	}
	return contract.RefuseWith(contract.UnknownField, detail, extra)
}

// AttachmentView is one attachment as a read reports it.
type AttachmentView struct {
	// ID is the attachment's identifier.
	ID string `json:"id"`
	// Ordinal is the attachment's one-based position among the attachments
	// of the entity it hangs from: its place in that collection once the
	// collection is sorted, which displayOrdinal counts rather than reading
	// the stored ordinal off the anchor.
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
	// Path is the absolute path to the file the attachment wraps, which is
	// what lets a client open it without a second call. It is absent when
	// the payload will not read, and it is an optional field of the wire
	// format rather than a core one, so a client reads it as optional and
	// shows an attachment carrying none as present and unopenable rather
	// than dropping the row.
	Path string `json:"path,omitempty"`
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
	// Ref is what a person types to reach the comment: the holder's own
	// reference, which is a card, a checklist item or a column, then comments
	// and the comment's one-based position among that holder's comments. It
	// is the spelling internal/bench/resolve.go resolves,
	// and it is a spelling to type now rather than a handle to keep, because a
	// position stops naming the same comment once an earlier one is deleted.
	// The identifier beside it is the handle, and the resolver accepts that in
	// the same slot.
	Ref string `json:"ref"`
	// Ordinal is the comment's one-based position among its holder's
	// comments, counted the way AttachmentView.Ordinal is counted.
	Ordinal int `json:"ordinal"`
	// TS is when it was written.
	TS string `json:"ts"`
	// Author is who wrote it.
	Author string `json:"author"`
	// Subject is the comment's first line carrying anything, with a leading
	// run of number signs and spaces removed so a comment opening with a
	// Markdown heading reads as that heading's words. It is capped at
	// subjectCap runes, and a capped value ends in a single ellipsis.
	Subject string `json:"subject"`
	// Size is the body's length in bytes, so a caller can price the read
	// that would fetch it before spending a round on it.
	Size int `json:"size"`
	// Body is the comment itself. It is the empty string on an entry a
	// card's answer serves as an index, and detailOf is what clears it, so
	// every other route to a comment view goes on carrying the whole of it.
	Body string `json:"body"`
	// Attachments are the comment's own attachments, on the terms a card's
	// are: the full list, each carrying its path. A comment is one of the
	// four kinds the containment grammar gives an attachments collection,
	// and a card's comments are bounded by that card, so the list costs
	// what the one card costs rather than what a listing costs.
	Attachments []AttachmentView `json:"attachments,omitempty"`
}

// ItemView is one checklist item as a read reports it.
type ItemView struct {
	// ID is the item's identifier.
	ID string `json:"id"`
	// Ordinal is the item's one-based position among the card's checklist
	// items in creation order, which is the meaning AttachmentView.Ordinal
	// already carries for attachments. It is not the item's position within
	// its own kind: that number is folded into Ref rather than carried a
	// second time, since two numbers on one row both answering which one is
	// this is a shape this board has already paid to remove once.
	Ordinal int `json:"ordinal"`
	// Ref is what a person types to reach this item, composed by itemRef.
	// An item of one of the three kinds the format declares is named by that
	// kind's word (questions, criteria, decisions) and its position among the
	// items of that kind; an item of any other kind is named by the checklist
	// collection and its position in it, which the resolver descends
	// unnarrowed. It is never empty.
	Ref string `json:"ref"`
	// Kind is one of acceptance_criterion, open_question and decision.
	Kind string `json:"kind"`
	// State is whatever the item's own file says, unvalidated, on the terms
	// CardView.Severity and CardView.Priority already report a level
	// verbatim rather than through a lookup.
	State string `json:"state"`
	// Column is the column this item names for gating, absent when the item
	// was filed without one.
	Column string `json:"column,omitempty"`
	// ColumnTitle is that column's title, resolved the way
	// CardView.ColumnTitle resolves the card's own, absent when Column names
	// no column this workbench still has.
	ColumnTitle string `json:"column_title,omitempty"`
	// Owner is who the item names as its answerer. bench.ItemOwnerOperator
	// is the one value enforced against the actor, by closeItem, which
	// refuses a terminal verb on such an item to anybody but the operator,
	// and by SetField, which refuses a rewrite of this key on one. Every
	// other value is enforced against nobody.
	Owner string `json:"owner,omitempty"`
	// Text is the item's own body: the judgement it records, unchanged from
	// when it was filed.
	Text string `json:"text"`
	// Note is the resolution note, absent while the item is pending and
	// absent whenever an item on disk carries none.
	Note string `json:"note,omitempty"`
	// CommentCount is how many comments the item carries. The count rather
	// than the comments themselves, because a card's checklist is read far
	// more often than any one item's argument is, and `dinah show <card>`
	// would otherwise carry every item's prose on every read. The threads
	// are reached through the item's own reference, which ItemDetail
	// answers.
	CommentCount int `json:"comment_count,omitempty"`
}

// ItemDetail is one checklist item as show answers for the item's own
// reference: the item's anchor exactly as a read of the file gives it, and
// the comments written on the item, in ordinal order.
type ItemDetail struct {
	// Ref is the item's own reference.
	Ref string `json:"ref"`
	// Text is the item's anchor, exactly as bench.ReadText returns it, which
	// is what show printed for an item reference before this change. An item
	// carrying no comments therefore prints byte for byte what it printed
	// before.
	Text string `json:"text"`
	// Comments are the comments written on the item, in ordinal order.
	Comments []CommentView `json:"comments,omitempty"`
}

// CommentListing is what list answers for a reference naming a comment
// collection: the reference the reader typed, the kind of thing the collection
// holds, and one index entry per comment in ordinal order.
type CommentListing struct {
	// Ref is the collection reference as the reader typed it.
	Ref string `json:"ref"`
	// Kind is what the collection holds, as the containment table spells it.
	Kind string `json:"kind"`
	// Members are the collection's comment index entries in ordinal order.
	Members []CommentIndexEntry `json:"members"`
	// Archived reports that the members listed are the archive mirror's own
	// rather than the live half's.
	Archived bool `json:"archived,omitempty"`
}

// CommentIndexEntry is one row of the comment listing index: the same fields
// the show index carries for a comment, with Body empty on entries at or
// before the --since ordinal and filled on entries after it.
type CommentIndexEntry struct {
	// ID is the comment's identifier.
	ID string `json:"id"`
	// Ordinal is the comment's one-based position among the card's comments
	// in creation order.
	Ordinal int `json:"ordinal"`
	// Ref is what a person types to reach this comment.
	Ref string `json:"ref"`
	// TS is the comment's timestamp, exactly as bench stores it.
	TS string `json:"ts"`
	// Author is the comment's author, exactly as bench stores it.
	Author string `json:"author"`
	// Subject is the first line of the body carrying anything, trimmed and
	// capped at subjectCap runes, computed by subjectOf.
	Subject string `json:"subject"`
	// Size is the byte length of the comment's body.
	Size int `json:"size"`
	// Body is the comment's full text on entries after the --since ordinal,
	// and the empty string on entries at or before it. An index served
	// without --since carries the empty string on every entry.
	Body string `json:"body"`
}

// ItemListing is what list answers for a reference naming a checklist
// collection: the reference the reader typed, the kind of thing the collection
// holds, and one index entry per item in creation order.
type ItemListing struct {
	// Ref is the collection reference as the reader typed it.
	Ref string `json:"ref"`
	// Kind is what the collection holds, as the containment table spells it.
	Kind string `json:"kind"`
	// Members are the collection's item index entries in creation order.
	Members []ItemIndexEntry `json:"members"`
	// Archived reports that the members listed are the archive mirror's own
	// rather than the live half's.
	Archived bool `json:"archived,omitempty"`
}

// ItemIndexEntry is one row of the checklist listing index: the same fields
// the show index carries for an item, with Text capped at subjectCap runes
// and Note removed, because the listing is the index and the recovery path
// is the item's own reference.
type ItemIndexEntry struct {
	// ID is the item's identifier.
	ID string `json:"id"`
	// Ordinal is the item's one-based position among the card's checklist
	// items in creation order.
	Ordinal int `json:"ordinal"`
	// Ref is what a person types to reach this item.
	Ref string `json:"ref"`
	// Kind is one of acceptance_criterion, open_question and decision.
	Kind string `json:"kind"`
	// State is whatever the item's own file says, unvalidated.
	State string `json:"state"`
	// Column is the column this item names for gating, absent when the item
	// was filed without one.
	Column string `json:"column,omitempty"`
	// ColumnTitle is that column's title, resolved the way
	// CardView.ColumnTitle resolves the card's own, absent when Column names
	// no column this workbench still has.
	ColumnTitle string `json:"column_title,omitempty"`
	// Owner is who the item names as its answerer.
	Owner string `json:"owner,omitempty"`
	// Text is the item's first line capped at subjectCap runes.
	Text string `json:"text"`
	// CommentCount is how many comments the item carries.
	CommentCount int `json:"comment_count,omitempty"`
}

// Record is what show prints for an entity whose answer is a set of fields
// rather than a body or a card detail. The workbench and the workstream are
// the two, and each carries the fields the references guide's field table
// gives its kind.
//
// It is a type of its own rather than a reuse of WorkstreamView, because that
// view is what `list workstreams` publishes and giving it a notes member would
// widen an envelope this change was not asked to touch.
type Record struct {
	// Kind is the entity kind, as the containment grammar spells it.
	Kind string `json:"kind"`
	// Ref is the reference a reader types to reach the entity.
	Ref string `json:"ref"`
	// Fields are the entity's own fields, in the order the kind declares
	// them, each carrying the name a reader types back and what is stored
	// under it. A field the entity carries nothing under is drawn with an
	// empty value rather than left out, because a record with a row missing
	// reads as a record of a different shape.
	Fields []RecordField `json:"fields"`
}

// RecordField is one row of a record: the field name, which is machine
// vocabulary and travels untranslated, and the value stored under it.
type RecordField struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// workbenchRecord is the workbench's own record, which carries the three
// fields the bare `dinah workbench` listing prints.
func (l *Library) workbenchRecord() *Record {
	record := &Record{Kind: bench.KindWorkbench, Ref: bench.WorkbenchRef}
	for _, name := range bench.WorkbenchListingFields {
		record.Fields = append(record.Fields, RecordField{Name: name, Value: l.Bench.WorkbenchField(name)})
	}
	return record
}

// workstreamRecord is one workstream's own record. The card count belongs to
// the set rather than to the record, so it is drawn by `list workstreams` and
// not here.
func (l *Library) workstreamRecord(workstream *bench.Workstream) *Record {
	record := &Record{Kind: bench.KindWorkstream, Ref: workstream.Ref()}
	for _, name := range bench.FieldsOf(bench.KindWorkstream) {
		record.Fields = append(record.Fields, RecordField{Name: name, Value: workstream.Field(name)})
	}
	return record
}

// Show reads a card, the record of the workbench or of a workstream, or the
// file any other reference names.
//
// It reads one entity and never a collection. A reference that stops at a
// collection is refused dinah.is-a-collection, whose next-step clause names
// the list invocation that answers, because listing a collection is what list
// is for and two commands answering one question is what this change removed.
//
// Every dinah.unknown-path this function raises carries req.Card, which is the
// reference exactly as the caller wrote it. Two of the three sites used to
// refuse on the head and one on the tail, so `dinah show workstream/addressing`
// reported a failure on `addressing`, a word the reader had not typed on its
// own. A refusal that rewrites what it was asked about tells the reader to go
// looking for the wrong thing.
//
// A card comes back as a Detail with an empty text. The workbench and a
// workstream come back as a Record. Every other reference comes back as a nil
// Detail beside the text of the file it named, since nothing but a card has a
// view to build. A caller reads whichever of the four is filled rather than
// assuming one.
func (l *Library) Show(req *Request) (*Detail, *Record, *ItemDetail, string, error) {
	// The field list is read before anything is resolved, so a call naming a
	// field this tool does not have performs no read and mutates nothing.
	chosen, err := parseDetailFields(req.Fields)
	if err != nil {
		return nil, nil, nil, "", err
	}
	// The two filters are read and checked in the same place and for the
	// same reason: a refused call reads nothing and mutates nothing.
	filters, err := parseDetailFilters(req)
	if err != nil {
		return nil, nil, nil, "", err
	}
	if err := checkDetailFilters(chosen, filters); err != nil {
		return nil, nil, nil, "", err
	}
	head, rest, _ := strings.Cut(req.Card, "/")
	// The workbench and the workstream are answered ahead of everything
	// else, because each has a record of its own and neither reaches a
	// branch below that could build one. The workstream is the reference the
	// card's framing records the operator typing, and answering it here is
	// the repair.
	if !req.Archived {
		if req.Card != "" && bench.IsWorkbenchRef(req.Card) {
			if chosen != nil {
				return nil, nil, nil, "", unknownDetailField(strings.TrimSpace(req.Fields), req.Card)
			}
			if filters.named() {
				return nil, nil, nil, "", refuseDetailFilter(filters.flagWord())
			}
			return nil, l.workbenchRecord(), nil, "", nil
		}
		if strings.HasPrefix(req.Card, bench.WorkstreamRefPrefix) {
			workstream, err := l.Bench.WorkstreamByRef(req.Card)
			if err != nil {
				return nil, nil, nil, "", err
			}
			if workstream == nil {
				return nil, nil, nil, "", contract.Refuse(contract.UnknownWorkstream, strings.TrimPrefix(req.Card, bench.WorkstreamRefPrefix))
			}
			if chosen != nil {
				return nil, nil, nil, "", unknownDetailField(strings.TrimSpace(req.Fields), req.Card)
			}
			if filters.named() {
				return nil, nil, nil, "", refuseDetailFilter(filters.flagWord())
			}
			return nil, l.workstreamRecord(workstream), nil, "", nil
		}
	}
	// A bare head under the flag gets a branch of its own, because neither of
	// the two branches below it reaches the resolver whose refusal the
	// archived half rests on: the column branch reads an anchor with no
	// resolver in the chain at all, and the branch at the bottom goes to
	// ResolveCard, which reads the live cards root. One resolution answers
	// both kinds here, and the workbench cannot come back from it, because
	// Bench.notArchivedFor refuses that reference ahead of the walk.
	//
	// This branch never calls lapseRead. An archived card is out of the flow
	// by construction, so expiring its claim would write an expired event
	// into an archived journal that nobody asked to change, and a read
	// command that writes to history is a surprise this contract does not
	// want.
	if rest == "" && req.Archived {
		entity, _, err := l.Bench.ResolveReferenceIn(bench.ArchivedHalf, req.Card)
		if err != nil {
			return nil, nil, nil, "", err
		}
		if entity.Kind != bench.KindCard {
			if chosen != nil {
				return nil, nil, nil, "", unknownDetailField(strings.TrimSpace(req.Fields), head)
			}
			if filters.named() {
				return nil, nil, nil, "", refuseDetailFilter(filters.flagWord())
			}
			text, err := bench.ReadText(filepath.Join(entity.Dir, bench.ColumnAnchor))
			if err != nil {
				return nil, nil, nil, "", contract.Refuse(contract.UnknownPath, req.Card)
			}
			return nil, nil, nil, text, nil
		}
		detail, text, err := l.detailOf(entity.Card, chosen, filters)
		return detail, nil, nil, text, err
	}
	// A column is an entity of the workbench, and the containment walk prints
	// a reference for one, so show reads it the way path and edit do rather
	// than refusing over a reference the tool told the reader to type.
	if rest == "" {
		if column := l.Bench.ColumnByRef(head); column != nil {
			if chosen != nil {
				return nil, nil, nil, "", unknownDetailField(strings.TrimSpace(req.Fields), head)
			}
			if filters.named() {
				return nil, nil, nil, "", refuseDetailFilter(filters.flagWord())
			}
			text, err := bench.ReadText(l.Bench.ColumnAnchorPath(column.ID))
			if err != nil {
				return nil, nil, nil, "", contract.Refuse(contract.UnknownPath, req.Card)
			}
			return nil, nil, nil, text, nil
		}
	}
	// A composed reference is whatever the resolver reaches, which is why the
	// resolution comes before the card is loaded: the head may name the
	// workbench or a column rather than a card, and every one of those forms is
	// a reference the containment walk prints.
	if rest != "" {
		// A composed reference never names a card, so it has no members to
		// select from and the refusal is raised ahead of the resolution.
		if chosen != nil {
			return nil, nil, nil, "", unknownDetailField(strings.TrimSpace(req.Fields), req.Card)
		}
		if filters.named() {
			return nil, nil, nil, "", refuseDetailFilter(filters.flagWord())
		}
		// The collection question is asked ahead of the resolution this
		// command already performs, and the resolver's error is ignored, so
		// every reference that refuses today goes on refusing with the
		// sentence it refuses with today. ResolvePath reaches an attachment's
		// payload, which this resolver refuses because a payload file carries
		// no anchor, and answering the collection first leaves that where it
		// is.
		if entity, collection, err := l.Bench.ResolveReferenceIn(halfFor(req), req.Card); err == nil {
			if collection != nil {
				return nil, nil, nil, "", collection.Refuse()
			}
			// An item reference answers a payload of its own, built beside
			// the card detail rather than as a bare text read, so `dinah
			// show <item>` prints the comments the checklist table only
			// counts.
			if entity != nil && entity.Kind == bench.KindItem {
				item, err := l.itemDetailOf(entity)
				if err != nil {
					return nil, nil, nil, "", err
				}
				return nil, nil, item, "", nil
			}
		}
		// The discarded error above stays discarded under the flag too, and
		// that is safe rather than lucky: ResolvePathIn on this line raises
		// the same refusal ResolveReferenceIn would have, which is why the
		// rule sits in both entry points rather than in one.
		path, err := l.Bench.ResolvePathIn(halfFor(req), req.Card)
		if err != nil {
			return nil, nil, nil, "", err
		}
		text, err := bench.ReadText(path)
		if err != nil {
			return nil, nil, nil, "", contract.Refuse(contract.UnknownPath, req.Card)
		}
		return nil, nil, nil, text, nil
	}
	found, err := l.Bench.ResolveCard(head)
	if err != nil {
		return nil, nil, nil, "", err
	}
	card := found.Card
	if err := l.lapseRead(card, req.Actor); err != nil {
		return nil, nil, nil, "", err
	}
	detail, text, err := l.detailOf(card, chosen, filters)
	return detail, nil, nil, text, err
}

// itemDetailOf builds the answer show gives for one checklist item's own
// reference: the item's anchor, unchanged from what show printed for it
// before this change, and its comments in ordinal order.
func (l *Library) itemDetailOf(entity *bench.EntityRef) (*ItemDetail, error) {
	anchor, ok := bench.AnchorPathOf(entity)
	if !ok {
		return nil, contract.Refuse(contract.UnknownPath, entity.Ref)
	}
	text, err := bench.ReadText(anchor)
	if err != nil {
		return nil, contract.Refuse(contract.UnknownPath, entity.Ref)
	}
	// The reference this prints is the kind-narrowed one the checklist table
	// already composes for the item, on the rule itemListing's own doc
	// comment states: one entity has one printed spelling, and
	// pb-1/checklist/1 is not it where pb-1/questions/1 is what filed it.
	// rootOf in tree.go composes the same form for a walk rooted at an item,
	// through this same method, so a third composition is not written here.
	ref, err := l.itemRefOf(entity)
	if err != nil {
		return nil, err
	}
	comments, err := l.commentViews(entity.Dir, ref)
	if err != nil {
		return nil, err
	}
	return &ItemDetail{Ref: ref, Text: text, Comments: comments}, nil
}

// subjectCap is how many runes of a first line an index entry carries. It is
// a rune count rather than a byte count, so a line of Thai or Chinese carries
// as many characters as a line of English.
const subjectCap = 120

// subjectEllipsis closes a value the cap cut short. One character rather than
// three full stops, so the cap costs a capped subject one rune.
const subjectEllipsis = "\u2026"

// subjectOf is what an index entry offers a reader in place of the comment
// itself: the first line of the body carrying anything, with a leading run of
// number signs and spaces dropped so a comment opening with a Markdown
// heading reads as that heading's words.
//
// firstLine in tree.go is what finds the line, and anchorTitle beside it is
// the prior art for treating a comment's first line as its title. The trim
// and the cap are what this adds, because a title drawn in a tree is not a
// value a caller prices a second read against.
func subjectOf(body string) string {
	line := strings.TrimSpace(firstLine(body))
	line = strings.TrimSpace(strings.TrimLeft(line, "#"))
	return capRunes(line, subjectCap)
}

// capRunes cuts a value to a rune count and closes a cut value with one
// ellipsis. The cut falls on a rune boundary, so a multi-byte rune straddling
// the limit is dropped whole rather than halved into bytes no reader can
// render.
func capRunes(value string, limit int) string {
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit]) + subjectEllipsis
}

// commentViews reads the comments written directly below one entity, in
// ordinal order, each carrying a reference composed against the holder's own
// reference. A card, a checklist item and a column are the three kinds a
// comment hangs from directly.
//
// Body is filled on every view, whatever the caller asked for. A card's own
// answer clears it on each entry it serves as an index, in detailOf, so that
// `dinah show <card>/comments/<n>` and an item's own comments go on serving
// what they serve today: both reach this function by another path.
func (l *Library) commentViews(dir, holderRef string) ([]CommentView, error) {
	stored, err := bench.Comments(dir)
	if err != nil {
		return nil, err
	}
	var comments []CommentView
	for _, comment := range stored {
		position, err := memberPosition(comment.Dir, bench.CommentAnchor)
		if err != nil {
			return nil, err
		}
		ref := commentRef(holderRef, position)
		view := CommentView{
			ID:      comment.ID,
			Ref:     ref,
			Ordinal: position,
			TS:      comment.TS,
			Author:  comment.Author,
			Subject: subjectOf(comment.Body),
			Size:    len(comment.Body),
			Body:    comment.Body,
		}
		// A comment's attachments compose their references against the
		// comment's own address rather than the holder's, so a reference the
		// view prints reaches the attachment the view describes.
		below, err := attachmentViews(comment.Dir, ref)
		if err != nil {
			return nil, err
		}
		view.Attachments = below
		comments = append(comments, view)
	}
	return comments, nil
}

// detailOf builds the answer show gives for one card: every member the card
// holds, then the selection applied over them, then the announcement of what
// was withheld. Show reaches it from two branches, the live bare-card one and
// the archived-half one, and neither carries a copy of the build.
//
// It is the whole of what show does once it has a card, so nothing about
// which half the card came from reaches inside it.
func (l *Library) detailOf(card *bench.Card, chosen detailSelection, filters detailFilters) (*Detail, string, error) {
	cardRef := card.Ref(l.Bench.Slug)
	// Every member is built before the selection is applied, because withheld
	// reports what the card holds rather than what the caller left out, and
	// only a built member answers that. The cost is a read of the card's own
	// directory, which show performs whatever the caller asked for.
	detail := &Detail{Path: card.AnchorPath(), selected: chosen}
	if chosen.carries("card") {
		view, err := l.view(card)
		if err != nil {
			return nil, "", err
		}
		detail.Card = *view
	}
	if chosen.carries("body") {
		detail.Body = card.Body
	}
	var links []LinkView
	for _, link := range card.Links {
		links = append(links, LinkView{Kind: link.Kind, To: link.To, Ref: l.linkRef(link.To)})
	}
	views, err := attachmentViews(card.Dir, cardRef)
	if err != nil {
		return nil, "", err
	}
	comments, err := l.commentViews(card.Dir, cardRef)
	if err != nil {
		return nil, "", err
	}
	items, err := bench.Items(card.Dir)
	if err != nil {
		return nil, "", err
	}
	var checklist []ItemView
	// The unresolved filter is applied over the items the card holds rather
	// than over the views this answer carries, so the announcement can say
	// that the card holds an item this answer dropped. It reads
	// bench.ItemLiftsColumnHold, which is the predicate a column hold reads
	// through GatingItems, so a station asking what still holds the card and
	// a station running dinah move get one answer about the same card.
	var carriedChecklist []ItemView
	// The position a reference carries is counted within the item's own kind,
	// which is what descend narrows a checklist segment by, so the two are
	// counted the same way over the same order rather than composed from the
	// overall ordinal and hoped to agree.
	//
	// The collection position is taken through memberPosition rather than from
	// this loop's index, because bench.Items skips an item whose anchor will
	// not open and the resolver counts the collection unfiltered. Counting
	// here would number every item after a damaged one one place low, so show
	// would print an address reaching a different item, which is worse than
	// the blank cell this card replaced.
	kindPosition := map[string]int{}
	for _, item := range items {
		kindPosition[item.Kind]++
		position, err := memberPosition(item.Dir, bench.ItemAnchor)
		if err != nil {
			return nil, "", err
		}
		view := ItemView{
			ID:      item.ID,
			Ordinal: position,
			Kind:    item.Kind,
			State:   item.State,
			Column:  item.Column,
			Owner:   item.Owner,
			Text:    item.Text,
			Note:    item.Note,
		}
		view.Ref = itemRef(cardRef, item.Kind, kindPosition[item.Kind], position)
		count, err := bench.CountComments(item.Dir)
		if err != nil {
			return nil, "", err
		}
		view.CommentCount = count
		if item.Column != "" {
			if column := l.Bench.Column(item.Column); column != nil {
				view.ColumnTitle = column.Title
			}
		}
		checklist = append(checklist, view)
		if !filters.unresolved || !bench.ItemLiftsColumnHold(item) {
			carriedChecklist = append(carriedChecklist, view)
		}
	}
	// Each collection is reduced to its index unless the caller asked for
	// that member in full, and the count of entries reduced is what the
	// announcement below reads: a member carried with every body filled
	// announces no modifier, and one carried with a single body missing
	// announces it.
	indexed := map[string]int{}
	if chosen.carries("comments") && !chosen.full("comments") {
		for i := range comments {
			if filters.sinceSet && comments[i].Ordinal > filters.since {
				continue
			}
			comments[i].Body = ""
			indexed["comments"]++
		}
	}
	if chosen.carries("checklist") && !chosen.full("checklist") {
		for i := range carriedChecklist {
			carriedChecklist[i].Text = capRunes(firstLine(carriedChecklist[i].Text), subjectCap)
			carriedChecklist[i].Note = ""
			indexed["checklist"]++
		}
	}
	if chosen.carries("links") {
		detail.Links = links
	}
	if chosen.carries("attachments") {
		detail.Attachments = views
	}
	if chosen.carries("comments") {
		detail.Comments = comments
	}
	if chosen.carries("checklist") {
		detail.Checklist = carriedChecklist
	}
	if !chosen.carries("path") {
		detail.Path = ""
	}
	// The announcement says what the card holds and this answer did not
	// carry, in the order DetailSelectors declares, so two runs against one
	// card compose one string. A member is named where the card holds an
	// entry the answer left out, whether the field list dropped the member
	// or a filter dropped an entry of it, and a modifier is named where the
	// answer carried an entry of its member without that entry's body.
	//
	// Both halves are read off what the answer carried rather than off the
	// selection, which is what lets the rule cover the unshaped read: a nil
	// selection carries every member, and a loop asking the selection what
	// it left out would answer that a nil map left out all nine.
	held := map[string]bool{
		"card":        true,
		"body":        card.Body != "",
		"links":       len(links) > 0,
		"attachments": len(views) > 0,
		"comments":    len(comments) > 0,
		"checklist":   len(checklist) > 0,
		"path":        card.AnchorPath() != "",
	}
	// A filter that dropped nothing narrowed nothing, so the recovery the
	// terminal offers names a flag only where dropping it would change the
	// answer.
	narrowedChecklist := len(carriedChecklist) < len(checklist)
	if narrowedChecklist {
		detail.narrowedBy = append(detail.narrowedBy, flagUnresolved)
	}
	for _, name := range DetailSelectors {
		if base := baseOfModifier(name); base != name {
			if indexed[base] > 0 {
				detail.Withheld = append(detail.Withheld, name)
			}
			continue
		}
		if !held[name] {
			continue
		}
		if !chosen.carries(name) {
			detail.Withheld = append(detail.Withheld, name)
			continue
		}
		if name == "checklist" && narrowedChecklist {
			detail.Withheld = append(detail.Withheld, name)
		}
	}
	if len(detail.Withheld) > 0 {
		detail.Reread = cardRef
	}
	return detail, "", nil
}

// commentListing builds a CommentListing from the collection's comments,
// applying the --since filter: entries at or before the since ordinal carry an
// empty Body, entries after it carry the full text. Without --since, every
// entry carries an empty Body (the index is the index, not the payload).
func (l *Library) commentListing(collection *bench.CollectionRef, sinceOrdinal int, sinceSet bool) (*CommentListing, error) {
	views, err := l.commentViews(collection.Holder.Dir, collection.Holder.Ref)
	if err != nil {
		return nil, err
	}
	members := make([]CommentIndexEntry, 0, len(views))
	for _, v := range views {
		body := ""
		if sinceSet && v.Ordinal > sinceOrdinal {
			body = v.Body
		}
		members = append(members, CommentIndexEntry{
			ID:      v.ID,
			Ordinal: v.Ordinal,
			Ref:     v.Ref,
			TS:      v.TS,
			Author:  v.Author,
			Subject: v.Subject,
			Size:    v.Size,
			Body:    body,
		})
	}
	return &CommentListing{Ref: collection.Ref, Kind: collection.Mount.Kind, Members: members, Archived: collection.Archived}, nil
}

// itemListing builds an ItemListing from the collection's checklist items,
// applying the --unresolved filter: when set, only items whose state does not
// lift a column hold are included.
func (l *Library) itemListing(collection *bench.CollectionRef, unresolvedOnly bool) (*ItemListing, error) {
	items, err := bench.Items(collection.Holder.Dir)
	if err != nil {
		return nil, err
	}
	cardRef := collection.Holder.Ref
	kindPosition := map[string]int{}
	positionByID := map[string]int{}
	itemByID := map[string]*bench.Item{}
	// Canonical kind positions come from the complete checklist. The
	// collection and unresolved filters run only after every readable item has
	// its address, so neither filter can compress an item's reference.
	for _, item := range items {
		kindPosition[item.Kind]++
		positionByID[item.ID] = kindPosition[item.Kind]
		itemByID[item.ID] = item
	}
	// The resolver has already narrowed Members for questions, criteria or
	// decisions. Reading that list also preserves the resolved collection's
	// order instead of rebuilding a wider collection from the holder.
	members := make([]ItemIndexEntry, 0, len(collection.Members))
	for _, id := range collection.Members {
		item := itemByID[id]
		if item == nil {
			continue
		}
		if unresolvedOnly && bench.ItemLiftsColumnHold(item) {
			continue
		}
		position, err := memberPosition(item.Dir, bench.ItemAnchor)
		if err != nil {
			return nil, err
		}
		text := capRunes(firstLine(item.Text), subjectCap)
		var col, colTitle string
		if item.Column != "" {
			col = item.Column
			if column := l.Bench.Column(item.Column); column != nil {
				colTitle = column.Title
			}
		}
		count, err := bench.CountComments(item.Dir)
		if err != nil {
			return nil, err
		}
		members = append(members, ItemIndexEntry{
			ID:           item.ID,
			Ordinal:      position,
			Ref:          itemRef(cardRef, item.Kind, positionByID[item.ID], position),
			Kind:         item.Kind,
			State:        item.State,
			Column:       col,
			ColumnTitle:  colTitle,
			Owner:        item.Owner,
			Text:         text,
			CommentCount: count,
		})
	}
	return &ItemListing{Ref: collection.Ref, Kind: collection.Mount.Kind, Members: members, Archived: collection.Archived}, nil
}

// AttachmentListing is one entity's attachments: a workbench's, a column's, a
// card's or a comment's, which are the four kinds the containment grammar
// gives an attachments collection.
type AttachmentListing struct {
	// Kind is the entity's kind, as the containment grammar spells it, and
	// verb.KindCollection where the reference named a whole collection that
	// hangs no attachments of its own.
	Kind string `json:"kind"`
	// Ref is what a person types to reach the entity the attachments hang
	// from. The workbench is written `workbench`, which is the spelling the
	// containment tree's own root row already prints for it, everything else
	// carries the reference the resolver composed, and a collection reference
	// that hangs nothing carries itself as the reader typed it.
	Ref string `json:"ref"`
	// Attachments are the entity's own attachments in creation order, never
	// those of anything it contains. An entity carrying none reports an
	// empty list rather than nothing at all.
	Attachments []AttachmentView `json:"attachments"`
}

// Attachments reports one entity's own attachments, named by any reference the
// entity resolver reaches.
//
// An entity of a kind the grammar gives no attachments collection, which is a
// checklist item, an attachment itself or a workstream, is not refused. It
// reports an empty list, which is the answer an entity of a mounted kind gives
// when it happens to carry nothing, and a caller walking a tree therefore asks
// the same question everywhere instead of deciding first whether the question
// is legal.
func (l *Library) Attachments(req *Request) (*AttachmentListing, error) {
	entity, collection, err := l.Bench.ResolveReference(req.Ref)
	if err != nil {
		return nil, err
	}
	if collection != nil {
		// An attachments collection is the holder's own attachments named
		// the long way, so it is answered from the holder and prints what
		// the holder prints. Every other collection hangs no attachments and
		// gets the empty listing an entity of an unmounted kind already gets,
		// for the reason this function's own doc comment gives.
		if collection.Mount.Kind != bench.KindAttachment {
			return &AttachmentListing{Kind: KindCollection, Ref: collection.Ref, Attachments: []AttachmentView{}}, nil
		}
		entity = collection.Holder
	}
	// EntityRef leaves the workbench's own reference empty, calling its
	// spelling a question the resolver does not settle, so composing an
	// attachment's reference from it would print /attachments/1, which
	// nothing accepts. rootOf already answered that question for the
	// containment tree, and this mirrors its answer rather than minting a
	// second one.
	ref := entity.Ref
	if entity.Kind == bench.KindWorkbench {
		ref = bench.WorkbenchRef
	}
	views, err := attachmentViews(entity.Dir, ref)
	if err != nil {
		return nil, err
	}
	if views == nil {
		views = []AttachmentView{}
	}
	return &AttachmentListing{Kind: entity.Kind, Ref: ref, Attachments: views}, nil
}

// attachmentViews reads an entity's attachments collection and renders each
// member as a read reports it, composing every reference against the entity's
// own address. Every read that publishes attachments goes through here, so a
// field one read carries cannot go missing from another.
func attachmentViews(dir, ref string) ([]AttachmentView, error) {
	attachments, err := bench.Attachments(dir)
	if err != nil {
		return nil, err
	}
	var views []AttachmentView
	for _, attachment := range attachments {
		ordinal, err := displayOrdinal(attachment)
		if err != nil {
			return nil, err
		}
		views = append(views, AttachmentView{
			ID:          attachment.ID,
			Ordinal:     ordinal,
			Ref:         attachmentRef(ref, ordinal),
			Filename:    attachment.Filename,
			Description: attachment.Description,
			Provenance:  attachment.Provenance,
			Path:        attachment.Path,
		})
	}
	return views, nil
}

// commentRef composes the reference a person types to reach one comment from
// its card: the card's own reference, then comments and the comment's ordinal.
func commentRef(cardRef string, ordinal int) string {
	return cardRef + "/" + bench.CommentsDir + "/" + strconv.Itoa(ordinal)
}

// itemRef is what a person types to reach one checklist item. An item of
// one of the three kinds the format declares is named by that kind's word
// and its position among the items of that kind, which is the spelling
// dinah show already prints and the one walkBelowCard narrows by. An item
// of any other kind is named by the collection and its position in it,
// which descend resolves without narrowing, so a damaged item or an
// extension kind still shows a reader something they can type.
func itemRef(cardRef, kind string, kindPosition, position int) string {
	if word, ok := bench.WordForItemKind(kind); ok {
		return cardRef + "/" + word + "/" + strconv.Itoa(kindPosition)
	}
	return cardRef + "/" + bench.ChecklistDir + "/" + strconv.Itoa(position)
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
func displayOrdinal(attachment *bench.Attachment) (int, error) {
	return memberPosition(attachment.Dir, bench.AttachmentAnchor)
}

// memberPosition is the one-based place a member holds in the collection its
// directory sits in, counted the way the reference resolver counts it, and
// zero when the directory is not a member of that collection at all.
//
// The resolver answers a positional segment by indexing the collection sorted
// through SortByOrdinal, so the position is an index into that sequence rather
// than the stored ordinal, and the two stop coinciding after one delete. The
// count is taken here so that every read composing a reference and the
// resolver reading one back agree by construction.
func memberPosition(dir, anchor string) (int, error) {
	collection := filepath.Dir(dir)
	id := filepath.Base(dir)
	ids, err := bench.ListIDs(collection)
	if err != nil {
		return 0, err
	}
	for n, member := range bench.SortByOrdinal(collection, anchor, ids) {
		if member == id {
			return n + 1, nil
		}
	}
	return 0, nil
}

// attachmentRef composes the reference a person types to reach one attachment
// from the entity it hangs from: that entity's own reference, then attachments
// and the position the caller resolved through displayOrdinal.
func attachmentRef(ownerRef string, ordinal int) string {
	return ownerRef + "/" + bench.AttachmentsDir + "/" + strconv.Itoa(ordinal)
}

// linkRef resolves a link's stored card identifier to what a person types to
// reach it. A link records only the identifier (card.go's Link comment: "a
// declaration rather than an entity"), so the alias is resolved fresh on
// every read rather than carried by the link itself, the same way a card's
// own Ref is computed at view time rather than stored. A card the link names
// that is no longer findable, archived or otherwise, still shows something
// typeable: the bare identifier the link already carried.
func (l *Library) linkRef(id string) string {
	if card, err := l.Bench.LoadCardIn(l.Bench.CardsRoot(), id); err == nil {
		return card.Ref(l.Bench.Slug)
	}
	if card, err := l.Bench.LoadCardIn(l.Bench.ArchivedCardsRoot(), id); err == nil {
		return card.Ref(l.Bench.Slug)
	}
	return id
}

// History reports a card's recorded acts in the order they were recorded. An
// identifier carried in an act is never resolved against the bench as it now
// stands, so a column renamed after a move still reads under its old title.
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
	// Loop is the card's standing against its column's declared loop_limit,
	// absent where the column declares none and absent whenever the chain was
	// served for a column rather than for a card, since a count belongs to a
	// card and a column named on its own carries no card to count for.
	Loop *Loop `json:"loop,omitempty"`
	// Column is the column the instructions were served for.
	Column string `json:"column"`
	// ChainServed carries the instruction-chain keys this answer served in
	// full, off the wire for the reason Response.ChainServed is.
	ChainServed []string `json:"-"`
}

// instructionColumn answers the column a request names directly, and nil when
// the reference names a card standing in one instead. It is the one place the
// two branches of an instruction request are told apart: the chain is served
// from it and the affordances are chosen from it, so the chain and the list
// can never disagree about which of the two the caller asked for.
func (l *Library) instructionColumn(req *Request) *bench.Column {
	return l.Bench.ColumnByRef(req.Card)
}

// Instructions serves the chain at a position named by a card or by a column.
//
// The two branches part company on withholding, which is CORE-INSTR-10's
// recovery route. A card-shaped request asks where a card stands and what
// applies there, so its answer is subject to withholding exactly as a claim's
// or a move's is. A column-shaped request names the text itself, so it serves
// every layer in full whatever the connection already holds, and an agent that
// has lost the chain gets it back in one call.
func (l *Library) Instructions(req *Request) (*Served, error) {
	if column := l.instructionColumn(req); column != nil {
		chain, keys := l.composeChain(req, column, false)
		served := &Served{
			Column:       column.ID,
			Instructions: *chain,
			ChainServed:  keys,
		}
		return served, nil
	}
	// The collection question comes ahead of the card resolution, and the
	// resolver's error is ignored, so a reference naming nothing goes on
	// refusing dinah.unknown-path with the whole reference below rather than
	// picking up a refusal about cards that nobody asked for.
	if _, collection, err := l.Bench.ResolveReference(req.Card); err == nil && collection != nil {
		return nil, collection.Refuse()
	}
	found, err := l.Bench.ResolveCard(req.Card)
	if err != nil {
		return nil, contract.Refuse(contract.UnknownPath, req.Card)
	}
	loop, err := l.cardLoop(found.Card)
	if err != nil {
		return nil, err
	}
	chain, keys := l.serve(req, found.Card)
	served := &Served{
		Column:       found.Card.Column,
		Instructions: *chain,
		LegalMoves:   l.legalMoves(found.Card),
		Loop:         loop,
		ChainServed:  keys,
	}
	return served, nil
}

// Identity is who the actor is, whether that owner is the operator, what the
// caller declared about what is performing the act, and the rung the
// workbench's table resolves that declaration to.
type Identity struct {
	// Actor is the owner the ladder produced.
	Actor string `json:"actor"`
	// IsOperator answers CORE-OWNER-2 for this bench.
	IsOperator bool `json:"is_operator"`
	// Operator is the owner reserved acts belong to.
	Operator string `json:"operator,omitempty"`
	// Harness, Provider, Model and Server are what the caller declared, each
	// absent where it declared nothing.
	Harness  string `json:"harness,omitempty"`
	Provider string `json:"provider,omitempty"`
	Model    string `json:"model,omitempty"`
	Server   string `json:"server,omitempty"`
	// MalformedHarness says the declared harness name is outside the grammar
	// one segment of a declared field key matches. The value is still reported
	// under Harness, because a person repairing a mistyped variable needs to
	// see what it says, and this is what tells them it is not being read as a
	// harness. A read is never refused over it, so whoami is where a person
	// meets it.
	MalformedHarness bool `json:"malformed_harness,omitempty"`
	// Tier is the rung the workbench's table resolved for this provider and
	// model. It is absent where the workbench declares no table, where it
	// declares no tier axis, and where the table lists neither this model nor
	// this model at this server.
	Tier string `json:"tier,omitempty"`
}

// Whoami reports the resolved actor, whether it is the operator, what the
// caller declared about what is performing the act, and the tier that
// declaration resolves to.
func (l *Library) Whoami(req *Request) (*Identity, error) {
	if req.Actor == "" {
		return nil, contract.Refuse(contract.NoOwner, "")
	}
	tier, _ := l.Bench.TierOf(req.Provider, req.Model, req.Server)
	identity := &Identity{
		Actor:            req.Actor,
		IsOperator:       req.Actor == l.Bench.Operator,
		Operator:         l.Bench.Operator,
		Harness:          req.Harness,
		Provider:         req.Provider,
		Model:            req.Model,
		Server:           req.Server,
		MalformedHarness: req.Harness != "" && !bench.HarnessName(req.Harness),
		Tier:             tier,
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
	// Outcome is contract.ReadFindings when Findings carries anything and
	// contract.ReadOK when it does not. It carries no omitempty and is
	// always present, so a client answers the coarse question with one
	// string comparison rather than by testing whether findings came back
	// empty, null or absent (dinah-346).
	Outcome string `json:"outcome"`
	// Findings are the defects the checker names, together with whatever a
	// repair in the same request could not do.
	Findings []bench.Finding `json:"findings"`
	// StampedOrdinals counts the creation ordinals the migration wrote, and
	// is absent from a request that did not ask for the migration.
	StampedOrdinals *int `json:"stamped_ordinals,omitempty"`
	// AssignedSlugs are the columns the slug migration repaired with the slug
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
	// RemovedStrandedColumns are the identifiers the columns migration removed
	// from the workbench's own columns list. It is absent from a request that
	// asked for no migration and from a request that asked and found nothing
	// to repair, which MigratedColumns below is what separates.
	RemovedStrandedColumns []string `json:"removed_stranded_columns,omitempty"`
	// MigratedColumns says the stranded-column migration ran, so a caller can
	// tell an empty list of removals from a migration nobody asked for.
	MigratedColumns bool `json:"migrated_columns,omitempty"`
	// WitnessedCards are the identifiers of the cards the witness repair
	// appended a manual_correction event to. It is absent from a request that
	// asked for no witnessing and from a request that asked and found nothing
	// to witness, which MigratedWitness below is what separates.
	WitnessedCards []string `json:"witnessed_cards,omitempty"`
	// MigratedWitness says the witness repair ran, so a caller can tell an
	// empty list of witnesses from a repair nobody asked for.
	MigratedWitness bool `json:"migrated_witness,omitempty"`
	// AssignedWorkstreamSlugs are the workstreams the slug migration
	// repaired with the slug each one was given, on the terms AssignedSlugs
	// carries the columns.
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
	// RegistryLines counts the lines the number migration wrote to the
	// registry, and is absent from a request that did not ask for the
	// migration. A run over a workbench whose registry already covers every
	// card answers zero, which is the migration's own word that it found
	// nothing to build, so the field is set whenever the migration ran
	// rather than only when it wrote.
	RegistryLines *int `json:"registry_lines,omitempty"`
	// MigratedNumbers says the number migration ran, so a caller can tell a
	// zero line count from a migration nobody asked for.
	MigratedNumbers bool `json:"migrated_numbers,omitempty"`
	// RenumberedCards are the identifiers of the cards that left a repair
	// holding a number they did not arrive holding, whether the number
	// migration renumbered a loser of a collision or the renumber repair
	// moved a later claimant of a number two lines both claimed. It is
	// absent from a request that asked for neither repair and from a request
	// that asked and found nothing to move, which the MigratedNumbers and
	// RenumberedNumbers flags are what separate.
	RenumberedCards []string `json:"renumbered_cards,omitempty"`
	// RenumberedNumbers says the renumber repair ran, so a caller can tell
	// an empty list of renumbered cards from a repair nobody asked for.
	RenumberedNumbers bool `json:"renumbered_numbers,omitempty"`
	// MigratedBranches is the branch migration's own account of the run, and
	// is absent from a request that did not ask for the migration. It carries
	// its own conflicts rather than raising them as findings, because a
	// caller that discarded the report on an error path would show an
	// operator one word and none of the cards to repair.
	MigratedBranches *bench.BranchMigration `json:"migrated_branches,omitempty"`
	// MigratedNewlines is the newline migration's own account of the run, on
	// the terms MigratedBranches is one: absent where the flag was not asked
	// for, and present with its own preview marker where it was.
	MigratedNewlines *bench.NewlineMigration `json:"migrated_newlines,omitempty"`
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
// migrate-slugs marker does the same for the columns of a workbench that
// predate the slug field, names the slug it gave each one, and derives the
// workbench's own slug when the workbench itself predates that field. A
// request carrying the migrate-columns marker removes every stranded
// identifier from the workbench's own columns list. A request carrying the
// witness marker records a manual-correction event on every live card whose
// anchor and journal disagree about where it stands. A request carrying the
// migrate-numbers marker builds the card-number registry from the numbers the
// cards still carry in their anchors, strips the number key from every anchor,
// and stamps the workbench with the format that declares the registry. A
// request carrying the renumber marker repairs duplicated registry numbers,
// leaving the number with the line that claimed it first and moving every
// later claimant above the highest number anything holds. Both refuse without
// the confirm flag, because each changes what a card is called and a reference
// somebody wrote down stops resolving.
//
// A non-nil error return still carries a non-nil report when the migration
// ran: the report is what the run had already stamped and already guessed
// before whatever ended it, and a caller that discards it on the error path
// loses that account the same way the run it is reporting on must not.
func (l *Library) Check(req *Request) (*CheckReport, error) {
	report := &CheckReport{}
	// A bare check reads and repairs nothing, and a read is never refused over
	// a malformed harness name. A check carrying a repair marker writes, and
	// several of the repairs write journal lines, so the refusal reaches those
	// runs on the same rule every other writing act is held to.
	if req != nil && req.Repairs() && req.Harness != "" && !bench.HarnessName(req.Harness) {
		return report, contract.Refuse(contract.MalformedHarness, req.Harness)
	}
	if req != nil && req.MigrateSlugs {
		assigned, reported := l.Bench.BackfillColumnSlugs()
		report.MigratedSlugs = true
		report.AssignedSlugs = assigned
		report.Findings = append(report.Findings, reported...)
		streamAssigned, streamReported, streamErr := l.Bench.BackfillWorkstreamSlugs()
		report.AssignedWorkstreamSlugs = streamAssigned
		report.Findings = append(report.Findings, streamReported...)
		if streamErr != nil {
			return report, streamErr
		}
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
	if req != nil && req.MigrateColumns {
		removed, err := l.Bench.RemoveStrandedColumns()
		report.MigratedColumns = true
		report.RemovedStrandedColumns = removed
		if err != nil {
			return report, err
		}
	}
	if req != nil && req.MigrateWitness {
		witnessed, reported, err := l.Bench.WriteWitnesses(req.Actor, bench.Stamp(l.Now()))
		report.MigratedWitness = true
		report.WitnessedCards = witnessed
		report.Findings = append(report.Findings, reported...)
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
	// The two number repairs refuse before they run when the request carries
	// no confirm, and the report travels beside the refusal rather than being
	// discarded for it, because a request may name an earlier repair whose
	// account has already been gathered here and the refusal does not undo
	// that work.
	if req != nil && req.MigrateNumbers {
		if !req.Confirm {
			return report, contract.Refuse(contract.Unconfirmed, "--migrate-numbers")
		}
		written, renumbered, reported, err := l.Bench.MigrateNumbers(req.Actor, bench.Stamp(l.Now()))
		report.MigratedNumbers = true
		report.RegistryLines = &written
		report.RenumberedCards = append(report.RenumberedCards, renumbered...)
		report.Findings = append(report.Findings, reported...)
		if err != nil {
			return report, err
		}
	}
	// The branch migration runs between the two number repairs, which is the
	// order the parameter table declares the three flags in and the order
	// checkStarvedMarkers prints them in. It reports rather than refusing when
	// it carries no confirmation, which is what makes its preview readable,
	// and that is where it parts company with the two either side of it.
	if req != nil && req.MigrateBranches {
		migrated, err := l.Bench.MigrateBranches(req.Actor, bench.Stamp(l.Now()), req.Confirm)
		report.MigratedBranches = migrated
		if err != nil {
			return report, err
		}
	}
	// The newline repair runs beside the branch migration and on the same
	// two-phase shape: it reports rather than refusing when it carries no
	// confirmation, which is what makes its preview readable.
	if req != nil && req.MigrateNewlines {
		migrated, err := l.Bench.MigrateNewlines(req.Actor, bench.Stamp(l.Now()), req.Confirm)
		report.MigratedNewlines = migrated
		if err != nil {
			return report, err
		}
	}
	if req != nil && req.Renumber {
		if !req.Confirm {
			return report, contract.Refuse(contract.Unconfirmed, "--renumber")
		}
		renumbered, reported, err := l.Bench.RenumberCards(req.Actor, bench.Stamp(l.Now()))
		report.RenumberedNumbers = true
		report.RenumberedCards = append(report.RenumberedCards, renumbered...)
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
		report.stampOutcome()
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
	report.stampOutcome()
	return report, nil
}

// stampOutcome records the report's own outcome from the findings it has
// gathered. Call it at each point the report leaves Check with no error, and
// call it after every branch that can still append to Findings has run, since
// a value computed before a migration branch appends would be stale by the
// time the caller reads it.
func (r *CheckReport) stampOutcome() {
	r.Outcome = contract.ReadOK
	if len(r.Findings) > 0 || !r.migrationsClean() {
		r.Outcome = contract.ReadFindings
	}
}

// migrationsClean reports whether the repairs that report rather than refusing,
// where one ran, met a conflict. A conflict is work a person has to do that no
// finding names, so the outcome carries it outward as the command's exit code
// rather than letting a run that migrated nothing exit zero.
//
// It asks about conflicts and not about repairs, and the difference is the
// whole of what this function is for. A repair that happened is not work left
// over, and a repair still pending is already named by the finding that reports
// the same file, so counting either would make a confirmed run that succeeded
// completely print that there are no defects and then exit non-zero.
func (r *CheckReport) migrationsClean() bool {
	if r.MigratedBranches != nil && !r.MigratedBranches.Clean() {
		return false
	}
	return r.MigratedNewlines == nil || r.MigratedNewlines.Clean()
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
		ev := bench.Event{TS: now, Event: contract.EventCreated, Actor: req.Acting()}
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
	// Commands is the live command roster used to classify stale aliases
	// that now collide with a declared command.
	Commands map[string]bool
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
	for _, alias := range cfg.Aliases() {
		source := bench.SourceConfig
		if alias.Defect != "" {
			source = bench.SourceInvalid
		} else if ctx.Commands[alias.Name] {
			source = bench.SourceShadowed
		}
		views = append(views, SettingView{Key: alias.Key, Value: alias.Template, Source: source})
	}
	for _, key := range cfg.Keys() {
		if bench.KnownConfigKey(key) || strings.HasPrefix(key, bench.AliasPrefix) {
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
	// Executable is where this binary is, which a client that must hand a
	// third party a runnable command needs and cannot work out for itself.
	// Absent when the operating system would not say.
	Executable string `json:"executable,omitempty"`
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

// ExecutablePath is os.Executable behind a seam, so the failure arm is
// reachable from a test. bench.statPath is the same shape. It is exported
// because the test that drives the failure arm runs the whole head in
// cmd/dinah rather than this package, and an unexported seam would put that
// arm out of its reach.
var ExecutablePath = os.Executable

// Version reports what this binary is and what it conforms to, optionally
// with the coverage of every shipped catalog.
func Version(withCatalogs bool) *VersionReport {
	release := &VersionReport{
		Tool:    ToolRelease,
		Profile: bench.ProfileVersion,
		Format:  bench.StorageFormat,
	}
	// A binary that cannot say where it is reports nothing rather than a
	// guess, and omitempty turns the empty string into an absent key.
	if path, err := ExecutablePath(); err == nil {
		release.Executable = path
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
