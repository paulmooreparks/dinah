# Group window lookup

BusResolveGroup is a read-only navigation API. It returns a state of resolved,
not-found or ambiguous. Only resolved includes a room. Operational failures use
bridge errors. The send API retains its refusal of historical titles.

Resolution prefers a stable key, then a current title, then recorded rename
history. A current title wins when another group once used that title. Multiple
historical matches are ambiguous, including when several rename records repeat
the same title across different live groups. Repeated records for one group
count as one match.

The window retains the returned canonical key across later renames. Front End
canonicalizes both opening and restoration, preserves drafts and message/mention
targets, and converges duplicate windows without discarding distinct unsent
drafts. An overview arriving late cannot establish deletion. Unresolved and
failed lookups show loading or lookup failure as appropriate.

DeleteRoom removes the room and its rename history. This change adds no deletion
records. A deleted group therefore resolves as not-found, which the UI describes
as unavailable. Neither a missing overview row nor not-found proves deletion.

Resolution chooses identity in a database transaction and fetches details by that
key. Concurrent deletion can produce not-found. Later renames cannot redirect
the selected identity through a reused title. The returned title is a snapshot;
normal overview updates refresh it after subsequent changes.

Backend regression coverage exercises stable/current/historical lookup, stable
key precedence, current-title reuse, historical ambiguity, stale-send refusal,
unknown and actually deleted rooms, operational failures and the unchanged bus
change counter. A split-phase test renames the selected room and reuses its old
title before details load, then verifies the selected identity remains fixed.
The desktop bridge test checks canonical results and absent room fields after
deletion or an unknown lookup. Delayed overview, restoration, draft preservation
and duplicate-window handling require Front End coverage and live UI review.
