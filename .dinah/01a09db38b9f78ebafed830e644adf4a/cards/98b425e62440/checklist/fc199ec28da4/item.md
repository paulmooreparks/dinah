---
kind: acceptance_criterion
state: verified
column: c9428b3bc921
ts: 2026-09-14T02:16:34Z
ordinal: 1
note: "Re-verified on merged tree (branch 314bfd2, containing current origin/main ba1b940). Ran built binary directly: `dinah --lang de --nosuchflag` and `dinah --nosuchflag --lang de` with DINAH_EDITOR/EDITOR/VISUAL/COLUMNS/DINAH_LANG cleared, no config lang. Both exit 2, stderr byte-identical (md5 71af19791431c66b00fb09a035e8695b), both German. go test ./cmd/dinah/ -run TestLangFlagIsHonouredWhateverItsPosition passes all 5 subtests. Arming: reverted line 108 to trunk's parsed.value(\"lang\") reading, reran suite: exactly 2 subtests redden (behind-the-word, last-complete-wins) and 3 stay green (ahead, incomplete-falls-through, value-slot), matching the predicted arming. Restored clean."
---
dinah --lang de --nosuchflag and dinah --nosuchflag --lang de both exit 2 and render byte-identical stderr, each equal to contract.Usage + " " + de.T("refusal.dinah.usage", "detail", "--nosuchflag") + de.T("refusal.dinah.usage.next") + de.T("refusal.dinah.usage.dash-hint") + "\n" where de := msg.For("de"). Verified by a table-driven test, TestLangFlagIsHonouredWhateverItsPosition in cmd/dinah/main_test.go, run with t.Setenv("DINAH_LANG", "") so the environment rung cannot be the actual source of the German answer; TestMain already clears LC_ALL, LC_MESSAGES and LANG for the whole binary. This is the direct regression test for dinah-97 and fails today against unmodified main.go.