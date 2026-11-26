# Completion waits on loaded runners

This sweep changes test failure ceilings, not product deadlines. The recovered wake test called TurnIO.Close with three seconds to join all running turns. Close seals admission and waits on the running-turn wait group. Recovery completion includes provider work and durable I/O. The test then proves that the original delivery attempt ended, the recovered chat retains its slot, one new session and one prompt occurred, and a fourth wake cannot exceed the background limit. Those assertions are unchanged.

Desktop completion waits now use the shared testWait ceiling of two minutes. This deliberately permits process startup, scheduling and durable writes under load; it is not a latency requirement or a claim that any correct machine must finish in that interval. The caller still fails on an unmet condition and the package retains a finite watchdog. Existing shared completion helpers inherit that ceiling. The fixture checks still wait on the same channels, receipts, persisted state or predicates, and no retries of failed tests were added.

The sweep inspected time.After, context.WithTimeout, NewTimer, time.Now().Add, sleep loops and fixed iteration polling. It retains explicit cancellation and expiry stimuli, negative observation windows, live-provider limits, synthetic clock changes, terminal shutdown policy checks and actual performance assertions. The two-second cached-service lock test now gets the completion ceiling while continuing to hold the app lock throughout the successful read, so it still proves the read does not acquire that lock. The window-close test similarly retains its held-agent gate while waiting for the reporting call to return.

Document-service discovery and queued-transcript persistence loops no longer count two hundred sleep iterations; they retain their polling predicates under the shared deadline. The quick adjacent-package sweep also covers the already reported applet-host completion and Claude fixture-subprocess waits. Product source is unchanged.

## Browser scope

The browser helper's generic locator readiness ceiling becomes thirty seconds. Genuine performance comparisons and deliberate wait periods remain explicit. Several positive readiness overrides that previously allowed three to five seconds use that same thirty-second ceiling. The extension readiness negative control from the preceding fix established that an unmet ready predicate still fails at its deadline; changing a ceiling does not make an absent predicate pass. Fixed sleep sites that model rate buckets, expiry, debounce or deliberate absence are left unchanged. This sweep does not claim that every sleep in every suite has been removed.

## Changed files

The following inventory records each file in the sweep. All entries are test source or test helpers.
| File | Change |
|---|---|
| `cmd/dinah-desktop/agent_threads_close_test.go` | Positive completion, discovery, drain or fixture-call waits use the shared test ceiling. |
| `cmd/dinah-desktop/applet_consent_test.go` | Positive completion, discovery, drain or fixture-call waits use the shared test ceiling. |
| `cmd/dinah-desktop/applet_host_test.go` | Positive completion, discovery, drain or fixture-call waits use the shared test ceiling. |
| `cmd/dinah-desktop/applet_mcp_test.go` | Positive completion, discovery, drain or fixture-call waits use the shared test ceiling. |
| `cmd/dinah-desktop/applet_request_settlement_test.go` | Positive completion, discovery, drain or fixture-call waits use the shared test ceiling. |
| `cmd/dinah-desktop/applet_standalone_test.go` | Fixed iteration polling uses the shared completion deadline. |
| `cmd/dinah-desktop/applet_tool_permission_test.go` | Positive completion, discovery, drain or fixture-call waits use the shared test ceiling. |
| `cmd/dinah-desktop/bus_wake_test.go` | Positive completion, discovery, drain or fixture-call waits use the shared test ceiling. |
| `cmd/dinah-desktop/chat_cleanup_drawing_acceptance_test.go` | Positive completion, discovery, drain or fixture-call waits use the shared test ceiling. |
| `cmd/dinah-desktop/chat_cleanup_restart_acceptance_test.go` | Positive completion, discovery, drain or fixture-call waits use the shared test ceiling. |
| `cmd/dinah-desktop/chat_delivery_test.go` | Positive completion, discovery, drain or fixture-call waits use the shared test ceiling. |
| `cmd/dinah-desktop/chat_opens_test.go` | Positive completion, discovery, drain or fixture-call waits use the shared test ceiling. |
| `cmd/dinah-desktop/chat_restore_limit_test.go` | Positive completion, discovery, drain or fixture-call waits use the shared test ceiling. |
| `cmd/dinah-desktop/chat_state_test.go` | The shared completion failure ceiling is two minutes, with its purpose documented. |
| `cmd/dinah-desktop/chat_transfers_test.go` | Positive completion, discovery, drain or fixture-call waits use the shared test ceiling. |
| `cmd/dinah-desktop/crash_capture_test.go` | Positive completion, discovery, drain or fixture-call waits use the shared test ceiling. |
| `cmd/dinah-desktop/document_window_test.go` | Positive completion, discovery, drain or fixture-call waits use the shared test ceiling. |
| `cmd/dinah-desktop/empty_chat_restart_test.go` | Positive completion, discovery, drain or fixture-call waits use the shared test ceiling. |
| `cmd/dinah-desktop/exit_guard_test.go` | Positive completion, discovery, drain or fixture-call waits use the shared test ceiling. |
| `cmd/dinah-desktop/file_access_test.go` | Positive completion, discovery, drain or fixture-call waits use the shared test ceiling. |
| `cmd/dinah-desktop/frontend/e2e/agent-diagnostics.js` | Capture expiry waits for its completed status instead of sleeping 1.2 seconds; rate-bucket waits remain. |
| `cmd/dinah-desktop/frontend/e2e/agent-file-access.js` | Positive browser readiness uses a thirty-second ceiling; assertions remain unchanged. |
| `cmd/dinah-desktop/frontend/e2e/agent-providers.js` | Positive browser readiness uses a thirty-second ceiling; assertions remain unchanged. |
| `cmd/dinah-desktop/frontend/e2e/app-ui4.js` | Positive browser readiness uses a thirty-second ceiling; assertions remain unchanged. |
| `cmd/dinah-desktop/frontend/e2e/background-work.js` | Positive browser readiness uses a thirty-second ceiling; assertions remain unchanged. |
| `cmd/dinah-desktop/frontend/e2e/chat-background-replay.js` | Positive browser readiness uses a thirty-second ceiling; assertions remain unchanged. |
| `cmd/dinah-desktop/frontend/e2e/chat-restoring-mark.js` | Positive browser readiness uses a thirty-second ceiling; assertions remain unchanged. |
| `cmd/dinah-desktop/frontend/e2e/chat-sequence.js` | Positive browser readiness uses a thirty-second ceiling; assertions remain unchanged. |
| `cmd/dinah-desktop/frontend/e2e/chat-snapshot-response.js` | Positive browser readiness uses a thirty-second ceiling; assertions remain unchanged. |
| `cmd/dinah-desktop/frontend/e2e/chat-stream-markdown.js` | Positive browser readiness uses a thirty-second ceiling; assertions remain unchanged. |
| `cmd/dinah-desktop/frontend/e2e/chats-busy-count.js` | Positive browser readiness uses a thirty-second ceiling; assertions remain unchanged. |
| `cmd/dinah-desktop/frontend/e2e/delivered-messages.js` | Positive browser readiness uses a thirty-second ceiling; assertions remain unchanged. |
| `cmd/dinah-desktop/frontend/e2e/document-links.js` | Positive browser readiness uses a thirty-second ceiling; assertions remain unchanged. |
| `cmd/dinah-desktop/frontend/e2e/effort-control.js` | Positive browser readiness uses a thirty-second ceiling; assertions remain unchanged. |
| `cmd/dinah-desktop/frontend/e2e/files.js` | Positive browser readiness uses a thirty-second ceiling; assertions remain unchanged. |
| `cmd/dinah-desktop/frontend/e2e/group-mentions.js` | Positive browser readiness uses a thirty-second ceiling; assertions remain unchanged. |
| `cmd/dinah-desktop/frontend/e2e/html-safety.js` | Positive browser readiness uses a thirty-second ceiling; assertions remain unchanged. |
| `cmd/dinah-desktop/frontend/e2e/lib/commands.js` | Positive browser readiness uses a thirty-second ceiling; assertions remain unchanged. |
| `cmd/dinah-desktop/frontend/e2e/lib/page.js` | Positive browser readiness uses a thirty-second ceiling; assertions remain unchanged. |
| `cmd/dinah-desktop/frontend/e2e/mention-seen.js` | Positive browser readiness uses a thirty-second ceiling; assertions remain unchanged. |
| `cmd/dinah-desktop/frontend/e2e/model-prices.js` | Positive browser readiness uses a thirty-second ceiling; assertions remain unchanged. |
| `cmd/dinah-desktop/frontend/e2e/packages.js` | Positive browser readiness uses a thirty-second ceiling; assertions remain unchanged. |
| `cmd/dinah-desktop/frontend/e2e/provider-continuation.js` | Positive browser readiness uses a thirty-second ceiling; assertions remain unchanged. |
| `cmd/dinah-desktop/frontend/e2e/provider-restore.js` | Positive browser readiness uses a thirty-second ceiling; assertions remain unchanged. |
| `cmd/dinah-desktop/frontend/e2e/recovery-cycle.js` | Positive browser readiness uses a thirty-second ceiling; assertions remain unchanged. |
| `cmd/dinah-desktop/frontend/e2e/section-order.js` | Positive browser readiness uses a thirty-second ceiling; assertions remain unchanged. |
| `cmd/dinah-desktop/frontend/e2e/skills-browser.js` | Positive browser readiness uses a thirty-second ceiling; assertions remain unchanged. |
| `cmd/dinah-desktop/frontend/e2e/taskbar-stacks.js` | Positive browser readiness uses a thirty-second ceiling; assertions remain unchanged. |
| `cmd/dinah-desktop/frontend/e2e/workbench.js` | Readiness uses thirty seconds; live resize waits for rendering frames and an exposed handle before measuring and dragging. Existing delta and ghost assertions remain. |
| `cmd/dinah-desktop/frontend/e2e/workspace-transfer.js` | Positive browser readiness uses a thirty-second ceiling; assertions remain unchanged. |
| `cmd/dinah-desktop/group_mentions_receiving_test.go` | Positive completion, discovery, drain or fixture-call waits use the shared test ceiling. |
| `cmd/dinah-desktop/judge_process_test.go` | Positive completion, discovery, drain or fixture-call waits use the shared test ceiling. |
| `cmd/dinah-desktop/judge_reply_test.go` | Positive completion, discovery, drain or fixture-call waits use the shared test ceiling. |
| `cmd/dinah-desktop/mention_task_receiving_test.go` | Positive completion, discovery, drain or fixture-call waits use the shared test ceiling. |
| `cmd/dinah-desktop/native_sessions_test.go` | Positive completion, discovery, drain or fixture-call waits use the shared test ceiling. |
| `cmd/dinah-desktop/permission_test.go` | Positive completion, discovery, drain or fixture-call waits use the shared test ceiling. |
| `cmd/dinah-desktop/person_only_test.go` | Positive completion, discovery, drain or fixture-call waits use the shared test ceiling. |
| `cmd/dinah-desktop/protocol_log_test.go` | Positive completion, discovery, drain or fixture-call waits use the shared test ceiling. |
| `cmd/dinah-desktop/providers_test.go` | Fixture drain and provider disconnect use the shared ceiling; fixture cleanup now reports a failed drain instead of ignoring it. |
| `cmd/dinah-desktop/quality_chat_cleanup_binding_test.go` | Positive completion, discovery, drain or fixture-call waits use the shared test ceiling. |
| `cmd/dinah-desktop/quality_chat_cleanup_descendant_test.go` | Positive completion, discovery, drain or fixture-call waits use the shared test ceiling. |
| `cmd/dinah-desktop/quality_chat_cleanup_ownership_binding_test.go` | Positive completion, discovery, drain or fixture-call waits use the shared test ceiling. |
| `cmd/dinah-desktop/quality_pause_provider_test.go` | Positive completion, discovery, drain or fixture-call waits use the shared test ceiling. |
| `cmd/dinah-desktop/queue_restore_test.go` | Fixed iteration polling uses the shared completion deadline. |
| `cmd/dinah-desktop/request_settlement_receiving_test.go` | Positive completion, discovery, drain or fixture-call waits use the shared test ceiling. |
| `cmd/dinah-desktop/restart_pause_flow_test.go` | Positive completion, discovery, drain or fixture-call waits use the shared test ceiling. |
| `cmd/dinah-desktop/service_chats_test.go` | Positive completion, discovery, drain or fixture-call waits use the shared test ceiling. |
| `cmd/dinah-desktop/session_recovery_test.go` | Positive completion, discovery, drain or fixture-call waits use the shared test ceiling. |
| `cmd/dinah-desktop/session_resume_test.go` | Positive completion, discovery, drain or fixture-call waits use the shared test ceiling. |
| `cmd/dinah-desktop/show_test.go` | Positive completion, discovery, drain or fixture-call waits use the shared test ceiling. |
| `cmd/dinah-desktop/subagents_test.go` | Positive completion, discovery, drain or fixture-call waits use the shared test ceiling. |
| `cmd/dinah-desktop/task_host_frozen_binding_test.go` | Positive completion, discovery, drain or fixture-call waits use the shared test ceiling. |
| `cmd/dinah-desktop/task_host_model_receiving_test.go` | Positive completion, discovery, drain or fixture-call waits use the shared test ceiling. |
| `cmd/dinah-desktop/task_host_test.go` | Positive completion, discovery, drain or fixture-call waits use the shared test ceiling. |
| `cmd/dinah-desktop/task_questions_test.go` | Positive completion, discovery, drain or fixture-call waits use the shared test ceiling. |
| `cmd/dinah-desktop/terminal_shells_windows_test.go` | Positive completion, discovery, drain or fixture-call waits use the shared test ceiling. |
| `cmd/dinah-desktop/terminal_shutdown_test.go` | Positive completion, discovery, drain or fixture-call waits use the shared test ceiling. |
| `cmd/dinah-desktop/typed_question_receiving_test.go` | Positive completion, discovery, drain or fixture-call waits use the shared test ceiling. |
| `cmd/dinah-desktop/update_test.go` | Positive completion, discovery, drain or fixture-call waits use the shared test ceiling. |
| `cmd/dinah-desktop/workspace_import_chats_test.go` | Positive completion, discovery, drain or fixture-call waits use the shared test ceiling. |
| `internal/applet/host/maintenance_test.go` | Handler start, completion and maintenance waits use a two-minute ceiling. |
| `internal/applet/host/service_test.go` | Handler start, completion and maintenance waits use a two-minute ceiling. |
| `internal/claudeagent/claudeagent_test.go` | Fixture subprocess calls use the test context and a two-minute ceiling. |
| `cmd/dinah-desktop/shell_server_test.go` | Each repeated test invocation gets unique self-guarded and cookie-guarded route prefixes; GET/POST/cookie assertions are unchanged. |
| `cmd/dinah-desktop/frontend/e2e/branding.js` | The logo/menu check waits for expanded after click and closes through the same invoker before the next mode, retaining its original assertions. |

## Verification

The baseline and corrected desktop runs use Go 1.26.11, GOMAXPROCS=1, -cpu=1, -p=1 and -count=3 with two owned CPU-load jobs. Neither run pushes or reruns a GitHub workflow. Per-test JSON is retained in the system temporary folder under dinah-wake-sweep-before.jsonl and dinah-wake-sweep-after.jsonl. The completed results are recorded below.

The 500-millisecond file-access notification check remains explicit because it tests delivery before the product's one-second audit poll. Its duplicate-event observation window also remains. The diagnostic expiry argument bound, live-provider tests, startup acceptance ceiling and terminal shutdown policy checks remain unchanged. Negative observations in permission, shutdown, context and update tests remain unchanged. The exit retry's completion wait changes, but the separate test that expires the product's exitSettle limit still exercises its original policy.

The repeated recovery, applet-host execution and Claude subprocess controls passed five repetitions (desktop 5.776 seconds, applet host 1.029 seconds, Claude adapter 1.585 seconds).

The untouched baseline exited one after 931.236 seconds. It recorded 2,135 passing cases and 15 existing skips before TestASelfGuardedRouteIsServedWithoutTheCookie panicked on its second registration of /test-shell-frames/. This stopped the requested count-three run before all repetitions completed. No timing assertion failed before that stop; the recovered wake case had passed twice. A focused baseline count-three reproduction captured the exact duplicate-route panic in dinah-wake-sweep-baseline-crashes. Unique lowercase fixture prefixes preserve the cookie/GET/POST assertions and passed a focused count-three check in 0.203 seconds (dinah-wake-sweep-route-final.log). The corrected full count-three run includes that fixture correction and writes crash evidence to dinah-wake-sweep-after-crashes if needed.

The full headless browser batch ran with two suites at a time and completed 344 suites in 2,176 seconds. It passed 343 and exposed one immediate-state observation in branding: the dark logo menu assertion read aria-expanded directly after click. Branding now waits for expanded after click and collapsed after toggling its invoker, before repeating the light/dark sequence. It retains the original logo, menu, refresh and accessibility assertions. Its focused final run is recorded separately; no product or vendor code changed. The failure log is retained in dinah-wake-sweep-browser/failures.txt and branding.log.

The final-readiness subset ran 27 suites in 125 seconds: 26 passed and Workbench sampled a resize handle before it had an exposed pointer target. Workbench now waits two rendering frames and polls the identical elementFromPoint predicate, then measures its starting geometry after that wait. It still requires the actual mounted window to grow by the original width/height deltas during drag and after release, with no ghost. The original failure remains in dinah-wake-sweep-browser-final/failures.txt. A final focused Workbench run is recorded separately.

The diagnostic expiry suite passed in nine seconds. The entire modified applet-host and Claude adapter packages passed count-three, one-CPU, serial-package checks (0.530 and 1.738 seconds). The combined desktop, production and receiving-tag compile passed with CGO_ENABLED=0 and no tests selected (0.232 seconds); this is compilation evidence only.

The corrected desktop package completed all three repetitions with exit zero in 906.023 seconds. It recorded 3,561 passing cases, 24 existing opt-in/helper skips and no failures. Both the recovered wake test and the self-guarded route test passed all three invocations. The mode was Go 1.26.11 with GOMAXPROCS=1, -cpu=1, -p=1, -count=3, -timeout=60m and two owned CPU-load jobs, stopped by the validation wrapper afterward. The baseline used the same flags and owned load; other machine activity differed, so the durations are not a throughput comparison. The baseline source was 1e5468d9 and its clean auxiliary worktree was removed after measurement. No GitHub workflow was rerun and no measurement push was made.

Workbench passed its final focused suite in ten seconds. Branding's initial added wait exposed that Escape did not reliably dismiss the launcher with focus left on its invoker; this was an invalid cleanup assumption rather than slow readiness. The final test closes through the same launcher button and observes collapsed state before changing mode. That source passed its normal run in three seconds and three further unchanged-source runs in three, three and four seconds. The failing Escape-wait attempts remain in dinah-wake-sweep-branding and dinah-wake-sweep-branding-final; the passing invoker-toggle evidence is in dinah-wake-sweep-branding-toggle and dinah-wake-sweep-branding-repeat-1 through -3. All original logo, opening, menu refresh and narrow accessibility assertions remain.

The browser coverage consists of the retained full batch plus the final changed-suite subset and the explicit final Workbench, Branding and diagnostic runs. The full batch itself was not repeated after those isolated test corrections. No product or vendor source changed, and no UI changed. The temporary dependency junction was removed with System.IO.Directory.Delete.

The configured named-file staged profile guard passed with no SKIP in 10.950 seconds. Its log is dinah-wake-sweep-profile-final.log. The corrected stress run retained the same opt-in/helper skip names as the baseline; this sweep added no skip. The final integration still needs the normal merged-tree gate. The publishing-rule decision must be observed before any integration push; no GitHub rerun or push is requested for measurement.
