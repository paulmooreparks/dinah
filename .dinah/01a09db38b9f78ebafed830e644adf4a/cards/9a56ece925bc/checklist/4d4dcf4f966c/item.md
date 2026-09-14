---
kind: decision
state: resolved
column: 0d86ad99cdbc
owner: holder
ts: 2026-09-14T02:18:18Z
ordinal: 17
note: The existing guards point DINAH_EDITOR at a name no machine carries, which proves only that resolution did not refuse and is exactly why this defect survived them. Observing the argument needs a real editor process. A `.cmd` shim is what this spec's own reproduction used and it must not become a test, because Microsoft documents CreateProcess as requiring the command interpreter to run a batch file, so a guard resting on a `.cmd` launching directly would rest on undocumented behaviour. Re-executing `os.Args[0]` uses an executable Go already built, works on every platform the suite runs on, and needs five lines at the top of TestMain. The child exits before `m.Run`, so it never runs the suite and never trips the unreached-table-site sweep that follows it. `DINAH_TEST_EDITOR_LOG` stays out of `isolatedEnv` because that list names variables production code reads and clearing this one would break the mechanism; the reason is recorded beside the list, as the workbench requires either way.
---
The end-to-end guard records the editor's argument by re-executing the test binary, not by running a batch shim.