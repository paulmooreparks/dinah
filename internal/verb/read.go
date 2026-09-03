package verb

import (
	"path/filepath"
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
		AttachmentCount: bench.CountAttachments(l.Bench.Root),
	}
	counts := map[string]int{}
	for _, card := range cards {
		if err := l.lapseRead(card, req.Actor); err != nil {
			return nil, err
		}
		counts[card.Column]++
		if card.Holder != "" && card.Holder == req.Actor {
			status.Holding = append(status.Holding, *l.view(card))
		}
		if card.State == contract.StateBlocked {
			status.Blocked = append(status.Blocked, *l.view(card))
		}
	}
	status.Columns = l.columnViews(counts)
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
	return l.columnViews(counts), nil
}

// columnViews renders the flow with each station's occupancy.
func (l *Library) columnViews(counts map[string]int) []ColumnView {
	views := make([]ColumnView, 0, len(l.Bench.Columns))
	for _, column := range l.Bench.Columns {
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
			AttachmentCount: bench.CountAttachments(l.Bench.ColumnDir(column.ID)),
		}
		if destination := carriesInto(column, l.Bench.Columns); destination != nil {
			view.PullDestination = columnRef(destination)
		}
		views = append(views, view)
	}
	return views
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
		listing.Cards = append(listing.Cards, *l.view(card))
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
		byPull := !column.TakesWorkUp() && carriesInto(column, l.Bench.Columns) != nil
		if !column.TakesWorkUp() && !byPull {
			offer.NoTaker = true
			offer.AwaitingOutside = column.AwaitingOutside
		} else if head := headOfReady(column.ID, cards); head != nil {
			offer.Card = l.view(head)
			offer.TakenByPull = byPull
		}
		offers = append(offers, offer)
	}
	return offers, nil
}

// headOfReady returns the next card pull (or next) would take from the given
// column, or nil if the column holds no ready card. The order is the queue
// order CORE-QUEUE-3 fixes, namely arrival into the current column first with
// the lower card identifier breaking a tie, and only ready cards are eligible.
//
// Reading the cards out of the bench's own latch-free snapshot means the
// returned card may have lapsed in between; both call sites re-read under the
// card's lock inside their own transaction, so a held card here is filtered
// again with `claim`'s state test at the only moment it would matter. A
// pull that reaches a held or blocked card re-reads it under its lock and
// refuses accordingly.
func headOfReady(columnID string, cards []*bench.Card) *bench.Card {
	var ready []*bench.Card
	for _, card := range cards {
		if card.Column == columnID && card.State == contract.StateReady {
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
	// TS is when it was written.
	TS string `json:"ts"`
	// Author is who wrote it.
	Author string `json:"author"`
	// Body is the comment itself.
	Body string `json:"body"`
	// Attachments are the comment's own attachments, on the terms a card's
	// are: the full list, each carrying its path. A comment is one of the
	// four kinds the containment grammar gives an attachments collection,
	// and a card's comments are bounded by that card, so the list costs
	// what the one card costs rather than what a listing costs.
	Attachments []AttachmentView `json:"attachments,omitempty"`
}

// Show reads a card, or the file any other reference names.
//
// A card comes back as a Detail with an empty text. Every other reference
// comes back the other way round, as a nil Detail beside the text of the file
// it named, since nothing but a card has a view to build. A caller reads the
// pair rather than assuming the Detail.
func (l *Library) Show(req *Request) (*Detail, string, error) {
	head, rest, _ := strings.Cut(req.Card, "/")
	// A column is an entity of the workbench, and the containment walk prints
	// a reference for one, so show reads it the way path and edit do rather
	// than refusing over a reference the tool told the reader to type.
	if rest == "" {
		if column := l.Bench.ColumnByRef(head); column != nil {
			text, err := bench.ReadText(l.Bench.ColumnAnchorPath(column.ID))
			if err != nil {
				return nil, "", contract.Refuse(contract.UnknownPath, head)
			}
			return nil, text, nil
		}
	}
	// A composed reference is whatever the resolver reaches, which is why the
	// resolution comes before the card is loaded: the head may name the
	// workbench or a column rather than a card, and every one of those forms is
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
	if err := l.lapseRead(card, req.Actor); err != nil {
		return nil, "", err
	}
	detail := &Detail{Card: *l.view(card), Body: card.Body, Path: card.AnchorPath()}
	for _, link := range card.Links {
		detail.Links = append(detail.Links, LinkView{Kind: link.Kind, To: link.To, Ref: l.linkRef(link.To)})
	}
	cardRef := card.Ref(l.Bench.Slug)
	views, err := attachmentViews(card.Dir, cardRef)
	if err != nil {
		return nil, "", err
	}
	detail.Attachments = views
	comments, err := bench.Comments(card.Dir)
	if err != nil {
		return nil, "", err
	}
	for _, comment := range comments {
		view := CommentView{ID: comment.ID, TS: comment.TS, Author: comment.Author, Body: comment.Body}
		// A comment's attachments compose their references against the
		// comment's own address rather than the card's, so a reference the
		// view prints reaches the attachment the view describes.
		below, err := attachmentViews(comment.Dir, commentRef(cardRef, memberPosition(comment.Dir, bench.CommentAnchor)))
		if err != nil {
			return nil, "", err
		}
		view.Attachments = below
		detail.Comments = append(detail.Comments, view)
	}
	return detail, "", nil
}

// AttachmentListing is one entity's attachments: a workbench's, a column's, a
// card's or a comment's, which are the four kinds the containment grammar
// gives an attachments collection.
type AttachmentListing struct {
	// Kind is the entity's kind, as the containment grammar spells it.
	Kind string `json:"kind"`
	// Ref is what a person types to reach the entity the attachments hang
	// from. The workbench is written `workbench`, which is the spelling the
	// containment tree's own root row already prints for it, and everything
	// else carries the reference the resolver composed.
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
	entity, err := l.Bench.ResolveEntity(req.Ref)
	if err != nil {
		return nil, err
	}
	// EntityRef leaves the workbench's own reference empty, calling its
	// spelling a question the resolver does not settle, so composing an
	// attachment's reference from it would print /attachments/1, which
	// nothing accepts. rootOf already answered that question for the
	// containment tree, and this mirrors its answer rather than minting a
	// second one.
	ref := entity.Ref
	if entity.Kind == bench.KindWorkbench {
		ref = "workbench"
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
		ordinal := displayOrdinal(attachment)
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
func memberPosition(dir, anchor string) int {
	collection := filepath.Dir(dir)
	id := filepath.Base(dir)
	for n, member := range bench.SortByOrdinal(collection, anchor, bench.ListIDs(collection)) {
		if member == id {
			return n + 1
		}
	}
	return 0
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
func (l *Library) Instructions(req *Request) (*Served, error) {
	if column := l.instructionColumn(req); column != nil {
		served := &Served{
			Column: column.ID,
			Instructions: Instructions{
				Global:   bench.GlobalInstructions(l.Home),
				Standing: l.Bench.Standing,
				Column:   column.Instructions,
			},
		}
		return served, nil
	}
	found, err := l.Bench.ResolveCard(req.Card)
	if err != nil {
		return nil, contract.Refuse(contract.UnknownPath, req.Card)
	}
	loop, err := l.cardLoop(found.Card)
	if err != nil {
		return nil, err
	}
	served := &Served{
		Column:       found.Card.Column,
		Instructions: *l.serve(found.Card),
		LegalMoves:   l.legalMoves(found.Card),
		Loop:         loop,
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
// anchor and journal disagree about where it stands.
//
// A non-nil error return still carries a non-nil report when the migration
// ran: the report is what the run had already stamped and already guessed
// before whatever ended it, and a caller that discards it on the error path
// loses that account the same way the run it is reporting on must not.
func (l *Library) Check(req *Request) (*CheckReport, error) {
	report := &CheckReport{}
	if req != nil && req.MigrateSlugs {
		assigned, reported := l.Bench.BackfillColumnSlugs()
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
	if len(r.Findings) > 0 {
		r.Outcome = contract.ReadFindings
	}
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
