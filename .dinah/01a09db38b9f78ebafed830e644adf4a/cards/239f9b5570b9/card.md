---
title: A card's comments print with no address, so nothing on screen tells a reader how to name one
column: b69abf918c42
state: ready
severity: major
priority: now
workstreams:
  - 994787601ae6
links:
  - kind: spawned_from
    to: c7d5ff7728c1
---
Reading a card shows its comments as a table of when and who, and nothing in that table is an address. A reader who wants to open, edit or delete one has to guess that the first comment written is `1`, and the tool never says so. The operator hit this directly: `dinah show cpe-1` printed one comment, and there was no step from that screen to `dinah show cpe-1/comments/1`. He called it a big hole and he is right.

The same screen already solves this for one of the three things a card carries, and solves it three different ways. Checklist items print their own reference as the row's first column, which is what dinah-435 was for, and a reader can type what they see. Attachments print a bare position integer, which is enough to work out the reference from if you already know the grammar, and is not the reference. Comments print neither: there is no address on the row and no count to infer one from.

So the fix is not one column on one table. It is to settle what a read surface owes a reader about an entity it draws, and then say it the same way everywhere. The grammar is already uniform (`<card>/comments/1`, `<card>/attachments/1`, `<card>/oq/1`); only the rendering is not.

**This card is specified by dinah-456 section 4, "What a surface owes a reader".** Read that section before starting. It settles the three questions this card was filed with, and it widens the card from the comment row on `dinah show` to a sweep of fifteen reference-taking commands at two heads (section 4.3). Three of its rulings the card did not originally carry: the printed spelling is the positional one and the identifier is carried only in JSON; the attachment row loses its `#` column rather than gaining a ref column beside it; and `contents` changes its checklist spelling to match `show` rather than the reverse.

**One claim in the original filing is wrong and is corrected here.** It said "The JSON already carries an ordinal for a comment; check whether it carries a reference." It does not. At trunk 22a35fc, `verb.CommentView` carries `ID`, `TS`, `Author`, `Body` and `Attachments`, and `dinah show pb-1 --json` prints a comment as `id`, `ts`, `author` and `body` with no ordinal and no ref. So the machine surface is worse off than the terminal rather than level with it, and the JSON needs both a ref and, if the ordinal is wanted, the ordinal.

Related work: dinah-435 established the pattern for checklist items on this same screen. dinah-451 established, on the extension side, that a position is not a name: positions shift once an earlier entity in the collection is deleted, so any address this card prints has to be understood as a spelling a reader types now rather than a handle they store.

Out of scope: the refusal message for a collection reference, which is dinah-455.

**The checklist aliases become words, and that rename is part of this card.** The operator ruled at this card's Operator Code Review on 2026-09-09: "The term 'oq' is handy as text-speak, but it's a bit non-obvious, especially if we use 'attachments' and 'comments'. It follows that we should use 'questions' instead of 'oq'. At least the grammar would be more consistent and discoverable." All three short forms change, not only the one he named: `oq` becomes `questions`, `ac` becomes `criteria`, and `d` becomes `decisions`. Fixing one and leaving two trades a general inconsistency for a stranger one, and `d` is a single letter naming a whole entity kind while everything else a workbench holds is addressed by a word. The three short forms keep resolving on input and are documented as accepted rather than deprecated, which is the shape this card already settled for the workstream's two spellings. The kind tokens `open_question`, `acceptance_criterion` and `decision` do not change; they travel on the wire and this ruling is about addressing. dinah-464 is superseded, because the operator chose to fold the rename in here rather than let it follow.

`checklistKinds` in `internal/bench/resolve.go` is the one declaration and `itemKindAlias` derives the composing direction from it, so changing that map moves both directions together. What is wide is downstream: every surface this card already sweeps, the references guide, the replayed transcripts checked against live output, and reader-facing text across the eight message catalogues. This card's own address guard, which reads the reference off every drawn screen and hands it back to the tool, is what proves the rename broke nothing, and that guard living here is why folding the rename in is cheap. Anything that stores an address rather than composing one should be found before it surprises somebody: dinah-451 established that a position is a spelling for now rather than a handle to keep, and an alias is the same kind of thing.

**dinah-456 sections 3.1 and 4.3 are the contract for the rename**, alongside section 4, which already specifies the sweep.

## Specification

This card implements section 4 of dinah-456, "What a surface owes a reader". That section is
settled and this spec does not re-derive it. Section 4.1 states the rule, section 4.2 rules
that a human row prints the positional reference alone while the machine payload carries both
the reference and the identifier, and section 4.3 rules that a checklist item prints in its
aliased form and that `contents` changes to match `show`. All of that binds here.

# 1. Where the facts below come from

Round 3. Everything below stands against `origin/main` at
22a35fca90c254625071b9b7eca237131cdce3b0, which `git fetch origin` at the start of this pass
confirms is still the tip, so it is the commit every round of this card and every review of it
has read. Round 3 worked in a worktree at `C:/dinah-scratch/dinah-454-spec3/wt`, detached
there, and removed it. Round 2's own worktree, binary and throwaway workbench under
`C:/dinah-scratch/dinah-454-spec2` were removed when that pass ended.

Round 3 changed four sections and five checklist items and nothing else. Section 7.4's quotation
of the false quick-start sentence was re-wrapped by hand, which made AC-12's containment
assertion pass before any implementer touched the document; the sentence is now quoted as the
document wraps it and the assertion flattens whitespace before it tests. Section 7.2 cited
`verb.CrossHeadIdentical` for a property that set does not carry and that `contents` is outside
of, and now cites the same-command path through `runTree`, `runContents` and `treeRows`.
Section 7.4's affected-lines list was attributed to a grep that produces neither of two of its
entries. Section 4's account of dinah-459 is replaced by the diff. Section 7.1's machine-side
guard now excludes test sources, as section 2.2's derivation already did.

Where a claim below rests on reading rather than on a run, the sentence says so.

Round 1 carried three defects of its own and this section names them rather than quietly
repairing them. Its arming recipe for AC-9 did not compile. Its roster of the human head's
tables was produced by a regular expression that missed five sites, and the split it reported
of that roster was a hand count. Its test mechanism could not read the one table that both
`dinah tree` and `dinah contents` draw. Sections 2.1, 7.2 and 7.5 are where each is settled.

# 2. The two rosters, derived rather than counted

dinah-456 section 4.3 gives a fifteen-row sweep. This card does not trust that list, and it
does not extend it by eye either. The set is produced by the tree, one command per head.

## 2.1 The human head

The roster this card needs already exists in the tree, and round 1 built a worse one beside
it. `cmd/dinah/output_check_test.go` declares `renderSitesInSource`, which parses the
package's non-test sources with `go/parser`, finds every call to `s.table` and `s.tableLines`
outside `table.go`, and returns each one keyed by file, enclosing function, the local its
table literal was bound to, and an ordinal. For each site it also reads the catalogue keys the
site's own `columns` field declares.

That walk reads both of the two constructors a `columns` field can call, which is what makes
it the complete roster and what a search for `s.columns` alone is not. The constructors are
`(*session).columns`, which composes one heading key per column, and `listColumn`, which
declares a single headingless column. That the set of constructors is closed at two is
produced by a command rather than by reading:

```
grep -rn ') \[\]tableColumn' cmd/dinah/*.go | grep -v _test.go
```

At 22a35fc it prints `cmd/dinah/table.go:224` for `listColumn` and `cmd/dinah/table.go:231`
for `(*session).columns`, and nothing else. A third constructor is a case the new guard has to
be taught about, and section 7.1 gives it an assertion of its own so that teaching it is a
failing test rather than an oversight.

`TestEveryTableSiteIsRegistered` already holds `sweptBlocks()` to that walk's output in both
directions: an unregistered site fails, and a stale entry fails. So the roster is enforced in
the tree today rather than merely available, and this card's completeness guard reads the same
function instead of parsing again.

Round 1 used this instead:

```
python - <<'PY'
import re,glob
pat=re.compile(r's\.columns\(\s*"([a-z]+)"((?:\s*,\s*"[a-z-]+")+)\s*\)')
...
PY
```

It prints 29 sites, and the head draws more than that. Five tables are built through
`listColumn` and carry no `s.columns` call at all: the two identifier listings a
`dinah check` repair prints (`cmd/dinah/render.go:807` and `:815`), the findings table
(`:834`), and the two listings a refusal draws (`:962` and `:979`). Round 1's roster could not
see any of them, and it then reported a split of the 29 by hand that was wrong as well. Both
faults are gone rather than corrected: the roster is a function the suite already runs, and no
split is asserted in prose anywhere in this spec. Section 3 names the swept sites, section 9
names the exempt ones, and the guard asserts that between them they account for every site the
walk returns.

A table is not the only thing this head prints. A sentence carrying an entity's address is the
other shape, and those are found in the catalogue rather than in the code, because every one
of them is a message with a `{ref}` placeholder:

```
python -c "import json;d=json.load(open('internal/msg/locales/en.json',encoding='utf-8'))['entries'];print(sorted(k for k,v in d.items() if '{ref}' in v['text'] or '{reference}' in v['text']))"
```

That prints eleven keys: `attachments.empty`, `attachments.header`, `card.line`,
`card.line.workstreams`, `columns.new.line`, `contents.empty`, `contents.header`,
`refusal.dinah.unknown-field.show.reference`, `tree.header`, `tree.header.filtered` and
`workstream.line`. Every one of them is fed from `verb.EntityRef.Ref`, from a card's own
reference, or from `Workstream.Ref()`, so section 5.5's single composer reaches all of them
without a per-site edit.

## 2.2 The machine head

A view is an addressable entity's payload when it carries the entity's identifier. Extracting
every struct in `internal/verb` that declares an `ID` field, and reporting whether it also
declares `Ref`, gives the machine half:

```
python - <<'PY'
import re,glob
struct=re.compile(r'^type (\w+) struct \{',re.M)
for f in sorted(glob.glob('internal/verb/*.go')):
    if f.endswith('_test.go'): continue
    src=open(f,encoding='utf-8').read()
    for m in struct.finditer(src):
        i=m.end(); depth=1
        while depth and i<len(src):
            depth += (src[i]=='{') - (src[i]=='}'); i+=1
        fields=re.findall(r'^\t(\w+)\s',src[m.end():i],re.M)
        if 'ID' in fields:
            print(f"{f}:{src[:m.start()].count(chr(10))+1} {m.group(1)} Ref={'Ref' in fields}")
PY
```

At 22a35fc that prints thirteen structs, of which five carry no `Ref`:

| Struct | Carries `Ref` |
|---|---|
| `verb.CardView`, `verb.AttachmentView`, `verb.ItemView`, `verb.TreeNode`, `verb.SearchHit`, `verb.WorkstreamView`, `verb.ChangeEvent`, `verb.GoneEntity` | yes |
| `verb.CommentView` | no |
| `verb.ColumnView` | no |
| `verb.ReshapeColumn`, `verb.ReshapeRetirement`, `verb.RemintReport` | no |

Round 1 wrote "four carry no `Ref`" over a table listing five, and put `verb.AttachmentListing`
in the yes row. That type declares no `ID` field, so the script never returns it and it is not
one of the thirteen. Both are corrected above and nothing downstream moved, because the four
exemptions section 7.1 declares are the correct four once `CommentView` gains its field.

`verb.CommentView` is the gap this card was filed for. The other four are ruled on in section
9 and left alone.

The MCP head serves those same structs. `internal/mcp/tools.go` maps `show` to `readShow`,
`contents` to `readContents` and `attachments` to `readAttachments`, each of which returns the
library's own value, so a field added to a view reaches both heads at once and no MCP-side
edit exists to make.

# 3. The sweep

Every surface either head draws an addressable entity on. "Today" was produced by running at
22a35fc unless the row says otherwise.

| Surface | Entity drawn | Today | Contract |
|---|---|---|---|
| `show <card>`, Comments block | comment | `When` and `Who`, no address | a leading `Ref` column carrying `<card>/comments/<n>` |
| `show <card>`, Attachments block | attachment | a bare position under `#` | a leading `Ref` column carrying `AttachmentView.Ref`, and the `#` column goes |
| `show <card>`, Checklist block | item | `<card>/oq/<n>`, and an empty cell for an item of an undeclared kind | every row carries a reference, the aliased form where the kind has one and `<card>/checklist/<n>` where it does not |
| `show <card>`, Links block | card | the linked card's reference | unchanged |
| `attachments <ref>` | attachment | a bare position under `#` | the same `Ref` column, drawn by the same `renderAttachments` |
| `contents <ref>` | comment, item, attachment, card, column | a reference per row, and `<card>/checklist/<n>` for an item | the item row changes to the aliased spelling and the rest is unchanged |
| `tree` | card, column | a reference per entity row and the group's value per grouping row | unchanged |
| `ls`, `next`, `query` | card | the card's reference | unchanged |
| `columns` | column | the column's slug | unchanged |
| the legal-moves table served under `instructions` | column | the destination column's reference | unchanged |
| the columns listing a refusal draws, and the columns an ambiguous pull carries | column | the column's reference | unchanged |
| `search` | card, column, workstream | the entity's reference | unchanged except for the workstream, which section 5.5 respells |
| `search` | workbench | the workbench's slug, which resolves to nothing | prints `workbench` |
| `workstream` listing | workstream | the bare slug, under a `Slug` heading | prints `workstream/<slug>` under a `Reference` heading |
| `changes` | card, workstream | the entity's reference | unchanged except for the workstream's spelling |
| `status` holding and blocked blocks, `workstream get` member list | card | the card's reference | unchanged |
| MCP `show` | comment | `id`, no `ref` | `CommentView` gains `Ref` |
| MCP `show` | attachment, item, card, link | `ref` and `id` | the item's `ref` becomes total, per section 5.2 |
| MCP `contents` | every contained kind | `ref` and `id` | the item node's `ref` changes with the human surface |
| MCP `attachments` | attachment | `ref` and `id` | unchanged |
| MCP `search_cards`, `changes` | card, column, workstream, workbench | `ref` and `id` | the workstream and workbench spellings change with the terminal's |

Three of those rows are new in round 2, and each is a site round 1's roster could not see or
mis-classified. The legal-moves table draws `move.Ref` in its first cell, which is a column's
own reference, so it draws an entity and round 1 was wrong to file it as drawing none. The two
refusal listings draw `column.Ref()` one per row, through `listColumn`, which is the shape the
old roster missed entirely. None of the three changes what the tool prints. All three become
cases of the round trip, so the rule they already satisfy is asserted rather than assumed.

Section 9 lists what was examined and left alone.

## 3.1 The three defects the sweep turned up that the card did not carry

Each was found by running at 22a35fc, and each is the defect the card was filed for standing
on a surface nobody had listed.

**An item of a kind the format does not declare draws a checklist row with no address.** The
run: a hand-written `item.md` carrying `kind: risk` at checklist position 3 draws
`             pending  a risk` under `dinah show`, with the reference cell empty.
`verb.ItemView.Ref` is left empty for such an item, and its doc comment gives the reason that
"nothing would resolve a reference composed from one". That reason is wrong.
`dinah show pb-1/checklist/3` answers the item, because `walkBelowCard` narrows by kind only
for the three aliases and otherwise descends the collection unnarrowed.

**A workbench that matches a search prints an address that resolves to nothing.** The run:
with `haystack prose` in the workbench anchor's body, `dinah search haystack` prints
`workbench  pb  pb  framing  haystack prose`, and `dinah show pb` answers
`unknown-card this workbench carries no card pb`. `internal/verb/search.go:264` fills the
hit's `Ref` with `l.Bench.Slug`, and a bare slug names the workbench nowhere, because
`Bench.resolveBelow` accepts the slug as a head only when something follows it.

**A workstream prints an address the reference-taking commands do not accept.** The runs:
`dinah workstream` prints `addressing` under a `Slug` heading, and `dinah contents addressing`
answers `unknown-card`, while `dinah contents workstream/addressing` succeeds. The reverse
also holds, and it is the half that makes this awkward: `dinah workstream get addressing`
answers `Addressing`, and `dinah workstream get workstream/addressing` answers
`dinah.unknown-workstream`. So a workstream has two spellings today, each accepted by a
different half of the surface, and printing either one alone strands the reader at the other
half. Section 5.5 rules on both directions.

# 4. What does not change, and why the card is not larger

The grammar is unchanged. This card adds no reference form, no refusal, and no command. It
changes what surfaces print, what one view carries, and which of two existing spellings a
workstream is printed in.

`dinah.is-a-collection`, the refusal for a collection reference, is dinah-455. The references
guide is dinah-457. `attach` refusing a kind with no attachments mount is dinah-459. `get` and
`set` over any reference are dinah-460. `restore` and `--archived` are dinah-461.

**Where this card and dinah-459 meet.** dinah-459 is in code review as of 2026-09-08 and may
merge before this card is implemented. It sits on the branch
`dinah-459-attach-writes-attachments-under-kinds-the-containment-grammar-cannot-address`, whose
tip is 4a8c66a and whose merge base with trunk is 22a35fc, the same commit this spec reads. It
refuses `dinah attach` against a kind the containment table gives no attachments mount and adds
a `dinah check` finding for the strays already written. The overlap below is the diff rather
than a reading of the card:

```
git diff --stat 22a35fc origin/dinah-459-attach-writes-attachments-under-kinds-the-containment-grammar-cannot-address
```

which touches twenty files and lists `cmd/dinah/render.go` among none of them. Four facts
follow, and the first is the one that matters most to this card.

- dinah-459 does not touch `cmd/dinah/render.go` at all. It adds no table site, it declares no
  third function returning `[]tableColumn`, and it changes no cell. So the attachments block
  this card rewrites at `cmd/dinah/render.go:641` is this card's alone, the roster
  `renderSitesInSource` returns is undisturbed, and neither arm of AC-8 is affected by the
  merge order.
- It edits `docs/quick-start.md`, which section 7.4 also edits. Its single hunk is
  `@@ -1227,8 +1227,8 @@` in the catalogue listing at the foot of the document, which is below
  every line section 7.4 cites and below the block section 7.4 inserts, so no line number in
  that section drifts and the two edits do not meet.
- It edits all eight locale catalogues, which section 6 also edits. The keys it adds are
  `check.attach.3`, `check.attach.4`, `check.attachments-without-a-mount` and four
  `refusal.dinah.not-attachable` keys. None is a `column.` key, so no key this card adds,
  renames or retires is touched, and AC-10 is unaffected either way the merge falls.
- It adds finding kinds, and the findings table at `cmd/dinah/render.go:834` is a `listColumn`
  site this card exempts in section 9 under `act-not-entity`, a ground a new finding kind does
  not disturb.
- Its one change to `internal/bench/entity.go` is inside `resolveWorkstreamRef`, at line 383,
  replacing the literal `"workstream"` with `KindWorkstream` in the returned `EntityRef`. That
  is the same function D-4 modifies, and D-4's edit is to the `strings.TrimSpace` line at the
  head of `WorkstreamByRef` rather than to this literal, so the two are separate lines of one
  function. This is the only place a textual conflict is plausible, and if it comes it is one
  line.

The one place the two cards depend on each other is D-10: this card leaves an attachment
hanging from a workstream with an unresolvable reference, and dinah-459 is what stops another
one being written. Whoever implements this card should fetch and rebase on trunk first and
report which of the two commits is beneath them, rather than assuming 22a35fc.

# 5. The changes

## 5.1 A comment's address, at both heads

`verb.CommentView` gains one field, placed after `ID` so the payload reads identity first:

```go
// Ref is what a person types to reach the comment: the card's own
// reference, then comments and the comment's one-based position among the
// card's comments. It is the spelling internal/bench/resolve.go resolves,
// and it is a spelling to type now rather than a handle to keep, because a
// position stops naming the same comment once an earlier one is deleted.
// The identifier beside it is the handle, and the resolver accepts that in
// the same slot.
Ref string `json:"ref"`
```

`Library.Show` already computes that reference at `internal/verb/read.go:821`, where it
composes the comment's own attachments against it. The composition moves one line up into a
local and fills both:

```go
ref := commentRef(cardRef, memberPosition(comment.Dir, bench.CommentAnchor))
view := CommentView{ID: comment.ID, Ref: ref, TS: comment.TS, Author: comment.Author, Body: comment.Body}
below, err := attachmentViews(comment.Dir, ref)
```

`memberPosition` and not `comment.Ordinal`. The stored ordinal is what a comment was assigned
when it was written, and the resolver counts position in the collection as it stands now, so
the two part company the moment an earlier comment is deleted. AC-3's fixture holds a deleted
comment for exactly that reason.

The terminal draws it as the row's first column, at `cmd/dinah/render.go:580`:

```go
comments := table{indent: 2, columns: s.columns("comments", "ref", "when", "who")}
for _, comment := range detail.Comments {
    fields := []string{comment.Ref, comment.TS, comment.Author}
    comments.rows = append(comments.rows, tableRow{fields: fields, note: comment.Body})
}
```

The comment's body stays where it is, as the row's note beneath the row.

## 5.2 A checklist item's address becomes total

`verb.ItemView.Ref` is filled for every item rather than for the three declared kinds alone,
and its tag loses `omitempty` because it is never empty. One composer replaces the inline
composition at `internal/verb/read.go:850`, and `contents` uses the same one:

```go
// itemRef is what a person types to reach one checklist item. An item of
// one of the three kinds the format declares is named by that kind's alias
// and its position among the items of that kind, which is the spelling
// dinah show already prints and the one walkBelowCard narrows by. An item
// of any other kind is named by the collection and its position in it,
// which descend resolves without narrowing, so a damaged item or an
// extension kind still shows a reader something they can type.
func itemRef(cardRef, kind string, kindPosition, position int) string {
	if alias, ok := bench.AliasForItemKind(kind); ok {
		return cardRef + "/" + alias + "/" + strconv.Itoa(kindPosition)
	}
	return cardRef + "/" + bench.ChecklistDir + "/" + strconv.Itoa(position)
}
```

`position` is the item's one-based place in `bench.Items`, which is what `ItemView.Ordinal`
already carries and what the resolver counts, since both walk
`SortByOrdinal(collection, ItemAnchor, ListIDs(collection))`.

The doc comment on `ItemView.Ref` saying the field is empty for an item outside the three
kinds is replaced, because the claim it rests on is false at 22a35fc.

## 5.3 `contents` prints the aliased spelling

`Library.containedNode` composes every non-card, non-column child as
`parentRef + "/" + mount.Dir + "/" + position`, at `internal/verb/tree.go:1037`. For the item
mount it calls `itemRef` instead, which needs the item's own kind and its position within that
kind. Both are read where the walk already reads the collection, in `containedChildren`:

```go
// kindSeen counts each item kind's members as the walk passes them, so an
// item's reference carries its position within its own kind rather than
// within the whole checklist. The walk lists a collection through
// containmentMembersOf, which sorts the way bench.Items sorts, so this
// count and the count Show takes over bench.Items agree by construction.
kindSeen := map[string]int{}
```

and a reader for the kind:

```go
// itemKindAt is the kind an item's own anchor records, and the empty string
// where the anchor will not read. An unreadable anchor composes the
// unaliased reference, which is what the walk printed for every item before
// this card and which still resolves.
func itemKindAt(dir string) string {
	item, err := bench.LoadItem(dir)
	if err != nil {
		return ""
	}
	return item.Kind
}
```

`show` and `contents` then print one spelling per item. dinah-456 section 4.3 rules that
`contents` moves to `show`'s spelling rather than the reverse.

## 5.4 An attachment's address replaces its position

`renderAttachments` at `cmd/dinah/render.go:640` draws the block for `dinah show` and for
`dinah attachments` alike:

```go
attachments := table{indent: 2, columns: s.columns("attachments", "ref", "filename", "description")}
for _, attachment := range views {
	attachments.rows = append(attachments.rows, tableRow{fields: []string{
		attachment.Ref,
		attachment.Filename,
		attachment.Description,
	}})
}
```

`AttachmentView.Ordinal` stays in the JSON. `show` and `attachments` are not members of
`verb.CrossHeadIdentical`, which declares `query`, `tree` and `changes` and nothing else, so no
parity guard requires the terminal to draw a field the payload carries.

## 5.5 A workstream is printed in the spelling the reference grammar takes

`Workstream.Ref()` at `internal/bench/workstream.go:96` becomes the single composer and returns
the prefixed form:

```go
// Ref is what a person types to reach this workstream in the reference
// grammar: the kind's own prefix, then the workstream's slug where it
// carries one and its identifier otherwise. The prefix is part of the
// reference rather than decoration, because a bare handle is tried against
// the columns and the cards first and resolves to neither.
func (w *Workstream) Ref() string {
	if w.Slug != "" {
		return WorkstreamRefPrefix + w.Slug
	}
	return WorkstreamRefPrefix + w.ID
}
```

It has five live call sites, produced by `grep -rn '\.Ref()' --include=*.go .` filtered to the
ones whose receiver is a workstream: `bench.resolveWorkstreamRef`
(`internal/bench/entity.go:386`, which fills `EntityRef.Ref` and so reaches `contents` and
`attachments`), `verb.workstreamView` (`internal/verb/beyond.go:930`), `Library.entityRef` for
a `changes` row (`internal/verb/changes.go:672`), the `search` hit
(`internal/verb/search.go:253`), and the card line's membership cell
(`cmd/dinah/render.go:1191`). Round 1 said six and counted `StructuralAct.WorkstreamRef` as
one. That field is filled by `workstreamRefSubject`, which reads `EntityRef.Ref` rather than
calling `Ref()`, so it is a consumer of the first of the five. The substance is unchanged: the
refusal raised when a workstream still holds cards comes to quote the prefixed spelling, which
is the spelling that reaches the command the reader runs next.

`verb.WorkstreamView.Ref`'s own doc comment states the rule this section retires. It reads
"what a person types to reach it: its slug where it carries one, its identifier otherwise",
and after this card the field carries the prefix in both branches. It is replaced, and its
`omitempty` goes, on the same argument section 5.2 makes for `ItemView.Ref`:

```go
// Ref is what a person types to reach the workstream: the kind's own
// prefix, then the slug where the workstream carries one and the
// identifier otherwise. It is never empty, because Workstream.Ref falls
// back to the identifier.
Ref string `json:"ref"`
```

The commit before this card's base was a sweep of six false documented statements, two of which
shipped in the binary, so a doc comment left asserting a retired rule is a defect this workbench
has just paid for.

The terminal's listing at `cmd/dinah/render.go:1104` prints the reference in place of the slug:

```go
t := table{indent: 2, columns: s.columns("workstreams", "reference", "name", "status", "cards")}
for _, workstream := range listing.Workstreams {
	fields := []string{workstream.Ref, workstream.Title, workstream.Status, strconv.Itoa(workstream.Cards)}
```

The cell no longer passes through `slugCell`, because `Ref` falls back to the identifier and is
never empty, where `Slug` can be.

**Where the card line's membership cell actually falls.** `s.workstreamsCell` is reached from
one place, `renderCard` at `cmd/dinah/render.go:96`, whose own doc comment says it is "the
single site every act prints its card line from, so the field appears after claim, move,
release, block, unblock, add and show as well as after join and leave". So the cost D-5 accepts
is carried by `dinah show` and by the line printed after an act, and by nothing else. Round 1's
D-5 also named `ls`, `next` and `status`, and that is wrong: all three draw tables with no
membership column at all. Verified by run at 22a35fc against a workbench whose only card
belongs to a workstream, where `dinah show pb-1` prints
`pb-1  a card with things below it  [Intake / ready]  addressing` while `dinah ls intake`,
`dinah status` and `dinah next` print no membership anywhere.

**Both spellings reach the commands that take a workstream.** Printing the prefixed form is
honest only if `dinah workstream get workstream/addressing` and
`dinah join <card> workstream/addressing` work, and today they refuse.
`Bench.WorkstreamByRef` at `internal/bench/workstream.go:230` is the one resolver every
workstream-taking call site goes through, so it takes the tolerance, at the head of the
function:

```go
// A caller may write the reference-grammar spelling or the bare handle.
// Every surface prints the prefixed form, and the workstream-taking
// commands took the bare form before this card, so both are accepted and
// exactly one prefix is stripped. A doubled prefix is not a spelling
// anything prints, and it is refused.
ref = strings.TrimPrefix(strings.TrimSpace(ref), WorkstreamRefPrefix)
```

The two callers that strip the prefix themselves before calling, `resolveWorkstreamRef` and
`Bench.ResolvePath` at `internal/bench/resolve.go:149`, pass the whole reference instead of the
remainder, so exactly one strip happens and it happens in one place. Each keeps refusing
`dinah.unknown-workstream` with the remainder as the detail, which is the text those refusals
carry today.

dinah-456's OQ-1 does not disturb this. Whether `workstream get` and `workstream set` are
retired in favour of the generic pair, both spellings arrive at `WorkstreamByRef`.

This tolerance does not reach the commands that take any reference at all. `archive`, `delete`,
`contents`, `attachments` and `path` go through `Bench.ResolvePath`, where a bare handle is
tried against the columns and the cards before anything else, so those keep requiring the
prefix. Section 7.4 is where the quick-start's prose is brought into line with that split.

## 5.6 The workbench's own row prints `workbench`

`internal/verb/search.go:264` fills the workbench hit's `Ref` with the slug, and a bare slug
resolves to nothing. It prints `workbench`, which is the spelling `Library.Attachments` and
`Library.rootOf` already use for the workbench's own row and which dinah-456 D-10 ruled on. The
literal is written in three places at 22a35fc, so it becomes a name in
`internal/bench/bench.go` beside `IsWorkbenchRef`, which reads it:

```go
// WorkbenchRef is the spelling a surface prints for the workbench itself,
// and one of the two forms IsWorkbenchRef accepts. The addresses below the
// workbench are composed against the slug instead, which dinah-151 OQ-9
// settled and this does not reopen.
const WorkbenchRef = "workbench"
```

`internal/verb/read.go:939` and `internal/verb/tree.go:915` use the name in place of their
literals.

`bench.KindWorkbench` at `internal/bench/containment.go:9` already names the same literal and
is not the constant to use here. It is the entity kind the containment table keys on, and it
answers what a thing is. `WorkbenchRef` answers how the workbench is spelled in the reference
grammar, which is a different question with the same answer today. Collapsing the two would tie
the containment table and the address grammar together for no reason but that the strings
match, and the pair that would then break is easy to name: a grammar that came to spell the
workbench `bench` would force a rename on a kind the format stores on disk.

# 6. The message catalogues

Two keys are added, one is renamed, and one is retired. Every catalogue under
`internal/msg/locales/` carries all four edits, which is `en`, `de`, `hi`, `cs`, `id`, `es`,
`fil` and `af`.

**Added: `column.comments.ref` and `column.attachments.ref`.** Both copy
`column.checklist.ref`, which is the heading the two blocks are being brought into line with,
so all three read the same word. English text `Ref`, German `Referenz`, Hindi `संदर्भ`, and the
five skeleton catalogues carry `Ref` with `"skeleton": true` and no source. The German and
Hindi entries carry `"source": "9ff1e019feac1ee2"`, which is `msg.Fingerprint("Ref")` and is
the value `column.checklist.ref` already stores in both catalogues, because the English text is
byte-identical.

Each entry's `context` says what the heading sits over and where a reader meets it. The
existing `column.checklist.ref` context is the model, and neither new context repeats its
sentence about a block that draws no heading row, because both new blocks do draw one.

**Renamed: `column.workstreams.slug` becomes `column.workstreams.reference`.** The cell holds a
reference rather than a slug. English text `Reference`, German `Referenz`, Hindi `संदर्भ`, and
the skeletons carry `Reference` with `"skeleton": true`. The German and Hindi entries carry
`"source": "fe88d270aa3e71da"`, which is `msg.Fingerprint("Reference")` and is what
`column.tree.reference` stores in both catalogues. The German entry loses the
`"verbatim": true` the retired key carried, because `Referenz` is not the English word. The
Hindi entry loses nothing, because `column.workstreams.slug` reads `उपनाम` there and carries no
verbatim flag.

**Retired: `column.attachments.position`.** Nothing draws it once the `#` column goes. It is
deleted from all eight catalogues in the same diff, so `TestEveryDeclaredLanguageShips` sees
one key count per catalogue.

`internal/msg/glossary.json` declares four terms, `state`, `the root`, `owner` and `level`, and
none of the four texts above triggers any of them, so `TestATranslationUsesTheDeclaredWord` has
nothing to say here.

**The decision record the staleness contract asks for is on the card already.** That document's
section "The semantic layer, which is not a test" requires one `decision`-kind checklist item
per changed key, and Agent Code Review treats a locale-touching diff with no such record as a
major finding. Round 1 recorded all three keys in one item, which is the collective form the
document reserves for a diff filling hundreds of skeleton entries. Three keys is the ordinary
case, so D-16, D-17 and D-18 now carry one key each in the shape the document prescribes, and
D-7 keeps the design argument for the key names. D-8 keeps the two deletions, which the per-key
rule does not reach, because a deleted key carries no translation to have fallen behind.

# 7. Tests

## 7.1 The new guard, which is what makes the rule checkable

`cmd/dinah/address_sweep_test.go` holds two tests. The first proves the addresses work, and the
second proves the first one covers everything.

**`TestEveryPrintedReferenceResolves`.** For each declared case it runs a command against a
fixture workbench holding one of every kind, reads the reference cell out of the rendered
table, and hands that exact text back to `dinah path`. The case passes when `path` exits 0 and
names a file that exists. A case declares the command, the site it drives, the index of the
cell holding the reference, and whether the block is guided, which section 7.2 explains.

The round trip is what gives the test its reach. It compares nothing against a hand-written
expectation, so a wrong reference fails whether it is misspelled, composed against the wrong
parent, or numbered from zero, and a reference that is merely absent fails as an empty argument
`path` refuses. Run at 22a35fc it would fail on the workbench search hit, on the undeclared
item kind and on the workstream listing, which are the three defects section 3.1 records.

The round trip alone is weaker than it looks on a block whose rows all address entities under
one parent, because any reference that resolves satisfies it. A renderer printing the card's own
reference on every attachment row would pass. So every case carries an identity assertion beside
the round trip, naming what that cell should hold for that row, and the criteria state both arms.

**`TestEveryEntityDrawingSurfaceIsSwept`.** It reads `renderSitesInSource` rather than parsing
again, and requires every site that walk returns to be either the site of a case of the test
above or a member of a declared exemption list. An exemption entry carries the site, a ground,
and a reason. The grounds are a closed set of four constants:

```go
const (
	// groundNoEntity is a table that draws no entity of a workbench:
	// commands, flags, settings, catalogues and the guide topics.
	groundNoEntity = "no-entity"
	// groundActNotEntity is a table whose rows are recorded acts rather
	// than entities, so a row's subject is an event and the entity it
	// names is the one the caller asked about.
	groundActNotEntity = "act-not-entity"
	// groundNamedByCaller is a table of one entity's own stored fields,
	// reached by naming that entity, so the reader already holds its
	// address.
	groundNamedByCaller = "named-by-caller"
	// groundOutsideWorkbench is a table whose rows are workbenches rather
	// than entities within one, addressed by filesystem path.
	groundOutsideWorkbench = "outside-workbench"
)
```

The test fails a site that is neither swept nor exempt, an exemption naming a site the walk does
not find, and an exemption whose ground is outside the four. A new table therefore cannot reach
a green build unargued.

It carries one further assertion, which is what keeps the roster honest as the renderer grows.
`renderSitesInSource` reads a `columns` field only where it calls `(*session).columns` or
`listColumn`, and a site whose columns come from anywhere else is reported as unresolvable
rather than as swept. So the test asserts that the package declares exactly those two functions
returning `[]tableColumn`, by walking the same parsed files for a function declaration whose
result type is that slice. A third constructor fails the test and names itself, which is the
failure that tells the next author the guard needs teaching rather than letting a site slip
past it.

**What a site being swept does and does not prove.** A site is swept when at least one case
drives it, and one site can draw more than one collection. The refusal listing at
`cmd/dinah/render.go:962` serves three: the columns, the guide topics and the settings keys.
Only the columns listing draws entities, so only that one is a case, and the entry says so. The
guard's claim is that no drawing site reaches a green build unargued. It is not the claim that
every row every site can ever draw has been round-tripped.

`internal/verb/address_view_test.go` holds the machine half,
**`TestEveryViewCarryingAnIdentifierCarriesAReference`**. It parses the package's non-test
sources, which is `internal/verb/*.go` with every path ending `_test.go` skipped, the same
exclusion section 2.2's derivation script makes and for the same reason: the rule is about the
views the product ships, and a test fixture declaring a struct with an `ID` field would redden a
guard about production views for a reason unrelated to the rule. It finds every struct
declaring a field named `ID` with the json tag `id`, and requires each to declare
a `Ref` field or to be named in an exemption list carrying a ground from a closed set of two:
`groundIdentifierResolves` for a view whose identifier is itself accepted in the head position
of a reference, and `groundNotAnEntityView` for a report about a run rather than a view of an
entity. `verb.ColumnView` takes the first, and `verb.ReshapeColumn`, `verb.ReshapeRetirement`
and `verb.RemintReport` take the second.

That guard on its own can be satisfied the wrong way, and AC-9 says so. An implementer who added
`verb.CommentView` to its exemption list with a plausible ground would pass it while leaving the
card's own defect in place. AC-3 is what forbids that, because it asserts against a real
`CommentView` value that its `Ref` resolves to the comment. The two criteria are sound together
and neither is sound alone.

## 7.2 The containment table, which both `tree` and `contents` draw

`cmd/dinah/render.go:331` is one site and two commands. Under `dinah contents` it draws
comments, items, attachments, cards and columns, and under `dinah tree` it draws columns, cards
and grouping rows. Section 3 gives it two rows, so the completeness guard requires it to be a
case rather than an exemption, and none of the four grounds would be honest for it. Two facts
about the drawing decide how the case reads a cell, and round 1's mechanism could read neither.

**The reference cell carries the tree's own drawing prefix.** `withGuides` at
`cmd/dinah/table.go:173` writes each row's prefix into the row's first field before the table is
laid, so the text in that cell is the prefix followed by the reference. Run at 22a35fc:

```
a card with things below it (pb-1) contains 2 entities.
  Reference               Entity      Title      Count
  ----------------------  ----------  ---------  -----
  |-- pb-1/comments/1     comment     a thought  0
  `-- pb-1/attachments/1  attachment  f.txt      0
```

The case strips the prefix with `guidedLead`, declared at `cmd/dinah/output_check_test.go:286`
and already used by the row sweep for the same reason. It composes the four possible pieces from
`table.go`'s own glyph constants rather than from literals, and it returns the prefix it
matched, so the reference is what follows. A case declares whether its block is guided, and a
guided case takes that step before the round trip. Nothing new is written to make this work. The
reader exists and this card uses it.

**A grouping row addresses nothing, and its cell carries the group's value.** `treeFields`
returns the node's `Value` in the Reference column for a node whose `Kind` is `verb.NodeGroup`,
and the group's `Axis` in the Entity column beside it, with `tree.unset`'s text standing in
where the value is empty. This is not a new ruling. `column.tree.reference`'s own English
context already states it: "On a group row the cell carries the value the group gathered at,
which nothing addresses, and the Entity cell beside it says so." A grouping row is therefore
not a row the address rule reaches, and the case skips it rather than round-tripping it.

**How the case tells the two apart, and why not by reading the Entity cell.** The axis
vocabulary and the entity-kind vocabulary overlap. `verb.FieldColumn` is `"column"` and
`bench.KindColumn` is `"column"`, and `verb.DefaultChain` groups on `column` first, so a bare
`dinah tree` prints `column` in the Entity cell of a grouping row while `contents` prints it in
the Entity cell of an entity row. Run at 22a35fc, `dinah tree` opens
`|-- intake      column         1`, which is a group, and `dinah contents` on a workbench draws
a column row whose Entity cell reads the same word. A test reading that cell would have to
guess, and a guess is not a contract.

The case pairs the drawn rows against the payload instead. It runs the same command twice
against one workbench, once at the terminal and once with `--json`, walks the payload's
`Root.Children` in pre-order, and pairs node i with row i. That pairing is what the renderer
does: `treeRows` appends one row per node and then recurses into that node's children, so the
two orders are one walk. The case then requires all of the following.

- The row count equals the node count. This arm fails first and loudest when the renderer and
  the walk disagree at all, which is the failure that would make every other arm meaningless.
- Every row of the block is guided. `guidedLead` reports false on a line opening with no guide
  piece, and the case fails that row by name rather than skipping it or reading the whole cell
  as a reference, so a renderer that stops drawing guides fails loudly. Every containment row is
  a child of the root and carries a guide today, so this arm cannot bite as the table stands.
- For a node whose `Kind` is not `verb.NodeGroup`, the row's stripped reference cell equals the
  node's `Ref`, and that same text exits 0 through `dinah path`.
- For a node whose `Kind` is `verb.NodeGroup`, the row's stripped reference cell equals the
  node's `Value`, or the text of `tree.unset` where the value is empty, and no round trip is
  attempted.

Both commands are reads, so nothing moves between the two runs of one command against one
fixture, and the library call each run makes is the same call with the same arguments.

The property that makes the pairing sound is one command's own path through the cli head, and
it covers `contents` and `tree` alike. `runTree` at `cmd/dinah/commands.go:721` and
`runContents` at `cmd/dinah/commands.go:733` each obtain one `*verb.Tree` from one library call
and then hand that same value either to `emitMachine`, which marshals it, or to `renderTree`,
which draws it. `renderTree` at `cmd/dinah/render.go:326` lays a table and calls `treeRows` at
`:359`, which appends exactly one row per node and then recurses into that node's children, and
the root is not a row. So the drawn rows and the payload's pre-order walk of `Root.Children` are
one walk of one structure, and no claim about the two heads matching is needed or made.

`verb.CrossHeadIdentical` is not that property and round 2 cited it wrongly. It is declared at
`internal/verb/definition.go:596` over `query`, `tree` and `changes`, and it says that the MCP
head answers with the payload the cli head prints under `--json` for the same arguments. The
pairing here is between one cli command's terminal output and that same cli command's `--json`
output, which is a different claim, and `contents` is not a member of that set in any case.

What the pairing does not prove is that the renderer put the right node on each row. A renderer
that permuted the nodes and permuted the references the same way would pass. The containment
walk's ordering is held instead by `TestTheContainmentWalkAndShowAgreeOnEveryItemReference`,
which pairs by identifier, and `dinah tree`'s ordering is held by the existing tree tests. This
case's job is the address on the row and it claims nothing beyond that.

## 7.3 The existing tests this card moves

- `cmd/dinah/row_sweep_test.go`, `TestEveryRowStartsItsColumnsAtOneDisplayColumn`. Three
  registry entries change: the comments site gains `column.comments.ref` at the head of its
  `keys`, the attachments site replaces `column.attachments.position` with
  `column.attachments.ref`, and the workstreams site replaces `column.workstreams.slug` with
  `column.workstreams.reference`.
- `cmd/dinah/output_check_test.go`, `assertEveryRegisteredSiteMatchesItsColumns`. It holds every
  registry entry to the columns its own site declares, so the three key changes above have to
  land in the registry and in the renderer in one diff or the pairing fails.
- `cmd/dinah/row_pairing_test.go`, `expectComments`. Each expected row gains the reference as
  its first cell, and the opaque stamp column moves from index 0 to index 1, so
  `sweptStampColumn(1, ...)`.
- `cmd/dinah/row_pairing_test.go`, `expectAttachments`. The first cell changes from the position
  to the composed reference.
- `cmd/dinah/row_pairing_test.go`, the workstreams expectation. Its first cell changes from the
  slug to the prefixed reference.
- The `internal/msg` guards. `TestEveryDeclaredLanguageShips`,
  `TestATranslationTracksItsEnglishSource`, `TestATranslationIsNotEnglishUnderAnotherTag` and
  `TestASkeletonEntryReallyCarriesTheEnglishText` all read the catalogues, and all four have to
  be green on the added, renamed and retired keys.
- `cmd/dinah/quickstart_test.go`, which section 7.4 covers on its own.

## 7.4 `docs/quick-start.md`

The document is a replayed transcript, and round 1 named neither it nor its regeneration flag.
`TestTheQuickStartMatchesTheTool` replays every fenced block whose first line opens `$ ` through
`runCLI` and compares the bytes against the document. This card moves three of the things those
blocks show, so the guard goes red until the document is regenerated, and one sentence of prose
that no guard can see becomes false.

**What regenerates, and how.**
`go test ./cmd/dinah -run TestTheQuickStartMatchesTheTool -update-quick-start` rewrites the
transcript blocks from the replay and then fails the run on purpose, which is what stops the
flag being left switched on in CI. The implementer runs it, reads the diff, and commits the
document in the same commit as the code.

Three changes reach the transcript: `workstream.line` prints the workstream an act names,
`card.line.workstreams` prints a card's memberships, and the `workstreams` listing changes both
its heading and its first cell. `grep -n 'autumn' docs/quick-start.md` at 22a35fc prints 850,
851, 856, 868, 869, 871, 872, 884, 891, 894, 911, 920, 921, 931, 932, 937 and 938, of which the
output lines that move are 851, 869, 872, 884, 921 and 932. Two further lines move that no grep
for `autumn` can find, because they carry no workstream name: line 882, the listing's heading
row reading `  Slug    Name            Status  Cards`, and line 883, the rule row beneath it,
whose widths are recomputed when the heading and the first cell grow. Those two were read out
of the block rather than grepped. Eight lines in five replayed blocks, then. The regeneration is
what decides the final list, and the implementer reports what it wrote rather than checking it
against this paragraph.

The `dinah workstream get autumn` block at line 891 carries a `skip=` reason and is not
replayed, and it would not move anyway: its field table draws the workstream's stored `slug`,
which this card does not touch.

**The sentence that ships false.** It spans lines 940 to 942, and the document's own wrapping
is load-bearing for the check AC-12 makes, so it is quoted here byte for byte as the document
holds it:

```
940: it. A workstream names its kind in both of those commands, and nothing else
941: does, so a workstream and a column may share a name without either one hiding
942: the other.
```

The clause breaks between `nothing else` and `does`. A containment test over the raw bytes for
the sentence as a person would write it therefore matches nothing today, which is the trap this
section set for round 2's criterion, and AC-12 now flattens the document's whitespace before it
tests. Line 940 also opens with `it. `, the tail of the sentence before it, so the false
sentence does not begin a line either.

After this card the middle clause is wrong in both directions. Every screen that prints a
workstream prints the kind, and the two commands the clause exempts are no longer the only ones
that accept it, because `workstream get`, `workstream set`, `join` and `leave` come to take the
prefixed spelling as well as the bare one. The reason the sentence gives is still true and is
the part worth keeping. The three lines are replaced with the following prose, which the
implementer wraps to the document's own width rather than to the width shown here:

> A workstream names its kind in those two commands because they take any reference at all, and
> a bare name is tried against the columns and the cards first. `dinah join`, `dinah leave` and
> `dinah workstream get` take a workstream and nothing else, so they accept either spelling, and
> Dinah prints the longer one everywhere. That is why a workstream and a column may share a name
> without either one hiding the other.

The replacement contains no run of words that flattens to the retired clause, so the wrapping
the implementer chooses cannot accidentally satisfy or defeat AC-12's assertion.

**Showing the half that can be shown.** The header comment of `quickstart_test.go` names its own
blind spot, "A claim the document makes in prose instead of showing", and this claim is one of
those. It can be shown, so a console block goes in after the replaced paragraph carrying two
reads that change nothing:

```
$ dinah workstream get workstream/autumn-2025 status
$ dinah contents autumn-2025
```

The first succeeds and the second refuses, which is the whole of the paragraph's claim. The
implementer writes the two command lines and lets `-update-quick-start` write the output lines
and the exit markers, so the bytes come from the tool rather than from this spec. Both are
reads, so no later block in the narrative moves.

**What still has no guard, said plainly.** AC-12 asserts that the false sentence is gone and
that the new block replays, and it cannot assert that the replacement is true, because nothing
mechanical reads prose. The gone-ness arm is a containment check over the
document's whitespace-flattened text, which proves only that the specific false clause was
removed and would pass against a replacement saying something else wrong. The flattening is
what makes it able to fail at all: the clause crosses a line break in the document as it
stands, so the same test over the raw bytes passes today and would pass again against a
restored sentence, which is exactly what round 2 shipped. The human check is the diff read at
Operator Design Review and again at Operator Code Review, and this section quotes the three
lines as the document holds them and gives the replacement in full, so that read has something
to compare against rather than a judgement to make from nothing.

## 7.5 Arming

Each criterion names a break that leaves the tree compiling, and the compiling half is the point
rather than a formality. A plant that fails the build produces no test output, and no output at
a glance reads exactly like a pass. This workbench has caught that shape four times, and round 1
of this card was the fourth: AC-9's recipe was to delete `Ref` from `verb.CommentView`, a field
this card's own `Library.Show` writes and whose value `cmd/dinah/render.go` reads into a cell,
so the plant broke two packages and ran nothing.

Every criterion was re-read against that finding and three moved.

- AC-9's plant is now an addition rather than a deletion. Adding a struct to `internal/verb`
  that declares `ID` with the json tag `id`, declares no `Ref`, and appears in no exemption
  compiles, since an unused type is legal Go, and it turns the guard red naming that type.
- AC-2's old plant reinstated the `#` column, whose catalogue key this card deletes from all
  eight catalogues, so the plant referred to a key no catalogue carries. It now swaps the
  block's first heading key for `column.checklist.state`, an existing key with different text,
  which compiles, leaves every catalogue guard green, and turns the heading assertion red.
- AC-1's plant compiles, and its note was wrong about which arm goes red. Composing a comment's
  reference against `bench.ChecklistDir` yields `<card>/checklist/1`, which resolves on a
  fixture holding checklist items, so `dinah path` exits 0 and the round trip passes. The arm
  that fails is the identity assertion, and the note says so now. This is the compiling-plant
  lesson from the other side: a plant that leaves the named assertion green is as useless as one
  that never runs.

Round 3 adds a fourth to that list, and it is the same lesson one step along. AC-12's document
arm was armed by restoring a sentence the assertion could not see, because the assertion tested
the raw bytes for a clause the document wraps mid-way through. The plant was a real edit, the
test stayed green, and the green read as a pass. A plant that compiles and runs is still useless
when the assertion it is aimed at cannot fail, so the arming step reports the assertion going
red rather than the plant landing.

Plant the break, watch the named test go red with the named message, restore from a
byte-identical copy, and watch it go green. Report the build succeeding, then report what the
red run said.

# 8. The acceptance criteria

The card's checklist carries them, and they are the half that gets verified. Where this prose
and a criterion disagree, the criterion governs.

# 9. Examined and left alone

Every one of these draws something on a screen or in a payload, and each is left as it stands
for the reason given. The grounds are the four of section 7.1 and the two of the machine-side
guard.

| Surface | Ground | Why |
|---|---|---|
| `help`, the commands list, the flags table, the arguments table, `guide`, `config`, `version --catalogs` | no-entity | The rows are commands, flags, settings and languages. None of them is an entity of a workbench and none has a reference. |
| `log` | act-not-entity | The rows are one card's recorded acts. The card is the entity, and the caller named it to reach the log. |
| `check` findings, and the two identifier listings a `check` repair prints | act-not-entity | A finding names the file a defect was found in, and several finding kinds name a file that by construction has no reference, such as a missing anchor or an attachments directory under a kind the containment table gives no mount. The two repair listings name what one run removed or witnessed. Giving those rows a reference would mean inventing one for a thing the grammar does not address, or addressing a column the repair has just taken away. |
| the guide-topics and settings listings a refusal draws | no-entity | Two of the three collections `refusalListings` serves are guide topics and configuration keys, neither of which is an entity of a workbench. The third is the columns, and that one is swept. |
| `workbench`, and `workstream get`'s field table | named-by-caller | The table lists one entity's own stored fields. The reader typed that entity's address to get there. |
| `workbenches` | outside-workbench | The rows are workbenches under a directory, addressed by the filesystem path the row already prints and passed back through `--workbench`. |
| `verb.ColumnView` | identifier-resolves | A column's identifier is accepted in the head position, verified by run: `dinah show <12-hex>` answers the column, as do its slug and its title. A client holding the view can name the column. `CommentView` is the contrasting case, because a comment's identifier resolves only in the selector slot of a path that already names the card, so a client holding the identifier alone cannot compose an address. |
| `verb.ReshapeColumn`, `verb.ReshapeRetirement`, `verb.RemintReport` | not-an-entity-view | Each reports what a run did or would do. A reshape preview names a column that does not exist yet, and the identifier it carries resolves once the column does. |
| A comment's attachments at the terminal | none needed | The terminal draws no block for them. `dinah show --json` carries them with a reference each, and `dinah attachments <card>/comments/<n>` draws them under the same renderer as every other attachment listing. Nothing is drawn without an address, so the rule has nothing to say here. |
| The VS Code extension's tree | none needed | It draws root, column, group, card, attachments-group and attachment nodes, and no comment or checklist node, so the field this card adds has no row there yet. Its attachment row acts through menu commands rather than through an address a reader types, which dinah-451 established. When it draws a comment it inherits `CommentView.Ref` with no further work. |
| `dinah export` | none needed | It writes the workbench definition, which is the columns and the workbench's own fields. It draws no card, comment, item or attachment. |

Two things this sweep found and does not fix, each with its owner named.

**An attachment under a workstream has no address at all.** `dinah attach workstream/addressing
f.txt` succeeds today, `dinah attachments workstream/addressing` lists the result with the
reference `addressing/attachments/1`, and neither that spelling nor
`workstream/addressing/attachments/1` resolves, because `ResolvePath`'s workstream arm reads
everything after the prefix as the workstream's name. After section 5.5 the listing composes
`workstream/addressing/attachments/1`, which is no better. dinah-459 owns this: a workstream
mounts no attachments collection, dinah-456 section 5.6 rules that it stays outside the
containment table, and dinah-459 both refuses the write and reports the files already written.
This card does not invent an address for an entity the grammar does not address.

**`dinah contents workstream/<slug>` prints an empty title.** The root sentence reads
` (workstream/addressing) contains nothing.` because `anchorOfKind` searches the mounts of the
workbench, a card and a comment, and a workstream is in none of them. That is a title rather
than an address, so it falls outside this card's rule, and it belongs to whoever next touches
the containment walk.

## Branch

dinah-454-a-cards-comments-print-with-no-address-so-nothing-on-screen-tells-a-reader-how-to-name-one
