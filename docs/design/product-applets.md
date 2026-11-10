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

## The build's own programs

A product may bring its own way of delivering a new build to the people who use it, and for that its packages need two things the command line's installer already had. `internal/programs` has both. `programs.Release()` is the release the running window belongs to, which the window program records first thing from the version its build was given; a product's package cannot read that variable itself. `programs.Place(src, dst)` replaces an installed program while something may be running it, by writing the new file beside it and renaming the old one aside, which is how `install` has always placed the two programs. A file placed that way in the home folder's `bin` is seen by the window's own watcher, which offers Restart to Update when the product's profile uses the installed updater.

The window program prints its name and release for `--version` and ends, as the command line does for `version`, so a build about to be placed can be asked what it is.

A product's packages can add options to the window program's own with `programs.RegisterStartOption(name, run)`, called from `init`. Started with that option as its first argument, the program opens no window: it calls `run` with the arguments that follow and ends with the status `run` returns. A product whose window program replaces an earlier program of the same name uses this to keep accepting what other programs still start it with. An option that neither the window program nor the product knows ends the program with status 2 and a line on standard error; it never opens a window, since whoever passed the option expected something else.

An agent lists every session it has for a folder, and a product may know that some of them were never chats of the person's, because another program ran them for a purpose of its own. `sessionhost.RegisterNotChat(f)`, called from `init`, gives the window a function that says so for a session's id. The window keeps such a session out of its lists of chats in the same way as a chat the person hid, and Show Hidden Chats Again does not bring it back. Nothing is deleted or changed at the agent. The window asks while it builds a list, so the function has to return quickly.

## What this does not do

A product's agent-facing actions go through applet bundles like the built-in ones: the product package registers a maker with `applet.RegisterBundle` in its `init`, and the host adds what it makes to the registry it assembles at start (`applet_builtins.go`), so the bundle's actions reach agents, flows and the page's applet SDK as the built-in bundles' do. Menus of the page's own menu bar, settings sections and new pinned categories have no registry yet; each is added when a product needs it.
