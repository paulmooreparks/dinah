# Background work observation verification

A chat retains its busy state after its foreground turn when documented observations still report running work. The implementation is on `engine/hook-background-work`, based on `8affc2f61dd9d800c7f8d805338e4d8273d79196`.

## Implementation

Claude hook observations use the current native session and its registered chat binding. They count unmatched starts without the steering timeout, replace native child counting without guessing an identity correspondence, and preserve unfinished children at turn completion while hooks report work. The shared activity also prevents parking and automatic native closure and keeps exit and restart readiness busy.

Child-routed shell tasks contribute to the owning chat after the child finishes. Every task has a scoped opaque handle, including root tasks, so arbitrary provider IDs cannot collide across scopes. Stop resolves the handle back to its originating session and provider task ID.

Codex uses documented read-only terminal and descendant-thread inventories. Observation begins separately for each chat after replay finishes. Complete live reads settle work; failed reads and missing, unloaded or errored children retain the previous observation. The observer neither executes commands nor resumes children. Provider capabilities live in the registry, and shared code uses the observation interface.

The provider audit and primary references are in [Background Work](../design/background-work.md#provider-coverage-and-known-limits). The [Chats busy-count note](../design/notifications.md#the-chats-busy-count) records the same coverage boundary. The page implementation is unchanged, and the count has no existing tooltip or help text to extend. There is no stored-format migration.

## Verification

The implementation passed the following checks under the shared test lock. Local receipts are retained under `%TEMP%/dinah-hook-background-validation-20261126`.

| Check | Result | Receipt |
| --- | --- | --- |
| Focused Go regressions and provider-name guard | Passed | `focused-go.log` |
| Full Go gate, `pwsh scripts/go-gate.ps1 ./... -count=1` | Passed | `full-go.log` |
| Uncached verbose profile guard with the private terms file configured | Passed without skips or cached results | `profile.log` |
| `background-tasks`, `background-work`, `chats-busy-count`, `subagents` | All four passed in 19 seconds | `browser.log` and `browser/` |

The Go checks cover real hook-store observations across turn completion, stop clearing, session binding, observation beyond the steering timeout, automatic-close protection, child task survival, opaque-ID collisions and Stop routing. The app-server transport checks cover descendant ownership, pagination, idempotent observations, refused and malformed reads, conservative retention and host admission after replay.

An earlier full Go run failed the provider-name guard because shared code contained provider-name literals. The correction moved the hook capability into the provider registry and used the observer interface for other programs. The guard was unchanged, and the final full run passed.

The tests use invented records and a stand-in app-server transport. Native agent background execution and installed-window acceptance remain unverified. Integration, publication and installation remain pending.
