# Taskbar applet identity

This change implements the current requirement in [taskbar-stacks.md](../plans/taskbar-stacks.md). Architecture approved the applet boundaries in bus message 2150. The root plan remains Architecture's working copy.

Each workbench registration declares `applet` metadata containing a stable `id`, a display `title`, a `glyph` and a boolean `workspace` ownership policy. A registration may supply a resolver when its metadata comes from a declared extension view. Internal window kinds do not become applet identities automatically.

Chat includes New Chat, saved chat sessions and chat auxiliary views. Documents includes the Editor, the legacy document window, resource viewers and diffs. Group Chats includes its browser, rooms, picker and threads. Terminal is workspace-free, including prepared-command windows. Other standalone applets declare their own identities. A singleton still carries metadata even though PUDL presents one represented window as an ordinary button.

The group key encodes the applet ID and the stable workspace ID as a JSON tuple using UTF-8 and unpadded base64url. Workspace-free groups encode only the applet ID. Labels use the applet title and current workspace label. Renaming either label cannot change membership. Unknown applet metadata and unresolved workspace ownership leave a window ungrouped; neither case becomes a workspace-free group. Workspace records, extension catalog resolution and extension workspace changes trigger regrouping.

The extension SDK's current contract declares one identity per manifest and several views within that identity. The host uses the declared manifest ID rather than a window kind, package filename or view title. It resolves ownership from the active frame, or from the saved pin and declared following/pane policy before the frame starts. An unavailable manifest stays unresolved. Architecture confirmed this SDK boundary in bus message 2158.

PUDL 0.95.2 already contains the documented `full` taskbar mode. Dinah supplies membership metadata and enables that mode. PUDL decides when separate buttons overflow, preserves canonical order, and owns singleton transitions, dock filtering, aggregation and group-command scope. This change does not modify the vendored implementation.

The browser regression checks ownership separately with projection held in `always` mode, then exercises the production `full` mode at widths where separate buttons fit and overflow. Further checks cover restored windows, workspace rename and continuation, equal workspace names, unrelated applet types, explicit workspace-free ownership and delayed ownership resolution. Extension pin and following behavior uses the existing extension regression.

Focused taskbar ownership and state tests passed. The extension regression passed after two added assertions were moved to the appropriate fixture state. The restored-window regression also passed with workspace facts held unresolved until after the window mounted.

The first full gate ran 156 suites in 374 seconds with eight workers. It failed `chat-scrollback` and `next4` while clicking chat-body controls outside the viewport or covered by a chat title bar. Both suites subsequently passed in isolation on this candidate and on released `c1b007e` source. Those isolated passes do not establish the concurrent failures' cause. Their code remains unchanged, and the failure details remain in `.diagnostics/taskbar-applet-full/failures.txt`.

The second full gate passed all 157 suites in 267 seconds with eight workers, including the new delayed-ownership regression. Its `failures.txt` is empty under `.diagnostics/taskbar-applet-full-rerun`. Runtime and test sources were unchanged during that run. JavaScript syntax and whitespace checks passed. UI and release approval are pending.
