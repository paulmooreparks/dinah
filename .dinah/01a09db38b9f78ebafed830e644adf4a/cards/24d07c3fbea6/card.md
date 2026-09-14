---
title: Nothing holds the CLI's message catalogue to the Go code that asks for it, in either direction
column: 5ea2db0272fc
state: ready
severity: major
priority: soon
links:
  - kind: spawned_from
    to: ca954fd7c228
---
dinah-406 closes this gap for the VS Code extension and deliberately declines to close it for the CLI. This card is the declined half, filed with the subject its design review named so that whoever picks it up starts from measurement rather than from scratch.

The gap is the same one. `msg.T("cmd.foo")` compiles and ships whether or not `internal/msg/locales/en.json` carries `cmd.foo`, and a catalogue key nothing asks for goes on costing seven translators. Neither direction is checked today.

What dinah-406's spec author measured, and what its reviewer reproduced independently at `65a80ad`, is why a naive sweep cannot ship. Run over 93 non-test Go files it finds 140 literal key asks and fires ten times, and all ten fires are false. Seven are places where the CLI glues a key together from a prefix (`msg.T("cmd."+name)` in `cmd/dinah/help.go`, `msg.T("reshape.wrote."+...)` in `reshape.go`, `msg.T("column."+...)` in `table.go`). Three are `Has` called on `internal/bench/frontmatter.go`'s own type, which is no message renderer and merely shares a method name. Behind the false fires sit roughly two dozen sites where the key is only known at run time, each needing its own declaration written before the sweep could be honest. The extension had one such site. A guard that lands red on its first run and needs two dozen declarations to quieten it is the shape this board has twice refused, and the refusal was right both times.

So the work here is the declarations first and the sweep second.

**Settle the package count before anything else.** dinah-406's spec names four packages holding the dynamic sites (`cmd/dinah`, `internal/bench`, `internal/mcp`, `internal/verb`); its reviewer, sweeping independently, found three. One of the two readings is wrong and neither is load-bearing yet. Counting by reading a function instead of tracing call paths has been wrong on this board four times, so settle this by two independent methods agreeing rather than by trusting either number here.

**Decide the matching rule up front, because it is the card.** dinah-406 matches a localizer call by the callee's name alone and whitelists no receiver, on the grounds that a missed spelling would skip call sites in silence. Carried to Go, that rule is what produces the three `frontmatter.Has` fires. Matching instead on the type the method is called on would make those three disappear, at the cost of needing type information rather than a bare syntax walk. Pick one, say why, and say what happens to a call the rule cannot classify.

**Copy the declaration shape rather than inventing one.** dinah-406 ships `KEY_FAMILIES`, where a family declares the prefix and the table its members come from and reads the member names off the code, so the names stay written in one place. Both hand-written fields fail loudly when wrong. Whatever the Go side declares should fail the same way.

Two things about the CLI that the extension work established and that belong here. `internal/msg` already has a plural mechanism (`Renderer.TN`, `TestPluralsFollowTheCategories`, 44 plural-category keys in `en.json`), so a plural key's spelling is composed at run time and is one of the dynamic families this card must declare. And an unresolvable key expression must fail rather than be skipped: a sweep that silently passes over what it cannot read reports success over the sites it happened to understand, and that reads exactly like a sweep that checked everything.

Filed by Claude Opus 5 at operator design review on dinah-406, on that card's D-7, which recorded the decline and left the filing to the operator.
