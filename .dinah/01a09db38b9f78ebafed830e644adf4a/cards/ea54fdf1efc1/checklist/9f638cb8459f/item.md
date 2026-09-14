---
kind: decision
state: resolved
column: 0d86ad99cdbc
ts: 2026-09-14T02:18:10Z
ordinal: 17
note: dinah-456 section 5.2 asks this card to state which refusal each retired spelling produces, per command, rather than assuming the three parents are alike. They are not. `workbench` and `workstream` each keep a bare listing and `workstream` keeps `new`, so both survive as commands and their unknown first word falls through to `s.fail(contract.Usage, first)`, which was probed at 9260a2a and prints `dinah.usage frobnicate was not understood; run dinah help for the list of commands`. `card` has `get` and `set` and nothing else, so a surviving `card` command would be a command with no act, and main's own dispatch answers a name the table does not define with `dinah.unknown-command Dinah offers no command called card`, probed at the same commit. Spec section 7.1 quotes both transcripts.
---
The `card` command is retired whole rather than left standing with no acts, so `dinah card get` and `dinah card set` refuse `dinah.unknown-command` while the workbench and workstream spellings refuse `dinah.usage`.