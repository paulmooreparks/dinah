---
kind: acceptance_criterion
state: verified
column: c9428b3bc921
ts: 2026-09-14T02:17:10Z
ordinal: 4
note: "Rerun: `go test ./cmd/dinah/ -run 'Vocabulary' -v` and the whole `go test ./cmd/dinah/` (green, 84s). buildTreeFixture now plants atRoot at <root>/.dinah/<hex> and nested at <root>/customer/project/.dinah/<hex> via plantInContainer (no rename), while sibling and current stay on plantBare. All thirteen tests built on the fixture pass unchanged, with no assertion weakened. Armed with the named mutation: `benchIn(full, true)` in walkFor (internal/bench/bench.go) turns TestTheVocabularyMigrationWalksTheWholeTree red at vocabulary_test.go:275/278/281/285 for the nested container-shape workbench only (\"does not open: dinah.needs-vocabulary-migration: dinah-core/0.6\"), while the bare sibling and the root-probed atRoot stay green. That split is expected: benchIn's anchor check runs before the skipBase branch, so skipBase can only cost a container-shape find."
---
TestTheVocabularyMigrationWalksTheWholeTree and every other test built on buildTreeFixture pass against the reworked fixture: atRoot and nested found via their new .dinah-container paths, sibling still found via its unchanged bare path, current still reported as needing nothing. Must go red if walkFor's existing per-child recognition regresses (e.g. skipBase passed true for children).