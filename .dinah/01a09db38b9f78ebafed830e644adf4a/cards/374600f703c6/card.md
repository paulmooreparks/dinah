---
title: The container migration reads a lock it cannot open as nobody holding it, and then renames
column: b69abf918c42
state: ready
severity: critical
priority: next
workstreams:
  - 58f3e3eb621a
---
The lock check in the container migration reads a directory it cannot open as nobody holding the lock, and the migration then proceeds to rename.

Every other member of this defect family produces a wrong answer on a screen. This one produces a wrong answer and then acts on it, against somebody's workbench, with a rename. That is why it is filed critical while its siblings are major.

The shape is the same one this board has closed in nine places this week: a failure becomes an empty result, and the emptiness then reads as a confident negative. Five were closed in the extension's status bar over five implementation rounds, one in the CLI's refusal reply, one in the code that draws that reply for a reader, one in the container listing function, and one sits on dinah-434 where it cannot be closed where it stands. In every one of those the cost was a lie. Here the cost is a lie plus a destructive operation taken on its strength.

What a spec here owns is what a lock check should do when it cannot tell. Refusing is the obvious answer and probably the right one, since proceeding on an unreadable lock is the failure mode this card exists to remove, but a migration that refuses too readily is its own problem and the answer should be argued rather than assumed. Whatever it lands on, the rule this board has been enforcing all week holds: a failure must not be able to present itself as a confident negative.

The platform trap that applies to the whole family applies here too, and dinah-433 settled how to handle it. Asking whether a read error means the path does not exist gives the same answer for a plain file sitting where a directory belongs as for a path nobody created, so existence is established with a separate documented check rather than by classifying the read's own error.

Found by the code reviewer on dinah-433 while checking whether that card closed the family. It does not, and this is the more serious of the two remaining.

Related: dinah-433 fixed the listing function, dinah-438 covers its sibling with thirty-one callers, dinah-434 covers the case that needs a different surface, dinah-437 covers the lift, and dinah-432 closed the reply-side door.

## Specification

## What this card is

The whole defect family travels on this card, not the lock check alone. An Intake sweep folded dinah-437 (`resumableLift`) and dinah-440 (`ListIDs`) into it with their full text, and the survey below adds three more sightings the sweep did not have. The unifying shape is one sentence: a directory-read failure becomes an empty collection, and the emptiness then reads as a confident negative.

Nine earlier sightings of this shape were dealt with one route at a time, and the family came back every time. dinah-437's consolidation, quoted in the consolidation comment on this card, enumerates those nine: five in the extension's status bar over five implementation rounds, one in the CLI's refusal reply, one in the code that draws that reply for a reader, one in the container listing function dinah-433 fixed, and one on dinah-434 that could not be closed where it stood. Eight of the nine were patched and the ninth is still open. The figure is derived once, here.

This card does not patch routes. It changes what a collection read returns and disposes of every directory read in the tree by name, so that every caller in the tree meets the question at compile time. The whole-tree guard that would stop a tenth member ever being written is no longer on this card: the operator split it onto dinah-486 on 2026-09-11, and section 8 says what that leaves open here.

The work behind this spec was done against `origin/main` at `65a80ad8b3b525e62af2262ad6d3203876ea6c7a` ("dinah-471: a card answers to its raw identifier, and the guide now says so"), which is the commit every round of this card and every Agent Design Review of it has read. Round five worked in the worktree `C:/dinah-scratch/dinah-439-spec5/wt`, round four in `dinah-439-spec4/wt`, and rounds one to three in the sibling directories `dinah-439-spec`, `dinah-439-spec2` and `dinah-439-spec3`. Round five's own commit on the card's branch is `e8c0eb0`, which changed the companion document and no Go file. Round six worked in `C:/dinah-scratch/dinah-439-spec6/wt` against that same commit, and its own commit `05f3689` removes that document from this branch and changes no Go file either. Trunk resolved to `main` from the Spec column's effective directives rather than defaulted.

Rounds four and five derived their claims from the tree rather than from shapes somebody had pictured, and that method is why the code facts below can be relied on. The shapes at `container.go:693`, `check.go:699`, `storage.go:299` and `vocabulary.go:328`, and the failure branch of all five sites the survey blesses, were each re-checked against the syntax trees rather than carried forward from an earlier round. Everything else those two rounds produced was about the guard, and it travels with the guard to dinah-486.

## The distinction that has to survive

A collection directory that is ABSENT legitimately means empty in this format, and dinah-455 has just made that load-bearing. Its acceptance run recorded `dinah path pb-2/comments` exiting 0 against a directory `ls` reports as absent, and `dinah show pb-2/comments` printing `pb-2/comments holds nothing.` A collection the workbench declares now resolves whether or not anything has been written into it yet.

A collection directory that CANNOT BE READ is a failure, and it must not arrive as that same answer.

Those two cases are not separable by classifying `os.ReadDir`'s own error, and this is the platform trap dinah-433 settled. It was verified again on this machine on 2026-09-11, from `C:/dinah-scratch/dinah-439-spec2/probe`, on Windows:

```
plainfile  ReadDir n=0 err=readdir ...\plainfile: The system cannot find the path specified. IsNotExist(readdir)=true | Stat err=<nil> IsNotExist(stat)=false
missing    ReadDir n=0 err=open ...\missing: The system cannot find the file specified.      IsNotExist(readdir)=true | Stat err=...  IsNotExist(stat)=true
empty      ReadDir n=0 err=<nil>                                                             IsNotExist(readdir)=false| Stat err=<nil> IsNotExist(stat)=false
```

`os.IsNotExist` applied to `os.ReadDir`'s error answers true for a plain file sitting where a directory belongs and true for a path nobody created. `os.Stat` separates them, and `os.Stat`'s documented contract is what separates them, so nothing here rests on undocumented behaviour. Existence is established by a second `os.Stat` made only after the read has already failed.

## The survey, and how it was produced

The set of directory reads is produced by a command over the whole tree rather than by reading the files this card was filed about.

```
grep -rn "os\.ReadDir(" --include=*.go . | grep -v "_test.go"
grep -rn "filepath\.WalkDir(\|fs\.WalkDir(" --include=*.go . | grep -v "_test.go"
```

The first returns twelve sites and the second returns three, all fifteen inside `internal/bench`. Nothing outside that package reads a directory on disk in the shipped binary. Two further calls spell `ReadDir` and read no disk at all: `guides.ReadDir("guides")` at `internal/guide/guide.go:43` and `locales.ReadDir("locales")` at `internal/msg/msg.go:100` are `embed.FS` methods over data compiled into the binary. They are named here so that a reader counting this repository's directory reads does not count them, and because dinah-486's guard matches the qualified `os.ReadDir` rather than any selector called `ReadDir`, which is what keeps both of them outside its sweep. Every one of the fifteen is dispositioned below.

| # | Site | Today | Ruling |
|---|---|---|---|
| 1 | `bench.go:880` `walkFor` | refuses `contract.UnknownRoot` | leave, already honest |
| 2 | `bench.go:1039` | refuses `contract.UnknownRoot` | leave, already honest |
| 3 | `check.go:699` `checkAttachmentFilename` | `if err != nil || len(entries) == 0 { continue }` | FIX, section 6 |
| 4 | `container.go:186` | refuses `contract.UnknownRoot` | leave, already honest |
| 5 | `container.go:485` `resumableLift` | `return "", nil` | FIX, section 4 (was dinah-437) |
| 6 | `container.go:693` `heldLocks` | `if entries, err := os.ReadDir(root); err == nil` | FIX, section 3 (this card's own) |
| 7 | `entity.go:250` | conflates, then refuses `UnknownPath` | leave, see Out of scope |
| 8 | `finish.go:60` `interruptions` | `if err != nil { continue }` | FIX, section 5 |
| 9 | `resolve.go:768` | conflates, then refuses `UnknownPath` | leave, see Out of scope |
| 10 | `storage.go:157` `ListIDs` | `if err != nil { return nil }` | FIX, section 2 (was dinah-440) |
| 11 | `storage.go:299` `ListWorkbenchIDs` | correct since dinah-433 | leave, it is the model |
| 12 | `vocabulary.go:328` `listIdentifiers` | classifies `os.ReadDir`'s own error | FIX, section 7 |
| 13 | `container.go:705` `heldLocks` member walk | `if err != nil || entry.IsDir() { return nil }` | FIX, section 3 |
| 14 | `container.go:750` `memberDigest` walk | `if walkErr != nil { return walkErr }` | leave, already honest |
| 15 | `container.go:799` `mirrorTree` walk | `if walkErr != nil { return walkErr }` | leave, already honest |

Sites 13, 14 and 15 sit in one file, and the two that report their walk error are the neighbours of the one that discards it. That makes site 13 a slip rather than a design, and it is a second swallow inside `heldLocks` that the consolidation comment did not have.

## 1. The ruling: the fix is at the helper

Patching callers is how this family survived the nine sightings named at the top of this spec, so the helper changes and every caller meets the question at compile time. `ListIDs` becomes:

```go
func ListIDs(collection string) ([]string, error)
```

`heldLocks` becomes `func heldLocks(root string) ([]string, error)`, `resumableLift` keeps its signature and stops discarding, and `listIdentifiers` keeps its signature and stops classifying.

The alternative shapes were weighed and refused. A second function beside `ListIDs` that checks its error leaves the old name in place, and the next writer reaches for the shorter one, which is the route-patching shape wearing a new name. A wrapper type whose zero value is not an empty collection buys nothing that two return values do not, because Go cannot force a method call any more than it forces an error check.

What the type change does not close on its own is the blank identifier. A caller writing `ids, _ := ListIDs(dir)` restores the defect with the compiler's blessing, and dinah-433 left that door open on `ListWorkbenchIDs`. This card leaves it open too. dinah-486 is where it shuts for both.

### The shared reader

One function performs the read and the existence discrimination, and the three collection listings call it:

```go
// readCollection lists a directory, separating a collection this format has
// not written yet from one that exists and will not read. The existence
// question is settled by a second os.Stat made only after the read has
// failed, because os.ReadDir's own error reports a plain file sitting where a
// directory belongs as not-existing on at least one supported platform, which
// is the same answer it gives for a path nobody created. os.Stat's documented
// contract separates the two on every platform alike.
func readCollection(dir string) ([]os.DirEntry, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if _, statErr := os.Stat(dir); os.IsNotExist(statErr) {
			return nil, nil
		}
		return nil, err
	}
	return entries, nil
}
```

It lands in `internal/bench/storage.go` beside the two listings that use it. `ListWorkbenchIDs` is rewritten to call it, which changes none of its behaviour and removes the second copy of the platform reasoning. The long comment now sitting on `ListWorkbenchIDs` about why the existence check runs a second call moves onto `readCollection`, where it governs all three readers rather than one.

## 2. ListIDs, and its 82 call sites

`ListIDs` returns `(nil, nil)` for a collection that names nothing on disk, which is the absent-means-empty rule the format depends on and dinah-455 made visible. It returns `(nil, err)` whenever something is at that path and could not be listed.

```go
// ListIDs returns the identifiers of a collection directory, sorted
// ascending, ignoring anything that is not a hex directory. An absent
// collection is an empty one, which is the absent-means-empty rule, and it is
// answered with a nil slice and a nil error. A collection that is there and
// will not read is answered with the error, so a caller receiving an empty
// list and a nil error has been told the collection really was read.
func ListIDs(collection string) ([]string, error) {
	entries, err := readCollection(collection)
	if err != nil {
		return nil, err
	}
	var ids []string
	for _, entry := range entries {
		if !entry.IsDir() || !IsID(entry.Name()) {
			continue
		}
		ids = append(ids, entry.Name())
	}
	return ids, nil
}
```

### The call-site count is 82, and dinah-440's figure of thirty-one is one subset of it

The number was produced by tracing rather than by reading, and four independent methods agree on it.

**Method A, the compiler.** A probe parameter was added to `ListIDs` in the worktree and `go test -gcflags=all=-e -run NONE ./...` was run repeatedly, patching the reported sites between rounds so the next package could type-check. The `-gcflags=all=-e` matters: without it the compiler caps errors per package and a plain `go vet ./...` reported eleven. Four rounds, then a fifth reporting nothing.

| Round | Sites | Where |
|---|---|---|
| 1 | 31 | `internal/bench`, production only |
| 2 | 8 | `internal/bench` tests (4) and `internal/verb` production (4) |
| 3 | 43 | `cmd/dinah` tests (31) and `internal/verb` tests (12) |
| 4 | 0 | the set is closed |

That totals 82, of which 35 are production and 47 are test.

**Method B, textual.** `grep -ro "\bListIDs(" --include=*.go . | wc -l` returns 84. Subtract the declaration at `internal/bench/storage.go:156` and the one prose mention inside a comment at `internal/verb/read.go:1147`, and 82 remains.

**Method C, the language server.** `gopls references -d internal/bench/storage.go:156:6` returns 83 lines. The declaration is among them, confirmed by grepping that output for `storage.go:156`, and the prose mention is not, confirmed by grepping it for `read.go:1147`. Eighty-three minus the declaration is 82.

**Method D, the syntax tree.** Round two re-derived the figure independently of the three above, by parsing every `.go` file under the worktree with `go/parser` and counting each `CallExpr` whose callee is an `Ident` or a `SelectorExpr` named `ListIDs`. Over 268 files it answers 35 production and 47 test, totalling 82, split `internal/bench` 31 and `internal/verb` 4 on the production side, which reproduces method A's rounds exactly. The first Agent Design Review ran the same method and reported the same figures. The second added a fifth method, the type checker itself: widening `ListIDs` to `([]string, error)` and driving `go test -gcflags=all=-e -run NONE ./...` to exhaustion gives rounds of 31, 8 and 43 and then a round reporting nothing, which is 82 with the same 35 and 47 split. Five methods and three authors now agree on 82, and the same run classifies the argument-position subset for free, 34 sites reporting "multiple-value in single-value context" and `entity.go:133` reporting "too many arguments for len".

dinah-440's thirty-one is method A's first round exactly, which is the production call sites inside `internal/bench` and nothing else. The figure was not wrong so much as narrower than the word "callers" suggests.

**What would make 82 wrong.** All four methods see only the identifier `ListIDs` written literally at a call. A call made through a function value, through reflection, or through a generated file not on disk at this commit would be counted by none of them, and a grep run over a tree with a build tag excluding a file would miss what the compiler would see. Two counter-checks are cheap and both were run: `grep -rn "ListIDs[^(]" --include=*.go .` finds only the prose mention, so no function value is taken anywhere in the tree, and method A is the compiler itself, which sees every file the default build includes. A file behind a non-default build tag would remain outside all four, and no such file exists in this module today. Recompute the figure with method A before starting the sweep rather than trusting this one, since another card may land a call site first.

### The 35 production call sites and what each does with a failure

They do not all want the same thing, and flattening them would be its own defect. Four dispositions cover them. **P** means the enclosing function already returns an error and now returns this one. **W** means the enclosing function returns no error today and gains one, with its own callers followed. **F** means the site sits inside the check walk and its error reaches `Bench.Check`'s existing error return. **R** means the site precedes a write and the write is refused.

| Site | Enclosing function | Disposition |
|---|---|---|
| `bench.go:1615` | `openWithVocabulary(...) (*Bench, error)` | P |
| `bench.go:2053` | `cardsWith(...) ([]*Card, error)` | P |
| `bench.go:2079` | `(*Bench).NextNumber() int` | W to `(int, error)`, 1 caller, R |
| `changes.go:59,63,67` | `(*Bench).WatchedEntities() (live, archive []Watched)` | W, 1 caller |
| `check.go:258` | `(*Bench).Check() ([]Finding, error)` | F |
| `check.go:313,318` | `checkAttachmentsWithoutAMount() []Finding` | W to `([]Finding, error)`, 1 caller, F |
| `check.go:343` | `mountlessAttachmentsBelow(dir, kind string) []Finding` | W, 3 callers, F |
| `check.go:659` | `checkOrdinals(cardDir string) []Finding` | W, 1 caller, F |
| `check.go:687` | `checkAttachmentFilename(cardDir string) []Finding` | W, 1 caller, F |
| `entity.go:70` | `Comments(cardDir string) ([]*Comment, error)` | P |
| `entity.go:100` | `Attachments(cardDir string) ([]*Attachment, error)` | P |
| `entity.go:133` | `CountAttachments(dir string) int` | W to `(int, error)`, 3 callers |
| `entity.go:452` | `(*Bench).ColumnOccupied(id, ref string) error` | P |
| `entity.go:835` | `refBelowHead(...) string` | W, 1 caller |
| `entity.go:920` | `Items(cardDir string) ([]*Item, error)` | P |
| `entity.go:1033` | `itemsWhere(...) []*Item` | W, 3 callers |
| `finish.go:132` | `(*Bench).siblingCollections() []string` | W, 1 caller |
| `finish.go:146` | `(*Bench).collectionsBelow(dir, kind string) []string` | W, 2 callers |
| `ordinal.go:69` | `nextOrdinal(collection, anchor string) int` | W to `(int, error)`, 5 callers, R |
| `ordinal.go:285` | `(*Bench).backfillCollection(...) (int, []Finding)` | W |
| `ordinal.go:360` | `(*Bench).BackfillOrdinals(...) (int, []Finding, error)` | P |
| `ordinal.go:423` | `ordinalCollections(cardDir string) []ordinalCollection` | W, 2 callers |
| `resolve.go:651` | `MemberIDs(collection string, mount Mount) []string` | W, 4 callers |
| `resolve.go:987` | `(*Bench).ArchivedColumnByRef(ref string) *Column` | W, 2 callers |
| `witness.go:67` | `(*Bench).WriteWitnesses(...) ([]string, []Finding, error)` | P |
| `workstream.go:171` | `workstreamsIn(collection string) []*Workstream` | W, 3 callers |
| `workstream.go:278` | `(*Bench).WorkstreamReferenced(id, ref string) error` | P |
| `workstream.go:427` | `(*Bench).checkWorkstreams() []Finding` | W, 1 caller, F |
| `verb/read.go:1174` | `memberPosition(dir, anchor string) int` | W, 3 callers |
| `verb/search.go:168` | `(*Library).Search(req *Request) (*SearchResults, error)` | P |
| `verb/tree.go:1029` | `(*Library).itemRefOf(entity *bench.EntityRef) string` | W, 1 caller |
| `verb/tree.go:1204` | `containedCount(dir, kind string) int` | W, 4 callers |

Ten sites are P, where the enclosing function already returns an error and the edit only has to check this one: `bench.go:1615`, `bench.go:2053`, `entity.go:70`, `entity.go:100`, `entity.go:452`, `entity.go:920`, `ordinal.go:360`, `witness.go:67`, `workstream.go:278` and `verb/search.go:168`. Twenty-one enclosing functions are W, spread over twenty-four sites, the extra sites being `changes.go:59,63,67` in one function and `check.go:313,318` in another. Ten P sites plus twenty-four W sites plus the one F-only site at `check.go:258` is 35, which is the arithmetic that makes the table add up. The cascade behind the W functions is shallow: the caller counts in that column were produced with `gopls references` against each declaration, and the largest is five. The widened set reaches no further than `internal/verb`, where every command entry point already returns an error to `cmd/dinah`'s `reportError`.

Every one of the 35 production sites is an expression-context call rather than a plain assignment, so every one of them needs the call hoisted to a temporary and the error checked before the enclosing expression can be written. There is no subset here that takes a simpler edit than the rest. Twenty-two sit in `for _, id := range ListIDs(...)`, one returns the result of a call wrapping it at `resolve.go:651`, one is `len(ListIDs(...))` at `entity.go:133`, and eleven sit in an argument position where a two-value call may not stand beside other arguments: nine pass `ListIDs` to `SortByOrdinal` (`entity.go:70`, `entity.go:100`, `entity.go:835`, `entity.go:920`, `resolve.go:651`, `workstream.go:171`, `workstream.go:427`, `verb/read.go:1174`, `verb/tree.go:1029`), one is the `len` call above, and one is `orderedByJournal(ListIDs(collection), order)` at `ordinal.go:285`. The argument-position set was produced by method D's walk, which flags a call whose parent node is an argument of another call, rather than by a regular expression, because a nested call is exactly what a regular expression over one line mis-reads. The type checker confirms the wider claim directly: widening the signature makes 34 of the 35 report "multiple-value in single-value context" and the thirty-fifth report "too many arguments for len". The first Agent Design Review's figure of nine is the `SortByOrdinal` subset of the eleven.

Widening `nextOrdinal` alongside `ListIDs` reaches two further expression-context sites that the 35 do not contain, `workstream.go:348` and `workstream.go:371`, where the call is a composite-literal field written `Ordinal: nextOrdinal(collection, WorkstreamAnchor)`. A composite-literal field takes a single value, so both need the same hoist. The W row for `ordinal.go:69` records five callers, which is the number `gopls references` gives; these two are among them, and they are named here because a hoist out of a composite literal is the one form an implementer working from the 35-row table would not see coming.

The 47 test call sites take the mechanical edit dinah-433 used, which is `ids, err := ListIDs(...)` followed by a `t.Fatalf` with the existing assertion untouched.

### The two sites where a wrong answer authorises a write

These are the sites marked R, and they are why the type change is worth 82 edits rather than a comment.

`NextNumber` (`internal/bench/bench.go:2076`) computes the number a newly filed card carries as one past the highest in use, and its own doc comment says a number is never reused. An unreadable cards collection makes it answer 1. Two cards sharing a number make `dinah show pb-1` ambiguous, and a number is the durable half of a card reference.

`nextOrdinal` (`internal/bench/ordinal.go:67`) computes the ordinal a new comment, item or attachment is stamped with. An unreadable collection makes it answer 1, colliding with the member that already holds ordinal 1, and the ordinal is what `pb-1/comments/1` addresses.

Both are widened to `(int, error)` and both refuse the write. Section 10 says what each can actually reach on disk today, which is a different question from what each computes.

## 3. heldLocks, the card's own instance

`heldLocks` (`internal/bench/container.go:691`) answers which locks are standing on a workbench about to be migrated. It carries two swallows rather than one.

```go
	if entries, err := os.ReadDir(root); err == nil {        // line 693, the root read
	...
		filepath.WalkDir(member, func(path string, entry fs.DirEntry, err error) error {
			if err != nil || entry.IsDir() {                 // line 706, the member walk
				return nil
			}
```

Both report that nobody holds the workbench for a directory nobody could read. The signature becomes:

```go
func heldLocks(root string) ([]string, error)
```

The root read goes through `readCollection`, so a root that is genuinely absent still answers no locks with no error, and a root that exists and will not read answers the error. The walk's callback returns its own error instead of discarding it, which is what `memberDigest` and `mirrorTree` already do forty-five and ninety-four lines below it in the same file. That error cannot be provoked by the plain-file fixture, because `filepath.WalkDir` over a path holding a plain file calls its callback once with a nil error rather than reporting a failure. Measured on 2026-09-11: `calls=1 sawErr=<nil> walkErr=<nil>`. The absent path is no route either, even though the same measurement shows `WalkDir` over a path that does not exist calling its callback once with a non-nil error, because `memberPaths` (`container.go:93`) filters every member through `Exists` before the walk starts, so reaching that case wants a deletion racing the walk rather than a fixture. So the member walk is reached through a test seam rather than through a fixture on disk: `memberWalk = filepath.WalkDir` becomes a package variable beside `containerRename`, `readAnchorContent` and `statPath` at `container.go:111`, whose comment already explains that seam as how this file makes a filesystem condition reachable from a test, and the test substitutes a walk that yields one callback carrying a non-nil error.

### The three callers all refuse, and that is argued rather than assumed

`remintInPlace` (`container.go:325`) renames the workbench directory. `liftIntoContainer` (`container.go:371`) moves the workbench's members into a container. `finishContained` (`container.go:607`) stamps the format through a read-modify-write of the whole anchor. All three currently write `if held := heldLocks(path); len(held) > 0 { return "", contract.Refuse(contract.Locked, lockedEntity(held[0])) }`.

Each becomes:

```go
	held, err := heldLocks(path)
	if err != nil {
		return "", err
	}
	if len(held) > 0 {
		return "", contract.Refuse(contract.Locked, lockedEntity(held[0]))
	}
```

Refusing is right here, and the objection to it is real enough to answer rather than wave at. A migration that refuses too readily is its own problem, and this one already refuses on a lock held anywhere in the tree, which is wider than a held lock would be. What makes refusal correct is the asymmetry between the two mistakes. Refusing a migration that could have proceeded costs the operator a second run once they have fixed a permission or removed a file sitting where a directory belongs, and `heldLocks`' own doc comment already says this is a repair an operator runs deliberately. Proceeding on a lock nobody could see costs a rename performed underneath a live writer, whose workbench members move out from under it while it writes. The migration cannot take a lock it can carry across the rename of the directory holding that lock, which the same doc comment states, so there is nothing weaker than refusal available to reach for.

The error is not wrapped in a `contract.Refusal`. dinah-433's D-4 settled that for `completedLift`: `liftIntoContainer` already lets `os.MkdirAll` and `os.Rename` failures reach `cmd/dinah`'s `reportError` as plain errors, and making this one call behave differently from its neighbours gives the caller nothing to act on differently. `contract.UnreadableContainer`, which dinah-433 minted, names a `.dinah` container rather than a workbench root, so it is the wrong name here, and no new refusal name is minted by this card.

## 4. resumableLift

`resumableLift` (`container.go:484`) answers which directory an interrupted lift left behind, and today `if err != nil { return "", nil }` makes a container it cannot read answer that there is none. `liftIntoContainer` then mints a fresh target and lifts into it, stranding whatever the interrupted run had already moved.

The read goes through `readCollection` and the error is returned. The signature already carries an error, so the one caller at `container.go:391` needs no change beyond the propagation it already performs. An absent container still answers `("", nil)`, which is the first-run case and the reason `liftIntoContainer` calls `os.MkdirAll` immediately afterwards.

## 5. interruptions, in finish.go

`(*Bench).interruptions` (`internal/bench/finish.go:57`) sweeps the live half of every collection for a sibling file, which is what an interrupted structural act leaves standing. The `if err != nil { continue }` at line 60 means a collection it cannot read contributes no interruptions, so `dinah finish` reports nothing to finish and exits successfully while a rename sits half-done.

This is the same class as the lock check and it is a sighting the consolidation sweep did not have. The read goes through `readCollection`, `interruptions` gains an error return, and `siblingCollections` and `collectionsBelow` are widened alongside it because they feed it.

## 6. checkAttachmentFilename's payload read

`check.go:699` reads an attachment's payload directory with `if err != nil || len(entries) == 0 { continue }`, so a payload that will not read produces no finding. It goes through `readCollection` and the error propagates into `Bench.Check`'s error return along with the rest of the check walk's sites.

## 7. listIdentifiers, in the vocabulary migration

`listIdentifiers` (`internal/bench/vocabulary.go:327`) returns its error, so it is not a swallow. It classifies `os.ReadDir`'s own error with `os.IsNotExist`, which is the platform trap, and `ListWorkbenchIDs`' doc comment names this function by file and line as carrying that gap.

It matters more than a latent gap because of where it sits. `migrateColumnDirectories` guards with `Exists(from)`, which succeeds for a plain file, then calls `listIdentifiers`, which answers `(nil, nil)` for that same plain file, then calls `os.Rename(from, filepath.Join(parent, ColumnsDir))`. A file named `states`, which is the value of `PreVocabularyDir` at `vocabulary.go:16`, is renamed to `columns` and the migration reports success. The pre-vocabulary spellings are `PreVocabularyDir = "states"` and `PreVocabularyAnchor = "state.md"`, and the string `flow` appears nowhere in this tree, so a fixture planting a file of that name would exercise nothing. That is a second migration acting on a read it never performed.

The body becomes a call to `readCollection`, and the doc comment loses its sentence about absent collections not being an error, since `readCollection` now carries that reasoning. `ListWorkbenchIDs`' comment loses the paragraph warning against this function, because the warning has been discharged.

## 8. The repository-wide guard is not on this card

The guard that would hold every directory read in the tree to answering its own failure left this card on 2026-09-11 and is now dinah-486. Five design reviews ran on this card, and every one of them found a real defect in that guard's prototype. Every defect was a branch that condemned correct Go with no instance anywhere in the tree, which is exactly why no run over the tree could see it, and a static analysis over the whole repository turns out to be its own project with its own ways of being wrong. The fix in sections 1 to 7 has stood unchallenged since round two, and keeping the guard here held that fix behind the analysis.

So this card promises nothing about a tenth member of the family. What it does is change what a collection read returns, so that all 82 `ListIDs` callers and all three `heldLocks` callers meet the question at compile time, and dispose of every one of the fifteen directory reads in the tree by name. What it leaves open is the blank identifier. A caller writing `ids, _ := ListIDs(dir)` restores the defect with the compiler's blessing, nothing here stops that, and the door has stood open on `ListWorkbenchIDs` since dinah-433 in exactly the same way. A new directory read written somewhere else in the tree is equally unguarded. dinah-486 closes both, and it is filed with its criteria and its decision record rather than left as a note.

The enumeration of every shape this repository writes a directory read in, together with the two programs that produced it, travels with the guard. It was committed on this branch as `docs/specs/dinah-439-directory-read-shapes.md`, `05f3689` removes it, and it is re-landed on dinah-486's branch as `docs/specs/dinah-486-directory-read-shapes.md` with a header naming the three defects round five's review left standing in it.

dinah-486's figures are measured over the tree with this card already landed, since this card changes which reads exist and where. That makes this card its predecessor in the ordinary way rather than a blocker on it: dinah-486 can be specified at any time, and only its numbers wait.

## 9. The check surface stays with dinah-462

dinah-462 owns what a reader is told when a sweep cannot read what it was asked to read, what exit code that carries, and what the machine surface says. It sits in Intake and it is the operator's.

This card answers none of those. The six `ListIDs` sites in `check.go`, plus `check.go:699` and `checkWorkstreams`, propagate their error into `Bench.Check`'s existing `([]Finding, error)` return, and `Check` returns the findings it has already gathered alongside the non-nil error, so the partial report survives at the library boundary. Both of its callers, `internal/verb/read.go:1452` and `internal/verb/read.go:1464`, discard the findings on a non-nil error today, so nothing a reader sees changes until dinah-462 decides what a partial report should say. No exit code is changed by decision of this card, and the one finding key this card does mint, in section 10, is an ordinary defect finding rather than anything about a sweep that could not look. What changes here is that the check surface can no longer be silent, which is the floor dinah-462 then builds its presentation on.

The implementer posts a comment on dinah-462 carrying the fifteen-site survey table from this spec and the two caller line numbers above, rather than only the statement that its baseline moved. dinah-462's own description opens by asking which sweeps swallow a read error; this card has answered that in full, and letting the answer sit only here makes dinah-462 pay to derive it again.

## 10. A workbench already damaged by this defect

A fixed migration will not find damage that has already happened, so the four classes are named and each one is answered.

**A rename performed under a live writer.** `remintInPlace` renamed the workbench directory while a lock stood. A lock in this format is a file inside the directory it protects, so the lock moved with the directory and the writer releases at a path that no longer holds a workbench, leaving the lock file standing. `dinah check` already reports that as `check.interrupted-act` through `interruptions`, and after section 5 it reports it from collections it previously skipped.

**A lift performed under a live writer.** `liftIntoContainer` moved members out from under a writer whose next write lands at the old path, so the workbench is split across two paths. `dinah check` already reports `check.entity-at-both-paths` for the entity-level form of that, and the workbench-level form shows as a bare workbench with a partial member set, which `dinah check` reports as `check.bare-workbench`.

**A colliding ordinal.** `nextOrdinal` answered 1 against a collection it could not list, and two members now carry ordinal 1. `dinah check` already reports `check.ordinal-duplicate`, and `BackfillOrdinals` already repairs it.

**A colliding card number.** `NextNumber` answered 1 against a cards collection it could not list. Nothing detects that today. The damage is reachable, and the route is the archived half rather than the live one. `NextNumber` (`bench.go:2076`) reads both `b.CardsRoot()` and `b.ArchivedCardsRoot()`, while `internal/verb/beyond.go:67` follows it with `bench.ClaimID(l.Bench.CardsRoot(), ...)`, whose `os.MkdirAll` and `os.Mkdir` touch the live collection alone. A live collection that will not read does abort the filing on that `os.MkdirAll` or that `os.Mkdir`. An archived collection that will not read aborts nothing: `NextNumber` silently omits every archived number from its maximum, the claim succeeds, and the new card is stamped with a number an archived card already carries. The Agent Design Review ran it on 2026-09-11 with the live collection readable and a plain file at `archive/cards`, where `NextNumber` answered 1 and `ClaimID` returned an identifier and a nil error. The second Agent Design Review re-ran the reproduction independently rather than reading it, with the same result, and also ran the converse case: with the live collection unreadable instead, `ClaimID` fails at its own `os.MkdirAll`, which is the asymmetry this paragraph turns on.

Round one concluded from a narrower reading that this class could not happen, and that conclusion was false. What follows is what ships instead.

### The duplicate-number detector ships here

Both halves ship on this card, and the operator ruled it so on 2026-09-11 rather than leaving it to be argued. The mint is the card: a swallowed read error producing a confident wrong answer that then writes is what this card is named for, so `NextNumber` must stop handing out a number computed from a directory it could not read. The detector is the honest consequence of round one having said no repair was needed on a premise that turned out false, because somebody's workbench may already carry duplicate numbers and nothing tells them. Filing that as a successor would put a card reading "your data may already be wrong" into a queue the operator is trying to drain, which is the outcome the workbench's filing rule exists to prevent.

It stays small. It names the duplicates and it rewrites nothing, so no repair command and no migration ships, and `dinah check` gains one finding rather than a tool.

`checkCardNumbers` lands in `internal/bench/check.go` and `Bench.Check` calls it beside the other collection checks. It walks both halves the way `NextNumber` does, groups the cards by number, and reports every member of a group larger than one.

```go
// checkCardNumbers reports every card sharing its number with another card,
// across both halves of the collection. A number is the durable half of a
// card reference, so two cards holding one number leaves a reference by
// number with two answers.
//
// Every card in a colliding group is reported rather than only the second
// one met, which is where this parts company with checkOrdinals. The live
// half is read before the archived one and the archived card is the one that
// held the number first, so reporting the second sighting would name the
// innocent card and leave the interloper unreported.
func (b *Bench) checkCardNumbers() ([]Finding, error) {
	var findings []Finding
	byNumber := map[int][]string{}
	for _, root := range []string{b.CardsRoot(), b.ArchivedCardsRoot()} {
		ids, err := ListIDs(root)
		if err != nil {
			return nil, err
		}
		for _, id := range ids {
			card, err := LoadCard(root, id)
			if err != nil {
				// An archived card whose anchor will not load is
				// reported rather than skipped, because the number
				// it holds is exactly the number that might be
				// colliding, and a detector going quiet on the half
				// that makes the damage reachable hides the defect
				// it exists to report. The live half is reported by
				// Bench.Check's own card walk, so reporting it here
				// too would name it twice. The path is the card's
				// directory, which is the spelling that walk uses.
				if root == b.ArchivedCardsRoot() {
					findings = append(findings, Finding{Path: filepath.Join(root, id), Key: unreadableCardFinding(err), Detail: id})
				}
				continue
			}
			byNumber[card.Number] = append(byNumber[card.Number], filepath.Join(root, id, CardAnchor))
		}
	}
	numbers := make([]int, 0, len(byNumber))
	for number := range byNumber {
		numbers = append(numbers, number)
	}
	sort.Ints(numbers)
	for _, number := range numbers {
		paths := byNumber[number]
		if len(paths) < 2 {
			continue
		}
		sort.Strings(paths)
		for _, path := range paths {
			findings = append(findings, Finding{Path: path, Key: FindingCardNumberDuplicate, Detail: strconv.Itoa(number)})
		}
	}
	return findings, nil
}
```

Both loops are ordered before anything is appended, because a finding list whose order comes from a map iteration is a fixture that disagrees with itself between runs. The block above was type-checked rather than read, against stubs for `Finding`, `Card`, `Bench`, `ListIDs`, `LoadCard` and `unreadableCardFinding`, because an earlier draft of it declared `findings` below the loop that appends to it and would not have compiled.

A card whose anchor will not load is reported rather than skipped, and that is a correction to an earlier draft of this section rather than an elaboration of it. The draft skipped it and recorded the cost in Out of scope, which is the card's own defect wearing the card's own uniform: an archived card whose anchor will not load is absent from the grouping, so a number it holds can collide with a live card's and `dinah check` answers clean, and the archived half is the half that makes the damage reachable in the first place. The report reuses `unreadableCardFinding`, which is the existing classifier and mints no key, so an unloadable archived anchor surfaces as `FindingMissingAnchor` or as the vocabulary finding the refusal names, which is the key the live half's report carries too. `Path` names the card's directory rather than its anchor, matching what the live walk writes at `check.go:268` and `check.go:273`, so one key does not print two spellings depending on which half found the card. The duplicate finding keeps the anchor path, for the reason "The finding key" gives. It fires for the archived root alone, because `Bench.Check`'s own card walk already reports the live half and a second report would name the same card twice.

The collection-level read failure is answered separately and more sharply. `ListIDs` now returns its error and `checkCardNumbers` returns that error, so an archived collection that will not read makes `Check` answer with an error rather than with a clean bill, which is the disposition every other check-walk site takes in section 9. The two failures are different sizes and get different dispositions on purpose: a collection nobody can list defeats the whole sweep and is an error, while one card nobody can load leaves the sweep able to finish and is a finding.

### The finding key

One key is minted: `check.card-number-duplicate`, declared as `FindingCardNumberDuplicate` in the constant block at `internal/bench/check.go:26`, alongside the `FindingOrdinalDuplicate` it is modelled on. `Path` is the card's anchor, which is what `render.go:884` prints in parentheses after the sentence, and `Detail` is the number in decimal, so two findings of one collision print one number against two openable paths.

The English sentence is `{detail} is a card number two cards carry, so a reference by number answers with either of them`, with the context line `A check finding: two cards hold one number, which leaves the durable half of a card reference ambiguous.` It writes no indefinite article in front of its placeholder, which `TestNoEnglishSentenceTakesAnArticleBeforeAPlaceholder` forbids.

All eight catalogues carry the key. All eight carry all 937 keys today, and the per-card coverage test is what makes a ninth catalogue arriving a failure rather than a miss. `af`, `cs`, `es`, `fil` and `id` carry it as a skeleton holding the English text, which is what those five do for every key they ship. `de` and `hi` carry a translation that differs from the English and a `source` of `msg.Fingerprint(<the English text>)`, which is the arrangement `check.ordinal-duplicate` already has in both of them and which `TestATranslationIsNotEnglishUnderAnotherTag` and `TestATranslationTracksItsEnglishSource` hold it to.

That key is the boundary with dinah-462 rather than a breach of it. dinah-462 owns what the surface says when a sweep could not look, together with the exit code and the machine surface for that state. A duplicate card number is an ordinary defect the workbench really has, reported through machinery that already exists, and it says nothing about a sweep that failed.

No repair command and no migration ships on this card. Three damage classes are already reported by `dinah check`, and the fourth is reported by the finding this card mints.

## 11. The fixture, and why it is not vacuous

Almost every test below plants an unreadable directory the same way: a plain file is written at the path where a directory belongs. It needs no permission manipulation, it behaves identically on Windows, Linux and macOS, and dinah-433's AC-3 already verified it against this repository.

What it cannot do is make a directory that exists fail to read, so it serves every `os.ReadDir` site on this card and none of the three `WalkDir` sites. Those are reached through the `memberWalk` seam described in section 3, and a criterion covering a walk says which of the two it uses.

The fixture names a constant rather than a spelling. Round one planted a file called `flow` at a path the code spells `states`, so the migration skipped it, renamed nothing, and every assertion in the criterion passed against the unfixed code. That is worse than no test, and the reason it survived a round is that a fixture naming the wrong thing looks exactly like a fixture naming the right one. So each criterion below cites the declared constant whose value it plants, and the implementer builds the path from the constant rather than from the literal, which makes a rename of the constant a compile error instead of a silent pass.

A fixture that quietly produced an empty directory instead would make every criterion on this card vacuous, so each test proves its own fixture before it exercises anything:

```go
if _, err := os.ReadDir(unreadable); err == nil {
	t.Fatalf("the fixture at %s reads cleanly, so this test proves nothing", unreadable)
}
```

That assertion runs first and it fails the test rather than skipping it.

The companion case runs in the same test function every time. A criterion asserting that a failure is reported passes against code that reports failure for everything, so each test also asserts the succeeding cases: a real, readable, empty directory answers empty with no error, and an absent directory answers empty with no error.

## Out of scope

The repository-wide guard is the largest thing out of scope, and section 8 says where it went and what it leaves open. Nothing on this card promises that a directory read written tomorrow will be caught.

`entity.go:250` and `resolve.go:768` both read an attachment's payload with `if err != nil || len(entries) == 0` and then refuse with `contract.UnknownPath`. A read failure does not become a confident negative at either site, so neither belongs to this card's class. What they do is name the wrong cause, telling a reader a path is unknown when it exists and would not read, and that is dinah-455's class rather than this one. Both are left as found and recorded here so that the next reader does not re-derive the question.

The health of an archived card's anchor is out of scope with one exception this card does ship. `Bench.Check` walks the live half alone today, reporting a missing or unreadable anchor there through `FindingMissingAnchor` and `unreadableCardFinding`, and section 10's number sweep is the first thing on this card to read the archived half at all. The exception is the sweep's own path: an archived card the number sweep cannot load is reported through that same classifier, because the alternative is a detector that goes silent on the half that makes a duplicate reachable. What stays out of scope is every other archived-half check, which is the card checks `checkCard` performs on the live half and which would widen a card whose diagnosis is about read failures rather than about anchors. That gap is recorded here rather than filed, under the workbench rule that a finding nobody would pick up on its own is a note.

## Branch

dinah-439-the-container-migration-reads-a-lock-it-cannot-open-as-nobody-holding-it-and-then-renames

## Retired checklist items (no Dinah state for these yet)

These crossed from the Andoneer board in a state Dinah has no word for. dinah-472 is the card that adds it; until then they live here.

- **acceptance_criterion** (Andoneer AC-10, state `obsolete`)
  - Text: TestEveryDirectoryReadAnswersItsFailure exists in internal/profile/guards_test.go and enforces the five rules of section 8 over an examined set with two halves. It walks every non-test .go file under the module root, and it collects its examined set by walking every call node rather than every statement, so a call nested inside another call is in the set; a collector walking statements reaches 29 of the second half's 40 sites at the merge base and reports success, which is the defect this clause exists to stop. The first half of the examined set is every call whose callee is the qualified os.ReadDir, filepath.WalkDir or fs.WalkDir. The second half is every call to the four collection readers readCollection, ListIDs, ListWorkbenchIDs and heldLocks, matched bare inside internal/bench and qualified as bench.ListIDs and bench.ListWorkbenchIDs outside it; without that half a caller writing ids, _ := ListIDs(dir) restores the defect and no rule sees it, because the callee it names is not a standard-library read. For each call in the set the guard asserts five things. First, that the error result is answered, in one of four forms: bound to a named identifier the following statements read, or bound in the init of an if whose own condition reads it, which is the ordinary if entries, err := os.ReadDir(dir); err != nil idiom, or returned directly as the enclosing function's own error result, or passed to a call that consumes it, so a bare call statement discarding the result fails as squarely as _ does. A rule 1 demanding a following statement condemns the if-init idiom, which is correct Go and the commonest form of a directory read in this language, so the fourth form is not optional. Second, that the enclosing function declares an error among its results or names an entry in the exemption table. Third, that no os.IsNotExist, errors.Is(..., fs.ErrNotExist) or os.IsPermission is applied to a value os.ReadDir produced. Fourth, that the error parameter of any function literal passed to filepath.WalkDir or fs.WalkDir is returned, or bound and read, rather than discarded. Fifth, that the enclosing function answers no ordinary absent-or-empty result anywhere on the read's failure path. That fifth rule is mechanical in three parts, and all three are asserted rather than left to the implementer's reading. The failure path is the body of an if whose condition is err != nil, the body of an if whose condition is any boolean combination containing err != nil, the fallthrough past an if whose init performs the read and whose condition is err == nil with no else branch, or the else branch of that same form where one exists; a guard recognising only the first of those four blesses container.go:693 and check.go:699, which are two of the reads this card fixes. Rule 5 asks nothing of a call whose callee declares no error result, because there is then no failure for the enclosing function to answer and rules 1 and 2 carry the whole obligation; a rule 5 without that narrowing fires on the three heldLocks callers at container.go:325, 371 and 607, which are correct code at the merge base, and a guard firing against correct code is refused here. The question is then asked of every exit reachable on the failure path rather than of the branch's last statement alone: a return must set a non-nil error result, and a nested return counts, and a continue or a break fails the rule because the loop carries on and the function goes on to report success. Run with that reading over BOTH halves of the examined set at 65a80ad, which is 55 calls comprising 15 in the first half and 40 in the second, the rule draws exactly seven findings, against resumableLift, interruptions, checkAttachmentFilename, heldLocks, ListIDs, listIdentifiers and ListWorkbenchIDs, and none against bench.go:880, bench.go:1039, container.go:186, entity.go:250 or resolve.go:768. The criterion pins both directions and the population: running the guard's own sweep function over the tree at the merge base must name all seven of those functions AND report 15 first-half and 40 second-half calls examined, and running it over the finished branch must name none of them, so a guard that does not catch the defect in the card that created it fails rather than shipping, and a run whose subject set silently narrowed reports the narrowing rather than a clean seven. Matching is on the callee's name, so guides.ReadDir at internal/guide/guide.go:43 and locales.ReadDir at internal/msg/msg.go:100, which are embed.FS methods over compiled-in data, are outside the sweep rather than inside it with an exemption; the guard asserts that neither is in the examined set. It logs the number of files scanned, the number exempted, and the number of call sites examined in each half separately, and it holds each half against a declared floor constant rather than against zero: minFirstHalfSites and minSecondHalfSites, each carrying in its comment the commit it was measured at and the command that produced it, with failure naming the half, the floor and the actual count. A non-zero check is what this criterion refuses, because 29 is not zero. Both floors are set from a run on the FINISHED branch and not from the merge-base figures, and the implementer records both runs in the pull request: the first half must come in smaller than 15, because seven of its twelve os.ReadDir calls are routed through readCollection which contributes one of its own, and a first half that did not shrink is a consolidation that did not happen; the second half must come in larger than 40, because readCollection is the fourth collection reader and every function now reading through it is a new second-half site. A run contradicting either direction fails this criterion rather than being adopted as the new number. It further asserts that the second half matched at least one bare spelling and at least one qualified spelling, so a matcher that handles only one form reports a number rather than passing. The exemption table carries exactly one entry when this card lands, readCollection against rule 5, with the reason that it is the single place in the binary where the existence question is settled; the guard fails on an exemption whose reason is empty and on an exemption matching no call site the sweep found, and the companion criterion below proves the entry suppresses something rather than merely sitting there. The counts it logs at merge time are recorded in the pull request and are not written into any other file.
  - Note: Obsolete on dinah-439 from 2026-09-11: the repository-wide guard was split onto dinah-486 by the operator's ruling, so this criterion is no longer this card's to satisfy. It is carried there as dinah-486 AC-1, with round five's F1 and F2 corrections applied, and the text is not repeated here because two copies of one criterion drift.
- **acceptance_criterion** (Andoneer AC-11, state `obsolete`)
  - Text: The guard is armed by a companion that proves it reddens, the companion covers all five rules rather than one, it pins rule 5's narrowing in both directions, and it proves the exemption table suppresses something. Test: TestGuardCatchesASwallowedDirectoryRead in internal/profile/guards_test.go, following the shape of TestGuardCatchesAHandRolledRow at guards_test.go:2656 in the same file. It runs the guard's own sweep function against twelve synthetic sources held as string literals in the test. Eight are defective: one discarding the error into a blank identifier, one whose enclosing function returns no error and names no exemption, one applying os.IsNotExist to os.ReadDir's own error, one whose WalkDir callback answers return nil to a non-nil error while binding and reading the call's own result, one in resumableLift's shape, which binds the read's error, tests it, and then returns a nil error from a function that declares one, one in heldLocks' shape, written if entries, err := os.ReadDir(root); err == nil with no else branch so that the failure path is the fallthrough, one in checkAttachmentFilename's shape, written if err != nil || len(entries) == 0 { continue }, and one that is a call to a SECOND-HALF reader rather than a standard-library read: a call to a collection reader declared with ([]string, error) whose result is bound in an if init under a condition testing the returned slice rather than the error, so the fallthrough answers empty while the error is never read. That eighth source is the arming case for the second half of the examined set, and without it nothing in this companion exercises a second-half call at all, which is how a divergence between the rule and its prototype survived a review round. Sources five, six and seven are the arming cases for rule 5 over the first half and the reason the companion cannot be a four-source test: a four-rule build of the guard passes the resumableLift source cleanly, and a rule 5 worded over the err != nil branch alone passes the other two cleanly. Those three must be written inside a signature that declares an error, matching the signatures sections 3 and 6 give heldLocks and checkAttachmentFilename, because otherwise rule 2 catches them and the companion proves nothing about rule 5. Four sources draw no finding and pin the accepting side. One does everything correctly. One returns filepath.WalkDir's result directly as the enclosing function's own error result, which is mirrorTree's form at container.go:799 and which a rule 1 written to demand a named binding would wrongly condemn. One reads the error in its own if init's condition, written if entries, err := os.ReadDir(dir); err != nil { return nil, err }, which is the ordinary Go idiom and which a rule 1 demanding a following statement would wrongly condemn; this card rewrites all four of the tree's current if-init reads, so the guard would ship with no live instance in front of it and an unmeasured false-positive set without this source. The fourth is the narrowing pin, and it is the byte-identical twin of defective source eight with one difference: its callee is declared returning []string alone rather than ([]string, error). It must draw NO finding, because rule 5 asks nothing of a call whose callee declares no error. The pair is what makes the narrowing testable in both directions at once, since a guard ignoring the narrowing reddens on the twin and a guard over-narrowing goes quiet on source eight, and the two sources differ only in the callee's signature so no other explanation of a divergence is available. The test asserts that each of the eight defective sources produces a finding and names which rule caught it, that none of the four accepting sources produces any finding, so a sweep that flagged every input would fail, and that the number of sources examined is twelve, so a source silently skipped reports a number rather than passing. The same test proves the exemption table is live rather than decorative: it runs the sweep once over a synthetic readCollection with the exemption table populated and asserts no finding, and once with the table emptied and asserts a rule 5 finding naming readCollection, so an exemption entry that suppresses nothing cannot ship. Without that second run the entry still passes the guard's own staleness check, which asks only that the exempted function contains a call site the sweep found. The synthetic sources are parsed rather than written to disk, so this companion touches no file in the repository.
  - Note: Obsolete on dinah-439 from 2026-09-11: the guard's arming companion went with the guard to dinah-486, where it is AC-2. It grew there from twelve synthetic sources to sixteen, nine defective and seven accepting, because round five's F1 and F2 named three accepting shapes and one defective shape that twelve could not pin.
- **decision** (Andoneer D-4, state `obsolete`)
  - Text: [SPLIT, 2026-09-11. This decision left dinah-439 with the guard it describes and now stands as dinah-486's D-1, where it is pending on round five's falsification. It is obsolete here rather than resolved, and nothing on dinah-439 rests on it. The note below is the preserved round-two-through-round-five history, untouched, and dinah-486 points at it rather than copying it.] A whole-tree AST guard, TestEveryDirectoryReadAnswersItsFailure in internal/profile/guards_test.go, holds five rules over an examined set with two halves: every qualified os.ReadDir, filepath.WalkDir and fs.WalkDir call, and every call to the four collection readers readCollection, ListIDs, ListWorkbenchIDs and heldLocks. The set is collected by walking every call node rather than every statement, because a collector walking statements misses a call nested inside another call and would reach 29 of the second half's 40 sites while reporting success. The fifth rule is that the enclosing function answers no ordinary absent-or-empty result anywhere on the read's failure path, where the failure path covers the err != nil branch, a branch whose condition is any boolean combination containing err != nil, the fallthrough past an if ...; err == nil that has no else, and the else branch of one that has an else, and where the question is asked of every exit reachable on that path rather than of the branch's last statement, a continue or a break failing it as squarely as a return setting a nil error. Rule 5 asks nothing of a call whose callee declares no error result, because there is then no failure to answer and rules 1 and 2 carry the obligation. Rule 1 accepts four answering forms rather than three, the fourth being an error bound in the init of an if whose own condition reads it, which is the ordinary Go idiom. The four failure-path forms, the three exit forms and the four answering forms are taken from an enumeration of every directory read in the tree produced from the syntax trees, over the whole examined set rather than over part of it, and both the enumerating program and the rule prototype report the population they examined. The guard holds each half of the examined set against a declared floor constant set from a run on the finished branch, rather than against zero. The exemption table is an inclusion set whose entries each carry a reason, it holds exactly one entry, readCollection against rule 5, and a companion proves that emptying it reddens the run.
  - Note: Re-resolved at Spec round five, on a measurement over the population the decision itself defines rather than over part of it. Round four's falsification is upheld and answered by taking resolution A from review comment 7460: the rule is narrowed and the program corrected, and the recorded count stays at seven. WHAT ROUND FIVE DID, and every number below came from a run rather than a reading. First, both programs now state their population. Program 1 examines 93 non-test .go files under the module root, excluding .git, node_modules and editors, and matches a call population WIDER than the guard's examined set on purpose, since its job is to find shapes the name set might have to grow to cover. Program 2 examines the same 93 files and its call population is the guard's examined set exactly, both halves. Each prints its own breakdown, so a run whose subject set has drifted says so in its own output instead of producing a clean-looking number. Second, program 1's blind spot was fixed rather than recorded. It reached a call only where the call was a statement's operative expression, so a call nested inside another call was invisible: `for _, id := range SortByOrdinal(collection, CommentAnchor, ListIDs(collection))` recorded SortByOrdinal and never descended. It now walks every call node. At 65a80ad it reports 60 sites in twelve groups rather than 49 in eleven, and the eleven recovered are exactly the argument-position ListIDs sites the review named: entity.go:70, 100, 133, 835, 920, ordinal.go:285, resolve.go:651, workstream.go:171, 427, verb/read.go:1174 and verb/tree.go:1029. Of the 60, 55 are the guard's examined set (15 first half, 40 second half) and 5 are outside it (three os.Open calls opening files, two embed.FS ReadDir methods). Third, the divergence F1 found was resolved by narrowing the rule, which is resolution A. Program 2's subject set is now both halves, and its third if-init arm skips rather than reports when the callee declares no error. Run that way over the whole examined set at 65a80ad it draws SEVEN findings, not ten, and they are the same seven: checkAttachmentFilename (check.go:701), resumableLift (container.go:487), heldLocks (container.go:717), interruptions (finish.go:62), ListIDs (storage.go:159), ListWorkbenchIDs (storage.go:302) and listIdentifiers (vocabulary.go:331). The run also prints EXAMINED HALF 1: 15, EXAMINED HALF 2: 40, SKIPPED, CALLEE DECLARES NO ERROR: 38. WHY A RATHER THAN B, which the review left to this decision. Resolution B keeps the arm that reports whenever an if-init condition tests neither direction. That arm fires on a call whose callee has no error to give, which is correct code: remintInPlace at container.go:325 has nothing to answer until heldLocks is widened, and this workbench refuses a guard that fires against correct code. B would also have pinned the figure ten, which describes the merge base alone, since all three of those sites disappear the moment heldLocks declares an error. The narrowing is not a permanent carve-out either: ListIDs and heldLocks account for all 38 exemptions, this card gives both an error result, and rule 5 reaches every second-half call from the moment it lands. Fourth, the guard's size check became a real floor. AC-10 asserted each half's count was non-zero, which 29 satisfies, so a guard built the way round four's program was built would have examined 29 of 40 second-half sites and passed. The guard now holds each half against a declared constant, minFirstHalfSites and minSecondHalfSites, each carrying the commit it was measured at. Both are set from a run on the FINISHED branch rather than from 15 and 40, and the direction of each change is itself asserted: the first half must SHRINK, because seven of its twelve os.ReadDir calls route through readCollection which contributes one of its own, so a first half that did not fall is a consolidation that did not happen; the second half must GROW, because readCollection is a fourth collection reader with callers. Copying 15 and 40 into the code would have reddened the guard on the very branch that fixes the card. Fifth, rule 1 gained its fourth answering form. It listed three exhaustively and omitted the ordinary `if entries, err := os.ReadDir(dir); err != nil` idiom, so a future correct read was condemned. This card rewrites all four of the tree's current if-init reads, so the guard would have shipped with no live instance in front of it and an unmeasured false-positive set. AC-11 now carries a correct source in that form. Sixth, AC-11 grew from nine synthetic sources to twelve, because none of the nine exercised a second-half call at all, which is how this divergence survived a round. Eight are defective, including one call to a second-half reader declaring ([]string, error) bound in an if init under a non-error condition. Four draw no finding, including the narrowing pin: the byte-identical twin of that source with the callee declaring []string alone, which must draw nothing. The pair makes the narrowing testable in both directions, since a guard ignoring it reddens on the twin and a guard over-narrowing goes quiet on its partner, and the two differ only in the callee's signature. Reproducibility. Both programs were re-extracted from the committed document, rebuilt and re-run, and their output matched byte for byte. Worked at 65a80ad in C:/dinah-scratch/dinah-439-spec5/wt; the branch commit carrying the corrected programs is e8c0eb0. --- The round-four falsification, preserved --- Returned to pending at Agent Design Review round four, on a falsification produced by running the decision's own program over the examined set the decision itself defines. This decision's first paragraph defines the examined set as two halves: every qualified os.ReadDir, filepath.WalkDir and fs.WalkDir call, and every call to the four collection readers. Its round-four note then claims that corrected rule 5 "draws exactly seven findings and nothing else" and that "the false-positive set across 93 files is empty". Both claims rest on one run, and that run covered the first half alone. Program 2 in docs/specs/dinah-439-directory-read-shapes.md declares its subject set on one line: var reads = map[string]bool{"os.ReadDir": true, "filepath.WalkDir": true, "fs.WalkDir": true}. I extracted that program, added the four collection readers in both spellings to that one line, rebuilt it, and ran the otherwise unmodified rule over the same tree at 65a80ad. It draws ten findings rather than seven. The three extra are the three heldLocks callers, remintInPlace (container.go:325), liftIntoContainer (container.go:371) and finishContained (container.go:607), each reported as "the read sits in an if-init whose condition does not test its error". Those three come from a branch of the program that the rule as written does not authorise. The rule names four failure-path forms, the fourth being the fallthrough past an if whose init performs the read and whose condition is err == nil with no else. At container.go:325 the condition is len(held) > 0, which is neither err != nil nor err == nil, so by the rule's own definition there is no failure path. The program's if-init handling carries a third arm that reports whenever the condition tests neither direction, and over the first half that arm can never fire, because the tree's only first-half if-init read is container.go:693 and its condition is err == nil. So the program the rule is derived from is not the rule the spec states, and the divergence was invisible because of the subject set the run used. Review comment 7460 carries two resolutions with the wording for each: narrow rule 5 to calls whose callee declares an error result and correct the program's third arm, keeping the recorded seven; or keep the wider rule and state the count as ten with the three named. Either way AC-11 needs an arming source for a second-half call, because none of its nine sources exercises one. --- The round-four resolution, preserved --- Resolved at Spec round four, on a measurement rather than on a reading. Rule 5 was implemented against an enumeration of the tree and run over the tree, and the run's output is what the spec and the criteria now assert. The round-three falsification note is preserved beneath. WHAT ROUND FOUR DID. First, the enumeration. A program parses every non-test .go file under the module root with go/parser, matches every call to os.ReadDir, filepath.WalkDir, fs.WalkDir, filepath.Walk, fs.ReadDir, ioutil.ReadDir, os.Open, filepath.Glob and os.DirFS, every call to a selector named ReadDir, Readdir, Readdirnames, Glob, WalkDir or Walk on any receiver, and every call to the four collection readers, then classifies each site by how the error result is taken and by the shape of the statement that tests it. Run at 65a80ad in C:/dinah-scratch/dinah-439-spec4/wt it reports 49 sites collapsing to seven shapes, and section 8 carries the table. The four failure-path forms rule 5 is now worded over are that enumeration's answer rather than a guess: a simple err != nil branch (13 sites), a compound condition containing it (4 sites, three with || and one with &&), the positive if ...; err == nil form whose failure path is the fallthrough (container.go:693), and the else branch of that form, which nothing in the tree writes today but which the rule covers because it is the same read one keystroke away. Second, the rule. Corrected rule 5 was implemented and run over the whole tree. It draws exactly seven findings and nothing else: checkAttachmentFilename (check.go:701, continue on a compound-condition failure path), resumableLift (container.go:487, return "", nil), heldLocks (container.go:717, return held reached by falling through the positive form), interruptions (finish.go:62, continue), ListIDs (storage.go:159, return nil), ListWorkbenchIDs (storage.go:302, a nested return nil, nil), and listIdentifiers (vocabulary.go:331, a nested return nil, nil). That is every read this card fixes, including the two round three showed a four-rule and a branch-worded guard blessing, and it includes the site the card is named for. It draws nothing on bench.go:880, bench.go:1039, container.go:186, entity.go:250 or resolve.go:768, so the false-positive set across 93 files is empty. Both falsifications are therefore answered by the same run. Rule 5 catches seven of seven rather than three of five, and the every-return reading is pinned in the spec, in AC-10 and in this decision's text, which is what makes the readCollection exemption live: rule 5 fires on ListWorkbenchIDs at storage.go:299, and that function's body is byte-for-byte readCollection's specified shape. Third, the exemption. AC-11 now requires the companion to run the sweep twice over a synthetic readCollection, once with the table populated and once with it emptied, asserting a rule 5 finding in the second run. An exemption nobody has watched suppress anything is the same defect as a rule nobody has watched fire, and the guard's own staleness check cannot see the difference, because it asks only whether the exempted function contains a call site the sweep found. Fourth, what the guard still cannot see, recorded rather than claimed away. Section 8 now names two gaps rather than one. A directory read reached by a callee name outside the examined set is invisible, which was already recorded and which the wider grep confirms is an empty set today. A directory read written in a failure-path shape the enumeration does not contain is also invisible, and the two shapes that would do it are nameable: a switch on the read's error, and a dispatch through errors.Is into a branch answering empty. Neither is in the tree at 65a80ad. Closing the second would want a control-flow graph rather than statement forms, which is a larger instrument than an empty set of instances justifies. --- The round-three note, preserved --- Returned to pending at Agent Design Review round three, on a falsification produced by building all five rules and running them, not by reading them. Two claims in this decision are false, and both are claims about what the guard catches rather than about the code. First, rule 5 as worded governs "the branch taken when a read's error is non-nil", and two of the five sightings this card fixes have no such branch. heldLocks' own root read at container.go:693 is written `if entries, err := os.ReadDir(root); err == nil {`, where the failure path is the fallthrough rather than a branch. checkAttachmentFilename's payload read at check.go:699 is written `if err != nil || len(entries) == 0 { continue }`, whose condition merely contains the test. I ran both shapes through a five-rule prototype inside the signatures this card gives their enclosing functions, ([]string, error) and ([]Finding, error), and both come back with no finding at all. Today each draws exactly one finding, rule 2, and this card removes the reason for it, which is the same failure round two found for interruptions and ListIDs, now in the site this card is named for. Rule 5 catches three of the five sightings, not five. Second, "The only shipping function it fires on is readCollection" was never watched, and readCollection's shape is in the tree today to watch. ListWorkbenchIDs at storage.go:299 has exactly the specified body: the failure branch tests os.Stat, returns nil, nil inside a nested if, and falls through to `return nil, err`. Reading rule 5 as the branch's terminating statement, which is what "the enclosing function returns a non-nil error" most directly says, the rule draws nothing on it and nothing on a synthetic readCollection, so the one exemption entry would suppress nothing and AC-10's "exactly one entry" would ship a decoration. Reading it as every return reachable on the failure path, it does fire on readCollection, it also fires on listIdentifiers and ListWorkbenchIDs, which this card rewrites, and it still fires on none of the five blessed sites. The spec has to choose the second reading, and say so. A third observation, which weakens the evidence rather than the conclusion: entity.go:250 and resolve.go:768 are two of the five blessed sites, and their condition is also compound, so rule 5 never recognises their failure branch. They do answer with contract.Refuse(contract.UnknownPath, payload), so the conclusion that the rule fires on none of the five holds; the measurement supporting it could not have seen two of them either way. What holds, verified by the same run rather than accepted. The examined set's second half reaches exactly 40 sites: 35 ListIDs production sites split 31 bare and 4 qualified as bench.ListIDs, three heldLocks callers and two ListWorkbenchIDs callers, with no second declaration of any of the four names anywhere in the tree. Rule 1's three answering forms bless mirrorTree and catch the bare call at container.go:705. Rule 4 catches round one's callback source. Rule 3 catches listIdentifiers and nothing else. --- The round-two resolution, preserved inside it --- The original note: The type change forces every existing caller to face the question once, and it leaves the blank identifier open. A caller writing ids, _ := ListIDs(dir) restores the defect with the compiler's blessing, and dinah-433 left that door open on ListWorkbenchIDs for the same reason. The guard is what makes this the last card in the family rather than the twelfth of many. Its exemption table is an inclusion set because the extension paid five rounds to learn that direction: VACANCY_REFUSALS in editors/vscode/src/tree.ts:1344 was an exclusion list until round five and was admitting three published refusal names that said the opposite of emptiness. Its model in every structural respect is TestNoLockIsCreatedOutsideTheOneAcquirer at internal/profile/guards_test.go:57, including the scanned==0 refusal and the assertion that the exemption still matches something real. What round two falsified: the four rules govern whether a read's failure is AVAILABLE to the function that made the read, and none of them governs whether the function then ANSWERS it, so resumableLift drew no finding, and interruptions and ListIDs tripped rule 2 only because their enclosing functions returned no error, which this card gives them. Rule 5 is therefore load-bearing rather than additive. The examined set gained its second half in the same round, naming the four collection readers, which is what makes the claim about dinah-433's escape hatch true rather than deleting it. Rule 1 gained its three answering forms in the same round, because as first worded it condemned mirrorTree at container.go:799.
