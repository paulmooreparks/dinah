---
title: the compatibility-window section still describes the floor as the oldest revision published
column: 5ea2db0272fc
state: ready
severity: minor
priority: soon
tier: workhorse
workstreams:
  - f1fd8d672caf
---
Written out in full by dinah-203's Implement stage, which had no card-creating tool, and filed here from the orchestrating session. Everything needed to work it is below, so nothing has to be rediscovered.

dinah-203 corrected four sites declaring the wrong revision and deliberately left these alone, because repairing them replaces a premise rather than substituting a number. That ruling is dinah-203's D-8, and its spec carries the argument as Group D.

Nothing is broken at runtime. A design document misdescribes the rule the code applies, and it misdescribes it in the direction that makes a reader believe an old workbench opens when it is in fact refused by name and sent to a migration.

## The five sites

**1. `docs/design/format.md`, the window section.** It reads "The floor is `dinah-core 0.1`, which is the oldest revision anything on disk declares, and the ceiling is the revision the build itself conforms to." The floor is `dinah-core 0.7`. `internal/bench/bench.go` sets `ProfileFloorMinor = 7`, and the comment above it states the design reason: the floor sits at the vocabulary rename rather than at the oldest revision anyone published, because a lenient reader would take an old card's `state:` field, which held a flow position, for the condition that key now names. Both halves are wrong, the value and the rule the value is offered as an instance of.

Why substituting the number is not the fix, tested rather than assumed: the one-token edit yields "The floor is `dinah-core 0.7`, which is the oldest revision anything on disk declares", and the tree contradicts that twice over, since the compatibility fixtures declare 0.4, 0.5, 0.6 and the retired 1.0. That trades a visibly stale line for a plausible false one.

**2. The same section's promise.** It says a raised ceiling exposes no workbench to a requirement it was not already meeting, and hands the never-breaks-an-existing-workbench promise to the fixture alarms. That promise now runs through a second window rather than through a floor at the bottom of the range. `internal/bench/vocabulary.go` names a pre-vocabulary floor and ceiling, and a workbench declaring anything inside it is refused with `dinah.needs-vocabulary-migration` and carried forward on request. The section never mentions that path, so a reader takes the promise to mean "opens" when today it means "opens, or is refused by name and migrated". How much of that belongs in this section is the judgment this card exists to make.

**3. `docs/design/format.md`, the alias explanation.** "Dinah has stamped `dinah-core/1.0` into every workbench it has created and has never stamped the other two." The universal is false now, since a workbench created today is stamped `dinah-core/0.7`. The intended claim, that of the three retired spellings only 1.0 ever reached disk, is still true and wants a clause bounding it to the pre-rename era.

**4. `internal/bench/bench.go`, the `retiredProfileName` comment.** "Only 1.0 is aliased here, because ProfileVersion has read dinah-core/1.0 in every build this tool has shipped and no workbench declaring 2.0 or 3.0 was ever written." Same shape as site 3, same remedy, and the conclusion it supports still holds.

**5. `internal/bench/bench.go`, inside `admitProfileWithin`, and this one is inverted rather than stale.** "No shipped build reaches this, because ProfileFloorMinor is 1 and the alias resolves to 0.1; a later floor raise is what opens it." `ProfileFloorMinor` is 7, the alias resolves a declared `dinah-core/1.0` to 0.1, and 0.1 sorts below 0.7, so the shipped build reaches that branch on every workbench still carrying the retired spelling. It is the branch raising the migration refusal, which dinah-203 verified is what a caller gets today. The floor raise the comment anticipates already happened.

That fifth line spells no revision number anywhere, so no grep over revision literals could have found it. It was found by checking the claim above it. Worth keeping in view: a survey bounded by a pattern bounds what it fixes and says nothing about what is wrong.

## Suggested acceptance criteria

- The window section states the floor as `dinah-core 0.7` and gives the reason the floor sits at the rename rather than at the oldest published revision, in the document's own voice rather than by quoting the code.
- The same section names the pre-vocabulary window and the migration path, so the never-breaks-an-existing-workbench promise reads true against `internal/bench/vocabulary.go` rather than only against a floor at the bottom of the range.
- Sites 3 and 4 bound their "every workbench" and "every build" claims to the pre-rename era, and each still supports the conclusion it was written to support.
- Site 5 says which builds reach the branch, correctly. The cheap guard is a test naming that branch, since `admitProfileWithin` takes its window as a parameter for exactly that reason, so the criterion can assert that a declared `dinah-core/1.0` under this build's own window refuses with the migration refusal rather than with the unsupported-version one.
- No revision literal outside these sites moves.

## Re-deriving the evidence

The window section, the alias paragraphs, the floor constants and their comment, the `retiredProfileName` and `admitProfileWithin` comments, and the pre-vocabulary window all read directly. `git log -L` over the window section's floor line dates it to 2026-08-19, under dinah-61, which predates the floor raise that landed with the vocabulary rename. This is drift the rename left behind rather than anything either card introduced.
