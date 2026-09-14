---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:17:18Z
ordinal: 3
note: Re-run at Test. get_workbench_document(id=47, fields="id,title,body") returned the full 199,737-byte body; get_workbench_document(id=52, fields="id,title,body") returned the full 132,579-byte body (current size, including the ratchet entry). Neither call truncated or errored; both bodies matched the body_bytes reported by list_workbench_documents.
---
get_workbench_document(id=<id>) still returns the full body for every part (regression check that the split did not break the route that already worked).