# Restructuring the storage layer before 1.0

This document records an analysis made on 2026-09-27, after the operator stopped work to ask whether Dinah's file-based storage is the right idea and, if it is, whether the implementation is off. It records what was measured, the answer, and the operator's ruling on what to do. The work it orders is the `storage` workstream on the development workbench, whose cards are listed at the end. Two reports the analysis rests on are attached to that workstream: a profile of the terminal UI and the view path, and an adversarial review briefed to make the strongest case against the current design.

## What was measured

The development workbench held 363 live cards in 9,262 files totalling 37 MB, with a median file of 592 bytes. Of those files, 5,059 were comments and 3,142 were checklist items, one directory each. Seventy-six Done cards held 5,237 of the files.

On the operator's machine with the installed build, v0.1.188-dev, `status` took 0.45 s and `show` 0.12 s, while `view board`, `next` and `prime` took 2.2 to 4.3 s. `prime` is the command every agent is told to run first.

Almost none of that time went to reading files. Two algorithms in the shared read code accounted for most of it. Sorting cards by arrival reads and parses each card's journal inside the sort comparison, so sorting 363 cards performs about 5,700 journal parses where 363 would do; that is 1.58 s of the board's 2.05 s draw, and the profiler measured a board draw of 0.43 s once each card's arrival was read once. The check that decides what the terminal UI's footer offers is quadratic in a card's checklist items and builds a full refusal for every act it declines and then discards; a card with 42 items costs 1.2 s per selection move. Two further costs compound them. Reads are serial, and sixteen goroutines cut the 234 ms file walk to 37 ms. `status` composes a full view for every card and prints only the held and blocked ones.

The disposable disk index proposed in dinah-622 would have touched none of this. It halves the cost of checking an unchanged file and leaves the number of reads alone.

## Is file-based storage the right idea?

It is, for what Dinah is. The properties that justify it are a record in plain text that a person or an agent can read without the tool, a command that needs no server to stand up, and an open contract with an independent reader in CI. All three hold, and all three are what let any harness drive the tool. A database store, which beads chose, wins on query speed at scale and on identity, but it trades away the plain-text record, and the process layer exists to produce that record.

Two claims in the pitch do not hold. Git-versionability is dead in practice, because the development workbench has been in `.gitignore` since 2026-09-14: every claim and move rewrote tracked files. The one-directory-per-entity layout was justified by clean merges of disjoint files, and nothing uses that property, since lock scope is the card. Auditability is partial. The journal audits acts, but current state is overwritten without history. On one card, agents rewrote the operator's open question three times and replaced the specification three times, and the journal records that it happened while the earlier texts exist nowhere.

## What is off in the implementation

| Problem | Evidence | Nature |
|---|---|---|
| Algorithms | The arrival sort, the quadratic offer, serial reads, and `status` building views it discards | Fixable in days, with no format change |
| Cold data | Seventy-six Done cards hold 5,237 of 9,262 files; ten cards have ever been archived; the change poll stats every archived journal every 500 ms | Fixable by archiving at Done |
| Granularity | 8,200 of 9,262 files are comments and items, one directory each | Structural; every cache must still stat them all, which is why an index saves only a quarter |
| Format churn | Storage format 1 to 11 in six weeks, 5 to 11 in twelve days, each a migration that breaks the editor extension | Fixable; one integer gates everything, and policy lives in the anchor files |
| Concurrency | A rename can fail under a concurrent reader with no retry; no flush before the rename; the capacity check counts every card under one card's lock; a dead agent's lock waits for a person | Fixable in days; latent today |
| No storage seam | Verbs pass directory paths into the store in 129 places | Structural for Dinah.Team, which runs on this library |

Granularity is the one design mistake among these. It was chosen for a property that is not used, and it caps every caching strategy.

## What closer to git means here

Git's storage layout has held for twenty years because its object model stores facts and no policy. Dinah's anchor files store policy, so every new rule becomes a format change that an old reader would misapply. Three git ideas transfer directly. Named capabilities, git's `extensions.*` keys, let a workbench declare a requirement only if it uses the feature, so an old build refuses only that workbench and the extension stops breaking on every bump. Parallel reads exist in git specifically for Windows. Packing cold data means an archived card becomes one file. Content addressing does not transfer, since identities are mutable and hand edits are allowed, and git's stat-cache heuristics are the undocumented behaviour this project's rule forbids.

Git also separates the working tree, which people read, from the object store, which the tool reads. Dinah uses the same files for both, which is why every claim dirtied git and why a raw worked card costs 40k to 143k tokens to read. A per-card projection rebuilt from the card's journal is the working-tree analogue.

## The ceiling

At the operator's minting rate of about nine cards a day, a single seat passes 1,000 cards in three months. With the algorithmic fixes, `status` stays near the 100 ms process floor to a few thousand live cards, and a whole-board read at 10,000 live cards would take 1 to 1.5 s, which is fine for agents and sluggish for the terminal UI. Archiving at Done keeps the live set in the hundreds. Ten thousand live cards is team scale and belongs to Dinah.Team by the operator's ruling of 2026-09-05, but that ruling protects nothing while Dinah.Team runs on this library and the library has no storage seam.

## The ruling

The operator ruled on 2026-09-27: continue, and restructure the storage layer before 1.0, in four steps. He agreed to all four as written.

1. First, with no format change: compute arrival once, read in parallel, stop building views `status` discards, fix the quadratic offer, and archive at Done by default. Add CI budgets for `view`, `next`, `prime` and the offer check, since only `show` and `status` carry them. The terminal UI's own reads leave its event loop.
2. Before 1.0, one format change and one migration: the card becomes the unit of storage, with comments and items as payloads in its append-only journal and `card.md` a rebuildable projection, taking 9,262 files to about 1,100. Named capabilities replace the integer format number. Attachment and item-text versions are kept rather than replaced in place.
3. Alongside Dinah.Team: a storage seam under the verbs, and one watched in-memory model for the long-lived heads, in the shape dinah-619 already has.
4. dinah-622, the disposable disk index, is dropped as the wrong tool for the problem.

Neither a rewrite nor a move to a database was chosen. The holds, the operator-reserved acts, the attribution, the contract and its independent reader, and about 170,000 lines of tests do not depend on the layout, and a rewrite would discard them to fix one layer.

## The cards

The `storage` workstream carries the work in the order below. The first seven need no format change and are worked first; the operator asked that the algorithms come first of all.

| Order | Card | What it delivers |
|---|---|---|
| 1 | dinah-630 | Arrival read once per card, not once per comparison |
| 2 | dinah-631 | The offer check linear in a card's items, with no discarded refusals |
| 3 | dinah-632 | Parallel reads across a workbench walk |
| 4 | dinah-620 | Reads that touch only the files their answer needs |
| 5 | dinah-634 | Archive at Done, so finished work leaves the live walk |
| 6 | dinah-635 | CI budgets for `view`, `next`, `prime` and the offer |
| 7 | dinah-636 | The terminal UI reads off its event loop and refreshes once per change |
| 8 | dinah-637 | The card as the unit of storage, a format change |
| 9 | dinah-638 | Named capabilities over a frozen core, in the same migration |
| 10 | dinah-639 | Attachment and item-text versions kept |
| 11 | dinah-640 | Concurrency: rename retry, flush, lock scope, dead locks |
| 12 | dinah-641 | A storage seam under the verbs |
| 13 | dinah-642 | One watched in-memory model for the long-lived heads |

Section 10 of the critical analysis called the upgrade path the roughest edge. This document names the mechanism behind that edge and the change that removes it.
