# Skin-switch request measurement

The skins suite intermittently attributed product-mark.svg to a measured skin switch. The shell creates images independently of dinahAppearance.ready. The suite previously measured a switch without proving those images had decoded, so its request window could include shell image work. The unchanged suite passed locally; the reported runner interleaving has not been reproduced locally, so this is a measurement-boundary repair rather than evidence of a product defect.

The test now awaits decoding of the current document images before taking the request count. This uses the existing bounded deadline helper and fails if an image cannot decode. After the switch it waits two animation frames before checking the unchanged zero-new-request assertion. The request listener still records every URL; no asset is filtered or allowed. Existing preservation assertions remain. Product source, vendored files and the visible interface are unchanged.

## Verification

The unchanged skins suite passed in 50 seconds. The corrected skins and skins-startup suites passed together in 50 seconds, hidden and headless through run.ps1. An injected-request control and the staged guard are recorded in the handoff after completion. Evidence folders are %TEMP%/dinah-skins-request-before, dinah-skins-request-after and dinah-skins-request-control. No GitHub retry, push or installation was performed by this work.
