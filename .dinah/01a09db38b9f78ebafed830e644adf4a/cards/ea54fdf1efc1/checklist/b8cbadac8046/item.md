---
kind: decision
state: resolved
column: 0d86ad99cdbc
ts: 2026-09-14T02:18:10Z
ordinal: 23
note: Every non-prose field is a frontmatter key, and `set <ref> note -` reading a multi-line pipe would otherwise write a header no parser reads back, silently, on a store this project treats as append-only history. Refusing is the only answer that leaves the anchor readable. `malformed` already carries the field name as its detail and already splices, so this costs one key rather than a name; the splice exists because "is missing, empty, or will not parse" does not tell a reader that the newline was the problem. AC-7's second half pins both directions, the refusal on a value with a break and the acceptance of the same value without one.
---
A value carrying a line break is refused on every field that is not prose, under `malformed` with a splice saying why.