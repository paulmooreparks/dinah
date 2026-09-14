---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:18:30Z
ordinal: 9
note: "Read docs/design/format.md:450-454 diff directly: corrected sentence describes `dinah set <column> hold on|off` / `get`. Swept both format.md and core-profile.md for gate_items; every remaining hit is CORE-JSON-10 interchange description or the declaration section, none claim hand-only."
---
docs/design/format.md's sentence claiming gate_items is not settable except by hand is corrected to describe `dinah set <column> hold on|off` / `dinah get <column> hold`; a search of docs/design/format.md and docs/spec/core-profile.md for "gate_items" shows every remaining mention is either the CORE-JSON-10 interchange description (unaffected) or the corrected creation-time sentence.