---
kind: decision
state: resolved
ts: 2026-09-14T02:16:31Z
ordinal: 14
note: "--json has no dinah config set equivalent today. Giving --format one and not the other would be an inconsistent surface, and matching the existing two-rung precedent avoids inventing a new persisted setting this card was not asked for."
---
Format has no persisted config-file layer. Resolution is flag over DINAH_FORMAT only, via bench.Resolve, with no third rung.