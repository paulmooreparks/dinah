---
title: A design document says the binary has two heads it does not have
column: 5ea2db0272fc
state: ready
severity: minor
priority: next
links:
  - kind: spawned_from
    to: 38d8c2b09eb7
---
## The statement

`docs/design/surfaces.md`, around lines 20 to 22, states in the present tense that the heads in the binary include an HTTP server (`dinah serve`) and an LSP (`dinah lsp`). Neither command exists. The document's opening caveat about the verb set still settling covers it loosely, which is why it survived a documentation audit as a minor item rather than a false statement, but the sentence reads as description rather than as intention.

## Why this is its own card rather than part of dinah-446

dinah-446 corrected six statements that were flatly false and left this one out deliberately, because fixing it is not a correction. It requires ruling on whether an HTTP head and an LSP head are still planned, and that is a design decision about what Dinah is going to be rather than a typo about what it is.

The implementer of dinah-446 raised it in its handoff for the same reason: the exclusion was sound, and it meant nothing tracked the sentence afterwards. This card is that tracking.

## What has to be decided before anything is written

**Are both still planned?** If they are, the sentence should say so in a tense that a reader cannot mistake for a description of the current binary, and the document should be readable as a roadmap in that paragraph rather than as an inventory. If either is not, the honest edit is to remove it and say why in the same change, because a design document that quietly drops an idea leaves the next reader to wonder whether it was abandoned or forgotten.

**Is the surfaces document the right home for intentions at all?** It describes heads and the verb set, and it opens with a caveat that the verb set is still settling. A document that mixes what exists with what is intended needs a convention for telling them apart, and this card is a good moment to establish one or to rule that the document holds only what exists.

## Scope

One document. No behaviour changes and no new commands. This card does not build an HTTP head or an LSP head, and a decision that either is still wanted produces a separate card rather than work here.

Do not absorb dinah-447, which covers documentation that omits things that exist, or dinah-448, which covers why the guards did not catch a class of stale statement. This one is the mirror of dinah-447: a document naming something that does not exist rather than failing to name something that does.

## One thing worth carrying from dinah-446

That card's standing rule was that a stale figure may not be repaired by transcribing a fresh one, because the figure moves. The equivalent here is that a claim about what the binary has should not be repaired by listing what it has today, since that list moves too. Prefer a form that stays true, which usually means describing the shape rather than enumerating the members, and pointing at the tool for the current set.
