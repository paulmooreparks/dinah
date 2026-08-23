# UX sketch: the principles guide

This sketch shows what a reader meets when the `principles` guide ships. It
draws the listing, the guide's section outline, and one section drafted in
full so the operator can rule on the register before the whole guide exists.

Every transcript below was run against the binary built from commit
`2425ac9` in a scratch tree, with `DINAH_HOME` pointed at that tree.

## The listing today

```
$ dinah guide
  Topic             Title
  ----------------  -----------------------------------
  getting-started   Getting started
  query             Asking questions of a workbench
  references        References
  verbs             The five verbs
  workbench-layout  What a workbench looks like on disk
```

## The listing after this card

`guide.Topics()` sorts the embedded filenames, so `principles` lands between
`getting-started` and `query`, and the widest title grows the second column by
four characters.

```
$ dinah guide
  Topic             Title
  ----------------  ---------------------------------------
  getting-started   Getting started
  principles        Why a workbench has the rules it has
  query             Asking questions of a workbench
  references        References
  verbs             The five verbs
  workbench-layout  What a workbench looks like on disk
```

If dinah-164 merges first and replaces the sort with a declared reading order,
`principles` takes the place after `verbs` instead, and the listing above
changes order without changing a word of the row.

## The section outline

Each section carries the same four parts in the same order, and the order is
what lets two readers stop in different places.

1. The principle, in one sentence, which both readers need.
2. What Dinah does about it, which both readers need.
3. What it costs you to set it aside, written for whoever runs the workbench.
4. The statements of the contract that bind a tool, on one line at the end,
   written for whoever is building a second tool or driving this one.

The sections themselves:

| Section | What it maps | The mechanism |
|---|---|---|
| Where these rules come from | The Lean lineage | none, this is the one section with no mechanism |
| You take work, and nobody hands it to you | Pull | `dinah claim` |
| A state says how much work it will hold | Capacity | `wip_limit` and `--override` |
| The instructions reach you where they apply | Explicit policy | serving at claim and at move |
| An obstacle is raised where everybody sees it | Visible impediment | `dinah block` and `dinah unblock` |
| The record is what improvement works against | Kaizen | the journal and `dinah log` |
| What binds you rather than the tool | The working agreement | nothing enforces these |

## One section, drafted in full

The capacity section is drafted here rather than any other because it is the
one an operator acts on directly, and because its four parts pull hardest in
different directions.

> ## A state says how much work it will hold
>
> A station that accepts every card offered to it stops being a station and
> becomes a pile. Work in progress is inventory: it costs you the time it sat
> there, it hides the problem that stalled it, and none of it is finished. A
> limit on how much a state holds turns that cost into a refusal you meet on
> the day the queue grows rather than a discovery you make a month later.
>
> A state declares its limit as `wip_limit` in `states/<id>/state.md`. Dinah
> refuses a move into a state that has reached its limit, and `dinah states`
> shows the count against the limit so you can see it coming:
>
> ```
>   Slug    Name    Kind    Cards  Owner
>   ------  ------  ------  -----  -----
>   intake  Intake  intake  1      agent
>   doing   Doing   work    2/2    agent
>   done    Done    done    0      agent
> ```
>
> A blocked card counts. It is still sitting in the state, still taking up the
> place, and still not finished, so exempting it would turn blocking into a way
> of hiding an overloaded station from the person who has to fix it.
>
> The operator may carry a card through a full state with `dinah move <card>
> <state> --override`, and Dinah records the move as an override. That is the
> whole of the exception. Dinah refuses the same request from anybody else, and
> it refuses it from the operator too when the marker is absent. A limit
> nobody can ever set aside gets worked around outside the tool, where nothing
> sees it, so Dinah gives you a way through and writes down that you took it.
>
> If you are deciding whether to set a limit at all, set one where cards
> arrive faster than they leave. That is where the pile forms, and the limit is
> what tells you about it.
>
> The contract: CORE-MOVE-4, CORE-MOVE-5, CORE-MOVE-9, CORE-MOVE-10, and
> CORE-MOVE-11.

## What the drafted section shows about the register

Three things worth ruling on before the other six sections are written.

The reader is `you` throughout, and Dinah is the only other subject. The
transcript is real output from a workbench built for this sketch, and it is
here because a count against a limit is easier to see than to describe.

The closing line of the section is a recommendation to an operator and it is
the only sentence a reading agent can skip. Putting it last is what makes it
skippable.

The contract line names statement identifiers and nothing else. An operator
who has never opened the profile loses nothing by stopping one line earlier,
and an implementer building a second tool has the reference it needs without
the guide restating the statements in its own words, which would give the
contract a second wording that can drift from the first.
