# Decisions end-to-end verification

Quality checked master `93a46489eb690a828a3701639180c7cdc03c21ad` on branch `quality/decisions-live`. This branch adds a repeatable hidden integration harness and an ACP stand-in. The passing run required no production correction.

## Joined path

The real desktop starts with an isolated `DINAH_HOME` and `DINAH_SHELL_ENTRY_FILE`. Headless Chromium loads the shell's authenticated entry address. The test uses the real window bridge and Decisions controls, without replacing their methods or inserting fixture rows.

The stand-in receives the host's MCP server configuration in `session/new`. It starts that supplied bus helper with `mcp.CommandTransport` and calls `ask_person` over MCP. The helper is the tested desktop executable in bus-server mode. The request enters the durable bus, appears in Decisions, and the person's replies return as actual ACP prompts to the stand-in. The stand-in records only the first text block of each prompt, excluding appended authorization context.

## Results

The final run exited zero and recorded no page errors.

| Required behavior | Observation |
| --- | --- |
| An agent asks two numbered questions through MCP. | `ask_person` returned a successful receipt for request revision 1 with questions 1 and 2. |
| Decisions shows both recommendations. | The request detail displayed “Use the reviewed release.” and “Proceed tomorrow.” |
| As Recommended answers only the first question. | Clicking As Recommended alone left revision 1 unchanged. Send Answers committed the stored recommendation, advanced the revision to 2 and left question 2 unanswered. |
| A written second answer closes the request. | Sending “Proceed next Monday after review.” removed the request from both the backend open list and the rendered request list. |
| The stand-in receives two readable replies. | Two separate ACP prompts contained the corresponding question number, full question text and answer. Neither depended on the other reply to explain the decision. |
| A group mention appears as a plain row and can be dismissed. | The stand-in used MCP `send` to address the person in Fixture Review. Decisions returned a mention with no questions. Dismiss Message and its confirmation removed the open row. |
| Broadcast to Active previews and delivers only to working chats. | One chat held an active turn; the asker and another chat were idle. The preview showed one target named Fixture Working. Confirmation committed one message. After its held turn ended, Fixture Working received that message as a new ACP prompt. The idle chat received no prompt. |

The broadcast receipt contains one message ID, matching the one-target frozen preview and the delivered prompt. The test checks the preview before confirming, rather than calling the confirmation API directly. This stand-in has no steering hook, so the broadcast waits on the bus until the current turn ends. Delivery into a still-running turn was not exercised.

## Reproduction and evidence

All builds and execution used `scripts/with-test-lock.ps1`. Build the desktop with `go build -tags 'desktop,production' -ldflags '-H windowsgui'`, and build `./cmd/dinah-desktop/testdata/decisions-agent` separately. Set `NODE_PATH` to an existing Playwright installation, then run:

```powershell
scripts/with-test-lock.ps1 node cmd/dinah-desktop/frontend/e2e/decisions-hidden.cjs <desktop.exe> <agent.exe> <fresh-output-folder>
```

The final artifacts are under `%TEMP%/dinah-decisions-live-02`. `receipt.json` contains the request, partial revision, delivered replies, mention, frozen broadcast preview and committed receipt. `agent.jsonl` records MCP results and received prompt text. The isolated application's logs remain under `home/logs`.

| Artifact | SHA-256 |
| --- | --- |
| Desktop executable | `884e6766b075864b9c59a06b44938c1fafdc529f5799e85acd3f7ed9bbd1645c` |
| Stand-in executable | `9124915246150728351ce04159e520b7fbbfdaea20795f1d360a55f2934a3f10` |
| Final receipt | `951847c7f08fb85437c7b7ac046e7b8783604bacdb55dbb1fbb331ca472abfc8` |

The initial run in `%TEMP%/dinah-decisions-live-01` passed the question and answer path but failed when the harness searched for “Dismiss” instead of the actual “Dismiss Message” button. That failed receipt and page text remain retained. The selector was corrected before the successful fresh-home run.

## Limits

The requested journeys were demonstrated with a compliant stand-in. No real model, visible window, shared-desktop mouse or keyboard, live bus, installation or restart was involved. The native shell stayed hidden while headless Chromium exercised its served page. Native WebView2 storage and visual rendering were not verified. Broadcast to All, mid-turn steering, crash recovery and lost-response reconciliation were outside this assignment. Existing unit and fixture suites cover additional cases, but they were not rerun or treated as evidence for this joined run. No full browser or repository suite ran.

All fixture processes exited. The branch is ready for lead review of the harness and evidence.
