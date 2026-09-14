---
kind: decision
state: resolved
column: 0d86ad99cdbc
owner: holder
ts: 2026-09-14T02:18:07Z
ordinal: 20
note: "The guide at internal/guide/guides/references.md:77 says attach takes a comment or an attachment below a card. That is true at 22a35fc and stops being true when an attachment reference needs --replace, so this card falsifies it and this card fixes it; dinah-457 may land before or after and cannot be relied on. The workstream is a separate matter: the guide's command table has no workstream column at all, so nothing in it becomes false when attach starts refusing one, and giving the workstream form its place belongs to the card that rewrites the grammar. The param.attach.ref catalog entries are left alone because they already qualify the attachment case with `with --replace`, so the guard makes them more true rather than less."
---
The references guide's attach sentence is corrected on this card rather than left to dinah-457, and the workstream still gets no row there.