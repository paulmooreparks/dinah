# Task tracking

Dinah tracks tasks. Each workspace has one board, which a person or an agent can drive entirely from the command line as `dinah task`, and which Dinah Desktop shows and edits in its own windows. This document records Paul's decisions of 25 October 2026 and the design that follows from them. Paul approved the design that day. Building it waits until the Git extension works, which Paul chose to finish first, so nothing has been built yet.

## Decisions

Paul decided these on 25 October 2026.

- Task tracking is a new feature of Dinah. It is not a merger with the old board tool, and none of that tool's code comes across.
- It should be somewhat lighter than the old tool, with a graphical side, and still fully usable from the command line.
- Every workspace has exactly one board. It comes with the workspace at no cost, so a workspace with no tasks has no board files at all.
- The board lives under Dinah's state folder, at `~/.dinah/workspaces/<workspace-id>/tasks/`, and not in the workspace's repository. Paul first asked for somewhere under a `.dinah` folder and then accepted this location, for the reasons given under Storage.
- No old boards are imported. Work starts from an empty board, and anything worth keeping is copied in by hand.
- The old tool's boards leave Paul's repositories. Both were moved on 25 October 2026 to `C:\Users\paul\source\archive\old-dinah-boards\`, the old tool's own board from its repository and DinahVisor's board from DinahVisor's.
- Andoneer is left out for now.
- The Git extension comes first, and task tracking is built after it.

He offered `dinah task` or `dinah track` as the command. I chose `dinah task`, because the subcommands read as plain English after a noun (`dinah task add`, `dinah task move T-4 doing`) and read awkwardly after a verb (`dinah track add`).

## What the old tool taught

The old tool's storage analysis of 27 September 2026 found that keeping the current state in files that are overwritten loses text, because agents rewrote approved questions and specifications in place with nothing left of the earlier wording. It also found that one folder for every comment and checklist item made the store slow and gained nothing, and that a board kept inside a repository was put in `.gitignore` within weeks, because every claim and move dirtied the working tree. Its last design, written but switched off, made the card the unit of storage, with an append-only journal as the record and a readable Markdown file rebuilt from it. This design starts from that conclusion.

Three more ideas from the old tool proved their worth and stay. A column's instructions are given to whoever arrives there, so an agent dispatched with nothing but a task reference learns what to do. Every command answers in one of four ways (ok, refused, stale or unreachable), so an unattended caller can act on the answer. A change may carry the revision it was decided against, and a change decided on an outdated view is refused as stale.

## Weight

The tracker keeps tasks, columns, checklists, comments, claims, blocks and history. It leaves out routes, typed checklist items with owners, citations and evidence, grants, tiers, workstreams, spend records, links between tasks, scheduling dates, translations, a terminal UI, a language server and a conformance profile. Any of these can be added later when a real need appears, and the storage design leaves room for them.

The checklist is the main place the design is lighter. A task's checklist is the Markdown task list in its body, which the Visual editor already edits with checkboxes, and not a separate collection of structured items. A column can require every box to be ticked before a task leaves it, which keeps the old tool's gate for the common case.

## The model

A board has an ordered list of columns. A new board starts with Backlog, Ready, Doing, Review and Done, and Paul can rename, add, remove and reorder them. Each column has a title, a short identifier used on the command line, and optionally instructions, a limit on how many tasks it may hold, a requirement that the checklist be complete before a task leaves, and a hold, which means only Paul may move a task out of it.

A task has an identifier such as `T-12`, numbered in order within its workspace, a title, a Markdown body, a column, an optional priority (low, normal or high) and any number of tags. It may be claimed by one owner, either Paul or an agent's chat, and it may be blocked with a reason. Comments form a thread under the task.

Only Paul may lift a block. A claim held by an agent lapses when that agent's chat session ends, which Dinah already knows through the bus.

## Storage

A workspace's board is kept at `~/.dinah/workspaces/<workspace-id>/tasks/`. The folder is created by the first task, so a workspace that never uses tasks costs nothing.

- `board.md` holds the columns. Its front matter has each column's settings, and each column's instructions are a section of the file's body, so a person can edit it in any editor.
- `T-12.jsonl` is task 12's journal. Every change appends one line, with the full new text for a change to the title, body or a comment, so no wording is ever lost. The journal is the record.
- `T-12.md` is the readable form of the task, rebuilt from its journal after every change. When someone edits it by hand, the next command notices and writes the edit into the journal as a witnessed change.
- `archive/` receives a task's two files when it reaches the last column, so finished work leaves the live set. A command can restore it.

Writes are flushed to disk before they are renamed into place, each task is locked while it changes, and the counter that numbers tasks is changed under a lock of its own. The board records the named capabilities it uses, so a future build that adds a feature changes only the boards that use it.

I chose Dinah's state folder over a `.dinah` folder in the workspace's repository for three reasons. A workspace can span several folders, so no single repository is the obvious owner. The workspace's stable identity lives in Dinah's registry, which is the natural place to key it. A board inside a repository dirties git with every move, as the old tool found. The cost is that the board does not travel with the repository; if Paul later wants a board shared through git, a workspace setting could move the folder into a repository without changing the format.

## Command line

`dinah task` finds the workspace from the current folder, as the registered workspace whose folders contain it, or from `--workspace` with a name or identifier. The registry is in the desktop's settings today, so reading it moves into the shared `internal/workspaces` package that the command line and the window both use.

```text
dinah task add <title> [--column <id>] [--priority <level>] [--tag <tag>] [--body <text|->]
dinah task list [--column <id>] [--tag <tag>] [--mine] [--blocked] [--archived]
dinah task board
dinah task show <task>
dinah task edit <task> [--title <text>] [--body <text|->] [--priority <level>] [--tag <tag>] [--untag <tag>]
dinah task move <task> <column>
dinah task done <task>
dinah task claim <task>
dinah task release <task>
dinah task block <task> <reason>
dinah task unblock <task>
dinah task comment <task> <text|->
dinah task check <task> <item-number> [--off]
dinah task archive <task>
dinah task restore <task>
dinah task columns
```

Every command takes `--json` for a machine-readable answer and `--basis <revision>` to make a change conditional on the task not having moved since. Exit codes follow the four answers: 0 for ok, 2 for refused, 3 for stale and 4 for unreachable. `claim` and `move` print the arriving column's instructions. Columns are changed by editing `board.md`, through `dinah task columns --edit` or the window, so there is one place a board's shape is defined.

## Agents

Agents get task tools on the bus's MCP servers, `dinah` for terminal sessions and `dinah-bus` for chats in the window, so no extra server has to be registered. The tools match the commands. A tool call acts as the calling chat's bus identity, so the board shows which chat holds a task, and a column's instructions are given once per chat rather than with every call.

## Dinah Desktop

A Tasks window shows the workspace's board as columns of task cards, built from PUDL components, with the filters for tag, owner and blocked tasks in its address so that a view can be reopened. Moving a card, by dragging it or through its row menu, runs the same move the command line does and shows the refusal when one comes back, such as a full column or an unticked checklist. A Task window shows one task: its title, column, priority, tags and owner, its body in the Visual editor with working checkboxes, its comments and its history. The windows follow changes made from the command line or by agents through documented file-change notification, so the board stays current without polling.

The What Needs Paul view, planned for Stage 3, takes blocked tasks and tasks waiting in held columns from every workspace's board.

## Delivery

Task tracking comes forward from Stage 3 and follows the Git extension. It is built in three steps, each usable on its own.

1. The store and `dinah task`, with tests of the journal, the rebuilt Markdown, hand-edit witnessing, locking, the stale check and archiving.
2. The task tools for agents on the bus servers.
3. The Tasks and Task windows in Dinah Desktop, with browser suites.

## Notes for whoever builds it

The old tool is still installed in `%LOCALAPPDATA%\dinah\bin`, and Codex starts its MCP server. With the boards moved, that server finds no workbench, and it can be unregistered from Codex when the old tool is retired. The old tool and Dinah both read `DINAH_HOME`, so nothing should set that variable for the old tool.

DinahVisor's board was tracked in its git repository, so moving it left 27 deleted files in DinahVisor's working tree. Committing that removal is Paul's call in that repository.
