# PUDL Distribution

The `pudl/` folder is the whole `dist/` folder of [PUDL v0.103.0](https://github.com/paulmooreparks/pudl/releases/tag/v0.103.0), including minified copies and the full font set, which Dinah does not load. The bus pages in `internal/web/static/pudl` hold the same copy. `VERSION` records the base release, without the tag's "v".

The 7 November source adoption uses the unmodified distribution from PUDL v0.103.0, commit 12abb6a8dbee5258d82f85baefa3e28ec707c5b5. Both vendor copies match all 193 released distribution files. This includes complete window stack persistence and the queued Start focus correction.

To update PUDL, replace both folders whole with the new release's `dist/`, write its version into `VERSION`, and run the Go tests and the browser suites in `e2e/`. Changes to PUDL itself go to Paul as a proposal first; publishing a release of PUDL needs his say-so.

The window loads PUDL's readable files from its own assets, because they are what PUDL documents and what a stack trace should show. `index.html` lists the modules it uses, with the theme script first, the applet runtime and tabs before Dinah's own scripts, and the window manager last, so that every window restored from the address has its applet registered when the manager starts. Dinah does not load `pudl-actions.js` or `pudl-actions-core.js`, since its applet broker in Go decides which actions an applet may take, asks for consent, and keeps the record. Dinah's own styles are in `brand.css`, `dinah.css` and `app.css`, with one stylesheet for each applet that needs one.

Dinah depends on these parts of PUDL's behaviour:

- Editors and restored chats are registered with `start: 'idle'`, so a window restored behind others starts its applet when it is seen, or in idle time after the windows in sight.
- PUDL keeps no window titles, so `workbench.js` keeps each open window's last title and detail in Dinah's UI state (`dinah-window-titles`) and gives them to the window's markup.
- A line moved between windows snapped side by side is part of the window address (`grid=`), which `workbench.js` saves with the layout, so it survives a restart.
- The window layer turns on PUDL's window switcher with `data-win-switcher` and gives its words in Title Case through `data-win-text-switcher`, `data-win-text-switch-last` and `data-win-text-switch-back`.
- A menu's mnemonic letter must appear in its label, which is why the Dinah menu uses N and the Editor's Insert menu uses S.
- A toolbar's tab stop has a `tabindex` attribute only where PUDL changed it, so a test of the tab stop reads the `tabIndex` property.
- A chat window's title bar shows its state as PUDL's title-bar status chips (0.88.0, `WINDOW-STATUS.md`). A window kind registered with `statuses: true` has an empty `.win-statuses` group in its markup from the start, because PUDL adopts the group with the window; the chat fills it with `.win-status` spans (Working is `strong`) and changes only those spans, since PUDL keeps its own compact button in the group. PUDL adds the Window status command to the window's commands, and Dinah's menus keep it.
- A folding toolbar folds again when one of its menus closes, if what it holds changed while the menu was open (0.93.1), so the chat's composer toolbar follows a turn starting or ending, or the Subagents count changing, behind an open menu; the composer-fold suite checks both directions. PUDL does not watch a change to a control's class or inline style, so a page making one that changes a width calls `pudlToolbar.fit(bar)`. Dinah's one such change is a theme's fonts, which `appearance.js` sets as inline styles on the page, so it folds every folding toolbar again as a theme is applied and again when the theme's fonts have loaded.
- The ribbon shows its Ribbon tabs chooser only when its tabs do not fit (0.87.1).
- The window layer turns on taskbar ordering with `data-win-taskbar-reorder` (0.89.0): entries move by dragging, by Alt+Shift+Left and Right, from a window's Taskbar order menu, or in the Arrange taskbar popover. PUDL keeps the order in the window address as `tb.order`, and Dinah saves that address with the layout, so the order survives a restart.
- Every Dinah theme sets `--category-1` to `--category-8` in both modes (0.91.0), since each changes the surfaces and the lighting. Theme validation checks PUDL's category rules (`theme_categories.go` and `appearance.js`), and `brand.css` repeats PUDL's forced-colours rule for them because it sets them after PUDL's stylesheet. `docs/design/appearance.md` explains how themes saved before this release get them.
- When a newer build is installed, `update.js` raises a toast whose action (0.90.0) is the page's `restart-now` command, registered with pudl-commands on `document.body`; the button takes the command's label without its ellipsis. The toast stays until dismissed, as PUDL requires of a toast with an action, its words say what Restart Now does, and the same step stays in the status bar's Update item and in a Notifications card.
- The Editor's diff view is a CodeMirror merge view with its own markup, so `diff-applet.css` meets PUDL's diff rules (0.93.0) through `--diff-add`, `--diff-del`, `--diff-add-bg` and `--diff-del-bg`: the fills, the `plus` and `minus` glyphs on every changed line, 2px word underlines, and PUDL's forced-colours treatment of solid and dashed edges and struck-through removed words. `appearance.js` sets the two fills for each theme to the strongest tint, at most 10 percent, at which the theme's syntax colours keep 4.5:1, as `docs/design/appearance.md` describes.
- Of the glyphs 0.92.0 adds, the diff window uses `compare` and the Files window's Upload Files uses `upload`; a package's update notification uses `download`, which now also means bringing something here from elsewhere. The Subagents windows use `branch` for a subagent beneath its chat, which is still its meaning; Dinah has no version-control branch.

Desktop enables `data-win-dock-splits` with shared tabs on all four edges. PUDL owns split placement, tab menus and `ds.<edge>` URL state. The workbench regression exercises split creation and URL serialization at each edge. The bus page does not enable shared docks.
- A chat's state chip is PUDL's execution-state badge, a `.badge` with `data-state` (0.96.4), the chip a taskbar stack's rows draw, so the Members dropdowns, the Chats lists and the stack rows show one chip. Background work is the working state with Dinah's words. A lifecycle word, such as Closed or Elsewhere, has no state and is a plain muted badge (`chat-state.js`, `chat-state.css`).
- A chat window with no execution state sets `data-win-lifecycle` (0.96.5) to its chip's lifecycle word, Closed, Retired, Elsewhere or Unknown, so its taskbar stack row shows that word as a plain badge. PUDL sizes the stack list to its longest title and the state column to its widest badge, so Dinah keeps no rule of its own for either (`chat-applet.js`, `app.css`).
- A command shortcut pressed with nothing focused, as after a window is activated by its title bar, reaches the front window's applet (0.96.7); the chat window's F2, Rename Chat, is checked in `e2e/shortcut-unfocused.js`. The menu bar's own shortcuts for an applet's contributed menus still need focus inside the applet.
- The Workspace pane's Sources tree sets PUDL's tree properties (0.96.8) so each row's toggle or glyph starts on the pane's indent line: `--tree-row-pad-start: 0px` and `--tree-indent: var(--space-4)` (`app.css`).
- The Sources tree marks itself `data-tree-toggles="buttons"` (0.96.9), so each folder's toggle is a button named Expand or Collapse and the folder's name, from `data-tree-label` on its link, which PUDL keeps as it toggles (`workspace-browser.js`).
- A group's Team Report popover sets `data-menu-anchor` to its own button (0.96.9); when the folding toolbar has folded that button away, PUDL places the report against the fold or More button (`bus-applets.js`).
- The menu bar's host lists are not redrawn while one of its menus is open: `host-menus.js` asks `pudlMenubar.isOpen()` and waits for `pudl:menubar-close` (0.96.9).
- Each grouped window sets `data-win-group-detail` to its workspace's name (0.97.0), so a stack row whose detail only repeats the stack's workspace draws none (`workbench.js`).
- The Workspace pane is PUDL's reorderable list (`pudl-reorder.js`, 0.97.0) for Reorder Sections: the frozen top and the applets' place are `data-reorder-fixed`, each movable section has `data-reorder-id`, `data-reorder-label` and a `data-reorder-grip` slot where its toggle stands, Done is `data-reorder-done`, and the pane hides every section's body under `data-reorder-active` (`workspace-sections.js`, `app.css`).
- The group picker's list is PUDL's rich list box (`pudl-listbox.js`, 0.97.0): PUDL gives it its roles, keys, type-ahead and check marks and does the choosing, and Dinah writes a `.listbox-section` for each workspace and keeps each row by its chat, filling it in place, so the row PUDL has active stays active. A newly created chat's row takes the keyboard through `pudlListbox.activate` (0.97.1). A chat with no title stays choosable and, when chosen, opens its title field instead (`bus-applets.js`).
- The status bar keeps each widget as a separate button. At compact widths, Dinah hides its label and badge while preserving its glyph and accessible name. It does not enable the aggregate Status folding control.


## Unpublished pinned-section candidate

Both bundled distributions now use frozen candidate pinned-section-73c8338a72e3. This is not a published release. Its CSS version comment remains 0.103.0.

- Archive SHA256: e8d7bf70b15416338769b1aa12183ea98dd2baffb5d4929da2775c26662f00c0
- Archive size: 7899501 bytes
- Source fingerprint: 73c8338a72e3c00f82b6a7b47804ed0fa4808e2625d05ed09bca0d85424b703d
- Upstream base: f585b5a18d2175afd351be5e64a7bd562761e2d0
- Host baseline: 902df02

All 805 archived files matched the supplied manifest. All 193 distribution files matched after copying into each bundle. Host regression work is paused at the person's request pending further instructions. No installation or restart was performed.

## Published 0.104.0 adoption

Both bundles now contain the complete 193-file distribution from published v0.104.0, tag commit 1793669234fd1c4ef50bee22a41807b91ff6a11a. The downloaded archive matches its published SHA256 ac9cb885e81b59528d007d9e097a95ac838e2875eacc8ebad65252e019c08a4d. Only pudl.css differs from the accepted candidate distribution. Every copied file was hash-verified.


## Published 0.105.0 adoption, 9 November 2026

Both bundles now contain all 193 distribution files from the latest published release, v0.105.0. GitHub's latest-release response and tag reference were checked directly. The tag commit is e329c43c51c4a19df35181f5a9421219fd48c864. The downloaded 7,186,020-byte archive matches the published checksum, 5c0e4f7ed32a3ca41e13e4c393ef74b7029b214d0aca4c7eaef57045fc98081d. Each copied file was hash-verified in both bundles, with no extra runtime files. VERSION is host metadata set to 0.105.0.

Six isolated headless Edge suites passed: taskbar-pins, taskbar-labels, title-launcher, launcher-commands, pinned-section-candidate and pinned-section-stacks. Evidence is in build/pudl-01050-release/browser-final. An initial invocation named a nonexistent terminal-shell suite and ran no tests. The subsequent invocation failed because playwright-core was absent; npm ci restored the existing locked dependencies and all six suites passed. The original dependency failures remain in build/pudl-01050-release/browser. No host runtime edits or vendor patches were required.

This adoption is a working-tree update. It does not install or restart the desktop, merge or commit the existing work. The installed executable remains the previously verified background-replay build. Native Restart validation remains outstanding.


## Published 0.106.2 adoption, 11 November 2026

Both bundles contain all 196 distribution files of the published release v0.106.2, tag commit a6e0d432cdd57f5798c4e58384b94f4f26d8722f. The downloaded archive is 7,213,880 bytes and matches its published SHA-256, 7e315f5775f2486808b94cb0de9142a8dec168f602560d68269137aace8afc10. The distribution manifest's SHA-256 is 60b3fff55740c41cf4e0e895582874fa1386ba0ee78fb918e2e62ffe9afa1b93, and every file in both bundles was compared with it. VERSION is host metadata set to 0.106.2. Releases 0.106.0 and 0.106.1 were never taken: 0.106.0 was held after two WebKit failures, which led to two menu corrections, and 0.106.2 has everything they had.

The release brings three things Dinah uses.

- A taskbar button that stands for several windows draws one edge behind it for two windows and two edges for three or more (`data-win-stack-depth`, `.win-stack-edges`). Dinah's markup did not change.
- Alloy Bold is a new skin. `index.html` and `show.html` each link `pudl-skin-alloy-bold.css` beside the other skins, and the View menu lists it from `pudl-skins.json`.
- The default mark between a window's title and its detail is encoded correctly. Dinah's two stand-ins for it are removed: the `data-win-text-title-sep` attribute on the window layer and the `:root` rule for `--win-title-sep` in `app.css`.

Standard's boundaries are firmer in this release, and a desktop title mnemonic pressed just after a window takes focus opens that window's menu.

Twenty-four headless suites passed on it: skins, skins-startup, skin-check-sync, skin-contrast-cache, skin-paints, skin-paints-2, web-skins, taskbar-stacks, taskbar-labels, taskbar-pins, taskbar-state-stability, stack-rows, stack-row-titles, pinned-section-stacks, window-stack-candidate, title-launcher, title-status-order, companion-themes, chat-menu-stale, documents-menu, menubar-fold, menudim, menus-through-updates and wsmenu. The real desktop, run hidden, showed stack depth 1 and 2 with their edges, Alloy Bold chosen, no taskbar button moved by a change of skin, and the middle dot on the taskbar. The full suite was not run.

The applets' release must bundle the same PUDL, because `yavc_release_test.go` compares every PUDL file in its manifest with Dinah's copy, so this adoption and yavc.app v0.2.66 were taken in one commit. The commit before it is the rollback.

## Published 0.107.1 adoption, 11 November 2026

Both bundles contain all 196 distribution files of the published release v0.107.1, tag commit e2c05a919dba47ead103b0940e51f3d8200fb01a. The archive `pudl-0.107.1.zip` has SHA-256 `8b9cac72f0430e9601f56b20e91915dec5a614cd501eafeedf4166ef982be9e6` and its distribution manifest `8469d48c529a7077b5a36477fb7d001cffeb218c5f0e8ead9edef5964a47f2b5`, and every file was checked against that manifest before it replaced the copy here. Four files differ from 0.106.2: the menu bar script, its minified copy and map, and the version comment in `pudl.css`. 0.107.0 was never published.

The release adds one thing Dinah uses. A host whose content is the application itself sets `data-menubar-composition="application"` on its menu bar, and PUDL then draws the host menus and the applet menus as one group under the host name, with the applet identity commands in the first menu. The stand-alone document page (`document.html`) sets it, so a stand-alone Editor has one menu bar: its name, then File, Edit, View, Insert, Format and Review. The main window does not set it and keeps its two groups.

PUDL published this release with one limitation Paul accepted: a single automated test of a taskbar popup timed out once on the tag and did not fail again in 160 repeats, and its cause is not known.

Seven headless suites passed on it: menubar-fold, menudim, menus-through-updates, wsmenu, documents-menu, chat-menu-stale and title-launcher. The stand-alone page, run against a headless document service in Chromium, drew one bar with those titles and no warning in its console, and edit, save, a refused save, recovery and the web root behaved as before. The real window was not seen, and Firefox, WebKit, the ribbon, the collapsed bar and keyboard use were not checked here. The full suite was not run.

This adoption and yavc.app v0.2.68, which is v0.2.67 with this PUDL bundled, were taken in one commit. The commit before it is the rollback.

## Published 0.108.0 adoption, 17 November 2026

Both bundles contain all 196 distribution files of the published release v0.108.0, tag commit 5e77a445fb774012be16f9fb75c6c989242cddf2. The archive `pudl-0.108.0.zip` has SHA-256 `982519da8242175b0ba029d62d21faf01ad7327d25c4ebd26d63d9fb39004d72` and its distribution manifest `10e42f32616106454284dd471ba13bc221468f23adc53652c54596b6f8298ce4`, and every file was checked against that manifest before it replaced the copy here. Twenty-one files differ from 0.107.1: `pudl.css` and the scripts for windows, commands, menus, the toolbar, the combo box and the date field, each with its minified copy and map. 0.107.2 was never taken.

The release is PUDL's answer to the scaling findings Dinah sent on 17 November (docs/review/pudl-scaling-brief-2026-11-17.md). A page with many windows open does less work for each small change: the window layer reads its size once until a resize observer says it changed, a change to one window's marks redraws that window's title state and tabs alone, and four stylesheet rules no longer ask an ancestor what it holds. Nothing in a project's markup changes. `pudlWindows.top()` and `pudlWindows.isOpen(key)` are new, for a page that called `state()` only for those; Dinah does not use them yet.

Every headless suite was run on it, 318 in all. Twenty-eight failed, and each of them failed in the same way on 0.107.1 or reads a window that was redrawn the same day; none failed because of this release. The real window was not seen, and Firefox and WebKit were not checked here.

This adoption and yavc.app v0.2.103, which is v0.2.102 with this PUDL bundled, were taken in one commit. The commit before it is the rollback.

## Published 0.108.1 adoption, 20 November 2026

Both bundles contain all 196 distribution files of the published release v0.108.1, release commit 3a77a4cba54bde35e34522af176be7645f01b962. The archive `pudl-0.108.1.zip` has SHA-256 `26df8249c560360a9a56c3cc249169f6268b4aa03a69fdbd2c084184762b9114` and its distribution manifest `5fa112285fff66f81b31f4b913be1c2f62aed2fc74f7fa3c956dd54cf8d0321f`, and every file was checked against that manifest before it replaced the copy here. Six files differ from 0.108.0: `pudl.css` and `pudl-menu.js`, each with its minified copy and map.

The release carries Paul's ruling of 16 November that a row's actions button is always shown and is an ordinary Tab stop, everywhere. PUDL no longer hides the button until hover or focus, and no longer takes it out of the Tab order or hides it from assistive technology. A button with no name of its own is labelled Actions. Dinah's row buttons are PUDL's, so they change with no code here. The Org Chart's actions buttons are ordinary buttons, and the rules that forced them visible were removed because nothing hides them.

This adoption and yavc.app v0.2.107, which is v0.2.106 with this PUDL bundled, were taken in one commit. The commit before it is the rollback.

## Published 0.108.2 adoption, 20 November 2026

Both bundles contain all 196 distribution files of the published release v0.108.2, release commit ee4426740ab543d040893555c74880a062c19ef4. The archive `pudl-0.108.2.zip` has SHA-256 `b3edc3c9926d5d8d8c1701a6cd2231fde5667a1a4a82578ac697bffd66683b5d`, and every file was checked against the release's distribution manifest before it replaced the copy here. Only `pudl-windows.css`, its minified copy and map, and the version line of `pudl.css` differ from 0.108.1.

The release carries Paul's ruling of 20 November that every taskbar button is 30px tall, the height of the bordered tray, so window buttons, stacks, the bar's own buttons and each tray action now share one height. The tray draws its border inside that height, enlarged text grows every entry together, and a coarse pointer keeps its wider targets at the same height. Dinah's taskbar entries are PUDL's, so they change with no code here. Measured in headless Chromium, every entry and the tray are 30px at 1280px wide, at 125% display scaling and at the two compact widths; a stack of three windows shows both edges inside the dock; the keyboard focus ring stays inside the bar; and with 20px text every entry is 34.5px.

This adoption and yavc.app v0.2.108, which is v0.2.107 with this PUDL bundled, were taken in one commit. The commit before it is the rollback.

## Published 0.109.0 adoption, 25 November 2026

Both bundles contain all 196 distribution files of the published release v0.109.0, whose tag resolves to commit f8f7f21ab4479fd5acc5edbe9f8dcc7c13bfb90c, the head of PUDL's main branch when it was taken. The archive `pudl-0.109.0.zip` has SHA-256 `1dbaf73287c543278e89e992aeb4861f37c9c600c3c8875e1f094783adc2a2fa`, which matches the release's own checksum file, and its distribution manifest `pudl-0.109.0-dist-manifest.json` has SHA-256 `6c61736b62b7ee24a9d63bbf8e0b4455b2f5e417acb3b259b2d1916470f42590`. Every file of the extracted `dist/` and of both bundles was checked against that manifest for its length and SHA-256, with no file missing or extra. Thirteen files differ from 0.108.2: `pudl.css` and the source, minified copy and map of `pudl-combo.js`, `pudl-commands.js`, `pudl-menu.js` and `pudl-menubar.js`.

The release brings PUDL's own edit menu (`docs/EDIT-MENUS.md` in PUDL). A right-click, Shift+F10 or the Menu key on a text field, an editable region or selected text now offers Cut, Copy, Paste and Select All, through `window.pudlClipboard`, whose transport each host sets. Dinah's transport, `clipboard-transport.js`, was written for it before the release and now takes effect. `docs/design/chat-clipboard.md` records which pages set it, which of Dinah's surfaces give their own edit rows, and the web view's own menu being turned off (Paul's ruling of 24 November). The menu bar no longer refuses a command's display-only shortcut that the browser reserves, such as Mod+X, C, V and A, so the warnings Docs caused about them in v0.2.116 are gone. A searchable select can now take its options from a lookup the page makes as the person types (`docs/REMOTE-OPTIONS.md` in PUDL); Dinah has no such select yet.

This adoption and yavc.app v0.2.117, which bundles this PUDL, were taken in one commit. The commit before it is the rollback.

## Published 0.110.0 adoption, 27 November 2026

Both bundles contain all 196 distribution files of the published release v0.110.0, whose tag object 0be9c893cdc84c85a45762c4b5d33233c7e48a3d resolves to commit be15ce340d4549625bdc091db9ead8fded5069ce. The archive `pudl-0.110.0.zip` has SHA-256 `0ea6a81e2688fb592dfda2114bfa038e9ef4f510a9cfe6adaf849147aff1f631`, which matches the release's own checksum file, and its distribution manifest `pudl-0.110.0-dist-manifest.json` has SHA-256 `778e140a292bfd7bf54deb73c397726f32ad408e35a7239c83b908302c43467b`. Every file of the extracted `dist/` and of both bundles was checked against that manifest for its length and SHA-256, with no file missing or extra. Fifteen files differ from 0.109.0: the source, minified copy and map of `pudl.css`, `pudl-windows.css`, `pudl-windows.js`, `pudl-menubar.js` and `pudl-launcher.js`. In `pudl.css` only the version line and one ribbon rule changed. No stylesheet was added, so the pages link nothing new.

The release draws a window's `data-win-count` as a neutral badge with no warning glyph, after any attention badge, with `data-win-count-label` as the suffix of the tab's accessible name and tooltip (`docs/WINDOW-STATUS.md` in PUDL, Neutral Counts). Dinah has set both attributes on the Chats window since 26 November (`docs/design/notifications.md`), so the Chats tab now shows how many chats are busy, and "3 busy" in its name and tooltip. The badge reserves no room while the count is absent and widens the tab when it appears, as Paul chose. The browser suite `chats-busy-count` checks the drawing and that nothing else in the strip, the Chats panel or the taskbar moves.

The release also gives a launcher-backed menu bar a product row in the ribbon (`docs/LAUNCHER.md` in PUDL, Launcher Ribbon Row). Dinah's main menu bar is launcher-backed, so when the person has turned the ribbon on, the Dinah title is a tab whose row holds one Launcher button, and the hamburger beside it opens the launcher directly; with the ribbon off, which is the default, nothing changes. PUDL gives no way to leave that row out. A host may add its own actions to it with `ul[data-menubar-launcher-actions]`; Dinah declares none, because Paul has not approved that row for Dinah.

PUDL's changelog says sites serving Firefox visitors should wait for two Firefox defects (PUDL issues 43 and 44, about the first launcher opening and a grouped taskbar's execution glyph). The desktop window is Microsoft Edge WebView2, which they do not affect. The bus page in `internal/web` is served on the loopback address and can be opened in any browser; it has a taskbar and no launcher.

This adoption and yavc.app v0.2.123, which bundles this PUDL and has no other change, were taken in one commit. The commit before it is the rollback.

## Published 0.110.1 adoption, 27 November 2026

Both bundles contain all 196 distribution files of the published release v0.110.1, whose tag object 8bc9fd3dc86523c4cef5dc3d7a5153bb2b38176b resolves to commit cd8d526fe847d64ae232a2b591c97fe666929d50 with tree 3b000580820c48c018fcd332648db70164b94a63. The archive `pudl-0.110.1.zip` is 8,363,245 bytes with SHA-256 `8841779a5c208f677c0238cd472c85bd0a97b21421454e1d94d4014963250791`, which matches the release's own checksum file (an LF file with SHA-256 `dec395f5d5f94d99dd7c888134831a83fdc86ee033c8c6aa1eb3425a88d62563`), and its distribution manifest `pudl-0.110.1-dist-manifest.json` has SHA-256 `51153c68800486f94bc63d7b56449fdd21a4e1d3b8b084e34a64b87f48a9d8ee`. Every file of the extracted `dist/` and of both bundles was checked against that manifest for its length and SHA-256, with no file missing or extra. Seven files differ from 0.110.0: the source, minified copy and map of `pudl-launcher.js` and `pudl-menubar.js`, and the version line of `pudl.css`, whose minified copy is unchanged. No stylesheet was added, so the pages link nothing new.

The release corrects two launcher faults that Dinah's browser suite `launcher-placement` recorded on 0.110.0 (`docs/review/launcher-placement-gate-2026-11-27.md`). With the ribbon on, `pudlMenubar.launcherButton({visible:true})`, which `title-launcher.js` gives the launcher as its anchor, returned the Dinah title in the top bar, so the panel opened over the ribbon's Launcher button and a second click on that button could not close it. It now returns the ribbon's Launcher button, and the panel opens below it. With the ribbon off, the first opening drew the panel at the top left corner for one frame and then moved it below the menu bar; PUDL now places the panel before it is shown, in both the native popover path and the launcher's own opening. Dinah's code did not change. In the suite at 1440 by 1000 pixels, in both themes, the ribbon's Launcher button spans y=57 to y=85 and the panel opens at y=91; with the ribbon off it opens at x=49, y=44, six pixels below the title, from its first visible frame. All 26 of the suite's checks pass, where six failed on 0.110.0.

PUDL issues 43 and 44, the two Firefox defects, are still open, and the changelog again says sites serving Firefox visitors should not take this release until both are closed. The desktop window is Microsoft Edge WebView2, which they do not affect, and the bus page has no launcher.

This adoption was committed before yavc.app had a release that bundles 0.110.1, so for one commit `yavc/` still bundled 0.110.0 and the Go test that compares the yavc.app release's PUDL with Dinah's copy failed on the seven changed files. yavc.app v0.2.124, which bundles this PUDL and has no other change, was taken in the next commit (`YAVC.md`). The commit before this adoption is the rollback for both.
