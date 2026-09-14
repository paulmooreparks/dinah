---
title: "The HTTP head: dinah serve implements the REST surface"
column: 2f6c18c9f5d0
state: ready
severity: minor
priority: soon
tier: frontier
workstreams:
  - 58f3e3eb621a
links:
  - kind: blocks
    to: 017d7be2bdd2
---
Of the four designed heads over the verb library, the CLI and MCP exist and the HTTP server does not; there is no serve command at all. Two pending cards already assume an HTTP surface (the query card and the tree-projections card), and the bench home URL promise in the format design expects a contract-conforming instance answering over HTTP, so the gap is now load-bearing rather than cosmetic. The REST shape is the most fully designed unbuilt thing in the repo: Fielding discipline with no invented verbs, entity revisions as ETags with If-Match as the basis guard, stale as 412, the claim as a resource, a move media type carrying the board semantics, and representations listing only the legal next transitions. The card's work is implementing that design as a thin head over the existing library, with the spec deciding the first-cut route roster and which mutations earn dedicated media types.
