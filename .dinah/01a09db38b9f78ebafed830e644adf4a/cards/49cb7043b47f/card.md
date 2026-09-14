---
title: Table rules line up under Devanagari headings
column: 5ea2db0272fc
state: ready
severity: major
priority: soon
---
A listing rendered in Hindi draws its rules and its columns out of line with the headings above them, because the width measurement under-counts Devanagari combining marks. A consonant carrying a spacing vowel sign occupies more screen columns than the measurement credits it with, so every column after the first drifts. This is the one language the tool ships complete, so it is the language in which a reader is most likely to see the tool at its worst. The measurement code is the tool's own rather than a library, so the fix is ours to make.
