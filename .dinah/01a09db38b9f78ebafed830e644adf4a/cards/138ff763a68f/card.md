---
title: A duration Dinah cannot parse is dropped without a word
column: 5ea2db0272fc
state: ready
severity: major
priority: next
---
`assignValue` in `internal/mcp/mcp.go` handles the `expires` argument by parsing it and keeping the result only when the parse succeeds:

    case "expires":
        if parsed, err := verb.ParseDuration(value); err == nil {
            req.Expires = parsed
        }

A value the parser rejects therefore reaches the library as no expiry at all. The call succeeds, the card is claimed with no lease, and the caller is told nothing. An agent that asks for `"expires": "8 hours"` rather than `"8h"` believes it has taken a lease that will lapse and has taken one that never will, which is the failure mode a lease exists to prevent.

The command-line head does not behave this way. `runClaim` and `runPull` both call `verb.ParseDuration` and hand the error to `s.reportError`, so a person typing a bad duration is refused and told. The two surfaces disagree about the same argument, and only one of them is right.

What the fix has to settle is what the machine head does with a bad argument in general, because `expires` is the only parameter whose assignment can fail and the switch has no way to report one. Either `assignValue` gains an error return that `request2Args` turns into a refusal, or the parse moves to where a refusal can be raised. Whichever it is, the answer should be the same refusal a person gets at the terminal, since a pull refusing differently from a claim for the same reason is the second vocabulary this codebase keeps refusing to grow.

`TestEveryDeclaredParameterReachesTheRequest` in `internal/mcp/mcp_test.go` does not catch this: it sends `8h` for `expires` precisely so that it tests reachability rather than parsing. Worth extending once the behaviour is settled.

Found at Agent Code Review on dinah-181, cycle 3.
