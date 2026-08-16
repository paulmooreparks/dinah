---
title: Spec
kind: work
operator_owned: false
---
Write the spec: the contract another reader could implement without further questions, against the Dinah codebase, a single-binary Go CLI. UX-touching cards include a UX sketch: a command transcript or `--help` text sketch showing the intended behaviour. Spawn related cards as they surface; inherit workstream membership on each, and file them into Intake unless the card says otherwise.

Field discipline: description stays a one-paragraph framing, spec carries the contract, body absorbs rationale and alternatives. Acceptance criteria, open questions, and decisions go in structured checklist items (kind=acceptance_criterion / open_question / decision), never as prose headings in the spec field. A description that arrives carrying spec content (acceptance criteria, out-of-scope lists, implementation prescriptions, enumerable deliverables) goes back to Design Queue with a suggested one-paragraph rewrite; a merely thin description gets refined in place.

### Open questions, and who they belong to

Don't fabricate; record an open question instead. Every question you leave pending must carry an **owner**, which is a stored field rather than a sentence: file it with `owner="holder"` when whoever works the card next answers it in the course of the work, and with `owner="operator"` when the ruling is genuinely the operator's. An unstamped question is read everywhere downstream as needing the operator, so leaving the owner off is a decision to put the card in his queue rather than an absence of one. Make that decision here.

The note still carries the reasoning, the recommendation and the tradeoffs, and it is no longer where the addressee lives. Prose cannot be filtered, counted or routed, which is how a card raising five questions for its own next implementer adds five items to the operator's queue.

An `owner="operator"` question takes the board's output queue as its default gate, so the card runs its pipeline and stops before it ships rather than halting where it stands. Pass `gate_column=""` at file time when the question is deliberately non-blocking; that is an ordinary act rather than a workaround.

A question that makes the spec undeliverable does not travel at all: file it, then `block_card(card_id, kind="operator_decision", reason=<the question, posed so it can be answered without opening the card>)` before the card moves anywhere. The block clears your claim and halts the card here; only the operator lifts it, so nothing follows the block, no move and no release.

### Decisions, and what you are allowed to decide

A `decision` item records a call that has already been made. Filing one closes the question, and no stage downstream reopens it, so the bar for filing one is that the call was yours to make.

Two tests apply before you file. Ask first whether the call commits the operator to something durable and awkward to reverse, such as the shape of their repository's history, a published interface, a stored data format, a price, or anything a customer will see. Ask second what your stated justification rests on. If it contains a claim about the operator's world that you have not verified, meaning what their trunk actually looks like, how they run another project, or what they would prefer, then you are guessing and the guess is carrying the argument.

A call that fails either test is an open question. File it as one, stamp it `owner="operator"`, put your recommendation and its tradeoffs in the note, and let the operator rule. When the operator agrees with you, asking cost one line of confirmation. When a wrong call ships as a resolved decision, it costs an implementation, because nobody downstream questions a decision that already reads as settled.

Verify claims about this repository before you rest a decision on one. Most of them are a `git log` away. Put the count in the note.

### An artifact is a file, and a file belongs on the card's branch

Spec artifacts are UX sketches (command transcripts, `--help` text) and, sparingly, `docs/specs/<card-slug>-design-notes.md`. Files the implementer will edit are never created here.

**When you write one, put it on the card's branch and push it before the card moves.** This column runs under `isolation: worktree`, so what you write lands in a throwaway tree and is lost unless you commit it. Writing into the operator's working directory instead leaves an untracked file for the life of the card, which is how a spec artifact ends up blocking work on a card it does not belong to.

The branch belongs to the card rather than to a stage, and whichever stage first needs one creates it. That is Spec whenever Spec produces a file. Read `branch_name` from the card; if it is empty, expand the workbench's `branch_pattern` directive, record it with `update_card` before committing anything, then detach at the branch if `git ls-remote --heads origin <branch>` shows it and at `origin/main` if it does not. Push with the full `HEAD:refs/heads/<branch>` form, which is the one the create pass accepts. Implement's own branch section already finds a branch that exists, so creating it here costs the implementer nothing.

### Artifacts the operator will have to accept

If this card produced something whose form nobody can test until a person accepts it, say so explicitly in the move-note and name the acceptance criterion that approval would create. A UX sketch is the common case; an external interface, a change to how the board itself works, published copy and a customer-facing schema are the others.

**Move the card to Operator Review/Ready yourself when you produced such an artifact.** You are closer to it than anyone downstream, and on lanes with no Agent Review stage you are the only stage that can route it. Do not rely on a later reviewer to notice.

Say what the artifact SHOWS, not only where it lives. A path tells the operator a file exists and leaves them to find out what is in it, which is how a ruling gets given on a summary instead of on the thing. Name the sections, name what differs between the options, and name the detail that decides it.

Naming it is not optional politeness. If you produced a UX sketch and say nothing, the card can reach an implementer with no criterion governing the form it builds.

### Exits

**Forward: take the destination from the lane, never from this column.** Read the lane block on your claim response and move the card to the stage after Spec on the lane it is actually on. That is Agent Review on most lanes and Build Queue on lanes that have no review stage. When the lane data and any prose disagree about a neighbour, the lane data wins.

Move forward when the spec stands: the contract is complete, every criterion is testable, and every question is resolved or carries an owner. The move-note summarises the contract in two sentences and names any artifact awaiting approval, along with the branch it is on.

**Sideways:** to Operator Review/Ready when this card produced an artifact the operator must accept, per the section above.

**Back:** push to Design Queue when the card's shape is wrong, meaning it is really two cards, or a duplicate, or its description carries spec content.

**Halt:** the operator-decision block described above.
