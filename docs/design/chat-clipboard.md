# Chat image paste

Paul reported that image paste into chat no longer worked on 24 October 2026. The handler accepted only events on the prompt, required a nonempty clipboard files list and returned without attaching images whenever plain text was also present. Those conditions left file-item clipboard representations and mixed text/image content unsupported. No operating system clipboard payload was captured during the reported failure, so the investigation does not identify which condition occurred in that instance.

The document's paste listener routes the event to the front chat. That controller verifies the target belongs to its window and excludes other text fields and nested dialogs. Pasting into the chat's log or window controls can attach a screenshot without requiring focus in the composer. Editor paste retains its own event handling. Closed or background chats do not receive the front chat's paste.

The handler reads the files list first and falls back to file-kind clipboard items through `getAsFile()`, avoiding duplicate attachment when both representations exist. Mixed text and image paste attaches the image while retaining native text insertion when the prompt owns the event. Plain text-only paste keeps its normal behavior. Existing attachment limits, previews and provider image payloads remain in force. File read errors produce a visible notice, and an asynchronous read cannot add an attachment to a deleted controller.

The browser suite exercises these cases with synthetic file and clipboard events, including two open chats and outgoing image bytes. This verifies DOM routing and attachment construction. It does not establish what a particular native clipboard owner supplies to WebView2; native paste remains part of acceptance testing.

After restarting into the corrected build, Paul still could not paste. A regression fixture reproduced another routing failure: the window key can retain a workspace identity or folder spelling different from the session controller's key. The front chat now resolves its controller through the session's view, falling back to the window key only when no view exists. This follows the existing rule that a session has one controller, independent of its folder spelling. The fixture seeds session metadata with a different folder spelling before opening the window and checks that paste reaches that session.

The production window enables Wails' documented default context menu option so the prompt has the browser's editing commands, including clipboard actions, deletion and selection. Custom PUDL menus retain their own handling. Ctrl+V and Shift+Insert remain native shortcuts; the regression checks that page handlers do not cancel either chord or the prompt's context-menu event. These event checks do not certify native clipboard contents or the operating system menu itself.

## Host clipboard for PUDL's edit menu

PUDL 0.109.0 adds its own edit menu (Cut, Copy, Paste, Select All) for plain fields, opened by a right-click, Shift+F10 or the Menu key. The menu reads and writes the clipboard through `window.pudlClipboard`, and PUDL's contract (its `docs/EDIT-MENUS.md`, "Host clipboard") leaves the native clipboard to the host. Every document that loads `pudl-menu.js` must call `window.pudlClipboard.setTransport({ readText, writeText })` itself, because a transport set in one window does not reach another window's accessor. A refused operation must reject, and PUDL never retries it against the browser's clipboard.

Dinah's side is `cmd/dinah-desktop/frontend/clipboard-transport.js`. The main page and the document page both load it as a deferred script directly after `pudl/pudl-menu.js`, so the accessor exists when it runs. It uses the two clipboard calls Dinah already has, and the shell server bridges both of them for document windows (Table 1). No page loads `pudl-menu.js` later or by script, and the page's own frames load no scripts, so one transport for each page is all the contract asks for.

**Table 1** *Clipboard Transport Methods and the Calls Behind Them*

| Transport method | Dinah call | Result | Failure |
| --- | --- | --- | --- |
| `readText()` | `App.ClipboardRead()` | Its `text`, or `""` when the clipboard has no text | The call's error, as an `Error` |
| `writeText(text)` | `window.runtime.ClipboardSetText(text)` | No result | A `false` return or the call's error, as an `Error` |

*Note.* `ClipboardRead` reads through Win32 on Windows and through Wails elsewhere; the edit menu uses only the plain text.

The script feature-detects the accessor. With a PUDL older than 0.109.0 there is no `window.pudlClipboard`, and the script does nothing; it never defines one. Running it twice sets the transport once, since each `setTransport` call also retires the menu's pending edits. The browser suite `clipboard-transport` checks both cases, the second against a PUDL build given in `DINAH_PUDL_BRANCH_DIST`.

The desktop windows still set Wails' `EnableDefaultContextMenu` to true (`main.go` and `document_window.go`), so a field with no PUDL menu keeps the browser's editing menu. That option is turned off as a later step, once PUDL 0.109.0 is released and taken into Dinah and yavc.app has adopted its edit menu. Turning it off before then would leave plain fields with no clipboard menu at all.
