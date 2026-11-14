# Set-aside removal through the real Settings page

The hidden real-program journey passed all thirteen assertions on branch `quality/set-aside-live`, based on master `9758260a`. The page listed four invented copies, disabled the running copy and newest fallback, and removed the two selected eligible copies after confirmation. Their logical sizes totaled 3,072 bytes. The service inventory decreased by that amount, and both protected files retained their identities and contents. The holder process remained running throughout removal.

## Execution

The product was built from the branch with `go build -tags desktop,production -ldflags '-H windowsgui'`. A separate minimal holder executable writes a readiness marker to stdout and waits on stdin. The harness copies it into the temporary bin under an older `.old` filename, starts it hidden and waits for readiness before starting the product. It creates a newer `.old` fallback plus two eligible files, one `.old` and one `.before`, of 1,024 and 2,048 bytes. The newer date on the `.before` copy also exercises the specified `.old` fallback precedence.

The product executable SHA-256 was `2b3b928e4ad2cfe24fff7ad94f5ca2e154761318b9aed46177c60f0c9e390535`. The holder executable SHA-256 was `15afc3d41840c168425fab543cd85d06f6323b6a02764885f9617dcaddc9d8f9`.

The product runs with the hidden shell entry hook, isolated home, profile, application-data, provider and temporary paths, and native notifications disabled. Headless Chromium drives the actual Settings page and confirmation. Neither the page bridge nor the service is replaced or mocked. Direct list calls observe the real inventory before and after; removal is requested only through the page. Browser traffic outside loopback is blocked. No installed executable or real home folder is used.

The successful run checked the following outcomes.

- The service inventory matched all four invented files and their total logical size.
- The held copy reported the holder PID and had a disabled checkbox.
- The newest `.old` fallback had a disabled checkbox.
- Selecting the two eligible copies displayed 3,072 bytes.
- Confirmation named both files and their total while both still existed.
- Both removal results reported success and those two files disappeared.
- The held copy and fallback retained their file identities and SHA-256 contents.
- The service then listed two copies with its total reduced by 3,072 bytes.
- The holder was still running and the page recorded no errors.

The displayed amount represents logical file lengths; physical free-space changes were not measured. No full Go or browser suite was run. This check covers the native Windows service and the real Dinah page together, with invented files and one hidden holder. It does not test another product profile or a real installed-bin cleanup.

## Initial failures and final result

The first two harness runs stopped before any assertion or removal with `Unknown applet: settings`. The general UI-state promise was available before the Settings registration script had finished. Capturing the shell state showed `document.readyState` as `interactive` and the Settings-section registry absent. The harness now waits for that observable registry before opening Settings. It adds no fixed startup sleep. The third run passed all thirteen assertions with zero page errors; production source was unchanged throughout.

The product and helper builds and all three runs used the shared FIFO. Node syntax and diff whitespace checks passed. Each run closed the headless browser, terminated and awaited its two owned children, and reported `childrenStopped: true`. The temporary fixtures contain only invented files and isolated product state. Their paths are printed in the run receipts; the successful fixture is under the system temporary folder as `dinah-set-aside-live-B1Q27U`.

The reusable harness is `cmd/dinah-desktop/frontend/e2e/set-aside-live-hidden.cjs`; it takes the built product and holder executable paths. The helper source is under `cmd/dinah-desktop/frontend/e2e/testdata/set-aside-holder`. No push, merge, installation, visible window or desktop input occurred.
