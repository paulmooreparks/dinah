---
title: A recorded count is taken before the last edit, so it is wrong by exactly what that edit added
column: 5ea2db0272fc
state: ready
severity: minor
priority: next
workstreams:
  - 4fd7a9f0b8ff
---
An agent finishing a card counts something, writes the number into a criterion's note or a spec paragraph, and then makes one more edit. The number is now wrong by exactly what that edit added, and nothing anywhere notices.

**It has happened three rounds running on dinah-367, each time by one.** The most recent instance says a sweep found nineteen sites where the tree holds twenty, and the extra is the assertion that round's own final commit introduced. The gating number was right every time, so nothing shipped broken and nothing was blocked. The habit survived three reviews because each instance looks like a typo rather than a pattern.

**The same shape has already cost a real round elsewhere on this board.** dinah-367's second round ran a completeness sweep for assertions a wrong answer satisfies by containment, found three, wrote a criterion claiming every such assertion was closed, and added a fourth site in the same commit as the sweep. A reviewer found it by planting a wrong value. When the sweep was re-run over the tree actually being handed off, it turned up two more, taking three to five. So the count being stale is the harmless face of it, and the completeness claim being stale is the expensive one.

The conventions corpus already carries the expensive face, recorded as a completeness sweep being a snapshot that a commit which both sweeps and edits invalidates. This card is about the mechanism rather than the lesson, because writing the lesson down has not stopped it recurring.

**What to settle.** Whether anything can make the count and the tree agree at the moment a card is handed off, rather than at the moment somebody chose to count. Options worth weighing rather than assuming: computing the number at read time instead of recording it, having a guard recompute and compare, or dropping recorded tallies where the number is not the thing that gates and keeping only the claim that is.

Read what actually gates before proposing anything. On dinah-367 the criterion that gates says five and is correct, and the stale nineteen sits in a supporting note that no downstream stage reads as a contract. A fix that adds machinery to keep a decorative number honest has cost more than the number is worth, so establish which recorded numbers are load-bearing before deciding what to do about the rest.

The operator ruled on 2026-09-03 that this gets its own card rather than a fourth round on dinah-367, and sent that card back separately for its own instance.
