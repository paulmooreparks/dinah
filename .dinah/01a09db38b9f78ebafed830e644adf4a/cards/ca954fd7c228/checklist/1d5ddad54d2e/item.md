---
kind: decision
state: resolved
column: 0d86ad99cdbc
owner: holder
ts: 2026-09-14T02:17:24Z
ordinal: 16
note: "Four spellings are live at 65a80ad: bare `t` at 70 sites, `context.host.t` at 24, `host.t` at 4, `this.deps.t` at 1. A receiver whitelist that missed a fifth spelling would skip those call sites in silence, which is the failure this card exists to close, so the rule errs toward reading more rather than fewer. An unrelated function named `t` would have its argument reported as an unresolvable key expression, and the remedy is to rename it rather than to exempt it. Measured: 99 call sites matched, 0 of them non-localizer."
---
The sweep matches a localizer call by the callee's name alone and whitelists no receiver spelling.