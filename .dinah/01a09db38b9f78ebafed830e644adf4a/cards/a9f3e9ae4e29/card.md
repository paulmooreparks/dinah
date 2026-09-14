---
title: Nothing turns a column's hold on except editing the file by hand
column: b69abf918c42
state: ready
severity: major
priority: now
tier: frontier
workstreams:
  - b3f924406e4c
---
The operator asked for this on 2026-09-10, after Test reported having to edit a column's definition directly because the tool exposes no way to do it.

dinah-450 gave a workbench the ability to hold a card at one of its own steps until a named item is settled, and a column declares that it does so with a single flag in its own definition. The spec said the flag is hand-edited and named no command, the operator approved that at his design station, and it was the right call for a card already carrying enough. It leaves the mechanism reachable only by somebody willing to open a file and edit frontmatter, which is not how anyone is expected to use this tool and is emphatically not how the first external workbench will meet it.

**What to build.** A way to turn a column's hold on and off from the command line, and a way to see whether it is on.

**His constraint, and it is the one that shapes the whole card: follow the style of the commands that already exist.** Establish that style by reading the tool rather than by inventing it. Look at how a column is already created, renamed and reshaped, at how a card's tier is set at a column, and at how any other declared property of a column is written today. The question of whether this is a new verb, a subcommand, or a flag on something that already exists is answered by what the tool already does, not by what reads well in isolation. Say in the spec which existing commands you read and which one this follows, and where it departs from them if it must.

**What the design has to settle beyond the shape.** Whether the same command turns the hold off, and what the tool says when asked to turn on a hold that is already on. Whether the state is readable, since a hold that cannot be seen is the same trap this project has just spent two cards closing: dinah-473 landed because a hold could silently fail to fire, and a hold nobody can inspect fails the same way for a different reason. Whether the change is journalled, given that the board records what happens to a card and this is a change to the board itself. And whether it is the operator's alone, which needs care because Dinah records who an item is for and enforces nothing today, so an authorisation claim would be a claim the tool cannot keep.

**The constraint that governs any wording.** The operator ruled on 2026-09-09 that Dinah must stay usable for workbenches with no code, no merge and no tests, and that this board's vocabulary must not reach the tool. The stored flag has a name already; the words a person types and reads do not have to match it, and should be chosen for somebody running a renovation rather than a pipeline. A person who wants a step to wait until its questions are answered should be able to find this command by guessing at it.

**What is not this card.** dinah-472 adds a fifth item state for letting a failed criterion through deliberately. dinah-474 fixes the same silent-storage defect on a second write path. Neither is required for this, and this card should not wait for either.

## Specification

## What this follows, and why

I read `cmd/dinah/commands.go` (the whole command table, `init`), `cmd/dinah/help.go`, `internal/verb/fields.go`, `internal/bench/fields.go`, `internal/verb/columns.go` (`dinah column new`), `docs/design/format.md`, and `docs/spec/core-profile.md`. I also built the binary from a fresh worktree cut from `origin/main` (commit `243285c`) and ran it against a scratch workbench to confirm the shapes below (`get`/`set` help text, refusal wording, operator enforcement, machine output) rather than reading them off the source alone.

Three things about a column are already declared once, in one file, and read from there by two heads and three write paths: `internal/bench/fields.go`'s `fields[bench.KindColumn]` table names every field a column carries (`title`, `slug`, `kind`, `tier`, `capacity`, `instructions`), and `dinah get <ref> <field>` / `dinah set <ref> <field> <value>` are the one pair of commands that read and write any of them, for any of the seven entity kinds the tool knows, not just columns. `capacity` is the closest precedent to this card: it is typed as `capacity` but stored under the frontmatter key `wip_limit` (`Field.Key`, `internal/bench/fields.go:15-19`), it is `Clearable`, and it declares a `Guard` (`GuardCapacity`) that validates the typed value before it is written (`internal/verb/fields.go:194-198`).

`gate_items` (dinah-450) is a column-level boolean exactly like `wip_limit`/`capacity`, and it is missing from that one table today. That is the whole gap this card closes: not a new verb, not a new subcommand, not a flag bolted onto `dinah column`. `dinah column`'s own doc comment (`cmd/dinah/commands.go:1811`) says plainly it authors a column at creation and nothing else ("Only new is implemented"), and `docs/design/format.md:450` already says outright that `operator_owned`, `awaiting_outside` and `gate_items` "are not settable at creation; write them into the column's own file by hand, as before" — creation is `column new`'s job, and turning the hold on and off *after* a column exists is `get`/`set`'s job, the same as it already is for `tier` and `capacity`. Adding one `Field` entry to the existing table, with its own guard, is the form that matches what the tool already does; everything else below follows from that one placement decision.

## The field

Add to `fields[bench.KindColumn]` in `internal/bench/fields.go`, positioned after `capacity` and before `instructions` (mirroring the existing order: name, then the settings a column declares, then the prose body last):

```go
{Name: HoldField, Key: "gate_items", Guard: GuardHold}
```

- **Typed name:** `hold`. It is the word the operator herself used when she asked for this ("turn a column's hold on and off"), it is a plain English verb a renovation-project or household user already has (holding a step until a question is answered), and it collides with nothing else the grammar names: no other kind declares a field called `hold`, and the only other place the word `Holder` appears in the codebase is a card's claim holder, a different kind, reached through a different reference, never in the same command.
- **Stored key:** `gate_items`, unchanged from dinah-450. This card does not touch the stored representation, the strict `"true"`/`"false"`-only parse `internal/bench/bench.go:1845-1851` already enforces (added specifically so a hand-typed `gate_items: yes` fails loud instead of silently holding nothing), or CORE-JSON-10/the interchange format. A workbench some other tool wrote by hand, or that dinah-450 already wrote, reads exactly as it did before.
- **`Clearable: false`.** A bare `dinah set <column> hold` (no value) refuses rather than doing anything, through the existing generic check in `SetField` (`internal/verb/fields.go:76-78`, `if value == "" && !field.Clearable`). This is deliberate: the card's own design constraint is "an operation that silently does nothing is the shape this project has just spent two cards closing," and an empty value is exactly the lazy answer that shape hands you if you let it.
- **Not settable at `dinah column new`.** `docs/design/format.md:450`'s sentence stays true for `gate_items` specifically only in the sense that creation-time is still out of scope; the fix below corrects what that sentence claims about *every later act*, which is no longer true once this lands.

## Values: `on` and `off`, and the on/off ↔ true/absent boundary

The person types and reads `on` / `off`. The frontmatter continues to carry `gate_items: true` or no key at all, exactly as dinah-450 left it. Nothing else in the tool needs to know both spellings exist; the translation is local to the write and the read of this one field.

**Write (`dinah set <column> hold on|off`).** Add a new guard:

```go
// GuardHold = "hold"
```

routed inside `admitFieldValue`'s switch (`internal/verb/fields.go:176-199`) alongside `GuardCapacity`/`GuardKind`:

```go
case bench.GuardHold:
    if value != "on" && value != "off" {
        return l.refuse(req, entity.Card, contract.Malformed, field.Name)
    }
```

This is the same shape `GuardCapacity` and `GuardKind` already use: validate the typed value, refuse with `contract.Malformed` and the field's own name when it fails. No new refusal name and no new catalog string are needed — `refusal.malformed`'s existing text (`"{detail} is missing, empty, or will not parse; write the command as \`dinah {usage}\` and try again"`) already reads correctly with `detail="hold"`, and I confirmed the sibling case (`dinah set <col> bogus-field x`) already produces exactly this generic, plain-English shape against a live build.

Immediately before `SetField` calls `writeField` (`internal/verb/fields.go:108`), when `field.Guard == bench.GuardHold`, remap the now-validated value to its stored form: `"on"` → `"true"`, `"off"` → `""`. `writeField` already treats an empty value as "delete the key" (`internal/verb/fields.go:261-264`), which is exactly how `bench.go`'s strict parser already reads an absent `gate_items` key: as not holding, identically to an explicit `gate_items: false`. So "off" clearing the key rather than writing `gate_items: false` changes nothing observable and never produces two on-disk spellings of "off."

**Read (`dinah get <column> hold`).** `readField` (`internal/verb/fields.go:206-215`) returns the raw stored value for every other field. For this one, translate on the way out: stored `"true"` reads back as `"on"`; anything else (`""` or `"false"`) reads back as `"off"`. This is the only field whose typed and stored spellings differ, so it is the only one that needs this pair of translations; every other field's `readField`/`writeField` path is untouched.

**Idempotent write.** `writeField` already treats writing a field to the value it already carries as a successful no-op: nothing is written, nothing is journalled, and the response still reports ok (`internal/verb/fields.go:266-270`, and its own comment: "on the terms join already returns ok for a workstream the card already belongs to"). This is the existing answer to "what happens when you turn on a hold that is already on": `dinah set <column> hold on` against a column already holding succeeds, changes nothing, and the machine response's `detail` still reads `"on"`. I confirmed this exact idempotent-success shape against a live build using `capacity` (`set intake capacity 5` twice: both calls exit 0, both report ok). This is not a new decision this card is inventing; it's the field inheriting the same rule every other field on this surface already keeps, which is precisely "no operation that silently does nothing" — the two calls are provably identical writes, not two different things that happen to look the same.

**Response detail.** `wroteField` sets `response.Detail` to the value it just wrote (`internal/verb/fields.go:295-303`), which for `GuardHold` at that point in the pipeline is the stored form (`"true"`/`""`). Remap it back to `"on"`/`"off"` before returning, the same translation the read path performs, so a caller scripting against `--json` sees the vocabulary it typed rather than the storage format underneath it.

## Authorization: already the operator's, and already enforced

`writeAuthority[bench.KindColumn] = bench.AuthorityOperator` (`internal/bench/fields.go:159-161`) already governs every column field write, `title` and `capacity` included: `SetField` refuses `not-operator` when the actor is not the workbench's designated operator (`internal/verb/fields.go:95-97`). Because `hold` becomes a column field through the same table, it inherits this rule automatically — no new authorization code, and no new claim about who may act. I confirmed this live: `dinah set intake title "Nope" --actor someoneelse` against a workbench whose operator is `prober` refuses `not-operator: this action is the operator's, and you are someoneelse; ask the operator to run it, or run \`dinah whoami\` to see who Dinah takes you to be`, while the same call as `--actor prober` succeeds.

This is the honest answer to the card's caution about authorization, and it is a different situation from the one the caution names. The caution is about a checklist item's `owner` field (andon-753 elsewhere, dinah-753/dinah-473's neighborhood here): Dinah records who an item is *for* and enforces nothing against who calls the verb that resolves it. Column write authority is a separate, older, already-enforced mechanism (`WriteAuthorityOf`/`req.Actor` vs `Bench.Operator`), unrelated to item ownership, and it already gates `hold` the moment `hold` is a column field, with zero new work. The limitation that does carry over honestly: this is a single-seat local tool, and `--actor`/the resolved actor is self-declared rather than authenticated, exactly as it is for every other operator-owned write today. Saying "only the operator may do this" would be true of the write; it is not a stronger guarantee than `title` or `capacity` already carry, and this spec makes no claim beyond that.

## Readability

`dinah get <column> hold` (any actor — `GetField` is open to anybody, `internal/verb/fields.go:12-14`, unchanged) reports `on` or `off`, one line, scriptable, over CLI and over MCP's `get_field` tool alike (`internal/mcp/tools.go:99-100` calls the same `Library.GetField`). This is the direct fix for the readability half of the card ("a hold nobody can inspect fails the same way as a hold that silently does not fire"): the state was invisible outside a text editor before this card, and after it, it is one command anyone on the workbench can run.

**Not added to `dinah columns`'s table.** I checked: `capacity` and `tier`, two column-authored settings of exactly this kind, are already absent from `dinah columns`'s listing (`cmd/dinah/render.go:264-292` draws slug/name/kind/cards/work/owner and nothing else) and from `dinah show`, which is card-scoped. `dinah get` is the tool's existing answer to "what does this one column declare," for every field, `hold` included. Adding a dedicated indicator to the `columns` table would be new work the card does not ask for and a special case for this one field that `tier`/`capacity` do not get; leaving it out is consistent with the surface as it stands, not an omission.

## Journalling

`writeField` already appends one journal event per field write, unconditionally, for every kind (`internal/verb/fields.go:271-284`, event name `column_updated` for a column, `internal/contract/contract.go:636`), carrying `field`, `from`, `to`, and the actor. A `hold` write is journalled exactly as a `capacity` or `title` write is: no special-casing needed, no opt-out exists to accidentally take. `from`/`to` on that event carry the stored form (`""`/`"true"`), not `on`/`off` — I chose not to translate the journal event, because no CLI command today reads a column's journal back to a person at all. I confirmed this: `dinah log` is card-scoped and refuses `unknown-card` when pointed at a column, and `dinah changes --column <col>` reports only a boolean "did anything change" cursor, not field-level detail, for `capacity` exactly as much as it would for `hold`. The event is on disk, in the vocabulary every other field's journal entry already uses, and building a column-journal reader is a separate, larger piece of work this card does not need and was not asked for.

## Documentation fix

`docs/design/format.md:450` currently reads: `` `operator_owned`, `awaiting_outside` and `gate_items` are not settable at creation; write them into the column's own file by hand, as before. `` That sentence becomes false for `gate_items` the moment this lands: hand-editing is no longer the only way. Reword to something carrying the same meaning for the two fields still true of and the corrected claim for the third, e.g.: "`operator_owned` and `awaiting_outside` are not settable at creation or afterward through any command; write them into the column's own file by hand. `gate_items` is not settable at creation, but `dinah set <column> hold on|off` writes it afterward, and `dinah get <column> hold` reads it back." Land this edit in the same change that adds the field.

## Out of scope

- **Setting `hold` at `dinah column new`.** The card asks for turning it on/off and reading it; creation-time is untouched, and `docs/design/format.md`'s "not settable at creation" claim stays true for this field in that one respect.
- **CORE profile / interchange changes.** `gate_items` already travels through `export`/`extract` (CORE-JSON-10, dinah-core 0.13) exactly as before; this card adds a CLI/MCP surface over an existing stored field, not a new stored field, so no CORE-profile revision entry applies.
- **dinah-472 (a fifth checklist-item state) and dinah-474 (a second silent-storage write path).** Named in the card as explicitly not this card's job; nothing here depends on either landing first.
- **A column-journal reader.** Named above; `hold`'s journal entry is exposed exactly as every other column field's already is (nowhere, via a CLI command), and adding one is out of scope.

## Acceptance criteria

1. `bench.FieldsOf("column")` includes `"hold"`, and `bench.FieldOf("column", "hold")` reports a field whose `Key` is `"gate_items"`, whose `Guard` is the new `GuardHold` constant, and whose `Clearable` is `false`. — gate: Merge
2. `dinah set <column> hold on` against a column not currently holding writes `gate_items: true` to that column's `column.md` and nothing else in its frontmatter changes; a subsequent `dinah get <column> hold` on the same column returns `on`. — gate: Merge
3. `dinah set <column> hold off` against a column currently holding (`gate_items: true` on disk) removes the `gate_items` key from that column's frontmatter entirely (not `gate_items: false`); `dinah get <column> hold` on the same column then returns `off`, and a column that never had the key set also returns `off`. — gate: Merge
4. `dinah set <column> hold on` run twice in a row against the same column: the second call succeeds (exit 0, `outcome: ok`), and no journal event is appended for the second call. Verify by comparing the column's journal file's line count before and after the second call. — gate: Merge
5. `dinah set <column> hold` with no value, and `dinah set <column> hold maybe` (or any value other than exactly `on`/`off`), both refuse with `malformed`, exit code 2, naming `hold` as the detail; neither call changes the column's frontmatter. — gate: Merge
6. `dinah set <column> hold on` run by an actor who is not the workbench's designated operator refuses `not-operator`, exit code 2, and changes nothing; the same call run as the operator succeeds. `dinah get <column> hold` run by a non-operator actor succeeds (reading is open to anybody). — gate: Merge
7. A successful `dinah set <column> hold on|off` appends exactly one `column_updated` journal event carrying `field: "hold"`, and `from`/`to` values that are the stored form (`""`/`"true"`), not `on`/`off`. — gate: Merge
8. `dinah set_field` / `dinah get_field` over MCP (the same tool names `internal/mcp/tools.go:99-100` already exposes) produce the same behavior as the CLI verbs for the `hold` field on a column reference, with no MCP-specific code added beyond what `internal/verb/definition.go:164`'s `bench.AllFields()`-derived schema already threads through automatically. — gate: Merge
9. `docs/design/format.md`'s sentence about `gate_items` not being settable is corrected to describe the new command, and no other passage in that file or in `docs/spec/core-profile.md` is left asserting that hand-editing is the only way to change a column's hold. Search both files for `gate_items` and confirm every remaining mention is either the CORE-JSON-10 interchange description (unaffected) or the corrected creation-time sentence. — gate: Merge
10. `TestEveryDeclaredGuardIsRouted` (`internal/verb/fields_routing_test.go`) passes with `GuardHold` added to both the `bench.Guards` closed list and a `case bench.GuardHold:` arm in `internal/verb/fields.go`; `TestEveryKindDeclaresItsFieldsAndItsAuthority` and `TestAllFieldsIsTheSortedUnionOfEveryKind` (`internal/bench/fields_test.go`) pass unmodified against the new field, since both are derived from the declaration table rather than hard-coding a field list. — gate: Merge

## Decisions

- **This follows `get`/`set`, not a new verb, not a subcommand of `dinah column`, and not a flag on `dinah column new`.** Resolved above under "What this follows, and why": `get`/`set` is the tool's one existing mechanism for reading and writing a declared column property after creation, `capacity` is the direct precedent (typed name differs from stored key, `Clearable`, carries a `Guard`), and `dinah column`'s own doc comment says it authors only at creation. — gate: Agent Design Review
- **Typed field name is `hold`; typed values are `on`/`off`.** Resolved above under "The field" and "Values": `hold` is the operator's own word for the mechanism and collides with nothing else the grammar names; `on`/`off` are her own words too ("turn a column's hold on and off"), plain household vocabulary, and distinct from the stored `true`/absent representation, which is left untouched. — gate: Agent Design Review
- **`off` clears the `gate_items` key rather than writing `gate_items: false`.** Resolved above under "Values": the strict parser in `bench.go` already reads an absent key and an explicit `false` identically, so this never produces two on-disk spellings of "off," and it reuses the `Clearable` machinery `writeField` already has rather than adding a second write path. — gate: Agent Design Review
- **No new authorization mechanism; `hold` inherits the existing operator-only write rule for column fields.** Resolved above under "Authorization": confirmed live against a build, that this is the same rule already gating `title`/`capacity`/`kind`/`tier`, not a new claim, and the card's caution about unenforced ownership concerns a different subsystem (checklist-item `owner`) entirely. — gate: Agent Design Review
- **The journal event is not translated to on/off vocabulary; no column-journal CLI reader is added.** Resolved above under "Journalling": confirmed live that no existing command (`log`, `changes`) surfaces column-level journal detail to a person for any field today, so there is nothing for this card to make consistent, and building that reader is separate, larger, unrequested work. — gate: Agent Design Review
- **`dinah columns`'s table is not extended with a hold indicator.** Resolved above under "Readability": confirmed live that `capacity` and `tier`, the closest existing precedents, are already absent from that table; `dinah get` is the existing answer to "what does this column declare" and `hold` uses it exactly as they do. — gate: Agent Design Review

## Expect an Operator Design Review stop

Triage predicted this and it is right, for the reason it named: this mints a command surface (a new field name and two new values a person will type), and the operator approves those. The transcript that review produces is not built yet — this spec has no artifact file, because nothing here is a UI mockup; the "transcript" is the command shapes fixed above (`dinah set <column> hold on|off`, `dinah get <column> hold`) plus the exact refusal and idempotent-success behavior, all derived from live runs against existing fields (`title`, `capacity`) rather than invented. **The one-line acceptance criterion his approval would create:** "The operator approves `hold`/`on`/`off` as the vocabulary this command uses, in place of `gate_items`/`true`/`false`." If he wants different words, that is a one-line change to the `Field` declaration and the two guard cases; nothing else in this spec depends on the specific spelling chosen.
SPECEOF

## Branch

dinah-477-nothing-turns-a-columns-hold-on-except-editing-the-file-by-hand
