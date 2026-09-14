---
title: get and set reach every field of every kind, so an agent can change prose a person can change
column: b69abf918c42
state: ready
severity: minor
priority: soon
tier: frontier
workstreams:
  - 994787601ae6
---
No prose field of any kind in a Dinah workbench is writable by a command that builds a request. A card's title and body, a comment's text, a checklist item's text and note, a column's instructions and a workbench's instructions are reachable only through `edit`, which executes the reader's editor and constructs no request at all. `edit` is deliberately held out of the MCP surface, so an agent connected over MCP cannot change one word of prose anywhere.

That asymmetry is what prompted dinah-456, and this card closes it. `dinah get <ref> <field>` and `dinah set <ref> <field> <value|->` take any reference the grammar resolves, and arrive over MCP as `get_field` and `set_field`.

`bench.FieldsOf(kind)` becomes the one declaration of what a kind's fields are, folding in the existing `CardFields` and `WorkstreamFields`. Four writes keep their current guards rather than being widened: `slug` keeps its uniqueness checks, a card's `tier` routes through `SetCardTierAt`, an item's `state` admits only the values its kind admits and records the event the matching verb records, and an attachment's `filename` routes through `Rename` so the payload moves with the name.

dinah-456 sections 5.1 to 5.3 are the contract for the two commands. dinah-456 section 6 is also this card's, because putting `get_field` and `set_field` on the MCP roster is what makes the exemption rule load-bearing: `toolExemptions` changes from a map of command to reason into a map of command to a struct of `ground` and `reason`, the four grounds are declared as constants, and the roster test asserts every entry's ground is one of them. That reshape touches all nine existing entries and no behaviour.

**The six kind-prefixed subcommands are retired, and this card builds one outcome rather than two.** The operator ruled on 2026-09-09, on dinah-456's OQ-1: "Make the change. Testers understand that this is pre-release." So `card get`, `card set`, `workbench get`, `workbench set`, `workstream get` and `workstream set` stop being accepted, and the three kind-shaped MCP tools `card`, `workbench` and `workstream` lose the `get` and `set` actions that `doCard`, `doWorkbench` and `doWorkstream` select on today. Their remaining actions are untouched, and the two capabilities leave under one rule rather than one of them surviving on the head a person cannot see.

dinah-456 section 5.2 is the contract for what retirement means. Two things it asks of this card are worth carrying here. State in your handoff which refusal each retired spelling now produces, per command rather than assuming the three parent commands are alike; `runColumn`, which refuses any first word but `new`, is the precedent. And sweep the documentation for the retired spellings rather than editing the places you remember, because the quick start's replayed transcripts are checked against live output and a stale line there fails a test rather than merely reading wrong.

An earlier version of this paragraph told the implementer that the retirement was an open question and to build the generic pair either way. That was right while the question was open and it is wrong now, which is why it was rewritten rather than annotated.

## Specification

Everything below was read against `origin/main` at `9260a2ace1f7b7849e667be15873a8fa106f8e2a`
("dinah-436: give a card link its write side"). The first draft was written in the worktree
`C:/dinah-scratch/dinah-460-spec/wt` and this revision in
`C:/dinah-scratch/dinah-460-spec2/wt`, both at that commit; `git fetch origin` was run at the
start of each, and the trunk had not moved between them, so this revision answers the review
against exactly the tree the review read. Four of this
workstream's siblings have landed since dinah-456 was written: `813e0bb` (dinah-459),
`8661604` (dinah-454), `70ff10f` (dinah-455) and `863b7c5` (dinah-457). Every claim this
spec makes about the trunk was read at `9260a2a` rather than carried forward from the
parent, and the probe transcripts quoted below were produced by a binary built from that
commit and run against a throwaway workbench under `C:/dinah-scratch/dinah-460-spec/`,
with `DINAH_HOME` pointed inside that directory.

This card implements dinah-456 sections 5.1, 5.2, 5.3 and 6. The contract those sections
carry is settled and is not reopened here. Two places where this spec adds a field the
parent's own rule requires and the parent's table omits are stated in section 3.4, loudly,
so a reviewer can overturn either one without reading the rest.

# 1. What lands

This card lands two commands, `dinah get` and `dinah set`, which reach every field of every
kind through any reference the grammar resolves and arrive over MCP as `get_field` and
`set_field`. One new file, `internal/bench/fields.go`, declares what a kind's fields are.
Six command spellings are retired, and the `card` command goes with them. Three journal
events are minted, so that a write below a card is recorded the way every other write
already is. `toolExemptions` is reshaped to carry a declared ground drawn from a closed set,
which is dinah-456's section 6 and rides here because this is the card that puts two new
tools on the roster.

# 2. The two verbs

```
dinah get <ref> <field>
dinah set <ref> <field> [value|-] [--at <column>] [--note <text>] [--yes]
```

`get` prints the field's value followed by a newline and exits 0. A field the entity
carries no value on prints an empty line, which is what `card get` does today at
`cmd/dinah/commands.go:307`. Under `--json` it prints `{"value": "<the value>"}`, which is
the payload object `doCard` already wraps for the MCP `card` tool at
`internal/mcp/tools.go:730`. Only the payload is carried forward from that call. The
affordance list beside it in the same expression is not, for the reasons section 8.1 gives.
Reading is open to anybody, so `get` asks for no owner and no operator, on the terms
`Library.CardField` states at `internal/verb/beyond.go:495`.

`set` writes the field, records the write on the journal of the nearest enclosing
journal-bearing entity, and prints the ordinary ok line with the written value as its
detail. A single dash in the value slot reads the value from stdin, which is the spelling
`dinah comment <card> -` already uses at `cmd/dinah/commands.go:365`. An omitted value
clears the field where the field declares itself clearable and refuses `malformed` where it
does not, which section 4.3 fixes per field.

Both commands declare `Guide: "references"` on the reference parameter and carry
`"references"` in the `guides` map at `internal/verb/definition.go:254`, because
`internal/verb/collection_roster_test.go` holds those two declarations to one another and a
command declaring the topic in one place alone is a defect that guard exists to catch.

## 2.1 The parameter declarations

In `internal/verb/definition.go`'s `params`:

```go
"get": {
	{Name: "ref", Required: true, Guide: referencesGuide, Field: "Ref"},
	{Name: "field", Required: true, Field: "Field"},
},
"set": {
	{Name: "ref", Required: true, Guide: referencesGuide, Field: "Ref"},
	{Name: "field", Required: true, Vocabulary: "entity-field", Field: "Field"},
	{Name: "value", Display: "value|-", Rest: true, Field: "Value"},
	{Name: "at", Flag: true, Value: "column", Vocabulary: "column", Field: "At"},
	{Name: "note", Flag: true, Value: "text", Field: "Note"},
	{Name: "yes", Flag: true, Marker: true, Shared: "yes", Field: "Confirm"},
},
```

`value` is the open tail, so a caller composing several words quotes them into one and
`freeText` refuses the unquoted form under `dinah.multiple-words`, which is the shape
`card set` has today.

`--at` is legal only where the reference names a card and the field is `tier`, and any
other use is `dinah.usage` naming `--at`. That is exactly the check `runCardSet` runs at
`cmd/dinah/commands.go:341`, moved rather than rewritten.

`--note` is legal only where the field is `state`, and any other use is `dinah.usage`
naming `--note`. Section 4.4 says what it carries.

`--yes` is required on a write to any field whose guard is `slug`, and section 4.4 says so
per field rather than per command.

## 2.2 Where they sit at the terminal

Both join `groupBench` in `cmd/dinah/commands.go`'s `commands`, beside `path` and `edit`.
The groups split on what a command acts on: `groupWork` holds the commands that act on a
card, and these two act on any entity of the workbench, which is what `path` and `edit`
already do from the same group. Both declare their own arity and mistyped-flag checks, on
the terms `runCard` states at `cmd/dinah/commands.go:272`.

```go
{name: "get", group: groupBench, run: runGet, bounded: 2},
{name: "set", group: groupBench, run: runSet, openTail: true},
```

`get` binds two positionals and takes no tail, so a third word is `dinah.usage` naming that
word. `set` binds two and lets the value run to the end of the line.

## 2.3 The field vocabulary

`vocabularies` in `internal/verb/definition.go:161` gains one entry and loses one:

```go
"entity-field": {Values: bench.AllFields()},
```

`bench.AllFields()` is the sorted union of `FieldsOf` over `bench.EntityKinds()`, derived
rather than typed. The `"field"` vocabulary, whose values are `bench.WorkbenchFields`, goes
with the `workbench` command's field argument.

The union is a closed set in the sense the `Vocabulary` doc comment means: every value the
argument accepts is in it. It is not the set any one reference accepts, because that
depends on the kind, and `param.set.field.summary` says so in one clause. A caller naming a
field the resolved kind does not carry is refused by `dinah.unknown-field`, whose sentence
lists that kind's own set, so the schema narrows the guess and the refusal completes it.

# 3. The field declaration

One file, `internal/bench/fields.go`, is the only statement of what a kind's fields are.

```go
// Field is one field of one kind: the name a reader types, where the value is
// stored, whether it may be cleared, and which guard a write to it runs.
type Field struct {
	// Name is what a reader types after the reference.
	Name string
	// Prose is true where the value is the anchor's body rather than a
	// frontmatter key. A prose field holds several lines; every other field
	// holds one.
	Prose bool
	// Clearable is true where a write with no value clears the field. A field
	// the entity may not be without declares false and refuses the clear.
	Clearable bool
	// Guard names the rule a write to this field runs beyond the kind's own
	// authority check, and is empty on a field whose write is a plain
	// rewrite. Every non-empty value is one of the Guard constants below.
	Guard string
}

// The guards a field write may declare. The set is closed, and
// TestEveryDeclaredGuardIsRouted holds it to the routing in internal/verb.
const (
	GuardSlug     = "slug"
	GuardLevel    = "level"
	GuardTier     = "tier"
	GuardState    = "state"
	GuardFilename = "filename"
	GuardKind     = "column-kind"
	GuardCapacity = "capacity"
)

// FieldsOf reports the fields of a kind a person wrote and may rewrite, in the
// order a listing prints them. A kind the grammar does not name reports no
// fields.
func FieldsOf(kind string) []string

// FieldOf reports one field's declaration, and whether the kind carries a
// field of that name at all.
func FieldOf(kind, name string) (Field, bool)

// EntityKinds lists every kind the containment table names, plus the
// workstream, sorted. It is what a sweep over the kinds iterates.
func EntityKinds() []string

// AllFields is the sorted union of every kind's field names, which is the
// closed set the set command's field argument declares.
func AllFields() []string

// WriteAuthorityOf reports who may write a field of this kind: AuthorityOwner,
// where any owner may, or AuthorityOperator, where the actor must be the
// workbench's operator.
func WriteAuthorityOf(kind string) string
```

`FieldsOf` keeps the signature dinah-456 section 5.3 gives it. `FieldOf` sits beside it
rather than replacing it, because the parent also requires that the prose distinction be
recorded in this one place and a list of names cannot record it. Both read one table,
`var fields = map[string][]Field`, so there is one declaration and two readings of it.

`bench.CardFields` and `bench.WorkstreamFields` are deleted and their readers point at
`FieldsOf`. `bench.KnownCardField` and `bench.KnownWorkstreamField` are deleted and their
readers call `FieldOf`. `bench.WorkbenchFields` and `bench.KnownWorkbenchField` go the same
way; `Bench.WorkbenchField` and `Bench.SetWorkbenchField` stay, because they are the
storage accessors rather than the declaration, and they gain an `instructions` arm.

## 3.1 The sets

| Kind | Fields |
|---|---|
| `workbench` | `title`, `slug`, `operator`, `instructions` |
| `column` | `title`, `slug`, `kind`, `tier`, `capacity`, `instructions` |
| `card` | `title`, `body`, `severity`, `priority`, `tier` |
| `comment` | `body` |
| `item` | `text`, `state`, `note`, `owner`, `column` |
| `attachment` | `filename`, `description` |
| `workstream` | `title`, `slug`, `status`, `notes` |

Seven kinds, and no count of the fields is written into any document. This card's
acceptance criteria compare the table against the code and against the round-trip sample
table beside it, in both directions, so a field added or dropped later moves one
declaration and one sample and nothing else.

Per field, the declaration:

| Kind | Field | Prose | Clearable | Guard | Stored as |
|---|---|---|---|---|---|
| workbench | `title` | no | no | | `title` |
| workbench | `slug` | no | no | `slug` | `slug` |
| workbench | `operator` | no | no | | `operator` |
| workbench | `instructions` | yes | yes | | the anchor's body |
| column | `title` | no | no | | `title` |
| column | `slug` | no | no | `slug` | `slug` |
| column | `kind` | no | no | `column-kind` | `kind` |
| column | `tier` | no | yes | `level` | `tier` |
| column | `capacity` | no | yes | `capacity` | `capacity` |
| column | `instructions` | yes | yes | | the anchor's body |
| card | `title` | no | no | | `title` |
| card | `body` | yes | yes | | the anchor's body |
| card | `severity` | no | yes | `level` | `severity` |
| card | `priority` | no | yes | `level` | `priority` |
| card | `tier` | no | yes | `tier` | `tier` |
| comment | `body` | yes | no | | the anchor's body |
| item | `text` | yes | no | | the anchor's body |
| item | `state` | no | no | `state` | `state` |
| item | `note` | no | yes | | `note` |
| item | `owner` | no | yes | | `owner` |
| item | `column` | no | yes | | `column` |
| attachment | `filename` | no | no | `filename` | `filename` |
| attachment | `description` | no | yes | | `description` |
| workstream | `title` | no | no | | `title` |
| workstream | `slug` | no | no | `slug` | `slug` |
| workstream | `status` | no | no | | `status` |
| workstream | `notes` | yes | yes | | the anchor's body |

## 3.2 Where the column set comes from

dinah-456 section 5.3 requires the column set to be derived rather than copied, and here is
the derivation, run against `9260a2a`. `bench.Column` at `internal/bench/bench.go:156`
carries `ID`, `Title`, `Slug`, `Kind`, `OperatorOwned`, `AwaitingOutside`, `RejectTo`,
`Tier`, `Capacity`, `LoopLimit`, `Instructions`, `Position` and `FM`. The `column` entry in
`params` at `internal/verb/definition.go:570` accepts `title`, `kind`, `tier`, `capacity`,
`slug` and `before`.

The six settable fields are the five `column new` accepts, minus `before`, which is a
position rather than a field, plus `title` and the prose body.

Seven members of the struct are not fields a person types. `ID` and `Position` are
structural, `FM` is the header itself, and `OperatorOwned`, `AwaitingOutside`, `RejectTo`
and `LoopLimit` are written by a workbench definition through `reshape` and by nothing
else. The parent's own list of these named four and missed `LoopLimit`; the derived set is
unchanged by the correction, and it is recorded here because the parent asked for a
derivation rather than a copy.

## 3.3 Which absences are deliberate

dinah-456 section 5.4's matrix says what can be done to a kind. This says what `get` and
`set` reach inside a kind, which is the finer question, and every name a kind's anchor
carries that the table above omits is listed here with the reason.

**Deliberate, on every kind: the identifier and the ordinal.** Neither is a person's to
type, both are minted by the tool, and rewriting either renames the entity out from under
every reference to it.

**Deliberate, on a card: `column`, `state`, `holder`, `claim_since`, `expires`,
`block_reason`, `block_kind`, `block_since`, `tier_at`, `links` and `workstreams`.** Each is
the record of an act, and every one of them has a verb that performs the act and journals
it. `move`, `claim`, `release`, `block`, `unblock`, `raise`, `link`, `unlink`, `join` and
`leave` are those verbs. A hand write to any of them would put the anchor and the journal
into a state no replay produces, which is the defect `dinah check`'s witness findings exist
to report. `tier_at` is reached by `set <card> tier --at <column>`, which routes through
`Library.SetCardTierAt`, so the per-column overrides are writable and the stored sequence
itself is not.

**Deliberate, on a comment: `ts` and `author`.** Both record who wrote the comment and
when, and neither is a field the writer authored.

**Deliberate, on an item: `kind`.** The item's kind is part of how the item is addressed,
because `pb-1/questions/1` and `pb-1/decisions/1` count within one kind. Changing it would
move an item between two collections and shift the position of every later member of both
in one write, and no verb re-addresses an entity today. A card wanting an item of a
different kind files one and deletes the other.

**Deliberate, on an attachment: `provenance`.** `attach` records where the bytes came from
and offers the writer no way to say, so it is the tool's record rather than the writer's.

**Deliberate, on a workbench: the columns, the levels and the profile declarations.** They
are the workbench's structure, `reshape` is the verb that rewrites them from a definition,
and a per-key write would leave the workbench disagreeing with the definition it was
reshaped from.

**Deliberate, on a workstream: nothing.** All four of its person-written fields are here.

## 3.4 Two fields the parent's table does not carry, and why they are here

dinah-456 section 5.1 is the rule: every field a person authored on any kind is readable
and writable by a command that builds a request. Section 5.3's table is that rule applied,
and applying it again turns up two names the table omits and gives no reason for. Both are
added here. If either is wrong, it is one row of one table and the reviewer should say so.

**`item.column`.** `dinah file` accepts `--column` at `internal/verb/definition.go:360` and
stores it on the item, so a person authored it. The parent's item set is `text`, `state`,
`note`, `owner`, which is `--owner` included and `--column` omitted, with nothing anywhere
saying why the two flags of one command part company. Reading that as an oversight rather
than a ruling, `column` is settable. Nothing validates it on `file` today, so nothing
validates it here either, and section 4.4 records that as the honest match rather than a
gap.

**`workstream.notes`.** A workstream's body is prose a person wrote, reachable only through
`dinah edit workstream/<ref>`, which is precisely the asymmetry this card exists to close.
The parent's workstream set is the three frontmatter keys, and its own section 5.1 rules
prose in. Leaving it out would ship the card that closes the prose gap with one prose field
still behind the editor.

# 4. What a write does

## 4.1 The order of checks

`Library.SetField` evaluates in this order, and `internal/verb/checks.go` draws the same
order as `set`'s check list on its help page:

1. the workbench designates an operator
2. the reference resolves to one entity rather than to a collection
3. the field is one the resolved kind records
4. the value is present where the field may not be cleared, and is one line where the field
   is not prose
5. the field's own guard admits the value
6. the request names an owner
7. that owner is the operator, where the kind's write authority is the operator's
8. a write to a `slug` field carries `--yes`

`Library.GetField` evaluates rows 2 and 3 and no more, because a read validates nothing.

Row 2 is free: `Bench.ResolveEntity` refuses `dinah.is-a-collection` for a reference naming
a whole collection, which dinah-455 landed at `70ff10f`. So `dinah set pb-1/comments body
x` refuses with the sentence naming the first member, and no code in this card raises that
name.

## 4.2 Who may write

`WriteAuthorityOf` declares one of two values per kind, and this card's criteria assert
that every kind carries one from that closed set.

| Kind | Authority |
|---|---|
| workbench | operator |
| column | operator |
| workstream | operator |
| card | owner |
| comment | owner |
| item | owner |
| attachment | owner |

The three workbench-level kinds keep the rule `SetWorkbench` and `SetWorkstream` hold
today, which is `not-operator` for an actor who is not the operator. A column is workbench
structure and joins them; nothing writes a column field today, so this is a rule being
minted rather than one being changed, and it is minted the way its two siblings already
are. The four card-level kinds keep the rule `SetCardField` holds today, which is that an
owner is required and any owner will do, because a classification is not a claim.

## 4.3 Clearing

A write with no value clears a `Clearable` field, and refuses `malformed` naming the field
on one that is not. The three reasons `SetWorkbench` gives at `internal/verb/beyond.go:427`
carry over unchanged: `Open` refuses a workbench whose title is empty, clearing the
operator leaves nobody who can set it again, and clearing the slug leaves every card
reachable by its identifier alone. `dinah set pb-1 severity` clears the severity, which is
what `card set` does today and is why the two commands cannot share one rule.

## 4.4 The guards

Each guard is a declared name on the field and a routing arm in `internal/verb`. This
card's criteria assert that every declared constant has an arm and that the arm set is not
empty.

One rule generates the whole table below, so a later field is placed by the rule rather
than arguing its way onto a list. **A field's write guard is the guard its create path
already applies, and a field whose create path applies none is written unvalidated.** Every
row below is that rule worked out for one field: `column-kind` and `capacity` cite the
checks `NewColumn` makes, `slug`, `level`, `tier`, `state` and `filename` each route to the
verb that already checks them, and the unguarded fields are the ones no create path checks.
A field validated on the rewrite while its create stays open would be legal to file and
illegal to correct, which is the asymmetry the rule exists to prevent.

**`slug`.** The value passes the kind's own slug grammar, which is `bench.ValidSlug` for
the workbench and `bench.ValidColumnSlug` for a column and a workstream, and a value that
does not is `malformed` naming `slug`. The write carries `--yes` or refuses
`dinah.unconfirmed` naming the value. Uniqueness is not enforced on a workstream, which
`docs/design/format.md:1513` states as the format's own rule and `dinah check` reports, and
this card does not change that.

**`level`.** `severity` and `priority` on a card, and `tier` on a column, run
`Library.admitLevels`, so a workbench declaring no set for the axis refuses
`dinah.no-levels` and a value outside the declared set refuses `dinah.unknown-level`. A
clear runs neither check, which is the rule `SetCardField` already holds and the reason it
holds it.

**`tier`.** A card's `tier` routes through `Library.SetCardTierAt` when the request carries
`--at`, and through the level guard otherwise. The relative expressions `+1` and `-1` and
the refusals `dinah.no-tier-default` and `dinah.tier-out-of-range` reach `set` exactly as
they reach `card set` today.

**`state`.** A write routes to the library verb that lands the state, and constructs
nothing of its own: `resolved` to `Library.Resolve`, `verified` to `Library.Verify`,
`failed` to `Library.Fail`, `pending` to `Library.Reopen`. So the kind check
(`dinah.wrong-item-kind`), the pending check (`dinah.not-pending`), the citation obligation
and the journal event all come from the verb rather than from a second implementation of
them. `--note` fills the note the terminal verb reads, so `dinah set pb-1/criteria/1 state
verified --note "ran the suite"` and `dinah verify pb-1/criteria/1 "ran the suite"` are one
act written two ways. A value outside the four is `dinah.unknown-value`, which section 6.3
shows renders as a whole sentence with no key minted.

**`filename`.** The write routes to `Library.Rename`, so `bench.ValidAttachmentName` admits
the name, the payload moves with it, and the journal records `attachment_renamed` rather
than any event this card mints.

**`column-kind`.** The value passes `bench.ValidColumnKind`, which is what `NewColumn`
checks at `internal/verb/columns.go:41`, and a value that does not is `malformed` naming
`kind`.

**`capacity`.** The value parses as an integer above zero, which is what `NewColumn` checks
at `internal/verb/columns.go:56`, and a value that does not is `malformed` naming
`capacity`. A clear removes the limit.

**No guard.** `title`, `operator`, `status`, `note`, `owner`, `column`, `description` and
every prose field are stored as written, by the rule at the head of this section: no create
path checks any of them. `item.column` and `item.owner` are the two worth naming, because
`dinah file` declares both at `internal/verb/definition.go:359-360` and stores both without
checking either.

## 4.5 One line, or several

A field the declaration marks `Prose` is stored as the anchor's body and may carry as many
lines as the writer sends. Every other field is a frontmatter key, so its value is one
line, and a value carrying a line break refuses `malformed` naming the field with the
`one-line` splice of section 6.2. That rule is what makes `dinah set pb-1/questions/1 note
-` safe against a multi-line pipe: it refuses rather than writing a header no parser reads
back.

# 5. What a write records

## 5.1 The rule

A write journals on the nearest enclosing journal-bearing entity, which is what
`Library.journalFor` at `internal/verb/beyond.go:643` already answers: a card's own journal
for anything below a card, the workstream's own for a workstream, and the workbench's for
everything else.

The event is the `*_updated` event of the written entity's own kind. Four exist, and this
card adds the three that do not.

| Written kind | Event | Journal | Always present | Conditional |
|---|---|---|---|---|
| workbench | `workbench_updated` | the workbench's | `field` | `from`, `to` |
| workstream | `workstream_updated` | the workstream's | `field` | `from`, `to` |
| column | `column_updated` | the workbench's | `note` (the column's id) | `field`, `from`, `to` |
| card | `card_updated` | the card's | `field` | `from`, `to` |
| comment | `comment_updated` | the card's | `note` (the comment's id), `field` | `from`, `to` |
| item | `item_updated` | the card's | `note` (the item's id), `field` | `from`, `to` |
| attachment | `attachment_updated` | the card's, or the workbench's below a column or the workbench | `note` (the attachment's id), `field` | `from`, `to` |

`column_updated` exists today and carries `note` alone, written by `reshape` one line per
changed column. It gains `field` on the lines this card writes, and `reshape`'s own lines
are unchanged and go on carrying `note` alone, which is why the table lists `field` as
conditional for that one event and always present for the three new ones.

**Changing an existing event's shape falsifies two published statements, and neither one is
checked by anything today.** Both are edited by this card, and section 5.3 names what starts
checking them.

- `docs/design/format.md:861` reads `| column_updated | note (the column's own id) | |`,
  with an empty conditional column. It gains `field`, `from` and `to` in that column, by the
  rule the `workbench_updated` row states. `scripts/derive_event_counts.py` derives five
  counts and three membership placements from the tree (`claims()` at line 139) and never
  reads a row's member list, so the script exits 0 against the stale row.
- `EventColumnUpdated`'s doc comment at `internal/contract/contract.go:626` says "Reshape is
  the only writer today". `set` becomes a second writer, so the sentence becomes false. It is
  rewritten to say that `reshape` writes one line per kept column whose rendered anchor
  changed and carries `note` alone, and that a `set` write to a column's own field writes one
  line carrying `field` beside the `note`. Nothing reads a doc comment, which is why the
  fixture work below is what actually guards the change.

**A prose field's write carries neither `from` nor `to`.** A journal records that an act
happened and who did it; the prose itself lives in the anchor, which is the file that
changed, and copying a whole instructions body into an append-only journal on every edit
would grow the journal without bound and put a second copy of the text where nobody edits
it. The line carries `field` and the entity's own name, and a reader who wants the before
text reads the git history of the anchor. `Field.Prose` is what decides, so the rule is read
off the same declaration everything else about the field is read off.

## 5.2 Why three new names rather than one reused one

The alternative considered and rejected was to write `card_updated` for everything landing
on a card's journal, naming the written entity in `note`. It costs no new event name and it
keeps the format's closed set where it is. It was rejected because `card_updated` would then
answer two questions, "the card's own field changed" and "something below the card
changed", and no reader could ask the first one alone. `contract.Events` is the vocabulary
`dinah query event:<name>` accepts, so a merged event is a query a person can no longer
write. The `*_updated` family is already per kind, and `column_updated` already sets the
precedent for a kind-named event landing on somebody else's journal and naming its subject
in `note`, so three new names follow a pattern rather than inventing one.

## 5.3 What that costs, stated so nobody discovers it at Implement

- `internal/contract/contract.go` gains three constants with doc comments in the shape the
  block already uses, and `contract.Events` gains all three, because each can land on a
  card's own journal and `cmd/dinah/compat_test.go`'s
  `TestEveryEventACardJournalCarriesIsOneAQueryCanName` requires that.
- `docs/design/format.md`'s journal event schema gains three rows and its counts move.
  Nothing in that section is edited by hand from a number in this spec:
  `python scripts/derive_event_counts.py` derives the counts from the tree and reports each
  document claim it checked, and the document is edited until that script exits 0.
- One figure in that same section is already wrong at `9260a2a`, and this card corrects it
  while the section is open. The extension paragraph at `docs/design/format.md:826` says "A
  name carrying no dot is one of the twenty-three, or one a different build wrote". No set in
  the section has twenty-three members: declared is thirty-two, written thirty-one, queryable
  twenty-nine, overlap twenty-eight, card-journal twenty-seven. The sentence is about any core
  event name a reader may meet, so the set it means is the declared one, and it reads "one of
  the thirty-five" once this card's three constants land. It has been wrong because
  `derive_event_counts.py` checks five counts and three placements and never reads it, so the
  script gains a sixth claim, `("declared count in the extension paragraph", rf"is one of the
  {word(len(sets['DECLARED']))}")`, in `claims()` at `scripts/derive_event_counts.py:139`.
  That is what stops the figure rotting again rather than being corrected a second time.
- `internal/bench/testdata/compat/populate.txt` gains a line writing each new event, since
  the sample-fixture coverage alarm reddens on a declared event no capture carries and
  `unwrittenEvents` is not the honest answer for an event this card writes.
- `populate.txt` also gains a line writing a column's own field, `set <column> title
  "<something>"`, which is a changed shape rather than a new event and therefore reaches
  none of the guards above. This is the line that makes the `column_updated` change
  checkable at all. `TestTheSampleFixtureCarriesEveryShapeThisBuildWrites`
  (`cmd/dinah/compat_test.go:200`) compares the freshly replayed tree's `members` map, which
  is the journal member names per event name, against the fixture's, so once the sequence
  writes `field` on a `column_updated` line the fixture has to carry it too.
- `cmd/dinah/compat_test.go`'s `wantedEvents` gains a `contract.EventColumnUpdated` row
  naming `ts`, `event`, `actor`, `note` and `field`. That table is a union per event, so one
  row covers both writers: `reshape`'s line supplies `note` and the `set` line supplies
  `field`. `TestReplayingThePopulationSequenceReachesEveryShapeItNames` then fails if a later
  edit drops either writer from the sequence. `column_updated` is absent from `wantedEvents`
  today, which is why the sequence's only writer of it has been unpinned.
- The compatibility fixture for the revision this build stamps is recaptured with
  `python scripts/capture_fixture.py`, and the manifest digest is re-blessed in the same
  diff. Every older revision under `internal/bench/testdata/compat/` is left untouched.
- `populate.txt`'s existing `card set`, `workbench set` and `workstream set` lines are
  rewritten as `set`, which is a rewrite of the sequence rather than of the events it
  writes.

# 6. The refusals

No refusal name is minted for `get` or `set`. Every refusal these commands raise already
exists, and each is raised by the resolver, by the library verb a guard routes to, or by
the argument parser.

| Condition | Refusal | Detail |
|---|---|---|
| the reference names nothing | `dinah.unknown-path`, or `unknown-card` for a bare head | the reference |
| the reference names a whole collection | `dinah.is-a-collection` | the reference |
| the field is not one the kind records | `dinah.unknown-field` | the field |
| the value is absent on a field that may not be cleared | `malformed` | the field |
| the value carries a line break on a field that is not prose | `malformed` | the field |
| the value fails the field's guard | the guard's own name, per section 4.4 | per guard |
| the request names no owner | `no-owner` | empty |
| the owner is not the operator, on an operator-authority kind | `not-operator` | the actor |
| a slug write carries no `--yes` | `dinah.unconfirmed` | the value |
| `--at` beside a field other than a card's `tier` | `dinah.usage` | `--at` |
| `--note` beside a field other than `state` | `dinah.usage` | `--note` |
| a third word after `get`'s field | `dinah.usage` | the word |

## 6.1 `dinah.unknown-field` gains two variants

The shape at `internal/contract/shape.go:601` declares `Variants: []string{"card", "show"}`.
`card` goes and `get` and `set` arrive, so the declaration becomes
`[]string{"get", "set", "show"}`, and the fragment list swaps the `card` next-step for two.

English, `refusal.dinah.unknown-field.get`:

```
Dinah has no field {detail} on {kind}. The fields a {kind} records are: {fields}.
```

`refusal.dinah.unknown-field.set` carries the same sentence. The two are separate entries
because the mechanism selects on the command word and because their next steps differ,
which is the whole reason a variant carries one.

`refusal.dinah.unknown-field.get.next`:

```
 Name one of those instead, or run `dinah help get` to see what the command takes.
```

`refusal.dinah.unknown-field.set.next` says the same with `dinah help set`.

`kind` joins `Values` beside `fields`, and both are filled at the raise site from
`entity.Kind` and from `strings.Join(bench.FieldsOf(entity.Kind), ", ")`. The kind token is
canonical vocabulary and travels untranslated, which is what
`TestContractTokensSurviveInBackticks` already requires of every name `Commands()` returns.

`refusal.dinah.unknown-field.card` and `refusal.dinah.unknown-field.card.next` are deleted.

## 6.2 `malformed` gains one splice

`refusal.malformed.one-line`, spliced on a filled `oneLine` value the way
`refusal.malformed.at` splices on `path`:

```
; a value for {detail} is stored on one line, and the value you gave carries a line break
```

## 6.3 `dinah.unknown-value` carries the item state, and needs no new key

`contract.UnknownValue` at `internal/contract/contract.go:264` already declares the shape
this needs. Its shape at `internal/contract/shape.go:760` carries `term`, `field` and
`legal`, and its entries read "Dinah does not accept {detail} in {term}." with the splice
" The values {field} takes are: {legal}." and the next step " Name one of those instead.".
The state guard raises it with the value as the detail, `state` as both the term and the
field, and the four states as `legal`, which renders as a whole sentence with no key minted.

`bench.ItemStates` is minted beside `bench.ItemKinds` in `internal/bench/item.go:37`, as
`[]string{ItemPending, ItemResolved, ItemVerified, ItemFailed}` over the four constants
that already sit there, for the reason `ItemKinds`' own comment gives: a surface offering a
caller the choice reads the list rather than writing the four out again.

# 7. Retirement

## 7.1 What each retired spelling now prints

Probed at `9260a2a` with a binary built from it, against a throwaway workbench:

```
$ dinah frobnicate
dinah.unknown-command Dinah offers no command called frobnicate; run `dinah help` for the list of commands, grouped by what they do
$ dinah workbench frobnicate
dinah.usage frobnicate was not understood; run dinah help for the list of commands
$ dinah workstream frobnicate
dinah.usage frobnicate was not understood; run dinah help for the list of commands
```

Both exit 2. So, per command rather than by assuming the three are alike:

- **`dinah card get` and `dinah card set` refuse `dinah.unknown-command`, naming `card`.**
  The `card` command has no act left once its two are gone, so the command itself is
  retired: its entry leaves `commands` in `cmd/dinah/commands.go`, its entry leaves
  `params`, its check list leaves `internal/verb/checks.go`, and `runCard`, `runCardGet`
  and `runCardSet` are deleted. A command the table does not define is refused by main's
  own dispatch, which is where `dinah.unknown-command` comes from.
- **`dinah workbench get` and `dinah workbench set` refuse `dinah.usage`, naming the word.**
  `runWorkbench` keeps its `""` arm, which lists the workbench's fields, and loses its
  `get` and `set` arms, so both words fall through to the `s.fail(contract.Usage, first)`
  at `cmd/dinah/commands.go:1570`. This is `runColumn`'s precedent, which refuses any first
  word but `new`.
- **`dinah workstream get` and `dinah workstream set` refuse `dinah.usage`, naming the
  word.** `runWorkstream` keeps its `""` and `new` arms and loses the other two, so both
  words reach the same fall-through.

## 7.2 What goes with them

- `Library.CardField` and `Library.SetCardField` become the card arm of the generic pair
  rather than separate commands. The code moves; nothing about what a card write checks
  changes.
- `Library.Workbench`'s `get` arm goes. The bare read stays, because
  `emitWorkbenchFields` is what `dinah workbench` prints.
- `Library.SetWorkbench` and `Library.SetWorkstream` become the workbench and workstream
  arms of the generic write.
- `Library.Workstream`, `WorkstreamDetail` and `Library.membersOf` are deleted with their
  last caller. Section 7.4 says what replaces the read they served.
- The MCP tool `card` goes, because the command behind it is gone and
  `TestEveryLibraryCommandIsServedOrExempted` fails a tool dispatching a command the verb
  table does not define. `doCard` is deleted.
- `doWorkbench` loses its `set` arm and answers the read alone. `doWorkstream` loses its
  `get` and `set` arms and answers `new` and the listing.
- The `workbench` command's params become `{}`, and the `workstream` command's become the
  action, the workstream and the slug, with the action's display narrowing from
  `new|get|set` to `new`.
- `check.card.1` to `check.card.8`, `check.workbench-field.1` to `.5` and
  `check.workstream-field.1` to `.6` are deleted, and `set` and `get` gain check lists of
  their own drawn from section 4.1.

## 7.3 What stays

The bare `dinah workbench` listing and the bare `dinah workstream` listing stay, since each
lists rather than reading a field. `add`, `column new` and `workstream new` stay as the
creating verbs. `rename` stays as the attachment verb, and `set <attachment> filename`
routes through it, so the two spellings write one journal line and nothing distinguishes
them in the record.

The bare `dinah workbench` listing keeps printing `title`, `slug` and `operator` and does
not grow an `instructions` row. It is a summary of who owns the workbench and what its
cards are called, and a listing that prints a whole instruction body is not a listing. So
`FieldsOf("workbench")` carries four names and that screen shows three, which the
references guide states in one sentence rather than leaving a reader to notice.

## 7.4 The workstream detail read, retired with the spelling

`dinah workstream get <ref>` with no field prints a field table rather than a value.
Probed at `9260a2a`:

```
$ dinah workstream get autumn
  Field   Value
  ------  ------------
  slug    autumn
  id      8beff2bfb3ea
  title   Autumn
  status  active
  cards   0
```

It goes with the spelling, and nothing it showed becomes unreachable. The bare
`dinah workstream` listing carries every column of it, probed at the same commit:

```
$ dinah workstream --json
{
  "workstreams": [
    {
      "id": "8beff2bfb3ea",
      "ref": "workstream/autumn",
      "slug": "autumn",
      "title": "Autumn",
      "status": "active",
      "cards": 0
    }
  ]
}
```

The two things the detail read adds beyond the listing are the workstream's notes and its
member cards. `dinah get workstream/autumn notes` prints the first and
`dinah query workstream:autumn` prints the second, and `dinah path workstream/autumn`
prints the directory the detail's `path` member carried.

What is lost is the narrowing, which is one row of a listing rather than a capability.
Widening `dinah show` to take a workstream head would restore the shape, and dinah-456's
D-25 already rules that widening out of this contract and names dinah-455's description as
where it would go. dinah-455 has since landed, so it is a card of its own if anybody wants
it, and this spec does not file one.

# 8. The machine surface

## 8.1 The two tools

`internal/mcp/tools.go`'s `tools` gains two entries and loses one:

```go
{name: "get_field", command: "get", run: readField},
{name: "set_field", command: "set", run: func(l *verb.Library, r *verb.Request) any { return l.SetField(r) }},
```

`readField` wraps the read as `{"value": "<the value>"}` and answers a refusal through
`l.FromError`. Both tools publish `ref`, `field` and, on the setter, `value`, `at`, `note`
and `yes`, generated from the parameter table the way every other tool's schema is. No
argument is exempted, so `argumentExemptions` gains nothing.

**The affordance list `readField` wraps with is `readAffordances`, and it is not the literal
`doCard` uses.** `readAffordances` at `internal/mcp/tools.go:528` is `status`, `columns`,
`list_cards` and `next_card`, and it is what every bench-level read in this file already
wraps with. `doCard` at `internal/mcp/tools.go:730` wraps with a bespoke
`[]string{"show", "log", "card"}` instead. Two things are wrong with copying that literal,
and the first is fatal on its own. It names `card`, which is the tool this card deletes, so
the copy would publish an affordance pointing at a tool the same change removes. And
`get_field` answers for a workbench, a column, a workstream, a comment, an item and an
attachment as well as for a card, so `show` and `log` are wrong for it wherever the
reference is not a card. `doCard` is deleted with the `card` tool, so the literal is not
left behind for a later reader to copy.

`set_field` publishes no literal at all. It returns `l.SetField(r)`, and a library response
carries its own affordances, which `internal/mcp/mcp.go:370` translates from the library's
command vocabulary into tool names through `surfaceAffordances` before the response is
served. That is the same route every other writing tool's response takes.

**Nothing in the tree asserts that an affordance names a tool this head serves, so this card
adds that check rather than relying on the correction above staying correct.** The two
existing assertions each check something else: `TestEveryToolResponseCarriesAffordances` at
`internal/mcp/mcp_test.go:244` checks only that the member is present, and the check at
`internal/mcp/mcp_test.go:302`, inside
`TestTheInstructionsListAgreesWithWhereTheCardIsStanding`, checks the opposite direction,
that an affordance is not a command spelling. Neither would have caught `card` standing in
`readField`'s list after the `card` tool went. `TestEveryToolResponseCarriesAffordances`
gains two rows, one calling `get_field` and one calling `set_field`, and inside its existing
loop every name in the response's `affordances` member is required to be a name the head's
own `tools/list` answer carries. The served set is read from a `tools/list` call in the same
test rather than hand-listed, so a tool renamed anywhere moves both sides at once. AC-11
carries it.

Neither joins `crossHeadIdentical`. The cross-head fixture at
`cmd/dinah/cross_head_test.go:165` drives a declared command by named values that the
terminal side spells as flags, and both of these take their reference and their field as
positionals, so the fixture cannot express an invocation to compare. AC-1 compares the two
heads directly instead, over every field of every kind, which is a wider comparison than
the declaration would have bought.

## 8.2 The exemption ground

This is dinah-456 section 6 and its D-17, implemented as written there.

```go
// exemption is one command this head does not serve: the ground it is held out
// on, drawn from the closed set below, and the prose a reader wants.
type exemption struct {
	ground string
	reason string
}

// The grounds an exemption may stand on. The set is closed. A fifth needs a
// card arguing for it, which is the point: the guard's job is to make an
// unargued exemption impossible rather than to decide that only one argument
// can ever be made.
const (
	GroundShellOrFilesystem   = "shell-or-filesystem"
	GroundMachineNotWorkbench = "machine-not-workbench"
	GroundTheHeadItself       = "the-head-itself"
	GroundProtocolServesIt    = "protocol-serves-it"
)

var toolGrounds = []string{
	GroundShellOrFilesystem, GroundMachineNotWorkbench,
	GroundTheHeadItself, GroundProtocolServesIt,
}

var toolExemptions = map[string]exemption{ ... }
```

The nine entries were read at `9260a2a` rather than taken from the parent's count, and the
four grounds still describe all nine. Their prose is unchanged; only the ground is added.

| Command | Ground |
|---|---|
| `path` | `shell-or-filesystem` |
| `edit` | `shell-or-filesystem` |
| `init` | `shell-or-filesystem` |
| `extract` | `shell-or-filesystem` |
| `reshape` | `shell-or-filesystem` |
| `config` | `machine-not-workbench` |
| `mcp` | `the-head-itself` |
| `guide` | `protocol-serves-it` |
| `help` | `protocol-serves-it` |

`TestEveryLibraryCommandIsServedOrExempted` keeps both directions it already holds, its
empty-reason arm reads `entry.reason`, and it gains one arm: every entry's ground is one of
`toolGrounds`. It gains no arm requiring every ground to be used, because retiring the last
member of a ground is a legitimate act and a guard refusing it would be paperwork.

The comment above `tools` loses the two paragraphs arguing `workbenches` as an exception to
a sentence that no longer says what they answer, and keeps the sentence pointing a reader
at `toolExemptions` for the current set. `workbenches` needs no ground, because it is
served rather than exempt.

# 9. Documentation, and the guards that move with it

## 9.1 The references guide

`internal/guide/guides/references.md` gains two rows in the "Which command takes what"
table:

| Command | A workbench | A column | A card | Below a card | A collection |
|---|---|---|---|---|---|
| get | yes | yes | yes | yes | no |
| set | yes | yes | yes | yes | no |

The opening figure moves from "Fifteen commands take a reference" to seventeen, and the
workstream sentence from "Six commands take a workstream" to eight, because `get` and `set`
both take one. Neither figure is typed from this spec: `TestEveryProseFigureIsDeclared` and
the ledger entry at `cmd/dinah/testdata/prose-figures.txt:75` derive the first from
`referenceCommands`, and `TestTheReferencesGuideNamesTheCommandsThatTakeAWorkstream` holds
the second against what the commands do.

`referenceProbeArgs` in `cmd/dinah/references_guide_test.go:254` gains an arm for each,
because a command it does not know is a fatal naming that command:

```go
case "get":
	return []string{"title"}
case "set":
	return []string{"title", "a new title"}
```

The guide also gains a per-kind field table, and that table is hand-written markdown held to
the code by a test. It is not generated. Nothing in this tree generates a guide: the guides
are static markdown embedded from `internal/guide/guides/`, and `scripts/` carries no
writer for one. The command table above it is the worked shape, hand-written at
`internal/guide/guides/references.md:104` and held to the roster in both directions by
`TestTheReferencesGuideTableNamesEveryCommandThatTakesAReference`, which is what dinah-457
built. A generated table could not be held that way at all, because a check comparing a
generator's output against the generator's own input passes on every input, including a
wrong one.

`TestTheReferencesGuideNamesEveryFieldOfEveryKind`, in
`cmd/dinah/references_guide_test.go`, parses the new table into a map of kind to field
names, the way `parseReferencesGuideTable` at line 220 parses the command table, and fails
in four directions: on a kind in `bench.EntityKinds()` the table draws no row for, on a row
naming a kind that is not one, on a name in `bench.FieldsOf(kind)` the row omits, and on a
name the row carries that `FieldsOf(kind)` does not. It is fatal when the table draws no
row, because a parser whose heading moved reads nothing and passes for free. A failing run
names the kind, the field, and which of the two sides is missing it. The only input it reads
that a person writes is the table, so it cannot fire against correct code. That is where a
reader discovers what a kind's fields are without guessing.

## 9.2 The quick start

`docs/quick-start.md` carries the retired spellings at lines 891, 920, 931, 942, 943 and
951, inside replayed transcripts that `cmd/dinah/quickstart_test.go` runs against live
output, so a stale line reddens a test rather than merely reading wrong. Four of the six
become the generic form directly. Line 920's `workstream set autumn status finished` and
line 931's `workstream set autumn slug autumn-2025 --yes` become `set workstream/autumn
status finished` and `set workstream/autumn slug autumn-2025 --yes`, and lines 942 and 943
are prose rather than transcript and are rewritten below. The remaining two, line 891 and
line 951, have no generic form at all, and each needs its passage rewritten rather than
re-spelled. They are the two largest edits in the document, so they are specified here
rather than left to the sweep.

**Line 891 is the workstream detail read, and no single command replaces it.** The block is
introduced by "Naming one reads its fields and the cards belonging to it:" and prints a
field table followed by a member-card listing, which section 7.4 retires whole. Two commands
replace it, one per half, so the passage becomes one sentence and one block running both:

> Reading one of its fields names the field, and the cards belonging to it come from a
> query:

```console
$ dinah get workstream/autumn status
active
$ dinah query workstream:autumn
  Card   Title                    Column
  -----  -----------------------  ------
  rel-1  Write the release notes  Done
  rel-2  Draft the changelog      Intake
[exit 0]
```

The block's existing `skip=` annotation moves onto the new block unchanged, and its reason
survives the move verbatim rather than approximately. `Library.membersOf` sorts with
`sortByArrival` at `internal/verb/beyond.go:1012` and `Library.Query` sorts with the same
function, `internal/verb/library.go:768`, reached from `internal/verb/query.go:123`. The tie
between two cards created inside one second is therefore the identical hazard under the
identical comparison. The `get` is given the prefixed reference because a bare slug no longer
resolves, and the query term is given the bare slug because a query term names a workstream
by slug or identifier rather than by reference, which `workstreamRoster` at
`internal/verb/query.go:509` builds.

**Line 951's block loses its bare-slug half, so it changes commands rather than spellings.**
The block runs `dinah workstream get workstream/autumn-2025 status` and then `dinah contents
autumn-2025`, under the sentence "The first of these two reads succeeds and the second is
refused, which is that split on one screen." The second command is the one that refuses the
bare slug; it does not take it. Re-pointing the first line at `dinah get`, which refuses a
bare slug too, would leave a block where neither command shows the accepting half, under a
guard whose own doc comment says the block exists to show that half. So the accepting half
moves to a command that still has it:

```console
$ dinah join rel-2 autumn-2025
rel-2  Draft the changelog  [Intake / ready]  workstream/autumn-2025
$ dinah contents autumn-2025
unknown-card this workbench carries no card autumn-2025; run `dinah ls` to see the cards this workbench carries
[exit 2]
```

`rel-2` left the workstream earlier in the narrative, at the `dinah leave rel-2 autumn`
block, so re-joining it here is a live act rather than a repeat. The introducing sentence
becomes "The first of these two commands takes the bare name and the second refuses it, which
is that split on one screen." The `join` line's printed output is captured from the tool
rather than typed from this spec, because `TestTheQuickStartMatchesTheTool` replays the block
against live output.

The prose at lines 942 to 943 changes meaning rather than spelling. It says today that
`dinah join`, `dinah leave`, `dinah workstream get` and `dinah workstream set` take a
workstream and nothing else, so they accept either spelling, meaning the bare slug as well as
`workstream/<slug>`. Two of those four commands are retired here, and the generic pair
replacing them does not accept a bare slug, because `Bench.ResolveReference` tries a bare head
against the columns and the cards and a workstream names its own kind in the grammar, which
`Workstream.Ref` states at `internal/bench/workstream.go:93`. So `dinah get autumn status`
refuses and `dinah get workstream/autumn status` is what a reader writes. That narrowing is
deliberate: the prefix is part of the reference, and the retired commands accepted the bare
slug only because they took a workstream and could not have meant anything else. The sentence
is rewritten to say that `dinah join` and `dinah leave` take a workstream and nothing else and
so accept either spelling, that Dinah prints the longer one everywhere, and that a command
taking any reference wants the prefixed form.

Two guards sit on that passage, and one of them moves with it.
`TestTheQuickStartShowsBothWorkstreamSpellings` at `cmd/dinah/quickstart_test.go:2017`
hard-codes the two lines `$ dinah workstream get workstream/autumn-2025 status` and `$ dinah
contents autumn-2025`, and fails when the document carries no replayed block running the
first and then the second. Both wanted lines move, to `$ dinah join rel-2 autumn-2025` and
`$ dinah contents autumn-2025`, matching the rewritten block above. The test's doc comment is
rewritten with them. It says the block exists so that "the claim the retired sentence made is
now shown in a replayed block", and after this card that claim is the one about `join` and
`leave` accepting either spelling rather than the one about the retired pair, so a comment
left alone would describe something the guard no longer holds.
`TestTheQuickStartDropsTheRetiredWorkstreamClause` at line 1999 asserts that a sentence
dinah-454 made false has not come back. This card changes nothing about that guard, and the
rewritten prose must not reintroduce the string it forbids, `A workstream names its kind in
both of those commands, and nothing else does`.

`docs/quick-start.md:105`'s "fifty-three commands" moves, because this card removes `card`
and adds two. Its ledger entry derives from `groupedCommands`, so the derivation moves with
the code and the sentence is edited to agree.

## 9.3 The sweep

The six retired spellings appear in 44 files at `9260a2a`, which is what
`grep -rIl "card get\|card set\|workbench get\|workbench set\|workstream get\|workstream set"`
over the tree reports, and most of those files are tests. The implementer sweeps rather
than editing the places this spec remembers, and reads for meaning as well as for wording,
because a passage describing the old shape without quoting a command name is invisible to a
grep. The non-test files carrying them are `docs/quick-start.md`,
`docs/design/surfaces.md:53`, `internal/bench/testdata/compat/populate.txt`, and the eight
message catalogues.

**Four further hits describe live behaviour and are edited, and the classification an
earlier draft of this section gave them was wrong.** Each of these four says what a reader
should type or what the tool does, so each becomes a false statement about a command that no
longer exists. They are named individually because two of them are Go doc comments that no
criterion on this card would otherwise reach.

- `docs/design/format.md:867`, inside the `tier_overridden` row: "both absent on an ordinary
  `card set ... --at` write" becomes "both absent on an ordinary `set <ref> tier <value>
  --at` write".
- `docs/design/format.md:1762`: "A deliberate downward or lateral correction is `dinah card
  set <ref> tier <value> --at <column>`" becomes the same sentence spelling the command
  `dinah set <ref> tier <value> --at <column>`.
- `internal/verb/raise.go:24`: "a deliberate downward or lateral correction is `card set
  <ref> tier <value> --at <column>`, which is unrestricted and which this verb does not
  replace" takes the same substitution.
- `internal/verb/tier.go:12`: `SetCardTierAt`'s doc comment, "It is what `dinah card set
  <ref> tier <expr> --at <column>` reaches, where the same command without --at writes the
  card's own baseline through SetCardField", is rewritten for the generic spelling and for
  the fact that `SetCardField` becomes the card arm of `SetField` rather than a command of
  its own.

The remaining hits in `internal/bench/bench.go:485`, `internal/bench/library.go:172` and
`:746`, `internal/verb/read.go:1526` and `:1533`, `internal/verb/tree.go:336` and `:459`, and
`docs/spec/core-profile.md:1273` genuinely are unrelated word pairs, of the shape "the
workbench setting's" and "the card set", and each is checked again on the day rather than
trusted from here.

**AC-10's walk grows to cover Go source, because the two doc comments above sit where its
walk does not reach.** As drafted it walked `internal/guide/guides/`, `internal/msg/locales/`,
`docs/quick-start.md`, `docs/design/` and the embedded help, so it would have caught the two
`format.md` hits and neither of the two Go ones. That is the finding rather than the two
edits: a spec that classifies a hit as needing no edit and provides no check for it has told
the implementer to skip something twice over. The walk therefore adds every non-test `.go`
file under `internal/` and `cmd/`, on the same allowlist terms the rest of the walk uses. The
word-pair false positives listed above are what the allowlist is for; there are eight of them
at `9260a2a`, each costing one entry and the reason it is legitimate, and the allowlist is
what makes a ninth an argument rather than a silent match. Test files stay outside the walk
because they invoke the retired spellings as arguments by the hundred, and section 9.3's
count of those invocations is how the sweep reaches them instead.

The call sites inside the tests were counted rather than estimated. A grep for the argument
pairs `"card", "get"`, `"card", "set"`, `"workbench", "get"`, `"workbench", "set"`,
`"workstream", "get"` and `"workstream", "set"` over `--include=*_test.go` reports 78
invocations in eight files under `cmd/dinah/`, and a grep for `CardField`, `SetWorkbench`,
`SetWorkstream` and a literal `get` or `set` action over the tests under `internal/`
reports 20 more. `cmd/dinah/row_sweep_test.go` is one of the eight, and the workbench's own
standing note says one fixture in this repository still keys on source line numbers with
nothing announcing it, so the whole suite runs after the sweep rather than the packages
that were touched.

## 9.4 The catalogues

All eight files under `internal/msg/locales/` move together, under the workbench document
"Translation staleness contract": `de` and `hi` translated, `cs`, `id`, `es`, `fil` and `af`
carried as skeletons holding the English text, every translated entry stamped with
`msg.Fingerprint` of its English, and one `decision`-kind checklist item per changed key on
the implementing card.

New keys: `cmd.get.summary`, `cmd.set.summary`, `param.get.ref.summary`,
`param.get.field.summary`, `param.set.ref.summary`, `param.set.field.summary`,
`param.set.value.summary`, `param.set.at.summary`, `param.set.note.summary`,
`refusal.dinah.unknown-field.get`, `refusal.dinah.unknown-field.get.next`,
`refusal.dinah.unknown-field.set`, `refusal.dinah.unknown-field.set.next`,
`refusal.malformed.one-line`, and one `check.get.N` or `check.set.N` per row of section
4.1's two lists.

Deleted keys: `cmd.card.summary`, `param.card.action.summary`, `param.card.field.summary`,
`param.card.value.summary`, `param.card.at.summary`, `refusal.dinah.unknown-field.card`,
`refusal.dinah.unknown-field.card.next`, `check.card.1` to `check.card.8`,
`param.workbench.action.summary`, `param.workbench.field.summary`,
`param.workbench.value.summary`, `check.workbench-field.1` to `.5`,
`param.workstream.field.summary`, `param.workstream.value.summary`, and the members of
`check.workstream-field.*` whose rules `workstream new` does not raise, which the
implementer reads off `NewWorkstream` rather than assuming.

Reworded keys: `cmd.workbench.summary`, `cmd.workstream.summary` and
`param.workstream.action.summary`, each of which currently promises acts that are going. A
reworded English key restamps its `de` and `hi` fingerprints and refreshes the five
skeletons, because a skeleton holding older English fails
`TestASkeletonEntryReallyCarriesTheEnglishText`.

`param.card.summary` and `param.workstream.summary` are shared sentences used by many
commands and stay untouched.

# 10. dinah-461, which is being specced beside this card

dinah-461 adds `restore`, and `restore` takes a reference. Both cards therefore change the
same four derived figures: the references guide's opening figure, the quick start's
grouped-command figure, the guide's command table, and the guide's workstream sentence.
`restore` takes any reference including a workstream, so "Six commands take a workstream" at
`internal/guide/guides/references.md:123` goes to eight on this card and to nine when both
have landed. It self-corrects the same way the others do, because
`TestTheReferencesGuideNamesTheCommandsThatTakeAWorkstream` holds it against what the commands
accept. Whichever card lands second re-derives rather than re-counting, since every one of
those four figures has a derivation behind it and none of them is typed. The conflict a merge
will actually show is textual, in `references.md`'s table and sentence and in the two ledger
lines, and it is resolved by taking both rows and re-running `go test ./cmd/dinah/`.

**Neither card is a precondition of the other, and dinah-461's description says otherwise.**
That description ends "This card wants dinah-460's journalling of field writes in place
first." The ordering is free in both directions. dinah-461 adds a `restore` verb writing the
already-declared `restored` event, and it reaches no field, so it neither reads nor needs
anything this card's journalling rule establishes; and this card writes no `restored` line and
touches no archival path. Two reviewers have now reached that answer independently. The stale
text is on dinah-461, which stands in Spec, so this spec states the finding rather than
editing another card, and the card comment accompanying this revision says so plainly for
whoever works dinah-461 next.

Nothing else collides. dinah-461 writes `restored`, which is a declared event this card
does not touch, and it generalises `--archived`, which reaches no field.

# 11. Out of scope

- **Widening `dinah show` to take a workbench or a workstream head.** dinah-456 D-25 rules
  it out of the contract, and section 7.4 says what that costs here.
- **A field predicate in a reference.** dinah-456 D-1 settled it: `query` selects by field
  and DinahPath addresses.
- **`edit`.** It stays exactly as it is, and stops being the only route to any field.
- **Repairing what `attach` wrote under checklist items before dinah-459 landed.**
- **Enforcing workstream slug uniqueness.** The format declares it unenforced and `check`
  reports it.
- **A fifth exemption ground.** Section 8.2 declares four and closes the set.

# 12. The files this card touches

`internal/bench/fields.go` (new), `internal/bench/card.go`, `internal/bench/item.go`,
`internal/bench/workstream.go`, `internal/bench/bench.go`, `internal/bench/containment.go`,
`internal/verb/beyond.go`, `internal/verb/definition.go`, `internal/verb/checks.go`,
`internal/verb/raise.go`, `internal/verb/tier.go`, `internal/contract/contract.go`,
`internal/contract/shape.go`, `internal/mcp/tools.go`, `internal/mcp/roster_test.go`,
`internal/mcp/mcp_test.go`, `cmd/dinah/commands.go`, `cmd/dinah/compat_test.go`,
`cmd/dinah/references_guide_test.go`, `cmd/dinah/quickstart_test.go`,
`cmd/dinah/testdata/prose-figures.txt`, `scripts/derive_event_counts.py`,
`internal/guide/guides/references.md`, `docs/quick-start.md`, `docs/design/format.md`,
`docs/design/surfaces.md`, `internal/bench/testdata/compat/populate.txt`, the
current-revision fixture and its manifest digest, the eight catalogues under
`internal/msg/locales/`, and the test files this card's acceptance criteria name.

Five entries are new in this revision, and each answers a review finding rather than a change
of scope. `internal/verb/raise.go` and `internal/verb/tier.go` carry doc comments naming a
retired command. `cmd/dinah/compat_test.go` carries the `wantedEvents` row that pins the
changed `column_updated` shape. `internal/mcp/mcp_test.go` carries the affordance check.
`scripts/derive_event_counts.py` gains the sixth claim that holds the extension paragraph's
figure. `internal/contract/contract.go` was already on the list for the three new constants,
and it now carries `EventColumnUpdated`'s doc comment as well.

## Branch

dinah-460-get-and-set-reach-every-field-of-every-kind-so-an-agent-can-change-prose-a-person-can-change
