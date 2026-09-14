---
title: A card was worked in a column that takes no work up, and nothing about the workbench showed it
column: 5ea2db0272fc
state: ready
severity: major
priority: next
tier: frontier
links:
  - kind: relates_to
    to: 1f944b3dea59
---
On 2026-09-04 a card went Triage, then Design Queue, then Agent Design Review, and never entered Spec. The spec work happened in Design Queue, whose own instructions say that a pull from it promotes the card into Spec and that nothing else happens there. The instructions were correct, they were sitting where anyone could read them, and the card walked past them without a word.

Paul's response is why this is a card rather than an apology: "Expecting you to try harder is an utter non-starter. This is a product problem." He is moving his own work off Andoneer and onto Dinah, and if Dinah's instructions are followed only when an agent happens to read them, the tool has no claim on anybody's behaviour.

## What this card is, and what it is not

The incident happened on Andoneer, and the columns named above are Andoneer's. The precedent it originally cited, a refusal of a forward move over a station that waits on somebody outside, is also Andoneer's. Dinah has no lanes, no stations, and no such refusal. So the evidence here is not evidence about Dinah's behaviour.

What is Dinah's is the requirement. A workbench should be able to tell that a card was worked somewhere its own instructions said no work happens, and a reader afterwards should be able to see it. That requirement stands whether or not Andoneer has the same problem.

## Why it came back to Intake

Specified once and returned by Paul on 2026-09-06. The spec that ran translated the incident into a Dinah defect about the distance a move travels, retitling the card accordingly, and in doing so dropped two of the three things this card originally asked for. Distance was the least of them. The agent did not jump because it misjudged distance; it obeyed a dispatch that contradicted the column, and nothing afterwards could tell the result from a card that had gone through properly.

The distance work is not wasted. It reached Operator Design Review with six acceptance criteria and a settled decision that it needs no lane concept, since a column's position is a running index over one sequence. If it is picked up again it should be its own card rather than this one.

## The three things originally asked for, restored

Refuse or mark a move that steps over a column the card's own route was meant to pass through, so the skip is either prevented or leaves a trace.

Serve a column's contract to whoever writes the dispatch, before the work, rather than only to the agent that arrives. The dispatcher is where the competing instruction gets written, so that is where the column's terms need to land. A dispatch composed against a column's contract cannot contradict it by accident.

Make a card that is worked while unclaimed, or worked in a column that takes no work up, visible as such. Both look exactly like idleness to a reader today. The second is this incident.

Whoever picks this up should treat those as candidates rather than a plan, and should say plainly which of them is Dinah's to solve and which is an Andoneer problem wearing a Dinah card. Paul's requirement is that the workbench notice, not that it notice in a particular way.

## Specification

## Restating the defect in Dinah's own terms

The card as filed described a "station on a lane," which is Andoneer's vocabulary. Dinah has neither word. Dinah's noun is the workbench, a workbench declares columns, and each column carries a `Position`. A move names a destination column and the request is legal or refused; nothing in that decision reads how many other columns sit between the departure and the destination.

Confirmed against `internal/verb/mutate.go`:

- `canRoute` (mutate.go:318-333) resolves the destination purely by looking it up with `l.Bench.ColumnByRef(req.Column)`. It does not compare the destination's `Position` to the departure's beyond deciding forward-or-backward.
- `canLand` (mutate.go:359-421) runs the CORE-MOVE rows in order: blocked, held, terminal-departure, capacity, loop-limit, takes-no-work-up, operator-reserved, retiring. None of these rows reads whether the destination is the very next column, five columns on, or the last one in the workbench. A move from column 1 to column 2 and a move from column 1 to column 5 pass exactly the same checks.
- `legalMoves` (internal/verb/library.go:500-527) iterates every declared column, classifies each as `Forward` or `Backward` by comparing `Position`, and skips only a forward entry when the current column is terminal. Every other column, however far away, is listed as a legal move. Confirmed by reading the loop: the only `continue` in it is the current-column skip and the terminal-forward skip.
- `Event` (internal/bench/journal.go:17-47) and the `moved` event's own field list in `docs/design/format.md`'s schema table carry `from`, `from_title`, `to`, `to_title`, and the two conditional flags `override` and `reject`. There is no field recording the columns between departure and destination, nor how many of them there were. A one-column move and a five-column move write the same shape, differing only in the two column identifiers.

So the defect is: **Dinah's move logic enforces no relationship between the size of a jump and its legality, and the record of a move does not distinguish a jump from an adjacent step.** That is true today with a single, ordered list of columns per workbench. It has nothing to do with more than one path existing.

## dinah-209 is not a precondition

dinah-209 ("Lanes, so that some cards travel a different path than others") asks for a workbench to declare that different cards take different routes through its columns. Read in full: it was split off from a second need (a queue column, which `awaiting_outside` already covers) and it stands deferred because every workbench the operator runs today has one route, not several.

That is exactly the condition under which this card's defect is well-defined without lanes, and the condition is stronger than "no workbench happens to use more than one route today." A workbench's columns are read from a single ordered sequence: `internal/bench/bench.go`'s opener reads one sequence key off the anchor (`ids := fm.Seq(vocab.SequenceKey)`, bench.go:1543) and assigns each column's `Position` as the running index over that one list while walking it (`readColumnIn(root, vocab, id, len(b.Columns))`, bench.go:1559). The on-disk format has no second sequence to index against, so it cannot declare a second route at all today, not merely that nobody has declared one. Under that format, `Position` alone gives a total order, and "how many columns did this move step over" is `destination.Position - departure.Position - 1` when the move is forward, no lane concept required.

If lanes land later, whatever check this card adds has to be re-read against a workbench that declares more than one route (the check would need to walk the card's own route rather than raw `Position`), and that re-read is dinah-209's job when it is picked back up, not this card's.

## Relationship to dinah-387

dinah-387 is about a claim that cannot name a distinct holder, so two sessions sharing one identity can both believe they hold a card. That is a defect in exclusivity at the point of taking work up. This card is about a move that lands somewhere the move's own record cannot distinguish from an adjacent step. The two touch neighbouring code (`internal/verb/mutate.go` and `internal/bench/journal.go` back both), but the checked-out lines do not overlap: dinah-387's fix space is `canLand`'s held/claim rows and `bench.Card.Holder`/claim plumbing, and this card's fix space is `canRoute`, the loop in `legalMoves`, and the `moved` event's field list. Implement either one first; neither's acceptance criteria touch a line the other's would change.

## What is not disputed

Nobody is asking whether a jump should be possible at all. The operator's own working agreement, quoted on this workbench's instructions, describes "the Fix lane and any fast-tracked traversal" as routes that skip stations deliberately, and CORE-MOVE-9 already lets an operator override a capacity limit explicitly. A jump is sometimes exactly the right move. What is missing is that the workbench cannot yet tell an intentional jump from an accidental one, before or after the fact.

## The product question (operator's call, filed below as an open question)

Three different responses to "a move's destination is more than one column past its departure, on the card's own single route" are all real products, and this spec does not choose among them:

**Refuse it as a new CORE-MOVE row**, the same shape as CORE-MOVE-7 (terminal-departure) or CORE-MOVE-4 (capacity), reporting a new refusal name, admissible only under an override marker the way CORE-MOVE-9 admits capacity overrides. This buys a workbench that cannot silently lose a station the way dinah-376's own filed incident describes. The cost is a capability every workbench has today, not a cost to any particular one: `canRoute` (mutate.go:318-333) admits a jump of any size for any owner, and it restricts nothing by who is asking except the override marker itself, which CORE-MOVE-11 already reserves to the operator (mutate.go:326-327, `if req.Override && !operator { ... refuse(... NotOperator ...) }`). A refusal that follows the same pattern therefore takes a capability every owner has today, jumping several columns in one move, and narrows it to the operator alone, on every workbench that adopts the new revision, whatever that workbench uses jumps for. It needs no lane concept, because it reads `Position` on the single route. It is the only one of the three that changes the published CORE-MOVE contract: a new mandatory refusal is a new `MUST` row, which is a protocol change to `dinah-core`, not a build-local one (see Constraints below).

**Record it and let it stand.** Add the optional field to the `moved` event (a count of columns skipped, or a boolean, decided at spec time once the operator picks this option) and change nothing about whether the move is admitted. This buys a record that finally distinguishes a jump from an adjacent step, at no cost to any existing route: nothing that works today stops working. It cannot, by itself, stop the kind of accident dinah-376's own description opens with, since the card still lands and gets worked before anyone reads the journal. It needs no lane concept and no CORE-MOVE change, only an additive journal field and the compat-fixture work below.

**Report it later on a health read.** `dinah check` already runs a battery of read-only findings against a workbench's journal (`internal/bench/check.go`'s `Finding` catalog: `FindingClaimWithoutActive`, `FindingPositionDiverges`, and siblings). A new finding key, scanning `moved` events for a departure/destination pair more than one `Position` apart on the card's route, would surface every jump the next time somebody runs `dinah check`, without touching `canRoute`, `canLand`, or the journal schema at all if the finding is computed from the existing `from`/`to` fields rather than a new one. This buys visibility with the least code and no schema change, and it costs the most in time-to-notice: a jump sits unreported until somebody runs `check`, which nothing on this board currently does on a schedule.

The three are not mutually exclusive; recording and reporting could both ship, and refusing does not preclude also recording. But which one (or which combination) ships, and whether a jump is refused by default or merely marked, is a product commitment the operator has to make, because it decides who may make a multi-column move at all on every workbench built against the new revision.

## Constraints on any answer

**The published move contract.** CORE-MOVE-1 through CORE-MOVE-11 (`docs/spec/core-profile.md`, lines 916-936, tabulated again at lines 1488-1498) is the closed list a conforming build promises to run, in the order `dinah help move`'s ratified table prints (`cmd/dinah/main_test.go`'s `ratifiedMoveRefusalTable`, confirmed present at line 7423). Adding a refusal row is adding a `CORE-MOVE-12` `MUST`, at whatever position in the evaluation order the row belongs (`canRoute` vs. `canLand` per the existing split, since that split already tracks which fields the row reads). That is a protocol change: it moves the ceiling from `dinah-core 0.12` (the version this repository's `core-profile.md` declares at line 3) to a new minor revision, because a build declaring the old revision is not required to run the new refusal, and a client written against the old revision does not expect the new refusal name. It requires updating the numbered list, its summary table, the ratified `dinah help move` table and its golden test, and the window-of-revisions machinery `docs/design/format.md` describes (lines 1657 onward) so a workbench declaring the old revision still opens under the old rules. Recording or reporting-only need none of this, since neither adds a `MUST`.

**The journal event schema.** `docs/design/format.md`'s "Journal event schema" section (lines 793-928) states the rule that governs every option here: adding a field to an existing event is safe, because "a reader ignores a key it declares no field for," and nothing rewrites a line already written. So a new optional member on `moved` (for the record-it option) needs no format-version bump and breaks no old reader. It does need the schema table itself updated (the `moved` row at the point listing `from`, `from_title`, `to`, `to_title` always-present and `override`/`reject` conditional) to name the new field and its condition.

**The compatibility fixtures.** `internal/bench/testdata/compat/` fixtures are frozen per profile revision (per this workbench's own instructions, and confirmed by the manifest/digest machinery in `internal/bench/compattest/compattest.go`). `cmd/dinah/compat_test.go` enforces two things that any schema change here has to satisfy: `wantedEvents[contract.EventMoved]` (line 96) is the member set the population sequence is asserted to write for `moved`, and `TestTheSampleFixtureCarriesEveryShapeThisBuildWrites` (line 168) asserts the *sample* fixture contains every member the current build's own replay writes for every event. Concretely: if the record-it option adds a field to `moved`, `populate.txt` needs a step that actually produces a jump so the field gets exercised, `wantedEvents[contract.EventMoved]` needs the new member name added, and the sample fixture for the revision this build stamps needs recapturing with the repository's own capture script (never by hand) so the sample-alarm test does not fail the moment the new member appears in a fresh replay but not in the frozen sample. Refusing needs the same recapture if the refusal path is exercised by `populate.txt` and if the refusal changes anything written to the journal (it should not, since a refused move writes nothing). Report-only, computed from existing fields, needs no fixture change at all.

## Decisions

- This card does not depend on dinah-209 landing first; see above. (resolved as D-1)
- The three-response product question stays open rather than being picked here; see OQ-1. (resolved as D-2)
- Whichever response the operator picks, it is scoped to a single-route workbench (today's reality); a re-read for multi-route workbenches is dinah-209's job, not this card's, and this card's acceptance criteria below are written against `Position` alone. (resolved as D-3)

## Acceptance criteria

AC-1. A test exercises a move whose destination's `Position` is more than one greater than the departure's `Position`, on a workbench declaring one route, and it currently succeeds with the same refusal outcome (none) as a move to the immediately-next column. The criterion is satisfied once the operator's chosen response is implemented and this same test's outcome changes to match that response: refused unless overridden, or admitted with the new journal field/finding present. A run that still admits the jump silently, with no field or finding distinguishing it from an adjacent move, fails this criterion.

AC-2. `legalMoves` for a card standing at a non-terminal column, on a workbench declaring at least four columns, is asserted against the full column list: every column at a `Position` greater than the current one is present with `Direction: Forward` and every column at a lesser `Position` is present with `Direction: Backward`, matching today's behaviour exactly unless the operator's chosen response also changes what `legalMoves` offers (a refusal-shaped response might drop a column more than one step forward from the offered list, or mark it distinctly; a record- or report-only response does not touch `legalMoves` at all). Whichever holds, the test names it explicitly rather than asserting silence.

AC-3. If the chosen response adds a field to the `moved` event: a fixture-replay test (extending `populate.txt`) produces one `moved` line for an adjacent move and one for a jump, and asserts the new field's value differs between the two lines in the way the response specifies (present vs. absent, or a count that is 0 vs. greater than 0). A run where both lines carry the same value for the new field fails this criterion.

AC-4. If the chosen response adds a CORE-MOVE refusal: `dinah help move`'s printed table carries the new row at the position the evaluation order assigns it, and `cmd/dinah/main_test.go`'s golden comparison against that table is updated and passes; a run against the old, un-updated golden string fails, which is what proves the golden string was actually exercised rather than merely trusted.

AC-5. If the chosen response adds a `dinah check` finding: a workbench fixture carrying one journal with a jump move and one with only adjacent moves is checked, and the jump-carrying one reports the new finding key while the adjacent-only one reports none of it. A run where the adjacent-only fixture also reports the finding fails this criterion (false positive), and a run where the jump fixture reports nothing fails it the other way (false negative).

AC-6. Whatever fields or events the implementation adds to the journal schema, `docs/design/format.md`'s schema table is updated in the same change to name them, and `wantedEvents[contract.EventMoved]` in `cmd/dinah/compat_test.go` is updated to include any new member name actually written; a run of `TestTheSampleFixtureCarriesEveryShapeThisBuildWrites` and `TestReplayingThePopulationSequenceReachesEveryShapeItNames` after the change passes without a stale sample fixture, which requires the fixture recapture named in Constraints when the sample no longer contains the new shape.

## Open questions

OQ-1 (filed as an open_question, owner=operator, gate=Test): the product question above, refuse / record-and-let-stand / report-later-on-a-health-read, or some combination, laid out with what each buys, what it costs, and what it cannot do.

## Out of scope

- Anything conditioned on more than one route existing per workbench (dinah-209's territory).
- Any change to `canLand`'s held/claim rows, `bench.Card.Holder`, or claim exclusivity generally (dinah-387's territory).
- Choosing the shape of a new refusal name, journal field name, or finding key: that follows from the operator's answer to OQ-1 and is Implement's job once the answer lands, not this spec's.
