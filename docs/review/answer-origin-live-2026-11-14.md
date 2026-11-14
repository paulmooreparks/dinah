# Decisions answer origin in the hidden program

Engine verified master d6e00dea82ce9846647f76a3f8831247199099aa on branch engine/answer-origin-live. The passing journey needed no production correction. This branch adds only a repeatable hidden integration harness and this evidence.

## Joined journey

The real desktop starts with a fresh isolated DINAH_HOME and DINAH_SHELL_ENTRY_FILE. Headless Edge loads its authenticated shell address. The existing ACP Decisions stand-in receives the host's MCP configuration, launches the supplied bus helper and calls ask_person with two numbered questions. The harness neither replaces bridge methods nor inserts database or transcript fixtures.

The browser selects As Recommended for question 1, writes an answer for question 2 and sends both together through the Decisions window. The asking agent receives one readable bus reply containing both questions and resolved answers. Its transcript contains one typed answer card, in question order, with the recommendation marker only on question 1.

| Check | Observed result |
| --- | --- |
| The agent asks through the actual MCP helper. | ask_person succeeds and the request appears in Decisions. |
| The person sends both answers through the window. | The open request closes and the stand-in receives both resolved answers. |
| The transcript retains the typed origin. | Its request identity and two snapshots match, with asRecommended values true and false. |
| The page reloads. | The same single card and saved origin remain. |
| The hidden program restarts. | PauseAndRestart ends the original process. A new authenticated shell entry appears, and the reopened chat retains the same card and origin. |
| Both answers can be read in the fixed card. | The body scrolls from the first question to the second. Geometry assertions place the complete second answer inside its scroll area at every stage. |
| The answer's link opens the closed request. | The real lookup returns closed and readOnly. The page shows both answers and offers no answer input or Send Answers button. |
| The page reports errors. | There are no page errors. |

The screenshots were inspected. The final second-question screenshot visibly shows the written answer after the program restart. The closed-request screenshot shows the read-only explanation. The card scroll area has a client height of 174 pixels and a content height of 303 pixels; its second question is visible at scrollTop 129.

## Reproduction and evidence

Build the desktop with desktop,production tags and -ldflags '-H windowsgui', and build ./cmd/dinah-desktop/testdata/decisions-agent separately. Both builds and every joined run used scripts/with-test-lock.ps1. Set NODE_PATH to an existing playwright-core installation, then run:

```powershell
scripts/with-test-lock.ps1 node cmd/dinah-desktop/frontend/e2e/answer-origin-hidden.cjs <desktop.exe> <agent.exe> <fresh-output-folder>
```

The browser defaults to the installed Edge executable. ANSWER_ORIGIN_BROWSER can select another Chromium executable. The final evidence is in %TEMP%/answer-origin-live-04. It includes receipt.json, agent.jsonl, the isolated home, application logs, first/second-question screenshots for all three stages, and closed-request.png. The receipt stores typed origin snapshots and the closed lookup response. It does not store the authenticated entry address.

| Artifact | SHA-256 |
| --- | --- |
| Desktop executable | 57de21f85813a9e19edc04b2c03ae55c88679b2cd9bef1d340a2b7ab16687e57 |
| Stand-in executable | 5adf821268b701b37220bd168fcd736a577d56ec544649bea56b701d5b968110 |
| Final receipt | 7a01dc88d977b3b2e1077957401199885a68d64da7a5946d4142dcfb15f17aee |

The first invocation stopped before launching the product because the harness requested playwright instead of the installed playwright-core. After correcting the module and browser selection, the joined journey passed. Runs 01 through 03 also passed its data, reload, restart and navigation assertions, but their element screenshots did not expose the second answer. The final harness uses page screenshots and checks rendered scroll geometry before capturing the second answer. The final fresh-home run exited zero. Assertions were strengthened, and no product assertion was removed.

## Limits and handoff

The native program stayed hidden and served the real page to headless Edge. Native WebView2 rendering and storage were not exercised. The stand-in is compliant ACP, without real model calls. The saved card is visible while its provider reconnects after restart; the test proves the persisted transcript and closed lookup, rather than a fresh model turn after restart. This is a direct-chat request, not a group request or a plain mention.

No full browser suite or unrelated Go suite ran. Existing Go and browser tests were unchanged. All owned desktop and stand-in processes had exited at the final check. Invented evidence folders remain under the temporary folder for review. Nothing was read from or written to the person's real application homes, and no credential or token files were read or copied. No visible window, shared desktop input, push, merge or installation occurred.

The branch is ready for Lead review of the harness and evidence.
