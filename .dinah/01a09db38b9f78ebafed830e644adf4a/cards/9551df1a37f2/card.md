---
title: A column titled 1 shadows card 1, and the tool refuses the slug that would not have
column: 5ea2db0272fc
state: ready
severity: major
priority: soon
---
Name a column `1` and `dinah show 1` silently gives you the column rather than card 1. Reproduced twice, by a spec author and independently by a reviewer.

The sharp part is not the collision, it is which spelling the tool already guards. Dinah refuses to give a column a *slug* of `1`. It then accepts a *title* of `1` without complaint, and the title is the spelling that actually shadows. So the guard exists, sits on the harmless half, and leaves the harmful half open.

There is a second instance of the same shape: a twelve-character hexadecimal title or slug shadows a card's own identifier the same way. Both belong on this one card. Column titles are unconstrained today, so both are reachable now rather than hypothetically.

Found while dinah-471 established which address spellings the tool accepts. That card teaches the shorthand and warns about the precedence in the same paragraph, which its reviewer ruled the right answer there, because changing the precedence would change what existing references mean and people find these forms by accident today. So this card is not "add a warning"; the warning is already going in elsewhere.

What the card should settle, none of it the operator's to rule on:

Whether a title that shadows an address form should be refused at write time, warned about, or left alone with the precedence documented. Refusing is the strongest answer and the most disruptive, because titles are prose a person chose and a refusal on a title is a refusal on somebody's words. Warning is weaker and this workstream has just filed dinah-482 about a warning that goes missing wherever nobody remembered to raise it. Leaving it alone is defensible if the precedence is taught, which dinah-471 is doing. Rule, and say what the existing slug refusal is for, because whatever reason justifies refusing the slug probably justifies something on the title.

The full set of shadowing shapes rather than the two known. A bare number shadows a card's number; twelve hex characters shadow an identifier. Establish the set by reading what the resolver accepts, and note that dinah-471 counted eleven accepting places by reading and nineteen by tracing, so trace rather than read and say how you enumerated.

What happens to workbenches that already carry a shadowing title. They exist or they will, and a rule applied only at write time never finds them. `dinah check` is the natural home for that sweep, which puts this card next to dinah-462.

Whether anything guards the answer. A sweep that reads nothing reports success, and this board has shipped that twice.

Related: dinah-471 teaches the precedence and deliberately does not fix this; dinah-482 is the warning that goes missing outside one path; dinah-462 is the check surface reporting an unreadable directory as clean.
