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

## What this does not do

A product's agent-facing actions still go through applet bundles (`applet_builtins.go`), which the product package can register as the comment threads applet is. Menus of the page's own menu bar, settings sections and new pinned categories have no registry yet; each is added when a product needs it.
