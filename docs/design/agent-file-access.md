# Agent File Access settings

Paul decided on 2 November 2026 that agents' reads through Dinah stay unconfined, with a deny list for known sensitive places, an audit trail of reads, and a clean, clear page to configure both. Engine owns the policy and documents it in `provider-file-reads.md`. This document covers the page, which Front End owns, and the contract the page needs from Go, which is proposed here for Engine to accept or change. Nothing here is built yet.

## Where it lives

The page is a new Settings section, Agent File Access, after Agents in the Settings tabs. Its address is the Settings window's address with the section `file-access`, as every other section has, and the log's filters follow in the same address (see "Addresses"), so a filtered view of the log can be linked to and comes back after a restart.

## What the page shows

The page has three cards, in this order.

**Protected by Dinah.** This card lists the default deny list as two read-only groups. The first holds Dinah's own protected paths, and the second holds the known sensitive places from Engine's default list: SSH keys and agent sockets, cloud and package-manager credentials, git credential stores, browser profiles, password-manager stores, and the data of every other Dinah workspace. Each row shows the place as it resolves on this computer, with the rule's reason in plain words underneath ("Your SSH keys. An agent that read them could sign in to your servers as you."). A row has no actions. A rule that resolves to nothing on this computer is still listed, with "Not present on this computer" as its detail, so the list does not change shape from one computer to another.

**Your entries.** This card holds the places the person has added. It has a sunken path field with a raised Add button beside it, and the list of entries below. While the person types, the page asks Go what the path resolves to and shows the answer under the field: the resolved path, whether it already falls under a rule above, and whether it is inside the current workspace. A path that cannot be an entry, because it is empty, relative or unreadable as a path, disables Add and says why under the field. Each entry in the list has a raised Remove button. Removing an entry loosens protection, so Remove asks first with the shared confirm dialog, which names the place in its question.

**Reads.** This card is the audit log: every read an agent made through Dinah outside the current workspace's registered roots, newest first. Each row shows the time, the chat, the path, and either the number of bytes returned or the rule that refused it. The outcome is shown as a glyph and a colour together, and the row's text says the outcome as well, so it never depends on colour alone. Above the list sit three filters, all sunken: the chat, the time range (last hour, today, last 7 days, everything kept), and the outcome (all reads or refused reads only). Load Older at the end reads the next page.

## PUDL categories

The page uses PUDL's categories as the Architectural Principles describe them. Add, Remove and Load Older are raised buttons. The path field and the three filters are sunken controls. The groups, the entries and the log are flat lists of display rows; a row does not look pressable unless it has a button. Allowed reads use PUDL's positive status glyph and colour, and refused reads its danger glyph and colour. Dinah's own protected paths carry PUDL's lock glyph beside their reason.

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
| `FileAccessPolicy()` | The policy: the built-in rules (each with an id, its kind, `dinah` or `sensitive`, its pattern, the paths it resolves to on this computer, and its reason), the person's entries (each with an id, the path as entered, the path it resolves to, and when it was added), and a version. | Only the person's entries carry `actions: ["remove"]`. |
| `CheckFileAccessPath(path)` | Whether the path can be an entry, the reason when it cannot, the resolved path, the ids of any rules it already falls under, and whether it is inside the current workspace. | Read-only; called while the person types. |
| `AddFileAccessEntry(path, version)` | The new policy. | Refused when the version is stale, so two windows cannot lose each other's changes. |
| `RemoveFileAccessEntry(id, version)` | The new policy. | Refused for a built-in rule and when the version is stale. |
| `FileAccessLog(filter)` | A page of reads, newest first, each with an id, the time, the chat's id and title, the workspace, the path, the bytes returned or the rule that refused it, and the cursor for the next page. | `filter` holds `chat`, `since`, `outcome`, `before` and `limit`. |
| Event `file-access:read` | The new read, as a log row. | Sent after the read is recorded. |
| Event `file-access:policy` | The new version. | The page then reads the policy again. |

## Guard

A browser suite, `e2e/agent-file-access.js`, runs the page against a stubbed Go with a fixed policy and log. It covers:

- the two read-only groups, with no actions on any row and a lock glyph on Dinah's own;
- adding an entry, with the resolution preview, the "already protected" note and the refusal of an invalid path;
- removing an entry, which asks first and is absent for built-in rules;
- each log filter, with the address it writes, and restoring a filtered view from its address;
- a live read arriving at the top and while scrolled down, with every row's box on screen unchanged in the second case;
- the rendered PUDL categories: the buttons raised, the field and filters sunken, the lists flat, and each outcome shown as a glyph, a colour and a word.

## Questions for Paul

1. Can the person switch off one of the known sensitive places, such as browser profiles, or are all built-in rules permanent? The decision says the person may add and remove entries and may not remove Dinah's own protected paths, which suggests the other built-in rules can be removed. The UX direction shows the whole default list as read-only. This design follows the UX direction and keeps every built-in rule permanent until Paul says otherwise.
2. Should a refused read also reach Notifications, or is the log enough? A refused read means an agent tried to read something sensitive, which the person may want to know about at the time.
