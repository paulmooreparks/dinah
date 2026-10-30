# Exporting and Importing a Workspace

A workspace set up on one computer can be taken to Dinah on another computer without building it again by hand. Paul approved the feature on 26 October 2026, and Dinah gained it on 27 October 2026. This note describes Dinah's version.

## What a Workspace Export Contains

An export is the workspace's structure and nothing of its content. It is one JSON file, `<name>.dinah-workspace`, whose extension comes from the profile (`WorkspaceFileExtension`), and it holds:

- the workspace's display name;
- each source, with its kind, its display name and what identifies it, which for a local folder is its location as described below;
- which local folder is the main folder;
- the pins of resources, kept by the source they belong to and the path within it;
- for each local folder, the folders inside it, without their files, so that the other computer can show and make the same folders.

It leaves out everything that belongs to the computer or the person. Workspace and source IDs are made afresh by the importing computer. Credentials, chats and chat pins are left out too, since a chat is kept by the agent on the computer it ran on, and so are drafts and the window layout. The workspace's skills are in its main folder's `.devin` folder, so they reach the other computer with the folder itself.

The file's format is a compatibility contract. It has a `format` number, which is 1, fields are only ever added, and a reader ignores fields it does not know. Within a source or a pin, Dinah keeps the fields it does not know rather than ignoring them, as described under "Kinds This Build Does Not Know".

## Local Folders on the Other Computer

A local folder may not exist on the other computer, may exist at another path, or may be on its way. A folder in OneDrive appears when OneDrive has synchronised it, possibly as files that are only downloaded when opened. Dinah therefore never creates a folder during an import unless the person asks, because a folder it made could collide with the one OneDrive is about to bring.

The export records each local folder's location in three ways, with `/` as the separator so that the file reads the same on Windows, Linux and macOS:

1. its path relative to the export file's own folder, when the folder and the file share an ancestor within the person's home folder;
2. its path relative to the person's home folder (Go's `os.UserHomeDir`), when it is inside it;
3. its absolute path.

The import tries the home folder first, then the export file's folder, then the absolute path, and proposes the first that exists. The home folder covers a synchronised folder that sits in the home folder on both computers under different user names. The export file's folder covers an export kept beside its folders outside the home folder, and it is trusted only when the folder it leads to has the recorded folder's name, since otherwise an export saved in Downloads would make Downloads the main folder. None of this needs any knowledge of OneDrive, and it relies only on the documented home folder.

When none of them exists, the folder is shown as Not on This Computer Yet, and the person can choose another folder for it, create it now, or import it as it is. A source imported as it is keeps the proposed path and waits for the folder to appear.

The main folder cannot be changed after a workspace is made, because chats find their workspace by its path. The import therefore settles it before the workspace is made. The person accepts the proposed path, chooses a folder, or imports with the main folder not there yet, in which case the workspace waits until the folder appears at that path.

### Folders Not Yet on the Computer

The export lists the folders inside each local folder, up to eight levels deep and 2,000 folders, leaving out hidden folders, symbolic links and every file. An import keeps that list with each local folder that is not on the computer, in the workspace record's `plannedFolders`, by source ID. The registry drops any list that does not belong to a local folder of the workspace or would leave it.

While a folder is missing, the Sources tree shows it and the folders inside it as the export recorded them, dimmed, with a Not Created Yet badge, and they can be expanded and browsed like any folder. Each of them has **Create the Folders…** in its menu. It asks first, saying that a folder still to arrive through a synchronising service is better left for that service, and then makes the source's folder and every recorded folder inside it that does not exist, never changing one that does. Once the folder exists, by either route, the tree shows the real folder and the recorded list is set aside. **Choose the Folder…** in the same menu points a source that is not the main folder at a folder elsewhere on the computer.

A workspace whose main folder is missing cannot be opened, since its agents start there. It is listed with (Not on This Computer Yet) after its name, and choosing it offers Create the Folders for its main folder, or leaving it to wait.

## A Workspace That Already Exists

A folder is the main folder of one workspace at most. When the importing computer already has a workspace on the proposed main folder, the import adds the sources and pins that workspace lacks rather than making a second one, and says how many it added. A source the workspace already has is left as it is, so importing the same export twice does no harm.

## Kinds This Build Does Not Know

Another build's exports may hold kinds of source and pin that Dinah does not have. Each kind's behaviour is a registration in package `workspaces`, in the way a `database/sql` driver registers itself, so that such a build can register its kinds from its own repository:

- `RegisterSourceKind(kind, SourceKind{...})` for a kind of source. `Read` checks and cleans a source of the kind as an export has it, `Describe` gives the site and reference the import window shows, `Check` says why the computer cannot use it yet (such as a missing connection), `Export` and `Recreate` turn a registry source into an export's and back, and `DefaultName` gives a name to an imported source that has none. Every field is optional.
- `RegisterPinKind(kind, PinKind{...})` for a kind of pin. `Read` says whether a pin of the kind in an export can be kept.

A local folder is built in and cannot be registered.

A source or pin of a kind nobody registered is kept. The registry's `Source` and `Pin`, and the export's `ExportSource` and `ExportPin`, keep the JSON fields they do not know in `Extra` and write them back unchanged, so the registry in `settings.json` keeps the shape another build wrote, and an export of an imported workspace writes such a source or pin exactly as it came. The import window shows such a source as Kept, Not Checked, says how many pins are of kinds Dinah does not use, and offers nothing else for them. In the Sources tree the source is dimmed, with a Not Available Here badge, and its only command is Remove from Workspace…; the Pinned group leaves such a pin out without removing it from the record.

## What the Person Sees

- **Export Workspace…** and **Import Workspace…** are in the Dinah menu after Rename Workspace…, in the status bar's workspace menu and in the Workspace pane's Actions menu. Export saves the file where the person chooses, suggesting the main folder.
- The import window lists each source with what was found. A local folder shows its proposed path, Found or Not on This Computer Yet, and Choose…, and a missing folder also has Create It Now with the number of folders recorded inside it. Which controls a source has comes from the plan's `actions`, so the window shows only what the person can do. Its button is Import, or Add to the Workspace when a workspace already uses the main folder. The workspace is selected when its main folder is there.
- `dinah workspace list` lists the workspaces and `dinah workspace export WORKSPACE [--out FILE]` writes an export. Both read the registry and never write it, and the command line has no import, because an import asks the person about folders and changes the registry, which only the window does.

## As Built, 27 October 2026

The format and its reading are in `internal/workspaces` (`export.go`, `kinds.go`, `extra.go` and `load.go`): `BuildExport`, `Location`, `Locate`, `ListFolders`, `CleanFolder`, `ReadExport` and `Load`. A source in the file has a key (`s1`, `s2`…) that its pins refer to, never a source ID. The window's side is in `cmd/dinah-desktop/workspace_export.go`, and the export file is written through `internal/statefile`, so an interrupted write leaves the old file or the new one. A missing local folder's source item is listed with `missing` set and the actions `create-folders` and, except for the main folder, `choose-folder`, and nothing can be written into it. State lists the workspaces whose main folder is missing as `missingWorkspaces`.

Go tests cover the export with hidden folders and files left out, the three ways a folder is found including another user name and an export in Downloads, a file that cannot be trusted, a registered kind, the folder limits, an import on a second computer with a folder still to come, browsing that folder's recorded folders, importing twice, Create the Folders…, moving a folder, a main folder still to come, a folder created during the import, and a source and pin of unknown kinds passing through an import and a second export unchanged. The command line has a test of `workspace export` and `workspace list`. The `workspace-transfer` browser suite covers the export command, the import window's states and choices, the dimmed folders and their menu, a kept source and pin, and choosing a workspace whose main folder is missing. A round trip between two real computers has not been tried.
