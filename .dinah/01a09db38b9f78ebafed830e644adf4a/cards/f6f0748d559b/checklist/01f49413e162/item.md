---
kind: decision
state: resolved
column: 0d86ad99cdbc
owner: holder
ts: 2026-09-14T02:17:29Z
ordinal: 29
note: "The conformance profile does not describe the version report's fields. `grep -n \"version\" docs/spec/core-profile.md` finds CORE-VER-1 and CORE-VER-2, which require a conformance claim to name a major and a minor number and to name no maturity channel, and nothing that fixes what else the report carries. `bench.ProfileMinor` therefore stays where it is and the changelog gains no entry, because no statement of the profile changed.\n\n`MINIMUM_PROFILE` in `editors/vscode/src/version.ts:35` stays at `dinah-core/0.4` for a stronger reason than convention. Raising it to demand a binary that reports `executable` would make every older binary unusable for the tree, the status bar and the diagnostics as well, which is a large refusal bought to avoid one small absence. The version gate's own doc comment already states the principle this rests on, that \"a client that only reads the fields it knows is unharmed by fields it does not\", and the converse holds here: a client that handles a field's absence is unharmed by a binary that does not send it. D-11 is where that absence is handled."
---
Neither the profile version nor the extension's `MINIMUM_PROFILE` moves for the new field.