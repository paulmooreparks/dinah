---
kind: decision
state: resolved
column: 4b38abe7ebd5
owner: holder
ts: 2026-09-14T02:18:13Z
ordinal: 45
note: "Round 2's second major says `scripts/derive_event_counts.py` exits 1 on `docs/design/format.md:20`, where a card identifier appears unbackticked, and that the fix is one pair of backticks. The script's own check is `BOARD_REFERENCE.findall(text)` over the whole document with the pattern `dinah-[0-9]+`, and nothing in it exempts a backticked span, so backticking the identifier leaves the script red. I tried it and it was red.\n\nThe sentence now reads that the board settled the vocabulary rather than naming the card that settled it, which is what the check is asking for, and the script exits 0.\n\nRecorded because it is a spec-side prediction found wrong on the day, which is worth more to the next reader than a silent correction."
---
The document's bare card identifier is removed rather than backticked, because the check the second review predicted a backtick would satisfy does not read backticks.