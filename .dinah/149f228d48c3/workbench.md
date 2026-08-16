---
format: 1
title: Dinah development
levels:
  severity: [trivial, minor, major, critical]
  priority: [later, soon, next, now]
groups:
  DESIGN: [2f6c18c9f5d0, ca3badf49985, 0d86ad99cdbc, 5729d4578008]
  BUILD: [0789fd2dbefd, 4fda9c9ca779, 4b38abe7ebd5, ee29487fad76]
  VERIFY: [c9428b3bc921, 6c5b9d6f4414, b69abf918c42]
states:
  - 5ea2db0272fc   # Intake
  - a2eb2436b77d   # Triage
  - 2f6c18c9f5d0   # Design Queue
  - ca3badf49985   # Spec
  - 0d86ad99cdbc   # Agent Design Review
  - 5729d4578008   # Operator Design Review
  - 0789fd2dbefd   # Build Queue
  - 4fda9c9ca779   # Implement
  - 4b38abe7ebd5   # Agent Code Review
  - ee29487fad76   # Operator Code Review
  - c9428b3bc921   # Test
  - 6c5b9d6f4414   # Merge
  - b69abf918c42   # Acceptance
  - aa6cd1c6ae5f   # Done
---
This bench holds the definition of the hosted board on which Dinah itself is
being built, extracted by hand into the on-disk format this repository
specifies. It carries that board's fourteen states in board order, the three
groups that file eleven of those states (Intake, Triage and Done sit in
none), and its two level sets. It carries no
cards, and it is not a mirror of anything happening now. The hosted board
stays the arbiter of live work until somebody deliberately cuts over, so a
reader who wants to know where a piece of work actually stands should look
there and not here.

The artifact pays three ways. It gives the CLI's earliest verbs a real bench
to read before the tool can write one. It is a rehearsal of the extraction
command that does not exist yet, performed by hand so that everything the
format cannot represent surfaces as a written finding instead of a silent
approximation. And it is the first draft of the software-pipeline template
the product will eventually ship, which is why each state keeps the board's
full instructions rather than a summary of them.

The states list above is the board's column order. Read the findings below
before treating it as a route, because on the hosted board it is not one.

## Findings from the extraction

Each item below is something the source board carries that this bench does
not. Nothing was approximated to make it fit.

1. Lanes have no representation, because flow in this format is linear. The
   board runs three of them. Build, the default, goes Design Queue, Spec,
   Agent Review, Build Queue, Implement, Code Review, Test, Merge,
   Acceptance. Design goes Design Queue, Spec, Agent Review, Build Queue,
   Implement, Code Review, Merge, Acceptance, skipping Test alone. Fix goes
   Build Queue, Implement, Code Review, Merge, Acceptance. A card's lane is
   chosen once, in Triage, and everything downstream inherits that choice, so
   dropping lanes drops the board's single most consequential routing
   decision.

2. Two states sit on no lane at all. Triage and Operator Review are covered
   by none of the three routes, which on the board means a card enters them
   by a deliberate move rather than by travelling the flow. The extracted
   linear list puts them between their neighbours because the board displays
   them there, so the list describes a display order that no card actually
   travels end to end.

3. The queue kinds collapse. Design Queue, Build Queue and Acceptance are
   pull queues on the board, which are buffers a station pulls out of rather
   than stations where work happens, and the format's three kinds map both
   them and the working stations onto `work`. Intake maps onto `intake` and
   Done onto `done` cleanly. What is lost with the queues is the board's rule
   that pulling out of a queue is the moment a station's budget gets
   committed.

4. Every typed column directive is dropped, because the format has no slot
   for harness configuration and deliberately does not want one. Triage
   carries a subagent and a reviewer tier. Spec, Agent Review, Implement,
   Code Review, Test and Merge each carry a subagent; Code Review, Test and
   Merge carry a reviewer tier as well, while Spec, Agent Review and
   Implement do not. Spec, Implement, Code Review, Test and Merge run
   under worktree isolation. Implement carries the self-test commands
   `go build ./...`, `go vet ./...` and `go test ./...`. Code Review carries a
   loop limit of three and an escalation behaviour at that limit. This is the
   clearest boundary-table candidate the extraction produced: the
   configuration is real and load-bearing, it is specific to an agent harness
   rather than to a board, and it wants a home that is neither this format's
   core nor an approximation inside a state body.

5. Push-back targets are dropped with the directives, and they are a flow
   fact rather than a harness fact. Spec pushes back to Design Queue, Agent
   Review to Spec, and Code Review and Test to Implement, each as a typed
   directive; Merge's push-back exists only in its prose, which is itself
   worth noting, because a backward edge that lives in instructions alone is
   invisible to every tool. A linear list of states cannot say where a
   rejection goes, so the board's backward edges have no expression here at
   all.

6. The one-line column summaries are dropped. Each column on the board
   carries a short description that a reader sees before opening the station,
   and a state in this format has a title and a body and nothing between
   them.

7. The level sets lose everything except their names and their rank order.
   Both axes on this board are the hosting product's global defaults rather
   than board-specific declarations, and each level carries a rubric of one
   or two sentences plus display glyph and colour fields. The format's
   optional one-line hint could carry a trimmed rubric, but these rubrics are
   longer than a hint and the global-versus-board provenance has no slot at
   all, so the compact list form is the honest extraction.

8. No state on this board sets a WIP limit, an exit checklist, a handoff
   section contract, or any of the per-column entry settings, so nothing was
   lost through the gap between those features and this format. The gap is
   recorded anyway, because a board that does set them will be extracted one
   day and only the WIP limit has somewhere to land.

9. The `home:` key is deliberately absent. In this format a home is a live
   promise that the named URL hosts a contract-conforming instance answering
   the verb contract and the mirror, and the hosted board does not expose
   those surfaces yet. Naming it anyway would make a promise nothing can
   keep, and the conformance suite is entitled to probe a claimed home. An
   absent home says truthfully that this bench answers for itself.

10. The instructions were copied mechanically and carry their instance
    details with them. They name subagents by name, name Go build commands,
    assume git worktrees and a trunk branch, and speak the hosted product's
    vocabulary of columns, zones and WIP throughout. That is what a faithful
    extraction produces. Scrubbing them into board-neutral method text is an
    editorial pass that belongs to the shipped-template work, and doing it
    here would have hidden how much instance detail an extraction actually
    carries.

11. The board's own workbench-scope standing context did not come across.
    The format gives the workbench body the standing-context slot in the
    overlay chain, and this extraction spent that slot on the bench's own
    framing instead. A real extraction command has to decide whether those
    two things share one body or whether the bench-level context wants a
    place of its own.

12. Board identity is adopted, and human handles are dropped. This bench
    and its states deliberately carry the source board's own identifiers
    (the bench directory is the board's public id, and each state directory
    is its column's id), per the format's kept-identifiers extraction rule:
    shared identity is what makes a future cutover a continuation rather
    than a migration, and this bench is the same board's definition, not a
    stranger's copy. The first cut of this extraction minted fresh
    identifiers instead and did not say so; a review caught the unconfessed
    choice, and the real extraction command must expose keep-versus-mint as
    an explicit option. The slug that appears in tool calls and the per-card
    human handles of the form `dinah-11` remain dropped, since hex-only
    identity is the format's rule and human handles are one of its open
    questions.

13. Groups lose their ordering and their capacity to nest. The board's groups
    each carry a position and a parent-group reference, while the `groups:`
    map here is flat and is checked only for the resolution of the ids it
    names. The three groups happen to be flat and adjacent on this board, so
    nothing was distorted, but the mismatch is real.

14. Cards, comments, checklist items, workstreams and card journals are
    absent by design, because this extraction is definition only. There is no
    bench journal either, and that is not an omission: a bench journal appears
    on the first bench-scoped act, and nothing has acted on this bench yet.

15. The reads this extraction needed were not all available where the work
    happened. The board's lanes, its column groups and its level sets are
    served by tools that were outside the extracting agent's tool surface,
    and reaching them meant calling the hosted server's endpoint directly. A
    `dinah extract` command has to be built on reads its caller actually
    holds, which is a constraint on the command's design rather than a
    property of the format.
