# Teams in the Chat Windows

This proposal implements the page behaviour Paul approved in [teams.md](teams.md). Dinah UI owns this proposal and the rendered review; Front End builds it after the PUDL batch. Architecture approved Engine's shape with changes in messages 1184 and 1185, and approved this proposal with the additional reads in message 1191. The cross-project rules remain in `C:\Users\paul\OneDrive\Documents\Architectural Principles.md`.

## The Lead's Team Strip

The lead's window places a section named Team between its state line and its transcript. A PUDL disclosure button shows or folds the section, with the member count and any stuck members stated in words. The transcript keeps its own scroll position when this section changes.

The section uses PUDL's navigation tree. Each member's title links to its chat; its state badge, task, provider/model and today's usage are readable parts of the node. A nested lead has PUDL's separate branch toggle and its members indented beneath it. Following the title opens the chat; expanding the branch stays in this window. Rows remain in a stable order while state and usage change.

| State | PUDL treatment | Visible words |
|---|---|---|
| Working | Accent badge with ring glyph | Working |
| Waiting for its lead | Warning badge with flag glyph | Waiting for Lead |
| Stuck on a failure | Danger badge with warning glyph | Stuck |
| Idle | Neutral badge with circle glyph | Idle |

Engine supplies the state and its reason. A full-context hold or unavailable provider must remain distinguishable from idle, with the reason beside the state. Retirement is a separate lifecycle label, not an idle state. Retired members remain in a folded list marked Retired. They receive no group wake or notice until reopened. Architecture approved this default in message 1184; Paul can overrule it.

The current task is the first line of the last brief or assignment, as Engine proposes. It occupies one line at normal width; its full text is available through the linked assignment in the team chat. At narrow widths, provider/model and usage move below the title, and the task may wrap. Status words stay visible. No field depends on hover to be understood.

The lead's title-bar status group adds a warning glyph and words such as "2 Members Stuck" when descendants are stuck. PUDL's existing Window status control handles compact headers. Dinah supplies that same aggregate to other representations of the lead without replacing its own Working state.

## A Member's Window

A member opens only when Paul follows its link or explicitly restores its window. Background work, membership changes and state updates must not create a window or taskbar entry. Closing its window closes the view and leaves the session running; Stop and Retire remain separate commands. An explicitly saved open member window may restore at startup.

The member keeps its transcript and all applicable chat controls, including Stop, provider/model choices, approvals, export and full-context recovery. Front End must separate those controls from the composer before removing the message box. The member has no direct Send, Queue or Steer input. Attachment and skill insertion actions that require a message destination belong in the team composer.

Where the message box stood, a flat explanation says "Speak to this member in [team name]." A labelled Open Team Chat link opens that group with this member mentioned in the draft; it sends nothing and preserves any existing draft. Paul steers a running member there. A nested lead's window combines this member treatment with its own team strip.

## Folded Traffic and Relays

The lead's transcript replaces each consecutive run of messages to or from its own team channel with one navigation link, such as "4 team messages; open the team chat". It opens the group at the first message in that run. Direct conversation with Paul stays in place. Messages from unrelated groups stay in their existing presentation. Saved history, model context and exports retain the original messages.

Engine supplies each run's stable group key and first and last message numbers, as specified in message 1186; the page never parses message prose to identify it. New traffic updates a run's count without stealing focus or moving a reader who has scrolled back. A run interrupted by direct conversation becomes two links. Each link names its team when several team channels appear through nesting.

A group message addressed through a lead shows a neutral information glyph and the words "For Paul through [lead]". That mark does not claim delivery to Paul. A failed relay produces the engine's persistent Notifications card with the original message, the reason, and links to the source and lead. The page neither creates a second notification nor infers failure from elapsed time.

## Usage and Team Commands

Each member row shows today's reported usage in its provider's unit. Team allowances appear separately, one labelled PUDL meter per provider, with spent, allowance and period stated in text. Money appears only where reported pricing supports it. Nested totals count each member once; a parent includes its descendants without adding a child's aggregate again. Tokens and ACUs never share a total or meter.

Unknown usage reads "Usage unavailable" or "3 turns unmeasured"; it never reads zero. If some usage is unknown, the allowance display states that its remaining amount is incomplete. With no allowance, show usage in words without a percentage. Engine supplies the period boundaries, unit and completeness. Architecture approved weekly allowances on the Judgements budget's period, with today's use shown separately, and a warning to the lead and in the strip when an allowance is spent. Exhaustion does not hold new turns. These defaults stand until Paul changes them.

Starting a member uses its provider's default permission mode rather than copying the lead's authority. Permission requests still reach Paul directly through Notifications; they are not relayed through the lead.

The group window offers Make This Group a Team as a short PUDL dialog asking which chat leads it. Cancel comes first. The result names members excluded because they already have another lead. Paul can also change a member's lead or make it an ordinary chat through its commands. These operations use Engine's permitted actions and reasons; errors stay beside the operation as notices. A member made ordinary regains its message box and opens as an ordinary chat.

## New Chat from Add Chats

Paul asked on 30 October 2026 for a New Chat… button in the Add Chats dialog of a group or team window, so that he need not go to the menu to make a chat before adding it. The dialog gains New Chat… beside its list. It makes the chat with the same choices as the menu's New Chat (workspace, provider, model and title) as a step inside the same dialog, not as a second dialog over the first, and then returns to the list with the new chat ticked, so Add adds it with any others chosen. Cancel in that step goes back to the list without making a chat. In a team's group, a chat added this way becomes a member led by the team's lead, as the other chats do when a group is made a team. Dinah UI designs the step and Front End builds it, with a test that the new chat arrives ticked and that cancelling makes nothing.

Dinah UI proposes the following step design in response to Architecture's message 1326. The existing picker keeps its host window and replaces its list body with the short form; no second modal opens.

- The list view places a secondary New Chat… button immediately above the list, beside its filter. Existing selections and the filter stay intact while the creation step is open.
- The creation step has the heading New Chat and labelled Workspace, Provider, Model and Title fields. It reuses New Chat's choices and defaults, updates model choices when the provider changes, and requires a nonempty title so the result can join the group. It does not switch the Workspace pane or copy a lead's permission mode.
- The creation step replaces the picker's actions with Cancel first and Create Chat last as its only primary action. Cancel returns to the unchanged list and restores focus to New Chat…; Escape has the same effect while this step is active. Validation and creation errors stay in the step, with field errors or a persistent notice, so the entered values can be corrected.
- Create Chat creates the named chat without sending a first message, then returns to the list with the new chat ticked, visible and focused alongside the previous selections. If the current filter would hide it, the picker clears that filter and says that it did so. A status line says "Chat created and selected. Press Add to add it to this group." The list's Add remains the membership commit.
- In a team's group, Add makes the new chat a member of that group's lead. Until Add succeeds, it remains an ordinary chat. The creation form states "The chat is created now and joins this group when you press Add. Cancelling Add Chats keeps the created chat." Cancelling the creation step itself creates nothing. If Add fails, the picker keeps the new chat selected and reports the reason without creating another chat on retry.

The fixture must check creation and selection, preservation of previous selections, cancellation before creation, filter visibility, and team membership occurring on Add. Screenshots must show the creation step and the returned list. Front End uses PUDL's existing fields, select wrappers, actions and notices; no new PUDL component is needed.

## PUDL Coverage and Remaining Contracts

| Surface | What pinned PUDL 0.94.0 provides | What Dinah must supply |
|---|---|---|
| Team strip and nested rows | Navigation tree, branch toggles, badges, layout tokens and links | Team hierarchy, row content, state mapping and responsive composition |
| Lead's stuck-member mark | Title-bar status chips and compact Window status popover | Descendant count and reason, updated with team state |
| Member window | Ordinary windows, toolbars, commands and notices | Session/view separation and controls independent of the message box |
| Folded traffic | Links and applet/window address state | Stable run grouping and a group address targeting a message |
| Allowances | Native meters with labels, values and marks | Provider units, period, completeness and budget policy |
| Conversion and errors | Dialogs, selects, notices and command surfaces | Permitted operations, validation and results |

PUDL has no team domain model or transcript-run component. These screens can compose its existing components without a new PUDL control or edits to vendored files. The approved page contract includes the stable team group key in `Team(lead)` and member lead information, source message IDs for folded traffic and the current assignment, state reasons, usage period boundaries and completeness, and the count of stuck descendants. Team snapshots and events carry a generation and version covering task, usage, member state and relations. Resync reads the snapshot again, and older events cannot replace newer state. Engine may coalesce usage events to at most once a second per team, as proposed in message 1186.

Front End's review fixtures must show nested teams, simultaneous working/waiting/stuck members, missing usage, a provider allowance, a member's controls and team link, folded traffic interrupted by Paul's conversation, and a failed relay. Screenshots must cover light and dark themes and a narrow chat window. Interaction checks must cover tree keys, preserving the team draft, a close without stopping work, restored views, and holding the reader's place during live updates. This proposal has not yet been rendered or implemented.
