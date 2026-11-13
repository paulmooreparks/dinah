# Kinds of workspace source

A workspace's sources have one built-in kind, a folder on this computer. A build of another product (docs/design/product-definition.md) can add kinds of its own, such as a wiki's page tree or an issue tracker's project, and they are listed, read, previewed, opened for editing and saved back through the same code as a folder, without that code knowing the kind. This document says how a kind plugs in and what the shared code does with it. Decided on 10 November 2026.

## Registering a kind

`workspaces.RegisterSourceKind(kind, workspaces.SourceKind{…})` already told the workspace export and import what a build knows about a kind (docs/design/workspace-export.md). The same registration now carries three more things:

- `Label` is the kind's name as people see it, "Wiki Page" for one.
- `Attach` is how the person adds a source of the kind: `Help` for the top of the form, `Fields` (text fields and check fields, with labels, placeholders, help lines, required marks and defaults) and `Build`, which takes the filled form, checks it, reaches the source's site when that is what checking takes, and returns the source with its `Kind`, `Name` and `Extra` set. A kind with a `Label` and an `Attach` is offered in the Add Source dialog; a kind without either can arrive only by import.
- `Provider` builds the resource provider of a source of the kind: a value that offers the capabilities of `internal/resource` the source supports.

A source of such a kind keeps its own fields (the page's ID, the site's address, whether child pages are included) in `Source.Extra`, the map of fields the shared struct does not have, which the registry and the export write back unchanged. `workspaces.ExtraString`, `ExtraBool` and `WithExtra` read and set them. The registry accepts a source of any plainly named kind with no path; nothing else about the kind is checked when the registry is loaded, so a source of a kind this build does not know is kept for a build that does, and shown as "Not Available Here".

A product registers its kinds in the window program's product package, `cmd/dinah-desktop/product`, which `main` imports for its side effects. Dinah's own build registers none.

## Providers and the Files applet

`Resolver.ProviderOf` returns a local folder's provider for a local folder and, for any other kind, the provider the kind registered, or `resource.ErrUnsupported`. The Editor's open and save, the drafts, Save As and attachments already went through providers (`internal/documents`, `internal/drafts`) and gain the new kinds with no change.

The Files applet, the Workspace pane and the mount paths agents and the Terminal use went through the local folder directly, since a folder's confinement and the agent file-access policy are a folder's. They now go through the provider for a source that is not a local folder (`cmd/dinah-desktop/workspace_provider_resources.go`):

- A folder lists through `resource.Lister`. An entry's `Name` is its path segment and its `Title`, when it has one, is what is shown, so a wiki page is listed by its title while its path is its ID. Files shows such an entry by its title alone, the path under a selected entry is made of titles, and a document of such a source is known to its applet by its workspace and title, so the ID is seen only through Copy Path. An entry marked `Document` is a folder that is also a document, as a page with child pages is: it lists, reads, previews and opens in the Editor. An entry with a `URL` offers Open Externally, which opens the address in the browser. `Kind` and `Revision` are shown when the provider gives them.
- A document reads through `resource.TextReader`, or `resource.Snapshotter` when the provider has no reader, and previews from that text, as Markdown or as plain text.
- A picture in the source previews through `resource.BlobReader`, which reads a file by its path; a picture a document shows by name reads through `resource.FileSource`, as a page's attachments do.
- The source's own row lists when the build has a provider for its kind, and is kept as unavailable otherwise.

Listing is paged in memory from the provider's whole listing of one folder, with folders before documents and files, by title. The local folder keeps its own path, which opens each entry under its root and governs agents' reads.

## One resource with more than one path

A source may let one resource be given by more than one path. A page of a wiki is the same page by its own ID and by the IDs of the pages down to it, which is the path the Files applet builds from the listing. A person opens the page from the tree by the long path, and an agent that knows only the page's ID gives the short one.

A provider of such a source implements `resource.Identifier`, whose `Identity` returns, from the path alone, what the path's resource is the same by. The drafts find a resource's draft by that (`drafts.Service.Find`): a draft opened by any path to the resource is the resource's draft, and opening the resource by another path continues it. A draft keeps the path it was opened by. The lock that makes two opens at once find one draft is taken by the identity too. A provider that is no `Identifier` has one resource for each path, as before.

Before 13 November 2026 a draft was found by its path as given, so one page had two drafts. An agent's suggestions went to the draft the person was not looking at, and accepting them changed nothing in the open document (Paul, 13 November 2026). Two drafts made that way are both kept. The resource opens as the one changed last, and the other stays in the list of drafts until its owner discards it.

`internal/drafts/identity_test.go` covers two paths with one draft, two old drafts of one resource, and opens at the same moment by two paths.

## A source that keeps what it has read

A source on a site is slow to list when every folder opened is a request, so a kind may keep what it has read and list from that. Two things in the shared code let it do so without showing people something old for long.

A kind that learns its kept listing is no longer true calls `workspaces.SourceChanged` with the source's ID. The usual way to learn it is to give the kept listing at once, read the site behind it, and compare. The window program gives a function with `workspaces.OnSourceChanged` when it starts (`cmd/dinah-desktop/workspace_source_changes.go`). It gathers a source's changes until the source has been quiet for a moment and then sends one `workspace:changed` event of kind `source-changed` for each workspace the source is in, and every tree showing the source lists its opened folders again, as it does for a change on disk. A program that shows nothing, such as a command that runs once, gives no function, so nobody would hear the correction there. A kind asks `workspaces.SourceChangesHeard` and gives only current answers when it returns false.

A provider that keeps what it has read implements `resource.Refresher`, whose `Refresh` forgets a path and everything under it. The Refresh commands of the Files applet, the Workspace pane and the document viewer call `RefreshWorkspaceResource` before they list or read again, so Refresh always asks the site. It does nothing for a local folder or for a provider that is no `Refresher`.

The document viewer reads a document once and does not read it again on an event, so a kind should give a document's text only as current as it would without keeping it, or check it against the site before giving it.

## Connections

A kind whose sources are on a site needs the site's address and the person's token. `internal/connections` is the registry for them: a build registers each connection with its label, help, default address, and its own `Load`, `Save`, `Remove` and `Test`, so the registry holds no address and no secret and the product decides where they are kept (the system keychain, through `internal/credentials`, in the usual case). Settings shows a Connections section drawn from the registry (`settings-connections.js`): the address, whether a token is saved, and Save, Test Connection and Forget. The token is a password field and is never read back. The section's tab is hidden while the build has no connection, as Dinah's own build has none.

## Tests

`internal/workspaces/kinds_provider_test.go` registers a kind with a form and a provider and resolves a source through it. `cmd/dinah-desktop/workspace_provider_resources_test.go` registers a small wiki and lists, pages, reads, previews and stats it through the Files applet's service and the mount paths, and adds a source of it through the Add Source form, including the kind's own refusal and the registry keeping the source's fields. `cmd/dinah-desktop/workspace_source_changes_test.go` checks that a burst of changes to a source is sent to the page once and that Refresh has the wiki forget the place shown. `cmd/dinah-desktop/frontend/e2e/workspace-source-kinds.js` drives the dialog and the Connections section against a stub backend, and checks that neither appears in a build that registers nothing.

A picture that is itself an entry of a source, such as a wiki page's attachment opened from Files, is drawn by asking for the picture by the entry's own path. The desktop reads it through the provider's `BlobReader` (11 November 2026). Until then it asked the provider's `FileSource` for a file beside the entry, with the picture's path given as the document's, which a provider that keeps files beside pages alone refused, so the window said the image could not be loaded.
