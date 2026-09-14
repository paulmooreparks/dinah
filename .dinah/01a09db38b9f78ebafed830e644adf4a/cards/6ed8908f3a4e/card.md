---
title: The extension and the tier system are invisible from the documentation
column: 5ea2db0272fc
state: ready
severity: minor
priority: next
links:
  - kind: relates_to
    to: 38d8c2b09eb7
---
## What this card fixes

The same audit that found six false statements (dinah-446) found three places where the documentation is accurate and silent about something that exists. Nothing here is wrong; it is missing, which is why it is a separate card with a different kind of work. Correcting a false sentence is mechanical. Deciding what a page should cover is judgement.

## The three gaps

**The README never mentions the VS Code extension.** `README.md:9` uses `editors/vscode/media/icon.png` as the page's illustration, so the extension supplies the artwork and gets no sentence. The extension has its own README, a walkthrough, a marketplace release pipeline documented in `docs/release.md:135-163`, and 21 contributed commands. The only link out of the README into this repository's own documentation is the quick start. A reader arriving at the project learns nothing about an editor integration that four cards built this month.

**The extension's own README advertises nine capabilities out of twenty-one commands.** `editors/vscode/README.md:15-20` is headed "What it gives you" and reads as exhaustive. Since it was written the extension gained New Card, Attach File, Pull, Open Attachment, Open History (dinah-422), Edit Workbench Definition, Edit Column Instructions, Copy Card Ref, a welcome view when no workbench is found (dinah-423), a status bar showing the held card and its remaining claim (dinah-419), check findings in the Problems panel (dinah-421), and Run Verb, which reaches every verb from the command palette generated from what the tool reports (dinah-420).

**The tier system appears in none of the eight embedded guides.** `raise` is a command and an MCP tool, `--tier` is an argument on `claim`, `pull` and `next`, and `dinah.below-tier` is a refusal that appears as check 10 on `dinah help claim`. `docs/design/format.md` mentions tier 44 times and the guides mention it zero times. `verbs.md` is titled "The five verbs" and explains why `pull` is not a sixth, which is exactly where a reader will look for `raise` and not find it. `mcp.md` never mentions it either, **so an agent over MCP meets a refusal it has no guide to explain**, which is the sharpest of the three.

## What to decide rather than assume

**A list that reads as exhaustive is a liability, and the extension README has already proved it.** Nine of twenty-one went stale because the list was a list. The fix is not necessarily a longer list; consider whether the section should describe what the extension is for and let the command palette be the inventory, since dinah-420 made the palette generate itself from what the tool reports. A page that cannot go stale beats a page that is currently correct.

**How much the README should say about the extension is a judgement call.** The README is deliberately short and mostly evergreen, and it earns that by not enumerating things. A paragraph and a link is probably right; a feature list would inherit the problem above.

**Where the tier material belongs is the real question on this card.** Adding it to `verbs.md` invites renaming a guide titled "The five verbs", which is a contract-flavoured decision rather than a documentation one. Adding it to `mcp.md` serves the agent that meets the refusal. Adding a guide of its own costs a guide. Rule on it in the spec rather than at the bench, and note that guides ship inside the binary, so a new one is not free.

## Scope

Documentation only. No behaviour changes.

This card does not correct any false statement; those are dinah-446 and must not be absorbed here. It does not touch the guard blind spots either, which are their own card.

One inherited rule from dinah-446 applies here too, because the same trap is available: **do not write a count into prose that nothing checks.** Twenty-one commands is true today and will not stay true, and the failure mode this card exists to fix is precisely a number that was accurate when written.
