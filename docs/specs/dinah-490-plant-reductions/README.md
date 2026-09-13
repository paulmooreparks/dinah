# dinah-490 plant reductions

These six files are spec artifacts rather than product code. Nothing in the
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
- `perimeter.mjs` reduces section 3a's perimeter and section 3b's
  `collectingHost` and `summaryFor`, over all three of the host shapes `src/`
  declares, and it carries the `finish` and `successMessage` fields the
  `oneCall` copy commands declare.
- `acPerimeter.mjs` drives `perimeter.mjs` through AC-32, AC-33, AC-35 and
  AC-36, which are the criteria round 6 minted for the one-message promise.

Run the baseline and each plant like this, from this directory:

```
node ac15.mjs
PLANT=A node ac15.mjs
PLANT=B node ac15.mjs
node ac13.mjs
PLANT=C node ac13.mjs
PLANT=D node ac13.mjs
node acPerimeter.mjs
PLANT=E node acPerimeter.mjs
PLANT=F node acPerimeter.mjs
PLANT=G node acPerimeter.mjs
PLANT=H node acPerimeter.mjs
```

Every assertion prints `green` or `RED` with the failure, and each script
prints how many assertions it ran and how many reddened, so a plant that
silently exercised nothing reads as a smaller sweep rather than as a pass. A
script that reddened any assertion exits non-zero, so a later round can run
these from a script and read the status rather than the output. Round 5 ran
them all by hand and found every one of them exiting zero on a red run, which
is the failure the scripts exist to prevent, so the count and the status now
travel together. What each plant is and which
assertion it is expected to redden is written in the note of the criterion it
belongs to, on the card.

Plant E is the one worth re-running first. It implements round 5's
`collectingHost`, which intercepts `showError` and nothing else, and its red
output is the defect round 5's review reported: three per-row toasts from a
three-workbench check, four from a five-column pull, each with the bulk summary
underneath. Plants F, G and H narrow the same promise from three other
directions. F omits `showWarning` from the intercepted set, so the
findings-toast case alone reddens. G hands `ask` the collecting host instead of
the real one, so a Move refusal is swallowed and the reader is told nothing at
all. H has the copy commands report per row instead of declaring `finish` and
`successMessage`, so one gesture produces three messages.

Plant B is the one worth re-running next. It implements round 4's section 8
faithfully, wrapping only the card rows into the mime entry, and it reddens
AC-15's round-trip clause on its own while leaving the `dragRowsFor` clause
green. That isolated red is the evidence that the criterion now sees the drag
boundary instead of driving around it.

Delete these files when dinah-490 merges if they have stopped earning their
place. Until then they are cheaper to keep than to rebuild, because every round
that has had to rebuild them has cost a review cycle.
