---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:18:09Z
ordinal: 4
note: "The guard already fails in both directions, so nothing new is asserted here beyond keeping it green with two more commands in the roster; the criterion exists because the guide's table is the roster's only published statement and dinah-457 built the guard for exactly this drift. Plant that reddens it: add the `set` row to the guide and leave `get` out. The run reports that get points at the references guide and the guide's table carries no row for it, and the row count mismatch line names the roster. A false failure from rewrapped prose cannot happen, because the checks parse the table's rows and the ledger's derivations rather than matching quoted sentences."
---
The references guide's command table carries a row for `get` and for `set`, and every figure that counts a command roster still derives. `go test ./cmd/dinah/ -run 'TestTheReferencesGuideTableNamesEveryCommandThatTakesAReference|TestTheReferencesGuideNamesTheCommandsThatTakeAWorkstream|TestTheReferencesGuideDeclaresEveryCommandTheCollectionColumnIsAheadOf|TestEveryProseFigureIsDeclared|TestNoProseFigureEntryIsStale|TestEveryDerivedProseFigureMatchesTheBinary|TestEveryCountedSetIsCountedConsistently|TestEveryLedgerReferenceIsLive'` passes with `referenceProbeArgs` carrying an arm for each of the two new commands, with the guide's opening figure reading seventeen, and with the quick start's grouped-command figure agreeing with the `groupedCommands` derivation.