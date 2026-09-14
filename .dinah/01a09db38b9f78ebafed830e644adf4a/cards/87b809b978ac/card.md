---
title: A glossary term in the plural is not the term, so the guard walks past it
column: 5ea2db0272fc
state: ready
severity: minor
priority: soon
---
The glossary guard matches a declared term exactly, so an English plural of that term is a different string and the guard does not recognise it. A translation that renders the plural of a glossary word however it likes passes, while the singular beside it is held to the declared rendering. The vocabulary that most needs to be consistent is therefore checked in one of its two commonest forms.

Two things stand in the way of the obvious fix, and both are real rather than excuses.

Loosening the match is forbidden in writing, by a comment at `internal/msg/msg_test.go:355`. Whoever weakens it has to answer that comment rather than delete it, and the reason it exists needs re-reading before anybody argues with it.

Closing the gap by declaring the plural forms restales every catalog. There is no per-entry exception in the glossary machinery, so a single new entry invalidates the whole declaration and every translated catalog needs its fingerprint recomputed. That was verified by the reviewer on dinah-311 by reading the mechanism rather than by taking the implementer's word for it.

So the card needs a shape that does not force a tree-wide restale on every future glossary addition, which is a design question and not a one-line repair. That is the reason it is a card rather than a fix folded into the work that found it.

Filed out of dinah-311, where the implementer named it as deliberately cut and the reviewer confirmed the reasoning holds. Neither of the two seats that met it can create a card, so it was written out in a comment there and lifted here. See the card comments on dinah-311 for the implementer's own wording and the reviewer's verification.
