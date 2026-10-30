# Choosing a Chat's Workspace, and Switching a Chat Visibly

Paul asked on 30 October 2026 for three things, which this plan takes together because they share one confirmation and one progress display.

1. **New Chat asks which workspace the chat belongs to.** The current workspace is the default, and nothing is asked when only one workspace is configured.
2. **A chat can be moved to another workspace.** A chat's workspace is the agent's working folder, the boundary of the files Dinah lets an Ollama chat touch, the scope of its applet actions, the source of its workspace skills, the folder its attachments are saved in, and the folder the provider is given whenever the chat is reopened or resumed. The agent's history also assumes that folder. Changing the folder under an existing conversation would leave the agent with paths that no longer mean what they did, and reopening it would depend on how each provider keys its sessions, which is not documented. So moving a chat is a continuation: "Continue in Another Workspace" starts a new chat with the same provider in the chosen workspace, carrying the summary and recent turns through the continuation path that compaction and provider switches already use. The two chats are linked, and the old one stays where it was, unchanged.
3. **Switching a chat's provider is confirmed and visibly in progress.** Today a switch shows only in the provider button's hover text. Paul asked for a confirmation first, with a warning about what will change, and then for the chat to be inactive with a progress bar while the switch runs, indeterminate when no percentage can be calculated, while every other window stays responsive. The same applies, in a smaller form, to continuing in another workspace.

## The Confirmation

Choosing another provider in a chat, or Continue in Another Workspace, opens a confirmation before anything happens. It says what will change, in plain words, from facts the engine sends rather than text the page makes up.

- The chat continues as a new chat with the chosen provider and model, or in the chosen workspace, and the current chat stays as it is, linked to the new one.
- The new chat starts from a summary of the older conversation and the recent turns in full, and how much of the conversation each part covers, from the handoff the engine is about to build.
- Settings that belong to the current provider, such as its approval mode, do not carry over, and the confirmation lists which.
- For a workspace move, the new chat's files, skills and applet actions are the new workspace's.
- A turn that is still running must end first. The confirmation offers to stop it, or the person cancels and waits.

The person confirms or cancels. Cancelling changes nothing.

## While It Runs

Once the person confirms, the chat becomes inactive. Its transcript can still be read and scrolled, but its message box, its controls and its provider and model pickers are disabled. A progress bar sits where the message box was, and it says which stage the switch is in: building the summary, starting the new provider, handing the conversation over, and opening the new chat. The engine reports each stage as it begins, through the chat's numbered state, so a resync or a window opened late shows the same thing. The bar is indeterminate, because the engine cannot know how long a provider takes to start. The stage names are facts and show progress honestly, whereas a percentage would be invented.

All of this runs in the background on the engine's side, so every other window and chat stays responsive throughout, and no lock that other chats need is held while a provider starts or a judge writes a summary.

When the switch finishes, the new chat opens in the same window and the old one's link to it is shown. When it fails, the chat becomes active again on its old provider or in its old workspace, unchanged, and the failure is said in plain words where the progress bar was.

## New Chat's Workspace

New Chat in the Dinah menu asks which workspace the chat belongs to when more than one is configured, with the current workspace chosen. Creating a chat already takes a workspace in the engine, so this is the page asking for one where it now assumes the current one.

## Who Builds It

Dinah Engine Cleanup builds:
- the facts the confirmation shows (what the handoff will contain, which provider settings will not carry, and whether a turn is running);
- the switch's stages in the chat's numbered state;
- Continue in Another Workspace, as a continuation into a chosen workspace with the same provider;
- making sure the whole switch runs without holding anything other chats need.

Dinah Front End Cleanup builds the confirmation, the inactive chat with its stage-labelled indeterminate progress bar, the failure message, the workspace choice in New Chat, and the Continue in Another Workspace command. Each half comes back for review with tests, including a switch that fails partway and leaves the chat as it was, and a second window kept responsive during a switch.

## The Engine's Shape

Engine proposed the shape on 30 October (message 926), and Architecture approved it (message 927).

Engine found that starting a provider held one lock for all providers for the whole start, which can take 20 to 30 seconds, so every other chat needing a connection waited behind a switch, including chats on providers already running. That lock becomes one for each provider, taken only after checking whether the provider is already running, so a chat on a running provider never waits and a second request for a provider that is starting reuses that start. This is part of this round.

The continuation's preview gains facts built by the engine. They say whether it is a provider switch or a workspace move, what it moves from and to, what the handoff will hold, which settings will not carry over, and what must finish first. The chat's numbered state gains the switch's stage, with a plain label from the engine for each of building the summary, starting the provider, handing over and opening the new chat, and a failure in plain words that leaves the chat as it was. New calls preview and carry out a workspace move and dismiss a failure. A moved chat records the workspace it came from, so the continuation chain may cross workspaces where it records a move.

A workspace move carries the chat's saved options, such as its model, because the provider is the same. Settings that grant authority, such as an approval or permission mode, are not carried: the new chat starts with its provider's default in the new workspace, and the confirmation lists them as not carried, so authority never moves into another folder without the person setting it there.

Paul decided on 30 October (message 928) that a failed switch's message stays in the chat until he dismisses it, since he may have looked away while it ran.

## Renaming a Group Chat

Paul found on 30 October (message 935) that a group chat cannot be renamed, when he wanted to call this group Dinah Development. It joins this round. The bus keeps each group under a stable name of its own and a title shown to people, so a rename changes the title alone, and the group's members, messages, budget and unread marks stay as they are. Messages delivered after a rename use the new title. A chat that still addresses the group by its old title is answered with a plain refusal that gives the new title, rather than a silent failure, so an agent's stale instructions correct themselves. Engine builds the rename on the bus and a page method for it; Front End adds Rename Group… to the group window's menu, with the same rules as renaming a chat.

## Built So Far

On 30 October 2026 the engine side of renaming a group chat was built (`BusRenameGroup`), and so was the engine side of the switch: the lock that starting a provider held across every provider is now each provider's own, the preview of a continuation gives the facts the confirmation shows, the chat's numbered state carries the switch's stage and a failure until it is dismissed, and `PreviewSessionWorkspaceMove`, `ContinueSessionInWorkspace` and `DismissSwitchFailure` move a chat to another workspace by continuation, carrying the settings that grant no authority and not a permission mode, with the workspace the chat came from recorded in the new chat's record.
