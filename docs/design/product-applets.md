# Product applets and services

A product built from Dinah's source (docs/design/product-definition.md) may bring applets of its own: windows with their own page files, backed by Go code of the product's. Dinah's page knows its applets by name in `index.html`, and its App's methods are the only ones the page bridge offers, so a product needed two general openings, one on each side. Both were added on 10 November 2026. Nothing in this repository names a product or an applet; `docs/design/product-definition.md` says why.

## Page files, listed in a manifest

The product directory, `cmd/dinah-desktop/product`, is served over Dinah's frontend, so a product's scripts and stylesheets live there and are served by their names. `product-applets.js`, loaded with Dinah's own applets, fetches `product-applets.json` from the same place and, when a product has put one there, adds each entry's stylesheet to the page and loads each entry's script, in the order listed and one after another:

```json
[{ "id": "timesheet", "script": "timesheet-applet.js", "style": "timesheet-applet.css" }]
```

A product's script registers its applet with `dinahWorkbench.register`, as Dinah's own scripts do, with a launcher entry when it belongs in the Launcher; the workbench accepts a kind registered after the page started, as it does for extensions, and opens a window of it from a saved layout or an `open` parameter once it is registered. The scripts use the page's shared modules by their names (`dinahHost`, `dinahWorkbench`, `dinahHelpers`, `dinahRegional`, `dinahUIState`, PUDL). Dinah's build has no manifest, and the page carries on without one. `dinahProductApplets.ready` settles when every listed file has loaded or failed, and the `dinah:product-applets` event follows.

## Go services, called by name

A product's applet needs Go of its own: a client of a site, a store under the state folder. The App's methods are Dinah's, and a product cannot add to them, so `internal/services` is a registry of named services, and one App method, `CallService(service, method, args)`, reaches them. The product's package registers a service in its `init` under a lowercase hyphenated name, and the page calls `dinahServices.call(name, method, [args])`, or takes `dinahServices.object(name)` and calls its methods as it would the App's. The arguments travel as a JSON array and the answer as JSON; an error comes back as the call's rejection, as an App method's does.

`services.Object(value)` makes a service of any value's exported methods, so a product writes its service as the App's methods are written, on a type of its own: a method may take `context.Context` first, which receives the call's context with its two-minute bound, then its arguments in order, and returns a value, an error, both, or nothing. A missing argument is its type's zero value, and a surplus one, a wrong one or an unknown method is refused with its reason. `ProductServices` lists what the build registered.

A service call counts as work for the exit guard, since the page cannot know which calls change something, so a quit while one runs warns about it.

## Files a product serves

An applet of a product may need to serve files of its own to the page: a client's fonts read from a cache, an embedded app's files. `internal/pageroutes` is the registry for that: the product registers an `http.Handler` under a prefix of its own, `/<name>/`, and the shell server hands it every request under that prefix that its own routes do not take, behind the same cookie that guards the page's files, so the route is reached by the page alone and never from outside. The handler decides what it serves and refuses the rest, as any file server must.

Files that a sandboxed frame loads need a route of the other kind, added with `RegisterSelfGuarded`. A sandboxed frame has no origin of its own, so its requests for its stylesheets and scripts are cross-site requests as far as the browser is concerned, and the page's cookie, which is SameSite=Strict, cannot be counted on to go with them; on 19 November 2026 WebView2 sent them without it, the server refused them and the frame's app was drawn without its styles or scripts. The shell server gives such a route its GET and HEAD requests before it asks for the cookie, and the handler admits a request only by an unguessable token it issued, which is in the path. This is how the extension frames' files are served too.

## The build's own programs

A product may bring its own way of delivering a new build to the people who use it, and for that its packages need two things the command line's installer already had. `internal/programs` has both. `programs.Release()` is the release the running window belongs to, which the window program records first thing from the version its build was given; a product's package cannot read that variable itself. `programs.Place(src, dst)` replaces an installed program while something may be running it, by writing the new file beside it and renaming the old one aside, which is how `install` has always placed the two programs. A file placed that way in the home folder's `bin` is seen by the window's own watcher, which offers Restart to Update when the product's profile uses the installed updater.

The window program prints its name and release for `--version` and ends, as the command line does for `version`, so a build about to be placed can be asked what it is.

A product's packages can add options to the window program's own with `programs.RegisterStartOption(name, run)`, called from `init`. Started with that option as its first argument, the program opens no window: it calls `run` with the arguments that follow and ends with the status `run` returns. A product whose window program replaces an earlier program of the same name uses this to keep accepting what other programs still start it with. An option that neither the window program nor the product knows ends the program with status 2 and a line on standard error; it never opens a window, since whoever passed the option expected something else.

An agent lists every session it has for a folder, and a product may know that some of them were never chats of the person's, because another program ran them for a purpose of its own. `sessionhost.RegisterNotChat(f)`, called from `init`, gives the window a function that says so for a session's id. The window keeps such a session out of its lists of chats in the same way as a chat the person hid, and Show Hidden Chats Again does not bring it back. Nothing is deleted or changed at the agent. The window asks while it builds a list, so the function has to return quickly.

## The Window That Opens an Entry

A product with windows of its own needs its entries to open in them. Until 22 November 2026 the page decided alone where an entry opens: an entry that previews opened in the read-only viewer, and a file opened in Docs, the Editor, Diagrams or Slides by the end of its name. A product's record therefore opened in the viewer or not at all, and a product's own kind of Markdown file opened in Docs because it is Markdown.

A product's page file now registers an opener, for a kind of entry or for the end of a file's name.

```js
dinahResources.registerOpener({
  itemKind: 'sample-record',            // the entry's Kind, as its provider gives it
  title: 'Records', glyph: 'ticket',    // optional words and PUDL glyph for the window
  open: function (item, opts) { return dinahWorkbench.open({ kind: 'sample-records', path: item.ref.path }); }
});
dinahResources.registerOpener({ suffix: '.sample.md', title: 'Samples', open: function (item, opts) { /* … */ } });
```

`registerOpener` takes exactly one of `itemKind` and `suffix`, and an `open` function, and returns true when the opener was registered. A suffix starts with a dot and is compared without regard to case. `open` is given the entry and an object of options, and what it returns, or the value its promise gives, is what the open returns to its caller. A function that returns the key `dinahWorkbench.open` gives lets a caller bring the window forward.

- `item` is the entry's resource item as the lists have it, with its `ref`, `title`, `kind`, `glyph`, `actions`, `status` and `changed`. An open that starts from a reference alone reads the item first. When the entry cannot be read the item has its `ref` only, and for a draft of a document that is in no source the item has a `path` made of the draft's title and no `ref`.
- `opts.intent` is `'read'` when the person opened the entry to look at it, as a double-click, Enter, a pin and a link do, and `'edit'` when they asked to edit it or opened a document they were working on.
- `opts.draft` is the draft, when the open started from one that already exists. An open that starts from an entry makes no draft, so a product that edits through drafts opens one itself.
- `opts.line` is the line a link asked for, when it asked for one.

Two openers may match one entry. An opener for the entry's kind is chosen before one for its name, because the kind is what the source says the entry is. Of two suffixes that both end the name, the longer is chosen. A folder is matched by its kind only. A second registration of a key that is already registered is refused, the first stays, and the console has a message that says which key. A registration with both keys, with neither, without an `open` function or with a suffix that has no dot is refused in the same way.

The openers are asked at the time of each open, by `dinahResources.openerFor`. An opener registered after the page restored its layout therefore serves every later open, and nothing is kept from an earlier answer. A build that registers no opener asks nothing and reads nothing more than before, and every open goes where it did.

These are the ways an entry opens, and each asks the openers before its own rules.

- A double-click or Enter in the Workspace pane's tree, and a pinned entry, which opens as the tree opens it.
- A double-click or Enter in the Files window, and the Terminal's `open` command. Both go through `dinahResources.open`.
- Edit in an entry's context menu and in Files, a Recent document, the Terminal's `edit` command, the Diff window's command that opens the file it compares, and a document window of the kind before the Editor that a saved layout still has. All go through `dinahEditor.openResource`.
- A link to a workspace's file or folder in a chat's message or in a document. The link opens in the opener's window even where Go would have sent it to the media viewer, an extension or the system. A link an opener takes by its name has "Open in" and the opener's title as its tooltip, with the opener's glyph. An opener for a kind is known only once the entry is read, so such a link keeps the words Go gave it.
- A draft that is already at hand, as In Progress and a notice about a document have it (`dinahEditor.open`). Its name is matched at once, and its kind is read from its entry when an opener for a kind is registered.

Some opens do not ask the openers, each for a reason.

- Edit as Text asks for the file's source by name, so it opens the Editor. It is also the person's way to the text of a file that a product's window opens.
- An application's own Open dialog, and an agent's action that asks one application to open a file, say which application, and the document opens there.
- A window in a saved layout or in the address's `open` parameter is given by its own kind and path, which is a statement of which window it is, so it is restored as that window. A window of a product's kind restores through the workbench once the kind is registered. A window of the read-only viewer that a layout saved before an opener was registered is restored as the viewer.
- Open in Files on a folder's context menu says where it opens, and a double-click in Files on a folder that is not also a document goes into the folder, since going into folders is what Files is for. Such a folder with an opener for its kind opens in the product's window from the tree.
- A file outside every workspace has no entry. It opens as a new document, and only an opener for the end of its name can match it.

There is no Open With for a file today, so nothing offers a choice among openers. The `title` and `glyph` of an opener are kept for a list that would. An entry's context menu is what it was. Its Open row is still offered where the entry previews, which leaves an entry that has an opener and no preview with a double-click and Enter and no row.

A document open in a window of Docs or the Editor stays open there when an opener registered later opens the same file in a product's window. Closing the first window is the product's to decide, since only it knows whether the two can show one document at once.

`cmd/dinah-desktop/frontend/e2e/resource-entries.js` registers an opener for a kind and two for suffixes and opens entries each of these ways, with and without the openers.

## A Product's Rows in an Entry's Menu

Every entry in the Workspace tree, the Files window and the Pinned list has a menu, which its actions button, a right-click and the keyboard open. Until 22 November 2026 the base decided every row of it, so a product could offer nothing of its own on a folder that stands for one of its saved queries, or on its source.

A product's page file now registers a function that returns rows, for a kind of entry or for a kind of source.

```js
dinahResources.registerEntryCommands({
  itemKind: 'sample-query',                 // the entry's Kind, as its provider gives it
  commands: function (item) {
    return [
      { id: 'sample-open-board', label: 'Open in Sample Board', glyph: 'board',
        run: function (item) { return dinahWorkbench.open({ kind: 'sample-board', path: item.ref.path }); } },
      { id: 'sample-mark-seen', label: 'Mark as Seen', disabled: !item.changedCount,
        reason: 'Nothing has changed since you last looked.', run: function (item) { /* … */ } },
      { id: 'sample-remove-query', label: 'Remove…', danger: true, run: function (item) { /* … */ } }
    ];
  }
});
dinahResources.registerEntryCommands({
  sourceKind: 'sample-tracker',             // the source's Kind, for the source's own row
  commands: function (item) { return [{ id: 'sample-choose-queries', label: 'Choose Queries…', run: function (item) { /* … */ } }]; }
});
```

`registerEntryCommands` takes exactly one of `itemKind` and `sourceKind`, and a `commands` function, and returns true when it was registered. An `itemKind` is matched against the kind of an entry inside a source. A `sourceKind` is matched against the kind of the source whose own row the menu is for, and its rows are on that row only. A second registration of a key that is already registered is refused, the first stays, and the console has a message that says which key. A registration with both keys, with neither or without a function is refused in the same way.

A row has these fields.

- `id` is the row's id, which is its command id in the Files window. It is a word of the product's own, such as `sample-open-board`.
- `label` is the row's words, in the product's own words and in title case. The base adds nothing to them.
- `glyph` is the name of a PUDL glyph, and is optional.
- `run(item)` does the work. It may return a promise.
- `disabled` and `reason` make a row that is drawn disabled, with the reason under its words. A product disables a row and says why. A row that is left out tells the person nothing.
- `danger` marks a row that destroys something. It is drawn as the base's destructive rows are.

`commands(item)` is called each time a menu for such an entry opens, so the rows are for the entry as it is then. In the Files window it is also called whenever the window reads its commands for the selected entry, which is often, so the function should be quick and change nothing. `item` is the item an opener receives, with its `ref`, `title`, `kind`, `glyph`, `actions`, `status`, `changed`, `count` and `changedCount`, and `run` is given the same item. The Pinned list has an entry's reference and not its item. There `commands` is given the reference, the kind, the title, whether the entry is a folder, and the status, changed mark and counts the trees have listed for it, and `run` is given the whole item, which is read when the row is chosen.

The product's rows stand in a group of their own after the base's rows, behind a separator. Its danger rows stand in the destructive group at the end of the menu. Where the base has a destructive row of its own, as a source has Remove from Workspace…, the product's danger rows come first in that group and the base's row stays last. Where the base has none, the product's danger rows are a last group of their own.

These are the menus that have the rows.

- The Workspace tree has them on an entry and on a source, from the row's actions button, a right-click, and Shift+F10 or the menu key.
- The Files window has them on a row of its list and on a row of its folder tree, from a right-click and from the keyboard. For the selected entry they are also in the window's Actions menu and in the window's own menu in the menu bar. The menu of the folder shown, on the empty part of the list, does not have them.
- The Pinned list has them on a pinned entry and on a pinned source, from the row's actions button, a right-click and the keyboard.

Three menus do not have them. A folder that is not on this computer yet and a source of a kind the build does not use keep the few rows they have, because a product's row would act on what is not there. The Open and Save As dialog leaves them out too, since a product's row may open one of the product's windows, which would stand behind the dialog while the dialog waits. A Files window that only displays a folder inside another window has no menu for an entry at all.

How the rows relate to commands follows what the base's rows do, and the base's rows differ between the lists. In the Workspace tree and the Pinned list a row of an entry's menu is a plain row given to PUDL's context menu. It has no id and is no command, so it is not in the commands an agent lists for a window, and it has no key of its own. The product's rows are the same kind of row there. PUDL's menu gives all of them the same keys: the arrows, Home and End, a letter, Enter and Escape. In the Files window the base's rows for the selected entry are commands of the window, registered with `pudlCommands`, and the product's rows are added to the same table. An agent therefore lists and runs them as it does the base's, and the window program refuses an agent a danger command, which is left to the person. The host's own registry, `dinahCommands`, which feeds the menu bar's menus, the list of keys and command search, has no row for an entry, and the product's rows are not added to it.

A `run` that throws, or whose promise is rejected, is reported as a failure of one of the base's rows is reported in that list: as an error toast from the Workspace pane and the Pinned list, and in the status line of the Files window.

Some rows are left out, each with one message on the console that says which: a row without an `id`, a `label` or a `run` function, a second row with an `id` the same entry already has, and a row whose `id` is one of the base's. The base's ids are `open`, `edit`, `edit-text`, `open-external`, `show-in-folder`, `copy-link`, `copy-path`, `rename`, `up`, `home`, `retry`, `browse-children`, `new-folder`, `upload`, `refresh`, `preview-pane`, `new-terminal`, `files-actions`, `open-in-files`, `pin`, `unpin`, `refresh-source`, `refresh-children`, `remove-source`, `create-folders`, `choose-folder`, `move-up` and `move-down`. A product that starts its ids with a word of its own never meets one.

A build that registers nothing asks nothing, and each of its menus has the rows it had before.

`cmd/dinah-desktop/frontend/e2e/resource-entry-commands.js` reads every one of these menus before and after a registration, runs rows from each list, and checks a disabled row, a danger row, a failing `run` and each refusal.

## Menus of Files and the Open Dialog

Paul asked on 23 November 2026 for the file browser to have a menu on the empty part of its list, a menu on every file and folder in the Open dialog as well as in Files, and a way to copy a path in the form the computer's own system writes it. He met the browser through the Open dialog that a document form's Browse… button shows, so the dialog has the same menus as the Files window, with fewer rows. Both use PUDL's context menu (`pudl:contextmenu`), so a right-click, Shift+F10 and the menu key open the same menu. With focus on a row, the keys open that row's menu. With focus elsewhere in the list, or in the path bar, they open the menu of the folder shown. A press on the empty part of the list puts focus there for that reason. A text field keeps the browser's own menu.

The rows of each menu are listed in Table 1. A row that cannot be used now is disabled and says why, and a row that has nothing to act on is left out. Show in Folder is left out for an entry with no place on the computer, while Copy Path stays and is disabled, so the person learns that the entry has no path rather than wondering where the row went.

**Table 1** *Rows of the File Browser's Menus*

| Where | Rows, in order |
|---|---|
| Files, the empty part of the list | New Folder…, Upload Files…; Copy Path, Show in Folder, New Terminal Here; Refresh, Show the Preview Pane |
| Files, a row or a folder in the tree | Open (Open in Files for a folder), Edit, Edit as Text, Open Externally, Show in Folder, Copy Link, Copy Path, Rename…; then a product's rows |
| Open dialog, the empty part of the list | New Folder…; Copy Path, Show in Folder; Refresh |
| Open dialog, a file it takes | the dialog's own button (Open, Choose, Attach); Copy Path, Copy Link, Rename…, Show in Folder |
| Open dialog, a folder | Open, which goes into it; Copy Path, Copy Link, Show in Folder, New Folder… |

*Note.* A semicolon in the second column marks a separator. Rows the entry does not have are left out, as Edit is for a file the Editor does not open and Rename… is for a file its source does not rename. A file the system would not open in a program, such as a script, has a single Show in Folder in the place of Open Externally. Save As has the same menus, without the action row on a file.

The dialog opens no window of the product from its menus. Show in Folder opens the system's file manager, and Rename… and New Folder… ask in a dialog over the dialog. The dialog's New Folder… on a folder makes the new folder there and goes into it, as its button does for the folder shown.

### Copy Path

Copy Path gives the path of a file or folder as the operating system writes it, for a file, a folder and the folder shown. The page puts on the clipboard the string Go's `WorkspaceLocalPath` returns, unchanged. Go makes it with package `filepath` from the source's folder and the entry's place in it, so it has the separators and the volume of the system Dinah runs on, and nothing in the page knows which system that is.

Whether an entry has such a path is a flag Go sets on the item, `local`. Every file and folder of a folder on the computer has it, and so do the source's own folder, the home folder, /bin and /lib, whether or not they can be written. The folders above the workspaces, a workspace itself, a folder that is missing, and an entry of another kind of source (a page of a wiki, a record of a tracker) do not have it, and their Copy Path is disabled with "This folder is not on the computer." or "This is not on the computer." An entry with a web address keeps Copy Link. Until 23 November 2026 the page took an entry that can be written to be one on the computer, which left Copy Path out for a folder that cannot be written.

The path bar of both windows ends with a raised button with PUDL's copy glyph and the tooltip Copy Path, which copies the path of the folder shown. It is always there. For a folder with no place on the computer it is disabled, and its tooltip says why.

Show in Folder shows a file or folder in the system's file manager, selected in the folder that contains it (`ShowWorkspaceResource`). It uses the same documented calls as the rest of the program: Explorer's `/select` switch on Windows, `open -R` on macOS, and `xdg-open` of the containing folder elsewhere.

The Workspace pane keeps its menus as they were. It still leaves Copy Path out for an entry with no place on the computer, and it now decides that by the same flag.

`cmd/dinah-desktop/frontend/e2e/files-menus.js` opens each menu in both windows by a right-click, by Shift+F10 and by the menu key, checks the rows and their reasons, copies paths from the menus and the path bar, and checks that the clipboard is given the path in the form of the system the suite runs on. `workspace_local_path_test.go` checks the form of the path Go gives and which items have the flag.

## A Product's Opener for a Web Address

Since 22 November 2026 a press on a link to another site opens the link in the person's browser, wherever the link is on the page. A product may know some of those addresses better than the browser does, such as an issue of a tracking system it shows in a window of its own. Until 23 November 2026 such a product had to add a listener of its own ahead of the page's, and a link in a chat's message still said "Open in the Browser" in its tooltip and its menu.

A product's page file now registers an opener for the addresses it opens.

```js
const unregister = dinahLinks.registerOpener({
  id: 'sample-issues',
  match: function (url) { return url.hostname === 'tracker.example' && url.pathname.startsWith('/issues/'); },
  open: function (href, event) { return dinahWorkbench.open({ kind: 'sample-issue', path: href }); },
  title: 'Open in Issue Window', glyph: 'ticket'
});
```

`match` is given a `URL` and must return `true` at once, because a press can be prevented only while the page is handling it. Any other value, a promise among them, is no match. `open` is given the address and the press, and the press is undefined when a menu row opened the link. A failure of `open`, thrown or as a rejected promise, is reported on the console and as an error toast. `title` and `glyph` are the words and the PUDL glyph a link's tooltip and menu show for the opener. A second registration with an id that is already registered replaces the first, in its place in the order. The function `registerOpener` returns unregisters the opener, and it does nothing once a later registration has replaced it. A registration without an id, a `match` function or an `open` function is refused with a message on the console.

`dinahLinks.open(href, event)` is the one place that decides where a press on a link goes, and it returns true when the link was opened. A press of the primary button with no modifier goes to the first opener whose `match` returns true. A press with Shift, Control or Command, or with the middle button, opens the browser, so the person can always reach the site itself. An address that no opener matches opens in the browser. `dinahLinks.openerFor(href)` returns the opener a plain press would use, or null, and `dinahLinks.browse(href)` opens a web address in the browser whatever opener there is.

These are the parts that ask the openers.

- The page's own listener for links to other sites (`helpers.js`), which serves every link that no part handles itself. A middle-button press reaches it as well.
- A web link in a document shown in the resource viewer (`document-applet.js`).
- A link in a chat's message, a group's message, a subagent's log and the release notes (`chat-text.js`). The tooltip of such a link is the opener's title, and is read again when the pointer or the focus reaches the link, since an opener may be registered after the message was drawn. Its menu has the opener's row first, with the opener's glyph, and Open in Browser after it.
- The link to a provider's own page beside a file limit in Settings.

The Docs editor and the other applications of the yavc.app release take no option from their host for links, so the page's listener serves whatever press on a link they leave to the page. In a document being edited a plain press on a link places the caret and goes no further, and a press with Control is left to the page. Under the rule above that press opens the browser and never an opener. The text editor's Files pane and its Preview command open addresses with `window.open`, which the host passes on to the browser's own `window.open` (`content-host.js`) without asking the openers.

A build that registers no opener asks nothing more than before, and every link opens where it did.

`cmd/dinah-desktop/frontend/e2e/outside-links.js` registers an opener and checks a plain press, the presses that go to the browser, a replacement, an unregistered opener, and the tooltip and menu of a link in a message.

## What this does not do

A product's agent-facing actions go through applet bundles like the built-in ones: the product package registers a maker with `applet.RegisterBundle` in its `init`, and the host adds what it makes to the registry it assembles at start (`applet_builtins.go`), so the bundle's actions reach agents, flows and the page's applet SDK as the built-in bundles' do. Menus of the page's own menu bar, settings sections and new pinned categories have no registry yet; each is added when a product needs it.

## An action that needs the page, and a file the page made

Two further seams were added on 11 November 2026, for a product applet that makes a picture of what its window draws.

An action of a product's bundle runs in Go, in the product's own package, and some answers only the page can make. `internal/pageask` lets such an action put a question to the page: `pageask.Ask(ctx, kind, args, within)` returns the page's answer as JSON text. The window program gives its own way of asking when it starts, the same one its own applets use (`askPageWithin`). The product's page file answers the kind with `dinahWorkbench.answerAsks(kind, fn)`. The package knows no question, so the product's Go code and its page file agree on the kind and on what is asked and answered. An action that returns a file to the agent still declares `artifact.create`, as any action does.

A page file that has made a file for the person, such as a picture, saves it with `dinahResources.saveFile({ name, title, blob })`. It shows the usual Save dialog, which starts in the current workspace and takes a folder on the computer that can be written. The dialog's To Your Computer… goes to the system's own dialog, through the App method `SaveFileToComputer`, which offers the kind of the suggested name first. A name the folder already has is asked about, as an upload's is. Nothing here is particular to pictures.
