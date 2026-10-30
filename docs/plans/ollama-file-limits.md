# Limiting the Folders Ollama's File Tools Can Reach

This is the shape of the limits on Ollama's file tools that Paul asked for on 29 October 2026 (message 593: "I want to be able to specify limits, or no limits") and that Architecture ruled on in message 594. It was written by Dinah Engine Cleanup before any code, and it answers the four questions Architecture asked: where the setting is stored, how the tools check a path against it, what the refusal says, and what the default is for a new install and for an existing one. The DESIGN.md change is listed at the end and made in the commit with this document. No code for it exists.

## What the code does today

Ollama has no tools of its own on the provider's side, so Dinah runs the loop and the tools (`internal/ollamaagent/tools.go`). Five of them reach files: `read_file`, `write_file`, `list_directory`, `find_files` and `search_text`. Each takes a path, which `turnTools.path` makes absolute against the chat's folder, and none of them checks where it points. `read_file` goes on to the window's shared handler, which calls `os.ReadFile` on whatever it is given. Only `write_file` has a check, and it asks the person, not refuses, when the path is outside the chat's folder (`turnTools.inside`). With Approve Everything chosen, even that question is skipped. This is what let the stuck chat search the whole of AppData.

A sixth tool, `run_command`, runs a shell, and a shell can reach anything the person can. Its `cwd` is also any path.

## The setting

Settings gains two fields, shown under Settings > Agents > Ollama and chosen by the person.

- `ollamaFileScope`, one of `workspace`, `folders` and `anywhere`.
- `ollamaFileFolders`, a list of absolute folders, used only with `folders`.

The three choices are the ones Architecture ruled: the chat's workspace folders; those plus a list of other folders; or anywhere.

**The chat's workspace folders** are the chat's own folder, which is the workspace's execution folder, and every local folder among the sources of the workspace that folder belongs to. They are worked out when a tool is called, from the settings at that moment, so a folder added to the workspace is reachable at once and a chat in no registered workspace has its own folder alone.

**Live.** The window gives the Ollama connection a function that answers "what may this chat reach?" and the tools ask it at each call, so a change in Settings applies to a chat that is already running, from its next tool call. Approvals are already passed to the live connection in the same way (`SetApprovals`).

Window methods follow the pattern of `SetOllamaApprovals`: `SetOllamaFileScope(scope, folders)` validates and saves in one settings write, and `State` reports the effective scope and folders so the page can show them. Validation refuses a scope that is not one of the three and a folder that is not absolute. A folder that does not exist yet is accepted, since a person may name one before making it.

## How a path is checked

**The check is made on the folder, not on the text of the path.** A string prefix would let `C:\work-secrets` pass for `C:\work`, and would be fooled by `..` and by links. Go's `os.Root` is the documented way to do it: opening a folder as a root and then opening names relative to it, it refuses a name that leaves the root by `..` or by a symbolic link, including a link inside the root that points outside. That documented behaviour is the whole argument, and the design assumes nothing about how Windows resolves a path that the documentation does not describe.

For a limited scope each file tool works like this.

1. The path is made absolute and cleaned, as it is now.
2. It is matched to the allowed folder it lies inside, by comparing the cleaned path with each folder's (`layout.Inside`, which compares by relative path and ignores case on Windows). A path inside none of them is refused here.
3. The operation is carried out through `os.OpenRoot(folder)` and the path relative to the folder: `Root.Open` for `read_file`, `Root.ReadDir` for `list_directory`, `fs.WalkDir` over `Root.FS()` for `find_files` and `search_text`, and `Root.WriteFile` with `Root.MkdirAll` for `write_file`. A link or `..` that would leave the folder makes the operation fail, and that failure is the refusal.

With `anywhere` the tools run as they do today.

The existing question for `write_file` outside the chat's folder remains for `anywhere`; with a limit set, a write outside it is refused, not asked, since the person has already said where Ollama may work.

**The shell.** `run_command` is not confined by this, because a shell cannot be: a command can read any file the person can. I will say that in the Settings text rather than leave it to be found out. What the limit does for it is check its `cwd` against the same folders, so a command is not started in a folder outside them. Whether a command should also always ask when a limit is set, even under Approve Everything, is a question for Paul, below.

## The refusal

The tool returns a failed result, which the model sees and the chat shows as a failed call. It names the path and says where the limit is set.

> /path/as/given is outside the folders this chat may use: C:\work, C:\notes. The person sets this in Settings > Agents > Ollama > File Access. Work within those folders, or tell the person what you need.

A path that is inside a folder but reaches out by a link says that, in the same words: it is outside, by the link. The list of folders is the effective list for this call, so the message is true when the person changes the setting.

## The default

- **A new install** gets `workspace`. `loadSettings` already knows when there was no `settings.json`; in that case it sets `ollamaFileScope` to `workspace`, and the first save writes it, so the choice is stored and does not depend on the file's absence afterwards.
- **An existing install** keeps what it has. A settings file without `ollamaFileScope` means `anywhere`, which is what Ollama can do today, so no one's chats lose a path they use until they choose. Paul's install is in this case. Settings shows a line beside the choice saying that Ollama can currently read any file the person can, with the way to change it.
- A stored value that is not one of the three is read as the default for an existing install (`anywhere`) and not as an error, as `OllamaApprovals` treats an unknown value.

## Tests

1. A path with `..` that leaves the folder is refused for each of the five tools; one that comes back inside is allowed.
2. A sibling whose name begins with the folder's (`work` and `work-secrets`) is refused. Replacing the check with a string prefix makes this fail.
3. A symbolic link or junction inside the folder that points outside is refused for each tool, and one that points inside is allowed. The test is skipped where the system will not let the test make a link.
4. With `folders`, the listed folders are reachable and others are not, and the chat's own folder and a workspace source folder are reachable with `workspace`.
5. `write_file` to a new file in a subfolder that does not exist yet works inside a folder and is refused outside, and does not ask under any approval setting.
6. A search over a limited folder walks the folder and not beyond it, and still stops at its file and byte limits.
7. The refusal names the path, lists the folders and says where the setting is.
8. Changing the setting changes what a running chat's next tool call may reach.
9. A fresh install's settings say `workspace`; an existing file without the key reads as `anywhere`; an unknown stored value reads as `anywhere`.
10. `SetOllamaFileScope` refuses an unknown scope and a relative folder, and saves in one write.
11. A command whose `cwd` is outside the folders is refused when a limit is set.
12. A mutation check for each of tests 2 and 3, as described.

## DESIGN.md changes

- A new paragraph in the Ollama part of the design says that Ollama's file tools can be limited to the chat's workspace folders, those plus a list, or nowhere in particular; how the limit is checked, which is by opening each folder as a root; that it does not confine `run_command`; and the defaults.
- The decisions table gets a row for the date.

## Questions for Architecture

1. Should a command always ask when a limit is set, even under Approve Everything, since the shell can step around the limit? It makes the limit mean more, and it overrides a choice Paul made on purpose. I would not do it without his word.
2. Is it right to refuse a write outside the limit, where `anywhere` asks, given that the person has already said where Ollama may work?

## Ruling

Architecture approved this shape on 29 October 2026 (message 599). Opening each allowed folder as a root with `os.Root` is the right check: it is documented to refuse a name that leaves the root by `..` or by a link, so the limit rests on Go's documented behaviour and on nothing about how Windows resolves paths. The Go version in go.mod has it.

1. **A command does not start asking because a limit is set.** Approve Everything is a choice Paul made on purpose, and a limit should not quietly override it. Settings says plainly, beside the limit, that it confines the file tools and a command's starting folder but not what a command reads once it runs. A person who wants commands confined as well chooses an approval level that asks.
2. **A write outside the limit is refused, not asked,** the same as a read. A limit that turned into a question for writes would mean two rules where the person set one, and the refusal says where the limit is set.

## What the code showed

Built on 29 October 2026 (Dinah Engine Cleanup). `os.Root` follows a symbolic link only when its target is relative and stays inside the root, so a link whose target is an absolute path is refused even when that path is inside the folder. The test therefore makes the link that stays inside with a relative target, and the plan's statement that such a link is allowed holds for relative links. A write inside the folders goes through the root and not through the window's shared writer, so the shared writer's own refusal of Dinah's settings and Devin's folders does not apply to it; those folders are outside any workspace folder unless the person has made one of them a workspace.
