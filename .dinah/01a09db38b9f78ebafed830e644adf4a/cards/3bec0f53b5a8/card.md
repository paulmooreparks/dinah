---
title: the isolation guard proves nothing on any machine that is already clean
column: 5ea2db0272fc
state: ready
severity: minor
priority: next
workstreams:
  - 4fd7a9f0b8ff
---
dinah-229 clears seven environment variables before the test binary runs, and guards that clearing three ways. Two of the three strands fail on any machine. The third, the one that proves the clearing actually happened, only bites where one of those variables is already set. That is the operator's machine and no CI runner, so on every automated run it asserts something that is trivially true and reports success either way.

dinah-229 states this openly rather than hiding it, and Agent Code Review accepted it as a known limit rather than a defect, which is the right call for that card. It is still worth closing, because it is the exact shape dinah-229 exists to prevent: a check that passes without checking. Today the gap is covered by the operator's own machine happening to be dirty, which is a fact about one box rather than a property of the suite.

The reviewer notes there is a standard Go technique that would make the strand bite everywhere, and that it is already written down in this board's own "Convention counterexamples" document. Whoever picks this up should start there rather than inventing something: setting the variable deliberately inside the test and proving the clearing removes it makes the assertion true of every machine, and it turns a check that depends on the host into one that does not, which is dinah-229's whole thesis applied to dinah-229's own guard.

A second, smaller item from the same review, in the same files and worth doing in the same pass. A doc comment in the test-support package was swallowed by the new test and now describes the wrong function. No tool catches that: gofmt does not read comments for sense and vet does not either, so it will sit there until a person reads it and is misled. The affected file is `internal/testenv/testenv_test.go` around lines 155 to 186, and the machine-conditional strand lives in `cmd/dinah/main_test.go` and `internal/bench/check_test.go`.

Sequence after dinah-229 merges, since both items are edits to what that card lands. Neither is urgent: the suite is genuinely green on this machine now and the two failures every agent was briefed to ignore are gone. What remains is that one of the three guards is load-bearing only here.
