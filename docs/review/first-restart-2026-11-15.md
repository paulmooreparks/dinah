# Historical first restart rehearsal

The historical source is ce4012d6f550389a11c16ddee708d9041f835fd9, committed on 13 November 2026 at 10:27:37 Singapore time. It is the last first-parent master commit before the requested 10:30 cutoff. The next first-parent commit is 6acbd3f1 at 10:44:03. This is a date-selected reconstruction of the running build, not a byte comparison with the person's running executable.

The destination is the requested 649016a1739a05559fb7fe42f442a67e1ac888dd, committed on 15 November at 11:46:21. Separate temporary worktrees hold both versions. Both module files replace the extension SDK with a sibling checkout. The first historical build failed because that sibling was absent. A separate temporary SDK checkout at 444129580fb6f8f8d3ce3248a09f8df97c6699ac, the latest SDK commit before the same cutoff, supplied the dependency. Both product builds then passed. No product source was modified.

| Desktop build | SHA-256 |
| --- | --- |
| Historical | 11f41875f34e73b15a0d176481598f9292c977f5cf48a5bd9fb5b48d059fe043 |
| Destination | f30b44668aa19c3bbf443fc8ac2e7870e8288e5d8aaaed7bda5cc634c34aa9d7 |

The desktop command is `go build -tags desktop,production -o <temporary executable> ./cmd/dinah-desktop`. The companion command is `go build -o <temporary executable> ./cmd/dinah`. Every build and hidden run goes through `scripts/with-test-lock.ps1`. The two product source trees are `%TEMP%/quality-first-restart-old` and `%TEMP%/quality-first-restart-new`; binaries are under `%TEMP%/quality-first-restart-builds`.

The driver is `cmd/dinah-desktop/frontend/e2e/first-restart-hidden.cjs`. It takes the old binary folder, new binary folder and old-source bus fixture executable. The fixture source is `scripts/first-restart-bus-fixture.go.txt`; copy it to a temporary command directory in the historical tree and build it there so it seeds the old bus through that version's own API. It only opens the explicitly isolated temporary home. It never connects to the live bus.

The fixture creates three workspaces, three named chats with an invented completed turn, open and closed chat windows, two pins, two group chats including unread messages from an invented sender, an unsent composer draft, an unsaved document draft, active and dismissed notifications, and settings supported by the historical build. A loopback Ollama stand-in supplies all agent responses. Product, provider, roaming and local data paths point to the temporary fixture. Browser requests outside loopback are blocked. Operating-system notifications are disabled in the invented settings.

The old program closes through its Quit command while the page remains available to complete its normal layout-save acknowledgement. The new program starts on the same home, then closes and starts a second time. The driver compares durable identities and contents, exercises the new pages, and finally uses Pause Chats and Restart on the carried state. Failure cleanup can stop only processes whose executable equals the fixture's private executable path.

The hidden native window serves the real page to headless Chromium through the existing shell-entry test mode. Its embedded native document remains blank. This verifies product persistence and the close handshake, but it does not prove retention of native browser-managed storage, native monitor placement or active work in real external providers. No real home or real provider is read.

## Results

The final rehearsal passed. Three initial runs exposed driver faults. The bus fixture inherited an isolated TMP beneath its own home and refused its safety check; giving that child the actual temporary root corrected the check. The close driver then timed out and was changed to observe and answer the normal pending-exit confirmation while retaining the page for its flush acknowledgement. The next run closed normally with zero working chats, terminals or unfinished requests. That run passed the initial carried-state assertions but compared the bus fixture's raw Go message against a lower-case field name. The fixture returns `ID` and `Text`, while the page returns `id` and `text`. The corrected assertion compares both identity and body.

These failures remain in the evidence folders `dinah-first-restart-Q8wV51`, `dinah-first-restart-rcgalc` and `dinah-first-restart-dgkPb1` under the temporary root. They do not establish a product fault or a completed rehearsal. The final run has its own receipt.

The final FIFO run, ticket 1942, exited zero. Evidence is in `%TEMP%/dinah-first-restart-h2fd7c/result.json` and `carried-state.png`. All 53 driver assertions passed, with no page errors. A comparison of the saved receipts also confirmed that all three workspace records and both groups' names and membership remained unchanged on both new starts.

| Check | Result |
| --- | --- |
| Historical normal close and layout flush | Pass |
| Three workspaces and three chat identities | Pass |
| Completed chat history | Pass |
| Open windows and closed chat window | Pass |
| Two taskbar pins | Pass |
| Composer draft and unsaved document draft | Pass |
| Active notification and persisted dismissal | Pass |
| Historical settings after the old program saved them | Pass |
| Both groups and their membership | Pass |
| Unread group and new bus messages after each start | Pass |
| Decisions and Set-aside Programs opening | Pass |
| Restart state, cancellation call, and actual Pause Chats and Restart | Pass |
| Second start without duplicate sessions, drafts or notices | Pass |

No state loss was observed. This bounds the result to invented state, idle provider turns and the selected source commits. The test checks repeated-start effects on the seeded objects; it is not an exhaustive proof of every migration. The historical program canonicalizes settings before the comparison, so the check preserves what that build actually saved. The public source and report contain no private product names. No push, merge, installation into a real home or live desktop input occurred.
