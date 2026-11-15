# Task host receiving handover

The task host is integrated on engine/task-host-current from master 15afdc2c. This base includes the shipped shared inventory and passive start-up. The integration applied without conflicts. Its new race/all-vet selection remains pending, so this note does not claim clearance on the integrated tree.

Quality accepted the earlier source 0153e053af1c75dd28524d00011e2f79140d59e7 in review 62126cf022064eb6e0db251afb500b91719fcb52. Its independent run passed all ten frozen top-level tests and both supplemental binding controls in 122.934 seconds with race detection and all vet analyzers. The existing Windows linker warning remains. Raw producer and toolchain logs remain on the private engine/task-host-contract branch. They are absent from this public candidate and its history. This note is the public receiving summary.

## Binding and provenance

Quality froze internal/qualitytaskhost/acceptance_test.go at 73054c5e47458e8eb51844c7e69225e89ceac68e. The receiving copy is cmd/dinah-desktop/task_host_frozen_assertions_test.go. Its adaptations change only the package and test-name prefixes; reversing those changes reproduces the frozen assertions. Quality's optional invented-room clarification at c38882b0 applies.

The quality_task_host binding registers an actual per-window service and a private consumer record port. It persists invented workspace/task records, then invokes production Review, Begin, Start, Read, Rebind and Cancel. Consent answers the actual task-owned question through App.Answer. The provider peer sits beneath ordinary App opening, prompt construction, TurnIO admission and permission handling. Counters observe native allocation, load and prompt requests. Unexpected program preparation and discovery refuse before external launch. The action peer runs a bundled action through the real broker. Every lost-response scenario requires a consumed request.

The optional invented room peer owns a room, canonical lead and phase claim. It consumes a post and loses the response. The real host records unknown in its journal, and repeat, restart and recovery make no second post. Pending-work scenarios also start a real host-owned observation under a reviewed ObserveRoom permit. Its peer records live, cancelled and completed observations, and Snapshot reads the live observer count. Positive controls require the observation to be alive before cancellation, binning or shutdown. Production registers no room adapter; a separate test requires absent-room refusal before work.

Protected-record observations read actual local and remote resource bytes, durable document draft JSON, saved baselines, unsaved edits and the consumer record linking those resources to the task. The invented remote writer counts attempts before refusal or mutation. Positive controls save through App.SaveDraft and require both a refused attempt and a successful resource/draft update to be visible. Restart joins the old App and reconstructs a replacement from the same isolated stores. Shutdown observations cover admission closure, cancellation, private receipt persistence and shared teardown.

## Scope and receiving gate

The integrated rerun must select Test(FrozenTaskHost|Task|AppTaskQuestion) under desktop,quality_task_host with -race -vet=all. It must also run the affected internal taskhost, services, sessionhost, ACP, Codex, Ollama, broker and applet-host packages and vet the desktop production build. Every command uses scripts/with-test-lock.ps1. No frozen assertion may be weakened.

Begin currently requires fresh person confirmation. Grant-only admission is unavailable. Review questions show operation summaries and fingerprints; the consuming feature must supply its task definition for review. No Flow consumer is registered. ReleaseChat removes a tracked runtime interest and leaves native unloading to the ordinary close policy. Conditional model restoration uses the task's own receipt and refuses after an intervening person choice or runtime reconstruction.

The earlier Quality clearance covers the assigned host gates only. Production rooms, Flow execution, rendered task questions, consumer recovery, combined Front End acceptance and installed-product acceptance remain outside it. Front End's Notifications owner variant remains separate. Engine retains corrections and will hand over the immutable integrated tip after its required rerun.
