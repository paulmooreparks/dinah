---
title: The references guide states the whole address contract, and a test holds its command table to the code
column: b69abf918c42
state: ready
severity: minor
priority: soon
tier: workhorse
workstreams:
  - 994787601ae6
links:
  - kind: spawned_from
    to: c7d5ff7728c1
  - kind: relates_to
    to: 3c5a0bb55ba0
---
`internal/guide/guides/references.md` becomes the contract of record for how a person names anything in a workbench. dinah-456 sections 3.1, 3.2, 7 and the rulings in 8 specify what it has to say.

The guide is wrong today in a way nobody noticed. Its opening sentence says ten commands take a reference, and fifteen commands point their reader at it. The five it omits are the checklist-state verbs. It also says `show` accepts anything below a card, and `show` refuses a collection where `path` accepts one, which is what led a reader to type a reference the tool then said did not exist.

The table's roster stops being hand-written. A test derives the fifteen from the `guides` map in `internal/verb/definition.go` and fails both ways: a command in the map with no row, and a row naming a command the map does not carry. A hand-maintained list is what produced the current drift.

This card goes first in its group, because the five cards beside it are checked against what the guide says.

## Specification

This card rewrites `internal/guide/guides/references.md` into the contract of record for
addressing, adds the reciprocal seam paragraph to `internal/guide/guides/query.md`, and lands
six tests plus one fixture that hold the guide to the code in both directions. It ships no
production Go change: the diff is two Markdown files, one new test file, one new testdata
fixture, four edits to existing tests, and two prose-ledger entries.

Everything below was verified by running against `origin/main` at
`b825059101980d52eb33dbd5bc62491f4c3c1e7a` ("dinah-438: repair the dashed array reader so a
multi-key entry keeps all its fields"). Round 1 ran in `C:/dinah-scratch/dinah-457-spec/wt`;
round 2 re-fetched `origin/main`, found it still at `b825059`, and ran its own plants in
`C:/dinah-scratch/dinah-457-spec2/wt`. Both siblings the parent's round 5 recorded have
landed and are in that tree: dinah-454's address sweep, so `show` now prints a `Ref` column
for comments and attachments and `checklistKinds` prints `questions`, `criteria` and
`decisions`, and dinah-459's `attach` refusals. Where a claim below was produced by running a
binary built at that commit, the section says so and gives what came back. The probe workbench
stood inside the worktree's own directory with `DINAH_HOME` pointed inside it.

The contract this card implements is dinah-456 sections 3.1, 3.2, 7, the rulings in 8, the
seam paragraph of 12.4, and the name rulings of 13.1 to 13.3. Nothing here re-derives it. Two
places go past what the parent wrote out, and section 9 names both. Section 10 records the
plants round 2 performed and the calls it took on the review's smaller findings.

# 1. The roster, derived rather than counted

Fifteen commands take a reference. That was derived from the exported API rather than read
off any prose, the parent's included:

```go
for _, name := range verb.Commands() {
        for _, g := range verb.Guides(name) {
                if g == "references" { refs = append(refs, name) }
        }
}
```

Run at `b825059` it prints:

```
15 [archive attach attachments cite contents delete edit fail instructions path rename
reopen resolve show verify]
```

`verb.Guides` merges the `guides` map at `internal/verb/definition.go:211` with every
`Param.Guide` in `params`, so one call covers both rosters and they cannot disagree unseen.
Both rosters are full: fifteen `guides` entries name `references`, and fifteen `params`
entries carry `Guide: "references"`, one per command. Clearing either one alone leaves the
merged roster at fifteen, which is what AC-3 has to plant against and what section 5.2's
comment has to say. The guide's table carries ten rows today and its section sentence says
"Ten commands take a reference". The five it omits are `cite`, `resolve`, `verify`, `fail`
and `reopen`.

# 2. What the guide says today that is false

Read at `b825059`, each line confirmed by running.

| The guide says | What the tool does |
|---|---|
| "Ten commands take a reference" (line 64) | fifteen do |
| the table carries ten rows | five commands have no row |
| nothing about a workstream | six commands take `workstream/<slug>` |
| nothing about a whole collection | `path` takes one, and the contract gives three more |
| an empty collection reports that nothing answers to it (line 60) | true today, and the contract makes an empty collection an empty answer |

# 3. The file this card rewrites

`internal/guide/guides/references.md`, in full. What follows is the text to land rather than
a sketch of it. The implementer may re-wrap prose paragraphs, since the server re-wraps them
to the reader's window anyway, but the table, the indented blocks, the counting sentences and
the two marker sentences named in section 3.2 land as written. Every test in section 5 that
reads guide prose reads it with whitespace folded, so a re-wrap of any other paragraph cannot
redden the suite. The text below was written into the worktree and the suite run against it,
which is where section 6's list of moved guards comes from.

**Before writing the file, compute the disagreement set.** dinah-455 is in Implement and it
changes what `show`, `contents`, `attachments` and `edit` do with a collection reference. Run
each command of the roster against a `<card>/comments` reference that holds a member, the way
section 5.4's test does, and compare what happened against the collection column of the table
below. If the set of commands that disagree with their cell is empty, dinah-455 has landed
first: drop the "Not every command has caught up" paragraph from section 3.1's text, drop
AC-5 and AC-6's plants, and say so in the handoff, because section 5.4's third rule then
requires the paragraph to be gone and shipping it verbatim fails the suite on arrival. If the
set is smaller than the four the paragraph names, name exactly the commands that disagree. At
`b825059` the set is `show`, `contents`, `attachments` and `edit`, which is the paragraph as
written.

## 3.1 The whole file

    # References

    You name a thing to Dinah by writing a reference, and the language a
    reference is written in is called DinahPath. DinahPath addresses things
    and never filters them, and every reference is one address rooted at
    something that names itself, so it takes no brackets, no wildcards, no
    functions, and no axes. Those four are examples of what those two rules
    leave out rather than the whole of it. When you want Dinah to find things
    for you instead of naming one yourself, write a query, and `dinah guide
    query` teaches how.

    A reference names this workbench, a workstream, a column, a card, or
    something that hangs off one of those, and you write it as a path with
    slashes between its parts.

    ## This workbench

    You may write this workbench in two ways, and the two mean the same thing:

        dinah path workbench
        dinah path .

    Your workbench's slug reaches the same place, so `wb/attachments/1` and
    `workbench/attachments/1` name one file.

    ## A card

    You write a card as its reference, which is your workbench's slug and the
    card's number:

        dinah show wb-1

    ## A column

    You write a column as its slug, its name, or its identifier:

        dinah attach doing notes.md

    ## A workstream

    You write a workstream as the word workstream, a slash, and the
    workstream's slug or its identifier:

        dinah contents workstream/addressing

    ## Something below a card

    You write something below a card as the card's reference, a slash, and the
    name of what you want:

        dinah path wb-1/card             the card's own file, which wb-1 alone gives you
        dinah path wb-1/journal          everything that has happened to the card
        dinah path wb-1/comments         every comment on the card
        dinah path wb-1/comments/1       one comment
        dinah path wb-1/checklist        every checklist item
        dinah path wb-1/checklist/1      one checklist item
        dinah path wb-1/attachments      every attachment
        dinah path wb-1/attachments/1    one attachment
        dinah path wb-1/attachments/1/payload
                                         the file the attachment carries

    Three more spellings each select one kind of checklist item:

        dinah path wb-1/questions        the open questions
        dinah path wb-1/criteria         the acceptance criteria
        dinah path wb-1/decisions        the decisions

    Dinah also accepts `oq`, `ac`, and `d` for those same three, which is what
    it printed and taught before the words landed. It never writes one of them
    back, so a reference you read off a screen carries the word.

    ## Naming a whole collection

    A reference that stops at a collection and picks nothing out of it names
    every live member of that collection:

        dinah show wb-1/comments

    A collection that holds nothing is an empty answer rather than a mistake.
    Which commands take a reference of that shape is settled one command at a
    time, in the table below, because an act that writes to a whole collection
    cannot be undone and Dinah has no restore.

    ## The number and the identifier

    You may write an entity's own identifier in place of its number, and you
    may write an attachment's filename in place of its number. Dinah tries the
    identifier first, then the position, then the filename, so an attachment
    named `1` or whose filename is twelve hex characters is reachable by
    ordinal and by identifier rather than by name. The number counts in the order the entities were created, which is not always the order a listing prints them in.

    The number is a spelling for now and the identifier is a handle to keep.
    Deleting an earlier member of a collection moves every number after it,
    and the identifier an entity is born with never changes. A screen prints
    the number because the number is what you are about to type, and `--json`
    carries both.

    ## Which command takes what

    Fifteen commands take a reference, and between them they accept six different sets of things. This table says what each one accepts:

    | Command      | A workbench | A column | A card | Below a card | A collection |
    |--------------|-------------|----------|--------|--------------|--------------|
    | path         | yes         | yes      | yes    | yes          | yes          |
    | edit         | yes         | yes      | yes    | yes          | no           |
    | show         | no          | yes      | yes    | yes          | yes          |
    | instructions | no          | yes      | yes    | no           | no           |
    | attach       | yes         | yes      | yes    | yes          | no           |
    | archive      | no          | yes      | yes    | yes          | no           |
    | delete       | no          | yes      | yes    | yes          | no           |
    | contents     | yes         | yes      | yes    | yes          | yes          |
    | attachments  | yes         | yes      | yes    | yes          | yes          |
    | rename       | no          | no       | no     | yes          | no           |
    | cite         | no          | no       | no     | yes          | no           |
    | resolve      | no          | no       | no     | yes          | no           |
    | verify       | no          | no       | no     | yes          | no           |
    | fail         | no          | no       | no     | yes          | no           |
    | reopen       | no          | no       | no     | yes          | no           |

    Six commands take a workstream: `path`, `edit`, `archive`, `delete`, `contents`, and `attachments`. The others refuse one, so the table leaves the workstream out rather than carrying a column that is mostly no.

    Five of those rows carry a detail the table is too coarse to hold.
    `attach` takes a comment below a card, and it takes an attachment only
    with `--replace`, which replaces that attachment's bytes rather than
    hanging a new file below it. It takes nothing else below a card, so `dinah
    attach wb-1/questions/1 notes.md` is refused. `instructions` takes a card
    or a column and nothing else at all. `contents` takes a card by the card's
    own reference and never through what holds it, so `dinah contents
    wb/cards/1` is refused and `dinah contents wb-1` is what you write.
    `rename` takes an attachment below a card and nothing else below one, so
    `dinah rename wb-1/comments/1` is refused. The checklist verbs `cite`,
    `resolve`, `verify`, `fail`, and `reopen` take a checklist item and nothing
    else below a card.

    Not every command has caught up with the collection column yet. Today
    `show`, `contents`, and `attachments` refuse a whole collection and tell
    you that nothing answers to it, and `edit` hands the collection's own
    directory to your editor instead of refusing it.

    Each command's own help page carries the same answer for that one command,
    so run `dinah help attach` when you want it beside the arguments rather
    than here.

    ## A reference or a query

    You name a thing with a reference and you find things with a query. A
    reference is an address: it starts at a card, a column, the workbench, or a
    workstream, and walks down to what that holds, so `wb-1/comments/1` names
    one comment and `wb-1/comments` names all of them. A query asks which
    cards match a condition, including conditions about what has happened to
    them, and it answers with cards. If you know which thing you want, write a
    reference. If you want Dinah to find the cards, write a query. Neither one
    does the other's job, so a reference takes no conditions and a query names
    nothing below a card.

## 3.2 Seven things in that text that are not free choices

**The name stands once, and the correction is the next sentence.** dinah-456 section 13.1
requires the surface that introduces DinahPath to say what DinahPath does not admit in the
sentence immediately after. The opening paragraph carries the two general statements, that it
addresses rather than filters and that a reference is one address rooted at a self-naming
head, four examples of what follows from them, the sentence saying the four are not the whole
of it, and the route to `dinah query`. Four rather than section 8's seven is the parent's
D-22. The four must not be written as a closed list, so no "only", no "the four things", and
no other closing word may enter that sentence.

**The name appears nowhere else in the file, and in no other file.** Section 13.2 rules the
title, the guide's body, help text, refusal text and every machine-surface field out one at a
time. The title stays "References", because `references` is the key in the `guides` map that
`dinah guide references` takes and that the MCP head serves the resource under. Section 5.5
is the guard.

**Three sentences land on one source line each, unwrapped.** The prose figure ledger scans
line by line, in `proseOccurrences` at `cmd/dinah/prose_figure_test.go:290`, so a figure and
its noun broken across a line break registers as no figure at all and the ledger entry naming
it goes stale. The three are "Fifteen commands take a reference, and between them they accept
six different sets of things. This table says what each one accepts:", "Six commands take a
workstream: `path`, `edit`, `archive`, `delete`, `contents`, and `attachments`. The others
refuse one, so the table leaves the workstream out rather than carrying a column that is
mostly no.", and the sentence already unwrapped today, "The number counts in the order the
entities were created, which is not always the order a listing prints them in." Section 6
gives the ledger entries the first two need. The draft above was run through the ledger's own
pattern and yields exactly two figures, on those two lines and nowhere else. Those two
sentences are the only prose in the file that is line-sensitive, and the ledger reddens if
either is wrapped, so a wrap is caught rather than trusted; every other paragraph, the caveat
paragraph included, may be re-wrapped freely.

**Every list of three or more items takes the serial comma.** The workbench document "Prose
standard" makes that a rule of the board's prose and names inserting the comma a safe
transformation, and the shipped guide corpus already follows it everywhere but one line. The
comma therefore stands in `no brackets, no wildcards, no functions, and no axes`; in the
workstream sentence's six names; in the checklist verbs `cite`, `resolve`, `verify`, `fail`,
and `reopen`; in the caveat's `show`, `contents`, and `attachments`; in the line naming the
three short spellings; and in both seam paragraphs' `a card, a column, the workbench, or a
workstream`. Section 9's D-12 records the call, section 5.6 asserts the first of them in its
comma'd form, and section 6 carries the one existing literal that has to move with it.

**No fenced block enters the file.** `cmd/dinah/testdata/guide-blocks.txt` declares every
fenced block of every guide, by guide and by the line its opening marker stands on, and a
block nobody declares fails. references.md has no entry in that ledger today, because every
block in it is a four-space indented block and that ledger governs fenced blocks alone.
Keeping every block indented leaves that ledger untouched.

**The guide names no refusal token.** `TestTheGuidesQuoteOnlyDeclaredRefusals` at
`cmd/dinah/guide_guard_test.go:177` fails a `dinah.`-prefixed name that `contract` does not
declare, and `dinah.is-a-collection` is minted by dinah-455 rather than here. So the caveat
paragraph says what happens in words and quotes no token. dinah-455 may quote its own once it
exists.

**The table is exactly eighty columns wide, and that is what the header rename buys.** A
table row is exempt from the guide wrap by design, the fourth exemption named on
`TestEveryShippedGuideFitsEveryWindowItIsWrappedFor` at `cmd/dinah/guide_wrap_test.go:275`,
so the terminal folds a row the wrap will not. The existing header "This workbench" becomes
"A workbench", which frees the five columns the new "A collection" column costs and makes the
six headers one register. Section 5.7 is the guard that holds the eighty, and section 9's D-3
records the call.

# 4. The reciprocal in query.md

`internal/guide/guides/query.md` gains one section, at the end of the file, under "When the
query cannot say it":

    ## A query or a reference

    `dinah query` finds cards for you, and a reference tells Dinah which one
    thing you already mean. Write a reference when you know what you want and
    can say where it sits, from a card, a column, this workbench, or a
    workstream down to the comment or the attachment it holds. Write a query
    when you want Dinah to pick the cards out by what is true of them. The two
    stay separate on purpose. No condition may be written inside a reference,
    and no query reaches below a card. The guide on references teaches how one
    is spelled.

Neither seam paragraph names DinahPath, per section 13.2 of the parent.

The two paragraphs share no sentence of eight words or more.
`TestNoSentenceStandsInTwoGuides` at `cmd/dinah/guide_guard_test.go:992` compares every guide
against every other, sentence by sentence, normalised for whitespace and case, and fails on a
match. The two texts above were written apart for that reason rather than as a stylistic
preference, and copying one into the other fails the suite. Both texts were run through that
test's own `guideProse` rule against the whole shipped corpus, and the shared-sentence count
came back zero.

# 5. The new test file

One new file, `cmd/dinah/references_guide_test.go`, package `main`. It sits in `cmd/dinah`
rather than `internal/guide` because four things it needs already live there: `runCLI` and
`newBench`, the table parser `parseReferencesGuideTable` at
`cmd/dinah/references_command_resolution_test.go:161`, `repositoryRoot` at
`cmd/dinah/guide_guard_test.go:38`, and `displayWidth` at `cmd/dinah/row.go:59`. It carries
two helpers and six tests.

Every test in this file that reads the guide's text reads it through `guide.Text("references")`
rather than through the output of `dinah guide references`, which is wrapped to the reader's
window and would make a line-sensitive read depend on a terminal width. That is what
`parseReferencesGuideTable` already does.

## 5.1 `commandsTakingAReference` and `foldedGuideParagraphStartingWith`

```go
// commandsTakingAReference lists, in sorted order, every command the library
// points at the references guide. It reads verb.Guides rather than either
// declaration behind it, because Guides merges the command's own topics with
// every topic its parameters declare, so a command declared in one roster and
// not the other is counted once and no caller has to know there are two.
func commandsTakingAReference() []string {
	var names []string
	for _, name := range verb.Commands() {
		for _, topic := range verb.Guides(name) {
			if topic == "references" {
				names = append(names, name)
				break
			}
		}
	}
	sort.Strings(names)
	return names
}

// foldedGuideParagraphStartingWith returns the references guide's paragraph
// whose text opens with marker, with its whitespace folded to single spaces.
// The guide is hard-wrapped and section 3 lets the implementer re-wrap any
// paragraph the prose ledger does not pin, so a caller that matched a marker
// against the start of a source line would report a failure against a guide
// that is perfectly correct. A marker that matches nothing is a fatal naming
// the marker, so a reworded paragraph is repaired rather than read as an
// empty set.
func foldedGuideParagraphStartingWith(t *testing.T, marker string) string {
	t.Helper()
	text, err := guide.Text("references")
	if err != nil {
		t.Fatalf("guide references: %v", err)
	}
	for _, paragraph := range regexp.MustCompile(`\n\s*\n`).Split(text, -1) {
		folded := strings.Join(strings.Fields(paragraph), " ")
		if strings.HasPrefix(folded, marker) {
			return folded
		}
	}
	t.Fatalf("the references guide carries no paragraph opening %q, so this check read nothing", marker)
	return ""
}
```

Both callers take the backticked tokens of the returned paragraph and keep the ones naming a
command in the roster, so the set they compare is derived from the guide's own words rather
than from a second hand-written list.

## 5.2 `TestTheReferencesGuideTableNamesEveryCommandThatTakesAReference`

The card's reason for existing. It compares the command names in the guide's table against
`commandsTakingAReference()` and fails in both directions.

```go
func TestTheReferencesGuideTableNamesEveryCommandThatTakesAReference(t *testing.T) {
	// parseReferencesGuideTable ends with its own fatal over show, path and
	// edit, so a table stripped of its rows stops there rather than here and
	// the guard below cannot be reached while that fatal stands. It is kept
	// because this test's contract is that an empty subject set stops the run,
	// and the day the parser stops demanding three named rows is the day this
	// line starts carrying that contract on its own.
	declared := parseReferencesGuideTable(t)
	if len(declared) == 0 {
		t.Fatal("the references guide's table draws no row, so this check read nothing")
	}
	roster := commandsTakingAReference()
	if len(roster) == 0 {
		t.Fatal("no command points at the references guide, so this check read nothing")
	}
	takes := map[string]bool{}
	for _, name := range roster {
		takes[name] = true
		if _, drawn := declared[name]; !drawn {
			t.Errorf("%s points at the references guide and the guide's table carries no row for it", name)
		}
	}
	for name := range declared {
		if takes[name] {
			continue
		}
		t.Errorf("the references guide's table carries a row for %s and no command of that name points at the guide", name)
	}
	if len(declared) != len(roster) {
		t.Errorf("the table draws %d rows against a roster of %d: %v", len(declared), len(roster), roster)
	}
}
```

The roster guard is load-bearing and reachable, and AC-3 arms it. The table guard is not
reachable today, for the reason its comment gives, and the parser's fatal is what actually
closes that side; AC-3 plants against the parser's message rather than against this line.

## 5.3 `TestTheReferencesGuideNamesTheCommandsThatTakeAWorkstream`

The workstream sentence is a hand-written list of six names, so behaviour is what holds it.
The test runs every command in the roster against a workstream reference and compares what
happened against the sentence.

Fixture: one workbench, and one workstream per command, so an `archive` or a `delete` that
succeeds cannot change what the next command sees. Slugs are `ws1` to `wsN` in roster order.

```go
func TestTheReferencesGuideNamesTheCommandsThatTakeAWorkstream(t *testing.T) {
	root := newBench(t)
	t.Setenv("DINAH_EDITOR", "dinah-no-such-editor")
	named := workstreamSentenceNames(t)
	roster := commandsTakingAReference()
	if len(roster) == 0 {
		t.Fatal("no command points at the references guide, so this check read nothing")
	}
	reached := map[string]bool{}
	for at, name := range roster {
		slug := fmt.Sprintf("ws%d", at+1)
		if got := runCLI(t, root, "workstream", "new", "Stream "+slug, "--slug", slug); got.code != 0 {
			t.Fatalf("workstream new %s: %d %s", slug, got.code, got.errw)
		}
		got := runCLI(t, root, append([]string{name, "workstream/" + slug}, referenceProbeArgs(t, name)...)...)
		if commandTookTheReference(name, got) {
			reached[name] = true
		}
	}
	if len(reached) == 0 {
		t.Fatal("no command reached a workstream, so the fixture is broken rather than the guide")
	}
	for name := range reached {
		if !named[name] {
			t.Errorf("`dinah %s workstream/<slug>` works and the references guide's workstream sentence does not name %s", name, name)
		}
	}
	for name := range named {
		if !reached[name] {
			t.Errorf("the references guide's workstream sentence names %s and `dinah %s workstream/<slug>` does not take one", name, name)
		}
	}
}
```

`workstreamSentenceNames` calls `foldedGuideParagraphStartingWith(t, "Six commands take a
workstream:")` and returns every backticked token in the returned paragraph that is also in
the roster. The sentence is one the prose ledger pins to a source line, so it cannot be
wrapped without the ledger reddening, and reading it folded costs nothing and removes the
question.

`referenceProbeArgs` returns the arguments a command needs after its reference, and it is the
one hand-written table in the file. It is arguments rather than roster, and a command it does
not know is a `t.Fatalf` naming that command, so a sixteenth command entering the roster stops
here rather than being probed with the wrong line:

| command | extra arguments |
|---|---|
| `path`, `edit`, `show`, `instructions`, `contents`, `attachments`, `archive` | none |
| `delete` | `--yes` |
| `rename` | `renamed.txt` |
| `attach` | the path of a file the test wrote into `t.TempDir()` |
| `cite` | `url`, `https://example.invalid/evidence` |
| `resolve`, `verify`, `fail`, `reopen` | `a note` |

`commandTookTheReference` is `got.code == 0` for every command but `edit`. `edit` launches an
editor and the test points `DINAH_EDITOR` at a name no machine carries, so for `edit` alone
the answer is `!resolutionRefused(got.errw)`, which is the rule
`TestEveryAddressTheContentsTreeDrawsResolvesThroughTheCommandsThatDeclareIt` already applies
at `cmd/dinah/references_command_resolution_test.go:99`. `resolutionRefused` is in that same
file and this test calls it rather than restating it.

What the probes returned at `b825059`, which is where the sentence's six names come from:

| command | against `workstream/addressing` |
|---|---|
| `path` | exit 0, printed the workstream's directory |
| `edit` | exit 4, `unreachable exec: "dinah-no-such-editor"`, so the address resolved |
| `archive` | exit 0 |
| `delete` | exit 0 |
| `contents` | exit 0, ` (workstream/addressing) contains nothing.` |
| `attachments` | exit 0, `workstream/addressing carries no attachments.` |
| `show` | exit 2, `dinah.unknown-path nothing in this workbench answers to addressing` |
| `attach` | exit 2, `dinah.not-attachable workstream/addressing is workstream, and Dinah keeps no attachments on workstream; attachments hang from the workbench, a column, a card, or a comment` |
| `instructions` | exit 2, `dinah.unknown-path` |
| `rename` | exit 2, `dinah.not-renamable workstream is not an attachment and rename will not change its name` |
| `cite`, `resolve`, `verify`, `fail`, `reopen` | exit 2, `dinah.unknown-path` |

## 5.4 `TestTheReferencesGuideDeclaresEveryCommandTheCollectionColumnIsAheadOf`

The collection column states the contract dinah-456 section 3.3 settled, and three of its
four accepting commands do not do it yet. This test holds the caveat paragraph to the tool,
so the caveat cannot outlive dinah-455 and cannot be written short either.

Fixture: one workbench, one card per command in the roster, each card carrying one comment,
so every probe meets a collection that exists and holds a member. An empty collection is not
usable here: at `b825059` even `dinah path pb-2/comments` refuses on a card with no comments,
because the directory is not written until a member is.

For each command, run it against `<card>/comments` and compare what happened against the cell
the table declares:

- the cell says `yes` and the command took it, or the cell says `no` and the command refused
  it, and the command agrees with the table;
- otherwise the command is one the caveat paragraph has to name.

`commandTookTheReference` from 5.3 decides "took it", so `edit` is judged by whether it
refused resolution rather than by its exit code.

The caveat's declared set is read the way the workstream sentence is:
`foldedGuideParagraphStartingWith(t, "Not every command has caught up with the collection
column yet.")`, then every backticked token in the returned paragraph that names a command in
the roster. Reading it folded is what lets section 3 keep licensing a re-wrap of that
paragraph, which the prose ledger does not pin.

Three rules hold the set:

- a command that disagrees with its cell and is not named in the paragraph fails,
- a command named in the paragraph that agrees with its cell fails,
- and if no command disagrees, the paragraph must be gone from the guide altogether, so
  finding the marker paragraph with an empty disagreement set fails naming dinah-455.

Two guards keep the sweep from passing on an empty subject set, and this test spells both
rather than pointing at 5.2. The roster is guarded by `if len(roster) == 0 { t.Fatal("no
command points at the references guide, so this check read nothing") }`, the same line 5.2
and 5.3 carry. The declared side is guarded by `foldedGuideParagraphStartingWith`'s own fatal
when the marker matches nothing, and by the third rule above when the marker matches and the
disagreement set is empty, so neither a missing paragraph nor an empty one is a silent pass.

At `b825059` the disagreement set is exactly `show`, `contents`, `attachments` and `edit`.
Probed:

| command | against `pb-1/comments` |
|---|---|
| `path` | exit 0, printed the collection's directory |
| `show`, `contents`, `attachments` | exit 2, `dinah.unknown-path nothing in this workbench answers to comments; run dinah ls to see what this workbench carries` |
| `edit` | resolved, handed the directory to the editor |
| `archive`, `delete`, `rename`, `attach`, `instructions`, `cite`, `resolve`, `verify`, `fail`, `reopen` | exit 2, `dinah.unknown-path` |

## 5.5 `TestTheNameDinahPathStandsOnlyWhereItIsDeclared`

dinah-456 section 13.3, which the parent assigns to this card. The test walks the repository
and holds every case-insensitive occurrence of `dinahpath` against a fixture allowlist of file
paths, failing in both directions.

- A hit in a file the allowlist does not carry fails, naming the file and the line. That is
  what refuses the name in a message catalogue, in a Go string, in `params` or `guides`, in an
  MCP tool name or schema, and in any guide but `references.md`.
- Zero hits in `internal/guide/guides/references.md` fails. A one-directional guard on a name
  is satisfied by deleting the name, and a guard anybody can satisfy by deletion is not a
  guard.
- Reading no file at all fails, so a walk that finds nothing cannot pass for free. The walk
  declared below reads 640 files at `b825059`, so the guard has room to fire rather than
  standing beside a set that was never going to be empty.
- An allowlist entry naming a file that does not exist fails, so the fixture is pruned rather
  than left to accumulate.

The walk starts at `repositoryRoot`. It descends into no directory whose name begins with a
dot, and into none named `node_modules`, `dist`, `out`, `bin` or `__pycache__`, which are the
generated and vendored trees `.gitignore` already excludes. It reads a file only if its
extension is one of `.go`, `.md`, `.ts`, `.mjs`, `.py`, `.txt`, `.json`, `.ndjson`, `.yml`,
`.sh`, `.ps1`, `.mod` or `.sum`. The extension set is why a built `dinah.exe` sitting in a
developer's checkout, which embeds the guide's bytes and therefore contains the name, does not
read as a hit. The set is declared in the test as a sorted slice carrying that reason.

Paths are compared as slash-joined paths relative to `repositoryRoot`, so the fixture reads
the same on every platform.

The fixture is `cmd/dinah/testdata/dinahpath-allowlist.txt`, read with
`filepath.Join("testdata", "dinahpath-allowlist.txt")`, which is how `testdata/uncovered.txt`
and `testdata/guide-blocks.txt` are read. A blank line and a line opening with `#` are
commentary, the way every other ledger in that directory reads. Its initial contents:

```
# The files that may carry the name DinahPath.
#
# DinahPath is the language a Dinah reference is written in, and dinah-456
# section 13.2 rules surface by surface where the name may stand: design
# documents, and the references guide's opening paragraph. It stands nowhere
# else, and it is deliberately absent from help text, from refusal text, from
# the guide's title, and from every field, key and tool name on the wire,
# because a name on the wire is a compatibility commitment.
#
# TestTheNameDinahPathStandsOnlyWhereItIsDeclared reads this file. A hit in a
# file this list does not carry fails, and zero hits in the references guide
# fails too, since a guard satisfied by deleting the name is no guard. Adding
# a design document that discusses the language is an edit to this list.

internal/guide/guides/references.md
cmd/dinah/references_guide_test.go
cmd/dinah/testdata/dinahpath-allowlist.txt
```

The test file and the fixture are on their own list because each carries the token in order to
search for it, and a guard that cannot name what it looks for cannot be written. The tree at
`b825059` carries zero case-insensitive occurrences of the name, so the three-line fixture is
complete the day it lands.

## 5.6 `TestTheReferencesGuideIntroducesDinahPathWithItsCorrection`

The parent's section 13.1 makes the correction a requirement on the paragraph rather than on
a person's care, and the mechanical part of it is worth a test even though the judgement is
not. Reading the guide with its whitespace folded to single spaces, the test asserts:

- the text carries `DinahPath` exactly twice, which is the name in the first sentence and its
  subject in the second, and no more,
- the folded text carries the phrase `no brackets, no wildcards, no functions, and no axes`,
  spelled with the serial comma section 3.2 requires,
- it carries `rather than the whole of it`, which is the clause that refuses to close the
  list,
- it carries none of the closing phrases `only four`, `the four things` or `and nothing else`,
- and it carries `dinah guide query`, which is the route out.

Whitespace is folded with `strings.Join(strings.Fields(text), " ")` before every comparison,
which is the rule the mcp guide's three checks already use at
`internal/guide/guide_test.go:82`. A criterion that quotes prose the file wraps reports a
false failure otherwise, and the opening paragraph is wrapped.

## 5.7 `TestTheReferencesGuideTableFitsAnEightyColumnWindow`

The width the header rename buys is held by nothing today. A table row is exempt from the
guide wrap by name, so the terminal folds a row no test measures, and the next person to
lengthen a heading breaks the table with the suite green. That is the card's own thesis turned
on the card, so the width becomes a test.

```go
func TestTheReferencesGuideTableFitsAnEightyColumnWindow(t *testing.T) {
	text, err := guide.Text("references")
	if err != nil {
		t.Fatalf("guide references: %v", err)
	}
	lines := strings.Split(text, "\n")
	headerAt := -1
	for at, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "| Command") {
			headerAt = at
			break
		}
	}
	if headerAt < 0 {
		t.Fatal("the references guide carries no \"| Command\" table header, so this check read nothing")
	}
	// Eighty is the window a guide table is written to fit. The wrap exempts
	// a table row, so nothing else in the suite measures one, and the terminal
	// folds what the wrap leaves alone. `dinah help attach` is held to the
	// same eighty at attachments_command_test.go:287.
	measured := 0
	for _, line := range lines[headerAt:] {
		if !strings.HasPrefix(strings.TrimSpace(line), "|") {
			break
		}
		measured++
		if drawn := displayWidth(line); drawn > 80 {
			t.Errorf("the references guide's table draws a line of %d columns, and the table is written to fit eighty: %q", drawn, line)
		}
	}
	if want := len(commandsTakingAReference()) + 2; measured != want {
		t.Errorf("this check measured %d table lines, and the table draws a header, a separator and one row per command, which is %d", measured, want)
	}
}
```

The line count is derived from the roster rather than written down, so the check cannot be
satisfied by a table that lost its rows, and `measured == 0` is impossible without the header
fatal firing first. Both plants were run against the draft table at `b825059` with a probe
carrying this body: restoring the header "This workbench" draws 83 and fails, and deleting the
`reopen` row measures 16 against a wanted 17 and fails.

# 6. The existing guards this card has to move

Five, each named with the line it stands on at `b825059`. None is optional. The draft guide
of section 3.1 was written into the worktree and the suite run against it, and these are the
tests that went red, with the message each gave:

**`cmd/dinah/main_test.go:7172`, `TestTheReferencesGuideSaysWhichCommandTakesWhat`.** Observed:
`the references guide does not teach "nothing answers to the reference rather than telling you
the collection is empty"`, then nine `does not carry the row` failures. Three edits to its
`form` list, which opens at `:7172` and reports through the `Errorf` at `:7181`. It loses the
literal `nothing answers to the reference rather than telling you the collection is empty`,
which the rewrite deletes. The literal at `:7176`, which quotes the guide's sentence about the
three short spellings, gains the serial comma before its final "and" to match section 3.1's
text; that one was observed failing against the comma'd draft with `the references guide does
not teach`. It gains `"dinah contents workstream/addressing"` and `"names every live member of
that collection"`. Its nine row literals become the fifteen rows of section 3.1's table,
spelled with the new column widths.

**`cmd/dinah/main_test.go:7214`, `assertTheGuideCountsItsOwnTable`.** Observed: `the references
guide draws no command row, so this assertion proves nothing`. Two edits. The cell count at
line 7224 goes from `!= 5` to `!= 6`, because a row now carries a command and five accept
cells rather than four, and leaving it at 5 makes every row skip, `commands` reach zero and
the assertion fatal on a guide that is perfectly correct. And the `words` slice at line 7241
runs to at least `"fifteen"`; it stops at `"ten"` today and the guard fatals when the count
reaches the length of the slice. Land it through `"twenty"`, so the next command to take a
reference does not have to touch it. Nothing else in that function changes: it goes on
deriving both figures from the drawn rows. Section 10 records why the `!= 6` stays a literal.

**`cmd/dinah/main_test.go:7690`, `TestAGuideTableSurvivesTheWindowItIsReadIn`.** Its four
literals carry the old header and separator. Replace them with the new header, the new
separator, and two rows of the new table, one from each end, `path` and `reopen`. That test
asserts the rows survive a 40-column read and measures no width, which is why section 5.7
exists beside it rather than inside it.

**`cmd/dinah/references_command_resolution_test.go:116` and `:131`.** Observed: `the references
guide's table declares nothing for show against "This workbench"`. The literal
`"This workbench"` becomes `"A workbench"` in the `seen` check and in `column`. Nothing else
in that file moves. `parseReferencesGuideTable` reads the header row for its column names and
requires every cell to be `yes` or `no`, and both hold of the new table, so the sixth column
is read and ignored by a test that asks about four.

**`cmd/dinah/testdata/prose-figures.txt:72`.** Observed: `TestEveryProseFigureIsDeclared` and
`TestNoProseFigureEntryIsStale` both failed. That is the one entry for references.md, and
section 5's new test replaces the holding it rests on. Both entries below, with `<line>` the
one-based line the sentence lands on, which the implementer reads off the file rather than
counting from this spec:

```
internal/guide/guides/references.md:<line> figure=Fifteen noun=commands counts=the commands that take a reference derives=referenceCommands
internal/guide/guides/references.md:<line> figure=Six noun=commands counts=the commands that take a workstream reference holds=elsewhere by=TestTheReferencesGuideNamesTheCommandsThatTakeAWorkstream reason=that test runs every command of the roster against a workstream reference and holds this sentence's named list against what each one did
```

`referenceCommands` is a new derivation in `derivationsByName` at
`cmd/dinah/prose_figure_test.go:433`:

```go
"referenceCommands": func(t *testing.T) (int, error) {
	return len(commandsTakingAReference()), nil
},
```

That turns the first figure from `holds=elsewhere` into `derives=`, which is the stronger of
the two holdings, and it removes the entry's pointer at a main_test.go line number that moves
whenever that file is edited.

No other prose figure enters the guide. The scan matches a figure, then up to two lower-case
words, then one of nine registered nouns, and the rewrite's other counting phrases ("Five of
those rows", "Three more spellings", "Those four are examples") stand beside nouns the ledger
does not register. The draft was run through that pattern in round 2, with the serial commas
in place, and yields exactly two matches, at lines 100 and 120 of the new file. An implementer
who rewords one of those phrases onto `commands`, `fields`, `flags`, `groups`, `kinds`,
`outcomes`, `settings`, `tools` or `verbs` owes the ledger an entry, and
`TestEveryProseFigureIsDeclared` says so by name.

# 7. What this card does not touch

- **`internal/verb/definition.go`.** The `guides` map is the source the test derives from, and
  a card that edited it while deriving from it would be proving nothing.
- **`dinah.is-a-collection`, and the behaviour of the eleven refusing commands.** dinah-455
  mints the refusal and moves the behaviour. This card writes the rule down and marks, in the
  guide and in a test, exactly where the tool has not reached it.
- **The checklist alias rename.** It landed with dinah-454 at `8661604`. The guide already
  carries `questions`, `criteria` and `decisions`, and the rewrite keeps that paragraph as it
  stands apart from the serial comma section 3.2 rules in.
- **The guide's title and its key in the `guides` map.** Both stay `references`, per the
  parent's 13.2.
- **Four spellings the parent's 3.2 names and this guide leaves out**, cut deliberately rather
  than missed: `card.md` and `journal.ndjson` as the files behind `wb-1/card` and
  `wb-1/journal`, the rule that a reference carries no leading slash, and an attachment
  hanging off a comment, `wb-1/comments/1/attachments/1`. The guide teaches a person how to
  name a thing, and all four are either a file layout a reader does not type or a shape the
  grammar already yields. Today's guide omits all four as well, so this is a cut held rather
  than a cut made. A later card that finds a reader tripping over one of them adds it.
- **The refusal text `attach` prints for a checklist item.** Probed at `b825059`, `dinah attach
  pb-1/questions/1 <file>` refuses with `dinah.not-attachable pb-1/checklist/1 is item`, naming
  the unaliased spelling of a reference the caller typed in the aliased one. That is a surface
  printing an address in a spelling nothing else prints any more, and it is recorded here so
  the observation is not lost. It belongs to whoever next touches that refusal rather than to
  this card, which changes no message.

# 8. How the implementer knows it works

```
cd C:/dinah-scratch/<your worktree>
go test ./...
```

The six new tests, the five moved guards, `TestEveryProseFigureIsDeclared`,
`TestNoProseFigureEntryIsStale`, `TestEveryDerivedProseFigureMatchesTheBinary`,
`TestNoSentenceStandsInTwoGuides`, `TestTheGuidesQuoteOnlyDeclaredRefusals`,
`TestTheDocumentationCarriesNoBannedTypography` and
`TestEveryAddressTheContentsTreeDrawsResolvesThroughTheCommandsThatDeclareIt` all read the two
files this card edits, and the acceptance criteria name the plant that reddens each of the new
ones.

# 9. The calls this card took past the parent's text

**The table gains one column, and the parent named one.** Section 7 of dinah-456 asks for the
per-command accept sets of 3.3 "folded in as a column rather than as footnotes", which is the
collection column. The workstream is in the parent's grammar at 3.1 and in its resolution
table at 3.2, so the guide has to teach it, and once the guide teaches a head the table is
silent about, a reader who types `dinah show workstream/addressing` gets a refusal the guide
predicted nothing about. The answer here is a sentence rather than a seventh column, held by
its own behaviour guard. D-4 records the reasoning, and a seventh column is unaffordable at
any spelling: the `Command` cell is pinned at 14 by `instructions` and `Below a card` at 14 by
its own header, so even a column headed `WS` adds five and breaks eighty.

**The dispatch brief said a collection reference is "accepted by four commands and refused by
twelve", and the number is eleven.** The parent's own table at 3.3 carries fifteen rows, four
`yes` and eleven `no`, and its section 10 says "four of them accept it and eleven refuse it
deliberately". Four and eleven is fifteen, which is the roster section 1 derived. **dinah-456
does not carry the wrong number and needs no repair.** The word "twelve" appears nowhere in
it, and its prose under the 3.3 table reads "Every writing command refuses, and so does
`instructions`", which names no number and double-counts nothing. The sentence that yields
twelve is in **dinah-455's own description**, which says "eleven writing commands plus
`instructions` refuse it" in the same paragraph as a correct "four accept it and eleven refuse
it deliberately". Eleven plus one is twelve, and four plus twelve is sixteen, which is nothing.
dinah-455's spec found the same slip independently and ruled the same way. The parent's table
governs. D-2 records it.

# 10. Round 2: the plants performed and the smaller calls

**Where the roster comes from, planted rather than reasoned.** Deleting the fifteen
`references` entries from the `guides` map leaves the roster at fifteen, because all fifteen
commands also declare `Guide: "references"` on a parameter and `verb.Guides` merges the two.
Clearing both declarations yields `ROSTER 0 []`. Every empty-subject guard on this card was
then checked by performing its plant rather than by reading the code:

- The roster guard of 5.2, 5.3, 5.4 and 5.7: planted both ways, as above.
- The table guard of 5.2: stripping the table's data rows stops in
  `parseReferencesGuideTable` with `the references guide's table carries no row for "show"`,
  reported at the caller's line because the parser is a `t.Helper`. The test's own
  `len(declared) == 0` line is never reached, which is what AC-3 now says.
- The declared-set guards of 5.3 and 5.4: the folded-paragraph read finds both markers in the
  draft, and still finds the caveat's marker after that paragraph is re-wrapped at 40 columns,
  where a line-prefix read finds nothing. That is the false failure the fold removes.
- The width guard of 5.7: both plants fire, as 5.7 records.
- The walk guard of 5.5: the declared extension set and skip list read 640 files at
  `b825059`, and the tree carries zero occurrences of the name.

**The serial comma is added, and one existing literal moves with it.** The prose standard
governs the guides, and the shipped corpus already carries the comma everywhere except the
line naming the three short spellings in this one file. Rather than leave the implementer to
guess, section 3.2 rules the comma in for all seven lists, section 5.6 asserts the opening
paragraph's phrase in its comma'd form, and section 6 carries the `form` literal at
`main_test.go:7176` that goes red without the matching edit, which was observed. D-12 records
it.

**"Which is why" leaves the workstream sentence.** The prose standard names it as the standard
causal connective to avoid, and turning "which is why X" into "so X" is on the standard's own
list of safe transformations. The sentence is pinned to one source line by the prose ledger,
and the replacement is one word shorter, so nothing else moves.

**The `!= 6` cell count stays a literal, and here is why.** Deriving it from the header row
would be the DIRT-aligned shape, and `assertTheGuideCountsItsOwnTable` is a function this card
edits rather than owns: it holds every guide table's counting sentence, not this one's, and a
change to how it finds a row changes what it proves for tables this card never reads. The
literal is left where it is, with the observation recorded that the next column addition
reproduces the misleading `draws no command row` fatal, so whoever adds that column has the
diagnosis in hand. D-14 records the decline and its reason.

## Branch

dinah-457-the-references-guide-states-the-whole-address-contract-and-a-test-holds-its-command-table-to-the-code

## Retired checklist items (no Dinah state for these yet)

These crossed from the Andoneer board in a state Dinah has no word for. dinah-472 is the card that adds it; until then they live here.

- **acceptance_criterion** (Andoneer AC-5, state `obsolete`)
  - Text: Remove the token `` `edit` `` from the guide's paragraph beginning `Not every command has caught up with the collection column yet.` and run `go test ./cmd/dinah/ -run TestTheReferencesGuideDeclaresEveryCommandTheCollectionColumnIsAheadOf`. It fails naming `edit` as a command whose behaviour disagrees with its cell and which the paragraph does not declare. Restore it and add `` `path` ``: it fails naming `path` as declared and agreeing with its cell. Then restore the paragraph and re-wrap it at 40 columns, changing no word, and the test passes, because `foldedGuideParagraphStartingWith` folds the paragraph's whitespace before matching the marker rather than reading the start of a source line.
  - Note: Obsolete because dinah-455 landed first, as 70ff10f on the trunk, which is the case spec section 3 anticipates. The disagreement set was recomputed at that commit before the guide was written, by running all fifteen commands against a <card>/comments reference holding a comment: path, show, contents and attachments answer it (exit 0) and the other eleven refuse with dinah.is-a-collection, which is exactly what the table's collection column declares. The set is empty, so the caveat paragraph is not in the shipped guide and this criterion's plants have no paragraph to edit. The live rule in its place is rule 3 of TestTheReferencesGuideDeclaresEveryCommandTheCollectionColumnIsAheadOf, which was armed: putting the caveat paragraph back into the guide fails with `references_guide_test.go:268: every command now agrees with its "A collection" cell and the references guide still carries the paragraph opening "Not every command has caught up with the collection column yet."; dinah-455 has landed, so the paragraph goes`. Rules 1 and 2 were armed too, by flipping show's collection cell to `no`: alone it fails on the missing paragraph, and with a caveat naming show and path it fails with `the references guide's caveat paragraph declares path and path agrees with its "A collection" cell`. One correction to the spec's judgement rule was needed and is in the diff: commandTookTheReference judges `edit` by whether the address was refused, and resolutionRefused knows only dinah.unknown-card and dinah.unknown-path, so a new addressRefused widens it with dinah.is-a-collection. Without that widening edit reads as having taken a collection it plainly refuses, and the set would have looked like {edit} rather than empty.
- **acceptance_criterion** (Andoneer AC-6, state `obsolete`)
  - Text: Delete the whole `Not every command has caught up` paragraph from the guide and run the same test. It fails four times, once per command whose behaviour disagrees with its cell and which nothing now declares, and it does not stop early on `foldedGuideParagraphStartingWith`'s missing-marker fatal, because the test asks for that paragraph only when the disagreement set is non-empty. This is the state the guide reaches when dinah-455 lands and somebody deletes the paragraph too early.
  - Note: Obsolete for the same reason as AC-5: dinah-455 landed as 70ff10f before this card, the disagreement set is empty at that commit, and the shipped guide carries no caveat paragraph to delete. This criterion's own note anticipated the other half, which is the half that is now live and which was armed: with nothing disagreeing, the paragraph's presence fails rather than its absence. See AC-5's note for the three plants that armed all three of the test's rules, each performed and each watched running.
