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
