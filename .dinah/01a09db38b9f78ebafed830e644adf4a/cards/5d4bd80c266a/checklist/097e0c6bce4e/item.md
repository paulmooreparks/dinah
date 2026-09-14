---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:17:45Z
ordinal: 2
note: "knownColumnKeys carries \"gate_items\": true; exportColumn emits the member only when the flag is set, in the block immediately after awaiting_outside's; writeColumnFromMember writes it back on import.\n\nCitation: internal/bench/gate_test.go, TestTheGateFlagRidesTheInterchange. It instantiates the two-column gatedDefinition, asserts the flag reads on the second column and not the first, asserts the export carries the member on the declaring column and omits it entirely on the other, then reads the export back through ReadDefinition and Instantiate and compares the second export to the first byte for byte.\n\nObserved: the byte-for-byte comparison is what would catch a field parsed and forgotten in exportColumn, and the omission assertion is what would catch a member written unconditionally.\n\nResolve with: go test ./internal/bench/ -run TestTheGateFlagRidesTheInterchange"
---
knownColumnKeys (interchange.go) includes "gate_items": true; exportColumn emits gate_items: true only when set, mirroring the existing operator_owned block. Check: an interchange_test.go case exports a column with GateItems==true and asserts the member is present and true; a case with it unset asserts the member is absent; an export-then-Instantiate round trip preserves the flag.