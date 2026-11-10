# A slide show on its own display

Paul asked on 10 November 2026 for a slide show that runs full screen in a window of its own, apart from Dinah's window. With two displays it should appear on the one Dinah is not on. With three or more it should appear on the largest display Dinah is not on. The same applies to every product built from Dinah. Paul approved it the same day, and his decisions are at the end. What was built is in `docs/design/detached-slide-show.md`.

## Why it does not work today

Slides presents by making an element of its own page full screen, with the browser's `requestFullscreen` (yavc.app `docs/slides-design.md`, decisions 7 and 22). In Dinah that page is Dinah's one window, so the show always covers the display Dinah is on. The presenter view opens through `window.open`, and Dinah's content host answers that with one of its in-page windows (`content-host.js`, Presenting). The show and the presenter speak over a `BroadcastChannel`, which works because both are on one page. The result is a presenter view that cannot be kept on one display while the audience watches the other, which makes it useless.

Two capabilities are missing.

1. Dinah's window comes from Wails v2, which gives a program one native window. Its screen API reports each display's size, and whether it is the current or primary one, but not where it is, so it cannot place a window on a chosen display either.
2. Slides decides where the show is drawn, and has no way to hand the show to a host. That is yavc.app's code, which Dinah takes as a release and never edits.

## The proposed shape

The show becomes a second copy of Dinah's own window program, started for the show alone, and placed full screen on the chosen display. The presenter view and the editing window stay in Dinah's window, on Dinah's display, where they are useful once the show is elsewhere.

**Dinah's side.**

- The window program gains a start option for a show, through `programs.RegisterStartOption`, the seam a product already uses for options of its own. Started so, it opens one borderless window, loads the Slides package in its show mode and nothing else, and takes no part in the bus, the chats or the workspaces.
- On Windows, the display is chosen with the documented Win32 calls. `MonitorFromWindow` gives the display Dinah's window is on, `EnumDisplayMonitors` and `GetMonitorInfoW` give every display's rectangle, and `SetWindowPos` puts the show's window over the chosen rectangle. With two displays the choice is the other one. With three or more it is the largest by area that Dinah is not on, the primary display winning a tie. With one display there is nowhere else to go, so the show stays full screen in Dinah's window, as it is today.
- The show's page and Dinah's page are in different processes, so a `BroadcastChannel` between them is not something either browser engine documents. The two pages are joined instead by a channel the host provides. The main program relays messages between the editing window and the show over a loopback connection that only it and the show process know, with a token the main program gives the show process when it starts it. The messages are the ones Slides already sends its presenter: bounded source snapshots, notes, the position and navigation.
- On Linux and macOS, until each has a documented way to place a window on a given display, the show stays in Dinah's window.

**yavc.app's side.** The applets need a second version of the `presentation` capability, and that is a request to yavc.app Development, which acts on it only with Paul's word.

- The host may offer `openShow({application, resource, channel})`. It returns a port with `postMessage`, `onmessage` and `close`, or nothing when the host cannot show the deck elsewhere.
- When the host offers it, Slides' Start Show calls it instead of making an element full screen, and drives the show through the port as it drives a presenter today. The editing window keeps the deck's authority, as it does for the presenter now.
- The show's own page is the Slides package in a show mode, which renders the snapshots it is sent, takes the keys a show takes, and sends navigation back over the same port. That is the presenter mode's arrangement with a different layout.
- Without `openShow`, Slides behaves as it does now.

## Paul's decisions

Paul decided all three choices on 10 November 2026, each as this proposal recommended.

1. The show runs in a second copy of the window program. Dinah does not move to Wails v3, which has several windows in one program and reports where each display is, because it is still an alpha release and the move would touch every part of the window.
2. The show starts on the other display by itself whenever one exists, and Escape ends it.
3. yavc.app Development builds the second version of the `presentation` capability, and was asked the same day. Dinah's side is built and tested alongside it, and nothing changes for the person until both are in.
