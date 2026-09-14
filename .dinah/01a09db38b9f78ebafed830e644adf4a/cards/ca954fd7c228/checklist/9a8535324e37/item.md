---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:17:24Z
ordinal: 9
note: "citation: scheme=test, target=editors/vscode/test/unit/l10n-placeholders.test.ts#\"the verdict reports a dropped name, an invented one, and nothing for a reordering\", observed before=fail after=pass. placeholderVerdict is driven over two catalogues written in the test: fixture.dropped loses {ref}, fixture.invented gains {ziel}, and fixture.reordered carries {from} and {to} in a different order and repeated on the English side. Both lists are asserted by content, not by length, and the reordered key appears in neither. Armed by making the invented branch report every pair, which is the over-reporting failure the clean case exists to catch: the test went red naming the four names it should not have reported, and restoring returned it to green."
---
`placeholderVerdict` is driven over two in-memory catalogues in a fixture test that asserts `dropped` and `invented` by content rather than by length: one key drops a name, one key invents one, and one key with two placeholders in a different order is asserted absent from both lists. That third case is the clean case, and without it a verdict function reporting every pair would satisfy AC-6 and AC-7.