---
kind: decision
state: resolved
column: 0d86ad99cdbc
owner: holder
ts: 2026-09-14T02:17:29Z
ordinal: 30
note: "Agent Design Review found on 2026-09-11 that round 1's spec said both new strings were covered by the guard at `manifest.test.ts:875` and that this was true of one of them. The setting's `markdownDescription` is inside the corpus, because the collector walks `configuration.properties`. The provider's `label` is outside it, being neither the manifest description, nor a setting's description, nor a welcome block.\n\nCovering it is the cheaper of the two repairs and the more honest one. The collector gains a loop over `contributes.mcpServerDefinitionProviders` and the `declared` expression gains that array's length, which keeps the guard's derived half and its literal half telling the same story; the alternative, writing down that the label is unguarded, leaves a reader-facing string outside a guard whose whole purpose is that no reader-facing manifest string claims the extension ships a binary. `resolveNls` at `manifest.test.ts:84-105` already walks the entire manifest, so the collector sees the English rather than the `%key%`, which is what makes the extension a two-line change rather than a new resolution path. AC-2 carries the count and the plant that proves the extension fires on a label rather than only on the count."
---
The guard that refuses a manifest string claiming the extension carries a binary grows to cover the provider label, rather than the label being declared unguarded.