---
title: dinah board draws the workbench as a board in the terminal
column: 5ea2db0272fc
state: ready
severity: minor
priority: next
tier: frontier
---
Operator, 2026-08-26: "I'd like `dinah board` sooner rather than later, with proper line-drawing characters; maybe even a proper TUI."

The vocabulary ruling of the same day makes this possible to name. A board is a representation of a workbench rather than a competing word for one, so `dinah board` is the command that draws one, and the VS Code tree and the Dinah.Team screen are other drawings of the same thing.

## What it is

A rendering of the workbench as columns side by side with their cards beneath, drawn with box-drawing characters, in the terminal. The surfaces design already anticipates something in this space, saying the in-binary TUI runs in the integrated terminal at every rung of the extension's ladder for free.

## It sits on top of two known renderer defects, and both are load-bearing here

**The output path mangles characters outside ASCII on a Windows console.** The terminal workstream reports that nothing in the repository handles the Windows console: no file calls a console API, and `main.go` hands `os.Stdout` straight through, so the tool writes UTF-8 and a console whose output code page is not 65001 decodes it as its legacy code page. One em dash currently becomes two replacement characters. **Box-drawing characters are the same defect at maximum volume.** A board is mostly box-drawing characters, so on an unprepared Windows console this command would produce a screen of mojibake rather than a board. This card cannot ship before that one and the spec must say so rather than discovering it in test.

**The renderer only breaks the last column.** `formatRow` in `cmd/dinah/row.go` writes the tail whole and calls `breakTail` only when the row carries `wrapTail`, and that operates on the tail alone. A middle column with a long value has no treatment anywhere: the row runs past the window, the terminal wraps where it likes, and the next column's value lands orphaned. dinah-200 reports that on a real workbench today. A board is nothing but middle columns, so the existing layout machinery does not reach it.

The spec should establish whether this card uses that machinery at all or draws its own, and if it draws its own, whether the two then have to agree about anything.

## The design questions

**What happens when the board does not fit.** This is the question the whole card turns on. The operator's own workbench has fourteen columns, and fourteen columns of readable card titles do not fit in eighty characters, or in two hundred. The honest options are drawing a horizontal window over the columns with some way to move it, drawing vertically the way the VS Code tree does, drawing a subset chosen by a filter, or narrowing columns until only counts and glyphs survive. Pick one and say what happens at the widths a real terminal has.

**What the character set is, and what the fallback is.** Box-drawing characters are not universally available: a console using a legacy code page, a terminal with a font missing the glyphs, and output redirected to a file all need an answer. ASCII fallback is the obvious one, and whether it is chosen by detection, by flag, or by both is a decision. **No part of that decision may rest on undocumented behaviour of a console or terminal, and measuring what one does is not a contract.** That rule has already cost this project a full card's worth of rework.

**Whether output redirected to a file or a pipe is the same drawing.** Every other read verb has a machine form and a human form, and the existing renderer already treats a window of zero, meaning redirected output, as no wrapping. A board is a human form with no obvious machine counterpart, so say whether the machine form is simply the tree verb and this command refuses to be piped, or something else.

**Whether it is one drawing or a loop.** The operator raised a TUI as a possibility rather than a requirement. A command that prints once and exits is a different piece of work from one that owns the terminal, redraws, and takes keys, and the second brings raw-mode input, resize handling, and a documented way to leave. Decide which this card is, and if it is the first, say plainly whether the second is a follow-on or a rewrite.

**What a card looks like at board width.** A column narrow enough to fit fourteen of them shows very little. Which of title, identifier, state, severity, priority and holder survive, and in what order they are sacrificed as the window narrows, is the actual design of this feature.

## What it must not do

It must not reimplement what the verb library answers. The board is a head over the same reads every other surface uses, and the tree verb already answers a hierarchy in one response.

It must not become the board UI the CLI's own positioning refuses. That refusal was about not growing a product surface, and the operator has since ruled that a UI is coming regardless, so the spec should restate the boundary in current terms rather than either citing the old sentence or ignoring it.

## Ordering

After the console encoding work in the terminal workstream, which is a hard dependency rather than a preference. The layout defect is a softer one, since this card may draw its own layout, and the spec decides that.

Not before dinah-287, which renames `state` to `column`. A brand-new surface written in the old vocabulary would need renaming in the same week it was written.
