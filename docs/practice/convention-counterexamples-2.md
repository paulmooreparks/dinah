# Convention counterexamples 2

This is part 2 of 2, and it is the part new entries are appended to. The purpose of this collection and the convention for writing an entry are stated at the top of "Convention counterexamples 1", which holds the older entries.

Before you append an entry here, read this document's current byte count. When the new entry would carry this part past 200,000 bytes, create "Convention counterexamples 3" instead and append the entry there, so that no part approaches the 262,144-byte ceiling at which the listing route stops returning a body.

## A construct excluded from a guard's threat model by an asserted property nobody tested

Caught on dinah-293, cycle 2, and it is the second catch of the class the entry "A word-boundary lookaround whose refused characters are the target language's own operators" records from one layer up. That entry is about the characters a rule refuses. This one is about the sentence that says a construct need not be considered at all.

A guard that reads text in a host language (a shell, a template language, a query dialect) eventually writes down which constructs of that language can change what the text means. The list is load-bearing: the code's trigger set is derived from it, and a future editor who trusts the sentence will keep the derivation. So an entry on that list which is asserted from the author's model of the language, rather than run, is a hole with a comment over it saying there is no hole.

**Wrong.** The safety argument names three constructs and rules each out by a property it asserts:

    Tilde expansion, filename expansion and history expansion are outside
    that trigger, and each is left out for a reason rather than forgotten. A
    tilde prefix becomes a home directory and a glob becomes a filename, so
    both hand git a longer word instead of a bare verb ... None of the
    three can delete a character and leave nothing behind.

Two of those three claims are false, and one line of shell shows it. With a file named `reset` in the working directory, `git rese[t] --hard` and `git rese? --hard` both hand git a bare `reset` and a bare `--hard`. Filename expansion deleted two characters and left nothing behind. The same argument's account of the normalisations fails the same way: `git reset -\<newline>-hard` runs a hard reset, because a line continuation inside a flag is deleted by the shell and blanked into two spaces by the guard, so the guard sees `-  -hard` and the shell sees `--hard`.

**Right.** Test every entry on the list before it is written down, and write the residual honestly when the test says the construct does reach:

    Filename expansion CAN delete characters from inside a word:
    `git rese[t] --hard` reaches git as `reset --hard` when a file of that
    name exists. It is outside the trigger anyway, because the trigger
    turns on a second reading of the same text and the verb is not in that
    text to be read. The guard on the trunk is blind here too, so this is a
    shared hole rather than a relaxation, and it is named here rather than
    excluded.

The distinction that makes the correction cheap: a construct can be outside a trigger for two quite different reasons, and only one of them is a claim about the host language. "It cannot do the thing" is a claim, and it has to be run. "It can do the thing and our mechanism would not help" is an observation about our own code, and it is checkable by reading. Reach for the second wherever it is true, because it survives the day somebody discovers the first was wrong.

The test that catches it: take every construct the argument excludes, write the shortest string in which that construct does the forbidden thing, and put the string to the real host language. If you cannot write such a string, say what you tried. If you can, the exclusion is wrong and the sentence has to change even when the code does not.


## A vocabulary rename applied to every occurrence of the word, including the ones that were ordinary English

Caught on dinah-287, cycle 5, in `internal/mcp/tools.go`.

A card renaming a domain term across a codebase sweeps for the word and replaces it. The sweep finds occurrences where the word was the term being renamed, which is the point, and it also finds occurrences where the same word was ordinary English carrying its dictionary meaning. Replacing the second kind produces a sentence that is not merely wrong but ungrammatical, and it survives review because a reader scanning a large mechanical diff reads the replacement as one more instance of the change they already approved.

The board has spent four review cycles on this card's rename hunting the opposite defect, an occurrence the sweep failed to reach. Over-reach and under-reach are the same sweep's two failure directions, and a card that has been pushed back three times for under-reach is a card whose next pass is tempted to widen the pattern.

Wrong, from the diff as it stands:

```go
// workbench falls inside that rule rather than outside it, even though config
// does not. A workbench's own fields are workbench column that travels with the
// repository, where a user setting is a machine artifact, and the operator
// check guards the write here exactly as it does at a terminal, because the
// library holds it.
```

`state` here meant condition, in its ordinary English sense: a workbench's fields are data that travels with the repository. The rename moved `state` to name a card's condition and `column` to name a board station, so neither new word belongs in this sentence at all, and "workbench column that travels with the repository" names nothing.

Right:

```go
// workbench falls inside that rule rather than outside it, even though config
// does not. A workbench's own fields are workbench data that travels with the
// repository, where a user setting is a machine artifact, and the operator
// check guards the write here exactly as it does at a terminal, because the
// library holds it.
```

The fix is not a wider or narrower pattern. It is a second pass over the prose the sweep touched, reading each replaced sentence for sense rather than for the pattern, because a pattern cannot tell a term from a homonym. Two mechanical checks help and neither is sufficient: grep the new noun in the constructions the domain term never appears in (here `workbench column`, `machine column`, `column that travels`), and read every replaced line that sits inside a comment rather than inside code, since code has a compiler and prose does not.

## A sweep bucketed by the preceding word, run over a tree whose identifiers are CamelCase

Caught on dinah-287, cycle 6, after the same completeness claim had already been wrong once on cycle 5.

A vocabulary rename cannot be audited by searching the tree for the new word, because most occurrences of that word predate the rename. The instrument that works is the rename's own word-level diff, bucketed by the word each replacement sat behind before the rename, on the reasoning that board vocabulary is a countable noun behind a determiner while ordinary-English usage sits behind other company. That instrument is sound and it found ten defects in one pass.

The blind spot is where the replaced token is a CamelCase identifier. The preceding word on the line is then `//` or `func`, never the word that actually modifies the renamed noun, so every renamed identifier lands in one enormous bucket that nobody reads. The defect that escaped was a Go test named `TestTheTwoUnresolvableCrashStatesAreReportedAndNotRepaired`, where the two "states" are the two conditions a crashed directory rename can leave behind, and the rename turned them into board columns.

Wrong, the bucketing rule as first written:

```
take the word preceding the replacement in the line's pre-rename text
```

Right, the same rule with the identifier case closed:

```
split the replaced token into sub-words on CamelCase and underscore
boundaries; if one of them is the renamed noun and it is not the first,
bucket on the sub-word before it; otherwise fall back to the word
preceding the token in the line
```

With the second rule the escaped defect surfaces in a bucket of two, beside the rest of the readable tail. The general lesson is that a sweep keyed on adjacency has to be told what the tree's own token boundaries are, because an identifier is a phrase with the spaces removed and an adjacency rule cannot see into it.

A second half of the same lesson concerns what the sweep can catch afterwards. The claim that this evidence expires when the branch squashes onto the trunk is false on a repository that squash-merges: the squash commit is a single-parent commit whose own diff is the branch's cumulative diff, so `git show --word-diff=plain -U0 <squash-sha>` reproduces the artifact the pass consumes. Run the pass before shipping because it is cheaper and because the reviewer is already holding the diff, not because the evidence is about to be destroyed.


## A guarantee moved from the structure into a table, with the comment still citing the structure

**Wrong:** lifting a shared walk out of a function so that a branch which used to be special-cased now flows through the same callback as every other branch, and leaving standing the comment, exclusion list or guard-test roster that excused that branch on the ground that it "cannot reach this by any route". Before the extraction the branch was handled inline and reached no capture, no accumulator and no rewrite path, whatever any lookup table said. After it, the branch reaches the common callback like everything else, and the only thing still stopping the old outcome is a name's membership in a table that any later card may edit. The prose reads as though the structural argument survived the refactor, so the guard test that would have covered the branch omits it, citing a reason that was true of the code being replaced.

**Right:** when a refactor routes a previously special-cased branch through common code, re-derive every claim that rested on the special case rather than carrying it across. Where the guarantee has become data-dependent, cover the branch in the guard test instead of excusing it, and rewrite the justification to name the table that now carries the guarantee. The tell is an exclusion whose stated reason describes control flow the diff has just deleted, and the check is to break the table entry and see whether the excluded branch now produces the outcome the comment says it cannot.

**Related:** "A comment enumerating what a change does not cover, left standing after the change covers it" names the stale-comment half. "A construct excluded from a guard's threat model by an asserted property nobody tested" names the untested-exclusion half. This entry is the case where the diff under review is itself what invalidated the property, which is why neither of those catches it on its own: the comment was accurate when it was written, and the property was tested, against the code the diff replaced.


## A test that names a flag, a command or a field it does not care about

Found on dinah-97, where trunk's dinah-287 renamed the "state" flag to "column" and five test functions fell over at once. The tests were not about --state. They needed some flag that takes a value, so that a --lang written into its value slot could be shown not to be a language choice. Any valued flag would have served.

The danger is not the breakage, which is loud and cheap to fix. It is the shape of the breakage when it does not break. A test naming a retired flag still compiles, and it often still passes, because the parser reads an unrecognized word as an unknown flag rather than as a valued one, so the value slot the test is about never opens and the assertion falls back to a case every other line already covers. The guard goes quiet and reports nothing.

Wrong:

```go
{
    name: "lang in state's value slot is state's value",
    argv: []string{"move", "card1", "--state", "--lang", "de"},
    want: "",
},
```

Right:

```go
domain, session := exampleValuedFlags(t)
...
{
    name: "lang in " + domain + "'s value slot is " + domain + "'s value",
    argv: []string{"move", "card1", "--" + domain, "--lang", "de"},
    want: "",
},
```

where the helper reads the same table the parser reads, excludes the flag under test by name, and fails outright rather than skipping when it finds no candidate:

```go
func exampleValuedFlags(t *testing.T) (domain, session string) {
    t.Helper()
    for _, name := range valuedFlags {
        if name == "lang" {
            continue
        }
        if sessionFlagNames[name] {
            if session == "" {
                session = name
            }
            continue
        }
        if domain == "" {
            domain = name
        }
    }
    if domain == "" || session == "" {
        t.Fatalf("no example valued flag to read a value slot with: valuedFlags is %v, from which the domain pick is %q and the session pick is %q", valuedFlags, domain, session)
    }
    return domain, session
}
```

Three conditions make the derived form safe, and a derivation missing any of them trades one silent failure for another.

- Exclude the thing under test. A derived pick that lands on the very flag the test is about asserts something else entirely, so name the exclusion in the loop rather than hoping the sort order keeps it away.
- Say which one you chose. Compose the subtest name from the pick and put the pick in every failure message, so a reader who was not there can tell from the output which flag the run used.
- Fail when the derivation comes up empty. A helper that returns "" and lets the test carry on is the same silence in a new place.

When to keep the literal name. The rule is about a name the test does not care about. Where the identity is the subject, as in a test that --lang itself is honoured or that the help page lists --column, spell it out; a derived name there would hide what is being asserted.

The two questions that separate the cases: does this test still mean the same thing with a different name in that slot, and would it still pass if the name it holds were retired tomorrow. A yes to both is a test that should read the table.


### The same shape in prose, where nothing compiles and nothing runs

Caught on the review of the entry above, in the same card and one file over. The tests were repaired by derivation, and the doc comment that explains why they are written that way went on spelling the retired flag:

```go
// Sharing walkFlags with parseArgs, rather than a second pattern match
// against the literal word "--lang", is what keeps this scan honest about a
// word that belongs to somebody else: dinah move card1 --state --lang de
// sets --state's value to the literal text "--lang" and gives the caller no
// --lang at all ...
```

A comment has no compiler and no assertion, so the silence here is total. Worse, the example inverts once the name retires. An unknown word opens no value slot, so the following `--lang` is read after all, and the comment now tells a reader the opposite of what the function does for the one invocation it chose to illustrate. The reader most likely to check it is the next person asking whether the shared walk is worth its complexity, and the example they test will contradict the reason given for it.

Right: when a rename drives a test to derive a name, read the prose around that test in the same pass, and give an illustration a name that either is the subject or is derived in the reader's head. Where a comment needs a concrete invocation, prefer one built from a flag the tool's own contract pins, and where no such flag exists, say "some flag that takes a value" and let the test hold the example.

The general rule this adds: a sweep prompted by a rename covers the prose that explains the code, not only the code the compiler checks. Grep the retired name across the whole diff rather than across its test files.


## A rename that rotates two words, swept only where the retired word was replaced

Caught on dinah-31, Agent Code Review, in `cmd/dinah/compact_test.go`. The entry "A vocabulary rename applied to every occurrence of the word, including the ones that were ordinary English" records the other failure direction of the same rename and prescribes a second pass over the lines the sweep replaced. That pass cannot find this one.

A rotation is a rename that retires no word. dinah-287 moved `state` to `column` and moved `substate` to `state`, so `state` came through the rename still current and carrying a new meaning. An occurrence the sweep failed to match therefore stays compilable, stays grammatical, and stays a word of the live vocabulary, and the only thing wrong with it is that it now names something other than what the writer meant. A pass that reads the lines the sweep replaced reads none of these, because these are exactly the lines it did not replace.

Wrong, from the diff as it stands:

```go
// TestTheCompactOffersCarryEveryFieldTheCanonicalOffersCarry asserts dinah-31
// AC-4: each state's offer decodes to the same field values in the same order
// under both machine forms, including whether the offer carries a card, over a
// fixture holding a state with a card to offer, a state with nothing ready and
// a state that waits on somebody outside.
```

A state is now one of ready, active and blocked. A state does not offer a card, does not hold cards, and does not wait on somebody outside, so all four phrases name nothing. The comment stands above the test that proves the compact `off` record's first field is read from `Offer.Column`, so it tells a reader auditing that proof that the field holds a state, which is the one wrong mapping the review was looking for.

Right:

```go
// TestTheCompactOffersCarryEveryFieldTheCanonicalOffersCarry asserts dinah-31
// AC-4: each column's offer decodes to the same field values in the same order
// under both machine forms, including whether the offer carries a card, over a
// fixture holding a column with a card to offer, a column with nothing ready
// and a column that waits on somebody outside.
```

**The test:** grep the reused word across every file the card touched, and read each hit for which of its two senses it carries now. Do not grep the retired word, since a rotation retires none. Sort the hits into the new domain sense, the ordinary English sense, and the retired domain sense, then fix the third group. Where one file describes the same fixture in a comment the sweep replaced and again in a comment it missed, the two descriptions disagreeing is the cheapest available signal that the third group is not empty.


## A stand-in pinned for determinism, chosen without measuring what it replaces

Caught on dinah-31, Implement, in `scripts/measure_compact_tokens.py`, before the figure it produced reached anybody.

A measurement that rebuilds its fixture on every run carries fresh random identifiers, and a byte-pair encoder segments random hex unpredictably, so the same binary reported a saving that moved by several points between runs. The fix is to replace each re-rolled span with a fixed stand-in before counting. The trap is in choosing it. A stand-in that looks arbitrary is not the same thing as a stand-in that costs what the real value costs, and the tokenizer is the only judge of the second.

Wrong:

```python
PINS = (
    (re.compile(r"sha256:[0-9a-f]{64}"), "sha256:" + "3f7a1c9e05b2d846" * 4),
    (re.compile(r"\b[0-9a-f]{12}\b"), "1a2b3c4d5e6f"),
)
```

Both stand-ins are the right shape and the right length, and both are wildly atypical of what they replace. A repeated sixteen-character pattern costs 55 tokens where a real revision costs a median of 39, and the ascending-digit identifier costs 12 where a real one costs a median of 7. Those extra tokens land on both sides of the comparison, so they inflate the denominator without changing the difference, and the reported saving fell by about six points. Nothing about the run looks wrong. It reproduces perfectly, which is exactly what was asked for, and it reproduces a number that is not the one being claimed.

Right:

```python
# The identifier and the revision below are values a real run produced, and
# they were chosen because this tokenizer spends the median cost of their own
# distribution on them: 7 tokens for an identifier against a median of 7 over
# 2000 random ones, and 39 for a revision against a median of 39.
PINS = (
    (re.compile(r"sha256:[0-9a-f]{64}"), "sha256:451e40cab90727cb4a128e2326db5b720e294faa3cddf69b4341e4e0cdd39203"),
    (re.compile(r"\b[0-9a-f]{12}\b"), "fa68cbea8361"),
)
```

**The test:** when you freeze a varying value to make a measurement reproducible, measure the stand-in against the distribution it stands for, under the same instrument the measurement uses, and record the comparison beside the constant. Prefer a value a real run produced over one you invented, and pick the one whose cost sits at the median rather than the first that came out of the sample. The class is wider than tokenizers: any pin that normalises an input for determinism can move the level of what it measures while making the noise go away, and the vanished noise is what makes the shift easy to miss.


## Line citations repaired in the checklist items a review named, and left standing in the items it did not

**Wrong:** a review finds that an acceptance criterion cites `file.go:247` for an assertion that has moved, and the implementer repairs that criterion and one decision note beside it, replacing both numbers with the assertion's own failure message. The other decision notes on the same card, and the spec's own references into the same files, are never opened, so they go on naming lines that a hoist and two rounds of edits have already moved. One note ends up citing a function at a line one hundred and twenty-nine lines above where that function now starts, which lands inside a different function that looks plausible to a reader who does not open it. Nothing reddens, because a checklist note is prose and no test reads it, and the careful repair of the two named items is what makes the card look handled.

**Right:** a review finding about a stale reference is a finding about the card's whole reference set, not about the two items the reviewer happened to open. Repair every citation on the card in the same pass, and find them by scanning the card's spec and every checklist note for the pattern of a filename followed by a digit rather than by recalling which items carry one. Where the target is an assertion, cite it by its failure message, which no edit above it can move; where the target is a function, cite it by name. A number is worth keeping only where nothing else identifies the target, and then it is re-derived rather than carried forward.

**The test:** name every citation the card carries and say for each one how you resolved it. A citation you did not open is one you did not check, and a reviewer who resolves the two the last review named will find the rest still pointing wherever they used to.

## A repair applied at every producer of a field, armed at the one the reviewer's reproduction happened to name

**Wrong:** splitting an overloaded wire field into two, changing all five producers that set it, and writing the guard against the single command the reviewer's push-back reproduction used. On dinah-281 a verb-level refusal moved off `bench.Candidate.Refused` onto a new per-member `Unanswered` field, edited identically in `TreeForest`, `StatusForest`, `ListForest`, `NextForest` and `changesFor`. The new test drove `dinah ls --root <fixture> --column nosuchcolumn`, which is the command the previous review had run, and asserted both the field that must be set and the field that must not. Putting the refusal back on `Refused` in the other three producers left the whole suite green, so the defect the split exists to prevent was reintroducible in three of the five places it was fixed, and `dinah next --root <fixture> --column nosuchcolumn` and `dinah changes --root <fixture> --column nosuchcolumn` both reach it on the identical fixture the guarded leg already builds.

**Right:** when one edit is made at N call sites, enumerate the sites and run the guard over the ones a fixture can reach, as a table keyed on the verb rather than as a single case. Where a site cannot be reached, say so in a comment at that site and say why, because an assignment no invocation can execute is either unreachable code or a gap the next reader will assume is covered. The arming proof is read the same way: breaking one producer and watching one test redden proves that producer, and it proves nothing about the four spelled beside it.

**The test:** grep the tree for the assignment the repair introduced, count the sites, and count the distinct sites the suite exercises. When the second number is smaller, name each unexercised site and say whether a fixture reaches it. Sibling entry: "A copy-path fix verified against the command that was named, not the class it belongs to" applies the same reasoning to a prose claim, where the resource is a file rather than a field.

## A call site declared unreachable on the strength of the provocation that first came to mind

**Wrong:** taking "no fixture reaches this site" as a fact about the code when it is a fact about the two invocations you tried. On dinah-281 the review that found a repair guarded at one producer of five judged two of the remaining four unreachable and asked for a comment at each saying so, reasoning that `StatusForest` takes no column reference and that `TreeForest` refuses a bad axis once ahead of its loop. Both readings are true and neither settles the question. `tree` carries a column reference in its query rather than in a flag, and a query term is resolved inside each workbench, so `dinah tree 'column:nosuchcolumn' --root <fixture>` reaches the site on every row. `status` takes no reference at all, and a card whose header carries no column key is a card written in the vocabulary the rename retired, which `bench.Open` does not read and `Library.Status` does, so that site is reachable too. Writing the two comments as asked would have put a false claim of unreachability at two live sites and left the guard covering three of five.

**Right:** an unreachability claim is a probe, not an inference. Before writing the comment, run the site: read what the function can return an error from, then build the invocation or the fixture that provokes each of those, and only write the comment when the provocations are exhausted rather than when the obvious one fails. Two provocations were enough here, and each took one command against a binary already built. Where a site really is unreachable, the comment says which provocations were tried and why each is refused earlier, so the next reader can extend the list rather than start it.

**The test:** for each site you are about to call unreachable, name the invocation you ran and quote what it answered. A claim with no command behind it is a guess, and a guess written into a comment at the site is worse than the gap, because it tells the next reader the question is settled.

## An instrument that answers zero for input it cannot read

**Wrong:** shipping a text instrument whose tokenizer recognises only the script it was developed against, and letting it report a clean result for every other script. On dinah-311 the over-eager-rename sweep splits a line into words with `unicode.IsLetter`, which is false for the Devanagari virama, anusvara and vowel signs, so every Hindi word shreds into single consonants and no token ever equals the word being searched for. Pointed at the same card's own Hindi rename, which moved 99 occurrences from स्तंभ to कॉलम, the tool answered `0 replacements of "स्तंभ" by "कॉलम", in 0 groups`. A German control over the same corpus answered 92 replacements in 30 groups, so neither the range nor the invocation was at fault. The document shipped beside the tool mints a rule requiring the next rename card to run the pass and report its group count, so the first non-Latin rename to follow that rule reports zero and reads it as clean.

**Right:** an instrument states the input it can read and refuses the input it cannot, rather than answering for it. Where the repair is one line of tokenizing, fold the combining marks into the word they belong to. Where that is genuinely out of scope, tokenize the search term itself before the run and refuse when it does not come back as a single token, because a term the tokenizer cannot represent can never match and the run is guaranteed to answer zero. A zero meaning "nothing found" and a zero meaning "I cannot read this" must not be spelled the same way.

**The test:** run the instrument against the corpus in every script the repository actually carries, and do that before writing down what the instrument does not do. This repository ships catalogs in German, Hindi and five skeleton languages, so a Latin-script trial is a trial against one of eight. When an instrument answers zero, prove the zero by feeding the same range a term you already know is present.

## A refusal that tests whether the tokenizer splits the term, on a tokenizer that can also merge it

This is the entry "An instrument that answers zero for input it cannot read" one layer deeper, caught on the fix written to satisfy that entry. Keep both: the first names the defect, and this one names the shape a repair takes when it closes only the direction the reproduction happened to use.

A word-boundary tokenizer can be wrong about a script in two directions. It can break one word into several, and it can run several words together into one. A guard built by tokenizing the search term and refusing a term that comes back as more than one token sees the first direction and is blind to the second, because a term that merges comes back as exactly one token and passes the guard. The run then reports the same zero the guard was written to abolish.

**Wrong:** the over-eager-rename sweep refused a search term whose tokenization did not return a single token identical to the term, on the stated ground that "no list of scripts can be complete, and the next thing the tokenizer cannot represent will meet a refusal rather than a zero." A script written without spaces meets neither. Two occurrences of 工作台 renamed to 板块 tokenize as parts of longer runs of Han characters, the term itself tokenizes as one token and clears the guard, and the sweep answers `0 replacements of "工作台" by "板块", in 0 groups by surrounding phrase, rarest first` with exit status 0. The document shipped beside the tool states the guarantee as universal, so a reader holding that sentence has no way to tell this zero from a clean tree.

**Right:** when a guard is built to distinguish "found nothing" from "cannot read this", enumerate the ways the reader can be unable to read, not the one way the reproduction demonstrated. Where the remaining ways cannot be closed in code, state the residual case in the limits the document publishes and do not write the guarantee as universal. A cheap script-independent backstop is available for this shape: when the run reports zero, scan the raw diff for the retired term on a removed line and the adopted term on an added line, and refuse rather than report when both are present, because the instrument can then see a rename it could not read.

**The test:** for any tokenizer-shaped guard, write down what the tokenizer does to input it handles badly, then check that the guard fires on each outcome rather than on the one you reproduced. Splitting and merging are the two outcomes a word tokenizer has. The neighbouring entry "An invariant asserted from one side only" gives the general form of this mistake, and this is its instance in text tooling.

## A guard switched off by a diagnostic about a different part of the same run

This is the third catch in one lineage on a single instrument, after "An instrument that answers zero for input it cannot read" and "A refusal that tests whether the tokenizer splits the term, on a tokenizer that can also merge it". Those two name holes in what a guard tested. This one names a hole in when the guard runs at all.

A backstop that fires on an empty result often carries a second condition meant to keep it quiet where the tool has already warned the reader. When that condition is computed over the whole run instead of over the region the backstop protects, a warning raised about one file suppresses the backstop everywhere else, including in the file that needed it.

**Wrong:** the over-eager-rename sweep refuses a zero the raw diff contradicts, and gates that refusal on the run having declined nothing, on the stated ground that a declined run is already named on the report's own face so the reader has been told. The gate reads a field aggregated across the whole result. A range carrying a Chinese rename the tokenizer cannot read, together with any unrelated block large enough to pass the alignment cap, reports `0 replacements of "工作台" by "板块", in 0 groups by surrounding phrase, rarest first` and exits 0, carrying one unaligned-run line that names a different file entirely. A reader who follows the published procedure opens the run that line names, finds the file it points at, and learns nothing about the rename that was missed. The document's own limit says the run "still cannot answer zero for a rename it could not read", and this run does.

**Right:** scope a suppressing condition to the region it explains. A warning about one file justifies silence about that file and about nothing further, so gate such a backstop per file or per run rather than on a whole-result field. Where the aggregate gate is kept deliberately, because narrowing it costs more than it returns, write the published limit so that it describes the gate the code actually has.

**The test:** for any guard carrying a "stay quiet, the reader was already told" clause, ask what the reader was told and about which part of the input. Then construct the case where the telling and the defect sit in different parts, and check that the guard still fires.


## A citation naming an identifier that exists and carries a different rule

**Wrong:** a doc comment, spec paragraph or checklist note attributes a rule to a normative statement whose identifier is real but whose text says something else, usually a neighbour of the statement that does carry the rule. A doc comment on `Column.States` explained that a column carrying a kind the build does not implement is read as a work column, "which CORE-STATE-11 reads as a work column". CORE-STATE-11 requires each column to carry exactly one kind, either one the profile declares or one carrying a layer's prefix, and it is the refusal rule. The reading rule is CORE-STATE-12, one line below it, and the card's own ratified decision record cited CORE-STATE-12 correctly. Every mechanical check passes. The identifier exists, so a sweep for undeclared identifiers finds nothing; the guard that reads documentation against the declared statement list is satisfied; and a reviewer who confirms that the citation resolves has confirmed only that the target is real.

**Right:** resolve a citation by reading the cited statement's own text against the sentence that cites it, rather than by confirming that the identifier exists. Where the card's spec already cites a statement for the same rule, check the code against the spec's identifier instead of deriving one again from memory, because a disagreement between the two is the cheapest signal available and the spec's identifier was ratified. Adjacent identifiers in a numbered series are where this happens, since a rule and the refusal that guards it are usually written side by side and only one of them says what the citing sentence claims.

**The test:** for each citation, quote the cited statement's text beside the sentence that cites it and ask whether the first entails the second. An identifier that resolves is not a citation that holds.

## A review finding repaired at the sites the reviewer listed, when the finding names a class

A reviewer who catches a recurring defect has to list somewhere, so the finding names the instances the reviewer happened to open. An implementer who repairs exactly that list has repaired a sample. The entry that produced the finding usually states the class in its own Right clause, so the scope is on the page and is read as illustration rather than as the instruction.

This is the general form of "Line citations repaired in the checklist items a review named, and left standing in the items it did not". That entry states the rule for citations. The rule is not about citations.

**Wrong:** a review names a colon-splice in one string constant and stale line numbers in eight acceptance-criterion notes. The implementer fixes that constant, and rewrites all ten acceptance-criterion notes rather than only the eight, which reads as going beyond the finding. Three colon-splices in docstrings added by the same diff survive, and two decision notes on the same card go on citing line numbers into the same file. Nothing reddens, and the generous treatment of the named list is what makes the card look thorough.

**Right:** read the finding for its class, then enumerate the class mechanically before repairing anything. A style tell is found by scanning every added line of the diff for the tell, not by fixing the site quoted. A stale reference is found by scanning the card's spec and every checklist note of every kind for a filename followed by a digit, not by opening the items the reviewer listed. Where a scan turns up a site the class covers and the change deliberately leaves alone, say which sites those are and why, so a reader can tell a considered exemption from an unopened file.

**The test:** state how you enumerated the class. A repair note that lists the sites it fixed proves a sample was fixed. A repair note that names the scan, its pattern and its full result proves the class was. Where the two lists differ, the difference is the finding the next review would otherwise make.

## A stale copy of a figure repaired by adding a second unguarded copy of it

**Wrong:** `docs/spec/core-profile.md` declares its own revision in two places, one held by a test and one held by nothing, and the unheld one sat at `dinah-core 0.5` for two revisions after the tree moved past it. The card that corrected it also inserted a changelog preamble opening "The current revision is `dinah-core 0.7`.", which is a third statement of the same figure, in the same file, read by no guard. The hand-off note that card left for the author of the next revision bump names the two older sites and not the one the card had just created.

**Right:** a figure a card is repairing because it drifted is a figure that card must not leave a fresh loose copy of. Where the wording genuinely has to carry the number, extend the guard that already reads the file so the new site is asserted in the same act that creates it. `internal/profile/amendment_test.go` already holds one sentence of this document with `strings.Contains` over the whole text, so a second sentence costs one constant and one assertion, and the plant that arms the first arms it too.

**The test:** when a card's subject is a value that went stale, count the places the finished diff states that value and count the places a guard reads it. If the first number grew and the second did not, the card moved the defect instead of removing it. Ask the same question of the hand-off note, because a note telling the next author which sites to carry forward is already wrong if it omits a site the note's own card added.

Related: "A count maintained by hand in more than one place" carries the general rule. This entry records the shape where the repair is what creates the hand-written copy, which is the case the general rule reads past, because there the copy looks like the fix.

## An accounting whose stated total covers an item its own enumeration never names

**Wrong:** closing a line-by-line attribution with a sum, and letting one addend stand for more items than the list beside it names. On `dinah-203` the criterion that certifies every surveyed line is ruled recorded its accounting as "5 correct-today declarations, 4 alias lines (`format.md:1374,1375,1376,1378`), 13 in `core-profile.md` ... 5+5+13+8+1+4+1+1+8+1 = 47". The sum is right and the tree is right. The enumeration is not: its second entry names four lines and its second addend is five, and the line that closes the gap, `format.md:1361`, appears nowhere in the note. That line was in fact ruled, in a different section, so nothing shipped wrong. The same note also asserts "none ruled twice" while the artifact it certifies deliberately rules one line under two groups and says so.

This is the failure the same card had already been pushed back for twice, moved up one level. Rounds one and two found a classification bullet whose stated reason covered a line the reason did not describe. Round three found the accounting of that classification doing the same thing with a total instead of a range. A container that implicitly covers a member its explicit reason never names reads as complete either way, and the arithmetic agreeing is what makes it read as checked.

**Right:** an accounting is checked by matching its parts to its members, never by adding it up. Write the enumeration so that every addend is the length of the list printed beside it, and where an item is genuinely ruled twice, say which item and why rather than asserting the negative. A stated method that forbids an exception the result then takes needs the method amended in the same edit, not the exception disclosed three paragraphs away.

**The test:** when a note reports a total over an enumerated set, count each addend against the items named for it before you check the sum, and read every "none of these" claim against the artifact rather than against the note. A sum that adds up proves the author did arithmetic. It proves nothing about attribution, which is the thing the accounting exists to establish.

Related: "A review finding repaired at the sites the reviewer listed, when the finding names a class" is the same defect at the level of the fix. This entry records it at the level of the record that certifies the fix, which is the layer a reviewer reaches last and trusts most.


## A doc comment left standing where an extraction inserted a new declaration beneath it

**Wrong:** lifting a shared check out of a function into a new helper, and placing that helper between the original function and the doc comment that belongs to it. The comment is now adjacent to the helper rather than to the function it describes, so the function is left undocumented and the helper carries two stacked blocks. Nothing fails when this happens: the compiler, the linter and the test suite are all indifferent to which declaration a block comment precedes, so no gate reports it. A reviewer reading the diff sees the comment's text edited correctly, because the edit itself is correct, and does not notice that the text moved relative to nothing while a declaration moved underneath it. The result is worse when the edited text cites the new helper by position, because a paragraph reading "the absent element is checked by isRow above" ends up sitting above isRow and contradicts itself.

**Right:** an extraction that adds a declaration puts that declaration and its own comment after the comment belonging to the existing function, or moves the existing comment down so it stays against its own function. Verify placement by reading the file rather than the diff, because adjacency is a property of the file and a hunk-by-hunk view never shows it. The mechanical check is that every doc comment is followed by the declaration it names before any other comment begins, so two block comments in a row are the signal to look.


## A repair that adds a new claim while correcting an old one

Caught on dinah-314, second Agent Code Review. A review found a false sentence in a design document and pushed the card back to fix it. The repair corrected every claim the review named, and while rewriting the sentence it added one more clause that nothing had asked for and nothing had checked. The clause was false, and the same document contradicted it a thousand lines further down.

The sweep the repair ran was real: it walked every statement in the file on the axis the review named. What it could not do is walk statements on an axis the repair itself introduced, because that axis did not exist until the repair wrote it. A sweep runs before the edit; the new claim arrives after.

The test is to re-read your own replacement sentence as though somebody else had written it, clause by clause, and ask of each clause what would have to be true for it to hold and where in the tree that is decided. A clause you added for rhythm or for closure is the one to be most suspicious of, because it was never derived from anything.

WRONG, from `docs/design/format.md`, where the first clause was derived from the code and the second was not:

> Eighteen names sit in both counts, and those eighteen are every core event a command writes onto a card.

The eighteen is right. The characterisation is wrong by exactly one: `deleted` is in both counts and is written to the workbench's journal rather than to a card's, because deleting a card destroys the journal inside it. The same document states that in its own words in the Corruption and recovery section.

RIGHT, either of:

> Eighteen names sit in both counts.

or, if the characterisation earns its place, derived rather than asserted:

> Eighteen names sit in both counts. Seventeen of those land on a card's own journal; `deleted` is the exception, since deleting a card destroys the journal inside it and the record goes to the workbench's.

The neighbouring entry "A republished statement checked only against the statements that name it" governs the sweep over statements that already exist. This one governs the statement the repair is in the act of writing.


## A checker that requires the right sentence and cannot see a wrong one added beside it

Caught on dinah-314, third Agent Code Review, and it is the second lap of the same class on one card. The round before, a false clause escaped a criterion that checked three cardinalities and three memberships, because no clause of the criterion reached it. The repair added a fourth check and then described the checker as rejecting the whole shape of clause that had escaped. It does not, and a reviewer who ran the checker rather than reading its description found that out in one run.

The shape is a checker built out of "the document must carry a statement matching this pattern", where the patterns are generated from the code. That construction is sound for what it does: a required claim that is missing reddens, and a required claim that is wrong reddens, and both stay correct when the code moves. What it cannot do is notice a sentence nobody required. Adding text is invisible to a presence test, so the one defect shape the checker was written in response to, a clause volunteered for closure that nothing derived, walks straight past it.

The trap is that arming looks complete. Every plant a writer thinks of is a plant that breaks a sentence the checker already looks for, so every plant reddens, and the run that would have falsified the description is the one nobody performs: leave the correct paragraph exactly as it is and append a further sentence.

**Wrong**, from the criterion's own text, describing the committed checker:

> A closing clause characterising the overlap set without naming a number or an event, which is how the previous round's false clause escaped every check above, fails this criterion.

**Right**, either strengthen the checker until the sentence is true, or say what the checker does and name the boundary:

> Each check asks whether the section carries a statement matching a pattern the derived sets build, so it catches a required claim that is missing or wrong, and it does not catch a further false sentence added beside a correct one.

**The test.** For any checker whose checks are "this must be present and correct", plant the addition, not only the corruption. Leave every required statement untouched and append one sentence the checker was never told about. If it still exits zero, the checker is a presence test, and any description of it that speaks about sentences in general is an over-claim.

**Where it bites hardest.** A checker whose whole purpose is to stop a document drifting, because the reader's next move after exit zero is to stop reading. A tool that catches the corruptions and misses the additions trains its user out of the one habit that would catch the additions.

The neighbouring entry "A universal claim generalised from the cases the reviewer named" governs the quantifier itself. This one names the construction that makes the quantifier plausible, and it says which run falsifies it.


## A mechanical sweep whose recorded output a later stage reads instead of re-running

A spec that replaces a hand-built list with a command and that command's output has repaired only half of the problem. The command is reproducible, and the output printed beside it is still a snapshot that goes stale the moment anything lands between the stage that took it and the stage that reads it. A recorded output reads as evidence rather than as a copy, and that is what persuades the next stage that re-running it would be redundant.

**Wrong:** dinah-346's spec recorded its sweep for hand-computed exit-2 sites as "three other sites exist, all in `runMCP`". A fourth site, `reportError` in `cmd/dinah/main.go`, had been in the tree the whole time. The same card had already been pushed back once for the same shape one layer down, where a hand-built list of the tests asserting the old exit code named three of the five, and the agreed repair was to record a mechanical sweep's output in the spec instead. The second record was as incomplete as the first.

**Right:** write the criterion so the later stage runs the sweep itself and reports what it finds, and read the spec's recorded output as an illustration of the command's shape rather than as the answer. Where the criterion holds its results against an allow-list, the allow-list is the durable artifact and the snapshot is not, so the allow-list is what a review resolves. dinah-346's AC-7 got this part right: its allow-list named `reportError` even though the prose above it did not, so the fresh sweep's extra site widened nothing and cost nobody a cycle.

## An arming break that does not compile, read as a suite that stayed green

This is a third sibling of the two arming-break entries above. There the break ran and the wrong assertion took the credit, or the break ran and reddened nothing. Here the break never ran at all, and the report that follows is not merely unsupported: it records a proof that did not happen.

**Wrong:** planting the arming break, running the suite through a grep for its failure marker, and reading the empty result as a verdict. On dinah-330 the break replaced a read of the CLI's published `outcome` member with a read of the findings array, which left the constant that member was compared against unreferenced. TypeScript's unused-local error stopped the build, the test command exited before a single test ran, and a `grep -E "^not ok"` over that output printed nothing. Nothing is exactly what a fully green suite prints too. The proof was one keystroke from being written up as "the guard did not fire, so it must be held elsewhere," which is the conclusion the sibling entry above already warns against, reached this time from a run that never executed.

The same shape reaches further than arming proofs. Any check whose pass condition is the absence of output shares it: a filtered test run that matches no test reports "no tests to run" and exits zero, a linter pointed at a path that does not exist has nothing to complain about, and a grep over a file that failed to open is silent.

**Right:** read the run's own count of what it did before reading what it found, and treat a run that reports no work as a failed run rather than a clean one. Every break in the same proof then names both numbers, so a reader can tell a suite of two hundred with one failure from a suite of zero. A break that will not compile is not a smaller version of a break that fails a test; it is a different event, and the honest note says the build broke and the break was rewritten into a form the compiler accepts. On dinah-330 the rewrite kept the constant referenced, the suite ran its two hundred, and one assertion went red by name.

**The test:** if your evidence that a check passed is that it printed nothing, ask what the same command prints when it never ran. If the two are the same string, the evidence is worth nothing until you add the count.


## A locale impact count taken from a search's hits rather than from the catalog directory

**Wrong:** counting how many catalogs a message change reaches by reading the files a search happened to surface. A review of `check.ordinal-missing` reported that the key "lives in six catalogs: `af`, `cs`, `en`, `es`, `fil`, and `id`". The key is in eight. The two the count omitted were `de` and `hi`, and those two were the only catalogs carrying a real translation; the other five carry the English verbatim under `skeleton: true`. The count was wrong in the direction that matters, because it omitted all of the translation work and none of the copying. A reader who trusted the six would have concluded the change was five mechanical pastes plus the English, which is a change with no translator in it at all, and would have sized the follow-on work from that.

**Right:** enumerate locale impact from the directory, never from a search result. `ls internal/msg/locales/` gives the catalog set, and a `grep -l` for the key run over that whole set gives which of them carry it. Then split the answer by `skeleton`, because a skeleton entry and a translated entry are different acts on different guards: the skeleton takes the new English verbatim and `TestASkeletonEntryReallyCarriesTheEnglishText` holds it there, while a translation is rewritten in its own language and its `source` fingerprint is recomputed, which `TestATranslationTracksItsEnglishSource` holds. Report both numbers, not their sum.

**The test:** state the denominator. A count of catalogs that never says how many catalogs exist is a count nobody can check against anything. When the reported figure happens to equal the number of files the author opened, it is a sample wearing a total's clothes.


## A correction recorded where it was found, in a store that does not gate

A card's spec prose and its checklist are two stores, and editing one never touches the other. When a stage discovers that a claim it inherited is false, the natural place to record the discovery is wherever the stage is already writing: a handoff note, a verification note, a commit message. Those are the stores nothing reads at the gate. The checklist item is what a later stage is held to, and it keeps the false text while the record beside it says the text is wrong.

The failure is not a missing correction. The correction exists, it is accurate, and it is easy to find. What is missing is the edit to the store that decides whether the card may move.

**Wrong.** An acceptance criterion asserts a property of two named functions, and the property holds for one of them. The implementer reads the code, finds the discrepancy, and writes it into the criterion's verification note: the spec says both functions probe their own root, only one does, no code changed on either side of this. The criterion's own text still asserts the property of both, and the criterion is marked verified. Nothing any test can do satisfies it as written, and the next reader who resolves the citation finds the note explaining that the sentence above it is false.

**Right.** Edit the store that gates, and record in the note that you edited it and why. Where the stage lacks the standing to change a criterion, say so in the note and name the item, so the correction arrives as a finding rather than as a footnote. Where the false claim reached more than one store, which it usually has, fix every store rather than the one you were reading: a claim that travelled from spec prose into a decision and into a criterion has three copies, and correcting one leaves two.

**The test.** After writing a correction, ask which sentence a later stage will be held to, and whether that sentence now says the true thing. If the corrected claim survives only in a note, the correction has not landed.


## A ship state cleared against one of the two guards that can refuse it

Named on dinah-297, where a spec ruled that a new catalog key could ship as an untranslated skeleton in German and Hindi, and a design review then checked that ruling against the catalog files and reported it correct. Both reasoned from `TestATranslationTracksItsEnglishSource`, which skips skeleton entries, so a skeleton cannot be stale and the ruling followed. Neither asked what else in the package looks at a skeleton entry. `TestEveryDeclaredLanguageShips` requires `translated == total` for every tag on `msg.Complete`, `Complete` is `{en, hi, de}`, and `Coverage` increments `translated` only for an entry whose `Skeleton` flag is false. A skeleton in either catalog therefore lowers `translated` while raising `total`, and the package fails to build green. Implement found it by running the suite, translated both entries, and recorded the supersession. The spec's statement is still wrong on the page.

The same error was already recorded once, in the other direction. The workbench document "Translation staleness contract" says outright: "Do not justify the exemption by claiming that `TestEveryDeclaredLanguageShips` already holds a skeleton entry to the English. That test counts keys and counts how many entries carry `Skeleton: false`, and it compares no bytes at all. The claim has been checked and it is false." One author reached for the completeness guard to answer a content question; a later author reached for the content guard to answer a completeness question. Neither read both.

**Wrong:** clear a ship state by naming the one guard that came to mind and reading its scope.

```
D-2: the new key ships skeleton in every locale including German and Hindi.
Reasoning: TestATranslationTracksItsEnglishSource only checks non-skeleton
entries, and a key that has never carried a translation cannot be stale.
```

**Right:** ask which checks read the field you are about to set, rather than which check you were thinking about. `Skeleton` is read by the staleness guard, by `TestASkeletonEntryReallyCarriesTheEnglishText`, by `TestATranslationIsNotEnglishUnderAnotherTag`, and by `Coverage`, which `TestEveryDeclaredLanguageShips` counts. Enumerate the readers from the code, then state the ruling against all of them.

**The test:** grep for the field or flag your decision sets and list every test that reads it. If your reasoning names fewer readers than the grep returns, you have cleared the state against a subset. A verification that confirms the author's own guard and stops is indistinguishable from one that read every guard, and only the grep separates them.

## A dependency tree installed inside the checkout, where the language's own package walk finds it

Caught on dinah-359, where the build script began installing the VS Code extension's `node_modules` into the checkout so that it could package the extension on every run.

`go help packages` says that names beginning with `.` or `_` are ignored, that directories named `testdata` are ignored, and that a wildcard element never matches a `vendor` element. It names nothing else, so a `node_modules` directory is walked like any other. An npm dependency that ships Go source therefore joins the main module: `go test ./...` in a checkout where the script has run reports `dinah/editors/vscode/node_modules/flatted/golang/pkg/flatted`, a package nobody in this repository wrote.

Being gitignored does not help. Git's ignore rules and the toolchain's package walk have nothing to do with each other, and the directory is on disk either way.

This board had already paid for one instance before the second arrived. The CI gofmt job runs `gofmt -l cmd internal` rather than `gofmt -l .`, and its comment records the reason. The other Go commands in CI still run from the root, and they stay clean only because the job running them never installs npm dependencies. That is an accident of job layout rather than a property of the repository, and it does not hold on a contributor's machine.

**Wrong:** adding a step that materialises a foreign dependency tree inside the module, and handing the work off without saying that `go build ./...`, `go vet ./...`, `go test ./...` and the `gofmt -l .` the Go style standard prescribes will now walk into it.

**Right:** when a step installs a dependency tree inside the checkout, say which whole-tree commands will now see it, and scope those commands the way the existing gofmt job already scopes itself. Pruning the subtree so the toolchain skips it is the other route, and a nested module file is the usual suggestion, so confirm against the toolchain's own documentation that the version in use prunes what you think it prunes before resting on it.

**The test:** run the tree's whole-tree commands in a checkout where the new step has run, and compare the package list against a checkout where it has not. A command that passes in CI and fails on a contributor's machine is what this catches, and the two checkouts differ by exactly the step under review.

## A claim swept out of every place that names it, while a sentence resting on the same premise survives

Caught on dinah-359, third code review, 2026-09-01. The workbench already warns that searching a document for a phrase finds copies of the phrase rather than copies of the claim. This entry is the version of that failure which survives a careful sweep, because the surviving sentence is about a neighbouring subject and shares no searchable word with the claim being removed.

The operator ruled that `scripts/build-dinah.ps1` is not wired to a build keystroke, so two sentences explaining themselves by that keystroke had to lose their reason. The implementer swept for the key name, found a third site the ruling had not counted, and corrected all three. The sweep was honest and its result was right. What it could not reach was a fourth sentence, sixty lines above, which never names the keystroke and instead reassures the reader that "VS Code's task runner and this script's own tests redirect at the OS level instead". That sentence rests on the same premise the ruling removed, which is that this script is ordinarily run from inside the editor.

**Wrong**, in `scripts/build-dinah.ps1`, after the keystroke claim was removed from the three places that named it:

```
# Windows PowerShell 5.1 has been observed to turn a native command's stderr
# into a terminating error while that stream is redirected [...] VS
# Code's task runner and this script's own tests redirect at the OS level
# instead, so neither of them meets it.
```

**Right**, naming only the callers the repository can actually produce:

```
# Windows PowerShell 5.1 has been observed to turn a native command's stderr
# into a terminating error while that stream is redirected [...] This
# script's own tests redirect at the OS level instead, so they do not meet it.
```

How to catch it without a second reviewer. After you have removed a claim, do not close the file. List the premises the removed claim carried, which here was "somebody is running this from inside the editor", and read the whole file once for sentences that make sense only if a premise on that list still holds. The search that finds them is a search for a meaning rather than for a string, so it has to be a read.


## A required input whose refusal is satisfied by a word meaning "no constraint"

Caught on dinah-361, first Agent Code Review, 2026-09-01. A check that fails closed on a missing input still fails open on a present one that carries nothing, and the two are easy to confuse when the refusal is written and tested and the placeholder is not.

The promotion workflow validates that no cut holds back a card's predecessor. It reads the dependency pairs from a `links` input, and it refuses an empty input, with a comment saying that a check switching itself off when somebody forgets an input is absent exactly when it is needed. The reasoning is right and the refusal works. What it does not reach is the in-band value: `links` also accepts the single word `none`, meaning this cut has no predecessors, and nothing compares the pairs against the cards the cut carries. So one word disables the whole check for a cut of any size, and a cut naming six cards while declaring one pair silently asserts that the other five have no predecessors.

The gap survives review because the two tests that exist both pass and both look like coverage. One proves an empty input is refused. One proves a declared predecessor held back is caught. Neither can see that the second test's input is what the operator has to remember to type.

**Wrong.** The guard is non-emptiness, and the population the check is supposed to cover is never consulted.

```go
if spec == "" {
    return nil, fmt.Errorf("the links input is empty; pass ... or the single word none if there are none")
}
if spec == "none" {
    return nil, nil
}
```

Demonstrated rather than argued. Cutting one card that genuinely depends on another, with `--links dependent>predecessor`, refuses and names the pair. The same cut with `--links none` passes validation and proceeds to assemble a tree missing the predecessor. It failed afterwards only because the two commits happened to touch the same line, so a cherry-pick conflict caught by accident what the dependency check had been told to ignore.

**Right.** Require an answer for each member of the population rather than one answer for the whole population, so that a forgotten card is as loud as a forgotten input. Either take the declaration per card, which turns `none` into a statement about one card that the check can count, or carry a second input naming which cards were looked up, and refuse when the set of cards declared does not cover the set of cards being promoted. Where an all-or-nothing word has to stay, make the run report which cards it treated as unconstrained, so the operator reads the assumption instead of supplying it invisibly.

The test: can somebody satisfy this refusal without answering the question it asks? If a single fixed token clears it for an input of any size, the refusal is a formality, and the check is off whenever the person dispatching it is in a hurry.

Neighbouring entry: "A checker its own document can switch off" covers a check disabled by the shape of the text it reads. This one covers a check disabled by a value its own contract invites.


## A declaration that satisfies a constraint by naming itself

Caught on dinah-361, second Agent Code Review, 2026-09-01. This is the sibling of the entry above, found while testing the fix that entry asked for. Requiring an answer per card closed the one-word hole, and the answers themselves are still values a hurried or evasive dispatcher chooses, so the next question is which values a check accepts without learning anything.

The promotion workflow now takes one dependency declaration per card in a cut, written `dependent>predecessor` or `dependent>none`, and it refuses a cut naming a card the input says nothing about. It also prints the cards it was told have no predecessor, which is what makes a false `>none` readable in the run log afterwards. Both properties hold. What neither reaches is a declaration that satisfies the check out of the cut's own membership: `dinah-3>dinah-3` passes, because the predecessor check asks only whether the named predecessor is in this cut or already carried, and the card names itself. Worse, the card is then counted as constrained, so it drops out of the printed list of cards declared to have no predecessor. A tautology is quieter than an honest lie.

The general shape is a satisfiability check that tests membership rather than substance. Any declaration whose subject is drawn from the same set the check validates against can be closed by pointing the declaration at that set, and the check reports success because its literal question was answered.

**Wrong.** The predecessor is tested for presence in the cut, and nothing tests that it is a different card.

```go
for _, link := range deps.Links {
    if wanted[link.Predecessor] || carriedCards[link.Predecessor] {
        continue
    }
    violations = append(violations, ...)
}
```

**Right.** Reject a declaration that refers to its own subject, before the membership test runs, and say why rather than folding it into the generic refusal. A card cannot be its own predecessor, so the input is malformed rather than unsatisfied, and it should read as malformed.

```go
if link.Card == link.Predecessor {
    return nil, fmt.Errorf("%s is declared as its own predecessor, which no card can be; declare its real predecessor, or %s>none where it has none", link.Card, link.Card)
}
```

The wider lesson is where to look for the next instance. When a check validates one member of a relation against a set, ask what happens when the other member of that relation is drawn from the same set, and ask separately whether the mitigation that reports assumptions still fires on that input. Here it did not, and an echo that goes quiet on the input most worth echoing is the part that makes this shape worth a paragraph rather than a code comment.

## A migration that moves a directory must move the entity, not the directory that happens to hold it

Caught at Agent Code Review on dinah-285, cycle 1. The card's migration lifts a workbench that sits outside a `.dinah` container into one. The workbench's own members are `workbench.md`, `journal.ndjson`, `cards/`, `columns/`, `archive/` and `attachments/`, and the format explicitly allows those members to sit directly in a project repository beside source code and a `.git` directory.

**Wrong.** The lift renames the whole enclosing directory into a container created beside it.

```go
container := filepath.Join(filepath.Dir(path), UserBaseName)
target, err := freshTarget(container)
err = containerRename(path, target)
```

Run against a repository at `repo/myproject` holding `workbench.md`, `src/main.go`, `README.md` and `.git/`, this produces `repo/.dinah/<id>/` containing all four, and `repo/myproject` no longer exists. The user's source tree and its git directory are now inside what the tool calls a workbench, the container was created outside the repository entirely, and every sibling repository under `repo/` now discovers that workbench through the climbing walk.

**Right.** Create the container inside the directory that holds the workbench, and move the workbench's own members into it, leaving everything else where it stands.

```go
container := filepath.Join(path, UserBaseName)
target, err := freshTarget(container)
for _, member := range benchMembers {   // workbench.md, journal.ndjson, cards, columns, archive, attachments
    if !Exists(filepath.Join(path, member)) {
        continue
    }
    if err := containerRename(filepath.Join(path, member), filepath.Join(target, member)); err != nil {
        return "", err
    }
}
```

**Why the tests did not catch it, which is the part worth carrying forward.** The card required the migration to be proven by content rather than by structure, after a previous card's migration destroyed data while its own test passed. The implementer did exactly that: every file compared by the SHA-256 of its bytes, keyed on the path relative to the workbench root, over a fixture carrying a card, a journal, a comment, an attachment with a payload and an archived card. Every one of those digests still matched after the defective migration, because no byte of the workbench changed. What changed was everything around it, and a digest keyed on paths relative to the workbench root cannot see outside the workbench.

The fixture sealed it. The bare workbench under test was `populatedBench(t, filepath.Join(tree, "b"), ...)`, a directory holding the workbench and nothing else, so the enclosing directory and the workbench directory were the same directory and the defect had nothing to destroy.

So: a content proof answers "did the entity survive" and never answers "did anything else". When a repair moves or renames a directory, the fixture needs a sibling that the repair has no business touching, and the assertion needs to name that sibling by its original path. Add an unrelated file and an unrelated subdirectory beside the entity, and assert after the run that both are still exactly where they were.

## Repair advice corrected where a test tripped over it, and left standing in the sibling refusals that give the same advice

Caught on dinah-362, first agent code review, 2026-09-02. A card narrowed two repair sweeps from a downward walk to an upward climb. That narrowing took several catalog sentences with it, because each one recommends a bare invocation of a sweep whose reach the card had just cut, and a sentence recommending a command is only correct while that command still does the job from where its reader is standing.

The implementer found one of those sentences by running it. A test cut the command out of the rendered refusal, ran it, and went red, so the sentence was corrected and the correction shipped with the test that proved it. That work was right and the reasoning behind it was written down. What did not happen is the second step: the same grep that produced the one sentence produces the others, and the others were never run.

This is not the failure where a surviving sentence shares no searchable word with the claim being removed. Here the surviving sentences name the command in full, so one grep for the command spelling returns every one of them. The gap is that a violation discovered by tripping over it feels located rather than classed, and a fix aimed at where the test went red stops at the instance that reddened.

**Wrong:** narrowing what `dinah check --migrate-container` reaches, correcting `refusal.dinah.no-workbench-found.bare` because a new test ran its advice and failed, and leaving `refusal.dinah.needs-container-migration.next` recommending the same bare invocation in all eight catalogs. Following that surviving sentence from the directory holding the workbench it names now answers with a different refusal, and following it from inside that workbench answers with the refusal the first sentence was corrected to escape.

**Right:** when a change narrows what a command reaches, grep every shipped catalog for that command's spelling, and for each hit run the sentence from each position its own reader can be standing in. Correct all of them in one pass, or record why a particular hit is descriptive rather than instructive. The card that narrows the command owns every sentence that recommends it, not only the sentence whose test happened to fail.

**The test:** how many catalog entries name the command whose reach this diff changed, and how many of them did the diff either run or explicitly disposition? If the second number is smaller than the first, the sweep stopped at the instance that reddened.


## A disposition that records a reason about the code has to be checked against the code

Caught on dinah-362, second agent code review, 2026-09-02. The round before this one closed a class of broken advice by building a table that forces every catalog sentence naming a narrowed command to be either a promise some test follows or a sentence somebody has recorded as descriptive. The table is a good mechanism and the guard around it is armed. What went wrong is one rung in: two of the table's entries carry a reason that describes a rendering case the tool does not produce.

The two entries are the unqualified halves of an alternation. Each refusal now renders a scoped sentence wherever the head resolved a workbench, and an unqualified sentence otherwise, and the disposition says the unqualified half "renders only where the head resolved no single workbench, and that is a sweep over a tree the caller has already named". A tree sweep does not render a refusal sentence at all. It collects the refusal into a per-workbench report row carrying the refusal's name and no next step, which the review confirmed on a fixture tree. The whole cmd/dinah suite also stays green when both unqualified sentences are replaced with nonsense, so nothing renders them.

The reason this matters more than tidiness is that the fallback is live in the code while the reason for it is wrong. The climb path through the sweep helper resolves a workbench without recording it on the head, so a raise site moving onto that path would print the unqualified sentence, which is the broken advice the card exists to remove, and the guard would report the family as dispositioned.

**Wrong:** writing a disposition, an exemption or a doc comment whose reason names a runtime case ("this renders when X", "this fires when Y"), and checking it by reasoning about the code rather than by producing the case. A reason that reads as an argued decision and describes a case that does not occur is the same defect as a comment claiming a protection the code does not provide, which this very round removed one rung down.

**Right:** produce the case before you record the reason. Where the reason names a rendering, render it; where it names a refusal, raise it. When the case turns out to be unreachable, say that instead, and say why the unreachable branch is still there, which for an alternation is usually that the last member has to carry no condition.

**The test:** take each reason you wrote and ask what invocation you would type to see it happen. If you cannot type one, the reason is a guess wearing the clothes of a decision.


## A guard has to hold the premise, not a neighbouring one

Caught on dinah-362, Agent Code Review round three. A reviewer had asked for a claim to be held by a test rather than by prose. The claim was "nothing renders the unqualified fallback sentence". The implementer shipped a test asserting a different proposition, "one field has one writer", and then wrote in four places that the test held the first.

Wrong:

```go
// The claim those dispositions make is that nothing renders the unqualified
// fragment ... So the premise is held here rather than asserted in a comment.
func TestWorkbenchRootHasOneWriter(t *testing.T) {
	// counts assignments to s.workbenchRoot across the package's sources
	// and fails when there is more than one
}
```

The sentence renders when a refusal is composed on a path that never set the field. A second writer sets it, so a second writer can only ever make the sentence render less. The guard fires on the safe direction and is blind to the unsafe one, and the card's record went further and claimed the opposite: "a second writer ... fails the build instead of quietly making the unscoped advice reachable again". Adding four lines to the function the comment itself names as the nearest hazard reproduced the original blocker with the guard green.

Right, either of these:

```go
// A guard whose assertion is the claim itself. The climb form of the sweep is
// the path that would compose the refusal without an open, so run it and
// assert it reaches the preview rather than a refusal.
func TestTheClimbingSweepNeverRefusesTheWorkbenchItRepairs(t *testing.T) {
	tree, workbench := preVocabularyFixture(t)
	got := runCLI(t, filepath.Dir(workbench), "check", "--migrate-vocabulary")
	if got.code != 5 {
		t.Fatalf("the climbing sweep answered %d rather than reaching the preview: %s%s", got.code, got.out, got.errw)
	}
}
```

or, when no test can hold the claim, write down the claim the test does hold and say plainly that the other one rests on reading rather than on a guard.

The test for this class: state the guard's assertion and the claim it is said to hold as two sentences, then ask whether the first implies the second. Where it does not, the guard is real and the sentence beside it is not, and a reviewer arriving next round reads the sentence rather than the implication. Arming does not catch it, because a guard pointed at the wrong proposition still goes red when you break that proposition.


## A guard that reads source text reads one spelling of the thing it is guarding

Found on dinah-362, round four, in a test that held a single-writer premise by scanning the package's own sources line by line.

Wrong:

```go
assigns := strings.Contains(trimmed, ".workbenchRoot =") && !strings.Contains(trimmed, ".workbenchRoot ==")
if assigns || strings.Contains(trimmed, "workbenchRoot:") {
    writers[source+" "+function]++
}
```

Right:

```go
parsed, err := parser.ParseFile(token.NewFileSet(), source, nil, 0)
...
case *ast.AssignStmt:
    for _, target := range found.Lhs {
        if namesWorkbenchRoot(target) {
            writers[where]++
        }
    }
```

The text scan sees the field name only where it lands immediately left of the ` = `, so `s.workbenchRoot, s.workbenchSource = root, source` passes it and `s.workbenchSource, s.workbenchRoot = source, root` fails it, though the two write the same field. `+=` escapes for the same reason, and so does handing the field's address to something else. Both spellings are gofmt clean and vet clean, so nothing else in the toolchain objects, and the guard reports the premise as held.

An assignment is a shape in the syntax tree rather than a shape in a line, so the syntax tree is what answers the question. Reach for `go/ast` whenever a guard's subject is a Go construct rather than a literal string, and where a guard genuinely does read text, arm it against the spellings a reader would reach for first rather than only the one in the tree today.


## A source-walking guard rooted at the package's parent while its claim names the tree

**Wrong:** a guard in `cmd/dinah` that asserts something about every non-test source in the repository and walks `".."` to find them. A Go test runs with its working directory set to its own package directory, so `".."` is `cmd/`, and the walk covers `cmd/dinah` and its siblings and nothing else. On this repository that is eight of the fifty-four refusal raise sites; the other forty-six live under `internal/` and the guard cannot see any of them. The test passes, its doc comment says "every non-test source in the tree", and the disposition that rests on it reads as proven. Arming does not catch this either, because the natural place to plant a break is the file the author was already editing, which is inside the walked subtree.

**Right:** derive the walk root rather than spelling a relative hop, and prove the root is the repository. `go list -m -f '{{.Dir}}'`, or climbing until `go.mod` is found, both answer the module root from wherever the test happens to run. Then make the non-vacuity check count what the guard needs rather than what it read: a guard over the whole tree should refuse to pass when it has parsed fewer files than the tree holds, or when a package it names by hand is absent from what it walked, because `read == 0` is satisfied by reading one directory. Arm the guard by planting the break in the package furthest from the test, not in the file already open.


## A premise audited at the raise sites, where one site takes its values from its caller

**Wrong:** establishing that a fallback sentence never renders by reading every place the refusal is raised and checking what each one attaches. The reading is correct and the conclusion is still false, because a raise site can be handed its values rather than build them. On dinah-362 two of the malformed raise sites take their value map as a parameter, and one of the callers passed a path with no workbench beside it, so the sentence recorded as unrendered was the sentence that site produced. An audit that stops at the raise sees the parameter's name and moves on, and so does a syntactic guard written from the same reading, which is how the audit and its guard agreed with each other and both missed it.

**Right:** when a claim is about the values a refusal carries, follow the values rather than the raises. Where a site takes its map as a parameter, check the call sites, and make that redirection carry its own weight: a guard naming a function whose callers it will check has to fail when it finds no such call, or the redirection quietly becomes an exemption. A value this kind of guard cannot resolve at all should fail rather than pass, because the claim rests on every site being checked and an unreadable site is an unchecked one.

**The test:** for each site your claim covers, ask where the values come from. If the answer for any site is "its caller", the claim is about the callers too, and reading only the raises has not tested it.

## A test written against a refusal the build cannot reach

**Wrong:** proving a repair by driving the command line at it, when a constant gates the code out of reach. On dinah-362 the slug refusal is raised only at or above `SlugMandatoryMajor`, which is 3, and the build declares major 0, so the function returns before refusing on every workbench the tool writes. The test ran the CLI, got exit 0, and its first draft would have been "fixed" by relaxing the assertion until it passed, at which point it would have asserted nothing about the repair and would have looked exactly like a test that works.

**Right:** find the seam the production code already has for reaching the state, and use it. `openWithVocabulary` takes the major from an injected admit function precisely so a test can drive a version the binary does not declare, which reaches the real call site and the real map. Where the repair is to a site that is unreachable today, say so in the criterion rather than leaving a reader to infer that it is live: the fix is still worth making, and a note that records when it becomes reachable is worth more than a test that pretends it already is.

**The test:** when a new test goes green on the first run, check that the code it names actually ran. When it goes red in a way that tempts you to weaken the assertion, ask whether the behaviour is reachable at all before you touch the assertion.

## A guard reported absent after a run bounded to the packages the change touched

**Wrong:** concluding that a property has no automatic check, and recording that conclusion on a criterion, after running the suite only over the packages the change edited. A card had to leave a version constant alone. Its criterion named the wrong test as the guard, so the implementer tested the claim properly, moved the constant, ran `go test ./internal/verb/ ./internal/profile/`, watched both stay green, and reported that the no-bump half of the criterion had no machine check and was verified by reading. The first half of that is right and the named tests genuinely do not read the constant, by an operator ruling recorded in their own doc comment. The conclusion is wrong: `go test ./...` on the same tree fails in four assertions across two other packages, because a compatibility fixture and a documentation transcript both carry the stamped revision. The report is more dangerous than a plain miss, because it is honest, specific, and reproducible, so a reader has every reason to believe it and none to widen the run.

An absence claim about the tree is a claim about the whole tree. A run bounded to the diff can establish that a named test does not catch something; it cannot establish that nothing does. The two conclusions read identically in a handoff.

**Right:** when a break is planted to find out whether any guard exists, run the whole suite over it. Naming which packages were run is not a substitute, because the reader who most needs to widen the run is the one who trusts the report. Where the break is expensive to run everywhere, say what the run covered and state the conclusion at that scope, so "no test in `internal/profile` catches this" stands in place of "this has no machine check". Where the guard is then found somewhere else, the criterion's own text is corrected to name it, since a criterion that names the wrong guard misleads in both directions at once: the downstream stage hand-verifies what CI already covers, and the real guard stays unnamed and unwatched.

**The test:** a handoff sentence of the form "this has no automatic check" is scoped by the command that produced it. Read that command. If its argument is anything narrower than `./...`, the sentence has not been shown and the reviewer either widens the run or rewrites the claim to the scope that was actually tested.


## A negative table row refused by a guard other than the one it is named for

A table of rejection cases reads as one assertion per guard, and the row's own label is what tells a later reader which guard it covers. That mapping is an assumption, not a fact, because the function under test refuses in order: a fixture built to exercise the third guard reaches it only if the first two let it past. Where the fixture also trips an earlier or later guard, the row still goes green with the named guard deleted, and the label is the only thing that ever claimed otherwise.

The shape hides better than a duplicated assertion, because the two rows are not textually alike. They differ in the field they set, in their label, and in the defect they name. They agree only on the answer, and on the reason for it.

**Wrong.** The row named for the dead-end branch sets `rowKind` and clears the field the next guard reads, so the next guard is what refuses it.

```ts
[
  "a dead-end row, which resolved to no workbench",
  { kind: "root", row: rowFixture({ rowKind: "deadEnd", data: undefined }) },
],
[
  "a root row carrying neither a resolved path nor a candidate one",
  { kind: "root", row: rowFixture({ data: undefined }) },
],
```

```ts
if (element.row.rowKind === "deadEnd") {
  return undefined;                       // never reached by either row above
}
const root = element.row.data?.path ?? element.row.candidate?.path;
if (root === undefined || root === "") {
  return undefined;                       // what refuses both rows above
}
```

**Right.** Build each negative fixture so that the guard it is named for is the only one it trips, which means satisfying every other guard in the function. Where the value that would satisfy the later guard cannot arise in production, set it anyway and say in a comment that the fixture is deliberately impossible, since that is what makes the row prove the guard rather than the guard's neighbour.

```ts
[
  "a dead-end row, which resolved to no workbench",
  // The candidate path is set so the path guard below cannot be what
  // refuses this row. A real dead-end row carries no path at all; this
  // fixture is impossible on purpose, so the dead-end check is the only
  // thing left that can answer.
  {
    kind: "root",
    row: rowFixture({
      rowKind: "deadEnd",
      data: undefined,
      candidate: { path: "C:\\work\\other", title: "Other" },
    }),
  },
],
```

**The test:** arm each row of a rejection table on its own, by deleting the single guard that row is named for and confirming that row alone goes red. Arming the table as a whole does not reach this, because a table with one over-covered row still goes red when a different guard is deleted. The count of red rows is the signal, not the presence of red.


## A planted defect that preserves the behaviour it was meant to break

Arming a guard means planting the wrong value and watching the guard name it. The plant is code, so the plant can be wrong, and the way it goes wrong is silent: a plant that changes the text of a line without changing what the line does leaves the suite green, and green from a no-op plant is indistinguishable from green because no guard exists. The reader then draws the conclusion the arming run was supposed to prevent, which is that the guard is missing or the case is unreachable, and either conclusion invites deleting a check that was doing its job.

This is a different failure from a plant that does not compile. That one produces no output, and the counterexample for it is elsewhere in this document. This one produces a full, clean, believable run.

The type checker is what pushes an agent into it. A guard whose deletion breaks compilation cannot simply be deleted, so the plant gets reshaped until it builds, and the reshaping is where the behaviour quietly comes back.

**Wrong.** Both of these were written to break an early return, both compiled, both ran, and both left the function answering exactly what it answered before.

```ts
// The guard returns undefined for a missing element. `element` IS undefined
// here, so returning it returns undefined, which is the guard intact.
if (element === undefined) {
  return element;
}

// `undefined as never` is still undefined at run time. The cast satisfies the
// compiler and changes nothing the caller can observe.
if (element.view === undefined) {
  return undefined as never;
}
```

**Right.** Plant a value the guard exists to prevent reaching the caller, rather than a differently spelled version of the value the guard already returns. Where the guard refuses by returning nothing, the plant returns something.

```ts
if (element === undefined) {
  return { spawner, exe, host, folder: "", root: "", ref: "", label: "" };
}
```

**The test:** before believing a green arming run, read the plant and say what the function now answers for the case under test, in the same words the assertion uses. If the answer is the word the assertion expects, the plant did not plant anything. A useful habit is to predict the failure message before running: an arming run whose red you cannot describe in advance is one you cannot read the green of either.


## A required input reported unreadable because one tool response exceeded a ceiling

**Wrong:** treating a size refusal from a read tool as proof that the content is out of reach. On dinah-332 the implementer's handoff recorded, as a note to the reviewer, that "the workbench document 'Convention counterexamples' is larger than the MCP response ceiling and could not be read at all", and the ratchet step that document exists to serve was skipped on that basis. The document is 293701 bytes against a 262144-byte ceiling, so the read does refuse. What the refusal actually says is that the payload was written to a named file on disk and that the caller should grep or slice it there, and the same session had already taken that route once for an oversized card read. The corpus was one command away the whole time.

**Right:** read the refusal before concluding from it. A response that names a fallback path is a redirection rather than an absence, so follow the path it names, and reserve "could not be read" for content no route reaches. Where a handoff does have to record something as unavailable, say which route was tried, because "could not be read at all" states a property of the document while what happened was a property of one call. The general form is that a tool's bounds are not the content's bounds, and an agent that infers the second from the first drops an input nobody downstream knows was dropped.


## A downstream consumer declared unexposed to a limit, on the strength of the one call it makes

A card that records a limit honestly still has to say who the limit reaches, and the tempting answer is to look at the call the consumer makes and check whether that call clears the limit. The call is the last step. The consumer had to obtain the call's arguments from somewhere first, and when the argument comes through the same mechanism the limit lives in, clearing the limit at the call site proves nothing about the consumer.

**Wrong:** on dinah-272 the recovery for `path workbench` was bounded by discovery's recognition test, so three corruptions of the workbench anchor answer under `--workbench <root>` and refuse under the ancestor climb. That measurement is correct and was driven rather than read. The conclusion drawn from it was that "the gap therefore reaches the operator standing inside a broken workbench with no root in hand, and not the extension that is already holding one", on the ground that the VS Code extension always passes `--workbench`. It does pass it. What was never checked is where the extension gets the root it passes: `resolveWorkbench` runs `dinah status` with the workspace folder as the working directory, and the forest mode runs the downward walk instead. Both apply the recognition test that produces the gap, so against a workbench with a botched frontmatter fence the extension has no root to pass and no row to act on. The claim about the consumer was inferred from the consumer's last call rather than from its first one.

**Right:** trace the argument back to its origin before ruling on who a limit reaches. Where the claim is that a consumer names a value explicitly, the question to answer is which call produced that value, and whether the producing call is subject to the same limit. On this card the answer took two commands against a binary already built: run the walk against each corrupted fixture and see which ones it lists, and run the consumer's own resolution call from the folder and read what came back. Where a consumer genuinely is unexposed, say which call supplies the value and why that call is outside the limit, so the next reader can check the chain rather than take the conclusion.

**The test:** for every "this consumer is not affected" sentence, name the call that gives the consumer the input which makes it unaffected, and say what that call does under the same failure. A sentence that reasons only about the consumer's final call has audited one end of a chain and reported on the whole of it.



## A single-purpose MCP tool that fills in an argument itself, and publishes it anyway

The MCP head builds each tool's input schema from the same parameter list the terminal composes its syntax line from, so the two heads cannot drift. That sharing has one seam. A terminal command can carry an action word the caller types, while the tool built on it may serve exactly one of those actions and set the word itself. When that happens the parameter is still on the shared list, so the schema goes on publishing it, and publishing it as required when the terminal marks it required. The head then asks callers for a value it discards, and the discarding is silent.

**Wrong:** on dinah-204 the `column` command declared `{Name: "action", Display: "new", Required: true, Field: "Action"}`, and `internal/mcp/tools.go` registered `{name: "new_column", command: "column", run: func(l *verb.Library, r *verb.Request) any { r.Action = "new"; return l.NewColumn(r) }}`. The generated schema listed `action` under `required`, described as "create a column; new is the only action this build carries", and `assignValue` duly landed the caller's value on `Request.Action`, where the tool's own closure overwrote it before the library saw it. A `tools/call` naming `"action":"get"` created a column and answered `outcome: ok`, where `dinah column get "Signed off"` refuses under `contract.Usage`. The branch's own parity test sidestepped the contradiction by sending no `action` at all, and its comment said so ("The tool fills in the action itself, so the call names none"), which means every test of the tool violated the schema the tool publishes.

**Right:** hold the parameter back rather than publishing it. `argumentExemptions` in `internal/mcp/tools.go` exists for exactly this and already carries the `check` tool's `root` and `max-depth` with a sentence each saying why that head does not honour them. A tool that fixes an action adds `"new_column": {"action": "the tool serves the one action this command carries and sets it itself, so a caller has nothing to choose"}`, and `schemaFor` then publishes neither the property nor the requirement. The alternative, if the value is genuinely meant to be honoured, is to dispatch on it the way the `workstream` tool does and let a wrong word refuse.

**The test:** for every tool whose `run` closure writes a field of the request, check whether that field's parameter still appears in the tool's published schema. `schemaFor`'s own doc comment states the rule this breaks, that "a property offered here and refused there would tell a caller two different things", and a property assigned by `assignValue` and then overwritten is refused in the only sense that matters to the caller. A guard that only asks whether each declared parameter reaches its declared field, as `TestEveryDeclaredParameterReachesItsDeclaredField` does, passes on this defect, because the value does reach the field and is destroyed one frame later.


## A claim about a consumer, settled from the consumer's plan instead of its call chain

**Wrong:** deciding what a downstream consumer does with a value by reasoning about what the value means. On dinah-272 the card claimed that an operator who has pinned `dinah.workbench` by hand escapes a recovery gap, "because that setting supplies the root with no discovery call behind it". The reasoning is about the setting. It never reaches the consumer. The VS Code extension does not use the pinned value as a root: `resolveWorkbench` passes it as `--workbench <pinned>` to `dinah status` and then takes the root from status's own answer, and only when that answer is ok. Against a workbench whose frontmatter fence is broken, `status --workbench <root>` exits 2, `runDinah` classifies any non-zero exit as not ok, and `blankState` records a root only for an ok resolution. The pinned operator lands in exactly the case the card says he escapes.

This is the same shape three times on one card. Round one claimed the gap could not reach the extension because the extension always names the workbench directory. Round three corrected that but then asserted a decision note about a command, `editWorkbenchDefinition`, that does not exist in `editors/vscode/src` at all, so the claim rested on another card's spec rather than on shipped code. Round four found the pinned sentence. Each time the premise came from a plan or from the meaning of a name, and each time driving one call would have settled it.

**Right:** when a card asserts what a consumer does, trace the value backward from the consumer's own code to the call that produced it, and run that call against the damaged fixture. Two questions decide it. Which function in the consumer holds the value, and what does the consumer require before it stores it? On this card both answers are one grep and one command: `blankState` stores a root only on an ok resolution, and `dinah --workbench <root> status --json` against the broken fixture exits 2. A claim about a consumer that names no line of the consumer and no command that was run is unverified, whatever its reasoning looks like.

The corollary is about scope. A card whose deliverable is a CLI shape will still carry claims about editors, scripts and sibling cards, and those claims are the ones nobody arms, because the card's own tests cannot reach them. Verify them the way the card's own criteria are verified, by running something, or state plainly that they rest on a plan that has not shipped.


## An argument the test double accepts and throws away

Giving a fixture's two roles two different values is what lets a test notice when the code hands one where the other belongs. That repair is wasted wherever the double never looks at the value at all, and the two defects look identical from the outside: the plant compiles, the behaviour changes, and the suite stays green.

This is the mirror of "A test input whose distinguishing shape is erased by the constructor that builds it". There the two values arrive at the function as one. Here the values stay apart the whole way and the observer discards one of them on receipt, so no amount of separating fixture values can ever arm the check.

**Wrong:** writing a recorder whose stub signature drops the parameters the test is not currently interested in, and then treating the recorder as the thing that proves what the call sends. A spawner double written as `async (_exe, argv) => { calls.push([...argv]); return answer; }` records the argv and silently discards the executable and the spawn options. The module under test passes all three, so `runDinah(spawner, context.exe, argv, { cwd: context.root })` can be changed to pass the workbench root as the executable, or the workspace folder as the working directory, and both plants compile, both change what a real process would receive, and both leave every test passing. A separate fixture pass that had just given the workspace folder and the workbench root two different paths, precisely so a swap between them would redden something, did not close either hole, because the hole is in the observer rather than in the fixture.

**Right:** have the double record every parameter the contract carries, and assert the ones the call actually decides at least once. The same repository already has the correct form, in a sibling test file whose spawner records `{ argv: [...argv], cwd: options.cwd }` and asserts the cwd on a single row. One assertion per decided argument is enough; what is not enough is a double that cannot see the argument.

**How to find it:** read the double's own parameter list against the signature of the thing it stands in for, before trusting any arming run made through it. A parameter named with a leading underscore is the visible form of this defect, and an omitted trailing parameter is the invisible one, because most languages let a function of fewer parameters stand where more are passed.


## A prose fix that folds a list into one sentence and invents a count

Caught on dinah-204 (Agent Code Review round 2), and previously on dinah-314, where three consecutive rounds each repaired a false claim and shipped a fresh one as a rounding-off sentence.

The trap is specific to the repair. A reviewer tags three identically-shaped sentences as a register tell, the implementer folds them into one, and the fold needs a topic clause to hang the list on. That clause is written from the three items in front of the author rather than from the code, so it quantifies over a set the author never counted.

Wrong (`docs/quick-start.md`, "Add a column to the flow"):

```
All three optional flags have defaults. A column created without `--before`
goes on the end of the flow, `--kind` defaults to `work` because that is what a
new station usually is, and a column given no capacity holds as many cards as
you put there.
```

`dinah column new` takes four optional flags, not three. `verb.Params("column")` declares `kind`, `capacity`, `slug` and `before`, and `--slug` has a default too (derived from the title via `SlugifyDashed`). The same section's own opening paragraph says so twenty lines above: "It takes the title, and optionally the kind, a capacity, a slug of your own, and the column to stand in front of." The three sentences the fold replaced made no count claim at all, so the rewrite introduced a false statement where there had been none, into the document a new user reads first.

Right: fold the list without quantifying over it, or count against the parameter table rather than against the sentences on screen.

```
A column created without `--before` goes on the end of the flow, `--kind`
defaults to `work` because that is what a new station usually is, `--slug` is
derived from the title, and a column given no capacity holds as many cards as
you put there.
```

Two things generalise. A rewrite is checked for meaning drift in both directions: a dropped qualifier is the well-known half, and an added universal is the half that slips through, because a reviewer comparing old text to new reads the new claim as context rather than as a claim. And no guard catches this one. `TestTheQuickStartMatchesTheTool` replays the fenced console blocks against the real binary and never reads the prose between them, so a wrong sentence beside a correct transcript is green on every platform.



## A spec citing code by line number, after the card's own diff moved the lines

A spec that quotes code by `file:start-end` is checkable, which is why this board writes them. The check has a shelf life. The card's own diff shifts the lines the spec cites, and nothing recomputes them, so a citation that was exact when it was written silently comes to name a different function. It reads as verified because it has the shape of a verified thing.

**Wrong:** on dinah-272 the spec opened by attributing the plumbing guarantee to "`runPath`'s own doc comment (`cmd/dinah/commands.go:892-897`)". After the card landed, `runPath` sits at 935 and lines 892 to 897 are `PathAnswer`'s comment, which the same card introduced. Five more had moved the same way in one spec: `runPath` at `commands.go:898-908`, `s.open()` at `main.go:419-435`, `withBench` at `main.go:449-455`, and `s.workbenchRoot` and `bench.Open` at `main.go:432` and `:434`. Every one of them pointed at real code, none of them pointed at the code the sentence named, and four rounds of review read past all six. The related major that review did catch, a quoted doc comment that no longer matched the shipped one, is the same rot in a form big enough to see.

**Right:** re-resolve every code citation in the card text against the branch head before handing off, rather than against the draft the citation was written from. Reading the range and asking whether it contains the named symbol is enough, and it is mechanical: `sed -n '<start>,<end>p' <file>` per citation. Where a citation names a symbol whose location the card's own diff is changing, prefer naming the symbol without a range, since a symbol name stays true and a line number does not. Drop the range rather than guessing a new one.

**The test:** for each `file:line` in a spec, run the range and check the symbol named beside it appears inside it. A citation that survives this is worth its precision; one that does not is worse than no citation, because it sends the next reader to code that has nothing to do with the sentence.



## A stubbed method that answers nothing and remembers nothing

"An argument the test double accepts and throws away" covers the parameters a double drops on receipt. A double drops calls as well as parameters, and the second form is harder to see, because the stub looks complete: every method of the interface is present, each one compiles, and each one answers something the caller can use.

A command host, a spawner, a clock or any other injected collaborator usually carries more methods than the code under test needs. The tempting shape is to satisfy the type by giving the unneeded methods a body that answers a neutral value and records nothing. That satisfies the compiler and hides a whole class of change: the code under test can start calling one of those methods and every test goes on passing, because the only evidence a call happened was thrown away at the moment it arrived.

It bites hardest where a decision is expressed as an absence. A decision that a command reports nothing on success, reveals nothing, and opens nothing is a decision about methods the command must not call, so a double that cannot see those calls leaves the decision with no guard at all while the file reads as though it were thoroughly tested.

**Wrong:** answering the collaborator's unused methods with a neutral value.

```ts
showInfo: () => undefined,
copyToClipboard: async () => undefined,
openFile: async () => undefined,
```

**Right:** answering the same neutral value and naming the call, then asserting the list is empty wherever the absence is the contract.

```ts
showInfo: () => {
    unused.push("showInfo");
},
copyToClipboard: async () => {
    unused.push("copyToClipboard");
},
openFile: async () => {
    unused.push("openFile");
},
```

```ts
await newCard(context);
assert.deepEqual(r.unused, []);
```

**How to find it:** read every method of the double, not only the ones the tests assert on, and ask of each one what would happen if the code under test called it tomorrow. A body that returns a literal and touches no captured variable is the visible form. The check is cheap to arm, because a plant that adds one such call to the production code compiles, changes nothing else, and shows immediately whether anything is watching.

**A caveat worth stating:** not every method belongs in the recorded set. A diagnostic log line is not something a user sees, so a decision about what the user is shown does not rule on it, and recording it would redden the guard on an edit that changes nothing the decision cares about. Record the methods the decision is about, and say in the double why the rest are left out.


## A compound guard whose second condition no fixture ever supplies

A guard written as a disjunction refuses on any one of its conditions, and a negative fixture row supplies one value. Those two facts together mean a guard of the form `x === undefined || x === ""` is usually driven on `undefined` alone, because that is the value the absent-field fixture already carries, and the row that drives it reads as covering the whole guard.

Deleting the untested condition then leaves the suite green. Nothing is wrong in the code today, which is what makes the shape survive review: the production line is correct, the negative row exists, and the criterion that names the guard is honestly marked. What is missing is any pressure on half of the condition, so the next edit that tidies the disjunction has nothing to stop it.

The untested half often guards the worse failure. An absent identifier tends to fail loudly at the next hop, while an empty one is frequently a legal value meaning something else in the same system. On dinah-331 an empty attachment ref means the workbench itself, so a card row whose ref arrived empty rather than absent would attach the operator's file to the workbench instead of to the card, with no refusal and no message. The guard against that is one clause of a three-clause `if`, and the suite provoked neither it nor the clause beside it.

**Wrong:** one negative row per field, driving whichever falsy value the fixture already produces.

```ts
["a card row whose ref never arrived", cardRow(undefined, { ref: undefined })],
```

**Right:** one negative row per condition, so the row count matches the disjunct count.

```ts
["a card row whose ref never arrived", cardRow(undefined, { ref: undefined })],
["a card row whose ref arrived empty", cardRow({ ref: "" }, { ref: "" })],
```

**The test:** count the conditions in every guard the diff adds, then count the negative rows that reach each one. Where the second number is smaller, delete the unexercised condition and run the suite. A green run names the disjunct that has no row, and it costs one edit and one run to ask. Sibling entry: "A repair applied at every producer of a field, armed at the one the reviewer's reproduction happened to name" applies the same counting to the sites of one assignment rather than to the conditions of one guard.

## A sweep reports the size of the list it walked, not the number of items it worked

Caught at Agent Code Review on dinah-268, 2026-09-03. The card's noise measurement swept a mistyped form of every word in a 57-word vocabulary against 300 generated titles and reported "sweeping 57 mistyped vocabulary words". Eight of those words were skipped by a length guard inside the loop, so 49 phrases actually ran, and the criterion whose entire deliverable is a recorded measurement recorded a number that was 16 percent larger than what happened. The conclusion held when the missing regime was measured separately, which is luck rather than method.

Wrong:

```go
for _, word := range vocabulary {
	phrase := mistype(word)
	if phrase == "" {
		continue
	}
	// ... measure phrase against every title
}
t.Logf("sweeping %d mistyped vocabulary words across %d titles answered %d unrelated titles",
	len(vocabulary), len(titles), noisy)
```

Right:

```go
swept := 0
for _, word := range vocabulary {
	phrase := mistype(word)
	if phrase == "" {
		continue
	}
	swept++
	// ... measure phrase against every title
}
t.Logf("sweeping %d of %d vocabulary words across %d titles answered %d unrelated titles",
	swept, len(vocabulary), len(titles), noisy)
```

The rule generalises past this loop. Where a measurement's own report is the deliverable, count what the body of the loop did and print that, rather than printing the length of what the loop ranged over. Any `continue` between the two makes them different numbers, and nothing in the output says so. Where the two counts differ, print both, because the gap is usually the interesting part.

The same catch had a second half worth keeping. The guard that caused the skip read `len(letters) <= floor+1` while the code under test refused only `length <= floor`, so the sweep excluded exactly the shortest input the feature admits, which is also the input where the feature is loosest. A guard inside a test that restates a threshold from the code under test should be written against that code's own constant and comparison, in the same form, or the test measures a different feature than the one it names.


## A before-and-after comparison of names that are not unique

Caught on dinah-331, round five, on the trunk reconciliation with dinah-332.

A merge brings two suites together and the count alone cannot show that a test quietly stopped running, so the honest check is to compare the test names on the merged result against the names on each parent. That check is right, and it is what AC-13 on that card asked for. It is run wrong when the names are compared as sets, because a set has no room for a name that appears twice.

The suite in question declares `a dismissed refusal toast leaves the channel unrevealed and the reason in it` in two different files. On trunk it appears twice, and on the merged result it appears twice. A set difference collapses each pair to one member, so either copy could vanish and the comparison would report nothing missing. The same collapse also invents a phantom: subtracting a set-of-the-union from a multiset makes a surviving duplicate look like a name that is new on this branch, which is how the wrong reading announces itself if anybody looks.

**Wrong.** Sort the names, `sort -u` them, and take `comm` of the two unique lists, reporting "nothing from the parent is missing" on the result.

**Right.** Compare as multisets, which is the same pipeline without `-u`, and separately assert that the names are unique by reading `sort | uniq -d` and expecting nothing. A duplicate is worth knowing about on its own terms, since two tests answering to one name make every later report about that name ambiguous, and the assertion costs one line.

The general shape is that a comparison over a collection is only as strong as the collection's own uniqueness, and nothing in a test runner enforces that test names are unique. Where a check's correctness rests on identity, establish the identity rather than assuming it.


## A completeness sweep that ran before the edit which invented a fourth site

Caught on dinah-367, agent code review round two, 2026-09-03. A previous round found an assertion a wrong answer satisfied by containment, the implementer closed it, then swept every path comparison the card had added and reported three sites of that shape, all closed. The criterion recording the sweep opened with a universal claim: every assertion this card added that compares a rendered path against an expected one is an identity rather than a containment. A fourth site existed, in a test the same commit added a few hundred lines away.

WRONG. The sweep's own scope was narrower than the claim it licensed. Its note enumerated the three closed sites, then accounted for "the remaining ones in `internal/bench`" and stopped there, while the new test sat in `cmd/dinah`. The test reads:

```go
if !strings.Contains(got.out, damaged) {
    t.Errorf("%v does not name the damaged workbench:\n%s", args, got.out)
}
```

where `damaged` is a directory and the wrong value is that directory plus `/workbench.md`, which contains it. Planting the wrong value compiles and leaves this test green.

RIGHT. Two things, and the second is the one that generalises.

First, pair the presence assertion with the absence of the value that contains it, exactly as the three closed sites do:

```go
if !strings.Contains(got.out, damaged) { ... }
if strings.Contains(got.out, filepath.Join(damaged, bench.WorkbenchAnchor)) { ... }
```

Second, and this is the reusable part: a sweep is a snapshot, and a commit that both sweeps and edits invalidates its own snapshot. Run the sweep last, after the final edit, and re-run it whenever the commit grows a new test. A criterion asserting that a class of defect is absent everywhere has to be re-established at the commit being handed off, not at the commit where the class was noticed.

This is distinct from the entries about containment assertions themselves, which name the trick. This one names the bookkeeping failure that lets one instance survive a sweep that genuinely found the others. It is the same family as the workbench instruction "treat that number as a hypothesis to test rather than as a given", applied to a count the author produced in the same breath as the work that changed it.


## A superseded source-text guard updated to the new spelling instead of retired

A change that replaces a text-scanning guard with an executable one covering strictly more ground leaves the scan behind, and the tempting repair is to edit the scan's match string so it goes on passing. The two guards then disagree about what they can see, and the weaker one is the only one that can produce a false red.

**Wrong:** dinah-369 routed every command registration in `extension.ts` through one helper and made activation fail when the declared roster and the registered roster disagree. A unit row that scanned `extension.ts` as text for two of the fifteen commands had its match string updated from `vscode.commands.registerCommand( COMMAND_NEW_CARD,` to `register(COMMAND_NEW_CARD,` and was left standing.

```ts
const EXTENSION_SOURCE = readFileSync(..., "utf8").replace(/\s+/g, " ");
for (const constant of ["COMMAND_NEW_CARD", "COMMAND_ATTACH_FILE"]) {
	assert.ok(
		EXTENSION_SOURCE.includes(`register(${constant},`),
		`${constant} is declared in the manifest and registered nowhere`,
	);
}
```

The scan recognises the one spelling the formatter produces for those two call sites today. A sibling call site in the same file already collapses to `register( COMMAND_EDIT_COLUMN_INSTRUCTIONS,` with a space, because it is long enough that the formatter breaks the line, so the two spellings live side by side in the file the guard reads. A rename, a longer constant, or a line-width change reddens the row with a message asserting the command is registered nowhere, which is false, and no rename of the helper reddens it for a reason a reader could act on.

**Right:** retire the scan in the same change that lands the stronger guard. Keep only the assertions covering ground the new guard cannot reach, rename the test to what it still claims, and say in the handoff what coverage went with the deleted half. dinah-343 already ruled that the answer to a fragile source-text guard is to make the real code callable and to call it, rather than to strengthen the pattern match. Updating the pattern is that ruling ignored one step later, and it costs more than leaving the old string alone would have, because the row now looks maintained.

**The test:** after this change, does the text scan assert anything the executable guard beside it cannot? Where the answer is no, deleting it loses no coverage and keeping it buys a false red.

## A claim corrected in one file and left standing in its twin

Caught on dinah-343 (a sweep that fixed the phrase it searched for and missed the passage stating the same claim in other words) and again on dinah-369, where a review push-back corrected an overclaiming sentence in `registrationGuard.ts`'s header and the identical claim survived in `manifest.test.ts`'s comment above the rows the same commit added.

Wrong, `test/unit/manifest.test.ts`, after the guard header had been corrected:

```
// ... so these prove that the comparison is correct
// and prove nothing about activate() calling it. Only the integration suite,
// which activates the extension in a real editor host, proves that half.
```

Right, the same correction carried to every place the claim appears:

```
// ... so these prove that the comparison is correct
// and prove nothing about activate() calling it. The integration suite is the
// only thing that reaches that call, and it does not hold the call in place
// either: activation succeeds with the call deleted, so the suite goes red
// only when a registration is genuinely missing at the same time.
```

The reviewer's test is not "did the named sentence change" but "does the claim appear anywhere else". A correction scoped to the file the finding cited leaves the reader who opens the other file with the sentence the finding rejected. Grep for the claim's subject (here, the integration suite) across the diff's touched files rather than for the wording the finding quoted, because the surviving copy is usually spelled differently.


## A comment that describes a plant nobody ran (dinah-369, 2026-09-04)

Caught in my own draft before it shipped, on the round that existed to fix two
earlier claims of the same family. The board already records "a correction
applied in one file and left standing in its twin" twice. This is the adjacent
shape: prose that describes what a code change WOULD do, written from reading
rather than from running, in a comment whose whole purpose is to be trusted by
somebody who was not here.

**Wrong.** A source comment asserting the outcome of a specific edit, composed
by reasoning about the code:

```ts
// Delete the registerCommand line from inside the helper and every id is still
// recorded, so this function still returns quietly.
```

The reasoning was sound and the sentence was false. That exact deletion does not
compile, because it leaves the `handler` parameter unread and TypeScript refuses
it (`error TS6133: 'handler' is declared but its value is never read`). Running
it produced no test output at all, which on this board looks identical to
everything passing. A reader who later tried the recipe would have got a broken
build and no idea which half of the sentence was wrong.

**Right.** Run the edit you are about to describe, then describe what you saw,
and keep the fact that the obvious version does not build:

```ts
// Stop the helper from registering and every id is still recorded, so this
// function still returns quietly.
```

with the observation recorded beside the helper it concerns:

```ts
// That was watched rather than assumed: a helper that records every id and
// registers nothing leaves the unit suite green at all 299 rows. Deleting the
// registration line by itself does break the build, because it leaves the
// handler parameter unread, but one further token restores the build with the
// commands still unregistered, so the compiler is not standing in for the
// check that is missing here.
```

**The test.** When a comment names an edit and predicts its result, that is a
claim about a run, not a claim about the code, so it needs a run behind it. Ask
of any such sentence: did I do this, or did I work it out? A predicted red is
worth no more than a predicted green, and the near-miss here was a prediction of
green that would have been a build failure.

**Why it bites where it does.** The compiler catching the minimal deletion is a
coincidence of one unused parameter rather than a guard, and one extra token
removes it. So the tempting repair, which is to write that the compiler covers
this case, would have replaced a false sentence with a worse one: a reader would
have credited the build with a check it does not perform. Where an accident
happens to catch your plant, say it is an accident and say what defeats it.


### The past-tense variant, caught in the same commit that filed the entry above

The entry above was written during dinah-369's third round, and the same commit
shipped the shape it does not quite cover. Its test asks whether a predicted
result was run or worked out. It does not ask the same question of a sentence
that reports a run in the past tense, and that is the one that got through.

**Wrong.** A comment asserting a verification pass as history, in a paragraph
whose job is to bound what the check proves:

```ts
// The wiring was established once, by dropping a registration deliberately
// and watching the suite fail (dinah-369).
```

Nobody had run that suite. All three implementation rounds recorded "I did not
run npm run test:integration", and the card's own criterion stages that arming
pass at Test, still pending. The sentence arrived as a repair for a different
false claim in the same paragraph, so a reviewer reading the diff sees a
correction and reads past the assertion inside it.

**Right.** Say what is still owed and name the item that owes it:

```ts
// Nothing has yet dropped a registration and watched that suite go red. The
// arming pass is staged at this card's Test stage (dinah-369 AC-6). Until it
// runs, no standing check reddens if the call is removed.
```

**The test.** Extend the question above to the past tense. Ask of any sentence
reporting a check that ran: which stage ran it, and does the card's own record
of that stage agree? A pending criterion that stages the pass is direct evidence
the pass has not happened, so a comment claiming it has contradicts the card it
sits under.


## A criterion verified by a call carrying an argument the criterion does not name

**Wrong:** writing a criterion that spells a tool call out in full, then closing it against that same call with one extra argument added, where the added argument is the reason it passes. A criterion read `list_workbench_documents(documents="Convention counterexamples 1", fields="id,title,body")` and was marked verified from a run that also passed `max_bytes=262144`. That parameter defaults to 48,000 bytes, so the call as the criterion spells it answers `documents: []` with `truncated_by: "max_bytes"` and no error at all. Anyone who runs the criterion's own text watches it fail while the checklist records a pass, and the caveat sits in the note where a reader scanning states never reaches it.

**Right:** the criterion's text is the check, so every argument that decides the outcome belongs inside it. When verification discovers that the spelled call cannot pass, that is a finding about the criterion rather than a caveat to append to the note: name the argument you had to add and why, and let the reviewer either correct the text or send the work back. The test: could a stranger paste the criterion's call unchanged and see the result the criterion claims? If the answer needs an argument the criterion does not carry, then the criterion is not the check that ran.

This is the argument-level form of the entry "An acceptance criterion marked verified against a test that does not exercise it", and it is worth its own entry because the tell is different. That one is found by opening the named test and reading what it asserts. This one is found by reading the criterion and the note side by side and diffing the call in one against the call in the other, which is a comparison nobody makes unless they are told to.

## A failure message that names a different dimension than its assertion tests

Caught on dinah-329. The implementer found the first instance by arming a guard and reading what it printed, and the review that followed found four more of the same shape in the same diff.

An assertion changes what it is about, and its message does not follow. The message goes on reporting the dimension the old assertion tested, so when the guard finally fires it prints a value that says nothing about the failure. The suite is still correct and the guard still fires. What is lost is the one line that was supposed to explain the failure, and the reader is told nothing at the moment they most need telling.

The shape appears wherever a node's children stop being homogeneous. A test that read a column's group values to say what that column drew keeps reading them after the column can also hold bare card leaves, and a card leaf carries no group value, so a leak of four leaves prints as an empty list. An empty list reads as the assertion having no complaint at all.

**Wrong.** The assertion tests how many children the node drew, and the message reports the group values, which the leaked children do not carry.

```go
if len(column.Children) != 0 {
	t.Errorf("the column draws %v and the depth left it nothing to draw", groupValues(column))
}
```

**Right.** The message names the count and the kinds, which is what the assertion is now about.

```go
if len(column.Children) != 0 {
	var kinds []string
	for _, child := range column.Children {
		kinds = append(kinds, child.Kind)
	}
	t.Errorf("the column draws %d children of the kinds %v and the depth left it nothing to draw",
		len(column.Children), kinds)
}
```

Reporting a bare count is the weaker version of the same fault rather than a cure for it. Four sibling assertions in the same diff report `len(node.Children)` and stop there, and three of those use `t.Fatalf`, which aborts the test before the assertion beneath it that would have named the kinds. A count is never misleading the way an empty list is, and it still leaves the reader to run the test a second time to find out what the extra child was.

**The test:** for every assertion a diff rewrites, read the message against the assertion's new subject rather than its old one, and ask what that message would print for the failure the assertion now exists to catch. Where the answer is a value that could be identical on a passing run, the message has not moved with the assertion. Arming the guard is what surfaces this, so read the text of the red run rather than only its colour.

## A text-reading guard widened in review, whose must-keep-passing corpus holds only the strings that exist today

Caught on dinah-378, round three. It is the other half of the entry "A guard that reads source text reads one spelling of the thing it is guarding", and it is what happens when that entry's advice is followed well. Round two of the same card found the guard too narrow and asked for the frames a writer would reach for. Round three delivered them, and the widened guard began refusing sentences that make no claim at all.

The guard's subject was a false claim that a VS Code extension carries a `dinah` binary. The widened predicate refuses a sentence when it names the extension and contains a possession verb, in either order, anywhere in the sentence.

**Wrong.**

```ts
const namesExtension = /\bextensions?\b/i.test(sentence);
for (const pattern of [POSSESSION, QUALIFIED_PAYLOAD]) {
    if (pattern === POSSESSION && !namesExtension) {
        continue;
    }
    // ... refuse on any unnegated POSSESSION match
}
```

Nothing here requires the thing possessed to be the payload the guard is about. Run against ordinary copy, it refuses "This extension provides a tree view of your workbench", "The extension includes a status bar item", "This extension ships with commands for claiming and moving cards" and "The extension bundles its own webview assets", and it reports each of them as saying a dinah binary lives inside the extension. Those are the sentences somebody actually adds to a settings description or a welcome block, so the guard fails on honest work and its message tells the author they wrote something they did not write.

**Right.** Require the payload as well as the subject, which is what the guard's own second frame already did:

```ts
const namesExtension = /\bextensions?\b/i.test(sentence);
const namesPayload = /\b(dinah|binary|binaries|copy|cli|executable)\b/i.test(sentence);
if (pattern === POSSESSION && !(namesExtension && namesPayload)) {
    continue;
}
```

Measured over the same sentence sets, that clears every one of those false refusals, keeps the claims the guard was widened to catch, and leaves room to add `has` and `contains`, the two commonest possession verbs in English, which could not safely be added while any object would do.

Two general points, and the second is the one that costs cycles.

A guard that reads text needs a corpus in both directions, and the honest half is the half that gets written badly. The must-keep-passing list here held the four strings the tree publishes today plus five hand-written negations, so every entry in it was a sentence about where dinah comes from. Not one was an ordinary sentence about what the extension does, which is the shape the widening actually endangered. A list built by copying the current strings tests that the guard still passes today's tree; it does not test the class. Write the innocent half by asking what a contributor would add next year, not by copying what is there now.

And a widening is a change of behaviour in both directions, so arm it in both. The eight arming plants on this card were all of the form "plant a claim, watch it caught", which proves the guard did not get looser. Nothing planted an innocent sentence to prove the guard did not get tighter, and that is exactly the direction the widening moved it.


## A non-vacuity fixture that drifts every site to the same value

Found on dinah-365, one round after a reviewer defeated the count-based version of the same test. The first repair was to assert the messages instead of their count, and it did not work, because the fixture was the other half of the defect.

A guard walks a table of sites, and each entry pairs a constant label with a function that reads that site. The failure message is built from the label. So when the fixture drifts both sites to the same number, an entry whose read function has been redirected at its neighbour still produces the label it always produced, carrying the value its neighbour holds, which is the same value. The expected messages come back verbatim from a check that read one site twice and never looked at the other. Asserting on content rather than count buys nothing here, because the content is identical either way.

Wrong. Both sites carry the same number, so the two complaints are indistinguishable from one site read twice:

```js
const lock = { version: "0.1.0", packages: { "": { version: "0.1.0" } } };
assert.deepEqual(lockfileVersionDrift({ version: "1.0.0" }, lock), [
    'package-lock.json version is 0.1.0 and package.json is 1.0.0, ...',
    'package-lock.json packages[""].version is 0.1.0 and package.json is 1.0.0, ...',
]);
```

Right. One case per site, drifting that site while leaving the other correct, so a check that stopped reading a site reports nothing for that site's case:

```js
const lock = { version: "1.0.0", packages: { "": { version: "0.1.0" } } };
assert.deepEqual(lockfileVersionDrift({ version: "1.0.0" }, lock), [
    'package-lock.json packages[""].version is 0.1.0 and package.json is 1.0.0, ...',
]);
```

The rule: a fixture proving a check reads N places has to distinguish those places, which means each case makes exactly one of them the reason for the complaint. A case that trips every site at once cannot tell you which sites were read, whether you count the complaints or read them.

The two other cases still belong in the table and neither replaces these. A case tripping both sites at once holds the guard's behaviour on a wholly stale file, and a case supplying no site at all holds the "found nothing to compare" hole shut. They just do not prove that distinct places are read.

How to know your repair worked: plant the mutation and watch. Redirect one entry's read function at its neighbour's site, run the suite, and require the test to go red naming the site that went silent. On this card the first repair passed that mutation, which is how the fixture problem surfaced at all.




## A validated snapshot written back after the lock was given up

Found on dinah-316, in a write phase deliberately built out of several short lock acquisitions instead of one long one. The composed form is legitimate and sometimes forced: the structural protocol takes the workbench lock itself, so an outer lock held across it would refuse the archive rather than serialise it. What does not survive the change is code that goes on treating a snapshot read under the first lock as still true under the second.

Two idioms already in the tree say what to do instead, and both say it in their own doc comments. `Library.Do` takes the card's lock and then loads the card, "so the revision the basis is compared against and the state every precondition reads are the ones on disk at the moment of the write rather than a snapshot taken before it." The column writer opens a second view under its lock rather than reusing the one the library holds, "so the flow the placement is judged against and the flow the placement is spliced into are the same flow."

The three shapes below all appeared in one file. Each reads correctly on its own and each is a lost update once the validating lock has been released.

Wrong. The guard runs under the validation lock, and the write it guards happens under a later one, with nothing between them re-asking the question:

```go
// validation, under the workbench lock
if held := heldCardIDs(cardsIn(cards, element.live.ID)); len(held) > 0 {
    return contract.RefuseWith(contract.ReshapeHeldCardInQueue, ...)
}
// ... lock released, other steps run ...
// step four, under a fresh acquisition
if err := bench.WriteColumnFromElement(root, element.id, element.slug, element.element); err != nil {
```

Right. The predicate is re-evaluated against a view opened under the lock that takes the write, so a claim taken in the interval is seen by the check that exists to refuse it.

Wrong. The freshness check on a card is made before the card's lock is taken, so the write that follows is composed from a pre-lock read:

```go
card, err := bench.LoadCard(fresh.CardsRoot(), standing.ID)
if card.Column != entry.id {
    continue
}
lock, err := bench.Acquire(card.Dir, req.Actor, now)
card.Column = destination.ID
card.Save()
```

Right. Acquire first, load under the lock, then decide, which is the order `Library.Do` documents and the reason it gives for it. `Card.Save` writes every field from the in-memory copy with no compare-and-set, so a stale load does not merely lose the column: it restores the state, the holder, the claim stamps and the block reason as they stood before the concurrent writer touched them, while that writer's own journal line survives.

Wrong. A collection is re-read under the lock and then overwritten with a list computed before it:

```go
fresh, err := bench.Open(l.Bench.Root)
order := make([]string, 0, len(plan.elements))
for _, element := range plan.elements {
    order = append(order, element.id)
}
if equalSequences(fresh.ColumnSequence(), order) {
    return nil
}
return fresh.SetColumnSequence(order)
```

Reading the live sequence and then discarding it is the tell. The comparison is there to skip a no-op, not to notice divergence, so an identifier another writer added in the interval is dropped from the sequence while its directory stays on disk, and an identifier another writer archived is put back with no directory behind it. Those are the two states `check.orphaned-column-directory` and `check.stranded-column` report.

Right. Compose the new order from what the fresh read returned, carrying forward any identifier the plan does not know about, or compare the fresh read against the sequence validation saw and refuse when they differ.

The rule: when a run gives up its lock between deciding and writing, every value carried across that boundary is a guess. Re-read it under the lock that performs the write, or compare it against a fresh read and refuse. An idempotency argument does not cover this. Idempotency is about the run's own crashed predecessor, whose partial work the retry is trying to finish; concurrency is about another writer whose work the retry has no reason to want to erase, and a step can be perfectly idempotent and still be a lost update.

How to know your repair worked: the `Library.Interleave` hook and a second library view are already in the tree for exactly this proof. Fire the competing write from inside the hook and assert that the run either sees it or refuses, the way the workbench-anchor test does. A write phase with no `Interleave` call site cannot be tested for this at all, which is itself the finding.

### Correction to "A validated snapshot written back after the lock was given up" (dinah-316, implement round following the review that filed it)

The entry closed by saying `Library.Interleave` is the in-tree way to prove the repair, and that reshape has no `Interleave` call site, so the class could not be tested there. The second half is no longer true, and the first half turns out to be the wrong tool for this class.

`Interleave` fires inside a lock, and it exists to drive a second process at a lock that must refuse it. The defect this entry is about lives in the window where NO lock is held, between two acts of a composed write phase, where nothing refuses anybody. Proving that repair needs the opposite hook.

`Library.Interpose func(step string)` is that hook, and `internal/verb/reshape.go` is its first caller. It fires between write-phase steps and, in the carry, once more before a single card's own lock is taken, because that is where a step deciding from an earlier read has already decided. `internal/verb/reshape_test.go` uses it for all three interleavings: a claim taken before the kind flip, a claim taken before one card's carry, and a column added or archived before the reorder.

One thing that cost a pass and is worth writing down: the first version of the carry test placed the seam between two whole steps and the planted break still passed, because a break that re-reads at the top of the step sees the concurrent write. A seam is only a proof when it sits exactly where the broken code has already decided and the repaired code has not.


## A refusal raised after the command has already written, reported as though it had not

Caught on dinah-316, Agent Code Review round two, 2026-09-05.

A verb whose write phase composes several separately locked steps can refuse at
the third step, having written the first two. The refusal then travels back
through the ordinary error path and the command prints the refusal sentence and
nothing else, which is byte for byte what the same command prints when the same
refusal is raised during validation with nothing written at all. The operator
has no way to answer the only question that matters at that moment, which is
whether the command touched the workbench.

The shape hides well, because the validation-phase refusal and the write-phase
refusal are the same refusal constant, raised by the same predicate, rendered by
the same catalog entry. Nothing in a diff distinguishes them. It hides even
better when the command's own doc comment asserts the identity as a feature.

**Wrong.** `dinah reshape` prints `Nothing was written.` at the top of a preview
and `This workbench now carries the new shape.` at the top of an applied run,
and on a write-phase refusal prints neither, only `dinah.occupied: aftercare`.
The command's doc comment says "A refusal reads the same on either form, because
the phase that raises one is the phase both forms run", which was true before the
write phase gained refusals of its own and is not true afterwards.

**Right.** Carry the fact that the write phase had begun out with the error, and
say so on the error path: which steps had already been applied, and that the
remedy is to clear what the refusal names and run the same source again. The
convergence argument the code rests on is only worth anything if the operator is
told they are standing in the state it describes.

**How to test it.** Drive a second writer through the window with a seam, take
the refusal, and assert on what the command PRINTS rather than on the workbench
state. A test that only asserts the workbench converges passes on a command that
converges silently, which is the defect.

**A related trap in the same family.** `dinah check` is not a substitute for
saying so. After the half-applied reshape above, check reported
`check.kind-out-of-position` against two columns that had nothing wrong with
them, because the added column had been appended to the end of the sequence and
the reorder step never ran. Check spoke, and what it said was a false lead. A
partial state that a general-purpose checker describes in its own vocabulary is
not the same as a partial state the operator can recognise.


### Addendum to "A refusal raised after the command has already written, reported as though it had not" (dinah-316, implement round three)

That entry says the class can only be tested by driving a second writer through a window, which needs a test hook the command surface cannot reach. That is true of the race that motivated it and not true of the class, and taking it as true of the class pushes the whole proof down into the library, where nobody asserts on what the operator actually reads.

A half-applied run is reachable from the command line with no hook at all, because the write phase takes locks it does not own. Plant a lock file by hand in the directory of an entity a later step has to write, let an earlier step write something first, and the run refuses `dinah.locked` with part of its work on disk. `cmd/dinah/reshape_test.go:TestAReshapeRefusedAfterItHadWrittenSaysSo` does exactly that: it retires an occupied column and adds a new one in the same definition, so step one writes the added column before the carry meets the lock, and the test then asserts on stdout, on which line did NOT appear (neither the preview's "nothing was written" nor the apply's "carries the new shape"), and on the refusal still leading stderr. Removing the lock and running the same command again finishes the run, which is the recovery the report promises, asserted rather than described.

The general form: where a command composes acts that each take their own lock, an unowned lock is a seam the command line can already reach, and it is a better seam than a test hook because the assertion can be made on what the operator sees.

The related trap the entry names, that a general checker describing the partial state is not the same as the operator being told, now has a measurement behind it rather than an argument. `dinah check` over the half-applied state reports `check.kind-out-of-position` against the done columns, which nobody touched, because the added column was appended past the terminal region and the check attributes a two-column relation to the earlier column rather than to the newcomer. The checker is not silent, it is confidently wrong about which columns are at fault.


## A repository sentence asserting a fact about a live surface the same card changed off-diff

Caught on dinah-385, agent code review, 2026-09-05. This is a second catch of the class recorded under "A claim swept out of every place that names it, while a sentence resting on the same premise survives", with one difference that defeats every mechanical check the earlier entry can lean on. The change that falsified the surviving sentence never entered the diff.

dinah-385 retired a founding sentence from eight sites. Seven were files in the tree. The eighth was the Dinah workbench's own standing instructions, which is live content with no repository file and no revision history, so it was written through the board rather than committed. A guard in the tree carries that same wording as a retired claim, and the reason it records for carrying it names the field explicitly: the sentence "still stands in the Dinah workbench's standing instructions and is therefore the likeliest text copied back into a design document". The write emptied the field of that sentence and the reason went on asserting it. No part of the diff, the suite or the pull request's checks could see the contradiction, because the edit that created it is not in the repository.

The `why` string is also what the guard prints when it fires, so the falsehood is user-facing rather than a comment nobody reads, and a maintainer who checks the field, finds nothing there, and concludes the entry has gone stale may delete a guard that is still correct.

**Wrong**, in `cmd/dinah/design_claim_guard_test.go`, after the same card removed the sentence from the field:

```
why: "this is the founding sentence's own wording, which still stands in the
Dinah workbench's standing instructions and is therefore the likeliest text
copied back into a design document; ..."
```

**Right**, keeping the entry and restating what now makes it worth keeping:

```
why: "this is the founding sentence's own wording. It stood in the Dinah
workbench's standing instructions until dinah-385 replaced it there, and
older drafts, transcripts and card history still carry it, so a design
document can still acquire it; ..."
```

How to catch it without a second reviewer. A card that changes live content as well as files owes the tree a read after the live write, not before it. Search for the name of the surface you changed ("standing instructions", the document's title, the column's name) rather than for the text you removed, because a sentence about a surface usually describes the text instead of repeating it. A card whose handoff says "the guard never reads this field, so a green suite is no evidence here" has already identified the blind spot and should turn the same observation on the guard's own prose.


## A whole-collection question asked at page scope

Caught on dinah-374, cycle 1 and again in cycle 2's sweep, which is what makes it an entry rather than a card comment. The two catches wear different clothes and are the same mistake, so an entry naming only the first shape would have missed the second in the same file.

A paged HTTP API answers a question about a collection one page at a time, and code that asks such a question has to say how it crosses the boundary. Code that says nothing still runs, and it returns the right answer for as long as the collection fits in one page. The defect is therefore invisible at the moment it is written, invisible in review, and invisible in every run until the collection grows, and this repository's release count grows on most pushes to the trunk.

**Wrong, shape one.** A reducing filter handed to a paged fetch. `gh api --help` states that under `--paginate` "each page is a separate JSON array or object", so the filter runs once per page and a reduction produces one answer per page:

    TAG=$(gh api "repos/$REPO/releases" --paginate \
      --jq '[.[] | select(...)] | sort_by(.created_at) | last | .tag_name')

With 92 releases and a hundred to a page this wrote one correct tag. At `per_page=30` it wrote four tags, and `echo "tag=$TAG" >> "$GITHUB_OUTPUT"` then appended an undelimited multi-line value, which the runner refuses several steps later with a complaint about the output file's format rather than about the lookup.

**Wrong, shape two.** A fixed page requested with no pagination at all, which asks the same question at the same scope without any filter being at fault:

    gh api "repos/$REPO/commits/$SHA/check-runs?per_page=100" --jq '...'

**Right.** Let the filter select and map without reducing, so the pages concatenate into one stream, and reduce on the far side where the whole stream is in hand:

    TAG=$(gh api "repos/$REPO/releases" --paginate \
      --jq '.[] | select(...) | .created_at + " " + .tag_name' \
      | sort | tail -n 1 | cut -d " " -f 2)

Then refuse the failure by name instead of letting it travel, because a per-page answer is several lines and every downstream guard tests for emptiness rather than for arity:

    if [ "$(printf '%s' "$TAG" | wc -l)" != "0" ]; then
      echo "::error::the dev release lookup returned more than one tag, so it answered per page rather than over the whole collection"
      exit 1
    fi

Two things about the correction are worth carrying. The first is that sorting on a documented fixed-width field lets the shell do the reduction, so the fix needs neither `jq` on the runner nor a `gh` new enough for `--slurp`. The second is that a guard written against shape one does not see shape two: a check that walks the `--jq` arguments of every `--paginate` read skips a call that never asked to paginate, which is exactly the call the sweep found next.

The test that catches it: for every call against a paged endpoint, ask what the code would print if the collection were three pages instead of one, and make the collection three pages to find out. Shrinking the page size is the cheap way, because `per_page` is a documented parameter and it needs no fixture invented. Run the old text and the new text back to back at the small page size and compare what each writes, rather than running the new one alone and reading its single line as proof.


## A carve-out from an escaping rule, defeated by the second condition that triggers the same quoting

Caught on dinah-283, cycle 2. It is the repair-side twin of "A universal escaping claim verified in only some of the shells it will be pasted into": that entry is about an escaping rule claiming more than it delivers, and this one is about what happens when the author narrows such a rule and carves characters out of it.

A quoting helper usually has more than one trigger. It quotes a token that is empty, a token carrying whitespace, and a token carrying one of a declared set of characters. When a review shows that quoting cannot make some character inert, the tempting repair is to take that character out of the declared set and write down that tokens carrying it are left alone. The carve-out then holds only for a token whose sole trigger was that character. A token that also carries whitespace is quoted by the other trigger, the carved-out character lands inside the quotes anyway, and the sentence describing the carve-out is false for the input the helper sees most often.

**Wrong.** Remove the character from the trigger set, and describe the consequence as though the trigger set were the only way a token gets quoted.

```go
// A backslash is an escape inside POSIX double quotes and an ordinary
// character inside cmd.exe's, so doubling it repairs the line for one shell
// and doubles every separator of a Windows path for the other. A token
// carrying any of the four is rendered as it stands and Line claims nothing
// about it.
const shellSpecial = "\"'|&;<>()*?[]#~="

func quoteArgument(arg string) string {
	if arg == "" {
		return `""`
	}
	plain := !strings.ContainsAny(arg, shellSpecial)
	if plain && strings.IndexFunc(arg, unicode.IsSpace) < 0 {
		return arg
	}
	return `"` + strings.ReplaceAll(arg, `"`, `\"`) + `"`
}
```

`quoteArgument("C:\\temp")` does render as it stands. `quoteArgument("dir C:\\temp\\")` renders as `"dir C:\temp\"`, whose closing delimiter is preceded by a backslash, so a POSIX shell and the Windows C runtime both read the quoting as continuing past it. Two arguments then read as one. The carve-out was written for Windows paths, and a Windows path with a space in it is the case it breaks.

The guard added alongside the carve-out cannot see this, because it walks the declared set and asserts the carved-out characters one at a time:

```go
for _, special := range shellSpecial {
	token := "a" + string(special) + "b"
	// round trip
}
if got := quoteArgument(`C:\Users\paul`); got != `C:\Users\paul` {
```

Every row exercises one trigger. The defect needs two.

**Right.** Enumerate the triggers, then check the carve-out against each of them rather than against the one it was written for. Where a carved-out character still reaches the quoted branch, say what the rendering does with it there, and give the guard a row whose token fires both triggers at once.

```go
// A token carrying one of the four is never quoted on account of that
// character. Whitespace or a set character quotes it anyway, and the four
// then sit inside the quotes with their shell meanings intact, which is
// what this rendering makes no claim about. A trailing backslash is the
// case that costs a reader the boundary, and <the chosen answer> is why
// the delimiter still terminates.
```

```go
{"a windows path with a space", `dir C:\temp\`, ...},
{"a title carrying a variable", `raise the $HOME limit`, ...},
```

**The test:** for each character you carve out of a quoting rule, write a token that carries it *and* fires one of the helper's other triggers, and read what comes out. If the carved-out character ends up inside the quotes, the sentence "rendered as it stands" is false, and if the character can reach the position adjacent to the closing delimiter, the boundary itself is unreadable. A carve-out is a statement about the whole helper, not about one branch of it.

## A guard that varies one trigger at a time, over a behaviour with two

Caught twice on dinah-283, in consecutive review rounds, on the same function. The class is a test whose table walks one input dimension while holding every other at a neutral value, guarding a behaviour whose branches are reached by more than one dimension at once. Such a guard is green for every defect that needs two triggers together, and it is green loudly, because the table looks exhaustive and its rows are named after the thing being guarded.

**Wrong.** The behaviour under test quotes a token when it carries whitespace OR when it carries a character from a declared set, and escapes inside the quoting. The guard walks the set:

```go
for _, special := range shellSpecial {
    token := "a" + string(special) + "b"
    rendered := quoteArgument(token)
    if got := readBack(rendered); got != token {
        t.Errorf(...)
    }
}
```

Every row is a single-character token with no space in it and no character outside the set. Two defects lived in the space the rows never reach. First, a token quoted for its spaces carries an excluded character inside the quoting with its meaning intact, so a documented carve-out ("a token carrying one of these is rendered as it stands") was false for the commonest input on the board, a card title. Second, a token that both carries a space and ends in a backslash renders with its closing delimiter escaped by that backslash, so the printed line reports one argument where there were two. Neither needs an exotic input. Both need two ordinary ones at once.

**Right.** Enumerate the triggers and the hazards as fragments, combine them, and assert the invariant that must survive every combination rather than the value each row happens to produce:

```go
fragments := []string{
    "", "a", "a b", "  ", "\t",
    `\`, `\\`, `C:\temp`, `C:\temp\`,
    `"`, `a"b`,
    "$HOME", "`id`", "%PATH%",
    "|", "*", "#", "'",
}
for _, head := range fragments {
    for _, tail := range fragments {
        token := head + tail
        line := Command{Verb: "add", Args: []string{token, "next"}}.Line()
        // three readers, one per shell family, must agree on the count
    }
}
```

Three things make this the right shape rather than a bigger version of the wrong one.

The fragments are chosen per trigger and per hazard, not per character, so the table stays small and stays readable. Eighteen fragments and 324 tokens run in well under a second, and the bound is stated in the test's own comment, which is what the workbench asks of anything that sweeps.

The assertion is an invariant rather than an expected string. A combination sweep cannot carry a hand-written expectation per row without becoming a second implementation, so it asserts the property that has to hold for every row (here, that three independent readers agree on how many arguments the line describes) and leaves the literal renderings to a short named table beside it. Keep both. The sweep says a property held and the named rows say what the output actually is.

Reading it back with more than one reader is what stops the guard healing the defect. A single reader written against the same rules as the writer agrees with the writer by construction, including where both are wrong. Modelling each consumer separately (here the Windows C runtime, a POSIX shell and PowerShell) is what turns "the code is self-consistent" into "the code is right for the thing that will read it", and it is also what forces the honest admission when the consumers genuinely cannot all be satisfied.

**The test to run on your own guard.** Take the defect the guard exists to catch and ask how many of the input's properties it needs at once. If the answer is more than one and every row in the table sets exactly one, the guard cannot catch it, however many rows there are. Adding rows in that shape buys coverage of the dimension already covered.

**Related.** [[A compound guard whose second condition no fixture ever supplies]] is the same blindness one level in, where the guard's own predicate has two conditions and the fixtures only ever reach one. [[A carve-out from an escaping rule, defeated by the second condition that triggers the same quoting]] is the production-side twin, and it is the defect this entry is the test-side answer to. Both were filed off dinah-283, which produced the pair.


## A rule stated in several places, corrected in all but one

Sharpening of "A guard that varies one trigger at a time, over a behaviour with two" and of "A refusal reused across shapes where its sentence is true of only some of them", rather than a sibling of either. Round three's reviewer on dinah-283 asked for exactly this: the shape survived into round four, so the entry it belongs to gets sharper wording instead of a new neighbour.

The class is a rule that is written down in more than one place, and a correction that lands on every place but one. On dinah-283 one quoting rule was stated in five: an exported doc comment, a test's own comment, that test's skip condition, the handoff note, and the card's spec. Round three's correction reached four of the five, and the one it missed was the exported doc comment, which is the only one a consumer of the library ever reads. Round four then repeated the shape one layer along: the spec's prose section was corrected while the Go block earlier in the same spec, summarising the same function, kept the old claim.

WRONG, and it reads as finished because the sentence in front of you is now true:

    // ... the boundary holds for every token that carries no double
    // quote of its own.          <- corrected here in round three

    if strings.ContainsAny(token, "\"`") { continue }   <- and here

    "for every token carrying neither a double quote nor a backtick"  <- and here

    // Line joins Verb and Args into one printable line ...   <- not here

RIGHT: when you correct a claim, find every statement of that claim before you correct any of them, and correct the set. Grepping the corrected wording finds the places already fixed; what finds the rest is grepping the OLD wording, and then reading for the claim in places that use none of its words, such as a summary, an abstract, or a code block quoting the same comment in shorter form.

The test that catches it: after the edit, ask how many places state this rule, name them, and say which one a stranger reaches first. If you cannot list them, you have not finished looking. On this card the answer to "which one does a stranger reach first" was the exported doc comment both times, and both times it was the one that still carried the old claim.


## An exclusion list justified by one property, reused to bound a different one

Filed from dinah-283, round five, after the shape survived four review rounds and the operator ruled on the remedy. The round-four reviewer drafted this pair and held it back until the ruling settled which of the two remedies was right; this is that pair with the ruling applied.

**Wrong.** A set is assembled to answer one question, and the list of what it excludes is then quoted verbatim to bound a different promise:

```go
// shellSpecial ... Some stay out because quoting would not help: a POSIX
// shell expands $ ... and cmd.exe expands %VAR% there.

// Line ... That boundary is the property Line holds, and it holds ... for
// every token that carries neither a double quote nor a backtick of its own.
```

The exclusions were derived from inertness, meaning whether quoting makes a character stop meaning something to a shell. The sentence that reuses them promises the boundary, meaning how many arguments the line describes. Those are different properties with different answers, and the reuse is invisible because both lists are short, both are about the same characters, and the second sentence reads as a summary of the first. `$` was correctly excluded from the first list and wrongly excluded from the second: quoting cannot stop a POSIX shell expanding it, and quoting does stop the shell field-splitting the result, so a bare `$HOME` is two arguments when the value has a space in it and none when the variable is unset.

**Right.** Derive each list from the property it bounds, walk every candidate against both questions separately, and let the two memberships come out different:

```go
// shellSpecial ... One question decides whether a character may be a member,
// and it is the boundary: nothing belongs here unless wrapping the token in
// double quotes leaves the argument count right in all three shells.
// Whether the quoting also leaves a character meaning itself is a second
// question with a different answer.
```

The walk moved `$` and `%` into the trigger set and turned up two more characters, `!` and `^`, that the old boundary sentence had been promising silently. Naming two lists with two memberships is what stops the next reader collapsing them again.

**How to catch it.** Where a comment excludes a set of things "because X does not help", find every other sentence that quotes that same exclusion list, and ask of each whether X is the property that sentence is about. Where it is not, the list is being borrowed rather than derived, and the borrowing is a defect even when the memberships happen to coincide today.

**The guard shape that lets it through.** A test that asserts the excluded characters are absent from the set passes under both readings, because it checks membership rather than the reason for membership. Make the exclusion assertion state the property in its own failure message (`quoting does not restore the argument boundary for it`, rather than `quoting cannot render it inert`), so a later reader planting the wrong reason sees the right one named in the red.


### Addendum to "An exclusion list justified by one property, reused to bound a different one" (dinah-283, review round five)

The entry above prescribes walking every candidate against both questions separately. The round that applied it did walk, and the walk still missed a character, because the candidates it walked were the ones the old comment already named rather than the host language's own list of constructs.

**Wrong.** The rewrite re-derives the two memberships honestly and takes its candidate set from the sentence it is replacing:

```go
// Three characters a shell reads specially stay out, each for its own
// reason. A backtick ... An exclamation mark ... The caret ...

// Line ... it holds ... for every token that carries none of a double
// quote, a backtick, an exclamation mark or a caret
```

The backtick came from the old exclusion list, and the `!` and the `^` came from a sentence in the same comment block that had mentioned them in passing. Nothing in the round consulted bash's own list of expansions, so brace expansion was never a candidate and never got asked either question. A bare `{a,b}` is four arguments where a quoted one is three, and quoting suppresses it by a documented rule, so the braces are members by the set's own admission test and appear in neither list. The sweep could not see it either, because its fragment table is built from the same inherited candidate set.

**Right.** Take the candidate set from the host language rather than from the code you are correcting. Enumerate what the language documents as changing a word into more or fewer words, which for a shell is the expansions its manual lists (parameter, command substitution, arithmetic, tilde, brace, filename) plus its operators and its escape and quoting characters, then ask both questions of each one and record the answer even where it is "no change". A walk that starts from the comment being fixed can only ever rediscover the characters that comment already thought about.

**How to catch it.** Ask where the list of candidates came from, and refuse "the previous revision" as an answer. On this card the same defect was found in three consecutive rounds, each time as one more character nobody had asked about, which is the signature of a walk with an inherited domain rather than of a hard problem.


## A widening measured against the existing corpus, read as measured in both directions

Filed from dinah-386, round three, where the corpus agreed with four candidate
repairs and one of them regressed the guard anyway.

The card widened a text-reading guard. A reviewer named two possible repairs
and asked for both to be measured before either was taken. Both were measured
against all 44 entries of the guard's own must-catch and must-pass arrays, and
every entry agreed under both. One of the two repairs nevertheless stopped the
guard catching five ordinary claims, which the arrays contained no case for
because nobody had ever written one.

**Wrong.** Widening a check, running the project's existing case corpus against
the old and new behaviour, finding no disagreement, and reporting the change as
measured. The corpus was built from the cases earlier rounds happened to think
of, so it cannot disagree about behaviour nobody has written a case for, and a
widening changes exactly that behaviour.

**Right.** Ask what the change newly makes possible, write the sentence or the
input that exercises it, and measure that. Then put it in the corpus in the
same commit, on both sides: the case the change must now catch, and the case it
must still let through. On dinah-386 the adversarial case took one sentence to
write once the question was asked out loud ("this extension ships dinah no
matter which platform you are on"), and it was the difference between the
repair that shipped and the one that did not.

The corpus agreeing is a floor and not a result. It says the change broke
nothing anybody thought of before, which is worth knowing and is not the claim
"this change is safe". Say which of the two you have.


## A hand-built list of hazards, where an allowlist was available

Caught on dinah-283 at Implement, 2026-09-06, after the operator ruled on it. Five consecutive review rounds of one function each found one more character missing from a hand-written list of shell-hazardous characters, and every fix was correct. The sixth character was always going to be there, because the list was written by people reasoning about characters they had thought of.

Wrong, and it looks careful rather than careless:

```go
// shellSpecial is the set of characters that make Line quote a token.
const shellSpecial = "\"'\\|&;<>()*?[]#~=$%"

func quoteArgument(arg string) string {
	if !strings.ContainsAny(arg, shellSpecial) && noWhitespace(arg) {
		return arg
	}
	return quote(arg)
}
```

Right:

```go
// shellInert is the set of characters a token may be made of and still be
// rendered without quotes. Everything else is quoted, including characters
// nobody has considered, which is the point.
const shellInert = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789.-_/"

func quoteArgument(arg string) string {
	if inertThroughout(arg) {
		return arg
	}
	return quote(arg)
}
```

The test is not whether the list is complete today. It is what happens to a value carrying a character the list has never named. A hazard list ships it bare and the failure is silent and wrong; a safe list quotes it and the failure is a pair of quotes nobody needed. Where the two are available for the same question, take the one whose unknown case is noisy.

Two conditions before reaching for this. Each member of the safe set has to be defensible from documentation of every system the value crosses, not from a feeling that it looks harmless, and a character no document settles stays out. And the safe list has to be short enough to state, which is what makes it reviewable: the wrong version above ran to eighteen characters and was still incomplete, and the right one is four beyond the alphanumerics.

Related: the entry on an exclusion list justified by one property and reused to bound a different one, which is the same defect one layer down.

## A reader in a test that vouches for a construct it does not model

Caught on dinah-283 at Implement, 2026-09-06, before it shipped, by re-reading the promise sentence and asking what would have to be true for it to be false.

A test in that card renders a token and reads the rendered line back with three hand-written models of shell parsing, asserting the argument count each model sees. The models are honest about their own limits in their comments. One of them expands `$name` and says so, and states outright that `${...}` and the special parameters are read as literal text, which is not what a shell does with them.

The defect is what happens when a later edit widens the token table past what the model covers. Adding a `@` fragment beside the existing `$` fragment produced the token `$@`, which POSIX documents as the one expansion producing several fields inside double quotes, and adding a `{a,b}` fragment produced `${a,b}`, which a shell reads as an unfinished parameter expansion. The model read both literally, answered three tidy arguments, and the sweep passed while asserting something a real shell contradicts. Nothing in the test was wrong when it was written, and nothing announced that it had become wrong.

Wrong:

```go
for _, token := range every pairing of the fragment table {
	line := render(token)
	if got := splitPOSIX(line); len(got) != 3 {
		t.Errorf(...)
	}
}
```

Right:

```go
for _, token := range every pairing of the fragment table {
	line := render(token)
	// splitPOSIX models $name and nothing else, so these three shapes are
	// pinned by name elsewhere and skipped here rather than passed. Letting
	// the reader answer for them vouches for a line that does not parse.
	if strings.Contains(token, "$(") || strings.Contains(token, "${") || strings.Contains(token, "$@") {
		continue
	}
	if got := splitPOSIX(line); len(got) != 3 {
		t.Errorf(...)
	}
}
```

The rule this yields: a model in a test owes an explicit skip for every construct it does not implement, and widening the input table means re-reading the model's own stated limits against the new inputs. A skip is not a weakening. It moves the claim from a reader that cannot see the case to a named pin that states what the case actually does, which is the only honest place to put it.

Note the arming asymmetry, because it is what makes this class survive review. The skip cannot be armed by breaking it: removing the skip makes the test pass, not fail. A guard whose absence produces green is invisible to every technique that looks for red, so it has to be caught by reading.


## An exception list left as an enumeration after the trigger became a rule

Caught on dinah-283 at Agent Code Review, cycle 6, 2026-09-06, one round after the entry above was written. It is an addendum to "A hand-built list of hazards, where an allowlist was available" rather than a separate class, and it records what that inversion does not reach.

Inverting a hazard list into a safe list fixes the trigger. It does not touch the second enumeration most such code carries, which is the list of cases the treatment fails to rescue. That list cannot be inverted, because there is no allowlist of exceptions, so it stays a hand-written enumeration and it goes on leaking in exactly the way the trigger no longer does. On dinah-283 the trigger became a rule and the promise sentence beside it kept naming instances, and one round of review found two of them.

Wrong, and it reads as a careful reorganisation:

```go
// The boundary holds under three conditions. The token carries none of a
// double quote, an exclamation mark or a caret. The token opens no expansion
// a shell reads past the closing delimiter to finish, which means no
// unmatched backtick, no unmatched $( and no unmatched ${. The token carries
// no $@, which POSIX documents as the one expansion producing several fields
// inside double quotes.
```

Right:

```go
// The boundary holds under three conditions. The token carries none of a
// double quote, a backtick, an exclamation mark or a caret; a balanced pair
// of backticks is no safer than an unmatched one, because PowerShell escapes
// whatever stands after each backtick. The token opens no expansion a shell
// reads past the closing delimiter to finish. The token carries no expansion
// of the @ parameter in any spelling, which POSIX gives its own rule inside
// double quotes: $@ and ${@} are the same expansion, and bash spells a third
// as ${name[@]}.
```

Two mechanisms produced those, and both are worth knowing separately.

The first is the one the parent entry already describes, applied to the exception rather than to the trigger. `$@` was written as the literal bytes a card had discussed, where the host language defines a parameter that several spellings reach. A reader holding the sentence concludes that `${@}` is covered, and it is not. When you write an exception, name the construct the language names, and then check the language's own list of ways to spell it.

The second is new here. A rewrite that groups a flat exclusion list into classes will quietly attach each class's qualifier to every member it swept in, and a member that was excluded unconditionally comes out excluded only under that qualifier. The backtick had been added to the unconditional list two rounds earlier, at the cost of a review cycle. Grouping it under "no unmatched backtick" narrowed it back, and the branch's own test still pinned a balanced pair breaking the boundary in PowerShell, so the code and its documentation contradicted each other in the same commit.

The read that catches both is cheap and neither reviewer nor author should skip it. Take each condition in the finished sentence, construct the smallest value that satisfies every condition and still breaks the promise, and go looking for it in the tests. Where the tests already pin the counterexample, as they did here, the sentence is falsified by the diff it shipped in.


## A printed count reports the value that was requested rather than the value the run used

Caught at Agent Code Review on dinah-381, 2026-09-06. A measurement harness took `--cards N`, built a fixed two-card fixture, truncated the worked set with `cards[:N]`, and then printed the header line from `args.cards`. Asked for three cards it printed "3 cards in the sequence", worked two, and exited zero.

Wrong:

```python
parser.add_argument("--cards", type=int, default=2)
...
fixture.cards = fixture.cards[:args.cards]
report.measure("cards in the sequence", args.cards, "cards")
```

Right:

```python
fixture.cards = fixture.cards[:args.cards]
if len(fixture.cards) != args.cards:
    raise Failure("the fixture builds %d cards and %d were asked for"
                  % (len(fixture.cards), args.cards))
report.measure("cards in the sequence", len(fixture.cards), "cards")
```

The rule this teaches: a figure a tool prints is a fact of the run, so derive it from what the run actually did and never from the argument that asked for it. Where the two can disagree, either refuse the request or report the outcome, and do not report the request. The failure is quiet because the requested value is usually also the used one, so the printed number is right on every path anybody exercises and wrong on the one nobody does.



## A narrow mechanical check is reported under the name of the broad standard it does not cover

Caught at Agent Code Review on dinah-381 and confirmed by the implementer's own diagnosis at the fix round, 2026-09-06. A handoff note's self-test section carried the line "House-style scan on the staged diff, with the control line armed each time: CLEAN on all three commits." The scan matched em-dash and smart-quote characters and nothing else. It could not see a serial-comma miss, a sentence fragment, or a machine-prose tell, and the tree it passed carried seven misses of a convention this workbench has ruled. The check itself was correct and its control line did arm; what was wrong was the name its result was reported under.

Wrong:

```
- House-style scan on the staged diff, with the control line armed each time: CLEAN on all three commits.
```

Right:

```
- Typographic character scan (em-dash, smart quotes) on the staged diff, control line armed each time: no match on any commit. This scan cannot see a serial-comma miss, a fragment, or a machine-prose tell.
- House-style sweep, as a read: candidates for the serial comma were produced mechanically over both files and every candidate was read; seven true three-item lists were fixed and the rest are two-element coordinations. Triads, closers and colon-splices were read sentence by sentence in the sections touched.
```

The rule this teaches: name a check after what it actually matches, not after the standard it is standing in for. A reviewer reading "house-style: CLEAN" stops looking, so a narrow check wearing a broad name buys silence rather than coverage, and it buys it in exactly the place where the missing coverage lives. Related failure, already in this corpus: a checker whose blind spot makes a file read clean. This one is its mirror, because the checker is sound and the label is what lies. The Prose standard's own rule applies on top of it, that a sweep is a read rather than a count and that a report naming no candidates cannot be told apart from a check that never ran, so a clean sweep reports which candidates it weighed and why they stood.




## A refusal's named values are lifted out of the call and the guard that reads them goes blind

Caught by the build on dinah-366's fix round, 2026-09-06, after Agent Code Review recommended the shape. The call site at `internal/bench/bench.go` that hands `admitSlug` its anchor map had grown to 157 columns, over the Go style standard's roughly-100-column rule, and the reviewer proposed lifting the map literal to a named local above the `if`. Doing that turned `TestEveryRefusalIsComposedTheOneWay` red on both of `admitSlug`'s raise sites.

The guard, check 5 in `internal/profile/guards_test.go`, resolves a refusal's third argument to the strings inside it along four routes. When the raise site names a parameter, the route is `readParameterMap`, and it reads the parameter through the composite literals the function's callers write in that argument position (`call.arguments[position].(*ast.CompositeLit)` at guards_test.go:3887). An identifier in that position is not a composite literal, so the route answers `reachOpaque` and the site is reported as one whose English the check cannot see. The fix for the line length is to break the literal across lines where it stands, which leaves the guard's operand intact and puts every line under the limit.

Wrong:

```go
slugAnchor := map[string]string{
	"path":                  path,
	contract.ValueWorkbench: root,
	contract.ValueColumn:    column.ID,
}
if err := admitSlug(column, major, seenSlug, slugAnchor); err != nil {
```

Right:

```go
if err := admitSlug(column, major, seenSlug, map[string]string{
	"path":                  path,
	contract.ValueWorkbench: root,
	contract.ValueColumn:    column.ID,
}); err != nil {
```

The rule this teaches: a refusal's named-value map is read by a source-walking guard rather than by a type, so where the map lives is part of what makes it checkable and a refactor that only moves it can switch the check off. Before giving such a value a name, ask which guard reads the shape you are about to change. The general form is already in this corpus, that a fixture or a guard walking source keys on the shape of the source and not on its meaning; this entry is the case where the shape in question is an argument's syntax at one call.


## A yes-or-no question asked through a pipeline whose right-hand side exits early, under pipefail

Caught on dinah-398, cycle 2, and it is the third catch of the class the entries "An instrument that answers zero for input it cannot read" and "A refusal that tests whether the tokenizer splits the term, on a tokenizer that can also merge it" record. Those two are about tokenizers. This one is about a shell pipeline, and it is worth its own pair because a reviewer who has read both of those will not recognise the shape when it arrives spelled in `grep`.

The setting is a guard that has just been repaired. Round one of that card asked whether a release existed by reading a non-zero exit as absence, which spells "nothing there" and "could not look" the same way, so round two replaced the instrument with a listing and a membership test. The repair closed the direction the reproduction used. It opened a second one in the same expression, because the membership test is a pipeline and the step runs under `set -o pipefail`.

**Wrong.** Asking the membership question as a pipeline whose right-hand side is `grep -q`:

    set -o pipefail
    if printf '%s\n' "$TAGS" | grep -Fxq "$TAG"; then
      # the tag is present
    else
      # the tag is absent
    fi

POSIX documents `grep -q` as exiting as soon as a line is selected, and bash documents `pipefail` as making a pipeline's status "the value of the last (rightmost) command to exit with a non-zero status", and it documents a command killed by signal N as exiting 128+N. Put those three together and a match closes the pipe under the writer, the writer dies on SIGPIPE with status 141, `pipefail` promotes that 141 over `grep`'s own 0, and the `if` takes the else branch. A present tag is then spelled exactly the way an absent tag is spelled, which is the defect the round was there to remove.

It is invisible while the data is small. The writer finishes before the reader can exit whenever the payload fits in the pipe buffer, so every test, every local run and every year of production reads correctly until the listing outgrows the buffer. Measured on the card: a two-line list matched at 0, and a 368 KB list matched at 141. The same file already carried a comment worrying that this repository's release count "is walking toward the boundary" for an unrelated pagination reason, so the growth was on the page and nobody connected it to the expression two steps below.

**Right.** Do not put a question whose answer you are reading into a pipeline at all. A here-string keeps the reader's early exit from being able to kill anybody:

    if grep -Fxq "$TAG" <<<"$TAGS"; then

Dropping `-q` and redirecting to `/dev/null` also works, because a `grep` that reads its whole input never closes the pipe early. Reach for the here-string first, since it removes the second process rather than arranging for it to survive.

**The test:** for any pipeline under `pipefail` whose status you branch on, name every command in it that can exit before its input is consumed, and say what the pipeline returns when one of them does. `grep -q`, `head`, `sed q` and `read` are the usual four. Where the answer is a status other than the one you are branching on, the pipeline is the wrong shape and the fix is to stop using one. Prove it by feeding the pipeline a payload larger than a pipe buffer rather than by feeding it the fixture, because the fixture is exactly the size at which this defect cannot appear.

## An assertion over a process's concatenated streams, where the property under test is which stream carried the text

Caught on dinah-399, cycle 2. This is the stream-level form of the healed-split assertion the board already knows from a test that concatenated the pieces back together before checking them.

A test that runs a program and searches `stdout + stderr` for a phrase asks only whether the phrase appeared somewhere. That is the right question when the phrase is the deliverable. It is the wrong question when the fix under test decides which stream the phrase travels on, or which of the program's own reporting paths carries it, because the fixed program and the broken one both put the phrase in the concatenation.

**Wrong.** A PowerShell script captures a helper's output and reports every failure through one `Fail` helper. The fix under review adds `2>&1` to the capture so the helper's stderr message reaches the variable and `Fail` carries it:

    $version = $tagsOutput | node scripts/print-newest-version.mjs - 2>&1
    if ($LASTEXITCODE -ne 0) { Fail "$version" }

The test asserts:

    assert.match(`${run.stdout}${run.stderr}`, /no extension release exists yet/iu);

Delete the `2>&1` and that assertion still passes. PowerShell sends an unredirected native command's stderr straight to the host's stderr, so the phrase reaches `run.stderr` on its own, `$version` is empty, and `Fail ""` still exits non-zero. Measured on pwsh 7.6.5: without the redirection the phrase is printed before the error record and the record's own message is blank, and with it the phrase sits inside the record. The suite cannot tell the two apart, so the redirection is guarded by nothing.

**Right.** Assert the property, which is that the message travelled through the script's own reporting path. The two worlds differ in where the phrase sits relative to the error record, so assert on the stream that is supposed to carry it and on the record around it:

    assert.match(run.stderr, /Fail[\s\S]*no extension release exists yet/u);

That fails when the phrase arrives ahead of the record, which is what an unredirected capture produces.

**The test:** before writing `stdout + stderr`, say what the broken version would print. When the answer is the same characters in a different place, the concatenation erases the one difference the check exists for, and the assertion has to name the place rather than the characters.

## A guard family ported from a sibling module, with one member left out and nothing saying so

Caught on dinah-379, Agent Code Review. The card ported `internal/msg`'s translation guards to the VS Code extension's two new catalogues. Design review had already caught one member of the family missing a round earlier, when the spec carried no staleness fingerprint and never named the gap as a cut, so this is the second catch of the class on the same card.

`internal/msg/msg_test.go` declares fourteen tests over its catalogues. The extension's port carries analogues of five of them: key parity, the skeleton and verbatim honesty flags in both directions, the glossary terms, the staleness fingerprint, and the regional-tag walk. Three it does not carry are `TestATranslationKeepsThePlaceholdersAndTheSplice`, `TestEveryKeyCarriesAContext` and `TestEveryUntranslatableIdentifierSurvivesTranslation`. All three properties hold in the shipped catalogues, checked by hand at review, so nothing on the branch is broken. What is missing is the sentence saying which members of the family were ported and which were not.

**Wrong.** Prose that names the source and, sitting beside four sibling guards, implies the whole of it:

    // This is internal/msg's TestATranslationTracksItsEnglishSource ported to
    // the extension's two catalogues. Same idea, same two failure messages,
    // separate implementation over separate files

A translator who respells `{holder}` as `{inhaber}` passes every guard in that file. The renderer leaves a brace name nobody passed a value for alone, so the reader sees the brace text on screen, which is the failure the module's own doc comment argues against when it refuses to print a missing key at a reader.

**Right.** Enumerate the source's guards by reading its test file, and for each one either port it or write down why it does not apply:

    // Ported from internal/msg: parity, the honesty flags in both
    // directions, the glossary, the staleness fingerprint. Not ported, and
    // why each: the placeholder guard, which these catalogues do need and a
    // follow-up card owes; the context guard, whose property every entry
    // here holds and no test enforces; the untranslatable-identifier guard,
    // which the manifest's four command URIs will need the first time a
    // translator edits one.

**How to catch it.** When a diff says it mirrors a mechanism from elsewhere in the tree, build the list of the sibling's guards from the sibling's own test file rather than from the diff's prose, and check the port against that list. A missing member is invisible from the port's side, because everything the port does carry is correct and the omission leaves no mark.


### A comment must not justify an ordering with a claim about the wrong statement

Caught at Agent Code Review on dinah-382, 2026-09-06, in `internal/mcp/mcp.go`
and `internal/mcp/chain.go`. The head writes down which instruction layers it
just served, and it writes that record before the answer is serialised. The
comment defends that ordering by asserting what happens when the serialisation
fails. The assertion is true of one encode in the file and false of the encode
the comment is standing next to.

Wrong:

```go
// What the act served is recorded after the tool has answered and before
// the answer is encoded. A failed encode ends the process, so a record
// written here can never outlive an answer the caller never saw.
memory.record(request.Actor, chainServed(payload))
encoded, err := json.MarshalIndent(payload, "", "  ")
if err != nil {
    return nil, err
}
```

That `err` returns to `dispatch`, which turns it into an `rpcError` on the
answer and returns; `Serve` encodes that error response and reads the next
line. The process continues. The encode that does end the process is
`encoder.Encode` in `Serve`, forty lines away and operating on a different
value, so the sentence names a real behaviour of the file and attaches it to
the wrong statement.

Right, either by naming the encode that actually carries the argument:

```go
// What the act served is recorded after the tool has answered. The record can
// outlive an answer the caller never saw only if json.MarshalIndent below
// fails, which it cannot for these types, and the hold's own expiry bounds the
// divergence if it ever did.
memory.record(request.Actor, chainServed(payload))
```

or by moving the write after the serialisation so no argument is needed.

The test is mechanical and worth running on any comment that justifies an
ordering: read the justification, find the statement it names, and check that
statement rather than the one you had in mind when you wrote it. A comment
asserting a control-flow fact is asserting something a reader will not re-derive,
which is the whole reason it earns its place, and a reviewer who accepts it
because the surrounding code is careful has reviewed nothing. On this card the
consequence was nil, because the failure is unreachable and the expiry bound
caps it regardless. The reason it is recorded here anyway is that dinah-382's
own work list includes correcting a doc comment on `serve` that made a false
claim about this same head, so the card fixed one instance of the defect and
shipped another.


## A member absent from an answer, inferred at the renderer from the member's own emptiness

Caught at Agent Code Review on dinah-383, 2026-09-06, in `cmd/dinah/render.go`,
and fixed in the round that followed. `show` gained a field list, so an answer
now carries only the members the caller named. The payload side learned to tell
"the caller did not ask for this" from "the card holds nothing here", because
its marshaller reads the selection. The terminal side did not, and it did not
have to be asked: three of the members it draws are slices or strings, whose
absence and whose emptiness render the same nothing, so guarding on emptiness
looked like a rule that held. One member is a struct, and a struct that nobody
filled in draws a header of empty cells rather than nothing at all.

Wrong:

```go
func (s *session) renderDetail(detail *verb.Detail) {
	s.renderCard(&detail.Card)
	if detail.Body != "" {
		...
	}
	if len(detail.Links) > 0 {
		...
	}
}
```

`dinah show fx-1 --fields links` then opened on the line `    [ / ]`, an empty
reference, an empty title, and an empty column, above the links it was asked
for. The payload of the same call omitted the member outright, so the two heads
of one command disagreed about what one answer held.

Right, by asking the answer what it carries, which is the question the payload
side was already asking:

```go
if detail.Carries("card") {
	s.renderCard(&detail.Card)
}
```

**How to catch it.** When a change makes a member optional, the members that
draw are not the place to look. List every member the renderer touches and sort
them by whether absence and emptiness are the same value at that member. For a
slice or a string they are, and the existing guard survives; for a struct, an
integer, a boolean, or a time they are not, and each one needs the selection
asked directly. A test that shapes its read around a field list including the
struct member cannot see the defect, so the test has to name a list that leaves
that member out, and it has to assert both that the header is absent and that
something else was printed, or it passes on an answer that printed nothing.

**The blast radius is the second half of the finding.** The fix removes a block
that used to draw unconditionally, so every block below it that wrote a blank
line above itself may now be first. Moving the separator to a gap drawn between
blocks is a change to the rendered output of every call the renderer serves,
not only the shaped ones, and it is checked by comparing an unshaped call under
binaries built either side of the change rather than by reading the code and
finding it plausible.
