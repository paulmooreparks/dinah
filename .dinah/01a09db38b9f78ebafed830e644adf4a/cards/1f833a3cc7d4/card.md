---
title: "Needs You: the view that answers what is waiting on me"
column: 5ea2db0272fc
state: ready
severity: minor
priority: soon
workstreams:
  - 58f3e3eb621a
links:
  - kind: spawned_from
    to: 81b9d0726b36
---
Sub-card of dinah-185. Depends on the scaffold, dinah-263. The operator called this the anchor of the extension in the design conversation of 2026-08-24, above the tree.

A view at the top of the container answering one question: what is waiting on me right now. Pending operator-owned questions, cards standing at operator stations, blocked cards awaiting a ruling, and pull requests awaiting approval.

The argument for putting it above the tree is behavioural rather than aesthetic. Watching an operator work this board for a day, every single engagement was to rule on something or to move a card at a station. Not once was the whole board the thing needed; the part with their name on it was. GitLens calls this Launchpad, and dinah-185 lists it as prior art without making it the point.

## Build the mechanism, not the view

**"Needs You" is one saved query among several and must not be hardcoded.** dinah-185 already wants saved queries as tree sections, each section a stored qualifier query, on the GitHub Pull Requests precedent. This card builds that mechanism and ships this view as its first instance.

Two reasons. What counts as needing you is a judgement that will change, and it should change by editing a setting rather than by shipping a release. And the same machinery immediately gives the holdings view and the aging-WIP view that dinah-185 wants, so building it once here is cheaper than three special cases.

Every section is a `dinah query` call, per the ruling on dinah-265. No client-side filtering.

## What the spec has to settle

**What "needs me" actually resolves to**, in query terms, using the twelve fields that exist. `substate:blocked` reaches blocked cards. `block_kind` reaches an operator decision specifically. Cards at an operator station want the states that declare `operator_owned`, which is a state property rather than a card field, so check whether the query language can express it or whether the section has to be assembled differently. That gap, if it is one, is worth reporting rather than working around on the client.

**Pending operator-owned questions are the hardest part and the most valuable.** Checklist items with `owner="operator"` in a pending state are what an operator is genuinely waiting to answer, and they are not cards. Whether dinah's read surfaces expose them at all is the first thing to check. If they do not, that is a card rather than a client-side walk of card files.

**Pull requests awaiting approval are not dinah's to know.** The extension knows the card's `branch_name`. Deciding whether this view reaches GitHub at all, or leaves that to the GitHub extension already installed beside it, is a real scope call and the refusals on dinah-185 lean toward leaving it alone.

## Out of scope

The tree itself (dinah-265), the LSP (dinah-264), the webview (dinah-266). The holdings and aging-WIP views are deliberately not built here even though this card's mechanism makes them cheap; file them separately once the saved-query surface exists and its shape is known.
