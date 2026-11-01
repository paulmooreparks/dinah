# Restart capture limitations

Ordinary restoration must not describe itself as a handoff. The restart path opens the session through SessionLifecycle.Open, loads provider history, captures replayed updates and finishes transcript replay. RecordMessageLocked previously added one handoff limitation for each unsupported item. FinishReplayLocked appended those rows again even when the replayed messages were already represented in the saved transcript.

Capture now retains the Unsupported flag without creating transcript rows. Replay completion merges that flag without appending generated warnings. Saved and parent-history views omit only the exact legacy producer shape: limitation role, empty kind, the fixed capture-omission message, and populated provider and session identifiers. Stored messages remain untouched. Ordinary quoted text and other limitations remain visible.

Explicit continuation preparation converts the capture flag or legacy records into one typed handoff limitation. Confirmation reads the same flag and retains genuine limitations. This keeps omission information available where transferring context requires it.

The investigation establishes repeated notice generation, not attachment loss. A synthetic replay that reports an image without its bytes preserves the image bytes already stored in the original transcript. This check does not establish whether every historical attachment remains available from its provider.

The focused sessionhost check passed after the final change in 0.298 seconds. It covers five fresh replay instances with non-text content, retained image bytes and legacy duplicates; repeated saved and parent-history views without disk changes; quoted-text preservation; and explicit continuation confirmation and preparation. The command was `pwsh -NoProfile -File scripts/go-gate.ps1 ./internal/sessionhost -run 'Test(CaptureLimitations|ConfirmationChainWarningsMatchPreparation)' -count=1`. Its output is retained in `.diagnostics-replay-corrected.txt`. No live chat was reopened or rewritten. Broad regression remains deferred to the end of the UI batch.
