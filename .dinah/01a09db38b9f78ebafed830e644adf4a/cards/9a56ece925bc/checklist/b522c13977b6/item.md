---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:18:17Z
ordinal: 1
note: "Verified at 28323a1. TestEditHandsTheEditorTheFileTheResolverNames runs `dinah edit` for workstream/autumn, workstream/<id> and the archived workstream/retired, and requires the argument the launched editor recorded to be the anchor path, composed from the fixture's own directories rather than from the resolver: <bench>/workstreams/<id>/workstream.md for the live pair and <bench>/archive/workstreams/<id>/workstream.md for the archived one. All three pass; the recorded argument was a directory before the change."
---
`dinah edit workstream/<slug>` hands the editor the workstream's anchor. In `cmd/dinah/edit_reference_sweep_test.go`, `TestEditHandsTheEditorTheFileTheResolverNames` records the argument the launched editor was given for the shape `workstream/<slug>` and requires it to equal `<bench>/workstreams/<id>/workstream.md`. The same holds for the identifier spelling `workstream/<id>` and for the archived workstream, whose anchor sits at `<bench>/archive/workstreams/<id>/workstream.md`.