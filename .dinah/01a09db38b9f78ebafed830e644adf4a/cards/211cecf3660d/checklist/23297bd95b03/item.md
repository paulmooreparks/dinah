---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:18:27Z
ordinal: 7
note: "Re-verified live on both surfaces against a scratch workbench with two hand-edited items. Text: `dinah check` printed one finding line per item, each ending \"...: &lt;card&gt; &lt;item-id&gt; &lt;stored-value&gt; (&lt;path&gt;)\", closed with \"2 defects.\" at exit 5. `dinah check --json` reported the same two entries under \"findings\" with Key \"check.item-column-unresolved\" and the three-token Detail. No separate wiring: report.Findings feeds both."
---
dinah check, both the text-rendered and --json surfaces, against the same fixture reports the new finding on both surfaces with no separate wiring beyond Check()'s own Findings slice.