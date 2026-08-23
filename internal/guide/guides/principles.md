# Why a workbench has the rules it has

## Where these rules come from

A workbench is a Kanban system. Kanban grew up on the production floor at Toyota, where a station ready for more work signalled the station upstream instead of taking whatever was pushed at it. You are running that idea over work nobody can watch moving: a card held by somebody three timezones away, or by an agent that will forget the whole afternoon when its context ends.

You will meet a few Kanban words in this guide, and Dinah has its own word for each of them. A column of a board is a state. Work in progress is the cards standing in a state. A pull is a claim. Each of those is mapped here, once, and the rest of this guide uses the Dinah word.

Dinah spells the contract's verbs `claim`, `move`, `release`, `block`, and `unblock`. Read `dinah guide verbs` if you want to know what each one does. This guide answers a different question: what the workbench protects by having the rule, and what you give up when you set it aside.

## You take work, and nobody hands it to you

A piece of work begins when whoever is going to do it decides that it begins.

You take a card with `dinah claim <card>`, and Dinah offers no way at all to write a claim in another owner's name. The command reads the card and an optional `--expires`, and no surface of the tool accepts the name of a holder other than you. So a card you hold is a card you decided to take, and you can read the board knowing that of every other card on it.

A claim you have forgotten about is a claim the queue is wrong about, so you may say up front how long you expect to be. Dinah returns the card to the queue when that time runs out and writes the lapse down. The queue corrects itself, and nobody has to reach in and take your card off you to make it happen.

Once somebody starts handing out the work, you have a queue with a foreman. You can no longer read the board to find where work piles up, because what you are reading is one person's picture of who ought to be busy. An agent handed a card has no reason to ask whether that card was the one worth doing, and it will not ask.

The contract: CORE-CLAIM-6 and CORE-CLAIM-7.

## A state says how much work it will hold

If you let a state accept every card offered to it, you stop having a station and start having a pile. Cards standing in a state are inventory. None of them are finished, and the time they spend waiting is time you have already paid for. A card that sits does not announce what stopped it either, so whatever stalled it stays out of sight for as long as you tolerate the pile. Setting a limit on how much a state holds means you meet that cost on the day the queue grows, as a refusal, instead of working it out a month later.

You set a state's limit by writing `wip_limit` into `states/<id>/state.md`. Dinah then refuses a move into that state once it is full, and `dinah states` shows you the count against the limit while you still have room:

```
  Slug    Name    Kind    Cards  Owner
  ------  ------  ------  -----  -----
  intake  Intake  intake  1      agent
  doing   Doing   work    2/2    agent
  done    Done    done    0      agent
```

Dinah counts a blocked card against the limit. The card has not left the state, and blocking it did not free the place it occupies. If Dinah exempted it, you could hide an overloaded station by blocking whatever was stuck in it, which keeps the trouble from the person who has to fix it.

You, as the operator, may carry a card through a full state with `dinah move <card> <state> --override`, and Dinah records the move as an override. Nobody else may do it, and Dinah refuses it from you too when you leave the marker off. That single exception exists because a limit nobody can ever set aside gets worked around outside the tool, where nothing sees it. Dinah would rather give you a way through and write down that you took it.

If you are wondering where to set a limit at all, set one wherever cards arrive faster than they leave, because that is where a pile forms and a limit is what will tell you about it.

The contract: CORE-MOVE-4, CORE-MOVE-5, CORE-MOVE-9, CORE-MOVE-10, and CORE-MOVE-11.

## The instructions reach you where they apply

A rule you have to go and look for is a rule you read once, on the day somebody told you it existed.

Dinah serves the workbench's standing instructions and then the state's own, most general first, every time you claim a card and every time you move one, and it prints the moves that card may make underneath them. Dinah never copies one of those layers into the other. When you edit the workbench file, the next reader to claim anything gets the new wording, and you do not have to go and find the states that quoted the old one.

If instead you keep the rules in a document beside the workbench, that document goes stale, and you never learn which reader read which version of it. The reader who got it wrong looks the same as the reader who never opened it.

The contract: CORE-INSTR-5, CORE-INSTR-6, and CORE-INSTR-7.

## An obstacle is raised where everybody sees it

Work that has stopped should look like work that has stopped.

`dinah block <card> <reason>` marks the card blocked, records your reason, and takes the holder off it. Dinah accepts any prose you like as the reason, because the things that stop real work do not fit on a list, and whoever hits the one you left off will reach for the nearest wrong answer instead. Only the operator lifts a block, with `dinah unblock <card>`. Dinah also refuses a block on a card another owner is holding, so nobody raises an obstacle in your name and strips your claim while doing it.

An obstacle you never raised is an obstacle nobody is working on. The card sits in its state looking like a card somebody has been slow about, and the person who could have cleared it in ten minutes never learns that it is there.

The contract: CORE-BLOCK-1, CORE-BLOCK-3, CORE-BLOCK-5, CORE-BLOCK-6, and CORE-UNBLOCK-2.

## The record is what improvement works against

You cannot improve work you cannot see afterwards, and afterwards you can only see what somebody wrote down at the time.

Every claim, move, release, block, and unblock lands in the card's journal, with the time and the owner who did it, in the order the acts happened. Dinah never rewrites a line of it. `dinah log <card>` reads it back to you. Each entry keeps the names of the things it referred to as those names stood that day, so renaming a state this month leaves last month's history saying what it always said.

A workbench that forgets cannot tell you why last month took as long as it did. You are left asking the people who were there, and they will remember the week that annoyed them rather than the week that cost you.

The contract: CORE-HIST-3, CORE-HIST-4, and CORE-HIST-6.

## What binds you rather than the tool

Four of the rules on a workbench are yours to keep, and Dinah cannot make you keep any of them.

No mechanism enforces the four rules below. Dinah will not stop you, and a second tool reading the same workbench will not stop you either.

- Claim a card before you start producing work on it.
- Do not hold a claim on a card you have stopped working.
- Treat the workbench, rather than a conversation, as the authority on where a card stands and who holds it.
- Do not move a card out of a state the operator owns unless you are the operator.

Each of the four leaves its trace in the journal, so somebody reading afterwards can see where you departed from one even though nothing stopped you at the time.

The contract: ACTOR-1, ACTOR-2, ACTOR-3, and ACTOR-4.
