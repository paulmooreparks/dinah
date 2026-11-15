# Page startup provider audit

The page starts agent programs and queries inventories even when no chat is open. Seventeen counted cases passed on source b6c06ca7. The baseline capture retains bridge call stacks, counters, native request records and executable hashes outside the public repository because it contains machine-specific paths. Every ordinary page case began with zero loaded chats.

## Measured behavior

| Page configuration | Unnecessary native activity at startup |
| --- | --- |
| Codex default | One app-server, initialization, thread/list and two skills/list requests; one Devin prices command. |
| Devin default | One ACP process, initialization and session/list; one separate Devin prices command. |
| Claude default | One adapter process and Node version check, initialization and session/list; one Devin prices command. |
| Ollama default | One in-process preparation, initialization and session/list. |
| Disabled judging with Codex selected | Two model/list and two thread/start requests in addition to ordinary Codex startup activity. |
| Disabled judging with Devin selected | A separate judge process with initialization, two session/new and two session/close requests. |
| Disabled judging with Claude selected | A separate adapter with initialization, two session/new and two session/close requests. |
| Refusing Codex executable | Three launch attempts and no protocol requests. The historical four-launch observation did not reproduce on this source. |

The ordinary default cases also queried Ollama tags three times and memory four times. Version checks varied with the selected provider and preparation path. The baseline retains the individual counts rather than treating these repeated calls as one operation.

Fresh query-only hosts established the attribution. State and saved provider preferences caused no provider activity. Sessions prepared the default provider and listed native sessions. ChatSkillCatalog independently prepared Codex and requested skills. AgentProviders scanned every declared program, read the fake Claude credential and queried Ollama's version. Codex preparation itself called this global scan. JudgementModels queried Ollama tags and memory; configured ACP judges also created temporary sessions even while judging was disabled. OllamaMemory queried memory. ModelPrices for Devin launched its models-list command in the background.

The source distinguishes two provider lists. Session listing considers the default and already-running providers; it does not enumerate every installed program. AgentProviders performs the full scan. Automatic Devin discovery can additionally probe executable versions and ACP initialization. That last path was inspected in source only because the measured runs used explicit invented executable overrides.

The positive control opened an invented Devin chat through OpenSessionAt and observed one session/load. That load occurred after the startup snapshot. Repeated diagnostic reads changed no counters and initialized nothing.

## Approved change

Team Lead approved these deferrals in message 9136. Engine agreed the host seams in 9135, 9137 and 9139. Engine owns host implementation; Front End owns page implementation on stacked branches.

| Current caller | First permitted use |
| --- | --- |
| Startup Sessions/SessionsAt | Startup uses SavedChats instead. Native history requires an explicit provider-session or import action. |
| Startup AgentProviders | Startup uses DeclaredProviders. The full scan runs when Settings Agents is shown or explicitly refreshed. |
| Codex preparation's AgentProviders scan | Opening Codex prepares Codex alone through providerInfoOf, retaining availability checks and failure reporting. |
| Blank composer ChatSkillCatalog | The person first focuses the composer or types a slash. Open chats retain their current skill behavior. |
| Provider-ready and provider-facts prices refresh | Only providers used by an open chat have prices fetched. |
| Mounted judgement settings inventories | Judging must be enabled and its settings section shown before inventory or temporary-session work occurs. |
| Mounted Ollama memory | The Ollama settings section must be shown before reading memory. |

SavedChats returns one captured local snapshot as {generation, chats}, using the logical reference, membership and metadata seams and already-present runtime state. Each row extends SessionInfo with chatId, resolution and reason. Resolved sessionId is the current head used by chat caches. Repair rows remain visible with a reason and no opening action. Missing metadata remains empty. Workspace changes filter the snapshot locally. Removal reservations remain excluded, and ancestry never establishes ownership.

DeclaredProviders returns id, name, default, facts and status unknown from declarations, saved defaults and already-present initialization facts. It omits availability, installation, executable and catalog fields. Passive providers:facts events update these rows. This gives the toolbar declarations without implying that an availability check succeeded or failed.

The implementation target is zero provider contact during startup when no open chat requires a provider. The counted cases become acceptance checks, with explicit queries and the open-chat positive control retained. Quality checks the combined result independently.

## Verification limits

The hidden run served the production embedded page and real App bridge through a loopback shell server with headless Edge. Stand-ins ran below real App and SessionLifecycle routing. All homes, credentials, executables and services were invented and isolated. No visible window, real provider account or real credential lookup was used.

This checks page startup, not the native Wails startup lifecycle. The fixture uses the existing native-window port and does not run App.startup. Ordinary lifecycle counters and native protocol records are separate because judge connections bypass the ordinary provider lifecycle. The native records preserve those judge requests.

Early fixture runs exposed an invalid native-window context, TestMain overriding the intended isolated home, a missing invented tools executable, and an incomplete fake Ollama memory response. The final seventeen-case run includes the corrected bindings. No production source was changed to obtain this baseline. Focused production asset and page-ownership checks passed in 0.448 seconds afterward.
