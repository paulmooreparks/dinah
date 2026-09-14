---
title: restore is the inverse of archive, and an archived entity has an address again
column: b69abf918c42
state: ready
severity: minor
priority: soon
tier: frontier
workstreams:
  - 994787601ae6
---
`dinah archive` exists for every kind and `dinah restore` is not a command at all: it refuses `dinah.unknown-command`. The reversible half of the pair is reversible only by moving a directory by hand.

For a card the archived half is at least still addressable, because `Bench.ResolveArchivedCard` reads the mirror. For every other kind it is not. After `dinah archive <card>/comments/1`, `dinah show <card>/comments/1` refuses `dinah.unknown-path`, and the comment has no address at any surface, so nothing could name it to a restore even if one existed.

`docs/design/format.md` already declares the `restored` event, the note it carries, which is the entity's own identifier, and the structural act, and says in its own words that the event is written by no command. This implements a published contract rather than minting one.

`restore <ref>` moves the directory back to the position it was archived from, appends `restored` to the journal `archive` appended `archived` to, and refuses `dinah.exists` when a live entity occupies the slot. `--archived` becomes a marker flag on `restore`, `show`, `path` and `contents`, resolving the same reference against the mirror, with positions counting the mirror's own members.

dinah-456 section 5.5 is the contract.

## Specification

Trunk this spec is written against is `9260a2a` ("dinah-436: give a card link its write side"), which is the commit the first two drafts named and the one both rounds of Agent Design Review read. It was fetched again on 2026-09-10 for round 3 and `origin/main` had moved on by one commit, `73f9590` ("dinah-450: a checklist item names a column and nothing enforces it"). That commit touches none of `internal/bench/resolve.go`, `internal/verb/read.go`, `internal/verb/tree.go`, `internal/contract/shape.go` or `cmd/dinah/commands.go`, verified with `git log --oneline 9260a2a..73f9590 -- <those files>`, which returns nothing, so every line citation below still holds and round 3's own reading was done against `73f9590`. Four siblings of this card landed ahead of it and all four are in it: dinah-454 at `8661604`, dinah-459 at `813e0bb`, dinah-455 at `70ff10f`, and dinah-457 at `863b7c5`.

Round 2 changed section 4.1 in full, section 7 in full, one paragraph each in sections 3, 3.1, 4.4 and 10, and the wording of thirty-four bolded captions. Sections 1, 2, 5, 6, 8, 9, 11 and 12 carry no change of substance. Five criteria and one decision moved with them, and section 13 says which.

Round 3 changed the raise-site account in sections 3.1, 3.2 and 4.1, three line citations in section 3.1, the workbench value's name in sections 4.1 and 7, and two rows of section 10. Section 14 says what moved and which call chains were read to produce it.

dinah-456 section 5.5 is the contract this card implements, and sections 3.3, 4 and 7 govern the surfaces it touches. Nothing below re-derives those. Where this spec settles something section 5.5 left open, it says so and says why.

# 1. What is true at 9260a2a

Every claim in this section was produced by running a binary built from `9260a2a` against a workbench created for the purpose, with `DINAH_HOME` pointed at a scratch directory outside the operator's profile.

`dinah restore` is not a command:

```
$ dinah restore pb-1
dinah.unknown-command Dinah offers no command called restore; run `dinah help` for the list of commands, grouped by what they do
```

Archiving is local to the entity's holder. `bench.ArchiveTarget` moves `<holder>/<collection>/<id>` to `<holder>/archive/<collection>/<id>`, so the mirror appears at whatever depth the archived entity sat at. After archiving one comment of a card and then archiving the card itself, the tree carries both mirrors:

```
.dinah/<workbench>/archive/cards/5e6d8d71af28/
.dinah/<workbench>/archive/cards/5e6d8d71af28/archive/comments/31b3b19f8214
.dinah/<workbench>/archive/cards/5e6d8d71af28/comments/a7b0fd3880d1
```

An archived entity has no address. Archiving the first of two comments leaves `pb-1/comments/1` naming the second one, because a position counts the live half's members:

```
$ dinah archive pb-1/comments/1
$ dinah show pb-1/comments/1
---
ts: 2026-09-10T01:12:45Z
author: spec
ordinal: 2
---
second comment
```

The card's own description says a card is at least still addressable through `Bench.ResolveArchivedCard`. That is narrower than it reads. `ResolveArchivedCard` has two callers at `9260a2a`, `Bench.ResolveLinkTarget` and the archived-card lookup in `internal/verb/changes.go`, so a link may name an archived card and a change feed may report one. Neither `show` nor `path` reaches it:

```
$ dinah archive pb-1
$ dinah show pb-1
unknown-card this workbench carries no card pb-1; run `dinah ls` to see the cards this workbench carries
$ dinah path pb-1
unknown-card this workbench carries no card pb-1; run `dinah ls` to see the cards this workbench carries
```

Archiving a column drops its identifier from the workbench anchor's `columns` sequence through `Bench.RemoveColumnID`, and nothing puts one back. `internal/bench/columnretire.go` declares `RemoveColumnID` and `RemoveStrandedColumns` and no adding counterpart.

The structural machinery a restore needs is already built and already reachable, and only the verb is missing. `bench.OpRestore` is declared in `internal/bench/lock.go`, `bench.RestoreTarget` in `internal/bench/entity.go`, and `StructuralAct.Target`, `StructuralAct.siblingDir`, `StructuralAct.apply` and `Bench.Run` each carry an `OpRestore` branch. `Bench.Run` already refuses `contract.Exists` when the destination of a move exists, and already skips the last-column check and `RemoveColumnID` on a restore. `internal/bench/finish.go` can already complete or roll back an interrupted restore, and its `eventRecords` already recognises a `restored` line carrying the entity's identifier in `note`. Nothing calls any of it, because no verb starts a restore.

`docs/design/format.md` specifies the event, its one field and the structural act, in the "Journal event schema" and concurrency sections. `contract.EventRestored` is declared and `cmd/dinah/compat_test.go`'s `unwrittenEvents` exempts it with the reason "archive has no inverse verb in the command surface, so nothing restores an entity". So this card implements a published contract and mints no event.

`--archived` already exists as a marker flag on one command. `internal/verb/definition.go` declares it under `search`, filling `Request.Archived`, and `Library.Search` reads it as a widening: the live cards are scanned either way and the archived cards are scanned as well when the flag is set. Each hit carries its own `Archived` boolean, so a reader can tell the two apart on the row.

# 2. `restore`

```
restore <ref> [--archived]
```

`restore` moves an entity's directory out of the archive mirror and back into the live half of the collection it was archived from, appends `restored` to the same journal `archive` appended `archived` to, and answers silently with exit 0. `archive` prints nothing on success at a terminal, and `restore` matches it.

`Library.Restore` is the inverse of `Library.Archive` line for line, and it goes in `internal/verb/beyond.go` directly beneath it:

```go
// Restore moves an entity's whole directory out of the archive mirror and
// back into the live half of the collection it was archived from, history
// and all. It is Archive read in the other direction, and it runs the same
// structural protocol under bench.OpRestore, which Bench.Run has carried
// since the format's concurrency section was written.
func (l *Library) Restore(req *Request) *Response
```

The body differs from `Archive` in four places and nowhere else.

1. The reference is resolved through `l.Bench.ResolveEntityIn(bench.ArchivedHalf, req.Ref)` rather than `l.Bench.ResolveEntity(req.Ref)`, so the entity comes out of the mirror.
2. A reference naming a whole collection is refused by `ResolveEntityIn` itself through `CollectionRef.Refuse`, which raises `dinah.is-a-collection`. `Restore` repeats no check of its own, exactly as `Archive` repeats none.
3. The event is `contract.EventRestored` rather than `contract.EventArchived`. It carries `note` holding the entity's own identifier, which is what `finish.go`'s `eventRecords` requires and what `format.md`'s event table states.
4. `bench.StructuralAct.Op` is `bench.OpRestore`.

The workbench guard `Archive` carries, which refuses `contract.UnknownPath` for `entity.Kind == bench.KindWorkbench`, is not repeated. The resolver refuses the workbench under `ArchivedHalf` before `Restore` sees it, per section 4.1.

`journalFor` and `lockDirFor` are called on the resolved entity exactly as `Archive` calls them. Both read the entity's directory, and under a restore that directory is the mirror path, which is where the entity's own journal still is. That is the point of `format.md`'s "appended before the move": the journal travels with the directory, so the event is written at the source and arrives at the destination inside it.

# 3. `--archived`, and which half it names

`--archived` reads the archive mirror at the reference's deepest collection step, and the live half at every step above it. That is the whole rule, and every consequence below follows from it rather than being declared separately.

A collection step is a step naming one of the containment table's collections, which is what `bench.MountOf` answers for. `card`, `card.md`, `journal`, `journal.ndjson` and `payload` are not collection steps: the first four name a card's own two files and the last names the bytes an attachment wraps. The head is a collection step, because `cards`, `columns` and `workstreams` are collections of the workbench, which is why `dinah show --archived pb-1` reads `archive/cards` and `dinah show --archived pb-1/journal` reads the journal of the archived card.

Worked, with `pb-1` and `doing` live except where stated:

| Reference under `--archived` | Deepest collection step | What it resolves against |
|---|---|---|
| `pb-1`, `pb-1` archived | the head, `cards` | `archive/cards/<id>` |
| `pb-1/journal`, `pb-1` archived | the head, `cards` | `archive/cards/<id>/journal.ndjson` |
| `pb-1/comments/1` | `comments` | `cards/<id>/archive/comments`, position 1 |
| `pb-1/comments` | `comments` | the same directory, listed |
| `pb-1/comments/1/attachments/2` | `attachments` | `cards/<id>/comments/<cid>/archive/attachments` |
| `pb-1/attachments/1/payload` | `attachments` | the payload of the archived attachment |
| `doing/attachments/1` | `attachments` | `columns/<id>/archive/attachments` |
| `doing`, `doing` archived | the head, `columns` | `archive/columns/<id>` |
| `pb/attachments/1` | `attachments` | `archive/attachments` below the workbench root |
| `workstream/effort`, archived | the head, `workstreams` | `archive/workstreams/<id>` |
| `workbench`, `.` | none | refused `dinah.not-archived`, per section 4.1 |

Positions count the mirror's own members. `bench.MemberIDs` is called on the mirror directory, so `pb-1/comments/1` under the flag names the first comment that was archived, whatever the live half holds. This is dinah-456 section 5.5's own ruling, and it is what makes `dinah show --archived pb-1/comments` a listing a reader can restore out of.

A read command under `--archived` shows the archived half and nothing else. It is not a widener on any of `show`, `path`, `contents` or `restore`. The reason is the positions: a reference names one entity, so a flag that merged the halves would have to count positions over the merged set, and archiving something else would then silently change what an existing address means. A merged listing would also need a column saying which half each row came from, which is width on every row for a question the flag has already answered.

That is not what the flag does on `search`, and the difference is the shape of the command rather than an inconsistency. `search` scans a set and reports hits, and a hit carries its own `Archived` boolean, so admitting the mirror there costs nothing and hides nothing. The four commands this card touches resolve a reference to one thing. One sentence covers both cases: `--archived` admits the archive mirror, and a command that resolves a reference admits it by resolving in it, where a command that scans a set admits it by scanning it too. The references guide states that sentence, per section 9.

An entity that travelled inside an archived holder is reached by restoring the holder. Archiving a card moves the card's whole directory, comments and all, and those comments were never archived in their own right. Under the rule above, `dinah show --archived pb-1/comments/1` with `pb-1` archived resolves its head live and fails, and section 4.1's refusal probe then finds the comment through the mirror at the head and reports `dinah.not-archived` against the reference, naming `dinah restore pb-1` as the act that makes it resolve. That is a real limit and it is a chosen one: a flag naming one half at one step is a flag, and a flag naming a different half at each step is a grammar, which dinah-456 section 8 refuses on the same ground it refuses axes. The route is `dinah restore pb-1`, after which every address below it resolves the ordinary way. The inverse itself is unaffected, because the comment comes back with the card that carries it.

An entity archived in its own right and then carried into an archived holder is restored in two acts. The mirror nests, as the tree in section 1 shows. `dinah restore pb-1` brings back the card together with its own `archive/comments/<id>`, and `dinah restore pb-1/comments/1` then brings back the comment. Nothing extra is needed for that to work, because the archive is local to its holder.

`contents --archived <ref>` walks the archived entity's tree and says so. The root resolves in the mirror and the walk below it reads the archived entity's own live collections, since that is where its children sit. The references those rows carry are the addresses the children will have once the root is restored, and they do not resolve while it is archived. `verb.Tree` gains an `Archived bool`, the human renderer prints one line under the heading from a new key `contents.archived`, and the guide carries the same statement. Saying it once on the listing is what keeps dinah-456 section 4.1 honest here, because the alternative is a screen of addresses that quietly do not work.

## 3.1 How the resolver carries the half

One value travels, and the three existing entry points keep their signatures by delegating to it. This is the whole of the mechanism, and no call site outside the four commands changes.

```go
// ResolutionHalf names which half of a collection a resolution reads at a
// reference's deepest collection step.
type ResolutionHalf int

const (
	// LiveHalf reads the live half, which is every resolution the tool
	// performed before restore existed.
	LiveHalf ResolutionHalf = iota
	// ArchivedHalf reads the archive mirror at the reference's deepest
	// collection step, and the live half at every step above it.
	ArchivedHalf
)

func (b *Bench) ResolveReferenceIn(half ResolutionHalf, ref string) (*EntityRef, *CollectionRef, error)
func (b *Bench) ResolveEntityIn(half ResolutionHalf, ref string) (*EntityRef, error)
func (b *Bench) ResolvePathIn(half ResolutionHalf, ref string) (string, error)
```

`ResolveReference`, `ResolveEntity` and `ResolvePath` become one-line calls passing `LiveHalf`, and they keep their doc comments. A caller that never heard of the archive goes on compiling and goes on behaving identically, so this is safe to land under four other cards in flight.

`descend` gains a `half ResolutionHalf` parameter and decides, per call, whether the collection it is about to read is the deepest step:

- `len(tail) == 0`, so the segments end on the collection itself. This call is the deepest step.
- `len(below) == 0`, so the member selected here is the entity the reference names. This call is the deepest step.
- `mount.Kind == KindAttachment && below[0] == PayloadDir`, so the only thing past the member is the payload file. This call is the deepest step.
- Otherwise the walk goes deeper, so this call reads the live half and recurses carrying `half` unchanged.

The test is decided from the segment count and the mount kind before the path is joined, and that is load-bearing rather than incidental. In `descend` as it stands at `9260a2a`, `collection := filepath.Join(dir, mount.Dir)` runs at `internal/bench/resolve.go:519`, `tail` is computed at 520 and `MemberIDs(collection, mount)` at 539, so a deepest-step test written in terms of `tail` and `below` decides after the directory it is supposed to choose has already been joined and after the member listing has been read out of it. Written in terms of `segments`, the same three conditions are available at the top of the function: `len(tail) == 0` is `len(segments) == 1`, `len(below) == 0` is `len(segments) == 2`, and the payload case is `mount.Kind == KindAttachment && len(segments) > 2 && segments[2] == PayloadDir`. The join then reads

```go
collection := filepath.Join(dir, mount.Dir)
if deepest && half == ArchivedHalf {
	collection = filepath.Join(dir, ArchiveDir, mount.Dir)
}
```

Getting this wrong compiles and reads the live members in silence, so it is written out here rather than left to be derived. A path longer than the payload case, such as `pb-1/attachments/1/payload/x`, is refused `dinah.unknown-path` at line 555 whichever half was chosen, so the `> 2` spelling changes nothing a reader can observe.

The mirror is read as `ListIDs` reads any collection, so a mirror directory nothing has been written into is an empty collection rather than an error, which is the behaviour the live half already has.

The head is the same test one level up. `resolveBelowLanding` resolves its head in the mirror exactly when the reference carries no collection step below it, which is the case for a bare head and for the four card-file segments. Those four are declared once:

```go
// cardOwnFileSegment reports whether a segment below a card names one of the
// card's own two files rather than a collection. walkBelowCard answers those
// segments ahead of the containment grammar, and the archived-half resolution
// asks the same question to decide whether the head is the reference's
// deepest collection step, so the set is declared once rather than written
// out in both places.
func cardOwnFileSegment(segment string) bool
```

It answers true for `CardAnchor`, `KindCard`, `"journal"` and `JournalName`, which is the exact set `walkBelowCard` already branches on, and `walkBelowCard` is rewritten to call it so the two cannot drift.

The head is resolved twice over, once in `ResolveReferenceIn`'s bare-head branch and once in `resolveBelowLanding`, and neither is written in terms of the other: the first answers an `EntityRef` and the second answers a directory to descend from. So the question of which half the head resolves in is one declared function rather than a condition written out in both places.

```go
// headHalf is the half a reference's head segment resolves in. The head is
// the reference's deepest collection step exactly when nothing below it
// names a collection, which is a bare head and a head followed only by one
// of the card's own file segments. Every other reference resolves its head
// live and carries the archived half down to the step that uses it.
func headHalf(half ResolutionHalf, rest string) ResolutionHalf
```

`resolveBelowLanding` gains the half and calls `headHalf(half, rest)` before it touches the head. Its `b.ColumnByRef(head)` becomes `b.columnByRefIn(headHalf(half, rest), head)` and its `b.ResolveCard(head)` becomes `b.resolveCardIn(b.cardsRootIn(headHalf(half, rest)), head)`. `ResolveReferenceIn`'s bare-head branch passes `half` straight through, because a bare head is always the deepest step. `columnByRefIn`, `cardsRootIn` and `workstreamsRootIn` each answer the live form under `LiveHalf` and the archived form under `ArchivedHalf`, so a caller passing `LiveHalf` compiles to what the function does today.

Head resolution under `ArchivedHalf`, when the head is the deepest collection step:

- A card head goes to `b.resolveCardIn(b.ArchivedCardsRoot(), head)`, which is what `ResolveArchivedCard` already is. `ResolveArchivedCard` stays and keeps its two callers.
- A column head goes to a new `b.ArchivedColumnByRef(ref)`, backed by a new `b.ArchivedColumnsRoot()` returning `filepath.Join(b.Root, ArchiveDir, ColumnsDir)`. `ColumnByRef` reads `b.Columns`, which is the live ordered list and cannot hold an archived column, so the archived form loads the anchors under the mirror instead. It matches identifier first, then slug, then title, in the order `ColumnByRef` matches, and the reason `ColumnByRef` records for that order applies here unchanged.
- A workstream head goes to `workstreamsIn(b.ArchivedWorkstreamsRoot())`, matched identifier first and then slug, which is the order `WorkstreamByRef` uses and for the reason it records. `WorkstreamByRef` itself already spans both halves and is not changed, because a card's membership has to resolve either way.
- The workbench is never reached. Both archived-half entry points refuse it ahead of the walk, per section 4.1, because neither walk fails on it: `ResolveReferenceIn` answers the workbench at its own top and `resolveBelowLanding` answers a bare workbench head with the live anchor and a success. Head resolution under `ArchivedHalf` therefore sees a column, a card or a workstream and nothing else.

`EntityRef` and `CollectionRef` each gain `Archived bool`, reporting whether the answer came out of the mirror. It is what the renderers read to mark a listing and what the machine views carry.

`refBelowHead` gains the half, so the reference composed for an archived entity counts positions in the mirror. A composed reference counting the live half would be an address naming a different entity, which is the defect dinah-454 was widened to remove.

## 3.2 The flag on each of the four commands

`internal/verb/definition.go` gains `{Name: "archived", Flag: true, Marker: true, Shared: "archived", Field: "Archived"}` on `restore`, `show`, `path` and `contents`. The `Shared` name resolves its sentence to `param.archived.summary`, so one written meaning serves the four. `search`'s own declaration is not given `Shared`, so it keeps `param.search.archived.summary` and its own wording, which is the widening sense.

`cmd/dinah/commands.go` sets `req.Archived = parsed.has("archived")` in `runRestore`, `runShow`, `runPath` and `runContents`, which is the line `runSearch` already carries.

`runPath` has a shortcut that answers `path workbench` out of `discoverRoot` without opening the bench at all. That shortcut is guarded on `!parsed.has("archived")`, so `dinah path --archived workbench` falls through to the bench and meets the `dinah.not-archived` refusal rather than printing the live anchor's path.

`Library.Contents` at `internal/verb/tree.go:883` passes `halfFor(req)` into `ResolveReferenceIn`, where `halfFor` is one unexported helper in `internal/verb` returning `bench.ArchivedHalf` when `req.Archived` is set. That is its only resolution and no other change reaches it. `runPath` is in `cmd/dinah` and cannot see `halfFor`, so it computes the half from `parsed.has("archived")` at its own call site.

`Library.Show` is the command this flag fits worst, and it is where round 2's spec was wrong. Show does not resolve once. It has three branches, and two of them never touch the resolver whose refusal the rest of this spec relies on. Traced at `9260a2a`:

- A bare head that `b.ColumnByRef` matches is answered at `internal/verb/read.go:788` by reading the column's anchor. No resolver is called at all.
- A composed reference asks the collection question at `:816` and **discards the resolver's error by design**, the reason being written out in the comment at `:809`, then takes `ResolvePath`'s answer and `ResolvePath`'s error at `:823`.
- A bare head that matches no column goes to `ResolveCard` at `:833`, which reads the live cards root.

So Show under `--archived` gets one new branch of its own rather than four resolvers each handed a half. When `req.Archived` is set and `rest == ""`, Show resolves once through `l.Bench.ResolveReferenceIn(bench.ArchivedHalf, req.Card)` and switches on what comes back: a `bench.KindCard` answer carries the archived card in `EntityRef.Card` and feeds the `Detail` build directly, a `bench.KindColumn` answer is read with `bench.ReadText(filepath.Join(entity.Dir, bench.ColumnAnchor))`, and an error is returned as it stands. A non-card answer with `req.Fields` set raises `unknownDetailField`, which is what the live column branch raises today. The workbench cannot come back, because section 4.1 refuses it inside `ResolveReferenceIn`.

Show's composed branch keeps its shape exactly, `:816` included, discarded error and all. It becomes `ResolveReferenceIn` and `ResolvePathIn` carrying `halfFor(req)`, and it stays correct under the flag because `ResolvePathIn` on the following line raises the same refusal `ResolveReferenceIn` would have. That equivalence is not an accident of this draft; it is why section 4.1 puts the refusal in both entry points rather than in one. The one reference the two answer differently is an attachment's payload, which `ResolveReferenceIn` refuses `dinah.unknown-path` and `ResolvePathIn` answers with a path, and discarding the first is what lets `show --archived pb-1/attachments/1/payload` go on working.

`Library.Show`'s bare-card branch calls `l.lapseRead(card, req.Actor)`, which expires a lapsed claim and writes an `expired` event. The new archived branch does not call it. An archived card is out of the flow by construction, so lapsing its claim is a write to history nobody asked for, and a read command that writes to an archived journal is a surprise this contract does not want. Round 2's spec guarded the existing call with a condition; a separate branch that never makes the call is the same behaviour with nothing to get wrong.

`dinah edit` shares two of these call sites in shape but not in code, at `cmd/dinah/commands.go:1269` and `:1272`, and it declares no `archived` parameter, so both stay on `LiveHalf` and nothing about `edit` changes.

`--archived` on `restore` is redundant and is declared anyway. `restore` always resolves in the mirror, because restoring a live entity is not an act, so passing the flag changes nothing and omitting it changes nothing. dinah-456 section 5.5 names `restore` among the four commands the flag reaches, and the spelling earns its place beyond that ruling: a reader who found an entity with `dinah show --archived pb-1/comments/1` restores it by changing one word of the line they already have. The alternative considered and not taken was to leave the flag off `restore`, on the ground that a flag no value changes is not a flag. It was not taken because the parent named it and because the symmetry is worth more than the purity here, and this paragraph is on the record so nobody has to reconstruct the choice.

# 4. The refusals

One name is minted. Two existing names gain a raise site, and one of those two needs its sentence corrected.

## 4.1 `dinah.not-archived`, minted

dinah-456 section 3.4 minted `dinah.is-a-collection` and `dinah.not-attachable` rather than widening `dinah.unknown-path` a third time, and both were read against this case before a third name was minted. `dinah.is-a-collection` is about a reference naming a set where one thing was wanted, which is a different mistake. `dinah.not-attachable` is about a kind that mounts no collection, which is a statement about the containment table rather than about which half an entity is in. Neither fits, and `dinah.unknown-path` and `unknown-card` are both false here, because the entity exists and the reader can see it. So the third name is minted, under `contract.LayerPrefix` because it is Dinah's own refusal rather than a profile name, in the lower-case hyphen-joined shape `not-renamable` and `not-attachable` already use.

```go
// NotArchived is raised when the archive mirror holds nothing the reference
// names and the reader can nevertheless see the thing they typed, either
// because it is live or because it travelled inside an archived holder. It
// is what an --archived read and every restore ask for.
NotArchived = LayerPrefix + "not-archived"
```

### Where it is raised, traced per command

Round 2 of this spec said the refusal was raised in `Bench.ResolveReferenceIn` and that all four commands therefore answered alike. Two of the four never read that function's error. `Library.Show` discards it at `internal/verb/read.go:816` on purpose, and `runPath` reaches `ResolvePath`, which never calls `ResolveReference` at all. What follows was produced by reading each command's call chain from its entry point down to the function that would fail, rather than by reading the resolver and assuming its callers.

The refusal is decided in one declared function and raised nowhere else:

```go
// notArchivedFor is the only place NotArchived is raised. Both archived-half
// entry points call it twice: once before the walk, passing a nil failure,
// and once on the walk's failure, passing the error it failed with.
//
// The pre-walk call exists because neither walk fails on the workbench.
// ResolveReferenceIn answers the workbench at its own top and
// resolveBelowLanding answers a bare workbench head with the live anchor and
// a success, so a refusal left to the failure path would never be reached
// for the one reference the archive can never hold.
//
// A pre-walk call answers nil for every reference but the workbench's. A
// failure call answers either NotArchived or the failure unchanged, and
// never nil.
func (b *Bench) notArchivedFor(ref string, failure error) error
```

`ResolveReferenceIn` and `ResolvePathIn` are its only callers, and both call it only under `ArchivedHalf`:

```go
func (b *Bench) ResolvePathIn(half ResolutionHalf, ref string) (string, error) {
	if half == ArchivedHalf {
		if refusal := b.notArchivedFor(ref, nil); refusal != nil {
			return "", refusal
		}
	}
	path, err := b.resolvePathBody(half, ref)
	if err != nil {
		if half == ArchivedHalf {
			err = b.notArchivedFor(ref, err)
		}
		return "", err
	}
	return filepath.Abs(path)
}
```

`ResolveReferenceIn` carries the same two calls, its pre-walk one placed above the `ref == "" || IsWorkbenchRef(ref)` return that answers the workbench today. `ResolveEntityIn` needs neither, because it is `ResolveReferenceIn` plus the collection refusal.

Two entry points rather than one, because neither can be written in terms of the other. `ResolvePathIn` answers an attachment's payload, which `ResolveReferenceIn` refuses because a payload carries no anchor, and `ResolveReferenceIn` answers a collection separately, which `ResolvePathIn` answers as a directory path. Their own doc comments say so at `9260a2a`. The rule is still in one place; what is in two places is the call.

The call chain each command actually takes, traced from the command table down:

| Command | Reference | The chain, at `9260a2a` plus this card | Where the refusal is raised |
|---|---|---|---|
| `restore` | any | `runRestore` → `Library.Restore` → `Bench.ResolveEntityIn` → `Bench.ResolveReferenceIn` | `ResolveReferenceIn` |
| `contents` | any | `runContents` → `Library.Contents` (`internal/verb/tree.go:883`) → `Bench.ResolveReferenceIn` | `ResolveReferenceIn` |
| `show` | bare head | `runShow` → `Library.Show`, the new archived branch of section 3.2 → `Bench.ResolveReferenceIn` | `ResolveReferenceIn` |
| `show` | composed | `runShow` → `Library.Show` → `ResolveReferenceIn` at `read.go:816`, **error discarded** → `Bench.ResolvePathIn` at `:823` | `ResolvePathIn` |
| `path` | `workbench` or `.` | `runPath`'s pre-bench shortcut at `cmd/dinah/commands.go:1217`, guarded on the flag per section 3.2, falls through to `Bench.ResolvePathIn` at `:1229` | `ResolvePathIn`, on the pre-walk call |
| `path` | everything else | `runPath` → `Bench.ResolvePathIn` at `cmd/dinah/commands.go:1229` | `ResolvePathIn` |

Three of those rows are the ones round 2 got wrong, and each is wrong in its own way rather than all three being one mistake. `show` on a bare head reached neither entry point, so it answered `unknown-card` for a live card, a live column and the workbench alike. `show` on a composed reference reached `ResolvePathIn` and threw away the answer of the function the spec named. `path` reached `ResolvePathIn` for every reference and `ResolvePathIn` raised nothing, and for the workbench head it did not even fail: `resolveBelowLanding` answers a bare workbench head at `internal/bench/resolve.go:399` with the live anchor path and a nil error, so `dinah path --archived workbench` printed the live path and exited 0.

### The refusal probe, and why it is not the answer rule

Section 3's rule decides the answer, and it decides it from the remaining segments and the mount kind alone. Nothing in the answer path looks in a second place, and that is what makes the rule determinate. The refusal path is where a second look is affordable, because its whole output is which of two refusal names to print.

So `notArchivedFor`, given the error the `ArchivedHalf` walk failed with, runs one diagnostic walk over the same reference. The diagnostic reads the live half first and the mirror second at every collection step, taking the first that holds the member the segment names. It answers with a boolean and a string, never an `*EntityRef`, and `notArchivedFor` answers an `error` and nothing else, so nothing the diagnostic reaches can become an answer to a caller on either entry point.

Three cases come out of it, and each fills the refusal's values differently.

- The diagnostic fails. Nothing in either half answers to the reference, so the mirror's own error travels unchanged and the reader goes on getting `unknown-card` or `dinah.unknown-path` with the sentence it renders today.
- The diagnostic succeeds having read only live halves. The entity exists and is not archived. `NotArchived` is raised with `holder` empty, and with `collection` filled when the reference carries a collection step below its head, so `pb-1/comments/3` fills it with `pb-1/comments` and `pb-1` leaves it empty.
- The diagnostic succeeds having read the mirror at one or more steps. The reference names something that travelled inside an archived holder. `NotArchived` is raised with `holder` holding the reference truncated to the member selected at the first step the diagnostic read out of the mirror, and with `collection` empty.

Live first rather than mirror first is deliberate here, and it is not the reading section 3's decision refuses. That decision is about which half an answer comes from. This order only decides which of two true sentences a refused reader gets, and live first makes the live case win any tie, so the advice never tells a reader to restore something that is standing in front of them.

This is what produces the refusal section 3 promises for an entity inside an archived holder. `dinah show --archived pb-1/comments/1` with `pb-1` archived resolves its head live under the rule and fails; the diagnostic then finds the comment by reading the mirror at the head, so the reader gets `dinah.not-archived` with `holder` filled as `pb-1` and an advice line naming `dinah restore pb-1`. The first draft of this section said the retry asked only whether the reference resolved live, which cannot produce that refusal, while section 3 and the criterion pinning it both promised it anyway. The mechanism has moved rather than the promise, because the promise is the better product and the diagnostic costs one walk on a path that is already refusing.

A reference one level deeper works the same way. With the card live and its first comment archived, `pb-1/comments/1/attachments/2` reads its head live, fails live at `comments/1`, answers there in the mirror, and fills `holder` with `pb-1/comments/1`. The advice is then `dinah restore pb-1/comments/1`, which is the act that makes the reference resolve.

The workbench head raises `NotArchived` with no diagnostic at all, because the workbench is never archived and `Library.Archive` refuses it at `internal/verb/beyond.go:238`. That is the pre-walk call, and it fires when the reference is empty or when `bench.IsWorkbenchRef` accepts the head and nothing follows it, which is `workbench` and `.` and nothing else. It fills `slug` with `Bench.Slug` and leaves the other two values empty, and it fills `detail` with `bench.WorkbenchRef` when the reference as typed is empty, so no raise renders a base sentence ending in nothing.

### What the reader reads

`restore` and the three read commands need different base sentences, because "there is nothing to restore" is not an answer to somebody who asked to look at something. `Shapes` already carries the mechanism for one name answering two acts, which is `Variants` selecting a `refusal.<name>.<command>` base entry, and section 4.2 uses that same mechanism on `dinah.exists`. So `NotArchived` declares `Variants: []string{"restore"}` and two base entries.

The next step is an ordered alternation of five, of which exactly one renders. A fragment carries at most one condition, which `contract.Fragment`'s own declaration states, so the branches are separated by their order rather than by compound conditions.

| Key | Text |
|---|---|
| `refusal.dinah.not-archived` | `nothing in the archive answers to {detail}` |
| `refusal.dinah.not-archived.restore` | `{detail} is not archived, so there is nothing to restore` |
| `refusal.dinah.not-archived.next-holder` | `; {holder} is archived and this entity travelled inside it, so run "dinah restore {holder}" to bring both back` |
| `refusal.dinah.not-archived.next-workbench` | `; the workbench {slug} is never archived, so name a column, a card, or something below a card instead` |
| `refusal.dinah.not-archived.next-collection` | `; run "dinah show --archived {collection}" to see what that collection holds in the archive` |
| `refusal.dinah.not-archived.restore.next` | `; run "dinah archive {detail}" first if moving it out of the live set is what you meant` |
| `refusal.dinah.not-archived.next` | `; drop "--archived" to read it where it is, in the live set` |

Every command spelling and the flag are written in backticks in the catalogue, in the shape every neighbouring entry uses; the table above writes them with quotes only because it is inside a fenced cell. `--archived` is backticked as a span of exactly one token, which is what section 7's guard reads.

Each base sentence is true at every raise site it serves. `nothing in the archive answers to {detail}` is true of a live entity, of an entity inside an archived holder, and of the workbench, and it says nothing about restoring to a reader who typed `show`, `path` or `contents`. `{detail} is not archived, so there is nothing to restore` is true of the same three, because an entity that travelled inside its holder was never archived in its own right.

Each advice line can be carried out by the reader who gets it, which is the test Convention counterexamples 1 states for a refusal reused across shapes. Every branch was walked by hand against the four commands and the three cases:

| Command | Case | Fragment | Is the advice takeable |
|---|---|---|---|
| `restore` | live head | `restore.next` | yes, `dinah archive pb-1` archives a live card |
| `restore` | live member of a collection | `next-collection` | yes, the mirror listing is what tells the reader there is something to restore |
| `restore` | inside an archived holder | `next-holder` | yes, restoring the holder is the route section 3 names |
| `restore` | workbench | `next-workbench` | yes, and this is the branch the first draft got wrong, because `dinah archive workbench` is refused by design and no branch now offers it |
| `show`, `path`, `contents` | live head | `next` | yes, dropping the flag reads the live entity |
| `show`, `path`, `contents` | live member of a collection | `next-collection` | yes |
| `show`, `path`, `contents` | inside an archived holder | `next-holder` | yes |
| `show`, `path`, `contents` | workbench | `next-workbench` | yes |

`restore` never reaches the unconditional last member, because one of the four before it always fires for it, and that last member is worded for a reader who asked to look rather than for one who asked to restore.

The shape, in `internal/contract/shape.go`:

```go
{
	// A reader who types one of these four commands against something they
	// can see is told which half it is in rather than that it does not
	// exist, which is the mistake dinah.is-a-collection was minted against
	// one layer up. The name answers two acts and four commands, so restore
	// carries its own base sentence and the alternation carries one branch
	// per case the reader can be in: inside an archived holder, at the
	// workbench, naming a member of a collection, or asking to archive
	// something still live. Each branch names an act the reader can carry
	// out.
	Name:     NotArchived,
	Values:   []string{"holder", "slug", "collection"},
	Variants: []string{"restore"},
	Fragments: []Fragment{
		{Key: "refusal.dinah.not-archived.next-holder", When: "holder"},
		{Key: "refusal.dinah.not-archived.next-workbench", When: "slug"},
		{Key: "refusal.dinah.not-archived.next-collection", When: "collection"},
		{Key: "refusal.dinah.not-archived.restore.next", WhenCommand: "restore"},
		{Key: "refusal.dinah.not-archived.next"},
	},
	NextStep: []string{
		"refusal.dinah.not-archived.next-holder",
		"refusal.dinah.not-archived.next-workbench",
		"refusal.dinah.not-archived.next-collection",
		"refusal.dinah.not-archived.restore.next",
		"refusal.dinah.not-archived.next",
	},
},
```

The workbench's value is named `slug` rather than `workbench`, and the difference is not cosmetic. `contract.ValueWorkbench` is declared at `internal/contract/shape.go:103` as the workbench **directory** discovery resolved for the invocation, and it is the one value in that set a raise site may also fill, which `nameTheWorkbench` at `cmd/dinah/main.go:476` reads when it decides whether to attach one. A shape spelling its own value `workbench` and filling it with `Bench.Slug` puts a slug where every other reader of that name expects an absolute path. Nothing renders wrong today, because `NotArchived` is not in `benchScopedAdvice` and so `nameTheWorkbench` never reaches it, and a reader adding it to that table later would get a value silently overwritten with a directory in one branch's sentence. `ConflictingScope` at `internal/contract/shape.go:867` already carries the collision and is out of this card's scope; this card declines to add a second instance. `slug` is free: no shape declares it and no catalogue entry uses `{slug}` for anything but a workbench slug, which `root.workbench` already does.

The three values are declared because three entries name them. That is the rule `NotAttachable` follows for `item` and `attachment`, and it is the opposite of the rule `IsACollection` follows for `count`, which is carried for a machine caller, named in no entry, and therefore left undeclared. `internal/profile/guards_test.go`'s `checkNoPlaceholderIsStrayOrOrphaned` fails a value declared and unused and a placeholder used and undeclared, and `checkEveryShapeSaysWhatToDoNext` fails a `NextStep` ending on a conditional member and a variant carrying no fragment of its own, so all four of those mistakes are caught by a guard that already runs on every build.

`checkOneRefusalIsOneDeclaration` refuses a shape declaring both `Variants` and `Subject`, so `NotArchived` declares no `Subject` and reaches no empty-subject case. `detail` is the reference as typed everywhere but one, and no raise carries an empty one: the pre-walk call substitutes `bench.WorkbenchRef` for a reference typed empty, per the workbench paragraph above, and every other raise happens on a walk that had a reference to fail on.

## 4.2 `dinah.exists`, whose sentence is wrong for this raise site

`Bench.Run` already refuses `contract.Exists` when the destination of a structural move exists, and that is the refusal dinah-456 section 5.5 names for a live entity occupying the slot. The sentence it renders today is written for `init` and `extract`:

```
refusal.dinah.exists          {detail} already carries a workbench.md
refusal.dinah.exists.next     ; move it aside, or choose a different directory
```

A restore refused that way would tell a reader their card directory carries a `workbench.md`. `Shapes` already has the mechanism for one name answering two acts, which is `Variants`, and `Unconfirmed` and `NoReason` both use it. `Exists` gains `Variants: []string{"restore"}` and two entries:

| Key | Text |
|---|---|
| `refusal.dinah.exists.restore` | `something live already stands at {detail}, so the archived entity has nowhere to go back to` |
| `refusal.dinah.exists.restore.next` | `; move or delete what stands there, then restore again` |

The existing two entries keep their wording, and the shape's `Fragments` and `NextStep` gain the restore fragment ahead of the unconditional one, switched on `WhenCommand: "restore"`.

How reachable this is, said plainly rather than dressed up. The slot is `<collection>/<id>` where `<id>` is the entity's own twelve-hex identifier, and identifiers do not collide in practice: a card's is claimed by `bench.ClaimID` against `Bench.HasIdentifier`, which spans both halves, and every other kind's is claimed by mkdir inside its collection. So this refusal guards a hand-edited tree or a race, and not a case a reader will meet in ordinary use. It is kept because `Bench.Run` already raises it and because a refusal rendering a false sentence is worse than one nobody meets.

## 4.3 `dinah.is-a-collection`, which restore inherits

dinah-456 section 3.3 rules that a writing command refuses a collection reference, and `restore` is a writing command, so its cell in the guide's table is `no` and it refuses `dinah.is-a-collection`. The refusal is raised by `ResolveEntityIn` through `CollectionRef.Refuse`, which is where every other writing command's is raised, and no new message key is needed. Section 11 says what this card does to the argument that rule rests on.

## 4.4 What restore refuses that archive accepts, and what it does not

`archive` accepts a column, a card, a comment, a checklist item, an attachment and a workstream, and refuses the workbench. `restore` accepts the same six kinds and refuses the same workbench, so nothing archive accepts is refused for being the kind it is. The three refusals above are all about the state of the reference rather than the kind it names: `dinah.not-archived` when it is live, `dinah.is-a-collection` when it names a set, and `dinah.exists` when the slot is taken.

Two checks `archive` runs are deliberately not run by `restore`, and one of them is a change to `Bench.Run`.

The last-column check is already skipped, and stays skipped. `Bench.Run` guards it with `act.Op != OpRestore`, and that is right, because a restore adds a column rather than removing one, and CORE-BENCH-2 is about the workbench keeping one.

The occupancy scan is skipped on a restore, and that is a one-line change. `Bench.Run` runs `b.ColumnOccupied(act.ColumnID, act.ColumnRef)` for any act carrying a column, restore included, and refuses `dinah.occupied` when a live card names the column. On an archive that is the point, because archiving a column out from under a card strands the card. On a restore it is backwards: a live card naming a column the workbench does not list is the stranded state `dinah check` reports as `check.unknown-column`, and restoring the column is the repair. Refusing the repair because the damage exists is the shape this workbench calls a check that fires against correct code. The guard becomes `if act.ColumnID != "" && act.Op != OpRestore`, which is the condition the two lines below it already carry. The `act.Op != OpRestore` at `internal/bench/entity.go:585`, which guards the last-column check inside that block, is then dropped: a restore can no longer enter the block at all, so the inner test is dead and leaving it there asks the next reader to work out which of the two is load-bearing. The last-column check reduces to `if len(b.Columns) <= 1`, and what it does is unchanged, since a restore skipped it before this card and skips it after.

No capacity check runs. `Bench.Run` performs none for any op, and `Library.Add` is where capacity is enforced. A restored card returns to the column its own anchor names, over that column's limit if that is where it was, because a limit governs work being pulled in rather than history reappearing.

# 5. Restoring each kind

Every kind restores through the one `StructuralAct`, and the notes below are what is true of each beyond that.

A card returns to `cards/<id>` carrying the column its anchor names. When that column was deleted or archived while the card was away, the restored card stands in a column the workbench does not list, and `dinah check` reports it under `bench.FindingUnknownColumn` exactly as it reports the same condition arising any other way. `restore` does not rewrite the anchor to some other column, because choosing one would be inventing a decision on the reader's behalf and the reader can `dinah move` the card in one command.

A column returns to `columns/<id>`, and its identifier has to go back into the workbench anchor's `columns` sequence, which is the single authority for order that `format.md`'s flow-definition section names. `internal/bench/columnretire.go` gains the counterpart to `RemoveColumnID`:

```go
// AddColumnID appends one identifier to the workbench's own ordered columns
// list and writes the anchor back, preserving every other key exactly as
// RemoveColumnID does. It is restoration's own write to the definition, made
// while the bench lock the restoring act already holds is still in force.
// Adding an id already present is a no-op, which is what makes a second call
// over the same bench safe.
func (b *Bench) AddColumnID(id string) error
```

`Bench.Run` calls it on the restore branch where it calls `RemoveColumnID` on the others, after `act.apply()` and before its sixth step, so the anchor never names a column whose directory is not yet there.

The restored column lands at the end of the order. A column's position is recorded nowhere but the workbench anchor's sequence, and archiving removes it from that sequence, so the position is gone by the time a restore runs. The three ways to get it back are all worse than appending: recording a position on the column anchor at archive time adds a format key that means nothing while the column is live, recovering it from the journal makes the order depend on replay, and refusing to restore columns at all would make `restore` an inverse of `archive` for five kinds out of six. Appending is honest, it is one line, and `dinah reshape` already exists for moving a column where the operator wants it. The references guide says so in one sentence, per section 9.

A workstream returns to `workstreams/<id>`. `Bench.Workstream` and `WorkstreamByRef` already span both halves, so a card's membership resolved while the workstream was archived and goes on resolving after it comes back. `StructuralAct.WorkstreamID` is filled for a deletion and left empty for an archive, and `Restore` leaves it empty for the reason `Archive` does: the membership scan exists to stop a deletion stranding a card, and nothing about a restore can strand one.

A comment, a checklist item and an attachment return to the live half of the collection below their holder. `MemberIDs` sorts by stored ordinal, so a restored member lands where its ordinal puts it rather than at the end, and `verb.memberPosition` counts the place rather than reading the field, so the positions every surface prints stay a contiguous count from one. The consequence a reader needs is that the address they typed under `--archived` is not the address the entity has once it is live, and the guide says so in the sentence that already says a position is a spelling for now and an identifier is a handle to keep.

The workbench is never archived and has no restore, which dinah-456 section 5.4 already records as deliberate.

# 6. The journal, the format document, and the two derivations

`restored` becomes an event this build writes, and three things in the tree assert that it is not.

`unwrittenEvents` empties. `cmd/dinah/compat_test.go`'s map has one entry and it goes. The map declaration stays, with its comment, because `TestEveryDeclaredEventLandsInACapture` reads it and because the comment above it is the argument for the guard.

`scripts/derive_event_counts.py` cannot read an empty map, and fixing that is this card's work rather than a reason to leave a dead entry behind. Its pattern is `(?s)var unwrittenEvents = map\[string\]string\{(.*?)\n\}`, which needs a newline before the closing brace. `gofmt` writes an empty map as `map[string]string{}` on one line, so the pattern will not match and the script exits through `SystemExit("cmd/dinah/compat_test.go declares no unwrittenEvents map this script can read")`. The pattern becomes `(?s)var unwrittenEvents = map\[string\]string\{(.*?)\}`, which matches both spellings and yields an empty body for the empty map, and `constant_names` already returns an empty list for an empty body. The script's docstring gains a sentence saying an empty exemption map is the expected shape once every declared event is written.

The population sequence gains a restore and the fixture is recaptured. `TestEveryDeclaredEventLandsInACapture` fails for any declared event no capture carries and `unwrittenEvents` does not exempt, so `restored` has to reach a card journal. `internal/bench/testdata/compat/populate.txt` gains an `archive` of something below the sample card followed by a `restore` of it, so the card's own journal carries both events in order. The capture is produced with `scripts/capture_fixture.py` rather than by hand, and it recaptures only the fixture for the revision this build stamps, leaving every older revision untouched, which is the workbench's standing rule on those fixtures. The manifest digest is re-blessed in the same diff.

`docs/design/format.md`'s "Journal event schema" section is rewritten to what the derivation says. The script derives the numbers, so the values below are what it will report after the change rather than numbers typed into prose. At `9260a2a` it reports `DECLARED 32, UNWRITTEN 1, WRITTEN 31, QUERYABLE 29, OVERLAP 28, CARD 27`. Afterwards `UNWRITTEN` is 0, `WRITTEN` is 32, `OVERLAP` is 29, and `CARD` is 28. The edits:

- "Thirty-one of them are written by some command in this build" becomes "Thirty-two".
- The two sentences after it, which name `restored` as declared and reserved and point at `unwrittenEvents` for the reason, go. The paragraph then says every declared event is written by some command, and names the guard that keeps it so.
- "Twenty-eight names sit in both counts" becomes "Twenty-nine", and "Twenty-seven of those land on a card's own journal" becomes "Twenty-eight".
- The sentence placing `restored` in the twenty-nine and outside the thirty-one goes, because the set it describes is now empty. The script generates no check for a membership that does not exist, so nothing would catch that sentence, and it is named here for that reason.
- The sentence placing `column_updated`, `workbench_updated` and `workstream_updated` in the thirty-one and outside the twenty-nine reads "in the thirty-two".
- The paragraph beginning "Every row but `restored`'s describes lines this build writes", and the paragraph after it explaining that the `restored` row was read off `finish.go`'s `eventRecords` rather than off writing code, are replaced by one sentence saying the row is now read off `Library.Restore` as every other written row is read off its own writer. The `restored` row of the event table does not change: it carries `note` and nothing else, which is what `Library.Restore` writes and what `eventRecords` requires.
- The concurrency section's restore paragraph is already correct and is not touched. Its sentence about the event being appended before the move is what `Library.Restore` implements.

One stray board reference in `format.md` is removed in the same pass. The script's last check refuses a bare card identifier in published text, and `format.md` line 20 carries one inside a terminology note. The script exits 1 at `9260a2a` for that reason alone and for no other, so a criterion asserting a clean exit would fail against a defect this card did not cause. The sentence loses the identifier and keeps its meaning, which makes the script's exit status usable as a check and is why this edit is in scope rather than filed away.

`internal/verb/checks.go` gains a `restore` list. Its rows are the refusals `Restore` raises, in the order it raises them, which is what a per-verb help listing prints:

| Order | Refusal | Key | English |
|---|---|---|---|
| 1 | `contract.NotArchived` | `check.restore.1` | the reference resolves to something in the archive |
| 2 | `contract.NoOwner` | `check.restore.2` | the request names who is acting |
| 3 | `contract.Exists` | `check.restore.3` | nothing live stands where the entity goes back |

`check.archive.2`, the occupancy row, has no counterpart here, because section 4.4 rules that scan out of a restore.

# 7. The message catalogues

Fifteen keys are added, in all eight catalogues under `internal/msg/locales/`, under the workbench document "Translation staleness contract".

| Key | English |
|---|---|
| `cmd.restore.summary` | Put a card, a column, or anything below a card, back into the live set |
| `param.archived.summary` | resolve the reference in the archive rather than in the live set |
| `check.restore.1` | the reference resolves to something in the archive |
| `check.restore.2` | the request names who is acting |
| `check.restore.3` | nothing live stands where the entity goes back |
| `refusal.dinah.not-archived` | nothing in the archive answers to {detail} |
| `refusal.dinah.not-archived.restore` | {detail} is not archived, so there is nothing to restore |
| `refusal.dinah.not-archived.next-holder` | ; {holder} is archived and this entity travelled inside it, so run `dinah restore {holder}` to bring both back |
| `refusal.dinah.not-archived.next-workbench` | ; the workbench {slug} is never archived, so name a column, a card, or something below a card instead |
| `refusal.dinah.not-archived.next-collection` | ; run `dinah show --archived {collection}` to see what that collection holds in the archive |
| `refusal.dinah.not-archived.restore.next` | ; run `dinah archive {detail}` first if moving it out of the live set is what you meant |
| `refusal.dinah.not-archived.next` | ; drop `--archived` to read it where it is, in the live set |
| `refusal.dinah.exists.restore` | something live already stands at {detail}, so the archived entity has nowhere to go back to |
| `refusal.dinah.exists.restore.next` | ; move or delete what stands there, then restore again |
| `contents.archived` | These addresses resolve once {ref} is restored. |

That is fifteen rather than the eleven the first draft of this section counted, and the four extra are all in section 4.1: a base entry for the `restore` variant and three of the five next-step branches.

`cmd.restore.summary` follows `cmd.archive.summary`, which reads "Move a card, a column, or anything below a card, out of the live set", so the pair reads as a pair.

Every entry carries a `context` written for a translator, saying what raises it and what each placeholder holds, in the shape the neighbouring entries use. `de.json` and `hi.json` carry real translations, each stamped with `msg.Fingerprint` of the English text in a `source` field, which is the FNV-1a 64-bit digest formatted as lower-case hex that `internal/msg/msg.go` computes. `cs.json`, `id.json`, `es.json`, `fil.json` and `af.json` carry the English text byte for byte with `"skeleton": true` and no `source`. The per-key decision-record rule in the translation document applies, because these are new keys rather than skeleton fills.

## 7.1 The guard whose subject set this card widens

Machine vocabulary is not translated, and that rule is not stated by the entries' contexts. It is enforced by `TestContractTokensSurviveInBackticks` in `internal/verb/contract_tokens_test.go`, which asserts that a name this project declares travels into every translation byte for byte wherever the English marks it as the literal thing by quoting it in backticks. dinah-245 coined three German names for one flag across four strings, and that test is the guard written to refuse them.

The implementer needs to know it exists before writing German, because this card widens what it covers without editing it. `contractTokens` composes its set from every name `Commands()` returns and, for each of those commands, every flag in its `params` entry. Adding `restore` to `Commands()` puts the bare token `restore` into the set, and declaring `archived` on `restore`, `show`, `path` and `contents` puts `--archived` into it. Both then bind every catalogue, including the five skeletons, from the moment the code lands.

Its selector is narrower than the guard sounds, and the difference matters for the entries above. `literalTokens` takes a backticked span only when the span is exactly one token, so `` `--archived` `` in `refusal.dinah.not-archived.next` is guarded and must survive into de and hi unchanged, while `` `dinah show --archived {collection}` ``, `` `dinah restore {holder}` `` and `` `dinah archive {detail}` `` are multi-word spans and are not guarded at all. A translator who renders one of those three command lines into German words breaks the advice and no test fires, so the contexts on those three entries say outright that the whole backticked span is a command line to be copied rather than read.

Nothing in this card is a reason to widen `literalTokens` to multi-token spans. That selector is deliberately narrow for the reason its own comment records, which is that word-boundary matching over every English sentence fails a correct translation of an ordinary verb, and changing it is a card about the guard rather than a line in this one.

# 8. Both heads

`restore` joins the MCP roster. `internal/mcp/tools.go` gains `{name: "restore", command: "restore", run: func(l *verb.Library, r *verb.Request) any { return l.Restore(r) }}`, directly after the `archive` entry. `internal/mcp/roster_test.go` requires every library command to be served or named in `toolExemptions` with a reason, so an omission fails the build rather than shipping a capability a person has and an agent does not. `toolExemptions` is not touched: `path` keeps its entry and its reason, and `restore` needs none.

`--archived` reaches the MCP head on `show` and `contents` without further work, because the schema generator publishes a command's declared parameters. It does not reach `path`, which is exempt from the roster and served nowhere.

dinah-456 section 6 says `get_field`, `set_field` and `restore` join the roster under the served-unless-exempt rule, and that dinah-460 is the card reshaping `toolExemptions` to carry a declared ground. This card adds one tool and changes nothing about the exemption shape.

The machine views carry the half. `verb.Tree` gains `Archived bool`, and `verb.CollectionListing` gains one too, both filled from the resolved answer's `Archived`. A client reading a listing then knows which half it is looking at without parsing the command line it sent.

# 9. The references guide

`restore` points its reader at the references guide, so `internal/verb/definition.go`'s `guides` map gains `"restore": {"references"}` and its `ref` parameter carries `Guide: "references"`. `verb.ReferenceTakingCommands` requires those two declarations to agree, and `internal/verb/collection_roster_test.go` holds them to one another.

The guide's command table gains a sixteenth row. dinah-457 landed the derivation at `863b7c5`, so the table's roster is no longer hand-maintained: `TestTheReferencesGuideTableNamesEveryCommandThatTakesAReference` compares the table's rows against `commandsTakingAReference()` and fails in both directions, and its third assertion compares the two counts. Adding `restore` to `guides` without adding the row fails it, so this paragraph is a check rather than a reminder.

The row:

| Command | A workbench | A column | A card | Below a card | A collection |
|---|---|---|---|---|---|
| restore | no | yes | yes | yes | no |

`referenceProbeArgs` gains a `restore` case returning `nil`. That helper is the one hand-written table in `references_guide_test.go`, and it calls `t.Fatalf` naming any command it does not know, so a sixteenth command entering the roster stops the run rather than being probed with the wrong line. `restore` takes a bare reference and nothing else, so it joins the case already listing `path`, `edit`, `show`, `instructions`, `contents`, `attachments` and `archive`.

`restore` is not added to the guide's workstream sentence, and that is deliberate. `TestTheReferencesGuideNamesTheCommandsThatTakeAWorkstream` runs every roster command against a live `workstream/<slug>` and requires exit 0 of exactly the commands that sentence names in backticks. `dinah restore workstream/<slug>` on a live workstream refuses `dinah.not-archived` and exits non-zero, so naming `restore` there would redden the test. The sentence stays at six commands. That `restore` reaches an archived workstream is said in the `--archived` paragraph instead, which is a different paragraph and therefore outside what `backtickedCommandsIn` reads.

`TestTheReferencesGuideDeclaresEveryCommandTheCollectionColumnIsAheadOf` probes `restore <card>/comments` against a live card and compares the result with the table's collection cell. `restore` refuses `dinah.is-a-collection` and exits non-zero, which agrees with the `no` cell, so `restore` must not appear in that test's caveat paragraph either. A command named there that agrees with its cell fails the test.

What the guide gains in prose, beyond the row:

- One paragraph on `--archived`, carrying the one-sentence rule of section 3, the deepest-collection-step rule, the statement that positions count the mirror's own members, and the statement that a read under the flag shows the archived half alone where the same flag on `search` scans both.
- One sentence saying an entity archived inside its holder comes back with the holder, and that the way to reach it is to restore the holder.
- One sentence saying a restored column lands at the end of the column order and that `dinah reshape` moves it.
- One sentence saying a reference printed under `contents --archived` below the walk's root resolves once the root is restored.

`internal/guide/guides/query.md` is not touched. The seam between the two languages is dinah-457's paragraph, and this card adds no field selection to either.

The refusal names the guide quotes follow dinah-456 section 3.2's prefix rule, so `dinah.not-archived` and `dinah.exists` are written with the prefix and `unknown-card` without one.

# 10. What lands, file by file

| File | Change |
|---|---|
| `internal/contract/contract.go` | `NotArchived` constant |
| `internal/contract/shape.go` | `NotArchived` shape; `Exists` gains `Variants: []string{"restore"}` and its fragment |
| `internal/bench/resolve.go` | `ResolutionHalf`, `LiveHalf`, `ArchivedHalf`; `ResolveReferenceIn`, `ResolveEntityIn`, `ResolvePathIn`, `resolvePathBody`; `descend`, `resolveBelowLanding`, `walkBelowCard`, `collectionAt`, `collectionHolder` carry the half; `headHalf`, `columnByRefIn`, `cardsRootIn`, `workstreamsRootIn`, `cardOwnFileSegment`; `ArchivedColumnByRef`; the archived workstream head; `notArchivedFor` and the refusal probe inside it |
| `internal/bench/bench.go` | `ArchivedColumnsRoot` |
| `internal/bench/entity.go` | `EntityRef.Archived`, `CollectionRef.Archived`; `refBelowHead` carries the half; `ResolveEntity` delegates; `Bench.Run`'s occupancy guard gains `act.Op != OpRestore` and the now-dead inner test at line 585 goes; `Bench.Run` calls `AddColumnID` on the restore branch |
| `internal/bench/columnretire.go` | `AddColumnID` |
| `internal/verb/beyond.go` | `Library.Restore`; `halfFor` |
| `internal/verb/read.go` | `Library.Show` gains its own bare-head branch under `--archived`, resolving once through `ResolveReferenceIn` and never calling `lapseRead`; the composed branch's two resolutions carry the half and the discarded error at `:816` stays discarded; `halfFor`; `CollectionListing.Archived`, and its doc comment stops saying the members are the live ones |
| `internal/verb/tree.go` | `Library.Contents` resolves through the half; `Tree.Archived` |
| `internal/verb/definition.go` | `restore` in `params` and in `guides`; the `archived` parameter on `restore`, `show`, `path`, `contents` |
| `internal/verb/checks.go` | the `restore` check list |
| `internal/mcp/tools.go` | the `restore` tool |
| `cmd/dinah/commands.go` | `restore` in the command table; `runRestore`; `req.Archived` in `runShow`, `runPath`, `runContents`; the `path workbench` shortcut guarded on the flag |
| `cmd/dinah/render.go` | the archived line on a `contents` listing |
| `cmd/dinah/compat_test.go` | `unwrittenEvents` empties |
| `cmd/dinah/references_guide_test.go` | `referenceProbeArgs` gains `restore` |
| `internal/bench/testdata/compat/populate.txt` | an archive and a restore below the sample card |
| `internal/bench/testdata/compat/<current revision>/` | recaptured, with the manifest digest re-blessed |
| `scripts/derive_event_counts.py` | the `unwrittenEvents` pattern tolerates an empty map |
| `internal/msg/locales/*.json` | the fifteen keys, in all eight catalogues |
| `internal/guide/guides/references.md` | the `restore` row and four statements of prose |
| `docs/design/format.md` | the event-schema rewrite, and the stray identifier at line 20 |

New code needs its coverage entries. `cmd/dinah/testdata/uncovered.txt` keys an allowlist by function and by rank within the function, and `cmd/dinah/row_sweep_test.go` holds a call-site registry the same way, so a new `runRestore` and a new render branch are added there deliberately rather than by shifting a neighbour's entry.

# 11. What this card does and does not reopen

Three rulings across this workstream turned on there being no restore, and a reader should not have to work out which of them this card moves. Two stand and one is re-argued.

dinah-456 section 3.3's rule that a writing command refuses a collection reference stands, on a re-stated reason. The rule's written reason is that an act writing to a set cannot be undone and Dinah has no restore. Restore removes the second half of that reason, and the rule survives on the first half, which is the load-bearing one. `dinah archive pb-1/comments` would be one act producing an unknown number of writes, and undoing it would take one `restore` per member with nothing recording how many there were. So the reason becomes that an act over a set is refused because a reader cannot see what it did, and a per-member inverse does not give them that. Nothing in dinah-455 changes, and `restore` joins the eleven refusing commands rather than the four accepting ones.

The refusal to offer Archive as a second action on a sidebar row stands, and this card does not reopen it. That ruling belongs to a surface this card does not touch, it was argued on the reversibility of a click rather than on the existence of a verb, and reopening it means settling what the second action's confirmation looks like on that surface. Whoever next works that surface may now argue it differently, and this paragraph is the note that its premise has changed. Filing a card here would be filing work nobody has scoped.

dinah-456 section 5.4's matrix stops under-reporting on one axis. Its Restore column reads `restore` for six kinds and "not applicable" for the workbench, which was written as the after-picture of this contract and becomes true when this card lands. No edit to that card is needed, because the row was already written forward.

# 12. Out of scope, and the two sibling questions

- **A grammar for naming an archived entity below an archived holder.** Section 3 states the limit and the route. Widening it means a per-step half, which is reference syntax and needs its own card and its own argument against dinah-456 section 8.
- **Repairing what a restore reveals.** A card whose column vanished, an over-capacity column and a stranded identifier are all reported by `dinah check` and repaired by the operator. `restore` puts the entity back and rewrites nothing else.
- **Archiving the workbench.** dinah-456 section 11 already records that refusal as deliberate, and `restore` inherits it rather than reopening it.
- **`search --archived`'s widening sense.** It stays exactly as it is, and section 3 states the sentence covering both senses rather than changing either.
- **Any change to `query`.** No field selection is added to either language.

The stated dependency on dinah-460 is a sequencing preference and not a technical one. dinah-456 section 9 says card 6 wants card 5's journalling in place, and this card's description said so until round 2 removed the sentence. Nothing in this contract reads or writes anything dinah-460 lands: `restore` journals one `restored` event through `bench.AppendEvent`, which `Library.Archive` already uses, and it touches no field write, no `FieldsOf`, and neither of the generic verbs. So this card can be implemented and merged before dinah-460, after it, or beside it. The one real interaction is the list merge below.

The collision with dinah-460. That card is being specified at the same time and adds `get` and `set`, which also join `params`, also point at the references guide, and also join the MCP roster. The overlaps are `internal/verb/definition.go`'s `params` and `guides` maps, `cmd/dinah/commands.go`'s command table, `internal/mcp/tools.go`'s `tools` slice, `cmd/dinah/references_guide_test.go`'s `referenceProbeArgs`, and the references guide's command table. Every one of those is a list, and neither card's entries exclude the other's. Neither card assumes it lands first. Whichever lands second resolves the merge by keeping both sets of entries and re-running the guide's derivation test, which fails on a missing row and on a surplus one, so a merge dropping either card's row is caught rather than shipped. dinah-460 also reshapes `toolExemptions` into a struct of two fields, and this card does not touch that map, so its entry set is the same either way.

# 13. What round 2 changed

Agent Design Review pushed this card back with two blocking findings, two majors, five minors and a nit. Both blockers were an acceptance criterion certifying behaviour the mechanism does not produce, so nothing downstream would have caught either. This section records what moved, so a third reader does not have to diff two versions of a spec to find out.

The first blocker was that section 4.1's raise rule could not produce the refusal section 3 and the nested-archive criterion both promised. The rule raised `dinah.not-archived` only when the reference resolved in the live half, and `pb-1/comments/1` with `pb-1` archived resolves in neither half, so the mechanism returned `unknown-card` at the one reference the spec singles out as the flag's chosen limit. The promise is kept and the mechanism moved: section 4.1 now runs a diagnostic walk on the refusal path, reading live first and the mirror second at every step, whose whole output is a boolean and a string. The answer path is untouched and still looks in exactly one place, which is what makes section 3's rule determinate.

The second blocker was that the minted refusal's English was written for `restore` and raised by three read commands and by the workbench head. Its advice told the reader to run `dinah archive {detail}`, and `dinah archive workbench` is refused by design at `internal/verb/beyond.go:238`, so one branch advised an act the tool will not perform. Section 4.1 now uses `Variants` and `WhenCommand`, the mechanism section 4.2 already uses for `dinah.exists`, giving `restore` its own base sentence and giving the workbench, the archived holder and the collection member their own next steps. Section 4.1 carries the table of every command against every case with the answer to whether its advice can be carried out, because walking that table by hand is the test the workbench's convention counterexamples prescribe for a refusal reused across shapes.

The two majors were an argument missing from a decision note and a guard nobody had named. The decision on the deepest-collection-step rule weighed a per-step half and a live-then-archive fallback and did not weigh trying the mirror first at every step, which survives both of its tests; that alternative and the ground it loses on are now in the note. Section 7 gained a subsection naming `TestContractTokensSurviveInBackticks`, whose subject set this card widens without editing it, and saying which of the new backticked spans its selector reaches and which it does not.

The five minors and the nit. Section 3.1 says the deepest-step test is decided from the segment count before the collection path is joined, because deciding it from `tail` and `below` decides after the join. Section 4.4 says the now-dead `act.Op != OpRestore` inside the column block is dropped. Section 10's `read.go` row names `CollectionListing.Archived`. The description no longer claims a dependency on dinah-460 that section 12 retracts. Thirty-four line-initial bolded captions are unbolded with no word changed, and five pure glosses of the "which is ..." shape are rewritten as plain clauses; the remaining clauses of that shape each carry a citation or a fact an implementer needs, and the prose standard's hard constraint keeps them.

The nit was that every decision on this card gates Agent Design Review, which was the column the card stood in when the review read it. That gate is what the Spec column's own instructions prescribe: a decision taken here is settled at Spec, so it gates the column after Spec, which is Agent Design Review. The gates are left where they are, and a decision records the reason so the next card copies the prescribed pattern rather than avoiding it.

# 14. What round 3 changed

Agent Design Review pushed this card back a second time with one blocking finding and two smaller ones. Both of round 2's blockers were accepted as answered and neither was reopened.

The blocker was round 1's first finding one layer further out. Section 4.1 said `dinah.not-archived` was raised in exactly one place, `Bench.ResolveReferenceIn`, and that all four commands therefore answered alike. Two of the four never read that function's error. `Library.Show` discards it at `internal/verb/read.go:816` with a comment at `:809` saying why, and `ResolvePath` never calls `ResolveReference` at all, so `dinah show --archived` on a live card and `dinah path --archived workbench` both answered as though nothing had been asked. Section 4.1 now carries the traced call chain of every command from its entry point to the function that fails, section 3.2 carries the same for `Library.Show`'s three branches, and the refusal moved into one declared function, `Bench.notArchivedFor`, called from both archived-half entry points rather than from one.

What was traced, and how. Every claim this spec makes about where something happens was checked by reading the call chain in the source rather than the paragraph beside it. The chains read were `runRestore` through `Library.Restore` and `Bench.ResolveEntityIn`; `runShow` through all three branches of `Library.Show`; `runPath` through its pre-bench shortcut at `cmd/dinah/commands.go:1217` and through `Bench.ResolvePath`, `resolveBelow`, `resolveBelowLanding` and `descend`; `runContents` through `Library.Contents`; and `Bench.ResolveReference` and `Bench.ResolveEntity` from their own tops. The enumerating read was `grep -rn 'ResolvePath(\|ResolveReference(\|ResolveEntity(\|ResolveCard(\|ResolveArchivedCard('` over `internal/verb` and `cmd/dinah` with tests excluded, which returns twenty-seven call sites, and each of the four commands' own sites was opened. That read is also what found `dinah edit` sharing the shape of two of them and taking no `--archived`, which is why section 3.2 now says `edit` is untouched.

The two smaller findings. The `NotArchived` shape's value naming the workbench was called `workbench` and filled with `Bench.Slug`, which is `contract.ValueWorkbench`'s reserved name for the workbench directory; it is now `slug`, and section 4.1 says what the collision would have cost. The decision on the minted refusal still described the live-retry mechanism round 2 replaced and still claimed a single raise site in `ResolveReferenceIn`; its text and its note are both rewritten.

Three minors from round 2's list. The three `internal/bench/resolve.go` line citations in section 3.1 were each one low and now read 519, 520 and 539. The implied `ResolveCardIn` and `ColumnByRefIn` in section 3.2, which the spec never declared, are gone: Show's archived bare head goes through `ResolveReferenceIn`, and the head's own half is decided by one declared `headHalf`. Round 2's own accounting, in the second paragraph above section 1, said four criteria moved with it. It had also edited AC-9, whose second arm carries the guard on `runPath`'s pre-bench shortcut, so the count should have been five and AC-9 should not have been among the seven reported unaffected. That paragraph now says five.

What moved on the checklist. AC-2 now drives its three cases through all four commands rather than through `restore` alone, because the defect this round found is a command that never reaches the refusal and a criterion exercising one command cannot see it. D-3's text and note are rewritten. AC-2's and AC-11's reddening notes name `Bench.notArchivedFor` where they named `Bench.ResolveReferenceIn`. Nothing else on the checklist changed, and no criterion was added or removed.

## Branch

dinah-461-restore-is-the-inverse-of-archive-and-an-archived-entity-has-an-address-again
