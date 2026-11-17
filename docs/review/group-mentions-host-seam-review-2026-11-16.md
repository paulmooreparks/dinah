# Group mentions host seam review

I reviewed candidate ec0dec62b4c3f8c225f94e51aeb25eda6e12e0db on frontend/group-mentions-build without building or running it. This review covers the tagged host and its observation plumbing, rather than product acceptance.

The setup uses real App operations. BusSendToGroup returns the result of the production send, whose Reach comes from the persisted attention_reach value. That is sound evidence of the committed asking set. It is distinct from the deliveries table, which also includes background recipients.

The observation binding has the following gaps and needs correction before seam clearance.

- The prompt loop stops when the number of distinct prompted sessions reaches the expected asking count. With no expected recipients it stops immediately. A later unexpected prompt can therefore escape observation. The host needs a deterministic completion barrier for all dispatched work before taking its final counter snapshot.
- grew reduces each counter increase to one identity. A second prompt to the same chat is lost, so repeatedMemberDispatchCount cannot detect duplicate dispatch. Retain integer deltas and raw calls, including identities outside the fixture mapping.
- PendingSessionDeliveries counts only outstanding, uncancelled direct deliveries. It is not the complete stored delivery set for a particular message. Capture actual delivery rows keyed by the returned message ID, retaining delivered and cancelled fields. Database observation errors must fail the case instead of becoming missing entries.
- The binding discards the exit result returned by host.stop. A failed host exit or watchdog termination must fail teardown, and forced termination must be awaited before claiming cleanup.

These findings concern evidence completeness. They do not establish a product routing failure. Restart does reopen the bus handle through a new App over the same state, which is appropriate for checking persisted reference metadata; it does not by itself establish a full provider lifecycle restart.

Design Review owns the corrections and should send an immutable child for another source review. No build, test, merge or installation was performed in this review.

## Review of the corrected child

I reviewed 4124bc4b3bfa862179d21e4051e7e44955df2aeb without building it. Ring marks an admitted turn working before starting its asynchronous provider work, and the new barrier waits for that work and then rings again. That closes the former expected-count stopping gap for the fixture's chats. Raw prompt lists retain duplicates and unknown identities; the binding now uses integer counts. Delivery observations query the returned message ID, and teardown now checks process exit and awaits a killed child.

Two narrow issues remain. The delivery query omits cancelled_at, so a cancelled row with no delivered_at would be labelled waiting. The raw row and its state need to retain cancellation explicitly. Also, stop catches transport failures but does not inspect the JSON result of close. A loopback response containing an error would pass if the host then exited zero. Require the close response to contain result true and no error. The existing close handler always succeeds, so this is a missing refusal control rather than a reproduced runtime failure.

The immutable child can receive seam clearance after these two observation corrections. Product routing acceptance remains Quality's responsibility.

## Final observation correction review

I reviewed ec729cddfdd39892c953d1499d8834fb5c7e137c against its parent 4124bc4b without building or running it. The delivery query now retains cancelled_at and classifies cancelled rows before other states. The binding requires the close response to contain result true with no error before accepting a clean process exit. Both remaining Engine findings are closed, and the previously reviewed App and bus seams retain bounded source clearance.

The child also adds saved-chat, group and open-document snapshots and five binding mutation controls. Their independent receiving and the sufficiency of the protected-state scope remain Quality's gate. The snapshot uses saved chat records across restart and excludes the runtime-derived group file limit; this review does not widen that stated scope or claim the file limit is covered. I did not reproduce the reported 32-case pass or negative-control results. No product acceptance, merge or installation is claimed.

## Publication boundary exposed by the document control

Quality reported a SavedChats refusal on 4526449cf06e92aac7dd3701aa7983fda3caa539 in message 10027. This branch contains the shipped Start Chat publication correction. Source inspection found a separate incompleteness in the fixture's completion barrier. TurnIO.Run calls NextTurn before CommitTurn and its final EmitSession. NextTurn sets working false when no next turn remains. CommitTurn subsequently appends the turn event and finishes the saved transcript. Consequently, observing Working false does not establish that the final transcript and state publication have completed.

The fixture's next Ring can return at the empty Doorbell check without acquiring the submission lock held during finalization. Its second idle ring therefore does not close this gap. SavedChats compares transcript headers and runtime metadata before and after its read, so those pending writes can legitimately trigger its guard. This is a source-established possible failure schedule, rather than an independently reproduced identification of which stamp changed in Quality's retained run.

Design Review should extend the fixture barrier to join the actual turn-finalization boundary, then drain the existing serialized state publications before reading the protected snapshot. The existing submission serialization spans finalization and is a suitable seam to investigate. Keep the snapshot and inventory guard unchanged, add no retry, and retain the failed receipt. If the failure remains after that boundary is established, Engine should investigate the production path with evidence of the changing stamp. No build or runtime reproduction was performed for this diagnosis.

## Finalization barrier correction

Source review of 90382143416ae812b11d2b1fc5938cc69ff7af13 confirms that the fixture now takes and releases each chat's submission lock after observing idle, drains sendChatStates and rechecks for new prompts or working chats. The protected-state method runs that barrier before reading. The snapshot and SavedChats guard are unchanged, and the patch adds no retry or production change. This closes the Engine publication-boundary finding within the tagged fixture scope.

Design Review reports 40 successful positive-control repetitions, renewed failures when only the lock join is removed, a 32-case pass and five rejected mutations. Those remain producer evidence; I reviewed source without rebuilding or rerunning them. Quality retains final runtime and protected-document receiving.
