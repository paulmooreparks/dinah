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
)

// ContractVerbs are the five verbs the profile specifies, in the order
// section 6 states them.
var ContractVerbs = []string{Claim, Move, Release, Block, Unblock}

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

// beyondChecks are the refusals the commands outside the five contract verbs
// report. They are not the profile's lists, so each name here is either one
// the profile already declares and fits, or one carrying Dinah's own prefix.
var beyondChecks = map[string][]Check{
	"add": {
		{Refusal: contract.Malformed, Key: "check.add.1"},
		{Refusal: contract.UnknownState, Key: "check.add.2"},
		{Refusal: contract.AtCapacity, Key: "check.add.3"},
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
	"log": {
		{Refusal: contract.UnknownCard, Key: "check.log.1"},
	},
	"ls": {
		{Refusal: contract.UnknownState, Key: "check.ls.1"},
	},
	"next": {
		{Refusal: contract.UnknownState, Key: "check.next.1"},
	},
	"instructions": {
		{Refusal: contract.UnknownPath, Key: "check.instructions.1"},
	},
	"whoami": {
		{Refusal: contract.NoOwner, Key: "check.whoami.1"},
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
	"workbench": {
		{Refusal: contract.UnknownKey, Key: "check.workbench-field.1"},
		{Refusal: contract.Malformed, Key: "check.workbench-field.2"},
		{Refusal: contract.NoOwner, Key: "check.workbench-field.3"},
		{Refusal: contract.NotOperator, Key: "check.workbench-field.4"},
		{Refusal: contract.Unconfirmed, Key: "check.workbench-field.5"},
	},
}

// Checks returns the ordered precondition list of a command, prefixed by the
// two workbench-level checks for the five contract verbs. It is what per-verb
// help is generated from, so the help text and the behaviour move together.
func Checks(name string) []Check {
	if list, ok := checkLists[name]; ok {
		return append(append([]Check{}, WorkbenchChecks...), list...)
	}
	if list, ok := beyondChecks[name]; ok {
		return append([]Check{}, list...)
	}
	return nil
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
