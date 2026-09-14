---
kind: decision
state: resolved
column: 0d86ad99cdbc
ts: 2026-09-14T02:18:15Z
ordinal: 22
note: "Library.Show's bare-card branch calls lapseRead, which expires a lapsed claim and writes an expired event. An archived card is out of the flow by construction, so a read command writing to an archived journal is a surprise, and the guard is one condition. The contents statement answers dinah-456 section 4.1 honestly: the walk's root resolves in the mirror and its children sit in the archived entity's own live collections, so the references those rows carry are the addresses the children will have once the root is restored and they do not resolve while it is archived. One line on the listing, from contents.archived, plus Tree.Archived on the machine view, is cheaper and truer than either suppressing the references or printing a screen of addresses that quietly do not work."
---
show skips its claim-lapse write for a card resolved out of the archive mirror, and contents under --archived says on the listing that the addresses below the root resolve once the root is restored.