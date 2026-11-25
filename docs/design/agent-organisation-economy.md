# Spending less attention and fewer tokens on the agent organisation

This document proposes changes to how chats in a team report, hand over and are woken, so that the same work costs Paul fewer interruptions and costs every chat fewer tokens. It follows from the review of one working day in `docs/review/agent-organisation-2026-11-25.md`, which has the measurements quoted here. It was written by the Software Director chat at Paul's request on 26 November 2026. Nothing in it is built. Paul accepted its decisions on 26 November 2026, as the last table records. He has said the work comes after the adoption of PUDL 0.109.0 is installed.

The document has four parts: what the design is for, the changes themselves, how the practices that already work are made permanent, and a plan in stages with the decisions Paul has to make.

## What the design is for

The review found that an agent organisation spends two scarce things, Paul's attention and tokens, and that both are spent mostly by waking chats. A woken chat reads its whole conversation again, and the window shows Paul what it wrote. The design therefore has one aim stated two ways: wake a chat only when it has something to do, and make each waking cheaper.

Table 1 gives the measures the work will be judged by, with their values on 25 November 2026.

**Table 1** *Measures and Their Starting Values*

| Measure | Value on 25 November | Aim |
|---|---|---|
| Director turns in an evening that ended with nothing to do | about 10 | 2 or fewer |
| Cached tokens the director read for each token it wrote | about 268 | under 100 |
| Tokens a turn with nothing to do added to a conversation | about 2,500 | under 1,000 |
| Chats whose usage Dinah recorded as known | 9 of 16 | every chat whose provider documents a way to report it |
| Leads with an allowance set | 0 | every lead |
| Merges that ran the full Go tests for a documents-only change | 3 of 3 | 0 |

*Note.* The aims are proposals. The first stage of the plan measures a full week before any of them is treated as a target.

## What Dinah already has

Several parts of a remedy exist already and were unused or incomplete on the day (Table 2). The design builds on them and adds as little as it can.

**Table 2** *Existing Parts the Design Builds On*

| Part | Where it is described | State on 25 November |
|---|---|---|
| A group post reaches a member who is not addressed as a one-line notice on that member's next turn | `docs/design/group-attention.md` | Working; this is the delivery that costs nothing |
| A turn allowance for a lead, counted up the chain | `docs/design/team-hierarchy.md`, `internal/bus/teams_budget.go` | Built; no allowance was set |
| A usage record for each turn, in the provider's own unit | `internal/bus/teams_budget.go` | Recorded for one vendor; Claude and Devin turns were recorded as unknown |
| Paul speaking to a member directly, with one copy of the exchange to its lead | `docs/design/team-hierarchy.md` | Built; Paul spoke through the director all day |
| A new team member started on a chosen model | `dinah.team/start@2` | Used once, for the integrator |
| A send receipt that says whom a post woke | `docs/design/group-attention.md` | Working |

## The changes

### 1. A message that informs without waking

Today a sender has two choices in a group. It can address a member, which starts a turn of that member, or address nobody, which wakes whoever was active lately or the group's lead. A sender that wants a member to know something, and wants nothing done, has no way to say so. That gap produced most of the director's empty turns.

The proposal is a second kind of recipient on a send and a reply. A member given as an inform recipient receives the whole message at its next turn, whatever starts that turn, and is never woken by it. Several such messages arrive together. The receipt says who was informed and that nobody was woken.

Three existing deliveries become this kind by default:

- A completion report from Dinah for an operation whose result the chat does not need in order to continue.
- The copy that a lead receives when Paul speaks to one of its members directly.
- A lead's summary of a member's report for the rest of a group.

Paul ruled on 10 November 2026 that a chat woken by a group post decides for itself whether the post concerns it. This proposal leaves that rule in place for posts that address nobody. It adds a way for the sender to say that no decision is being asked for, which the measurements show is common and expensive. Because it changes what the ruling covers, it is listed among the decisions at the end.

### 2. Turns that do not interrupt Paul

The bus tells a chat with nothing to do to end its turn with one short line, because one agent reports a turn with no words as an error, and that error is not documented well enough for the window to read (`docs/design/group-attention.md`). The line has to stay. What can change is how the window treats it.

The proposal is that a turn started by the bus, in which the chat neither addressed Paul nor asked him for a decision, raises nothing for Paul: no unread mark, no toast, no line in a list of chats waiting for him. Its text stays in the transcript, drawn as activity. Paul already has a way to be told that a turn matters, which is the chat addressing him or asking through Decisions, and the bus knows when either happened, so the window needs no guess about what the text says.

This review has not established what the window raises for such a turn today, so the first step is to find out. Any change to what Paul sees needs his approved mockup first.

### 3. A shorter instruction block on each turn

Dinah attaches a block of instructions to every turn. On 25 November it was about 1,500 tokens, and its text was the same each time apart from one private value. Seventy-five copies went into one conversation that day, and every copy is read again by every later call until the conversation is compacted.

The proposal is to send the block in full on a chat's first turn and whenever its text changes, and otherwise to send only the private value and one line saying the earlier instructions still apply. The block must be sent in full again after a conversation is compacted, since compaction may drop it. That needs a documented signal from each agent program that compaction happened. Where a program documents one, Dinah uses it. Where a program documents none, Dinah keeps sending the full block to chats on that program, and the design note says so. Guessing at compaction from the size of a transcript is not acceptable.

### 4. Handovers and decisions as records the product knows

Two kinds of message did most of the useful work on the day, and both were written by hand each time in a form that the leads had to be told.

A handover has a fixed content: the branch and its head, what changed, the tests run with their results, whether new interface has Paul's approved mockup, and what is asked of the receiver. The receiver accepts or refuses, and ownership moves on acceptance. A decision from Paul has his words, quoted, and a statement of what they cover and do not cover.

The proposal is a tool for each. A handover tool takes those fields and refuses a handover that lacks one. A decision record takes Paul's words and the scope, and is what a lead or the integrator quotes afterwards. Both are stored by the bus, so a chat that has lost its conversation can ask the product what is open and what Paul said. A long report then becomes a short message with a link to a file, where today the whole table travels in the message and into every reader's conversation.

### 5. The merge procedure as a script

The integrator follows a written guide of some eleven steps, and almost all of them are commands whose result is pass or fail. A model reads each result today. On 25 November it ran the full Go tests, 73 packages, three times for changes that touched only documents.

Three changes are proposed, in this order.

1. Checks in proportion to what changed. A merge whose every changed file is in a documents folder runs the forbidden-terms guard and the commit scan, which protect a public repository, and skips the Go tests and the browser suites. The guide already has this rule for installs.
2. One script that runs the guide's steps and stops at the first failure, with each step's result written to a file. The integrator runs it, reads the result, and writes the report. The guide stays the authority, and the script is the guide's steps in a form that cannot be skipped by a tired or compacted chat.
3. A cheaper model for the integrator once the script exists, since what is left for the model is reading one result and deciding whether to stop.

Leads batch small corrections to one document into one handover, which is a working rule and needs no code.

### 6. Roles that outlive their chats

The cost of waking a chat grows with its conversation. A lead that has worked for a week pays several times more for each message than the same lead on its first day, and a message that arrives after the provider's cache has expired costs more again, because the whole conversation is written to the cache afresh at a premium.

A role already lives mostly in its written records: its remit, its handover notes, the guides it follows. The proposal is to make moving a role to a fresh chat an ordinary, cheap act. A lead writes its handover notes, Dinah starts a new chat with the same remit, team and group memberships, and the old chat is retired with its history kept. The bus address of the role does not change, so nobody else has to learn a new name. Dinah suggests the move when a chat's conversation passes a size Paul sets, and never does it unasked.

The same reasoning applies inside a team. Reading many files in order to answer one question should be given to a short-lived member whose conversation is thrown away, so that the lead keeps the conclusion and not the files.

### 7. Measuring spend and limiting it

Dinah recorded usage for nine of sixteen chats on the day, and no lead had an allowance. Four things are proposed.

- Record usage for every provider that documents a way to report it for a turn. The figures in the review for the director chat came from its transcript file, whose layout is not a documented contract, so the product must not read it. The Dinah lead is to find the documented source for each provider and say plainly where there is none.
- Record the kinds of token separately where the provider reports them: fresh input, cached input, and output. A single total hides the fact that most of the spend is rereading.
- Show the cost of a waking where it is caused. A send receipt can say roughly how large the conversation is that it is about to wake, so a sender sees that a one-line report to a long-lived chat is an expensive message.
- Set an allowance for each lead, using the allowances that are already built, and show Paul each team's use against it.

Any new view needs Paul's approved mockup.

### 8. Working rules that need no code

Table 3 lists the rules that take effect by instruction. The first two were sent to the leads on 25 November.

**Table 3** *Working Rules*

| Rule | Who follows it |
|---|---|
| Address the director only for a decision, a blocker, a finished result or a change to a shared rule | Leads |
| Address every group post to the member it is for | Everyone |
| Tell Paul the size of a request and ask whether he wants it now, before any lead starts work that is larger than a small fix | Director |
| Check the repository, the team tree or the bus before stating their condition to Paul or to another chat | Director |
| Send Paul one digest at a natural stopping point, and a line at once only for a decision or a problem | Director |
| Batch small corrections to one document into one handover | Leads |
| Put a long report in a file and send its path with three lines of summary | Everyone |
| Give bulky reading to a short-lived member and keep only the conclusion | Leads |
| When Paul's instruction is for one lead and needs no interpretation, Paul may give it to that lead directly; the director receives the copy | Paul |

## Ideas for token spend

Table 4 gathers every idea in this document that reduces tokens, with a judgement of how much each might save and how sure that judgement is. The judgements are the author's, from one day of figures.

**Table 4** *Ideas for Reducing Token Spend*

| Idea | Section | Expected saving | Confidence |
|---|---|---|---|
| Inform without waking | 1 | Large: each avoided turn of a long-lived chat is its whole conversation | High |
| Move a role to a fresh chat | 6 | Large for old chats: cost for each waking falls with the conversation's size | High on the arithmetic, medium on how well a role survives the move |
| Script the merge procedure | 5 | Large for the integrator | High |
| Checks in proportion to the change | 5 | Medium: three full test runs avoided on the day | High |
| Shorter instruction block | 3 | Medium: about 1,500 tokens a turn, reread by every later call | High where compaction can be detected |
| Cheaper model for mechanical roles | 5 | Medium | Medium; needs a trial |
| Reports as a link with a summary | 4, 8 | Medium | Medium |
| Short-lived members for bulky reading | 6, 8 | Medium | Medium |
| Ask before starting sizeable work | 8 | Varies: one mockup on the day | High |
| Allowances for each lead | 7 | None directly; it stops a runaway branch | High |
| Smaller tool results | below | Small | High |

*Note.* "Large" means a saving that would show in a day's total for the chat concerned. None of these has been measured.

Two small defects seen on the day belong with the last row. The completion report for a question asked of another chat contained that chat's answer twice. Listing an applet's actions returned the same large schema once for each action that shares it. Both add tokens to the asking chat's conversation for good.

### Providers billed in compute units

Paul also works with a provider that bills in compute units and gives him a limited supply. The author knows less about that provider's accounting than about tokens, so this part is a set of hypotheses to test, and each should be checked against the provider's own documentation before it is relied on.

- A session is billed for the work it does, exploration included. Planning, reading and specification done first in a chat on a plan with spare capacity, and handed over as a small task with the exact files and the checks that define done, should cost fewer units than asking the metered agent to find its own way.
- One task for each session, ended when the task is done, should cost less than a long session that accumulates context.
- Questions and discussion belong in a chat that is not metered that way.
- Checks that a script can run should be run locally and not by the metered agent.
- Dinah has a usage unit for this provider and recorded nothing for it. Recording it, where the provider documents a way, would let a lead's allowance be set in units.

The handover tool in section 4 fits the first of these directly: a handover with its fields complete is the small, exact task that a metered agent should be given.

## Making the practices that work permanent

Table 5 takes each practice the review found worth keeping and says how it becomes the normal way of working.

**Table 5** *How Each Working Practice Is Reinforced*

| Practice | Reinforced by |
|---|---|
| Handoff with explicit acceptance | The handover tool of section 4 |
| Paul's words quoted with a stated scope | The decision record of section 4 |
| A written procedure as the authority, reread before use | The same pattern for every role with a repeated duty; each lead's remit points to its guide |
| An independent check by a different chat | A rule that the chat which writes a change does not accept it, stated in each team's guide |
| Models from two vendors | Choosing a reviewer on a different vendor from the author where a team has both |
| Testing a recovery step before writing it down | A line in the guide template: a recovery section says which steps were run and when |
| A record readable with the product closed | The install record, kept; the same test applied to any new recovery step |
| Receipts that say what did not happen | Kept, and extended with whom a message informed and how large a chat it woke |
| A first run with nothing at stake | A line in the guide template for any new role that can change shared state |
| Handover notes kept current | The move of a role to a fresh chat in section 6, which makes the notes necessary and so keeps them honest |

## Plan

The work is in five stages (Table 6). Each stage is useful on its own, and each later stage is chosen on what the earlier ones measured.

**Table 6** *Stages of the Work*

| Stage | Content | Owner | Needs from Paul |
|---|---|---|---|
| 0. Rules | The working rules of Table 3 | Director | Nothing; two are in force |
| 1. Measure | Usage for every provider with a documented source, split by kind of token; one week of figures against Table 1 | Dinah lead | Nothing until a view is proposed |
| 2. Wake less | Inform recipients; the shorter instruction block; the two small defects; what the window raises for a turn Paul was not addressed in | Dinah lead | The decision on inform recipients; a mockup for any change to what he sees |
| 3. Cheaper merges | Checks in proportion to the change; the merge script; a trial of a cheaper model for the integrator | Dinah lead writes, integrator runs | Agreement that documents-only merges skip the Go tests |
| 4. Records and roles | The handover tool and decision record; moving a role to a fresh chat; allowances and a view of use against them | Dinah lead | Mockups; a size at which Dinah suggests a move; the allowances |

*Note.* Stage 0 has no product code and has begun. Stages 1 to 4 start after the PUDL 0.109.0 adoption is installed, as Paul asked. The integrator moving from the Dinah lead's team to the director's, which Paul agreed to and deferred, fits at the start of stage 3.

Stage 2 is placed before stage 3 because it reduces both of the scarce things, and stage 3 reduces only tokens.

## Decisions for Paul

Table 7 lists the decisions this design needed from Paul. He accepted every recommendation on 26 November 2026, in these words: "I accept all of your recommendations for my decisions in table 7." The table records each decision as it now stands.

**Table 7** *Decisions Made on 26 November 2026*

| Question | Decision |
|---|---|
| Whether a sender may mark a recipient as informed and not woken, beside the 10 November ruling | Yes. The ruling stays for posts that address nobody |
| Whether a turn that did not address him should raise nothing in the window | Yes, after he approves a mockup |
| Whether documents-only merges skip the Go tests and browser suites | Yes. The guard and the commit scan still run |
| Whether the integrator moves to a cheaper model once the script exists | It is tried for a week and compared |
| Whether Dinah may suggest moving a role to a fresh chat, and at what size | Yes. The size is chosen after the week of measurement |
| Whether each lead gets an allowance | Yes, set after the week of measurement |
| Whether he gives simple instructions to a lead directly, with the copy to the director | Left to his preference from case to case |

*Note.* Acceptance of these decisions approves the direction and the order of the plan. It approves no change to what Paul sees: each such change still needs his approved mockup. The decision on documents-only merges takes effect when it is written into the merge guides, and not before.

## Risks and open questions

- A message that informs without waking can sit unread for a long time in a chat that nobody wakes. The receipt has to say so, and an inform recipient should be shown the age of what it is given.
- A fresh chat in an old role knows only what was written down. The first moves should be of roles with good handover notes, and each should be reviewed for what was lost.
- A script hides its steps from the model that runs it. The result file must give every step and its output, so a failure can still be reasoned about.
- The saving from a shorter instruction block depends on a documented compaction signal from each agent program. Where none exists the saving is not available for that program.
- The review rests on one day and one chat. The week of measurement in stage 1 may change the order of the later stages, and it should be allowed to.
