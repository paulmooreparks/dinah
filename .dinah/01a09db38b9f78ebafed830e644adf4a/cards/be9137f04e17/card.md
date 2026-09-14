---
title: "two claims left standing on the enumeration fix: a comment that contradicts itself and a directory nothing uses"
column: 5ea2db0272fc
state: ready
severity: minor
priority: soon
tier: minimal
workstreams:
  - 4fd7a9f0b8ff
links:
  - kind: spawned_from
    to: 846c6d6fefda
---
dinah-312 landed with two items its second code review found and deliberately left alone, and the operator moved the card on knowing about both. Neither touches behaviour. Both are the shape that card was filed about, which is a written claim nothing stands behind.

## The comment contradicts itself

The doc comment on `bench.Enumerate` in `internal/bench/bench.go` says "the one `.dinah` this listing reads is the root's own, opened by the root probe rather than reached by descending", and its next sentence says "A `.dinah` belonging to a directory further down is read the same way, when that directory is itself the entry the walk is testing." Both cannot be true. The code reads a `.dinah` for every directory it tests, so the first clause is the false one and the second describes what happens.

The provenance is worth keeping, because it is the reason this is worth a card rather than a shrug. dinah-312's repair pass corrected a false claim in that same comment, which said the walk descends into a `.dinah` when it skips every dotted name, and the correction introduced this one. A comment on the function that card exists to change is the worst place on the tree to leave a sentence that misdirects the next reader, and two consecutive passes have now put a wrong sentence there.

## The directory nothing uses

The end-to-end test in `cmd/dinah/vocabulary_test.go` creates a `cards` directory under the board it builds, by way of `os.MkdirAll(filepath.Join(board, bench.CardsDir), 0o755)`. Nothing reads it, no assertion depends on it, and no comment says why it is there. The reviewer removed it, reran the test green, and put it back rather than changing code at a review station.

Dead setup in a test is not free. A reader deciding what that test covers counts the fixture as evidence of intent, so a directory built and never used suggests a case the test does not actually exercise.

## What this card owes

Correct the comment so it states one rule that matches the code, and remove the directory or give it a sentence saying what it is for. Reread the whole `Enumerate` comment rather than patching the contradicting clause alone, since the passage has now been wrong twice in two passes and the next reader deserves a paragraph that survives a full read.
