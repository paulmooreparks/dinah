---
title: Settle on one word for what a user runs, and say it everywhere
column: 5ea2db0272fc
state: ready
severity: minor
priority: soon
workstreams:
  - 58f3e3eb621a
---
Dinah currently calls the same thing by two names. The word "verb" runs through the guides, the design documents and the core profile, while the CLI's own reader-facing catalogue barely uses it and VS Code's surface is called a command palette. Ruling the palette's copy on dinah-420 exposed the split: two adjacent rows in the same menu were about to say "verb" and "command" for one idea. The operator ruled that surface to "command" and asked for the word to be harmonised across the docs, the help, the tool and the rest.

This card is that harmonisation. It is mostly a judgement call about where the boundary sits rather than a search and replace, and the judgement has to be made before any text is edited.

What the word touches today, from a sweep of the tree. Three guide topics use it: internal/guide/guides/verbs.md throughout, plus mcp.md and principles.md. The CLI's English catalogue, internal/msg/locales/en.json, uses it in four places. It appears in README.md, docs/quick-start.md, four design documents and the core profile at docs/spec/core-profile.md, alongside a long tail of per-card UX sketches under docs/specs/.

The questions that have to be answered before anything is edited, and none of them should be assumed:

Whether "verb" is a contract term or reader-facing prose. docs/spec/core-profile.md is the document other implementations are written against, and if the contract names its operations verbs then that word is part of what Dinah publishes, not copy this card is free to change. The likely shape of the answer is that the contract keeps its own vocabulary while everything a person reads says command, but that is exactly the ruling this card owes rather than a premise it may start from.

Whether `dinah guide verbs` keeps its name. The topic name is a vocabulary token that travels on the wire and appears in the MCP schema's published members, so renaming it is a compatibility change with a deprecation question attached, not a rename of a file.

Whether the archived UX sketches under docs/specs/ are edited at all. They are a record of what was designed on a given day. Rewriting them makes the historical record say something it did not say, which argues for leaving them alone entirely.

What the tool's own eight message catalogues do, since any English change to a reader-facing string drags seven translations and a fingerprint recomputation behind it.

Out of scope, ruled on dinah-420 already: identifiers. Message keys, command ids, filenames and Go and TypeScript symbols keep the spellings they have. Nothing in any code path reads either word, and renaming symbols would bury a copy change under a large diff.

Also out of scope: the two VS Code manifest titles and the palette's own strings, which dinah-420 is changing on its own branch.
