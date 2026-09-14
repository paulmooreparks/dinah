---
kind: open_question
state: resolved
column: c9428b3bc921
owner: operator
ts: 2026-09-14T02:16:58Z
ordinal: 16
note: D
---
The new guard denies any command run with cwd at the main checkout that matches none of the three allowed forms (AC-2 through AC-4), with no exception for Paul's own interactive session working there directly, because no signal available in this repository or in the PreToolUse payload is established as reliably telling that session apart from a dispatched agent's. Three ways to close this, and a fourth is welcome: (A) the guard applies to every session alike, and Paul disables it locally through /hooks for the stretches where he works the checkout directly; (B) the guard exempts his own interactive sessions on a signal he sets himself in his shell profile, understanding that this card has not verified, and cannot verify, whether such a signal reaches or fails to reach a dispatched subagent's own shell calls, since that depends on Claude Code's own environment-inheritance behavior between an orchestrating session and the subagents it dispatches, which is not established as documented; (C) a narrower version of (B) tied to a specific already-documented Claude Code mechanism, if one exists that this card has not found; (D) a different shape entirely. Which one, and if (B) or (C), what signal?