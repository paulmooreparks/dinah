# Leads and Teams

Paul observed on 30 October 2026 that since the work moved to a lead with a team, he talks almost only to the lead chats (Dinah Architecture here, PUDL Architect elsewhere) and hardly ever to the chats that do the building, while those chats still send him messages he will probably never read, because each of them is a full chat window competing for his attention. A provider's subagent is the wrong answer, because it is transient and cannot be reopened, while the building chats carry days of context that has to survive restarts. This document proposes a team member: a chat that keeps everything a chat has, belongs to a lead, and reaches Paul only through that lead. Paul approved writing it the same day, and decided that he sees a team member's messages to him but that they are relayed through the lead, and that a team member can lead a team of its own. Nothing here is built.

## What a Team Member Is

A team member is an ordinary chat, with its provider, model, workspace and saved history, which can be opened at any time and continues across restarts. Three things set it apart.

1. **It belongs to a lead.** The bus records, for each team member, the chat that leads it. A lead is any chat with at least one team member, and a team member can itself lead a team, so teams nest. Paul is above every lead.
2. **It reports to its lead.** A message in which a team member mentions Paul is delivered to its lead instead, as a request to relay, and it also appears in the group for Paul to read, marked as addressed to him through the lead, without notifying him. The lead decides whether Paul needs it, and relays what he would act on as its own message, which notifies him as any lead's message does. A nested team member's message rises one level at a time, so it reaches Paul only through the chain of leads above it. Paul's own messages to a team member go to it directly, as now, and its answer to a message of Paul's reaches him directly, because he asked.
3. **It has no window or taskbar button until Paul opens one.** It is shown in its lead's window instead, as described below. Opening it from there gives it a window like any chat's, and closing that window returns it to the lead's window without stopping it.

## The Team in the Lead's Window

A lead's window has a team strip listing its team members, each with its state as a glyph and a colour (working, waiting for its lead, stuck on a failure, idle), its current task in one line taken from the last brief or assignment it was given, its provider and model, and its usage for the day. A nested team shows under its lead as an indented row that opens to its own members. Pressing a member opens its chat. The strip is where Paul sees, without reading any chat, who is doing what. The lead's state shown elsewhere in Dinah includes a mark when one of its members is stuck.

## What a Lead Can Do

A lead acts on its own team through applet actions, and only on its own team.

- **Start a team member** with a title, a brief, a workspace, and optionally a provider and model, which otherwise follow the lead's. The brief is the new chat's first message.
- **Assign work** to a team member, which is an ordinary message mentioning it in the team's group.
- **Retire a team member**, which ends its session and keeps its history, so it can be read and reopened later.

Paul can do everything a lead can, and can also move a team member to another lead or out of any team, at which point it becomes an ordinary chat with a window and reaches him directly again.

## How It Fits What Exists

The team's channel is a group chat the lead runs, with the lead and its members as members and Paul included. The group-chat machinery stays as it is: notices for unmentioned messages, mentions that deliver the whole message, "everyone", the turn budget and the queue on disk. A team adds two facts to the bus, a chat's lead and the routing of a member's mention of Paul through it. The existing group Dinah Cleanup (to be renamed Dinah Development) becomes a team with Dinah Architecture as its lead and Dinah Engine Cleanup, Dinah Front End Cleanup and Dinah UI as its members.

When a member's lead is closed, the request to relay opens it in the background, as any message that asks a closed chat to answer does. When a member's lead is deleted, its members' messages to Paul reach him directly until Paul gives them another lead, so nothing addressed to him is lost.

## Who Builds It

Dinah Engine Cleanup builds the bus side: the lead relation, the routing of a member's mention of Paul through its lead with the message still visible in the group, the nesting, and the applet actions to start, assign and retire. Dinah Front End Cleanup builds the page side: the team strip, no window or taskbar button for a member until it is opened, and the marking of a relayed message in the group. Dinah UI reviews the page. Each half comes back for review with tests, including a member's message to Paul reaching the lead and appearing in the group without notifying Paul, a nested member's message rising through two leads, a lead deleted with members left, and a member moved out of its team reaching Paul directly again.

## Paul's Answers and the Second Round

Paul answered on 30 October.

- **A relay that fails.** The lead's judgement stands when it chooses not to relay, but a message held up by an error reaches Paul directly. It is held up when the delivery to the lead fails, when the lead's turn on it ends in an error, or when the lead cannot run (its provider is down, its budget is spent or its window is full and held). Dinah then notifies Paul with the member's message and the reason it could not be relayed.
- **A team budget.** It is optional, and it is not normalized across providers, because tokens and ACUs have no honest exchange rate. A lead's team budget is instead a set of per-provider allowances, each in that provider's own unit, that the team's members on that provider share, with money shown beside it where the provider reports prices.
- **Converting.** Paul wants to turn a chat into a team member and a team member back into a chat, and an existing group into a team: "Make This Group a Team" asks which member chat leads it, and the group's other chats become its members. A chat has at most one lead. In other groups it stays an ordinary member, and its messages to Paul there still go through its lead.

Paul also proposed how the windows should divide the work, and Architecture agrees.

- **The team chat stays.** It is the team's own window, where Paul talks to the team, and where he speaks to a member, by mentioning it. The lead's own window remains his conversation with the lead.
- **The lead's window keeps the lead's conversation readable.** The team's messages reach the lead and stay in its context, but its window folds them, each run of team traffic shown as one line ("4 team messages; open the team chat") that opens the team chat at that point. The lead's own messages to the team fold the same way. Nothing is removed from the lead's saved history; only its window shows it folded.
- **A team member's window has every control except the message box.** Paul can stop a turn, change the model or provider, answer a full-context hold, and read everything, but where the box would be there is a line saying that the member is spoken to in the team chat, with a button that opens it with the member already mentioned. Steering a running member also goes through the team chat.

## The Bus Side's Shape

Engine proposed the bus side on 30 October (message 1182), and Architecture approved it with changes (messages 1184 and 1185). A table of team members, keyed by the member, records each chat's one lead, refuses a cycle, and is shared by every program on the machine through the bus's database. A team's channel is a group chat marked with its lead. Paul changes the relation from the page, and a lead changes it only for its own direct members, through a new applet, dinah.team, with start, assign, retire and list. A member's mention of Paul is changed when the message is stored: Paul leaves the list that wakes and notifies, the member's nearest lead takes his place, and the message is marked as for Paul through that lead, so it shows in the group without a notification, and a lead's own relay rises the same way. A relay fails when its delivery to the lead fails, when the lead's turn on it ends in an error, or when the lead cannot run, and Paul is then notified once with the member's message and the reason. A team budget, if set, is a list of per-provider allowances, never added across providers, with turns of unknown cost counted apart.

A retired member stays listed in the group and the strip but is left out of the group's wake and notice set until it is reopened, so a group message never reopens its ended session. A member started by dinah.team/start takes its provider's default for any setting that grants authority, never the lead's, and its permission requests reach Paul through Notifications as now. The page reads the team's group key with the team and with a member's lead, and the team's changing state as a numbered snapshot and event, so the strip follows a working member. Paul confirmed on 30 October that a team that uses up an allowance is warned rather than held, that the team budget runs on the same weekly period as the Judgements budget, and that a retired member stays in the strip, folded.

Engine built the bus side on 30 October (message 1244), and Architecture settled its two open points (message 1246). A lead starts, assigns and retires its own members without Dinah asking Paul each time, because the lead's own direct members are the limit and Paul chose warnings over holds for spending; every start posts one line in the team's group with the new member's title, provider, model and the first line of its brief, so no start is silent. The budget counts only what an adapter reports a request used, which today is Ollama and Codex. A Claude Code turn is counted as unknown, because its usage_update reports how full the context is rather than what the turn spent, unless its prompt response reports the turn's usage in a field the protocol documents. Engine found that the protocol's field for a turn's usage in the prompt response is marked unstable, liable to be removed or changed at any point, so Dinah does not rely on it, and the budget lists the providers it cannot measure. The lead's own successful messages to its team are marked with the team's group and the stored message number, in restored history as well, so the lead's window folds them without reading tool calls or prose, and a failed send keeps its error and is never folded.

Paul decided on 30 October (message 1305), after the group became the first team, that a message of his in a team's group that mentions nobody goes to the team's lead as a request to answer, as a mention of the lead would. A mention of a particular chat still goes to that chat, and the other members still see the message as a notice. Until then the team's working rule is that only the chat whose work an unmentioned question concerns answers it.

Paul also agreed the same day that the build and release work leaves the lead, and he added a member for it, Dinah Release, on Claude Sonnet. Dinah Release merges into master only the commit Architecture approves, runs the Go and browser gates, installs, and reports the counts and the failure record word for word. It owns the test slot, so only one full gate runs at a time. After each successful install it pushes master and the work branches to the private GitHub repository, which Paul asked for on 30 October so that the work is kept somewhere safe; the repository stays private until the cleaning pass. It never force-pushes, restarts, edits product code or tests, or changes these plans. Architecture keeps the review and the decision to merge. The brief is message 1313 in the team's group.
