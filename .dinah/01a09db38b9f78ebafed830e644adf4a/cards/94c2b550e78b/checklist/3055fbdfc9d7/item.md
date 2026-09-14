---
kind: decision
state: resolved
column: 0d86ad99cdbc
owner: holder
ts: 2026-09-14T02:18:33Z
ordinal: 26
note: "Measured: folding the references guide's table gives \"| show | no | yes | yes | yes | yes | | instructions | ...\", so the pipes survive and the pattern never reaches a command name across a cell boundary. The scan gives zero firings with the strip and zero without it, confirming the card's correction. referencesGuideProseParagraphs is deleted, so its comment travels no further. guideProse still drops table rows, and the reason is simply that a table row is not a sentence; the new test's doc comment says so, because guideProse defers its rule to whichever test calls it."
---
The wrong table-strip justification is retired with the function that carries it, and the surviving reader's rule is written out on the new test instead.