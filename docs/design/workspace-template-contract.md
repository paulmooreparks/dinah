# Workspace Template Contract

A product built on Dinah is designing a guided way to start a new workspace for a customer: the person picks a customer, reviews the sources, chooses a folder, sets up a team of agents and decides whether the lead starts with research. Paul agreed the split of the work on 25 November 2026. The product owns the customer journey, which covers how a customer's starter is found and stored, the shared brief, publishing, onboarding and training. Dinah owns a neutral template format and the host operations the product's wizard calls. This document is the draft of Dinah's side. It contains no product code, and nothing in it is built.

Dinah's code must not know any company, customer, wiki product or issue tracker by name. Everything below is written so that a wiki page tree, an issue tracker's project or any other kind of source arrives through the source kind registration that already exists (docs/design/source-kinds.md), and a product's own data passes through Dinah without Dinah reading it.

The product's design document was the reference for this draft. Where this draft differs from it or goes further, the reason is given in the last section.

## The Template Format

A workspace template describes a workspace that has not been made yet: its sources, the folders and stock files of its main folder, and the team of agents that starts in it. It is a file format with a version, and it is a compatibility contract in the same way the workspace export is (docs/design/workspace-export.md).

### The Package

A template travels as a package. The package is either a bare JSON document or a zip archive whose `template.json` entry is that document, with one entry `files/<sha256>` for each stock file whose bytes are not written inline. A reader tells the two apart by the zip signature, as `workspaces.OpenArchive` already does for exports, and applies the same entry rules that `internal/workspaces/archive.go` applies today: safe entry names, no duplicates in any case, a size limit for each entry and for the whole archive, and a SHA-256 check of each file against the name it was stored under. The file extension comes from the profile, as `WorkspaceFileExtension` does for exports, through a new `TemplateFileExtension` whose default is `.dinah-template`.

The package is all Dinah reads. Where the product stores it, whether as an attachment, a table on a page or a file on a share, is the product's business, and its adapter hands Dinah the package's bytes together with an origin record that Dinah keeps without reading. This keeps the format independent of the first open question below.

The template's digest is the SHA-256 of the `template.json` bytes as stored. Because every stock file is listed in that document with its own SHA-256, the digest covers the files too, and a bare document and an archive with the same `template.json` have the same digest.

### Identity and Revision

The top of the document says what the template is and which revision it is (Table 1). Item IDs are the part that matters most for later updates, since they let Dinah tell a renamed source or role from a new one.

**Table 1** *Top-Level Fields of a Template*

| Field | Required | Meaning |
| --- | --- | --- |
| `format` | Yes | The fixed string `dinah-workspace-template`. |
| `version` | Yes | The format's major version, 1 for this draft. A reader refuses a version it does not know. |
| `id` | Yes | A stable identifier the author chooses, of letters, digits, `.`, `-` and `_`, at most 200 characters. It never changes across revisions. |
| `revision` | Yes | A positive integer that grows with each published revision of the same `id`. |
| `title` | Yes | The template's display title. |
| `description` | No | Plain text shown in the review. |
| `maintainer` | No | Free text saying who maintains the template. Dinah shows it and does nothing else with it. |
| `workspaceName` | No | A suggested name for a new workspace. The person can change it. |
| `sources` | No | The sources, described in Table 2. |
| `folder` | No | The main folder's directories and stock files, described in Table 3. |
| `team` | No | The lead, members and groups, described in Table 4. |
| `extensions` | No | An object keyed by product identifier. Dinah keeps each value as it came and hands it back to the product that registered that identifier. |

*Note.* Within version 1, fields are only ever added, and a reader keeps the fields it does not know (the `Extra` practice of `workspaces.Source`) so that a later Dinah reads an earlier template and an earlier Dinah passes a later one through unchanged. A change that alters what an existing field means needs version 2. Two packages with the same `id` and `revision` and different digests are refused as a reused revision.

The product puts its own concepts, such as a customer's identity, a project scope, the shared brief's location or a training link, in `extensions` under its own identifier. Dinah never interprets them. A product reads them back through the same registration that gives Dinah its source kinds, so a build that does not have that product keeps the data and shows nothing for it.

### Sources

A source in a template is described generically (Table 2). Its `kind` is a kind some build registered with `workspaces.RegisterSourceKind`, and its `locator` is that kind's own description of where the source is. Dinah turns the locator into the `Extra` fields of an `ExportSource` and then uses the kind's existing `Read`, `Describe`, `Check` and `Recreate`, so a kind that already imports from a workspace export imports from a template with no new code on the kind's side.

**Table 2** *Fields of a Source Item*

| Field | Required | Meaning |
| --- | --- | --- |
| `id` | Yes | The item's stable ID, unique within the template and kept across revisions. |
| `kind` | Yes | A registered source kind. A kind this build does not know is kept and shown as Not Available Here, as an imported one is today. |
| `name` | Yes | The source's display name. |
| `locator` | Yes | A JSON object that only the kind reads, such as a site address and a page or project identifier. |
| `connection` | No | The identifier of a connection registered with `internal/connections`, which is how the source's credentials are found on this computer. |
| `purpose` | No | Plain text saying why the source is in the template. |
| `required` | No | True when the workspace is not considered complete without the source. The person still decides what happens when it cannot be reached. |

*Note.* A local folder cannot be a source in a template, since its location belongs to one computer. The main folder is chosen when the template is applied, and every other folder the template wants is a directory inside it.

Credentials are only ever referred to. A template says which registered connection a source uses and never contains an address-and-token pair, a token, a password or a cookie. The connection's token stays where the product's connection keeps it today, normally the system keychain through `internal/credentials`. When the person has no such connection yet, the source's check says so, in the way `SourceKind.Check` reports a missing connection for an imported source.

### The Starting Folder

The `folder` section lists directories and stock files to make inside the main folder (Table 3). Every path is relative to the main folder, uses `/`, and must pass the rules of `localfs.CleanPath` and the write policy of `writablePath` in `applet_workspace_write.go`. That means no absolute path, no `..`, no hidden or dot-named part, no name Windows reads as another name, and no `AGENTS.md`.

**Table 3** *Fields of the Folder Section*

| Field | Required | Meaning |
| --- | --- | --- |
| `directories` | No | A list of relative paths of directories to make. |
| `files` | No | A list of stock files. |
| `files[].id` | Yes | The file item's stable ID. |
| `files[].path` | Yes | Its relative path. |
| `files[].sha256` | Yes | The SHA-256 of its bytes. |
| `files[].size` | Yes | Its size in bytes, at most `resource.MaxFile`. |
| `files[].text` | No | The file's content as UTF-8 text, for a text file of at most 64 KiB. Without it, the bytes are the archive entry `files/<sha256>`. |
| `files[].optional` | No | True when the person may leave the file out. |

*Note.* The limit on inline text keeps a bare JSON template small enough to store in places that cannot hold an archive. The 64 KiB figure is a proposal.

### The Team

The `team` section describes agents to be made fresh in the new workspace (Table 4). Its fields are the ones `dinah.team/describe@1` already returns for a chat in a team, less everything that belongs to a running chat, so a lead's existing team can be turned into a template's team section by dropping fields.

**Table 4** *Fields of the Team Section*

| Field | Required | Meaning |
| --- | --- | --- |
| `lead` | Yes, when `team` is present | The role that reports to the person. It has every field of a member except `lead`. |
| `members` | No | The other roles. |
| `members[].id` | Yes | The role's stable ID. |
| `members[].title` | Yes | The chat's title. |
| `members[].lead` | Yes | The ID of the role it reports to, which is the lead or another member. |
| `members[].remit` | Yes | What the role is for, kept as the member's remit in `team_members.remit`. |
| `members[].brief` | No | The role's first message. Without it, the role starts with no turn. |
| `members[].provider` | Yes | A provider identifier from Dinah's provider table. |
| `members[].model` | No | A model value that provider offers. Without it, the provider's default. |
| `members[].options` | No | Settings by option ID, as `dinah.team/start@2` takes them. Any setting of category `mode` is refused. |
| `members[].optional` | No | True when the person may leave the role out. |
| `groups` | No | Group chats beyond each lead's team group, each with an `id`, a `title`, a list of role IDs and an optional turn budget. |
| `budgets` | No | For a role that leads others, an allowance for each provider in that provider's unit, as `SetTeamBudget` takes it. |

*Note.* A team allowance warns the person when it is passed and stops nothing (`team_budget.go`). A template therefore cannot promise a hard limit on what research costs.

A template never grants authority. A permission mode in `options` is refused when the template is read, so every agent made from a template starts with its program's default mode, which is the fallback rule of docs/design/team-duplication.md for the case of a caller that does not exist. The person can raise a chat's mode afterwards, as for any chat.

When a role's provider or model is not on this computer, the role is shown as pending. Dinah never substitutes another provider. The person may choose one that is available, and that choice becomes a local override of the role.

### What a Template Never Contains

A template describes how to start and contains no work (Table 5). Dinah refuses a template that has any of these, and an authoring path that builds a template from a workspace leaves them out without asking.

**Table 5** *Content a Template Must Never Contain*

| Content | Why It Is Refused | How It Is Detected |
| --- | --- | --- |
| Chat histories and transcripts | They belong to the person who had the conversation. | The format has no field for them, and an archive entry outside `template.json` and `files/` is refused. A workspace export, which can hold chats, is a different format and is never accepted as a template. |
| Bus messages, group conversations and trusted links | A trusted link lets agents act as the person, and a file must not grant that. | No field exists for them. |
| Credentials of any kind | A shared file would spread them. | Only connection identifiers are accepted; a kind's `Read` refuses a locator that contains a secret, and the product's adapter checks the same before publishing. |
| Personal sources | They are the person's own choice for their own workspace. | The authoring path offers only sources the author selects, and the apply operation never treats a source it did not create as part of the template. |
| Session IDs, bus IDs and logical chat IDs | They identify running chats on one computer. | No field exists for them. |
| Absolute paths and computer names | They describe one computer. | Path rules in Table 3 and the refusal of local folder sources. |
| Permission modes | They are authority. | Options of category `mode` are refused. |
| Agent instruction files and hidden folders | They steer every agent in the workspace. | `AGENTS.md` and dot-named paths are refused, as `writablePath` refuses them for agents. |

*Note.* Dinah cannot prove that free text, such as a brief or a remit, contains no secret. That check belongs to whoever publishes the template.

### Example

The example below shows the shape with invented values. The `sample-wiki` kind and the `sample` connection stand for whatever a product registers.

```json
{
  "format": "dinah-workspace-template",
  "version": 1,
  "id": "example.onboarding",
  "revision": 3,
  "title": "Example Onboarding",
  "sources": [
    {"id": "src-handbook", "kind": "sample-wiki", "name": "Handbook",
     "locator": {"site": "https://wiki.example.com", "page": "4100"},
     "connection": "sample", "purpose": "Background reading", "required": true}
  ],
  "folder": {
    "directories": ["notes", "deliverables"],
    "files": [{"id": "file-readme", "path": "notes/README.md", "sha256": "…", "size": 120, "text": "# Notes\n"}]
  },
  "team": {
    "lead": {"id": "role-lead", "title": "Lead", "remit": "Coordinate the work.", "brief": "Read the handbook and list open questions.", "provider": "claude"},
    "members": [{"id": "role-analyst", "title": "Analyst", "lead": "role-lead", "remit": "Analyse requirements.", "provider": "codex", "optional": true}]
  },
  "extensions": {"example-product": {"anything": "the product reads"}}
}
```

### The Record of an Applied Template

A workspace made or updated from a template keeps a record of what was applied, so that a retry knows what is done and a later update can compare three states. Today's `workspaces.Record` has no field for this and, unlike `Source` and `Pin`, no `Extra` map, so the record needs a new field (Table 6).

**Table 6** *Fields of the Applied Template Record*

| Field | Meaning |
| --- | --- |
| `template.id`, `template.revision`, `template.digest` | The template that was applied last. |
| `template.origin` | The product's origin record, kept as it came, at most 4 KiB. |
| `template.appliedAt` | When the last operation finished. |
| `template.operation` | The ID of the operation that applied it. |
| `template.items` | For each item ID, what it became here and its state: a source ID, a directory path, a file's path and the SHA-256 written, a role's logical chat ID, or a group's key. The state is `applied`, `kept-local`, `declined` or `pending`. |

*Note.* The template document itself is kept in Dinah's state folder under `templates/<digest>.json`, so the base of a later three-way comparison can be read again. Stock file bytes are not kept, since the comparison needs only their SHA-256.

A source, file, role or group that is not in `template.items` was not made by a template. That is how Dinah knows a source is the person's own, and it needs no flag the person could forget to set.

## The Boundary

Each concern in the onboarding work has one owner (Table 7). Where both sides are involved, Dinah offers a mechanism with no knowledge of customers, and the product decides what to do with it.

**Table 7** *Ownership of Each Concern*

| Concern | Owner | What the Other Side Does |
| --- | --- | --- |
| Finding a customer's starter, labels, catalogues and setup pages | Product | Dinah has no part. |
| Storing and fetching the package | Product | Dinah reads the bytes it is handed and keeps the origin record unread. |
| The template format and its validation | Dinah | The product produces packages that pass Dinah's validation. |
| Source kinds and their locators | Product, through `RegisterSourceKind` | Dinah calls the kind's `Read`, `Check` and `Recreate` and stores the source. |
| Access checks against a site | Product's source kind | Dinah shows the result and keeps the classes apart. |
| Credentials | Product's connection, through `internal/connections` | Dinah keeps only the connection's identifier. |
| The main folder, directories and stock files | Dinah | The product supplies the template; the person chooses the folder in a Dinah step. |
| Conflicts with local files | Dinah | The product's wizard shows Dinah's plan and passes the person's choices back. |
| Making agents, teams and groups | Dinah | The product supplies roles through the template and may replace a role's first brief at apply time. |
| Operation identity, receipts, retry and resume | Dinah | The product keeps the operation's ID in its own route and asks Dinah for the operation's state. |
| Comparing an applied template with a newer one | Dinah | The product shows the change list and passes back the person's choice for each change. |
| Starting the lead's first turn | Dinah sends it; the product writes it | The product composes the brief, including any reference to the shared brief and its scope. |
| Research status as the person sees it | Product | Dinah reports the step's receipt and the chat's state, and the product derives its own status words. |
| The shared brief, its review and its access filtering | Product | Dinah stores a copy only as an ordinary stock file or a source, if the product puts it there. |
| Publishing a template or a brief | Product | Dinah never publishes anything to a site. |
| Onboarding and training | Product | Dinah has no part. |
| Building a template from an existing workspace | Dinah builds the neutral part; the product adds its extension and publishes | Dinah reads sources, folders and `describe@1` and writes a package locally. |

*Note.* "Product" here means any product built on Dinah. Dinah's own build registers no source kinds and no connections, so in Dinah alone a template can carry only directories, stock files and a team.

## Host Integration Points

The wizard needs five things from Dinah. For each, the table says what exists in the code on master today and what this contract would add. Nothing marked new exists yet.

### Source Import

Most of what source import needs already exists for workspace import, and the template path should reuse it rather than grow a second one (Table 8). The gaps are a structured check result and matching by item ID.

**Table 8** *Source Import, Today and New*

| Need | Today | New |
| --- | --- | --- |
| A kind can describe and recreate a source | `workspaces.SourceKind` with `Read`, `Describe`, `Check`, `Export`, `Recreate` and `DefaultName` (`internal/workspaces/kinds.go`). | A function that turns a template source into an `ExportSource` whose `Extra` contains the locator and connection, so the same functions apply. |
| Plan before acting | In `workspace_export.go`, `planLocal` lists each source and locates each local folder, and `PlanWorkspaceImport` then calls the `Check` of every other registered kind, with an 8 second limit for each source. | A `PlanTemplate` call that returns the same kind of plan for a template, including files and roles. |
| Check results the person can act on | `Check` returns a single string, empty when the source can be used. | A result with a class (`ok`, `denied`, `missing`, `offline`, `no-connection`, `unchecked`) and a detail, so a denied page is never shown as a missing one. Kinds that only return a string are read as `unchecked` with that string as the detail. |
| Add a source without a duplicate | `ImportWorkspace` skips a source when `sameWorkspaceSource` (`workspace_sources.go`) finds one of the same kind with the same path, for a local folder, or the same `Extra`, for any other kind; `attachWorkspaceSource` does the same for one source. | Matching by item ID through `template.items` first, then by `sameWorkspaceSource`, so a template source whose locator changed between revisions is updated in place and is not added again. |
| Credentials | `internal/connections.Register` and `Lookup`; tokens in `internal/credentials`. | Nothing in Dinah. A source whose connection is not registered or not set up is shown with the class `no-connection`. |
| A kind this build does not know | Kept with its fields and shown as Not Available Here. | The same for templates. |

*Note.* `AttachWorkspaceSource` in `workspace_source_kinds.go` builds a source from a form through `Attach.Build`. The template path does not use it, because a template already contains the built locator.

### Folder and Stock Files

Dinah today can make folders for an import but cannot place files from one, and it never makes a folder unless the person asks, because a folder still arriving through a synchronising service could collide (Table 9). The template path keeps that rule and adds stock files with an explicit conflict plan.

**Table 9** *Folder and Stock Files, Today and New*

| Need | Today | New |
| --- | --- | --- |
| Choose or create the main folder | The import window settles the main folder before the workspace is made, since chats find their workspace by its path and it cannot change later (`ImportWorkspace`, `checkWorkspaceFolder`). Create It Now runs `createFolders` only when the person ticks it. | The same rule. The plan proposes no path of its own beyond the product's suggestion, and the folder is made only on the person's choice. |
| Make directories | `createFolders` uses `os.MkdirAll` and skips any path that `layout.Inside` rejects; `plannedFolders` records folders to make later. | Directories from the template go through `localfs.Folder.MakeFolder`, which works inside an `os.Root`, so a link cannot lead outside the main folder. |
| Write a new file without overwriting | `localfs.Folder.CreateFile` opens with `O_EXCL` inside an `os.Root` and refuses a file that exists; `dinah.workspace/write-file@1` writes text and replaces a file only at the revision the caller read. | Stock files are written with `CreateFile`. A file that exists is never replaced unless the person chose a replacement and the file still has the SHA-256 seen in the plan, checked through `WriteText` with that revision. |
| Classify each file before writing | Nothing. Exports carry no file contents at all, as the earlier proposal noted. | Each file in the plan is one of: `new`, `same` (already there with the same bytes), `conflict` (a different file at that path, never written by a template), `unchanged-since-applied` (written by the last revision and not edited since) or `edited-locally`. |
| Resolve a conflict | Nothing. | The choices are Keep Local, which is the default, Save Incoming Beside It under a name that adds the template revision, and Replace, offered only for `unchanged-since-applied`. A merge is not offered by Dinah; the person can merge the saved copy themselves. |
| Changes between plan and apply | Nothing. | Apply re-reads each file's SHA-256. A file that changed since the plan stops that step and asks for a new plan of that file. |

*Note.* Whether a synchronising service will later bring a file to the same path is not something any documented interface can tell Dinah. The plan therefore treats only what is on disk at apply time as the truth and says so beside a folder the person did not create in this operation.

### Team Creation

The team machinery covers most of the shape, but every one of the six `dinah.team` actions needs a calling chat, and the wizard has none (Table 10). Each handler starts with `teamCaller` in `applet_team.go`, which refuses an invocation whose owner kind is not `chat`, a calling chat that is not open, and one that is not on the bus. The task host can already open a fresh chat for a person-owned task with no calling chat, but nothing in it makes that chat a team member with a remit. The new work is a host operation that makes the lead through the task host and then its members through the same internal steps `teamStart` uses.

**Table 10** *Team Creation, Today and New*

| Need | Today | New |
| --- | --- | --- |
| Read a team's shape | `dinah.team/describe@1` in `team_duplication.go` returns remit, provider, model, options and settings for each chat in the caller's tree. | Authoring maps that output into Table 4 and drops what belongs to a running chat. |
| Make a chat with a durable record of the operation that made it | The task host's `OpenChat` operation (`task_host_operations.go`) calls `SessionLifecycle.NewRecorded` (`internal/sessionhost/session_lifecycle.go`) with an `AllocationIntent` whose `Source` is a digest of the service, workspace, task and operation IDs. Its callbacks persist the dispatch obligation before the provider is asked and the provider's session ID at the first documented success boundary (docs/design/recorded-session-allocation.md). `CreateNamedChat` in `named_chat.go` makes a chat with a title and a model by the ordinary path and deletes or hides it again when either fails. | Every role's chat is made through recorded allocation. `CreateNamedChat` is not used, because the task host forbids a native delete as rollback and its ordinary path keeps no record of the operation. |
| Make a member with a remit under a lead | `teamStart` in `applet_team.go`, used by `dinah.team/start@1` and `start@2`, starts the chat with `NewSessionWithProviderAt`, calls `bus.SetLeadWithRemit`, and posts the start line and the brief in the lead's group. It needs the calling chat from `teamCaller`. | A host function that runs the bus steps of `teamStart` for a chat made by recorded allocation, with the person as the requester, so a member gets its remit and its place in the tree without a calling chat. `start@1` and `start@2` keep their own rule. |
| Make a chat a member without a remit | `SetTeamLead` in `team_views.go`, which is the person's own action. | Not used, since it writes no remit. |
| Settings and authority | `checkTeamOptions` and `applyTeamOptions` in `team_duplication.go` accept a setting of category `mode` only when `permittedModes` allows it, which is the caller's own mode on the same program or the program's default, and list settings that could not be given in `notApplied`. Both take the calling chat's session as an argument. `checkTeamOptions` checks nothing when the program has no saved option list, leaving the check to `applyTeamOptions` once the chat exists. | A variant of the two functions with no caller. Since a template contains no mode, it checks only that each setting is offered and allowed, and lists the rest in `notApplied`. |
| Groups | `bus.ImportGroup` (`internal/bus/imported.go`) makes a group with its turn budget or joins one of the same title; a team's group is made by setting its lead. The task host's room operations (`EnsureRoom`, `PostRoom` and the rest) are refused today, because the desktop never sets a room adapter (`taskRooms` in `task_host.go` returns nil), as docs/design/task-host-binding.md requires until the complete adapter exists. | Template groups are made with `ImportGroup`, keyed by the operation and group ID rather than by title, so a group of the same title made by the person is not joined by accident. |
| Asking the person once | The `duplicate-a-team` built-in skill asks with `ask_person` before starting anyone. | The wizard's own review is that single approval. An agent that applies a template through an applet action asks with `ask_person` first, as the skill does. |
| Budgets | `SetTeamBudget` in `team_budget.go`. | Applied from `budgets` after the roles exist. |

*Note.* No action in the `dinah.workspace` applet today makes a workspace or attaches a source, and no action in `dinah.team` makes a top-level chat. A template applied by an agent would need new actions for both, with the capability `team.manage` and a new capability for workspace creation.

### Resumable Create and Update

Dinah already has a durable journal for multi-step work that opens chats, takes turns and invokes actions: the per-window task host of docs/design/task-host-binding.md, in `internal/taskhost` and `cmd/dinah-desktop/task_host*.go`. It does not cover registering a workspace, attaching sources or writing stock files, and it is reached only by a service registered with `services.RegisterWindow`, which Dinah's own build does not do (Table 11). The task host's design says that work of this kind must reuse its allocation journal, with one receipt owner, and must not invent a second one. The template operation is therefore a window service of Dinah's that consumes the task host for chats, turns and actions and keeps the steps the task host does not have in its own task record.

**Table 11** *Resumable Operations, Today and New*

| Need | Today | New |
| --- | --- | --- |
| A journal of steps with receipts | `taskhost.Window` persists an intent for each operation before any native work, with a typed receipt state (`prepared`, `in_progress`, `succeeded`, `failed`, `cancelled` or `unknown`). Reusing an operation ID with the same request returns the existing receipt, and reusing it for different work is refused. A receipt left `prepared` or `in_progress` blocks renewed execution until it is reconciled, and shutdown turns such receipts into `unknown` with no replay (`internal/taskhost/window.go`). | A Dinah window service, registered with `services.RegisterWindowNamed`, whose task is one apply operation. Chat, turn and action steps are task host operations. |
| Steps the task host does not have | The task host's operation kinds are `OpenChat`, `TakeTurn`, `UseModel`, `ReadOptions`, `CancelTurn`, `ReleaseChat`, `InvokeAction`, `CancelAction`, `AskPerson` and the room operations (`task_host_operations.go`). None registers a workspace, attaches a source or writes a file. | The service's own task record contains a step and a receipt for each of these, with the same receipt states, written through `internal/statefile`, which leaves either the old file or the new one after an interrupted write. |
| Consent before work | `Begin` accepts only a review receipt the person confirmed through the trusted page path, or the grants of an existing invocation. `Rebind` after a restart allows reconciliation only, and renewed execution needs a new confirmed review. | The wizard's Review and Create step is that confirmation. Resume after a restart is a reconciliation followed by a new confirmation of what is left. |
| One action invocation done once | The applet broker derives a stable invocation ID from the caller's `requestId` (`applet_mcp.go`). A retry with the same ID returns the stored result, and an interrupted `once` action is reported as interrupted and never re-run (`internal/applet/broker/broker.go`). | Action steps go through the task host's `InvokeAction`, which keeps the broker as the owner of the invocation. |
| Import twice without harm | `ImportWorkspace` adds only missing sources and pins; chats match by logical chat ID; group messages match through the bus table `message_origins`. | The same idea for the steps outside the task host, keyed as in Table 12. |
| Plan identity | The task host binds a review to the task record's generation and each operation's input fingerprint, and refuses changed input. | The task record's generation includes the template digest and a digest of the person's choices. A changed choice after any step has run needs a new review. |
| Cancel | Task cancellation closes admission, cancels owned work and records the real outcome of anything that completed. Nothing is rolled back. | The same for the service's own steps. |

*Note.* Every claim about the task host in this table is from its design document and from the code on master. The task host design records its own acceptance separately and claims no acceptance pass in the document itself.

Each step's key decides whether the step's work already exists (Table 12). A step writes its intent before it acts and its receipt after, and a step found with an intent and no receipt on resume is settled by looking for its key.

**Table 12** *How Each Step Is Recognised on Retry*

| Step | Key | Settled On Resume By |
| --- | --- | --- |
| Make the workspace | The main folder's path | `workspaces.IndexAt`, as import does. |
| Attach a source | Item ID, then kind and locator | `template.items`, then `sameWorkspaceSource`. |
| Make a directory | Relative path | Whether it exists. |
| Write a stock file | Relative path and SHA-256 | Whether the file has that SHA-256. |
| Make a role's chat | Operation ID and role ID | The task host's `OpenChat` receipt, which contains the provider's session ID once received. |
| Make a group | Operation ID and group ID | A tag recorded on the group. |
| Send the lead's first brief | Operation ID and role ID | The task host's `TakeTurn` receipt. |
| Send a member's first brief | Operation ID and role ID | A bus origin record for that pair, in the way `message_origins` records an imported message, since the task host's room operations are unavailable. |

*Note.* A `TakeTurn` whose response was lost is `unknown`, and the task host never resends a prompt on that account. The person sees the lead's chat and decides.

### Tagging a New Chat With Its Operation

Software Director decided on 25 November 2026 how a new chat is tied to the operation that made it. The design to aim for is to tag the new chat with its operation in the same write that records the chat. The fallback is to report that the operation may have made a chat and to wait for the person, and it is acceptable only where that one-write tag cannot be had.

The one-write tag can be had today for the part of the allocation that any documented interface can establish. `NewRecorded` writes the operation's intent, whose `Source` identifies the operation and role, before the provider is asked for a session. Its `Received` callback then writes the provider's session ID into that operation's receipt at the first documented success boundary. That receipt is the first durable record of the chat anywhere in Dinah, because `newSession` writes the transcript, the chat's options and its bus identity only after `Received` returns. A chat made this way is therefore never recorded without its operation. The tag sits in the operation's receipt and not on the chat's own record, which is enough for a retry to find the chat by its operation and role.

The fallback remains for one case. Between the moment the request reaches the provider and the moment the provider's answer arrives, a crash or a lost connection leaves the receipt `unknown`. Only a method-not-found answer with no session ID proves that no session was made (docs/design/task-host-binding.md, as corrected on 16 November 2026), so the operation cannot know whether the provider made one. In that case, and only in that case, the step reports that it may have made a chat and waits for the person. It never allocates again on its own.

The draft's earlier wording asked for a tag in the same update that records the chat in Dinah's inventory, through `NewSessionWithProviderAt`. That path cannot provide it, since `NewSessionWithProviderAt` calls `SessionLifecycle.New` with no callbacks and nothing records the operation. Recorded allocation is the path the tag needs.

Re-applying a newer revision is an update operation that compares three states: the applied template kept under `templates/<digest>.json`, the new template, and the workspace as it is now. Each item lands in one category, which the product's wizard shows as a list to choose from (Table 13). Nothing in the person's own work changes without a choice.

**Table 13** *Changes an Update Offers*

| Category | When | Default |
| --- | --- | --- |
| Added | The item is in the new revision only. | Listed for the person to choose; nothing changes until they do. |
| Changed | The item differs between the two revisions and the workspace still matches the old one. | Listed for the person to choose; nothing changes until they do. |
| Changed Here and There | The item differs between the revisions and the person also changed it locally. | Keep Local. |
| Suggested Removal | The item is in the old revision only. | Keep. For a role, the choice offered is retiring the chat with `dinah.team/retire@1`'s behaviour, which keeps its history. |
| Kept Local | The person changed an item that the new revision did not change. | Nothing to do; listed for information. |
| Personal | A source, file or chat not in `template.items`. | Never touched and not listed as a change. |

*Note.* An update never deletes a chat, a conversation or a file, and it never starts a turn.

### The Lead's First Research Turn

Dinah can already start a chat's turn and post a brief to a member, and an update must never start one by accident (Table 14). The first turn is therefore its own step that runs only when the operation's choices include it.

**Table 14** *Starting the First Turn, Today and New*

| Need | Today | New |
| --- | --- | --- |
| Start a chat's turn | The task host's `TakeTurn` takes the chat's exclusive turn admission, sends one labelled turn through the normal queue, file authority and permission checks, and reports completion, refusal, cancellation or an unknown outcome (`task_host_operations.go`). `App.Send` in `session.go`, the composer's path, sends a prompt with no receipt and refuses while the window waits to restart (`TurnIO.Send`). | The `start-brief` step sends the lead's brief with `TakeTurn` after the team exists. |
| Brief a member | `teamStart` posts the brief in the lead's group with `SendMentioning`, which wakes the member. | Members whose role has a brief get it the same way when the step runs, with the bus origin key of Table 12. |
| The words of the brief | The caller writes them. | The template's `brief` is the default. The apply call accepts a replacement per role, so the product can compose the research instruction, including the shared brief and its scope, without Dinah reading it. |
| Not starting twice | The `TakeTurn` receipt for the lead, which is never resent after a lost response. Nothing for a brief posted in a group. | The `TakeTurn` receipt for the lead and the bus origin key of Table 12 for members. A retried creation never sends a second brief, and an update has no `start-brief` step. |
| Cost limits | Team allowances warn and hold nothing; group turn budgets limit how many turns agents start in a group (docs/plans/group-turn-budgets.md); effort is a model setting. | No new limit. The step can set a group turn budget and a team allowance from the template, and the review must not call either a cap on cost. |
| Progress | A chat's state and current task, as `dinah.team/list@1` returns them. | The operation's state reports the step's receipt and the lead's chat. The product turns those into its own research status. |

*Note.* Whether the step runs by default is open question 2 below. The contract supports either answer.

## Questions Open With Paul

The contract was written so that most of the open questions do not change the format or the host operations (Table 15). Two further questions come from this draft rather than from the product's design.

**Table 15** *Open Questions and Whether the Contract Depends on Them*

| No. | Question | Contract Depends On It? | How the Draft Stays Independent |
| --- | --- | --- | --- |
| 1 | Where is a starter stored: a table on a page or a versioned attachment? | No | Dinah reads a package's bytes and keeps an opaque origin. Inline text in a bare JSON document keeps a table-based store possible. |
| 2 | Does research start by default? | No | The first brief is a separate step chosen per operation. The product picks the default. |
| 3 | Who publishes shared briefs? | No | Dinah publishes nothing and stores no brief as such. |
| 4 | What is the pilot's scope? | Partly | The format does not change. The pilot decides which source kinds must be registered and tested, and whether the update operation is needed in the first build. |
| 5 | Is authoring a skill, a flow or a command? | Partly | The host functions are the same for all three. A skill run by an agent also needs applet actions for building and applying a template, guarded by `ask_person` as `duplicate-a-team` is. A command or a product flow can call the host functions directly. |
| 6 | May the person, through a wizard, make a lead and its members with remits without a calling chat? | Yes | Raised by this draft. Without it, the lead can still be made as a person-owned task host chat, but members could only be started by the lead's own agent with `dinah.team/start@1` or `start@2` once it has a turn. |
| 7 | Is a workspace the unit of source access? | Yes, for the product's wish to scope sources per agent | Raised by this draft. Dinah gives no chat a source list of its own. The workspace applet's source, resource and mount actions find the calling workspace from the chat's folder (`callingWorkspaceRecord` in `applet_workspace_resources.go`) and limit a read to that workspace's ID and nothing narrower (`scopedWorkspaceVFS` in `workspace_vfs.go`), so every chat in a workspace reaches every source attached to it. File reads through Dinah follow workspace-wide rules, unconfined apart from deny rules, one default of which covers other workspaces' data (docs/design/agent-file-access.md). The draft gives roles no source list. |

## Where This Draft Differs From the Product's Design

The product's design document asked for a neutral contract from Dinah and sketched one. This draft agrees with most of it, and the points where it disagrees or goes further are listed with their reasons (Table 16).

**Table 16** *Differences From the Product's Design*

| No. | Point | Kind | Reason |
| --- | --- | --- | --- |
| 1 | The brief, training and project scope are not template fields; they go in `extensions` under the product's identifier. | Disagrees | The agreed split gives the brief, training and scope to the product. Fields in Dinah's format would make Dinah responsible for their meaning. |
| 2 | The origin is an opaque record the product supplies, and the digest is one Dinah computes itself. | Disagrees | The sketch listed connector and resource fields in the origin, which tie the format to one kind of store. |
| 3 | Unknown fields inside a known major version are kept and ignored; an unknown major version is refused. | Goes further | The sketch said only that unknown versions fail. Keeping unknown fields is how workspace exports already stay compatible between builds. |
| 4 | The existing team actions cannot be used by a wizard, because they need a calling chat. | Disagrees | The design assumed the existing agent and team mechanisms suffice. `teamCaller` refuses every caller that is not an open chat on the bus, and the task host, which needs no calling chat, has no way to make a team member, so a new host function is needed (open question 6). |
| 5 | Roles carry no source list and no capability requirements. | Disagrees | Dinah has no per-agent source scope (open question 7). A field Dinah cannot enforce would be a promise it does not keep. |
| 6 | Permission modes are refused in a template. | Goes further | The design said a starter cannot expand authority. Refusing the setting outright makes that true without a comparison of modes, which no program documents. |
| 7 | `AGENTS.md`, hidden paths and local folder sources are refused. | Goes further | The design excluded executable instructions and machine paths. These three are where Dinah's own write policy and path rules draw the same line. |
| 8 | Dinah offers no merge of a stock file. The choices are Keep Local, Save Incoming Beside It and Replace when unchanged. | Disagrees | The design allowed an explicitly reviewed merge. A merge needs an editor and judgement; the person can merge the saved copy in Docs. |
| 9 | No research limit is described as a cap. Template budgets map to team allowances and group turn budgets, which warn or count turns. | Goes further | The design asked the host to settle limit semantics. Dinah has none that stop spending, and saying so now prevents a hard-cap label later. |
| 10 | Research status words belong to the product; Dinah reports a step receipt and a chat state. | Goes further | The six states in the design are the product's journey. Dinah's operation states are about its own steps. |
| 11 | Removing a role in an update means retiring its chat, which keeps the history. | Goes further | The design said removal does not erase conversations. Retirement is the existing action with that behaviour. |
| 12 | A source is personal when `template.items` does not list it. | Goes further | The design required personal sources to stay personal without saying how Dinah knows. Absence from the record needs no flag. |
| 13 | Stock file content is inline text or an archive entry addressed by SHA-256. | Goes further | The design listed a content reference without a form. This form lets the digest cover every file. |
| 14 | A chat made by a template is found on retry by its task host allocation receipt, a group by a tag of operation and item, and a group is never matched by title. | Goes further | The design required no duplicates. Import matches groups by title, which would join a person's own group of the same title by mistake. |
| 15 | The operation journal is the existing task host, extended by a Dinah window service for the steps it lacks. | Goes further | The design asked for operation identifiers and durable step receipts without saying where they live. The task host already has them, and its design forbids a second allocation authority. |
