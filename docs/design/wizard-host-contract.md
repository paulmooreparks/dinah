# Wizard Host Contract

This document is the design of the host calls that a linear set-up wizard uses to create a workspace, its files and its team of agents on the person's authority. Paul asked for it through Software Director on 28 November 2026. It covers two mechanisms: creating a team when no chat is calling, and a durable record of a creation operation that Resume and Stop can rely on. It also describes a plain Dinah workspace wizard built on the same calls. Nothing here is built, and no product code follows from this document.

The template format itself stays in docs/design/workspace-template-contract.md. This document replaces that draft's sections on team creation and resumable operations with a concrete call surface, and the last section lists what changes in it.

Any wizard screen, whether Dinah's own or a downstream product's, needs a PUDL mockup that Paul approves before it is built. This document decides no screen, layout or wording.

## Rules the Contract Keeps

The contract keeps a short set of rules, and every call below is checked against them (Table 1).

**Table 1** *Rules Every Call Keeps*

| Rule | How the contract keeps it |
| --- | --- |
| No reliance on undocumented behaviour | A chat's existence is settled only by a documented success boundary of the agent protocol (docs/design/recorded-session-allocation.md). When the outcome cannot be known, the item is recorded as unknown and waits for the person. |
| The person's word before any write | Only a confirmation made on Dinah's trusted page path starts or resumes an operation. A caller's copy of a plan is never authority. |
| Nothing written to an external service | Dinah writes local settings, local files and its own bus database. Publishing anything anywhere is the caller's business and needs its own confirmation. |
| Deterministic import | Planning and applying read only the package, the person's choices and the computer's current state. No model is asked anything during an operation. |
| No chat histories or credentials | The template rules of docs/design/workspace-template-contract.md (its Table 5) apply unchanged. |
| No overwrite by default | A stock file is written only with an exclusive create. Replacing a file needs the person's choice and a matching digest at the moment of writing. |
| Illegal actions are absent | The operation's representation lists only the actions that are legal now, each with a token bound to the record's revision, as `OrgChartActions` already does (`org_chart_actions.go`). |

## Mechanism 1: Creating a Team With No Calling Chat

### What Exists Today

Dinah can already make members under a lead, record a remit and walk a tree of any depth, but only an open chat can ask for it (Table 2). The gap is an entry point whose authority is the person and that leaves the new chats idle.

**Table 2** *Team Creation Code on Master*

| Piece | Where | What it does today | Use in this contract |
| --- | --- | --- | --- |
| `teamCaller` | `cmd/dinah-desktop/applet_team.go:74` | Refuses any invocation whose owner is not an open chat on the bus. Every `dinah.team` action starts with it. | Not used. The team actions keep their rule. |
| `teamStart` | `applet_team.go:128` | Requires a title and a brief, starts the chat with `NewSessionWithProviderAt`, calls `bus.SetLeadWithRemit`, writes a start line and posts the brief with `SendMentioning`, which wakes the member. | Its bus step is reused. Its start path and brief are not, since they keep no operation record and always start a turn. |
| `followModel` | `applet_team.go:221` | Sets a model only when the program offers it, and otherwise leaves the program's default with no report. | Not used, because the wizard must never substitute silently. |
| `checkTeamOptions`, `applyTeamOptions`, `permittedModes` | `team_duplication.go:318-420` | Check settings against the program's saved option list and list refusals in `notApplied`. All three take the calling chat's session. | A variant with no caller, which refuses every `mode` setting outright. |
| `bus.SetLeadWithRemit`, `checkTeamLead`, `InTree` | `internal/bus/teams.go:94, 260, 65` | Make a chat a member with its remit in one transaction, refuse a cycle, and allow a tree of any depth. A lead's team room is made with its first member. | Used as they are. |
| `newChatDefaults` and the lifecycle's `Defaults` hook | `session_lifecycle.go:83`, `internal/sessionhost/session_lifecycle.go:289` | Give every new chat the person's saved defaults for its program, then the provider row's own. | This is how "each program's defaults" are applied. Nothing new is needed. |
| `SessionLifecycle.NewRecorded` | `internal/sessionhost/session_lifecycle.go:191` | Starts a fresh session for an operation whose intent is already durable, and reports dispatch, the provider's session ID and setup completion through callbacks. | The only way a role's chat is made. |
| Task host `OpenChat`, `UseModel` | `task_host_operations.go:63` | Opens a chat through `NewRecorded` with a source digest of service, workspace, task and operation IDs, and keeps the chat's runtime open for the task (`holdTaskChat`). | Used for every role. |
| `bus.ImportGroup` | `internal/bus/imported.go:208` | Joins a group of the same title or makes one. | Not used, since a person's own group with the same title would be joined by mistake. |
| `SetTeamBudget` | `team_budget.go:187` | Sets a lead's allowance, which warns and stops nothing. | Used for template budgets. |

### What Is New

The new host function is `CreateTeam`, which runs only as steps inside an operation (Mechanism 2), so that a team has the same record, receipts, Stop and Resume as everything else. A wizard that only wants a team in an existing workspace starts an operation of kind `team`.

The person is the requester. Each role's chat is made through the task host's `OpenChat`, so its allocation receipt is the role-to-chat receipt, and the program's defaults come from the lifecycle hook that every new chat already uses. The chat is then given its model when the role asks for one, given its other settings, and placed in the tree with `SetLeadWithRemit`. No message is posted to any group and no turn is started, so every new chat is idle until someone gives it work. A protocol `session/new` request creates a session without a prompt, which is the documented boundary this relies on.

The top role reports to the person, so it has no `team_members` row. A lead-only team is therefore a single chat with no team room, and an empty team section makes no chats at all. A role whose `lead` is another member becomes a nested lead, which `checkTeamLead` already allows at any depth.

### Validation Before Anything Is Made

The plan validates the whole team before the person confirms, and any failure blocks the team items while leaving the rest of the plan intact (Table 3). An explicitly chosen "no team" never needs a program.

**Table 3** *Team Validation Checks*

| Check | Error code |
| --- | --- |
| Role IDs are unique and follow the template's ID rule. | `duplicate_role` |
| Exactly one role has no `lead`, and it is the template's `lead`. | `no_lead`, `two_leads` |
| Every `lead` refers to a selected role. A role whose lead the person left out is refused until its lead is selected or it is moved under another selected role. | `unknown_lead` |
| The relation has no cycle. | `cycle` |
| Each selected role's program is offered by this build. | `program_unavailable` |
| A requested model is in the program's last saved option list. | `model_unavailable` |
| No setting has the category `mode`. | `mode_setting` |
| Each other setting is in the program's last saved list. A program with no saved list is marked `unchecked`, and the setting is tried after the chat exists. | `setting_unavailable` or `unchecked` |
| Each group's members are selected roles, and its title is not empty. | `unknown_member`, `empty_title` |

*Note.* "Offered" means `product.Offers`, as `checkTaskProgram` reports it (`task_host_inspection.go:47`). Dinah has no documented way to know before allocation that a program is installed and signed in, so that failure surfaces as the role's own item outcome.

### Role-to-Chat Receipts and Idempotency

Each role produces one receipt in the operation record, and the receipt is what makes a retry safe (Table 4). The task host operation ID for a role is `role:<roleID>`, inside a task whose ID is the operation ID, so the allocation source digest is stable for the life of the operation.

**Table 4** *Fields of a Role Receipt*

| Field | Meaning |
| --- | --- |
| `role` | The role ID from the plan. |
| `state` | `pending`, `running`, `succeeded`, `failed`, `unknown`, `declined` or `skipped`. |
| `chat` | The canonical chat ID, once the provider has returned it. |
| `busId` | The chat's bus identity, once it has one. |
| `lead` | The role ID and chat ID of its lead, or `person`. |
| `teamRoom` | The key of the team room it joined, when it reports to a role. |
| `effective` | The program, model and settings the chat actually has, read back after the steps ran. |
| `notApplied` | Each requested model or setting the chat refused, with the reason. A refused model is listed here and never replaced quietly by another one. |
| `allocation` | The task host receipt ID for `role:<roleID>`. |

A retry of a role never allocates a second chat. A receipt that is `succeeded` is left alone. A receipt that is `running` or `unknown` is settled first through the task host's own receipt, and only a receipt that proves no session was made (`AllocationNotCreatedError`) lets the step allocate again. Any other unknown outcome waits for the person, who either picks the chat that was made from a list of chats in the workspace made after the intent was written, or says that no chat was made, or leaves the role out.

## Mechanism 2: The Durable Operation Record

### What Exists Today

Dinah has a durable journal for chats, turns and actions, but nothing records workspace creation, sources or files, and the journal is not switched on in Dinah's own build (Table 5).

**Table 5** *Operation Code on Master*

| Piece | Where | What it does today | Use in this contract |
| --- | --- | --- | --- |
| Task host window | `internal/taskhost/window.go` | Persists an intent before native work, keeps typed receipts, refuses a reused operation ID for different work, blocks renewed execution while a receipt is `prepared` or `in_progress`, and marks such receipts `unknown` at shutdown. A review allows at most 256 operations, expires after 5 minutes, and binds the task to a deadline within 24 hours. | Owns every chat step. |
| Task scope | `task_host.go:207` | Resolves a task's workspace by an existing workspace ID. | The task for the team steps begins only after the workspace exists. |
| Window services | `internal/services/window.go:76-80` | `RegisterWindow` and `RegisterWindowNamed` exist. Only tests call them, so Dinah's build has no window service. | Dinah registers one service, `dinah.workspace-setup`. |
| Room operations | `task_host.go:46` | Refused, because the desktop sets no room adapter. | Not used. Groups use a new bus call. |
| `statefile` | `internal/statefile` | Writes a file so that an interruption leaves the old or the new content. | Stores the operation record. |
| `ImportWorkspace`, `PlanWorkspaceImport` | `workspace_export.go:239, 341` | Plan and import an export, add only missing sources and pins, and find a workspace by its main folder with `workspaces.IndexAt`. | Its record creation and source matching are reused. |
| `localfs.Folder` | `internal/resource/localfs` | `MakeFolder` and `CreateFile` (exclusive) inside an `os.Root`, `WriteText` with an expected revision, and `CleanPath`. | Every directory and stock file step. |
| `workspaces.Record` | `internal/workspaces/workspaces.go:53` | Has no field for an operation or an applied template, and no `Extra` map. | Gains two fields (Table 8). |

### What Is New

The operation record is a JSON file at `<configDir>/operations/<operationID>.json`, written through `statefile`. It is the one place that says what an operation intended and what it did. The task host keeps the receipts for chat steps, and the record keeps their receipt IDs beside its own receipts for every other step, so there is still one receipt owner for each kind of step (Table 6).

**Table 6** *Fields of an Operation Record*

| Field | Meaning |
| --- | --- |
| `id` | The operation ID the caller chose. It is the idempotency key for the whole operation. |
| `kind` | `create`, `team` or `update`. |
| `rev` | An integer that grows with every write. Every transition is a compare-and-set on it. |
| `state` | `running`, `stopping`, `stopped`, `needs-attention` or `finished`. |
| `target` | The main folder path for `create`, or the workspace ID for `team` and `update`. |
| `workspace` | The workspace ID, once the workspace exists. |
| `template` | The template's `id`, `revision` and `digest`, and the caller's opaque origin record of at most 4 KiB. Empty when the person started with no template. |
| `plans` | Each confirmed plan revision in order, with its digest, the person's choices and the confirmation's time. A Resume with changed choices adds a revision. |
| `items` | One entry for each source, directory, stock file, role, group and budget, with its item ID, plan revision, key, precondition, state, produced resource and classified error. |
| `confirmedBy` | `person`, and the trusted page path's confirmation receipt. |
| `started`, `updated`, `finished` | Times. |

*Note.* The record never contains a token, a password, a chat transcript or a source's content. A classified error contains a code and Dinah's own reason text.

Only one operation may run against a target at a time. A second `Begin` for the same main folder or workspace is refused with `busy` while the first is `running`, `stopping` or `needs-attention`.

### The Order of Steps

An operation runs its steps in a fixed order, and each step writes its intent before it acts and its receipt after (Table 7). The order puts the cheapest steps to undo by hand first and the chats last.

**Table 7** *Steps, Keys and How Each Is Settled on Resume*

| Order | Step | Key | Settled on resume by |
| --- | --- | --- | --- |
| 1 | Preflight | The plan digest | Re-reading every precondition. A changed one stops the operation before any write. |
| 2 | Register the workspace (`create` only) | The main folder path | `workspaces.IndexAt` and the record's `createdBy` field, written in the same settings write as the record itself. |
| 3 | Attach a source | Item ID, then kind and locator | The workspace record's `template.items`, then `sameWorkspaceSource`. |
| 4 | Make a directory | Relative path | Whether it exists inside the main folder's `os.Root`. |
| 5 | Write a stock file | Relative path and SHA-256 | Whether the file there has that SHA-256. |
| 6 | Make a role's chat | Operation ID and `role:<roleID>` | The task host receipt. |
| 7 | Place a role in the tree | Member chat and lead chat | `bus.LeadOf` returning the planned lead, and the stored remit. |
| 8 | Make a group | Operation ID and group item ID | A new bus table `room_origins` written in the same transaction as the room. |
| 9 | Set an allowance | Lead chat | Reading the allowance back. |
| 10 | Record the applied template | Operation ID | The workspace record's `template.operation`. |

*Note.* The workspace step creates the main folder only when the person chose Create It Now, as import does today. Whether a synchronising service will later bring a file to a path is not something a documented interface can tell Dinah, so the plan treats only what is on disk at apply time as true.

The chat steps run under a task host task whose scope is the new workspace. That task can begin only after step 2 has a receipt, while the person confirmed the whole plan before step 1. The service's `Confirm` hook therefore accepts the team steps' review only when every permit in it appears in the plan the person already confirmed for this operation. Question 1 asks Paul to approve that rule.

### Stock Files and Conflicts

A stock file is classified in the plan and checked again at the moment of writing. A file that exists is never replaced unless the person chose to replace it and its SHA-256 still matches the plan.

- `new`: written with `CreateFile`, which fails when a file has appeared since the plan.
- `same`: already present with the same bytes, and recorded as applied without a write.
- `conflict`: a different file is there. The default is Keep Local. The person may choose Save Incoming Beside It, written with `CreateFile` to `<name>.r<revision><ext>`, and a file already at that name is reported as a further conflict.
- `unchanged-since-applied` (update only): replaced only by the person's choice, through `WriteText` with the expected revision.
- `edited-locally` (update only): Keep Local or Save Incoming Beside It.

### Stop, Resume and Finish

Stop closes admission, cancels any running task host operation and waits until every step in flight has a receipt. Its report lists each item as done, not done or unknown, and it never claims that anything was undone. Nothing is rolled back.

Resume first reconciles every `running` and `unknown` item by its key (Table 7). It then plans the remaining items against the computer as it is now, and the person confirms that remaining plan on the trusted page path. Unknown chat steps that cannot be settled stay in `needs-attention` until the person settles them, and nothing after them runs. A restart between steps behaves the same way, since the task host marks interrupted receipts `unknown`.

An operation finishes when every required selected item has succeeded. Declined and skipped optional items are kept in the record with their reasons, and an operation with an optional failure finishes only on the person's word through `finish-without`.

## The Calls

### Call Surface

The wizard makes seven calls, all on the `App` and bound to the page (Table 8). A downstream product built from this source calls the same Go methods. Every call that writes needs a confirmation made on Dinah's trusted page path, so an agent can plan but cannot begin.

**Table 8** *Calls the Wizard Makes*

| Call | Input | Output | Idempotency |
| --- | --- | --- | --- |
| `PlanWorkspaceOperation` | `kind`; `target`; optional template package bytes and origin; `choices` (per item: include, conflict choice, program and model for a role); `operation` when re-planning an existing one | A plan with its `digest`, each item's classification and check result, team validation, blockers and the legal actions | A pure read. The same input on the same computer state gives the same digest. |
| `BeginWorkspaceOperation` | `operation` ID, `planDigest`, confirmation token | The operation's representation (Table 9) | Same ID and digest returns the existing operation. Same ID with another digest is refused with `operation_reused`. |
| `ReadWorkspaceOperation` | `operation` ID | The representation | A read. |
| `ListWorkspaceOperations` | Optional `target` | Each operation's ID, kind, target, state and time | A read, used after a restart to find unfinished work. |
| `StopWorkspaceOperation` | `operation` ID, action token | The representation with the Stop report | Stopping a stopped operation returns it unchanged. |
| `ResumeWorkspaceOperation` | `operation` ID, the remaining plan's `planDigest`, confirmation token | The representation | Same digest twice returns the running operation. |
| `SettleOperationItem` | `operation` ID, item ID, `resolution` (`use-chat` with a chat ID, `not-made`, `leave-out`), action token | The representation | The first settlement wins. A second is refused with `stale`. |

*Note.* A digest is the SHA-256 of the plan's JSON as Go's `encoding/json` writes it, which sorts map keys and fixes field order by the struct. Templates keep their own digest, the SHA-256 of `template.json` as stored.

### The Representation

Every call except the list returns the same representation of an operation (Table 9). The actions array contains only what is legal now. Resume is absent while a chat step is unsettled, Stop is absent once the operation has stopped, and so on.

**Table 9** *The Operation Representation*

| Field | Content |
| --- | --- |
| `id`, `kind`, `state`, `rev`, `target`, `workspace`, `template` | As in Table 6. |
| `items` | Each item with its state, produced resource, `notApplied` list and classified error. |
| `roles` | The role receipts of Table 4, keyed by role ID. |
| `summary` | Counts of done, not done, unknown and declined items. |
| `actions` | Each legal action (`stop`, `resume`, `settle`, `finish-without`) with a token bound to `rev`, and for `settle` the item and its offered resolutions. |

### Errors

Every error has a stable code and a reason the person can read (Table 10). A plan reports item-level errors in the plan and fails as a call only when the package itself cannot be read.

**Table 10** *Error Codes*

| Code | Raised by | Meaning |
| --- | --- | --- |
| `invalid_template` | Plan | The package breaks a format rule. |
| `unknown_version` | Plan | The format's major version is not known here. |
| `reused_revision` | Plan | Another package with the same `id` and `revision` was applied with a different digest. |
| `forbidden_content` | Plan | The package contains something in the template contract's Table 5. |
| Team codes of Table 3 | Plan | The team cannot be made as chosen. |
| `path_refused` | Plan | A path fails `CleanPath` or the write policy. |
| `folder_in_use` | Plan, Begin | The main folder already belongs to a workspace. Re-applying to it is an `update`. |
| `stale` | Begin, Resume, Settle | The plan or the token no longer matches the computer or the record. |
| `operation_reused` | Begin | The operation ID was used for a different plan. |
| `busy` | Begin | Another operation is running against the same target. |
| `not_authorized` | Begin, Resume | No confirmation from the trusted page path. |
| `unknown_outcome` | Resume | An unsettled item blocks further work. |

## A Plain Dinah Workspace Wizard

Dinah itself can offer a workspace wizard on the same calls, which is also the proof that the contract does not depend on any downstream product. Dinah's build registers no source kinds, so in Dinah alone a template's sources show as Not Available Here and its useful parts are directories, stock files and a team. Each step maps to calls already listed (Table 11).

**Table 11** *Steps of the Plain Wizard and the Calls Behind Them*

| Step | What the person does | Calls |
| --- | --- | --- |
| 1 Start | Choose a template file, or start empty. | `PlanWorkspaceOperation` with or without a package. |
| 2 Sources | Review the template's sources and their check results. | `PlanWorkspaceOperation` again with the choices. |
| 3 Your Own Sources | Add sources of their own, including local folders. These are plan items with no template item ID, so they stay personal. | The same. |
| 4 Folder | Choose the main folder, and whether to create it. | The same. |
| 5 Team | Keep, leave out or add roles, and choose an offered program and model for each. | The same. |
| 6 Review and Create | Confirm the plan. Progress, Stop and Resume stay on this step. | `BeginWorkspaceOperation`, `ReadWorkspaceOperation`, `StopWorkspaceOperation`, `ResumeWorkspaceOperation`, `SettleOperationItem`. |
| 7 Done | See what was made and what was left out. | `ReadWorkspaceOperation`. |

Re-applying a template later is an operation of kind `update`. Its plan compares the applied template kept under `templates/<digest>.json`, the new package and the workspace as it is now, and lists the changes in the categories of the template contract's Table 13. The person chooses each change, and nothing in their own work changes without a choice. An update never deletes a chat, a conversation or a file, and it never starts a turn.

## Changes to the Workspace Template Contract

The template draft of 25 November needs these edits once Paul accepts this document (Table 12).

**Table 12** *Edits to docs/design/workspace-template-contract.md*

| Section | Edit |
| --- | --- |
| Team Creation, Table 10, Groups row | Replace `ImportGroup` with a new bus call that makes a room and a `room_origins` row in one transaction. `ImportGroup` matches by title and the `rooms` table has no column for a tag. |
| Team Creation, Table 10 | Point to this document's Mechanism 1 for the host function, the validation and the receipts. |
| The Team, Table 4 | Say that a member's `lead` may be any role, which makes nested teams, and that an absent team section or a lead with no members is valid. |
| The Team, model | Say that a requested model the program does not offer blocks the role at plan time and is reported in `notApplied` if it disappears before creation. |
| The Lead's First Research Turn | Move out of the first build. Creation and update never start a turn, and Table 12's two brief rows go with it. A first turn becomes a separate operation later. |
| Resumable Create and Update | Point to this document's Mechanism 2, and add the confirmation rule across the workspace step (question 1). |
| Record of an Applied Template, Table 6 | Add `createdBy`, the operation ID, written in the same settings write that registers the workspace. |
| Host Integration Points, Folder | Note that Replace uses `WriteText` and so works for text files only until a byte-level compare-and-set exists. |
| Questions Open With Paul, question 2 | Mark it as answered by this design for the first build, subject to question 3 below. |

## Open Questions

1. May the setup service's `Confirm` hook accept the team steps' task host review on the strength of the person's earlier confirmation of the whole plan, when every permit is in that plan? I recommend yes. The alternative is a second confirmation in the middle of an operation, which the person would see as a repeated question about something already approved.
2. Should Replace of a stock file work for binary files in the first build? I recommend no. `WriteText` is the only compare-and-set write Dinah has, and Save Incoming Beside It covers binary files safely.
3. Is it settled that creation never starts the lead's first turn? I recommend yes, matching the downstream proposal that defers all research. The format keeps the `brief` field for a later operation.
4. Should the person be able to create a workspace on a main folder that already belongs to one? I recommend no. That case is a re-apply, which has its own plan and change list, and refusing it keeps `create` free of merging rules.
5. Should the plain Dinah wizard be built in the first increment, or only the calls? I recommend building the calls with tests first and the plain wizard next, after its mockup is approved, because the calls alone are enough for a downstream wizard to proceed.

## What in the Code Disagrees With This Design

Several things in the code or the documents disagree with the design, or with each other, and the build has to deal with each (Table 13).

**Table 13** *Disagreements Found While Reading the Code*

| Where | What disagrees |
| --- | --- |
| `followModel`, `applet_team.go:221` | A model the program does not offer is dropped with no report, and `start@2`'s `notApplied` does not include the model. That is a silent substitution in the existing team actions too. |
| `teamStart`, `applet_team.go:131-210` | A member cannot be made without a brief, and the brief always wakes it, so there is no idle creation path today. |
| `ImportGroup`, `internal/bus/imported.go:208`, and `rooms` in `internal/bus/store.go:149` | The template draft says groups are made with `ImportGroup` keyed by operation. `ImportGroup` matches by title and a room has nowhere to keep a key. |
| `checkTaskProgram`, `task_host_inspection.go:47` | It reports only whether this build offers a program. It cannot say whether the program is installed or signed in, so a plan can pass and the chat step still fail. |
| `taskWorkspace`, `task_host.go:207` | The task host needs a workspace that exists, so it cannot cover the workspace step, and a single task host review cannot cover a whole `create`. |
| Task host review limits, `internal/taskhost/window.go:111-160` | At most 256 operations, a 5 minute review and a deadline within 24 hours. A team of more than about 120 roles, each needing two operations, would need more than one review, and a Resume after a day needs a new review. |
| `internal/services/window.go` | No production code registers a window service, so the setup service would be Dinah's first. |
| docs/design/org-chart-team-lead-change.md | It says a current lead cannot be assigned to another team. `checkTeamLead`, the comment in `org_chart_actions.go:230` and docs/design/team-hierarchy.md all allow any depth. The wizard builds top-down and never moves a current lead, so this does not block it, but the document is out of date. |
| `workspaces.Record` | No `Extra` map and no field for an operation or an applied template, so two fields are added and older builds will drop them on a settings write. |
