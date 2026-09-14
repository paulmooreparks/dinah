---
title: A pending item's note cannot hold more than one line, though a closed item's can
column: 5ea2db0272fc
state: ready
severity: major
priority: next
workstreams:
  - 994787601ae6
links:
  - kind: relates_to
    to: 5ef07a3b83a3
---
A pending item cannot carry the reasoning that a pending item exists to carry, because the two writers of its note disagree about line breaks.

The `note` field on a checklist item has two writers and they accept different values. `dinah resolve|verify|fail <item> <note|->` stores a multi-paragraph note without complaint. `dinah set <item> note <value|->` refuses one outright:

```
malformed note is missing, empty, or will not parse; a value for note is stored
on one line, and the value you gave carries a line break
```

Same field, same item, same bytes. Reproduced on 2026-09-14 while carrying dinah-449 across: one item took a two-paragraph note through `resolve` and the identical value was refused through `set` on a sibling item.

## Why it matters here rather than being a curiosity

`resolve`, `verify` and `fail` all close an item. So the only way to give an item a multi-paragraph note is to close it, and a PENDING item can only ever carry one line.

This workbench's own instructions require the opposite. An open question stamped for the operator is meant to ride the card to his station carrying, in its note, the reasoning, the recommendation and the tradeoffs he is being asked to rule between. That note is the whole content of the question. Today it has to fit on one line, or the question has to be closed before it is answered, which is the one thing it must not be.

The same applies to a decision an implementer files and to any item a reviewer stamps and leaves pending for a later stage.

## What was done instead, on dinah-449

The question was filed with a one-line note pointing at a comment on the card carrying the full reasoning. That works and it is worse: a comment is not attached to the item, nothing ties the two together, and a reader of the item sees a pointer rather than an argument.

## Scope

Decide which writer is right. Either `set` learns to store a multi-line value the way the closing verbs already do, which is the smaller change and makes the two writers agree, or the one-line rule is the real contract and the closing verbs are wrong to accept more, which would be a much larger change and would break notes already stored. The first reading is almost certainly the right one: the storage clearly handles multi-line values, since the closing verbs write them and a read returns them.

Whichever way it goes, the two writers must agree afterwards, and `dinah help set` should say what the field takes.
