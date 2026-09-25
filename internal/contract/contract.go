// Package contract holds the machine vocabulary of the core profile: the
// outcome tokens, the declared refusal names, the states, the column kinds
// and the journal event names, together with the error types a verb returns
// when it refuses or finds the caller's knowledge out of date.
//
// The vocabulary lives in one package because every layer above it needs the
// same spellings. A refusal name invented at a call site is a second
// vocabulary for the same refusal, which the Go style standard's fifth rule
// forbids and the conformance suite would not recognise.
package contract

import (
	"fmt"
	"strings"
)

// The four outcome tokens of CORE-OUT-1. Every verb response carries exactly
// one of them.
const (
	OutcomeOK          = "ok"
	OutcomeRefused     = "refused"
	OutcomeStale       = "stale"
	OutcomeUnreachable = "unreachable"
)

// ExitCode returns the process exit code a given outcome carries: 0 for ok,
// 2 for refused, 3 for stale and 4 for unreachable. An outcome the profile
// does not declare exits 1, which no conforming response can produce.
func ExitCode(outcome string) int {
	switch outcome {
	case OutcomeOK:
		return 0
	case OutcomeRefused:
		return 2
	case OutcomeStale:
		return 3
	case OutcomeUnreachable:
		return 4
	}
	return 1
}

// The two outcome tokens a structural read reports. They are a vocabulary of
// their own and not a fifth and sixth member of the four above, which
// CORE-OUT-1 closes at four for a verb's outcome. A read that dinah check or
// a tree-wide vocabulary migration performs is not a verb the core profile
// governs. A read's own outcome is never refused, stale or unreachable
// either: an invocation that could not attempt the read at all is reported
// through the ordinary refusal path before the read runs, and never carries
// one of these values (dinah-346).
//
// A token minted here does cost a profile revision, which reverses what
// dinah-346 recorded in this comment. The core profile published CORE-OUT-7
// at 0.9, holding a tool that answers an outcome as a number to giving
// `refused` a number no other outcome it reports uses, so the exit code a
// token here resolves to falls inside the profile's scope and moves the
// minor number when it changes. dinah-358 made that ruling, and the 0.9
// changelog entry records why: a caller with no published number to read for
// this convention is left with the tool's own release number, which says
// nothing about conformance and which a second implementation cannot answer.
const (
	ReadOK       = "ok"
	ReadFindings = "findings"
)

// ExitCodeForRead returns the process exit code a structural-read report
// carries: 0 for ReadOK and 5 for ReadFindings. A token neither of those
// names exits 1, on the same terms ExitCode reserves 1 for an outcome the
// profile does not declare.
//
// The table is deliberately kept apart from ExitCode's own, so that 2 out of
// dinah check means one thing and one thing only, which is that the
// invocation itself was refused. Both commands used to force 2 by hand for a
// read that completed and found something to report, and a caller reading $?
// could not tell a bad --workbench from a workbench carrying defects
// (dinah-346). 5 rather than 1 carries the new meaning because ExitCode
// reserves 1 for a response nothing conforming can produce, and handing that
// code a real, expected meaning would replace one overload with another.
func ExitCodeForRead(outcome string) int {
	switch outcome {
	case ReadOK:
		return 0
	case ReadFindings:
		return 5
	}
	return 1
}

// The nineteen refusal names section 6.1 of the profile declares. A refusal
// Dinah reports that is not one of these carries the layer prefix of
// LayerPrefix, which CORE-OUT-3 admits and DOC-LAYER-1 keeps collision-free.
const (
	UnknownCard       = "unknown-card"
	UnknownColumn     = "unknown-column"
	UnsupportedVer    = "unsupported-version"
	Held              = "held"
	NotRequester      = "not-requester"
	Blocked           = "blocked"
	NotBlocked        = "not-blocked"
	NotHolder         = "not-holder"
	AtCapacity        = "at-capacity"
	NotOperator       = "not-operator"
	NoOperator        = "no-operator"
	NoOwner           = "no-owner"
	NoReason          = "no-reason"
	Terminal          = "terminal"
	Malformed         = "malformed"
	LayerCollisionErr = "layer-collision"
	UnresolvedItem    = "unresolved-item"
	// UndeclaredField is a write naming a field key the workbench does not
	// declare, and a write naming a declared key whose declaration does not
	// reach the kind the reference resolved to. One name covers both because
	// to a reader they are the same mistake, which is the reasoning
	// UnknownField's own comment already records for its two cases.
	//
	// It is spelled apart from UnknownField, which means a query or a write
	// naming a field the tool does not have. The two mistakes have different
	// repairs: the first is fixed by typing a different word, the second by
	// declaring the key.
	UndeclaredField = "undeclared-field"
	// MissingField is a move or a pull into a column whose require_fields
	// declaration names a key the card holds no value for.
	MissingField = "missing-field"
)

// Declared lists the profile's nineteen refusal names in the order section 6.1
// prints them.
var Declared = []string{
	UnknownCard, UnknownColumn, UnsupportedVer, Held, NotRequester,
	Blocked, NotBlocked, NotHolder, AtCapacity, NotOperator,
	NoOperator, NoOwner, NoReason, Terminal, Malformed, LayerCollisionErr,
	UnresolvedItem, UndeclaredField, MissingField,
}

// LayerPrefix is the prefix every refusal name Dinah introduces carries. The
// full stop is what CORE-OUT-3 admits and what DOC-LAYER-1 keeps out of the
// profile's own vocabulary, so a name minted here can never collide with one
// a later profile revision declares.
const LayerPrefix = "dinah."

// The refusal names Dinah introduces for the commands the profile does not
// specify. Each carries LayerPrefix.
const (
	Unconfirmed  = LayerPrefix + "unconfirmed"
	Interrupted  = LayerPrefix + "interrupted"
	UnknownGuide = LayerPrefix + "unknown-guide"
	// UnknownShell is dinah completion asked for a shell it prints no script
	// for, or asked for none. The detail carries the word typed, empty when
	// none was.
	UnknownShell = LayerPrefix + "unknown-shell"
	UnknownKey   = LayerPrefix + "unknown-key"
	Occupied     = LayerPrefix + "occupied"
	Locked       = LayerPrefix + "locked"
	Exists       = LayerPrefix + "exists"
	// DirectoryNotEmpty is init refusing to write a .dinah into a directory
	// that already existed and already held something, unless the caller
	// passed --here. It is a distinct name from Exists, which fires when the
	// directory already carries a recognised Dinah workbench.md of its own;
	// this one fires on an ordinary populated directory that carries no
	// workbench at all, the shape an existing, unrelated project takes.
	DirectoryNotEmpty = LayerPrefix + "directory-not-empty"
	UnknownPath       = LayerPrefix + "unknown-path"
	NoEditor          = LayerPrefix + "no-editor"
	NoWorkbench       = LayerPrefix + "no-workbench"
	UnknownVerb       = LayerPrefix + "unknown-command"
	Usage             = LayerPrefix + "usage"
	InvalidAlias      = LayerPrefix + "invalid-alias"
	AliasShadow       = LayerPrefix + "alias-shadows-command"
	AliasMissing      = LayerPrefix + "missing-alias-argument"

	// NoWorkbenchFound is the walk coming up empty, which NoWorkbench once
	// shared a sentence with. The two are separated because one template
	// cannot honestly describe both a path the caller named and a search
	// that reached the root of the filesystem.
	NoWorkbenchFound = LayerPrefix + "no-workbench-found"
	// AmbiguousWorkbench is a base directory holding several workbenches
	// with nothing closer to choose between them. The tool refuses to
	// guess, so it names the candidates instead.
	AmbiguousWorkbench = LayerPrefix + "ambiguous-workbench"
	// LastColumn is archiving or deleting the one column a workbench has
	// left, which CORE-BENCH-2 forbids the workbench from ending up with
	// none of.
	LastColumn = LayerPrefix + "last-column"
	// UnreadableBench is a workbench.md the discovery walk found and could
	// not read. The walk stops there rather than climbing past it or
	// reporting it as absent, because a file it could not open might be the
	// real workbench.
	UnreadableBench = LayerPrefix + "unreadable-workbench"
	// DamagedBench is a workbench.md sitting at the one address the
	// containment rule gives a workbench, immediately inside a .dinah
	// container under a name that container's own listing already admitted,
	// whose content readAnchor cannot recognise as a Dinah workbench's claim
	// to its directory. Per dinah-285 every such name is Dinah's to mint, so
	// this can never be somebody else's document the way a stray
	// unrecognised file elsewhere in the tree can be: it is this workbench's
	// own anchor damaged past the point the content test reads it. soleBench
	// raises it once it has read every entry in the base and none of them,
	// healthy or damaged, produced a better answer, so the walk refuses over
	// it exactly where UnreadableBench already refuses over a file it could
	// not open, rather than climbing past it and answering as though the
	// workbench it names were not there at all.
	DamagedBench = LayerPrefix + "damaged-workbench"
	// UnreadableContainer is a .dinah container the walk found and could not
	// list. Something exists at that path and os.ReadDir would not read it,
	// which the walk refuses over rather than reporting the container as
	// holding nothing, because an ambiguity answer built on a directory
	// nobody managed to read is not an answer. It is a separate name from
	// UnreadableBench because UnreadableBench's next step opens by telling
	// the reader to fix a file's permissions, and permissions are not at
	// fault for a container replaced by a plain file. Routing past the
	// container does help, since DiscoverSource returns on a --workbench or
	// DINAH_WORKBENCH override before the walk opens anything, so this
	// refusal's next step offers that route ahead of the repair. A container
	// that plainly does not exist is not this refusal at all;
	// ListWorkbenchIDs answers that case with an empty list and no error,
	// since a directory this format has not created yet is an ordinary shape
	// rather than a defect.
	UnreadableContainer = LayerPrefix + "unreadable-container"
	// NoConfiguredWorkbench is the workbench setting naming a path that no
	// longer carries a workbench.md, consulted only once the search has
	// found nothing local to answer with. It is a distinct name from
	// NoWorkbench, which the override branch already carries for the same
	// underlying condition, because the two need different sentences: one
	// names what the caller just typed, the other names what was stored
	// earlier and may have gone stale with nobody around to notice.
	NoConfiguredWorkbench = LayerPrefix + "no-configured-workbench"
	// WorkbenchBoundary is the ancestor walk stopping at the nearest git
	// repository root rather than climbing past it, having found no workbench
	// at or below that root. It is a distinct name from NoWorkbenchFound
	// because that refusal's sentence says the user base was tried, and this
	// search deliberately never reaches it: falling back to the user base from
	// inside a bounded repository is the exact hazard this refusal exists to
	// stop, so a shared sentence would misreport what the search actually did.
	WorkbenchBoundary = LayerPrefix + "workbench-boundary"
	// WorkbenchNotApplicable is --workbench or DINAH_WORKBENCH given to
	// init. Every other verb reads the flag as the path to a workbench that
	// already exists; init has none yet at the path it is about to create,
	// so the flag names nothing init can act on.
	WorkbenchNotApplicable = LayerPrefix + "workbench-not-applicable"
	// RepairWouldEmptyColumns is dinah check --migrate-columns declining to
	// remove every remaining stranded column, which CORE-BENCH-2 forbids
	// leaving the workbench definition with none of.
	RepairWouldEmptyColumns = LayerPrefix + "repair-would-empty-columns"
	// NeedsVocabularyMigration is the version gate refusing a revision the
	// retired-name alias resolved and the floor still rejects. It is
	// distinct from UnsupportedVer because this build knows what the
	// declared spelling means and can name the migration that carries the
	// workbench forward, where UnsupportedVer reports a revision this build
	// has no reading for at all. No shipped build raises it yet, since the
	// floor sits below the alias's own output.
	NeedsVocabularyMigration = LayerPrefix + "needs-vocabulary-migration"
	// NeedsContainerMigration is Open refusing a workbench that declares the
	// storage format the containment rule arrived at and does not sit where
	// that rule puts a workbench: as the immediately-named child of a .dinah
	// container, under an identifier Dinah minted. It is distinct from
	// Malformed because nothing inside the workbench is wrong, and the repair
	// is a move rather than an edit: `dinah check --migrate-container` carries
	// it into a container and names it there.
	NeedsContainerMigration = LayerPrefix + "needs-container-migration"
	// NeedsNumberMigration is Add refusing to file a card into a workbench
	// that still carries its card numbers in frontmatter, because the
	// registry a filing allocates from is the half of the format that
	// workbench has not reached. It is distinct from Malformed because
	// nothing inside the workbench is wrong, and the repair is a migration
	// rather than an edit: `dinah check --migrate-numbers --yes` builds the
	// registry, strips the number keys and stamps the format the registry
	// arrived at.
	NeedsNumberMigration = LayerPrefix + "needs-number-migration"
	// VocabularyMixed is a header carrying a key from each of the two
	// vocabularies this format has had, which no writer produces and which
	// Dinah refuses rather than guessing its way through. Two shapes reach
	// it: a workbench anchor carrying both sequence keys, and a card carrying
	// the current column key beside the retired substate key. Each of them is
	// one file holding half of each vocabulary, which is what the refusal's
	// sentence describes and what its next step tells the reader to undo.
	VocabularyMixed = LayerPrefix + "vocabulary-mixed"
	// VocabularyRetired is a card still written in the vocabulary this format
	// retired, inside a workbench whose anchor declares the current one. The
	// file itself is consistent, so it is not the mixture above; what
	// disagrees is the card and the workbench around it. The disagreement is
	// worth its own name because a workbench carried across the rename at its
	// anchor and not in its cards passes the version gate, and the column
	// identifier sitting in the state slot would otherwise be read as the
	// card's condition. The gate reads the anchor, so only a check on the
	// card itself can see it.
	VocabularyRetired = LayerPrefix + "vocabulary-retired"
	// AddNeedsAColumn is Add declining to file a card into a workbench whose
	// columns list has no live entries left for the card to land in.
	AddNeedsAColumn = LayerPrefix + "add-needs-a-column"
	// AtLoopLimit is a regressive move out of a column whose declared
	// loop_limit this card has already reached. It is a separate name from
	// AtCapacity because the two limits are declared on opposite ends of the
	// move: capacity is a property of the destination and counts the cards
	// standing there, where this one is a property of the departure and
	// counts how often this card has left it backwards. The operator carries
	// a card past either with the same --override marker, and the cap stays
	// absolute, so the count goes on rising and the next regressive move is
	// refused again.
	AtLoopLimit = LayerPrefix + "at-loop-limit"
	// UnresolvedItemExit is a move or a pull out of a column whose declared
	// hold covers the way out, made while the card carries an item naming
	// that column and not settled. It is a separate name from
	// UnresolvedItem because that one is the profile's, fixed by CORE-GATE-3
	// for a card arriving at a column, and the profile says nothing about a
	// card leaving one, so borrowing the name would claim the profile covers
	// a case it does not. The operator carries a card past this one with the
	// same --override marker that already carries a card into a held or a
	// full column.
	UnresolvedItemExit = LayerPrefix + "unresolved-item-exit"
	// UnknownField is a query naming a field this tool does not have, or
	// naming one with an operator it does not take. One name covers both
	// because to a reader `Priority>=next` and `at:` are the same mistake:
	// each names a combination the tool has no reading for.
	UnknownField = LayerPrefix + "unknown-field"
	// UnknownValue is a query giving a closed-vocabulary field a value that
	// vocabulary does not hold. It is distinct from matching nothing,
	// because an empty result is also the honest answer to a query that is
	// exactly right, and a reader cannot tell a typo from a fact.
	UnknownValue = LayerPrefix + "unknown-value"
	// UnknownAxis is a group-by chain naming a word this tool does not
	// group on. It is a distinct name from UnknownField because column is
	// both a field and an axis and at is a field and not an axis, so one
	// name covering both would tell a reader that at is not a field, which
	// is false.
	UnknownAxis = LayerPrefix + "unknown-axis"
	// RepeatedAxis is a group-by chain naming one axis twice. Grouping twice
	// on one axis puts every card of a group into a single child group,
	// always, so it is a typing mistake rather than a query. It carries a
	// name of its own because the sentence that lists the legal axes would
	// name the repeated axis as not an axis and then list it as one.
	RepeatedAxis = LayerPrefix + "repeated-axis"
	// UnknownView is dinah view naming a view no layer the caller can see
	// declares. It carries every visible name, so the reader picks one.
	UnknownView = LayerPrefix + "unknown-view"
	// MalformedView is dinah view naming a view that resolved to a
	// declaration it cannot draw. A malformed view keeps its place in name
	// resolution, so this is what a malformed user view shadowing a
	// well-formed workbench view answers, rather than the workbench's view.
	MalformedView = LayerPrefix + "malformed-view"
	// ViewsUnreadable is dinah view meeting a views layer it cannot read: a
	// user config.md that exists and cannot be read, or a dinah.views value
	// that is not a mapping in either file. It refuses rather than reading
	// the layer as empty, because a user view that should have shadowed a
	// workbench view would otherwise vanish and the workbench's be drawn.
	ViewsUnreadable = LayerPrefix + "views-unreadable"
	// WatchUnavailable is dinah view --watch refused before anything is
	// drawn, because the watch cannot redraw in place on this output. The
	// value reason names why, as one of the WatchReason tokens, and a
	// too-small window adds its size while a missing capability adds the
	// terminfo name of the capability.
	WatchUnavailable = LayerPrefix + "watch-unavailable"
	// TUIUnavailable is dinah tui refused before anything is drawn, because
	// the keyboard or the window cannot carry it.
	TUIUnavailable = LayerPrefix + "tui-unavailable"
	// MalformedUrgency is a view ordered by urgency drawn on a workbench
	// whose dinah.urgency block cannot be read. It refuses rather than
	// falling back to the shipped weights, because a ranking computed on
	// weights nobody declared is the thing --explain exists to prevent. A
	// view ordered any other way draws as usual.
	MalformedUrgency = LayerPrefix + "malformed-urgency"
	// CardNotInView is dinah view naming a live card that no section of the
	// view selects, so there is no row to narrow the view to.
	CardNotInView = LayerPrefix + "card-not-in-view"
	// ViewNotRanked is dinah view asked to explain a view whose order is not
	// urgency. Such a view ranks nothing, so it has no arithmetic to show.
	ViewNotRanked = LayerPrefix + "view-not-ranked"
	// ChainTooLong is a group-by chain naming more axes than a tree nests
	// along. It carries a name of its own because it has no offending word
	// to name: every axis in the chain may be legal, and the length is the
	// whole of the mistake.
	ChainTooLong = LayerPrefix + "chain-too-long"
	// UnknownDepth is a depth level neither tree ladder declares, or one the
	// other command declares and this one does not. The sentence lists the
	// levels of the command that refused rather than the union of both.
	UnknownDepth = LayerPrefix + "unknown-depth"
	// MultipleWords is an open-tail command's free-text slot (add's title,
	// block's reason, comment's text, config set's value) getting more than
	// one unquoted word. The sentence names the word count and rebuilds the
	// command line with the free text quoted, since that is the whole cost
	// of the rule and the fix a reader needs to see.
	MultipleWords = LayerPrefix + "multiple-words"
	// EmptySearch is dinah search given no phrase to search for, either
	// because nothing was typed or because what was typed was an empty
	// quotation. It is a refusal rather than an answer: a caller who wrote no
	// phrase made a mistake, and reading the empty phrase as a request for
	// every card would hand them a whole workbench they never asked for.
	EmptySearch = LayerPrefix + "empty-search"
	// UnknownWorkstream is a reference naming no workstream of this
	// workbench, in either half of the collection. It is separate from
	// unknown-column because a workstream is not a station of the flow, and
	// a reader told about a column they never named would go looking in the
	// wrong listing.
	UnknownWorkstream = LayerPrefix + "unknown-workstream"
	// Referenced is deleting a workstream that live cards still belong to.
	// It is separate from Occupied, whose sentence says cards still occupy
	// a column, because a card belongs to a workstream and stands in a
	// column, and one sentence cannot honestly say both.
	Referenced = LayerPrefix + "referenced"
	// WorkstreamSlugTaken is an explicit slug `workstream new` was given
	// already carrying a live workstream. It is separate from the collision
	// NewWorkstream resolves for a title-derived slug, because that collision
	// is resolved silently by construction (nobody typed the slug, so nobody
	// can be told their input was rejected), while a caller who typed a slug
	// and had it silently replaced with something else would have no way to
	// notice.
	WorkstreamSlugTaken = LayerPrefix + "workstream-slug-taken"
	// ColumnSlugTaken is an explicit slug column new was given already
	// carrying a live column. A caller who typed nothing has nothing to be
	// surprised by when the tool derives one, and a caller who typed a
	// specific slug and got a different one back silently has no way to
	// notice.
	ColumnSlugTaken = LayerPrefix + "column-slug-taken"
	// ColumnRoutingDisrupted is a column new placement (--before or bare
	// append) that would change an existing column's automatic pull-routing
	// answer (carriesInto, internal/verb/pull.go) while that column still
	// carries a live card. Modeled on Occupied, which already refuses to
	// archive or delete a column cards still stand in; this is the same
	// invariant applied to a column the placement does not touch directly
	// but whose routing the placement would still change out from under it.
	ColumnRoutingDisrupted = LayerPrefix + "column-routing-disrupted"
	// UnknownRoot is a --root naming a directory the filesystem does not
	// carry at startup. The mcp command raises it before serving, and the
	// beyond check that names it carries the same wording.
	UnknownRoot = LayerPrefix + "unknown-root"
	// ConflictingScope is one invocation naming two scopes: a root to walk
	// downward from, through --root or through the positional path
	// workbenches takes, together with a single workbench through --workbench
	// or DINAH_WORKBENCH. The two answer different questions and neither
	// outranks the other, so the tool refuses rather than honouring one and
	// discarding the other silently. One name serves every command that can
	// be given both, because the mistake is the same mistake wherever it is
	// made.
	ConflictingScope = LayerPrefix + "conflicting-scope"
	// DepthWithoutRoot is --max-depth given with nothing for it to bound,
	// meaning neither --root nor the path workbenches takes. The flag bounds
	// a downward walk, and no downward walk runs, so the value would be read
	// and dropped. Saying so is better than accepting a flag that changes
	// nothing about the answer.
	DepthWithoutRoot = LayerPrefix + "depth-without-root"
	// MalformedDepth is --max-depth given a value that is not a whole number
	// of rungs, or a negative one. It is distinct from DepthWithoutRoot,
	// which is about the flag having nothing to bound, and from UnknownDepth,
	// which names a tree projection's level rather than a walk's reach.
	MalformedDepth = LayerPrefix + "malformed-depth"
	// OutsideRoot is a workbench named by an MCP caller whose path lies
	// outside the root the server was started with. The mcp command raises
	// it at startup when --workbench named the contradiction, and the call
	// dispatch raises it when the per-call workbench argument does.
	OutsideRoot = LayerPrefix + "outside-root"
	// UnknownToolProfile is a --tools flag naming a value outside the three
	// this head serves: station, operator or all. It is checked before the
	// server opens any workbench, beside UnknownRoot and OutsideRoot.
	UnknownToolProfile = LayerPrefix + "unknown-tool-profile"
	// NotLoopback is a --listen host for dinah serve outside the loopback
	// interface. The head authenticates nobody, so it binds nowhere else, and
	// the check runs before anything is bound. It is the one refusal of the
	// HTTP head that reaches a terminal.
	NotLoopback = LayerPrefix + "not-loopback"
	// ForeignHost is an HTTP request whose Host header names neither the
	// address the head is bound to nor localhost with the bound port, which
	// is how a DNS-rebinding page would reach the head under a hostile name.
	ForeignHost = LayerPrefix + "foreign-host"
	// ForeignOrigin is an unsafe HTTP request that says it came from a page
	// other than the head's own, through Sec-Fetch-Site or through Origin.
	ForeignOrigin = LayerPrefix + "foreign-origin"
	// OriginRequired is a form post, or an empty-bodied post, that carries
	// no proof it came from the head's own pages. A page on any site can
	// make a browser send either shape without asking first.
	OriginRequired = LayerPrefix + "origin-required"
	// BodyTooLarge is an HTTP request body over the head's limit.
	BodyTooLarge = LayerPrefix + "body-too-large"
	// UnknownResource is an HTTP path no route of the head matches. It is
	// not UnknownRoute, which is a write naming a workflow route the
	// workbench does not declare.
	UnknownResource = LayerPrefix + "unknown-resource"
	// MethodNotAllowed is an HTTP method the matched route does not take.
	MethodNotAllowed = LayerPrefix + "method-not-allowed"
	// NotAcceptable is an HTTP Accept header admitting no representation
	// the matched route offers.
	NotAcceptable = LayerPrefix + "not-acceptable"
	// UnsupportedMediaType is an HTTP request body whose type the matched
	// route does not take for the method as received.
	UnsupportedMediaType = LayerPrefix + "unsupported-media-type"
	// BasisRequired is an HTTP PATCH carrying no basis at all. The act
	// changes a card another writer may have changed since the caller read
	// it, so the head requires the caller to say which revision it read, or
	// to say with * that it chose not to.
	BasisRequired = LayerPrefix + "basis-required"
	// NotImplemented is an HTTP route the head reserves and cannot answer
	// yet, because the one representation it offers has no renderer.
	NotImplemented = LayerPrefix + "not-implemented"
	// NotServed is a command typed into the pages' command log that the HTTP
	// head has no route for, so the pages cannot run it. It reaches only the
	// command log, because the typed line always answers with a redirect,
	// and it maps to 501 so that a later route raising it has a status.
	NotServed = LayerPrefix + "not-served"
	// AmbiguousName is a name selector matching more than one entity of a
	// collection that declares a name field, raised before the resolver
	// guesses which one the caller meant. The detail names the selector and
	// the ordinal of every match, so the caller can pick one by ordinal and
	// retry, since ordinal is tried ahead of name.
	AmbiguousName = LayerPrefix + "ambiguous-name"
	// NoLevels is a write naming an axis this workbench declares no set for.
	// It is separate from UnknownLevel because no name is acceptable there,
	// so a sentence calling the one the caller typed unknown would be false,
	// and the repair is to declare the set rather than to correct a name.
	// The sentence is written about the one axis the write named and never
	// about the workbench as a whole, since the two axes are declared
	// independently and a workbench declaring one of them is ordinary.
	NoLevels = LayerPrefix + "no-levels"
	// UnknownLevel is a write naming a level the workbench's declaration for
	// that axis does not carry. The sentence lists the levels it does carry,
	// and it too is written about one axis. A relative tier write raises it
	// over the column's own stored default as well, since a default naming
	// no declared member is the same defect arriving from the other side.
	UnknownLevel = LayerPrefix + "unknown-level"
	// InapplicableField is a write of a non-empty value to a level axis or a
	// declared field whose applies_when condition does not admit the card.
	// It is separate from UndeclaredField because the slot is declared, and
	// from Malformed because the value may be a perfectly good one: what is
	// wrong is that the question is not asked of this card. The refusal
	// runs after the value checks, so a malformed value is still refused for
	// being malformed, and a clearing write never meets it.
	InapplicableField = LayerPrefix + "inapplicable-field"
	// NoTierDefault is a relative tier write, +N or -N, against a column
	// carrying no tier default of its own. It is separate from NoLevels
	// because the workbench's set is declared and the column is what says
	// nothing, and separate from UnknownLevel because there is no value to
	// call unknown. The repair is to write the tier absolutely or to give
	// the column a default.
	NoTierDefault = LayerPrefix + "no-tier-default"
	// TierOutOfRange is a relative tier write whose rung count lands off
	// either end of the declared set. Nothing clamps it, because a caller
	// asking for a rung above the top has said something the workbench
	// cannot honour and silently writing the top would tell them nothing.
	TierOutOfRange = LayerPrefix + "tier-out-of-range"
	// BelowTier is a claim declaring a tier below what the card requires at
	// the column being claimed into. The requirement comes from the card
	// alone, its own baseline or a per-column override, and never from the
	// column's tier default, which informs and refuses nothing. The gate is
	// a floor rather than a match: declaring more than the card asks for is
	// waste rather than an error, and only declaring less is refused.
	BelowTier = LayerPrefix + "below-tier"
	// UnlistedModel is a claim by a caller whose declared provider and model
	// the workbench's tier table lists under no tier, on a card that requires
	// a tier at the column being claimed into. It is separate from BelowTier
	// because the two lead to two different repairs: a model the table lists
	// below the requirement is a model to switch away from, and a model the
	// table lists nowhere is a model to add to the table or to switch away
	// from, and the reader cannot tell which without being told.
	UnlistedModel = LayerPrefix + "unlisted-model"
	// UndeclaredModel is a claim by a caller that declared no model at all,
	// on a card that requires a tier at the column being claimed into. It is
	// separate from UnlistedModel because its reader is a harness that has
	// not been configured rather than one running the wrong model, so the
	// sentence names the variable to set and the parameter to send.
	UndeclaredModel = LayerPrefix + "undeclared-model"
	// MalformedHarness is an act that writes a journal line under a declared
	// harness name outside the one-segment grammar bench.HarnessName admits.
	// It refuses only such an act: a read is never refused over it, because a
	// mistyped variable that stopped show and ls would take the whole tool
	// away from whoever has to repair it, and a read stamps nothing a
	// malformed name could damage.
	MalformedHarness = LayerPrefix + "malformed-harness"
	// MalformedMemberName is a definition document carrying an object member
	// whose name carries a line ending. A member name becomes the left-hand
	// side of a frontmatter line, which nothing quotes, so such a name would
	// split the header and store a key the document never carried. The detail
	// names the member with its line ending written as an escape, since
	// printing the name raw would split the refusal's own line too.
	//
	// It sits beside MalformedHarness because both name a caller-declared name
	// outside the grammar its destination can carry, and it is the one place in
	// the line-ending contract where the answer is a refusal rather than a
	// normalisation: normalising would not help, because the line feed a CRLF
	// becomes still splits the header.
	MalformedMemberName = LayerPrefix + "malformed-member-name"
	// TierNotHigher is a raise whose resolved tier does not rank above what
	// the card already requires at the column being raised. Equal counts as
	// not higher: a raise that changes nothing is not a raise. The comparison
	// is against the card's own current requirement (the baseline or the
	// override for that column), never against the column's own tier default,
	// because a relative expression resolves against that default and the two
	// can differ, so a card already overridden above the column's default can
	// see "+1" resolve to a value at or below what it already requires. When
	// the card's current requirement names a tier the workbench no longer
	// declares, this refusal is never raised: there is nothing to compare
	// against, and TierRank's own second return value says so.
	TierNotHigher = LayerPrefix + "tier-not-higher"
	// NotRenamable is a rename aimed at something that is not an attachment.
	// The detail names what the reference resolved to, so the caller sees
	// what was misunderstood rather than what they tried to write.
	NotRenamable = LayerPrefix + "not-renamable"
	// NotAttachable is an attach aimed at a reference that resolves to a kind
	// the containment table gives no attachments collection. The detail names
	// the reference and the kind rides beside it, so the caller sees what the
	// reference reached rather than what they hoped it would reach.
	NotAttachable = LayerPrefix + "not-attachable"
	// NotCommentable is a comment aimed at a reference that resolves to a
	// kind the containment table gives no comments collection. The detail
	// names the reference and the kind rides beside it, on the terms
	// NotAttachable already carries both, so the caller sees what the
	// reference reached rather than what they hoped it would reach.
	NotCommentable = LayerPrefix + "not-commentable"
	// NotArchived is raised when the archive mirror holds nothing the
	// reference names and the reader can nevertheless see the thing they
	// typed, either because it is live or because it travelled inside an
	// archived holder. It is what an --archived read and every restore ask
	// for. The detail names the reference as typed; holder, slug and
	// collection each ride beside it on the one case that fills them.
	NotArchived = LayerPrefix + "not-archived"
	// IsACollection is a command that takes one entity handed a reference
	// naming a whole collection. The detail names the reference as typed,
	// the member count rides beside it, and so does the reference of the
	// first member, which is a spelling the reader can type.
	IsACollection = LayerPrefix + "is-a-collection"
	// AmbiguousCard is a card reference whose number more than one card of the
	// half being read carries, raised before the resolution picks one of them.
	// The candidates ride as a carried set of identifiers, because an
	// identifier is the address every card always answers to and is what the
	// caller retypes to get past this.
	AmbiguousCard = LayerPrefix + "ambiguous-card"
	// AmbiguousColumn is a pull with no destination named finding more than
	// one column it could pull into. The sentence names the columns that
	// qualified, because a reader whose command stopped needs to know what
	// to type instead.
	AmbiguousColumn = LayerPrefix + "ambiguous-column"
	// NoUpstream is a pull naming a column that stands first in the flow, so
	// nothing precedes it for a card to come from. It is a fact about the
	// flow rather than about what is on the workbench today.
	NoUpstream = LayerPrefix + "no-upstream"
	// AwaitingOutside is an act that would take work up at a column whose
	// definition says the workbench waits there on somebody who is not an
	// owner of it. One name covers all four raise sites, the claim, the
	// named pull in either direction and the move that arrives holding the
	// card, because a caller cannot act differently on four names for one
	// fact and the sentence carries the fix.
	AwaitingOutside = LayerPrefix + "awaiting-outside"
	// TakesNoWork is an act that would take work up at a column where no
	// owner does, when the column does not declare awaiting_outside and so
	// has nobody to name. A card stands at such a column until a pull carries
	// it into the station beyond, and the sentence says so.
	TakesNoWork = LayerPrefix + "takes-no-work"
	// UnknownFormat is a --format or DINAH_FORMAT value naming no output
	// form this tool writes. The closed set the value fell outside of is the
	// two machine forms, json and compact, and the absence that selects the
	// rendering a person reads. It is a name of its own rather than
	// UnknownValue, whose own comment scopes that name to a query, where an
	// empty result and a mistyped field value have to be told apart. No
	// result set stands behind a format name, so a caller who mistyped one
	// is told so instead of being handed prose where they asked for
	// structure.
	UnknownFormat = LayerPrefix + "unknown-format"
	// ReshapeNeedsDestination is a reshape retiring a column that live cards
	// still stand in, with neither a replaces declaration on the new
	// definition nor a --map entry naming where those cards go. The tool
	// refuses rather than choosing a destination, because which station the
	// work continues at is the operator's decision and no rule in the flow
	// answers it.
	ReshapeNeedsDestination = LayerPrefix + "reshape-needs-a-destination"
	// ReshapeHeldCardInQueue is a reshape that would leave a held card
	// standing where no owner takes work up: a retirement carrying its cards
	// into such a column, or a kept column whose new kind stops taking work
	// up under a card somebody is already holding. Both are refused before
	// the write rather than reported by check afterwards.
	ReshapeHeldCardInQueue = LayerPrefix + "reshape-held-card-in-queue"
	// ReshapeMapSourceEmpty is a --map whose retired side names no live
	// column, no column of the new definition, and no card's stored column
	// either, so the entry would carry nothing. A typed identifier that
	// matches nothing reads as a refusal rather than as a silent no-op.
	ReshapeMapSourceEmpty = LayerPrefix + "reshape-map-source-empty"
	// ReshapeDestinationRetiring is a destination that resolves into the set
	// of columns this same run retires. Carrying cards there would leave them
	// standing in a column the run is about to archive, so validation refuses
	// before anything is written.
	ReshapeDestinationRetiring = LayerPrefix + "reshape-destination-retiring"
	// ReshapeDestinationAmbiguous is a destination matching the title of two
	// or more columns the new definition adds, with no declared identifier on
	// any of them to break the tie. An added column has no live identifier
	// yet, so the refusal names each candidate by its position in the new
	// definition's columns array.
	ReshapeDestinationAmbiguous = LayerPrefix + "reshape-destination-ambiguous"
	// UnknownItemKind is a checklist item filed under a kind outside the
	// three the format declares. It has UnknownLevel's shape, a write naming
	// a value the declaration does not carry, except that this set is fixed
	// by the format rather than by the workbench, so the sentence names the
	// three legal spellings instead of reading a declaration.
	UnknownItemKind = LayerPrefix + "unknown-item-kind"
	// UnknownItemState is settle asked to land an item at a state outside
	// the six the format declares: resolved, verified, failed, waived,
	// withdrawn or pending.
	// It has UnknownItemKind's shape, checked ahead of resolving the item,
	// since there is nothing else to check until the state names one of the
	// four verbs settle becomes.
	UnknownItemState = LayerPrefix + "unknown-item-state"
	// WrongItemKind is a terminal verb handed an item of a kind it cannot
	// land: resolve handed an acceptance criterion, or verify or fail handed
	// an open question or a decision. The detail names the item's own kind,
	// because the caller already knows which verb they typed.
	WrongItemKind = LayerPrefix + "wrong-item-kind"
	// NotPending is a terminal verb asked to close an item that is not
	// pending. It is NotBlocked's shape: the condition the verb requires is
	// not the condition on disk.
	NotPending = LayerPrefix + "not-pending"
	// NotResolved is a reopen asked for an item already pending, the mirror
	// of NotPending.
	NotResolved = LayerPrefix + "not-resolved"
	// NotWaivable is a waive aimed at an item standing at resolved, verified,
	// waived or withdrawn. A waiver lifts a hold, so it is legal only from a
	// state that is holding, which is pending or failed. The detail is the
	// item's current state.
	//
	// NotPending is not reused for it. That name says the item is not
	// pending, and a waiver of a failed item is exactly the call this card
	// makes legal, so the older name would refuse it.
	NotWaivable = LayerPrefix + "not-waivable"
	// AlreadyWithdrawn is a withdraw aimed at an item already withdrawn.
	// Withdrawal is legal from every other state, so this is the one source
	// state the verb refuses. The detail is the item's current state and the
	// next step names reopen.
	AlreadyWithdrawn = LayerPrefix + "already-withdrawn"
	// GrantExcludesFinding is a withdrawal of an acceptance criterion
	// standing at failed or at waived, attempted by somebody who is not the
	// operator on a card carrying a criterion-retirement grant. A grant
	// admits the retirement of a criterion nobody has found anything wrong
	// with; a finding that exists is the operator's to retire. The detail is
	// the item's current state.
	GrantExcludesFinding = LayerPrefix + "grant-excludes-finding"
	// NoGrant is a revoke naming a card that carries no criterion-retirement
	// grant. The detail is the card. It refuses rather than succeeding
	// quietly so that the operator is told nothing was standing, rather than
	// being answered as though something had been taken away.
	NoGrant = LayerPrefix + "no-grant"
	// DesignationRequired is a clear of the resolution key on an item that is
	// not pending. A settled item asserts that somebody decided something,
	// and an erasable record of who and why is not a record. The detail is
	// the item's state and the next step names reopen, which is the one
	// legitimate erasure and which clears the state along with the key.
	DesignationRequired = LayerPrefix + "designation-required"
	// WorkbenchInUse is the designation conversion run on a workbench where
	// some live card is claimed. The detail names the first such card and the
	// owner holding it rides as a value. The conversion is a cutover that
	// locks every older binary out of the store, so it runs at a moment when
	// nobody else is mid-card.
	WorkbenchInUse = LayerPrefix + "workbench-in-use"
	// Uncited is an acceptance criterion asked to leave pending with no
	// citation, on a workbench that declares an evidence block. It is the
	// write-time enforcement of the citation obligation the format states.
	Uncited = LayerPrefix + "uncited"
	// EvidenceSchemeRequired is an item carrying an evidence key asked to
	// leave pending by resolve, verify or fail while no citation of the item
	// names that scheme. It runs after Uncited, which asks whether any
	// citation exists, and asks whether one of them is the right one. The
	// detail is the scheme and the item's reference rides as a value, so
	// the sentence can say which item to settle with which citation.
	EvidenceSchemeRequired = LayerPrefix + "evidence-scheme-required"
	// ObservationRequired is a citation naming a scheme whose declaration
	// carries observed: required, written with no observation. Such a
	// citation is one no terminal verb could ever legally close against, and
	// catching it at write time is cheaper than meeting it once the evidence
	// has gone stale.
	ObservationRequired = LayerPrefix + "observation-required"
	// UnknownLink is an unlink naming a kind and target pair the card does
	// not carry. It carries LayerPrefix, unlike the two link events, because
	// removal is Dinah's own invention: neither the format document nor the
	// profile describes an unlink verb or names a refusal for one, so this is
	// not a declared-shape name the way unknown-card and malformed are.
	UnknownLink = LayerPrefix + "unknown-link"
	// CommentBodyDiverged is a verb asked to write a comment whose stored
	// digest disagrees with the body it is about to replace. The body was
	// edited by something other than a verb, so writing over it would erase
	// the evidence of that edit and, on an edit, would attribute somebody
	// else's words to whoever ran the command. The detail names the
	// comment. dinah accept-divergence clears it.
	CommentBodyDiverged = LayerPrefix + "comment-body-diverged"
	// NotADesignation is a terminal verb handed a reference that is not a
	// comment of the item being settled: another item's answer, a card
	// comment, or something that is not a comment at all. The detail is the
	// reference as the caller typed it, which is the spelling they can
	// compare against.
	NotADesignation = LayerPrefix + "not-a-designation"
	// NotDesignatable is a delete aimed at a comment an item designates as
	// its answer. The detail names the comment and the designating item
	// rides as a value, because an answer of record cannot be destroyed
	// while it is still the answer. Reopening the item first frees it, and
	// --force reopens it as part of the same act.
	NotDesignatable = LayerPrefix + "not-designatable"
	// StoreAwaitingMigration is a read opening a workbench whose checklist
	// items still carry the retired note key, which the dinah-525 note
	// migration carries into comments. It names the script, because a
	// refusal a user meets is named like any other even where the migration
	// behind it carries no surface of its own.
	StoreAwaitingMigration = LayerPrefix + "store-awaiting-migration"
	// UnknownRoute is a write naming a route the workbench does not declare.
	// The sentence lists the routes it does declare, read off the workbench
	// rather than written into a catalog, so a route declared later reaches
	// the message with nobody editing a sentence.
	UnknownRoute = LayerPrefix + "unknown-route"
	// RouteStrandsItem is a route write on a card carrying a pending item
	// naming a column the named route does not carry. Such an item is a hold
	// that would never fire, which is the commonest way a stop somebody meant
	// to create silently fails to exist. Pending is the state that matters,
	// because a settled item holds nothing on entry and has nothing to
	// strand.
	RouteStrandsItem = LayerPrefix + "route-strands-item"
	// RouteSkipsOperatorColumn is a route write whose named route omits an
	// operator-owned column standing at or after the card's current column.
	// Declaring a route is the operator's write and placing a card on one is
	// any owner's, so without this refusal an agent could carry a card around
	// a station the workbench reserves by a road the operator drew for other
	// work, and the reservation would never engage because the card never
	// enters the column.
	RouteSkipsOperatorColumn = LayerPrefix + "route-skips-operator-column"
	// ItemOffRoute is an item filed against, or moved onto, a column the
	// card's own route does not carry. It is separate from the profile's
	// unresolved-item, which covers an item naming no declared column at all:
	// that is a misfiling with a different repair, and one name with two
	// repairs tells a caller nothing.
	ItemOffRoute = LayerPrefix + "item-off-route"
	// RouteOffColumn is a filing whose destination column the route it names
	// does not carry, which would stand the new card off its route from its
	// first moment. It refuses at creation while a route change on a live
	// card is not refused, because a new card has no history and stands
	// nowhere, so refusing costs one corrected flag.
	RouteOffColumn = LayerPrefix + "route-off-column"
	// UnknownRecipe is a dinah setup naming a recipe that no place setup
	// searches holds: the project's container, the user base, or the recipes
	// the binary ships. The detail is the name as typed.
	UnknownRecipe = LayerPrefix + "unknown-recipe"
	// MalformedRecipe is a recipe setup found and cannot use. The detail is
	// <file>: <defect>, naming the first defect in the order the recipe's
	// files are read. A place holding a broken recipe of the name is still
	// the place that answered, so setup does not fall through to the next.
	MalformedRecipe = LayerPrefix + "malformed-recipe"
	// UnknownScope is a dinah setup naming a scope the recipe does not
	// declare. The detail is the scope as typed.
	UnknownScope = LayerPrefix + "unknown-scope"
	// SetupNoTarget is a project-scope setup with no directory it may write
	// into: the workbench does not stand in a .dinah container inside a
	// project, or the base would be the home directory or one of its
	// ancestors, where a project step would write user configuration.
	SetupNoTarget = LayerPrefix + "setup-no-target"
	// SetupAgentIsOperator is a dinah setup whose agent name equals the
	// workbench's operator or the user's configured actor. An agent acting
	// under the operator's name holds the operator's authority.
	SetupAgentIsOperator = LayerPrefix + "setup-agent-is-operator"
	// SetupUnreadableTarget is a file setup would change that it cannot read:
	// a JSON file that is not one object, markers out of balance, a ledger
	// that does not parse, or a path leaving the scope's base. The detail is
	// <file>: <defect>.
	SetupUnreadableTarget = LayerPrefix + "setup-unreadable-target"
	// SetupConflict is a location setup would change that it did not write,
	// or that somebody has edited since. The detail lists every such
	// location, one per line, so one edit clears them all.
	SetupConflict = LayerPrefix + "setup-conflict"
	// UntrustedRecipe is a recipe found in a project's container used without
	// --trust-project-recipe. A project's recipe arrived with a clone, and
	// its printed text can ask whoever runs setup for anything.
	UntrustedRecipe = LayerPrefix + "untrusted-recipe"
	// SetupRelocatedHome is a user-scope setup run while DINAH_HOME names a
	// directory other than the machine's own home, which would write the
	// real home's files and record their ownership somewhere else.
	SetupRelocatedHome = LayerPrefix + "setup-relocated-home"
	// SetupOtherWorkbench is a project-scope setup of a recipe the project
	// is already set up with for a different workbench. The detail is that
	// workbench's directory.
	SetupOtherWorkbench = LayerPrefix + "setup-other-workbench"
	// SetupRunNotAllowed is a setup whose recipe runs programs, without
	// --allow-run. The detail lists each run step as <id>: <command line>.
	SetupRunNotAllowed = LayerPrefix + "setup-run-not-allowed"
	// SetupStepFailed is a program a recipe runs that could not be found or
	// exited non-zero. The data steps before it stay applied and recorded.
	SetupStepFailed = LayerPrefix + "setup-step-failed"
)

// Introduced lists every refusal name Dinah mints beyond the profile's own.
var Introduced = []string{
	Unconfirmed, UnknownGuide, UnknownShell, UnknownKey, InvalidAlias, AliasShadow, AliasMissing, Occupied, Locked, Exists,
	DirectoryNotEmpty,
	UnknownPath, NoEditor, NoWorkbench, UnknownVerb, Usage, Interrupted,
	NoWorkbenchFound, AmbiguousWorkbench, LastColumn, UnreadableBench, DamagedBench, UnreadableContainer,
	NoConfiguredWorkbench, WorkbenchBoundary,
	WorkbenchNotApplicable, RepairWouldEmptyColumns, NeedsVocabularyMigration,
	AddNeedsAColumn, NeedsNumberMigration, MultipleWords, EmptySearch,
	UnknownField, UnknownValue, UnknownAxis, RepeatedAxis, ChainTooLong,
	UnknownDepth, UnknownWorkstream, Referenced, WorkstreamSlugTaken,
	ColumnSlugTaken, ColumnRoutingDisrupted,
	UnknownRoot, OutsideRoot, UnknownToolProfile, ConflictingScope, DepthWithoutRoot, MalformedDepth,
	NotLoopback, ForeignHost, ForeignOrigin, OriginRequired, BodyTooLarge, UnknownResource,
	MethodNotAllowed, NotAcceptable, UnsupportedMediaType, BasisRequired, NotImplemented, NotServed,
	AmbiguousName, NotRenamable, NotAttachable, NotCommentable, IsACollection, NotArchived,
	AmbiguousCard, AmbiguousColumn, NoUpstream, AwaitingOutside, TakesNoWork,
	NoLevels, UnknownLevel, UnknownFormat, InapplicableField,
	NoTierDefault, TierOutOfRange, BelowTier, TierNotHigher,
	UnlistedModel, UndeclaredModel, MalformedHarness, MalformedMemberName,
	ReshapeNeedsDestination, ReshapeHeldCardInQueue, ReshapeMapSourceEmpty,
	ReshapeDestinationRetiring, ReshapeDestinationAmbiguous,
	UnknownItemKind, UnknownItemState, WrongItemKind, NotPending, NotResolved, Uncited,
	EvidenceSchemeRequired,
	NotWaivable, AlreadyWithdrawn, GrantExcludesFinding, NoGrant, DesignationRequired,
	WorkbenchInUse,
	UnresolvedItemExit,
	ObservationRequired, UnknownLink,
	CommentBodyDiverged, NotADesignation, NotDesignatable, StoreAwaitingMigration,
	UnknownRoute, RouteStrandsItem, RouteSkipsOperatorColumn, ItemOffRoute, RouteOffColumn,
	UnknownRecipe, MalformedRecipe, UnknownScope, SetupNoTarget, SetupAgentIsOperator,
	SetupUnreadableTarget, SetupConflict, UntrustedRecipe, SetupRelocatedHome,
	SetupOtherWorkbench, SetupRunNotAllowed, SetupStepFailed,
	UnknownView, MalformedView, ViewsUnreadable,
	MalformedUrgency, CardNotInView, ViewNotRanked, WatchUnavailable,
	TUIUnavailable,
}

// The reasons dinah.watch-unavailable carries in its reason value, which are
// machine tokens and never translated.
const (
	// WatchNotATerminal is standard output that is not a terminal.
	WatchNotATerminal = "not-a-terminal"
	// WatchNoSize is a terminal that does not report its size.
	WatchNoSize = "no-size"
	// WatchTooSmall is a window narrower than 40 columns or shorter than 6
	// rows.
	WatchTooSmall = "too-small"
	// WatchNoTerminalDescription is a POSIX terminal whose terminfo entry,
	// named by TERM, could not be found or read.
	WatchNoTerminalDescription = "no-terminal-description"
	// WatchMissingCapability is a terminfo entry lacking a capability the
	// watch needs, or using a parameter operation the reader does not
	// evaluate in one.
	WatchMissingCapability = "missing-capability"
)

// The reasons dinah.tui-unavailable carries in its reason value that the
// watch's reasons do not already name. The command reuses WatchNotATerminal,
// WatchNoSize and WatchTooSmall where the meaning is the same, and these are
// machine tokens that are never translated either.
const (
	// TUIDumbTerminal is a POSIX terminal whose TERM is dumb or unset.
	TUIDumbTerminal = "dumb-terminal"
	// TUINoKeyDescription is a POSIX terminal whose terminfo entry could not
	// be read, or lacks a key capability the key reader needs.
	TUINoKeyDescription = "no-key-description"
)

// NameIsLegal reports whether a refusal name is one CORE-OUT-3 admits: one
// the profile declares, or one carrying a full stop.
func NameIsLegal(name string) bool {
	if strings.Contains(name, ".") {
		return true
	}
	for _, d := range Declared {
		if d == name {
			return true
		}
	}
	return false
}

// The three states of a card. The set is closed, because the tool enforces
// the meaning of each member.
const (
	StateReady   = "ready"
	StateActive  = "active"
	StateBlocked = "blocked"
)

// The three column kinds the profile declares.
const (
	KindIntake = "intake"
	KindWork   = "work"
	KindDone   = "done"
)

// KindBuffer is a column where no owner takes work up and a pull carries a
// card through into the station beyond. It carries LayerPrefix because
// CORE-STATE-11 admits a kind of a layer's minting and no other.
const KindBuffer = LayerPrefix + "buffer"

// MintedKinds lists every column kind Dinah introduces beyond the three the
// profile declares. It sits beside Introduced, which does the same for the
// refusal names, because the layer prefix carries both and a reader meeting a
// dotted token in a document needs one place to ask what it is.
var MintedKinds = []string{KindBuffer}

// ViewsKey is the frontmatter key Dinah's views layer is declared under, in a
// workbench's workbench.md and in the user's config.md alike. It carries the
// layer prefix because a layer's own keys must, which keeps it clear of every
// key the profile declares.
const ViewsKey = LayerPrefix + "views"

// UrgencyKey is the frontmatter key a workbench declares the weights of the
// urgency order under. It is read from the workbench's own workbench.md
// alone, because the weights are declared per workbench.
const UrgencyKey = LayerPrefix + "urgency"

// ScheduleKey is the frontmatter key a workbench declares its schedule
// settings under: the time zone whose calendar today is read in, and how far
// ahead soon looks. It is read from the workbench's own workbench.md alone,
// because today is a fact about the workbench rather than about whoever is
// asking.
const ScheduleKey = LayerPrefix + "schedule"

// HoldsKey is the frontmatter key a workbench declares its commitment column
// and its holding link kinds under: which kinds hold one end of a link back
// from selection until the other end starts or finishes, and the column at
// which work counts as started. It is read from the workbench's own
// workbench.md alone, because a link kind's grammar is the workbench's.
const HoldsKey = LayerPrefix + "holds"

// MintedKeys lists every frontmatter key Dinah introduces under the layer
// prefix. It sits beside Introduced and MintedKinds for the reason MintedKinds
// gives: the prefix carries all three, and a reader meeting a dotted token in
// a document needs one place to ask what it is.
var MintedKeys = []string{ViewsKey, UrgencyKey, ScheduleKey, HoldsKey}

// The six schedule conditions a card can hold, computed on every read from
// its three scheduling dates, its position, the links the workbench declares
// under dinah.holds and today, and never stored.
const (
	// ScheduleOverdue holds where the card's due date is before today.
	ScheduleOverdue = "overdue"
	// ScheduleLateStart holds where the card's start_by date is before
	// today and the card has not started.
	ScheduleLateStart = "late_start"
	// ScheduleDueSoon holds where the card's due date falls from today to
	// the end of the workbench's soon window.
	ScheduleDueSoon = "due_soon"
	// ScheduleStartSoon holds where the card's start_by date falls from
	// today to the end of the soon window and the card has not started.
	ScheduleStartSoon = "start_soon"
	// ScheduleNotYet holds where the card's start_after date is after
	// today, which selection reads beside ScheduleWaiting.
	ScheduleNotYet = "not_yet"
	// ScheduleWaiting holds where a link the workbench declares under
	// dinah.holds holds the card back today, because the card it waits on
	// has not started or finished, or did so too few days ago.
	ScheduleWaiting = "waiting"
)

// ScheduleConditions is the closed set, in precedence order, highest first.
var ScheduleConditions = []string{ScheduleOverdue, ScheduleLateStart, ScheduleDueSoon, ScheduleStartSoon, ScheduleNotYet, ScheduleWaiting}

// The ends of a holding link and the events it waits for, as the dinah.holds
// layer writes them.
const (
	HoldHeldNamed   = "named"
	HoldHeldCarrier = "carrier"
	HoldWaitsStart  = "start"
	HoldWaitsFinish = "finish"
)

// Kinds lists every column kind this build admits by name: the three the
// profile declares and the one Dinah mints. A surface offering a caller the
// choice reads this rather than writing the four out again, so the set a
// command accepts cannot drift from the set the tool understands.
var Kinds = []string{KindIntake, KindWork, KindDone, KindBuffer}

// The journal event names. The set is closed, and an extension kind's own
// events carry a dotted name of their own rather than joining this list.
const (
	EventCreated            = "created"
	EventClaimed            = "claimed"
	EventMoved              = "moved"
	EventReleased           = "released"
	EventBlocked            = "blocked"
	EventUnblocked          = "unblocked"
	EventExpired            = "expired"
	EventCommented          = "commented"
	EventAttached           = "attached"
	EventAttachmentReplaced = "attachment_replaced"
	EventAttachmentRemoved  = "attachment_removed"
	// EventAttachmentRenamed carries the attachment's identifier in
	// Attachment, the new filename in Filename, and the previous filename
	// in From. The same shape as the three sibling attachment events uses,
	// since the name the attachment has as of the line is the answer a
	// reader of the journal wants by one rule across the family.
	EventAttachmentRenamed = "attachment_renamed"
	EventArchived          = "archived"
	EventRestored          = "restored"
	EventDeleted           = "deleted"
	EventManualCorrection  = "manual_correction"
	// EventWorkbenchUpdated records a write to one of the workbench's own
	// fields, on the workbench journal. It covers a title change, a slug
	// change and an operator change alike, which is why it is not named for a
	// rename: an operator change is no rename, and the name lands in
	// append-only history in every workbench that runs the command.
	EventWorkbenchUpdated = "workbench_updated"
	// EventWorkstreamUpdated records a write to one of a workstream's own
	// fields, on that workstream's journal. It covers a title change, a slug
	// change and a status change alike, and it carries Field, From and To
	// exactly as EventWorkbenchUpdated does.
	EventWorkstreamUpdated = "workstream_updated"
	// EventColumnUpdated records a rewrite of a column's own anchor, on the
	// workbench journal, and carries the column's identifier in Note. It
	// sits beside EventWorkbenchUpdated on the same terms: a column is a
	// workbench-level entity carrying no journal of its own, so the record of
	// a write to it belongs to the workbench that holds it.
	//
	// Two commands write it, and the two lines differ. Reshape writes one
	// line per kept column whose rendered anchor the new definition actually
	// changed, carrying Note alone, so a run that repeats a column unchanged
	// leaves no line behind. A write to a column's own field carries Field
	// beside the Note, and From and To with it wherever the field is not the
	// column's prose body.
	EventColumnUpdated = "column_updated"
	// EventCommentUpdated records a write to a comment's own field, on the
	// journal of the entity the comment hangs below, which is the card for a
	// card comment and for an item comment and the workbench for a comment
	// left on a column, and carries the comment's identifier in Note beside
	// the Field. It is the comment's own event
	// rather than EventCardUpdated so that a query for the card's own field
	// changing stays a question a reader can ask.
	EventCommentUpdated = "comment_updated"
	// EventDivergenceAccepted records that an operator ratified a comment
	// body somebody edited outside the tool, on the journal of the entity
	// the comment hangs below, and carries the comment's identifier in Note.
	// It is its own event rather than a comment_updated because the two acts
	// mean different things: ratifying says the body somebody typed is now
	// the record, and writing says here is the record instead.
	EventDivergenceAccepted = "divergence_accepted"
	// EventItemUpdated records a write to a checklist item's own field, on
	// the journal of the card the item hangs below, and carries the item's
	// identifier in Note beside the Field. It covers the fields a terminal
	// verb does not land: a state change is EventItemResolved and its three
	// siblings, which say which act closed the item.
	EventItemUpdated = "item_updated"
	// EventAttachmentUpdated records a write to an attachment's own field, on
	// the journal of the nearest enclosing journal-bearing entity, which is
	// the card below which the attachment hangs and the workbench for an
	// attachment hanging below a column or below the workbench itself. It
	// carries the attachment's identifier in Note beside the Field. A
	// filename change is EventAttachmentRenamed rather than this name,
	// because the payload moves with the name and the family of attachment
	// events already says so.
	EventAttachmentUpdated = "attachment_updated"
	// EventWorkstreamJoined and EventWorkstreamLeft record a card entering
	// and leaving a workstream, on the card's own journal, because membership
	// is card-owned and the card is the file that changed. Each carries the
	// workstream's identifier in Workstream.
	EventWorkstreamJoined = "workstream_joined"
	EventWorkstreamLeft   = "workstream_left"
	// EventCardUpdated records a write to one of a card's own fields, on that
	// card's journal. It covers a severity change and a priority change
	// alike, and it carries Field, From and To exactly as
	// EventWorkbenchUpdated does. Clearing a field writes the line with To
	// absent, and a first write to a field carrying none writes it with From
	// absent, since omitempty drops an empty value either way.
	EventCardUpdated = "card_updated"
	// EventTierOverridden records a write to one of a card's per-column tier
	// overrides, on that card's journal. It carries the resolved column's
	// identifier in Column, the override's previous and new absolute values
	// in From and To, exactly what the caller typed in Expr, and the column's
	// own tier default in Against, which is what a relative expression was
	// measured from and is absent where the expression was absolute and
	// needed no baseline. The provenance rides here rather than on the anchor
	// because the anchor stores the resolved value alone.
	EventTierOverridden = "tier_overridden"
	// EventTierOverrideDropped records a per-column tier override removed
	// because reshape retired the column it was written for, on that card's
	// journal. It carries the retired column's identifier in Column and the
	// dropped absolute value in From. Nothing carries it forward to the
	// destination, since a tier chosen for one station is not evidence about
	// a different one.
	EventTierOverrideDropped = "tier_override_dropped"
	// The six checklist item lifecycle events, on the card's own journal by
	// the nearest-enclosing rule. Each carries the item's identifier in Item,
	// and the item's text stays in the item's anchor rather than travelling
	// on the line, exactly as a comment's text stays in the comment.
	//
	// They join this block rather than carrying LayerPrefix because the
	// prefix is reserved for what Dinah invents beyond what the format
	// declares, and a checklist item is declared. EventItemFiled carries the
	// kind in Kind. EventItemCited carries the citation's scheme and target
	// in Scheme and Target. The three terminal events and EventItemReopened
	// carry the state the item left in From, and a reopen carries its reason
	// in Reason.
	EventItemFiled    = "item_filed"
	EventItemCited    = "item_cited"
	EventItemResolved = "item_resolved"
	EventItemVerified = "item_verified"
	EventItemFailed   = "item_failed"
	EventItemReopened = "item_reopened"
	// EventItemWaived and EventItemWithdrawn are the two settling events the
	// waive and withdraw verbs write. Each carries the state the item left in
	// From and the state it reached in To, on the three terminal events' own
	// terms.
	//
	// Neither verb reuses EventItemResolved. A journal saying an item was
	// resolved when somebody waived it says the question was answered and the
	// work held, and a reader of the card's history has no other source for
	// what happened.
	//
	// EventItemWithdrawn carries the boolean marker Grant where the act was
	// admitted because a criterion-retirement grant stood, which is exactly
	// when the actor was not the operator. A reader that does not know the
	// marker reads an ordinary withdrawal, which is what the line already is.
	EventItemWaived    = "item_waived"
	EventItemWithdrawn = "item_withdrawn"
	// EventRetirementGranted and EventRetirementRevoked record a
	// criterion-retirement grant given to one card and taken back from it.
	// The grant carries in To the identifier of the column the card stood in
	// when it was given, which is the station the grant is bound to and which
	// the card's next move spends.
	//
	// A grant spent by a move writes no line of its own. The moved event
	// already records the departure that spent it, and a second line saying
	// the same thing in other words is a fact a reader has to reconcile
	// rather than one they gain.
	EventRetirementGranted = "retirement_granted"
	EventRetirementRevoked = "retirement_revoked"
	// EventDesignationsMigrated records a run of the designation conversion
	// that passed a claimed card, on the workbench's own journal. It carries
	// the identifier of every card whose claim the run passed, so the
	// judgement the operator made about which claims were dead is nameable
	// afterwards. A run that passed no claim writes it too, carrying none, so
	// the flag is never a silent no-op.
	//
	// It lands on the workbench's journal rather than on any card's, so it is
	// held out of Events for the reason that list already gives for the three
	// *_updated names it holds out.
	EventDesignationsMigrated = "designations_migrated"
	// EventLinked and EventUnlinked record a link written onto a card and
	// removed from it, on that card's own journal, because a link is
	// card-owned and the card carrying it is the only file that changes.
	// Each carries the link's kind in Kind and the resolved identifier of
	// the card the link names in To.
	//
	// They join this block rather than carrying LayerPrefix for the reason
	// the checklist events give: the prefix is reserved for what Dinah
	// invents beyond what the format declares, and a link is declared. The
	// removal refusal is not, which is why UnknownLink does carry the prefix
	// and these two do not.
	EventLinked   = "linked"
	EventUnlinked = "unlinked"
	// EventRenumbered records a change to the creation ordinal a card answers
	// to, on that card's own journal, so the operator whose card is called
	// something else this morning can read why. Two acts write it: the number
	// migration, whose tie-break moves a card so two cards of one workbench
	// cannot share a number, and the renumber repair, which moves the later
	// claimant of a number two cards hold. It carries the number the card
	// answered to before in From and the number it answers to now in To.
	//
	// It joins this block rather than carrying LayerPrefix for the reason the
	// checklist events give: the creation ordinal is a declared concept and
	// this is its lifecycle event, where the prefix is reserved for what
	// Dinah invents beyond what the format declares.
	EventRenumbered = "renumbered"
)

// Events lists the event names a query over cards accepts in its event field,
// in the order the constants above declare them, so a caller
// checking a value against the closed set reads one list rather than repeating
// it. Every event a card's own journal can carry has to be here, since an
// event a card carries and this list omits is an event nobody can ask for. The
// containment does not hold the other way. EventRestored is listed and no
// command writes it, so a query naming it is accepted and selects nothing.
//
// EventWorkbenchUpdated, EventWorkstreamUpdated, EventColumnUpdated and
// EventDesignationsMigrated are the four declared names this list holds out,
// and each is held out for the same reason: it lands on the workbench's
// journal or on a workstream's, never on a card's, so no card a query reads
// can ever carry it. EventCardUpdated is the
// fourth of the *_updated family and is listed, because it lands on a card's
// own journal and an event a card carries that nobody can ask for is exactly
// what the containment above forbids.
var Events = []string{
	EventCreated, EventClaimed, EventMoved, EventReleased, EventBlocked,
	EventUnblocked, EventExpired, EventCommented, EventAttached,
	EventAttachmentReplaced, EventAttachmentRemoved, EventAttachmentRenamed,
	EventArchived, EventRestored, EventDeleted, EventManualCorrection,
	EventCommentUpdated, EventDivergenceAccepted, EventItemUpdated, EventAttachmentUpdated,
	EventWorkstreamJoined, EventWorkstreamLeft, EventCardUpdated,
	EventTierOverridden, EventTierOverrideDropped,
	EventItemFiled, EventItemCited, EventItemResolved, EventItemVerified,
	EventItemFailed, EventItemReopened, EventItemWaived, EventItemWithdrawn,
	EventRetirementGranted, EventRetirementRevoked,
	EventLinked, EventUnlinked,
	EventRenumbered,
}

// Refusal is the error a verb returns when a rule says no. It carries the one
// refusal name CORE-OUT-2 requires and a detail the head renders for a person.
type Refusal struct {
	// Name is the refusal name, from the profile's nineteen or dotted.
	Name string
	// Detail names what the refusal was about: the column asked for, the
	// owner holding the card, the version wanted. It is not a sentence and
	// never reaches a reader untranslated.
	Detail string
	// Extra carries named values a catalog fragment may reference beyond
	// Detail: the file a malformed field belongs to, the base directory an
	// ambiguous search looked in. It is nil for every refusal that needs
	// none, which is every refusal Refuse builds.
	Extra map[string]string
}

// Error renders the refusal for a Go caller. The name leads, because a caller
// reading the string rather than the type still reads the name first.
func (r *Refusal) Error() string {
	if r.Detail == "" {
		return r.Name
	}
	return fmt.Sprintf("%s: %s", r.Name, r.Detail)
}

// Refuse returns a refusal carrying a name and an optional detail.
func Refuse(name, detail string) *Refusal {
	return &Refusal{Name: name, Detail: detail}
}

// RefuseWith returns a refusal carrying named values beyond its detail, for
// the refusals whose sentence tells the reader where the tool looked.
func RefuseWith(name, detail string, extra map[string]string) *Refusal {
	return &Refusal{Name: name, Detail: detail, Extra: extra}
}

// ValueColumn is the column identifier a raise site fills in a refusal's
// Extra when the refusal is about one column of an opened workbench. It
// rides Extra, and so the JSON envelope's context map, exactly as
// ValueWorkbench does, so that a caller wanting the identifier reads a
// declared field rather than parsing the English a raise site composed into
// Detail or recovering the id from a path's spelling. No catalog fragment
// interpolates it, so the Malformed shape does not declare it in Values.
const ValueColumn = "column"

// ValueHarness is the harness a request declared, filled in a no-owner or a
// not-operator refusal's Extra when the request that raised it named one. It
// is what tells the harness-variant sentence apart from the plain one, and its
// presence is the whole of the condition.
const ValueHarness = "harness"

// Stale is the error a verb returns when the request's basis does not name
// the card's current revision. It carries that revision, which CORE-BASIS-4
// requires and which is what the caller reads against before retrying.
type Stale struct {
	// Current is the card's revision as it now stands.
	Current string
	// Basis is the revision the request named.
	Basis string
}

// Error renders the staleness for a Go caller.
func (s *Stale) Error() string {
	return fmt.Sprintf("stale: basis %s, current %s", s.Basis, s.Current)
}

// Unreachable is the error a verb returns when whatever answers for the
// workbench could not be reached at all, which CORE-OUT-4 keeps distinct from
// a refusal.
type Unreachable struct {
	// Detail names what could not be reached.
	Detail string
}

// Error renders the unreachability for a Go caller.
func (u *Unreachable) Error() string {
	return "unreachable: " + u.Detail
}

// With returns a copy of a refusal carrying one more named value, so a caller
// holding something the raise site below it could not know attaches it on the
// way out without that site being edited.
//
// Anything that is not a refusal comes back unchanged, and so does a refusal
// handed an empty value, which is what lets a caller wrap unconditionally: a
// dinah init that named no source attaches nothing, and the rule that no
// placeholder is ever filled with an empty string holds with no test at the
// call site. The copy is built here because contract is the one package that
// builds a refusal at all.
func With(err error, name, value string) error {
	refusal, ok := err.(*Refusal)
	if !ok || value == "" {
		return err
	}
	extra := make(map[string]string, len(refusal.Extra)+1)
	for key, carried := range refusal.Extra {
		extra[key] = carried
	}
	extra[name] = value
	return RefuseWith(refusal.Name, refusal.Detail, extra)
}

// EventNames are every event name this build declares, which is the closed set
// a journal line's event member is drawn from.
//
// It is Events plus the four an entity other than a card records: a column's
// own rewrite, a workstream's own rewrite, the workbench's own, and the
// designation conversion's account of the claims it passed. Events stayed the
// card-journal set it has always been, and a caller asking what names exist at
// all reads this.
func EventNames() []string {
	return append(append([]string(nil), Events...),
		EventColumnUpdated, EventWorkstreamUpdated, EventWorkbenchUpdated,
		EventDesignationsMigrated)
}
