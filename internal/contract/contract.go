// Package contract holds the machine vocabulary of the core profile: the
// outcome tokens, the declared refusal names, the substates, the state kinds
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

// The sixteen refusal names section 6.1 of the profile declares. A refusal
// Dinah reports that is not one of these carries the layer prefix of
// LayerPrefix, which CORE-OUT-3 admits and DOC-LAYER-1 keeps collision-free.
const (
	UnknownCard       = "unknown-card"
	UnknownState      = "unknown-state"
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
)

// Declared lists the profile's sixteen refusal names in the order section 6.1
// prints them.
var Declared = []string{
	UnknownCard, UnknownState, UnsupportedVer, Held, NotRequester,
	Blocked, NotBlocked, NotHolder, AtCapacity, NotOperator,
	NoOperator, NoOwner, NoReason, Terminal, Malformed, LayerCollisionErr,
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
	UnknownKey   = LayerPrefix + "unknown-key"
	Occupied     = LayerPrefix + "occupied"
	Locked       = LayerPrefix + "locked"
	Exists       = LayerPrefix + "exists"
	UnknownPath  = LayerPrefix + "unknown-path"
	NoEditor     = LayerPrefix + "no-editor"
	NoWorkbench  = LayerPrefix + "no-workbench"
	UnknownVerb  = LayerPrefix + "unknown-command"
	Usage        = LayerPrefix + "usage"

	// NoWorkbenchFound is the walk coming up empty, which NoWorkbench once
	// shared a sentence with. The two are separated because one template
	// cannot honestly describe both a path the caller named and a search
	// that reached the root of the filesystem.
	NoWorkbenchFound = LayerPrefix + "no-workbench-found"
	// AmbiguousWorkbench is a base directory holding several workbenches
	// with nothing closer to choose between them. The tool refuses to
	// guess, so it names the candidates instead.
	AmbiguousWorkbench = LayerPrefix + "ambiguous-workbench"
	// LastState is archiving or deleting the one state a workbench has
	// left, which CORE-BENCH-2 forbids the workbench from ending up with
	// none of.
	LastState = LayerPrefix + "last-state"
	// UnreadableBench is a workbench.md the discovery walk found and could
	// not read. The walk stops there rather than climbing past it or
	// reporting it as absent, because a file it could not open might be the
	// real workbench.
	UnreadableBench = LayerPrefix + "unreadable-workbench"
	// NoConfiguredWorkbench is the workbench setting naming a path that no
	// longer carries a workbench.md, consulted only once the search has
	// found nothing local to answer with. It is a distinct name from
	// NoWorkbench, which the override branch already carries for the same
	// underlying condition, because the two need different sentences: one
	// names what the caller just typed, the other names what was stored
	// earlier and may have gone stale with nobody around to notice.
	NoConfiguredWorkbench = LayerPrefix + "no-configured-workbench"
	// WorkbenchNotApplicable is --workbench or DINAH_WORKBENCH given to
	// init. Every other verb reads the flag as the path to a workbench that
	// already exists; init has none yet at the path it is about to create,
	// so the flag names nothing init can act on.
	WorkbenchNotApplicable = LayerPrefix + "workbench-not-applicable"
	// RepairWouldEmptyStates is dinah check --migrate-states declining to
	// remove every remaining stranded state, which CORE-BENCH-2 forbids
	// leaving the workbench definition with none of.
	RepairWouldEmptyStates = LayerPrefix + "repair-would-empty-states"
	// AddNeedsAState is Add declining to file a card into a workbench whose
	// states list has no live entries left for the card to land in.
	AddNeedsAState = LayerPrefix + "add-needs-a-state"
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
	// MultipleWords is an open-tail command's free-text slot (add's title,
	// block's reason, comment's text, config set's value) getting more than
	// one unquoted word. The sentence names the word count and rebuilds the
	// command line with the free text quoted, since that is the whole cost
	// of the rule and the fix a reader needs to see.
	MultipleWords = LayerPrefix + "multiple-words"
	// UnknownWorkstream is a reference naming no workstream of this
	// workbench, in either half of the collection. It is separate from
	// unknown-state because a workstream is not a station of the flow, and
	// a reader told about a state they never named would go looking in the
	// wrong listing.
	UnknownWorkstream = LayerPrefix + "unknown-workstream"
	// Referenced is deleting a workstream that live cards still belong to.
	// It is separate from Occupied, whose sentence says cards still occupy
	// a state, because a card belongs to a workstream and stands in a
	// state, and one sentence cannot honestly say both.
	Referenced = LayerPrefix + "referenced"
)

// Introduced lists every refusal name Dinah mints beyond the profile's own.
var Introduced = []string{
	Unconfirmed, UnknownGuide, UnknownKey, Occupied, Locked, Exists,
	UnknownPath, NoEditor, NoWorkbench, UnknownVerb, Usage, Interrupted,
	NoWorkbenchFound, AmbiguousWorkbench, LastState, UnreadableBench, NoConfiguredWorkbench,
	WorkbenchNotApplicable, RepairWouldEmptyStates, AddNeedsAState, MultipleWords,
	UnknownWorkstream, Referenced, UnknownField, UnknownValue,
}

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

// The three substates of a card. The set is closed, because the tool enforces
// the meaning of each member.
const (
	SubstateReady   = "ready"
	SubstateActive  = "active"
	SubstateBlocked = "blocked"
)

// The three state kinds of the flow.
const (
	KindIntake = "intake"
	KindWork   = "work"
	KindDone   = "done"
)

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
	EventArchived           = "archived"
	EventRestored           = "restored"
	EventDeleted            = "deleted"
	EventManualCorrection   = "manual_correction"
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
	// EventWorkstreamJoined and EventWorkstreamLeft record a card entering
	// and leaving a workstream, on the card's own journal, because membership
	// is card-owned and the card is the file that changed. Each carries the
	// workstream's identifier in Workstream.
	EventWorkstreamJoined = "workstream_joined"
	EventWorkstreamLeft   = "workstream_left"
)

// Events lists the fifteen journal event names in the order the constants
// above declare them, so a caller checking a value against the closed set
// reads one list rather than repeating it.
var Events = []string{
	EventCreated, EventClaimed, EventMoved, EventReleased, EventBlocked,
	EventUnblocked, EventExpired, EventCommented, EventAttached,
	EventAttachmentReplaced, EventAttachmentRemoved, EventArchived,
	EventRestored, EventDeleted, EventManualCorrection,
}

// Refusal is the error a verb returns when a rule says no. It carries the one
// refusal name CORE-OUT-2 requires and a detail the head renders for a person.
type Refusal struct {
	// Name is the refusal name, from the profile's sixteen or dotted.
	Name string
	// Detail names what the refusal was about: the state asked for, the
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
