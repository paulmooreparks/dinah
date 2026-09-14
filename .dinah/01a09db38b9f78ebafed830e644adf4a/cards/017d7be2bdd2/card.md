---
title: "Rung four: the webview, once there is an HTTP head to embed"
column: 5ea2db0272fc
state: ready
severity: minor
priority: later
workstreams:
  - 58f3e3eb621a
links:
  - kind: spawned_from
    to: 81b9d0726b36
---
Sub-card of dinah-185, the last rung of the ladder. Parked behind dinah-152.

dinah-185 says the webview rung comes last and embeds the server-rendered ui pages once they exist. Those pages are dinah-152, "The HTTP head: dinah serve implements the REST surface", which is still in Intake. So there is nothing to embed and this card cannot start.

It is filed now rather than later for one reason: the inventory reads the Jira and Azure Boards extensions as negative examples, because they mostly bolt a webview into the sidebar and feel like a browser in a panel. Recording that judgement against a card means the rung arrives with its own warning attached rather than being reinvented by somebody who has not read the inventory. The trap here is not technical.

## What it will have to settle, when dinah-152 lands

Which pages are worth embedding at all, given that rungs one through three deliberately use native VS Code surfaces (tree, LSP, command palette, Problems panel) precisely because they feel like the editor rather than like a browser. A webview that duplicates the tree earns nothing and costs the thing the earlier rungs bought.

What the webview does that a native surface cannot. If the honest answer turns out to be nothing, this card's correct outcome is to be closed unbuilt, and saying so now is cheaper than discovering it after the work.

How it behaves when the HTTP head is not running, since `dinah serve` is a process somebody has to start and the rest of the extension works without it.

## Out of scope

Everything below it on the ladder. This card does not start until dinah-152 has shipped a surface to embed.

The refusals on dinah-185 bind hardest here, because a webview is the easiest place to break them by accident: no button runs an agent, no cross-bench analytics or portfolio rollups, no multi-user presence.
