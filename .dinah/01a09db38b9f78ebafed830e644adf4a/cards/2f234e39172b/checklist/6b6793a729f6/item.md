---
kind: decision
state: resolved
column: 4b38abe7ebd5
owner: holder
ts: 2026-09-14T02:18:38Z
ordinal: 39
note: "The guard, and it was cheap: one table-driven test over `bench.HoldValues` in the file that already holds this field's cases, about eighty lines, no new machinery and no new dependency. The two readers that are unexported in `internal/verb` are reached through the command rather than by exporting anything, which is what keeps the cost down; the interchange half runs through `dinah init --from` for the same reason, and that is the path a person actually uses. The wording is corrected as well, in both copies, because the guard and the sentence answer different readers: the guard stops the next person shipping the half-done change, and the sentence stops them starting it believing there is nothing to do. Naming the seven readers in the doc comment also gives the guard something to be checked against by a person, since no test can assert that a prose list of call sites is complete."
---
Review offered the choice of correcting the wording or building the guard, and asked for the guard only if it is genuinely cheap. Which, and was it?