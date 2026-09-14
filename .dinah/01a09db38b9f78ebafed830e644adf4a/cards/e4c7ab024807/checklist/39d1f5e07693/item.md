---
kind: decision
state: resolved
column: 4b38abe7ebd5
owner: holder
ts: 2026-09-14T02:17:49Z
ordinal: 17
note: "The spec spells `spawner: nodeSpawner` inline in contextForAttachment, and that shape makes three of the spec's own criteria unrunnable: AC-5, AC-6 and AC-8 all assert on the recorder's `calls` and `checkpoints`, and a command that composes its own context from an element leaves the test nothing to inject through. Written as specified, AC-5 failed with calls.length 0 and AC-8's ok arm failed with errors.length 1, because the real node spawner tried to run a binary called \"dinah\".\n\nThe reading taken is the one the repository already uses for exactly this shape. contextForAttach (creationCommands.ts:132-136) and contextForColumn (creationCommands.ts:50-55) each take `spawner: Spawner` as a fourth argument, and extension.ts:1078 and 1093 pass nodeSpawner at the call site. contextForAttachment now matches them, and deleteAttachment takes a spawner between its host and its log for the same reason. Every other name, argument and literal the spec mints is unchanged, and the argv the criterion pins is unchanged.\n\nThe alternative would have been to keep the signature and drop the three criteria's assertions to something a real spawn can satisfy, which trades a testable command for a spelling."
---
contextForAttachment takes its spawner as an argument rather than reaching for nodeSpawner, and deleteAttachment passes one through.