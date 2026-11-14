# Extension callback probe verification

The `engine/host-options` branch starts from master `4d4e8f0c4d2740353dae5818185322c36cc36c3d`. The receiver now authenticates OPTIONS on `/host`, applies the existing protocol guard and returns 204 before reading or dispatching a callback. POST behavior remains unchanged.

The tests cover equal and absent protocol offers, an incompatible offer, missing and unknown tokens, an expired scope, incorrect Host, browser Origin and an unsupported method. A body spy proves that none of these probes reads a callback. A real loopback callback test exercises the SDK client against the receiver using invented temporary state.

The touched extensions suite passed through the shared FIFO with the existing SDK main in 1.181 seconds. It passed again in 1.210 seconds with the prepared SDK branch selected through a temporary module file outside the repository. The candidate selection changed no checked-in dependency file or SDK main branch. Both runs used `go test ./internal/extensions/... -count=1`. Formatting and `git diff --check` passed.

The SDK branch is `engine/helper-protocol` at `483d6464ee17dd4e974aaa0d6bfac914c5d3d343`, with implementation at `3b9e3ee1c5b66afdc405fad764882726b7e70e61`. Its worktree is `C:\Users\paul\source\repos\dinah-extension-go\.worktrees\engine-helper-protocol`. The SDK's own Go tests, vet and seven simulated browser bridge tests passed, as recorded in its `docs/review/helper-protocol-2026-11-14.md`. SDK main and origin/main remain at `444129580fb6f8f8d3ce3248a09f8df97c6699ac`.

The SDK release remains held for the person's decision because its main branch also contains other unreleased work. After publication, Dinah needs the chosen module version and matching build source, and extension publishers need rebuilt, versioned and signed packages. The host change can ship independently while existing callers stay on legacy 1.

No push, merge, installation, tag, visible window, desktop input or live extension operation occurred. No real home or credentials were used. Native extension package execution and a published SDK upgrade were not tested. The requested next action is the lead's review of the host branch; the SDK branch stays unchanged pending the release decision.
