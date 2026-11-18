---
description: Duplicate a Team. Copy the structure of a team in your own tree onto a new assignment, with a new lead and the same members, each with the same program, model and settings as its counterpart. The person approves the roster once before anyone is started.
argument-hint: the team to copy and the new assignment
---

# Duplicate a Team

Use this skill when you lead a team, directly or through your members, and you are asked to set up a new team shaped like one you already have. You can copy only a team in your own tree, which is your own members, their members, and so on.

The new team has a new lead and one member for each member of the team you copy. Each new member has the same program, model and settings as its counterpart, and a remit rewritten for the new assignment.

## 1. Describe the Team to Copy

Call the applet action `dinah.team/describe@1`. Give `member` the session ID of the lead of the team you copy, or leave it out to read your whole tree and find the team there.

The result has the team's lead and its members, nested under their leads. For each chat, read its `title`, `remit`, `provider`, `model.value`, `options` and `workspace`. The `settings` list gives the same settings with their names; a setting whose category is `mode` is a permission mode. When `remitSource` is empty, the member's remit is not known, and you write it from its title and its current task instead.

## 2. Write the New Roster

Write the new lead and each new member as a list. For each one, give:

- its title, made for the new assignment;
- its remit, rewritten for the new assignment from its counterpart's;
- its program, model, settings and workspace, copied from its counterpart;
- the chat it reports to in the new team.

Keep the shape of the team you copy. A member that leads members of its own there leads the copies of them here.

For example, suppose you are Harbor, and your member Checkout leads a team with a member called Docks. To copy that team onto a new assignment, Keyboard, you start Keyboard under yourself and Docks Keyboard under Keyboard. Docks Keyboard's remit says what Docks's remit said, written for the Keyboard assignment.

## 3. Ask the Person Once

Before you start anyone, ask the person with `ask_person`, showing the roster: every chat you will start, with its title, remit, program, model, permission mode and the chat it reports to. Recommend starting the roster as written, and ask whether to change anything.

Wait for the answer. Start no one until the person has approved the roster. When the person changes it, use the changed roster. When the person declines, stop and say so in your chat.

## 4. Start the New Team

Start the new lead under yourself with `dinah.team/start@2`. Give it `title`, `brief`, `remit`, `provider`, `model`, `workspace` and `options` from the roster. The brief is the new lead's first message; it can say what the team is for and that its members are being started.

Then start each member under the chat it reports to, with `lead` set to that chat's session ID, which `dinah.team/start@2` returned when it started it. Start a member only after the chat it reports to has been started.

A permission mode is accepted only when it is your own mode on the same program or the program's default. When `dinah.team/start@2` refuses a mode, it says which modes you may give: choose one of them or leave the mode out, and tell the person which member differs from its counterpart and why. When a result lists settings in `notApplied`, tell the person which settings the member does not have.

## 5. Assign the First Task

Give the new lead its first task with `dinah.team/assign@1`. It can then assign work to its own members.

Tell the person in your chat which chats you started, under whom, and anything that differs from the approved roster.
