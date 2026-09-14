---
title: Measuring a string ignores the escape sequences that colour it
column: 5ea2db0272fc
state: ready
severity: minor
priority: soon
---
The measure that decides how wide a piece of text draws counts every character it is given, including the invisible sequences a terminal uses to set colour. Nothing ships colour today, so nothing is wrong yet. The day anything does, every column holding a coloured value measures too wide by the length of its escape sequences, and every table drifts. Every table library surveyed that ships colour strips those sequences before measuring, which is the shape of the fix and the reason it is worth doing before rather than after colour arrives.
