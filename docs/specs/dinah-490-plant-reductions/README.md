# dinah-490 plant reductions

These four files are spec artifacts rather than product code. Nothing in the
extension imports them, and nothing in the test suites runs them.

They exist because six arming recipes on dinah-490 were found vacuous or
uncompilable across rounds 3 and 4, and the cause was structural rather than
careless: a recipe prescribed for code that does not exist yet is a guess
unless somebody builds enough of the thing to perform it. The Agent Design
Review's round 4 ruling was that every plant a spec prescribes must either be
performed and reported as performed, or marked plainly as unverified.

Each file is a reduction of the shapes the card's spec declares, small enough
to run under plain node and faithful enough that a plant behaves the way it
would behave against the real modules.

- `drag.mjs` reduces the spec's section 8: `dragRowsFor`, `offerDrag` with its
  redeclared wrap callback, and `dragRowsFrom`.
- `ac15.mjs` drives `drag.mjs` through AC-15's four clauses.
- `move.mjs` reduces `runBulk` from section 3a, the cases of `summaryFor` from
  section 3b that the Move branches reach, and Move's own `ask` from section 5.
- `ac13.mjs` drives `move.mjs` through AC-13's two new clauses.

Run the baseline and each plant like this, from this directory:

```
node ac15.mjs
PLANT=A node ac15.mjs
PLANT=B node ac15.mjs
node ac13.mjs
PLANT=C node ac13.mjs
PLANT=D node ac13.mjs
```

Every assertion prints `green` or `RED` with the failure, and each script
prints how many assertions it ran, so a plant that silently exercised nothing
reads as a smaller sweep rather than as a pass. What each plant is and which
assertion it is expected to redden is written in the note of the criterion it
belongs to, on the card.

Plant B is the one worth re-running first. It implements round 4's section 8
faithfully, wrapping only the card rows into the mime entry, and it reddens
AC-15's round-trip clause on its own while leaving the `dragRowsFor` clause
green. That isolated red is the evidence that the criterion now sees the drag
boundary instead of driving around it.

Delete these files when dinah-490 merges if they have stopped earning their
place. Until then they are cheaper to keep than to rebuild, because every round
that has had to rebuild them has cost a review cycle.
