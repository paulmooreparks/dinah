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

What separates the two senses is the company the word keeps. Group the
rename's own replacements by the words on either side of each one, and a
few thousand replacements become a couple of thousand phrases, most of
which appear once or twice and are read at a glance.

The company has to be counted on both sides, and the reason is worth
setting down because an earlier version of this pass counted only one.
Reading backwards alone says that a board column is a countable noun
behind a determiner, so a group keyed on "pending" or on
"coordination-plane" gives a defect away at once. What that misses is the
commonest shape English has, which is an ordinary noun behind an ordinary
determiner. "The state it left behind" and "the column of the board"
share their determiner and share no verdict, so a key made of the
determiner alone drops the first into a group of hundreds that a reader
passes over. Seven over-eager renames of exactly that shape were planted
in this repository's own corpus and swept, and not one of them reached
the report. The word after the replacement is what tells them apart, so
the key is three words wide: what stands before, the replaced word
itself, and what stands after.

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
pending column carries   1 site   docs/design/format.md:702  leaving the pending column carries at least one
```

The phrase comes first so that the eye runs down the left edge of the
report and reads phrases. What follows it is the number of sites in the
group and one of them, with an excerpt of its line.

Groups come rarest first. A legitimate replacement sits in the same
phrase hundreds of times, while a wrong sense sits in a phrase that
appears once or twice, so the top of the report is where the attention is
worth spending. Dinah's own rename, swept over `e9c0abd^..e9c0abd`,
produced 2257 groups out of 6397 replacements, and 1455 of those groups
hold a single site. That range is pinned history, so anybody can
reproduce the three counts.

Read every phrase and ask one question of it: does this mean the thing
the rename was about?

Three rules govern the answer, and the third of them replaces a rule this
document used to carry.

- A label that can only be the board phrase clears its group. "a column
  named", "the column of" and "each column in the flow" are of that kind.
- A label that could be either is opened rather than judged, with the
  `--all` run below. "the column it" is the example that matters here,
  because "the column it removed" is a board phrase and "the column it is
  in" is a defect, and both carry that label. "live column" is the same
  case one word shorter.
- A large group is never passed over because it is large. An earlier
  version of this document told the reader that "the column" and "each
  column" answer themselves, and that instruction is what hid the seven
  planted renames described above: they landed in groups of 19, 27, 750
  and 815 sites, every one of which the reader had been told to skip. A
  sweep whose advice is to skip its largest groups has a hole the size of
  those groups.

Settle an ambiguous group by reading its sites rather than its example:

```
go run ./internal/rename/renamesweep --old state --new column --range main..HEAD --all
```

Two more things the report can say deserve attention. A group whose
phrase is closed up and marked "(identifier)", as `crashColumns are` is,
holds replacements that are parts of a longer name rather than words of
their own, which is why `CrashStates` is judged against "Crash" and not
against the comment marker that opens its line. A line beginning
"unaligned run" names a run of changed text the sweep declined to read,
and each of those is opened by hand, because a run the instrument skipped
is exactly where a defect would survive it. That line names the retired
term beside the file, since a run is declined for its size alone and the
file it names is often unrelated to the rename being swept. Without the
term the line reads as news about somebody else's file, and the reader
has no way to tell that their own search went unanswered inside it.

## What the pass does not do

The sweep reports and does not rule. Every verdict is a person's, and the
instrument's whole claim is that it turns a few thousand judgements into
a couple of thousand phrases and puts the suspicious ones first. On
Dinah's own rename that is the 2257 groups counted above, most of them
one site long and read at a glance. An earlier version of this paragraph
said a few hundred, which described the narrower key this pass used
before the following word went into it, and it understated the reading
cost by about five times.

Six limits are worth stating plainly, because each one is a place a
reader could assume more than the tool delivers.

- The sweep matches one word in the singular and in the plural formed by
  a trailing s, which is an English inflection and only that. A rename
  between words with an irregular plural is run once for each form, and
  so is a rename in a language that inflects some other way. The Hindi
  rename this pass was repaired on is the worked example: स्तंभ to कॉलम
  reports 98 replacements, and the ninety-ninth is the oblique plural
  स्तंभों to कॉलमों, which is a second run of one command.
- The sweep reads a rename of one word to one word. A rename that
  replaces a word with a phrase, or a phrase with a word, is outside what
  it aligns, and it refuses such a term rather than sweeping it. See "A
  term the sweep will not accept" below.
- The sweep reads the diff of a range and knows nothing about the rest of
  the tree. A word the rename took in a file the range never touched is
  not its business, and no instrument here looks for one.
- The key is three words wide, so a defect whose three words match an
  idiom already in the tree shares a group with that idiom. Two of the
  seven planted renames that produced the current key are of this kind:
  "a column of" and "the column it" are both real board phrases, and the
  planted defects wearing those labels are reached by opening those
  groups rather than by reading them. Widening the key further trades
  this away for a report nobody finishes, so the trade stops here and the
  `--all` run is the answer.
- The sweep has no word segmenter, so it cannot count a rename in a
  script that writes no spaces between its words. Chinese is the case
  this was found on: the tokenizer swallows a whole clause as one token,
  nothing inside it equals the term, and the alignment finds nothing to
  align. What the sweep does instead of counting is refuse, under the
  backstop described in "A zero the diff contradicts" below, so a run
  that could not read the rename ordinarily says so rather than answering
  zero. The guarantee lapses in one case, and it is worth being able to
  recognise from the report alone. The backstop fires only where the
  sweep declined nothing else, so a range that also carries an unrelated
  run too large to align lets the zero through: the report then holds a
  zero, no groups at all, and an "unaligned run" line naming a file with
  nothing to do with the rename. This repository's JSON catalogs are the
  file shape that trips the cap, so a rename swept over a range that also
  touches a large catalog edit lands there. The line carries the retired
  term, which is the signal that the term went uncounted rather than
  unfound, and the way out is to narrow the range until nothing is
  declined, or to read the diff by hand. A card renaming a word in such a
  script reads its diff by hand in any case.
- The sweep reports and does not know which sense a group means. Nothing
  in it understands English or Hindi, and the counts and the ordering are
  the whole of its contribution.

## A term the sweep will not accept

The sweep reads a word by tokenizing it, and it matches by testing one
token against the whole term. A term that comes apart into more than one
token therefore matches nothing, whatever the diff holds. Rather than
running and reporting the zero that guarantees, the sweep refuses the
term and exits non-zero:

```
$ go run ./internal/rename/renamesweep --old "board state" --new column --range main..HEAD
renamesweep: --old "board state" is not one word to this sweep, which reads
it as board + state. A term the tokenizer splits matches nothing, so the run
would report zero replacements and a reader would take that for a clean tree
```

The refusal exists because of a defect this pass had on the day it
shipped, and the shape of that defect is worth keeping in view. The
tokenizer read a word as a run of letters, and the marks Devanagari
writes its vowels with are not letters, so every Hindi word came apart
into single consonants and no token could equal the word a caller was
looking for. Running the rule at the bottom of this document against this
repository's own Hindi rename, over the range carrying all ninety-nine
substitutions, produced `0 replacements of "स्तंभ" by "कॉलम", in 0 groups`.
Nothing distinguished that from a clean tree. A German control over the
same range answered 92 replacements in 30 groups, so neither the range
nor the invocation was at fault.

Both halves of that were fixed, and they are separate fixes doing
separate jobs. A combining mark now stays inside its word, so Devanagari,
Arabic, Hebrew and a decomposed Latin accent all tokenize as a reader
reads them. The refusal is the half that outlives the tokenizer, since no
list of scripts can be complete and a term this tokenizer splits is
refused whatever alphabet it is written in. The defect this whole
document was written against is an instrument that answers zero for input
it cannot read, and a tokenizer meeting a script it was never built for
is that same defect one layer up.

## A zero the diff contradicts

The refusal above covers a term the tokenizer splits. A tokenizer can
also merge, and the two failures are mirrors of each other. A script that
writes no spaces between its words hands the tokenizer a whole clause,
the clause comes back as one token, and no token inside it equals the
term the caller asked for. Nothing splits, so the term is accepted, and
the run reports zero replacements in zero groups.

Chinese found this. Two occurrences of 工作台 renamed to 板块, swept over
the range that carried them, answered `0 replacements of "工作台" by
"板块", in 0 groups` and exited zero.

The sweep now checks its own zero before it prints one. When the
alignment produced no replacement and declined no run, the raw diff is
read for the retired term on a removed line and the adopted term on an
added line. Finding both, the sweep refuses:

```
$ go run ./internal/rename/renamesweep --old 工作台 --new 板块 --range 9da3fcb..940dfba
renamesweep: the sweep aligned nothing at all, and the diff contradicts that:
it removes a line carrying "工作台" and adds a line carrying "板块". A tokenizer
that merges the words of a script written without spaces reads such a rename as
one long token and matches none of it, so this run cannot tell a rename it
failed to read from a range that carries none. Read the diff by hand, or narrow
the range until the sweep aligns what it changed
```

Two properties of that test are what make it worth having. It compares
substrings and case-folds them, asking nothing about word boundaries, so
it holds in a script this package cannot segment. And it shares no
machinery with the sweep it is checking, which is the point of a
backstop.

A range that carries no rename at all can still trip it, when a removed
line happens to mention the retired word and an added line the adopted
one. The cost of that is one reader opening one diff. The cost of the
zero it replaces is a rename card losing its only instrument without
anybody finding out, which is why the trade runs in this direction.

The condition in that first sentence is load-bearing and cuts a real hole
in the check. A run the sweep declined is a run it announced, and the
backstop stands down on the reasoning that the reader has already been
told something went unread. That reasoning holds only while the declined
run is the one carrying the rename. Nothing ties the two together: a run
is declined for its size, so an oversized edit to an unrelated file
suppresses the backstop for a rename elsewhere in the same range, and the
report answers zero with one "unaligned run" line pointing at the
stranger. That is why the line names the retired term. A reader who
meets a zero, no groups, and an unaligned run in a file they were not
renaming is looking at this case rather than at a clean range, and the
answer is to narrow the range until nothing is declined.

What this does not do is count anything. Nobody can read a report of a
Chinese rename out of this pass, and adding a word segmenter to get one
would be a larger tool than the pass is worth. The limit stands, and the
run that meets it says so.

## The rule for the next rename

A card that renames a word across the tree runs this pass before it hands
off, and says in its handoff how many replacements and how many groups
the report held and which groups it opened. A rename card that does not
say so has not run it, since the counts are the only part of the pass
that cannot be produced from memory.

Report both counts rather than the group count alone. A run that refuses,
whether it refused the term before the sweep or refused its own zero
afterwards, prints no counts at all, and a run whose term the sweep accepts
but whose range is wrong reports zero of both, so a handoff carrying one
number leaves a reader unable to tell a swept range from an unswept one.
A rename in more than one language runs the pass once per language, and a
rename in a language that inflects the word runs it once per form, so the
handoff carries a line for each run rather than a single total.
