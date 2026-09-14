---
kind: decision
state: resolved
column: c9428b3bc921
owner: holder
ts: 2026-09-14T02:16:35Z
ordinal: 12
note: "Both answered by building origin/main at e50fbdb and running it; evidence table in the triage comment. The defect as filed is fixed: run builds the renderer from the language ladder before it tests the parse error, so DINAH_LANG, config and OS locale all localise a parse-time refusal, and the catalog translations do display. What survives is narrower and different in kind, namely that the --lang rung alone is positionally dependent, honoured before the offending word and ignored after it. No collision with dinah-31: its UnknownFormat refusal is raised on a parse that succeeded, so the renderer is fully built and all four rungs have run by the time that refusal is composed."
---
Is the parse-time English-only refusal still real against today's main, and does it collide with dinah-31's new contract.UnknownFormat refusal?