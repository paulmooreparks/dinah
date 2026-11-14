# Member information event count

The test counted two independent information changes as if both described a lead change. Chat creation schedules the chat-state publisher after 400 ms and the team publisher after one second. Under load, the initial chat-state publication can run inside the test's lead assertions. Both publishers send `agent:session`. The original event slice also had no synchronization with those callbacks.

The branch `quality/team-info-twice` starts at master `9386987acfe4ffd2d81277167681d54c7835e2ae`. Only test code changes. The original assertions still require one event for adding a lead, no additional event for an unchanged lead, and a second event for removing it.

## Cause and correction

The fixture now accepts configuration before it creates chats. The focused information tests stop the chat-state publisher and reserve the team timer slot with a stopped timer, so no automatic callback can enter their event capture. They drive publication explicitly. The default fixture retains its existing automatic behavior for the other tests. No wait was extended and no production notification was suppressed.

A deterministic regression publishes the lead, then computes and emits the initial chat state through the same operations used by the chat-state publisher. It requires two events, an unchanged lead, a newly present chat state, and an increasing state version. Repeating both publishers must leave the count at two. This demonstrates the extra legitimate event without relying on machine speed.

The product's lead setter compares and updates under the conversation mutex. A second regression releases sixteen refresh calls together and requires exactly one member-information event. Its event counter is synchronized. The focused evidence supports test interference as the cause; it does not prove that every possible product event interleaving is correct.

## Verification

All commands ran through `scripts/with-test-lock.ps1`, with no visible program or desktop automation.

- The original test and the distinct-change regression passed in 4.706 seconds.
- The original test, distinct-change regression and concurrent-refresh regression passed with `-race -count=10` in 22.825 seconds. The Windows linker printed `.rsrc merge failure: multiple non-default manifests`; the test command exited zero and reported no data race.
- All twelve tests declared in `team_views_test.go` and `team_relays_test.go` passed in 5.569 seconds.
- `git diff --check` passed.

The full repository gate was not run. The reported historical full-gate failure was not rerun under artificial machine load. The regressions exercise the interfering change directly and preserve exact counts rather than depending on a favorable timing window. No push, merge or installation was performed.
