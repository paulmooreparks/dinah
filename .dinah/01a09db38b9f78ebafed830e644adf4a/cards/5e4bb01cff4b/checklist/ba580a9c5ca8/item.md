---
kind: open_question
state: resolved
column: c9428b3bc921
owner: operator
ts: 2026-09-14T02:17:03Z
ordinal: 42
note: "My call: prose() removes a backtick span, an angle-bracket placeholder and a --flag token as well as a {brace} placeholder, on the same rationale D-5 already gives for braces. It changes exactly one key's trigger status, measured over all four terms: hi/refusal.dinah.ambiguous-state.next, whose English is \"name one as `dinah pull <state>`, or pass --state <state>\" and which names the flag three times and the concept never. Without the extension the guard would demand a Hindi word for \"state\" in an entry that is pure command syntax, and the only way to make it pass would be to pad a correct translation. German passes it today by accident, on a Zustand elsewhere in the sentence. Counts, measured on origin/main: the term \"state\" triggers on 78 keys with brace-stripping alone and 77 with the extension; \"the root\" 4 and 4; \"owner\" 18 and 18; \"level\" 7 and 7. Tradeoff: the extension is three regexes nobody asked for, and it narrows one trigger. Leaving it out would ship the guard red on a correct entry."
---
The glossary trigger strips three more kinds of machine-vocabulary span than the spec's brace placeholders. Ratify the extension, or redirect.