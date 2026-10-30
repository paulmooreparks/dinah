# Content Packages

A content package is one ZIP of skills, and of anything else the build knows how to use, that a person installs into a workspace or for themselves, updates by installing a newer version, and removes again. Dinah gained the installer on 27 October 2026, together with the general part of package updates. This note describes Dinah's version.

## What a Package Contains

A package is a ZIP whose files sit where they are to be installed, relative to a workspace's main folder, with a manifest beside them. The manifest is at the profile's `PackageManifest`, which is `.dinah/setup-manifest.json` in Dinah, both in the ZIP and in the folder the package is installed in. It is JSON with these fields:

- `schema`, which is 1;
- `name` and `version`, where the name is also the name of the package's folder when it is installed for the person;
- `built`, the date it was built;
- `min_tool`, the oldest version of the profile's `PackageTool` (`dinah` in Dinah) that the package works with, or empty;
- `roots`, the folders and files the package owns, a folder written with its slash;
- `files`, each file's slash-separated path and the SHA-256 of its content.

The manifest is a compatibility contract between whoever builds packages and every installed copy of the program. Fields are only ever added, and `schema` changes only for a change an older reader would misread, which it then refuses. Every listed file must lie under a root, every file in the ZIP must be listed with the right hash, and a path that is absolute, escapes the folder or is not in clean slash form is refused. A ZIP may have at most 20,000 entries, 64 MB in one file and 256 MB in all, so that a damaged or hostile ZIP cannot use up the computer's memory.

## Installing in a Workspace

An install and an update are the same operation, and both are planned before anything is written. For each path the plan compares three hashes: the one the installed manifest recorded, the file in the workspace now, and the one in the new manifest. A file that is missing is added, one that matches is left alone, and one that differs is replaced. When the file in the workspace differs from what the last install wrote, the person changed it, and the plan says so. A file the last version shipped and this one does not is removed, so that a retired skill stops being offered. A folder whose files were put there by hand has no record to compare with, so the plan warns that changes cannot be told apart and that nothing can be retired.

Every file replaced or removed is copied first to a backup folder outside the workspace, `setup-backups` in the person's home folder by default, whether or not the person changed it. The files are then written, each to a temporary file in its folder that is renamed into place, and the manifest is written last. A run that fails part-way therefore leaves the previous manifest in place and can be run again. A package with another name than the one installed is refused, and so is one that needs a newer program, although a development build whose version cannot be compared is only warned about.

Removing a package backs up and removes every file its manifest records that is still there, then the manifest. Files the package never shipped are never touched.

## For You, in Every Workspace

A package can be installed for the person instead, so that every workspace has its skills however it is set up. The package is unpacked unchanged into its home, `packages/<name>` in the state folder, by the same install as a workspace's, so versions, the hand-changed check and backups work in the same way. The items of each kind of package that has a personal folder are then copied there. Skills go to the person's own skills folder, the one Devin reads in every workspace.

A package's files refer to one another by paths relative to the main folder, such as `.devin/rules/style.md`. In each copy, a path that begins with one of the package's roots is made absolute. It leads to the copy when it refers to an item that is copied, and into the home otherwise. A kind can change the copied text further, for example to rewrite a call from one of its items to another item of the same package.

A ledger in the home, `personal-copies.json`, records each copy and its hash as written. An update replaces only copies the ledger records, backs up every copy it replaces or removes, and removes copies the new version no longer makes. An item that exists and that the ledger does not record belongs to the person, so the plan lists it as in the way and refuses to install until the person moves it. A package that brings a ledger of its own is refused, since the ledger decides what a later update removes, and a recorded copy outside the person's kind folders is never removed.

Under a state folder given for a test or a trial run (`--state-dir` on the command line, or a test's state folder in the window), every personal folder is inside the state folder, so nothing of the person's own is touched.

## Kinds of Package

The kinds a package may contain are `workspaces.PackageKinds`, the same kinds that make a workspace's package collections. Skills are built in. A build adds a kind when it starts with `workspaces.RegisterPackageKind(key, workspaces.PackageKind{...})`, in the style of `RegisterSourceKind`, which is how another build can add kinds of its own from its own repository. A kind's items are in `.devin/<key>/` of a package and of a main folder (`workspaces.PackageFolder`). Every field is optional:

- `Label` is how people see the kind, as in "Workspace Skills".
- `Personal(state, isolated)` returns the folder a personal install copies the kind's items to. Without it, the items stay in the package's home.
- `FileItems` says that each file in the kind's folder is one item, as a flow is. Otherwise each folder is one item, as a skill is. An item is what a personal install refuses to overwrite when it did not write it.
- `Rewrite(text, items)` changes the text of each copied file after its paths are made absolute.

Registering a kind twice, or registering skills, panics, as registering a `database/sql` driver twice does.

### A Kind This Build Does Not Know

A package with a kind this build does not know is installed, not refused. In a workspace the kind's files are ordinary files in the main folder, where a later build finds them. For the person they stay in the package's home, and the plan lists their folders under Kept in the Package's Folder, with the explanation that the program does not copy them to the person's own folders. References to them from the copied skills lead into the home, so the skills still find them.

Passing such a kind through is safer than refusing it. Its files reach none of the person's folders, so they cannot overwrite anything, and they are backed up and removed like every other file of the package. When a build that knows the kind installs the next version, the ledger has no copies of it yet, so the kind's items are added as copies like any new item. Refusing would only stop a person from installing another build's packages in a stock Dinah. The workspace export made the same choice for sources of a kind the build does not know. The listing cannot tell a kind from a plain folder such as `.devin/rules/`, which is never copied either, so both are listed, and the statement is true for both.

## Package Updates

Packages are updated from package sources, places where newer versions can be found, such as a web page that packages are attached to. A build registers each source when it starts with `setup.RegisterSource(id, setup.Source{...})`:

- `Label` is how people see the source.
- `Enabled()` says whether the person has the source turned on. Without it, the source is always on.
- `Find(ctx)` lists the candidates the source offers, each with its `Version` and a `Fetch(ctx, dir)` that puts the ZIP in the folder updates are kept in and returns its path.

`setup.FindUpdates` asks every source that is on. It fetches a candidate only when it is newer than something installed, or when packages not yet installed were asked for, and it reads which package a candidate is from the fetched manifest. A file that is not a package is removed after it is read, so that a corrected one is fetched at the next check. Versions are compared as numbers separated by dots, so 17 is newer than 16 and 1.10.0 newer than 1.9.9. An error from one source does not stop the others.

When at least one source is registered, the window asks the sources when it starts and every six hours after. It offers each newer version once in Notifications, with Update…, saying what of the person's is in the way, which is either an item the program did not install or files changed by hand. Nothing is installed without asking, because a source is only as trustworthy as whoever can put a file there, and a package brings skills the agents follow; Paul decided on 22 October 2026 that only a signed package may be installed without asking, and packages are not signed. Update Packages…, in Settings › Packages and the Dinah menu, asks the sources at once and shows what is installed and what is new, and its one button shows each install's plan in turn.

Dinah registers no source. Its window starts no check, offers nothing in Notifications, and shows neither Update Packages… nor any setting for updates, because a command or a setting with nothing behind it is an action that cannot happen.

## What the Person Sees

- **Settings › Packages** lists the packages installed for the person and in the workspace in front, each with Remove…, which shows what goes and removes it on Remove. Install From a File… asks whether the package is for the person in every workspace or for this workspace only, asks for the ZIP, shows the plan and installs on Install. The plan shows the version, the files added, updated and removed, where it goes, skills the workspace has its own copy of, the folders kept in the package's folder, the files changed by hand, and any warning. After an install or a removal, the agent in front restarts to read the skills, unless a chat is working.
- `dinah package install ZIP [--personal] [--workspace DIR] [--backup-dir DIR] [--dry-run] [--json] [--state-dir DIR]` installs or updates a package.
- `dinah package status [--workspace DIR] [--state-dir DIR]` prints the packages installed for the person and in the workspace as JSON.
- `dinah package remove [--personal [--name NAME]] [--workspace DIR] [--backup-dir DIR] [--dry-run] [--state-dir DIR]` removes one. Without `--name`, it removes the one package installed for the person, and refuses when there are several.

`dinah setup` still prints the Claude Code hook configuration, as before.

## As Built, 27 October 2026

The installer is in `internal/setup`: `setup.go` has the manifest, `PlanApply`, `Apply`, `PlanRemove` and `Uninstall`; `personal.go` has `Targets`, `PlanPersonal`, `ApplyPersonal`, `PlanRemovePersonal` and `RemovePersonal`; and `sources.go` has the source registry, `InstalledPackages`, `FindUpdates` and the obstacle messages. The kind registry is in `internal/workspaces/collections.go`. The window's side is `cmd/dinah-desktop/packages.go` and `package_updates.go`, and the command line's is `cmd/dinah/package.go`. The profile has `PackageManifest` and `PackageTool`.

Go tests cover a fresh install, an update that replaces, retires and keeps the person's changes, an update over files put there by hand, a program too old, a backup inside the workspace, damaged packages, a ZIP without a manifest, a different package, a package with its own ledger, and the manifest's shape. Personal installs are tested with document types and flows registered the way another build would register them, for absolute references, flow calls, an update, a removal, something in the way, and a kind the build does not know. Package updates are tested with sources that offer newer, older and broken files, a source turned off and one that fails, and in the window for an offer made once and its reason. The command line has tests of a personal install, status and removal, and of a workspace install. The `packages` browser suite covers the install dialog and plan, something in the way, a workspace install, Remove…, the absence of updates with no source, and an offer and Update Packages… with one.
