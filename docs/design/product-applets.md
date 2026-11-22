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

## What this does not do

A product's agent-facing actions go through applet bundles like the built-in ones: the product package registers a maker with `applet.RegisterBundle` in its `init`, and the host adds what it makes to the registry it assembles at start (`applet_builtins.go`), so the bundle's actions reach agents, flows and the page's applet SDK as the built-in bundles' do. Menus of the page's own menu bar, settings sections and new pinned categories have no registry yet; each is added when a product needs it.

## An action that needs the page, and a file the page made

Two further seams were added on 11 November 2026, for a product applet that makes a picture of what its window draws.

An action of a product's bundle runs in Go, in the product's own package, and some answers only the page can make. `internal/pageask` lets such an action put a question to the page: `pageask.Ask(ctx, kind, args, within)` returns the page's answer as JSON text. The window program gives its own way of asking when it starts, the same one its own applets use (`askPageWithin`). The product's page file answers the kind with `dinahWorkbench.answerAsks(kind, fn)`. The package knows no question, so the product's Go code and its page file agree on the kind and on what is asked and answered. An action that returns a file to the agent still declares `artifact.create`, as any action does.

A page file that has made a file for the person, such as a picture, saves it with `dinahResources.saveFile({ name, title, blob })`. It shows the usual Save dialog, which starts in the current workspace and takes a folder on the computer that can be written. The dialog's To Your Computer… goes to the system's own dialog, through the App method `SaveFileToComputer`, which offers the kind of the suggested name first. A name the folder already has is asked about, as an upload's is. Nothing here is particular to pictures.
