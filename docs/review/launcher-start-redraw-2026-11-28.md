# Launcher startup command redraw

The Launcher shortcut could remain disabled after startup. Its registry predicate became true after enhancement, but the menu source retained the disabled value drawn earlier. Later unrelated command notifications could hide the defect.

Launcher initialization now calls `dinahCommands.changed()` after successful enhancement and restoration. The existing command redraw updates the menu source and keyboard dispatch. The command, key, labels and placement are unchanged.

## Regression evidence

The startup case in `cmd/dinah-desktop/frontend/e2e/launcher-shortcut.js` loads the actual host page with launcher initialization held until the other startup work has completed. It then loads the unchanged launcher source, draws its disabled command, and releases initialization through the UI-state event with its required detail fields. After release, the test supplies no command flush, menu opening or incidental redraw. It waits for the command notification, checks the menu source is enabled, and presses the physical shortcut. The closing assertion runs only after opening succeeded.

On base `1270775a`, both menu and ribbon cases failed the notification, enabled-row and opening checks. The existing shortcut cases passed. The first fixture attempt omitted the event detail and also produced a page error; that fixture was corrected before recording the clean failing run. The corrected red log is retained privately under `%TEMP%/launcher-start-red-valid`.

After the fix, the hidden headless run passed all four requested suites in 35 seconds:

| Suite | Passing checks |
|---|---:|
| keyboard | 31 |
| host-menus-order | 51 |
| launcher-shortcut | 120 |
| title-launcher | 15 |

The green logs are retained privately under `%TEMP%/launcher-start-green`. Existing checks cover Windows Control and Mac Command branches, ribbon and menu modes, editor, prompt and terminal focus, focus return and unchanged terminal input. This branch does not claim installed-window verification.

## Related startup commands

Source review found no other registered command gated by a local startup-ready flag. Native fullscreen availability refresh already flushes command state; update-state rendering and workspace source changes already call `changed()`. Workspace section setup registers its commands after setup and notifies on subsequent state changes. Document commands notify on content changes, and front-chat commands subscribe to command-state changes. This is a source audit, not an exhaustive runtime test of every command.
