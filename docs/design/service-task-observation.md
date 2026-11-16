# Service Task Observation

Lead approved these generic task-client ports in message 10413 on 16 November 2026. The existing registered client identity determines the service boundary. Neither port accepts a caller-selected event name or another service identity.

`Client.PendingQuestions(ctx, scope)` returns the host's actual pending questions for the requested workspace, task and current generation. It validates the client identity and current task authority before reading, refuses any returned question outside that scope and rechecks authority after reading. It does not answer, cancel or create a question. The opaque host key and typed bounds are preserved.

`Client.PublishChange(ctx, scope, sequence)` emits a saved-record identity after a successful consumer save. Sequence must be positive. The event contains the registered service, workspace, task, generation and sequence, without record content or authority. The window emits it as `service:<registered-service>:changed`. Consumers must publish only after their durable save succeeds. Pages reread authoritative service representations and discard older sequences.

Both ports refuse a closed window, invalid current scope or missing host capability. They admit no execution and create no task handle. The question adapter filters the actual pending host list before returning records to the client. These additions do not introduce a subscription port through which a service could observe another service's events.

## Verification

The task-host package passes with race detection and all vet checks in 1.966 seconds. The focused desktop gate for task-host, local-inspection and task-question tests passes in 1.440 seconds with production and desktop tags. The receipt is retained at `%TEMP%/dinah-service-observation-desktop.txt`. Tests cover registered identity scoping, refused cross-service reads and emissions, foreign question results, copied answer options, positive event identity and sequence, zero-sequence refusal and closed-window refusal. No provider work or execution handle is admitted by these tests. Quality source review and Lead integration remain pending.
