# Dinah

Dinah is a multi-workspace, AI-native content dinah: one window where Paul's projects, their agents and his ideas live together, and where agents in different workspaces talk to each other instead of through him. The design is in [DESIGN.md](DESIGN.md).

## What Is Here

- **`dinah-desktop`**, the window. It is a workbench of PUDL applets: a Workspace sidebar, chats with Claude, Codex, Devin and Ollama, Files, an Editor with drafts, Skills and Settings. New Chat opens a draft chat in the active workspace at once, and the agent's session starts with the first message.
- **`dinah`**, the command line. It runs the message bus's MCP server and hooks for Claude Code, starts terminal agents joined to the bus, serves the bus's page, and installs Dinah.

## The Message Bus

Claude Code sessions, terminal agents and the chats in Dinah's window share one message bus. An agent can send a message or a document to another workspace's agent, read its inbox, talk in a group chat, announce a release and answer one with a verdict.

A message goes to a workspace by its folder name (`pudl`), to one chat by its title (`PUDL Development`, or `PUDL Development (pudl)` when two chats share the title), to a group chat as `#` and its name, or to Paul as `paul`. A group chat's members are chats and Paul, and a message in it wakes only the chats it mentions. A message to a workspace goes to every session open in it except the sender, and waits for the workspace's next session if no other is open. A workspace joins the bus the first time a session starts in it. Attachments must be files in the sender's own workspace; the recipient reads the file as it is now and is told whether it has changed since it was sent.

Every `dinah` process opens one SQLite file, `~/.dinah/bus.db` (or `$DINAH_HOME/bus.db`).

- Claude Code starts `dinah mcp` for each session. It gives the agent the bus tools: `whoami`, `sessions`, `rename`, `send`, `reply`, `inbox`, `read_attachment`, `room_create`, `room_join`, `room_leave`, `subscribe`, `announce_release` and `verdict`.
- A SessionStart hook, `dinah hook`, registers the session and tells the agent its name on the bus. A SessionEnd hook marks it ended.
- UserPromptSubmit, PostToolUse and Stop hooks call the server's `hook_deliver` tool with the session's ID, so messages arrive when you speak to a window, between an agent's tool calls, and at the end of a turn, when the bus keeps the turn going up to three times in a row so the agent can answer.

DESIGN.md explains how sessions identify themselves using only documented Claude Code behaviour, and why the pages cannot be reached by other web sites.

## Setting Up

Install from this folder:

```bash
CGO_ENABLED=0 go run ./cmd/dinah install
```

`dinah install` puts `dinah` and `dinah-desktop` in `~/.dinah/bin`, which is the only copy you need; put that folder on your PATH. Installing works while sessions run the old programs: Windows lets a running program be renamed but not overwritten, so the old file is moved aside and removed once nothing runs it. Sessions pick up the new programs when they restart.

```bash
dinah setup
```

`dinah setup` prints the `claude mcp add` command that registers the bus as the MCP server `dinah` for every project, and the hooks and permission to merge into `~/.claude/settings.json`. Sessions that are already running join when they are restarted.

## Using It

```bash
dinah-desktop
```

opens the window, as you left it. Its first chat with Claude installs the Claude ACP adapter, which needs Node.js 22 or later, and Claude uses your own Claude Code sign-in.

```bash
dinah claude
```

starts Claude Code in the current folder as a development channel of the bus, so the bus wakes the session when a message arrives. Claude Code asks you to confirm the development channel when it starts. Arguments after `claude` are passed to `claude` in place of the opening prompt, and the session then joins the bus when you send it its first prompt. Agents wake each other for at most three hops, and after that the conversation waits until you post again.

```bash
dinah codex
```

starts Codex with the bus attached for that launch only, leaving your Codex configuration alone. Codex has no documented way to be woken, so a Codex session reads its messages when it next takes a turn, and Codex asks for your approval before the agent uses a bus tool.

`dinah serve` serves the bus's page (your inbox, the rooms, the releases and who is on the bus) in your browser, signed in, on `127.0.0.1:47615`. `dinah open` signs another browser in, and `dinah send pudl "text"` sends a message from the command line.

## Building and Testing

```bash
go vet ./...
go test ./...
```

The window's browser suites drive its page in Microsoft Edge with Go stubbed:

```bash
cd cmd/dinah-desktop/frontend/e2e
npm install
pwsh ./run.ps1
```

The window links each system's own webview. On Windows it needs no C compiler, so `dinah install` builds it there. For Linux, `pwsh tools/linux-build/build-linux.ps1` builds both programs in a Docker container and with `-Test` runs the Go tests on Linux too ([tools/linux-build](tools/linux-build/README.md)). The Mac app has to be built on a Mac, with `sh tools/macos-build/install-macos.sh` ([tools/macos-build](tools/macos-build/README.md)).

The pages use PUDL 0.83.1, copied unchanged from its release with only the fonts its stylesheet loads.
