---
title: The rule that one module alone reaches the editor is enforced by matching one line of text, so another spelling walks past it
column: 5ea2db0272fc
state: ready
severity: major
priority: soon
workstreams:
  - 58f3e3eb621a
---
Found on 2026-09-13 by dinah-490's code reviewer, who demonstrated it rather than reasoning about it. The defect is older than that card and already on trunk, and dinah-490 does not make it worse. What dinah-490 does is lean on it.

## The gap

The VS Code extension holds a rule that only one module may reach the editor's own API. That rule is enforced by matching a single exact line of text. A file that reaches the editor by a slightly different spelling walks straight past the check and the whole suite stays green.

The reviewer wrote such a file and watched it pass.

## Why it matters more now than it did last week

dinah-490 builds a message perimeter: every way a command can talk to a reader goes through one wrapper, so a run over several rows produces one message rather than one per row. Two of its criteria hold that perimeter by reading what one file binds, and they are sound **only while that file remains the sole module reaching the editor**. The design review named that explicitly as a change to refuse rather than a number to recompute.

So the rule that was a tidiness rule is now load-bearing for a user-visible promise, and the thing enforcing it can be stepped over by writing the same call a different way. Somebody adding a second module would not be stopped, and no count would move.

## The shape, which this board has now met repeatedly

A guard that matches text catches the spellings its author pictured. dinah-424 widened one twice by example and had it broken twice, and it was only settled by reading the construct as a syntax tree through the TypeScript compiler, after which fourteen spellings from three reviewers failed to escape. dinah-439 reached the same place from the Go side after five design rounds, by enumerating from the source every shape the repository actually uses rather than picturing them. The workbench instructions carry that rule now: parse the construct, or delete the guard and say plainly in the code what is not guarded.

This is the same defect in a third place, and the route out is already known.

## What this card owes

Either hold the rule by reading the construct rather than the line, which the extension's own test suite already does elsewhere and has a worked example of on trunk from dinah-406's catalogue sweep, or state plainly in the code that the rule is advisory and that the perimeter's guarantee rests on review rather than on a check.

The second is a legitimate answer. What is not legitimate is leaving a check that reports success while a file walks past it, which is the thing the operator's filing and guard rules both exist to prevent.

Whoever takes it should start by reproducing the reviewer's demonstration, since a fix nobody has watched fail is not a fix.
