# Decisions reading review

The reading layout is ready on `frontend/decisions-reading` at source commit `5e4238c30d3ab70063f2f80516e69e90c58bdd01`, based on master `4cd9b954`. The design is in [decisions-reading.md](../design/decisions-reading.md). Dinah Team Lead should review this branch for integration. The comparison branch remains unchanged.

The window uses PUDL's documented room request to gain a wider reading pane while automatic side docks become temporary rails. Pinned-open docks keep their chosen placement, and automatic docks return when Decisions closes. At the 1440 by 1000 fixture size, the screenshots show a 352px list and a 1008px detail pane, compared with the earlier 504px detail pane. The text column stays within 76ch.

The asking chat heads the request, with its workspace, group and time beneath it. Numbered question text leads into its recommendation and answer field. A fixed delivery region keeps the receipt and Check Delivery together above independently scrolling questions. This also serves broadcast receipt recovery. The review warning has reserved space within the question region. Plain mentions keep their original text and use the Group Mention label when applicable.

As Recommended still fills a draft. Send Answers remains explicit. The change leaves partial answers, draft storage, destination review, dismissal, operation identities, retries and broadcast confirmation unchanged. The ten original Decisions suite files are byte-identical to the base commit; none of their behavioural or geometry assertions changed.

## Verification

The final production bytes passed thirteen headless suites in 73 seconds with exit zero. These were the ten original Decisions suites, decisions-status, decisions-attention-entry and the new decisions-reading suite. The failure record at `%TEMP%/decisions-reading-fixed-regions-20261114/failures.txt` is empty. The run used one browser job under the shared FIFO lock.

The reading suite verifies the wider pane, unchanged dock preferences, source and question ordering, stable window and row bounds on selection and arrival, retained focus and draft, visible delivery recovery, pinned docks, narrow overflow and restoration of automatic docks on close. After strengthening its fixed-region and overflow assertions, that suite passed again in seven seconds at `%TEMP%/decisions-reading-final-layout-20261114`, with an empty failure record. No production bytes changed between these runs. ESLint passed for decisions-applet.js, Node syntax checking passed for the new suite, and diff whitespace checks passed.

The initial layout check and the preceding twelve-suite regression also passed. Screenshot review nevertheless rejected the first sticky delivery treatment because it could cover an answer while scrolling. The final layout gives delivery its own space above the scrolling content. The earlier records remain at `%TEMP%/decisions-reading-layout-20261114` and `%TEMP%/decisions-reading-regression-20261114`.

The final draft, uncertainty, plain-mention and narrow screenshots were inspected. Local copies are in `.diagnostics/decisions-reading/` in this worktree. These use invented bridge responses and isolated browser state. They do not prove live bus delivery or native window behaviour. No full browser or Go suite ran. No visible window, desktop input, real home-folder access, credential read, push or installation occurred.
