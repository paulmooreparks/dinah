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

- A folder lists through `resource.Lister`. An entry's `Name` is its path segment and its `Title`, when it has one, is what is shown, so a wiki page is listed by its title while its path is its ID. An entry marked `Document` is a folder that is also a document, as a page with child pages is: it lists, reads, previews and opens in the Editor. An entry with a `URL` offers Open Externally, which opens the address in the browser. `Kind` and `Revision` are shown when the provider gives them.
- A document reads through `resource.TextReader`, or `resource.Snapshotter` when the provider has no reader, and previews from that text, as Markdown or as plain text.
- A picture in the source previews through `resource.BlobReader`, which reads a file by its path; a picture a document shows by name reads through `resource.FileSource`, as a page's attachments do.
- The source's own row lists when the build has a provider for its kind, and is kept as unavailable otherwise.

Listing is paged in memory from the provider's whole listing of one folder, with folders before documents and files, by title. The local folder keeps its own path, which opens each entry under its root and governs agents' reads.

## Connections

A kind whose sources are on a site needs the site's address and the person's token. `internal/connections` is the registry for them: a build registers each connection with its label, help, default address, and its own `Load`, `Save`, `Remove` and `Test`, so the registry holds no address and no secret and the product decides where they are kept (the system keychain, through `internal/credentials`, in the usual case). Settings shows a Connections section drawn from the registry (`settings-connections.js`): the address, whether a token is saved, and Save, Test Connection and Forget. The token is a password field and is never read back. The section's tab is hidden while the build has no connection, as Dinah's own build has none.

## Tests

`internal/workspaces/kinds_provider_test.go` registers a kind with a form and a provider and resolves a source through it. `cmd/dinah-desktop/workspace_provider_resources_test.go` registers a small wiki and lists, pages, reads, previews and stats it through the Files applet's service and the mount paths, and adds a source of it through the Add Source form, including the kind's own refusal and the registry keeping the source's fields. `cmd/dinah-desktop/frontend/e2e/workspace-source-kinds.js` drives the dialog and the Connections section against a stub backend, and checks that neither appears in a build that registers nothing.
