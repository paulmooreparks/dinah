# Restart recovery notifications and steering verification

Quality prepared this change on `quality/pause-restart-left`, from master `e87cdf5328bd6c28863aa429abdab045e873d173`. The lead's decision preserves the first-turn safeguard. A chat whose first turn was interrupted still requires the person to review its saved history before continuing.

## Behavior

Restart previously enqueued the continuation and cleared the pause record without telling the person when reopening failed. The new restart outcome record tracks those asynchronous open attempts. A failed attempt adds its chat title and workspace to one durable Notifications card for that restart and writes a warning to the application log. Each chat has an Open button. The button uses the existing chat opening and saved-history recovery flow; it does not bypass the safeguard or consume the waiting continuation.

The card groups failures by restart identity. Repeated callbacks do not duplicate entries or revive a dismissed card. A newly discovered failure updates the grouped card. Successful opens consume their tracking entry, including foreground opens, so later unrelated failures cannot become restart warnings. An already open chat is excluded when tracking begins. Existing delivery-failure notices remain available alongside this restart summary.

The original pause record captures the workspace for error paths that cannot resolve a route. Older records can recover it from the transcript. Notification storage failures are logged and leave the outcome tracked. This change does not introduce a retry timer for storage failure or promise recovery from every crash boundary.

## Focused checks

The final focused Go command passed all seven matching tests in 20.347 seconds:

```powershell
scripts/with-test-lock.ps1 go test ./cmd/dinah-desktop -run 'Test(RestartLeft|PauseFlow|Notifications)' -count=1 -timeout=60s
```

The tests cover grouped failures, persisted titles and actions, workspace fallback, successful-open cleanup, unrelated failures, dismissal, invalid targets, pause steering, cancellation and continuation idempotency. The headless `frontend/e2e/restart-left.js` suite passed its four assertions and page-error check. It verifies two chat buttons on one card, delivery refresh without duplication and restoration after a page reload.

The real-process harness is `frontend/e2e/restart-hidden.cjs`. Its stand-in now accepts Dinah's hook steering through `BindAgentTurn` and `TakeAgentSteers`, records the pause request, then ends its ACP turn. It does not contact a model or execute tools. Only the first prompt block is written to receipts, excluding appended authorization context.

The desktop was built through the shared FIFO with `go build -tags 'desktop,production' -ldflags '-H windowsgui'`. The executable SHA-256 is `a1199a30231826cfe32ce53b73c7753632eb46c45615b889ab0cd1c351d688ac`. The harness uses an isolated home, `DINAH_SHELL_ENTRY_FILE`, a hidden native window and headless Chromium. It replaces only its temporary executable, then observes the original process exit and the replacement's new entry address. All builds and tests used the shared FIFO.

## Retained failures and scope

An early button assertion expected backend error wording instead of the chat's recovery heading. A separate UI selector initially selected the hidden notification menu row instead of the card. Both failed and were corrected in the harness. Two final-build attempts omitted desktop build tags and timed out before the shell entry appeared; another build command failed because PowerShell split an unquoted comma in the tags argument. These are retained failed checks, not successful product evidence.

The test establishes host steering delivery and process transitions using a compliant stand-in. It cannot establish a real model's choice of safe stopping point. The first-turn scenario can also produce existing turn-failure and delivery-failure cards; the new guarantee is one restart-summary card per restart. No full browser suite, race suite, installation, live restart or desktop input was performed for this change.

## Final process checks

Both final checks exited zero using the executable identified above. Receipts and isolated application logs are under `%TEMP%/dinah-pause-restart-check`.

| Directory | Observed result |
| --- | --- |
| `production-steer` | The working stand-in received the pause instruction through the steering hook and ended its turn. Process 20044 exited automatically. The replacement served a new entry address and submitted one continuation saying that the chat had paused. The prompt receipt contains the initial completed turn, the working turn and that single continuation. A further delivery interval did not duplicate it. |
| `production-first` | The first-ever turn remained working while restart waited. Restart Now closed process 73804 and a replacement started. One restart-summary card named the chat. Clicking its Open button in headless Chromium opened the chat, showed the first-turn recovery heading and exposed Continue from Saved History. The safeguard still refused automatic reopening. |

The existing shutdown path also recorded an application error saying that the window bus was closed in the interrupted-first-turn run. This report retains that observation; the change does not address shutdown error presentation. The hidden native window itself was not visually inspected. No installed executable was replaced, and there was no shared-desktop mouse, keyboard or focus automation.

This closes the prior report's real-process steering gap with a controlled stand-in. The branch is ready for lead review.
