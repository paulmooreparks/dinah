# Tool outcome presentation

Routine tool failures do not require the person to intervene. Dinah must preserve the provider outcome without interpreting whether a nonzero exit was expected or whether the agent recovered.

Collapsed activity summaries count operations by kind and show the current step. They do not count failures or acquire failure styling. A failed tool keeps its exact status chip inside Details alongside its output. Details remains available even when the provider supplied no output. Pending, running and completed steps retain their existing indicators.

Individual tool failures already avoid Notifications by default. The existing optional failure-summary setting remains available for people who explicitly enable it. Turn failures, provider failures and requests for attention retain their existing behavior. Exported transcripts retain the original tool status.

The change was built and installed on 6 November 2026 as 0.0.0-tool-outcomes-20261106. The installed executable matches the build hash, and the previous executable is preserved for rollback. Dinah remains running until the person restarts it. The receipt is build/tool-failure-noise-final/install-receipt.json. Focused browser verification covers grouped calls, an output-free failed call, streaming updates, optional failure review and turn-error notifications. Evidence is retained under build/tool-failure-noise and build/tool-failure-noise-verified.

All four focused browser suites passed. The final summary assertion run is retained under build/tool-failure-noise-final. The earlier summary assertion failures remain in the preceding evidence directories.

