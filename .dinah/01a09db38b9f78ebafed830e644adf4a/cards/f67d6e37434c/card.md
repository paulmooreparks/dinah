---
title: "Rung three: the integrations VS Code gives away"
column: a2eb2436b77d
state: ready
severity: minor
priority: soon
tier: frontier
workstreams:
  - 58f3e3eb621a
links:
  - kind: relates_to
    to: 049295437f6e
  - kind: spawned_from
    to: 81b9d0726b36
---
Sub-card of dinah-185. Depends on the scaffold, dinah-263. This is the rung that dinah-264 and dinah-266 both refer to as "rung three" and that had no card until now.

Each item here is small on its own and earns its place because VS Code already built it. The card exists so they are not each rediscovered as an afterthought inside a larger rung.

**Status bar.** The held card and its lease countdown, or a count of what needs you. The operator asked for this by name. It is the cheapest ambient awareness available and it is a few lines.

**Command palette.** Every verb reachable through a quick pick. The verb list and each verb's arguments are already declared in the binary, so the palette entries should be generated from what the tool reports rather than typed out, or they drift the first time a verb is added.

**Problems panel.** `dinah check` as a task provider, with its findings as diagnostics. The checker already produces structured findings, so this is a projection rather than an analysis.

**Timeline.** A card's journal events in the view the built-in Git extension already fills with commits. dinah-185 calls this free real estate with no new interface to design, which is exactly right.

**Welcome view and walkthrough.** A welcome view when no workbench is found, and a walkthrough that opens `dinah guide` output rather than a copy of it seeded into the extension. The walkthrough should use the tab mechanism from dinah-270 rather than inventing a second way to show served text.

**Registering `dinah mcp` with VS Code's agent mode** for the discovered workbench. This wires the user's own harness without the extension becoming one, which keeps it on the right side of the refusal that no button runs an agent. Worth stating explicitly in the spec that registering a server is not running an agent, because the distinction will be questioned.

## What the spec has to settle

**Which of these are actually one card.** Six items with no dependency between them may want splitting once their real sizes are known. Say so rather than delivering a bundle where one item did the work and five were mentioned.

**Command palette generation.** If the verb surface cannot be enumerated from a machine form today, that is a finding worth reporting rather than a reason to hardcode a list.

**Agent-mode registration is the one item with a real boundary question.** It touches the product's central refusal. Get it right in prose before writing it in code.

## Out of scope

The tree (dinah-265), Needs You (dinah-269), served text as tabs (dinah-270), the LSP (dinah-264), the webview (dinah-266). Drag-and-drop moves belong with the tree rather than here. The holdings and aging-WIP views belong with the saved-query mechanism in dinah-269.
