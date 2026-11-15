# Stable update and restart controls

This branch starts at `cdc7b9b8`, the accepted frame audit. It fixes the taskbar and Updates findings assigned in message 8389. The carry-on transcript finding remains with Front End.

The taskbar Update control reserves the width of Restarting, its longest current label, using a hidden grid cell in the same font as the visible label. Both labels occupy that column. Compact and folded taskbar behavior remains controlled by the existing window bar rules.

The Updates progress row gives the progress element a five-rem column and the text a separate bounded column. The text box has the row's fixed four-rem height, wraps, and can scroll if needed. Its contents no longer determine the bar's width. The native progress fill continues to reflect the download value.

The two audit suites now assert that all retained visible elements stay fixed in every captured frame for Updates and for pause/cancel. They still capture eight frames after the observed state and keep all intervening frames. Their capture scope and one-pixel threshold are unchanged. The transcript captures remain observations because another branch owns those fixes.

## Verification

The unchanged product at `cdc7b9b8` fails both strengthened suites. The baseline reports pause and cancel failures, plus downloading, known-size progress and installation-completion failures. Its runner exits one. Evidence is in `%TEMP%/quality-update-layout-before`; FIFO ticket 2038 ran the two suites in 23 seconds.

An initial shell edit used a root-relative path while already in the e2e directory. It changed no file. The paths were corrected while the baseline was still queued, before either test began.

The after-run results are recorded below when complete. No product or test assertion is relaxed to hide movement.

Both strengthened frame suites pass after the CSS changes, and the existing update behavior suite passes. FIFO ticket 2041 completed all three in 31 seconds, with exit zero. Evidence is in `%TEMP%/quality-update-layout-after`. The installed-state screenshot was inspected; the status text wraps in its reserved box and the progress bar keeps its width. The taskbar button retains the same bounds through pause and cancel. All recorded Updates transitions now keep retained elements fixed.

Syntax and whitespace checks pass. No changes were made to transcript rendering, service responses or timing waits. No push, merge or installation occurred. The branch is ready for the lead to review and integrate.
