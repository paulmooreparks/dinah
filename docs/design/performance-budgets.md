# Performance budgets

Every other test in this repository runs against a workbench of a few cards, so a read that opens every file in the store passes them all. The `perf` job in `.github/workflows/ci.yml` closes that gap. It runs `TestReadBudgets` in `internal/perfstore`, which generates a workbench the size of Dinah's own development workbench from a fixed seed, confirms `dinah check` finds nothing in it, and holds four reads to budgets.

## The four reads

| Operation | What the test times |
|---|---|
| `status-warm` | Opening the workbench and running `status` inside the test process |
| `show` | Opening the workbench and running `show perf-1` with no fields named |
| `page-card` | The head's handler answering `GET /cards/perf-1` as HTML, with no socket |
| `status-cold` | `dinah status` in a fresh process, from start to exit |

Each operation runs once to warm up and then ten times, and the test judges the median of the ten. Every run also checks its own answer, so a read that starts failing quickly cannot pass on speed.

## Reading a failure

Every operation prints one line to the job log whether it passed or not, carrying its median, its fastest and slowest run, its budget, and the CI median and card that set the budget. When an operation fails, the log carries a block like this one:

```
status-warm over budget: median 1,412ms (retry 1,388ms) against a budget of 1,200ms
  runs:  1,390 1,401 1,405 1,410 1,412 1,415 1,420 1,433 1,450 1,502 ms
  retry: 1,370 1,377 1,380 1,385 1,388 1,390 1,395 1,398 1,402 1,460 ms
  budget set by dinah-621 at 3.0x a CI median of 396ms
  reproduce: DINAH_PERF=measure go test -count=1 -run '^TestReadBudgets$' -v ./internal/perfstore
  the store is seed 20260926, DevelopmentShape, digest <the digest>
```

The test measures an operation a second time before it fails it, so both medians in the first line were over the budget. A pull request that changed nothing about the operation does not usually fail twice in a row, and when one does, the runs line shows whether every run was slow or a few runs pulled the median up.

A slack report means the reverse. The operation ran more than six times under its budget twice in a row, and the report names the budget the rule below would give it.

## Reproducing it locally

Run the command the failure names:

```
DINAH_PERF=measure go test -count=1 -run '^TestReadBudgets$' -v ./internal/perfstore
```

The first line of the log carries the seed, the file count and the digest of the generated store. When your commit matches the one CI ran, your digest matches the one in the CI log, and that is how you confirm you are measuring the store CI measured.

Your numbers are judged against the Windows CI budgets, so a faster machine passes with slack and a slower one can fail. `measure` mode never fails on slack for that reason. CI runs in `ci` mode, which also fails on slack. With `DINAH_PERF` unset the test skips, and any other value fails it.

If you want the store for profiling, set `DINAH_PERF_KEEP` to an empty directory. The test then generates into that directory rather than a temporary one, logs the workbench's root, and leaves it in place. The probe attached to dinah-618 runs against that root unchanged.

## The rule that sets a budget

A budget is three times the median the `perf` job measured on CI, rounded up to the next 10ms, and never below 30ms. The multiple the log prints is the budget divided by that median.

The median comes from three runs of the `perf` job, never from one. Re-running the job on the pull request is enough, and the basis is each operation's median across the three runs' medians. A single slow run makes a bad basis, because every ordinary run afterwards sits more than six times under the budget it set and the slack check then fails pull requests that changed nothing.

A card that makes an operation faster sets that operation's row by this rule in the same pull request, with its basis taken from three runs on its own pull request, and names itself in the row's `setBy`. In `ci` mode the slack check enforces this: a gain large enough to leave a budget more than six times over the median fails the `perf` job until the row is tightened. Loosening a budget needs a justification a reviewer can read in the pull request, and `setBy` records who did it.

## The band a pull request lives in

With a basis `b` and a budget of about `3b`, a pull request that does not touch an operation fails over budget only when both of its medians exceed `3b`, and fails on slack only when both fall below about `b/2`. No single median can trip both checks, because that would need a median `m` with `m > budget > 6m`. In the ordinary case the band is six times wide.

Rounding narrows the lower edge when the basis is small. A basis of 11ms gives a budget of 40ms, and slack below about 6.7ms. Once a budget reaches the 30ms floor the slack check stays silent, because it exempts budgets of 30ms and below. When a run leaves the band with no code change behind it, recalibrate from three runs under the rule and say why in the pull request. Do not widen the multiple.

## What a green run proves

The budgets measure speed on Windows only. The ordinary suite tests correctness, the generator's own small-shape tests included, on Linux, macOS, and Windows. The `perf` job runs on one platform because a budget is a number about one runner image, and because a file open costs more on Windows than elsewhere, so a regression in opens shows there first and largest.

That leaves a gap you should keep in mind. A change that slows only Linux or macOS passes this job. One example is a cache keyed on paths built with the host's separator, which never hits on one platform and is still correct, so every test stays green. A card that adds a cache closes the gap for its own cache by asserting hit behaviour in its ordinary tests, such as an entry reused or a file not read again, and those tests run on all three platforms. The `perf` job cannot close it, because counting file opens needs a seam in `internal/bench` that this harness does not add.

On a platform with no budgets pinned, `measure` and `ci` modes still generate the store and log every number, and then the test skips, so you get numbers without a verdict nobody calibrated.
