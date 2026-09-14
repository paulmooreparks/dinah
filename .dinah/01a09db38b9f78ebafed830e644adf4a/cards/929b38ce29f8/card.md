---
title: The profile does not say where the rest of the model lives
column: b69abf918c42
state: ready
severity: major
priority: soon
tier: frontier
workstreams:
  - 20b295305c71
---
The coordination profile requires four card fields and mentions links, while the format document carries levels, checklist items, workstreams and comments. An agent reading only the profile concludes the rest does not exist, and one did, answering two questions wrongly on that basis. A pointer from the profile's card section to the format document would have prevented both. This is documentation work rather than a behaviour change.

## Specification

## What this card lands

Two prose additions to `docs/spec/core-profile.md`, plus one guard test. No
normative statement changes, so the profile's version stays `dinah-core 0.4`
and section 12 gains no entry. Section "Version and changelog" below carries
the derivation.

## What the respin changed

Agent Design Review pushed this spec back over three sentences and endorsed
everything else. The section 7 paragraph wrote bare "layers", which section 4
defines as the section 9 mechanism, so the paragraph that exists to stop a
reader believing something false was manufacturing the belief that Dinah
declares layers. It now says "instruction layer" throughout, and its
positional claim is widened to what CORE-INSTR-5 actually constrains. The 5.3
paragraph claimed section 10 leaves no silence, which is false in general and
false about attachments in particular, so that clause is now bounded to the
five concepts the sentence names. The paragraph also names the product once
rather than twice. The citation framing below is rewritten, because the
review disproved the premise the first draft argued from.

## Why the fix is a pointer and not a general statement

The profile already says in general terms that it is a floor. Section 1 has
"Two conforming tools may share no design decision below the model this
document states", CORE-CARD-8 permits fields the profile does not define, and
CORE-CARD-9 requires a tool to preserve them. A reader still leaves section
5.3 believing the model is complete, because 5.3 opens the sentence "The
profile requires four things of a card and no more" and closes without ever
naming a fifth thing anybody actually carries.

Section 10 does not repair that on its own. Its rows for ranked priority
levels, ranked severity levels, structured items recording judgements, named
groupings of cards, and free prose attached by readers are all ruled `out`,
and `out` reads to a reader as "not present". A reader who reaches section 10
learns the concepts were considered and declined, which is a different claim
from "an implementation carries them anyway". Both halves have to be said, in
one place, at the point the reader forms the belief.

## Piece one: the paragraph in section 5.3

Insert as a new paragraph between the existing paragraph beginning "The
profile requires four things of a card and no more" and the line
`[CORE-CARD-1]`. Leave the existing paragraph unchanged; it is correct and the
prose standard's rule against changing meaning applies to it.

The text, verbatim, wrapped as shown to match the document's existing width:

```
This paragraph is non-normative. The four required fields are a floor, and a
tool's own card model is ordinarily much larger. Ranked priority levels,
ranked severity levels, structured items recording judgements, membership of
a named grouping of cards, and prose a card's readers attach are all ordinary
card fields, riding under CORE-CARD-8 and preserved across tools by
CORE-CARD-9. Section 10 rules all five of those concepts out and records for
each the condition that would bring it back, so a reader meeting one of them
can see that this profile declined it deliberately. Each tool records the
model it built instead in its own documents, and Dinah keeps its own in
`docs/design/format.md` of its source tree. That document is not required
reading, and nothing here rests on it.
```

Four constraints the paragraph is written to satisfy, each checkable:

- **Section 1's prohibition holds, and the citation is no architectural
  first.** Section 1 reads "No other document, product, source tree or vendor
  is required reading, and the profile deliberately cites none of them in a
  normative statement." The two clauses are scoped differently. The first bars
  anything else from being required reading, and the second bars a citation
  only inside a normative statement. This paragraph carries no RFC 2119
  keyword, so it never reaches the second clause, and its last sentence
  discharges the first in the same breath as the citation. The document also
  already cites documents outside itself in prose and rests on them
  substantively: RFC 2119 and RFC 8174 in section 3.1, RFC 8259 in sections
  3.5 and 5.7, and Unicode in 3.5, with section 3.5 speaking of "the
  specifications this document cites". A prose citation is therefore ordinary
  here. What is new is naming a product and a repository-relative path, which
  is a narrower thing than triage and the first draft of this spec both took
  it for, and the last sentence is what makes it safe. A later pointer of this
  kind inherits the whole condition: no keyword, a non-normative marker, and
  the disclaimer in the same paragraph.
- **The excluded-word lists hold.** Section 3.5 binds the two lists to three
  places: every normative statement, section 4, and the walkthrough of 10.1.
  Body prose in 5.3 is none of them, and 3.5 says in terms that "Prose
  elsewhere may use these words where the subject is the thing the word
  names". The paragraph still avoids `workstream` and every other entry on the
  product-vocabulary list, and names each concept by section 10's own neutral
  description instead, because a reader of this profile has no way to know
  what those product words mean. That is section 10's stated discipline for
  naming an excluded concept, applied one section earlier.
- **The keyword rule holds.** `CORE-CARD-8`, `CORE-CARD-9` and `Section 10`
  are identifiers and a cross-reference, not keywords, so `doc.StrayKeywords`
  stays empty under `internal/profile.Extract`.
- **The register already exists in this document.** Section 5.8 closes its
  prose with "What a workbench does about a link is for the people reading it,
  or for a layer that declares itself and says so under its own name", which
  is the same move without the concrete pointer. Section 2 marks a paragraph
  non-normative inline with the same sentence shape. Section 1.1 marks a whole
  subsection non-normative in its heading. Triage's reading of the register was
  right and 5.3 is the right home.

Two things about the wording an implementer must not smooth away. The clause
about section 10 is bounded to "all five of those concepts" on purpose,
because the general claim is false: the profile never mentions attachments
anywhere, section 10 included, while Dinah's card model carries them and
`docs/design/format.md` discusses them at length. OQ-3 puts that gap to the
operator. The product is also named exactly once, in the clause that gives the
path, and the surrounding sentences say "a tool" rather than walking around
the name a second time.

## Piece two: the paragraph in section 7

The same silence sits in section 7 and nobody has tripped over it yet. Section
7 names two instruction layers, the workbench's standing instructions and the
column's, and its prose reads as the whole chain. `docs/design/format.md`
composes three, adding a user-global instruction layer at
`~/.dinah/instructions.md` ahead of both, and it records the intention to
serve a shared layer ahead of the whole chain as well. The product serves
three rather than merely documenting three: `verb.Instructions` carries
`Global`, `Standing` and `Column` (`internal/verb/library.go:194`), and
`bench.GlobalInstructions` reads the user-global file
(`internal/bench/config.go:345`). Nothing in the profile forbids any of that,
and nothing in the profile says so.

Insert as a new paragraph after the paragraph beginning "The workbench carries
standing instructions that apply wherever a card is" and before the paragraph
beginning "Instructions are served at the two moments":

```
This profile fixes the order of those two instruction layers and says nothing
about how many a tool serves. A tool may compose further instruction layers
of its own, in any position that leaves the workbench's standing text ahead
of the column's. CORE-INSTR-5 constrains the order of the two named here and
nothing else. CORE-INSTR-6 forbids copying the text of one instruction layer
into another, and that prohibition reaches a tool's own instruction layers
too.
```

The noun is qualified as "instruction layer" everywhere it appears, and an
implementer must not shorten it. Section 4 defines **Layer** as "A body of
declarations and behaviour outside this profile, named so it cannot collide
with it", which is the section 9 mechanism, a declaration in the workbench
definition under a dotted name. A reader who has read section 4 takes a bare
"layer" here for that mechanism and concludes that Dinah declares layers,
which it does not, since its user-global instruction file is a file on disk.
That is the confusion OQ-2 raises, and a paragraph written to stop a reader
believing something false must not plant a second false belief inside itself.
CORE-INSTR-6 already uses the compound, so the qualified form is the
document's own word rather than a new one.

The positional claim is widened for the same reason. CORE-INSTR-5 requires
only that a workbench's standing instructions precede the column's, so a
further instruction layer served between them or after them breaks it no more
than one served ahead of both. The first draft licensed only "ahead of these
two", which a reader could take for the sole permitted position and which is
narrower than the statements are. The wording above names the constraint that
actually binds and leaves every position it allows open.

This paragraph carries no non-normative marker. It states what the published
statements already do and do not require, which is what the surrounding prose
in this document does throughout without marking itself. The 5.3 paragraph is
marked because it cites a document outside the profile, and that is the thing
worth flagging to a reader.

## Piece three: the guard that stops the pointer rotting

A pointer naming a path is the same defect species one move later if the path
moves and the profile does not. The repository already carries this pattern in
`cmd/dinah/guide_guard_test.go:607`
(`TestTheDocumentationNamesOnlyPublishedFilesThatExist`), but that guard scans
the guides, the quick start and the message catalogs, and it matches published
URLs rather than a bare repository-relative path, so it does not reach the
profile.

Add a guard to `internal/profile`, which is where the profile's own guards
live and where `profilePath` and `readProfile` already are
(`internal/profile/extract_test.go:12` and `:33`). It asserts both halves:

- Section 5.3 carries the path `docs/design/format.md`. Without this half the
  guard passes vacuously the moment somebody deletes the sentence, which is
  the failure mode `TestTheDocumentationNamesOnlyPublishedFilesThatExist`
  guards against with its own `named == 0` check.
- The working tree carries a file at that path.

Extract the path out of the section text and stat what you extracted. A guard
that asserts one hard-coded constant twice, once against the section and once
against the filesystem, does not test what AC-5 states, which is that the file
exists at the path section 5.3 names. Match the path with a pattern over the
section rather than searching the section for a literal, so a sentence
rewritten to name some other path still gets stat'd and still fails honestly.

Use `section(text, "### 5.3 Cards")`, the helper at
`internal/profile/extract_test.go:392`, so the guard fails if the sentence
migrates out of 5.3 rather than merely out of the file. Suggested name:
`TestTheCardSectionPointsAtADocumentTheRepositoryCarries`. The failure message
says which half failed and why the pointer exists, so whoever hits it moves
the sentence rather than deleting it.

## Version and changelog

The profile's version stays `dinah-core 0.4` and section 12 gains no fifth
entry. The passages that decide it, in order:

- Section 2.2 states the unit of change: "The unit of change is the extracted
  statement list, which section 3 defines and section 11 indexes." Prose that
  carries no RFC 2119 keyword on a statement line is not in that list, so this
  amendment changes it by nothing.
- `[DOC-VER-1]` "A patch increment MUST NOT change the extracted statement
  list", and section 2.2's "A patch increment carries editorial change alone."
  This amendment is exactly editorial change.
- The published version carries a major and a minor and no third number. The
  identity line reads `dinah-core 0.4`, `[CORE-VER-1]` asks a claim to name
  "the major number and the minor number", and every changelog heading matches
  `^### [0-9]+\.[0-9]+, channel ...`. A patch increment therefore has no number
  to move, and the version string is unchanged by construction.
- `[DOC-CHG-2]` requires an entry to carry "every identifier it affects marked
  as introduced, relaxed or retired". This amendment affects no identifier, so
  an entry written for it would carry an empty list and would say nothing a
  caller could act on.

This is settled precedent on this board rather than a fresh reading. dinah-201
ruled the same question the same way for its boundary-row amendment, and
`internal/profile/amendment_test.go` enforces the ruling with three constants:
`publishedStatements = 127`, `declaredVersion` holding section 2's version
sentence in full, and `publishedChangelogEntries = 4`. All three stay
untouched by this card, and `TestAnEditorialAmendmentMovesNeitherVersionNorChangelog`
passing unchanged is the machine-checkable proof that this card moved neither.

## Constraints and out of scope

- No normative statement is added, retired, reworded or reordered. A statement
  would take a version increment, a changelog entry, and a boundary-table row
  under section 10's rule that "each statement of this profile appears against
  exactly one row", which is a different card.
- The boundary table is not edited. It gains no row and no ruling changes,
  because no concept enters or leaves the core. The attachments gap OQ-3 names
  is a boundary-table amendment and belongs to whoever answers that question.
- `docs/design/format.md` is not edited. This card points at it and does not
  touch it.
- No behaviour changes and no non-test Go code changes. The only Go file this
  card touches is the new guard's test file.
- Section 5.2 is deliberately left alone. See OQ-1: it carries a real gap, and
  closing it needs a statement rather than prose, so it is a separate card.

## Files

| File | Change |
| --- | --- |
| `docs/spec/core-profile.md` | One paragraph added in 5.3, one paragraph added in section 7. Nothing else. |
| `internal/profile/` (test file, new or existing) | The two-halved pointer guard. |

## Branch

dinah-198-the-profile-does-not-say-where-the-rest-of-the-model-lives
