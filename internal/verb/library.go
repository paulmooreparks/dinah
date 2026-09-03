package verb

import (
	"sort"
	"time"

	"dinah/internal/bench"
	"dinah/internal/contract"
)

// Library is the one implementation of every verb, over one opened bench.
type Library struct {
	// Bench is the workbench every verb acts on.
	Bench *bench.Bench
	// Home is the user base, which carries the user's config and the
	// user-global instruction layer.
	Home string
	// Now is the clock. It is a field so that a test can advance time past
	// a recorded expiry without waiting for it.
	Now func() time.Time
	// Interleave, when set, is called inside a mutation's transaction, after
	// the entity's lock has been taken and before the work the lock covers:
	// in a contract verb, after the card has been read and before any
	// precondition is evaluated against it, and in an attach, before
	// anything is written. It exists so that a test can drive a second
	// process into the middle of a transaction and observe that the lock
	// refuses it there, which is the window a lock taken only around the
	// write would leave open.
	Interleave func()
}

// New returns a library over an opened bench, on the real clock.
func New(b *bench.Bench, home string) *Library {
	return &Library{Bench: b, Home: home, Now: time.Now}
}

// Request is what a head hands the library. One shape covers every verb, so a
// head parses arguments and never decides what a verb means.
type Request struct {
	// Verb is the command being asked for.
	Verb string
	// Actor is the owner the act is attributed to, resolved by the ladder
	// before the request is made. An empty actor is refused inside the
	// verb's own order rather than ahead of it.
	Actor string
	// Card is the card reference the verb names.
	Card string
	// Ref is the entity reference the entity-shaped commands name.
	Ref string
	// Column is the destination a move names, or the column a read narrows to.
	Column string
	// Action is the word a command dispatching on its own first word was
	// given: get, set, or empty for the bare invocation.
	Action string
	// Field is the workbench or workstream field a read or a write names.
	Field string
	// Workstream is the workstream slot of the commands that name one. It
	// carries the reference join, leave, get and set resolve, and it carries
	// the title `workstream new` files, which is the one slot the parameter
	// list spells workstream|title.
	Workstream string
	// Slug is the slug a creation call supplies (`workstream new`, `column
	// new`), so a caller may finish provisioning what it just made without a
	// later write it is not permitted to make. Empty means derive one from
	// the title, exactly as creation has always done.
	Slug string
	// Value is what a write puts in that field.
	Value string
	// Severity and Priority are the levels a filing names, empty when the
	// invocation named none. They are separate from Value because add names
	// both axes at once through two flags where card names one axis by
	// field.
	Severity string
	Priority string
	// Holder is the owner a claim names as holder, which the pull discipline
	// requires to be the owner asking.
	Holder string
	// Expires is the duration a claim's lease runs for.
	Expires time.Duration
	// Reason and Kind are a block's prose reason and its optional class, or
	// the kind a column creation names.
	Reason string
	Kind   string
	// Capacity is the wip_limit a column creation names, as the caller wrote
	// it, empty for unlimited. It is a string for the reason MaxDepth is:
	// the verb that reads it parses it, so the request builder never has to
	// know what any one verb's arguments mean.
	Capacity string
	// Before is the column a column creation places the new column
	// immediately ahead of, empty to append at the end of the flow.
	Before string
	// Override is the marker CORE-MOVE-9 admits and CORE-MOVE-11 reserves.
	Override bool
	// NoClaim is the marker a pull carries to move a card without claiming
	// it. The card lands in the ready state an ordinary move leaves, so
	// the next owner to claim it or to pull it onward takes it from there.
	// The marker weakens no precondition: a pull still runs the claim's own
	// rows, so a card a claim would refuse is a card a pull refuses.
	NoClaim bool
	// Basis is the revision the owner read before deciding.
	Basis string
	// Title is the title a new card carries.
	Title string
	// Text is a comment's body.
	Text string
	// File is the path an attachment's bytes are copied from.
	File string
	// Description is an attachment's optional description.
	Description string
	// Replace aims an attach at an existing attachment's payload.
	Replace bool
	// Confirm is the deliberate flag a delete requires.
	Confirm bool
	// Query is the query string the query command reads, carried byte for
	// byte as the caller wrote it, since Matches echoes what it was given
	// rather than what the parser made of it.
	Query string
	// GroupBy is the axis chain a tree nests along, as the caller wrote it:
	// one comma-separated word, empty for the default chain.
	GroupBy string
	// Depth is the named level a tree projection stops at, empty for the
	// command's own default.
	Depth string
	// ReadyOnly narrows a listing to the cards whose state is ready.
	ReadyOnly bool
	// Since is the opaque cursor a checkpoint hands back, empty on a first
	// call, which mints one rather than replaying the board's history.
	Since string
	// Root is the root a root-scoped read walks from, as the caller wrote it,
	// empty for the ordinary single-workbench read. A read carrying one
	// answers for every workbench beneath it rather than for the one the
	// caller is standing in.
	Root string
	// MaxDepth is the root walk's depth bound, as the caller wrote it, empty
	// for the surface's own default, which is bench.DefaultEnumerateDepth. It
	// is a string for the same reason GroupBy and Depth are: the verb that
	// reads a flag parses it, so the request builder never has to know what
	// any one verb's arguments mean.
	MaxDepth string
	// Finish asks check to complete or roll back the interrupted structural
	// acts it reports, rather than only reporting them.
	Finish bool
	// MigrateOrdinals asks check to stamp a creation ordinal on every entity
	// of the workbench that predates the field, before it reports.
	MigrateOrdinals bool
	// MigrateSlugs asks check to derive a slug for every column of the
	// workbench that predates the field, before it reports.
	MigrateSlugs bool
	// MigrateColumns asks check to remove every stranded identifier from the
	// workbench's own columns list, before it reports.
	MigrateColumns bool
	// MigrateVocabulary asks check to carry every workbench at or beneath the
	// discovered root from the retired state and substate vocabulary to the
	// current column and state one. It is the one marker check reads before a
	// workbench is opened rather than after, because a workbench still written
	// in the old vocabulary is exactly what the ordinary open refuses.
	MigrateVocabulary bool
	// MigrateContainer asks check to carry every workbench at or beneath the
	// root into a .dinah container under an identifier Dinah minted. Like
	// MigrateVocabulary it is read before a workbench is opened rather than
	// after, because a workbench outside a container is exactly what the
	// ordinary open now refuses.
	MigrateContainer bool
	// Remint is the one workbench directory the container repair reminents,
	// as the caller wrote it, empty when the caller asked for no remint. It
	// carries a path rather than a flag because the choice of which of two
	// colliding directories keeps its identifier is the operator's, and this
	// is where he states it.
	Remint string
	// MigrateWorkstreams asks check to create a workstream at every
	// identifier the live cards list that names none, before it reports.
	MigrateWorkstreams bool
	// MigrateWitness asks check to witness every live card whose anchor and
	// journal disagree about its position, before it reports. It carries no
	// migrate- prefix on the flag because it repairs a disagreement that is
	// true right now rather than carrying a workbench past an older shape.
	MigrateWitness bool
	// WorkbenchSource names the rung that resolved the active workbench for
	// this invocation (flag, environment, search, or config), set by the
	// head once discovery has run, since that is the earliest point the
	// answer is known.
	WorkbenchSource string
}

// CardView is the card as a response carries it.
type CardView struct {
	// ID is the card's 12-hex identifier.
	ID string `json:"id"`
	// Ref is the card's human reference.
	Ref string `json:"ref,omitempty"`
	// Title is the card's title.
	Title string `json:"title,omitempty"`
	// Column is the identifier of the column the card occupies.
	Column string `json:"column,omitempty"`
	// ColumnTitle is that column's title, so a reader needs no second call.
	ColumnTitle string `json:"column_title,omitempty"`
	// State is one of ready, active and blocked.
	State string `json:"state,omitempty"`
	// Severity and Priority are the levels the card records on the two axes
	// a workbench may declare, empty when the card carries none. Displayed
	// verbatim: no lookup against Bench.Levels or Bench.Level runs here, so
	// an undeclared level is shown exactly as stored (dinah-193 D-2) and no
	// hint or rank ever reaches this surface.
	Severity string `json:"severity,omitempty"`
	Priority string `json:"priority,omitempty"`
	// Holder is the owner holding the card.
	Holder string `json:"holder,omitempty"`
	// ClaimSince is when the claim began.
	ClaimSince string `json:"claim_since,omitempty"`
	// Expires is when the lease lapses.
	Expires string `json:"expires,omitempty"`
	// BlockReason and BlockKind carry the obstacle.
	BlockReason string `json:"block_reason,omitempty"`
	BlockKind   string `json:"block_kind,omitempty"`
	// Workstreams are the identifiers of the workstreams the card belongs
	// to, reported as the card's frontmatter stores them. A reader wanting
	// the slugs resolves them, the way a reader of a link's to already does.
	Workstreams []string `json:"workstreams,omitempty"`
	// Revision is the card's opaque current revision.
	Revision string `json:"revision"`
	// AttachmentCount is how many attachments the card carries. A listing
	// reports the number rather than the list, which is what a reader needs
	// in order to decide whether the card has anything to open, and the
	// attachments themselves are one read away for the one card somebody
	// asked about.
	AttachmentCount int `json:"attachment_count,omitempty"`
}

// Instructions are the three layers of the served chain, carried separately
// so that no layer is ever written into another.
type Instructions struct {
	// Global is the user-global layer, absent on a machine carrying none.
	Global string `json:"global,omitempty"`
	// Standing is the workbench's own standing text.
	Standing string `json:"standing,omitempty"`
	// Column is the station's own instructions.
	Column string `json:"column,omitempty"`
}

// Loop reports one card's regressive-departure count against the declared
// loop_limit of the column it is standing in. It is served only where that
// column declares one, so an agent holding a card in a bounded review loop
// reads how far round it has been without replaying a journal, and an agent
// anywhere else reads nothing extra.
type Loop struct {
	// Column is what a person types to reach the declaring column, which is
	// the departure the limit is counted at rather than any destination.
	Column string `json:"column"`
	// Limit is the column's own declared loop_limit.
	Limit int `json:"limit"`
	// Count is how many times this card has already left that column by a
	// regressive move.
	Count int `json:"count"`
	// AtLimit says the count has reached the limit, so the next regressive
	// move out of the column is refused and only the operator's --override
	// carries it. It stays true once reached: an override carries one move
	// and does not reset the count.
	AtLimit bool `json:"at_limit"`
}

// LegalMove is one departure the workbench allows a card at this moment.
type LegalMove struct {
	// Column is the destination's identifier.
	Column string `json:"column"`
	// Ref is what a person types to name this destination on a move: the
	// column's own slug when it has one, the identifier otherwise. Mirrors
	// CardView.Ref's fallback, so a column written before the slug field
	// existed still gives a caller something to type.
	Ref string `json:"ref"`
	// Title is the destination's title.
	Title string `json:"title"`
	// Direction is forward or backward along the declared flow.
	Direction string `json:"direction"`
	// Reject marks the one row, if any, that the departure's own reject_to
	// declaration names. At most one row of a card's legal moves ever
	// carries this, since RejectTarget answers at most one column.
	Reject bool `json:"reject,omitempty"`
}

// The two directions a legal move can carry.
const (
	Forward  = "forward"
	Backward = "backward"
)

// Response is the canonical form of a verb's answer, and the frozen machine
// contract both heads project.
type Response struct {
	// Outcome is one of ok, refused, stale and unreachable.
	Outcome string `json:"outcome"`
	// Verb is the command that produced the response.
	Verb string `json:"verb"`
	// Refusal is the one refusal name an outcome of refused carries.
	Refusal string `json:"refusal,omitempty"`
	// Detail names what the refusal was about, as a machine token rather
	// than a sentence. A head renders the sentence from the catalog.
	Detail string `json:"detail,omitempty"`
	// Card is the card as it now stands.
	Card *CardView `json:"card,omitempty"`
	// Workstream is the workstream as it now stands, on the responses of the
	// two acts whose subject is a workstream rather than a card.
	Workstream *WorkstreamView `json:"workstream,omitempty"`
	// Column is the column as it now stands, on the one act whose subject is
	// a column rather than a card or a workstream.
	Column *ColumnView `json:"column,omitempty"`
	// Basis is the revision the request was evaluated against.
	Basis string `json:"basis,omitempty"`
	// Instructions are the served layers, on a claim or a move that
	// succeeded and nowhere else.
	Instructions *Instructions `json:"instructions,omitempty"`
	// LegalMoves are the moves legal for the card at this moment.
	LegalMoves []LegalMove `json:"legal_moves,omitempty"`
	// Loop is the card's standing against its column's declared loop_limit,
	// absent where the column declares none.
	Loop *Loop `json:"loop,omitempty"`
	// Affordances name what the caller may do next with the entity the
	// response concerns, on every response whatever its outcome.
	Affordances []string `json:"affordances"`
	// Warning is a catalog key for something the caller should know that
	// did not stop the act, such as a card reference carrying a prefix that
	// names no current slug.
	Warning string `json:"warning,omitempty"`
	// WarningDetail is the token the warning is about.
	WarningDetail string `json:"warning_detail,omitempty"`
	// Context carries the refusal's named values as data, absent on a
	// response that needs none. It is what refusalReport already calls
	// context, so a caller parsing --json reads one shape whichever layer
	// said no.
	Context map[string]string `json:"context,omitempty"`
	// Message is a catalog key the head reads to compose the one-line
	// sentence printed on a cardless OK response, the empty answer a read
	// or a mutation can give. Empty when the response carries a card or
	// carries no printable line.
	Message string `json:"message,omitempty"`
	// MessageValues are the named values a Message template inserts, and it
	// is nil for a sentence carrying no slot.
	MessageValues map[string]string `json:"message_values,omitempty"`
}

// view renders a card for a response.
func (l *Library) view(card *bench.Card) *CardView {
	v := &CardView{
		ID:          card.ID,
		Ref:         card.Ref(l.Bench.Slug),
		Title:       card.Title,
		Column:      card.Column,
		State:       card.State,
		Severity:    card.Severity,
		Priority:    card.Priority,
		Holder:      card.Holder,
		ClaimSince:  card.ClaimSince,
		Expires:     card.Expires,
		BlockReason: card.BlockReason,
		BlockKind:   card.BlockKind,
		Workstreams: card.Workstreams,
		Revision:    card.Revision,

		AttachmentCount: bench.CountAttachments(card.Dir),
	}
	if column := l.Bench.Column(card.Column); column != nil {
		v.ColumnTitle = column.Title
	}
	return v
}

// serve composes the instruction chain for a card's current position. Nothing
// is stored anywhere, so an edit to any layer reaches every reader on the
// next serve, and no layer carries another's text.
func (l *Library) serve(card *bench.Card) *Instructions {
	instructions := &Instructions{
		Global:   bench.GlobalInstructions(l.Home),
		Standing: l.Bench.Standing,
	}
	if column := l.Bench.Column(card.Column); column != nil {
		instructions.Column = column.Instructions
	}
	return instructions
}

// legalMoves reports the departures the workbench allows a card now. A card in
// a column whose kind is done has no forward move, which is CORE-STATE-9.
func (l *Library) legalMoves(card *bench.Card) []LegalMove {
	current := l.Bench.Column(card.Column)
	if current == nil {
		return nil
	}
	target := l.Bench.RejectTarget(current)
	var moves []LegalMove
	for _, column := range l.Bench.Columns {
		if column.ID == current.ID {
			continue
		}
		direction := Backward
		if column.Position > current.Position {
			direction = Forward
		}
		if direction == Forward && current.Terminal() {
			continue
		}
		moves = append(moves, LegalMove{
			Column:    column.ID,
			Ref:       columnRef(column),
			Title:     column.Title,
			Direction: direction,
			Reject:    target != nil && column.ID == target.ID,
		})
	}
	return moves
}

// cardLoop composes the loop block for a card, and answers nil where the card
// stands at a column no longer declared or at one declaring no loop_limit.
//
// The journal read is the cost of the block, and it is paid only at a
// declaring column. The error is returned rather than swallowed, because a
// journal this call could not read would otherwise serve a count of zero,
// which reads as a card at the start of a loop rather than as an unanswered
// question.
func (l *Library) cardLoop(card *bench.Card) (*Loop, error) {
	column := l.Bench.Column(card.Column)
	if column == nil || column.LoopLimit <= 0 {
		return nil, nil
	}
	events, _, err := bench.ReadJournal(card.JournalPath())
	if err != nil {
		return nil, err
	}
	count := l.Bench.RegressiveDepartures(events, column.ID)
	return &Loop{
		Column:  columnRef(column),
		Limit:   column.LoopLimit,
		Count:   count,
		AtLimit: count >= column.LoopLimit,
	}, nil
}

// columnRef is what a person types to reach a column. Thin wrapper over
// bench.Column.Ref so every caller in this package reads the same name it
// already used before that method existed.
func columnRef(column *bench.Column) string {
	return column.Ref()
}

// affordances names what a caller may do next with a card, which is the same
// question every response answers whatever its outcome.
//
// The ready list asks where the card is standing rather than deciding from the
// state alone. A claim is refused at a column that takes no work up, so a
// list naming claim there advertises an act the tool refuses, and the reader
// most likely to act on it is an agent that cannot see the board.
func (l *Library) affordances(card *bench.Card) []string {
	if card == nil {
		return []string{"status", "columns", "ls", "next"}
	}
	switch card.State {
	case contract.StateReady:
		return append(l.takeUpActs(l.Bench.Column(card.Column)), Move, Block, "comment", "show", "log")
	case contract.StateActive:
		return []string{Move, Release, Block, "comment", "show", "log"}
	case contract.StateBlocked:
		return []string{Unblock, "comment", "show", "log"}
	}
	return []string{"show", "log"}
}

// takeUpActs names the act that would take a ready card up at a column. It
// asks the two predicates the acts themselves ask rather than repeating
// either rule: claimableColumn admits a claim exactly where the column holds an
// active card, and carriesInto answers where a pull would put a card standing
// at this one, which is nil when no pull can reach it.
//
// It takes the column rather than a card because the question is the column's
// alone, and because the instructions chain is served for a bare column as
// often as for a card. A caller holding either one reaches the same rule.
//
// A column taking no work up loses the claim and gains a pull, because an agent
// reading a list with the claim simply missing would meet the refusal with
// nothing telling it what to reach for instead. That is the same reason the
// next_card tool's list carries pull beside claim.
func (l *Library) takeUpActs(column *bench.Column) []string {
	if column == nil || column.HoldsState(contract.StateActive) {
		return []string{Claim}
	}
	if carriesInto(column, l.Bench.Columns) != nil {
		return []string{Pull}
	}
	return nil
}

// CardAffordances answers, for the card a request names, the list every
// card-shaped response already carries. A head that assembles its own payload
// instead of returning a Response reaches this rather than writing out a list
// of its own, since a written-out list is a second answer to the question
// affordances answers and goes stale the moment an act's rules change. A
// reference naming no card falls back to the list a response with no card
// carries, which spells two of its reads as library commands. A head serving
// a vocabulary of its own translates what it publishes; the mcp head does
// that in surfaceAffordances.
func (l *Library) CardAffordances(req *Request) []string {
	found, err := l.Bench.ResolveCard(req.Card)
	if err != nil || found == nil {
		return l.affordances(nil)
	}
	return l.affordances(found.Card)
}

// ServedAffordances answers what a caller may do where an instruction chain
// was served. Instructions answers for a card or for a bare column, and this
// answers for whichever of the two the request reached, from the one rule the
// card list already reads.
//
// A written-out list here is worse than a written-out list anywhere else,
// because instructions is the tool a caller reaches for precisely to learn
// what it may do where the card is standing. A list naming claim at a column
// that takes no work up walks the reader into the refusal it came here to
// avoid, and it does so in the answer that was supposed to prevent that.
//
// The bare-column branch carries no card acts, since there is no card to move,
// comment on or read. What it carries is the act that would take work up here
// and the two reads that find a card to take it with.
func (l *Library) ServedAffordances(req *Request, served *Served) []string {
	if served == nil {
		return nil
	}
	if l.instructionColumn(req) != nil {
		return append(l.takeUpActs(l.Bench.Column(served.Column)), "ls", "next", "show")
	}
	return l.CardAffordances(req)
}

// refuse builds a refused response. It keeps its signature and delegates to
// refuseWith with no named values, so none of its call sites is edited and no
// precondition sequence in this package carries a diff line.
func (l *Library) refuse(req *Request, card *bench.Card, name, detail string) *Response {
	return l.refuseWith(req, card, name, detail, nil)
}

// refuseWith builds a refused response carrying the refusal's named values,
// for a raise site holding something the sentence needs and the detail alone
// cannot say.
func (l *Library) refuseWith(req *Request, card *bench.Card, name, detail string, extra map[string]string) *Response {
	response := &Response{
		Outcome:     contract.OutcomeRefused,
		Verb:        req.Verb,
		Refusal:     name,
		Detail:      detail,
		Affordances: l.affordances(card),
		Basis:       req.Basis,
		Context:     extra,
	}
	if card != nil {
		response.Card = l.view(card)
	}
	return response
}

// ok builds a successful response.
func (l *Library) ok(req *Request, card *bench.Card) *Response {
	response := &Response{
		Outcome:     contract.OutcomeOK,
		Verb:        req.Verb,
		Affordances: l.affordances(card),
		Basis:       req.Basis,
	}
	if card != nil {
		response.Card = l.view(card)
		response.Basis = card.Revision
	}
	return response
}

// FromError turns an error the format layer returned into a response, so that
// a head never has to decide what an error means.
func (l *Library) FromError(req *Request, err error) *Response {
	switch typed := err.(type) {
	case *contract.Refusal:
		// A refusal always travels through ComposeRefusal, so a caller with a
		// library and a caller without one compose the same shape. The
		// affordances default to the no-card set and a card the library was
		// holding never reaches this branch, so passing through ComposeRefusal
		// rather than refuseWith drops the card the library might otherwise
		// attach.
		return ComposeRefusal(req, typed)
	case *contract.Stale:
		response := &Response{
			Outcome:     contract.OutcomeStale,
			Verb:        req.Verb,
			Basis:       typed.Basis,
			Affordances: l.affordances(nil),
		}
		return response
	case *contract.Unreachable:
		return &Response{
			Outcome:     contract.OutcomeUnreachable,
			Verb:        req.Verb,
			Detail:      typed.Detail,
			Affordances: l.affordances(nil),
		}
	}
	return &Response{
		Outcome:     contract.OutcomeUnreachable,
		Verb:        req.Verb,
		Detail:      err.Error(),
		Affordances: l.affordances(nil),
	}
}

// sortByArrival orders cards the way CORE-QUEUE-3 fixes.
func sortByArrival(cards []*bench.Card) {
	sort.SliceStable(cards, func(i, j int) bool {
		return bench.ByArrival(cards[i], cards[j])
	})
}
