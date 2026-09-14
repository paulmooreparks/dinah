---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:17:32Z
ordinal: 13
note: "Verified by internal/verb/link_test.go:TestNoVerbRefusesOnAccountOfALink's closing assertions, which compare the card's title, severity, priority and tier across a link write and an unlink, and separately assert the claim holder is unchanged. The column and state are covered structurally as well: Link and Unlink set only Card.Links before Save, and no other field is touched anywhere in internal/verb/link.go."
---
Neither `link` nor `unlink` changes the source card's column, state, holder, or any level field; a `dinah show` of the card immediately after either call reports every other field unchanged from before the call.