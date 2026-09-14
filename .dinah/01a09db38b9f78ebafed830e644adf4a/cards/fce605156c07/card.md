---
title: three resolvers read the environment directly while a fourth already takes it as a parameter
column: 5ea2db0272fc
state: ready
severity: minor
priority: next
---
`ResolveEditorSource` in `internal/bench/config.go` already takes `lookPath func(string) bool` as a parameter, and its doc comment says exactly why: "passed in so a test can drive the ladder without depending on what the machine running it happens to have installed." Whoever wrote that identified the hazard and closed it for the fallback rung. The three rungs above it still call `os.Getenv` inside the function, for `DINAH_EDITOR`, `VISUAL` and `EDITOR`, with no injection point at all.

The same shape repeats beyond the editor. The actor, language and locale resolvers each read the process environment directly the same way. So the codebase has one function that half-solved a problem and three siblings that did not solve it, and a reader cannot tell from any one of them what the house rule is.

Raised while spec'ing dinah-229, which fixes the resulting test leak at the test boundary rather than here. That was the right call for that card and the reasoning belongs on this one: threading the three editor variables in as parameters would fix one resolver out of three and deepen the inconsistency rather than remove it. Doing it properly means all of them, which reaches three exported functions, both call sites and every ladder test, and that is a redesign rather than a paragraph inside a test-isolation card.

This is deliberately not urgent. dinah-229 closes the failure that made the inconsistency visible, so nothing is broken while this waits. What remains is that the next test to depend on one of these resolvers inherits the same hazard with no seam to reach for, and that the codebase gives contradictory guidance about whether ambient state is injected or read.

The spec should decide the house rule and apply it uniformly: whether each resolver takes its inputs as parameters the way `lookPath` is taken, whether the ladder takes a resolved environment value object, or whether ambient reads are legitimate at this layer and `lookPath` is the anomaly to remove. All three are defensible and the point is that one of them becomes the answer everywhere rather than in one function. It should also say what happens to the existing callers, since the exported signatures change under two of the three options.

Sequence after dinah-229.
