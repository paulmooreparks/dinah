# When a pipeline earns its cost

## What this pattern is

A pipeline, in Dinah's own sense, is a workbench one card walks through
once. `dinah init --from pipeline` ships one: Intake, two work columns
named Draft and Review, and two terminals named Done and Returned, with no
column meant to hold a card while somebody waits on it. This guide is
about when to reach for that shape and when a plain skill already does the
job.

## The test

Build a pipeline only when a stage in it benefits from not knowing what
produced its input, and the artefact chain makes that ignorance checkable
afterward. Where no stage benefits from being blind, a plain skill does
the same work for less: it costs no columns, no claims, and no move
ceremony, and a pipeline still pays for all three even where nothing in
it needs to be blind.

## When the cost is worth paying

The pattern earns its cost when at least one of these holds.

- A stage has to judge its input without the context that produced it,
  the way a reviewer, a back-translator, or a blind grader does.
- The run has to survive a crash or a context reset and pick up again
  from the card, which a running conversation cannot do.
- Several cards run at once and their claims on the same material have to
  stay apart.
- Somebody has to audit the run afterward, and the card's own attachments
  are the record that audit reads.

## A non-software example

A translation pipeline fits the same two work columns this template
ships. Intake carries the source text and the language pair. Draft
translates it. Review is given only the translation and the source; it
produces a back-translation and reports where its meaning drifts from the
source. Because Review never saw the translator's reasoning, a phrase
that reads naturally in the target language but drifted in meaning is
caught rather than waved through by the person who chose the words. A
drift Review cannot resolve from the source alone moves the card to
Returned, for a human translator to settle; a translation that survives
Review moves to Done.

The same shape carries an incident postmortem, where a narrative stage
writes what happened and a second stage checks every sentence of that
narrative against a timeline it is given but did not write, catching a
gap the narrator's own account would otherwise explain away. It also
carries a grant or tender response, where a draft answers each
requirement and a compliance stage maps every requirement to the sentence
that answers it, without seeing why the drafter believed one already did.

## Refinements worth keeping

Building the first pipeline of this shape by hand left practices worth
carrying into any pipeline built from this template.

The session driving the pipeline should do nothing but move cards and
read results. Each work column's judgment belongs to its own fresh
subagent; a driving session that also writes the draft or the review has
collapsed the pipeline back into the skill it was meant to replace.

Where a stage runs an automated scan rather than an agent's judgment, its
findings travel better as checklist items filed against the column that
must see them settled than as a paragraph of prose nobody is obliged to
act on. Give that column its own `gate_items` declaration, so the hold
belongs to the workbench rather than to the script that ran the scan.

The stage moving a card to Returned should file its reason on the
workbench that owns the work the pipeline was run for, and the card that
left this way stays closed. A fresh card starts the pipeline again once
what was missing is settled.

## The mechanics this template already carries

Draft and Review each declare `reject_to: Returned`, so a stage that hits
something it cannot resolve moves the card there by name rather than by
an ordinary backward move nobody can tell apart from a revision. Review
declares `loop_limit: 3`, so a card cannot cycle back to Draft more than
three times without the operator carrying it through with an override.
The first hand-built pipeline enforced the same bound by discipline
alone; here `dinah check` reads it as a declaration, and `dinah move`
refuses against it on its own.
