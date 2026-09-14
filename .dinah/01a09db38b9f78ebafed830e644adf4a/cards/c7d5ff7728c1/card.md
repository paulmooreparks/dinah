---
title: One way to address and manipulate everything a workbench holds
column: b69abf918c42
state: ready
severity: major
priority: now
tier: frontier
workstreams:
  - 994787601ae6
---
The operator's requirement, stated 2026-09-08: a completely consistent way of addressing and manipulating all items in a workbench. This card is that requirement. It is a design card first; the code changes it implies should be filed as its children once the contract is settled.

Start from what is already right, because the fix is not a rewrite. The containment grammar is a single statement in one table naming what contains what, every resolver reads it rather than repeating it, and the reference spelling that falls out is uniform: a head, then a collection name, then a position, as deep as the grammar goes. That part holds. What is inconsistent is the surface built on top of it, and the inconsistency is in three separate layers.

WHAT THE SURFACE ACTUALLY DOES TODAY, from a read of the tree on 2026-09-08 at trunk 22a35fc. Every claim here should be re-verified by whoever specs this rather than trusted.

Which commands take a reference. Fifteen commands point their reader at the references guide. That guide's own table describes ten of them. The five checklist-state verbs (cite, resolve, verify, fail, reopen) take a reference and are absent from the table a reader is sent to.

What each command accepts is genuinely different and only partly documented. `path` takes a collection and a member; `show` takes a member and refuses a collection; the guide's table says both take anything below a card. Four rows of that table already carry footnotes for exactly this kind of narrowing, so the mechanism to say it exists and was not used here.

What a listing prints. The three kinds of thing a card holds are addressed three different ways on the same screen. A checklist item prints its own full reference as the row's first column. An attachment prints a bare position integer. A comment prints neither, so a reader who wants to name one has nothing on screen to work from and has to know the grammar already.

What can be done to each kind, which is the sharpest asymmetry of the three:
- An attachment can be created, have its payload replaced, be renamed, archived and deleted.
- A checklist item can be created, moved through five states, cited, archived and deleted.
- A comment can be created, archived and deleted, and its text cannot be changed by any command that builds a request. `edit` opens the reader's own editor and never constructs one, so there is no route to a comment's text from a script or from an MCP client at all.
- A card, a column, a workbench and a workstream each have their own update verb.

What contains what. A comment mounts attachments; a checklist item mounts nothing. Whether that asymmetry is deliberate is not written down anywhere the grammar's table can be read against.

`archive` exists for every kind and there is no `restore` for any of them, so the reversible half of the pair is reversible only by moving a directory by hand.

WHAT THIS CARD OWES AN ANSWER TO. None of these is the operator's to rule on; he has stated the requirement and the shape is ours to settle.

The addressing contract. State, in one place, what a reference may name, what every command that takes one accepts, and how a reader learns the address of a thing they can see. The likely rule is that any surface drawing an addressable entity prints the address a reader types, and that a command refusing a reference says what the reference does name and what to type instead. Write the rule down before writing code against it, because the point of this card is that the rule was never stated and each surface answered it locally.

The manipulation contract. For each kind the grammar names, say which of create, read, update, delete and archive it supports, and whether an absence is deliberate or an omission. A comment with no update path is the case that prompted this; decide it rather than inheriting it. Whatever the answer, the same answer must hold at a terminal and over MCP, because a capability that exists only behind an interactive editor is not a capability an agent has.

Whether the containment table's own asymmetries are intended, and say so in the table.

Whether archive without restore is a contract this project means to publish.

The machine surface. Every decision above has to be true of the MCP head as well as the CLI, and the head deliberately does not serve the commands that need a shell or a filesystem. Check that no capability lands only on the side an agent cannot reach.

SCOPE. This card produces the settled contract plus the child cards that implement it. It should not itself become a large diff. Two of its instances are already filed and should be linked as children rather than re-specified here: dinah-454, that a card's comments print with no address; and dinah-455, that naming a collection tells the reader it does not exist when what was refused was the addressing. dinah-435 is the precedent worth reading first, because it fixed exactly this defect for checklist items on the same screen and established the pattern the other two rows do not follow. dinah-451 establishes the constraint that a position is not a stable handle: positions shift once an earlier member of a collection is deleted, so whatever address this contract tells a reader to type, it must be clear whether it is a spelling for now or a handle to store.

RELATED, and deliberately not folded in: dinah-443 is settling what a user-facing command is called, which is vocabulary rather than addressing.

## Specification

This card produces a contract and the child cards that implement it. It produces no
production diff of its own.

Everything below was verified against `origin/main` at 22a35fca90c254625071b9b7eca237131cdce3b0,
partly by reading and partly by running a binary built from that commit against a throwaway
workbench outside the operator's profile. Round 1 worked in `C:/dinah-scratch/dinah-456-spec/wt`
and round 2, which repaired the four findings of the second design review, worked in
`C:/dinah-scratch/dinah-456-spec2/wt` at the same commit and re-ran the probes its own repairs
depend on. Where a claim was produced by running, the section says so and gives what came back.
The description's own inventory was treated as a hypothesis, and three of its claims turned out
to be wrong. Those three are named in section 2.7.

Round 3 is this amendment, and it was written in `C:/dinah-scratch/dinah-456-spec3/wt` at
`origin/main` `813e0bb0ef539be385c692ccce27395862c264f8`. It carries no fresh design. The
operator approved the contract, answered OQ-1 and overruled D-17, and both of those answers
live in prose no station downstream may edit, which is why the card came back here. What
changed is listed in section 14, and nothing outside that list was reopened.

Round 4 repaired the findings of the third design review, in
`C:/dinah-scratch/dinah-456-spec4/wt` at the same `813e0bb`, after `git fetch origin`
confirmed the trunk had not moved since round 3. It carries no fresh design either. Section
14 lists what it changed.

Round 5 repaired one table cell and took one call the round-4 verification pass named rather
than took. It worked in `C:/dinah-scratch/dinah-456-spec/wt` at `22a35fc`, which is the
commit sections 2.3 to 2.6 are pinned to and the one its own probes had to run against, and
it read the current trunk out of the same clone. `git fetch origin` put the trunk at
`b37486b`, one commit past the `8661604` the pass reported and three past the `22a35fc`
these probes run at: `813e0bb` and `8661604` are
this workstream's own children, and `b37486b` is dinah-448, which touches the documentation
guards and reaches nothing this spec describes. Section 14 lists what round 5 changed, and it
is two things.

Trunk moved between round 2 and round 3, by exactly one commit, and that commit is one of
this card's own children: `813e0bb`, "dinah-459: attach refuses a kind the containment table
gives no mount". So section 2.6's defect is fixed and sections 3.4 and 5.6 now describe
shipped behaviour rather than proposed behaviour. Each of those three sections says so where
it stands, and section 9 marks child card 4 as landed. Every other claim in this spec was
re-read against that commit rather than assumed to have survived it. What the commit reaches
was produced by `git show --stat 813e0bb` rather than by memory: `attach` and its refusal in
`internal/verb/beyond.go`, a new `dinah check` finding in `internal/bench/check.go`,
`bench.KindWorkstream` in `containment.go`, which that same commit's doc comment puts
deliberately outside the containment table, seven new keys plus one changed key across
all eight catalogues under `internal/msg/locales/`, the `attach` footnote in
`internal/guide/guides/references.md`, the quick start, and the tests for all of it. Twenty
files. Two claims this spec makes about the same territory were re-checked against the
post-commit tree rather than carried forward: the references guide still opens its command
section with "Ten commands take a reference" against the fifteen section 2.2 derives, so that
finding stands, and `checklistKinds` was unchanged by that commit, so the alias spellings
were still `oq`, `ac` and `d` on the trunk when round 3 read it. dinah-454 has since landed
as `8661604` and changed them to `questions`, `criteria` and `decisions`, which round 5
records here rather than leaving a sentence that reads as a claim about the trunk today.

# 1. The two languages

Dinah has two selection surfaces. The first is **DinahPath**, the language this card
settles. A **reference** is one expression of DinahPath: it is an address, it names one
entity or one collection of entities by walking the containment grammar from a head, and it
answers "which thing". The second is **query**, which selects cards by field and by recorded
act and answers "which cards match".

DinahPath borrows its surname from XPath and almost none of XPath. It admits no axes, no
node tests, no wildcards over kinds, no string or boolean functions, no unions, no
leading-slash absolute form, and no predicates in any spelling at all. A reader who
meets the name and reasons onward from XPath will be wrong about every one of those, so
section 8 states each refusal and gives the reason it was refused, and section 13.1 makes
delivering that correction alongside the name a requirement on every surface that introduces
it rather than a courtesy.

The operator named the language on 2026-09-09, overruling this spec's own recommendation.
Section 13 records the ruling, the surfaces the name may and may not appear on, and the
superseded reasoning, which is kept because it is what section 13.1 rests on.

The operator raised the relationship between them to a deliverable of this card rather
than a premise of it. Section 12 is that comparison, written out, with the case for each
of three outcomes made before the ruling. The ruling is that the two coexist as separate
languages with a stated seam, and section 12.2 gives the reasoning. Section 13 answers the
naming question he asked alongside it.

What follows from that ruling, and what the rest of this contract rests on: the XPath
steer is taken on the address grammar alone, references gain no field predicate, and
`query` gains no containment step. Section 8 lists every XPath idea considered and says
which were refused and why.

# 2. What is true today

## 2.1 The grammar as it stands

`internal/bench/containment.go` is the one statement of what contains what:

| Kind | Mounts |
|---|---|
| `workbench` | `columns` (column), `cards` (card), `attachments` (attachment, NameField `filename`) |
| `column` | `attachments` (attachment, NameField `filename`) |
| `card` | `comments` (comment), `checklist` (item), `attachments` (attachment, NameField `filename`) |
| `comment` | `attachments` (attachment, NameField `filename`) |
| `item` | nothing |
| `attachment` | nothing |

All four attachments mounts declare the same NameField, so selecting an attachment by its
filename already works below the workbench, a column, a card and a comment alike. Section 8's
borrowing argument rests on that.

A workstream is addressable as `workstream/<slug>` and appears in no row of that table.

`Bench.resolveBelow` and `descend` in `internal/bench/resolve.go` walk it: a head, then a
pair of segments per level, where the pair is a collection name and a selector. `pick`
resolves a selector as an identifier first, then as a one-based position, then as the
collection's NameField value, and refuses `dinah.unknown-path` when none of the three
answers. A name matching more than one member refuses `dinah.ambiguous-name` and reports
every matching position. A collection whose kind is addressed in its own right, meaning
`cards` and `columns`, refuses with an `addressed` detail saying how the thing is named
instead.

Three segments below a card are answered ahead of the grammar. `card` and `card.md` reach
the card's anchor, `journal` and `journal.ndjson` reach its journal, and `payload` below an
attachment reaches the file it wraps. `walkBelowCard`'s own comment says of the first two
that neither is an entity of the containment table, because the anchor is the card itself
and the journal is content. Section 12 turns on that sentence. The three checklist aliases
`oq`, `ac` and `d` narrow the `checklist` collection to one kind. `checklistKinds` in
`internal/bench/resolve.go` declares those three spellings once and `itemKindAlias` derives
the composing direction from that same map, so the spelling a reference resolves by and the
spelling a reference is printed in cannot drift apart. The operator ruled on 2026-09-09, at
dinah-454's Operator Code Review, that the printed spellings become `questions`, `criteria`
and `decisions`, with the three short forms still resolving on input; section 3.1 carries
that as contract and dinah-454 lands it. At `813e0bb` the short forms are still what
resolves and what prints, which is why every probe transcript quoted in sections 2.3 to 2.6
shows them.

## 2.2 Which commands take a reference

Fifteen, and the two rosters that could disagree agree. This was derived rather than
counted by eye, over `internal/verb/definition.go`:

```
python - <<'PY'
import re
src=open('internal/verb/definition.go',encoding='utf-8').read()
g=src[src.index('var guides = map[string][]string{'):]; g=g[:g.index('\n}\n')]
gcmds=[m.group(1) for m in re.finditer(r'^\t"?([A-Za-z]+)"?:\s*\{"references"\}',g,re.M)]
p=src[src.index('var params = map[string][]Param{'):]
cur=None; pcmds=set()
for line in p.splitlines():
    m=re.match(r'^\t(?:"([a-z_]+)"|([A-Za-z]+)):\s*[\{\[]',line)
    if m: cur=m.group(1) or m.group(2)
    if 'Guide: "references"' in line and cur: pcmds.add(cur)
print(len(gcmds), sorted(gcmds)); print(len(pcmds), sorted(pcmds))
PY
```

Both print fifteen: `archive attach attachments cite contents delete edit fail
instructions path rename reopen resolve show verify`. The table in
`internal/guide/guides/references.md` has ten rows, and the sentence opening the section that
holds it, "Which command takes what", says "Ten commands take a reference". The guide's own
opening sentence is "You name a thing to Dinah by writing a reference."; the ten-command claim
is the section's, not the guide's. The five it omits are the checklist-state verbs `cite`,
`resolve`, `verify`, `fail` and `reopen`.

## 2.3 What each command does with a collection reference

Run against a probe workbench built from 22a35fc, on a card carrying one comment, one open
question and one attachment. `path` is the only command that accepts one:

| Invocation | Result |
|---|---|
| `dinah path pb-1/comments` | prints the collection directory, exit 0 |
| `dinah show pb-1/comments` | `dinah.unknown-path nothing in this workbench answers to comments; run dinah ls to see what this workbench carries` |
| `dinah archive pb-1/comments` | the same sentence |
| `dinah delete pb-1/comments --yes` | the same sentence |
| `dinah rename pb-1/comments x.md` | the same sentence |
| `dinah attach pb-1/comments <file>` | the same sentence |
| `dinah contents pb-1/comments` | the same sentence |
| `dinah attachments pb-1/comments` | the same sentence |
| `dinah instructions pb-1/comments` | the same sentence, quoting `pb-1/comments` |
| `dinah edit pb-1/comments` | resolves, hands the directory to the editor, which fails with `Error 0x80070006: The handle is invalid.` and `unreachable exit status 1` |

Eight commands say a collection the workbench plainly has does not exist, and send the
reader to `dinah ls`, which lists cards.

## 2.4 What the surfaces print

`dinah show pb-1` at 22a35fc:

```
Attachments:
  #  File
  -  -----
  1  f.txt

Comments:
  When                  Who
  --------------------  -----
  2026-09-08T10:56:39Z  probe
first comment

Checklist:
  pb-1/oq/1  pending  a question
```

Three kinds of thing a card holds, addressed three ways on one screen. The checklist row
prints what a reader types. The attachment row prints a bare position. The comment row
prints no address at all.

`dinah contents pb-1` prints a reference for all three, and for the checklist item it
prints a different spelling from the one `show` prints:

```
  |-- pb-1/comments/1     comment     first comment  0
  |-- pb-1/checklist/1    item        a question     0
  `-- pb-1/attachments/1  attachment  f.txt          0
```

In the JSON, `verb.AttachmentView` and `verb.ItemView` both carry a `Ref` field.
`verb.CommentView` carries `ID`, `TS`, `Author`, `Body` and its own attachments, and
carries neither a `Ref` nor an ordinal. `cmd/dinah/render.go` draws the attachment table
from `attachment.Ordinal` and never from `attachment.Ref`, so the human surface withholds
an address the machine surface has already computed.

## 2.5 What can be done to each kind

Verified against the library API in `internal/verb` and the CLI command table in
`cmd/dinah/commands.go`, which its own comment calls "the whole surface".

| Kind | Create | Read | Update | Delete | Archive | Restore |
|---|---|---|---|---|---|---|
| workbench | `init` | `workbench`, `workbench get` | `workbench set` on `title`, `slug`, `operator` only | refused | refused | none |
| column | `column new` | `columns`, `show`, `instructions` | none at all | `delete` | `archive` | none |
| card | `add` | `show`, `card get` | `card set` on `severity`, `priority`, `tier` only | `delete` | `archive` | none |
| workstream | `workstream new` | `workstream`, `workstream get` | `workstream set` on `title`, `slug`, `status` | `delete` | `archive` | none |
| comment | `comment` | `show` | none at all | `delete` | `archive` | none |
| item | `file` | `show` | state only, through `cite`, `resolve`, `verify`, `fail`, `reopen` | `delete` | `archive` | none |
| attachment | `attach` | `attachments`, `show` | `attach --replace` for the payload, `rename` for the filename | `delete` | `archive` | none |

**The workbench's read cell, corrected in round 5.** The round-4 form of that cell read
"`workbench`, `show`", which omitted a command that exists and named one that refuses.
`workbench get` is real, and it is one of the six section 5.4 retires, so a reader diffing
the two tables could see only five of the six leave. `show` reaches no spelling of the
workbench reference. Probed at 22a35fc:

```
$ dinah show workbench
unknown-card this workbench carries no card workbench; run `dinah ls` to see the cards this workbench carries
$ dinah show .
unknown-card this workbench carries no card .; run `dinah ls` to see the cards this workbench carries
$ dinah workbench get title
pb
```

`dinah path workbench` resolves the same reference and prints the workbench anchor's path.
That invocation is named rather than quoted, because its output is an absolute path on the
machine that ran it. So the refusal belongs to `show` rather than to the grammar:
`Library.Show` sends a head carrying no `/` to `ResolveCard`, and routes only a composed
reference through `ResolvePath`. The same is true of a workstream, where
`dinah show workstream/addressing` refuses `dinah.unknown-path` while `dinah path
workstream/addressing` prints the directory, and the workstream row's read cell has always
been right to omit `show`. Section 5.4's workbench and workstream rows both name `show`
anyway. Round 5 left those two cells alone: no section of this contract specifies widening
`show`, and section 9 gives the matrix to no child card, so a widening asserted only there
would never be built. The fifth design review has since answered the question this
paragraph referred to it, and the answer belongs here rather than only in a checklist
item. The two cells stand as a summary of a capability rather than as an instruction to
widen `show`, because section 5.2 gives `get` any reference the grammar resolves and
`workbench` is such a reference, so the wrong part is one token in two cells rather than
the capability the cells claim. dinah-455 builds section 3.3's collection acceptance and
leaves head resolution alone. If the widening is wanted later it is one sentence in that
card's description, and it is not a precondition of this contract.

Two things fall out of that table and both are larger than the card's framing suggested.

The first is that **no prose field of any kind is writable by a command that builds a
request.** A card's title and body, a comment's text, a checklist item's text and note, a
column's instructions and a workbench's instructions are all reachable only through `edit`,
which resolves a path and executes the reader's editor and constructs no `verb.Request` at
all. Probed at 22a35fc:

```
$ dinah card set pb-1 title "New title"
dinah.unknown-field Dinah has no field title. The fields a card records are:
severity, priority, tier. ...
$ dinah workbench set instructions "x"
dinah.unknown-key Dinah knows no setting or field called instructions; it knows these
  lang
```

`internal/mcp/tools.go` holds `edit` out of the MCP surface by name, with the reason "opens
a file in the reader's own editor, which needs a terminal this head does not have". That
exemption is correct about `edit`, and it is what makes the gap total: an agent connected
over MCP cannot change one word of prose anywhere in a Dinah workbench.

The second is that **`archive` has no inverse.** `dinah restore` is not a command:
`dinah.unknown-command Dinah offers no command called restore`. `docs/design/format.md`
already declares a `restored` event and describes the structural act, and says in its own
words that `restored` "is not written by any command". For a card the archived half is at
least still addressable through `Bench.ResolveArchivedCard`. For every other kind it is
not: after `dinah archive pb-1/comments/1`, `dinah show pb-1/comments/1` refuses
`dinah.unknown-path`, and the entity has no address at any surface.

## 2.6 A defect found while verifying: `attach` writes attachments the grammar cannot address

**This defect is fixed.** It was found at 22a35fc, specified in section 5.6, filed as child
card 4, and landed at `813e0bb` as "dinah-459: attach refuses a kind the containment table
gives no mount". `contract.NotAttachable` and its catalogue entries are on the trunk, and
the transcript below is the reproduction at 22a35fc rather than a description of today. It
is kept because section 5.6's contract is only readable against the behaviour it replaced.

`Library.Attach` resolves a reference through `ResolveEntity` and never checks the resolved
kind against the containment table. The guide claims otherwise, saying `attach` "takes a
comment or an attachment below a card and takes nothing else below one". Probed:

```
$ dinah attach pb-1/oq/1 f.txt
pb-1  A probe card  [Intake / ready]           <- exit 0
$ dinah path pb-1/oq/1/attachments/1
dinah.unknown-path nothing in this workbench answers to attachments; ...
$ dinah attachments pb-1/oq/1
pb-1/checklist/1 carries 1 attachments.
$ dinah contents pb-1
A probe card (pb-1) contains 3 entities.
```

The attachment is written, is listed by one command, is unaddressable by every command, and
is invisible to the containment walk, which then reports a count short by one. An `item`
mounts nothing in the table, so `descend` correctly refuses the path, and it is the write
that should have been refused.

## 2.7 Where the description is wrong

Three claims in this card's own description do not hold at 22a35fc, and a child card built
on any of them would be built on sand.

- "A card, a column, a workbench and a workstream each have their own update verb." A
  column has none. `runColumn` in `cmd/dinah/commands.go` refuses any first word other than
  `new`, and no `SetColumn` exists in `internal/verb`.
- dinah-455 says the collection refusal comes from `ResolveEntity` failing `KindOfAnchor`.
  For `show` it does not. `Library.Show` sends a composed reference through `ResolvePath`,
  which returns the collection directory successfully, and the refusal is raised when
  `bench.ReadText` fails to read a directory. The other seven commands do go through
  `ResolveEntity`. A fix aimed only at `ResolveEntity` would leave `show`, which is the
  surface the defect was reported on, unchanged.
- dinah-454 says "The JSON already carries an ordinal for a comment." `verb.CommentView`
  carries no ordinal and no reference. It carries the identifier, which the resolver does
  accept as a selector, so an MCP client can name a comment today by an address nothing
  tells it about.

Both dinah-454 and dinah-455 now carry these corrections in their own descriptions, which
section 10 explains.

# 3. The addressing contract

## 3.1 The grammar, written out

```
reference    = head ( "/" step )*

head         = "workbench" | "." | <slug> | <column-ref> | <card-ref>
             | "workstream/" <workstream-ref>

step         = <collection> "/" <selector>     ; a member of a collection
             | <collection>                    ; the collection itself, last step only
             | "card" | "card.md"              ; below a card head, last step only
             | "journal" | "journal.ndjson"    ; below a card head, last step only
             | "payload"                       ; below an attachment, last step only

collection   = "comments" | "checklist" | "attachments" | "columns" | "cards"
             | "questions" | "criteria" | "decisions"     ; printed and accepted
             | "oq" | "ac" | "d"                           ; accepted, no longer printed

selector     = <12-hex identifier> | <positive integer> | <name>
```

`<slug>` heads a path below the workbench without naming the workbench itself.
`<column-ref>` is a column's slug, its title, or its identifier. `<card-ref>` is the card's
number, optionally prefixed with any slug, or the card's identifier. `<workstream-ref>` is
a workstream's slug or its identifier.

Which collections a step may name is decided by the kind reached so far, and by the
containment table alone. The six checklist aliases are `checklist`
narrowed to one kind, and each is legal exactly where `checklist` is legal: `questions`,
`criteria` and `decisions` select open questions, acceptance criteria and decisions, and
`oq`, `ac` and `d` are the older spellings of the same three. Both spellings resolve, the
long ones are what every surface prints, and the short ones are accepted on input and
documented as accepted rather than as deprecated. This is the shape section 4.3 already
rules for the workbench's two spellings, and the operator ruled it for the checklist on
2026-09-09 at dinah-454's Operator Code Review, in these words: "The term 'oq' is handy as
text-speak, but it's a bit non-obvious, especially if we use 'attachments' and 'comments'.
It follows that we should use 'questions' instead of 'oq'." dinah-454 lands the change and
extends it to all three, since fixing one and leaving two trades a general inconsistency for
a stranger one. `checklistKinds` is the one declaration and `itemKindAlias` derives the
reverse from it, so the map is where the spelling is changed and both directions move
together. The kind tokens themselves are untouched: `open_question`, `acceptance_criterion`
and `decision` travel on the wire and are a separate question from how a reference is
spelled. `cards` and `columns` are in
the grammar as collections but every member of them is refused, because a card is named by
its own reference and a column by its slug, which is what `addressedInItsOwnRight` already
enforces.

A selector resolves in a fixed order: identifier, then position, then the collection's
declared name field. The order is fixed so that adding a name field to a collection can
never change what an existing reference means. A position counts from one, in the
collection's creation order, and it is an index into the sorted collection rather than the
ordinal stored in an anchor. Those two numbers stop agreeing after a delete leaves a gap,
which is why `verb.memberPosition` counts the place rather than reading the field, and
every surface that prints a position goes through it.

Dinah accepts no leading slash. Every reference is rooted at a head that names itself, so
there is no absolute form to distinguish from a relative one.

## 3.2 What each form resolves to

| Form | Resolves to | Example |
|---|---|---|
| `workbench`, `.` | the workbench itself | `dinah path workbench` |
| `<slug>` alone | nothing, refused `unknown-card` | `dinah show pb` |
| `<slug>/<collection>/...` | below the workbench | `pb/attachments/1` |
| `<column-ref>` | one column | `doing` |
| `<card-ref>` | one card | `pb-1` |
| `workstream/<ref>` | one workstream | `workstream/addressing` |
| `<head>/<collection>` | every live member of that collection, in creation order | `pb-1/comments` |
| `<head>/<collection>/<selector>` | one member | `pb-1/comments/1` |
| `<card>/questions/<n>` | the nth open question | `pb-1/questions/1`, and `pb-1/oq/1` for the same item |
| `<card>/card` | the card's anchor file | `pb-1/card` |
| `<card>/journal` | the card's journal file | `pb-1/journal` |
| `<attachment>/payload` | the file the attachment wraps | `pb-1/attachments/1/payload` |

Worked examples for every kind the containment table names, plus the workstream:

| Kind | An address that resolves to one |
|---|---|
| workbench | `workbench` |
| column | `doing` |
| card | `pb-1` |
| comment | `pb-1/comments/1`, or `pb-1/comments/<id>` |
| item | `pb-1/questions/1`, or `pb-1/checklist/2`, or `pb-1/checklist/<id>` |
| attachment on a card | `pb-1/attachments/1` |
| attachment on a comment | `pb-1/comments/1/attachments/1` |
| attachment on a column | `doing/attachments/1` |
| attachment on the workbench | `workbench/attachments/1`, or `pb/attachments/1` |
| workstream | `workstream/addressing` |

Both workbench-attachment spellings resolve at 22a35fc and both keep resolving. Which one a
surface prints is ruled in section 4.3.

**Which refusals carry the `dinah.` prefix, and which do not.** `unknown-card` is the one
refusal named above that carries no prefix, and the omission is the contract rather than a
slip. It is a profile-declared name: `docs/spec/core-profile.md` requires it by CORE-VERB-1
and lists it among the names any conforming tool reports, and `internal/contract/contract.go`
holds it as `UnknownCard = "unknown-card"` beside the other sixteen profile names, none of
which take a prefix. `contract.LayerPrefix` is `dinah.`, and it marks a refusal Dinah
introduces beyond the profile, which is what `dinah.unknown-path`, `dinah.ambiguous-name` and
the two tokens section 3.4 mints all are. Verified by running at 22a35fc:

```
$ dinah show pb
unknown-card this workbench carries no card pb; run `dinah ls` to see the cards this workbench carries
$ dinah show pb --json
{
  "outcome": "refused",
  "refusal": "unknown-card",
  "detail": "pb"
}
```

Round 1 of this spec wrote `dinah.unknown-card` in that table cell, which is a token the tool
never emits. The guide card 1 writes copies this table, so a prefixed spelling there would
tell a reader to expect a refusal name that does not exist, and AC-3 is the check that now
covers unprefixed names as well as prefixed ones.

## 3.3 A collection reference is an expression

`<head>/<collection>` is well formed and names every live member of the collection in
creation order. An empty collection is an empty set rather than an error. This is the first
XPath borrowing and it is the one that carries the card.

Whether a command accepts a set is decided per command, because an act that writes to a set
cannot be undone and Dinah has no restore. The table below is the contract, and it is the
answer to dinah-455's "which commands are affected":

| Command | One entity | A collection | Behaviour on a collection |
|---|---|---|---|
| `path` | yes | yes | prints the collection directory. Unchanged from today. |
| `show` | yes | yes | reads every member in creation order, each under its own address heading |
| `contents` | yes | yes | walks the containment tree rooted at the collection |
| `attachments` | yes | yes for `attachments`, empty listing for any other collection | lists the members |
| `edit` | yes | no | refuses `dinah.is-a-collection` |
| `attach` | yes | no | refuses `dinah.is-a-collection` |
| `archive` | yes | no | refuses `dinah.is-a-collection` |
| `delete` | yes | no | refuses `dinah.is-a-collection` |
| `rename` | yes | no | refuses `dinah.is-a-collection` |
| `instructions` | yes | no | refuses `dinah.is-a-collection` |
| `cite` | yes | no | refuses `dinah.is-a-collection` |
| `resolve` | yes | no | refuses `dinah.is-a-collection` |
| `verify` | yes | no | refuses `dinah.is-a-collection` |
| `fail` | yes | no | refuses `dinah.is-a-collection` |
| `reopen` | yes | no | refuses `dinah.is-a-collection` |

The four accepting commands are the four that read without writing and whose answer is
naturally a list. Every writing command refuses, and so does `instructions`, whose answer is
one served chain rather than a list of them. `edit` refuses because handing a directory to a
text editor is what it does today, and section 2.3 records what came back.

## 3.4 The refusals

Two tokens are minted, both lower-case hyphen-joined words under the `dinah.` prefix, the
shape `dinah.unknown-path` and `dinah.not-renamable` already use. Both are Dinah's own
refusals rather than profile names, which is why both take the prefix; section 3.2 states
the rule that decides.

**`dinah.is-a-collection`.** Raised when a command that takes one entity is handed a
collection reference that resolves. It carries `ref`, the reference as typed; `count`, how
many members the collection holds; and `member`, a reference to the first member, which is a
spelling the reader can type. English text under `refusal.dinah.is-a-collection`:

```
{ref} names a whole collection rather than one thing in it; write {member} for one
of them, or `dinah path {ref}` for the collection itself
```

`count` is carried as a detail and stays out of the sentence. The workbench's prose standard
rules out the product's internal vocabulary on a user-facing surface, and "entities" is
internal vocabulary that no message in `internal/msg/locales/en.json` uses today. Writing
"{count} comments" instead would put a count against a noun, which reads as "1 comments" for a
one-member collection and needs a plural rule in each of the eight catalogs; the sentence above
is true at any size and needs none. A machine caller that wants the number reads the detail.

A collection holding nothing carries `count` of `0` and no `member`, and takes a second key,
`refusal.dinah.is-a-collection.empty`, spliced the way
`refusal.dinah.unknown-path.next-addressed` already splices:

```
{ref} names a collection, and it is empty; `dinah path {ref}` gives you the
collection itself
```

**`dinah.not-attachable`.** **Landed at `813e0bb` by dinah-459.** `contract.NotAttachable`
exists, the catalogue carries the key and its splices, and this subsection is the
specification the implementation was built from rather than an outstanding mint. One detail
of the shipped shape is wider than what is written below and the difference is deliberate:
the alternation carries a third, unconditional member, `refusal.dinah.not-attachable.next`,
which is what a kind neither branch names would print. Two named branches with no default is
an alternation that can print nothing, and dinah-459 closed that rather than shipping the
two below exactly as specified.

Raised by `attach` when the reference resolves to a kind the containment table gives no
`attachments` mount. It carries `ref` and `kind`. English text under
`refusal.dinah.not-attachable`:

```
{ref} is {kind}, and Dinah keeps no attachments on {kind}
```

Two next-step splices follow it, chosen on the kind, because the honest advice differs.
`refusal.dinah.not-attachable.next-item`:

```
; attach the file to the card and cite it from here with `dinah cite {ref} attachment <id>`
```

`refusal.dinah.not-attachable.next-attachment`:

```
; an attachment carries bytes rather than other attachments, so attach the file
to whatever this one hangs from
```

Both tokens need an entry in all eight catalogs under `internal/msg/locales/`, translated
for `de` and `hi`, carried as skeletons for `cs`, `id`, `es`, `fil` and `af`, each
translated entry stamped with `msg.Fingerprint` of its English. The workbench document
"Translation staleness contract" governs, and its per-key decision-record rule applies
because these are new keys rather than a skeleton fill.

Two existing refusals keep their current text and are not touched. `dinah.unknown-path`
stays the refusal for a reference naming nothing, and its `next-addressed` splice stays the
refusal for a card or column reached through the collection that holds it. That splice is
the model the new sentences follow, because it already refuses the addressing rather than
the word.

# 4. What a surface owes a reader

## 4.1 The address rule

**Every surface that draws an addressable entity prints, on that entity's own row, a
reference that resolves to it.** A position is not a reference, because a position only
becomes an address once you know the grammar, and the reader this rule protects has not read
the grammar. An identifier alone is not a reference either, for the same reason.

The rule binds both heads and the test differs between them. At a terminal the test is the
operator's: somebody who has never read the references guide gets from what is on their
screen to a command that works. Over MCP there is no screen, and the test is that a client
can name any entity a response draws without composing an address out of parts.

## 4.2 Which spelling gets printed

A reader is handed a spelling to type now rather than a handle to keep. Positions shift once
an earlier member of a collection is deleted, which dinah-451 established, so a printed
positional reference is true when it is printed and may not be true tomorrow.

The two surfaces therefore answer differently, and each answers once.

- **The human surface prints the positional reference and nothing else.** A second column
  carrying a twelve-hex identifier costs width on every row, is not what anybody types, and
  would put two answers to "which one is this" on one line, which is a shape this project
  has already paid to remove once, per the comment on `verb.ItemView.Ordinal`.
- **The machine surface carries both.** Every view of an addressable entity carries `ref`
  and `id`. A client acting now uses `ref`; a client storing a handle uses `id`, which the
  resolver already accepts in the selector slot ahead of the position.

The references guide states in one sentence that a position is a spelling for now and an
identifier is a handle to keep.

## 4.3 The sweep

Every surface that draws an addressable entity, at either head. Each row says what it does
at 22a35fc and what it will do. This is the enumeration the operator asked for under "other
places", and it includes the surfaces that are already right.

| Surface | Entities drawn | Today | Contract |
|---|---|---|---|
| `show <card>`, Comments block | comment | when and who only | a leading `Ref` column carrying `<card>/comments/<n>` |
| `show <card>`, Attachments block | attachment | bare position under `#` | a leading `Ref` column carrying the value already in `AttachmentView.Ref`, and the `#` column goes |
| `show <card>`, Checklist block | item | `<card>/oq/<n>` | the alias spelling only: `<card>/questions/<n>`. It is otherwise the model the other two rows follow. |
| `show <card>`, Links block | card | the linked card's reference | unchanged |
| `attachments <ref>` | attachment | bare position under `#` | the same `Ref` column as the Attachments block, drawn by the same `renderAttachments` |
| `contents <ref>` | every kind | a reference per row | keeps the reference, and changes the item row from `<card>/checklist/<n>` to the aliased `<card>/questions/<n>` |
| `tree` | card | the card's reference | unchanged |
| `ls`, `next`, `query`, `search` | card | the card's reference | unchanged |
| `columns` | column | the column's slug | unchanged |
| `workstream` listing | workstream | the slug, without the `workstream/` prefix | prints `workstream/<slug>`, which is what resolves |
| `log`, `changes` | card, entity | a reference where one is carried | unchanged |
| MCP `show` | comment | `id` only, no `ref` | `CommentView` gains `Ref`, composed by `verb.commentRef`, which already exists and already composes a comment's attachments' addresses |
| MCP `show` | attachment, item | `ref` and `id` | unchanged |
| MCP `contents`, `tree` | every kind | `ref` and `id` | the item node's `ref` changes with the human surface |
| MCP `attachments` | attachment | `ref` and `id` | unchanged |

Two rulings inside that table are worth stating on their own.

**The checklist item's printed spelling is the aliased one, and the alias is the long
word: `<card>/questions/<n>`.** Every spelling keeps resolving, including `<card>/oq/<n>`
and the unaliased `<card>/checklist/<n>`. The aliased form is what `show` already prints and
what dinah-435 established, and it carries the item's kind in the address, so a reader who
copies it out of a listing can still tell an open question from a criterion. `contents`
changes to match rather than the other way round, because one surface printing an address
another surface does not is the inconsistency this card exists to remove, and the
containment walk is the surface with one reader where `show` has many.

The long word rather than the short one is the operator's ruling of 2026-09-09, taken at
dinah-454's Operator Code Review and quoted in section 3.1. It applies to all three
aliases, so `ac` and `d` become `criteria` and `decisions` on the same pass. dinah-454
carries the whole rename, because the address guard that card already builds, which reads
the reference off every drawn screen and hands it back to the tool, is the check that proves
the rename broke nothing, and running that guard on a branch that does not carry the rename
would prove it about the wrong spelling.

**The workbench's own row keeps printing `workbench`, and its children keep composing
against the slug.** `dinah contents workbench` prints `workbench` for the root and
`pb/attachments/1` for a workbench attachment, and both resolve. That was ruled on dinah-151
OQ-9 and it is not reopened here, because changing the seed the children compose against
would rename every address below the workbench and desync the printed form from the
resolver's own default. The references guide states that the workbench has two spellings and
that both resolve.

# 5. The manipulation contract

## 5.1 The rule

**Every field a person authored on any kind is readable and writable by a command that
builds a request, and therefore by an agent over MCP.** A capability that exists only behind
an interactive editor is not a capability an agent has. `edit` stays exactly as it is and
stops being the only route to anything.

## 5.2 The verbs

Two commands, taking any reference the grammar resolves:

```
dinah get <ref> <field>
dinah set <ref> <field> <value|->
```

`get` prints the field's value on a line of its own, and prints an empty line for a field
the entity carries no value on, which is what `card get` does today. `set` writes it,
journals the write on the nearest enclosing journal-bearing entity exactly as `attach` and
`comment` already do, and clears the field when the value is left off. `-` reads the value
from stdin, which is the spelling `dinah comment <card> -` already uses for a body that will
not fit on a command line and which `params` declares as `Display: "text|-"`.

These two subsume `card get`, `card set`, `workbench get`, `workbench set`, `workstream get`
and `workstream set`, and **those six are retired.** The operator ruled on 2026-09-09, on
OQ-1 of this card: "Make the change. Testers understand that this is pre-release."

That was the one user-visible removal on this card, so the reasoning is recorded rather than
left in a resolved checklist item. Three kind-prefixed pairs beside a generic one would be
two spellings for one act, which is the disease this card exists to cure, and DIRT says do it
today rather than leave the debt inside a published surface. What that argument had to beat
is that the six are shipped subcommands of a published CLI with a first external tester.
Dinah's `VERSION` reads 0.1 and the README states that no release promises compatibility with
the dev channel, so nothing is broken that was ever promised, and the operator's answer says
the tester understands the channel he is on. Agent Design Review had demoted an earlier
ruling here to a question, on the ground that this pipeline does not remove published surface
on its own authority. That ground is satisfied: the authority ruled.

What retirement means, concretely, and it is the same at both heads:

- `card get`, `card set`, `workbench get`, `workbench set`, `workstream get` and
  `workstream set` stop being accepted. Each refuses through whatever its parent command
  already refuses an unknown first word with, and dinah-460 states in its own handoff which
  refusal that is per command rather than assuming the three are alike; `runColumn` is the
  worked precedent, refusing any first word but `new`.
- The three kind-shaped MCP tools `card`, `workbench` and `workstream` lose their `get` and
  `set` actions, which `doCard`, `doWorkbench` and `doWorkstream` select on today. Their
  remaining actions are untouched, and the two capabilities leave under one rule rather than
  surviving on the head a person cannot see.
- The references guide, the quick start and the verbs guide lose the retired spellings
  wherever they name them. dinah-460 sweeps for them rather than editing the places it
  remembers, because the quick start's replayed transcripts are checked against live output
  and a stale line there fails a test rather than merely reading wrong.
- Nothing else goes. The bare `dinah workbench` and `dinah workstream` listings stay, since
  they list rather than read a field, and `add`, `column new` and `workstream new` stay as
  the creating verbs.

Over MCP the generic pair arrives as two tools, `get_field` and `set_field`, taking `ref` and
`field`, and `value` on the setter.

## 5.3 The field sets

A field set belongs to a kind, is declared in `internal/bench` beside that kind, and is
reached through one function:

```go
// FieldsOf reports the fields of a kind a person wrote and may rewrite, in the
// order a listing prints them. A kind the grammar does not name reports no
// fields.
func FieldsOf(kind string) []string
```

`bench.CardFields` and `bench.WorkstreamFields` already exist in that shape and are folded
into it rather than duplicated. The sets:

| Kind | Fields |
|---|---|
| `workbench` | `title`, `slug`, `operator`, `instructions` |
| `column` | `title`, `slug`, `kind`, `tier`, `capacity`, `instructions` |
| `card` | `title`, `body`, `severity`, `priority`, `tier` |
| `comment` | `body` |
| `item` | `text`, `state`, `note`, `owner` |
| `attachment` | `filename`, `description` |
| `workstream` | `title`, `slug`, `status` |

Every name there is a key the kind's anchor already carries, or the name of the prose below
its front matter. `instructions`, `body` and `text` name that prose rather than a
front-matter key, and `FieldsOf` is the only place the distinction is recorded.

The column set is the one the implementer derives rather than copies, because the column
anchor carries fields nobody types, which are `operator_owned`, `awaiting_outside`,
`reject_to` and the order key. The six above are the five `dinah column new` accepts, minus
`before`, which is a position rather than a field, plus `title` and the prose. The
implementer derives the set from `bench.Column` and the `column` entry in
`internal/verb/definition.go`'s `params`, puts the result in `FieldsOf`, and states in the
guide that the guide's table is generated from it rather than kept in step by hand.

`set` on a field outside the kind's set refuses `dinah.unknown-field`, which already exists
and already lists the set it recognises. No new token.

Four writes keep the guards they have today rather than being widened by this rule.
`slug` on any kind stays subject to the existing uniqueness and shape checks. `tier` on a
card routes through `Library.SetCardTierAt`, so its column-relative resolution and its
override recording are not bypassed. `state` on an item is writable only to a value the
item's own kind admits, which is the same set the five checklist verbs move it through, and
writing it that way records the same journal event the verb records. `filename` on an
attachment routes through `Library.Rename`, so the payload moves with the name.

## 5.4 The full matrix

After this contract lands, per kind:

| Kind | Create | Read | Update | Delete | Archive | Restore |
|---|---|---|---|---|---|---|
| workbench | `init` | `show`, `get` | `set` | refused, deliberate | refused, deliberate | not applicable |
| column | `column new` | `show`, `get`, `instructions` | `set` | `delete` | `archive` | `restore` |
| card | `add` | `show`, `get` | `set` | `delete` | `archive` | `restore` |
| workstream | `workstream new` | `show`, `get` | `set` | `delete` | `archive` | `restore` |
| comment | `comment` | `show`, `get` | `set` | `delete` | `archive` | `restore` |
| item | `file` | `show`, `get` | `set`, and the five state verbs | `delete` | `archive` | `restore` |
| attachment | `attach` | `show`, `attachments`, `get` | `set`, `attach --replace` | `delete` | `archive` | `restore` |

Every absence in that matrix is deliberate and each has its reason recorded. The workbench
refuses `delete` and `archive` because it is the root that holds the archive mirror, so an
archived workbench would have nowhere to go; `Library.Archive` and `Library.Delete` already
refuse it by name. The workbench has no `restore` because it is never archived. Nothing else
is absent.

**The six retired spellings, stated rather than left to a reader to notice.** Compare this
matrix against section 2.5's and six command names have gone: `card get`, `card set`,
`workbench get`, `workbench set`, `workstream get` and `workstream set`, along with the
`get` and `set` actions of the three kind-shaped MCP tools. Section 5.2 carries the ruling
and what retirement means. The round-2 design review was right that a reader stepping from
one table to the other would otherwise see the six vanish with nothing saying why, and this
paragraph is the answer to it.

The matrix itself is not the place to say so, and that is a decision rather than an
oversight. A cell reading "`set`, was `card set`" would carry one release's history into a
table whose job is to be true after the release, and it would go stale the moment somebody
who never knew `card set` read it. The table answers what can be done to a kind. Section 2.5
is the before-picture, this is the after-picture, the paragraph above names the difference,
and each of the three does one job. There is a second reason to keep the shape: AC-2 reads
every cell of this matrix against the kinds in `containment.go`, and a seventh column or a
parenthetical inside a cell would give that check a second thing to mean.

## 5.5 `restore`

`restore <ref>` moves an entity's directory out of the archive mirror back to the position
it was archived from, appends `restored` to the same journal `archive` appended `archived`
to, and refuses `dinah.exists` when a live entity already occupies the slot.
`docs/design/format.md` already specifies the event, the note it carries, which is the
entity's own identifier, and the structural act, and already says the event is written by no
command. So this implements a published contract rather than minting one.

An archived entity needs an address before `restore` can take one. `--archived` becomes a
marker flag on `restore`, `show`, `path` and `contents`, resolving the same reference
against the archive mirror. `Bench.ResolveArchivedCard` already does this for cards and is
generalised to the whole grammar. Positions inside the mirror count the mirror's own
members, which is the only reading that lets `dinah show --archived pb-1/comments` list what
is there to restore.

## 5.6 `attach` and the containment table

**Landed at `813e0bb` by dinah-459.** This subsection is the contract that card was built
from, and it is kept as the record of what was specified. It shipped with one carve-out that
is not written below and that is correct: `dinah attach <attachment> --replace` stays legal,
because replacing an attachment's bytes writes nothing below the attachment. `beyond.go`
decides the refusal and the writing branch from one expression, `req.Replace && entity.Kind
== bench.KindAttachment`, so the exception cannot drift away from the rule it excepts.

`attach` refuses `dinah.not-attachable` when the resolved kind mounts no `attachments`
collection, which is `item` and `attachment`. The check reads
`bench.MountOf(kind, bench.AttachmentsDir)` rather than naming the two kinds, so a kind
added to the table gains attachments with no second edit.

The table's asymmetries are deliberate and the table says so in its own comment.

- **An item mounts nothing on purpose.** An item is a sentence with a state, and its
  evidence reaches it by citation rather than by containment. `Library.Cite` records a
  citation carrying a scheme and a target on the item, and the attachment scheme's target is
  the twelve-hex identifier of an attachment on the card. One file cited by three criteria
  is stored once, which containment could not do.
- **A comment mounts attachments on purpose.** A comment is prose somebody may need to
  evidence in place, and its attachments belong to the comment rather than to the card.
- **An attachment mounts nothing** because it wraps bytes, and `payload` is the one segment
  that may follow one.
- **A workstream stays outside the table** because it is a membership rather than a
  container. Cards join and leave a workstream, and a card is not contained by one. It sits
  in DinahPath under its own `workstream/` prefix, and this contract does not move it into
  containment.

The attachments this defect has already written under checklist items are found by
`dinah check`, which gains a finding naming an `attachments` directory under a kind the
table gives no mount. The finding reports and does not repair, because the bytes are
somebody's and deleting them silently is worse than the defect.

# 6. The machine surface

Every ruling above is true of the MCP head as well as the CLI, and one rule already enforces
most of it: **a command that builds a request is served as a tool unless `toolExemptions`
names it with a reason.** That rule lives in `internal/mcp/tools.go`, `roster_test.go` holds
it in both directions, and `get_field`, `set_field` and `restore` join the roster under it.
`edit` and `path` stay exempt with their existing reasons unchanged.

What the rule does not constrain is the reason. Any sentence in the map value satisfies it, so
a future exemption can be argued on any ground at all and no test notices. An earlier draft of
this section proposed to close that by requiring every reason to turn on the command needing a
shell or a filesystem, on the belief that the nine current entries each already say so. They do
not. All nine, read at 22a35fc:

| Command | Reason as written | Ground it actually gives |
|---|---|---|
| `path` | resolves a filesystem path for a shell to consume | a shell or the filesystem |
| `edit` | opens a file in the reader's own editor, which needs a terminal this head does not have | a shell or the filesystem |
| `init` | creates a workbench in a directory, which is a filesystem act | a shell or the filesystem |
| `extract` | copies a workbench definition out to a directory | a shell or the filesystem |
| `reshape` | reads its new column layout from a definition file or another workbench's directory | a shell or the filesystem |
| `config` | writes the user's own machine settings, which travel with the person rather than the workbench | the machine rather than the workbench |
| `mcp` | starts this head, so a tool for it would be the server offering to start itself | this head itself |
| `guide` | served as a resource rather than a tool, because a guide is read rather than run | the protocol serves it another way |
| `help` | the surface's own tools/list carries every tool's schema and description | the protocol serves it another way |

Five turn on a shell or the filesystem and four do not, so the tightening that draft proposed
would have refused four shipped entries on the day it landed. The comment above `tools` states
the single-ground version in prose and then carves `workbenches` out of it by name, which is
the same overreach admitting its own counter-example.

**The rule this card lands instead.** An exemption carries a declared ground beside its prose,
drawn from a closed set the code declares. `toolExemptions` becomes a map to a struct of two
fields, `ground` and `reason`, and the four grounds in the table above are declared as
constants: `shell-or-filesystem`, `machine-not-workbench`, `the-head-itself`, and
`protocol-serves-it`. The roster test gains one assertion, that every entry's ground is one of
the declared constants, and it keeps the two it already makes. A fifth ground needs a card
arguing for it, which is the point: the guard's job is to make an unargued exemption
impossible, not to decide that only one argument can ever be made.

The ground is a declaration rather than a fact recovered from prose, and that is why it is a
new field instead of a tighter test over the sentence. A test grepping the reason for "shell"
or "filesystem" would pass `config` the day somebody wrote "the user's own shell profile" into
it and fail `init` the day somebody reworded it, and neither result has anything to do with
whether the exemption is sound. The prose stays, because it is what a reader wants; the ground
is what the guard reads.

`workbenches` needs no ground, because it is served rather than exempt. The comment above
`tools` loses the two paragraphs arguing it as an exception to a sentence that no longer says
what they answer, and keeps the sentence pointing a reader at `toolExemptions` for the current
set.

Two asymmetries this closes.

Prose was writable at a terminal through `edit` and not writable at all over MCP. After
`set`, both heads write prose the same way, and `edit` becomes a convenience for a person
with an editor rather than the only door.

A comment could be named over MCP only by a client that already knew the grammar, since the
response carried an identifier and no address. After `CommentView.Ref`, a client names any
entity it can see straight out of the response.

# 7. What the guide has to say

`internal/guide/guides/references.md` becomes the contract of record and gains the grammar
of section 3.1, written for a reader rather than as a production rule; the fifteen-row
command table replacing the ten-row one, with the per-command accept sets of section 3.3
folded in as a column rather than as footnotes; the seam paragraph of section 12.4, with its
reciprocal in `query.md`; the two-spellings sentence, that a position is a spelling for now
and an identifier is a handle to keep, and that both resolve; and the two sets of
alternative spellings, the workbench's pair and the checklist item's six. The four existing
footnotes stay where a detail does not fit a cell.

**The guide's opening paragraph is where DinahPath is introduced to a user, and it is the
only user-facing text that names the language.** Section 13.2 rules that surface by surface
and 13.1 states what the paragraph owes. Concretely, that paragraph names DinahPath in its
first sentence. Its second sentence carries two general statements and four examples:
DinahPath addresses things rather than filters them, and every reference is one address
rooted at a head that names itself, so it carries no brackets, no wildcards, no functions
and no axes, and a reader who wants filtering is sent to `dinah query`. The four are
examples of the two statements rather than the whole of what section 8 refuses, and the
sentence must not be written as though they were the whole. Nothing in it may say "only",
or "the four things", or anything else that closes the list.

**Four examples rather than section 8's seven, and the two general statements are what
keep that honest.** D-22 records the ruling. "Addresses things rather than filters them"
is what refuses predicates, node tests and functions. "One address rooted at a head that
names itself" is section 8's own reason for refusing the leading-slash absolute form, and
it refuses unions in the same breath, since two references joined by a bar are not one
address. Axes and wildcards over kinds follow from that second statement as well. Section
8 refuses an axis on the ground that the card is the head of the reference the reader
just typed, which is that statement restated, and a wildcard over kinds names a class of
things rather than one address, exactly as a union does. A reader who takes those two
statements away does not reason onward from XPath into any of the seven, which is the
whole job the correction has. Seven named constructs in the sentence a user meets first
would fail the operator's screen test, because a warning nobody finishes reading protects
nobody, and the four named are the four whose spelling a reader arriving from XPath is
likeliest to type. What this paragraph does not do is give
the guide's reader the complete refusal list, and no shipped surface carries it, because
section 8 is a spec rather than documentation. The guide owes the two statements, the four
examples and the route to `dinah query`. The guide's title stays "References" and its
key in the `guides` map stays `references`, because that key is what `dinah guide references`
takes and what the MCP head serves the guide as a resource under. The rest of the guide goes
on saying "reference", which is what a reader is writing.

The refusal names the guide quotes follow section 3.2's prefix rule, so `unknown-card` is
written bare and Dinah's own refusals keep the `dinah.` prefix.

The table's roster is derived rather than typed. A test compares the command names in the
guide's table against the fifteen the `guides` map names, and fails on either a command in
the map with no row or a row naming a command the map does not carry. That is what stops the
ten-against-fifteen drift recurring, and a hand-maintained list would not.

# 8. XPath, borrowed and refused

Four ideas were named in the operator's steer. Each was put to his own screen test: does
somebody who has never read the guide get from what is on their screen to a command that
works.

This section is also the correction the language's name owes. DinahPath is named for XPath
and takes almost nothing from it, so section 13.1 requires every surface that introduces the
name to deliver the refusals below alongside it. The seven refusals are axes, node tests,
wildcards over kinds, string and boolean functions, unions, the leading-slash absolute form,
and predicates in any spelling. Nothing in the operator's naming ruling reopened any of
them.

**Borrowed: a collection reference names many entities.** Section 3.3. It passes because
`pb-1/comments` is spelled out of a heading the reader is already looking at, and because
`path` already accepts it, so half of it exists.

**Borrowed in the form it already has rather than as syntax: selection by a value the entity
carries.** The name arm of `pick` already selects an attachment by its filename, and the
filename is in the File column of the table on screen, so
`dinah show pb-1/attachments/notes.md` passes the test with no bracket in it. Extending that
to another collection needs a `NameField` in the containment table, and no other collection
has a natural name: a comment has an author and a timestamp, and an item has a sentence. So
the idea is honoured by what exists and this card adds no name field.

**Borrowed as a flag rather than as syntax: the descendant step.** "Every attachment under
this card" genuinely has no spelling, because `Library.Attachments` reports one entity's own
attachments and never those of anything it contains, and it says so in its own comment. The
answer is `dinah attachments <ref> --deep`, which walks the containment table from the
reference and lists every attachment below it, each under its own address. A flag appears in
`dinah help attachments`, which is a screen. `pb-1//attachments` appears on no screen and
adds a second separator to a published address format. The flag is served over MCP as an
argument on the `attachments` tool.

**Refused: predicates.** No `[...]` in a reference, in any form. The positional predicate
`comments[1]` is a second spelling for `comments/1` and buys nothing. The field predicate
`comments[author=alka]` is the one the whole of section 12 is about, and it is refused
there, on the ground that it does not subsume `query`, fails the screen test, and needs a
format change to reach the fields it would want.

**Refused: axes.** No `parent::`, no `ancestor::`, no upward step of any spelling. Naming
the card a comment belongs to is a real gap in the abstract and not one in practice, because
the card is the head of the reference the reader just typed and the first segment of the
reference every surface prints. An agent holding `pb-1/comments/1` holds `pb-1`.

**Refused: node tests and wildcards over kinds.** No `*`, no `comment()`, no `pb-1/*/1`. A
reader who wants everything a card holds runs `dinah contents pb-1`, which answers the
question a wildcard would ask, prints an address per row, and needs no syntax.

**Refused: string and boolean functions.** No `contains()`, no `starts-with()`, no
`count()`, no `last()`. Every one of them is a filter, and filtering is `query`'s job.
`dinah query --json` piped into a downstream tool is the documented answer for anything the
query language will not say, and `query.md` already says so.

**Refused: the absolute and relative distinction.** XPath's leading `/` marks a path as
absolute. Every Dinah reference is rooted at a head that names itself, so a leading slash
would carry no information and the grammar admits none.

**Refused: unions.** No `|` between references. A reader who wants two things runs two
commands, and the surfaces that answer over many entities take a collection reference
already.

# 9. The child cards

This card produces the contract; these produce the code. Each is filed into Intake and
joined to the addressing workstream.

1. **The references guide states the whole contract, and a test holds its command table to
   the code.** Sections 3.1, 3.2, 7, the rulings in 8, and the seam paragraph of 12.4 with
   its reciprocal in `query.md`. Prose plus a derivation test. It goes first because every
   card below it is checked against what it says. This is dinah-457.
2. **Every surface that draws an addressable entity prints its address, and the checklist
   aliases become words.** Section 4, and the alias ruling of section 3.1. This is dinah-454,
   widened from the comment row to the sweep and then again, by the operator's ruling of
   2026-09-09, to the `questions` / `criteria` / `decisions` rename. The rename rides on this
   card rather than following it because this card's own address guard, which reads the
   reference off every drawn screen and hands it back to the tool, is what proves the rename
   broke nothing. It has landed, at `8661604` on the trunk, so `checklistKinds` now prints
   `questions`, `criteria` and `decisions` and section 4's sweep describes shipped behaviour.
3. **A collection reference is an expression, and the commands that cannot take one say so.**
   Sections 3.3 and 3.4's `dinah.is-a-collection`. This is dinah-455, and section 10 says
   what changes about it.
4. **`attach` refuses a kind the containment table gives no attachments mount, and `check`
   reports the attachments already written under one.** Section 5.6. This is dinah-459, and
   it has landed, at `813e0bb` on the trunk.
5. **`get` and `set` reach every field of every kind, at both heads, and the six
   kind-prefixed spellings are retired.** Sections 5.1 to 5.3, and section 6, because this is
   the card that puts `get_field` and `set_field` on the MCP roster and so it is the card
   that reshapes `toolExemptions` to carry a declared ground. This is the largest child and
   the one the operator's asymmetry complaint is about, and it is dinah-460. OQ-1 is answered
   in favour of retirement, so this card builds one outcome rather than two.
6. **`restore` is the inverse of `archive`, and an archived entity has an address.**
   Section 5.5. This is dinah-461.

Card 5 does not depend on cards 1 to 4 and can run beside them. Card 6 wants card 5's
journalling in place, so it follows.

Section 12's ruling adds no seventh card. Coexistence is what the tree already does, so the
only work it creates is the seam paragraph and its reciprocal, which card 1 carries. The one
loose thread section 12.3 turns up is recorded there and belongs to whoever next touches
`query`, rather than to this workstream.

# 10. What this contract changes about dinah-454 and dinah-455

Both keep their numbers, and both are corrected in their own descriptions rather than by a
comment. Whoever works one of them reads its description and may never open this card, so a
comment reaches the wrong reader, and a comment does not satisfy AC-4, which requires each
child card to name its own spec section in its own description.

**dinah-454 grows, and section 4 specifies it.** It was filed as the comment row on
`dinah show`. It becomes the sweep of section 4.3, which is fifteen rows at two heads, and it
inherits three rulings it did not carry: that the printed spelling is positional and the
identifier is carried only in JSON, that the attachment row loses its `#` column rather than
gaining a ref column beside it, and that `contents` changes its checklist spelling to match
`show`. Its own claim that the JSON already carries an ordinal for a comment is wrong at
22a35fc. `verb.CommentView` carries `ID`, `TS`, `Author`, `Body` and `Attachments`, and
`dinah show pb-1 --json` prints a comment as those fields, with no ordinal and no ref:

```
"comments": [
  {
    "id": "ddc5a8ae50fc",
    "ts": "2026-09-08T11:34:55Z",
    "author": "spec",
    "body": "first comment"
  }
]
```

The description now carries that correction and names section 4.

**dinah-455 survives, with a different answer.** The XPath framing does not dissolve it. It
dissolves the half where a collection reference is malformed, because `<card>/comments`
becomes a legal expression. What it leaves is the harder half: eight commands refuse a
collection today with a sentence saying it does not exist, and after this contract four of
them accept it and eleven refuse it deliberately, with a refusal that says what the
reference names and what to type instead. That is more work than rewording one message
rather than less.

The card also carried a wrong cause. `Library.Show` sends a composed reference through
`Bench.ResolvePath`, which returns the collection directory, and refuses only when
`bench.ReadText` fails on that directory, at `internal/verb/read.go:773-780`. So `show` never
reaches `ResolveEntity` or `KindOfAnchor` at all, and a fix aimed at those two would leave the
reported surface answering exactly as it does today. The description now carries that
correction and names sections 3.3 and 3.4.

**dinah-460's description has been corrected twice and now states the retirement as
decided.** Round 2 rewrote it to say the retirement was the operator's open question and to
tell the implementer to build the generic pair either way, which was right while OQ-1 was
open. OQ-1 is answered, so round 3 rewrote that paragraph again: the six kind-prefixed
subcommands and the three kind-shaped tools' `get` and `set` actions are retired, the card
builds one outcome, and the description quotes the ruling and points at section 5.2 for what
retirement means. A description telling an implementer to hedge against a question the
operator has already answered is worse than no note at all, because it reads as current.

**dinah-454's description gains the alias rename.** The operator folded the rename into that
card at its Operator Code Review, superseding dinah-464, so its description now carries the
ruling, the three spellings, and the rule that the short forms keep resolving on input.
Sections 3.1 and 4.3 here are the contract it works against.

Beyond those description edits, none of dinah-454, dinah-455 or dinah-460 is re-specified
here.

# 11. Out of scope

- **What a user-facing command is called.** dinah-443 is settling vocabulary, and this card
  takes the names as it finds them. Where the two meet, dinah-443 wins on the word and this
  card wins on the shape.
- **The query language's own grammar.** Section 12 rules on the relationship between the two
  languages and changes nothing inside `query`: its twelve fields, its refusal to bracket or
  negate, and its documented smallness all stand exactly as they are.
- **Archiving a workbench.** Refused today, and this contract records the refusal as
  deliberate rather than proposing to lift it.
- **The workstream's place in the containment table.** Section 5.6 rules that it stays
  outside, and moving it in would be a format change with its own card.
- **The kind tokens `open_question`, `acceptance_criterion` and `decision`.** Section 3.1's
  alias ruling changes how a checklist item is addressed and changes nothing about the tokens
  that travel on the wire and appear on the machine surface. The operator named the two as
  separate questions when he ruled, and this card keeps them separate.
- **Repairing the attachments already written under checklist items.** Card 4 reports them
  through `dinah check` and does not delete them.
- **Widening the MCP exemption grounds.** Section 6 declares four and closes the set. A fifth
  is a card of its own, and no exemption on this contract needs one.

# 12. `query` and DinahPath: one language or two

The operator asked for this comparison to be made seriously rather than settled in a
sentence, and for the ruling to show its reasoning. Three outcomes are on the table:
DinahPath subsumes `query` and `query` goes away, the two coexist as separate
languages, or there is one semantics with two spellings where `query` desugars into a
reference expression.

## 12.1 What each language actually reaches

The two do not overlap the way the framing suggests, and this is the fact the ruling turns
on.

DinahPath addresses seven kinds: the workbench, a column, a card, a comment, a
checklist item, an attachment and a workstream. It filters nothing. It cannot reach a
recorded act at all, because a journal is not an entity of the containment table.
`<card>/journal` resolves to the file, and `walkBelowCard`'s own comment says of that
segment and the anchor beside it that neither is an entity of the table, since the anchor is
the card itself and the journal is content.

`query` selects cards, and only cards. `Library.Query` returns `Matches`, whose members are
`CardView`, and nothing else in the language yields any other kind. Of its twelve fields,
seven describe the card as it now stands and five describe a recorded act read out of the
journal, and `query.md` states the binding rule for those five: they must all be satisfied
by one and the same recorded act, which is why a card that entered Doing in June and was
commented on in August does not match `entered:doing at>=2026-08-01`.

So neither language contains the other. `query` reaches facts about acts that no reference
can address, and DinahPath reaches six kinds that no query can select.

## 12.2 The three outcomes

**Outcome A: DinahPath subsumes `query`, and `query` goes away.**

The case for it is real. XPath's model does unify navigation and predicate, so
`workbench/cards[column=doing][state=ready]` is `dinah query "column:doing state:ready"`
with the implied "every card" written down, and one language means one guide, one parser and
one thing for a reader to learn. DIRT's instinct is to prefer one general mechanism to two
special ones.

Three things defeat it.

It does not subsume. Five of `query`'s twelve fields select on a recorded act, and the five
bind to one and the same act. Expressing that as a predicate needs journal events to be
addressable entities below a card, which the containment table deliberately does not have,
and then needs an existential quantifier over them, because `[actor=alka and
event!=commented]` has to bind one act rather than two. So outcome A is not a reshaping of a
surface. It is a format change adding a kind, followed by a predicate language with
quantification. Section 8 refuses a large language, and this is the largest thing on the
list.

It fails the operator's screen test. `dinah query "entered:doing at>=2026-08-01
at<2026-09-01"` reads left to right in words a person already uses, and it is the shape
GitHub and Jira search already use, which dinah-135 gave as its reason for choosing it. The
reference form of the same question carries a step, a quantifier and three comparisons, and
no screen anywhere prints it.

It is a breaking change to a published surface that buys no capability. `query` ships at the
terminal and as an MCP tool. DIRT counts what escapes into an interface, and removing
`query` costs every caller while buying uniformity alone.

**Outcome C: one semantics, two spellings, `query` desugaring into a reference expression.**

The case for it is the strongest of the three, because it keeps `query`'s terse spelling for
the common case, makes the two surfaces unable to disagree since one evaluator answers both,
and gives a caller who needs what `query` cannot say a general form to fall back on rather
than a pipe into another tool.

It inherits every cost of outcome A, because the general grammar underneath still needs
journal events as entities and still needs quantification, and it adds a desugaring nobody
can see. And it forces the question the dispatch names, which is whether `query`'s restraint
belongs to the shorthand or to the whole language. Both branches are bad, and naming which
is what exposes it. If no `or`, no negation, no bracketing and twelve fields belong to the
whole language, the general grammar cannot say the things generality was for, and the
outcome delivers a large machine that is not allowed to do anything. If they belong to the
shorthand alone, then a query string and its desugared form behave differently under one
reading, and the shorthand becomes a leaky subset a reader has to learn twice, which is
worse than two languages honestly labelled.

**Outcome B: two languages with a stated seam. This is the ruling.**

They are different questions over different populations. A reference answers "which thing",
over seven kinds, by containment. A query answers "which cards match", over one kind, by
field and by recorded act. Nothing is duplicated across the seam, because DinahPath gains no
field predicate and `query` gains no containment step, so there is no pair of syntaxes doing
one job.

The cost is honest and worth naming rather than hiding: a reader learns two small things
instead of one large thing. Two small things with a stated seam is what this tool already
does elsewhere, since `ls` answers a positional question, `query` answers the rest, and
`search` answers over prose. The collection expression of section 3.3 narrows the gap
without merging the two, because `dinah show <card>/comments` is DinahPath answering over
many entities, which is the thing outcome A wanted and the part of it that
costs nothing.

What would reopen this: if journal events became addressable entities for some other reason,
outcome C's cost would fall a long way and the question should be asked again. Nothing on
the board proposes that today.

## 12.3 What was already ruled, and one premise that turned out false

dinah-135 compared four candidate grammars, chose the qualifier grammar, and put to the
operator whether it enters the shared contract or ships as Dinah's own tool surface. Its
recommendation was the second, on the ground that the core profile specifies no read verb
beyond history and instruction serving, and that five of the fields read a card's recorded
history, which the profile's boundary table places outside the contract. That row reads:
"Measurement and reporting over a workbench's history | out | History is already in the
core, and a measurement is a reading of it. Fixing the measurements would freeze somebody's
dashboard into the contract."

The ruling held. `query` appears zero times in `docs/spec/core-profile.md`, checked by
count, and the conformance suite names no `CORE-` statement about it. DinahPath is not in
the profile either.

That corrects a premise this comparison was handed. `query` is not a contract surface that
another implementation is written against, so the DIRT cost of reshaping it is smaller than
it first appears. It remains a published surface at two heads, which is why outcome A's
third count stands on its own rather than resting on the contract.

One loose thread turned up while checking this, and it belongs to whoever next touches
`query` rather than to this card. `Library.Query`'s doc comment says its refusal order is
normative "so that a second implementation's output is comparable", which is a contract
claim about a surface dinah-135 ruled out of the contract. One of those two sentences is
wrong and somebody should decide which.

## 12.4 The seam, as the guides will state it

One paragraph in `references.md` and its reciprocal in `query.md`, saying the same thing
from each side. Neither names DinahPath, per section 13.2: this paragraph is written for a
reader choosing which of two things to type, and it does that in plain words on both sides.

> You name a thing with a reference and you find things with a query. A reference is an
> address: it starts at a card, a column, the workbench or a workstream, and walks down to
> what that holds, so `wb-1/comments/1` names one comment and `wb-1/comments` names all of
> them. A query asks which cards match a condition, including conditions about what has
> happened to them, and it answers with cards. If you know which thing you want, write a
> reference. If you want Dinah to find the cards, write a query. Neither one does the
> other's job, so a reference takes no conditions and a query names nothing below a card.

# 13. What the language is called

The operator asked for a name, weighed the recommendation this section made, and ruled
against it. This section records the ruling, what it changes, and the reasoning it
overturned, because the reasons the name was resisted are the reasons the two rules under
13.1 and 13.3 exist.

**The ruling, Paul, 2026-09-09: "DinahPath blessed as the pathing name."**

The language is **DinahPath**. An instance of one is still a **reference**, which is the
word the guide, the help text and the `ref` parameter all use today, and the act of using
one is still **addressing**, which is the word section 4.1's address rule already uses. Only
the language's own name changes, and the two words a reader meets on a screen are unchanged.

**A name was earned before a name was chosen, and those were two questions.** The language
is separately learnable: it has a grammar, a guide of its own, refusals of its own, and
after section 3.3 its own notion of an expression that yields many entities rather than one.
That settles the first question in favour of naming it. The second question was whether a
new word was needed, given that the instance already had one and only the language lacked
one, and the recommendation preserved in 13.5 answered no. The operator answered yes.

## 13.1 The name owes a correction, and the correction is delivered where the name is

DinahPath takes its surname from XPath, and a reader who knows XPath will arrive expecting
constructs DinahPath does not have. Section 8 refuses seven of them by name: axes, node
tests, wildcards over kinds, string and boolean functions, unions, the leading-slash
absolute form, and predicates in any spelling at all. That refusal list is older than
the name, the operator's ruling did not touch it, and section 8 spends most of its length on
it. The name now advertises a resemblance section 8 exists to deny, so the denial has to
travel with the name rather than sit ten sections away from it.

**Every surface that introduces DinahPath states what DinahPath does not admit, in the
sentence or the paragraph immediately following the one that names it.** That is a
requirement on three identified pieces of text rather than a principle for somebody to
apply:

- **This spec.** Section 1 introduces the name, and the paragraph directly under it carries
  the seven refusals and points at section 8. Written.
- **The references guide's opening paragraph**, which section 7 specifies and which child
  card 1 writes. Its first sentence names DinahPath. Its second carries two general
  statements and four examples: that DinahPath addresses things rather than filters them,
  that every reference is one address rooted at a head that names itself, and that it
  therefore carries no brackets, no wildcards, no functions and no axes, sending a reader
  who wants filtering to `dinah query`. The two general statements are what make four
  examples an honest correction rather than a truncated one, and section 7 gives that
  reasoning. The seam paragraph of section 12.4, already bound for that guide, is where the
  reader lands.
- **Any design document that introduces the name.** The same obligation, dischargeable by
  pointing at section 8 rather than by restating it, since a design document has section 8
  to hand.

**The three surfaces owe different amounts, and AC-11 checks each against what it owes
rather than against one number.** A surface that names DinahPath with no correction beside
it fails, and the failure belongs to the name rather than to the surface, which is why the
rule is recorded in the naming section rather than left to whoever writes the sentence.
Beyond that, a surface with room to restate the refusals states all seven, which binds this
spec's section 1 and any design document that restates rather than points. A design
document discharging by the pointer is checked for the pointer. The guide's opening
paragraph is checked against the sentence section 7 specifies, which is the two general
statements, the four examples and the route to `dinah query`. D-22 records that ruling and
the round-3 criterion it replaces, which required seven constructs on every surface and so
condemned two of the three surfaces this same section specifies.

## 13.2 Which surfaces carry the name, ruled one at a time

The superseded recommendation confined "the reference grammar" to design documents and the
references guide's opening paragraph, so a reader who never opened a design document carried
one word rather than two. A proper noun is a stronger claim than a descriptive phrase, so
the question is asked again for DinahPath and each surface is ruled rather than left to
whoever types next.

| Surface | Carries `DinahPath` | The ruling's reason |
|---|---|---|
| Design documents, this spec included | yes | This is where the language is discussed as a language rather than used, and it is the audience the name was minted for. |
| The references guide's opening paragraph | yes | A reader learning the language deliberately is the one reader the name helps, and this is the paragraph they arrive at. Subject to 13.1. |
| The references guide's title and filename | no | The guide is keyed `references` in the `guides` map in `internal/verb/definition.go`; that key is what `dinah guide references` takes and what the MCP head serves the guide as a resource under. Renaming the guide renames a machine surface in order to gain a word the guide's own first sentence already carries. |
| The rest of the references guide, and every other guide | no | The guide's body tells a reader what to type. It says "reference" because a reference is what they are writing, and repeating the language's name through a how-to is vocabulary for its own sake. |
| Help text | no | Help says what a command takes, and what a command takes is a reference. The parameter stays `ref` and its description stays a description of the argument. A reader running `dinah help show` wants the argument rather than the language. |
| Refusal text | no | A refusal reaches a reader at the moment they are stuck, and its job is to say what they typed, what it named, and what to type instead. Teaching a proper noun there spends the one sentence a stuck reader will certainly read on something that does not unstick them. |
| Any machine-surface field, key, tool name or JSON member | no | The wire carries `ref` and `id`, section 4.2 rules that it carries both, and neither is renamed. A name on the wire is a compatibility commitment, and this name buys no capability that would justify making one. |
| The seam paragraph of section 12.4 and its reciprocal in `query.md` | no | The seam is written for a reader deciding which of two things to type, and it does that in plain words on both sides. Naming one side alone would make the two look unequal, and naming both would put two proper nouns into a paragraph whose whole subject is the difference between an address and a condition. |

## 13.3 What stops the name spreading by use

A ruling that lives only in this section is a ruling the next contributor never reads, and a
proper noun spreads by being convenient. The check is mechanical, and child card 1 lands it
beside the guide it governs.

A test greps the whole tree for the token `DinahPath`, case-insensitively so that
`dinahpath` and `DINAHPATH` are caught too, and holds every hit against a declared allowlist
of file paths. The allowlist is a fixture the test reads rather than a list written inside
the test, so adding a design document is an edit to data. The test fails in both directions:

- A hit in any file not on the allowlist fails. That is what refuses the name in a message
  catalogue under `internal/msg/locales/`, in a Go string literal, in `params` or `guides` in
  `internal/verb/definition.go`, in a tool name or schema under `internal/mcp/`, and in any
  guide other than `references.md`.
- Zero hits in `internal/guide/guides/references.md` fails. That is what stops the name
  quietly disappearing from the one user-facing surface it is ruled onto.

The second arm is the half worth arguing for, because a one-directional guard on a name is
satisfied by deleting the name, and a guard anybody can satisfy by deletion is not a guard.
What the test cannot see is a paragraph that names DinahPath and omits the 13.1 correction,
because that is a judgement about prose. AC-11 is the human read that covers it, and it says
in its own text that it is a human read and what that costs.

## 13.4 The alternatives, and why each was rejected

Each reason below names something true of that candidate alone, which is the standard AC-10
sets. `DinahPath` won on the operator's authority rather than on an argument made here, and
these are the arguments that were on the table when he ruled.

`path` is the obvious candidate and the worst one, because `dinah path` already resolves a
reference to a filesystem path, so the word names the answer on the very surface where it
would have to name the question. One word meaning two things on one surface is the disease
this card is treating.

`DPath` abbreviates the product to an initial, which is XPath's habit rather than Dinah's,
and an initial says nothing at all to a reader who has not been told what the D stands for.
DinahPath spells the product out and loses nothing by it, which is the whole difference
between the two.

`CardPath` names the wrong root. The language addresses the workbench, a column, a
workstream, and an attachment hanging off any of them, none of which is a card or below one,
so a card-shaped name understates the reach by four kinds and would have to be explained
away in the paragraph that introduced it.

`selector` is taken, and taken for something smaller. `pick` calls one segment of a
reference a selector and the grammar in section 3.1 names that production `<selector>`, so
promoting the word would make it name both a segment and the whole.

`locator` borrows a promise from another vocabulary and then breaks it. In the sense the
letters of URL made familiar, a locator says where a thing is to be fetched from, which is
the question `dinah path` answers rather than the question a reference asks. It fails the
way `path` does and one step further from Dinah's own words, and it appears nowhere in this
repository today.

`expression` is taken on the machine surface. A tier expression is the relative or absolute
form an `--at` value is written in, `internal/bench/tier.go` calls it that throughout, and
the `tier_overridden` event publishes what the reader typed under the key `expr`. Section
3.3 also uses the word for one form inside DinahPath, the collection reference that yields
many things rather than one, so promoting it would make one word mean both a part and the
whole.

`addressing` was offered as the fallback proper noun and is not rejected. It survives as the
name of the act, which is what section 4.1 and the addressing workstream already call it,
and the ruling above leaves it exactly there. What it lost is the language, and it lost that
to the operator's ruling rather than to any defect in the word.

## 13.5 The superseded recommendation, kept rather than deleted

The recommendation this section carried into Operator Design Review was that an instance
stays a reference, that the language is **the reference grammar**, that the act is
addressing, and that no new noun enters the product; that the words appear in design
documents and in the references guide's opening paragraph and nowhere else, so a reader who
never opens a design document carries one word rather than two; and that `DPath`,
`DinahPath` and `CardPath` be rejected together, on the ground that they borrow XPath's
register and advertise a resemblance section 8 spends most of its length refusing.

The operator overruled the conclusion. He did not overrule the observation underneath it,
and nothing in the ruling makes DinahPath less XPath-like than the reference grammar was.
That observation is why 13.1 exists and why AC-11 checks it, so the superseded reasoning is
load-bearing here rather than merely historical, and deleting it would leave 13.1 reading
like an unexplained ceremony.

D-17 on this card carries the same pair in the same shape, so the two stores agree.

# 14. What rounds 3, 4 and 5 changed, and what they did not

This card has been back to Spec three times. Round 3 carried two operator answers rather
than a defect, and both live in prose no station downstream may edit, which is the whole
reason for that return trip. Round 4 repaired the third design review's findings, and round 5
repaired one cell the verification pass caught. No round carried fresh design, and this
section is the closed list of what each moved, in both stores. A reviewer can bound their
read to it.

## 14.1 Round 3, the operator's amendment

**OQ-1, answered in favour of retiring the six.** Section 5.2 now records the ruling and what
retirement means at both heads. Section 5.4 states the six departures beneath the matrix and
says why the matrix itself is the wrong place to say it. Sections 9 and 10 tell dinah-460 to
build one outcome, and dinah-460's own description was rewritten to match rather than left
telling its implementer to hedge against an answered question.

**D-17, overruled: the language is DinahPath.** Section 13 is rewritten to argue for and
record what was decided, and it keeps the superseded recommendation in 13.5 rather than
erasing it, because 13.1 rests on the observation that recommendation was built from. Section
13.1 makes the XPath correction travel with the name on three named surfaces. Section 13.2
rules eight surfaces one at a time. Section 13.3 gives the guard that stops the name spreading
by use. Sections 1, 5.6, 7, 8 and 12 were swept so that no section still calls the language
"the reference grammar", and section 12's heading changed with them.

**The checklist alias rename, which is dinah-454's ruling and reaches this spec's quoted
addresses.** Sections 2.1, 3.1, 3.2, 4.3, 9, 10 and 11 carry it. The probe transcripts in
sections 2.3 to 2.6 keep the old spellings, because they are records of what a binary built
at 22a35fc printed and AC-5 checks them against that commit.

**Trunk moved by one commit and it was one of this card's own children.** Sections 2.6, 3.4,
5.6 and 9 record that dinah-459 landed, and 3.4 records the one place the shipped shape is
wider than what was specified.

**What was not reopened.** The two headline rulings stand untouched and were not
relitigated: D-16, that `query` and DinahPath coexist because neither contains the other, and
the collection-reference framing of section 3.3. So do the address rule, the sweep, the field
sets, `restore`, the exemption grounds, the child-card split and every refusal in section 8.
Four acceptance criteria were added, AC-11 to AC-14, and three decisions, D-19 to D-21. Each
of the four checks a paragraph this amendment wrote, and each of the three records a call this
amendment took. No existing criterion was weakened. AC-10 was widened to cover the two
candidates that stopped being rejected as a group and to read the surfaces ruling off section
13.2's table. In the checklist store, four items were re-worded off the superseded
name, AC-7's text, D-5's note, D-7's text and D-16's text, and D-4's text and note were
re-worded onto the ruled alias spellings while keeping D-4's original observation, dated so
that it reads as history. That is six checklist edits, and the round-3 form of this
paragraph named two of them.

## 14.2 Round 4, the third design review's findings

Round 4 was written at the same commit as round 3, `813e0bb`, confirmed by running
`git fetch origin` at the start of the round rather than assumed. Trunk had not moved.
dinah-454 was then in Implement carrying the `oq` to `questions` rename and had not
landed, so `checklistKinds` read `oq`, `ac` and `d` on the trunk and no address this spec
quotes was touched by it. It landed on 2026-09-09, squashed as `8661604`, and the trunk now
prints `questions`, `criteria` and `decisions`. That paragraph keeps round 4's observation
in the past tense rather than being rewritten to today's trunk, which is round 5's call and
D-24 gives the reasoning. What the landing changes is the reason the probe transcripts in
sections 2.3 to 2.6 still show the short spellings: they show them because AC-5 pins them to
the binary built at 22a35fc, and no longer because the trunk agrees with them.

**The defect: a criterion no writer following the spec beside it could satisfy.** AC-11
required every surface introducing DinahPath to name all seven of section 8's refusals,
while section 7 and section 13.1 specify the references guide's opening sentence as naming
four and let a design document discharge by pointing at section 8, which names none. The
implementer of dinah-457 would have written what section 7 specifies and then failed AC-11
at Test. The criterion is the half that moved and D-22 gives the reasoning, of which the
short form is that widening the guide's sentence to seven closes only one of the two
mismatches, and that a seven-construct warning in the sentence a user meets first fails the
operator's screen test. Section 13.1 now states what each of the three surfaces owes.
Section 7 gains the second general statement that makes four examples honest, and a rule
that the four must not be written as an exhaustive list.

**The same shape was searched for elsewhere, and found once more.** AC-4 required all six
child cards to sit in Intake, while section 9's own bullet 4 records that dinah-459 has
landed on the trunk. Read against the board on 2026-09-09, dinah-459 is in Acceptance and
dinah-454 is in Implement, so the criterion already failed on two cards for the reason that
the workstream is being worked, and it gates Merge, so the failure would have been read at
Test long after the position it asserts stopped being true. AC-4 now checks existence,
workstream membership and the section each child card names, and it says in its own text
what dropping the column check costs. D-23 records that. Every other criterion was read
against the prose it checks and no third instance was found: AC-1's fifteen derived command
names match section 3.3's fifteen rows exactly, AC-2's six columns are section 5.4's six
columns and no cell of that matrix is blank, AC-8's exemption ground agrees with section
5.2's arrival of `get_field` and `set_field` over MCP, and AC-12, AC-13 and AC-14 name the
same sets their sections state.

**Three sentences repaired, each a minor.** The preamble said `813e0bb` put
`bench.KindWorkstream` "in the containment table", and the constant sits in
`containment.go` outside that table, with a doc comment saying it is deliberately absent
because a workstream is a membership rather than a container. Section 12's opening sentence
read "the DinahPath subsumes", an article the superseded phrase took and a proper noun does
not. Sections 1 and 13.1 wrote section 8's seventh refusal as "field predicates", narrowing
a refusal section 8 states as predicates in any spelling and refuses positionally as well,
at `comments[1]`.

**AC-5 was amended too, because it is the defect's own class.** It said the transcripts
reproduce when run against "the commit the spec names", and this spec names two: `22a35fc`,
where the transcripts were captured, and `813e0bb`, where rounds 3 and 4 were written. A
tester rebuilding at the trunk would report true records as failures, because dinah-459
landed between the two and changed `attach`. AC-5 now names `22a35fc` and says why.

**The narrowing was repaired in both stores, not only in the spec.** D-17's note carried the
same list with the same "no field predicates at all", and it now reads "no predicates in any
spelling at all". Nothing else in D-17 changed, and the note is otherwise the superseded
record it has been since round 3.

**Round 4's edits, listed so a reviewer can hold this section to them.** In the spec: the
preamble gained a round-4 paragraph and lost the containment-table claim, section 1 and
section 13.1 lost the word "field" from the seventh refusal, section 7 gained the second
general statement and the paragraph arguing for four examples, section 13.1's second bullet
and its AC-11 paragraph were rewritten, section 12's opening lost an article, and section 14
was renumbered into 14.1 and 14.2. In the checklist store: AC-4, AC-5 and AC-11 had their
text and their note rewritten, D-17's note had one clause corrected, and D-22 and D-23 were
added.

**What round 4 did not touch, which is everything else.** The two headline rulings stand,
along with the operator's two answers that round 3 carried, section 13.2's surfaces table,
section 13.3's guard, every other checklist item, and every child card's description. No
criterion was added, none was removed, and none was weakened except AC-4, which says so in
its own text.

## 14.3 Round 5, one cell and one call

The verification pass ran all fourteen criteria and recorded evidence on each. Twelve
verified, AC-4 was closed from the board afterwards, and AC-14 failed. Round 5 exists for
that failure and for one call the pass named rather than took.

**The failure was the contract's, not the check's.** AC-14 proves the six retirements by
diffing section 2.5's command names against section 5.4's, and the diff yielded five.
`workbench get` is a real command, and section 5.4 says it is going away, but section 2.5's
workbench row never listed it, so the before-picture was short by one and the criterion
correctly refused to call six five. That cell now reads "`workbench`, `workbench get`", and
the paragraph beneath the table carries the probe. AC-14 is not amended: the criterion was
right and the table was wrong.

**The same cell also named `show`, which refuses.** `show` reaches no spelling of the
workbench reference, and round 5 removed it on the strength of a probe rather than leaving a
false entry in a section headed "What is true today". Section 5.4's workbench and workstream
rows still name `show`, which is a claim about the contract rather than about the trunk, and
the paragraph beneath section 2.5's table says why round 5 did not touch it.

**Every other cell of both tables was read the same way.** What that read found in section
5.4 is recorded in D-25, along with the reason round 5 left it there rather than widening its
own charter.

**The call: a dated record is not re-dated.** Section 14.2 said dinah-454 "has not landed",
which was true when round 4 wrote it and is false of the trunk now. The sentence is re-tensed
so it reads as the observation of a round rather than as a claim about today, and one dated
sentence records what has since changed. Nothing round 4 concluded was rewritten. D-24
carries the reasoning. The preamble carried the same claim in different words and was
repaired the same way, which a search for the phrase would have missed.

**Round 5's edits, listed so a reviewer can hold this section to them.** In the spec: section
2.5's workbench read cell, the probe paragraph beneath that table, section 14.2's dated
sentence, the preamble's copy of the same claim, section 9's bullet 2, which left dinah-454
unmarked while bullet 4 already marked its sibling as landed, the preamble's round-5
paragraph, and this subsection with section 14's heading and opening. In the checklist store:
D-24 and D-25 were added, and AC-14's state moved from failed back to pending for Test to
re-run. Nothing else in either store was touched, and no ruling was reopened.
