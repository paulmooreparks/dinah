---
kind: decision
state: resolved
column: c9428b3bc921
owner: operator
ts: 2026-09-14T02:16:54Z
ordinal: 21
note: "Operator ruling, Paul, 2026-08-27: \"Oh... 'ahead of the column's.' Yes, that's fine.\"\n\nThe substitution is ratified. The paragraph is not reopened.\n\nThe implementer was right to ask rather than deviate silently, having been told the wording was the deliverable, and right about the substance too. The word comes from the profile's own live text rather than from anyone's preference: CORE-INSTR-5 reads \"A tool MUST serve a workbench's standing instructions ahead of the column's instructions\" at docs/spec/core-profile.md:1021, verified on trunk. No state carries instructions any more, so shipping \"the state's\" would have named a thing that does not exist, inside the paragraph whose whole purpose is to stop a reader concluding something false. That paragraph had already been corrected once at design review for the same species of defect.\n\nThe obligation is unchanged either way. What the paragraph licenses is still a further instruction layer in any position that leaves the workbench's standing text ahead of the other layer's.\n\nThe spec was corrected to match what shipped before this ruling, so the document, the spec and the criteria now agree. The three further stale snapshots the implementer noted, AC-3's and AC-4's counts and OQ-4's quoted index row, were corrected in the same pass, and the two criteria were reworded to require that this card move nothing rather than to name figures that go stale on the next card."
---
The section 7 paragraph ships reading "ahead of the column's", not the spec's "ahead of the state's".