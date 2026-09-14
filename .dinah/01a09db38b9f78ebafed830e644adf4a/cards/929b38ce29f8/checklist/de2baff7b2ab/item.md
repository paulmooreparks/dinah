---
kind: decision
state: resolved
ts: 2026-09-14T02:16:54Z
ordinal: 19
note: "The first draft argued from a false premise and reached the right answer, which would have misled the next reader of this spec. The document already cites documents outside itself in prose and rests on them substantively: RFC 2119 and RFC 8174 in section 3.1, RFC 8259 in sections 3.5 and 5.7, and Unicode in 5.6, with section 3.5 speaking of \"the specifications this document cites\". Section 1's sentence also carries two clauses scoped differently, one barring anything else from being required reading and one barring citation inside a normative statement, and the paragraph clears both independently. The condition a later pointer inherits is stated in the spec: no keyword, a non-normative marker, and the disclaimer in the same paragraph. Ruling recorded by Agent Design Review, comment 5869."
---
The spec's citation framing is corrected: a prose citation is ordinary in this document, and only naming a product and a repository-relative path is new.