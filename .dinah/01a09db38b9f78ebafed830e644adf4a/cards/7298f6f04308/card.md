---
title: "The ui pages: server-rendered HTML over the verbs, one frontend for every container"
column: 5ea2db0272fc
state: ready
severity: minor
priority: soon
links:
  - kind: relates_to
    to: 017d7be2bdd2
---
The pages work was split out of dinah-283 after the operator ratified, on 2026-08-29, the two rulings that shape it. The first keeps the ruling the surfaces design already made: the pages are server-rendered from the binary, every card a URL, every act a form posting to a route, and no client-side framework. The second settles the fork the parent card left open: there is one HTTP process, and the pages ride the same routes the REST surface serves, with the representation chosen by media-type negotiation, so `dinah ui` is `dinah serve` plus embedded assets plus opening a browser. The GUI boundary section of docs/design/surfaces.md is rewritten as part of this card's work, so the rulings land in the design document when the work that embodies them lands.

## What the pages render

The main view is the tree verb's node shape, the same node the sidebar rung already renders, so a page is a renderer of nodes rather than a second grouping of them. A card's actions are the response's affordances block verbatim, which keeps an illegal action absent from the representation without the page writing a rule of its own. A claim or a move answers with the served instruction chain, and the page shows those layers the way the machine heads do. The Tela Design Language arrives as CSS, the medium the hosted product already renders it in. Progressive enhancement holds, so a page works before any script loads and no act requires one.

## The command log rides the pages

Every act the pages perform is logged as the CLI command that would have performed it. The entry is generated rather than written at a call site: a renderCommand beside the parameter table in internal/verb/definition.go composes the spelling, because that table is already the single definition the CLI syntax line and the MCP schema project from, and every parameter there already declares the Request field it binds. A round-trip test, parsing the rendering back into the same request, is the drift guard in miniature. Entries are copyable because they are real command lines, and re-runnable as fresh acts against current state; a stale re-run is refused by the basis guard, and the refusal is the lesson. The log records the request while the journal records the events, because one pull is two journal events and no flag ever reaches the journal.

## Two behaviours the spec must carry

The pages poll the changes verb with its since cursor and re-render only when something happened, because agents mutate the workbench underneath them as the normal case. The actor on a click defaults to the operator, with the act-as-owner spelling the CLI already carries.

## What this card waits on

The first-cut route roster that dinah-152's spec chooses is chosen so these pages can start on it, with the same routes answering text/html to a browser and the machine media types to a client. If 152 slips, the fallback is a ui-only head rendering straight from the library and marked to converge onto serve's routes later, and that shape stays a fallback rather than the plan because a second HTTP head over one library is the drift the guard dinah-282 landed now watches for.

## Who consumes the result

The browser through `dinah ui` is the first container. dinah-266's webview rung embeds these pages instead of building a frontend of its own, and dinah-283's desktop shell points a window at the same handler, so no container builds a frontend of its own.

## Scope inherited

The pages serve a single seat working one workbench at a time, the ceiling the gitk precedent sets in the surfaces design, and no button runs an agent. No portfolio view, no multi-user board, no live coordination, no analytics; where that ceiling is reached the answer is the upgrade path to the hosted product.
