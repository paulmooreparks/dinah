---
title: What a command accepts is declared once
column: 5ea2db0272fc
state: ready
severity: minor
priority: soon
---
How many positional words a command takes, and where its free text begins, is one fact about that command, and it is currently written in three separate places that each drive something different. Seven cards so far have been instances of the resulting confusion rather than separate defects, and the argument parser has grown into a long accumulation of rules whose comments cite four different card numbers. One divergence is visible today: the command that creates a workbench accepts a positional path that its own help does not mention, which is the same shape as an earlier defect that put a workbench into the wrong repository. The parser rules underneath rest on a question the operator has not yet ruled on, so this card follows that ruling rather than leading it.
