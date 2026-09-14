---
title: Nothing holds the extension's message catalogue to the code that reads it, in either direction
column: b69abf918c42
state: ready
severity: minor
priority: soon
workstreams:
  - 58f3e3eb621a
---
dinah-379 gave the extension a message catalogue and four guards that keep the translations honest. None of them checks the other direction: that every message name the code asks for is a name the catalogue actually carries.

All fifty-five names line up today, confirmed by hand during code review. A typo introduced later would not be caught by any test. It would throw while the sidebar is being drawn, which is the worst place to find out, because the reader sees a broken panel rather than a wrong word.

The check wants to run over the source rather than at runtime, so it fails a build instead of a reader's editor. Whether it can be written as a static read of the call sites, or needs the names to be declared somewhere the compiler already sees, is the part worth deciding rather than assuming.

Raised by the code reviewer on dinah-379 and deliberately not fixed there, since the card's own specification never asked for it and bouncing a finished card back a column to add an unrequested guard is the wrong trade.

## Specification

Worked against `origin/main` at `65a80ad8b3b525e62af2262ad6d3203876ea6c7a` ("dinah-471: a card answers to its raw identifier, and the guide now says so"), fetched 2026-09-11. Every count below was produced by running the proposed sweep over the tree at that commit, in a worktree at `C:/dinah-scratch/dinah-406-spec/wt`.

## What is missing

The extension's message catalogue and the code that reads it are held together by nothing. Three failures pass every check in `editors/vscode/test/unit/`:

1. **The code asks for a name no catalogue carries.** `t("tree.group.redy")` compiles, lints, packages and installs. `localizerOver` in `src/l10n.ts` throws `no catalogue entry for tree.group.redy in de or en`, and the throw happens while the tree is being drawn, so the reader gets a broken panel rather than a wrong word.
2. **A translation drops a placeholder its English carries.** `dialog.card.copiedRef` reads `Copied {ref}` in English and `{ref} kopiert` in German. Edit the German to ` kopiert` and the parity guard, the honesty guard, the glossary guard and the staleness guard all stay green, because none of them reads the text's placeholders. The reader is told something was copied and never told what.
3. **A translation carries a placeholder its English does not.** Edit the German to `{ref} kopiert nach {ziel}` and the same four guards stay green. Nobody passes `ziel`, so `fill` leaves the token alone and the reader sees the literal characters `{ziel}` on screen, which is a defect that looks like a translation.

The third is the second wearing the other hat, and the CLI does not catch it either. `internal/msg`'s `TestATranslationKeepsThePlaceholdersAndTheSplice` reads the English entry's placeholders and asserts each one survives, so it is one-directional by construction.

## Scope

**In:** the extension's two namespaces, and one added direction on the CLI's existing placeholder guard.

**Out, on measured evidence:** a code-to-catalogue key sweep over the CLI. Running the equivalent probe over 93 non-test Go files found 140 literal key asks and fired 10 times, and all 10 are false. Seven are prefix concatenations that a proper `go/ast` walk would report as unresolved rather than as missing: `msg.T("cmd."+name)` in `cmd/dinah/help.go`, `msg.T("reshape.wrote."+...)` in `cmd/dinah/reshape.go`, `msg.T("column."+...)` in `cmd/dinah/table.go`. Three are `Has` called on `internal/bench/frontmatter.go`'s own type, which is not a message renderer at all and merely shares the method name. Beyond the false fires there are 24 distinct dynamic key sites spread across `cmd/dinah`, `internal/bench`, `internal/mcp` and `internal/verb`, each of which would need a family declaration before the sweep could be honest. The extension has one such site. That is a different piece of work of a different size, somebody would pick it up on its own, and bundling it here would mean shipping a guard whose first run needs two dozen declarations written to silence it.

**Out:** the three related cards. See "The three related cards" at the foot.

## Half one: the code-to-catalogue sweep

New file `editors/vscode/test/unit/l10n-keys.test.ts`. It reads the same modules `l10n-coverage.test.ts` reads, with the TypeScript compiler API, and it reuses that file's shape deliberately: a reader comparing the two finds the same exclusions and the same conditional descent.

### What it sweeps

```ts
const EXCLUDED_DIRS = ["generated", "locales", "test"];
const EXCLUDED_FILES = ["l10n.ts"];
```

`l10n.ts` is the catalogue reader itself and asks for no key. `locales/` holds the catalogues rather than code.

### Which calls it reads

```ts
function isLocalizerCall(node: ts.CallExpression): boolean
```

A call is a localizer call when its callee is the identifier `t`, or a property access whose property is named `t`. The receiver is deliberately **not** whitelisted. The four spellings live today are the bare `t` (70 sites), `context.host.t` (24), `host.t` (4) and `this.deps.t` (1), and a whitelist that missed a fifth spelling would skip those call sites in silence, which is the exact failure this card exists to close. Matching on the name errs toward reading more rather than fewer. If some unrelated function named `t` ever appears, the sweep reports its argument as an unresolvable key expression, and the remedy is to rename that function rather than to exempt it.

### How it resolves a key

`keyCandidates` descends a parenthesised expression, both arms of a conditional, and both sides of `??` and `||`, which is `l10n-coverage.test.ts`'s own `candidates` applied to the first argument instead of to the message. `runVerbCommand.ts:291`, `:402` and `:409` each pass a conditional between two literal keys, so without the descent three key expressions would read as unresolvable.

Three rules resolve a key expression, and **anything else fails the test**:

1. A string literal or a no-substitution template literal. Its `.text` is the key.
2. An identifier bound to a module-level `const NAME = "literal"` in the same source file. `diagnostics.ts:238` passes `UNCERTAIN_KEY`, declared at `diagnostics.ts:72`.
3. A template expression whose head matches a declared family's prefix, with exactly one interpolation and an empty tail. `servedText.ts:349` composes the key `history.event.` followed by an interpolation of `event.event`.

Rule 3's declarations:

```ts
interface KeyFamily {
	/** The literal head of the template, ending at the interpolation. */
	readonly prefix: string;
	/** The module carrying the table whose property names are the members. */
	readonly module: string;
	/** The module-level object literal whose property names are the members. */
	readonly table: string;
}

const KEY_FAMILIES: readonly KeyFamily[] = [
	{ prefix: "history.event.", module: "servedText.ts", table: "HISTORY_ROWS" },
];
```

A family declares where its members come from and does not copy them. `familyMembers` reads the named object literal's property names off the AST, so `HISTORY_ROWS` stays the one place the event names are written. The only hand-written parts are the prefix and the pointer, and both fail loudly when wrong: a prefix matching no catalogue key fails, and a template head matching no family fails as an unresolvable key expression.

`KEY_FAMILIES` lives in the test file rather than in `src/`, because it is build-time data no reader's text passes through and `src/` is bundled into `dist/extension.js`. Nothing in it can rot silently: the member list is read off the code, and the two hand-written fields each have a test that fails when they are wrong.

### The four assertions

**Test `"every message name the extension asks for is one the catalogue carries"`.** Every resolved literal key is a key of `src/locales/en.json`'s `entries`. English membership is sufficient rather than a shortcut: `l10n.test.ts`'s `"every runtime sibling carries exactly the base catalogue's keys"` already holds all seven siblings to the base's key set in both directions, so a key in the base is a key in all eight.

**Test `"every key expression the sweep meets resolves by a declared rule"`.** The unresolved list is empty. This is what makes the first test's reach honest. Without it, a key expression the sweep cannot read is a key the sweep silently does not check, and that reads exactly like a key that checked out.

**Test `"each declared key family names exactly the catalogue entries its table reaches"`.** For each family, set equality between the catalogue's keys under the prefix and the union of the table's members prefixed, plus any literal key under the prefix the code asks for directly. `history.event.unknown` is reached that way, at `servedText.ts:345`, so the family needs no hand-written extras and carries none.

**Test `"every catalogue key is one the extension actually reaches"`.** Every key in `en.json` is a resolved literal ask or a family member. A key nobody renders is dead weight seven translators still pay for. This generalises `l10n.test.ts`'s `"every held-card key the catalogue declares is one the rendering reaches"`, which does the same thing for the nine `status.holding.*` keys by reading `status.ts` as text. Leave that test alone: it names its nine keys one at a time and reports better for them than a whole-catalogue set difference does.

### Floors

Each figure gets its own assertion, with the message naming what read nothing:

| Figure | Value at `65a80ad` |
| --- | --- |
| modules swept | 26 |
| localizer call sites | 99 |
| key expressions inspected | 102 |
| literal keys resolved | 101, of which 97 distinct |
| family key expressions | 1 |
| catalogue keys in `en.json` | 118 |
| family members examined | 22 |

Assert `> 0` on modules swept, call sites, key expressions, literal keys resolved and catalogue keys, and assert `KEY_FAMILIES.length > 0` and a non-empty member set per family. Do not write any of the numbers above into an assertion. They are a reading taken on one tree on one day and they will be wrong by the next commit; the floor is the durable claim, and this table is the evidence that the floor is not being met by luck. A sweep that resolved nothing, or one that met the family and skipped it, fails on its own counter rather than on the set difference.

## Half two: the placeholder sweep

New file `editors/vscode/test/unit/l10n-placeholders.test.ts`.

```ts
/** The placeholder names a message carries, as a set. */
function placeholdersIn(text: string): Set<string>
```

The pattern is `/\{([A-Za-z0-9_]+)\}/g`, character for character the one `fill` in `src/l10n.ts` uses. Nothing else may be written here: a guard whose idea of a placeholder differs from the renderer's checks a different thing from the one that ships.

```ts
interface PlaceholderVerdict {
	/** Key pairs compared. */
	readonly pairs: number;
	/** English placeholder names examined, counted per pair. */
	readonly placeholders: number;
	/** One entry per name the English carries and the translation does not. */
	readonly dropped: string[];
	/** One entry per name the translation carries and the English does not. */
	readonly invented: string[];
}

function placeholderVerdict(
	base: Readonly<Record<string, LocaleEntry>>,
	other: Readonly<Record<string, LocaleEntry>>,
	tag: string,
): PlaceholderVerdict
```

Each reported entry reads `tag/key: {name}`, so a failure names the language, the key and the placeholder without a reader opening a diff.

The function takes its two catalogues as arguments rather than reading them off disk, for `localizerOver`'s reason: a fixture can then drive it over a catalogue that really does drop a placeholder, and no shipped catalogue has to be allowed to carry a defect in order for the check to have an armed path.

**One test per tag**, seven of them, each named for the tag whose catalogue it walks, matching `l10n-staleness.test.ts`'s per-tag shape so a failure names the language. Each asserts `verdict.pairs > 0` and `verdict.placeholders > 0` before it asserts `dropped` and `invented` are empty, so a catalogue emptied down to nothing fails rather than passing.

Per-tag floors rather than one combined floor is the point. The seven tags split into two populations that fail differently, and a single counter would hide either behind the other:

| Population | Pairs | English placeholders |
| --- | --- | --- |
| translated: `de`, `hi` | 236 | 244 |
| skeleton: `af`, `cs`, `es`, `fil`, `id` | 590 | 610 |
| all seven | 826 | 854 |

Skeleton entries are compared rather than skipped. A skeleton is byte-identical English, so the comparison is trivially true today, and the `l10n.test.ts` honesty guard already enforces the byte-identity that makes it so. It stays in because the day somebody translates one of the five is the day this check starts doing work there, and nothing has to be edited for that to happen. The cost is 590 trivially-true pairs, which is why the two populations are counted apart.

Comparison is by **set, not multiset**. English carrying `{ref}` twice against a translation naming `{ref}` once is not a finding: word order and repetition are the translator's business, and a count comparison would fire on correct German. What is a finding is a name present on one side and absent on the other, in either direction.

### The manifest namespace

VS Code substitutes nothing into a `package.nls` value. A placeholder written into one would render as its own literal characters, so parity is the wrong check there and its absence is the right one:

**Test `"a manifest value carries no placeholder, because VS Code fills none"`.** All 312 values across the eight `package.nls*.json` files contain no `{` and no `}`. Measured: 312 scanned, 0 carrying a brace.

This replaces a manifest parity check, which would read 312 pairs and 0 placeholders and assert nothing at all. That is the vacuous half the card warns about, and pinning the true claim beats sweeping a namespace that has nothing to sweep.

### The CLI's missing direction

`internal/msg/msg_test.go` gains `TestATranslationInventsNoPlaceholder`, beside the existing `TestATranslationKeepsThePlaceholdersAndTheSplice` rather than folded into it, because the two report different defects and a reader meeting a failure should not have to work out which half tripped. It walks the same catalogues with the same placeholder pattern the existing test compiles, and reports every placeholder a translation carries that its English does not. It counts the pairs it compared and fails when that count is zero, matching the `checked == 0` fatal already at the foot of `TestATranslationTracksItsEnglishSource`.

Measured over the CLI's eight catalogues at `65a80ad`: 6,559 pairs, 2,807 English placeholders, 0 fires in the existing direction and 0 in the new one.

## False positives, measured

Every check above was run over the tree as it stands before being proposed. Each row is the number of times the check fires and the number of those fires that are true.

| Check | Fires | True |
| --- | --- | --- |
| extension: key the catalogue does not carry | 0 | 0 |
| extension: key expression that does not resolve | 0 | 0 |
| extension: key family set equality | 0 | 0 |
| extension: catalogue key nothing reaches | 0 | 0 |
| extension: placeholder dropped, 7 tags | 0 | 0 |
| extension: placeholder invented, 7 tags | 0 | 0 |
| extension: manifest value carrying a brace | 0 | 0 |
| CLI: placeholder invented, 7 tags | 0 | 0 |
| CLI: key the catalogue does not carry, **declined** | 10 | 0 |

The declined row is why the CLI key sweep is out of scope, and it is the only row that fires at all. None of the eight checks that ship needs a narrowing, an exemption or a suppression list, and none reads a translation for meaning, so no fire from any of them lands in a language nobody here can adjudicate. Every failure names a key and a placeholder name, and both are machine vocabulary a reader of this repository can judge without reading German or Hindi. That is the property the two endorsed refusals, over German articles and over the reverse glossary direction, were protecting.

## Arming

Every check ships with a plant that compiles and runs. Perform each one, watch it go red, restore from a byte-identical copy, and watch it go green. The rows marked observed were run in a scratch tree while this spec was written, against a prototype of the sweep rather than against the shipped test, so the implementer still performs all of them.

| Check | Plant | Observed |
| --- | --- | --- |
| key the catalogue does not carry | In `src/tree.ts`, change `t("tree.group.ready"` to `t("tree.group.redy"`. It type-checks, because the parameter is `string`. | red: `tree.ts:526 tree.group.redy` |
| key expression that does not resolve | Drive `sweepKeys` over `test/fixtures/unresolvable-key-call-site.ts.txt`, a fixture whose call is `context.host.t(chosenKey)` with `chosenKey` a function parameter. | |
| key resolved from a module constant | The same fixture also carries `context.host.t(FIXTURE_KEY)` with `const FIXTURE_KEY = "fixture.resolved"` at module level, and the test asserts that key resolves. This is the clean case pinned beside the refusing one: without it, a sweep reporting every identifier as unresolvable would satisfy the row above. | |
| key family set equality | Delete the `archived:` property from `HISTORY_ROWS` in `src/servedText.ts`. It compiles, because the table is typed `Record<string, ...>`. | red: `catalogue carries history.event.archived, nothing reaches it` |
| catalogue key nothing reaches | Add `"history.event.bogus": { "text": "x {actor}" }` to all eight runtime catalogues. | red, on this check and on the family check |
| placeholder dropped | Change `de.json`'s `dialog.card.copiedRef` from `{ref} kopiert` to ` kopiert`. The staleness guard stays green, because `source` fingerprints the English and the English did not move. | red: `de/dialog.card.copiedRef: {ref}` |
| placeholder invented | Change the same entry to `{ref} kopiert nach {ziel}`. | red: `de/dialog.card.copiedRef: {ziel}` |
| placeholder verdict, both arms | Drive `placeholderVerdict` over two in-memory catalogues, one dropping a name and one inventing one, and assert both lists by content. This is the clean case for the two rows above: a verdict function that reported every pair would satisfy a red plant, and this fixture pins what it must not report. | |
| manifest carries no placeholder | Add `{x}` to any value in `package.nls.json`. | |
| CLI placeholder invented | Add `{ziel}` to any German entry in `internal/msg/locales/de.json`. | |

Every floor is armed the same way: empty the population the counter reads and watch the test fail on the counter rather than on the set difference. `scripts/run-unit-tests.mjs` says outright that it cannot see one file among several registering nothing, so the floors inside each new file are the only thing standing between an empty sweep and a green run.

## Files

| Path | Change |
| --- | --- |
| `editors/vscode/test/unit/l10n-keys.test.ts` | new; four tests plus two fixture-driven arms |
| `editors/vscode/test/unit/l10n-placeholders.test.ts` | new; seven per-tag tests, the manifest brace test, one fixture-driven arm |
| `editors/vscode/test/fixtures/unresolvable-key-call-site.ts.txt` | new; `.ts.txt` so the sweep never walks it and `tsconfig.test.json`'s `exclude` keeps `tsc` off it, exactly as `confirmDestructive-call-site.ts.txt` is handled |
| `internal/msg/msg_test.go` | `TestATranslationInventsNoPlaceholder` added |

No production module changes. Nothing is added to `src/`, so the packaged extension is byte-identical and no `.vscodeignore` entry is needed. New unit test files are picked up by `scripts/run-unit-tests.mjs` off `out/test/unit/` with no registration.

## Out of scope

- A code-to-catalogue key sweep over the CLI's Go source. Measured above: 10 fires, all false, and 24 dynamic sites needing declarations first.
- Anything that reads a translation for meaning. These checks compare placeholder names and key names. They say nothing about whether a sentence reads naturally, and they must not start to.
- `package.nls` key existence in either direction. `l10n.test.ts`'s `"every %key% the manifest spells resolves to a key in package.nls.json"` already walks the parsed `package.json`, asserts one placeholder per catalogue key by count, and asserts membership per placeholder, which closes both directions for that namespace. Duplicating it here would be the worse-than-either case the card names.
- The glossary check and its exemption mechanism. Nothing specified here reads a translator note, honours an exemption or consults the glossary, so none of it depends on that mechanism working.

## The three related cards

None of the three is this sweep, and each is left where it is.

**dinah-476**, the forty-four entries a phrase in a translator note exempts from every glossary term, is a different guard reading a different thing. It concerns `TestATranslationUsesTheDeclaredWord` and its extension twin, both of which read a translation's words against declared forms. Nothing specified here reads a note, honours an exemption or consults the glossary, so this card assumes nothing dinah-476 has disproved. The two cards also run in opposite directions: dinah-476 is about a guard that silently checks less than it appears to, and this one is about checks that do not exist.

**dinah-466**, the count in a sentence needing a plural form, is a sibling rather than the same work, and it touches this card's machinery at one point worth naming. A plural entry is the first key whose spelling is composed at run time from a category, so it arrives as a key family, and the `KEY_FAMILIES` declaration above is where it would be declared. That is an interaction to honour rather than a reason to merge: dinah-466's work is deciding whether the catalogue gains a plural mechanism at all, and sweeping every string that interpolates a count, which is a question about wording and about the staleness contract rather than about key agreement.

dinah-466's description also carries a claim this card's reading disproves, recorded here rather than lost. It says a plural entry would be the first plural-aware entry this repository's message catalogues have ever carried. That is true of the extension's catalogues and false of the CLI's: `internal/msg` has `Renderer.TN`, `TestPluralsFollowTheCategories`, and 44 plural-category keys in `internal/msg/locales/en.json` today, `check.count.one` and `check.count.other` among them. Whoever takes dinah-466 should start from the CLI's mechanism rather than designing one.

**dinah-475**, German agreeing with a value it cannot see, is the class this board has twice refused to guard, on measured evidence endorsed both times. It needs a reader of German to adjudicate a fire. Every check here fires on a key name or a placeholder name and needs no such reader. Folding them together would put a guard that cannot false-fire into the same card as one that is believed unable to avoid it.

## Branch

dinah-406-nothing-holds-the-extension-s-message-catalogue-to-the-code-that-reads-it-in-either-direction
