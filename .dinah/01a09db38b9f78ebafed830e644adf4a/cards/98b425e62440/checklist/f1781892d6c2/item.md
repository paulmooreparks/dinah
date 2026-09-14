---
kind: acceptance_criterion
state: verified
column: c9428b3bc921
ts: 2026-09-14T02:16:35Z
ordinal: 8
note: Covered by the same TestScanLangFlagReadsOnlyALangThatIsAFlag run (subtest "lang in card's value slot past the word that fails to parse"), passing.
---
A --lang sitting in a valued flag's value slot after the word that already fails to parse (dinah --nosuchflag --SOMEVALUEDFLAG --lang de) exits 2 and renders in English; scanLangFlag(argv) for that exact argv is asserted to return "" in the same unit test as the previous criterion, showing the value-slot rule holds on the far side of the failing word too. The valued flag is read out of valuedFlags at test time rather than named (D-5). Verified with t.Setenv("DINAH_LANG", "") and no configured lang.