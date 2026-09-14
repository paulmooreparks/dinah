---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:18:17Z
ordinal: 5
note: Verified at 7d50d5b. The sweep declares dinah.is-a-collection for the nine collection shapes and dinah.unknown-path for workbench/cards and workbench/columns, and TestEditHandsTheEditorTheFileTheResolverNames runs `dinah edit` for workstream/nosuch, "" and fx-99 in that order, requiring stderr to lead with dinah.unknown-workstream, unknown-card and unknown-card and requiring exit 2 with no editor launch recorded. Arm 5 discards the entity resolver's error and reports ResolvePath's, which is what keeps workstream/nosuch from becoming an unknown card.
---
Every refusal `edit` raises today is spelled the same way after the change. The sweep's declared refusal names cover `dinah.is-a-collection` for a collection shape and `dinah.unknown-path` for `<slug>/cards` and `<slug>/columns`, and `TestEditHandsTheEditorTheFileTheResolverNames` additionally runs `dinah edit` for `workstream/nosuch`, for the empty reference, and for a bare slug naming no card, requiring stderr to lead with `dinah.unknown-workstream`, `unknown-card` and `unknown-card` in that order and requiring the refused exit code.