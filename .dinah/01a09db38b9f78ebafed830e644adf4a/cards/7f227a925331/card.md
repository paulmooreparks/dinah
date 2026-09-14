---
title: The card number moves out of the card and into a registry, so two clones cannot mint the same one silently
column: b69abf918c42
state: ready
severity: major
priority: next
tier: frontier
workstreams:
  - 994787601ae6
links:
  - kind: relates_to
    to: d8ae20807731
---
## The problem, stated from the code

A card's number, the `99` in `dinah-99`, is stored in the card's own frontmatter and minted by `NextNumber()`, which scans both halves of the card collection and returns one past the highest it sees. There is no shared counter anywhere.

So when two clones of a workbench each file a card from the same base, both scan their own tree, both find 98, and both write `number: 99` into `cards/<their own random hex>/card.md`. The paths differ, so git merges both without a conflict, and the workbench ends up with two cards called dinah-99. Nothing detects it. `dinah check` has no finding for it, the profile's uniqueness statement (`CORE-CARD-1`) constrains the hex identifier and not the number, and resolution takes the first match (dinah-487 makes that refuse; this card makes it not happen).

The design document's merge story does not reason about this case. `docs/design/format.md` says the layout "merges well by construction, since entity directories keep concurrent card work in disjoint files", and that is true of the same card edited twice. It is not true of two new cards claiming one number, and the reason it is not true is the subject of this card.

## The design, settled with the operator on 2026-09-11

Paul and the parent session worked this through before filing, and these are decisions rather than options. The spec should build on them, not reopen them.

**The hex is the identity. The number is a label.** The 12-hex id is random, collision-free without coordination, and already the name Dinah answers to (dinah-471). The number is a convenience for humans. Every stored reference carries the hex; a number is never an address that internal code depends on.

**Uniqueness and sequence are different requirements, and only one needs an arbiter.** Uniqueness can be guaranteed offline, by claiming visibly so that a collision becomes a git conflict. Sequence, meaning the next card is one more than the last with no gaps, cannot be guaranteed without something that knows what the last number was. The file format guarantees uniqueness. Gaps are normal. Dense, authoritative numbering is what a live arbiter provides, and that arbiter belongs in the hosted product rather than in the format.

**The number moves out of the card into one registry file.** `cards/numbers.txt` or a name the spec mints, one line per card, `<number> <hex>`, appended on creation. Two clones from the same base both append `99 <hex>` at the end of the same file, so git conflicts on it, which is the property the design wants. Whoever merges second renumbers. Line order records allocation order in the content itself, which repair needs and which git history alone would not give a workbench that is not in git.

Per-number files were weighed and declined. Both schemes conflict in exactly the same cases, because the scan-based allocator means every concurrent creation from one base is a collision. The single file records order in its content and is one thing to read; per-number files buy only free local atomicity, and a lock covers that.

**The registry is canonical and `number` leaves the frontmatter.** Keeping both would be two stores for one fact, which this workbench has paid for repeatedly. A card file stays self-identifying through the hex in its path. Renumbering becomes a one-line edit in one file.

**Local concurrency is a lock**, not a merge problem. Two processes on one machine filing cards serialise on the registry.

## What the card delivers

- The registry file, its grammar, and its place in the layout.
- Creation appends to it under a lock; `Ref()` reads the number from it rather than from the card.
- `number` removed from card frontmatter, with a storage format bump and a migration that builds the registry from existing frontmatter, in journal order where two cards would otherwise tie.
- A `check` finding for a number claimed twice, and for a number in the registry whose hex names no card, and for a card with no registry line.
- A repair that renumbers the later claimant, taking "later" from line order.
- The profile amended: a statement that no two live-or-archived cards in one workbench share a number, and the changelog's false claim corrected (it says the interchange form carries `number`; the interchange form carries no cards at all).
- `format.md`'s merge story extended to cover two new cards, not only one card edited twice.
- Guides and the quick start updated wherever they describe the number as living in the card.

## What it does not do

It does not make numbers dense or sequential across clones. That needs an arbiter and the arbiter is the hosted product's to provide. It does not change the hex, its minting, or the path layout. It does not touch entity ordinals on comments, attachments and checklist items, which are scoped to one card and already checked by `check.ordinal-duplicate`.

## The cost to accept deliberately

Without an arbiter, a contributor's card can land with a different number than it carried on their branch, and references in their commit messages and branch name go stale. That is the price of working offline, and it is the price GitHub avoids by being the authority. The spec should say so plainly rather than promising otherwise. The hex is the reference that survives, which is why it is the identity.

## Sequencing

dinah-487 ships first and independently: it makes resolution refuse an ambiguous number, which is correct under every storage scheme. This card then makes the ambiguity impossible to create silently.

This is a storage format change and a profile amendment, so it takes the build lane with its Test stage, and it must not be worked concurrently with any other card touching card frontmatter or the compat fixtures.

## Specification

## What this spec is written against

Every line number below was read at `933f0d5` on 2026-09-11, in a worktree at `C:/dinah-scratch/dinah-488-spec/wt`, which is `origin/main`. Trunk moved one commit after round 5 was written, from `c1ae4a9` to `933f0d5` (dinah-424), and that commit touches `editors/vscode` and adds an `os.Executable` seam to `internal/verb/read.go` without introducing a free-reader reference, so every citation here was re-established against `933f0d5` rather than carried over. One citation was wrong and is corrected in this round: `carryToDoing` stands at `cmd/dinah/main_test.go:248` rather than at `:244`, and it stood there at `c1ae4a9` as well.

Round 4 of this spec was written with the binary built and run rather than read. The seven commands AC-13 drives were run against a throwaway workbench under `C:/dinah-scratch` with `DINAH_HOME` pointed there, in both the state a correct build produces and the state a call site reading through a free card reader produces, and the expected sets the criterion names are the sets those runs produced. A prototype of the guard's two rules was built and run over the tree and over the eleven planted attack shapes the corpus then stood at, including the one that defeated round 4's guard. The settled design is untouched, and everything rounds 3 and 4 verified is left exactly as it stood: the conformance-constant argument, the revision arithmetic, the five revision sites, AC-14's boundary at `:1566`, the call-site sweep, and the AST case list.

Round 5 was written the same way, and it drew on the reviewer's runs as well as the author's rather than on either alone. The prototype of the guard's two rules was repaired in two places and re-run over the tree and over every planted attack shape written against it so far, including three the reviewer wrote. The section 4 entry this card publishes was run against `go test ./internal/profile` in a copy of the tree, in the wording that ships and in the wording round 4 proposed, because the round 4 wording used a word the profile's own excluded list bars from that section and no amount of reading it catches that. Everything rounds 3 and 4 verified is left exactly as it stood, and nothing about the design changed.

Round 6 was the same again, and its subject was the guard alone. The reviewer found that rule 2c followed a card through an assignment, an alias, and an `append` but not into a composite literal, wrote the escape three ways, ran one of them end to end with a caller in `internal/verb` reading a live unstamped card's rendered reference, patched the prototype with one predicate, and costed the predicate at zero against the tree. This spec's author rebuilt that patched prototype and re-ran it over `933f0d5` and over all twelve planted directories, then wrote five further attacks against the repaired rule and found that all five defeated it. Four more clauses close those five, and each clause was measured against the tree before it was written down here. Every figure this round states came from a run made in this session at `933f0d5`: thirty-two rule 1 references in nineteen files, one rule 2 hit at `internal/bench/check.go:288`, nothing under rule 2 once that one line is converted, and all twenty-three planted shapes reported. The profile half of this card did not move in round 6, and the three profile runs, the revision arithmetic, the section 4 entry, the converted-line scan, and the design all stand exactly as round 5 left them.

`dinah-439` (pull request 251) has landed. It is on trunk as `c1ae4a9`, and everything the previous round of this spec attributed to a branch is now ordinary trunk code: `checkCardNumbers` at `internal/bench/check.go:811`, the finding key `check.card-number-duplicate` at `:48`, a `NextNumber()` that answers an error rather than computing a number from a collection it could not read (`internal/bench/bench.go:2122`), and a `ListIDs` answering `([]string, error)`. This card adapts that finding rather than writing a second one, per D-4.

`dinah-487` is the one card this spec still assumes rather than reads. It makes resolution by number refuse when more than one card answers, with the refusal name `dinah.ambiguous-card`, and it ships first. Its refusal stays after this card: a merged registry can carry two lines claiming one number, and the resolver has to refuse that state rather than pick a line. Nothing else here depends on code that is not on trunk.

## The registry file

**Name and place.** The registry is `card-numbers.txt`, sitting at the workbench root beside `workbench.md`, `journal.ndjson`, `.gitignore`, and `.gitattributes`. Declare it in `internal/bench/bench.go` beside the other fixed names as `CardNumbersName = "card-numbers.txt"`.

It does not go inside `cards/`, which the card's description offered as one option. `docs/design/format.md:178` states that collections get no ceremony files, and an absent collection directory means an empty collection. A file inside `cards/` would be the first exception to that rule, and it would sit inside the live half of a collection whose numbering spans both halves, so a reader would have to be told that the file in `cards/` also speaks for `archive/cards/`. The name carries the entity kind because every other numbering in this format is a per-collection ordinal stored on the entity, and a bare `numbers.txt` at the root would not say which of those it holds.

**Grammar.** One line per allocated number, in allocation order:

```
<number> <identifier>
```

`<number>` is a decimal integer of one or more digits, no leading zero, no sign, and greater than zero. A single ASCII space separates the two fields. `<identifier>` is either a 12-hex card identifier as `IsID` (`internal/bench/storage.go:111`) admits one, or the single character `-`, which is the tombstone described below. Every line ends with `\n`, including the last. There are no comments, no blank lines, no header, and no trailing whitespace. The file is UTF-8 and is read through `ReadText`, so a byte-order mark an editor added is stripped and `SplitLines` tolerates CRLF, which is the same tolerance every other text file in this format gets.

Line order is allocation order and is the only record of it. The file is never sorted, and a repair that rewrites a line rewrites it where it stands.

**The tombstone.** Deleting a card rewrites its line's identifier to `-` rather than removing the line. The number stays allocated, because `NextNumber` reads the registry's high-water mark and a removed line would let the next filing hand out a number a deleted card once answered to. A tombstoned line is not a defect and is reported by nothing.

**Absence.** A workbench with no `card-numbers.txt` has allocated no numbers, which is the same absent-means-empty rule collections keep. `init` writes no registry, and the first filing creates it.

**Git.** Nothing is added to `.gitattributes`. `unionJournals` (`internal/bench/bench.go:52`) names the two journal patterns and nothing else, and git's default three-way text merge is exactly the behaviour this design wants: two branches appending a line at the end of one file conflict, and the conflict is the point. Adding `merge=union` here would restore the silent duplicate this card exists to remove, so pin it with a test asserting that `unionJournals` does not mention `CardNumbersName`.

## Reading a number

**In memory.** `Bench` gains a registry loaded once by `Open`:

```go
// NumberRegistry is the workbench's card-number registry as read from
// card-numbers.txt: the lines in file order, which is allocation order,
// plus the two indexes a read path asks for.
type NumberRegistry struct {
    Lines    []NumberLine     // file order, malformed lines included
    ByID     map[string]int   // identifier to the first number claiming it
    ByNumber map[int][]string // number to every identifier claiming it, file order
    Highest  int              // the greatest number any well-formed line carries
}

// NumberLine is one line of the registry.
type NumberLine struct {
    Number int    // zero on a malformed line
    ID     string // the identifier, "-" on a tombstone, "" on a malformed line
    Raw    string // the line as stored, which is what a malformed line reports
}
```

A malformed line is kept in `Lines` and enters neither index. Nothing refuses a workbench over one, because refusing on read would make a hand-damaged workbench unopenable, which is the posture `FindingUnknownLevel` and `FindingUnknownColumn` already keep.

**`Card.Number` stays a field and stops coming from frontmatter.** `LoadCard` (`internal/bench/card.go:100`) stops reading the `number` key at `:193`, and the field is stamped by the bench instead. Add two methods in `card.go`, beside the readers they wrap:

```go
// LoadCardIn loads a card and stamps the number the workbench allocates it.
// The free LoadCard reads a card's own file and can know nothing about a
// number that no longer lives there, so every caller that goes on to read
// Number or to call Ref comes through here.
func (b *Bench) LoadCardIn(root, id string) (*Card, error)

// loadRetiredCardIn is LoadCardIn for a bench written in the retired
// vocabulary, which the lenient opener is the only source of.
func (b *Bench) loadRetiredCardIn(root, id string) (*Card, error)
```

Both call their free reader, then hand the card to one stamping helper:

```go
// stamp writes the number the workbench holds for this card onto the card it
// is given. A workbench at RegistryFormat or above reads the registry; one
// below it reads the number key the card's own frontmatter still carries,
// which is the whole of the legacy read path.
func (b *Bench) stamp(c *Card)
```

A card with no line keeps `Number` zero, which is what `Ref` already reads as no number (`internal/bench/card.go:403`). A card claimed by two lines takes the first in file order, and `check` reports the state.

The legacy branch lives in `stamp` rather than in `LoadCardIn` alone, and that placement is the whole reason the second method exists. `Bench.Cards` routes a lenient-opened workbench through `retiredCardsIn` (`bench.go:2079`) to `loadRetiredCard`, which never passes through `LoadCardIn`, so a fallback living in `LoadCardIn` would leave every card of every retired-vocabulary workbench carrying `Number` zero and every reference of theirs rendering as hex. That is the same defect the guard below exists to catch, reached by a route a guard whose subject is `LoadCard` alone cannot see.

Both methods take a string and answer an entity, so both fall into the subject set of `TestEveryAddressAnsweringFunctionIsRosteredOrExempted` (`cmd/dinah/address_form_arms_test.go:221`), which sweeps every function declared in `internal/bench` and fails on one that is neither rostered as a resolver nor exempted on a stated ground. That test errors in both directions, so the roster in `internal/addressform/addressform.go` takes four edits rather than two: add `LoadCardIn` and `loadRetiredCardIn` under `GroundLoadsByIdentifier`, beside the `LoadCard` entry at `:370`, on the ground that each reads a card out of the directory its identifier names and stamps the number the workbench holds for it; and delete the `cardsIn` entry at `:391` and the `retiredCardsIn` entry at `:394`, because both functions are deleted below and an exemption excusing nothing fails the same test.

Convert every non-test call site to `LoadCardIn`. There are twenty-five at `933f0d5`, seven in `internal/bench` and eighteen in `internal/verb`, plus one reference to a reader as a value and one further reference to `loadRetiredCard`. The guard section below enumerates them and says how the enumeration is kept honest.

`Bench.Cards` (`internal/bench/bench.go:2064`) becomes registry-aware on both of its branches, which is what keeps `ByArrival` (`internal/bench/card.go:572`) computing CORE-QUEUE-3's tie-break from a number that is still there:

```go
func (b *Bench) Cards() ([]*Card, error) {
	if b.retiredVocabulary {
		return cardsWith(b.CardsRoot(), b.loadRetiredCardIn)
	}
	return cardsWith(b.CardsRoot(), b.LoadCardIn)
}
```

`cardsIn` (`:2073`) and `retiredCardsIn` (`:2079`) are both deleted rather than rewritten, because each is a free function taking a root and a root carries no registry. Each method value satisfies `cardsWith`'s existing `func(string, string) (*Card, error)` parameter with no change to `cardsWith` itself, and `resolveCardIn` stops scanning the half at all, as the resolution paragraph below says, which takes the other caller of `cardsIn` away. Deleting the two functions is also what removes the reference at `:2074` and the reference at `:2080`, so the guard's allowlist below names three files rather than four.

**`Card.Save`** (`internal/bench/card.go:413`) stops writing the `number` key at `:416`. Nothing else in `Save` changes.

**Resolution by number** (`internal/bench/resolve.go:44`) stops scanning the collection. After `splitRef` yields a number at `:69`, `resolveCardIn` reads `b.Numbers.ByNumber[number]` in place of the `cardsIn(root)` call at `:73`, and then:

- No identifier claims the number, so it refuses `unknown-card` against the reference the caller typed, exactly as today.
- One identifier claims it, so it loads that card from the root it was given and answers it. An identifier the registry carries whose directory is not in this root is the card being in the other half, and that refuses `unknown-card` too, which preserves today's split between `ResolveCard` and `ResolveArchivedCard`.
- More than one identifier claims it, so it refuses `dinah.ambiguous-card` with the candidates, which is dinah-487's raise site moved from the scan to the registry lookup.

A tombstone claims no card, so a number whose only line is a tombstone refuses `unknown-card`.

**What that costs.** The read path gets cheaper rather than dearer. Today `resolveCardIn` calls `cardsIn`, which reads and parses every anchor in the half to find one number. After this card it reads one file at `Open` and then one anchor. The registry is about seventeen bytes a line, so the Dinah workbench's own five hundred cards cost under nine kilobytes and one `os.ReadFile` per invocation. That read happens on every `Open`, including the many invocations that never mention a number, and that is the honest cost: one small sequential read against a scan of every card anchor that resolution used to pay per reference.

## The guard over the free card readers

Three functions in `internal/bench` read a card off its own directory and can know nothing about a number that no longer lives there. `LoadCard` (`internal/bench/card.go:100`) and `loadRetiredCard` (`:112`) both delegate to `loadCard` (`:120`). All three stay and all three stay number-free, because the migration and one check have to read a card before there is a registry to stamp from, and tests build cards directly. All three answer a card whose `Number` is zero, and `Ref` (`card.go:403`) renders zero as the card's bare hex identifier, so a card that leaves one of them and reaches a person shows twelve hex digits where a reference belongs.

Every review round from 2 to 6 defeated the guard the round before it proposed, and round 6 then defeated the repair it opened with, so the guard is specified here as two rules with the attacks run against them rather than described. None of those defeats needed a new reference. The counts below came from running a scanner over the tree at `933f0d5` rather than from grepping it.

**The subject is all three readers rather than the exported one.** `retiredCardsIn` (`bench.go:2079`) reaches `loadRetiredCard` rather than `LoadCard`, and `Bench.Cards` calls it on a lenient-opened workbench (`:2065`), so an unstamped card already reaches a caller today by a route a one-name subject set cannot see. A scan whose subject is `LoadCard` alone reports twenty-seven references on trunk. The subject of three names reports thirty-two, and the five it adds are `card.go:101`, `card.go:112`, `card.go:113`, `card.go:120`, and `bench.go:2080`.

**Rule 1, the reference rule.** The guard parses every non-test `.go` file under `cmd` and `internal` with `go/parser` and walks each file with `ast.Inspect`, recording every node that refers to one of the three readers. Two node shapes are the whole subject: an `*ast.Ident` whose `Name` is exactly one of the three, in a file whose package clause is `bench` or which dot-imports `dinah/internal/bench`; and an `*ast.SelectorExpr` whose `Sel.Name` is exactly one of the three and whose `X` is an identifier the file's own `ImportSpec` list binds to `dinah/internal/bench`. A reference in a file no allowlist entry names fails the guard, and a reference count above an entry's budget fails it too.

`internal/bench/kindguard_test.go` is the precedent and the shape to copy: `scanForKindAnswers` at `:56` walks and parses, `kindExemption` at `:191` carries a path, a line budget, and a reason, `kindAllowlist` at `:208` asserts the list in both directions, `repositoryRoot` at `:363` finds the module root by climbing to `go.mod`, and `TestTheKindGuardGoesRed` at `:301` proves the scan fails by running it over a planted file. Build this guard from the same parts, in `internal/bench`, and prove it red the same way.

**What the shape buys, case by case.**

- A call inside package `bench`, spelled bare, is an `*ast.Ident` and is caught.
- A call from `internal/verb`, spelled `bench.LoadCard(`, is an `*ast.SelectorExpr` and is caught.
- A function value rather than a call, as `cardsWith(root, LoadCard)` at `bench.go:2074` is today, is an `*ast.Ident` in an argument position. Nothing in the scan asks whether the reference is called, which is the point of counting references rather than calls.
- A reference split across a line break is one node whatever its source spans, so the match does not depend on the break.
- An aliased import, `b "dinah/internal/bench"` followed by `b.LoadCard`, is caught because the qualifier is resolved against the file's imports.
- A dot-import in a file of another package spells the reference as a bare `LoadCard`, and the `*ast.Ident` rule reaches it because that rule reads the file's import list as well as its package clause.
- `LoadCardIn` and `loadRetiredCardIn` are not caught, because an identifier's `Name` is compared whole. A substring search would have flagged both and taught whoever ran it to distrust the guard.
- The string `"LoadCard"` in the exemption roster at `internal/addressform/addressform.go:370` is an `*ast.BasicLit` and is correctly not a subject, so the guard does not report the entry that exists to describe the function.

**The allowlist, with the budget each entry carries.** A budget counts the declaration of a reader as well as every reference to one, because a declaration is an `*ast.Ident` like any other node the rule reads and excluding it would be a special case nobody reading the number could predict.

| File | Budget | Why it may refer to a free reader |
| --- | --- | --- |
| `internal/bench/card.go` | 7 | It declares all three readers, and it declares the two bench methods that stamp what they answer. |
| `internal/bench/check.go` | 1 | It probes whether a card directory a registry line names will read at all, which is a question about the file rather than about the card. |
| the migration's own file | 1 | It reads every card's frontmatter number to build the registry, on a workbench that has no registry yet. |

Nine references in three files, against thirty-two in nineteen files today. `internal/verb` gets no entry, and a fourth file wanting one is the defect this guard reports rather than a line to add.

**Where the other twenty-three go.** All eighteen in `internal/verb` become `b.LoadCardIn`. In `internal/bench`, `entity.go:487`, `resolve.go:50`, `witness.go:84`, and `workstream.go:291` become `b.LoadCardIn`. `bench.go:2074` and `bench.go:2080` go with the deletion of `cardsIn` and `retiredCardsIn`, and `bench.go:2130` goes with `NextNumber`'s rewrite, which leaves `bench.go` with none. `check.go:283`, which is `Bench.Check`'s own card walk, becomes `b.LoadCardIn`, and it has to: `checkItemColumns` puts `card.Ref(b.Slug)` at the head of its detail (`check.go:548`), so a walk answering unstamped cards makes `dinah check` print hex where a reference belongs. `check.go:820` is inside `checkCardNumbers`, which stops reading cards at all once it is re-pointed at the registry, and the one surviving free reference in that file is the readability probe the stranded check needs in order to keep the property D-4 requires.

**Rule 2, the escape rule.** Rule 1's subject is syntactic references to three names. The property the design needs is that no unstamped card reaches a caller, and those are different sets, which is how round 4 defeated the guard without writing a forbidden name: lift the one allowed call in `check.go` into an exported method, have the walk call the method, and every file in the module can reach an unstamped card at a spelling the scan never reads. Rule 2 closes that by asserting, over the allowlisted files alone, that a card a free reader answers does not leave the function that read it by any of the shapes 2a, 2b, and 2c enumerate. That is an enumeration and not a proof. The guard reads syntax and knows no types, so what it asserts is that the listed escapes are absent from the listed files, and a shape nobody has written is a shape it does not report.

Each allowlist entry names the functions in that file permitted to answer a card, and `card.go`'s entry names `LoadCard`, `loadRetiredCard`, and `loadCard`. Every other function in an allowlisted file is held to all three halves of the rule:

- **2a, the declaration half.** A `*ast.FuncDecl` in an allowlisted file whose result list mentions `Card`, in any composite form, fails the guard unless the file's entry names it. Exported and unexported both, because an unexported one still reaches every other file of package `bench`.
- **2b, the storage half.** A package-level `var` or a struct field declared in an allowlisted file whose type mentions `*Card` fails the guard.
- **2c, the value half.** Within one function, the guard tracks the identifiers a free reader's value is bound to and fails on that value leaving the function. It reads both of Go's binding forms, an assignment (`*ast.AssignStmt`, which covers `c, err := LoadCard(...)` and a later `c = ...`) and a declaration (`*ast.ValueSpec`, which covers `var c, err = LoadCard(...)`), because the two bind the card equally and a guard reading one of them makes the other an evasion. Four right-hand shapes carry the taint to whatever the binding names, and each of the four is a shape that defeated an earlier prototype rather than a precaution:
  - a tracked identifier standing alone, read through any number of parentheses, address-of operators, and dereference operators, so that `d := c`, `d := (c)`, `p := &c`, and `d := *c` all bind the card;
  - an `append` whose argument is tracked;
  - a composite literal carrying a tracked identifier anywhere inside it, whether the literal has a named struct type, a slice type, or a map type;
  - a call whose function expression carries a tracked identifier, which is a method called on the card, because the method that answers the card can be declared in a file rule 2 never inspects.

  It then fails on a tracked identifier appearing anywhere inside a `return` statement, on a tracked identifier assigned to a selector or an index expression, on a tracked identifier assigned to one of the enclosing function's own named results, on a tracked identifier sent on a channel, on a tracked identifier standing anywhere in the argument list of a call, and on a free-reader call standing inside a `return` statement. The named-result clause is there because a function with named results can answer with a bare `return`, which carries no result expressions for the return clause to read. Only the first target of a multiple assignment or declaration takes the value, because a free reader answers the card in its first result and an error in its second. That last restriction is not a nicety: without it `err` is tracked, and every `return findings, err` in `Bench.Check` is reported, which is the false-positive rate that would get the guard deleted in its first month.
  The composite-literal clause and the call-receiver clause each taint a value that may be something other than the card, a struct holding it or a string rendered from it, and that is deliberate. The alternative is to taint on any expression mentioning a tracked identifier, which reaches `n := card.Number` and so forbids the migration the allowlist exists to permit.

**The call-argument clause closes the route round 5 found, and it is the clause with the widest reach.** Without it, a permitted function hands its card to a method declared in `bench.go`, that method holds the card on the bench, and an exported accessor beside it answers the card to any caller in the module. Rule 2 never reads `bench.go`, because rule 2's subject is the allowlisted files, and 2b does not reach the shape either, since the struct field is declared where the guard is not looking. Passing the card is the only step of that route that happens inside an allowlisted file, so the argument list is where it has to be caught. The builtin `append` is the single call excluded from the clause, because the guard already follows its taint and reporting the argument as well would say one thing twice.

The clause is affordable only because of what this card leaves behind in the three allowlisted files. `check.go` keeps one free reference, in a probe that asks whether a card directory reads at all and has no reason to hand a card anywhere. `card.go`'s three readers are named by its own entry and are exempt from rule 2. The migration reads `card.Number` off a selector rather than passing the card on, and the criterion binds that read tighter than the sentence suggests: 2c reads the whole of a call's argument expression, so `strconv.Itoa(card.Number)` fires on the tracked `card` inside it and was run doing so. The migration binds the number to an identifier of its own first and passes that, which the same run reports nothing for. An implementer needs that sentence rather than the shorter one. On the tree as it stands the clause fires exactly once, at `check.go:288`, where `Bench.Check`'s card walk hands its card to `b.checkCard`, and that walk is the call site this card converts to `b.LoadCardIn`. Both halves of that were run rather than reasoned: the scan over the tree reports the one hit, and the same scan over a copy of `check.go` with `:283` converted reports one rule 1 reference and nothing under rule 2.

**The attack, run against the rules rather than reasoned about.** A prototype of both rules was built and run on 2026-09-11, and by the end of round 6 it had been run over the tree at `933f0d5` and over twelve planted directories, five of which this card's reviewers wrote against the prototype rather than its author. On the tree it reports thirty-two rule 1 references in nineteen files and one rule 2 hit, which is the `check.go:288` call argument described above and which this card's own conversion of that line removes. Those two figures were reproduced in round 6 against `933f0d5` by the reviewer and again by this spec's author, independently, with the repaired rules in place, and neither run reported a single reference the earlier runs had not. The first planted directory is round 4's own defeat, written as the reviewer described it:

```go
package bench

func (b *Bench) LoadCardUnstamped(root, id string) (*Card, error) {
	return LoadCard(root, id)
}

func (b *Bench) checkCards(ids []string) []Finding {
	var findings []Finding
	for _, id := range ids {
		card, err := b.LoadCardUnstamped(b.CardsRoot(), id)
		if err != nil {
			continue
		}
		findings = append(findings, b.checkCard(card)...)
	}
	return findings
}
```

The file carries one reference, so rule 1's budget of one is satisfied and no new file wants an entry, which is exactly why the round 4 guard passed it. Rule 2 reports it twice, once under 2a for a declaration answering a card that `check.go`'s entry does not name, and once under 2c for a free-reader call standing inside a return statement.

The second planted directory holds ten shapes, each written to get a card out of `check.go` by a different route, and all ten now fail. Nine are named here, and the tenth is round 4's own defeat standing in the same file:

| The shape | The rule that reports it |
| --- | --- |
| An unexported `cardAt` answering `(*Card, error)` | 2a, and 2c on the returned identifier |
| A method aliasing the value into `d` before returning `d` | 2a and 2c |
| A method storing the card on `b.lastCard` and returning nothing | 2c, on the assignment to a selector |
| A method whose results are `(any, error)`, laundering the type | 2c, on the returned identifier |
| A method answering `holder{card: c}`, laundering through a composite literal | 2c, because the tracked identifier stands inside the return |
| A method collecting cards with `append` and answering the slice | 2a, and 2c with the taint following the `append` |
| A method answering `c.Number` rather than the card | 2c, on the tracked identifier inside the return |
| A method copying the card's fields into a local struct of its own and answering that struct | 2c, once a composite literal carrying a tracked identifier taints what it is bound to; this is the shape rounds 4 and 5 admitted |
| A closure answering `(any, error)` that calls the reader in its body | 2c, because the walk enters the function literal |
| Any of the above written in a file the allowlist does not name | rule 1 |
| Round 4's defeat rewritten as `var c, err = LoadCard(...)` answering `(any, error)` | 2c, once `*ast.ValueSpec` is a binding form the guard reads |
| A method binding with `var` and answering the value as `any` | 2c, on the same clause |
| A permitted probe handing its card to a helper declared in `bench.go`, which stashes it on the bench for an exported accessor to answer | 2c, on the call argument |
| A permitted probe binding a struct literal that carries the card and answering the binding on the next line, with the struct type declared in `bench.go` | 2c, once a composite literal carrying a tracked identifier taints what it is bound to |
| The same evasion with a slice literal, `cs := []*Card{c}`, which declares no type anywhere | 2c, on the same clause |
| The same evasion with a map literal bound by `var`, `var m = map[string]*Card{id: c}` | 2c, on the same clause |
| A permitted probe assigning the card to one of its own named results and answering with a bare `return` | 2c, on the named-result clause |
| An alias taken through the address-of operator, `p := &c`, answered as `any` | 2c, once the alias is read through the operator |
| An alias taken through parentheses, `d := (c)`, answered as `any` | 2c, on the same clause |
| A dereference copy answered by its address, `d := *c` then `return &d`, which carries every field of the live card | 2c, on the same clause |
| A method declared in `bench.go` whose receiver is the tracked card and whose result is that card, bound and then answered | 2c, once a call whose function expression carries a tracked identifier taints its result |

The last eleven rows were written against a prototype rather than against this prose, and every one of them got through the prototype as the round before it left it. Three are round 5's reviewer's. The first two of those are one evasion twice, because the prototype tracked `*ast.AssignStmt` alone, so a `var` declaration bound nothing and round 4's own defeat came back clean with three characters changed; the third is the call argument. Four are round 6's reviewer's. Three of those are one gap three ways, because a struct literal, a slice literal, and a map literal each carry the card to a binding the prototype then followed nowhere, and the fourth writes the first of the three exported, in its own directory, with a caller in `internal/verb` reading `h.Card.Ref(slug)` off the escaped card. Five are this spec author's, written in round 6 against the rules as the composite-literal clause had just repaired them, and all five defeated that repair: a named result answered by a bare `return`, an alias taken through `&`, an alias taken through parentheses, a dereference copy answered by its address, and a method on the card declared where rule 2 does not look. Each of those five spends exactly one free reference, so rule 1's budget for `check.go` is satisfied and the escape is rule 2's to catch or miss. Every repair is in the rules above and in AC-12, and after the last of them every shape in every planted directory was re-run. The corpus now stands at twenty-three shapes in twelve planted directories, sixteen of them this spec author's and seven the reviewers', and the repaired rules report all twenty-three.

**The count is a statement about the prototype rather than about the guard, and the two are not the same thing.** The prototype implements rule 1, 2a, and 2c. It implements no 2b at all, it enforces no budget, and it dedupes its rule 1 hits per source line, so its thirty-two is a count of lines carrying a reference rather than of references. Where the two disagree the criterion governs, and AC-12 is the contract. Every evasion rounds 5 and 6 found is the reason AC-12 now names `*ast.ValueSpec`, the argument list, the composite literal, the operators an alias may be read through, the named result, and the call whose receiver is the card, each in as many words. None of them can be inferred from "the identifiers a free reader's value is bound to", which is the phrase that shipped in round 4 and which an implementer copying the prototype would have read as covering nothing at all.

**The shape this spec used to admit is closed, and what it said about that shape was wrong twice.** The shape is a permitted function copying the card into a local struct of its own and answering the struct. Rounds 4 and 5 said what escapes that way is an integer zero rather than a card, and said that closing it needs type information rather than syntax. Both sentences were false. The struct's field type is written by whoever writes the struct, so a `*Card` field declared in a file rule 2 never inspects carries the live card, and round 6's reviewer ran that end to end with a caller in `internal/verb` reading `h.Card.Ref(slug)` off it while the whole scan reported nothing but the one budgeted reference. Closing it took syntax and not type information: a composite literal carrying a tracked identifier taints what it is bound to, which is the treatment `append` already had. The clause costs nothing on the tree, and the corpus shape that used to get through is now reported at its return.

**What the guard still does not reach, stated rather than implied.** Rule 2 reads the three allowlisted files, so a card that arrives anywhere else as a function parameter is outside it entirely, and rule 1 is what keeps a fourth file from reading a card in the first place. Rule 2c reads one function at a time and knows no types, so its reach is exactly the list of shapes 2c enumerates. Three consecutive rounds have each been decided by a shape nobody had written before that round, and the round 6 repair was itself defeated within the round by five more, so the honest reading of the corpus is that it records what has been tried rather than what is possible. The cover for what the scan cannot see is the observable described two paragraphs below rather than a further clause here.

**What no source scan can reach, and what covers it.** `LoadCard` is exported from a library another module can import, and no test in this tree sees a caller outside it. Its doc comment says the card it answers carries `Number` zero and that a caller wanting a number goes through `Bench.LoadCardIn`, so an outside caller reads a documented sentinel rather than a stale number. That is a degradation rather than a falsehood, and it is the only defence a library has.

The second cover is the observable, which matters because a syntactic guard can only answer a syntactic question. AC-13 drives it: over a workbench whose registry deliberately disagrees with the numbers its cards were filed under, every command that prints a card reference prints the registry's number, and no command prints a bare hex identifier where a reference belongs. Both halves were run on 2026-09-11 and both redden on the wrong build, which the criterion records. The two nets catch different things on purpose. The scan catches a new reference before it can do harm and names the file and the line, and the observable catches a reference the scan could not see and names the command whose output is wrong.

## Allocating a number

Filing a card runs in this order, under the workbench lock:

1. Admit the levels and resolve the destination column, unchanged.
2. Acquire the workbench lock, refusing `dinah.locked` with the holder's name when another process holds it.
3. Read the registry. Refuse `dinah.needs-number-migration` when the workbench has no registry and its cards carry `number` in frontmatter, per the migration gate below.
4. Compute `Highest + 1`.
5. Claim the identifier with `ClaimID`, write the anchor without a `number` key, and append the created event, all as today.
6. Append `<number> <identifier>\n` to the registry with an append-open followed by `Sync`, which is what `AppendEvent` (`internal/bench/journal.go:122`, its append-open at `:130`) already does for a journal, rather than the write-temp-then-rename of `WriteText`. An append cannot lose lines that are already in the file.
7. Release the lock.

The order matters: the identifier is claimed before the registry line is appended, so a crash between the two leaves a card with no line, which `check` reports and `--migrate-numbers` repairs. The reverse order would leave a line naming a card that was never created, which is the harder state to reason about.

`NextNumber` moves off the collection scan. Replace the body at `internal/bench/bench.go:2122` with a read of `b.Numbers.Highest + 1`, and keep the error return dinah-439 gave it.

Its doc comment promises that a number is never reused, and the promise has to be narrowed rather than restated, because the tombstone does not make it true outright. From the migration forward it holds: a card deleted after migration leaves a tombstoned line, the high-water mark never falls, and the number is gone for good. Below the migration it does not: the migration builds the registry from the cards that survive, so a card deleted before the migration leaves no tombstone and its number is reissuable. That is exactly as good as today, since the present scan over surviving cards has the same hole, and it is worth saying why it is not closed. The workbench journal does record deletions, so a migration could mine `deleted` events for tombstones, and this card does not, because a deleted card's number is not recoverable from an event that names an identifier rather than a number, and reading the number would mean reading an anchor that is gone. The comment says the number a migrated workbench allocates is never reused and says which side of the migration the promise starts on.

## Archive, restore, and delete

Archiving a card changes nothing in the registry. The card still exists, still answers to its number, and `check` looks in both halves, so the line stays exactly as written. Restoring changes nothing for the same reason.

Deleting a card rewrites its line's identifier to `-`, under the workbench lock the structural act already holds first (`docs/design/format.md:2033`). The rewrite goes through `WriteText`, because it is a modification rather than an append.

## The storage format bump

`StorageFormat` moves from 2 to 3 (`internal/bench/bench.go:76`). Add a second boundary constant beside `ContainerFormat` (`:87`), on the same reasoning its comment gives:

```go
// RegistryFormat is the storage format from which the card-number registry
// binds. A workbench declaring this number or a higher one carries its card
// numbers in card-numbers.txt; one declaring less carries them in card
// frontmatter and is read that way until it is migrated.
const RegistryFormat = 3
```

**What each build does with each workbench.** The table is the whole compatibility contract:

| The workbench declares | This build does | An older build does |
| --- | --- | --- |
| no format, or 1, or 2 | opens it, reads numbers from frontmatter, reports `check.card-number-missing` for every card, and refuses any act that would allocate a number with `dinah.needs-number-migration` | opens it and reads it exactly as it does today |
| 3 | opens it and reads numbers from the registry | refuses with `unsupported-version`, naming `format 3`, at `internal/bench/bench.go:1590` |

The refusal an older build gives is already in place and needs no code. A build reading a format it does not implement refusing loudly is the point of the number, and the interesting half of the table is the other row: this build reads an unmigrated workbench rather than refusing it, because an operator whose workbench will not open has no route to the migration that would open it.

The frontmatter fallback lives in `Bench.stamp` and nowhere else. When `b.Format < RegistryFormat`, it reads the `number` key off the frontmatter the card already carries rather than the registry. That one branch is the whole of the legacy read path, it is not reachable on a workbench declaring 3, and it sits in the stamping helper rather than in `LoadCardIn` so that the retired-vocabulary route reaches it too.

`Card.Save` (`card.go:413`) stops writing the `number` key, and on a workbench still at format 2 that is not data loss. Save's own doc comment (`:410`) says every field the tool does not own is preserved by the frontmatter holding raw lines, so an unmigrated card keeps its `number:` key across an edit and the legacy branch above goes on reading it. The fallback's correctness depends on that, which is why it is written down rather than left for the reader to work out.

Add `NeedsNumberMigration = LayerPrefix + "needs-number-migration"` to `internal/contract/contract.go` beside `NeedsContainerMigration` (`:223`), add it to `Introduced` (`:539`), and give it a shape in `internal/contract/shape.go` whose next step names `dinah check --migrate-numbers --yes`.

## The migration

`dinah check --migrate-numbers --yes` builds the registry from card frontmatter, strips the `number` key from every card anchor, and stamps `format: 3` on the workbench anchor. Register the flag in the `"check"` option block (`internal/verb/definition.go:619`) beside the markers already there, which are `--finish`, `--migrate-ordinals`, `--migrate-slugs`, `--migrate-columns`, `--migrate-vocabulary`, `--migrate-container`, `--migrate-workstreams` and `--witness`, plus `--remint`, which takes a path rather than standing alone, add `MigrateNumbers` to `verb.Request` beside `MigrateOrdinals` (`internal/verb/library.go:209`), and add the case to the MCP head's flag switch (`internal/mcp/mcp.go:942`) and its tool description (`internal/mcp/tools.go:201`).

**What it reads.** Every card in both halves of the collection, live first and archived second, through the free `LoadCard`. A card whose anchor will not load ends the migration with a refusal rather than being stepped over, because a card carrying a number nobody read is a number the registry may hand out again, and this repair is the one place where a silent gap is unrecoverable.

**The order it writes.** Lines ascend by number. For a workbench that has never had a duplicate, that is allocation order exactly, since the old allocator counted up. Ties are broken as below, and the loser is renumbered above the high-water mark, so its line lands last.

**The tie-break, which AC-1 binds.** Two cards carrying one number are ordered by journal order, which this spec fixes as a total order so that two clones migrating the same workbench independently produce the same registry:

1. The timestamp of the card's own `created` event, earliest first, parsed by `ParseStamp`.
2. Where those are equal, or where a card has no `created` event or no journal at all, the card's 12-hex identifier ascending.

A card with no readable `created` event sorts after every card that has one, and then by identifier. The first card in that order keeps the number. Every other card in the group is renumbered to the next number above the high-water mark, taking them in the same order, so the renumbering is deterministic too.

A renumbered card gets a line in its own journal, which is what answers the operator who asks why a card is called something else this morning:

```go
Event{TS: now, Event: contract.EventRenumbered, Actor: actor, From: "99", To: "137"}
```

Add `EventRenumbered = "renumbered"` to `internal/contract/contract.go`, add it to `Events` (`:753`), give it a detail arm in the event renderer (`cmd/dinah/render.go`) reading `From` and `To`, and add `token.renumbered` to the catalogs.

**What it reports.** The count of lines written, the count of cards renumbered, and a finding per card it renumbered so the operator sees which references went stale. Mint `check.card-number-renumbered` for that, on the terms `FindingOrdinalGuessed` is minted: it is raised by the repair and never survives on disk for a later check to find.

**Idempotence.** A second run on a migrated workbench writes nothing and reports nothing, because the workbench declares 3, no card carries a `number` key, and the registry already covers every card.

## What `check` reports

Six keys, because the six conditions have six different repairs and one sentence naming several of them would help nobody. All six are card-scoped and all six read the registry rather than frontmatter.

| Key | Condition | Repair |
| --- | --- | --- |
| `check.card-number-duplicate` | Two or more lines claim one number. | `--renumber` |
| `check.card-number-repeated` | Two or more lines claim one identifier, so one card has two numbers. | By hand; choosing what a card is called is the operator's. |
| `check.card-number-missing` | A card in either half has no line. | `--migrate-numbers` |
| `check.card-number-stranded` | A line that parses, whose identifier is neither a tombstone nor a card in either half. | By hand; removing the line would release the number for reuse. |
| `check.card-number-malformed` | A line does not parse under the grammar above. | By hand. |
| `check.card-number-in-frontmatter` | A workbench declaring 3 carries a card with a `number` key. | `--migrate-numbers` |

The stranded rule is scoped to lines that parse, and that scoping is load-bearing rather than tidy. A malformed line carries `ID == ""` and enters neither index, and an empty identifier is neither a tombstone nor a card in either half, so a stranded check reading `Lines` naively reports the same damaged line twice under two keys with two different repairs. Malformed lines are excluded from the stranded check by construction: the stranded check reads only lines whose `Number` is non-zero and whose `ID` parsed.

`check.card-number-duplicate` is dinah-439's finding re-pointed at the registry, and D-4 requires that both of its hard-won properties survive the move:

- An archived card whose anchor will not load is reported rather than skipped, because the archived half is where a duplicate hides, and a detector going quiet there hides the case it exists to find. After this card the grouping is over registry lines rather than anchors, so the unreadable-anchor report moves to the stranded check: a line whose identifier names a directory that is present but unreadable is reported with `unreadableCardFinding(err)` rather than as stranded, since a directory that is there is not a line naming nothing.
- A card carrying no number is skipped rather than grouped on zero. Under the registry that case cannot arise in the duplicate check, because a card with no line contributes no line to group, and it is reported by `check.card-number-missing` instead. The guard's purpose survives in the new shape: a workbench mid-repair is full of numberless cards, and it must draw exactly one finding each rather than a collision report each.

**The missing-number flood on an unmigrated workbench is intended.** A workbench declaring format 2 is entirely legal, and it draws one `check.card-number-missing` per card, which on this workbench is five hundred findings. A single workbench-scoped finding naming the migration would tell an operator the same thing in one line, and it is the wrong trade here. The finding is card-scoped in every other state it reports, and a key that is sometimes card-scoped and sometimes workbench-scoped forces every reader of a finding to ask which it got. The flood also stops after one migration run, which is a cost paid once rather than a standing noise. D-10 already settles that an unmigrated workbench opens and reads; this is the visible consequence of that, and the refusal on `dinah add` is what an operator meets first anyway.

`check.ordinal-missing` (`internal/bench/check.go:636`) stops being raised for a card and goes on covering comments, attachments, and checklist items. Its repair is `--migrate-ordinals` and a card's is now `--migrate-numbers`, so one key naming two flags would send half its readers to the wrong command.

**The two checks read different halves, and that is deliberate.** The card-number checks read both halves, because the registry spans both and an archived card holding a number is exactly where a collision hides. `checkOrdinals` (`internal/bench/check.go:727`) is reached from `Bench.Check`'s card walk, which lists `b.CardsRoot()` alone (`:266`), so below-card ordinals are checked in the live half only. This card does not widen it. A comment's ordinal is scoped to the one card holding it, so an archived card's comments can collide with nothing outside that card, and the card itself is unreachable by reference until it is restored, at which point the live-half walk covers it. The asymmetry is therefore between a workbench-scoped fact and a card-scoped one rather than between two checks that disagree. A later card wanting the archived half swept for below-card ordinals should say what damage it is defending against, because this card could not name one.

Detail and path on each finding follow what the existing findings do. A finding about a card names the card's anchor and carries the card identifier. A finding about a line names `card-numbers.txt` and carries the line as stored, which is what a reader searches the file for.

## The repair

`dinah check --renumber --yes` repairs `check.card-number-duplicate` and nothing else. For each number more than one line claims, the line appearing first in the file keeps it, and every later line is rewritten in place with the next number above the high-water mark, taking them in file order. Each renumbered card gets the `renumbered` event above, and each is reported with `check.card-number-renumbered`.

The line is rewritten where it stands rather than moved to the end. Line order is the only record of who claimed first, and that record is the evidence the repair itself depends on, so a repair that reordered the file would destroy its own input. The consequence is that numbers do not ascend down the file after a repair, which is why nothing anywhere may assume they do.

`--renumber` is registered at the same four places `--migrate-numbers` is, and it needs saying because the previous round registered one and not the other while AC-9 drives both. Add `{Name: "renumber", Flag: true, Marker: true, Field: "Renumber"}` to the `"check"` option block (`internal/verb/definition.go:619`), add `Renumber` to `verb.Request` beside `MigrateOrdinals` (`internal/verb/library.go:209`), add its case to the MCP head's flag switch (`internal/mcp/mcp.go:942`), and give it a line in the tool description's marker map (`internal/mcp/tools.go:201`) saying that it renumbers the later claimant of a number two cards hold and that the reference somebody wrote down for that card stops resolving.

Both `--renumber` and `--migrate-numbers` require `--yes`. Every other marker either reports or stamps a field nobody reads aloud; these two change what a card is called, and a reference somebody wrote down last week stops resolving. The `--yes` doc comment in `internal/verb/definition.go` currently says that only `--migrate-vocabulary` and `--migrate-container` read it, and that sentence is amended rather than left to go stale.

## The cost this design accepts

A contributor working offline can have their card land under a different number than it carried on their branch. The registry makes the collision a git conflict instead of a silent duplicate, and whoever merges second renumbers, so the number a reference in a commit message or a branch name names is the number that was allocated on a branch rather than the number the workbench ended up with. The hex identifier is what survives that, which is why it is the identity and the number is a label.

Nothing here should be softened in any document. The format guarantees that no two cards share a number. It does not guarantee that a number is dense, sequential, or stable across a merge, and it cannot, because that needs an arbiter that knows what the last number was and no such thing exists offline. Dense authoritative numbering belongs to the hosted product, and the README's promise that there is no server to run stays intact by not making a promise the file format cannot keep.

## The profile amendment

The profile constrains the model rather than the file, so the amendment is about the creation ordinal and says nothing about a registry. `CORE-CARD-1` constrains the identifier and stays exactly as it is.

**The revision this card publishes is 0.14.** The document is already at 0.13, published on 2026-09-09 by dinah-450 and carrying CORE-JSON-10, CORE-GATE-3 and CORE-GATE-4. Read it off the document rather than off this spec before writing a line: `docs/spec/core-profile.md:3` reads ``Version identity: `dinah-core 0.13` ``, `:1559` reads ``The current revision is `dinah-core 0.13`.``, the last entry heading is at `:2028`, and `internal/profile/amendment_test.go` pins all three (`declaredVersion` at `:28`, `publishedStatements` at `:30`, `publishedChangelogEntries` at `:33`, `declaredCurrentRevision` at `:40`). If trunk has moved again by the time this is implemented, the same four places say so and the new revision is whatever they read plus one.

**Section 4 gains a vocabulary entry.** The term is used by CORE-QUEUE-3 (`:592`) and defined nowhere, which is its own small defect. Place it beside the card entries in the order section 4 already keeps, after **Field** (`:350`). This is the exact text, as a paragraph of the same shape the entries around it take:

> **Creation ordinal.** A whole number a workbench allocates to a card when the card is created, unique among the cards of that workbench. A tool may reallocate a card's ordinal when two copies of one workbench are reconciled and two cards arrive holding the same one. The ordinals a workbench has allocated need not run consecutively.

The entry is written from what the implementation guarantees rather than from what the word suggests, which is the correction round 4 required. The previous round proposed "The position a card fell in among the cards of its workbench, fixed when the card was created and never afterwards changed", and this card disclaims both halves of that sentence in its own text. Two paths here change a card's number after creation, the migration's tie-break and `dinah check --renumber --yes`, and the `renumbered` event exists precisely to answer the operator whose card is called something else this morning. Tombstones and renumbering both leave gaps, so the accepted-cost section is right that the format does not guarantee that a number is dense, sequential, or stable across a merge, and a definition promising a position among anything contradicts it.

**The entry's wording is constrained by section 3.5, and the constraint was run rather than trusted.** Section 3.5 (`:251`) binds the two excluded word lists to three places, and section 4 is the second of them: "The three places are every normative statement, section 4, and the walkthrough in section 10.1." The first list carries `merge` (`:281`, and `tradeTerms` at `internal/profile/extract_test.go:19`), and `TestExcludedTermsAreAbsent` (`internal/profile/extract_test.go:206`) runs `Excluded` over the span `section(text, "## 4. Core vocabulary")` returns, matching whole words without regard to case. Round 4 of this spec wrote the accepted cost as "when two workbenches merge", which is the one word that section forbids, so AC-11 could not have passed on the text that round shipped. The entry above says the same thing in the vocabulary the profile allows itself, and an implementer copies it verbatim rather than paraphrasing it, because a paraphrase can walk back into either list.

Proof, run on 2026-09-11 over a copy of the tree at `C:/dinah-scratch/dinah-488-spec/profcheck`. On the unmodified document `go test ./internal/profile` is `ok`. With the entry above spliced in after the **Field** entry, `go test ./internal/profile` is `ok` again, at 47.5s, and `TestProfileExtractsCleanly`, `TestExcludedTermsAreAbsent`, and `TestHouseStyle` pass individually. With the one clause put back to the round 4 wording and nothing else changed, the run is `--- FAIL: TestExcludedTermsAreAbsent` reporting `extract_test.go:223: the core vocabulary carries excluded terms [merge]`. Both directions were run, so the entry is known to pass rather than believed to.

**The entry carries no cross-reference to a statement either.** Round 4's wording closed with "and CORE-QUEUE-3 breaks a tie on the ordinal", and section 4 carries no `CORE-` identifier anywhere in its span, which runs from `:318` to `:438`. Nothing mechanical forbids one, and the extractor reads the line as prose rather than as a statement, but CORE-QUEUE-3 already says the thing where it stands and the entry loses nothing by dropping the clause.

The entry carries no RFC 2119 keyword in upper case. `extract.go` reports a keyword outside an identified statement as a stray (`:107` to `:111`), and the keyword pattern at `:40` matches upper case alone, so "may" in the second sentence is prose rather than a stray.

**Read the statement and the entry as one thing, because a conformance run does.** CORE-CARD-10 asserts that a card carries a creation ordinal and that no other card in its workbench carries the same one. Section 4 says what a creation ordinal is, and the statement means whatever section 4 says. Together they assert uniqueness at an instant, across both halves of the collection, and they assert nothing about density, about order, or about an ordinal outliving a merge. That is exactly the property the registry enforces: one number per line, one line per card, a `check` finding when two lines claim one number, and a repair that restores uniqueness by reallocating. Under the round 3 wording, the first operator to run that repair would have put Dinah outside its own CORE-CARD-10, which is why the entry rather than the statement was the thing that had to change.

**Section 5.3 gains one statement**, after `CORE-CARD-9` (`:550`). This is the exact text that goes into the document, on a single line beginning at column one with no blockquote marker and nothing else on the line, which is the form section 3.2 fixes and the form every statement from `:534` to `:550` already takes:

```
[CORE-CARD-10] Every card MUST carry a creation ordinal unique within its workbench.
```

That sentence is `CORE-CARD-1`'s shape with one noun changed. `CORE-CARD-1` (`:534`) reads "Every card MUST carry an identifier unique within its workbench", and `CORE-STATE-1` (`:485`) and `CORE-STATE-10` (`:505`) take the same shape for a column's identifier and its slug. It carries exactly one keyword, which section 3.2 requires, and it says both halves of what this card needs: that a card has a creation ordinal at all, and that no other card in its workbench carries the same one. It uses no word from either excluded list in section 3.5, and `creation ordinal` is the noun section 4 gains above, so section 3.5's closed-vocabulary rule is met rather than assumed.

**Nothing in the tree reads that sentence for meaning, so it has to be right on the way in.** The previous round of this spec proposed "No two cards in one workbench MUST carry one creation ordinal", which denies an obligation rather than forbidding a shared ordinal, and no test would have caught it. `internal/profile/extract.go` rules on shape alone: one line, the bracketed identifier, exactly one keyword, and an identifier unique in the document. `TestHouseStyle` (`internal/profile/extract_test.go:575`) bans dashes and smart quotes, and `TestProfileExtractsCleanly` (`:177`) holds the extraction. None of them reads English. `DOC-CHG-1` forbids editing the sentence once the revision publishes, so an implementer who finds a better wording raises it before the entry lands and not after.

**Section 10 gains a row** and its tally moves. The concept is the ordinal's uniqueness, which is not the concept the waiting-order row carries, and section 10 rules one concept per row. `TestBoundaryTableRulesEveryStatement` requires the new statement to appear against exactly one row, and `TestTheBoundaryTallyCountsTheRowsTheTableCarries` reads the sentence at `:1298`, so `Rows ruled in: 36` becomes 37 and `Total rows: 58` becomes 59.

**Section 11 gains an index row** beside the other CORE-CARD rows (`:1444` to `:1450`) and its tally moves. The row's observable-outcome column states the check a conformance run makes, in the voice the other rows use. `The index carries 138 rows` at `:1554` becomes 139, which `TestTheIndexCountCountsTheStatementsTheDocumentPublishes` and `TestIndexMatchesTheExtraction` both read.

**The changelog gains a 0.14 entry**, and this is where AC-2's first half is answered. The entry records CORE-CARD-10 as introduced, records that nothing is retired, and states the consequence for a caller: a tool whose workbench carries two cards with one creation ordinal is not conformant at 0.14 and was not evaluated against the question at 0.13. DOC-CHG-2 (`:174`) fixes what an entry must carry, and the increment is classified by DOC-VER-11 and DOC-VER-8 (`:145`, `:153`), which is a minor increment because the document's major is still 0.

The entry also carries the correction to the false claim, because there is nowhere else to put it. `DOC-CHG-1` (`:172`) reads "A published changelog entry MUST NOT be edited or removed", and the false parenthetical sits at `:1596`, inside the published 2.0 entry (heading `:1586`), where it calls `number` a field the interchange form already carries. The interchange form carries columns and no cards at all, as 5.7 shows and as `internal/bench/interchange.go` implements. The new entry names the entry it corrects and the sentence that is wrong, and the 2.0 entry stays byte-identical.

**Nothing inside a published entry is edited, and that includes the revision numbers they carry.** This is where the previous round of this spec broke the rule it had just cited, so it is stated as a rule with its own criterion rather than as a list of sites. The document names its current revision in five present-tense places and nowhere else, and `TestAllOfTheProfilesRevisionStatementsAgree` (`internal/profile/amendment_test.go:174`) enumerates exactly those five: the header (`:3`), section 2's version sentence, section 2.1's conformance-claim example (`:95`), section 5.7's interchange-form example (`:626`), and the sentence at the head of section 12 (`:1559`). Those five move to 0.14 together and an increment moving fewer than five reddens that test.

Every other occurrence of a revision number in the document names a past event and stays exactly as it is. That test's own doc comment says so for the 0.7 occurrences it deliberately does not read, and the same reasoning covers the consequence-for-a-caller prose inside each published entry: the 0.12 entry's sentences at `:2015` and `:2026` say what a caller conforming to 0.12 was entitled to, the 0.13 entry's at `:2058` and `:2064` say the same for 0.13, and all four are historical record. Editing any of them would violate DOC-CHG-1 and falsify the record in one stroke.

**`internal/profile/amendment_test.go` moves with the document**, and it is a fifth file rather than an afterthought: `declaredVersion` (`:28`) and `declaredCurrentRevision` (`:40`) take 0.14, `publishedStatements` (`:30`) goes 138 to 139, and `publishedChangelogEntries` (`:33`) goes 13 to 14.

**The build's conformance claim does not move, and neither does anything downstream of it.** `ProfileMinor` stays 12 (`internal/bench/bench.go:106`). The build already conforms to a revision older than the published one, and the profile permits that: section 2 says the version of the profile is unrelated to the release numbering of any tool, and `amendment_test.go`'s own comment at `:165` records the operator's ruling that no guard may require the two to agree, because a build may conform to an older revision than the one published. Moving `ProfileMinor` to 14 would claim conformance with 0.13's CORE-JSON-10, CORE-GATE-3 and CORE-GATE-4 as well as with 0.14's CORE-CARD-10, and this card implements one of those four and evaluates none of the other three. Moving it to 13 would claim the three this card does not implement and would not claim the one it does.

Three things follow, and each is a thing this card does not do:

- The compatibility window is untouched. `internal/bench/compat_test.go:85` keeps its admitted list ending at `dinah-core/0.12`, and keeps `dinah-core/0.13` in its refused list, where it is still the example of this build's own major at a minor above the ceiling it implements. `cmd/dinah/compat_test.go:730` keeps reading `through dinah-core 0.12`.
- No compatibility fixture is captured. `scripts/capture_fixture.py:47` names the fixture directory from the binary's own conformance claim, and that claim has not moved, so `internal/bench/testdata/compat/dinah-core-0.12` stays the sample and `TestSomeFixtureDeclaresTheRevisionThisBuildStamps` (`internal/bench/compat_test.go:502`) stays green without one. Do not run the capture script on this card; running it would rewrite the existing 0.12 fixture and fail `TestTheFixtureManifestMatchesWhatIsCommitted`.
- The `dinah-core/0.12` strings in test fixtures and in `docs/quick-start.md` stay where they are. They record what this build claims, and this build's claim is unchanged.

Whoever bumps the build's claim next reckons with 0.13 and 0.14 together, captures one fixture for whatever revision that lands on, and moves the window then. That is a separate card and this spec does not write it.

**The frozen fixtures are not re-blessed and not touched.** Their cards go on carrying `number` in frontmatter at storage format 2, which is exactly the state the legacy read path exists for, and `TestEveryCompatFixtureOpensAndReads` passing over them unchanged is the compatibility claim this card is making. The storage format and the profile revision are separate numbers with separate audiences, as `docs/design/format.md:1894` says, and the fixture set is keyed on the profile revision, so a storage-format bump adds no fixture.

## The documents

**`docs/design/format.md`.**

- The layout diagram at `:110` gains `card-numbers.txt` beside `journal.ndjson` (`:114`), with a one-line gloss.
- A new subsection under the card sections states the registry: what it holds, its grammar, why the number left the card anchor, and that the hex is the identity while the number is a label.
- `:381` and `:1303` both say a card's creation ordinal is the `number` it was born with. Both are amended to say the ordinal is the number the registry allocates the card, which leaves CORE-QUEUE-3's tie-break reading correctly.
- `:1306` says the ordinal is unique within one collection instance and nowhere wider. The card's own ordinal is now unique across the workbench, which includes both halves of the collection, so the sentence is split: a below-card ordinal keeps the collection-instance scope, and a card's number is workbench-scoped.
- The merge story is where AC-2's second half is answered. The paragraph at `:2152` reasons about one card edited twice and calls a frontmatter conflict "real contention, rare, and resolved by a human" at `:2160`. It gains the case it does not cover: two clones each filing a new card from one base write disjoint directories, so git merges both with no conflict at all, and a fact that has to be unique across the workbench therefore cannot live in a per-card file. The registry is named there as what turns that case into a conflict, and the section says plainly that the conflict is wanted.
- The concurrency section at `:1993` gains the registry's lock: a filing now takes the workbench lock, which the section's own "creating an entity needs no separate lock" sentence at `:2001` has to be qualified against.
- The corruption section at `:2281` gains the registry's own recovery story: the file is plain text, a damaged line is reported rather than guessed at, and the two automatic repairs are named.
- The versioning section says at `:1904` that the storage number has moved once, from 1 to 2. It has now moved twice.

**`docs/quick-start.md`.** The `dinah edit rel-1` sample at `:1273` shows a card anchor carrying `number: 1`, and that anchor no longer has one. The four `dinah-core/0.12` strings stay, because the build's claim is unchanged. The passage at `:855` is about a workstream slug a second workstream of the same title would collide on, which is a different collision from this card's and needs nothing.

**`internal/guide/guides/references.md`.** The guide tells a reader that Dinah resolves a card on its number rather than its prefix (`:49`), and the section headed "The number and the identifier" at `:111` is where the accepted cost belongs in the reader's own words: the identifier is the reference that survives a merge, and a number can change when two clones meet. The guide says nothing today about where the number is stored, so nothing there goes stale.

## The catalogs

Eight finding and event keys land in `internal/msg/locales/en.json`: the six `check.card-number-*` findings, `check.card-number-renumbered`, and `token.renumbered`, plus the two refusal keys `refusal.dinah.needs-number-migration` and its `.next`, plus the repair's own summary lines with `.one` and `.other` plural forms on the model of `check.ordinal-stamped`.

Every other catalog needs each key present, because `TestEveryDeclaredLanguageShips` (`internal/msg/msg_test.go:17`) fails when a tag's coverage reports `present != total` (`:30`). Hindi and German are on `msg.Complete` and take real translations carrying a `source` fingerprint computed by `msg.Fingerprint` over the English text, which the same test holds to `translated == total`. Czech, Indonesian, Spanish, Filipino, and Afrikaans are on `msg.Skeleton` and take the English text verbatim with `"skeleton": true` and no `source`, which the same test holds to `translated == 0` and which `TestASkeletonEntryReallyCarriesTheEnglishText` (`:746`) checks the other way.

## The audit of the criteria

This workbench holds a criterion to four questions, and round 4 added the fourth after three rounds each found a check that could not fail or could be walked around. The answers are recorded here rather than left for a reader to re-derive.

**One: does the criterion pass on the tree it ships on?** AC-11's arithmetic was re-read off the tree rather than carried: section 12 carries thirteen entry headings, `publishedStatements` is 138, `publishedChangelogEntries` is 13, the section 10 tally at `:1298` reads 36 ruled in and 58 total, and the section 11 sentence at `:1554` reads 138 rows, so 0.14, fourteen, 139, 37, 59, and 139 are what the amendment writes. AC-14's boundary was established by grepping every entry heading: `### 1.0` stands at `:1566`, `### 2.0` at `:1586`, and the last at `:2028`, so 1566 is the first and the criterion's region covers the whole published span. AC-13's seven commands were run against a built binary on 2026-09-11, and the expected set the criterion names for each command is the set that run produced. AC-12's first test, that `unionJournals` names no pattern covering the registry, passes before and after this card, which is a regression guard doing its job rather than a check that cannot fail.

**Two: does the criterion's collection rule agree with the set it asserts?** AC-12's scan declares every reference to the three free card readers declared in `internal/bench`, and the prototype run reports thirty-two of them in nineteen files, which is the set the allowlist is written against. Three shapes are correctly outside it: `LoadCardIn` and `loadRetiredCardIn`, because a name is compared whole, and the string literal `"LoadCard"` in the exemption roster at `internal/addressform/addressform.go:370`, because it is an `*ast.BasicLit`. AC-13 collects card references as `fx-<digits>` on a whole run of digits with a non-word character before it, so a hex identifier, a date, and a per-column count are outside the set by construction, and it collects bare hex identifiers separately on the same anchoring. Both collections were run and both reported exactly what the criterion says they report. AC-8 declares six findings each firing on exactly the state it names, and a malformed registry line is excluded from the stranded check so that one damaged line draws one finding rather than two.

**Three: can the fixture reach every path the criterion claims to guard?** AC-6, AC-8, and AC-9 each name a handwritten fixture per condition, built with the helpers at `internal/bench/check_test.go:149`, because the tool refuses to mint the states they test. AC-12's third test plants a violating file outside the tree and asserts the scan reports it, on the `TestTheKindGuardGoesRed` model. AC-4 plants two branches and merges them. AC-13's setup was the one that could not reach its own state: `dinah claim` on a card standing at an intake column is refused with `dinah.takes-no-work`, which was confirmed by running it, so the setup now carries the card to the work column with `carryToDoing` (`cmd/dinah/main_test.go:248`) first, and plants a checklist item with an unresolvable column so that `check` has something to report. AC-14 is the one criterion with no plant, because it asserts that a diff is empty in a region: its red state is any edit at all inside the region, and an implementer who wants to see it red edits one character inside a published entry and re-runs the diff. AC-5, AC-7, AC-10, and AC-11 assert behaviour over a built state rather than arming a detector, so the question does not reach them.

**Four: can somebody do the forbidden thing while still satisfying the criterion?** This is the question rounds 2, 3, and 4 all missed, and it is answered by writing the hostile code rather than by reading the check. AC-12 has now been attacked with twenty-three shapes in twelve planted directories under a prototype of its two rules. Seven of the twenty-three were written by this card's reviewers against the prototype rather than by its author, and five more were written by its author in round 6 against the repair round 6 opened with. The corpus includes round 4's own defeat verbatim, that defeat rewritten with a `var` binding, three ways of carrying the card out inside a composite literal, a named result answered by a bare `return`, an alias read through `&`, through parentheses, and through a dereference, and a method on the card declared where rule 2 does not look. All twenty-three are reported by the rules as this round leaves them, and the shape rounds 4 and 5 admitted as permitted is one of the twenty-three. Rule 2 took two repairs in round 5 and four more in round 6 to reach that state; D-19 records the first pair and D-20 the rest. What the corpus does not establish is completeness, which is said plainly in the guard section rather than implied here. AC-13 was attacked by deleting the `number:` key from one card's anchor, which is the state a call site reading through a free reader produces, and both halves of its assertion redden. AC-14 can be satisfied while editing published text only by editing above line 1566, which is section 11 and below, and nothing this card does touches that region. AC-9's forbidden act is a repair that reorders the registry, and the criterion asserts that every line other than the repaired one stands at the same index afterwards, so a repair that rebuilt the file in number order fails it. AC-3's forbidden act is a filing that writes a number into frontmatter as well as into the registry, and the criterion asserts the anchor carries no `number:` key, so a belt-and-braces implementation fails it rather than passing twice.

## Whether this card is two cards

Five review rounds have each found a different check that could not fail or could be walked around, and this column's instructions say a card failing repeatedly on one class of defect is telling you about its shape. The question the reviewer should rule on is whether the code change and the profile publication belong on one card.

**What the two halves are.** The first is the registry: the file, the format bump, the read path, the allocator, the migration, the six `check` findings, the repair, the guard, and the amendments to `docs/design/format.md`, `docs/quick-start.md`, and the references guide. The second is one act on one document: publish `dinah-core 0.14` carrying CORE-CARD-10 and the creation-ordinal entry, move the two tallies and the five revision sites, correct the 2.0 entry's false claim about the interchange form in the new entry, and move the four constants in `internal/profile/amendment_test.go`.

**Neither half needs the other.** The profile half touches `docs/spec/core-profile.md` and one test file, and it reads nothing the registry produces. The code half is untouched by whether the document says anything, because D-13 keeps `ProfileMinor` at 12 either way, so no build asserts conformance with 0.14 on either card. Somebody could work the halves in either order, or at the same time, and the only care needed is that the profile half is not worked beside another profile card, which is already true of it on its own.

**The reason to split them is that the profile half rots while it waits.** Round 1 of this spec proposed 0.13. dinah-450 published 0.13 on 2026-09-09, while this card sat in design, and round 3 had to re-establish the revision, the statement count, the entry count, and both tallies against a moved tree. Every number the profile half asserts is contended by every other profile card, and this card cannot ship them for as long as it takes to build a storage-format migration. The profile half is an afternoon; the registry is not. DOC-CHG-1 also makes the published entry unamendable, which is a reason for it to get a reviewer's whole attention rather than the last page of a spec about a text file at the workbench root.

**The reason not to split them is weaker than it looks.** They are one idea, which is that a card's number is unique in its workbench. Publishing the rule before the tool keeps it is the normal direction for a specification: the profile constrains the model rather than a tool, and the 0.14 entry's own consequence sentence says what a caller conforming to 0.14 is entitled to, which is a sentence written for tools that do not do it yet. It is also fair to say that splitting would not have prevented any of the five push-backs, since the defects were spread across both halves rather than concentrated in one. A split changes the review surface, not the review quality.

**Round 5 added a fact to the question rather than answering it.** The one defect that round found that would have shipped wrong is in the profile half: the section 4 entry used a word the profile's own excluded list bars from that section, so the criterion asserting `go test ./internal/profile` passes could not have passed. The registry half survived every hostile shape written against it that round, and the two repairs the guard took were repairs to the evidence rather than to the design. Across five rounds the two halves have failed for unrelated reasons every time. The reviewer's own recorded view is that the argument to split is sound, that the split's real benefit is a shorter contention window on the numbers every other profile card also moves, and that the profile half now needs a review round of its own in any case, because its text has changed. None of that answers the question, which is the operator's.

**Round 6 added a second fact and answers the question no more than round 5 did.** The only defect round 6 found is in the guard, which is the third consecutive round in which the guard is the only thing wrong with this card. The guard lives entirely on the registry side. It is also the part of the card a reviewer has to attack rather than read, and attacking it is what has cost the last three rounds. The profile half has been correct and stable since round 5 fixed its excluded term, it moved not at all in round 6, and it is the half that goes stale while it waits, because every number it asserts is contended by every other profile card on this workbench. So the two halves now differ in the shape of their remaining risk as well as in their subject: one half attracts defects and the other accrues staleness. That is a fact for the operator to weigh and not a ruling, and this spec has neither answered OQ-1 nor acted on it.

**The recommendation is to split, and it is the operator's call rather than mine.** OQ-1 carries it. This spec is written to work either way: if the split is approved, the profile amendment section and AC-11 and AC-14 lift out whole onto the second card with no edit to anything else, and this card's deliverable list loses exactly those three things. If it is refused, nothing here changes. The implementer is not waiting on the answer either way, which is why it is an open question rather than a block.

## What this card does not do

It does not make numbers dense or sequential across clones, which needs an arbiter. It does not change the hex identifier, how it is minted, or the path layout. It does not touch the ordinals on comments, attachments, and checklist items, which are scoped to one card and are already covered by `check.ordinal-duplicate`. It does not widen `checkOrdinals` to the archived half. It does not move the build's conformance claim, the compatibility window, or any compatibility fixture. It does not add a number to the interchange form, which carries no cards. It does not change `splitRef` (`internal/bench/resolve.go:93`), so a reference still cuts at its last dash, still keys on the number alone, and still accepts a stale prefix with `Resolved.StalePrefix` set.

## Sequencing and conflict

D-1 holds this card at Implement until dinah-487 merges to trunk. D-4's hold is discharged: dinah-439 is on trunk at `c1ae4a9` and this spec is written against it.

## Branch

dinah-488-card-number-registry
