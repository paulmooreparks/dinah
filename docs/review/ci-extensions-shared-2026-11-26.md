# Extension frame readiness budget

The extensions-shared suite failed in run 45788472231 while waiting for the second frame's visible ready output. Both frames use the fixture workspace: the first resolves the empty selector to fixture and the second names it explicitly. The ordinary local baseline passed in six seconds.

The test inherited the browser helper's generic ten-second locator timeout for the entire asset load and readiness handshake. The product's separate ten-second bridge timeout starts after iframe load. A slow asset response can therefore exceed the test timeout before the product begins its bridge deadline. The suite now gives frame readiness a bounded thirty seconds and still requires the same visible ready output. Subsequent shared-state, workspace-following and isolation checks are unchanged. The page, SDK and product bridge deadline are unchanged.

A failed readiness wait now includes the matching frames' source, visibility and host status. Those details distinguish a missing frame, an asset load that has not completed and a host bridge failure if CI fails again. No retry or skip was added, and no visible interface changed.

## Verification

The ordinary baseline passed in six seconds. An isolated route probe delayed only the second fixture script response by eleven seconds. The original ten-second wait failed at the same readiness boundary in thirteen seconds. With the new wait, that probe passed all suite assertions in fifteen seconds. The unchanged normal asset path also passed in five seconds after the correction.

These results reproduce a sufficient cause for the timeout. The runner's original log contains no frame status or resource timings, so they do not establish that its actual failure was solely slow loading. The next CI run remains the confirmation, with richer failure details if needed.

Logs and output folders are retained under the system temporary folder using dinah-extensions-shared-baseline, dinah-extensions-delay-old, dinah-extensions-delay-fixed and dinah-extensions-shared-fixed names. The probe was temporary test-only source, not a product change.
A negative control served an empty second fixture script. It still failed after the new thirty-second deadline (33-second suite), even though the SDK bridge had connected. The failure included the correct source and host status. Its log and source are retained as dinah-extensions-never-ready.log and dinah-extensions-never-ready-probe.js. The final diagnostic also records whether the host status is hidden, distinguishing its retained loading text from an active error.
