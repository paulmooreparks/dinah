# Editor, Docs, Diagrams and Slides from yavc.app

Status, 9 November 2026: Editor, Docs, Diagrams and Slides are hosted and drivable by agents, and the yavc.app Editor is Dinah's only Editor. Paul ruled the same day that it replaces the built-in Editor, and that all plain text editing uses it; the built-in Editor, its Visual view and their Go support are removed. Paul's rulings are recorded under "Rulings". The numbered decisions record what was built; "Built" and "Still to do" at the end record the state of the work.

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

### 2. Agents reach the applets through Dinah's Go broker

The applets install only on a page that has PUDL's own actions broker, `pudl-actions.js`, which refuses a manifest whose capabilities it does not know, so the page loads it. It is the page's dispatcher and nothing more: no agent reaches it directly, and the Go broker stays the one authority over what an agent may run. A Go bundle is made from each package's `actions.json` as the release has it (`applet_content.go`), so the actions appear to agents under their own references through the existing `applet_discover` and `applet_invoke`, with no new MCP tools. A handler hands the call to the page over the page-ask channel, with a longer wait than the usual answer; the page runs it in the right window through the binding each window's applet makes under `documentWindows` (its `accept`), and the result, with any artifact, comes back into the broker record. Consent stays in the chat, as for every other action.

Opening a document and starting one make a window of their own; an agent's save to a path is a Save As of the window's document to a new file; closing closes the window. An action that works on text alone, such as Slides' `parse@1`, runs in the page's broker with no window.

### 3. Capabilities

What an action may do in Dinah is Dinah's to say, so the bundle puts the release's capabilities in Dinah's words instead of adding new ones: `files.read` is `workspace.read`, `files.write` is `workspace.write`, `window.open` is `window.command`, and any live action that may change the document is `workspace.write`, since Dinah saves an open document to its file as it changes. Reading actions ask nothing; changing ones ask as a workspace write does. The page registers `files.read` and `files.write` with its broker, as the workspace contract requires, backed by the draft service.

### 4. Live actions open their window

A live action with no attached instance opens or raises the right window, waits for the applet's readiness and recovery, and then runs. The current refusals ("not open", "not shown since startup", PUDL's three-second attach wait) do not apply to these applets. Long actions such as export get a timeout of their own rather than the fifteen-second page-ask limit.

### 5. Documents are addressed by resource, not by tab

The yavc actions address documents by tab index and by `~/` paths, which mean nothing in Dinah. Action references are contracts, so the fix belongs upstream. yavc.app adds an instance or resource target to its live actions in new action versions, and Dinah routes on it. Until those versions exist, Dinah's open actions return a window key and a live action without a target goes to the most recently focused window of that application in the chat's workspace. Dinah does not ship wrapper actions of its own under the yavc provider ids.

### 6. Windows

Dinah supplies `documentWindows` version 1 and answers `single` for every mount, so each document gets its own independent window, which is how Dinah's Editor already works (ruling 6). New, Open, agent creation and recovery all allocate a new window or raise the existing one; none adds a document to an open window. Because a window holds one document, Dinah routes an agent's call by window, and the applet's internal document index inside a window is always its only document. Title and changed state go through `dinahWorkbench.retitle` and the title-bar status chip, and commands, menus and shortcuts resolve to the owning window.

PUDL's limit of nine numbered instances applies only to the windows PUDL opens itself for an applet's `handles` verbs. Dinah opens its own windows, one for each draft, and defines the packages without `handles` or `instances`, so the limit does not arise in Dinah.

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
| The Files pane hidden unless turned on (ruling 5) | Done in v0.2.50. |
| Two comments in the Editor's source named the downstream product, which Dinah's forbidden-terms guard refuses in a public repository | Removed in v0.2.50, which Dinah pins; the local commits that held v0.2.49 were rewritten to hold v0.2.50 before the first push. |

## Built

- The release vendored and verified (`frontend/yavc`, `YAVC.md`, `yavc_release_test.go`).
- The host (`content-host.js`): resources backed by drafts, with readable resource names kept in the UI state; recovery snapshots in the UI state; settings; `documentWindows`; Save As and Open through Dinah's file picker; the toolbar service (`content-toolbar.js`).
- The windows (`content-windows.js`): Rename and Discard from the Documents menu, and a rename made elsewhere followed; one document to a window, saved a moment after typing stops and before closing; changes made elsewhere shown, or asked about with Keep Mine and Take Theirs; a file changed on disk taken, or offered with Bring In Their Changes; a document moved between applications rather than open in two.
- Agent control (`applet_content.go`): the 99 actions of the four packages, routed to the window in front, with exports kept as artifacts and `dinah.workspace/save-artifact@1` to save one as a file.
- Suggestions (`applet_documents.go`): `dinah.documents/read@1` and `suggest@1`, and the review panel in each window with Edit, Accept, Reject, Accept All and Reject All.
- The yavc.app Editor as Dinah's Editor: its windows are of kind `editor`, so layouts, pins and the Editor's window setting carry over; skill editing embeds it; Tools › Editor and New Document start one; chat links to a line go to that line. Dinah's checks of a document (a skill's package rules) are listed in the window under a Problems button, which a skill's Check opens.
- Routing: Markdown by Settings › Markdown Opens In (Visual is Docs), `.docs.md` to Docs, D2, Mermaid and Canvas to Diagrams, `.slides.md` to Slides, everything else to the Editor; Edit as Text to the Editor's Text and opening to read to its Preview.
- The D2 worker's own content policy (`shell_server.go`).
- Two fixes found on the way: opening one resource twice at once made two drafts, and the hidden window under test loaded a second page.

## Typing speed, 9 November 2026

Paul found typing in Docs far slower in Dinah than in a browser, and the cause was measured on a copy of his real layout (29 windows) in a hidden window: a keystroke cost 385 ms at the median and up to 2.7 s, against 65 ms on a fresh state. Nearly all of it was redrawing. The applet reports a change of its commands at every keystroke, PUDL then redraws every command-bound control of every window on the page, and in doing so evaluates each menu command's items, which for the applets' Insert menu runs the Markdown engine to list the document's shortcodes. Dinah's own windows added to the storm: every chat window and every group window asked for a page-wide redraw at each chat-state snapshot, bus overview and periodic poll, about ten redraws a second whether or not anyone typed; each retitle of a window redrew the docks and taskbar for all windows, and the applets' tab strip retitles at every change; Dinah's Window menu rebuilt its list of open windows dozens of times per redraw; and a document's "(changed)" mark made the taskbar re-resolve every pinned destination.

The fixes, all in Dinah's page: redraw requests are gathered into one every 20 ms (workbench.js); a retitle to an unchanged title is dropped before it reaches PUDL; the applets' dynamic menu items were evaluated at most four times a second until yavc.app cached them in v0.2.52; the open-windows list is built once per task; the pins refresh waits for a quiet quarter second; the toolbar reads only the table's own rows; the recovery snapshot is written once a second; the Problems check runs once per revision after the document is still; and a window's own save no longer compares the file on disk. yavc.app's #66 stops its page furniture being redrawn while typing and #67 caches the Insert menu and reports command changes only when a state changes; both came to Dinah in v0.2.52.

Results, 61 keys in a 400-paragraph document, before #66: fresh state 65 → 15 ms median; real layout 385 → 46 ms median and 1134 → 457 ms at the 90th percentile, with spikes of about a second left. What remains on the real layout is Dinah's chat and group windows reacting to periodic polls (chat times, team state, session information) by re-laying out their transcripts, about 14% of the main thread while typing, which is a separate piece of work.

### The sweep of pointless updates, 9 November 2026

Paul then asked for every pointless update that typing sets off to go, in the document applets and in chat windows alike. Measured the same way, typing in a chat composer cost 19 ms a key at the median; the handlers themselves took 11 ms across 73 keys, so the composer was already quiet apart from sizing its box twice a key, now once. What interrupted typing came from the other windows: every redraw of the chat list made the bus pane and every group window redraw, and each group window then asked Go who its draft would reach, rebuilt its list of other saved drafts and fetched and redrew its whole message list, whether or not anything of its own had changed. Those now happen only when the text or files of the box changed, when the saved drafts differ, or when the overview shows the room changed. A document window's title lost its mark for unsaved typing, its status line no longer says Saving and Saved at every write, the applets' tab strip may no longer retitle the window, and the applet's report of its caret line no longer writes the window layout to Go. Restored chats replaying their transcripts at startup still arrive as thousands of events, which is a startup cost, not typing's. Measured once that replay has settled, typing in Docs on Paul's layout costs 31 ms a key at the median and 94 ms at the 90th percentile, against 385 ms and 1134 ms at the start of the day.

### Chat-state redraws, 9 November 2026

A chat-state snapshot from Go made every chat window draw itself again (its composer options, queue, usage, state line and approvals) and every group window redraw its members, whichever chats had changed. The snapshot now says which chats' states changed and whether any group's counts did; only those chats draw again, and a snapshot that changed nothing draws nothing. The teams strip refolded a chat's traffic at every change under its log, including each chunk of text streaming into a row; it now refolds only when a row is added, removed or changed. Measured idle on Paul's layout, the page was busy 16% of the time, three quarters of it applying the providers' replay of chat histories after the restored chats reopened; the rest was the teams fold, the content windows' periodic file comparisons and small redraws.

### The startup replay, 9 November 2026

When a restored chat reopened its session, the provider replayed the whole conversation and Go forwarded it to the page in batches of updates, thousands of them on Paul's layout. The page had already drawn the chat from Dinah's saved copy and reads what followed from the session journal, so it built the replay into a log it then threw away; that was three quarters of the idle page's work in its first minute. Go now forwards a replay to the page only when the page asked to see one, which is Load Full History, and records it in the journal as before for an ordinary reopening (`replayToPage` on the conversation, set beside `SetReplaying`). Measured idle on Paul's layout after the restored chats settled, the page was busy 249 ms in 20 seconds, against 3194 ms before, and no replay events reached it.

A chat window the layout kept hidden did not finish restoring until it was shown: the render queue holds a hidden window's work by design, the saved copy's last draw was such work, and the reconnect to the agent waited behind it, so the chat stayed closed on the bus with "Reading the saved conversation…" in its window. Paul ruled that a hidden chat must reconnect, since the bus may contact it, so the saved copy's last draw now runs whether or not the window is in sight.

### The launcher, 9 November 2026

The launcher listed none of the four applications: the Editor was filtered out in favour of New Document, and Docs, Diagrams and Slides had no launcher entry. Each is now a launcher entry in the Documents category, and a Tools entry in the menu bar, that starts an empty document of its kind in the current workspace; New Document left the launcher as the Editor's duplicate and stays in the Documents menu. The launcher's entries are in alphabetical order whichever category is chosen (Paul's ruling).

### Compare, for a file changed on disk, 9 November 2026

The built-in Editor's notice for a file changed on disk had a Compare button that opened a modal dialog listing the conflicting parts. The content windows' notice has Compare again, and it opens Dinah's Diff window instead, with the draft's text as the "before" side and the file as it is now as the "after", titled "Yours, then the file as it is now"; the architectural principles rule out a modal, and the Diff window keeps the document in sight. When the two touch no part in common, the Diff window has Bring In Their Changes as the notice does; both merge the file's changes into the draft, save the merged draft to the file at once (the applet counts nothing as modified after showing the merged text, so a save through it would have written nothing, which was also true of the notice's button before), close the Diff window and show the merged text. The comparison from Go now includes the two texts themselves (`draft` and `current`) beside its unified diffs.

### Files added to a document, 9 November 2026

The built-in Editor kept an image pasted or a file dropped into a document, or chosen through Insert › Image… and File…, with the draft, and wrote it beside the document when the draft was saved. The content windows do the same for the Editor and Docs. Insert Image… and Attach File… are in the Documents menu, an image pasted or a file dropped into the document is caught at the window before the applet sees it, each file is kept with the draft (AddDraftFile, PickDraftFiles), and a reference to it goes in at the cursor through the applet's own insert action: an image as `![](name)`, any other file as a link. Saving writes the kept files beside the document, as before. Diagrams has no cursor to insert at, and Slides takes pictures through its own figure shortcode, so the commands stand aside for both. The picture itself does not yet show in the window; see Still to do.

### SVG artifacts, 9 November 2026

The broker admitted an SVG artifact only when it was rectangles and text with a handful of attributes, the shapes an early test applet drew, so every drawing from Diagrams' `render@1` was refused and only PNG came through. The check is now a set of rules rather than a list of shapes: one SVG document, elements in the SVG namespace (and XHTML inside `foreignObject`, as Mermaid draws its labels), nothing that runs code, loads another document or takes input, no event handlers, references only to a part of the drawing or to data it carries (a picture or an embedded font), styles with no imports and the same rule for `url()`, and no declarations in the prologue beyond the XML declaration. A real D2 rendering, with its nested `svg`, masks, markers, classes and embedded fonts, passes; the refusals are tested one by one. The artifact window shows an SVG as an image, where none of those would act anyway; the check is for the file a person saves and opens elsewhere.

## Still to do

- v0.2.52 is pinned, with #66 (page furniture kept while typing) and #67 (the Insert menu's shortcodes cached, and hosts told of a command change only when a command's state changes). Dinah's interim once-a-quarter-second memo of the applets' menu items went with it.
- Images beside a document do not show in Docs, the Editor's preview or Slides: the applets' sanitiser keeps only web and data: image addresses (md-engine/safe-html.js), and where a relative `![](shot.png)` does reach the page, as in Docs' Visual view, the browser reads it against the applet's own address, so the picture is broken. Dinah serves a draft's files at `/draft-file?draft=<id>&name=<file>`, so what is needed from yavc.app is a host-supplied resolver for document-relative addresses, which the integration handoff already anticipates ("editable document dependencies require separate resolvers").
- Suggestions marked in the text itself, which needs yavc.app to accept host-supplied suggestions.
- Insert Image and Attach File in Slides, through its append-to-slide action with a figure shortcode, and the yavc.app resolver above for the picture to show.
- Customising the toolbar, and Slides' presenter window and its `fs.save` export.
- An end-to-end run with a real chat agent through the MCP helper and the consent prompt; the broker path is tested, the chat path is not.
- A new document's window is "Untitled.d2" in Diagrams and "Untitled.slides.md" in Slides, from the application showing it; the person's Save As still picks the file's real name.
- The browser suites keyboard and documents-menu, which failed before the change for other reasons, still test the removed Editor's own keys and menus and need rewriting for the yavc.app Editor.
