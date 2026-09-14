---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:18:30Z
ordinal: 12
note: "Re-run: go test -run TestTheStoredHoldKeyReachesNothingAPersonReads passes and its own subject-per-surface loop is real (not vacuous: 5 surfaces captured, checked individually). Also verified live against built binary in a throwaway workbench: write/read/refusal/help text all use hold/on/off only. Swept the tree for gate_items outside test files/CORE-JSON-10 docs: all remaining hits are docs/design/format.md's interchange description and declaration section, and internal/bench source (bench.go, entity.go, interchange.go) which is never surfaced to a person. No leak."
---
The typed vocabulary is the one the operator approved on 2026-09-10 and nothing else. A person turns a column's hold on and off by naming the field `hold` and the values `on` and `off`, and reads it back by the same field name. The stored representation stays what it already is and is never what a person types or reads. Check by running the built binary against a scratch workbench: set the hold on by that wording and confirm it takes, read it back by that wording and confirm the answer uses the same words, set it off and confirm the same. Then read the command's own help text and confirm it offers those words rather than the stored ones. Failing case: the typed or displayed vocabulary differs from `hold`, `on` and `off` in any of the four places above, or the stored name surfaces anywhere a person reads.