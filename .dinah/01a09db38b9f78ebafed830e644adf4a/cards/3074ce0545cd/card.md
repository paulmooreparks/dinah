---
title: A gate on the output queue did not stop a card entering it
column: 5ea2db0272fc
state: ready
severity: major
priority: soon
---
Raised by the dinah-317 sweep on 2026-09-08 and recorded there as a finding rather than acted on. Filing it as its own card because it needs an answer and a finding in a comment is addressed to nobody.

dinah-200 sits in Complete carrying ten pending acceptance criteria whose gate column is Complete. The gate names the very move that ended the card's life on the board, and that move happened anyway. The move was made by the operator from the web session with an empty note, carrying neither of the two override prefixes.

So one of two things is true and the sweep could not tell which. Either an operator session bypasses gate evaluation by design, in which case the gate is advisory whenever he is the one moving the card and every reader should know that. Or the entry was never evaluated at all, in which case there is a hole in gate evaluation that has nothing to do with who moved the card, and every gate on this board is weaker than it reads.

This matters more since the gating rule changed on 2026-09-08. The board now leans on gates to hold questions at the station that answers them, so a gate that silently fails to fire is worse than one placed wrongly: a wrong placement is visible on the card, and a gate that does not fire looks exactly like a gate that passed.

The ten items on dinah-200 were deliberately left in place, by the sweep and by its reviewer, precisely because they are the evidence. Re-gating them would erase the state that makes this legible, so whoever works this card should read them before changing anything.

What this card has to settle: which of the two explanations is true, established from the code rather than by reasoning about the transition log, and then whether the behaviour is right. It is a question about Andoneer's gate evaluation rather than about Dinah, and it is filed here because this is where everything about this project gets filed, including the parts that are really about our own working method.
