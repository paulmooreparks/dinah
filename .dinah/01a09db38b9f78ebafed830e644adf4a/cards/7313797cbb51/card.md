---
title: A card answers to its raw identifier, the guide never says so, and the spelling the guide implies is refused
column: b69abf918c42
state: ready
severity: minor
priority: soon
workstreams:
  - 994787601ae6
---
A card can be named by its raw twelve-character identifier, and the references guide never mentions it. Worse than an omission: the form the guide's own wording implies is actually rejected, so a reader who reasons from what the guide teaches lands on a spelling the tool refuses.

Found by the Test stage on dinah-457, which built the binary and ran every claim the guide makes rather than reading it, and then asked the reverse question the guide cannot answer for itself: what does the tool do that the guide does not mention. The derived table catches a command drifting out of the guide; nothing catches a whole addressing form that was never written down.

What the card should settle, none of it the operator's to rule on:

Whether the identifier form is taught or left undocumented on purpose. There is a real argument for silence: dinah-451 established that a position is a spelling for now rather than a handle to keep, and dinah-456 ruled that a human row prints the positional reference alone while the machine payload carries both the reference and the identifier. That split may mean the identifier is deliberately the machine's spelling and not a reader's. If so, say it in the guide rather than leaving a reader to discover it works, because an undocumented form that works is a form people come to rely on.

The spelling the guide implies but the tool refuses. Establish exactly what it is by running rather than by reasoning about the prose, then decide whether the guide stops implying it or the tool starts accepting it. Note that dinah-457 already shipped one case of this shape and caught it: its implementer wrote the bare workbench slug into the guide as a spelling, and it does not resolve, because a bare slug reads as a card and the slug names the workbench only at the head of a longer path.

Whether anything guards the reverse direction generally. The guide's table is derived from the code so a command cannot vanish from it, but no check asks whether the tool accepts a form the guide never teaches. That is the same one-directional weakness this workstream has now found three times in other guards. If a general check is impractical, say why rather than leaving the asymmetry unremarked.

Read the Test stage's evidence comment on dinah-457 for what it observed, then verify by running.

Related: dinah-456 is the addressing contract, dinah-457 is the guide and its derived table, dinah-451 established that a position is not a durable handle.

## Specification

Worked against `46abf67` (`origin/main` at the time of writing, `dinah-478: every guide is scanned for a denial, and five archive statements are held to their tests`), in the worktree `C:/dinah-scratch/dinah-471-spec3/wt` for round 3, and in `C:/dinah-scratch/dinah-471-spec2/wt` for the round before it. Every accept-or-refuse claim below was produced by running a binary built from that commit against a throwaway workbench at `C:/dinah-scratch/dinah-471-spec2/probe`, with `DINAH_HOME` pointed inside the scratch directory. The runs are quoted in the card's round-2 evidence comment.

Round 2 rewrote sections 1, 6 and 7 against a corrected measurement. Round 1 said the resolver accepts at eleven places and rested the guard's whole promise on that number, and Agent Design Review proved it false by tracing call paths and running the tool. Section 6 carries a differently shaped guard, and section 6.6 says what it cannot do.

Round 3 changes four things and leaves the rest of the card standing. Section 6.4 now partitions the forty-six, because round 2 required a comparison in both directions that failed in both, and it gains a fourth exemption ground for the seven functions that fit none of the three. Section 6.1 lands the seventeen `Guide` strings that two rounds asked for. Section 6.5 gives the fixture the rule that lets the `column-title` probe fail, which round 2's fixture did not. Section 6.6 gains two further stated limits. The arithmetic of section 6.4 was re-run over the whole package rather than adjusted by its deltas, and the run is quoted in the round-3 evidence comment.

## 1. What the card settles

Dinah accepts spellings of a card that the references guide never teaches: the card's twelve-hex identifier standing alone, the card's number standing alone, and a reference whose prefix names no current slug. The guide teaches the identifier for a column, for a workstream, and for a member of a collection, so silence about the card's identifier is an inconsistency rather than a policy. This card teaches all three, and it teaches the precedence that decides what a bare head names, because a column can shadow both of the new card forms and does so silently.

Dinah accepts two more spellings of a workstream that the guide never teaches, namely the slug and the identifier standing bare, which is what `dinah join`, `dinah leave` and the other workstream-taking commands read. `dinah help join` documents the bare form already. The guide teaches only the prefixed form, and `dinah path` refuses the bare one, so the same string is accepted by one command and refused by another with nothing written down about the split. This card teaches both forms and states the split.

Dinah refuses `wb-<identifier>`, and the guide's sentence "You may write an entity's own identifier in place of its number" is what sends a reader to that spelling, because a card is an entity and a card's reference ends in a number. This card narrows the sentence to the collection member it was always about and says outright that a card is not reached that way.

Nothing guards the reverse direction, and a guard is practical. It is not the guard round 1 proposed. `internal/bench` answers an address from eighteen functions rather than from one resolver, and eight of the accepting arms sit in `ColumnByRef`, `ArchivedColumnByRef` and `WorkstreamByRef`, which build neither of the two structs round 1's scan was going to read and which `internal/verb` calls directly at twenty-three sites. Section 6 declares the eighteen functions, holds each one's arm count against the source, holds the roster against a type-based sweep of the whole package so a nineteenth function cannot appear unrostered, and runs one example of every declared form and every declared near miss through the command each was measured on.

## 2. What running established

`dinah path <ref>` against a workbench with slug `wb`, cards `wb-1` (identifier `a94be9ddf5a4`) and `wb-2`, one comment, one open question, one attachment named `notes.md`, columns `intake` / `doing` / `done` (`9177155ea3d5`, `7601a1095bf5`, `e77c42d5475d`) and workstream `addressing` (`65cd4a46c729`).

Accepted, and the guide teaches them: `workbench`, `.`, `wb/attachments`, `wb-1`, `doing`, `Doing`, `7601a1095bf5`, `workstream/addressing`, `workstream/65cd4a46c729`, `wb-1/comments/1`, `wb-1/attachments/notes.md`, `wb-1/attachments/1/payload`, `wb-1/questions/1`, `wb-1/comments`, `wb-1/card`, `wb-1/journal`.

Accepted, and taught nowhere: `a94be9ddf5a4`, `1`, `foo-1`, and each of the three standing as the head of a longer path. Two more are accepted by the workstream-taking commands and taught nowhere in the guide: `dinah join wb-1 addressing` and `dinah join wb-1 65cd4a46c729` both exit 0, and `dinah help join` says "the workstream you are naming, written as its slug or its identifier".

Refused through `dinah path`, each with the refusal name its `--json` payload carries: `wb-a94be9ddf5a4` and `wb` and `column/doing` and `addressing` and the workbench's own thirty-two-character directory name all raise `unknown-card`; `wb/cards/1` raises `dinah.unknown-path` with `addressed: card`.

Accepted and not a spelling to teach: `wb-01`, `wb--1`, `WB-1`, and a reference carrying leading or trailing whitespace. `strconv.Atoi` accepts a leading zero, `splitRef` cuts at the last dash so any prefix is a prefix, and `resolveCardIn` trims the reference before it does anything. Dinah writes none of these back.

**A column shadows both new card forms, and this was proven by running rather than reasoned about.** `resolveReferenceBody` asks `columnByRefIn` before `resolveCardIn` for a bare head, and `ColumnByRef` matches the identifier, then the slug, then the title. A column title is unconstrained, so `dinah set done title 1 --yes` followed by `dinah path 1` answers the column's `column.md` and `dinah show 1` prints the column, while `wb-1` goes on opening the card. A column slug cannot do this, because `dinah set <column> slug 1` refuses `malformed` and `ValidColumnSlug` admits only what `SlugifyDashed` leaves unchanged, but a slug of twelve hex characters is admitted and would shadow a card identifier the same way. Round 1 recorded this as a reading of lines 404 to 421 that nobody had run, and the truth is worse than the reading: the guide edit of section 5 therefore teaches the precedence in the same paragraph as the forms.

## 3. Ruling one: the identifier is taught

`docs/quick-start.md` already scopes the rule correctly at line 761, "You name a thing in a collection either by its twelve-hex identifier or by its position", and `bench.ResolveCard`'s own doc comment names both accepted card forms: "The accepted forms are the 12-hex identifier and the dash-joined reference whose last segment is the card's number." The references guide is the outlier rather than the site of a deliberate silence.

So the identifier is taught. The argument for silence, that dinah-456 gave the human row the positional reference and the machine payload both, is an argument about what Dinah writes rather than about what Dinah reads, and one of section 6's guards holds Dinah to writing the positional reference back whichever spelling was typed.

## 4. Files to land

| File | What lands |
|---|---|
| `internal/addressform/addressform.go` | new package: the form roster, the near-miss roster, the resolver roster and the sweep exemptions |
| `cmd/dinah/address_form_roster_test.go` | new: the roster's internal consistency and its tie to `verb.ReferenceKindOrder` |
| `cmd/dinah/address_form_arms_test.go` | new: the two AST guards of sections 6.3 and 6.4 |
| `cmd/dinah/address_form_run_test.go` | new: the three running guards of section 6.5 |
| `cmd/dinah/address_form_guide_test.go` | new: the two guide guards of section 6.2 |
| `internal/guide/guides/references.md` | the sentences of section 5 |

**No production code changes and no signature changes.** Round 1 proposed a `Form` field on `Resolved` and on `EntityRef`, and form-reporting siblings for three functions. Section 6.5 reaches the same assurance by asserting which entity each spelling resolves to rather than by asking the tool which arm ran, so none of that is needed and none of it lands. A reviewer should read the absence of a production diff as deliberate.

No message catalogue key is added and no locale file is touched, because nothing new is printed to a person. Section 7 gives the reason.

`internal/addressform` is read by tests alone. `internal/guide/guidepin` is the precedent: it is a non-test package that no non-test code imports, and it exists for the same reason, which is that a declaration two test packages both read has to live where both can reach it.

## 5. The guide edits, spelled out

### In `## A card`, after the existing `dinah show wb-1` block

```
You may also write the card's identifier on its own:

    dinah show 4f0a1c2b8d31

You may also write the card's number on its own:

    dinah show 1

Dinah reads a bare head as a column before it reads it as a card, so a
column whose identifier, slug or title is what you typed answers instead of
the card. Write the card's reference when you want the card whatever your
columns are called.

Dinah resolves a card on its number rather than on its prefix, so a
reference carrying a prefix that names no current slug still opens the card
it named. If you rename your workbench after writing a reference down, that
reference goes on working.
```

`4f0a1c2b8d31` stands nowhere in the tree today and is an illustration rather than a fixture value. No test executes the references guide's indented blocks, and the roster of section 6 builds each example from the fixture the run creates, so the guide's literal identifier is resolved by nothing.

### In `## A workstream`, after the existing `dinah contents workstream/addressing` block

```
The commands that take a workstream also accept the slug or the identifier
on its own, so `dinah join wb-1 addressing` names the same workstream. You
write the prefixed form wherever a reference is read as an address, because
Dinah reads a bare handle there as a card and refuses it.
```

### In `## The number and the identifier`

The first sentence changes from "You may write an entity's own identifier in place of its number" to "You may write a collection member's own identifier in place of its number". Nothing else in that paragraph changes. A paragraph is added after it:

```
You do not address a card this way. You write the card's identifier on its
own, and Dinah refuses `wb-4f0a1c2b8d31`, because a card's reference joins
your workbench's slug to the card's number and to nothing else.
```

Nothing else in the guide changes. The "Which command takes what" table, its workstream sentence, the detail paragraph below it, and the archive section are untouched, so the eleven existing checks stay green without editing.

## 6. The declaration and the guards

### 6.1 The three rosters

`internal/addressform` declares `type AddressForm string` and one constant per form, each constant's string value being its dash-joined name in the DIN-HIN convention.

A **head form** is a way of naming an entity at the head of a reference, and it carries the `verb.ReferenceKind` that entity is. Fourteen stand on `46abf67`: `workbench-word`, `workbench-dot`, `workbench-slug-head`, `card-reference`, `card-identifier`, `card-number`, `card-stale-prefix`, `column-slug`, `column-title`, `column-identifier`, `workstream-prefixed-slug`, `workstream-prefixed-identifier`, `workstream-bare-slug`, `workstream-bare-identifier`.

A **selector form** is a way of picking one member out of a collection, and it carries no kind at all. Three stand: `member-position`, `member-identifier`, `member-name`. A selector names a step within a reference rather than a thing a reference names, and `verb.ReferenceKind` is the vocabulary of what a command's reference may name, so none of its six values is honest for a selector row and the field is left empty rather than filled with the nearest one. Round 1 required a kind on every row, and no honest value satisfies that for these three.

```go
type Declaration struct {
    Form    AddressForm
    Kind    verb.ReferenceKind // empty on a selector form
    Guide   string             // the references-guide text that teaches this form
    Command string             // the command one example of this form is run through
}

func Declarations() []Declaration
```

Every declaration's `Guide` value is a verbatim string from `internal/guide/guides/references.md` as section 5 leaves it. The seventeen rows are these:

| Form | `Kind` | `Guide` | `Command` |
|---|---|---|---|
| `workbench-word` | `workbench` | `dinah path workbench` | `path` |
| `workbench-dot` | `workbench` | `dinah path .` | `path` |
| `workbench-slug-head` | `workbench` | `Write the third, which is your workbench's own slug, with something below it, because Dinah reads a slug standing alone as a card and refuses it.` | `path` |
| `card-reference` | `card` | `You write a card as its reference, which is your workbench's slug and the card's number` | `path` |
| `card-identifier` | `card` | `You may also write the card's identifier on its own` | `path` |
| `card-number` | `card` | `You may also write the card's number on its own` | `path` |
| `card-stale-prefix` | `card` | `a reference carrying a prefix that names no current slug still opens the card it named` | `path` |
| `column-slug` | `column` | `You write a column as its slug, its name, or its identifier` | `path` |
| `column-title` | `column` | `You write a column as its slug, its name, or its identifier` | `path` |
| `column-identifier` | `column` | `You write a column as its slug, its name, or its identifier` | `path` |
| `workstream-prefixed-slug` | `workstream` | `You write a workstream as the word workstream, a slash, and the workstream's slug or its identifier` | `path` |
| `workstream-prefixed-identifier` | `workstream` | `You write a workstream as the word workstream, a slash, and the workstream's slug or its identifier` | `path` |
| `workstream-bare-slug` | `workstream` | `The commands that take a workstream also accept the slug or the identifier on its own` | `join` |
| `workstream-bare-identifier` | `workstream` | `The commands that take a workstream also accept the slug or the identifier on its own` | `join` |
| `member-position` | empty | `The number counts in the order the entities were created` | `path` |
| `member-identifier` | empty | `You may write a collection member's own identifier in place of its number` | `path` |
| `member-name` | empty | `you may write an attachment's filename in place of its number` | `path` |

`workbench-word` and `workbench-dot` pin their example lines from the code block rather than the sentence standing above it. That sentence, "You may write this workbench in three ways, and the three name one workbench", teaches all three workbench forms at once, so pinning it on all three would make a fourth multi-member group and fail the check of section 6.2. `Carries` searches the whole guide text rather than its prose alone, so an indented example line is a pin like any other, and `workbench-slug-head` has prose of its own and pins that instead. A reader meeting two example-line pins in a table of sentences should read them as this choice rather than as an oversight.

`member-identifier` pins the narrowed sentence section 5 rewrites, and `member-name` pins the second half of the same sentence, which section 5 leaves alone. `member-position` pins the sentence about creation order rather than the precedence sentence, because "Dinah tries the identifier first, then the position, then the filename" teaches all three selectors at once and would make a fourth group in the same way.

All seventeen strings were checked against the guide as section 5 leaves it, folded to single spaces, and again against that same text re-wrapped at forty-one columns. All seventeen stand under both, and grouping them by value gives thirteen distinct strings and exactly the three multi-member groups section 6.2 declares.

`TestEveryHeadFormNamesADeclaredReferenceKind` asserts that the set of kinds the head forms name is exactly `{workbench, workstream, column, card}` and that every selector form's kind is empty. Both directions are asserted rather than one, so a form claiming `below-card` fails and a head form losing its kind fails. `below-card` and `collection` are named by no head form because a reference reaches both by composing a head form with segments, and the test's declared expectation is what records that rather than a comment.

The near-miss roster is the refusing half, and every row names the command it was measured through. Round 1 declared a bare workstream slug as refusing `unknown-card` and its criterion probed only `dinah path`, which is the single command where that is true; the declaration was false of `dinah join` and the criterion passed anyway. Keying a near miss to a command is what stops that recurring.

```go
type NearMiss struct {
    Key     string
    Command string // the command this spelling was measured through
    Refusal string // the refusal name its --json payload carries
    Why     string
}

func NearMisses() []NearMiss
```

Six near misses stand: `card-slug-and-identifier` (`wb-<identifier>` through `path`, `unknown-card`), `card-through-its-holder` (`wb/cards/1` through `path`, `dinah.unknown-path`), `workbench-bare-slug` (`wb` through `path`, `unknown-card`), `workbench-directory-name` (the thirty-two-character directory name through `path`, `unknown-card`), `column-with-a-kind-prefix` (`column/doing` through `path`, `unknown-card`), and `workstream-bare-handle-as-an-address` (`addressing` through `path`, `unknown-card`).

That last row and the `workstream-bare-slug` head form are one string measured through two commands, and both rows are required. The workbench's rule that a criterion asserting a refusal must pin the accepting case beside it is satisfied here in its sharpest form, because the accepting case and the refusing case are the same spelling.

The resolver roster is what sections 6.3 and 6.4 read:

```go
type Resolver struct {
    File      string        // the file within internal/bench, such as "resolve.go"
    Function  string        // the function name, such as "ColumnByRef"
    Returns   int           // every return statement in the function's own body
    Accepting int           // the returns that answer something with no error
    Forms     []AddressForm // the forms this function's accepting arms serve
    Why       string        // what this function resolves, in one sentence
}

func Resolvers() []Resolver
```

Eighteen rows stand on `46abf67`, and the two counts below were produced by the AST counter quoted in the evidence comment rather than by reading:

| Function | Returns | Accepting |
|---|---|---|
| `bench.go` `Column` | 2 | 1 |
| `bench.go` `ColumnByRef` | 4 | 3 |
| `entity.go` `resolveWorkstreamRef` | 3 | 1 |
| `resolve.go` `resolveCardIn` | 8 | 2 |
| `resolve.go` `resolvePathBody` | 4 | 2 |
| `resolve.go` `resolveReferenceBody` | 11 | 4 |
| `resolve.go` `collectionAt` | 2 | 1 |
| `resolve.go` `collectionHolder` | 2 | 2 |
| `resolve.go` `resolveBelowLanding` | 6 | 2 |
| `resolve.go` `walkBelowCard` | 6 | 5 |
| `resolve.go` `descend` | 8 | 4 |
| `resolve.go` `pick` | 8 | 3 |
| `resolve.go` `ResolveLinkTarget` | 5 | 3 |
| `resolve.go` `columnByRefIn` | 2 | 2 |
| `resolve.go` `ArchivedColumnByRef` | 5 | 3 |
| `resolve.go` `workstreamByRefIn` | 5 | 3 |
| `workstream.go` `Workstream` | 3 | 1 |
| `workstream.go` `WorkstreamByRef` | 4 | 2 |

The implementer recomputes both columns with the guard itself rather than copying this table. It stands here so a reviewer can see the size of what is being declared, and section 6.3 is what makes the numbers a contract.

### 6.2 The guide guards

`TestEveryDeclaredAddressFormIsTaughtByTheReferencesGuide` calls `guidepin.Carries("references", d.Guide)` for every declaration. `Carries` folds both the guide text and the pinned string to single spaces with `strings.Fields` before it searches, so a re-wrap of the guide is not a failure and a sentence split across two source lines is still found. The test logs the number of declarations checked and fails when it is zero.

Several forms share one guide sentence, which is a real weakness and is declared rather than hidden. The three column forms share "You write a column as its slug, its name, or its identifier", the two prefixed workstream forms share "You write a workstream as the word workstream, a slash, and the workstream's slug or its identifier", and the two bare workstream forms share the new sentence of section 5, so a surviving sentence does not tell you which of the group it teaches. `TestTheSharedGuideSentencesAreTheDeclaredOnes` groups the declarations by `Guide` and asserts the multi-member groups are exactly those three, with those memberships. A form quietly joining an existing sentence rather than earning one therefore fails, and the running guard of section 6.5 is what separates the members of a group.

### 6.3 The arm-count guard

`TestEveryDeclaredResolverCarriesTheArmsItDeclares` parses each file named by the resolver roster under `internal/bench` with `go/ast` and, for each declared function, counts two things over the function's own body while descending into no function literal. `Returns` is every `return` statement. `Accepting` is every `return` statement whose first result is not the literal `nil` and whose last result, on a return of more than one value, is the literal `nil`. Both counts are compared against the declaration and both are logged.

Two counts rather than one, and each catches what the other misses. `Accepting` is the meaningful number and it pairs with the `Forms` list, but it does not see an arm whose return delegates and passes an error through, of the shape `return path, err`. `Returns` sees every arm, because an arm that answers has a return, but it also moves when an unrelated refusal is added. Requiring both is what makes a new arm in `ColumnByRef`, which is the hole this round was pushed back for, redden the run.

Arming plant: add a fourth arm to `ColumnByRef` matching a column whose title lowercased and dashed equals the reference. The package still compiles, `Returns` reads 5 against a declared 4 and `Accepting` reads 4 against a declared 3, and the failure message names the function and tells the implementer to declare the form the new arm accepts.

### 6.4 The completeness guard

`TestEveryAddressAnsweringFunctionIsRosteredOrExempted` parses every non-test `.go` file under `internal/bench` and collects every function declaration that takes at least one `string` parameter and whose results include a `Column`, `Workstream`, `Card`, `Resolved`, `EntityRef`, `CollectionRef`, `Attachment`, `Item` or `Comment`, by pointer or by slice. That subject set is the type-based question "what in this package can answer a caller's string with an entity", and it rests on the result type rather than on an inference from a name. Forty-six functions stood on `46abf67`. Every member of the subject set appears either in `Resolvers()` or in the exemption roster. Thirteen of the eighteen roster rows are members of it and thirty-three functions are exempted. The reverse direction is asserted of the exemption roster alone, which must name no function the sweep did not find; it is not asserted of `Resolvers()`, because five rostered functions answer a `string` rather than an entity and a subject set keyed on entity result types cannot see them. `resolvePathBody`, `walkBelowCard`, `descend`, `pick` and `ResolveLinkTarget` are those five. The resolver roster is not left unchecked by that exclusion: section 6.3 fails when a roster row names a function that does not stand in the file the row names. The test logs the size of the subject set, the number rostered and the number exempted, and it fails when the subject set is empty.

The exemption roster follows the shape `cmd/dinah/address_sweep_test.go` already uses for its own exemptions, with a closed set of grounds so an exemption is an argument rather than a word anybody can invent. Four grounds cover what stands today, and between them they account for all thirty-three exemptions.

- `ground-loads-by-identifier` is a function reached only after an accepting arm has already matched, which loads an entity out of a directory rather than deciding which entity a spelling names. Eight are of this kind: `LoadCard`, `LoadWorkstream`, `LoadAttachment`, `LoadItem`, `readColumnIn`, `readColumn`, `loadCard` and `loadRetiredCard`.
- `ground-creates` is a function that mints an entity rather than finding one, so its string argument is a title, a filename or an identifier it is about to store. Eight are of this kind: `NewColumn`, `NewWorkstream`, `AddComment`, `AddItem`, `AddAttachment`, `ReplaceAttachment`, `RenameAttachment` and `AdoptWorkstream`.
- `ground-lists` is a function that answers a whole collection, so its string argument names where to read rather than which member to take. Ten are of this kind: `Comments`, `Items`, `Attachments`, `cardsIn`, `workstreamsIn`, `cardsWith`, `retiredCardsIn`, `BlockingItems`, `GatingItems` and `itemsWhere`.
- `ground-delegates` is a function that answers a caller's string by handing it to a rostered resolver and returning what that resolver answered, adding no accepting arm of its own. Seven are of this kind: `ResolveCard`, `ResolveArchivedCard`, `ResolveReference`, `ResolveReferenceIn`, `ResolveEntity`, `ResolveEntityIn` and `resolveBelow`.

An exemption carrying an empty reason fails, and an exemption whose ground is outside the closed set fails.

The arithmetic closes, and it was run rather than reasoned about. A `go/ast` sweep over the non-test files of `internal/bench` reads 443 function declarations and answers 46 in the subject set, which is the number two earlier methods reached by two other routes. Comparing that set against the eighteen roster rows and the thirty-three exemptions gives thirteen roster rows inside the subject set, five outside it, no exemption naming a function the sweep did not find, no function named by both rosters, and no subject-set member named by neither, so 13 + 33 = 46. The implementer runs the same comparison from the test rather than trusting these numbers, and the numbers stand here so a reviewer can see the size of what is being declared.

`ground-delegates` was checked against the seven bodies rather than against their names. `ResolveCard` and `ResolveArchivedCard` are one-line calls to `resolveCardIn`, `ResolveReference` and `ResolveEntity` are one-line calls to their own `In` siblings, `resolveBelow` is a one-line call to `resolveBelowLanding`, and `ResolveReferenceIn` and `ResolveEntityIn` each wrap a rostered resolver with refusing arms of their own while returning what it answered on the accepting path. None of the seven adds an accepting arm, which is what the ground claims.

`AdoptWorkstream` sits under `ground-creates` rather than under `ground-loads-by-identifier`, which departs from the corrected text Agent Design Review wrote out. Its own doc comment says "AdoptWorkstream creates a workstream at an identifier a card already names", and its body builds a `Workstream` and calls `Save` rather than reading one off disk, so the loading ground is false of it and the creating ground is true. The exemption count is unchanged either way, because both grounds are exemptions, and the partition closes on the corrected assignment.

This guard is what closes the hole round 1 left. A nineteenth address-answering function added to `internal/bench` is found by the sweep, and it fails the run until somebody either rosters it with its arm counts and its forms, or exempts it on a stated ground.

### 6.5 The running guards

Each of these builds its own fixture through `runCLI` and drives the same `run()` dispatch `main()` uses.

`TestEveryDeclaredAddressFormResolvesToTheEntityItNames` generates one example per declaration from the fixture, runs it through the command that declaration names, and asserts both a zero exit and that the answer is the entity the example was built from. For a declaration whose `Command` is `path`, the answer asserted is the fixture entity's own file path, so a resolver that answered some other entity fails even though it exited zero. That is what separates the three column forms and the two workstream forms, which section 6.2 cannot separate. For the two bare workstream forms, whose command is `join`, the answer asserted is the workstream's identifier standing in the card's membership afterwards.

The fixture has to be built for that separation rather than assumed to give it. `ColumnByRef` tries every column's slug before any column's title and lowercases both, and `dinah init` titles each column with its own slug capitalised, so an example of `Doing` matches on the slug arm and goes on resolving with the title arm deleted. Agent Design Review proved that by building a binary with the title loop removed and watching `dinah path Doing` answer the same file and exit 0 under both binaries. The fixture therefore retitles the column it uses for `column-title` to a string that is no column's slug under ASCII lowering, `In Flight` for instance, and the same rule binds any later form whose example could be caught by an arm tried ahead of its own.

Reading the arm order of every resolver a probe passes through found one further instance of that shape and no others. `resolveReferenceBody` asks `columnByRefIn` before `resolveCardIn` for a bare head, so no column in the fixture may carry the `card-number` example as its slug, its title or its identifier; the fixture's columns are `intake`, `doing` and `done` under the retitling above, and none of them is `1`. The forms that survived the reading do so for stated reasons rather than by luck. `WorkstreamByRef` strips exactly one `workstream/` prefix before any arm runs, so the prefixed and bare workstream forms are separated by the command each is measured through rather than by an arm, which is why every bare row's `Command` is `join`. `card-reference`, `card-number` and `card-stale-prefix` share one accepting arm in `resolveCardIn`, so none of them can be deleted on its own, and each bites against a different narrowing of `splitRef` instead. `column-identifier`, `column-slug`, `workstream-prefixed-slug` and `workstream-prefixed-identifier` are each strings no arm ahead of their own can match, because an identifier arm tests twelve hex characters and none of the three slugs is one.

The example generator is a switch over the declared constant with a default arm that fails the run, so a form added to the roster with no example is a refusal rather than a silent skip. The test logs the number of forms probed and fails when that number is zero or differs from `len(addressform.Declarations())`.

Arming plant: delete the `card-identifier` arm from the example switch, which reaches the default arm and reddens the run while the package still compiles. The obvious alternative plant, making that arm emit the card's reference instead of the identifier, does not work: the reference resolves to the same card and the same path, so the run stays green. The implementer performs the plant rather than trusting it and records what the red run said. Round 1's plant for this test relied on the tool reporting which arm ran, which section 4 removes.

Second arming plant: delete the title loop from `ColumnByRef` and re-run. The `column-title` example must go red, and it does not go red on a fixture whose titles are their own slugs capitalised, so this plant is what proves the fixture rule above was applied rather than merely written down.

`TestEveryDeclaredNearMissRefusesWithItsDeclaredRefusal` runs one example per near miss through the command that row names, asserts a non-zero exit, and asserts the `refusal` key of the `--json` payload equals the declared refusal name. Asserting the name rather than merely a non-zero exit is what stops a resolver that has started refusing everything from passing. It logs the number of near misses probed and fails when it is zero.

`TestANonCanonicalNumberSpellingAnswersWithTheReferenceDinahWrites` runs `dinah show --json` on `wb-01`, `wb--1`, `WB-1` and a reference padded with a leading and a trailing space, asserts each exits zero, and asserts the `ref` the payload carries is `wb-1` in every case. It names the four spellings rather than counting them and fails when fewer than four ran. This is what makes the tolerance of section 7 a recorded property rather than an accident, and it is the half of dinah-456's ruling that section 3 leans on.

### 6.6 What this guards, and what it does not

Nobody can add an address-answering function to `internal/bench` without rostering or exempting it, because the sweep of section 6.4 is keyed on result type rather than on a name. Nobody can add an arm to a rostered function without its two counts moving. Nobody can declare a form without a guide sentence `Carries` finds, an example the running guard resolves, and a command that example is run through.

Five things it does not do, and each is a real gap.

It does not prove that no undeclared string resolves. The space of strings is unbounded and no test enumerates it. What it removes is the way this gap actually opened, which is a resolver arm landing with nothing written down about it.

It does not catch an existing arm being widened in place. Changing `ColumnByRef`'s title arm from an equality to a prefix match adds no return statement and moves neither count, and the running guard goes on passing because the declared examples still resolve. Nothing in this card sees that, and a reviewer of such a diff is what catches it.

It does not tell you which arm answered. The running guard asserts which entity a spelling resolved to, and two spellings reaching one entity through different arms are indistinguishable to it, so a form whose example is caught by an arm tried ahead of its own is a probe that cannot fail. That is a property of the fixture rather than of the guard, and section 6.5 says how the fixture avoids it. The same weakness is why entity identity is strictly weaker than a form the tool reports, which is the cost section 4 accepts in exchange for a card that changes no production code.

It does not re-check an exemption. An exempted function carries no arm counts, so a function exempted as a lister that later grows an arm selecting one member by name keeps its exemption and moves no number. The ground is an argument made once, and a reviewer of the diff that changes the function is what catches it going stale.

It does not reach outside `internal/bench`. `internal/verb/reshape.go` carries a second column-naming grammar in `resolveMapSource` and `resolveMapDestination`, which resolve a column within a proposed definition by identifier and by title over elements that are not yet workbench entities. That is a command argument rather than a DinahPath reference, so the roster leaves it alone, and section 8 records it so the omission is stated rather than discovered.

## 7. Out of scope, and why

No refusal message changes. `wb-<identifier>` raises `unknown-card`, whose next step is `dinah ls`, and `dinah ls` prints every card beside the reference Dinah writes for it, so the reader who typed the wrong spelling is shown the right one. A dedicated next-step key would cost eight catalogue translations and the translation staleness contract's ceremony, for a spelling the guide stops implying in the same diff.

No resolver narrowing. `wb-01`, `wb--1` and a reference carrying whitespace go on resolving. They are `strconv.Atoi` and `strings.TrimSpace` doing what they do, nothing writes them back, and refusing them would break references that work today for no reader's benefit.

No narrowing of the bare workstream handle either, which is the second answer Agent Design Review offered and this spec declines. `dinah help join` documents the bare form today, `WorkstreamByRef`'s own doc comment records that the retired kind-prefixed commands took it, and narrowing it would break `join`, `leave`, `get`, `set` and the query filter and make a shipped help page false. Teaching it costs one sentence, and the sentence has to say where the bare form is refused, because that asymmetry is the trap rather than the form.

No change to the precedence that lets a column shadow a bare card number. `orAWorkstreamNamedBarely`'s comment records that the card lookup runs before the workstream lookup so a workstream cannot shadow a card, and the column lookup running first is the same deliberate ordering. Changing it would move which entity an existing reference names, which is a compatibility change and not this card's subject. The guide states the precedence instead, in the same paragraph that teaches the form it affects.

No rename of "number" to "position" below a card. The word collision between a card's number and a collection member's number is what made the wrong inference easy, and renaming it would touch the section heading, four sentences in the guide and the quick start's own wording. Narrowing the one sentence's subject fixes the inference the card names.

No segment-level roster. `card`, `journal`, `questions`, `criteria`, `decisions`, `oq`, `ac`, `d` and `payload` are segments of the containment grammar rather than spellings of an address, they are declared already in `checklistSegments` and `cardOwnFileSegment`, and the guide teaches every one of them. The arms in `walkBelowCard` and `descend` that match them are rostered by section 6.3 and carry no form, which is how the roster records that they match a declared segment vocabulary rather than a property of an entity.

## 8. Findings recorded, not fixed here

**The stale-prefix warning gap has gone to a card of its own.** A card reached through a stale prefix is warned about only by the verbs that route through `Library.Do`, so `dinah move foo-1 doing --json` carries `"warning": "warn.stale-prefix"` while `comment`, `file`, `attach` and `set` are each silent, and nine further sites resolve a card by reference outside `Library.Do`. Agent Design Review confirmed all four silent commands by running. The operator is filing it as the defect class rather than as a sighting, and this card does not widen for it. The guide sentences of section 5 claim nothing about a warning.

**The numeric-column-title collision has gone to a card of its own.** `dinah set <column> slug 1` refuses `malformed`, because `ValidColumnSlug` forbids a leading digit, while `dinah set <column> title 1 --yes` succeeds silently and captures `dinah path 1` and `dinah show 1` from that moment on. The tool already holds the policy that a column handle must not look like a card number, and it applies that policy only to the handle that cannot shadow anything; a twelve-hex column slug is admitted and shadows a card identifier the same way. The operator is filing that as one card carrying both instances, and this card does not widen for it. What stays here is the guide sentence of section 5, which teaches the precedence rather than changing it, and D-11 records why changing it is out of scope.

**A second column-naming grammar lives in `internal/verb/reshape.go`.** `resolveMapSource` at line 516 and `resolveMapDestination` at line 549 accept a column by identifier, slug or title through `ColumnByRef`, and then accept a pending element of a proposed definition by its `id` and by its title compared case-insensitively. The last two name no workbench entity and are reachable by no DinahPath reference, so the roster of section 6 leaves them out. Recorded here so the roster's reach is stated rather than discovered by the next reader who asks whether it covers everything.

## Branch

dinah-471-a-card-answers-to-its-raw-identifier
