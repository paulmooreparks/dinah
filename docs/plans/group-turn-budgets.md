# Group Turn Budgets in Place of the Hop Limit

This is the shape of the second item in [Group Chat Work After the Cleanup](group-chat.md), written by Dinah Engine Cleanup on 29 October 2026 at Architecture's request, before any code. It answers the four questions Architecture asked: where the budget is stored, how a group's delivery meets the hop count today and how it would after this change, what happens to a mention once the budget is spent, and when the workspace's link to itself can go. The DESIGN.md changes are made in the same commit and are listed at the end. No code for the budget exists.

## What the code does today

A message to a group goes through the same path as any message. `insertMessage` gives it a hop count one past the furthest message its sender had read, and one delivery per member chat is made. Nothing in that path treats a group differently. The difference is only in who is asked to answer: a group's message wakes a chat only if it mentions it (`wakesSQL`).

The hop limit is applied where chats are woken. `Doorbell` (used by the window's `Ring` and `AfterTurn`, and by the terminal helper) and `Waking` (which opens closed chats) both count a delivery only when the message's hop count is at most the limit between the sender's workspace and the recipient's, which is three unless a trusted link between those two workspaces says otherwise (`hopLimitSQL`). A message past the limit is still delivered, but only when the chat next takes a turn for another reason, and the person speaking in the chat, or a message from the person reaching it, starts the count again (`Deliver`).

So a group does not bypass the hop count at all. Inside a group the count is the guard, and because chats of one workspace count against each other too, the group stopped twice during the cleanup. The remedy on 29 October was a trusted link from the workspace to itself with no limit. That works, but it sits in a setting nobody sees, and a new workspace starts without it.

## The shape

**1. The hop count stops deciding whether a group message wakes a chat.** In `Doorbell` and `Waking` a message that belongs to a group is tested against the group's budget in place of the hop limit. A message that does not belong to a group is tested exactly as now, with the same trusted links. The hop test and the budget test are two terms of one condition on `m.room`, so no message is ever tested by both.

**2. A group has two numbers, stored on its row in the bus.** The `rooms` table gets `turn_budget INTEGER NOT NULL DEFAULT -1` and `turns_used INTEGER NOT NULL DEFAULT 0`, added by the same `added` list that adds the other columns, so an existing database is upgraded when it is opened and its links are not touched. A budget of -1 (`NoTurnBudget`) means unlimited and is the default, as Paul decided. A budget of 0 or more is the number of turns agents may start in the group between the person's posts. A budget of 0 is allowed: it means only the person starts turns. The budget is a property of the group and the bus is the only place it is kept, so every process that delivers agrees.

**3. What a turn is, and when it is spent.** A turn is spent when `Deliver` hands a chat a message from an agent in a group that asks that chat to answer, through the doorbell or at a Stop (the two ways a delivery starts or continues a turn). One message that mentions three chats spends three, one for each chat it starts. Messages from the person are never counted and never refused: the person's post always wakes the chats it mentions. A message delivered inside the person's own prompt, or to a chat that is already in a turn and reads it through its inbox, is not counted either, since it starts nothing. The count is made inside `Deliver`'s transaction, so two chats woken at once by the same message cannot both take the last turn.

**4. The person's post refills it.** In `insertMessage`, a message from the person to a group sets `turns_used` to 0 in the same transaction as the insert. Setting a new budget also sets it to 0, since a person who has just chosen a number expects it to be whole.

**5. The person sets it; agents cannot.** A new `Bus.SetGroupBudget(group, n)` is called by a window method for the page (`BusGroupBudget`), as `Trust` is for links. No agent tool changes it. `Room` gains `Budget` and `Used` so the group list and the header can show "12 turns left before the group waits for you" when `Budget` is not -1 and nothing when it is. The header and the control are Front End's, as Architecture said.

## What happens to a mention when the budget is spent

The mention is still delivered, and it is not lost. It stops starting turns. In `Doorbell` and `Waking` a group message from an agent counts only while `turn_budget < 0 OR turns_used < turn_budget`, so a chat with only such messages waiting is not woken, and a closed chat is not reopened for them. The messages wait in the chat's inbox and are read the next time the chat takes a turn for any reason, as a message past the hop limit is today.

Inside `Deliver`, a message that finds the budget spent is handed over with `Asked` false, so the agent reads it as background and is not asked to answer it, and its text says the group is out of turns and the person has to post before anyone is asked to answer. A chat whose doorbell rang just before another chat took the last turn may therefore start a turn whose messages are all background. That is rare and harmless, and the alternative, holding the claim across the doorbell and the delivery, would need a reservation the bus does not have.

When the person posts, the budget is refilled and the messages that were waiting count again at the next doorbell, so the chats they mention are woken by the next watcher pass, within a second.

## Hop counts inside a group

The hop count of a group message is still stored, but `Deliver` no longer advances a chat's own count from group messages: only messages outside groups count toward `furthest`. Otherwise a long group session would leave every member with a count in the dozens, and their next direct message to another workspace would be past the limit and wait for the person.

This has one consequence the group's owner should know. A loop that alternates between direct messages and a group's mentions has nothing in it that counts all the legs, because the direct legs restart their count at each group leg. With the default of unlimited the group leg is unguarded, as Paul chose, and the way to guard it is a number on the group. Direct messages between workspaces keep the hop limit and trusted links as they are.

## Stop continuations

`MaxStopContinues` is a separate cap on how many times in a row the Stop hook keeps one terminal agent's turn going, and it is unchanged. The doorbell turn that follows resets it, so a group's mentions are not held back by it for more than one turn.

## When the workspace's link to itself can go

The link was made on 29 October because of the group, but it applies to every message between chats of the workspace, groups or not. After this change the group no longer needs it. The code that allows a workspace to trust itself stays, because direct messages between a workspace's own chats still obey the hop limit, and whether the person wants that raised is the person's choice and not a defect.

So the link can be removed by the person, in the Group Chats pane, once the build with budgets is installed and one working session in the group has run past three exchanges without waiting. There is no code or migration step for it. DESIGN.md's paragraph about it is changed to say it affects direct messages only.

## Tests

1. A group message from an agent past the old hop limit wakes its mentioned chat (`Doorbell` and `Waking`) with the default budget, with no link in place.
2. A direct message past the limit still does not, with and without a link, so the old behavior is unchanged outside groups.
3. With a budget of 2, the first two agent mentions wake and the third does not, and the third is still delivered to the chat's inbox, with `Asked` false.
4. The person's post refills the budget, and the waiting mention then wakes its chat; a post that mentions nobody refills as well.
5. A message mentioning three chats with one turn left wakes one and hands the other two their message as background.
6. A budget of 0: the person's mention wakes its chat, and that chat's reply to another does not wake it.
7. Group messages do not advance a chat's own hop count, and a direct message after a long group session is within the limit.
8. An existing database without the new columns is upgraded, its links and rooms unchanged and every room unlimited.
9. `SetGroupBudget` refuses a value below -1 and a group that does not exist, and resets the used count.
10. A mutation check: with the budget test removed from `Waking`, test 3 fails for a closed chat; with the hop test applied to groups again, test 1 fails.

## DESIGN.md changes

DESIGN.md is changed in the same commit as this document.

- The rooms paragraph says that a group has no limit unless the person sets one, and that handoffs between agents in a group go on until the group's turn budget, if there is one, is spent.
- The paragraph on the three-hop limit says it applies to messages outside groups, and points to the budget for groups.
- The paragraph on the window's delivery says a chat is woken for messages within the hop limit or, in a group, within its budget.
- The paragraph on trusted links says a workspace's link to itself now affects only direct messages between its chats.
- A new paragraph, Group turn budgets, describes the two numbers, what is counted, the refill, the background delivery and the person-only control.
- The decisions table gets a row for the date.

## Questions for Architecture

1. Is it acceptable that a budget counts the turns agents start and not the messages they send? A chat that writes five messages in one turn spends one. I chose it because a turn is what costs the person money and time.
2. Is it acceptable that group messages stop advancing a chat's own hop count, with the loop consequence above, or should a group message advance it by one per turn and the direct limit stay as a backstop? I prefer the first because the second brings the original defect back for a chat that works in a group all day.

## Ruling

Architecture approved this shape on 29 October 2026 (message 573), with one change to what is counted.

1. **The unit is one request to one chat, counted at the bus.** Each time the bus hands a chat one or more asking messages from agents in the group, whether that starts a turn or steers a running one, the budget is spent by one. Messages the bus collects for the same chat into one delivery count once, so a chat that writes five messages to another during one turn spends one, as the proposal wants. Counting this at the bus keeps the count in the one place that every process shares, rather than depending on the window to report when a turn starts, and it matches what the budget exists to bound: agents asking each other to act. A message that mentions three chats spends three, as the shape says.
2. **Group messages stop advancing a chat's hop count**, as proposed. Paul chose no guard inside a group by default, and a group with a budget is guarded by it. A loop that alternates direct messages with group mentions is unguarded until its group has a budget, and DESIGN.md says so.
3. The workspace's self-link stays until Paul removes it after the budgets build has run a working session past three exchanges, as proposed.
