# Provider Switch Preparation

This implementation follows [Immediate Confirmation and Preparation After Approval](chat-workspaces-and-switching.md). Architecture approved the bridge contract on 31 October 2026 in message 2086. The cross-project [Architectural Principles](<C:/Users/paul/OneDrive/Documents/Architectural Principles.md>) apply.

## Confirmation and Approval

The existing preview methods return confirmation facts, warnings, blockers and an approved source revision. They leave the history and context empty and omit `facts.handoff`. The page opens the dialog before requesting these facts, keeps Cancel and Escape available, and enables Continue only after current warnings and authority facts arrive. An absent handoff means preparation has not run.

The revision covers saved content in the continuation chain, source options, source and destination identities, the target handoff budget and applet invocation records. Confirmation checks saved-history integrity, interrupted capture, inherited limitations, unavailable ancestors and artifact ownership without formatting a handoff or asking a model. The existing guards continue to reject running turns, queued work and pending approvals.

After approval, the engine checks the revision before assembling history, prepares the handoff once and starts the destination provider. Final validation checks the same content revision and guards without formatting or judging again. A stale source rejects the operation and requires renewed confirmation. Failure preserves the original chat and records the existing dismissible failure state. Workspace moves and offline recovery use the same split.

## Remaining History Reads

Content hashing still reads the saved files during confirmation and final validation. The hash uses the file contents; file size and modification time cannot authorize a cache hit. This preserves detection of edits that keep both size and modification time unchanged.

The metadata cache holds only identity, integrity fields, summary fallback and limitation warnings. Successful transcript writes populate it from the transcript already in memory. A cold read validates the complete transcript schema before retaining its metadata, so malformed message or attachment fields cannot evade the existing integrity checks. A matching content digest reuses the parsed metadata. The cache is bounded to 1,024 paths and is disposable; clearing it changes performance only.

Preparation still reads the full history after approval, and storage generates the separate deterministic recovery summary kept beside the new chat. That recovery summary serves a different purpose from the handoff and does not invoke the judge. Final revision validation performs no handoff generation.

## Target Provider as Summarizer

Using the selected target provider and model could align summarization quality with the model that will continue the conversation. It would also require a defined preparation request on each provider, with a confirmed model, bounded output, cancellation and tool authority disabled. A normal chat turn is insufficient because it can start agent work and consume the new conversation's context before the person has continued it.

A conversation larger than the target's context cannot be summarized by sending it whole. A bounded preprocessing step would still be required. Chunked summarization would add one request per chunk and possibly a combining request; truncation or deterministic digests would lose detail before the target sees it. The implementation would need to disclose those limits and preserve the recent turns separately, as the existing handoff does.

The target would need to start before summary preparation, moving startup cost ahead of the summary stage. Model-generated preparation adds at least one request, with the target provider's normal token and quota costs, and chunking could add several. Authentication failure, exhausted quota, cancellation or an invalid response would need an explicit failure policy. My proposal is to retain the deterministic fallback so that a failed summarizer does not prevent leaving an unavailable provider, and to keep a failed or cancelled preparation session separate from the destination chat.

The current judge summary is optional, has a 12,000-byte input cap and a ten-second deadline, and falls back to deterministic formatting. It summarizes bounded digest lines rather than the full earlier conversation. This fix preserves that policy. Changing the summarizer remains an architectural decision requiring a separate proposal and provider-specific capability review.

## Verification

Engine regression coverage holds summary preparation unresolved, checks that the summary stage is already observable, verifies that shared transcript and runtime locks are available, rejects duplicate approval and confirms that one successful switch invokes the judge once. Additional checks reject same-size history edits with preserved timestamps, detect external edits and corrupt files despite cached metadata, and exercise existing startup rollback, workspace moves and recovery.

Timing evidence uses synthetic saved conversations and an immediate fake provider. It measures engine confirmation and preparation independently from browser click-to-dialog timing. It does not measure live provider startup or real model latency. The benchmark covers 20,000 messages, approximately 39 MB of encoded history, with warm and cold metadata. Review evidence must retain that distinction.

On 31 October 2026, three benchmark iterations averaged 27.2 ms with warm metadata and 232.2 ms with cold metadata. Both paths read and hash the full saved contents. A smaller 2,000-message fixture measured 29.0 ms for confirmation and 73.4 ms after approval, including 59.4 ms of preparation and 14.0 ms of final validation and storage. The fake provider completed immediately. The fixture observed one judge invocation and checked that the shared locks were available while it was unresolved. These numbers describe this machine and fixture, without imposing timing thresholds on the correctness tests.

The full Go gate passed all 49 tested packages. Focused continuation, confirmation, preparation, workspace-move and recovery checks passed again after the final snapshot corrections. Live-tag vet, gofmt and whitespace checks were clean. The first full run failed an older test that expected confirmation to invoke the judge; the corrected test now verifies that confirmation performs no preparation and that preparation releases the shared locks before invoking the judge. Browser and live UI review belong to the combined Front End candidate and remain separate from this engine evidence.
