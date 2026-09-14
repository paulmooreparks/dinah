---
title: The log prints two event names as raw machine tokens
column: 5ea2db0272fc
state: ready
severity: minor
priority: next
---
Reading a card's history prints two of its event names as raw machine tokens rather than as something written for a person. A reader meets an underscore-joined identifier where every neighbouring line reads as English.

Found by building the binary and reading the output during dinah-409's code review, rather than by inspecting templates. That is worth recording because it is how the defect stayed invisible: the tokens render, nothing fails, and a test asserting the line contains the event's name passes on the token exactly as it would on a sentence.

The card that found it had just added a rendering for its own new event because without one the log would have printed a raw token to a reader for the first time. These two predate that and were missed by the same reasoning, which suggests the question to ask is not which events are missing a rendering but whether anything establishes that every event has one. Prefer a check that enumerates the events and fails when one has no rendering over a card that adds two more.

Note the neighbouring finding from the same review: the log's own detail column was empty for tier events until dinah-409 added the case. So this is the third instance of the same shape in one area, and a card that fixes two names without closing the class will be followed by a fourth.
