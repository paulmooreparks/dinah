# Copy feedback timing investigation

The failing assertion compares the button width and height and also requires its temporary accessible label to remain Copied. The hosted failure log contains no individual values, so that failure does not establish a size change.

The browser job runs on windows-latest with Edge. It is separate from the Linux guards. Local reproduction used PUDL 0.112.0 at 4518a373.

## Reproduction

Eight unmodified copies of the suite passed concurrently with fourfold CPU throttling. A temporary control delayed the driver observation by 1.6 seconds after the live-region confirmation appeared. It failed the same assertion while both boxes remained x=0, y=867, width=1280, height=33 and fonts were loaded. The label had returned to Copy path.

The product deliberately removes confirmation after 1.5 seconds. The original test performs separate driver calls to observe the announcement, measure the box and read the label. A busy driver can cross that expiry between calls. The confirmation implementation and original test are unchanged from 7aa7797d. This establishes a pre-existing test timing defect, but the absent runner values prevent proving that the hosted failure had this cause. No genuine geometry regression has been reproduced.

## Correction

The fixture now installs Playwright's documented clock before navigation, lets startup finish with time running, then pauses time before creating the button. It advances 40 ms after the click to fire the 30 ms announcement while keeping the 1.5 second confirmation alive for measurement. After the repeated-copy assertion it advances 1.5 seconds and verifies restoration as before.

The width, height and label conditions remain unchanged. Failure output now includes both boxes and the label. No product code or PUDL asset changes. The clock controls the existing timer rather than lengthening its lifetime in the product.

Playwright documents pauseAt and runFor in its Clock API, also included in the installed playwright-core type declarations. The fixture follows the documented install-before-navigation ordering.

Evidence remains in the temporary folders dinah-copy-feedback-before and dinah-copy-feedback-clock. Temporary control sources are retained with their evidence and are absent from the branch. No GitHub job was started or retried for this investigation.

The corrected suite passed normally and with the same 1.6-second driver delay. A control that widened the button by 10 px still failed the unchanged geometry assertion and printed both boxes with Copied still present.
