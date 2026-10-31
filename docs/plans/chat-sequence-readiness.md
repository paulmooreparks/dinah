# Chat Sequence Suite Readiness

Release reported two failures of the first chat-sequence assertion on master e153534. The original assertion compared one read of the delivered sequence with its expectation, then fetched the sequence again for the failure detail. One detail was empty; the other was the expected sequence, g1:1,g1:2,g1:3. During that phase the observation list only appends. The latter result therefore establishes that delivery was incomplete at the comparison and complete by the subsequent diagnostic read. It does not establish duplicate or reordered delivery.

The suite previously waited 250 ms after emitting events. The cooperative render queue applies agent events before the sequence listener observes them, and a fixed delay does not establish that application has finished. The correction waits up to five seconds for the expected delivered sequence, then retains a 250 ms observation interval for a repeat or stray event. A failed wait still reaches the equality assertion and reports the observed sequence. Log-read checks likewise wait for the requested read count. Expected and observed values now come from one snapshot for each sequence assertion. The first phase also writes emit/delivery times, frame observations, visibility and log calls to chat-sequence-initial.json in the run's output folder.

## Comparison Evidence

The instrumented suite retained the original 250 ms wait during twenty interleaved isolated runs on each build. All forty runs passed. The event-routing, sequencing and scheduler source files were identical between the two builds. The fixture supplies no team state for the team's additional execution-state repaint to draw.

| Build | Runs | Failures | Minimum dispatch wait | Mean dispatch wait | Maximum dispatch wait |
| --- | --- | --- | --- | --- | --- |
| Installed baseline cbd5766 | 20 | 0 | 3.1 ms | 20.5 ms | 132.4 ms |
| Candidate 845d275 | 20 | 0 | 6.1 ms | 41.9 ms | 149.4 ms |

These waits measure the first emitted event to its delivery. The candidate was slower on average in this isolated comparison, but neither build reproduced the assertion failure. An instrumented full candidate run delivered the sequence correctly after 183.7 ms; that run failed chat-unread separately. The baseline full run delivered it correctly after 25.9 ms, but its three extension suites failed because the temporary checkout lacked the sibling extension checkout. That baseline run is partial evidence, not a passing full gate or an equivalent clean comparison.

The corrected sequence suite was present in the Front End full browser gate on 31 October 2026. All 156 suites passed in 258 seconds on production candidate a4e9e63, whose parent is Engine a5f1764, plus this then-uncommitted test correction. The correction is committed separately from the provider-switch implementation. It changes no production event routing, sequencing or rendering behavior, and the passing gate does not cover later Engine changes.

Raw comparison output remains in .worktrees/front-end/.diagnostics/sequence-matrix. The instrumented full runs remain in sequence-full-diagnostic-absolute and sequence-full-baseline beside it. The latest passing gate is switch-full-gate. These local artifacts supplement the recorded figures; they are not installed-app verification or a product latency guarantee.
