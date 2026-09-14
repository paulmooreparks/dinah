---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:18:31Z
ordinal: 8
note: "Verified at 5a88bec, re-measured on this branch rather than read off the spec. A throwaway probe in cmd/dinah drove the tree's own guideProse and guideDenialOfACapability over guide.Topics(), then was deleted before any commit. Result: roster 56 commands, 8 topics, per-guide sentences first-session 49, getting-started 34, verbs 25, principles 76, references 77, query 88, workbench-layout 18, mcp 159, total 526; firings 12 with backticks in place, 14 with them stripped, and 0 of those 14 captures a name in the roster. The widened check therefore reports zero findings on the unmodified tree, which the shipped test confirms by passing and logging \"8 guides scanned, 526 sentences, against a roster of 56 commands\". Every figure matches the spec's measurement at 866221f exactly. The quick start reproduced too: 401 sentences, 7 firings, exactly one naming a command (`block`, in the true sentence \"carries no block\")."
---
On the unmodified tree the widened check reports zero findings across all eight guides, so the widening ships no false positives on arrival. If it reports any, the finding is investigated and reported rather than exempted, and the card returns to Spec with the count.