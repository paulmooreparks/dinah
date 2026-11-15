# Passive startup queries

Paul approved passive startup on 15 November 2026 in message 9136. The page builds its chat list from local saved records and existing runtime observations. It contacts a provider when an open chat needs that provider or when the person explicitly requests its sessions. Startup must not prepare a provider, initialize a connection, list native sessions, query a model catalog, load a session or fetch remote prices.

## Host queries

`SavedChats()` returns `{generation, chats}` across all workspaces. Each chat embeds the existing `SessionInfo` fields and adds `chatId`, `resolution` and `reason`. A resolved row's `sessionId` is its current native head, which preserves the existing page cache and opening path. `chatId` is the stable logical identity. A repair row retains its reference identity and reason, with no opening action. Missing title, provider or workspace metadata stays empty. The page filters the captured inventory locally and shows repair rows without guessing their workspace.

Generation is an opaque content token, not an ordered version. The page serializes refreshes or rejects an older request result by its own request sequence. Per-chat state clocks retain their existing meaning. Missing workspace or provider metadata also requires a repair row because it cannot authorize an opening in the selected workspace.

The query clones settings once, captures existing runtime information, and resolves local references through `logicalChatReferencesFromSettings`, `logicalChatMembersFromSettings` and `logicalChatMetadataFromSettings`. Transcript headers supply local metadata; transcript parent links supply no ownership. A switched chain appears once, while separate recipients remain separate chats. Every removal reservation excludes its owned chain, including future phase names. User-hidden chats and judgement records stay excluded. The query refuses a concurrent settings or transcript identity change instead of combining generations. It starts no provider, opens no chat and constructs no action broker.

`DeclaredProviders()` returns product-declared rows containing `id`, `name`, `default`, `facts` and `status: "unknown"`. Facts may reflect an already initialized connection. The query performs no executable lookup, credential lookup, HTTP request or installation check. It omits installation and availability booleans because neither has been checked. The existing `providers:facts` event remains a passive update.

`AgentProviders()` remains an explicit scan for Settings' agents page or its refresh action. Codex preparation calls `providerInfoOf("codex")` to inspect only the requested provider. Existing availability checks and preparation failures retain their meanings.

Devin initialization does not fetch prices. The page requests prices when a chat uses that provider, through the existing `ModelPrices` call. The call still returns its cache immediately and emits the existing event when a background refresh finishes. Its first requested refresh uses the current initialized version when available.

The existing native session-list calls remain separate from `SavedChats()`. The page invokes native discovery only after the person's explicit provider-session action. Codex history that lacks local ownership evidence remains subject to the accepted import boundary; reading history does not import a chat.

## Page deferrals and receiving gate

Front End owns the page changes. The blank composer reads its skills on first focus or slash entry. Prices are requested only for a provider used by an open chat. Disabled judging performs no inventory request or temporary allocation; enabled judging loads those choices when its settings page is shown. Ollama memory is read only when its settings section is shown.

Opening the judge store and reading judgement state, saved settings or calibration reports are passive. Automatic calibration begins at the first enabled judgement call, or when the visible enabled settings section requests its model inventory. Explicit calibration keeps its existing entry point. A judgement arriving while calibration is pending retains the existing rule path until a model is available. Disabled judgement kinds cannot trigger automatic calibration.

The seventeen counted audit cases become acceptance tests, with positive controls for explicit actions and unchanged open-chat behavior. Host checks cover canonical chains, separate recipients, lifecycle reservations, hidden and repair rows, all-workspace filtering metadata, provider declarations without probes, and preparation of Codex without scanning another provider. Front End combines the immutable host and page candidates before independent Quality acceptance. Invented homes and providers remain mandatory for every run.
