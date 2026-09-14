---
title: Six documented statements are false, and two of them ship inside the binary
column: b69abf918c42
state: ready
severity: major
priority: now
tier: frontier
workstreams:
  - c51696dd44b0
---
## What this card fixes

A documentation audit on 2026-09-08, run against a binary built at `cca531b` rather than by reading, found six statements a reader would act on that are no longer true. Each is verifiable by running a command, and each is contradicted by another document in the same tree, which is how they were caught.

The audit report is the parent session's; every finding below names how it was established so the implementer can reproduce it rather than take it.

## The six

**1. `docs/quick-start.md`, around lines 1549 to 1553.** The MCP paragraph says the client gets "twenty-one tools against its twenty-nine commands", that eight commands are missing, and names them as `init`, `config`, `path`, `edit`, `extract`, `workbenches`, `mcp` and `guide`. A `tools/list` dump over `dinah mcp` returns 43 tools and `dinah help` lists 51 commands. The count of eight missing is still right and the membership is not: `workbenches` is a tool now, and the eighth shell-only command is `reshape`. The real set is `config`, `edit`, `extract`, `init`, `mcp`, `path`, `reshape` and `guide`. The same document says fifty-one commands at line 105, so it contradicts itself.

**2. `internal/guide/guides/first-session.md`, line 109.** It says "Dinah does not accept `--help`." It does. `dinah show --help` prints the command's help page and exits 0, and `dinah help` lists `--help`, `-h` and `-?` among the global flags. The spellings landed in `e607816` (dinah-213) after the guide was written, and `docs/quick-start.md:108` says the opposite. **This one ships inside the binary and tells an agent not to try something that works**, which is why it is on this card rather than a tidier one.

**3. `editors/vscode/README.md`, lines 51 to 54.** It says every archive this repository publishes carries the pre-release mark so VS Code offers it only to people who opted in. dinah-405 reversed that. `editors/vscode/scripts/verify-package.mjs` now asserts the archive carries no pre-release property and names dinah-405 and the 1.0.0 release in its header comment, and `.github/workflows/vscode-release.yml:272` publishes without `--pre-release`. **This is marketplace-facing text stating the reverse of what ships.**

**4. `docs/quick-start.md`, lines 327 to 329, and line 1412 with it.** The `file` block shows `format: 1` and `profile: dinah-core/0.9`. A fresh `dinah init` writes `format: 2` and `profile: dinah-core/0.12`, which is what the document's own `dinah version` transcripts print at lines 94 and 1221.

**Read this before touching it.** `replayQuickStart` in `cmd/dinah/quickstart_test.go:1214` writes `file` blocks into the sandbox before the next command runs. The document injects its own stale frontmatter, and the `dinah export` transcript at line 1412 then correctly echoes `"profile": "dinah-core/0.9"` back. The guard passes because the document agrees with itself, not because it agrees with the tool. **Fixing 329 without 1412 will turn the suite red, and fixing 1412 without 329 will too.** The two edits are one edit.

**5. `internal/guide/guides/principles.md`, lines 30 to 34.** The `dinah columns` transcript shows `Slug  Name  Kind  Cards  Owner`. The tool prints `Slug  Name  Kind  Cards  Work  Owner`, with values like `none taken` and `taken`. This is the only transcript in the guides and it is stale.

**6. `internal/guide/guides/query.md`, line 83.** It says "any of the nine fields that take `:`" and should say eleven. The same page correctly says twelve fields two paragraphs earlier and correctly says `at` is the only one taking comparisons, so nine is impossible against its own text. Nine is the count from before severity and priority were added.

## Three minor items, worth taking while in the file

`internal/guide/guides/mcp.md:28` shows `"profile": "dinah-core/0.7"` in an illustrative response. Line 95 of the same guide says `show` "returns one card in full, its body, its links, and its comments", which is three of the six members its own `fields` section lists forty lines later.

`docs/design/surfaces.md`, lines 20 to 22, states in the present tense that the binary's heads include an HTTP server (`dinah serve`) and an LSP (`dinah lsp`). Neither exists. The document's opening caveat covers it loosely, so judge whether to correct the tense or leave it.

`docs/quick-start.md` never shows `show --fields`, which dinah-383 shipped, though the document claims to walk every command.

## The rule that governs how these get fixed

**Do not repair a stale number by transcribing a fresh one.** Every count in this card moves: the catalog reads 896 today and read 838 four days ago, the tool count reads 43 and will not stay there, and trunk broke on 2026-09-07 because two cards each wrote a correct count that was wrong the moment the other landed. Where a figure is load-bearing, prefer a form that cannot go stale, which usually means deriving it, quoting a transcript a guard replays, or rewriting the sentence so it does not need the number. Where a number genuinely must be written, say what produces it.

That rule is why finding 1 is not simply an arithmetic fix. A paragraph naming a tool count, a command count and a membership list is three hostages to fortune in one sentence, and the honest repair may be to stop stating two of them.

## Scope

Documentation and guide text only. No behaviour changes, no test changes beyond what the quick-start guard forces in finding 4, and no new counts written into prose that nothing checks.

The systemic problem, that findings 1, 5 and 6 all sit in gaps the guards name in their own comments as blind spots, is a separate card and must not be absorbed here.

## Specification

## What this card lands

Six false statements are corrected, plus two in `internal/guide/guides/mcp.md`
that are the same class of defect in the same file. One guard is added, and it
is added because the card's own no-stale-number rule leaves finding 4 no other
honest repair. Nothing else in the test tree changes, and no shipped behaviour
changes.

Every location below was re-established against trunk at `178e264`. Where a
line number moved since the filing at `cca531b`, this section says so.

## Locations, re-established

| Finding | File | Line at 178e264 | Filed as | Moved? |
| --- | --- | --- | --- | --- |
| 1 | `docs/quick-start.md` | 1548 to 1553 | 1549 to 1553 | the sentence starts one line earlier than filed |
| 2 | `internal/guide/guides/first-session.md` | 109 | 109 | no |
| 3 | `editors/vscode/README.md` | 52 to 54 | 51 to 54 | the false clause starts mid-line at 52 |
| 4 | `docs/quick-start.md` | 328, 329, and 1412 | 327 to 329, 1412 | no |
| 5 | `internal/guide/guides/principles.md` | 30 | 30 to 34 | the stale line is 30 alone |
| 6 | `internal/guide/guides/query.md` | 83 | 83 | no |

What the tool actually reports, derived from source rather than from a run:

- `internal/bench/bench.go:104-106` gives `ProfileName = "dinah-core"`,
  `ProfileMajor = 0`, and `ProfileMinor = 12`, so `bench.ProfileVersion` is
  `dinah-core/0.12`.
- `internal/bench/bench.go:76` gives `StorageFormat = 2`.
- `cmd/dinah/commands.go:38` declares 51 commands carrying a `group`, plus
  `help`, which carries none and which `dinah help` does not list. The fixture
  `cmd/dinah/testdata/help.txt` holds 51 listed lines.
- `internal/mcp/tools.go:66` declares 43 tools.
- 51 minus 43 is 8, and the eight grouped commands with no tool are `config`,
  `edit`, `extract`, `guide`, `init`, `mcp`, `path`, and `reshape`. The card's
  membership claim is confirmed by derivation, and `workbenches` is a tool
  (`internal/mcp/tools.go`, last entry of the table). Those eight are the keys
  of `toolExemptions` (`internal/mcp/tools.go:123`) minus `help`, which the
  head exempts as well and which `dinah help` does not list as a command.
- `cmd/dinah/args.go:109-114` maps `--help`, `-help`, `-h`, `-?`, `--?`, and
  `/?` to the help command, so all six spellings are accepted.

## How each repair satisfies AC-1

AC-1 forbids repairing a stale figure by transcribing a fresh one. Each fix
below names which of its three permitted forms it takes.

- Finding 1 takes the third form. The sentence is rewritten so it states no
  count at all, and so it states no membership either.
- Finding 2 takes the third form. The false sentence is deleted and the
  replacement states a capability rather than a number.
- Finding 3 takes the third form. The reversed clause is rewritten to state
  what ships, and it names no version.
- Finding 4 takes the first form. A new guard derives the two values from
  `internal/bench` and holds the document to them at read time.
- Finding 5 takes the second form. The stale table is brought into agreement
  with a fenced block the quick-start replay drives.
- Finding 6 takes the third form. The count is removed and nothing replaces it.

AC-1 governs figures. It does not govern a false statement that carries no
figure, which is how the previous revision of this spec came to prescribe a
paraphrase of the shell-only commands that was itself untrue. The section below
states the standing rule that closes that hole, and AC-9 and AC-10 are the two
criteria added to hold it.

## Do not replace a false statement with an unchecked one

The card's rule is that no stale figure is repaired by transcribing a fresh one.
Its spirit reaches further, because a paraphrase of a set is a membership claim
written in prose and it rots the same way a count does. Every sentence this card
writes about how the binary behaves is therefore either checked by a criterion
that names the command returning its verdict, or it is not written. The table
under "Every behavioural claim in this spec" below is the whole inventory, and a
reviewer who finds a behavioural claim in this spec that is missing from that
table has found the defect this section exists to prevent.

## Finding 1: the MCP paragraph in the quick start

`docs/quick-start.md:1548-1553` currently reads as follows. The span ends
mid-sentence at "Your AI" because line 1554 continues it with "colleague claims,
moves, releases, and blocks under the same rules", so a replacement has to end
on those two words or orphan the rest of that sentence:

```
workbench outside that root is then refused. Dinah hands the client the rules
for working this workbench and twenty-one tools against its twenty-nine
commands. Every command that files, moves, or reads a card is there. Seven of
the eight that are missing only make sense at a shell: `init`, `config`, `path`,
`edit`, `extract`, `workbenches`, and `mcp` itself. The eighth is `guide`, and
the client reads it as a resource rather than calling it as a tool. Your AI
```

Three figures are asserted there, and the ruling is that the document stops
stating all three.

The tool count and the command count go because neither earns its place. The
command count is already stated at line 105, where
`TestTheQuickStartCountsTheCommandsTheBinaryOffers`
(`cmd/dinah/quickstart_test.go:1693`) derives it from `commands` and fails the
build when it drifts. Stating it a second time 1444 lines later puts a second
copy beyond that guard's reach, and the guard's regexp
`lists all ([a-z-]+) commands` will not match the second copy however it is
worded. The tool count is held by nothing at all, and it is a number the reader
never acts on: nobody counts the tools their client offers before using one.

The membership list goes for the same reason one layer down. Eight names is
eight hostages, `workbenches` has already escaped once, and holding the set
would need a guard this card is not the place to write, since a guard that
should exist and does not is dinah-448's subject.

**A paraphrase of the set is the membership list again, and it is not
permitted.** The previous revision of this spec proposed describing the
left-out commands as the ones that create a workbench, print a path, open an
editor, copy a definition out, read your settings, or start the server. That
paraphrase was already false of `reshape` on the day it was written.
`readReshapeSource` (`internal/verb/reshape.go:336`) reads a definition either
from an interchange file or by exporting another workbench's directory, and
`planReshape` (line 366) then rewrites this workbench's columns from it. The
command's own summary is `cmd.reshape.summary` in
`internal/msg/locales/en.json`, "Carry this workbench to the column layout a
new definition declares", and `param.reshape.from.summary` describes `--from`
as "the definition to take the new shape from". `reshape` takes a definition
in and changes this workbench. It does not copy one out, and it is not
`extract` wearing a different name.

**The inclusion sentence goes too, because it has counterexamples in the set it
is meant to exclude.** "Every command that files, moves, or reads a card is
there" is contradicted by `path`, which prints a card's file path, and by
`edit`, which opens a card in the reader's editor. Neither has a tool. The set
is defined by what is left out and why, so the paragraph needs no companion rule
about what is present.

**What the paragraph states instead is the rule the head states about itself.**
`internal/mcp/tools.go:49-52` says a command that exists only because a shell
and a filesystem exist gets no tool, and that `toolExemptions` (line 123) is
where a reader goes for the current set with each command's reason.
`TestEveryLibraryCommandIsServedOrExempted` (`internal/mcp/roster_test.go:21`)
holds that map to the verb table in both directions, so the rule cannot drift
away from the code without turning the build red. `guide` is the one exclusion
the rule does not explain, and the document already explains it.

Replace lines 1548 to 1553 with prose to this effect, wrapped to the
paragraph's existing width of roughly 79 columns:

> workbench outside that root is then refused. Dinah hands the client the rules
> for working this workbench, and a tool for every command that has a use over
> a protocol. The ones it leaves out are the ones that only make sense where a
> shell and a filesystem are, and `guide`, which the client reads as a resource
> rather than calling it as a tool. `tools/list` names the set your client
> actually got. Your AI

The replacement names exactly one command, and it names `guide` because the
sentence is explaining why `guide` is the exception rather than telling the
reader what `guide` is for. Do not restore a count, do not name any other
command, do not paraphrase what the left-out commands do, and do not close the
paragraph with a sentence summarising what it has just said. AC-9 returns the
verdict on all four of those.

This paragraph sits at line 1548, below the highest entry in
`cmd/dinah/testdata/quickstart-exempt.txt`, which is 1367, so a change of
length here shifts no fence any fixture names. That is the only quick-start
edit in this card free to change line count.

## Finding 2: the `--help` claim inside the binary

`internal/guide/guides/first-session.md:109` opens with `Dinah does not accept
`--help`.` Delete that sentence. Replace the paragraph with prose saying that
`dinah help` lists the commands, that `dinah help <command>` gives one
command's arguments, what can go wrong with it in the order each thing is
checked, and its exit codes, and that `dinah <command> --help` prints the same
page.

Two constraints bind the replacement. `TestNoSentenceStandsInTwoGuides`
(`cmd/dinah/guide_guard_test.go:1024`) fails if a sentence of eight or more
words lands in two embedded guides, so the wording must not be lifted from
`getting-started.md` or `mcp.md`. `docs/quick-start.md` is outside that
corpus, so agreeing with lines 107 to 113 of the quick start is safe and is in
fact the point. Do not enumerate the six accepted spellings here; the quick
start already does, and repeating them creates a second copy to keep true.

## Finding 3: the pre-release claim in the extension README

`editors/vscode/README.md:52-54` currently ends the paragraph with:

```
Every archive this
repository publishes carries that mark, so VS Code offers it only to people who
have opted into pre-releases.
```

That is the reverse of what ships. `editors/vscode/scripts/verify-package.mjs`
declares `PRE_RELEASE_PROPERTY` at line 59 and asserts the packaged archive does
not carry it, and its header comment at lines 10 to 12 names dinah-405 and the
1.0.0 release as the reason. `.github/workflows/vscode-release.yml:272` runs
`vsce publish --skip-duplicate --packagePath vsix/dinah-universal.vsix` with no
`--pre-release`.

Replace those three lines with prose saying that no archive this repository
publishes carries the mark, so every published version installs as a release
and needs no opt-in. Keep the two sentences before it, which say that the
marketplace listing is where a pre-release would be marked, because they remain
true and they are what makes the correction readable. Name no version number in
the replacement.

## Finding 4: the stale frontmatter, and the guard it forces

`docs/quick-start.md:328-329` sits inside the block opening at line 326 with
`` ```file path=<workbench>/workbench.md ``. It declares `format: 1` and
`profile: dinah-core/0.9`. A fresh `dinah init` writes `format: 2` and
`profile: dinah-core/0.12`, which is what the constants above give and what the
document's own driven `dinah version` transcripts print at lines 94 and 1221.

The coupling is real, and it runs through the replay rather than through the
prose. `replayQuickStart` (`cmd/dinah/quickstart_test.go:1200`) dispatches a
`file` block to `writeNarrativeFile` (line 1232), which writes the block's bytes
over the sandbox's `workbench.md` before the next command runs. Only the classes
in `normalisationTable` (line 468) are restored from the standing file, and that
table carries release, revision, timestamp, path, and identifier, and nothing
else. A profile revision is therefore written through verbatim. The block at
fence 1385 then runs `dinah export`, which is a driven block and not exempt, and
its output at line 1412 reads `"profile": "dinah-core/0.9",` because the
document told the workbench to say so.

Sequence the edit as one change, in one commit, and verify it once:

1. Set `docs/quick-start.md:328` to `format: 2`.
2. Set `docs/quick-start.md:329` to `profile: dinah-core/0.12`.
3. Set `docs/quick-start.md:1412` to `  "profile": "dinah-core/0.12",`,
   preserving the two leading spaces and the trailing comma.
4. Run the replay and confirm it is green before touching anything else.

All three edits are line-count neutral and must stay so. Lines 328, 329, and
1412 all sit below fence entries that `quickstart-exempt.txt` names by number,
so inserting or removing a line here silently invalidates the fixture.

Neither key may be dropped from the file block instead of corrected.
`internal/bench/bench.go:1546` refuses a workbench whose anchor declares no
`profile` with `contract.Malformed`, and `declaredFormat`
(`internal/bench/container.go:622`) reads an anchor declaring no `format` as
not contained, so an elided pair changes what the narrative's own workbench is
rather than only what the page shows.

That closes AC-1's second and third forms for this finding. The transcript at
1412 is not an independent witness, because the value it prints is the value the
document supplied. Correcting the pair by hand leaves exactly the loop that let
`0.9` survive four profile revisions. So the first form is the only one left,
and this card adds the guard that supplies it.

**The new guard.** Add one test to `cmd/dinah/quickstart_test.go`, beside
`TestTheQuickStartCountsTheCommandsTheBinaryOffers`, which is its closest
sibling in shape. Name it
`TestTheQuickStartWorkbenchFileDeclaresWhatTheBinaryWrites`. It does this:

- Parse the document with `parseQuickStart(readQuickStart(t))`, which both
  already exist at lines 219 and 206.
- Select the blocks where `block.kind == "file"` and
  `block.directive("path")` returns `<workbench>/workbench.md`.
- Fail with `t.Fatal` when no such block is found, so the guard reports that it
  read nothing rather than passing vacuously. This workbench has had three
  tests that could not fail, and a selector that silently matches nothing is
  how a fourth arrives.
- For each selected block, read the `format:` and `profile:` lines out of
  `block.body` and compare them against `strconv.Itoa(bench.StorageFormat)` and
  `bench.ProfileVersion`. Import `dinah/internal/bench`, which
  `cmd/dinah/advice_test.go:15` already does, so the dependency is not new.
- Fail when a selected block declares neither key, since a workbench anchor
  that declares neither is not the shape the narrative is teaching.
- Word each failure so it names the document, the line, the value found, and
  the value the binary writes.

Arm it before handing off. Set `ProfileMinor` to `11`, run the test, confirm it
reports the mismatch, restore `internal/bench/bench.go` from a byte-identical
copy, and confirm it goes green again. Say in the handoff what the red run
printed. Do the same for `StorageFormat`. Break the constant rather than the
assertion, and check that the plant compiled and the test actually executed,
because a plant that fails to build prints nothing and nothing looks exactly
like a pass.

The guard belongs on this card and not on dinah-448 because AC-1 requires it
for this fix to be legal, which is what the card means by "no test changes
beyond what finding 4 forces". Do not widen it to cover the guide corpus, the
`$ ` selection rule, or any other blind spot; those are dinah-448's.

## Finding 5: the stale `dinah columns` table in a guide

`internal/guide/guides/principles.md:30` reads:

```
  Slug    Name    Kind    Cards  Owner
```

The tool prints a `Work` column between `Cards` and `Owner`. The authority for
its shape is not a fresh run and is not this spec. It is
`docs/quick-start.md:367-371`, which sits inside the console fence opening at
line 359, is driven by the replay, is absent from
`cmd/dinah/testdata/quickstart-exempt.txt`, carries a `wip_limit` exactly as the
guide's example does, and reads:

```
  Slug    Name    Kind    Cards  Work        Owner
  ------  ------  ------  -----  ----------  -----
  intake  Intake  intake  0      none taken  agent
  doing   Doing   work    0/1    taken       agent
  done    Done    done    0      none taken  agent
```

That block is the output of `dinah status` rather than of `dinah columns`, and
it is authoritative for the guide's `dinah columns` listing anyway, because both
heads print the same table through the same function: `renderColumns`
(`cmd/dinah/render.go:256`) is called by the `columns` command at
`cmd/dinah/commands.go:550` and by `renderStatus` at `cmd/dinah/render.go:204`.
AC-10 holds that claim to a run of `dinah columns` itself rather than leaving it
resting on this paragraph.

Bring the guide's block into that shape, keeping the guide's own narrative
counts, which are `1`, `2/2`, and `0` and which belong to the story the
paragraph is telling rather than to any run. The values under `Work` are
`none taken` for the intake and done columns and `taken` for the work column.
`renderColumns` (`cmd/dinah/render.go:275-281`) selects `columns.work.waiting`
for a column awaiting somebody outside, `columns.work.none` for a column that
takes no work up, and `columns.work.taken` otherwise, and those keys carry
`waiting`, `none taken`, and `taken` in `internal/msg/locales/en.json`. None of
the guide's three columns awaits anybody outside. Widen the separator row to
match; the `checkSeparatorRowsMatchTheirTables` rule
(`cmd/dinah/quickstart_test.go:1016`) governs the quick start rather than the
guides, but a separator that does not match its heading is wrong on its own
terms.

Do not add a `$ dinah columns` line above the block.
`TestNoGuideCarriesATranscriptTheReplayDoesNotDrive`
(`cmd/dinah/guide_guard_test.go:117`) fails any guide block whose first body
line opens `$ `, and the block is fine as a listing. That the guard cannot see
the block's staleness is dinah-448's finding and is not repaired here. Nothing
will hold this table to the tool after this card lands: its shape is true on the
day it is written because AC-6 and AC-10 both check it at merge, and the guard
that would keep it true afterwards belongs to dinah-448.

## Finding 6: the field count in the query guide

`internal/guide/guides/query.md:83` reads "any of the nine fields that take
`:`". Eleven is correct today, since the page lists twelve fields at lines 45 to
72 and names `at` as the only one taking comparisons. Write neither number.
Rewrite the clause so it needs none, for example "any field that takes `:`".
The sentence loses nothing, because the reader has just been given the list and
the one exception.

Leave the "listing the twelve there are" clause further down the page alone. It
is correct at `178e264`, this card does not correct it, and AC-1 binds figures
this card's fix introduces or corrects.

## The two extra items taken, and the two left

Two of the four items the filing flagged as minor are taken here, and two are
not. The rulings are recorded as decisions on the card.

**Taken: `internal/guide/guides/mcp.md:28`.** The illustrative response shows
`"profile": "dinah-core/0.7"`, which is a revision this binary never emits and
which sits at the compatibility floor (`ProfileFloorMinor = 7`). It is one line,
it ships inside the binary, and it is the same defect as finding 4. Do not
replace it with `0.12`, which would arm the same trap. Elide the value, so the
`status` member reads something of the shape
`"status": { "workbench": "Your workbench", "profile": "...", ... }`, matching
the ellipsis the same line already uses for the members it omits. Keep the line
valid JSON inside the escaped string.

**Taken: `internal/guide/guides/mcp.md:95`.** It says `show` "returns one card in
full, its body, its links, and its comments", which names three of the six
members the same guide lists at lines 152 to 153, where the six are `card`,
`body`, `links`, `attachments`, `comments`, and `path`. Rewrite so the sentence
enumerates nothing, for example that `show` returns one card and every member of
it, that the section below names the members and how to ask for fewer, and that
it is the call to make before acting on a card you have not already met. A
second enumeration forty lines from the first is what went stale, so do not
write a corrected list of six in its place.

**Not taken: `docs/design/surfaces.md:19-21`.** Correcting the present tense on
`dinah serve` and `dinah lsp` requires ruling whether those heads are still
planned or have been abandoned, and that is a design decision rather than a
transcription error. The document's own caveat at lines 12 to 14 says the verb
set is still under design and is recorded as it settles, which is the frame a
reader arrives with. Leaving it costs nothing this card is measuring.

**Not taken: `show --fields` in the quick start.** The document's claim at line
4 is that a reader meets every command, and `show` is met. `--fields` is a flag
rather than a command, so no statement in the document is false. This is an
omission, and omissions are dinah-447's subject.

## Every behavioural claim in this spec, and what returns its verdict

Each row is a statement this spec makes about how the binary behaves, together
with the criterion whose named command decides it. A claim this card writes into
a document and that appears in no row is a defect of the same kind the card
exists to fix.

| Claim | Where the spec uses it | Criterion |
| --- | --- | --- |
| The head serves a tool for every command that has a use over a protocol, and leaves out the ones that need a shell or a filesystem, plus `guide`. | the finding 1 replacement prose | AC-9 |
| `dinah <command> --help` prints that command's help page and exits 0. | the finding 2 replacement prose | AC-4 |
| No archive this repository publishes carries the pre-release mark. | the finding 3 replacement prose | AC-5 |
| A fresh `dinah init` writes `format: 2` and `profile: dinah-core/0.12`, and the replay writes a `file` block through verbatim, so the export transcript echoes it. | the finding 4 edit and its guard | AC-2 |
| `dinah columns` prints `Slug Name Kind Cards Work Owner`, with `none taken` on an intake or done column and `taken` on a work column, and `dinah status` prints the same table. | the finding 5 replacement table | AC-10 |
| The corrected guide text is what the shipped binary prints. | findings 5 and 6 | AC-6 |
| No fence the exempt fixture names by line has moved. | the whole card | AC-8 |

## The fixture that keys on line numbers

`cmd/dinah/testdata/quickstart-exempt.txt` names blocks of `docs/quick-start.md`
by the one-based line their fence opens at. The standing entries are 24, 187,
209, 890, 978, 1004, 1118, 1337, and 1367. Nothing announces a drift: an entry
whose number lands on a line that is not a fence fails, and an entry whose
number lands on a fence the replay drives fails, but an entry that happens to
land on another exempt fence would pass while naming the wrong block.

This card is safe as specified. The edits at 328, 329, and 1412 are
value-for-value and change no line count, and the rewrite at 1548 is below every
entry in the fixture. Should any quick-start edit end up changing line count at
or above 1367, every entry below the edit is renumbered by hand in the same
commit and the whole `cmd/dinah` suite is rerun.

The two fixtures dinah-234 rekeyed, the call-site registry in
`cmd/dinah/row_sweep_test.go` and the allowlist in
`cmd/dinah/testdata/uncovered.txt`, now name a place by its enclosing function
and its rank within that function. Adding a test function does not disturb
either, since neither indexes test sources by line. Run the whole suite anyway.

## Out of scope

No shipped behaviour changes, and no file under `internal/verb`,
`internal/bench`, `internal/mcp`, or `internal/msg` is edited except as a
temporary armed break that is restored before the commit. The only test file
this card touches is `cmd/dinah/quickstart_test.go`, and the only change there
is the added guard. Do not repair the guide guard's `$ ` selection rule
(dinah-448). Do not add anything the documentation currently omits
(dinah-447). Do not run `scripts/verb_selection.py`.

## Verification

Run these from the card's worktree.

```
go test ./cmd/dinah/ -run 'QuickStart|Quick'
go test ./internal/mcp/ -run TestEveryLibraryCommandIsServedOrExempted -v
go test ./cmd/dinah/ ./internal/guide/ ./internal/mcp/ ./internal/bench/
go test ./...
node editors/vscode/scripts/verify-package.mjs   # after packaging, see AC-5
```

The document-versus-tool comparisons that decide the prose findings are these,
each of which prints the tool's answer beside the sentence to be read:

```
go build -o C:/dinah-scratch/dinah-446-impl/dinah ./cmd/dinah
cd C:/dinah-scratch/dinah-446-impl && DINAH_HOME=C:/dinah-scratch/dinah-446-impl/home ./dinah show --help
cd C:/dinah-scratch/dinah-446-impl && DINAH_HOME=C:/dinah-scratch/dinah-446-impl/home ./dinah guide first-session
cd C:/dinah-scratch/dinah-446-impl && DINAH_HOME=C:/dinah-scratch/dinah-446-impl/home ./dinah guide principles
cd C:/dinah-scratch/dinah-446-impl && DINAH_HOME=C:/dinah-scratch/dinah-446-impl/home ./dinah guide query
cd C:/dinah-scratch/dinah-446-impl && DINAH_HOME=C:/dinah-scratch/dinah-446-impl/home ./dinah guide mcp
cd C:/dinah-scratch/dinah-446-impl/scratch && DINAH_HOME=C:/dinah-scratch/dinah-446-impl/home ../dinah init && ../dinah columns
```

Build and run the binary only from a directory under `C:\dinah-scratch\`, and
point `DINAH_HOME` inside it. Workbench discovery climbs to the drive root, so
a run from the operator's checkout or from anywhere under his profile reaches
his live data.

## Branch

dinah-446-six-documented-statements-are-false
