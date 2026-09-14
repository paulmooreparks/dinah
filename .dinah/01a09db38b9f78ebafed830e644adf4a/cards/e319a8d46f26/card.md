---
title: The extension can select several rows at once, and archive a card or a selection of them
column: b69abf918c42
state: ready
severity: major
priority: now
tier: frontier
workstreams:
  - 58f3e3eb621a
links:
  - kind: relates_to
    to: f3e53f155840
---
Asked for by the operator on 2026-09-13. He needs to archive cards from the extension, and to archive several at once, and named multi-select as the thing missing underneath both.

## What exists already, established from the code

The CLI needs nothing. `dinah archive <ref>` already archives a workstream, a column, a card, or anything below a card, so the entity coverage the operator eventually wants is there and this card is the extension catching up. It takes exactly one reference per invocation, which is what makes bulk a question rather than a flag.

The extension has no archive command at all. Its manifest contributes twenty-two commands and none of them archives anything; the nearest is `dinah.tree.deleteAttachment`. It does have `confirmDestructive`, already used by the card commands, so the confirmation a destructive act needs has a shape to follow rather than to invent.

Multi-select is off, and off deliberately. `createTreeView` in `extension.ts` sets no `canSelectMany`, so it defaults to false, and `dragAndDrop.ts` documents that assumption in prose and reads only the first dragged row because of it. Turning multi-select on therefore falsifies a comment in shipped code, and that comment is the sort a reader trusts.

The eight message catalogues already carry `history.event.archived` and `history.event.restored`, added for forward compatibility against a binary that writes those events. Their context notes say no command writes a restore today. Read those notes before adding strings, because the wording convention for this family is already settled there.

## The operator's two rulings, taken 2026-09-13

**Multi-select applies to the other commands too, acting on all selected rows where that makes sense.** Selecting five cards and having Claim quietly claim one reads as a bug, and he ruled against that. Every one of the twenty-two commands needs a decision recorded: what it does with a selection of several, and what a partial failure looks like when the third of five refuses. That last question is the substance of this card, because the CLI archives one reference per call, so any bulk act is a loop and a loop can stop halfway.

**Archive ships without a way to see the archive, and the view is filed as its own card.** He took that trade knowingly. Say plainly in the confirmation text that an archived card cannot be found again from the extension, because that is true until the companion card lands and a reader deserves to know it at the moment they act rather than afterwards.

## Scope

Cards are what the operator is in pain over, so cards ship. The CLI already archives columns, workstreams and entities below a card, so nothing here should be built in a way that has to be torn up to reach them; whether the command is offered on those rows in this card is a scope decision to take and record, not to widen into silently. A column carries refusal rules a card does not, `dinah.occupied` and `dinah.last-column`, so offering it there is not the same work.

## What will need care

A loop over a one-reference verb is where this gets interesting rather than where it gets tedious. Decide before building what the reader sees when four of twelve succeed and the fifth refuses: whether the run stops or continues, whether the successes stand, and what the message says. A bulk act that reports only its first failure, or that reports success because it never looked, is the defect class this board meets most often.

Every count in whatever guards this card ships wants a real floor rather than a more-than-zero check, and every plant wants performing.

## Specification

Worked against `2aba8fc` on `dinah-490-multi-select-and-archive`, whose `editors/vscode/src/` is byte-identical to the trunk at `4c33c2c` ("CI: pin every action to a commit, and sign what the release publishes (#255)"), in a worktree at `C:/dinah-scratch/dinah-490-spec6/wt`. Round 6, revised against the Agent Design Review in comment 7560. Round 7 revises it again against comment 7562, in a worktree at `C:/dinah-scratch/dinah-490-spec7/wt`.

Code is cited by symbol rather than by line throughout, because this card's own diff moves nearly every line it would otherwise name. The one exception is `src/dragAndDrop.ts:71`, where the point is the exact sentence being deleted and AC-17 quotes it in full.

**How to read an arming recipe on this card.** Six recipes here have been found vacuous or uncompilable across rounds 3 and 4, and the cause is structural: a recipe written against code that does not exist yet is a guess. So a plant that has been performed is performed against a reduction of the declared shapes, committed at `docs/specs/dinah-490-plant-reductions/` so that a later round re-runs it rather than re-reasoning about it, and the criterion it belongs to carries a note that opens with the word PERFORMED, names the plant, and names the assertion it reddened.

Thirteen of this card's thirty-six criteria carry such a label and twenty-three carry none, so read the absence of a label as "prescribed, not run" rather than as an oversight. Five say PERFORMED outright: AC-15, AC-32, AC-33, AC-35 and AC-36. Seven say NOT PERFORMED and say what a reader must build first: AC-14, AC-16, AC-19, AC-29, AC-30, AC-31 and AC-34. One, AC-13, says both, because its count clauses were performed in round 5 and its placeholder clauses are new at round 6 and were not. That census is derived rather than counted off by eye, and the derivation is this: read the card's acceptance criteria, and for each one count the occurrences of `NOT PERFORMED` in its text and note together, subtract that number from the occurrences of `PERFORMED`, and bin the criterion as PERFORMED where the remainder is positive and the `NOT PERFORMED` count is nought, as NOT PERFORMED where the reverse holds, as mixed where both are positive, and as unlabelled where both are nought.

```python
# Reads the JSON of get_card(card_id="dinah-490", fields="checklist") from a file.
import json, re, sys

criteria = json.load(open(sys.argv[1], encoding="utf-8"))["checklist"]["acceptance_criteria"]
bins = {}
for item in criteria:
    text = (item.get("text") or "") + " " + (item.get("note") or "")
    negative = len(re.findall("NOT PERFORMED", text))
    positive = len(re.findall("PERFORMED", text)) - negative
    key = ("both" if positive and negative else "performed" if positive
           else "not-performed" if negative else "unlabelled")
    bins.setdefault(key, []).append(item["local_ref"])
print({key: (len(refs), refs) for key, refs in sorted(bins.items())})
```

That run returned 5 PERFORMED, 7 NOT PERFORMED, 1 mixed and 23 unlabelled, which are the figures above. Re-run it after any edit to the checklist rather than adjusting the sentence, because round 5's spec claimed here that every criterion carried a label when four did, and round 6's claimed ten when thirteen did, and both figures were arrived at by reading. Test performs the twenty-three unlabelled recipes rather than reasoning about them.

No figure describing the tree is written into this spec. Every count this card ships lives in exactly one place, which is the criterion that asserts it, and the prose below names that criterion instead of quoting its number.

## What the reading confirmed, and two corrections

Every code fact the description carries was checked against the tree rather than against the description.

- The manifest contributes the same set of commands `TREE_COMMANDS` in `src/identity.ts` lists, in the same order, which `test/unit/manifest.test.ts` holds by `deepEqual`. None of them archives anything.
- `dinah archive <ref>` takes exactly one reference. `commands.go` declares `{name: "archive", group: groupWork, run: runArchive, bounded: 1}`, and `runArchive` reads `at(parsed.rest(), 0)`. A second reference on the line is not a second archive.
- `vscode.window.createTreeView` is called once in `src/extension.ts`, and its options carry `treeDataProvider` and `dragAndDropController` and nothing else. The only `canSelectMany: false` in the extension is the attach-file dialog's option in `src/creationCommands.ts`, which is not the tree's.
- The drag-and-drop comment exists and reads, at `src/dragAndDrop.ts:71`: "Only the first element is read. The view does not set canSelectMany, so a drag carries one row, and this card neither adds multi-select nor makes a column row draggable for reordering." That sentence becomes false the moment the view sets the option.
- The eight runtime catalogues under `src/locales/` (af, cs, de, en, es, fil, hi, id) carry `history.event.archived` and `history.event.restored`, with the context notes the description quotes.

**First correction, carried from round 1.** `confirmDestructive` is declared on `CommandHost` in `src/cardCommands.ts` and called at exactly one site, `deleteAttachment` in the same file. No card command uses it. The convention is real and the precedent is the attachment delete rather than a family of card commands.

**Second correction, from round 1's review.** Archiving a card somebody holds is not refused, and the reason is `Library.Archive` in `internal/verb/beyond.go` rather than the check table. That function refuses an absent operator, an unresolvable reference, an absent actor, and an entity of workbench kind. It reads nothing about a claim. `internal/verb/checks.go` is a table of declared preconditions rather than the verb's refusal order, and the verb refuses on two conditions that table does not list. The reviewer ran the verb in a scratch workbench with `DINAH_HOME` pointed at a scratch directory: a card pulled into Doing so that it stood `active` and held, then `dinah --json --workbench <wb> archive wb-1`, answered `{"outcome":"ok","verb":"archive"}`. So the confirmation dialog really is the only thing standing between a misclick and an active card leaving the board.

**One observed effect the reader can see, recorded so nobody downstream reports it as new.** Archiving a held card leaves the claim behind. Immediately after the run above, `dinah status` showed Doing with `Cards 0` and `Work taken`, so a column reads as occupied with no cards in it. A later `dinah pull` into that same column still succeeded, so nothing wedges. The effect is cosmetic and it clears when the claim expires or the card is restored. The confirmation does not mention it, and D-11 records why.

**The route back exists and was run, not read.** `dinah restore wb-1`, given the plain card reference and no flag, answered `{"outcome":"ok","verb":"restore"}` against the card archived above, and the card came back to Doing. The copy in §6 rests on that run.

## What round 5's review changed

Round 5's reviewer checked the drag premise against the installed type definitions rather than against this spec's sentence, ran the four committed reduction scripts exactly as the README writes them, and reconciled every count on the card against the real files. All of that holds and none of it is re-derived here.

Three things changed, and the first is this card's central promise.

The first is the blocker. The card promises that a run over several rows produces one message, and the mechanism round 5 shipped silenced error messages alone, so three selected workbenches produced three informational toasts plus the summary and five selected columns spoke once per empty one. §3a's claim that three routes existed did not follow from a wrapper that intercepted two members of one host. The repair is at the interface rather than at the call sites: `src/reporter.ts` declares `ReporterHost` and the two channel sets, all three host interfaces extend it, and `collectingHost` is generic over the host and intercepts every member `REPORT_CHANNELS` names. A collected informational or warning message becomes a note, notes reach the channel, and `summaryFor` gains a case so that a run whose rows had something to say cannot report itself as uneventful. This is round 2's blocker class arriving through a second message channel, which is why the repair is stated over a declared set rather than over the members somebody remembered.

The second is the copy sweep. Round 4 asked which sentences name a number, and the question is which sentences a run raises once over rows they do not describe. Asking it that way reached Block's "Why is this card blocked?" and Move's "Move {ref} to", neither of which names a count and both of which lie over five cards, and it reached the two copy toasts, whose `{ref}` and `{path}` would have been filled with a newline-joined list. Five new keys land in §7 and the figures AC-16 pins move with them.

The third is a false claim about this card's own verification, corrected at the top of this spec. The claim that every criterion's note said whether its plants had been performed was true of four criteria out of thirty.

## What round 4's review changed

Round 4's reviewer traced the registration chain rather than reading it, confirmed the `commandTable.ts` extraction is sound, wrote the missing clause into AC-3 themselves, and replaced two of AC-3's plants that did not compile. All of that is kept as they wrote it, and §1 below is brought into line with the criterion's new text.

Three things changed here.

The first is the blocker, and it is a design decision rather than a repair. §8 declared what `applyDropVerdicts` takes and never declared what crosses the mime entry between the drag and the drop, so a criterion driving the drop function with a hand-built list of four rows went green over a list the editor would never build. VS Code hands `handleDrop` a target and a data transfer and no selection, so the mime entry is the only channel there is. §8 now declares that channel, D-22 records the choice and the alternative that was rejected, `dragRowsFrom` is minted to read the channel back, and AC-15 drives the round trip rather than the drop function alone.

The second is the count vocabulary. Round 4 corrected Archive's confirmation to name the number of rows that resolved to a card context and left §5's Move branches keyed on the number of rows targeted, which makes one card and two column headers produce a sentence about three cards. §5 is corrected, and the rule behind it is stated once in §3a and then checked against every command on the card rather than against the two that had been touched. That sweep found a third site: §2's table commits Delete Attachment to one confirmation naming a count, and the catalogue holds no plural for it, so §7 mints one.

The third is the arming recipes. Six recipes on this card have now been found vacuous or uncompilable, three of them by round 3 and three by round 4, and the cause is that a recipe written against code that does not exist yet is a guess. So every plant this round mints was performed against a reduction of the shapes §5 and §8 declare, committed at `docs/specs/dinah-490-plant-reductions/`, and each criterion's note says which plants were performed and which were not. A plant this round did not perform says so in its own note.

## What round 3's review changed

Round 3's reviewer proved by compilation that a report is a value only `src/bulk.ts` can mint, could not empty the subject set of any sweep, and found every count on the card matching the code. The design below is unchanged above the wiring. Three things in the wiring were wrong, and all three are repaired here.

The first was a contradiction. §1 had each registration loop map its rows through `contextFor` and hand the command "the surviving list", which is a filter, while §3a said no filter stands between the reader's rows and the report. §1 was wrong and is rewritten. Filtering now happens nowhere: every command hands `runBulk` the whole targeted list, and `resolve` inside `runBulk` is the only thing in this card that reads whether a row yielded a context.

The second was a criterion nobody could satisfy. AC-19 asked a unit test to invoke the handler `extension.ts` registers, and `test/unit/layers.test.ts` forbids the unit layer from reaching `extension.ts` at all. Extracting two inline handlers, which is what round 3 offered, bought importability without buying the mapping from a command id to a function. §1 now moves the whole mapping into `src/commandTable.ts`, a module that imports no vscode symbol, so the test imports the real table rather than a copy of it.

The third was a vacuous arming recipe in AC-24, corrected in place by the reviewer and kept as they wrote it. Every other recipe on this card was re-read with the same question, which is whether something upstream intercepts the plant before the code being aimed at runs. Two more were wrong, both in AC-26 and AC-28, where a plant that pointed the walk at an empty directory was said to leave an exactly-one assertion green when an exactly-one assertion over nothing fails like any other. Both plants are replaced with ones whose red and green halves are real.

## What round 2's review changed, and why the shape moved

Round 2's reviewer attacked the reconciling report directly and could not make it lie. One entry is recorded per row, the counts and the list are the same object, and the summary refuses to speak over a report that does not add up. All of that survives here unchanged.

What the reviewer did instead was walk around it. Four routes reached the reader without going through the construction: a declined confirmation, which had no constructor at all; the drag path, which handed a caller-supplied row count back into the report; the host switch, which keyed on the rows that survived a filter rather than on the rows the reader aimed at; and a special case in the running shape that showed nothing where the summary would have spoken.

Patching four routes would leave the fifth to be found. The repair is §3a: one entry point owns the run from the prompt to the message, the report is a value only `src/bulk.ts` can mint, and a command that wants to say anything to the reader either hands rows to that entry point or refuses before the run starts. The perimeter is then a property of the module rather than a rule each caller has to remember, which is the same replacement this card already made once when the counts came off the callers.

## What ships

Multi-select on the tree, a selection policy declared for every contributed command, an Archive command on card rows, and one way of reporting a run that half worked.

### 1. The view selects many, and one table names every row command

The one `createTreeView` call in `src/extension.ts` gains `canSelectMany: true` in its options object. Nothing else about the view changes.

VS Code's own declaration is what the rest of this design rests on. `TreeViewOptions.canSelectMany` in the installed `@types/vscode` says: "Whether the tree supports multi-select. When the tree supports multi-select and a command is executed from the tree, the first argument to the command is the tree item that the command was executed on and the second argument is an array containing all selected tree items."

Two things follow, and only these two. A handler now receives a second argument, and that argument holds the selection. The declaration does not promise that the executed-on item appears in that array, and no other behaviour of the editor is assumed anywhere below.

The local `register` helper in `src/extension.ts` widens to match:

```ts
function register(
	id: string,
	handler: (
		element: TreeElement | undefined,
		selection?: readonly TreeElement[],
	) => Promise<unknown>,
): void
```

**The mapping from a command id to the function that serves it moves out of `activate()`.** Today `src/extension.ts` holds it three ways: two array literals iterated by two loops, six standalone `register` calls whose handler is an inline arrow, and one array entry whose function is an inline wrapper around `checkWorkbench`. A unit test can reach none of them. `test/unit/layers.test.ts` carries the test "no unit-test file imports the vscode module", `src/extension.ts` is the one module in `src/` that imports vscode as a value, verified by searching the whole of `src/` at `4c33c2c`, and importing `extension.ts` from a unit file would therefore load vscode outside an extension host and fail at run time rather than at review. That is why round 3's AC-19 could not be implemented, and why extracting two handlers would not have been enough: the extraction makes a function importable and says nothing about which id the editor wires it to.

So a new module, `src/commandTable.ts`, holds the mapping, and `activate()` iterates it. The module imports no vscode symbol, which is what the unit layer needs and which costs nothing: every module it names, `cardCommands.ts`, `workbenchCommands.ts`, `columnCommands.ts`, `pullCommands.ts`, `creationCommands.ts`, `diagnostics.ts` and `tree.ts`, already imports none.

```ts
/** What activate() holds and a row command may need some of. */
export interface Wiring {
	readonly exe: string;
	readonly binaryLabel: string;
	readonly spawner: Spawner;
	readonly t: Localizer;
	readonly cardHost: CommandHost;
	readonly workbenchHost: WorkbenchCommandHost;
	readonly columnHost: ColumnCommandHost;
	readonly appendLine: (line: string) => void;
	readonly applyCheckResult: (
		path: string,
		label: string,
		outcome: CliOutcome,
	) => Promise<void>;
}

/** One contributed command that reads rows. */
export interface RowCommand {
	readonly id: string;
	/** Runs the command over every targeted row and answers the one report. */
	readonly invoke: (
		elements: readonly TreeElement[],
		wiring: Wiring,
	) => Promise<BulkReport>;
}

/** Every contributed command whose declared policy is not `noRow`. */
export const ROW_COMMAND_TABLE: readonly RowCommand[];
```

Each entry's `invoke` composes the contexts and calls `runBulk`. The contexts are composed inside `runBulk`'s `resolve`, which is the whole of the repair for round 3's first blocker: **no command filters its targeted rows.** The list `invoke` receives is the list `targetsFor` answered, `invoke` passes that list to `runBulk` whole, and `resolve` is the only thing anywhere in this card that reads whether a row yielded a context. A row that yields none is recorded rather than dropped, so `selected` is the number of rows the reader aimed at in every command without exception.

`Wiring` is what lets the entries be values in a pure module rather than closures over `activate()`. Round 3's shape needed a closure for `dinah.tree.checkWorkbench`, because its wrapper reads a `diagnostics` object the context does not carry, and for `dinah.tree.openAttachment`, whose handler writes to the channel. Both are members of `Wiring` now, `applyCheckResult` declared structurally so that `commandTable.ts` names no type from a module that owns a `DiagnosticCollection`, and neither command needs a closure.

`extension.ts` then registers in two shapes and no others:

```ts
for (const { id, invoke } of ROW_COMMAND_TABLE) {
	register(id, async (element, selection) =>
		invoke(targetsFor(element, selection, elementKey), wiring),
	);
}
```

and one standalone `register` call for each `noRow` command, whose handlers read no row and are unchanged. That is fewer `register` calls in the file than there are today, and AC-3 is written to the new shape: it asserts how many calls the file holds and how many of them name a `noRow` id, both against literals; it asserts that every call in the `noRow` group registers a handler declaring no parameter; it asserts that the one remaining call sits inside a `for`-`of` whose iterated expression is the identifier `ROW_COMMAND_TABLE`, imported into `extension.ts` from `src/commandTable.ts`, and registers a handler declaring both parameters, passing both to `targetsFor`, and calling the `invoke` the loop bound from that iterated value; and it asserts that the id set of `ROW_COMMAND_TABLE` equals the set of `SELECTION_POLICIES` keys whose policy is not `noRow`, in both directions. Naming the iterated identifier is what makes the rest of the sentence worth anything: a loop over a second array of the same shape satisfies every other assertion while `ROW_COMMAND_TABLE` wires nothing and AC-19 drives a table the editor never reads. Those two literals in AC-3's own text are the only statement of either figure on this card, and the derivation behind them is in its note.

What this buys AC-19 is the thing round 3's extraction did not. The test imports `ROW_COMMAND_TABLE`, finds the entry whose `id` is the command's, and calls that entry's own `invoke`, so the subject of the assertion is the value `extension.ts` iterates rather than a function a test table claims corresponds to it. What it still does not buy is stated in §2 beside the criterion, because a wiring guard that is credited with more than it proves is this card's recurring defect.

The loop needs two functions over a row, and one of them is minted in `src/tree.ts`, beside the row shapes it reads, rather than invented at the call site.

```ts
/** A row's identity, for deduplicating a selection against the invoked row. */
export function elementKey(element: TreeElement): string;
```

`elementKey` answers a string that distinguishes any two rows the tree draws: the kind, then the workbench root, then whichever of the reference, the node reference, the attachment identifier or the text the row carries, joined by a character none of them contains. It is a key rather than a display string and nothing renders it. The reference a `BulkEntry` carries is a different thing and already exists: `treeItemFor(element, t).label` is the label the reader sees in the tree, exported and pure today, and it is what `runBulk` records for a row that resolves to no context.

Archive joins `ROW_COMMAND_TABLE` as one more entry rather than taking a registration of its own, and D-16 records why.

### 2. `src/selection.ts`, a new pure module

`src/selection.ts` exports a policy declaration and one resolver.

```ts
/** How one command treats a selection of more than one row. */
export type SelectionPolicy = "fanOut" | "oneInput" | "rowOnly" | "noRow";

/** What a fanOut command's effect looks like over several rows. */
export type FanOutEffect = "perRow" | "oneCall";

export interface SelectionEntry {
	readonly policy: SelectionPolicy;
	/** Declared by every fanOut entry and by no other. */
	readonly effect?: FanOutEffect;
}

/** What every contributed command declares, keyed by command id. */
export const SELECTION_POLICIES: Readonly<Record<string, SelectionEntry>>;

/** The rows a command was aimed at, given what the editor handed the handler. */
export function targetsFor<T>(
	element: T | undefined,
	selection: readonly T[] | undefined,
	keyOf: (item: T) => string,
): readonly T[];
```

`targetsFor` resolves in three cases.

- The selection is absent or empty. The answer is `[element]`, or `[]` when the element is absent too. This is the Command Palette invocation and the single-row invocation, and it keeps today's behaviour byte for byte.
- The selection is present and the element's key is among its keys. The answer is the selection, in its own order, with duplicate keys removed.
- The selection is present and the element's key is not among its keys. The answer is the element followed by the selection.

The third case is the one that needs its reason on the record. The editor's declaration says the array holds the selected items and says nothing about the invoked one, so the only reading that cannot silently drop the row the reader aimed at is to include it. Dropping that row is the shape the operator ruled against, in its worst form: a reader right-clicks one card and Dinah acts on five others.

Deduplication is by a key the caller supplies rather than by object identity, because nothing documents that the editor hands back the same object in both arguments. `targetsFor` stays generic in the row type so that the drop path and a future caller can supply their own key; `extension.ts`'s one registration loop supplies `elementKey` from §1.

`SELECTION_POLICIES` is the declaration that makes the operator's ruling enforceable. Its four meanings:

- `fanOut`: the command acts on every selected row of the kind it handles.
- `oneInput`: the command asks the reader once, before it spawns anything, and applies that one answer to every selected row of the kind it handles.
- `rowOnly`: the command acts on one row, and a selection of more than one actionable row is refused with a message rather than narrowed in silence.
- `noRow`: the command reads no row, so a selection cannot reach it.

The `effect` field is what round 2's review forced into the table. AC-19 drives every `fanOut` handler with two rows and asserts the effect it produced, and two of those commands produce one call rather than two by design, so the expectation has to be declared somewhere. Round 2 put it in the criterion's prose, where the criterion's own text and its note ended up saying different things. Declaring it in the same table the policy lives in means the test reads one record per command and there is no second list for the first to drift from.

The table is the per-command decision this card owes. It carries one entry per contributed command, and AC-2 holds its key set to `TREE_COMMANDS` in both directions.

| Command | Policy | Effect | What a selection of several does |
|---|---|---|---|
| `dinah.tree.refresh` | `noRow` | | Reads no row. |
| `dinah.tree.openCard` | `fanOut` | `perRow` | Opens each selected card's file. |
| `dinah.tree.claim` | `fanOut` | `perRow` | Claims each, continuing past a refusal. |
| `dinah.tree.move` | `oneInput` | | One destination prompt over the shared destinations, then one move per card. The prompt names how many cards resolved rather than one card's reference. AC-13. |
| `dinah.tree.release` | `fanOut` | `perRow` | Releases each. |
| `dinah.tree.block` | `oneInput` | | One reason prompt, the same reason on every card. The prompt asks about the cards that resolved rather than about "this card". AC-14. |
| `dinah.tree.unblock` | `fanOut` | `perRow` | Unblocks each. |
| `dinah.tree.copyCardRef` | `fanOut` | `oneCall` | One clipboard write, the references newline-joined in selection order, and one message naming how many were copied. AC-35. |
| `dinah.tree.checkWorkbench` | `fanOut` | `perRow` | Checks each selected workbench. Each row's clean toast or findings toast becomes a channel note rather than a toast of its own. AC-33. |
| `dinah.tree.copyWorkbenchPath` | `fanOut` | `oneCall` | One clipboard write, newline-joined, and one message naming how many were copied. AC-35. |
| `dinah.tree.editWorkbenchDefinition` | `fanOut` | `perRow` | Opens each definition. |
| `dinah.tree.editColumnInstructions` | `fanOut` | `perRow` | Opens each column's instructions. An unreadable file's toast becomes a channel note. AC-33. |
| `dinah.tree.openAttachment` | `fanOut` | `perRow` | Opens each attachment's file. |
| `dinah.tree.deleteAttachment` | `oneInput` | | One confirmation naming the number of attachments that resolved, then one delete per attachment. AC-30. |
| `dinah.tree.newCard` | `rowOnly` | | Refuses, because one title makes one card. |
| `dinah.tree.attachFile` | `oneInput` | | One file picker and one description prompt, then the file attached to each selected owner. |
| `dinah.tree.pull` | `fanOut` | `perRow` | One pull from each selected column. An empty column's toast becomes a channel note. AC-33. |
| `dinah.tree.openInstructions` | `fanOut` | `perRow` | Opens a tab per card. |
| `dinah.tree.openHistory` | `fanOut` | `perRow` | Opens a tab per card. |
| `dinah.tree.archiveCard` | `oneInput` | | One confirmation naming the number of rows that resolved to a card, then one archive per card. AC-7. |
| `dinah.walkthrough.openFirstSessionGuide` | `noRow` | | Reads no row. |
| `dinah.runVerb` | `noRow` | | Reads no row. |
| `dinah.refreshVerbCatalog` | `noRow` | | Reads no row. |

What the table is held to, and what it is not. AC-2 pins its key set against `TREE_COMMANDS`, and `manifest.test.ts` already pins `contributes.commands` against that same roster by `deepEqual`, so the chain from the manifest to this table is three independent literals and no link of it compares a value against the thing that produced it. AC-3 pins the registration shape §1 describes, including that the id set of `ROW_COMMAND_TABLE` is exactly the set of keys this table declares as something other than `noRow`. AC-19 drives every `fanOut` entry by looking its id up in `ROW_COMMAND_TABLE` and calling that entry's `invoke` with two actionable rows, and asserts the effect that entry's own `effect` field declares. Both arms are stated over what the reader is shown rather than over how many times the host was called, which is the round-5 correction: a `perRow` entry spawns twice and shows the reader exactly one message, and a `oneCall` entry spawns nothing per row, performs one effect in `finish`, and shows exactly one message. Round 5's `perRow` arm asserted two host calls, which pinned the duplicate toasts as the expected result and would have gone green on the very defect the card exists to fix. It carries no exemption list, and that is the repair for round 2's fifth blocker: round 2's criterion let an implementer exempt every entry with a one-line reason each, exercise nothing, and still satisfy a reconciliation that closed over an empty subject set. The subject set is now every `fanOut` entry in `SELECTION_POLICIES`, its size is asserted against a literal so that re-declaring a command out of the family reddens the test rather than shrinking it silently, and the number exercised is asserted against that same size with nothing in between.

**What AC-19 proves, and what it does not.** It proves that the value `activate()` iterates, for a given command id, produces the declared effect over two rows. The link it does not prove is the last one: that VS Code invokes the registered handler with the element and the selection the way `TreeViewOptions.canSelectMany` declares. Nothing in the unit layer can prove that, because proving it means running an extension host, and the integration suite that could is never run on this workbench. AC-3 narrows the unproven part to as little as it can be made, by asserting that the one loop registering every row command passes `invoke` the answer of `targetsFor` over both of the handler's parameters, so what is left unproven is the editor's own behaviour rather than this extension's wiring. That is the same limit AC-1 states for `canSelectMany` itself, and it is stated here rather than left for a reader to infer from a green suite.

Four commands are proven against their declared behaviour by criteria of their own: Move (AC-13), Block (AC-14), New Card (AC-18) and Archive (AC-6 through AC-9). For the rest, what is proven is that the policy is declared, that the handler can see the selection, and for the `fanOut` family that the handler produces the declared effect over two rows. What is not proven for them is that the effect is the right effect in every particular. That limit is stated here rather than left to be discovered, because a table read as a behavioural contract when it is a wiring contract is exactly the shape that lets a command declare `fanOut` and act on one row.

### 3. `src/bulk.ts`, a new pure module

The loop lives here rather than in each command, so that a report's counts cannot disagree with the list they describe.

```ts
/** What one row's act came to. */
export type RowOutcome =
	| { readonly kind: "done" }
	| { readonly kind: "failed"; readonly failure: string }
	| { readonly kind: "skipped"; readonly why: string };

/** One row of the run, and what became of it. */
export interface BulkEntry {
	readonly ref: string;
	readonly outcome: RowOutcome;
}

/** What a run in which nothing was acted on says to the reader. */
export type EmptyRunVoice = "speak" | "silent";

/** What one run did, whole. Only this module can mint one; see §3a. */
export interface BulkReport {
	readonly [reportMark]: true;
	/** The rows targetsFor handed the command; equals entries.length by construction. */
	readonly selected: number;
	readonly entries: readonly BulkEntry[];
	/** True when no act ran, because a prompt was declined or the command refused. */
	readonly cancelled: boolean;
	readonly emptyRun: EmptyRunVoice;
	/** What the collecting host captured, empty on a single-row run; see §3b. */
	readonly notes: readonly CollectedNote[];
	/** A `oneCall` command's own success sentence, composed by runBulk; see §3a. */
	readonly doneMessage?: string;
}

/**
 * Runs one act over every row and records exactly one entry per row.
 *
 * The report is built here rather than by the caller so that its counts cannot
 * disagree with the list they describe: `selected` is `rows.length`, one entry
 * is appended per row before the loop advances, and a row whose act throws is
 * recorded as failed rather than ending the run.
 */
export async function runOverRows<T>(
	rows: readonly T[],
	refOf: (row: T) => string,
	act: (row: T) => Promise<RowOutcome>,
	emptyRun?: EmptyRunVoice,
): Promise<BulkReport>;
```

Each of the following is a property of `runOverRows` rather than of its callers.

- It appends one entry per row of `rows`, so `report.entries.length === report.selected` holds by construction. Neither number is passed in by a caller, and no command accumulates a count of its own.
- `act` rejecting is caught, recorded as failed, and the loop advances. The run never rethrows. This is what puts an unanticipated failure into the failed count instead of outside all three, and the paths that need it are live: `openDocument`, `copyToClipboard`, `openFile` and `openServedText` all reach the editor and all can reject, so a fan-out over Open Card or Copy Reference can lose a whole run at row three without it. The recorded `failure` is `err instanceof Error ? err.message : String(err)`, which is the spelling `src/extension.ts` and `src/servedText.ts` already use, so a rejection carrying a string or `undefined` records something a reader can read rather than the word `undefined`.
- A row is failed when the outcome its act observed has a `kind` other than `"ok"`. `CliOutcome` in `src/cli.ts` is a union whose only successful arm is `ok`, and `refused` is one arm of several: a run whose binary vanished answers `spawn-failed`, a run against an older binary answers `stale`, and defining failure as a refusal would record either as a run of successes. AC-22 enumerates that union from the source rather than from a figure written here, and drives every arm it finds. `refusalMessage` in `src/cardCommands.ts` already renders every non-ok arm through its generic case, so one call covers all of them and no new rendering is written.

### 3a. The perimeter: how a message reaches the reader

Round 2 found four routes that reached the reader without a report, and the repair was a perimeter around the reporting. Round 5 found that the perimeter had been built around one channel rather than around the reader: `collectingHost` intercepted `showError`, so a run over several rows still produced one informational toast per row from Check Workbench and one per empty column from Pull. This section therefore answers two questions rather than one. Through which members can a command speak at all, and which of those members are collected when a run spans more than one row.

**A command speaks through six members and no others.** Every module under `src/` except `extension.ts` imports no vscode symbol, which `test/unit/layers.test.ts` already holds in its test "no module the unit layer reaches imports the vscode module at run time", so a command module holds no handle to the editor and reaches the reader only through the host it was handed. Searching the whole of `src/` for `vscode.window.show` at `2aba8fc`, this card's branch tip, whose `src/` the card has not touched, returns fourteen lines: thirteen calls in `extension.ts` and one mention inside a comment in `src/creationCommands.ts`. Six of the thirteen show a message, and they are one `showErrorMessage`, two `showInformationMessage` and three `showWarningMessage`. Each of the six sits inside one of the three host factories, bound to one named member, and four more arrive with this card: `CommandHost`'s `showWarning`, `WorkbenchCommandHost`'s `showError`, and `ColumnCommandHost`'s `showError` and `showInfo`. Six plus four is ten, and the same figure falls out of the post-card world read per factory rather than as a delta, `commandHost` 4, `workbenchCommandHost` 3 and `columnCommandHost` 3. That is the enumeration this section rests on, and AC-31 is the criterion that holds it.

The six members are declared in a new pure module, `src/reporter.ts`, rather than remembered:

```ts
/** Every member through which a command puts words in front of the reader. */
export const REPORT_CHANNELS = ["showError", "showInfo", "showWarning"] as const;

/** Every member through which a command asks the reader a question. */
export const PROMPT_CHANNELS = ["pick", "input", "confirmDestructive"] as const;

/** What every host carries, so that one wrapper can collect any host's reporting. */
export interface ReporterHost {
	readonly t: Localizer;
	readonly showError: (message: string) => void;
	readonly showInfo: (message: string) => void;
	readonly showWarning: (
		message: string,
		actions: readonly string[],
	) => Promise<string | undefined>;
	readonly appendLines: (lines: readonly string[]) => void;
	readonly revealOutput: () => void;
}
```

`CommandHost` in `src/cardCommands.ts`, `WorkbenchCommandHost` in `src/workbenchCommands.ts` and `ColumnCommandHost` in `src/columnCommands.ts` each extend `ReporterHost` and each drop the members it now carries. What that costs is six one-line bindings in `extension.ts`: `CommandHost` gains `showWarning`, `appendLines` and `revealOutput`, `WorkbenchCommandHost` gains `showError`, and `ColumnCommandHost` gains `showError` and `showInfo`. Four of the six are `vscode.window.show*Message` calls and two are not, since `appendLines` and `revealOutput` bind to `channel.appendLine` and `channel.show`. Each is copied in shape from a binding the same file already holds, and `showWarning`'s is `vscode.window.showWarningMessage(message, ...actions)`, which two of the three factories already write.

That is the repair for round 5's blocker, and it is taken at the interface rather than at the call sites. Round 5's shape made the perimeter a property of `CommandHost` and left two other hosts outside it, so closing the hole by naming `showInfo` and `showWarning` in `collectingHost` would still have left a workbench command's host unwrapped. One declared interface, extended by every host, means a command that can speak is a command whose messages a run can collect, and a fourth host would have to extend it to be usable at all.

**Under a multi-row run, every REPORT_CHANNEL is collected and every PROMPT_CHANNEL reaches the reader.** The prompts are the reader's own question and must arrive; the reports are what the summary replaces. `collectingHost` is generic over the host type and intercepts the members `REPORT_CHANNELS` names, plus `checkpoint` where the host carries one, and forwards everything else. AC-32 drives it over all three host shapes, each built with the members that shape declares in the source rather than with one member list repeated, so the prompt-identity half of that criterion reaches `CommandHost` alone and says so.

**The promise is one message per run, and it is a promise about messages rather than about tabs.** `collectingHost` forwards every member `REPORT_CHANNELS` does not name, and `openDocument`, `openFile` and `openServedText` are three of those members. Each opens an editor tab, §2's table declares six `fanOut` commands `perRow` over them, and one tab per selected row is what those commands mean rather than a hole in the perimeter. A reader who selects five rows and runs one of them therefore gets five tabs and one message, which is the design. An implementer must not extend the collection to reach those members in order to make the tab count match the message count, and a tester must not read five tabs as a failed run. The first complaint phrased as "it opened five things" is this paragraph and not a defect.

**What a collected message becomes.** A collected `showError` is discarded, because the row that produced it also records the same text as its failure, which reaches the channel as that row's line; AC-36 drives a run in which every row fails and asserts one channel line per row, so a change that made the two texts diverge would show up as a missing line rather than as silence. A collected `showInfo` or `showWarning` becomes a **note**, and notes are neither discarded nor shown as toasts: they are written to the channel in the order they were collected, and their number changes what the summary says. Every note-producing sentence already names the row it is about, so a note in the channel is readable without the run attributing it. That claim is produced by the same sweep as the enumeration above rather than by reading the ones that came to mind: the `showInfo` and `showWarning` call sites in `src/` reachable from an act are `checkWorkbench`'s clean branch, `offerOutput` on its two callers in `workbenchCommands.ts`, the empty-pull branch in `pullCommands.ts`, and the unreadable-instructions branch in `columnCommands.ts`, and the five keys they raise, `dialog.workbench.checkClean`, `dialog.workbench.checkFindings.toast`, `dialog.workbench.noReport.toast`, `dialog.pull.empty` and `dialog.column.instructionsUnreadable.toast`, are each filled from their row's own label.

A collected `showWarning` answers `undefined`, which is what the editor answers when the reader dismisses a toast without choosing an action. Both of this card's `showWarning` callers, `offerOutput` in `src/workbenchCommands.ts` and the unreadable-instructions branch in `src/columnCommands.ts`, reveal the channel only on a non-`undefined` answer, so collecting the warning also stops a run of five rows revealing the output channel five times. The summary's own warning still carries the `dialog.openOutput.label` action, so the reader keeps one route to the channel rather than none. AC-32 pins the `undefined` answer.

**The prompts and the refusals run before the switch.** `runBulk` resolves the rows, then calls `ask` with the real host, and only then wraps the host for the loop. A refusal composed inside `ask`, which is where Move's `dialog.move.noSharedDestination` and New Card's `dialog.bulk.oneRowOnly` are shown, therefore reaches the reader rather than being collected into a summary that a cancelled run makes silent. This is load-bearing and it is easy to lose: handing `ask` the collecting host makes a refused run say nothing at all, which is plant G in the reductions and which reddens AC-32's first clause.

**What a count means, settled once and applied to every sentence a run raises.** Two numbers are in play on every run and they are not the same number.

- The report counts **targeted** rows. `selected` is `rows.length`, the rows the reader aimed at, and `entries` holds one per row whether it resolved or not. `dialog.bulk.partial` names that number beside `failed` and `skipped`, so the reader is told how many of the rows they aimed at came to nothing.
- Every prompt and every refusal counts **resolved** rows. This is structural rather than a convention to remember: `ask` is declared as `(resolved: readonly R[], host) => Promise<A | undefined>` and is handed only the rows that yielded a context, so a message composed inside `ask` has no access to the targeted count and cannot name it by accident.

**Which sentences a multi-row run makes false, which is a different sweep from which sentences name a number.** Round 4's sweep asked which sentences name a count, and round 5's review was right that this is the wrong question: a singular sentence misdescribes a run of five exactly as a wrong number does. The question is which sentences a run raises **once** over rows it does not describe. A sentence raised once per row is not at risk, because it is filled from its own row.

The sweep that answers it is mechanical. Take §2's table, take every entry whose policy is `oneInput` or `rowOnly` or whose effect is `oneCall`, because those are exactly the entries that raise one sentence over many rows, and read every catalogue key each of them raises. `oneInput` raises its prompt once over the selection and `oneCall` raises its report once over it; `rowOnly` raises its refusal once over it, which is the sentence this card mints `dialog.bulk.oneRowOnly` for, and a subject set stated over the first two alone walks past that class while covering it in fact. That produced five sentences, and all five are repaired in §7:

| Command | Key raised once | What a run of several makes of it | Repair |
|---|---|---|---|
| Move | `dialog.move.placeholder` ("Move {ref} to") | Names one card's reference while moving five. | `dialog.move.placeholder.many`, filled with the resolved count. |
| Block | `dialog.block.reasonPrompt` ("Why is this card blocked?") | Asks about one card while blocking five. | `dialog.block.reasonPrompt.many`, filled with the resolved count. |
| Delete Attachment | `dialog.attachment.delete.confirm` | Names one filename and one reference. | `dialog.attachment.delete.confirm.many`, filled with the resolved count. |
| Copy Reference | `dialog.card.copiedRef` ("Copied {ref}") | Fills `{ref}` with a newline-joined list. | `dialog.card.copiedRef.many`, filled with the count copied. |
| Copy Path | `dialog.workbench.copiedPath` ("Copied {path}") | Fills `{path}` with a newline-joined list. | `dialog.workbench.copiedPath.many`, filled with the count copied. |

Three sentences the sweep reached and left alone, with the reason recorded so that a later reader does not take the omission for an oversight. `dialog.attach.descriptionPrompt` ("Description for this attachment, or leave blank") is about the one file being attached rather than about the rows receiving it, and one file is what Attach File puts on every selected owner, so it is true over any selection. `dialog.newCard.titlePrompt` is raised only after New Card has refused a selection of more than one column, so it is never raised over several rows at all. `dialog.move.noLegalMoves` is reached only on §5's third branch, where exactly one card resolved.

**The routes, stated so that a fourth is visibly a fourth.** After this card a message reaches the reader through these and nothing else:

1. The summary, from `summaryFor` over a marked report, shown by `runBulk` through the real host's `showInfo` or `showWarning`. At most one per run.
2. A command's own prompt or refusal, inside `ask`, on the real host, before the loop starts. A refusal there makes route 1 silent, because the report is cancelled.
3. The real host's own per-row calls when `rows.length <= 1`, which is today's single-row behaviour unchanged, and where route 1 is silent for the same arithmetic.

Route 3 is stated over `REPORT_CHANNELS` as a set rather than over `showError` alone, which is the sentence round 5 got wrong. Nothing else in `src/` may show a message about a multi-row run, and what makes that checkable rather than asserted is AC-31, which walks `extension.ts` and refuses a `vscode.window` message call bound to any member outside the two declared sets, together with AC-34, which counts the report-channel call sites in the command modules so that a new one has to be classified rather than merely added.

**A report is a value only `src/bulk.ts` can mint.** The module holds an unexported `const reportMark = Symbol("dinah.bulk.report")`, every report it builds carries that key, and `summaryFor` throws on a report that lacks it. A hand-built object literal cannot obtain the symbol, because it is not exported, so the reconciliation arm stops being the only thing between a future caller and a fabricated summary. AC-25 pins both directions of that refusal.

The mark and the reconciliation cover different attacks, and the limit of each is worth stating rather than leaving to be found. The mark refuses a report fabricated from nothing. It does not refuse one spread from a real report with the counts changed, because object spread copies the symbol along with everything else, and that case is the reconciliation arm's. Neither refuses a module that composes a message without a report at all, and what bounds that is AC-26 and AC-34 together with the fact that only `extension.ts` imports `vscode` in the whole of `src/`.

One route walks past both, and it is written down here rather than closed. A caller holding any real report can read the symbol off it with `Object.getOwnPropertySymbols` and mint a fresh object carrying that key, with counts and entries that add up to whatever it likes. The mark arm accepts it because it carries the mark, and the reconciliation arm accepts it because it reconciles. Nothing in a JavaScript object graph prevents that, and the thing being defended against is a future author taking a shortcut rather than an adversary, so buying a defence against a deliberate forgery would cost the module a private field and a class and buy nothing the shortcut case does not already get.

The workbench's rule is that a gap gets recorded where somebody meets it, so `bulk.ts`'s header carries that route. The text it carries is quoted here rather than specified by sentence position, because the paragraph above is spec voice and its first sentence loses its antecedent the moment it is lifted out:

```
// The report mark and summaryFor's reconciliation arm both have one route
// past them, and it is recorded here rather than closed. A caller holding any
// real report can read the mark off it with Object.getOwnPropertySymbols and
// mint a fresh object carrying that key, with counts and entries that add up
// to whatever it likes. The mark arm accepts that object because it carries
// the mark, and the reconciliation arm accepts it because it reconciles. The
// route stays open because what this module defends against is a future
// author taking a shortcut rather than an adversary, and closing it would
// cost a private field and a class and buy nothing the shortcut case already
// lacks.
```

Nothing checks that text mechanically, and no criterion on this card asserts it. A criterion asserting a comment's presence proves that somebody wrote a comment, and one asserting its wording proves that somebody maintained the wording twice; this board has shipped both shapes and neither caught anything. The check is a human read at Test against the block above.

**One entry point owns the run.** Every command that acts on rows calls `runBulk`, which resolves, prompts, chooses the host, runs the loop, performs a `oneCall` command's single act, checkpoints, writes the channel lines, and shows the one message.

```ts
export interface BulkDeps<R, H extends ReporterHost> {
	readonly host: H;
	readonly t: Localizer;
	/** The channel line for a row the command cannot act on. */
	readonly skipReason: string;
	/** Archive alone sets this; see D-17. */
	readonly lineEveryRow?: boolean;
	/** The drop path alone sets "silent"; see D-19. */
	readonly emptyRun?: EmptyRunVoice;
	/**
	 * The one act a `oneCall` command performs after the loop, over the rows
	 * that finished. It shows nothing; see D-23.
	 */
	readonly finish?: (finished: readonly R[], host: H) => Promise<void>;
	/** The sentence a wholly successful run shows instead of the bulk one; see D-23. */
	readonly successMessage?: (finished: readonly R[], t: Localizer) => string;
}

export async function runBulk<T, R, A, H extends ReporterHost>(
	rows: readonly T[],
	refOf: (row: T) => string,
	resolve: (row: T) => R | undefined,
	deps: BulkDeps<R, H>,
	ask: (resolved: readonly R[], host: H) => Promise<A | undefined>,
	act: (resolved: R, answer: A, host: H) => Promise<RowOutcome>,
): Promise<BulkReport>;
```

`rows` is what `targetsFor` answered, unfiltered, and §1 is written to match: a `ROW_COMMAND_TABLE` entry's `invoke` passes its whole element list here and composes no contexts of its own. `resolve` maps a row to the context the command acts on, or to `undefined` for a row of the wrong kind, and `runBulk` records that row as `{ kind: "skipped", why: deps.skipReason }`. **Filtering a targeted row out of a run happens in no command and in no loop; `resolve` inside `runBulk` is the one place this card reads whether a row yielded a context, and it records the row either way.** So the number of rows the run reports is the number of rows the reader aimed at. This is what closes round 2's third blocker at the root: the host switch below reads `rows.length`, which is the targeted count, so a selection of one card and two column headers takes the collecting host and produces one message rather than a per-row toast under a summary.

`ask` runs once, before anything spawns, and it is where a command asks its question or states its refusal. Answering `undefined` means no act runs, and `runBulk` then builds the cancelled report itself, one `skipped` entry per row carrying `deps.skipReason`, with `cancelled` true. A caller never constructs that report, which closes round 2's first blocker: the shape that used to throw is no longer spellable, and the shape that is built reconciles like any other. `summaryFor`'s cancelled case then shows nothing, so a command that told the reader why it refused inside `ask` does not get a second message on top of it.

`finish` and `successMessage` are what the two `oneCall` commands declare instead of reporting per row, and D-23 records the decision. Copy Reference's `act` performs no effect and records the row `done`; its `finish` makes the one clipboard write over the rows that finished; its `successMessage` composes `dialog.card.copiedRef` when exactly one row finished and `dialog.card.copiedRef.many` when more did, so the single-row gesture keeps the sentence it shows today and a multi-row gesture gets one true sentence rather than one toast per row or a `{ref}` filled with a list. Copy Path declares the same three over `dialog.workbench.copiedPath`. `finish` shows nothing at all, which is what keeps route 1 the only route these two commands speak through, and plant H in the reductions is the shape where they report per row instead.

**The host switch lives here and nowhere else.** `runBulk` wraps `deps.host` in `collectingHost` when `rows.length > 1` and hands the real host through when it is 1 or 0. The same number decides the summary's single-row case, so the per-row toast and the summary are mutually exclusive by arithmetic rather than by two call sites agreeing. AC-26 asserts that `collectingHost` and `summaryFor` are each named in exactly one file under `src/`, and AC-27 drives the one-survivor case end to end.

### 3b. `collectingHost` and `summaryFor`

```ts
/** A collected message, and the level it would have been shown at. */
export interface CollectedNote {
	readonly level: "info" | "warning";
	readonly text: string;
}

/**
 * A host that captures what a per-row call would have shown, so a run of
 * several rows produces one message rather than one message per row.
 *
 * Generic over the host, because three host interfaces extend ReporterHost and
 * a run collects whichever one its command was given. Every member named in
 * REPORT_CHANNELS is intercepted, `checkpoint` is intercepted where the host
 * carries one, and every other member, the PROMPT_CHANNELS among them, is the
 * real host's.
 */
export function collectingHost<H extends ReporterHost>(
	host: H,
): {
	readonly host: H;
	readonly drain: () => {
		readonly errors: readonly string[];
		readonly notes: readonly CollectedNote[];
		readonly folders: readonly string[];
	};
};

/** The single message a run produces, or none. */
export function summaryFor(
	report: BulkReport,
	t: Localizer,
): { readonly level: "info" | "warning" | "none"; readonly message: string };
```

`collectingHost` answers `undefined` from the intercepted `showWarning`, which is the answer the editor gives when the reader dismisses a toast, so a caller that reveals the channel on assent reveals nothing. `checkpoint` records the folder and runs nothing. `confirmDestructive`, `input` and `pick` reach the real host, because the prompts are the reader's and only the reporting is being collected. Nothing here reads a member by string at run time: the interception is written member by member against `ReporterHost`, and `REPORT_CHANNELS` is what AC-31 and AC-32 hold that hand-written set against.

`BulkReport`, declared in §3 above, carries `notes` and `doneMessage` so that `summaryFor` stays a pure function of one value. `runBulk` fills both: `notes` from the drain, and `doneMessage` from `deps.successMessage` over the rows that finished, or `undefined` where the command declared none.

`summaryFor` counts `done`, `failed` and `skipped` off `report.entries` and resolves in this order, taking the first case that matches.

1. The report does not carry `reportMark`. It throws. A report the bulk layer did not build must not be able to produce a message, whatever it says about itself.
2. `done + failed + skipped !== report.selected`. It throws rather than composing a message, and the thrown message names both counts. A report that cannot add up must not be able to produce a cheerful one, and this is the arm that makes "the report cannot lie" a checkable claim rather than an asserted one.
3. `report.cancelled` is true. The level is `none`. Nothing ran, and `ask` has already said whatever the reader needed to hear.
4. `report.emptyRun === "silent"` and `done === 0` and `failed === 0`. The level is `none`. Only the drop path declares this, because a drag that missed has to go on looking like a drag that missed; D-19 records the decision and AC-28 holds the declaration to one call site.
5. `failed === 0` and `skipped === 0` and `report.doneMessage` is present. The level is `info` and the message is `doneMessage`. This case is ahead of the single-row case on purpose, because a `oneCall` command's sentence is the whole of what a successful copy shows at any row count, including one.
6. `report.selected <= 1`. The level is `none`. This is the single-row path, and it keeps today's behaviour exactly: the per-row call in `runVerb` is the whole of what a refusal shows, because a single-row run is given the real host rather than the collecting one. A single targeted row that yields no context is this case too, so it writes its channel line and shows nothing, which is what shipped before this card.
7. `failed === 0` and `skipped === 0` and `report.notes` is empty. The level is `info`, and the message is `dialog.bulk.allSucceeded` filled with `done`. It is filled from the number of rows that actually finished rather than from `selected`.
8. `failed === 0` and `skipped === 0`, with notes. The message is `dialog.bulk.allSucceededNotes`, filled with `done` and with the number of notes. The level is `warning` when any note is a warning and `info` when they are all informational, so three workbench checks that found defects escalate exactly as one check does today, and four empty pulls do not. This is the case that stops a clean-looking summary hiding what the rows had to say: the notes are in the channel, and the warning form carries the `dialog.openOutput.label` action that opens it.
9. Anything else. The level is `warning`, and the message is `dialog.bulk.partial` filled with `done`, `selected`, `failed` and `skipped`. A partial run needs no notes clause, because its own sentence already points at the channel the notes were written to.

Case 6 reading `report.selected` alone is a change from round 2, which guarded it with `skipped === 0` and so sent a single unusable row to the partial case while §4 claimed it showed nothing. The two now agree, and the special case in §4 that used to carry the difference is gone.

Case 9 covers the two shapes this board meets most often: a run where one row failed, and a run where every targeted row was of the wrong kind so nothing was attempted at all. Both produce a message, unless the drop path declared silence. A menu invocation that attempted nothing and says nothing is the "reports success because it never looked" failure, and case 9 is written so that it cannot be reached from a menu.

The warning is shown through `showWarning` with the `dialog.openOutput.label` action, and `revealOutput` runs on the reader's assent. That is `offerOutput` in `src/workbenchCommands.ts` applied to a second family. The channel lines are written before the message is shown, and the per-row ones are untranslated English diagnostics, matching the channel lines the two registration loops in `src/extension.ts` write today, which §1 replaces with one. A note's line is the translated sentence the row composed, because that sentence is what the reader would have been shown and it already names its row.

### 4. Running a command over several rows

Every `fanOut` and `oneInput` command follows one shape, and `runBulk` performs all of it.

1. `targetsFor` resolves the rows in `extension.ts`'s one registration loop, and the whole list goes to the `ROW_COMMAND_TABLE` entry's `invoke`, which hands it to `runBulk` whole.
2. `resolve`, declared by that entry and run inside `runBulk`, maps each row to the context the command acts on, through `contextFor`, `contextForAttach`, `contextForWorkbench`, `contextForColumn` or `contextForPull`, whichever the command already uses. A row that yields no context is a row of `rows` all the same, and it is recorded as skipped, so it is counted by being enumerated rather than by a counter somebody maintains.
3. `ask` runs once, before any spawn. A `fanOut` command's `ask` answers a unit value without prompting. A `oneInput` command asks its question here, and a `rowOnly` command states its refusal here. Answering `undefined` ends the run with the cancelled report and nothing spawned.
4. The host is the collecting one when more than one row was targeted and the real one otherwise, decided in `runBulk` from `rows.length`.
5. `runOverRows` runs the act for each row in selection order.
6. A `oneCall` command's `finish` runs once over the rows that finished, on the real host, and shows nothing. Every other command declares no `finish` and this step does nothing.
7. One checkpoint runs per distinct folder drained from the collecting host, whatever the outcome, after the loop. A run of a dozen cards in one folder checkpoints once rather than a dozen times. A single-row run took the real host, so its checkpoint already happened as it does today.
8. Every row that did not finish reaches the channel, one line per row, as `<ref>: <failure>` for a failed row and `<ref>: <why>` for a skipped one. Then every note the collecting host captured reaches the channel in the order it was collected, each as the sentence the row composed. Then `summaryFor` produces the one message.

Step 8's rule is "every row that is not done" rather than round 2's "every failed row", because the skipped rows are what a reader needs when a gesture did less than they expected, and because the single-row channel line the handlers write today is exactly a skipped row's line. Archive widens the rule one step further and is the only command that does: it lines every attempted row, including the ones that finished, as `<ref>: archived`. The reason is in §6, and D-17 carries it.

A failure does not stop the run, and nothing is rolled back. The decision is taken per command rather than as a principle, and the reasons differ:

- Claim, Release and Unblock are refused for reasons belonging to the individual card, so a refusal on the third says nothing about the fourth, and stopping would abandon work the reader asked for and can see nothing wrong with.
- Archive is refused when the reference does not resolve. Its other two declared preconditions are about columns, which this command does not reach.
- Block applies one reason to each card, and a card already blocked refusing does not change what the rest need.
- Move is the one where stopping has an argument: a refusal can belong to the destination rather than to the card, so a run could refuse identically all the way down. It continues anyway, because the extension does not model which refusals are destination-wide, and deciding that it knows would be predicting the verb's answer. `src/dragAndDrop.ts`'s header already rules that out for the drag path, on the grounds that the sidebar renders the effect of a verb rather than the verb's decision, and the same reasoning binds here. The summary's counts are what tell the reader that the whole selection refused.
- Pull acts on a column at a time, and an empty column refusing says nothing about the next column.
- Attach File puts one file on several owners, and a failure on one leaves the others legitimately attached.
- Delete Attachment deletes one file at a time.

Nothing is undone, because `dinah` has no transaction spanning two references and a compensating run would be a second verb written inside the extension. The successes stand, and the summary and the channel are how the reader learns which ones they were.

### 5. Move, and the shared destination

`moveCard` today spawns `dinah instructions <ref>` for the one card and offers its legal moves. Over a selection it spawns that read once per card, up front, intersects the answers by `move.column`, and asks once. Both the reads and the prompt happen inside Move's `ask`, so an empty intersection is a refusal that ends the run rather than a message shown beside a summary.

```ts
/** The destinations every card in the selection will accept, in the first card's order. */
export function sharedLegalMoves(
	perCard: readonly (readonly LegalMove[])[],
): readonly LegalMove[];
```

The intersection is keyed by `move.column`, which `movePick` in `src/cardCommands.ts` already establishes as the value the move verb takes. The titles and the direction shown are the first card's, and the order is the first card's after `orderLegalMoves`.

The quick pick's placeholder is the sentence §3a's sweep found lying. `dialog.move.placeholder` reads "Move {ref} to" and is filled at `src/cardCommands.ts:341` from the one card's reference, so over five cards it names one of them. Move fills it from `resolved.length`: the existing key when exactly one card resolved, unchanged from today, and `dialog.move.placeholder.many` filled with that count when more did. AC-13 pins which key each count reaches.

Move's branches key on the **resolved** count throughout, which is §3a's rule and which `ask`'s own signature enforces: `runBulk` hands `ask` the resolved list, so `resolved.length` is the only count Move can read. Round 4 said "targeted" here, and under §3a's vocabulary that made a selection of one card and two column headers show a sentence about three cards, and made the single-card branch unreachable whenever any non-card row was selected.

Move resolves in four branches, and they are exhaustive over `resolved.length`:

- The intersection is non-empty. One prompt, then one `dinah move <ref> <column>` per resolved card.
- The intersection is empty and more than one card resolved. No prompt, `showError` carries `dialog.move.noSharedDestination` filled with `resolved.length`, and `ask` answers `undefined`.
- Exactly one card resolved and it has no legal moves. `dialog.move.noLegalMoves` filled with that card's reference, as today and unchanged, and `ask` answers `undefined`.
- No card resolved at all, which is the mixed or all-column selection. Move spawns nothing, asks nothing and says nothing inside `ask`, and answers a unit value rather than `undefined`. The run then proceeds with every row recorded skipped, `cancelled` stays false, and `summaryFor` reaches case 9 and shows the partial warning. That is the same answer Claim gives over the identical selection, which AC-11's second half already pins, so two commands cannot diverge on one gesture. Answering `undefined` here instead would suppress the summary and leave a menu invocation that attempted nothing saying nothing, which is the failure case 9 exists to make impossible.

The `instructions` reads happen over the resolved list, so branch four spawns nothing at all.

Asking once rather than once per card is the decision. A prompt per card over a dozen cards is the shape a reader abandons halfway, and abandoning halfway is exactly the partial result this card exists to make visible. The `instructions` calls are reads, so a reader who cancels the single prompt has changed nothing on the board.

### 6. Archive

A new command, `dinah.tree.archiveCard`, declared as `COMMAND_ARCHIVE_CARD` in `src/identity.ts`, appended to `TREE_COMMANDS` and to `ROW_COMMANDS` in that same file, and given an entry in `ROW_COMMAND_TABLE`.

The name carries `Card` so that offering the same act on a column row later is a second command rather than a rename of this one. A column refuses under `dinah.occupied` and `dinah.last-column`, and explaining either to a reader is work this card is not doing.

Manifest:

- `contributes.commands` gains an entry at the position `TREE_COMMANDS` puts it, with title `%manifest.command.dinah.tree.archiveCard.title%`, resolving to `Dinah: Archive Card` in `package.nls.json`.
- `contributes.menus.commandPalette` gains `{ "command": "dinah.tree.archiveCard", "when": "false" }`.
- `contributes.menus["view/item/context"]` gains `{ "command": "dinah.tree.archiveCard", "when": "view == dinah.workbenchView && viewItem =~ /^dinah\\.card\\./", "group": "9_destructive@1" }`.

Every card state is offered the act, because archiving a card Dinah will not let you claim is exactly the case a reader reaches for. The group sorts last, so a destructive item does not sit against Claim.

The command is two exported functions in `src/cardCommands.ts` and one entry in `ROW_COMMAND_TABLE`, which is the shape every other row command takes:

```ts
/** Archives one card. */
export async function archiveCard(context: CommandContext): Promise<CliOutcome>;

/** Asks once, before anything spawns, and answers undefined when declined. */
export async function askArchiveConfirmation(
	resolved: readonly CommandContext[],
	host: CommandHost,
): Promise<true | undefined>;
```

`archiveCard` is a sibling of `claimCard` and `releaseCard` and runs `["archive", context.ref]` through `runVerb`. `pinnedArgv` puts `--workbench <root>` in front of that argument list and `composeArgv` in `src/cli.ts` returns `["--json", ...args]`, so the argv the spawner sees is `["--json", "--workbench", <root>, "archive", <ref>]`. `askArchiveConfirmation` is the entry's `ask`: it calls `confirmDestructive` once, with `dialog.archive.confirm.one` when one card resolved and `dialog.archive.confirm.many` when several did, and `dialog.archive.action` as the affirmative label. A declined confirmation answers `undefined`, so nothing spawns, `runBulk` builds the cancelled report, and the reader sees nothing further.

Splitting the confirmation from the verb is what keeps D-2's claim true. A later card offering Archive on a column row writes its own `ask`, because the sentence a column needs is not this one, and reuses `archiveCard` and the whole bulk layer unchanged.

The copy has to be true on the day it ships, and it has to name a route the reader can actually take. `dinah restore <ref>` is that route, confirmed by a run. What the reader lacks is the references, and the channel is where this card puts them. The English:

```
dialog.archive.confirm.one
  "Archive {ref}? The card leaves the board. This extension cannot show you
   the archive or put the card back. To put it back, run dinah restore {ref}
   in a terminal."

dialog.archive.confirm.many
  "Archive {count} card(s)? They leave the board. This extension cannot show
   you the archive or put them back. Dinah lists every reference it archived
   in the Dinah output channel, and dinah restore puts one back from a
   terminal."

dialog.archive.action
  "Archive"
```

No code span appears in either sentence. `confirmDestructive` in `src/extension.ts` calls `vscode.window.showWarningMessage(message, { modal: true }, label)`, whose `message` parameter is documented as a string and carries no promise of Markdown rendering, so a backtick reaches the reader as a backtick in all eight languages. `dialog.attachment.delete.confirm` is the precedent in the same family and carries none either.

The context note on both confirmation keys records that dinah-491 makes these sentences wrong, so the card that adds the archive view finds the obligation in the catalogue rather than having to remember it.

### 7. The catalogues

Thirteen new runtime keys in all eight files under `src/locales/`. Eight of them were minted before round 6:

`dialog.archive.confirm.one`, `dialog.archive.confirm.many`, `dialog.archive.action`, `dialog.bulk.allSucceeded`, `dialog.bulk.partial`, `dialog.bulk.oneRowOnly`, `dialog.move.noSharedDestination`, `dialog.attachment.delete.confirm.many`.

Five are minted in round 6, by §3a's sweep over the sentences a run raises once, and by §3b's notes case:

`dialog.bulk.allSucceededNotes`, `dialog.move.placeholder.many`, `dialog.block.reasonPrompt.many`, `dialog.card.copiedRef.many`, `dialog.workbench.copiedPath.many`.

`dialog.attachment.delete.confirm.many` is the one round 4's sweep found. §2's table commits Delete Attachment to `oneInput` with one confirmation naming a count, and the catalogue's `dialog.attachment.delete.confirm` reads "Delete {filename} ({ref})? Dinah destroys the file and cannot bring it back.", which names one file and can fill nothing from a run over several. `deleteAttachment` in `src/cardCommands.ts` calls it at one site today. So the plural is minted here rather than gestured at, and `askDeleteAttachmentConfirmation`, the entry's `ask`, calls `confirmDestructive` once with the singular key when one attachment resolved and the plural when more than one did, exactly as Archive does. `dialog.attachment.delete.action` is the affirmative label in both, unchanged. AC-30 checks all of it, including that the count is the resolved one.

The four singular keys the round-6 plurals stand beside are unchanged and stay in the catalogue, because each is still what a single-row gesture shows. A command chooses between the pair on the same count it chooses everything else on: `resolved.length` inside `ask`, and the number of rows that finished inside `successMessage`.

The English of the ten not quoted above:

```
dialog.bulk.allSucceeded
  "Dinah finished all {count} selected row(s)."

dialog.bulk.allSucceededNotes
  "Dinah finished all {count} selected row(s) and wrote {notes} note(s) to
   its output channel."

dialog.bulk.partial
  "Dinah finished {succeeded} of {selected} selected row(s). It refused
   {refused} and could not act on {skipped}. See the Dinah output channel
   for details."

dialog.bulk.oneRowOnly
  "Dinah runs this command on one row at a time. Select a single row and try
   again."

dialog.move.noSharedDestination
  "Dinah found no destination that all {count} selected cards can move to."

dialog.move.placeholder.many
  "Move {count} cards to"

dialog.block.reasonPrompt.many
  "Why are these {count} cards blocked?"

dialog.card.copiedRef.many
  "Copied {count} card references."

dialog.workbench.copiedPath.many
  "Copied {count} workbench paths."

dialog.attachment.delete.confirm.many
  "Delete {count} attachment(s)? Dinah destroys the files and cannot bring
   them back."
```

These sentences mint no convention. Each of them follows one the catalogue already uses.

Plurals ride the `(s)` form the catalogue already uses. `dialog.workbench.checkFindings.toast` writes "{count} defect(s)" and `dialog.runVerb.excludedSeparator` writes "{count} command(s)". A sentence whose verb or article agrees with a number nobody can see is the counterexample "A word that agrees with an interpolated token the sentence never sees", and a translator into German or Hindi has no way to resolve the agreement at all. Four of the new keys keep an unpluralised `{count}` and read as plain plurals instead, because each is reached only when more than one row resolved: `dialog.move.noSharedDestination`, `dialog.move.placeholder.many`, `dialog.block.reasonPrompt.many`, and the two `copied...many` keys. Their context notes say so, so that a later reader does not correct them into the `(s)` form. `dialog.attachment.delete.confirm.many` takes the `(s)` form instead, because nothing stops a later card reusing it over one attachment, and `dialog.bulk.allSucceededNotes` takes it on both placeholders for the same reason.

`allSucceeded`'s placeholder is `{count}`, and §3b fills it from `done`. The rename is deliberate: `{selected}` invited the message to be filled from the selection, which is the defect case 7 exists to close.

`Dinah` or `you` stands in the subject slot, which is what the Prose standard's documentation section asks for. Two of the new keys are placeholders rather than sentences, `dialog.move.placeholder.many` among them, and they follow the existing `dialog.move.placeholder`, which is a fragment by the same convention that governs a quick pick's placeholder text. No sentence here opens with an abstraction, and none takes an article immediately before a placeholder. That last rule is followed rather than enforced on this surface: `TestNoEnglishSentenceTakesAnArticleBeforeAPlaceholder` lives in `internal/msg/msg_test.go` and sweeps the Go catalogue's `Keys()` and `BaseEntry`, and a search of the whole tree for that test name found one definition and no caller reading `editors/vscode/src/locales`. Nothing under `editors/vscode/test/` holds the rule either. The convention is met by the author here, and no guard on this surface would catch a later key that broke it.

One new manifest key, `manifest.command.dinah.tree.archiveCard.title`, in `package.nls.json` and its seven siblings. The thirteen runtime keys raise the runtime total by five against round 5 and leave the manifest total where it was; both figures live in AC-16 and nowhere else, and both were re-derived from the files rather than adjusted by hand when the five keys landed.

The contract every new key meets, read off the guards rather than restated from memory:

- Every key carries a `context` note in every file, and the note is the English one in all eight.
- `de` and `hi` carry real translations and a `source` fingerprint computed by `src/fingerprint.ts` over the English text. `af`, `cs`, `es`, `fil` and `id` carry the English with `skeleton: true` and no fingerprint.
- The placeholder set of each translation equals the English's, compared as a set.
- The glossary in `src/locales/flags.json` already rules "the archive" as `Archiv` in German and `अभिलेखागार` in Hindi. The archive confirmation's translations use those forms.
- `Dinah` stays in Latin script in every language.
- The new manifest key joins the `skeleton` list in `flags.json` for the five skeleton tags, and the `source` map for `de` and `hi`.
- The German is written so that it needs no quotation mark at all. The counterexample "A quotation mark a translator reaches for by reflex, in a repository that bans two of the four" records that U+201C, which is German's closing mark, is banned while U+201E is not, so the pair a German writer produces by reflex is half-refused. The document names the mixed pair (U+201E opening, ASCII `"` closing) as the fallback and says outright that a sentence needing no quotation mark is better than either spelling. None of the thirteen English sentences quotes anything, so the German has nothing to quote. Recorded so nobody credits a guard that is not here: searching this tree for U+201C and for a scan naming it found neither, so the ban is enforced by the pre-commit scan the counterexample describes and not by anything in `editors/vscode/test/`.

The totals the new keys produce are asserted in exactly one place, which is AC-16's assertion, and the two figures live there and nowhere else.

What this card does not do to the l10n suite: the population floors in `l10n-keys.test.ts`, `l10n.test.ts`, `l10n-coverage.test.ts`, `l10n-placeholders.test.ts` and `l10n-staleness.test.ts` are `> 0` guards over sweep populations, not per-population totals waiting to be raised. Those five files are the whole of the extension's l10n suite. This card changes none of them, and it adds one new assertion of its own: the catalogue-size check AC-16 mints, which holds `en.json`'s entry count and `package.nls.json`'s key count against literals. That is the count this card ships, and it is a real floor rather than a more-than-zero check. Turning the suite's existing `> 0` guards into computed floors is a separate piece of work over a suite this card barely touches, and doing it here would widen the card past its spec.

### 8. Drag and drop

Turning multi-select on makes a multi-row drag possible, and `dragPayloadFor` would move one of the rows. The reviewer confirmed both halves by tracing the call chain: `dragPayloadFor` reads `source[0]` and nothing else, `offerDrag` wraps that single payload, and `applyDropVerdict` moves that one card. That is the operator's ruling failing in the place it is most visible, so drag is in scope.

`src/dragAndDrop.ts` changes shape, and the change is what lets the drop path use `runBulk` like any other command:

```ts
/** One dragged row: a card that composes a payload, or a row that does not. */
export type DragRow =
	| {
			readonly kind: "card";
			readonly ref: string;
			readonly payload: DragPayload;
	  }
	| { readonly kind: "other"; readonly ref: string };

/** Every dragged row, in the order dragged, one entry per row. */
export function dragRowsFor(source: readonly TreeElement[]): readonly DragRow[];

/**
 * The dragged rows read back off a mime entry, or undefined when the value is
 * not one offerDrag put there.
 */
export function dragRowsFrom(value: unknown): readonly DragRow[] | undefined;

/** Acts on each dragged row against one resolved drop target. */
export async function applyDropVerdicts(
	rows: readonly DragRow[],
	drop: DropTarget | undefined,
	exe: string,
	host: CommandHost,
	spawner: Spawner,
): Promise<BulkReport>;
```

`dragRowsFor` answers one `DragRow` per dragged element rather than one payload per card, which is round 2's second blocker closed at the root. A non-card row carries its tree label as its `ref`, so the entry the report records for it names the row instead of being a synthetic hole, and the drag stops handing a row count back to a caller. `draggedRows` is gone, `selected` is `rows.length` again, and the drop path gets the same by-construction guarantee every other command has.

`applyDropVerdicts` calls `runBulk` over those rows. `resolve` keeps the `card` rows and answers `undefined` for the others. `act` classifies the payload against the target and runs the move for an `act` verdict, records a `crossWorkbench` verdict as failed with `crossWorkbenchMessage`, and records an `ignore` verdict as skipped. `deps.emptyRun` is `"silent"`, and `deps.host` is the real host, because `runBulk` does the wrapping: `crossWorkbench` reaches `host.showError` per payload today, and without the collecting host a five-card cross-workbench drop would show five toasts with a summary on top of them.

`classifyDrop` is unchanged, because it is already a pure function of one payload and one target and is already tested that way. It moves inside `applyDropVerdicts`, which is where `act` calls it, so `extension.ts` stops calling it; `dragPayloadFor` becomes an internal helper of `dragRowsFor` rather than an export the controller reaches for, and `applyDropVerdict`, the singular verb, is deleted. AC-29 asserts that none of the three is called from `extension.ts` after this card, because a diff that adds the new path and leaves the old one live passes every other criterion here.

**What crosses the drag boundary, which this spec previously did not say.** The two halves of a drag are two separate calls with no shared state, and the editor's own declarations are what bound the design. `TreeDragAndDropController.handleDrag` is declared `handleDrag?(source: readonly T[], dataTransfer: DataTransfer, token: CancellationToken)` with `source` documented as "The source items for the drag and drop operation". `handleDrop` is declared `handleDrop?(target: T | undefined, dataTransfer: DataTransfer, token: CancellationToken)`, and its parameters are documented as the target row and "The data transfer items of the source of the drag". There is no selection argument and no other channel, so whatever the drop path is to know about the gesture, `handleDrag` has to put in the data transfer.

What the data transfer promises is also declared rather than measured. `DataTransferItem.value` is documented as "Custom data stored on this item. You can use `value` to share data across operations. The original object can be retrieved so long as the extension that created the `DataTransferItem` runs in the same extension host", and `handleDrag`'s own text says "When the items are dropped on **another tree item** in **the same tree**, your `DataTransferItem` objects will be preserved." Both handlers are this extension's, in one host, on one tree, so the object handed to `new vscode.DataTransferItem(...)` is the object read back. Nothing here rests on a measurement or on an undocumented behaviour, and neither declaration restricts the value to a single record, so a list is as ordinary a value as a record.

**The decision: the mime entry carries every dragged row, non-card rows included.** `offerDrag`'s `wrap` callback is redeclared to take the whole list:

```ts
export function offerDrag<Item>(
	source: readonly TreeElement[],
	mime: string,
	sink: DragSink<Item>,
	wrap: (rows: readonly DragRow[]) => Item,
): void;
```

`offerDrag` calls `dragRowsFor(source)`, sets nothing when no row in the answer is of kind `card`, and otherwise sets the mime entry to `wrap` over the whole answer, `other` rows and all. `extension.ts`'s `handleDrag` is unchanged except that its wrapper now reads `(rows) => new vscode.DataTransferItem(rows)`.

The alternative was to let the mime entry go on carrying only the cards and to narrow what the report claims, so that a five-row drag reported a three-row run and said so in its own criterion. It is defensible and it was rejected, because it makes `selected` mean the targeted rows in every command on this card except one, and §3a exists to stop two routes counting differently. A reader who drags four cards and a column header and is told that four rows finished has been told the truth about the rows Dinah looked at and nothing about the row it did not, and that silent narrowing is the shape the operator ruled against. D-22 records the choice and this reasoning.

**The drop path reads the entry back through a function rather than a cast.** Today `handleDrop` writes `dataTransfer.get(DRAG_MIME_TYPE)?.value as DragPayload | undefined` and acts on whatever comes back. `DataTransferItem.value` is declared `any`, and nothing documents that the entry under this mime type is one `offerDrag` set: the recommended tree mime type is the editor's own, and what the editor does or does not put under it is not something this design may assume in either direction. A single record survived that cast because every field read off it was checked before use. A list does not, because the first thing the drop path does with it is iterate.

So `dragRowsFrom` is the reader, and it answers `undefined` for anything it does not recognise: a value that is not an array, an empty array, an element that is not an object, an element whose `ref` is not a string, an element whose `kind` is neither `"card"` nor `"other"`, a `card` element whose `payload` is not an object carrying `ref`, `root`, `folder` and `columnId` all as strings, or an array holding no `card` element at all. That is the conservative-miss discipline `dragPayloadFor` already applies to a row, moved to the boundary. `handleDrop` becomes:

```ts
handleDrop: async (target, dataTransfer) => {
	const rows = dragRowsFrom(dataTransfer.get(DRAG_MIME_TYPE)?.value);
	if (rows === undefined) {
		return;
	}
	await applyDropVerdicts(
		rows,
		dropColumnFor(target),
		binary.state === "ok" ? binary.path : "",
		host,
		nodeSpawner,
	);
},
```

AC-15 drives that round trip end to end rather than driving `applyDropVerdicts` with a list somebody typed: it calls `offerDrag` into a recording sink, takes the value the sink was handed, passes it through `dragRowsFrom`, and hands the answer to `applyDropVerdicts`. AC-29 closes the last link with an AST walk over `extension.ts`, asserting that `handleDrag` passes its own `source` parameter to `offerDrag` and that `handleDrop` passes the mime read to `dragRowsFrom` and that call's result to `applyDropVerdicts`. That is the shape AC-3 uses on the registration boundary, for the same reason: a criterion that drives a function the wiring never feeds proves nothing about the product.

§8 decides the following rather than leaving them to be discovered.

A shipped behaviour changes, and it changes on purpose. Today a drag whose first row is a column row sets no mime entry at all, and `offerDrag`'s own comment says so: the drop is then indistinguishable from one this controller never handled. Under `dragRowsFor` the same gesture sets the entry whenever any dragged row is a card, and the entry carries every dragged row, so a mixed drag becomes a drop this controller does handle and the drop can name the rows it could not act on. That is right, because a reader who selected several cards and a column header and dragged the lot meant the cards and is owed an account of the header. AC-15's first clause assumes it, and D-15 declares it. A drag holding no card row at all still sets nothing, and `dragRowsFrom` refuses a list holding no card row for the same reason, so the two halves of that rule cannot drift apart.

A drop that acted on nothing stays silent, whatever it skipped. This is the `emptyRun` declaration, and it is the one place where the drop path's reporting differs from a menu command's. A menu invocation is an explicit request, so a run that attempted nothing still answers; a drag is a gesture that can miss, and a gesture that missed has to go on looking like a gesture that missed. Round 2 found the gap this closes: a mixed drag onto the cards' own column skips every row, which under a menu command's rule would warn where today the reader sees nothing. Declaring it as a value on the report, set by `runBulk` from one caller, rather than as a special case in the drop path, is what keeps the summary the only route. AC-28 holds the declaration to one call site so that a later command cannot quietly borrow the silence.

Then:

- Every verdict is `ignore`, or every dragged row was of the wrong kind, or both. Nothing spawns and nothing is shown, which is today's behaviour for a drop back onto the card's own column and for a drop on a row naming no column.
- Otherwise each `act` runs its move, each `crossWorkbench` is a failed row whose channel line is `crossWorkbenchMessage`, each `ignore` and each non-card row is a skipped row, and `summaryFor` produces the one message.

The module header, the `dragRowsFor` doc comment and the `dragRowsFrom` doc comment replace the sentence at `src/dragAndDrop.ts:71`. `dragRowsFrom`'s own comment says what it refuses and why, because a reader meeting a function that answers `undefined` for a plausible-looking value needs the reason where the function is rather than in a card. What the replacement has to say is that the view selects many, that a drag therefore carries every selected row once any of them is a card, and that a drop moves each dragged card independently against the one target while recording the rows it could not move. What it must not say is anything about column rows being draggable for reordering, which stays false and stays out of scope. `offerDrag`'s own comment about a mixed drag setting no entry is replaced in the same edit, because this card makes it false.

### 9. What is not in this card

- No view of the archive, and no restore. dinah-491 carries both, at the operator's direction.
- Archive on a column row, on a workstream, or on anything below a card. The CLI reaches all of them, and nothing here is built in a way a later card would have to tear up: `archiveCard` takes one `CommandContext` rather than a card element, and §6 keeps the confirmation out of it, so a column command declares its own row resolution and its own `ask` and reuses the verb and the bulk layer unchanged. The tree draws no workstream row at all: `TreeElement` in `src/tree.ts` is `root`, `note`, `column`, `group`, `card`, `attachmentsGroup` and `attachment`.
- Reordering cards inside a column by dragging. The workbench records no ordering a drop could express, and `classifyDrop`'s own comment already says so.
- Any change to `dinah archive` itself. The verb takes one reference and goes on taking one reference.
- Any repair of the cosmetic claim-after-archive effect recorded at the top of this spec. It belongs to the CLI, it blocks nothing, and it is noted here so that a reader who meets it knows it is known.
- Any change to the `> 0` population floors in the existing l10n suite, for the reason §7 gives.
- Any defence against a report forged from a symbol read off a real one. §3a states that route and states why it stays open.
- Any change to the four `noRow` commands. They read no row, they keep their own `register` calls, and §1's table does not carry them.
- Any article guard over the extension catalogue. §7 records that the rule is met and that no guard on this surface enforces it; minting one is a card of its own and nobody has asked for one.
- Any validation of the mime entry beyond the shape `dragRowsFrom` checks. It answers whether the value is a `DragRow[]` this controller could have set; it does not and cannot answer whether this session set it, because nothing in the declared API carries that.
- Any rewording of a per-row sentence that already names its own row. §3a's sweep reached them and left them, because a note in the channel filled from its row is true whatever the run's size.

## Branch

dinah-490-multi-select-and-archive
