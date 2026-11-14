# Person-only operations reviewed on 14 November 2026

This review compares yesterday's 4fe5a52c with master a4afc5e0. It inventories exported App methods whose implementations changed, the changed agent actions and the changed HTTP boundaries. The review covers source entry points, not arbitrary code already running with the person's operating-system account. A process that can read all of that account's files can also alter its state directly; the loopback interfaces do not claim to sandbox that process.

## Authority map

The page bridge requires the window's private cookie and accepts mutation calls only from its own origin. A loopback connection or an applet bearer token grants no page authority. Its pageQuery and pageWork labels classify exit accounting, not caller permissions. Applet callers receive a separate bound channel and a closed action registry. They cannot call App methods by name. The bus MCP server binds the asking chat and registers an explicit tool list.

| Changed App entry | Who can reach it | Review result |
| --- | --- | --- |
| Answer | Authenticated window page only. | Permission answers cannot be supplied through agent tools. The changed implementation dismisses the retained attention card. |
| State (new background-limit output), BackgroundChatLimit, SetBackgroundChatLimit | Authenticated window page only. | The agent preferences action does not expose this setting. |
| BusGroupMessageContext | Authenticated window page only. | This bounded read does not mark messages read. Existing bus reads remain available to agents. |
| Decisions, DecisionByID, DecisionEntries, DecisionOperation | Authenticated window page only. | These queries read the person's list, closed record or operation receipt. No matching agent action exists. |
| AnswerDecision, DismissDecision, ConfirmDecisionBroadcast | Authenticated window page only. | The page remains the only caller that can answer, dismiss or send as the person. |
| PreviewDecisionBroadcast | Authenticated window page only. | This creates a frozen preview without sending. Confirmation remains page-only. |
| UpdateState, RestartToUpdate, CancelRestart, PauseAndRestart | Authenticated window page only, with the indirect Windows command route described below. | Agent attempts to run application restart commands are now refused before page dispatch and omitted from its command inventory. |
| Notifications and its retained-card projection | Authenticated window page; agents can post or request a notice through the existing limited notification actions. | Agent actions cannot dismiss a permission or complete a human request. |
| ListSetAsidePrograms, RemoveSetAsidePrograms | Authenticated window page only. | Listing grants no removal authority. Removal is absent from bus and applet offerings. |
| RedrawDraftPictures, SaveDraftAs | Authenticated window page; document services have their separate scoped document actions. | Existing resource grants, Save As picker and conflict handling remain in force. These methods cannot reach person-only application operations. |
| ShortcodeCatalog, RenderShortcode | Authenticated window page; reviewed content actions can use rendering within their document. | These return drawing data and create no person-only application operation. |

The comparison found no other changed exported App implementation. Startup-only changes keep attention cards, restart records, settings copies and browser-profile selection behind the existing startup paths. They add no remotely callable App method.

| Bus or applet entry added or changed | Who can reach it | Review result |
| --- | --- | --- |
| ask_person and its stood-down signpost | An agent on its bound bus MCP session. | It creates the asking chat's request, with a required recommendation. It cannot supply an answer or masquerade as the person. |
| Bus send/reply guidance | Existing bound bus MCP callers. | Today's change adds guidance only. Sender identity still comes from the bound session. Person-only mutations are neither listed nor callable as tools. |
| Applet helper discover, invoke, status and artifact operations | An authenticated, live, bound applet channel. | Protocol negotiation adds no general App dispatch. The existing registry, digest and invocation ownership checks remain. |
| yavc.docs/open-file@1, insert@1 | Reviewed content applet callers through host capability mapping. | File opening and document changes use existing workspace/file grants and document permission flow. They offer no application installation, repair, restart or decision-answer authority. |
| yavc.docs/position@1, find@1, print@1 | Reviewed content applet callers. | Position and Find read or navigate content. Print reaches the existing print command/dialog and has no application mutation authority. |
| yavc.diagrams/canvas-contents@1 | Reviewed content applet callers. | This returns canvas data without changing it. |
| Other existing Editor, Docs, Diagrams and Slides actions using the changed bundled runtime | Bound agent applet callers through the same reviewed content adapter. | Existing reads, file/window actions and document writes retain their mapped capabilities. No content action is mapped to application installation, settings repair, restart or person-authored Decisions mutations. |
| Windows list/commands/run/open | Bound agent applet callers, subject to ordinary window-command consent. | The new restart identities are refused even after a general grant. Ordinary commands and window reads remain. Decisions' broadcast commands only open the form; they never send its message. |
| Limited Settings read/update actions | Existing agent callers with their normal consent. | Their closed input schemas expose selected preferences only, without the background limit, agent registration repair or installation methods. |

The window's Decisions answer, dismissal and broadcast submission are buttons rather than agent command actions. The Settings removal control is also page-only. The reviewed product's repair method is registered as a page service, without a corresponding agent action. Its installation action does exist for agents, so the separate product branch marks it for fresh approval rather than removing it. Product-specific names and implementation remain outside this repository.

| Changed HTTP channel or route | Who can reach it | Review result |
| --- | --- | --- |
| Shell helper-compatibility GET | Local programs with the expected host and channel header; browser-origin requests are refused. | It returns a protocol selection only. It returns no cookie or page entry code. |
| Shell /bridge/call, /bridge/events and /bridge/app.js | The signed-in window page. | Existing cookie checks remain; calls also require the page origin. New App methods inherit this boundary. |
| Shell page files, including missing-file responses | The signed-in window page; public support assets retain their existing narrow exception. | Today's no-store policy changes caching only. |
| Applet /rpc POST and OPTIONS | A bound applet-channel bearer credential, correct host, no browser-origin request. | OPTIONS negotiates before request decoding. It grants no extra operation or page authority. |
| Extension /host POST and OPTIONS | A live invocation-scope bearer credential, correct host, no browser-origin request. | OPTIONS authenticates and negotiates without decoding a callback. POST dispatches only the existing scoped host operations. |
| Document /control/* including OPTIONS probes | The document service credential on its own host, with browser-origin requests refused. | Negotiation leaves the existing control operations unchanged. Document actions still ask through the owning document page. |
| Document /d/<id>/bridge/* | That document's authenticated page and origin. | The added UseFile operation can reuse only an untouched blank document and only an already granted file. It is not an App bridge. |
| Steering helper and slide-show startup checks | Existing helper session or show-specific authority. | The new exchange checks protocol compatibility only. It supplies no page cookie and widens no operation set. |

## Corrections and approval contract

An agent's general window-command grant must not authorize application restart. The Go adapter rejects pause-and-restart, restart-to-update and restart-now before it asks the page. The page also removes those identities from agent command listings and refuses guessed calls. Human menu commands retain their existing behavior.

An action can declare the generic capability person.approval. RequiresFreshApproval and Asks then require a new approval for every chat-owned invocation, even if the action declares a read/local effect. Approve Everything, provider unattended mode and a saved workspace grant do not cover it. The permission question and its retained Notifications card say that approval is required every time. The available answers are Allow and Don't Allow. A forged Always Allow answer is refused and cannot create a grant. An invocation started by the trusted human page remains the person's own operation.

The mark is part of the action manifest and therefore its reviewed digest. Ordinary actions retain their existing approval behavior. Products must deploy the host support before registering a marked action; older hosts reject its unknown capability rather than silently ignoring it.

## Verification

All checks used the shared FIFO. The full desktop package passed in 284.540 seconds. The applet registry, broker and host packages passed in 0.519, 1.199 and 0.652 seconds; builtin and contract have no tests. The full bus MCP server package passed in 5.371 seconds. A final focused run on the completed source passed the new person-only and fresh-approval tests in 1.270 seconds for the desktop, 0.437 seconds for the registry and 0.394 seconds for the bus MCP server.

The person-only-window-commands, window-commands and update browser suites passed together in 10 seconds, with no page errors. The first browser invocation failed to load playwright-core because NODE_PATH was missing in this new worktree. It ran no assertions. The corrected invocation used the existing dependency installation and fresh output at %TEMP%/person-only-browser-02. No assertion was weakened.

The tests exercise each blanket-grant mode twice, inspect the actual permission choices and retained Notifications card, and reject an invalid Always Allow response. They also try guessed applet and MCP mutation names and verify that unauthenticated loopback callers, applet bearer tokens and wrong-origin requests cannot reach person-only App methods. The page test proves restart commands are absent from agent listings and refused when guessed, while a normal command still runs. Existing tests remain unchanged.

The product branch tests the actual installation action with the shared host assertions and records its own results separately. This upstream branch contains no product-specific action identity. It must be integrated before the product marks its action, because an older host rejects the new capability.

This was a source review with isolated Go and headless browser checks. No full browser or whole-repository suite ran, no real model was called, and no installed program or real application home was changed. Native WebView2 and a live installation were not exercised. The branches are ready for Lead review and combined integration.
