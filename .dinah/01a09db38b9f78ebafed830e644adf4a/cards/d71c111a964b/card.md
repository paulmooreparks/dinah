---
title: the "no external dependencies" claim is stale in two places
column: 5ea2db0272fc
state: ready
severity: minor
priority: soon
---
Two places in the tree assert that dinah has no external dependencies. Neither is true. `go.mod` requires `golang.org/x/term` and `golang.org/x/text`, with `golang.org/x/sys` indirect, and `go.sum` exists. The dependencies arrived with the row-rendering work.

**The workbench document "Go style standard", section 6.** "The standard library first" opens: "The module has no external dependencies and no `go.sum`." The rule the sentence introduces is still right and still wanted, namely that a card adding a dependency names it in WHAT SHIPPED, says which standard-library route it considered, and says what made that route insufficient. Only the opening claim is stale.

It matters because the sentence is load-bearing rather than decorative. An implementer reads "the module has no external dependencies" as a hard prohibition, and will either not propose a dependency that is genuinely the right answer, or will conclude the whole section is out of date and skip the rule that follows.

**`internal/mcp/mcp.go`, the doc comment on `Serve` (around line 74).** "The transport is the standard library's: encoding/json over bufio, which covers the whole of what this head needs, so the module keeps its record of no external dependencies." The first half is accurate and worth keeping; the closing clause claims a record the module no longer holds.

Found on dinah-120, where the spec agent checked the claim against `go.mod` rather than believing it, and the design reviewer then found the second copy. Both wants killing in one pass, which is why they are one card.

Rewrite the standard's opening to say what is true: the module keeps its dependency surface deliberately small, currently three `golang.org/x` modules, and every addition is justified in WHAT SHIPPED against the standard-library alternative. Say whether the `golang.org/x` family is treated as ordinary third-party code or as a near-standard tier, because a reader cannot tell today and the answer changes what a future card has to argue. Trim the mcp.go comment to the claim it can support, that this head needs nothing beyond the standard library.

While in there, check the rest of the style standard for the same class of staleness, since this one survived unnoticed for months, and grep for any third copy of the claim.
