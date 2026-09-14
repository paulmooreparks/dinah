---
kind: acceptance_criterion
state: verified
ts: 2026-09-14T02:17:41Z
ordinal: 21
note: "Re-armed independently by Test on 56db064. Rewriting the entry for docs/quick-start.md:888 to `counts=none holds=prose reason=...` and running `go test ./cmd/dinah -run TestEveryCountedSetIsCountedConsistently` fails with \"testdata\\prose-figures.txt:63: the entry for docs/quick-start.md:888 writes counts=none; write a phrase saying what the figure stands for, because a shared sentinel groups unrelated figures together\". Restored and green. `grep -c \"counts=none\" cmd/dinah/testdata/prose-figures.txt` reports 0 on the shipped ledger, so the second half of the verdict holds too."
---
A `counts=` phrase spelled as the literal `none` is refused. Arming: change the `counts=` phrase of the entry for `docs/quick-start.md:888` to `none`, run `go test ./cmd/dinah -run TestEveryCountedSetIsCountedConsistently`, and watch it fail naming the document, line 888, and telling the author to write a phrase saying what the figure stands for. Restore the phrase and watch the run go green. The verdict also requires that no entry in the shipped ledger writes `counts=none`, which `grep -c "counts=none" cmd/dinah/testdata/prose-figures.txt` reports as zero.