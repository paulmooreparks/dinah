---
kind: decision
state: resolved
column: 0d86ad99cdbc
owner: holder
ts: 2026-09-14T02:18:33Z
ordinal: 22
note: "Two of the five proving tests are in internal/bench and three are in cmd/dinah, and a test in one package cannot read a helper in another's test files. internal/bench/compattest is the established precedent on this repository: a package existing solely to be shared between test files in those same two packages, imported by no production file. guidepin follows it, takes no *testing.T so it never imports testing, and imports only internal/guide, which imports only internal/contract, so no cycle closes."
---
The pin helper lives in a new package, internal/guide/guidepin, rather than being duplicated in the two packages that call it.