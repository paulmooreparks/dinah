# Skin-switch favicon isolation

The earlier shell-image decoding fence did not fix the runner failure. It awaited document images, while the browser also fetched product-mark.svg for its favicon after window-history changes. The no-request assertion counted that independent browser work as a skin asset request.

A local CDP Network.requestWillBeSent trace captured one parser image request and eight later browser-initiated requests to the mark, reported as Image or Other with initiator other. Changing only the fixture favicon to an inline data URL containing the same bundled SVG removed all eight later requests. The parser image request remained. This control identifies the favicon as the source of those additional requests; it does not turn their observed scheduling into a required browser contract.

The suite now routes its initial HTML with the existing favicon bytes inline. It still loads the normal logo through its original URL, and records all requests without a URL or resource-type exception. A missing expected favicon link fails setup. The ineffective image-decoding fence is removed; the post-switch two-frame observation remains. No product, vendor or visible interface changes are included.

The test uses documented browser-context routing and an HTML icon data URL. Playwright documents that routing disables HTTP cache. HTML defines the icon as a user-agent resource, as a linked resource. The fixture removes the timing dependency rather than waiting for an undocumented favicon-ready signal.

- https://playwright.dev/docs/api/class-browsercontext#browser-context-route
- https://html.spec.whatwg.org/multipage/links.html#rel-icon

## Verification

Both traced probes passed in seven seconds. Raw CDP traces are retained in %TEMP%/dinah-skins-origin-probe/request-origin.json and %TEMP%/dinah-skins-origin-inline-control/request-origin.json. The corrected skins and skins-startup suites passed together in 49 seconds. A control injecting fetch of the exact product-mark.svg URL during the switch failed the unchanged no-request assertion in six seconds. Final setup validation and the staged guard are recorded in the handoff. No GitHub retry, push or installation is part of this repair.

## Real-host and cold-context comparisons

The installed desktop program was copied to a disposable home and started through the hidden-desktop harness. Its actual HTTP-served page was reloaded in headless Edge after removing all page routes. The mark request count was three at readiness, fifteen after ten history changes, and fifteen after a skin switch. The receipt passed with no page errors. Thus the product's own page also permits these favicon fetches in a browser; this is not limited to the invented App fixture. The shell server returns Cache-Control: no-store for its assets, so cache reuse is not a valid synchronization argument. Evidence is %TEMP%/dinah-skins-host-origin-20261127/receipt.json.

A separate attempt to observe the actual hidden native WebView2 did not expose its CDP endpoint despite the standard remote-debugging environment argument. Its isolated entry was written, and only that fixture process was ended. No native WebView2 request behavior is claimed from this attempt. The real-host HTTP comparison above uses headless Edge and must not be described as a native-renderer result.

Fresh routed contexts have HTTP cache disabled, matching the runner's test setup. The original and corrected measurement paths were also compared with fourfold CPU throttling and run.ps1 configured for two jobs. Both unforced shortened paths passed (29 and 28 seconds); the runner's original scheduling failure was not reproduced in those unforced runs. The final complete skins suite passed in 48 seconds with two jobs, after the setup-link guard was added. No timeout was raised and no URL was excluded.

A controlled overlap moves the history address briefly and restores it before the preservation checks. With fresh contexts, fourfold CPU throttling and two suites at a time, the original favicon fixture failed only the no-request assertion with two product-mark.svg requests; the corrected fixture passed. The final URL, mounted content, focus, selection and geometry checks remained intact. This demonstrates the attribution failure without requiring the runner's unforced timing. Evidence is %TEMP%/dinah-skins-history-url-controls (32 seconds). A same-address history control passed both variants and is retained separately in %TEMP%/dinah-skins-history-controls.
