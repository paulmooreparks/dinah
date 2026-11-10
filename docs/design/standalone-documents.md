# The stand-alone document applications

Paul authorized the Editor as an application of its own on 11 November 2026 (`docs/proposals/standalone-document-apps.md`). This note records what was built, starting that night, and what each proof point of the proposal rests on. The applications are named YAVC Editor, YAVC Docs, YAVC Slides and YAVC Diagrams for now, and only the Editor is offered until the others are proved. The names are made in one place, `cmd/dinah-desktop/document_apps.go`.

## Three kinds of process

An application shows one document in one window, and the window toolkit gives a program one window. A stand-alone application is therefore the window program started with an option, and there are two such options.

`--document APPLICATION [FILE]` is a document's window (`document_window.go`). It finds the document service, starts it when there is none, and asks it for the document. It then opens one window on the address the service gave it. From then on it does only what the service asks of the window: come forward, change the title, show the system's Open or Save dialog, close. Everything else is between the page and the service.

`--document-service` is the document service (`document_service.go`, `internal/docservice`). It has no window. The first document window starts it, later windows find it through a file it writes in its folder, and it ends twenty seconds after the last window has gone. It serves the page, and it is where the windows' shared things are: which files are open, the reads and writes of those files, the unsaved text kept for recovery, and the applications' settings.

The main window is the third kind and is unchanged. It needs neither of the others, and they do not need it.

The service's folder is `documents` in the product's state folder. A product built from this source has its own state folder and so its own service.

## What a page may reach

A document's page is `document.html`, served by the service under `/d/` and the document's ID. It signs in as the main window and the slide show do, with an address that is good once and for a minute, and receives a cookie. The cookie is sent only under that document's path, and it holds that document's own token, so one document's page has nothing to present for another's.

The page's bridge is the `Page` type in `internal/docservice/page.go`. It reads and writes the file the window was started for and the files the person chose in that window's own Open and Save dialogs. Any other path is refused. A file a document refers to, such as a diagram beside it, is read only from the folder of one of those files or below it, and a picture is served only from beside one of them.

The service's other face is the control interface under `/control`, for programs: a window's process, and any other program of the product that wants a document opened or acted on. It wants the token in the service's file, which is in the person's own state folder, and it refuses a request that carries a browser's `Origin`.

## The file, and what is not saved yet

The service keeps no copy of a document and there is no draft store behind the page, as there is in the main window. The page gives the Editor yavc.app's written contracts and nothing else, as yavc.app's own bare test host does.

- A resource is a file's full path, since the applets show it.
- A read is the file as it is now, with the revision of its content.
- A write names the revision it was based on. The file provider compares and replaces under the save lock every program of the product takes (`internal/savelock`), so a file that any of them changed meanwhile refuses the write and is left as it is. The Editor keeps the person's text and says why.
- Save As uses the system's dialog, which asks before an existing file is chosen. The revision of that file is kept with the choice, and the write is refused if the file changed between the choice and the write. The Editor's own check of the name runs first.
- What the person has not saved is in the Editor's recovery snapshot, which the page sends to the service a second after each change and at once when the window closes. The service keeps it in a file for that document and gives it to the next window that opens the same file. The snapshot holds the revision the text was based on, and the Editor offers it as a draft of that revision, so a file that changed meanwhile is not overwritten by it.

A window with a document that has a file closes without a question. Its unsaved text is offered when the file is next opened. This is what the Editor's contract means by a host that remembers a closed document, and it is how the Editor behaves on yavc.app. A changed document that has no file yet could not be opened again, so the window asks before it closes, and its text is kept only for a window that ends by force. The next blank window shows it.

A window's document can change under it. A blank window opens a file in place, and Save As gives a document another place. The page tells the service which file the Editor shows whenever that changes, and the recovery record and the open-file list follow what the Editor says, not what the window was started for.

## One window for each file

The service knows which file each window shows. A window process that asks for a file some window of that application already shows is told so and given that window's process ID. It lets that process come forward, asks it to, and ends. Two paths are the same file when the operating system says so of the two as they are at that moment (`os.SameFile`), or when they are the same text.

When the Editor wants another window, for New or for Open from a window that is in use, it asks through the `documentWindows` capability. The service starts a window process with a ticket, the new window's Editor takes the request, and its answer goes back to the Editor that asked.

## Other programs and agents

Another program of the product opens a document in a stand-alone application by starting the window program with `--document`, or through the service's control interface. An action on an open document names the document by its ID. It reaches the Editor in that document's page, and the page asks the person first, in the window that owns the document. The person can allow the one action, allow that sender until the window closes, or refuse.

## Per system

The service and the page use nothing particular to an operating system. What is particular is in files of its own for each system (`document_window_windows.go`, `document_window_other.go`): the taskbar identity, which on Windows is an application user model ID set with `SetCurrentProcessExplicitAppUserModelID`, and letting another process come to the front, which is `AllowSetForegroundWindow`. The Open and Save dialogs and the close question are the window toolkit's, which has them on Windows, Linux and macOS.

## Checks

`internal/docservice/service_test.go` covers the cookie's reach, the control token, a refused save after a change on disk, a file made by its first save, the second open of a file, recovery after a window ends by force and across a new service, the document with no file, Save As and its refusal, a window asked for by another, reads limited to the document's folder, an action and its answer, kept settings, and two starts making one service.

A service started with `--headless` starts no window processes and stays until stopped. A test asks its control interface for a document and opens the address it is given in a browser of its own, reads the windows the service would have started, and says what the system's dialogs will choose. yavc.app's package test for this host runs that way. A window started with `DINAH_DOCUMENT_ENTRY_FILE` set stays hidden, loads no page, and writes its page's address and what the service asked of it to that file.

## Limits

1. A file is read and written as UTF-8 text of up to 1 MiB, which is the file provider's bound. A file in another encoding, or with a byte order mark, is not kept as it was. yavc.app has contracts for encodings that this host does not implement yet.
2. A recovery record is found by the path the file was opened with. A file reopened by another path to it is not offered its record, which stays under the first path.
3. A window that loses the service says so and closes. The text typed up to the last second was already sent.
4. Nothing built here prunes old recovery records. There is one small file for each document ever edited.
