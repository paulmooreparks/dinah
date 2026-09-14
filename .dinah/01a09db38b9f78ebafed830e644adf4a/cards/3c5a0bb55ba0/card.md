---
title: Naming a collection tells the reader it does not exist, when what the command refused was the addressing
column: b69abf918c42
state: ready
severity: major
priority: soon
workstreams:
  - 994787601ae6
links:
  - kind: spawned_from
    to: c7d5ff7728c1
---
`dinah show cpe-1/comments` answers `dinah.unknown-path nothing in this workbench answers to comments; run dinah ls to see what this workbench carries`. The comments collection is there, the card has a comment in it, and `dinah path cpe-1/comments` prints its directory. What `show` refused is a collection reference, because it reads one entity and a collection is not one. The reader is told the thing does not exist.

This is the defect family this workbench has closed ten times in recent weeks: a failure that arrives as an absence, and the absence then reads as a confident negative answer. The other instances were empty collections standing in for failed reads; this one is a refusal that names the wrong cause. The reader's next move is wrong in both cases, and here it is worse than useless, because the advice attached to it (`run dinah ls`) sends them to look for something they will not find, since `ls` lists cards.

**This card is specified by dinah-456 sections 3.3 and 3.4.** Read those before starting. Section 3.3 rules that `<head>/<collection>` is a well-formed expression naming every live member in creation order, and gives the per-command contract: `path`, `show`, `contents` and `attachments` accept a collection, and eleven writing commands plus `instructions` refuse it. Section 3.4 mints the refusal, `dinah.is-a-collection`, with its English text, the details it carries, and a second key for the empty case. The XPath framing does not dissolve this card. It dissolves the half where a collection reference is malformed, and what it leaves is the harder half: eight commands refuse a collection today with a sentence saying it does not exist, and after this contract four accept it and eleven refuse it deliberately with a sentence that says what the reference names and what to type instead.

**The cause stated in the original filing is wrong and is corrected here.** It said the refusal comes from `Bench.ResolveEntity` asking `KindOfAnchor` what kind of entity the collection directory's base names. That is not the path `show` takes. At trunk 22a35fc, `Library.Show` sends a composed reference through `Bench.ResolvePath`, which returns the collection directory quite happily, and refuses only when `bench.ReadText` fails on that directory, raising `dinah.unknown-path` with the trailing segment as the detail (`internal/verb/read.go:773-780`). So `show` never reaches `ResolveEntity` or `KindOfAnchor`, and a fix aimed at those two would leave the reported surface answering exactly as it does today. Other commands do resolve through `ResolveEntity`, so the sweep has to establish the path each one takes rather than assuming one path for all of them.

The guide is wrong too, and dinah-457 owns that half: `internal/guide/guides/references.md` says `show` accepts anything below a card, which is what led the reader to type this reference in the first place.

Related: dinah-454 is the other half of the same reader's problem, that nothing on the card screen tells them how to name a comment at all.

## Specification

Specified by dinah-456 sections 3.3 and 3.4. Nothing below re-opens that contract; where this spec goes further than the parent it says so and gives the reason.

Everything asserted about current behaviour was established at trunk `b825059101980d52eb33dbd5bc62491f4c3c1e7a` ("dinah-438: repair the dashed array reader so a multi-key entry keeps all its fields"), reached by `git fetch origin` on 2026-09-09, in the worktree `C:/dinah-scratch/dinah-455-spec/wt`, with a binary built there and run against a throwaway workbench under the same directory with `DINAH_HOME` beside it. Section 10 records the runs.

# 1. What this card lands

Four commands come to accept a reference naming a whole collection, eleven come to refuse one with a sentence that says what the reference names and what to type instead, and the refusal is a new name rather than a widening of `dinah.unknown-path`.

The reported defect is one row of that. `dinah show pb-1/comments` answers `dinah.unknown-path nothing in this workbench answers to comments`, and after this card it prints the comments.

**Three counts in the framing are wrong at trunk, and these are the right ones.** The roster of commands taking a reference is fifteen rather than sixteen, derived in section 2.1 by running dinah-456 section 2.2's own derivation at `b825059`. Of the fifteen, four accept a collection and eleven refuse it, which is dinah-456 section 10's arithmetic. The parent's section 3.3 prose calls the refusers "eleven writing commands plus `instructions`", which is twelve and disagrees with the fifteen-row table in the same subsection; ten of the eleven refusers write, and `instructions` is the eleventh. The table governs and the sentence is the slip.

# 2. What is true at `b825059`

## 2.1 The fifteen commands that take a reference

Derived rather than counted, by running dinah-456 section 2.2's script over `internal/verb/definition.go` in the worktree:

```
$ python - <<'PY'
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
15 ['archive', 'attach', 'attachments', 'cite', 'contents', 'delete', 'edit', 'fail', 'instructions', 'path', 'rename', 'reopen', 'resolve', 'show', 'verify']
15 ['archive', 'attach', 'attachments', 'cite', 'contents', 'delete', 'edit', 'fail', 'instructions', 'path', 'rename', 'reopen', 'resolve', 'show', 'verify']
```

A sixteenth command taking a reference under another name does not exist: the only other parameter in that table named `ref`, `reference`, `target` or `path` is `cite`'s second positional `target`, which names an attachment kind and a selector rather than a reference, and `workbenches`'s `path`, which names a directory on the filesystem.

## 2.2 What each of the fifteen does with a collection reference today

Run against a workbench initialised from a binary built at `b825059`, on card `pb-1` carrying one comment, one open question and one attachment. Every invocation used `pb-1/comments`, including the five checklist verbs, because a collection reference is legal input to all fifteen and the question is what each does with it.

| Invocation | Result at `b825059` |
|---|---|
| `dinah path pb-1/comments` | prints the collection directory, exit 0 |
| `dinah edit pb-1/comments` | resolves and hands the directory to the editor, exit 0 with a shim editor and a failure with a real one |
| `dinah show pb-1/comments` | `dinah.unknown-path nothing in this workbench answers to comments; run \`dinah ls\` to see what this workbench carries`, exit 2 |
| `dinah contents pb-1/comments` | the same sentence |
| `dinah attachments pb-1/comments` | the same sentence |
| `dinah archive pb-1/comments` | the same sentence |
| `dinah delete pb-1/comments --yes` | the same sentence |
| `dinah rename pb-1/comments x.md` | the same sentence |
| `dinah attach pb-1/comments f.txt` | the same sentence |
| `dinah cite pb-1/comments attachment 1` | the same sentence |
| `dinah resolve pb-1/comments note` | the same sentence |
| `dinah verify pb-1/comments note` | the same sentence |
| `dinah fail pb-1/comments note` | the same sentence |
| `dinah reopen pb-1/comments why` | the same sentence |
| `dinah instructions pb-1/comments` | the same sentence, quoting `pb-1/comments` rather than `comments` |

Twelve refuse the same way, one refuses with a wider detail, one resolves into an editor, and one answers.

## 2.3 Where each refusal actually comes from

The card's own description corrects the original filing and is right about `show`, and the correction holds at trunk with the line numbers moved. `Library.Show` at `internal/verb/read.go:748` cuts the head at the first `/`, and for a composed reference it calls `Bench.ResolvePath`, which returns the collection directory, then refuses only when `bench.ReadText` fails on that directory, at `read.go:785-788`, quoting the whole tail as the detail. `show` reaches neither `ResolveEntity` nor `KindOfAnchor`.

The other fourteen split three ways, established by reading each call path rather than by assuming one path:

- **Eleven resolve through `Bench.ResolveEntity`** at `internal/bench/entity.go:722`, which walks to the collection directory and then asks `KindOfAnchor` what kind of anchor its base name is. A collection directory is not an anchor, so the `!named` branch at `entity.go:755-757` refuses `dinah.unknown-path` with the tail as the detail. The eleven are `attach`, `archive`, `delete` and `rename` (`internal/verb/beyond.go:166, 231, 273, 322`), the five checklist verbs through the one `withItem` body (`internal/verb/checklist.go:251`), `attachments` (`internal/verb/read.go:942`) and `contents` (`internal/verb/tree.go:875`).
- **`instructions` never resolves the grammar at all.** It takes a column or a card, and `internal/verb/read.go:1138-1140` refuses `dinah.unknown-path` with `req.Card` when `ResolveCard` fails, which is why its detail is the whole reference.
- **`path` and `edit` resolve in the head**, both through `Bench.ResolvePath` at `cmd/dinah/commands.go:1191` and `cmd/dinah/commands.go:1227`. Neither is served as an MCP tool.

So one edit inside `ResolveEntity` reaches eleven of the fifteen, and the other four are edited one at a time. The original filing's cause was wrong about `show` and right about those eleven.

## 2.4 A collection with no directory does not resolve at all

`descend` at `internal/bench/resolve.go:294` returns the collection directory only when `Exists` says it is there. A card that has never carried a comment has no `comments` directory, so:

```
$ dinah path pb-2/comments
dinah.unknown-path nothing in this workbench answers to C:\dinah-scratch\dinah-455-spec\probe\pb\.dinah\01a0861e440671e1bae7c6c353146334\cards\83b30b4c7b3f\comments; run `dinah ls` to see what this workbench carries
```

That is the reported defect twice over. It denies a collection the containment table declares, and it leaks an absolute path into a refusal a reader is meant to act on. It is also the commonest case, since most cards carry no comment, so a fix that only reaches collections which already hold something would leave the reported surface answering the reported sentence for most cards. Section 3.2 closes it.

# 3. The resolver

## 3.1 One resolution, two answers

`internal/bench` gains one exported resolver, and `ResolveEntity` becomes a reading of it:

```go
// ResolveReference resolves any reference the grammar admits and answers
// which of the two things it names: one entity, or a whole collection.
// Exactly one of the two is non-nil whenever the error is nil.
func (b *Bench) ResolveReference(ref string) (*EntityRef, *CollectionRef, error)

// ResolveEntity ... [existing doc] ... A reference naming a whole collection
// is refused dinah.is-a-collection here rather than in each caller, because
// every caller of this resolver takes one entity.
func (b *Bench) ResolveEntity(ref string) (*EntityRef, error) {
	entity, collection, err := b.ResolveReference(ref)
	if err != nil {
		return nil, err
	}
	if collection != nil {
		return nil, collection.Refuse()
	}
	return entity, nil
}
```

`CollectionRef` is what a reference naming a whole collection resolves to:

```go
// CollectionRef is what <head>/<collection> resolves to: the directory the
// collection lives in, the containment mount it is, and its live members in
// creation order. A collection is not an entity of the format, so it has no
// anchor and no identifier of its own.
type CollectionRef struct {
	// Ref is the reference as typed, trimmed.
	Ref string
	// Dir is the collection's directory, whether or not it exists yet.
	Dir string
	// Mount is the containment-table mount the last step named, which is
	// where the member kind and the member anchor filename come from.
	Mount Mount
	// Narrow is the item kind a checklist segment selected, empty for every
	// other collection and for the bare checklist segment.
	Narrow string
	// Members are the member directory identifiers, in creation order and
	// after any narrowing, which are the ids the positional selector counts.
	Members []string
	// Holder is the entity the collection hangs from.
	Holder *EntityRef
}

// FirstMember is the reference of the collection's first member, and the
// empty string when the collection holds none. A collection reference plus
// /1 always names its first member, because a position is counted in the
// same creation order this list is in.
func (c *CollectionRef) FirstMember() string

// Refuse is the refusal a command that takes one entity raises when its
// reference names a whole collection.
func (c *CollectionRef) Refuse() error
```

`Refuse` builds its named values inline at each of its two raise sites rather than filling one map conditionally, because `internal/profile/guards_test.go`'s check 5 reads a composite literal written at the raise site and reports a container it cannot read to its strings:

```go
func (c *CollectionRef) Refuse() error {
	count := strconv.Itoa(len(c.Members))
	if len(c.Members) == 0 {
		return contract.RefuseWith(contract.IsACollection, c.Ref, map[string]string{"count": count})
	}
	return contract.RefuseWith(contract.IsACollection, c.Ref, map[string]string{"count": count, "member": c.FirstMember()})
}
```

## 3.2 What the walk has to report, and the one behaviour it changes

`descend` returns a collection directory at exactly one branch, the one where the segments run out on a collection name (`internal/bench/resolve.go:325-331`). That branch is where the walk learns it has landed on a collection, so it is where it says so, through an out-parameter the callers that care pass and the callers that do not pass nil:

```go
// landing is what the walk reports about where it stopped, filled only by
// the branch that stops on a collection. A caller wanting the answer passes
// a landing to write into; every other caller passes nil.
type landing struct {
	collection bool
	dir        string
	mount      Mount
	narrow     string
	holderDir  string
	holderKind string
}
```

`descend` and `walkBelowCard` gain a trailing `landed *landing` parameter and pass it down; `resolveBelow` gains a sibling that takes one, and keeps its current signature as a wrapper passing nil. `ResolveReference` passes one, and builds the `CollectionRef` from what comes back.

**The `Exists` guard on that branch goes.** A collection the containment table declares for the holder's kind resolves whether or not its directory has been created, and it is empty until somebody writes into it. Section 2.4 is why. This changes `path`, which is the one command whose behaviour the parent's table calls unchanged: `dinah path pb-2/comments` now prints the directory a first comment would be written into rather than refusing. That is the honest answer for a plumbing command, it is what `path` already prints for a collection that happens to exist, and no reference that resolved before stops resolving. `ListIDs` returns nil for a directory that is not there (`internal/bench/storage.go:156-160`), so an absent collection resolves as empty and `pb-2/comments/1` goes on refusing `dinah.unknown-path` through `pick`.

Two refusals that come earlier in the same walk are untouched, and neither may turn into the new one. A collection whose kind is addressed in its own right, which is `cards` and `columns`, still refuses `dinah.unknown-path` with the `addressed` detail at `resolve.go:299-308`, because `pb/cards` names a set of things each of which has its own address. A segment naming no mount of the holder's kind still refuses `dinah.unknown-path` with that segment.

## 3.3 The holder, and one list of members

`Holder` is the entity the collection hangs from, which is the reference minus its last segment. It is answered by `ResolveEntity` for every spelling except a bare workbench slug, which names no entity to that resolver and is a legal head below the workbench, so `ResolveReference` answers that one case itself with `&EntityRef{Kind: KindWorkbench, Dir: b.Root}`, which is byte-identical to what `ResolveEntity` answers for `workbench`. Leaving `Holder.Ref` empty for the workbench is deliberate and matches `ResolveEntity`: the two consumers that need a spelling for the workbench already carry their own rule for it, `Library.Attachments` at `read.go:952-955` and `Library.Contents` at `tree.go:897-900`, and inventing a third here would give the workbench a spelling nothing else uses.

The member list has one statement. The expression `SortByOrdinal(collection, mount.Anchor, ListIDs(collection))` appears today in `descend` and again as the default branch of `Library.containmentMembersOf` (`internal/verb/tree.go:1070`), and this card extracts it:

```go
// MemberIDs are one collection's live members, in the creation order a
// positional reference counts in.
func MemberIDs(collection string, mount Mount) []string
```

`descend`, `ResolveReference` and `containmentMembersOf`'s default branch all call it, so the list the resolver counts positions in and the list the containment walk draws rows from cannot become two lists. `CollectionRef.Members` is that list, narrowed by `filterByKind` when the segment was a checklist alias, which is what `descend` already does before `pick`.

# 4. The refusal

## 4.1 The name

`internal/contract/contract.go` gains one constant beside `NotRenamable` and `NotAttachable`, and its name goes into `Introduced`:

```go
	// IsACollection is a command that takes one entity handed a reference
	// naming a whole collection. The detail names the reference as typed,
	// the member count rides beside it, and so does the reference of the
	// first member, which is a spelling the reader can type.
	IsACollection = LayerPrefix + "is-a-collection"
```

The parent minted the name and gave the reason in its D-9: `dinah.unknown-path` means the workbench answers to nothing by that name, and a collection reference resolves to something real, so reusing it would go on telling a reader that a thing they can see does not exist. No second name is invented here.

## 4.2 The shape

`internal/contract/shape.go` gains one entry:

```go
	{
		// A collection reference resolves, so the reader is told what it
		// names rather than that it names nothing. The next step is an
		// alternation of two, because a collection holding members can
		// offer one of them to type and an empty collection cannot, and
		// the empty branch carries no condition so every rendering ends on
		// a next step. count is carried for a machine caller and named in
		// no entry, so it is not declared here: the guard reads Values to
		// find a name that outlived the entry using it, and a declared
		// value no sentence carries fails it.
		Name:   IsACollection,
		Values: []string{"member"},
		Fragments: []Fragment{
			{Key: "refusal.dinah.is-a-collection.next-member", When: "member"},
			{Key: "refusal.dinah.is-a-collection.empty"},
		},
		NextStep: []string{
			"refusal.dinah.is-a-collection.next-member",
			"refusal.dinah.is-a-collection.empty",
		},
	},
```

The empty branch keeps the key the parent minted, `refusal.dinah.is-a-collection.empty`, rather than being renamed to `.next-empty` for symmetry with its sibling. It splices the way `refusal.dinah.unknown-path.next-addressed` splices, which is what the parent said of it.

## 4.3 The English

Three entries in `internal/msg/locales/en.json`. The reference travels as `{detail}` rather than as the `{ref}` the parent's draft wrote, because `detail` is the slot the composer fills from a refusal's own detail and a shape that declared `ref` beside it would be declaring a second name for one value. dinah-459 made the same substitution when it landed `dinah.not-attachable`, whose shipped English reads `{detail} is {kind}`, and the parent recorded that shipped shape as the authority for its own draft.

```json
"refusal.dinah.is-a-collection": {
  "text": "{detail} names a whole collection rather than one thing in it",
  "context": "The sentence printed after the refusal name dinah.is-a-collection, when a command that reads or writes one thing is handed a reference naming a whole collection, such as pb-1/comments. {detail} is the reference as the reader typed it. The reference resolves, so the sentence says what it names rather than saying that nothing answers to it."
},
"refusal.dinah.is-a-collection.next-member": {
  "text": "; write {member} for one of them, or `dinah path {detail}` for the collection itself",
  "context": "Spliced onto refusal.dinah.is-a-collection when the collection holds at least one member. {member} is the reference of the first member, which is the collection's own reference followed by /1, and it is an address rather than prose. The second clause offers the one command that takes a collection reference and answers with a filesystem path."
},
"refusal.dinah.is-a-collection.empty": {
  "text": "; it holds nothing, so `dinah path {detail}` for the collection directory is all there is to name",
  "context": "The last member of refusal.dinah.is-a-collection's alternation, printed when the collection holds no members. It carries no condition, because the last member of an alternation is what a reader gets when no branch above it matches. An empty collection is an empty set rather than a mistake, so the sentence offers what can still be named instead of reporting a failure."
}
```

Rendered, on a card carrying one comment:

```
dinah.is-a-collection pb-1/comments names a whole collection rather than one thing in it; write pb-1/comments/1 for one of them, or `dinah path pb-1/comments` for the collection itself
```

and on a card carrying none:

```
dinah.is-a-collection pb-2/comments names a whole collection rather than one thing in it; it holds nothing, so `dinah path pb-2/comments` for the collection directory is all there is to name
```

The machine form carries the count, which stays out of every sentence for the reason dinah-456 section 3.4 gives:

```json
{
  "outcome": "refused",
  "refusal": "dinah.is-a-collection",
  "detail": "pb-1/comments",
  "context": {
    "count": "1",
    "member": "pb-1/comments/1"
  }
}
```

## 4.4 The other seven catalogues

All three keys land in `internal/msg/locales/de.json` and `hi.json` translated, each carrying `source` equal to `msg.Fingerprint` of its English, and in `cs.json`, `id.json`, `es.json`, `fil.json` and `af.json` carrying the English text with `"skeleton": true` and no `source`. That is the workbench document "Translation staleness contract", whose per-key decision-record rule applies because these are new keys rather than a skeleton fill: the implementer records one `decision`-kind checklist item per key per translated catalogue, of the shape the document prescribes, and files no open question about whether the German or the Hindi reads naturally, which the document forbids by name.

Two register notes for the translator, both drawn from the neighbouring entries rather than invented here. The word for the collection follows whatever `attachments.header` and `contents.header` already use in that catalogue for the thing an entity holds. The backticked command spans travel byte-identical, which `TestContractTokensSurviveInBackticks` enforces for `dinah path` because `path` is a name `Commands()` returns.

The glossary sweep that Agent Code Review requires of a diff adding an English key runs on these three, and a term the reading confirms goes into `internal/msg/glossary.json` in the same diff with evidenced forms per language.

**No command's precondition list gains a row.** `verb.checkLists` and `beyondChecks` publish what can go wrong per command, and eleven near-identical rows across eight catalogues would say one thing about the addressing grammar eleven times, in the place a reader looks for one command's own preconditions. The addressing is documented where dinah-456 section 7 puts it, in the references guide's fifteen-row command table with the per-command accept sets folded in, which dinah-457 lands. Reopen this if the refusal ever becomes command-specific, meaning a command refuses a collection for a reason of its own rather than for the grammar's.

# 5. The four commands that accept a collection

## 5.1 `path`

`path` is unchanged except for section 3.2's absent-directory case. It goes on calling `ResolvePath` and never asks the collection question, which is what keeps it the one command that answers a collection reference with a filesystem path.

## 5.2 `show`

`Library.Show` gains a return value rather than folding a listing into its text, so a machine caller reading a collection gets a shape rather than a stream:

```go
func (l *Library) Show(req *Request) (*Detail, *CollectionListing, string, error)
```

with, in `internal/verb/read.go`:

```go
// CollectionListing is what show answers for a reference naming a whole
// collection: the reference the reader typed, the kind of thing the
// collection holds, and one member per live member in creation order.
type CollectionListing struct {
	Ref     string             `json:"ref"`
	Kind    string             `json:"kind"`
	Members []CollectionMember `json:"members"`
}

// CollectionMember is one member of a collection: the address a reader
// types to reach it, and the text show prints for that address on its own.
type CollectionMember struct {
	Ref  string `json:"ref"`
	Text string `json:"text"`
}
```

`Kind` is `Mount.Kind`, so a checklist collection reports `item` however it was spelled. `Text` is `bench.ReadText` of the member's anchor, which is the same read `show <member>` performs today, so a member reads identically whether it is asked for alone or through its collection.

`Ref` on a member is composed by the containment walk's own composer rather than by appending a position to what the reader typed, because dinah-454's rule is that one entity has one printed spelling. So `dinah show pb-1/checklist` prints `pb-1/questions/1` for an open question, which is what `dinah contents pb-1` prints for the same item, and the address it prints resolves.

The composition has one home. The per-member body of `Library.containedChildren` (`internal/verb/tree.go:1017-1029`) becomes:

```go
// memberNodes builds one node per member of a collection, in the order the
// ids arrive, named the way this walk names members. containedChildren runs
// it once per mount of an entity's kind, and the two commands rooted at a
// collection run it once. Both therefore print one spelling per entity, and
// the item counter lives here because an item's reference carries its
// position within its own kind.
func (l *Library) memberNodes(collection string, mount bench.Mount, ids []string, seed string) []TreeNode
```

`containedChildren` calls it per mount and fills each returned node's children as it does today. `Show` calls it once and reads `Ref` off each node.

The `seed` is the reference the members compose against, which is the holder's own reference except for the workbench, whose children compose against the slug. That rule already exists inside `Contents` and is extracted so both callers read one statement of it:

```go
// childSeed is the reference an entity's contained members compose against.
// The workbench is the one entity whose children are seeded with the slug
// rather than with its own printed spelling, per dinah-151 OQ-9.
func (l *Library) childSeed(entity *bench.EntityRef) string
```

`runShow` in `cmd/dinah/commands.go` renders the listing: in the machine formats through `s.emitMachine(listing)`, and in the human format by writing each member's reference on its own line, then that member's text, with one blank line between members. The address line carries no words and needs no catalogue key. An empty collection prints one sentence and exits 0, under a new key:

```json
"show.collection.empty": {
  "text": "{ref} holds nothing.",
  "context": "Printed when `dinah show` is asked for a collection that has no members. No text follows and the call succeeds, because a collection holding nothing is an answer rather than a mistake. It reads as attachments.empty and contents.empty read, since the three commands are answering the same shape of question."
}
```

`readShow` in `internal/mcp/tools.go:652` answers a collection with `wrap(map[string]any{"collection": listing}, readAffordances)`, beside the `text` and `detail` members it already carries.

## 5.3 `contents`

`Library.Contents` asks `ResolveReference` and takes the collection branch when it answers one. The tree it builds is the tree rooted at the collection:

- `Root.Kind` is `verb.KindCollection`, a new constant declared beside `NodeGroup` in `internal/verb/tree.go`: `// KindCollection is the kind a payload gives a collection reference. Like NodeGroup it names no entity of the format, and both the containment tree's root node and an attachments listing carry it where an entity kind would otherwise sit.` Its value is `"collection"`.
- `Root.Ref` is the reference as typed, so the header reads back what the reader wrote.
- `Root.ID` and `Root.Title` are empty, because a collection has neither.
- `Root.Count` is the members plus everything below them, computed by walking rather than by adding up the drawn children, which is what `containedCount` already does per member.
- Children are `memberNodes` over `Members`, each filled by `fillContained` exactly as `containedChildren` fills them.
- The root's rank is `rankOfKind(Holder.Kind)`, so a member of the collection lands at the rank it lands at in a walk rooted at the holder, and `--depth` cuts the two walks identically.

`cmd/dinah/render.go` renders a collection root through two new keys, because `contents.header` and `contents.empty` both open with `{title} ({ref})` and a collection has no title:

```json
"contents.header.collection": {
  "text": "{ref} contains {count} entities.",
  "context": "The sentence above the tree `dinah contents` prints when the reference names a whole collection rather than one entity. A collection has no title of its own, so it is named by the reference alone, and {count} is how many entities sit below it at every depth rather than only the ones drawn."
},
"contents.empty.collection": {
  "text": "{ref} contains nothing.",
  "context": "Printed when `dinah contents` is asked about a collection with no members. No table follows and the call succeeds, for the reason contents.empty gives for an entity."
}
```

Both go into all eight catalogues under section 4.4's rules, as does `show.collection.empty`.

## 5.4 `attachments`

`Library.Attachments` asks `ResolveReference` and branches on what the collection holds:

- A collection whose `Mount.Kind` is `attachment` is the entity's own attachments named the long way, so the listing is built from `Holder` and is byte-identical to what `dinah attachments <holder>` answers, in both formats. `dinah attachments pb-1/attachments` and `dinah attachments pb-1` print the same bytes.
- Any other collection answers an empty listing with `Ref` set to the reference as typed and `Kind` set to `verb.KindCollection`, so `dinah attachments pb-1/comments` prints `pb-1/comments carries no attachments.` and exits 0. That is the answer `Attachments` already gives for an entity of a kind the grammar mounts no attachments on, and its own doc comment gives the reason: a caller walking a tree asks the same question everywhere rather than deciding first whether the question is legal.

# 6. The eleven commands that refuse

Nine of them need no edit of their own. `attach`, `archive`, `delete`, `rename` and the five checklist verbs resolve through `ResolveEntity`, which section 3.1 makes refuse, and every one of them refuses before it takes a lock, writes a journal line or touches an anchor, because each resolves its reference as its first or second step.

Two need an edit, and both take the same shape: ask the collection question ahead of the resolution the command already performs, act on it only when the answer is a collection, and leave every existing refusal alone.

**`instructions`**, in `internal/verb/read.go` after the `instructionColumn` branch and before `ResolveCard`:

```go
	if _, collection, err := l.Bench.ResolveReference(req.Card); err == nil && collection != nil {
		return nil, collection.Refuse()
	}
```

The error is ignored on purpose. A reference naming nothing must go on refusing `dinah.unknown-path` with the whole reference, which is what the existing `ResolveCard` branch answers and what `dinah instructions <nonsense>` prints today.

**`edit`**, in `runEdit` at `cmd/dinah/commands.go:1222`, with the same three lines and the same reason, ahead of its `ResolvePath` call. `edit` is the one refuser whose current behaviour is not a refusal at all: it resolves the directory and hands it to the reader's editor, which dinah-456 section 2.3 records failing with a real one. Handing a directory to a text editor is what it does today, and refusing it is the parent's ruling.

The parent's reason for eleven refusals rather than a general acceptance is the operator's own, and it is not re-argued here: a set-wide write cannot be undone and Dinah has no restore, so `dinah delete pb-1/comments` must not be a spelling.

# 7. Files this card touches

| File | What changes |
|---|---|
| `internal/bench/resolve.go` | `CollectionRef`, `ResolveReference`, `MemberIDs`, the `landing` out-parameter through `descend` and `walkBelowCard`, and the removal of the `Exists` guard on the collection branch |
| `internal/bench/entity.go` | `ResolveEntity` becomes a reading of `ResolveReference` and refuses `dinah.is-a-collection` |
| `internal/bench/storage.go` | nothing; `ListIDs` and `Exists` are read rather than changed |
| `internal/contract/contract.go` | `IsACollection`, and its name in `Introduced` |
| `internal/contract/shape.go` | one `Shape` |
| `internal/msg/locales/*.json` | six new keys in eight catalogues |
| `internal/verb/read.go` | `Show`'s new return, `CollectionListing`, `CollectionMember`, `Attachments`'s branch, `Instructions`'s guard |
| `internal/verb/tree.go` | `KindCollection`, `memberNodes`, `childSeed`, `Contents`'s branch, `containmentMembersOf` reading `MemberIDs` |
| `cmd/dinah/commands.go` | `runShow`'s new arm, `runEdit`'s guard |
| `cmd/dinah/render.go` | the collection renderings for `show` and `contents` |
| `internal/mcp/tools.go` | `readShow`'s new arm |

# 8. Out of scope

- **The references guide.** `internal/guide/guides/references.md` says `show` accepts anything below a card, and its command table has ten rows against the code's fifteen. dinah-456 section 7 gives that whole surface to dinah-457, including the per-command accept sets this card implements, so nothing here edits the guide.
- **`show <member> --json` printing raw anchor text rather than JSON.** The composed branch of `Library.Show` returns text and `runShow` writes it whatever the format, so `dinah show pb-1/comments/1 --json` prints frontmatter today. This card leaves that alone and gives the collection form a proper JSON shape, which is a new surface rather than a change to that one. Section 10 records it as found rather than fixed.
- **Widening `show` to a bare workbench or workstream head.** dinah-456's D-25 records the reading that this card builds section 3.3's collection acceptance and leaves head resolution alone.
- **A precondition row per refusing command**, ruled in section 4.4.
- **`attachments <ref> --deep`.** dinah-456's D-7 borrows the descendant step as a flag, and no section gives it to this card.
- **`dinah attachments pb-1/questions/1` echoing its reference back as `pb-1/checklist/1`.** That is the resolver-composed spelling reaching a header where the narrowed spelling belongs, and it is dinah-454's sweep rather than this card's, which is why section 5.2 fixes the spelling only where this card composes it.

# 9. What would make each criterion fail

Every criterion in the checklist runs a command and reads what came back, so the failure of each is a wrong string rather than an argument. Two shapes are guarded against explicitly, both of which this workstream has produced before.

A criterion asserting that a collection reference is refused passes against code that refuses everything, so AC-2 pins the four acceptances beside the eleven refusals in one sweep and asserts both counts.

A sweep whose subject set can go empty reports success when it finds nothing, so AC-1 asserts the roster is fifteen, AC-2 asserts that fifteen invocations ran, and AC-8 asserts that the sweep saw both refusal names rather than one.

# 10. What this spec was written from

Worked in `C:/dinah-scratch/dinah-455-spec/wt`, a worktree detached at `b825059` created from the operator's checkout, with a binary built there into a directory of the same card's own, and a throwaway workbench under `C:/dinah-scratch/dinah-455-spec/probe` with `DINAH_HOME` pointed at `C:/dinah-scratch/dinah-455-spec/home`. Nothing under the operator's profile or his OneDrive was read or written, and nothing was installed.

The runs behind section 2.2 and section 2.4 were the fifteen invocations of the table, `dinah path pb-2/comments` on a card with no comments, `dinah show pb-1/comments --json`, `dinah show pb-1/comments/1` and `dinah show pb-1/comments/1 --json`, `dinah contents pb-1`, `dinah contents pb-1/comments/1 --json`, `dinah contents pb/cards/1 --json`, `dinah attachments pb-1`, `dinah attachments pb-1/questions/1 --json` and `dinah attach pb-1/questions/1 f.txt --json`. The editor arm used a batch file that echoes its argument, so no editor was launched.

**Found while specifying, and deliberately not fixed here.** `dinah show pb-1/comments/1 --json` prints the member's anchor text rather than JSON, because `Library.Show`'s composed branch returns text and `runShow` writes it in both formats. It is a real defect on a shipped surface, it is not this card's, and no card carries it today.

## Branch

dinah-455-naming-a-collection-tells-the-reader-it-does-not-exist
