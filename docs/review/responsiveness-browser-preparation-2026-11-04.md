# Browser timing snapshot preparation

The browser implementation follows the reviewed timing snapshot contract, including the final disclosure header and the preparation-only ten-second deadline. It lives on feat/responsiveness-timing-snapshot in .worktrees/fe-responsiveness-snapshot. No installation, restart or push is authorized or performed.

## Verification

The unit guards pass for canary exclusion, closed schemas, populated backend fields, unsupported values, disabled hot paths, raw-write rate limits, aggregate preservation, ring wrapping, suspension coverage and the UTF-8 copy limit. The browser checks pass for recording controls, serialized acknowledgment, stale generations, stalled reads, late settlement, one outstanding preparation, a fresh copy after settlement and clipboard completion after recording changes.

The memory check uses Chromium CDP collection and heap measurements only in the test browser. At 3,000, 6,000 and 12,000 simulated periods, each sample retained 600 aggregate slots and 200 raw records per source. Serialized sizes were 795947, 795947 and 796750 bytes. Used heap was 5963604, 5957380 and 5970836 bytes, a 13456-byte spread. This establishes bounded retained test storage; it does not measure live desktop memory.

The existing responsiveness suite and new snapshot suite passed together. The final snapshot suite, including memory and deadline checks, passed separately. Production JavaScript lint and syntax checks pass. Logs are retained under the temporary directory in dinah-responsiveness-snapshot-deadlines.log, dinah-responsiveness-snapshot-memory.log and dinah-snapshot-focused.

## Pending gates

Three alternating recording-on/off pairs at zero, one and four synthetic streams are running against frozen production files. The source manifest and every raw result are retained in the temporary directory under dinah-responsiveness-overhead-20261104. Early results vary and do not establish a pass. The browser regression suite is queued through the same shared lock, with its output in dinah-responsiveness-full-20261104.log.

The overhead harness uses a schema-valid backend control stub. It measures browser recording overhead, while Engine owns the actual backend microbenchmarks and integration review. No live input stall was captured by these tests, and no live relief is claimed.


## Review corrections

Quality reproduced two blockers in eabaf5c. A delayed enable acknowledgment could collapse intervening off/on intent into an idempotent enable and preserve old backend storage. The corrected control loop retains a single reset obligation until a disable acknowledgment completes. A delayed observer report at the modulo boundary could also replace a newer slot; eligibility and export now share the 600-bucket retained window, and slot replacement rejects older buckets. Regression checks retain a backend sample before the delayed acknowledgment and verify its exclusion, then exercise delayed input, task and frame reports at wrap while preserving the current maximum and raw-write budget.

The earlier overhead run, aggregate browser run and queued integrated checks were stopped for these production corrections. Their artifacts remain evidence for the earlier source only, and establish no completed gate. The corrected snapshot suite is registered as a mandatory browser guard and invokes its unit checks before launching the browser. Corrected unit checks and production lint pass. Browser and overhead execution must be repeated against the corrected source.

The corrected unit guards, production lint and the paired responsiveness/responsiveness-snapshot browser suites pass. The latter ran in 36 seconds, including deadline races, wrap/memory checks and the delayed acknowledgment reset test. Corrected output is retained in the temporary directory under dinah-snapshot-corrected and dinah-snapshot-corrected.log. The old partial overhead run retains 14 completed cases plus incomplete-run-status.json; it cannot satisfy the three-pair gate.

The integrated focused race checks pass on corrected 6ba0ae3 for internal/responsiveness, internal/bridge, cmd/dinah-desktop and internal/mcpserver, using the Responsiveness|Timing|Overflow selection. The existing multiple-manifest linker warning appeared before the desktop tests passed. The complete log is retained in the temporary directory as dinah-responsiveness-integrated-6ba0ae3.log. The corrected full browser and overhead runs remain pending.


## Completed corrected validation

All 211 browser suites passed in 714 seconds on the frozen corrected source. The log is retained as dinah-responsiveness-full-6ba0ae3.log and artifacts as dinah-responsiveness-full-6ba0ae3 in the temporary directory. This includes web-skins, the snapshot suite and its synchronous unit guards. The earlier incomplete run remains separate.

All eighteen corrected overhead cases completed with zero page errors. Production file hashes match source-manifest.json. The harness preserves the retained benchmark workload, runs three on/off pairs per load, reverses pair order in the second round, and observes Event Timing and Long Animation Frames in both modes. Style/script timings are Chromium CDP ten-second totals; key p95 is the existing keystroke-to-next-frame probe. The backend is stubbed, so these results cover browser recording overhead. Backend microbenchmarks remain separate.

| Streams | Pair | Script off/on ms | Style off/on ms | Key-to-frame p95 off/on ms |
| --- | --- | --- | --- | --- |
| 0 | 1 | 0 / 0 | 0 / 0 | 14 / 14 |
| 0 | 2 | 0 / 0 | 0 / 0 | 13 / 11 |
| 0 | 3 | 0 / 0 | 0 / 0 | 12 / 13 |
| 1 | 1 | 350 / 387 | 669 / 696 | 18 / 19 |
| 1 | 2 | 430 / 431 | 813 / 818 | 20 / 19 |
| 1 | 3 | 413 / 445 | 815 / 833 | 18 / 15 |
| 4 | 1 | 1070 / 1072 | 1834 / 1930 | 25 / 24 |
| 4 | 2 | 1213 / 1256 | 2338 / 2298 | 35 / 35 |
| 4 | 3 | 1205 / 1264 | 2320 / 2403 | 34 / 35 |

- At 0 streams, script medians off/on are 0/0 ms, ranges 0..0/0..0, and paired deltas [0, 0, 0] ms.
- At 0 streams, style medians off/on are 0/0 ms, ranges 0..0/0..0, and paired deltas [0, 0, 0] ms.
- At 0 streams, key p95 medians off/on are 13/13 ms, ranges 12..14/11..14, and paired deltas [0, -2, 1] ms.
- At 1 streams, script medians off/on are 413/431 ms, ranges 350..430/387..445, and paired deltas [37, 1, 32] ms.
- At 1 streams, style medians off/on are 813/818 ms, ranges 669..815/696..833, and paired deltas [27, 5, 18] ms.
- At 1 streams, key p95 medians off/on are 18/19 ms, ranges 18..20/15..19, and paired deltas [1, -1, -3] ms.
- At 4 streams, script medians off/on are 1205/1256 ms, ranges 1070..1213/1072..1264, and paired deltas [2, 43, 59] ms.
- At 4 streams, style medians off/on are 2320/2298 ms, ranges 1834..2338/1930..2403, and paired deltas [96, -40, 83] ms.
- At 4 streams, key p95 medians off/on are 34/35 ms, ranges 25..35/24..35, and paired deltas [-1, 0, 1] ms.

The differences between on/off medians satisfy the numerical ceilings. However, one-stream paired script deltas are 37, 1 and 32 ms, so their median is 32 ms against a roughly 21 ms five-percent allowance. The script and style baselines also vary materially across pairs. This does not establish a variance-resolved overhead pass. The overhead gate remains unproven pending Quality's assessment; no release clearance follows from the median-only comparison. Raw Event Timing, long-frame entries and all pair results remain in dinah-responsiveness-overhead-6ba0ae3.
