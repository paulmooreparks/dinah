---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:18:03Z
ordinal: 6
note: "Both arms planted and observed at c25c20f. Deleting the line `internal/guide/guides/references.md` from cmd/dinah/testdata/dinahpath-allowlist.txt failed with `references_guide_test.go:369: internal/guide/guides/references.md:4 carries the name DinahPath and testdata\\dinahpath-allowlist.txt does not name that file`, which is the fixture being read rather than the paths being written into the test. Restored, then added `docs/specs/no-such-file.md`: it failed with `references_guide_test.go:323: testdata\\dinahpath-allowlist.txt names docs/specs/no-such-file.md and no such file stands in the tree, so the entry is dead`. Restored byte-identically, test ok. The walk itself reads 646 files at this commit, measured by a temporary t.Logf in the test and then removed; the spec's 640 and the reviewer's 639 were both counts at b825059, and no test or criterion quotes the figure."
---
Delete the line `internal/guide/guides/references.md` from `cmd/dinah/testdata/dinahpath-allowlist.txt` and run the same test. It fails naming references.md as a file carrying the name that the allowlist does not carry, which proves the fixture is read rather than the paths being written into the test. Then add a line naming a file that does not exist, `docs/specs/no-such-file.md`: it fails naming that entry as dead.