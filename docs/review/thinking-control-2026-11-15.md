# Thinking control review

The candidate is on frontend/thinking-control, based on master 26d67f03. Product source is ca686e6e14a3768baa592da80a08d4f596b1d3ab. The final browser fixture and viewport assertions are aac07c733113072eed2d89ecab6f52b665675e79.

The chat toolbar now offers Thinking immediately after Model when the chat reports select options in category thought_level. Each option uses the agent's ID, values, labels and descriptions. Several reported options remain separate choices within the Thinking menu. A missing option produces no control. PUDL folds the control with its Agent group at narrow widths, retaining the Thinking heading and checked choice. Its tooltip and accessible name include the current value when the toolbar uses glyphs alone.

Selection uses the existing SetSessionOption path. The host already saves per-chat choices, applies the model first on reopen and handles unavailable dependent choices through its substitution rules. The page adds no persistence layer or provider mapping. The pending new-chat choice and provider defaults use the existing shared option path too.

The documented inventory and source links are in [thinking-control.md](../design/thinking-control.md). Claude's pinned adapter reports effort for supporting models. Devin documents a separate Thinking level and ACP configuration controls; it is not limited to effort encoded in model names. The actual option catalogue still determines what Dinah displays. This work neither upgrades an agent nor claims that an installed agent exposes a documented upstream capability.

## Verification

The first combined run passed app-ui3, provider-model-prefs and thinking-control in 21 seconds under FIFO ticket 2216. The existing host option persistence and restore checks passed in 0.136 seconds under ticket 2217. Those include model-first restoration using the updated catalogue.

The final sizing rules passed thinking-control and composer-fold together in 22 seconds. The final evidence run passed thinking-control in five seconds under ticket 2232. It verifies the agent's names, the selected ID and value passed to the ordinary per-chat call, absence when the option disappears, return of the authoritative current value, and preservation after refusal. Its 72 frame samples cover long and short current values at 1440 and 390 pixels. The toolbar, composer and text entry keep their geometry, and the chat fits the viewport. All four final screenshots were visually inspected, including the open Thinking menu and the narrow folded menu.

A trial fixture forced a floating chat wider than the available reading surface. Its screenshots were unsuitable evidence even though the old composer-only overflow assertion passed. The final fixture uses the actual maximized reading surface and explicitly checks the whole window against the viewport. The supplied evidence uses only that corrected fixture.

Frontend ownership, release-asset and UTF-8 checks passed in 0.427 seconds on the final product source. Whitespace checks passed. The browser command was `pwsh ./cmd/dinah-desktop/frontend/e2e/run.ps1 -Only thinking-control -Jobs 1 -Out $env:TEMP/thinking-control-final2-20261115`.

The evidence is in [the review directory](thinking-control-2026-11-15/). Browser checks use invented options and a stand-in host. Actual native agent startup, account use and a product restart are outside these checks. No visible window, real home, credentials or remote write was used. Nothing was pushed, merged or installed.

## Retained follow-up

The Team Lead accepted the read-only chart in message 8866. Its next stage must space different group lanes by at least a gutter's width. That improvement is not part of this Thinking change. The Import from Codex page remains queued after this handover and needs Release's reviewed host contract.

## Cleanup integration

The branch was rebased onto master db6f8d1131ceae0cefa6a3e8e08848f16d2b2584 at the Team Lead's request. The app.css conflict keeps both the cleanup styles and Thinking sizing. The chat command conflict keeps Delete Chat, its Recycle Bin operation and Clean Up Chats, adding only Thinking after Model. Product source is now bfd5384a and the viewport fixture is a907f89f. The previous source IDs above are historical references from before the rebase.

The rebased branch passed chat-cleanup, composer-fold and thinking-control together in 27 seconds. The command used -Only thinking-control,composer-fold,chat-cleanup and output folder %TEMP%/thinking-cleanup-rebased-20261115. The attached browser log now records that combined run. No cleanup behaviour or host code was changed by the resolution.
