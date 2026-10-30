# The bus stops reading the window's settings

## Status and scope

This proposal covers phase 3 step 2 of the cleanup plan. It was written by Dinah Engine Cleanup on 29 October 2026 for Architecture's review, and no code for it exists yet. The change is Go only, and the page, the browser suites and the exported App signatures are untouched.

## What reads settings.json today

Two shared-package readers look inside the window's settings.json, and a survey of every Go file found no others.

1. `Bus.PersonName` in `internal/bus/person.go` reads `personName` from settings.json in the bus's own folder. Its callers are the bus's text for agents, the hooks, the MCP server and the web server, which run in processes of their own. The window does not use it, because it passes the name it already holds (`agentPerson`).
2. `workspaces.Load` in `internal/workspaces/load.go` reads `workspaceRecords` from settings.json. Its only caller is the command line, through `loadWorkspaces` in `cmd/dinah/workspace.go`. It deliberately never writes, so a save the window left pending is not finished by the reader.

The window's own settings folder and the bus's folder are the same folder, because `configDir` returns `bus.Home`. That is why the contract has been invisible: nothing in the code says the two programs share a file, and the window can rename a field without any compiler complaint.

Three tests pin the current contract and will change with it. They are `internal/bus/person_test.go`, the registry fixture at `internal/workspaces/export_test.go:279`, and `cmd/dinah/workspace_test.go`, which asserts that settings.json is byte-for-byte unchanged after a read.

The window writes both values through `updateSettings`. The name is set by `SetPersonName`, which keeps its validation (80 characters, no control characters). The registry is changed by the workspace identity, sources, pins and export code, about 100 references to `Settings.WorkspaceRecords` across 22 files.

## Where the two values should live

The two values have different shapes, so I propose different homes for them.

The person's name goes into `bus.db`, in a new table with one row. Every process that needs the name already opens the database, SQLite supplies the concurrency, and the bus is where the person's handle (`bus.Human`) already lives. The table is created with `CREATE TABLE IF NOT EXISTS` in the same place as the others, so a build that does not know about it ignores it. A row that is present with an empty name means the person cleared the name, and an absent row means it was never set. The migration below depends on telling those apart.

The workspace registry goes into a new file, `workspaces.json`, in the same folder, owned by `internal/workspaces`. A registry is a document with its own record type and its own extra-field preservation, and the command line reads it without otherwise touching the bus. Putting it in `bus.db` would make every `dinah workspace` command open the database, which creates and upgrades it when it is missing, and that is a side effect a read should not have. The package gets two functions, `Load(home)` and `Save(home, records)`. `Save` goes through `statefile.Update` and `Load` through `statefile.ReadShared`, which is the path the statefile documentation prescribes for a file two programs touch. The command line therefore finishes a replacement the window left pending, where today's reader refuses to. That is the documented behaviour of the package for a shared file, and the test that asserts settings.json is unchanged moves to asserting that `workspaces.json` is unchanged when nothing was pending.

If Architecture would rather keep the two values symmetric, the name can go in a small file instead. It costs one more shared-file protocol for one string, which is why I do not recommend it.

## Who writes

The window is the only writer of both. It writes the name through a new `Bus.SetPersonName` and the registry through `workspaces.Save`. The command line, the hooks, the MCP server and the web server only read. Because there is a single writer, no revision check is needed beyond the crash safety statefile already gives.

## What changes inside the window

The window keeps `WorkspaceRecords` on the in-memory `Settings`, so the 100 call sites in the identity, sources, pins and export code do not change in this step. Only persistence changes, which keeps the diff reviewable. `Settings.PersonName` is removed, and the window reads the name from the bus when it builds its state and its headings.

There is one trap in this approach. `Settings.clone` makes its deep copy by marshalling the settings to JSON and back, and its comment says that this is the form they are saved in. If the registry were simply tagged `json:"-"` to keep it out of settings.json, `clone` would silently drop every record, and the next `updateSettings` would save an empty registry. The proposal therefore separates the two forms. `loadSettings` and `saveSettings` convert between `Settings` and the two documents, and `clone` becomes a real deep copy that uses the existing `workspaces.Copy` for the records. A new test sets every field of `Settings` to a non-zero value through reflection, clones it, and fails if any field differs, so a field added later cannot be dropped by a copy.

The two files cannot be replaced atomically together. `updateSettings` writes `workspaces.json` first and only when the registry changed, then settings.json, and the in-memory copy is replaced only after both succeed, as it is today. A crash between the two writes can leave a registry that is newer than the settings. The settings fields that refer to a workspace are `Workspace`, `Workspaces`, `Recent` and the per-workspace maps. I have not yet read every place that validates them against the registry, so I do not claim the window already tolerates a reference to a workspace that is gone. The implementation reads that code first, and the rule I intend to apply is that loading repairs a dangling reference by dropping it and never fails because of one. A test injects a failure between the two writes, reopens the window on the result and checks that it starts with the registry intact. If the existing validation cannot support that rule without a change to behaviour, I will stop and report it before going on.

## Migrating an older settings file

The first start of the new window migrates once, and it is safe to repeat.

1. If `bus.db` has no name row and settings.json has a non-empty `personName`, the window stores the name in the bus. If a name row exists, even an empty one, the row wins and the settings value is ignored.
2. If `workspaces.json` does not exist and settings.json has `workspaceRecords`, the window writes `workspaces.json` from them. If the file exists, it wins and the settings value is ignored.
3. Only then does the window save settings.json without the two fields. A crash between steps leaves the old fields in place beside the new stores, and the next start finds the stores present, ignores the old values and strips them on its save.
4. Before the first rewrite, the window copies the old file once to `settings.json.before-split`. Nothing reads that copy. It exists because an older window started after the migration would find no records in settings.json and could mint fresh workspace identities from the legacy folder list, orphaning the state that is keyed by the identities it replaced. The copy lets the person put the file back. It can be deleted in a later release.

The command line and the other bus processes do not read settings.json after this change, and they have no fallback to it, because a fallback would keep the contract alive. Until the new window has started once, a freshly installed command line sees an empty registry, and agent text calls the person "the person". The window starts first in every normal use, since it is what starts the agents, so I accept that.

## Tests

- The name round trips through the bus, a cleared name differs from a never-set name, and an empty or unreadable database gives the fallback.
- `workspaces.Load` and `Save` round trip, preserve extra fields, finish a pending replacement, and report a clear error for a corrupt file.
- The command line lists workspaces from `workspaces.json`, and a read changes no bytes when nothing is pending.
- The migration runs on a settings file with both fields, on one with only the name, on one with neither and on one already migrated, and in each case a second run changes nothing. A name cleared before the migration stays cleared.
- Migration with a crash after step 2 and before step 3 starts cleanly on the next run.
- The clone test described above, and the failure-injection test for the two writes.

## Review sequence

1. The first commit moves the name into the bus, with its migration step, `SetPersonName` and the removal of `Settings.PersonName`. It is small and independent.
2. The second commit adds the registry store, the new reader for the command line, the persistence split with the clone test, the registry migration and the backup copy.

Each commit runs the full gates, which are focused tests, the full Go suite, vet, vet with the live tag, the production build and the full browser run, and each returns to Architecture for review. Nothing is installed or pushed.

## Questions for Architecture

1. Is the split acceptable, with the name in `bus.db` and the registry in `workspaces.json`, or should both be files?
2. Is the one-time `settings.json.before-split` copy wanted, or is returning to an older window not a case to support?
3. May a command line or hook that runs before the new window has migrated see an empty registry and the fallback name, with no read of settings.json as a fallback?

## Ruling

Architecture approved this design on 29 October 2026 (message 483), with these answers and one addition.

1. The split is accepted. The name belongs in `bus.db`, since every process that reads it already opens the database. The registry belongs in `workspaces.json`, so that a command-line read never creates or upgrades the database.
2. The one-time `settings.json.before-split` copy is wanted. It costs one file and protects against a migration defect as well as a return to an older build. Running an older window afterwards is still not a supported path, because that window would not read the copy. The copy exists so the person can restore by hand, and the design says so.
3. Yes, with one condition. A process that runs before the window has migrated must be able to tell "no registry file yet" from "a registry with no workspaces", and the command line must say that the registry has not been migrated rather than report that there are no workspaces. The fallback name is acceptable for hooks. No shared package reads settings.json, even as a fallback.
4. The order of the two writes is chosen so that a failure between them is harmless. Adding a workspace writes `workspaces.json` before `settings.json` refers to it. Removing one drops the references from `settings.json` before the record leaves `workspaces.json`. A failure between the two then leaves only an unreferenced record, which loading tolerates, and the injected-failure test covers both directions. If an existing check rejects an unreferenced record, stop and report it.

The name commit comes first, then the registry, each with full gates and its own review.
