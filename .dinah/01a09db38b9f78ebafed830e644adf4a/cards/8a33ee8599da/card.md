---
title: A refusal names the word it did not understand but never what the verb accepts
column: 5ea2db0272fc
state: ready
severity: minor
priority: next
---
When a verb refuses a word it could not read, the message names the offending word and stops there. Somebody who typed an extra word has usually misunderstood what the verb takes, so the one thing that would help them is the thing the message withholds.

The refusal is raised from two places and only one of them knows which command is running, so saying what the verb accepts means threading the command name through both call sites. A design review judged that too large to fold into the card that surfaced it, and named the shape the fix should take.

The card decides how much a refusal should say and does that threading.
