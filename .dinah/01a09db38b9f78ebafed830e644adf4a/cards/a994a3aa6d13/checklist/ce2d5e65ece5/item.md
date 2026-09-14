---
kind: open_question
state: resolved
owner: holder
ts: 2026-09-14T02:16:52Z
ordinal: 43
note: "Moot, and resolved rather than deleted so the reasoning stays on the record.\n\nThis question existed only because an earlier ruling accepted slugs as well as paths, which created a collision case and a coupling to the cached enumeration. The operator has since withdrawn that ruling: the workbench property takes a path alone, on the ground that a machine caller should be unambiguous. A path names one directory, so there is no collision to answer and no ambiguity refusal to reuse or mint.\n\nThe second half of the question dissolves with it. Address resolution reads the filesystem through a stat rather than the cached slug set, so the cache's staleness no longer reaches resolution. What remains of that staleness belongs to the listing alone and is recorded on OQ-2.\n\nNothing here needs building, and no follow-on card is owed. If slugs are ever admitted to this surface, this item is the record of what admitting them costs."
---
What answers a slug that matches more than one workbench under the root, and does slug resolution read the cache or the filesystem?