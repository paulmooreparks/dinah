---
title: An exported DINAH_WORKBENCH reddens the suite
column: 5ea2db0272fc
state: ready
severity: major
priority: soon
---
Exporting `DINAH_WORKBENCH` breaks three tests in `cmd/dinah`, and the failure message talks about `init`, which sends the reader looking in the wrong place entirely.

Found on 2026-08-27 during dinah-31, by reading the list of variables the test binary clears against every environment variable the tool actually reads, and then exporting each unlisted name and running the package. `DINAH_HOME` and `DINAH_MCP_ROOT` hold fine. This one does not. `DINAH_FORMAT` had the same defect and was closed inside dinah-31, because that card is what made the variable matter in the first place.

This one predates dinah-31 and was left alone deliberately rather than folded into a review fix, because closing it changes the starting environment of every test in the package and that deserves its own run rather than riding along. The fix is reported to be about two lines.

The rule this belongs to was settled on dinah-229: the tests bind their own environment rather than asking the operator to keep his shell clean. He keeps variables set because he uses them and has said he will not unset them, so a test that inherits his shell is the test's defect. `DINAH_WORKBENCH` is a variable somebody working two workbenches would plausibly export and then spend an afternoon on a failure message pointing at `init`.

Worth doing at the same time, and the reason this is a card rather than a two-line commit: the list was found incomplete twice in one day, both times by somebody reading it against the source by hand. Decide whether something should hold the two in agreement, so the next variable the tool learns to read cannot join without the list noticing. That is the same shape as the roster guard dinah-97 is closing, and the two answers should probably look alike.
