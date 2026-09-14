---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:18:18Z
ordinal: 11
note: "Verified at 7d50d5b. editDegenerateShapes emits Group C's seventeen rows with the verdict spec section 3's last column declares; the test asserts the group is non-empty, asserts by name that the empty-string shape is present, declared refusing and declared unknown-card, and holds every Group C shape to the same opens/refuses assertions. TestEditHandsTheEditorTheFileTheResolverNames runs runCLI(t, root, \"edit\") with no reference and requires exit 2 and no recorded launch. Armed by plant 4 (deleting arm 1): exactly the three whitespace shapes reddened at the resolver, each reporting it opened workbench.md, plus the no-argument case and the AC-5 empty-reference case at the command; every other shape stayed green."
---
The degenerate references are swept as a class, not as one repaired instance. `editReferenceShapes` emits a Group C carrying every row of spec section 3's degenerate table that is not already a Group A or Group B shape, each with the verdict that table's last column declares, and `TestEveryReferenceShapeEditAcceptsNamesAFile` asserts Group C is non-empty, asserts by name that the shape whose `ref` is the empty string is present and declared refusing, and holds every Group C shape to the same opens/refuses assertions the other groups get. `TestEditHandsTheEditorTheFileTheResolverNames` additionally runs `runCLI(t, root, "edit")` with no reference argument at all and requires the refused exit code and no recorded editor launch.