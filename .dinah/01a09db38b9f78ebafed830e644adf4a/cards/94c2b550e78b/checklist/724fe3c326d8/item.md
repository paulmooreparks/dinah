---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:18:32Z
ordinal: 16
note: internal/profile reads the whole tree while being a package almost no diff names, and this diff adds a package it will see. A merged run is what catches a rule on one branch meeting data on another.
---
Current trunk is merged into the branch, the whole-tree suite is run on the merged result including internal/profile, and the pull request's checks are read green on every platform before the card leaves Test.