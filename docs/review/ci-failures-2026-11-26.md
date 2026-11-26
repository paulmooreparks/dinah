# CI failure repairs

The investigation uses the logs and artifacts of [build run 45670785197](https://github.com/paulmooreparks/dinah/actions/runs/45670785197), whose source was `56a1d3f5`. The dependency and compiler upgrades are delivered separately. This branch repairs the remaining checks without changing the product UI.

## Browser expectations

Both failing `chat-cleanup` assertions required British-style date strings. The CI artifact shows fully populated rows containing `Jun 1, 2024` and `Dec 1, 2026`, rather than the expected `1 Jun 2024` and `1 Dec 2026`. A local run with an explicit U.S. locale reproduced both failures in seven seconds. The test now supplies a valid locale, waits for regional initialization and compares the correct row cells with the configured date formatter and number formatter. Its workspace, provider, count, state and retention-message assertions remain intact. The locale can be selected through `DINAH_CLEANUP_LOCALE` for U.S. and British verification.

The `message-boundaries` artifact shows Load Full History disabled when the test attempts to press it. The test had just queued the session snapshot that makes it available. Agent events are applied within the render budget; returning from the emit is insufficient. The test now waits for the actual command to become available before opening its menu. The history-reset and item-identity assertions are unchanged.

The window verification also exposed an obsolete `startup-stable` assertion. It required a frame containing default Notifications windows before the saved layout, although a startup can construct the saved layout directly or finish that intermediate construction between sampled frames. The repaired assertion requires a recorded unfinished layout behind the covering start screen. Every assertion about the final saved layout, visible geometry, pointer access and absence of movement remains unchanged.

## Isolated provider fixture

`TestTheEarlierStageOfAContinuedChatIsArchived` failed while preparing the managed applet integration because it could not find an installed command program. Its provider is a stand-in, but starting its Devin chat still validates the command's location. The test now supplies a disposable command beside a simulated window and clears PATH for the test. Both the archived and refused cases pass with Go 1.26.10 in 7.457 seconds. The fixture file is never executed, and production command discovery is unchanged.

## Review guards

The same ten ESLint errors reproduce locally. The repairs remove unused destructured bindings and write-only counters, preserve their surrounding control updates, and give the live-log trimming variable its own name. ESLint and the approved-path npm audit pass after the changes.

The guard job also stopped before running any browser tests because its manifest still named `thinking-control`, which commit `5159c48a` replaced with `effort-control`. The manifest now names that documented successor. No listed suite is missing.

The original run log and downloaded artifacts remain in `%TEMP%/dinah-ci-45670785197-failed.log`, `%TEMP%/dinah-ci-45670785197-browser` and `%TEMP%/dinah-ci-45670785197-guards`. The local reproduction is retained separately in `%TEMP%/dinah-ci-chat-cleanup-before`. No branch was pushed and no product was installed.

The repaired `chat-cleanup` and `message-boundaries` suites pass together in seven seconds with the U.S. locale. `chat-cleanup` also passes in six seconds with the British locale. Their separate receipts remain in `%TEMP%/dinah-ci-browser-final-us` and `%TEMP%/dinah-ci-browser-final-gb`.

All ten neighboring suites pass in twenty seconds, including the corrected startup expectation, continuous tool retention, command registry, request drafts and races, judgement settings, workspace toolbar and template interactions. Their receipt remains in `%TEMP%/dinah-ci-neighbours`. The untouched-master startup run passed in eleven seconds; the timing-dependent failure and that passing baseline are both retained rather than attributed to a framework change.

The complete review guard run passed with Go 1.26.10, including all 69 browser suites in 193 seconds. Its log and individual reports remain in `%TEMP%/dinah-ci-final-guards.log` and `%TEMP%/dinah-ci-final-guards`.

A subsequent combined dependency gate exposed an unrelated 30-second deadline in the Ollama round-count fixture. It cancelled after 75 requests, already beyond the historical 60-round cap. The helper now uses the test lifetime context while retaining every real tool call. The resumed-turn assertion also requires normal completion after all 101 requests instead of accepting cancellation. The whole Ollama package passes with Go 1.26.11 in 54.527 seconds; its log remains in `%TEMP%/dinah-ollama-rounds-fixed.log`. The original failed gate remains in `%TEMP%/dinah-wails213-go12611-gate.log`.
