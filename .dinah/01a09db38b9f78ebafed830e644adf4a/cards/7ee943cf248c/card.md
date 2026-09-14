---
title: "Rung one: the LSP tells you what you broke while you are editing"
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
Sub-card of dinah-185, rung one of the ladder. Depends on the scaffold card.

The format document promises that a person may edit a bench by hand and then ask the tool whether they broke anything. dinah-185 records the thesis: the LSP turns that promise into immediate feedback, and that is the whole feature. Everything else on this rung is a consequence.

The inventory names what it covers, and it should be read there rather than restated. In outline: diagnostics for the invariants the checker already knows, plus the two lints the surfaces design named; completion drawing closed enums from the token registry and open sets from the bench itself; hover showing a token's registry definition and the profile statement governing it; go-to-definition from a card's state id to its `state.md` and find-references inverting it; rename made safe by identity living in the hex identifier; a formatter once canonical frontmatter key order is ruled; and code actions for the quick fixes, one of which can invoke the CLI to record a manual-correction event.

## The question this card actually has to settle

**Where the knowledge lives.** Every diagnostic here is something `dinah check` already computes. An LSP that reimplements those invariants is a second implementation of the contract, which is the thing this product refuses everywhere else. So the design has to say whether the language server shells to the binary, embeds it, or is the binary under another flag, and what that costs in latency when the feedback is supposed to be immediate. This is the card's real content; the feature list is the easy half.

**What is checkable without a whole bench.** A person editing one `card.md` in an editor may be nowhere near a workbench, or in a broken one. Diagnostics that need the bench opened behave differently from diagnostics that read one file, and the split decides what still works in the degraded case.

## Two things the inventory could not know

`awaiting_outside` landed on trunk (dinah-201) and is a state declaration parsed strictly: exactly `true` or `false`, anything else refuses the workbench as malformed. That is a diagnostic and a completion, and it is the newest member of the closed set completion draws from, so it is worth using as the worked example that proves the registry-driven path rather than a hardcoded list.

`dinah query` shipped (dinah-135) across CLI, MCP and HTTP. Find-references over cards standing in a state is a query rather than a walk, so check whether that surface answers it before writing a traversal.

## Out of scope

The tree, the views, the webview, the free editor integrations. The CodeLens verb actions on a card file are arguably this rung by dinah-185's own wording ("the LSP plus the verbs"), so this card either takes them or hands them to a sibling explicitly; it does not leave them unclaimed.

The refusals on dinah-185 bind: no button runs an agent, and the menus come from affordances rather than a hardcoded list.
