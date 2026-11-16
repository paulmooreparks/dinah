# Team hierarchy

This document records the decisions of 17 November 2026 about how chats report to one another. Nothing in it is built yet. The Org Chart change needs a clickable mockup that the person approves before any product code.

## Two layers

Teams and group chats are separate things, and both stay.

A team is hierarchy. One or more chats report to a lead, and the lead reports to another lead or to the person. A group chat imposes no hierarchy. Any chat can be in any number of group chats whatever its place in a team, and the person uses them as cross-functional rooms where leads and members of different teams share information and coordinate.

The bus already keeps the two apart. A team room is a group whose record has a lead (`rooms.lead`), and an ordinary group chat has none. Every rule below that speaks of a team room applies to such a group only.

## One relation

Every chat reports to exactly one thing: a lead, or the person. From that one relation a chat is one of three things, and can be the first two at once:

- A lead has at least one chat that reports to it.
- A member reports to a lead.
- A free agent reports only to the person.

There are no levels and no kinds of lead. A lead whose reports are themselves leads is what an overview of a larger piece of work looks like, and a lead that reports straight to the person is allowed beside a deeper branch. The tree can have any depth, and two branches need not have the same shape. Examples such as one lead for each customer with a lead for each piece of work under it are one use of this, and the design must not be shaped around any one use.

The bus stores this already, as one row for each member with its lead (`team_members`). Two refusals stand in the way and are to be removed (`checkTeamLead` in `internal/bus/teams.go`): a chat that leads a team cannot be made a member of another, and a chat that belongs to a team cannot be made a lead. The refusal of a cycle stays. Existing teams are valid trees one level deep, so no saved state is converted.

## Team rooms

Every lead has one team room, with its direct reports and nobody else. A lead two levels up is not in it.

Communication follows the chain of command. A member speaks with its lead and the others in its team room, and a lead speaks with its own lead in that lead's room. A chat that needs to reach across the chain does so in a group chat, which is what group chats are for.

A chat that leaves its team room, or is taken out of it by anyone, is no longer a member of that team (built on 16 November 2026, `leaveTeamOfGroup`). Leaving an ordinary group chat changes nothing about who reports to whom. A lead with reports that can still be reassigned cannot be deleted; a lead whose reports are all in the Recycle Bin can.

## Questions for the person

A member's question for the person goes to its lead first, as it does today. With depth it rises one lead at a time, and each lead answers it or passes it up.

## Turn allowances

An allowance is set for a lead. The turns of a team count against the allowance of its lead and against the allowance of every lead above it, so one number caps a whole branch.

## Grouping on the chart

The Org Chart groups chats by workspace, and a branch of the tree is drawn inside its workspace's box. A piece of work is one workspace in practice today. The person has said that may change, so the grouping must not be written into the reporting relation: who reports to whom is independent of which box a chat is drawn in.

## Open decisions

A chat that reports to a lead has no input box today, and the person reaches it only through its team room. The person is inclined to give every chat its input box back while keeping a room for each team. Two things are undecided:

1. What the lead learns when the person speaks to its member directly: nothing, one line in the team room saying that it happened, or the message itself. The recommendation is the one line, which keeps the lead's overview without making every quick question a turn for the lead.
2. Whether the member's answer stays in that chat only. The recommendation is that it does.

## What is to be built

- The two refusals removed, with the cycle refusal kept and tested at depth.
- A team room for every lead at every depth, with direct reports only.
- Allowances counted up the chain.
- The Org Chart drawn as a tree of any depth inside each workspace's box, after its mockup is approved.
- The input box, once the two open decisions are made.
