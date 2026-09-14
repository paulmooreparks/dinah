---
kind: decision
state: resolved
column: 0d86ad99cdbc
owner: holder
ts: 2026-09-14T02:18:01Z
ordinal: 26
note: A column has no update verb at all, where the description says each of four kinds has one; runColumn refuses any first word but `new` and no SetColumn exists. dinah-455 blames ResolveEntity and KindOfAnchor, but show refuses through ResolvePath and a failed ReadText of a directory, so a fix aimed only at ResolveEntity would leave the reported surface unchanged. dinah-454 says the JSON already carries an ordinal for a comment; verb.CommentView carries neither an ordinal nor a ref, only the identifier. Each was checked in the source at that commit, and the first and third were confirmed by running.
---
Three claims in this card's own description are wrong at trunk 22a35fc, and the spec records the corrections rather than carrying them forward.