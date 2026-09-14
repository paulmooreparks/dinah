---
title: Every guide is scanned for a denial, and the five archive statements are held to the tests that prove them
column: b69abf918c42
state: ready
severity: major
priority: soon
workstreams:
  - 994787601ae6
---
The references guide shipped a sentence saying Dinah has no restore, thirty lines above a table that listed restore, inside the binary. Nothing read that sentence, so a green whole-tree suite and eleven passing acceptance criteria went straight past a guide contradicting itself. dinah-461 fixed the sentence and added a check that fails on any sentence denying a command the build carries.

That check reads one guide of the eight the tool ships, and it catches only one shape of falsehood. Four other statements in that same guide describe how archiving and restoring behave; all four shipped, and nothing anywhere in the tree reads any of them. Established twice, independently: the implementer grepped every one of their opening phrases across the whole tree and got zero hits, and the reviewer confirmed it. Each can go stale exactly the way the restore sentence did, with every test still passing.

The cheap route, ruled by code review, and it is two steps rather than a mechanism:

Point the new denial check at the whole-corpus scanner that already reads all eight guides and already handles fenced code blocks. That is a small change and it multiplies the coverage by eight. It also resolves a defect this card inherits: dinah-461 wrote a second prose reader for the guides when the same package already had one, and the two disagree, the new one ignoring tables and the old one ignoring fenced blocks with neither covering both. Nothing breaks today only because that one guide has no fenced blocks.

For the four statements, add one line to each of the tests that already prove the behaviour, asserting the guide still carries the sentence. No new machinery, and it closes the four that are known.

A general ledger of prose claims is possible and should not be built until those two have run and shown what they miss.

Two corrections to carry, both from code review:

The justification attached to the table-stripping in the new check is wrong. It says a folded table row runs a "no" cell into the next command's name; that does not happen, because the pipe characters survive the fold, and running the scan with and without the strip gives identical results. The strip is worth keeping and its stated reason is not.

The check fires on true prose whenever a command name is used as an ordinary noun. "A card has no comment" and "a workbench has no path" both trip it, demonstrated. It is loud and easy to diagnose rather than dangerous, but somebody will meet it, and widening it to eight guides widens that too.

Related: dinah-457 made the guide's command table derived from the code and holds it in both directions; dinah-460 and dinah-461 each shipped a considered refusal to build a guard that would fire falsely, both on measurement rather than principle, and this card should reach the same standard before adding a check that cries wolf across eight documents.

## Specification

Worked against `866221f` (`origin/main` at the time of writing, the merge of
dinah-470), in the worktree `C:/dinah-scratch/dinah-478-spec/wt`. Every count and
every firing quoted below was produced by running a scan over that tree rather
than by reading, and the commands are named beside the numbers.

## What this card changes

Dinah embeds eight guides in the binary. One check reads a guide for a sentence
denying a command the build carries, and it reads `references` alone. This card
points that check at all eight, retires the second prose reader it was built on,
and ties the five statements in the references guide's "Reading the archive"
section to the tests that already prove the behaviour each one describes.

The card's description proposed two steps and no new mechanism. Both premises
hold and both steps are specified here. The premises were checked rather than
assumed, and the checks are recorded under each step.

## Step one: every guide is scanned for a denial

### The scanner already exists and already reads all eight

`guideProse` in `cmd/dinah/guide_guard_test.go:1025` reduces a guide to its
sentences. It skips fenced blocks, skips headings, skips table rows, strips list
markers, folds whitespace, lowercases, and splits at a full stop, a question
mark or an exclamation mark. Its one caller today is
`TestNoSentenceStandsInTwoGuides`, which walks `guide.Topics()`, and
`guide.Topics()` answers eight topics: `first-session`, `getting-started`,
`verbs`, `principles`, `references`, `query`, `workbench-layout`, `mcp`.

### The corpus is the eight embedded guides, and the quick start is left out

Running the denial pattern over `guideProse` output for all eight guides gives
526 sentences, 12 firings with backticks left in place and 14 with backticks
stripped, and none of those firings captures a name in the 56-command roster
`verb.Commands()` returns. The widened check is silent against the tree as it
stands, so it does not cry wolf.

Per-guide sentence counts, which matter because a sweep that reads nothing
reports success: `first-session` 49, `getting-started` 34, `verbs` 25,
`principles` 76, `references` 77, `query` 88, `workbench-layout` 18, `mcp` 159.

`docs/quick-start.md` is deliberately not in the corpus. Scanning it the same way
gives 401 sentences and one firing that does name a command:

    ...but a card standing there is ready in the ordinary way, carries no
    block, and anybody may move it on when the answer comes

`block` is a command this build carries, and the sentence is true. Admitting the
quick start would ship a guard that fires falsely on its first run, which is the
outcome dinah-460 and dinah-461 each refused on measurement.

One alternative was considered and rejected. The quick start's sentence could be
reworded so the corpus could be nine documents. That makes the guard dictate how
a true sentence may be phrased, and it moves the pressure from an exemption list
onto the prose. The exclusion is stated in the new test's own doc comment, with
the sentence quoted, so the next reader can weigh it rather than rediscover it.

### What lands

`cmd/dinah/guide_guard_test.go` gains:

- `guideDenialOfACapability`, moved unchanged from
  `cmd/dinah/references_guide_test.go:737`. The pattern itself is not edited by
  this card.
- `TestNoGuideDeniesACommandTheToolHas`, which walks `guide.Topics()`, feeds each
  guide's text through `guideProse`, strips backticks from each sentence, applies
  the pattern, and fails on a capture naming a member of `verb.Commands()`.

`cmd/dinah/references_guide_test.go` loses:

- `TestTheReferencesGuideDeniesNoCommandTheToolHas` at line 789, replaced by the
  test above.
- `referencesGuideProseParagraphs` at line 747, with its doc comment.
- `guideDenialOfACapability` at line 737, which moves rather than being copied.

A tree-wide grep for `referencesGuideProseParagraphs` finds one caller, the
denial check at line 797. Repointing the check leaves the function dead, so it is
deleted in the same diff. That closes the duplicate prose reader the card
inherits, and no other test loses a helper.

### The backtick strip is applied at the call site

`guideProse` leaves backticks in place. Stripping them raises the firing count
over the corpus from 12 to 14, and it is what lets the pattern read a denial
written with either word in backticks. The strip belongs in
`TestNoGuideDeniesACommandTheToolHas`, applied to the sentence it has just
received, rather than inside `guideProse`.

Moving the strip into `guideProse` would change what
`TestNoSentenceStandsInTwoGuides` compares. Measured over the eight guides, that
introduces no new collision today, so the change would be safe. It alters
another check's subject for no gain to this one, so it is not made.

### The counts the sweep asserts

`TestNoGuideDeniesACommandTheToolHas` fails, rather than passing quietly, unless
all of these hold:

- `verb.Commands()` is not empty, so the roster it compares against read
  something.
- `guide.Topics()` is not empty, and the number of guides read equals
  `len(guide.Topics())`.
- Every single guide yields at least one sentence. The floor is per guide rather
  than over the corpus, because a total of 500 hides a guide that contributed
  none, and the smallest guide today yields 18.

The run logs the number of guides read and the number of sentences scanned, so a
passing run says what it examined.

### The table-strip justification does not survive

The comment on `referencesGuideProseParagraphs` says a folded table row runs a
`no` cell into the next command's name. That is false, and the function carrying
it is deleted, so the reason travels no further. The measurement behind the
correction, for the record: folding the references guide's table produces
`| show | no | yes | yes | yes | yes | | instructions | ...`, the pipes survive
the fold, the pattern never reaches a command name across a cell boundary, and
the scan gives zero firings both with the strip and without it.

Dropping table rows is still right, and `guideProse` still does it by skipping
lines beginning with a pipe. The reason is that a table row is not a sentence.
`TestNoGuideDeniesACommandTheToolHas` writes that rule out in its doc comment,
because `guideProse`'s own comment defers the rule to whichever test calls it.

### The false-positive shape is named in the failure message

The pattern reads a denial rather than a command mention, and it fires on true
prose wherever a command name stands as an ordinary noun. "A card has no comment"
and "a workbench has no path" both trip it, and both `comment` and `path` are
commands. No sentence of that shape stands in the eight guides today, which is
why the check is silent, and the quick start's "carries no block" shows the shape
is not hypothetical.

The failure message carries the guide's file name, the matched fragment, the
whole sentence, and a closing clause saying that a command name used as an
ordinary noun trips this check and that the fix is to reword the sentence rather
than to exempt it. Whoever meets the noise should be able to diagnose it from the
message alone, without opening the test.

    guide %s says %q and `dinah %s` is a command this build carries: %s
    (a command name standing as an ordinary noun trips this check; reword the
    sentence rather than exempting it)

## Step two: the archive statements are held to the tests that prove them

### The section carries five statements, not four

The card says four. Splitting `## Reading the archive` in
`internal/guide/guides/references.md` on blank lines gives five paragraphs, and
each opens with one statement about how archiving or restoring behaves. Grepping
the tree for a distinctive phrase from each finds no reader: `archive mirror at a
reference`, `Positions under the flag`, `travelled into the archive`, `says so on
its own first line` and `scans both halves` return nothing outside the guide
itself, and `restored column lands` returns only an error message at
`cmd/dinah/restore_test.go:460` that resembles the sentence rather than reading
it. The card's premise holds and its count was one short.

Each statement, with the test that already proves the behaviour it describes:

| Statement | Proving test |
|---|---|
| `--archived` reads the archive mirror at a reference's deepest collection step, and the live half at every step above it. | `internal/bench.TestTheArchivedHalfIsReadAtTheDeepestCollectionStep` |
| Positions under the flag count the mirror's own members. | `internal/bench.TestAPositionUnderTheFlagCountsTheMirrorsOwnMembers` |
| An entity that travelled into the archive inside its holder comes back with that holder. | `cmd/dinah.TestANestedArchiveRestoresInTwoActs` |
| A restored column lands at the end of the column order. | `cmd/dinah.TestRestoringAColumnReturnsItToTheOrderAndRepairsAStrandedCard` |
| A reference printed under `dinah contents --archived` below the walk's root is the address that child will have once the root is restored, and it does not resolve while the root is archived. | `cmd/dinah.TestAnArchivedReadShowsOneHalfAndWritesNothing` |

Two of the five are proved in `internal/bench` and three in `cmd/dinah`, so the
pin cannot be a helper in either package.

### The pin lives in its own package

`internal/guide/guidepin/guidepin.go`, package `guidepin`, follows
`internal/bench/compattest` exactly. It exists to be shared between test files in
two packages, no production file imports it, and it takes no `*testing.T`, so it
never imports `testing`. It imports `internal/guide` and nothing else from this
tree, and `internal/guide` imports only `internal/contract`, so no cycle closes.

```go
// Statement is one sentence a guide carries and one test proves.
type Statement struct {
	// Topic is the guide's topic, spelled as guide.Topics() spells it.
	Topic string
	// Text is the sentence, folded to single spaces, as the guide carries it.
	Text string
	// Provenance is the name of the test function that proves the behaviour
	// this sentence describes.
	Provenance string
}

// The five sentences, each named for what it claims.
const (
	ArchivedReadsTheDeepestCollectionStep         = "..."
	PositionsCountTheMirrorsOwnMembers            = "..."
	AnEntityComesBackWithItsHolder                = "..."
	ARestoredColumnLandsAtTheEndOfTheOrder        = "..."
	AnArchivedContentsRowIsTheAddressAfterRestore = "..."
)

// Pinned returns every pinned statement, in the order its guide carries them.
func Pinned() []Statement

// Carries reports whether the named guide still carries the sentence. It folds
// the guide to single spaces first, so a re-wrap of the source is not a
// failure. It refuses an empty sentence and an unknown topic rather than
// answering nil, because a pin naming nothing would otherwise pass.
func Carries(topic, sentence string) error
```

The sentence text lives in the constants and nowhere else, so a reworded guide is
corrected in one place.

### The five call sites

Each proving test opens with one line:

```go
if err := guidepin.Carries("references", guidepin.ARestoredColumnLandsAtTheEndOfTheOrder); err != nil {
	t.Error(err)
}
```

`t.Error` rather than `t.Fatal`, so a stale guide does not stop the behaviour the
test was written to prove from being verified in the same run.

### The pin roster is swept

`cmd/dinah/guide_pin_test.go` holds
`TestEveryPinnedStatementStandsInItsGuideAndNamesALiveTest`. It walks
`guidepin.Pinned()` and:

- fails if the roster is empty, and fails if it does not hold five statements;
- fails on any statement its guide no longer carries, naming the topic and the
  sentence;
- fails on any `Provenance` naming no `func <name>(t *testing.T)` anywhere under
  `repositoryRoot`, which stops a pin from outliving the test it claims to be
  tied to;
- fails if the scan for test functions read no `_test.go` file at all;
- logs the number of statements checked and the number of test files scanned.

`repositoryRoot` is already declared at `cmd/dinah/guide_guard_test.go:38` and
`guide_pin_test.go` sits in the same package, so it needs no second climb.

### The fifth statement is corrected before it is pinned

The guide's last sentence in that section reads "The listing says so on its own
first line." That is false. `renderTree` at `cmd/dinah/render.go:335` prints
`treeHeader` first and the `contents.archived` line after it, so the sentence
appears on the second line. Running the tool against a scratch workbench confirms
it:

    a card with a comment (probe-1) contains 1 entities.
    These addresses resolve once probe-1 is restored.
      Reference               Entity   Title      Count
      ----------------------  -------  ---------  -----
      `-- probe-1/comments/1  comment  a comment  0

The guide sentence becomes "The listing says so once, on the line under its
heading." A pin that fixed the wrong sentence in place would be worse than no
pin, so the correction lands in the same diff and the constant carries the
corrected text.

`TestAnArchivedReadShowsOneHalfAndWritesNothing` asserts the line with
`strings.Contains` at `cmd/dinah/restore_test.go:726`, which cannot see where the
line sits. It is tightened to split the human listing on newlines and require the
sentence on the second line, so the sentence the guide makes and the sentence the
test proves are the same sentence. The tightened assertion fails naming the line
it found instead.

### The first statement's set claim is derived rather than pinned

The same paragraph continues "`restore`, `show`, `path` and `contents` take it."
That is a claim about a whole set, and pinning it as a literal would prove only
that the guide still says it. `internal/verb` already declares the answer. Five
commands declare an `archived` parameter, and four of them carry
`Shared: "archived"`, which are `contents`, `path`, `restore` and `show`. The
fifth is `search`, whose `archived` parameter carries no `Shared` because the
flag widens a scan there instead of naming one half, and the guide's next
sentence says exactly that.

`TestTheReferencesGuideNamesEveryCommandThatTakesTheArchivedFlag` lands in
`cmd/dinah/references_guide_test.go`. It:

- derives the set by walking `verb.Commands()` and reading `verb.Params(name)`
  for a parameter named `archived` whose `Shared` is `archived`;
- reads the backticked names out of the guide's sentence and compares the two as
  sets, failing on a name standing in either side alone;
- asserts that `search` declares an `archived` parameter with an empty `Shared`,
  and that the same paragraph names `dinah search` separately. Pinning the
  excluded case beside the included ones stops a check that simply demanded four
  names from passing by being too strict;
- fails if either set is empty, and logs both sizes.

This is one addition beyond the card's two steps. The reason is that the sentence
is a set claim standing in the same paragraph as the statements the card asked to
pin, and pinning it as a phrase would give it the weakest form of protection
while looking like the same protection as the others.

## The general ledger of prose claims is not built

Agreed with the card, and for the reason the card gives. The two steps above cost
one moved test, one deleted helper, one small package, five call sites and two
new checks. A ledger is a mechanism, and until these have run and something has
slipped past them there is no evidence about what shape that mechanism should
take. Nothing here forecloses one.

## Out of scope

- `guideDenialOfACapability` is moved and not edited. This card adds no
  narrowing, because the widened check fires zero times against the tree today,
  and a narrowing with no measured noise behind it is speculative machinery.
- No exemption list, and no mechanism for one.
- `docs/quick-start.md` is not reworded and not admitted to the corpus.
- No new message key, no locale file changes, and no change to any shipped string
  other than the one corrected guide sentence. The guides are English only and
  carry no locale copies.
- No change to the behaviour of `archive`, `restore` or `--archived`.

## Files this card touches

| File | Change |
|---|---|
| `cmd/dinah/guide_guard_test.go` | Gains `guideDenialOfACapability` and `TestNoGuideDeniesACommandTheToolHas`. |
| `cmd/dinah/references_guide_test.go` | Loses `guideDenialOfACapability`, `referencesGuideProseParagraphs` and `TestTheReferencesGuideDeniesNoCommandTheToolHas`; gains `TestTheReferencesGuideNamesEveryCommandThatTakesTheArchivedFlag`. |
| `internal/guide/guidepin/guidepin.go` | New. The five constants, `Statement`, `Pinned` and `Carries`. |
| `cmd/dinah/guide_pin_test.go` | New. `TestEveryPinnedStatementStandsInItsGuideAndNamesALiveTest`. |
| `cmd/dinah/restore_test.go` | Three `guidepin.Carries` call sites, and the `contents.archived` assertion tightened to a line position. |
| `internal/bench/resolve_archived_half_test.go` | Two `guidepin.Carries` call sites. |
| `internal/guide/guides/references.md` | One sentence corrected, from "on its own first line" to "once, on the line under its heading". |

## Arming, and every recipe below has to be performed rather than read

Each plant leaves the tree compiling, and each is restored from a byte-identical
copy afterwards.

1. **The widening.** Add the sentence "Dinah has no restore." as its own
   paragraph to `internal/guide/guides/verbs.md`, which is a guide the retired
   check never read. `TestNoGuideDeniesACommandTheToolHas` must go red naming
   `internal/guide/guides/verbs.md`. A plant in `references.md` proves nothing
   about the widening, because the retired check would have caught that one too.
2. **The backtick strip.** Plant the same sentence in `verbs.md` with `restore`
   in backticks. The check must go red. Removing the strip from the call site
   must make that plant pass, which is how the strip is shown to do work.
3. **The corpus floor.** Make `guideProse` return an empty slice for one topic.
   The check must fail on that guide's sentence count rather than passing.
4. **The roster floor.** Shadow `verb.Commands()` with an empty slice at the call
   site. The check must fail saying the roster read nothing.
5. **Each pin.** Delete one pinned sentence from `references.md` and run the test
   named in that statement's `Provenance`. It must go red naming the topic and
   the sentence, and the run must still verify the behaviour it was written for.
   Repeat for all five.
6. **The provenance scan.** Rename one proving test function. The sweep must go
   red saying the pin names a test that does not exist.
7. **The line position.** Move the `s.line(s.r.T("contents.archived", ...))` call
   in `cmd/dinah/render.go` above `s.line(s.treeHeader(tree))`. The tightened
   assertion in `TestAnArchivedReadShowsOneHalfAndWritesNothing` must go red, and
   the retired `strings.Contains` form would have passed.
8. **The derived set.** Remove `Shared: "archived"` from `path`'s archived
   parameter in `internal/verb/definition.go`.
   `TestTheReferencesGuideNamesEveryCommandThatTakesTheArchivedFlag` must go red
   naming `path`. Then add `Shared: "archived"` to `search`'s archived parameter
   and confirm the check goes red on the excluded-case half as well.

For every plant, confirm the run compiled and executed before reading its red. A
plant that does not build prints nothing, and nothing looks like everything
passing.

## Branch

dinah-478-every-guide-is-scanned-for-a-denial
