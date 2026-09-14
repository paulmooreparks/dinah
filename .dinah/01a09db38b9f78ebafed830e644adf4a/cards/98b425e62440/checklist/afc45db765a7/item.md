---
kind: acceptance_criterion
state: verified
column: c9428b3bc921
ts: 2026-09-14T02:16:35Z
ordinal: 7
note: go test ./cmd/dinah/ -run TestScanLangFlagReadsOnlyALangThatIsAFlag passes all 10 subtests, covering value-slot cases against two distinct valued flags picked dynamically by exampleValuedFlags.
---
A table-driven unit test in cmd/dinah/args_test.go, alongside TestParseArgsHonorsTheEndOfOptionsMarker, asserts scanLangFlag(argv) directly (no CLI dispatch, no fixture bench) against a table including at minimum: a --lang standing in a valued flag's value slot, wanting "" (that --lang is the other flag's value, not a language choice); the same valued flag given a normal value with an ordinary --lang after it, wanting that language (an unambiguous --lang still reads correctly once a filled value slot precedes it); and the value-slot case again against a second, different valued flag, showing the fix is general rather than special-cased to one flag. The example flags are read out of valuedFlags at test time by exampleValuedFlags rather than spelled in the table, so a rename cannot leave a case naming a word the parser no longer knows, which is a case that still compiles and still passes while the scenario it exercises has stopped happening (D-5). This is the direct regression test for the review blocker and fails against the previous scanLangFlag, which pattern-matched --lang blind to position.