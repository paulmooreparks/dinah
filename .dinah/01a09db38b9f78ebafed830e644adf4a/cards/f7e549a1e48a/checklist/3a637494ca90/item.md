---
kind: decision
state: resolved
ts: 2026-09-14T02:17:20Z
ordinal: 6
note: "Confirmed by reading internal/verb/mutate.go:230-237 and its own comment: \"A claim cannot take a card somebody is already working, its own asker included.\" The refusal exists and fires; what it lacks is a way to tell a real conflict from the asker's own settling claim."
---
The state machine already refuses a second claim while a card is active, whoever asks (claimableState, internal/verb/mutate.go:230-237). This card is not "add a refusal that is missing"; it is "give the refusal, and the record, something besides a shared label to compare."