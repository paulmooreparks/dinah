---
kind: open_question
state: resolved
owner: holder
ts: 2026-09-14T02:16:52Z
ordinal: 42
note: "Answered here rather than left for the implementer, because D-10 settled the thing that made it a question.\n\nThe answer is internal/bench. Two exported functions live there: one that reports whether a path lies under a root, and one that walks downward from a root and returns `[]Candidate`. The package comment on internal/mcp says the head is a projection and nothing else, it holds no filesystem logic today, and the containment test and the walk are both filesystem logic. internal/bench is also where `walk`, `benchIn`, `describe`, and `samePath` live, which are the four things the new code has to agree with, and where the tests that cover them already are.\n\nWhat had kept this open was the risk that putting the containment test beside `samePath` would invite an implementer to reuse it. D-10 rules that out in writing, requires the new function to stat and compare on its own, and puts a sentence in `samePath`'s own doc comment naming the new function and saying the two want opposite failure postures. With that written down, proximity is an advantage rather than a hazard: the two functions sit next to each other and the comment on each says why they differ.\n\nThe cost is one exported function in a package the cli head also imports, which reads as a capability the cli head could use and does not. Its doc comment says so."
---
Where does the downward enumeration live, in internal/bench beside Reachable or in internal/mcp?