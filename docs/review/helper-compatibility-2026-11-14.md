# Installed helper compatibility checks

The final source candidate is bd19913a28ffe0f5258e3d8ee0a6fcd80a8655e2 on engine/helper-compatibility, based on master 27827608. It contains implementation 71cf1d113b390e7d97fc5eec67097cc4487a9759 and a later correction to the refusal's program names and document recovery instruction. The inventory and contract were committed first at 6eebb722. DESIGN.md records the rule and links to docs/design/helper-compatibility.md.

The shared policy gives separate protocol constants to bus, steering, applets, documents, extension package control, extension callbacks and detached shows. Every current constant is 1. An absent offer means 1. A compatible supported intersection selects its highest number; incompatible positive offers refuse with the newer side, channel and restart instruction. Release strings do not determine compatibility.

HTTP operations have a read-only probe, request offer and response selection. Guards precede operation decoding and dispatch. The window exposes only a read-only loopback compatibility endpoint to SQLite helpers through an inherited profile environment variable. Bus helpers check before opening SQLite. Steering command hooks check their own channel first, then the command's shared bus boundary. No endpoint file or new credential is added. Bus and steering helpers retain standalone operation when no window answers within 500 ms. The detached-show wrapper checks its parent before creating a window.

The independent extension SDK was left unchanged with the lead's agreement. The callback receiver advertises and guards legacy 1; SDK producer negotiation remains a separate recorded follow-up. External provider protocols and browser-generated bridge protocols are inventoried separately and were not altered.

## Verification and corrections

The shared protocol matrix exercises every channel with equal numbers, newer helper, older helper, a supported older peer, absent request and absent response. Real loopback HTTP tests check no dispatch on incompatibility. Terminal tests cover no endpoint, a closed listener, a server that never answers before the client budget, and an answered incompatible window. Malformed offers have separate refusal cases.

Entry-point tests cover the window endpoint, rejection before SQLite creation in both bus programs, applet connection state, refusal before opening a document, preserving an incompatible document endpoint without starting another service, both extension receivers, skipping package mutation, and refusing a detached show before any window can be created. Fixtures use invented temporary state and httptest servers.

The first full gate found a real cancellation regression in extension resume: its new probe used the canceled operation context. Probe and resume now share the existing context.WithoutCancel boundary, and the strict lease test passes. That gate also caught broker fixtures treating read-only OPTIONS as operations. Their method guards now model a legacy host refusing OPTIONS, leaving the input-byte, admission, forwarding-count and cancellation assertions intact. The initial full gate exited nonzero; a corrected focused gate passed.

The complete touched-package gate passed at 71cf1d11 through FIFO with `scripts/go-gate.ps1 ./internal/helpercompat/... ./internal/docservice/... ./internal/extensions/... ./cmd/dinah/... ./cmd/dinah-desktop/... -count=1`. Results were helpercompat 2.324 seconds, docservice 6.486 seconds, extensions 1.570 seconds, command program 1.225 seconds and desktop 253.404 seconds. The desktop product subpackage has no test files. The gate exited zero.

The later source change at bd19913a only changes refusal labels and the document recovery instruction, with a regression for the independent service. All compatibility matrix and affected entry-point tests then passed through FIFO across the same five packages, in 2.107, 0.211, 0.154, 0.155 and 0.317 seconds respectively. The full desktop suite was not repeated for this wording change. Formatting and diff checks passed.

Engine ran no browser suite, visible window, live endpoint probe or real-home write. No credential or token files were read or copied. No push, merge or installation occurred. Legacy behavior was checked through absent-number peers and legacy method guards, rather than by starting Paul's old executable.
