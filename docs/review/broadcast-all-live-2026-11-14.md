# Broadcast to All hidden integration check

Quality checked master `5db16def0bf1fbc25d46dafd15f33c6a9a759dae` on branch `quality/broadcast-all`. The work uses the real desktop program, an isolated home and headless Chromium. No application method is replaced by a fixture implementation.

## Test path

The harness creates five stand-in chats and completes a seed turn in each, so each chat is eligible to reopen. It then terminates only its own hidden process and starts the same executable with the same isolated home. None of the chats has an open window. The background limit is two.

The Decisions controls create a Broadcast to All preview and confirm it. The stand-ins hold their broadcast turns until the harness writes a release file for that session. Two chats must receive their messages while the other three wait. The harness releases each wave and allows the production one-minute idle handoff to admit the next wave. Session events and actual ACP prompts are retained. Repeating the committed confirmation through the real bridge must return the same receipt without sending another message.

A separate preview is left in the confirmation dialog until its real five-minute expiry. Confirming then must produce the backend's `preview_expired` receipt and show its reason in the window, with no messages committed. The test changes neither the system clock nor the bus's expiry values.

## Evidence

The run uses `%TEMP%/dinah-broadcast-all-01`. It retains `receipt.json`, `agent.jsonl`, the hidden desktop log and the isolated home. Both builds exited zero through the shared FIFO.

The delivery portion completed with five distinct message IDs and one actual ACP broadcast prompt per original session. Replaying the same confirmation identity returned an identical receipt. Session events recorded a maximum of two background slots. The first pair reopened at 00:54:28 UTC and finished at 00:54:34 UTC; the second pair reopened at 00:55:35 UTC and finished at 00:55:36 UTC. The final chat reopened at 00:56:37 UTC and finished at 00:56:38 UTC after the next idle handoff.

The full integration run exited zero on its first execution and recorded no page errors. After the real five-minute deadline, confirmation returned a rejected receipt with reason code `preview_expired` and no message IDs. The window displayed "This broadcast preview expired. Preview it again before sending." No stand-in received that message. All five original deliveries still had one prompt each at the end of the expiry observation. No production correction was needed.

The final receipt SHA-256 is `d75547328ffaaaf710de98c2197dcd623f08f9ae7393673479c286ba175d2da1`. Cleanup stopped the owned hidden desktop and stand-in processes; a process inventory confirmed none remained. No push, merge or installation was performed.

The tested desktop SHA-256 is `b63cfb050d61acc5e58016797897d7aaf57e00e4738d533a7d24803ae233fed7`. The stand-in SHA-256 is `92602afbe3d07ed4b9fb66fe07cd1fa47a1ec3b0fa854240f1dfc53023e560f9`.

## Reproduction

Run these commands from the worktree. The output folder must not exist. The test takes roughly eight minutes because it observes the production handoff and expiry periods.

```powershell
& scripts/with-test-lock.ps1 go build -tags 'desktop,production' -ldflags '-H windowsgui' -o "$env:TEMP/dinah-broadcast-desktop.exe" ./cmd/dinah-desktop
& scripts/with-test-lock.ps1 go build -o "$env:TEMP/dinah-broadcast-agent.exe" ./cmd/dinah-desktop/testdata/decisions-agent
$env:NODE_PATH='C:/Users/paul/source/repos/pudl/node_modules'
& scripts/with-test-lock.ps1 node cmd/dinah-desktop/frontend/e2e/broadcast-all-hidden.cjs "$env:TEMP/dinah-broadcast-desktop.exe" "$env:TEMP/dinah-broadcast-agent.exe" "$env:TEMP/dinah-broadcast-all-01"
```

## Limits

The ACP stand-in accepts session loading and ends turns when released. It does not replay a model's history or exercise a real provider. The setup restart is a hidden process restart, not another test of Pause Chats and Restart. All five recipients belong to this isolated desktop; remote terminals and unavailable providers are outside this check.

The browser runs headless. Native WebView2 rendering, shared desktop input and native focus are not exercised. The test does not change the installed program, the live bus or the person's home. The full Go and browser suites are not part of this task.



