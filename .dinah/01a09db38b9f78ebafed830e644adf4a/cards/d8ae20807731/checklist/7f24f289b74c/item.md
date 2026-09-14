---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:18:40Z
ordinal: 3
note: TestTheAmbiguousCardRefusalNamesCandidatesThatResolve asserts the set of 12-hex identifiers in the refusal equals the set of card directories carrying the number, read off disk, then runs path against each and requires a path under that identifier's own directory.
---
The refusal names every candidate and each named candidate resolves. Against the CLI fixture in the spec, the test collects the 12-hex identifiers printed in the refusal's rows, asserts the set equals the set of card directories carrying that number in the fixture, and then for each identifier runs `runCLI(t, root, "path", <id>)`, requiring exit 0 and a path under that identifier's directory.