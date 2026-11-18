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
