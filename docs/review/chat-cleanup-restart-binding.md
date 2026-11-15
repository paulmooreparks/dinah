# Cleanup restart observation

The tagged fixture in `cmd/dinah-desktop/chat_cleanup_restart_acceptance_test.go` supplies the missing provider observation requested by Quality. It is compiled only with `chatcleanup_acceptance` and changes no product code.

Call `chatCleanupAcceptanceCountProviders(t)` before constructing the restarted App or its provider lifecycle. The fixture replaces every registered provider module with an invented adapter, so no real executable, sign-in or provider preparation can run. Its adapter still passes through the production ProviderLifecycle start, initialize and watcher code. The test must not run in parallel because module registration is process-wide.

Construct the restarted App with `NewApp()` on the existing isolated home, then call `fixture.Bind(t, app)`. Bind registers cleanup and disables automatic judgement. `fixture.Count(provider, "")` counts launches. A method such as `session/load`, `session/resume`, `session/new` or `session/prompt` counts the actual RPCs through that fake connection. Counts are synchronized and cumulative across Apps using the fixture.

Call `app.chatCleanupAcceptanceRestartChats()` after installing any clock/deletion hooks needed by the case. It runs removal reconciliation, the production queue-restoration sweep, the page's `ChatWindows(nil)` notification, and a production bus coordinator discovery/delivery round. `app.chatCleanupAcceptanceDiscoveryRound()` repeats the latter after an invented message. `watchConnects(app)` records the real requests that would ask the page to open a chat; the existing `openAsAsked` helper can execute those requests through OpenSessionAt.

This binding does not call the whole App.startup method. That method also starts package update checks, standard-file installation and unrelated maintenance, which are outside this isolated acceptance run. It does not claim a native desktop restart. The binding calls the same chat reconciliation, queue and discovery implementations that startup and its watcher use, without a background clock or an alternative scheduler.

The binding's control case first observes zero launches, loads and prompts for a binned chat after restarting and trying to enqueue a bus message. Restore must keep those counters unchanged. Explicit reopening then increments the launch and load counters, and an invented prompt through the real turn path increments the prompt counter. Those controls ensure that a zero observation is not merely an inert counter.

Quality retains ownership of the frozen expectations and independent fixture binding. This addition supplies observations; it does not replace those assertions.

The focused control passed through FIFO ticket 2184 in 0.935 seconds. The command was `go test -tags chatcleanup_acceptance ./cmd/dinah-desktop -run '^TestCleanupAcceptanceRestartCountsRealProviderEntry$' -count=1 -timeout=90s`. The full desktop package with this additional tagged fixture remains a separate pending gate.
