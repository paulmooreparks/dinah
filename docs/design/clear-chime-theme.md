# Clear Chime default theme

Paul approved the revised Clear Chime palette on the Standard skin on 5 November 2026 and requested installation for daily use. The approved dark version reduces saturation with charcoal surfaces and muted teal chrome. Light mode retains warm ivory surfaces and dark turquoise accents.

Clear Chime is a built-in theme and the Dinah product default. Existing explicit theme selections remain intact. For Paul's trial, the appearance library is updated to Clear Chime and Standard through the shared state-file lock after validation, with a backup of the previous library. Custom themes, content overrides and light/dark/system preference are preserved.

Desktop and web receive identical built-in theme data and default brand styles. The implementation changes palette inputs only. It introduces no custom skin, component geometry overrides or changes to PUDL's shipped files. The existing application icon remains separate from this palette installation; the dinner-triangle artwork is still a concept.

The source palette is recorded in docs/mockups/themes/clear-chime.json. The candidate generator uses Dinah's existing contrast solver. The shared appearance package fills category colors and validates structure, text/control contrast and category separation. Neither approved surface palette required contrast adjustments. Dark added-line diff tint is 9 percent under the existing contrast rule.

Validation covers default selection, persistence, light/dark surfaces, Standard skin selection, appearance controls, system mode, existing theme preservation and editor regression behavior. Screenshots and run results are retained under build/clear-chime. This does not claim that the pre-existing Standard focus-paint contrast issue has been corrected or that every optional skin has been newly admitted with Clear Chime.

The appearance, clear-chime, editor and theming browser suites passed in 64 seconds. Targeted Go tests for desktop, appearance, profile and web passed. Both production binaries built successfully and were installed with staged and installed SHA-256 verification. The desktop version is 0.0.0-clear-chime-20261105. Install and appearance receipts under build/clear-chime record the prior binaries and appearance-library backup. The stored selection was verified as clear-chime with standard. The running desktop was not restarted; the person will restart it to load the new assets.
