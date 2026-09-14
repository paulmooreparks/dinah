---
title: A column can be added, changed, and reordered by the tool
column: 5ea2db0272fc
state: ready
severity: minor
priority: next
---
The tool can retire a column, through `archive` and `delete`, and both refuse while a card stands in it. It cannot add one, change one, or reorder the flow. `dinah workbench set` reaches three fields of the workbench and none of its columns. So the first act of any change to a workbench's shape is editing `workbench.md` and a `column.md` by hand, with `check` as the only thing standing between the editor and a broken flow.

Columns are definition, and definition edits are unjournaled by design, since definition history is git's when the workbench is versioned. This card keeps that. Nothing here writes a journal line.

## The shape

`dinah column add <title> [--kind <kind>] [--after <column>] [--operator-owned] [--capacity <n>]` writes a `column.md` with a minted identifier and slug, on the same path `Instantiate` already uses, and inserts the identifier into the ordered list. With no `--after` the column lands before the terminal region.

`dinah column get|set <column> <field> [value]` reaches title, kind, capacity, operator_owned, awaiting_outside, and reject_to, on the grammar `workbench`, `workstream`, and `card` already share. Leaving the value off clears a field that may be absent.

`dinah column move <column> --after|--before <column>` reorders the list.

The position rules `check` reports today, an intake column first, a done column in the terminal region, and a buffer in neither, are refused at write time rather than reported afterwards, on the same posture the format takes for a `reject_to` naming a column ahead of itself. A refusal names the rule and the column.

The MCP head gets the same tool from the generated schema, with nothing written twice.

## What it does not touch

No card moves. A column added under live cards affects none of them, and a column changed under live cards changes what those cards' column means and where they stand in it, which is the ordinary consequence of editing definition. Retiring stays `archive`, with its occupancy refusal.

A change to the whole shape at once, with cards carried from retired columns into their replacements, is a separate card that composes these acts.
