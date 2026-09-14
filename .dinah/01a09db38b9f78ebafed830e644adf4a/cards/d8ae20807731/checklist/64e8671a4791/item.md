---
kind: decision
state: resolved
ts: 2026-09-14T02:18:41Z
ordinal: 22
note: "Round 2 made both attempts unwrap in each of the two callers, which is five changed arms, and then armed them with three plants against fixtures holding exactly one archived card on the colliding number. Those fixtures cannot produce an archived-half ambiguity at all, because a live card on the number answers the first attempt and the archive is never read, so restoring either archived unwrap left every test green and two of the five arms were guarded by nothing. The fixtures now hold a second collision on a different number, which the live half does not carry at all and the archive carries twice; fx-3 in the spec's recipe. AC-11 drives both archived unwraps against it and AC-8 plants five times. The general form of the mistake is worth naming, since it is the same failure as the original defect one level up: a criterion that arms a guard has to name a plant that reddens the check it arms, and counting plants against intent rather than against the diff is how an arm ends up covered by a criterion that cannot reach it."
---
Each fixture carries two collisions, one per half, and the plants are counted against the arms the fix changes rather than against the arms it set out to change.