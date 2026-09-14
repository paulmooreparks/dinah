---
title: edit opens a workstream's directory in your editor instead of its anchor
column: b69abf918c42
state: ready
severity: minor
priority: soon
workstreams:
  - 994787601ae6
---
`dinah edit` hands a workstream's directory to the reader's text editor. A workstream is not a file, so what opens is whatever that editor does with a directory, which is not what anybody asked for and is not a thing `edit` means to offer.

This is the same shape dinah-455 found and fixed for a collection reference: `edit` had been handing a collection directory to the editor too, with no check in between. That card refuses a collection now. The workstream case was left out of it deliberately, because refusing it needs a refusal name that card does not mint, plus its English and its entries across the eight message catalogues under the translation staleness rules. The implementer argued it is a card rather than a nit and the reviewer agreed.

What the card should settle, none of it the operator's to rule on:

Whether `edit` refuses a workstream, or reaches something inside it. A workstream has notes, a journal and attachments in its directory, so "edit the workstream" is not meaningless the way "edit a collection" is; it is ambiguous. If one of those is the obvious target, say which and why, and if none is, refuse and say what to type instead. The parent contract on dinah-456 settled that a refusal names what the reference does name and offers something real to type, so follow that shape rather than refusing bare.

Which refusal name it uses. dinah-456 minted two names rather than widening an existing one, with the reason recorded: the existing name means the workbench answers to nothing by that spelling, and a reference that resolves to something real must not be told it does not exist. Check whether either of those two fits this case before minting a third.

Whether anything else `edit` accepts has the same problem. The collection case and this one were both found by looking at one command's behaviour rather than by a sweep, which is how the second survived the first. Establish what `edit` does for every reference shape the grammar admits, and say what you found, including the shapes that are fine.

Filed from dinah-455. Related: dinah-456 is the addressing contract, and dinah-457 is the references guide, which will describe whatever this card decides.

## Specification

Worked against `origin/main` at 808d105c21516d59a9d73e7d8d8f7ae369c37f6c ("dinah-461: restore is the inverse of archive, and an archived entity has an address again"), which carries all seven of dinah-455, dinah-456 with its five children, and dinah-461. Every observation below was produced by running a binary built from that commit against a throwaway workbench, with `DINAH_HOME` pointed inside the card's own scratch directory and `DINAH_EDITOR` pointed at a shim that logs its arguments and opens nothing.

Round 1 ran in `C:\dinah-scratch\dinah-467-spec`. Round 2 rebuilt from the same commit in `C:\dinah-scratch\dinah-467-spec2\wt` and re-ran every claim it changed, plus the degenerate references section 3 now enumerates; `origin/main` was still at 808d105 when round 2 started, so the two rounds read the same tree.

## 1. The defect, reproduced

`dinah edit workstream/<slug>` exits 0 and hands the workstream's directory to the editor:

    $ dinah edit workstream/addressing
    $ cat editor.log
    ARGS: ...\.dinah\<bench>\workstreams\be6d4b335088

That path is a directory. An archived workstream behaves the same way, handing over `...\archive\workstreams\<id>`. The identifier spelling `workstream/<id>` does too.

The route is `runEdit` in `cmd/dinah/commands.go:1258`, which resolves through `Bench.ResolvePath`. That resolver's workstream arm, `resolvePathBody` at `internal/bench/resolve.go:211`, returns `workstream.Dir` where every other arm returns an anchor file.

## 2. The sweep

The card asks what `edit` does for every reference shape the grammar admits, because the collection case and this one were each found by looking at one shape. Twenty-one shapes were run through `dinah edit`, chosen to cover every kind in the containment table (`internal/bench/containment.go`), both spellings of the workbench, the card's own two file segments, all four collection mounts, the three checklist words, the attachment payload, both halves of the workstream grammar, and the two collections whose member kind is addressed in its own right.

Twelve shapes opened something, and eleven of the twelve opened a regular file:

| reference | what reached the editor |
|---|---|
| `workbench` | `<root>/workbench.md` |
| `.` | `<root>/workbench.md` |
| `doing` (a column) | `columns/<id>/column.md` |
| `wb-1` | `cards/<id>/card.md` |
| `wb-1/card` | `cards/<id>/card.md` |
| `wb-1/journal` | `cards/<id>/journal.ndjson` |
| `wb-1/comments/1` | `comments/<id>/comment.md` |
| `wb-1/checklist/1` | `checklist/<id>/item.md` |
| `wb-1/questions/1` | `checklist/<id>/item.md` |
| `wb-1/attachments/1` | `attachments/<id>/attachment.md` |
| `wb-1/attachments/1/payload` | `attachments/<id>/payload/<filename>` |
| `workstream/addressing` | `workstreams/<id>`, which is a directory |

Nine shapes refused, and each refusal is the right one:

| reference | refusal |
|---|---|
| `wb/attachments` | `dinah.is-a-collection` |
| `wb-1/comments` | `dinah.is-a-collection` |
| `wb-1/checklist` | `dinah.is-a-collection` |
| `wb-1/questions` | `dinah.is-a-collection` |
| `wb-1/attachments` | `dinah.is-a-collection` |
| `doing/attachments` | `dinah.is-a-collection` |
| `wb-1/comments/1/attachments` | `dinah.is-a-collection` |
| `wb/cards` | `dinah.unknown-path`, carrying `addressed: card` |
| `wb/columns` | `dinah.unknown-path`, carrying `addressed: column` |

Nothing below a workstream is addressable at all. `workstream/addressing/journal`, `workstream/addressing/notes` and `workstream/addressing/attachments` each refuse `dinah.unknown-workstream`, because `workstreamByRefIn` reads everything after the prefix as one slug or identifier.

So the workstream is the only shape in the grammar that hands `edit` a directory, and the sweep found no third instance of the defect.

### Why the existing guard did not catch it

`TestEveryAddressTheContentsTreeDrawsResolvesThroughTheCommandsThatDeclareIt`, in `cmd/dinah/references_command_resolution_test.go`, already runs `show`, `path`, `edit` and `attachments` over addresses read off the guide's own table. It misses this defect three times over, and each miss is worth naming because the guard in section 4.5 closes all three.

1. It walks the tree `dinah contents workbench --depth all` draws. A workstream is deliberately absent from the containment table, so no node of that walk is ever a workstream and no workstream reference is tested.
2. Its verdict for `edit` is `resolutionRefused(got.errw)`, which reads only whether stderr leads with `dinah.unknown-card` or `dinah.unknown-path`. A run that opened the wrong thing and a run that opened the right thing are the same observation to it.
3. It points `DINAH_EDITOR` at `dinah-no-such-editor`, so nothing observes the argument the editor was given. A directory and a file fail the launch identically.

### What a workstream's directory actually holds

The card's framing says a workstream has notes, a journal and attachments in its directory. The first two are there and the third is not:

    $ dinah path workstream/addressing
    ...\workstreams\be6d4b335088
    $ ls
    journal.ndjson  workstream.md
    $ dinah attach workstream/addressing notes.md
    dinah.not-attachable workstream/addressing is workstream, and Dinah keeps no
    attachments on workstream; attachments hang from the workbench, a column, a
    card, or a comment
    $ dinah contents workstream/addressing
     (workstream/addressing) contains nothing.

`workstream.md` is an ordinary anchor. It carries `title`, `slug`, `status` and `ordinal` in its header and the workstream's `notes` field as its body, which is the field `dinah set workstream/<slug> notes` writes and `dinah get` reads. The same false claim about attachments sits in the code, in `ResolvePath`'s doc comment at `internal/bench/resolve.go:174`, where it is the stated reason a workstream resolves to its directory. Section 5 requires that sentence to be corrected.

## 3. What `edit` does about it

**`edit` opens the workstream's anchor, `workstream.md`. It refuses nothing new and mints no refusal name.**

This is not the collection case. A collection has no anchor and no identifier, so there is no file behind it and dinah-455 was right to refuse one. A workstream is one entity of the format with one anchor, exactly as a card, a column and the workbench are. Three things make the anchor the obvious target rather than an arbitrary pick among the directory's contents.

- Every other kind `edit` accepts opens its own anchor, and the anchor is where that kind's editable prose lives. A card's body is in `card.md`, a column's instructions are in `column.md`, and a workstream's notes are in `workstream.md`. Opening the anchor makes the workstream ordinary rather than special.
- The journal is not a competing candidate. It is machine-written `ndjson` that nothing edits by hand, and where a reader does want a card's journal they ask for it by name with `<card>/journal`. No such spelling exists below a workstream, so no reference in the grammar is being taken away from anybody.
- The attachments the ambiguity argument rests on do not exist, as section 2 shows.

Refusing instead would cost more than it buys. `dinah edit workstream/<slug>` would become the only route to a workstream's own prose that the tool answers with a refusal while `path`, `get` and `set` all answer it, and the references guide's workstream sentence would have to drop `edit`, which is a live claim held against behaviour by `TestTheReferencesGuideNamesTheCommandsThatTakeAWorkstream`.

### Which refusal name it uses

None, because the fix opens a file rather than refusing. The two names dinah-456 minted and the one dinah-461 minted were each read against this case and each would be wrong here for the reason dinah-456 recorded: a reference that resolves to something real must not be told that nothing answers to it, and `workstream/<slug>` resolves to a real entity carrying a real file.

This card therefore adds no reader-facing string, touches no message catalogue, and raises no translation-staleness work. That is the opposite of what the card's framing anticipated, and the reason is that the framing expected a refusal. Nothing new is said to a reader because nothing new happens to them: the command now opens the same kind of thing for a workstream that it already opens for every other kind.

### The empty reference, and every other degenerate input

This subsection is round 2's central correction. Round 1 specified `ResolveEditTarget` to ask `ResolveReference` first and to answer whatever entity came back with that entity's anchor. `ResolveReference` answers the EMPTY reference with the workbench, so `dinah edit` typed with no argument at all would have stopped refusing and started opening `workbench.md`. Nobody decided that, and AC-5 requires the empty reference to keep refusing, so round 1's design and round 1's own criteria contradicted each other.

**The criterion is kept and the design is corrected.** `edit` goes on refusing the empty reference. Three things back that direction and none backs the other.

- `IsWorkbenchRef`'s own doc comment at `internal/bench/bench.go:2001` already declares the rule, and it names `edit` while declaring it: "The empty reference is not one of them. ResolveEntity treats it as the workbench because attach and archive take the workbench as a default subject; ResolvePath keeps refusing it, because path and edit both declare the argument required and a bare `dinah path` is somebody who forgot it." Opening the workbench would falsify a comment that names this command, which is the class of defect section 5 exists to clear rather than to add to.
- `internal/verb/definition.go:565` declares `edit`'s one parameter with `Required: true`. Nothing enforces that at parse time, since `runEdit` reads `at(parsed.rest(), 0)` and gets the empty string from an absent argument, so the resolver's refusal IS the enforcement of the declared requirement. Removing the refusal would leave the declaration unenforced everywhere.
- D-3 refused to move `path`'s settled answer in order to fix `edit`. Letting a bare `dinah edit` succeed while a bare `dinah path` refuses splits the two commands the other way round for no reason anybody asked for.

Round 1's miss was not an oversight about one input. It is the third time this week that a claim about what a resolver returns held for the case being thought about and failed for a case nobody was thinking about, and each time somebody running the degenerate input found it rather than somebody reading the function. So the answer here is the whole degenerate set, enumerated and pinned rather than the one instance repaired.

Every reference below was run against `dinah edit`, `dinah path` and `dinah get <ref> title` on the round-2 binary. `get` resolves through `ResolveEntity`, which is `ResolveReference` refusing a collection, so its column is what `ResolveReference` answers. The last column is what `ResolveEditTarget` must answer once section 4.3 is built.

| reference as typed | `ResolveReference` (via `get`) | `ResolvePath` (via `path`) | `edit` at 808d105 | required after this card |
|---|---|---|---|---|
| absent argument | the workbench | refuses `unknown-card` | refuses `unknown-card` | **refuses `unknown-card`** |
| `""` | the workbench | refuses `unknown-card` | refuses `unknown-card` | **refuses `unknown-card`** |
| `"   "` (spaces) | the workbench | refuses `unknown-card` | refuses `unknown-card` | **refuses `unknown-card`** |
| `"\t"` (a tab) | the workbench | refuses `unknown-card` | refuses `unknown-card` | **refuses `unknown-card`** |
| `.` | the workbench | `workbench.md` | opens `workbench.md` | opens `workbench.md` |
| `workbench` | the workbench | `workbench.md` | opens `workbench.md` | opens `workbench.md` |
| `/` | refuses `unknown-card` | refuses `unknown-card` | refuses `unknown-card` | refuses `unknown-card` |
| `//` | refuses `unknown-card` | refuses `unknown-card` | refuses `unknown-card` | refuses `unknown-card` |
| `..` | refuses `unknown-card` | refuses `unknown-card` | refuses `unknown-card` | refuses `unknown-card` |
| `wb-1/` | the card | `card.md` | opens `card.md` | opens `card.md` |
| `wb-1/card/` | the card | `card.md` | opens `card.md` | opens `card.md` |
| `wb-1//card` | refuses `dinah.unknown-path` | refuses `dinah.unknown-path` | refuses `dinah.unknown-path` | refuses `dinah.unknown-path` |
| `workbench/` | refuses `unknown-card` | `workbench.md` | opens `workbench.md` | opens `workbench.md` |
| `doing/` (a column) | refuses `unknown-card` | `column.md` | opens `column.md` | opens `column.md` |
| `workstream/` | refuses `dinah.unknown-workstream` | refuses `dinah.unknown-workstream` | refuses `dinah.unknown-workstream` | refuses `dinah.unknown-workstream` |
| `workstream//` | refuses `dinah.unknown-workstream` | refuses `dinah.unknown-workstream` | refuses `dinah.unknown-workstream` | refuses `dinah.unknown-workstream` |
| `workstream` (bare) | refuses `unknown-card` | refuses `unknown-card` | refuses `unknown-card` | refuses `unknown-card` |
| `workstream/addressing/` | refuses `dinah.unknown-workstream` | refuses `dinah.unknown-workstream` | refuses `dinah.unknown-workstream` | refuses `dinah.unknown-workstream` |
| `workstream/workstream/addressing` | refuses `dinah.unknown-workstream` | refuses `dinah.unknown-workstream` | refuses `dinah.unknown-workstream` | refuses `dinah.unknown-workstream` |
| ` wb-1 ` (spaced) | the card | `card.md` | opens `card.md` | opens `card.md` |

`unknown-card` is printed without the `dinah.` namespace prefix and `dinah.unknown-path` and `dinah.unknown-workstream` are printed with it. That asymmetry is what the binary does at 808d105, it is what AC-5 already records, and this card does not touch it.

Four things in that table are the load-bearing ones.

1. **The four whitespace rows are the only divergence in the accepting direction.** They are the only references in the whole set where `ResolveReference` succeeds and `ResolvePath` refuses. Everywhere else, a reference the entity resolver answers is a reference `ResolvePath` answers with the same file, the one exception being the workstream, which is this card's whole subject. So a design asking the entity resolver first is safe for every reference except the empty one, and section 4.3 arm 1 is the guard that keeps it that way.
2. **A trailing slash makes the entity resolver miss where `ResolvePath` hits.** `workbench/` and `doing/` both refuse `unknown-card` through `ResolveReference` and both resolve through `ResolvePath`, because `strings.Cut` reduces them to a bare head that is neither a column reference nor a card. Both reach section 4.3's fallback arm and go on opening what they open today.
3. **The workstream prefix swallows everything after it.** `workstream/`, `workstream//`, `workstream/addressing/` and the doubled prefix all refuse `dinah.unknown-workstream` in both resolvers, so all four reach the fallback arm and keep their refusal spelled exactly as it is spelled today.
4. **Trimming happens inside both resolvers, not at the command.** ` wb-1 ` opens `card.md`, and the whitespace-only references are refused because they trim to empty rather than because the command inspected them. Section 4.3 arm 1 therefore trims before it asks, so a tab-only argument takes the same route as an absent one.

### Where the fix goes, and why not in `ResolvePath`

`ResolvePath` keeps answering the workstream's directory. It has two callers, `dinah path` at `cmd/dinah/commands.go:1227` and the library's own `path` verb at `internal/verb/read.go:863`, and its job is to hand out the filesystem address a shell consumes. dinah-456 settled that answer deliberately. Changing it would move a documented, settled behaviour of a second command in order to fix a first, so `edit` gains its own statement of what it opens instead.

## 4. What to build

### 4.1 `bench.AnchorPathOf`

A new exported function in `internal/bench/entity.go`, beside `EntityRef`:

```go
// AnchorPathOf is the path of the file that IS an entity: the entity's own
// directory joined with the anchor filename its kind declares. The second
// answer reports whether the kind declares one at all, which is false only
// for a kind outside the grammar. It is reported rather than swallowed
// because AnchorOf answers such a kind with the empty string, and a caller
// joining that gets the entity's directory back, which is a directory where
// it asked for a file. That is the defect dinah-467 fixed for the one kind
// that had it.
func AnchorPathOf(entity *EntityRef) (string, bool)
```

`internal/verb/fields.go` joins `entity.Dir` to `bench.AnchorOf(entity.Kind)` by hand at lines 222 and 271. Both call sites move to `AnchorPathOf`, and on a false second answer each returns the `contract.UnknownPath` refusal over `entity.Ref` that it already returns when the anchor cannot be read. Behaviour there does not change, since reading a directory as text fails today anyway, and the join stops existing in three places.

### 4.2 `bench.AddressedInItsOwnRight`

`addressedInItsOwnRight` at `internal/bench/resolve.go:704` is renamed to `AddressedInItsOwnRight` and exported, with its doc comment kept. Its one caller inside `descend` follows the rename. The sweep in 4.5 needs the predicate in order to know which collection shapes refuse `dinah.unknown-path` rather than `dinah.is-a-collection`, and a test writing out `card` and `column` for itself would be a second copy of a rule the resolver already declares.

### 4.3 `Bench.ResolveEditTarget`

A new method in `internal/bench/resolve.go`, next to `ResolvePath` and `ResolveReference`:

```go
// ResolveEditTarget is the file `edit` opens for a reference, and it is the
// whole of that command's reference policy in one place. An entity is
// answered with its anchor, which is what makes edit open a workstream's
// workstream.md rather than the directory holding it. A whole collection is
// refused with the refusal CollectionRef.Refuse composes. Everything else
// falls through to ResolvePath, which answers exactly two further
// references, both of them files: a card's journal and an attachment's
// payload.
//
// The empty reference is answered by ResolvePath rather than by the entity
// resolver, and that ordering is the whole of arm 1. ResolveReference reads
// an empty reference as the workbench, so asking it first would make a bare
// `dinah edit` open workbench.md; IsWorkbenchRef's own comment declares that
// edit refuses it, because edit declares its argument required and somebody
// who typed no argument forgot it rather than meaning the workbench.
func (b *Bench) ResolveEditTarget(ref string) (string, error)
```

The body has five arms, and the first is new in round 2.

1. Where `strings.TrimSpace(ref)` is empty, return `b.ResolvePath(ref)` and nothing else. The refusal is delegated rather than restated, so the sentence a reader gets for a bare `dinah edit` keeps being raised by the code that raises it today and cannot drift from the one `dinah path` raises. The trim is what makes a tab-only or space-only argument take this arm too, per section 3's fourth point.
2. Call `b.ResolveReference(ref)`.
3. Where it answers a collection, return `collection.Refuse()`. This is the check `runEdit` performs inline today, moved rather than changed.
4. Where it answers an entity, ask `AnchorPathOf`. On a true second answer, return `filepath.Abs` of that path, so the answer is absolute in the way `ResolvePathIn` already makes its own answer absolute. On a false second answer, return an ordinary Go error reading `the kind %q declares no anchor, so %q names nothing to open`, built with `fmt.Errorf` rather than with `contract.Refuse`. That case is a defect in this build rather than a mistake by the reader, `reportError` at `cmd/dinah/main.go:375` prints a non-refusal error as `unreachable` and exits 4, and `ResolvePathIn` already lets `filepath.Abs`'s own error out by that route. Section 4.6 arms the arm.
5. Where `ResolveReference` returns an error, discard that error and return `b.ResolvePath(ref)` with whatever it answers. This is what `runEdit` does today, and it is what keeps every refusal in section 2's second table and every refusal in section 3's table spelled exactly as it is spelled today. `workstream/nosuch` refuses `dinah.unknown-workstream` through `ResolvePath` and must go on doing so rather than becoming `dinah.unknown-card`.

The function does not stat the file it names. No arm of `edit` does today, and a workbench damaged past having an anchor is `dinah check`'s business.

### 4.4 `runEdit`

`runEdit` at `cmd/dinah/commands.go:1258` loses the inline collection check and the `ResolvePath` call, and calls `l.Bench.ResolveEditTarget(ref)` in their place. Everything from `bench.ResolveEditor` onward is untouched. The comment explaining the collection check travels to `ResolveEditTarget` rather than being deleted.

### 4.5 The sweep guard

A new file `cmd/dinah/edit_reference_sweep_test.go` holds one generator and two tests. It lives in `cmd/dinah` because that package already imports `internal/bench` and already carries `runCLI`, `newBench`, `benchDir` and `addCard`.

`editShape` is one generated reference and the verdict declared for it:

```go
type editShape struct {
    ref     string // the reference as a reader would type it
    kind    string // the containment kind it names, empty for a shape naming no entity
    opens   bool   // whether edit is required to open a file for it
    refusal string // the refusal name required when opens is false
}
```

`editReferenceShapes(t *testing.T, root string) []editShape` generates the shapes rather than listing them. It emits three groups, and the group boundaries are what make the counts in the plants of section 8 derivable rather than guessed.

**Group A, derived from the containment table.** It reads `bench.Contains` recursively from `bench.KindWorkbench`, emitting for each mount the collection reference and, where the fixture put a member there, the member reference and everything the member's own kind mounts in turn. A mount whose kind `bench.AddressedInItsOwnRight` reports true emits its collection shape with `refusal: contract.UnknownPath`; every other collection shape carries `contract.IsACollection`.

**Group B, the shapes the grammar declares outside the containment table**, each with a comment naming where the resolver declares it: `workbench` and `.` (`bench.IsWorkbenchRef`), `<slug>/attachments/1` (the bare-slug head `resolveBelowLanding` admits), `<card>/card` and `<card>/journal` (`bench.cardOwnFileSegment`), each of the three `Word` spellings of `checklistSegments` applied to a card both as a collection and as a member, `<card>/attachments/1/payload` (`bench.PayloadDir`), and `workstream/<slug>` and `workstream/<id>` (`bench.WorkstreamRefPrefix`). The three `Short` spellings (`oq`, `ac`, `d`) are deliberately not emitted: they resolve the same three collections the `Word` spellings do, so they add a shape without adding reach, and `internal/bench` holds the two spellings together already.

**Group C, the degenerate references of section 3's table**, written out with the verdict that table's last column declares. Every row of that table that is not already a Group A or Group B shape belongs here, which is the absent argument written as `""`, `"   "`, `"\t"`, `/`, `//`, `..`, `wb-1/`, `wb-1/card/`, `wb-1//card`, `workbench/`, `<column>/`, `workstream/`, `workstream//`, `workstream` bare, `workstream/<slug>/`, `workstream/workstream/<slug>` and ` wb-1 `. This group is the answer to the shape rather than to the instance: the empty reference is guarded here as one member of a class, so the next degenerate input somebody invents is caught by a table a reader can read rather than by a case somebody happened to think of.

A new mount in the containment table therefore produces new Group A shapes with no edit to this file, which is the property that would have caught this card's defect had a workstream been mounted.

The fixture the generator runs against holds one column carrying an attachment, one card carrying an attachment, a comment carrying an attachment of its own, one checklist item of each of the three kinds, an attachment on the workbench, one live workstream and one archived workstream.

`TestEveryReferenceShapeEditAcceptsNamesAFile` opens the fixture with `bench.Open(benchDir(t, root))`, calls `ResolveEditTarget` on every generated shape, and for each one:

- where `opens` is true, requires no error, and requires `os.Stat` of the answer to report a regular file through `Mode().IsRegular()`. A directory fails, and so does a path nothing stands at.
- where `opens` is false, requires an error that is a `*contract.Refusal` whose `Name` equals the shape's declared `refusal`.

It then asserts three declared constants against what it swept: `wantEditShapes`, `wantEditOpens` and `wantEditRefusals`, all three greater than zero, and the last two summing to the first. The numbers are written into the file as constants with a comment naming what the fixture holds, and none of them is computed from anything under test. It also asserts by name rather than by count that the swept set carries at least one shape for each of `bench.KindWorkbench`, `KindColumn`, `KindCard`, `KindComment`, `KindItem`, `KindAttachment` and `KindWorkstream`, so a fixture that quietly stopped creating one cannot leave the sweep proving nothing about that kind. It separately asserts that Group C is non-empty and that the shape whose `ref` is the empty string is present and declared refusing, because a group that silently stopped generating is the failure mode this card is repairing.

`TestEditHandsTheEditorTheFileTheResolverNames` runs the command rather than the resolver. For each generated shape it points `DINAH_EDITOR` at the test binary itself and runs `runCLI(t, root, "edit", shape.ref)`, then reads back what the launched process was given. A shape whose `opens` is true must have recorded exactly one argument, and that argument must equal what `ResolveEditTarget` answers for the same reference. A shape whose `opens` is false must have recorded nothing, and `runCLI` must have exited with the refused code. The absent-argument case is run separately as `runCLI(t, root, "edit")` with no reference at all, since a shape carries a reference by construction and the argument nobody typed is the one this round's defect turned on.

The recording works by re-execution and needs five lines in `cmd/dinah/main_test.go`. A new constant `editorRecordVar = "DINAH_TEST_EDITOR_LOG"` names an environment variable holding a file path, and `TestMain` reads it as its very first act: where it is set, the process appends `strings.Join(os.Args[1:], " ")` to that file and calls `os.Exit(0)` without running `testenv.IsolateTempDir`, `testenv.ClearVars` or `m.Run`. The test sets that variable to a fresh file per shape with `t.Setenv` and points `DINAH_EDITOR` at `os.Args[0]`.

Three things about the mechanism are deliberate. The child exits before `m.Run`, so it never runs the suite and never trips the unreached-table-site sweep `TestMain` performs after `m.Run`. The editor is a real executable rather than a script, because Windows `CreateProcess` is documented as requiring the command interpreter for a batch file, and a guard resting on a `.cmd` running directly would rest on behaviour Microsoft does not document. And `DINAH_TEST_EDITOR_LOG` does not join `isolatedEnv`, because that list names variables production code reads, this one is read only by `TestMain`, and clearing it would break the mechanism it exists for. Record that reason in the comment beside the list, which is what the workbench requires of a card that touches the environment either way.

### 4.6 The kind-with-no-anchor guard

A new file `internal/bench/edit_target_test.go` carries `TestAnchorPathOfReportsNoAnchorForAKindOutsideTheGrammar`. It calls `AnchorPathOf(&EntityRef{Kind: "frobnicate", Dir: "/somewhere"})` and requires the second answer to be false and the first to be empty, which is the arm section 4.3 step 4 depends on. It also calls `AnchorPathOf` for each of the seven kinds `ResolveReference` can answer and requires a true second answer for every one, which is what makes step 4 unreachable today. `internal/bench/fields_test.go:143` already holds `AnchorOf` to the same set, and this test holds the join beside it.

## 5. The false sentence in `ResolvePath`'s comment

`internal/bench/resolve.go:169-178` currently reads:

```go
// ResolvePath resolves a reference to an absolute path: the workbench itself,
// a column, a workstream, a card, or anything below any of the first three
// composed by path. It is what the plumbing guarantee of `path` rests on,
// what `edit` walks, and what `show` walks for the composed form.
//
// A workstream resolves to its directory rather than to its anchor, because
// its notes, its journal and its attachments all sit inside it and the
// reference names the entity rather than one file of it. A workstream is
// tried first, per WorkstreamRefPrefix and the reasoning on resolveWorkstreamRef,
// before the rest of the grammar gets a chance to shadow it.
```

Two clauses of that go wrong once this card lands, and round 1 named only one of them and described its repair rather than writing it. Both are written out here.

The attachments clause states a fact about the format that is not true, as section 2 shows, and it is the sentence a reader would reach for to justify the defect this card fixes. Deleting the two words is not enough on its own: what is left says the notes sit inside the directory, and the notes are a field of `workstream.md` rather than a file beside it, so the shortened sentence is a fresh false claim in place of the old one.

The `what edit walks` clause narrows too. After section 4.4, `edit` walks `ResolveEditTarget`, which reaches `ResolvePath` only through arms 1 and 5.

**Replace the whole block above with exactly this:**

```go
// ResolvePath resolves a reference to an absolute path: the workbench itself,
// a column, a workstream, a card, or anything below any of the first three
// composed by path. It is what the plumbing guarantee of `path` rests on,
// what `show` walks for the composed form, and what ResolveEditTarget falls
// back to for the references the entity resolver does not answer.
//
// A workstream resolves to its directory rather than to its anchor, because
// the reference names the entity and `path` hands a shell a filesystem
// address to work from. The directory holds the anchor workstream.md, which
// carries the workstream's notes, and the machine-written journal.ndjson.
// ResolveEditTarget answers that anchor rather than this directory, because
// an editor is handed a file. A workstream is tried first, per
// WorkstreamRefPrefix and the reasoning on resolveWorkstreamRef, before the
// rest of the grammar gets a chance to shadow it.
```

The `show` clause is carried across from the current text unchanged and is not a claim this card establishes. The replacement names no attachment, so AC-8's sweep for the workstream-and-attachment adjacency stays as narrow as AC-8 declares it rather than having to exempt the comment this card just wrote.

## 6. What does not change

- **The references guide.** `internal/guide/guides/references.md` keeps every cell of its "Which command takes what" table and keeps its workstream sentence naming `edit`. This card moves nothing `edit` accepts or refuses, so the derived checks reading that table and that sentence, `TestEveryAddressTheContentsTreeDrawsResolvesThroughTheCommandsThatDeclareIt` and `TestTheReferencesGuideNamesTheCommandsThatTakeAWorkstream`, stay green without being edited. Had the card refused a workstream instead, the second of those would have reddened and the sentence would have needed an edit, which is one more reason the anchor is the right answer.
- **The eight message catalogues.** No string is added, changed or removed, so no `source` fingerprint moves, no skeleton catalogue needs refreshing, and no locale decision record is owed. The rule that no word may agree with a placeholder holding an untranslated token is not engaged, because this card interpolates nothing.
- **`dinah path`.** It goes on printing the workstream's directory, per section 3.
- **The MCP head.** `edit` is exempt from it at `internal/mcp/tools.go:150` on the ground that it needs a terminal, so no tool schema moves.
- **Every refusal in section 3's table.** The four whitespace rows keep `unknown-card` through arm 1, and every other refusing row keeps its refusal through arm 5.

## 7. Out of scope, and the three residues recorded here

Round 1 recorded one finding here, about `edit` and `path` under-declaring what they take. Round 2's reviewer ruled on it and the ruling split it three ways. What follows is the split, so that a future reader finds each piece where it now lives rather than following round 1's version into work that has moved.

**The parameter sentences are dinah-470's, and its answer changes what this card should say about them.** `param.edit.card.summary` and `param.path.card.summary` both read "this workbench written as `workbench` or `.`, a card, or something below a card such as wb-1/comments/1", omitting a column and a workstream. dinah-470's spec, read on 2026-09-10 while it sat in Spec on its own second round, rewrites both: `param.edit.card.summary` becomes "the thing you are opening in your editor, such as wb-1/comments/1", `param.path.card.summary` becomes "the thing whose file path you are printing, such as wb-1/comments/1", and the kinds move out of prose into a rendered clause driven by a new `referenceKinds` declaration in `internal/verb`. So round 1's proposed repair, which was to copy the wording of `param.contents.ref.summary`, is obsolete and must not be handed to anyone: the sentence it would have been copied into is being deleted.

**dinah-470 does not close the workstream half, and that gap is structural.** Its declaration knows five kinds, `workbench`, `column`, `card`, `below-card` and `collection`, and workstream is not among them, because the guide's table that the declaration reproduces deliberately keeps workstreams out. So after dinah-470 lands, `dinah help edit` will name a column and will still not name a workstream, and the same holds for the seven other commands that take one. Round 2's reviewer has posted that on dinah-470 with two ways out, the cheaper being a sixth kind. Nothing about it is this card's to fix, and this card files nothing for it, because it is already raised on the card whose answer decides it.

**Two residues stay here, and neither is a card.** Both are recorded rather than fixed, per the workbench's rule that a finding nobody would pick up on its own is a note rather than a card.

1. The one-line command summaries are outside dinah-470's scope entirely, because that card rewrites only `param.*` keys. `cmd.edit.summary` reads "Open this workbench, a card, or anything below a card in your editor" and `cmd.path.summary` reads "Print the file path of this workbench, of a card, or of anything below a card". Both omit a column, which both commands have always taken, and both omit a workstream. Verified in `internal/msg/locales/en.json` at 808d105. Whoever fixes them owes the two English texts plus `de` and `hi` retranslations with fresh `source` fingerprints and the five skeleton catalogues.
2. `edit`'s refusal table in its own help page is short a row. `internal/verb/checks.go:212` declares two rows for `edit`, `contract.UnknownPath` as `check.edit.1` and `contract.NoEditor` as `check.edit.2`, and dinah-455 added the `dinah.is-a-collection` refusal without adding a row for it. Neither this card nor dinah-470 touches that table. This card does not add the row either, because doing so would mint a catalogue key and D-2 records that this card mints none, which is what keeps section 6's second bullet true.

## 8. Arming the guards

Every test above is armed by breaking the behaviour it guards, watching it redden, restoring a byte-identical copy, and watching it go green. Four plants are named because each one arms a different claim, and all four compile. Plant 1's expected outcome is corrected in round 2 and plant 4 is new.

- **The pre-fix behaviour.** Replace the whole body of `ResolveEditTarget` with `return b.ResolvePath(ref)`. Round 1 claimed this reddens the workstream shapes alone. It does not, and an implementer who believed it would have concluded something was broken. `ResolvePath` answers a collection reference with the collection's directory and exit 0, which was run for all seven collection spellings at 808d105 and returned a path every time, so removing the collection arm un-refuses every shape declared `contract.IsACollection` as well as reddening the two workstream shapes. What must redden is exactly two classes: every shape whose declared refusal is `contract.IsACollection`, which fails the "requires a `*contract.Refusal`" half, and both `KindWorkstream` shapes, which fail the "requires a regular file" half. Against the fixture and generator section 4.5 specifies that is eleven shapes, derived as six collection mounts reachable from the workbench through `bench.Contains` (`workbench/attachments`, `<column>/attachments`, `<card>/comments`, `<card>/checklist`, `<card>/attachments`, `<comment>/attachments`), plus the three narrowed checklist words Group B emits as collections, plus the two workstream spellings. Two shapes that look as though they should redden must stay green: `workbench/cards` and `workbench/columns` keep refusing `dinah.unknown-path`, because `ResolvePath` raises that refusal itself, which was run at 808d105 and returned exit 2 for both. The implementer records the observed reddening set and reconciles it against that derivation rather than against the number; a difference means the generator emits a different set from the one specified here, and the difference is the finding.
- **Refusing everything.** Make `ResolveEditTarget` return `contract.Refuse(contract.UnknownPath, ref)` before it does anything else. Every shape whose `opens` is true must redden. This is the plant that proves the sweep pins the accepting cases and is not satisfied by code that refuses the lot.
- **A thin subject set.** Delete the two workstream shapes from Group B of `editReferenceShapes`. The `wantEditShapes` and `wantEditOpens` assertions must redden, and so must the named-kind assertion for `bench.KindWorkstream`. This is the plant that proves the count guard is armed rather than decorative.
- **The empty reference, opened.** Delete arm 1 of `ResolveEditTarget`, so an empty reference reaches `ResolveReference` and comes back as the workbench. Exactly the Group C whitespace shapes must redden, four of them at the resolver, reporting that a shape declared refusing opened `workbench.md`; `TestEditHandsTheEditorTheFileTheResolverNames` must also redden on its no-argument case, reporting an editor launch where none was allowed. Every other shape must stay green, which is what distinguishes this plant from plant 2. This is the plant that arms round 2's central correction, and it is the one whose absence let round 1 ship a design contradicting its own AC-5.

Record in the handoff what each red run printed, and confirm from the test count that each plant built and ran rather than failing to compile, since a build failure prints nothing and nothing looks like everything passing.

## Branch

dinah-467-edit-opens-a-workstreams-directory-in-your-editor-instead-of-its-anchor
