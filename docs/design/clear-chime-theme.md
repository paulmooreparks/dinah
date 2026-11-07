# Clear Chime default theme

Paul approved the revised Clear Chime palette on the Standard skin on 5 November 2026 and requested installation for daily use. The approved dark version reduces saturation with charcoal surfaces and muted teal chrome. Light mode retains warm ivory surfaces and dark turquoise accents.

Clear Chime is a built-in theme and the Dinah product default. Existing explicit theme selections remain intact. For Paul's trial, the appearance library is updated to Clear Chime and Standard through the shared state-file lock after validation, with a backup of the previous library. Custom themes, content overrides and light/dark/system preference are preserved.

Desktop and web receive identical built-in theme data and default brand styles. The implementation changes palette inputs only. It introduces no custom skin, component geometry overrides or changes to PUDL's shipped files. The existing application icon remains separate from this palette installation; the dinner-triangle artwork is still a concept.

The source palette is recorded in docs/mockups/themes/clear-chime.json. The candidate generator uses Dinah's existing contrast solver. The shared appearance package fills category colors and validates structure, text/control contrast and category separation. Neither approved surface palette required contrast adjustments. Dark added-line diff tint is 9 percent under the existing contrast rule.

Validation covers default selection, persistence, light/dark surfaces, Standard skin selection, appearance controls, system mode, existing theme preservation and editor regression behavior. Screenshots and run results are retained under build/clear-chime. This does not claim that the pre-existing Standard focus-paint contrast issue has been corrected or that every optional skin has been newly admitted with Clear Chime.

The appearance, clear-chime, editor and theming browser suites passed in 64 seconds. Targeted Go tests for desktop, appearance, profile and web passed. Both production binaries built successfully and were installed with staged and installed SHA-256 verification. The desktop version is 0.0.0-clear-chime-20261105. Install and appearance receipts under build/clear-chime record the prior binaries and appearance-library backup. The stored selection was verified as clear-chime with standard. The running desktop was not restarted; the person will restart it to load the new assets.

On 6 November, switching back to Clear Chime exposed a Standard light primary-button gradient contrast failure of 4.49:1 at the sampled text. The light accent now uses #005b5d and its hover token uses #004c4f. White text has 4.64:1 contrast even against the brightest endpoint of the Standard resting gradient. The admission threshold remains unchanged. The regression fixture now switches away and back, explicitly checks rendered skin admission, and checks the gradient endpoint independently of glyph placement. Dark colors and surface colors remain unchanged.

The appearance, clear-chime, and theming browser suites and focused desktop/web appearance tests passed. The light screenshot was inspected. Installed desktop build 0.0.0-clear-chime-return-20261106 with matching SHA256 and unchanged running process IDs. The prior executable and receipt are retained at the paths recorded in build/clear-chime-return/install-receipt.json. No restart or saved theme selection change was performed. The web source assets are updated but the web executable was not deployed.

