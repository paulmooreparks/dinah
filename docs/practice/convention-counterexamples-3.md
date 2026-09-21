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

## A permission test that seals the file, where the format document already says the directory governs

Caught by continuous integration on dinah-514, 2026-09-15, after the same suite ran green on Windows. Two tests passed on `test (windows-latest)` and failed on `test (ubuntu-latest)` and `test (macos-latest)`, and the red was the honest answer: the write the tests expected to be refused had genuinely succeeded.

Every write in this format is a temporary renamed over its target, so the right that governs a write is the right to replace a name. POSIX grants that right through the containing directory and says nothing about the mode of the file being replaced. Windows asks the file's own attribute instead. The two are exact mirror images, and `docs/design/format.md` states the rule in full, under the ordinal migration, along with the reason the tool checks no permission ahead of a write.

The tell is a test that constructs "cannot be written" out of one platform's spelling. `os.Chmod(path, 0o444)` on the destination reads like the obvious way to make a file unwritable, and on Windows it is. On POSIX it changes nothing that matters, the write succeeds, and the assertion that a conflict was reported fails. Worse is the version that notices the failure and skips on POSIX, which converts a test that found three defects on that card into one that cannot fail on two of the three platforms it ships to.

**Wrong.** Seal the file, on every platform, from the criterion's own literal wording.

```go
locked := dirty[0]
if err := os.Chmod(locked, 0o444); err != nil {
	t.Fatalf("chmod: %v", err)
}
t.Cleanup(func() { os.Chmod(locked, 0o644) })
```

**Right.** Seal whatever the platform says governs, in one helper that carries the rule and cites where it is written down, so every platform runs the case and none of them runs a vacuous one.

```go
// sealDestination makes one destination genuinely impossible to write, in the
// way that is real on the platform the test runs on, and restores it when the
// case ends.
//
// Windows and POSIX disagree about what governs replacing a file, and the
// format document already says so, under the ordinal migration: every write in
// this format is a temporary renamed over its target, so the right that governs
// is the right to replace a name, POSIX grants that right through the
// containing directory, and Windows asks the file's own attribute instead.
func sealDestination(t *testing.T, path string) {
	t.Helper()
	target, sealed, open := path, os.FileMode(0o444), os.FileMode(0o644)
	if runtime.GOOS != "windows" {
		target, sealed, open = filepath.Dir(path), os.FileMode(0o555), os.FileMode(0o755)
	}
	if err := os.Chmod(target, sealed); err != nil {
		t.Fatalf("seal %s: %v", target, err)
	}
	t.Cleanup(func() { os.Chmod(target, open) })
}
```

**The test:** for any test that constructs a filesystem refusal, name the operation the code actually performs and ask which platform right governs that operation. Where the answer differs by platform, the helper carries the difference and every platform keeps running the case. The plant that settles it is to seal the wrong one of the two and require the case to go red; a case that stays green under the wrong seal was asserting nothing on that platform. Grep the format document before writing the rule down afresh, because a rule already stated there is a rule the reviewer will hold the test to.

The same question reaches the implementation, and on this card it found a live defect. The repair took a per-file lock before reading, and read any failed acquisition as "another process holds this". On POSIX a sealed directory refuses the lock file first, so a permission failure was reported as a busy file, under the one condition the run is built to tolerate and retry. A reader would have been told to run the command again over a condition that running it again cannot clear. Distinguishing the refusal that means "held" from every other failure to acquire is the fix, and nothing on Windows could have shown it.

**Related:** "A criterion whose fixture exercises nothing the ordinary fixture beside it does not" is the same vacuity arriving through the fixture rather than through the platform. The entry on a guard whose justification carried a false half past a second reader is the nearest instance of prose about permissions going unchecked.

## A source guard matching the spelling of the lookup that is there, saying nothing about the lookup that is not

Caught twice on dinah-519, once by the implementer and once by Agent Code Review. The card deletes a composed page served for a checklist item, and part of its purpose is that the page cannot come back in disguise. A page is served by registering a renderer against a kind in a resolver table, so the guard has to establish that the served-text path consults exactly one such table. What shipped instead read the source of `extension.ts` for occurrences of `resolvers[...]` and asserted the set of hits.

A regular expression over a lookup finds copies of that lookup. It cannot see a lookup written against something else, which is the only way the defect ever arrives: an implementer restoring the page does not rename the table that is already there, they add a second one beside it. The reviewer wrote that second table into `provideTextDocumentContent`, ahead of the surviving lookup, and the whole suite stayed green.

The replacement then had to be defeated twice more before it was finished. Parsing the registration construct and enumerating what its body consults refuses a table name nobody foresaw, which is the whole gain over matching text. It refuses only the lookup constructs the walk collects, so a third way of spelling a lookup inside that same body escapes it, and Agent Code Review escaped it on the second pass with a computed property in an object binding pattern. It also does not refuse the same table consulted one function call out, because a walk over a lexical body stops at the body. The third block below carries the call-following repair, and the reach a guard claims is as much part of it as the reach it has.

**Wrong.** The set of spellings, matched as text.

```ts
const source = readFileSync(join(__dirname, "..", "..", "..", "src", "extension.ts"), "utf8");
const lookups = [...source.matchAll(/resolvers\[[a-zA-Z.]+\]/g)].map((hit) => hit[0]);
assert.deepEqual(
	[...new Set(lookups)].sort(),
	["resolvers[kind]", "resolvers[parsed.kind]"],
	`the resolver table is read from an unexpected site: ${lookups.join(", ")}`,
);
```

**Right.** Find the construct the behaviour actually hangs from, then read what it does. The served-text path is the two function bodies the platform and the refresh loop call, located by the registration that installs each. Inside each one, collect every lookup keyed by a value rather than by a literal, in the two spellings this walk recognises, and assert that the thing being looked up in is always the one table.

```ts
const sites = servedTextSites(file);
assert.deepEqual(
	[...sites.keys()].sort(),
	["provideTextDocumentContent", "refreshLoop.resolve"],
	"the walk did not find both served-text sites, so it read nothing it claims to read",
);
for (const [name, site] of sites) {
	const lookups = keyedLookupsIn(site);
	assert.ok(lookups.length > 0, `${name} performs no keyed lookup at all, so this walk read nothing`);
	assert.deepEqual(
		[...new Set(lookups)].sort(),
		["resolvers"],
		`${name} consults a registry other than resolvers: ${lookups.join(", ")}`,
	);
}
```

**Righter.** Extend the region by one bounded step, so that the lookup cannot escape by moving into a helper the site calls. Collect the names each site calls directly, resolve those names against the functions the same module declares, and read the bodies that resolve as part of the site. One step rather than a fixed point, because following call names without a type checker resolves nothing reliably once shadowing and re-export are in play, and a region a reader can state exactly is worth more than one nobody can characterise.

```ts
const declared = moduleFunctions(file);
assert.ok(
	declared.size > 0,
	"the walk found no function declared in extension.ts, so it can follow no call and read nothing",
);
for (const [name, site] of sites) {
	const callees = directCalleesIn(site);
	assert.ok(
		callees.length > 0,
		`${name} calls nothing by name, so the call-following half of this walk read nothing`,
	);
	const lookups = keyedLookupsIn(site);
	for (const callee of callees) {
		const body = declared.get(callee);
		if (body !== undefined) {
			lookups.push(...keyedLookupsIn(body));
		}
	}
	// ... same enumeration assertion as above
}
```

**The test:** read the assertion and ask what it does when the defect is spelled a way the author did not picture. A guard naming the good value can only report that the good value is still present, and presence is not exclusivity. Turn the question round so the guard enumerates what is there and holds the whole enumeration against an expected set, which makes an unforeseen table name an extra member rather than a miss. The plant that settles it is the defect itself: add the second table, consult it ahead of the first, and watch the guard name it. Then run the same plant again with the lookup moved into a helper, because that is where the first structural repair stopped and its comment did not say so. Widening the pattern to cover the plant is the wrong repair and this project has paid for it twice, because the widened pattern fits its examples and nothing else.

**What such a guard still cannot see, and saying so.** A structural guard is bounded twice over, and a comment naming one of the two bounds misleads about the other. Distance bounds it, because it reads a stated region of code and nothing outside that region. Spelling bounds it inside that region as well, because it recognises a closed list of syntactic constructs and declines every other way of writing the same operation.

Write the positive list, which is short and exact, and let the negative follow from it. This guard collects a lookup written as an element access whose key is not a literal, and a lookup written as a one-argument call through a property named `get`. It follows a call whose callee is a bare identifier, one step and no further, and it resolves that name only against a named `function` declaration or a variable whose initializer is directly a function expression or an arrow. Everything else that reads a value out of a table is invisible to it, wherever it sits.

Then give the escapes as demonstrated examples, each with a reproduction, and say what the list of them is worth. Six are on the record for this guard, and every one of the six was written into the source and run rather than argued from reading the collectors: a dispatch consulting no table at all, of the shape `if (parsed.kind === "item") { return renderItem(...); }`; a lookup two calls out, where a followed helper calls a second helper that holds the table; a lookup in a helper the module imports rather than declares, or obtains from a factory call, or binds through a cast or a `satisfies`, since none of those is an initializer the walk reads as a function; a lookup behind a call through a property access, whose callee carries no bare name to resolve; a second table read by computed destructuring, `const { [parsed.kind]: page } = itemPages;`, written into the site's own body; and `Reflect.get(itemPages, parsed.kind)`, which is a call but not a call through a property named `get`. That list is not known to be complete, and it must not be written as though it were. It records the shapes somebody has run. Nothing in it rules out a seventh, because the positive list above is a list of constructs and the language spells an indexed read in more ways than anybody here has enumerated.

The count is where this goes wrong in practice. A sentence saying "four shapes fall outside" reads as an exhaustive partition, so a reader holding a fifth shape checks it against the four, finds no match, and concludes the guard sees it. Naming the shapes without a count of what falls outside, and saying outright that the list is open, costs one clause and closes that reading off. A count of the runs on the record is a different number and stays, because it measures how much evidence there is rather than partitioning what escapes. This entry's own first draft named one hole and implied the rest were covered, and its second named four and implied the same. Each was caught by the next reviewer with a plant from outside the count.

**Related:** "A universal claim generalised from the cases the reviewer named", in `convention-counterexamples-1.md`, is the general form of the two false sentences this entry kept producing, and its test applies here unchanged: for every universal quantifier, and a closed count is one, name the run that would falsify it before you write it. "A sweep bucketed by the preceding word, run over a tree whose identifiers are CamelCase" is the nearest neighbour on the guard itself, and it is a neighbour rather than the same entry: there the sweep reads the right construct and mis-tokenises it, here the sweep reads a spelling instead of a construct. "A walk that finds its target by asserting the top-level node, missing the same node nested inside a container" is the same failure inside an AST walk that is otherwise structural.

## A rule over a set, tested against the members its author had in mind

Caught at design review on dinah-518, 2026-09-15, and it is the third sighting of this class. A rule selects members of a set: a filter deciding which collections a sweep visits, a switch deciding which stored events a renderer draws, a pattern deciding which context values a menu entry appears under. The test then exercises the rule over a list of members, and that list is written by the same person in the same sitting as the rule. The two agree by construction, and the test catches nothing its author had not already thought of.

The population is not a matter of judgement, and it is not a list to be recalled. Something in the code produces the members, and whatever that is, its own output is the whole population. Derive the list by calling it.

**Wrong.** Three rounds of dinah-518's contract prescribed `/^dinah\.column\./` for the pattern deciding which context values a new command appears under, and its criterion asked that the pattern be run against "each of the context-value suffixes the extension already emits". `columnActionsFor` (`editors/vscode/src/tree.ts`) returns five values, and four of them carry a suffix. The criterion's own phrasing selects those four, every one of which the prescribed pattern matches, so the criterion passed while its first sentence, "on every column row", was false of the build that passed it. The fifth value carries no suffix, and the function returns it whenever two answers of one checkpoint disagree, which happens on every first paint.

**Right.** Derive the population by calling the producer, and assert its size before using it.

```ts
function everyColumnContextValue(): string[] {
	const drawn = [
		columnActionsFor(undefined),
		columnActionsFor(view({ count: 1, capacity: 0 })),
		columnActionsFor(view({ count: 2, capacity: 2 })),
		columnActionsFor(view({ count: 1, capacity: 0, takes_work_up: false }), "doing"),
		columnActionsFor(view({ count: 2, capacity: 2, takes_work_up: false }), "doing"),
	];
	return [...new Set(drawn)];
}

const values = everyColumnContextValue();
assert.equal(values.length, 5);
assert.ok(values.includes("dinah.column"));
```

The two assertions do different work. The count catches a branch somebody adds to the producer later and does not enumerate here. Naming the one member that separates the candidate rules catches the rule this entry is about, which is why it is named rather than left to the count.

The same card carries the same class away from any menu. `ordinalCollections` appended every collection it walked to its answer and `checkOrdinals` reported every unstamped member of one, which was harmless while a card was the only root because every collection below a card holds stamped members. Rooting the walk at the workbench brought the columns and cards collections into the set for the first time, and the run that showed it was the one over a workbench written entirely by the verbs: it reported one finding per column and one per card. A findings-only test with a planted defect passes against that build, because the planted finding is in the answer along with all the noise.

**The test:** for any rule that selects members of a set, ask what produces a member, and write the population by calling that thing rather than by listing what you expect it to return. Assert the population's size, so a member added later is somebody's problem before it is nobody's. Where two candidate rules are both plausible, find the member that separates them and assert it by name, because every other member passes both. A criterion that describes the population in words, such as "each of the suffixes the code emits", is the tell: the words were written after the rule, and they select what the rule already matches.

**Related:** "A zero-spawn assertion whose driver was never answered far enough to spawn" is the same failure sitting in the driver rather than in the population. "An invariant asserted from one side only", in `convention-counterexamples-1.md`, is the degenerate case, where the population is the whole input space and the rule accepts all of it.

## A derived figure copied into a contract, where the command that derived it would have kept

Caught at code review on dinah-518, 2026-09-15, twice on one card and in two different shapes, which is what makes it a class rather than two accidents.

A criterion, a table or a handoff states something a command worked out: how many lines a sweep answered, which files it named, which sites carry a falsified sentence. The command ran and its answer was right. Then somebody copied the answer into prose, and from that moment the prose is a fact about a revision nobody names, checked by nobody, and drifting from the moment the next merge lands.

**Wrong.** Two instances, one dropping a row and one keying a count to a dead revision.

A specification swept every source comment asserting a comment's containment, published the command that did it, and listed the nineteen sites the answer carried. The answer carried twenty. One fell out between the sweep and the table, the sentence it named shipped falsified, and the same sentence four files away was corrected during a merge, so the repository shipped one file saying a comment hangs only below a card and its neighbour saying it hangs below a column too.

The same document asserted that its sweep answers "128 lines across 37 files before the edits". It did, at the trunk the round was written against. Another card landed in between, the branch's own merge base answers 156 lines across 39 files, and the criterion could no longer be satisfied by anything. The handoff had listed which counts the merge moved and this one was not among them.

**Right.** Where a figure or a list comes from a command, put the command and the revision it runs at into the contract, and leave the answer out of it.

```
Wrong: "the sweep answers 128 lines across 37 files before the edits"
Right: "the sweep is re-run at this branch's merge base, which
        git merge-base origin/main HEAD names, and what it answers there is
        recorded in the handoff beside the base it was taken at"
```

Better still, where the population is walkable by a test, walk it. The nineteen-site table became a test that reads every non-test Go and TypeScript file under the three trees the sweep reads and fails on the stale clauses by name. A sweep that runs on every build cannot drop a row while somebody copies it, and it cannot go stale against a revision, because it has no revision in it.

**The test:** for every number and every list in a contract, ask which command produced it. If one did, the contract names that command and the base it runs at; if the contract names the answer instead, the answer is already a claim about a revision the reader cannot see. A merge is the event that falsifies these, and a handoff that lists "the counts this merge moved" is enumerating by hand the very thing this entry says not to enumerate by hand.

**Related:** "A mechanical sweep whose recorded output a later stage reads instead of re-running", in `convention-counterexamples-2.md`, is the nearest neighbour and the entry this one extends. That entry stops a reader trusting a sweep's recorded answer; this one says where the answer should have gone instead, which is nowhere, with the command left in its place. "A review finding repaired at the sites the reviewer listed, when the finding names a class", in the same file, is the same loss between a scan and the repair it drives. "A rule over a set, tested against the members its author had in mind", above, is its sibling on the other side: there the population is recalled instead of derived, here it is derived and then transcribed.

## A phrase quoted from a wrapped sentence, asserted to answer nothing

Caught at code review on dinah-518, 2026-09-15, in the criterion written to prevent it.

A contract requires that a stale sentence is gone, and spells the requirement as a grep that must answer nothing. The phrase is quoted out of the source by eye, out of a doc comment that wraps. A line-oriented grep never answered for it, before the edit or after, so the clause is satisfied by a tree in which nothing whatever was done.

**Wrong.** The criterion quoted "a comment hangs on a card or on one of that card's checklist items" from a doc comment reading

```go
// same for every kind that fails it: a comment hangs on a
// card or on one of that card's checklist items.
```

and required that it answer nothing after the edits. It answered nothing before them either. The criterion carried, two sentences later, the rule that would have caught it: each phrase is first run against the merge base and asserted to answer at least one line there. That rule was written because the same defect had already been found in a third phrase of the same list, and the fifth phrase was not put through it.

**Right.** Run every phrase at the base before you write it down, and where the text is prose rather than code, join the comment's continuation lines before scanning so the wrap cannot decide the answer.

```go
// commentProseContinuation matches the break between two lines of one
// comment, in both styles this repository writes.
var commentProseContinuation = regexp.MustCompile(`[ \t]*\r?\n[ \t]*(?://|\*)[ \t]*`)

joined := commentProseContinuation.ReplaceAllString(string(body), " ")
```

**The test:** an assertion that something answers nothing proves nothing until you have watched it answer something. Run it where the defect is known to live, which for a repair is the base the repair was cut from. A rule of this shape written into a contract binds every clause of that contract, including the ones added after it was written, and the clause added last is the one nobody puts through it.

## A check that builds its own driver, where the package already holds a fuller one

Caught at code review on dinah-518, 2026-09-15, on the check written to close the same card's earlier finding.

A guard that compares a document against the code needs something to drive it: a tree to read, a walker to read it with, and a list of what it may not cover. Writing those three by hand produces a guard whose reach is whatever its author thought to exercise, and the author thinks of the shape the card is about. The repository usually already holds a driver with a coverage alarm attached to it, and one line of reuse buys every shape nobody thought of.

**Wrong.** `cmd/dinah/format_event_fields_test.go` built a fixture of eleven acts, a journal walker decoding each line as a raw object, and an exemption list of nine events. All three already stood in `cmd/dinah/compat_test.go`, one file away in the same package: `readShape` returns each event's member names, `sampleFixture` is the tree it reads, and `TestTheSampleFixtureCarriesEveryJournalEventTheContractDeclares` holds that tree to the whole declared vocabulary. The hand-built fixture reached nine events. The one already there reaches thirty-five. Two rows of the very table the new check pinned were false against the current build and sat underneath a green run of it: a `created` line carries `note` when a column is created, and a `moved` line carries `reshape` when a reshape wrote it.

The exemption list inherited the same bound. Deleting `"linked"` from it compiled and left the check green, because nothing in the hand-built fixture wrote a `linked` line, so the list could not self-clean in the direction its own doc comment claimed.

**Right.** Keep the hand-built fixture only for the shape it alone can carry, which here is a locator younger than the frozen capture, and union it with the tree that carries the coverage alarm. Hold the exemption list against the vocabulary the code declares rather than against whatever a fixture happens to write, and assert the reach as a number rather than logging it.

```go
live := readShape(t, benchDir(t, exerciseTheJournalWriters(t))).members
frozen := readShape(t, sampleFixture(t)).members
// walk the union, and fail on a declared event neither half carries
```

**The test:** before writing a fixture, a walker or an exemption list for a guard, grep the guard's own package for the thing it is about. Where a fixture already exists with a test holding it to a declared vocabulary, that test is a coverage alarm you inherit for free and your own fixture has none. Then ask what each half of the union can and cannot carry, and say it in the doc comment, because a frozen capture cannot carry a shape invented after it and a live run reaches only the acts somebody wrote down. Assert the count of what was walked; a guard that reads nine of thirty-five reports success exactly as one that reads all of them.

**Related:** "A rule over a set, tested against the members its author had in mind", above, is the same failure in the population of a rule rather than in the driver of a check. "A guard reported absent after a run bounded to the packages the change touched", in `convention-counterexamples-2.md`, is the bound arriving from the run rather than from the fixture. "A source-walking guard rooted at the package's parent while its claim names the tree", in the same file, is this one level up, where the reach shortfall is in what the walk is rooted at.

## A test that edits the state it is watching before the watch has taken its baseline

**Wrong:** driving a change checkpoint from a test that makes the change first. On dinah-515 four tests in `internal/lsp/live_test.go` opened a document, edited the workbench, and then drove one turn of the poll loop, expecting the loop to report what had been edited. The loop's first walk mints a cursor and reports nothing, which is what a first checkpoint is specified to do, and that walk starts on a goroutine the `initialize` handler launches. So which happened first, the mint or the edit, was a race between that goroutine and the test. When the goroutine won, everything behaved as the tests assumed. When the test won, the edit was folded into the baseline and never reported, and the tick driven afterwards found nothing changed. All four passed on Windows and on macOS through a whole implementation cycle and a whole review cycle, and the ubuntu job went red on one of them with "the server sent 0 dinah/annotationsChanged, wanted 1" only after unrelated tests were added ahead of them in the same package. Nothing about the test's own source says an order is being assumed: every line is sequential, and the thing running out of order is not in the file.

**Right:** drive one whole turn of the watch before touching the state, so the baseline is taken at a point the test chose. A named seam is worth more than an inline call here, because the reason belongs in one place rather than at each site: `ticker.baseline(t)` does what `step` does and carries the paragraph explaining what a first checkpoint answers. Arm the fix by making the race a certainty rather than by running the test more times. Delaying the poll loop's first walk by a few hundred milliseconds turns the losing order into the only order, and with the baseline calls removed under that delay three of the four tests reddened, one of them reproducing the continuous-integration message word for word.

**Related:** "An arming claim whose fixture leaves the deciding order to a random identifier", in `convention-counterexamples-1.md`, is the same class pointed the other way. There the falsification depended on an order the fixture did not choose, so the arming evidence was a coin toss; here the passing run depends on an order the test did not choose, so the guard is a coin toss. The shared rule is that a fixture whose outcome turns on an ordering chooses that ordering. The tell peculiar to this direction is concurrency the test never names: a goroutine somebody else's constructor started, a watcher, a background flush. Ask what has already run by the time the first assertion is reached, and where the answer is "it depends", make the test decide.

## A test whose expected value is computed by the code it is meant to be checking

Caught at Test on dinah-515, 2026-09-15, on two of twenty-seven criteria, and a third instance found by the implementer while sweeping the rest of the card for the same shape. All three guarded behaviour that was correct, and all three stayed green against a build where it was not. The corpus already holds "A test asserting the value the write happened to produce, where the write was the defect", which is the same failure reached by hand-writing the wrong expected form. This entry is the mechanical route to it, where nobody wrote an expectation at all.

**Wrong:** a test that reads its expectation out of the production symbol it is asserting about. The completion cap is stated as two hundred in the contract, and `TestCardCompletionIsCappedAndSaysSo` never wrote that number. It grew its fixture with `for len(cards) <= completionCap` and asserted `len(answer.Items) == completionCap`, so both halves moved with the constant. Test changed `completionCap` to 500 and the test passed, obligingly growing the fixture to 501 cards on the way. The same shape reaches the round trip: every location assertion in `internal/lsp` composed its expectation with `fileURI` and read the reply back with `uriPath`, so a pair that was wrong and mutually inverse, one dropping the scheme or mangling a drive letter, left them all green while no link the editor received could be opened. A build with both halves broken was planted and `TestTheFourStandardHandlersAnswerContent` passed against it.

**Right:** write the number, or the string, that the contract states. Where the production symbol should equal it, pin that separately, in its own assertion with its own message, and let the behaviour assertions run against the test's own value rather than the code's. On dinah-515 that meant `const contractCap = 200` beside a check that `completionCap` equals it, with the three fixture assertions counting against `contractCap`. For the round trip it meant one test that names neither helper on the expected side and spells the composed URI out, `file:///tmp/a%20b/card.md` for a path with a space in it, with the reading half given literal URIs rather than the output of the composing half.

**The test:** for every assertion, ask whether it would still fail if the behaviour it names were wrong but the code it reads were self-consistent. A count compared against `len(fixture)` where the fixture is the test's own hand-written table passes that question, because nothing under test wrote the table. A count compared against a production constant fails it. A value composed by a helper from the package under test fails it, and it fails it twice over where the same package also reads the value back, since an inverse pair of mistakes cancels. The tell is that the expected side of the comparison names a symbol the diff could change.

**Related:** the workbench discipline "Where a check sweeps a set, assert how big the set was", whose remedy this entry borrows. A set the test itself writes still wants its size pinned, because a later edit that drops a row moves the expectation with it. Two further instances on the same card took that one-line fix, one on a list of five annotated kinds and one on a four-row quiet half.

## A guard that parses the construct and then matches an identifier's name inside it

Caught at Agent Code Review on dinah-515, 2026-09-15, in the repair of an
earlier finding rather than in the shipped behaviour. The corpus already holds
"A guard that reads source text reads one spelling of the thing it is
guarding", whose remedy is to parse the construct instead of matching text.
This entry is where that remedy is followed one step and stopped: the guard
parses, walks a real syntax tree, reaches the right node, and then asks what
the node's identifier is called. Everything a compiler would answer about that
identifier goes unasked, so the guard covers the spelling its author had in
front of him and nothing else. The failure is harder to see than the text one,
because the code around it is structural and reads as though the question had
been settled.

**Wrong:** a sweep that enters its check only for two literal identifier
names. `TestNoStringLiteralReachesTheEditorsOwnChannels` in `internal/lsp`
walked every `conn.notify` call in the package and tested the method argument
with `call.Args[0].(*ast.Ident)`, admitting it only when `Name` read
`methodLogMessage` or `methodShowMessage`. Any other spelling returned without
a report, and, worse, without incrementing the count, so the sweep-size
assertion did not catch it either. Two routes were written against the shipped
code and both compiled clean and both passed the guard. The first writes the
method as the protocol URI and assembles the payload field by field:

```go
func (s *Server) hostileA() error {
	var payload logMessageParams
	payload.Type = messageTypeWarning
	payload.Message = "the walk is running slowly and nobody translated this"
	return s.conn.notify("window/logMessage", payload)
}
```

The second holds the method in a package-level variable and returns the
payload from a helper, so no composite literal is written anywhere:

```go
var aliasMethod = methodShowMessage

func (s *Server) hostileB() error { return s.conn.notify(aliasMethod, s.hostilePayload()) }
```

An untranslated English sentence reached `window/logMessage` under the first
and `window/showMessage` under the second, and the guard reported success both
times. The same file held the miniature: a helper answering whether an
expression came from the message catalogue accepted any call through a
selector named `T`, so any type at all could satisfy it by declaring a method
of that name.

**Right:** type-check the package and ask what each expression denotes.
`go/types` is in the standard library and a package's own sources can be
checked with `importer.ForCompiler(fileset, "source", nil)`, which cost 2.7
seconds here on ten files. The method argument is then read with
`info.Types[expr].Value`, which folds an identifier, a local `const` alias, a
concatenation of constants and the literal URI to one string, and the two
person-facing methods are resolved once at their own declaration through
`pkg.Scope().Lookup("methodLogMessage")` so that every call site is compared
by value. The catalogue helper resolves through `info.Selections`, comparing
the selected object's `FullName()` against `(*dinah/internal/msg.Renderer).T`,
which a lookalike method on another type cannot satisfy. The composite literal
is recognised by `types.Identical` against the payload type rather than by the
type name written at the site, and its Message member is found either by a key
whose `info.Uses` entry is the field itself or, where the literal is
positional, by the field's own index in the struct.

**Where the repaired guard still cannot see, said in the guard:** a method
argument the compiler cannot fold to a constant, which is what a variable
holding the method gives, and a payload that is not a composite literal at the
call site, which is what assembling one field by field gives. Neither is
resolved and neither is passed over. Each is reported with its file and line,
in a message saying that the scan cannot read it, and the doc comment says
which two shapes those are and that what the guard proves about them is that
they exist rather than what they say. That is the workbench rule about saying
where a guard cannot see, met at the place somebody meets the guard.

**The test:** having parsed the construct, read every comparison the guard
makes inside it and ask of each whether it compares a name or a value. A
comparison against a string that happens to be a Go identifier is the tell,
and it is a tell whether the identifier belongs to a constant, a type, a field
or a method. Then arm the repair against spellings nobody has used yet, and
three are enough to separate a fix from a widening: the shape the reviewer
wrote, a second shape that reaches the same channel by a different route, and
one the repair was not designed around. The third is what settles it. Here it
was a local `const chatter = "window/" + "showMessage"` handed a payload a
helper assembled into a variable, and the name-matching guard answered `ok`
against it while the resolving guard folded the concatenation to
`window/showMessage` and named the call, the unreadable payload and both
counts. A fourth spelling is worth recording because it failed in a way that
looks like success: a positional `logMessageParams{messageTypeWarning, "..."}`
did redden the name-matching guard, but on `builds a message payload carrying
no Message member`, which is the wrong defect, since the member was there and
the guard could only read keyed elements. A guard reporting the wrong reason
is not evidence that it saw the thing. Pin the accepting case beside the
refusing ones, since a guard that refuses everything passes a criterion about
refusal. A legitimate catalogue call reached through a local variable holding
the renderer still passes the repaired guard, with only the sweep-size
assertions firing on the extra notification.

**Related:** "A guard that reads source text reads one spelling of the thing
it is guarding", in `convention-counterexamples-2.md`, is this entry one level
down, and the two together say that parsing is the direction of the remedy
rather than the whole of it. "A source guard matching the spelling of the
lookup that is there, saying nothing about the lookup that is not" is the
neighbour on exclusivity, and its bounds paragraph is the model for the
saying-where-it-cannot-see clause above. The workbench discipline "A guard
widened by example fits only its examples" is what the third spelling is for.
## A digest taken over the bytes handed in, where the writer normalises on the way to disk

Caught at Implement on dinah-525, 2026-09-20, by the newline suite rather than by any test the card added. The feature is a SHA-256 a comment's anchor carries over its own body, so that a hand edit is detectable. The first store the tool wrote after it reported a divergence in a comment nobody had touched.

**Wrong:** hashing the caller's string. `StampCommentDigest(fm, body)` took the body a verb was given and recorded `sha256(body)` on the header. Three transformations then stood between that string and what a reader gets back. `Frontmatter.Render` settles the trailing newline; `WriteText` reduces every run of carriage returns that ends at a line feed, which is dinah-514's own normalisation; and `ParseAnchor` is the door every reader comes through. So a comment whose text carried `before\r\r\r\nafter` was hashed as the author typed it and read back as `before\nafter`, and `dinah check` reported a comment the tool had written one second earlier as edited by something outside the tool. The feature's first act was a false accusation.

**Right:** hash what the reader will get. `_, stored := ParseAnchor(NormalizeNewlines(fm.Render(body)))` composes the file the write is about to make, parses it back through the reader's own door, and records the digest over that. It costs one render and one parse per comment write, which is nothing beside the file write already happening, and it makes the agreement a property of the code rather than a claim about three functions staying in step. The stamping is then one function every writer of that anchor goes through, so "recomputed by every verb that writes the anchor" is true of one place instead of being a rule each call site has to remember.

**The test:** wherever a value is stored beside the bytes it describes, ask whether the writer between them is the identity. A digest, a length, a line count and a checksum are all this shape. The tell is a normalising writer: any `Write` that trims, re-renders, re-encodes or settles a line ending is a transformation the stored value has to be taken after rather than before. Arm it with an input the normalisation actually changes, which for a line-ending rule means a run rather than a single pair, since a writer that reduces one pair per pass and a writer that reduces the whole run agree on `\r\n` and part company on `\r\r\n`.

**Related:** "A test asserting the value the write happened to produce, where the write was the defect", in `convention-counterexamples-1.md`, is the reading-side neighbour. There a test took the writer's output as the expectation; here the writer took its own input as the record. The shared rule is that the value of record is the one a reader can reproduce.

## A reference composed for one resolver and handed to another that resolves differently

Caught at Implement on dinah-525, 2026-09-20, by the card's own delete test, which went red on its first run with `unknown-card` where it expected a deletion to land.

**Wrong:** handing a verb an entity's bare identifier because the identifier is what the code had. Deleting a comment a checklist item designates is refused, and `--force` reopens that item as part of the same act. The forced path found the item by climbing out of the comment's directory, so what it held was the item's 12-hex identifier and its directory, and it routed a reopen with `routed.Ref = designator.ID`. `Reopen` resolves what it is given through the reference grammar, and an item's bare identifier is not a reference that grammar answers: it resolves a card's, a column's and a workstream's, and an item is reached through the card that holds it. Every forced deletion came back `unknown-card`, having already removed the comment, so the item was left settled with its answer gone.

**Right:** compose the reference the resolver reads, and share the composer with whoever else composes one. `itemCanonicalRef(card, itemID)` walks the card's checklist, counts the kind positions the reader counts, and answers `<card>/<kind word>/<n>`. Two callers now use it: the settling that stores a designation, and the forced deletion that hands one to `Reopen`. Sharing it is not tidiness. The two would otherwise count the same positions twice and could disagree, and a designation that disagreed with the reference a reader types is a stored value nobody can follow.

**The test:** for every value routed from one verb to another, ask which resolver reads it at the far end and whether the near end composed it for that resolver. An identifier, a path and a reference all read as strings and all look interchangeable at the call site. The tell is a field named `ID` or `Dir` being assigned to a field named `Ref`, or the reverse. Where a kind is reached only through what holds it, a bare identifier is not a reference for it, whatever the resolver does for other kinds. Arm it by driving the far verb rather than by reading the near one: the composer looked right in isolation and the route it fed was refused on the first invocation.

**Related:** the neighbour is "A guard that reads source text reads one spelling of the thing it is guarding", in `convention-counterexamples-2.md`, one layer up: there a guard matched one spelling of a name, here a resolver answered one spelling of an address. The shared rule is that a string travelling between two layers carries no type, so the layer that composes it owes the layer that reads it the spelling that layer reads.
## A repair that stamps its own version by equality, where the version is a floor

Caught at Implement on dinah-525, 2026-09-20, on the second review cycle, by three separate tests going red after a gate started refusing stores below the current storage format. Three repairs carried the shape, all written when the format each stamped was the newest there was.

**Wrong:** `if anchor.format != MyFormat { anchor.format = MyFormat }`. The container migration, the card-number migration and the branch migration each end by recording on the workbench anchor that the store has crossed them. Each wrote that as an inequality, which was indistinguishable from a floor on the day it was written, because no higher format existed. Four formats later the same line reads a store at format 6, finds 6 is not 2, and stamps it back down to 2. The repair that only ever meant to raise a store lowers it, and the store is then refused by every reader whose gate is keyed on that number. Nothing about the line looks wrong: it is guarded, it is idempotent against its own value, and it was correct for years.

**Right:** `if anchor.format < MyFormat`. A version stamp is a high-water mark, so the comparison that expresses it is an ordering and not an equality. The fix is one character per site and the reasoning belongs in the comment beside it, because the next reader will otherwise see a guard that looks equivalent to the one it replaced. Where a repair really does need to set a version rather than raise it, that is a different act and deserves a different name: a downgrade is not a migration.

**The test:** for every write of a version, a format number, a schema revision or a generation counter, ask what the line does to a value higher than the one it writes. An equality guard answers "lowers it", and that answer is usually the one nobody intended. The shape hides while the constant is the newest one, so grepping for the constant finds nothing suspicious; grep instead for the pattern of comparing a stored version against a constant with `!=` or `==`, and read each hit against a store that has gone further. Arm it by planting a store at a later version and running the repair, which is a one-line fixture and is what three of this repository's own migrations had never been given.

**Related:** the workbench discipline about a gate that refuses the way out of itself, which this card met in the same hour: widening a read gate to refuse every store below the current format locked out the repairs that carry a store up to it, and the repairs then had to open through a reader that skips that one gate. The two are the same lesson about version numbers at opposite ends. A gate keyed on a version has to let the things that change the version through, and a repair that changes the version has to move it the way the gate expects.

## A script that writes text files in Python's text mode on Windows, silently converting the line endings it did not mean to touch

Caught at Implement on dinah-541, 2026-09-21, by `git add` printing a CRLF-normalisation warning on five files a script had just edited, none of which the script's own diff was supposed to touch beyond a few inserted lines.

**Wrong:** `open(path, "w", encoding="utf-8").write(content)` where `content` was read back out of the same file with `open(path, encoding="utf-8").read()` a moment earlier. Five locale catalogs in this repository are stored LF-only, as the repository as a whole is. A short Python program read each file, inserted a few new catalog entries by string replacement, and wrote the result back the same way. Python's text-mode write on Windows translates every `\n` in the string to `os.linesep`, which is `\r\n`, so every one of the roughly 5,600 lines in each file, not just the handful the script touched, came back CRLF. `go test` never noticed, because its JSON parser and its catalog-completeness checks read bytes and care nothing about which line ending separates them. The only thing that noticed was `git add`, printing a normalisation notice that reads as routine on a repository that does not, in fact, normalise, and which nothing downstream of it enforces as a build gate.

**Right:** open for write with `newline=""`, or write bytes rather than text, so the line endings already in the string pass through unchanged: `open(path, "w", encoding="utf-8", newline="").write(content)`. Where the mistake has already landed, the fix is not to hand-edit the string but to normalise the bytes on disk directly, `data.replace(b"\r\n", b"\n")`, and diff the result against the same file on the trunk to confirm nothing but the intended lines actually changed.

**The test:** after any script writes a text file it also read, on any platform, compare the file's line-ending byte counts before and after: `data.count(b"\r\n")` against `data.count(b"\n") - data.count(b"\r\n")`. A file that was LF-only and comes back with a nonzero CRLF count was silently reflowed by the write, whatever the tool's own functional tests say, because none of them read bytes at that grain. Running this check against a fixed file created on POSIX, then edited on Windows, is what caught this one: the sibling catalogs edited with a line-oriented `Edit` tool rather than a whole-file Python rewrite stayed LF throughout, which is what made the five Python-written files stand out under the same check.

**Related:** the workbench's own house-style scan runs a control string through its matcher before trusting a clean result, for the same reason: a check that can pass without ever having looked is worse than no check, and `git add`'s own CRLF notice is exactly that kind of near-miss, easy to read as boilerplate until the byte counts are actually compared.

## An allowlist keyed on set membership, widened from one justified exception to a whole category

Caught at Agent Code Review on dinah-540, 2026-09-21. `TestPerCommandHelpFollowsTheProfile` holds a verb's rendered check-order table against the profile document, and it lets a row past that the profile does not declare for that verb only when the row's own reasoning is sound: a layer's own refusal, prefixed so a reader can tell it apart, or no-owner, the one refusal CORE-VERB-2 fixes for every verb rather than one. The card that needed the second exception wrote it as set membership instead of as the one name it was actually arguing for.

**Wrong:** `declaredElsewhere[check.Refusal]`, built by collecting every refusal name the profile declares on any contract verb's own list, then read as `!strings.HasPrefix(check.Refusal, contract.LayerPrefix) && !declaredElsewhere[check.Refusal]`. The decision note beside it claimed the widened rule "still rejects a genuinely fabricated profile-shaped row", and for the row the card actually added, no-owner inserted into move's, release's and unblock's own lists, that reads true: no-owner is declared on claim's and block's own lists, so it is in the set, and nothing else changed. What the set actually admits is any of the five verbs' own refusals standing anywhere in any other verb's table. Agent Code Review inserted `{Refusal: contract.NotHolder, Key: "check.claim.hostile"}` into Claim's own list, between UnknownCard and NoOwner, not-holder being release's own profile refusal and answering nothing canClaim checks at that position, and the widened test passed it, because not-holder sits in the set the same way no-owner does.

**Right:** name the one refusal the exception is for. `check.Refusal != contract.NoOwner` in place of the membership test, with the doc comment saying why that one name and no other is universal (CORE-VERB-2, quoted correctly wherever the profile itself declares a row for it, and run ahead of every one of the five verbs' own lists independent of whether the table said so). A fabricated row carrying any other verb's own refusal is refused by name now, wherever in the table it lands.

**The test:** where a rule's justification is "this one case is different, and here is why," write the rule as a comparison against that one case rather than as membership in the smallest existing set that happens to contain it. A set built by unioning several verbs' own declarations looks principled and is not the argument that was made; the argument was about one shared, profile-fixed name, and a set is a wider claim than that argument supports. Arm it the way the reviewer did: plant a value the set admits and the argument does not, in the position and under the name an attacker would actually reach for, and watch the guard wave it through before narrowing it to the name.

**Related:** "A rule over a set, tested against the members its author had in mind" is the sibling failure on the test side of this same shape: there the population a test checks against is the one its author enumerated by hand; here the population a guard *admits* is the one its author's own example happened to sit inside. The workbench discipline "A guard widened by example fits only its examples" names both.
