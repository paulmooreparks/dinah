# Editor, Docs, Diagrams and Slides as applications of their own

Paul asked on 11 November 2026 what it would take to make the four document applets stand-alone applications, and whether they could still work with Dinah, and with a product built from Dinah, for people and for agents. Dinah's team, yavc.app Development and PUDL Team Lead all answered yes and agreed on the shape. This proposal records that shape, the decisions Paul has made, the ones still open, and a first proof with the Editor. Paul authorized the Editor to be built on 11 November 2026, once the stacked taskbar buttons and a launcher label were delivered, and left the open choices to the proposal's recommendations.

## What Paul has decided

1. The applications keep their HTML and PUDL interface. A native rewrite was considered the same night and set aside.
2. An application shows one document in one window, and a document has one window. There is no window manager, taskbar or tab strip inside an application.
3. The Editor is proved first, and the other three are packaged only after it.
4. The document service is a process of its own, the file is what programs share, and the applications are named YAVC Editor, YAVC Docs, YAVC Slides and YAVC Diagrams for now. His answers to the choices this proposal first left open are at its end.

## What exists already

The four applets are packages from yavc.app, taken as one pinned release. Each asks its host for everything outside its page through written contracts: reading and saving a document, drafts and recovery, settings, pictures, the folder a document is in, and the slide show's window. yavc.app's release tests run them in a bare host that has nothing of Dinah in it. A stand-alone host is therefore a third implementation of contracts that already have two.

Dinah's side of those contracts is about twenty-six calls from the page to the program, most of them about drafts: open, read, update with the revision the text was based on, save, save as, discard, compare and merge. The draft store is a package of its own (`internal/drafts`). A save already says which revision it was based on and is refused as a conflict when the document has moved on, and Dinah already treats a file changed on disk by another program that way.

The slide show on another display, built on 10 November 2026, is a small working case of the pattern this proposal needs (`docs/design/detached-slide-show.md`). The toolkit gives a program one window, so the show is a second copy of the window program, started with an option. Its page signs in with a one-time address and a cookie of its own, and that cookie reaches a bridge with six methods and nothing else of the program.

What does not exist is everything Dinah's page gives an applet around its edges. The content windows depend on nineteen of the page's own objects, among them the workbench, the saved layout, the workspace browser and the file picker. A stand-alone page cannot carry those, and under decision 2 it does not need most of them.

## The proposed shape

**One host, four applications.** The host is Dinah's window program started with an option that says which application to be and which file to open. Four applications are four names, icons, file associations and taskbar identities over that one host and over one pinned content release. On Windows a taskbar identity is an application user model ID, which a process sets for itself with `SetCurrentProcessExplicitAppUserModelID`, and a file association is a registered command line.

**One process for each document window.** This follows from decision 2 and from the toolkit's one window for each program. The window process is as thin as the slide show's: a window, a browser view and the address to load.

**One document service for the person, shared by every window.** The window processes do not each carry the draft store, file access, the agent connection and updates. One service process holds those. The first document window starts it, later windows find it, and it ends a short while after the last window closes. It is the same program started headless, with no window of its own. A window signs in to it as the slide show's page does, and its cookie reaches only the calls for its own document.

**A page of its own.** The stand-alone page mounts one applet and gives it PUDL's ordinary page controls, menus and dialogs. It supplies the contracts' services from the document service: the document and its revisions, recovery, settings, Open and Save As through the operating system's own dialogs, pictures, and the document's folder. It has no workbench.

**The file is what the programs share.** Each program keeps its own drafts for text that is not saved yet. A save states the revision of the file it was based on, and the service refuses it as a conflict when the file has changed since, whoever changed it. That is the rule Dinah already applies to a change on disk, so two programs with the same file open behave as two editors do today: the second to save is told, keeps its edits, and chooses. The service also keeps a list of the files its windows have open, so that opening a file a second time brings its window forward.

**Agents go through the owning application.** The applets already publish their actions with schemas. The document service offers them over an authenticated local connection, and Dinah's applet broker lists a stand-alone document as a target. A call says which application and which document it is for. Nothing is ever sent to "the window last used". The person's permission is asked in the window that owns the document, as it is in Dinah today.

**A product's own rules stay with the product.** A product built from Dinah adds shortcodes and document sources of its own. A stand-alone application packaged by that product includes the product's shortcode sets, and a document from one of the product's own sources opens through the product, under the product's rules. The first proof covers files on the computer only.

## The Editor proof

The proof is one application, the Editor, on Windows, opening files on the computer.

What it must show, from the lists Quality, Release, Engine, Front End, yavc.app Development and PUDL Team Lead each gave:

1. A file opens by double-click, by Open With and from a command line, in a window of its own.
2. It is edited and saved.
3. After the window's process is ended by force, the unsaved text is offered again.
4. A save is refused as a conflict when another program has saved the file meanwhile, and the person's edits are kept.
5. Opening the same file again brings its window forward.
6. Dinah hands a document to the stand-alone Editor, and a link to a document opens it there.
7. An agent in Dinah edits the document in the stand-alone Editor through a call that says which application and document it means, with the person's permission asked in the Editor's window.
8. The person and an agent edit the same file from the two programs at once, and neither overwrites the other. Two saves that overlap in time are part of this, under the save contract below.
9. The Editor works with Dinah closed, and Dinah works with the Editor closed.
10. yavc.app's own package tests pass against the stand-alone host.

The first step of the proof is an audit, which PUDL Team Lead asked for: how much of the draft store, file access and settings can run without Dinah's workspaces, chats and bus. Its finding is in the next section.

## The audit's finding

The audit was done on 11 November 2026, by reading, before anything was built. The assumption holds.

The draft store (`internal/drafts`) depends on three small packages and on nothing of Dinah's workspaces, chats or bus. Its service is given two things: a folder to keep drafts in, and a way to find the provider of a draft's source. The provider for files on the computer (`internal/resource/localfs`) takes a folder and already does what the proof needs of it. It reads a file with the revision of its content, and it writes one only when the file still has the revision the caller expected, which is the conflict at the save. A document service is therefore the draft store, that provider, and a rule for which folder a file belongs to.

One thing Dinah does today is not good enough for a stand-alone Editor. A file that is in none of the person's workspaces is opened as a new, untitled draft that happens to start with the file's text. It has no file to save back to and no revision to compare, so Save asks for a place and a change made by another program is not noticed. The document service will open such a file as a document of its own folder, through the same provider a workspace's folder uses, so that it saves in place and meets a conflict honestly. Dinah's own Open of a file on the computer could later take the same route.

Settings and appearance are small packages of their own as well (`internal/appearance`, `internal/skins`, `internal/statefile`). File access rules, recent documents and the workspaces themselves depend on Dinah's agents and layout, and the document service does not use them.

## The save contract

PUDL Team Lead reviewed the audit on 11 November 2026 and found that the provider's write does not yet give what point 8 of the proof asks. It checks the file's revision, writes the new text to a temporary file, checks again and renames the temporary file over the target. Nothing arbitrates between two processes, so Dinah and the document service could both pass the last check before either renames, and the later rename would replace the earlier one's text without a conflict. A test that saves one after the other cannot show this. The contract below closes it for programs built from this source and says plainly what remains for any other program.

**Between programs built from this source.** Dinah, a product built from it and the document service are cooperating writers. Before its last check, a cooperating writer takes the operating system's exclusive lock on a lock file that stands for the target, and it holds the lock until the rename is done. The lock is the one Dinah's state files already use: `LockFileEx` on Windows and `flock` elsewhere, both documented to exclude a second holder whatever process it is in. Every cooperating writer finds the same lock file, because it is in one folder for the person, outside any product's own folder, and its name is made from the identity of the target's folder and the target's name. A folder's identity is what the operating system gives for it (the volume and file index on Windows, the device and inode elsewhere), so two paths to one folder give one lock. The file's own identity is not used, because the rename gives the target a new one at every save. With the lock held, the check and the rename of one writer cannot fall between the check and the rename of another. Of two overlapping saves based on the same revision, one is written and the other is refused as a conflict, with its text kept.

A writer that cannot take the lock within a few seconds fails the save and says that the file is being saved by another program. It does not report a conflict it has not seen, and it never writes without the lock.

**Against any other program.** A program that knows nothing of this lock, such as another editor, is an unrelated writer. Its change is seen, and the save refused, when it lands before the last check. A change that lands between the last check and the rename is replaced and not seen. No file interface that Windows, Linux and macOS all document compares and replaces a file's content in one step, so this is stated as the limit and not worked around. The rename itself is documented to replace the target whole, so a reader sees the old text or the new and never part of either.

**The test.** A deterministic test holds one writer between its last check and its rename, starts a second writer on the same file from the same revision, and releases the first. Without the lock both succeed, which is the fault. With it the second waits, then finds the revision changed and is refused. The test is run within one process and across two.

**Built.** The lock was built the same night, ahead of the Editor, because the fault was already there between Dinah and a product built from it when both had one file open. It is `internal/savelock`, and the provider's one write path takes it (`WriteText` in `internal/resource/localfs`). The lock files are in the person's cache folder, as the operating system gives it, under a name from this source's own product. Three tests in the provider cover the overlapping save: within one process with the lock, within one process without it, where both saves succeed and the fault is on record, and between two processes. Where a file system gives a folder no identity, the folder's path with its links resolved stands in for it, so two paths to such a folder that resolve differently are not arbitrated. That is the one case in which two cooperating writers are still as they were.

## What it would cost

The slide show took one evening for Dinah's half, with its tests and checks. By that measure the proof is several such pieces of work.

| Part | Size | Why |
| --- | --- | --- |
| The audit of what the document service can reuse | Small | Reading and a written finding |
| The window process and the start options | Small | The slide show's window is the model |
| The document service, its sign-in and its narrow bridge | Medium | Drafts, files and revisions behind one cookie for each document |
| The stand-alone page and its services | Medium | Twenty-odd calls, the operating system's dialogs, recovery |
| File association, identity and the installer | Small to medium | Per-application names and icons |
| The connection for agents and the hand-off from Dinah | Medium to large | New, and the part with the most to get wrong |
| Tests and hidden checks | Medium | Items 1 to 10 above, each as a check |

Docs, Diagrams and Slides then reuse the host. Slides asks the most of it, because of the show window, pictures and referenced diagrams, and is taken last.

## Paul's answers to the open choices

Paul answered on 11 November 2026, the night he authorized the Editor.

1. The document service is a process of its own. The applications behave the same whether Dinah is open or not, and Dinah reaches them the way any other program would.
2. The file is what the programs share, with a conflict at the save. One draft shared live between programs is not built, and can be added later without undoing this.
3. The applications are called YAVC Editor, YAVC Docs, YAVC Slides and YAVC Diagrams for now. Paul is open to another brand if the three teams agree on one that is short and free to use. The names are therefore kept in one place in the host, so that a later brand is one change.
4. Windows is first. Paul would like Linux as well if it is not arduous, and does not want macOS ruled out. The host therefore keeps what is particular to an operating system, which is the window's identity, file associations and the Open and Save dialogs, behind files of their own for each system, and the document service and the page use nothing of Windows. The Editor proof is checked on Windows. Linux is attempted after it, and what stands in its way is reported, not worked around.
5. The audit and the Editor proof go ahead, after the stacked taskbar buttons are delivered.
