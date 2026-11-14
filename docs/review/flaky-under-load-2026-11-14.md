# Timing-dependent test corrections

Quality prepared this change on `quality/flaky-under-load` from master `fb650e775ca40c12fe6019117580373f41ab6382`. The lead supplied the two failing assertions from full test runs that overlapped other agents' testing. Each affected test had subsequently passed three times alone.

## Local file save counterexample

`TestWithoutTheLockOverlappingSavesBothSucceed` failed with `overtook false, first <nil>, second the resource has changed since it was read`. Its helper held the first writer immediately before replacement, started a second writer with locking disabled, then released the first after 400 ms regardless of the second writer's progress. If the second writer had not checked the revision by then, the first writer changed it and the second correctly returned a conflict. The fixture had stopped reproducing the intended overlap.

The unlocked counterexample now waits for the second writer's result before releasing the first. Its completion order follows channel synchronization and does not depend on finishing a file operation in 400 ms. Both original success assertions and the final file-content assertion remain. The lock-enabled tests retain their existing bounded exclusion observations; this correction does not lengthen those waits or change file-save behavior. The folders and save-lock namespace were already isolated with `t.TempDir`.

## Bus watcher shutdown

`TestWatcherCallbacksAndTwoPhaseShutdown` failed at `watcher did not run` after 5.96 seconds. The five-second wait covered asynchronous database initialization, waiting for the production one-second ticker, and scheduling the first round. It was a latency assertion in a test of callback ordering and shutdown, with no shared port or folder involved.

The service now has a private watcher starter that accepts a tick channel. The public starter still selects the same production one-second timer. The test sends one explicit tick, waits for the callback to enter, requests stop, then releases the callback and waits for shutdown. The Go test runner's overall timeout bounds hangs. No per-step deadline decides whether correct callback ordering passed.

The test also waits for the service mutex to become available after observing the stop signal. Stop closes that signal while holding the mutex, so the old callback lock assertion could race with the tail of StopWatcher's critical section. The callback remains blocked until that critical section has ended. All callback-order, join, handle-lifetime and refusal-to-restart assertions remain.

## Verification

All commands used the shared FIFO in the isolated worktree. No visible application or desktop automation was used.

| Command | Result |
| --- | --- |
| `go test ./internal/bushost ./internal/resource/localfs -count=10 -timeout=120s` | Both packages passed, 6.788 s and 10.685 s. |
| `go test -race ./internal/bushost ./internal/resource/localfs -count=3 -timeout=120s` | Both packages passed with the race detector, 6.674 s and 4.358 s. |
| `GOMAXPROCS=1`, the two reported test names, `-count=30 -timeout=120s` | Both passed, 6.302 s and 0.945 s. |

The original machine-load failures were supplied by the lead; this investigation did not recreate uncontrolled competing full-suite runs. Repetition is supporting evidence. The correction removes the two timing assumptions identified in the failing paths. It does not claim that every test in the repository is now independent of machine load, and no full repository suite was run.
