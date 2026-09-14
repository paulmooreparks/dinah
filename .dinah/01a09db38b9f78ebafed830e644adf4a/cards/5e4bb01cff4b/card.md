---
title: A translated entry can carry the wrong word, or never reach a reader in that language, and nothing catches either
column: b69abf918c42
state: ready
severity: major
priority: soon
tier: frontier
workstreams:
  - fdfdeaaff2dd
links:
  - kind: relates_to
    to: d78f210b2d97
  - kind: relates_to
    to: 88f6e4e10b7e
---
dinah-249 shipped the fingerprint/staleness mechanism this card's description originally proposed alongside it. This card is retitled because its old title ("Every translated entry is tested against the English it came from") now describes dinah-249's shipped work rather than what remains.

What remains, per dinah-249's own decision D-4 and its "Scope boundary with dinah-252" section: a fingerprint match proves a translation is current, and says nothing about whether it chose the right word (terminology drift within one language) or whether anyone using that language ever sees it (reachability). Both are this card's mechanism. The operator has also ruled (dinah-249 OQ-4) that key reachability, previously homeless, folds into this card.

A third question, whether an LLM read of meaning belongs in the build, is this card's to decide rather than to defer; see the spec's own section on it.

## Specification

This card ships three guards and one deliberate non-guard, all built against dinah-249's shipped `Entry.Source`, `msg.Fingerprint`, `msg.BaseEntry`, `msg.Keys()`, `msg.Complete`, `msg.Base` and `msg.Tags()`.

## Layer 1: the declared layer, part A — a per-language glossary

**What it catches.** A translation that is current (its `Source` matches `Fingerprint` of the English) and still uses the wrong word for a recurring term. dinah-251 fixed this in Hindi for the word "state" before this card was written; a live, unfixed instance of the same defect exists today in German for the words "root", "owner" and "level" (evidence below), and is this layer's first proof that it works.

**Data.** New embedded file `internal/msg/glossary.json`, read by a new `internal/msg/glossary.go`:

```go
package msg

import "encoding/json"

// glossaryTerm is one concept a translator repeatedly meets. EN is the
// phrase whose presence in the base English text triggers the check; it is
// deliberately a phrase rather than always a bare word, because a bare word
// can name a second, unrelated sense in this catalog (see the note on
// "root" below), and a trigger that fires on the wrong sense produces a
// false failure rather than a caught defect. Forms lists, per language tag,
// every rendering a translator is allowed to use; more than one entry lets a
// declared grammatical variant (an inflected form) pass without loosening
// the check for an actual wrong word.
type glossaryTerm struct {
	EN    string              `json:"en"`
	Forms map[string][]string `json:"forms"`
}

var glossary = loadGlossary()

func loadGlossary() []glossaryTerm {
	var terms []glossaryTerm
	if err := json.Unmarshal(glossaryData, &terms); err != nil {
		return nil
	}
	return terms
}
```

`internal/msg/glossary.json`, seeded with four terms, established by the search method below and not a longer list guessed at:

```json
[
  {
    "en": "state",
    "forms": { "hi": ["स्थिति"], "de": ["Zustand", "Zustands"] }
  },
  {
    "en": "the root",
    "forms": { "hi": ["रूट"], "de": ["Wurzelverzeichnis"] }
  },
  {
    "en": "owner",
    "forms": { "hi": ["स्वामी"], "de": ["Akteur"] }
  },
  {
    "en": "level",
    "forms": { "hi": ["स्तर"], "de": ["Stufe"] }
  }
]
```

**How this glossary's completeness was established, and what it does not prove.** Design review on this card's first draft correctly found that the two-term seed missed a live, already-flagged defect: sibling keys `check.attach.2`/`check.claim.2` (German "Akteur") against `check.pull.1` (German "Eigentümer") for the identical English "the request names an owner". That specific divergence had been on the record since before this spec was first written, in the dinah-251/Agent-Code-Review measurement quoted below, and the first draft never went back to check it. This pass does two things about that: it re-derives the "owner" term properly, and it replaces "I happened to notice two terms" with a search method that can be re-run and audited.

The method is the one already run on this card's own comment thread, not a new one invented for this fix. dinah-251's implementer and Agent Code Review had already measured every English string in `en.json` shared by more than one key, and recorded which of those groups render inconsistently in German and Hindi once dinah-251 landed. That measurement is exhaustive over the whole corpus by construction, because it groups all 631 keys by their exact English text rather than sampling; it is not scoped to a package, a file, or a set of keys somebody already suspected. Reproduced independently for this pass:

```
$ python3 -c "
import json, io
from collections import defaultdict
def load(p):
    with io.open(p, encoding='utf-8') as f:
        return json.load(f)['entries']
en = load('internal/msg/locales/en.json')
de = load('internal/msg/locales/de.json')
hi = load('internal/msg/locales/hi.json')
groups = defaultdict(list)
for k, v in en.items():
    groups[v.get('text', '')].append(k)
dup = {t: ks for t, ks in groups.items() if len(ks) > 1}
print('total duplicate-English groups:', len(dup))
for t, ks in dup.items():
    de_texts = {de.get(k, {}).get('text') for k in ks}
    hi_texts = {hi.get(k, {}).get('text') for k in ks}
    if len(de_texts) > 1:
        print('DE diverges:', repr(t), ks, de_texts)
    if len(hi_texts) > 1:
        print('HI diverges:', repr(t), ks, hi_texts)
"
total duplicate-English groups: 36
DE diverges: 'the named state is one the workbench declares' [4 keys] {'die Werkbank deklariert den genannten Zustand', 'der genannte Zustand ist einer, den die Werkbank erklärt', 'die Werkbank deklariert den Zustand'}
DE diverges: 'the request names an owner' [10 keys] {'die Anfrage nennt einen Akteur', 'die Anfrage nennt einen Eigner', 'die Anfrage benennt einen Eigentümer'}
DE diverges: ' Name one of those instead.' [2 keys] {' Nennen Sie stattdessen einen davon.', ' Nennen Sie stattdessen eine davon.'}
DE diverges: 'What it does' [2 keys] {'Was sie tut', 'Was er tut'}
HI diverges: 'the named state is one the workbench declares' [4 keys] {two forms differing only by a copula}
HI diverges: ' Name one of those instead.' [2 keys] {two forms differing only by the demonstrative}
HI diverges: 'No cards here.' [2 keys] {two forms differing only by a trailing copula}
```

36 groups share identical English text; 4 diverge in German and 3 in Hindi, matching the count already on record in this card's own thread. Each is adjudicated here rather than assumed settled by that earlier count:

- **"the named state is one the workbench declares" (German and Hindi).** Every rendering in both languages contains the declared "state" form (`Zustand` or `स्थिति`); the three German variants and the two Hindi variants differ only in surrounding sentence structure, not in the term. Already covered by the seeded "state" term. No action.
- **"the request names an owner" (German).** Three distinct German nouns for one concept: `Akteur` (12 of the 18 keys where the English word "owner" appears at all, including this ten-key group's `check.attach.2`, `check.block.2`, `check.claim.2`, `check.comment.2`, `check.join.2`, `check.leave.2`, `check.rename.3`, `check.workstream-field.4`, plus `check.whoami.1`, `check.workbench-field.3`, `check.workbench-field.4` and `flag.actor.summary`), `Eigentümer` (`check.pull.1`), and `Eigner` (`check.card.5` and, outside this duplicate-English group, the column heading `column.states.owner`). This is the review's finding, and it is worse than the review stated: German uses three words for "owner", not two, because `check.card.5` and the `column.states.owner` heading were missed by a review pass scoped to the three sibling keys the comment thread had already named. Corrected here: seeded term, evidence and fix below.
- **" Name one of those instead." (German and Hindi).** German gender agreement on the pronoun ("einen davon" against "eine davon"), already documented in the dinah-251 measurement as a legitimate agreement difference rather than a wrong word; Hindi differs only by which demonstrative ("उनमें"/"इनमें") is idiomatic for its antecedent. Neither is a term. No action.
- **"What it does" (German).** Documented gender agreement ("Was er tut" against "Was sie tut", referring to the commands table and the flags table respectively). Not a term. No action.
- **"No cards here." (Hindi).** The two renderings differ only by a trailing copula ("है"), a completeness-of-sentence choice rather than a word swap. Not a term. No action.

**Completeness this method can and cannot claim, and the counterexample round two found.** The duplicate-English grouping is exhaustive over literal duplicate strings: every one of the 631 keys is in exactly one group, and every group of size greater than one was read above. That is not a proof the seed set is complete, and this pass stops describing it as one. Round two of design review found a live defect the method's own construction cannot see: `check.card.4` ("the value is a level that field declares") and `check.add.5` ("each named level is one that field declares") name the same concept in near-identical, but not identical, English, so the grouping puts them in two different size-one groups and never compares them. German's rendering of the pair diverges exactly the way "owner" diverged: `check.add.5` correctly says `Stufe`; `check.card.4` drops the word and says `"das Feld deklariert diesen Wert"` ("the field declares this value"), current per its `source` fingerprint and wrong regardless of currency. Hindi renders both correctly with `स्तर` and needs no fix.

So the method is not a proof of completeness with one acknowledged footnote. It is a seeding heuristic that finds every literal duplicate and nothing else, and it has now missed a live defect twice: first the "owner" cluster's `check.card.5`/`column.states.owner` pair, and now this `level` pair. Neither miss is the method executed poorly; both are exactly what a check built on identical-string grouping can and cannot do. A fourth seeded term, `level`, closes the specific defect this pass found, backfilled from the same worked evidence as the other three: a full grep of `en.json` for the bare word "level", nine occurrences, each read individually.

`en.json`'s nine occurrences are `check.add.5`, `check.card.4`, `param.add.priority.summary`, `param.add.severity.summary`, `param.card.value.summary`, `refusal.dinah.unknown-level`, `check.contents.2`, `check.tree.4` and `refusal.dinah.unknown-depth`. All nine mean the same concept, a value drawn from a declared ordered set, with no second sense to carve out the way "root" has one, so the trigger is the bare word "level", case-insensitive, word-bounded. German already carries `Stufe` on eight of the nine; only `check.card.4` needs the backfill. Hindi already carries `स्तर` on all nine; no Hindi fix is needed for this term.

Finding a fourth term this way does not make a fifth one findable the same way, and claiming otherwise would repeat the mistake this rewrite exists to correct. The next section says plainly what actually catches the next one, since naming a hand method here is not the same as specifying who runs it, when, and what happens if nobody does.

**The mechanism for adding a term later, specified rather than named.** Completeness is not claimed beyond the literal-duplicate-string case and the four words this pass had reason to check by name (`state`, `the root`, `owner`, `level`). A fifth term recurring across non-identical English is not found by any test this card ships, by construction: deciding that two different English sentences are about the same underlying concept is a reading a person does, not an algorithm this guard runs. What follows is where that reading is required to happen, rather than left to whichever agent happens to notice.

- **Who.** The author of any diff that adds a new key to `internal/msg/locales/en.json`, or changes an existing key's English `text`.
- **When.** Before that diff is handed to Agent Code Review, in the same card that changes `en.json`.
- **Trigger.** The diff touches `en.json`'s key set or an existing key's `text` field. This is a superset of the trigger Layer 3 already declares for locale-touching diffs (`de.json`, `hi.json`, or a future `Complete` catalog): a diff can trip this one without tripping that one, when it changes the English before any translation of the new text exists to review.
- **Where it is written.** The author re-runs the duplicate-English-text grouping (the script above, or `go test -run TestATranslationTracksItsEnglishSource -v`, which builds the same roster of duplicate-text keys as a side effect of its own subtest names) against the changed catalog, reads every group whose translations diverge, and for a suspect word recurring across *non*-identical English, greps `en.json` for that word by hand, the way `root`, `state`, `owner` and `level` were each checked here. A term this reading confirms is added to `glossary.json` in the same diff, with evidenced accepted forms per language, the way this spec's four terms were each seeded from a worked list of occurrences rather than a guess.
- **What stops it being forgotten.** Agent Code Review's column instructions (id `4b38abe7ebd5`) gain a second new subsection, alongside the Layer 3 subsection this card already adds (AC-11 below), reading:

```
## A changed or new English key re-runs the glossary sweep

A diff that adds a key to internal/msg/locales/en.json, or changes an
existing key's English text, is reviewed with one further check: re-run the
duplicate-English-text grouping (internal/msg's TestATranslationTracksItsEnglishSource
builds the same roster of duplicate-text keys as a side effect) and read
every group whose translations diverge for the changed keys. A divergence
that looks like a wrong word for a recurring concept, rather than legitimate
grammatical variation, is a [major] finding: push the diff back until
glossary.json declares the term, with evidenced accepted forms per
language, and the catalog is backfilled, the way dinah-252 seeded "state",
"the root", "owner" and "level". This check cannot be exhaustive over every
possible term (dinah-252's own spec says what the duplicate-English method
proves and does not); it is the point where a missed term is caught by a
second reader instead of never.
```

- **What makes the guard fail if the glossary is never touched.** No `go test` can fail for a term nobody declared; that is the permanent limit of a seeded declaration, not a defect this card can close. What actually answers "what stops this being forgotten" is the review gate above, the same enforcement shape and the same column Layer 3 already uses for its own semantic-decision record: a diff changing `en.json` with no corresponding glossary check is a [major] finding at Agent Code Review. `TestATranslationUsesTheDeclaredWord` itself keeps failing, as designed, the moment a term *is* declared and an entry does not carry an accepted form of it, which is what caught `check.mcp.2`, the six "owner" entries, and `check.card.4` here. It was never the thing that could notice an undeclared term, and this rewrite stops implying otherwise.

**Why "the root" and not "root".** `en.json`'s 11 occurrences of the bare word "root" are not all the same concept. Four of them — `check.mcp.2`, `mcp.reach`, `mcp.reach.nodefault`, `refusal.dinah.outside-root` — mean the root directory and contain the literal phrase "the root". Two of them — `param.contents.depth.summary` ("written as root, cards, entities, or all") and `param.tree.depth.summary` ("written as root, groups, or cards") — use "root" as one member of a closed vocabulary of literal `--depth` values, kept untranslated in both German and Hindi today exactly as `cards`/`entities`/`all`/`groups` are, and correctly so. A bare-word trigger would demand `Wurzelverzeichnis`/`रूट` in those two entries and fail a translation that is already right. The phrase "the root" (case-insensitive, word-bounded) matches the first four and none of the five other "root" occurrences (`check.mcp.1`'s `--root` reference, `init.done`'s `{root}` placeholder, `status.workbench`'s `{root}` placeholder, and the two `refusal.*.next` lines naming the `--root` flag). This is the check-the-whole-corpus discipline the column instructions require: every entry containing "root" is listed above, by key, not sampled.

**Why "state" is a bare word.** All 82 English occurrences of `\bstate\b` mean the workbench-state concept with one exception: `param.tree.group-by.summary`, whose text is `"...; state,substate when you name none"` and whose context reads "The axis names themselves are machine vocabulary and are never translated." That exception is caught by the context rule below, not by narrowing the trigger.

**Why "owner" is a bare word, and the fix that ships it green.** Every one of the 18 keys where the English word "owner" appears refers to the same concept: the actor a request or a state names as its owner. There is no second sense to carve out the way "root" has one, so the trigger is the bare word, case-insensitive, word-bounded. The correct German rendering is `Akteur`, not `Eigentümer` or `Eigner`: it is the word 12 of the 18 keys already use, and it is the word the project's own `--actor` flag summary uses (`flag.actor.summary`: "Act as this owner" → "Als dieser Akteur handeln"), so it is the rendering consistent with the project's own vocabulary for the concept rather than a generic dictionary synonym for "owner". Hindi already uses one word, `स्वामी`, on every one of the 18 keys; no Hindi fix is needed for this term except the two entries named next. German needs six: `check.pull.1` (`Eigentümer` → `Akteur`), `check.card.5` and `column.states.owner` (`Eigner` → `Akteur`), and `check.claim.3`, `refusal.dinah.takes-no-work` and `check.claim-where-no-work-is-taken`, whose current German paraphrases the concept away entirely (`Inhaber`/`Anfragende` in the first, `niemand` in the other two) rather than naming it, and needs `Akteur` worked in. `refusal.dinah.takes-no-work` and `check.claim-where-no-work-is-taken` need the same treatment in Hindi too, since both currently drop `स्वामी` the same way German drops `Akteur` (both translators independently chose the same paraphrase for the same two keys, which is a real linguistic signal but not a licence to leave two entries the seeded term cannot see; consistency with the other 16 keys wins, per the same backfill-before-green precedent this card already applies to `check.mcp.2`). The exact wording in every case is Implement's call, mirroring the `check.mcp.2` precedent below; only that each contains an accepted form of the declared term is this spec's requirement.

**Placeholder stripping.** Before a trigger is matched, every `{...}` span is removed from the base English text. Without this, `card.line`'s `"{ref}  {title}  [{state} / {substate}]"` would trigger the "state" term on its `{state}` placeholder even though the line carries no prose word at all, and `mcp.reach`'s `{root}` placeholder would trigger "the root" a second, redundant time.

**Context exclusion.** An entry whose base `Context` contains the phrase `"never translated"` is skipped by every term, reusing the same declared signal `TestEveryUntranslatableIdentifierSurvivesTranslation` already reads (`internal/msg/msg_test.go`, existing). This is what excludes `param.tree.group-by.summary`. See the note on dinah-258 below: this phrase-selector is known incomplete, and no live entry today is affected by that gap for any of the four seeded terms.

**The guard**, in `internal/msg/msg_test.go`, next to `TestATranslationTracksItsEnglishSource`:

```go
func TestATranslationUsesTheDeclaredWord(t *testing.T) {
	if len(glossary) == 0 {
		t.Fatal("the glossary carries no terms, so this guard is asserting nothing")
	}
	strip := regexp.MustCompile(`\{[^}]*\}`)
	trigger := make(map[string]*regexp.Regexp, len(glossary))
	for _, term := range glossary {
		trigger[term.EN] = regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(term.EN) + `\b`)
	}
	checked := 0
	for _, tag := range Tags() {
		if tag == Base {
			continue
		}
		t.Run(tag, func(t *testing.T) {
			for _, key := range Keys() {
				base, ok := BaseEntry(key)
				if !ok || strings.Contains(base.Context, "never translated") {
					continue
				}
				entry, carried := CatalogEntry(tag, key)
				if !carried || entry.Skeleton {
					continue
				}
				plain := strip.ReplaceAllString(base.Text, "")
				for _, term := range glossary {
					if !trigger[term.EN].MatchString(plain) {
						continue
					}
					forms, declared := term.Forms[tag]
					if !declared {
						continue
					}
					checked++
					ok := false
					for _, form := range forms {
						if strings.Contains(entry.Text, form) {
							ok = true
							break
						}
					}
					if !ok {
						t.Errorf("%s: wanted the glossary word for %q (one of %v), got %q", key, term.EN, forms, entry.Text)
					}
				}
			}
		})
	}
	if checked == 0 {
		t.Fatal("no entry triggered a glossary term, so this guard is asserting nothing")
	}
}
```

Note the loop now iterates `Tags()` and skips `Base`, and reads a translated catalog's entry through `CatalogEntry` rather than the package-private `loaded` map, exactly the shape the contract-token guard below already uses. This replaces the first draft's `for _, tag := range Complete`, which Agent Design Review correctly flagged as reproducing a roster-fragility pattern: `dinah-287`, an unmerged card, removes German and Hindi from `Complete`, and the moment it lands, a `Complete`-keyed loop here would find zero tags to check and hard-fail on `checked == 0`, asserting nothing rather than testing anything. Workbench document 51's own "Amendment: what the guard iterates, after dinah-287" section describes `TestATranslationTracksItsEnglishSource` as already iterating `Tags()` for this reason, but that section describes `dinah-287`'s unmerged branch, not the trunk this spec is written against. Confirmed directly against `internal/msg/msg_test.go` on `origin/main` while writing this revision, `TestATranslationTracksItsEnglishSource` still iterates `Complete` today. This guard does not depend on `dinah-287` landing either way, since `Complete` and `Tags()` name the same roster on trunk right now; it adopts `Tags()` from the start because design review found that shape sound, not because it is landed precedent.

**Cost.** At most 631 keys × 4 declared terms × 2 complete non-English catalogs ≈ 5,048 regex/substring checks, each over one short string. No I/O beyond the embedded files already loaded at package init, no network, no LLM.

**Landing green, not red.** Three live defects this layer exists to catch, all corrected as part of landing this card rather than left for the guard to find red on day one, mirroring how dinah-249 backfilled `source` before its own guard shipped:

- `check.mcp.2`'s German text reads `"die genannte Werkbank liegt unter --root"` today; Implement corrects it to contain `Wurzelverzeichnis`, matching `mcp.reach`, `mcp.reach.nodefault` and `refusal.dinah.outside-root`, which already use it correctly for the same concept.
- The six German entries and two Hindi entries named above under "Why 'owner' is a bare word" are corrected to contain `Akteur` (German) or keep/gain `स्वामी` (Hindi).
- `check.card.4`'s German text reads `"das Feld deklariert diesen Wert"` today, dropping "level" entirely; Implement corrects it to contain `Stufe`, matching its near-twin `check.add.5`, which already uses it correctly for the same concept.

Every corrected entry gets a fresh `source` with `Fingerprint` of its English, the same mechanical step dinah-249's own guard requires whenever text changes. The exact wording in each case is Implement's call; only that it contains the declared form is this spec's requirement.

## Layer 1: the declared layer, part B — contract tokens

**What it catches.** A word that names a thing on this project — a command, a substate, a level axis, a flag, the workbench anchor filename — respelled or dropped in translation. dinah-245 coined three different German names for `--root` across four strings; a check keyed on this layer would have caught it the same way it catches `check.mcp.1`'s and `refusal.dinah.unknown-root.next`'s already-correct verbatim uses of `--root`.

**Where this guard lives, and why not `internal/msg`.** The token set is composed from each vocabulary's own live declaration (below) rather than hand-copied, per the column's "prefer a declaration to an inference" rule. Reading those declarations means importing `internal/verb`, `internal/bench` and `internal/contract`. `internal/verb/read.go` already imports `internal/msg`, so `internal/msg` importing `internal/verb` back would cycle. This guard is therefore a new file in `internal/verb`, alongside the package's existing `TestVersionCarriesTheConformanceClaim`, which already reads `msg.Complete` and `msg.Skeleton` from outside `internal/msg` — the same cross-package shape, extended to read entry text as well as the completeness roster.

**The accessor this needs, and the decision to add it.** dinah-249's code review flagged that a guard reaching a whole catalog from outside `internal/msg` has no way in, since `loaded` is unexported, and ruled that nothing should be exported until a caller exists. This guard is that caller: it needs a translated catalog's raw entry (its `Text` and `Skeleton` flag, with no fallback to English and no placeholder substitution) for a tag that is not necessarily `Base`. Add to `internal/msg/msg.go`, beside `BaseEntry`:

```go
// CatalogEntry returns one entry exactly as tag's own catalog carries it: no
// fallback to Base, no placeholder substitution. BaseEntry is this function
// specialized to tag == Base. A caller outside this package that needs a
// translation's own Skeleton flag or raw Text, rather than what a reader
// would see rendered, calls this instead of reaching into Renderer.
func CatalogEntry(tag, key string) (Entry, bool) {
	catalog, ok := loaded[tag]
	if !ok {
		return Entry{}, false
	}
	entry, ok := catalog.Entries[key]
	return entry, ok
}
```

Add `TestCatalogEntryReadsWithoutFallback` to `internal/msg/msg_test.go`:

```go
func TestCatalogEntryReadsWithoutFallback(t *testing.T) {
	if entry, ok := CatalogEntry("de", "word.yes"); !ok || entry.Text == "" {
		t.Fatalf("wanted the German entry, got %+v, %v", entry, ok)
	}
	if _, ok := CatalogEntry("qq", "word.yes"); ok {
		t.Error("wanted false for a language no catalog answers to")
	}
	if _, ok := CatalogEntry("de", "no.such.key"); ok {
		t.Error("wanted false for a key the catalog does not carry")
	}
}
```

**Token composition**, new file `internal/verb/contract_tokens_test.go` (package `verb`, so it may read the unexported `params` map `internal/verb/definition.go` already declares):

```go
package verb

import (
	"regexp"
	"sort"
	"strings"
	"testing"

	"dinah/internal/bench"
	"dinah/internal/contract"
	"dinah/internal/msg"
)

// contractTokens is the closed set of machine vocabulary this project's own
// commands, substates, level axes and flags are spelled with, composed from
// each vocabulary's own declaration so a name added there is protected
// without a second list to keep in step. "stdout" is written by hand: it
// names a Unix stream, not anything this project declares a constant for,
// and en.json's cmd.export.summary is the one place it appears.
func contractTokens() []string {
	set := map[string]bool{
		contract.SubstateReady:   true,
		contract.SubstateActive:  true,
		contract.SubstateBlocked: true,
		bench.WorkbenchAnchor:    true,
		"stdout":                 true,
	}
	for _, axis := range bench.LevelAxes {
		set[axis] = true
	}
	for _, name := range Commands() {
		set[name] = true
		for _, p := range params[name] {
			if !p.Flag {
				continue
			}
			shown := p.Name
			if p.Display != "" {
				shown = p.Display
			}
			set["--"+shown] = true
		}
	}
	tokens := make([]string, 0, len(set))
	for t := range set {
		tokens = append(tokens, t)
	}
	sort.Strings(tokens)
	return tokens
}

var backtickSpan = regexp.MustCompile("`([^`]+)`")

// literalTokens returns the contract tokens key's English text exposes as a
// literal a reader is meant to copy: every backtick-quoted span that is
// exactly one token, and — when key is a token.* entry — the whole text
// when the whole text is one token. Ordinary prose using one of these words
// as a plain verb or noun, such as cmd.archive.summary's "Move a card...",
// is neither, and is deliberately not checked: word-boundary matching over
// every English sentence would fail that sentence's correct German
// ("Eine Karte ... nehmen") for translating an ordinary verb.
func literalTokens(key, text string, tokens map[string]bool) []string {
	var found []string
	for _, m := range backtickSpan.FindAllStringSubmatch(text, -1) {
		if tokens[m[1]] {
			found = append(found, m[1])
		}
	}
	if strings.HasPrefix(key, "token.") {
		if trimmed := strings.TrimSpace(text); tokens[trimmed] {
			found = append(found, trimmed)
		}
	}
	return found
}

func TestContractTokensSurviveInBackticksAndBareTokenEntries(t *testing.T) {
	tokens := map[string]bool{}
	for _, tok := range contractTokens() {
		tokens[tok] = true
	}
	checked := 0
	for _, key := range msg.Keys() {
		base, ok := msg.BaseEntry(key)
		if !ok {
			continue
		}
		wanted := literalTokens(key, base.Text, tokens)
		if len(wanted) == 0 {
			continue
		}
		for _, tag := range msg.Tags() {
			if tag == msg.Base {
				continue
			}
			entry, carried := msg.CatalogEntry(tag, key)
			if !carried || entry.Skeleton {
				continue
			}
			checked++
			for _, tok := range wanted {
				if !strings.Contains(entry.Text, tok) {
					t.Errorf("%s/%s: wanted the contract token %q untranslated, got %q", tag, key, tok, entry.Text)
				}
			}
		}
	}
	if checked == 0 {
		t.Fatal("no entry carried a contract token in a backtick span or a token.* entry, so this guard is asserting nothing")
	}
}
```

**Cost.** 631 keys scanned for a backtick span or a `token.*` match (cheap regex, no catalog lookups needed unless one is found); of those, at most 7 non-English shipped catalogs each. Worst case 631 × 7 ≈ 4,417 substring checks; in practice far fewer, since only entries actually carrying a backtick or bare-token literal reach the inner loop.

**Relationship to dinah-258.** dinah-258's guard (`TestEveryUntranslatableIdentifierSurvivesTranslation`) selects entries by a context phrase and checks ALL-CAPS identifiers within them; this contract-token guard selects entries structurally (backticks, `token.*` keys) and checks a fixed, project-declared vocabulary. Neither reads the other's selector, so the contract-token guard does not depend on dinah-258's fix and does not duplicate it. See the section below for the glossary guard's different relationship to dinah-258, which is not the same claim.

## Layer 2: reachability

**What it catches.** A key translated correctly that a reader in that language never sees, because the code path that would render it is pinned to one language. Agent Code Review on dinah-245 found this concretely: the MCP surface always renders in English. Verified again here, directly:

```
$ git grep -n 'msg\.For(' -- '*.go' | grep -v _test.go
cmd/dinah/main.go:102:    r: msg.For(bench.ResolveLang(parsed.value("lang"), cfg)),
internal/mcp/mcp.go:166:  catalog := msg.For(msg.Base)
internal/mcp/tools.go:102: catalog := msg.For(msg.Base)
internal/mcp/tools.go:138: catalog := msg.For(msg.Base)
```

`cmd/dinah/main.go`'s call is parameterized on the resolved language; the three `internal/mcp` calls are pinned to `msg.Base`. This is every `msg.For` call site in the repository today (confirmed by grepping every `.go` file, not the ones already suspected).

**Design.** This does not compute full reachability — that would need call-graph analysis this card cannot bound the cost of. It instead makes the pinned set visible and stops it from growing silently, per the operator's dinah-249 OQ-4 ruling that reachability folds into this card. New file `internal/verb/reachability_test.go`:

```go
package verb

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// PinnedCallSite is one place msg.For is called with a fixed language
// instead of the caller's own. Every one the parser finds has to be entered
// here, with a reason, or the guard below fails — turning an invisible
// English-only surface into a line somebody reviewed and signed off on.
type PinnedCallSite struct {
	File   string
	Line   int
	Reason string
}

// declaredPinnedCallSites is the reviewed inventory, seeded with the three
// call sites Agent Code Review found on dinah-245 and this spec reconfirmed
// by grep above.
var declaredPinnedCallSites = []PinnedCallSite{
	{File: "internal/mcp/mcp.go", Line: 166, Reason: "the MCP surface is deliberately pinned to English; dinah-245"},
	{File: "internal/mcp/tools.go", Line: 102, Reason: "the MCP surface is deliberately pinned to English; dinah-245"},
	{File: "internal/mcp/tools.go", Line: 138, Reason: "the MCP surface is deliberately pinned to English; dinah-245"},
}

// scanDirs excludes internal/msg itself: every msg.For call there is the
// renderer's own fallback logic, not a caller choosing a language.
var scanDirs = []string{"../../cmd/dinah", "../../internal/mcp", "../../internal/verb"}

func TestEveryLanguagePinnedCallSiteIsDeclared(t *testing.T) {
	found := findPinnedCallSites(t)
	declared := map[string]bool{}
	for _, d := range declaredPinnedCallSites {
		declared[d.File+":"+strconv.Itoa(d.Line)] = true
	}
	seen := map[string]bool{}
	for _, f := range found {
		key := f.File + ":" + strconv.Itoa(f.Line)
		seen[key] = true
		if !declared[key] {
			t.Errorf("%s:%d: msg.For is called with a fixed language and is not in declaredPinnedCallSites; parameterize it on the caller's language, or add it there with a reason", f.File, f.Line)
		}
	}
	for key := range declared {
		if !seen[key] {
			t.Errorf("declaredPinnedCallSites carries %s, but the parser found no pinned msg.For call there any more; remove the stale entry", key)
		}
	}
}

func findPinnedCallSites(t *testing.T) []PinnedCallSite {
	var out []PinnedCallSite
	fset := token.NewFileSet()
	for _, dir := range scanDirs {
		filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return err
			}
			file, err := parser.ParseFile(fset, path, nil, 0)
			if err != nil {
				t.Fatalf("parsing %s: %v", path, err)
			}
			ast.Inspect(file, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok || len(call.Args) != 1 {
					return true
				}
				sel, ok := call.Fun.(*ast.SelectorExpr)
				if !ok || sel.Sel.Name != "For" {
					return true
				}
				pkg, ok := sel.X.(*ast.Ident)
				if !ok || pkg.Name != "msg" {
					return true
				}
				pos := fset.Position(call.Pos())
				rel := relPath(t, path)
				switch arg := call.Args[0].(type) {
				case *ast.BasicLit:
					out = append(out, PinnedCallSite{File: rel, Line: pos.Line, Reason: "msg.For called with a string literal"})
				case *ast.SelectorExpr:
					if arg.Sel.Name == "Base" {
						out = append(out, PinnedCallSite{File: rel, Line: pos.Line, Reason: "msg.For called with msg.Base"})
					}
				}
				return true
			})
			return nil
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].File != out[j].File {
			return out[i].File < out[j].File
		}
		return out[i].Line < out[j].Line
	})
	return out
}
```

`relPath` normalizes a walked path (e.g. `../../internal/mcp/mcp.go`) to the repo-root-relative form the table above uses (`internal/mcp/mcp.go`); Implement writes it as a small `filepath.Rel`-based helper, since its exact shape depends on how `go test` resolves `scanDirs` from `internal/verb`'s own directory.

**Cost.** Parses roughly two dozen `.go` files under the three scanned directories once per test run; no per-catalog-entry cost, no LLM, no network.

**Scope decision: the second dinah-245 finding is not this guard's job.** dinah-245 also found that four strings never print at all because the CLI emits a bare refusal name instead of rendering the message. That is a defect in `cmd/dinah`'s own refusal-formatting code — nothing about the catalog or a translation is wrong — and no static check over `msg.For` call sites can see it, since the bug is precisely that no such call happens. It is out of this card's scope; whoever fixes the CLI's refusal rendering fixes it there.

## Layer 3 (declared, not built): the semantic layer

**Decision: it does not run as a Go test, and it is never part of `go test ./...` or CI.** Two properties of an LLM-graded check are true regardless of model or prompt: it is non-deterministic, so the same code can pass on one run and fail on the next with nothing having changed, and it costs money and latency per call. A repository guard that can fail with no code change is exactly the "green on your machine is not green" problem the workbench instructions already name, except worse: here the same commit can be red on one CI run and green on the retry. Running it over all 631 keys × up to 8 catalogs on every push, the shape `TestATranslationTracksItsEnglishSource` and the two guards above use, would also be the exact cost mistake the column instructions warn against by name (629 keys × 8 catalogs, an unbounded per-entry sweep) — except paid per call instead of free.

**What carries it instead, and what makes it structural rather than a hope.** The judgment call this layer makes, whether a translation still says what the English says, is a call dinah-249's own workbench-document workflow already asks a human or an LLM to make, at the moment a translation's `source` goes stale and somebody is about to write a fresh one (workbench document 51, "Translation staleness contract", section "What you do when the guard fails on your change"). This layer does not add a second pass; it names and records the pass that workflow already requires. Concretely: any card whose diff touches `internal/msg/locales/de.json`, `hi.json`, or a future catalog `Complete` names, records one `decision`-kind checklist item per key its diff changes, of the shape "`<key>`: read against the current English and its context; \[unchanged / retranslated\] because \[reason\]."

Design review correctly found that this convention, as first drafted, existed only as a description in this card's own spec and in workbench document 51, with nothing making Agent Code Review actually check for it. Read directly: Agent Code Review's own column instructions (id `4b38abe7ebd5`) name a "Convention counterexamples" corpus check and a "Prose standard" check, both by explicit cross-reference, but carry no comparable line for a locale-touching diff and no reference to workbench document 51 at all. A future reviewing agent inheriting only that column's instructions would have no reason to look for the record this layer depends on, which is precisely the dinah-259 failure mode this card cites as its own justifying evidence: a translation that is fluent, current and on-glossary, and wrong in meaning, passing every mechanical guard because nothing mechanical checks meaning.

The fix is to add the check to the one surface a reviewing agent is already reading. Implement (or whoever lands this card, using `update_column_instructions` or the equivalent live-configuration write path; this is not a repository file and no `git` command reaches it) appends the following to Agent Code Review's column instructions, verbatim, as its own subsection:

```
## Locale diffs carry a decision record

A diff touching internal/msg/locales/*.json is reviewed as any other, with
one addition: for every key the diff changes, confirm the card carries a
resolved decision-kind checklist item of the shape "<key>: read against the
current English and its context; [unchanged/retranslated] because [reason]."
(Workbench document 51, "Translation staleness contract", names the
convention and what an author does when the read finds a real mismatch.) A
locale-touching diff carrying no such record for a changed key is a [major]
finding, pushed back to Implement/Ready to add the missing record, not a nit
to note and pass on.
```

This is the enforcement path, not the workbench document alone: workbench document 51 is where an author learns the convention before writing a translation; the column instructions are where a reviewer is required to check it. Both are necessary and neither substitutes for the other, which is why AC-9 (the workbench document content) and the new criterion below (the column instructions content) are separate and both required.

**Bound.** The population this touches is "however many locale lines one card's own diff changes," categorically smaller than a whole-catalog sweep — a handful to a few dozen entries per card, not 629 × 8. No future card should read this layer as license to run it over an entire catalog in one pass; if that ever seems useful, it is a new decision with its own cost argument, not an extension of this one.

**What an author does when the read finds a real mismatch.** Rewrite the translated text, then write a fresh `source` with `Fingerprint` of the current English, the same mechanical step `TestATranslationTracksItsEnglishSource` already requires whenever `source` goes stale. Nothing new to do beyond recording which of the two happened, in the decision above.

**Evidence this scope is right.** dinah-259 (a German `check.*` line stating a condition as fact where the English qualifies it) is exactly the failure shape this layer exists for, fluent, current, on-glossary, and wrong, and it is a single-entry defect on a card that, when worked, will touch exactly the locale lines it fixes: precisely the bound above. It stays its own card; it is cited here only as evidence, not folded in.

## dinah-258, and whether it undermines this card's guards

dinah-258 says the catalog guard `TestEveryUntranslatableIdentifierSurvivesTranslation` recognises only the literal context phrase "never translated" and misses a second wording, "the same in every language", that two entries (`token.dinah-editor`, `token.visual`) use to declare the identical property. The question is whether that incompleteness reaches this card's guards, and whether the two cards can stay separate.

**The contract-token guard does not share the risk.** It selects entries structurally, by backtick span or `token.*` key, and never reads either phrase. dinah-258's fix or its absence changes nothing about what this guard checks.

**The glossary guard does share it, in one place.** Its context exclusion reads `strings.Contains(base.Context, "never translated")` to skip entries pinned to a literal spelling, the same phrase dinah-258 names as incomplete. An entry declared untranslatable only via "the same in every language" would not be excluded here either, and if such an entry's English also triggered a glossary term, the guard would demand a translated word a correctly-untranslated entry does not carry: a false failure, not a caught defect.

That is a real structural gap, and it is checked here rather than assumed away. Every entry in `en.json` whose context carries "the same in every language" today:

```
refusal.dinah.unknown-field.ordered  (text: " Only {instantField} accepts >=, <=, > and <.")
token.dinah-editor                   (text: "DINAH_EDITOR")
token.visual                         (text: "VISUAL")
```

None of the three texts contains "state", "the root", or "owner" (case-insensitive, word-bounded), so no live entry fails today because of this gap. That is the same finding the review already made and it is reconfirmed here directly against the corpus rather than taken on trust, because a gap that is provably harmless today is still a gap and the check is what makes "provably" true instead of assumed.

**The two cards stay separate.** dinah-258 is a narrow fix to an existing test's selector, already scoped and already sitting in Intake; this card's glossary guard is new code that happens to reuse the same phrase for the same purpose. This card does not depend on dinah-258 landing first (the shared phrase works correctly for every entry that exists today), and folding a review of an unrelated card's fix into this one would gate this card's landing on a card nobody has claimed, for a risk that is currently zero. The honest statement of the shared fate: if dinah-258's fix turns the phrase-selector into a shared, structured signal (the "structured field" option its own description raises and leaves open), whoever lands that fix should update this glossary guard's exclusion to read the same signal, so the catalog keeps exactly one definition of "declared untranslatable" rather than two that can drift apart silently. That is a note for dinah-258's implementer, not a blocker on this card, and it is written here so the note is not lost between the two cards' threads.

## What this card does not do

Neither dinah-258 nor dinah-259 is folded into this card (reasoning above, at each layer). Neither is refolded into dinah-249, which is Done with its own ten acceptance criteria closed.

## The workbench document

`Translation staleness contract` (workbench document 51) already carries a section header "dinah-252's section of this document," written by dinah-249's Implement as a placeholder pointing here. Implement replaces that placeholder with the glossary format and its four seeded terms, the term-adding mechanism specified above, the contract-token list and its backtick/`token.*` scope, the reachability inventory and what it does and does not cover, and the semantic-layer procedure (where it runs, how often, what makes it fail, what an author does), each in the terms this spec used above but citing the real field, function and test names Implement lands. Not drafted here, for the same reason dinah-249's D-6 gives: the accurate names exist only once the code does.

## Branch

dinah-252-a-translated-entry-can-carry-the-wrong-word-or-never-reach-a-reader-in-that-language-and-nothing-catches-either
