---
title: The guide says each command's help repeats its answer, and for three commands the help says something narrower
column: b69abf918c42
state: ready
severity: major
priority: soon
workstreams:
  - 994787601ae6
---
The references guide tells a reader that each command's own help page carries the same answer about what that command accepts, so they can run `dinah help attach` and get it beside the arguments. For three commands that is false: their help pages describe a narrower set of things than the guide's table grants them.

The sentence was already wrong at trunk before dinah-457, and that card made it wrong in one additional way while deliberately not touching help text, since its scope was the guide and a test holding the guide's table to the code. It was found by the Test stage on that card, which built the binary and checked all seventy-five promises the table makes by running each command against each kind.

This matters more than a doc slip because of what the guide now is. dinah-456 settled that the references guide states the whole address contract, and dinah-457 made its table derived from the code so a command cannot drift out of it. Help text is the other surface a reader reaches for the same question, and nothing holds the two together. So the tool now has one authoritative answer and one unguarded answer, and points readers at both.

What the card should settle, none of it the operator's to rule on:

Which of the two is corrected. The three help pages may be understating what their commands accept, in which case they are simply wrong and get widened; or the guide's table may be granting something the command does not really do, in which case the defect is in the derivation and is far more serious. Establish which by running each of the three rather than by reading, and say so. Do not assume the newer document is the right one.

Whether the two surfaces are held together by a test, and if so which direction it runs in. A one-directional check is worth nothing here, which this workstream has now demonstrated three separate times. Note that dinah-457 already derives the guide's table from the library's own roster, so there is a working shape to copy rather than a mechanism to invent.

Whether the guide should keep making that promise at all. Telling a reader that two documents agree, when nothing enforces it, is a promise the project cannot keep by intention alone. Removing the sentence is a legitimate answer if the alternative costs more than it is worth; if it stays, it needs a guard behind it.

Which three commands. The Test stage on dinah-457 found them and named them in its evidence comment; read that rather than re-deriving from scratch, then verify by running.

Related: dinah-456 is the addressing contract, dinah-457 is the guide and its derived table, and dinah-444 is the standing card for text that tells a reader something the code does not do.

## Specification

The references guide promises that a command's own help page answers the addressing
question the guide answers. Five help pages answer it more narrowly than the guide's
table does, and eight more say nothing about the workstream the guide's own sentence
grants them. This card makes the promise true by construction: one declaration in
`internal/verb` carries which address kinds each command's reference may name, the
guide's table and the guide's workstream sentence are both held to that declaration,
and both heads render the declared kinds beside the argument instead of restating them
in prose that nothing holds.

Round 3 works against `origin/main` at
`ab5debc5fc3af39f2d079960ceb59ee5bc2d8bcd` ("dinah-477: turn a column's hold on and
off from the command line"), one commit ahead of the
`808d105c21516d59a9d73e7d8d8f7ae369c37f6c` that rounds 1 and 2 and both reviews read.
That commit moves nothing this card counts: the roster is eighteen commands at both,
the guide's table is eighteen rows at both, and eighteen parameters name the
references guide at both, each derived by the commands in section 0. Runs happened in
`C:/dinah-scratch/dinah-470-spec3`, against a throwaway workbench with `DINAH_HOME`
pointed inside the same directory, a fresh copy of the fixture per invocation, and
`DINAH_EDITOR` pointed at a logging shim proven to fire. Where a claim below came from
a run, the section gives the invocation and what came back.

# 0. Every figure in this spec, and the command that produces it

Three hand-counted figures in earlier rounds of this card came back wrong on review,
each of them supporting an argument that was itself sound. So no figure below is
counted by hand. Each one names the command that derives it, run at `ab5debc`, and a
reader who doubts a figure re-runs its command rather than trusting this document.

| Figure | Value | Derivation |
|---|---|---|
| commands taking a reference | 18 | `sed -n '/^var guides = map/,/^}/p' internal/verb/definition.go \| grep -c '"references"'` |
| parameters naming the references guide | 18 | `grep -cE 'Guide: *(referencesGuide\|"references")' internal/verb/definition.go` |
| rows in the guide's table | 18 | `sed -n '/^\| Command /,/^$/p' internal/guide/guides/references.md \| grep -c '^\| [a-z]'` |
| cells in the guide's table | 90 | eighteen rows against the table's five kind columns |
| message catalogues | 8 | `ls internal/msg/locales/*.json \| wc -l` |
| of section 4.2's ten sentences, those carrying `"; "` today | 5, in every catalogue | the Python in section 4.2 |
| references guide figures pinned by the prose ledger | 4 | `grep -c '^internal/guide/guides/references.md:' cmd/dinah/testdata/prose-figures.txt` |
| commands taking a workstream | 9 | the probe table in section 1.3 |
| command summaries that enumerate address kinds | 6 | the listing in section 4.3 |

One warning for whoever re-runs the second of those. Grepping for the literal
`Guide: "references"` alone answers 16, because `get` and `set` write the constant
`referencesGuide` instead, and the two spellings are one fact. That is the shape the
workbench instructions name, where searching a file for a phrase finds copies of the
phrase rather than copies of the claim.

# 1. Which surface is wrong, established by running

The guide is right and the help pages understate. Every cell the five disputed
commands disagree about was probed against a built binary, on a fresh copy of the
fixture workbench per run, with the five references `.` (this workbench), `intake`
(a column), `wb-1` (a card), `wb-1/comments/1` (below a card) and `wb-1/comments`
(a collection):

    path        | . YES | intake YES | wb-1 YES | wb-1/comments/1 YES | wb-1/comments YES
    edit        | . YES | intake YES | wb-1 YES | wb-1/comments/1 YES | wb-1/comments NO
    show        | . NO  | intake YES | wb-1 YES | wb-1/comments/1 YES | wb-1/comments YES
    contents    | . YES | intake YES | wb-1 YES | wb-1/comments/1 YES | wb-1/comments YES
    attachments | . YES | intake YES | wb-1 YES | wb-1/comments/1 YES | wb-1/comments YES

`edit intake` exits 0 with `DINAH_EDITOR` pointed at a logging shim, and the shim's
log shows the launch, so the command reaches the column rather than exiting 0 without
doing anything. `show intake` and `show wb-1/comments` both exit 0, `contents
wb-1/comments` prints `wb-1/comments contains 1 entities.`, and `attachments
wb-1/comments` exits 0. The only refusal in that block is `edit wb-1/comments`, which
raises `dinah.is-a-collection` and matches the table's `no`.

Each of those five answers is what the guide's table already grants. So the defect is
not in dinah-457's derivation, and no cell of that table changes.

## 1.1 The set is five, and the Test stage's three was an undercount

The disagreement set is `path`, `edit`, `show`, `contents` and `attachments`.
dinah-457's Test stage named `path`, `show` and `contents`; it read the collection
column and the column cells of `path` and `show`, and it did not compare every cell of
every row against every sentence. The sentence for `edit` has never named a column and
`edit` has accepted one throughout, and the sentence for `attachments` narrows
below-a-card to a comment while `attachments wb-1/attachments/1` and `attachments
wb-1/checklist/1` both exit 0. Both were already wrong when that report was written.

Sentence against row, for all eighteen commands of the roster, reading the English
catalogue:

| Command | Row grants | Sentence names | Verdict |
|---|---|---|---|
| path | workbench, column, card, below, collection | workbench, card, below | narrow by column and collection |
| edit | workbench, column, card, below | workbench, card, below | narrow by column |
| get | workbench, column, card, below | workbench, column, card, below | agrees |
| set | workbench, column, card, below | workbench, column, card, below | agrees |
| show | column, card, below, collection | card, below | narrow by column and collection |
| instructions | column, card | column, card | agrees |
| attach | workbench, column, card, below | workbench, column, card, below | agrees |
| archive | column, card, below | column, card, below | agrees |
| restore | column, card, below | column, card, below | agrees |
| delete | column, card, below | column, card, below | agrees |
| contents | workbench, column, card, below, collection | workbench, column, card, below | narrow by collection |
| attachments | workbench, column, card, below, collection | workbench, column, card, below | narrow by collection, and narrow within below |
| rename | below | below, narrowed to an attachment | agrees |
| cite | below | below, narrowed to an item | agrees |
| resolve | below | below, narrowed to an item | agrees |
| verify | below | below, narrowed to an item | agrees |
| fail | below | below, narrowed to an item | agrees |
| reopen | below | below, narrowed to an item | agrees |

The bottom six rows narrow WITHIN the below-a-card kind, which the table is too coarse
to hold and the guide's detail paragraph already qualifies. That is not disagreement,
and section 4.2 keeps it in prose. Every row of that comparison is silent about the
workstream, which section 1.3 is about.

## 1.2 What the roster has done in nine days

dinah-457 shipped a table of fifteen rows in `863b7c5` on 2026-09-10. `get` and `set`
joined the roster in `8dfef06` and `restore` in `808d105`, both the same day, taking
it to eighteen. `git log -S` on the `guides` map entries in
`internal/verb/definition.go` dates all three. Each of those three commands needed a
table row and a help sentence written by hand against each other, and the earlier
hand-maintained roster is what drifted from ten to fifteen with nothing noticing. The
rate at which this roster changes is the measurement behind section 3.

## 1.3 The workstream is a sixth kind, and nine commands take one

The guide's answer for a command is not only its table row.
`internal/guide/guides/references.md:12` says a reference names this workbench, a
workstream, a column, a card, or something that hangs off one of those, and line 127
says which commands take the workstream, because the table deliberately leaves that
kind out. The promise sentence at the end of the section is not scoped to the table,
so a help page that says nothing about the workstream breaks it for the eight commands
line 127 names.

All eighteen roster commands were run against `workstream/astream` on a fresh copy of
the workbench per invocation, with the reference-argument conventions
`cmd/dinah/references_guide_test.go:referenceProbeArgs` already uses, and with
`restore` run a second time after archiving the workstream first:

    dinah path workstream/astream                     exit 0
    dinah edit workstream/astream                     exit 0   (shim logged the launch)
    dinah get workstream/astream title                exit 0
    dinah set workstream/astream title X              exit 0
    dinah show workstream/astream                     exit 2   dinah.unknown-path
    dinah instructions workstream/astream             exit 2   dinah.unknown-path
    dinah attach workstream/astream notes.txt         exit 2   dinah.not-attachable
    dinah archive workstream/astream                  exit 0
    dinah restore workstream/astream                  exit 2   dinah.not-archived
    dinah archive workstream/astream, then restore it exit 0
    dinah delete workstream/astream --yes             exit 0
    dinah contents workstream/astream                 exit 0
    dinah attachments workstream/astream              exit 0
    dinah rename workstream/astream r.txt             exit 2   dinah.not-renamable
    dinah cite workstream/astream url <url>           exit 2   dinah.unknown-path
    dinah resolve workstream/astream "a note"         exit 2   dinah.unknown-path
    dinah verify workstream/astream "a note"          exit 2   dinah.unknown-path
    dinah fail workstream/astream "a note"            exit 2   dinah.unknown-path
    dinah reopen workstream/astream "a reason"        exit 2   dinah.unknown-path

The shim log line reads `LAUNCHED
C:\dinah-scratch\dinah-470-spec3\probe\work\.dinah\<workbench>\workstreams\<id>`,
which is the workstream's own directory, so `edit` reaches the workstream rather than
exiting 0 having done nothing. The negative control is
`DINAH_EDITOR=dinah-no-such-editor dinah edit workstream/astream`, which exits 4 with
an exec failure, so an exit 0 here means a launch happened.

Nine commands take a workstream: `path`, `edit`, `get`, `set`, `archive`, `restore`,
`delete`, `contents` and `attachments`. The guide's sentence names eight of those nine
and omits `restore`, which section 4.4 corrects and section 5.8 holds.

# 2. The declaration

`internal/verb/reference_kinds.go` is new and carries the whole of it.

```go
// ReferenceKind is one of the kinds of thing a reference may name. The six
// values are the kinds the references guide's opening sentence lists, in the
// order it lists them, and they are the vocabulary a command's declaration is
// written in. Five of them are the columns of that guide's "Which command
// takes what" table; the workstream is the sixth, which the table leaves out
// and a sentence below the table answers instead.
type ReferenceKind string

const (
	ReferenceKindWorkbench  ReferenceKind = "workbench"
	ReferenceKindWorkstream ReferenceKind = "workstream"
	ReferenceKindColumn     ReferenceKind = "column"
	ReferenceKindCard       ReferenceKind = "card"
	ReferenceKindBelowCard  ReferenceKind = "below-card"
	ReferenceKindCollection ReferenceKind = "collection"
)

// ReferenceKindSeparator joins the kinds where a rendered clause names several
// of them. It is punctuation rather than prose, so it is written here rather
// than in the catalogues, and no kind label in any language may carry it.
const ReferenceKindSeparator = "; "

// ReferenceKindOrder is the order a rendered clause draws the kinds in.
func ReferenceKindOrder() []ReferenceKind

// MessageKey is where a kind's written label lives.
func (k ReferenceKind) MessageKey() string   // "reference.kind." + string(k)

// ReferenceKindsFor reports the kinds a command's reference may name, in
// ReferenceKindOrder, and whether the command declares any at all.
func ReferenceKindsFor(command string) ([]ReferenceKind, bool)

// referenceKinds is where the three published answers to "what does this
// command take" all come from: the references guide's table and its workstream
// sentence are held to it, the terminal help page renders it, and the tool
// schema renders it. What a command actually accepts is decided by its
// resolver, and this map is a declaration of that rather than the thing that
// enforces it, so it is held against the running binary by the probes named in
// sections 5.2, 5.8 and 5.9, for the cells those probes reach.
var referenceKinds = map[string][]ReferenceKind{ ... }
```

`referenceKinds` carries eighteen entries. Its five table columns reproduce the
guide's table exactly and no cell of them changes; its workstream column is section
1.3's measurement:

| Command | workbench | workstream | column | card | below-card | collection |
|---|---|---|---|---|---|---|
| path | yes | yes | yes | yes | yes | yes |
| edit | yes | yes | yes | yes | yes | no |
| get | yes | yes | yes | yes | yes | no |
| set | yes | yes | yes | yes | yes | no |
| show | no | no | yes | yes | yes | yes |
| instructions | no | no | yes | yes | no | no |
| attach | yes | no | yes | yes | yes | no |
| archive | no | yes | yes | yes | yes | no |
| restore | no | yes | yes | yes | yes | no |
| delete | no | yes | yes | yes | yes | no |
| contents | yes | yes | yes | yes | yes | yes |
| attachments | yes | yes | yes | yes | yes | yes |
| rename | no | no | no | no | yes | no |
| cite | no | no | no | no | yes | no |
| resolve | no | no | no | no | yes | no |
| verify | no | no | no | no | yes | no |
| fail | no | no | no | no | yes | no |
| reopen | no | no | no | no | yes | no |

A command absent from the map answers `false` from `ReferenceKindsFor` and renders no
clause, so a reference-taking command nobody declared is caught by section 5.1 rather
than by printing an empty parenthesis.

## 2.1 The shared renderer

`ArgumentMeaning` moves into `internal/verb` beside `SummaryKey`, whose doc comment
already says that both heads resolve the key in one place so that the page a person
reads and the schema an agent reads carry one sentence. The clause has to obey that
same rule, so it is composed here and not in either head.

```go
// ArgumentMeaning composes one argument's written meaning: the sentence written
// for it, followed by the address kinds its reference may name where the
// parameter takes a reference at all. translate is the caller's renderer, so
// this package composes the sentence without knowing where the words come from.
func ArgumentMeaning(command string, p Param, translate func(key string, args ...string) string) string
```

Its behaviour: resolve `translate(p.SummaryKey(command))`. Where `p.Guide` is not
`referencesGuide`, or `ReferenceKindsFor(command)` reports nothing, return that
sentence unchanged. Otherwise resolve `translate(k.MessageKey())` for each declared
kind in `ReferenceKindOrder()`, join them with `ReferenceKindSeparator`, and return
`translate("help.reference-kinds", "summary", summary, "kinds", joined)`.

`cmd/dinah/help.go:argumentMeaning` becomes a call to it, keeping the vocabulary
append it already performs layered on top. `internal/mcp/tools.go:442` stops calling
`catalog.T(param.SummaryKey(t.command))` and calls it too.

No parameter may declare both a `Vocabulary` and the references guide, because the two
appends would then compose and nobody has written the sentence that results. Section
5.1 refuses that combination; none exists today.

# 3. Whether the guide keeps the promise

It keeps it, for the whole of the answer the guide gives rather than for its table
alone, and the sentence in `internal/guide/guides/references.md` stands unchanged:

    Each command's own help page carries the same answer for that one command,
    so run `dinah help attach` when you want it beside the arguments rather
    than here.

Round 2 of this card claimed the same thing while declaring five kinds, and the claim
was false for a sixth. That is why the workstream is in the declaration rather than in
a paragraph explaining why it is absent. A card that replaces a soft prose
understatement with a generated closed enumeration, held in eight languages by a test
that refuses an undeclared label, converts a documentation slip into machine-enforced
text that tells a reader something the code does not do. The sixth kind costs one more
`reference.kind.*` key across eight catalogues, one more cell per row in
`referenceKinds`, a clause on nine help pages that grows by one label, and the
rewiring of one existing test in section 5.8. Section 8 records what was left undone
beside it.

**Deleting the promise sentence** leaves five help pages telling a reader that `path`
does not take a column and that `show` does not take a collection, in a tool whose
guide is the address contract. Someone reading `dinah help show` who wants a whole
collection is told to write something else. Deleting the promise does not repair that,
so deletion is a smaller card that leaves the defect standing.

**A guard over the English sentences** was rejected on the one measurement that
settles it. Such a guard reads `internal/msg/locales/en.json` and nothing else, and the
promise the guide makes is made to a reader in any of eight languages. The de and hi
catalogues carry real translations of all ten sentences under discussion, and this
workbench has already ruled twice that a guard which has to adjudicate German or Hindi
prose is not buildable here: dinah-460 refused one that fired falsely five times in a
language nobody on the project reads, and dinah-461 refused a reverse glossary check
after measuring eight Hindi false hits and one wrong verdict on a correct German
translation. A prose guard would hold the promise in one language and abandon it in
seven, and a promise that is true in English only is not the promise the sentence
makes.

**Rendering the kinds from a declaration** holds it in all eight languages with no
prose read anywhere. Nothing in section 5 parses a sentence for meaning. The two checks
that touch catalogue text at all compare a rendered clause against the same catalogue
entries the renderer drew from, after reversing the template that wraps it, which
section 5.3 sets out. That is why this card wants no whole-corpus prose scanner and does
not overlap with dinah-478: if dinah-478 lands its scanner, nothing here reads it, and
if it does not, nothing here is blocked.

The false-positive reach of what section 5 proposes is zero prose judgements. The
sources of a false failure are enumerated in section 6, and each is closed by
construction rather than by an exemption list.

# 4. The catalogue work

## 4.1 Seven new keys

Each lands in all eight catalogues: translated in `de` and `hi` with a `source`
fingerprint, and carried as a `skeleton` holding the English text in `af`, `cs`, `es`,
`fil` and `id`, which is the shape `internal/msg/collection_keys_test.go` already
requires of a key family.

| Key | English |
|---|---|
| `reference.kind.workbench` | this workbench, written as `workbench` or `.` |
| `reference.kind.workstream` | a workstream, written as `workstream/<slug>` |
| `reference.kind.column` | a column |
| `reference.kind.card` | a card |
| `reference.kind.below-card` | something below a card |
| `reference.kind.collection` | a whole collection |
| `help.reference-kinds` | {summary} (one of: {kinds}) |

The six labels carry no per-command example, deliberately. `dinah help resolve` grants
below-a-card and accepts only a checklist item, so a label reading "something below a
card, such as wb-1/comments/1" would print an example that command refuses. The label
states the kind and the command's own sentence keeps its own example. The workstream
label names a spelling rather than an example, the way the workbench label names
`workbench` and `.`, and every command granting the kind takes that spelling.

`help.reference-kinds` needs this in its `context`, because the rule that governs
`{kind}` elsewhere in these catalogues does not govern `{kinds}` here, and a translator
who applies the wrong one loses grammar the sentence is entitled to:

    One argument's meaning followed by the kinds of thing its reference may
    name. {summary} is the argument's own sentence. {kinds} holds the labels
    reference.kind.workbench, reference.kind.workstream, reference.kind.column,
    reference.kind.card, reference.kind.below-card and
    reference.kind.collection, joined by a semicolon and a space, and those
    labels are translated prose rather than machine vocabulary, so unlike
    {kind} in the unknown-field refusals a word of this sentence may agree with
    what is substituted here. No label may itself contain a semicolon followed
    by a space, because the guard over this clause splits the list on that
    pair.

## 4.2 Ten sentences lose the enumeration they duplicate

The rendered clause carries the kinds, so a sentence that also lists them is a second
copy of the fact this card exists to stop copying. Each English text below replaces the
current one; `de` and `hi` are retranslated and refingerprinted, and the five skeleton
catalogues take the new English.

| Key | New English |
|---|---|
| `param.path.card.summary` | the thing whose file path you are printing, such as wb-1/comments/1 |
| `param.edit.card.summary` | the thing you are opening in your editor, such as wb-1/comments/1 |
| `param.get.ref.summary` | the entity you are reading, written as its reference |
| `param.set.ref.summary` | the entity you are writing, written as its reference |
| `param.show.card.summary` | the thing whose detail you are reading, such as wb-1 or wb-1/comments/1 |
| `param.instructions.card.summary` | the position whose instructions you are reading |
| `param.attach.ref.summary` | what the file hangs off; below a card that is a comment, or, with --replace, the attachment whose bytes you are replacing |
| `param.ref.summary` | the entity you are acting on, written as its reference, such as wb-1/comments/1 |
| `param.contents.ref.summary` | the entity whose contents you are drawing, written as its reference |
| `param.attachments.ref.summary` | the entity whose attachments you are listing; this workbench when you name none |

Two of those ten new sentences carry `"; "` themselves, `param.attach.ref.summary` and
`param.attachments.ref.summary`, and that is allowed. The ban in section 5.3 falls on
the six kind labels alone, because the check reverses the `help.reference-kinds`
template before it splits anything and therefore never splits a summary. Five of the
ten sentences shipping today already carry the pair, in every one of the eight
catalogues, so a ban that reached summaries would be a rewrite of prose this card has
no reason to touch. That five is derived rather than counted, by this, run from the
repository root:

```python
import json, glob, os
keys = ["param.path.card.summary", "param.edit.card.summary",
        "param.get.ref.summary", "param.set.ref.summary",
        "param.show.card.summary", "param.instructions.card.summary",
        "param.attach.ref.summary", "param.ref.summary",
        "param.contents.ref.summary", "param.attachments.ref.summary"]
def txt(e):
    return e["text"] if isinstance(e, dict) else e
for f in sorted(glob.glob("internal/msg/locales/*.json")):
    entries = json.load(open(f, encoding="utf-8"))["entries"]
    hits = [k for k in keys if "; " in txt(entries[k])]
    print(os.path.basename(f), len(hits), sorted(k.split(".")[1] for k in hits))
```

At `ab5debc` every one of the eight lines reads `5 ['attach', 'attachments',
'instructions', 'ref', 'show']`. The other five sentences carry a colon rather than a
semicolon.

Three of the ten carry a correction rather than only a strip.

`param.attach.ref.summary` keeps its narrowing, because `attach` takes only a comment
below a card and takes an attachment only with `--replace`. Probed: `attach
wb-1/comments/1 <file>` exits 0, `attach wb-1/checklist/1 <file>` raises
`dinah.not-attachable`, and `attach wb-1/attachments/1 <file>` raises the same without
`--replace`.

`param.attachments.ref.summary` LOSES a narrowing that was wrong. It said "or a comment
such as wb-1/comments/1", and `attachments wb-1/attachments/1` and `attachments
wb-1/checklist/1` both exit 0, so `attachments` takes anything below a card. Dropping
the clause corrects it.

`param.ref.summary` carries a `context` reading "The meaning of the ref argument of
archive and delete, which take the same set of things." `restore` joined that shared key
in `808d105` and the context did not follow. It becomes "The meaning of the ref argument
of archive, restore and delete, which take the same set of things."

The three sentences that name a sub-kind rather than a kind are untouched:
`param.rename.ref.summary`, `param.cite.item.summary` and `param.item.summary`.

One thing for the implementer to notice while retranslating. The German shipping today
for `param.show.card.summary` reads "ein Zustand, eine Karte oder etwas unterhalb einer
Karte ...", and "ein Zustand" names a kind the English never named. The key is
retranslated here anyway, so this is not extra work; it is a warning not to carry the
phrase forward.

## 4.3 Six command summaries lose their enumeration too

dinah-467 found that `cmd.edit.summary` and `cmd.path.summary` enumerate address kinds
in the one-line summary that heads a help page, and asked this card to take that or
decline it. This card takes it, and the set is six rather than two. Every roster
command's summary was read, so the six is the whole of it rather than the part somebody
noticed. The listing is produced by resolving `cmd.<name>.summary` from
`internal/msg/locales/en.json` for each of the eighteen roster commands of section 0:

    archive       Move a card, a column, or anything below a card, out of the live set
    attach        Attach a file, or replace its bytes
    attachments   What is attached to an entity of the workbench
    cite          Cite evidence on a checklist item
    contents      What an entity of the workbench contains
    delete        Destroy a card, a column, or anything below a card, along with its history
    edit          Open this workbench, a card, or anything below a card in your editor
    fail          Record an acceptance criterion as failed
    get           Read one field of any entity of this workbench
    instructions  The instructions served at a position
    path          Print the file path of this workbench, of a card, or of anything below a card
    rename        Rename an attachment
    reopen        Return a closed checklist item to pending
    resolve       Resolve an open question or a decision
    restore       Put a card, a column, or anything below a card, back into the live set
    set           Write one field of any entity of this workbench
    show          A card, or anything below it
    verify        Record an acceptance criterion as verified

Six of the eighteen enumerate: `archive`, `delete`, `edit`, `path`, `restore` and
`show`. All six are wrong against section 2's declaration. `edit` omits the column and
the workstream, `path` omits the column, the collection and the workstream, `show`
omits the column and the collection, and `archive`, `restore` and `delete` each omit
the workstream. The three that look right against the table alone are wrong once the
workstream counts, which is the sharpest reason not to leave any of them enumerating.

Each becomes a summary of what the command does, with the kinds left to the argument
row that generates them:

| Key | New English |
|---|---|
| `cmd.archive.summary` | Move an entity out of the live set |
| `cmd.delete.summary` | Destroy an entity, along with its history |
| `cmd.edit.summary` | Open an entity of this workbench in your editor |
| `cmd.path.summary` | Print the file path of an entity of this workbench |
| `cmd.restore.summary` | Put an archived entity back into the live set |
| `cmd.show.summary` | The detail of an entity of this workbench |

`get`, `set`, `contents` and `attachments` already read that way, so the six join a
shape the catalogue already uses rather than inventing one.

Nothing holds these six stripped, and that is a limitation to state rather than to
paper over. A guard would have to decide whether a sentence enumerates address kinds,
in eight languages, which is the wall section 3 refuses. What makes the strip durable
instead of decorative is that a stripped summary makes no claim about what the command
accepts, so there is nothing left to drift out of step. A future summary that
reintroduces an enumeration is a new defect rather than a return of this one.

## 4.4 Two corrections that ride with the sixth kind

**The guide's workstream sentence is short by one.**
`internal/guide/guides/references.md:127` names eight commands and `restore` takes a
workstream, measured in section 1.3. The line is rewritten in place, staying one line
so that the prose ledger's pinned lines below it do not move:

    Nine commands take a workstream: `path`, `edit`, `get`, `set`, `archive`, `restore`, `delete`, `contents`, and `attachments`. The others refuse one, and the table leaves the workstream out rather than carrying a column for it, so this sentence is where that answer lives.

The trailing clause loses "a column that is mostly no", which was true of eight rows in
eighteen and is false of nine. `cmd/dinah/testdata/prose-figures.txt:77` pins that
sentence's figure, so its `figure=Eight` becomes `figure=Nine` on the same line.

**`edit`'s refusal table names two of the three things that can go wrong.**
`internal/verb/checks.go:212` declares `dinah.unknown-path` and `dinah.no-editor` for
`edit`, and dinah-455 minted `dinah.is-a-collection` without adding a row. This card
takes that too, because the missing row states the same fact as the `no` in
`referenceKinds["edit"]` against the collection kind, and it sits on the same help page
this card regenerates. A third row
`{Refusal: contract.IsACollection, Key: "check.edit.3"}` lands between the existing
two, and `check.edit.3` lands in all eight catalogues by the shape of section 4.1.

The position is measured rather than assumed, since the table is headed "in the order
each is checked". `dinah edit nosuch` raises `dinah.unknown-path` with the editor
unset, so resolution runs first. `dinah edit wb-1/comments` raises
`dinah.is-a-collection` with a working shim, with a broken editor name, and with
`DINAH_EDITOR`, `EDITOR` and `VISUAL` all unset, so the collection refusal precedes
every editor check. English text for the row: "the reference names a whole collection
rather than one thing in it".

Nothing in the tree holds any command's refusal rows against the refusals it can
actually raise, so `edit` is one sighting of a class nobody has enumerated. This card
does not enumerate it and mints no guard over it, because doing so means running every
command against every refusal it declares and every one it does not, which is a card of
its own rather than a paragraph of this one. Section 8 records it.

# 5. What holds it

Every check below asserts how many things it read, so a sweep whose subject set went
empty stops rather than passing.

## 5.1 `internal/verb/reference_kinds_test.go`, new

`TestEveryReferenceTakingCommandDeclaresItsKinds` compares the key set of
`referenceKinds` against `ReferenceTakingCommands()` in both directions and fails on
either asymmetry. It asserts the roster is non-empty, asserts the declaration carries a
kind list for all eighteen, and logs the size. A nineteenth command joining the roster
reddens here rather than rendering nothing.

`TestNoReferenceTakingParameterAlsoDeclaresAVocabulary` walks every command of the
roster and every parameter carrying `Guide: referencesGuide`, and fails on one that also
names a `Vocabulary`. It asserts it read eighteen such parameters, which is the figure
section 0 derives, so a run that found none stops. It reads the parameter's `Guide`
field rather than grepping the source, since the two spellings of that guide's name are
what makes a source grep answer sixteen.

## 5.2 `cmd/dinah/references_guide_test.go`, extended

`TestTheReferencesGuideTableDrawsTheDeclaredReferenceKinds` reads the shipped table
through the existing `parseReferencesGuideTable` and compares it against
`ReferenceKindsFor`, cell by cell, in both directions: a declared command with no row
fails, a row naming an undeclared command fails, and a cell whose `yes` or `no`
disagrees with the declaration fails naming the command, the column and both answers. It
asserts eighteen rows and ninety cells were compared before it reports anything.

`referenceGuideHeading(kind verb.ReferenceKind) (string, bool)` is the one place the
table's English column headings are mapped onto the declaration's tokens. It answers
`false` for `ReferenceKindWorkstream` alone, because the table draws no column for that
kind and section 5.8 holds it instead. The test asserts that exactly one kind of
`ReferenceKindOrder()` is unmapped and that it is the workstream, so a seventh kind
fails here rather than becoming a cell nobody compares. A kind the helper does not know
at all is a fatal naming the kind.
`references_command_resolution_test.go:column` is rewritten to answer a
`verb.ReferenceKind` and its callers go through this helper, so the mapping is not
written twice.

The existing bidirectional guards keep their reach and change only where they read the
declaration from.
`TestTheReferencesGuideDeclaresEveryCommandTheCollectionColumnIsAheadOf` already probes
all eighteen commands against a real collection reference and already fails both ways,
and `TestEveryAddressTheContentsTreeDrawsResolvesThroughTheCommandsThatDeclareIt`
already probes `show`, `path`, `edit` and `attachments` against the other four kinds.
Both now read `verb.ReferenceKindsFor` rather than parsing the guide's table for the
declaration, which is what makes the guide derived rather than authoritative.

## 5.3 `cmd/dinah/reference_kinds_help_test.go`, new

`TestEveryHelpPageNamesTheReferenceKindsItDeclares` is the check that makes the guide's
sentence true. For every catalogue in `internal/msg/locales/` and every command of the
roster, it runs `dinah help <command>` through `runCLI`, folds the output's whitespace
to single spaces, and locates the reference argument's row.

It then isolates the kinds region before it splits anything, because the rendered row is
a summary inside a template and two of the summaries of section 4.2 carry
`verb.ReferenceKindSeparator` themselves. It resolves that catalogue's summary for the
parameter directly, renders that catalogue's `help.reference-kinds` entry with that
summary and a sentinel standing in for `{kinds}`, and splits the rendered template on
the sentinel to obtain the literal prefix and suffix the template wraps the kinds in.
The observed row must equal prefix + region + suffix, failing and naming the command and
the language when it does not, and the region between them is the kinds list. Splitting
that region on `verb.ReferenceKindSeparator` is then exact, because the region holds only
labels and the sibling check below refuses a label carrying the pair.

The resulting set is compared against the labels that catalogue carries for the declared
kinds. A declared kind whose label is absent fails naming the command, the language and
the label; a label present for an undeclared kind fails the same way.

Its expectation is built from the catalogue entries and the declaration, and it never
calls `verb.ArgumentMeaning`, the template reversal above included, since that reads the
catalogue's own entries. A guard that recomputed its expected value from the function
under test would pass against any renderer at all.

The catalogue set is enumerated from the directory rather than listed, so a ninth
language fails here instead of being missed. It asserts eight catalogues and one hundred
and forty-four pages read, which is the eighteen commands of section 0 against those
eight catalogues.

Each page is rendered twice, at `COLUMNS=40` and at `COLUMNS=200`, and the folded clause
has to be identical at both widths. That is what proves the check reads the clause rather
than an accident of one window.

`TestNoReferenceKindLabelCarriesTheClauseSeparator` reads the six labels from all eight
catalogues and fails on one containing `verb.ReferenceKindSeparator`. It asserts
forty-eight labels were read, which is six against eight. Its subject is the six
`reference.kind.*` labels and nothing else, and the summaries are outside it, by section
4.2. This is the guard that lets the region split be exact, and it is the one instruction
a translator needs.

## 5.4 `internal/msg/reference_kind_keys_test.go`, new

`TestTheReferenceKindKeysReachEveryCatalogue` follows
`internal/msg/collection_keys_test.go` line for line over the seven keys of section 4.1:
the base entry carries no `source` and no `skeleton`; `de` and `hi` carry text differing
from the English and a `source` equal to `msg.Fingerprint` of the English of the day;
every other catalogue carries the skeleton mark, no source, and the English text. It
enumerates the catalogue directory rather than naming the languages, asserts eight files,
and asserts fifty-six entries read, which is the seven keys against those eight files.

## 5.5 `internal/mcp/levels_test.go`, extended

`TestTheFieldToolsCarryTheSameSentencesTheTerminalPrints` compares the schema's property
description against `verb.ArgumentMeaning(command, param, catalog.T)` instead of against
`catalog.T(param.SummaryKey(command))`, which is what keeps its name true once the
terminal prints more than the bare sentence. That comparison stops being independent the
moment `internal/mcp/tools.go` calls the same function: both sides become one expression,
and a defect inside `ArgumentMeaning` passes it. What it still proves is what its name
claims, that the two heads publish one string. Its doc comment says so, so that a later
reader does not take it for a proof of what the clause contains.

`TestEveryReferenceTakingToolPublishesTheKindsItsReferenceMayName` is the independent
proof for the schema, the way 5.3 is for the terminal. It walks the tool roster, keeps the
tools whose command is in `ReferenceTakingCommands()`, and requires each one's reference
property description to carry the kinds the declaration grants that command. It builds its
expected clause from the catalogue entries and the declaration, by the same template
reversal 5.3 uses, and its source names `verb.ArgumentMeaning` nowhere. It asserts it found
at least one such tool and logs how many, so a tool roster that stopped carrying any
reference-taking command stops the run.

## 5.6 Existing expectations that move

Named here so the implementer meets them before the build does. Each is named by the
symbol it lives in rather than by a line number, because a line number in this spec was
wrong once already and the files will have moved by the time anybody reads this.

- `cmd/dinah/levels_test.go:ratifiedSetHelp`, the golden page for `dinah help set`, gains
  the clause on its `<ref>` row and loses the enumeration from the sentence.
- `cmd/dinah/main_test.go:TestEveryPageSaysWhatEachArgumentIs`, whose `carries` literals
  for `path`, `edit`, `show`, `instructions` and `archive` quote sentences section 4.2
  rewrites. Those five are the whole of what moves in that table, since the entries for
  `init`, `claim`, `block`, `query` and `delete` quote nothing section 4.2 touches.
- `cmd/dinah/row_pairing_test.go:expectArguments`, which builds its expected arguments
  table from `param.SummaryKey` and moves to `verb.ArgumentMeaning`. Its swept command is
  `attach`, which is in the roster, so this one currently passes only because the clause
  does not exist yet.
- Any golden page, `carries` literal or fixture quoting one of the six command summaries
  of section 4.3 or `edit`'s refusal table from section 4.4. The implementer finds those
  by searching the tree for each old English rather than by trusting this list, because
  sections 4.3 and 4.4 are new in round 3 and no earlier round swept for them.

## 5.7 What is deliberately not built, and what that leaves unproven

The binary probe is NOT widened to all eighteen commands across the five table kinds.
Four commands are probed against four kinds and all eighteen against the collection kind,
which is the reach the existing guards already have, and section 5.8 adds all eighteen
against the workstream.

The reason is that a uniform probe would fire falsely against a correct declaration. Every
line below was run at `808d105` and re-run at `ab5debc`, against a throwaway workbench
holding one column with a card in it (`intake`), one empty column (`doing`), one card
(`wb-1`) and one comment below that card (`wb-1/comments/1`), with a fresh copy of the
workbench per invocation. Each line gives the invocation and the exit status, so the next
reader can repeat the measurement rather than trust it.

The `no` cells are observable by exit code. An earlier draft of this section said they were
not, and that was wrong:

    dinah archive .        exit 2   dinah.unknown-path
    dinah restore .        exit 2   dinah.not-archived
    dinah delete . --yes   exit 2   dinah.unknown-path

Bare `dinah delete .` does answer `dinah.unconfirmed`, which is a refusal for the wrong
reason, but it still exits 2, and
`cmd/dinah/references_guide_test.go:referenceProbeArgs` already supplies `--yes` for
`delete`, which reaches the genuine refusal above. The arguments half of a widened probe is
therefore already solved in the tree.

The `yes` cells are where it breaks. Five references the guide's table grants are refused,
because the entity is in the wrong state rather than of the wrong kind:

    dinah restore intake             exit 2   dinah.not-archived
    dinah restore wb-1               exit 2   dinah.not-archived
    dinah restore wb-1/comments/1    exit 2   dinah.not-archived
    dinah archive intake             exit 2   dinah.occupied
    dinah delete intake --yes        exit 2   dinah.occupied

`dinah.occupied` is a third pre-kind refusal and an earlier draft of this card named it
nowhere. A probe that ran those five cells and read a non-zero exit as `no` would report
five disagreements against a declaration that is right, which is the guard this workbench
refuses to ship.

Classifying by refusal name does not rescue it, and this is the sharpest measurement of the
set. `restore .` is a `no` cell and `restore intake` is a `yes` cell, and both answer
`dinah.not-archived`. A per-command table of which refusals mean "wrong kind" cannot
separate those two, whatever is written in it.

The two refusals do carry different sentences after the name, and a future card must not
read that as an escape. `.` draws "the workbench wb is never archived, so name a column, a
card, or something below a card instead" and `intake` draws "run `dinah archive intake`
first if moving it out of the live set is what you meant". Discriminating on that means
reading a refusal sentence for meaning in eight languages, which is the guard dinah-460 and
dinah-461 each refused and which section 3 refuses again. So the residue is setup rather
than classification, and the door is closed rather than merely leaned on.

Arranging the state does separate them:

    dinah archive wb-1                 exit 0 ; dinah restore wb-1                exit 0
    dinah archive doing                exit 0 ; dinah restore doing               exit 0
    dinah archive wb-1/comments/1      exit 0 ; dinah restore wb-1/comments/1     exit 0
    dinah delete doing --yes           exit 0    (an empty column)
    dinah delete wb-1 --yes            exit 0
    dinah delete wb-1/comments/1 --yes exit 0

So a widened probe needs, per command and per kind, both a reference of that kind the
command can reach and the workbench state that lets the cell answer at all: archive the
target before restoring it, empty the column before archiving or deleting it. The six
commands that narrow within below-a-card need the reference chosen per command as well,
since naming a comment draws `dinah.unknown-path` from `resolve` and `verify` and
`dinah.not-renamable` from `rename`, each against a `yes` cell:

    dinah resolve wb-1/comments/1 "a note"    exit 2   dinah.unknown-path
    dinah verify wb-1/comments/1 "a note"     exit 2   dinah.unknown-path
    dinah rename wb-1/comments/1 renamed.txt  exit 2   dinah.not-renamable

That per-cell setup table is a second hand-maintained list of the shape this workstream
keeps deleting, and whoever added the nineteenth command would be the one maintaining it.
Once the setup is right the classification is trivial: on every cell measured above, exit 0
is `yes` and any non-zero exit is `no`.

What that leaves unproven, stated plainly: for the fourteen commands outside the existing
probe, nothing runs the binary against a workbench, a column or a card and compares the
answer to the declaration. Those fifty-six cells rest on the matrix in section 1 having been
run by hand and on the guide's table and the help pages now being unable to disagree with
each other. A future card that wants those cells held should mint the per-cell setup as a
declaration each command carries, so that the nineteenth command declares how it is probed
rather than being added to a table in a test. Section 5.8 builds that declaration at one
column's width, shaped so a future card widens it rather than replacing it.

## 5.8 The workstream column, and the test that passes by luck

`TestTheReferencesGuideNamesTheCommandsThatTakeAWorkstream` exists and it passes today
against a sentence that is wrong. It runs every roster command against a workstream of its
own and reads `commandTookTheReference`, which is `got.code == 0` for every command but
`edit`. Its fixture creates a workstream and never archives one, so `restore` answers
`dinah.not-archived`, the helper reads that as "did not take the reference", and the check
agrees with a sentence that omits `restore`. That is section 5.7's hazard running live and
green in the repository, and it is the concrete instance of the argument section 5.7 makes
abstractly.

The test is rewritten rather than replaced, so that the `by=` reference at
`cmd/dinah/testdata/prose-figures.txt:77` keeps naming a test that exists. Its doc comment
is rewritten too: it currently calls the sentence "a hand-written list of six names" while
the sentence names eight, which is itself the class of defect dinah-444 stands for.

Three changes:

- A per-command setup hook, `workstreamProbeSetup(name string) [][]string`, returning the
  invocations to run against the fresh workbench before the probe. `restore` returns one,
  `{"archive", "workstream/<slug>"}`; every other command returns none. The hook is written
  as a declaration with a default of nothing, so a nineteenth command needing state is one
  entry rather than a rewrite, which is the shape section 5.7 asks a future card to widen.
- A three-way comparison rather than two-way. The check holds what the binary did, what
  `ReferenceKindsFor` declares for the workstream kind, and what the guide's sentence names,
  against each other in both directions on each pair, failing and naming the command and the
  two sides that disagree. The guide-against-binary comparison it already performs is kept,
  so nothing it proves today is given up.
- Two counts rather than one. It asserts eighteen commands probed, and asserts the two
  halves separately: nine reached the workstream and nine were refused. A single combined
  count would hide a half that read nothing behind a half that read plenty.

The marker the check passes to `foldedGuideParagraphStartingWith` becomes "Nine commands
take a workstream:", matching section 4.4's rewritten sentence, and `prose-figures.txt:77`
carries the same figure, so the two move together the way that ledger already intends.

## 5.9 What the workstream column is now held to

Adding the sixth kind buys eighteen more cells against the running binary, which is more
coverage than any of the five table columns has except the collection. Stated so the next
reader does not have to work it out:
`TestTheReferencesGuideDeclaresEveryCommandTheCollectionColumnIsAheadOf` holds eighteen
collection cells, `TestEveryAddressTheContentsTreeDrawsResolvesThroughTheCommandsThatDeclareIt`
holds sixteen more across four commands and four kinds, and section 5.8 holds eighteen
workstream cells. That is fifty-two of the declaration's one hundred and eight cells held
against the binary, against thirty-four of ninety before this card.

# 6. What a false failure would look like, and why it cannot happen

**Wrapping.** The arguments table wraps the meaning column to the reader's window.
`cmd/dinah/row.go:packTokens`, declared at line 164 at `808d105`, lays out whole tokens
taken from `strings.Fields` and joins them with one space, and its own comment records that
a token wider than the room is written whole rather than split. It never hyphenates and
never breaks inside a word, so folding the output's whitespace to single spaces recovers the
clause byte for byte. The two-width run in 5.3 is what keeps that true if the wrapper ever
changes.

**A label containing the separator.** 5.3 splits the kinds region rather than matching
substrings, so a label carrying `"; "` would be read as two labels and the check would fire
against correct code. `TestNoReferenceKindLabelCarriesTheClauseSeparator` refuses that
catalogue instead, in whichever language introduced it, and the remedy is one punctuation
change rather than an exemption. A summary carrying the pair is harmless and stays legal,
because the template reversal removes the summary before the split.

**A missing catalogue entry.** A language lacking a label would render the English fallback
and the comparison would fire. `TestTheReferenceKindKeysReachEveryCatalogue` requires the
entry in all eight before 5.3 ever runs.

**A workstream cell refused for its state rather than its kind.** This is section 5.7's
false-failure shape, and section 5.8 meets it with per-command setup rather than with a
refusal-name lookup. `restore` is the one command needing it today, and a command added
later that needs it and does not declare it reddens 5.8 rather than passing, because the
declaration says the command takes a workstream and the probe says it did not.

No check in this card reads a sentence for meaning, so the failure mode that stopped
dinah-460 and dinah-461, a guard adjudicating prose in a language nobody here reads, does not
arise. The checks that touch translated text compare it against itself and against a
separator.

# 7. What this card edits outside the code

Round 2 of this card ruled that `internal/guide/guides/references.md` is not edited. The
sixth kind reopens that ruling for exactly one line, and the narrower ruling is this.

- `internal/guide/guides/references.md:127` is rewritten in place by section 4.4, staying
  one line. Nothing else in that file changes: its table is correct on every cell probed,
  its counting sentences stay right at eighteen commands and six distinct sets, and
  `cmd/dinah/testdata/prose-figures.txt` pins four of its figures by line number, at lines
  18, 104, 127 and 129, which is the figure section 0 derives. Rewriting line 127 in place
  moves none of them.
- `cmd/dinah/testdata/prose-figures.txt:77` changes `figure=Eight` to `figure=Nine` on the
  same line, and may restate its `reason=` to mention the per-command setup. Nothing else in
  that ledger changes.

No workstream column is added to the guide's table. The table draws five kinds and the
sentence below it answers the sixth, which is the arrangement the guide already chose;
adding a column would move lines 129 onward and cost two ledger entries for no reader gain.

# 8. Out of scope

- The bare twelve-hex card identifier the dinah-457 Test stage found to be an address the
  guide never shows is a defect in the guide's own text rather than in help text, and it is
  not this card's.
- No new command, and no change to what any command accepts. The one refusal row of section
  4.4 documents a refusal that already exists rather than minting one.
- Nothing holds any command's declared refusal rows against the refusals it can raise, and
  this card does not build that. `edit` is taken because its missing row states the same fact
  as a cell of this card's own declaration; the class behind it wants a card that runs every
  command against every refusal it declares and every one it does not, which is work somebody
  would pick up on its own.
- Nothing holds the six command summaries of section 4.3 stripped, for the reason that
  section gives, and this card builds no prose guard to do it.
- No whole-corpus prose scanner, and no dependency on dinah-478 in either direction.

## Branch

dinah-470-the-guide-says-each-commands-help-repeats-its-answer-and-for-three-commands-the-help-says-something-narrower
