# Release notes rendering check

The Updates page now renders Markdown through `dinahReleaseNotes.set`. The update dialog can use the same helper. It reuses the existing Markdown parser, sanitizer and browser action. Relative and file links appear as text. Code examples scroll horizontally, and tables wrap or scroll inside the notes panel. Unchanged source text preserves the rendered nodes, focus and scroll positions.

The new `release-notes` browser suite passed in light and dark at 1440 and 390 pixels wide. It checks headings, emphasis, lists, tables, long code, unsafe markup, relative links, file links and web links. It also checks keyboard focus, unchanged-source preservation and stable surrounding geometry after a change. It confirms that reading notes starts no download or restart.

The existing `settings-updates-available`, `settings-updates-errors` and `settings-updates-service` suites passed without changes. The four-suite run took 17 seconds. After adding the existing browser action for web links, the new suite passed again in 3 seconds. Focused desktop tests for frontend ownership, release asset exclusions and UTF-8 passed in 0.542 seconds. All checks ran through the shared FIFO.

Screenshots are in `%TEMP%/release-notes-final` and `%TEMP%/release-notes-links`. The dialog adapter has a separate private integration check and four screenshots in `%TEMP%/release-notes-dialog`. The narrow dark screenshots were inspected. These are headless page checks with the real frontend and a stubbed update service. A native update or installation was not performed.

The first new suite run used a nonexistent maximize helper and failed before its assertions. It was corrected to use the existing window snap API. A subsequent assertion incorrectly required an already fitting, wrapped table to scroll. The final assertion requires a table either to fit or to scroll, while code must actually overflow and reach its far end. The code clipping failure from the release audit remains covered.

No frozen archive, real home, credential file or remote service was changed. No push, merge or installation occurred. The change is ready for review for the next build.
