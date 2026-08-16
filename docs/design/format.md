# The Dinah on-disk format

This document captures the design of Dinah's per-workbench storage as agreed
in discussion. It is a working document, not the contract profile; the profile
will restate the parts of this that are contract, in normative form, and leave
the rest as implementation detail. Decisions recorded here are settled unless
reopened. Open questions are collected at the end.

A note for readers arriving fresh: Andoneer, cited throughout, is the hosted,
multi-seat implementation of the same coordination contract, described in this
repository's README under "Relationship to Andoneer". It is cited here as
design history, because much of this format encodes lessons that
implementation paid for first, and as the named boundary for concerns this
format deliberately excludes.

Terminology note: "workbench", "state", "card", and "substate" are working
terms. Whether Dinah keeps Andoneer's vocabulary wholesale is an open
question tracked on the board, and nothing below depends on the final names.

## Storage is the filesystem, entirely

A workbench is a directory of plain-text files. There is no database. The
state that genuinely cannot live in text files, which is concurrent multi-seat
access, cross-workbench portfolio queries, and telemetry, is exactly the state
that makes something Andoneer rather than Dinah. Dinah staying all-text is
the marker of where the upgrade path to the hosted product begins, not a
limitation to be patched with SQLite later.

Any derived file an implementation adds for speed, such as an index or cache,
must be deletable and rebuildable from the text. Derived files never become
truth.

## Entities

An entity is a thing the format tracks as an individual: it has an identity
that survives renaming, a lifecycle (created, possibly archived, possibly
deleted), and rules attached to it. The entity kinds are the workbench
itself, its states, its cards, its workstreams, each card's comments and
checklist items, attachments (which any entity may carry), and the folders
that organize attachments.

Everything else in a bench is one of two lesser things: content (prose
bodies, attachment payloads), which belongs to the human and carries no
imposed shape, or declarations (levels, groups, the states list), which are
configuration inside an entity's anchor and have no identity of their own.
An attachment shows the three-way split at its clearest. The entity wraps
its payload, carrying identity, metadata, and a journaled lifecycle around
bytes the format never inspects.

Containment is a closed grammar, stated here once and in full. The
workbench contains states, cards, workstreams, and attachments. A card
contains comments, checklist items, and attachments (and bears a journal,
as do the workbench and each workstream). A comment contains attachments,
and so does a checklist item. An attachment contains exactly
its payload. A folder contains attachments and folders, and may itself
exist only inside an `attachments/` collection. Two asymmetries carry the
design. Attachments may belong to any entity, and folders may belong only
to attachments. Anything the grammar does not say is thereby refused; a
containment this map lacks arrives only as a versioned spec change or as a
bench-declared, dot-named extension per the Extensions section, never as
an implementation's quiet liberty.

Formally, with `?` optional, `*` zero or more, and every collection
governed by absent-means-empty:

```
bench       ::= workbench.md journal.ndjson? attachments? states?
                cards? workstreams? archive?
state       ::= state.md attachments?
card        ::= card.md journal.ndjson comments? checklist? attachments?
checklist   ::= item*
item        ::= item.md attachments?
workstream  ::= workstream.md journal.ndjson? attachments?
comment     ::= comment.md attachments?
attachment  ::= attachment.md payload/
payload/    ::= exactly one payload-file
folder      ::= folder.md (attachment | folder)*
attachments ::= (attachment | folder)*
states      ::= state*        cards ::= card*
comments    ::= comment*      workstreams ::= workstream*
archive     ::= mirrors of the sibling collections, recursively

payload-file:   exactly one content file, any name, any bytes; content,
                not an entity, per the taxonomy
```

The notation makes two asymmetries visible. The card's journal is the
one non-anchor file that is not optional, because birth writes the created
event, while the bench journal appears on first bench-scoped act. `folder`,
meanwhile, occurs only on the right-hand side of `attachments`, which is
the folders-only-inside-attachments rule stated structurally. Attachment
and folder are siblings in the collection, not layers: an
attachments collection may legally hold only folders, only files, or any
mix at any depth, and an empty folder is legal and real, distinguished
from an absent collection by its anchor. The model is a filesystem's files
and directories, the intuition users arrive with. The token
registry is the authoritative, machine-readable home of this grammar; this
rendering is generated prose in waiting, and if the two ever disagree the
registry is right.

This taxonomy does quiet work throughout the document. Whether something is
an entity, content, or a declaration decides which rules reach it, so most
"should X get a directory?" questions are answered here before they are
asked.

## Layout

```
<bench>/
  workbench.md              # anchor: definition and overview
  journal.ndjson            # append-only history of bench-scoped acts
  attachments/
    <12-hex>/...            # bench-level attachments, same shape as below
  states/
    <12-hex>/state.md       # anchor: one state of the flow
  cards/
    <12-hex>/
      card.md               # anchor: identity, position, content
      journal.ndjson        # append-only history of this card
      comments/
        <12-hex>/comment.md # anchor: one comment; attachments/ on demand
      attachments/
        <12-hex>/
          attachment.md     # anchor: filename, description, provenance
          payload/
            <payload file>  # the bytes, under their original filename
  archive/
    cards/<12-hex>/...      # archived cards, on-demand only
```

### Identifiers

Every entity is named by a 12-character lowercase hex identifier, unique
within its containing collection. Identity therefore detaches from title, so
renames are free, and entity identity is never a word in any language. The
same identifier scheme is Andoneer's public-id scheme, so a bench paired with
a hosted board can share identifiers outright, and that reduces the eventual
passthrough MCP from a mapping problem to none.

The legibility cost is deliberate. A raw directory listing is plumbing, not
porcelain; humans read the bench through the CLI, and the fixed anchor names
keep grep workable (`grep -r "^title:" cards/*/card.md`).

### Anchor files and collections

There are no exceptions to the entity shape anywhere in the format. Every
entity kind named in the Entities section, from the workbench down to a
single attachment, is a hex directory claimed atomically by `mkdir` and
made real by its anchor file. Every general rule (id claiming, anchor
validity, archiving, fsck coverage) therefore applies to every entity kind
with no special cases, and a new entity kind added later inherits the whole
rulebook by construction.

Fsck's name and job are both borrowed from the Unix file system consistency
check of the same name, whose checking role Dinah's bench checker inherits
along with its spelling.

The anchor file is what makes the hex directory an entity, the way `.git`
makes a directory a repository. A card directory without `card.md` is
garbage by definition, which gives fsck a free validity rule and gives
entity creation a free crash story: make the directory, write the anchor,
and an interruption leaves a detectably incomplete thing.

The anchor's name is typed (`card.md`, `state.md`, `comment.md`) rather
than a uniform `entity.md`. The parent collection already carries the type,
so the typed name adds redundancy, and it is redundancy in the good
direction: an editor showing three open anchors names what each is instead
of three tabs all reading alike, a detached or mailed entity directory
stays self-describing, and grep targets stay narrow. Generic tooling needs
only the kind-to-anchor map, which the token registry serves.

Collections (`states/`, `cards/`, `comments/`, `attachments/`) get no
ceremony files. An absent collection directory means an empty collection;
implementations create them on first use and tolerate their absence. No
`.gitkeep` convention.

### Extensions

A bench may declare entity kinds the base format does not ship, and the
mechanism is declaration rather than code: a `kinds:` block in
`workbench.md` names each extension kind, its anchor filename, and which
of the standard mount points (bench, card, workstream) its collection may
hang from. Extension kind and collection names are dotted,
`<namespace>.<name>` (acme.assumption, acme.milestone), and core names never
contain a dot, so collision with future core vocabulary is unrepresentable
rather than managed. The no-exceptions entity shape is what makes
extensions safe. An implementation that does not understand a declared
kind still parses it, checks it structurally, archives it, and displays it
generically, because the uniform rulebook needs no semantics. Core
conformance is untouched by extensions, extension containments may add to
the grammar but never alter core productions, and interchange carries
extension entities marked as such for a receiver to preserve or decline.
An extension kind may declare `journal: true`, and an extension-borne
journal obeys the universal event skeleton (ts, event, actor; RFC 3339
timestamps; NDJSON; append-only), so generic tooling works on journals
whose event semantics it does not know. The worked example that earns the
feature is token accounting: a card-mounted ledger kind whose journal
lines are spend records, append-only as accounting demands, crash-safe by
the torn-tail rule, union-merging across clones because facts about the
past are the mergeable kind, with attribution built in. The plane split
lands where the two-planes rule puts it: recording spend is mergeable
facts and needs no arbiter; governing spend (budgets, throttling, pacing)
is decisions about now, which is the hosted product's plane, ingesting
local ledgers without mapping because card identifiers are shared. The
ledgers are naturally per-provider (anthropic.spend, ollama.usage), each
speaking its provider's native accounting vocabulary, because token counts
are not comparable across tokenizers and billing categories differ; the
skeleton is the only shared contract, and comparability is manufactured
honestly at the governing layer, in currency where currency exists, rather
than pretended in the records.

The promotion path is the product's own method: an extension proves itself
on working benches, and entering the shared core is a deliberate,
versioned spec change through the boundary table, so nothing joins the
core vocabulary that did not work somewhere first.

## Where benches live

Benches live in an overlay that works like git's. `~/.dinah/` is the user
base, holding benches that belong to no repository, keyed by bench id. A
bench may equally live inside a project repository (or anywhere else);
discovery walks up from the current directory before falling back to the
user base. The user base may also hold pointers to repo-local benches so a
listing sees everything. A bench inside a repository is versioned by that
repository's git, so board history rides project history and board changes
can be reviewed like code.

A distributed bench borrows git's model of a canonical home, pegged to a
URL, without borrowing git's URL. Two addresses answer two different
questions and neither is stored where the other lives. The git remote,
when the bench is in a repository, is where the files synchronize: the
content plane's transport, owned by git's own configuration and never
duplicated into the bench. The bench's home, an optional `home:` URL in
`workbench.md` frontmatter, is a live promise rather than a marker. The
URL must host a contract-conforming instance answering on the exposed surfaces
(the canonical-JSON verb contract and the mirror), so every bench that
calls it home can expect a definitive answer at verb time, per "Verb
outcomes and staleness", whose refused, stale, and unreachable outcomes
cover the times it cannot. A home naming a hosted board is how a bench
declares its arbiter. A bench that only wants to say which copy the team
treats as canonical needs no home for that, because sharing the git URL
already says it, and the home is likewise not the template provenance the
surfaces document records at instantiation: provenance is a display-tier
fact about where a bench was born, while the home is a promise about who
answers for it now. A bench with no home answers for itself, alone or
under the turn-taking etiquette of the transport rules. The promise is
testable, so the conformance suite can probe a claimed home.

## The card owns its position

A card's `card.md` frontmatter carries its state id and its substate. States
never list their member cards; membership is derived by scanning (or by a
rebuildable index). This is the single-writer rule: the thing that moves owns
its position. A move is one atomic frontmatter write plus a journal append.
Deleting a card is one directory removal, complete everywhere at once. The
alternative, states listing members, makes every move a two-file transaction
whose interruption strands a card nowhere or in two places, which is a
strictly worse failure than any dangling reference.

The dangling-reference risk this creates is handled from both ends. Forward,
the tool refuses to delete a state that cards currently occupy. Backward,
`dinah fsck` verifies that every card's state id resolves and that the
invariants hold. Fsck is what makes "manually editable if absolutely
necessary" safe: edit by hand, then ask the tool whether you broke anything.

### Substate

`substate` is one of `ready`, `active`, `blocked`. The invariants: `active`
and the presence of claim fields imply each other; `blocked` carries a reason
and is cleared only by the operator; `ready` means pullable. This maps onto
Andoneer's zone concept.

### Claims and blocks

A claim is two frontmatter fields on the card: `claim_holder` (an actor
string, see Actors below) and `claim_since` (a timestamp). Present together
with `substate: active`, absent together otherwise; fsck enforces the
implication both ways. A block is `block_reason` (required, posed so the
operator can answer it without opening the card) and optionally `block_kind`
and `block_since`, present exactly when `substate: blocked`. Clearing a block
is the operator's act and is journaled.

### What the card file carries

The card anchor's body is the card's framing: what this work is, why it
exists, who it affects, the paragraph an operator scanning the bench needs.
That is the only prose the format gives a card, because it is the only
prose every card on every kind of board needs. Everything else the work
produces or accumulates (a specification, research notes, a design, a
transcript, a menu) is an attachment, named and shaped by the method the
bench's instructions encode, not by the format. The hosted product's
three-field convention (description, spec, body on every card) is the
counterexample this rule corrects: a spec field on a wedding-planning board
is a wasted slot that quietly tells every non-software user the tool was
not built for them, while an attachment exists exactly when the method
called for it. The first hand-executed run demonstrated the shape
unprompted, carrying its framing in the card body and producing its
research and its concept as attachments.

### Lifecycle defaults

A new card enters the first state of the ordered list, substate `ready`, and
its journal opens with the created event. Pull order is deterministic so two
implementations agree on "the next card": highest declared priority first,
cards without a priority after all cards with one, ties broken oldest-created
first. A pull honors the destination's WIP limit and never takes a card out
of an operator-owned state.

Removal has two shapes with different promises. Archiving moves the card's
whole directory to `archive/cards/<id>/`, history and all; see Archive
below. Deletion is directory removal, and it destroys the card's history
with it, by design; archive is the default the CLI offers, deletion the
deliberate act.

### Manual edits are witnessed, not prevented

Hand-editing frontmatter is legal the way editing a git ref with an editor is
legal. The CLI notices divergence between current state and the journal on
next touch and records a manual-correction event rather than pretending the
edit did not happen.

## Flow definition

`workbench.md` frontmatter carries the ordered list of state ids. A YAML
sequence is ordered by definition, so this list is both the membership and
the sequence of the flow, and it is the single authority for order. State
files carry no position field; reordering the flow is reordering lines in one
file. Trailing comments on the list entries are annotation for humans, and a
lint warns when a comment drifts from the state's actual title.

Each `states/<id>/state.md` carries the state's own nature in frontmatter
(title, kind, operator flag) and its instructions as the body. `kind` is one
of `intake`, `work`, `done`. A state marked operator-owned is one an agent
never moves a card out of; only the operator does.

A state may declare `wip_limit: <n>`; absent means unlimited. The limit
counts every card in the state regardless of substate, because a blocked
card still occupies the station and exempting it would make blocking a way
to hide overload. Enforcement happens at entry: a move or pull into a full
state is refused, and an operator override is witnessed in the journal.
`wip_limit` is a closed registry token with enforced semantics, and it is
flagged for the profile's boundary table as a candidate for the core, since
it passes the wedding-planning test and is the most load-bearing mechanism
of the discipline.

Flow is linear for now. Real branching, such as lanes or a shortcut jump, is
deliberately deferred until building the CLI forces the question; the format
can absorb it later as an additive change (per-state transitions or a lane
construct in the definition), which the versioning posture below classifies
as non-breaking.

Groups, the folders a wide board subdivides its states into, are a display
overlay, not entities and not core: no verb consults a group, and an
implementation that ignores them loses only visual comfort. A `groups:` map
in `workbench.md` frontmatter names lists of state ids, kept separate from
the states list so the single authority for order stays intact; fsck checks
only that the referenced ids resolve. Whether groups enter the contract at
all is a boundary-table row.

### What "serve the instructions" composes

Instructions are an overlay chain, served most general first and never
copied between layers: the user-global layer (`~/.dinah/instructions.md`,
optional), then the workbench body, then the state body. The global layer
carries what applies to every bench on this machine, the bench body the
standing context of this workbench, the state body the station's own work.
Nothing is ever written from one layer into another; composition happens at
serve time. This chain is Andoneer's agent-context layering reproduced at
file scale, and it is the socket that role-scoped method packs plug into
later: a shared layer can be served ahead of the whole chain without any
bench storing a copy, and Andoneer paid for that same lesson.

Changes to the definition files themselves (states edited, list reordered)
get no journal; a bench versioned by git has that history in git, and a
bench outside git accepts that definition history is unwitnessed. The
journal is for cards.

### No structural inheritance

A bench never derives its structure (states, order, levels, groups) from
another bench or a global base at read time. The temptation splits into two
safe halves this format already provides, and a dangerous middle it
refuses. Guidance composes live through the overlay chain. That is safe
because concatenation has no override semantics, so layers cannot conflict.
Structure copies once through templates, and that is safe because
instantiate-and-own has no ongoing coupling. Live structural derivation is
the fragile-base-class problem with running work attached: cards hold
state ids and claims hang off stations, so a base edit would mutate every
derived bench's flow mid-flight, orphaning cards on boards whose operators
changed nothing, and the override rulebook it would demand is pure cost.
The fleet-update desire behind the temptation has uncoupled answers: shared
served text is the role-pack layer, and structural propagation is a
template diff applied per bench as a deliberate, reviewed change.

## History

Which kinds bear journals is a per-kind registry fact, decided by one test:
a journal belongs to an entity whose own history someone will read. Today
three kinds pass it: each card (required, born with its created event), the
workbench (on demand, for bench-scoped acts), and each workstream (on
demand, for its arc: created, status changes, archived), with a
workstream's journal traveling in its directory like a card's. Every other
event is recorded in the nearest enclosing journal-bearing entity: a card's
moves, claims, comments, attachments, and workstream-membership changes in
the card's journal, so the card's story stays one readable narrative; a
state archived, or a bench attachment replaced, in the bench's. One
asymmetry is accepted deliberately: an archived state's history does not
travel with its directory, because states are definition, and definition
history is git's when the bench is versioned. Definition file edits remain
unjournaled per the composition section; it is lifecycle acts that are
witnessed. A journal is append-only, one JSON object per line, recording
created, claimed, moved, released, blocked, archive-lifecycle, and
attachment-lifecycle events with timestamp, actor, and optional note.

NDJSON, newline-delimited JSON, is also known as JSON Lines: every line is
one complete JSON object and the newline separates records. It is chosen
over a JSON array because appending touches nothing that exists, so a crash
can tear at most the final line, while an array's closing bracket makes
every append a rewrite and every torn write a whole-file corruption. It is
chosen also because line order is visible order, lines grep and hand-append
like text, and two git branches' appends union-merge.

Journals are per-card rather than one per bench because history then
travels with the card when it is moved or archived, because concurrent
moves touch disjoint files, and because agents re-reading `card.md` at
every claim never pay for history they are not reading. Cross-card views
are a merge-sort over journals by timestamp.

The journal is authoritative for history; the card frontmatter is
authoritative for current position. They are reconciled by fsck and by the
manual-correction rule above.

Journal events are self-contained history: any cross-entity reference in an
event carries both the identifier and the display name as of the event
(from and to plus from_title and to_title on a move; the card's own title
on created), so the journal reads as a story without resolving anything and
survives rename, archive, and deletion of everything it mentions. The
title-then is deliberately a snapshot, never checked against the live bench
and never consulted by replay, which needs only the id sequence; the free
prose note remains the human's why, while the names ride as structured
fields because a note is unparseable by design.

File order is event order: the journal is append-only and the sequence of
lines is authoritative, while timestamps are information, not ordering. This
forecloses every clock-skew argument before it starts. Timestamps everywhere
in the format are RFC 3339 with a numeric UTC offset, as the worked example
shows.

Journal events carry no hash of their entity. A content hash cannot tell a
legitimate prose edit from a corruption, because editing the content plane
without a verb is the design working, and git merges would invalidate every
historical hash on both sides. Integrity of documents is git's job when the
bench is versioned, and integrity of decisions is the journal's, earned by
being append-only and witnessed; machine-field divergence is caught by
replay, and verb-time races by the basis guard, which hashes transiently
and stores nothing.

## Checklist items

A checklist item is a card-scoped entity recording a structured judgment:
`checklist/<12-hex>/item.md`, with `kind`, `state`, `owner`, and timestamps
in frontmatter, the item's text as the body, a resolution note required to
leave pending, and attachments for evidence per the universal rule. Kinds
are a closed set of three (acceptance_criterion, open_question, decision)
and states a closed set (pending, resolved, verified, failed), closed
because method text travels between boards and "file it with owner
operator" must mean the same thing everywhere. Items are per-item entities
rather than a list in the card anchor for the same reason comments are:
different actors add items concurrently, and per-item directories are the
conflict-free shape. Item lifecycle events land in the card's journal per
the nearest-enclosing rule.

Checklist items are not in the spec-and-notes category of imposed card
fields, and the distinction is the field-versus-collection line: a field
occupies a slot on every card whether the method wants it or not, while a
collection under absence-means-empty is invisible until used. A board
whose method never files a criterion carries no checklist anywhere. What
is fixed is only the shape when used, which is what lets shared method
packs speak one vocabulary. Whether contract behavior ever attaches to
items (gating a move on unresolved items) is a boundary-table ruling, not
assumed here.

## Comments and attachments

A comment is an entity like every other, per the no-exceptions rule in
"Anchor files and collections": a hex directory under `comments/` whose
anchor is `comment.md`, with timestamp and author in frontmatter and the
comment as body, and its own `attachments/` on demand. Ordering comes from
the timestamp field, not the directory name.

An attachment is likewise an entity: a hex directory whose anchor,
`attachment.md`, records the original filename, a description, and
provenance, beside a `payload/` directory holding exactly one file
carrying the bytes under their original name. The payload directory is
what makes the payload structurally identified: a stray file beside the
anchor is unambiguously garbage rather than a candidate payload, and the
payload's namespace contains no reserved names, so filename collisions
with anchors are unrepresentable. The payload is content, never inspected
by the format; the
entity around it is what makes the attachment referenceable, replaceable
accountably, and archivable. Any entity may carry an `attachments/`
collection: the workbench itself (reference documents that belong to the
board rather than to any card), a state, a card, a comment. Replacing a
payload is a journaled act (attached, attachment_replaced, and
attachment_removed are registry members of the closed event set, carrying
the attachment id and its filename as of the event), recorded in the
nearest enclosing journal per the History section; prior payload versions
are git's concern when the bench is versioned, per the content plane's
integrity assignment. A multi-file deliverable is multiple attachments.

Attachments may be organized into folders, and a folder is itself an
entity, by the taxonomy's own tests: it has identity that survives
renaming, a lifecycle (archiving a folder of obsolete attachments is one
act on the subtree, via the recursive archive pattern), and an anchor
(`folder.md`: title and description). A folder is a hex directory inside an
`attachments/` collection, holding attachment entities and, recursively,
other folders; uniqueness stays per containing collection; refiling an
attachment is a directory move journaled with names as of the event.
Folders exist only inside attachment collections: cards are organized by
states and workstreams and states by groups, and a folders-of-anything
generalization would compete with all three. Folders are additive and
deferred from the first cut: a new entity kind inherits the whole rulebook
by construction, so shipping them later leaks into no interface.

## Encoding

Every text file in a bench is UTF-8 without a byte-order mark. Journal
records are separated by LF; writers emit LF everywhere, and readers
tolerate CRLF, which a Windows editor or a misconfigured git filter will
inevitably introduce, by stripping a trailing carriage return per line
rather than failing. Content prose is any language UTF-8 carries;
identifiers and tokens are ASCII lowercase by construction. Case
operations on tokens and identifiers use ASCII rules, never the locale:
the Turkish dotted and dotless i turn locale-aware lowercasing into a
correctness bug (an uppercase I lowercased under a Turkish locale is not
`i`), and a bench must parse identically on a machine in Istanbul and a
machine in Iowa.

Lowercase-only is load-bearing for a second reason: identifiers and anchor
names are directory and file names, and a bench travels between
case-sensitive filesystems (Linux) and case-insensitive ones (Windows and
macOS by default). Names differing only by case would produce trees that
commit cleanly on one platform and collide on checkout on another, a
classic cross-platform git failure; single-case-by-construction makes the
collision unrepresentable. The one case-arbitrary name in the format, an
attachment's original payload filename, is contained the same way: it
lives alone inside the attachment's `payload/` directory, a namespace
holding no reserved names, so a collision with an anchor or journal name
is unrepresentable rather than guarded against.

## Language independence

The rule is that `ready` is not an English word but a contract token that
happens to be mnemonic in English, the way HTTP's GET and git's commit are.
Formats that localized their vocabulary are the cautionary tale (Excel
translates formula names, so spreadsheets break crossing a border). Three
tiers split the vocabulary by who the text is for:

1. Tokens. Structural keys and closed enum values. Machine vocabulary,
   defined once in the profile, stored canonically, never translated on disk.
2. Display. How a UI renders a token, via a locale catalog. Localization
   lives entirely in each implementation's presentation layer.
3. Content. Titles, instructions, notes, comments. Free prose in any
   language; the format never inspects it.

A bench written in Jakarta is byte-compatible with a tool built in Stuttgart.

The initial display-catalog languages are English (en), German (de), Czech
(cs), Bahasa Indonesia (id), Spanish (es), Hindi (hi), Filipino (fil), and
Afrikaans (af). Mandarin and Japanese are likely next; Mandarin arrives as
two catalogs under the standard script subtags (zh-Hans, zh-Hant), which
the BCP 47 hierarchy already handles. A bare tag implies its default
script per the standard's likely-subtags data, so hi is Devanagari without
saying so, and an explicit script subtag appears only to deviate from the
default (hi-Latn for romanized Hindi, sr-Latn for Latin-script Serbian);
Mandarin is the notable case where no default is safe to assume, which is
why it ships as two tags. The set reflects where the first users work;
adding a language is adding a catalog, never touching the format,
and an incomplete catalog falls back to English per token rather than
failing.

Language identifiers are BCP 47 tags. Regional variants (en-US, en-GB, de-AT)
are supported through the standard hierarchy: lookup walks the tag from most
specific to least (en-GB, then en, then the English fallback), and a regional
catalog is an overlay holding only the tokens that differ from its base
language, never a full copy. English ships with en-US and en-GB overlays from
the start; other regions are added when someone needs one. The tool accepts
the common miswriting en-UK and normalizes it to en-GB.

### Display language resolution

The reader's language is resolved by the tool at render time, first hit wins:
a per-invocation flag (`--lang hi`), then the `DINAH_LANG` environment
variable, then the user config (`config.md` in the user base, set once via
`dinah config set lang hi`), then the operating system locale as a hint
only, then English. The OS locale describes the machine, not the person
reading the screen; a user on a foreign-language laptop overrides it once in
user config and never thinks about it again.

Two boundary rules apply. The bench never dictates display language: a
workbench is shared, display is personal, and tokens always render in the
reader's language, so the bench has no slot to express a preference. The
machine surfaces never localize: MCP responses, JSON output modes, journal
contents, and anything meant for parsing carry canonical tokens regardless
of any language setting. Localization applies exactly where a human reads.

### The token registry

The profile ships its vocabulary as a machine-readable registry (a small JSON
file listing every key, every enum, and what may appear where). One artifact
feeds four consumers: the conformance suite, fsck and the lints, an LSP for
people who hand-edit these files (completion and diagnostics fall out of a
schema server), and localization catalogs. Nobody maintains four lists that
drift.

### Closed versus open enums

An enum is closed when the contract attaches behavior to its members:
`substate`, `kind`, and the journal event set are closed, because the tool
enforces their meanings and a member it has never heard of cannot be
enforced. Adding a member is a spec change. An enum is open when only humans
interpret it: severity and priority level sets are the proof case, declared
per workbench, because no contract behavior hangs on their members. The
registry marks which is which, and this rule is the test that settles every
future "should this be configurable?" argument.

## Workstreams

A workstream is an entity: `workstreams/<12-hex>/workstream.md`, with title
and status in frontmatter and long-form notes as the body. Membership is
card-owned, a `workstreams:` list of ids in card frontmatter, by the same
single-writer logic as position: deleting a card removes its memberships
with it, deleting a workstream that cards still reference is refused, and
fsck catches danglers. Archiving a workstream moves its directory to
`archive/workstreams/<id>/` like any other entity.

The real use case is within-bench grouping of concurrent efforts (several
concepts flowing through one concept bench at once), which is distinct from
the portfolio machinery the exclusion-candidate list meant, and it passes
the wedding-planning test trivially. Whether workstreams land in the
contract core or an extension is a boundary-table ruling; the format
supports them either way.

## Severity and priority

Severity and priority are both optional, workbench-declared level sets. A
card that omits the field is a card without one; absence is legal, not an
error. The declaration reuses the ordered-sequence-as-data precedent from
the states list: a `levels:` block in `workbench.md` frontmatter lists each
axis's members from low to high, and the sequence order is the rank.

```yaml
levels:
  severity: [trivial, minor, major, critical]
  priority: [later, soon, next, now]
```

Levels are declared tokens, not entities: a level's identity is its name
within the bench, nothing claims it and nothing accumulates on it, so it
earns no directory. Guidance thickens the declaration in place: a list
entry may be a bare name or a name mapping to a one-line hint, the two
forms mix freely, and order still carries rank.

```yaml
levels:
  severity:
    - trivial
    - minor
    - major: A person's work is wrong or blocked; fix before new work.
    - critical: Data loss or money; drop everything.
```

A hint is content-tier text in the author's language. Guidance longer than
a hint, a real rubric with examples, is ordinary prose in the workbench
body, which the serve-time composition already delivers as standing
context.

(Andoneer currently requires both on every card; making them optional there
is tracked on its own board.) Domain fields generally, such as `project` and
`repository` on a ticket card, live on the card without the core knowing
them; unknown keys are tolerated.

## Versioning

The format carries two version numbers with two audiences, and they are
never conflated:

- **Storage format version.** `format: 1` in `workbench.md` frontmatter,
  an integer governing the whole bench directory. An implementation that
  opens a bench with a higher number than it knows refuses loudly and names
  the version it wanted. This is Dinah's private business; the git
  precedent (`core.repositoryformatversion`, carried always, bumped
  approximately once) is the model, and the ambition is to never bump it.
- **Profile version.** The contract's public promise, with the channel and
  increment rules recorded with the contract-profile work. Andoneer never
  reads `format:`; implementations meet through the JSON interchange, which
  declares the profile version it speaks.

One reader posture makes upgrades safe. Readers ignore keys they do not
know, and the format version gates only changes that would make an old
reader wrong rather than merely incomplete. Additive changes (a future
`lane` key, a declared level set) need no bump.

## Actors and attribution

Every journal event and comment carries an `actor`, a free string identifying
who acted: a person's handle, an agent's name, a harness's session label.
Actors are self-declared attribution, not authorization; the format has no
account system, and honesty is enforced socially and by the journal being
append-only, which is the same stance Andoneer takes with self-reported agent
identity. The tool resolves the actor the same way it resolves language:
per-invocation flag, then environment, then `actor:` in the user config, and
it refuses to write an event with no actor rather than inventing one. One
seat running many agents is therefore many actors in one bench, and that is
what makes the journal's story readable after the fact.

## Concurrency and atomicity

Git's answer comes in two halves, and both are adopted honestly.

Locally, concurrency is solved with primitives, not cleverness. Writes are
write-temp-then-rename. Mutations lock at card scope: a move or claim takes
a lockfile inside that card's directory, so two processes working different
cards never contend. Creating an entity needs no separate lock, because
`mkdir` of the hex directory is itself the atomic test-and-claim of the id.

The lockfile is specified concretely so any process, including a careful
human, can participate. It is named `lock`, sits directly inside the entity
directory it protects, and is created with the filesystem's exclusive-create
primitive (O_EXCL; in a shell, noclobber redirection gives the same
atomicity). Its content is a single JSON line carrying actor, pid, and ts.
It is removed by plain deletion after the protected write's rename lands. A
lock is never recorded inside an entity's own frontmatter: acquiring a lock
by read-modify-write is a race, an in-band lock is coordination-plane state
embedded in a mergeable document, and rewriting the file on every
acquisition would churn the basis hash. The lock is a mutex measured in
milliseconds; the claim is a lease measured in hours; frontmatter is right
for the second and wrong for the first.

A tool finding a stale lock refuses loudly and names the holder from the
lock's own content, and a human removes it, the git contract exactly.
Nothing auto-breaks a lock silently. A stale claim after a crash is a
visible line in a text file that a human can fix with an editor, then run
fsck.

Remotely, the honest half: git does not support two machines sharing one
working tree, and neither does Dinah. A bench on a sync service or a
network share with concurrent writers is explicitly unsupported, because
rename atomicity there is exactly the undocumented behavior this design
never relies on. The supported remote story is git as transport: clone the
bench, work, push, pull. The format merges well by construction, since
entity directories keep concurrent card work in disjoint files and
append-only journals take a union merge; a conflict inside one card's
frontmatter is real contention, rare, and resolved by a human. Real-time
multi-writer coordination is the hosted product's job, not this format's.

## The two planes

A board decomposes into two planes with different concurrency DNA, and the
design's coherence comes from never letting either borrow the other's
semantics.

The content plane is documents: definitions, instructions, card prose,
comments, history. Documents reconcile after the fact, so this plane has
git's DNA: it snapshots, distributes, and merges, and a change to a bench's
definition arriving as a reviewable diff is a better workflow than editing
live.

The coordination plane is decisions about now: claims, substates, positions
as facts of the present, WIP accounting. It has the checkout DNA of the
lock-based SCMs, and unapologetically so, because the general rule is that
concurrency style follows mergeability, and a claim locks effort, the least
mergeable artifact there is. Two agents completing the same card cannot be
reconciled into one spend, so exclusion must happen at decision time.
Merging later only documents the conflict after the money is gone. The
classic lock-system injuries each have a structural answer here: the absent
holder (leases with TTL and heartbeat, and release-when-idle as a
working-agreement rule), the unbreakable lock (operator override, witnessed
in the journal), lock hoarding (WIP limits bound the stations themselves),
the invisible lock (claim state is the display).

The arbiter rule follows: the moment two or more writers coordinate on one
bench concurrently, claim state and WIP accounting need a single live
arbiter. Turn-taking writers over git transport need none, which is why the
remote story holds. The live arbiter for many writers is the hosted
product; that is the product boundary restated. Andoneer never re-platforms
its coordination storage onto this format, and this format never becomes a
hosted product's disk layout; the two implementations meet at the contract,
the conformance suite, and the interchange. A mirror or export carries
positions as facts as-of a moment, never as the live "is"; consumers of a
mirror read history, consumers of the arbiter read the present.

## Verb outcomes and staleness

Coordination truth is consulted, never replicated. Claim, move, release,
block, and the WIP check inside a move are synchronous questions asked at
the moment of action; there is no background synchronization of claim
state, because there is no local copy with standing to disagree. The only
timer is the lease: a claim carries a TTL and is kept alive by heartbeat,
and expiry lapses it visibly and journaled rather than silently
reassigning.

Mutating verbs carry a basis: the revision of the entity the actor read
before acting (in a local bench, a content hash checked under the card
lock). The arbiter refuses a mutation whose basis is stale. Every
coordination verb therefore has three distinct outcomes that must never be
conflated, for scripts and agents above all: refused (policy says no),
stale (the actor's knowledge is old; refresh and retry is legitimate), and
unreachable (the question could not be asked). Exit codes and machine
output keep the three apart.

Offline needs no special rule. Content verbs work offline because documents
are the content plane; coordination verbs against a live arbiter fail
loudly. There are no queued claims: a claim resolved after the work started
resolved nothing, and queuing one is merge-later smuggled back into the
lock plane.

Displays are the one place eventual consistency is welcome. A view learns
of change by poll or push at whatever cadence pleases the eye, marks itself
stale, and never rearranges under the actor: notify, do not mutate, and let
the basis guard catch whoever acts on old knowledge. A pushed event is
never evidence that an action is permitted, only a hint that asking might
now succeed.

## Corruption and recovery

Backup, and git when the bench lives in a repository, are the last resort;
before them the format's own redundancy carries most recoveries, and
`dinah fsck` is the tool that uses it. Frontmatter present but mangled:
the journal replays to reconstruct the card's current position, since
history determines the present. An anchor that is absent entirely is the
quarantine case below, not a replay case, and the two inputs get different
answers. Journal lost: the frontmatter still carries present truth, the
history is gone, and fsck records a witnessed history-lost event rather
than pretending otherwise. Torn journal tail after a crash: readers
tolerate a trailing partial line, and fsck trims it with a witness. Anchor
file missing: the directory is quarantined to `lost+found/`, never silently
deleted. Every repair is journaled as itself an event, so recovery leaves a
trail instead of a mystery.

## Archive

Archiving moves an entity's whole directory into `archive/`, which mirrors
collection structure (`archive/cards/<id>/`, and the same shape serves
retired states). History travels with the directory. The pattern recurses:
any collection at any depth may have an archive mirror at its own level, so
a card's noisy old comments archive to `cards/<id>/archive/comments/<id>/`
exactly the way a card archives at bench level, and the live-is-fair-game,
archive-is-on-demand rule applies at every depth. One pattern serves every
kind, with no per-kind machinery.

The rule is structural, not a filter: whatever is in `cards/` is live and
always fair game, whatever is in `archive/` is crawled only on demand.
Listings, pull, and WIP counts therefore ignore archives by construction
rather than by remembering to check a flag, and scan cost stays bounded by
the live set for the life of the bench. Archiving and restoring are
themselves journal events on the entity's own journal, so a restored card's
history shows its archive years instead of a silent hole; both are registry
tokens in the closed event set.

Neither archiving nor deleting an entity can invalidate any journal, for
two structural reasons: colocation (a journal travels or dies with its
entity, never orphaned) and the rule that history describes then, not now.
A journal's references to other entities are facts about the moment they
were written, so fsck's reference checks apply to frontmatter, never to
journals; an implementation that verified historical ids against the live
bench would turn normal housekeeping into false corruption reports across
every old journal. Replay needs only the id sequence, and the present-tense
pointer that must resolve is guarded from the front by the refusal to
delete an occupied state. Identifier uniqueness keeps its per-collection
scope from the Identifiers rule, with a collection spanning its live and
archived halves: a card id may not repeat between cards/ and
archive/cards/, while the same id under two different cards' comments/
collections remains legal. Uniqueness is checked at creation against both
halves; restoring an entity is moving its directory back.

## The worked example

`~/.dinah/c1eeb1998b99/` holds a filled example: a Jira-ticket-resolution
bench with thirteen states (an eleven-step agent workflow plus intake and
done, four states operator-owned), one card mid-flight with a nine-event
journal and a comment. It exists to be walked and edited; the state set is
knowingly imperfect as a Kanban board, and editing it after creation is the
point being exercised.

## Open questions

- The journal event schema, normatively: required fields per event kind
  (the shape is settled by the worked example; the profile still owes the
  normative statement of it).
- Card-to-card links. The Alka flow already produces a real case (a ticket
  found to duplicate another), and Andoneer's link kinds with gate semantics
  are on the excluded-candidate list, so the open question is whether a
  minimal semantics-free link (kind plus target id in frontmatter) earns a
  place before lanes and gates do.
- Branching and lanes, when the linear flow stops being enough; the Alka
  bench already contains one prose shortcut that will eventually force this.
- Human handles: whether cards get a slug or number alias for CLI ergonomics,
  or titles resolved by search are enough.
- Terminology: whether "workbench", "state", "card" survive into the
  contract, coherent with Andoneer either way.
- The CLI, API, and MCP surfaces, which are the next conversation.
