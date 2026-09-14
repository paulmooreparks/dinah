---
title: A cancelled release deletes its tag and leaves the release record, so somebody has to notice an orphaned draft by hand
column: 5ea2db0272fc
state: ready
severity: minor
priority: next
---
When a release run is interrupted between creating the release record and finishing, the cleanup job deletes the reserved tag and leaves the record behind. GitHub turns a release whose tag has gone into a draft, so what survives is an orphaned draft that nothing reports and a person has to find and delete by hand.

This is not hypothetical and it is not a design worry. It happened on 2026-09-05 while dinah-363 was being proved on the real platform: an implementing agent cancelled a healthy run deliberately, to avoid publishing a dev release from a throwaway branch, and an empty release record for a dev tag had already been created. Deleting the tag turned it into a draft, which the agent then noticed and deleted. Nothing else moved, and the dev channel manifest was independently confirmed untouched.

The defect predates dinah-363 and that card did not cause it. The reviewer established that the gate had completely finished, thirty-four seconds and six completed build jobs earlier, so what was interrupted was the publishing step itself, and the ordering is the same as the old design's. It is recorded here rather than folded into that card because it is a different failure and closing it there would have widened a card that was already on its third round.

What this card has to decide, and it is a real question rather than an obvious fix. Cleanup deletes the tag because a reserved tag pointing at a release that never happened is its own kind of litter, so the current behaviour is not simply wrong. The choice is whether cleanup should also delete the release record it can prove belongs to the run it is cleaning up after, or whether an interrupted release should leave both and be reported instead. Deleting a release record is destructive and the run has to be certain the record is its own, so whichever way it goes, work out how cleanup identifies the record as belonging to this run rather than to a concurrent one.

Note that the cheap alternative, leaving the draft and reporting it, needs somewhere for the report to go, and nothing currently reads release-channel state to notice litter. If that is the direction, say what does the noticing before specifying the message.
