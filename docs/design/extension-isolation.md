# Extension isolation proposal

Paul requested this design in message 3327. It proposes an isolation boundary for extension code and records the migration costs. It authorizes no sandbox implementation. The [Architectural Principles](C:/Users/paul/OneDrive/Documents/Architectural%20Principles.md) apply.

## Current authority

The extension manager starts a signed native executable with the person's account token (`internal/extensions/manager.go:210`). The executable can use operating-system APIs directly. Invocation-scoped host permissions limit its broker calls, but cannot prevent direct file access, networking, child processes or credential access. Signatures identify the publisher and package contents; they do not restrict execution.

The process job controls lifetime. Microsoft documents [job objects](https://learn.microsoft.com/en-us/windows/win32/procthread/job-objects) as process groups with resource and termination controls. We cannot describe that mechanism as filesystem, network or credential confinement. The frontend's sandboxed iframe is a separate boundary and does not restrict the native backend.

The current environment deny list removes selected secret names while passing arbitrary others. The authorized interim correction replaces it with a documented standard allow list plus explicit variable names in the signed manifest, and tests it with synthetic secrets. The installation review must say that native code runs with the person's full local authority until isolation is implemented. Environment filtering reduces accidental disclosure; it cannot stop a native extension from reading secrets elsewhere.

## Candidates

| Architecture | Honest boundary | Cost and limitations |
| --- | --- | --- |
| Separate operating-system user | Access follows that account's ACLs and privileges. | Requires identity provisioning, private storage and authenticated IPC. Existing broad ACLs and shared resources still need review. Installation and credential handling become platform-specific. |
| Restricted token plus job | Restricting SIDs and removed privileges limit access checks; the job adds lifetime and resource controls. | Requires explicit token, ACL, handle and desktop policy. It does not automatically create a filesystem or network capability boundary. Microsoft also warns about sharing the normal desktop with restricted applications. |
| AppContainer for native backends | Windows enforces application identity and capability restrictions for resources including files, credentials and networking. | Requires per-extension profiles, explicit resource grants, handle-based IPC and a supported launcher. Existing loopback callbacks and native dependencies need redesign. Other operating systems require separate designs before we claim equivalent isolation. |
| WASM-only extension backends | A runtime mediates host imports; guest code receives only the APIs and resources supplied by the host. | Requires a guest SDK, bounded execution and a maintained runtime. Existing native binaries and arbitrary native libraries cannot run unchanged. Every imported function remains part of the trusted boundary. |
| Broker-only declarative extensions | No publisher executable runs; the host performs a finite set of declared operations. | Offers the smallest execution surface, but complex algorithms become host features or cannot be expressed. A general command or scripting escape would restore broad authority. |

Microsoft documents the double access check for [restricted tokens](https://learn.microsoft.com/en-us/windows/win32/secauthz/restricted-tokens), including the separate-desktop warning. Its [AppContainer contract](https://learn.microsoft.com/en-us/windows/win32/secauthz/appcontainer-isolation) covers application identity and resource isolation. These are different guarantees; adding a restricted token to a job does not supply the AppContainer contract.

[Wazero documents](https://wazero.io/docs/) guest memory isolation and host imports for operations outside the core WebAssembly environment. That makes it a candidate runtime, subject to API and dependency review. We must choose documented memory limits, cancellation behavior and host-call limits before implementation. This proposal makes no claim based on a probe or successful sample run.

## Recommendation

I recommend WASM-only backends with a small broker API for new untrusted extension logic, while retaining the existing signed-native mode with its full-authority warning during migration. This provides one portable extension contract and avoids making the Windows security model part of every extension's application code. AppContainer is the alternative if Paul requires native binary compatibility; its cost includes a separate isolation design for every other supported operating system.

The guest should receive no inherited environment, network access, filesystem preopens, native library loading or process creation. Each host call must bind to an invocation, workspace and declared action, with bounded input, output, memory and time. Signed manifest grants remain subject to the person's approval policy. The host must reject unsupported imports and capabilities rather than silently run a guest with native authority.

The broker also needs its own security review. A host call that launches an unrestricted native tool delegates that tool's authority even when the caller is isolated. The UI and documentation must disclose that distinction.

## Migration of the Git extension

The existing `com.parkscomputing.dinah-git` extension already invokes Git through SDK host services, as its README and `git.go` describe. That separates application logic from process launch, but does not yet make the application logic safe to distrust. Its existing Go SDK uses native MCP transport; we have not established that it can compile unchanged for a WASI target.

1. Inventory the extension's host calls and preserve its signed package, settings, views and data format while defining the guest transport. Build the guest SDK around broker imports, without a loopback HTTP callback or inherited bearer token.
2. Port the backend logic and retain the existing frontend views. Replace general process requests with reviewed Git operations or explicitly privileged tool delegation. Git hooks, credential helpers, SSH commands and repository configuration can launch other programs under the person's account. Their current behavior must be disclosed and decided, rather than silently disabled or described as confined.
3. Run the existing Git behavior fixtures and negative boundary guards against the guest. Review package signatures, upgrades and rollback. Keep the native package available until the guest preserves the accepted product behavior and Paul accepts the remaining delegated-tool authority.
4. Make isolated execution a manifest contract with an explicit minimum host version. An older host must refuse an isolated package it cannot enforce. A failed guest launch must never fall back to native execution automatically.

The acceptance guards must attempt undeclared file, environment, network and process access, unsupported imports, cross-invocation callbacks and stale capabilities. Resource tests must prove bounded guest memory and host-call concurrency and that cancellation releases the invocation. Equivalent tests are required for AppContainer if that alternative is chosen. Native-mode warning and canary-environment tests remain necessary during coexistence.

Paul chose the WASM guest contract with a constrained broker in decision 7, relayed in message 3399. Native mode retains its full-authority warning and environment allowlist during migration. AppContainer is not the selected implementation path.

The implementation plan follows TM-06 and the file-access policy landing. Engine will append it here for the lead to review before implementation begins. The plan must specify every broker import and its invocation, workspace and declared-action binding; input, output and concurrency bounds; the runtime's documented memory and cancellation contracts; the guest SDK changes in dinah-extension-go; signed manifest minimum-host-version and refusal rules; and the acceptance guards above. The four Git migration steps remain, with hooks, credential helpers and SSH delegation presented to Paul as an explicit remaining authority decision. This records the choice and required next deliverable; it does not begin the implementation plan or authorize guest code.
