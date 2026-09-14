---
kind: acceptance_criterion
state: verified
column: c9428b3bc921
ts: 2026-09-14T02:16:35Z
ordinal: 6
note: "grep -n 'value(\"lang\")' over cmd/dinah and internal on merged tree: exactly one shipping-source match, commands.go:936 inside runConfig (LangFlag: parsed.value(\"lang\")); the only other match is args_test.go:261, a test asserting the two readings agree."
---
grep -n 'value("lang")' run over the whole repository tree (cmd/dinah and internal/**) returns exactly one match in shipping sources after the change, the call inside runConfig in cmd/dinah/commands.go. Any other shipping-source match means a second call site still depends on the pre-fix resolution and was missed. A match in a _test.go file is not such a call site, and it does not violate this criterion so long as the test asserts that the two readings agree rather than depending on the pre-fix one. Both greps, the whole-tree one and the shipping-source one, go in the PR description or a card comment beside this criterion's verification.