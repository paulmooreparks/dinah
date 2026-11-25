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
| Plan before acting | `PlanWorkspaceImport` and `planLocal` in `workspace_export.go` list each source with what was found, calling each kind's `Check` with an 8 second limit. | A `PlanTemplate` call that returns the same kind of plan for a template, including files and roles. |
| Check results the person can act on | `Check` returns a single string, empty when the source can be used. | A result with a class (`ok`, `denied`, `missing`, `offline`, `no-connection`, `unchecked`) and a detail, so a denied page is never shown as a missing one. Kinds that only return a string are read as `unchecked` with that string as the detail. |
| Add a source without a duplicate | `ImportWorkspace` skips a source when `sameWorkspaceSource` finds one with the same kind and the same `Extra`; `attachWorkspaceSource` does the same for one source. | Matching by item ID through `template.items` first, then by `sameWorkspaceSource`, so a template source whose locator changed between revisions is updated in place and is not added again. |
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

The team machinery covers most of the shape, but every action that starts a member needs a calling chat, and the wizard has none (Table 10). The new work is a host operation that makes the lead and its team on the person's behalf, through the same internal path.

**Table 10** *Team Creation, Today and New*

| Need | Today | New |
| --- | --- | --- |
| Read a team's shape | `dinah.team/describe@1` in `team_duplication.go` returns remit, provider, model, options and settings for each chat in the caller's tree. | Authoring maps that output into Table 4 and drops what belongs to a running chat. |
| Make a chat with a title and a model, or none | `CreateNamedChat` in `named_chat.go` makes the chat, gives it its title and model, and deletes or hides it again when either fails. | The lead is made with `CreateNamedChat`, tagged with the operation and role ID. |
| Make a member with a remit under a lead | `teamStart` in `applet_team.go`, used by `dinah.team/start@1` and `start@2`, calls `bus.SetLeadWithRemit`, posts the start line and the brief in the lead's group. `teamCaller` refuses any invocation whose owner is not a chat. | A host function that runs the same steps with the person as the requester, so a member gets its remit and its place in the tree without a calling chat. `start@1` and `start@2` keep their own rule. |
| Make a chat a member without a remit | `SetTeamLead` in `team_views.go`, which is the person's own action. | Not used, since it writes no remit. |
| Settings and authority | `checkTeamOptions` and `applyTeamOptions` refuse a permission mode the caller does not hold and list settings that could not be given in `notApplied`. | Template options go through the same functions. With no caller, only the program's default mode is accepted, which the template's refusal of modes already ensures. |
| Groups | `bus.ImportGroup` makes a group with its turn budget or joins one of the same title; a team's group is made by setting its lead. | Template groups are made with `ImportGroup`, keyed by the operation and group ID rather than by title, so a group of the same title made by the person is not joined by accident. |
| Asking the person once | The `duplicate-a-team` built-in skill asks with `ask_person` before starting anyone. | The wizard's own review is that single approval. An agent that applies a template through an applet action asks with `ask_person` first, as the skill does. |
| Budgets | `SetTeamBudget` in `team_budget.go`. | Applied from `budgets` after the roles exist. |

*Note.* No action in the `dinah.workspace` applet today makes a workspace or attaches a source, and no action in `dinah.team` makes a top-level chat. A template applied by an agent would need new actions for both, with the capability `team.manage` and a new capability for workspace creation.

### Resumable Create and Update

Dinah has idempotent single steps but no record of a multi-step operation, so a crash halfway through a wizard today would leave no account of what was done (Table 11). The new piece is an operation journal whose every step can be retried without doing its work twice.

**Table 11** *Resumable Operations, Today and New*

| Need | Today | New |
| --- | --- | --- |
| One step done once | The applet broker gives an invocation a stable ID from the caller's `requestId` (`applet_mcp.go`); a retry with the same ID returns the stored result, and an interrupted `once` action is reported as interrupted and never re-run (`internal/applet/broker/broker.go`), with the advice to inspect the resource first. | Each journal step has its own natural key and a receipt, and a retry reconciles the receipt with what exists before acting. |
| Atomic state files | `internal/statefile` writes so that an interrupted write leaves the old file or the new one. | The journal is `operations/<id>.json` in the state folder, written through `statefile`. |
| Import twice without harm | `ImportWorkspace` adds only missing sources and pins; chats match by logical chat ID; group messages match through the bus table `message_origins`. | The same idea for every template item, keyed as below. |
| Plan identity | Nothing. | The operation records the template digest and a digest of the person's choices. A changed choice after any step has run starts a new operation that begins with reconciliation of the old one. |
| Cancel | Nothing. | Cancel stops before the next step and reports what was done. Nothing is rolled back. |

Each step's key decides whether the step's work already exists (Table 12). A step writes "started" to the journal before it acts and its receipt after, and a step found as "started" on resume is settled by looking for its key.

**Table 12** *How Each Step Is Recognised on Retry*

| Step | Key | Settled On Resume By |
| --- | --- | --- |
| Make the workspace | The main folder's path | `workspaces.IndexAt`, as import does. |
| Attach a source | Item ID, then kind and locator | `template.items`, then `sameWorkspaceSource`. |
| Make a directory | Relative path | Whether it exists. |
| Write a stock file | Relative path and SHA-256 | Whether the file has that SHA-256. |
| Make a role's chat | Operation ID and role ID | A tag recorded with the chat. |
| Make a group | Operation ID and group ID | A tag recorded on the group. |
| Send a first brief | Operation ID and role ID | A bus origin record for that pair, in the way `message_origins` records an imported message. |

*Note.* The chat tag must be written in the same update that records the new chat in Dinah's inventory. Whether `NewSessionWithProviderAt` offers such a point has to be confirmed before building. Where it does not, a step found as "started" with no tagged chat is reported to the person as possibly having made a chat, with the candidates listed, and is not repeated without their word.

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
| Start a chat's turn | `App.Send` in `session.go` sends a prompt to a chat and starts a turn. `CreateNamedChat` sends none. | The `start-brief` step sends the lead's brief with `Send` after the team exists. |
| Brief a member | `teamStart` posts the brief in the lead's group with a mention, which wakes the member. | Members whose role has a brief get it the same way when the step runs. |
| The words of the brief | The caller writes them. | The template's `brief` is the default. The apply call accepts a replacement per role, so the product can compose the research instruction, including the shared brief and its scope, without Dinah reading it. |
| Not starting twice | Nothing beyond the broker's per-invocation record. | The step's bus origin key in Table 12. A retried creation never sends a second brief, and an update has no `start-brief` step. |
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
| 6 | May the person, through a wizard, make a lead and its members with remits without a calling chat? | Yes | Raised by this draft. Without it, the lead can be made but members could only be started by the lead's own agent on its first turn. |
| 7 | Is a workspace the unit of source access? | Yes, for the product's wish to scope sources per agent | Raised by this draft. Every chat in a workspace reads all of its sources today, and agents' reads are unconfined with a deny list (docs/design/agent-file-access.md). The draft gives roles no source list. |

## Where This Draft Differs From the Product's Design

The product's design document asked for a neutral contract from Dinah and sketched one. This draft agrees with most of it, and the points where it disagrees or goes further are listed with their reasons (Table 16).

**Table 16** *Differences From the Product's Design*

| No. | Point | Kind | Reason |
| --- | --- | --- | --- |
| 1 | The brief, training and project scope are not template fields; they go in `extensions` under the product's identifier. | Disagrees | The agreed split gives the brief, training and scope to the product. Fields in Dinah's format would make Dinah responsible for their meaning. |
| 2 | The origin is an opaque record the product supplies, and the digest is one Dinah computes itself. | Disagrees | The sketch listed connector and resource fields in the origin, which tie the format to one kind of store. |
| 3 | Unknown fields inside a known major version are kept and ignored; an unknown major version is refused. | Goes further | The sketch said only that unknown versions fail. Keeping unknown fields is how workspace exports already stay compatible between builds. |
| 4 | The existing team actions cannot be used by a wizard, because they need a calling chat. | Disagrees | The design assumed the existing agent and team mechanisms suffice. `teamCaller` refuses every caller that is not a chat, so a new host function is needed (open question 6). |
| 5 | Roles carry no source list and no capability requirements. | Disagrees | Dinah has no per-agent source scope (open question 7). A field Dinah cannot enforce would be a promise it does not keep. |
| 6 | Permission modes are refused in a template. | Goes further | The design said a starter cannot expand authority. Refusing the setting outright makes that true without a comparison of modes, which no program documents. |
| 7 | `AGENTS.md`, hidden paths and local folder sources are refused. | Goes further | The design excluded executable instructions and machine paths. These three are where Dinah's own write policy and path rules draw the same line. |
| 8 | Dinah offers no merge of a stock file. The choices are Keep Local, Save Incoming Beside It and Replace when unchanged. | Disagrees | The design allowed an explicitly reviewed merge. A merge needs an editor and judgement; the person can merge the saved copy in Docs. |
| 9 | No research limit is described as a cap. Template budgets map to team allowances and group turn budgets, which warn or count turns. | Goes further | The design asked the host to settle limit semantics. Dinah has none that stop spending, and saying so now prevents a hard-cap label later. |
| 10 | Research status words belong to the product; Dinah reports a step receipt and a chat state. | Goes further | The six states in the design are the product's journey. Dinah's operation states are about its own steps. |
| 11 | Removing a role in an update means retiring its chat, which keeps the history. | Goes further | The design said removal does not erase conversations. Retirement is the existing action with that behaviour. |
| 12 | A source is personal when `template.items` does not list it. | Goes further | The design required personal sources to stay personal without saying how Dinah knows. Absence from the record needs no flag. |
| 13 | Stock file content is inline text or an archive entry addressed by SHA-256. | Goes further | The design listed a content reference without a form. This form lets the digest cover every file. |
| 14 | A chat or group made by a template is found on retry by a tag of operation and item, and a group is never matched by title. | Goes further | The design required no duplicates. Import matches groups by title, which would join a person's own group of the same title by mistake. |
