---
title: Done
slug: done
kind: done
operator_owned: false
---
The terminal column. A card here has completed its work, its commit is on `origin/main`, and it was accepted there without incident.

### Discipline

- **No work happens here.** Cards in Done are read-only by convention; the timeline is the audit trail.
- **No re-opening.** If a Done card needs revisiting (regression, follow-on, scope expansion), file a NEW card with `link_cards kind=spawned_from` pointing at the Done card. Don't drag the Done card back into flow.
- **Comments after Done are allowed but rare.** Cross-references and follow-on links are fine; substantive new content belongs on the spawned-from card, not here.

### Reporting

`/fast-track` and `/work` call `report_spend` on the Done transition so the per-card token / cost ledger captures the work. That call is best-effort; the card is Done regardless of whether the spend report succeeded.

### What being Done means

The card's spec is satisfied, its acceptance criteria are verified, its commit is on `origin/main`, and the change has been exercised without incident. Merge landed it and Acceptance watched it.

Done therefore means as far as this board currently ships. A card is Done when it has passed every gate the board has, whatever those are at the time.
