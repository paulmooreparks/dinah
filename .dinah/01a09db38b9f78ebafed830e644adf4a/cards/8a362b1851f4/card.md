---
title: "dinah serve: the HTTP head over the library"
column: 5ea2db0272fc
state: ready
severity: minor
priority: soon
---
Dinah's architecture puts every surface over one library, and two of the three heads exist. The CLI and the MCP server both ship; `dinah serve` is described at length in `docs/design/surfaces.md`, including its REST discipline, its media types, and its use of revisions as ETags, and no command implements it. Specs are now accumulating routes with nowhere to land, dinah-135's query route among them, and each one either invents its own conventions or defers them to a head nobody has built. The work is the head itself and the answers it has to give once for every verb rather than once per card: how a refusal maps to a status code, how the basis rides a request, and how discovery picks the workbench a server serves.
