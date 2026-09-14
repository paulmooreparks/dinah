---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:17:33Z
ordinal: 19
note: "Added as cmd/dinah/compat_test.go:TestLinkAndUnlinkRoundTripOnEveryFixture, one subtest per manifest row. All eight pass: dinah-core-0.4, 0.5, 0.6, 0.7, 0.9, 0.12, 1.0 and 1.0-pre-slug. Each copies the fixture, runs the migrations the tool prescribes for opening an older workbench (which is what every command meets, not a condition link imposes), snapshots the card anchor after that, links, asserts the entry landed, unlinks, and requires the anchor byte-identical to the snapshot. The pre-slug fixture carries one live card and links it to itself, which the format gives no ground to refuse, so no fixture is skipped. Armed: with Save's Delete(\"links\") clause disabled the subtest went red naming the leftover block on dinah-core-0.4. Restored, green."
---
The compat suite gains one case: opening each existing fixture workbench under `internal/bench/testdata/compat/` and calling `link` then `unlink` on the same pair against a card in it round-trips to a byte-identical anchor, without requiring the card to be re-migrated.