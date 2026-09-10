// Package addressform declares the spellings Dinah accepts for an address,
// the spellings it refuses, and the functions in internal/bench that answer
// one. It is read by test files in cmd/dinah and by nothing else.
//
// It exists because dinah-471 found a whole addressing form the tool accepts
// and the references guide never taught: a card answers to its own twelve-hex
// identifier, to its bare number, and to a reference carrying any prefix at
// all. The derived table in the references guide catches a command drifting
// out of the guide, and nothing asked the reverse question, which is what the
// tool accepts that the guide never mentions.
//
// internal/guide/guidepin is the precedent for a non-test package no
// production file imports. It exists for the same reason this one does, which
// is that a declaration two test packages both read has to live where both
// can reach it.
package addressform

import (
	"path"

	"dinah/internal/verb"
)

// AddressForm is one way of naming a thing to Dinah. Each constant's string
// value is its dash-joined name, spelled the way this tree spells machine
// vocabulary.
type AddressForm string

// The head forms. A head form names an entity at the head of a reference, and
// it carries the verb.ReferenceKind that entity is.
const (
	// WorkbenchWord is the literal word workbench.
	WorkbenchWord AddressForm = "workbench-word"
	// WorkbenchDot is the single dot.
	WorkbenchDot AddressForm = "workbench-dot"
	// WorkbenchSlugHead is the workbench's own slug carrying something below
	// it, which is the only place a bare slug names the workbench.
	WorkbenchSlugHead AddressForm = "workbench-slug-head"
	// CardReference is the workbench's slug joined to the card's number.
	CardReference AddressForm = "card-reference"
	// CardIdentifier is the card's twelve-hex identifier standing alone.
	CardIdentifier AddressForm = "card-identifier"
	// CardNumber is the card's number standing alone.
	CardNumber AddressForm = "card-number"
	// CardStalePrefix is a card reference whose prefix names no current slug.
	CardStalePrefix AddressForm = "card-stale-prefix"
	// ColumnSlug is a column's slug.
	ColumnSlug AddressForm = "column-slug"
	// ColumnTitle is a column's title.
	ColumnTitle AddressForm = "column-title"
	// ColumnIdentifier is a column's twelve-hex identifier.
	ColumnIdentifier AddressForm = "column-identifier"
	// WorkstreamPrefixedSlug is the word workstream, a slash, and the slug.
	WorkstreamPrefixedSlug AddressForm = "workstream-prefixed-slug"
	// WorkstreamPrefixedIdentifier is the same with the identifier.
	WorkstreamPrefixedIdentifier AddressForm = "workstream-prefixed-identifier"
	// WorkstreamBareSlug is the slug standing alone, which the commands that
	// take a workstream accept and a reference read as an address refuses.
	WorkstreamBareSlug AddressForm = "workstream-bare-slug"
	// WorkstreamBareIdentifier is the identifier standing alone, on the same
	// split.
	WorkstreamBareIdentifier AddressForm = "workstream-bare-identifier"
)

// The selector forms. A selector form picks one member out of a collection,
// and it carries no kind at all. verb.ReferenceKind is the vocabulary of what
// a command's reference may name, and a selector names a step within a
// reference rather than a thing a reference names, so none of its six values
// is honest here. Collection is wrong because the reference names one member
// rather than the collection, and below-card is wrong because a member's
// holder need not be a card, as wb/attachments/1 shows.
const (
	// MemberPosition is the member's ordinal within its collection.
	MemberPosition AddressForm = "member-position"
	// MemberIdentifier is the member's own twelve-hex identifier.
	MemberIdentifier AddressForm = "member-identifier"
	// MemberName is an attachment's filename.
	MemberName AddressForm = "member-name"
)

// The commands a declaration or a near miss is measured through.
const (
	// CommandPath is `dinah path`, which answers the file a reference names.
	CommandPath = "path"
	// CommandJoin is `dinah join`, which takes a card and a workstream and is
	// where the bare workstream handle is accepted.
	CommandJoin = "join"
)

// Declaration is one address form, the kind it names, the guide sentence that
// teaches it, and the command one example of it is run through.
type Declaration struct {
	// Form is the constant this row declares.
	Form AddressForm
	// Kind is the reference kind a head form names, and is empty on a
	// selector form.
	Kind verb.ReferenceKind
	// Guide is the references-guide text that teaches this form, verbatim.
	// guidepin.Carries folds both sides to single spaces before it searches,
	// so a pin survives a re-wrap of the guide's source.
	Guide string
	// Command is the command one example of this form is run through. A form
	// is measured through a command rather than in the abstract, because the
	// same spelling can be accepted by one command and refused by another:
	// a bare workstream slug is what `dinah join` takes and what `dinah path`
	// refuses.
	Command string
}

// declarations is the roster, head forms first and selectors after them, in
// the order the references guide teaches them.
//
// Four rows pin something other than the sentence nearest them, so that the
// forms sharing one sentence stay at the three groups the guard declares.
// WorkbenchWord and WorkbenchDot pin their example lines, because the sentence
// standing above the block teaches all three workbench forms at once and only
// WorkbenchSlugHead has prose of its own; Carries searches the whole guide
// text rather than its prose alone, so an example line is a pin like any
// other. MemberPosition pins the creation-order sentence rather than the
// precedence sentence, which teaches all three selectors at once, and
// MemberIdentifier and MemberName then take the two halves of the sentence
// dinah-471 narrowed.
var declarations = []Declaration{
	{Form: WorkbenchWord, Kind: verb.ReferenceKindWorkbench, Command: CommandPath,
		Guide: "dinah path workbench"},
	{Form: WorkbenchDot, Kind: verb.ReferenceKindWorkbench, Command: CommandPath,
		Guide: "dinah path ."},
	{Form: WorkbenchSlugHead, Kind: verb.ReferenceKindWorkbench, Command: CommandPath,
		Guide: "Write the third, which is your workbench's own slug, with something below it, because Dinah reads a slug standing alone as a card and refuses it."},
	{Form: CardReference, Kind: verb.ReferenceKindCard, Command: CommandPath,
		Guide: "You write a card as its reference, which is your workbench's slug and the card's number"},
	{Form: CardIdentifier, Kind: verb.ReferenceKindCard, Command: CommandPath,
		Guide: "You may also write the card's identifier on its own"},
	{Form: CardNumber, Kind: verb.ReferenceKindCard, Command: CommandPath,
		Guide: "You may also write the card's number on its own"},
	{Form: CardStalePrefix, Kind: verb.ReferenceKindCard, Command: CommandPath,
		Guide: "a reference carrying a prefix that names no current slug still opens the card it named"},
	{Form: ColumnSlug, Kind: verb.ReferenceKindColumn, Command: CommandPath,
		Guide: "You write a column as its slug, its name, or its identifier"},
	{Form: ColumnTitle, Kind: verb.ReferenceKindColumn, Command: CommandPath,
		Guide: "You write a column as its slug, its name, or its identifier"},
	{Form: ColumnIdentifier, Kind: verb.ReferenceKindColumn, Command: CommandPath,
		Guide: "You write a column as its slug, its name, or its identifier"},
	{Form: WorkstreamPrefixedSlug, Kind: verb.ReferenceKindWorkstream, Command: CommandPath,
		Guide: "You write a workstream as the word workstream, a slash, and the workstream's slug or its identifier"},
	{Form: WorkstreamPrefixedIdentifier, Kind: verb.ReferenceKindWorkstream, Command: CommandPath,
		Guide: "You write a workstream as the word workstream, a slash, and the workstream's slug or its identifier"},
	{Form: WorkstreamBareSlug, Kind: verb.ReferenceKindWorkstream, Command: CommandJoin,
		Guide: "The commands that take a workstream also accept the slug or the identifier on its own"},
	{Form: WorkstreamBareIdentifier, Kind: verb.ReferenceKindWorkstream, Command: CommandJoin,
		Guide: "The commands that take a workstream also accept the slug or the identifier on its own"},
	{Form: MemberPosition, Command: CommandPath,
		Guide: "The number counts in the order the entities were created"},
	{Form: MemberIdentifier, Command: CommandPath,
		Guide: "You may write a collection member's own identifier in place of its number"},
	{Form: MemberName, Command: CommandPath,
		Guide: "you may write an attachment's filename in place of its number"},
}

// Declarations returns every declared address form, in the order the roster
// carries them.
func Declarations() []Declaration {
	roster := make([]Declaration, len(declarations))
	copy(roster, declarations)
	return roster
}

// The refusal names the near misses carry, spelled as the payload spells them.
const (
	// RefusalUnknownCard is what a bare head naming no card raises.
	RefusalUnknownCard = "unknown-card"
	// RefusalUnknownPath is what a step below a resolved head raises when
	// nothing in the workbench answers to it.
	RefusalUnknownPath = "dinah.unknown-path"
)

// NearMiss is a spelling Dinah refuses, the command it was measured through,
// and the refusal name that command's --json payload carries.
type NearMiss struct {
	// Key names the near miss in a failure.
	Key string
	// Command is the command this spelling was measured through.
	Command string
	// Refusal is the refusal name the --json payload carries.
	Refusal string
	// Why says what a reader was reaching for when they typed it.
	Why string
}

// nearMisses are the spellings a reader reaches for and Dinah refuses.
//
// The last row and the WorkstreamBareSlug head form are one string measured
// through two commands, and both rows are required. A criterion asserting a
// refusal passes against code that refuses everything, so the accepting case
// is pinned beside the refusing one, and here the two are the same spelling.
var nearMisses = []NearMiss{
	{Key: "card-slug-and-identifier", Command: CommandPath, Refusal: RefusalUnknownCard,
		Why: "the guide's sentence about a collection member's identifier once read as though a card were reached by substituting its identifier into its reference"},
	{Key: "card-through-its-holder", Command: CommandPath, Refusal: RefusalUnknownPath,
		Why: "a card is addressed in its own right rather than through the collection that holds it"},
	{Key: "workbench-bare-slug", Command: CommandPath, Refusal: RefusalUnknownCard,
		Why: "the workbench's slug names the workbench only at the head of a longer path, and standing alone it reads as a card"},
	{Key: "workbench-directory-name", Command: CommandPath, Refusal: RefusalUnknownCard,
		Why: "the workbench's directory carries a thirty-two-character name that no reference spells"},
	{Key: "column-with-a-kind-prefix", Command: CommandPath, Refusal: RefusalUnknownCard,
		Why: "a column takes no kind prefix, unlike a workstream, so the prefix is read as the head itself"},
	{Key: "workstream-bare-handle-as-an-address", Command: CommandPath, Refusal: RefusalUnknownCard,
		Why: "a bare handle where a reference is read as an address is read as a card, which is what keeps a workstream from shadowing one"},
}

// NearMisses returns every declared near miss, in the order the roster carries
// them.
func NearMisses() []NearMiss {
	roster := make([]NearMiss, len(nearMisses))
	copy(roster, nearMisses)
	return roster
}

// PackageDir is the package the rostered functions live in.
const PackageDir = "internal/bench"

// packageOwnFile names the file called after the package itself, composed
// from PackageDir rather than spelled out. internal/profile's vocabulary
// guard refuses the short form of the product's word inside a Go string
// literal, and the remedy it names is to rewrite the text rather than to
// widen its list of exceptions, so the one file name that would carry it is
// derived from the package path the guard already admits.
var packageOwnFile = path.Base(PackageDir) + ".go"

// Resolver is one function in internal/bench that answers a caller's string,
// with the arm counts it carries and the forms its accepting arms serve.
type Resolver struct {
	// File is the file within internal/bench, such as "resolve.go".
	File string
	// Function is the function's own name.
	Function string
	// Returns is every return statement in the function's own body. It sees
	// every arm, including one that delegates and passes an error through, of
	// the shape `return path, err`, and it also moves when an unrelated
	// refusal is added.
	Returns int
	// Accepting is the returns that answer something with no error. It is the
	// meaningful number and it pairs with Forms, and it is the one that does
	// not see a delegating arm. Both counts are declared because each catches
	// what the other misses.
	Accepting int
	// Forms are the address forms this function's accepting arms serve. It is
	// empty where a function's arms match a declared segment vocabulary or a
	// step of the containment grammar rather than a property of an entity.
	Forms []AddressForm
	// Why says what this function resolves, in one sentence.
	Why string
}

// resolvers are the eighteen functions in internal/bench that answer an
// address. The counts were produced by an AST counter rather than by reading,
// and cmd/dinah/address_form_arms_test.go recomputes both columns on every
// run, so this table is a contract rather than a note.
var resolvers = []Resolver{
	{File: packageOwnFile, Function: "Column", Returns: 2, Accepting: 1,
		Forms: []AddressForm{ColumnIdentifier},
		Why:   "answers a column by its own identifier"},
	{File: packageOwnFile, Function: "ColumnByRef", Returns: 4, Accepting: 3,
		Forms: []AddressForm{ColumnIdentifier, ColumnSlug, ColumnTitle},
		Why:   "answers a column by identifier, then by slug, then by title"},
	{File: "entity.go", Function: "resolveWorkstreamRef", Returns: 3, Accepting: 1,
		Forms: []AddressForm{WorkstreamPrefixedSlug, WorkstreamPrefixedIdentifier},
		Why:   "answers a workstream reference carrying the workstream prefix"},
	{File: "resolve.go", Function: "resolveCardIn", Returns: 8, Accepting: 2,
		Forms: []AddressForm{CardIdentifier, CardReference, CardNumber, CardStalePrefix},
		Why:   "answers a card by its identifier or by the number a reference ends in"},
	{File: "resolve.go", Function: "resolvePathBody", Returns: 4, Accepting: 2,
		Why: "answers the file a resolved reference names"},
	{File: "resolve.go", Function: "resolveReferenceBody", Returns: 11, Accepting: 4,
		Forms: []AddressForm{WorkbenchWord, WorkbenchDot, WorkbenchSlugHead, ColumnSlug, ColumnTitle, ColumnIdentifier, CardReference, CardIdentifier, CardNumber, CardStalePrefix},
		Why:   "reads the head of a reference, asking the column lookup before the card lookup"},
	{File: "resolve.go", Function: "collectionAt", Returns: 2, Accepting: 1,
		Why: "answers the collection a reference stops at"},
	{File: "resolve.go", Function: "collectionHolder", Returns: 2, Accepting: 2,
		Why: "answers the entity a collection hangs off"},
	{File: "resolve.go", Function: "resolveBelowLanding", Returns: 6, Accepting: 2,
		Why: "answers the card a below-a-card reference lands on"},
	{File: "resolve.go", Function: "walkBelowCard", Returns: 6, Accepting: 5,
		Why: "walks the segments below a card, which are a declared segment vocabulary rather than address forms"},
	{File: "resolve.go", Function: "descend", Returns: 8, Accepting: 4,
		Why: "walks one step below a resolved head"},
	{File: "resolve.go", Function: "pick", Returns: 8, Accepting: 3,
		Forms: []AddressForm{MemberIdentifier, MemberPosition, MemberName},
		Why:   "picks one member out of a collection by identifier, then by position, then by name"},
	{File: "resolve.go", Function: "ResolveLinkTarget", Returns: 5, Accepting: 3,
		Why: "answers the reference a card's recorded link names"},
	{File: "resolve.go", Function: "columnByRefIn", Returns: 2, Accepting: 2,
		Forms: []AddressForm{ColumnIdentifier, ColumnSlug, ColumnTitle},
		Why:   "answers a column in either half of the workbench"},
	{File: "resolve.go", Function: "ArchivedColumnByRef", Returns: 5, Accepting: 3,
		Forms: []AddressForm{ColumnIdentifier, ColumnSlug, ColumnTitle},
		Why:   "answers an archived column by identifier, then by slug, then by title"},
	{File: "resolve.go", Function: "workstreamByRefIn", Returns: 5, Accepting: 3,
		Forms: []AddressForm{WorkstreamPrefixedSlug, WorkstreamPrefixedIdentifier, WorkstreamBareSlug, WorkstreamBareIdentifier},
		Why:   "answers a workstream in either half of the workbench"},
	{File: "workstream.go", Function: "Workstream", Returns: 3, Accepting: 1,
		Forms: []AddressForm{WorkstreamBareIdentifier},
		Why:   "answers a workstream by its own identifier"},
	{File: "workstream.go", Function: "WorkstreamByRef", Returns: 4, Accepting: 2,
		Forms: []AddressForm{WorkstreamPrefixedSlug, WorkstreamPrefixedIdentifier, WorkstreamBareSlug, WorkstreamBareIdentifier},
		Why:   "strips one workstream prefix and answers by identifier, then by slug"},
}

// Resolvers returns every rostered resolver, in the order the roster carries
// them.
func Resolvers() []Resolver {
	roster := make([]Resolver, len(resolvers))
	copy(roster, resolvers)
	return roster
}

// The grounds a function may be exempted from the address sweep on. The set
// is closed: an exemption carrying anything else fails the run rather than
// passing unread, so a ground is an argument a reviewer weighs rather than a
// word anybody can invent. cmd/dinah/address_sweep_test.go is the shape this
// follows.
const (
	// GroundLoadsByIdentifier is a function reached only after an accepting
	// arm has already matched, which loads an entity out of a directory
	// rather than deciding which entity a spelling names.
	GroundLoadsByIdentifier = "ground-loads-by-identifier"
	// GroundCreates is a function that mints an entity rather than finding
	// one, so its string argument is a title, a filename or an identifier it
	// is about to store.
	GroundCreates = "ground-creates"
	// GroundLists is a function that answers a whole collection, so its
	// string argument names where to read rather than which member to take.
	GroundLists = "ground-lists"
	// GroundDelegates is a function that answers a caller's string by handing
	// it to a rostered resolver and returning what that resolver answered,
	// adding no accepting arm of its own.
	GroundDelegates = "ground-delegates"
)

// Exemption is one function the sweep of internal/bench finds and the resolver
// roster does not carry, with the ground it is excused on and the reasoning.
type Exemption struct {
	// Function is the function's own name.
	Function string
	// Ground is one of the four declared grounds.
	Ground string
	// Reason is why that ground holds of this function.
	Reason string
}

// exemptions are the thirty-three functions the sweep finds that answer a
// caller's string with an entity without deciding which entity a spelling
// names.
//
// GroundDelegates was checked against the seven bodies rather than against
// their names. ResolveCard and ResolveArchivedCard are one-line calls to
// resolveCardIn, ResolveReference and ResolveEntity are one-line calls to
// their own In siblings, resolveBelow is a one-line call to
// resolveBelowLanding, and ResolveReferenceIn and ResolveEntityIn each wrap a
// rostered resolver with refusing arms of their own while returning what it
// answered on the accepting path.
//
// AdoptWorkstream sits under GroundCreates rather than under
// GroundLoadsByIdentifier. Its own doc comment says it creates a workstream at
// an identifier a card already names, and its body builds a Workstream and
// calls Save rather than reading one off disk, so the loading ground is false
// of it. An exemption resting on a false ground is the defect a closed set
// exists to prevent.
var exemptions = []Exemption{
	{Function: "LoadCard", Ground: GroundLoadsByIdentifier, Reason: "reads a card out of the directory its identifier names"},
	{Function: "LoadWorkstream", Ground: GroundLoadsByIdentifier, Reason: "reads a workstream out of the directory its identifier names"},
	{Function: "LoadAttachment", Ground: GroundLoadsByIdentifier, Reason: "reads an attachment out of the directory its identifier names"},
	{Function: "LoadItem", Ground: GroundLoadsByIdentifier, Reason: "reads a checklist item out of the directory its identifier names"},
	{Function: "readColumnIn", Ground: GroundLoadsByIdentifier, Reason: "reads one column's anchor out of the half it is given"},
	{Function: "readColumn", Ground: GroundLoadsByIdentifier, Reason: "reads one column's anchor out of the live half"},
	{Function: "loadCard", Ground: GroundLoadsByIdentifier, Reason: "reads a card's anchor off a directory the caller already resolved"},
	{Function: "loadRetiredCard", Ground: GroundLoadsByIdentifier, Reason: "reads a retired card's anchor off a directory the caller already resolved"},

	{Function: "NewColumn", Ground: GroundCreates, Reason: "mints a column from a title and a slug it is about to store"},
	{Function: "NewWorkstream", Ground: GroundCreates, Reason: "mints a workstream from a title and a slug it is about to store"},
	{Function: "AddComment", Ground: GroundCreates, Reason: "mints a comment from the body it is about to store"},
	{Function: "AddItem", Ground: GroundCreates, Reason: "mints a checklist item from the text it is about to store"},
	{Function: "AddAttachment", Ground: GroundCreates, Reason: "mints an attachment from the file it is about to copy in"},
	{Function: "ReplaceAttachment", Ground: GroundCreates, Reason: "writes new bytes under an attachment the caller already resolved"},
	{Function: "RenameAttachment", Ground: GroundCreates, Reason: "stores a new filename on an attachment the caller already resolved"},
	{Function: "AdoptWorkstream", Ground: GroundCreates, Reason: "creates a workstream at an identifier a card already names, building it in memory and saving it"},

	{Function: "Comments", Ground: GroundLists, Reason: "answers every comment below a card"},
	{Function: "Items", Ground: GroundLists, Reason: "answers every checklist item below a card"},
	{Function: "Attachments", Ground: GroundLists, Reason: "answers every attachment below a holder"},
	{Function: "cardsIn", Ground: GroundLists, Reason: "answers every card in a half of the workbench"},
	{Function: "workstreamsIn", Ground: GroundLists, Reason: "answers every workstream in a half of the workbench"},
	{Function: "cardsWith", Ground: GroundLists, Reason: "answers every card matching a condition"},
	{Function: "retiredCardsIn", Ground: GroundLists, Reason: "answers every retired card in a half of the workbench"},
	{Function: "BlockingItems", Ground: GroundLists, Reason: "answers every checklist item that blocks a claim"},
	{Function: "GatingItems", Ground: GroundLists, Reason: "answers every checklist item that gates a column"},
	{Function: "itemsWhere", Ground: GroundLists, Reason: "answers every checklist item matching a condition"},

	{Function: "ResolveCard", Ground: GroundDelegates, Reason: "one line, handing the reference to resolveCardIn"},
	{Function: "ResolveArchivedCard", Ground: GroundDelegates, Reason: "one line, handing the reference to resolveCardIn against the archived half"},
	{Function: "ResolveReference", Ground: GroundDelegates, Reason: "one line, handing the reference to ResolveReferenceIn"},
	{Function: "ResolveReferenceIn", Ground: GroundDelegates, Reason: "wraps resolveReferenceBody with refusing arms of its own and returns what it answered"},
	{Function: "ResolveEntity", Ground: GroundDelegates, Reason: "one line, handing the reference to ResolveEntityIn"},
	{Function: "ResolveEntityIn", Ground: GroundDelegates, Reason: "wraps the reference resolver with refusing arms of its own and returns what it answered"},
	{Function: "resolveBelow", Ground: GroundDelegates, Reason: "one line, handing the reference to resolveBelowLanding"},
}

// Exemptions returns every declared exemption, in the order the roster carries
// them.
func Exemptions() []Exemption {
	roster := make([]Exemption, len(exemptions))
	copy(roster, exemptions)
	return roster
}

// Grounds returns the closed set of exemption grounds.
func Grounds() []string {
	return []string{GroundLoadsByIdentifier, GroundCreates, GroundLists, GroundDelegates}
}
