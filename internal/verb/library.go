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
	// Interpose, when set, is called between the steps of a write phase that
	// composes several locked acts rather than running inside one, at the
	// points where the run holds no lock at all. It is the complement of
	// Interleave: that one drives a second process at a lock it must be
	// refused by, this one drives a second process through a window where
	// nothing refuses it, so a test can construct the interleaving a composed
	// write phase really admits instead of reasoning about it. The step names
	// which window the run is standing in, because the answer a test wants
	// differs per step. Reshape calls it between its own steps, and the
	// checklist item verbs call it in the one window before the card lock is
	// taken, where a test runs a whole second write and then asserts that the
	// first one reads it rather than overwriting it.
	Interpose func(step string)
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
	// Tier is what a claim, a next and a pull declare about the caller, what a
	// column creation gives the new column as its default, and what a raise
	// asks the card to require at the column it is standing in, which are three
	// different acts sharing one argument name the way Kind already serves a
	// block and a column creation. On the three reading and claiming verbs the
	// value is self-reported and nothing verifies it: the claim gate refuses
	// below it and selection withholds above it, and neither establishes
	// anything about who is asking. A raise's value is an expression rather
	// than a name: it resolves through ResolveTierWrite exactly as a per-column
	// tier write does, so it may be a declared member or a relative step.
	Tier string
	// At is the column a per-column write names, empty when the write is
	// about the card as a whole. A tier write carrying it sets an override
	// for that column instead of the card's own baseline.
	At string
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
	// Owner is the owner a checklist item names, blank for none said. It is
	// recorded on the item and never enforced against the actor calling a
	// terminal verb, on the terms Comment is unauthenticated today.
	Owner string
	// Scheme and CiteTarget are a citation's scheme and the target that
	// scheme names, each as the caller typed it. Nothing resolves either at
	// write time.
	Scheme     string
	CiteTarget string
	// Observed is the raw before:after pair a citation may carry, parsed
	// inside cite for the reason MaxDepth and GroupBy are parsed inside the
	// verbs that read them rather than at the request builder.
	Observed string
	// Note is the resolution note the three terminal checklist verbs
	// require. It is separate from Reason, which reopen takes, because the
	// two say different things: a note is what the check showed, and a
	// reason is why a closed item is being opened again.
	Note string
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
	// SearchText is the phrase dinah search matches against every field it
	// reads. It is distinct from Query, which narrows the card set the phrase
	// is run over rather than naming what it is run against.
	SearchText string
	// Archived asks search to include the archived half of the cards
	// collection in its scan, which is otherwise left out on the format's own
	// rule that the archive is read on demand.
	Archived bool
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
	// From is the source a reshape reads its new column layout from, as the
	// caller wrote it: an interchange file or another workbench's directory,
	// which are the two shapes `dinah init --from` already accepts.
	From string
	// Map is the reshape destinations the caller named, each one written
	// `<retired>=<destination>`, in the order they were written. It is a list
	// rather than a single value because one run retires as many columns as
	// the new definition drops, and a later entry for the same retirement
	// wins over an earlier one.
	Map []string
	// Fields is show's field list, carried byte for byte as the caller wrote
	// it, since Library.Show parses it: the request builder never has to know
	// what any one verb's arguments mean. Empty means every member, which is
	// what show has always answered, so no head diverges from another on an
	// unasked question.
	Fields string
	// HeldChain is the set of instruction-layer keys this caller's connection
	// has already been sent and has not yet re-served, each one written
	// <actor> + "\x00" + <text revision>. The MCP head fills it from its own
	// connection state, having already dropped every expired record; every
	// other head leaves it nil, which serves the whole chain. The field is
	// declared in no Params entry and in no injectedProperties row, so it
	// reaches no published schema and no caller can supply one.
	HeldChain map[string]bool
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
	// BlockingItems is how many of the card's checklist items are, right
	// now, ones CORE-CLAIM-9 would refuse a claim over. A reader sees the
	// refusal coming rather than meeting it and being told afterwards.
	BlockingItems int `json:"blocking_items,omitempty"`
}

// The three names a withheld layer is reported under, general to specific,
// which is the order CORE-INSTR-11 fixes for a response serving both of the
// layers the profile names. They are minted here so that no caller and no test
// spells one by hand.
const (
	LayerGlobal   = "global"
	LayerStanding = "standing"
	LayerColumn   = "column"
)

// Instructions are the three layers of the served chain, carried separately
// so that no layer is ever written into another.
type Instructions struct {
	// Global is the user-global layer, absent on a machine carrying none.
	Global string `json:"global,omitempty"`
	// Standing is the workbench's own standing text.
	Standing string `json:"standing,omitempty"`
	// Column is the station's own instructions.
	Column string `json:"column,omitempty"`
	// Withheld names the layers this response did not carry because this
	// connection has already sent this owner their current text. A name here
	// is a positive statement rather than a silence: it says the layer's
	// current text is byte-identical to text this connection already served
	// this owner. A layer that is neither carried nor named here is empty.
	Withheld []string `json:"withheld,omitempty"`
	// Reread is the reference an instructions call names to be served the
	// withheld layers in full, which is the column's own ref. It is present
	// exactly when Withheld is non-empty, so an agent that has lost the text
	// recovers from the marker alone.
	Reread string `json:"reread,omitempty"`
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
	// ChainServed carries the instruction-chain keys this act served in full,
	// which is what a head holding a connection records against the owner. The
	// tag keeps the member off every payload, so the machine contract section 5
	// of the profile freezes is untouched and no head has to be told to ignore
	// it.
	ChainServed []string `json:"-"`
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
		BlockingItems:   bench.CountBlockingItems(card.Dir),
	}
	if column := l.Bench.Column(card.Column); column != nil {
		v.ColumnTitle = column.Title
	}
	return v
}

// serve composes the instruction chain for a card's current position, and
// reports the chain keys the act actually served. A layer whose current text
// this request's connection has already sent this owner is withheld and named
// instead, which is the rule CORE-INSTR-8 and CORE-INSTR-9 publish.
//
// This function stores nothing and holds no clock. It reads the set the head
// filled and it answers from the bench it was given, so what an edit reaches
// depends on the head that calls it. The cli head opens the workbench once per
// invocation, so every layer it serves is the text on disk. The MCP head opens
// the library once before it begins serving, so under that head the standing
// text and the column text are frozen for the life of the process and only the
// user-global layer is read from disk on each serve.
func (l *Library) serve(req *Request, card *bench.Card) (*Instructions, []string) {
	return l.composeChain(req, l.Bench.Column(card.Column), true)
}

// chainKey is what one served layer is recorded under: the owner it was served
// to, then the revision of the text that was served. The text is the whole of a
// layer's identity, so two workbenches whose standing text is byte-identical
// hold each other's layer. That is correct rather than a loophole, because the
// owner holds the text and the card on the response says which workbench it
// came from.
func chainKey(actor, text string) string {
	return actor + "\x00" + bench.TextRevision(text)
}

// composeChain builds the instruction chain for a column and reports the keys
// it served. withhold says whether the request's held set may suppress a layer.
// A card-shaped request answers where a card stands and is subject to the rule,
// and a column-shaped request names the text itself and is the recovery route,
// so the second serves in full whatever the connection holds.
//
// A column this workbench no longer declares withholds nothing, because the
// marker owes the agent a reference that fetches the layers back and there is
// no column to name in one.
func (l *Library) composeChain(req *Request, column *bench.Column, withhold bool) (*Instructions, []string) {
	if column == nil {
		withhold = false
	}
	instructions := &Instructions{}
	var served []string
	layer := func(name, text string, into *string) {
		if text == "" {
			return
		}
		key := chainKey(req.Actor, text)
		if withhold && req.HeldChain[key] {
			instructions.Withheld = append(instructions.Withheld, name)
			return
		}
		*into = text
		served = append(served, key)
	}
	layer(LayerGlobal, bench.GlobalInstructions(l.Home), &instructions.Global)
	layer(LayerStanding, l.Bench.Standing, &instructions.Standing)
	if column != nil {
		layer(LayerColumn, column.Instructions, &instructions.Column)
	}
	if len(instructions.Withheld) > 0 {
		instructions.Reread = columnRef(column)
	}
	return instructions, served
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
