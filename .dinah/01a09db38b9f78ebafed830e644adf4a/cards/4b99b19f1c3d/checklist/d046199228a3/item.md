---
kind: decision
state: resolved
column: c9428b3bc921
ts: 2026-09-14T02:17:18Z
ordinal: 8
note: That code is Andoneer platform server code, not in the dinah repository (grep -ril "list_workbench_documents" --include="*.go" . over the dinah checkout returns nothing) and not reachable by any tool this workbench's agents hold. Out of scope for this card. If the operator wants that route hardened, it needs a card against the Andoneer platform's own project; this card does not file it.
---
Do not attempt a server-side fix to list_workbench_documents/list_project_documents (the "make the failing route refuse cleanly" candidate).