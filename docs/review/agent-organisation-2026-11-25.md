# How the agent organisation worked on 25 November 2026

This review looks at one working day of the agent organisation that Paul runs in Dinah: what it cost, where the cost bought nothing, and what worked well enough to keep. It was written by the Software Director chat at Paul's request, the night of that day, from the bus database, from that chat's own transcript and from the messages it handled. The design and plan that follow from it are in `docs/design/agent-organisation-economy.md`.

The organisation on that day was Paul, one director chat that reports to him, five team leads that report to the director, each lead's own team, and an integrator chat started that evening to merge, push and install. The leads ran on two vendors' models. Four products were in active work, among them Dinah, a private fork of it, PUDL and yavc.app.

## Summary

The structure did its main job. Paul dealt with one chat for decisions, the leads settled most cross-team work among themselves, and a merge step that nobody inside Dinah could see or wake was brought inside.

The cost was mostly paid in two places that the structure itself created. The first is Paul's attention: about a third of the director's turns in the evening ended with a line telling him that nothing needed an answer. The second is tokens: a chat that is woken reads its whole conversation again, so the cost of a message is set by the size of the chat that receives it and hardly at all by the size of the message. The director chat read 24.8 million cached input tokens that day in order to write 93 thousand tokens of output.

Dinah already has several of the parts a remedy needs: informational notices that wait for a chat's next turn, a usage table, turn allowances for a lead, and a way for Paul to speak to a member with a copy to its lead. Most of them were unused or incomplete on the day.

## What was measured

The figures in Table 1 come from the bus database, opened read-only, for the day of 25 November 2026 in Paul's time zone.

**Table 1** *Bus Traffic on 25 November 2026*

| Measure | Value |
|---|---|
| Messages on the bus, all workspaces | 352 |
| Characters in those messages | 461,326 |
| Messages in the team leads' group | 84 |
| Characters in the team leads' group | 111,815 |
| Deliveries made for the team leads' group messages | 486 |
| Posts in that group by the director | 29 |
| Posts in that group by the Dinah lead | 28 |
| Posts in that group by Paul | 9 |
| Posts in that group by the other four leads together | 18 |

*Note.* A delivery is one message reaching one recipient. A group message is delivered to every member, so 84 messages became 486 deliveries. A delivery to a member who is not addressed arrives as a one-line notice on that member's next turn and starts no turn of its own.

The director's own cost is in Table 2. It comes from the usage figures that the model provider returns with each call, as recorded in the chat's transcript.

**Table 2** *Token Use of the Director Chat on 25 November 2026*

| Measure | Value |
|---|---|
| Model calls | 153 |
| Cached input tokens read | 24,827,132 |
| Input tokens written to the cache | 559,798 |
| Uncached input tokens | 441 |
| Output tokens | 92,701 |
| Size of the conversation at the end of the day | about 247,000 tokens |
| Tokens read by one turn that had nothing to do, at that size | about 230,000 |
| Tokens that such a turn added to the conversation | about 2,500 |
| Tokens that such a turn wrote as its answer | about 60 |

*Note.* The conversation was compacted once during the day, so its size at the end understates what an uncompacted day would reach. Cached reads are billed at a tenth of the normal input price under the provider's published API pricing. How a subscription plan weighs cached reads against its limits is not something this review has documentation for.

Table 3 gives what Dinah's own usage table recorded for the same day.

**Table 3** *Usage Recorded by Dinah on 25 November 2026*

| Group of chats | Chats | Recorded tokens | Recorded as known |
|---|---|---|---|
| Chats on the second vendor's models | 9 | about 238 million | Yes |
| Chats on Claude | 7 | 0 | No |
| Chats on Devin (unit: ACU) | none that day | none | No |

*Note.* The largest single figure was 46 million tokens for one builder chat over three turns. The review has not established whether the second vendor's figures count cached input in full, so they should not be compared directly with Table 2. No allowance was set for any lead that day.

Three things stand out from Table 3. Dinah records usage for one vendor only, so for most of the organisation, the director and the integrator included, the product cannot say what a day cost. The nine measured chats alone used about 238 million tokens. No allowance was in force, so nothing would have stopped a branch of the tree from spending without limit.

The instruction block that Dinah attaches to every turn was also measured. It was attached to 75 of the director's prompts that day and averaged about 6,000 characters, which is roughly 1,500 tokens each time. Its text is the same on every turn apart from one private value. Each copy stays in the conversation and is read again on every later call, so the block made up more than half of the 2,500 tokens that a turn with nothing to do added.

## Where the cost bought nothing

Table 4 lists the overhead seen in the evening session, most frequent first.

**Table 4** *Organisational Overhead Seen in the Evening Session*

| Overhead | What happened | Cause | Who paid |
|---|---|---|---|
| Turns with nothing to do | About ten times a lead's progress or completion report woke the director, which then wrote Paul a line saying no answer was needed | Leads addressed routine reports to the director; the window shows Paul every turn | Paul's attention, and about 230,000 cached tokens each time |
| Pure relaying | The director carried Paul's sentences to leads four or five times, adding only a statement of scope | Paul speaks to the director, and the work sits with the leads | A turn of the director and a turn of each lead |
| One decision recorded twice | Two leads recorded Paul's short answer, then recorded it again when his full wording arrived | The short form travelled ahead of the full one | Two leads, two rounds each |
| Statements made from memory | The director asked for branches to be merged that were already merged, and recommended a change to the team tree that was already in place | The director did not check the repository or the tree first | Extra messages, and Paul's time on a wrong recommendation |
| Work started, then stopped | A mockup was built and checked, then set aside minutes later | The director told the lead to start before asking Paul whether he wanted the work now | The lead's tokens |
| Small changes sent singly | A guide was corrected in three separate documents-only merges, each with the full Go test run | Each correction was handed over as it was found | The integrator's tokens |
| Double delivery | The director asked a lead directly about a change after the group post had already reached that lead | The director assumed an old delivery hold was still in place | One wasted question and answer |

*Note.* The counts are the director's estimate from its own session, and the bus log has not been tallied to confirm them. The table leaves out work that looked like overhead and earned its cost, which the next section covers.

The first row is the largest by count, and it is the one the structure makes inevitable as built. A report addressed to the director starts a turn. The turn reads the whole conversation. The bus instructions then require the chat to end with one short line, because one agent reports a turn with no words as an error (`docs/design/group-attention.md`), and the window shows that line to Paul like any other answer. Every step is working as designed, and the sum is a quarter of a million tokens and one interruption to learn that nothing happened.

## What worked and should be kept

Table 5 lists the practices that earned their cost that day. Each one is a candidate for being made the normal way of working, and some for being built into the product.

**Table 5** *Practices That Worked*

| Practice | What it achieved that day |
|---|---|
| A handoff between teams with explicit acceptance | The PUDL lead handed over a release with its limits stated; three leads accepted ownership within a minute and divided the work without the director |
| An independent check by a different chat | A separate verification chat refused a release over a submenu that opened on top of its parent in right-to-left layouts, after all 772 automated checks had passed |
| A written procedure as the authority | The integrator works from a guide in the repository, rereads it before every merge, and refuses a request from any lead to skip a step |
| Testing a recovery step before writing it down | Running the rollback on scratch folders found that the written steps could not build where they said to, and replaced an unsupported method with the documented one |
| A record that can be read with the product closed | Every install is written to a plain file outside the bus, so the last good build can be found when Dinah will not start |
| Paul's words quoted with a stated scope | Each permission the director passed on carried his sentence and what it did and did not cover, so no lead read it more widely |
| Receipts that say what did not happen | A send receipt said when a message was held and when a chat could not be woken, which is how a delivery defect of several days was found |
| Models from two vendors | A reviewer on a different model does not share the author's blind spots |
| A first run with nothing at stake | The integrator's first merge was documents only, so it tested the path without changing a program |
| Handover notes kept current | Two leads refreshed their handover documents the same day, so a later session can take over without the conversation |

*Note.* The fourth and fifth rows came out of the director's review of the integrator's guide. That review took three rounds and is counted here as work that paid for itself.

## What this says about the structure

An agent organisation has the same shape as a human one, and its costs fall in different places. Starting a chat costs almost nothing. Every message costs the reader its whole conversation, and every message Paul sees costs him attention. A human organisation has the opposite problem, with expensive people and cheap talk.

Three consequences follow, and the design takes them as its starting points.

1. The scarce things are Paul's attention and tokens. A structure should be judged by how little of each it spends for a given result.
2. A long conversation is a liability as well as a memory. The longer a chat has worked, the more each new message costs it, so a role should live in its written records and be able to move to a fresh chat.
3. A step that is the same every time belongs in a script. A model should be spent on judgement.

## Limits of this review

The evidence is one day, seen from one chat. The overhead counts are estimates. The token figures for the director are exact, and those for other chats are what Dinah recorded, with the gaps Table 3 shows. The review did not read the transcripts of other chats, so it cannot say how much of each lead's spend was overhead of the same kinds. The first step of the plan is a week of measurement so that later steps are chosen on better numbers than these.
