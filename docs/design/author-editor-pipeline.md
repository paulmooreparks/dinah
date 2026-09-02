# The Author-Editor Pipeline for Agent-Written Technical Prose

A method for getting an LLM to produce technical documents that a demanding human editor will accept, worked out on a functional concept for a retail point-of-sale integration during August and September 2026. This file is written for another agent, or for someone building a harness, and it is self-contained. Project-specific paths and names are given only as examples.

## The Problem It Solves

An agent writing a long technical document produces prose that is accurate sentence by sentence and unreadable as a whole. The same fact is explained from scratch in every section that touches it. Design statements trail justifications. Sentences announce what follows, pair a thing with what it is not, and close on epigrams. The other party's systems get described instead of our own design. Corrections do not stick: an instruction to be concise tightens the sentence in front of the reviewer and nothing else, and the same shapes reappear in the next section, and in the agent's own rewrites of sections it has just tightened.

Two further failures come from trying to fix the first ones. A cleanup pass loses facts, because a deleted clause turns out to have been the only statement of something. And a cleanup pass changes facts, because merging two sentences produces a third claim that neither made.

## The Principle

Separate the work by what is in context, not by who does it. Every failure above came from one agent holding too much at once: the drafter who cannot see its own repetition because the reasons for each sentence are still in its head, the editor who inherits the drafter's justifications, the checker who trusts text it just wrote. An editor that has never seen the sources cannot add a fact. An editor forbidden to change facts cannot lose one silently, because every removal has to be written down with a reason.

The governing rule: accuracy is fixed at the draft, from sources. Every later stage may delete, consolidate and reshape. None may assert.

## The Stages

| Stage | Holds in context | Produces | May not |
| --- | --- | --- | --- |
| Skeleton | The request, the sources, the record of prior decisions, the fact-ownership table | One line per claim, each with its source and its owning section, in the section's question shape | Prose |
| Write | The approved skeleton, two or three exemplar passages, the style rules | One sentence per claim, in the exemplars' register | A sentence with no claim behind it |
| Edit | The draft, the editor's brief, the sentence tests, the banned shapes, the ownership table, the exemplars | The edited draft, an adjudication of every removal, and flags | A new fact, a corrected fact, a restatement of a fact another section owns |
| Verify | The baseline and the edited text | A worklist from mechanical scans | Judgement |
| Apply | The verified text and the document | Exact-match edits, a record entry, a report | Anything the verified text does not say |

Edit repeats while Verify finds anything. Apply runs once.

The human is present twice. The first checkpoint is after the skeleton, for a new section, because the claims are the design and the prose is not. The second is after Apply, where the report says per section what came out and which test it failed. The human is not present in between, and never sentence by sentence. A correction whose claim is the request itself skips the first checkpoint.

## The Skeleton

Each section answers a fixed question and no other. For a design section: what the requirement is, and what our design does to meet it. Baseline behaviour of the underlying product appears only as a clause where the design is unintelligible without it. What the other party's systems do appears nowhere; where the design rests on such a fact, it is stated once as an assumption in the chapter for assumptions and used by name elsewhere.

Each claim carries its source. Each recurring fact carries its owner from the ownership table, and a claim in a non-owning section is written as a use of the fact, never a restatement. This is the one mechanism that removes repetition at its cause, and it only works if the table is consulted before writing, not after.

## Exemplars Over Rules

A page of style rules does less than three paragraphs of approved prose in context when the draft is written. The model imitates the register in front of it far more reliably than it obeys abstract instructions. The rules file existed throughout the project; the tightening happened only when each section was held against concrete examples and a concrete list of banned shapes.

So the writer stage loads real passages of the same section type that have passed review, verbatim, labelled by section type, with a short note on what they have in common. Not a template with placeholders. When a better passage passes review, it replaces one; nobody edits an exemplar in place.

## The Fact-Ownership Table

One row per recurring fact: the fact, the section that owns it, and, during a cleanup, how many sections currently mention it. The counts are the size of the problem, not a list of edits. On the concept this was built for, the worst four facts were being explained in 23, 20, 19 and 18 sections respectively.

The owner states the fact once, in full. Everyone else names it and depends on it. When a restatement is removed, the owner must already say everything the restatement said, at the same scope; where it does not, the owner is extended first and the restatement removed second. Deleting the fuller statement in favour of the thinner one is how facts get lost.

## The Editor's Brief

The following is the brief handed to the editor stage, verbatim except that project names are generalised. It is designed to be the only instruction the editor has.

> You are editing a draft of one section of a technical design document. You will not see the sources it was written from, and you must not need them. Your job is clarity, not truth: every fact in the draft is taken as true, and you may not change one, add one, or restate one that another section owns.
>
> Read the draft once as its reader would, with the heading covered. Then take each sentence through these questions, in order, and act on the first that fails.
>
> 1. Does it change what someone would build, configure, decide, or expect? If not, delete it.
> 2. Is this section the owner of the fact, per the ownership table? If not, reduce the sentence to a use of the fact by name.
> 3. Delete it and read the paragraph. If nothing is lost, it stays deleted.
> 4. Does it answer this section's question? If it explains why the other party's business works as it does, what the underlying product does beyond what the design needs, where a fact was learned, or what another chapter says, delete it or move it to the section whose question it answers.
> 5. Does it describe the other party's systems, or say that they accepted or confirmed something? Delete the description; an assumption may carry it. Delete the attribution; acceptance of the document is acceptance of its contents.
>
> Then the shapes. Delete a sentence that announces what follows. Split a sentence that carries more than one claim. Rewrite the "not X but Y" and "X rather than Y" closers as plain statements. Remove a justification clause that follows a design statement with why it is so. Remove a clause that names another chapter or an identifier inside a sentence. Remove a provenance clause. Replace a verb that names no relationship, such as carries, lives in, sits on, speaks, sees, drives, reaches, with the verb that does. One topic per paragraph. Complete sentences. No typeset characters. No hard wraps.
>
> A qualifier is accuracy, not verbosity. Scope words never come out.
>
> Where a sentence is unclear because it may be wrong, leave it as it is and flag it. Where two sentences in the draft contradict each other, leave both and flag them. You do not resolve facts.
>
> Return the edited section and, beneath it, one line per removal: the sentence, and which question or shape it failed. Then the flags.

Two design points in that brief matter more than the rest. There is no target length, because a target invites stopping while the prose is still unclear; the pass ends when every sentence has been through the questions. And the adjudication list is not optional output; it is the artefact the next stage checks against.

## The Mechanical Scans

Scripts do the half of verification that is mechanical, and they produce worklists, never verdicts. Their silence proves nothing. On the first real run, the shape scan found thirteen hits in a document that had just been through the full cleanup, most of them in text the agent had written that same day.

**Conservation.** For each sentence in the baseline, find the closest sentence anywhere in the edited document, and list the ones with no close match. Consolidation moves claims to their owner, so a claim that moved still matches and is not reported. Each reported sentence is adjudicated by hand as carried (it survives elsewhere, and where), dropped (removed on purpose, and which test it failed), or lost (an accident, restore it). A similarity threshold is a compromise: on the first section it correctly surfaced seven removals and missed two that scored just above the cut because a retained sentence nearby looked similar. So the worklist is the union of what the scan finds and what the editor recorded as removed; the two catch different mistakes.

**Shapes.** Regexes for the sentence shapes in the brief, the vague verbs, claims about the other party's systems, attributions of acceptance or confirmation, internal identifiers leaking into external text, hard-wrapped paragraphs, and typeset characters. Each has known false positives that are exempted by section rather than by weakening the pattern: an assumptions chapter may describe the environment, a scope list may cite requirement ids.

**Record and structure.** Whatever the document's own integrity checks are: unique identifiers in the decision record, no dangling references, balanced markup, a clean render, the required chapter structure unchanged.

## Apply and Report

Exact-match replacements with an assertion that the match count is one, so a phrase that occurs twice is caught before half of it is edited. A backup before the first change. A record entry stating what changed and its source. A report per section of what came out and under which test. Not how much came out: a number becomes a goal, and the goal is clarity.

## Multiple Levels

The stages divide by context, so a stage holding two jobs does the second badly, and a large section is better served by two editors than one: a structural editor applying the five questions and consolidating against the ownership table, then a line editor applying the shapes. Each returns its own adjudication. Adding a level costs a brief and a fresh context and nothing else.

## What the First Run Found

On a 27,000-word customer-facing document: about 1,600 words removed, and the facts that had been explained in up to 23 places each reduced to one owner and uses by name. Six accuracy defects surfaced by the process rather than by reading: two facts the cleanup itself would have lost, caught by conservation and the owner-first rule; two contradictions between sections that already existed; one plainly wrong sentence; two references to items that had been closed long before. Four problems in sentences the editor stage had created by merging, caught only by reading the result as prose, which is why Verify is not the last word.

## Generalising It Into a Harness

The unit is the context, not the agent. A harness needs to be able to start a stage with a fixed brief file and a named set of inputs, and nothing else.

Briefs are files, versioned with the rules they encode. The editor's brief above is one; the skeleton rules and the exemplar set are others.

Scans are gates that emit worklists, and a stage cannot proceed while its worklist is non-empty. The adjudication of each item is a required artefact, written by the stage that clears it.

The human has two entry points, and the harness should make direct writes to the document outside the pipeline impossible rather than discouraged, for example with a hook that refuses a write unless the adjudication artefact for that change exists.

## Implementing It as a Dinah Workbench

The stages above were first run by hand, one fresh context at a time, with the artefacts passed as files. On 2 September 2026 the pipeline was rebuilt as a board in dinah, so that the stages, the artefacts and the exits are recorded by a tool rather than by discipline. This section describes that build in enough detail to reproduce it.

### What Dinah Is

Dinah is a file-based workbench tool. A workbench is a directory tree under `.dinah/<container id>/` holding `workbench.md`, a `columns/` directory with one `column.md` per column, and a `cards/` directory with one `card.md` per card. Every file is Markdown with YAML front matter, so the whole board is readable and diffable without the tool. The version used here is dinah 0.1.0, conforming to profile dinah-core/0.9, storage format 2.

The parts of the model the pipeline depends on are these. A column has a `kind`, which is `intake`, `work` or `done`. An intake column takes no work; a card leaves it by `dinah pull <column>`, which moves the head of the queue and claims it in one act. A work column takes work, meaning an agent holds the card while it is there. A done column takes no work, and a held card cannot enter one, so a hold is released before the move. A workbench may have more than one done column, and that allows the two exits. Each column's instructions are the body of its `column.md`, and `dinah instructions <card>` serves the workbench body and the current column's body to whoever holds the card. Attachments hang on a card or on the workbench itself, with a description, a stable number and a payload file, and `dinah contents <card>` lists them. Comments are appended to a card and are not editable. `dinah check` verifies the structure.

A workbench is created with `dinah init <dir> --from <source> --slug <prefix> --operator <actor>`. The source is the container directory, `.dinah/<id>`, of a template workbench, and not the folder above it. Container ids are generated by the tool and a hand-made id is refused, so a template is made by letting `dinah init` create an empty workbench and then replacing its columns and workbench body.

The working agreement dinah states binds the agent rather than the tool. Claim a card before producing work on it. Do not hold a claim on a card you have stopped working. Treat the workbench as the authority for where a card stands. Do not move a card out of an operator-owned column unless you are the operator. The pipeline has no operator-owned column, which is deliberate and is discussed below.

### The Board

The template is one workbench with seven columns in order.

| Column | Kind | What happens there |
| --- | --- | --- |
| Intake | intake | A card arrives as a change to a named document whose claims are already sourced. A script checks it, attaches the document as it stands, records its hash, and pulls it into Write or moves it to Returned. |
| Write | work | A fresh context turns the claims into sentences in the exemplars' register, as OLD/NEW pairs against the document's current text, and attaches `draft.md`. |
| Edit | work | A fresh context holding only the editor's brief, the draft, the ownership table, the exemplars and the rubric deletes, consolidates and reshapes, and attaches `edited.md` and `adjudication.md`. |
| Verify | work | The conservation and shape scans run on the edited draft and their output is attached. Anything unadjudicated sends the card back to Edit. |
| Merge | work | A fresh context copies the verified OLD/NEW pairs into an exact-match patch. A script applies it with an asserted match count of one, backs the document up, runs the document's own checks and render, appends a record row, and moves the card to Done. |
| Done | done | The change is in the document. Nothing moves from here. |
| Returned | done | The change could not land. A task has been filed on the document's own board saying why, and the card is closed here with every artefact it produced. |

The workbench body states the rules that apply to every column. The board is a pipeline and not a place where work waits. Nothing after Intake may change a fact. Each stage is a fresh context. The artefact chain is the card's attachments in order. The loop between Write and Edit is bounded at three passes. The operator never appears on the board.

A rubric is attached to each workbench, and it is the only thing that varies by document type. It says where a document of that type sits, where its task board and ownership table are, which exemplar file the Write stage loads, which rules files the Edit stage adds to the brief, and which scan exemptions apply. One board serves every document of one type for one customer, so there is a concept board and a one-pager board per customer, and the rubric is the difference between them.

### Cards and the Artefact Chain

A card body is a skeleton and nothing more.

```
Document: <path to the Markdown>
Sections: <heading>; <heading>
Claims:
- <claim> [<source>]
- replace "<exact text>" with "<text>" [<source>]
```

The operator's request is itself a valid source, written `[operator: <the request>]`, and needs no other. A claim that would have to be researched has no source; it is not a card, it is a task on the document's board.

The attachments on a finished card are the record of the run. On the first card through the concept board they were, in order, the document as it stood at intake, the draft from Write, the edited draft, the adjudication of removals, the Verify output, the exact-match patch, and the backup taken before the patch was applied. Every stage reads the previous stage's attachment and writes its own. Nothing is passed in prose, and a stage that needs something it was not given is the signal that the card should be Returned.

### The Two Exits and the Coupling to Task Boards

A pipeline must not contain a column where a card waits for a person, because a waiting card is a queue and a queue is what the task board is for. So the pipeline has two exits and no review column. Done means the change is in the document. Returned means a stage needed a fact, a decision or the operator, and the script that handles it files a task on the document's own dinah board, in that board's intake, with the reason and a pointer back to the pipeline card with all its attachments. The task is worked there in the ordinary way, and when a complete skeleton exists a new pipeline card is filed. The old card is never reopened.

Approval, where a change needs one, happens on the task board before the card is cut, on the task that produced the skeleton. A request the operator makes directly has the operator's instruction as its source and sails through. Publishing is a separate act on the task board and never happens from the pipeline.

This coupling is the general pattern. A pipeline's Returned exit files onto the board that owns the work, so pipelines can be chained through task boards without any pipeline holding state.

### Scripts and Skills

Three scripts do the mechanical stages, and each is run from inside the pipeline workbench directory with a card reference.

`edit-intake.py` parses the card body, refuses a card with no document, no section or an unsourced claim and moves it to Returned, and otherwise attaches the document as `baseline.md`, comments the hash, and pulls the card into Write.

`edit-merge.py` reads the `patch.py` attachment, which defines a list of exact (old, new) string pairs, asserts that each old string occurs once and only once in the document, backs the document up into its review folder, applies the pairs, runs the document's record checks and its render, restores the backup on failure, appends a resolved row to the document's internal log naming the card and the backup, attaches the backup, releases the hold and moves the card to Done.

`edit-return.py` finds the `.dinah` beside the document, files the task there with the reason and the pointer, comments the pipeline card, releases the hold and moves it to Returned.

The scans from the earlier sections, `claim-diff.py` and `shape-scan.py`, are unchanged and run at Verify.

Two skills operate the board. The first turns a request made in conversation into a card. The working agent's answer to "add a line about X to section Y" is a card and not an edit, and the skill exists so that the agent that holds the conversation writes the skeleton and nothing else. It finds the board by document type, writes one line per claim with its source, files the card at Intake, runs the intake script, and hands over. The second skill turns the handle. It drains the board or runs one card, spawning a fresh subagent per work column with the inputs that column names and nothing from the running session, running the scripts, reading results, and stopping when every card is at Done or Returned. The working session moves cards and reads results; it writes no prose and decides no fact.

### What the First Run Through the Board Showed

The first card was a small correction to a data-mapping chapter, with two claims sourced to product documentation and one to the operator's instruction. It went from Intake to Done in about four minutes across four subagent contexts. The result was one changed table row and one record row, and the diff against the backup contained nothing else.

The editor stage split two sentences that each held two claims on a "so" clause, replaced a verb that named no relationship, and cut a phrase the row already stated, and it wrote all three down. Both scans came back clean on the first pass.

Two defects in the build surfaced on that run and were fixed the same afternoon. The merge script moved the card to Done while it was still held, and a done column refuses a held card, so the release now precedes the move. And `dinah show` on an attachment printed its front matter only, so a subagent had to be told how to find the payload file; dinah-334 has since made a read say where an attachment lives, so the workaround in the running skill can come out.

### Where the Human Is

The operator is present before the pipeline, on the task board, where the skeleton for a new section is approved and where research and decisions are settled. The operator is present after it, in the document, reading the diff the merge left on the card and deciding whether to publish. The operator is never inside it. That is the property that makes the editing stage useful, since the stage that removes what the operator complains about in the agent's prose is never given the reasoning that produced it, and it is also what lets the pipeline finish in minutes.

## Controlled One-Shots: Where Else the Shape Applies

The pipeline is a controlled one-shot. An agent goes off and does something, and the output is filtered, checked or edited by one or more further agents before it lands, with each of those agents starved of the context that produced the input. Any task with a judgement step that gets corrupted by its own context is a candidate, provided the facts can be frozen up front. Where no stage benefits from being blind, a plain skill is the right tool and a pipeline is ceremony.

### In the Same Shop

These are the candidates identified against the consultancy work the edit pipeline came from, in rough order of expected gain.

- Till test runs. Plan, with the expected outcome sourced; Run, producing screenshots, receipts and dumps only; Read, a blind stage that describes what the captures show without knowing the expectation; Compare; then a research row. The same context that wrote the expectation had been reading the screenshots, and it read what it expected.
- Concept and one-pager review. Extract claims one per line; check each against its source in a context that does not know why the claim was wanted; shape scan; file findings on the task board. An invented identifier survived several review passes in the old arrangement because the reviewer inherited the author's confidence.
- Customer answers and reviewer feedback. Transcript; extract decisions and facts one per line with the verbatim quote; attribution check; cards on the task board, and sourced changes straight into the edit pipeline. This is the fix for unquoted customer positions drifting into a document.
- Publishing. Render; an export-boundary diff against the last published version showing what the customer will actually see; publish. The stage deciding it is safe is never given the edit conversation.
- Research questions. Question; source hunt returning citations only; fact extraction from the citations; an existence check whose only job is to confirm every identifier named exists in the product's configuration or solution models.
- The agent's own reports to the operator. Raw report; an editor holding the brief to read it back as the operator would; out. This one needs no board.
- Ticket and status writeups. Draft; editor; field validation against the process configuration; post.

### In Other Work

- Translation. Translate; back-translate in a fresh context; diff the back-translation against the source for meaning drift; terminology check. The back-translator is never given the original.
- Contract and policy drafting. Clause skeleton from the term sheet; draft; a blind adversarial reader told only to find what the text lets the other side do; redline.
- Incident postmortems. Timeline from logs and chat, quotes only; narrative; a stage that checks every sentence of the narrative against a timeline entry; blameless-language edit; publish.
- Hiring. Structured extraction against the rubric with names, schools and dates stripped; blind scoring; reconciliation of two scorers; shortlist.
- Grant, tender and RFP responses. One line per requirement; a draft per requirement; a compliance stage that maps each requirement to the sentence answering it and returns the gaps; editor. Evaluators score this way.
- Scientific and technical review. Claims; for each, whether the cited source supports it, judged by a stage given only the citation; statistics sanity; reviewer summary.
- Financial and expense controls. Receipts; extraction; a policy check by a stage that is given the policy and the fields but not the claimant's justification; exceptions to a person.
- Code changes from an agent. Spec; implement; a reviewer given the diff and the spec but not the implementer's reasoning; a test author given the spec only, never the code; merge.
- Security and compliance evidence. Control statement; evidence collection; a stage that judges whether the evidence proves the control without the collector's notes; audit register.
- Clinical summaries. Chart; summary; a stage that checks each summary statement back to a chart entry; clinician sign-off.
- Journalism and communications. Notes; draft; a fact-check stage given only the draft and the source list; legal read; editor.
- Data reports. Query; result; a stage that re-derives the headline numbers independently from the raw data; narrative; editor. The narrator is never given the query.
- Teaching and assessment. Marking scheme; blind marking; a second blind marker on disagreements; moderation. For material, a "student" stage that has only the material reports what it could not follow.
- Support and escalation. Ticket; reproduce in a clean context from the customer's words alone; classify; draft reply; editor with the tone brief; send.
- Physical inspection. Field photos; blind description; comparison against the checklist; work order.
- Software localization. Catalog diff naming the changed source strings; a translator stage given each string with its context line and never the code; a verifier that checks placeholders, widths and markup survive; a staleness fingerprint tying each translation to the source it rendered.
- Data migrations. A plan from the schema diff; a generated migration; a stage that proves reversibility on a copy it made itself; an apply gate that refuses without the proof artefact.
- Release notes. Facts extracted from the landed diffs and their cards, one line per change with its commit; a draft; a verifier that maps every sentence back to a commit or card and returns the unmapped ones; an editor with the audience brief.
- Vulnerability triage. The advisory; an applicability check by a stage given only the dependency graph; an exploitability read given only the code paths named; a patch card or a recorded non-applicability, either way with the chain attached.
- Prompt and model regression. The candidate change; blind grading of its outputs against the rubric, with the grader never told which output is the candidate; a second grader on disagreements; promote or return with the gradings attached.

The common thread is one stage starved of context, and an artefact chain that makes the starvation auditable.

## What the Tool Still Owes This Shape

Two of the board's rules bind only the agent today, and each has a card.

The Write-to-Edit bound of three passes is prose in the workbench body, counted by nobody. dinah-364 gives a column a `loop_limit` declaration, a served count of the card's regressive departures, and a check finding at the limit, so the bound becomes something the tool can state and a harness can read rather than a sentence an agent must remember.

The Returned exit's pointer at the task it filed is prose too. Workbench identifiers are now universally unique, so a pointer can carry the owning workbench's identifier beside the card reference and survive the directory moving; the return script should write that form.

## Pitfalls Learned the Hard Way

- The shapes return in rewrites as readily as in original prose, under the same instructions that just removed them, so the scan runs after every pass rather than once at the end.
- Conservation is not truth: a merged sentence can be a new claim that neither source made, so somebody reads the result as a reader would.
- A qualifier is accuracy. "Of the types in scope", "on a return without receipt", and "for a digital coupon" never come out, however much tighter the sentence reads without them.
- Owner first: extend the owner, then remove the restatement, and never in the other order.
- A count is not a goal. The pass ends when every sentence has been through the tests, whatever the section then weighs.
