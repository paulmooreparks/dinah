# Exporting and Importing a Workspace

A workspace set up on one computer can be taken to Dinah on another computer without building it again by hand. Paul approved the feature on 26 October 2026, and Dinah gained it on 27 October 2026. This note describes Dinah's version.

## What a Workspace Export Contains

An export is the workspace's structure and nothing of its content. It is one file, `<name>.dinah-workspace`, whose extension comes from the profile (`WorkspaceFileExtension`). Until 18 November 2026 the file was a JSON document; it is now a zip archive whose `workspace.json` entry is that document, as described under "The Export File Becomes an Archive". The document holds:

- the workspace's display name;
- each source, with its kind, its display name and what identifies it, which for a local folder is its location as described below;
- which local folder is the main folder;
- the pins of resources, kept by the source they belong to and the path within it;
- for each local folder, the folders inside it, without their files, so that the other computer can show and make the same folders.

It leaves out everything that belongs to the computer or the person. Workspace and source IDs are made afresh by the importing computer. Credentials are left out, and so are drafts and the window layout. The workspace's skills are in its main folder's `.devin` folder, so they reach the other computer with the folder itself.

Until November 2026 an export also left out the workspace's chats. Paul decided on 18 November 2026 that a workspace export takes its chats, their history and the groups and teams they belong to, as described under "Chats, Their History and Their Groups" below.

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

- `RegisterSourceKind(kind, SourceKind{...})` for a kind of source. The same registration can bring the kind's form and resource provider, so that the person can add a source of it and open what it holds (docs/design/source-kinds.md). `Read` checks and cleans a source of the kind as an export has it, `Describe` gives the site and reference the import window shows, `Check` says why the computer cannot use it yet (such as a missing connection), `Export` and `Recreate` turn a registry source into an export's and back, and `DefaultName` gives a name to an imported source that has none. Every field is optional.
- `RegisterPinKind(kind, PinKind{...})` for a kind of pin. `Read` says whether a pin of the kind in an export can be kept.

A local folder is built in and cannot be registered.

A source or pin of a kind nobody registered is kept. The registry's `Source` and `Pin`, and the export's `ExportSource` and `ExportPin`, keep the JSON fields they do not know in `Extra` and write them back unchanged, so the registry in `settings.json` keeps the shape another build wrote, and an export of an imported workspace writes such a source or pin exactly as it came. The import window shows such a source as Kept, Not Checked, says how many pins are of kinds Dinah does not use, and offers nothing else for them. In the Sources tree the source is dimmed, with a Not Available Here badge, and its only command is Remove from Workspace…; the Pinned group leaves such a pin out without removing it from the record.

## Chats, Their History and Their Groups

This section is the design of 18 November 2026. Its export side was built the same day, as described under "As Built, 18 November 2026: the Export Side"; the import side has not been built yet. Paul asked to move chats, their history and their hierarchy to another computer, and decided that it is part of workspace export and import. He accepted that a moved chat continues from its saved history rather than from the agent's own session, for the reason given under "Continuing a Moved Chat".

### The Export File Becomes an Archive

A workspace with its chats can run to hundreds of megabytes, which one JSON document cannot hold sensibly. From this design on, an export is a zip archive (Go's `archive/zip`) with the same file extension. Its `workspace.json` entry is the format 1 document described above, with the fields `chats`, `groups` and `teams` added, and the chats' histories and group conversations are entries of their own beside it. A reader tells the two apart by the zip signature at the start of the file and still reads a bare JSON export written by an earlier build. An earlier build cannot read an archive and says the file is not a workspace export.

### Which Chats Go

The export takes every chat listed in the workspace, open or closed, including hidden ones. It never takes a chat in the Recycle Bin. The export window lists the chats with every one ticked, and the person can untick any of them.

### What Each Chat Takes

A chat is recorded in `workspace.json` with a key (`c1`, `c2`…) that groups, teams and chat pins refer to. Its record holds:

- the logical chat ID and the IDs of its stages, oldest first, so that the chat keeps its identity on the other computer;
- its provider and its options, including the model;
- its titles, both the one the person gave and the one Dinah judged;
- whether it is hidden;
- its folder, relative to the workspace's main folder;
- its chat pins.

Its history is the newest stage's saved transcript, in the entry `chats/<key>.json`. That one file already contains the whole visible conversation, because each stage's transcript keeps a copy of the stages before it, so the earlier stages' files are not exported. This keeps the archive to a fraction of what Dinah stores, since most of the stored size is those repeated copies. Pictures are inside the transcript and go with it. Files attached to a message were saved in the workspace folder, so they reach the other computer with the folder.

The export leaves out what belongs to the computer or the running agent: the native session records and recovery attempts, the bus identity, queued messages, the composer draft, the full chat log (whose only extra is tool output beyond 16 KB), the journal of message times, prompt recall, and applet invocations.

### Groups and Teams

`groups` lists every group chat with at least one member among the exported chats, with its title, its lead, its turn budget, whether the person is a member, and every member. `teams` lists every lead and member pair in which either chat is exported, and the team budgets of exported leads.

A member in another workspace is recorded by its logical chat ID, its title and its workspace's name. An import adds a member only when its chat is in the archive or is already on the computer. The others are shown as waiting for their workspace, and nothing is stored for them. Each workspace's export lists the whole group, so when the other workspace is imported later, its import adds the members that were waiting. Importing the workspaces in either order gives the same groups and teams.

A group is matched by its title. An import adds its members to a group of the same title that is already on the computer, and otherwise makes the group. A group that already has a lead keeps it, and the window says so when the archive says otherwise. A team pair is added when both chats are on the computer.

The group conversations go too, in `groups/<key>.jsonl`, with the files attached to them. The importing bus gives each message a new ID and records where it came from: a random identity of the exporting Dinah, kept in its settings and made on its first export, and the message's ID there. A second import that brings the same message again, such as the other workspace's export of the same group, skips it. A chat's transcript refers to the bus messages it received and sent, and the import rewrites those references to the new IDs. A reference to a message that did not travel keeps its text and loses its link.

Trusted links are not imported. A trusted link lets agents act on each other's requests as on the person's, and a file should not be able to grant that. The import window lists the links the exporting computer had, so the person can add them again.

### Paths

A chat finds its workspace by its folder's absolute path, which is usually different on the other computer. The import therefore rewrites the paths in each transcript, `workspace` and `parentWorkspace`, from the exporting computer's main folder to the importing computer's, keeping the chat's folder relative to it. It records the chats only once the main folder has been settled, as it already does for the workspace.

### Continuing a Moved Chat

The agent's own session, the part Claude or Codex keeps, stays on the computer it ran on. Neither provider documents a way to move it, so Dinah does not try.

An imported chat is recorded as moved here, with its saved history and its stages, and with no native session. Dinah never asks the provider to load a moved chat. When the person opens it, it shows the saved history. The person's next message starts a new session with the chat's provider and model, and that session receives the handoff built from the saved history. This is the same continuation that Dinah uses when a chat changes provider. The new session becomes the chat's next stage, so the chat keeps its logical ID and its place in every group and team. When the chat's provider is not on this computer, the chat opens on its saved history and asks the person to choose another provider, as it does today when a provider is missing.

### A Chat That Is Already Here

A chat is matched by its logical chat ID, and its stages decide what happens:

- When the archive's stages are the ones on this computer, or older, the chat is already up to date and is left as it is.
- When the archive's stages continue from this computer's newest stage, the chat has been worked on elsewhere and the import updates it. This is the round trip of a chat moved to a laptop and back. An open chat must be closed before it can be updated, and the window says so.
- When both computers have stages the other lacks, the chat changed on both. The import leaves this computer's chat as it is. The window offers Import as a Separate Chat, which gives the archive's chat new IDs and adds "(Imported)" to its title.

### The Windows

Export Workspace… opens a window before the file is saved. It shows the workspace's sources, and a list of its chats, each with its title, provider and model, and the size of its history, every one ticked. Below the list it says how many groups and team pairs go with them and how many of their members are in other workspaces. The window warns that the file contains the chats' whole conversations and should be kept as private as the chats are. Export… then asks where to save the file.

The import window gains two sections below the sources. Chats lists each chat with one of these marks: New, Already Up to Date, Updated From the Other Computer, Changed on Both Computers (with Import as a Separate Chat), or Close It to Update It. A chat whose provider is not on this computer also says so. Groups and Teams lists each group as New Group or Joins the Existing Group, with how many members are here and how many wait for another workspace, and then the team pairs. The trusted links that were not imported are listed last.

`dinah workspace export` writes the archive too, with its chats, and gains `--no-chats` to leave them out.

### Decisions of 18 November 2026

Paul approved the mockup, `docs/mockups/workspace-chats-transfer.html`, on 18 November 2026, and decided two further points. The archive is not encrypted for now. A passphrase can be added later as an archive entry that changes no other part of the format. A chat changed on both computers is left alone unless the person ticks Import as a Separate Chat.

He also asked whether a source can be moved to another folder on import, for a OneDrive folder under `C:\Users\paul` on one computer and `C:\Users\pparks` on the other. The import already handles that, as described under "Local Folders on the Other Computer". The folder is found through its path relative to the home folder, and Choose… points any local folder somewhere else. Chats follow the main folder wherever it settles.

### Before Building

The Engine team has an unmerged design for durable logical chat records (`release/logical-chats-current`) that changes how a chat's identity and stages are stored. The import must write whichever records master has when this is built, and the format above refers only to logical chat IDs and stage IDs, which both models have.

## What the Person Sees

- **Export Workspace…** and **Import Workspace…** are in the Dinah menu after Rename Workspace…, in the status bar's workspace menu and in the Workspace pane's Actions menu. Export saves the file where the person chooses, suggesting the main folder.
- The import window lists each source with what was found. A local folder shows its proposed path, Found or Not on This Computer Yet, and Choose…, and a missing folder also has Create It Now with the number of folders recorded inside it. Which controls a source has comes from the plan's `actions`, so the window shows only what the person can do. Its button is Import, or Add to the Workspace when a workspace already uses the main folder. The workspace is selected when its main folder is there.
- `dinah workspace list` lists the workspaces and `dinah workspace export WORKSPACE [--out FILE] [--no-chats]` writes an export. Both read the registry and never write it, and the command line has no import, because an import asks the person about folders and changes the registry, which only the window does.

## As Built, 27 October 2026

The format and its reading are in `internal/workspaces` (`export.go`, `kinds.go`, `extra.go` and `load.go`): `BuildExport`, `Location`, `Locate`, `ListFolders`, `CleanFolder`, `ReadExport` and `Load`. A source in the file has a key (`s1`, `s2`…) that its pins refer to, never a source ID. The window's side is in `cmd/dinah-desktop/workspace_export.go`, and the export file is written through `internal/statefile`, so an interrupted write leaves the old file or the new one. A missing local folder's source item is listed with `missing` set and the actions `create-folders` and, except for the main folder, `choose-folder`, and nothing can be written into it. State lists the workspaces whose main folder is missing as `missingWorkspaces`.

Go tests cover the export with hidden folders and files left out, the three ways a folder is found including another user name and an export in Downloads, a file that cannot be trusted, a registered kind, the folder limits, an import on a second computer with a folder still to come, browsing that folder's recorded folders, importing twice, Create the Folders…, moving a folder, a main folder still to come, a folder created during the import, and a source and pin of unknown kinds passing through an import and a second export unchanged. The command line has a test of `workspace export` and `workspace list`. The `workspace-transfer` browser suite covers the export command, the import window's states and choices, the dimmed folders and their menu, a kept source and pin, and choosing a workspace whose main folder is missing. A round trip between two real computers has not been tried.

## As Built, 18 November 2026: the Export Side

The archive is in `internal/workspaces` (`archive.go` and `chats.go`). `ArchiveWriter` writes the histories, conversations and files as they come and the document last, through `Finish`, which refuses a document that refers to an entry it did not write. `OpenArchive` reads a file, telling an archive from a bare document by the zip signature (`PK\x03\x04`, or `PK\x05\x06` for an archive with no entries). The format number stayed 1, because the chats, groups and teams are fields added to the document, and a reader of format 1 that met them in a bare document would ignore them; a build that cannot read the container never reaches the document, and says the file is not a workspace export.

Everything read is checked before it is used. An entry's name must be one that cannot leave the folder it would be extracted into: no empty, `.` or `..` part, no `\` or `:`, and no absolute path. A name may not come twice, even in another case. Each entry has a limit, which is 8 MB for the document, 1 GB for a history or a conversation and 25 MB for a file (the most the bus accepts), and the archive's contents are limited to 16 GB in all and 200,000 entries; a limit is enforced on what is read as well as on what the entry's header says. Keys of chats and groups must be letters, digits, `-` and `_`, since a key is part of an entry's name, and must be unique, and so must logical chat IDs and stage IDs. A reference to a chat whose key belongs to another chat is refused, and so is a chat folder that leaves the main folder. A history must be a JSON object whose session is the chat's newest stage. A conversation must have as many lines as the document says, each with an ID of its own, and a file is checked against the SHA-256 its name gives. An entry this build does not know, with a safe name, is passed over, so a later build can add one. The new records keep the fields this build does not know, as sources and pins do.

The window's side is in `cmd/dinah-desktop/workspace_export_chats.go`. `PlanWorkspaceExport` finds the chats as the Chats list does (`saved_chats.go`), from master's logical chat records, with hidden chats included, and keeps those whose folder belongs to the workspace, which is the workspace whose main folder holds the folder most closely. It never takes a chat in the Recycle Bin, a chat whose ownership needs repair, or a chat with no saved history. `ExportWorkspace(id, choice)` takes the person's choice as the logical chat IDs that stayed ticked, so a chat made after the window opened is not taken. A chat's history is the newest stage's saved transcript, written as Dinah reads it, without the record of which queued messages started. Group members, leads and senders are found from their bus identities as the org chart finds them, and an identity that belongs to no chat this window lists, such as one shared by two chats or one that runs outside the window, is passed over. Setting a lead makes the team's own group, so a team usually brings that group too. A file attached to a group message goes when it is still a regular file of at most 25 MB with the SHA-256 it was sent with; otherwise its record stays, without `file`.

Two points differ from the design above. A chat's pins stay in the document's `pins`, in their place among the workspace's pins, as pins of kind `chat` whose `chat` is the chat's key, so the Pinned group keeps its order; a pin to a chat that does not go is left out. The exporter identity is kept in its own file, `exporter.json` in the state folder, written through `statefile.Update`, and not in `settings.json`, because the command line also exports and must be able to make the identity while the window runs, and only the window writes `settings.json`.

`dinah workspace export` writes the archive with `--no-chats` itself. With chats it starts the window program beside it, hidden and with no window, with the option `--export-workspace` (`workspace_export_option.go`), since the logical chat records belong to that program. The option reads `settings.json` and `chat-inventory.json` as they are on disk, without finishing a save the window has pending, and changes no state but the exporter identity. Without `--out` the archive is written to the standard output. An export file is written to a temporary file beside it and moved into place when it is complete, since `statefile` writes JSON only and keeps a copy of the old content, which an archive of hundreds of megabytes should not have.

The export window is `#workspace-export` in `index.html`, drawn by `workspaces-applet.js` with the shipped dialog and import classes. Each line about a group or team is held at the height of its tallest wording once the window opens, so ticking a chat moves nothing below it. With the lead of a team left behind, the window says that the team's budget does not go and that its members rejoin it on a computer that has the lead, which follows the rule above that a pair goes when either chat goes; the mockup had said that the team does not go.

Go tests cover an archive round trip with fields this build does not know, a bare export, unsafe entry names and the other refusals, histories and conversations that do not match their document, the exporter identity made once by callers at the same moment, a plan that includes a hidden chat and leaves out a chat in the Recycle Bin and another workspace's chat, an archive with stages, folder, chat pin, a group with a member in another workspace, its conversation and file, and a team with its budget, an export with the lead unticked, and the command line's option reading the state without changing it. The `workspace-transfer` browser suite covers the export window: every chat ticked, a closed and a hidden chat marked, the total and the group and team lines as chats are unticked, the warning not moving, a cancelled save leaving the window open, and Export… sending the ticked chats. The import still reads only an archive's sources and pins.
