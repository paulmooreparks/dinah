# Org chart host query

The read-only App.OrgChart query and pageMethods entry consume Release's captured inventory and membership seams on base b74f8219. Engine owns the host files; the page remains Front End's work. This branch is separate from the cleanup release and has not been integrated into a product.

Focused tests passed in 0.627 seconds through FIFO. They use invented canonical chains, a separate recipient, a reserved chat and repair ownership. They observe no provider calls or launch attempts, verify canonical team edges and compare generation changes. Fake adapters refuse every launch attempt, so a test failure cannot start a real provider.

The first compile failed because TranscriptHead has no Title field. The query now reads titles from captured settings and the model, and uses transcript headers for workspace and provider metadata. The next run failed both tests because JSON cloning normalized empty optional maps to nil, causing a false concurrent-change refusal. The query now captures its ownership stamp from the original settings under the same lock as cloning and compares that stamp against the current settings before return. The original assertions are unchanged.

## Standalone interim port

The query is now ported onto accepted cleanup master db6f8d11 in engine/org-chart-integration. It uses the pure inventory and metadata declarations agreed in contract c709d20a. No logical model, journal or importer is included. A detached Settings snapshot carries captured transcript headers in a private, nonserialized map. The query validates both the ownership stamp and transcript-header stamp before returning. The page DTO and canonical node identities are unchanged.

The focused query and seam selection passed in 1.012 seconds through FIFO after the final seed changes. It covers canonical edges, repair rows, future removal phases, transcript-only local evidence, provider-history exclusion and detached snapshot metadata. Provider preparation is replaced with a counted refusal throughout these fixtures.

TestOrgChartExportHiddenSeed passed separately in 0.549 seconds and retained invented state at C:\Users\paul\AppData\Local\Temp\engine-org-chart-seed-940a879e4234477398823dac661d453d. Its home and workspace subdirectories contain closed chats, a switched chain, a separate fork, a reserved chat, a team and a group. Front End can use this home with its isolated hidden entry and provider access disabled. The seed calls the real local stores and OrgChart without provider activity.

To reproduce the export, set DINAH_ORG_CHART_SEED to a fresh child directory below TEMP and run TestOrgChartExportHiddenSeed through scripts/with-test-lock.ps1 with the chatcleanup_acceptance tag. Existing destinations are refused. Ordinary package runs skip the export.

The complete desktop package, vet and hidden real-query rendering remain pending. The seed is local host evidence and does not prove native startup or rendered behavior. No visible window, native provider, installation or real home was used.
