# dinah-desktop

`dinah-desktop` is Dinah's window. It is a workbench of PUDL applets that open as independent windows, which can float, snap to the sides or share tabs, and Dinah remembers the layout. The top-level [README](../../README.md) explains how to install and start it, and [DESIGN.md](../../DESIGN.md) explains how it fits with the message bus and the `dinah` command line.

## What Is in the Window

- **Workspace.** A workspace is a main folder and any number of further local folders attached to it as sources. The sidebar shows the workspace's chats, flows and documents. Workspaces have stable identities and names that can be changed without moving files, and a workspace with no main folder runs its chats and flows in private storage.
- **Chats.** A chat talks to Claude, Codex, Devin or Ollama through the Agent Client Protocol. Each chat chooses its provider, mode and model, and an idle chat can change provider, continuing in the same window with the earlier conversation shown for review first. Permission requests appear beside the chat's prompt and in Notifications. During active work, ordinary sends steer when the provider and model support it and queue otherwise. The Queue and Steer buttons choose explicitly. Steer preserves the current turn and delivers guidance through the provider adapter or hooks; if that path is unavailable, the message waits for the next turn. Stop and Steer explicitly interrupts, and Stop ends the turn.
- **Files and Terminal.** Both browse `/workspaces` and the home folder at `~/`. The Terminal runs its own commands (`ls`, `cd`, `cat`, `open` and the rest, with path completion and applet commands) and never launches an operating-system shell. Writes through it are allowed only in local folders.
- **Editor.** A document opens in one Editor window, as a draft that is saved back to its file. The Editor has four views. Text is the Markdown itself; Preview draws it; Split shows the text beside the page; Visual edits the page in place. Agents can suggest changes to a draft, and the Editor shows each suggestion with Accept, Reject and Edit, both in the text and on the page.
- **Skills and Flows.** View > Skills lists the personal and workspace skills, runs a skill with its parameters, and puts a skill's prepared instructions into a chat for review. A flow has a chart, a list of step forms, its text and its runs.
- **Settings.** Settings has a tab each for the appearance, the agents and the chats, among others. The Agents tab finds each provider's executable, or uses one you choose, and sets the default agent and the approval rules. Credentials, executables and approval policies are changed only in this interface, never by an agent.

## The Preview and the Visual View

The preview draws Markdown as GitHub Flavored Markdown with `internal/preview`, which uses goldmark: headings, paragraphs, emphasis, links, lists, task lists, tables, block quotes, code and images. A fenced block in `mermaid` or `d2` is drawn as a diagram in the window. A local image is loaded from beside the document, or from the draft when it was added to the draft. The preview marks each top-level block with the line it starts on, so the preview and the text scroll together, and it reports a table row with more cells than its header row as a problem.

The Visual view is one ProseMirror editor over the whole document (`internal/pageview` and `frontend/page-view.js`). Paragraphs, headings, lists and tables are edited as rich text. Any other block, such as code, a diagram, an image or a quotation, is drawn as the preview draws it, and a click edits its Markdown in place. A block the person did not change is written back exactly as it was read, so an edit changes only its own block. The / menu on an empty line, and the Insert menu, put in a table or a code block.

## Where State Lives

Personal settings, saved window layouts and other state are kept in Dinah's own folder, `~/.dinah`, on Windows, macOS and Linux. The log, and a trace of each agent's protocol traffic, are in the `logs` folder there.

## Development

The window is built from `cmd/dinah-desktop`; the packages it shares with the rest of the repository, such as `internal/preview` and `internal/pageview`, are under `internal/`, and the Agent Client Protocol client is in `acp/` at the top of the repository.

- `app.go` holds the window's state and answers the agents' requests (permissions, file reads and writes).
- `frontend/` is the page, drawn with PUDL, which is copied into `frontend/pudl` as a tagged release.
- `frontend/e2e/` holds the browser suites, which run the page in a headless browser against a stubbed window. `pwsh ./run.ps1` in that folder runs them all; its README describes each suite.

The window is built with the tags `desktop,production`. On Windows it uses WebView2 and needs no C compiler. On Linux it links GTK 3 and WebKitGTK 4.1 through C and is built in Docker by `tools/linux-build`; on macOS it links Apple's WebKit and is built on a Mac by `tools/macos-build`. Each folder's README explains its build.

`go test ./...` at the top of the repository runs the Go tests. Tests that talk to a real agent are behind the `live` build tag, such as `go test -tags live -run TestOptions ./cmd/dinah-desktop`.
