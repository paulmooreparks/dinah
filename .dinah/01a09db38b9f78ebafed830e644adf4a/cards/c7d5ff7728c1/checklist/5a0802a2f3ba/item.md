---
kind: decision
state: resolved
column: 0d86ad99cdbc
owner: holder
ts: 2026-09-14T02:18:01Z
ordinal: 33
note: The operator asked for the surfaces question to be answered again for a proper noun, which is a stronger claim than the descriptive phrase it replaces. Spec section 13.2 rules eight surfaces one at a time with a reason each. The two rows worth naming here are the ones where the answer was not obvious. The guide's title and filename stay "References", because `references` is the key in the `guides` map in internal/verb/definition.go, it is what `dinah guide references` takes, and it is what the MCP head serves the guide as a resource under, so renaming the guide would rename a machine surface to gain a word the guide's own first sentence already carries. The seam paragraph of 12.4 and its reciprocal in query.md carry no proper noun on either side, because naming one side alone makes the two look unequal and naming both puts two proper nouns into a paragraph whose subject is the difference between an address and a condition. What stops the name spreading by use is section 13.3's guard, which fails both on a hit outside a declared allowlist fixture and on zero hits in the references guide; the second arm exists because a one-directional guard on a name is satisfied by deleting the name.
---
DinahPath appears in design documents and in the references guide's opening paragraph, and on no other surface: not in the guide's title or key, not in the rest of the guide, not in help text, not in refusal text, not on any machine-surface field, and not in the seam paragraph.