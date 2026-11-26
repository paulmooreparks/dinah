# Document save and publish outcomes

A refused draft save previously resolved to its caller. Publish could therefore continue with older draft text, and another explicit Save of the refused text could return without any outcome. The existing Publish failure notice remains distinct from the preceding Save notice.

Explicit flushes now reject after showing the refusal. Publish preserves that failure and never calls the publication service after a failed flush. Explicit Save retries unchanged refused text, so fixing the service does not require editing the document merely to retry. Successful explicit Save confirms its outcome even when no changes remain. Automatic saves retain the refused-text loop protection and repeated-error suppression.

Save As submits the applet's current text to the chosen destination. It waits for an in-flight save to settle but can recover after that write fails, rather than requiring the old destination to accept a write first. Its existing failure result remains false when the packaged applet handles a refusal; thrown command errors are shown and rethrown. The document's pending-publication title and Publish availability remain truthful after refusal.

## Verification

The rendered regression uses the real packaged Editor, window controller and content host with an invented draft service. It covers a Save refusal followed by Publish with the original notice dismissed or retained, rejected callers, zero stale-draft publications, repeated explicit retries, unchanged-text automatic-save suppression, repaired-service retries, Save As recovery with the latest text after a held write fails, synchronous and asynchronous service errors, and successful and unchanged Publish outcomes.

The original source at 1433a982 fails the silent-Publish assertions in both notice variants. Source at 306e04ca passes the initial visible-notice regression in 11 seconds but fails the six stricter failure-propagation and stale-publication assertions. The final focused selection passes all eleven suites in 41 seconds, with no page errors: content-save-outcomes, document-windows, document-links, document-diagrams, document-comments, documents-menu, content-host-location, content-action-outputs, contained-content, covered-editor and diff-then-editor.

The first fixture timed out because it invoked the Editor's File Save replacement without answering its Publish confirmation. That failed receipt is retained. The first implementation blocked the Documents menu's Save As recovery after a previous refusal; the unchanged base passed that suite, and the implementation was corrected without changing the menu suite. A later positive fixture initially continued refusing the service after its simulated repair; that failed receipt is also retained.

Private evidence folders are `%TEMP%/dinah-content-outcomes-red`, `dinah-content-outcomes-baseline`, `dinah-content-outcomes-strict-red`, `dinah-content-outcomes-family`, `dinah-content-outcomes-documents-baseline`, `dinah-content-outcomes-retry`, `dinah-content-outcomes-save-as-recovery`, `dinah-content-outcomes-original-red` and `dinah-content-outcomes-complete`. All runs were hidden and headless through the shared test queue. No full browser run or installed-window acceptance was performed.

## Receiving

The branch is frontend/content-action-outcomes, based on 306e04ca. The Integrator should receive it under docs/guides/merge-and-install.md and rerun the eleven focused suites. The handover supplies the final staged guard and branch scan receipts. Front End retains correction ownership until acceptance. No merge, push or installation was performed by Front End.
