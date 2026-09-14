---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:18:09Z
ordinal: 3
note: "This is dinah-456 D-17 implemented as written, with no arm requiring every ground to be used, because retiring the last member of a ground is legitimate. Plant that reddens it: change the `config` entry's ground to `\"machine-not-workbench \"` with a trailing space, or to `\"because I said so\"`. The package compiles and the run reports that config is exempted on a ground the closed set does not declare. A false failure cannot come from a reworded reason, which is the point of reading a declared field rather than grepping the prose: the earlier draft's test, which searched the reason for \"shell\", would have failed `init` on a rewording and passed `config` on one."
---
Every MCP tool exemption declares a ground from the closed set, and the roster keeps both directions it already holds. `internal/mcp/roster_test.go`, `TestEveryLibraryCommandIsServedOrExempted` gains one arm asserting that each entry of `toolExemptions` carries a `ground` that is one of `toolGrounds`, and its existing empty-reason arm reads `entry.reason` while a new arm fails an empty ground. The run is fatal when `toolExemptions` is empty. In the same package, `get_field` and `set_field` appear in `tools`, each dispatching a command `verb.Commands()` defines, and no tool dispatches `card`.