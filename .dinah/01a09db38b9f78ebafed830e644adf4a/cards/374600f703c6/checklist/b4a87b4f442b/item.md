---
kind: decision
state: resolved
column: 0d86ad99cdbc
owner: holder
ts: 2026-09-14T02:17:36Z
ordinal: 18
note: "Refusal is argued rather than assumed, because a migration that refuses too readily is its own problem. The two mistakes are asymmetric. Refusing a migration that could have proceeded costs a second run once a permission is fixed or a plain file is removed from where a directory belongs, and heldLocks' own doc comment already says this is a repair an operator runs deliberately. Proceeding on a lock nobody could see costs a rename performed under a live writer, whose members move out from under it while it writes. The same doc comment records that no lock exists in this format that the migration could carry across the rename of the directory holding it, so nothing weaker than refusal is available. The error travels unwrapped on dinah-433's D-4 reasoning: liftIntoContainer already lets os.MkdirAll and os.Rename failures reach cmd/dinah's reportError as plain errors, and contract.UnreadableContainer names a .dinah container rather than a workbench root."
---
The three heldLocks callers, remintInPlace, liftIntoContainer and finishContained, all refuse on a read failure rather than proceeding, and the raw error travels unwrapped with no new contract.Refusal name minted.