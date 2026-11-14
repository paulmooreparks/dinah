# The stand-alone document applications

Paul authorized the Editor as an application of its own on 11 November 2026 (`docs/proposals/standalone-document-apps.md`). This note records what was built, starting that night, and what each proof point of the proposal rests on. The applications are named YAVC Editor, YAVC Docs, YAVC Slides and YAVC Diagrams for now, and only the Editor is offered until the others are proved. The names are made in one place, `cmd/dinah-desktop/document_apps.go`.

## Three kinds of process

An application shows one document in one window, and the window toolkit gives a program one window. A stand-alone application is therefore the window program started with an option, and there are two such options.

`--document APPLICATION [FILE]` is a document's window (`document_window.go`). It finds the document service, starts it when there is none, and asks it for the document. It then opens one window on the address the service gave it. From then on it does only what the service asks of the window: come forward, change the title, show the system's Open or Save dialog, close. Everything else is between the page and the service. A dialog runs beside the loop that asks the service for commands, because the service takes a window that has stopped asking to have ended, and a person may leave a dialog open for any length of time.

`--document-service` is the document service (`document_service.go`, `internal/docservice`). It has no window. The first document window starts it, later windows find it through a file it writes in its folder, and it ends twenty seconds after the last window has gone. It serves the page, and it is where the windows' shared things are: which files are open, the reads and writes of those files, the unsaved text kept for recovery, and the applications' settings.

The main window is the third kind and is unchanged. It needs neither of the others, and they do not need it.

The service's folder is `documents` in the product's state folder. A product built from this source has its own state folder and so its own service.

## What a page may reach

A document's page is `document.html`, served by the service under `/d/` and the document's ID. It signs in as the main window and the slide show do, with an address that is good once and for a minute, and receives a cookie. The cookie is sent only under that document's path, and it holds that document's own token, so one document's page has nothing to present for another's.

The page's bridge is the `Page` type in `internal/docservice/page.go`. It reads and writes the file the window was started for and the files the person chose in that window's own Open and Save dialogs. Any other path is refused. A file a document refers to, such as a diagram beside it, is read only from the folder of one of those files or below it, and a picture is served only from beside one of them.

The service's other face is the control interface under `/control`, for programs: a window's process, and any other program of the product that wants a document opened or acted on. It wants the token in the service's file, which is in the person's own state folder, and it refuses a request that carries a browser's `Origin`.

## The web root

A document written for a site may show a picture by a site path, such as `/images/chart.png`, which starts at the top folder of the site and says nothing about where that folder is on this computer. The person says where it is with Set Web Root in the application menu, which shows the system folder dialog. The folder chosen must contain the document. It is then the web root of every file under it, in every application, and it is kept in `webroots.json` in the service folder (`internal/docservice/webroot.go`). Clear Web Root forgets it.

Nothing guesses a web root, and a document cannot state one for itself. Paul decided both on 11 November 2026. A folder name such as `wwwroot` is one site's habit, and a document that could say where its web root is could point the page at a folder of its own choosing, on a computer where that path means something else.

The page's picture resolver returns nothing for a site path when the document has no web root. With one, the service reads the file from that folder through `os.Root`, after cleaning the path, so that neither `..` nor a link leads out of the folder. It serves pictures only, as it does for the files beside a document.

## The file, and what is not saved yet

The installed YAVC Editor program selects Docs for `.md` and `.markdown` files, ignoring case, and Editor for plain text. Its executable, Open With name, permissions and file recovery keys remain the same. Command-line opens and the registered shell command use the same service selection. Open in a blank untouched window may replace its applet in place; Open in a used window allocates or raises a window through the existing document-window contract. The host supplies the applets' documented file-resolution handoff. A New request from Docs retains the Docs applet in its new blank window, without registering another application with the operating system.

The compound `.slides.md` suffix remains in Editor because the bundled Docs explicitly refuses slide decks. This preserves the previous source-editing behavior until a stand-alone Slides application is offered. File applet selection uses the longest configured filename suffix.

An in-place replacement first drains the blank mount's state and snapshots, then asks the service to select a file already granted by the picker. The service refuses a used window or a file already open elsewhere. The host retires the old snapshot handles before destroying the old mount, preventing its asynchronous cleanup from writing over the incoming document. The new applet receives that file's recovery record and the existing resource, save and permission services. No vendor package is modified.

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

## Installed under a name of its own

`dinah install` ends by running the window program with `--document-install`, and `--document-uninstall` takes away what that added. Each application is installed as a copy of the window program under a name of its own beside it, `yavc-editor` for the Editor, and the program started under that name is that application: `yavc-editor notes.md` opens the file, and `yavc-editor` alone opens a blank window. The system then has one program to show by that name, to pin and to offer in Open With, and every window of the application is that program, including the ones the service starts. A product built from this source names its copies after its own command, so two products on one computer keep their own.

On Windows the program is made known for the person only, under `HKEY_CURRENT_USER\Software\Classes`, in the three ways Windows documents: a ProgID with the command that opens a file, the program under `Applications` with the name Windows shows for it and the kinds of file it supports, and the ProgID under each kind's `OpenWithProgids`. That lists the Editor in Open With for `.md`, `.markdown` and `.txt` and leaves each kind's default as it was, so a double-click opens the Editor once the person has chosen it for that kind. A shortcut goes in the Start menu. On Linux a desktop entry in the person's own applications folder does the same, as the Desktop Entry Specification describes. On macOS nothing is registered yet, since an application there is a bundle.

## Per system

The service and the page use nothing particular to an operating system. What is particular is in files of its own for each system (`document_window_windows.go`, `document_window_other.go`, `document_register_windows.go`, `document_register_other.go`): making the application known to the system, letting another process come to the front, which on Windows is `AllowSetForegroundWindow`, and the taskbar identity of a window started from the window program's own name, which is an application user model ID set with `SetCurrentProcessExplicitAppUserModelID`. A window started from the application's installed name needs none, since it is already a program of its own. The Open and Save dialogs and the close question are the window toolkit's, which has them on Windows, Linux and macOS.

## Checks

`internal/docservice/service_test.go` covers the cookie's reach, the control token, a refused save after a change on disk, a file made by its first save, the second open of a file, recovery after a window ends by force and across a new service, the document with no file, Save As and its refusal, a window asked for by another, reads limited to the document's folder, an action and its answer, kept settings, and two starts making one service.

A service started with `--headless` starts no window processes and stays until stopped. A test asks its control interface for a document and opens the address it is given in a browser of its own, reads the windows the service would have started, and says what the system's dialogs will choose. yavc.app's package test for this host runs that way. A window started with `DINAH_DOCUMENT_ENTRY_FILE` set stays hidden, loads no page, and writes its page's address and what the service asked of it to that file.

## Limits

1. A file is read and written as UTF-8 text of up to 1 MiB, which is the file provider's bound. A file in another encoding, or with a byte order mark, is not kept as it was. yavc.app has contracts for encodings that this host does not implement yet.
2. A recovery record is found by the path the file was opened with. A file reopened by another path to it is not offered its record, which stays under the first path.
3. A window that loses the service says so and closes. The text typed up to the last second was already sent.
4. Nothing built here prunes old recovery records. There is one small file for each document ever edited.
