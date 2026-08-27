# Renaming a word across the tree

Renaming one word everywhere it appears is the cheapest change a codebase
ever accepts and one of the easier ones to get quietly wrong. The rename
that retired `state` as the name of a board position and adopted `column`
touched 176 files and made 6397 replacements. Twenty of those
replacements landed in sentences that meant the word's other sense,
which is the condition a thing is in. Not one of the twenty broke a test,
because prose asserts nothing, and the first eleven were found by a
person reading the diff by eye.

This document is the procedure that replaces the reading with an
instrument, together with the reasoning behind it, so that the next
rename on this repository costs one command and one careful scan rather
than a fortnight of noticing.

## Why the searches you would reach for first do not work

A rename leaves two kinds of defect behind, and they need different
instruments.

The first kind is the replacement that did not happen, where the retired
word survives somewhere the pass did not reach. A tree-wide search for
the retired word finds those, and a list of named exceptions keeps the
legitimate survivors quiet. That instrument works and it is not what this
document is about.

The second kind is the replacement that should not have happened. It
leaves no retired word anywhere, so the search above has nothing to
report and the exception list has no row to hold. Searching for the
adopted word instead finds every legitimate use as well, which on this
tree meant 7126 hits, nearly all of them older than the rename and none
of them distinguishable from the defects by any pattern a regular
expression can express.

What separates the two senses is the company the word keeps. A board
column is a countable noun and stands behind a determiner, so the great
majority of legitimate replacements follow "the", "a", "each" or a
possessive. The other sense is a mass noun or a verb and stands behind
other words entirely, which is how "the pending state" and
"coordination-plane state" and "those loops state local intent" give
themselves away.
Group the rename's own replacements by the word standing in front of each
one, and a few thousand replacements become a few hundred phrases a
person judges in a couple of minutes.

## Running the pass

The instrument is `internal/rename`, and `internal/rename/renamesweep`
runs it against a git range. It lives under `internal` because it
maintains this repository rather than being part of Dinah, and no release
ships it.

Run it before the rename is handed off, against the range that carries
the work:

```
go run ./internal/rename/renamesweep --old state --new column --range main..HEAD
```

A rename that has already landed is swept the same way, naming the commit
that carried it:

```
go run ./internal/rename/renamesweep --old state --new column --range e9c0abd^..e9c0abd
```

That second form works on a squashed branch as well as on an ordinary
commit, because a squash commit's own diff is the whole branch. The
evidence a rename leaves does not expire when the branch is squashed onto
the trunk, and an earlier account of this pass that said it did was
mistaken. Running the pass before shipping is still the better order,
since a defect caught before the handoff costs one edit and a defect
caught afterwards costs a card.

## Reading the report

The report opens with its own size, so the reading it is asking for is
known before it begins. Every line after that is one group, and the group
opens with the phrase it stands for:

```
pending column   4 sites   docs/design/format.md:702  leaving the pending state carries at least one
```

The phrase comes first so that the eye runs down the left edge of the
report and reads phrases. What follows it is the number of sites in the
group and one of them, with an excerpt of its line.

Groups come rarest first. A legitimate replacement follows the same
handful of determiners hundreds of times, while a wrong sense follows a
word that appears once or twice, so the top of the report is where the
attention is worth spending.

Read every phrase and ask one question of it: does this mean the thing
the rename was about? A group reading "the column", "each column" or
"another column" answers itself. A group reading "pending column",
"recordless column" or "loops column" does not, and those are the ones to
open.

A group whose phrase is genuinely ambiguous in English cannot be settled
from its label, and "live column" is the example that matters, since it
is a real board phrase and was also a real defect in the same rename.
Settle one of those by reading its sites rather than its example:

```
go run ./internal/rename/renamesweep --old state --new column --range main..HEAD --all
```

Two more things the report can say deserve attention. A group whose
phrase is closed up and marked "(identifier)", as `crashColumns` is,
holds replacements that are parts of a longer name rather than words of
their own, which is why `CrashStates` is judged against "Crash" and not
against the comment marker that opens its line. A line beginning
"unaligned run" names a run of changed text the sweep declined to read,
and each of those is opened by hand, because a run the instrument skipped
is exactly where a defect would survive it.

## What the pass does not do

The sweep reports and does not rule. Every verdict is a person's, and the
instrument's whole claim is that it reduces a few thousand judgements to
a few hundred and puts the suspicious ones first.

Three limits are worth stating plainly, because each one is a place a
reader could assume more than the tool delivers.

- The sweep matches one word in the singular and in the plural formed by
  a trailing s. A rename between words with an irregular plural is run
  once for each form.
- The sweep reads a rename of one word to one word. A rename that
  replaces a word with a phrase, or a phrase with a word, is outside what
  it aligns.
- The sweep reads the diff of a range and knows nothing about the rest of
  the tree. A word the rename took in a file the range never touched is
  not its business, and no instrument here looks for one.

## The rule for the next rename

A card that renames a word across the tree runs this pass before it hands
off, and says in its handoff how many groups the report held and which
ones it opened. A rename card that does not say so has not run it, since
the count is the only part of the pass that cannot be produced from
memory.
