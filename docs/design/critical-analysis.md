# A critical analysis of Dinah, with proposed improvements

This document assesses Dinah as a tool for managing tasks and for working with agents on those tasks, and it proposes a prioritised set of improvements. It was written on 2026-09-20 from a reading of the profile, the format document, the token-cost measurements, the author-editor pipeline document, the live workbench's definition and column bodies, and a pass over every card journal on the live workbench. The figures it quotes are a snapshot taken that day and are gathered in the appendix so the argument can be re-read against later numbers.

The document is an assessment and is dated by nature. Where it states a rule, the rule is meant to outlive the snapshot; where it states a figure, the figure is not.

## 1. Summary

Dinah works, and its own journal is the evidence. In the six days after development moved onto the in-repo workbench, fifteen cards travelled from Spec to Acceptance in between 1.3 and 35 hours each, with a median near nine, and the two agent review stations sent cards back 82 times across 24 cards. Each push-back is a defect caught before the operator saw it, and that is the tool paying for itself.

The costs are equally visible. The intake column holds 208 cards. Agents filed 23 new cards in the same period that produced no Done cards. The workbench's standing text has grown into a list of scars, each rule written after an agent broke something, and several of those rules guard against hazards the tool itself could remove.

The assessment, in one sentence, is that Dinah is a sound coordination substrate whose own board has a flow problem, and the flow problem comes mostly from the route's shape rather than from the tool.

## 2. What is novel

Most tools that offer a kanban board to agents are a task list with an API. Dinah differs in four ways that I have not seen combined elsewhere.

**A position serves its instructions and its legal moves at the moment an owner arrives there.** The text comes in three layers (user, workbench, column), served most general first and never copied between layers, and a layer already sent on a connection is withheld and named rather than sent again. This is the HATEOAS principle applied to work. A fresh-context subagent dispatched with nothing but a card reference can derive everything it needs from the response to its claim, and that property is what makes stage isolation cheap enough to use routinely.

**Structured items are the gate mechanism, and one mechanism covers three kinds of judgement.** An item names one column, and the column declares whether it holds a card on entry, on exit, or both. Acceptance criteria, decisions, and open questions all ride the same rule, so no column needs a per-kind rule of its own. The owner stamp on an item is how a question routes to the operator's station without prose, and the live workbench carries 1,726 items, 84 of them pending, so the mechanism is under load rather than on paper.

**Every verb answers with one of four outcomes, refusals carry a closed vocabulary, and the order of evaluation is fixed.** An unattended caller can branch on the answer. The fourth outcome, `stale`, keyed on a basis revision the caller names, catches the commonest way an automated caller does the wrong thing, which is deciding on a card that has since moved. Most tools of this kind return prose errors that only a person can read.

**The boundary table holds the contract to a discipline.** Sixty-one concepts are each ruled in with the statements that bind them or ruled out with the condition that reopens them, and a checker fails when a statement has no row. This is why the core stayed domain-neutral in fact: the wedding walk-through in the profile is a real test, and the three non-software workbenches on this machine (a concept pipeline, an incident-resolution route written for a colleague, and the document-editing pipeline) needed no change to Dinah.

Two further points deserve a mention. The tier gate lets a card declare the rung of worker it needs and measures a claim against a provider and model the harness declared in the environment, and the format document is candid that this stops only an honest claimant. And the scope line, "Dinah Is Not A Harness", is the defining bet of the design; three harnesses already drive the live workbench, which no harness-bound tool could show.

## 3. Strengths

The files-and-git substrate is the right call for a single operator. A workbench is diffable, greppable, and copyable as a template through `init --from` and `extract`, and nothing has to be run or hosted. The two-planes argument in `format.md`, under which content merges the way git merges and coordination locks the way a checkout locks, is sound, and it explains why the command-line tool can be single-seat without apology and why the many-writer case belongs to a hosted arbiter.

The measurement culture is rare in a project this size and it is load-bearing. `token-cost.md` measured before advising and found the workstream's own ordering wrong: repeat serves of the instruction chain were the largest cost, and filesystem-first reads the smallest. The suite runs to more than two lines of test per line of production Go, tests are armed by breaking the behaviour they guard, and the conformance report names every normative statement or records why it is out of reach.

The review loop catches things. The distribution of push-backs is heavy-tailed, with five cards sent back once and four sent back eight or more times, and the project's own memory records specific saves, including a guard that survived three careful readings and fell to one reviewer who wrote the code it was meant to stop. That is the fresh-context principle working as designed.

The instruction chain is edited as a document rather than as configuration. A column body on the live workbench reads as a contract for the stage, with its exits and its holds spelled out, and a person can edit it with any editor and see the change served on the next claim.

## 4. Weaknesses

### 4.1 One route, fourteen stations

Every card on the live workbench walks all fourteen columns, and the workbench body says so: a card that needs a shorter road is a card whose route the workbench has not been asked to grow. A one-line fix therefore pays for Spec, two design reviews, two code reviews, Test, and Merge. Combined with a filing rate above the completion rate, this produces an intake of 208. The ruling that findings are fixed on the card that found them rather than filed treats the symptom. Lanes are ruled out of the core until a second tool needs them, which is the right rule for the contract and the wrong constraint for the operator's own board.

### 4.2 Rules live in prose, and prose does not enforce itself

The workbench body runs to about 9KB and a column body to about 13KB, and the section of the workbench body headed "Disciplines this project has paid for" is a list of failures rewritten as instructions. The evidence that prose leaks is in the repository. Five stale worktrees sit under `.claude/worktrees/`, the one location every instruction forbids, and the station bindings record that an agent's identity was once lost and its findings attributed to the operator. Some of these rules belong in the tool. A call that names no actor anywhere is already refused `no-owner`, and the misattribution happens one rung lower. The operator's user configuration names him as the actor, that file is shared by every process on the machine, and a harness whose shell drops `DINAH_ACTOR` between calls therefore resolves to the operator and is answered with his authority. `format.md` gives this exact reason for keeping the provider and the model off the configuration rung, and the actor ladder is the one place the reason was not applied.

### 4.3 Discovery climbs to the drive root

A workbench is found by climbing from the current directory to the root of the drive. A scratch tree anywhere under the operator's profile reaches the live workbenches in his home, `DINAH_HOME` does not bound the walk, and the safety document records the consequences: a language setting changed while he watched, and a workbench written into an unrelated repository. This is defended entirely by instruction, and by the project's own debt test a hazard in the design that has to be patched in every agent's prose is debt.

### 4.4 Token cost is structural

A bare `show` on a worked card once returned 570KB where the members a station needs came to 5KB, and the remedy is a `--fields` discipline every agent must remember. The MCP head serves 36 tools, and the definition block costs about 12,800 tokens on every round trip. Column bodies are read in full by every fresh context, which is the price of the fresh-context benefit, and it means long column prose is expensive at exactly the place the design encourages writing it. The mitigations that shipped, withholding already-served layers and narrowing reads, are good, and the shape still rewards short column bodies while the live workbench's are not short.

### 4.5 The contract is unproven as a contract

The profile exists so that a second implementation can be verified against it, and none exists. `format.md` records the blind spot: the code and the suite are written from one reading of the document, so they can share a misreading nothing catches. Until a second reader appears, the profile is a disciplined design document rather than a standard, and the cost of maintaining it as a standard (a changelog entry, a boundary row, and a statement-index update for every change) is paid now for a benefit that remains speculative.

### 4.6 Nothing watches

A card that lands in a column stays there until a session reads the workbench and carries it on. This follows from the scope line and it is the correct place to draw the boundary, and it also caps the tool's effectiveness at the harness driving it. `changes --since` gives a polling cursor, which is enough for a skill loop and not enough for an unattended board without somebody writing the loop and its timer.

## 5. The pipeline pattern

The pattern the operator describes, in which a workbench accepts a card from an agent and the agent drives it through the columns as the stages of a one-shot workflow ending in Done or Returned, has already been built once. `author-editor-pipeline.md` describes a seven-column workbench with two terminal columns, scripts for the mechanical stages, one skill that turns a conversational request into a card, and a second skill that turns the handle by spawning a fresh subagent per work column. The first card through it ran from Intake to Done in four minutes across four contexts. The format supports the shape directly, because the terminal region admits several `done` columns, `reject_to` may name one, and `loop_limit` bounds a loop between two stations.

That document also states the test a candidate pipeline has to pass: one stage must be starved of context, and the artefact chain must make the starvation auditable. Where no stage benefits from being blind, a plain skill is the right tool and a pipeline is ceremony. A one-shot workflow driven by a single agent through every column in one context fails that test. It keeps the token cost of instruction serving and the move ceremony, and it gives up the one benefit a skill cannot provide, which is that the reviewer never sees the author's reasoning.

The pattern earns its cost when at least one of these holds:

1. A stage must judge without the context that produced its input, as in review, verification, back-translation, or blind grading.
2. The run must survive a context reset or a crash and resume from the card, which a skill's transcript cannot do.
3. Several cards run in parallel and their claims have to exclude each other.
4. The artefact chain has to be auditable afterwards, and the journal is the audit.

Three refinements follow from the first build. The driving session should move cards and read results and do nothing else, with every work column worked by its own subagent, or the pipeline collapses into the skill it was meant to replace. A mechanical check should produce items rather than prose, so that a scan emitting a worklist files an item against the next column and the hold belongs to the tool rather than to the script. And the Returned exit should file onto the workbench that owns the work and never reopen the pipeline card, which is the coupling that lets pipelines chain through task workbenches without any pipeline holding state.

## 6. Non-software work

The core is domain-neutral in practice as well as in claim. The barrier for an operator who is not the author of Dinah is authorship rather than the model. Each column body is a contract that a fresh context reads in full, and writing thirteen of them to the standard the live workbench's columns reach is a week of work for somebody who writes well. The absence of any surface for a person beyond the command line and the editor extension is the second barrier, and it is the one the hosted product exists to remove.

## 7. Proposed improvements

The proposals are ordered by the ratio of what they remove to what they cost. Each carries a short design and an implementation sketch, and each is sized as small (one card, one package), medium (one card touching several packages), or large (a workstream). Where a proposal touches the profile, the sketch says so, because a profile change carries a changelog entry, a boundary row, and a statement-index update with it.

| # | Proposal | Priority | Size | Removes |
| --- | --- | --- | --- | --- |
| 1 | Routes through one flow | 1 | medium | the fourteen-station tax on small cards |
| 2 | Bound workbench discovery | 1 | small | the whole class of scratch-tree-reaches-live-data incidents |
| 3 | Keep the configured actor from answering for a harness | 1 | small | silent promotion of an agent to the operator |
| 4 | Narrow the default `show` | 2 | small | the 570KB read a forgotten flag produces |
| 5 | Column reference material served by listing | 2 | medium | column bodies growing past what every context should read |
| 6 | Tool-surface profiles on the MCP head | 2 | small | roughly half the per-round tool-definition cost for a station agent |
| 7 | A blocking `changes --wait` | 3 | small | the polling timer every driving loop writes |
| 8 | A pipeline template and guide | 3 | small | the setup cost of the pattern in section 5 |
| 9 | An independent reader of the interchange form | 4 | medium | the shared-misreading blind spot |

### 7.1 Routes through one flow (priority 1, medium)

**Design.** The workbench definition gains a `routes:` map. Each entry names a route and carries an ordered list of column identifiers that is a subsequence of `columns:`. A card carries a scalar `route` key naming one entry; a card carrying none is on the default route, which is the full `columns:` list. The forward move from a column, the destination a pull lands in, the `next` a column offers, and the legal moves a claim or move serves all read the card's route in place of the whole list. A regressive move for `loop_limit` is derived against the route rather than against `columns:`. A route names only columns the workbench declares, must begin at the intake column and end in the terminal region, and may omit any column between. `dinah check` reports a card whose `route` names no declared route and a route that is not a subsequence.

The design keeps one notion of position. The card's column is still where it stands, and the route only changes which column the forward move reaches, so no verb gains a second position and nothing about claims or holds changes. A card can be moved onto or off a route with `dinah set <card> route <name>`, which is an ordinary field write.

**Profile.** The boundary table rules lanes out until a second tool needs them, and this proposal does not promote them. Dinah carries `routes` in the interchange form as a member the profile does not list, on the path `reject_to` and `loop_limit` already take, so a second tool preserves it and no profile revision moves.

**Implementation.** The workbench anchor parser in `internal/bench` gains the map and its validation. The forward-move computation, the pull destination, and `next` in `internal/verb` take the card's route as an argument. `internal/contract` carries the member through export and import. `dinah check` gains two findings. The live workbench then declares a second route for small work that skips Spec, both design reviews, and one code review, and triage stamps it.

### 7.2 Bound workbench discovery (priority 1, small)

**Design.** The walk from the current directory upward stops at the nearest ancestor that is a git repository root, when the current directory is inside one. A workbench above that root is reached only by `--workbench` or by running from outside the repository. Outside any repository the walk continues to the drive root as it does today, so a personal workbench in the home directory is still found from a plain directory under it. `dinah init` refuses a directory it did not create unless the directory is empty or the caller passes `--here`.

The rule is documented behaviour of git (a repository root is the directory holding `.git`) and of the filesystem, so nothing here rests on anything undocumented.

**Implementation.** The discovery function in `internal/bench` gains the stop condition and a test for each of the three cases: inside a repository with a workbench above it, inside a repository with a workbench inside it, and outside any repository. The `init` guard is one condition in `cmd/dinah`. The safety document and the workbench body then lose the paragraphs that exist only because the walk was unbounded.

### 7.3 Keep the configured actor from answering for a harness (priority 1, small)

**Design.** The configuration rung of the actor ladder answers only a call that declares no harness. A process that sets `DINAH_HARNESS`, or names a harness over MCP, and names no actor is refused `no-owner`. The operator at his keyboard declares no harness, so his calls are attributed as they are today, and an agent that loses its actor is refused instead of promoted. The limit is stated rather than hidden: a process that loses every declaration at once still resolves to the configured actor, so the change narrows the hole without closing it. Removing the rung altogether was considered and moves the hazard to the operator's shell environment, which agents have inherited before.

An earlier draft of this section proposed adding the `no-owner` refusal itself. Tracing the code for dinah-540 showed that the refusal has existed since dinah-137, and the same trace found three smaller defects that dinah-540 now carries: three help pages omit `no-owner` from their check-order tables, one path can append a journal event with an empty actor before the owner check fires, and nothing sweeps the verb registry for a verb lacking the check. Whether the ladder changes is the operator's ruling and is filed on that card.

**Implementation.** The actor resolution in `cmd/dinah` and the MCP head's resolution skip the configuration rung when a harness is declared. One test per surface pins the refusal beside the accepting case of a call with no harness.

### 7.4 Narrow the default `show` (priority 2, small)

**Design.** `dinah show <card>` with no `--fields` serves the card, the body, the links, the attachment listing, and an index of comments and checklist items, which is the shape the workbench body currently asks every agent to type. `--all` serves everything. The MCP `show` tool takes the same default. The project is unreleased and the operator has ruled that a format or default changes rather than carrying a shim, so no compatibility flag is kept.

**Implementation.** The default field set in `internal/verb` changes, `--all` is added in `cmd/dinah` and the MCP schema, and the quick start's transcript for `show` is regenerated.

### 7.5 Column reference material served by listing (priority 2, medium)

**Design.** A column gains attachments on the same terms a card has them, with a description, a stable number, and a payload. A column body then carries what every arriving context must read, and material read only when a situation arises (the merge procedure for a pull request with no checks, the list of fixtures keyed on source lines) becomes an attachment listed by description in the served instructions and read by path on demand. `dinah check` reports a column body above a declared size, with the size set in the workbench definition so a workbench that wants long bodies may have them.

**Implementation.** The entity model in `internal/bench` already treats attachments generically; the change is admitting a column as a parent and adding the listing to the served instructions in `internal/verb` and `internal/mcp`. The live workbench's column bodies are then split, which is an editing task rather than a code task and belongs on its own card.

### 7.6 Tool-surface profiles on the MCP head (priority 2, small)

**Design.** `dinah mcp --tools <profile>` serves a named subset of the 36 tools. A `station` profile carries what an agent working one card needs, which is the claim, move, release, block, comment, attach, item verbs, and the narrowed reads. An `operator` profile adds the workbench and column verbs. The default stays `all`. The item verbs `verify`, `fail`, `resolve`, and `reopen` are additionally offered as one `settle` tool taking the state as an argument, which removes three definitions from the block without removing a verb from the command line.

**Implementation.** The tool registry in `internal/mcp` gains a profile filter. The harness profiles under `.devin/agents/` pass the flag. The measurement harness in `scripts/` is re-run and the figure recorded in `token-cost.md`.

### 7.7 A blocking `changes --wait` (priority 3, small)

**Design.** `dinah changes --since <cursor> --wait [--timeout <duration>]` returns when the cursor advances or the timeout lapses, and its output is the output `changes` gives today. It reports and never dispatches, so the boundary in the scope line holds. A driving loop becomes a loop with no sleep in it.

**Implementation.** The command polls the checkpoint digest at a short interval inside the process, which uses only documented filesystem behaviour and needs no notification API. It lands in `internal/verb` beside `changes` with a test that advances the cursor from a second goroutine.

### 7.8 A pipeline template and guide (priority 3, small)

**Design.** The binary ships a template workbench with an intake column, a configurable run of work columns, and two terminal columns named Done and Returned, its workbench body carrying the rules from section 5, and a guide under `dinah guide pipeline` that states the test a candidate pipeline must pass and the three refinements. `dinah init --from pipeline` instantiates it.

**Implementation.** The template is files under `internal/guide` or beside the other embedded guides, and the guide is one Markdown file. No verb changes.

### 7.9 An independent reader of the interchange form (priority 4, medium)

**Design.** A reader of the interchange form is written from the profile alone, by an agent given the profile and the interchange section of the format document and nothing of Dinah's Go code or tests. It validates a workbench export against every normative statement about the form and reports each statement it checked. The conformance report then records for each interchange statement whether the independent reader agrees with the suite. This is the one closing of the blind spot that `format.md` names, and it is itself a pipeline of the kind section 5 describes, because the writer of the reader has to be starved of the code.

**Implementation.** A small script in a language other than Go, under `scripts/`, run by CI against the export of the compatibility fixtures. Disagreement fails the build and is adjudicated by a person, on the rule `format.md` already states for a red conformance run.

### 7.10 Proposals considered and not made

An arrival hook on a column, running a command when a card lands, would make Dinah a harness and is not proposed. A dependency link that holds one card behind another is ruled out of the core with a reopen condition and is not needed on the live workbench today. A structured handoff carried on the move was considered and rejected because the comment-before-move discipline already produces a record the next station reads by one reference, and a second slot for the same text would be the copying between layers the design forbids.

## Appendix: the snapshot of 2026-09-20

These figures were computed from the live workbench's journals and files on the date above, and they date the moment rather than state a rule.

- Cards: 282 in the live set, of which 208 in Intake, 38 in Done, 19 in Acceptance.
- Journal events: 7,469 across all cards, of which 3,164 after the cutover of 2026-09-15.
- Push-backs: 82 regressive moves across 24 cards, 51 from Agent Design Review and 31 from Agent Code Review; the per-card count runs from one to ten.
- Spec to Acceptance, post-cutover cards: 1.3, 2.9, 3.0, 6.8, 7.3, 7.6, 8.3, 8.7, 10.7, 14.5, 15.7, 19.7, 23.7, 24.7, and 35.0 hours.
- Cards filed after the cutover: 24, of which 23 by an agent.
- Cards moved to Done after the cutover: none.
- Checklist items: 1,726, of which 877 acceptance criteria, 728 decisions, and 121 open questions; 84 pending; 80 stamped for the operator.
- Blocks over the workbench's life: four, all for an operator ruling. Overrides: none.
- Actors on the journal: `claude` 3,999 events, `paul` 3,421, `devin` 49, `devin-review` 17.
- Production Go, excluding tests and testdata: about 55,000 lines; test Go: about 121,000 lines.
- Column bodies on the live workbench: about 13KB each; workbench body: about 9KB.
- Stale worktrees under the forbidden `.claude/worktrees/`: five.
