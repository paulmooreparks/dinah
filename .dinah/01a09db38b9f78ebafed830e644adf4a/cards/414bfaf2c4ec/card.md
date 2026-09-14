---
title: The English-in-source guard cannot follow a value that reaches a map by a second name
column: 5ea2db0272fc
state: ready
severity: major
priority: next
tier: frontier
---
Operator ruling, 2026-08-26, on dinah-282's second escalation: "accept the card and file the two holes as their own card." This is that card.

## What the guard is for

A check in `internal/profile` stops an English phrase written in Go source from reaching a user through a refusal's named values, where it would bypass the locale catalogs entirely and appear untranslated to everyone.

## Two holes, both demonstrated rather than reasoned

**A value that reaches the map by a second name is invisible.** The guard decides a map was fully read by matching writes against the name the map was declared under. Go lets another name reach the same map, so an alias, a pointer handed to a filler, and the map parked in a struct field each write English into it with the guard silent. A reviewer ran all three against a real refusal site, confirmed each compiles, and restored the file between runs.

**A phrase assembled rather than written whole is invisible.** The guard decides a value is English by asking whether the whole expression is a string literal. So `"no workbench was found beneath " + path` and `fmt.Sprintf("no workbench was found beneath %s", path)` both slip through, and both are English somebody typed. The codebase already writes that concatenation shape at `internal/bench/bench.go:974`.

## Why this is its own card rather than more cycles on dinah-282

Four cycles went into this guard, and each closed whatever spelling a reviewer had just demonstrated while staying open to whatever nobody had written yet. That is not carelessness in any round. It is what happens when dataflow is attempted with an AST walk: Go has more ways for a value to reach a container than a pattern can enumerate, so the guard is always one spelling behind whoever is looking.

dinah-282's own deliverable, the cross-head and roster and argument layers, was found clean across three reviews. It only touched this guard because its new maps tripped it.

**The second hole is not a bounded fix.** Looking inside an expression needs a test for what counts as a phrase, and `", "` contains a space. That is a design question rather than a repair.

## What this card should settle before writing any code

**Whether an AST walk is the right instrument at all.** It has now failed four times in the same way. The alternative within the same approach is the real type-checking machinery, which can follow a value through aliases and fields because it understands them.

**Or design the problem away, which is likely cheaper.** If a refusal's named values could carry only catalog keys rather than free strings, by construction, English could not get into them and the guard would be unnecessary rather than merely correct. A guard exists to catch what the types permit; changing what the types permit removes the need for it. Weigh that first, because it converts an unbounded analysis problem into a bounded refactor.

**What counts as a phrase**, if the expression route is taken. Establish it against real values rather than in the abstract: the reviewer read what these maps actually carry and found `strings.Join`, `filepath.Join`, `strconv.Itoa` and struct fields, so a rule refusing every non-literal would redden a correct tree in 33 places.

**Whether the guard's current reach is written down.** dinah-282 ships it with limits nobody has stated. A guard whose reach is unstated is one people assume is total, and this batch has been about not doing that. Whatever this card decides, the interim state should say what the check does and does not see.

## One thing this card should establish rather than assume

Whether the shape-based check that existed before dinah-282 caught any of these five spellings. If it did, dinah-282 shipped a narrow regression and that is worth knowing. If it did not, all five predate it. Nobody has checked, and the answer changes how urgent this is rather than what it should do.

## Related

`Convention counterexamples` carries two entries from this work: "A construct recognised in the spellings its author enumerated" and "A resolver that treats reaching the container as having read its contents", plus a correction to a sentence in the second that licensed the assembled-phrase miss. Read all three before starting.
