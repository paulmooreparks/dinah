---
title: Move this project's own development onto Dinah
column: 4fda9c9ca779
state: ready
severity: major
priority: soon
tier: frontier
workstreams:
  - b3f924406e4c
---
The card that settles what dogfooding requires, and then tracks whether we have got there. It exists because "how close are we?" has been asked several times and answered from memory each time, which makes the answer drift.

**This card is worked, and it is worked first.** An earlier version of this description said nobody works it directly, which contradicted the rest of it and would have sent the gap cards ahead of the decision that governs them. The design half of this card produces one thing: a ruling on which of the four gaps are requirements and which are Andoneer habits this project would drop on the way over. Every other card in this workstream is re-weighed against that ruling rather than worked before it. After the ruling lands, this card stays open as the tracker and closes when its criteria pass on a real workbench.

**What Dinah can already do**, established on 2026-09-08 by reading the code rather than the cards: cards, columns, claims, moves, blocks, comments, attachments, workstreams, levels and tiers, and the whole checklist-item lifecycle in both directions. dinah-206 gave items their write side and dinah-435 gave a reader what they say.

**The four gaps**, each with its own card in this workstream: gating (dinah-450), citations that write but cannot be read back (dinah-438), card links that cannot be written (dinah-436), and lanes (dinah-209).

**What "moved" has to mean, so this card can close honestly.** Not "Dinah could in principle run a board." This board, with its own columns, its own routing and its own discipline, running this project's cards, with the operator working from it instead of from Andoneer.

**The scope question, which is the design half's whole job.** Take each gap and ask whether the pipeline genuinely needs it or whether it is a habit worth dropping. Lanes exist here because Triage routes a card down one of several paths; a single-route board is a real option and a simpler one. Gating exists because criteria and questions hold cards at stations; Dinah already refuses a claim on a card whose questions are unanswered, and that blunter rule may be enough. Citations and card links are likelier to be genuine, since evidence you cannot read back is not evidence and a duplicate you cannot link is a duplicate nobody finds. Rule on each one, say which are requirements and which are preferences, and cut the preferences rather than porting them.

**Deliberately out of scope.** Andoneer keeps running the portfolio: this card moves one project, not the operator's whole working life. Nothing here asks Dinah to grow a board UI, which the product framing rules out. The hosted product is a separate matter, since it runs on Dinah's library rather than being a second implementation.

**The alternative this card must weigh rather than assume away.** The first non-Andoneer workbench was always meant to be a GK colleague's eleven-step Jira workflow, and a simpler board is a better first customer than this one. Dogfooding on a workbench Dinah can already serve teaches more, sooner, than closing four gaps to reach parity with a fifteen-column pipeline. If that is the right call, say so here, and the gap cards get re-weighed against it rather than worked by default.

## Specification

## What this card delivers

The design half of this card is a ruling and it stands unchanged below. The tracker half is a cutover, and the section "What 'moved' means, concretely, and what the cutover pass must do" is its contract. Nothing here asks Implement to write Go: the cutover is two migration commands, workbench data, workbench prose, and a short list of commands, some of which are the operator's alone to run.

## Verification method

Each of the four claims in the description was re-read against the trunk at commit `8661604`. Those paragraphs describe a tool that has since moved, and they are kept as the record of what the ruling rested on rather than as current fact; the cutover section re-establishes all of it against `78ff230`.

- **Gating.** `internal/verb/checks.go` lists the `Move` checklist (`check.move.1`..`check.move.9`): `UnknownCard, UnknownColumn, NotOperator, NotOperator, Blocked, Held, Terminal, AtCapacity`, plus the loop-limit row appended at 9. `UnresolvedItem` is absent from that list; it appears only in the `Claim` list (`check.claim.7`) and in `checkLists[Pull]` (`check.pull.13`). `claimableItems` (`internal/verb/mutate.go:138`) reads `bench.BlockingItems(card.Dir)` with no column argument, which resolves to `bench.ItemBlocksClaim` (`internal/bench/entity.go:914`). `ItemBlocksClaim` (`internal/bench/entity.go:926`) refuses a claim only for `item.Kind == "open_question" || item.Kind == "decision"`; an `acceptance_criterion` never blocks, by the function's own comment: "it is verified after the work rather than before it, so blocking a claim on one would refuse the card the very work that lets anybody verify it."
- **Citations.** `docs/design/format.md:1539` and the `citations:` example at `format.md:966` document `scheme`, `target`, and a nested `observed: {before, after}` per entry. At `8661604`, `arrayFromChildren` iterated only the children at the shallowest indent, so every line indented beneath a dash was skipped, and `CountCitations` only counted. dinah-438 was filed for the fix.
- **Card links.** `docs/design/format.md:1559`: "Nothing in the tool reads a link. Pull order does not consult one, no verb refuses because of one, and the CLI shows a card's links and computes nothing over them." At `8661604` a repository-wide search for a link writer (`grep -rn '"links"' --include=*.go .`, and separately `grep -rln 'func.*Link' --include=*.go . | grep -v _test`) turned up only the reader, the view, and an unrelated release-cut flag, and `cmd/dinah/commands.go` registered no `link` command.
- **Lanes.** `grep -rn '"lane"\|Lane\b' --include=*.go internal` returns exactly two lines, both test vocabulary lists that happen to contain the word: `internal/profile/extract_test.go:35` and `internal/verb/search_test.go:796`. Neither declares a type, a field, or any behavior. The format has no lane concept at all: every card in a workbench travels the same column sequence.

## The ruling

Four gaps, four verdicts. Each verdict says what the pipeline needs rather than what this pipeline happens to have, and names which layer the capability would sit in if it were ever built. Three are recorded as resolved `decision` checklist items so Agent Design Review can check them against this text rather than against memory; the fourth (gating) was recorded as an operator-owned `open_question`, and the operator answered it on 2026-09-09.

### 1. Gating (dinah-450): required, generalized. Not Andoneer parity: a domain-neutral primitive the contract was missing for one item kind.

**This verdict changed on the 2026-09-09 pass, on operator instruction received while writing it, and this section was itself pushed back once for crediting him with more of that instruction than he gave.** The first draft argued that Dinah's existing global claim/pull refusal makes per-column gating unnecessary, and Agent Design Review found that argument checked open_question/decision coverage and generalized the result to "gating" as a whole, when `bench.ItemBlocksClaim` excludes `acceptance_criterion` from that refusal by design. The operator then added one constraint before this ruling closed: Dinah has to stay usable for non-software workbenches (a GK colleague's eleven-step Jira workflow is the first one planned), so no verdict here may bake a software-specific concept like "merge" into what the contract enforces. That is the whole of what he said.

The mechanism/policy split below, and the "required" conclusion it leads to, are that pass's own reading of that constraint, not a ruling he made. Two things want keeping separate: the *mechanism* (an item names a column; a workbench-declared rule refuses passage while the item is unsatisfied, expressible entirely in terms of columns, items and refusals) and the *policy* (which column, which kinds, which workbenches use it at all).

Read against that split, the exclusion the review found is itself a policy decision compiled into the core: `ItemBlocksClaim` hardcoded, for every workbench that will ever exist, that an `open_question` or `decision` always blocks and an `acceptance_criterion` never blocks anything, anywhere. A workbench that wants its criteria held until some station had no way to ask for that, and a workbench that wants no blocking at all had no way to decline the other half either. That is a domain assumption escaping into the one layer every workbench shares, which is exactly the shape the DIRT principle and the operator's product framing both rule against.

**This split was not invented for this card.** `docs/spec/core-profile.md` section 6.4 already stated the identical shape: "A workbench MAY mark a column as requiring a named structured item to be resolved before a card enters it" (CORE-GATE-1) and "A tool MUST refuse a move into a column so marked... while the card carries the item that column names and that item is not resolved" (CORE-GATE-2), reporting the refusal name `unresolved-item`. Neither statement exempts any item kind.

**Verdict: required.** Not because Andoneer gates per column and this project's board should match it, that is the reason this ruling explicitly rejects, but because the contract needs a domain-general way for a workbench to declare its own hold policy. He ruled on 2026-09-09: build it first. dinah-450 reopened, and dinah-473, dinah-474, dinah-477 and dinah-484 carried the mechanism the rest of the way. **Layer:** contract-core (`internal/verb` claim/move/pull checks, `internal/bench` for the workbench-declared gate table), reusing the existing `item.column` field for storage.

**The operator's 2026-09-08 Andoneer ruling, read in this light, supports the mechanism rather than complicating it.** That ruling ("an acceptance criterion gates on Merge specifically... replacing a flat rule") is itself an instance of a workbench declaring its own hold policy in its own vocabulary: Andoneer's Merge column is Andoneer's word for the station its own contract needs a criterion held against, not a concept the tool enforces by name. Dinah's core adopts no "Merge" concept. The Jira workbench would declare its own column for the same underlying need, in its own words, or declare none at all.

### 2. Citations that write but cannot be read back (dinah-438): required. Landed.

A citation that cannot be read back is not evidence, and the citation obligation format.md documents is hollow if nothing downstream of the write can see what was cited. This was a defect in already-shipped machinery, not an Andoneer habit to weigh against a simpler alternative. **Verdict: required.** **Layer:** contract-core (`internal/bench/blockjson.go`'s `arrayFromChildren`); the fix generalized the existing block reader rather than adding a citations-specific one.

### 3. Card links (dinah-436): required, at the scope already drafted. Landed.

`docs/design/format.md:1552`'s own ruling on links, settled independent of this card, is that a link carries zero contract behavior. That answers the harder question a write verb usually raises before dinah-436 had to ask it, which is why dinah-436 correctly scoped itself as two small verbs over an already-documented shape.

The only question left was whether an inert, unenforced annotation is worth building at all. It is, for a reason this very card demonstrates: dinah-449 carries three `parked_behind` links and two `relates_to` links on the Andoneer side, recording exactly the dependency structure that governs which cards may enter Design Queue and when. A workbench with no way to write that relationship pushes it entirely into prose, which is the same drift this card's own description names as its reason for existing. **Verdict: required.** **Layer:** contract-core (`internal/bench/card.go`'s `readLinks` sibling).

### 4. Lanes (dinah-209): not required. Drop it, consistent with the operator's standing ruling. dinah-209 stays filed, not closed.

dinah-209 already carries the operator's own verdict: "deferrable for the work he is tracking now, since every card on his current board travels the same seven states." The only new fact this ruling adds is that the board being moved is not a single-route board today: it has three lanes and gates that key off which lane a card travels.

**Verdict: not required**, and the reshaped workbench this ruling produces is single-route by construction. **Layer, if ever built:** workbench-shape (the column/route model in `internal/bench`), not the CLI surface; the "no board UI" constraint is untouched either way.

**What a future lane addition would do to the hold this ruling asked for.** The storage does not change: an item still names one column via `item.column`. What changes is the refusal check, which today only has to ask "is the named item settled," and under lanes would also have to ask "does this card's route ever reach the named column at all." That second question is new logic, not a rewrite of the first.

**Consequence for dinah-209 itself: the card stays filed and deferred.** This ruling does not close it and does not reopen it.

## Consequences

- **dinah-450 (gating).** Required, reopened and rescoped, and the operator ruled "build it first" on 2026-09-09. Built.
- **dinah-436 (card links).** Required, unchanged from its scope. Landed.
- **dinah-209 (lanes).** Not required for the move. Stays filed and deferred. Not closed, not reopened.

## The alternative this card must weigh: the GK workbench first

The card poses this as a choice: move here now, or let the GK colleague's eleven-step Jira workflow be the first non-Andoneer test, on the reasoning that a simpler board teaches more, sooner, than closing four gaps to reach parity with a fifteen-column pipeline.

The premise inside that reasoning was the fifteen-column, three-lane parity target, and this ruling rejects that target rather than arguing around it: lanes are dropped and the reshaped board is single-route. But the lift was not as small as the first draft claimed. What this ruling actually asked to be built was dinah-436's two small verbs, dinah-438's citation-readback fix, and a domain-general per-column hold. That third item was real contract-core work, and it would have been dishonest to call this "close to the same size of lift" as the GK workbench without saying so.

What changes the comparison is that the hold is not board-specific work. It is the same domain-general primitive the operator's constraint asked for, expressible without "merge" or any other software word, and a workbench that wants to hold a step until some condition clears needs exactly this capability. Building it, driven by this project's own need, was not Andoneer-parity debt; it is shared infrastructure the GK workbench draws on rather than re-deriving.

**Verdict: proceed with the move, on the reshaped board this ruling defines, without waiting on the GK workbench.** The GK workbench remains the harder, richer test of the tool's domain-neutrality as a whole, since it is someone else's process rather than a shape this board can already lean toward; nothing here changes its priority or schedule.

## What "moved" means, concretely, and what the cutover pass must do

Rewritten over four rounds of Agent Design Review, then conformed on 2026-09-14 to the operator's ruling on OQ-2. The whole of it was established against trunk `78ff230` **by building the binary into a scratch directory and running it against copies of the real workbench**, with `DINAH_HOME` pointed at a scratch directory and the real workbench read but never written. Where a step below says the tool does something, it was made to do it and the output is quoted.

### What the Dinah workbench holds today

Read at `c:\Users\paul\source\repos\dinah\.dinah\149f228d48c3`. That path is correct only until step 1 of the order below runs, and step 1 changes it; see "The workbench's folder name does not survive".

- Fourteen column directories, carrying the Andoneer board's own column identifiers, in board order: Intake `5ea2db0272fc`, Triage `a2eb2436b77d`, Design Queue `2f6c18c9f5d0`, Spec `ca3badf49985`, Agent Design Review `0d86ad99cdbc`, Operator Design Review `5729d4578008`, Build Queue `0789fd2dbefd`, Implement `4fda9c9ca779`, Agent Code Review `4b38abe7ebd5`, Operator Code Review `ee29487fad76`, Test `c9428b3bc921`, Merge `6c5b9d6f4414`, Acceptance `b69abf918c42`, Done `aa6cd1c6ae5f`. Andoneer's Complete is Dinah's Done. Andoneer's Refused column has no counterpart here.
- Zero cards, zero workstreams, no `card-numbers.txt`, and `grep -rn "gate_items" | wc -l` returns 0, so all fourteen columns hold neither way.
- Exactly three columns declare `operator_owned: true`: Operator Design Review, Operator Code Review, Acceptance. Two of those three come off; see the table.
- The anchor declares `format: 1` and `profile: dinah-core/0.7`. The binary implements `StorageFormat = 3` (`internal/bench/bench.go:80`) and the profile ships at 0.12.
- The fourteen column bodies total 707 lines, copied mechanically from Andoneer.

### Two migrations run before anything else, and their order is fixed

**The workbench predates the card-number registry, so the first `dinah add` is refused outright.** Reproduced verbatim:

```
$ dinah add "probe card one"
dinah.needs-number-migration C:\...\.dinah\149f228d48c3 predates the card-number
registry, so a new card cannot be given a number; run `dinah check
--migrate-numbers --yes` to build it, then file the card again
[exit 2]
```

**Running that first is wrong, and it is silently wrong.** `--migrate-numbers` stamps `format: 3` on the anchor. Format 3 is at or above `ContainerFormat = 2` (`internal/bench/bench.go:91`), which makes the containment rule bind, and the workbench then fails it and will not open at all:

```
$ dinah add "carried card A"
dinah.needs-container-migration Dinah no longer opens C:\...\.dinah\149f228d48c3,
because a workbench lives inside a .dinah container and this one does not; run
`dinah check --root C:\...\.dinah\149f228d48c3 --migrate-container` to carry it
into one, then open it again
```

The container migration then stamps `format: 2`, overwriting the 3, so the first migration's work is thrown away and has to be run again. **Container first, numbers second.** The two commands, in order, run from `c:\Users\paul\source\repos\dinah`:

```
dinah check --root c:\Users\paul\source\repos\dinah\.dinah\149f228d48c3 --migrate-container --yes
dinah check --migrate-numbers --yes
```

Run without `--yes` the container migration previews and writes nothing, exiting 5 with "Nothing was written. This repair moves directories, so read the list above before you authorize it." Run it that way first.

**What they do to the existing definition, stated plainly.** On this workbench, which holds no cards, `--migrate-numbers` writes no registry file at all ("Wrote 0 lines of the card-number registry. Renumbered 0 cards.") and changes exactly one thing, the `format:` key on the workbench anchor. `--migrate-container` changes exactly one thing too, and it is not a key.

**No command reports the storage format.** `dinah export`, `dinah status --format json`, `dinah workbench --format json` and `dinah check --format json` were each run and none carries a `format` member; export's top-level members are `columns`, `groups`, `instructions`, `levels`, `profile` and `title`. Anything checking the format reads the `format:` key from the anchor file and says that is what it is doing.

### The workbench's folder name does not survive, and every path in this card changes with it

The container migration renames the workbench directory:

```
Renamed 1 workbench already in a container.
C:\...\.dinah\149f228d48c3 -> C:\...\.dinah\01a09b8d3243700eba5ab248775cbe93
```

`IsWorkbenchID` (`internal/bench/storage.go:245`) requires `WorkbenchIDLength = 32` lowercase hex decoding to a UUID whose version field is 7, and its own comment says the two predicates "are disjoint by their lengths alone, which is what makes a legacy workbench directory and a migrated one impossible to confuse." A 12-hex directory name is a legacy name by construction, `dinah check --migrate-container` reports it as `legacy-name`, and no flag anywhere preserves it.

**What cannot be kept is the folder name, not the identity.** A 32-hex name embedding the old identifier is legal, and it was tried rather than assumed: a directory called `149f228d48c37a2b8000000000000000` satisfies the predicate and the tool opened the workbench from it without complaint. It is still the wrong answer, because the first 48 bits of a UUIDv7 are the milliseconds it was minted at, so that name claims a creation instant of 2688-07-02 where the name actually minted decodes to 2026-09-13. A directory that lies about its own age to smuggle a reference through is worse than a sentence, and a sentence does the job: the workbench's standing text names the Andoneer board identifier `149f228d48c3` as the board this workbench continues, which is readable, true, and where a reader is already looking.

The **column** identifiers survive untouched: all fourteen directory names are unchanged by the migration, verified by listing them after each run. That is the identity that matters mechanically, because the hold table, the item `column` field and every gate reference are keyed on it.

Two consequences. No path naming `149f228d48c3` as a directory may outlive step 1, on the card or in any criterion; both address the workbench by letting discovery find it from the repository root. And the new identifier is minted at migration time and cannot be predicted, so it is recorded on the card when it is known.

### Which columns hold, and in which direction

The rule this table applies is dinah-484's own, recorded forward on that card because it could not edit this text: **an item names the column that settles it and is held there on the way out; an item that must already be settled before a station is reached names that station and is held there on the way in.**

Three rows are the operator's own rulings and are not reopened. Merge holds on the way in, on his ruling of 2026-09-01, recorded word for word in the Andoneer workbench's standing text: "An acceptance criterion gates on Merge. Test is the stage that verifies criteria, so gating one on Test requires it to be verified before the column that verifies it. The operator found this on 2026-09-01 on dinah-361, whose criteria could only be proven by dispatching a release: the card was blocked out of Test for failing to have already done Test's work. Gating on Merge keeps what the rule was for, since the card still cannot leave Test with a criterion unverified." Acceptance stays owned outright and holds neither way, on the same standing ruling. And **his two review stations let a clean card through, on his ruling of 2026-09-14 answering OQ-2**: ownership comes off both, the exit hold goes on at both.

| # | Column | Slug | Id | Hold | Operator-owned | Why |
|---|--------|------|----|------|----------------|-----|
| 0 | Intake | `intake` | `5ea2db0272fc` | off | no | Nothing is settled here and nothing depends on an item being settled before a card arrives. |
| 1 | Triage | `triage` | `a2eb2436b77d` | off | no | Triage files items and routes; every item it files is settled downstream. |
| 2 | Design Queue | `design-queue` | `2f6c18c9f5d0` | off | no | A queue is a place to wait, not a place where anything is answered. |
| 3 | Spec | `spec` | `ca3badf49985` | out | no | The decisions a spec author takes are settled here, so a card does not leave Spec carrying one still pending. |
| 4 | Agent Design Review | `agent-design-review` | `0d86ad99cdbc` | out | no | The reviewer settles or reassigns the items arriving from Spec. |
| 5 | Operator Design Review | `operator-design-review` | `5729d4578008` | out | **no, changed from yes** | Operator ruling of 2026-09-14. A card carrying a pending question for him stops here; a card carrying none runs past. Pass-through becomes the board's own behaviour and stopping becomes a filed item. |
| 6 | Build Queue | `build-queue` | `0789fd2dbefd` | off | no | A queue. |
| 7 | Implement | `implement` | `4fda9c9ca779` | out | no | The decisions the implementer takes in the course of the work are settled here. |
| 8 | Agent Code Review | `agent-code-review` | `4b38abe7ebd5` | out | no | **Every diff is read here**, by the agent that works this station, on every card without exception. That is unchanged by the ruling below and is the reason a clean card passing the operator's station is not a diff going unread. The reviewer's own items are settled here, hence the exit hold. |
| 9 | Operator Code Review | `operator-code-review` | `ee29487fad76` | out | **no, changed from yes** | Operator ruling of 2026-09-14, the same shape as row 5. A card carrying a pending question for him stops here; a clean card runs past to Merge. |
| 10 | Test | `test` | `c9428b3bc921` | out | no | A decision a tester takes is settled here. Acceptance criteria are not held here: they name Merge and are held at its entry. |
| 11 | Merge | `merge` | `6c5b9d6f4414` | **on** | no | The operator's ruling of 2026-09-01, quoted above. A criterion names Merge and the card cannot enter while it is unverified. |
| 12 | Acceptance | `acceptance` | `b69abf918c42` | off | **yes, unchanged** | His standing ruling. Ownership stops every card for him here unconditionally, which is what he asked for at the station where his judgement is the last word. An exit hold on top would additionally refuse his own move out while an item named Acceptance sat pending, which is a second stop he did not ask for. |
| 13 | Done | `done` | `aa6cd1c6ae5f` | off | no | Terminal. No card leaves. |

Six columns hold neither way, seven hold on the way out, one holds on the way in. One is owned outright. That is fourteen, which is the whole flow.

**What the operator gave up in choosing this, stated without softening.** He does not see a clean card's diff at his own station. Between Implement and the trunk there is then nothing but an agent choosing to file an item. He was told that in those terms when the question was put to him and he chose it. It is not reopened here and nothing below argues against it.

**Two things remain true and are the reason this is not simply the weaker shape.** Row 8 is the first: Agent Code Review reads every diff, on every card, and the ruling did not touch that station. The second is the hold's own behaviour, below.

**The exit hold refuses the operator himself, which is what makes his stations stop at all now that they are not owned.** The two checks live in different functions and ask different questions. `canRoute` (`internal/verb/mutate.go:350`) refuses a departure when `departure.OperatorOwned && !operator`, so it never reaches him. `canLand` (`internal/verb/mutate.go:445`) applies the exit hold reading only the card's items and never the actor, so it refuses everyone including him. That was reproduced on a copy of the real workbench, on a column then carrying both settings, and the actor-blindness is the part that carries over to an unowned station:

```
$ DINAH_ACTOR=agent dinah move dinah-1 test
not-operator this action is the operator's, and you are agent
$ DINAH_ACTOR=paul dinah move dinah-1 test
dinah.unresolved-item-exit this card carries the item 826ba0f596ba, which is not
resolved, and this column holds until it is; answer it, then resolve, verify or
fail that item

$ DINAH_ACTOR=paul dinah set operator-code-review hold off
$ DINAH_ACTOR=paul dinah move dinah-1 test
dinah-1  composition probe  [Test / ready]
```

The last two lines are what the hold prevents: without it, his own unanswered question rode forward into Test with the card. Under the configuration above that refusal is what stops a card at either of his stations, and AC-9 provokes it at both.

**Recorded history: the split this card recommended and he did not take.** The spec put three shapes to him. A, which he chose, is the table above. B was ownership on at both stations, stopping every card at each. C was the split, design review as A and code review as B, and it was this card's recommendation, on the argument that the two stations are not the same kind of thing: the design station exists to take the questions the design half raised, so a card raising none genuinely needs nothing from him, while the code station exists so that he reads the diff, which is not contingent on anybody having filed anything. It was a real recommendation and it was considered. OQ-2's note carries the three shapes and their costs in full, as the record of what he was choosing between. Nothing in this section is an argument to revisit it.

**The exit hold refuses a push-back.** Rows 3, 4, 7, 8, 9 and 10 push work backwards, and while an item naming one of them is pending, a move out of it in either direction is refused by name (`internal/verb/mutate.go:445`, whose comment says "an unresolved item is exactly as good a reason to keep a card at the column that raised it on a push-back as it is on an advance"). A reviewer who wants to send a card back settles the review's own items first, or reassigns one with `dinah set <item-ref> column <other-column>`, which moves the hold rather than answering it.

**Writing a hold is the operator's act.** A column's fields carry `AuthorityOperator` (`internal/bench/fields.go`, `writeAuthority`), so `dinah set <column> hold <value>` is refused to every other owner. Eight lines, one per column that holds:

```
dinah set spec                   hold out
dinah set agent-design-review    hold out
dinah set operator-design-review hold out
dinah set implement              hold out
dinah set agent-code-review      hold out
dinah set operator-code-review   hold out
dinah set test                   hold out
dinah set merge                  hold on
```

The six columns that hold neither way need no command, because off is the value already stored.

**`operator_owned` is not settable and has no command at all.** It appears in no kind's field table in `internal/bench/fields.go`, and `internal/bench/bench.go:1862` only reads it. Two hand edits are needed, each changing `operator_owned: true` to `operator_owned: false` on line 5 of a column anchor: the Operator Design Review anchor, `columns/5729d4578008/column.md`, and the Operator Code Review anchor, `columns/ee29487fad76/column.md`. The Acceptance anchor, `columns/b69abf918c42/column.md`, keeps `true` and is not touched. Those paths are relative to the workbench directory, whose name changes at step 1, so find the workbench rather than typing a path from here.

**Recorded forward rather than filed.** `ColumnView` (`internal/verb/read.go:17` to `:71`) publishes `operator_owned`, `awaiting_outside`, `takes_work_up`, `pull_destination`, `capacity`, `reject_to` and `count`, and does not publish the hold. So `dinah columns` and `dinah status` cannot tell a reader which way a column holds, and `dinah export` is the only whole-workbench read that carries it (`exportColumn`, `internal/bench/interchange.go:100` to `:107`, writes `gate_items` as `true`, `"out"` or `"both"` and omits it for off). A member on `ColumnView` is the obvious repair and belongs to whoever next touches that file.

### The instructions

All fourteen bodies stop being Andoneer's. They are not edited into shape; they are rewritten against the Dinah workbench's actual behaviour. What governs the rewrite is an enumeration, not a list of words somebody noticed, and the enumeration runs in two shapes because Andoneer's vocabulary comes in two.

**Shape one, `snake_case`, which is where the command names are.** Every such token was extracted mechanically, reading each file whole, with `perl -0777 -ne 'while(/\b([a-z][a-z0-9]*(?:_[a-z0-9]+)+)\b/g){print "$1\n"}'`. That yields **35 distinct tokens**, of which **18 are Andoneer command names**: `add_card_to_workstream`, `add_comment`, `block_card`, `claim_card`, `evaluate_gates`, `get_column`, `get_project_document`, `get_workbench_document`, `link_cards`, `list_cards_brief`, `list_lanes`, `list_workstreams`, `move_card`, `release_card`, `report_spend`, `set_card_lane`, `update_card`, `update_checklist_item`. Agent Design Review enumerated independently and reached the same eighteen. Four of them tell an agent to run something with no Dinah counterpart and never will have one: `evaluate_gates`, `list_lanes`, `set_card_lane` and `report_spend`. `evaluate_gates` is the worst, because Build Queue's body instructs an agent to run it and read a status before claiming, and under a hold there is nothing to ask in advance: the agent attempts the move and reads the refusal by name.

The remaining seventeen are field names and values. The ones that stay are admitted from somewhere a check can derive rather than from anybody's list, and getting that derivation right took a correction: **an allow-set built only from the field-name constants and the command table rejects `reject_to`, `loop_limit`, `awaiting_outside` and `operator_owned`**, which are Dinah's own column configuration keys, read through bare `fm.Value("...")` literals in `internal/bench` and declared in no field table. One of those four is a key this very section instructs the author to declare. So the derivation reads those literals too, and with that source the admitted vocabulary is 108 terms and a station description written in Dinah's own words rejects nothing. The rest of the seventeen are Andoneer's field vocabulary and go with the commands that used them: `at_limit`, `at_limit_reached`, `branch_name`, `branch_pattern`, `card_id`, `column_id`, `expected_tier`, `gate_column`, `item_id`, `operator_decision`, `pull_queue_no_downstream`, `spawned_from`, `tier_class`.

**Shape two, hyphenated, which is where the subagents, the directive values and the card references are.** Shape one cannot see any of them, so a body still telling an implementer to label every commit `<human-id>: <summary>` and to hand the card to `card-triage` passes it clean. Three widths were measured on the real bodies. Every hyphenated token anywhere gives **102 distinct**, firing on `best-effort`, `byte-identical`, `board-wide`, `cross-package` and `customer-facing`, which is a guard somebody disables in week one. Tokens forming a whole backticked span give **4**, which is readable but misses `human-id` entirely, and `human-id` is the commit-labelling case. Tokens appearing anywhere inside a backticked span give **15**, catch `human-id` (four occurrences), `card-triage`, `card-implement`, `card-test`, `card-slug` and `worktree-push`, and cost a reader a minute. That is the width the rewrite is held to. It is an adjudication list rather than an allow-list and it is weaker on purpose: it cannot tell a new Andoneer coinage from an ordinary hyphenated word, so it prints and a person decides. `dinah-gh` appears in it and legitimately stays, being a real machine account.

Four further rules govern the rewrite.

1. **No lane vocabulary.** The format's flow is linear and there is no `Lane` type, field or behaviour in the repository. Ten of the fourteen bodies route by lane. Every such passage states a single route or is cut.
2. **Dinah's verbs, spelled as Dinah spells them**, which the command table in `cmd/dinah/commands.go` declares and which the rewrite draws from rather than from memory.
3. **No directive the format cannot carry.** The format carries a column's title, slug, kind, tier, capacity, hold, reject-to target, loop limit, `operator_owned` and `awaiting_outside`, and its instructions body. It carries no subagent, no isolation mode, no self-test command list and no short description. Push-back targets, which extraction finding 5 records as dropped, do have a slot now: `reject_to` (`internal/bench/bench.go:202`, resolved at `:1937`, checked under `check.reject-target-unknown`), so the backward edges become data rather than prose.
4. **The gating guidance is corrected for the direction dinah-484 introduced.** The board's existing rule, that an item gates the column after the one that answers it, was a workaround for entry being the only direction and is now wrong. The replacement is the rule stated above the hold table.

The workbench anchor's own standing text is rewritten with them. It currently declares that this workbench is not a mirror of anything, that the hosted board stays the arbiter, that it carries no cards, and that it rehearses an extraction command that does not exist. All four become false at the cutover, and the last was already false; see below.

### The workstreams

The Andoneer board declares twenty-one: `addressing`, `dogfood`, `structured-judgements`, `vsix`, `token-cost`, `beta`, `beta-wave-1`, `beta-wave-2`, `beta-wave-3`, `beta-wave-4`, `beta-wave-5`, `how-work-moves`, `state-model`, `levels`, `integrity`, `terminal`, `handoff`, `conformance`, `authoring`, `model-gaps`, `spinout`. The list was produced by asking the Andoneer server for a workstream that does not exist and reading the valid set out of the refusal, which is the only whole-set read that surface offers. The Dinah workbench declares none.

The pass creates each with `dinah workstream new <title>` and attaches each carried card with `dinah join <card> <workstream>`.

### The cards

The Andoneer board carried 468 cards when this was read: Complete 227, Intake 198, Acceptance 36, Design Queue 3, Refused 2, Triage 1, Spec 1. **239 open**, 198 of them never triaged. Those figures move, so the cutover recomputes them at its own instant rather than quoting these.

**There is no import verb**, so a card crosses by being created and moved. The command set is `add claim move pull release block unblock raise comment attach file cite resolve verify fail reopen link unlink join leave archive restore delete rename status columns ls next query search tree contents attachments show log changes instructions guide init export extract reshape path edit get set config check whoami workbench workstream column workbenches version mcp help`. `extract`'s own doc comment says it "carries the flow and the instructions, and none of the cards."

**The workbench's claim that the extraction command does not exist is false.** `dinah extract` is registered in `cmd/dinah/commands.go`, run by `runExtract` at line 1102, backed by `Library.Extract` at `internal/verb/read.go:1656`, and documented in the quick start with a worked transcript. A line-oriented search for that claim returns nothing, because it wraps across a line break between "extraction" and "command"; the whole-file sweep that found it matched in exactly one file of the set.

#### How the numbers are preserved, proven end to end rather than reasoned about

**An early draft of this card had this backwards.** `dinah add` takes no number argument and `NextNumber` (`internal/bench/bench.go:2157`) simply answers `Highest + 1`, so seeding the registry in advance does not bring card 449 back as 449; it mints a card above the seeded high-water mark. The working route is the opposite order:

1. Both migrations, in the order fixed above. The registry does not exist yet, so `Highest` is 0.
2. Create the cards. They take 1, 2, 3 in creation order, and **no card anchor carries a number key at all** (`grep -c "^number:" cards/*/card.md` returns 0 for every card), because at `RegistryFormat` the number "lives in card-numbers.txt and nowhere else" (`internal/bench/card.go:453`). There is therefore no second copy to disagree with the rewrite.
3. Rewrite `card-numbers.txt` from whole lines, one `<number> <identifier>` per carried card, ascending by number, plus one `<number> -` tombstone line above the highest number the Andoneer board has ever issued so that no post-cutover filing can reissue a retired one.
4. Read the cards back.

The transcript of steps 3 and 4, run against a scratch copy after three cards were created as `dinah-1`, `dinah-2` and `dinah-3`:

```
$ printf '449 f6ee18c7d061\n484 33c72ea36e94\n488 e32bb351c8f6\n492 -\n' > card-numbers.txt
$ dinah show 449
dinah-449  carried card A  [Intake / ready]
$ dinah show dinah-484
dinah-484  carried card B  [Intake / ready]
$ dinah check
No structural defects found.
$ dinah add "card filed after the cutover"
dinah-493  card filed after the cutover  [Intake / ready]
```

The tombstone at 492 is what makes the last line read 493 rather than 489. The highest live Andoneer card number was 492 when this was written and it is a moving target, so the pass reads the true high-water mark at the cutover instant and tombstones above that, rather than above any number written here.

#### What crosses with a card

An early draft said title, body, severity, priority, column and workstreams, which throws away the only thing the holding machinery acts on. A card's checklist crosses too, and there is a verb for it.

`dinah file <card> <kind> <text> [--column <column>] [--owner <owner>]` files an item, where kind is `acceptance_criterion`, `open_question` or `decision`. The `--column` argument resolves a slug to the column identifier and stores that, so a criterion crosses as `--column merge` and lands storing `6c5b9d6f4414`. The `--owner` argument records who the item is meant for, and `operator` is the one value the tool enforces.

An item is born pending, so an item that was settled on Andoneer is filed and then closed with `dinah resolve|verify|fail <item-ref> "<note>"`, which carries the note. Two things the pass must know, both reproduced:

- **An item owned by the operator can only be closed by the operator.** `DINAH_ACTOR=agent dinah resolve dinah-449/questions/1 "..."` on an item carrying `owner: operator` answers `not-operator this action is the operator's, and you are agent`. So a resolved operator-owned question crosses only if the pass runs that one command as `paul`, or it arrives pending and holds its card. Say which, per item, rather than discovering it mid-run.
- **A criterion verifies without a citation on this workbench**, because the anchor declares no `evidence:` block and the obligation in `internal/verb/checklist.go:184` is conditioned on `l.Bench.EvidenceDeclared()`. If the instructions rewrite adds an evidence block, every carried verified criterion needs a citation and this changes.

#### The retired items, which Dinah has no state for

Andoneer can mark an item as no longer applying. Dinah declares four states, `ItemStates = []string{ItemPending, ItemResolved, ItemVerified, ItemFailed}` (`internal/bench/item.go:62`), and asking for a fifth is refused by name:

```
$ dinah set dinah-2/criteria/1 state obsolete
dinah.unknown-value Dinah does not accept obsolete in state. The values state
takes are: pending, resolved, verified, failed.
```

**dinah-472 is the card that closes this**, and its scope was widened on 2026-09-14 so that it genuinely does. It is titled "A checklist item's four states cannot say 'let this through anyway' and cannot say 'this no longer applies'", it carries both missing states as one question about what the state set lacks, it argues why a single word for both would repeat the conflation that made `failed` ambiguous, it records that this card carries four such items and that the prose workaround does not become permanent by being convenient, and it marks the second state as the one on the cutover's path. It sits in Intake at priority next, severity major, and its `relates_to` link to this card was verified present rather than assumed. An earlier draft of this section cited dinah-472 for something it did not then cover, which made the word "temporary" a promise with nothing behind it.

Every landing was tried on a scratch copy and each is worse than not carrying the item. Pending is what `dinah file` produces and there is no way to file an item already closed, so pending is what doing nothing gets you. **A pending decision or open question refuses every claim on its card even when it names no column at all**, reproduced as `unresolved-item this card carries the item 74c34624f87b, which is not resolved`, which pins the card with nothing on it explaining why. A pending acceptance criterion naming no column is genuinely inert, also reproduced: the claim succeeded and a move into Merge succeeded, because Merge's entry hold only sees items naming Merge. But it then reads as outstanding work nobody has verified, which is a false statement about the card. `failed` says the check ran and did not pass, which is a different lie, and `verified` is a plain one.

**So a retired item does not cross as a checklist item.** Its kind, its Andoneer local reference, its text, its note and its Andoneer state go into the carried card's body under the literal heading `## Retired checklist items (no Dinah state for these yet)`.

**The census that finds them again is anchored to a whole line, not a plain text search.** Run it as:

```
grep -rlE '^## Retired checklist items \(no Dinah state for these yet\)$' <workbench>/cards/*/card.md
```

A plain `grep -rl "Retired checklist items"` is wrong and this card is the proof: its own spec discusses the heading at length, so the plain form would count the card that introduced the rule as one carrying parked items. Demonstrated on two throwaway files, one holding the heading and one only describing it, the plain form returned both and the anchored form returned one. The heading is a fixed literal precisely so the anchored form is a complete and exact census at any later date, and that is what stops this being a promise: whoever works dinah-472 finds every parked item by running one command rather than by remembering that the parking happened. AC-16 requires the heading and the anchored census, and fails an unanchored one. Recorded forward for whoever specs dinah-472, since this card may not write another card's contract: that card should carry a criterion requiring the census to return zero once the state exists.

#### What genuinely does not cross

- **Card journals.** A Dinah journal is written by the verbs as they run and no verb accepts a timestamp, so replaying 468 cards' transitions would stamp every one with the cutover's own clock and produce a journal that reads as true and is not. The Andoneer board is kept readable as the archive, and both boards' standing texts say so.
- **Comments.** `dinah comment` writes one, so they could cross, but each would carry the cutover's own timestamp and the pass's own actor rather than the original author's. They stay on the Andoneer side with the journals, for the same reason.
- **An item's own citations.** `dinah cite` writes one, so a citation can be re-filed, but a citation's target names an Andoneer artifact. Cards whose criteria were verified against a citation cross with the criterion verified and the note carrying the citation's text, and the structured citation is not re-filed.

**Links** cross. `dinah link <from> <to> <kind>` exists, and the pass uses Andoneer's four spellings verbatim (`blocks`, `relates_to`, `supersedes`, `parked_behind`) since the enum is open and carries no behaviour on either side.

### What "the operator works from it" means concretely

1. He reads the board with `dinah next`, `dinah ls` and `dinah show <card>` in `c:\Users\paul\source\repos\dinah`, and moves cards with `dinah move`. The Andoneer web board is not the thing he opens to find out where a card stands.
2. Agent sessions on this project reach the board through Dinah's own MCP server against this workbench, not through Andoneer's.
3. Cards are filed on the Dinah workbench. No card is created on the Andoneer board after the cutover instant.
4. The Andoneer board stops moving.

Points 3 and 4 are checkable by command over a window, and are checked that way below.

### The order

1. **The two migrations**, container then numbers, on the workbench as it stands with no cards in it. The workbench directory is renamed here and the new identifier is recorded on this card.
2. **Definition.** Rewrite the fourteen column bodies and the anchor's standing text against both enumerations above, including the sentence naming `149f228d48c3` as the Andoneer board this workbench continues. Declare `reject_to` on the columns that push back. Bring `profile:` to whatever the repository ships. Run `dinah check` and clear what it reports.
3. **The operator writes the holds**: the eight `dinah set` lines and the two `operator_owned` hand edits. Re-run `dinah check`.
4. **Rehearse against a copy.** `dinah extract` then `dinah init --from` in a directory outside the operator's profile, with `DINAH_HOME` pointed at a scratch directory. Provoke every refusal the table implies and every admission beside it, including the operator's own refusal at both of his review stations and a clean card passing both without stopping.
5. **The cutover instant.** The operator declares the Andoneer board frozen. The open cards are created with their checklists, joined to their workstreams, linked, and placed in their columns, with retired items landing in card bodies under the fixed heading; then `card-numbers.txt` is rewritten and the read-back is performed.
6. **Flip the arbiter, in writing, in both places.** The Dinah anchor stops saying the hosted board is the arbiter and starts saying it is. The Andoneer workbench's instructions say the board is closed, name the Dinah workbench as the arbiter, and say the Andoneer board is the archive of everything before the cutover.

Step 6 is what makes this irreversible enough to be real, and it is prose rather than a lock because nothing in either tool can enforce it. The two boards each carry a sentence naming the other's role, so a session reading either one is told which board it is standing on.

## Out of scope, restated from the description

Andoneer keeps running the operator's whole portfolio; this moves one project's development. Nothing here asks Dinah to grow a board UI. Dinah.Team is untouched, since it runs on Dinah's library rather than being a second implementation. Nothing the cutover asks of Dinah bakes this board's vocabulary into the tool: every column name, every item's placement and every link spelling above is workbench data, and the only code change this section names is a missing member on a view.

## Decisions recorded

Three resolved decisions carry the gap verdicts for citations, links and lanes. The gating verdict was filed as an operator-owned question and he answered it on 2026-09-09. The hold shape at his two review stations was filed as a second operator-owned question and he answered it on 2026-09-14, choosing that both let a clean card through; OQ-2's note carries his ruling and, beneath it, the three shapes and their costs as the record of what he was choosing between. The later decisions carry the cutover's own calls: what crosses, the migration order, the folder name, and the retired items.

The four acceptance criteria this card carried until 2026-09-13 all measured the quality of the ruling rather than the state of the move, and every one could have passed with the Dinah workbench holding zero cards. They are marked obsolete rather than deleted, with a note on each saying what replaced it, and they are themselves the four items the retired-item rule above will have to park.

## Branch

dinah-449-move-this-projects-own-development-onto-dinah

## Retired checklist items (no Dinah state for these yet)

These crossed from the Andoneer board in a state Dinah has no word for. dinah-472 is the card that adds it; until then they live here.

- **acceptance_criterion** (Andoneer AC-1, state `obsolete`)
  - Text: The card carries exactly three resolved decision items (dinah-438, dinah-436, dinah-209) and exactly one pending, operator-owned open_question item (dinah-450), each naming: the verdict or the alternatives, a reason distinct from "parity with Andoneer," and the layer the capability would live in if it were ever built. Check by reading the card's checklist: three resolved decisions plus one pending open question, none of the three decisions pending, each note naming a human-id and a layer, and the open question stating both build-it and don't-build-it alternatives rather than a single recommendation.
  - Note: Obsolete on 2026-09-13. It measured the card's own checklist bookkeeping (three resolved decisions plus one pending operator-owned question), which describes the quality of the ruling rather than the state of the move, and it could have passed with the Dinah workbench holding zero cards. It is also already false against the card: OQ-1 was resolved by the operator on 2026-09-09, so the criterion now requires a pending item that must not be pending. Kept as the record of what the design half owed. Replaced by the criteria filed on 2026-09-13, which describe the board actually running.
- **acceptance_criterion** (Andoneer AC-2, state `obsolete`)
  - Text: The spec states, for each of the three cards gated behind this one (dinah-450, dinah-436, dinah-209), a stated consequence of the ruling: proceed as scoped, reopen only against a future incident, or drop and stay filed for a future board. Check by reading the spec's "Consequences" content against those three human-ids; a verdict with no stated consequence for its own card fails this criterion even if the verdict itself is sound.
  - Note: Obsolete on 2026-09-13. The spec's Consequences section does state a consequence for each of dinah-450, dinah-436 and dinah-209, and that section stands, so this criterion has in substance been met. It is retired because it cannot fail once the section is written and because it says nothing about whether this project's development has moved, which is what the card tracks. Kept as the record of what the design half owed.
- **acceptance_criterion** (Andoneer AC-3, state `obsolete`)
  - Text: The spec addresses the GK-workbench alternative on its own terms rather than dismissing it: it states what the reshaped board actually requires building (after the four verdicts) and compares that lift against the GK workbench rather than against the original fifteen-column parity target. Weaker check, human-verified at Agent Design Review: read the "alternative" section and confirm it draws its conclusion from the ruling's own reduced scope rather than restating the card's framing.
  - Note: Obsolete on 2026-09-13. The spec's GK-workbench section does weigh the alternative against the ruling's own reduced scope and does say plainly that the lift grew when gating was reopened, so this criterion has in substance been met. It is retired because it was self-admittedly weak ("weaker check, human-verified") and because it measures the ruling rather than the move. Kept as the record of what the design half owed.
- **acceptance_criterion** (Andoneer AC-4, state `obsolete`)
  - Text: Every factual claim about current Dinah behavior in the spec's "Verification method" section cites a file and, where the claim is an absence (no writer, no lane concept), the exact search command run rather than a description of having searched. Check by reading that section: each of the four gap paragraphs names at least one file path and, for gap 3 and gap 4 (both absence claims), the literal grep command used.
  - Note: Obsolete on 2026-09-13, and it is the one of the four worth regretting. Its discipline was right: an absence claim cites the search command rather than a description of having searched. What it could not do was fail, since it checks a section that was written to satisfy it, and it says nothing about the move. The discipline is not dropped; it is folded into the replacement criteria, each of which names the command that produces its answer and asserts how big the set it swept was. The cutover section added on 2026-09-13 also applies it to itself, including the widened multiline sweep that found the workbench's stale claim about the extraction command, which no line-oriented search could match because the claim wraps across a line break.
