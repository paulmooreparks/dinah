---
kind: decision
state: resolved
owner: holder
ts: 2026-09-14T02:17:42Z
ordinal: 28
note: "The block at internal/guide/guides/principles.md:29 shows invented data, with an intake column holding one card and a doing column at 2/2, and no run reproduces those numbers. The defect dinah-446 corrected there was the header row, since the renderer grew a Work column and the guide did not. Round 1 also compared the separator row, which is wrong: the separator's widths follow the widest cell in each column, so a lesson that chose a wider example would redden a check that has found nothing. Comparing the header alone catches the class the audit found and cannot cry wolf. The heading text comes from the catalog key column.columns.work in internal/msg/locales/en.json, which is what AC-12 edits to arm the check."
---
The guide's one rendered table is held by its header row alone, and its separator row and data rows are held by nothing.