---
title: blockValue's array reader drops every field but the first on a multi-key dashed entry
column: b69abf918c42
state: ready
severity: minor
priority: next
tier: workhorse
workstreams:
  - b3f924406e4c
links:
  - kind: relates_to
    to: 5ef07a3b83a3
---
`arrayFromChildren` (internal/bench/blockjson.go) reads a frontmatter block's array shape by looking only at each dashed entry's own line; `dashedValue` then splits that one line on its first colon into a single name/hint pair. Anything indented beneath a dashed entry is silently dropped rather than folded into that entry.

This is invisible for a sequence of bare names or one-field hints (the shapes `levels:` and today's callers use), but it means the reader cannot correctly parse a sequence whose entries carry more than one field each, for example the `citations:` shape docs/design/format.md documents (`scheme`, `target`, and an optional nested `observed` mapping per entry). dinah-435 (D-1) scoped citations out of its read surface specifically because of this gap, having first (incorrectly, then corrected on review) thought no block reader existed at all; blockValue does exist and handles the mapping-of-mappings shape `evidence:` needs (see dinah-206's D-4/OQ-1), so this is the one real remaining gap in that reader.

Fixing `arrayFromChildren` to gather each dashed entry's own deeper-indented follower lines into that entry (mirroring how `objectFromChildren` already gathers a mapping member's deeper children) would let a future citations-reading card build on the existing machinery instead of writing a second one. Filed as its own card per this workbench's scoping convention: a reader shared across future callers is separate, useful-on-its-own work rather than something folded silently into whichever card first needs one more field.

## Branch

dinah-438-blockvalues-array-reader-drops-every-field-but-the-first-on-a-multi-key-dashed-entry
