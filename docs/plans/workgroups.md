# Workgroups: One Running Dinah for Each

Paul found on 30 October 2026, after running several projects in one running Dinah, that it would be better to run them separately, each in its own instance of Dinah, with the bus bridging the instances so that chats in different instances can still talk to each other. He wants one window still able to hold several workspaces, just not as many as now. This document is the proposal. Nothing here is built.

## The Names

What an instance of Dinah opens, a collection of workspaces, is a **workgroup**. Paul first considered renaming a workspace to a project as well, and decided the same day to keep "workspace" for now, because that name is in contracts that agents and stored data depend on (the applet dinah.workspace, the bus's workspace fields, the page's methods and the saved state), and renaming them would reach far for little gain. Only the new term is added.

## Why

One running Dinah for everything has three costs that this week made plain. Restarting for a new build stops every project at once, so every team has to pause first. One taskbar carries every project's windows, which is most of the overflow. And a crash, a full context or a bad build in one project reaches all of them. With an instance for each workgroup, each restarts on its own, each has its own window and taskbar, and a failure stays in its own instance.

## The Workgroup

A workgroup is what an instance of Dinah opens. It holds the definition of its workspaces, the folders each one mounts and the settings each one overrides, and it keeps its own state: its windows and their layout, its chats and their saved histories, its queued messages, its notifications, its Judgements settings and records, and its recent documents. A workgroup may hold one workspace or several, so the Dinah workgroup can keep Dinah and its extension side by side, while Maize and PUDL each run in workgroups of their own.

A workgroup is kept in a folder of its own, by default under Dinah's home folder (`~/.dinah/workgroups/<name>`), with one file, `workgroup.json`, that describes it and can be copied or kept under version control, and the workgroup's state beside it. Opening a workgroup starts an instance of Dinah for it, from a list of recent workgroups at start, from the command line (`dinah open <workgroup>`), or by opening its `workgroup.json`. Two instances never open the same workgroup, and the second one to try brings the first one's window forward instead.

## What Is Shared Across Workgroups

Paul decided on 30 October that each workgroup keeps its own state, with only the bus and his provider sign-ins shared, and that the budget is shared while Judgements are kept per workgroup.

- **The bus** is shared across the machine, so a chat in one workgroup can mention, ask or join a group chat with a chat in another, as chats in different workspaces can now. The bus is already a SQLite file that several programs open at once (the window and each chat's bus server process do today), which SQLite documents as safe, so sharing it across instances needs no new transport on one machine. Every identity on the bus gains the workgroup it belongs to, so a title is unique within a workgroup and a chat in another workgroup is named with its workgroup.
- **Provider sign-ins** are the providers' own and already apply to the whole user account.
- **The budget** for a provider is shared, because it is the same plan's usage whichever workgroup spends it. Each workgroup's spending is counted against the one shared allowance, and the widget shows each workgroup's share.
- **Judgements** are per workgroup, each with its own model, settings and records, since what a judge decides is about that workgroup's chats and documents.

## Costs

Each instance starts its own provider processes, so two workgroups each with three open Claude chats run six agents' worth of processes, at about 270 MB each. Settings that are the person's own, such as appearance and the default provider, are kept once in Dinah's home folder and read by every workgroup, and a workgroup overrides only what it sets.

## Moving to Workgroups

The existing single running Dinah becomes the first workgroup, holding all of today's workspaces, so nothing is lost on the day workgroups arrive. Paul can then move a workspace out into a new workgroup, which carries its chats, their histories and its windows, and leaves the chats it talks to on the bus as they are.

## Remote Control

Paul wants to control Dinah remotely, as he misses doing with Claude Code. The bus is the right boundary for it: everything a chat says, every message to Paul and every answer he gives already travels over the bus, so a remote client that can read and write the bus can follow and steer every workgroup without being one of Dinah's windows. This design keeps that open by making the bus the one place where workgroups meet, with no workgroup reaching into another's state, so that a later network bridge or an authenticated remote client sits on the bus alone. Remote control is not part of this design, and it will need its own on authentication, transport and what a remote client may do.

## Relation to Teams

A lead is usually the lead of one workgroup's work, as Dinah Architecture is Dinah's, and a team usually lives within one workgroup (`teams.md`). A team member in another workgroup still works, through the shared bus, for a team that spans workgroups.
