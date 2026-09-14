---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:18:46Z
ordinal: 2
note: "Compares two arrays read from src/identity.ts and src/selection.ts at test time. A command contributed without a declared policy fails as missing; a policy for a command nobody contributes fails as unexpected. The roster literal in the criterion's own text is the only copy of that figure on this card: the spec prose names this assertion rather than quoting the number, so there is nothing for it to disagree with. It is the second half of the same doubling test/unit/manifest.test.ts already uses for its string corpus: the set comparison catches a divergence, and the literal catches a command silently removed from both at once, which the set comparison would read as clean. A future card adding a command edits it, and that is the intended cost. The chain is not vacuous: manifest.test.ts holds `contributes.commands` deepEqual TREE_COMMANDS, so the manifest, the roster and this table are three independent literals and no link compares a value against the thing that produced it."
---
A unit test asserts that the key set of `SELECTION_POLICIES` and the members of `TREE_COMMANDS` are the same set, reporting each direction separately, and asserts `TREE_COMMANDS.length === 23`.