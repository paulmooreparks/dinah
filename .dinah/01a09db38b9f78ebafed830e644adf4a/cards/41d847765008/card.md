---
title: A queue column still draws a ready group, though a card in one is not ready or unready
column: b69abf918c42
state: ready
severity: major
priority: soon
tier: frontier
workstreams:
  - 3fce00c6e629
---
The operator ruled that a column of kind intake has no ready and no active states, because a card sitting in a queue is not in either condition. The tree does not honour that ruling. A queue-kind column still draws a ready group, and it draws it with a count of zero, which is the case the ruling exists to suppress.

Half the ruling shipped. dinah-275 suppressed the active group and suppressed an empty blocked group, and it never touched ready. The trunk's own tree test asserts the current behaviour for an intake column, so the gap is written down as intended rather than merely present, and closing it means changing that assertion along with the code.

The same is true of a buffer-kind column, which was confirmed rather than assumed by building the trunk binary at `08e4254` and running the tree against a fresh fixture.

What this card asks for is that the state breakdown follow from the column's kind rather than from the set of states the format happens to declare. Where a card's position in the column is the whole of its condition, the tree draws no breakdown. The rule belongs in the model rather than in each surface, because the sidebar, the terminal board and any other client would otherwise each need the same exception, and a rule implemented twice is the defect this workbench keeps paying for.

Filed out of the design review of dinah-265, where the sidebar spec was written to the ruling and the reviewer checked it against the shipped binary rather than against the card that recorded the ruling. The sidebar cannot draw the tree the operator asked for while the read it draws from answers differently, so dinah-265 depends on this.

## Specification

## The defect, restated precisely

`Column.States()` (`internal/bench/bench.go:200-205`) answers which states a card standing at a column may carry:

```go
func (s *Column) States() []string {
	if s.TakesWorkUp() {
		return []string{contract.StateReady, contract.StateActive, contract.StateBlocked}
	}
	return []string{contract.StateReady, contract.StateBlocked}
}
```

`internal/verb/tree.go`'s grouping recursion (`declaredValues`, `internal/verb/tree.go:589-603`) asks this method for a column's declared states, and its caller (`axisValueOrder`, `tree.go:537-569`) draws a group for every value it returns whether or not a card is standing in it, except `blocked`, which it draws only when occupied (dinah-275's tier 2). Because `States()` still declares `ready` for a column where `TakesWorkUp()` is false, an intake column, a done column, a `dinah.buffer`, or any column marked `awaiting_outside`, the tree still draws an empty `ready` group there whenever no card happens to occupy it. `blocked` was fixed by dinah-275; `ready` (and, moot in practice since a queue column can never legally hold an active card, `active`) was not.

## The call-site census

```
grep -rn "TakesWorkUp\b\|\.States()\|HoldsState" --include="*.go" . | grep -v _test.go | grep -v '\.claude/worktrees'
```

resolves, on the current tree, to exactly these 8 production files:

- `internal/bench/bench.go:164,169,201,210,211` — `TakesWorkUp()` and `States()`/`HoldsState()` built on it. **This is the only file this card's production code touches.**
- `internal/bench/check.go:318` — `!column.TakesWorkUp()` gating `FindingClaimWhereNoWorkIsTaken`. Untouched: it asks `TakesWorkUp()` directly, not `States()`.
- `internal/verb/library.go:389` — `column.HoldsState(contract.StateActive)` in `takeUpActs`. Untouched: checks `Active` membership only, which `States()` still answers identically (`false` for a queue column, both before and after this fix).
- `internal/verb/mutate.go:166,354` — `HoldsState(contract.StateActive)` in `claimableColumn`, and `!destination.TakesWorkUp()` in the move's claim-clearing check. Untouched, same reason.
- `internal/verb/pull.go:446,447,453` — a pull's source and destination `TakesWorkUp()`. Untouched.
- `internal/verb/read.go:33,36,136,247,248` — the wire field `TakesWorkUp bool` and the `byPull` computation, both reading `TakesWorkUp()` directly. Untouched.
- `cmd/dinah/render.go:259` — `!column.TakesWorkUp` reads the already-serialised wire bool `read.go:136` put there. A read, not a re-derivation, same as dinah-275's AC-5 found for its own equivalent line.
- `internal/verb/tree.go:593,622` — `column.States()` inside `declaredValues` and `stateUnion`. **No code change needed here**: see Decision D1.

Adding `_test.go` back in, the same grep also hits `cmd/dinah/row_pairing_test.go`, `internal/bench/columnkind_test.go`, `internal/bench/kindguard_test.go`, `internal/mcp/awaiting_test.go`, `internal/verb/takesnowork_test.go`, for 13 files total. Of the test files, only `columnkind_test.go` and `row_pairing_test.go` assert a literal expectation this card's fix changes; both are covered below. `kindguard_test.go`, `awaiting_test.go` and `takesnowork_test.go` test `TakesWorkUp()` itself, unaffected.

Re-running the production-only grep after this card's implementation must resolve to the same 8 files. A ninth production hit would mean a caller re-derived the rule instead of reading `States()`, which is the eight-call-site duplication this workstream (`dinah-207`, `dinah-253`, `dinah-273`, `dinah-275`) exists to stop happening a third time.

## The fix

One method, no new parameter, no new exported name:

```go
// States returns the states a card standing at this column may carry, in the
// order ready, active, blocked, when an owner takes work up at this column.
// A column where no owner takes work up (an intake column, a done column, a
// buffer, or a column marked awaiting_outside) declares none of the three:
// nobody claims a card there, so an unoccupied ready or active group would
// tell a reader work is waiting when nothing at that column ever will be
// picked up, and a block, though it stays meaningful wherever a card
// stands, is drawn only where one is actually raised.
//
// A card standing at such a column and genuinely carrying one of the three
// states, ready by the default every new card starts with, or blocked
// because a block may land at any column whatever its kind, is still drawn
// wherever the tree groups it: axisValueOrder's carried-value branch
// (tree.go:553-564) draws a group for any value some card actually carries,
// declared or not, which is what keeps this method's answer from ever
// dropping an occupied card out of a view whose root still counts it.
//
// The slice is fresh on each call, so a caller may sort or trim it without
// reaching the next caller.
func (s *Column) States() []string {
	if s.TakesWorkUp() {
		return []string{contract.StateReady, contract.StateActive, contract.StateBlocked}
	}
	return nil
}
```

`HoldsState` (`bench.go:207-217`) needs no change: it already just walks `States()`, so it now correctly answers `false` for `ready` and `blocked` at a queue column too, alongside the `active` answer it already gave.

### Why this is enough, and why nothing in tree.go changes

`declaredValues` already threads the resolved column into `column.States()` and returns whatever it gets, including an empty or nil slice; the `for` loop that consumes it (`axisValueOrder`'s first loop) simply iterates zero times when `States()` returns none. The second loop in the same function, the "carried" branch, is unconditional (it does not check `closedAxis` at all) and already exists solely to draw a group for any value some card carries that a closed axis's declared members did not name, exactly the case a card genuinely `ready` at a now-undeclaring queue column becomes. No code in `tree.go` distinguishes "declared but empty, suppress" from "declared but empty, draw anyway": that distinction was already fully expressed by what `States()` chooses to declare. Fixing the declaration is the whole fix.

Two comment blocks in `tree.go` currently overstate the old, symmetric promise and need re-wording so they do not read as contradicting this fix:

1. `groupsOn`'s doc comment (`tree.go:473-482`), which says a closed axis "draws a group for every member the workbench declares that the group's own column can hold, including the members holding nothing." Append: "A column `TakesWorkUp()` answers false for declares none of the state axis's members at all, so this rule draws no state group there whether or not a card stands in it; such a card is still drawn, through the open-valued 'carried' rule below."
2. `axisValueOrder`'s doc comment (`tree.go:507-536`), specifically the sentence "ready and active are drawn whether or not a card stands in them, which is the promise the paragraph above makes." Append a clause: "...for a column whose `States()` declares them. A column that declares no state at all makes no such promise, and a card standing there is drawn only where it actually stands, through the carried branch below, the same as any value the axis does not declare."

No other file under `internal/verb/` or `cmd/dinah/` needs a production change. `cmd/dinah/render.go` is untouched, so `cmd/dinah/row_sweep_test.go`'s line-number-keyed rows (`TestEveryRowStartsItsColumnsAtOneDisplayColumn`, `TestEveryStatementOfTheRenderingHeadIsCoveredOrNamed`) stay valid unedited, on the same ground dinah-275's AC-6 already established for the equivalent line.

## Tests to change

### 1. `internal/bench/columnkind_test.go`: `TestStatesAndHoldsStateAgree`

Currently asserts, for each of the four queue-kind cases (`an intake column`, `a done column`, `a buffer`, `a column waiting on somebody outside`):

```go
wanted := []string{contract.StateReady, contract.StateBlocked}
```

Change `wanted` to an empty slice (`nil` compares equal via `strings.Join`, both render `""`) for all four cases; the station case's `all := []string{contract.StateReady, contract.StateActive, contract.StateBlocked}` assertion is unchanged. `assertHoldsAgreesWithStates` (`columnkind_test.go:106-119`) needs no change: it derives its own expectation from `column.States()` rather than hardcoding one, so it keeps proving `HoldsState` never disagrees with `States()`, whatever `States()` now returns.

### 2. New tests in `internal/verb/tree_test.go`, over the existing buffer harness (`internal/verb/takesnowork_test.go:16-41`, `bufferIntake`/`bufferQueue`/`bufferDoing`/`bufferDone`)

**`TestAColumnThatTakesNoWorkUpDrawsNoStateGroupWhenEmpty`**, this card's positive case:

```go
func TestAColumnThatTakesNoWorkUpDrawsNoStateGroupWhenEmpty(t *testing.T) {
	h := newBufferHarness(t)
	ref := h.add("moved off the queue columns")
	h.at(ref, bufferDoing)

	built := treeOf(t, h, "", []string{FieldColumn, FieldState}, LevelCards)
	for _, column := range []string{bufferIntake, bufferQueue, bufferDone} {
		group := groupAt(t, built.Root, column)
		if len(group.Children) != 0 {
			t.Errorf("the %s column draws the state groups %v and holds no card",
				column, groupValues(group))
		}
	}
	doing := groupAt(t, built.Root, bufferDoing)
	want := []string{contract.StateReady, contract.StateActive}
	if got := groupValues(doing); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("the doing column draws the state groups %v and a station always draws %v", got, want)
	}
}
```

`h.add` files the card at `bufferIntake` (the leftmost column); `h.at(ref, bufferDoing)` moves it away as the operator, leaving `bufferIntake`, `bufferQueue` and `bufferDone` with no card at all. Before this fix, `bufferIntake` and `bufferDone` would each draw an unoccupied `ready` group (this is the exact defect the card title names); `bufferQueue` (`dinah.buffer` kind) is the second case the card's description says was confirmed by building the trunk binary. `bufferDoing`, a station, must still draw `ready` and `active` unconditionally (tier 3, unchanged by this card), which is what stops this test from passing by accident if a future change deleted the declared-group promise everywhere instead of only for queue columns.

**`TestABlockedCardAtAQueueColumnStillDrawsItsGroup`**, the regression guard for the "never drop an occupied card" invariant:

```go
func TestABlockedCardAtAQueueColumnStillDrawsItsGroup(t *testing.T) {
	h := newBufferHarness(t)
	ref := h.add("blocked while waiting")
	h.at(ref, bufferQueue)
	h.mustDo(&Request{Verb: Block, Card: ref, Actor: "alka", Reason: "waiting on a ruling"})

	built := treeOf(t, h, "", []string{FieldColumn, FieldState}, LevelCards)
	group := groupAt(t, built.Root, bufferQueue)
	want := []string{contract.StateBlocked}
	if got := groupValues(group); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("the waiting column draws the state groups %v and one card stands there blocked", got)
	}
}
```

`Column.States()` now declares nothing for `bufferQueue`, so this proves the group is drawn through the carried-value path rather than through a declaration, exactly the mechanism Decision D1 below says the fix relies on. Without this test, a future implementer could satisfy `TestAColumnThatTakesNoWorkUpDrawsNoStateGroupWhenEmpty` by deleting the carried-value branch entirely (making every queue column always draw nothing, occupied or not), which would silently drop a genuinely blocked card from the tree.

Existing tests need no change beyond these: `TestTheDefaultChainDrawsTheStatusTree` (`tree_test.go:493-546`) already asserts `intake` draws `{ready}` with two ready cards genuinely standing there, which the carried-value path still produces after this fix (nothing there was ever a declared, unoccupied group); `TestAFullyExpandedTreeHidesNothingAnywhere` (`tree_test.go:333-354`) already asserts no `blocked` group is drawn at `intake` when nothing is blocked, true before and after since `blocked` was already occupancy-gated; `TestNoCardFallsOutOfAClosedAxisTree`'s `FieldState` case (`tree_test.go:636-705`) groups the state axis standalone over a fixture whose declared union still includes a genuine work column, so `stateUnion()` (`tree.go:619-633`) still contributes `ready`/`active` from that column exactly as before.

### 3. `cmd/dinah/row_pairing_test.go`: `sweptStates`

This helper (`row_pairing_test.go:1742-1758`) predicts, for the CLI-level row-sweep suite, which state groups the real tree draws under one column, by reading `column.column().States()` directly:

```go
func sweptStates(column sweptColumnRecord, held []sweptCardRecord) []string {
	var drawn []string
	for _, state := range column.column().States() {
		if state == contract.StateBlocked && !sweptAnyStanding(held, state) {
			continue
		}
		drawn = append(drawn, state)
	}
	return drawn
}
```

Because it only ever iterates `States()`, it has no equivalent of `axisValueOrder`'s carried-value branch. Once `States()` answers `nil` for a queue column, this helper would predict zero groups under, for instance, `Intake` (`kind: contract.KindIntake`, one of the three columns `sweptInitColumns()`, `row_sweep_test.go:1925-1931`, gives every fixture the row-sweep suite builds) even while a card genuinely stands there ready, which is the suite's ordinary starting condition. The real tree still draws that group, through the carried-value path this spec relies on throughout; the helper must be corrected to match it or the row-sweep suite will assert the wrong expectation against a tree that is behaving correctly.

Rewrite it to mirror `axisValueOrder`'s two rules exactly: a state the column's `States()` declares is drawn unconditionally, except `blocked`, which additionally requires occupancy (unchanged); a state the column's `States()` does not declare is drawn only when some held card actually carries it, mirroring the carried-value path:

```go
func sweptStates(column sweptColumnRecord, held []sweptCardRecord) []string {
	declared := map[string]bool{}
	for _, state := range column.column().States() {
		declared[state] = true
	}
	var drawn []string
	for _, state := range []string{contract.StateReady, contract.StateActive, contract.StateBlocked} {
		switch {
		case state == contract.StateBlocked:
			if sweptAnyStanding(held, state) {
				drawn = append(drawn, state)
			}
		case declared[state], sweptAnyStanding(held, state):
			drawn = append(drawn, state)
		}
	}
	return drawn
}
```

The doc comment above it (`row_pairing_test.go:1742-1751`) describes only the old two-tier rule and needs the same addition Decision D1 makes to `axisValueOrder`'s: a column declaring no state at all draws a group only where a card is actually held there, the same rule an undeclared value already followed.

## Acceptance criteria

1. `internal/bench.Column.States()` returns `nil` for every column `TakesWorkUp()` answers `false` for (an intake column, a done column, a `dinah.buffer`, or any column with `AwaitingOutside: true`), and returns `{ready, active, blocked}` unchanged for every column it answers `true` for. Test hook: `go test ./internal/bench/... -run TestStatesAndHoldsStateAgree` green after the table rewrite above.
2. `HoldsState` continues to agree with `States()` for every case, including the four now-empty ones, with no change to `HoldsState` itself. Test hook: same run, `assertHoldsAgreesWithStates` covers this per-case already.
3. A tree grouped by column then state draws no state group at all under a column with no card standing in it and no owner taking work up there, an intake column, a `dinah.buffer`, or a done column alike, closing the defect the card title names. Test hook: `go test ./internal/verb/... -run TestAColumnThatTakesNoWorkUpDrawsNoStateGroupWhenEmpty`.
4. The same tree still draws `ready` and `active` under a station (a column that does take work up) whether or not either is occupied, unchanged from dinah-275's tier 3. Test hook: same test, the `bufferDoing` half of its assertion.
5. A card actually standing, blocked, at a column that declares no state at all still draws its own group there, rather than being silently dropped from a tree whose root count still includes it. Test hook: `go test ./internal/verb/... -run TestABlockedCardAtAQueueColumnStillDrawsItsGroup`.
6. `cmd/dinah`'s row-sweep suite predicts the corrected behaviour rather than the old one. Test hook: `go test ./cmd/dinah/... -run TestEveryRowStartsItsColumnsAtOneDisplayColumn` green with `sweptStates` rewritten as above.
7. No production call site of `TakesWorkUp`/`States`/`HoldsState` beyond `internal/bench/bench.go` changes behaviour. Test hook: `grep -rln "TakesWorkUp\b\|\.States()\|HoldsState" --include="*.go" . | grep -v _test.go | grep -v '\.claude/worktrees'` resolves to the same 8 files this spec's call-site census names, both before and after implementation. This is a weaker proof than a behavioural one: it shows no *new* file matches the pattern, not that an existing file's use of the predicate could not itself have silently changed meaning; ACs 1-6 are what cover that.
8. `go test ./...` is green at the branch tip.

## Decisions

**D1. The fix lives in `internal/bench.Column.States()` alone; `internal/verb/tree.go`'s grouping recursion needs no code change, only doc-comment corrections.** `tree.go` already threads the resolved column into `States()` (via `declaredValues`) and already has a mechanism, the carried-value branch in `axisValueOrder`, that draws a group for any value a card actually carries beyond what a closed axis declares. That mechanism was built for a card standing at a column or state the workbench no longer declares (dinah-275 AC-4); a card genuinely `ready` at a column whose `States()` now declares nothing is the same shape of case, and the existing mechanism handles it with no new parameter, no new branch, and no second copy of the occupancy rule. This is what the single-predicate discipline this workstream (`dinah-207`, `dinah-253`, `dinah-273`, `dinah-275`) asks for: fix the one place the rule is declared, and let every existing reader of that declaration answer correctly on its own.

**D2. No change to `docs/spec/core-profile.md` or `docs/design/format.md`.** The tree's grouped state breakdown is a Dinah tool feature (`internal/verb/tree.go`, the `tree` verb), not a wire-format concept; `docs/spec/core-profile.md` never mentions a tree or a grouped display at all (confirmed: `grep -n -i "\btree\b" docs/spec/core-profile.md` matches only its own citation-policy sentence at line 15). CORE-CARD-5 fixes every card's literal state to one of `ready`/`active`/`blocked` regardless of the column it stands in, and `docs/design/format.md:288` already glosses `ready` as meaning "pullable," which a card sitting in an intake column or a buffer literally is. What this card changes is only which of those literal, wire-legitimate values a *display* groups by, per column kind, a question the format leaves entirely to the tool. This is the same ground dinah-275's D-2/D-3 already used for its own parallel change, and dinah-275 touched no doc file beyond `tree.go`/`bench.go`'s own comments for the same reason.

**D3. Every column kind's answer, stated once rather than four times.** `TestTakesWorkUpAnswersForEveryKind` (`internal/bench/columnkind_test.go:18-38`) already enumerates the full case table this rule turns on: a plain work column, and a kind this build does not implement (which CORE-STATE-12 reads as a work column), both answer `TakesWorkUp() == true` and so declare all three states; an intake column, a done column, a `dinah.buffer`, and a work column with `AwaitingOutside: true`, all answer `false` and so, after this fix, declare none. This card adds no new branching to that table and no new exception: `States()`'s only change is which slice it returns for the `false` side of the one boolean that table already tests exhaustively.

**D4. `internal/verb/query.go`'s `closedValues(FieldState)` (`query.go:451-453`, the full `ready`/`active`/`blocked` triple used to validate a typed filter term such as `state:active`) stays untouched.** It validates whether a literal filter value is a member of the state vocabulary at all, independent of any column, and never draws a group or claims a card can reach the value; this is the same ground dinah-275's D-3 already gave for the identical function. A card standing at an intake column can, after a hand edit, genuinely carry `state: active` (`dinah check` reports the anomaly, per `internal/bench/check.go:313-320`'s `FindingClaimWhereNoWorkIsTaken`, but does not refuse it, per `docs/design/format.md`'s "Manual edits are witnessed, not prevented"), so narrowing this validation by column would be a new and unrelated restriction this card was not asked to make.

## Branch

dinah-322-a-queue-column-still-draws-a-ready-group-though-a-card-in-one-is-not-ready-or-unready
