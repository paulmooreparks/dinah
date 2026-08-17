# The core profile

Version identity: `dinah-core 2.0`, maturity channel `dev`.

## 1. Scope and audience

This profile specifies how a tool coordinates board-based work: how a
workbench declares its flow, what a card is, what it means for somebody to
hold a card, which acts on a card are legal and which are refused, what a
state tells whoever arrives at it, and what history a tool owes the people
who read it later.

The audience is somebody building a second tool. Everything needed to build
a conforming tool is in this document. No other document, product, source
tree or vendor is required reading, and the profile deliberately cites none
of them in a normative statement. A reader who has never used any existing
board tool can implement this profile from these pages.

The profile does not specify storage. It says nothing about files,
directories, databases, wire protocols, transports, user interfaces, or the
words a screen shows a reader in any particular language. Two conforming
tools may share no design decision below the model this document states, and
the profile is written on the assumption that they will not.

The profile also does not specify a method. It says how a card moves and who
may move it; it never says what work should happen at any position in the
flow. That is what each workbench's own instructions carry, and the profile's
job is to serve them faithfully at the moment they are needed.

The subject matter is deliberately domain-neutral. A workbench may plan a
wedding, resolve support requests, run a research programme, or organize a
harvest. Section 10.1 walks a wedding through every state, act and refusal
the profile defines, and that walkthrough is the standing demonstration that
nothing domain-specific has crept in.

### 1.1 What a layer is for (non-normative)

Concerns this profile excludes are not thereby forbidden. A tool that wants
version-control integration, capability routing, portfolio reporting or any
other specialization declares a layer, as section 9 describes, and section 10
records for every excluded concept the reason it stayed out and the condition
that would bring it in.

## 2. Version identity and compatibility

This document is version 1.0 of the profile whose identity string is
`dinah-core`. The version of this profile is a property of this document. It
is unrelated to the release numbering of any tool, and a tool's own version
number tells a reader nothing about which profile version that tool
implements.

The paragraph is non-normative. The identity string names the contract after
the first tool that implements it. The string rides inside a conformance
claim and inside the `profile` member of section 5.7.

[CORE-VER-1] A tool that claims conformance MUST name the major number and the minor number of the profile version it conforms to.

[CORE-VER-2] A conformance claim MUST NOT name a maturity channel.

[SUITE-CONF-1] A conformance claim MUST be evaluated over the tool-directed statements alone.

### 2.1 Maturity channels

A revision of this document sits on exactly one of three channels, and the
channel describes the document's own lifecycle rather than any promise to a
caller.

```
dev     The text moves freely. Statements are added, reworded and withdrawn
        without notice, and nothing below binds.
beta    Breaking changes are announced before they land. The classification
        rules in 2.2 are followed, and a reader is told when a major is
        coming.
stable  The compatibility promise binds in full. Every rule in 2.2 applies
        without exception.
```

This revision sits on `dev`. Promotion to `stable` is a named event, recorded
in the changelog like any other change, and the promise starts to bind at
that event. A conformance claim names `dinah-core 1.0` and says nothing about
the channel, because the channel belongs to the document's history and the
number belongs to the contract.

### 2.2 What an increment may carry

The unit of change is the extracted statement list, which section 3 defines
and section 11 indexes. Every increment is classified by comparing that list
across two revisions, so the version discipline is itself checkable by
machine.

A patch increment carries editorial change alone. A minor increment adds
statements, weakens the keyword of an existing statement, or does both. A
major increment retires a statement, strengthens one, or carries any change
the rules below cannot classify.

One thing outside the extracted list is carried by the same discipline. The
order in which section 6 states its checks decides which refusal name a
conforming tool reports, so a revision that changes that order takes a major
increment under DOC-ORDER-1 even though the extraction is unchanged. The
comparison stays mechanical, because the orders are published as fixed lists
and two revisions' lists compare as text.

Two rules make the classification mechanical rather than a judgement about
prose. First, the text of a published statement is frozen, so a change in
what a statement demands is made by retiring its identifier and publishing a
new one, which turns retirement and strengthening back into operations on a
set of identifiers. Second, the one edit permitted in place is a weakening of
the keyword along this order:

```
MUST     ->  SHOULD  ->  MAY
MUST NOT ->  SHOULD NOT
```

[DOC-VER-1] A patch increment MUST NOT change the extracted statement list.

[DOC-VER-2] A minor increment MUST leave every identifier published by the prior revision present in the list.

[DOC-VER-3] Retiring an identifier MUST take a major increment.

[DOC-VER-4] The text of a published statement MUST NOT change except by weakening its keyword along the order stated in section 2.2.

[DOC-VER-5] A revision whose difference from its predecessor cannot be classified by DOC-VER-1 through DOC-VER-4 MUST take a major increment.

[DOC-VER-6] An identifier retired by any revision MUST NOT be published again.

### 2.3 How a workbench declares what it targets

A workbench definition names the profile version it was written against, so a
tool meeting an unfamiliar workbench refuses it in one clear sentence instead
of misreading it quietly. The statements that carry this are CORE-BENCH-3 and
CORE-BENCH-4 in section 5.

### 2.4 The changelog

The changelog in section 12 is append-only.

[DOC-CHG-1] A published changelog entry MUST NOT be edited or removed.

[DOC-CHG-2] Each changelog entry MUST carry its version, its channel, its date, every identifier it affects marked as introduced, relaxed or retired, and its consequence for a caller in prose.

## 3. Notation

### 3.1 Keywords

The keyword vocabulary is the one RFC 2119 defines as amended by RFC 8174,
and only the uppercase forms carry meaning:

```
MUST, MUST NOT, SHOULD, SHOULD NOT, MAY
```

Lowercase spellings of those words are ordinary English throughout this
document and carry no requirement. A keyword is matched longest first, so the
two words of a negated keyword count once rather than twice.

### 3.2 The statement syntax

A normative statement occupies exactly one line. The line begins with an
opening square bracket at column one, carries the statement's identifier,
closes the bracket, then carries one space and the statement itself. Nothing
else appears on that line. An extractor reads the document line by line and
needs no Markdown parser:

```
^\[([A-Z][A-Z0-9]*(-[A-Z0-9]+)*)\] (.+)$
```

Capture group one is the identifier and capture group three is the statement.
An identifier is uppercase ASCII letters, digits and hyphens, and it names a
family and a number, as `CORE-CLAIM-3` does.

Every statement carries exactly one keyword. It carries an outcome somebody
can observe, which section 11 states for each identifier, and it is unique
in the document, so an extraction over a valid revision returns no
identifier twice.

Uppercase keywords appear nowhere else. A checker treats an uppercase keyword
found on a line that is neither a statement line nor inside a fenced block as
a defect in this document.

### 3.3 The four classes of statement

The profile constrains four different subjects, and conflating them would
make a conformance suite claim to test things it cannot reach.

A statement whose identifier begins with `CORE-` is tool-directed. Its
subject is a tool, and a conformance suite asserts it by driving a tool and
observing what comes back. Conformance to this profile is exactly the set of
these statements, per SUITE-CONF-1.

A statement whose identifier begins with `DOC-` is document-directed. Its
subject is this document and its revisions, and a checker asserts it by
comparing two published revisions. A tool cannot pass or fail one.

A statement whose identifier begins with `ACTOR-` is owner-directed. Its
subject is the conduct of whoever works a card. No tool can be made to fail
one, so these are excluded from conformance; each names an outcome observable
in a workbench's recorded history, which is what makes an owner's departure
from the agreement visible after the fact.

A statement whose identifier begins with `SUITE-` is suite-directed. Its
subject is whoever evaluates a conformance claim, and it is asserted by
reading a claim together with the statements the run exercised. Neither a
tool nor this document can pass or fail one. The class carries a single
statement in this revision, and it exists because the rule that says what
conformance is evaluated over cannot itself be evaluated by driving a tool.

### 3.4 Tokens

Words in `this typeface` are tokens. A token is machine vocabulary with a
fixed spelling, stored and transmitted as written here, and never translated.
The tokens this profile defines are the state kinds, the substates, the
outcome names, the refusal names, and the member names of the interchange
form in section 5.7.

### 3.5 The closed vocabulary and the excluded words

Section 4 is a closed list of the model's nouns. Every noun naming a part of
the model that a statement uses appears there, and a revision that introduces
such a noun without adding it to section 4 has a defect.

Three groups of nouns are outside that list on purpose, because each is
defined where it is introduced and repeating it in section 4 would put the
model's vocabulary and the document's own machinery in one bag. The first
group is this document's machinery: statement, identifier syntax, keyword,
class, extraction, increment, patch, changelog entry, conformance and
maturity channel, defined in sections 2 and 3. The second is the vocabulary
of the specifications this document cites: JSON's object, array, element,
member, string and number, as RFC 8259 defines them, and Unicode's encoding
and case terms, used in sections 5.6 and 5.7. The third is ordinary English,
which carries no special meaning here and needs no declaration. A reader who
finds a noun in a statement that is in none of these groups and not in
section 4 has found a defect worth reporting.

Two lists of words are excluded from three places, and a checker matches them
on whole words, without regard to case. The three places are every
normative statement, section 4, and the walkthrough in section 10.1. Prose
elsewhere may use these words where the subject is the thing the word
names.

The first list is the vocabulary of one trade, and its presence in a
statement would tell every workbench outside that trade that this profile was
not written for them:

```
repository, branch, commit, build, deploy, merge, pull request, code, bug,
ticket, sprint, backlog grooming, continuous integration, issue, test suite,
developer, engineer
```

One word is deliberately absent from that list. The word `release` names one
of this profile's four ratified acts, and the sense the list means to exclude
is a published version of a piece of software, which no statement below
carries. The removal is recorded here rather than left for a reader to infer.

The second list is the product vocabulary of tools that already implement
work like this one. These words carry meanings a reader of this profile has
no way to know, and a later revision that admits one would be importing a
model this document never states:

```
lane, gate, loop limit, column, station, swimlane, zone, persona,
capability tier, shopping queue, external wait, workstream
```

The boundary table of section 10 falls outside all three places, because
naming what stayed out is that table's whole function and it could not do its
job under either list. It carries its own discipline instead. Each row ruled
out is named by a neutral description of the concept, and where a familiar
word helps a reader who already knows one, it follows in brackets, so no
reader meets an undefined name standing alone.

## 4. Core vocabulary

**Workbench.** One coordinated body of work, carrying a flow of states and
the cards travelling it.

**Workbench definition.** The declaration of a workbench: its title, its
ordered states, and whatever a state carries.

**State.** One named position in a workbench's flow, which cards occupy.

**Kind.** The nature of a thing, drawn from a declared set. A state's kind is
one of three and says whether cards enter there, are worked there, or come to
rest there. A block's kind names the class of the obstacle and is drawn from
whatever set a workbench finds useful. A link's kind names what one card is
to another and is drawn from whatever set a workbench finds useful.

**Flow.** The ordered sequence of a workbench's states.

**Position.** Where a state stands in the flow, which is where it stands in
the workbench's ordered list of states.

**Card.** One unit of work, occupying exactly one state at a time.

**Title.** The short prose name of a workbench, a state or a card, which a
person reads and which may change without changing identity.

**Identifier.** The name by which a workbench, a state or a card is referred
to, unique in its context, which never changes.

**Field.** One named value carried on a card.

**Link.** A reference one card carries to another card of the same workbench,
naming a kind and the card it points to.

**Substate.** The condition of a card within its state, drawn from a closed
set of three, which says whether the card is waiting, being worked, or held
up.

**Owner.** Whoever acts on a workbench, named on every act.

**Operator.** The owner a workbench designates as answering for it, who alone
may take the acts this profile reserves.

**Holder.** The owner whose claim on a card is current.

**Claim.** The record that one owner has taken up a card and is working it.

**Expiry.** The lapse of a claim after a declared interval, which returns the
card to waiting.

**Queue.** The cards waiting in one state, in the order the profile defines.

**Verb.** One of the acts a tool offers on a card.

**Request.** What an owner sends to ask for a verb, carrying that owner's
name, whatever the verb needs, and any marker the act requires.

**Act.** One entry in a card's history, recording either a performance of a
verb or the expiry of a claim.

**Move.** The verb that carries a card from one state to another.

**Release.** The verb by which a holder gives up a claim.

**Block.** The verb that marks a card as held up, with a reason.

**Unblock.** The verb by which the operator returns a blocked card to
waiting.

**Reason.** The prose a block carries, saying what stands in the way.

**Override.** The operator's deliberate performance of an act a limit would
otherwise refuse, asked for by an override marker and recorded as such.

**Override marker.** The mark an operator puts on a request to say that the
act is meant to pass a limit that would otherwise refuse it.

**Capacity limit.** The greatest number of cards a state accepts.

**Count.** The number of cards a state holds, whatever their substate, which
is what a capacity limit is compared against.

**Instructions.** The prose a workbench or a state carries for whoever
arrives.

**Outcome.** What a tool reports back from a verb, drawn from a closed set of
four.

**Refusal name.** The token that says which rule refused an act.

**Basis.** The revision of a card an owner had read before asking for a
change.

**Revision.** The token by which a tool distinguishes one version of a card
from another.

**History.** The recorded acts of a card, in the order they were recorded.

**Layer.** A body of declarations and behaviour outside this profile, named
so it cannot collide with it.

**Interchange form.** The one serialization of a workbench definition this
profile defines, by which two tools exchange one.

**Profile version.** The major and minor numbers naming a revision of this
document.

**Major number.** The first of the two numbers of a profile version, which
changes when a revision retires or strengthens a statement.

**Minor number.** The second of the two numbers of a profile version, which
changes when a revision adds a statement or weakens one.

**Maturity channel.** The lifecycle stage of a revision of this document.

## 5. The model

The model is stated as concepts. Nothing here says how a tool keeps any of it.

### 5.1 Workbenches

A workbench is one coordinated body of work with one flow. Everything else in
the model belongs to exactly one workbench, and this profile says nothing
about how one workbench relates to another.

[CORE-BENCH-1] A workbench definition MUST carry a title.

[CORE-BENCH-2] A workbench definition MUST carry an ordered list of one or more states.

[CORE-BENCH-3] A workbench definition MUST declare the major number and the minor number of the profile version it targets.

[CORE-BENCH-4] A tool MUST refuse to act on a workbench definition whose declared major number it does not implement, reporting the refusal name `unsupported-version`.

### 5.2 States

A state is a named position in the flow. Its position in the workbench's
ordered list is its position in the flow, so reordering the flow is
reordering that list, and a state carries no position of its own.

A state's kind says what happens there. Cards enter a workbench at a state of
kind `intake`. Work happens at a state of kind `work`. A card that has
finished its journey comes to rest at a state of kind `done`, and such a
state is terminal.

A state may be operator-owned, which reserves departure from it to the
operator. A workbench uses this where a person has to look at the work before
it goes on, and it is the mechanism behind ACTOR-4.

[CORE-STATE-1] Each state MUST carry an identifier unique within its workbench.

[CORE-STATE-2] Each state MUST carry a title.

[CORE-STATE-3] Each state MUST carry exactly one kind from `intake`, `work` and `done`.

[CORE-STATE-4] A state MAY declare itself operator-owned.

[CORE-STATE-5] A state MAY declare a capacity limit, which is a whole number greater than zero.

[CORE-STATE-6] A tool MUST treat a state's position in its workbench's ordered list as that state's position in the flow.

[CORE-STATE-7] A tool MUST offer, as the forward move from a state, a move to the next state in the workbench's ordered list.

[CORE-STATE-8] A tool MAY offer a move to a state other than the next one in the workbench's ordered list.

[CORE-STATE-9] A tool MUST NOT offer a forward move out of a state whose kind is `done`.

### 5.3 Cards

A card is one unit of work. It occupies exactly one state and carries exactly
one substate, and those two values together are the whole of its position.

The substates are `ready`, `active` and `blocked`. A card that is `ready` is
waiting and may be taken up. A card that is `active` is being worked by its
holder. A card that is `blocked` is held up for a reason, and only the
operator lifts that.

The profile requires four things of a card and no more: an identifier, a
title, a state, and a substate. A card may carry anything else a workbench
needs, and a tool that does not know what a field means keeps it anyway,
and that is what lets one tool hand a workbench to another without loss.

[CORE-CARD-1] Every card MUST carry an identifier unique within its workbench.

[CORE-CARD-2] A card's identifier MUST NOT change when the card's title changes.

[CORE-CARD-3] Every card MUST carry a title.

[CORE-CARD-4] Every card MUST carry exactly one state identifier that resolves to a state its workbench declares.

[CORE-CARD-5] Every card MUST carry exactly one substate from `ready`, `active` and `blocked`.

[CORE-CARD-6] A card whose substate is `active` MUST carry its holder and the time the claim began.

[CORE-CARD-7] A card whose substate is not `active` MUST NOT carry a holder.

[CORE-CARD-8] A card MAY carry fields this profile does not define.

[CORE-CARD-9] A tool MUST preserve the fields it does not recognize on a card it has read and written back.

### 5.4 Owners

Every act names the owner who took it. An owner is whoever acts, whether that
is a person, an automated agent, or a person's assistant acting on somebody
else's stated decision. The profile treats an owner's name as attribution.
Whether a tool also demands evidence that the name belongs to whoever
presented it is a question of deployment, and a tool serving one person
answers it differently from a tool serving a hundred.

One thing about owners is not optional. Every workbench designates an
operator, and several statements reserve acts to that owner, so a tool has to
be able to answer whether a given owner is the operator of a given workbench.
How a workbench designates one is outside this profile, but that it does so
is not, because a workbench with no operator has blocks nobody can lift and
operator-owned states nobody can leave. A tool refuses such a workbench
rather than serving one whose two reserved acts are dead.

[CORE-OWNER-1] Every recorded act MUST name the owner it is attributed to.

[CORE-OWNER-2] A tool MUST be able to report whether a given owner is the operator of a given workbench.

[CORE-OWNER-3] A tool MUST refuse to act on a workbench that designates no operator, reporting the refusal name `no-operator`.

[CORE-VERB-1] A tool MUST refuse a verb naming a card its workbench does not carry, reporting the refusal name `unknown-card`.

[CORE-VERB-2] A tool MUST refuse a verb naming no owner, reporting the refusal name `no-owner`.

### 5.5 Queues

The queue of a state is the cards waiting there. Two tools reading one
workbench have to agree on which card is next, or a workbench handed from one
to the other reorders itself for no reason a reader can see, so the profile
fixes one order and lets a tool offer others beside it.

The fixed order is arrival: the card that entered the state earliest comes
first. Ties are broken by ascending creation ordinal, which makes the order
total. The order deliberately consults nothing about the card except when it
arrived, because every richer ordering would import a ranking the profile does
not define.

[CORE-QUEUE-3] The next card of a state MUST be the card in that state whose substate is `ready` that entered the state earliest, with ties broken by ascending creation ordinal.

[CORE-QUEUE-4] A tool MAY offer orders beside the one CORE-QUEUE-3 defines, provided the order CORE-QUEUE-3 defines remains available.

### 5.6 Text

Prose in a workbench is whatever language its people write. Tokens are not
language, and a tool that translated one on a machine-readable surface would
break every other tool reading it.

[CORE-TEXT-1] Every text this profile defines MUST be encoded in UTF-8.

[CORE-TEXT-2] A tool MUST NOT apply locale-dependent case rules to an identifier or a token.

[CORE-TEXT-3] A tool MUST NOT translate a token on a machine-readable surface.

[CORE-TEXT-4] A tool MAY render a token in the reader's own language where a person reads it.

### 5.7 The interchange form

A workbench definition that cannot leave the tool that holds it is a
workbench nobody can carry anywhere, so the profile defines exactly one
serialization for it. The interchange form is a means of exchange and not a
storage mandate. A tool keeps its workbench definitions however it likes, and
what it owes is the ability to produce this form and to consume one.

The member names below are tokens. They are excluded from the closed
vocabulary rule of section 3.5, which governs the nouns of the model, and
they are governed instead as tokens under section 3.4. The words object,
array, element, member, string and number in this subsection are JSON's own,
with the meanings RFC 8259 gives them.

```json
{
  "profile": "dinah-core/1.0",
  "title": "Wedding",
  "states": [
    { "id": "s1", "title": "Ideas",   "kind": "intake" },
    { "id": "s2", "title": "Deciding", "kind": "work",
      "instructions": "Pick one and say why.", "capacity": 3 },
    { "id": "s3", "title": "Booked",  "kind": "done" }
  ]
}
```

[CORE-JSON-1] A tool MUST be able to read and to write the interchange form of a workbench definition.

[CORE-JSON-2] The interchange form MUST be one JSON object encoded in UTF-8.

[CORE-JSON-3] The interchange object MUST carry the members `profile`, `title` and `states`.

[CORE-JSON-4] The member `states` MUST be a JSON array whose element order is the order of the flow.

[CORE-JSON-5] Each element of `states` MUST be a JSON object carrying the members `id`, `title` and `kind`.

[CORE-JSON-6] A state object MAY carry the members `instructions`, `operator_owned` and `capacity`.

[CORE-JSON-7] A tool MUST preserve the members it does not recognize in an interchange object it has read and written back.

[CORE-JSON-8] A tool MAY hold a workbench definition in any form, provided it can produce the interchange form of that definition on request.

### 5.8 Links between cards

A card sometimes has something to say about another card. One card repeats work
another card already covers, one came out of another, one bears on another. A
tool that cannot record that leaves its owners to write the other card's
identifier into a piece of prose, where a second tool reading the same
workbench finds text rather than a reference.

A link is that record and nothing more. It names a kind and the card it points
to, and the card it is recorded on is the card that carries it. The profile
attaches no behaviour to one. No verb consults a link, no queue orders itself
differently because a card carries one, and no refusal this profile declares
follows from one. What a workbench does about a link is for the people reading
it, or for a layer that declares itself and says so under its own name.

The kind is an open value on the same ground as a block's kind. Nothing in the
contract enforces a meaning for it, so fixing the spellings here would bind
every workbench to a vocabulary this document cannot justify. Two tools
exchanging a workbench exchange the kinds they find and leave them alone.

The card a link names is a card of the same workbench. The profile is scoped to
one workbench throughout, and a reference reaching outside it would be the
first thing in the model that resolves nowhere. A link naming a card the
workbench does not carry is refused under the name the profile already uses
when a verb names one.

A card offered carrying a link with no kind and naming a card the workbench
does not carry fails two checks at once. The tiers of section 6.1 order them,
since what a thing names has to exist before anything is evaluated against it,
so `unknown-card` is reported and `malformed` is not.

[CORE-LINK-1] A card MAY carry links.

[CORE-LINK-2] Every link MUST carry a kind and the identifier of the card it names.

[CORE-LINK-3] A tool MUST refuse a link offered to it naming a card its workbench does not carry, reporting the refusal name `unknown-card`.

[CORE-LINK-4] A tool MUST NOT restrict a link's kind to a closed set of values.

[CORE-LINK-5] A tool MUST NOT report a refusal name this profile declares for a claim, a move, a release, a block or an unblock refused on the ground of a link.

[CORE-LINK-6] A tool MUST NOT add a link to a card as a consequence of a link another card carries.

## 6. The verbs

Every verb reports an outcome, and the outcomes are kept apart deliberately.
A caller driving a workbench without a person watching has to tell the
difference between a rule saying no, its own knowledge being out of date, and
the question never having been asked, because the three call for three
different next moves.

### 6.1 Outcomes and refusals

[CORE-OUT-1] A tool MUST report the outcome of every verb as exactly one of `ok`, `refused`, `stale` and `unreachable`.

[CORE-OUT-2] An outcome of `refused` MUST carry exactly one refusal name.

[CORE-OUT-3] A refusal name MUST be one this profile declares or one carrying a layer's prefix.

[CORE-OUT-4] A tool MUST report `unreachable` when it cannot reach whatever answers for the workbench.

[CORE-OUT-5] A tool MUST report the refusal name `malformed` when it refuses a workbench definition, a state, a card or an interchange object for want of something this profile requires it to carry and no more particular refusal name this profile declares applies.

The refusal names this profile declares are these, and each is carried by the
statement that names it:

```
unknown-card, unknown-state, unsupported-version, held, not-requester,
blocked, not-blocked, not-holder, at-capacity, not-operator,
no-operator, no-owner, no-reason, terminal, malformed, layer-collision
```

One of the sixteen is general where the others are particular. Something
offered to a tool without what section 5 requires it to carry is refused as
`malformed`, because a separate name for each missing title and each absent
member would leave a caller holding a dozen names it cannot act on
differently. Where a more particular name fits, that name is reported
instead, so a card naming a state its workbench does not declare is
`unknown-state` rather than `malformed`.

Two refusals belong to the workbench rather than to any verb, and they are
evaluated before the verb's own list. CORE-BENCH-4 refuses a workbench whose
declared major number the tool does not implement, and CORE-OWNER-3 refuses a
workbench that designates no operator. A tool that cannot read the definition
cannot evaluate anything a verb requires of it, and a workbench with nobody
answering for it cannot be acted on whatever the verb is, so both sit ahead
of every list in 6.3 to 6.7:

```
1  the workbench declares a major number the tool implements   unsupported-version
2  the workbench designates an operator                        no-operator
```

Each verb's list is derived rather than composed. Five tiers order the
checks: what the request names has to exist before anything can be evaluated
against it, then the request has to carry what the verb requires it to carry,
then whoever asks has to be entitled to ask, then the card has to be in a
condition the verb accepts, and last the destination has to be in a condition
it accepts. Where two checks compete, the earlier tier decides. A fault in
the request is reported ahead of a condition of the card, because a caller
refused for a full destination may wait and ask again, while a caller whose
request was never legal would meet a second refusal for a reason that was
already true. The basis sits between the card's existence and the rest of the
verb's list, as section 6.2 states. The lists in 6.3 to 6.7 govern, and the
tiers say how each was built and how a verb added by a later revision
acquires its order.

[CORE-OUT-6] An outcome of `refused` MUST carry the refusal name of the first unsatisfied check in the order section 6 declares, which places the two checks preceding every verb ahead of the preconditions the verb's own subsection states.

[DOC-ORDER-1] A revision that changes the order in which section 6 states a check MUST take a major increment.

The order settles which name comes back when several checks fail together,
and it settles nothing about which name fits a single failure. A refusal that
arises from reading a workbench definition, a card or an interchange object
is named by whichever statement carries it, and CORE-OUT-5 supplies
`malformed` only where no more particular name does.

### 6.2 The basis

An owner reads a card, decides something, and asks for a change, and between
the reading and the asking the card may have moved. The basis is the owner's
statement of what it read, and it is request metadata with no home on the
card.

[CORE-BASIS-1] A request for a verb that changes a card MAY carry a basis naming the revision of the card its owner read.

[CORE-BASIS-2] A tool MUST report `stale` for a request whose basis does not name the card's current revision.

[CORE-BASIS-3] A tool MUST NOT hold a basis on the card it names.

[CORE-BASIS-4] A tool MUST carry the card's current revision in an outcome it reports as `stale`.

A request may carry an out-of-date basis and fail a check as well. The basis
is compared once the card has been found and before the rest of the verb's
list, so an owner that decided on a card it has not read as that card now
stands is told to read it again rather than told about a card it never saw.
The revision CORE-BASIS-4 carries in the `stale` outcome is what it reads
against.

[CORE-BASIS-5] A tool MUST report `stale` for a request whose basis does not name the card's current revision even where a precondition stated by the verb's own subsection, other than the card's existence, is also unsatisfied.

### 6.3 Claim

Claiming is how an owner takes up a waiting card and says so where everybody
can see it. A claim is a lease rather than a permanent assignment. A tool may
give a claim an expiry, and if it does, a lapse returns the card to waiting
in the open, with the lapse recorded, so nobody discovers weeks later that a
card was quietly reassigned.

A claim is also where the profile's pull discipline lives. Work here is
taken, never handed out: the owner that asks for a claim is the owner the
claim names, and no owner assigns a card to another. This is what makes the
flow pull rather than push, and it binds every conforming tool whether or not
any state in the workbench declares a capacity limit. A limit governs how
much work a state accepts; this rule governs who decides that a piece of it
begins. A tool wanting to point an owner at a card offers that owner the
card, and the owner claims it or does not.

```
1  the card exists                                  unknown-card
2  the request names an owner                       no-owner
3  the owner named as holder is the owner asking    not-requester
4  the card's substate is not `blocked`             blocked
5  the card's substate is not `active`              held
```

Effect: the substate becomes `active`, and the card carries its holder and
the time.

[CORE-CLAIM-1] A tool MUST refuse a claim on a card whose substate is `blocked`, reporting the refusal name `blocked`.

[CORE-CLAIM-2] A tool MUST refuse a claim on a card whose substate is `active`, reporting the refusal name `held`.

[CORE-CLAIM-3] A claim that succeeds MUST set the card's substate to `active` and record its holder and the time the claim began.

[CORE-CLAIM-4] A claim MAY carry an expiry.

[CORE-CLAIM-5] A tool MUST set a card's substate to `ready` and record the expiry when a claim on it expires.

[CORE-CLAIM-6] A tool MUST NOT change a card's holder except through release, expiry or block.

[CORE-CLAIM-7] A tool MUST refuse a claim naming as holder an owner other than the one asking for it, reporting the refusal name `not-requester`.

### 6.4 Move

Moving carries a card from one state to another. A move changes where the
card is and nothing else about it, which settles a question two
implementations would otherwise answer opposite ways: a holder who moves a
card still holds it afterwards, and a waiting card that is moved is still
waiting.

```
1  the card exists                                      unknown-card
2  the destination is a state the workbench declares    unknown-state
3  an override marker, if carried, is the operator's    not-operator
4  the departure is legal for whoever asks              not-operator
5  the card's substate is not `blocked`                 blocked
6  the card is unheld or held by whoever asks           held
7  the move is not a forward move out of a `done` state terminal
8  the destination is below its capacity limit          at-capacity
```

Effect: the card's state becomes the destination.

Capacity is how a state says how much work it will hold. A card enters a
state because that state has room, so a state whose limit is reached stops
accepting work no matter how much is waiting upstream. The count is every
card in the state whatever its substate, because a blocked card still
occupies the place, and exempting it would turn blocking into a way of hiding
an overloaded state. The pull discipline itself does not rest on this
mechanism, which a workbench may decline to use; it rests on the claim, as
section 6.3 states.

A limit nobody can ever set aside gets worked around outside the tool, where
nothing sees it, so the profile gives the operator a way through it and makes
that way visible. The operator marks the request as an override, and the act
that results is recorded as an override rather than passing as an ordinary
move. The marker is the whole of the exception: a request that does not carry
one is refused at a full state however senior whoever asks, and a request
that carries one from an owner who is not the operator is refused as well. A
tool is free to offer no override at all, in which case a full state refuses
every entry.

[CORE-MOVE-1] A tool MUST refuse a move to a state the workbench does not declare, reporting the refusal name `unknown-state`.

[CORE-MOVE-2] A tool MUST refuse a move of a card whose substate is `blocked`, reporting the refusal name `blocked`.

[CORE-MOVE-3] A tool MUST refuse a move of a card held by an owner other than the one asking, reporting the refusal name `held`.

[CORE-MOVE-4] A tool MUST refuse a move into a state whose count of cards has reached that state's declared capacity limit, reporting the refusal name `at-capacity`, except where CORE-MOVE-9 permits the move.

[CORE-MOVE-5] The count CORE-MOVE-4 compares MUST include every card in the state whatever its substate.

[CORE-MOVE-6] A tool MUST refuse a move out of an operator-owned state asked for by an owner that is not the operator, reporting the refusal name `not-operator`.

[CORE-MOVE-7] A tool MUST refuse a forward move out of a state whose kind is `done`, reporting the refusal name `terminal`.

[CORE-MOVE-8] A move MUST NOT change a card's substate or its holder.

[CORE-MOVE-9] A tool MAY admit a move into a state that has reached its capacity limit when the request carries an override marker and the owner asking is the operator of the workbench.

[CORE-MOVE-10] A tool that admits a move under CORE-MOVE-9 MUST record that move as one act marked an override.

[CORE-MOVE-11] A tool MUST refuse a request carrying an override marker from an owner that is not the operator, reporting the refusal name `not-operator`.

### 6.5 Release

Releasing gives a card back. An owner that has stopped working a card
releases it, so that the queue is honest about what is available and nobody
waits on somebody who has walked away.

```
1  the card exists                 unknown-card
2  whoever asks holds the card     not-holder
```

Effect: the substate becomes `ready` and the holder is removed.

[CORE-RELEASE-1] A tool MUST refuse a release asked for by an owner that does not hold the card, reporting the refusal name `not-holder`.

[CORE-RELEASE-2] A release that succeeds MUST set the card's substate to `ready` and remove the card's holder.

### 6.6 Block

Blocking says that this card cannot go on, and why. The reason is prose
because the obstacles that stop real work are various and a closed list of
them would send whoever hits an unlisted one looking for the nearest wrong
answer. A tool may offer a kind beside the reason for the sake of counting
obstacles by class, and the kind never replaces the reason.

A block clears the claim. Somebody who cannot proceed should not go on
holding the card, and freeing it is what makes the obstacle visible as an
obstacle rather than as an owner being slow.

Because a block clears the claim, it is an act on somebody else's work
whenever the card is held by another owner, and the profile treats it exactly
as it treats a move of a held card. A holder blocks the card it holds, and an
owner blocks a card nobody holds, but neither strips another owner's claim by
raising an obstacle on their behalf.

```
1  the card exists                             unknown-card
2  the request names an owner                  no-owner
3  the request carries a reason                no-reason
4  the card is unheld or held by whoever asks  held
```

Effect: the substate becomes `blocked`, the card carries the reason, and the
holder is removed.

[CORE-BLOCK-1] A block MUST carry a reason in prose.

[CORE-BLOCK-2] A tool MUST refuse a block carrying no reason, reporting the refusal name `no-reason`.

[CORE-BLOCK-3] A block that succeeds MUST set the card's substate to `blocked` and remove the card's holder.

[CORE-BLOCK-4] A block MAY carry a kind naming the class of the obstacle.

[CORE-BLOCK-5] A tool MUST NOT restrict a block's reason to a closed set of values.

[CORE-BLOCK-6] A tool MUST refuse a block on a card held by an owner other than the one asking, reporting the refusal name `held`.

### 6.7 Unblock

A block with no way out is a one-way door, so the profile defines the verb
that lifts one and reserves it to the operator. The obstacle that stopped the
work was, by the act of blocking, handed to whoever answers for the
workbench, and an owner that could block and unblock at will would be pausing
its own work privately rather than raising an obstacle anybody else has to
see.

An unblock asked of a card that is not blocked is refused rather than
granted quietly. The verb lifts an obstacle, and a card carrying no
obstacle has nothing to lift, so a tool reporting success would tell its
caller the request had an effect the card never received. The caller most
likely to ask is an automated one working from knowledge that has gone out
of date, and a refusal tells it what it got wrong where a success would
hide it. The condition is evaluated after the one naming the operator, so
an owner that is not the operator is refused `not-operator` whatever the
card's substate.

```
1  the card exists                     unknown-card
2  whoever asks is the operator        not-operator
3  the card's substate is `blocked`    not-blocked
```

Effect: the substate becomes `ready`.

[CORE-UNBLOCK-1] A tool MUST offer a verb that sets a card whose substate is `blocked` to substate `ready`.

[CORE-UNBLOCK-2] A tool MUST refuse an unblock asked for by an owner that is not the operator, reporting the refusal name `not-operator`.

[CORE-UNBLOCK-3] A tool MUST NOT set a card's substate away from `blocked` as a consequence of any verb other than unblock.

[CORE-UNBLOCK-4] A tool MUST refuse an unblock of a card whose substate is not `blocked`, reporting the refusal name `not-blocked`.

### 6.8 History

History is what makes a workbench answerable afterwards. The recorded acts
say who did what and when, in the order it happened, and nothing rewrites
them.

An act carries the names of whatever it refers to as they stood at the time.
A move records the title of the state it left as well as that state's
identifier, so the history still reads years later when the state has been
renamed or removed. The names in history are a snapshot, and a tool reading
history back never resolves them against the workbench as it stands now.

Two entries in history are not performances of a verb, and the profile says
what each carries rather than leaving an implementer to invent it. An expiry
is a lapse rather than an act somebody took, and it is attributed to the
owner whose claim lapsed, which is the only owner it concerns. An override is
not a separate entry at all: it is a mark on the move it permitted, so an
overridden move produces one act and not two.

[CORE-HIST-1] A tool MUST record every claim, move, release, block and unblock as an act carrying the time, the owner and the name of the verb.

[CORE-HIST-2] A tool MUST record the expiry of a claim as an act carrying the time and, as the owner it is attributed to, the owner whose claim lapsed.

[CORE-HIST-3] A tool MUST NOT alter or remove a recorded act.

[CORE-HIST-4] A recorded move MUST carry the identifier and the title, as they stood at the time of the move, of the state left and the state entered.

[CORE-HIST-5] A tool MUST present a card's recorded acts in the order they were recorded.

[CORE-HIST-6] A tool MUST NOT resolve an identifier carried in a recorded act against the workbench's present contents when it presents that act.

## 7. Instruction serving

A workbench carries its method in prose, and the method reaches whoever needs
it at the moment they need it. Serving is what turns a workbench definition
from a diagram into working guidance.

The workbench carries standing instructions that apply wherever a card is,
and each state carries the instructions of that position. They are served
together, most general first, and they are never copied into one another. A tool that wrote the workbench's standing text into
each state would freeze a copy that stops tracking its source, and every
later edit would then reach some readers and not others.

Instructions are served at the two moments an owner's situation changes: when
a claim succeeds, and when a move succeeds. Alongside them a tool says which
moves are legal for that card at that moment, so an owner is never left
guessing which departures the workbench allows.

[CORE-INSTR-1] A state MAY carry instructions in prose.

[CORE-INSTR-2] A workbench MAY carry standing instructions in prose.

[CORE-INSTR-3] A tool MUST serve the instructions of a card's state to the owner whose claim on that card has just succeeded.

[CORE-INSTR-4] A tool MUST serve the instructions of the state entered to the owner whose move has just succeeded.

[CORE-INSTR-5] A tool MUST serve a workbench's standing instructions ahead of the state's instructions.

[CORE-INSTR-6] A tool MUST NOT write the text of one instruction layer into another.

[CORE-INSTR-7] A tool MUST carry, with served instructions, the moves that are legal for that card at that moment.

## 8. The working agreement

The four rules below bind an owner rather than a tool. They are the
discipline that makes a shared workbench trustworthy, and no tool can be made
to fail one, so section 3.3 puts them outside conformance. Each
still names something a reader can check in a workbench's history after the
fact, so an owner's departure from the agreement leaves a trace.

[ACTOR-1] An owner MUST claim a card before producing work on it.

[ACTOR-2] An owner MUST NOT hold a claim on a card it has stopped working.

[ACTOR-3] An owner MUST treat the workbench as the authority for where a card stands and who holds it.

[ACTOR-4] An owner that is not the operator MUST NOT move a card out of an operator-owned state.

The tool-directed statements that make these rules observable are CORE-CLAIM-3
for the first, CORE-RELEASE-2 and CORE-CLAIM-5 for the second, CORE-HIST-1
and CORE-INSTR-7 for the third, and CORE-MOVE-6 for the fourth.

## 9. Layers

A layer is how a tool adds what this profile leaves out without pretending
its additions are part of the contract. A layer declares itself in the
workbench definition under a dotted name, and the profile's own names never
contain a dot, so a layer's name can never collide with a name a later
revision of this profile introduces.

Nothing in this profile requires a layer. A tool that implements no layer at
all conforms, and a workbench that declares a layer a tool has never heard of
is still readable by that tool, which keeps the declaration and leaves it
alone.

No layer content ships in this revision. Section 10 records what a layer
would be for.

[CORE-LAYER-1] A layer MUST declare itself in the workbench definition under a name containing a full stop.

[DOC-LAYER-1] A name this profile defines MUST NOT contain a full stop.

[DOC-LAYER-2] A tool-directed statement of this profile MUST NOT require a layer.

[CORE-LAYER-2] A tool MUST preserve the content of a declared layer it does not understand.

[CORE-LAYER-3] A tool MUST refuse a workbench definition whose layer declaration reuses a name this profile defines, reporting the refusal name `layer-collision`.

## 10. The boundary

The table below is this document's central artifact. Every concept the
profile uses appears here ruled in, and every concept considered and left out
appears here ruled out with the condition that would bring it back. An
exclusion recorded without a reason cannot be told apart from an oversight,
and an inclusion recorded without a reason cannot be argued with, so both
carry one.

One item means one concept. Where a familiar phrase bundles two concepts that
could be ruled differently, the two are separate rows, so a capacity limit
on a state and a limit on who may take up a card are counted apart.

Rows ruled in are named in the vocabulary of section 4, and rows ruled out
are named by a neutral description of the concept, with a familiar word
following in brackets where one exists.

The last column binds the table to the rest of the document. A row ruled in
names every statement that carries the concept, and each statement of this
profile appears against exactly one row, so the claim that no concept enters
the core undecided is a set comparison rather than a reading. A row ruled out
names no statement, since nothing normative may rest on a concept the profile
excluded, and carries a reopen condition instead. A checker asserts all four
of those properties, so a later revision that adds a statement without ruling
its concept, or an exclusion without a way back in, fails rather than passes
quietly.

| Item | Ruling | Reason | Reopens when | Statements |
| --- | --- | --- | --- | --- |
| The workbench definition | in | Without a declared flow there is nothing for two tools to agree about, and every other concept hangs off it. | | CORE-BENCH-1, CORE-BENCH-2 |
| States and the moves between them | in | The flow is the thing a board is. A tool that could not say where a card stands would not be coordinating anything. | | CORE-STATE-1, CORE-STATE-2, CORE-STATE-6, CORE-STATE-7, CORE-STATE-8, CORE-MOVE-1, CORE-MOVE-8 |
| Per-state instruction serving, with the legal moves alongside | in | A workbench carries its method in prose, and the method is worthless if it does not reach whoever arrives at the position it describes. Saying which moves are legal at the same moment is part of the same service, since an owner told what to do and not where it may go is told half of it. | | CORE-INSTR-1, CORE-INSTR-2, CORE-INSTR-3, CORE-INSTR-4, CORE-INSTR-5, CORE-INSTR-6, CORE-INSTR-7 |
| The verbs claim, move, release and block | in | These four are the whole of what an owner does to a card, and each has refusals a second tool would otherwise invent differently. | | CORE-VERB-1, CORE-VERB-2, CORE-CLAIM-3, CORE-RELEASE-2, CORE-BLOCK-3 |
| The four-rule working agreement | in | The discipline is what makes a shared workbench trustworthy, and stating it in the contract keeps it from being reinvented per tool. | | ACTOR-1, ACTOR-2, ACTOR-3, ACTOR-4 |
| Card identity and required fields | in | A card handed between tools has to survive the trip, and the four required fields are the fewest that keep its position meaningful. | | CORE-CARD-1, CORE-CARD-2, CORE-CARD-3, CORE-CARD-4, CORE-CARD-8, CORE-CARD-9 |
| The substate, and the claim's dependence on it | in | Waiting and being worked are different situations, and a claim that ignored the difference would let two owners take up one card. | | CORE-CARD-5, CORE-CARD-6, CORE-CARD-7, CORE-CLAIM-1, CORE-CLAIM-6, CORE-MOVE-2 |
| The pull invariant | in | Work here is taken and never handed out, which is what makes a flow pull rather than push. The invariant is stated as a rule about agency rather than about capacity, because a rule about capacity binds only the workbenches that declare a limit, and a tool declaring none would otherwise conform while pushing work at people. CORE-CLAIM-7 carries it: the owner that asks is the owner the claim names, so nobody assigns a card to anybody else. The limit below is the capacity layer built on top of that, not the invariant itself. | | CORE-CLAIM-7 |
| The unblock verb, reserved to the operator | in | A block with no defined lift is a one-way door, and reserving the lift is what keeps a block from becoming a private pause the blocker alone can end. The verb's answer when there is nothing to lift belongs to the same row, because a caller that cannot tell a lift from a request that changed nothing cannot drive the verb without watching it. | | CORE-UNBLOCK-1, CORE-UNBLOCK-2, CORE-UNBLOCK-3, CORE-UNBLOCK-4 |
| The reason on a block, as free prose | in | The obstacles that stop real work are various, and a closed list would send whoever hits an unlisted one to the nearest wrong answer. | | CORE-BLOCK-1, CORE-BLOCK-2, CORE-BLOCK-5 |
| The kind on a block, as an open value | in | Counting obstacles by class is worth having, and leaving the values open costs nothing because no rule hangs on them. | | CORE-BLOCK-4 |
| The owner and operator identity model | in | Every act names who took it, and several rules turn on whether that owner is the operator, so the concept cannot be deferred, and a workbench that designates none has reserved acts nobody can take. Whether a name is proved is left to deployment, which is what lets one tool serve one person and another serve many. | | CORE-OWNER-1, CORE-OWNER-2, CORE-OWNER-3 |
| Recorded history | in | A workbench that cannot say who did what is not answerable, and the append-only rule is what makes the record worth reading. | | CORE-HIST-1, CORE-HIST-3, CORE-HIST-5 |
| Self-contained references in history | in | History that resolved its names against the present would turn ordinary renaming into apparent corruption. | | CORE-HIST-4, CORE-HIST-6 |
| State kinds | in | Where cards enter, where they are worked and where they come to rest are three different situations, and a tool has to tell them apart to know what to offer. | | CORE-STATE-3 |
| Terminal states | in | Somewhere the journey ends, and a tool that offered a forward move out of the end would be inviting a card into nowhere. | | CORE-STATE-9, CORE-MOVE-7 |
| States a workbench reserves to its operator | in | Some positions exist so that a person looks at the work before it goes on, and a flow that could not express one would push that check outside the tool where nothing records it. | | CORE-STATE-4, CORE-MOVE-6 |
| The capacity limit on a state | in | This is the capacity layer of the discipline, and it is what stops a state accepting more work than it can hold. It is optional per workbench, which is why the pull invariant above is stated separately rather than resting on it. | | CORE-STATE-5, CORE-MOVE-4, CORE-MOVE-5 |
| The operator's override of a capacity limit, recorded as an override | in | A limit nobody can ever set aside gets worked around outside the tool, where nothing sees it. Requiring an explicit marker from the operator, and recording the resulting act as an override, keeps the exception inside the record. | | CORE-MOVE-9, CORE-MOVE-10, CORE-MOVE-11 |
| The closed set of refusal names | in | A caller that cannot tell one refusal from another cannot decide what to do next, and a refusal reported as prose is a refusal nobody can act on. | | CORE-OUT-2, CORE-OUT-3, CORE-OUT-5 |
| The order in which a tool evaluates its checks | in | Several mandatory refusals can apply to one request, and a caller that cannot predict which name comes back cannot decide what to do with the name it gets. The order fixes which name that is, and DOC-ORDER-1 puts the order under the same version discipline as the statements, so no later revision changes a tool's answer by rearranging a list. | | CORE-OUT-6, DOC-ORDER-1 |
| The behaviour when an owner acts on a card another owner holds | in | This is the one place where two owners collide, and leaving it undefined would let one tool queue the second owner's request and another grant it. Claim, move, release and block are ruled together, because a block clears the claim and would otherwise be the way around the rule. | | CORE-CLAIM-2, CORE-MOVE-3, CORE-RELEASE-1, CORE-BLOCK-6 |
| The layer declaration mechanism | in | Section 9 is required structure, and a profile that excluded concerns without saying how to add them back would be telling implementers to fork it. | | CORE-LAYER-1, CORE-LAYER-2, CORE-LAYER-3, DOC-LAYER-1, DOC-LAYER-2 |
| The profile version a workbench targets | in | A tool meeting a workbench from a future revision has to refuse it in one clear sentence rather than misread it quietly. | | CORE-BENCH-3, CORE-BENCH-4 |
| The conformance claim and what it is evaluated over | in | A claim nobody can check is a claim worth nothing, so the profile fixes what a claim names and which statements a run exercises. | | CORE-VER-1, CORE-VER-2, SUITE-CONF-1 |
| This document's own version discipline and its changelog | in | Two tools built against different revisions have to be able to tell what changed between them, and a discipline stated as operations over the statement list is one a machine can check rather than one a reader has to trust. | | DOC-VER-1, DOC-VER-2, DOC-VER-3, DOC-VER-4, DOC-VER-5, DOC-VER-6, DOC-CHG-1, DOC-CHG-2 |
| The waiting order within a state | in | Two tools reading one workbench have to agree which card is next, or the workbench reorders itself for no visible reason when it changes hands. | | CORE-QUEUE-3, CORE-QUEUE-4 |
| The basis on a changing verb, and the revision it names | in | Deciding on a card that has since moved is the commonest way an automated caller does the wrong thing, and a basis compared against the card's current revision is the smallest thing that catches it. | | CORE-BASIS-1, CORE-BASIS-2, CORE-BASIS-3, CORE-BASIS-4, CORE-BASIS-5 |
| The four outcomes of a verb | in | Refused, stale and unreachable call for three different next moves, and a caller that cannot tell them apart cannot be driven without a person watching. | | CORE-OUT-1, CORE-OUT-4 |
| The claim as a lease that may expire | in | An owner that disappears must not be able to hold a card forever, and expiry that is recorded rather than silent keeps the record honest. | | CORE-CLAIM-4, CORE-CLAIM-5, CORE-HIST-2 |
| The interchange form of a workbench definition | in | A workbench definition nobody can carry between tools makes the whole exercise theoretical. One serialization is the smallest thing that solves it, and storage stays unconstrained. | | CORE-JSON-1, CORE-JSON-2, CORE-JSON-3, CORE-JSON-4, CORE-JSON-5, CORE-JSON-6, CORE-JSON-7, CORE-JSON-8 |
| Text encoding and the untranslated token | in | Two tools that disagree about encoding or that translate a token cannot read each other at all. | | CORE-TEXT-1, CORE-TEXT-2, CORE-TEXT-3, CORE-TEXT-4 |
| Parallel routes through the flow [lanes] | out | The core gains a simple model by having one route, and a tool that needs several can declare them in a layer. A real board already routes work three ways, so this is the likeliest first promotion. | A second tool needs routes and the layer form proves too weak to carry them. | |
| Conditions that must be satisfied before a card may enter a state [gates] | out | The core would gain enforcement it cannot describe generally, since what is worth gating differs per workbench. | A gate condition emerges that every workbench needs, rather than one each workbench defines. | |
| A limit on how many times a card may travel one backward edge [loop limits] | out | The core does not model backward edges as a distinct thing, so there is nothing yet for such a limit to count. | Backward edges are modelled, at which point counting travel over one becomes describable. | |
| Display grouping of states [column groups] | out | The core loses nothing, because no verb consults a grouping and a tool that ignores it loses only visual comfort. | A grouping starts carrying meaning a verb has to consult. | |
| A group of states behaving as one stage of the flow | out | The core would gain a second notion of position competing with the state, and two positions is one too many. | A workbench needs to move a card between groups without naming a state. | |
| Named groupings of cards within one workbench | out | The core loses only convenience. Grouping is a view over cards, and no verb changes behaviour because of one. | Membership starts constraining an act, such as a limit counted per grouping. | |
| Declared working identities with attached configuration [personas] | out | Configuration for whoever drives a tool is a property of that tool rather than of the shared model, and putting it here would make every conforming tool carry somebody else's settings. | Two tools need to agree on the identity of an automated owner beyond its name. | |
| A declared capability level attached to a card | out | Rating a card's difficulty is a judgement each workbench makes differently, and the core gains nothing by fixing the scale. | A capability level is used to refuse an act, at which point the refusal belongs to the contract. | |
| Refusing a claim on the ground of the owner's capability | out | A refusal that no other tool can evaluate would be a refusal name nobody can implement. | The preceding row is ruled in, since a refusal needs something to evaluate. | |
| Ranked priority levels on a card | out | The core's waiting order deliberately consults only arrival, and admitting ranks would make the order depend on a scale the profile does not define. | A shared ordering across tools is needed and arrival order proves insufficient. | |
| Ranked severity levels on a card | out | Severity changes no act and constrains nothing, so it is a field a workbench declares for its people to read. | Severity begins to constrain an act. | |
| Views across several workbenches at once | out | The core is scoped to one workbench, and a view over many is a reading built on top of conforming tools rather than a rule inside one. | Two tools need to agree how a card in one workbench refers to a card in another. | |
| Measurement and reporting over a workbench's history | out | History is already in the core, and a measurement is a reading of it. Fixing the measurements would freeze somebody's dashboard into the contract. | Two tools must produce identical numbers from identical history. | |
| Free prose attached to a card by its readers [comments] | out | The core loses nothing, because no verb consults prose, and a workbench can hold conversation in any field it likes. | A recorded act needs to reference a piece of that prose. | |
| Structured items on a card recording judgements | out | The core would gain a second bookkeeping model whose vocabulary each workbench defines differently. A real run wanted these badly, which is why the reopen condition is close at hand. | Such an item is used to refuse a move, which would put the refusal in the contract. | |
| The link a card carries to another card | in | Owners record that one card repeats, follows from or bears on another whether or not the contract has a place for it, and a reference kept in prose is text to the second tool rather than a reference. The kind stays open on the same ground as a block's kind, since nothing in the core consults it, and the card a link names stays inside the workbench because the profile is scoped to one throughout. The behaviour such a reference might carry is a separate concept and is ruled out in the row below. | | CORE-LINK-1, CORE-LINK-2, CORE-LINK-3, CORE-LINK-4, CORE-LINK-5, CORE-LINK-6 |
| Behaviour attached to a reference between cards [dependency ordering, ready-work listing] | out | The core would gain enforcement whose meaning each workbench sets differently, and what a workbench should do about a reference is exactly the judgement that differs between them. A tool that wants one card to hold another back declares a layer and refuses under that layer's own name, which CORE-LINK-5 leaves it free to do. | A relationship must refuse an act, such as one card holding another back. | |
| Documents belonging to a workbench rather than a card | out | Standing prose already has a home in the workbench's instructions, so a second one would be a slot with no rule attached. | A document must be served differently from the standing instructions. | |
| A state that buffers for a downstream state | out | The core already has capacity limits and arrival order, which is what a buffer is made of, and the extra kind would add a name without adding a rule. | A buffer needs a rule that a plain state cannot express. | |
| Charging a downstream state's budget at the moment a card is taken from a buffer | out | The core has no notion of a budget, so there is nothing yet to charge. | A budget enters the core, at which point the moment it is charged matters. | |
| A state where the workbench waits on somebody outside it | out | The core can express this already, as a state whose cards nobody claims, and a distinct kind would add machinery for the same result. | Waiting on an outside party needs a rule the block verb cannot express. | |
| Several people sharing one workbench, and who may do what | out | The core names an owner on every act and reserves some acts to the operator, which is the whole of what the model needs. Anything further is deployment. | Two tools must agree on a permission, rather than each enforcing its own. | |
| Proving that an owner name belongs to whoever presents it | out | A single-person tool has nobody to prove anything to, and a shared one has its own means. Fixing one would exclude both. No statement of this profile rests on the question, which is why section 5.4 settles it in prose: the core neither requires such proof nor forbids it. | Two tools must accept each other's evidence about an owner. | |

Rows ruled in: 33. Rows ruled out: 22. Total rows: 55.

### 10.1 Walking a wedding through the whole profile

This section is the standing demonstration that nothing here needs a
specialist's vocabulary. It is bound by the excluded-word lists of section
3.5 on the same terms as a normative statement, so a later revision that
smuggles a trade's vocabulary into the profile fails loudly here.

Priya and Sam are getting married. They keep a workbench called Wedding, and
they are its operators. Their flow has four states in this order: Ideas,
which is of kind `intake`; Deciding, of kind `work`, with a capacity limit of
three; Booking, of kind `work`, operator-owned; and Booked, of kind `done`.
The workbench's standing instructions say that nothing is agreed until both
of them have said so. Deciding carries its own instructions, which say to get
two quotes and write down why the chosen one won.

Priya writes a card titled "Flowers for the tables" into Ideas, where it
waits at substate `ready`. Two others are already waiting there, and the
queue puts hers third because it arrived last.

Priya notices that a card in Ideas titled "Table centrepieces" covers the same
work as the flowers card. She records a link on it, of kind `duplicates`,
naming the flowers card. Nothing about either card changes, both stay where
they are, and Sam decides later which of the two they keep.

She takes the next card from Ideas, which is the earliest arrival rather than
hers, and moves it to Deciding. The tool serves her the standing instructions
about both of them agreeing, followed by Deciding's own instructions about
two quotes, and tells her the moves that card can make now. She claims it,
and the card's substate becomes `active` with her named as holder and the
time recorded.

Sam tries to claim the same card. The tool refuses him, reporting `held`, and
tells him who has it. He takes a different card instead.

Later, wanting to be helpful, Sam tries to claim a waiting card in Priya's
name so that it is sitting ready for her when she gets back. The tool refuses
him, reporting `not-requester`. Work on this workbench is taken rather than
handed out, so what he can do instead is tell her the card is there, and she
claims it herself if she wants it. Nothing about their workbench's capacity
limits enters into that refusal, which is why the rule holds on a workbench
that declares no limit at all.

Priya gets called away for two days without releasing anything. Their tool
gives claims a two-day expiry, so hers lapses, the card returns to `ready`,
and the lapse is recorded where they can both see it. Nobody had to guess
whether she was still working.

Sam moves a third card into Deciding, which now holds three. He tries a
fourth and the tool refuses him, reporting `at-capacity`, because the limit
counts every card in Deciding whatever its substate. The refusal is the whole
point of the limit: they finish what they have started before they start
more. As an operator he could ask again with the override marker on the
request, and that move would be admitted and recorded as an override rather
than passing as an ordinary move. Priya, who is also an operator here, could
do the same; a guest helping them with the catering could not, and the tool
would refuse the marker in that guest's hands.

Priya returns, claims the flowers card, and finds that the florist she wanted
will not answer. She blocks the card with the reason "Florist has not replied
in nine days; do we go to the second quote?". The substate becomes `blocked`
and her claim is gone, so the card is visibly stuck rather than looking like
somebody being slow. Sam tries to move it and the tool refuses him, reporting
`blocked`.

Sam, as operator, decides they go to the second quote and unblocks the card.
It returns to `ready`. Priya claims it again, moves it to Booking, and keeps
her claim across the move, because a move changes where a card is and nothing
else.

Booking is operator-owned. Priya, having paid the deposit, tries to move the
card onward and the tool refuses her, reporting `not-operator`, because
leaving that state is a decision the two of them take together. Sam moves it
to Booked.

Booked is of kind `done`. The tool offers no forward move out of it, and when
Sam asks for one anyway, out of curiosity, the tool refuses him, reporting
`terminal`.

Weeks later they read the card's history. It shows every claim, move,
release, block and unblock in the order they happened, each with the time and
who did it, and the expiry of Priya's claim sits among them attributed to
her, since hers was the claim that lapsed. Sam's overridden move is one act
marked an override rather than a move and a second entry beside it. The
move into Deciding still says it came from Ideas, even though they have
since renamed Ideas to Wishlist, because the recorded act carried the title
as it stood that day. Nothing in the history was ever altered.

Priya later opens their workbench in a different tool. The first one hands
over the workbench definition in the interchange form, naming the profile
version it targets. The new tool reads it, keeps the two fields it does not
recognize, and shows the same four states in the same order.

Coverage. Every state kind: `intake` at Ideas, `work` at Deciding and
Booking, `done` at Booked. Every substate: `ready` while waiting, `active`
under Priya's claim, `blocked` at the florist. Every verb: claim, move,
release by expiry, block, unblock, and the reads for the next card and the
served instructions. Every refusal name reached by a person's act: `held`,
`not-requester`, `at-capacity`, `blocked`, `not-operator` and `terminal`.
Also exercised: the capacity count including a card of every substate, the
pull invariant on a claim, the operator's override and its marker, the
attribution of an expiry, the claim surviving a move, the queue's arrival
order, the two layers of instructions served together, the recorded history
with its titles as of the act, the link one card carries to another,
consulted by nothing, and the interchange form carrying unrecognized fields
across.

## 11. Index of normative statements

The subject column says what a statement is asserted against: a tool, this
document, a workbench's recorded history, or a conformance run. Keywords are
given in lowercase here, since only the uppercase forms in the statements
themselves carry meaning.

| Identifier | Keyword | Subject | Observable outcome |
| --- | --- | --- | --- |
| CORE-VER-1 | must | tool | A conformance claim the tool reports carries a major number and a minor number. |
| CORE-VER-2 | must not | tool | No conformance claim the tool reports carries a channel name. |
| SUITE-CONF-1 | must | suite | A conformance run exercises the statements whose identifier begins with CORE and no others. |
| DOC-VER-1 | must not | document | Extractions over two revisions differing by a patch increment are identical. |
| DOC-VER-2 | must | document | Every identifier of the prior revision appears in the extraction of a minor revision. |
| DOC-VER-3 | must | document | No revision whose extraction has lost an identifier carries an unchanged major number. |
| DOC-VER-4 | must not | document | For every identifier in both revisions, the texts match, or they differ only by a keyword weakened along the declared order. |
| DOC-VER-5 | must | document | A revision whose difference falls under none of DOC-VER-1 to DOC-VER-4 carries an incremented major number. |
| DOC-VER-6 | must not | document | No identifier retired by an earlier revision appears in a later extraction. |
| DOC-CHG-1 | must not | document | Every changelog entry of the prior revision appears unchanged in the current one. |
| DOC-CHG-2 | must | document | Each changelog entry carries a version, a channel, a date, at least one marked identifier, and prose. |
| CORE-BENCH-1 | must | tool | A workbench definition offered with no title is refused with `malformed`. |
| CORE-BENCH-2 | must | tool | A workbench definition offered with an empty state list is refused with `malformed`. |
| CORE-BENCH-3 | must | tool | A workbench definition offered with no declared profile version is refused with `malformed`. |
| CORE-BENCH-4 | must | tool | A definition declaring a major number the tool does not implement is refused with `unsupported-version`. |
| CORE-STATE-1 | must | tool | A definition carrying two states under one identifier is refused with `malformed`. |
| CORE-STATE-2 | must | tool | A definition carrying a state with no title is refused with `malformed`. |
| CORE-STATE-3 | must | tool | A definition carrying a state whose kind is outside the three is refused with `malformed`. |
| CORE-STATE-4 | may | tool | A definition marking a state operator-owned is accepted. |
| CORE-STATE-5 | may | tool | A definition declaring a capacity limit is accepted. |
| CORE-STATE-6 | must | tool | Reordering the state list reorders the flow the tool reports, with nothing else changed. |
| CORE-STATE-7 | must | tool | The legal moves reported for a card include a move to the next state in the list. |
| CORE-STATE-8 | may | tool | A move to a state other than the next one is accepted where no other rule refuses it. |
| CORE-STATE-9 | must not | tool | The legal moves reported for a card in a `done` state include no move to a later state. |
| CORE-CARD-1 | must | tool | No two cards in one workbench carry one identifier, and a card offered with an identifier already in use is refused. |
| CORE-CARD-2 | must not | tool | Retitling a card leaves its identifier unchanged. |
| CORE-CARD-3 | must | tool | A card offered with no title is refused with `malformed`. |
| CORE-CARD-4 | must | tool | A card naming a state the workbench does not declare is refused with `unknown-state`. |
| CORE-CARD-5 | must | tool | Every card the tool reports carries exactly one of the three substates. |
| CORE-CARD-6 | must | tool | A card the tool reports as `active` carries a holder and a claim time. |
| CORE-CARD-7 | must not | tool | A card the tool reports as `ready` or `blocked` carries no holder. |
| CORE-CARD-8 | may | tool | A card offered with a field the profile does not define is accepted. |
| CORE-CARD-9 | must | tool | A card read and written back carries the unrecognized field it arrived with. |
| CORE-OWNER-1 | must | tool | Every act in a card's history names an owner. |
| CORE-OWNER-2 | must | tool | The tool answers whether a named owner is the operator of a named workbench. |
| CORE-OWNER-3 | must | tool | A verb asked on a workbench that designates no operator is refused with `no-operator`. |
| CORE-VERB-1 | must | tool | A verb naming a card the workbench does not carry is refused with `unknown-card`. |
| CORE-VERB-2 | must | tool | A verb naming no owner is refused with `no-owner`. |
| CORE-QUEUE-3 | must | tool | Over a fixture with known arrival times, the next card is the earliest `ready` arrival, ties broken by ascending creation ordinal. |
| CORE-QUEUE-4 | may | tool | A tool offering another order still returns the CORE-QUEUE-3 order when asked for it. |
| CORE-TEXT-1 | must | tool | Text the tool writes decodes as UTF-8. |
| CORE-TEXT-2 | must not | tool | Identifier and token comparison gives the same answers under a Turkish locale as under a neutral one. |
| CORE-TEXT-3 | must not | tool | A machine-readable response carries the canonical token whatever language is asked for. |
| CORE-TEXT-4 | may | tool | A rendering meant for a person may carry a translated token while the machine-readable surface is unchanged. |
| CORE-JSON-1 | must | tool | A workbench definition written in the interchange form reads back with the same title, states and order. |
| CORE-JSON-2 | must | tool | The interchange form the tool writes parses as one JSON object and decodes as UTF-8. |
| CORE-JSON-3 | must | tool | The written object carries `profile`, `title` and `states`, and an object missing one of them is refused with `malformed`. |
| CORE-JSON-4 | must | tool | The order of `states` in the written object is the order of the flow. |
| CORE-JSON-5 | must | tool | Every element of `states` carries `id`, `title` and `kind`, and an element missing one of them is refused with `malformed`. |
| CORE-JSON-6 | may | tool | A state object carrying `instructions`, `operator_owned` or `capacity` is accepted. |
| CORE-JSON-7 | must | tool | An interchange object read and written back carries the unrecognized member it arrived with. |
| CORE-JSON-8 | may | tool | A tool holding definitions in some other form still produces the interchange form on request. |
| CORE-LINK-1 | may | tool | A card offered with a link is accepted. |
| CORE-LINK-2 | must | tool | A card offered carrying a link with no kind, or a link naming no card, is refused with `malformed`. |
| CORE-LINK-3 | must | tool | A link naming a card the workbench does not carry is refused with `unknown-card`. |
| CORE-LINK-4 | must not | tool | Two links carrying unrelated arbitrary kinds are both accepted. |
| CORE-LINK-5 | must not | tool | A claim, move, release, block or unblock refused because a card carries a link reports no refusal name this profile declares. |
| CORE-LINK-6 | must not | tool | A card another card's link names carries no link the tool added. |
| CORE-OUT-1 | must | tool | Every verb response carries exactly one of the four outcome tokens. |
| CORE-OUT-2 | must | tool | Every response of `refused` carries exactly one refusal name. |
| CORE-OUT-3 | must | tool | Every refusal name reported is one section 6.1 declares or one containing a full stop. |
| CORE-OUT-4 | must | tool | With whatever answers for the workbench made unavailable, a verb reports `unreachable`. |
| CORE-OUT-5 | must | tool | A definition, state, card or interchange object missing something the profile requires of it is refused with `malformed` wherever no more particular refusal name applies. |
| CORE-OUT-6 | must | tool | A request failing two checks is refused with the name carried by the earlier of the two in the order section 6 declares. |
| DOC-ORDER-1 | must | document | Two revisions stating section 6's checks in different orders differ by a major number. |
| CORE-BASIS-1 | may | tool | A request carrying a basis is accepted. |
| CORE-BASIS-2 | must | tool | A request whose basis names a superseded revision reports `stale`. |
| CORE-BASIS-3 | must not | tool | The card written after a request carrying a basis carries no basis. |
| CORE-BASIS-4 | must | tool | A response of `stale` carries the card's current revision. |
| CORE-BASIS-5 | must | tool | A request carrying an out-of-date basis and failing a precondition other than the card's existence comes back `stale`. |
| CORE-CLAIM-1 | must | tool | A claim on a `blocked` card is refused with `blocked`. |
| CORE-CLAIM-2 | must | tool | A claim on a card another owner holds is refused with `held`. |
| CORE-CLAIM-3 | must | tool | After a claim, the card reads `active`, names the claiming owner as holder, and carries a claim time. |
| CORE-CLAIM-4 | may | tool | A claim carrying an expiry is accepted. |
| CORE-CLAIM-5 | must | tool | After an expiry, the card reads `ready` and the history carries the expiry. |
| CORE-CLAIM-6 | must not | tool | No sequence other than release, expiry or block changes a card's holder. |
| CORE-CLAIM-7 | must | tool | A claim naming as holder an owner other than the one asking is refused with `not-requester`. |
| CORE-MOVE-1 | must | tool | A move to a state the workbench does not declare is refused with `unknown-state`. |
| CORE-MOVE-2 | must | tool | A move of a `blocked` card is refused with `blocked`. |
| CORE-MOVE-3 | must | tool | A move asked for by an owner other than the holder is refused with `held`. |
| CORE-MOVE-4 | must | tool | A move carrying no override marker into a state that has reached its limit is refused with `at-capacity`. |
| CORE-MOVE-5 | must | tool | A state whose limit is reached by cards of mixed substate refuses the next entry. |
| CORE-MOVE-6 | must | tool | A move out of an operator-owned state asked for by another owner is refused with `not-operator`. |
| CORE-MOVE-7 | must | tool | A forward move out of a `done` state is refused with `terminal`. |
| CORE-MOVE-8 | must not | tool | A card's substate and holder read the same before and after a move. |
| CORE-MOVE-9 | may | tool | An operator's move into a full state carrying the override marker is accepted where the tool offers the override. |
| CORE-MOVE-10 | must | tool | The history of a move admitted under CORE-MOVE-9 carries one act for that move, marked an override, and no second act beside it. |
| CORE-MOVE-11 | must | tool | A request carrying an override marker from an owner that is not the operator is refused with `not-operator`. |
| CORE-RELEASE-1 | must | tool | A release asked for by an owner that is not the holder is refused with `not-holder`. |
| CORE-RELEASE-2 | must | tool | After a release, the card reads `ready` and carries no holder. |
| CORE-BLOCK-1 | must | tool | Every blocked card the tool reports carries a reason. |
| CORE-BLOCK-2 | must | tool | A block carrying no reason is refused with `no-reason`. |
| CORE-BLOCK-3 | must | tool | After a block, the card reads `blocked` and carries no holder. |
| CORE-BLOCK-4 | may | tool | A block carrying a kind is accepted. |
| CORE-BLOCK-5 | must not | tool | Two blocks carrying unrelated arbitrary reasons are both accepted. |
| CORE-BLOCK-6 | must | tool | A block on a card another owner holds is refused with `held`. |
| CORE-UNBLOCK-1 | must | tool | The tool offers a verb after which a `blocked` card reads `ready`. |
| CORE-UNBLOCK-2 | must | tool | An unblock asked for by an owner that is not the operator is refused with `not-operator`. |
| CORE-UNBLOCK-3 | must not | tool | No verb other than unblock leaves a card that was `blocked` in another substate. |
| CORE-UNBLOCK-4 | must | tool | An unblock of a card whose substate is not `blocked` is refused with `not-blocked`. |
| CORE-HIST-1 | must | tool | After each of the five verbs, the card's history carries an entry with a time, an owner and the verb's name. |
| CORE-HIST-2 | must | tool | After a claim lapses, the history carries an entry with the time, attributed to the owner whose claim lapsed. |
| CORE-HIST-3 | must not | tool | History read after later acts still carries every earlier act unchanged. |
| CORE-HIST-4 | must | tool | A recorded move carries the identifier and the title of the state left and of the state entered. |
| CORE-HIST-5 | must | tool | The order of acts reported matches the order in which they were performed. |
| CORE-HIST-6 | must not | tool | A recorded act still reports its titles after the states it names have been renamed or removed. |
| CORE-INSTR-1 | may | tool | A state carrying instructions is accepted. |
| CORE-INSTR-2 | may | tool | A workbench carrying standing instructions is accepted. |
| CORE-INSTR-3 | must | tool | The response to a claim that succeeded carries the state's instructions. |
| CORE-INSTR-4 | must | tool | The response to a move that succeeded carries the entered state's instructions. |
| CORE-INSTR-5 | must | tool | The served text carries the workbench's standing instructions ahead of the state's. |
| CORE-INSTR-6 | must not | tool | After serving, neither the workbench's nor the state's stored instructions carry the other's text. |
| CORE-INSTR-7 | must | tool | The response to a claim or a move carries the moves legal for that card. |
| ACTOR-1 | must | history | The card's history carries a claim by that owner ahead of any other act by that owner on it. |
| ACTOR-2 | must not | history | No claim in the history extends past its owner's last act on the card without a release or an expiry. |
| ACTOR-3 | must | history | No act in the history was taken against a position the workbench did not hold at that time. |
| ACTOR-4 | must not | history | No move out of an operator-owned state in the history names an owner other than the operator. |
| CORE-LAYER-1 | must | tool | A layer declared under a dotted name is accepted, and one declared under an undotted name is refused. |
| DOC-LAYER-1 | must not | document | No token, member name or refusal name this document defines contains a full stop. |
| DOC-LAYER-2 | must not | document | No statement whose identifier begins with CORE names a layer among the things it requires. |
| CORE-LAYER-2 | must | tool | A workbench carrying a declared layer the tool does not understand still carries that layer's content after a read and a write. |
| CORE-LAYER-3 | must | tool | A definition declaring a layer under a name this profile defines is refused with `layer-collision`. |

The index carries 122 rows, which is the number of identifiers an extraction
over this revision returns.

## 12. Changelog

Entries are appended and never edited.

### 1.0, channel `dev`, 2026-08-16

Identifiers affected: every identifier carried in the index of section 11,
all of them introduced by this entry. No identifier is relaxed or retired,
this being the first published revision.

Consequence for a caller. This is the first statement of the profile, so
everything a conforming tool owes is new. A tool may now claim conformance to
`dinah-core 1.0` if it carries a workbench definition with an ordered flow of
titled and kinded states, cards with an identifier, a title, a state and a
substate, the five verbs claim, move, release, block and unblock with the
refusals section 6 names, claims that name the owner asking for them,
capacity limits enforced at entry with the operator's marked override as the
one way past one, instruction serving at claim and at move with the legal
moves alongside, append-only history with references carried as they stood,
the four outcomes kept apart, and the interchange form of section 5.7.
Nothing previously conformed, so nothing ceases to. The document sits on
the `dev` channel, so the compatibility promise of section 2 does not yet
bind, and it starts to bind at the recorded promotion to `stable`.

### 2.0, channel `dev`, 2026-08-17

Identifiers affected: CORE-QUEUE-1, retired. CORE-QUEUE-2, retired.
CORE-QUEUE-3, introduced, carrying CORE-QUEUE-1's demand with its tie-break
clause reworded to name ascending creation ordinal instead of ascending card
identifier. CORE-QUEUE-4, introduced, carrying CORE-QUEUE-2's demand with its
cross-reference reworded to name CORE-QUEUE-3 instead of the identifier it
retires. No other identifier in the section 11 index is affected.

Consequence for a caller. A tool computing the next `ready` card in a state
now breaks a same-arrival-time tie by the card's creation ordinal (the
`number` field the interchange form and every reference implementation
already carry) rather than by the card's identifier. A tool that offers
another order beside the fixed one now checks CORE-QUEUE-4 rather than
CORE-QUEUE-2 for the rule permitting it, though the two statements ask the
same thing of a tool under different names. A tool whose fixture data ties on
arrival time and asserted an identifier-ordered outcome will see a different
card returned; a tool with no such fixture, and no code that names
CORE-QUEUE-1 or CORE-QUEUE-2 directly, sees no behavior change beyond the
renumbering. The document sits on the `dev` channel, so nothing here binds a
caller who has not already opted into `dinah-core 2.0`.
