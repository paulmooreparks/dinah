// Package verb is the library layer: one implementation of every verb, which
// the cli head and the mcp head both project. A head renders and parses; it
// never computes a refusal, an ordering or an instruction composition of its
// own, so the two surfaces cannot drift apart.
package verb

import (
	"strconv"

	"dinah/internal/contract"
)

// Check is one row of a verb's ordered precondition list.
type Check struct {
	// Refusal is the refusal name reported when the check is unsatisfied.
	Refusal string
	// Key is the catalog key carrying the check in the reader's language.
	// Its English text is the profile's own wording, so a per-verb help
	// listing reads as section 6 of the profile reads.
	Key string
}

// The names of the five contract verbs and of the beyond-contract commands
// the library carries beside them.
const (
	Claim   = "claim"
	Move    = "move"
	Release = "release"
	Block   = "block"
	Unblock = "unblock"
	Join    = "join"
	Leave   = "leave"
	Raise   = "raise"
)

// ContractVerbs are the five verbs the profile specifies, in the order
// section 6 states them.
var ContractVerbs = []string{Claim, Move, Release, Block, Unblock}

// Pull is the verb name reserved for the one-command route that combines a
// claim and a move. It is not in ContractVerbs because section 6.1's refusal
// table names the five, and a sixth would contradict the profile's shape.
// runsWorkbenchChecks names it all the same, so the workbench's two refusals
// still head its list.
const Pull = "pull"

// WorkbenchChecks are the two refusals that belong to the workbench rather
// than to any verb. Section 6.1 evaluates them ahead of every verb's own
// list, which is what CORE-OUT-6 makes observable.
var WorkbenchChecks = []Check{
	{Refusal: contract.UnsupportedVer, Key: "check.workbench.1"},
	{Refusal: contract.NoOperator, Key: "check.workbench.2"},
}

// checkLists are the ordered precondition lists of section 6.3 to 6.7. The
// order is the contract's, and a test holds it against the profile document
// so that a reordering there which the code does not follow fails the build.
var checkLists = map[string][]Check{
	Claim: {
		{Refusal: contract.UnknownCard, Key: "check.claim.1"},
		{Refusal: contract.NoOwner, Key: "check.claim.2"},
		{Refusal: contract.NotRequester, Key: "check.claim.3"},
		{Refusal: contract.Blocked, Key: "check.claim.4"},
		{Refusal: contract.Held, Key: "check.claim.5"},
		{Refusal: contract.NotOperator, Key: "check.claim.6"},
		{Refusal: contract.UnresolvedItem, Key: "check.claim.7"},
		// The eighth row is Dinah's own, appended rather than inserted for
		// the reason check.move.9 gives: the profile's section 6.3 list ends
		// at the seventh, and inserting among them would renumber rows the
		// profile numbers. canClaim runs it where this list prints it, after
		// the unresolved-item row.
		{Refusal: contract.BelowTier, Key: "check.claim.8"},
	},
	Move: {
		{Refusal: contract.UnknownCard, Key: "check.move.1"},
		{Refusal: contract.UnknownColumn, Key: "check.move.2"},
		{Refusal: contract.NotOperator, Key: "check.move.3"},
		{Refusal: contract.NotOperator, Key: "check.move.4"},
		{Refusal: contract.Blocked, Key: "check.move.5"},
		{Refusal: contract.Held, Key: "check.move.6"},
		{Refusal: contract.Terminal, Key: "check.move.7"},
		{Refusal: contract.AtCapacity, Key: "check.move.8"},
		// The ninth row is Dinah's own: the profile's section 6.4 list ends
		// at the eighth, so the loop limit is appended rather than inserted
		// among the eight, which would renumber rows the profile numbers. Its
		// key names where it sits in this list, and canLand runs it where
		// this list prints it, after the capacity row, so a move failing
		// both is refused at-capacity. dinah help move heads its table
		// Order and promises the rows in the order each is checked, so the
		// published numbering decides the evaluation order rather than the
		// code deciding what the page prints.
		{Refusal: contract.AtLoopLimit, Key: "check.move.9"},
	},
	Release: {
		{Refusal: contract.UnknownCard, Key: "check.release.1"},
		{Refusal: contract.NotHolder, Key: "check.release.2"},
	},
	Block: {
		{Refusal: contract.UnknownCard, Key: "check.block.1"},
		{Refusal: contract.NoOwner, Key: "check.block.2"},
		{Refusal: contract.NoReason, Key: "check.block.3"},
		{Refusal: contract.Held, Key: "check.block.4"},
	},
	Unblock: {
		{Refusal: contract.UnknownCard, Key: "check.unblock.1"},
		{Refusal: contract.NotOperator, Key: "check.unblock.2"},
		{Refusal: contract.NotBlocked, Key: "check.unblock.3"},
	},
}

// pullChecks is pull's own precondition list, kept apart from checkLists so
// IsContractVerb continues to answer false for pull while Checks still returns
// the full list for the help and the refusal-set tests.
//
// These are rows 3 to 14 of pull's fourteen-row list, in order; rows 1 and 2
// are the workbench pair Checks prefixes. Two of them are pull's own names:
// ambiguous-column is what the bare form answers when more than one column
// qualifies, and no-upstream is what the named form answers for a column
// standing first in the flow. Pull raises both before any lock is taken,
// which is why neither reaches a generic precondition walker.
//
// Two rows carry not-operator for the operator-owned reservation, one at each
// end of the pull. Row 6 reads the column the card is leaving, which is
// CORE-MOVE-6, and row 11 reads the column it would land in and be claimed at,
// which is CORE-CLAIM-8.
var pullChecks = []Check{
	{Refusal: contract.NoOwner, Key: "check.pull.1"},
	{Refusal: contract.UnknownColumn, Key: "check.pull.2"},
	{Refusal: contract.NotOperator, Key: "check.pull.3"},
	{Refusal: contract.AmbiguousColumn, Key: "check.pull.4"},
	{Refusal: contract.NoUpstream, Key: "check.pull.5"},
	{Refusal: contract.NotOperator, Key: "check.pull.6"},
	{Refusal: contract.Blocked, Key: "check.pull.7"},
	{Refusal: contract.Held, Key: "check.pull.8"},
	{Refusal: contract.Terminal, Key: "check.pull.9"},
	{Refusal: contract.AtCapacity, Key: "check.pull.10"},
	{Refusal: contract.NotOperator, Key: "check.pull.11"},
	{Refusal: contract.Locked, Key: "check.pull.12"},
	{Refusal: contract.UnresolvedItem, Key: "check.pull.13"},
	// Row 14 is Dinah's own tier gate, the claim list's row 8 reached at the
	// destination, and it is appended for the same reason.
	{Refusal: contract.BelowTier, Key: "check.pull.14"},
}

// beyondChecks are the refusals the commands outside the five contract verbs
// report. They are not the profile's lists, so each name here is either one
// the profile already declares and fits, or one carrying Dinah's own prefix.
var beyondChecks = map[string][]Check{
	// Rows 4 and 5 run whenever the corresponding flag is present, since a
	// flag that is present always carries a value and add has no clearing
	// case. Each is evaluated against the axis its own flag names rather than
	// against the workbench, so on a workbench declaring severity and no
	// priority a --severity is filed and only a --priority refuses.
	"add": {
		{Refusal: contract.Malformed, Key: "check.add.1"},
		{Refusal: contract.UnknownColumn, Key: "check.add.2"},
		{Refusal: contract.AtCapacity, Key: "check.add.3"},
		{Refusal: contract.NoLevels, Key: "check.add.4"},
		{Refusal: contract.UnknownLevel, Key: "check.add.5"},
	},
	// The command covers two acts, a clear is a third case within one of
	// them, and --at splits the write in two, so this list carries the
	// mapping the workstream list below carries in its own comment.
	//
	// Row 1 belongs to every act. Row 3 belongs to every act that does not
	// name a column, since runCardSet refuses --at paired with any field but
	// tier as a usage error before either write function runs, and a get
	// evaluates rows 1 and 3 alone because a read validates nothing. Rows 4
	// and 6 run only where a value is present, so a clear evaluates rows 1
	// and 3 and then row 8. Row 8 runs on every write, a clear included,
	// because a clear rewrites the anchor and journals a line and both need
	// an actor.
	//
	// Rows 2, 5, and 7 belong to the --at write alone, which SetCardTierAt
	// carries. Row 2 resolves the column the override is written for. Rows 5
	// and 7 belong to the relative branch of that write: a +N or -N is
	// measured against the column's own tier default, so a column carrying
	// none has nothing to be relative to, and a result off either end of the
	// declared set is out of range. An absolute --at write reaches neither,
	// and reaches row 6 instead when the name is not a declared tier.
	//
	// The three tier rows are inserted where the code runs them rather than
	// appended, which is the opposite of what check.claim.8, check.move.9,
	// and check.pull.14 do. The reason each way is the same reason. Those
	// three lists are the profile's, numbered by the profile document, so a
	// Dinah row among them would renumber a row the profile names. This list
	// is Dinah's own, numbered by nothing outside it, and the page heads the
	// table "What can go wrong, in the order each is checked", so a row
	// printed away from where it runs would break that promise for nothing.
	// The renumbering it costs was ruled on rather than assumed (dinah-408
	// D-10, and dinah-413 carried it out).
	//
	// Rows 4 and 6 take the named field's own axis as their subject, never
	// the workbench. check.card.4's sentence is bound by its last three
	// words: it asks whether this workbench declares a set for that one axis,
	// and reading it as a single workbench-wide test for whether any
	// declaration exists would break the format's posture that the axes are
	// declared independently.
	//
	// The rows-4-and-6 rule is not merely a convenience. A stored level the
	// workbench does not declare is tolerated everywhere and reported by
	// check, and the only workbench where somebody wants to clear one is a
	// workbench whose declaration has since changed or gone. A clear running
	// row 4 would refuse there, leaving the one card that needs clearing as
	// the one card that cannot be cleared.
	//
	// The keys are check.card.N rather than the card-field prefix the two
	// commands below need, because nothing else holds check.card.N and
	// CheckKey composes it.
	"card": {
		{Refusal: contract.UnknownCard, Key: "check.card.1"},
		{Refusal: contract.UnknownColumn, Key: "check.card.2"},
		{Refusal: contract.UnknownField, Key: "check.card.3"},
		{Refusal: contract.NoLevels, Key: "check.card.4"},
		{Refusal: contract.NoTierDefault, Key: "check.card.5"},
		{Refusal: contract.UnknownLevel, Key: "check.card.6"},
		{Refusal: contract.TierOutOfRange, Key: "check.card.7"},
		{Refusal: contract.NoOwner, Key: "check.card.8"},
	},
	"comment": {
		{Refusal: contract.UnknownCard, Key: "check.comment.1"},
		{Refusal: contract.NoOwner, Key: "check.comment.2"},
		{Refusal: contract.Malformed, Key: "check.comment.3"},
	},
	"attach": {
		{Refusal: contract.UnknownPath, Key: "check.attach.1"},
		{Refusal: contract.NoOwner, Key: "check.attach.2"},
		{Refusal: contract.NotAttachable, Key: "check.attach.3"},
		{Refusal: contract.UnknownPath, Key: "check.attach.4"},
	},
	"archive": {
		{Refusal: contract.UnknownPath, Key: "check.archive.1"},
		{Refusal: contract.Occupied, Key: "check.archive.2"},
		{Refusal: contract.LastColumn, Key: "check.archive.3"},
	},
	"delete": {
		{Refusal: contract.UnknownPath, Key: "check.delete.1"},
		{Refusal: contract.Unconfirmed, Key: "check.delete.2"},
		{Refusal: contract.Occupied, Key: "check.delete.3"},
		{Refusal: contract.LastColumn, Key: "check.delete.4"},
	},
	"guide": {
		{Refusal: contract.UnknownGuide, Key: "check.guide.1"},
	},
	"config": {
		{Refusal: contract.UnknownKey, Key: "check.config.1"},
	},
	"init": {
		{Refusal: contract.Exists, Key: "check.init.1"},
		{Refusal: contract.Malformed, Key: "check.init.2"},
	},
	"extract": {
		{Refusal: contract.Exists, Key: "check.extract.1"},
	},
	"edit": {
		{Refusal: contract.UnknownPath, Key: "check.edit.1"},
		{Refusal: contract.NoEditor, Key: "check.edit.2"},
	},
	"path": {
		{Refusal: contract.UnknownPath, Key: "check.path.1"},
	},
	// The field list is read before the reference is resolved, so its row
	// comes first: a call naming a field a card's detail does not carry
	// performs no read at all.
	"show": {
		{Refusal: contract.UnknownField, Key: "check.show.1"},
		{Refusal: contract.UnknownPath, Key: "check.show.2"},
	},
	// rename checks UnknownPath first so a reference naming nothing fails on
	// the resolution it tried rather than on the rename-specific name it did
	// not try yet, then NotRenamable for the reference that resolved to
	// something the verb does not rename, then NoOwner for the same reason
	// the workbench-level pair does, then the name itself. Both name rules
	// are Malformed, since ValidAttachmentName is one gate raising one
	// refusal, and the list draws them as two rows because a reader asking
	// what a name may carry is owed both rules rather than a summary.
	"rename": {
		{Refusal: contract.UnknownPath, Key: "check.rename.1"},
		{Refusal: contract.NotRenamable, Key: "check.rename.2"},
		{Refusal: contract.NoOwner, Key: "check.rename.3"},
		{Refusal: contract.Malformed, Key: "check.rename.4"},
		{Refusal: contract.Malformed, Key: "check.rename.5"},
	},
	"log": {
		{Refusal: contract.UnknownCard, Key: "check.log.1"},
	},
	// The cursor is asked first because a call carrying a bad one is not a
	// call about a card or a column yet. A malformed token and a token minted
	// against another workbench raise the one name, since both are the same
	// finding: the value handed back is not a cursor this workbench issued.
	"changes": {
		{Refusal: contract.Malformed, Key: "check.changes.1"},
		{Refusal: contract.UnknownCard, Key: "check.changes.2"},
		{Refusal: contract.UnknownColumn, Key: "check.changes.3"},
	},
	"ls": {
		{Refusal: contract.UnknownColumn, Key: "check.ls.1"},
	},
	"next": {
		{Refusal: contract.UnknownColumn, Key: "check.next.1"},
	},
	"query": {
		{Refusal: contract.Malformed, Key: "check.query.1"},
		{Refusal: contract.UnknownField, Key: "check.query.2"},
		{Refusal: contract.UnknownField, Key: "check.query.3"},
		{Refusal: contract.UnknownValue, Key: "check.query.4"},
		{Refusal: contract.UnknownColumn, Key: "check.query.5"},
		{Refusal: contract.UnknownValue, Key: "check.query.6"},
		{Refusal: contract.UnknownValue, Key: "check.query.7"},
	},
	// The three axis checks run in this order, so a chain of five axes one
	// of which is a word Dinah does not group on is refused for the unknown
	// word rather than for its length.
	"tree": {
		{Refusal: contract.UnknownAxis, Key: "check.tree.1"},
		{Refusal: contract.RepeatedAxis, Key: "check.tree.2"},
		{Refusal: contract.ChainTooLong, Key: "check.tree.3"},
		{Refusal: contract.UnknownDepth, Key: "check.tree.4"},
	},
	"contents": {
		{Refusal: contract.UnknownPath, Key: "check.contents.1"},
		{Refusal: contract.UnknownDepth, Key: "check.contents.2"},
	},
	"instructions": {
		{Refusal: contract.UnknownPath, Key: "check.instructions.1"},
	},
	"whoami": {
		{Refusal: contract.NoOwner, Key: "check.whoami.1"},
	},
	Join: {
		{Refusal: contract.UnknownCard, Key: "check.join.1"},
		{Refusal: contract.NoOwner, Key: "check.join.2"},
		{Refusal: contract.UnknownWorkstream, Key: "check.join.3"},
	},
	Leave: {
		{Refusal: contract.UnknownCard, Key: "check.leave.1"},
		{Refusal: contract.NoOwner, Key: "check.leave.2"},
		{Refusal: contract.UnknownWorkstream, Key: "check.leave.3"},
	},
	// The keys carry the workbench-field prefix rather than check.workbench.N,
	// which check.workbench.1 and check.workbench.2 above already hold for the
	// two workbench-wide preconditions of section 6.1. One consequence is
	// worth naming: CheckKey composes check.<command>.<order>, so this is the
	// one list whose keys that helper cannot compose.
	//
	// The workbench-level operator check runs ahead of all five at runtime and
	// is not listed, because Checks prefixes WorkbenchChecks onto the five
	// contract verbs alone and every beyond-contract command lists only its
	// own. Listing it here would name one of the workbench-level pair while
	// leaving out the other, in the one command that does it.
	// The keys carry the workstream-field prefix for the reason the workbench
	// list gives: CheckKey composes check.<command>.<order>, and this list
	// covers two acts rather than one, since `new` files a workstream and
	// `set` writes a field of one. Row 1 belongs to get and set, row 3 to all
	// three, and rows 5 and 6 to set alone.
	"workstream": {
		{Refusal: contract.UnknownWorkstream, Key: "check.workstream-field.1"},
		{Refusal: contract.UnknownKey, Key: "check.workstream-field.2"},
		{Refusal: contract.Malformed, Key: "check.workstream-field.3"},
		{Refusal: contract.NoOwner, Key: "check.workstream-field.4"},
		{Refusal: contract.NotOperator, Key: "check.workstream-field.5"},
		{Refusal: contract.Unconfirmed, Key: "check.workstream-field.6"},
	},
	"workbench": {
		{Refusal: contract.UnknownKey, Key: "check.workbench-field.1"},
		{Refusal: contract.Malformed, Key: "check.workbench-field.2"},
		{Refusal: contract.NoOwner, Key: "check.workbench-field.3"},
		{Refusal: contract.NotOperator, Key: "check.workbench-field.4"},
		{Refusal: contract.Unconfirmed, Key: "check.workbench-field.5"},
	},
	// mcp carries the two checks the startup path raises: the directory
	// --root names has to exist, and any workbench the registration names
	// has to lie under that root. The order is the one AC-20 and AC-21
	// exercise: the unknown-root check trips first when both are true at
	// once. The dinah.no-workbench refusal startup case 2 raises is not
	// here because it belongs to workbench discovery rather than to mcp,
	// and no other command's check list carries it either.
	"mcp": {
		{Refusal: contract.UnknownRoot, Key: "check.mcp.1"},
		{Refusal: contract.OutsideRoot, Key: "check.mcp.2"},
	},
	// raise checks the operator itself, as Library.Raise's own first line,
	// the same way reshape does, so the operator row sits in this table
	// rather than being prefixed by Checks: raise is a beyond-contract
	// command, IsContractVerb answers false for it, and nothing prefixes
	// WorkbenchChecks onto a list held here.
	//
	// The order is the order Library.Raise evaluates them in. Identity comes
	// before the reason so that a caller who does not hold the card is told
	// that rather than being asked for prose it would then discard, and the
	// reason comes before the tier resolution so that a raise typed with no
	// justification is refused for the justification whatever the expression
	// would have resolved to.
	Raise: {
		{Refusal: contract.NoOperator, Key: "check.raise.1"},
		{Refusal: contract.UnknownCard, Key: "check.raise.2"},
		{Refusal: contract.NoOwner, Key: "check.raise.3"},
		{Refusal: contract.NotHolder, Key: "check.raise.4"},
		{Refusal: contract.NoReason, Key: "check.raise.5"},
		{Refusal: contract.UnknownColumn, Key: "check.raise.6"},
		{Refusal: contract.NoTierDefault, Key: "check.raise.7"},
		{Refusal: contract.UnknownLevel, Key: "check.raise.8"},
		{Refusal: contract.TierOutOfRange, Key: "check.raise.9"},
		{Refusal: contract.TierNotHigher, Key: "check.raise.10"},
	},
	// reshape's list is the order a reader meets the refusals in, which is
	// the order the help text reads best in: the workbench and the owner
	// first, then the source, then each destination as it is resolved, then
	// the two questions asked once every destination is known. It is not the
	// evaluation order in two places, and saying that it was cost a reviewer
	// the time to check it: an empty --from raises dinah.malformed before the
	// source read can raise dinah.unknown-path, and resolveReshapeDestination
	// tests for a retiring destination ahead of both of the refusals listed
	// above it.
	//
	// dinah.no-workbench is not listed, for the reason mcp's list gives for
	// leaving it out: it belongs to workbench discovery rather than to any
	// verb, and no other command's list carries it either.
	"reshape": {
		{Refusal: contract.NoOperator, Key: "check.reshape.1"},
		{Refusal: contract.NoOwner, Key: "check.reshape.2"},
		{Refusal: contract.UnknownPath, Key: "check.reshape.3"},
		{Refusal: contract.Malformed, Key: "check.reshape.4"},
		{Refusal: contract.ReshapeDestinationAmbiguous, Key: "check.reshape.5"},
		{Refusal: contract.UnknownColumn, Key: "check.reshape.6"},
		{Refusal: contract.ReshapeDestinationRetiring, Key: "check.reshape.7"},
		{Refusal: contract.ReshapeMapSourceEmpty, Key: "check.reshape.8"},
		{Refusal: contract.ReshapeNeedsDestination, Key: "check.reshape.9"},
		{Refusal: contract.ReshapeHeldCardInQueue, Key: "check.reshape.10"},
	},
}

// Checks returns the ordered precondition list of a command, prefixed by the
// two workbench-level checks for the five contract verbs and for any other
// command whose transaction runs them. It is what per-verb help is generated
// from, so the help text and the behaviour move together.
func Checks(name string) []Check {
	own, found := ownChecks(name)
	if !found {
		return nil
	}
	if !runsWorkbenchChecks(name) {
		return append([]Check{}, own...)
	}
	return append(append([]Check{}, WorkbenchChecks...), own...)
}

// ownChecks returns a command's own precondition list, without the workbench
// pair, and reports whether the command declares one at all. Pull's list is
// held apart from checkLists so that IsContractVerb goes on answering false
// for it while Checks still returns the whole list the help is generated
// from.
func ownChecks(name string) ([]Check, bool) {
	if list, ok := checkLists[name]; ok {
		return list, true
	}
	if name == Pull {
		return pullChecks, true
	}
	list, ok := beyondChecks[name]
	return list, ok
}

// runsWorkbenchChecks reports whether a command's transaction evaluates the
// workbench pair ahead of its own list. The five contract verbs do, and so
// does pull, whose transaction is a claim and a move and whose refusals
// therefore begin where theirs begin.
func runsWorkbenchChecks(name string) bool {
	return IsContractVerb(name) || name == Pull
}

// IsContractVerb reports whether a name is one of the five the profile
// specifies, which is what decides whether a command's refusals come from the
// contract's vocabulary or from Dinah's own layer.
func IsContractVerb(name string) bool {
	_, ok := checkLists[name]
	return ok
}

// CheckKey composes the catalog key of one check, so a caller reading the
// profile's own list can name the same rows the code names.
func CheckKey(command string, order int) string {
	return "check." + command + "." + strconv.Itoa(order)
}
