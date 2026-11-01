# Useful Inbox

The Inbox defaults to unread, undismissed messages addressed to the person. Its badge reports the full unread, undismissed count, including items beyond the first page. Read and Dismissed views provide explicit history access; All includes both. A zero unread count stays zero. Reading or dismissing an item records no answer, approval or agent delivery acknowledgement.

Each preview opens and highlights its own stable message ID. A group item opens its group with that ID as the target, suppressing the normal blanket group-read operation. A direct item opens `BusThread(id)` rooted at the selected message, regardless of `replyTo`. Desktop replies retain that displayed root so a sent reply remains visible on refresh. Other bus callers retain their existing parent-thread reply behavior. The page marks only that item read after its target row has been drawn and highlighted. Failed lookup or rendering leaves it unread. Updates preserve the selected target and scroll position.

Engine owns Go, SQL, bridge declarations and this document. Front End owns the PUDL Inbox presentation, exact-message navigation, action wiring and the integrated focused browser check. Both implementations start from released `8bc2cc0`, rather than the older root checkout. Front End owns the direct integrated Release handoff.

## Backend contract

`BusOverview.inbox` contains the first unread page. `BusOverview.unread` is the uncapped count. `BusInboxPage(view, before)` returns `items`, `unread`, `nextBefore` and `newest`. Views are `unread`, `read`, `dismissed` and `all`. Read and Unread exclude dismissed items; All includes them. IDs descend, and a positive `before` cursor excludes that ID and all newer IDs. `nextBefore` is zero when there are no more items. `newest` is the highest human Inbox ID at query time, independent of the view, and bounds explicit bulk actions. Each item exposes both `unread` and `dismissed`.

`BusInboxMessage(id)` retrieves the exact Inbox item without marking it read, including items beyond pagination or previously dismissed. It rejects messages outside the person's Inbox. `BusMarkInboxRead(id)` and `BusDismissInbox(id)` persist only that item's state. Dismiss leaves its read state unchanged and retains history. `BusMarkInboxReadThrough(maxID)` and `BusDismissInboxThrough(maxID)` affect Inbox items no newer than the displayed snapshot. The UI labels these as bulk actions and does not invoke them on opening the pane. Arrivals beyond that boundary remain untouched. Dismissed history remains accessible.

`BusGroupMessagesFrom(group, fromID, context)` validates that the target belongs to the named group, then returns messages oldest first from up to `context` preceding rows through the newest. The page requests this only when the target falls outside the ordinary latest-message page. `context` is clamped to 0 through 100. The exact target is included. Ordinary group-window reading retains its existing behavior when there is no Inbox target.

The database adds nullable `inbox_dismissed_at` to human delivery rows. Existing `delivered_at` retains its established human read meaning. These operations change no message contents, attachment records, verdicts or session-recipient deliveries. Existing broad-read APIs remain available to older callers, but the corrected Inbox uses the explicit item or snapshot-bound operations.

## Focused verification

The integrated check covers selecting a reply rather than its parent, an older group target outside the latest page, display-before-read ordering, persisted dismissal and read state, truthful zero and uncapped counts, and stable selection through updates. Front End owns inspection of the rendered result. No live Inbox state is changed for this work. Broad regression remains at batch end.

Engine's focused bus checks passed with 205 unread items, a 200-item first page, cursor access to the remaining five, a later arrival outside the bulk boundary, persistent read and dismiss state after database reopen, and a group target older than its latest 300-message page. The bus command was `pwsh -NoProfile -File scripts/go-gate.ps1 ./internal/bus -run 'Test(HumanInbox|InboxGroup)' -count=1` and completed in 2.519 seconds. Its initial fixture incorrectly omitted the direct recipient; assigning the explicit human recipient fixed that setup error. No runtime code was changed to accept an unspecified recipient.

The desktop command was `pwsh -NoProfile -File scripts/go-gate.ps1 ./cmd/dinah-desktop -run 'Test(ThePaneShowsAndAnswersAThread|EveryPageMethodIsListed)' -count=1` and passed in 0.404 seconds. It checks exact reply selection, visibility of a reply after refreshing that selected conversation, and bridge method classification. Compilation is included in these checks. Front End still owns the integrated rendered result; these backend results alone do not establish UI acceptance.
