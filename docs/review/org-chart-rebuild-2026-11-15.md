# Org chart rebuild receiving result

The rebuilt chart follows the approved mockup and the lead rules recorded in docs/plans/org-chart-view.md. Its chat population comes only from the shared inventory. The chart projects teams and groups from the host's transactional relation snapshot.

The page uses 216-pixel chat boxes, a toolbar legend and a status-bar count. Historical nested teams appear at their parent's member position, and folding an ancestor hides every descendant. New host writes refuse nested team roles. Title, state and time updates preserve every box's geometry.

Actions remain visible through a page-level rule and open through the public PUDL menu API. Native dialogs preserve the chart and its scroll position. Cancel, Escape and successful changes restore the originating Actions control when it survives. Deletion focuses the chart and names the deleted chat in its status message. Failed and partial results remain visible in their dialog.

The host refuses assigning or deleting a current lead. It offers replacement on an eligible member, rechecks membership and updates the reporting records atomically. The old lead becomes a member of the same team. When the final reporting member leaves, the channel and history survive as an ordinary group. Bulk cleanup reports the refused lead separately and processes eligible rows.

Org Chart and Clean Up Chats are available from the launcher and the Chats header. Org Chart is in View and is removed from Window. The implementation uses shipped PUDL and app.css.

## Tested source and evidence

The product and receiving files in this delivery match tested source f18295b8. The delivery branch starts at 9ec3496e and consolidates unpublished work so its history contains no private product references. Earlier source commits remain local receiving references. Compiler output and raw App snapshots remain private.

All 60 unchanged independent frozen cases passed through the real App on both receiving products. The 40 rendered cases passed in 248.758 seconds across light and dark at 1440 and 390 pixels. They cover population, links, window reuse, visible keyboard actions, all six dialog journeys, transitive folding, narrow horizontal fit and live metadata stability. The 20 host cases passed in 165.597 seconds, including forged requests, deletion after both permitted exits, bulk partial success and committed-reader observation of atomic replacement.

All six negative controls rejected altered captured observations. Their unmodified controls passed first. The mutations introduced a duplicate box, an empty container, hidden Actions, lost focus, a wrong link target and a partially visible lead swap. These controls exercised the unchanged frozen assertions and did not alter product files.

The seven page regression suites passed in 31 seconds. They cover the chart, its actions and stability, common inventory, cleanup and both workspace suites. The complete profile package passed. The final focused desktop race run found one failure in the second action query after successful chat creation. Engine owns that consistency failure. Lead reproduced it under race detection and requires a production correction that settles creation writes or excludes unrelated late state from the identity check. A fixture-side wait does not close the hold. The complete bus package passed with race detection and all vet analyzers in 172.179 seconds. No acceptance is claimed for the failed desktop regression.

The pictures in the adjacent folder come from the final real-App run for this product. live-geometry.json contains the observed title, state, time and box rectangles without raw local paths. Earlier receiving failures are retained privately, including the command-peer setup error and the measured narrow overflow.

Quality independently cleared the unchanged 60-case matrix in 471.220 seconds, without skips, on the tested source. Its durable receipt is ff08823f on quality/org-chart-receiving. This clearance retains that source attribution while the production correction is pending. The delivery at 30551e5a passed the full profile package with the private forbidden-terms list, plus commit-message, branch-diff and local-path scans. The tagged hidden host does not establish installed native-window behavior. No merge, push or installation has occurred.
