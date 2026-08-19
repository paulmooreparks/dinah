# dinah-164 UX sketch: the guide listing, and the shape of the new topic

Every block below was drawn through the real renderer rather than composed by
hand. The "today" blocks come from a build of `fb307b3` run in a scratch
workbench. The "after" blocks come from the same tree carrying a placeholder
`internal/guide/guides/first-session.md`, the declared reading order, and the
lead line, built and run the same way. The prototype was reverted and nothing
of it is proposed as an implementation.

## What `dinah guide` prints today

```
  Topic             Title
  ----------------  -----------------------------------
  getting-started   Getting started
  query             Asking questions of a workbench
  verbs             The five verbs
  workbench-layout  What a workbench looks like on disk
```

The rows stand in alphabetical order, because `guide.Topics()` sorts the
embedded filenames. A reader who has opened no guide yet has nothing here that
says which one answers the question they arrived with.

## What `dinah guide` prints after this card

```
The guides stand in the order Dinah recommends reading them.

  Topic             Title
  ----------------  -----------------------------------
  first-session     Your first session at a workbench
  getting-started   Getting started
  verbs             The five verbs
  query             Asking questions of a workbench
  workbench-layout  What a workbench looks like on disk
```

The widest line of that block is 55 columns, so the table does not stack at the
assumed window of 80.

## The unknown-topic refusal

Today, at `fb307b3`:

```
dinah.unknown-guide no guide is embedded under the topic nope; Dinah carries guides on these
  getting-started
  query
  verbs
  workbench-layout
run `dinah guide <topic>` with one of them
```

After this card, drawn from the same prototype:

```
dinah.unknown-guide no guide is embedded under the topic nope; Dinah carries guides on these
  first-session
  getting-started
  verbs
  query
  workbench-layout
run `dinah guide <topic>` with one of them
```

The refusal lists the topics through the same function, so the reading order
reaches it without a second mechanism.

## The outline of `first-session`

The guide runs in the order a session runs rather than by subject. Eight
sections, each answering a question the reader has at that moment.

| Heading | What the reader can do after reading it |
|---|---|
| Find out where you are | Run `dinah status` and read the workbench path, the actor, the flow, and what you already hold. |
| Find out whose name you are acting under | Run `dinah whoami` and `dinah config`, see which rung supplied the actor, and set your own before you write anything. |
| Read what this workbench asks of you | Run `dinah states` and `dinah instructions`, and know that standing prose arrives at two levels. |
| Take a card | Run `dinah next`, `dinah ls`, or `dinah query`, then `dinah claim`, and read the instructions the claim serves back. |
| Read the card before you work it | Run `dinah show` and `dinah log`, and know which references answer today. |
| Record what you did on the card | Run `dinah comment` and `dinah attach`, so the next reader learns it from the workbench rather than from you. |
| Move the card, then give it back | Run `dinah move`, then `dinah release`, every time, and `dinah block` when the work cannot go on. |
| When Dinah refuses | Read the refusal name, the exit code, and what each of the four outcomes asks you to do next. |

The guide carries no fenced block opening with `$ `, so it adds no transcript
that the quick start's replay guard does not drive.
