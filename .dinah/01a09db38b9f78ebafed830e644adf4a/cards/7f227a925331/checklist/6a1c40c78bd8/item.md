---
kind: decision
state: resolved
owner: holder
ts: 2026-09-14T02:18:45Z
ordinal: 29
note: "Round 3's proposed entry read \"The position a card fell in among the cards of its workbench, fixed when the card was created and never afterwards changed\", and this card disclaims both halves of it. The migration's tie-break and `dinah check --renumber --yes` both change a card's number after creation, which is what the `renumbered` event exists to explain, and tombstones plus renumbering leave the numbers sparse, which the accepted-cost section states outright. A conformance run reads CORE-CARD-10 through section 4, so under that entry the first operator to run the repair this card ships would have put Dinah outside its own statement. The entry now reads: a whole number a workbench allocates to a card when the card is created, unique among the cards of that workbench, which a tool may reallocate when two workbenches merge and two cards arrive holding one, with the ordinals not necessarily consecutive and CORE-QUEUE-3 breaking a tie on the ordinal. The statement itself did not move: it is CORE-CARD-1's shape with one noun changed, it carries one keyword, and round 4 confirmed both its shape and its scope against the code. The entry carries no upper-case RFC 2119 keyword, because `extract.go` reports one outside an identified statement as a stray and its pattern matches upper case alone."
---
Section 4's creation-ordinal entry is written from what the implementation guarantees, and it does not say the ordinal is fixed at creation or that it is a position.