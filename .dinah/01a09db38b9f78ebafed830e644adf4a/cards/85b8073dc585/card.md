---
title: A tier change reads as broken rendering when it sits next to one that names its station
column: 5ea2db0272fc
state: ready
severity: minor
priority: next
---
Two tier events print side by side in a card's history and one of them looks like a rendering fault. A raise names the station and carries its reason, reading something like "Build: no requirement to apex (found deeper coupling than the ticket suggested)". An ordinary per-column assignment prints the stored identifier and nothing else, reading something like "f32b7374a3c7: apex to frontier".

Both are correct. The second event never captured a title, because only a raise does, so falling back to the identifier is the honest thing for the renderer to do rather than inventing a name or resolving one that may since have changed. Read on its own it is unremarkable. Read directly beneath a line that does name a station, a person reasonably concludes the hex string is broken output rather than a different kind of event with less recorded about it.

Found at Test on dinah-409 by producing both lines against a real workbench rather than by reading templates, and correctly reported as a rough edge in the surface rather than a defect in that card.

What this card has to decide rather than assume. Whether the fix is to capture a title on the ordinary assignment as well, which makes both lines alike at the cost of storing a name that can go stale, or to make the fallback line say plainly that no station name was recorded, which keeps the honesty and removes the ambiguity. The first is what the raise already does and dinah-408 chose write-time capture deliberately, so there is precedent; the second costs nothing stored and admits what happened.

Do not answer it by resolving the identifier at read time. That was considered and rejected during dinah-409's design on the ground that a title resolved later describes the board as it is now rather than as it was when the event happened, and the move event's own precedent is to store both titles at write time for exactly that reason.

Worth reading dinah-409's third and fourth review comments first, since the distinction between a title captured at write time and one resolved at read time was argued out there and should not be relitigated here.
