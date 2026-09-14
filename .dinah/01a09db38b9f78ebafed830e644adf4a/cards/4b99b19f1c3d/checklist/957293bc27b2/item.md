---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:17:18Z
ordinal: 7
note: "Re-confirmed at Test. list_workbench_documents(workbench=\"dinah\", fields=\"id,title,body_bytes,version\") shows the four other documents unchanged: id 48 \"Prose standard\" 15,831 bytes v9; id 49 \"Go style standard\" 12,389 bytes v2; id 50 \"Working safely against the operator's own data\" 3,550 bytes v3; id 51 \"Translation staleness contract\" 36,234 bytes v9. Same ids, titles, byte counts and version numbers as recorded at implementation."
---
The other four workbench documents (Prose standard, Go style standard, Working safely against the operator's own data, Translation staleness contract) are untouched: same ids, same titles, same body content as before this card.