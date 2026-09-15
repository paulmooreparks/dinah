# Convention counterexamples 3

This is part 3 of 3, and it is the part new entries are appended to. The purpose of this collection and the convention for writing an entry are stated at the top of "Convention counterexamples 1", which holds the oldest entries. Part 2 was closed at 203,684 bytes under its own rotation rule.

Before you append an entry here, read this document's current byte count. When the new entry would carry this part past 200,000 bytes, create "Convention counterexamples 4" instead and append the entry there, so that no part approaches the 262,144-byte ceiling at which the listing route stops returning a body.

## A reused refusal audited over the list the design enumerated, rather than over the messages the caller reads

Caught at Agent Code Review on dinah-409, 2026-09-07, and it is the second catch of this class on that one card. The first catch was the design review's, which found that reusing `contract.NotHolder` for a new verb made one catalog context comment stale. This is the same reuse making a catalog's printed sentence wrong, which is the form that reaches a person.

A new verb that reuses an existing refusal name inherits that name's English. The English was written for whichever verb raised it first, and it is free to name that verb, to describe that verb's act, and to end in a next step that runs that verb. None of that is visible from the constant, from the shape entry, or from a criterion that pins the refusal's name.

The audit that misses it is the audit that walks the list the design wrote down. dinah-409 reused four names. The spec examined three of them in one paragraph, quoting each sentence and observing that none named a verb, and it settled the fourth in a different paragraph on a different ground: that block already holds callers to the same bar for an empty reason. That ground is about the rule, not about the sentence, so the sentence was never read.

**Wrong.** `internal/verb/raise.go` refuses an empty reason with `contract.NoReason`, and the design records the reuse as settled:

    `contract.NoReason` (already `block`'s refusal for an empty reason,
    core-profile, no `LayerPrefix`) is reused for a raise with no reason,
    rather than minting a Dinah-layer twin of a refusal the core profile
    already declares for exactly this fact.

`dinah raise sample-4 apex` with nothing typed after it then prints:

    no-reason a block carries a reason, posed so the operator can answer it;
    run `dinah block sample-4 "the question the operator has to answer"`

The sentence names an act the caller did not perform, and the next step is a runnable command that performs a third act: it blocks the card instead of raising the tier and handing it back. A reference to a destination that does not exist is caught by grep. This is a reference to a destination that does exist and is the wrong one, so nothing mechanical sees it.

The criterion could not see it either. dinah-409 AC-5's failing case reads "the refusal name is anything other than dinah's existing no-reason", so a test satisfying the criterion asserts the name and never reads the sentence.

**Right.** Audit a reused refusal by reading what it prints, verb by verb, on the surface the caller meets, and treat the next step as part of the message rather than as decoration. The same card's `not-holder` reuse is the worked example of the audit done properly: the design read the printed sentence, judged that "nobody holds this card, so there is nothing to release" still reads correctly for the new verb because releasing is still how the card is freed, and separately corrected the context comment that had claimed release was the only caller. Where the sentence does not survive that reading, the catalog already carries the mechanism: `not-holder` ships `.unnamed` and `.next-unheld` variants beside its base text, so a per-verb variant is an ordinary act rather than a new pattern.

**How to catch it.** For every refusal name a new verb reuses, run the verb into that refusal and read the output, rather than reading the constant's doc comment or the shape entry. Two questions decide it: does the sentence describe the act the caller performed, and does the next step, if the caller runs it verbatim, do what the caller was trying to do. A next step that runs a different verb fails the second question even when the first passes. Reading the catalog entries is not a substitute for running the verb, because an entry reads as generic prose until a caller arrives under a verb it was not written for.


## A shell failure mode asserted in published guidance, on a card that measured everything else

Caught at Agent Code Review on dinah-380, 2026-09-07. The card's whole discipline is that it measures before it advises, and two earlier versions of it were closed for advising from an argument. Every token figure in the shipped passage is transcribed from a run. The one sentence in that passage describing what happens when the command goes wrong was written from reasoning about the command's shape, and it is wrong.

A published command carries two kinds of claim. The cost claims get measured because a harness exists to measure them. The behaviour claims, which say what the command does when an input is bad, get reasoned about, because running them looks like it needs no instrument. It needs one shell and thirty seconds, and the reasoning is what fails.

**Wrong.** `internal/guide/guides/mcp.md` publishes this command and then describes its failure mode:

    for ref in <ref> <ref> <ref>; do printf '=== %s\n' "$ref"; cat "$(dinah path "$ref")"; done

    On a reference the workbench does not know, the refusal goes to standard
    error, standard output stays empty, and `cat` fails on an empty name, so a
    mistyped reference stops the read rather than quietly returning some other
    card.

Run with a good reference, a bad one, and a good one, the loop prints all three delimiter lines, prints both good bodies, writes two lines to standard error for the bad one, and exits 0. A `for` loop does not stop when a command inside it fails, and the loop's exit status is the status of its last iteration. So the read does not stop, and a caller reading only the exit status sees success while holding one card fewer than it named.

The safe half of the claim does hold, and it is the half that matters most: the substitution yields an empty string rather than a stale path, so the command never returns a card the caller did not name. That is why the sentence reads as plausible. One true clause carried a false one past review.

**Right.** State the failure mode you ran, and state the exit status, because the exit status is what a caller branches on. The honest form of the same sentence says the bad reference contributes no body, that its delimiter line still marks its place in the output, that the error appears on standard error, and that the command's exit status does not report the failure, so a caller wanting the read to be all-or-nothing has to check the output rather than the status.

The same card's harness shows the standard being met elsewhere. It runs the published command through a real POSIX shell against its own fixture and refuses the run unless the shell's output equals the text it counted, which is a behaviour claim turned into a check. That guard covers what the command prints on the happy path and nothing about what it does on a bad reference, so the gap sat directly beside the mechanism that would have closed it.

**How to catch it.** For any command a guide publishes, run it three ways before the passage ships: with every input good, with one input bad, and with the input that triggers whatever the passage promises about failure. Then read the exit status, not just the output. Where the passage uses a verb like "stops", "refuses", "fails", or "aborts", that verb is a testable assertion about control flow and it needs the run that demonstrates it. A loop, a pipeline, and a command substitution each swallow failure in a different place, so a claim about one of them cannot be carried over from another.


## A fixture chosen for looking unusual, in characters the property under test passes through unchanged

Caught by the implementer on dinah-270, 2026-09-07, and reproduced at Agent Code Review on the same card. This is the sibling of "A test input whose distinguishing shape is erased by the constructor that builds it" in part 1. There a normalising helper removed the difference on the way in. Here nothing removes it, because the difference was never a difference to the code under test: the fixture is unusual on one axis and ordinary on the axis being asserted.

An acceptance criterion is the usual source of the mistake, since a criterion names a fixture in the vocabulary of the domain the card is about rather than in the vocabulary of the mechanism the test exercises. A reader then trusts the criterion, writes the fixture it asks for, and gets a case that cannot fail.

**Wrong.** dinah-270 AC-2 asked for a URI round trip proving that `servedTextUriParts` percent-encodes what it puts in a query string, and it named the fixture: "a Windows root containing a colon and a space". A colon, a backslash and a space all survive an unencoded query string untouched, so the composition

    query: `root=${root}&ref=${ref}`

round-trips `C:\Users\paul\My Benches\dinah` perfectly. Planting exactly that break and running the suite with only the criterion's own fixtures leaves it green, 396 of 396 passing. The case looks like the hardest input on the page, and it exercises nothing the ordinary POSIX fixture beside it does not.

**Right.** Choose the fixture from the grammar the code has to defend against, not from the domain the card is about. The characters that matter to a query string are the ones that mean something inside one: an ampersand opens a further parameter, an equals sign moves the boundary between a name and its value, and a plus sign decodes back as a space. A root carrying all three,

    C:\Work\Bench & Vise\ref=1\a+b

turns the same plant red and names itself in the failure message. Keep the criterion's own fixture as well, because a Windows path is still worth carrying, and say in the test's comment which fixture arms it and which fixture does not, so the next reader does not delete the one doing the work.

**How to catch it.** Take each fixture in turn, break the behaviour the test guards, and ask which fixtures go red. A fixture that stays green under the break is decoration, whatever its name promises. Where a criterion prescribes the fixture, run that check against the criterion's fixture specifically, because a criterion is written before the mechanism exists and can name an input that the finished mechanism treats as ordinary. The finding then belongs to the criterion as much as to the test, and correcting the criterion is part of the fix.


## An aggregate that reads "not there" as "nothing to report"

Caught three times on dinah-419, 2026-09-07: twice at Agent Code Review and once by the implementer while making the answer total. The first two catches were the same defect one path over, which is what earns this an entry rather than a note on the card.

A summariser walks a collection of per-place reports and decides one answer for all of them. Each report carries an optional field standing for "we have not heard from this place", so the summariser reads that field and knows when to say it cannot tell. The trap is that the collection has a second way of expressing the same fact, and the summariser cannot see it: a place that could not be read may contribute no report at all. The two encodings look nothing alike in the source, and the one nobody wrote down is the one a failure produces, because whatever deletes the answer usually deletes the carrier with it.

The result is the reassuring reading of a failure. An empty collection and a collection of confirmed-empty reports are the same value, so the summariser answers "nothing" where the truth is "no idea".

**Wrong.** The VS Code status bar reports the cards a reader is holding. Each workbench reported `{ title, holding, fetchedAt?: number }`, with an absent `fetchedAt` meaning nobody had heard from it, and the summariser set its doubt flag from those entries:

    for (const entry of snapshot) {
        if (entry.fetchedAt === undefined || now - entry.fetchedAt > staleAfter) {
            stale = true;
            continue;
        }
        ...
    }
    return { cards, uncertain: stale && cards.length === 0 };

A folder whose root-scoped walk failed produced no rows, so it contributed no entry, so `stale` stayed false and the bar rendered its ordinary idle text at a reader holding a card. Round one repaired the walk to keep the rows a previous good walk had left. Round two found that a folder whose walk had NEVER answered still had no rows to keep, so the same lie survived on the same function's first checkpoint, and it repeated on every later failure. Round three, having made the answer total, found a third route neither review had named: a helper substitutes a dead-end row into a folder whose walk found nothing, so the folder was no longer rowless and that row read as dinah confirming the folder was empty.

**Right.** Make the report a closed union whose arms are the answers, and make the producer total over the things it reports about:

    export type WorkbenchHoldingReport =
        | { state: "answered"; source: string; title: string;
            holding: readonly CardView[]; fetchedAt: number }
        | { state: "unheard"; source: string }
        | { state: "vacant"; source: string };

Every row produces one, and a folder holding no rows produces one for itself, so no failure can remove a place from the collection. The summariser switches over the union and hands its default arm to a `never` parameter, which means an arm added later stops the build instead of falling through to the reassuring branch. `fetchedAt` is required on the arm that has one, so the field can no longer double as the flag.

Totality is what makes the fix a fix rather than a third patch. The first two rounds each closed the route that had been reproduced, and each left the type able to say the wrong thing somewhere else.

**How to catch it.** Where a summary over a collection has a state meaning "cannot tell", ask what happens when an element is missing rather than when its field is absent, and go and find every producer that can drop one. Then ask whether the type can express the difference at all: an optional field carrying a state, in a collection whose elements can also vanish, cannot, because absence is now spelled two ways and only one of them is read. The repair belongs at the type and at the producer, not at the reader, and a reader-side repair is recognisable because it names one route.

Prove it by driving the failure two ways rather than by asserting on the code. On this card the review put the same failure to the window through a folder and through a single workbench and got two different answers, and the fix is finished when the two answers are one answer. Keep a control in the same test: something that answers and legitimately reports nothing, so a summariser that has simply been made to warn at everything fails too.


### Refinement, after the same class was caught twice more: a total answer whose arms are told apart by name alone

Caught at Agent Code Review on dinah-419, round three, 2026-09-07, and repaired in round four. The entry above stands, and this is what it did not say. Making the aggregate total moves the defect out of omission and into misclassification, so a reader who follows the catch method above (find every producer that can drop an element) finds nothing, because after the repair no producer drops anything.

The union in the entry above has three arms and only one of them is obliged to carry evidence. `answered` requires a `fetchedAt`, so nobody can construct an answer they did not receive. `unheard` and `vacant` are structurally identical, both being `{ state, source }`, so the type obliges a producer to pick one and gives it no way to fail. A producer holding a failure can still pick the reassuring one, and two did.

**Wrong.** A workspace folder the extension could not launch dinah in fell through a resolution branch that recorded the folder as having been answered for, and its report came out `vacant`. The comment on that branch read "Dinah refused, and the refusal is itself the answer", which was true for one of the refusals reaching it and false for the four transport kinds, since the code that turns an outcome into a resolution flattens `spawn-failed`, `unreachable`, `stale` and `not-json` into the same string a real refusal envelope uses. Separately, a folder that answered "nothing here" once was `vacant` for the life of the window, so a folder that went silent two and a half hours earlier was still drawn as a confident empty hand where an `answered` report in the same position would have gone stale and warned.

**Right.** Every reassuring arm carries the moment it was learned, not just the one that carries data:

    | { state: "vacant"; source: string; answeredAt: number }

A failure has no moment to record, so it cannot construct a vacancy and has to name the doubt. The summariser then ages a vacancy on the same rule it ages a hand, so no conclusion outlives the observation behind it. Where the producers were told apart by a boolean saying whether an answer had arrived, the boolean becomes the stamp itself, and a single predicate decides what earns one rather than the same condition being spelled at each producer.

Two consequences are worth expecting. The distinction the type needs may have been flattened away upstream, so recovering it is part of the repair: here the resolution type gained a field saying whether the tool itself produced the refusal or the window never reached it. And a place that is never re-read cannot hold a stamp that stays fresh, so anything the fix makes expire needs somebody to renew it; the folder kind this provider had never re-read is now re-resolved on each checkpoint, and the answer either renews the stamp or lets it lapse into a doubt.

**How to catch it.** Once a union has been made total, ask a second question of it: for each arm, what does a producer have to hold in order to name it. List the arms a summariser treats as reassuring, and check that each one requires something a failing producer cannot supply. Structural identity between a reassuring arm and the doubt arm is the tell, because it means the choice between them rests on a producer's judgement and nothing checks that judgement. An exhaustiveness gate does not help here: it refuses a fourth arm nobody handles, and it cannot refuse a third arm chosen without grounds.

Prove it by enumeration rather than by instance. This defect survived three rounds by appearing one path over each time, so the test that finished it drives every way the window can fail to learn about a place (a read that refuses, a walk that never came back, a walk that came back once and stopped, a vacancy nobody renewed, each way a resolution can fail without the tool having answered, a place never opened, a spawn that threw, and an answer that simply aged) and makes one assertion over all of them. Keep the controls beside it, since a window made to warn at everything passes any enumeration of failures.



### Second refinement: an arm earned by excluding the bad names rather than by admitting the good ones

Caught at Agent Code Review on dinah-419, round four, 2026-09-07. The entry above and its first refinement both stand. This is the third shape the same class has taken on one card, and it is what a reader gets after doing everything the two passages above prescribe.

The first refinement's remedy is that every reassuring arm must carry evidence a failing producer cannot supply. That remedy is satisfied by a stamp, and a stamp is written by a predicate. The defect moves into the predicate. Where the predicate decides membership by naming the cases that do not qualify, it admits every case nobody thought of, and the cases nobody thought of are the ones a vocabulary grows.

**Wrong.** One predicate decided which resolutions earn a vacancy, and its own doc comment states the rule correctly: "the refusal has to be one that means emptiness". The code underneath states the opposite rule.

    function vacancyAnsweredBy(resolution: WorkbenchResolution): boolean {
        if (resolution.state !== "refused" || !resolution.answered) {
            return false;
        }
        return (
            resolution.refusal !== NO_WORKBENCH_FOUND &&
            resolution.refusal !== AMBIGUOUS_WORKBENCH
        );
    }

Two refusal names are excluded and every other name the tool can produce is admitted. The tool publishes over a hundred refusal names, and several of the ones reaching this call site say the opposite of emptiness. `dinah.unreadable-workbench` means a workbench file was found and could not be opened. `dinah.damaged-workbench` means a workbench sits at the one address the containment rule gives it and its anchor will not parse. `dinah.needs-container-migration` means a workbench exists in the layout the format used to have. Each of those is a place that may hold the reader's card, and each was reported to the status bar as a confirmed empty hand with a fresh stamp on it.

Driven at the extension and confirmed at the tool. Loading one folder resolved through the real `parseRefusal` from a `dinah.unreadable-workbench` envelope gives `[{"state":"vacant","source":"C:\\ws\\broken","answeredAt":1000}]`, and a second folder answering normally beside it composes to `{"cards":[],"uncertain":false}` with the bar reading `$(checklist) Trees` and no warning glyph. On the tool's side `cmd/dinah/checkscope_test.go` already runs `status` in a workbench with a hand-damaged anchor and asserts it refuses `dinah.damaged-workbench`, which is the same command and the same working directory the extension uses to resolve a folder.

**Right.** Name the answers that mean the thing you are about to assert, and let everything else fall to the doubt.

    const VACANCY_REFUSALS = new Set([NO_CONFIGURED_WORKBENCH, NO_WORKBENCH]);

    function vacancyAnsweredBy(resolution: WorkbenchResolution): boolean {
        return (
            resolution.state === "refused" &&
            resolution.answered &&
            VACANCY_REFUSALS.has(resolution.refusal)
        );
    }

The set is short because the population of answers meaning "nothing is here" is short, while the population of answers meaning something else is open and grows with every refusal a later card mints. A new refusal then arrives outside the set and produces a doubt, which is the safe direction, where under the exclusion form it arrives inside the set and produces a false reassurance.

**How to catch it.** Wherever a predicate gates a reassuring conclusion on a value drawn from an open vocabulary, read which way round it is written. An exclusion list is correct only where the vocabulary is closed and the code owns it. A refusal catalogue, an error code set, a status string and a kind field are all open, because another card adds to them without visiting this predicate. The tell is a comment stating the rule as an inclusion while the expression underneath is a chain of inequalities, and that combination is worth a grep of its own.

The deeper lesson belongs to the whole entry rather than to this refinement. Three repairs on one card each closed the route that had been shown and each left the rule stated in a place the next failure walked around: first a reader, then a type, then a predicate. Ask at each repair where the assertion is finally earned, and put the requirement there rather than at the last place the defect was seen.




### Third refinement: an admitted name whose raise sites hold two conditions, only one of which is the thing

Caught at Agent Code Review on dinah-419, round five, 2026-09-07. The refinement above is what a reader gets after switching a predicate from an exclusion list to an inclusion list, and its "Right" block is a snippet somebody will copy, so it needs the qualification this entry carries.

An inclusion list is written in the vocabulary of names. The thing it is really asserting is a condition. A name is a good proxy for a condition only while the name has one raise site, and a name minted for a sentence rather than for a fact can grow a second raise site that means something else without ever changing its spelling. The reviewer who checks the list against a doc comment, a review comment or a constant's name cannot see that. Only a reader who opens each raise site can.

**Wrong.** dinah-419's repaired predicate admits two refusal names.

    const VACANCY_REFUSALS: readonly string[] = [
        NO_WORKBENCH,
        NO_CONFIGURED_WORKBENCH,
    ];

`dinah.no-configured-workbench` has exactly one raise site and means what it says. `dinah.no-workbench` has two arms inside a single branch of `internal/bench/bench.go`'s `DiscoverSource`, and the arms disagree about the fact the predicate is asserting:

    if !Exists(filepath.Join(abs, WorkbenchAnchor)) {
        if beneath, ok := SoleBeneath(abs); ok {
            return ..., contract.RefuseWith(contract.NoWorkbench, abs, map[string]string{"found": beneath})
        }
        return ..., contract.Refuse(contract.NoWorkbench, abs)
    }

The second arm is a vacancy. The first arm has just located a workbench in the named path's own `.dinah` container and put its path in the refusal, so it is the very thing the predicate exists to doubt, and it arrives wearing the name the predicate admits. A VS Code user who pins `dinah.workbench` at a project root whose workbench lives at `<root>/.dinah/<id>` reaches it, and the status bar answers `state: "vacant"`, `uncertain: false`, and no warning glyph.

**Right.** Check each admitted name by opening its raise sites and reading the condition each one refuses under, then admit the condition rather than the name wherever the two come apart. Where the distinguishing evidence rides the refusal's `Extra` map, as `found` does here, carry it across the boundary instead of dropping it: the extension's own `CliOutcome` already declares `context?: Record<string, string>` on its refused arm, and `parseRefusal` discards it while keeping `detail` and `candidates`, so the value the predicate needs is thrown away one function before the predicate runs.

**How to catch it.** For every name on an inclusion list gating a reassuring conclusion, grep the producing codebase for that constant and read each raise site, counting them. One site is a name that can stand for a condition. Two or more sites is a name that has to be read, and the question to ask at each is whether that site's condition is the one the list is asserting. A refusal built with a builder that takes extra values, rather than with the plain one, is the loudest tell available, because the extra values exist precisely to say something the name does not.

The lesson the whole entry has been circling is worth stating once more in this form. Each round moved the assertion closer to where it is earned, from a reader to a type to a predicate to the predicate's own vocabulary, and each round's check was run against the artifact the previous round produced rather than against the source of truth. The read that ends it is the one that leaves the language the predicate is written in and opens the code that mints the values it tests.




### Fourth refinement: an invariant earned by the order of two calls at one call site

Caught at Agent Code Review on dinah-421, 2026-09-07. The entry above and its three refinements all stand, and each of them moved the assertion closer to where it is earned: from a reader, to a type, to a predicate, to the predicate's own vocabulary. This is the same journey run on a second module, and it stops one step short of the end.

A module holds per-place state and offers two calls. One call writes the placeholder that says a place has not been heard from. The other runs the real read and replaces whatever the placeholder left. Neither call is wrong. The invariant lives in the fact that a caller makes the first before the second, so the invariant is a property of the call sites rather than a property of the module, and a second call site that reaches only the reading half reopens the reassuring reading the first call site closed.

The tell is a module whose reassuring output is an absence. Where "nothing here" is spelled as an empty collection, an unwritten entry and a confirmed-clean answer are the same value, so the only thing separating them is whether some earlier call happened to run.

**Wrong.** `editors/vscode/src/diagnostics.ts` guards an empty Problems panel with a placeholder written by `markPending`, and `extension.ts` calls the two in the right order at activation:

    for (const report of provider.holdingSnapshot()) {
        if (report.state !== "answered") {
            continue;
        }
        await diagnostics.markPending(report.source, report.title);
        void diagnostics.runFor(report.source, report.title);
    }

The `continue` is the hole. A workbench that is not `answered` at activation, which is every candidate of a folder holding several workbenches until a reader opens one, is skipped here and never receives a placeholder. The checkpoint callback and the manual command both call `runFor` and `applyResult` without `markPending`, so when such a workbench later becomes `answered` it is read for the first time from a site that writes no placeholder. Driven directly: `applyResult` with a refused outcome on a root that never saw `markPending` makes zero calls to the panel and leaves it empty, which is byte-identical to what a confirmed-clean workbench produces.

A unit test pinned the count of `markPending` call sites at one, so the guard against a third reading site also refuses the second placeholder site that would close the hole.

**Right.** Put the requirement in the module that owns the state, so no call site can decline it. The reading path already knows whether the place has ever been confirmed, and that is the only fact the placeholder needs:

    if (outcome.kind !== "ok") {
        this.deps.log(...);
        if (!state.confirmedOnce) {
            await this.markPending(root, title);
        }
        return;
    }

`markPending` is already a no-op once `confirmedOnce` is set, so the call is safe from every site and the ordering at any one site stops being load-bearing. A caller that reaches only the reading half now gets the placeholder anyway.

**How to catch it.** Where a module's reassuring answer is an absence rather than a value, list every entry point that can produce that absence and ask which of them writes the marker that distinguishes it from ignorance. One entry point writing the marker and two producing the absence is the defect, and it survives review because the site everyone reads is the one that does it correctly. The check is a grep of the module's own public calls against the call sites in the wiring, counting each side, rather than a read of the wiring's happy path.

A test pinning the number of call sites is worth reading in the same pass. Such a test is written to stop an unnoticed third caller, and it cannot tell that apart from a second caller somebody added on purpose to close a hole, so it will report the fix as a violation. That is not a reason to drop the test. It is a reason to state in its comment which of the two counts it is really defending, so the next reader changes the number deliberately instead of routing around it.


## A translated string that renders identically and does not match, because the script has two spellings of one letter

Caught by the implementer on dinah-423, 2026-09-08, before the strings shipped. Every guard over these catalogues compares bytes, and a human reader comparing the same two strings on screen sees one string. Devanagari, Vietnamese, Korean and any script with combining marks can spell one letter two ways: precomposed as a single code point, or decomposed as a base letter followed by a combining mark. Nothing about the rendered glyph says which one is stored.

The catalogue then holds two spellings of one word, and no test can see it. The parity guard asks whether a key exists. The honesty guard asks whether a translation differs from the English, which it does under either spelling. The staleness guard fingerprints the English rather than the translation. So the drift lands inside the language nobody on the project reads, and it surfaces later as a search that finds one entry out of two.

**Wrong.** Adding Hindi for a new key, the word for folder was written with the precomposed letter U+095E:

    "इस फ़ोल्डर से ..."

The rest of `package.nls.hi.json` and `src/locales/hi.json` spell the same sound as a base letter plus a combining nukta, U+092B U+093C, thirteen times against one. The two render as the same word. A reader searching the catalogue for the existing spelling finds every entry but the new one.

**Right.** Normalise a new entry to the spelling the catalogue already uses, and prove the choice by counting rather than by eye:

    python -c "import io;s=io.open('src/locales/hi.json',encoding='utf-8').read();
    print(s.count('फ़'), s.count('फ़'))"

Thirteen against one settles which spelling is the convention here, and `unicodedata.normalize('NFD', text)` moves the new entry onto it. The count is the evidence; the decision is which form the file already carries, not which form is canonical in the abstract.

**How to catch it.** Any diff adding text in a script with combining marks gets one mechanical check before it is committed: count the precomposed code points of that script in the file, and count the decomposed sequences, and read whether the new entry joined the majority. For Devanagari the precomposed range is U+0958 to U+095F. The check costs one command and it is the only thing standing between a byte comparison and a reader's eye, because those two agree on every other kind of typo and disagree on this one.

The same trap reaches any string a program compares rather than renders: a key, an identifier, a slug, a path. Where the value is compared, say in the file's own comment which normalisation it is stored in, so the next writer has something to match rather than a glyph to copy.




## A comment strengthened past what a review asked, into a claim the code contradicts

Caught at Agent Code Review on dinah-433, 2026-09-08, and it is the second catch of the same claim on one card. Agent Design Review had already flagged the weaker form of it and asked for the justification to be tightened before any code was written.

A review that asks an author to strengthen a justification is asking for a better reason, and the author supplies one by writing a firmer sentence. Firmness and truth come apart at that moment. The reason under review was about a message the reader sees, and the firmer sentence became a claim about control flow in a function nobody opened. Nothing in the pipeline reads a comment against the code it describes, so the claim shipped in two production files and then travelled into the user-facing advice, which is where it cost a reader something.

**Wrong.** dinah-433 minted a refusal for a container that will not list, and both the code comment in `soleBench` and the doc comment on the constant justified minting it this way:

    // ... and that advice is wrong for a container replaced by a plain
    // file, where nothing about permissions is at fault and --workbench
    // does not route around the directory the search has to open.

`DiscoverSource` (`internal/bench/bench.go`) returns as soon as its `override` argument is non-empty, before it calls `walk`, and `--workbench` and `DINAH_WORKBENCH` are the two values that populate that argument. A reader who passes either one never reaches the container at all. The stored next step was then written to match the comment, so it told a stuck reader to restore the container from a backup or from git and withheld the remedy that would have had them working in ten seconds.

The design review's own minor finding had said the milder true thing, which is that the sibling refusal's escape generalises to a broken container fine. The half of the justification that was sound, that "fix its permissions" is wrong advice for a plain file sitting where a directory belongs, carried the false half past a second reader.

**Right.** Hold a comment to a test wherever it asserts what another function does. dinah-433's second round corrected both comments and added `TestTheWorkbenchOverrideRoutesAroundAContainerThatCannotBeListed`, which runs the ambient climb over the corrupt container first and requires it to refuse, then names a healthy workbench through the override and requires it to resolve. Disabling the override branch turns the test red and quotes the old false claim back in the failure message. Two comments and eight catalog entries had rested on that claim, and a run now carries it.

**How to catch it.** Read every comment that says what happens elsewhere as an assertion rather than as commentary, and open the function it names. The tell is a comment containing a verb about a code path the file it sits in does not own. Where a review asks for a justification to be strengthened, compare the new sentence against the old one for what was added rather than for whether it reads better, because a strengthened claim is a larger claim and it can grow past anything anybody checked. And where such a claim has already reached user-facing copy, budget for the correction reaching every translation and its source fingerprint, which is the cost that makes this worth catching while it is still a comment.


## A quotation mark a translator reaches for by reflex, in a repository that bans two of the four

Caught by the implementer on dinah-422, 2026-09-08, by the mechanical scan that runs immediately before every commit, and it had already survived being typed, reviewed on screen and round-tripped through a JSON writer.

German quotes a word as „Wort“, with U+201E opening and U+201C closing. Both are correct German and any writer who knows the language produces the pair without thinking about it. This repository's house-style scan bans U+201C and U+201D outright, because those two are what an editor's autocorrect substitutes for a straight quote inside a string or a comment, and one such substitution cost a full review cycle here on 2026-07-11. The ban does not know that this particular U+201C was typed on purpose in a language that needs it.

The trap is that the two halves of the German pair have different standing. U+201E is not on the ban list and passes. U+201C is on it and does not. So a writer who tests only the opening mark, or who reads the pair as one indivisible thing, concludes the pair is allowed.

**Wrong.** Adding the German for a history row that quotes a card's title:

    "text": "{actor} hat die Karte „{title}“ in {toTitle} erstellt.",

Correct German, and the pre-commit scan names the line and refuses it.

**Right.** Spell the closing mark as an ASCII double quote, which is what the card's own specification had already written and which the scan passes:

    "text": "{actor} hat die Karte „{title}\" in {toTitle} erstellt.",

The pair is then typographically mixed, which is a real cost and a small one, and it is the cost this repository has already decided to pay everywhere else. Where a string can be rewritten so no quotation mark is needed at all, that is better than either spelling, and it is worth trying before reaching for the mixed pair.

**How to catch it.** Run the scan, and read the line it names rather than assuming a hit is autocorrect. The scan cannot tell a deliberate German quotation mark from an injected one, so a hit inside a translated catalogue entry is a question about which spelling to ship rather than a defect to undo. Where a specification has already written the string, copy it byte for byte instead of improving its typography, because a specification that has been through review has usually met this rule already and the improvement reopens it.

Two of the four curly marks being banned and two not is the fact worth carrying away. Any language whose quotation convention uses a mark on the list meets this, and the check is one grep against the ban list rather than a judgement about whether the mark was typed on purpose.

## A block registered in the row sweep, cited as the guard for what the block prints under its rows

Caught at Agent Code Review on dinah-435, 2026-09-08. The card's whole deliverable is that a checklist item's own text becomes visible under its row, and the run with that text deleted passes the entire `cmd/dinah` package.

The row sweep is the strongest rendering guard this repository has. It draws every registered block in eight locales, harvests the rows, and pairs each drawn row against an expectation built independently of the renderer, so a reordered field fails by name in every language at once. That strength is what makes it easy to over-cite. A block registered in the sweep looks guarded, and a criterion whose failing case is about the block will happily name the sweep entry as the assertion that would go red.

What the sweep harvests is rows. `tableRow.note`, the free text a row carries underneath itself, is not a row and is never paired. A block declaring `blanksAreLost: true` is harvested from its own indented lines only, and the note is drawn flush left, so the note's lines are discarded before pairing even begins. The comments block has always been in the sweep on those terms, and a comment's body has never been asserted by it either, so the new block inherited a hole rather than opening one.

**Wrong.** dinah-435 registered its block and cited it for a criterion the block cannot see:

    {
        site: renderSite{File: "render.go", Function: "renderDetail", Label: "checklist", Ordinal: 1},
        keys:          []string{"column.checklist.ref", "column.checklist.kind",
                                "column.checklist.state", "column.checklist.owner"},
        blanksAreLost: true,
        opensAt:       "show.checklist", expect: expectChecklist,
    }

AC-1's failing case reads "either item's kind, state, owner or text is missing from the printed table", and its note cited this entry, paired by `expectChecklist`, as the assertion covering the printed table. `expectChecklist` builds each expected row out of four values and the item's text is not among them. Planting `note := ""` in the block, which removes every item's text from every printed row while leaving the rest of the render untouched, runs the whole package green in 115 seconds.

A second plant is worth recording because of how it misleads. Removing the resolution branch as well, so the block becomes `note := ""` with `if false`, does turn the package red, and it does so through `TestEveryStatementOfTheRenderingHeadIsCoveredOrNamed`. That is a coverage guard reporting an unreachable statement. Reading its red as proof that the text is asserted is the mistake this entry is about, one layer further in.

**Right.** Assert the note separately from the row, because the sweep structurally cannot. The narrow form is a single-locale test that runs the render over a fixture whose items carry known text and requires each item's text to appear on its own line beneath the row bearing that item's reference, and requires a resolved item's note to appear under its own label while a pending item's does not. Keep the sweep entry, since it is what holds the four columns and their headings across eight locales, and say in the new test's comment that it exists because the sweep drops notes.

**How to catch it.** Where a criterion names something a render prints, plant its deletion and run the package, rather than reading the registration and believing it. For this sweep specifically, the question to ask of any block is whether the thing the criterion names is a field of a row or something drawn around the rows, because a heading, a rule, a section label and a row's note are four different mechanisms and the pairing machinery sees one of them. `blanksAreLost: true` on the registration is the tell that the block draws prose the sweep is deliberately throwing away.

The citation is the artifact to correct, not only the test. A criterion marked verified against an assertion that cannot fail for the criterion's own failing case is worse than one marked verified against a hand run, because the hand run at least says out loud that it is an observation and invites the next reader to repeat it.


## A test fixture that mocks a wire shape the producing binary never emits

Caught at Agent Code Review on dinah-422, 2026-09-08, as a blocker, and caught a second time in the same diff by the implementer while repairing the first. The review recorded the class and deliberately withheld a pair on the first catch, because this column appends on the second. The second catch arrived before the card left the column.

A unit test that drives a consumer of a tool's machine output has to invent the output, since the tool is not there to run. The fixture is then written from what the consumer's types say, or from what a reader assumes a JSON array looks like, and the consumer is written from the same assumption. The two agree, the test passes, and neither has ever met the tool. Nothing in the pipeline compares a mock against a real run, so the disagreement surfaces at a reader's screen.

The shape most often wrong is the empty one. Go's `encoding/json` writes a nil slice as `null` and an allocated empty slice as `[]`, and which of the two a producer returns is decided several functions away from the surface that marshals it.

**Wrong.** dinah-422 shipped an eight-language sentence for a card whose journal holds nothing, and the sentence could not reach a reader. `bench.ReadJournal` declares `var events []Event` and returns it untouched for a journal with no lines, and returns an explicit nil for a missing file, so `dinah --json log <ref>` prints the literal `null`. The renderer asked that value for its length:

    if (events.length === 0) {
        return t("history.empty");
    }

which throws `TypeError: Cannot read properties of null (reading 'length')`, and the content provider's catch drew the type error where the sentence belonged. The test that would have caught it supplied the shape the code expected:

    const { spawner, calls } = historySpawner({ code: 0, stdout: "[]", stderr: "" });
    ...
    assert.equal(text, "No history is recorded for this card.");

Repairing the renderer alone leaves that fixture describing a tool that does not exist, so the two repairs belong in one edit. Reproduced afterwards by planting the old renderer and the old fixture together, which runs green over a renderer that throws on every real empty journal.

The second catch, one test down the same file, is the same fault in a refusal envelope. The fixture built `{ outcome: "refused", refusal: "dinah.unknown-card", detail: "no card is filed under that reference" }` and asserted on that sentence. Running the built binary against a reference no card carries prints `{"outcome":"refused","refusal":"unknown-card","detail":"probe-999"}`, so the refusal name is bare where the fixture dotted it and `detail` is the reference itself where the fixture wrote prose about it. The assertion passed either way, which is why it survived the first reading.

**Right.** Capture the fixture from a run rather than composing it. Build the binary into a scratch directory, make a throwaway workbench, drive it into the state the test is about, and paste what it printed. dinah-422's repair records the capture in the test's own comment, so the next reader knows the bytes came from somewhere:

    // stdout is the literal null the binary prints for a card with no journal
    // lines, captured from a run of the built binary rather than imagined.

The renderer then establishes it holds an array before it asks one for a length, which costs one line and covers every non-array answer a later binary could give.

**How to catch it.** For every fixture standing in for a tool's output, ask which run produced those bytes. Where the answer is that somebody wrote them, run the tool into that state and compare, and do it first for the empty case and the failure case, since those are the two a consumer's types describe least well and the two a happy-path fixture is least likely to have met. A Go producer's empty collection deserves the check every time, because reading the marshalling code cannot settle it and one run can.

Two smells make the fixture worth opening even before the run. A mock whose value is the tersest possible spelling of the concept, such as `[]` or `{}`, is usually a reader's idea of emptiness rather than a transcript. An assertion that would pass under several different inputs, such as one matching a substring of a detail the fixture itself supplied, cannot report a wrong fixture and so lets one live.


## Padding a value written into a ceiling-bearing table's capped column

Found on dinah-435 (Implement, 2026-09-08), while packing a checklist item's reference, state and owner into the capped first column of the two-column table `dinah help` draws its command list with.

**Wrong.** Pad the parts so the sub-fields line up down the page, and read the aligned sketch you were given as achievable at the call site.

```go
fields := []string{pad(item.Ref, widest) + " " + item.State + " " + item.Owner, item.Text}
```

**Right.** Join with single spaces, and say at the call site why the alignment is not there.

```go
// The three parts are joined by single spaces rather than padded to a
// common width. A capped column is word-wrapped before it is drawn, and
// that wrap rebuilds the value one space between words, so padding written
// here reaches the page collapsed.
fields := []string{strings.Join(packed, " "), item.Text}
```

**Why.** `ceilingRowLine` runs the capped column's value through `breakWords` (or `breakOnOptions`) before it lays the row out, and those rebuild the value from its words with one space between them. Every run of blanks inside the value is therefore lost, whatever the call site wrote. Nothing in `table.go` says so at the point a caller writes a value, and the padded form compiles, runs and looks right until you read the output.

The trap is worth its own entry because the wrong form fails silently in the direction that looks like success: the page still draws, the columns still align, and only the packing inside the capped column is quietly different from what was asked for. The sweep does not catch it either, since `sweptNextFieldColumn` reads a gutter's worth of blanks as a field boundary, so an internally padded value can also confuse the column derivation it feeds.

The general rule this instance belongs to: a value handed to a renderer that reflows it cannot carry its own layout. Where you need alignment inside a reflowed cell, either the renderer learns about the structure or the layout goes.


## Reach for a capped two-column table only when two things are being drawn (dinah-435)

This is the companion to the entry above about a capped column collapsing the padding written into it. That entry records the trap. This one records what to write instead, because an agent who knows only the trap concludes that ragged columns are unavoidable, and one did.

Wrong, when a block has to draw a reference, a state, an owner and a run of prose:

```go
checklist := table{indent: 2, columns: s.columns("checklist", "ref", "description"),
    labels: labelInTheStack, ceilingColumn: 0, hasCeiling: true, wrapTail: true}
for _, item := range detail.Checklist {
    fields := []string{strings.Join([]string{item.Ref, item.State, item.Owner}, " "), item.Text}
    checklist.rows = append(checklist.rows, tableRow{fields: fields})
}
```

Right:

```go
checklist := table{indent: 2, columns: s.columns("checklist", "ref", "state", "owner", "description"),
    labels: labelInTheStack, wrapTail: true}
for _, item := range detail.Checklist {
    fields := []string{item.Ref, item.State, item.Owner, item.Text}
    checklist.rows = append(checklist.rows, tableRow{fields: fields})
}
```

The rule underneath it: `rowLine` pads every field but the last out to its own measured column, and the last field alone goes unpadded, so a value that has to line up with the value below it has to be a column. Packing several values into one field forfeits their alignment, whether or not a ceiling is involved, and a ceiling then forfeits it a second time by word-wrapping the packed value before drawing it. `hasCeiling` also documents itself as drawing correctly only for exactly two columns.

`labels: labelInTheStack` with `wrapTail: true` and no ceiling gives aligned columns and wrapping prose together, on any number of columns, and neither option is new. Reach for the capped form only when the block genuinely draws two things, one of which is a wide value with no siblings to line up with, which is what `dinah help`'s command list is.

One further consequence, worth knowing before it costs a run: `deriveHeadinglessColumns` in `cmd/dinah/row_sweep_test.go` could read the columns of a headingless block of two columns and no more, because it scanned for each boundary only on lines leading where the previous column begins, and nothing leads past the first column. A wider headingless block failed there with "no row of this block shows where column 2 begins" until it was taught to scan onward from the previous column's position on the lines that open a row.


## A declaration inserted between a doc comment and the declarations it documented

Caught at Agent Code Review on dinah-451, 2026-09-08, twice in one commit. The card also carried a deliberate, correct repair of a doc comment its own change had falsified, which is what makes the pair worth recording: the author was actively watching for stale comments and still landed this one twice, because it is not produced by writing a wrong sentence. It is produced by writing a correct sentence in the wrong place.

A run of related declarations very often shares one doc comment sitting above the first of them. Adding a new declaration to that file means picking a line to insert at, and the blank line above such a run reads like the natural seam. Inserting there puts the new declaration between the old comment and the declarations it described. Nothing goes red. The type checker has no opinion about comment placement, the editor's hover shows the new declaration's own comment because the nearer block wins, and a reviewer scrolling the diff sees an added block that is entirely correct on its own terms. What is wrong is only visible in the file, never in the diff: one comment now describes the wrong declaration, and the declarations it used to describe have silently lost their documentation.

**Wrong.** Two constants added to two files, each landing on the seam above an existing run:

```ts
/** The contextValue a column row and a state group row carry. */
/**
 * The contextValue every attachment row carries, whatever its payload.
 * ...
 */
export const CONTEXT_ATTACHMENT = "dinah.attachment";

export const CONTEXT_COLUMN = "dinah.column";
export const CONTEXT_STATE_GROUP = "dinah.stateGroup";
```

```ts
/** The node kinds the grouped producer emits, as TreeNode.Kind spells them. */
/**
 * The collection segment an attachment reference carries.
 * ...
 */
export const ATTACHMENTS_SEGMENT = "attachments";

export const NODE_GROUP = "group";
export const NODE_CARD = "card";
```

In both, the surviving comment now sits above a declaration it does not describe, and `CONTEXT_COLUMN`, `CONTEXT_STATE_GROUP`, `NODE_GROUP` and `NODE_CARD` are undocumented. A reader looking up what a node kind is finds a sentence about attachment references.

**Right.** Insert below the run a shared comment heads, not above it, and put the new declaration with its own comment after the last member the old comment covers. Where the new declaration genuinely belongs first, move the old comment down with the declarations it describes in the same commit, so the pairing survives the insertion. Two adjacent doc-comment blocks with no declaration between them is the shape to recognise, and it is always a defect: a declaration carries one doc comment, so the outer one has been orphaned.

**How to catch it.** The diff cannot show this, so read the file rather than the hunk. Mechanically, the tell is two comment-block terminators with nothing but whitespace between them, which greps in one pass:

```
grep -Pzo '\*/\s*\n\s*/\*\*' <file>
```

More generally, when a diff adds a declaration to a file, look at what sits immediately above the insertion point in the resulting file and ask which declaration that text describes. This is the placement sibling of "A comment enumerating what a change does not cover, left standing after the change covers it": there the sentence went stale where it stood, here the sentence never changed and the code moved out from under it.

## A comment whose premise gained an exception while the conclusion drawn from it stayed unqualified

Caught at Agent Code Review on dinah-455, 2026-09-09, on the third consecutive round of correcting false comments on one card. The seven corrections before it each replaced a claim that was flatly wrong. This one is the shape a correction itself produces, so it survives the review that the correction passes.

A doc comment often states a property and then draws a consequence from it, in the form "A holds, so B follows." When a card falsifies A, the repair that gets written narrows A and leaves B exactly as it was. B was only ever true because A was unqualified, so narrowing A silently makes B false, and B is the half a reader actually relies on. The reviewer checks the sentence that was reported wrong, finds it now correct, and passes the paragraph.

The tell is that the exception is enumerated close enough to the conclusion to look accounted for. A reader who meets "accepts everything but two" and then meets the two exceptions named in the next sentence will credit the paragraph with handling them, without going back to ask whether the intervening conclusion still holds over the narrowed premise.

**Wrong.** `ResolveEntity` in `internal/bench/entity.go`, after the correction:

```go
// ResolveEntity resolves the reference the entity-shaped commands take: ...
// It accepts every reference ResolvePath accepts but two, so a
// reference a walk prints names the same entity to every command that takes
// one. It refuses an attachment's payload, which carries no anchor, and it
// refuses a reference naming a whole collection, which is not an entity of the
// format and has no anchor either. ResolvePath answers both with a path.
```

The premise moved from "the same references" to "every reference but two" and the conclusion did not move at all. One of the two new exceptions is a reference a walk does print: the same card gives `contents` a root node of `TreeNode{Kind: KindCollection, Ref: collection.Ref}` at `internal/verb/tree.go:939`. So a person who copies that root reference out of a walk and hands it to `edit` is refused, which is the case the conclusion denies. The card's own corrected comment on `TreeNode.Ref` says so outright, and the two comments shipped in the same branch contradict each other.

**Right.** The sibling comment on `ResolveReference`, six lines away in `internal/bench/resolve.go`, takes the same premise through the same narrowing and moves the conclusion with it:

```go
// It accepts every reference ResolvePath accepts but one, so a reference a
// walk prints names the same thing to every command that takes one. The
// exception is an attachment's payload: ...
```

That comment says "the same thing" where the other says "the same entity", and the difference is the whole fix. A collection is not an entity, so `ResolveReference` really does answer the walk-printed collection reference with the same thing every one of its callers gets, and the conclusion survives its narrowed premise. The unaddressable exception there is the payload, which no walk prints, so nothing a walk emits falls through the gap.

**How to catch it.** When a card falsifies a doc comment, read the whole paragraph rather than the clause that was reported. For every "so", "therefore" or "which means" downstream of the claim you just narrowed, ask whether the new exception is a member of the set the conclusion quantifies over. Where it is, either narrow the conclusion in the same edit or delete it, because a consequence nobody re-derived is a consequence that was inherited from a premise that no longer supports it.


## A test doc comment that enumerates more cases than its body drives, in the comment a harness reads as the coverage mapping

Caught at Agent Code Review on dinah-436, 2026-09-10, twice in one diff. The card's whole point was to move six conformance statements out of the "we do not implement this" table and into "a named test drives it", so a comment overstating what a test covers is the one defect that converts an honest gap into a false claim of conformance.

The mechanism is that nothing compares a test's prose to its body. `internal/profile/conformance_test.go` maps a statement to a test by regex over the test's source, and a doc comment naming `CORE-LINK-5` is enough to credit the statement as reached. The comment is therefore load-bearing in exactly the way a comment normally is not, and it is written by the same hand, in the same act, as the body it describes.

The tell is a comment that counts. "All five run", "one per fixture", "no fixture carries one": each is a claim about a set, and a claim about a set is checkable by command rather than by reading. The author who writes the sentence has the set in mind and does not go back to count.

**Wrong.** `internal/verb/link_test.go`, where the profile statement names five acts:

```go
// TestNoVerbRefusesOnAccountOfALink drives CORE-LINK-5, which forbids
// reporting one of the profile's refusal names for a claim, a move, a release,
// a block or an unblock refused on the ground of a link. All five run against
// a card carrying links, including one to itself, and all five are admitted.
func TestNoVerbRefusesOnAccountOfALink(t *testing.T) {
	...
	h.mustDo(&Request{Verb: Claim, Card: source, Actor: "bob"})
	h.mustDo(&Request{Verb: Block, Card: source, Actor: "bob", Reason: "stopped for a moment"})
	h.mustDo(&Request{Verb: Unblock, Card: source, Actor: "alka"})
	h.mustDo(&Request{Verb: Claim, Card: source, Actor: "bob"})
	h.mustDo(&Request{Verb: Release, Card: source, Actor: "bob"})
	h.mustDo(&Request{Verb: Claim, Card: source, Actor: "bob"})
```

Four verbs run. `Move` is never issued, and the statement's own observable names it. The card carries the move to the ready column before the links are written, so no move ever happens on a card carrying one.

**Wrong, second instance, same diff.** `cmd/dinah/compat_test.go`:

```go
// The links block is declared at the profile's current revision and no fixture
// carries one, so what this proves is that a card written by an older build
// takes the block a newer one writes and loses it again cleanly, ...
```

The same diff gave `internal/bench/testdata/compat/dinah-core-0.12` a card carrying two link entries, as the recapture the two new journal events required. On that one fixture the test is adding a third entry to an existing block rather than creating a block, which is a stronger case than the comment describes and not the case it describes.

**Right.** Drive the set the statement names, and derive the sentence from the body rather than from intent:

```go
// TestNoVerbRefusesOnAccountOfALink drives CORE-LINK-5 ... Each of the five
// acts the statement names runs against a card carrying links, including one
// to itself, and each is admitted.
	...
	h.mustDo(&Request{Verb: Move, Card: source, Actor: "bob", Column: other})
```

and, where a comment states a fact about a fixture set, check it against the fixture set:

```
grep -rl "^links:" internal/bench/testdata/compat/
```

The general rule this board already carries applies to test prose as much as to published documentation: where a comment states a fact about the code, check it against the code rather than against another sentence. The addition here is that a coverage-mapping comment is not decoration. It is the only input a conformance harness reads, so an overstatement in it ships as a conformance claim.


## A statement credited to an ordering test whose every case proves that statement's refusal is never the one reported

Caught at Agent Code Review on dinah-436, round two, 2026-09-10, by sweeping the whole class rather than the three instances the repair named. The entry above this one records the class. This is the variant that survives a reader who checks whether the named cases run, because here every case does run and the coverage is still absent.

The defect lives on the trunk in `internal/verb/mutate_test.go` and predates dinah-436, which touches neither that file nor any of the statements below.

`TestRefusalOrderFollowsTheProfile` exists to prove that a request failing several checks at once reports the first unsatisfied check. Its doc comment then adds a second claim:

**Wrong.**

```go
// Each case also drives the refusal it lands on, so the table covers
// CORE-VERB-1, CORE-VERB-2, CORE-CLAIM-1, CORE-CLAIM-2, CORE-CLAIM-7,
// CORE-MOVE-1, CORE-MOVE-2, CORE-MOVE-3, CORE-MOVE-6, CORE-MOVE-11,
// CORE-BLOCK-2, CORE-BLOCK-6 and CORE-UNBLOCK-2 as well.
```

Three of those thirteen require the refusal name `held`. CORE-CLAIM-2 requires a claim on an active card to report `held`, CORE-MOVE-3 requires the same of a move on a card another owner holds, and CORE-BLOCK-6 requires it of a block. No case in the table reports `held`, and no case can, because the ordering the test exists to prove is precisely that some other row runs first. Read the `why` fields and the test says so itself: "the operator-owned departure precedes held", "no-reason precedes held", "not-requester precedes blocked". The test proves that `held` is not reported, and its comment credits it with driving three statements that require `held` to be reported.

Nothing else in the tree names those three identifiers, so this comment is their whole coverage mapping. The behaviour itself is exercised, in `internal/verb/pull_test.go` and `cmd/dinah/main_test.go`, but neither names a statement, so the harness credits the test that disproves the requirement and misses the tests that meet it.

**Right.** A statement demanding a named refusal is driven by a test that asserts that name. Where an ordering test sets up the condition and lands elsewhere on purpose, say that, and credit the statement to the test that reports the name:

```go
// Each case also drives the refusal it lands on, so the table covers
// CORE-VERB-1, CORE-VERB-2, CORE-CLAIM-1, CORE-CLAIM-7, CORE-MOVE-1,
// CORE-MOVE-2, CORE-MOVE-6, CORE-MOVE-11, CORE-BLOCK-2 and
// CORE-UNBLOCK-2 as well. The held cases are set up and deliberately
// land elsewhere, which is what the ordering means, so CORE-CLAIM-2,
// CORE-MOVE-3 and CORE-BLOCK-6 are driven where held is reported.
```

**The sweep that finds this class mechanically.** Reading 64 doc comments against 64 bodies is what missed it twice. Deriving the requirement from the profile and testing the credit against the body finds it in one run. For every statement of the form "reporting the refusal name `x`", take the tests the harness credits it to, strip their comment lines, and require the constant for `x` to appear in what is left. Over this tree that is 25 statements, 23 of them credited, and it returns four hits of which one is a naming false positive (`unsupported-version` is spelled `contract.UnsupportedVer`). The other three are the ones above.

The same shape generalises past refusals. Wherever a statement's observable is a named token the code carries as a constant, the credit can be checked against the body by machine, and a coverage mapping that no machine checks is an assertion nobody has read.


## A word that agrees with an interpolated token the sentence never sees

Caught at Agent Code Review on dinah-460, rounds 1 and 2, 2026-09-09 and 2026-09-10. Round one found the first instance by rendering the sentence. Round two's author found a second that nobody had reported, by writing a guard for the class rather than patching the instance, and that second one was waiting for a vowel-initial value to expose it.

A sentence that interpolates a value has to be written for every value the placeholder can carry. English indefinite articles are the loud case, because the choice between "a" and "an" is made before the value is known, and a sentence generalised from one hard-coded noun keeps the article that agreed with the noun it used to name. Gendered languages have the same defect in a worse form: the token substituted at the placeholder is untranslated English, so it has no gender in the target language at all, and any article or possessive standing next to it is picking one at random.

**Wrong.** `refusal.dinah.unknown-field` on dinah-460 read `The fields a {kind} records are: {fields}.` The sentence it replaced hard-coded "a card", so the article agreed by construction. Generalised to seven kinds it rendered "a item" and "a attachment". The German carried `Die Felder, die ein {kind} führt, sind:`, where "ein" assigns masculine or neuter gender to an English word that has none. A second entry, `refusal.dinah.unknown-path.next-addressed`, read `; a {addressed} is named by its own reference`, and stayed correct only because the two values reachable that day were "card" and "column".

**Right.** Name the value without an article. `The fields of {kind} are: {fields}.` reads correctly for all seven kinds, and an eighth kind cannot break it, because nothing in the sentence depends on which values exist. That property is what makes the rule cheap enough to hold everywhere, and dinah-460 holds it with `TestNoEnglishSentenceTakesAnArticleBeforeAPlaceholder`, which reads every English entry carrying a placeholder, logs how many it read, and is fatal on zero.

Two things this class teaches beyond the article itself.

The guard belongs on the English catalogue alone, and that is a precision decision rather than a shortcut. German writes "an" as a preposition, so the same pattern run over `de.json` reports five hits and every one of them is correct German. A rule applied across every message in a product is worth having only where it can be stated tightly enough not to cry wolf, and the tight statement here was "the English catalogue, indefinite articles, immediately before a placeholder".

The article is not the only word that agrees. dinah-460 dropped "ein" from the German `next-addressed` entry and left "seine eigene Referenz" and "was es hält" standing, both of which select gender for the same untranslated token, while adding a context note to that entry declaring that no word of the sentence may agree with it. So the repair fixed the instance the reviewer had named and shipped a fresh false claim beside it, which is the shape "Do not close a paragraph with a sentence nobody asked for" warns about. When you write a rule into an entry's context, read the entry's own text back against the rule you just wrote.

## A test asserting the value the write happened to produce, where the write was the defect

Caught at Agent Code Review on dinah-473, 2026-09-10, from the implementer's own self-catch. The corpus already holds "A golden fixture regenerated so that it records the defect instead of failing on it", which reaches the same end by regenerating a fixture from the changed code. This is the second catch of a guard encoding the defect, arrived at by a different route: the assertion was written by hand, against a value nobody had checked was the right form.

**Wrong:** guarding a write path by filing a value, reading the stored field back, and asserting it equals what was filed. Such an assertion is true of whatever the code does and says nothing about whether the code is right. A case on dinah-473 filed a checklist item against a column by the short name a person types, then asserted the stored field carried that short name. The gate that consumes the field matches on the column's identifier, so the stored short name matched nothing, the hold never fired, and a card walked through the station the workbench meant to stop it at with nothing reported. The guard existed, it ran, and it passed, and what it protected was the defect.

**Right:** pin the expected value to the code that consumes the field rather than to the code that wrote it. Where a write path exists to feed a reader, the assertion is written in the reader's vocabulary, and the case runs the whole way to that reader as well as to the stored field. On dinah-473 that meant filing by the short name a caller types, asserting the stored value is the identifier, and then calling the gate itself and asserting the card is held.

**The test:** for every assertion of the shape "the stored field equals the argument I passed", ask what else reads that field and in which form. Where nothing reads it, the assertion pins a value nobody consumes and is harmless. Where something does, ask whether the case would still be green if the write stored the wrong form. If it would, the case is not a test of the write. Watch also for the inverse, which passes a careless read in the other direction: a fixture that files the consumer's own form, such as an identifier where a caller would type a name, passes whether or not the resolution happens and proves neither half.

## An enumerated form read only in its first element

Caught at Agent Code Review on dinah-488, 2026-09-12, from the round-2 reviewer's own attack corpus. The card-reader guard enumerated Go's binding forms and then read only the first target of two of them, and the escape needed no exotic syntax: one var line carrying two names.

**Wrong:** a guard that enumerates a form reads one element of it and calls the form covered. The card-reader guard on dinah-488 bound taint from an assignment's Lhs[0] and a var declaration's Names[0] alone, on the stated ground that a free reader answers its card in the first result and its error in the second, so tracking every target would taint the error and report every error return in the file. A round-2 review planted `var first, second = card, card` and returned the second name, and the card left the function with no violation, because the second target's own right-hand side carried the same tracked card and nothing read the pair. The ground was true and the clause did not follow from it: pairing each target with its own right-hand side taints no error, because the error of a two-value call pairs with nothing at all.

**Right:** an enumerated multi-element form is read per element, each target paired with the element it takes its value from, and the stated ground is tested against the per-element reading rather than standing in for it. For the assignment and var forms that means Lhs[i] with Rhs[i] and Names[i] with Values[i], stopping where the right side runs out, which is exactly where the error result of a two-value call stops. The ground survives and the laundering closes, because `first, second = card, card` tracks both names while `card, err := LoadCard(root, id)` still tracks only the card.

**The test:** for every clause that enumerates a form, ask whether the form has instances with more than one element, and whether the clause reads every element or only the first. Where it reads only the first, ask what the stated ground actually forbids, and whether the per-element reading would violate it. A ground about not tainting an error result is the common case, and per-element pairing preserves it whenever the extra elements pair with nothing.

## A capture the walk read before the binding that fills it, through a closure holding the variable by reference

Caught at Agent Code Review on dinah-488, 2026-09-12, in round 3, from the reviewer's attack corpus. The taint walk ran one pass in source order, and its own doc comment stated that order as a feature: a binding is tracked before the statements below it read. A closure defined above the binding captures the variable by reference, so the value the closure answers at call time is the card, while the pass reads the closure while the tracked set is still empty.

**Wrong:** the walk visits the closure's return with nothing tracked, then tracks the name at the assignment below it, and answers no violation anywhere, because the binding clause records nothing at its own line and the closure's own name carries no taint:

```go
var d any
var err error
f := func() any { return d }
d, err = LoadCard(root, id)
return f, nil
```

The card leaves the function inside the returned closure, and the same shape one clause over launders a reader through an alias bound the same way.

**Right:** tracking and reporting run as separate passes, or each function literal is re-read once the enclosing function's bindings are complete, so a capture's answer is judged against every binding in the function rather than only the ones lexically above it.

**The test:** for every order-dependent walk, write the binding below a closure that captures it and check whether the guard still answers. Source order describes where a reader's eye travels, and Go's scoping does not follow it, because a closure body may mention a name lexically before the assignment that fills it.

## A walk that finds its target by asserting the top-level node, missing the same node nested inside a container

Caught in the round-4 hostile sweep on dinah-488, 2026-09-12, by the implementer's own probe, before any reviewer saw it. The blocker-3 clause walked a function literal standing in a package var's initializer, and its discovery read only an initializer that was a literal itself. A literal nested inside a composite literal in that same initializer is stored just the same, and the walk never reached its body, so the reader it answered counted as a budgeted reference and no violation named the shape anywhere.

**Wrong:** the discovery type-asserts the initializer itself and walks the literal only when the assertion answers:

```go
for _, initial := range value.Values {
	if literal, isLiteral := initial.(*ast.FuncLit); isLiteral {
		found = append(found, valueFindings(literal, literal.Type, bound, at, composed)...)
	}
}
```

The walk exists and is correct, and the discovery is the hole, because the same node standing one level down never reaches it.

**Right:** search under the initializer for the node kind and walk each outermost one, stopping the search at the first literal it meets, because the walk over a literal reports the literals nested inside it and a second entry in the list would report their bodies twice:

```go
for _, initial := range value.Values {
	for _, literal := range outermostLiteralsIn(initial) {
		found = append(found, valueFindings(literal, literal.Type, bound, at, composed)...)
	}
}
```

**The test:** for every walk that targets one node kind, plant that same node nested inside a container the caller accepts, and check whether the walk still reaches it. A discovery that asserts the top level answers only the shapes its author happened to write first.


## A taint walk that reads every binding form and no form that reads a value back out of one

Caught in the round-4 hostile sweep on dinah-488, 2026-09-12, by the implementer's own probes, in both halves of the guard at once. The walk bound a name from a container holding a reader, because the composite literal read its elements, and then read nothing of the container again: an index, a slice, a type assertion, a type switch, and a field read out of that binding all laundered the value into a fresh name no clause tracked, a range clause over the container bound no alias, and a call placed through such an extraction was invisible to the callee read. The card half held the same hole at its binding boundary, through an index and a slice.

**Wrong:** the alias read stops at identifiers reached through parentheses and the address-of and dereference operators, and every extraction form falls to the default clause:

```go
held := []any{LoadCard}
read := held[0]
return read
```

The reader leaves the function inside `read`, and no clause answers, because the walk never asks what an index of an alias holds.

**Right:** the extraction forms read through what they read out of, on the ground the range clause's both-names binding already rests on: the holder carries the value somewhere inside it and the syntax cannot say which element or field. An index, a slice, a type assertion, and a field descend to the holder, the range clause binds aliases out of an alias subject, and the callee read delegates to the same reads, so a call placed through an extraction calls what the holder holds. A field read stays outside the card half's reads on purpose, because a card's own fields are read all over the tree and are not cards; the comment on that read names the half it leaves open.

**The test:** for every binding a walk creates from a holder, read the value back out of that holder in every form Go spells, and check whether the taint survives the trip. A walk that binds holders and never reads them again has written the launderer's half of the protocol for it.

## A module header citing the guard that holds it, by a path no file occupies

Caught at Agent Code Review on dinah-490, 2026-09-13. `src/reporter.ts` declares the two channel sets the whole one-message perimeter rests on, and its header tells the reader who holds them: the sets "are read by the guards in `test/unit/perimeter.test.ts` at test time rather than copied into them, so the sets and the code that honours them cannot drift." The guards are real and they do read the sets, but they live in `test/unit/wiring.test.ts`. Nothing named `perimeter.test.ts` has ever existed in the repository, and the only occurrence of that string anywhere in the tree is the sentence claiming it. The citation is the entire warrant for the "cannot drift" claim, so a reader who wants to check the warrant is sent to a file that is not there, and a reader who does not check carries away a guarantee nobody verified.

This survives review by default for the same reason a wrong count does. A criterion asserting that the header names its guard passes against a header naming any guard, and a reader skimming for the shape of the sentence finds the shape.

**Wrong:** the header names the file it believes holds the guard.

```ts
// Nothing here imports vscode. The two channel sets are read by the guards in
// test/unit/perimeter.test.ts at test time rather than copied into them, so
// the sets and the code that honours them cannot drift.
```

**Right:** the header names the file that holds it, checked against the directory rather than against the author's memory of where the test was going to go.

```ts
// Nothing here imports vscode. The two channel sets are read by the guards in
// test/unit/wiring.test.ts at test time rather than copied into them, so
// the sets and the code that honours them cannot drift.
```

**The test:** every path a comment spells is a claim about the filesystem, so resolve it. Grep the tree for each cited filename before the diff leaves your hands, and treat a single hit, the citation itself, as the failure rather than as the match. A file that was renamed during the work, or that was named while the author still meant to create it, reads exactly like one that is there.

## A double assertion telling the compiler an object is a type it does not satisfy

Caught at Agent Code Review on dinah-506, 2026-09-15, at two sites in one diff, which is what makes it a class rather than a slip. `as unknown as` is not a widening and it is not a conversion. It switches the check off for one expression, and what is left holding the code up is an unrecorded fact about the callee's body: that the function happens to read only the members the value really has. That fact is true when the cast is written and nobody is told when it stops being true, so the next member the callee reads compiles, ships, and fails at run time with `undefined is not a function`.

Both sites were live. In `editors/vscode/src/commentDrafts.ts` a `DraftHost`, which carries filesystem calls and two window calls, was asserted into `CommandHost`, which declares a clipboard, a quick pick, an input box, two file openers, a served-text opener, a confirmation dialog, and a checkpoint. The object had none of the first six. The call survived because `runVerb` reads `host.showError` and `host.checkpoint` and nothing else. In `editors/vscode/src/itemCommands.ts` a `FileItemContext` was asserted into `ItemCommandContext`, which requires `card: string` and `view: ItemView`; the object had neither, and that assertion bought nothing at all, because `FileItemContext` already declares every member the function it was passed to reads.

**Wrong:** the value is asserted into the type the parameter names, and a comment is not even available to record why it holds, because the reason lives in another module's body:

```ts
const context: CommandContext = {
	spawner,
	exe,
	host: host as unknown as CommandHost,
	folder: entry.folder,
	root: entry.root,
	ref: entry.target,
};
const outcome = await runVerb(context, entry.argv, text);
```

**Right:** the parameter asks for what the function actually reads, so the value satisfies it and the compiler goes on checking. `runVerb` takes a `VerbContext` whose host declares the two members it uses, `CommandContext` extends that context with the full host, and every existing caller is unchanged:

```ts
export interface VerbHost {
	readonly showError: (message: string) => void;
	readonly checkpoint: (folder: string) => Promise<void>;
}

export interface VerbContext {
	readonly spawner: Spawner;
	readonly exe: string;
	readonly host: VerbHost;
	readonly folder: string;
	readonly root: string;
	readonly ref: string;
}

export interface CommandContext extends VerbContext {
	readonly host: CommandHost;
}
```

The second site needs no new type. Deleting the assertion is the whole fix, and the call then type-checks on `FileItemContext` as it stands.

**The test:** grep the diff for `as unknown as` and read each hit against the type it lands in, member by member. Where the value is missing a member the target declares, the cast is load-bearing on the callee's body and the repair is to narrow the parameter or to give the value the members it claims. Where the value is missing nothing, the cast is dead and the repair is to delete it. Neither repair is a comment, and neither is a test: a test around a lie asserts what the code does today, and the compiler is the only reader that will still be checking when somebody edits the callee. A narrowing widening, such as a JSON import read as its declared shape or an unknown error read as an error, is a different class and is not this.

## A zero-spawn assertion whose driver was never answered far enough to spawn

A command's refusal is proved by driving the command and asserting that nothing was spawned. The driver supplies no answers to the form the command opens, so the first prompt declines, the command returns on the cancellation path, and the count is zero for a reason that has nothing to do with the guard under test. A guard that complained about the selection and then acted anyway satisfies the same assertion word for word.

Caught twice on dinah-506, both times inside the fix written to close the previous round's unarmed-test finding. In the second instance the five-step filing form was driven with an empty host log, so `host.pick` answered `undefined` at the first step, `askFileItem` returned long before the multi-row guard could matter, and `assert.deepEqual(raise.calls, [])` held. Deleting that guard's `return undefined;` while leaving its error toast in place kept all 696 unit tests green.

**Wrong.** Every other arm of the same sweep scripts its inputs, which is what makes this arm's silence invisible.

```ts
const raise = await invoke(COMMAND_FILE_ITEM, [raiseRow("wb-1"), raiseRow("wb-2")]);
assert.deepEqual(raise.calls, [], "Raise spawned over a two-row selection");
```

**Right.** Answer every prompt that stands between the entry point and the action, so that a zero reads as a refusal rather than as a cancellation.

```ts
const raiseLog = emptyLog();
raiseLog.typed = "Which column settles this?";
const raiseAnswers = [
	{ label: "Open question", value: "open_question" },
	{ label: "here", value: "here" },
	{ label: "The operator", value: "operator" },
];
let raiseAt = 0;
const raisePicking: HostLog = {
	...raiseLog,
	get picked() {
		return raiseAnswers[Math.min(raiseAt++, raiseAnswers.length - 1)];
	},
} as HostLog;
const raise = await invoke(COMMAND_FILE_ITEM, [raiseRow("wb-1"), raiseRow("wb-2")], {
	log: raisePicking,
});
assert.deepEqual(raise.calls, [], "Raise spawned over a two-row selection");
```

**The test:** for any assertion that a count is zero, walk the prompts between the entry point and the action and ask what the driver answered at each one. A prompt the driver never answered is an early return, and an early return makes the zero free. The plant that settles it is to leave the guard complaining and remove only its refusal, so the code says no and then does the thing. A plant that deletes the guard outright is weaker, because it can redden the error assertion while the spawn assertion stays untested.

**Related:** "An absence assertion with no control that the thing ever existed" is the same zero misread, but there the fixture does drive the path and the code loses the subject upstream, so the tell sits in the code; here it sits in the driver's own inputs and the code is innocent. "A negative table row refused by a guard other than the one it is named for" covers the run that reaches a guard, just not the one the row is named for. "A test written against a refusal the build cannot reach" is the compile-time form of the same unreachability.

## A rule over row shapes tested against the shapes its author had in mind

Caught at design review on dinah-518, 2026-09-16, and it is the third sighting of this class. A rule selects rows: a manifest `when` clause deciding which tree rows a command appears on, a filter deciding which entities a sweep visits, a switch deciding which events a renderer draws. The test then exercises the rule over a list of row shapes, and that list is written by the same person in the same sitting as the rule. The two agree by construction, and the test catches nothing the author had not already thought of.

The population is not a matter of judgement. Some function produces the row shapes, and its own returns are the whole population.

**Wrong.** Three rounds of dinah-518's contract prescribed `/^dinah\.column\./` for a column command's `when` clause, and its criterion asked that the clause be run against "each of the context-value suffixes the extension already emits". `columnActionsFor` (`editors/vscode/src/tree.ts`) returns five values, and four of them carry a suffix. The criterion's phrasing selects those four, all of which the prescribed pattern matches, so the criterion passed while its own first sentence, "on every column row", was false of the build that passed it. The fifth value is the bare `dinah.column`, which the function returns whenever the status and tree answers of one checkpoint disagree, which happens on every first paint. That is the row the command was missing.

**Right.** Derive the population from the producing function and assert its size before using it.

```ts
function everyColumnContextValue(): string[] {
	const values = [
		columnActionsFor(undefined),
		columnActionsFor(view({ count: 1, capacity: 0 })),
		columnActionsFor(view({ count: 2, capacity: 2 })),
		columnActionsFor(view({ count: 1, capacity: 0, takes_work_up: false }), "doing"),
		columnActionsFor(view({ count: 2, capacity: 2, takes_work_up: false }), "doing"),
	];
	return [...new Set(values)];
}

assert.equal(values.length, 5);
assert.ok(values.includes("dinah.column"));
```

The two assertions do different work. The count catches a branch the extension adds later and nobody enumerates here. Naming the one value that separates the candidate rules catches the rule this entry is about, which is why it is named rather than left to the count.

**The test:** for any rule that selects rows, ask which function produces a row's selector, and write the population by calling it rather than by listing what you expect it to return. Where two candidate rules are both plausible, find the input that separates them and assert that input by name, because every other input passes both. A criterion that describes the population in words, such as "each of the suffixes the code emits", is the tell: the words were written after the rule and they select what the rule already matches.

**Related:** "A zero-spawn assertion whose driver was never answered far enough to spawn" is the same failure in the driver rather than in the population. "A refusal that any value satisfies is not a refusal" is the degenerate case, where the population is the whole input space and the rule accepts all of it.
