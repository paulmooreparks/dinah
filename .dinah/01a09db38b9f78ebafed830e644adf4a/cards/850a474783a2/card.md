---
title: Two malformed sites put the reader's own word where every other site puts a slot name
column: 5ea2db0272fc
state: ready
severity: minor
priority: soon
---
`verb.ParseDuration` refuses `malformed` with the text the reader typed, so `dinah claim scr-1 --expires 5x` prints `malformed 5x is missing, empty, or will not parse`. Every other raise site of that refusal passes the name of the slot instead, as in `malformed title is missing, empty, or will not parse`, and the sentence around the value reads as though the value were a slot name.

dinah-102 works around it by writing the next step so that it names only how the command is spelled. This card asks whether those two sites should name the slot and carry the offending word as a named value, which would let the sentence say both.
