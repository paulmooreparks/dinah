# Background close fixture wait

The full Go gate on `2ca62dc4` reported `TestClosingAWindowNeverWaitsForTheAgent` failing after 5.65 seconds because the agent had not yet been asked to close the session. The original failed gate remains in `%TEMP%/integrator-gogate-2ca62dc4-fail-1345.log`. The isolated test and the full gate subsequently passed.

The failed assertion was the fixture's wait for a background request, after the window-close call had returned. That wait now uses the test lifetime context. The separate five-second bound on the window-close call is unchanged. The agent remains blocked on its gate until the test has observed the call returning and the close request arriving, so a close call that waits for the agent still fails. Archive refusal, unloaded-session and reopened-window assertions are unchanged.

With Go 1.26.11, the test passed five times in 2.734 seconds. Its output remains in `%TEMP%/dinah-close-fixture-wait.log`. Only one test line and this record change. No UI or product implementation changes, and no browser checks apply. The full merged-tree gate remains a receiving check.
