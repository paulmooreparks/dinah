# A slide show on its own display

Paul asked on 10 November 2026 for a slide show that runs full screen in a window of its own, so that the presenter view can stay on one display while the audience watches the other. He approved the proposal the same day (`docs/proposals/detached-slide-show.md`), and this note records what was built in Dinah. Every product built from Dinah gets it.

Slides is yavc.app's, and its half is version 2 of the presentation capability (`docs/presentation-contract.md` in yavc.app), which arrived in yavc.app v0.2.62 the same day and was corrected in v0.2.63. Dinah's content host offers version 2 with `openShow` beside `presenterURL`.

## Which display

A show goes to a display the main window is not on. With two displays that is the other one. With three or more it is the largest by area, and the primary display wins a tie. With one display there is nowhere else, so Slides presents full screen in its own window as before. `chooseShowDisplay` in `cmd/dinah-desktop/show_display.go` is that rule.

On Windows the displays come from `MonitorFromWindow`, `EnumDisplayMonitors` and `GetMonitorInfoW` (`show_display_windows.go`). Wails v2 reports each display's size but not its position, which is why the operating system is asked. On Linux and macOS no display is offered yet, so a show stays in the main window there.

## The second window

Wails v2 gives a program one window. The show is therefore a second copy of the window program, started with `--slide-show` (`show_window.go`). It opens one borderless window with a black background and no page of its own, and loads the address the main program gave it. On its first page it moves the window over the chosen display with `SetWindowPos`, makes it full screen with the toolkit's own full screen, shows it, asks for the keyboard with `SetForegroundWindow`, and keeps the display awake with `SetThreadExecutionState`.

The main program starts that process, puts it in its own job object so that it cannot outlive Dinah, and ends it when the show ends. There is one show at a time, and a new one ends the one before. The process uses a browser profile of its own, beside the main window's.

## What the show's page may reach

The show's page is `show.html`, served by the main program's page server. It signs in with a one-time address, as the main window does, and receives a cookie of its own with the show's own token. That cookie admits it to the page's files, to the pictures of the one draft it presents, and to a bridge with six methods (`showPage` in `show_sessions.go`): `Attach` and `Post` for the show's messages, and `ReadAppearance`, `ShortcodeCatalog`, `RenderShortcode` and `ReadWorkspaceMountText`, which are the reads the Slides canvas has in the main window. It can write nothing, and no chat, workspace, setting or other draft answers it. When the show ends, its cookie opens nothing.

`show-host.js` gives Slides, mounted with `show=1`, the port from `presentation.showPort()` and the read services, and it applies the person's theme and skin so that a slide looks as it does in the window.

## How the two pages speak

The editing window and the show's page are in different browser processes, so a `BroadcastChannel` between them is not something the browser engine documents, and Dinah carries the messages itself. The editing window's port posts through `PostShow`, and hears the show through the `show:message` event. The show's page hears the editing window through an event of its own bridge and posts through `Post`. A message is JSON text that Dinah does not read, at most 8 MiB.

`OpenShow` answers as soon as the display is chosen, in a few milliseconds, and the window appears a little later. Messages posted before the show's page reads are kept in order, up to 32 MiB. The page listens for events first and calls `Attach` second, which returns what was kept, so no message is lost or delivered twice.

Closing the port, closing the deck's window, or ending Dinah ends the show. When the person closes the show's window some other way, the editing window's port hears `onclose`.

## Checks

- `show_test.go` covers the display rule, the kept and carried messages, the one-time address, and that the show's cookie reaches the six methods and its own draft's pictures and is refused everything else.
- The browser suite `show-port` covers the editing window's port: no port with one display, the order of messages, `onmessage`, `onclose`, and closing.
- A hidden window program was run on 10 November 2026 with two pretended displays, Slides from yavc.app v0.2.62, and the show's page in a second browser profile. Present from the Start got a port in 2 to 3 ms and left the editing window as it was. The show's page drew the first slide and filled its window, the show's own arrow key moved it, Leave the Show ended it, Escape in the show ended it, and a picture beside the deck loaded. With one display the answer was null and Slides presented as before.
- The real show process was also started hidden: it signed in, and its window was at the rectangle it was given.

Under test (`DINAH_SHELL_ENTRY_FILE`) no window is started. The show's address is written beside the main page's, as `entry.txt.show`, and `DINAH_SHOW_DISPLAYS` gives the displays, so a check does not depend on the machine's own.

Slides fills the show's window by itself from yavc.app v0.2.63. With v0.2.62 its root kept the height of a page in flow, and a diagram referred to by a relative path, as `{{diagram:../drawings/chart.mmd}}`, was not drawn in the show's window, because Slides took the deck's folder from the resource's text there. Both were reported to yavc.app Development and mended the same evening. With v0.2.63 in the hidden window, that diagram is drawn in the show, and a diagram whose file is missing says so.

With v0.2.63 every slide change in a show made the editing window send the whole deck again, and the show's page built a new stage for it, so each slide faded in, vanished and zoomed into place. Paul saw that on 11 November 2026 on a second display. yavc.app v0.2.64 mends it in Slides. Measured through Dinah's relay in the hidden window, a slide change from the show now sends nothing back to it, one from the editing window sends a single `at`, and the stage is not rebuilt in either case.

## What is left

1. Two things have not been seen, because no check may show a window on Paul's screen: the show's window visible and full screen on a real second display, and the keyboard going to it. Paul tries it on two displays.
2. A file a slide refers to is read when the slide is drawn. The show's page is not told when such a file changes on disk, as the canvas in the main window is, so a diagram edited during a show is redrawn at the next change to the deck.
