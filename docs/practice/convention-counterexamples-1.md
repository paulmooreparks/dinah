# Convention counterexamples 1

This is part 1 of 2, and it holds the oldest entries in this collection. The entries continue in "Convention counterexamples 2".

This document collects wrong/right pairs this board has paid for: a convention that was violated, what the violation looked like, and what the correct form is. Reviewers read it before ruling on a spec or a diff, and grow it when a review catches a mistake worth never repeating.

It is empty because the board is new. The first entries will come from the first reviews. When adding one, state the wrong form and the right form concretely enough that a reader who was not present can apply the rule, and leave out incident dates and story detail; the rule is the payload.

One entry is inherited from the parent project rather than paid for here, because it governs this board's whole reason to exist:

## Software vocabulary in shared protocol text

**Wrong:** writing repository, branch, commit, build, or deploy vocabulary into text that ships in the protocol core or any layer a non-software workbench cannot decline.

**Right:** the protocol core speaks only in cards, states, claims, owners, and queues. Software vocabulary lives in a declinable domain layer. The test: would the sentence still make sense on a wedding-planning workbench? If not, it does not belong in the core.

## Internal board references in public-facing text

**Wrong:** ending a public README's status section with "That work starts with a later card", or citing bare card ids such as `yokoten-2` and `andon-734` in shipped source comments, config strings, and documentation. The reader of a public repository has no access to the board those identifiers name and cannot resolve them.

**Right:** public-facing prose is self-contained. Say "Protocol commands come in a later change" instead of naming a card. Where a source comment genuinely needs to record provenance, spell out what the reference is for rather than leaving a bare id, and keep board ids in commit messages and the board itself, which is where they resolve.

The test: could a stranger who cloned the repository act on the sentence without asking anyone a question? If not, the sentence is written to the board rather than to the reader.

## Documented toolchain behavior nobody ran

**Wrong:** describing what a build command produces from recollection of the toolchain's rules, for example telling a reader that `go build -o yokoten ./cmd/yokoten` writes `yokoten.exe` on Windows. Go appends the platform executable suffix only when it picks the output name itself; an explicit `-o` path is used verbatim, so that command writes a file named `yokoten` on every platform.

**Right:** run the command on the platform the sentence names, then write down what the filesystem actually shows. When a document states a filename, an exit code, or an output location, that statement is a claim about observed behavior and gets verified the same way a test does. If it cannot be verified on the platform in question, say which platform was checked instead of generalizing.

The test: is there a command whose output I could paste to support this sentence? If not, the sentence is a recollection wearing documentation's clothes.

## A case-preserving rename of a proper name

**Wrong:** renaming a lowercase code name to a proper name with a mechanical substitution, so every mid-sentence occurrence keeps the old orthography while sentence-initial ones come out capitalized. The tell is two spellings in adjacent sentences: "the state that makes something Andoneer rather than dinah. Dinah staying all-text is the marker...". Headings inherit it too ("# The dinah on-disk format"), and so do user-visible strings the substitution touched.

**Right:** decide the new name's orthography before the substitution runs, then apply it by case rather than by string. A tool whose name is a word (git, npm, yokoten) is lowercase in prose and in the command; a tool whose name is a proper name is capitalized in prose and lowercase only where it names the executable or a verb invocation. Sweep the result for the surviving old case rather than for the old spelling, because a case-preserving substitution leaves no instance of the old word to grep for.

The test: does one grep of the new name return two spellings in the same file? If it does, the rename is half-applied, and the half that is wrong is the half that was not at the start of a sentence.

## A normative statement whose exception lives only in prose

**Wrong:** writing an unconditional requirement and putting its carve-out somewhere the extractor cannot see. `[CORE-MOVE-4] A tool MUST refuse a move into a state whose count of cards has reached that state's declared capacity limit` next to `[CORE-MOVE-9] A tool MAY let the operator move a card into a state that has reached its capacity limit`, with the reconciliation carried only by the surrounding paragraph and the walkthrough. A suite derived from the extracted list sees two statements that contradict each other, and a tool cannot satisfy both.

**Right:** every exception a statement tolerates is written into that statement's own text, because the extracted line is the whole of what a conformance check receives. Either bound the requirement ("MUST refuse ... unless the owner asking is the operator") or state the exception as a separate statement the first one names. Prose around a statement explains it; it never modifies it.

The test: read each extracted line alone, with no surrounding paragraph. If two lines then demand opposite behaviour of the same act, the reconciliation is in the wrong place.

## A checker its own document can switch off

**Wrong:** writing a mechanical check whose scope depends on how the text it reads happens to be formatted. Two shapes of this were paid for on one document. A banned-phrase check built `\b<term>\b` against raw text, so a two-word term matched only when both words landed on the same line and a paragraph wrapping between them silently exempted it. A keyword check skipped fenced blocks by toggling a flag on every fence marker, so a single unclosed fence turned the check off from that line to the end of the file and everything after it read clean.

**Right:** normalize before matching, and treat the shape the check depends on as something the check itself verifies. Collapse whitespace, including line breaks, before matching a multi-word term. Where a checker honours an exemption region, make an unterminated region a reported defect in its own right rather than an open door. A green result then means the check ran, which is the only thing a green result is worth.

The test: can I make this check pass by reformatting the document without changing a word of its meaning? If I can, the formatting is deciding whether the rule applies, and the rule is not being enforced.


## Two mandatory refusals and one slot to report them in

**Wrong:** writing each refusal as its own unconditional MUST while the outcome format admits exactly one refusal name, so that a request satisfying two of them has no defined answer. `A tool MUST refuse a move into a state that has reached its capacity limit, reporting `at-capacity`` beside `A tool MUST refuse a request carrying an override marker from an owner that is not the operator, reporting `not-operator``: a non-operator marking an override on a move into a full state trips both, and a suite deriving an expected name from each statement alone fails whichever implementation it meets.

**Right:** decide, once and in normative text, what happens when several refusals apply. Either the verb's preconditions are declared to be evaluated in a stated order, so the first one that fails names the refusal, or the outcome rule says the reported name is any one of those the applicable statements declare and the suite accepts any of them. Adding a refusal to a verb then means placing it in that order rather than adding an isolated MUST.

The test: for each pair of mandatory refusals on one verb, can I write a request that satisfies both? If I can, and the document does not say which name comes back, the two statements are a contradiction waiting for a second implementer.


## A style fix that quietly edits the meaning

**Wrong:** removing a prose tell by deleting or paraphrasing the words that carry the claim. Collapsing a colon-splice by dropping its head clause, so "Dinah coordinates work: it knows which card is where" becomes "Dinah knows which card is where" and the positive role assertion is gone. Rewriting for an active register and losing a qualifier on the way, so "the harness around the agents is deliberately somebody else's job" becomes "that work belongs to whatever harness you already use" and the design-intent marker disappears. Dropping an adverb that names the practice being defined, so "deploying a practice proven in one place horizontally to everywhere it applies" loses "horizontally" and the definition no longer describes horizontal deployment.

**Right:** fix the tell with punctuation and word order before reaching for deletion. A colon-splice becomes a period, which leaves both clauses standing. A balanced aphorism gets its concrete noun back rather than an abstract stand-in. Then diff the rewritten passage word by word against the original and account for every changed word as style-safe or as a meaning change. Qualifiers, intensifiers, negations, and the head clause of a spliced sentence are the words most likely to vanish unnoticed, because they are the words the tell was attached to.

The test: read the rewrite alone and list the claims it makes, then do the same for the original. If the original's list is longer, or any item on it is weaker in the rewrite, the style fix carried a claim out with it.


## A rewrite that adds a claim the original never made

**Wrong:** restructuring a sentence for style and introducing a small connective word that asserts something new. Turning "Two asymmetries the notation makes visible:" into "The notation makes two more asymmetries visible." adds "more", which claims both items are additional to an earlier enumeration when one of them is the same rule restated in another form. Words like "more", "also", "another", "again", and "still" read as glue, so an eye checking for lost qualifiers slides right over them.

**Right:** treat every added word with the same suspicion as every deleted one. The word-frequency accounting already catches this shape mechanically: an added word that traces to no named transformation is a meaning change by definition, so run the accounting in both directions and refuse to wave any residue through as glue.

The test: for each word the rewrite adds, state the claim that word makes about the surrounding text. If the original makes no such claim, the addition is new meaning wearing style's clothes.

## A shipped flag the ratified surface does not name

**Wrong:** implementing a command flag that appears in neither the ratified help block nor the spec's command prose nor the command's own generated help, and then documenting it in a shipped guide. The surface fixture counted commands rather than flags, so the flag rode through every test: the binary accepted two undeclared `init` flags, the generated `help init` showed neither, and the getting-started guide taught one of them.

**Right:** the ratified surface names every flag a command accepts, and the argument parser's accepted set is tested against the ratified surface per command. A guide teaches only what the ratified block admits. The test: diff the parser's flag whitelist per command against the spec's syntax lines; a flag on one side only is a leak, whichever direction it leaks.

## A genuine transcript of output the tool does not order deterministically

**Wrong:** capturing real output once, then writing the guarantee that output happens to illustrate. A guide printed `dinah show rel-1/comments/1` and taught from it that "`rel-1/comments/1` is the first comment on `rel-1`". The listing sorts on a second-granularity timestamp and breaks ties on the random twelve-hex identifier, so two comments written in the same second come back in either order. Re-running the guide's own session returned the second comment under position 1. The transcript was honest and the sentence next to it was not.

**Right:** before a transcript is used to teach a rule, read the code that produced the ordering, the count, or the selection, and confirm the rule holds for every input rather than for the one that was run. Where the tie-break is arbitrary, either say so, or fix the tool so the guarantee is real and document the fixed behaviour. A guide that states a guarantee is making a claim about all runs, and a single capture is evidence about one.

The test: if I threw this workbench away and rebuilt it from the same commands, would the block print the same bytes? If it would not, the sentence beside it may not promise that it will.

## A shell-specific path in a document that never names the shell

**Wrong:** pasting transcripts from the shell the author happened to be sitting in when the path form is that shell's rather than the tool's. A Windows-flavoured guide showed `C:\work\release-notes` everywhere else and then wrote `dinah --bench /c/work/release-notes`, plus `extract /c/work/release-template` and `init --from /c/work/release-template`. Those run only where the shell rewrites `/c/...` into a drive path on its way to the process. In PowerShell and cmd the same argument reaches the tool as `C:\c\work\release-notes` and the command refuses. The same document taught PowerShell composition a few sections earlier, so the reader it broke was a reader it had already addressed.

**Right:** write paths in the form the tool receives, and keep one form throughout a document unless the document says which shell each block assumes. When a transcript is genuinely shell-specific, label the shell in the surrounding sentence, the way a POSIX-only pipe or a PowerShell-only call already has to be labelled.

The test: would this line work if the reader pasted it into every shell the document elsewhere assumes they might use? If not, either the line changes or the document says which shell it is for.

## A walkthrough that changes directory and never changes back

**Wrong:** removing absolute paths from a guide by making the session depend on the working directory, then moving the working directory mid-document to demonstrate something and leaving the reader there. A quick start restructured to be shell-neutral had every command rely on the tool discovering its workbench by walking up from wherever the reader stood. One section demonstrated addressing a workbench from outside it, opened with `cd ..`, and the next section resumed issuing bare commands that require standing inside. Every remaining section refused. The blocks were all real output, captured by an author whose own shell was in the right place.

**Right:** when a document's correctness depends on state the reader carries between blocks, the document owns every change to that state in both directions. A section that steps out steps back before the next one starts, and the step back is a visible command line rather than an assumption. Verify by executing the document's command lines in order, from the top, in a fresh directory, taking no step the document does not print.

The test: could a reader who types only what is printed, in order, reach the last block? If a step depends on something the reader was never told to do, the document has a gap where its own author had context.

## A style tell removed by swapping its connective

**Wrong:** answering a finding about the trailing gloss, where a statement is followed by a comma and an explanation of itself, by changing the word that introduces the gloss. `, which is how you tell the two layers apart` became `, and that is how you tell the two layers apart`, and `, which makes it the command worth running` became `, and it is the command worth running`. A search for the flagged word comes back clean and the shape the finding named is still on the page.

**Right:** fix the shape rather than the search term. A gloss goes away when its content becomes a sentence of its own, when it moves into the clause it explains, or when it turns out to be restating the sentence it hangs off and can go. Where a standard lists sanctioned transformations, use one of those; a connective swap that preserves the structure is not on that list because it does not change the structure.

The test: read the sentence aloud and ask whether it still ends by explaining itself. If it does, the tell survived whatever word now introduces it.

## A prose tell hunted one instance at a time when it is a document-wide cadence

**Wrong:** treating a sentence-shape tell as a list of locations to fix. A review flagged seven trailing which-clause glosses by line number, the next pass rewrote those seven, and a later grep for `which` came back clean while the document still read as machine prose. The structure had not gone anywhere. It had spread: forty-one percent of the document's prose sentences ended as a main clause, a comma, and a consequence introduced by `so` or `and`, and the contrastive connective `but` appeared zero times in a hundred and thirty-eight sentences. Each individual sentence was defensible and every named tell searched clean.

**Right:** when a finding names a sentence shape rather than a word, count the shape across the whole document before and after the fix, and report the frequency rather than the instances. Strip fenced blocks and inline code spans, split the remaining prose into sentences, and measure: how many carry the flagged structure, what the distribution of connectives is, how many sentences open with the same word, and what the standard deviation of words per sentence is against its mean. A rewrite that moves the instance count to zero while leaving the frequency where it was has relocated the shape, not removed it.

The test: if I deleted the flagged lines entirely, would the document's shape distribution change? If it would barely move, the flagged lines were symptoms and the finding is about cadence, so the fix has to be measured in the aggregate or it will come back wearing a different connective.


## A cadence fix that pays for variety with the causal link

**Wrong:** breaking up a document-wide statement-then-consequence cadence by splitting the sentence at its connective and dropping the connective on the floor. `Dinah never translates the machine spellings, so the same command emits the same bytes under any language setting` becomes two sentences with nothing between them, and `whoami answers two questions at once, because the second one governs what you are allowed to do` loses the `because` and leaves the reader two adjacent facts to join for themselves. The stride measurement improves, the flagged shape count falls, and every individual claim is still on the page, so both the metric and a word-frequency check pass. What left the page is the relationship between the claims, and the prose standard counts a causal relationship as meaning rather than as style.

**Right:** carry the causation across the split instead of deleting it. A trailing `for that reason`, a leading `Because`, an imperative-and-consequence pair, or a rewrite that puts the cause in the subject all vary the shape without spending the link. Where the sentences genuinely sit in bare sequence and the reader loses nothing, say so in the pass notes rather than leaving it for a reviewer to work out, because a deleted connective and a deliberate juxtaposition look identical in a diff.

**The test:** read the two resulting sentences and ask what the second one has to do with the first. If the honest answer is "it is the reason for it" or "it is what follows from it", and no word on the page says so, the variety was bought with a claim.



## A repair verb that abandons its walk and its account

**Wrong:** writing a workbench-wide repair that returns on the first obstacle it meets, and handing its caller an error in place of the report it had already built. A one-time backfill walked every card, stamped what it could, and on the first entity it could not write handed back an error; the caller answered with the error alone, so the count of what had been stamped and the names of the entities the repair could only guess at went with it. The workbench was left half repaired, the guesses were never named, and a second run stamped nothing and reported nothing, because a repair that skips what is already done has nothing left to say about it.

**Right:** a repair that walks a collection reports what it did before it reports what stopped it. An obstacle on one member is a finding against that member and the walk carries on, the same way a lock on one card is. Where an error genuinely has to end the run, the account of the work already done travels out beside it rather than being replaced by it, because that account is the only record of a judgement the repair will never make a second time.

**The test:** interrupt the repair partway and ask what the operator can still learn about the part that finished. If the answer is nothing, and a second run answers "nothing to do", then the report was tied to the happy path and the work already done is now indistinguishable from work nobody did.



## A spliced fragment whose pronoun has no referent on one of its branches

**Wrong:** writing a message fragment that is appended conditionally, and giving it a pronoun or an article that only resolves for the case the author had in mind. A refusal spliced its repair fragment `; hand-edit the file and add it, then run `dinah check` to confirm` onto a base sentence `{detail} is missing, empty, or will not parse, in {path}`. For a missing field the pair reads correctly: "profile is missing ... in C:\wb\workbench.md; hand-edit the file and add it". For the case where the file itself will not read, the same code sets `{detail}` to the file's own name, and the reader gets "workbench.md is missing, empty, or will not parse, in C:\wb\workbench.md; hand-edit the file and add it" - the file named twice, and an instruction to hand-edit a file that will not open and to add the file to itself. Both branches were tested and both printed; only one was read as a sentence.

**Right:** enumerate every value the splice's variable can take and read the assembled sentence aloud for each one before shipping the fragment. Where one branch cannot share the wording, give that branch its own fragment rather than stretching one across both, which is the same discipline that already forbids branching inside a catalog template. A pronoun in a spliced fragment is a promise about what the other fragment supplied.

**The test:** list the distinct values of the interpolated variable, assemble the full sentence for each, and ask of every pronoun and definite article which noun it points at. If one assembly points at nothing, or points at the wrong noun, the fragment is written for a single case and is being reused by accident.


## A finding the read path refuses before the checker can report it

**Wrong:** adding a check finding for a corruption the reader already treats as fatal, so the finding, its catalog entries and its repair are all unreachable from the command a person would run. A new stored field was refused as malformed by the open path at every declared version, and the integrity checker gained findings for the same two conditions. Every command opens the workbench first, so a single hand-typed bad value made the workbench refuse to open, the checker never ran, the one-shot repair that would have fixed the rest of the states could not run either, and the refusal named neither the offending entity nor the file. The unit test covering the findings had to build the workbench as an in-memory literal, with a comment saying the open path would refuse the fixture, which is the shape this mistake takes when it is written down rather than noticed.

**Right:** decide, per condition, whether it belongs to the reader or to the checker, and let only one of them own it. A condition the reader refuses needs no finding, and a condition the checker reports has to survive being read. Where a requirement is scoped to a version the workbench declares, scope the reader's refusal the same way the requirement is scoped, so that below that version the checker sees the defect and the repair can reach it. A repair command is the operator's way out, so anything that stops it from opening a workbench is a wall in front of the exit.

**The test:** for each finding the checker can produce, write the defect onto disk by hand and run the command. If the answer is a refusal instead of the finding, the finding is dead code and its catalog entries are strings nobody will ever read.


## Two filesystem paths compared as strings

**Wrong:** deciding whether two paths name the same directory by comparing the strings, with or without a case fold. A discovery boundary that had to recognise one particular directory compared cleaned paths and lowercased them under ASCII rules on Windows. Windows hands out the 8.3 short form of a long user name, so `C:\Users\PAULPA~1` and `C:\Users\Paul Parks` are one directory that the comparison reads as two; an ASCII fold leaves accented letters in their original case, so a home directory spelled with a differently-cased non-ASCII letter misses as well; and macOS mounts a case-insensitive volume by default and reaches its temporary directory through a symlink, both of which the exact comparison on non-Windows targets gets wrong. The boundary then silently does not apply, and a boundary that fails open is worse than one that was never claimed. Tests do not catch it, because a test builds both sides of the comparison from the same string.

**Right:** `os.Stat` both paths and compare with `os.SameFile`, which asks the filesystem for identity rather than guessing at its spelling rules, and handles case folding, short names, symlinks and substituted drives without knowing which of them applied. Fall back to a string comparison only where a stat cannot succeed, such as a path that names no existing directory, and say in the code why that fallback is acceptable there. An ASCII case fold belongs to identifiers and tokens, where determinism across locales is the requirement; it does not belong to paths, where the filesystem already owns the answer.

**The test:** can I write two different strings that name the same directory on any platform this code runs on? Windows short names and macOS case-insensitivity say yes on two of the three, so a string comparison is answering a question only the filesystem can answer.


## A transcript from a workbench the reader does not have

**Wrong:** illustrating a command with output captured from some other workbench, and printing it in the same prompt form the walkthrough uses for the reader's own session. A quick start ran `dinah check` in the reader's clean workbench and got `No structural defects found.`, then two sections later showed a bare `$ dinah check` reporting two defects in that same named workbench, which the reader had never damaged; then another bare `$ dinah check` reporting five defects in a `legacy` workbench the document had not mentioned; then `$ dinah workbenches` listing two home workbenches nine lines after the identical command had answered `no workbench is reachable from here`. Every block was genuine output and the surrounding prose gestured at each situation, but the gesture was a subordinate clause and the block was a command prompt, so a reader following along got three contradictory answers to commands she had just been taught to trust.

**Right:** a block the reader cannot reproduce says so in the sentence that introduces it, names whose workbench it came from, and says what hers will print instead. Reserve the walkthrough's prompt form for the walkthrough. Where the demonstration genuinely needs a different starting state, either build that state with printed commands the reader can type, or mark the block as somebody else's screen.

**The test:** for each transcript, could the reader have produced it by typing only what the document printed before it? Where the answer is no, does the sentence above it tell her so in words she cannot skim past? A conditional verb in the prose ("can hold", "here a card names") does not do that work, because the prompt line under it is unconditional.

The companion failure is the same document promising more than it delivers in the other direction: a walkthrough that names a file in a command without ever having the reader create it. Both are caught by executing the document's command lines in order in a fresh tree and taking no step the document does not print, which is the discipline the working-directory entry above already asks for. Extend that replay to every artifact the commands mention, not only the working directory.


## A help line kept because its check was unchanged, when the behaviour around it changed

**Wrong:** deciding that a plain-English description of a precondition stays true because the code implementing that precondition was not edited. A card moved workbench creation into a `.dinah` container and left `init`'s help intact, reasoning that the refusal it documents (a `workbench.md` sitting directly in the directory) fires on exactly the same condition as before. The code claim was correct and the sentence still read `the directory holds no workbench already`. After the change, a directory whose container already holds a workbench does hold a workbench, and a second `init` there succeeds, so a person reading the help before running the command is told a refusal will happen that will not.

**Right:** re-read every user-facing sentence that describes a condition against what the reader can now observe, not against whether the code behind it moved. A description of a check is a claim about outcomes, so it goes stale whenever the set of inputs reaching that check changes, even when the check itself is byte-identical. Where the description is in a message catalog and rewording it means retranslating, that cost is a reason to raise the wording as a decision rather than a reason to record the sentence as still true.

**The test:** take each condition the help states, construct the case a reader would build from it, and run it. If the tool does something other than what the sentence led you to expect, the sentence is stale however still the code behind it was.


## A republished statement checked only against the statements that name it

**Wrong:** retiring a statement and publishing its replacement, then verifying the change by searching for the retired identifier's name. A versioning rule that put retirement on the major number was retired and republished as a pair conditioned on whether the document's own major number had reached 1, so that before 1 a retirement takes a minor increment. The sweep confirmed that no live statement named either retired identifier, and it was correct: none did. An untouched neighbour still forbade the new behaviour by describing it rather than by naming it, since it read that a minor increment leaves every identifier of the prior revision present in the list. Read alone, the new rule requires a pre-1.0 retirement to be a minor increment and the neighbour requires a minor increment to retain every identifier, so the extracted list demands opposite behaviour of one act. The revision performing the retirement was itself the first instance, and the reconciliation existed only in the changelog prose.

**Right:** when a republished statement moves an operation from one axis, one number, or one category to another, re-read every live statement that constrains the destination, not only those that cite the retired identifier. Identifier citations are found by search; a constraint expressed in ordinary words about the same axis is not, and it is the one that survives the sweep. List the statements governing the destination before writing the replacement, and say for each whether it still holds under the new rule.

**The test:** write down the act the new statement now permits or requires, then read every other extracted line and ask whether any of them forbids that act in its own words. A search for the retired identifier cannot answer this question, because the statement that breaks does not mention it.


## A wording change that outgrows the column it is rendered into

**Wrong:** approving replacement wording for a message catalog by reading the sentences on their own, when some of those sentences are rendered into a fixed-column layout. A vocabulary sweep replaced `the reference resolves to an entity` with `the reference resolves to a card, a state, or something below a card`, which is 68 runes against a 52-rune pad. The padding helper returns the text plus a single space once the text reaches the column, so in the per-command help for two commands the refusal name beside that line moved 16 columns right and ended up separated from the sentence by one space. A newcomer then reads `something below a card dinah.unknown-path` as one phrase, which is the opposite of what a plain-English sweep is for. The main help block has a byte-exact fixture and caught nothing, because the overrun was in a different block that has no fixture.

**Right:** before approving replacement text, measure every replacement against the width of whatever column renders it, in every block that renders it, and check the translations too, since a translation of the same sentence is usually longer. Where the text has to be longer than the column, change the layout in the same card rather than shipping a row that runs its identifier into its prose: widen the column, or put the identifier on its own indented line when the text overruns. A fixture over one block is evidence about that block alone.

**The test:** for each string being lengthened, find every call site that pads or aligns it and compute the rune count against that call site's width. If any string reaches the width, render that block and read it rather than trusting the block that has a fixture.


## A subject-slot fix applied as a substitution rather than heard

**Wrong:** converting every non-compliant sentence's subject to the same actor's name, one sentence at a time, without reading the paragraph that results once the edits stack up. The getting-started guide's subject-slot rewrite converted every mechanism subject to "Dinah" and passed every mechanical check (sentence count, meaning-preservation, fragment check), but one paragraph came out as "Dinah writes... Dinah prints... Dinah never copies...", three sentences in a row with the same opener, and ten of the file's twenty compliant sentences open with the literal word "Dinah". Each edit was individually correct and the document still sounds like a rule being recited.

**Right:** after a subject-slot pass, read the whole document aloud, paragraph by paragraph, and treat two or more consecutive sentences opening with the same subject word as a finding regardless of how many mechanical checks passed. Where a paragraph clusters, vary the opener: give a sentence to "you" where the reader is the one reading, editing, or running the command anyway, or fold two same-subject sentences into one.

**The test:** for each paragraph, list the first word of every sentence. If a subject word repeats twice or more back to back, the pass fixed the rule and skipped the read-through.

## A copy-path fix verified against the command that was named, not the class it belongs to

**Wrong:** correcting an absolute claim about a special-cased behavior ("copies workbench.md") by reading the one command whose name matches the wrong claim, then asserting the sweep is complete. The getting-started guide's false "nothing copies that file anywhere" was rewritten to "you get a copy of it only when you run `dinah extract`" after the fixer read `Bench.Extract` and confirmed it copies the file. `dinah init --from <bench-directory>` (`verb.Init` -> `readSource` -> `bench.Instantiate`) also copies the same file's prose body into a new workbench.md, verified by running it: editing a marker string into a source workbench's `workbench.md` and running `dinah init --from <that bench>` reproduced the marker in the new workbench's `workbench.md`, with `dinah extract` never invoked. The new absolute word ("only") repeated the exact defect class the fix was meant to close.

**Right:** when a claim is about "every command that does X to this file/resource," grep for every call site that reads and rewrites the resource, not just the one command whose name suggests the behavior. Here that meant grepping for the file's own constant (`WorkbenchAnchor`) across the tree and reading every function that both reads and writes it, which surfaces `Instantiate` and `Extract` as siblings under the same read-then-write shape. A claim scoped with "only" needs every producer of that outcome enumerated, not the first one found.

**The test:** for the resource the claim is about, grep its identifying constant or field name across the whole tree, list every function that both reads and writes it, and check the claim against each one individually. If the claim was verified against fewer functions than the grep returns, the "only" is unproven.

## A test's expected path guessed at instead of reproduced

**Wrong:** fixing a test that compares a path literal against what the head resolved by adding a general-purpose normalization step to one side of the comparison. Resolving symlinks before comparing fixes a macOS temporary directory that sits behind one, and breaks a Windows CI runner that hands out an already-short path, because resolving symlinks there expands the short form back to its long one and produces the same mismatch running the other way. Each fix reads correct on the platform it was tested on and wrong on the platform it was not.

**Right:** when a test's expected value has to match a path the head derived through its own working directory (`os.Getwd`, `filepath.Abs`, or an internal resolution step), reproduce that exact call sequence in the test rather than transforming a fixture-supplied string to guess at it. A helper that chdirs into the target directory and calls `os.Getwd()`, the same sequence the head itself runs, agrees with the head on every platform without asserting anything about which platform's quirk is in play.

**The test:** for each side of a path comparison, name the mechanism that produced it. If one side is a literal built by string-joining a fixture value and the other is what the head resolved at runtime, and no step reproducing the head's own mechanism sits between them, the comparison is testing platform spelling rather than behavior.

## A pure-decision test that never proves the guard calls it correctly

**Wrong:** believing a safety guard is tested because its decision logic is tested. `dinah-88` added `TestResolveHomeRefusesAnUnresolvedHome` and `TestResolveHomeRefusesAHomeThatFailsStat`, which drive `resolveHome(home, homeErr, statErr)` against faked error values and assert it refuses correctly. Nothing calls the real `IsolateTempDir()` with a manipulated environment and confirms it computes `homeErr`/`statErr` correctly and actually calls `os.Exit(1)`. A reviewer manually verified (in a throwaway, uncommitted test, since `os.Exit(1)` inside a `go test` subprocess only kills that subprocess and never the invoking shell) that the wiring happens to be correct today, but a regression that swapped the two error arguments, stopped calling `resolveHome`, or dropped the `os.Exit(1)` call would leave both committed tests green.

**Right:** when the defect being fixed is a real function drawing the wrong conclusion from a real input, at least one test has to exercise that real function end to end, not only the pure decision it delegates to. Where the real path ends in `os.Exit`, use the standard subprocess idiom (`exec.Command(os.Args[0], "-test.run=TestHelperProcess")` with an explicit `Cmd.Env`, the same shape `os/exec_test.go` uses in the standard library) rather than treating "it would kill the test binary" as a reason to test only the pure function. That idiom scopes every environment change to the spawned subprocess, so it is safe even when the guard under test exists specifically to protect real data from environment manipulation.

The test: for the specific defect the card reproduces (a real function misreading a real input), is there a test that calls that real function under the misleading input? If every new test instead calls a pure function the real one merely delegates to, the guard's wiring is unverified, however well its logic is.

## A resolved reference printed in one place and left raw in the refusal for the same entity

**Wrong:** fixing a listing that named a destination by its identifier to name it by its alias instead, without checking whether a refusal naming that same destination on the failure path still carries the identifier. dinah-29 cycle 2 added `stateRef` and used it in the legal-moves listing, so `claim`/`move`/`show` print `doing` for a state a person can type back. The at-capacity and terminal-move refusals name the same state by its raw identifier (`internal/verb/mutate.go`'s `contract.Terminal, departure.ID` and `contract.AtCapacity, destination.ID`, and `internal/verb/beyond.go`'s `contract.AtCapacity, named.ID` on `add --state`), so a session can read "doing" in the moves list, attempt the move, and be told `state 9e41685eddd3 has reached its limit` about the same state one command later.

**Right:** when a fix makes one surface resolve an entity to its alias, grep every other surface, including refusal and error paths, that names the same kind of entity, and check each one against the same resolver. A destination a person cannot act on because it refused is exactly the destination they most need to be able to name back to a person or another command; a refusal is not exempt from the reference-resolution rule just because it is on a failure path.

**The test:** for a fix that resolves one kind of reference to its alias in a listing, grep the reference's own identifier field (`.ID`) across the package and read every remaining call site that passes it into a refusal or an error message. If any of them prints the identifier where the listing right above it just printed the alias, the fix stopped at the happy path.

## A raw identifier surviving in a refusal a grep for the reviewer's own three call sites would not find

**Wrong:** treating "grep every refusal call site that passes `.ID`" as complete once it has covered the call sites a prior review named. dinah-29 cycle 3 fixed the three refusals a reviewer had already reproduced (`contract.Terminal`/`contract.AtCapacity` in `internal/verb/mutate.go`, `contract.AtCapacity` in `internal/verb/beyond.go`), all of which pass a `*bench.State` field's `.ID` straight into a refusal. A structural act's `LastState` and `Occupied` refusals (`internal/bench/entity.go`, reached by `archive`/`delete` on a state) carry the same defect through a different shape: the state a person named is resolved once in `ResolveEntity`, its identifier alone survives into `StructuralAct.StateID`, and the refusal composed two calls later has no slug left to read, only the identifier the resolution step already had and discarded. `grep '\.ID'` over the refusal call sites finds the first three; it does not find a refusal built two calls away from the resolution that once held the slug.

**Right:** when a reviewer's finding is "these refusal call sites leak an identifier," read it as "this class of refusal leaks an identifier" and grep the class, not the cited lines: every place a reference is resolved to an entity, follow what happens to the resolved slug through every later call that can refuse over the same entity, not only the calls that already pass `.ID` at the point being read. Where a resolved slug does not survive to the refusal site, carry it forward explicitly (a struct field alongside the identifier it travels with) rather than re-deriving it or accepting the identifier as good enough.

**The test:** for an entity kind whose reference resolves to both an identifier and a human-readable form, list every refusal reachable after that resolution, not only the ones a prior review already named, and check each one for a live slug versus a bare identifier at the point it composes its message.


## Sibling refusals for the same defect worded independently instead of from one template

**Wrong:** writing two refusal messages for two related failure modes of the same underlying condition, each hand-composed rather than built from a shared template, so they carry different amounts of actionable detail. A repair that refuses rather than emptying a workbench's states list, and a filing verb that refuses rather than crashing on that same empty list, shipped in one card: the filing refusal names the concrete file to hand-edit, the repair refusal only says "the workbench's states list" with no path, even though both refusals are raised by code that has the file's path on hand. The gap was scoped deliberately (a shared render path was judged too large a change for the card), but the decision to keep the messages independent is exactly what let them diverge in the same commit that introduced both.

**Right:** when a card mints two or more refusal names for siblings of one condition, draft their catalog text together and check that a reader moving from one to the other gets the same class of guidance (the file to edit, the concrete next command), not just individually complete sentences. Where a shared render path is deliberately out of scope, that is a reason to compare the sibling texts by hand at review time, not a reason to skip the comparison.

**The test:** for each pair of refusals a card adds for the same underlying condition, list what each one tells the reader to do and check whether one gives strictly less than the other. If it does, the messages were written independently rather than from one understanding of what the reader needs.


## A universal escaping claim verified in only some of the shells it will be pasted into

**Wrong:** escaping a value for a copy-pasteable command-line suggestion with one rule and documenting it as correct for bash, cmd.exe, and the tool's own parser, without checking the shell most likely to receive the paste. dinah's `dinah.multiple-words` refusal rebuilds the caller's command with the free text wrapped in double quotes and any embedded quote backslash-escaped. Pasted back, bash and cmd.exe both reproduce the original text exactly (verified against the built binary in all three shells). PowerShell re-quotes the already-escaped sequence on its way to the native process, and for free text containing a literal `"` the argument arrives corrupted: the exact suggestion the tool prints refuses again, with a garbled message, instead of filing the card.

**Right:** when a fix's whole purpose is "here is the exact text to paste," test the paste in every shell a user of the platform is likely to run, not only the shells that happen to share a quoting convention. On Windows that is a POSIX-like shell (bash/WSL), cmd.exe, and PowerShell, and PowerShell's native-command argument re-quoting is its own dialect, not a variant of either of the other two. Where one shell cannot be made to agree, say so in the code comment or the message rather than leaving the gap unstated.

**The test:** for a suggested command line, would pasting it into PowerShell, cmd.exe, and a POSIX shell all reproduce the exact input? If any one of them does not, and nothing marks that gap, "safe to paste" is unproven for that shell.

## A consumer parsing a document its own publisher reformats

**Wrong:** writing a line-oriented text parser for a JSON document produced elsewhere in the same tree, and testing it against a stand-in the parser's author wrote by hand. A release workflow assembled a channel manifest with `printf '    "%s": { "sha256": "%s", "size": %s }'`, one binary per line, then ran the file through `python3 -m json.tool` before uploading it, which is the in-tree idiom for proving the assembly produced valid JSON. `json.tool` expands every nested object onto its own set of lines, so the published document puts each `"sha256"` on a different line from the name it belongs to. The install script read that document with `sed -n 's/.*"NAME"[[:space:]]*:[[:space:]]*{[[:space:]]*"sha256"[[:space:]]*:[[:space:]]*"\([0-9a-fA-F]*\)".*/\1/p'`, which is line-scoped and cannot match across a newline, so the capture came back empty and the script exited with its network-failure message on every run. The local test passed because the stand-in manifest was written in the compact shape the spec printed, which no published manifest ever has. The sibling PowerShell script used `ConvertFrom-Json` and was unaffected, so one platform's consumer worked and the other's could not.

**Right:** when two artifacts in one tree are a publisher and a consumer, take the consumer's fixture from the publisher's own output rather than from the shape the spec illustrates. Run the publisher's assembly steps, including any reformatting, validation, or minification step that touches the bytes after they are composed, and feed the result to the consumer. Where the consumer parses a structured format with a line-oriented tool, treat the format's whitespace freedom as the hazard it is: a document that is valid JSON may be rewritten by any conforming tool at any point in the pipeline without notice, so a parser that depends on the layout is depending on something nobody promised.

**The test:** for each field the consumer extracts, produce the document by running the publisher and check the extraction against that file. If the fixture was written by hand, or copied from the spec, the two sides have never met.


## A normative statement and its published gloss that say different things

**Wrong:** publishing a statement line whose scope differs from the index gloss written for the same identifier, on the assumption that the index test covers them both. The index test compares identifiers and their order; it never reads the gloss against the statement. So a statement reading "except one the changelog marks as retired" can sit beside a gloss reading "unless the changelog marks it retired", one capped at a single item and the other unbounded, and every check passes.

**Right:** read the statement and its gloss as a pair, out loud, before publication, and make the quantifier, the condition and the exception match word for word in scope. Where the two must be worded differently because the gloss is a checkable restatement rather than a copy, say in the gloss what the statement's scope is rather than a scope of the gloss author's own. The statement line is what a conformance check receives, so where they disagree the gloss is the one that lied to the reviewer.


## A same-cycle PATH-readiness check normalized on only one platform

**Wrong:** matching a PATH entry against a target directory with a literal, unnormalized comparison on one script while the sibling script for the same feature normalizes it. `scripts/install.sh`'s PATH-readiness check (`case ":${PATH}:" in *":${install_dir}:"*`) requires the PATH entry to be byte-identical to the install directory, so an entry with a trailing slash (`~/.local/bin/`) is read as "not on PATH" even though the shell's own PATH search treats it the same as the entry without one. `scripts/install.ps1`'s equivalent check, written in the same commit, does normalize (`TrimEnd('\')` plus a case-insensitive `-eq`) because an earlier cycle on this same card already had to solve that problem for the registry-PATH comparison.

**Right:** when two sibling scripts implement the same user-facing check, carry a normalization rule discovered on one platform to the other rather than re-deriving it independently, or note explicitly why the platforms differ. Reproduced by feeding `case ":${PATH}:" in *":/home/user/.local/bin:"*` a PATH containing `/home/user/.local/bin/` (trailing slash): the pattern does not match.

## A new environment input read by the product and neutralized in only the new tests

**Wrong:** teaching the product to read an ambient environment variable, then neutralizing that variable in the tests written for the change while leaving the rest of the suite to inherit whatever the developer's shell exports. A renderer learned to read `COLUMNS`, and the tests written for it set `COLUMNS` explicitly. The suite's older tests, which render the same rows and compare them against byte-exact fixtures, set nothing. `COLUMNS=40 go test ./cmd/dinah/` then fails three tests, two of them untouched by the change, while the same command passes on the trunk and CI stays green because the CI image exports nothing.

**Right:** a new ambient input is neutralized once, in `TestMain` or in whatever helper every test already runs through, so that no test's result depends on the environment the developer happens to be sitting in. Neutralize it for the whole package in the same change that introduces the read, and let the tests that care about the variable set it themselves on top. The test: export the new variable at a plausible value and run the whole suite. A test that fails and is not about the variable is a test whose environment the change forgot to close.



## Sample transcripts must be run, not recalled

Wrong: a README transcript for `certutil -hashfile <file> SHA256` described the hash line as "32 two-digit hex bytes separated by spaces" (dinah-103, cycle 9). That shape is carried over from older certutil's classic output; on current Windows (build 10.0.26100) the command prints one contiguous 64-character lowercase hex string, no spaces. WHAT SHIPPED had verified the tool's *availability* against Microsoft's documented "Applies to" list but not run the command itself before writing the transcript.

Right: any prose that shows a reader what a command's output looks like gets that transcript from an actual run on a real file, on the same platform the reader will be on, not from memory of an older version or a doc page that only confirms the tool exists. Citing a doc settles whether something is available; it does not settle what it prints.

## A conditioned pair that forbids the crossing between its own conditions

**Wrong:** retiring an unconditional rule and republishing it as a "before the crossing" half and an "after the crossing" half, where the before-half's own demand makes the crossing unreachable. A versioning rule that put every breaking change on the major number was split into "before the document's own major number first reaches 1, this change MUST take a minor increment, leaving the major number unchanged" and "once it has reached 1, it MUST take a major increment", and the same split was made for two neighbouring rules. Every route open to a revision written while the number is 0 then ends in "leaving the major number unchanged", so no such revision may set the number to 1, while the surrounding prose promises the crossing will happen in a named entry. Whether the pair works at all turns on a question neither half answers: is the condition read against the revision being published, or against the one it follows? A checker comparing two revisions reads it off the earlier one, which is the reading that makes the crossing illegal.

**Right:** say in the statement text, or in prose the statements point at, which revision the condition is read against, and name the half that governs the revision performing the crossing. Where a document also classifies a revision by which rules were in force when it opened, keep the two questions apart in writing: whether a rule exists is judged at the opening, and which half of a conditioned rule applies is judged on the revision being published.

**The test:** write out the revision that performs the crossing and classify it under each half in turn, before publishing either half. If the answer changes depending on whether the condition is evaluated before or after the revision is applied, the pair is not finished. The revision that introduces a conditioned pair is not this test, because it usually satisfies both halves and passes every check.

## A tidying pass re-run after the constraint pass that bounds it

**Wrong:** running a cosmetic normalizer, a constraint solver, and then the normalizer again, and reasoning about the second run's cost as though it were the first run's cost. A layout routine chose column widths, ran a pass that widens a column by one where a value misses it by one, ran the narrow-window backstop that shrinks columns until the block fits, and then ran the widening pass a second time. The widening loops while any value equals the current width plus one, so once the backstop has pushed a column down to its floor, every value between the floor and the original width is a step the widening climbs back up. On a workbench whose state slugs draw 4, 5, 6, 9, 11, 13 and 15 columns, the backstop's result of 4 was walked back to 6; on one whose slugs draw a contiguous run, it was walked back to the full width and the last column started at display column 39 inside a 30-column window. The decision note recorded the cost as "one display column".

**Right:** when a tidying pass runs after the pass that enforces a bound, either re-check the bound afterwards or prove the tidying cannot cross it. State the bound as something the code or a test asserts rather than as a sentence in a note, since a per-step cost of one says nothing about a loop whose steps chain. Where the two passes genuinely disagree, name which one wins and record the reasoning where the ordering lives.

The test: read the passes in the order they run and ask what the last one may undo. If the answer is "whatever the pass before it decided", the ordering is a decision that needs an assertion behind it, not a comment.


## An invariant asserted from one side only

A guard that clamps, narrows, truncates or falls back has two halves to its contract. It has to do its work when it is needed, and it has to stand aside when it is not. A test asserting the first half alone is passed perfectly by a guard that fires far too early, because a guard that fires on everything satisfies "whatever it left is within bounds" on every input there is.

**Wrong.** The table's narrow-window backstop narrows columns until the window leaves the tail its room. The test read the widths off the laid-out table and asserted the pair the backstop guarantees: the columns before the last one either leave the tail its room, or every one of them stands at its own heading. The backstop was comparing against a flat reservation rather than against what the last column had measured, so it fired fifteen columns of window before anything was tight and broke rows eleven columns before it had to. The assertion stayed green throughout, and so did every rendering-based check, because an over-narrowed table renders exactly like a table whose values were always that short.

**Right.** Assert both halves, and name them so a reader can see one is missing. Beside the post-condition test, add one that computes what the input needs, skips the windows where it does not fit, and asserts the guard changed nothing at all on the rest. Where the "what the input needs" figure comes from a production pass, factor that pass out and call it, so the test holds the before and the after side by side and no fixture's arithmetic is retyped into the test as a literal.

**How to tell you have only one half.** Arm the guard to fire early on purpose and run the suite. If it stays green, the suite is measuring the guard's output against the guard's own decision to act, and nothing is checking the decision. Arm it in the other direction too, so that a guard which never fires is caught by the post-condition test.


## Cross-package test-fixture helpers landing in a production package

**Wrong:** two test files in different packages both need to read the same test-fixture manifest shape and the same anchor-reading helper, so the fix moves the shared code into a real, non-test file inside the shipped library package (e.g. `internal/bench/compat.go`, exporting `FixtureRow`/`FixtureManifest`/`ReadFixtureManifest`/`DeclaredProfile`). Nothing in production code ever calls these; they exist solely for `*_test.go` files in two packages, and their symbols now sit in the production package's public API and compile into every binary that imports it.

**Right:** this codebase already has the pattern for a test-only concern shared across package boundaries: a dedicated package named for its test-support purpose (see `internal/testenv`), imported only by `*_test.go` files and never by production code. A cross-package test helper belongs in a package like that, not folded into the library it is fixturing.

## Prose that names the product's own vocabulary, re-verified only by re-running the transcripts

**Wrong:** re-recording every fenced block in a guide against the current build, checking the blocks mechanically, and treating the surrounding prose as covered by that check. A guide taught that "Dinah's own messages call a comment, an attachment, or a checklist item an entity" and described an interrupted "structural act". Both words had been the tool's own, and both were the stated reason the guide was allowed to keep an internal term the prose standard otherwise bans. A message-rewording card had since replaced them, so no string the reader can see carried either word, and the guide had become the only place the vocabulary survived while still attributing it to the tool. Every transcript in the file was correct, because a transcript is re-cut by running the command; a sentence about what the tool calls something is not.

**Right:** treat a sentence that quotes or attributes the product's wording as a claim needing its own check against the message catalog, separate from the replay that covers the blocks. Grep the catalog for every word the prose says the product uses, and for every internal term the prose kept on the grounds that the reader meets it in output. A term kept under that exemption loses the exemption the moment the output stops carrying it, so the exemption is re-earned on each pass rather than inherited.

**The test:** for each sentence of the form "Dinah calls this X" or "its messages say X", can you name the catalog key whose text carries X? If not, the sentence is describing the product as it used to print.


## A field added to a struct that helper functions rebuild literal by literal

**Wrong:** adding a field to a struct that one or more helpers copy by constructing a fresh literal out of the fields they knew about, and then working around the loss at the one call site that noticed. A rendering table gained a `labels` value saying whether a heading row is drawn. Two narrowing helpers each return `table{indent: ..., columns: ..., rows: ...}`, so both dropped it, and the layout step compensated by reading the field off the caller's original table with a comment explaining why. The result is correct and the class is still open: the narrowed table now carries a zero value that reads as a real answer, and the next field added to the struct is dropped the same way with no comment guarding it.

**Right:** make the copy total at the point where it is written. Add the new field to every helper that rebuilds the struct, so a copy is a copy and no downstream reader has to know which fields survived the trip. Where a helper deliberately resets a field, say so on that helper rather than at the site that later has to route around it. A compensating read is a note about one call site; a complete copy is a property of the type.

**The test:** for each field of the struct, grep for literals of that type and check whether each one sets it. A field that some literals omit is either intentionally zeroed there, which the code should say, or silently lost, which is the defect. Where the type has enough fields that this is tedious, that is itself the signal to copy the value and assign the changed fields rather than to build a new literal.


## An assertion written against a value the accessor never returns

**Wrong:** asserting a lookup's failure with the condition the API does not use to report it. A test walked every shipped catalog and reported a missing sentence with `if msg.For(tag).T(key) == ""`. The renderer answers a miss with the literal `{key}` placeholder rather than an empty string, because a missing sentence must still print something a reader can act on, so the condition was unsatisfiable and the loop asserted nothing in any catalog. It sat green through three review cycles and was read, twice, as evidence that the claim above it held.

This is worse than an absent assertion. An absent one leaves a visible gap that a reviewer counting criteria against tests will find. A vacuous one occupies the gap, so the count comes out right and the claim looks carried.

**Right:** call the accessor the API provides for the question you are asking, and let the failure branch use its answer. `Renderer.Has` reports presence; `T` renders. Where no such accessor exists, assert against the sentinel the accessor actually returns, and name it in the failure message so the next reader can see which contract is being relied on.

**The test:** for every equality or emptiness comparison in a test, ask what the function returns when the thing under test is absent, then confirm from that function's own body rather than from the shape of its name. Two cheap mechanical checks catch nearly all of it: temporarily invert the condition and confirm the test goes red, or break the thing the assertion guards and confirm the same. A loop over a collection also earns a guard that the collection is non-empty, since a per-item claim over zero items is the same failure by a different route.

## A test that compares generated output against the generator it came from

**Wrong:** asserting that a rendered surface matches its source by walking the source, rendering each entry, and looking for that rendering in the output. A command's help block is composed by iterating an ordered list of checks and printing one row per entry. The test iterated the same list, rendered each entry the same way, and searched the printed block for each row in turn. It passes on every input, including a list whose order has been changed, because reordering the list moves the expectation and the output together. Every mutation the author tried turned it red except the one the list exists to fix, which is the order.

The shape survives review because it looks like the opposite of a hard-coded expectation. Deriving the expected value from the code is the right instinct when the code is not the thing under test. It is the wrong one when the derivation runs through the very function whose behaviour is being asserted.

**Right:** name what the test is holding the output against, and check that the source of the expectation is independent of the property being asserted. Deriving the row text from the list is fine, because the row text is not what the assertion is about. Deriving the row order from the same list is not, because the order is. Pin an order against something that cannot move with it: a literal sequence, an independently computed key (a key derived by formula from a position holds a list's order against the formula rather than against the list), or, best of all, the behaviour the order governs, exercised through the real entry point.

Where the ordered list drives a real decision, a second test asserting that decision is the one that guards it, and the derived-output test keeps its own narrower job of proving the surface is generated rather than typed. Say so in both doc comments. A derived-output test whose comment claims to cover reordering will be read as covering it, and the next reader will not repeat the arming attempt that showed it does not.

**The test:** for each assertion, ask what value would make it fail, then ask whether that value can be produced by the thing under test while the expectation stays put. If changing the code changes both sides of the comparison in step, the assertion is an identity and asserts nothing about that property. Arm it: perturb the property, not the rendering, and confirm red.


## A shipped journal event the closed query vocabulary does not name

**Wrong:** declaring a new event name, writing it into an entity's journal, and leaving the list the query surface treats as the closed vocabulary for that plane untouched. A card gained `workstream_joined` and `workstream_left` on its own journal while `contract.Events` kept its previous fifteen names, so `event:workstream_joined` refuses `dinah.unknown-value` over a value the card demonstrably carries, and the refusal's `legal` list omits it. The event constant, the catalog token that renders it in the log, and the compatibility fixture that captures it were all added in the same diff, so every guard that counts declarations against captures stayed green. Nothing counted declarations against the query's vocabulary.

**Right:** an event name a card's own journal can carry is a name the card-plane query accepts, so a card that adds one adds it to the vocabulary list in the same commit. Leave out only the events that land on a journal the query never walks, which are the workbench's and the workstream's, and say so where the list is declared rather than leaving the omission to be read as an oversight. This is the flag-leak entry above applied to a second list.

**The test:** for each new event constant, ask which journal it is appended to. If any of those journals is one the query walks, grep the vocabulary list for the name. Absence there is a defect rather than a scoping decision, unless the list's own comment says why the name is held out.

## A deferral justified by a mitigation nobody read the code for

**Wrong:** deciding not to refuse a reachable bad state on the grounds that the checker already reports it well enough, and writing that mitigation into a source doc comment, a shipped design document and the handoff note, without reading the branch that produces it. `workstream set <ref> slug <taken> --yes` was left accepted because "check reports the pair, so a person can find both and decide which one changes". The workstream slug checker walks the collection with a `seen` map, exactly as the state slug checker does, and raises its duplicate finding on the later of the two alone. A person meets one identifier, and it is the one whose slug is now shadowed. The test pinning the accepted state counted findings carrying the key and failed only at zero, so it held the behaviour and not the sentence, and the operator's next station was being asked to bless the deferral on the strength of the sentence.

**Right:** a deferral's justification is a claim about behaviour and is read against the code like any other. Find the line that performs the mitigation before writing what it does, and pin the claim at the granularity the prose states it: assert which record the finding names and how many fire, not that the count is non-zero. When the mitigation turns out narrower than it sounded, correct the prose rather than widening the code, because the narrower behaviour is often the house idiom the deferral was chosen to preserve.

**The test:** in every sentence arguing that a bad state may stand, underline the verb whose subject is the tool, and name the function that performs it. A verb with no function behind it is the deferral resting on something nobody checked. Then ask what a test would have to assert for that sentence to be false, and check whether the test asserts it.


## An existence assertion standing in for an identity assertion

**Wrong:** proving that every reference a tool prints opens what it names by feeding each one back through the resolver and calling `os.Stat` on whatever comes back. A resolver that reads the first collection segment of `<card>/comments/1/attachments/1` and discards the rest returns the comment's own anchor, which is a real file, so the check passes while the printed address opens a different entity. The same guard's fixture attached only to the card and never to a comment, so the two-level reference the defect needs was never built either. Two independent reasons the guard could not fail, in the guard written to catch that defect. The shape is not confined to resolvers: searching a whole rendered block for a label proves the label was drawn and not that it was drawn on the right row.

**Right:** assert what came back rather than that something came back. The walk that printed the reference already holds the directory it built each node from, and every entity of this format keeps its anchor in a directory named for its identifier, so the resolution is checked against the node's own identity. Build the fixture at the full depth the printed form reaches, and fail the test outright when the fixture produced no instance of the case, so a fixture that stops short reports itself rather than going green.

**The test:** for each assertion, ask what a wrong answer would look like and whether this assertion tells it apart from the right one. `err == nil`, `os.Stat`, `len(x) > 0`, and a search of the whole output for a string all answer only that something happened. Then ask which line of the fixture builds the case the assertion is about. If you cannot point at it, add it, and add a count the test fails on when it is missing.

## A name checked against a corpus by plain substring, where a longer name contains it

**Wrong:** holding a documented name against the file that publishes it with `strings.Contains(published, name)`. The names a release publishes overlap by construction: `dinah-windows-amd64.exe` contains `dinah-windows-amd64`, and `SHA256SUMS.txt` would contain `SHA256SUMS`. A document that names the Windows binary without its extension therefore passes, and a reader who types that name gets nothing, which is the exact failure the check exists to report. The arming proof hides it too when the plant is another published name, since renaming one artifact to a sibling the same file also publishes is accepted for a second, unrelated reason.

**Right:** match the name inside its own boundaries, the way `blockLists` already holds a usage line against the ratified help block by requiring the padding or the line ending after it. A small anchored expression is enough: the name, preceded by the start of the text or a character that cannot be part of a name, and followed by the same. Then choose the plant that the bounded form catches and the substring form does not, which for a family of overlapping names means truncating one rather than swapping it for another.

**The test:** list the other members of the corpus the name lives in and ask whether any of them contains this one as a prefix, a suffix, or an infix. Where one does, a substring test cannot tell the two apart, and the plant that proves the check has to be the containment rather than a substitution.


## A prose claim about what a command prints, written from reading the code rather than from running it

**Wrong:** describing a checker's output in a doc comment, a design document, or a handoff note by tracing the function that builds the finding. Reading `checkWorkstreams` shows one finding appended for a duplicate slug and a `seen` map that fills as the walk proceeds, which reads as "the finding names the later of the two". The walk is `ListIDs`, meaning directory order over random identifiers, so the workstream it names is neither the later one nor fixed from run to run. Two reviews described that output and both were wrong, in different ways, because both were read.

**Right:** run the command, or write a throwaway test that runs it several times, before any sentence claims what it prints. Where the claim is load-bearing, meaning somebody rules on a deferral or a design by reading it, the sentence has to survive somebody typing the command. The test: for every claim about output, name the run that produced it. A claim with no run behind it is a reading of the source, so write it as one or replace it with a measurement.

Its sibling failure is worth naming with it. A collection walked in listing order beside a sibling collection walked in creation order looks like the same idiom and is not, so a precedent claim ("this mirrors the check for states") needs the walk compared rather than the shape.


## A writer that produces the bytes the comparison canonicalises away

**Wrong:** adding a step to a regeneration path that writes into a field the checker deliberately ignores, and reasoning that the step is needed precisely because no check can see it. A transcript comparison drops table padding and folds a separator row of dashes to one token, so column widths are outside the comparison on both sides. A regeneration step that redraws those separator rows therefore emits bytes no assertion in the suite reads. The step went in with a doc comment saying so in as many words, and it shipped two separator rows whose column boundaries disagree with the header and the data rows standing around them, on a page a customer reads, with every leg of CI green. The same shape had already been caught once on the same work, where restoring the document's own paths into captured output was going to write a build machine's temporary directory onto that page while every check stayed green.

**Right:** treat "the comparison cannot see this" as the reason to build a check rather than the reason a check is unnecessary. A value the guard erases needs its own assertion, held against something the erasure does not touch. For a table it is the geometry: the field offsets of a separator row have to equal the field offsets of the header and of every data row of the same table, which is a property of the written document and needs neither the tool nor a replay to test. Write that assertion in the same change as the writer, and arm it by perturbing one width.

Two smaller traps sit inside the same step and each produced one of the two wrong rows. A walk that gathers a table's rows by requiring an identical field count stops at the first row with an empty trailing cell, so the widths come from a fraction of the table. And a walk over the concatenated output of several commands, with the command lines stripped out, has no boundary between one command's table and the next, so it takes a width from a table the row does not belong to. Bound the walk by the step that produced the lines, and count a row's columns by their offsets rather than by how many non-empty cells it happens to carry.

**The test:** for every byte the regeneration writes, name the assertion that would fail if the byte were wrong. Where the answer is "the comparison normalises this away", the writer has just been given a licence to be wrong, and the licence is the defect.


## A non-vacuity counter that counts work the check never validated

**Wrong:** a guard reads two shapes out of its corpus, validates one of them and can only recognise the other, and increments one counter for both. The empty-corpus assertion then reads that counter, so a corpus carrying nothing but the unvalidatable shape passes while asserting nothing. The counter reports that the check found something, which is true, and implies that the check held something, which is not.

**Right:** count each shape into its own total and assert on the total the check can actually fail on. Say in the failure message how many of the recognised-but-unchecked shape stood in the corpus, so a reader can tell an empty corpus from one full of text this rule cannot speak to. Arming it is what proves the difference: narrow the validated shape's pattern to match nothing, and the check must go red rather than riding the other count.


## An arming break that reddens a different assertion than the one it is credited with

**Wrong:** proving a guard by breaking the code it watches, seeing the suite go red, and reporting that the guard holds the behaviour, without reading which assertion fired. Two shapes of this were paid for on one card. A guard over a syntax line was reported as armed by re-inlining a decoration inside the function that composes it, but the pre-card body composed the same bytes, so the equality the test asserts stayed true and the redness came from somewhere else in the run. And a break to the workbench gate in front of a help page's vocabulary lookup was credited with proving that the page names no states outside a workbench, while the assertion it actually reddened was the inside one: with no workbench opened the page stopped naming states in a workbench that has them. The outside half was held by a second nil check further down the call, so removing the credited gate alone left that half green, and a break aimed squarely at the outside case left the whole test green.

**Right:** an arming proof names the assertion that failed, not the suite. Run the single test, read the failure message, and check that the message describes the property the guard is credited with. Where two mechanisms independently produce one observable, write that into the test's own comment along with which of them the pass does not prove, because a later reader takes a green test as evidence for whichever mechanism they arrived looking at. Where the two are really one invariant expressed twice, say that instead, and name the line that establishes it.

**The test:** for each break, write down the failure message before writing the handoff sentence. If the message describes a different property than the sentence claims the break proves, the guard is not armed, and the sentence is a claim about a test nobody has yet seen fail.


## A transcript fenced so that the guard reading the document never sees it

**Wrong:** adding a section of console transcripts to a guarded document with the plain fence the surrounding prose uses, rather than the fence the guard's replay selects on. The blocks are then neither driven nor exempt. Nothing replays them, no exemption entry is demanded of them, the catalog sentences they quote are never held, and every check over the document stays green while the section says whatever it said the day it was written. A whole page's worth of guarded surface can be lost this way in a single card, and the loss is invisible in a green run, in the diff, and in a review that reads the guard's design rather than its selection rule.

**Right:** before trusting a guard to hold text you added, find the predicate it selects on and confirm your text satisfies it, then prove it by perturbing one of your own lines and watching the guard go red. Where the selection rule is a fence, an info string, a file name or a directory, that rule is what decides whether your work is covered, and a rule that silently drops what it does not recognise gives no signal at all. Fix the omission by adding an assertion that fails on the unselected shape, so the next author cannot leave the guard the same way.

**The test:** perturb one line of the new text and run the guard. A green run means the guard is not reading it, whatever its documentation says about what it covers. A second, cheaper test on the way in: ask what makes your new block different from the blocks the guard already holds, and if the only difference is cosmetic, that is the difference that dropped it.

**The sibling failure this belongs with.** Two reviews of the same card described a guard's reach from reading its source and were wrong in opposite directions. One reported that a replay could not reproduce a randomly minted identifier, when the guard already normalises every identifier to a token for exactly that reason. The same reading missed that the blocks in question were not replayed at all. A guard's coverage is measured the same way a command's output is: by running it and breaking something, never by reading it.


## A resolver widened at one root, filling only the fields its old callers read

**Wrong:** widening a reference resolver so a new head reaches entities it could not reach before, while the branch handling that head fills in only the fields it happens to have. Callers written against the old resolver go on reading the rest without asking, so the gap surfaces wherever they read. One card paid for this twice on one branch. The widened head returned an answer marked as a card with no card in it: the tree walk dereferenced it and panicked, and the write path sent one entity's history to a different journal under a different lock, so two spellings of one comment excluded neither each other nor a concurrent writer. The same branch also fills in no reference, so a walk rooted at exactly the entity the widening exists for prints its header sentence with empty parentheses where the address belongs, on a workbench that is in no way unusual.

**Right:** state the answer's completeness as an invariant on the resolver itself, where a caller meets it, and refuse rather than return a half-filled answer. Fill every field the new head can fill, not only the ones the caller that motivated the widening reads. Where a field genuinely cannot be filled, make the absence visible at the boundary rather than letting a renderer print it as nothing.

**The test:** take the answer's fields one at a time and ask which branch of the resolver fills each. Read the new branch against the old one field by field. Reading only the caller that motivated the widening finds the field that caller reads and none of the others, and every other caller is where the defect lands.

## A branch's document edits measured against the trunk it was cut from, on a trunk that keeps moving

**Wrong:** finishing a card that edits a document under a mechanical guard (`docs/quick-start.md`, the message catalogs, an embedded guide) and handing it forward on the strength of a merge that was clean when the last commit was pushed. Nothing on the branch says anything is wrong: the suite is green, the guard passes, and the handoff note truthfully records that the trunk was zero commits ahead at push time. Then a card lands on the trunk touching the same document, and the next reader meets `CONFLICTING` on the pull request rather than a diff. Worse, the conflict is easy to resolve wrongly. Twice on one card the correct value was one that neither side of the merge held: a replayed transcript carrying a count of the whole catalog (branch 476, trunk 424, correct 477), and a `sweptBlocks` entry naming a source location by line number (branch 399, trunk 393, correct 401). Both wrong resolutions produce a merge git calls clean and a suite that fails.

**Right:** re-take the trunk immediately before the handoff rather than at the start of the pass, and name the trunk commit the branch now carries in the note, so the next station can tell in one glance whether it is still true. Resolve any conflict landing in a counted, generated or line-numbered value by recomputing it, never by choosing a side, and then run the guard that reads the document over the merged tree rather than over the branch. A document that carries a count of a set the whole repository contributes to, and a test that names a source location by number, are both conflict magnets on an active trunk; treat a hunk in either one as arithmetic rather than as arbitration.

The test: for each conflict hunk, can I name the command whose output produced the value I wrote? If the answer is "I took the newer side", the resolution is a guess, and the only thing that will notice is a test run somebody has not done yet.


## A second hand-maintained copy of a roster the consuming package already declares

**Wrong:** a repository tool that regenerates data (here, translation-catalog skeletons) carries its own hardcoded copy of the roster it must agree with. It drifted from the canonical declaration in the consuming package: it still listed a language as a skeleton to write after that language shipped complete, and omitted a language that shipped. The tool was `//go:build ignore` and imported by nothing, so the build and the tests never read the second copy, and the drift stayed invisible until somebody ran it and silently overwrote a finished translation.

**Right:** the roster is declared once, in the package that consumes it, and the tests that guarantee every declared language ships and every complete catalog is complete read that one declaration. A hand-maintained second copy is deleted, not reconciled, because reconciling today's instance leaves the whole class of drift in place for the next key.


## A spelling enumerated in a translator's note that the parser never accepts

**Wrong:** a catalog entry's `context` field listing the alternate spellings a flag answers to, written from an earlier draft of the parser and never re-read against the shipped map. `flag.version.summary` shipped in all eight locales saying the flag "also answers to -V, -v, -version and /version" while the same commit's `askedFor` carried no `/version` at all, and a comment three files away recorded that `/version` had been tried and dropped because it is a legitimate absolute path. Running `dinah /version` refuses with `dinah.unknown-command`.

**Right:** every enumeration of a flag's accepted spellings, wherever it lives, is copied from the map the parser reads and is checked by running each spelling. This is the leak named under "A shipped flag the ratified surface does not name" pointing the other way, and that entry's test applies unchanged: a spelling on one side only is a leak, whichever direction it leaks. The surface a reviewer has to remember here is that a `context` field is prose about behaviour even though nothing prints it, so no test covers it and no reader can tell a stale enumeration from a current one. If an enumeration is worth writing in eight files, name in the handoff the run that produced it.

## A line-number-keyed fixture left pointing at the lines a file used to have

**Wrong:** editing a source file above a block that `cmd/dinah/testdata/uncovered.txt` names, and not re-pointing the entry. `cmd/dinah/help.go` gained seven lines near its top, which moved `verbHelp`'s unknown-command branch from 140 to 147 and `vocabularyValues`'s giving-up branch from 235 to 242. Both allowlist entries stayed on the old numbers in the same commit that moved the blocks. One of them then pointed at a block the suite does execute, so the fixture asserted an untruth in both directions at once: a covered block was named as unreached, and an unreached block was named nowhere. The entry's reason had gone stale too, still crediting `runHelp` with settling an unknown command after that decision had moved to `helpFor`.

**Right:** a fixture keyed on `file:line.col,line.col` is a reference into a file that any edit can invalidate, so treat every insertion above a named block as an edit to the fixture as well. Re-derive the entries from a real `go test -coverprofile` run over the package and copy the block ranges out of the profile rather than counting lines by hand, and re-read each entry's prose while you are there, since the reason names functions that refactoring moves. Two local hazards make this easy to miss and neither is an excuse: `TestEveryStatementOfTheRenderingHeadIsCoveredOrNamed` shells out to `go test .` and reports only that the coverage run failed whenever anything else in the package is red, so any unrelated local failure hides the real four lines, and a developer environment variable the suite does not isolate can supply that unrelated failure. When the coverage test reports the run failed, that is not a pass and not an unrelated problem; run the profile directly before believing the fixture is current.

## A test crediting its hook for a state an earlier half of the same test left on disk

**Wrong:** running two commands against one fixture inside one test function, driving each through the same interleave hook, and writing a doc comment that says the hook put the card into the state both commands read. The first half's hook really does write that state, and the first half is refused, so it writes nothing else and the state survives on disk. The second half then loads a card that is already in the state, whether or not its own hook fires early enough to matter. Where the second command's transaction fires the hook after it has already read the card, the hook the comment credits is a no-op and the assertion rests on the leftover instead. The test still guards the property, so nothing goes red and the false sentence survives every run.

**Right:** name the mechanism each half actually depends on. Where two halves share a fixture, either reset the fixture between them so each half stands alone, or say in the comment that the second half reads the state the first half left and that its own hook is not what put it there. When one of the two transactions cannot drive the state from its hook, say which one and point at the card that tracks it.

**The test:** for each command the test drives, ask what the fixture would look like if that command ran first. If reordering the halves changes which mechanism supplies the state under test, the comment is describing the order rather than the code, and the order is undeclared.


## A known-defect exemption that stops asserting once the defect is fixed

**Wrong:** a guard that walks a corpus and skips the entries named in an exemption table, where one table holds defects the guard itself found and left for another card. The skip is a `continue`, so the guard says nothing about an exempted entry in either direction. The day the other card lands its fix, the entry keeps its place, the guard keeps stepping over it, and a later defect on the same name can be parked under the same excuse forever. A comment claiming the entry expires with the defect makes it worse, because a reader who trusts the comment stops looking. Paid for on a check driving every declared MCP parameter through the head's argument assignment, where fixing the exempted parameter in a scratch tree left the check green.

**Right:** an exemption for a defect is an assertion that the defect is still there. Run the same check on the exempted entry and fail when it now passes, naming the entry and telling the reader to delete it. The same holds for the other table, the one naming behaviour that is deliberate rather than broken, since a deliberate no-op that later grows a real effect is a change nobody declared. An exemption written this way needs no comment promising it will expire, because it expires.

The test: fix the exempted defect in a scratch copy and run the guard. If it stays green, the exemption is permanent and the corpus can only grow.


## A comment enumerating what a change does not cover, left standing after the change covers it

**Wrong:** a field or a file carrying a list of the cases it reaches, or of the ground nothing checks, and a later change adding a case or closing one of the gaps without touching the list. The code is right and the list beside it is false in one entry, and that entry is what a reader consults before deciding whether the work is already done. A reader who trusts a stale "nothing here reads that sentence" builds a second guard over ground the file already holds. A reader who trusts a stale enumeration of call sites writes a test against a placement that has moved. Paid for twice on one card: a hook field whose doc comment named two call-site categories after a third arrived, and a guard file whose boundary list said no check reads the document's command count, in the same commit that added the check reading it.

**Right:** treat an enumerating comment as part of the surface the change edits. When a diff adds a case to a set a comment lists, or closes a gap a comment records, the comment moves in the same commit. Where an entry is only half falsified, say which half still holds rather than deleting the entry, because the remaining half is usually the next card's ground.

The test: grep the packages the diff touches for a comment that counts, lists, or says nothing here reaches something, and read every hit against what the diff just did.

## A second line-keyed registry over the same file, left stale because the first one was updated

**Wrong:** `cmd/dinah/render.go` carries two registries keyed on its own line numbers, the swept-block table in `cmd/dinah/row_sweep_test.go` and the coverage allowlist in `cmd/dinah/testdata/uncovered.txt`. A change inserted six lines into `renderCard` and re-pointed all twenty-five entries of the first registry, twice, once for the insertion and again after a trunk merge. The second registry was never opened, so ten of its entries went on naming the lines those blocks used to occupy. Nothing went red, because `TestEveryStatementOfTheRenderingHeadIsCoveredOrNamed` shells out to a child `go test .` and fails on the child's exit status before it reads the allowlist at all, and an unrelated known-failing test in the same package supplied that exit status. Handling one registry carefully is what made the file look handled.

**Right:** treat an insertion into a rendering file as an edit to every artifact that stores a line number of it, and find those artifacts by grepping the tree for the file's name followed by a digit rather than by recalling which ones exist. Then read the fixture yourself instead of asking the suite: run the profile directly (`DINAH_COVERAGE_PASS=1 go test -count=1 -coverprofile=<path> ./cmd/dinah`), pull the zero-count blocks out of it, and compare them against the allowlist by hand. The profile is written even when the package is red, so a red package is never a reason to postpone the comparison.

**The test:** list every artifact in the tree that records a line number of the file you edited, and say for each one how you checked it. An artifact you cannot name is one you did not check, and a suite that is red for any other reason cannot tell you the difference.


## An equivalence between two heads asserted on the one answer that carries nothing

**Wrong:** proving that a second head projects the same value as the first by marshalling both and comparing the bytes, on a fixture where nothing has happened. A checkpoint tool was compared against the library call it wraps by minting a cursor, handing it straight back, and asserting the two encodings match. Both sides were the empty answer, so the four collections the shape exists to carry were absent from each and the comparison held whatever the projection did to them. The assertion is strong and its input is trivial, which reads as rigour on the page.

**Right:** move the fixture between the two calls so the answer being compared carries at least one of every collection the shape declares, and fail the test when it does not. An equivalence assertion is only as wide as the value it ran on, so name the members that were populated when the comparison was made rather than the fact that byte equality was used.

**The test:** for each equality assertion between two producers, write down what was in the value on both sides. If the answer is "the same fields that are omitempty and were omitted", the test compares two absences and the equality is decoration.

## A fallback triggered by the absence of one kind of evidence when the answer already holds several

**Wrong:** falling back to a whole-collection resync when no per-item evidence was found, while only counting evidence of one kind. A checkpoint reports the items it can attribute a change to, and reports every item when it can attribute the change to none. The attribution loop counted only card-scoped journal lines, so a change explained perfectly well by a workbench-scoped or workstream-scoped line, or by an item that had just left the collection being scanned, counted as no evidence at all and fired the resync. The written decision bounded the fallback to "the case where nothing else explains the movement", and the code bounded it to "the case where no card explains the movement", which are different sets and the second is much larger.

**Right:** when a fallback keys on the absence of evidence, enumerate every source of evidence the call already holds before declaring there is none. Where the deliberate degradation is written up as a decision, read the decision's own bound back against the branch that implements it, because an honest write-up of a narrow degradation is what stops anyone from looking at the wide one.

**The test:** for the fallback's guard condition, list the inputs that could have explained the observation and check which ones the guard actually consults. Then build a fixture where the collection is non-empty at the moment the fallback fires; a fixture that has emptied the collection by then cannot tell a narrow fallback from a total one.

## An evidence clause answered by leaving it out

**Wrong:** a criterion that requires the handoff to show its working on something no search can check, answered by a handoff that says nothing about it at all. A prose criterion asked the implementer to quote the sentences weighed against four named machine-prose tells and say why each was left standing, and it said in terms that a report of counts fails it. The handoff that arrived reported the guide's sections, its provocations and its three armed checks, and carried no prose report of any kind. A reviewer scanning for the banned shape, a page of character counts and sentence-length statistics, finds none and can read the silence as a pass. Absence is the one answer the criterion did not think to forbid, and it is cheaper than the answer it did forbid.

The same class had already been caught once on the same card, in its other form: a first draft was swept, reported clean on counts, and the operator read it and named a colon-splice carrying a triad, a second triad, and two epigram closers that no count could have found.

**Right:** a criterion whose evidence is a judgement states what the handoff must contain, and the reviewer checks for that content rather than for the absence of the forbidden substitute. Where the handoff is silent, say so as a finding in its own words, and do not let the reviewer's own read stand in for it silently: a reviewer who performs the missing sweep and reports only its result has removed every reason for the next implementer to perform one. Record both, the reviewer's read and the fact that the handoff owed it.

**The test:** for every criterion that binds the handoff rather than the artifact, open the handoff and find the paragraph that answers it. If you cannot point at one, the criterion is unmet, whatever the artifact is like. Silence and a bad answer are the same verdict, and the bad answer is the one that at least shows up in a search.


## A golden fixture regenerated so that it records the defect instead of failing on it

**Wrong:** changing a renderer, regenerating the whole-output fixture from the changed renderer, and reading the resulting green suite as evidence that the change is right. A card moved a wrapped continuation two columns to the right without narrowing the room it was wrapped for, so any continuation that filled its line ran two columns past the window. The fixture is drawn at one assumed width, and after regeneration it carried three lines two columns over that width. Nothing failed. The fixture's own bytes were the defect, and every downstream assertion was made against them. The criteria were no help either: all nine were checked at the two widths the desired output was drawn at, and at both of those the words happen to stop short of the edge, so the overrun appeared at every other width and at none of the tested ones.

**Right:** a regenerated fixture is a record of what the code now draws, so it can only be believed alongside an assertion about what the output must be true of, written independently of the code that produces it. Where the change is about fitting text into a bounded space, that assertion is the bound itself: no rendered line draws past the window, swept across a spread of widths including narrow ones, where a wrap runs often. Regenerate the fixture and add the invariant in the same change, rather than treating the fixture as the test.

**The test:** ask what a regenerated fixture would look like if the change were wrong, and whether anything in the suite would then be red. If the honest answer is that the fixture would simply record the wrong output and the suite would stay green, the change has no test yet. Then ask which values of the varying input were exercised: a criterion pinned at the one or two widths, locales, or sizes the design was drawn at proves the drawing, not the rule, and the defect will live at the values nobody drew.


## An identity assertion on a fixture that holds only the subject

**Wrong:** asserting which entity an answer named, on a fixture whose collection contains exactly that entity. A test that files one card, acts on it, and then checks `len(answer.Cards) != 1 || answer.Cards[0].ID != theCard` reads like an identity assertion, but an implementation that returns every entity on the board satisfies it too, because the board is that one entity. The same shape appears as a test that empties the collection before asserting nothing in the answer names the departed thing.

**Right:** put a bystander in the fixture. Add a second entity that nothing in the window touches, and assert that the answer excludes it (or includes it, where the design says the answer is a deliberate superset). The bystander is what separates "the code chose this one" from "the code returned everything and there was only one". This is distinct from an existence assertion standing in for an identity assertion: here the assertion already reads the identity, and the fixture is what makes it vacuous.

The test to apply: name a wrong implementation, then ask whether this fixture would redden under it. If the wrong implementation is "answer with the whole collection" and the collection has one member, the fixture cannot tell.


## A mandatory instruction naming a tool the actor's own toolset does not carry

**Wrong:** writing a required step around a call the agent reading the step cannot make. A replacement isolation check told six agent definitions that the two values it compares against "come from the workbench instructions", that the agent must "fetch them with `mcp__andoneer__get_workbench(workbench=<slug>, fields=\"instructions\")`", and that "this read is its own step and you cannot skip it". None of the six definitions listed `mcp__andoneer__get_workbench` in its frontmatter `tools:` line, though a seventh definition on the same board did, so the tool existed and only the grant was missing. The same edit put `get_column` into a step in a file whose tool list does not carry that either. Because the check also said that a value it cannot obtain is its own branch ending in `block_card`, the missing grant did not degrade the check, it inverted it: every dispatch would reach the block, which is the defect the card was written to remove. The author noticed the tool was absent from its own session, recorded that as a limitation on one acceptance criterion, and did not carry the observation across to the six files it was editing.

**Right:** when an instruction names a tool, verify the grant in the same pass that writes the instruction, in every file the instruction lands in. A prose deliverable aimed at an agent has a dependency list exactly as code does, and the frontmatter is where it is declared. Where the instruction is mandatory and its failure path is a block, treat the grant as part of the deliverable rather than as configuration somebody else owns.

**The test:** for each tool named in the text, grep the tool list of every file the text was installed into. Any name that appears in the body and not in the list is either an edit to the list or an edit to the body, and deciding which is not optional. Reading the author's own session limitations is a second, cheap source for this: an author who says "that tool is not in this session's toolset" has already found the defect and has not yet recognised it.


## An absence assertion with no control that the thing ever existed

A test asserts that a narrowing rule kept something out of an answer, and reads the answer's empty arrays as proof that the rule did the keeping out. Nothing in the fixture establishes that the thing was ever produced, so a defect upstream that loses it entirely satisfies the assertion word for word. The two causes are indistinguishable from inside the test, and the one the test is about is the less likely of the two.

Caught on dinah-120 at cycle 3, in the criterion for a filtered checkpoint. The fixture moved a card outside the filter and asserted the filtered answer carried no events, no cards and nothing gone. A cursor-ordering defect was dropping that move before any filter ran, and the test passed on both readings for two review cycles.

Wrong:

```go
h.mustDo(&Request{Verb: Move, Actor: "alka", Card: ref, State: doing})

set := h.checkpoint(&Request{Since: minted, State: aftercareSlug})
if len(set.Events) != 0 || len(set.Cards) != 0 || len(set.Gone) != 0 {
    t.Errorf("a filter let through a change outside it: ...")
}
```

Right:

```go
h.mustDo(&Request{Verb: Move, Actor: "alka", Card: ref, State: doing})

set := h.checkpoint(&Request{Since: minted, State: aftercareSlug})
if len(set.Events) != 0 || len(set.Cards) != 0 || len(set.Gone) != 0 {
    t.Errorf("a filter let through a change outside it: ...")
}

// A control from the same cursor with the filter off. It is what makes the
// empty arrays above mean "the filter narrowed it" rather than "the walk
// never saw it", and those two read identically without it.
control := h.checkpoint(&Request{Since: minted})
if names := eventNames(control); len(names) != 1 || names[0] != contract.EventMoved {
    t.Fatalf("the move was never delivered to anybody, so the filtered answer above proves nothing: %v", names)
}
```

The rule: whenever a test asserts that X is absent from an answer because some rule excluded it, the fixture also has to show X present in an answer taken with that rule off. Same inputs, one knob moved. A read-only surface makes the control free, since the two calls do not interfere. This is distinct from "An identity assertion on a fixture that holds only the subject", which is about a collection too small to distinguish which member is present; here the collection is empty on purpose and the question is why.


## A reader-facing value list ordered one way by the write path and another by the read path

**Wrong:** adding a query filter for an axis whose legal values a write command already lists back in its own refusal, and building the query's list with `sort.Strings`. The write path's helper fixes the order on purpose and says so in its doc comment ("one axis's members in declaration order, which is the order the refusal lists them in"), because for a ranked axis the declared order is the ranking. The read path takes its precedent instead from a different field whose values have no inherent order, where sorting loses nothing. One workbench then answers one question two ways: setting a level on a card lists `trivial, minor, major, critical`, and querying the same axis for the same unknown value lists `critical, major, minor, trivial`. The alphabetical list looks like a ranking to the reader and it is the wrong one.

**Right:** one ordering rule per axis, taken from whichever surface already publishes that axis to a reader. Where the read path's roster is deliberately wider than the write path's, because a query tolerates a value some card carries and the workbench no longer declares, the declared members keep their declaration order at the front and the undeclared remainder follows them, so the reader meets the ranking first and the drift after it. Pick the precedent by what the values are rather than by what the function looks like. A helper written for an unordered collection is not precedent for a ranked one merely because both build a deduplicated roster of strings.

## An acceptance criterion marked verified against a test that does not exercise it

**Wrong:** closing a behavioural criterion by naming a test in its verification note when that test never runs the behaviour. The named test compiles, passes, and lives in the right package, so a reader who checks that it exists finds it and stops. A criterion saying a query returns the matching cards is closed against a test that only counts how many checks the command declares, and another is closed against "the unit coverage in the package tests" where the package has none for this path. The behaviour ships having never executed once, and the checklist records the opposite.

**Right:** a verification note quotes the assertion that would go red if the behaviour broke, not the file or the test name that happens to sit near it. Where no such assertion exists, the criterion is not verified and the honest note says which test still has to be written. A criterion whose only evidence is that the code reads correctly is verified by reading, and it says so, so a reviewer can weigh the reading rather than trust a citation that does not hold.


## An arming claim whose fixture leaves the deciding order to a random identifier

**Wrong:** recording that a criterion's fixture reddens when the defect is put back, where whether it reddens depends on how two randomly generated identifiers happen to sort. The note reads as a settled property ("armed: with the old comparison restored it fails at ..."), a reviewer who restores the defect and runs the test once sees red and agrees, and the fixture is in truth a coin toss that passes over the defect on the runs where the identifiers fall the other way. Measured on dinah-120 at cycle 4, one such note reddened on ten of twelve runs and another on six of twenty-four subtest runs, both while claiming to be arming evidence for a data-loss path. The class is not confined to identifiers: any fixture whose falsification turns on the order of two values the fixture did not choose has the same defect, and the ones that generate their subjects fresh each run are where it hides, because nothing about the source reveals that the order is not fixed.

**Right:** a fixture whose falsification depends on an ordering chooses that ordering rather than drawing it. Sort the two generated identifiers and address them as the lower and the higher, or pick the subject out of a collection whose key is known to sort above every candidate, and say in a comment which comparison the fixture is putting the code in front of. Where the fixture genuinely cannot choose, the note says the arming is probabilistic and names the deterministic fixture the criterion actually rests on, so a later reader weighs the evidence that holds rather than the evidence that reads best. The cheap check on any arming claim is to restore the defect and run the test a dozen times, not once.

## A separator rule applied to only one side of the state a shell parser carries

A guard that reads shell text and carries a state change forward (a `cd`, an exported variable, a `set -e`) has two questions to ask about every separator, not one. Does the state reach the command after it? And would the shell have performed the state change at all, in the process the later command runs in? A parser that asks only the first question is fail-open on every conditional and every subshell, and the direction it fails in is the one that matters, because the state it carries forward is the one that makes a refused command look permitted.

**Wrong:** a guard resolving which directory a `git` invocation will run in, given the rule that a `cd` reaches a later invocation across `&&`, `;` and a newline and across no other separator. The parser split the command into segments, reset the carried directory whenever the separator *before a segment containing git* was not a carrying one, and then let a `cd` in that same segment bind a fresh directory that carried forward normally. So `true || cd <worktree> ; git reset --hard` and `echo x | cd <worktree> ; git reset --hard` both resolved to the worktree and were allowed, while bash skips the `cd` in the first (the left side succeeded) and runs it in a pipeline subshell in the second, leaving the destructive command in the operator's own checkout. The suite tested the rule only in the shape `cd <worktree> || git ...`, where the separator sits between the `cd` and the git, so both leaks were green.

**Right:** apply the rule at the point the state is created as well as at the point it is consumed. A `cd` in a segment whose preceding separator is non-carrying does not rebind anything: the sequence keeps the directory it inherited, which for the top-level sequence is the payload's own working directory, so the invocation fails closed. Test each separator in both positions, `cd <wt> SEP git ...` and `x SEP cd <wt> ; git ...`, because a table that only tests one position is passed by a parser that only implements one.

**Where this bites beyond `cd`:** the same asymmetry appears in any parser that tracks an environment assignment, a `set -x`, or a `pushd` across the same separators, and in any guard that decides whether an earlier command "already happened" from the text alone. When the shell's own answer is conditional, the guard's answer is unresolved, and unresolved fails closed.


## A whole-text transform applied before the parser knows which text is data

A guard that rewrites a command before parsing it, folding line continuations or stripping comments or collapsing whitespace, applies that rewrite to every character it holds, including the characters the shell will treat as data rather than as command text. The parser then reads a command the shell never had. The failure is quiet, because the rewrite is right everywhere except inside the one region the parser has not yet delimited, and that region is exactly where the text that matters ends up.

**Wrong:** the destructive-git guard folded a backslash-newline out of the whole command up front, so that `git reset \` followed by `--hard` would read as one invocation. A heredoc body is text the parser drops, and it is delimited later, during segmentation. A body line ending in a backslash was therefore folded onto the terminator line, so `body\` and `EOF` on the following line became the single line `body EOF`, the terminator was never recognised, the heredoc ran to the end of the command, and every invocation after it went unread. `cat <<EOF` with a body line `body\`, then `EOF`, then `git reset --hard` was allowed in the operator's own checkout.

**Right:** perform the transform where the parser stands, so that it can only reach text the parser is actually reading. The segmenter consumes a backslash-newline as it walks, and it never walks a heredoc body at all, because the body is skipped whole at the newline that opens it. Bash was measured on the same input and swallows the trailing command too, so the old behaviour was not a divergence from the shell on this build. That is beside the point. A guard should not rest on how one shell happens to resolve the interaction between two rewrites, and finding the terminator first is the reading that fails closed whichever way the shell goes.

**Where this bites beyond continuations:** any preprocessing pass over command text, and the same shape in reverse wherever a parser normalises a document before it has found the regions that are exempt. A Markdown renderer that unescapes before it has found the fenced code blocks and a log scrubber that redacts before it has found the JSON string boundaries are the same defect. A transform whose correctness depends on which region a character sits in cannot run before the regions are known.


## A construct recognised in the spellings its author enumerated, and invisible in the rest

A guard that parses somebody else's language has to decide what a construct looks like, and the tempting way to decide is to write down the spellings that come to mind. Every other legal spelling then reads as absent rather than as unrecognised, which is the fail-open direction: the parser does not know it missed anything, so it carries on and clears the command. The two halves of the mistake are one mistake. A pattern that enumerates the forms of a construct will be short a form, and a state change tracked by the name of one command will be reached by another command that does the same thing.

**Wrong:** the destructive-git guard matched a heredoc opener as `<<` followed by an unquoted delimiter of the shape `[A-Za-z_][A-Za-z0-9_]*`. Bash's delimiter is an ordinary word, so `cat <<E-OF` and `cat <<EOF.1` are legal and the pattern reads only the leading `E` or `EOF`. The terminator line then never matches the delimiter the guard recorded, the body runs to the end of the text, and every command after the heredoc goes unread: `cat <<E-OF`, a body, `E-OF`, then `git reset --hard` was allowed in the operator's own checkout. A delimiter starting with a digit, `cat <<1EOF`, fails the pattern outright, so the body is read as command text instead, and a `cd <worktree>` written inside it binds a directory the shell never enters. The same guard tracked the working directory by the word `cd` alone, so `cd <worktree> && pushd <checkout> && git reset --hard` resolved to the worktree while bash ran the reset in the checkout, and `builtin cd` and `\cd` leaked the same way.

**Right:** parse the construct's grammar, and where the grammar is wider than the parser wants to implement, refuse what it cannot read rather than treating it as absent. A `<<` whose delimiter the guard cannot parse makes the whole command unresolvable, exactly as an unterminated quotation mark already does. A state change is tracked by what changes the state, so the directory set includes `pushd` and `popd` and the prefixes that reach an ordinary builtin, and a command word the guard does not recognise standing where a directory change could stand leaves the sequence unresolved instead of unchanged. The test for the class is not another case, it is the question asked once per construct: what else does the shell accept here, and what does my parser do when it arrives?


## A correction that points at a neighbouring sentence and inherits its scope

A false claim is often repaired by replacing the wrong description with a reference to something a nearby sentence already said, which reads as economical and avoids describing one rule twice. The reference carries everything the neighbouring phrase carries, not only the dimension under repair, so a correct ordering can arrive with the wrong set of things attached to it.

**Wrong:** the MCP guide said "`next_card` offers the highest-priority ready card", which was wrong about the order and right about the population, since `next_card` does filter to ready cards. The correction became "`next_card` offers that same first card on its own", pointing back at "the order the workbench fixes" that the preceding `list_cards` sentence had just named. `Library.List` filters on substate only when the caller passes the optional `ready` argument, while `Library.Next` always filters through `headOfReady`, so the head of a bare `list_cards` and the answer from `next_card` coincide only when nobody holds the oldest card in the state. On a board in use that is the exception. The repair traded a false claim about ordering for a false claim about which cards are being ordered, and review had to catch it a second time.

**Right:** name the dimension you are correcting and keep every other true claim the original made explicitly stated, even where a neighbour appears to supply it. The landed sentence reads "`next_card` offers the first ready card in that order", which borrows the ordering from the previous sentence and states the readiness filter in its own words. Where a phrase is genuinely shared, confirm the sharing holds for each fact the phrase carries: an ordering and a population are separate facts about a query, and code that sorts identically can still select differently.

The test: write down every claim the sentence under repair makes, mark which one is false, then write down every claim the replacement makes. A true claim present in the first list and missing from the second is the repair leaking, and a claim in the second list that no code path supports is the neighbour's scope arriving uninvited.


## A tokeniser taught a new word, and a reader downstream that was never taught it

Replacing a tokeniser changes what every consumer of its output sees, and the consumers were written against the old output. A consumer that walks tokens looking for the shapes it knows, and stops at the first shape it does not, will stop on the new tokens and hand back whatever it stopped on. When the caller then looks that answer up in a table of forbidden things and finds nothing, the command is cleared. The fix at one layer becomes a hole at the next, and the suite that proved the fix says nothing about it, because the suite was generated from the spellings the fix was written for.

**Wrong:** the destructive-git guard's verb scan hid an invocation whenever a metacharacter was glued to a word, so `echo a;git reset --hard` was allowed. The fix gave the guard a lexer of its own, breaking words at `; & | ( ) < >`, a backtick and a newline and emitting each metacharacter as a word. That closed the glued spellings. Downstream, `read_invocation` still walked an invocation's words the way it always had: skip a word beginning with `-`, and break on the first word that does not, treating that word as the subcommand. Handed a metacharacter word it broke on it and returned `>` or `\` as the subcommand, and the deny table has no row for either, so the invocation was cleared. A shell allows a redirection anywhere in a simple command, so `git >/dev/null reset --hard`, `git 2>/dev/null commit -m x`, `git </dev/null clean -fdx` and `git >>log push origin --delete topic` all reach git with the denied verb and were allowed; so did a line continuation, `git \` then a newline then `reset --hard`. Every verb in the deny set was reachable this way, and the guard already deployed on the operator's machine refused several of them. The suite was 567 cases green, because its glued-spelling generator appends a redirection to the *end* of the command and never places one between `git` and its subcommand.

**Right:** when a layer is replaced, re-read every consumer of what that layer produces and ask what the new output means to each one, rather than checking that the layer itself now behaves. And carry the "refuse what you cannot read" rule down to every layer instead of applying it only at the outermost one. An invocation whose subcommand slot holds a word the guard cannot classify is unreadable, and it should be refused exactly as an unterminated quotation mark already is, rather than returned as a subcommand the deny table happens not to name. Generate the test shapes from the grammar's positions (before the subcommand, between the subcommand and its flags, after the whole command) rather than from a list of spellings, because a generator that only appends is passed by a reader that only breaks on a prefix.

**Where this bites beyond this guard:** any change that moves a lexer, parser or normaliser boundary under consumers that were written against the old boundary, and any table-driven refusal whose "not in the table" branch means allow. A deny table asked about a value its producer could not have produced before is answering a question it was never given, and the safe answer to an unrecognised value in a fail-closed guard is a refusal rather than a miss.


## A separator set wider than the shell's, so punctuation inside one command reads as the end of it

A guard that decides how far a pattern may reach has to choose the characters that end a command, and widening that set looks like caution. It is the opposite. Every character added to the set is a place the thing being looked for can hide, because the span stops there and the pattern never reaches the verb. The shell's own grammar is the only authority on which characters actually end a command, and it allows most punctuation inside a single word.

**Wrong:** the destructive-git guard's span between a `git` word and its verb was `[^|;&`(){}\n]*`, where the guard it replaced used `[^|;&\n]*`. The four added characters and the backtick are command substitution, parameter expansion and brace grouping, all of which a shell expands inside a single simple command. So `git $(echo --no-pager) reset --hard`, ``git `echo --no-pager` reset --hard``, `git ${OPTS} reset --hard`, `git${IFS}reset --hard` and `git {reset,--hard}` each ran the verb, the span ended before the verb was reached, no pattern matched, and all five were allowed in the operator's own checkout. The previous guard refused every one of them. The same file had already reasoned correctly about the same question one constant earlier, blanking `>`, `<` and the `&` of `2>&1` because they are punctuation inside a single command rather than boundaries between two, and then did not carry that reasoning to the expansion family.

**Right:** a character belongs in the boundary set only when the shell cannot use it inside a word, and everything else that looks like punctuation is blanked the way redirection is blanked, length-preserved, so the pattern reads across it. Where a guard replaces a predecessor, the new class is checked against the predecessor on the same strings, because a widened class fails open and its own suite cannot see the widening.

**Where this bites beyond shell text:** any scanner with a stop set. A log parser that ends a record at a character the writer emits inside a field, a CSV reader that treats a quoted comma as a boundary, a path splitter that stops at a separator a filename may contain. Widening a delimiter set is a fail-open edit and wants the same treatment as loosening a permission.


## Detection reading the normalised text while permission reads the raw text

A guard that normalises text before matching has two readers, the one that finds the thing and the one that decides what to do about it, and they have to agree about which characters are data. When only the finder is given the normalised view, the decider is reading a document in a different language: it sees, and can be persuaded by, exactly the characters the finder was careful to discount. The failure direction is always the permissive one, because the finder's job is to notice and the decider's job is to excuse.

**Wrong:** the destructive-git guard blanked quoted spans before running its patterns, so that a commit message mentioning a command could not be read as a command. It then took the matched span out of the ORIGINAL text and searched that for the `-C <path>` that grants permission. A path written inside an argument therefore vouched for an invocation that never carried it: `git reset --hard "git -C C:/dinah-scratch/card-impl/wt x"` matched as a bare reset and was then cleared, and the same string cleared `clean -fdx`, `stash pop`, `push --force` and `commit`. Git never sees that `-C` as an option; the shell hands it over as one argument of the destructive command. The string is ordinary rather than adversarial, because the board's own agent definitions, four column instruction blocks and this guard's own refusal text all tell an agent to write `git -C <worktree>`, so quoting the advice in a commit message disarms the guard.

**Right:** decide permission over the same view the match was made in. The guard's blanking already preserved length for exactly this reason, so the normalised span was available at the same offsets and was simply not used. Where the decision genuinely needs a raw value, such as a quoted path carrying a space, locate the token in the normalised text and then read only its argument out of the raw text, so the flag has to be real even when its value is quoted.

**Where this bites beyond this guard:** any pipeline that sanitises for one stage and not the next. A validator that strips comments before checking syntax and then executes the original, a linter that ignores generated regions while the formatter rewrites them, an authorisation check run against a canonicalised path where the open() uses the string as typed.

## A reportable condition described in prose and never given the name it reports under

**Wrong:** writing that a checker reports some condition, and describing that condition by analogy to a finding that already has a name instead of minting a name for it. A format document defined `check.unknown-scheme` and `check.dangling-citation` for two citation failures, then said of a third that "an entry whose scheme demands the observation and carries none is a structural defect the checker reports". Whoever later writes the checker has a described behaviour and no constant, so they either invent a name the document never ruled on or file a second card against the document to get one. The same document was already pushed back once for exactly this, on the two findings that did get names, and the third arrived in a passage added after the review that would have caught it.

**Right:** a document that says a tool reports something names the thing the tool reports it under, in the same sentence, drawn from the existing catalog's idiom rather than a new convention. Where the naming genuinely is somebody else's ruling, the document says so and carries an open question rather than a sentence promising a report nobody can implement. A description is not a name, and an implementer cannot write a constant from a description.

**The test:** grep the passage for every sentence saying the checker, the tool, or the validator reports, refuses, or flags something. Each one either contains a literal name in the catalog's namespace, or points at the open question that will supply it. A sentence doing neither has left vocabulary for a later reader to invent.



## Adding a field to a Go composite literal and realigning only the lines you touched

**Wrong:** adding a member to an aligned struct literal or field block whose name is longer than every name already there, and letting the diff show only the lines that changed. gofmt aligns the whole run of consecutive lines, so a longer name widens the column for every line of the run, including the ones the change never touched. A diff that realigns only the added line looks correct in isolation, in review, and to every mechanical check except gofmt: `go build`, `go vet` and the entire test suite pass on it, because the defect is whitespace inside a literal.

**Right:** run `gofmt -l .` over the whole tree, not over the files you edited, and run it as a step rather than as an assumption. When a handoff reports which gates were run, the report names gofmt explicitly or the gate did not happen. A handoff meticulous about the expensive verification and silent about the free one is the shape to distrust: the silence is where the failure lives.

**The test:** after any edit that lengthens the longest identifier in an aligned run, `gofmt -l .` must print nothing. If the command was not run, no other evidence in the handoff bears on the question, because nothing else in the toolchain can see the class.


## A displayed index computed by the reader instead of taken from the resolver that defines it

**Wrong:** printing a positional reference such as `card/attachments/3` from a number the read path works out for itself, when a resolver elsewhere already defines what that position means. In Dinah the resolver's position arm is `ids[position-1]` over `SortByOrdinal(ListIDs(collection))`, so a position is an index into that sequence and nothing else. The read path printed the anchor's stored `ordinal` field instead, which agrees with the index only while the stored ordinals are a contiguous run from one. Deleting one member leaves a gap, and the same tool then draws the row `3  c.txt` carrying the reference `card/attachments/3`, which resolves to nothing, beside the row `2  b.txt`, whose reference resolves to `c.txt`. A second read surface in the same binary counted the index and printed the right references, so the tool disagreed with itself about one card. Centralising the wrong number into one helper does not repair this, and it makes the disagreement harder to see, because the helper's doc comment then promises an agreement the code does not deliver.

**Right:** a displayed reference is produced by the rule the resolver consumes, and the read path calls that rule rather than reconstructing it. Where the resolver indexes a sorted sequence, the display finds the member in that same sorted sequence and prints its index. A stored field that only orders the sequence is a sort key rather than an address, however much it looks like one.

**The test:** feed every reference a read command prints straight back to the resolver, and assert it reaches the entity whose row printed it. Run that round trip on a collection whose stored ordering fields carry a gap, because a freshly built collection cannot fail this check and a fresh collection is the only shape most tests construct.

## A verification hook that measures a branch against wherever the trunk stands today

A criterion earns its keep by being rerunnable, so its test hook has to give the same answer to the next reader that it gave the person who closed it. A hook comparing the branch to the trunk by two dots does not: it reports the difference between two trees as they stand at the moment of the run, so every commit the trunk gains after the branch's last merge arrives in the output as a reversal. The criterion then reads red, or worse reads green on a different population, through no change to the work it was written about.

**Wrong:** dinah-240 declared a no-code cut and made it checkable with "Test hook: `git diff --name-only origin/main..HEAD` run on the card's branch lists no path beginning with `cmd/` or `internal/`". It was true when the implementer ran it, and the note recorded one path, `docs/design/format.md`. Two cards landed on the trunk before review. Re-running the hook printed three paths, two of them `internal/msg/locales/de.json` and `internal/msg/msg_test.go`, which is another card's work showing up as a reversal. A reviewer who trusted the hook would have opened a finding against a card that changed no Go at all, and a reviewer who trusted the note over the hook would have skipped the check. Two consecutive reviews had to substitute the three-dot form by hand before the criterion's text was corrected.

**Right:** write the hook against the merge base, `git diff --name-only origin/main...HEAD`, so the question is what this branch did rather than how the two trees currently differ. The rule generalises past git: a hook naming a moving reference point has to pin it, either by taking the fork point (three dots), by naming the base commit the note was written against, or by measuring something the trunk cannot move.

The test: would this hook give the same answer if somebody unrelated merged a card an hour from now? If the answer depends on the trunk's current head rather than on the branch's own commits, the criterion is closed against a measurement nobody can reproduce, which is the failure the citation was supposed to end.

Neighbouring entry: "A branch's document edits measured against the trunk it was cut from, on a trunk that keeps moving" covers the merge-conflict half of the same fact about active trunks. This entry is about the verification hook rather than the merge, and the two are worth keeping apart because the remedy differs: that one says re-take the trunk at handoff, this one says stop measuring against it.

## A token boundary written as the separator that usually stands there

A rule reading text rather than a parse tree has to say where a token begins, and the tempting way to say it is to name the character that normally sits in front of one. In shell text that character is whitespace, and whitespace is the right answer only for the spelling a person types. Everything the shell does to a command after the typing stops can move a token boundary somewhere else, so a rule anchored on whitespace holds until the text has been through one expansion and no longer.

**Wrong:** dinah-233's guard decided each dangerous flag by requiring whitespace in front of it, `\s-[a-z]*f[a-z]*\b` for the `-fdx` of `git clean -fdx` and `\s--delete\b` for `git push origin --delete topic`. Eight of its twenty-three rules were written that way and all eight were fail-open against `git {clean,-fdx}`, which is brace expansion and which a shell runs as `git clean -fdx`. The verb was found, because the verb's own pattern needed no whitespace in front of it, and the flag that condemns the verb was not, so the guard cleared the command. Three cycles of hand-written cases had never put a comma in that position, and the guard deployed on the trunk was wrong the same way, so the branch's own regression check could not see it either.

**Right:** say what a token cannot contain and let every other character start one. `(?<![\w.=/~@+-])` in front of the flag admits whitespace, a comma, a brace, a parenthesis and the start of the span without naming any of them. Being wrong about a separator nobody has thought of yet then costs a refusal rather than a leak, because an unlisted character starts a token instead of hiding one.

**The test:** run the construct through an actual shell instead of through the spelling you had in mind, and compare what your reader decided against what the shell handed the command. Brace expansion, parameter expansion and command substitution each rewrite the whitespace of a command before it runs, so generating any one of them is enough to find this class.


## A normalisation that disagrees with the shell about which characters are code

A guard that reads command text has to decide, before any rule runs, which characters are a command and which are data. Getting that wrong in either direction is a defect, and the two directions fail differently. Treating code as data hides a command from every rule at once. Treating data as code costs a refusal, which is survivable. The tempting shortcut is to sort the characters by the punctuation around them, and shell punctuation does not sort that way, because a quotation mark is not one thing and a separator inside a construct is not separating the command that construct is a word of.

**Wrong:** dinah-233's guard blanked every quoted span alike, on the ground that a commit message must not be read as a command. A shell runs a command substitution inside double quotation marks exactly as it runs one outside them, so `git -C <a linked worktree> commit -m "$(git reset --hard)"` normalised down to a qualifying commit, was allowed, and was then run by bash with the reset inside it. The same file left boundary characters visible inside a substitution, which is the mirror of the same mistake. The `;` in `git $(echo;) reset --hard` separates the two commands inside the substitution and does not end the command the substitution belongs to, so the guard's span stopped before the verb and there was nothing left to refuse. Both strings reached a recording stub as a bare `reset --hard`, and the guard deployed on the trunk allowed both as well, so the branch's own regression comparison could not see either.

**Right:** derive each rule from what the shell does with the construct rather than from what the punctuation looks like. A single-quoted span is data all the way through, because a shell runs nothing inside one. A double-quoted span is data except for its substitutions, which stay visible to the rules. A boundary character inside a substitution is blanked, because it belongs to the commands in there rather than to the command outside. Every one of those is a line of documented shell grammar that was written down long before the guard was.

**The test:** for each construct the normalisation treats as data, ask whether a shell executes anything inside it, and for each character it treats as a boundary, ask which command that character actually ends. A generator that puts a real command inside the construct, with a recording stub first on PATH, answers both questions without anybody reasoning about them.

Neighbouring entry: "A whole-text transform applied before the parser knows which text is data" is about when the transform runs. This one is about what the transform decides, and the remedies differ, so they are kept apart.

## An affordance list still naming an act the same change made refusable

**Wrong:** narrowing when an act is permitted, updating the code path that refuses it, and leaving the lists that advertise what a caller may do next still naming it. dinah-273 made `claim` refuse at an intake state, a done state and a buffer. It corrected one affordance list, the MCP `next_card` tool's, adding `pull` beside `claim` with a comment saying that an agent reading only `claim` meets a refusal with nothing telling it what to reach for. It left the sibling list untouched: `Library.affordances` switches on the card's substate alone, so every response for a ready card standing at any of those three kinds went on offering `claim`, and the journal tool's hard-coded `{"show", "claim"}` did the same. The two lists answer one question and only one of them was corrected, in the same diff, on reasoning that named both.

**Right:** an act removed from a caller's reach is removed from every list that says what the caller may do, and a list that cannot see the fact it needs is given a route to it rather than left guessing. `affordances` holds the library, so it can read the state the card stands at and ask the predicate; nothing forced it to decide from the substate alone. Where the correction genuinely belongs on a later card, the exception is recorded on this one rather than left to be discovered.

**The test:** when a change makes an act refuse where it used to succeed, grep for the act's own name as a string, not for the call site that refuses it. Every list, table or affordance array holding that literal is a caller that has to be read. A guard scanning for a re-derived rule will not find these, because a list naming `"claim"` answers no question about a state and compares no kind.

Neighbouring entry: "A shipped flag the ratified surface does not name" is a surface that gained something nobody declared. This one is a surface that kept something the tool withdrew, and the direction is what makes the greps differ.

## An agreement test whose early return skips the half it was written for

A test that holds an offer against the act it offers has two halves on every case: the offered act must succeed, and the act that is not offered must be the one that refuses. A table-driven test that branches on which act was offered, runs that act, and returns is asserting the first half only. On every case that takes the branch, nothing checks what else the list carried, and those cases are usually the exact ones the defect lived at, because the branch is taken precisely where the behaviour is unusual.

The failure is invisible to arming. Breaking the code so the list offers too little redirects those cases into the other branch, where the assertions do run, so the arming proof goes red and reads as though the test were sound. Breaking it so the list offers too much is what stays green, and offering too much is the defect an affordance list actually has.

**Wrong.** Five states, each carrying a card, each reading the offered list off a real response. Where the list offered the pull, the pull was run and the case returned; where it did not, the claim was run and its outcome compared against whether the list offered a claim. At an intake state and at a buffer the list offers the pull, so those two cases returned before anything looked at the claim. Putting `claim` back into the list at exactly those two states left the whole repository green.

```go
if contains(offered, Pull) {
    response := h.library.Pull(&Request{Verb: Pull, Actor: "bo", State: "doing"})
    if response.Outcome != contract.OutcomeOK || response.Card.Ref != ref {
        t.Fatalf("the list offered %s and the pull answered %s", Pull, response.Outcome)
    }
    return // nothing below ever runs for an intake state or a buffer
}
response := h.do(&Request{Verb: Claim, Card: ref, Actor: "alka"})
if contains(offered, Claim) != (response.Outcome == contract.OutcomeOK) {
    t.Fatalf(...)
}
```

**Right.** Run every act the case can run, and order them so that one does not consume the other rather than skipping one of them. A refused act consumes nothing, so the act expected to refuse goes first and its absence from the list is checked there; the act expected to succeed goes last.

```go
response := h.do(&Request{Verb: Claim, Card: ref, Actor: "alka"})
permitted := response.Outcome == contract.OutcomeOK
if contains(offered, Claim) != permitted {
    t.Fatalf("the list %v offers claim: %t, and the claim answered %s %s",
        offered, contains(offered, Claim), response.Outcome, response.Refusal)
}
if contains(offered, Pull) {
    // the claim above refused and took nothing, so the card is still here
    pulled := h.library.Pull(&Request{Verb: Pull, Actor: "bo", State: "doing"})
    ...
}
```

The test for it: pick the case the branch is taken on, add the act the branch skips to the offered list, and run the suite. A green suite means the case is unasserted.


## A vocabulary translation applied per payload shape instead of at the head's one exit

A head that serves a different vocabulary from the library it wraps has to translate what it publishes. When the translation is applied call site by call site, the sites that get it are the ones whose payload shape the author was looking at, and the site whose payload is shaped differently keeps the untranslated spelling. The reader following it reaches for a name the surface does not serve, which is the dead end the translation existed to prevent.

The shape that hides it is a payload that declares its own affordances member and therefore does not travel through the head's wrapper. Every wrapped payload gets the translation because the wrapper is where it was installed; the one self-declaring payload is returned whole, and nothing on its path ever sees the list.

**Wrong.** The MCP head translates the library's command spellings (`ls`, `next`) into its own tool names (`list_cards`, `next_card`). Three card-shaped reads were routed through a helper that translates, and the refusal path was translated in the envelope. `readChanges` returns the `ChangeSet` itself rather than a wrapped payload, because the shape already declares an `affordances` member, so the `ls` in `changeAffordances()` reaches an agent that has no `ls` tool to call.

```go
func readChanges(l *verb.Library, r *verb.Request) any {
	changes, err := l.Changes(r)
	if err != nil {
		return l.FromError(r, err)
	}
	return changes // Affordances: {"status", "ls", "show", "log"}, untranslated
}
```

**Right.** Translate where the head's answers leave it, not where each one is built, so a payload cannot opt out of the translation by being shaped differently. Where that is impractical, enumerate the exits rather than the shapes: every function that returns a payload carrying an affordances member is a site, whether or not it goes through the wrapper.

```go
func readChanges(l *verb.Library, r *verb.Request) any {
	changes, err := l.Changes(r)
	if err != nil {
		return l.FromError(r, err)
	}
	changes.Affordances = surfaceAffordances(changes.Affordances)
	return changes
}
```

The test for it: assert over every tool the head serves, not over the ones that share a wrapper, that no published affordance name is a key of the translation map. A per-tool test written beside the tool that was being fixed will pass while the sibling ships the untranslated name.


## A prediction list left un-narrowed when a new refusal row joins the rows it already mirrors

**Wrong:** adding a precondition row to the authoritative sequence and leaving untouched the cheap predicate that enumerates, ahead of the lock, what the act could do. dinah-253 added an operator-owned row to `canLand`'s destination list, so a pull that would land holding a card at a state reserved to the operator refuses `not-operator`. `pullCandidates`, which builds the bare form's set of destinations, already mirrors every sibling state-level row: it skips a destination at capacity, skips one being retired, and skips a source reserved to the operator. It did not learn the new row. A non-operator's bare pull therefore goes on offering the reserved destination, and either names it in an `ambiguous-state` list the caller cannot act on or selects it and meets a refusal where the same command used to succeed. The predicate's own doc comment states the rule it broke: it narrows "rather than refusing, because the bare form enumerates destinations and one the caller cannot reach makes a worse answer than a shorter list."

**Right:** a new row in the authoritative list is added to the predictor in the same diff whenever its siblings are already there, and the predictor has the values it needs (`pullCandidates` had already computed whether the asking owner is the operator). Where the predictor genuinely cannot see the fact, the divergence is recorded on the card rather than left for a caller to find. A predictor documented as a prediction tolerates a race with the lock; it does not tolerate a standing disagreement that fires every time.

**The test:** for each row you add to an ordered precondition list, enumerate the other rows of that same list and ask which of them a pre-lock predicate already reproduces. If the siblings are mirrored and yours is not, that is the finding. Grepping for the act's own name as a literal will not reach this, which is what separates it from the neighbouring entry "An affordance list still naming an act the same change made refusable": that one is a hard-coded roster of verb names, this one is a computed set of targets, and only the first is findable by searching for a string.


## A document's own count of its rows, left standing when a row is added

**Wrong:** adding a row to a table in a published document and leaving the sentence beneath it that counts the table's rows. `docs/spec/core-profile.md` closes its section 10 boundary table with "Rows ruled in: 33. Rows ruled out: 22. Total rows: 55." A card added one row ruled out, seven lines above that sentence, and the sentence went on claiming 22 and 55. Every guard stayed green, because `internal/profile` counts the numbered statements and the boundary table's rulings against the statements that name them, and nothing counts the rows against the sentence that says how many there are. The card's own reasoning had already established that its change touched the boundary table and no numbered statement, which is true, and that is what made the tally look like somebody else's problem.

**Right:** a count written into prose is derived data over the thing beside it, so an edit to the thing is an edit to the count. Recompute it from the table you just changed rather than from the number you just read, and say in the handoff which figures moved. This is the line-number-registry entries above applied to a second kind of derived reference: a line number points into a file and a tally points into a table, and both are stored copies of something the artifact already knows.

**The test:** after editing any table, list, or enumerated block in a document, search the surrounding text for a digit that describes it, and recount. Where the document states a total, restate the recount in the handoff so a reviewer can check the arithmetic instead of taking the edit's word for it. A count no guard reads is a count only a reader will ever catch, and by then it has shipped.


**Since this entry was written:** the count above is now read by a guard. `TestTheBoundaryTallyCountsTheRowsTheTableCarries` in `internal/profile/extract_test.go` parses that sentence with a regexp and holds its three numbers against the rows `BoundaryTable` reads out of the published profile, so the specific defect this entry records cannot ship again. That does not retire the entry, and it narrows rather than replaces the test above. One document now has a guard over one tally; every other table, list and enumerated block in `docs/` still carries its counts by hand, and the guard fails loudly if somebody rewords the sentence rather than silently accepting the new wording, which is the behaviour a tally guard has to have to be worth writing.

## An arming break that reddens nothing, because a second path produces the same output

This is the sibling of "An arming break that reddens a different assertion than the one it is credited with", and it fails in the opposite direction. There the break reddened something and the wrong property got the credit. Here the break reddens nothing at all, and the two readings of that are not equally likely: the guard may be unnecessary, or it may be doing real work that no test can currently see. An implementer who reports a green suite as "already covered elsewhere" has chosen the first reading without checking.

**Wrong:** breaking a guard, watching the suite stay green, and concluding the guard is redundant or that some other test already holds the behaviour. The tree's substate axis suppresses a blocked group that no card occupies. Removing the suppression reddened five tests, so the guard looked well armed. Inverting it, so that a blocked group was suppressed whether occupied or not, left every test green, and so did narrowing its occupancy read from both card sets to the survivors alone. Neither result meant the guard was idle. A group at a value some card carries is drawn by the carried-value path whether or not the closed axis declared it, so an occupied blocked group appeared in the tree under every variant, and its presence proved nothing about where the decision was made.

**Right:** when a break leaves the suite green, find the observable the two versions actually differ on before deciding anything. They differed on order rather than on membership: a group drawn because the axis declares it stands among the declared values, while one drawn because a card carries it sorts in among the hand-written values by its bytes. A fixture carrying a hand-written value that sorts ahead of the declared one separates the two orders, and asserting the whole order rather than a membership catches both variants. The filtered case needs its own half, because a guard reading only the surviving cards behaves identically until a filter hides the card the guard is reading.

**The test:** a green suite under a deliberate break is a finding, not a pass. Write down what the broken version would print differently, and if the answer is nothing, say so and delete the guard. If the answer is something, that something is the assertion the suite is missing.


## A helper rewritten in a second package, where the compiler cannot report the duplication

**Wrong:** writing a new unexported helper for a question the tree already answers, in a package that cannot see the existing answer. `internal/profile/guards_test.go` gained a `calledName(ast.Expr) string` that returns the identifier for a plain call and the selected name for a method call, which is exactly what `cmd/dinah/args_coverage_test.go`'s `calleeName(*ast.CallExpr) string` already does. The two packages cannot share an unexported helper, so nothing in the build, the vet pass or the test run says a word about it. A duplicate written inside one package is caught in seconds by the compiler, and that is the reason a duplicate written across two is the one that ships: the fast feedback that trains the habit is absent exactly where the habit is needed.

**Right:** search before writing, by the question rather than by the name you were about to give it, since a name you invent will not match the name the other package chose. Where the duplication cannot be removed because the packages genuinely cannot share the helper, spell the second copy exactly as the first, signature included, and say in its doc comment which file it mirrors and why the copy exists. The next reader then finds both from either one, and a later card that wants to lift the pair into a shared package can see the pair.



## A resolver that treats reaching the container as having read its contents

A static check that follows a value back to its declaration has two ways to fail, and only one of them is the one people guard against. The familiar one is that the check does not recognise the spelling and reads the value as absent, which the entry "A construct recognised in the spellings its author enumerated, and invisible in the rest" already covers. The other is that the check recognises the spelling perfectly, arrives at the container the value lives in, finds the container empty at the point of declaration, and reports success. The strings the container will actually carry arrive from somewhere the walk never goes, so the check announces that it read the value when what it read was an empty box.

The two failures look different from inside the code and identical from outside it. A check that cannot follow a construct can be made to say so. A check that follows a construct to an empty container has nothing to say, because by its own lights it finished the job.

**Wrong:** a guard resolves a refusal's named-value map by walking the enclosing function for composite literals assigned to that name, for `make(map[string]string, ...)` calls, and for index writes into the name, and reports the map as read if any of those is found. Every one of these three passes the guard in silence while an English phrase reaches a reader:

```go
values := map[string]string{}
fillValues(values)                       // the strings are written in another function
return contract.RefuseWith(shape, detail, values)

values := make(map[string]string, 1)
fillValues(values)                       // the same, through make
return contract.RefuseWith(shape, detail, values)

values := map[string]string{}
for name, text := range phraseTable {    // the strings live in a table the walk never reads
    values[name] = text
}
return contract.RefuseWith(shape, detail, values)
```

**Right:** separate "I reached the container" from "I read everything that goes into it", and let only the second one clear the site. A container is read when every write into it resolves to a literal the check itself examined. A container the enclosing function hands to another call, or fills from a value the check cannot resolve, is unresolved and gets reported at the raise site exactly as an unfollowable argument does. The zero-write case is the sharp one: an empty literal with no resolvable writes in scope is the least evidence a check can have, and reporting it as the most evidence is the whole of this mistake.

The test for the class is one question asked of any resolver: when this walk returns success, which of the value's possible contents did it actually look at, and what would it have done if the answer were none of them.


## A fail-closed rule stated over values when it belongs over writes

The entry above, "A resolver that treats reaching the container as having read its contents", prescribes the right direction and states its rule one notch too tight. Its Right form says a container is read when every write into it resolves to a literal the check itself examined. Implementing that sentence literally on dinah-282 turned a green tree into thirty-three findings in one run, because most values a real codebase puts in such a container are not literals at all.

The reason is worth holding on to. A guard of this kind is hunting for English somebody typed into the source, and English somebody typed is a string literal. A value that arrives as `filepath.Join(root, name)`, `strconv.Itoa(len(words))`, `s.r.T(label)` or `search.userBase` carries data to a reader and carries no English written into this container. When the check looks at one of those and finds it is not a literal, it has answered the question rather than failed to answer it. Treating every non-literal as an unread value refuses the entire legitimate corpus, and a guard that refuses everything gets weakened or deleted within a cycle.

**Wrong:** the strictness is placed on the value, so any expression that is not a string literal leaves the container unread.

```go
// every one of these is reported, and every one of them is fine
return contract.RefuseWith(shape, detail, map[string]string{"home": search.userBase})
extra := map[string]string{"count": strconv.Itoa(len(words)), "label": s.r.T(label)}
anchor := map[string]string{"path": filepath.Join(root, WorkbenchAnchor)}
```

**Right:** the strictness is placed on the writes, so the question is whether the check walked over every place a string can enter the container, and the value's shape decides only whether that particular string is English.

```go
// read: the walk went over this write, and the value is not a literal, which is an answer
values["home"] = search.userBase

// read: a literal at the raise site carries its whole content in front of the check
return contract.RefuseWith(shape, detail, map[string]string{"why": "..."})

// unread, and reported: the container is handed somewhere the walk does not go
fillValues(values)

// unread unless the ranged container itself resolves: a copy moves another container's literals in
for name, text := range phraseTable {
    values[name] = text
}
```

The two rules agree on every case the entry above demonstrates and disagree on the ordinary code around them, which is why the difference only shows up when somebody runs it. So the test to apply is not "did each value resolve to a literal" but "could a literal have entered this container through a route I did not walk", and the second question is the one that both fails closed and leaves a working tree green.

Stating a fail-closed rule is cheap and calibrating it is not. Run the rule over the whole tree before writing it into a doc comment, and let the count of findings tell you whether you wrote down the property you meant.

## A write-walk keyed on the identifier rather than on the container

A guard that resolves a container by walking its enclosing function for writes has to decide what counts as a write into that container. Keying the walk on the identifier the container was declared under is the obvious choice, and it is wrong, because Go lets a second name reach the same map. An alias assignment, a pointer taken and handed on, and a struct field the map is parked in each give the write a different spelling on the left-hand side, and a walk matching `values[...]` sees none of them. The site then clears as fully read while the strings arrive through a route the walk never went down.

This entry sits under "A resolver that treats reaching the container as having read its contents" and closes the gap that entry leaves. That one says to walk every write; this one says the walk cannot find every write by name alone.

**Wrong:** a write is recognised by comparing the assignment target's identifier against the declared name, and the name is treated as unread only when it is passed to a call. Each of the three below passes such a guard in silence while an English phrase reaches a reader, verified on dinah-282 against `internal/bench/bench.go:369`:

```go
values := map[string]string{}
alias := values
alias["home"] = "a phrase written through an alias"
return contract.RefuseWith(shape, detail, values)

values := map[string]string{}
fillThrough(&values)                 // func fillThrough(v *map[string]string) { (*v)["home"] = "..." }
return contract.RefuseWith(shape, detail, values)

values := map[string]string{}
holder := carrier{inner: values}
holder.inner["home"] = "a phrase written through a struct field"
return contract.RefuseWith(shape, detail, values)
```

**Right:** treat every expression that carries the name somewhere the walk cannot follow as an escape, on the same terms as a call that is handed the name. An assignment whose right-hand side is the bare name, a unary `&` applied to the name, and the name appearing as an element of a composite literal are all the container leaving the walk's sight, and each one reports the site. The question to ask of any such walk is whether the identifier is the container's identity or merely one handle on it. In Go it is only ever a handle, so a walk that treats it as an identity is counting a subset of the writes and calling it all of them.

## English hidden one node below the top of the expression

A guard hunting for English somebody typed has to decide, for each value, whether that value carries a phrase. Classifying the value by the shape of the whole expression is cheap and it misses the shapes people actually reach for, because a typed phrase very often sits inside a larger expression rather than being the whole of one. A concatenation and a format call both put an English literal one node below the top, and a check that inspects only the top node reports neither.

The entry above this pair, "A fail-closed rule stated over values when it belongs over writes", is right that a non-literal expression is an answer rather than a gap, and its supporting sentence goes one clause too far when it says that English somebody typed is a string literal. English somebody typed *contains* a string literal, and the difference is the whole of this mistake.

**Wrong:** the value is classified by the top node of the expression, so anything that is not a `*ast.BasicLit` of string kind counts as carrying data.

```go
// both of these pass, and both put a typed English phrase in front of a reader
values["home"] = "no workbench was found beneath " + search.userBase
values["home"] = fmt.Sprintf("no workbench was found beneath %s", search.userBase)
```

**Right:** classify the expression, then descend into it and test every string literal it is built from. Descending needs one piece of care, because a literal that is punctuation or a separator is not a phrase: `strings.Join(fields, ", ")` carries a literal containing a space and carries no English, so the phrase test wants a rule better than "contains a space" once the walk goes below the top node. Choosing that rule is real work, and skipping the descent to avoid the work leaves the guard blind to the two spellings above.

## A migration whose skip-guard cannot tell its own output from its input

A rename migration is expected to be re-run. It walks a tree, one workbench fails on a permission or a lock, the operator fixes that and runs it again, and the report's own failed list is what invites the second run. So the guard deciding whether an entity has already been migrated is load-bearing, and it has to answer correctly for an entity the previous run already rewrote.

That guard breaks silently when the rename maps one key onto a name another key already uses. dinah-287 renamed a card's `state` key to `column` and its `substate` key to `state`, so `state` names the flow position before the migration and the condition after it. The guard asked whether the card carried `state` or `substate`, which is true of an unmigrated card and equally true of a migrated one, and the rename primitive drops the target's lines when the target name is taken. A card that had already been carried across was carried across a second time: its real column identifier was deleted, the string `ready` was renamed into the `column` slot, and the `state` key vanished. An absent state reads as `ready`, so the card answered a non-empty column and a plausible-looking state, and the run reported success and exited 0.

Two paths reach it and both are ordinary. A card locked partway through the walk ends that workbench's migration with some cards rewritten and some not. A workbench anchor that cannot be written ends it with every card rewritten and the anchor still declaring the old revision. In both cases the anchor still speaks the old vocabulary, so the second run opens the workbench through the lenient opener and walks the cards again.

**Wrong:** deciding an entity is unmigrated by testing for a key name the two vocabularies share, and relying on the rename primitive to be harmless when it is not.

```go
if !fm.Has("state") && !fm.Has("substate") {
    continue
}
fm.Rename("state", "column")   // "column" is taken on a migrated card; its lines are dropped
fm.Rename("substate", "state")
```

**Right:** key the guard on a name only one of the two vocabularies can carry, so the answer is unambiguous whichever side of the migration the entity is on. Here that is `column`, which no pre-vocabulary card has. Better still, make the container's own declaration the completion marker and write it last, so a partly-migrated container is recognised as partly migrated rather than as unstarted, and never walked with a per-entity guess.

```go
if fm.Has("column") {
    continue // already carried across by an earlier run
}
```

**The test:** run the migration, kill it partway, run it again, and compare every value against what it held before the first run rather than against emptiness. Then ask, for each key the guard reads, whether that key means the same thing on both sides of the rename. A key whose meaning the migration itself changes cannot be the key the guard reads. The happy-path test does not reach this: it starts from a clean old workbench, and the corruption needs a run that starts from the previous run's output.

**Related:** "An existence assertion standing in for an identity assertion" is why the damage stays invisible, since a destroyed card still opens, still exports and still lists. "A repair verb that abandons its walk and its account" covers the neighbouring case where the interrupted run loses its report rather than its data.


## A test input whose distinguishing shape is erased by the constructor that builds it

This is the mirror of "A writer that produces the bytes the comparison canonicalises away". There the writer emitted bytes no assertion could read; here the test emits an input the function under test never receives, because a normalising constructor resolved it on the way in.

**Wrong:** writing a test case whose whole point is an unusual spelling, and building that spelling with a helper that normalises. A containment test meant to prove that two spellings of one directory are settled by the filesystem added a case named "a child spelled through itself and back out", built as `filepath.Join(child, "..", "child")`. `filepath.Join` calls `Clean`, so the argument that reached `PathUnderRoot` was the plain `child` string and the case was byte-identical to an ordinary containment check standing beside it. Breaking both `os.SameFile` comparisons in the function down to string equality left this case green while the sibling case built by concatenation went red. The name on the subtest asserted a property the constructor had already removed, and a reader auditing the criterion counts it as coverage.

**Right:** build an input meant to be unusual with concatenation, and say in the code why the obvious helper is not used. The sibling case in the same test does this: `root + string(filepath.Separator) + "child" + string(filepath.Separator) + ".."`, which reaches `os.Stat` uncleaned and is resolved by the operating system rather than by the standard library. Where a normalising constructor is unavoidable, assert first that the value still differs from the plain form, so the case fails loudly when the normalisation swallows it.

**The test:** print the argument the function under test actually received, and compare it against the plain form the case is supposed to differ from. When the two are equal, the case is a duplicate wearing a name that claims otherwise. The general shape reaches past paths: `filepath.Join`, `filepath.Clean`, `url.JoinPath`, `strings.TrimSpace` on fixture text, and any builder that canonicalises, all remove the difference a case exists to exercise.

## A word-boundary lookaround whose refused characters are the target language's own operators

A rule that reads another language's text and wants to say "this token stands alone" writes a lookaround listing the characters that would continue a token. The list is then defended by asking what each character does when it survives into the token, and that is the wrong question by half. A character that is punctuation in the target language is precisely the character the target language is entitled to consume before the token is ever formed, so it can sit in the text immediately beside the token while never reaching the program that receives it.

**Wrong:** the destructive-git guard said a git subcommand is a whole shell word by wrapping every deny-set verb in `(?<![\w.=/~@+-])` and `(?![\w.=/~@+-])`, and defended the set with the claim that each of those characters, left in the text, hands git a longer token than the verb (`xreset`, `.reset`, `/reset`, `reset-`, `reset/`). Four of the seven are also parameter-expansion operators. `git ${x-reset} --hard`, `${x:-reset}`, `${x=reset}`, `${x+reset}` and `${x/y/reset}` each put a refused character immediately to the left of a literal, unquoted `reset`, and bash hands git exactly `reset --hard`. Every verb in the deny set was cleared by that one shape, and `git reset ${x---hard}` cleared the condemning flag by the same route. The guard on the trunk had refused all of them with a plain `\b`, so a change made to refuse less refused less than it meant to, on a security guard, in the direction that permits.

**Right:** before a character joins a boundary class, ask what the target language does with it *in the position the lookaround inspects*, not only what it does when it survives into the token. Where the class has to contain such a character, normalise the construct that consumes it first. This file already blanks redirection operators and the boundary characters inside a substitution, length-preserving, for exactly this reason; a `${...}` expansion's operator and parameter name want the same treatment, so the literal word is left standing beside a space and the ordinary rule reads it.

**The test:** the entry "A token boundary written as the separator that usually stands there" already prescribes generating brace expansion, parameter expansion and command substitution through a real shell. The case that separates this class from the ones already covered is a parameter expansion *whose value is the token under test*. A generator that produces only `${OPTS}` and `${OPTS:-}`, and uses them as empty separators in the gap between the program and its subcommand, exercises the gap and never the subcommand, and will report green over tens of thousands of strings while the whole deny set is open.

## A failure provoked by a mechanism that is a no-op on some of the platforms shipped to

A test that exercises what a tool does when a write fails has to make a write fail. The lever a Windows developer reaches for is `os.Chmod(path, 0o444)` on the file the tool writes, and that lever really does work on Windows. On Linux and macOS it induces nothing at all, so the tool succeeds, and the test asserting that a failure was reported fires there and only there.

The reason is documented in `docs/design/format.md`, in the passage on the ordinal migration. Every write in this format is a temporary file renamed over its target, so the right that governs a write is the right to replace a name. POSIX grants that right through the containing directory, so a read-only file sitting in a directory its owner can write is replaced without complaint; Windows asks the file's own attribute instead and refuses. One lever, two platforms, opposite answers.

What makes this a class rather than one mistake is the shape of the evidence it produces. dinah-287 landed such a test, ran the whole suite on Windows, read green, and handed off. Two of three jobs on the pull request were red on that test, and the carry-on-past-a-failure behaviour it was written for had never once run on Linux or macOS. A green local run is evidence about one platform, and a comment written from that run ("a directory permission does not stop a rename on every platform this runs on") can state the rule exactly backwards while every check the author ran agrees with it.

**Wrong:** one lever, chosen from the platform the author happens to be on, with a comment asserting the other platforms behave the same way.

```go
// The anchor is made unwritable rather than the directory, because the
// migration's last write is to that file and a directory permission does
// not stop a rename on every platform this runs on.
if err := os.Chmod(filepath.Join(root, bench.WorkbenchAnchor), 0o444); err != nil {
	t.Fatalf("chmod: %v", err)
}
```

**Right:** provoke the failure by the mechanism the running platform actually uses, in one helper, with the documented rule quoted where the branch is taken.

```go
func denyWrite(t *testing.T, path string) {
	t.Helper()
	target := path
	if runtime.GOOS != "windows" {
		target = filepath.Dir(path) // POSIX grants the right to replace a name through the directory
	}
	...
}
```

**The test:** ask what your failure-inducing lever does on each platform the project ships to, and confirm the answer from that platform's own documentation rather than from the one machine in front of you. Then check that the test can still tell: a test asserting only "the tool reported a failure" is indistinguishable from a test asserting nothing when the lever is inert, so give it a second assertion about the state the failure left behind, which an inert lever cannot satisfy. Read the pull request's checks before handing off, on every platform, because that is the only place the other two answers appear.

**Related:** rule 7 of "Go style standard" covers the neighbouring case, a path comparison that asserts how a platform spells paths rather than what the code does. Both come from taking one machine's answer for the contract.


## A before-and-after coverage claim whose "before" is counted on the "after" tree

A change to a guard's selector changes how many things the guard checks, and the honest way to describe it is a pair of numbers. Both numbers have to be counted where they actually live. The "after" number is counted on the branch, which is the tree in front of you. The "before" number lives on the trunk, and counting it on the branch gives a number for a population that never existed, because the branch has usually also added or removed the very items being counted.

dinah-287 produced both halves of this in consecutive rounds, on one sentence. The first version claimed the edit gave "a strictly larger set: no entry it used to check is dropped", with no count behind it at all, and the opposite was true. The correction that replaced it did count, and counted the "before" population on the branch: the branch's catalogs carry 647 keys, so two catalogs read as 1288 where the trunk's catalogs carry 631 and the old guard covered 1262. The card had minted thirteen new keys, so the two figures could not have agreed. The corrected loss of 216 is really 190. The wrong figure then travelled into a workbench document's amendment and into an open question put in front of the operator for a release ruling, which is what makes this expensive rather than untidy: a reviewer who checks the arithmetic finds it consistent, because every number in the sentence was taken from the same tree.

**Wrong:** count both populations on the branch, and describe the difference as though one of them were the trunk's.

```go
// Before dinah-287 the roster was [en, hi, de] and the loop covered every
// entry of German and Hindi, which is 1288. After it, this loop covers 1072.
// The population fell by 216 and gained none.
```

**Right:** read the "before" population out of the trunk, name the tree each number came from, and account for anything the branch itself added to the denominator.

```go
// On origin/main the roster was [en, hi, de] and each catalog carried 631
// keys, so the loop covered 1262. On this branch the catalogs carry 647,
// thirteen of them minted here, and the loop covers 1072 because it skips
// the 111 skeleton entries in each of German and Hindi. The population fell
// by 190 and gained none.
```

**The test:** for any sentence of the form "it used to cover N and now covers M", ask which checkout each of N and M was measured on. If the same working tree produced both, N is wrong whenever the branch touched the population, and a branch that changes a guard's selector has usually touched it. `git show origin/<trunk>:<path>` is the one-line way to count the trunk's copy without leaving the branch. Where the number is going into a document or an operator ruling rather than into a comment, count it twice, because the reader of a ruling has no diff in front of them and cannot check it.

**Related:** "A verification hook that measures a branch against wherever the trunk stands today" is the same mistake made about the trunk's motion rather than about the branch's own edits. "A non-vacuity counter that counts work the check never validated" is the neighbouring case, where the number is real but measures the wrong thing.

## An assertion on a value that is also the reader's default

An assertion comparing a field against a literal says nothing when the reader
fills that same literal in for an absent field. The test passes against code
that carried the value across and against code that dropped it on the floor,
and the two are indistinguishable from the outside. It is the existence
assertion one layer in: the field exists, so it reads back, so the check is
satisfied.

It is worth its own entry rather than folding into the existence entry,
because the tell is different. An existence assertion is visible in its own
text, since `!= ""` and `err == nil` announce themselves. This one looks like
a value assertion and reads as a strong check, and only the reader's own
defaulting rule gives it away. You find it by asking what the reader does
with an absent field, not by looking at the assertion.

Wrong, from `cmd/dinah/vocabulary_test.go` before dinah-287's third review
cycle. `LoadCard` reads an absent `state` key as `ready`, and every card in
the fixture was ready:

```go
if card.State != "ready" {
    t.Errorf("a card of %s carries the state %q, wanted the condition rather than a column identifier", where, card.State)
}
```

A migration that dropped the condition key entirely passed this, on every
card, while destroying the field the assertion was written to protect.

Right, in two parts, because either half alone is still vacuous. Compare
against the value the card actually held before the run, and make the fixture
hold a value that is not the default, with a guard that fails the test if it
ever stops doing so:

```go
if card.State != was.condition {
    t.Errorf("a card of %s is in the condition %q and was in %q before the run", where, card.State, was.condition)
}
```

```go
func assertConditionVaries(t *testing.T, stood map[string]standing) {
    t.Helper()
    for _, was := range stood {
        if was.condition != contract.StateReady {
            return
        }
    }
    t.Fatalf("every card in view stands in %q, so comparing conditions across the run asserts nothing", contract.StateReady)
}
```

The second half is what keeps the repair from decaying. A later change to the
fixture that leaves every card ready would make the value comparison vacuous
again and nothing would report it, so the fixture's own property is asserted
beside the behaviour it makes visible.

Where to look for it. Any field whose zero value the reader replaces with a
meaningful default: a condition that defaults to ready, a count that defaults
to zero, a kind that defaults to the first member, a flag that defaults to
false. Ask what an absent field reads as, then ask whether the fixture ever
holds anything else.


## A universal claim generalised from the cases the reviewer named

Caught twice on dinah-192, in the same sentence of `internal/guide/guides/mcp.md`, one lap apart. It is the successor to "A prose claim about what a command prints, written from reading the code rather than from running it", and it is the failure that entry does not catch: the run happened, and the sentence written from it still overreaches.

The shape is a writer who is handed a defect in a claim, reproduces the reported cases faithfully, and then writes a sentence whose quantifier covers ground none of those cases touched. Running the tool is what makes the new sentence feel settled, so nobody probes the quantifier itself.

**Wrong.** The reviewer reports that "every workbench this server can reach" is false, and names two shapes: a server with no root, and a root that itself holds a workbench. The writer runs four servers covering exactly those shapes, all four agree, and the replacement reads "it reports a workbench held by any directory below that one, at any depth". The walk it describes skips every dot-named directory and every symlinked one, so a workbench at `<root>/.hidden/proj` is below the root, at a depth the sentence claims to cover, and absent from the list. The sentence now carries one stated exception, which teaches the reader there are no others.

**Right.** Probe the quantifier, not only the reported cases. Before writing "any", "every", "at any depth" or "always", read the loop the sentence describes and enumerate its `continue` and `skip` branches, then build one fixture per branch and run it. Either the sentence names each exclusion, or it drops the universal: "it reports the workbenches it finds by walking down from that directory" promises a walk without promising the walk is exhaustive.

**The test.** For every universal quantifier in a claim about what a command produces, name the run that would falsify it. A quantifier with no falsifying run behind it is an assumption wearing a measurement's clothes, and the measurement of the neighbouring cases is what disguises it.

**Where it bites hardest.** A claim about an enumeration tool, because an agent's only affordance for discovering what it may name is the enumeration, so an omission the prose does not warn about reads to the agent as an absence rather than as a gap.


## A roster-keyed assertion emptied by the same change that empties the roster

A guard that iterates a declared list asserts nothing once the list is empty, and the change that empties the list is usually the change that should have noticed. The tell is that the guard keeps passing, its own package suite keeps passing, and nothing anywhere reports that a check stopped checking. The list is data rather than code, so no compiler and no vet pass sees the loop go cold.

dinah-287 produced this in `internal/msg`. Its D-6 took German and Hindi off `msg.Complete`, leaving `Complete = []string{Base}`. Two guards were keyed on that roster. `TestATranslationTracksItsEnglishSource` carried a `checked == 0` fatal, so it failed loudly, was investigated, was rewired onto `msg.Tags()`, and the whole episode was written into the workbench document "Translation staleness contract" as an amendment. `TestEveryDeclaredLanguageShips` carried no such fatal and went quiet in the same commit for the same reason, and three review cycles passed over it. Its completeness arm reads `if complete[tag] && translated != total`, which after D-6 can fire only on `en`, and `en` is complete by construction. German and Hindi joined neither roster, since `Skeleton` still names only the five generated catalogs, so both fell out of every arm of the test except the one counting key presence.

The cost was measured rather than argued. Marking all 536 of German's remaining translations as skeletons and running `go test ./internal/msg/` on the branch reports `ok`. The same edit against `origin/main` reports `--- FAIL: TestEveryDeclaredLanguageShips: de ships complete, got 0 of 631 translated`. The guard was live before the card and dead after it.

**Wrong:** iterate a roster and assert per member, with no assertion about the roster itself.

```go
for _, tag := range Tags() {
    translated, present, total := Coverage(tag)
    if complete[tag] && translated != total {
        t.Errorf("%s ships complete, got %d of %d translated", tag, translated, total)
    }
}
```

**Right:** assert the population before iterating it, and assert that every shipped member belongs to one roster or another, so a catalog cannot fall between them.

```go
graded := 0
for _, tag := range Tags() {
    if tag == Base {
        continue
    }
    if !complete[tag] && !skeletons[tag] {
        t.Errorf("%s ships and is on neither roster, so nothing here grades its contents", tag)
    }
    if complete[tag] {
        graded++
    }
}
if graded == 0 {
    t.Fatal("no catalog is declared complete, so the completeness arm asserts nothing")
}
```

**The test:** for every guard keyed on a declared list, ask what it asserts when the list holds one entry or none, and whether the change in front of you moves the list. Where a sibling guard over the same list already carries a non-vacuity fatal, that fatal is evidence the hazard is real and an argument for giving this one the same, not an argument that this one is covered.


## A refusal reused across shapes where its sentence is true of only some of them

One refusal name answering several defects is good economy when the defects really are one defect. It stops being economy when the sentence the operator reads is false of one of the shapes that raises it, because the operator then acts on a description of a file he does not have.

dinah-287 minted `dinah.vocabulary-mixed` for three shapes. Two of them genuinely mix the vocabularies inside one file: a workbench anchor carrying `states:` beside `columns:`, and a card carrying `column:` beside `substate:`. The third is a card written wholly in the retired vocabulary, carrying `state:` and `substate:` and nothing else, inside a workbench whose own anchor declares the current revision. That card is not mixed. It is internally consistent and it disagrees with the anchor above it, which is a different defect with a different repair. The catalog sentence reads "{detail} carries a key from each of the two vocabularies this format has had, so Dinah cannot tell which of them the file is written in", and Dinah can tell perfectly well which vocabulary that file is written in. The next-step fragment then instructs the reader to "hand-edit the file so that it carries one vocabulary and not both", which cannot be done to a file that already carries one and not both.

The spec prose written in the same commit gets the distinction right, which is what makes the message's error visible without leaving the diff. `docs/spec/core-profile.md` describes "the two shapes that mix the vocabularies within one file" and describes the third separately, so the author held the distinction and the catalog did not receive it.

**Wrong:** give a new refusal one sentence covering every raise site, worded from the raise sites that share a shape.

```go
// three raise sites, one message
return contract.RefuseWith(contract.VocabularyMixed, detail, map[string]string{"path": path})
```

**Right:** count the shapes before wording the message. Where every shape shares a repair, one message with a repair-shaped sentence serves them all. Where one shape has a different repair, it gets its own refusal name or its own detail-conditioned fragment, so the sentence is true wherever it prints.

**The test:** raise each site by hand, read the sentence the operator gets, and ask whether it describes the file now on disk and whether its instruction can be carried out on that file. A next step naming an action already true of the file is the loudest form of this defect, and it is only visible from the output, never from the call site.


## A count maintained by hand in more than one place

Named on dinah-287 after three consecutive review rounds each corrected the same figure and each left it wrong somewhere else. This entry also supersedes the "Right" block of "A before-and-after coverage claim whose 'before' is counted on the 'after' tree", which was itself written with the figures 647 and 111 and which was stale within one commit. That entry's lesson stands, which is that a before-and-after claim names the tree each number came from. Its example is retired, because naming the ref is not enough when the number is still typed by hand into three files.

The tell is a figure that describes the tree, written into the tree. A catalog key count, a coverage total, a number of skeleton entries, a count of commands or tools: each is computed from something the repository already holds, and each goes stale on the next commit that touches what it counts. Writing it in three places is three chances to be wrong and no chance to notice, and the failure is invisible locally because nothing compares the copies. On dinah-287 the same commit regenerated a transcript reading 650 and left a comment forty files away reading 647.

Wrong:

```go
// Before this card the loop covered 631 entries of German and 631 of
// Hindi, which is 1262. After it, en carries 647 keys and each of those
// two carries 647 of which 111 are skeletons, so the loop covers 536
// apiece and 1072 together.
```

Right:

```go
// The counts live in one place and that place computes them: `dinah
// version --catalogs` prints every shipped catalog with its translated
// count over its total, read off the catalogs themselves. The population
// this loop covers is the sum of the translated column over every catalog
// but English, and the entries a retranslation would face in a language is
// the difference between its two columns.
```

The repair has two acceptable shapes and a hand-written number is neither. Compute the figure where it is asserted, so it cannot disagree with itself, which is what a replayed transcript does and what a `checked == 0` fatal does. Or state it in exactly one place together with the command that produces it, and have every other place name that command rather than quote its output. When a decision genuinely needs the number in front of a reader, give the command and the commit to run it on.

The counterexample this entry is easiest to confuse with is a figure that describes a decision rather than the tree. "Ninety-five entries went back to English at this rename" is a fact about one commit and does not go stale, because the commit does not change. A count of what the tree holds today is a different animal, and the test is whether the next commit can falsify the sentence without touching it.

