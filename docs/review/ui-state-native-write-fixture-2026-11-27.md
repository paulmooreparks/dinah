# UI-state serialized-write fixture

The browser test held every SaveUIState invocation and released only the first two. The real flush drains changes arriving while a save is in progress, including layout and scroll updates. A third native save could therefore be parked forever by the fixture while the two explicit flush promises waited for it. Raising the timeout would leave that deadlock intact.

The test now intercepts SaveUIStateChanges, the page's native delta contract. It holds only writes containing its invented test-serialized key and passes unrelated changes to the completing native fixture. The two controlled versions, failed-save rejection and subsequent recovery checks remain. A background change is deliberately introduced while the second write is held, and its persisted value is checked after both flushes complete. No product source or visible interface changes.

## Verification

Focused browser results and the staged guard are recorded in the handoff. A temporary control uses the original fixture with the same injected background change. Evidence folders are %TEMP%/dinah-ui-state-write-before and dinah-ui-state-write-after. No GitHub retry, push or installation is part of this repair.
