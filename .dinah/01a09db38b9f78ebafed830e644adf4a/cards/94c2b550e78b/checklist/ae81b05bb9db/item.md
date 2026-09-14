---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:18:31Z
ordinal: 3
note: Verified at 5a88bec. The assertion is `if len(roster) == 0 { t.Fatal("the library declares no command, so the roster this check compares against read nothing") }` at the head of TestNoGuideDeniesACommandTheToolHas. Armed by shadowing `verb.Commands()` with `[]string{}` at the call site; the run went red on exactly that sentence. Restored byte-identically and green.
---
The sweep fails when verb.Commands() is empty, so a universal claim over an empty roster cannot pass. The failure says the roster read nothing.