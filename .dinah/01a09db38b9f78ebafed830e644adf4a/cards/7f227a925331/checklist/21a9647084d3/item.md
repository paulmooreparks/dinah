---
kind: decision
state: resolved
ts: 2026-09-14T02:18:44Z
ordinal: 28
note: Answers the observation left on this card about dinah-439's reviewer finding that the missing-ordinal check reads only the live half. A card's number is workbench-scoped, so an archived card holding a number is exactly where a collision hides and the registry checks must walk both halves; internal/bench/check.go:266 shows Bench.Check's card walk listing b.CardsRoot() alone, which is what scopes checkOrdinals (:727) to the live half. A comment's or attachment's ordinal is scoped to the one card holding it, so an archived card's members can collide with nothing outside that card, and the card is unreachable by reference until restored, at which point the live-half walk covers it. The two checks therefore answer a workbench-scoped question and a card-scoped one rather than disagreeing about the same question. A later card wanting the archived half swept for below-card ordinals should name the damage it is defending against, because this card could not name one.
---
The card-number checks read both halves of the collection and `checkOrdinals` goes on reading the live half alone. The asymmetry is deliberate and this card does not close it.