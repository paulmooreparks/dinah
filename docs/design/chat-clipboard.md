# Chat image paste

Paul reported that image paste into chat no longer worked on 24 October 2026. The handler accepted only events on the prompt, required a nonempty clipboard files list and returned without attaching images whenever plain text was also present. Those conditions left file-item clipboard representations and mixed text/image content unsupported. No operating system clipboard payload was captured during the reported failure, so the investigation does not identify which condition occurred in that instance.

The document's paste listener routes the event to the front chat. That controller verifies the target belongs to its window and excludes other text fields and nested dialogs. Pasting into the chat's log or window controls can attach a screenshot without requiring focus in the composer. Editor paste retains its own event handling. Closed or background chats do not receive the front chat's paste.

The handler reads the files list first and falls back to file-kind clipboard items through `getAsFile()`, avoiding duplicate attachment when both representations exist. Mixed text and image paste attaches the image while retaining native text insertion when the prompt owns the event. Plain text-only paste keeps its normal behavior. Existing attachment limits, previews and provider image payloads remain in force. File read errors produce a visible notice, and an asynchronous read cannot add an attachment to a deleted controller.

The browser suite exercises these cases with synthetic file and clipboard events, including two open chats and outgoing image bytes. This verifies DOM routing and attachment construction. It does not establish what a particular native clipboard owner supplies to WebView2; native paste remains part of acceptance testing.

After restarting into the corrected build, Paul still could not paste. A regression fixture reproduced another routing failure: the window key can retain a workspace identity or folder spelling different from the session controller's key. The front chat now resolves its controller through the session's view, falling back to the window key only when no view exists. This follows the existing rule that a session has one controller, independent of its folder spelling. The fixture seeds session metadata with a different folder spelling before opening the window and checks that paste reaches that session.

The production window enabled Wails' documented default context menu option so the prompt had the browser's editing commands, including clipboard actions, deletion and selection. Since PUDL 0.109.0 that menu is off, and the prompt's edit menu has PUDL's rows, given by the chat box itself so that Paste can attach a picture (below). Ctrl+V and Shift+Insert remain native shortcuts; the regression checks that page handlers do not cancel either chord, and that a right-click on the prompt opens PUDL's edit menu. These event checks do not certify native clipboard contents or the operating system menu itself.

## Host clipboard for PUDL's edit menu

PUDL 0.109.0 adds its own edit menu (Cut, Copy, Paste, Select All) for plain fields, opened by a right-click, Shift+F10 or the Menu key. The menu reads and writes the clipboard through `window.pudlClipboard`, and PUDL's contract (its `docs/EDIT-MENUS.md`, "Host clipboard") leaves the native clipboard to the host. Every document that loads `pudl-menu.js` must call `window.pudlClipboard.setTransport({ readText, writeText })` itself, because a transport set in one window does not reach another window's accessor. A refused operation must reject, and PUDL never retries it against the browser's clipboard.

Dinah's side is `cmd/dinah-desktop/frontend/clipboard-transport.js`. The main page and the document page both load it as a deferred script directly after `pudl/pudl-menu.js`, so the accessor exists when it runs. It uses the two clipboard calls Dinah already has, and the shell server bridges both of them for document windows (Table 1). No page loads `pudl-menu.js` later or by script, and the page's own frames load no scripts, so one transport for each page is all the contract asks for.

**Table 1** *Clipboard Transport Methods and the Calls Behind Them*

| Transport method | Dinah call | Result | Failure |
| --- | --- | --- | --- |
| `readText()` | `App.ClipboardRead()` | Its `text`, or `""` when the clipboard has no text | The call's error, as an `Error` |
| `writeText(text)` | `window.runtime.ClipboardSetText(text)` | No result | A `false` return or the call's error, as an `Error` |

*Note.* `ClipboardRead` reads through Win32 on Windows and through Wails elsewhere; the edit menu uses only the plain text.

The script feature-detects the accessor. With a PUDL older than 0.109.0 there is no `window.pudlClipboard`, and the script does nothing; it never defines one. Running it twice sets the transport once, since each `setTransport` call also retires the menu's pending edits. The browser suite `clipboard-transport` checks the transport against the PUDL 0.109.0 that Dinah ships, and checks the first case by serving the menu script with its accessor taken away.

Dinah took PUDL 0.109.0 and yavc.app v0.2.117, which bundles it, on 25 November 2026. The documents that load `pudl-menu.js`, and what each does for the transport, are listed in Table 2.

**Table 2** *Documents That Load PUDL's Menu and Their Clipboard Transport*

| Document | Transport |
| --- | --- |
| The main window's page (`index.html`) | Dinah's, from `clipboard-transport.js` |
| A document window's page (`document.html`) | Dinah's, from `clipboard-transport.js` |
| The yavc.app applets | The page's own; Dinah mounts them in the page (`content-windows.js`), not in frames |
| The bus's web page (`internal/web/templates/layout.html`) | PUDL's default, the browser's own clipboard |

*Note.* The bus's web page is opened by `dinah open` in an ordinary browser, where there is no App to call and the browser's clipboard, with the browser's own permission rules, is the right one. The slide show page (`show.html`) does not load `pudl-menu.js`. An extension's frame is sandboxed, gets no PUDL from Dinah and cannot reach the App, so no transport is given to it.

yavc.app's applets send plain text through the accessor and keep rich HTML on `navigator.clipboard.read` and `write`, as PUDL's guide leaves rich formats to the editor (yavc.app `docs/pudl-upgrade-0.109.0.md`).

### Surfaces with their own editing state

PUDL adds its generic Cut, Copy, Paste and Select All to any right-click on a field, an editable region or selected text, unless the surface claims the edit menu with `event.detail.handlesEdit()` and gives rows of its own. Dinah's frontend was searched for surfaces that keep their own editing state, and Table 3 lists what each does.

**Table 3** *Dinah Surfaces and Their Edit Menus*

| Surface | Edit menu |
| --- | --- |
| A Terminal running a real shell (`terminal-shell.js`) | Claims it: Copy (Ctrl+C), Paste (Ctrl+Shift+V) and Select All, on xterm's own selection |
| Dinah's own Terminal (`terminal-applet.js`) | Claims it: Copy, Paste and Select All, with no shortcuts shown, since Ctrl+C there stops a command |
| The diff view (`diff-applet.js`) | PUDL's generic rows; the CodeMirror view is read-only and not editable, so its selection is the page's and Cut and Paste are unavailable |
| The yavc.app editors | yavc.app's own rows (v0.2.114 and later) |
| A chat's box and a group's box that takes files (`composer.js`, `editMenu`) | Claims it: PUDL's four rows, whose Paste attaches a copied picture (below) |
| Plain fields, a group's box without files, and selected text | PUDL's generic rows |

*Note.* xterm keeps its selection apart from the page's and takes typing through a hidden field, so the generic rows would copy nothing and paste into that field. Both Terminals read and write the clipboard through `window.pudlClipboard`, for their keys and their window commands too, and leave Shift+F10 and the Menu key to the page through xterm's documented custom key handler, so those keys open the menu instead of reaching the shell.

### Menu Paste attaches a picture

PUDL's generic Paste reads text only, so once the browser's menu was off, a right-click Paste in a chat box could no longer attach a picture the way Ctrl+V does. Paul approved restoring it with the stock look on 25 November 2026. The chat box and a group's box that takes files claim the edit menu and give PUDL's four rows themselves, with the stock ids (`cut`, `copy`, `paste`, `select-all`), labels, display-only shortcuts (Mod+X, Mod+C, Mod+V, Mod+A) and icons. They follow PUDL's rules for those rows: Cut and Copy need a selection, Cut deletes only after the clipboard write succeeds, text changes go through `document.execCommand` `insertText` and `delete` so that native Undo keeps working, an edit whose box or selection changed while the clipboard was being read is dropped with an error, and errors go out as PUDL's `pudl:edit-error`. Nothing else in the menu changes.

Paste first asks the host for a picture with `App.ClipboardImage()`. When one comes back, the page builds a `File` from its bytes and attaches it through the same path a Ctrl+V picture takes, and no text is pasted. When there is no picture, Paste reads text through `window.pudlClipboard.readText()` and inserts it with `insertText`. A failed or refused picture read is not shown to the person: it is logged to the console and Paste falls back to the text (a ruling of 26 November 2026). An error appears only when the text cannot be read either, as with PUDL's own Paste.

`ClipboardImage` has one shape on every system, agreed before macOS and Linux are built (Table 4).

**Table 4** *The ClipboardImage Host Method*

| Result | Meaning |
| --- | --- |
| `{ type: "image/png", data: <PNG bytes> }` | The clipboard holds a picture; Go encodes `data` in JSON as base64, which the page decodes |
| `{ type: "", data: null }` | The clipboard holds no picture |
| An error | The read was refused or failed |

*Note.* The method is a query in `pageMethods` (`page_api.go`), next to `ClipboardRead`.

On Windows, `clipboard_image_windows.go` opens the clipboard with the same retry as the text read and reads `CF_DIBV5` when present, otherwise `CF_DIB`, both standard formats in Microsoft's "Standard Clipboard Formats". Windows synthesizes each of these from `CF_BITMAP` and from each other, so a picture copied in any of the three can be read. The unregistered "PNG" format that some programs also write is not read, because Microsoft does not document it. `clipboard_dib.go` converts the bitmap to PNG with Go's `image/png`, supporting the forms BITMAPINFOHEADER and BITMAPV5HEADER document without a palette (Table 5); anything else is refused with an error that says what the clipboard held, which the page then treats as no picture.

**Table 5** *Bitmap Forms Converted to PNG*

| Compression | Depth | Alpha |
| --- | --- | --- |
| `BI_RGB` | 24 bits | Opaque |
| `BI_RGB` | 32 bits | Opaque; BITMAPINFOHEADER says the fourth byte is not used |
| `BI_BITFIELDS` | 32 bits, masks after a BITMAPINFOHEADER or inside a V4 or V5 header | From a BITMAPV5HEADER's `bV5AlphaMask` only, otherwise opaque; opaque too when every pixel's alpha is zero |

*Note.* Rows may be bottom-up (positive height) or top-down (negative height). Palettes, 16-bit bitmaps, JPEG and PNG compression, and headers of undocumented sizes are refused. Alpha is taken as straight, not premultiplied. Some programs write 32-bit bitmaps with an alpha mask but leave the alpha bytes unused, and a fully transparent paste is never what the person copied, so a picture whose alpha is zero everywhere is made opaque (a ruling of 26 November 2026). The rule depends on no undocumented behaviour; it only refuses to produce an invisible picture. A picture with any non-zero alpha keeps its alpha as read.

On macOS and Linux, `ClipboardImage` returns no picture for now, so menu Paste pastes text there. macOS (NSPasteboard) and Linux (GTK 3's `gtk_clipboard_request_image`) follow in their own branch with the same method and result.

The Go tests convert synthetic bitmaps of every supported form, decode the PNG and compare pixels, and check each refusal. The browser suites `chat-clipboard` and `group-attachments` stub `ClipboardImage` and check the four rows, Cut and Copy disabled without a selection, a picture attached by menu Paste, text pasted when there is no picture and taken away by Undo, the quiet fallback when the picture read is refused, the error when the text read fails as well, and Cut with Undo. What a real program puts on the Windows clipboard, and whether WebView2's Undo history matches the browser's here, can only be checked on an installed build.

The desktop windows set Wails' `EnableDefaultContextMenu` to false (`main.go` and `document_window.go`), by Paul's ruling of 24 November 2026 that no browser control ever appears. The slide show window and the start-up failure window never set it, and Wails leaves it off in a production build. Where nothing adds a row, a right-click now opens no menu. Physical right-click suppression, the native clipboard, Undo after a menu edit, Shift+F10 and the Menu key can only be checked on an installed build.
