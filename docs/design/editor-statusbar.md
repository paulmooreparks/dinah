# Editor status bar

The Editor shows the document location in its bottom status bar alongside line, column, and operation messages. Its save state appears in the titlebar status area, following the Chat window pattern. The workspace remains in the window title. The repeated workspace and source description below the toolbars is removed.

Local documents obtain their full path from the existing `WorkspaceLocalPath` host API, which applies the host OS path rules and validates the source. The frontend displays and copies that returned string. It caches the result for the current origin and source root, and ignores responses for an older origin. Rename and Save As invalidate the location. Failed resolution leaves an explicit unavailable label and a disabled Copy Path button.

New documents show that they have no file location. Nonlocal documents retain a source and resource description without offering an OS path. Long paths shorten visually, with the full text available in the tooltip and through Copy Path. The status bar wraps at narrow widths.

The save chip uses a glyph, text, and semantic theme color. It distinguishes saved files, unsaved changes, new documents, work in progress, and changes detected elsewhere. Keeping a draft does not imply that a new document has been saved to a file. Existing error messages and conflict actions remain available.

Paul approved moving the save chip to the titlebar on 4 November 2026. It participates in PUDL window status fitting and its narrow-window status popup. Embedded editors keep the chip within their own status bar because they share a containing window. The chip retains Save failed and its error explanation until a successful save; the existing Save command retries the operation. Path and caret details remain in the bottom bar.

The visual conventions follow [Architectural Principles](<C:/Users/paul/OneDrive/Documents/Architectural Principles.md>).

## Verification

Verification uses headless Edge with a stubbed clipboard bridge. It must not take the user's mouse, keyboard, or desktop focus. The Editor suite checks canonical path display and copying, narrow layout, line and column display, save state, lookup caching, new documents, rename response ordering, and failed path resolution. Its existing checks cover Save As, automatic saving, conflicts, and restored editors.

On 4 November 2026, `editor`, `skills-browser`, and `app-skills` passed in 54 seconds with one suite running at a time. The wide status bar and narrow layout screenshots were inspected. Evidence is in `build/editor-statusbar-final` in this checkout. The changes have not been installed.

The subsequent far-right chip adjustment passed the focused Editor suite in 42 seconds. Screenshots with the copied-path message and caret position show the chip at the same right edge; the narrow screenshot confirms that the message wraps to its left. Screenshots are in `cmd/dinah-desktop/frontend/e2e/build/editor-chip-right`, with the empty failure receipt in `build/editor-chip-right`. Paul authorized its build and installation, completed at 18:54 Singapore time on 4 November 2026.

The titlebar implementation passed Editor and Skills Browser in 58 seconds. The Editor suite now verifies titlebar placement, automatic-save progress, retained failure explanation and successful retry, along with its existing conflict and document checks. Embedded editors retain their local save chip. The narrow-window screenshot exposed low contrast on the active header, so the titlebar chip now uses the theme surface behind its semantic state color. Final verification and a rebuilt executable follow this correction.

Final Editor verification passed in 51 seconds after the contrast correction. The narrow titlebar screenshot was inspected and the Saved chip is readable without a duplicate inner border. The production executable is build/editor-titlebar-20261104/dinah-desktop.exe, SHA-256 f74e012fabeb23a8f0644517981d33b0787bbf21b4b24b8013bcc36ccc48f2f5. It includes the measured list corrections and glyph-first tray. It has not been installed.

Paul corrected the titlebar styling on 4 November. PUDL WINDOW-STATUS.md specifies an inherited header foreground, transparent background and currentColor outline for host .win-status chips. The custom surface background and nested save-chip paint are removed from the titlebar. Embedded editor styling is retained. The Editor regression now checks inherited paint across every available skin in light and dark themes.

The final documented-chip implementation passed Editor in 46 seconds, including every available skin in light and dark. The narrow screenshot was inspected. Installed version 0.0.0-pudl-chip-20261104 has SHA-256 572322E5DD5E644B1C78B761149A5EB2AAC1A261742AEE032A75F70D6A7F181C. Rollback is C:/Users/paul/.dinah/bin/dinah-desktop.exe.old-chip-20261104-210416. No restart occurred.
