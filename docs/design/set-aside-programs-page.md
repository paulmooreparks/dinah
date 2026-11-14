# Set-aside programs in Settings

Settings has a Set-aside Programs section whenever the host supplies ListSetAsidePrograms and RemoveSetAsidePrograms. It uses the service contract in docs/design/set-aside-programs.md. The profile defines the programs, so the same section serves distributions without customer names or product-specific branches in the host.

The section reuses the Settings card, PUDL controls and update-row styles. It inherits the current surface, text, border and accent tokens and the existing font. A fixed scrolling list contains fixed-height rows with a selection checkbox, name, date, size and protection reason. New identities append, and missing identities keep a disabled row in place. The status, warning, selected-total and removal-result regions reserve their height. Only their contents scroll when the inventory changes.

Selection is limited to one hundred currently eligible opaque IDs. Refresh discards selection of identities that vanished or became protected, without selecting replacements. Remove first lists the selected names and their logical total in the existing confirmation dialog. Cancel changes nothing. Confirmation disables selection and refresh until the request settles. The service remains responsible for identity, process and fallback protection.

Every removal result is shown, including mixed refusals. A rejected or malformed response does not establish removal; the page says completion is unconfirmed, clears selection and lists again. It does not retry removal automatically. Another attempt requires a fresh selection and confirmation. Inspection warnings remain visible and file sizes are disclosed as logical lengths rather than guaranteed physical disk recovery.

Focused headless fixtures cover inventory, protected rows, empty and unsupported states, selection totals, confirmation cancellation, exact-ID removal, mixed results, lost responses, stale reads and fixed geometry. They use invented data and never call the real installation service. The Architectural Principles remain at `C:\Users\paul\OneDrive\Documents\Architectural Principles.md`.
