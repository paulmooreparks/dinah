# Agent File Access settings

Paul decided on 2 November 2026 that agents' reads through Dinah stay unconfined, with a deny list for known sensitive places, an audit trail of reads, and a clean, clear page to configure both. Engine owns the policy and documents it in `provider-file-reads.md`. This document covers the page, which Front End owns, and the contract the page needs from Go, which is proposed here for Engine to accept or change. Nothing here is built yet.

## Where it lives

The page is a new Settings section, Agent File Access, after Agents in the Settings tabs. Its address is the Settings window's address with the section `file-access`, as every other section has, and the log's filters follow in the same address (see "Addresses"), so a filtered view of the log can be linked to and comes back after a restart.

## What the page shows

The page has two cards: the rules, then the reads.

**Rules.** This card holds one PUDL list of every rule, in three kinds, each drawn with the controls its kind allows. Go says which actions each row has, so the page never draws a control that Go did not offer.

- A protected rule covers one of Dinah's own protected paths. It has no control and carries PUDL's lock glyph.
- A default rule is one of the known sensitive places from Engine's default list: SSH keys and agent sockets, cloud and package-manager credentials, git credential stores, browser profiles, password-manager stores, and the data of every other Dinah workspace. Its path and reason cannot be edited. It has a switch that turns it on or off, and the setting persists. Turning a default off loosens protection, so it asks first with the shared confirm dialog. A default that is off stays in the list and says plainly that it is off; it is never hidden.
- A custom rule is one the person added, and it has a raised Remove button. Removing one also asks first.

Each row shows the rule's path as it resolves on this computer, with its reason in plain words underneath ("Your SSH keys. An agent that read them could sign in to your servers as you."). A rule that resolves to nothing on this computer is still listed, with "Not present on this computer" as its detail, so the list does not change shape from one computer to another.

Below the list sit a sunken path field and a raised Add button for a new custom rule. While the person types, the page asks Go what the path resolves to and shows the answer under the field: the resolved path, whether it already falls under another rule, and whether it is inside the current workspace. A path that cannot be a rule, because it is empty, relative or unreadable as a path, disables Add and says why under the field.

**Reads.** This card is the audit log: every read an agent made through Dinah outside the current workspace's registered roots, newest first, together with each change to the rules, such as a default turned off, with its time. Each read's row shows the time, the chat, the path, and either the number of bytes returned or the rule that refused it. The outcome is shown as a glyph and a colour together, and the row's text says the outcome as well, so it never depends on colour alone. Above the list sit three filters, all sunken: the chat, the time range (last hour, today, last 7 days, everything kept), and the outcome (all reads or refused reads only). Load Older at the end reads the next page.

## PUDL categories

The page uses PUDL's categories as the Architectural Principles describe them. Add, Remove and Load Older are raised buttons, and a default rule's switch is PUDL's switch. The path field and the three filters are sunken controls. The rules and the log are flat lists of display rows; a row does not look pressable unless it has a control. Allowed reads use PUDL's positive status glyph and colour, and refused reads its danger glyph and colour. Dinah's own protected paths carry PUDL's lock glyph beside their reason.

## Live updates

The log is the one part that changes while the page is open. A new read arrives as an event from Go. When the reader is at the top of the log, the new row is added above the others. When the reader has scrolled down, the rows on screen stay where they are, and a count above the list says how many newer reads are waiting; choosing it scrolls to the top. A policy change from another window replaces the two lists in place, keeping the focused entry focused, or moving focus to the field when that entry is gone. Nothing on the page moves or changes size because of a live update, except the log growing below the rows already shown.

## Addresses

The section's state is written into the Settings window's address as `file-access`, followed by the filters that differ from their defaults:

- `chat=<chat ID>` limits the log to one chat.
- `since=hour`, `since=today` or `since=week` limits the time range; with no `since`, everything kept is shown.
- `outcome=refused` shows refused reads only.

The path being typed is not part of the address, since it is a draft and not a view.

## The contract the page needs from Go

This is a proposal for Engine. The page follows the Architectural Principles' rule that an action the person may not take is absent from what Go returns, instead of being hidden by the page.

| Call | Returns | Notes |
| --- | --- | --- |
| `FileAccessPolicy()` | The policy as one list of rules and a version. Each rule has an id, a kind (`protected`, `default` or `custom`), a path pattern, a plain reason, an enabled flag where the kind allows one, the path it resolves to on this computer, and its actions. | A default rule carries `actions: ["enable"]` or `["disable"]`, a custom rule `["remove"]`, and a protected rule none. |
| `CheckFileAccessPath(path)` | Whether the path can be a custom rule, the reason when it cannot, the resolved path, the ids of any rules it already falls under, and whether it is inside the current workspace. | Read-only; called while the person types. |
| `AddFileAccessRule(path, version)` | The new policy. | Refused when the version is stale, so two windows cannot lose each other's changes. |
| `SetFileAccessRuleEnabled(id, enabled, version)` | The new policy. | Only for a default rule; the change is recorded in the log. |
| `RemoveFileAccessRule(id, version)` | The new policy. | Only for a custom rule; refused when the version is stale. |
| `FileAccessLog(filter)` | A page of reads, newest first, each with an id, the time, the chat's id and title, the workspace, the path, the bytes returned or the rule that refused it, and the cursor for the next page. | `filter` holds `chat`, `since`, `outcome`, `before` and `limit`. |
| Event `file-access:read` | The new read, as a log row. | Sent after the read is recorded. |
| Event `file-access:policy` | The new version. | The page then reads the policy again. |

## Guard

A browser suite, `e2e/agent-file-access.js`, runs the page against a stubbed Go with a fixed policy and log. It covers:

- the three kinds in one list, each with only the controls Go offers it: none and a lock glyph on a protected rule, a switch on a default, and Remove on a custom rule;
- turning a default off, which asks first, keeps the row in place marked as off, and adds a policy change to the log;
- adding an entry, with the resolution preview, the "already protected" note and the refusal of an invalid path;
- removing a custom rule, which asks first;
- a refused read, which adds to the chat's passive count and does not reach Notifications;
- each log filter, with the address it writes, and restoring a filtered view from its address;
- a live read arriving at the top and while scrolled down, with every row's box on screen unchanged in the second case;
- the rendered PUDL categories: the buttons raised, the field and filters sunken, the lists flat, and each outcome shown as a glyph, a colour and a word.

## Decided by the lead under Paul's decision 3

The Team Lead settled two questions on 2 November 2026 (messages 3357 and 3359), so the design has none open for Paul.

The default rules can be turned off but not edited or removed. Their paths and reasons are read-only, each has a persistent switch, and turning one off asks first and is recorded in the log with its time. Protected rules have no switch. Custom rules keep Add and Remove.

A refused read does not go to Notifications. It is the policy working, and a notification for every refusal would teach Paul to dismiss them. It goes to the log, and the chat whose read was refused shows a passive count of refused reads in its status detail and tooltip, which clears when he opens the log.
