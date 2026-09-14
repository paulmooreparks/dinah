---
kind: decision
state: resolved
owner: holder
ts: 2026-09-14T02:16:58Z
ordinal: 13
note: "Andoneer's own copy of the recorder already records the reason at andon-730 D-1: one shared exception handler between a guard and a recorder would silently turn the guard advisory the day somebody edits the shared code for the recorder's sake. That reasoning applies here without change."
---
The recorder (record-isolation-cwd.py) and the new guard (deny-main-checkout-cwd.py) stay two files with two independent PreToolUse registrations, never merged.