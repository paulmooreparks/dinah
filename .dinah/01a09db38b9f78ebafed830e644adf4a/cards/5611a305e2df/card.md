---
title: The documentation guards name their own blind spots, and both blind spots have now shipped defects
column: b69abf918c42
state: ready
severity: major
priority: next
tier: frontier
workstreams:
  - 4fd7a9f0b8ff
links:
  - kind: relates_to
    to: 38d8c2b09eb7
---
## The finding

This repository holds its documentation to the tool with two guards, and both guards carry comments naming what they cannot see. A documentation audit on 2026-09-08 found live defects sitting in exactly those named gaps, and trunk broke on 2026-09-07 on the same class. The guards are working; the problem is that their boundaries are recorded as prose and nothing acts on that prose.

## What each guard cannot see, in its own words

**The quick start's guard lists "a count written out in prose" as a known blind spot.** It replays the document and compares transcripts, so a fenced block that disagrees with the tool fails. A sentence saying "twenty-one tools against its twenty-nine commands" does not, and that sentence is wrong today by 22 and 22 respectively (dinah-446, finding 1).

**The guide guard forbids a fenced block whose first line starts with `$ `, and tells authors to write the command without the dollar sign.** `TestNoGuideCarriesATranscriptTheReplayDoesNotDrive` is what stops a guide carrying an undriven transcript, and the instruction it gives authors is precisely what makes a transcript invisible to it. The one guide transcript in the tree, in `principles.md`, is stale (dinah-446, finding 5).

**A third gap, found by the audit rather than named by a comment, is worse than either.** `replayQuickStart` at `cmd/dinah/quickstart_test.go:1214` writes the document's own `file` blocks into the sandbox before the next command runs. So the quick start injects its stale frontmatter and a later transcript faithfully echoes it back. **The guard proves the document agrees with itself, not that it agrees with the tool.** That is how `format: 1` and `profile: dinah-core/0.9` survived to today while `dinah init` writes `format: 2` and `0.12`.

## Why this is worth a card rather than a note

The evidence that instructions do not fix this is on the record twice in one week.

On 2026-09-07 trunk went red because two cards each added message keys, each correctly re-derived the catalog count, each merged and re-ran, and each correctly found the other absent because when it looked it genuinely was. Both authors did exactly what the workbench instructions ask. The repair landed as dinah-425 and the operator ruled a branch-protection setting on, which addresses that instance: no untested combination lands. **It does nothing for a number nobody re-derived at all, which is this card.**

The pattern the audit surfaced is the same failure with the noun changed. There it was a number carried without being re-derived; here it is a number nobody could re-derive because nothing looks at prose, and a behaviour described without being re-executed. In every case the author was careful and the reasoning sound.

## What this card is for

Close the gaps, or make them visible where somebody meets them. The mechanism is the spec's to rule and the shapes worth weighing include, at least:

Making a prose figure checkable, so a number in a sentence is held the way a number in a transcript is. Consider whether the honest answer is that prose should not carry a figure at all, which is the rule `docs/design/token-cost.md` already follows and which dinah-380 was held to.

Making a guide transcript driveable, which means resolving the contradiction between the guard's rule and the instruction it gives authors. A rule that pushes writing into the shape it cannot check is worse than no rule, because it reads as coverage.

Stopping the quick-start replay from feeding the document its own stale input, so the guard compares the document against the tool rather than against itself. Note this one may be expensive: the `file` blocks exist because the replay needs a workbench in a known state, so removing them needs a substitute rather than a deletion.

## What must not happen

**Do not fix the six defects here.** They are dinah-446. This card fixes the reason they were invisible, and a card that repairs both the instrument and the instances proves nothing about the instrument.

**Do not widen a guard until it passes.** The workbench has ruled on this shape twice, on dinah-382 where an implementer refused to widen a bound to make a run go green, and on dinah-383 where the operator ruled a tolerance be tightened rather than loosened. A guard adjusted to admit what it currently sees certifies nothing.

**Do not rest a check on undocumented behaviour**, which is a standing rule here.

## How to know it worked

The test is not that the suite is green, since it is green today with six false statements in the tree. The test is that a defect of each named class, introduced deliberately, is caught. That means arming each new check by breaking the thing it protects, watching it fail, and restoring, which is the discipline every card in the token-cost workstream was held to.

`docs/design/token-cost.md` is prior art worth reading before specifying: every figure lives inside a fenced block quoted from a real run, with no count or ratio in the surrounding prose. The session that repaired trunk on 2026-09-07 pointed dinah-425 at it for the same reason.

## Specification

## What the two guards cover today

I read both guards in the tree at `cf651ab` rather than taking the card's account, and the account needs three corrections.

`cmd/dinah/quickstart_test.go` holds `docs/quick-start.md`. `TestTheQuickStartMatchesTheTool` at line 800 parses the document into fenced blocks, drives every `console` block the exemption file does not excuse through `runCLI` at two window widths, and compares the captured bytes against the document's after five normalisation classes are erased. `cmd/dinah/testdata/quickstart-exempt.txt` names the eight blocks the replay does not drive, each with a reason and the catalog keys it quotes, and five rules keep that file honest: a stale entry fails, an undeclared exempt block fails, an entry with no reason fails, an entry declaring neither `quotes=` nor `quotes=none` fails, and a `quotes=none` entry with no `because=` fails. The doc comment at lines 29 to 66 lists eleven things the guard cannot see, and "a count written out in prose" is the second of them.

That comment is one release out of date, which is the first correction. dinah-446 landed `TestTheQuickStartCountsTheCommandsTheBinaryOffers` at line 1693, and it holds exactly one sentence of prose: it counts the grouped entries of the `commands` table, spells the count through `numberWord`, finds `lists all ([a-z-]+) commands` in the document, and fails naming both values. A second prose figure is held elsewhere, which round 1 of this spec missed. `assertTheGuideCountsItsOwnTable` at `cmd/dinah/main_test.go:7213` counts the rows of the references guide's own table and holds the sentence "Ten commands take a reference" against that count, so `internal/guide/guides/references.md:60` is checked today by a mechanism reading the document's table rather than the binary. Two prose figures out of twenty-three are held, and both are held by hand-written assertions aimed at one sentence each. I opened both assertions rather than trusting their names, because a holder that does not read the document and does not count the set is not a holder, and this spec claimed one such holder in its previous round. `TestTheQuickStartCountsTheCommandsTheBinaryOffers` reads `docs/quick-start.md` off disk, finds `lists all ([a-z-]+) commands`, and compares the word it captures against the grouped entries of the `commands` table. `assertTheGuideCountsItsOwnTable` reads the rendered references guide, counts the rows of the drawn table, and compares the spelled figure against that count. Both pass that test, and nothing else in either guard does.

The second correction concerns the third gap. dinah-446 also landed `TestTheQuickStartWorkbenchFileDeclaresWhatTheBinaryWrites` at line 1745, which reads the `format:` and `profile:` lines of the `file path=<workbench>/workbench.md` block against `bench.StorageFormat` and `bench.ProfileVersion`. Its own doc comment states the loop this card is about. The gap is therefore narrowed rather than open: the two keys of the one anchor block are held, and every other key of every other `file` block still travels the loop.

`cmd/dinah/guide_guard_test.go` holds the eight embedded guides and the quick start as text. Eleven checks stand in it, and they read refusal names, environment variable names, published filenames, release artifact names, the layout guide's drawn paths, profile statements, the contract verbs, and repeated sentences. `TestNoGuideCarriesATranscriptTheReplayDoesNotDrive` at line 117 is the one this card is about. It walks each embedded guide's fenced blocks and fails when the line after an opening marker begins with `$ `, and its message tells the author to "write the command without its leading dollar sign".

The third correction is that the rule is narrower than the card says even within its own shape. It reads the line immediately after the fence, so a transcript whose commands begin on the second line of the block is invisible to it even when the dollar sign is written, and `quickBlock.commandBlock` in the other file already documents that difference for the quick start's own selection.

## What the tree actually carries, measured

The guide corpus is 21 fenced blocks: thirteen `json` payloads in `mcp.md`, five command listings in `getting-started.md`, one shell one-liner at `mcp.md:195`, one directory tree at `workbench-layout.md:6`, and one rendered `dinah columns` table at `principles.md:29`. Not one of them opens with `$ `, so the guard reports nothing today, and the block that shipped a defect is the table.

The quick start carries three `file` blocks, whose opening fences stand at lines 326, 344, and 724. The first two land on files `dinah init` already wrote, and the third creates `notes.txt`, which the tool never wrote and which the `dinah attach` step at line 731 needs.

Counting number words and digits outside fenced blocks across the guides and the quick start finds 385 figures. That number is the whole design problem of the first gap, and it is why this spec rules the way it does below.

## The ruling on where a prose guard may look

A guard that reads prose has to choose a reach, and 385 is the count if the reach is every figure. Most of those figures are ordinary English, because "one" and "two" carry sentences here as pronouns and as counts of things no source file declares. A guard failing on those is a guard somebody switches off within a week, and this repository has ruled twice that a guard is not adjusted to pass, so the way to protect that ruling is to give the guard a reach it can hold rather than a reach it will have to be argued out of.

The reach this card ships is a minted vocabulary of countable nouns, declared in a ledger and nowhere else. A figure is visible to the guard when it stands within two words of a noun the ledger registers, and it is invisible otherwise. Nine nouns are registered, and they make 23 occurrences visible across the whole corpus. Registering "cards" or "columns" would make dozens more visible while proving nothing, because those nouns count the content of an example rather than a set the binary declares, so the ledger does not register them and the guard never reads them. The vocabulary is a value rather than a literal, so widening the reach later is one line in a text file, and narrowing it is a diff a reviewer sees.

## The ruling on what an entry keys on, and why it is not the noun

Round 1 of this spec bound a derivation to a noun, which does not survive contact with the corpus. The word "commands" occurs eight times under the detector, and it names four different sets: the fifty-one entries `dinah help` lists, the five verbs that change where a card stands, three of the reading commands, and the ten commands that take a reference. One derivation cannot hold four sets, and "fields" splits the same way between the twelve query fields and the five journal fields among them. A noun is a word, and a countable claim is about a set, so the ledger keys on the claim.

Three rulings follow from that, and together they replace the bound round 1 put on the escape hatch.

**The unit is the occurrence.** One entry stands for one figure standing next to one registered noun at one line of one document, identified by the tuple of document, line, figure, and noun. A line carrying two occurrences, as `docs/quick-start.md:105` does, carries two entries.

**Every entry names the set its figure counts, in the `counts=` phrase, rather than inheriting one from the noun.** The phrase is prose written by whoever files the entry, and it is what a reader of the ledger sees first. The derivation, where there is one, hangs off the entry through `derives=`, and the noun registers nothing but the reach.

**Entries sharing a `counts=` phrase must agree.** Two entries naming the same set must carry the same figure value and must be held the same way. That rule is what keeps D-2 intact after the per-noun bound is gone, because the corpus states the five verbs seven times, and an author who wants to escape the derivation on one of those seven has to escape it on all seven, in one visible diff, or redden the run.

A second bound stands beside it. Every derivation registered in the code must be named by at least one entry, so converting the last entry that uses a derivation into a declared exception leaves a registered derivation nothing reads, and that fails too. Neither bound is airtight, and the residual is stated under "What this closes and what it does not" rather than dressed up.

## The ruling on a figure no derivation reaches

A figure inside the reach that the mechanism cannot compute is declared with a reason rather than forbidden, because "one of four outcomes" is a true sentence and a mechanism that pushes writing around to suit itself is worse than one that records what it cannot check. The declaration is what makes such a figure visible to the next reader.

Four classes cover every such figure in the corpus, and each carries its own rule rather than standing as a comment.

`holds=partitive` is a figure counting a member or a subset rather than the set, as "one of four outcomes" and "three of the reading commands" do. It requires `reason=`.

`holds=elsewhere by=<test> reason=<how>` is a figure another check in the tree already holds. The mechanism verifies that the named test is declared somewhere in the module's test files, and that is the whole of what it verifies. It cannot tell whether the named test opens the document or counts anything, so `by=` proves the test exists and the `reason=` phrase carries the claim that the test holds this figure. That claim belongs to a person, and whoever files or reviews an entry of this class opens the named test and reads it before the entry stands.

The class is written that way because round 2 of this spec published a claim of exactly the kind this card exists to abolish. It declared `internal/guide/guides/mcp.md:298`, the sentence "Eight tools carry `basis`", as held by `TestBasisIsPublishedExactlyWhereItIsConsumed`. That test stands at `internal/mcp/tools_inventory_test.go:148`, it never opens `mcp.md`, and it counts nothing. It walks `tools` and asserts that a tool inside `basisProbeArguments` answers stale on an impossible basis while a tool outside it is refused the argument, so a ninth basis-consuming tool leaves it green and leaves the sentence stale. A ledger publishing that entry would say a figure was held when nothing held it, in the artifact whose whole purpose is to make coverage boundaries honest. One seed entry takes the class now, `internal/guide/guides/references.md:60`, and its holder does read the document and does count.

`holds=unreachable at=<file>:<line> declares=<identifier>` is a figure whose set the tree declares where package `cmd/dinah` cannot read it, which today means an unexported declaration in another package. The mechanism checks that the file exists and that the named line carries the named identifier as a word. Round 2 specified that the line merely be non-blank, which passes on a doc comment, on a closing brace, and on whatever the file grows next, so it was not a check on a pointer at all. This class needs no `reason=`, because the pointer is the reason, and a later card can export the set and convert the entry to a derivation.

`holds=prose` is a figure counting something the tree declares nowhere, including a figure that turns out on reading to be no count at all. It requires `reason=`.

`counts=` never takes the literal `none`, and `TestEveryCountedSetIsCountedConsistently` fails an entry that writes it. A figure that counts nothing still stands for something, and the phrase says what it stands for. The consistency rule groups entries by the phrase as written, so a sentinel spelled one way puts unrelated figures in one group, and the second author to reach for it reddens a run over a collision rather than over a disagreement. Round 2 seeded `counts=none` on `docs/quick-start.md:888`, which taught that spelling in the one place a later author goes looking for an example. The ledger's header comment carries the refusal as well, since the seed is what gets copied. The `teaches=none` spelling in `cmd/dinah/testdata/quickstart-file-blocks.txt` is a different file, a different directive, and a list of keys rather than a phrase, so nothing here reaches it.

## The ruling on the three gaps

The first gap narrows with a ledger and a derivation table rather than closing with a rewritten convention. `docs/design/token-cost.md` proves the no-figures-in-prose convention works when one author holds it in mind across one document, and the audit proves it does not survive across nine documents and four releases. The convention stays, and the ledger is what notices when it lapses inside the registered reach.

The second gap closes by retiring the dollar-sign rule and holding every fenced block of every guide instead. The contradiction the card names cannot be repaired by changing which shape the rule keys on, because any shape-keyed rule tells authors to write the other shape. A rule over blocks rather than over first lines has no shape to evade.

The third gap closes for every frontmatter key a `file` block and the standing file share, and it closes without removing anything. The `file` blocks serve two purposes: they teach the reader what the file looks like, and they seed the sandbox so that later transcripts show the edited title and the wip limit. Both survive. What stops is the document's tool-owned values being written through into the sandbox unread, and `writeNarrativeFile` at line 1232 already has the machinery, because it reads the standing file and calls `restoreDocumentValues` on the document's bytes before the write.

## What this closes and what it does not

The card exists because boundaries recorded as prose went unacted on, so this spec states its own boundaries plainly rather than claiming three closures.

**Gap one is partially closed.** A figure standing next to one of the nine registered nouns is held. A figure standing next to any other noun is invisible, and nothing anywhere notices that a new countable noun has entered the corpus. A sentence saying that Dinah ships nineteen refusals raises nothing, because `refusals` is not registered. The registry has the same "somebody must remember" failure mode as the count it replaces, moved one level up. What it buys is that the boundary becomes a file with a reviewable diff instead of a sentence in a doc comment, that `TestEveryLedgerReferenceIsLive` stops the vocabulary rotting in the other direction, and that the twenty-three figures inside the reach include every count the corpus makes about the binary's own declared sets. The action a writer can take is stated in the ledger's own header comment: when you write a sentence counting something, either write it about a registered noun or add the noun, because nothing will remind you.

Three further residuals belong to gap one. Two entries meaning the same set in different words escape the consistency rule, since the rule compares the `counts=` phrases as written. An author who converts one of several sibling entries away from its derivation reddens the run only while the siblings share a phrase, which they do throughout the seed, and only until somebody rewords one.

The third residual is the `holds=elsewhere` class, and it is the sharpest one here, because a coverage claim nothing stands behind is this card's own subject. The mechanism proves the named test is declared. It cannot prove that the test opens the document or that it counts anything, so an entry of that class rests on the `reason=` phrase a person wrote and a reviewer read. Round 2 of this spec shipped such an entry with a false phrase, which is the evidence that the residual is real rather than theoretical. The class stays, because pointing at a real holder beats pretending a held figure is unheld, and it is bounded: one seed entry uses it, and each new one costs a reviewer the minute it takes to open the test.

**Gap two closes.** `TestEveryGuideBlockIsDeclared` sees a new fenced block however its author spells it, so there is no shape an author can choose that hides a block from the ledger. That is the contrast worth drawing against gap one: a rule over blocks has no evasion, and a rule over words has one.

**Gap three closes for shared keys.** Every frontmatter key that a `file` block and the file the binary wrote both carry is compared, and a divergence fails unless the ledger declares that the block teaches it. A key the document carries and the standing file does not is not shared, so it is written through unchecked, which is correct because the tool never had an opinion about it. A `file` block landing where the tool wrote nothing is declared rather than compared, and `notes.txt` is that case.

## Files this card lands

The card adds two test files, three testdata ledgers, and one change to an existing helper. It removes three checks that the new mechanisms subsume, together with one helper of one of them. It corrects no documented figure except where the new guard reports one, which is covered under "What this card does not do" below.

```
cmd/dinah/prose_figure_test.go                  new
cmd/dinah/testdata/prose-figures.txt            new
cmd/dinah/guide_block_test.go                   new
cmd/dinah/testdata/guide-blocks.txt             new
cmd/dinah/testdata/quickstart-file-blocks.txt   new
cmd/dinah/quickstart_test.go                    writeNarrativeFile changed, two tests and one helper removed
cmd/dinah/guide_guard_test.go                   one test removed
```

## Gap one: the prose figure ledger

### The ledger

`cmd/dinah/testdata/prose-figures.txt` carries two kinds of line, and it follows the grammar `quickstart-exempt.txt` already uses, so `splitDirectives` at `cmd/dinah/quickstart_test.go:390` reads the directives of both files. A blank line and a line opening with a hash are commentary.

A noun line opens with the word `noun` and registers one countable noun for the scan. It carries nothing else.

```
noun commands
noun fields
```

A figure line opens with the document and the one-based line the figure stands on, and it carries `figure=`, `noun=`, `counts=`, and exactly one of `derives=` or `holds=`.

```
docs/quick-start.md:105 figure=fifty-one noun=commands counts=the commands dinah help lists derives=groupedCommands
docs/quick-start.md:561 figure=Five noun=commands counts=the verbs that change where a card stands derives=contractVerbs
internal/guide/guides/first-session.md:99 figure=one noun=outcomes counts=one of the four outcomes a command reports holds=partitive reason=the sentence says one of four outcomes, so the figure counts the choice rather than the set
```

`counts=` and `reason=` take a phrase. Where an entry carries both, `reason=` is written last and runs to the end of the line, and `counts=` ends at the first following directive name, which is the reading `splitDirectives` already gives `because=` in the exemption file. The implementer confirms that reading before writing the seed, and reports rather than works around it if the existing splitter cannot express a two-phrase line, in which case `counts=` takes a single quoted phrase.

### The derivations

`derivationsByName` in `cmd/dinah/prose_figure_test.go` maps a derivation name to a `func(t *testing.T) (int, error)`. The map is the only place a derivation is spelled, and an entry naming a derivation the map does not carry fails. A derivation must be computable from package `cmd/dinah` out of exported API, the in-package `commands` table, or `runCLI` output, which is what makes `holds=unreachable` a class rather than an excuse.

A derivation returning zero is a failure rather than a comparison, in the shape `TestTheQuickStartCountsTheCommandsTheBinaryOffers` already uses at line 1699, because a derivation that read nothing proves nothing.

Seven derivations serve the seed, and each one was read out of the tree at `cf651ab` rather than assumed.

| Name | What it counts | Source | Value today |
|------|----------------|--------|-------------|
| `groupedCommands` | the entries of the `commands` table carrying a group | `cmd/dinah/commands.go:39` onward | 51 |
| `commandGroups` | the distinct `group` values in that table | the same table | 4 |
| `contractVerbs` | the verbs the profile specifies | `verb.ContractVerbs` | 5 |
| `configKeys` | the settings `dinah config` accepts | `bench.ConfigKeys` | 4 |
| `checklistKinds` | the kinds a checklist item may carry | `bench.ItemKinds` | 3 |
| `queryFields` | the field names the query language admits | `verb.QueryFields` | 12 |
| `columnNewFlags` | the optional flags `dinah column new` accepts | the `Flag` entries of `verb.Params("column")`, which is the list `argumentLines` at `cmd/dinah/help.go:223` draws the page from | 5 |

`columnNewFlags` is settled rather than left to the first run, and it is five. Review ran `dinah help column` against this tree and the page draws five optional flags, which are `--kind`, `--tier`, `--capacity`, `--slug`, and `--before`. The page draws them from `verb.Params("column")`, whose `column` entry at `internal/verb/definition.go:504` carries five parameters marked `Flag: true`, so the derivation counts the page's own source rather than re-parsing rendered text or reading the request struct. `docs/quick-start.md:1493` says "All four optional flags have defaults", so this entry starts red and the sentence is a live defect this card's own instrument found.

The correction is wider than a word. The paragraph running from `docs/quick-start.md:1493` to 1497 names `--before`, `--kind`, `--slug`, and a column's capacity, and it omits `--tier` altogether, so changing four to five over a list of four ships a paragraph that is still wrong. The figure and the enumeration it counts move together, and D-10 carries the scope ruling.

### The comparison

A figure is written as a number word or as digits. `numberWord` at `cmd/dinah/quickstart_test.go:1720` spells twenty to ninety-nine and returns digits below twenty, so it cannot be reversed into a parser on its own. The comparison therefore carries `numberFromWord`, covering one to ninety-nine in the spellings the scan admits, and it reads digits with `strconv.Atoi`. The figure is compared case-insensitively, because a sentence-opening "Five" and a mid-sentence "five" are the same claim. A figure the parser cannot read fails naming the figure, since a figure nobody can read is a figure nobody can hold.

### The detection rule, stated exactly

The scan reads a line outside every fenced block and applies one pattern per registered noun:

```
(?i)(?P<figure>ONE|TWO|...|NINETY(-(ONE|...|NINE))?|[0-9]+)(?P<gap>( [a-z]+){0,2}) (?P<noun>NOUN)\b
```

Go's regexp has no lookbehind, so the implementation matches the whole shape and reports the submatches, and it rejects a match whose figure is preceded by a word character, a backtick, or a hyphen, so `dinah-446` and `<column:one>` are not figures. The alternation is ordered longest first, so `twenty-one` matches as one figure rather than as `twenty`. The gap admits at most two intervening words, each of them alphabetic and lowercase, so a match reaching across a quotation mark, a backtick, a comma, or a heading marker is not a match.

### The seed, walked line by line

The corpus at `cf651ab` yields 23 occurrences under the nine registered nouns, which are `commands`, `fields`, `flags`, `groups`, `kinds`, `outcomes`, `settings`, `tools`, and `verbs`. Every one of them is written below in the ledger's own grammar. This is the seed the implementer should expect to produce, and a seed differing by more than a line or two means the detection rule differs from the one above, which is reported rather than adjusted to match.

```
docs/quick-start.md:105  figure=fifty-one noun=commands counts=the commands dinah help lists derives=groupedCommands
docs/quick-start.md:105  figure=four noun=groups counts=the groups dinah help sorts its commands into derives=commandGroups
docs/quick-start.md:222  figure=four noun=settings counts=the settings dinah config accepts derives=configKeys
docs/quick-start.md:438  figure=five noun=commands counts=the verbs that change where a card stands derives=contractVerbs
docs/quick-start.md:439  figure=five noun=commands counts=the verbs that change where a card stands derives=contractVerbs
docs/quick-start.md:445  figure=three noun=commands counts=three of the reading commands holds=partitive reason=the sentence names ls, next, and show, which are three of the reading group rather than the whole of it
docs/quick-start.md:546  figure=five noun=commands counts=the verbs that change where a card stands derives=contractVerbs
docs/quick-start.md:561  figure=Five noun=commands counts=the verbs that change where a card stands derives=contractVerbs
docs/quick-start.md:562  figure=five noun=verbs counts=the verbs that change where a card stands derives=contractVerbs
docs/quick-start.md:756  figure=three noun=kinds counts=the checklist kinds a card may carry derives=checklistKinds
docs/quick-start.md:888  figure=one noun=fields counts=nothing, since the figure stands for the workstream the sentence above names holds=prose reason=the sentence reads Naming one reads its fields, so one is a pronoun and no set is being counted
docs/quick-start.md:1130 figure=two noun=groups counts=the two origins of a refusal name holds=prose reason=the groups are the shared rules and Dinah's own coinage, which no table in the tree declares as a set
docs/quick-start.md:1493 figure=four noun=flags counts=the optional flags dinah column new accepts derives=columnNewFlags
internal/guide/guides/first-session.md:99 figure=one noun=outcomes counts=one of the four outcomes a command reports holds=partitive reason=the sentence says one of four outcomes, so the figure counts the choice rather than the set
internal/guide/guides/mcp.md:298 figure=Eight noun=tools counts=the MCP tools whose verb reads the request's basis holds=unreachable at=internal/mcp/tools.go:305 declares=injectedProperties
internal/guide/guides/mcp.md:330 figure=two noun=kinds counts=the directory kinds the descent skips holds=prose reason=a dotted directory and a symbolic link are two branches of one walk rather than a set the tree declares
internal/guide/guides/query.md:44 figure=Twelve noun=fields counts=the field names the query language admits derives=queryFields
internal/guide/guides/query.md:80 figure=five noun=fields counts=the query fields read from a card's journal holds=unreachable at=internal/verb/query.go:72 declares=actPlane
internal/guide/guides/references.md:60 figure=Ten noun=commands counts=the commands that take a reference holds=elsewhere by=TestTheReferencesGuideSaysWhichCommandTakesWhat reason=assertTheGuideCountsItsOwnTable at cmd/dinah/main_test.go:7213 reads the rendered guide, counts the rows of the drawn table, and holds this sentence against that count
internal/guide/guides/verbs.md:1 figure=five noun=verbs counts=the verbs that change where a card stands derives=contractVerbs
internal/guide/guides/verbs.md:3 figure=Five noun=verbs counts=the verbs that change where a card stands derives=contractVerbs
internal/guide/guides/verbs.md:33 figure=two noun=commands counts=two of the five verbs holds=partitive reason=the two are claim and move, whose separate records a pull writes together
internal/guide/guides/verbs.md:37 figure=one noun=outcomes counts=one of the four outcomes a command reports holds=partitive reason=the sentence says one of four outcomes, so the figure counts the choice rather than the set
```

Three properties of that seed are worth naming, because they are what the rules above exist for. Seven entries share the phrase "the verbs that change where a card stands", and all seven carry the figure five and `derives=contractVerbs`, which is the consistency rule doing its work across two documents. The two "one of the four outcomes" entries share a phrase and a figure and are both partitive, so the rule holds over the escape hatch as well as over the derivations. Every one of the seven derivations in the table is named by at least one entry, so the derivation liveness rule passes.

The three pointers in the seed were each opened and read rather than cited. `internal/verb/query.go:72` carries `var actPlane = map[string]bool{` with the five field names beneath it, so the figure five is true today and the line carries the identifier the entry names. `internal/mcp/tools.go:305` carries `var injectedProperties = []injectedProperty{`, whose `basis` entry names eight tools across lines 308 to 310, so the figure Eight is true today; the variable is unexported and `cmd/dinah` cannot reach it, which is what puts the entry in this class rather than in a derivation. `TestTheReferencesGuideSaysWhichCommandTakesWhat` is declared at `cmd/dinah/main_test.go:7158` and calls `assertTheGuideCountsItsOwnTable` at 7213, which reads the guide's own drawn table and holds the sentence against the row count, so the one `holds=elsewhere` claim in the seed is a claim I checked by reading the test rather than by confirming the name resolves.

### The checks

Five tests stand in `cmd/dinah/prose_figure_test.go`, and each one names `cmd/dinah/testdata/prose-figures.txt` or the document in its message.

`TestEveryProseFigureIsDeclared` scans every document of the corpus outside its fenced blocks, finds every occurrence of a registered noun preceded within two words by a figure, and fails when the ledger carries no entry for that document, line, figure, and noun. The message names all four, and it says that the entry to write goes in `cmd/dinah/testdata/prose-figures.txt`.

`TestNoProseFigureEntryIsStale` fails on an entry whose document and line no longer carry the figure and the noun it names, so a moved sentence is repaired rather than left pointing at a line that has become something else.

`TestEveryDerivedProseFigureMatchesTheBinary` runs the derivation of every entry carrying `derives=` and fails naming the document, the line, the figure as written, the derivation, and the value the binary yields.

`TestEveryCountedSetIsCountedConsistently` groups the entries by their `counts=` phrase and fails on a group whose members disagree about the figure's value or about how the figure is held. The message names the phrase and every entry in the group, with its figure and its holding, so a reader sees which one moved. The same test refuses an entry whose `counts=` phrase is the literal `none`, naming the document and the line and telling the author to write a phrase saying what the figure stands for, because a shared sentinel groups unrelated figures together.

`TestEveryLedgerReferenceIsLive` fails on any of five dead references: a registered noun matching no occurrence anywhere in the corpus, an entry naming a derivation `derivationsByName` does not carry, a derivation `derivationsByName` carries that no entry names, a `by=` naming no test function declared in the module's test files, and an `at=` naming a file that does not exist or a line that does not carry the entry's `declares=` identifier as a word.

The scan reuses the fence arithmetic the tree already has. `quickStartMarkerRun` at `cmd/dinah/quickstart_test.go:190` is the one predicate both existing readings of a fence use, and the guide scan in `TestNoGuideCarriesATranscriptTheReplayDoesNotDrive` at line 120 shows the walk. A line inside a fenced block is not prose and is not scanned.

### What this replaces

`TestTheQuickStartCountsTheCommandsTheBinaryOffers` goes away in the same commit, and `numberWord` moves to `cmd/dinah/prose_figure_test.go` beside `numberFromWord`. The general mechanism holds the sentence that test holds, through the `derives=groupedCommands` entry at `docs/quick-start.md:105`, and it holds the other twenty-two occurrences the bespoke regular expression could never reach. Keeping both would put one fact behind two mechanisms, and the tree's one deliberate duplication, `bannedTypography` at `cmd/dinah/guide_guard_test.go:147`, is justified there by a package boundary that does not exist here.

`assertTheGuideCountsItsOwnTable` stays. It reads the references guide's drawn table rather than the binary, so it is not the same mechanism wearing a different name, and the ledger records the dependency through `holds=elsewhere by=TestTheReferencesGuideSaysWhichCommandTakesWhat`.

The removal is not free of risk, and the arming below is what pays for it: the same break that reddens the removed test must redden the ledger, and the arming runs the binary-side break dinah-446 used.

## Gap two: the guide block ledger

### The ledger

`cmd/dinah/testdata/guide-blocks.txt` declares every fenced block of every embedded guide, keyed by the guide's file and the one-based line its opening marker stands on:

```
internal/guide/guides/getting-started.md:9 shows=commands
internal/guide/guides/mcp.md:23 shows=json
internal/guide/guides/mcp.md:195 shows=shell reason=the line drives a shell loop around dinah path, which no replay here runs
internal/guide/guides/principles.md:29 shows=table command=columns
internal/guide/guides/workbench-layout.md:6 shows=tree reason=TestTheLayoutGuideDrawsPathsTheToolWrites holds every path this block draws
```

`shows=` takes one of five values, and each one carries its own check rather than standing as a comment.

`commands` means every non-blank line of the block is a command a reader types, which the check reads as beginning with `dinah ` once the line is trimmed. It fails on a line opening `$ `, with the message the retired rule gave authors, and it now fails on such a line wherever it stands in the block rather than only on the first. It fails on any other line too, because a block declared as commands that shows output is a transcript nothing drives, which is the defect class of this gap. The check does not read the command name, since `TestTheGuidesTeachOnlyDeclaredFlags` at `cmd/dinah/main_test.go:1767` already holds that ground. All five blocks of `getting-started.md` take this class, and every line of all five begins with `dinah `.

`json` means the block parses as JSON through `encoding/json`. Thirteen blocks take it, and a payload mangled by an edit becomes a failure rather than a thing a reader trips over.

`table` means the block draws a table the tool renders, and the entry names the command through `command=`. The check builds a workbench with `newBench`, runs the command through `runCLI` the way `exercisedWorkbench` at `cmd/dinah/guide_guard_test.go:844` does, and compares the block's header row against the rendered one. It compares nothing else. The data rows of such a block are invented for the lesson, and the separator row's widths follow the data, so comparing either would make the check fail on a lesson that chose a wider example. Comparing the header alone catches the class the audit found and cannot cry wolf.

`tree` and `shell` are the two shapes nothing here can drive, and both require a reason naming what does hold the block or why nothing can.

### The checks

Three tests stand in `cmd/dinah/guide_block_test.go`.

`TestEveryGuideBlockIsDeclared` walks every embedded guide's fenced blocks with `quickStartMarkerRun` and fails on a block the ledger does not name, giving the guide, the line, and the ledger's path. This is the check that sees an undriven transcript however its author spells it.

`TestNoGuideBlockEntryIsStale` fails on an entry naming a line that carries no opening fence, so an edit that moves a block is repaired rather than silently unheld.

`TestEveryGuideBlockShowsWhatItDeclares` runs the per-class check above for each entry and fails naming the guide, the line, and what disagreed. For the `table` class the message carries the block's header row and the tool's, on separate lines, so a reader sees which column moved.

### What this replaces

`TestNoGuideCarriesATranscriptTheReplayDoesNotDrive` goes away in the same commit. Its rule survives inside `shows=commands`, and its message to authors survives with it, but the rule now applies to a block an author has declared rather than to a shape an author was told to avoid. The instruction and the rule stop contradicting each other, because the author's obligation is now to declare the block rather than to spell it a particular way.

The `principles.md` table is what proves the gap is really closed. dinah-446 corrected that block by hand after the audit found it, and the `table` class is what would have found it, because the tool grew a `Work` column and the guide's header row did not. That heading is drawn from the catalog key `column.columns.work` in `internal/msg/locales/en.json`, which is what the arming below edits.

## Gap three: the replay stops feeding the document its own values

### What the `file` blocks are for

They are for two things, and I established both by reading the document around them rather than by inference. `docs/quick-start.md:318` introduces the block at 326 with "The block below is the file itself rather than a transcript", and the section teaches the reader to edit the frontmatter by hand. The blocks at 326 and 344 also put the sandbox into the state the transcripts after them show, since `dinah status` at line 366 prints the title "Release 0.2" and the wip limit `0/1`, and neither exists until the blocks land. The block at 724 creates `notes.txt` so that `dinah attach` at line 731 has a file to copy.

Both purposes survive this change. Nothing is deleted and no substitute fixture is needed.

### The change to `writeNarrativeFile`

`writeNarrativeFile` at `cmd/dinah/quickstart_test.go:1232` already reads the file standing in the sandbox and restores the per-run identifiers into the document's bytes through `restoreDocumentValues`. Three steps are added after that restoration and before the write.

The first step parses both the restored body and the standing file as frontmatter, meaning the block between the opening `---` line and the next one. A body that carries no frontmatter, which is `notes.txt`, skips the remaining steps, and so does a block whose target the sandbox does not carry.

The second step compares every key the two frontmatters share. A key whose values agree is silent. A key whose values disagree is a failure naming the document, the line inside the block, the document's value, and the file's value, unless `cmd/dinah/testdata/quickstart-file-blocks.txt` declares that key for that block under `teaches=`.

The third step writes the block's bytes with every shared key that is not declared under `teaches=` taking the standing file's value. The document can then no longer inject a stale value into the sandbox, so a later transcript echoes the tool rather than the document. A declared `teaches=` key still carries the document's value, and so does a key the standing file does not have, which is what makes the reader's edit take effect and keeps the seeding role intact.

The ordering matters and is part of the contract. The identifier restoration runs first, so the minted `columns:` list of `<workbench>/workbench.md` is reconciled before the comparison reads it and never appears as a divergence.

### The ledger

`cmd/dinah/testdata/quickstart-file-blocks.txt` carries one entry per `file` block, keyed by the one-based line of the block's opening fence:

```
326 teaches=title reason=the section tells the reader to give the workbench a real title
344 teaches=none because=every key this block writes either agrees with what dinah init wrote or is absent from it, so the block seeds the wip limit without overwriting anything the tool decided
724 unwritten reason=notes.txt is a file the reader creates, so the tool wrote nothing here to hold it against
```

The `teaches=` list of each entry is settled by the first run rather than by this spec, and the rules below are what keep it minimal. What the seed will contain is partly known. `dinah init` writes the workbench title from the directory's own name, which is `release-notes` in the sandbox, while the block at 326 writes `Release 0.2`, so `title` diverges and must be declared. `defaultDefinition` at `internal/verb/beyond.go:849` gives the `doing` column the title `Doing`, the slug `doing`, and the kind `work`, and `writeColumnFromMember` at `internal/bench/interchange.go:311` writes no `wip_limit` for a column with no capacity, so every key of the block at 344 either agrees or is unshared and the entry declares `teaches=none`.

Four rules give the file teeth, and they are the rules `quickstart-exempt.txt` already lives by.

An entry naming a key under `teaches=` whose values now agree fails, so a list cannot quietly accumulate keys that stopped diverging, and an entry declaring `teaches=none` needs a `because=`.

A block whose target the sandbox does not carry must be declared `unwritten` with a reason, so a `file` block that stops landing on a tool-written file becomes visible rather than becoming unheld.

An entry naming a fence line that carries no `file` block fails.

A `file` block with no entry fails.

The four rules above are evaluated for every `file` block the document carries, independently of the two skips in the comparison steps. A block whose body carries no frontmatter, and a block whose target the sandbox does not carry, are skipped by the comparison and are still held by the ledger, so the block at 724 must declare `unwritten` even though the comparison never reaches it. Without that independence the block would be governed by whichever step ran first, and the fifth arming in AC-17 would be ambiguous.

### What this replaces

`TestTheQuickStartWorkbenchFileDeclaresWhatTheBinaryWrites` at `cmd/dinah/quickstart_test.go:1745` goes away in the same commit, together with `checkAnchorBlockDeclaresTheBinarysValues` at line 1770. It holds two keys of one block by reading the document statically. The new rule holds every shared key of every `file` block against the file the binary actually wrote, which is a strictly wider statement, and the `unwritten` rule covers the one case the static check reached that a sandbox comparison otherwise would not, which is a block that lands where the tool wrote nothing.

Where this differs from dinah-446's shape, the reason is that dinah-446 read two constants out of `internal/bench` and compared them against the document. That works for two keys whose constants are exported and stable. It does not generalise, because most of what a workbench file carries is not a constant anywhere. Reading the file the binary just wrote is the general form of the same idea, and it needs no list of keys.

## What this card does not do

The six statements dinah-446 corrected are not touched. They are fixed and merged as `cf651ab`, and this card's subject is the reason they were invisible.

No guard is widened, no tolerance is loosened, and no exemption is granted to make a run go green. The seed ledgers are the exception that proves the rule and are bounded by it: a seed entry may carry `holds=partitive`, `holds=prose`, `holds=elsewhere`, `holds=unreachable`, a `shows=` class with a reason, a `teaches=` key, or `unwritten`, and no entry may declare that a figure sharing a `counts=` phrase with a derived figure is exempt from that derivation.

There is one case where correcting a document is permitted here, and it is the case where the new guard reports a figure that is false today. `docs/quick-start.md:1493` is that figure, and it is settled rather than predicted: the page draws five optional flags and the sentence says four. Such a figure is a defect this card's instrument found rather than one of dinah-446's six, and leaving it red would put a broken trunk behind a green card.

The correction reaches the sentence and the enumeration the figure counts, and no further. `docs/quick-start.md:1493` becomes five, and the paragraph beneath it gains `--tier` beside `--before`, `--kind`, `--slug`, and capacity, because a figure corrected over a list of four leaves a paragraph that is still false. The bound is the paragraph the figure stands in. A repair reaching past that paragraph is not this card's work, and the implementer files it as its own card rather than carrying it in a handoff, which is how the two defects dinah-446 could not absorb became dinah-452 and dinah-453. Nothing else in the corpus is corrected here, and a documented flag list that is wrong somewhere this guard cannot see belongs to dinah-447's class rather than to this one.

## Constraints

Nothing here may rest on undocumented behaviour, which is a standing rule on this repository. The three mechanisms read the tree's own files, the binary's own tables, and the output of `runCLI`, so nothing external is consulted.

The prose scan reads no line inside a fenced block, and it shares `quickStartMarkerRun` with the two existing readings so that a fence the document opens is a fence all three agree on.

Every new message names the file a reader must open and the line inside it, which is the shape every existing message in both guards uses.

The ledgers are plain text with the directive grammar `splitDirectives` reads, and no new parser is written where that one serves.

House style binds every word this card writes into the tree, including the ledger comments: no em-dash, complete sentences, one register per list, and the serial comma. `TestTheDocumentationCarriesNoBannedTypography` enforces the first of those over the documents already.

## Out of scope

The card does not add a check on the command names a guide writes, since `TestTheGuidesTeachOnlyDeclaredFlags` at `cmd/dinah/main_test.go:1767` and `TestTheReferencesGuideSaysWhichCommandTakesWhat` at line 7158 already read that ground.

The card does not extend the replay corpus to drive a guide's transcript end to end. The one rendered table in the guides shows invented data that no run reproduces, so the header comparison is what a real check can say about it, and a block that genuinely wanted driving would belong in the quick start.

The card does not export `actPlane` in order to derive the five journal fields, and it does not export `injectedProperties` in order to derive the eight basis-consuming tools. Both entries are declared `holds=unreachable` and name their declarations, and exporting either is a later card's work.

The card does not touch `docs/design/token-cost.md` or its convention.

## How this is verified

Every acceptance criterion on this card names the command that produces its verdict, and every criterion that arms a check names what to break, the message to expect, and the restoration. The package under test is `cmd/dinah`, so the verdict command is a `go test` invocation naming the test by `-run`. Several breaks redden more than the check being armed, because the ledger's rules overlap by design, so every arming scopes its run to the one check it is arming and the criterion says which other rules the same break would also trip.

Nine checks are new on this card and every one of them is armed, which D-3 requires and which round 2 missed by one: `TestNoGuideBlockEntryIsStale` carried a green verdict and no break. AC-20 is its arming.

Two criteria carry the proof that the third gap was really open rather than argued open. The break they describe, setting `format: 2` back to `format: 1` inside the `file` block that opens at `docs/quick-start.md:326`, is run once on the tree as it stands before this card's change, where it must pass, and once after, where it must fail. A guard that catches a break the previous tree also caught has closed nothing. The pre-tree half is expected to pass, because `readBench` at `internal/bench/bench.go:1562` refuses only a format above the binary's own and the document carried `format: 1` for four profile revisions without reddening. Should the pre-tree half fail anyway, the failure is recorded and the arming is retried against `profile:`, which is the other tool-owned key of that block, and the criterion is met by whichever key the previous tree accepted.

## Branch

dinah-448-documentation-guards-name-their-own-blind-spots
