# Passive page startup receiving progress

The page branch is based on master c399ac9d. It carries Engine's recorded-inventory amendment ba43df294a02ad33348223675cf431c0582ea405 with the earlier passive queries and judgement calibration correction. The Chats panel, chart and cleanup retain their common inventory projection. A chart relationship uses canonical chat identity while opening uses the current session reference; unresolved ownership stays visible with a reason and no opening target.

Startup reads SavedChats and DeclaredProviders. Explicit Refresh Chats asks each declared provider in the selected workspace and then rereads SavedChats, ignoring the intermediate provider responses. Successful explicit opening and saved-layout restoration refresh only the opened provider, coalesced by workspace and provider. A removal invalidates an outstanding inventory request before a replacement read can begin.

Fresh layouts no longer open a blank New Chat window. Restoring an old layout removes its blank composer while retaining saved conversation windows. Explicit New Chat remains available. Skills wait for composer focus or slash input, prices follow open providers with the declared capability, and hidden judgement settings do not read their calibration report. Manual calibration remains explicit.

## Verification completed

Eight headless page suites passed together after combining the recorded-inventory host: startup-passive, startup-layout, workspace-last-choice, org-chart, org-chart-stability, chat-cleanup, thinking-control and composer-fold. The run took 25 seconds. A preceding run also passed all eight after correcting fixtures to open their own test windows and supply shared inventory rows.

All seventeen counted cases passed before adding the recorded-inventory amendment. That run included zero-contact startup, explicit opening and saved-layout restoration for every offered provider, isolated query controls and a refusing executable. Its positive controls initially exposed two invented-peer errors: a missing Codex workspace and a Claude identity without its provider prefix. The peer now supplies both; the real ownership checks and acceptance assertions were retained. Open-state assertions wait for the authoritative state event.

The tagged host exposes StartupPageCounters and StartupPageTrace as diagnostic reads. The trace retains boundary observations without filtering or initiating work; native peer requests remain separate raw evidence. Machine-specific traces stay outside this public repository.

## Remaining gates

The combined counted run, the previous-visible-set comparison and independent receiving checks remain pending. The receiving journey must preserve identities, titles and states through rename, removal, restore and restart. No product acceptance or installation is claimed by the page checks above.

The hidden host serves the real embedded page and App bridge with invented isolated providers. It does not execute native Wails startup. Native Claude adapter maintenance and the separately recorded manual-calibration runtime crash remain outside this page-startup result.
