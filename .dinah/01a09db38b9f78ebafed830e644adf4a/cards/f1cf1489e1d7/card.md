---
title: A dotted leader carries the eye from a command to its description
column: 5ea2db0272fc
state: ready
severity: minor
priority: next
---
Depends on dinah-220 landing first. That card decides where the description column sits and how a row breaks; this one fills the gap those decisions leave, and specifying it against a layout still in flux would waste the work.

Even after dinah-220 positions the description column against the widest command line, the river survives. `check [--finish] [--migrate-ordinals] [--migrate-slugs] [--migrate-states] [--migrate-workstreams]` is ninety-nine columns wide and `status` is six, so the short rows still cross ninety blank columns to reach their description, and the eye loses the line. Capping the command column so the outlier wraps reduces the distance without closing it, because the spread between `status` and `workstream [new|get|set] [workstream|title] [field] [value] [--yes]` is inherent to the command set rather than to any one row.

The operator's answer is the leader a table of contents uses. His example, verbatim:

```
  add <title> [--state <state>] ..................................................................... File a new card in the first state
```

The rule is unconditional and mechanical. Where a row's command column ends short of the description column, the gap fills with a single space, a run of periods, and a single space. No threshold and no special case for a wide gap against a narrow one; a row ending two columns short gets a shorter run.

Because the leader is fully determined by two measured positions, the golden-output tests stay derivable from the desired blocks in `docs/specs/dinah-220-help-formatting-ux-sketch.md`: take a block, replace the gap with what the rule produces. The operator does not have to redraw a sample, and his hand-drawn output stays the byte-level contract for both cards.

Three details were decided by default when this was first raised, and each is his to overturn at Operator Design Review rather than to discover in the rendering code. A gap under three columns fills with spaces as it does today, since a leader needs a space, a period and a space before it is a leader at all. On a row whose command column wrapped, the leader belongs on the line where the description begins, which is that column's last line rather than its first, and a continuation line of the description gets no leader because nothing sits to its left to connect it to. The global-flags table gets no leaders, since it is narrow, carries its own header rule, and the operator's desired output leaves it alone in every other respect.

One thing genuinely wants his eyes rather than an agent's. Forty rows of eighty-five periods is a great deal of texture on one screen, and he named that risk himself when he raised it. Whether the leader reads as helpful or as noise is a judgement to make against real output, so this card should produce the rendered help at 107 and 200 columns and put it in front of him at Operator Design Review before anything commits to it. The honest alternative, if it does read as noise, is a dimmer glyph or a spaced run rather than a solid one.

Scope is the leader alone. Nothing here reopens what dinah-220 settled about capitalization, option packing, continuation alignment or column placement.
