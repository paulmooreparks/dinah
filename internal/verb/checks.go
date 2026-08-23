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
	},
	Move: {
		{Refusal: contract.UnknownCard, Key: "check.move.1"},
		{Refusal: contract.UnknownState, Key: "check.move.2"},
		{Refusal: contract.NotOperator, Key: "check.move.3"},
		{Refusal: contract.NotOperator, Key: "check.move.4"},
		{Refusal: contract.Blocked, Key: "check.move.5"},
		{Refusal: contract.Held, Key: "check.move.6"},
		{Refusal: contract.Terminal, Key: "check.move.7"},
		{Refusal: contract.AtCapacity, Key: "check.move.8"},
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
// These are rows 3 to 13 of pull's thirteen-row list, in order; rows 1 and 2
// are the workbench pair Checks prefixes. Two of them are pull's own names:
// ambiguous-state is what the bare form answers when more than one state
// qualifies, and no-upstream is what the named form answers for a state
// standing first in the flow. Pull raises both before any lock is taken,
// which is why neither reaches a generic precondition walker.
var pullChecks = []Check{
	{Refusal: contract.NoOwner, Key: "check.pull.1"},
	{Refusal: contract.UnknownState, Key: "check.pull.2"},
	{Refusal: contract.NotOperator, Key: "check.pull.3"},
	{Refusal: contract.AmbiguousState, Key: "check.pull.4"},
	{Refusal: contract.NoUpstream, Key: "check.pull.5"},
	{Refusal: contract.NotOperator, Key: "check.pull.6"},
	{Refusal: contract.Blocked, Key: "check.pull.7"},
	{Refusal: contract.Held, Key: "check.pull.8"},
	{Refusal: contract.Terminal, Key: "check.pull.9"},
	{Refusal: contract.AtCapacity, Key: "check.pull.10"},
	{Refusal: contract.Locked, Key: "check.pull.11"},
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
		{Refusal: contract.UnknownState, Key: "check.add.2"},
		{Refusal: contract.AtCapacity, Key: "check.add.3"},
		{Refusal: contract.NoLevels, Key: "check.add.4"},
		{Refusal: contract.UnknownLevel, Key: "check.add.5"},
	},
	// The command covers two acts and a clear is a third case within one of
	// them, so this list carries the mapping the workstream list below
	// carries in its own comment. Rows 1 and 2 belong to get and set alike.
	// Rows 3 and 4 run only where a value is present, so a clear evaluates
	// rows 1, 2 and 5, and get evaluates rows 1 and 2 alone, since a read
	// validates nothing. Row 5 runs on every write, a clear included, because
	// a clear rewrites the anchor and journals a line and both need an actor.
	//
	// Rows 3 and 4 take the named field's own axis as their subject, never
	// the workbench. check.card.3's sentence is bound by its last three
	// words: it asks whether this workbench declares a set for that one axis,
	// and reading it as a single workbench-wide test for whether any
	// declaration exists would break the format's posture that the two axes
	// are declared independently.
	//
	// The rows-3-and-4 rule is not merely a convenience. A stored level the
	// workbench does not declare is tolerated everywhere and reported by
	// check, and the only workbench where somebody wants to clear one is a
	// workbench whose declaration has since changed or gone. A clear running
	// row 3 would refuse there, leaving the one card that needs clearing as
	// the one card that cannot be cleared.
	//
	// The keys are check.card.N rather than the card-field prefix the two
	// commands below need, because nothing else holds check.card.N and
	// CheckKey composes it.
	"card": {
		{Refusal: contract.UnknownCard, Key: "check.card.1"},
		{Refusal: contract.UnknownField, Key: "check.card.2"},
		{Refusal: contract.NoLevels, Key: "check.card.3"},
		{Refusal: contract.UnknownLevel, Key: "check.card.4"},
		{Refusal: contract.NoOwner, Key: "check.card.5"},
	},
	"comment": {
		{Refusal: contract.UnknownCard, Key: "check.comment.1"},
		{Refusal: contract.NoOwner, Key: "check.comment.2"},
		{Refusal: contract.Malformed, Key: "check.comment.3"},
	},
	"attach": {
		{Refusal: contract.UnknownPath, Key: "check.attach.1"},
		{Refusal: contract.NoOwner, Key: "check.attach.2"},
	},
	"archive": {
		{Refusal: contract.UnknownPath, Key: "check.archive.1"},
		{Refusal: contract.Occupied, Key: "check.archive.2"},
		{Refusal: contract.LastState, Key: "check.archive.3"},
	},
	"delete": {
		{Refusal: contract.UnknownPath, Key: "check.delete.1"},
		{Refusal: contract.Unconfirmed, Key: "check.delete.2"},
		{Refusal: contract.Occupied, Key: "check.delete.3"},
		{Refusal: contract.LastState, Key: "check.delete.4"},
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
	"show": {
		{Refusal: contract.UnknownPath, Key: "check.show.1"},
	},
	// rename checks UnknownPath first so a reference that resolves to
	// anything but an attachment fails on the resolution it tried rather
	// than on the rename-specific name it did not try yet, then NoOwner
	// for the same reason the workbench-level pair does, then Malformed
	// for the name itself, then NotRenamable for the case the reference
	// resolved to something the verb refuses.
	"rename": {
		{Refusal: contract.UnknownPath, Key: "check.rename.1"},
		{Refusal: contract.NoOwner, Key: "check.rename.2"},
		{Refusal: contract.Malformed, Key: "check.rename.3"},
		{Refusal: contract.NotRenamable, Key: "check.rename.4"},
	},
	"log": {
		{Refusal: contract.UnknownCard, Key: "check.log.1"},
	},
	// The cursor is asked first because a call carrying a bad one is not a
	// call about a card or a state yet. A malformed token and a token minted
	// against another workbench raise the one name, since both are the same
	// finding: the value handed back is not a cursor this workbench issued.
	"changes": {
		{Refusal: contract.Malformed, Key: "check.changes.1"},
		{Refusal: contract.UnknownCard, Key: "check.changes.2"},
		{Refusal: contract.UnknownState, Key: "check.changes.3"},
	},
	"ls": {
		{Refusal: contract.UnknownState, Key: "check.ls.1"},
	},
	"next": {
		{Refusal: contract.UnknownState, Key: "check.next.1"},
	},
	"query": {
		{Refusal: contract.Malformed, Key: "check.query.1"},
		{Refusal: contract.UnknownField, Key: "check.query.2"},
		{Refusal: contract.UnknownField, Key: "check.query.3"},
		{Refusal: contract.UnknownValue, Key: "check.query.4"},
		{Refusal: contract.UnknownState, Key: "check.query.5"},
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
