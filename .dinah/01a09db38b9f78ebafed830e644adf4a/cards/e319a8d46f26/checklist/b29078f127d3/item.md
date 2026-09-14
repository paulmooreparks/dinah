---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:18:48Z
ordinal: 20
note: "This is the invariant the whole design turns on, asserted directly against the module that is supposed to establish it by construction. The comparison at the moment the assertion runs is between the returned `report.entries` array and the `rows` array the test passed in, index by index, so it checks attribution rather than arithmetic: a run that lost row three and gained a duplicate of row four adds up and fails here. The combination count is computed as 4 raised to the row count, summed over lengths 0 to 5, and asserted against what the generator emitted, because a universal claim over an empty set is true and a generator with an off-by-one bound is the ordinary way to get one. Red run to produce at Test: make `runOverRows` skip appending an entry when the act answers `skipped`, and watch the length assertion redden on every run that generated a skip while the length-0 run stays green."
---
A unit test drives `runOverRows` over row lists of length 0 through 5 with every combination of `done`, `failed`, `skipped` and throwing acts, and for each run asserts `report.selected === rows.length`, `report.entries.length === report.selected`, and that the entry at each index carries the reference `refOf` produced for the row at that index. It asserts the number of combinations it exercised against a computed total, so a generator that produced nothing fails.