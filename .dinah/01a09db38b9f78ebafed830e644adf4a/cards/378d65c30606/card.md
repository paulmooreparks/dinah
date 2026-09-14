---
title: the deleted event names neither what it removed nor what kind of thing it was
column: 5ea2db0272fc
state: ready
severity: major
priority: next
---
Filed on the operator's ruling against dinah-120 OQ-9, which asked whether the journal's `deleted` event should name the entity it removed and that entity's kind. The ruling was to file the card. This is that card, and it carries the `Note`-field defect too, because OQ-9 established that one write-side change fixes both and two migrations of the same records would be waste.

## Two defects, one cause

**The event cannot say what it removed.** `removalRecord` at `internal/verb/beyond.go:557` composes exactly one shape for every non-attachment removal: `bench.Event{TS, Actor, Event: contract.EventDeleted, Note: entity.ID}` with `Title` added from `titleOfEntity`. Cards and workstreams both take the `Bench.JournalPath()` branch, a retired state takes the fallback to the same journal, and `contract.EventDeleted` at `internal/contract/contract.go:264` is a single vocabulary entry with no kind member anywhere on `bench.Event`. So a deleted card, a deleted workstream and a retired state append byte-identical records, and nothing downstream can tell them apart.

**The identifier sits in a field the format disowns.** `Event.Note` in `internal/bench/journal.go` is documented as "The human's free prose, unparseable by design." `removalRecord` writes the entity identifier into it on every archive, restore and deletion, and `eventRecords` at `internal/bench/finish.go:214` reads it straight back out as an identifier on the crash-recovery path, its own doc comment saying "everything else is named by the act's own event with the identifier in the note." One field carries free prose on some events and a machine identifier on others, and its documentation covers only the first. A reader who believes the doc comment is the one who gets it wrong.

## Why it surfaced now

dinah-120 is the first consumer to trip on the gap. It reports what vanished from a workbench since a caller last asked, and it had to narrow that report to a bare identifier with the kind left empty, because the journal cannot prove more. That narrowing ships and is honest, but it means a caller can only match an identifier it already holds and cannot reason about one it has never seen. Fixing the format is what would lift the narrowing.

## What it costs, which is why it is its own card

Naming the entity and its kind in members of their own, or splitting the vocabulary per kind, touches `internal/contract`, the `wantedEvents` table and fixture comparison in `cmd/dinah/compat_test.go`, and the reader in `finish.go`. It raises a back-compatibility question about journals already on disk carrying the old shape, which the spec has to settle rather than assume: whether an old record is read as kind-unknown forever, or migrated, and what `check` says about one either way.

Settle the `Note` question in the same pass. Splitting it, so an identifier-bearing event gains a member of its own and `Note` returns to being prose, is the honest shape and is the same migration. Documenting the dual use instead is cheaper and leaves a field whose meaning depends on the event name beside it. Either way the two doc comments must stop contradicting each other.
