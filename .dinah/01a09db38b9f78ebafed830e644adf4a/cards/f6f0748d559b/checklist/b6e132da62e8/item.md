---
kind: decision
state: resolved
owner: operator
ts: 2026-09-14T02:17:30Z
ordinal: 33
note: "Paul ruled on 2026-09-11, at Operator Design Review. The fence is his own from this card's first decision, and design review had deliberately left its verification to a person reading the diff once, on the grounds that an automated check firing on any new command would fire falsely on the next card that legitimately adds one. He was offered the automated alternative explicitly and kept the human check: an honest human check beats a nuisance automatic one that cries wolf. Whoever implements this card performs that read and says so; it is one of the three checks on this card that are not tests, and the spec labels it as such rather than letting it read as automated."
---
The promise that this card adds no control which starts an agent stays a human check rather than an automated one.