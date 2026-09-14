---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:18:47Z
ordinal: 14
note: "NOT PERFORMED. `dialog.block.reasonPrompt.many` does not exist yet, so the plant cannot be run against anything. The plant, for Test: raise `dialog.block.reasonPrompt` in both branches and watch the three-card clause redden while the one-card clause stays green. A plant that changes the prompt in both branches reddens both clauses and proves less, so aim it at the plural branch alone.\n\nThe prompt clause is new at round 6. Round 5's text pinned only the call count, and the shipped English reads \"Why is this card blocked?\", so a run over five cards asked the reader about one and blocked five. Comment 7560 finding 2 reported it.\n\nCompares the fake host's `input` call log, including the prompt string it was handed, and the spawn log, against expected values. The empty-reason half preserves the rule `blockCard` in src/cardCommands.ts already carries, which is that a blank reason records a block nobody can act on, and it is the refusing partner to the accepting case so that an implementation that never spawns cannot pass on the first half alone."
---
Block over three cards calls `input` exactly once, with the prompt `dialog.block.reasonPrompt.many` filled with 3, and every one of the three spawns carries the same trimmed reason as its final argument. Block over one card calls `input` once with `dialog.block.reasonPrompt`, which is today's behaviour unchanged. A reason that is empty or whitespace after trimming spawns zero times.