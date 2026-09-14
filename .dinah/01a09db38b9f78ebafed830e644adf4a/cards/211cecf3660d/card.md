---
title: A checklist item's column is never validated, so an item can hold a card out of a column that does not exist
column: b69abf918c42
state: ready
severity: major
priority: next
tier: workhorse
workstreams:
  - 7c54146c716f
links:
  - kind: relates_to
    to: ea54fdf1efc1
  - kind: relates_to
    to: 4da8a08aa404
  - kind: relates_to
    to: 5d4bd80c266a
  - kind: relates_to
    to: a9f3e9ae4e29
---
A checklist item can carry a column, and that column is what decides where the item holds a card. Nothing validates it. `dinah file --column` accepts anything, and so does the generic write dinah-460 adds, so an item can be filed against a column identifier that resolves, a column slug that never will, or pure garbage, and the tool says yes to all three.

The consequences differ in a way that makes this worse than a missing check. An item filed with a column's identifier holds a card out of that column, which is the intended behaviour. The same item filed with that column's slug holds nothing at all, silently, because the gate compares against the identifier and a slug can never match it. So a reader who types the spelling every other surface prints gets an item that looks like a gate and is not one. A write can also lift a live hold silently by replacing a matching value with a non-matching one.

Established by running rather than reading, twice: once by the implementer of dinah-460 while ruling whether that card should validate the field, and once by its reviewer, who found the gap wider than first reported. Leaving it alone on dinah-460 was the right call, because fixing only the new write path would leave the two paths disagreeing about one field, which is a worse state than both being wrong the same way.

What the card should settle, none of it the operator's to rule on:

The subject is the field, not one command that writes it. Both write paths validate or neither does, and they must agree about what a legal value is. Say what a legal value is: an identifier only, or an identifier and a slug with the slug resolved on write, or something else. The reference grammar accepts both spellings for a column elsewhere, and dinah-454 established that a printed address is what a reader types, so a field that silently rejects the printed form is the odd one out.

What happens to items already carrying an unresolvable value. They exist on real workbenches, and a validating write will not find them, because nothing rewrites a value nobody touches. `dinah check` is the natural home for that sweep, which makes this card touch the surface dinah-462 is about; read that card before choosing a shape.

Whether a silently lifted hold should be reported. A write that turns a live gate into a dead one is a state change nobody is told about, and that is the same family as the read failures this workstream has been closing: an absence of effect arriving as a success.

Related: dinah-460 built the generic write and deliberately left this alone; dinah-462 covers the check surface reporting an unreadable directory as clean; dinah-450 gave a checklist item's column real teeth at move time, which is what makes an unresolvable value consequential rather than inert.

## Specification

## Where this was read

Read against `origin/main` at `ab5debc` (dinah-477, already merged), from a worktree at `C:/dinah-scratch/dinah-474-spec/wt`. No build was needed; every claim below is a direct citation.

`dinah-473` landed already. Its ruling is real, not inherited by assertion: `internal/verb/checklist.go`'s `File` (lines 45-113) resolves `req.Column` through `l.Bench.ColumnByRef` before writing, and refuses `contract.UnknownColumn` when it does not resolve (lines 71-75). This closes exactly one of the field's two write paths. Triage's view holds: this card applies the same rule to the second path rather than re-deciding what a legal value is.

## The two write paths, and where they now disagree

A checklist item's column is a frontmatter field, `column`, declared once in `internal/bench/fields.go`:

```go
KindItem: {
    ...
    {Name: ItemColumnField, Clearable: true},
},
```

No `Guard` is set. Two things write it:

1. **`File`** (`internal/verb/checklist.go:45`) — the create path. Resolves and refuses, per dinah-473, before calling `bench.AddItem`.
2. **`dinah set <item> column <value>`**, which resolves to `SetField` (`internal/verb/fields.go:49`), the generic get/set grammar dinah-460 built for every kind. Because `ItemColumnField` carries no `Guard`, `SetField` never calls anything that checks the value: `admitFieldValue` (`internal/verb/fields.go:191-217`) only runs a case for a field that declares one, and none is declared, so the switch falls through and `writeField` (`internal/verb/fields.go:288`) stores whatever string was typed, verbatim, under `fm.Set("column", value)`.

This is the second path the card names. A value filed through `File` is always either empty or a resolved column identifier. A value written through `dinah set` can be anything: an identifier, a slug, a title, or garbage, exactly as the card describes, and nothing here has changed since the card was filed — dinah-473's fix touched `File` alone, deliberately (its own description: "fixing only the new write path would leave the two paths disagreeing").

## What consumes the field, and why a resolvable value can still be a dead gate

`internal/bench/entity.go:997`, `GatingItems`:

```go
func GatingItems(cardDir, columnID string) []*Item {
    ...
    return itemsWhere(cardDir, func(item *Item) bool {
        return item.Column == columnID && !ItemLiftsColumnHold(item)
    })
}
```

`internal/verb/mutate.go:401` calls this with `destination.ID` (the column's own identifier) as `columnID`. The match is Go string equality against an identifier, not a resolution. This is the whole reason the bug in the card's second paragraph exists: an item stored with the column's slug, or its title, or anything that is not byte-identical to that column's ID, will never satisfy `item.Column == columnID`, no matter how sensible the stored value looks to a person reading it.

Compare this to `card.ColumnTiers`' own per-column override, the closest sibling in the codebase (`internal/bench/card.go:322,345,369`, `internal/verb/reshape.go:1117`): every comparison against a `ColumnTier.Column` goes through `b.ColumnByRef(...)`, never through raw equality. `tier_at` never had this bug, because its consumers resolve on every read. `gate_items`' consumer (`GatingItems`) does not, and changing `GatingItems` to resolve on every read is not this card: `mutate.go:401`'s hot path runs on every move, and dinah-450 built the strict-identifier discipline into the gate on purpose (see `bench.go:1845`'s comment on `gate_items: yes` refusing rather than reading loose). Widening the comparison is a change to the gate's own contract, un-asked-for here. This card's job is the field: store what the gate actually compares against, at both write paths, so nothing is ever written that could not gate.

## 1. What a legal value is

**A legal value is anything `bench.ColumnByRef` resolves: a column's identifier, its slug, or its title (`internal/bench/bench.go:1898-1913`, tried in that order) — the same three spellings `move`, `pull`, and `File` already accept for a column reference.** This is not a new ruling; it is dinah-473's ruling, applied to the field rather than to one command that writes it, which is exactly what this card's description asks for ("the subject is the field, not one command").

**Both write paths resolve to the column's identifier before writing, and store the identifier — never the typed spelling.** `File` already does this. `dinah set <item> column <value>` gains the same behavior:

- Add a new guard constant to `internal/bench/fields.go`, alongside the existing eight:
  ```go
  GuardColumnRef = "column-ref"
  ```
  and add it to the closed `Guards` list.
- Declare it on the field:
  ```go
  {Name: ItemColumnField, Clearable: true, Guard: GuardColumnRef},
  ```
- In `internal/verb/fields.go`'s `admitFieldValue` (the switch at line 191), add:
  ```go
  case bench.GuardColumnRef:
      if l.Bench.ColumnByRef(value) == nil {
          return l.refuse(req, entity.Card, contract.UnknownColumn, value)
      }
  ```
  This reuses `contract.UnknownColumn`, the same refusal `File` already raises for the same condition, so the two write paths refuse identically and no new refusal string or catalog entry is needed.
- `admitFieldValue` validates but does not transform `value`. `writeField` needs the *resolved identifier*, not the typed spelling, exactly as `File` resolves before calling `bench.AddItem`. Add one small routing branch in `SetField` (`internal/verb/fields.go`, immediately before the existing `if field.Guard == bench.GuardHold` branch, following that branch's own shape):
  ```go
  if field.Guard == bench.GuardColumnRef && value != "" {
      value = l.Bench.ColumnByRef(value).ID
  }
  ```
  (`admitFieldValue` has already refused a `nil` resolve above this point, so the lookup here cannot be nil; the `value != ""` guard is what lets a clear — `dinah set <item> column` with no value — pass through unresolved, since `ItemColumnField` is `Clearable: true` and a clear has no value to resolve, matching the existing rule stated in `admitFieldValue`'s own doc comment: "a clear runs no guard.") Then fall through to the existing `return l.writeField(req, entity, field, value)` call, now carrying the resolved identifier.

**Reading is unaffected.** `readField` (`internal/verb/fields.go:243`) returns the raw stored value for every field with no `GuardHold` special case, and after this card the stored value is always either empty or a resolved identifier — never a spelling that needs translating back, unlike `hold`'s `on`/`off` vocabulary. No change to `readField`, `GetField`, or `dinah show`'s existing column rendering (`internal/verb/read.go:955-962`, which already resolves `item.Column` through `l.Bench.Column(...)` for display and is untouched by this card).

## 2. Items already carrying an unresolvable value

A validating write closes the door on new bad values; it does nothing for an item already on disk with one, written by either path before this card lands, or by a hand edit. `dinah check` is the natural sweep, per the card, and there is a direct precedent to follow: `checkTierOverrides` (`internal/bench/check.go:435-453`), which sweeps a card's `ColumnTiers` for exactly this shape of defect (`b.ColumnByRef(override.Column) == nil`) and reports rather than refuses, because a check sweep never writes.

**New finding: `check.item-column-unresolved`.** Add the constant and its doc comment beside the other `Finding*` constants in `internal/bench/check.go` (near `FindingUnknownTierColumn`, line ~119):

```go
// FindingItemColumnUnresolved names a checklist item whose column value
// will never hold a card: either it resolves to no column at all, or it
// resolves to one whose identifier the value does not spell, so
// GatingItems' identifier-equality test (entity.go) can never match it.
// A write filed or set before this card validated the field, or a hand
// edit, both produce it.
FindingItemColumnUnresolved = "check.item-column-unresolved"
```

**New sweep function**, called from `checkCard` (`internal/bench/check.go:487`) alongside the existing `checkTierOverrides` call, reusing the existing `itemsWhere` helper (`internal/bench/entity.go:1013`) that both `GatingItems` and `CountBlockingItems` already share:

```go
// checkItemColumns reports every checklist item whose column value cannot
// currently hold a card at any column: unresolvable, or resolvable but
// not stored in the identifier spelling GatingItems compares against.
func (b *Bench) checkItemColumns(card *Card) []Finding {
    var findings []Finding
    for _, item := range itemsWhere(card.Dir, func(item *Item) bool {
        return item.Column != ""
    }) {
        resolved := b.ColumnByRef(item.Column)
        if resolved != nil && resolved.ID == item.Column {
            continue
        }
        findings = append(findings, Finding{
            Path:   filepath.Join(item.Dir, ItemAnchor),
            Key:    FindingItemColumnUnresolved,
            Detail: card.Ref(b.Slug) + " " + item.ID + " " + item.Column,
        })
    }
    return findings
}
```

Call it from `checkCard`:

```go
findings = append(findings, b.checkItemColumns(card)...)
```

placed next to the existing `findings = append(findings, b.checkTierOverrides(card)...)` line (`check.go:479`).

`Detail` carries three space-separated tokens (card reference, item identifier, the stored value) on the same convention `checkTierOverrides` already uses for its own `Detail` (`card.Ref(b.Slug) + " " + override.Column`), so the catalog text can name all three without a new `Finding` field.

**Catalog entry**, added to all eight locale files (`internal/msg/locales/{af,cs,de,en,es,fil,hi,id}.json`), English first:

```json
"check.item-column-unresolved": {
  "text": "a checklist item on {detail} names a column that will never hold a card: it either resolves to no column, or resolves to one whose identifier does not match what is stored",
  "context": "A check finding: every checklist item's column value, where set, is stored as the identifier of a column this workbench declares. {detail} carries the card's reference, the item's identifier, and the stored column value, in that order. A write filed or set before the field validated, or a hand edit, both produce it."
}
```

The other seven locales carry the same `text` verbatim as a skeleton entry (the convention `TestASkeletonEntryReallyCarriesTheEnglishText` enforces, and the one dinah-473's own D-1 decision describes for a context-only change to an existing key — here the key itself is new, so `text` is added, not just `context`, and the seven non-English copies start as skeletons the way any new English-only key does until translated).

**No self-repair.** `dinah check` reports; nothing in `internal/bench/check.go` writes to disk anywhere in the file, and this sweep does not become the first exception. The repair, once an operator or agent reads the finding, is the now-validated `dinah set <item> column <the right spelling>`, which either succeeds (storing the resolved identifier) or refuses `unknown-column` and names what was typed.

**Machine surface.** No separate work: `Check()`'s findings feed `report.Findings` (`internal/verb/read.go:1333,1407`) generically, which both the text renderer (`cmd/dinah/render.go:877`) and `--json` read from the same slice, exactly as every other finding in this file already does.

## 3. Whether a silently lifted hold should be reported

**No new reporting mechanism, and here is why that is the honest answer rather than a shortcut.** The card's concern is a write that "turns a live gate into a dead one" with nothing telling anybody. After this card, that can no longer happen as a side effect of an *unnoticed* write, because the only way an item's `column` value stops matching a column's identifier is:

- **Clearing it** (`dinah set <item> column` with no value) — an explicit act naming the item and the intent, on the terms every other clearable field already carries; a caller who clears a field typed the command that clears it.
- **Setting it to a different, valid column** — also an explicit act, and also not silent: the caller typed the new column's identifier, slug, or title, and the write's own response (`response.Detail`, `internal/verb/fields.go`'s `writeField`, carrying the resolved identifier) names what it landed on.
- **Setting it to something that does not resolve** — refused outright by this card's fix (§1), so it never reaches disk and never lifts anything.

There is no fourth path left by which a write can produce a dead gate without the caller having typed the exact value responsible. The case the card is naming, an *absence of effect arriving as a success*, described the state before this card's write-time validation existed: `dinah set <item> column some-typo` used to succeed, silently storing a value that would never gate, with the response reporting `ok` and nothing distinguishing that success from a real one. §1 closes exactly that gap by refusing the write instead of accepting it. Once the write cannot silently produce the dead state, there is nothing left for a report to announce that the response does not already say.

This is a decision, not an open question: the family this card belongs to (`dinah-433`, `434`, `437`, `439`, `440`, `462`) is about a read reporting false confidence, and about a write that stores a value it never checked. This card's own §1 removes the write-side member of that family for this field. It is recorded as a decision below, gated at Agent Design Review, so the next stage can push back if it disagrees rather than discovering the reasoning missing.

## Interaction with dinah-462

`dinah-462` is not yet specified or built, so this card does not depend on it and does not wait for it. §2 above follows the *existing* shape of `dinah check`'s findings (three-part `Finding{Path, Key, Detail}`, appended per-card, rendered identically on both surfaces) rather than a shape `dinah-462` might introduce later. If `dinah-462` lands a typed multi-state report (its own description floats "a report became one of three states"), `check.item-column-unresolved` migrates into whatever that shape is exactly as every other finding in the file does, and nothing about this card's finding is bespoke enough to need special handling in that migration.

## Out of scope

- **Widening `GatingItems`' comparison to resolve on every read.** Covered above under "What consumes the field": `tier_at`'s consumers do this and `gate_items`' does not, and changing the gate's own comparison is a change to dinah-450's contract, not to the field's write side, which is what this card is about.
- **Auto-repairing a stale value from `dinah check`.** Covered in §2: check reports, it does not write, anywhere in the file.
- **A column-journal reader**, and any change to what `writeField`'s existing per-write journal event carries for this field. `ItemColumnField` carries no `Guard`-specific journal translation (unlike `hold`'s on/off remap): the journal continues to carry the resolved identifier in `from`/`to`, exactly as it does today for every write through `writeField`, and nothing about this card changes that shape.
- **`dinah-472`'s fifth item state.** Named in the card's own related-work list as irrelevant to this one.

## Decisions

- **A legal value is anything `bench.ColumnByRef` resolves (identifier, slug, or title), and both write paths store the resolved identifier, never the typed spelling.** This is dinah-473's own ruling, applied to the field rather than to `File` alone, which is the gap this card exists to close. — gates Agent Design Review
- **No new reporting mechanism for a "silently lifted hold."** Write-time validation (§1) removes every path by which a write could produce a dead gate without the caller having typed the value responsible; the remaining case (an item already stale on disk) is `dinah check`'s job (§2), not the write's. — gates Agent Design Review
- **`GatingItems`' identifier-equality comparison is left unchanged.** Resolving on every read (as `ColumnTiers` does) is a change to the gate's own contract from dinah-450, not to the field, and is out of scope here. — gates Agent Design Review

## Acceptance criteria

1. `bench.FieldOf("item", "column")` reports a field whose `Guard` is `bench.GuardColumnRef`; `bench.GuardColumnRef` is a member of the closed `bench.Guards` list; `TestEveryDeclaredGuardIsRouted` (`internal/verb/fields_routing_test.go`) passes with a `case bench.GuardColumnRef:` arm routed in `admitFieldValue`.
2. Running the built binary against a scratch workbench: `dinah file <card> criterion "text" --column <a column's slug>` succeeds, and a subsequent `dinah get <card>/checklist/<n> column` (or the item's own printed reference) returns that column's identifier, not the slug that was typed. Prove this by running the tool, not by reading the source: create the item, read the value back, and show it equals the column's ID from `dinah get <column> id` (or equivalent).
3. Running the built binary: `dinah set <item> column <a column's slug or title>` succeeds (exit 0) and stores the column's identifier on disk (read the item's anchor file directly, or `dinah get <item> column`), matching what §1 specifies for `File`.
4. Running the built binary: `dinah set <item> column bogus-value` (a value `bench.ColumnByRef` cannot resolve) refuses `unknown-column`, exit code matching the tool's existing refusal exit for this refusal name, and the item's anchor file is unchanged (compare its contents before and after the call).
5. End-to-end proof that a validated write actually holds a card: create a column with `hold on` (dinah-477), file an item naming that column by its slug through `dinah set <item> column <slug>` (not by identifier), then attempt `dinah move <card> <that column>` and confirm it is refused `unresolved-item`; resolve the item and confirm the same move now succeeds. This is the one criterion proving the fix has an effect on the gate, not merely on what is stored.
6. `bench.Check()` against a fixture workbench containing one card with a checklist item whose `column` frontmatter key is hand-written to a value that does not resolve via `ColumnByRef`, and a second card with an item whose `column` is hand-written to a real column's slug (not its identifier), reports `FindingItemColumnUnresolved` for both items and for no others; a third card with an item correctly carrying a resolved identifier produces no such finding. Assert the sweep found exactly these two, not merely "at least these two" (per the workbench's own rule on sweeps: assert the size of the set the sweep read).
7. `dinah check` (both the text-rendered and `--json` surfaces) against the same fixture reports the new finding on both surfaces, proving the finding reaches the machine surface with no separate wiring, per §2's claim that `Check()`'s output feeds both renderers from one slice.
8. The new catalog key `check.item-column-unresolved` exists in all eight locale files under `internal/msg/locales/`, and `TestEveryKeyCarriesAContext` and the message-catalog completeness tests in `internal/msg/msg_test.go` pass with it added.
9. `internal/bench/check.go` gains no write to disk: a diff review (or a test asserting the sweep is read-only, e.g. comparing the fixture directory's mtime/contents before and after `bench.Check()` runs) confirms the new sweep, like every other in the file, only reads.
10. A card with no checklist items, and a card whose items all carry an empty `column`, both produce zero `FindingItemColumnUnresolved` findings, proving the sweep does not fire on the absence of a value (which `itemsWhere`'s `item.Column != ""` predicate should already guarantee, but is worth proving live since an empty-string false positive would make every ordinary item report a finding).

All ten gate: Merge.

## Branch

dinah-474-a-checklist-items-column-is-never-validated-so-an-item-can-hold-a-card-out-of-a-column-that-does-not-exist
