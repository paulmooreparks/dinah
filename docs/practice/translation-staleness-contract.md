# Translation staleness contract

A translated message in Dinah's catalogs can fall behind the English it was translated from, and nothing about the translation looks wrong when it happens. The German still reads as German and the Hindi still reads as Hindi, so no check that asks "is this string still English?" can see the drift. This document describes the mechanism that turns that drift into a failing test, and it tells you what to do when the test fails on your change.

## What a translated entry records

Every message lives in `internal/msg/locales/<tag>.json` under a key the code names it by. A translated entry carries three fields:

```json
"card.blocked": {
  "text": "  blockiert: {reason}",
  "context": "Printed under the card line when the card is blocked.",
  "source": "fcdf8f7dac8e539d"
}
```

`source` is a fingerprint of the English text at the same key, taken at the moment somebody translated the entry. It records what the translator was looking at. When the English at that key later changes, the fingerprint the entry stores and the fingerprint of the new English no longer agree, and the guard below reports the key.

The field is `Source` on the `Entry` struct in `internal/msg/msg.go`, tagged `json:"source,omitempty"`.

The entry stores a hash rather than the English text itself, for two reasons. A fixed-width hash costs the same however long a message grows, where a second copy of English prose would roughly double the size of every translated catalog, and these catalogs are compiled into the binary with `go:embed`. A stray copy of English sitting inside a translated catalog also reads, at a glance, like a translation aid a translator should trust, when what it really holds is an aging snapshot. A hash cannot be misread that way, and `en.json` is one file away whenever you want the current English.

## Which entries carry a source

`en.json` carries none, because an English entry is not a translation of anything.

An entry marked `"skeleton": true` carries none either. A skeleton entry holds the English text unchanged, which is what a generated catalog ships until somebody translates it, so there is no translation to have fallen behind. The five skeleton catalogs are `cs`, `id`, `es`, `fil` and `af`.

That exemption rests on a decision the operator made rather than on any property of the existing test suite. The skeleton catalogs are temporary and are scheduled to be translated, nobody uses them for reading yet, and a guard built over them would protect a category on its way out. There is a harder reason behind that. An earlier check that treated the skeletons as a class regenerated `de.json` from the English templates and destroyed the German translation, which dinah-211 then spent a whole card restoring by hand. Any future proposal to run a standing generator across the catalogs has to answer that history first.

Do not justify the exemption by claiming that `TestEveryDeclaredLanguageShips` already holds a skeleton entry to the English. That test counts keys and counts how many entries carry `Skeleton: false`, and it compares no bytes at all. The claim has been checked and it is false.

A skeleton catalog joins the mechanism on its own the day somebody translates it. The exemption reads the `Skeleton` flag on each entry rather than a list of language tags, and the guard's subtests come from `Complete`, so a catalog that moves from `Skeleton` to `Complete` gains a subtest and its entries gain the obligation with no code change. Nobody has to plan a migration for that.

## Fingerprint

`msg.Fingerprint(text string) string` computes the digest, using FNV-1a 64-bit from the standard library's `hash/fnv` and formatting the sum with `strconv.FormatUint(sum, 16)`.

It is the one place this project computes that digest. The guard calls it to check a source and the migration that populated the catalogs called it to write one, so no second implementation exists to disagree with the first. If you write a tool that touches `source`, call `Fingerprint` rather than hashing the text yourself.

Two calls on the same text return the same value on any machine and under any Go release, because FNV-1a is a pure function of the bytes handed to it and `hash/fnv` fixes the algorithm.

## The guard

`TestATranslationTracksItsEnglishSource`, in `internal/msg/msg_test.go`, runs one subtest per tag in `Complete` other than `Base`. Each subtest walks the base catalog's keys, skips any entry its catalog does not carry or marks as a skeleton, and compares the entry's `source` against `Fingerprint` of the current English text.

Two failures are possible and they read differently. An entry with an empty `source` is reported as carrying no recorded source. An entry whose `source` disagrees with the English of the day is reported as stale, with a pointer to `git log -p internal/msg/locales/en.json` so that a reader who arrives days after the English edit can find out what changed.

The subtest boundary is what keeps a failure readable. One English edit touches every translated catalog at that key, and without a subtest per language the output repeats the same key once per catalog with nothing to say which language you are looking at. The guard also fails outright when it finds no translated entry to check, so a change that quietly empties `Complete` cannot leave a green test asserting nothing.

## What you do when the guard fails on your change

You edited an English message, and one or more translations now point at the older wording. The guard is asking you to make a decision, and it is asking now, while you still have the two versions of the English in front of you in your own diff.

For each key the guard names, read the current English text and its `context`, then read the existing translation. Decide whether the translation still says what the English now says. Sometimes it does, because the English edit changed punctuation or tightened a phrase without moving the meaning. Sometimes it does not, and the translation needs rewriting.

Then write a fresh `source` for the entry, equal to `Fingerprint` of the current English text, in the same commit that changed the English. Whether or not you rewrote the translated text, the new fingerprint records that somebody looked at both and made a call.

Do not reach for the second option that will occur to you, which is to delete the `source` field to make the failure go away. An entry with no source fails the guard as well, with the other message, and an entry given a fingerprint nobody checked against the English is a silenced check wearing the costume of a passing one.

## What this mechanism does not see

The guard answers one question, which is whether a translation still matches the English it came from. Three failures sit outside it.

A gap that predates the baseline stays invisible. The migration that populated `source` stamped the current English fingerprint onto the translations as they stood, so a translation that was already missing something the English said now records the English of the day and passes. German's `help.environment` omitted `DINAH_MCP_ROOT` in exactly that way, and dinah-248 fixed the content on its own. Landing this mechanism does not retroactively find a gap of that shape; it stops the next one from opening.

Terminology drift inside one language stays invisible too. A translation that renders a term with the wrong word is still, in every other respect, a faithful and current rendering of its English source, so its fingerprint agrees and the guard passes. That belongs to dinah-252.

Reachability stays invisible as well. A key can be translated correctly and still never reach a reader in that language, because the surface it prints on is pinned to English or because the command emits a bare refusal name instead of rendering the message. Answering that needs a check over the call sites rather than over the catalogs, so it is a different kind of guard from either of the two described here. The operator has ruled that the work folds into one of the two existing translation-quality cards rather than becoming a card of its own. dinah-252 is the recommended home, because dinah-249 shipped the catalog-side mechanism and closed.

## dinah-252's section of this document

dinah-252 landed three guards and one deliberate non-guard, all beside the mechanism above rather than inside it. Each answers a question a fingerprint match cannot: whether the translation chose the right word, whether it left the project's own names alone, and whether a reader in that language ever sees it.

### The glossary

`internal/msg/glossary.json` is an embedded list of terms, read by `loadGlossary` in `internal/msg/glossary.go` into the package-level `glossary`. A term is a `glossaryTerm`, which carries `EN`, the phrase whose presence in the base English triggers the check, and `Forms`, a map from language tag to every rendering that language is allowed to use.

Four terms are seeded, each from a hand-read list of every English occurrence rather than from a guess:

| term | German forms | Hindi forms |
|---|---|---|
| `state` | Zustand, Zustands, zustand | स्थिति |
| `the root` | Wurzelverzeichnis | रूट |
| `owner` | Akteur | स्वामी |
| `level` | Stufe, stufe | स्तर |

A term carries more than one form where the language inflects the word or spells it inside a compound. German writes `Zielzustand` for a destination state and `Tiefenstufe` for a depth level, and both of those carry the term; the lowercase forms are what let a compound through. Add a form with the evidence for it when the corpus turns out to need one, rather than widening the trigger or the match.

`the root` is a phrase and not the bare word, because `root` names a second thing in this catalog. It is one member of the closed `--depth` vocabulary, written as `root, cards, entities, or all`, which both translated catalogs correctly leave untranslated alongside `cards` and `all`. A bare-word trigger would demand a German word in two entries that are already right.

### What the trigger reads

`TestATranslationUsesTheDeclaredWord`, in `internal/msg/msg_test.go`, runs one subtest per tag in `Tags()` other than `Base`, reads each translation through `msg.CatalogEntry`, and skips an entry the catalog does not carry or marks as a skeleton. It fails an entry whose text carries none of its triggered term's declared forms, naming the key, the term, the accepted forms and the text it found.

Before a trigger is matched, `prose` removes four kinds of span from the base English, declared together as `machineSpans`: a `{brace}` placeholder, a backtick span, an `<angle>` placeholder, and a `--flag` token. Each of the four names a thing rather than saying a word about it, so a trigger firing inside one would demand a translated word where a correct translation carries none. `card.line`'s `"{ref}  {title}  [{state} / {substate}]"` is a placeholder line with no prose in it at all, and `refusal.dinah.ambiguous-state.next` names the flag three times and the concept never.

An entry whose base `Context` contains `never translated` is skipped by every term, reusing the declaration `TestEveryUntranslatableIdentifierSurvivesTranslation` already reads. dinah-258 records that this phrase selector is incomplete. Whoever lands that fix should point this exclusion at the same signal, so the catalog keeps one definition of declared-untranslatable rather than two that can drift apart.

### Adding a term later

No test can fail for a term nobody declared. That is a permanent limit of a seeded declaration rather than a defect waiting to be closed, because deciding that two differently-worded English sentences are about one concept is a reading a person does.

So the mechanism is a review-time check, and it is carried by Agent Code Review's own column instructions under the heading "A changed or new English key re-runs the glossary sweep". The author of a diff that adds a key to `en.json`, or changes an existing key's English text, re-runs the duplicate-English-text grouping, reads every group whose translations diverge, and hand-greps `en.json` for a suspect word recurring across non-identical English, the way `state`, `the root`, `owner` and `level` were each read here. A term the reading confirms goes into `glossary.json` in the same diff, with evidenced forms per language. A diff that skips the check is a [major] finding.

Say plainly what the seeding method proves. Grouping keys by identical English text is exhaustive over literal duplicates and blind to everything else, and it has missed a live defect twice: the `owner` cluster, where German used three words, and the `level` pair `check.card.4` against `check.add.5`, whose English differs by enough words that no grouping puts them together. Both were found by reading.

### Contract tokens

`TestContractTokensSurviveInBackticks`, in `internal/verb/contract_tokens_test.go`, asserts that a name this project declares travels into every translation byte-identical. `contractTokens` composes the set from each vocabulary's own declaration, so a name added there is protected with no second list to keep in step: `contract.SubstateReady`, `contract.SubstateActive`, `contract.SubstateBlocked`, `bench.WorkbenchAnchor`, every axis in `bench.LevelAxes`, every name `Commands()` returns, every flag in that command's `params`, and the literal `stdout`, which names a Unix stream this project declares no constant for.

The guard lives in `internal/verb` rather than in `internal/msg` because composing that set needs `internal/verb`, `internal/bench` and `internal/contract`, and `internal/verb/read.go` already imports `internal/msg`, so the reverse import would cycle. It reads translated entries through `msg.CatalogEntry`, the accessor dinah-252 added for it, which answers out of one catalog with no fallback to English.

`literalTokens` selects only a backtick-quoted span that is exactly one token. Ordinary prose using one of these words as a plain verb is not selected, and that is deliberate: `cmd.archive.summary`'s "Move a card, a state, or anything below a card, out of the live set" is correctly rendered in German as "Eine Karte ... nehmen", and a word-boundary sweep over every English sentence would fail it for translating a verb.

The `token.*` namespace is not selected either, and dinah-252's spec was wrong to say it should be. Those entries hold the reading a person gets for a canonical token, and every catalog is meant to translate them: `token.active`'s own context says that the machine surface always carries the canonical spelling. Selecting on that namespace fails `de/token.active` for saying `aktiv`, which is the entry doing its job. The two entries of the namespace whose rendering really is the identifier, `token.dinah-editor` and `token.visual`, declare that in their context and are guarded by `TestEveryUntranslatableIdentifierSurvivesTranslation`.

### Reachability

`TestEveryLanguagePinnedCallSiteIsDeclared`, in `internal/verb/reachability_test.go`, parses `cmd/dinah`, `internal/mcp` and `internal/verb` with `go/parser` and finds every `msg.For` call whose one argument is a string literal or `msg.Base`. That set has to equal `declaredPinnedCallSites`, a reviewed inventory of `PinnedCallSite` entries each carrying a file, a line and a reason. Three are declared today, all in `internal/mcp`, each recording that the MCP surface is deliberately pinned to English.

It fails in both directions. A new pinned call site fails until somebody enters it with a reason, and a declared entry the parser no longer finds fails as a stale exemption, so the inventory cannot go on asserting after the surface it describes has moved.

What it does not cover is worth stating, because the name invites more than the guard delivers. This is not reachability analysis. A key can be unreachable in a language for a reason no `msg.For` call site shows, and the bare-refusal-name defect dinah-245 found is exactly that shape, since the bug there is that no such call happens at all. `internal/msg` is left out of the scan because every `msg.For` in it is the renderer's own fallback logic rather than a caller choosing a language.

### The semantic layer, which is not a test

Whether a translation still says what the English says is a judgment, and dinah-252 ruled that it does not run as a Go test, is no part of `go test ./...`, and never runs in CI. An LLM-graded check is non-deterministic, so one commit could be red on a run and green on the retry, which is a worse version of a problem this board already knows; and it is priced per call, so running it over every key in every catalog on every push repeats a cost this board has paid for before.

What carries it instead is the pass the section "What you do when the guard fails on your change" already asks for, named and recorded rather than added to. A card whose diff touches a locale file records one `decision`-kind checklist item per key it changes, of the shape "`<key>`: read against the current English and its context; [unchanged / retranslated] because [reason]."

Agent Code Review is required to check for that record, under the heading "Locale diffs carry a decision record" in its own column instructions, and a locale-touching diff carrying no record for a changed key is a [major] finding. Those instructions are where a reviewer is required to check; this document is where an author learns the convention. Neither substitutes for the other.

The bound is the locale lines that one card's own diff changes, which is a handful to a few dozen entries. Nobody should read this layer as licence to run it over a whole catalog in one pass; that would be a new decision with its own cost argument rather than an extension of this one.


## Amendment: what the guard iterates, after dinah-287

Two paragraphs above are now out of date, and this section supersedes them rather than being read alongside them. The section "Which entries carry a source" says the guard's subtests come from `Complete`, and the section "The guard" says the same and adds that the guard "fails outright when it finds no translated entry to check, so a change that quietly empties `Complete` cannot leave a green test asserting nothing." Neither sentence describes the code any longer.

`TestATranslationTracksItsEnglishSource` now runs one subtest per shipped catalog, taken from `msg.Tags()`, and skips the base catalog and every entry marked `"skeleton": true`. The exemption is unchanged and still reads the flag on each entry rather than a list of language tags. What changed is the selector for the outer loop.

dinah-287 forced the change and it is worth recording why, because the alarm this document designed did fire and was then rewired. That card renamed the board's vocabulary, which moved the English text of ninety-five entries, and its D-6 took German and Hindi off `Complete` rather than shipping stale translations or deleting the keys. `Complete` fell to `[Base]` alone, the roster-keyed loop found no catalog to iterate, and the `checked == 0` fatal reported that the guard was asserting nothing. That is the alarm working. Rewiring the loop to read every catalog is the response this document had ruled out by name, and it went in anyway because the alternative was leaving 1072 live translations unchecked for the length of the follow-up card.

The population is smaller than it was, and the card got the size wrong twice before these figures were counted. Its first note claimed a strictly larger set with no count behind it. Its second note gave 1288 for the before figure, which is what you get by counting the before on the card's own branch, where English has already gained the card's thirteen new keys, so it names a population no tree ever held.

The counts below name the ref each was taken on. On `origin/main`, every catalog carried 631 entries and none of them was a skeleton, and the roster was `[Base, hi, de]`, so the loop covered 631 entries of German and 631 of Hindi, which is 1262. On dinah-287's branch, English carries 647 keys and German and Hindi carry 647 apiece of which 111 are skeletons, entries the rename left holding renamed English, so the loop covers 536 apiece and 1072 together. The five skeleton catalogs that joined the loop are skeletons throughout and contribute no entry at all. The guard lost 190 entries of coverage and gained none, and a translator taking German or Hindi back to `Complete` faces 111 entries per language rather than the 108 an earlier note gave, the extra three being the card's own preview keys.

What the guard can no longer see is a roster emptied by accident. `checked == 0` now means every shipped catalog is a skeleton throughout, which is a different and much rarer condition than the one the fatal was written for. Anybody removing a language from `Complete` gets no failing test out of it, so the removal has to be a ruling somebody makes rather than a consequence somebody notices. dinah-287 puts its own removal of German and Hindi in front of the operator on that basis.

Restoring the alarm needs a check over the roster itself rather than over the catalogs, of the shape "no language leaves `Complete` without a card recording why," and this document does not specify one. Whoever writes it should read this amendment first, because the failure mode it is guarding against has now happened once.


## Amendment: the figures are not written down any more, and what is checked for a language off the roster

Every count in the sections above is retired, including the ones the previous amendment gave as corrections. Read no number out of this document. 647 keys, 111 skeletons, 1262 before, 1072 after and 190 lost were each true of one tree on one day, and each of the three review rounds that corrected them left one of them wrong somewhere else. The last round found the branch's own quick-start transcript and a comment forty files away disagreeing inside a single commit.

The counts have one home and it computes them. Run `dinah version --catalogs`, which prints every shipped catalog with its translated count over its total, read off the catalogs in the binary you just built. Three things fall out of that table and nothing else needs recording:

- The population `TestATranslationTracksItsEnglishSource` covers is the sum of the translated column over every catalog but English.
- The entries a retranslation of a language would face is the difference between that language's two columns.
- A language whose translated column reads zero is a skeleton throughout.

Cite the commit you ran it on whenever you quote the output, and quote the output rather than a number lifted out of it.

## What is checked for a language that is not on the completed list

dinah-287 took German and Hindi off `Complete`, and the round-3 review found that the removal had silently taken their contents out of every check. `TestEveryDeclaredLanguageShips` asserts a translated count only for a language on `Complete` and a zero count only for one on `Skeleton`, so a language on neither roster was asserted by nothing at all: replacing all 536 German translations with the English text left the package green on the branch and red on the trunk.

Completeness and correctness are different properties, and the guards now keep them apart. A language off `Complete` is not expected to be complete. It is still expected that what it does carry is genuinely translated, and that an entry holding English says so. Two guards in `internal/msg/msg_test.go` assert that, both keyed on the entry rather than on the roster its catalog is on, so neither can be emptied by a roster change:

- `TestATranslationIsNotEnglishUnderAnotherTag`. An entry that is not marked as a skeleton must differ from its English source, unless it carries `"verbatim": true`. It also fails an entry marked verbatim whose text no longer matches English, which keeps the flag from outliving the claim it makes.
- `TestASkeletonEntryReallyCarriesTheEnglishText`. An entry marked as a skeleton must carry the English text, and must not also be marked verbatim. Without this arm a catalog could answer the first guard by marking everything a skeleton.

`Entry.Verbatim` is new and it is how a translator says the answer really is the English word, letter for letter, because the language uses the same word. A table heading reading "Name" in German is the ordinary case, and 21 German entries and 8 Hindi entries carry the flag today. Nothing outside the guards reads it: to a reader a verbatim entry is an ordinary translation, and `Coverage` counts it as one.

The second guard found real drift the moment it was written. Three entries in each of the five generated catalogs still carried an older English sentence, because the English at those keys had moved and nothing regenerates a skeleton. They were refreshed in the same commit.

What no guard here does is notice a language leaving `Complete`. That is still a ruling somebody makes rather than a consequence somebody notices, and the check for it, of the shape "no language leaves `Complete` without a card recording why", is still unwritten.


## Amendment: the removal was overruled, and both languages came through translated

Two sections above describe German and Hindi leaving `Complete`, and one of them opens by saying dinah-287 took them off it. That did not end up happening, and this section supersedes both on the point of fact while leaving everything they say about the guards standing.

The operator ruled on 2026-08-27, in his words: "I am not going to ship something with incomplete translations, so fix them first." So D-6 is superseded, the entries the rename left holding renamed English were translated on dinah-287 itself, and both tags stayed on the roster. Read the sections above as an account of a branch that existed for a few days rather than as an account of the tree.

Nothing about the guard work changes. The round-3 finding that a language leaving the roster used to leave every check was real, and the two entry-keyed guards that closed it are still there and still keyed on the entry. They now protect two languages that remain on the roster rather than two that left it, which is the safer of the two shapes. `TestATranslationTracksItsEnglishSource` still iterates `Tags()` rather than `Complete`, for the reason the first amendment gives.

The figures stay where the previous amendment put them, which is in the tool. On dinah-287's branch at commit 8440d2b, before the fill, `dinah version --catalogs` read 536 of 655 for German and 536 of 655 for Hindi, so each language faced 119 entries. After the fill both read 655 of 655. Quote the tool at a named commit, as that amendment says, rather than lifting either number out of this paragraph.

### How those translations were produced

Recorded here because translation quality is the operator's to judge, and a reader deciding whether to trust an entry should be able to find out where it came from without reading a card.

Seventy of the 119 keys per language existed before the rename under the same key, and a further two dozen existed under the retired spelling of the same name. For every one of those, the rendering was produced by taking the trunk's own German or Hindi at that key and applying the noun substitution the English rename applied, then re-reading the sentence for agreement. That is why the register matches the rest of the catalog: it is the rest of the catalog, moved one noun.

The remaining two dozen keys are new with the rename, being the vocabulary migration's own reporting, the two vocabulary refusals and the two check findings beside them. Those were translated from the English against the register of the neighbouring entries, and they are the ones a fluent reader should look at first.

The vocabulary choice is the substantive judgement and it was forced rather than invented. English moved `state` down a level, from naming the board station to naming the card's condition, and both languages already had a word for the station: German's `Zustand` and Hindi's `स्थिति`. The `column.*.state` table headings kept their English text through the rename, so those two words were already sitting on the condition and could not be moved. The station therefore needed a new word in each language, and it got `Spalte` in German and `स्तंभ` in Hindi.

Three German check sentences are terser than a literal rendering, because the refusal table on `dinah help <command>` allots its middle column 44 display columns and a longer sentence wraps out of its row. `TestEveryRowStartsItsColumnsAtOneDisplayColumn` catches that, and caught it here.

### What is uncertain and was left uncertain

`स्तंभ` for the board column is the item to check first. It is the standard Hindi word for a column and it fits a catalog that translates rather than transliterates its concepts, but Hindi computing prose often transliterates this particular word as `कॉलम`, and which one a Hindi-reading operator expects is a question about readers rather than about Hindi.

The Hindi catalog carries three words for "workbench" already: `वर्कबेंच`, `कार्यक्षेत्र` and `कार्यपीठ`. This batch uses `वर्कबेंच` throughout, which is the dominant form, so the entries it rewrote are now consistent with each other and with the majority. The drift in the entries it did not touch is untouched and belongs to dinah-252.

Four German entries carry `verbatim`, and two Hindi ones. Both card lines are placeholders and separators with no word in them, and the two German splices reading ", in {path}" read in German as they read in English, which is what `refusal.malformed.at` already claimed for the same string.


## Amendment: what an author records when a rename refills a whole catalog

The per-key decision record has a size above which it stops recording anything, and dinah-287 reached it. That card filled 119 skeleton entries in each of two catalogs, and the convention as written asks for 238 checklist items. Agent Code Review ruled on 2026-08-27 that the collective record the card carried satisfies the convention, and this amendment states the rule so the next card of the same shape does not have to argue it again.

The section "The semantic layer, which is not a test" already bounds itself. It says the bound is "the locale lines that one card's own diff changes, which is a handful to a few dozen entries", and it says in the same breath that nobody should read the layer as licence to run it over a whole catalog in one pass. A card filling a few hundred entries is therefore outside what that section governs, rather than being a large instance of it.

The reason the bound exists is the shape of the judgement being recorded. A per-key record captures whether an existing translation still says what its English now says, which is a decision somebody makes one key at a time. A skeleton entry carries no translation, so it carries no such decision, and a card that fills skeletons answers one question the same way several hundred times. Writing that answer down several hundred times buries the entries that do carry a judgement.

So the rule has two branches, and an author takes the one that matches the diff.

- A diff changing existing translations records one decision-kind item per changed key, exactly as the earlier section says. This is the ordinary case and nothing about it changes.
- A diff filling skeleton entries, or otherwise carrying a block of entries across a mechanical change to the English, records one collective decision-kind item instead. That item states how the renderings were produced, which entries came from an existing translation and which were written fresh, what vocabulary choices were made and why, and every term the author is unsure of. Each uncertain term is additionally filed as its own open question on the card, owned by the operator, so it travels to the operator station rather than resting in a note.

The second branch is not a lighter obligation. It moves the writing from a per-key form that would say nothing into a provenance account a reader can actually check, and it requires the uncertain terms to be named individually rather than buried in the account.

Agent Code Review reads the branches the same way. A collective record on a diff that changed existing translations is still a [major]; a per-key demand on a diff that filled skeletons is a misreading of this document.

## Amendment: the verbatim counts in the section above are stale, and should not have been written

The section "What is checked for a language that is not on the completed list" says "21 German entries and 8 Hindi entries carry the flag today". Read at dinah-287's head 15119d5 the catalogs carry 25 German and 10 Hindi, because that card's own fill added four German entries and two Hindi ones. The section describing that fill then writes the delta down as "Four German entries carry `verbatim`, and two Hindi ones", which reads as a total and is not one.

Both sentences are the thing the amendment two sections above them retires by name. Read no count out of this document, including these. The verbatim flag is a field on each entry in `internal/msg/locales/<tag>.json`, so the current figure is whatever counting that field returns on the commit you care about, and a sentence here cannot stay right across a card that touches the catalogs.

Neither sentence is load-bearing. Both were written as illustrations of how ordinary the flag is, and the illustration survives without a number: a table heading reading "Name" in German is the ordinary case, and a handful of entries per translated catalog carry the flag for that reason.

## Who checks a translation, ruled 2026-08-31

The operator ruled that a mechanically edited translation ships without a fluent reader having seen it, and that this is the project's standing practice rather than a lapse to be flagged each time. Record it here so nobody files the question again.

**What that means when you touch a catalog.** German and Hindi carry real translations; the other five non-English catalogs carry the English text verbatim as skeletons. When the English behind a real translation changes, edit the German and the Hindi in their own language, following the vocabulary the neighbouring entries in that same catalog already establish, and recompute the staleness fingerprint. Then ship it. Do not file an open question asking whether the wording reads naturally, and do not ask a reviewer to confirm fluency. Nobody on this project can answer either, so a question of that shape cannot be closed and only parks the card.

**What you must still do.** Say in the decision record what you changed and what you based it on, exactly as this document already requires. A mechanical edit that moves one noun is a different act from rewriting a clause, and the record should make clear which one you performed. Where a change is large enough that a mechanical edit will not carry it, say so plainly rather than attempting a fluent rewrite and presenting it as one.

**What this ruling does not license.** It does not license pasting English into a catalog that carries a real translation, which remains wrong and which a guard already catches. It does not license leaving a translation stale when its English has moved. And it does not make the result verified. The honest description of a translated string on this project is that an agent edited it and no fluent reader has read it, which is a known and accepted cost rather than a claim of correctness. Do not write a note that says a translation was verified, because nothing verified it.

**Why the operator ruled this way.** The alternatives were to find a fluent reviewer before beta, or to demote both catalogs to English skeletons until one exists. He chose to ship, which keeps the two real translations in front of the first customers most likely to want them and accepts that some wording may be wrong. Whoever revisits this should know that the trade was made deliberately with the risk named, rather than by nobody noticing.