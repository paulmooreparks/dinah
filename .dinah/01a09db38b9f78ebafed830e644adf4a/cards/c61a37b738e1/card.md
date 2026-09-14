---
title: The unresolvable-item-column check message explains itself once per finding and wraps to five lines
column: b69abf918c42
state: ready
severity: minor
priority: now
workstreams:
  - 4fd7a9f0b8ff
  - a996e51c55fa
---
The operator ruled on 2026-09-11, after reading the rendered output, that this message should be cut down.

dinah-474 shipped a `dinah check` finding for a checklist item whose column can never hold a card. Its English text is:

    a checklist item names a column that will never hold a card, because it either resolves to no column at all or resolves to one whose identifier is not what is stored: {detail}

Followed by the item's file path, that runs to about five lines in an 80-column terminal, and it repeats in full for every stale item on the workbench. Three stale items print the same two-clause explanation three times. Nothing else in the check output behaves that way. Its nearest sibling, `check.unknown-column`, does the same job in one line:

    a card names column {detail}, which this workbench does not declare

The reasoning the long message carries is not lost by cutting it, because the key's own context field already says it: every checklist item's column value, where set, is stored as the identifier of a column this workbench declares, and either a write made before the field validated or a hand edit produces the finding.

What to ship. Replace the text of `check.item-column-unresolved` in every locale catalog with:

    a checklist item names a column that will never hold a card: {detail}

The detail stays last, which is the house rule Code Review established on dinah-474: a one-token detail may sit mid-sentence, and every multi-token detail in the catalog goes last without exception. This message's detail carries three tokens, the card's reference, the item's identifier and the stored column value, so it stays at the end.

The context field is not changed. No key is added or removed, so the catalog totals and the quick-start guard do not move. German and Hindi need re-fingerprinting, and the remaining skeletons follow the new English.

Check whether any test, document or acceptance criterion quotes the old sentence, and correct every one of them rather than only the sites a reviewer names. Note also that dinah-474's own spec prose already quotes a superseded wording of this same message, which was recorded rather than corrected because no stage below the operator holds a tool that writes spec prose. That is now two supersessions on one string, so say plainly in the handoff which store you changed.

Verify by running the built tool against a workbench carrying more than one stale item and paste what an 80-column terminal shows, because the whole point of the change is how it reads on screen.

The operator's standing constraint applies to the wording: Dinah must stay usable for workbenches with no code, no merge and no tests, so the sentence has to make sense to somebody running a workbench that has nothing to do with software.

## Branch

dinah-481-the-unresolvable-item-column-check-message-explains-itself-once-per-finding-and-wraps-to-five-lines
