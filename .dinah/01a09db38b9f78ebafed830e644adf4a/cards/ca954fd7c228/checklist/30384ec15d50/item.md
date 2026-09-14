---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:17:24Z
ordinal: 15
note: "citation: scheme=test, target=internal/msg/msg_test.go#TestThePlaceholderComparisonReportsBothDirections, observed before=fail after=pass. comparePlaceholders takes its two catalogues as arguments rather than reading the package's loaded map, and the test drives it over three keys: one whose translation drops {ref}, one whose translation invents {ziel}, and one carrying {from} and {to} in a different order, asserted absent from both lists. Both lists are compared by content. Armed by making the invented branch report every pair, which went red with `got \"xx/fixture.invented: {card}, xx/fixture.invented: {ziel}, xx/fixture.reordered: {from}, ...\"`, and restoring returned it to green."
---
`internal/msg/msg_test.go` drives its placeholder comparison over two in-memory catalogues in a fixture test that asserts the invented list by content rather than by length: one key whose translation invents a name, one key whose translation drops one, and one key carrying the same two names in a different order asserted absent from both lists. The third case is the clean case, and without it a comparison reporting every pair would satisfy AC-11's armed plant. The comparison therefore takes its two catalogues as arguments rather than reading package globals, for `placeholderVerdict`'s reason: no shipped catalogue has to be allowed to carry a defect for the check to have an armed path.