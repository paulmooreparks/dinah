# Help accuracy review, 14 November 2026

The review uses `quality/help-accuracy`, based on master `3916e362`. It checks Working with Chats and the command-reference rows added with that guide. Product behaviour is unchanged. The branch corrects wording and adds focused checks.

The overdue restart card fails the guide's claim that Notifications shows which chats are still working. The card's accessible label contains the blocked chat, but its rendered content does not. `update.js` places the names in `description`; `notices.js` renders `title` and `body` on the full card instead. The retained assertion in `help-accuracy-places.js` fails for that reason. The two action buttons exist. This needs a product correction by the feature owner.

## Claim inventory

PASS means the specified evidence supports the claim within its stated scope. Browser evidence uses the real frontend with a stubbed host in headless Chromium. Go evidence exercises the host with temporary state and stand-in providers. Source-only conclusions are identified and are not live verification.

| Claim | Result | Evidence |
| --- | --- | --- |
| Working with Chats is in Help. | PASS | `help-new-work` opens it with Alt+H, W and restores it after reload. |
| Decisions is in View and opens from the status count. | PASS | `help-accuracy-places` checks the command registry; `decisions-status` exercises the status entry. |
| Questions show the asking chat, destination and recommendation. | PASS | `decisions-answers` and the rendered question forms. |
| As Recommended prepares an answer without sending. | PASS | `help-accuracy-keys`, with Enter and Space. |
| Send Answers sends the chosen answers, leaving others open. | PASS | `help-accuracy-keys` and `decisions-answers`; host calls are stubbed here. |
| Drafts remain per request after close and reload. | PASS | `help-accuracy-keys` closes and reopens; `decisions-drafts` reloads and checks separate drafts. |
| Broadcast to Active and Broadcast to All are in View. | PASS after wording correction | The reference now includes the displayed ellipsis. `help-accuracy-places` checks both registered commands. |
| Active means working chats; All includes closed chats. | PASS | `TestDecisionActiveBroadcastUsesRuntimeWorkingState` and `TestDecisionBroadcastQueuesWithinBackgroundLimit`. |
| Preview shows recipient count and titles before confirmation. | PASS | `decisions-broadcast` and keyboard checks. |
| Editing a broadcast requires a fresh preview. | PASS | `decisions-broadcast`. |
| Closed chats wait for background capacity. | PASS | Go bounded-admission test. |
| Pause Chats and Restart is in the application menu without an update. | PASS | `help-accuracy-places` checks the initial menu. |
| Every working agent can receive the pause request. | FAIL as an unqualified reading; wording corrected | The guide now distinguishes agents that accept input during a turn from agents that must be waited for. The provider routing contract and existing pause tests support this distinction. No real provider was contacted. |
| Recorded chats receive carry-on messages; chats that cannot reopen need manual opening. | PASS with the added qualification | Pause flow tests reconstruct the App with persisted state. `RestartLeft` tests check the grouped notification and reload. This run does not restart an OS process. |
| A restart can use the current program, or an installed newer build. | PASS by source inspection | Shared restart selection in `update.go`; `TestSameProgramResumeTextNeverClaimsAnUpdate` checks the message. No executable replacement or native restart ran. |
| The overdue threshold is two minutes. | PASS | `TestHelpTwoMinutePauseClaim` checks one and three minutes and the exact duration constant. It does not sleep two minutes. |
| The overdue card visibly names the working chats. | FAIL | Retained headless assertion. Names are only in the accessible label and compact description. |
| The overdue card offers Restart Now and Cancel the Restart. | PASS | Both actual buttons are present in `help-accuracy-places`. |
| Restart Now can cut turns short; Cancel carries paused chats on. | PASS within host-test scope | Pause flow cancellation tests and restart record handling. Native process replacement was not repeated. |
| Restart When Chats Finish appears for an installed update; Cancel appears while pending. | PASS | Dynamic menu assertions in `help-accuracy-places`. |
| Restart Now is available through the status update menu. | PASS | Actual `update-now` control and its registered handler; reference label now includes the ellipsis. |
| Notifications is in View and the status bar, and Ctrl+Shift+I opens it. | PASS | Registry and status popover checks; keyboard test verifies the active window. |
| Kept cards show source, time, first line and a coalesced count. | PASS | `attention-kept` and card rendering with invented messages. |
| Cards persist through background refresh and reload until opened or dismissed. | PASS | `attention-kept`. |
| Open Decision targets the request; other cards target the source message. | PASS | `decisions-attention-entry`. |
| Opening a decision leaves its unanswered questions open. | PASS | `decisions-attention-entry` checks that no answer is sent. |
| Answering a request removes its attention and updates the count. | PASS | Decisions entry/status fixtures. |
| Maximum Chats is in Settings, Chat, under Background Chats. | FAIL; wording corrected | The actual heading is Chats Opened in the Background. Both Help references now use it. |
| Maximum Chats accepts typed input through keyboard navigation. | PASS | Tab, typing and blur invoke the setter with 6. |
| The default background limit is eight; changing it applies on the next check without closing open chats. | PASS | `TestBackgroundLimitDefaultsPersistsAndAppliesWithoutRestart`. |
| A closed team member explains that its message is waiting. | PASS by source and host-state checks | Team row uses the supplied state reason; waiting-sender Go test passes. The broader Teams fixture is not clean, as recorded below. |
| Updates is conditional on the build and is available in Settings. | PASS | `settings-updates-unavailable` and the available/current fixtures. |
| Updates compares installed and running copies, including this window. | PASS | `settings-updates-current` and `settings-updates-installed`. |
| Release notes are read before an explicit install; installation does not restart programs by itself. | PASS within fixture and host-test scope | Available/installed fixtures and `TestHelpInstalledBuildDoesNotRequestRestart`. No real download or install ran. |
| The page checks and updates skill packages. | PASS | Package fixture and keyboard check. The update action opens its authorization plan. |
| The skill-package button is named Update Skill Package. | FAIL; wording corrected | The actual label is Update. |
| Check Agent Settings previews a repair before Repair Agent Settings applies it. | PASS | Repair fixture and keyboard check. |
| As Recommended, Send Answers, Earlier Requests, Preview Recipients and Confirm Broadcast support Tab then Enter or Space. | PASS | Every named control is exercised with both keys in `help-accuracy-keys`. Confirm Broadcast opens the confirmation dialog without sending. |
| Check for Updates, Install Update, Check Skill Packages, Update, Check Agent Settings and Repair Agent Settings support Tab then Enter or Space. | PASS | Every named control is exercised with both keys in `help-accuracy-updates`. |

## Verification record

The first selected browser batch passed all 14 suites in 15 seconds. It covered attention, Decisions answers, entry points, broadcast, drafts, pagination and status, Help, and the six Updates states. The new keyboard suites also pass, including the added close-and-reopen draft check. The final combined four-suite run took 10 seconds: `help-accuracy-keys`, `help-accuracy-updates` and `help-new-work` passed; `help-accuracy-places` failed the visible blocked-chat assertion.

The focused Go command passed in 7.673 seconds:

```text
go test ./cmd/dinah-desktop -run 'Test(Help|BackgroundLimit|WaitingSender|DecisionActiveBroadcast|DecisionBroadcastQueues|PauseFlow|SameProgramResume|RestartLeft)' -count=1 -timeout=120s
```

An additional `teams` browser run failed its existing assertion, `a failed relay keeps the reason and both source and lead actions`. That failure is retained and was not changed or retried away. It is outside the Help wording edits and needs separate investigation. The two-suite run took 8 seconds and also contained a test-label mismatch for the status Restart Now ellipsis, subsequently corrected.

Earlier new-suite attempts failed because the initial fixture omitted Decisions and the background-limit setter, selectors matched multiple buttons or card copies, and expected labels differed from the product. Those test setup issues were corrected. The initial label failures also exposed the actual Help naming errors described above. No assertions about the missing blocked-chat text were relaxed.

Evidence is under the temporary folders `quality-help-accuracy`, `quality-help-accuracy-new`, `quality-help-accuracy-corrected`, `quality-help-accuracy-final`, `quality-help-accuracy-labels`, `quality-help-accuracy-card`, `quality-help-accuracy-card-dom`, `quality-help-accuracy-reviewed`, `quality-help-accuracy-entry-final`, and `quality-help-accuracy-final-entry`. These contain invented fixture data only.

No full browser or Go suite ran. There was no native window, desktop input, real provider call, download, installation, push, merge or real-home write. The report supports the named frontend and host contracts; it does not certify native keyboard routing or a fresh live restart in this turn.

The final entry-point rerun took 4 seconds and failed only the retained visible blocked-chat assertion. The status Restart Now label and conditional menu assertions passed. JavaScript syntax checks and Git whitespace checks passed.
The final two new Go checks passed in 2.149 seconds after widening their observation points to one and three minutes while retaining the exact two-minute constant assertion.
