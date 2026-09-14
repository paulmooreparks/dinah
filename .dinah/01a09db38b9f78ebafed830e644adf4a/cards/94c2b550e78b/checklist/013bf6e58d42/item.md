---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:18:31Z
ordinal: 2
note: "Verified at 5a88bec. The assertion is `if len(prose) == 0 { t.Errorf(\"the guide %s yields no sentence...\") }` inside the per-topic loop of TestNoGuideDeniesACommandTheToolHas. Armed by setting `prose = nil` for topic workbench-layout at the call site; the run went red naming that topic, while the total silently fell to 508, which is exactly what a corpus total would have hidden. Restored byte-identically and green. The passing run logs each guide's count: first-session 49, getting-started 34, verbs 25, principles 76, references 77, query 88, workbench-layout 18, mcp 159, total 526, re-measured on this branch and matching the spec."
---
The sweep asserts a per-guide sentence floor rather than a corpus total: it fails naming the topic when any single guide yields zero sentences, and it logs the per-guide counts alongside the total. Today the counts are first-session 49, getting-started 34, verbs 25, principles 76, references 77, query 88, workbench-layout 18 and mcp 159, totalling 526.