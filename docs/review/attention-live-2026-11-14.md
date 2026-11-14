# Attention and Decisions hidden integration check

Quality checked master `205cfd81d3c88d623d3068dfc4faefc79b08a523` on branch `quality/attention-live`. The first integration run passed without a production correction. This branch adds the hidden journey and a second message variant to the existing ACP stand-in.

## Joined behavior

The real desktop runs with an isolated `DINAH_HOME` and `DINAH_SHELL_ENTRY_FILE`. Headless Chromium uses the authenticated shell entry. The stand-in invokes the desktop's supplied bus MCP server for group messages and `ask_person`; the harness does not insert notification records or replace application methods. Native notifications are disabled in this isolated home before messages are sent.

| Check | Observed result |
| --- | --- |
| A group mention arrives while its group window is covered. | Notifications displayed one retained card with one request. The group window remained mounted behind Notifications. |
| The covered group is read and refreshed. | A real `BusGroupMessages` read and `dinahBus.load()` refresh preserved the same card identity. |
| The same sender posts another mention. | One card displayed two requests and the newer first line. The second line was absent. |
| A different chat calls `ask_person`. | The status count rose from two plain mentions to three rows. A separate request card appeared. |
| The program restarts with that request unanswered. | The new hidden process restored the two-request group card, the one-request question card and status count three. The request still had revision one. |
| The grouped mention card is opened. | Its Open Decision action selected the newer mention's row and acknowledged both retained notifications. The two plain rows remained until dismissed through Decisions. |
| The question card is opened. | Its Open Decision action selected the request's own row and displayed its recommendation. Acknowledgement cleared the card while the request remained open. |
| The request is answered. | The row left the list and the status count fell to zero. |
| A fresh request is answered without opening its card. | The harness entered through the status bar, verified that its card still existed, and answered both questions. The count fell from one to zero and the card disappeared. |

## Meaning of opening the group card

A plain group mention is now also a Decisions row. Its card therefore offers Open Decision, as the accepted entry-point implementation specifies. This check demonstrates that actual route and its acknowledgement. It does not claim that the card opened the group window directly. No route was changed to satisfy the older wording of the assignment.

## Evidence and reproduction

Both builds exited zero through the shared FIFO. The first integration execution exited zero with no page errors. Artifacts are in `%TEMP%/dinah-attention-live-01`, including `receipt.json`, `agent.jsonl`, the hidden process log and isolated home. The receipt records the before/after restart cards and final empty count and notification list. A process inventory after cleanup found no remaining owned desktop or stand-in process.

The desktop SHA-256 is `9388fde69f466f6f75a300e28af16724826d5a040baa06429e38a779230fa018`. The stand-in SHA-256 is `58579b5ec705c760bbb65f51453d20ad96a2cbb858d754710bbe69c22f806130`. The receipt SHA-256 is `ff9123949571f2fc2fb1c5aaeaca3fe142d2657d231755835d1f4402fe670629`.

Run from the worktree with a fresh output folder:

```powershell
& scripts/with-test-lock.ps1 go build -tags 'desktop,production' -ldflags '-H windowsgui' -o "$env:TEMP/dinah-attention-desktop.exe" ./cmd/dinah-desktop
& scripts/with-test-lock.ps1 go build -o "$env:TEMP/dinah-attention-agent.exe" ./cmd/dinah-desktop/testdata/decisions-agent
$env:NODE_PATH='C:/Users/paul/source/repos/pudl/node_modules'
& scripts/with-test-lock.ps1 node cmd/dinah-desktop/frontend/e2e/attention-hidden.cjs "$env:TEMP/dinah-attention-desktop.exe" "$env:TEMP/dinah-attention-agent.exe" "$env:TEMP/dinah-attention-live-01"
```

## Limits

The restart flushes the page's persisted UI state, stops only the owned hidden process, and starts the same binary with the same home. It tests startup restoration, not Pause Chats and Restart or crash recovery during a write. The stand-in is a cooperative ACP implementation; no real model is used. Native WebView2 rendering, operating-system notifications and desktop focus are not tested. The full Go and browser suites were not run. Nothing was pushed, merged or installed.
