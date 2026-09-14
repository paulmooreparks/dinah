---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:17:27Z
ordinal: 11
note: "This is the arming obligation, and it is written as a criterion because the Spec column does not run the product's tests, so no plant in this spec was performed by its author. The item deliberately names no criterion range, because round 1 wrote \"AC-1 through AC-9\" and round 2 added two criteria after it, which would have left the new plants outside an obligation nobody would have noticed had shrunk. Two traps the board has paid for apply here. A plant that fails to compile produces no output, and no output looks exactly like everything passing, so each red run's test count is read and recorded alongside its message. And a recipe can itself be vacuous, which is why the recipe is performed rather than read: if a plant does not go red, that is a finding about the check rather than about the plant, and the check is repaired before this item is verified. The card has already paid for that trap once, at Agent Design Review on 2026-09-11, where AC-13's plant named a guard that never opens the README and so could not have gone red. Gated on Merge rather than on Test because Test is the stage that performs it and gating a stage on its own work locks the card out of it."
---
Every PLANT named in the note of any acceptance criterion on this card has been performed against the branch, watched go red, restored from a byte-identical copy, and watched go green, and the red output of each is recorded in a card comment with the test count the red run reported.