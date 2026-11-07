# Clipboard feedback

Successful clipboard actions do not create success toasts, Notifications entries or badge counts. Copy buttons show a brief checkmark, with Copied text where the existing width permits it, and restore their original appearance after 1.5 seconds. Their geometry stays fixed. Menu commands close normally without visible success feedback. A polite live region announces Copied.

The shared clipboard helper handles native and browser writes. False results, rejected promises, synchronous exceptions and missing clipboard support remain actionable errors. Feedback appears only after success. Chat message buttons, pinned prompts and Editor path buttons use local confirmation. Link and resource context menus use the quiet shared helper. The Skills command copy also uses the helper so failures are no longer silently swallowed. Existing diagnostic snapshot guidance remains inline in Settings.

This change was installed on 6 November 2026 as 0.0.0-copy-feedback-20261106. The installed executable matches the build hash. The previous executable is preserved for rollback, and Dinah was left running for the person to restart. The installation receipt is build/copy-feedback/install-receipt.json. Focused browser evidence is retained under build/copy-feedback.

