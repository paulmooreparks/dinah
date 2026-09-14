---
title: The VS Code extension
column: 5ea2db0272fc
state: ready
severity: minor
priority: soon
workstreams:
  - 58f3e3eb621a
---
A bench is a folder of Markdown files, so VS Code is already half a client, and the editor is where most of Dinah's users will meet a board first. The extension is one more head over the verb library: it speaks the machine surfaces, never parses human output, and re-implements nothing. This card is the cover for that work, holding the surveyed capability inventory so the rungs can be filed as sub-cards against a shape that was thought through once. The design chat that produced the inventory ranged over sidebar views, an LSP for hand-edited bench files, editor integrations that come free from VS Code's own extension points, and the boundaries the extension has to refuse in order to stay a viewer rather than a harness or a portfolio product.

## Working notes

Capability inventory from the design chat of 2026-08-19. Recorded here so the sub-cards inherit it; nothing below is a ruling.

## The ladder, as the surfaces design already states it

Rung one is the LSP plus the verbs: registry-driven validation and completion inside anchor files, and CodeLens actions on a card file that shell to the CLI and refresh from the receipt. Rung two is the sidebar tree: benches at the top from the same discovery walk the CLI uses, states beneath in definition order, cards beneath those grouped by substate, and every node opening its anchor on click because every node is a file. Substate rides the tree's badge and color decorations, a capacity-limited state shows its count against the limit, and a node's context menu is the affordances block rendered as menu items, so illegal actions stay absent from the representation. The in-binary TUI runs in the integrated terminal at every rung for free. The webview rung comes last, embedding the server-rendered ui pages once they exist.

## Views

- The bench tree is the anchor, and it wants dinah-151's node form so the extension renders one response instead of assembling a hierarchy from list calls.
- A holdings view shows what this actor holds with lease countdowns, which is the state an operator or an agent checks most often.
- Saved queries become tree sections, each section a stored qualifier query. The precedent is the GitHub Pull Requests extension, whose query-driven trees are authored by the user in settings rather than shipped one at a time. This wants dinah-135.
- An aging-WIP view lists cards in work states ordered by time in state, oldest first. It teaches the discipline rather than only displaying data, and the journals already carry everything it needs.
- An instructions panel follows the current claim and renders the served chain as Markdown. The tool's thesis is that the station says what happens there, and no competing board tool has anything to put in that panel.
- Blocked cards belong in a section of the tree rather than a view of their own, because a block is the operator's inbox.
- A Timeline contribution puts a card's journal events in the view the built-in Git extension already fills with commits, which is free real estate with no new interface to design.

## The LSP

The format document promises that a person may edit by hand and then ask the tool whether they broke anything. The LSP turns that promise into immediate feedback, which is the whole feature.

- Diagnostics cover the invariants the checker already knows: claim fields present exactly when the substate is active, a block carrying its reason, a state id that resolves, identifiers unique within their collection. They also cover the two lints the surfaces design already named, meaning a trailing comment that has drifted from its state's real title, and backwards timestamps across journal lines.
- Completion draws closed enums from the token registry and open sets from the bench itself, so state slugs, declared level names, workstream identifiers, and actors seen in the journal all complete.
- Hover on a token shows its registry definition and the profile statement that governs it, which makes the contract teach at the point of use.
- Go-to-definition carries a card's state id to its state.md, and find-references inverts it to every card standing in that state.
- Rename is safe because identity is the hex identifier, so changing a slug or a title cannot break a reference.
- A formatter implements canonical frontmatter key order once that question is ruled.
- Code actions handle the quick fixes, and one of them can invoke the CLI to record a manual-correction event, which surfaces the format's witnessed-edit rule as a lightbulb.

## Other capabilities

- Registering `dinah mcp` with VS Code's agent mode for the discovered bench wires the user's own harness without the extension becoming one.
- A status bar item carries the held card and its lease countdown, which is the cheapest ambient awareness available.
- Every verb is reachable from the command palette through a quick pick.
- The bench checker runs as a task provider with its findings in the Problems panel.
- Tree drag-and-drop performs moves, with what is draggable coming from the affordances block and a refused drop reporting its refusal.
- The watch feed from dinah-120 becomes notifications, so an operator pushing a card back reaches the holder without polling.
- A welcome view appears when no bench is found, and a walkthrough shells to `dinah guide` output so guidance stays served rather than seeded into the extension.
- Binary acquisition carries real adoption weight. A marketplace extension often installs on a locked-down corporate laptop where downloading a binary does not, so the extension may be the delivery vehicle for the binary itself.

## Prior art to mine

GitLens for blame-style annotation, rich hovers, and the Launchpad idea of surfacing what needs attention now. The GitHub Pull Requests and Issues extension for query-driven trees and for the Comments API, which is how card comments could render as real threads. The built-in Git extension for the Timeline contribution, the SCM view shape, and welcome views. Foam and Dendron are the non-obvious ones, because they solved navigating a folder of Markdown files with identifiers and cross-references inside VS Code, including backlinks, which is structurally what a bench is. The Jira and Azure Boards extensions are worth reading as negative examples, since they mostly bolt a webview into the sidebar and feel like a browser in a panel.

## What the extension refuses

No button runs an agent, ever, because that is what the name refuses and it is what every competitor in this space already is. No cross-bench analytics or portfolio rollups, per the gitk precedent that a local viewer and a hosted product answer different questions. No multi-user presence. The extension talks to the binary over the machine surfaces, and its menus come from affordances rather than a hardcoded list.

## Convergence

The tree wants dinah-151, saved queries want dinah-135, live refresh wants dinah-120, and the webview rung wants dinah-152. Building the extension pressures all four into being right, and it pressures the affordances block into completeness, because any verb whose affordance the library fails to report becomes a visibly missing menu item.
