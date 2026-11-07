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
