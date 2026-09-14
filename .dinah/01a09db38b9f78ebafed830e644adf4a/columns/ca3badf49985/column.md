---
title: Spec
slug: spec
kind: work
reject_to: design-queue
operator_owned: false
gate_items: out
---
Write the specification: the contract another reader could implement without asking a further question, against the Dinah codebase, a single-binary Go command-line tool. A card that touches what a user sees includes a sketch of it, meaning a command transcript or a draft of the help text. File related cards as they surface, with the same workstreams, into Intake unless the card says otherwise.

Field discipline: the title says what the card is, and the body carries the contract along with the rationale and the alternatives. Acceptance criteria, open questions and decisions are checklist items filed with `dinah file <card> <kind> <text>`, never prose headings inside the body. A card that arrives with its contract buried in a title goes back to Design Queue with a suggested rewrite; a merely thin card gets filled in here.

### Where an item is filed, and which way its column holds

An item names exactly one column, written with `dinah file <card> <kind> <text> --column <column>`, and the column it names is what decides where the card stops. Two directions, and they are not interchangeable.

**An item is filed against the column that settles it, and that column holds on the way out.** A decision you take while writing the specification is settled here, so it names Spec, and the card cannot leave Spec while it is pending. A question the reviewer settles names Agent Design Review. A question only the operator can rule on names the operator station that answers it.

**An item that must already be settled before a station is reached names that station, and that station holds on the way in.** An acceptance criterion is the case this workbench runs: a criterion names Merge, and the card is refused entry to Merge while the criterion is unverified. Test is the station that verifies criteria, so a criterion naming Test would have to be verified before the column that verifies it.

Getting the direction wrong is not a style matter. An exit hold reads only the column the card is leaving, so an item naming a column the card has already passed holds nothing at all, and the stop somebody meant to create silently does not exist. That is why a question raised at Test, Merge or Acceptance blocks the card where it stands instead of being filed against a column behind it.

### Open questions, and who they belong to

Do not invent an answer; file an open question instead. Every question you leave pending carries an owner, which is a stored field rather than a sentence in the note: file it with `--owner holder` when whoever works the card next answers it in the course of the work, and with `--owner operator` when the ruling is genuinely the operator's. An unstamped question reads downstream as the operator's, so leaving the owner off puts the card in his queue rather than declining to decide.

The owner is also enforced at one value. An item carrying `--owner operator` can be closed only by the operator, and anybody else is refused by name. So a question you stamp for him is a question only he can take off the card.

The note carries the reasoning, the recommendation and the tradeoffs. It does not carry the addressee, because prose cannot be read back by a command, which is how a card raising five questions for its own next implementer puts five items in the operator's queue.

A question the specification can be finished around rides the card forward and is answered at Operator Design Review. Name it in your move note so the reviewer and then the operator see it coming.

A question that makes the specification undeliverable does not travel at all. File it, then run `dinah block <card> "<the question, posed so it can be answered without opening the card>" --kind operator-ruling`. The block frees the card, so nothing follows it: no move and no release.

### Decisions, and what you are allowed to decide

A decision item records a call that has already been made. Filing one closes the question, and no stage after you reopens it, so the bar for filing one is that the call was yours to make.

Two tests apply before you file. Ask first whether the call commits the operator to something durable and awkward to reverse, such as the shape of the repository's history, a published interface, a stored data format, a price, or anything somebody outside this project will see. Ask second what your stated justification rests on. If it contains a claim about the operator's world that you have not verified, meaning what the trunk actually looks like or how he runs another project, then you are guessing and the guess is carrying the argument.

A call that fails either test is an open question. File it as one, stamp it for the operator, put your recommendation and its tradeoffs in the note, and let him rule at his station. When he agrees with you, asking cost one line. When a wrong call ships as a settled decision, it costs an implementation, because nobody after you questions a decision that already reads as decided.

Verify a claim about this repository before you rest a decision on it. Most of them are one `git log` away. Put the count in the note.

### An artifact is a file, and a file belongs on the card's branch

Spec artifacts are command transcripts, drafts of help text, and, sparingly, a design note under `docs/specs/`. Files the implementer will edit are never created here.

When you write one, put it on the card's branch and push it before the card moves. You work in a throwaway tree, so what you write is lost unless you commit it. Writing into the operator's own checkout instead leaves an untracked file there for the life of the card.

The branch belongs to the card rather than to a stage, and whichever stage first needs one creates it. A Dinah card has no field for a branch name: its fields are title, body, severity, priority and tier, and none of them is the branch. So the branch name goes in the card body under the literal heading `## Branch`, on a line of its own, and every stage after you reads it from there. Record it before you commit anything.

### Artifacts the operator will have to accept

If this card produced something whose form nobody can test until a person accepts it, the operator rules on it at Operator Design Review. Your job is to name the cargo: say in the move note what the artifact shows and state the one-line acceptance criterion his approval would create.

Say what the artifact shows, not only where it lives. A path tells the operator a file exists and leaves him to find out what is in it, which is how a ruling gets given on a summary instead of on the thing. Name the sections, name what differs between the options, and name the detail that decides it.

Naming it is not optional politeness. If you produced a sketch and say nothing, the operator waves the card through blind, and it reaches an implementer with no criterion governing the form it builds.

### This column holds on the way out

The decisions a specification author takes are settled here, so a card does not leave Spec carrying one still pending. The refusal names itself, as `dinah.unresolved-item-exit`. Answer the item and then close it with `dinah resolve <item> "<the answer>"`, or move the hold with `dinah set <item> column <other column>` when the item genuinely belongs to a later station.

That refusal applies in both directions. While an item naming Spec is pending, the card cannot be pushed back to Design Queue either, because an unresolved item is as good a reason to keep a card here on a push-back as on an advance.

### Exits

**Forward** to Agent Design Review, when the contract stands: complete, every criterion testable, and every question either settled or carrying an owner. The move note summarises the contract in two sentences, lists any pending questions stamped for the operator, and names any artifact awaiting approval together with the branch it is on.

**Back** to Design Queue, which this column declares as its reject target, when the card's shape is wrong: it is really two cards, or a duplicate, or its contract belongs to a different card.

**Halt** with the block described above.
