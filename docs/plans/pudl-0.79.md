# Bringing PUDL 0.79.0 into Dinah

This plan follows PUDL's upgrade brief for Dinah, written against Dinah at PUDL 0.78.0. PUDL 0.79.0 is tagged. Its distribution has the files the brief relies on, `pudl-tooltip.js` among them, and every glyph the plan uses below (`document`, `eye`, `gear`, `window`, `info`, `minimize-all`, `restore-all`, `close-all`). It mentions no other product, which PUDL.md asks Dinah to check of every release.

The work goes in five steps, each a commit of its own with the Go tests and every browser suite passing, so a step that goes wrong can be undone alone. The first step takes only the changes 0.79.0 makes without being asked. The others each adopt one feature. Paul decided on 23 October 2026 to move the status items into the tray and to adopt the ribbon as a preview, keeping the Editor's own toolbar while the ribbon shows; the rest of the plan is mine.

One change in step 1 was not in the brief. Dinah's stand-in rule for a window title and its detail, from 0.61.1, gave the detail priority over the title, the opposite of what PUDL does now, so a narrow window showed "#pars…" beside most of its detail. The rule now keeps the detail beside the title, which is what it was for, and lets the detail give way first, as PUDL does.

## Step 1: The release, and what changes without opting in

Both copies, `cmd/dinah-desktop/frontend/pudl` and `internal/web/static/pudl`, are replaced whole from the tag, as PUDL.md describes. PUDL.md, README.md and the decisions table in DESIGN.md then give 0.79.0.

Six things change in the window without Dinah asking for them, and each needs a look.

1. The menu titles' ellipsis rule in `dinah.css` sets `display: block` on `.menubar-title`, which breaks a title that is now a row of its glyph and its words. The rule moves onto the words, as the brief gives it: `.menubar-title > .menubar-words` with the 16em limit, and `.menubar-front > .menubar-title > .menubar-words` with the 12em limit.
2. A window's title bar buttons grow to 28 pixels, and the window menu button widens for its caret. The title and `.win-title-detail` rules in `app.css`, the stand-in from 0.61.1, are checked at Dinah's narrowest window width of 720 pixels, in both themes, with a long title and a long detail such as a Terminal's folder.
3. A maximized window now has the short, docked-style title bar. No window kind's layout in Dinah assumes the taller head as far as I can find, but the suites that restore maximized windows are run and their screenshots looked at.
4. The brand title folds to the skillet mark before the menu bar collapses. Its words become a `span.menubar-words` and stay its accessible name and tooltip, so nothing in Dinah has to change, and the keyboard suite confirms that the Dinah menu's mnemonic, N, still works.
5. Right-clicking a window's taskbar button opens that window's menu. Dinah keeps it. Nothing in Dinah answers `pudl:contextmenu` today, so nothing competes with it.
6. Folding toolbars measure themselves, which the chat composer's toolbar may ignore, as the brief says. A suite that narrows a chat window until the composer's Agent group folds, and checks the provider, mode and model submenus, comes in this step, because the 0.78.0 review found that fold was untested.

## Step 2: PUDL's taskbar

Dinah's dock moves onto PUDL's taskbar, which does what Dinah's own code does today.

- The dock is wrapped in `<footer class="win-bar">` at the foot of `main.shell-main`, keeping its attributes.
- The code PUDL now covers goes: the `.shell-main > .win-dock` padding and rule, the `.win-tab::after` hit area and `.win-dock:empty` in `app.css`, the click forwarding from the dock's padding in `workbench.js`, and the `.workspace-taskbar` rules in `dinah.css`, which match nothing.
- Minimize All, Restore All and Close All become icon buttons at the bar's right, using PUDL's own controls and glyphs. They are one press from anywhere, where today they are in the Window menu only.
- The Window menu's Close All Windows calls `pudlWindows.closeAll()`. That closes each window as its own Close would, so an Editor with unsaved changes can still refuse through `pudl:window-closing`. A suite checks that refusal, and that the Workspace pane, which the layer requires, is left alone.

The bus's web page keeps its dock in the topbar. The brief recommends leaving it, and the window is where Paul works.

## Step 3: The tray (decided by Paul: yes)

The Workspace, Notifications and agent status items could move from the topbar into the taskbar's `.status-area`, keeping their popover menus. The topbar would then hold the menu bar alone, with the ribbon's toggle at its end, and the status items would sit at the bottom right, where Windows keeps its own tray.

I recommend the move. The topbar is the scarcer room, since the menu bar and the front window's menus share it and are the first to fold on a narrow window, while the taskbar has room to the right of the window buttons. Against it, the status items move away from the place Paul has learned. The status item that still says Devin should say which agent it means, but that is separate work.

## Step 4: Glyphs on the menu titles

The host's titles get glyphs from PUDL's set: Documents `document`, Tools `gear`, View `eye`, Window `window` and Help `info`. The brand keeps its mark. Each window kind's own menus get the glyph that kind already shows in its title bar, such as `comments` for a chat, `document` for the Editor, `terminal` for a Terminal and `inbox` for the Bus pane, given through the menus' `glyph` property. The menu bar then folds a title to its glyph before it collapses to one button. A suite narrows the window and checks the order of folding and that every title keeps its name for a screen reader.

## Step 5: The ribbon, as a preview (decided by Paul: yes)

I would now adopt the ribbon, where the 0.78.0 review said it did not fit. That review imagined a ribbon drawn inside a window beside its own toolbar. In 0.79.0 the ribbon is another way of showing the menu bar itself. It is built from the same menu rows, the person turns it on and off with a toggle in the topbar, and each applet remembers the tab last chosen in it. Turning it on duplicates nothing that the menu bar does not already show.

- The menu bar gets `data-menubar-ribbon`, and the toggle is off until the person turns it on.
- The page loads `pudl-tooltip.js`, which the ribbon's glyph-only buttons need for their names.
- The common rows of the Documents, Tools and Window menus, and the Editor's Text and Lists rows, get glyphs, and `glyphOnly` goes on the rows everyone knows, such as Bold, Italic and the lists. Rows that already carry `danger` become danger buttons without further work.

With the ribbon on and an Editor in front, the Editor's formatting appears twice, in the ribbon and in the Editor's own toolbar. Paul chose to keep both while the ribbon is a preview, and to decide when PUDL marks it stable whether the Editor hides its toolbar while the ribbon shows.

The ribbon's markup may change before PUDL marks it stable, so Dinah depends only on the attributes and properties the brief documents, and its suite checks behaviour through the menu bar and the public window API, never through the ribbon's internal markup.

## What the work found for PUDL

Three findings belong to PUDL rather than Dinah, and each is worth a proposal.

1. In the ribbon, a tool whose words are folded away to save room is drawn with its words hidden by `display: none` and no tooltip, so it has no name for a screen reader and none for a pointer. A row marked `glyphOnly` is given a tooltip, but an ordinary row that the ribbon folds is not. Minimize All, Restore All, Close All Windows and All Windows in Dinah's Window tab show it.
2. Toasts stand a fixed distance from the foot of the window and cover a taskbar's tray. Dinah raises them by the taskbar's measured height; PUDL could do this itself for any page with a `.win-bar`.
3. Under heavy load, restoring windows from the address can throw from `setPlacement` when a window's markup has not arrived yet, which makes Dinah's `ui-state` suite fail now and then when every suite runs at once. The same code is in 0.78.0.

## Later, and not part of this upgrade

Resident applets would let an applet's menus stay in the bar whatever window is in front. The Bus pane is the first candidate, so that New Message and the unread count are always one press away, and the status items could follow as the brief suggests. Both wait until the tray decision is made and the resident applet is no longer a preview.

## Checks for every step

- The Go tests, `go vet`, and every browser suite, headless.
- Screenshots of the window at 1440 and at 720 pixels wide, in the light and the dark theme, looked at before the step is committed.
- A live check in a hidden window, signed in through `DINAH_SHELL_ENTRY_FILE`, so that nothing appears on Paul's screen.
