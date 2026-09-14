---
kind: decision
state: resolved
ts: 2026-09-14T02:18:41Z
ordinal: 16
note: Every card reference from both heads arrives at that one function, through resolveReferenceBody, resolveBelowLanding, probe, or the two exported wrappers, and the CLI's only direct resolver calls at cmd/dinah/commands.go:1227 and :1263 descend into it as well. The head-side plumbing for a carried set already exists on both surfaces, so no head changes at all. A rule enforced in one head and not the other is the defect this workbench keeps finding, and one raise site makes that shape impossible here.
---
The rule is enforced once, in resolveCardIn, rather than in either head.