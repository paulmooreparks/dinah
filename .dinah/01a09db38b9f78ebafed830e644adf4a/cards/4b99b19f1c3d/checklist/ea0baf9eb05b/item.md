---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:17:18Z
ordinal: 1
note: Re-verified at Test, 2026-09-04. list_workbench_documents(workbench="dinah", fields="id,title,body_bytes,version") now shows id 47 "Convention counterexamples 1" at 199,737 bytes and id 52 "Convention counterexamples 2" at 132,579 bytes (grew by 1,816 bytes since implementation, a documented ratchet entry from code review's AC-2 finding). Both still well under the 262,144-byte ceiling.
---
Every workbench document titled "Convention counterexamples N" (N = 1, 2, ...) is at or under 262,144 bytes, verified via list_workbench_documents(fields="id,title,body_bytes") reading body_bytes for each, not by inference from the split method.