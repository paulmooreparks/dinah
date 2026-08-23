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
	// lastActor and lastOverride carry the values Pull set on entry, so the
	// qualifying predicate reads the same actor and override the caller
	// named without threading them through every helper. They are populated
	// by Pull only and read by pullCandidates while it runs.
	lastActor    string
	lastOverride bool
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
	// State is the destination a move names, or the state a read narrows to.
	State string
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
	// Value is what a write puts in that field.
	Value string
	// Holder is the owner a claim names as holder, which the pull discipline
	// requires to be the owner asking.
	Holder string
	// Expires is the duration a claim's lease runs for.
	Expires time.Duration
	// Reason and Kind are a block's prose reason and its optional class.
	Reason string
	Kind   string
	// Override is the marker CORE-MOVE-9 admits and CORE-MOVE-11 reserves.
	Override bool
	// NoClaim is the marker a pull uses to skip the claimed event and the
	// claim stamp, so a pull naming --no-claim lands a card without taking
	// its lease. The card's substate still becomes active because the move
	// half of the transaction writes it; a follow-up claim by some other
	// owner sees an active, unheld card and acts on it then.
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
	// ReadyOnly narrows a listing to the cards whose substate is ready.
	ReadyOnly bool
	// Finish asks check to complete or roll back the interrupted structural
	// acts it reports, rather than only reporting them.
	Finish bool
	// MigrateOrdinals asks check to stamp a creation ordinal on every entity
	// of the workbench that predates the field, before it reports.
	MigrateOrdinals bool
	// MigrateSlugs asks check to derive a slug for every state of the
	// workbench that predates the field, before it reports.
	MigrateSlugs bool
	// MigrateStates asks check to remove every stranded identifier from the
	// workbench's own states list, before it reports.
	MigrateStates bool
	// MigrateWorkstreams asks check to create a workstream at every
	// identifier the live cards list that names none, before it reports.
	MigrateWorkstreams bool
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
	// State is the identifier of the state the card occupies.
	State string `json:"state,omitempty"`
	// StateTitle is that state's title, so a reader needs no second call.
	StateTitle string `json:"state_title,omitempty"`
	// Substate is one of ready, active and blocked.
	Substate string `json:"substate,omitempty"`
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
}

// Instructions are the three layers of the served chain, carried separately
// so that no layer is ever written into another.
type Instructions struct {
	// Global is the user-global layer, absent on a machine carrying none.
	Global string `json:"global,omitempty"`
	// Standing is the workbench's own standing text.
	Standing string `json:"standing,omitempty"`
	// State is the station's own instructions.
	State string `json:"state,omitempty"`
}

// LegalMove is one departure the workbench allows a card at this moment.
type LegalMove struct {
	// State is the destination's identifier.
	State string `json:"state"`
	// Ref is what a person types to name this destination on a move: the
	// state's own slug when it has one, the identifier otherwise. Mirrors
	// CardView.Ref's fallback, so a state written before the slug field
	// existed still gives a caller something to type.
	Ref string `json:"ref"`
	// Title is the destination's title.
	Title string `json:"title"`
	// Direction is forward or backward along the declared flow.
	Direction string `json:"direction"`
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
	// Basis is the revision the request was evaluated against.
	Basis string `json:"basis,omitempty"`
	// Instructions are the served layers, on a claim or a move that
	// succeeded and nowhere else.
	Instructions *Instructions `json:"instructions,omitempty"`
	// LegalMoves are the moves legal for the card at this moment.
	LegalMoves []LegalMove `json:"legal_moves,omitempty"`
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
	// carries no printable line. Substitution values live on MessageDetail
	// for a single token and on MessageValues for the named-value map a
	// templated catalog entry needs.
	Message string `json:"message,omitempty"`
	// MessageDetail is the single token a Message template inserts.
	MessageDetail string `json:"message_detail,omitempty"`
	// MessageValues is the named-value map a Message template inserts.
	MessageValues map[string]string `json:"message_values,omitempty"`
}

// view renders a card for a response.
func (l *Library) view(card *bench.Card) *CardView {
	v := &CardView{
		ID:          card.ID,
		Ref:         card.Ref(l.Bench.Slug),
		Title:       card.Title,
		State:       card.State,
		Substate:    card.Substate,
		Holder:      card.Holder,
		ClaimSince:  card.ClaimSince,
		Expires:     card.Expires,
		BlockReason: card.BlockReason,
		BlockKind:   card.BlockKind,
		Workstreams: card.Workstreams,
		Revision:    card.Revision,
	}
	if state := l.Bench.State(card.State); state != nil {
		v.StateTitle = state.Title
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
	if state := l.Bench.State(card.State); state != nil {
		instructions.State = state.Instructions
	}
	return instructions
}

// legalMoves reports the departures the workbench allows a card now. A card in
// a state whose kind is done has no forward move, which is CORE-STATE-9.
func (l *Library) legalMoves(card *bench.Card) []LegalMove {
	current := l.Bench.State(card.State)
	if current == nil {
		return nil
	}
	var moves []LegalMove
	for _, state := range l.Bench.States {
		if state.ID == current.ID {
			continue
		}
		direction := Backward
		if state.Position > current.Position {
			direction = Forward
		}
		if direction == Forward && current.Kind == contract.KindDone {
			continue
		}
		moves = append(moves, LegalMove{State: state.ID, Ref: stateRef(state), Title: state.Title, Direction: direction})
	}
	return moves
}

// stateRef is what a person types to reach a state. Thin wrapper over
// bench.State.Ref so every caller in this package reads the same name it
// already used before that method existed.
func stateRef(state *bench.State) string {
	return state.Ref()
}

// affordances names what a caller may do next with a card, which is the same
// question every response answers whatever its outcome.
func (l *Library) affordances(card *bench.Card) []string {
	if card == nil {
		return []string{"status", "states", "ls", "next"}
	}
	switch card.Substate {
	case contract.SubstateReady:
		return []string{Pull, Claim, Move, Block, "comment", "show", "log"}
	case contract.SubstateActive:
		return []string{Move, Release, Block, "comment", "show", "log"}
	case contract.SubstateBlocked:
		return []string{Unblock, "comment", "show", "log"}
	}
	return []string{"show", "log"}
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
