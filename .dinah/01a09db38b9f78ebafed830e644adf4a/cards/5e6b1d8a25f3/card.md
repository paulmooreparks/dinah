---
title: A sentence naming a file breaks apart before the guides are compared
column: 5ea2db0272fc
state: ready
severity: minor
priority: next
---
`TestNoSentenceStandsInTwoGuides` in `cmd/dinah/guide_guard_test.go` holds the boundary between two guides by failing when a sentence of eight or more words stands in both. It splits prose into sentences on a full stop, a question mark or an exclamation mark, which is the rule its own doc comment states.

A file name carries a full stop. So "the reading order lives in guide.go and the guides sit beside it" splits into three fragments, none of which is eight words, and the whole sentence drops out of the comparison. The same happens to any sentence naming `state.md`, `workbench.md`, `journal.ndjson`, or a version like `dinah-core/1.0`. The guides that ship today name files often, and the guide this check was written for names two of them.

Nothing fails, and that is the shape of the defect: the check goes on passing over a corpus in which one guide has copied a sentence from another, so long as the sentence happens to mention a file. A guard that quietly narrows its own corpus is worse than a narrower guard that says so.

## What a fix has to settle

The naive repair, splitting only on a full stop followed by a space, breaks on a sentence ending a paragraph and on `docs/spec/core-profile.md, which` in the middle of one. The rule wants to be stated rather than guessed at, and stated in the doc comment where the current rule already is, because that comment is what the check means.

Two candidates worth weighing rather than one being obviously right. Recognise a full stop as terminal only when it is followed by whitespace or the end of the text and the next non-space character is not lowercase. Or mask the spans this codebase already knows are not prose, which are backticked runs, before splitting at all; the guides write every file name and every command between backticks, so masking those spans first also removes the whole class rather than one instance of it.

The second is likelier to be right here, because the same masking would let the check see a sentence whose only difference between two guides is a backticked flag, which the current rule cannot.

## Acceptance

- A sentence naming a file survives the split whole and is compared, proved by copying such a sentence between two guides and watching the check fail.
- The five guides that shipped before this card, and the two that landed with dinah-164 and dinah-212, still pass.
- The doc comment states the new rule, since the rule is the check's meaning.

Found at Agent Code Review on dinah-182 and left off that card because the check it weakens is the one that card added, so repairing it there would have meant reviewing the repair in the same breath as the thing it repairs.
