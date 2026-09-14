---
kind: decision
state: resolved
ts: 2026-09-14T02:16:35Z
ordinal: 14
note: "Share it. parseArgs's inner loop is split into walkFlags(argv, valued, known, onPositional, visit, onUnknown), which both parseArgs and scanLangFlag call. A second, independent reading of argv is exactly the drift this repository already has cards about, and it is also what let the review blocker happen: a pattern match against the bare word \"--lang\" cannot know that the word is sitting in --state's value slot, because that fact lives only in the value-slot decision walkFlags makes. Sharing the decision means the two callers cannot disagree about which word belongs to which flag; they can only disagree about what they each do once walkFlags tells them."
---
Should scanLangFlag pattern-match --lang against argv on its own, or share the value-slot logic parseArgs already has?