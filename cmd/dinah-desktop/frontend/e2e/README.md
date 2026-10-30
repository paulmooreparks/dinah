# Browser Suites

Each `*.js` file here drives `../index.html` in Microsoft Edge through Playwright, with `window.go` and `window.runtime` stubbed, so the page is tested without the Go side or an agent. Each prints a `PASS` or `FAIL` line per check, ending with "no page errors". A browser dialog (`alert`, `confirm`, `prompt`) counts as a failure in most suites, because the app may only use its own PUDL dialogs. All data and service responses are fixtures; no suite reads personal settings or files, calls a live service, or invokes an agent model.

## Running

```
cd cmd/dinah-desktop/frontend/e2e
npm install
pwsh ./run.ps1                  # every suite; prints only failures
pwsh ./run.ps1 -Only editor,files
```

The suites expect Edge at `C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe`. Screenshots are written to a temporary folder (`-Out` changes it); look at them after a layout change.

## Machine-wide browser lock

`run.ps1` holds `%ProgramData%/Dinah/browser-tests.lock` with .NET `FileShare.None` for the whole batch. A directly invoked Node suite acquires the same lock before Edge starts, through `lib/browser-lock.ps1`. Independent runs wait and print the holder's description, process ID and UTC start time every five seconds. PowerShell 7 must be on PATH for either entry point.

The batch passes its children a ticket naming the current lock owner. The helper validates the ticket against the owner record, process birth time and held file handle before allowing the batch's parallel suites to proceed. Other runs queue behind the batch. The file's existence is never treated as ownership, and nobody deletes an apparently stale lock to acquire it. Windows releases the exclusive handle when its holder exits; a direct Node run also closes its helper's input pipe when Node exits.

`node browser-lock.js` checks exclusion between independent PowerShell and Node processes, holder reporting, valid and stale tickets, and recovery after killing either holder. It uses an isolated temporary lock and opens no browser.

A package-wide or full Go gate takes the same lock through `pwsh scripts/go-gate.ps1 <go test arguments>` from the repository root, for example `pwsh scripts/go-gate.ps1 ./cmd/dinah-desktop ./internal/... -count=1`. It waits behind any browser or Go run, printing the holder every five seconds, runs `go test` with the arguments it is given and returns go test's exit code. Everyone runs package-wide and full Go gates through it; a single-test run may skip it.

The underlying contracts are documented by Microsoft in [FileShare](https://learn.microsoft.com/en-us/dotnet/api/system.io.fileshare) and [file handle lifetime](https://learn.microsoft.com/en-us/windows/win32/fileio/file-handles). Node's exclusive-create flag tests whether a file exists, so it is unsuitable for this handle-lifetime lock.

## The Shared Stub

`lib/page.js` is the one stubbed window the suites share. `launch()` starts Edge, routes `http://dinah.test/` to the page's files, stubs `window.runtime` and a default `window.go.main.App` whose `State` answers from `window.__state`, records every call in `window.__bridgeCalls`, collects page errors and browser dialogs, and waits until the page is ready. A suite passes only what is its own: `state` to merge into the default State, `app`, a standalone function run in the page whose result is laid over the default App, `init` for anything else the page needs before Dinah's scripts, `viewport`, and `ready` (`'ui-state'` by default, `'workbench'`, `'chat'`, or a function). It returns the page with `ok`, `emit`, `calls`, `shot` and `close`, and `close` prints the "no page errors" line and sets the exit code. The comment at the top of the file is the reference. Every suite uses it (only `render-scheduler`, which runs the scheduler in Node without a browser, does not), so a bridge method changing shape is one edit. A suite that needs more than the fixture offers, such as another route, a second browser or a clipboard permission, passes `goto: false`, adds what it needs on `t.context`, and navigates itself.

## The Suites

### Agents and Providers

`agent-providers` covers a Codex-only installation, choosing between two Devin command lines, Claude adapter setup and its API key, Ollama not running, choosing and resetting an executable (including a cancelled native picker and validation errors), managed sign-in and connection checks, a new chat's provider and model, and changing the default from Settings and the status menu. The Go tests verify persistence and rejection of unavailable overrides without fallback.

`provider-continuation` verifies reviewed handoffs between providers, retained drafts and attachments, idle-only switching, failure recovery, restored history and old session links. It also delivers notifications before the continuation reply and verifies that early text survives and subsequent chunks and turn completion are consumed once.

`provider-usage` keeps a chat's context apart from account usage and checks reported quota windows and reset times, the status menu's usage line and its Usage… row, the usage page following the focused provider, draft chats, provider changes, unknown values and API-key billing. It also checks that a late response cannot replace another chat's usage display.

`model-prices` checks Devin's model prices (DESIGN.md, Devin Model Prices). A model with a price shows it as a quiet second line in Choose the Model, a model without one shows its name alone, prices arriving while the list is open draw it again, and with no prices the list's markup is exactly what it was before prices existed. It also checks the price of the chat's model under Usage, the price on the model menu's row, and the note in Settings › Agents › Devin.

`provider-model-prefs` checks separate model defaults and favorites for each provider without changing the application's default agent. It covers cached offline preferences, failed reads and writes, provider switches during pending operations, empty catalogs and the dialog on a narrow display.

`provider-restore` checks existing chats reopened from Workspace and restored workbench windows after the application defaults change. It covers provider and model retention, options arriving before metadata, provider-specific skills and catalogs, new-chat defaults and visible preference-save failures.

`prefs` checks model defaults and favourites set from the chat, and `signin` and `restart` check signing in to Devin and restarting it from the status area.

### Chat

`session-recovery` checks that provider load failures retain local history, identify the correct provider, allow full history export and continue into a fresh thread without losing the draft or sending a model turn during recovery.

`chat-clipboard` checks screenshot files, clipboard file items without a files list, mixed text and image data, paste outside the composer, chat ownership and outgoing image payloads. It uses synthetic paste events and does not change the operating system clipboard.

`background-tasks` checks that a chat whose agent has work running after its turn shows a Background chip with the number of tasks, beside Working when a turn runs too, lists the tasks under its state line with Stop where the agent allows it, and shows a turn the agent starts of its own as Working. `subagents` checks a chat's subagents: the row in the chat where the agent handed work to one, kept current; the Subagents chip and the state line while they run; the Subagents menu button in the composer's Turn group, absent until a subagent starts, with the count, a menu of the subagents with their states that opens each one's window, the same submenu under Actions, keys, and its rows kept when a narrow window folds the group; the tree from Show All Subagents with a nested subagent under its parent, times that tick, and Stop only where the agent allows it; one subagent's window, read-only with no prompt box, drawing its kept work and then live messages and tool calls without repeating what it already holds; the disconnected state as an unknown outcome with a glyph of its own; and a subagent's question in its chat saying which subagent asks. `chat-windows` opens chats as independent windows across workspaces. `chat-saved-restore` restores saved chats into windows while state loads late, retitled windows among them. `chat-scrollback` checks that a long chat keeps its latest 600 rows with a marker that opens the chat log, that huge tool output is cut, that terminal colours are drawn, and that a Markdown table in a message is drawn as a table with pipes kept in their cells.

`delivered-messages` checks that messages the bus delivers into a chat are cards of their own rather than the person's messages: a reply from another chat with its status as a glyph and a word, another agent's message, and two messages delivered in one prompt as two cards, each body drawn as Markdown and timed when it was sent. A long message is folded behind PUDL's disclosure and opens from its toggle. A delivery event from before deliveries were kept is still drawn as a message from the bus. The same cards come from a chat's saved copy and from a replay Dinah recognised, and Export Chat writes each card under its sender.

`composer-fold` narrows a chat window until the composer's Agent group folds into one button, checks that its menu keeps the provider, the mode and the model with their choices ticked and that a choice made there is the chat's, and widens the window until the group unfolds.

`chat-approvals` checks permission choices beside the originating chat's composer while Notifications is closed, natural sizing, scrolling within half the output space, collapse when empty, shared answer submission, retry after a failed answer, cancellation, provider shutdown and focus isolation between chats.

`chat-streaming` checks that 5,000 command-output updates stay within a small layout budget, preserve terminal colors, escaped text, output bounds and final status, keep background output in its own chat and flush output after history replay. It checks typing during two simultaneous command streams and catches up a minimized chat when restored.

`render-scheduler` checks the shared queue with a controlled clock, including coalescing, yielding at the frame budget, fairness between views, visibility, cancellation, reentrant invalidation, error isolation and the absence of idle frames.

`applet-mentions` checks Add > Windows and Applets and the `@` picker: pending, failed and retried discovery, keyboard and Tab selection, complete-token matching, multiple applets, built-in providers, tool windows with and without an applet, the workspace's other open chats, per-chat drafts, the native file picker and narrow menus. Selection never invokes an action. Its Send stub records attachment metadata; Go attachment tests separately verify registered identities and rejection of unavailable providers. It also includes Settings in its discovery and selection checks.

`parity` checks `@` mentions of tool windows written into the text and sent, removed mentions left out, paste and drop attachments, Edit and Quote, folded tool-call runs and per-chat drafts. `next4` checks the Add menu, personal templates (`/`, save, rename, delete), link and file chips in answers, and a turn's time, context and cost with the usage menu and usage page.

`app-skills` checks the `/` skill pop-up and the queue and steer controls. `app-form` shows a skill's argument hint as a form: the Editor's document as chat context, one field per parameter, required fields, preview, Enter and Ctrl+Enter sending, choices, flags and tick boxes, Esc back to text, and hintless skills inserting. `app-ui3` covers the workspace chooser, the sessions list, new and background sessions, renaming and hiding chats, and `app-ui4` covers long history replay and the model controls.

`headsup` checks that failed tools stay out of Notifications, that their per-chat history summary requires opting in, and that review, dismissal and retention limits work. It leaves rejected steps out and keeps turn errors after the agent stops. `diagrams` draws d2 and Mermaid blocks in chat messages, unfinished and broken ones among them, with links to files. `times` checks chat times and the reopened history of earlier chats, and `pin` checks pinning chats.

### Documents and the Editor

`editor` opens a workspace file in Split with the cursor in its text and checks typing kept in the draft and saved after a pause, changes made by an agent, another program or on disk (Take Theirs, Compare, Bring In), suggestions with Edit, Accept, Go To and Reject All, the chat's document and selection context, views kept across a reload, New Document and Save As with read-only places, taken names and New Folder, the Split handle, unsaved drafts in Find and Open, the Open dialog and From Your Computer, the Edit, Insert, Format and Review menus, saving a copy to the computer, Preview opening to read, automatic saving stopping and restarting, images and attachments, the Visual page, Find and Replace, and the formatting toolbar in Text.

`page-view` checks the Editor's Visual view, where the whole document is one editor. It covers typing reaching the text with only its block changed, the arrow keys and Enter across blocks, Undo, Ctrl+B, code blocks edited in place, the / menu and Edit › Insert, Edit › Table, the folding formatting toolbar, the Editor's commands run for an agent through the page bridge, Save taking the last typing, a change made elsewhere read into the page, Find and Replace, suggestions on the page, Split editing on both sides, and Ctrl+E going on to Text and Preview. Its fixture reads and writes the document as `pageview` does, simplified; Go's `pageview` tests cover the real conversion.

`diff` checks the Editor's diff view, opened with `window.dinahDiff.open` on fixture texts: the window's title and detail, the inline view with both sides' line numbers and the sign column, the hunk strips with their unified diff headers, the file actions in the head bar with one primary and Edit the File for a workspace file, a hunk action and a file action reaching the opener with the right ranges, tick boxes enabling the line button and a line action reaching the opener with the ticked lines, a danger action asking first in Dinah's dialog, the side by side view with the sides aligned at each hunk and the layout kept in the window's state, a second open with the same key reusing the window, a diff with no actions, the theme switching, Next Change, and screenshots of both layouts and the dark theme.

`page-clipboard` checks Cut, Copy and Paste in the Editor with the keys and the Edit menu, on the page, in the text and in a box: copied blocks keeping their form (code, tables, lists, emphasis), HTML from another program keeping its structure, plain text read as Markdown, pastes into table cells, Undo of a paste or cut, and the menu's Paste reading the clipboard Go reads.

`documents-menu` checks the Documents menu drawn from the command registry: its rows, keys and order, the current document (the Editor in front, else the one most recently in front), Save from inside the text and with the Workspace in front, Recent and Show All…, keys refused while a dialog is open, Window's Ctrl+F4 and Ctrl+F6, Keyboard Shortcuts on F1, command search, Ctrl+P then Enter, the In Progress pane's drafts and changed documents, and the Editor's own row under Open New Windows with Reset. `in-progress-intro` checks that a layout saved before In Progress existed gets the pane once, and never again after it is closed.

`document-windows` checks that a document listed in Find and Open, or a document window kept in an older layout, opens in the Editor, and that the resource viewer keeps external links, table-of-contents links, page breaks, problem links and a preview following the application palette in dark mode. `document-diagrams` checks D2 and bundled Mermaid diagrams in documents: expansion, render errors joining the problem list, hostile SVG isolation, unique SVG IDs across documents, Refresh retrying a failed render, a narrow dark layout, and late or closed renders being discarded. `view-state` restores a document window's scroll position and expanded problems list after its content loads late. `media-viewer` checks the media viewer for large images and diagrams in a resource preview. `document-links` checks paths in a chat's messages (docs/plans/document-links.md): a relative path, an absolute path, a path in inline code, a path with a line, a folder, a picture and a PDF each become a link with the glyph and tooltip of where it opens and open that place through a stubbed resolver, while a path to nothing, a path in a fenced block and words that look like file names stay text; it also streams the message five characters at a time and compares it with the whole text after every chunk.

### Workspaces, Sources and Files

`workspace-identity` uses a persistent backend fixture to check workspace names, stable-ID selection, duplicate-name disambiguation, Unix case and backslash paths, Remove Workspace…, the frozen Pinned group and reload. It verifies that renaming leaves open chats and their drafts unchanged, and that failed renames or unavailable folders preserve the previous selection and saved records.

`workspace-sources` checks local-folder sources in the Workspace pane: tree typography, source menus and keys, lazy paged listing, an unavailable source recovering through Retry, empty folders, refresh, folder actions, selection without fetching a preview, restore on reload, read-only resource windows keeping their owner after a workspace switch, Add Source… with a cancelled picker, live labels and confirmed removal, with narrow and dark screenshots.

`new-workspaces` checks New Workspace… on a chosen main folder, including a cancelled picker and a folder that is already a main folder, then empty workspace creation, generated names, identity-based selection, rename and reload, failed creation, and hidden internal execution paths. `wsmenu` checks the Workspace pane's chooser and its Actions menu.

`files` checks the Files browser against the shared virtual mount adapter: listing, New Folder, uploads with conflict choices and drops, previews with attachments and diagrams, late previews and diagrams rejected, Show in Folder, paging, native-state restoration, narrow layouts, refusal of alias replacement, retrying an offline source, cancelled opens and New Terminal Here. `files-renames` checks that an open Files folder follows a workspace rename and that a saved location resolves its stable reference after a restart.

`resource-attachments` checks attachment URLs and the lightbox for resources from two local sources, missing images joining Problems, refresh, direct image, SVG, D2 and Mermaid resources, and closed windows ignoring late image events. `resources-live` checks the resource tree's live labels, disk watching, source identity, Home lookup, the launcher registry and the generic applet commands with their workspace scope.

### Terminal

`terminal-shells` checks a Terminal running a real shell: the shell chosen in Settings sized to its window, output drawn even before the start call returns, typed text sent whole and in order, size changes, Ctrl+C copying a selection and otherwise interrupting, Ctrl+Shift+V pasting, Enter starting an ended shell again, closing the window closing its shell, Tools › New Terminal With, and choosing the default shell in Settings. The Go tests in `internal/pty` and `terminal_shells_windows_test.go` run real programs in a pseudoconsole.

`terminal` checks Dinah's own shell over a virtual file system: pipes, redirection, wildcards, file commands, scripts in ~/bin, nano, less, completion, tools, read-only workspace catalogs, `edit -g`, Ctrl+C, per-terminal state and restore. `terminal-sh` checks the sh interpreter, and `terminal-lua` checks the lua command, the standard /bin scripts, ~/bin precedence, io and require, the applet bridge, log and Ctrl+C.

### Skills and Applets

`skills-browser` checks the Skills browser (grouping, scope filter, duplicate names), the Skill window's Editor, the Parameters designer with undo, redo and row commands, Check problems, the rooted Files tab, Use in Chat…, Run opening the skill's chat window, Copy to My Skills, Rename…, refresh after changes and restored state.

`applets` checks human requests in Notifications (a stale version and retry, required booleans, completion into a result notice, duplicate completion, reload, dismissal, cancellation against a stale refresh), the applet artifact viewer and its export, opening a result from its source workspace, posted notifications as notice and toast, narrow fits, the chat composer fit and workspace group headings.

### The Bus

`bus` checks the Group Chats pane, named so in its title and the View menu, with New Group Chat… and New Message… on its toolbar: its inbox, newest first with the unread message marked and a group's message that mentions the person; its group chats with their members and unread counts; every chat by workspace, closed ones too, each with its state as a glyph and a word; a new message and a refused one; a message opening its whole thread and a reply in it. A group opens in its own window with its members as chips, each with its state and a menu that opens the chat or takes it out after a question, then Add Chats…; its conversation under the senders' titles with mentions marked and a line for a chat added; drawing again when the bus changes; and its box, whose @ list offers the members, everyone and Add a Chat…, puts a mention in whole, takes it out whole with Backspace, and sends the list of chats mentioned. The group's menu has Delete Group… and no Leave. The picker is a window with an address that keeps its choices: it lists every chat by workspace, filters them, notes a closed chat ticked, asks an untitled chat for its title in place, shows the name the group would take, and starts the group. A chat window's Actions menu has Start a Group Chat…, which opens the picker with the chat ticked, and Add to a Group Chat…, a submenu of the groups the chat is not in with New Group Chat… at its foot. A trusted link is made with the offered limit, its limit turned off only after a warning, and removed after a question.

`group-chat-composer` checks that a group's window has a chat's box and draws its messages as a chat does. A message is drawn from its Markdown with link chips that open in the browser, a d2 block goes to the diagram engine, its time is in the regional format, and a message containing an image tag with a handler, a script tag and script addresses is shown as text with nothing run. Mentions stay marked. Copy copies the Markdown as written, from the head or from the foot that a long message shows, and Quote puts the message in the box as a quotation. The box grows with its text and stops at the limit a chat's box has. Up and Down bring back the person's own messages in the group, a mention with them, and leave a caret inside several lines alone. Enter sends or makes a new line as Settings › Chat says, and Send's tooltip follows the setting. The list stays at its end for a new message while the reader is there and keeps the reader's place after scrolling back, and messages already drawn are kept. A draft and its mention survive closing and reopening the window, a sent message leaves none, and Edit puts the person's message back with its mention as one unit.

`drafts` checks that a chat's and a group's unsent drafts outlast a restart, which it simulates as `ui-state` does, with a new browser context reading the state the last one saved. A draft is saved as it is typed without being asked, and after the restart the box shows it, Up brings back the last message sent and Down returns to the draft. While Up has an earlier message in the box, the draft it set aside is what stays saved. A sent message drops the saved draft. A group's draft keeps the chats it mentions, sends with them after the restart, and keeps its mention as one unit that Backspace takes out whole, and deleting the group drops its draft.

### Notifications

`notifications-pane` checks that every toast is kept as an entry under Notifications with its severity, newest first and after the questions, that a repeated report is one entry, that the count is of questions and errors, the type and text filters, a document's report reopening its draft, Clear All leaving the questions and dismissing applet results, and narrow report cards. `notifications-persistence` checks that notifications persist and reload across sessions.

`notifications-popover` checks the list that the taskbar's Notifications item opens, which is drawn from the same model as the pane. It covers the order (questions first, then the rest, each newest first, as in the pane), a glyph and a colour for each type, the two-line clamp with the whole text in each row's name and tooltip, a question opening its chat with the question in view, a report opening the pane at its card, the arrow keys and Escape, the empty state, the badge, rows added and answered while the list is open, and Show All and Clear All. Its screenshots show the list in both themes and the pane.

`notification-severity` checks failure, approval, timeout, cancellation and result cards in light and dark themes, with semantic classes, colour tokens and distinct PUDL status glyphs, and that a burst of applet changes makes one load.

### Shell, Windows and Settings

`ribbon` checks PUDL's ribbon: off until the topbar's toggle turns it on, then each menu title a tab whose rows are tools that run their commands, danger rows drawn as danger buttons, and the menus back when it is turned off. It checks the menu bar's documented behaviour only, never the ribbon's own markup.

`update` checks Restart When Chats Finish: the tray's Update item appearing for a newer build, restarting once turns end with the count of working chats shown, cancelling, Restart Now asking first, and the Dinah menu's row.

`menubar-fold` checks that the menu bar's titles show their glyphs and words in a wide window, fold to their glyphs with their names and tooltips kept in a narrower one, and collapse to one button only in a very narrow one.

`workbench` checks the first-run layout, the Workspace pane's scoped chats, the Skills and Chats browsers' docking, menus, scope and filters, the Tools launchers and their search, Go To on a recent document, Settings as a window, taskbar hit areas and wrapping (wait for the resized taskbar layout before measuring row positions), bottom-dock tab strips, the Window menu and the narrow layout.

`taskbar-stacks` checks that the taskbar stacks a workspace's windows under the workspace's name and folder glyph, keyed by its ID, with a list of its windows that opens from the stack. The workspace chosen in the Workspace pane stacks like the others and pressing its stack pins the list, New Chat goes with the chosen workspace, and a window of no workspace (Settings) stays an ordinary button. An ID with characters a key cannot have becomes the same safe key every time. A rename keeps one stack and changes its label, and a chat moved to another workspace joins that workspace's stack. It then starts the page again with the layout only in the saved state, as the program starts, and finds the restored windows stacked with no workspace chosen first. It saves screenshots of the taskbar and of a stack's list.

`window-switcher` checks Ctrl+Tab, which switches windows as Alt+Tab does on the Windows desktop: the two most recent windows, a longer walk while Ctrl is held, docked windows left out, a minimised window restored, Escape, and Ctrl+PageDown still going through every window in order. `keyboard` checks the Editor's keys, F6 and Ctrl+Shift+M, the F1 reference and its Keys For list, Notifications and window keys, command search, and the menu bar's keys, letters, theme rows, destructive and disabled rows, and Alt and F10 access. `menudim` checks dimmed menus and the Find and Open frame in each theme.

`settings-ui` checks the four Settings sections (Appearance, Agents, Chat, Diagnostics), their PUDL cards and navigation, segmented-control keyboard operation, pending saves and rollback after a failure, live preference updates, out-of-order responses and per-section scroll restoration. Its screenshots cover narrow floating windows.

`ui-state` uses a native-store fixture shared across fresh browser contexts. It verifies window geometry, docks, scroll restoration after delayed document and chat loads, startup ordering, new-window preferences, explicit address precedence, migration of named preferences, serialized writes and persistence failures. The fixture excludes unrelated browser storage from migration. `shell-settings` checks the new-window preference, the status menus and the native-close save acknowledgement.

## Writing a Suite

Copy the nearest existing suite. Stub every bound method the page calls at start (`State`, `Sessions`, `SkillsAt`, `RecentDocuments` and so on); a missing one throws and shows as a page error. Wrap calls to back-end methods that a suite may not stub in `Promise.resolve().then(...)` in the page code, as the existing code does.

Item windows use workspace-explicit methods such as `OpenSessionAt`, `NewSessionAt` and `SkillsAt`. Chat controls have an instance-specific ID and retain their template ID in `data-view-id`, so scope selectors to the owning window. The older suites use `lib/windows.js` to raise a control's owner through the public workbench API before interacting with it, and `lib/open-document.js` opens a fixture's recent document by its title through Find and Open. `lib/commands.js` presses toolbar and menu commands whether their toolbar group shows them or has folded them into a menu.

Workspace uses separate lists that share the browser renderers. Tests scope commands to their owning window, because New Session appears both in Workspace and in the Chats browser.

`provider-turn-lock` checks that reopening running Codex, Ollama, Claude and Devin chats disables provider selection from the initial snapshot, that background turn changes update the owning window, and that partial metadata preserves the lock.

`steering-recovery.js` verifies that unresolved submissions are never resent automatically, recovered attachments survive copying to a draft, local rejection restores input, and ambiguous delivery requires an explicit copy.

`responsiveness` checks bounded rendering of oversized messages, page navigation during streaming, typing during saved-history reconstruction, and the optional timing diagnostic, including clearing and disabling collection. Its browser fixtures do not establish native WebView2 input latency.

`appearance` checks the Mineral palette in both modes, text and syntax contrast across application surfaces, filled-action and warning-badge contrast, input boundaries, tinted document/composer surfaces, inherited menu colors, draft preservation, system mode and the matching desktop/bus stylesheets.

The `theming.js` suite covers the appearance library, isolated drafts, font overrides, import/export and failed saves under the shell content security policy. Editor and terminal suites also verify live font changes without recreating their instances.
