# Product definition

Dinah's source can be built as another product. The platform plan of 4 November 2026 (`docs/proposals/dinah-platform-plan.md` in the plan worktree) calls this seam the product definition: a build chooses, at build time, the product's name, company, state folder, environment prefix, programs, ports, cookies, keychain service, window classes, package manifest, agent providers and default theme, and brings its own mark, brand stylesheet and theme data. Nothing in this repository names or describes any such product; its definition lives in its own repository and reaches the build through two general openings, described here. Decided on 10 November 2026.

## The profile, at link time

`internal/profile` has always gathered a product's values in one `Profile`, and every place that shows or uses one of them reads it from `profile.Current()`. Until now `Current()` was Dinah's profile and nothing could choose another. A build now supplies a whole profile as a JSON document, encoded in standard base64 so that it survives the linker's command line, in the linker-set string `studio/internal/profile.injected`:

```
go build -ldflags "-X studio/internal/profile.injected=<base64 of the JSON>" ...
```

The profile is parsed when the package initialises, before any package-level value derived from it anywhere else, so every derived default (the CLI's, the bus's, the window's) sees the product's values. Dinah's own builds set nothing. An injected profile must be whole and sound: every value a product must give is required, the two ports must be real and different, the default provider must be one of the offered ones, and a field this version does not know is refused. A profile that fails stops the program before it starts, with every problem said at once, since an incomplete profile would mix one product's names with another's. The JSON keys are the `Profile` struct's field names (`Name`, `HomeFolder`, `Providers`, …); `internal/profile/injected_test.go` holds a whole example of a made-up product.

Names written on the wire and read back stay Dinah's whatever the build, as the profile's own documentation says: the `dinah.*` action IDs, the MCP servers `dinah`, `dinah-bus` and `dinah-applets`, and markers such as "[Dinah chat: …]". The agent providers a product offers, and its default, were already the profile's (`Providers`, `DefaultProvider`, `Offers`), so a product that offers Devin alone lists Devin alone, and Settings, new chats and the provider registry follow.

## The page's files, over Dinah's

The page refers to four brand files by product-neutral names: `product-mark.svg` (the mark beside the product's name and the window's icon), `product-logo.svg` (the About box's logo, 96 pixels square unless the product's stylesheet sizes it), `brand.css` (the brand stylesheet, loaded after PUDL's and before Dinah's own) and `theme-data.js` (the built-in themes, one of which the profile's `DefaultTheme` names). Dinah's copies are in `cmd/dinah-desktop/frontend`. A product puts its own copies, and any other page file it means to replace, in `cmd/dinah-desktop/product`, which Dinah ships with nothing but its README. The window serves the page from the product's files laid over Dinah's (`product_assets.go`): a path found in the product directory is served from there, and any other from the frontend. The theme data is read the same way, so a product's built-in themes are the ones the appearance library knows. The Windows executable's icon and version resources are a build step of the product's own, as they are of Dinah's.

That one directory is the only place in Dinah's tree a product's repository adds files; the guard the platform plan describes allows additions there and nowhere else among inherited files.

## What this does not yet do

The plan lists further seams a product may need: a composition root that calls one host entry point with the product's applet bundles, resource providers, package kinds, settings sections and page contributions; registries for menus, settings and the page bridge; a workflow-hosting adapter. Each is added when a product needs it, as a general seam with a neutral name and a contract test, and is recorded here or beside here. Until a product's executable has its own composition root, it is Dinah's `cmd/dinah-desktop` and `cmd/dinah` built with the profile injected and the product directory filled, under the product's executable names.
