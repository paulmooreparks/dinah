---
kind: decision
state: resolved
column: c9428b3bc921
ts: 2026-09-14T02:17:13Z
ordinal: 10
note: The tree's grouped state breakdown is a Dinah tool feature (the tree verb), not a wire-format concept; core-profile.md never mentions a tree or grouped display at all (grep -n -i "\btree\b" docs/spec/core-profile.md matches only its own citation-policy sentence). CORE-CARD-5 fixes every card's literal state regardless of column, and format.md:288 already glosses ready as "pullable," which a card at an intake column or buffer literally is. This card changes only which of those wire-legitimate values a display groups by, per column kind, which the format leaves to the tool. Same ground as dinah-275's D-2/D-3, which touched no doc file beyond tree.go/bench.go's own comments for the parallel change.
---
No change to docs/spec/core-profile.md or docs/design/format.md.