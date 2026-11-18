# Duplicating a Team

An agent that leads a team can copy the structure of a team in its own tree onto a new assignment: a new lead with the same members, each with the same remit, program, model and settings as its counterpart, but working on something else. Paul asked for this on 18 November 2026, for a customer lead that wanted a new concept team shaped like an existing one.

## What Already Worked

A chat could already start members under itself with `dinah.team/start@1`, with a title, a brief, and optionally a workspace, provider and model. A member started that way could start its own members, which made a nested team. `dinah.team/list@1` listed the members with their states and current tasks.

Three things were missing. A lead could not read a team's structure as data: the members' remits, models and settings were known only by asking each member, which wakes it and depends on the agent knowing its own model. A lead could start members only under itself, so a nested team had to be built by asking each new sub-lead to start its own members. And a started member always had its program's default settings, so a copy did not match the member it copied and asked the person for approvals the original did not.

## Decisions of 18 November 2026

Paul decided the following.

1. A member may be started with a permission mode no more permissive than the calling chat's own. A started chat still has its program's defaults unless the caller asks for a setting, so authority is never handed on unasked.
2. A chat may describe and copy only teams in its own tree: its own members, their members, and so on. It cannot read or build under another lead's team.
3. Creating a batch of agents is something the person approves once. The skill that duplicates a team asks the person, through Decisions, with the planned roster before it starts anyone. This is the skill's procedure; the program does not block a start, since team turn budgets already limit runaway work.

## The Actions

`dinah.team/describe@1` returns the calling chat's tree, or the subtree under one of its members, as data. Each member has its session ID, title, remit, program, model (the value and the name the program gives it), settings (permission mode, effort and the other options the chat was given), workspace, lead, state and whether it is retired. It reads and changes nothing, and needs no running member.

A member's remit is the brief it was started with. It is kept apart from the member's current task, which `dinah.team/assign@1` replaces, so later assignments do not overwrite what the member is for. For a member started before remits were kept, the remit is recovered from the brief its lead posted when it started it, when that message can be found.

`dinah.team/start@2` is `start@1` with three additions. `lead` is a chat in the caller's tree to start the member under; the caller itself when it is left out. `options` are settings for the new chat, such as its permission mode and effort. `remit` is kept as the member's remit; the brief when it is left out. `start@1` keeps working as it did.

A permission mode is accepted only when it is no more permissive than the caller's own. When the program does not document an order of its modes, or the new member runs a different program from the caller, the only modes accepted are the caller's own mode on the same program and the program's default. A refusal says which modes the caller may give. Other settings, such as effort, are not authority and are accepted as the program's option list allows.

## The Skill

A built-in skill, Duplicate a Team, gives a lead the procedure.

1. Describe the team to copy with `dinah.team/describe@1`.
2. Write the new roster: the new lead and each member, with its remit rewritten for the new assignment, and the same program, model and settings as its counterpart.
3. Ask the person once, with `ask_person`, showing the roster, and wait for the answer.
4. Start the new lead under the caller with `dinah.team/start@2`, then each member under the new lead.
5. Assign the first task with `dinah.team/assign@1`.

A product built from this source can point its own skills, such as a customer lead's, at this one.

## As Built

The two actions were built on 18 November 2026 in `cmd/dinah-desktop/team_duplication.go`, beside the existing team actions in `applet_team.go`. The Team applet's version is now 1.1.0. `dinah.team/describe@1` has the capability `chat.read` and is replayed as pure; `dinah.team/start@2` has `team.manage` and runs once, as `start@1` does.

### Describe

`describe@1` takes an optional `member`, the session ID of a chat in the caller's tree, and returns `{"team": …}` with that chat, or the caller, at the root. Each chat in it has `sessionId`, `busId`, `title`, `remit`, `remitSource`, `provider`, `model` (with `value` and `name`), `options`, `settings`, `workspace` (the folder), `workspaceName`, `lead` and `leadBusId`, `state`, `stateWhy`, `retired` and `members`. `options` holds the chat's settings other than its model, by setting ID, in the form `start@2` takes them. `settings` lists the same settings with the names and categories the program gives them, so a reader can see which one is the permission mode. The model is left out of both because it has its own field.

An open chat's program and settings come from the chat itself. A closed chat's come from what Dinah saved for it, read with the program's last list of options, which is what the chat gets back when it opens; the model's value follows a stored substitution as the Chats list does. Nothing is opened, woken or written.

### Remit

The remit is kept in a new column, `team_members.remit`, added to the bus database the way earlier columns were, so older rows read as empty. Both `start@1` and `start@2` write it in the same transaction that makes the chat a member; `start@1` keeps the brief, and its input and output are unchanged. `assign@1` does not touch the row. A member that the person moves to another lead keeps its remit, since the remit says what it is for. When a member replaces its lead, the former lead becomes a member with no remit, because it was never started as one.

`remitSource` is `kept` for a remit in the row. For an older member it is `start brief` when the remit was recovered, and empty when nothing was found. The recovered remit is the lead's first message that mentions the member, and it counts only when the newest line the lead left before it in the same group, saying it started a member, ends with that message's first line, as `start@1` writes it. It is worked out on every read and never written back, so `describe@1` stays a read.

### Starting under a chat in the tree

`start@2` checks that `lead` is in the caller's tree and not retired, before anything is started. The new chat's workspace, program and model default to the caller's, as `start@1`'s do. Its brief is posted by its own lead in that lead's team group, so the member's task, wake and relays work as they do for any member, and the line saying it was started reads "at the request of" the caller. The output is `start@1`'s with `lead` and `notApplied` added.

### The own-tree rule

A chat's tree is its members, their members, and so on; a chat is not in its own tree, and its lead is not either. `bus.InTree` decides it by walking up from the chat through its leads, and both new actions refuse a chat outside the tree with a reason. The existing actions already act only on the caller's direct members.

### The permission rule

Dinah has no documented order of any program's permission modes. The provider table records no such fact, and a program's option list gives its modes in the program's order with names but says nothing about which allows more. The rule is therefore the design's fallback for every program. A permission mode is a setting the protocol gives the category `mode`, the same test a workspace move uses to keep authority from following a chat. A new member may have the caller's own mode, when it runs the same program as the caller, or the program's default.

The program's default is the mode Dinah gives a new chat of that program as it is made: the person's saved default, or else the provider table's value, such as Manual for Claude. A requested mode that is neither the caller's own nor that default is refused before anything starts. When Dinah gives the program's new chats no mode, the default is the program's own, known only once the chat exists, so the mode is checked against the new chat's starting mode at that point. A mode that fails that check is not set; the member keeps its default and `notApplied` says why. Every refusal lists the modes that may be given, with their names.

Other settings, such as effort, are checked against the program's last option list before the start and given after it with the same call the window uses, so the chat's menus show them and they are saved with the chat. A setting the new chat refuses is listed in `notApplied` with the reason. A model among the options is refused and must be given as `model`.

### The skill

Dinah had no built-in skills before this one. A built-in skill is a folder with a `SKILL.md` in `cmd/dinah-desktop/builtin-skills`, embedded in the program. Built-in skills are listed in every skills catalogue with the scope `builtin` and the origin Built-in Skills. That includes the Skills pane, a chat's skill menu and `dinah.skills/list@1`, and `dinah.skills/read@1` returns the text. They are read only, and one cannot be run in a new chat of its own, since it needs a chat that already has its work. The skill is `duplicate-a-team`, with the procedure above; its step 3 asks the person once with `ask_person`, showing the roster, before anyone is started.

### Tests

`cmd/dinah-desktop/team_duplication_test.go` and `internal/bus/team_remit_test.go` cover the nested tree with remits, model values and names and settings, a closed member, refusals outside the tree, a start under a sub-lead, settings applied and saved, the permission rule both before and after the chat is made, the remit surviving assignments and moves, recovery from a start brief, `start@1` unchanged, and the skill listed for the person and for agents.
