---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:17:31Z
ordinal: 5
note: Verified by internal/verb/link_test.go:TestLinkRefusesItsPreconditionsInTheProfilesOrder, whose malformed cases drive an all-whitespace kind and an all-whitespace to for both verbs, assert contract.Malformed, assert the detail names the field, and compare the whole anchor before and after.
---
Given an empty kind or an empty `to` argument, `link` refuses `malformed` naming the empty field and writes nothing.