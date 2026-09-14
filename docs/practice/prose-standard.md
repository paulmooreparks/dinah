# Prose standard

Every piece of prose this board produces follows this standard: spec fields, card descriptions, comments, move notes, documents, and any diff that touches README or docs prose. Spec and Implement write to it so review does not have to catch what should never have been written; Agent Review and Code Review enforce it, naming the tell when they push back.

## The tells

The machine-prose signatures to find and remove. The colon-splice used as the default sentence shape (occasional legitimate use is fine, especially introducing a genuine enumeration). Three short declaratives in a row with matching structure. The "X, not Y" antithesis, especially as a closer. "Not only... but also." Ending a section on a tidy epigram. Restating a concrete claim one notch more abstract. Reflexive rule-of-three lists. "This matters because" throat-clearing, and any sentence that instructs the reader how to read the sentences that follow. Intensifiers doing no work. The count-then-list opener ("Three courses are open"). Runs of bolded declarative captions used as bullet headlines. The trailing which-clause gloss, where every statement is followed by a comma and an explanation of itself. "Which is why" as the standard causal connective. The abstract noun parked where an adjective or verb belongs, and the adjective that adds register rather than information; both have their own sections below. Sentence-length uniformity, which is the strongest signal and the one no search finds; it can be measured, and a standard deviation of words-per-sentence below about half the mean means the prose never varies its stride, which is what produces the even, unhurried, machine cadence.

Also standing from the operator's global rules: no em-dash characters anywhere; complete sentences with subjects and main verbs in prose; a section's first sentence never borrows its subject from the heading; within one list, fragments and full sentences do not mix.

## Vocabulary the product has ruled

**The product's word is "workbench", with no exceptions a person can see.** That holds in every register: documents, help text, refusal messages, message catalogs in every language, card text, commit messages, and chat with the operator. The short form survives only where no reader meets it, meaning Go identifiers and package paths, the `.dinah` directory name, and git history, which describes the past and is not edited.

The flag and the environment variable used to be exceptions and are not any more. A flag a person types and a variable a person sets are surfaces they read, so both say the long word. Any text proposing to reintroduce the short form on a surface a person meets is wrong, whatever the reason offered, and a card that finds itself wanting to is a card that has misread this rule rather than found an exception to it. A test in the repository enforces this, so the mistake fails the build rather than reaching review.

Also ruled: "AI colleagues", never "coworkers", on user-facing surfaces.

## The serial comma

Every in-sentence list of three or more items takes the serial (Oxford) comma before its final conjunction. The rule exists for clarity rather than taste: "the party had strippers, Kennedy and Khrushchev" invites a reading the writer did not intend, and "strippers, Kennedy, and Khrushchev" does not. Adding the comma to an existing list is a safe transformation; it changes no word and can only remove a false apposition.

## Banned words: the register-raising adjective

Words that exist to make a plain noun sound weightier. The test: delete the word, and if nothing is lost it was never doing anything, and its only effect was to raise the register. The exemplar is "worked", as in "worked example": a real idiom from mathematics textbooks, where a problem is solved step by step so the reader can follow the method, and not what an example in a design document is. The family: comprehensive, robust, seamless, rich, powerful, detailed, careful, concrete, real-world, key, core, critical, essential. A key requirement is a requirement; a detailed description is a description. When some requirements really are more important than others, say which and why rather than reaching for an adjective. The same instinct hides in verbs, where "leverage" stands in for "use".

## The adjective or the verb over the abstract noun

The most portable rule of the set. "The call timeout is configuration" says the timeout can be configured, so write "the call timeout is configurable". A copular sentence whose complement is an abstract noun turns a property into a category, and the same shape hides as "X is a requirement" for "X is required", "X is a dependency" for "X depends on Y", and "X is a consideration" for saying what to consider. The second form parks the nominalisation in the subject ("Protection of the customer number is open issue 24"); make the actor the subject and use the verb ("Open issue 24 asks what protects the customer number").

The test: when the main verb is a form of "to be" and its complement is an abstract noun, especially one ending in -tion, -ment, -ance, -ence, -ity or -ness, try the adjective or the verb.

The exception matters. Sometimes the noun is the literal claim: "the change is configuration on the delivered functions rather than a new function" means the change consists of configuration, which is not the same as being configurable, and must stay. Separate "can be X-ed" from "consists of X" before replacing; the first substitution preserves meaning and the second does not.

## The hard constraint on rewrites

Meaning cannot change. Not a design, a requirement, an obligation, an exclusion, a qualifier, or a causal relationship. Rewritten text must survive an accuracy review by a reader who is not looking at style at all.

Safe transformations: deleting a count sentence when the list follows immediately; deleting a stranded sentence-initial And or Or; turning "which is why X" into "so X" with every other word kept; splitting a sentence at a conjunction with no word changed; replacing a colon-splice with a full stop and a capital; replacing an abstract-noun complement with its adjective, in the "can be X-ed" sense only; inserting a serial comma before the final conjunction of a list; deleting a register-raising adjective once the deletion test above confirms nothing is lost; cutting a clause that restates the sentence it is attached to, once its addition of no fact is confirmed; unbolding a caption with no word changed, or demoting it to a plain sentence when the text beneath does not fully carry its claim; deleting an intensifier only where it modifies description rather than commitment.

Unsafe transformations, each with a recorded failure behind it: dropping a qualifier; paraphrasing a precise condition; adding a word that tightens a claim; adding evaluative framing; deleting a negative clause, and this is the important one, because the antithesis is a style tell that is often also the specification, and the excluded party is a requirement; narrowing a generalisation to today's instances; weakening an intensifier that carries a commitment, since "behave exactly as today" promises no visible change and the same sentence without "exactly" does not.

On this board the negative-clause warning carries double weight: the contract's refusal statements are exclusions by construction, and a style pass that deletes a "not" deletes a requirement.

## Verification of a cleansing pass

Run a word-frequency comparison across the document, before against after, and account for every added and removed word against the transformations intended; an unexplained moved word means an unintended change. Then produce a sentence-level diff for a fresh reviewer whose only instruction is to find a sentence whose meaning changed, with the risk points named so they check those rather than skim. Anything that reviewer returns removes its transformation class from the safe list. The sentence-length measure above doubles as a check on the pass itself: a document whose stride did not vary before the pass should vary after it.

## Registers this does not govern

Bullet-list items, label-value lines, table cells, and headings are their own register and may be fragments. Code, commit-message subject lines, and journal notes follow their own conventions. The operator's own ruled text is his voice; audit it, flag findings for him, and never revise it without his word.

## No dated anecdotes in durable prose

A specification, a design document, a README, and a column's instructions are all read long after the day they were written, so an incident story dates the moment it lands. State the rule the incident taught instead, in the present tense, and let the reasoning stand on its own.

The shapes to cut: a count that was true on a particular date; a story about what one agent or one person did on one afternoon; a justification that rests on the reader knowing what happened last week. Where evidence genuinely belongs on the record, a card comment or a decision note holds it, because those are dated by construction and nobody mistakes them for the rule.

The test: read the sentence as somebody arriving in six months who was not there. If it now needs a footnote to make sense, it was an anecdote rather than a reason.

## The product name is never translated

Dinah stays in Latin script in every language, in prose and in message catalogs alike. Treat it the way a placeholder is treated: a token a translator may move within a sentence and may not respell.

The name is the command a person types, the domain the product lives at, and the string anyone pastes into a search box when something goes wrong. A second spelling in another script gives one thing two names and no path between them, and the reader who learned the second one cannot type the first. Transliteration also fixes a pronunciation, and it can fix the wrong one: the name is said DYE-nuh, after Alice's cat and the railroad song where Dinah blows the horn. Scripts without a phonetic alphabet make the problem worse rather than better, because characters chosen for their sound carry meaning as well, so spelling the name in them amounts to choosing a different name in a language the person choosing it cannot read.

Where a document introduces the tool to a new reader, a pronunciation gloss beside the Latin name is welcome, in the shape `Dinah（ダイナ）`. That is a documentation convention rather than a catalog string, and the Latin name still carries the sentence.

The expansion "Dinah Is Not A Harness" follows the same rule for a different reason. It is a pun on the name, so a translation either loses the joke or loses the letters. Keep the English and let the surrounding prose say what it means.


## Documentation: the reader or the tool is the subject of the sentence

This section governs anything a person reads to learn the product: the quick start, the guides, the help text, and error messages. It outranks the rhythm and tell rules below when they conflict, because a document can pass every one of those checks and still be unreadable.

**Put `you` or `Dinah` in the subject slot of almost every sentence.** An abstraction as the subject is the single loudest problem in documentation prose, and it survives every stylistic check because nothing about it is ungrammatical.

| Instead of | Write |
|---|---|
| The flag spelling you may reach for first is not one it accepts. | Dinah does not accept `-w`. |
| Every act Dinah records carries an owner. | Dinah assigns an owner to every action it takes. |
| The slug is the prefix every card reference carries. | Every card in a workbench has a human-readable prefix, called a slug. |
| Create one where the work is. | You may create a workbench in the same directory as the rest of your work. |

**Write a condition as a condition.** "Leave `--slug` out and Dinah derives one" reads as an instruction followed by a surprise. Write "If you do not provide a slug with the `--slug` option, Dinah derives one from the directory name." The same goes for a heading or a caption that stands in for the condition: "Asked for nothing in particular" is not a sentence, and the reader is left assembling it. Write "If you do not give `config` an argument, Dinah lists every setting."

**Never use the product's internal vocabulary.** A reader has never met the words this project uses among its own documents. A refusal is an error message. The surface is the list of commands. A person reading a guide should not have to learn the team's names for things in order to follow it, and a sentence that explains the vocabulary before it can make its point has already lost.

**Follow a definition with an example immediately.** A reader accepts a new term when they can see one. Name the thing, then show it.

The test, and it is quick: read a paragraph and underline the subject of each sentence. If the subjects are mostly not `you` and not `Dinah`, rewrite the paragraph rather than editing it.


## A sweep is a read, not a count

This section governs the check you run over prose you have just written, before you hand it to anybody. The cleansing-pass verification above governs a rewrite of somebody else's text; this one governs your own first draft, and the failure it exists to prevent is different.

Almost everything a search can find is the minor half of the standard. A count of em-dash characters, curly quotation marks, serial commas, and words per sentence is cheap to produce and reads like diligence, and a report full of those numbers can sit on top of prose carrying every tell that actually matters. The four loudest tells cannot be counted. The colon-splice used as the default sentence shape, three short declaratives in a row with matching structure, the tidy epigram closing a section or a paragraph, and the concrete claim immediately restated one notch more abstract all require somebody to read the sentences and judge them.

**Report the candidates, not the totals.** For each of the four, a sweep report names the sentences it weighed and says why it left each one standing. A bare "none found" is a claim with no evidence behind it, and a reviewer cannot tell it apart from a check that was never run. Where a sweep genuinely found nothing, the report says which sentences it considered and rejected.

**No pass is clean on counts alone.** A report that offers only numbers has not swept the prose. Send it back.

The tests, each of which takes one pass over the text.

For the triad, look for any sentence whose clauses run in parallel three times, and for any pair of adjacent short sentences with the same shape. The signature is repetition of the opening word or the grammatical frame: "it costs you, it hides, and none of it is finished", or "still sitting, still taking up the place, and still not finished". Two coordinated clauses read as a thought; three read as a cadence.

For the closer, read the last sentence of every section and every paragraph on its own. If it says again, in tidier words, what the sentences above it already said, cut it and lose nothing. "That is the whole of the exception" and "That is where the pile forms" are the shape: a demonstrative pronoun, a form of "to be", and a summary.

For the colon, the question is not how many colons the document holds. Labelled lines and genuine enumerations are fine and do not count. The question is whether a clause after a colon is a full sentence elaborating the one before it, and whether that construction is carrying the document's ordinary explanatory work. Where it is, replace it with a full stop and a capital.

For the restatement, look for a sentence that names something specific followed by a sentence that names the same thing in the vocabulary of principle. Keep the specific one.
