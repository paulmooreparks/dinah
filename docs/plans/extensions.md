# Extensions, and Git as the First

Dinah will host applets it does not ship. An extension is installed from a folder or a zip, runs its back end as its own process and its front end in a sandboxed frame, and reaches Dinah only through a published contract. The Git applet is the first extension. It is built in its own repository against that contract alone, so that it proves the contract is enough. Paul decided this on 23 October 2026, together with the four choices below:

1. The Git applet lives in its own repository and compiles without Dinah's source.
2. Its back end is written in Go.
3. Every extension's front end runs in a sandboxed frame.
4. This plan is written before anything is built, for Paul to mark up.

Today every applet is a Go bundle compiled into Dinah (`internal/applet/builtin`), and its window is drawn by Dinah's own page code. The built-in applets move onto the same contract later, which is how core Dinah sheds what it has baked in.

## The Extension Host

### What an extension is

An extension is a folder holding a manifest, `dinah-extension.json`, and the files it names. A zip of that folder installs the same way. The manifest gives:

- its ID, in reverse-domain form (`com.parkscomputing.dinah-git`), its version, its title and its publisher;
- the contract version it was built against;
- its back end: an executable for each operating system, relative to the folder;
- its front ends: the HTML pages it draws, each the root of one pane or window;
- its contributions, described below;
- the permissions it asks for, described below.

Dinah keeps installed extensions in `~/.dinah/extensions/<id>/<version>`. Installing never runs the extension's code, and Dinah shows the publisher and the permissions before the person agrees. Removing an extension deletes its folder and the data Dinah keeps for it.

### The back end

The back end is an MCP server over standard input and output, started by Dinah when the extension is first needed and stopped when Dinah closes. It joins the app's job object, as agents and terminals do, so it never outlives the window. A crash ends that extension only. Dinah restarts it once, and says so if it fails again.

Its tools are its actions. Each tool declares, in its MCP annotations under `dinah`, the permission it needs and whether it has an effect, which is what `applet.Effectful` decides for a built-in applet today. Dinah offers the actions to agents through the applet tools chats already use (`applet_discover`, `applet_invoke`), and an action with an effect asks the person first, through the same consent the built-in applets use.

The back end receives Dinah's requests as MCP tool calls and calls Dinah back through a small set of host tools Dinah serves to it. Those host tools are the contract.

| Host tool | What it does | Permission |
|---|---|---|
| `host.workspaces` | The workspaces, their folders and sources | `workspace.read` |
| `host.run` | Runs a program the manifest allows, in a folder of a workspace, and returns its output | `process.run` with the program's name |
| `host.read`, `host.write` | Read and write files in a workspace's folders | `workspace.read`, `workspace.write` |
| `host.notify` | A notification or a question for the person | none |
| `host.terminal` | Opens a Dinah Terminal in a folder with a command typed and not yet run | `terminal.open` |
| `host.decorate` | Sets or clears decorations on files and lines (see Contributions) | `workspace.decorate` |
| `host.watch` | Tells the back end when files under a folder change | `workspace.read` |

`host.run` is how the Git applet runs `git`. Dinah runs the program itself, inside the workspace's folders only, through the same process runner as the Terminal, so the person's permission is enforced by Dinah rather than promised by the extension.

### The front end

Each pane or window an extension contributes is a page from its folder, shown in a sandboxed frame with an origin of its own. The frame cannot read Dinah's page, another extension's frame, the person's cookies or the bridge to Go. It talks to Dinah through `postMessage`, wrapped in a small script Dinah serves, `dinah-extension.js`:

- `dinah.call(action, input)` calls one of the extension's own back-end actions;
- `dinah.on(event, fn)` hears what Dinah tells the extension, such as the workspace in front changing;
- `dinah.commands(list)` gives the pane's commands, which Dinah draws in its menu bar and ribbon as it does for its own windows;
- `dinah.theme` gives the current theme, so the frame can follow it.

A frame loads PUDL from Dinah, at the version Dinah ships, so an extension looks like the rest of the window. Its Content Security Policy allows its own scripts and PUDL's, and nothing from the network unless the manifest asks for `network`.

### Contributions

The manifest declares what an extension adds, and Dinah draws each one; an extension never reaches into Dinah's page.

- **Panes and windows.** A pane docks like Workspace and In Progress and follows the workspace in front; a window opens like a document. Each has a title, a glyph and the page that draws it.
- **Kinds of file.** A window may list the media types it opens, each with its file extensions, so that a link to such a file in a chat opens in that window. Paul approved this on 28 October 2026; [document-links.md](document-links.md) and the design document describe it.
- **Commands.** Commands with a title, a glyph, a key and the menu they belong in, run by the back end.
- **Decorations.** Through `host.decorate`, a mark on a file in the Workspace tree and in Files (a glyph, a colour from PUDL's status set and a tooltip), and a mark on lines in the Editor's gutter (added, changed or deleted, with what the line was).
- **Workspace groups.** A group in the Workspace pane, such as Changes, whose rows the back end supplies and whose rows open the extension's windows.
- **Actions.** Everything above that changes something is an action, so agents can do what the person can, with the person's consent.

### Permissions

| Permission | Means |
|---|---|
| `workspace.read` | Read the workspaces' folders |
| `workspace.write` | Change files in them, with consent for each action that does |
| `workspace.decorate` | Mark files and lines |
| `process.run:<program>` | Run that program, by name, in a workspace's folders |
| `terminal.open` | Open a Terminal with a command typed |
| `network` | Reach the network from the front end |

The person sees the permissions when they install, and again when an update asks for more. Dinah refuses anything the manifest did not ask for.

### Installing from the command line

`dinah install` already installs Dinah itself, so extensions get a command of their own:

- `dinah ext install <folder | zip | git URL>` installs an extension, after showing its publisher, its key and its permissions;
- `dinah ext update [id]` updates one extension, or all of them, from where each came;
- `dinah ext list` lists what is installed, with versions and where each came from;
- `dinah ext remove <id>` removes one.

The window offers the same from an Extensions pane. A Dinah Shop, a catalog to browse and install from, comes later, once there is somewhere to host it; `dinah ext install` then also takes a name from the Shop.

### Updates and trust

An extension is identified by its ID and its publisher's key, and Dinah records the SHA-256 of what it installed and refuses to run a folder that has changed since.

Each publisher has an Ed25519 key, and every release carries the publisher's signature over the extension's files. Nobody signs the signers at first. Dinah trusts a key on first use, as SSH trusts a host: the first install of an ID shows the key's fingerprint and where the extension came from, the person agrees, and Dinah pins that key to that ID. Every update must be signed by the same key, so an update from anyone else is refused, and a change of key needs the person's say-so again.

That answers who may update an extension, but not who the publisher is. Two answers can come later, and they can stand together. The Shop can countersign a release once it has checked who published it, so a Shop extension carries the Shop's word as well as the publisher's. A publisher can also sign with Sigstore, which ties a release to an identity the person already knows, such as a GitHub account, and records the signature in a public transparency log, so the identity provider and the log vouch for the signer instead of a key nobody knows.

## The Git Applet

The Git applet lives in its own repository, `dinah-git`, as a Go module that depends only on the published contract module, `dinah-extension-go`, which holds the manifest types and the host tools' client. Its back end is one Go program; its front end is PUDL pages.

### The rule: real commands, shown

The applet does everything by running `git` through `host.run`, and never through a Git library. What the person sees is therefore what Git does, and it rests on Git's documented, machine-readable output: `git status --porcelain=v2 -z`, `git log -z` with a format, `git diff` and so on.

Every command it runs goes into a **Command Log**: the command line as the person would type it, its output, and a one-line explanation that opens into what each option means. Each entry has **Run in Terminal**, which opens a Dinah Terminal in the repository's folder with the command typed and not yet run. A **Show Me First** setting puts each command in front of the person before it runs, to run it, change it or type it themselves.

### What it does

The first release:

- a Git pane, docked and following the workspace in front, with a section for each repository among the workspace's sources;
- changes, unstaged and staged, by file, with staging and unstaging of whole files and of hunks;
- a diff window for a file, side by side or inline;
- commit, with the message and amend;
- the log of the current branch, with a graph, and a window for one commit;
- branches: switch, create and delete;
- fetch, pull and push, with the remote's state;
- the Command Log and Run in Terminal.

The second release brings Git into the workspace: status marks on files in the Workspace tree and Files, a Changes group in the Workspace pane, and the Editor's gutter marks for changed lines. After that come stash, merge and rebase with conflicts, tags, blame and worktrees.

### For agents

The applet's actions (status, diff, log, branch, stage, commit, push) reach every chat through the applet tools. An agent's commit goes through the same Command Log, so the person sees what it ran, and anything with an effect asks the person first.

### What it learns from

| Project | Licence | What to take |
|---|---|---|
| lazygit | MIT, Go | The command log; how it runs Git and reads its output; staging by hunk |
| GitHub Desktop | MIT | Flows for commit, branches, conflicts and undo |
| VS Code's Git extension | MIT | Source control as an extension: file marks, the gutter diff |
| gitui | MIT | Status and diff at speed |
| Learn Git Branching | MIT | Pictures of branches and rebases for teaching |
| Magit, tig | GPL | Ideas only; their code cannot come into Dinah or the applet |

## Order of Work

1. The contract: the manifest, the host tools and `dinah-extension.js`, published as `dinah-extension-go` with a written reference.
2. The host in Dinah: installing and removing, starting back ends, sandboxed frames, panes and commands, permissions and consent.
3. The Git applet's first release, in `dinah-git`, installed into Dinah like any extension.
4. Decorations and workspace groups in the host, then the Git applet's second release.
5. The built-in applets move onto the contract, one at a time.

## Decided

1. `dinah-extension-go` and `dinah-git` are public repositories under `paulmooreparks` from the start (Paul, 23 October 2026).
2. Installing from a folder, a zip or a Git URL is enough for now, through `dinah ext`; a Dinah Shop comes later, once Paul has chosen where to host it (Paul, 23 October 2026).
3. Releases are signed with the publisher's key, which Dinah trusts on first use and pins to the extension's ID; the Shop's countersignature and Sigstore can vouch for publishers later (recommended, 23 October 2026, after Paul preferred signing).

## Open Questions

1. Where does the Dinah Shop live, and who runs it?

## The Git Applet's Overhaul, 25 October 2026

Paul reviewed the Git extension's first release and found its interface jangly and short of what a graphical Git tool offers. The review, the proposed shape (a pane and a window that work together, one tree in the pane, actions on the rows, a diff with line staging, a commit graph) and working PUDL mockups are in the `dinah-git` repository, in `docs/design/overhaul.md` and `mockups/index.html`. The shape needs three additions to the extension contract, which Dinah makes before the applet is rebuilt:

1. Shared state within an extension: `dinah.shared.set(key, value)` and a `shared` event delivered to the extension's other frames in the same workspace, so the pane's selection can be drawn by the window.
2. `open-view` reuses an open window of that view in the workspace, giving it the new state, instead of opening another.
3. A view declared `follows: true` takes the selected workspace as a pane does, and the person can pin it to one; today only a pane follows, and a window pins at opening.
4. A diff view in the Editor, built on CodeMirror's merge view and themed from PUDL, and a host command, `open-diff`, by which an extension opens one with a title, the two sides, and the hunk and line actions the Editor shows and routes back. Paul decided on 25 October 2026 that diffs open in the Editor so that one diff viewer serves Git, reviews and agents' suggestions. The Editor side is done: `window.dinahDiff.open(spec)` in `diff-applet.js` opens the window, draws the texts inline or side by side, and routes the file, hunk and line actions back to the opener with the hunk's index and its line ranges on both sides. The host command `open-diff`, which passes an extension's request to that call and sends its presses back as `command` events, is the remaining piece.
5. Effect levels on manifest actions (reads, changes local state reversibly, rewrites history, reaches a network), so that consent for agents and Show Me First ask only for the last two. Paul found a permission prompt for staging a file too much.

A recursive `host.watch` and the reserved `host.decorate` surface help but do not block the first step.

All five landed on 25 October 2026 (Dinah `d7a5742` and `c614ff5`, SDK `b06af1c`), additively within contract 1: the new manifest fields are `follows` on a view and `effect` on an action, and an installed package without them behaves as before. Dinah's `go.mod` replaces the SDK module with the local checkout until the SDK's next release is tagged, which is Paul's call.

## Implementation status, 25 October 2026

The first three work items are implemented. The public [SDK 0.1.0](https://github.com/paulmooreparks/dinah-extension-go/releases/tag/v0.1.0) and signed [Git extension 0.1.0](https://github.com/paulmooreparks/dinah-git/releases/tag/v0.1.0) build independently of Studio. Dinah loads the published SDK module without a sibling-source override.

Dinah provides the Extensions window, folder/ZIP/Git-package installation, publisher-key pinning, enable/disable/remove, signed asset serving, native MCP backends, invocation-scoped host callbacks and sandboxed views. Extension actions enter the existing agent discovery and consent path. Every process action is conservatively effectful, including Git status. The host refreshes action catalogs after package changes and refuses an old discovered action against a newer package signature.

The Git release includes the planned first-release operations and also stashes, tags, worktrees, merge, rebase, cherry-pick and revert commands. The docked pane follows the workspace; a separate Git window pins its workspace and can open the selected diff. Show Me First reviews each actual external command, and the Command Log retains literal arguments and bounded output. Commands using stdin patches cannot be replayed through the current Terminal handoff.

The implementation adjusts several details of this plan. Host callbacks use invocation-scoped authenticated HTTP rather than assuming reverse MCP tool calls. Tool metadata uses namespaced MCP `_meta`. Native code runs under the user's operating-system account; a program's starting directory is checked, but process permissions are not an OS filesystem sandbox. Windows uses a process job for lifetime control, while other platforms use the cooperative MCP stdio lifecycle. Frames permit local form handling and block browser form requests with CSP. [The implementation contract](../design/extensions.md) and the public SDK contract describe these decisions.

File-tree decorations, editor gutter marks and workspace groups remain the fourth work item. `host.decorate` is reserved and returns an unavailable error until those surfaces exist. Built-in applet migration remains the fifth item. Git credentials, interactive editors and conflict resolution continue to use Git's configured tools or Terminal.

## Language Packs, Themes, Formatters and Linters, 28 October 2026

Everything Dinah can install today is an applet, with a back end process and frames. Language support, themes, formatters and linters are mostly data: a grammar, a set of colours, or a rule that one program formats or checks one language. They change the Editor and the window's appearance rather than adding a pane, so forcing them into the applet shape would make a theme ship a signed native executable it never runs. Paul decided on 28 October 2026 that they are not extensions, and that each is known by its own name rather than by a collective one: a Language Pack, a Theme, a Formatter or a Linter.

### What they have in common

Each is a folder with a manifest and the files it lists, signed and pinned to its publisher's key as an extension is, and installed through the same `dinah ext` commands and Extensions window, where each kind has its own list. None contains an executable, a back end or a frame. Because nothing in it runs, the installation screen does not carry the native-code warning an extension's does. One folder may contain more than one kind, so a Language Pack can bring the Formatter its language usually uses, and the Extensions window lists each separately. The contract needs a single word for the folder and its manifest; that word stays in the contract and the code, and the person never sees it.

A language server, formatter or linter program is native code, and installing one runs code under the person's account. That consent belongs to installing the program, which Dinah does from the mason registry or finds already on the path, and it is asked when the program is first needed rather than when the Language Pack, Formatter or Linter is installed.

### Language Packs

A language pack says which files belong to a language and what the Editor needs for them: the CodeMirror grammar or legacy mode, comment syntax, brackets and indentation. It can also give the mason registry entry of a language server, which Dinah installs and runs in the daemon as the design already describes. Each running server's diagnostics, references, definitions and rename are offered to the workspace's agents as actions as well as to the person in the Editor, which is the part VS Code cannot match, since its language features serve only its own interface.

The first packs are Go (gopls), TypeScript and JavaScript, Python, C#, Markdown (marksman), and YAML and JSON with schema support. Several of the best-known VS Code language extensions are licensed for Microsoft's products only, Pylance, C/C++ and C# Dev Kit among them. Python therefore uses basedpyright and C/C++ uses clangd; C# uses csharp-ls, or Roslyn's own server once its licence has been checked.

### Themes

A theme gives values for PUDL's tokens and a map from CodeMirror's highlight tags to colours. It recolours the window but cannot restructure it, so raised, sunken and flat surfaces keep their meanings under every theme, and a theme that would make two categories indistinguishable is refused at install.

Most of the themes people choose already exist as VS Code colour themes: One Dark Pro, Dracula, Catppuccin, Tokyo Night, GitHub Theme, Nord, Gruvbox and Solarized. A VS Code theme's `tokenColors` map reasonably well onto highlight tags, and its window colours map in part onto PUDL's tokens. Dinah converts such a theme when it is installed and produces an ordinary Dinah Theme, so an installed theme never depends on VS Code's format staying the same. Paul decided on 28 October 2026 that the converter can also be run on demand, outside installation, for example to try a theme before installing it, to convert a theme file someone wants to edit by hand, or to convert again after the original changes. Open VSX publishes these themes under their own licences and is a lawful place to fetch them from, provided each theme's licence is checked.

### Formatters and Linters

A Formatter says that one program formats one language, with its arguments, and whether it runs on save or only on command. A Linter says the same for a checker and how to read its findings. Dinah runs both through the process runner the Terminal and `host.run` use, shows a linter's findings in the Editor's gutter and in Problems, and offers them to agents. Many important tools are not language servers, which is why this cannot rely on a server's formatting alone.

The first Formatters and Linters are Prettier, gofmt, CSharpier or `dotnet format`, Ruff, markdownlint and cspell for spelling. The first wave also includes Vale, a prose linter driven by rule files, for the writing half of Dinah. Paul's own writing rules can be written as a Vale style, so the person sees a problem while writing and an agent checks its own draft against the same rules before handing it over.

### After the first wave

These are applets or extensions in the existing sense, listed here so the first wave is chosen with them in view:

- any existing MCP server wrapped in a signed extension manifest with permissions and effect levels, which needs little more than the manifest because an extension's back end is already an MCP server;
- skill and role packs, which install a working set of skills and agent definitions, such as a reviewer, a spec writer and a tester, into a workspace;
- document tools: Mermaid and PlantUML in the Markdown preview, export to Word and PDF through Pandoc, a CSV and table editor, and citations from Zotero;
- a test runner whose results both the person and the agents can see;
- pull requests on GitHub and GitLab, built on the Git extension and the Editor's diff view.

A debugger over DAP, remote SSH and containers wait until later. Each is a large piece of work, and none shows what is distinctive about Dinah.

## Decided, 28 October 2026

1. Language support, themes, formatters and linters are not extensions and have no back end or frame (Paul).
2. Each is known by its own name, Language Pack, Theme, Formatter or Linter, with no collective name shown to the person. Paul preferred this to a single word such as "pack", which prior art (Minecraft's resource packs, Blender's add-ons and themes, WordPress's plugins and themes) had suggested. "Language Pack" means a programming language here, whereas in VS Code it means a translation of the interface (Paul).
3. A VS Code theme is converted when it is installed, and the converter can also be run on demand (Paul).
4. Each is signed and pinned to its publisher's key as extensions are, because a Theme or a Formatter can still do harm (a Formatter chooses which program runs on save), and one trust model is simpler than two (Paul).
5. Linters are a kind of their own rather than a variety of Formatter, because a Linter reports problems and a Formatter rewrites the file (Paul).
