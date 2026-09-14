---
kind: decision
state: resolved
column: 4b38abe7ebd5
owner: holder
ts: 2026-09-14T02:18:12Z
ordinal: 41
note: "Spec section 3.1's per-field table gives every field a \"stored as\" cell equal to its own name, and for `column.capacity` that is false against the tree: `bench.loadColumn` reads `wip_limit`, which is the profile's word for the limit, while `dinah column new` calls the flag that writes it `capacity`. Writing the frontmatter key `capacity` succeeded, journalled and read back as nothing, which is how the round-trip sweep found it.\n\n`Field` therefore carries a `Key`, empty on every field stored under its own name, and `Field.Stored()` resolves the two so no caller has to remember which. One field uses it today.\n\nThis is a spec prediction found wrong rather than a change of scope: the declaration still records where the value lives, which is what section 3.1 set out to record, and it now records it correctly."
---
A column's `capacity` is stored under `wip_limit`, so `bench.Field` gained a stored key and the spec's own table is wrong on that one row.