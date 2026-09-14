---
title: A card number that names two cards resolves silently to one of them
column: b69abf918c42
state: ready
severity: major
priority: next
tier: workhorse
workstreams:
  - 994787601ae6
---
## The defect

A card's human reference, the `dinah-99` form, is resolved by number alone. `resolve.go` walks the cards and returns the first whose `Number` matches. There is no second-match detection and no refusal. If two cards carry the same number, `dinah show dinah-99` opens whichever the filesystem lists first, and nothing tells the caller that a choice was made.

This is reachable today. The number lives in each card's own frontmatter and is minted by scanning for the highest in use, so two clones of a workbench that each file a card from the same base both write `number: 99` into different directories. Git merges both cleanly, because the paths differ. Nothing in `dinah check` looks for the result, and the profile constrains only the hex identifier's uniqueness, not the number's.

The full facts, established from the code on 2026-09-11, are on dinah-488's description. This card is the small half.

## What this card does

Resolution by number refuses when more than one card carries it, naming the candidates by hex so the caller can pick one. A number that names one card resolves as today. A number that names none refuses as today.

That is the whole card. It converts a silent wrong answer into a loud right one, and it is correct under every storage scheme, including the registry dinah-488 introduces, because it constrains what resolution may do rather than where the number lives.

## What this card does not do

It does not detect duplicates on a health read, does not repair them, and does not change where the number is stored. All of that is dinah-488, which moves the number into a registry file and adds the check and the repair against it. Doing the check here against frontmatter would mean writing it twice.

It does not change the hex path. A raw identifier resolves exactly as it does today, which dinah-471 established as the always-available name.

## Why it ships ahead of the larger card

The registry work is a storage format change with a profile amendment and a migration, and it will take its time. This refusal is a few lines, needs no format change, and closes the actively dangerous behaviour now. A workbench that never sees a duplicate loses nothing; one that does gets a refusal instead of a wrong card.

## Profile

`CORE-CARD-1` says every card carries an identifier unique within its workbench, and the glossary defines identifier as the hex. This card adds no statement and amends none. Whether resolution by a non-unique label should be a profile statement at all is dinah-488's question, since that card decides what the label is.

## Specification

## What the code does today

Every line number below was read at `c1ae4a9`, which is `origin/main` on 2026-09-11, in a worktree at `C:/dinah-scratch/dinah-487-spec/wt`. Read the tree at `origin/main` rather than at a local `main`, which in the operator's checkout is two commits behind and carries no `dinah-439`. Trunk moved during this card: `dinah-439` landed, so the line numbers in the first two review comments no longer hold and the ones here supersede them. Two things `dinah-439` changed matter to this card. `dinah check` now carries a duplicate-number finding, `checkCardNumbers` at `internal/bench/check.go:811`, which detects the state this card refuses over and repairs nothing. And `cardsWith` (`internal/bench/bench.go:2085`) now fails on a card it cannot read rather than skipping it, so `cardsIn` returns an error where it used to return a short slice. Neither changes the design below.

`(*Bench).resolveCardIn` in `internal/bench/resolve.go:44` is the whole of card resolution, and both `ResolveCard` (line 29) and `ResolveArchivedCard` (line 37) are one-line calls to it differing only in which root they hand it. A 12-hex reference is served by `LoadCard` at line 50. Everything else goes to `splitRef`, which is called at line 69 and declared at line 93, and which cuts at the last dash and keys on the trailing number alone, and then to the scan at lines 77 to 86:

```go
for _, card := range cards {
	if card.Number != number {
		continue
	}
	found := &Resolved{Card: card}
	if prefix != "" && prefix != b.Slug {
		found.StalePrefix = prefix
	}
	return found, nil
}
return nil, contract.Refuse(contract.UnknownCard, ref)
```

The loop returns on its first hit and never looks at the rest of the collection.

One correction to the card's description. The wrong answer is not whichever the filesystem happens to list first, it is deterministic and it is the lowest identifier. `cardsWith` walks `ListIDs` (`internal/bench/storage.go:182`), which returns the entries `os.ReadDir` gives it, and `os.ReadDir` is documented to sort by filename. Two cards carrying one number therefore always resolve to the same one of the pair, which is worse rather than better: an intermittent wrong answer gets noticed, and a stable one does not.

The format document already forbids the state this card refuses over. `docs/design/format.md:1306` says the creation ordinal "is unique within one collection instance and nowhere wider", and a card's ordinal is its `number` by the same passage. So this card enforces a rule the format already states and nothing at resolution ever checked, rather than accommodating a legal shape. Nothing in that document, or anywhere else under `docs/`, claims a number resolves to the first match, so no document carries a sentence this change falsifies.

## The contract

Resolution by number answers a card when exactly one card of the half being read carries the number, and refuses when more than one does, naming every candidate by its 12-hex identifier. A number no card carries goes on refusing `unknown-card` exactly as it does now, and a 12-hex reference goes on resolving by the path at line 50, untouched.

The ambiguity is a refusal and not an empty result. `resolveCardIn` has no empty-result contract to extend, since every arm of it either answers a card or returns an error, and an absence returned here would reach every caller as the card not existing. That is the same silent wrong answer in different clothing. The two sibling ambiguities in this codebase, `dinah.ambiguous-workbench` (`internal/contract/contract.go:151`) and `dinah.ambiguous-column` (`internal/contract/contract.go:446`), are both refusals, and both refuse rather than guess for this reason.

The refusal must also survive the journey to the caller. A caller who asks a question about an ambiguous number is entitled to be told the number is ambiguous, rather than to be told the card does not exist, and rather than to be handed one of the two cards. Two call sites currently break the second half of that and are changed below.

## Changes to land

### 1. The refusal name

Add to `internal/contract/contract.go`, in the block of names Dinah mints beyond the profile's own, beside the other ambiguity names:

```go
// AmbiguousCard is a card reference whose number more than one card of the
// half being read carries, raised before the resolution picks one of them.
// The candidates ride as a carried set of identifiers, because an
// identifier is the address every card always answers to and is what the
// caller retypes to get past this.
AmbiguousCard = LayerPrefix + "ambiguous-card"
```

Add `AmbiguousCard` to `Introduced` (`internal/contract/contract.go:539`), which carries 70 names today and 71 after. Three guards read that slice and fail without the entry: `checkOneRefusalIsOneDeclaration` in `internal/profile/guards_test.go:2961`, the catalog-key sweep at `cmd/dinah/main_test.go:1462`, and `TestOnlyTheAmbiguousRefusalCarriesACandidateWalk` at `cmd/dinah/refusal_walk_test.go:216`, whose subtest count moves by one.

That last test's doc comment states the count in prose, and the prose is already wrong: it says "59 names on this build and so 58 subtests" while `Introduced` holds 70. Correct it to 71 and 70 in the same edit. A count nothing checks drifts, and leaving a figure that is wrong by eleven in a comment this card is already editing teaches the next reader that the comment is decoration.

### 2. The shape

Add to `contract.Shapes` in `internal/contract/shape.go`, modelled on the `AmbiguousColumn` entry at line 1035:

```go
{
	// A card reference's number is carried by more than one card, so the
	// resolution refuses rather than answering with the lowest
	// identifier. The candidates ride as a Carried set rather than a
	// Listing, because the set depends on the reference that was typed
	// and no enumerable listing names it. The next step tells the reader
	// to retype one of the identifiers the rows carry.
	Name:      AmbiguousCard,
	Carried:   "cards",
	Fragments: []Fragment{{Key: "refusal.dinah.ambiguous-card.next"}},
	NextStep:  []string{"refusal.dinah.ambiguous-card.next"},
},
```

`Carried` rather than `Values` is the choice worth stating. `dinah.ambiguous-name` interpolates its matches into the sentence (`internal/contract/shape.go:944`), and that suits it because an attachment ordinal is a one-character token. The two refusals that name candidate addresses both draw rows instead, and a 12-hex identifier is an address the reader retypes, so this one follows them.

### 3. The catalog entries

`internal/msg/locales/en.json` takes two new keys:

```json
"refusal.dinah.ambiguous-card": {
  "text": "more than one card answers to {detail}",
  "context": "The opening sentence printed after the refusal name dinah.ambiguous-card, naming the reference the reader typed. The candidate rows print beneath it, one 12-hex card identifier per row, and refusal.dinah.ambiguous-card.next follows as the closing line naming the way forward."
},
"refusal.dinah.ambiguous-card.next": {
  "text": "name one by the identifier above",
  "context": "The line printed after dinah.ambiguous-card's candidate rows, naming the way forward."
}
```

The next-step fragment carries no leading separator, which is correct and is not an oversight. `TestEverySplicedFragmentCarriesItsOwnSeparator` (`cmd/dinah/main_test.go:8355`) skips a shape declaring `Carried`, because that shape's fragment renders as a line of its own beneath the rows and a leading semicolon there would be the mistake.

All seven other catalogs need both keys present, because `TestVersionCarriesTheConformanceClaim` (`internal/verb/beyond_test.go:605`) fails on `Present != Total` at line 645 for any catalog. Hindi and German are on `msg.Complete` (`internal/msg/msg.go:142`) and take real translations carrying a `source` fingerprint computed by `msg.Fingerprint` over the English text, since the same test fails at line 649 on any complete catalog whose `Translated` is short. Czech, Indonesian, Spanish, Filipino, and Afrikaans are on `msg.Skeleton` (line 145) and take the English text verbatim with `"skeleton": true` and no `source`, since line 652 fails a skeleton catalog carrying any translation at all.

### 4. The raise site

Replace the first-match return in `resolveCardIn` with a collect-then-decide, leaving both unchanged cases byte-for-byte identical in behaviour:

```go
var matches []*Card
for _, card := range cards {
	if card.Number == number {
		matches = append(matches, card)
	}
}
switch len(matches) {
case 0:
	return nil, contract.Refuse(contract.UnknownCard, ref)
case 1:
	found := &Resolved{Card: matches[0]}
	if prefix != "" && prefix != b.Slug {
		found.StalePrefix = prefix
	}
	return found, nil
}
ids := make([]string, 0, len(matches))
for _, card := range matches {
	ids = append(ids, card.ID)
}
return nil, contract.RefuseWith(contract.AmbiguousCard, ref, map[string]string{
	"cards": strings.Join(ids, "\n"),
})
```

Four properties of that placement are the point of it.

The refusal sits in the one function every card reference reaches, so neither head can be given the rule and the other left without it. `resolveCardIn` is called from `resolveReferenceBody` (line 422), `resolveBelowLanding` (line 599), `probe` (lines 1187 and 1189), and the two exported wrappers, and every command of both heads arrives through one of those. The CLI's own direct resolver calls are `ResolvePathIn` at `cmd/dinah/commands.go:1227` and `ResolveEditTarget` in `runEdit`, which both descend into the same function.

Nothing else in the tree resolves a card by its number. Nine non-test functions read a card's `Number` field at all, in fifteen mentions across thirteen lines of five files, and `resolveCardIn` is the only one of them that turns a number into an answer to a reference. `resolveCardIn` (`resolve.go:78`) is the scan above. `NextNumber` (`bench.go:2134` and `:2135`) takes the highest in use while minting. `checkCard` (`check.go:636`) and `checkCardNumbers` (`check.go:836`) guard a finding against a card carrying no number, and `checkCardNumbers` also groups by number at `check.go:848`. `Ref` (`card.go:404` and `:407`) composes the human reference, `Save` (`card.go:415` and `:416`) writes the number back to frontmatter, `loadCard` (`card.go:194`) parses it in, and `ByArrival` (`card.go:577`) sorts on it. `jsonNumber` (`blockjson.go:508`) is a false positive of the field name rather than a reader of a card: the selector there is the type `json.Number`. Section 7's family C asserts that set, so a hand-rolled second scan cannot appear quietly.

The rows are in ascending identifier order without a sort call, because `cardsWith` preserves the order `ListIDs` gives it and `os.ReadDir` documents sorting by filename. The test named in AC-4 asserts the order, so a later change to that walk fails a check rather than quietly reordering a refusal.

Every match is named and none is dropped. There is no cap and no truncation rule, because a truncation would hide the candidate the caller was reaching for, which makes the refusal unactionable in exactly the case it most needs to be actionable. The set is bounded by the cards in one workbench either way.

The carried rows are identifiers alone, with no title beside them. The renderer that draws a carried set builds a one-column table (`cmd/dinah/render.go:1016`, inside `composeRefusal` at line 987), so a title would need a second column and a renderer change, and the identifier is the actionable half in any case. `dinah-471` fixed the raw identifier as the address a card always answers to, so a reader handed one can act on it without a second lookup.

Adding one return statement to this function moves its arm count. `internal/addressform/addressform.go:269` declares `Returns: 8, Accepting: 2` for `resolveCardIn`, and `cmd/dinah/address_form_arms_test.go` recomputes both columns from the AST on every run. Set `Returns: 9`. `Accepting` stays 2 and `Forms` is unchanged, because a refusal is not an accepting arm and this card serves no new address form.

### 5. Two callers that read both halves and would swallow the refusal

This section covers the call sites that try the live half, fall through to the archive when the live attempt returns any error, and would therefore step past the new refusal. There are three such sites in the tree. Two of them are changed here, and the third is `probe`, which is argued under "What is deliberately unchanged" below. How that set was established is in section 6.

#### 5a. `ResolveLinkTarget`

`ResolveLinkTarget` (`internal/bench/resolve.go:913`) is the write-time resolution a link target takes, and it reads both halves because a link may legally name an archived card:

```go
if found, err := b.ResolveCard(raw); err == nil {
	return found.Card.ID, nil
}
if found, err := b.ResolveArchivedCard(raw); err == nil {
	return found.Card.ID, nil
}
return "", contract.Refuse(contract.UnknownCard, raw)
```

Left alone, a number ambiguous among live cards falls past the refusal and, where the archive holds a single card carrying that number, records the link against the archived card. That is this card's own defect reintroduced on a write path. Where the archive holds no such card, the ambiguity is discarded and replaced with `unknown-card`, so the caller is told the card does not exist.

Both attempts get the same unwrap, so an ambiguity in either half is reported as an ambiguity in that half:

```go
found, err := b.ResolveCard(raw)
if err == nil {
	return found.Card.ID, nil
}
var refusal *contract.Refusal
if errors.As(err, &refusal) && refusal.Name == contract.AmbiguousCard {
	return "", refusal
}
found, err = b.ResolveArchivedCard(raw)
if err == nil {
	return found.Card.ID, nil
}
if errors.As(err, &refusal) && refusal.Name == contract.AmbiguousCard {
	return "", refusal
}
return "", contract.Refuse(contract.UnknownCard, raw)
```

The `errors.As` form is the one this package already uses for exactly this question, at `internal/bench/resolve.go:62` inside `resolveCardIn` itself. No helper is added and no new exported surface appears in `internal/contract`, because two call sites do not pay for one.

This moves the function's arm counts. `internal/addressform/addressform.go:290` declares `Returns: 5, Accepting: 3` for `ResolveLinkTarget`. The body above has seven returns and the same three accepting arms, since `armCounts` (`cmd/dinah/address_form_arms_test.go:72`) counts an arm as accepting only when its first result is not the literal `nil` and its last result, on a return of more than one value, is the literal `nil`, which the four `return "", <refusal>` arms are not. Set `Returns: 7` and leave `Accepting: 3`. `addressform.go` is the only file in the tree declaring `Returns:`, so nothing else keys on the old figure.

The refusal reaches the caller with its rows intact on this path as well. `linkArguments` (`internal/verb/link.go:139`) passes `refusal.Name`, `refusal.Detail`, and `refusal.Extra` to `l.refuseWith` at line 160, so the carried identifiers travel rather than being dropped at the verb boundary.

#### 5b. `watchedCard`

`Library.watchedCard` (`internal/verb/changes.go:400`) resolves the `--card` filter of the `changes` verb on both heads, and it has the identical shape:

```go
if found, err := l.Bench.ResolveCard(ref); err == nil {
	return found.Card.ID, nil
}
if found, err := l.Bench.ResolveArchivedCard(ref); err == nil {
	return found.Card.ID, nil
}
if trimmed := strings.TrimSpace(ref); bench.IsID(trimmed) {
	return trimmed, nil
}
return "", contract.Refuse(contract.UnknownCard, ref)
```

With the raise site alone, a number two live cards carry, where the archive also holds a card carrying it, keys the watch on the archived card's identifier and says nothing, so the caller watches a card they did not name. Where the archive holds no such card, the identifier arm rejects the reference too and the ambiguity becomes `unknown-card`. The function's own doc comment makes the promise the fall-through then breaks: "Anything else still refuses UnknownCard, so a mistyped reference is caught rather than answered with silence."

The third arm here is not a fall-through of the same kind and stays exactly as it is. A well-formed identifier that resolves in neither half is accepted on purpose, because a removed entry in `gone` carries an identifier and nothing else, and an identifier is never ambiguous.

Unwrap both resolutions, the same way and for the same reason:

```go
found, err := l.Bench.ResolveCard(ref)
if err == nil {
	return found.Card.ID, nil
}
var refusal *contract.Refusal
if errors.As(err, &refusal) && refusal.Name == contract.AmbiguousCard {
	return "", refusal
}
found, err = l.Bench.ResolveArchivedCard(ref)
if err == nil {
	return found.Card.ID, nil
}
if errors.As(err, &refusal) && refusal.Name == contract.AmbiguousCard {
	return "", refusal
}
if trimmed := strings.TrimSpace(ref); bench.IsID(trimmed) {
	return trimmed, nil
}
return "", contract.Refuse(contract.UnknownCard, ref)
```

`internal/verb/changes.go` does not import `errors` today, so the import block at lines 3 to 10 takes it. Amend the doc comment above the function as well, since it currently states the refusal behaviour that is changing: an ambiguous number now refuses `dinah.ambiguous-card` rather than resolving or refusing `unknown-card`.

`internal/verb` carries no declared arm counts, so nothing in `internal/addressform` moves for this site.

### 6. The sweep for this shape, and what else it found

The defect section 5 closes was found twice by reading, once at `ResolveLinkTarget` and once at `watchedCard`, so the question is whether reading found all of it. It was settled by searching for the shape rather than for the two names, and the searches are written out here so a reviewer can judge the sweep rather than the result. Each was run from the worktree root.

1. `grep -rn "ResolveArchivedCard(" --include=*.go . | grep -v _test.go` Two non-test call sites exist, `internal/bench/resolve.go:924` and `internal/verb/changes.go:404`, plus the declaration itself at line 37.
2. `grep -rn "ArchivedHalf\|ArchivedCardsRoot()\|cardsRootIn(" --include=*.go . | grep -v _test.go` This is the wider question, because the archived half is also reachable without going through `ResolveArchivedCard`. The only additional number-capable route it finds is `probe` at `internal/bench/resolve.go:1189`, which calls `resolveCardIn` against the archived root directly. The remaining sites either take the half from a flag and propagate the error (`internal/verb/read.go:828`, `internal/verb/beyond.go:286`, `cmd/dinah/commands.go:1224`), or read a directory rather than resolve a reference. This search is not exhaustive over routes into the archived half, and search 6 is here because it is not.
3. `grep -rn "err := .*; err == nil" --include=*.go . | grep -v _test.go` This finds the textual shape itself wherever it occurs, which is what a search keyed on the two known names cannot do. Of the sites it returns, three resolve a card by number and are the three already named.
4. `grep -rn "ResolveCard(" --include=*.go . | grep -v _test.go` Every call site read one at a time. Most propagate the resolver's error unchanged. The rest are the two fixed here and the flattening sites discussed below.
5. Every read of a card's `Number` field in non-test code, read one at a time, which keys on the mechanism rather than on the archive. The nine functions it returns are listed in section 4, and `resolveCardIn` is the only one that resolves a reference. This is wider than a search for a comparison, and the width earns its keep: `checkCardNumbers` groups cards with `byNumber[card.Number] = append(...)` at `check.go:848`, which is a number scan carrying no comparison at all, so a search for `.Number ==` or for any binary operator would have walked past a shape this tree already contains.
6. Every call, from a non-test file outside `internal/bench`, to a method whose first parameter is a `bench.ResolutionHalf`, whatever expression the caller writes in that argument. The three such exported methods are `ResolveEntityIn` (`internal/bench/entity.go:797`), `ResolvePathIn` (`internal/bench/resolve.go:198`), and `ResolveReferenceIn` (`internal/bench/resolve.go:372`), and they are called from six sites: `runPath` (`cmd/dinah/commands.go:1227`), `Restore` (`internal/verb/beyond.go:286`), `Show` (`internal/verb/read.go:828`, `:876`, and `:887`), and `Contents` (`internal/verb/tree.go:889`). Search 2 sees only the three of those that spell `ArchivedHalf` out. The other three take the half from `halfFor(req)` (`internal/verb/beyond.go:318`), so the archived half is chosen by a flag and handed over by a helper, and no text a grep for the identifier can match appears at the call at all. `Contents` is the plain case: under `--archived` it resolves a card by number against the archived half, through `resolveReferenceBody` to `resolveCardIn(b.cardsRootIn(half), head)` at `resolve.go:422`, and it names none of the three mechanisms searches 1, 2, and 5 key on. It propagates the resolver's error, so it carries no defect, but it is the counterexample to any claim that naming the half is how the half gets named.

The conclusion is that exactly three sites resolve a card by number and then fall through to the archived half on any error. Searches 5 and 6 are what make that a finding rather than an absence of evidence, and search 2 is not, which is the correction round 3 earned. An ambiguity needs two things: the archived half has to be reached, and a number has to be read. Search 5 is exhaustive over the second, because a card's number cannot be read without the field being named. Search 6 is exhaustive over the first from outside `internal/bench`, because a caller outside the package cannot choose the archived half except by handing a half to one of the three resolvers, whatever expression it writes there, and searches 1 and 2 together are exhaustive over the first from inside, because inside the package the half is either the archived root named directly or the parameter `cardsRootIn` turns into it. Each of the six sites search 6 returns was opened and read, and all six propagate the resolver's error. Section 7 turns searches 1, 2, 5, and 6 into a standing guard, so a fourth fall-through cannot be added silently. Round 4 found that an earlier statement of that guard kept only the `ArchivedHalf` mechanism out of search 2 and dropped the other two, which made the guard a strict narrowing of the sweep it claimed to encode. Family D restores `ArchivedCardsRoot` and `cardsRootIn`, and it adds `ArchiveDir`, which search 2 did not key on at all and which `migrateCardVocabulary` (`internal/bench/vocabulary.go:190`) already uses to build the archived cards root without the accessor.

Six further sites are named rather than passed over, because a harmless instance of the shape is a finding and silence about it is not.

`anchorOf` (`internal/verb/changes.go:663`) and `linkRef` (`internal/verb/read.go:1249`) carry the same live-then-archive `err == nil` shape, and both are harmless. Each takes a stored 12-hex identifier and calls `bench.LoadCard` directly, so no number is parsed, no collection is scanned, and no ambiguity can arise. Neither changes.

`ResolveEditTarget` (`internal/bench/resolve.go:262`) is a discard-and-retry of a different shape, and it is the one site none of the six searches can reach: it names no archived half, it uses no `err == nil` idiom, and it calls `ResolveReference` rather than `ResolveCard`. Its body reads `entity, collection, err := b.ResolveReference(ref)` and, on any error, returns `b.ResolvePath(ref)` instead. It is benign, and the reason is worth writing down rather than re-deriving. `ResolvePath` descends through `resolvePathBody` to `resolveBelowLanding` at line 599, which reaches the same `resolveCardIn`, so the retry re-raises the identical refusal and `dinah edit fx-1` refuses `dinah.ambiguous-card` rather than opening one of the two cards. Its own doc comment fixes the discard as deliberate, so that "every reference that refuses today refuses the same way", and that property is exactly what keeps it safe here. It does not change, and AC-12 drives it so the safety is checked rather than asserted.

The collection probe inside `Show` (`internal/verb/read.go:876`) is a second discard-and-retry of the same class, and unlike `ResolveEditTarget` it reaches the archived half. It asks `ResolveReferenceIn(halfFor(req), req.Card)` ahead of the resolution the command performs anyway, ignores the error, and falls to `ResolvePathIn(halfFor(req), req.Card)` on line 887. Its own two comments already argue the safety and the argument is right: the discarded error is raised again by `ResolvePathIn` on the next line, which descends through `resolvePathBody` to the same `resolveCardIn`, so `dinah show --archived fx-3` refuses `dinah.ambiguous-card` rather than opening one of the two archived cards. It is named here because a reader auditing the sweep later will meet it beside `ResolveEditTarget` and would otherwise have to derive the same argument twice. It does not change.

Two callers flatten a resolver error into something else, and neither changes. `Instructions` (`internal/verb/read.go:1306`) replaces any resolution error with `dinah.unknown-path`, and `CardAffordances` (`internal/verb/library.go:681`) answers the workbench-level affordances when the reference does not resolve. Both are deliberate and commented, both already flatten today's `unknown-card` in exactly the same way, and neither answers with a card the caller did not name, so neither carries this card's defect. Reopening either one is a decision about those two verbs' own refusal vocabulary, which this card does not own.

### 7. The standing guard

Section 6 is a paragraph, and a paragraph ages. The guard below is the enumeration turned into a test, and it lives in `cmd/dinah` beside `address_form_arms_test.go`, which is this repository's worked precedent for a test that parses the tree and asserts a declared set against what the AST holds.

The guard parses every non-test `.go` file under the repository root and collects six families. Each family is asserted against its own recognised set and reports its own file count, mention count, and enclosing-function count, so a family that reads nothing cannot hide behind a family that reads plenty. A mention outside a family's recognised set fails the test with the file, the line, the enclosing function, and the family named, so the failure is a pointer to the work rather than a puzzle.

The families divide on two axes, and a silent wrong answer needs one from each. Families A, B, and D are the reaching axis, which is every name by which code gets at the archived half of the cards collection. Families C, E, and F are the reading axis, which is every name by which code gets at a card's number. A route that reaches the archive but reads no number cannot answer a number, and a route that reads a number but never reaches the archive cannot answer an archived card, so an attacker has to evade one family on each axis rather than one family in total.

**The guard collects mentions of a name, not calls to it.** That is the round 3 correction generalised, and it is what the tables below are built on. Round 3 keyed family A on a call whose first argument was written as a particular call expression, which a caller defeats by hoisting that argument into a local variable, and it keyed family B on the identifier the archived half is spelled by, which a caller defeats by taking the half from a helper. Both defeats are ordinary refactors rather than acts of malice, and the tree already contains the second one. So each family names a set of identifiers and collects every mention of one of them, wherever it appears and whatever surrounds it: as the callee of a call, as an argument, on the right of an assignment, as a method value, or as the name of a declaration. A rule written on mentions cannot be defeated by rewriting the expression around the name, because the name is still there.

**Families A through E read identifier nodes, never text, and family F reads a string literal because its subject is one.** A mention in families A through E is an identifier node in the parsed file, so a comment and a string literal are invisible to those five. That exclusion carries real weight now that family B reads inside `internal/bench`, because the names it collects are discussed at length in the package's own doc comments. A guard written with `grep` instead would report a site whose enclosing function does not exist, on thirty-three lines: `internal/addressform/addressform.go` carries the collected resolver names in string literals at `:269`, `:399`, `:400`, `:401`, `:402`, `:403`, and `:404`, and in comments at `:356`, `:357`, and `:359`; `internal/guide/guidepin/guidepin.go:71` carries `ArchivedHalf` inside the provenance name `TestTheArchivedHalfIsReadAtTheDeepestCollectionStep`; `internal/bench`'s comments carry collected names on eighteen further lines, at `resolve.go:33`, `:41`, `:207`, `:208`, `:232`, `:380`, `:381`, `:406`, `:595`, `:930`, `:971`, `:982`, `:997`, `:1118`, `entity.go:794`, `:795`, `:796`, and `bench.go:1969`; and `internal/verb`'s comments carry them on four further lines, at `beyond.go:275`, `:276`, `read.go:885`, and `:886`. Family F is the one rule whose subject is a literal, because a frontmatter key is a string and nothing else, and its rule says so rather than pretending otherwise.

**Family A, card resolution by number against a chosen root.** Collect every mention of the identifier `resolveCardIn` or `ResolveArchivedCard` in a non-test file, including the two declarations. Ten mentions, two files, eight enclosing functions:

| Site | Mentions | Why it is recognised |
| --- | --- | --- |
| `internal/bench/resolve.go` `ResolveCard` | 1 (`:30`) | The live accessor. Named explicitly rather than filtered out, because the rule no longer inspects the root argument and a live site is a decision rather than a syntactic accident. |
| `internal/bench/resolve.go` `ResolveArchivedCard` | 2 (`:37`, `:38`) | The exported archived accessor, whose whole body is `resolveCardIn(b.ArchivedCardsRoot(), ref)`. The declaration's own name is the first mention. |
| `internal/bench/resolve.go` `resolveCardIn` | 1 (`:44`) | The declaration of the function this family is about. |
| `internal/bench/resolve.go` `resolveReferenceBody` | 1 (`:422`) | `resolveCardIn(b.cardsRootIn(half), head)`, the parameterised route, propagating the resolver's error. |
| `internal/bench/resolve.go` `resolveBelowLanding` | 1 (`:599`) | The same parameterised route for a below-a-card reference, propagating the error. |
| `internal/bench/resolve.go` `ResolveLinkTarget` | 1 (`:924`) | Changed by section 5a; reads both halves and unwraps the ambiguity from each. |
| `internal/bench/resolve.go` `probe` | 2 (`:1187`, `:1189`) | The argued fall-through, which answers no card to any caller. The reasoning is in "What is deliberately unchanged", and the row exists so that a reader who gives `probe` a second caller meets that reasoning rather than reconstructing it. |
| `internal/verb/changes.go` `watchedCard` | 1 (`:404`) | Changed by section 5b; the same shape as `ResolveLinkTarget`. |

`ResolveArchivedCard` is the row round 2's criterion collected without declaring, and it stays declared rather than carved out. It resolves a card by number against the archived half, which is this family's own subject, so excluding the canonical instance would leave the rule and the set disagreeing about what the guard is for.

**Family B, the half-taking resolvers everywhere, and the archived half named from outside `internal/bench`.** The guard first reads `internal/bench`'s own declarations and takes the name of every exported method of `*Bench` whose first parameter has type `ResolutionHalf`. Three today, and the test asserts that name set as well: `ResolveEntityIn` (`internal/bench/entity.go:797`), `ResolvePathIn` (`internal/bench/resolve.go:198`), and `ResolveReferenceIn` (`internal/bench/resolve.go:372`). It then collects every mention of one of those three names in any non-test file, inside `internal/bench` as well as outside it, plus every mention of `ArchivedHalf` in a non-test file outside `internal/bench`. Seventeen mentions, six files, eleven enclosing functions:

| Site | Mentions | Why it is recognised |
| --- | --- | --- |
| `cmd/dinah/commands.go` `runPath` | 2 (`:1224` `ArchivedHalf`, `:1227` `ResolvePathIn`) | The `--archived` flag of `dinah path`, propagating the resolver's error. |
| `internal/verb/beyond.go` `Restore` | 2 (`:286`, both names on one line) | Restores an entity from the mirror, propagating the error. |
| `internal/verb/beyond.go` `halfFor` | 1 (`:320` `ArchivedHalf`) | The flag-to-half mapping the flag-taking commands share. This is where the archived half is minted outside the package, so it is collected on its own terms rather than through whoever calls it. |
| `internal/verb/read.go` `Show` | 4 (`:828`, both names on one line, `:876` `ResolveReferenceIn`, `:887` `ResolvePathIn`) | Three resolutions, one spelling the half out and two taking it from `halfFor`. The discard-and-retry at `:876` is argued in section 6. |
| `internal/verb/tree.go` `Contents` | 1 (`:889` `ResolveReferenceIn`) | `ResolveReferenceIn(halfFor(req), req.Ref)`, which under `--archived` resolves a card by number against the archived half and propagates the error. Round 3's rule could not see this site at all, and it is why the rule is now keyed on the resolver rather than on the half. |
| `internal/bench/resolve.go` `ResolvePath` | 1 (`:186` `ResolvePathIn`) | The one-line live delegate, which chooses `LiveHalf` and propagates the error. |
| `internal/bench/resolve.go` `ResolvePathIn` | 1 (`:198`) | The declaration of a collected name. |
| `internal/bench/resolve.go` `ResolveReference` | 1 (`:359` `ResolveReferenceIn`) | The one-line live delegate for a reference. |
| `internal/bench/resolve.go` `ResolveReferenceIn` | 1 (`:372`) | The declaration of a collected name. |
| `internal/bench/entity.go` `ResolveEntity` | 1 (`:791` `ResolveEntityIn`) | The one-line live delegate for an entity. |
| `internal/bench/entity.go` `ResolveEntityIn` | 2 (`:797` declaration, `:798` `ResolveReferenceIn`) | The declaration, and the in-package call that reaches card resolution through `resolveReferenceBody`. |

The last six rows are what round 4 found missing. A new resolver written inside `internal/bench`, beside the one this card is fixing, reaches the archived half by calling one of the same three methods, and a rule that stops at the package boundary cannot see it. The boundary was argued on the ground that family A catches every in-package route into card resolution, and that argument is wrong in one direction: family A catches the route at `resolveReferenceBody`, which is an already-recognised function whose own count does not move when a new caller appears above it. Collecting the three names in-package puts the new caller's own enclosing function outside the recognised set, which is where the test can name it.

`ArchivedHalf` keeps the package boundary, and that is a size judgement rather than a principle. Inside `internal/bench` the identifier appears on twenty-three non-comment lines, nearly all of them a comparison against a parameter the package threads through its own machinery, so collecting it would drown the table in rows that carry no information. Nothing is lost, because in-package code that chooses the archived half either names one of family D's anchors or hands the half to one of family B's three resolvers.

Deriving the three names from their declarations rather than writing them down is what makes this family a guard against a resolver nobody has written yet. A fourth exported method taking a `ResolutionHalf` joins the collected set on the run after it is declared, and the assertion over the name set means it is noticed even before anybody calls it.

**Family C, reading a card's number field.** Collect every mention of the field name `Number` in a selector expression, in a non-test file. Fifteen mentions across thirteen lines, five files, nine enclosing functions: `resolveCardIn` (`resolve.go:78`), `NextNumber` (`bench.go:2134`, `:2135`), `checkCard` (`check.go:636`), `checkCardNumbers` (`check.go:836`, and `:848` twice), `Ref` (`card.go:404`, `:407`), `Save` (`card.go:415`, `:416`), `loadCard` (`card.go:194`), `ByArrival` (`card.go:577`, twice on one line), and `jsonNumber` (`blockjson.go:508`).

Two things about that rule are deliberate. It collects a read of the field rather than a comparison of it, because `checkCardNumbers` groups cards with `byNumber[card.Number] = append(...)` and compares nothing, so a rule keyed on a comparison would miss a number scan this tree already contains. And `jsonNumber` is recognised by name with its reason recorded: the selector there is the type `json.Number` from `encoding/json` rather than a card's field, and the guard parses without type information, so telling the two apart would mean running `go/types` over the tree to remove a single row. The row is cheaper and it says why it is there.

**Family D, reaching the archived anchors.** Collect every mention of `ArchivedCardsRoot`, `cardsRootIn`, or `ArchiveDir` in a non-test file, including the declarations. Twenty-nine mentions, eleven files, twenty-two enclosing functions and two package-level sites:

| Site | Mentions | Why it is recognised |
| --- | --- | --- |
| `internal/bench/bench.go` `ArchivedCardsRoot` | 2 (`:1972` declaration, `:1973` `ArchiveDir`) | The accessor this family is named for, and its own body's use of the directory constant. |
| `internal/bench/bench.go` `HasIdentifier` | 1 (`:2108`) | Existence across both halves, which reads no number. |
| `internal/bench/bench.go` `NextNumber` | 1 (`:2124`) | Minting, which reads numbers from both halves and answers the highest plus one. It answers no card, and family C carries its two number reads. |
| `internal/bench/bench.go` `ArchivedColumnsRoot` | 1 (`:1981` `ArchiveDir`) | The columns half of the mirror, which holds no cards. |
| `internal/bench/bench.go` package-level const block | 1 (`:62` `ArchiveDir`) | The declaration of the directory name. Its enclosing site is the file's const block rather than a function, and the guard reports it as package-level. |
| `internal/bench/changes.go` `WatchedEntities` | 2 (`:77`, `:82`) | Fingerprinting, which lists identifiers and reads journals rather than resolving a reference. |
| `internal/bench/check.go` `checkCardNumbers` | 2 (`:814`, `:831`) | The duplicate-number finding `dinah-439` landed. It reports rather than resolves, and this card neither reads it nor extends it. |
| `internal/bench/container.go` package-level `benchMembers` | 1 (`:80` `ArchiveDir`) | The workbench's member list, used when a whole workbench moves. Package-level, like the const. |
| `internal/bench/entity.go` `ArchiveTarget` | 1 (`:364` `ArchiveDir`) | Where an entity directory goes when it is archived. It answers a path for a directory the caller already holds, so it cannot enumerate the mirror. |
| `internal/bench/entity.go` `refBelowHead` | 2 (`:839`, `:861`, both `ArchiveDir`) | Rendering a reference for a directory below a head, which reads no number. |
| `internal/bench/resolve.go` `ResolveArchivedCard` | 1 (`:38`) | The archived accessor, which family A also carries. Both families collect it, and that is the point rather than a duplication: it is a reaching site by one rule and a resolution site by the other. |
| `internal/bench/resolve.go` `cardsRootIn` | 2 (`:965` declaration, `:967` `ArchivedCardsRoot`) | The half-to-root mapping, and its own body's use of the accessor. |
| `internal/bench/resolve.go` `resolveReferenceBody` | 1 (`:422` `cardsRootIn`) | The parameterised route family A also carries. |
| `internal/bench/resolve.go` `resolveBelowLanding` | 1 (`:599` `cardsRootIn`) | The same, for a below-a-card reference. |
| `internal/bench/resolve.go` `probe` | 1 (`:1189`) | The argued diagnostic fall-through, which family A also carries. |
| `internal/bench/resolve.go` `descend` | 1 (`:730` `ArchiveDir`) | The collection walk, which builds a mounted collection's archived half at each step and resolves nothing by number. |
| `internal/bench/resolve.go` `ArchivedColumnByRef` | 1 (`:1016` `ArchiveDir`) | A column lookup in the mirror, which reads no card. |
| `internal/bench/resolve.go` `probeBelow` | 1 (`:1233` `ArchiveDir`) | The diagnostic walk's own descent, reached only from `probe`. |
| `internal/bench/vocabulary.go` `migrateCardVocabulary` | 1 (`:190` `ArchiveDir`) | `filepath.Join(b.Root, ArchiveDir, CardsDir)`, which is the archived cards root assembled from the constants rather than taken from the accessor. This row is the whole reason `ArchiveDir` is collected: the tree already contains one instance of reaching the archived cards without naming `ArchivedCardsRoot`, so a rule collecting the accessor alone would be a rule with a documented hole. The migration reads directory names and rewrites them, and it resolves nothing. |
| `internal/bench/vocabulary.go` `migrateColumnDirectories` | 1 (`:264` `ArchiveDir`) | The same migration for columns. |
| `internal/bench/workstream.go` `ArchivedWorkstreamsRoot` | 1 (`:58` `ArchiveDir`) | The workstreams half of the mirror, which holds no cards. |
| `internal/verb/changes.go` `anchorOf` | 1 (`:667`) | The identifier-keyed live-then-archive pair section 6 names as harmless. It parses no number. |
| `internal/verb/read.go` `linkRef` | 1 (`:1249`) | The other identifier-keyed pair, harmless for the same reason. |
| `internal/verb/search.go` `Search` | 1 (`:167`) | Searching the archived half, which matches text against loaded cards and answers no reference by number. |

Nine of those twenty-four rows are about columns, workstreams, containers, or migrations rather than about cards, and they are carried rather than filtered, because filtering them would need the guard to decide what a path is for. Each one is a line in a table with a reason beside it, which is cheap, and the alternative is a rule that has to be argued rather than run.

Family D is what closes the route that reaches the mirror through the ordinary accessor. Round 4's second attack called `ArchivedCardsRoot()`, listed the identifiers under it, loaded each anchor, and matched on the card's rendered reference, so it named no resolver and read no `Number` field, and it defeated families A, B, and C together. Every component of it is already an idiom here: `ArchivedCardsRoot()` is called from `internal/verb` at `changes.go:667`, `read.go:1249`, and `search.go:167`, and `bench.LoadCard` against that root at the first two.

**Family E, rendering a card's human reference.** Collect every call whose selector is named `Ref` and which carries exactly one argument, in a non-test file. The guard also asserts, from `internal/bench`'s own declarations, that `Card.Ref` (`internal/bench/card.go:403`) is the only method named `Ref` that takes a parameter; `Column.Ref` (`bench.go:231`) and `Workstream.Ref` (`workstream.go:82`) take none, which is what makes the argument count a sound discriminator without type information. Sixteen mentions, eight files, fourteen enclosing functions: `checkTierOverrides` (`check.go:509`), `checkItemColumns` (`check.go:548`), `resolveReferenceBody` (`resolve.go:426`, `:456`), `goneFrom` (`changes.go:635`), `entityRef` (`changes.go:680`), `view` (`library.go:475`), `Pull` (`pull.go:93`), `detailOf` (`read.go:916`), `linkRef` (`read.go:1247`, `:1250`), `searchCard` (`search.go:195`), `cardNode` (`tree.go:520`), `rootOf` (`tree.go:1019`), `itemRefOf` (`tree.go:1062`), and `containedNode` (`tree.go:1243`).

This family exists because `Card.Ref` reads the number without naming the field. Its body is `slug + "-" + strconv.Itoa(c.Number)` at `card.go:407`, inside a function family C already recognises, so a caller matching an input reference against `card.Ref(slug)` scans by number while moving no count in family C. Twenty-five zero-argument `Ref()` calls in the tree are excluded by the argument count, and they are column and workstream references, which carry no number at all.

**Family F, reading the number out of frontmatter.** Collect every basic string literal `"number"` in a non-test file. Three mentions, two files, three enclosing functions: `loadCard` (`internal/bench/card.go:193`, `fm.Value("number")`), `Save` (`internal/bench/card.go:416`, `c.FM.Set("number", ...)`), and `Add` (`internal/verb/beyond.go:89`, `fm.Set("number", ...)`). A card's number lives in its anchor's frontmatter under that key, so a scan that loads an anchor and reads the key directly reads a number while naming neither `Number` nor `Ref`. Three rows close it, and this is the one family whose subject is text rather than an identifier, which is why its rule says so.

**Counts are asserted per row, not only per family.** Each row carries its own mention count and the test asserts it. Without that, a second resolution added inside an already-recognised function changes no set and fails nothing, which would leave the guard blind to a new fall-through written in the one place a reader is least likely to look for it.

**Trying to defeat the guard.** Reading the guard is what failed in rounds 2, 3, and 4, so the rules above were settled by writing the attacker first. Each attack below is code somebody could plausibly write, and each verdict was checked against the rule as stated rather than against the rule as intended. Attacks 1 through 6 were written against round 4's three families, attacks 7 and 8 are the two the round 4 review wrote and got through, and attacks 9 through 13 are new against the six families above.

1. *Take the half from a helper.* A new function in `internal/verb` tries the live half, then writes `entity, _, err := l.Bench.ResolveReferenceIn(halfFor(req), ref)` and refuses `unknown-card` on any error. This is the attack that defeated round 3, and the tree already holds three instances of the shape. It fails at family B, which collects the mention of `ResolveReferenceIn` whatever the half argument says, so the new enclosing function is not in the table of eleven and the test names it.
2. *Hoist the root into a local.* `root := b.ArchivedCardsRoot()` on one line and `b.resolveCardIn(root, ref)` on the next. This defeated round 3's family A, whose rule inspected the first argument's syntax. It fails now at family A, which collects the mention of `resolveCardIn` and never looks at the argument, and again at family D on the accessor.
3. *Call through a method value.* `resolve := l.Bench.ResolveReferenceIn` on one line and `resolve(halfFor(req), ref)` on another, so that no call in the file has a half-taking resolver as its callee. It fails because family B collects mentions rather than calls, and the assignment is a mention.
4. *Scan by hand without comparing anything.* Read `ListIDs(l.Bench.ArchivedCardsRoot())`, load each anchor, fill `byNumber[card.Number] = id`, and answer the map lookup. No `resolveCardIn`, no `ResolveArchivedCard`, and no comparison of a number anywhere. This defeats round 3's family C, which required a binary comparison. It fails now twice over, at family C on the field read and at family D on the accessor.
5. *Add the fall-through inside a function the table already recognises.* A second `ResolveReferenceIn(halfFor(req), ref)` inside `Show`, which family B recognises with four mentions today. It fails because the row asserts its own count, so `Show` reporting a fifth mention fails the test even though the recognised set of functions is unchanged.
6. *Build the path from the layout constants.* `filepath.Join(b.Root, ArchiveDir, CardsDir)` and a hand-rolled scan over it, which is the shape `migrateCardVocabulary` already writes at `vocabulary.go:190`. Against round 4's families this survived A and B, and C caught it only while the scan still read the `Number` field. It fails now at family D on `ArchiveDir`, whatever the scan then reads.
7. *Write the fall-through inside `internal/bench` itself.* A new `ResolveCommentTarget` in `internal/bench/resolve.go` tries `b.ResolveReferenceIn(LiveHalf, raw)`, discards the error, tries `b.ResolveReferenceIn(ArchivedHalf, raw)`, and refuses `unknown-card`. This is round 4's first successful attack, and it worked because family B stopped at the package boundary while family A collected neither name. It fails now at family B, whose three derived names are collected in-package, so `ResolveCommentTarget` appears as an enclosing function outside the recognised eleven. The variant that adds the second attempt inside an existing in-package function fails too, on that row's own mention count.
8. *Match on the rendered reference.* List `ArchivedCardsRoot()`, load each anchor, and compare `card.Ref(l.Bench.Slug)` against the trimmed input. This is round 4's second successful attack, and it defeated all three of that round's families from anywhere in the tree, because it named no resolver, no half, and no `Number` field. It fails now twice, at family D on `ArchivedCardsRoot` and at family E on the one-argument `Ref` call.
9. *Build the path from the constants and match on the rendered reference.* Attack 6's reaching half joined to attack 8's reading half, which is the strongest combination available out of names the tree already uses. It fails at family D on `ArchiveDir` and at family E on the `Ref` call, and either one alone is enough.
10. *Take the archived identifiers from a fingerprint.* Call `b.WatchedEntities()`, keep the archive half of its answer, recover each card's directory from the watch entry's `Journal` path with `filepath.Dir`, load the anchor, and compare `card.Ref(slug)`. This names no member of families A, B, C, D, or F, because `WatchedEntities` is the function that names `ArchivedCardsRoot` and the attacker only calls it. It fails at family E alone, which is the reason family E is worth its sixteen rows.
11. *Read the key rather than the field.* Any of the reaching routes above, with the scan comparing `strconv.Itoa(n)` against `card.FM.Value("number")` so that `Number` is never named. It fails at family F on the literal, and at whichever reaching family the route's own path construction names.
12. *Declare a fourth half-taking resolver.* Add `func (b *Bench) ResolveCardIn(half ResolutionHalf, ref string) (*Resolved, error)` to `internal/bench` and call it from the new code. It fails at family B's name-set assertion, which is derived from the declarations rather than written down, so the new name is reported on the run after it is declared and before any caller exists.
13. *Rename the rendering call away.* `render := card.Ref` on one line and `render(slug) == ref` on another, which is attack 3's dodge aimed at family E. This one survives family E, because family E collects a call carrying one argument and a method value is not a call, and a guard parsing without type information cannot tell the method value `card.Ref` from a read of the struct field `Ref`, which the tree does at ninety-six non-call selector sites. It is half an attack rather than a whole one, because the route still has to reach the archive and every reaching name is collected. It is written into the limits below rather than answered.

**What the guard still cannot see.** Six limits, and they belong in the criterion rather than in a reviewer's head. The first states the floor, which is the shape of a route no family collects at all.

- The floor is a route that reaches the archived anchors while naming none of `ArchivedCardsRoot`, `cardsRootIn`, `ArchiveDir`, `resolveCardIn`, `ResolveArchivedCard`, or the three half-taking resolvers, and that reads a card's number while naming none of the `Number` field, a one-argument `Ref` call, or the literal `"number"`. Both halves have to hold at once. Two constructions of the reaching half exist: a path assembled from a hand-typed `"archive"` literal, which no non-test file in the tree writes today, and a path derived from some other value the package already answers with, such as `ArchiveTarget`'s result or a watch entry's journal path. One construction of the reading half exists, and it is attack 13's method value. So the floor is reached only by an attacker who writes both an unusual path construction and an unusual number read in the same function, which is a long way from the fall-through this card is fixing.
- Family E is defeated on its own by a method value, per attack 13. The guard does not claim otherwise, and a route taking that dodge still has to clear the reaching axis.
- Family B collects `ArchivedHalf` outside `internal/bench` only, so in-package code that chooses the archived half is seen through family A or family D rather than through the half's own name.
- The guard reports source shape and never behaviour. It says that a site exists, not that the site handles the refusal correctly, which is what the plants in AC-8 are for.
- Recognising a function does not follow that function's callers. A helper wrapping a half-taking resolver is collected at the helper, so the helper's own row has to be argued and its callers are then invisible. Attack 10 is that limit in its sharpest form, since `WatchedEntities` is a collected function whose answer carries archived paths outward. The per-row counts are what force the argument to be made again when such a helper's body changes.
- Generated code and reflection are outside the guard entirely. It reads the `.go` files in the tree as they are committed.

### 8. The audit every check on this card is put through

Four rounds of review have pushed this card back, and not one of them touched the design. Every push-back has been about a check: a guard that collected a site it did not declare, a guard that encoded half the enumeration it claimed to encode, fixtures that could not reach the arms their plants were supposed to arm, a guard whose collection rule two ordinary refactors walk straight through, and a guard whose written limits claimed coverage its own rules did not provide. Each of those is a check failing to cover what it asserts it covers, which is this card's own defect one level up. The resolver claimed to answer a reference and answered a different one; these claimed to check a rule and checked less.

Finding that by reading is what failed in rounds 2, 3, and 4, so every check the card files is put through four questions instead, and the answers are recorded here rather than left for a reviewer to derive.

**Does the criterion name a plant that genuinely reddens it?** The fix changes five arms: the raise site, and a live and an archived unwrap in each of the two callers. AC-2, AC-3, AC-4, and AC-6 drive the raise site, so plant 1 reddens them. AC-5 drives `ResolveLinkTarget`'s live unwrap and AC-9 drives `watchedCard`'s, each against a fixture whose archive holds a card on the colliding number, which is what stops them passing on the unrepaired code. AC-11 drives both archived unwraps against a number the live half does not carry at all, which is the only fixture shape in which an archived-half ambiguity is reachable, since a live card on the number answers the first attempt and the archive is never read. Five arms, five plants in AC-8, and a criterion reaching each. This is the question round 2 failed, by counting plants against the arms the fix set out to change rather than against the arms it changed.

**Does the guard collect exactly the set it declares?** This question is asked of AC-10, which is the only criterion that collects rather than drives. Each of its six families was run over the tree by hand in both directions: every mention the rule collects appears in a row, and every row is a mention the rule collects. Family A's ten mentions in eight functions, family B's seventeen in eleven, family C's fifteen in nine, family D's twenty-nine in twenty-two functions and two package-level sites, family E's sixteen in fourteen, and family F's three in three are the whole of what those rules return at `c1ae4a9`. Running family B's widened rule by hand is what produced its six in-package rows, and running family D's is what found `migrateCardVocabulary` at `vocabulary.go:190`, which reaches the archived cards root from the layout constants and which no earlier round's rule could see.

**Does the check pass on the tree it ships on?** Every criterion other than AC-8 is satisfiable by the shipped code plus this card's diff, and AC-8 is satisfiable by construction, because each plant restores a conditional the shipped code already contains. AC-10 is the one that could ship red, since it is the only criterion asserting a set against the whole tree rather than a behaviour against a fixture, and its six tables were enumerated from the tree rather than from the previous round's tables. Round 2's version of it shipped red on a row its own rule collected.

**Can somebody write code that does the forbidden thing and still satisfies the check?** This is the question rounds 3 and 4 failed, and it is the one that has to be answered by construction rather than by reading. Thirteen attacks are written out in section 7 with the verdict for each. Twelve fail. The thirteenth, a `Card.Ref` method value, survives family E on its own and cannot reach the archive without tripping a reaching family, so it is recorded as half an attack in the limits rather than answered. Two of the thirteen, the in-package fall-through and the match on the rendered reference, are the attacks the round 4 review wrote against the previous round's rules and got through, and they are kept in the list with their new verdicts rather than deleted, because an attack that once worked is the only evidence that the rule which now stops it was needed.

Two further things this audit does not claim. It does not claim the checks are complete against the code, which is what section 6's sweep and AC-10's guard are for, and it does not claim they stay correct as the tree moves, which is what AC-7's whole-suite run is for. What it claims is that every check this card files has been read against its own assertion and then attacked, rather than read against its author's intent.

**On whether the guard should ship with the fix at all.** Four rounds have now pushed this card back over the guard while leaving the fix untouched, which is the shape of a card carrying two pieces of work, so the question is asked here rather than left implicit. The answer is that they still belong together, and the reason is specific rather than a preference. The fix removes three fall-throughs; nothing in the fix stops a fourth being written next month, and the sweep in section 6 is the evidence that the three are all there are, which decays the day somebody adds a resolver. A guard is what carries that evidence forward. What the four rounds actually produced is a rule that now collects six families instead of three, derives two of its name sets from declarations rather than from a written list, and carries thirteen attacks with their verdicts. That is a guard worth defending, and it was cheaper to reach here, beside the enumeration that motivated it, than it would have been on a card of its own that had to re-derive the sweep first. If a reviewer disagrees, the split is theirs and the operator's to make, and the fix is independently shippable as it stands.

## What is deliberately unchanged

`splitRef` keeps its behaviour exactly. A reference splits at its last dash, resolution keys on the number alone, and a prefix naming no current slug is accepted with `Resolved.StalePrefix` set rather than refused. That is documented at `internal/bench/resolve.go:20` and at `docs/design/format.md:1398`, it is deliberate, and this card does not touch it. A stale prefix on an ambiguous number changes nothing either: the ambiguity is decided before a prefix has anything to qualify, so the refusal wins and no warning is composed.

`probe` (`internal/bench/resolve.go:1144`) keeps its `err == nil` fall-through at lines 1187 to 1191, and it is the third site of the shape section 5 covers. It is a diagnostic walk reached from one caller, `notArchivedFor` (`internal/bench/resolve.go:1093`), every return on whose walking branch is an error. That function does carry one `nil` return, at `resolve.go:1098`, and it sits on the pre-walk branch, which answers before `probe` is called at `:1106`, so no caller can reach `probe` and be handed a nil error in its place. Its whole output is which of two refusal names an already-refused `--archived` reference should print, and its own comment says nothing it reaches can become an answer to a caller. It answers no card to anybody, so it cannot answer the wrong one.

Its behaviour under an ambiguous number is worth stating exactly, because the obvious sentence about it is only half true. Where the archive holds no card on the number, both attempts fail and the walk answers `found = false`, so `notArchivedFor` returns the mirror's own error unchanged and the reader gets today's sentence. Where the archive holds one, the live attempt now fails on the ambiguity, the archive attempt succeeds, and the walk answers `holder = head`, so the reader gets the mirror-holder sentence rather than the live one. Both sentences are true of a reference that has already been refused, and neither names a card, so `probe` is safe either way rather than safe because the walk finds nothing.

The tripwire follows from that. `probe` is safe exactly while its result stays a choice between two refusal names, which is the property its own comment asserts and the reason `notArchivedFor` answers nothing but an error on the branch that reaches the walk. The moment anything it answers becomes an answer a caller acts on, whether by `probe` gaining a caller that is not `notArchivedFor` or by `notArchivedFor` gaining a non-error return on the branch that reaches it, this fall-through becomes a fourth instance of the defect and needs the same unwrap `ResolveLinkTarget` gets. The guard's family A keeps `probe` on the recognised list with that reasoning recorded beside it, so a reader who adds the caller meets the reasoning rather than having to reconstruct it.

Ambiguity is scoped to the half being read, because `resolveCardIn` scans one root. A live card and an archived card sharing a number is not this refusal, and `ResolveCard` goes on answering from the live half first, which `ResolveArchivedCard`'s own comment already fixes as the order.

The head-side plumbing needs no work at all, and the implementer should confirm rather than extend it. A refusal carrying `Extra` raised inside `internal/bench` already reaches both heads with its rows intact: `verb.ComposeRefusal` copies `refusal.Extra` into `Response.Context` (`internal/verb/response.go:30`), `outcomeValues` copies that into the composer's values (`cmd/dinah/render.go:918`), `composeRefusal` draws the `Carried` set as rows (`cmd/dinah/render.go:1015`), and `reportError` puts the same map on the machine form as `context` (`cmd/dinah/main.go:387`). The MCP head serves the same `Response`, so its `context` member carries the identifiers as data.

## Out of scope

This card adds no `dinah check` finding, no repair, and no storage change, and it does not move the number out of card frontmatter. The duplicate-number finding already exists as of `dinah-439`, at `checkCardNumbers` (`internal/bench/check.go:811`), and this card neither extends nor reads it. The registry work is `dinah-488`, which is in design.

This card adds no profile statement and amends none. `CORE-CARD-1` constrains the identifier's uniqueness and the glossary defines the identifier as the hex, so nothing in section 6 of the profile changes. Whether resolution by a non-unique label deserves a profile statement is `dinah-488`'s question, since that card decides what the label is.

## The fixtures

A workbench with two cards carrying one number cannot be minted through the tool, because `add` scans for the highest number in use and counts up. Both fixtures below therefore set each card's `number` explicitly, so nothing in a criterion rests on what `add` happens to mint.

Both fixtures hold two collisions rather than one: a number the live half carries twice, and a different number the live half carries not at all while the archive carries it twice. The second collision is what makes the archived-half unwraps reachable. With a live collision alone, `ResolveLinkTarget` and `watchedCard` never get past their live attempt, so restoring either archived unwrap as a plant leaves every test green and the arm is guarded by nothing.

### The bench-package fixture

`internal/bench/check_test.go` already gives the helpers. `newFixture(t)` (line 149) writes a clean workbench under `t.TempDir()` carrying one column `b00000000001` and one card `c00000000001` whose anchor is `cleanCard` (line 135) with `number: 1`, and whose workbench anchor carries `slug: fx`. `write(t, path, text)` (line 160) puts a file on disk, creating the directories above it. `archiveDir(t, dir)` (`internal/bench/resolve_archived_half_test.go:37`) moves an entity into the archive mirror and answers where it went, and `buildArchivedHalfFixture` in the same file at line 146 is the worked precedent for writing a second card's anchor by hand.

```go
root := newFixture(t)
card := func(id, title string, number int) string {
	dir := filepath.Join(root, CardsDir, id)
	write(t, filepath.Join(dir, CardAnchor),
		fmt.Sprintf("---\ntitle: %s\nnumber: %d\ncolumn: b00000000001\nstate: ready\n---\nFraming.\n", title, number))
	write(t, filepath.Join(dir, JournalName), cleanJournal)
	return dir
}
card("c00000000002", "The live twin", 1)        // live, collides with c00000000001
card("c00000000003", "The link source", 2)      // live, the only card answering fx-2
archiveDir(t, card("c00000000004", "The archived odd one", 1))
archiveDir(t, card("c00000000005", "The archived twin", 3))
archiveDir(t, card("c00000000006", "The other archived twin", 3))
```

`c00000000001` comes from `newFixture` already carrying `number: 1`. The live half therefore answers `fx-1` with two cards, `fx-2` with one, and `fx-3` with none, while the archive answers `fx-1` with one and `fx-3` with two. Three references drive three different questions against one fixture.

- `fx-1` is the live collision. The archived card on the same number has to be there, or a guard over a live-half unwrap passes for the wrong reason, since without it the unrepaired fall-through would refuse `unknown-card` too.
- `fx-2` is the sole answer to its number, and it is the link subject, because `link` resolves its subject before its target.
- `fx-3` is the archive collision, with nothing in the live half, which is what makes the archived-half unwrap the arm under test. The live attempt refuses `unknown-card`, the archived attempt refuses `dinah.ambiguous-card`, and only the archived unwrap can carry that refusal to the caller.

AC-4's ordering variant is a separate build of this recipe with `c00000000002` and `c00000000003` both on `number: 1` and no archived card, which gives `c00000000001`, `c00000000002`, and `c00000000003` in ascending order.

### The CLI fixture

`cmd/dinah` builds its workbenches through the head rather than by hand: `newBench(t)` (`cmd/dinah/main_test.go:227`) runs `init --slug fx --operator alka` and sets `DINAH_ACTOR=alka`, `addCard(t, root, title)` (line 3747) files a card and answers its alias, `cardID(t, root, ref)` (line 3767) answers a card's identifier, and `soleBenchDir(t, container)` (line 260) answers the workbench directory inside the container. `runCLI(t, dir, argv...)` (line 198) runs a command and answers its exit code, stdout, and stderr.

The duplicate is made by filing cards normally and then rewriting the `number:` line of each anchor, which is how `writeAnchorlessCard` (`cmd/dinah/changes_test.go:190`) already reaches into a fixture's files. The helper takes an identifier and a root rather than a reference, because a reference stops resolving the moment its number collides and stops resolving at all once the card is archived:

```go
// setNumber rewrites one card's number in its own anchor, which is the only
// way to build a state add refuses to mint. The card is named by identifier
// and the root is named explicitly, so the same helper reaches a live card
// and an archived one.
func setNumber(t *testing.T, root, id string, number int) {
	t.Helper()
	anchor := filepath.Join(root, id, bench.CardAnchor)
	// read the anchor, replace the single `number: <n>` line, write it back
}
```

The live root is `filepath.Join(soleBenchDir(t, container), bench.CardsDir)` and the archived root is `filepath.Join(soleBenchDir(t, container), bench.ArchiveDir, bench.CardsDir)`, both built from exported constants. The recipe files six cards, records each identifier with `cardID` while its number is still unique, archives the three that belong in the mirror with `runCLI(t, root, "archive", <ref>)`, and only then rewrites the numbers. Archiving before duplicating is what keeps `cardID` unambiguous, and rewriting after archiving is what lets an archived card carry a number no live card has.

The identifiers a CLI fixture mints are random, so a CLI-level criterion compares against the set of card directories it finds rather than against literal identifiers. The fixed-identifier ordering in AC-4 is driven in `internal/bench`, where the recipe names the directories.

`TestTheAmbiguousNameRefusalNamesPositionsThatResolve` (`cmd/dinah/main_test.go:8274`) is the template for the refusal tests: it refuses an ambiguous reference, then takes each candidate the refusal named and proves that candidate resolves on its own. This card wants the same two-part shape.

## Acceptance criteria

The criteria are filed as checklist items. AC-1 was filed at triage and binds the compatibility claim; the rest were added with the spec.

## Branch

dinah-487-a-card-number-that-names-two-cards
