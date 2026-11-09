# Editor, Docs, Diagrams and Slides from yavc.app

Status: plan, 9 November 2026. Nothing here is implemented. Paul's rulings of 9 November are recorded under "Rulings"; the numbered decisions are proposals unless a ruling settles them.

Dinah's built-in Editor is to be replaced by the portable Editor from yavc.app, and Docs, Diagrams and Slides are to be added beside it. Agents in Dinah chats must be able to drive all four applications completely. The rest of Dinah stays as it is. The larger purpose is that Dinah becomes the base a private downstream product is rebuilt on, so the editing stack chosen here is the one that product's document work will eventually sit on.

The producer's handoff is [dinah-integration-handoff.md](../dinah-integration-handoff.md). Its contracts live in the yavc.app repository at the release tag, not in this repository, and this plan cites them by their yavc.app paths.

## What exists today

Dinah's Editor is a PUDL in-page window kind (`frontend/editor-applet.js` and thirteen part files, with ProseMirror in `page-view.js`), one draft per window, with Visual, Text, Split and Preview views. D2 and Mermaid are rendered in previews but cannot be edited, and there is no Canvas, Docs pagination, Slides or Office import. The native authority is already the one the contracts ask for. Drafts are revision-conditional, saves compare a stored baseline, create-only and Save As exist, attachments are kept with the draft and written beside the document, and drafts survive restarts as files. Its limits are UTF-8 text up to 1 MiB, local folders only, and no recovery snapshot of the version-2 shape.

Agents reach the Editor only through `dinah.windows/run@1`, which runs a command id against whatever is selected. They cannot read a document's text, open a file by path, set a selection or insert given text through the Editor; they can only write files on disk with `dinah.workspace/write-file@1`.

Dinah's applet broker is in Go. Actions are registered as Go bundles or signed extensions, consent is asked in the chat, and every invocation is a durable record. Dinah does not load PUDL's `pudl-actions.js`, by design.

The yavc.app applets publish 99 actions in PUDL's applet-actions form (Editor 25, Docs 27, Diagrams 16, Slides 31), almost all `live`, attached in the page with `pudlApplets.attach`. They call `pudlApplets.attach`, `can`, `register` and `request`. So the actions exist, but they live in the page broker that Dinah has chosen not to run.

## Rulings

Paul ruled on these on 9 November 2026.

1. The order is Editor, then Docs, then Diagrams, and Slides if there is time.
2. Agent suggestions must be reviewable, editable, and acceptable one at a time or all at once. The mechanism is open as long as it is solid; resemblance to the downstream product's implementation is a bonus, and clean code and a clean UI come first.
3. Closing a document asks nothing. Drafts are always available, and changes to existing documents are always saved automatically.
4. Paul may need more than nine open documents per application, and he is asking yavc.app for that change himself.
5. A further yavc.app fix is in progress that hides the Files pane unless it is turned on. Dinah pins the release that contains it, not v0.2.49.
6. The tabbed form of the applets is not ported. Every document gets its own independent window, as in Dinah today, and no applet window ever shows tabs.
7. Integration starts now without waiting on yavc.app. Each upstream item gets the interim Dinah answer listed under "Work needed in yavc.app", and is replaced when the upstream change lands.

## Proposed decisions

### 1. Adopt the complete release, pinned and verified

Dinah vendors one whole yavc.app content release as a single tree (v0.2.49 is the verified one today, with manifest digest `7391737…684a` and 483 files, but ruling 5 moves the pin to the release with the Files pane fix), the way it vendors PUDL, with a provenance file stating release, commit and digest. A Go test verifies the aggregate digest and every file's length and SHA-256, the tree gets a `-text` rule and a checkout byte test like the PUDL trees, and the release is never edited in place. Upgrading means replacing the tree with another reviewed release. The release bundles PUDL 0.105.0, which Dinah already ships; the build refuses the tree if the bundled PUDL differs from Dinah's, and Dinah serves one PUDL.

### 2. Dinah implements the page side of PUDL's actions contract itself

The applets need `pudlApplets.attach/can/register/request` in the page. Rather than loading PUDL's broker, which would create a second authority beside the Go one, Dinah supplies its own implementation of that documented page interface (PUDL specification, chapter 9). Attach records the live handlers per window. A Go bundle is generated from each package's `actions.json` at startup, with the manifest digest as its bundle digest, so the actions appear to agents through the existing `applet_discover` and `applet_invoke` with no new MCP tools. A Go handler forwards the invocation to the page over the existing page-ask channel, the page calls the attached handler, and the result comes back into the broker record. Consent stays in the chat, as for every other action. `request` from the applet (its calls for host services) goes to Dinah's native services.

This is the piece that gives agents control, and it also makes any future PUDL applet with a manifest drivable from chat without further work.

### 3. Capabilities

Dinah's capability list gains `files.read`, `files.write` and `window.open`. `files.read` is treated like `workspace.read` and asks nothing. `files.write` and `window.open` are effectful and ask, like `workspace.write` and `window.command`. Each is backed by the invocation's own workspace.

### 4. Live actions open their window

A live action with no attached instance opens or raises the right window, waits for the applet's readiness and recovery, and then runs. The current refusals ("not open", "not shown since startup", PUDL's three-second attach wait) do not apply to these applets. Long actions such as export get a timeout of their own rather than the fifteen-second page-ask limit.

### 5. Documents are addressed by resource, not by tab

The yavc actions address documents by tab index and by `~/` paths, which mean nothing in Dinah. Action references are contracts, so the fix belongs upstream. yavc.app adds an instance or resource target to its live actions in new action versions, and Dinah routes on it. Until those versions exist, Dinah's open actions return a window key and a live action without a target goes to the most recently focused window of that application in the chat's workspace. Dinah does not ship wrapper actions of its own under the yavc provider ids.

### 6. Windows

Dinah supplies `documentWindows` version 1 and answers `single` for every mount, so each document gets its own independent window, which is how Dinah's Editor already works (ruling 6). New, Open, agent creation and recovery all allocate a new window or raise the existing one; none adds a document to an open window. Because a window holds one document, Dinah routes an agent's call by window, and the applet's internal document index inside a window is always its only document. Title and changed state go through `dinahWorkbench.retitle` and the title-bar status chip, and commands, menus and shortcuts resolve to the owning window.

The contract allows nine numbered instances per application, and the host refuses a tenth. Paul is asking yavc.app to lift that (ruling 4).

### 7. Recovery and closing

Drafts stay the safety net. The applets' recovery snapshots are saved as keys of Dinah's one UI-state store, never with a store of their own, and the existing close handshake, which already waits for an acknowledged flush, is extended to wait for every open applet's flush. The contract's synchronous `flush()` must not report success before Dinah's asynchronous write is acknowledged; the proposal is that `flush()` returns "pending" and the host waits on the acknowledgement, which needs agreement with yavc.app because it touches their contract.

Dinah's Editor never asks on close, because the draft keeps the work, and the new applets keep that behaviour (ruling 3).

### 7a. Agent suggestions

Ruling 2 sets the requirement. Two mechanisms exist. Dinah's suggestions are a layer on the draft, kept outside the document's text, which works for any file type, code included. yavc.app's review marks are CriticMarkup written into the Markdown source, with a review list, Accept, Reject, Accept All, Reject All and editing of a proposal, all as undoable edits. Because existing documents save automatically (ruling 3), CriticMarkup marks proposed by an agent would reach the saved file before anyone reviewed them, and they cannot be used in a Go or C++ file at all.

The proposal is that agent suggestions stay a draft-side layer, so they never touch the saved file until accepted and they work in every file type, and that the new applets show and resolve them with the same review list and controls yavc.app built for its marks. That needs yavc.app to accept host-supplied suggestions (ranges, replacement text, author) through its host interface, and to report Accept, Reject and edits back to the host. CriticMarkup stays available for people who want marks kept in a Markdown document. The downstream product's implementation is to be compared before this is settled.

### 8. Content security policy

The D2 layout worker needs evaluation. Only that one worker route gets a narrower policy that allows it; the page policy and its test stay unchanged.

### 9. Resource providers stay general

The adapter talks to Dinah's resource and draft authority through resource references, never local paths, so that the pages the downstream product publishes to can later be resources of a provider of their own without changing the adapter. Local folders remain the only provider for now.

## Sequence

1. Vendor and verify the release, serve it, add the worker policy, and mount nothing yet.
2. Build the page-side actions implementation and the generated Go bundles, with capabilities and open-on-demand, proved on the Editor.
3. The Editor, behind a setting, beside the old Editor, including agent suggestions (decision 7a). Parity is checked against the list below, and the old Editor is removed only after Paul accepts the parity result.
4. Docs.
5. Diagrams.
6. Slides, if there is time.

Agent acceptance closes each application's step. A chat agent creates, edits, formats, reviews, exports and saves a document in it with no help from the person, on copied documents in an isolated profile.

Each step ends with hidden-window acceptance on a copy of real state and its own commit. Installation stays Paul's action.

## Parity the old Editor has that the new one must keep or replace

- Agent suggestions shown in the document with Accept, Reject and Revise (`AcceptSuggestion`, `RejectSuggestion`, `ReviseSuggestion`), which yavc.app's review annotations may or may not replace (question 2).
- Compare and three-way Bring In Their Changes when the file moved on.
- Take Theirs and Keep Mine when a draft changed elsewhere.
- Attachments kept with the draft and written beside the document.
- Skill editing, which embeds the Editor.
- The Documents menu, Recent, In Progress and pinned documents.
- Document links and the editor status bar.
- Auto-save and the default view per media type.

## Work needed in yavc.app, and the interim answer in Dinah

Ruling 7 means none of these blocks the start.

| Upstream change | Interim answer in Dinah |
| --- | --- |
| A resource or instance target on live actions (decision 5) | Dinah routes by window, since each window holds one document. |
| A selection action, so `format` and `table` act on a stated range | Agents use `replace` and `set-text`; formatting a chosen span waits for the upstream action. |
| Save As that takes its destination as input | A Dinah action calls the draft authority's own Save As. |
| The asynchronous `flush()` result (decision 7) | `flush()` puts the snapshot into the in-memory UI-state store and returns at once; the existing close handshake waits for the durable write, and drafts keep the text in any case. |
| Host-supplied agent suggestions in the review list (decision 7a) | A Dinah review panel beside the document lists each suggestion with Edit, Accept, Reject and Accept All, applied through the existing draft suggestion methods. Highlighting in the text waits for upstream. |
| More than nine instances per application, a PUDL change Paul is requesting (ruling 4) | The limit stands until Dinah adopts the PUDL release that lifts it. |
| The Files pane hidden unless turned on, in progress (ruling 5) | Work starts on v0.2.49 and the pin moves when that release is out. |
