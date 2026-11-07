# Overnight responsiveness investigation

Dinah became severely sluggish after remaining open overnight. This is a release blocker. The running instance has been preserved. The diagnostic build was installed on 6 November at 08:25; restart remains under the person's control.

## Live evidence

The installed executable is version `0.0.0-scroll-wheel-20261105`, SHA-256 `D6EBA1BF8364C2A2301CB484318A29B2F714D10227F33750B5E366B7A3E5908F`. Its main process began on 5 November at 15:41. The retained timing snapshot contains a 715 ms main-thread task and a 337 ms input delay. These values establish stalls but do not identify their cause.

The renderer used about 735 MB of private memory during this investigation. A single process sample does not establish a leak. The installed production build omits Wails' `devtools` build tag, so its developer-tools shortcuts are disabled. No undocumented runtime injection or restart was used to circumvent that boundary.

## Confirmed defects and corrections

The transcript unread tracker retained every previously observed message entry in a Map. Entries contained strong DOM references even after trimming removed their rows. Five replacements of 200 rows retained 1,000 rows after garbage collection while only 200 remained displayed. The tracker now retains only the current keys and clears unread references on destruction. The same test retains 200 rows after each replacement.

Folded tool calls bypassed the transcript limit because trimming counted only top-level rows. An uninterrupted run of 5,000 calls retained all 5,000 calls and 40,009 transcript nodes. Trimming now counts nested activity rows, removes the oldest calls within an oversized fold, updates the fold summary, and releases discarded queued tool renders. The 600-row threshold and 450-row trim target remain. The saved-history action continues to provide older content.

After the correction, the same run retained 470 calls and 3,773 transcript nodes. Chromium's reported embedder heap after forced garbage collection decreased from 64,002,920 to 16,206,896 bytes in the isolated reproduction. JavaScript used heap decreased from 11,172,244 to 5,440,268 bytes. These measurements apply to the synthetic fixture, not the running desktop process.

## Responsiveness limits

Typing into the idle fixture after the 5,000-call run did not reproduce the live stalls. Input queue delay had the same 0.3 ms p95 before and after. Time to the animation-frame callback had p95 4.6 ms before and 12.0 ms after, with maxima 15.3 and 12.2 ms. These single runs do not demonstrate a typing improvement. Both results are retained, including the unfavorable comparison.

The retention corrections are verified defect fixes, but the overnight responsiveness blocker remains open. A diagnostic build with local DevTools support is needed for attributed CPU and heap captures if the installed build cannot expose them. Restarting would discard the existing renderer's accumulated heap, so it requires a deliberate transition after preserving the available evidence and the person's work.

## Evidence

Evidence is under `build/leak-20261106/` in the integration checkout. The `before` directory records the failing transcript retention test. `tools-before` and `soak-before` record unbounded tool growth. `soak-after` records the bounded transcript references and `soak-after-tools` records the bounded 5,000-call run. `input-before` and `input-after` retain all 60 input samples for each variant. `live-timing.txt` and `process-initial.json` preserve the live measurements.

The regression suites are `transcript-retention.js` and `chat-tool-retention.js`. Existing unread, scrolling and streaming checks pass with the corrections. The Last Read, scrollback and history-loading suites also pass. The destroy retention assertion initially ran before pending DOM mutations had been delivered; that failure is retained under `lifecycle`. After allowing two animation frames, the same assertion passes under `destroy-settled`.

## Diagnostic build

The executable is `build/leak-20261106/diagnostic/dinah-desktop.exe`, version `0.0.0-retention-diagnostic-20261106`, SHA-256 `7DC6A730494FB05229E8BF3AD1658C2A5320FE90BC59F9C4282DC50CE44C67CE`. It was built with `desktop,production,devtools`. The person authorized installation in chat. It was installed at 08:25 on 6 November with its SHA-256 verified. The previous executable is preserved at `C:/Users/paul/.dinah/bin/dinah-desktop.exe.before-retention-20261106-082556`. The running instance was left open for the person to save work and restart. The installation receipt is `build/leak-20261106/diagnostic/install-receipt.json`. The next diagnostic step is to capture attributed renderer CPU and heap data in this build if sluggishness persists.
