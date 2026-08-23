# The five verbs

Five verbs change where a card stands, and the coordination contract fixes
what each one does. A second tool reading the same workbench answers the same
way, which is what makes a workbench something you can hand to somebody else.

`claim` takes up a card that is waiting. Work here is taken rather than handed
out, so you claim your own card and nobody assigns you one. A claim may carry
an expiry with `--expires 8h`, and a lapsed claim returns the card to the
queue with the lapse recorded.

`move` carries a card to another state. It changes where the card is and
nothing else, so a holder who moves a card still holds it and a waiting card
that is moved is still waiting. A state may declare a limit on how much work
it holds, and a move into a full state is refused; the operator may carry one
through with `--override`, which is recorded on the move.

`release` gives a card back. Release it as soon as you stop working it, so
that the queue is honest about what is available.

`block` says the card cannot go on, and why. The reason is prose, because the
obstacles that stop real work are various. A block frees the card, which is
what makes the obstacle visible as an obstacle rather than as somebody being
slow.

`unblock` lifts a block, and only the operator may do it. An obstacle raised
is an obstacle handed to whoever answers for the workbench.

`pull` is not a sixth verb. It is a claim and a move run as one act, over a
card you did not have to name: `dinah pull <state>` takes whatever stands at
the head of the queue in the state before the one you named, carries it in,
and claims it for you. The card's history afterwards is the same claimed event
and the same moved event the two commands write separately, so nothing about
the record tells you which way the card was taken. Add `--no-claim` to move
the card and leave it waiting for somebody else.

Every verb reports one of four outcomes, and the tool keeps them apart because
each calls for a different next move. `ok` means it happened. `refused` means
a rule said no, and the refusal carries a name you can read with `cut -d' '
-f1`. `stale` means the card moved between your reading it and your acting, so
read it again. `unreachable` means the question could not be asked at all. The
exit codes are 0, 2, 3 and 4 in that order.
