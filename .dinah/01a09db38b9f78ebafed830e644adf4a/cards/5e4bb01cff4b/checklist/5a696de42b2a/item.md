---
kind: decision
state: resolved
ts: 2026-09-14T02:17:01Z
ordinal: 14
note: "dinah-249's code review deferred this exactly until a caller existed. The contract-token guard is that caller: it needs a translated catalog's raw entry (Text, Skeleton) for a tag that isn't Base, from outside package msg, which the unexported `loaded` map cannot serve."
---
Add `msg.CatalogEntry(tag, key string) (Entry, bool)`, exported, symmetric to `BaseEntry`.