# Review baseline

Quality ran the full Go and browser suites on master 4a34588 at the start of the November 2026 review. Every failure was classified. Either the test expected behaviour that Paul deliberately changed that week, in which case its expectation was updated with the commit as evidence, or the product was wrong, in which case the failure became a ledger finding and the test was left unchanged as its guard. No assertion was removed, no time limit was raised, and no retry was added. The work is on `fix/review-baseline`, rebased onto the rewritten master 5ea76fc.

## Go suite

Four tests failed, and all four were product findings.

| Test | Finding | Outcome |
| --- | --- | --- |
| `TestNoForbiddenTermsInTrackedFiles` | BL-01: private terms were published in three files | The files were reworded (now 2a4591c on master), and the published history was rewritten. |
| `TestNoProviderNamesInSharedCodeBeyondTheList` | BL-02: a provider-name literal in shared code | A provider-row fact replaces it (now 252b691 on master). |
| `TestThePaneShowsAndAnswersAThread` | BL-03: a live, untitled sender showed in Inbox as "no longer available" | Fixed on this branch. |
| `TestAFailedCompactionPutsTheHoldAndTheQueueBack` | BL-04: a definite provider refusal left the chat's continuation permanently blocked | Engine's fix, `fix/recovery-allocation`. |

## Browser suite

Of 199 suites, 178 passed and 21 failed. Seventeen were stale and have been updated. One was too slow and has been split. Three were product findings.

| Suite | Why it changed | Evidence |
| --- | --- | --- |
| bus | Inbox rework (unread first, sender-led rows, exact-message reply); Members dropdown | 29b1e5e, c3d1b6d, 2d082fe, docs/design/inbox.md; f22f9b5, 8bc2cc0 |
| group-lookup, group-rename | Members moved into the Members dropdown | f22f9b5, 8bc2cc0 |
| group-controls | Last Read and Members in their own toolbar groups; team-only buttons hidden for plain groups | f22f9b5, 9f20613 |
| composer-completion | The state badge in the mention list; team buttons fold Mention Everyone into the Group menu at 330 px | 2d0f96d, 9f20613, f22f9b5 |
| headsup, judgements-warnings, judgements-widget | An empty count now shows as a neutral "0" | 11d4c27 |
| stack-row-titles | Stack rows keep the empty state column; a new check requires that column to be zero width when no window has a state | a2385ab |
| theming | Bevel, Gel and Glaze themes added; the inventory is now pinned by ID | 7616cf1 |
| chat-states, group-chats-background, list-state-stability, list-row-layout, member-stability, chat-background-member | A state chip now sits beside hidden copies of every state's chip, so that its row does not move; members moved into the dropdown; chips use PUDL's state badge | c8031ec, f22f9b5, 8bc2cc0, 70e4a94, 2d0f96d |
| teams | Team reasons, task, usage and allowances moved to the team group's Team Report and Team Allowances | 9f20613, f22f9b5, dada2db |
| app-ui3 | One selector follows c8031ec. BL-05 adds a guard that each group's count matches the rows it shows | c8031ec |
| skin-paints | Not failing; it ran past the 600 s limit once there were 14 themes and 8 skins. It is now two shares over one shared body (`lib/skin-paints-run.js`), and theme i goes to share i % 2 | BL-08 |

The three product findings are BL-05 (`app-ui3`, a hidden chat stays listed), BL-06 (`chat-continued-open`, a continued window shows an empty transcript) and BL-07 (`provider-continuation`, following onto a window that is already open leaves two windows). Front End's fixes are `fix/hide-chat-focus` and `fix/continuation-follow`, and Quality has accepted both.

## Decisions recorded for Paul

Two expectations follow adopted PUDL behaviour rather than an earlier Dinah rule. In member-stability, a member chip's glyph is now required at PUDL's fixed 9 px, where the old rule from 31 October was 16 px. In theming, Bevel's dark added-line tint derives to 7 percent, below the 8 percent Dracula exception, and no design record covers that value.

## Green

The baseline is green when this branch, BL-04, BL-05 and BL-06/07 are all on master and the full Go and browser suites pass there in one run. Quality runs that check after those handoffs land.
