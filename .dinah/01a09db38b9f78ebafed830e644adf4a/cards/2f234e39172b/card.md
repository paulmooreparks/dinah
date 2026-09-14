---
title: A column can only hold a card on the way in, so passing an operator's station without stopping is a convention rather than a rule
column: b69abf918c42
state: ready
severity: major
priority: now
tier: frontier
workstreams:
  - b3f924406e4c
  - f302445714bb
links:
  - kind: relates_to
    to: 5d4bd80c266a
  - kind: relates_to
    to: 211cecf3660d
  - kind: relates_to
    to: a9f3e9ae4e29
  - kind: blocks
    to: 5ef07a3b83a3
---
A column's hold refuses entry while the card carries an unsettled item naming that column. That is the only direction there is, and it is the wrong one for a station whose whole job is to answer something. A card cannot be held at the place its answer comes from, so a station that exists for a person to rule at has no way to say "this card has something for you" and every way of saying "you may not come in yet".

The operator ruled on 2026-09-11 that he will not depend on convention for this. Today an agent carries a clean card past his review stations because it has been told to, and stops when it judges there is something he must see. That works exactly as well as the agent's discipline, it leaves no record of the judgement, and a card that should have stopped looks identical to one that should not.

The shape he approved is to give the hold a direction. A column declares whether it holds on the way in or on the way out. A station holding on the way out admits any card and releases any card, unless that card carries an unsettled item naming the station, in which case the card stays until the item is settled. Passing through without stopping becomes the board's own behaviour rather than an agent's restraint, and stopping becomes something somebody has to do on purpose by filing the question that causes it.

Two consequences the operator ruled on directly, and both are to be preserved rather than reasoned about again. His acceptance station stays owned by him outright, where nobody else moves a card out at all, because "this works" is a judgement every card needs rather than a conditional one. A workbench therefore carries both shapes at once, and that is the intended end state, not a transitional one. And the station that verifies work before it lands keeps holding on the way in, because a criterion still has to be settled before the card arrives there.

**The second half, without which the first half is still a convention.** A checklist item records an owner, and nothing enforces it: an item marked as the operator's can be settled by anybody. An exit hold cleared by whoever happens to be holding the card is not a gate. So settling an item whose owner is the operator has to be refused to everyone else. That is the smaller change of the two and it is the one that actually removes the convention, which is why it belongs on this card rather than a card of its own.

Worth knowing while weighing the risk here: the move override is already safe. It is refused to anyone who is not the operator, so nothing that this card builds can be bought past with a flag.

**A benefit that falls out, and should be taken deliberately rather than by accident.** Because entry is currently the only direction, an item has to name the column after the one that answers it, and a workbench that gates the answering station itself deadlocks the card out of the place its answer comes from. That has already happened on this board and had to be caught by reading column positions. Once a station can hold on the way out, an item names the station that settles it, which is what a reader expects and what the operator said in the first place. Whoever specs this should say plainly which direction each kind of item is expected to use afterwards, because the existing guidance on this board says the opposite and will need correcting with it.

**What is not settled and is not the operator's to settle.** How the direction is spelled and stored, and what happens to the workbenches already carrying the boolean the command `dinah set <column> hold on` writes today. Whether a column may hold in both directions at once. Whether an exit hold refuses a backward move as well as a forward one, which matters because a push-back out of a review station is the common case and an unanswered question probably should not travel in either direction. Whether the refusal keeps one name or wants two. Whether an agent may reopen or fail an operator-owned item even when it may not resolve one. What `dinah check` should say about a workbench whose declarations no longer make sense. Read dinah-450, dinah-473, dinah-474 and dinah-477, which built the hold and the item's column between them, before choosing any of it.

The core must stay free of this board's vocabulary. Nothing here may name a column, a role, or a kind of work: a workbench with no code and no reviews declares the same two directions in its own words, or declares neither.

This card is sequenced ahead of the move of this project's development onto Dinah, tracked by dinah-449, because the column instructions that move needs rewritten would otherwise describe the old shape and be written twice.

## Specification

## Verification method

Read against trunk `65a80ad8b3b525e62af2262ad6d3203876ea6c7a`, confirmed by a fresh `git fetch origin main` for this pass too (the commit has not moved across any of the last three passes), in a fresh worktree at `C:/dinah-scratch/dinah-484-spec4/wt` for this revision. This revision responds to Agent Design Review's single finding on the third pass: the site left deliberately unfixed on the second pass, `cmd/dinah/main_test.go`'s "thirteen checks" subtest name, should be folded in rather than recorded and skipped, since it is a one-word fix inside a sweep and a count this card is already running and changing. The second pass's own sweep reconciled exactly to its twelve sites, confirmed independently by the review with no thirteenth found there; this pass adds the one additional site the review asked to fold in, verified against the corrected check list rather than taken on the review's own say-so.

This revision changes only the renumbering enumeration and the sites it names; the field design, the `canLand` placement fix, the override criterion and the ordering-pinning criterion are all unchanged and were confirmed sound by the review (no fifth vocabulary word, placement matches its claimed row, arithmetic checked out, criteria genuinely pin what they claim).

## What changed in this revision, in one paragraph

The sibling-boolean field (`ExitHold bool`, stored key `gate_items_exit`) is gone. `hold` becomes the one field the operator's own words describe: it now carries a direction, stored under its existing key `gate_items`, with `true` preserved exactly as the legacy spelling of "holds on entry" and two new values, `out` and `both`, added to the same closed vocabulary. The departure-side check in `canLand` is relocated to run immediately after the loop-limit block, which is where its own row's position in `dinah help move` already claimed it ran and where, on trunk as it stands, it did not. The same relocation, applied consistently rather than only for `move`, requires one new row inserted into `pullChecks` rather than appended after it, which renumbers four existing rows; the full set of sites that renumbering reaches is enumerated below, following a widened repository-wide sweep rather than the earlier, incomplete one. Two acceptance criteria are added: one that pins the corrected order by construction, and one that proves `--override` reaches the new check. AC-14 is folded into AC-9 rather than left as a standalone doc-comment check.

## The field: `hold` gains a direction, not a sibling

**Decision, superseding the prior revision's "sibling field" decision, which Agent Design Review rejected as narrower than the operator's own words.** The operator's description says "a column declares whether it holds on the way in or on the way out": one property with an axis, not two independently-settable switches. `hold` (dinah-477) keeps its typed name, its stored key (`gate_items`), and its guard (`GuardHold`); what changes is the set of values it may carry.

**Vocabulary, minted in full, endorsed as-is by Agent Design Review's second pass.** `dinah set <column> hold <value>` accepts exactly four values: `on`, `off`, `out`, `both`. `on` and `off` are unchanged from dinah-477 in every respect: `on` writes `gate_items: true` (byte-identical to today), holds a card entering the column, and is what any existing script or muscle memory already types. `off` clears the key entirely, exactly as it does today. `out` is new: it writes `gate_items: out` and holds a card leaving the column, settling nothing about entry. `both` is new: it writes `gate_items: both` and holds a card both ways. There is no fifth word `in`; `on` already means "holds on entry" and stays the only spelling for it, on the same reasoning the review itself applied against the prior revision's two-boolean shape (two spellings for one thing is the defect being corrected, not a pattern to repeat one level down). The review's second pass considered and rejected adding a fifth value word; the vocabulary stands as spec'd.

**What an untouched existing declaration means.** A column carrying `gate_items: true` from before this card is read exactly as before: `dinah get <column> hold` still prints `on`, the column still refuses a card entering it while an unsettled item names it, and nothing about its behavior changes. This is not a policy choice made for this card; it falls out of the parser change below treating `true` as the literal legacy spelling of the `on` value, not as a distinct case requiring translation.

**`dinah get <column> hold` prints exactly the word that was last set**, on the same enumeration: `on` for stored `true`, `off` for stored `false` or an absent key, `out` for stored `out`, `both` for stored `both`. There is no case where two different typed inputs produce indistinguishable stored states yet different printed outputs, and no case where the print value depends on anything but the stored value, because `on` is the only spelling that ever produces the stored value `true`.

**Whether a column may hold in both directions at once.** Yes, via the fourth value, `both`. This is not two independent booleans whose cross product happens to include a "both" case; it is one closed enumeration of four states (`off`, `on`, `out`, `both`), which is smaller than what the prior revision's two-boolean shape could express and is exactly the size the operator's own words describe: a direction, with both as a legitimate third answer to "which way," alongside "in" and "out."

### Concretely, in `internal/bench/bench.go` and `internal/bench/fields.go`

- `Column.GateItems bool` (`bench.go:205-210`) is replaced with `Column.Hold string`, doc-commented to name its four legal values (`bench.HoldOn`, `bench.HoldOff` -- read as the zero value `""`, not written as a struct default meaning anything else -- `bench.HoldOut`, `bench.HoldBoth`) and to say plainly that callers read `HoldsOnEntry()`/`HoldsOnExit()` below rather than comparing the field directly.
- Two methods are added beside `TakesWorkUp`/`Terminal` (`bench.go:255-275`):

```go
// HoldsOnEntry reports whether an item naming this column can hold a
// card from entering it. CORE-GATE-3 reads this at a move's or a pull's
// destination.
func (s *Column) HoldsOnEntry() bool {
    return s.Hold == HoldOn || s.Hold == HoldBoth
}

// HoldsOnExit reports whether an item naming this column can hold a
// card from leaving it, Dinah's own addition beside CORE-GATE-3. canLand
// reads this at a move's or a pull's departure.
func (s *Column) HoldsOnExit() bool {
    return s.Hold == HoldOut || s.Hold == HoldBoth
}
```

- `readColumnIn`'s `gate_items` switch (`bench.go:1844-1851`) becomes:

```go
// The same strict reading as above, extended: absent and false both mean
// no hold, true is the legacy spelling of holding on entry and is
// preserved exactly rather than reinterpreted, and out and both are the
// two new directions this field now carries. Anything else is refused,
// on the same terms a hand-edited flag has always been refused here.
switch fm.Value("gate_items") {
case "":
case "false":
case "true":
    column.Hold = HoldOn
case "out":
    column.Hold = HoldOut
case "both":
    column.Hold = HoldBoth
default:
    return nil, contract.RefuseWith(contract.Malformed, "column "+id, anchor)
}
```

- `internal/bench/fields.go`'s constants gain two rows beside `HoldOn`/`HoldOff` (`fields.go:103-105`): `HoldOut = "out"` and `HoldBoth = "both"`, doc-commented that `on` stays the sole spelling for the entry direction and `in` is deliberately not a fifth word. `HoldField`'s own doc comment (`fields.go:94-97`) is corrected to name all four values and both directions plainly.
- The `KindColumn` field table (`fields.go:145`) is **unchanged**: `{Name: HoldField, Key: GateItemsKey, Guard: GuardHold}`, no new row, because this is one field, not two.

**That last line is why the CLI/MCP surface needs almost no new code.** `admitFieldValue`'s `GuardHold` case (`internal/verb/fields.go:227-230`) becomes:

```go
case bench.GuardHold:
    switch value {
    case bench.HoldOn, bench.HoldOff, bench.HoldOut, bench.HoldBoth:
    default:
        return l.refuse(req, entity.Card, contract.Malformed, field.Name)
    }
```

`typedHold` (`fields.go:262-268`) becomes:

```go
// typedHold reports a column's hold in the words a person types, given
// what the anchor stores. The stored spelling is the profile's
// gate_items, whose value is exactly true, false, out, both, or absent;
// the five readings this collapses to four typed words are the five that
// can reach it, malformed values having already been refused at open.
func typedHold(stored string) string {
    switch stored {
    case "true":
        return bench.HoldOn
    case "out":
        return bench.HoldOut
    case "both":
        return bench.HoldBoth
    }
    return bench.HoldOff
}
```

`storedHold` (`fields.go:277-284`) becomes:

```go
func storedHold(typed string) string {
    switch typed {
    case bench.HoldOn:
        return "true"
    case bench.HoldOut:
        return "out"
    case bench.HoldBoth:
        return "both"
    }
    return ""
}
```

`writeHold` itself (`fields.go:122-134`) is untouched: it already routes through `storedHold`/the typed detail rewrite generically, with no reference to which values exist.

### Interchange (`internal/bench/interchange.go`)

No new key. `knownColumnKeys` is unchanged. `exportColumn`'s `gate_items` block (`interchange.go:96-98`) becomes:

```go
// CORE-JSON-10 blesses this member; it now carries either the boolean
// true (holds on entry, unchanged from before this card) or the string
// out or both (Dinah's own extension of the same member's value set).
switch column.Hold {
case bench.HoldOn:
    element["gate_items"] = mustMarshal(true)
case bench.HoldOut:
    element["gate_items"] = mustMarshal("out")
case bench.HoldBoth:
    element["gate_items"] = mustMarshal("both")
}
```

`writeColumnFromMember`'s `gate_items` read-back (`interchange.go:331-336`) becomes:

```go
if raw, ok := element["gate_items"]; ok {
    var asBool bool
    var asString string
    switch {
    case json.Unmarshal(raw, &asBool) == nil:
        if asBool {
            fm.Set("gate_items", "true")
        }
    case json.Unmarshal(raw, &asString) == nil && (asString == "out" || asString == "both"):
        fm.Set("gate_items", asString)
    }
}
```

following the existing block's own lenient discipline: a shape this build does not recognize is silently not written, the same way today's code silently drops a non-boolean `gate_items` member, rather than refusing the whole import over one column's one member.

## The check: departure-side, running immediately after the loop-limit row, its own refusal name

**Blocker 2 (prior revision), confirmed fixed by the review's second pass and unchanged in this one.** The new block sits directly after the loop-limit block and before the `TakesNoWork`/`AwaitingOutside` row, in `internal/verb/mutate.go`, immediately following the closing brace of the `if departure != nil && departure.LoopLimit > 0 ...` block and before the `if !destination.TakesWorkUp() ...` block:

```go
// CORE-GATE-6, Dinah's own: the departure's own exit hold, symmetric with
// the entry row above and running immediately after the loop-limit row it
// follows, before the three rows beneath it that ask about the
// destination rather than about this card's own departure. Nothing below
// reads forward, on the same terms the entry row above reads neither: an
// unresolved item is exactly as good a reason to keep a card at the
// station that raised it on a push-back as it is on an advance.
exitGateHeld := false
if departure != nil && departure.HoldsOnExit() {
    holding := bench.GatingItems(card.Dir, departure.ID)
    if len(holding) > 0 {
        exitGateHeld = true
        if !req.Override {
            return false, l.refuse(req, card, contract.UnresolvedItemExit, holding[0].ID), nil
        }
    }
}
```

The final `return` is unchanged in shape: `return (reached || loopReached || gateHeld || exitGateHeld) && req.Override, nil, nil`. The entry check (`mutate.go:397`) changes only its condition, from `if destination.GateItems {` to `if destination.HoldsOnEntry() {`.

`canLand`'s own doc comment (`mutate.go:349-365`) gains one clause, inserted after "the departure has not reached its own declared loop_limit for this card" and before "the destination does not wait on somebody outside the workbench": "the departure holds no unresolved item of this card's that names it for departure,".

### `checks.go`: row 11 on `Move`, unchanged in number, now true about where it runs

`checkLists[Move]` (`checks.go:74-95`) keeps exactly the row the prior revision proposed, appended after row 10 (`AtLoopLimit`):

```go
{Refusal: contract.UnresolvedItemExit, Key: "check.move.11"},
```

### `pullChecks`: a new row 12, and the renumbering it requires

Pull reuses `canLand` (`pull.go:374`), so relocating the exit-hold block inside `canLand` moves it, for a pull exactly as for a move, to run immediately after the destination's own entry-gate check and before the operator-owned reservation and the retiring check. In `pullChecks` (`checks.go:133-151`), the entry-gate row is row 11 (`UnresolvedItem`); the operator-owned reservation (today's row 12, `NotOperator`) and the retiring check (today's row 13, `Locked`) come immediately after it, so the new exit-hold check's correct position is between row 11 and today's row 12: a genuine insertion, renumbering four rows. `pullChecks` carries entirely Dinah's own numbering (Pull is not a contract verb), so this does not touch a CORE-numbered row.

`pullChecks` becomes:

```go
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
	{Refusal: contract.UnresolvedItem, Key: "check.pull.11"},
	// Row 12, Dinah's own: the departure's own exit hold, canLand's new
	// row, reached between the destination's entry gate above and the
	// operator-owned reservation below, exactly where canLand runs it.
	{Refusal: contract.UnresolvedItemExit, Key: "check.pull.12"},
	{Refusal: contract.NotOperator, Key: "check.pull.13"},
	{Refusal: contract.Locked, Key: "check.pull.14"},
	{Refusal: contract.UnresolvedItem, Key: "check.pull.15"},
	{Refusal: contract.BelowTier, Key: "check.pull.16"},
}
```

Only the `Key` strings of the last four rows change (12 to 13, 13 to 14, 14 to 15, 15 to 16); their `Refusal` values and order among themselves are untouched.

## The re-swept enumeration, in full, replacing the prior revision's incomplete one

**Why the prior list was wrong, and what changed about how this one was produced.** The prior revision's enumeration was built by re-reading the specific files the placement fix already touched, and by pattern-matching the literal wording used elsewhere in the card ("Row 14", "rows 11 to 14"). That is exactly the failure mode the workbench's own standing instructions describe: a claim of completeness produced by rereading familiar ground rather than by a command over the whole tree. This pass instead ran, against this same trunk commit, in the fresh worktree above:

```
grep -rniE "\brow[s]? [0-9]+" --include=*.go --include=*.md --include=*.json .
grep -rniE "seventeen|sixteen|fifteen|fourteen|thirteen" --include=*.go --include=*.md --include=*.json .
```

and every hit was read in context (not assumed from the surrounding word alone) to decide whether it names a `pullChecks` position affected by the insertion. The first search's hits outside `internal/verb/{checks,pull,pull_test}.go` were read and are not `pullChecks` references at all (they name unrelated command's own check lists: `add`, `set`/`get`, workstream, table rendering, or `checkLists[Claim]`), and are not listed below because they name nothing this card's renumbering touches. The second search's hits outside the same three files are unrelated counts (UUID byte counts, the profile's seventeen refusal names, other cards' own criterion counts) and are likewise not listed.

**Thirteen sites in Go source, across four files, plus the mechanical catalog edit across eight locale files:**

1. `internal/verb/checks.go:117` -- the header comment "These are rows 3 to 15 of pull's fifteen-row list" becomes "rows 3 to 16 of pull's sixteen-row list".
2. `internal/verb/checks.go:126` -- "row 12 reads the column it would land in and be claimed at" (the `NotOperator`/CORE-CLAIM-8 row) becomes "row 13 reads...".
3. `internal/verb/checks.go:148` -- "Row 15 is Dinah's own tier gate, the claim list's row 8 reached at the destination" (`BelowTier`) becomes "Row 16 is Dinah's own tier gate...". **Missed by the prior revision's enumeration entirely; found on this pass's sweep.**
4. The `pullChecks` list itself (`checks.go:133-151`), shown in full above.
5. `internal/verb/pull.go:334` -- "It evaluates rows 8 to 14 of pull's table" becomes "rows 8 to 16". **Missed by the prior revision.** This range was already an undercount before this card: it describes the whole of `pull()`'s own checks, which include `claimableItems` (today's row 14) and `claimableTier` (today's row 15) after `canLand` returns, so the true span was already 8 to 15, not 8 to 14. This card's insertion moves the true span to 8 to 16, and the comment is corrected to that, closing the pre-existing undercount in the same pass rather than perpetuating it one further off.
6. `internal/verb/pull.go:343` -- "canLand is the rest of the move's list, rows 11 to 14" keeps the same numeric range (the insertion lands inside it: 11 entry-gate, 12 exit-hold, 13 operator-reservation, 14 retiring) but gains a clause: "...rows 11 to 14, which now include the departure's own exit hold between the entry gate and the operator-owned reservation."
7. `internal/verb/pull.go:388` -- "Row 14, Dinah's own, is asked about the destination rather than the column the card is leaving, because that is where the claim is taken," directly above the `claimableTier` call, becomes "Row 16...". This was already wrong before this card (`claimableTier`/`BelowTier` was row 15, not 14, on trunk as it stood); corrected to the post-renumbering value in the same pass.
8. `internal/verb/pull_test.go:393-396` -- "Row 13 is evaluated under the card's lock" (`Locked`) becomes "Row 14 is evaluated under the card's lock". **This is the site Agent Design Review's first finding named.**
9. `internal/verb/pull_test.go:415` -- "Rows 8 to 13 read the one card the selection chose" becomes "Rows 8 to 14 read the one card the selection chose". **Missed by the prior revision.**
10. `internal/verb/pull_test.go:748-775`, `TestPullChecksAgainstTheFullSeventeenRowTable`: three things change together, all missed by the prior revision, which touched only the `want` list inside this function and not its name or its own header comment. The function is renamed to `TestPullChecksAgainstTheFullEighteenRowTable` (workbench pair, 2, plus pull's own list, now 16, is 18). Its header comment, "followed by the fourteen pull rows the spec owns and Dinah's own tier row," becomes "followed by the fifteen pull rows the spec owns and Dinah's own tier row" (the new exit-hold row joins the same "pull rows" bucket the comment already counts `AmbiguousColumn`/`NoUpstream` in, both of which are Dinah's own layer refusals rather than the profile's, so the bucket was never "profile-owned rows" in a strict sense and the new row belongs in it on the same terms). The `want` list gains the new row 12 and renumbers rows 12-15 to 13-16, as already specified above.
11. `internal/verb/pull_test.go:983-992`, `TestPullReadsTheCardRowsBeforeTheDestinationRows`: "row 10 reads the card, rows 11 to 13 read the destination, and the card decides first" is not a simple renumbering; the new row breaks the contiguous claim. Row 11 (entry-gate) and the old rows 12-13 (operator-reservation, retiring) all read the destination, but the new row 12 (exit-hold) reads the departure, not the destination. The corrected text: "row 10 reads the card, row 11 and rows 13 to 14 read the destination (row 12, the new departure-side exit hold, reads the departure instead), and the card decides first." The test's own assertions are behavioral (it asserts a `Held` refusal, not a row number), so nothing in the function body changes, only this comment.
12. `internal/msg/locales/en.json:472-484` (`check.pull.12` through `check.pull.15`) and the identical keys in the other seven locale files under `internal/msg/locales/`: renamed in place, highest key first (15 to 16, 14 to 15, 13 to 14, 12 to 13), with a new `check.pull.12` entry inserted, exactly as specified in the prior revision (unchanged by this pass).
13. `cmd/dinah/main_test.go:7656` -- the subtest name `"help pull prints the arguments and the thirteen checks in order"` becomes `"help pull prints the arguments and the eighteen checks in order"`. Folded in on this pass rather than left recorded and skipped: it is a one-word fix, found by this same sweep, inside a count this card is already changing, and not a defect anybody would pick up as a card of its own. The number is `verb.Checks(Pull)`'s length once this card lands, verified rather than assumed: `WorkbenchChecks` (`checks.go:50-53`) carries exactly two entries, unchanged by this card, and `pullChecks` carries sixteen rows after the insertion specified above (fifteen today plus the one new row), so the total is eighteen. The literal count in the subtest's name was already wrong before this card (the true count on trunk today is seventeen, not thirteen, for a reason this card's renumbering does not create and does not explain); this fix does not audit or correct that pre-existing gap, it only lands the name on the number that becomes true once this card's own change ships. The subtest's own numeric assertions are already computed dynamically from `len(verb.Checks(verb.Pull))` rather than from the word in its name, so nothing about its behavior changes, only the literal word.

Nothing about `canClaim`/`claimableItems` changes beyond the key rename above; claiming a card in place is still not a departure.

## The refusal name: two names, not one

Unchanged from the prior revision. `contract.UnresolvedItemExit = LayerPrefix + "unresolved-item-exit"`, added to `contract.Introduced` and to `internal/contract/shape.go`'s `Shapes`, mirroring `UnresolvedItem`'s own shape entry, with catalog text in all eight locales:

```json
"refusal.dinah.unresolved-item-exit": {
  "text": "this card carries the item {detail}, which is not resolved, and this station holds until it is",
  "context": "The sentence printed after the refusal name dinah.unresolved-item-exit. {detail} is the item's identifier, on unresolved-item's own convention."
},
"refusal.dinah.unresolved-item-exit.next": {
  "text": "; answer it, then resolve, verify or fail that item",
  "context": "Spliced onto refusal.dinah.unresolved-item-exit as the next step."
}
```

The bundled correction to `refusal.unresolved-item.next`'s stale text is also unchanged from the prior revision: corrected to name `resolve`/`verify`/`fail` across all eight locale files, in the same diff that is already touching these files for the renumbering above.

## Ownership: unchanged from the prior revision

The entire "Ownership" section (the `closeItem` operator-only check for `Resolve`/`Verify`/`Fail`, `Reopen`'s deliberate non-restriction, and the `SetField` guard closing the owner-field write-around) is unchanged, and was confirmed sound on the review's second pass. `internal/verb/checklist.go`'s `closeItem` gains the operator-only check on a `owner == "operator"` item; `internal/bench/item.go` gains `ItemOwnerOperator = "operator"` and its `ItemOwnerField` doc-comment correction; `internal/verb/fields.go`'s `SetField` gains the owner-field write-guard. See the prior revision's text for the full diffs; nothing here changed.

## `dinah check`: no new finding

**Decision, unchanged from the prior revision.** `Column.Hold`'s four values, `OperatorOwned` and `AwaitingOutside` combine exactly as before: `OperatorOwned` alone (`Hold == ""`) is the acceptance-station shape the operator ruled must be preserved; `AwaitingOutside` combined with `Hold` carrying `out` or `both` is the review-station shape this card exists to enable; an inert `out`/`both` declaration is exactly as legitimate as an inert `on` declaration is today. No new `dinah check` finding.

## The benefit that falls out: naming conventions, stated plainly

**Decision, unchanged from the prior revision.** An item meant to be settled at the station where it was raised names that same station, declared to hold `out` (or `both`); an item meant to be verified before later work may begin still names that later column, declared to hold `on` (or `both`). Recorded forward for the workbench's own instructions text and the dinah-449 cutover, unchanged.

## The core-profile question

**Decision, unchanged from the prior revision.** `docs/spec/core-profile.md` sits on the `dev` maturity channel and binds nobody yet. CORE-GATE-5/6 are drafted in the prior revision's text and not landed by this card. Recorded forward.

## Out of scope

- **Editing `docs/spec/core-profile.md`.** Recorded forward, above.
- **Editing this workbench's own instructions text.** Recorded forward, above.
- **A `column` field write-guard for operator-owned items.** Recorded as a related, separate concern.
- **A fifth item state.** dinah-472's territory, untouched.
- **Lanes.** dinah-449 already ruled them out.
- **Renumbering any row of `checkLists[Claim]` or `checkLists[Move]`'s first nine, profile-numbered rows.** Only `pullChecks` is renumbered by this card.

## Decisions

See the checklist. D-1, D-2, D-3, D-9, D-12 are unchanged from the prior revision. D-13 (the `pullChecks` renumbering decision) is corrected again on this pass: it previously named twelve Go-source sites and recorded the `main_test.go` subtest name as a deliberate omission in the spec's Out of Scope rather than in D-13 itself; it is rewritten to name all thirteen sites including the fold-in, and the Out of Scope bullet that used to record the omission is removed since there is no longer an omission to record.

## Acceptance criteria

See the checklist. AC-1 through AC-16 are unchanged from the prior revision; none of them asserted the renumbering enumeration's own completeness (AC-11 asserts every `check.pull.N` key used by `pullChecks` has a matching catalog entry, which is a claim about the code and the catalogs agreeing with each other, verified by reading `pullChecks` itself rather than by trusting a hand-written list, and it continues to hold unchanged: it is satisfied by the corrected `pullChecks` table regardless of how many comments elsewhere cite a row number). AC-16 pins the corrected order and the renumbering's functional effect on the pull path by testing `Checks(Pull)` directly against the corrected table, which also does not depend on the comment sweep. Nothing here required a new criterion; the finding was about prose exhaustiveness, not about an unverified behavior.

## Open questions

None filed, unchanged from the prior revision's own reasoning.

## Branch

dinah-484-a-column-can-only-hold-a-card-on-the-way-in
