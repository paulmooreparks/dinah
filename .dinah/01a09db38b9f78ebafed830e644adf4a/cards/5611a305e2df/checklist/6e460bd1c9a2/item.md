---
kind: acceptance_criterion
state: verified
column: aa6cd1c6ae5f
ts: 2026-09-14T02:17:40Z
ordinal: 6
note: "Armed. Red: 'the 7 entries counting \"the verbs that change where a card stands\" disagree about the figure or about how it is held:', followed by all seven entries on their own lines, six reading derives=contractVerbs and internal/guide/guides/verbs.md:3 reading holds=prose. Restored, green."
---
The consistency rule catches an entry escaping a derivation its siblings still carry. Arming: change the entry for `internal/guide/guides/verbs.md:3` from `derives=contractVerbs` to `holds=prose reason=nobody declares this set`, leaving its `counts=` phrase alone, run `go test ./cmd/dinah -run TestEveryCountedSetIsCountedConsistently`, and watch it fail naming the phrase "the verbs that change where a card stands", the seven entries carrying it, and the one entry holding it differently from the other six. Restore the entry and watch the run go green.