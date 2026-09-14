---
title: The order the mcp refusals are checked in is declared in one place and implemented in another
column: 5ea2db0272fc
state: ready
severity: major
priority: soon
---
`beyondChecks["mcp"]` in internal/verb/checks.go declares that dinah.unknown-root is checked before dinah.outside-root, and `dinah help mcp` prints that order to the reader. Nothing at runtime reads the declaration. `runMCP` in cmd/dinah/commands.go hard-codes the order in Go, and the two are tied together by nothing but the fact that somebody wrote them the same way once.

Found on 2026-08-24 while retesting dinah-192. AC-20 exists to hold the declared order, and its own instructions say to arm it by swapping the two rows in `beyondChecks["mcp"]`. Doing exactly that changes no behaviour at all. The swap only relabels the help table, which then prints each refusal name against the other's sentence, while the binary goes on answering in the original order. The criterion's guard had to be armed by hoisting the check inside `runMCP` instead, which is not what the card said to do.

So the help page can drift from the runtime silently, and the drift shows up as a page that confidently tells the reader the wrong order. It also means a future reader who follows the card's arming instruction will conclude the guard is dead when it is not.

The card is the general question rather than the one instance. Should the declaration become the thing the runtime reads, or should a test hold the two in agreement? Every command carrying a `beyondChecks` entry has the same shape, so the answer is worth settling once.
