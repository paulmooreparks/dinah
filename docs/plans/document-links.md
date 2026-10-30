# Document References Open Inside Dinah

Paul asked on 27 October 2026 that every reference to a document in a chat be a link that opens it inside Dinah. A path to a workspace document, or to any document, opens in the Editor. A file that belongs to another applet opens there, as an image or an SVG does in the media viewer. A file that no part of Dinah can show yet, such as DOCX, XLSX or PDF, opens as it does today. He asked for it while reading a reply that pointed him to a mockup's folder and screenshots by their paths, which he then had to find by hand.

Paul placed the request after the group chats work ([group-chats.md](group-chats.md)) on 27 October 2026 and approved starting it on 28 October 2026. It is built; the design as built is under Design and Progress below.

## What Dinah Did Before

A chat message turned two kinds of reference into links, which it drew as chips. A Markdown link to a relative path was resolved in the chat's folder by `OpenChatPath` in `cmd/dinah-desktop/open.go`, and a `file://` address was turned into a path. Both ended in `OpenPath`, which opens a folder in the system's file manager, opens a file of a known type with the program the system associates with it, and shows any other file in its folder. Nothing opened in the Editor or in another applet.

A path written as plain text or as inline code, which is how agents usually mention a file (`docs/plans/group-chats.md`), was not a link at all.

## What the Request Needs

- **Recognising paths in text.** A path in inline code or plain text, relative to the chat's folder or absolute, becomes a link when it points to a file or folder that exists. Checking that the file exists keeps ordinary words that look like paths from turning into links that lead nowhere.
- **Choosing where it opens.** A file of a type the Editor supports opens in the Editor, in the view Settings chooses for its media type. A file that an applet has claimed by its media type opens in that applet. A folder opens in Files. Anything else keeps today's behaviour. This needs one place in Dinah that says which part of the window handles which media type, which the Editor's per-media-type views may already be most of.
- **The workspace.** A path inside one of Dinah's workspace sources opens as that workspace's document, so drafts, Recent and pins work as they do when the document is opened from Files.
- **What the person sees.** A link to a document says which applet will open it, for example in its tooltip, so the person is not surprised by a file opening in another program.

## Questions to Settle

Dinah Development answered both questions on 28 October 2026 so that the work could go ahead. Each answer is a proposal, and Paul can reverse either.

1. Whether a path that does not exist yet, such as one an agent is about to write, is shown as a link that waits for the file, or as plain text. **Proposed: plain text.** The path becomes a link when the message is drawn again after the file appears, for example when the chat is opened again, or when the block it is in is drawn again more than ten seconds after Dinah last looked. Nothing polls for it. A link that waited would need Dinah to watch every path every message mentions, and a dead link that might come alive later is the thing the existence check is there to prevent.
2. Whether the same links work in places other than chats, such as the bus, notifications and the Terminal's output. **Proposed: every place that draws a message with the shared renderer.** That is a chat, a group chat, a thread, a subagent's window and a card for a message the bus delivered into a chat, and the files attached to a group chat's messages. Notifications and the Terminal's output are left for later; they draw their text in their own way, and each would need its own pass and its own folder to read relative paths from.

## Design and Progress

The work was built on 28 October 2026.

### One place decides where a file opens

`cmd/dinah-desktop/document_links.go` decides which part of the window opens a reference, in Go, so that the page, its tooltips and anything else that asks get the same answer. `ResolveDocumentLinks(cwd, refs)` takes up to 200 references at once and `ResolveDocumentLink(cwd, ref)` takes one; `cwd` is the folder of the workspace that wrote the message. Each answer says whether a file or folder is there, its absolute path, its media type, where it opens, the tooltip and PUDL glyph for that place, the line asked for, and the workspace resource it is when it is inside a workspace source.

The resolver decides in this order:

1. A folder inside a workspace source opens in Files, at its place there. A folder anywhere else is shown in the system's file manager.
2. A file of a media type that a part of the window has claimed opens there. The media viewer claims the pictures Dinah shows itself (PNG, JPEG, GIF, WebP and SVG, up to 20 MiB), so an SVG opens as a picture even though it is text. A claim comes before the Editor.
3. A file the Editor opens opens in the Editor. The rule is `editorOpens`, which Files' Edit command now uses as well, so there is one list: the text kinds in `textExtensions`, up to 1 MiB. The answer gives the view Settings chooses for the file's media type (Visual, Text, Split or Preview for Markdown), or Text for any other kind; the Editor opens in that view itself.
4. A file in a workspace source, of a kind that an enabled extension's view declares and that no part of Dinah opens itself, opens in that view. Files that open in an extension's view, below, describes the rule.
5. Anything else is left to the system as before, through `OpenPath`. A document of a kind in `openableExt`, such as PDF, DOCX or XLSX, opens in the system's program for it, and any other file, such as a program, is shown in the system's file manager.

A reference is a path relative to `cwd`, an absolute path, or a `file:` address. A slash before a drive letter, as Markdown often writes one, is dropped, and on Windows a path that begins with a single slash is read from `cwd`. A line suffix (`path:42`, `path:42:7`, `path#L42`, `path#L42-L50`) is set aside and returned as the line, unless a file whose name really ends that way is there. A folder, a file one of Dinah's own parts claims and a file left to the system have no line; an extension's view is given the line.

A file or folder inside one of Dinah's workspace sources, or in the home folder, `/bin` or `/lib`, is given as that workspace's resource, looked for first in the workspace whose folder is `cwd`. The Editor then opens its draft, so drafts, Recent and pins work as they do from Files. A path Files does not show, such as a hidden one, is a local file instead. The Editor opens a local text file outside every workspace source as a new document with the file's text, as Open's From Your Computer does.

`ReadLinkedImage(cwd, ref)` gives the media viewer the picture as a `data:` address. It reads only a reference the resolver sends to the media viewer, and only a file whose contents are a picture of a kind Dinah shows.

The claims are a registry (`claimMediaType`), so another applet can claim a media type in one line. The enabled extensions' claims are kept in the same registry, apart from Dinah's own, as the next section describes.

### Files that open in an extension's view

Paul approved on 28 October 2026 letting extensions claim media types, including the change to the public extension contract that it needs. A window view in an extension's manifest may now declare the kinds of file it opens in an optional `opens` member, a list of IANA media types each with the file extensions that have it, for example `{"mediaType": "text/x-diff", "extensions": [".diff", ".patch"]}`. The SDK (`dinah-extension-go`, release 0.2.0) defines and validates the member, and its `CONTRACT.md` is the public description; wildcards such as `image/*` are refused, as are `text/plain` and `application/octet-stream`, because Dinah decides by a file's extension and a wildcard would not say which files it covers.

`cmd/dinah-desktop/extension_claims.go` registers the declarations of every enabled extension whose package passes its integrity check. It reads them from the extension store the first time a link needs them, and again after every installation, update, enabling, disabling and removal, so a disabled, removed or damaged extension claims nothing and an update replaces its claims. Dinah decides between claims in this order:

1. Dinah's own parts come first. A media type one of them claims, such as the media viewer's pictures and SVG, and a kind of file the Editor opens stay with Dinah, so installing an extension never takes them away. A way for the person to give such a type to an extension is left for later.
2. Among extensions, the one enabled first opens a type that two of them declare. An update keeps an extension's place, and disabling and enabling it again puts it after the others. The store records when each extension was enabled (`Installation.EnabledAt` in the SDK); an installation recorded before that field existed counts from when it was installed.
3. A declaration never changes the media type of a file extension Dinah already knows. A file matches a claim when Dinah knows its extension as the declared type, or when Dinah has no type for its extension and the declaration lists it, so a claim of `text/x-diff` takes `.diff` files, which Dinah has no type for, and a claim that lists `.xlsx` under another type takes nothing.
4. Only a file inside a source of one of the person's workspaces opens in an extension's view. The view opens in the file's workspace and reads the file through the host's workspace services, which reach no other file; any other file is left to the system as before.

The resolver's answer for such a file has `opens` set to `extension`, the extension's and the view's IDs, and the view's navigation state, which is the JSON text of the SDK's `OpenState`: `{"open": {"path", "mediaType", "resource": {"workspaceId", "sourceId", "path"}, "line"}}`. Go builds the state so that the page passes it on unchanged. Pressing the link calls `dinahExtensions.openFile`, which opens the view through the same machinery as the `open-view` host command: a window of the view already open in that workspace is raised and receives the state as a `state` event, and otherwise a window opens in that workspace and the frame receives the state from `dinah.ready`. The frame reads it with `dinah.openedFile(state)`. The link's tooltip is "Open in" and the view's title, and its glyph is the view's glyph, or `app` when the view has none.

Settings › Extensions lists the file types each extension's views declare. When some of a type's file extensions open elsewhere, it shows a warning, with PUDL's warning glyph, that says where they open and why: in a part of Dinah, which comes first; in an extension enabled earlier; or nowhere in this view, because Dinah knows the file extension as another type.

### Recognising references in a message

`chat-text.js` links references in a pass over the nodes markdown-it drew, after any window's own marking (a group chat's mentions), in `linkPaths`. markdown-it's own parse is unchanged. The pass looks at three things:

- A Markdown link to a path and a `file:` address, which were links already, get the tooltip and glyph of where they open.
- Inline code whose whole text looks like a path becomes the text of a link. A path in inline code may have spaces when it begins at a drive, a slash or a dot.
- A word in running text that looks like a path becomes a link, without the punctuation around it.

A string looks like a path when, with any line suffix set aside, it has a slash or a backslash, or it is a name with an extension, and it has no address scheme. A path in a fenced or indented code block, in a link, in math or in a mention is never looked at. A relative path is asked about only when the message has a folder to read it from; a person's message in a group chat has none, so only its absolute paths are linked.

Only a reference to a file or folder that is there becomes a link, so that ordinary words that look like paths, such as `Node.js` or `e.g.`, stay text. The page asks Go about every reference drawn in one batch per folder, 40 milliseconds after the first one is wanted, at most 200 at a time, and keeps each answer for ten seconds. When the answers arrive, the blocks that wanted them are linked. Every block drawn is linked: a block still streaming in is drawn again with each chunk and linked again with it, and a settled block is linked once and kept, so a message streamed in shows what the whole message shows once the answers are in. A link wraps the reference's text and adds a glyph that has no text, so the message's text, its Copy and its Quote do not change.

### What a link shows and does

A link's tooltip says where it opens, and its glyph, from PUDL, matches the words:

| Opens in | Tooltip | Glyph |
| --- | --- | --- |
| The Editor | Open in the Editor | `document` |
| The media viewer | Open in the Media Viewer | `image` |
| Files | Show in Files | `folder` |
| The system's program | Open with the System's Program | `open` |
| The system's file manager | Show in the System's File Manager | `open` |
| An extension's view | Open in followed by the view's title | The view's glyph, or `app` |

The fifth row is needed for a folder outside the workspace sources and for a file Dinah will not start, such as a program, which the link shows in its folder as it did before. A link that has no answer yet, or a Markdown link to a file that is not there, says "Open the File" and has no glyph.

Pressing a link asks Go again (`openDocument`), since the file may have changed, and opens the place the answer gives. A line suffix opens the Editor at that line; when the Editor's view is one for reading (Visual or Preview), going to the line shows the text beside the page, as going to a line from the preview already does. A link opens the Editor with no reading intent, so the document opens in the view Settings chooses, where a chat's link used to open it in Preview.

Links work in a chat (`app.js`), a card for a message the bus delivered into a chat, a group chat and a thread (`bus-applets.js`), and a subagent's window (`subagents.js`), which now uses the shared click handler. The files attached to a group chat's message get the same tooltip and glyph. `OpenChatPath` was removed, since nothing calls it.

### Left for later

- Notifications and the Terminal's output, as above.
- A choice by the person to let an extension open a type that Dinah opens itself, or to prefer one extension over another that was enabled first.
- An action for agents. The resolver is in Go so that one could ask it, but no applet action offers it yet; nothing has needed it.

### Tests

`document_links_test.go` checks the resolver: Markdown in the view Settings chooses, Go source at a line, pictures and SVG to the media viewer, PDF, DOCX and XLSX to the system, a program shown in its folder, a text file over 1 MiB to the system, a folder in a workspace to Files and one outside to the file manager, workspace resources against local files, a hidden file in a workspace as a local file, a `file:` address with a line, paths that are not there, a file whose name looks like a line suffix, a claimed media type before the Editor and a file larger than its claim takes, and `ReadLinkedImage`. The browser suite `e2e/document-links.js` checks a chat message with a relative path, an absolute path, a path in inline code, a path with a line, a folder, a picture and a PDF, each opening the right place through the stubbed Go side with the right tooltip and glyph; a path to nothing and a path in a fenced block staying text; the message's text unchanged; and a message streamed in five characters at a time matching the whole text after every chunk.

`extension_claims_test.go` installs signed test extensions through the same entry point Settings uses and checks that a declared type opens the view with the right state, line and generic glyph; that a file outside the workspace sources does not; that pictures, SVG and the Editor's kinds stay with Dinah while PDF and an unknown extension go to the extension, and a declaration cannot change the type of `.xlsx`; that the extension enabled first wins, disabling gives the type to the other, enabling again keeps the other first, and an update or a removal replaces or removes the claims; that claims load when first needed; that a damaged package claims nothing; and what Settings reports for each case. The browser suite `e2e/extension-file-links.js` checks a chat link's tooltip and glyph, that pressing it opens the view in the file's workspace with the state `dinah.openedFile` reads, that a second link raises the same window with a `state` event, that a link to an extension that is gone fails with a clear error, and the file types and warnings in Settings › Extensions.
