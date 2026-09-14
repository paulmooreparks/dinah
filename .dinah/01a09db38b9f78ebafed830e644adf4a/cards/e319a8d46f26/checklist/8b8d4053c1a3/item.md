---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:18:49Z
ordinal: 27
note: "Round 2's blocker 4 was two sections of the spec disagreeing about this exact gesture: §4 said the command wrote its channel line and showed nothing, while `summaryFor`'s single-row case required `skipped === 0` and so sent this report to the partial warning instead. The repair is in the code rather than in the prose: the single-row case now reads `report.selected <= 1` alone, and §4's special case is deleted, so there is one rule and this criterion verifies it. The case is not exotic. It is what a keybinding or another extension invoking a command against a row it cannot act on produces, and it is today's shipped behaviour, so a change here would be a silent regression rather than a new feature. The zero-call assertions are what distinguish \"shows nothing\" from \"shows the partial warning with the counts that happen to look harmless\", and the channel-line assertion is what stops an implementation achieving silence by doing nothing at all. Red run to produce at Test: restore the `skipped === 0` guard on the single-row case and watch the `showWarning` zero assertion redden while the channel-line assertion stays green."
---
A single targeted row that yields no context, handed to a `fanOut` command, spawns nothing, calls `showWarning`, `showInfo` and `showError` zero times, writes exactly one channel line naming that row and the command's skip reason, and returns a report with `selected` 1 and one `skipped` entry over which `summaryFor` answers level `none`.