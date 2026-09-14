---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:17:18Z
ordinal: 2
note: "Re-run at Test, 2026-09-04, both branches of the criterion. list_workbench_documents(documents=\"Convention counterexamples 1\", fields=\"id,title,body\") with the default max_bytes=48000 returned documents:[] with truncated_by:\"max_bytes\" (confirms the failing route still fails). The same call with max_bytes=262144 returned _meta.truncated:false and a full non-empty body for id 47 (199,737 bytes). Identical result for \"Convention counterexamples 2\" at max_bytes=262144: truncated:false, full 132,579-byte body returned. Both parts pass the corrected criterion's own call exactly as written."
---
list_workbench_documents(documents="Convention counterexamples <N>", fields="id,title,body", max_bytes=262144) returns a non-empty documents array with a non-empty body field for each part in turn, one call per part. The explicit max_bytes is part of the criterion because the parameter defaults to 48,000 bytes: the same call without it answers documents: [] with truncated_by: "max_bytes" for every part larger than that, which is both parts as shipped. Rotation puts each part under the 262,144-byte response ceiling; it does not and cannot put a part under the browsing route's default, and the column instructions are what route an agent away from that route.