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

- A folder lists through `resource.Lister`. An entry's `Name` is its path segment and its `Title`, when it has one, is what is shown, so a wiki page is listed by its title while its path is its ID. Files shows such an entry by its title alone, the path under a selected entry is made of titles, and a document of such a source is known to its applet by its workspace and title, so the ID is seen only through Copy Path. An entry marked `Document` is a folder that is also a document, as a page with child pages is: it lists, reads, previews and opens in the Editor. A source sets `NoEdit` on such an entry when editing it in the Editor would be redundant, as when a product opens the entry in a window of its own. The entry then lists, reads and previews as before, and its menu keeps Open in Files and Refresh Children but has no Edit. An entry with a `URL` offers Open Externally, which opens the address in the browser. `Kind` and `Revision` are shown when the provider gives them.
- A document reads through `resource.TextReader`, or `resource.Snapshotter` when the provider has no reader, and previews from that text, as Markdown or as plain text.
- A picture in the source previews through `resource.BlobReader`, which reads a file by its path; a picture a document shows by name reads through `resource.FileSource`, as a page's attachments do.
- The source's own row lists when the build has a provider for its kind, and is kept as unavailable otherwise.

Listing is paged in memory from the provider's whole listing of one folder, with folders before documents and files, by title. The local folder keeps its own path, which opens each entry under its root and governs agents' reads.

### Listings Load Whole

Paul ruled on 24 November 2026 that no view offers "Load More". A real file system never shows one, so every window of Dinah lists a folder whole. The pages of `ListWorkspaceMounts` and `ListWorkspaceMountsExpected` stay, since a page bounds each call, but the page fetches every page itself. A page may hold up to 1000 entries (`maxListPage`). Each call to a source other than a folder on the computer lists the whole folder on the source's side and returns a part of it, so the number of calls is what a long listing costs. The applet actions that list for agents keep their limit of 100.

The Files window, the Open and Save As dialog and the Workspace pane's Sources tree share one model, `dinahResources.createTree` in `resources.js`. A folder listed for the first time is drawn once it is whole or after a quarter of a second, whichever comes first, so most folders draw once and whole. A slow folder draws what has arrived, and the pages after it are added below, the views redrawn at most every 150 ms. A folder listed again, by Refresh or after a change on disk, keeps what it shows until its new listing is whole and then changes once, so a list is never emptied and filled again. A refresh asked for while a first listing is still arriving is made when that listing is whole. A page that fails after others were listed leaves them shown, and the error is shown as a failed listing's is: in the status line of Files and as "Unavailable" with Retry in the Sources tree. Retry goes on from the page that failed.

The file browser (`vendor/parks-files/filebrowser.js`) draws a long list a part at a time: what is in view at once, and the rest 300 rows at a time, each part below the last. A redraw whose rows begin with the rows already drawn only adds the new ones, so the rows in view do not move and the list keeps its scroll. A folder of 20,000 entries is drawn with no task longer than about a fifth of a second.

The Terminal lists a folder whole in pages of 1000 for `ls`, `tree`, `find` and completion. It no longer refuses a folder of more than 10,000 entries, and Ctrl+C ends a listing between pages. `ls` still prints once the listing is whole, because it sorts by name and lays names out in columns.

`e2e/files.js`, `e2e/workspace-sources.js`, `e2e/workspace-indent.js` and `e2e/terminal.js` check that folders given in pages are listed whole with no Load More, that a slow folder of 2500 entries is drawn as it arrives with its first rows in place, that a failed page keeps what was listed, and that `ls` lists 12,000 entries.

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

## An Entry's Status

A source may know a state for each of its entries, as a tracking system knows whether a record is to do, in review or done. Since 22 November 2026 a provider says so on the entry it lists, and every list of entries shows it. Before then an entry of such a source showed a name and a glyph and nothing else.

`resource.Entry` has two more fields, and a provider that sets neither has entries that are listed as they were before.

- `Status` is a `resource.Status`, which has a `Label` and a `State`. The label is the state in the source's own words, such as "In Review". The state is one of `resource.StatusNeutral`, `StatusAccent`, `StatusPositive`, `StatusWarn` and `StatusDanger`, which are the plain badge of the visual layer and its four variants. A source maps its own states onto these five, because the badge gives each a glyph and a colour together and a list never tells a state by colour alone. A state outside the five is shown as the plain badge, and a status with no label is no status.
- `Changed` is true when the entry has changes the person has not seen. The source decides what counts as seen and clears the mark itself.

The window program copies both to the item the page receives (`ResourceItem.Status`, which is a label and a state, and `ResourceItem.Changed`, in `workspace_provider_resources.go`), and the mount listing has the same two fields. An item without a status has no `status` field at all. A local folder's items have neither field.

The page draws both through one function, `dinahResources.marks(item)` in `resources.js`, so the lists agree. The status is PUDL's badge with the label as its words, and its accessible name is "Status:" followed by the label. The changed mark is PUDL's history glyph with the accessible name "Changed since you last looked". The mark has a place of its own, which is kept whenever the entry has a status, so a mark that comes or goes does not move the badge. At the end of a row the mark's place is last and the badge ends before it. In the Files list, where the marks follow the name, the mark's place comes first. These are the lists that draw them.

- The Workspace pane's tree puts the marks at the end of the entry's row, before the row's actions button.
- The Files window, and the Open and Save As dialogs that share its list, put them after the entry's name in the list and at the end of the row in the folder tree.
- The Pinned list puts them at the end of the pinned entry's first line. A pin records an entry's reference and not its item, so the page keeps the statuses the trees have listed and asks once for a pinned entry no tree has listed, when the entry's source is not a folder of files.

The name of a row with marks starts where the name of a row without them does, and both rows are the same height.

### The Badge's Two Forms

Since 22 November 2026 the badge has two forms. The first build always drew the label, and in a Workspace pane of ordinary width, about 300 px, a row then showed a few letters of its name and a label cut short with an ellipsis, so neither could be read.

- The compact form is the badge's glyph and colour in a box that has the same size for every state. It shows no words.
- The full form adds the label, and the label is always whole.

A badge's words are never cut short in either form. The label is the badge's accessible name and its tooltip in both forms, so a person who uses a keyboard or a screen reader gets it whichever form is drawn.

The style sheet chooses the form from the width of the list the row is in, with a CSS container query (`app.css`, under "An entry's status"). A viewport query would be wrong, because a pane can be narrow on a wide screen, and no script measures a row. Each nested list of a tree is a container of its own, so a deeper level, whose rows have less room, changes form at the same room for a row.

The width at which the label appears depends on the label's length, since a longer label needs more room. `marks` puts the length on the badge as one of four sizes, counted in characters, and each size has its width.

| Label | Tree list at least | Pinned list at least |
| --- | --- | --- |
| Up to 12 characters | 360 px | 380 px |
| 13 to 24 characters | 440 px | 460 px |
| 25 to 40 characters | 540 px | 560 px |
| More than 40 characters | never shown | never shown |

These widths were chosen from pictures of a pane 260, 300, 360, 420 and 520 px wide, in both themes, with thirty entries whose names were a key of eight characters and a summary. They are the widths at which a row still shows about 24 letters of its name beside the longest label of its size. A row one level into the Workspace tree has 133 px of glyph, gaps, changed mark and compact badge around its name, and the label "In Progress" adds 65 px, so at 360 px the name keeps about 160 px, which is 24 letters. A pinned row has about 20 px more around its name. The list of a source's entries is 50 px narrower than the pane, so in the Workspace tree short labels appear when the pane is about 410 px wide, and at 300 px every badge is compact and a name shows about 17 letters.

A plain badge has no glyph in PUDL, which gives glyphs to its four status variants only. The page gives the plain badge PUDL's circle drawn small (`dinah.css`), and the positive badge has the same circle at full size. In the compact form these two differ by the size of the glyph and by colour, and the other three states have shapes of their own. A glyph of its own for the plain badge is a thing PUDL lacks, and the page does not draw one of its own making.

A row of Files is a row of a table whose Name column is as wide as its longest name, so there the marks have a place of one fixed width after the name, and a status that changes cannot change a column's width. The place is as wide as the changed mark and the compact badge until the Files window is 920 px wide. From there the place is 20 characters wide and shows a label of up to 12 characters whole. A longer label would not fit the place, and it stays compact at every width. The width of 920 px is where a list beside its folder tree has room for a name of about 40 letters and the wider place without scrolling sideways. Below it the narrow place keeps the status in sight next to the name. The folder tree of Files follows the rule of the Workspace tree.

A status changes on a listed entry when the source reports a change (`workspaces.SourceChanged`, above) and the trees list their opened folders again. The badge's words, its state and the mark change in place. No row moves and no name's start moves. In the compact form the badge's box does not change at all. In the full form, in the tree and the Pinned list, a longer label takes its extra room from the end of the name, and the badge's end, the changed mark and the row's actions button stay where they are. The same holds when a list is resized and its badges change form.

The tree had no mark for a changed or unsaved entry before this one, so the changed mark shares its place with nothing. A document with changes that are not saved is still listed under In Progress, which is a different fact about a different thing: the person's own unsaved work, where the changed mark is about changes made at the source.

Search has no list of a source's entries. Find and Open lists commands, chats and recent documents by their titles, and it shows no status.

## A Folder's Counts

A source may know how many entries a folder stands for before the folder is opened, as a tracking system knows how many records a saved query finds, and how many of those have changes the person has not seen. Since 22 November 2026 a provider says so on the folder's entry, and every list of entries shows it. Before then an entry had no place for these numbers.

`resource.Entry` has two more fields.

```go
Count        *int // how many entries the folder stands for
ChangedCount *int // how many of them have changes the person has not seen
```

Each is nil while the source does not know it. A folder that has not been read has no counts, and that is a different fact from a folder with nothing in it, whose count is zero. A provider sets a known count with `resource.Counted(n)`. Two pointers were chosen over a struct with a flag for three reasons. The two counts become known separately. A nil pointer is how Go usually writes "no value", and a source that set a number in a struct and forgot its flag would show nothing. A pointer with `omitempty` is written to JSON as a number when it is known, zero included, and is left out when it is nil, so the page reads the same distinction with no field of its own for it.

```go
resource.Entry{Name: "open", Title: "Open Records", Kind: "sample-query", Folder: true,
	Count: resource.Counted(12), ChangedCount: resource.Counted(3)}
```

The window program copies both to the item the page receives, as `count` and `changedCount` (`ResourceItem.Count` and `ChangedCount` in `workspace_resources.go`, set by `providerItem`), and the mount listing has the same two fields. A count that is not known is absent from the item. A count below zero is treated as not known, and the counts of an entry that is not a folder are left out. The base's own sources set no counts. Counting the files of a folder on the computer would cost a read of every folder listed, which the tree does not make today.

The page draws the counts through `dinahResources.marks(item)`, with the status and the changed mark, so every list that draws those draws the counts too.

- The total is the count badge that the section heads of the Workspace pane have ("PINNED 4"), with the accessible name "12 entries", or "1 entry".
- The changed count is PUDL's accent badge with the changed mark's glyph, and its accessible name is "3 changed since you last looked".
- A count that is not known has no element, so no room is kept for it.
- A changed count of zero draws nothing, and a total of zero is drawn as 0.
- An entry that is not a folder draws no counts, whatever its item says.

The marks of a row stand in this order: the status badge, the changed mark, the changed count, the total. The total is therefore the last thing before the row's actions button in the Workspace tree and the Pinned list. A folder with counts and no status has no place for a changed mark, since the place is kept only beside a status.

A count that arrives, changes or goes takes its room from the end of the name. The toggle, the folder's glyph, the name's start, the row's height and the row's actions button stay where they are, and the last count ends where the marks ended before. The numbers have figures of one width and the total's box is at least two figures wide, so a count that changes within two figures changes no width at all. A status badge and a changed mark on the same row stand before the counts, so they move toward the name when a count on that row grows. That is the one movement this seam adds, and it happens only on a folder that has both a status and counts.

### What Gives Way in a Narrow List

The counts are never cut short. The style sheet decides what a narrow list shows with container queries on the list's own width, as it does for the badge's two forms (`app.css`, under "A folder's counts"). These things give way, in this order, as a list narrows.

1. The words of a status badge give way first, at the widths in the table above, and the badge is then compact.
2. The name gives way next. It is cut short with an ellipsis where the marks begin.
3. In a list less than 290 px wide, a folder that has a status as well as counts shows its counts and its changed mark without its status badge.
4. In a list of the Workspace tree less than 244 px wide, and in a Pinned list less than 256 px wide, a folder with both counts shows its changed count without its total. The changed count stays because it is the number a person acts on.

A folder with a total alone always shows it, and a changed count is shown at every width. A badge or a total that is not drawn is still read by a screen reader, and the changed count's tooltip then has the total as well ("3 changed since you last looked, of 12 entries").

The widths were measured with a total of three figures and a changed count of two, in a pane 250, 300 and 520 px wide and in both themes. In a pane 300 px wide the list of a source's entries is 265 px wide. A folder with two counts shows both there and keeps about 13 letters of its name, and a folder with a status as well shows its counts without the status badge. In a pane 250 px wide a folder with two counts shows its changed count and keeps about 11 letters.

In the Files list, where the marks follow the name, the counts follow the marks in a place of their own. The place has one width for every row that has counts, the total comes first in it and the changed count after, so a changed count that comes or goes and a total that grows move nothing and change no column's width. A row whose counts are not known has no such place. When a count becomes known on the row with the longest name, the Name column can grow by the width of the place, as it can when a status arrives there.

The Pinned list has an entry's reference and not its item, so it shows the counts the trees have listed for the entry, and asks once for an entry no tree has listed, as it does for a status.

A product sets the counts when it lists the folder's parent, from what it already knows. A count the product learns later is shown once the product calls `workspaces.SourceChanged` and the trees list their opened folders again.

## A Folder's Order

A folder's entries are shown with the folders first and each by name, with a run of digits in a name read as a number. A source can keep an order of its own for a folder, as a list of results has the latest first. Its provider then implements `resource.OrderedLister`, and `ListsInOrder(path)` reports true for each folder whose entries are shown in the order `List` gives them. The other folders of that source, and every folder of a source that does not implement it, are ordered by name as before. `orderEntries` in `cmd/dinah-desktop/workspace_provider_resources.go` applies the rule, and `ordered_lister_test.go` covers both cases.

## Connections

A kind whose sources are on a site needs the site's address and the person's token. `internal/connections` is the registry for them: a build registers each connection with its label, help, default address, and its own `Load`, `Save`, `Remove` and `Test`, so the registry holds no address and no secret and the product decides where they are kept (the system keychain, through `internal/credentials`, in the usual case). Settings shows a Connections section drawn from the registry (`settings-connections.js`): the address, whether a token is saved, and Save, Test Connection and Forget. The token is a password field and is never read back. The section's tab is hidden while the build has no connection, as Dinah's own build has none.

## Tests

`internal/workspaces/kinds_provider_test.go` registers a kind with a form and a provider and resolves a source through it. `cmd/dinah-desktop/workspace_provider_resources_test.go` registers a small wiki and lists, pages, reads, previews and stats it through the Files applet's service and the mount paths, and adds a source of it through the Add Source form, including the kind's own refusal and the registry keeping the source's fields. `cmd/dinah-desktop/workspace_source_changes_test.go` checks that a burst of changes to a source is sent to the page once and that Refresh has the wiki forget the place shown. `cmd/dinah-desktop/frontend/e2e/workspace-source-kinds.js` drives the dialog and the Connections section against a stub backend, and checks that neither appears in a build that registers nothing. `cmd/dinah-desktop/workspace_resource_status_test.go` checks that an entry's status and changed mark reach its item and that an entry without them has an item with neither field. `cmd/dinah-desktop/frontend/e2e/resource-entries.js` lists a stub source whose entries have statuses and changed marks, in the tree, in Files and in the Pinned list, and checks that a status changing on a listed entry moves nothing. The same Go test file checks that a folder's counts are on its item when they are known, when they are zero and when they are not known, and `cmd/dinah-desktop/frontend/e2e/resource-folder-counts.js` lists a stub source whose folders have counts in the tree, in Files and in the Pinned list, at three widths of the pane, and checks that a count that arrives or changes moves no toggle, no name's start and no actions button.

A picture that is itself an entry of a source, such as a wiki page's attachment opened from Files, is drawn by asking for the picture by the entry's own path. The desktop reads it through the provider's `BlobReader` (11 November 2026). Until then it asked the provider's `FileSource` for a file beside the entry, with the picture's path given as the document's, which a provider that keeps files beside pages alone refused, so the window said the image could not be loaded.
