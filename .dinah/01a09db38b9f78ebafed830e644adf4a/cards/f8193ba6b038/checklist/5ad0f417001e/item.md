---
kind: decision
state: resolved
column: c9428b3bc921
owner: holder
ts: 2026-09-14T02:16:31Z
ordinal: 18
note: "Verified against the merged trunk rather than assumed: `grep -n 's.emit(' cmd/dinah/commands.go` finds emit's callers and `grep -n 'emitWorkstream(' cmd/dinah/commands.go` finds the two workstream acts, and the diff renames emitJSON and adds emitMachine without adding, removing or rerouting a call site. TestTheCompactFormReachesBothResponseCallSites in cmd/dinah/compact_test.go asserts the pair directly: it runs `--format compact claim fx-2` through emit and `--format compact workstream new Gamma` through emitWorkstream, and reads back the card and the workstream each wrote."
---
The eighteen commands the spec enumerates as reaching a *verb.Response through emit and emitWorkstream is still eighteen after this card, sixteen and two, and nothing in this diff changes which command reaches which call site.