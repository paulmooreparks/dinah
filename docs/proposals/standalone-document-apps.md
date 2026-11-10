# Editor, Docs, Diagrams and Slides as applications of their own

Paul asked on 11 November 2026 what it would take to make the four document applets stand-alone applications, and whether they could still work with Dinah, and with a product built from Dinah, for people and for agents. Dinah's team, yavc.app Development and PUDL Team Lead all answered yes and agreed on the shape. This proposal records that shape, the decisions Paul has made, the ones still open, and a first proof with the Editor. Nothing in it is built.

## What Paul has decided

1. The applications keep their HTML and PUDL interface. A native rewrite was considered the same night and set aside.
2. An application shows one document in one window, and a document has one window. There is no window manager, taskbar or tab strip inside an application.
3. The Editor is proved first, and the other three are packaged only after it.

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
8. The person and an agent edit the same file from the two programs at once, and neither overwrites the other.
9. The Editor works with Dinah closed, and Dinah works with the Editor closed.
10. yavc.app's own package tests pass against the stand-alone host.

The first step of the proof is an audit, which PUDL Team Lead asked for and which has not been done: how much of the draft store, file access and settings can run without Dinah's workspaces, chats and bus. This proposal assumes the draft store can, because it is a package of its own, and has not measured it. If the audit finds otherwise, the proof stops there and the finding comes back to Paul.

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

## Choices for Paul

1. Whether the document service is a process of its own, or Dinah when Dinah is running. This proposal recommends a process of its own. The applications then behave the same whether Dinah is open or not, and Dinah reaches them the way any other program would.
2. Whether the file is what the programs share, with a conflict at the save, or whether one draft is shared live between programs. This proposal recommends the file. It is the rule already in use, it needs nothing new between programs for a person's edits, and live sharing can be added later without undoing it.
3. What the four applications are called, and whether a product built from Dinah ships them under its own names.
4. Whether the proof is Windows alone. This proposal recommends that, since the slide show's window placement and the file associations are written for Windows first.
5. Whether to go ahead with the audit and the Editor proof.
