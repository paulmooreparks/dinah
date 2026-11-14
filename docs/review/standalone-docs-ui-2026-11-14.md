# Stand-alone Docs chrome verification

The candidate is c90e60a3 on frontend/standalone-docs-ui, based on master d6e00dea. The comparison and applicable controls are recorded in [the design note](../design/standalone-docs-chrome.md).

The stand-alone host now has Document, Save As and Help with Working with Documents, Keyboard Shortcuts and Key Bindings. The shortcut reference uses the current applet command table, including configured keys. F1 opens it. Save As and Key Bindings follow command availability and the mount's ready state, including after a blank Editor changes into Docs. The bundled applet, file service, installed application identity and native title policy are unchanged.

## Final checks

All browser checks ran headless through FIFO. The production-tagged window program built with desktop,production. Its SHA256 is f1d709c1b9c0550231cb77cf9608882ca62ea1d2ce42a7f203ca241684f61d31, at %TEMP%/standalone-docs-ui-candidate/host.exe.

The new scripts/test-standalone-docs-ui.cjs passed against that program, exit zero. It started an isolated document service and an actual hidden window process using DINAH_DOCUMENT_ENTRY_FILE. Headless Edge opened the process's page address. The checks exercised File, Save and Document, Save As through the rendered menus, checked the dirty dot and file name before and after saving, checked the editing status, and verified that Publish and Request a Change are absent. They opened the guide, the F1 reference and Docs Key Bindings, checked a 390-pixel guide, opened Editor Key Bindings for plain text, and replaced a blank Editor with a previously unopened Markdown document before opening Docs Key Bindings there. No page error was recorded.

The main window's existing documents-menu suite passed unchanged in fourteen seconds, exit zero. The focused Go frontend policy, release-asset and UTF-8 checks passed in 0.527 seconds, exit zero. ESLint for document-host.js, Node syntax checks and git diff whitespace checks passed. No full suite was run.

The final screenshots were inspected. The evidence is retained in .diagnostics/standalone-docs-ui in this worktree, with docs-chrome.png, docs-help-narrow.png and chrome-check.json. The main-window run's failures.txt is empty in %TEMP%/standalone-docs-ui-main-reference-20261114.

## Corrections and limits

The initial build command passed an unquoted comma in the tags to PowerShell and failed. The quoted tags built successfully. The first hidden run passed the Docs cases but found Key Bindings disabled in a freshly opened Editor. The host had watched command changes without watching the ready-state attributes. The added attribute observer fixed that case without a delay. The next run passed that Editor check but used the already-open Markdown file for blank-window replacement. The service correctly reused its existing window. The fixture now chooses a different file, and the final run passed. No product assertion was removed.

The first focused Go check failed with an embedded-file length error because I edited the frontend while that queued command was compiling it. The final build, hidden check and Go check ran against the frozen production source. Their results above are the final results.

The browser title, rendered menu controls and status were checked. The native titlebar and operating-system file dialogs were not inspected. Dialog choices came from the isolated service's documented hidden controls. Publishing to an external source was neither offered nor tested. No installed-product journey, real model, visible window, desktop input, real-home write, credential-file read, push, merge or installation occurred.

Automatic approval review rejected deletion of these four owned temporary fixture folders with the reason blocked by policy: standalone-markdown-18gw1I, standalone-markdown-itIHx8, standalone-docs-ui-F4nF4K and standalone-docs-ui-oLwCxy. They remain under %TEMP%. The harness closed its browser, stopped the service and waited for or terminated its owned hidden processes; no cleanup of those folders is claimed.

The candidate is ready for the lead's review and integration. Front End retains ownership for findings.