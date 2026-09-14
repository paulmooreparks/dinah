---
kind: decision
state: resolved
owner: holder
ts: 2026-09-14T02:16:35Z
ordinal: 13
note: "Pre-scan. docs/design/format.md:1129-1130 already documents the ladder as first-hit-wins with no mention of scan position, so the fix brings the implementation in line with what is already written rather than adding a new promise. A flag is not ordinarily positional, and a scan that stopped before reaching --lang cannot honestly report what a full read of the line would have said. The alternative would put on record that two callers making the same typo in different word order get answered in different languages, which this spec declines to codify. Cost: one extra pass over argv already held in memory, and one resolved sub-question, that an incomplete --lang found after the failing word (no value follows it) is silently treated as absent, falling through to the next rung exactly as it does today, since the primary refusal already names the real mistake and does not owe a second opinion about a flag that never got a value either way. No other flag reaches this defect: --workbench is read before the parse-error check but never consulted when composing a parse-time refusal, --json is forced false on this path regardless of its resolved value, and --actor is only resolved when parsing succeeds, so none of the three change what gets printed on the path this card is about."
---
Should the flag rung be made order-independent (pre-scan argv for --lang), or should the current position-dependent behaviour be pinned down as a contract with a test?