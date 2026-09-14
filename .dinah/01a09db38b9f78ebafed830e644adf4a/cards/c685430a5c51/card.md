---
title: attach writes attachments under kinds the containment grammar cannot address
column: b69abf918c42
state: ready
severity: minor
priority: soon
tier: workhorse
workstreams:
  - 994787601ae6
---
`dinah attach <card>/oq/1 <file>` succeeds at trunk 22a35fc and writes an attachment nothing can address afterwards. `Library.Attach` resolves a reference and never checks the resolved kind against the containment table, where `item` and `attachment` mount nothing.

What the run produced, on a probe workbench built from that commit: the attach exits 0; `dinah path <card>/oq/1/attachments/1` refuses `dinah.unknown-path`; `dinah attachments <card>/oq/1` lists the file anyway; and `dinah contents <card>` reports a count short by one, because the containment walk cannot see it. The references guide states the opposite, saying `attach` takes nothing below a card but a comment or an attachment.

`attach` refuses `dinah.not-attachable` when `bench.MountOf(kind, bench.AttachmentsDir)` reports no mount, reading the table rather than naming the two kinds. `dinah check` gains a finding naming an attachments directory under a kind the table gives no mount, and the finding reports without repairing, because the bytes are somebody's evidence.

dinah-456 section 5.6 is the contract, and D-14 there records why the existing strays are not deleted.

## Specification

This card implements section 5.6 of dinah-456, and D-14 there governs what happens to the
attachments the defect has already written. Nothing below re-derives the containment
contract; D-5 of that card settles which kinds mount what, and this card takes it as given.

Everything below was read and run against `origin/main` at
22a35fca90c254625071b9b7eca237131cdce3b0. The reproduction ran a binary built from that
commit against a throwaway workbench at `C:/dinah-scratch/dinah-459-spec/probe`, with
`DINAH_HOME` pointed inside that directory, so nothing touched the operator's own data.

# 1. What the run showed

The probe workbench holds one card, `probe-1`, with one open question, one comment, one
workstream, and one attachment on the card itself.

```
$ dinah attach probe-1/oq/1 f.txt
probe-1  A probe card  [Intake / ready]        <- exit 0
$ dinah path probe-1/oq/1/attachments/1
dinah.unknown-path nothing in this workbench answers to attachments; ...   <- exit 2
$ dinah attachments probe-1/oq/1
probe-1/checklist/1 carries 1 attachments.
  #  File
  -  -----
  1  f.txt
$ dinah contents probe-1
A probe card (probe-1) contains 1 entities.
  Reference                Entity  Title                Count
  -----------------------  ------  -------------------  -----
  `-- probe-1/checklist/1  item    Does attach refuse?  0
```

The write lands at
`.dinah/<id>/cards/9225494a1172/checklist/4abf066e1107/attachments/e4782321f44b/payload/f.txt`.
One command lists it, no command addresses it, the containment walk cannot see it, and the
item's own count reads zero. `dinah check` on that workbench answers `No structural defects
found.` and exits 0.

Two more kinds behave the same way, and only one of them is in the parent's enumeration.

```
$ dinah attach probe-1/attachments/1 g.txt        <- exit 0, nests an attachment
$ dinah path probe-1/attachments/1/attachments/1
dinah.unknown-path nothing in this workbench answers to attachments; ...
$ dinah attach workstream/probe-stream f.txt      <- exit 0, prints nothing at all
$ dinah path workstream/probe-stream/attachments/1
dinah.unknown-workstream this workbench carries no workstream probe-stream/attachments/1; ...
$ dinah contents workstream/probe-stream
 (probe-stream) contains nothing.
```

The four legal targets behave correctly and must go on doing so. `dinah attach workbench
f.txt`, `dinah attach intake f.txt`, `dinah attach probe-1 f.txt`, and `dinah attach
probe-1/comments/1 f.txt` each exit 0 and each leave an attachment `dinah path` resolves.
`dinah attach probe-1/attachments/1 g.txt --replace` also exits 0 and is a legal act: it
rewrites the bytes of an existing attachment rather than hanging a new one below it.

# 2. Why it happens, and how far it reaches

`Library.Attach` at `internal/verb/beyond.go:162` resolves the reference through
`Bench.ResolveEntity` and never asks the containment table what the resolved kind holds.
Its order today is the operator check, the resolve, the owner check, and the file check,
after which it takes the lock and writes.

`internal/bench/containment.go` gives `KindItem` and `KindAttachment` an empty mount list,
and leaves `workstream` out of the table altogether, which `MountOf` reports the same way.
Three of the seven kinds `ResolveEntity` can answer with therefore mount no `attachments`
collection, and `attach` writes into all three.

The blast radius is one verb. Three functions in `internal/bench` create an entity below
another one, and a tree-wide search names every production call site of each:

```
$ grep -rn "AddAttachment(\|AddComment(\|AddItem(" --include=*.go . | grep -v _test.go
internal/bench/entity.go:33:func AddComment(...)
internal/bench/entity.go:170:func AddAttachment(...)
internal/bench/item.go:81:func AddItem(...)
internal/verb/beyond.go:141:	comment, err := bench.AddComment(found.Card.Dir, ...)
internal/verb/beyond.go:191:		attachment, err := bench.ReplaceAttachment(entity.Dir, req.File)
internal/verb/beyond.go:199:		attachment, err := bench.AddAttachment(entity.Dir, req.File, ...)
internal/verb/checklist.go:69:	item, err := bench.AddItem(found.Card.Dir, ...)
```

`AddComment` and `AddItem` are reached only through `Library.Comment` and `Library.File`,
both of which resolve a card through `Bench.ResolveCard` and can therefore land nowhere
else. `AddAttachment` has exactly one caller, which is the line this card guards. The other
four verbs that take a reference through `ResolveEntity` are `Archive`, `Delete`, `Rename`,
and `withItem`, and none of them creates a child, so none of them can open a hole of this
shape.

Both heads route through `Library.Attach`. `internal/mcp/tools.go:77` publishes the `attach`
tool as a direct call to it, so one guard covers the CLI and the MCP surface with no second
edit.

The guide is already right about this. `internal/guide/guides/references.md:77` says
`attach` "takes a comment or an attachment below a card and takes nothing else below one",
and `param.attach.ref` in all eight catalogs lists the workbench, a column, a card, a
comment, and, with `--replace`, an attachment. The code is what has to catch up.

# 3. The refusal

## 3.1 The name

`internal/contract/contract.go` gains `NotAttachable = LayerPrefix + "not-attachable"`,
declared beside `NotRenamable` at line 411 and added to the `Introduced` slice at line 517.
dinah-456 D-10 minted this token for exactly this shape, and `dinah.is-a-collection`, the
other token that decision minted, belongs to dinah-455 and is no part of this card.

Doc comment on the constant:

```go
	// NotAttachable is an attach aimed at a reference that resolves to a kind
	// the containment table gives no attachments collection. The detail names
	// the reference and the kind rides beside it, so the caller sees what the
	// reference reached rather than what they hoped it would reach.
	NotAttachable = LayerPrefix + "not-attachable"
```

## 3.2 The shape

`internal/contract/shape.go` gains one entry in `Shapes`, modelled on the `UnknownPath`
entry at line 736, whose Values double as its fragment conditions:

```go
	{
		// attach is the only verb that writes a new entity below the
		// reference it is handed, so it is the only one that can be aimed at
		// a kind the containment grammar gives nothing to hang from. The next
		// step splits on the kind, because the honest advice differs: an item
		// takes its evidence by citation, an attachment wraps bytes and holds
		// nothing, and the unconditional member covers every other kind the
		// table leaves out, which is the workstream today.
		Name:   NotAttachable,
		Values: []string{"kind", "item", "attachment"},
		Fragments: []Fragment{
			{Key: "refusal.dinah.not-attachable.next-item", When: "item"},
			{Key: "refusal.dinah.not-attachable.next-attachment", When: "attachment"},
			{Key: "refusal.dinah.not-attachable.next"},
		},
		NextStep: []string{
			"refusal.dinah.not-attachable.next-item",
			"refusal.dinah.not-attachable.next-attachment",
			"refusal.dinah.not-attachable.next",
		},
	},
```

`holds` in `cmd/dinah/render.go:1036` switches a fragment on the presence of a named value
and on nothing else, so the branch value is the reference itself, stored under the name of
the kind it belongs to. `checkNoPlaceholderIsStrayOrOrphaned` in
`internal/profile/guards_test.go:3212` accepts a placeholder whose name is the fragment's
own condition, which is what lets `{item}` and `{attachment}` render inside the fragments
they switch on.

`checkEveryShapeSaysWhatToDoNext` in the same file fails a shape whose last `NextStep`
member carries a condition, so the unconditional third member is required rather than
optional. It is also the member a workstream reference renders.

## 3.3 The English

`internal/msg/locales/en.json` gains four entries. Each `context` is written for a
translator who cannot see the code.

```json
"refusal.dinah.not-attachable": {
  "text": "{detail} is {kind}, and Dinah keeps no attachments on {kind}",
  "context": "The sentence printed after the refusal name dinah.not-attachable, when attach is aimed at a reference that resolves to a kind that holds no attachments. {detail} is the reference the caller typed and {kind} names what it resolved to, which is machine vocabulary and is never translated."
},
"refusal.dinah.not-attachable.next-item": {
  "text": "; attach the file to the card instead, then cite it from {item} with `dinah cite`",
  "context": "Spliced onto refusal.dinah.not-attachable when the reference resolved to a checklist item. {item} repeats that reference. A checklist item takes its evidence by citation rather than by holding a copy, so the reader is sent to the card and then back to the item."
},
"refusal.dinah.not-attachable.next-attachment": {
  "text": "; an attachment carries bytes rather than other attachments, so attach the file to whatever {attachment} hangs from",
  "context": "Spliced onto refusal.dinah.not-attachable when the reference resolved to an attachment. {attachment} repeats that reference. An attachment wraps the bytes of one file, so a second file belongs on whatever the first one hangs from."
},
"refusal.dinah.not-attachable.next": {
  "text": "; attachments hang from the workbench, a column, a card, or a comment",
  "context": "The last member of refusal.dinah.not-attachable's alternation, printed for any kind the two branches above do not name. It carries no condition, because the last member of an alternation is what a reader gets when none of the branches matches."
}
```

The base sentence keeps the wording dinah-456 section 3.4 minted and spells the reference
`{detail}` rather than `{ref}`. The head fills `detail` with the reference already, in
`session.refusalValues` at `cmd/dinah/render.go:1049`, and `dinah.not-renamable` carries the
caller's reference in that slot today. A second value holding the same string would print
the reference twice into the machine payload and would buy nothing.

The item advice names no evidence scheme. A workbench declares its own schemes in its
frontmatter, `Bench.EvidenceObservedRequired` reads them from there, and a workbench made by
`dinah init` declares none, so a sentence naming `attachment` as a scheme would be true of
Andoneer's own workbench and false of a new one.

`TestEverySplicedFragmentCarriesItsOwnSeparator` requires each fragment to begin with its
own punctuation, and all three begin with a semicolon and a space. None of the four texts
carries the string `dinah check`, so `TestEveryCheckAdviceIsDispositioned` gains no entry.

## 3.4 The eight catalogs

`de` and `hi` carry a translation of each of the four keys plus a `source` equal to
`msg.Fingerprint` of the English above. `cs`, `id`, `es`, `fil`, and `af` carry the English
text with `"skeleton": true` and no `source`. The workbench document "Translation staleness
contract" governs, and its standing ruling of 2026-08-31 applies: translate in-language
following the register of the neighbouring entries, ship it, and do not file a question
asking whether the wording reads naturally.

The neighbours to follow are `refusal.dinah.not-renamable` and its `.next`, which refuse the
same shape of mistake for a related reason and which both catalogs already carry.

These are new keys rather than a skeleton fill, so the per-key branch of that document's
amendment applies, and the implementer files one `decision`-kind checklist item per key it
adds to or changes in a translated catalog.

## 3.5 The guard, and where it sits in the order

`Library.Attach` gains one guard between the owner check and the file check:

```go
	// Replacing an attachment's bytes writes nothing below it and stays legal,
	// so one expression decides both the refusal and the branch that writes,
	// and the two cannot drift apart.
	replacing := req.Replace && entity.Kind == bench.KindAttachment
	if _, mounts := bench.MountOf(entity.Kind, bench.AttachmentsDir); !mounts && !replacing {
		return l.refuseWith(req, entity.Card, contract.NotAttachable, entity.Ref,
			map[string]string{"kind": entity.Kind, entity.Kind: entity.Ref})
	}
```

The write branch at `internal/verb/beyond.go:190` then reads `if replacing {` in place of
its inline condition.

The guard asks `MountOf` and names no kind, which is what dinah-456 D-14 requires of the
check and which is the same reason here. A kind the table later gives an attachments mount
becomes attachable with no second edit, and a kind added without one is refused from the day
it exists.

The second map entry uses the kind as its own key, so an `item` reference carries
`values["item"]`, an `attachment` reference carries `values["attachment"]`, and a
`workstream` reference carries a value no fragment reads, which is how the unconditional
next step is reached. `entity.Ref` is the canonical spelling of the reference, and it is
what `dinah.not-renamable` carries at `internal/verb/beyond.go:319`.

For a workstream, `EntityRef.Ref` is the bare slug rather than `workstream/<slug>`, because
`Workstream.Ref` at `internal/bench/workstream.go:96` answers with the slug alone.
`dinah.not-renamable` prints the same bare slug today, verified by running
`dinah rename workstream/probe-stream x.txt --json`, whose payload reads
`"detail": "probe-stream"`. Changing that spelling would move printed output on every
surface that names a workstream, so this card keeps the house form and leaves the wart where
it found it.

## 3.6 The precondition list

`internal/verb/checks.go:225` declares two rows for `attach`, and the first of them is
already wrong: it says the reference and the file both resolve before the owner is checked,
and the code checks the owner between the two. Adding a row in the middle would make it
wronger, so the list is split to say what the code does.

```go
	"attach": {
		{Refusal: contract.UnknownPath, Key: "check.attach.1"},
		{Refusal: contract.NoOwner, Key: "check.attach.2"},
		{Refusal: contract.NotAttachable, Key: "check.attach.3"},
		{Refusal: contract.UnknownPath, Key: "check.attach.4"},
	},
```

Catalog entries, in all eight catalogs, each carrying the context every other row of this
family carries, which is "One row of the precondition list of a command outside the five the
profile specifies."

| key | English |
|---|---|
| `check.attach.1` | `the reference resolves`, changed from `the reference and the file both resolve` |
| `check.attach.2` | `the request names an owner`, unchanged |
| `check.attach.3` | `the reference names a kind that keeps attachments` |
| `check.attach.4` | `the file resolves` |

`check.attach.1` changes its English, so its German and Hindi entries need a fresh `source`
and a decision record whether or not their text moves.

Two width guards govern these strings. `TestChecksColumnNeverGluesInAnyLanguage` renders
every row of every command in every shipped locale at a 52-rune width and fails a glued row.
`TestTheArgumentsTableWrapsAndNoOtherTableMoved` fails any help line wider than 80 display
columns at `COLUMNS=80`. On the attach page the refusal column widens to the 20 columns of
`dinah.not-attachable`, which leaves 49 for the middle column. The English above is 48 and
fits, and a German rendering has the same 49 to work in.

# 4. What `dinah check` reports

dinah-456 D-14 binds this half. The strays are reported and are not deleted, and the check
reads the containment table through its own accessor rather than naming the kinds.

## 4.1 The finding

`internal/bench/check.go` gains one constant beside the others:

```go
	// FindingAttachmentsWithoutAMount names an attachments directory sitting
	// below an entity whose kind the containment table gives no attachments
	// mount. The attach verb wrote them before it refused the act, and nothing
	// reaches them afterwards: descend refuses the path and the containment
	// walk cannot see them, so the entity holding them reports a count short by
	// what is inside.
	//
	// Path names the directory itself rather than the anchor above it, because
	// a reader has to open it to decide what to do with the files, and Detail
	// names the kind rather than an identifier, because the identifier is
	// already the last segment of the path. Nothing repairs it: the bytes
	// belong to whoever attached them, and the anchor beside each one records a
	// filename and a provenance that a silent removal would destroy.
	FindingAttachmentsWithoutAMount = "check.attachments-without-a-mount"
```

Catalog entry in `en.json`, plus the German, the Hindi, and the five skeletons:

```json
"check.attachments-without-a-mount": {
  "text": "{detail} carries an attachments directory, and Dinah keeps no attachments on {detail}, so nothing reaches the files inside it",
  "context": "A check finding: an attachments directory sitting below an entity of a kind that holds no attachments. {detail} names that kind, which is machine vocabulary and is never translated. The path beside the sentence is the directory itself, because the reader has to open it to decide what to do with the files. Dinah does not remove it, because the files belong to whoever attached them."
}
```

`renderFindings` at `cmd/dinah/render.go:829` fills `detail` and appends the path in
parentheses, and it substitutes nothing else, so the sentence names no other slot.

## 4.2 The walk

`internal/bench/check.go` gains two functions, and `Bench.Check` calls the first of them
beside `b.checkWorkstreams()` at line 258.

```go
// checkAttachmentsWithoutAMount reports every attachments directory sitting
// below an entity whose kind mounts no attachments collection.
//
// The walk descends the containment table and asks MountOf at each entity
// rather than naming the kinds it expects, so a kind the table later gives an
// attachments mount stops being reported here with no second edit, and a kind
// added without one is covered from the day it exists.
//
// The workstreams are walked beside the table rather than through it, because a
// workstream is a membership rather than a container and the table deliberately
// leaves it out. The reference grammar reaches one all the same, so attach can
// be aimed at one and the collection has to be swept.
func (b *Bench) checkAttachmentsWithoutAMount() []Finding {
	var findings []Finding
	for _, mount := range Contains(KindWorkbench) {
		dir := filepath.Join(b.Root, mount.Dir)
		for _, id := range ListIDs(dir) {
			findings = append(findings, b.mountlessAttachmentsBelow(filepath.Join(dir, id), mount.Kind)...)
		}
	}
	root := b.WorkstreamsRoot()
	for _, id := range ListIDs(root) {
		findings = append(findings, b.mountlessAttachmentsBelow(filepath.Join(root, id), KindWorkstream)...)
	}
	return findings
}

// mountlessAttachmentsBelow visits one entity, and everything the containment
// table says hangs below it, reporting an attachments directory wherever the
// kind mounts none.
//
// A kind mounting no attachments is a leaf of the grammar, so the walk reports
// what it finds there and descends no further: a directory below a stray is
// unreachable for the same reason the stray is, and one finding names the whole
// of what an operator has to look at.
func (b *Bench) mountlessAttachmentsBelow(dir, kind string) []Finding {
	if _, mounts := MountOf(kind, AttachmentsDir); !mounts {
		attachments := filepath.Join(dir, AttachmentsDir)
		if !Exists(attachments) {
			return nil
		}
		return []Finding{{Path: attachments, Key: FindingAttachmentsWithoutAMount, Detail: kind}}
	}
	var findings []Finding
	for _, mount := range Contains(kind) {
		collection := filepath.Join(dir, mount.Dir)
		for _, id := range ListIDs(collection) {
			findings = append(findings, b.mountlessAttachmentsBelow(filepath.Join(collection, id), mount.Kind)...)
		}
	}
	return findings
}
```

`siblingCollections` and `collectionsBelow` at `internal/bench/finish.go:127` already walk
the table this way for the interruption sweep, and this pair is the same descent with a
question asked at each stop. They stay separate functions because that pair answers with
collection directories and this one answers with findings, and folding them together would
give one function two return shapes.

The walk covers the live tree and not the archive mirror, which is what every other member
of `Bench.Check` does.

It reports an `attachments` directory and no other collection directory. Nothing writes a
`comments` or a `checklist` directory below a kind that mounts none, because the two verbs
that could resolve a card and only a card, which section 2 shows by command. A generalised
sweep would therefore report exactly what this one reports, at the cost of a directory
listing per entity per mount.

# 5. The workstream kind gets a constant

`internal/bench/entity.go:383` assigns the entity kind as the bare string `"workstream"`,
and six comparisons in `internal/verb/beyond.go` read it back the same way. The check above
has to name that kind, and naming it as a seventh literal would put one spelling in a
seventh place.

`internal/bench/containment.go` gains the constant beside the other six, with a comment
saying why it is not in the table:

```go
	// KindWorkstream is one workstream. It is named by the reference grammar
	// and it is deliberately absent from the containment table below, because a
	// workstream is a membership rather than a container: cards join and leave
	// one, and a card is not contained by one.
	KindWorkstream = "workstream"
```

Every production site that decides an entity kind reads a constant afterwards. The set is
`internal/bench/entity.go:383`, `internal/verb/beyond.go` lines 639, 652, 697, 710, 736, and
748, and the `"attachment"` literal at `internal/verb/beyond.go:190`, which becomes
`bench.KindAttachment`. The set was produced by
`grep -rn '"workstream"' --include=*.go . | grep -v _test.go` over the tree and by reading
each hit. The hits left alone are the command name, the JSON tags, the query field name, the
search kind, and the parameter tables, none of which is an entity kind.

Nothing enforces the convention afterwards. `internal/bench/kindguard_test.go` scans for
column-kind literals and not for entity-kind ones, and widening it is a card of its own
rather than a line of this one. AC-9 runs the search and compares it against a declared
expectation, and it does not stop a later file from introducing a new literal.

# 6. The guide

`internal/guide/guides/references.md:76` opens a paragraph of four details the command table
is too coarse to hold. The `attach` detail is true today and becomes untrue once an
attachment reference needs `--replace`, so this card corrects it. The other three details in
that paragraph, and the table above it, are left alone.

Replace:

> `attach` takes a comment or an attachment below a card and takes nothing else below one,
> so `dinah attach wb-1/journal notes.md` is refused.

with:

> `attach` takes a comment below a card, and it takes an attachment only with `--replace`,
> which replaces that attachment's bytes rather than hanging a new file below it. It takes
> nothing else below a card, so `dinah attach wb-1/oq/1 notes.md` is refused.

The table's `attach` row still reads `yes` under "Below a card", because a comment is below a
card. The guide gives workstreams no row at all, so nothing in it becomes false when
`attach workstream/<slug>` starts refusing, and dinah-457 is the card that gives the
workstream form its place in the guide.

# 7. What this card does not do

It does not change `dinah attachments`, which goes on listing a stray under an item until
somebody removes it. That is how an operator reads what the check names, and dinah-456 D-14
leaves the removal to them.

It deletes and moves nothing, and `dinah check` gains no repair flag for this finding.

It does not touch `dinah.is-a-collection`, which dinah-456 D-10 minted for dinah-455.

It does not change `EntityRef.Ref` for a workstream, and it adds no workstream row to the
references guide.

It does not widen the check to collection directories other than `attachments`.

It writes no journal event, so no compatibility fixture under
`internal/bench/testdata/compat/` is recaptured and no manifest digest is re-blessed.

# 8. The tests

Three new tests carry the behaviour, and the existing guards catch the rest. Every new test
is armed the way the workbench requires: break the behaviour, watch it go red, restore from
a byte-identical copy, and record what the red run said.

- `internal/verb/beyond_test.go`, `TestAttachRefusesAKindTheContainmentTableGivesNoMount`.
  It carries both halves of the contract, the refusal and the permission, because a guard
  that only proves the refusal passes on code that refuses everything.
- `internal/bench/check_test.go`,
  `TestCheckReportsAnAttachmentsDirectoryUnderAKindThatMountsNone`. `newFixture`,
  `writeItem`, `writeAttachment`, `writeComment`, and `writeWorkstream` in that file already
  plant the shapes it needs.
- `internal/bench/check_test.go`, `TestAKindGivenAnAttachmentsMountStopsBeingReported`. The
  test is in package `bench`, so it can add a mount to the `containment` map, re-run
  `Bench.Check`, and restore the map in `t.Cleanup`.
- `cmd/dinah/attachments_command_test.go`,
  `TestTheNotAttachableRefusalPrintsTheAdviceForItsKind` and
  `TestTheAttachHelpPageNamesTheKindPrecondition`.

## Branch

dinah-459-attach-writes-attachments-under-kinds-the-containment-grammar-cannot-address
