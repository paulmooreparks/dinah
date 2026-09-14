---
title: The rule saying which commands the client gets is wrong about help, in the code and on the page
column: 5ea2db0272fc
state: ready
severity: minor
priority: next
links:
  - kind: spawned_from
    to: 38d8c2b09eb7
---
## The defect

Dinah's MCP head states a rule about itself: a command that exists only because a shell and a filesystem exist gets no tool. The rule lives as a comment at `internal/mcp/tools.go`, around lines 60 to 65, and dinah-446 put a restatement of it on the published page in `docs/quick-start.md`, around line 1550, to replace a paragraph that had been counting things.

**The rule does not cover `help`.** `help` is exempted from the tool roster for a reason the rule does not explain, and the comment is wrong about it. The page inherits that imprecision faithfully, so both now say something not quite true, and they say it in the same words.

## How it was found, and why it has no card until now

dinah-446 corrected six false documented statements. Its code review raised this as a nit, and its implementer declined it for a reason both the reviewer and the parent session judged correct: fixing the page alone would leave the page and the code disagreeing, which is precisely the defect that card existed to remove, and that card's file scope did not reach `internal/mcp`.

Declining it was right. Leaving it untracked was not. The reviewer said so plainly in its round-2 comment: the defect is real, it ships in the binary, and nothing covered it. A defect acknowledged and then dropped is worse than one nobody noticed, because the noticing is the expensive part and it gets thrown away.

## What to do

**Repair the comment first, then the page.** That order matters and is the whole shape of the card. The page is a restatement of the comment, so fixing the page first would mean writing a sentence that disagrees with its own source, and the next person to reconcile them would have no way to tell which one was intended.

Establish what actually exempts `help` before writing either. The honest options are that the rule is right and `help` is a genuine exception that should be named as one, or that the rule is too narrow and should be stated in a form that covers `help` without special-casing it. Read the roster rather than reasoning from the comment, because the comment is the thing under suspicion.

## Scope

Two files: a comment in `internal/mcp/tools.go` and a sentence in `docs/quick-start.md`. No behaviour changes, and no change to which tools the head publishes.

**Do not extend the tool roster or remove an exemption.** This card corrects a description of what the roster does. If reading it turns up that an exemption is itself wrong, that is a separate card and a behaviour change on a published surface, which is not this.

## One thing worth carrying

dinah-446's standing rule was that a stale figure may not be repaired by transcribing a fresh one, because the figure moves. The equivalent here is that a rule with a known exception should either name the exception or be restated so it has none. Adding a parenthetical that lists today's exceptions recreates the problem dinah-446 spent two spec rounds removing from this exact paragraph, since a list of exceptions goes stale the same way a count does.

`docs/quick-start.md` is held to the tool by a guard, and dinah-446 added a check that reads two values out of `internal/bench` at test time rather than trusting the document. Whether this rule can be held the same way is worth asking, though it may not be: a rule is prose, and dinah-448 exists precisely because prose is what the guards cannot see.
