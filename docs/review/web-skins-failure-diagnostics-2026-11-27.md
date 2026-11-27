# Web skin failure diagnostics

This branch starts at d6c9afcf and changes only the web-skins browser suite and this review note. It is prepared locally and has not been handed over for integration or publication.

The suite retains its original first-frame assertion, predicate and failure detail. If that assertion fails, it additionally prints a JSON diagnostic line through the runner's existing FAIL output filter. Passing assertions print no diagnostic line.

The suite records stylesheet request URLs, HTTP response statuses, request failures and epoch-millisecond timestamps. Browser events use performance.timeOrigin plus performance.now(), so their timestamps can be compared with the network observations. The browser records skin-link load/error events, skin-change events, and each registration attempt, return value or thrown error. The registration wrapper forwards the original receiver and arguments and preserves its return or exception. It does not inspect CSSOM before registration.

After an assertion has failed, the diagnostic collection waits for three animation frames and reads the effective skin, retained preference and available skins. The existing deadline helper bounds this collection. The later observation never replaces the first-frame assertion or turns its failure into a pass. No host, vendored or other product source changes.

## Verification

The complete web-skins suite passed locally with the instrumentation. A temporary single-case control returned HTTP 404 for Glaze on the dark, narrow conversation page. The unchanged assertion failed with Standard effective and Glaze preferred. The printed diagnostic included the 404 status, the native error event, registration attempt and result, the first-frame snapshot and Standard still effective three frames later. The runner exited with failure. Other stylesheet load events were recorded by the same observer.

The control source and output remain in the temporary folder dinah-web-skins-diagnostics-control. The control is absent from this branch. Normal-run evidence remains in dinah-web-skins-diagnostics-normal and final verification output in dinah-web-skins-diagnostics-final. The diagnostic JSON is included in the suite log and failures.txt by the existing runner.

Instrumentation adds small synchronous bookkeeping costs before sampling. A passing instrumented run cannot prove that the original intermittent failure has gone away. The branch makes the next failure explainable; it does not repair or reclassify either reported product symptom.
