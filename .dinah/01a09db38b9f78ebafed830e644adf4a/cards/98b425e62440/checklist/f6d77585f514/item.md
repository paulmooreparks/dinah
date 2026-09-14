---
kind: acceptance_criterion
state: verified
column: c9428b3bc921
ts: 2026-09-14T02:16:34Z
ordinal: 3
note: "Direct run: `dinah --nosuchflag --lang` (DINAH_LANG cleared, no config lang) exits 2, prints only the English contract.Usage refusal naming --nosuchflag, no second refusal about --lang."
---
dinah --nosuchflag --lang (an incomplete --lang following the failing word, nothing after it) exits 2, reports only the contract.Usage refusal naming --nosuchflag, and renders it in English, with no second refusal about --lang anywhere in stderr. Verified with t.Setenv("DINAH_LANG", "") and no lang key in the fixture's config.